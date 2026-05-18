package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sebastianstupak/m2a/internal/watcher"
)

func TestWatcher_DetectsFileWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "score.mscz")
	if err := os.WriteFile(path, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(path, 50) // 50ms debounce for fast tests
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no event received after file write")
	}
}
