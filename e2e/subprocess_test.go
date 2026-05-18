package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/state"
	"github.com/sebastianstupak/m2a/internal/testutil"
)

var m2aBin string

func TestMain(m *testing.M) {
	// Build the binary into a temp dir
	tmp, err := os.MkdirTemp("", "m2a-e2e-*")
	if err != nil {
		log.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	m2aBin = filepath.Join(tmp, "m2a.exe")
	cmd := exec.Command("go", "build", "-o", m2aBin, "../cmd/m2a")
	cmd.Dir = "." // e2e directory
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("build failed: %v\n%s", err, out)
	}

	os.Exit(m.Run())
}

// writeConfig writes a YAML config file to the given path.
func writeConfig(t *testing.T, path string, scorePath string, museScoreBin string, oscHost string, oscPort int, recvPort int) {
	t.Helper()
	content := fmt.Sprintf(`score: %s
musescore_bin: %s
ableton_osc_host: %s
ableton_osc_port: %d
ableton_recv_port: %d
`,
		filepath.ToSlash(scorePath),
		filepath.ToSlash(museScoreBin),
		oscHost,
		oscPort,
		recvPort,
	)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

// TestSubprocess_SyncCommand_FreshState builds the real binary and runs
// "m2a sync", verifying the full pipeline end-to-end.
func TestSubprocess_SyncCommand_FreshState(t *testing.T) {
	// Build MIDI fixture
	midPath := buildMIDIFixture(t)

	// Write a fake MuseScore batch script that copies the fixture to the output path
	// The exporter calls: bin -o <outPath> <scorePath>
	// So %1 = -o, %2 = outPath, %3 = scorePath
	dir := t.TempDir()
	batPath := filepath.Join(dir, "fake_musescore.bat")
	batContent := fmt.Sprintf("@echo off\r\ncopy /Y \"%s\" %%2 >nul\r\n", filepath.FromSlash(midPath))
	if err := os.WriteFile(batPath, []byte(batContent), 0755); err != nil {
		t.Fatalf("writing bat file: %v", err)
	}

	// Start FakeServer with no tracks
	fakeServer := testutil.NewFakeServer(t)
	fakeServer.TrackNames = []string{}

	// Find a free port for the ableton recv port
	recvPort := freePort(t)

	// Create the fake .mscz file (just touch it, MuseScore is fake)
	scoreDir := t.TempDir()
	scorePath := filepath.Join(scoreDir, "test.mscz")
	if err := os.WriteFile(scorePath, []byte("fake"), 0644); err != nil {
		t.Fatalf("creating fake score: %v", err)
	}

	// Write config YAML
	configPath := filepath.Join(dir, "m2a.yml")
	writeConfig(t, configPath, scorePath, batPath, "127.0.0.1", fakeServer.Port, recvPort)

	// Run m2a sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m2aBin, "sync", "--config", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("m2a sync failed: %v\noutput: %s", err, out)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("m2a sync exit code = %d, want 0\noutput: %s", cmd.ProcessState.ExitCode(), out)
	}

	// Assert fakeServer received /live/clip/add/notes
	if !fakeServer.HasReceived("/live/clip/add/notes") {
		t.Error("expected /live/clip/add/notes to be received")
		t.Logf("received: %v", fakeServer.ReceivedAddrs())
	}

	// Assert state file was created beside the .mscz file
	statePath := state.StatePath(scorePath)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created at %s: %v", statePath, err)
	}

	// Load state file and verify contents
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	pianoSnap, ok := loaded.Tracks["Piano"]
	if !ok {
		t.Fatal("state file missing 'Piano' track")
	}
	if len(pianoSnap.Notes) != 1 {
		t.Fatalf("Piano notes count = %d, want 1", len(pianoSnap.Notes))
	}
}

// TestSubprocess_ResetCommand verifies that "m2a reset" deletes the state file.
func TestSubprocess_ResetCommand(t *testing.T) {
	dir := t.TempDir()

	// Create a fake .mscz file
	scorePath := filepath.Join(dir, "test.mscz")
	if err := os.WriteFile(scorePath, []byte("fake"), 0644); err != nil {
		t.Fatalf("creating fake score: %v", err)
	}

	// Create a .m2a_state.json file beside it
	statePath := state.StatePath(scorePath)
	snap := &state.SyncState{Version: 1, Tracks: make(map[string]state.TrackSnapshot)}
	if err := snap.Save(statePath); err != nil {
		t.Fatalf("snap.Save: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Write config YAML (ableton ports don't matter for reset)
	batPath := filepath.Join(dir, "fake_musescore.bat")
	if err := os.WriteFile(batPath, []byte("@echo off\r\n"), 0755); err != nil {
		t.Fatalf("writing bat file: %v", err)
	}
	recvPort := freePort(t)
	configPath := filepath.Join(dir, "m2a.yml")
	writeConfig(t, configPath, scorePath, batPath, "127.0.0.1", 11000, recvPort)

	// Run m2a reset
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m2aBin, "reset", "--config", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("m2a reset failed: %v\noutput: %s", err, out)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("m2a reset exit code = %d, want 0\noutput: %s", cmd.ProcessState.ExitCode(), out)
	}

	// Assert state file no longer exists
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state file should not exist after reset, but Stat returned: %v", err)
	}
}
