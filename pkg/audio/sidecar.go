package audio

import (
	"log"
	"net"
	"os"
	"sync"
)

type SidecarServer struct {
	path        string
	listener    net.Listener
	clients     map[net.Conn]struct{}
	clientsLock sync.Mutex
}

func NewSidecarServer(path string) *SidecarServer {
	return &SidecarServer{
		path:    path,
		clients: make(map[net.Conn]struct{}),
	}
}

func (s *SidecarServer) Start() error {
	// Remove existing socket if it exists
	if _, err := os.Stat(s.path); err == nil {
		os.Remove(s.path)
	}

	l, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	s.listener = l

	log.Printf("Sidecar UDS server listening on %s", s.path)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.addClient(conn)
		}
	}()

	return nil
}

func (s *SidecarServer) addClient(conn net.Conn) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()
	s.clients[conn] = struct{}{}
	log.Printf("Sidecar client connected: %s", conn.RemoteAddr())
}

func (s *SidecarServer) Broadcast(data []byte) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	for conn := range s.clients {
		_, err := conn.Write(data)
		if err != nil {
			log.Printf("Sidecar client disconnected: %s", conn.RemoteAddr())
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
