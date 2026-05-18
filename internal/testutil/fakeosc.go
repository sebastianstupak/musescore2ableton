package testutil

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hypebeast/go-osc/osc"
)

// ReceivedCmd records one OSC command received by the fake server.
type ReceivedCmd struct {
	Addr string
	Args []interface{}
}

// NoteData is the flat AbletonOSC note format.
type NoteData struct {
	Pitch     int32
	StartBeat float32
	Duration  float32
	Velocity  int32
	Mute      int32
}

// FakeServer is a stateful UDP OSC server that simulates AbletonOSC for tests.
type FakeServer struct {
	Port int

	conn *net.UDPConn
	mu   sync.Mutex

	received []ReceivedCmd

	trackNames   []string
	clipNotes    map[int][]NoteData
	nextTrackIdx int
}

// NewFakeServer starts a fake AbletonOSC server bound to a free UDP port.
// It registers a t.Cleanup to stop the server.
func NewFakeServer(t *testing.T) *FakeServer {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fakeosc: listen: %v", err)
	}
	s := &FakeServer{
		Port:      conn.LocalAddr().(*net.UDPAddr).Port,
		conn:      conn.(*net.UDPConn),
		clipNotes: make(map[int][]NoteData),
	}
	go s.serve()
	t.Cleanup(s.Stop)
	return s
}

// Stop shuts down the server.
func (s *FakeServer) Stop() {
	s.conn.Close()
}

// SetTrackNames sets the track names returned by the fake server.
func (s *FakeServer) SetTrackNames(names []string) {
	s.mu.Lock()
	s.trackNames = names
	s.mu.Unlock()
}

// SetClipNotes sets the clip notes returned for a given track index.
func (s *FakeServer) SetClipNotes(notes map[int][]NoteData) {
	s.mu.Lock()
	s.clipNotes = notes
	s.mu.Unlock()
}

// SetNextTrackIdx sets the next track index the server will return for create_midi_track.
func (s *FakeServer) SetNextTrackIdx(idx int) {
	s.mu.Lock()
	s.nextTrackIdx = idx
	s.mu.Unlock()
}

// GetNextTrackIdx returns the current next-track-index counter.
func (s *FakeServer) GetNextTrackIdx() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextTrackIdx
}

// Received returns a copy of all commands received.
func (s *FakeServer) Received() []ReceivedCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ReceivedCmd, len(s.received))
	copy(out, s.received)
	return out
}

// ReceivedAddrs returns the OSC address of each received command.
func (s *FakeServer) ReceivedAddrs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	addrs := make([]string, len(s.received))
	for i, c := range s.received {
		addrs[i] = c.Addr
	}
	return addrs
}

// HasReceived returns true if addr was received at least once.
func (s *FakeServer) HasReceived(addr string) bool {
	for _, a := range s.ReceivedAddrs() {
		if a == addr {
			return true
		}
	}
	return false
}

// WaitForReceived polls until addr is received or timeout elapses.
func (s *FakeServer) WaitForReceived(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.HasReceived(addr) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func (s *FakeServer) serve() {
	buf := make([]byte, 65535)
	for {
		n, sender, err := s.conn.ReadFrom(buf)
		if err != nil {
			return
		}
		pkt, err := osc.ParsePacket(string(buf[:n]))
		if err != nil {
			continue
		}
		msg, ok := pkt.(*osc.Message)
		if !ok {
			continue
		}

		args := make([]interface{}, len(msg.Arguments))
		copy(args, msg.Arguments)
		s.mu.Lock()
		s.received = append(s.received, ReceivedCmd{Addr: msg.Address, Args: args})
		s.mu.Unlock()

		reply := s.buildReply(msg)
		if reply == nil {
			continue
		}
		data, err := reply.MarshalBinary()
		if err != nil {
			continue
		}
		s.conn.WriteTo(data, sender) //nolint:errcheck
	}
}

func (s *FakeServer) buildReply(msg *osc.Message) *osc.Message {
	switch msg.Address {
	case "/live/song/get/track_names":
		reply := osc.NewMessage(msg.Address)
		s.mu.Lock()
		for _, name := range s.trackNames {
			reply.Append(name)
		}
		s.mu.Unlock()
		return reply

	case "/live/clip/get/notes":
		reply := osc.NewMessage(msg.Address)
		var trackIdx int32
		if len(msg.Arguments) > 0 {
			if v, ok := msg.Arguments[0].(int32); ok {
				trackIdx = v
			}
		}
		s.mu.Lock()
		notes := s.clipNotes[int(trackIdx)]
		s.mu.Unlock()
		for _, n := range notes {
			reply.Append(n.Pitch)
			reply.Append(n.StartBeat)
			reply.Append(n.Duration)
			reply.Append(n.Velocity)
			reply.Append(n.Mute)
		}
		return reply

	case "/live/song/create_midi_track":
		s.mu.Lock()
		idx := int32(s.nextTrackIdx)
		s.nextTrackIdx++
		s.mu.Unlock()
		reply := osc.NewMessage(msg.Address)
		reply.Append(idx)
		return reply

	default:
		return nil
	}
}
