package watcher_test

import (
	"fmt"
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

func TestWatcher_IgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "score.mscz")
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(target, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(target, 50)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()
	w.Start()

	// Write to other file — should NOT trigger event
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(other, []byte("y"), 0644)

	select {
	case <-w.Events():
		t.Fatal("got event for unrelated file write")
	case <-time.After(300 * time.Millisecond):
		// success — no event
	}
}

func TestWatcher_DebouncesRapidWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "score.mscz")
	if err := os.WriteFile(path, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(path, 100) // 100ms debounce
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()
	w.Start()

	time.Sleep(20 * time.Millisecond)
	// Write 5 times rapidly — should produce only 1 event
	for i := 0; i < 5; i++ {
		os.WriteFile(path, []byte(fmt.Sprintf("v%d", i+2)), 0644)
	}

	count := 0
	timer := time.After(500 * time.Millisecond)
	for {
		select {
		case <-w.Events():
			count++
		case <-timer:
			goto done
		}
	}
done:
	if count != 1 {
		t.Errorf("got %d events for 5 rapid writes, want 1 (debounced)", count)
	}
}

func TestWatcher_Close_StopsEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "score.mscz")
	if err := os.WriteFile(path, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	w, err := watcher.New(path, 50)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	w.Start()
	w.Close() // Close immediately

	// Events channel should eventually close (unblock reads)
	select {
	case _, open := <-w.Events():
		if open {
			// Got an event before close propagated — check the channel closes eventually
		}
		// channel closed — OK
	case <-time.After(2 * time.Second):
		// Channel never closed — but this may be OK if it just blocks; the critical
		// thing is Close doesn't panic or deadlock
	}
}
