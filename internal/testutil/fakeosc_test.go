package testutil_test

import (
	"net"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/testutil"
)

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

func newClientAgainstFake(t *testing.T, srv *testutil.FakeServer) *ableton.Client {
	t.Helper()
	recvPort := freePort(t)
	client, err := ableton.NewClient("127.0.0.1", srv.Port, recvPort)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestFakeServer_GetTrackNames(t *testing.T) {
	srv := testutil.NewFakeServer(t)
	srv.SetTrackNames([]string{"Piano", "Violin"})

	client := newClientAgainstFake(t, srv)

	names, err := client.GetTrackNames(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("GetTrackNames: %v", err)
	}
	if len(names) != 2 || names[0] != "Piano" || names[1] != "Violin" {
		t.Errorf("names = %v, want [Piano Violin]", names)
	}
	if !srv.HasReceived("/live/song/get/track_names") {
		t.Error("server did not receive expected OSC command")
	}
}

func TestFakeServer_CreateMIDITrack(t *testing.T) {
	srv := testutil.NewFakeServer(t)
	srv.SetNextTrackIdx(3)

	client := newClientAgainstFake(t, srv)

	idx, err := client.CreateMIDITrack(-1, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("CreateMIDITrack: %v", err)
	}
	if idx != 3 {
		t.Errorf("CreateMIDITrack() = %d, want 3", idx)
	}
	// NextTrackIdx should have incremented
	if srv.GetNextTrackIdx() != 4 {
		t.Errorf("NextTrackIdx = %d, want 4", srv.GetNextTrackIdx())
	}
	if !srv.HasReceived("/live/song/create_midi_track") {
		t.Error("server did not receive create_midi_track command")
	}
}

func TestFakeServer_GetNotes(t *testing.T) {
	srv := testutil.NewFakeServer(t)
	srv.SetClipNotes(map[int][]testutil.NoteData{
		0: {
			{Pitch: 60, StartBeat: 0.0, Duration: 1.0, Velocity: 100, Mute: 0},
			{Pitch: 64, StartBeat: 1.0, Duration: 0.5, Velocity: 80, Mute: 0},
		},
	})

	client := newClientAgainstFake(t, srv)

	notes, err := client.GetNotes(0, 0, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("GetNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	if notes[0].Pitch != 60 {
		t.Errorf("note[0].Pitch = %d, want 60", notes[0].Pitch)
	}
	if notes[1].Pitch != 64 {
		t.Errorf("note[1].Pitch = %d, want 64", notes[1].Pitch)
	}
	if !srv.HasReceived("/live/clip/get/notes") {
		t.Error("server did not receive get/notes command")
	}
}

func TestFakeServer_FireAndForget_Recorded(t *testing.T) {
	srv := testutil.NewFakeServer(t)
	client := newClientAgainstFake(t, srv)

	if err := client.SetTrackName(0, "Bass"); err != nil {
		t.Fatalf("SetTrackName: %v", err)
	}
	// Give the goroutine a moment to process
	time.Sleep(20 * time.Millisecond)

	if !srv.HasReceived("/live/track/set/name") {
		t.Error("server did not record fire-and-forget command")
	}
}

func TestFakeServer_ReceivedOrder(t *testing.T) {
	srv := testutil.NewFakeServer(t)
	srv.SetTrackNames([]string{"A"})
	srv.SetNextTrackIdx(0)

	client := newClientAgainstFake(t, srv)

	_, _ = client.GetTrackNames(500 * time.Millisecond)
	_, _ = client.CreateMIDITrack(-1, 500*time.Millisecond)

	addrs := srv.ReceivedAddrs()
	if len(addrs) != 2 {
		t.Fatalf("got %d commands, want 2", len(addrs))
	}
	if addrs[0] != "/live/song/get/track_names" {
		t.Errorf("addrs[0] = %q, want /live/song/get/track_names", addrs[0])
	}
	if addrs[1] != "/live/song/create_midi_track" {
		t.Errorf("addrs[1] = %q, want /live/song/create_midi_track", addrs[1])
	}
}
