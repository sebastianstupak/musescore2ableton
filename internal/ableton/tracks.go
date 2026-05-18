package ableton

import (
	"fmt"
	"time"
)

const ColorRed = int32(16711680)

// GetTrackNames returns the ordered list of track names in the Live set.
func (c *Client) GetTrackNames(timeout time.Duration) ([]string, error) {
	msg, err := c.SendRecv("/live/song/get/track_names", timeout)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(msg.Arguments))
	for _, arg := range msg.Arguments {
		s, ok := arg.(string)
		if !ok {
			continue
		}
		names = append(names, s)
	}
	return names, nil
}

// FindTrack returns the zero-based index of a track by name, or -1 if not found.
func (c *Client) FindTrack(name string, timeout time.Duration) (int, error) {
	names, err := c.GetTrackNames(timeout)
	if err != nil {
		return -1, err
	}
	for i, n := range names {
		if n == name {
			return i, nil
		}
	}
	return -1, nil
}

// CreateMIDITrack inserts a new MIDI track at the given index (-1 = end).
// Returns the index of the created track.
func (c *Client) CreateMIDITrack(index int, timeout time.Duration) (int, error) {
	msg, err := c.SendRecv("/live/song/create_midi_track", timeout, int32(index))
	if err != nil {
		return -1, err
	}
	if len(msg.Arguments) == 0 {
		return -1, fmt.Errorf("ableton: create_midi_track returned no index")
	}
	idx, ok := msg.Arguments[0].(int32)
	if !ok {
		return -1, fmt.Errorf("ableton: unexpected type for track index")
	}
	return int(idx), nil
}

// SetTrackName renames the track at the given index.
func (c *Client) SetTrackName(index int, name string) error {
	return c.Send("/live/track/set/name", int32(index), name)
}

// SetTrackColor sets the track color using a packed RGB integer.
func (c *Client) SetTrackColor(index int, color int32) error {
	return c.Send("/live/track/set/color", int32(index), color)
}
