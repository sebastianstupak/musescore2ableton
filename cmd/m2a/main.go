package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/sebastianstupak/m2a/internal/ableton"
	"github.com/sebastianstupak/m2a/internal/config"
	"github.com/sebastianstupak/m2a/internal/exporter"
	"github.com/sebastianstupak/m2a/internal/globalconfig"
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
		// No --config flag: use auto-detect mode.
		if !cmd.Flags().Changed("config") {
			return runAutoWatch()
		}

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
		pauseCh := make(chan bool, 1)

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

		var mu sync.Mutex
		go func() {
			paused := false
			for {
				select {
				case <-w.Events():
					if !paused {
						if mu.TryLock() {
							runSync()
							mu.Unlock()
						}
					}
				case <-forceSyncCh:
					if mu.TryLock() {
						runSync()
						mu.Unlock()
					}
				case p := <-pauseCh:
					paused = p
				case <-quitCh:
					return
				}
			}
		}()

		log.Printf("m2a watching %s", cfg.Score)
		tray.Run(forceSyncCh, updateCh, quitCh, pauseCh, nil, nil)
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

// config root sub-commands

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage m2a configuration",
}

var configRootCmd = &cobra.Command{
	Use:   "root",
	Short: "Manage project root directories",
}

var rootAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a project root directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := globalconfig.Load()
		if err != nil {
			return err
		}
		root := args[0]
		if _, err := os.Stat(root); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: %s does not exist — saving anyway\n", root)
		}
		cfg.AddRoot(root)
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Added root: %s\n", root)
		return nil
	},
}

var rootRemoveCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Remove a project root directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := globalconfig.Load()
		if err != nil {
			return err
		}
		cfg.RemoveRoot(args[0])
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Removed root: %s\n", args[0])
		return nil
	},
}

var rootListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured project root directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := globalconfig.Load()
		if err != nil {
			return err
		}
		if len(cfg.Roots) == 0 {
			fmt.Println("No roots configured. Use: m2a config root add <path>")
			return nil
		}
		for _, r := range cfg.Roots {
			fmt.Println(r)
		}
		return nil
	},
}

// install / uninstall commands

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Register m2a as a Windows startup task (runs on login)",
	RunE:  runInstall,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the m2a Windows startup task",
	RunE:  runUninstall,
}

// create command

var createRoot string

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new m2a project folder",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		slug := slugify(name)

		baseDir := createRoot
		if baseDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			baseDir = cwd
			cfg, _ := globalconfig.Load()
			if cfg != nil && !underAnyRoot(cwd, cfg.Roots) {
				fmt.Fprintf(os.Stderr, "Warning: %s is not under any known root. Run `m2a config root add %s` to enable auto-detection.\n", cwd, cwd)
			}
		}

		projectDir := filepath.Join(baseDir, slug)
		if err := os.MkdirAll(filepath.Join(projectDir, "ableton"), 0755); err != nil {
			return err
		}
		yml := fmt.Sprintf("score: %s.mscz\n", slug)
		if err := os.WriteFile(filepath.Join(projectDir, "m2a.yml"), []byte(yml), 0644); err != nil {
			return err
		}
		fmt.Printf("Created project: %s\n", projectDir)
		fmt.Printf("Next: open Ableton, create a new set, and Save As → %s\n",
			filepath.Join(projectDir, "ableton", name+".als"))
		return nil
	},
}

func slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ', r == '_':
			b.WriteRune('-')
		}
	}
	return b.String()
}

func underAnyRoot(path string, roots []string) bool {
	clean := filepath.Clean(path)
	for _, r := range roots {
		rel, err := filepath.Rel(filepath.Clean(r), clean)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
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
	setupFileLog()
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "m2a.yml", "path to m2a.yml")
	createCmd.Flags().StringVar(&createRoot, "root", "", "parent directory to create project in (default: current directory)")
	configCmd.AddCommand(configRootCmd)
	configRootCmd.AddCommand(rootAddCmd, rootRemoveCmd, rootListCmd)
	rootCmd.AddCommand(watchCmd, syncCmd, resetCmd, configCmd, createCmd, installCmd, uninstallCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupFileLog() {
	logDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	logDir = filepath.Join(logDir, "m2a")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(logDir, "m2a.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}
