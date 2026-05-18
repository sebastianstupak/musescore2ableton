package state_test

import (
	"os"
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

func TestStatePath(t *testing.T) {
	scoreDir := t.TempDir()
	scorePath := filepath.Join(scoreDir, "song.mscz")
	path := state.StatePath(scorePath)
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if dir != scoreDir {
		t.Errorf("StatePath dir = %q, want %q", dir, scoreDir)
	}
	if base != ".m2a_state.json" {
		t.Errorf("StatePath base = %q, want .m2a_state.json", base)
	}
}

func TestLoad_CorruptedJSON_ReturnsEmpty(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "state-*.json")
	f.WriteString("{not valid json}")
	f.Close()
	s, err := state.Load(f.Name())
	// Should return empty state, not error (spec: treat malformed as missing)
	if err != nil {
		t.Fatalf("Load() should not error on corrupted JSON: %v", err)
	}
	if s == nil || s.Tracks == nil {
		t.Error("Load() returned nil state or nil Tracks map")
	}
}

func TestSnapshotChanged_NotesChanged(t *testing.T) {
	snap := state.TrackSnapshot{
		Notes: []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}},
	}
	track := parser.Track{
		Name:  "Piano",
		Notes: []parser.Note{{Pitch: 62, Velocity: 100, StartBeat: 0, Duration: 1}},
	}
	if !state.SnapshotChanged(snap, track) {
		t.Error("SnapshotChanged should return true when notes differ")
	}
}

func TestSnapshotChanged_NoChange(t *testing.T) {
	note := parser.Note{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}
	snap := state.TrackSnapshot{Notes: []parser.Note{note}}
	track := parser.Track{Name: "Piano", Notes: []parser.Note{note}}
	if state.SnapshotChanged(snap, track) {
		t.Error("SnapshotChanged should return false when identical")
	}
}

func TestSnapshotChanged_TempoChanged(t *testing.T) {
	snap := state.TrackSnapshot{
		Tempos: []parser.TempoEvent{{Tick: 0, BPM: 120}},
	}
	track := parser.Track{
		Name:   "Piano",
		Tempos: []parser.TempoEvent{{Tick: 0, BPM: 140}},
	}
	if !state.SnapshotChanged(snap, track) {
		t.Error("SnapshotChanged should return true when tempo differs")
	}
}

func TestNotesEqual_EmptySlices(t *testing.T) {
	if !state.NotesEqual(nil, nil) {
		t.Error("nil slices should be equal")
	}
	if !state.NotesEqual([]parser.Note{}, []parser.Note{}) {
		t.Error("empty slices should be equal")
	}
}

func TestNotesEqual_DifferentLengths(t *testing.T) {
	a := []parser.Note{{Pitch: 60}}
	b := []parser.Note{}
	if state.NotesEqual(a, b) {
		t.Error("different-length slices should not be equal")
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	// Save should write to a tmp file then rename — verify the file exists after Save
	dir := t.TempDir()
	path := filepath.Join(dir, ".m2a_state.json")
	s := &state.SyncState{
		Version:   1,
		ScorePath: "/tmp/score.mscz",
		Tracks:    map[string]state.TrackSnapshot{},
	}
	if err := s.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("state file does not exist after Save")
	}
	// Verify no temp file was left behind
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file in dir after Save, got %d: %v", len(entries), entries)
	}
}

