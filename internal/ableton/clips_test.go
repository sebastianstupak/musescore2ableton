package ableton_test

import (
	"testing"
	"time"
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
