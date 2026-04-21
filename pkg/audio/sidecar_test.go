package audio

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSidecarServerBroadcastAndCommand(t *testing.T) {
	socketPath := testSocketPath(t, "broadcast")

	server := NewSidecarServer(socketPath)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Close()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	waitForClient(t, server)

	wantPCM := []byte{0x01, 0x02, 0x03, 0x04}
	server.Broadcast(wantPCM)

	gotPCM := make([]byte, len(wantPCM))
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	if _, err := conn.Read(gotPCM); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(gotPCM) != string(wantPCM) {
		t.Fatalf("broadcast mismatch: got %v want %v", gotPCM, wantPCM)
	}

	if _, err := conn.Write([]byte("tx K1ABC\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	select {
	case got := <-server.CommandChan:
		if got != "tx K1ABC" {
			t.Fatalf("CommandChan got %q want %q", got, "tx K1ABC")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for sidecar command")
	}
}

func TestSidecarServerReadsLineDelimitedCommands(t *testing.T) {
	socketPath := testSocketPath(t, "commands")

	server := NewSidecarServer(socketPath)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Close()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	waitForClient(t, server)

	writer := bufio.NewWriter(conn)
	if _, err := writer.WriteString("pileup 3\nwpm 35\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	for _, want := range []string{"pileup 3", "wpm 35"} {
		select {
		case got := <-server.CommandChan:
			if got != want {
				t.Fatalf("CommandChan got %q want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for command %q", want)
		}
	}
}

func testSocketPath(t *testing.T, name string) string {
	t.Helper()

	socketPath := filepath.Join(os.TempDir(), "mr-"+name+"-"+time.Now().Format("150405.000000")+".sock")
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})
	return socketPath
}

func waitForClient(t *testing.T, server *SidecarServer) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		server.clientsLock.Lock()
		clientCount := len(server.clients)
		server.clientsLock.Unlock()
		if clientCount > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for sidecar client registration")
}
