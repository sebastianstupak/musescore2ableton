package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"

	"github.com/sebastianstupak/m2a/internal/parser"
)

func buildMIDI(t *testing.T) string {
	t.Helper()

	s := smf.New()
	s.TimeFormat = smf.MetricTicks(480)

	// Conductor track: tempo + time sig
	var cond smf.Track
	cond.Add(0, smf.MetaMeter(4, 4))
	cond.Add(0, smf.MetaTempo(120))
	cond.Close(0)
	s.Add(cond)

	// Instrument track: Piano, C4 quarter note
	var inst smf.Track
	inst.Add(0, smf.MetaTrackSequenceName("Piano"))
	inst.Add(0, midi.NoteOn(0, 60, 100))
	inst.Add(480, midi.NoteOff(0, 60))
	inst.Close(0)
	s.Add(inst)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mid")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := s.WriteTo(f); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParse_ReturnsTrackWithName(t *testing.T) {
	path := buildMIDI(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	if tracks[0].Name != "Piano" {
		t.Errorf("tracks[0].Name = %q, want Piano", tracks[0].Name)
	}
}

func TestParse_ReturnsNote(t *testing.T) {
	path := buildMIDI(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	notes := tracks[0].Notes
	if len(notes) != 1 {
		t.Fatalf("len(notes) = %d, want 1", len(notes))
	}
	n := notes[0]
	if n.Pitch != 60 {
		t.Errorf("Pitch = %d, want 60", n.Pitch)
	}
	if n.StartBeat != 0 {
		t.Errorf("StartBeat = %f, want 0", n.StartBeat)
	}
	if n.Duration != 1.0 {
		t.Errorf("Duration = %f, want 1.0", n.Duration)
	}
}

func TestParse_ConductorTempoAppliedToTrack(t *testing.T) {
	path := buildMIDI(t)
	tracks, _ := parser.Parse(path)
	if len(tracks[0].Tempos) != 1 {
		t.Fatalf("len(Tempos) = %d, want 1", len(tracks[0].Tempos))
	}
	if tracks[0].Tempos[0].BPM != 120 {
		t.Errorf("BPM = %f, want 120", tracks[0].Tempos[0].BPM)
	}
}
