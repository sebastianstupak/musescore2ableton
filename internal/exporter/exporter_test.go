package exporter_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestExportMIDI_OutputPath_IsInTempDir(t *testing.T) {
	exp := exporter.NewWithRunner("musescore", func(bin string, args []string) error {
		os.WriteFile(args[1], []byte("MIDI"), 0644)
		return nil
	})
	path, err := exp.ExportMIDI("song.mscz")
	if err != nil {
		t.Fatalf("ExportMIDI() error: %v", err)
	}
	if !strings.HasPrefix(path, os.TempDir()) {
		t.Errorf("output path %q not in temp dir %q", path, os.TempDir())
	}
	if !strings.HasSuffix(path, "m2a_sync.mid") {
		t.Errorf("output path %q should end with m2a_sync.mid", path)
	}
}

func TestExportMIDI_CorrectArgs(t *testing.T) {
	var gotBin string
	var gotArgs []string
	exp := exporter.NewWithRunner("MuseScore4.exe", func(bin string, args []string) error {
		gotBin = bin
		gotArgs = args
		os.WriteFile(args[1], []byte("MIDI"), 0644)
		return nil
	})
	exp.ExportMIDI("C:/scores/song.mscz")
	if gotBin != "MuseScore4.exe" {
		t.Errorf("bin = %q, want MuseScore4.exe", gotBin)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "-o" || gotArgs[2] != "C:/scores/song.mscz" {
		t.Errorf("args = %v, want [-o <output> C:/scores/song.mscz]", gotArgs)
	}
}
