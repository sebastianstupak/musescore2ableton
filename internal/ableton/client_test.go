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
