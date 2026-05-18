package syncer

import (
	"fmt"
	"time"

	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/state"
)

type Action int

const (
	ActionSkipped  Action = iota
	ActionUpdated
	ActionConflict
	ActionError
)

func (a Action) String() string {
	switch a {
	case ActionUpdated:
		return "updated"
	case ActionConflict:
		return "conflict"
	case ActionError:
		return "error"
	default:
		return "skipped"
	}
}

type TrackResult struct {
	Action Action
	Error  error
}

type Result struct {
	Tracks map[string]TrackResult
}

// AbletonBridge is the interface SyncTracks needs — satisfied by *ableton.Client.
type AbletonBridge interface {
	GetTrackNames(timeout time.Duration) ([]string, error)
	FindTrack(name string, timeout time.Duration) (int, error)
	CreateMIDITrack(index int, timeout time.Duration) (int, error)
	SetTrackName(index int, name string) error
	SetTrackColor(index int, color int32) error
	GetNotes(trackIdx, clipSlotIdx int, timeout time.Duration) ([]parser.Note, error)
	CreateClip(trackIdx, clipSlotIdx int, lengthBeats float64) error
	ClearNotes(trackIdx, clipSlotIdx int) error
	AddNotes(trackIdx, clipSlotIdx int, notes []parser.Note) error
	SetSongTempo(bpm float64) error
	SetSongSignatureNumerator(n int) error
	SetSongSignatureDenominator(d int) error
}

// SyncTracks compares new MIDI tracks against the last snapshot and the current
// Ableton state, then applies the appropriate action per track.
func SyncTracks(ab AbletonBridge, snap *state.SyncState, tracks []parser.Track, timeout time.Duration) (Result, error) {
	result := Result{Tracks: make(map[string]TrackResult)}

	for _, track := range tracks {
		action, err := syncOneTrack(ab, snap, track, timeout)
		result.Tracks[track.Name] = TrackResult{Action: action, Error: err}
	}

	// Apply tempo from first track that has tempo events.
	for _, track := range tracks {
		if len(track.Tempos) > 0 {
			_ = ab.SetSongTempo(track.Tempos[0].BPM)
			break
		}
	}

	// Apply time signature from first track that has time sig events.
	for _, track := range tracks {
		if len(track.TimeSigs) > 0 {
			_ = ab.SetSongSignatureNumerator(int(track.TimeSigs[0].Numerator))
			_ = ab.SetSongSignatureDenominator(int(track.TimeSigs[0].Denominator))
			break
		}
	}

	return result, nil
}

func syncOneTrack(ab AbletonBridge, snap *state.SyncState, track parser.Track, timeout time.Duration) (Action, error) {
	lastSnap, hasSnap := snap.Tracks[track.Name]

	// MuseScore delta: did the score change since last sync?
	scoreChanged := !hasSnap || state.SnapshotChanged(lastSnap, track)

	// Find existing Ableton track.
	trackIdx, err := ab.FindTrack(track.Name, timeout)
	if err != nil {
		return ActionError, fmt.Errorf("finding track %q: %w", track.Name, err)
	}

	// Ableton delta: did the Ableton clip change since last sync?
	abletonChanged := false
	if hasSnap && trackIdx >= 0 {
		currentNotes, err := ab.GetNotes(trackIdx, 0, timeout)
		if err == nil {
			abletonChanged = !state.NotesEqual(lastSnap.Notes, currentNotes)
		}
	}

	if !scoreChanged && !abletonChanged {
		return ActionSkipped, nil
	}

	if scoreChanged && abletonChanged {
		return createConflictTrack(ab, track, trackIdx, timeout)
	}

	// Only MuseScore changed — update in place.
	if trackIdx < 0 {
		trackIdx, err = ab.CreateMIDITrack(-1, timeout)
		if err != nil {
			return ActionError, fmt.Errorf("creating track %q: %w", track.Name, err)
		}
		if err := ab.SetTrackName(trackIdx, track.Name); err != nil {
			return ActionError, err
		}
	}
	if err := writeNotesToClip(ab, trackIdx, track.Notes); err != nil {
		return ActionError, err
	}
	return ActionUpdated, nil
}

func createConflictTrack(ab AbletonBridge, track parser.Track, existingIdx int, timeout time.Duration) (Action, error) {
	insertAt := existingIdx
	if existingIdx < 0 {
		insertAt = -1
	}
	newIdx, err := ab.CreateMIDITrack(insertAt, timeout)
	if err != nil {
		return ActionError, fmt.Errorf("creating conflict track: %w", err)
	}
	conflictName := track.Name + " (CHANGES)"
	if err := ab.SetTrackName(newIdx, conflictName); err != nil {
		return ActionError, err
	}
	if err := ab.SetTrackColor(newIdx, ableton.ColorRed); err != nil {
		return ActionError, err
	}
	if err := writeNotesToClip(ab, newIdx, track.Notes); err != nil {
		return ActionError, err
	}
	return ActionConflict, nil
}

func writeNotesToClip(ab AbletonBridge, trackIdx int, notes []parser.Note) error {
	var length float64 = 4 // default one bar at 4/4
	for _, n := range notes {
		if end := n.StartBeat + n.Duration; end > length {
			length = end
		}
	}
	if err := ab.CreateClip(trackIdx, 0, length); err != nil {
		// Clip likely already exists — clear it instead of creating a new one.
		if clearErr := ab.ClearNotes(trackIdx, 0); clearErr != nil {
			return fmt.Errorf("clip setup failed: create: %w, clear: %v", err, clearErr)
		}
	}
	return ab.AddNotes(trackIdx, 0, notes)
}
