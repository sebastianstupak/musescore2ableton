package watcher

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	path       string
	debounceMS int
	events     chan struct{}
	fsw        *fsnotify.Watcher
	once       sync.Once
}

func New(path string, debounceMS int) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("watcher: resolving path: %w", err)
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watcher: creating fsnotify watcher: %w", err)
	}
	return &Watcher{
		path:       absPath,
		debounceMS: debounceMS,
		events:     make(chan struct{}, 1),
		fsw:        fsw,
	}, nil
}

func (w *Watcher) Start() error {
	dir := filepath.Dir(w.path)
	if err := w.fsw.Add(dir); err != nil {
		return fmt.Errorf("watcher: adding dir: %w", err)
	}
	go w.loop()
	return nil
}

func (w *Watcher) Events() <-chan struct{} {
	return w.events
}

func (w *Watcher) Close() {
	w.once.Do(func() { w.fsw.Close() })
}

func (w *Watcher) loop() {
	var timer *time.Timer
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			absEv, _ := filepath.Abs(ev.Name)
			if absEv != w.path {
				continue
			}
			if ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) {
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(
					time.Duration(w.debounceMS)*time.Millisecond,
					func() {
						select {
						case w.events <- struct{}{}:
						default:
						}
					},
				)
			}
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}
