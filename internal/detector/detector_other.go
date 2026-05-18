//go:build !windows

package detector

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const musescoreExe = "MuseScore4"
const pollInterval = 2 * time.Second

type otherDetector struct{}

// New returns the non-Windows polling ProcessDetector.
func New() ProcessDetector { return &otherDetector{} }

func (d *otherDetector) OpenFiles() ([]string, error) {
	return openMsczFiles()
}

func (d *otherDetector) Watch(ctx context.Context) (<-chan []string, error) {
	ch := make(chan []string, 1)
	go func() {
		defer close(ch)
		var last []string
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				files, err := openMsczFiles()
				if err != nil {
					log.Printf("detector: poll error: %v", err)
					continue
				}
				if !slicesEqual(files, last) {
					last = files
					select {
					case ch <- files:
					default:
					}
				}
			}
		}
	}()
	return ch, nil
}

func openMsczFiles() ([]string, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name != musescoreExe {
			continue
		}
		cmdline, err := p.CmdlineSlice()
		if err != nil || len(cmdline) < 2 {
			continue
		}
		for _, arg := range cmdline[1:] {
			if strings.EqualFold(filepath.Ext(arg), ".mscz") {
				files = append(files, filepath.Clean(arg))
			}
		}
	}
	return files, nil
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
