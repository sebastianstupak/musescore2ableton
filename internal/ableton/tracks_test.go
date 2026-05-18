package ableton_test

import (
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/ableton"
)

func TestGetTrackNames(t *testing.T) {
	c := newTestClient(t, "/live/song/get/track_names", []interface{}{"Piano", "Violin"})
	names, err := c.GetTrackNames(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("GetTrackNames() error: %v", err)
	}
	if len(names) != 2 || names[0] != "Piano" || names[1] != "Violin" {
		t.Errorf("names = %v, want [Piano Violin]", names)
	}
}

func TestFindTrack_Found(t *testing.T) {
	c := newTestClient(t, "/live/song/get/track_names", []interface{}{"Piano", "Violin", "Drums"})
	idx, err := c.FindTrack("Violin", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("FindTrack() error: %v", err)
	}
	if idx != 1 {
		t.Errorf("FindTrack() = %d, want 1", idx)
	}
}

func TestFindTrack_NotFound(t *testing.T) {
	c := newTestClient(t, "/live/song/get/track_names", []interface{}{"Piano", "Violin"})
	idx, err := c.FindTrack("Bass", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("FindTrack() error: %v", err)
	}
	if idx != -1 {
		t.Errorf("FindTrack() = %d, want -1", idx)
	}
}

func TestCreateMIDITrack(t *testing.T) {
	c := newTestClient(t, "/live/song/create_midi_track", []interface{}{int32(2)})
	idx, err := c.CreateMIDITrack(-1, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("CreateMIDITrack() error: %v", err)
	}
	if idx != 2 {
		t.Errorf("CreateMIDITrack() = %d, want 2", idx)
	}
}

func TestSetTrackName(t *testing.T) {
	// SetTrackName is fire-and-forget; just verify no error on send.
	c := newTestClient(t, "/live/track/set/name", nil)
	if err := c.SetTrackName(0, "MyTrack"); err != nil {
		t.Fatalf("SetTrackName() error: %v", err)
	}
}

func TestSetTrackColor(t *testing.T) {
	c := newTestClient(t, "/live/track/set/color", nil)
	if err := c.SetTrackColor(0, ableton.ColorRed); err != nil {
		t.Fatalf("SetTrackColor() error: %v", err)
	}
}
