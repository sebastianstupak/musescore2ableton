package parser

// Note represents a single MIDI note in beat-based time.
type Note struct {
	Pitch     uint8
	Velocity  uint8
	StartBeat float64 // beats from clip start
	Duration  float64 // beats
}

// TempoEvent is a BPM change at a given absolute tick.
type TempoEvent struct {
	Tick uint32
	BPM  float64
}

// TimeSigEvent is a time signature change at a given absolute tick.
type TimeSigEvent struct {
	Tick        uint32
	Numerator   uint8
	Denominator uint8
}

// Track holds all parsed data for one instrument/MIDI track.
type Track struct {
	Name     string
	Notes    []Note
	Tempos   []TempoEvent
	TimeSigs []TimeSigEvent
}
