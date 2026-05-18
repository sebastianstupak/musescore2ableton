package ableton_test

import (
	"net"
	"testing"
	"time"

	"github.com/hypebeast/go-osc/osc"
	"github.com/sebastianstupak/m2a/internal/ableton"
)

// startMockServer runs a UDP OSC server on a free port.
// It responds to addr with the provided reply args.
func startMockServer(t *testing.T, addr string, reply []interface{}) (port int, stop func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mock server: listen: %v", err)
	}
	port = conn.LocalAddr().(*net.UDPAddr).Port
	go func() {
		buf := make([]byte, 1024)
		for {
			n, sender, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			_ = n
			// Echo back a response to the sender
			msg := osc.NewMessage(addr)
			for _, a := range reply {
				msg.Append(a)
			}
			data, _ := msg.MarshalBinary()
			conn.WriteTo(data, sender)
		}
	}()
	return port, func() { conn.Close() }
}

// newTestClient creates a Client wired to a mock server that responds to addr with reply args.
func newTestClient(t *testing.T, addr string, reply []interface{}) *ableton.Client {
	t.Helper()
	mockPort, stopMock := startMockServer(t, addr, reply)
	t.Cleanup(stopMock)

	// Bind a free local port for the receive side, then release it for the client.
	recvConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newTestClient: bind recv port: %v", err)
	}
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port
	recvConn.Close()

	client, err := ableton.NewClient("127.0.0.1", mockPort, recvPort)
	if err != nil {
		t.Fatalf("newTestClient: NewClient() error: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestClient_SendRecv_Timeout(t *testing.T) {
	// Server that never responds — bind a port then close it so nothing listens there
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	silentPort := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	recvConn, _ := net.ListenPacket("udp", "127.0.0.1:0")
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port
	recvConn.Close()

	// Point client at the now-closed port (no server will respond)
	client, err := ableton.NewClient("127.0.0.1", silentPort, recvPort)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	_, err = client.SendRecv("/live/song/get/tempo", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_Send_NoError(t *testing.T) {
	mockPort, stop := startMockServer(t, "/fire-and-forget", nil)
	defer stop()

	recvConn, _ := net.ListenPacket("udp", "127.0.0.1:0")
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port
	recvConn.Close()

	client, err := ableton.NewClient("127.0.0.1", mockPort, recvPort)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	if err := client.Send("/fire-and-forget"); err != nil {
		t.Errorf("Send() error: %v", err)
	}
}

func TestClient_SendRecv_ReturnsResponse(t *testing.T) {
	mockPort, stopMock := startMockServer(t, "/live/song/get/tempo", []interface{}{float32(120.0)})
	defer stopMock()

	// Bind a free local port for the receive side
	recvConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind recv port: %v", err)
	}
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port
	recvConn.Close()

	client, err := ableton.NewClient("127.0.0.1", mockPort, recvPort)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	defer client.Close()

	msg, err := client.SendRecv("/live/song/get/tempo", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("SendRecv() error: %v", err)
	}
	if len(msg.Arguments) == 0 {
		t.Fatal("response has no arguments")
	}
}
