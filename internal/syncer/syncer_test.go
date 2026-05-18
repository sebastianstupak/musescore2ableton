package syncer_test

import (
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/state"
	"github.com/sebastianstupak/m2a/internal/syncer"
)

// stubAbleton is a fake ableton client for testing syncer logic.
type stubAbleton struct {
	trackNames         []string
	clipNotes          map[int][]parser.Note
	createdTracks      []string
	addedNotes         map[int][]parser.Note
	colors             map[int]int32
	lastTempo          float64
	lastSigNumerator   int
	lastSigDenominator int
}

func newStub(names []string) *stubAbleton {
	return &stubAbleton{
		trackNames: names,
		clipNotes:  make(map[int][]parser.Note),
		addedNotes: make(map[int][]parser.Note),
		colors:     make(map[int]int32),
	}
}

func (s *stubAbleton) GetTrackNames(timeout time.Duration) ([]string, error) {
	return s.trackNames, nil
}
func (s *stubAbleton) FindTrack(name string, timeout time.Duration) (int, error) {
	for i, n := range s.trackNames {
		if n == name {
			return i, nil
		}
	}
	return -1, nil
}
func (s *stubAbleton) CreateMIDITrack(index int, timeout time.Duration) (int, error) {
	idx := len(s.trackNames)
	s.trackNames = append(s.trackNames, "new")
	return idx, nil
}
func (s *stubAbleton) SetTrackName(index int, name string) error {
	if index < len(s.trackNames) {
		s.trackNames[index] = name
	}
	return nil
}
func (s *stubAbleton) SetTrackColor(index int, color int32) error {
	s.colors[index] = color
	return nil
}
func (s *stubAbleton) GetNotes(trackIdx, clipSlotIdx int, timeout time.Duration) ([]parser.Note, error) {
	return s.clipNotes[trackIdx], nil
}
func (s *stubAbleton) CreateClip(trackIdx, clipSlotIdx int, lengthBeats float64) error { return nil }
func (s *stubAbleton) ClearNotes(trackIdx, clipSlotIdx int) error                      { return nil }
func (s *stubAbleton) AddNotes(trackIdx, clipSlotIdx int, notes []parser.Note) error {
	s.addedNotes[trackIdx] = notes
	return nil
}
func (s *stubAbleton) SetSongTempo(bpm float64) error {
	s.lastTempo = bpm
	return nil
}
func (s *stubAbleton) SetSongSignatureNumerator(n int) error {
	s.lastSigNumerator = n
	return nil
}
func (s *stubAbleton) SetSongSignatureDenominator(d int) error {
	s.lastSigDenominator = d
	return nil
}

func TestSync_FreshState_WritesNotes(t *testing.T) {
	ab := newStub([]string{"Piano"})
	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{}}
	tracks := []parser.Track{{
		Name:  "Piano",
		Notes: []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}},
	}}

	result, err := syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("SyncTracks() error: %v", err)
	}
	if result.Tracks["Piano"].Action != syncer.ActionUpdated {
		t.Errorf("action = %v, want Updated", result.Tracks["Piano"].Action)
	}
	if len(ab.addedNotes[0]) != 1 {
		t.Errorf("notes written = %d, want 1", len(ab.addedNotes[0]))
	}
}

func TestSync_BothSidesChanged_CreatesConflictTrack(t *testing.T) {
	// Ableton has different notes than last snapshot → conflict
	ab := newStub([]string{"Piano"})
	ab.clipNotes[0] = []parser.Note{{Pitch: 62, Velocity: 80, StartBeat: 0, Duration: 1}} // different from snapshot

	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{
		"Piano": {Notes: []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}},
	}}
	tracks := []parser.Track{{
		Name:  "Piano",
		Notes: []parser.Note{{Pitch: 64, Velocity: 100, StartBeat: 0, Duration: 1}}, // new from MuseScore
	}}

	result, err := syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("SyncTracks() error: %v", err)
	}
	if result.Tracks["Piano"].Action != syncer.ActionConflict {
		t.Errorf("action = %v, want Conflict", result.Tracks["Piano"].Action)
	}
}

func TestSync_NoChange_Skips(t *testing.T) {
	notes := []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}
	ab := newStub([]string{"Piano"})
	ab.clipNotes[0] = notes

	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{
		"Piano": {Notes: notes},
	}}
	tracks := []parser.Track{{Name: "Piano", Notes: notes}}

	result, _ := syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	if result.Tracks["Piano"].Action != syncer.ActionSkipped {
		t.Errorf("action = %v, want Skipped", result.Tracks["Piano"].Action)
	}
}

func TestAction_String(t *testing.T) {
	cases := []struct {
		a    syncer.Action
		want string
	}{
		{syncer.ActionSkipped, "skipped"},
		{syncer.ActionUpdated, "updated"},
		{syncer.ActionConflict, "conflict"},
		{syncer.ActionError, "error"},
	}
	for _, tc := range cases {
		if got := tc.a.String(); got != tc.want {
			t.Errorf("Action(%d).String() = %q, want %q", tc.a, got, tc.want)
		}
	}
}

func TestSync_OnlyAbletonChanged_Updates(t *testing.T) {
	// Ableton changed but MuseScore didn't — syncer still updates (one-sided change → update)
	notes := []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}}
	abletonNotes := []parser.Note{{Pitch: 62, Velocity: 80, StartBeat: 0, Duration: 1}} // Ableton changed

	ab := newStub([]string{"Piano"})
	ab.clipNotes[0] = abletonNotes

	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{
		"Piano": {Notes: notes}, // snapshot matches score, not Ableton
	}}
	tracks := []parser.Track{{Name: "Piano", Notes: notes}} // score unchanged

	result, _ := syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	// Only Ableton changed → one-sided change → ActionUpdated (score notes written back)
	if result.Tracks["Piano"].Action != syncer.ActionUpdated {
		t.Errorf("action = %v, want Updated (one-sided Ableton change triggers update)", result.Tracks["Piano"].Action)
	}
}

func TestSync_TrackNotInAbleton_CreatesTrack(t *testing.T) {
	ab := newStub([]string{}) // no tracks in Ableton
	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{}}
	tracks := []parser.Track{{
		Name:  "Piano",
		Notes: []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}},
	}}

	result, err := syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("SyncTracks() error: %v", err)
	}
	if result.Tracks["Piano"].Action != syncer.ActionUpdated {
		t.Errorf("action = %v, want Updated", result.Tracks["Piano"].Action)
	}
}

func TestSync_WithTempoEvents_SetsTempo(t *testing.T) {
	ab := newStub([]string{"Piano"})

	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{}}
	tracks := []parser.Track{{
		Name:   "Piano",
		Notes:  []parser.Note{{Pitch: 60, Velocity: 100, StartBeat: 0, Duration: 1}},
		Tempos: []parser.TempoEvent{{Tick: 0, BPM: 140}},
	}}

	syncer.SyncTracks(ab, snap, tracks, 500*time.Millisecond)
	if ab.lastTempo != 140 {
		t.Errorf("tempo set = %v, want 140", ab.lastTempo)
	}
}

func TestSync_EmptyTracks_ReturnsEmptyResult(t *testing.T) {
	ab := newStub([]string{})
	snap := &state.SyncState{Version: 1, Tracks: map[string]state.TrackSnapshot{}}
	result, err := syncer.SyncTracks(ab, snap, []parser.Track{}, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("SyncTracks() error: %v", err)
	}
	if len(result.Tracks) != 0 {
		t.Errorf("got %d track results, want 0", len(result.Tracks))
	}
}
