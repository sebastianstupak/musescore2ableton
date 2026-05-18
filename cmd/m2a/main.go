package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/config"
	"github.com/sebastianstupak/m2a/internal/exporter"
	"github.com/sebastianstupak/m2a/internal/parser"
	"github.com/sebastianstupak/m2a/internal/state"
	"github.com/sebastianstupak/m2a/internal/syncer"
	"github.com/sebastianstupak/m2a/internal/tray"
	"github.com/sebastianstupak/m2a/internal/watcher"
)

const oscTimeout = 2 * time.Second

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "m2a",
	Short: "musescore2ableton — sync MuseScore saves to Ableton Live",
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch score file and sync on every save (launches tray)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		ab, err := ableton.NewClient(cfg.AbletonOSCHost, cfg.AbletonOSCPort, cfg.AbletonRecvPort)
		if err != nil {
			return fmt.Errorf("connecting to AbletonOSC: %w", err)
		}
		defer ab.Close()

		exp := exporter.New(cfg.MuseScoreBin)
		statePath := state.StatePath(cfg.Score)

		forceSyncCh := make(chan struct{}, 1)
		updateCh := make(chan tray.StatusUpdate, 4)
		quitCh := make(chan struct{})

		runSync := func() {
			midPath, err := exp.ExportMIDI(cfg.Score)
			if err != nil {
				log.Printf("export error: %v", err)
				updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err}
				return
			}
			tracks, err := parser.Parse(midPath)
			if err != nil {
				log.Printf("parse error: %v", err)
				updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err}
				return
			}
			snap, _ := state.Load(statePath)
			result, err := syncer.SyncTracks(ab, snap, tracks, oscTimeout)
			if err != nil {
				log.Printf("sync error: %v", err)
				updateCh <- tray.StatusUpdate{Time: time.Now(), Err: err}
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
			updateCh <- tray.StatusUpdate{Time: time.Now(), Result: result}
			logResult(result)
		}

		w, err := watcher.New(cfg.Score, cfg.DebounceMS)
		if err != nil {
			return err
		}
		defer w.Close()
		if err := w.Start(); err != nil {
			return err
		}

		go func() {
			for {
				select {
				case <-w.Events():
					runSync()
				case <-forceSyncCh:
					runSync()
				case <-quitCh:
					return
				}
			}
		}()

		log.Printf("m2a watching %s", cfg.Score)
		tray.Run(forceSyncCh, updateCh, quitCh)
		return nil
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "One-shot sync without watching or tray",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		ab, err := ableton.NewClient(cfg.AbletonOSCHost, cfg.AbletonOSCPort, cfg.AbletonRecvPort)
		if err != nil {
			return err
		}
		defer ab.Close()

		exp := exporter.New(cfg.MuseScoreBin)
		midPath, err := exp.ExportMIDI(cfg.Score)
		if err != nil {
			return err
		}
		tracks, err := parser.Parse(midPath)
		if err != nil {
			return err
		}
		statePath := state.StatePath(cfg.Score)
		snap, _ := state.Load(statePath)
		result, err := syncer.SyncTracks(ab, snap, tracks, oscTimeout)
		if err != nil {
			return err
		}
		logResult(result)
		return nil
	},
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete state file (forces full re-sync next time)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		path := state.StatePath(cfg.Score)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("State reset: %s\n", path)
		return nil
	},
}

func logResult(result syncer.Result) {
	for name, r := range result.Tracks {
		if r.Error != nil {
			log.Printf("  ✗ %-20s error: %v", name, r.Error)
		} else {
			log.Printf("  %s %-20s %s", actionIcon(r.Action), name, r.Action)
		}
	}
}

func actionIcon(a syncer.Action) string {
	switch a {
	case syncer.ActionUpdated:
		return "✓"
	case syncer.ActionConflict:
		return "⚠"
	default:
		return "–"
	}
}

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "m2a.yml", "path to m2a.yml")
	rootCmd.AddCommand(watchCmd, syncCmd, resetCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
