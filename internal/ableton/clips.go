package ableton

import (
	"time"

	"github.com/sebastianstupak/m2a/internal/parser"
)

// GetNotes returns all notes in the clip at the given track and clip-slot index.
func (c *Client) GetNotes(trackIdx, clipSlotIdx int, timeout time.Duration) ([]parser.Note, error) {
	msg, err := c.SendRecv("/live/clip/get/notes", timeout, int32(trackIdx), int32(clipSlotIdx))
	if err != nil {
		return nil, err
	}
	// Response: flat list of [pitch, start, duration, velocity, mute, ...]
	args := msg.Arguments
	notes := make([]parser.Note, 0, len(args)/5)
	for i := 0; i+4 < len(args); i += 5 {
		pitch, _ := toInt32(args[i])
		start, _ := toFloat64(args[i+1])
		dur, _ := toFloat64(args[i+2])
		vel, _ := toInt32(args[i+3])
		notes = append(notes, parser.Note{
			Pitch:     uint8(pitch),
			Velocity:  uint8(vel),
			StartBeat: start,
			Duration:  dur,
		})
	}
	return notes, nil
}

// CreateClip creates an empty MIDI clip at trackIdx / clipSlotIdx with length in beats.
func (c *Client) CreateClip(trackIdx, clipSlotIdx int, lengthBeats float64) error {
	return c.Send("/live/clip_slot/create_clip", int32(trackIdx), int32(clipSlotIdx), float32(lengthBeats))
}

// ClearNotes removes all notes from the clip.
func (c *Client) ClearNotes(trackIdx, clipSlotIdx int) error {
	return c.Send("/live/clip/remove/notes", int32(trackIdx), int32(clipSlotIdx),
		int32(0), int32(127), float32(0), float32(9999))
}

// AddNotes writes notes into the clip as a flat OSC argument list.
func (c *Client) AddNotes(trackIdx, clipSlotIdx int, notes []parser.Note) error {
	if len(notes) == 0 {
		return nil
	}
	args := make([]interface{}, 0, 2+len(notes)*5)
	args = append(args, int32(trackIdx), int32(clipSlotIdx))
	for _, n := range notes {
		args = append(args,
			int32(n.Pitch),
			float32(n.StartBeat),
			float32(n.Duration),
			int32(n.Velocity),
			int32(0), // mute = false
		)
	}
	return c.Send("/live/clip/add/notes", args...)
}

// SetSongTempo sets the Live set's master tempo.
func (c *Client) SetSongTempo(bpm float64) error {
	return c.Send("/live/song/set/tempo", float32(bpm))
}

// SetSongSignatureNumerator sets the time signature numerator.
func (c *Client) SetSongSignatureNumerator(n int) error {
	return c.Send("/live/song/set/signature_numerator", int32(n))
}

// SetSongSignatureDenominator sets the time signature denominator.
func (c *Client) SetSongSignatureDenominator(d int) error {
	return c.Send("/live/song/set/signature_denominator", int32(d))
}

func toInt32(v interface{}) (int32, bool) {
	switch t := v.(type) {
	case int32:
		return t, true
	case int:
		return int32(t), true
	}
	return 0, false
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float32:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}
