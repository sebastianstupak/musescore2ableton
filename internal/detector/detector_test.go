package detector_test

import (
	"context"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/detector"
)

// Smoke test: New() returns a non-nil detector and OpenFiles doesn't panic.
// It may return an empty slice if MuseScore isn't running — that's fine.
func TestNewAndOpenFiles(t *testing.T) {
	d := detector.New()
	files, err := d.OpenFiles()
	if err != nil {
		t.Fatalf("OpenFiles returned error: %v", err)
	}
	_ = files // may be empty; just ensure no panic
}

// TestWatchCancels verifies that Watch returns a channel that closes on ctx cancel.
func TestWatchCancels(t *testing.T) {
	d := detector.New()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := d.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			// A value was sent — that's fine, drain and wait for close.
			select {
			case <-ch:
			case <-time.After(3 * time.Second):
				t.Fatal("channel not closed after ctx cancel")
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel not closed after ctx cancel")
	}
}
