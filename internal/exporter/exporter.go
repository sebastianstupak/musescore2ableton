package exporter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CommandRunner func(bin string, args []string) error

type Exporter struct {
	bin    string
	runner CommandRunner
}

func New(bin string) *Exporter {
	return &Exporter{bin: bin, runner: realRunner}
}

func NewWithRunner(bin string, runner CommandRunner) *Exporter {
	return &Exporter{bin: bin, runner: runner}
}

func (e *Exporter) ExportMIDI(scorePath string) (string, error) {
	outPath := filepath.Join(os.TempDir(), "m2a_sync.mid")
	if err := e.runner(e.bin, []string{"-o", outPath, scorePath}); err != nil {
		return "", fmt.Errorf("musescore export: %w", err)
	}
	return outPath, nil
}

func realRunner(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(out))
	}
	return nil
}
