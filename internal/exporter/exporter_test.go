package exporter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/exporter"
)

func TestExportMIDI_CommandCalled(t *testing.T) {
	called := false
	stubRunner := func(bin string, args []string) error {
		called = true
		// args: ["-o", outputPath, scorePath]
		outputPath := args[1]
		return os.WriteFile(outputPath, []byte("MIDI"), 0644)
	}

	dir := t.TempDir()
	scorePath := filepath.Join(dir, "song.mscz")
	os.WriteFile(scorePath, []byte("fake"), 0644)

	exp := exporter.NewWithRunner("musescore", stubRunner)
	outPath, err := exp.ExportMIDI(scorePath)
	if err != nil {
		t.Fatalf("ExportMIDI() error: %v", err)
	}
	if !called {
		t.Error("stub runner was not called")
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if string(data) != "MIDI" {
		t.Errorf("output = %q, want MIDI", string(data))
	}
}

func TestExportMIDI_CommandFails_ReturnsError(t *testing.T) {
	stubRunner := func(bin string, args []string) error {
		return os.ErrPermission
	}
	exp := exporter.NewWithRunner("musescore", stubRunner)
	_, err := exp.ExportMIDI("song.mscz")
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}
