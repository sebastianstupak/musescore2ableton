package parser

import (
	"fmt"

	"gitlab.com/gomidi/midi/v2/smf"
)

func Parse(path string) ([]Track, error) {
	s, err := smf.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading midi: %w", err)
	}

	tpb, ok := s.TimeFormat.(smf.MetricTicks)
	if !ok {
		return nil, fmt.Errorf("unsupported MIDI time format (only metric ticks supported)")
	}
	ticksPerBeat := float64(tpb.Ticks4th())

	var globalTempos []TempoEvent
	var globalTimeSigs []TimeSigEvent
	if len(s.Tracks) > 0 {
		globalTempos, globalTimeSigs = parseConductor(s.Tracks[0], ticksPerBeat)
	}

	var tracks []Track
	for i := 1; i < len(s.Tracks); i++ {
		t := parseInstrumentTrack(s.Tracks[i], ticksPerBeat)
		t.Tempos = globalTempos
		t.TimeSigs = globalTimeSigs
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func parseConductor(track smf.Track, ticksPerBeat float64) ([]TempoEvent, []TimeSigEvent) {
	var tempos []TempoEvent
	var timeSigs []TimeSigEvent
	var abs uint32
	for _, ev := range track {
		abs += ev.Delta
		var bpm float64
		if ev.Message.GetMetaTempo(&bpm) {
			tempos = append(tempos, TempoEvent{Tick: abs, BPM: bpm})
		}
		var num, denom uint8
		if ev.Message.GetMetaMeter(&num, &denom) {
			timeSigs = append(timeSigs, TimeSigEvent{Tick: abs, Numerator: num, Denominator: denom})
		}
	}
	return tempos, timeSigs
}

func parseInstrumentTrack(track smf.Track, ticksPerBeat float64) Track {
	var t Track
	pending := make(map[uint8]struct {
		tick     uint32
		velocity uint8
	})
	var abs uint32

	for _, ev := range track {
		abs += ev.Delta

		var name string
		if ev.Message.GetMetaTrackName(&name) {
			t.Name = name
			continue
		}

		var ch, key, vel uint8
		if ev.Message.GetNoteOn(&ch, &key, &vel) && vel > 0 {
			pending[key] = struct {
				tick     uint32
				velocity uint8
			}{tick: abs, velocity: vel}
			continue
		}

		isNoteOff := ev.Message.GetNoteOff(&ch, &key, &vel)
		isNoteOnZeroVel := ev.Message.GetNoteOn(&ch, &key, &vel) && vel == 0
		if isNoteOff || isNoteOnZeroVel {
			if start, ok := pending[key]; ok {
				delete(pending, key)
				t.Notes = append(t.Notes, Note{
					Pitch:     key,
					Velocity:  start.velocity,
					StartBeat: float64(start.tick) / ticksPerBeat,
					Duration:  float64(abs-start.tick) / ticksPerBeat,
				})
			}
		}
	}
	return t
}

