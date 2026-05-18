package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/config"
	"github.com/sebastianstupak/m2a/internal/detector"
	"github.com/sebastianstupak/m2a/internal/exporter"
	"github.com/sebastianstupak/m2a/internal/globalconfig"
	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/projectregistry"
	"github.com/sebastianstupak/m2a/internal/state"
	"github.com/sebastianstupak/m2a/internal/syncer"
	"github.com/sebastianstupak/m2a/internal/tray"
	"github.com/sebastianstupak/m2a/internal/watcher"
)

func runAutoWatch() error {
	globalCfg, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	projects, err := projectregistry.Scan(globalCfg.Roots)
	if err != nil {
		return fmt.Errorf("scanning roots: %w", err)
	}
	log.Printf("autowatch: found %d project(s)", len(projects))

	det := detector.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileCh, err := det.Watch(ctx)
	if err != nil {
		return fmt.Errorf("starting detector: %w", err)
	}

	forceSyncCh := make(chan struct{}, 1)
	updateCh := make(chan tray.StatusUpdate, 4)
	quitCh := make(chan struct{})
	pauseCh := make(chan bool, 1)
	switchableProjectsCh := make(chan []string, 1)
	activateCh := make(chan string, 1)

	type activeState struct {
		project *projectregistry.Project
		cfg     *config.Config
		w       *watcher.Watcher
		exp     *exporter.Exporter
	}
	var (
		mu     sync.Mutex
		active activeState
	)

	activateProject := func(p *projectregistry.Project) error {
		mu.Lock()
		defer mu.Unlock()
		if active.project != nil && active.project.Dir == p.Dir {
			return nil
		}
		if active.w != nil {
			active.w.Close()
		}
		cfg, err := config.Load(p.ConfigPath)
		if err != nil {
			return fmt.Errorf("loading project config %s: %w", p.ConfigPath, err)
		}
		w, err := watcher.New(cfg.Score, cfg.DebounceMS)
		if err != nil {
			return err
		}
		if err := w.Start(); err != nil {
			return err
		}
		active = activeState{project: p, cfg: cfg, w: w, exp: exporter.New(cfg.MuseScoreBin)}
		log.Printf("autowatch: activated project %s", p.Name)
		return nil
	}

	runSync := func() {
		mu.Lock()
		if active.project == nil {
			mu.Unlock()
			updateCh <- tray.StatusUpdate{Time: time.Now()}
			return
		}
		proj := active.project
		cfg := active.cfg
		exp := active.exp
		mu.Unlock()

		midPath, err := exp.ExportMIDI(cfg.Score)
		if err != nil {
			log.Printf("export error: %v", err)
			updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err, ProjectName: proj.Name}
			return
		}
		tracks, err := parser.Parse(midPath)
		if err != nil {
			log.Printf("parse error: %v", err)
			updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err, ProjectName: proj.Name}
			return
		}
		ab, err := ableton.NewClient(cfg.AbletonOSCHost, cfg.AbletonOSCPort, cfg.AbletonRecvPort)
		if err != nil {
			log.Printf("ableton connect error: %v", err)
			updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err, ProjectName: proj.Name}
			return
		}
		defer ab.Close()

		statePath := state.StatePath(cfg.Score)
		snap, _ := state.Load(statePath)
		result, err := syncer.SyncTracks(ab, snap, tracks, oscTimeout)
		if err != nil {
			log.Printf("sync error: %v", err)
			updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err, ProjectName: proj.Name}
			return
		}
		for _, t := range tracks {
			if r := result.Tracks[t.Name]; r.Action != syncer.ActionError {
				snap.Tracks[t.Name] = state.TrackSnapshot{
					Notes:    t.Notes,
					Tempos:   t.Tempos,
					TimeSigs: t.TimeSigs,
				}
			}
		}
		snap.SyncedAt = time.Now()
		snap.ScorePath = cfg.Score
		_ = snap.Save(statePath)

		updateCh <- tray.StatusUpdate{Time: time.Now(), Result: result, ProjectName: proj.Name}
		logResult(result)
	}

	// Detector and project resolution loop.
	go func() {
		paused := false
		for {
			select {
			case openFiles, ok := <-fileCh:
				if !ok {
					return
				}
				var matches []*projectregistry.Project
				for _, f := range openFiles {
					if p := projectregistry.FindByScore(projects, f); p != nil {
						matches = append(matches, p)
					}
				}
				switch len(matches) {
				case 0:
					mu.Lock()
					active.project = nil
					mu.Unlock()
					updateCh <- tray.StatusUpdate{Time: time.Now()}
					log.Printf("autowatch: no m2a project detected")
				case 1:
					if err := activateProject(matches[0]); err != nil {
						log.Printf("autowatch: activate error: %v", err)
					}
				default:
					if err := activateProject(matches[0]); err != nil {
						log.Printf("autowatch: activate error: %v", err)
					}
					names := make([]string, len(matches))
					for i, m := range matches {
						names[i] = m.Name
					}
					select {
					case switchableProjectsCh <- names:
					default:
					}
				}

			case name := <-activateCh:
				for i := range projects {
					if projects[i].Name == name {
						if err := activateProject(&projects[i]); err != nil {
							log.Printf("autowatch: manual activate error: %v", err)
						}
						break
					}
				}

			case <-forceSyncCh:
				if !paused {
					runSync()
				}

			case p := <-pauseCh:
				paused = p

			case <-quitCh:
				return
			}
		}
	}()

	// File watcher event forwarding loop.
	go func() {
		for {
			select {
			case <-quitCh:
				return
			default:
			}
			mu.Lock()
			w := active.w
			mu.Unlock()
			if w == nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			select {
			case <-w.Events():
				runSync()
			case <-quitCh:
				return
			case <-time.After(500 * time.Millisecond):
				// Re-check active watcher in case it was replaced.
			}
		}
	}()

	if os.Getenv("M2A_NO_TRAY") == "1" {
		<-quitCh
		return nil
	}

	tray.Run(forceSyncCh, updateCh, quitCh, pauseCh, switchableProjectsCh, activateCh)
	return nil
}
