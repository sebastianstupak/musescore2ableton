package ableton_test

import (
	"net"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/ableton"
)

func TestGetNotes_ParsesResponse(t *testing.T) {
	// AbletonOSC returns notes as flat: pitch, start, duration, velocity, mute
	reply := []interface{}{
		int32(60), float32(0.0), float32(1.0), int32(100), int32(0),
	}
	c := newTestClient(t, "/live/clip/get/notes", reply)
	notes, err := c.GetNotes(0, 0, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("GetNotes() error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("len(notes) = %d, want 1", len(notes))
	}
	if notes[0].Pitch != 60 {
		t.Errorf("Pitch = %d, want 60", notes[0].Pitch)
	}
	if notes[0].StartBeat != 0.0 {
		t.Errorf("StartBeat = %f, want 0.0", notes[0].StartBeat)
	}
	if notes[0].Duration != 1.0 {
		t.Errorf("Duration = %f, want 1.0", notes[0].Duration)
	}
}

func TestGetNotes_EmptyResponse(t *testing.T) {
	c := newTestClient(t, "/live/clip/get/notes", []interface{}{})
	notes, err := c.GetNotes(0, 0, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("GetNotes() error: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("len(notes) = %d, want 0", len(notes))
	}
}

func TestGetNotes_MultipleNotes(t *testing.T) {
	reply := []interface{}{
		int32(60), float32(0.0), float32(1.0), int32(100), int32(0),
		int32(64), float32(1.0), float32(0.5), int32(90), int32(0),
	}
	c := newTestClient(t, "/live/clip/get/notes", reply)
	notes, err := c.GetNotes(0, 0, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("GetNotes() error: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("len(notes) = %d, want 2", len(notes))
	}
	if notes[1].Pitch != 64 {
		t.Errorf("notes[1].Pitch = %d, want 64", notes[1].Pitch)
	}
}

func TestSetSongTempo_NoError(t *testing.T) {
	mockPort, stop := startMockServer(t, "/live/song/set/tempo", nil)
	defer stop()

	recvConn, _ := net.ListenPacket("udp", "127.0.0.1:0")
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port
	recvConn.Close()

	client, err := ableton.NewClient("127.0.0.1", mockPort, recvPort)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	if err := client.SetSongTempo(140.0); err != nil {
		t.Errorf("SetSongTempo() error: %v", err)
	}
}
