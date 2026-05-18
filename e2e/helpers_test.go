package e2e_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"

	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/testutil"
)

// buildMIDIFixture creates a MIDI file with a single "Piano" track containing
// one C4 quarter note at 120 BPM. Returns the path to the MIDI file.
func buildMIDIFixture(t *testing.T) string {
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

// buildMIDIFixtureWithPitch creates a MIDI file with a single "Piano" track
// containing one quarter note at the given pitch. Returns the path to the MIDI file.
func buildMIDIFixtureWithPitch(t *testing.T, pitch uint8) string {
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
	inst.Add(0, midi.NoteOn(0, pitch, 100))
	inst.Add(480, midi.NoteOff(0, pitch))
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

// freePort finds a free UDP port by binding briefly and then closing.
func freePort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

// newTestClient creates an ableton.Client pointed at fakeServer with a free recv port.
func newTestClient(t *testing.T, fakeServer *testutil.FakeServer) *ableton.Client {
	t.Helper()
	recvPort := freePort(t)
	client, err := ableton.NewClient("127.0.0.1", fakeServer.Port, recvPort)
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}
