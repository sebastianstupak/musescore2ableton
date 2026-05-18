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

// buildMIDIMultiNotes creates a MIDI with two sequential notes on the same track.
func buildMIDIMultiNotes(t *testing.T) string {
	t.Helper()
	s := smf.New()
	s.TimeFormat = smf.MetricTicks(480)

	var cond smf.Track
	cond.Add(0, smf.MetaMeter(4, 4))
	cond.Add(0, smf.MetaTempo(120))
	cond.Close(0)
	s.Add(cond)

	var inst smf.Track
	inst.Add(0, smf.MetaTrackSequenceName("Piano"))
	// First note: C4 quarter note (ticks 0-480)
	inst.Add(0, midi.NoteOn(0, 60, 100))
	inst.Add(480, midi.NoteOff(0, 60))
	// Second note: D4 quarter note (ticks 480-960)
	inst.Add(0, midi.NoteOn(0, 62, 90))
	inst.Add(480, midi.NoteOff(0, 62))
	inst.Close(0)
	s.Add(inst)

	dir := t.TempDir()
	path := filepath.Join(dir, "multi_notes.mid")
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

// buildMIDIMultiTempo creates a MIDI with two tempo events in the conductor track.
func buildMIDIMultiTempo(t *testing.T) string {
	t.Helper()
	s := smf.New()
	s.TimeFormat = smf.MetricTicks(480)

	var cond smf.Track
	cond.Add(0, smf.MetaTempo(120))
	// Second tempo at tick 480 (delta 480 from previous)
	cond.Add(480, smf.MetaTempo(140))
	cond.Close(0)
	s.Add(cond)

	var inst smf.Track
	inst.Add(0, smf.MetaTrackSequenceName("Violin"))
	inst.Add(0, midi.NoteOn(0, 64, 80))
	inst.Add(480, midi.NoteOff(0, 64))
	inst.Close(0)
	s.Add(inst)

	dir := t.TempDir()
	path := filepath.Join(dir, "multi_tempo.mid")
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

// buildMIDITimeSig creates a MIDI with a specific time signature event.
func buildMIDITimeSig(t *testing.T) string {
	t.Helper()
	s := smf.New()
	s.TimeFormat = smf.MetricTicks(480)

	var cond smf.Track
	cond.Add(0, smf.MetaMeter(3, 4))
	cond.Add(0, smf.MetaTempo(100))
	cond.Close(0)
	s.Add(cond)

	var inst smf.Track
	inst.Add(0, smf.MetaTrackSequenceName("Guitar"))
	inst.Add(0, midi.NoteOn(0, 55, 70))
	inst.Add(480, midi.NoteOff(0, 55))
	inst.Close(0)
	s.Add(inst)

	dir := t.TempDir()
	path := filepath.Join(dir, "timesig.mid")
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

// buildMIDIMultiTrack creates a MIDI with two instrument tracks.
func buildMIDIMultiTrack(t *testing.T) string {
	t.Helper()
	s := smf.New()
	s.TimeFormat = smf.MetricTicks(480)

	var cond smf.Track
	cond.Add(0, smf.MetaMeter(4, 4))
	cond.Add(0, smf.MetaTempo(120))
	cond.Close(0)
	s.Add(cond)

	// First instrument track
	var inst1 smf.Track
	inst1.Add(0, smf.MetaTrackSequenceName("Piano"))
	inst1.Add(0, midi.NoteOn(0, 60, 100))
	inst1.Add(480, midi.NoteOff(0, 60))
	inst1.Close(0)
	s.Add(inst1)

	// Second instrument track
	var inst2 smf.Track
	inst2.Add(0, smf.MetaTrackSequenceName("Bass"))
	inst2.Add(0, midi.NoteOn(1, 36, 90))
	inst2.Add(480, midi.NoteOff(1, 36))
	inst2.Close(0)
	s.Add(inst2)

	dir := t.TempDir()
	path := filepath.Join(dir, "multi_track.mid")
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

func TestParse_MultipleNotes(t *testing.T) {
	path := buildMIDIMultiNotes(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	notes := tracks[0].Notes
	if len(notes) != 2 {
		t.Fatalf("len(notes) = %d, want 2", len(notes))
	}
	if notes[0].Pitch != 60 {
		t.Errorf("notes[0].Pitch = %d, want 60", notes[0].Pitch)
	}
	if notes[1].Pitch != 62 {
		t.Errorf("notes[1].Pitch = %d, want 62", notes[1].Pitch)
	}
	if notes[1].StartBeat != 1.0 {
		t.Errorf("notes[1].StartBeat = %f, want 1.0", notes[1].StartBeat)
	}
}

func TestParse_TimeSigParsed(t *testing.T) {
	path := buildMIDITimeSig(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("no tracks returned")
	}
	timeSigs := tracks[0].TimeSigs
	if len(timeSigs) != 1 {
		t.Fatalf("len(TimeSigs) = %d, want 1", len(timeSigs))
	}
	if timeSigs[0].Numerator != 3 {
		t.Errorf("Numerator = %d, want 3", timeSigs[0].Numerator)
	}
	if timeSigs[0].Denominator != 4 {
		t.Errorf("Denominator = %d, want 4", timeSigs[0].Denominator)
	}
}

func TestParse_MultipleTempoEvents(t *testing.T) {
	path := buildMIDIMultiTempo(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("no tracks returned")
	}
	tempos := tracks[0].Tempos
	if len(tempos) != 2 {
		t.Fatalf("len(Tempos) = %d, want 2", len(tempos))
	}
	if tempos[0].BPM != 120 {
		t.Errorf("tempos[0].BPM = %f, want 120", tempos[0].BPM)
	}
	if tempos[1].BPM < 139.99 || tempos[1].BPM > 140.01 {
		t.Errorf("tempos[1].BPM = %f, want ~140", tempos[1].BPM)
	}
}

func TestParse_MultipleInstrumentTracks(t *testing.T) {
	path := buildMIDIMultiTrack(t)
	tracks, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	names := map[string]bool{}
	for _, tr := range tracks {
		names[tr.Name] = true
	}
	if !names["Piano"] {
		t.Error("expected track named Piano")
	}
	if !names["Bass"] {
		t.Error("expected track named Bass")
	}
}

func TestParse_NonexistentFile_ReturnsError(t *testing.T) {
	_, err := parser.Parse(filepath.Join(t.TempDir(), "nonexistent.mid"))
	if err == nil {
		t.Fatal("expected error for nonexistent MIDI file, got nil")
	}
}
