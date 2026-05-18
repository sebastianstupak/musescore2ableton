package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/sebastianstupak/m2a/internal/parser"
)

func StatePath(scorePath string) string {
	dir := filepath.Dir(scorePath)
	return filepath.Join(dir, ".m2a_state.json")
}

func Load(path string) (*SyncState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &SyncState{Version: 1, Tracks: make(map[string]TrackSnapshot)}, nil
	}
	if err != nil {
		return nil, err
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		return &SyncState{Version: 1, Tracks: make(map[string]TrackSnapshot)}, nil
	}
	if s.Tracks == nil {
		s.Tracks = make(map[string]TrackSnapshot)
	}
	return &s, nil
}

func (s *SyncState) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

const floatEps = 1e-9

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func notesApproxEqual(a, b parser.Note) bool {
	return a.Pitch == b.Pitch &&
		a.Velocity == b.Velocity &&
		abs(a.StartBeat-b.StartBeat) < floatEps &&
		abs(a.Duration-b.Duration) < floatEps
}

func NotesEqual(a, b []parser.Note) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !notesApproxEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func SnapshotChanged(snap TrackSnapshot, track parser.Track) bool {
	if !NotesEqual(snap.Notes, track.Notes) {
		return true
	}
	if len(snap.Tempos) != len(track.Tempos) {
		return true
	}
	for i := range snap.Tempos {
		if snap.Tempos[i] != track.Tempos[i] {
			return true
		}
	}
	if len(snap.TimeSigs) != len(track.TimeSigs) {
		return true
	}
	for i := range snap.TimeSigs {
		if snap.TimeSigs[i] != track.TimeSigs[i] {
			return true
		}
	}
	return false
}
