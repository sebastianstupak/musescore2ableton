package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/state"
	"github.com/sebastianstupak/m2a/internal/syncer"
	"github.com/sebastianstupak/m2a/internal/testutil"
)

// TestIntegration_FreshSync_CreatesTrackAndWritesNotes verifies that syncing
// a new MIDI file with empty Ableton state creates a track and writes notes.
func TestIntegration_FreshSync_CreatesTrackAndWritesNotes(t *testing.T) {
	t.Parallel()

	fakeServer := testutil.NewFakeServer(t)
	fakeServer.TrackNames = []string{}

	midPath := buildMIDIFixture(t)
	tracks, err := parser.Parse(midPath)
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}

	snap := &state.SyncState{
		Version: 1,
		Tracks:  make(map[string]state.TrackSnapshot),
	}

	client := newTestClient(t, fakeServer)

	result, err := syncer.SyncTracks(client, snap, tracks, 2*time.Second)
	if err != nil {
		t.Fatalf("SyncTracks: %v", err)
	}

	pianoResult, ok := result.Tracks["Piano"]
	if !ok {
		t.Fatal("result.Tracks missing 'Piano'")
	}
	if pianoResult.Action != syncer.ActionUpdated {
		t.Errorf("Piano action = %v, want ActionUpdated", pianoResult.Action)
	}

	if !fakeServer.HasReceived("/live/song/create_midi_track") {
		t.Error("expected /live/song/create_midi_track to be received")
	}
	if !fakeServer.HasReceived("/live/clip/add/notes") {
		t.Error("expected /live/clip/add/notes to be received")
	}
	if !fakeServer.HasReceived("/live/song/set/tempo") {
		t.Error("expected /live/song/set/tempo to be received")
	}
}

// TestIntegration_SecondSync_NoChange_Skips verifies that when both Ableton
// and MuseScore have the same notes as the last snapshot, the track is skipped.
func TestIntegration_SecondSync_NoChange_Skips(t *testing.T) {
	t.Parallel()

	fakeServer := testutil.NewFakeServer(t)
	fakeServer.TrackNames = []string{"Piano"}
	// ClipNotes[0] = C4 quarter note, matching the snapshot
	fakeServer.ClipNotes[0] = []testutil.NoteData{
		{Pitch: 60, StartBeat: 0, Duration: 1.0, Velocity: 100, Mute: 0},
	}

	midPath := buildMIDIFixture(t)
	tracks, err := parser.Parse(midPath)
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}

	// Snapshot matches the parsed notes exactly (including tempos and time sigs)
	snap := &state.SyncState{
		Version: 1,
		Tracks: map[string]state.TrackSnapshot{
			"Piano": {
				Notes: []parser.Note{
					{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1.0},
				},
				Tempos:   tracks[0].Tempos,
				TimeSigs: tracks[0].TimeSigs,
			},
		},
	}

	client := newTestClient(t, fakeServer)

	result, err := syncer.SyncTracks(client, snap, tracks, 2*time.Second)
	if err != nil {
		t.Fatalf("SyncTracks: %v", err)
	}

	pianoResult, ok := result.Tracks["Piano"]
	if !ok {
		t.Fatal("result.Tracks missing 'Piano'")
	}
	if pianoResult.Action != syncer.ActionSkipped {
		t.Errorf("Piano action = %v, want ActionSkipped", pianoResult.Action)
	}

	if fakeServer.HasReceived("/live/clip/add/notes") {
		t.Error("did not expect /live/clip/add/notes to be received (no change)")
	}
}

// TestIntegration_ConflictDetection_CreatesConflictTrack verifies that when both
// Ableton and MuseScore have changed since the last snapshot, a conflict track is created.
func TestIntegration_ConflictDetection_CreatesConflictTrack(t *testing.T) {
	t.Parallel()

	fakeServer := testutil.NewFakeServer(t)
	fakeServer.TrackNames = []string{"Piano"}
	// Ableton has pitch 62 (D4) — different from snapshot pitch 60
	fakeServer.ClipNotes[0] = []testutil.NoteData{
		{Pitch: 62, StartBeat: 0, Duration: 1.0, Velocity: 100, Mute: 0},
	}

	// MuseScore has pitch 64 (E4) — also different from snapshot pitch 60
	midPath := buildMIDIFixtureWithPitch(t, 64)
	tracks, err := parser.Parse(midPath)
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}

	// Snapshot has pitch 60 (C4)
	snap := &state.SyncState{
		Version: 1,
		Tracks: map[string]state.TrackSnapshot{
			"Piano": {
				Notes: []parser.Note{
					{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1.0},
				},
			},
		},
	}

	client := newTestClient(t, fakeServer)

	result, err := syncer.SyncTracks(client, snap, tracks, 2*time.Second)
	if err != nil {
		t.Fatalf("SyncTracks: %v", err)
	}

	pianoResult, ok := result.Tracks["Piano"]
	if !ok {
		t.Fatal("result.Tracks missing 'Piano'")
	}
	if pianoResult.Action != syncer.ActionConflict {
		t.Errorf("Piano action = %v, want ActionConflict", pianoResult.Action)
	}

	if !fakeServer.HasReceived("/live/song/create_midi_track") {
		t.Error("expected /live/song/create_midi_track to be received (conflict track)")
	}
	if !fakeServer.HasReceived("/live/track/set/name") {
		t.Error("expected /live/track/set/name to be received (conflict track naming)")
	}
	if !fakeServer.HasReceived("/live/track/set/color") {
		t.Error("expected /live/track/set/color to be received (colored red)")
	}
}

// TestIntegration_StateFilePersistence verifies that after a sync the state
// can be saved to disk and loaded back with correct data.
func TestIntegration_StateFilePersistence(t *testing.T) {
	t.Parallel()

	fakeServer := testutil.NewFakeServer(t)
	fakeServer.TrackNames = []string{}

	midPath := buildMIDIFixture(t)
	tracks, err := parser.Parse(midPath)
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}

	snap := &state.SyncState{
		Version: 1,
		Tracks:  make(map[string]state.TrackSnapshot),
	}

	client := newTestClient(t, fakeServer)

	result, err := syncer.SyncTracks(client, snap, tracks, 2*time.Second)
	if err != nil {
		t.Fatalf("SyncTracks: %v", err)
	}

	// Populate snapshot with synced track data (as the real sync command does)
	for _, track := range tracks {
		if r := result.Tracks[track.Name]; r.Action != syncer.ActionError {
			snap.Tracks[track.Name] = state.TrackSnapshot{
				Notes:    track.Notes,
				Tempos:   track.Tempos,
				TimeSigs: track.TimeSigs,
			}
		}
	}

	// Save state to a temp file
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".m2a_state.json")
	if err := snap.Save(statePath); err != nil {
		t.Fatalf("snap.Save: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Load it back
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	pianoSnap, ok := loaded.Tracks["Piano"]
	if !ok {
		t.Fatal("loaded state missing 'Piano' track")
	}
	if len(pianoSnap.Notes) != 1 {
		t.Fatalf("loaded Piano notes count = %d, want 1", len(pianoSnap.Notes))
	}
	if pianoSnap.Notes[0].Pitch != 60 {
		t.Errorf("loaded Piano note pitch = %d, want 60", pianoSnap.Notes[0].Pitch)
	}
}
