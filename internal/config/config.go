package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Score           string `yaml:"score"`
	MuseScoreBin    string `yaml:"musescore_bin"`
	AbletonOSCHost  string `yaml:"ableton_osc_host"`
	AbletonOSCPort  int    `yaml:"ableton_osc_port"`
	AbletonRecvPort int    `yaml:"ableton_recv_port"`
	DebounceMS      int    `yaml:"debounce_ms"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	applyDefaults(&cfg)
	return &cfg, cfg.Validate()
}

func applyDefaults(cfg *Config) {
	if cfg.AbletonOSCHost == "" {
		cfg.AbletonOSCHost = "127.0.0.1"
	}
	if cfg.AbletonOSCPort == 0 {
		cfg.AbletonOSCPort = 11000
	}
	if cfg.AbletonRecvPort == 0 {
		cfg.AbletonRecvPort = 11001
	}
	if cfg.DebounceMS == 0 {
		cfg.DebounceMS = 500
	}
	if cfg.MuseScoreBin == "" {
		cfg.MuseScoreBin = defaultMuseScoreBin()
	}
}

func defaultMuseScoreBin() string {
	if runtime.GOOS == "windows" {
		return `C:\Program Files\MuseScore 4\bin\MuseScore4.exe`
	}
	return "mscore"
}

func (c *Config) Validate() error {
	if c.Score == "" {
		return fmt.Errorf("config: 'score' is required")
	}
	return nil
}
