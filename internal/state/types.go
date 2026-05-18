package state

import (
	"time"

	"github.com/sebastianstupak/m2a/internal/parser"
)

type TrackSnapshot struct {
	Notes    []parser.Note         `json:"notes"`
	Tempos   []parser.TempoEvent   `json:"tempos"`
	TimeSigs []parser.TimeSigEvent `json:"time_sigs"`
}

type SyncState struct {
	Version   int                      `json:"version"`
	ScorePath string                   `json:"score_path"`
	SyncedAt  time.Time                `json:"synced_at"`
	Tracks    map[string]TrackSnapshot `json:"tracks"`
}
