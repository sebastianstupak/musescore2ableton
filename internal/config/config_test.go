package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/config"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "m2a-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoad_MinimalConfig(t *testing.T) {
	path := writeYAML(t, `score: "C:/scores/song.mscz"`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Score != "C:/scores/song.mscz" {
		t.Errorf("Score = %q, want %q", cfg.Score, "C:/scores/song.mscz")
	}
	// Defaults applied
	if cfg.AbletonOSCHost != "127.0.0.1" {
		t.Errorf("AbletonOSCHost = %q, want 127.0.0.1", cfg.AbletonOSCHost)
	}
	if cfg.AbletonOSCPort != 11000 {
		t.Errorf("AbletonOSCPort = %d, want 11000", cfg.AbletonOSCPort)
	}
	if cfg.DebounceMS != 500 {
		t.Errorf("DebounceMS = %d, want 500", cfg.DebounceMS)
	}
	if cfg.AbletonRecvPort != 11001 {
		t.Errorf("AbletonRecvPort = %d, want 11001", cfg.AbletonRecvPort)
	}
}

func TestLoad_MissingScore_ReturnsError(t *testing.T) {
	path := writeYAML(t, `ableton_osc_port: 11000`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing score, got nil")
	}
}

func TestLoad_MissingFile_ReturnsError(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	path := writeYAML(t, `
score: "C:/scores/song.mscz"
musescore_bin: "C:/custom/MuseScore.exe"
ableton_osc_host: "192.168.1.1"
ableton_osc_port: 9000
ableton_recv_port: 9001
debounce_ms: 200
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MuseScoreBin != "C:/custom/MuseScore.exe" {
		t.Errorf("MuseScoreBin = %q", cfg.MuseScoreBin)
	}
	if cfg.AbletonOSCHost != "192.168.1.1" {
		t.Errorf("AbletonOSCHost = %q", cfg.AbletonOSCHost)
	}
	if cfg.AbletonOSCPort != 9000 {
		t.Errorf("AbletonOSCPort = %d", cfg.AbletonOSCPort)
	}
	if cfg.AbletonRecvPort != 9001 {
		t.Errorf("AbletonRecvPort = %d", cfg.AbletonRecvPort)
	}
	if cfg.DebounceMS != 200 {
		t.Errorf("DebounceMS = %d", cfg.DebounceMS)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeYAML(t, `score: [not a string`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EmptyScore_ReturnsError(t *testing.T) {
	path := writeYAML(t, `score: ""`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty score path")
	}
}
