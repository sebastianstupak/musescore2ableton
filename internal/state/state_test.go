package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/state"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".m2a_state.json")

	original := &state.SyncState{
		Version:   1,
		ScorePath: "song.mscz",
		SyncedAt:  time.Now().UTC().Truncate(time.Second),
		Tracks: map[string]state.TrackSnapshot{
			"Piano": {
				Notes: []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}},
			},
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.ScorePath != original.ScorePath {
		t.Errorf("ScorePath = %q, want %q", loaded.ScorePath, original.ScorePath)
	}
	if len(loaded.Tracks["Piano"].Notes) != 1 {
		t.Errorf("notes count = %d, want 1", len(loaded.Tracks["Piano"].Notes))
	}
}

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	s, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load() error on missing file: %v", err)
	}
	if s.Tracks == nil {
		t.Error("Tracks map is nil, want empty map")
	}
}

func TestNotesEqual_SameNotes_True(t *testing.T) {
	a := []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}
	b := []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}
	if !state.NotesEqual(a, b) {
		t.Error("NotesEqual returned false for identical notes")
	}
}

func TestNotesEqual_DifferentPitch_False(t *testing.T) {
	a := []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}
	b := []parser.Note{{Pitch: 61, Velocity: 100, StartBeat: 0, Duration: 1}}
	if state.NotesEqual(a, b) {
		t.Error("NotesEqual returned true for different pitches")
	}
}
