package audio

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
)

// SidecarServer handles two-way IPC via Unix Domain Sockets for AI sidecars.
type SidecarServer struct {
	path        string
	listener    net.Listener
	clients     map[net.Conn]struct{}
	clientsLock sync.Mutex
	CommandChan chan string // Channel for receiving commands from AI sidecars
}

func NewSidecarServer(path string) *SidecarServer {
	return &SidecarServer{
		path:        path,
		clients:     make(map[net.Conn]struct{}),
		CommandChan: make(chan string, 100),
	}
}

func (s *SidecarServer) Start() error {
	// Remove existing socket if it exists
	if _, err := os.Stat(s.path); err == nil {
		if err := os.Remove(s.path); err != nil {
			return err
		}
	}

	l, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	s.listener = l

	slog.Info("Sidecar UDS server listening", "path", s.path)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				if !errorsIsClosed(err) {
					slog.Error("Sidecar: accept failed", "error", err)
				}
				return
			}
			s.addClient(conn)
		}
	}()

	return nil
}

func (s *SidecarServer) addClient(conn net.Conn) {
	s.clientsLock.Lock()
	s.clients[conn] = struct{}{}
	s.clientsLock.Unlock()
	slog.Info("Sidecar client connected", "addr", conn.RemoteAddr())

	// Read loop for incoming commands
	go func() {
		defer func() {
			s.clientsLock.Lock()
			delete(s.clients, conn)
			s.clientsLock.Unlock()
			conn.Close()
			slog.Info("Sidecar client disconnected", "addr", conn.RemoteAddr())
		}()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			cmd := scanner.Text()
			select {
			case s.CommandChan <- cmd:
			default:
				slog.Warn("Sidecar: command channel full, dropping command", "cmd", cmd)
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Warn("Sidecar: command read failed", "error", err)
		}
	}()
}

func (s *SidecarServer) Broadcast(data []byte) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	for conn := range s.clients {
		_, err := conn.Write(data)
		if err != nil {
			slog.Error("Sidecar: error broadcasting", "error", err)
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

func (s *SidecarServer) Close() error {
	s.clientsLock.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clients = make(map[net.Conn]struct{})
	s.clientsLock.Unlock()

	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func errorsIsClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.Is(opErr.Err, net.ErrClosed)
	}
	return false
}
