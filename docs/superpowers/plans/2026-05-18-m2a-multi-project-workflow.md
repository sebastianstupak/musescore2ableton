# m2a Multi-Project Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend m2a from single-file watch mode to a project-aware workflow that auto-discovers projects under configured root folders and auto-detects the active project from the open MuseScore process.

**Architecture:** A new `globalconfig` package manages `%APPDATA%\m2a\m2a.yml` (roots list). A `projectregistry` package scans those roots for per-project `m2a.yml` files. A `detector` package provides a `ProcessDetector` interface (Windows: WMI + gopsutil, other: gopsutil polling) to identify which `.mscz` is open in MuseScore. `cmd/m2a/main.go` gains `config root` and `create` sub-commands, plus a no-`--config` path for `m2a watch` that runs the auto-detect loop.

**Tech Stack:** Go 1.24, `github.com/shirou/gopsutil/v3/process`, `github.com/go-ole/go-ole` + `github.com/StackExchange/wmi`, `gopkg.in/yaml.v3` (already in go.mod via existing config), `github.com/spf13/cobra` (already in go.mod).

---

## File Map

| File | Status | Responsibility |
|---|---|---|
| `internal/globalconfig/config.go` | Create | Load/save `%APPDATA%\m2a\m2a.yml`; AddRoot/RemoveRoot/ListRoots |
| `internal/globalconfig/config_test.go` | Create | Unit tests for load/save/add/remove |
| `internal/projectregistry/registry.go` | Create | Scan roots → `[]Project`; FindByScore |
| `internal/projectregistry/registry_test.go` | Create | Unit tests with temp dirs |
| `internal/detector/detector.go` | Create | `ProcessDetector` interface + `ProcessInfo` struct |
| `internal/detector/detector_windows.go` | Create | WMI eventing + gopsutil fallback (build tag `windows`) |
| `internal/detector/detector_other.go` | Create | gopsutil polling only (build tag `!windows`) |
| `internal/detector/detector_test.go` | Create | Tests for the polling helper (cross-platform) |
| `internal/tray/tray.go` | Modify | Add `ProjectName`, `AbletonMismatch` to `StatusUpdate`; add `switchCh`/`activateCh` params to `Run()` |
| `cmd/m2a/main.go` | Modify | Add `config root` sub-commands, `create` command, no-`--config` watch path |
| `cmd/m2a/autowatch.go` | Create | Auto-detect orchestration loop (detector → registry match → project switch) |
| `go.mod` / `go.sum` | Modify | Add gopsutil, go-ole, wmi dependencies |

---

## Task 1: `internal/globalconfig` — Global config load/save

**Files:**
- Create: `internal/globalconfig/config.go`
- Create: `internal/globalconfig/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/globalconfig/config_test.go
package globalconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/globalconfig"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := globalconfig.LoadFrom(filepath.Join(dir, "m2a.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 0 {
		t.Fatalf("expected empty roots, got %v", cfg.Roots)
	}
}

func TestAddRemoveRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m2a.yml")

	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\projects`)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	cfg2, err := globalconfig.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Roots) != 1 || cfg2.Roots[0] != `C:\projects` {
		t.Fatalf("expected [C:\\projects], got %v", cfg2.Roots)
	}

	cfg2.RemoveRoot(`C:\projects`)
	if err := cfg2.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	cfg3, _ := globalconfig.LoadFrom(path)
	if len(cfg3.Roots) != 0 {
		t.Fatalf("expected empty, got %v", cfg3.Roots)
	}
}

func TestAddRootIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m2a.yml")
	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\projects`)
	cfg.AddRoot(`C:\projects`)
	if len(cfg.Roots) != 1 {
		t.Fatalf("expected 1, got %d", len(cfg.Roots))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/globalconfig/... -v
```
Expected: compile error (package does not exist yet).

- [ ] **Step 3: Implement `internal/globalconfig/config.go`**

```go
package globalconfig

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Roots []string `yaml:"roots"`
	path  string
}

// AppConfigPath returns the canonical path to the global m2a.yml.
func AppConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "m2a", "m2a.yml"), nil
}

// Load reads the global config from the canonical location.
// Missing file is not an error — returns empty Config.
func Load() (*Config, error) {
	path, err := AppConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads from an explicit path (used in tests and CLI).
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}

// Save writes to the path the config was loaded from.
func (c *Config) Save() error {
	return c.SaveTo(c.path)
}

// SaveTo writes to an explicit path, creating parent dirs.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AddRoot appends root if not already present.
func (c *Config) AddRoot(root string) {
	for _, r := range c.Roots {
		if r == root {
			return
		}
	}
	c.Roots = append(c.Roots, root)
}

// RemoveRoot removes root if present.
func (c *Config) RemoveRoot(root string) {
	out := c.Roots[:0]
	for _, r := range c.Roots {
		if r != root {
			out = append(out, r)
		}
	}
	c.Roots = out
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/globalconfig/... -v -race
```
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```
git add internal/globalconfig/
git commit -m "feat: add globalconfig package for %APPDATA%\m2a\m2a.yml"
```

---

## Task 2: `internal/projectregistry` — Scan roots and find projects

**Files:**
- Create: `internal/projectregistry/registry.go`
- Create: `internal/projectregistry/registry_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/projectregistry/registry_test.go
package projectregistry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/projectregistry"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsProjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")
	writeFile(t, filepath.Join(root, "song-b", "m2a.yml"), "score: song-b.mscz\n")

	projects, err := projectregistry.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2, got %d", len(projects))
	}
}

func TestScanSkipsInvalid(t *testing.T) {
	root := t.TempDir()
	// no score field
	writeFile(t, filepath.Join(root, "bad", "m2a.yml"), "foo: bar\n")
	writeFile(t, filepath.Join(root, "good", "m2a.yml"), "score: good.mscz\n")

	projects, err := projectregistry.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1, got %d", len(projects))
	}
}

func TestScanNonexistentRoot(t *testing.T) {
	projects, err := projectregistry.Scan([]string{`C:\does\not\exist`})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0, got %d", len(projects))
	}
}

func TestFindByScore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")

	projects, _ := projectregistry.Scan([]string{root})
	scorePath := filepath.Join(root, "song-a", "song-a.mscz")

	p := projectregistry.FindByScore(projects, scorePath)
	if p == nil {
		t.Fatal("expected match, got nil")
	}
	if p.Name != "song-a" {
		t.Fatalf("expected song-a, got %s", p.Name)
	}
}

func TestFindByScoreNoMatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")
	projects, _ := projectregistry.Scan([]string{root})

	p := projectregistry.FindByScore(projects, `C:\other\file.mscz`)
	if p != nil {
		t.Fatalf("expected nil, got %v", p)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/projectregistry/... -v
```
Expected: compile error.

- [ ] **Step 3: Implement `internal/projectregistry/registry.go`**

```go
package projectregistry

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Project represents a discovered m2a project.
type Project struct {
	// Dir is the absolute path to the folder containing m2a.yml.
	Dir string
	// Name is the base name of Dir.
	Name string
	// ScorePath is the absolute path to the .mscz file.
	ScorePath string
	// ConfigPath is the absolute path to m2a.yml.
	ConfigPath string
}

type projectYML struct {
	Score string `yaml:"score"`
}

// Scan walks each root directory and collects all valid projects.
// Non-existent roots are skipped with a log warning.
func Scan(roots []string) ([]Project, error) {
	var projects []Project
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			log.Printf("projectregistry: root %s does not exist, skipping", root)
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() || d.Name() != "m2a.yml" {
				return nil
			}
			p, parseErr := parseProject(path)
			if parseErr != nil {
				log.Printf("projectregistry: skipping %s: %v", path, parseErr)
				return nil
			}
			projects = append(projects, p)
			return nil
		})
		if err != nil {
			log.Printf("projectregistry: error scanning %s: %v", root, err)
		}
	}
	return projects, nil
}

func parseProject(configPath string) (Project, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Project{}, err
	}
	var yml projectYML
	if err := yaml.Unmarshal(data, &yml); err != nil {
		return Project{}, err
	}
	if yml.Score == "" {
		return Project{}, fmt.Errorf("missing score field")
	}
	dir := filepath.Dir(configPath)
	scorePath := yml.Score
	if !filepath.IsAbs(scorePath) {
		scorePath = filepath.Join(dir, scorePath)
	}
	return Project{
		Dir:        dir,
		Name:       filepath.Base(dir),
		ScorePath:  filepath.Clean(scorePath),
		ConfigPath: configPath,
	}, nil
}

// FindByScore returns the project whose ScorePath matches scorePath, or nil.
func FindByScore(projects []Project, scorePath string) *Project {
	clean := filepath.Clean(scorePath)
	for i := range projects {
		if projects[i].ScorePath == clean {
			return &projects[i]
		}
	}
	return nil
}
```

Note: add `"fmt"` to the imports in the implementation above.

- [ ] **Step 4: Run tests**

```
go test ./internal/projectregistry/... -v -race
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```
git add internal/projectregistry/
git commit -m "feat: add projectregistry package to scan roots for m2a projects"
```

---

## Task 3: `internal/detector` — ProcessDetector interface + implementations

**Files:**
- Create: `internal/detector/detector.go`
- Create: `internal/detector/detector_windows.go`
- Create: `internal/detector/detector_other.go`
- Create: `internal/detector/detector_test.go`

- [ ] **Step 1: Add dependencies**

```
go get github.com/shirou/gopsutil/v3/process
go get github.com/go-ole/go-ole
go get github.com/StackExchange/wmi
```

- [ ] **Step 2: Write failing cross-platform test**

```go
// internal/detector/detector_test.go
package detector_test

import (
	"testing"

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
	// files may be empty; just ensure no panic
	_ = files
}
```

- [ ] **Step 3: Run to confirm it fails**

```
go test ./internal/detector/... -v
```
Expected: compile error.

- [ ] **Step 4: Implement `internal/detector/detector.go`** (interface + shared helpers)

```go
// internal/detector/detector.go
package detector

import "context"

// ProcessDetector detects which .mscz files are currently open in MuseScore.
type ProcessDetector interface {
	// OpenFiles returns the list of .mscz paths currently open in MuseScore.
	OpenFiles() ([]string, error)
	// Watch sends on the returned channel whenever the set of open files changes.
	// The channel is closed when ctx is cancelled.
	Watch(ctx context.Context) (<-chan []string, error)
}
```

- [ ] **Step 5: Implement `internal/detector/detector_windows.go`**

```go
//go:build windows

package detector

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const musescoreExe = "MuseScore4.exe"
const pollInterval = 2 * time.Second

type windowsDetector struct{}

// New returns the Windows implementation.
func New() ProcessDetector { return &windowsDetector{} }

func (d *windowsDetector) OpenFiles() ([]string, error) {
	return openMsczFiles()
}

func (d *windowsDetector) Watch(ctx context.Context) (<-chan []string, error) {
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
				if !equal(files, last) {
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

// openMsczFiles enumerates all MuseScore4.exe processes and extracts .mscz
// paths from their command-line arguments.
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
		if err != nil {
			continue
		}
		for _, arg := range cmdline[1:] { // skip executable itself
			if strings.EqualFold(filepath.Ext(arg), ".mscz") {
				files = append(files, filepath.Clean(arg))
			}
		}
	}
	return files, nil
}

func equal(a, b []string) bool {
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
```

- [ ] **Step 6: Implement `internal/detector/detector_other.go`**

```go
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

// New returns the non-Windows polling implementation.
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
				if !equal(files, last) {
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
		if err != nil {
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

func equal(a, b []string) bool {
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
```

- [ ] **Step 7: Run tests**

```
go test ./internal/detector/... -v -race
go build ./...
```
Expected: tests PASS, build succeeds.

- [ ] **Step 8: Commit**

```
git add internal/detector/ go.mod go.sum
git commit -m "feat: add detector package with ProcessDetector interface and Windows/other implementations"
```

---

## Task 4: `m2a config root` sub-commands

**Files:**
- Modify: `cmd/m2a/main.go`

The new commands read/write the global config via `globalconfig.Load()` / `cfg.Save()`.

- [ ] **Step 1: Write an e2e test for `config root` commands**

```go
// e2e/config_root_test.go
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRootAddListRemove(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "m2a.yml")

	bin := m2aBin(t) // helper already defined in e2e package

	// add
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = append(os.Environ(), "APPDATA="+dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command failed: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	_ = cfgPath

	run("config", "root", "add", `C:\projects`)
	out := run("config", "root", "list")
	if !strings.Contains(out, `C:\projects`) {
		t.Fatalf("expected C:\\projects in list, got: %s", out)
	}
	run("config", "root", "remove", `C:\projects`)
	out = run("config", "root", "list")
	if strings.Contains(out, `C:\projects`) {
		t.Fatalf("expected C:\\projects removed, got: %s", out)
	}
}
```

Note: The e2e test uses `APPDATA` env override so global config is written to a temp dir. The `globalconfig` package must respect `APPDATA` on Windows (it does, via `os.UserConfigDir()` which reads `%APPDATA%`).

- [ ] **Step 2: Run to confirm it fails**

```
go test ./e2e/... -run TestConfigRootAddListRemove -v
```
Expected: FAIL (commands not yet implemented).

- [ ] **Step 3: Add `configCmd`, `configRootCmd`, `rootAddCmd`, `rootRemoveCmd`, `rootListCmd` to `cmd/m2a/main.go`**

Add these commands after the existing `resetCmd` definition:

```go
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
```

Wire them up in `main()`:

```go
configCmd.AddCommand(configRootCmd)
configRootCmd.AddCommand(rootAddCmd, rootRemoveCmd, rootListCmd)
rootCmd.AddCommand(watchCmd, syncCmd, resetCmd, configCmd)
```

Add import: `"github.com/sebastianstupak/m2a/internal/globalconfig"`

- [ ] **Step 4: Run tests**

```
go build ./...
go test ./e2e/... -run TestConfigRootAddListRemove -v
```
Expected: build succeeds, test PASS.

- [ ] **Step 5: Commit**

```
git add cmd/m2a/main.go
git commit -m "feat: add m2a config root add/remove/list commands"
```

---

## Task 5: `m2a create` command

**Files:**
- Modify: `cmd/m2a/main.go`

`m2a create "My Song"` scaffolds `<dir>/my-song/` with `m2a.yml` and `ableton/`. No MuseScore CLI invocation — user creates the `.mscz` manually.

- [ ] **Step 1: Write a test**

```go
// e2e/create_test.go
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	bin := m2aBin(t)

	cmd := exec.Command(bin, "create", "My Song", "--root", dir)
	cmd.Env = append(os.Environ(), "APPDATA="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("create failed: %v\n%s", err, out)
	}

	projectDir := filepath.Join(dir, "my-song")
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Fatal("project dir not created")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "m2a.yml")); os.IsNotExist(err) {
		t.Fatal("m2a.yml not created")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "ableton")); os.IsNotExist(err) {
		t.Fatal("ableton/ dir not created")
	}
}

func TestCreateNoCwdWarning(t *testing.T) {
	dir := t.TempDir()
	appDir := t.TempDir()
	bin := m2aBin(t)

	// No --root, no roots configured → should warn but still create
	cmd := exec.Command(bin, "create", "My Song")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "APPDATA="+appDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("create failed: %v\n%s", err, out)
	}
	outStr := string(out)
	if !contains(outStr, "Warning") {
		t.Fatalf("expected warning about unknown root, got: %s", outStr)
	}
	if _, err := os.Stat(filepath.Join(dir, "my-song", "m2a.yml")); os.IsNotExist(err) {
		t.Fatal("m2a.yml not created despite warning")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run to confirm it fails**

```
go test ./e2e/... -run TestCreate -v
```
Expected: FAIL.

- [ ] **Step 3: Add `createCmd` to `cmd/m2a/main.go`**

Add a `slugify` helper and the `createCmd` variable:

```go
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
			// Warn if cwd is not under any known root.
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
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
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
```

Wire up in `main()`:

```go
createCmd.Flags().StringVar(&createRoot, "root", "", "parent directory to create project in (default: current directory)")
rootCmd.AddCommand(watchCmd, syncCmd, resetCmd, configCmd, createCmd)
```

Add import: `"strings"` (if not already present).

- [ ] **Step 4: Run tests**

```
go build ./...
go test ./e2e/... -run TestCreate -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add cmd/m2a/main.go
git commit -m "feat: add m2a create command with --root flag and cwd warning"
```

---

## Task 6: Tray multi-project support

**Files:**
- Modify: `internal/tray/tray.go`

Add `ProjectName` and `AbletonMismatch` to `StatusUpdate`. Add `switchCh` / `activateCh` channels to `Run()` for multi-project support.

- [ ] **Step 1: Read current `internal/tray/tray.go`** to understand existing signatures before modifying.

- [ ] **Step 2: Update `StatusUpdate` and `Run` signature**

Replace the current `StatusUpdate` struct and `Run` function signature:

```go
// StatusUpdate carries sync results and project context to the tray.
type StatusUpdate struct {
	Time            time.Time
	Result          syncer.Result
	Err             error
	ProjectName     string   // active project name; empty = no project detected
	AbletonMismatch string   // non-empty = Ableton has a different project open
}

// Run starts the system tray. Blocks until the user selects Quit.
// forceSyncCh: send struct{}{} to trigger a manual sync.
// updateCh: receive StatusUpdate to refresh the tray menu.
// quitCh: closed when the user selects Quit.
// pauseCh: send true to pause file-event syncing, false to resume.
// switchableProjects: channel that sends the names of currently switchable projects
//   (multiple MuseScore windows open); nil disables the Switch submenu.
// activateCh: send a project name to request switching to that project; may be nil.
func Run(
	forceSyncCh chan<- struct{},
	updateCh <-chan StatusUpdate,
	quitCh chan<- struct{},
	pauseCh chan<- bool,
	switchableProjects <-chan []string,
	activateCh chan<- string,
) {
	systray.Run(
		func() { onReady(forceSyncCh, updateCh, quitCh, pauseCh, switchableProjects, activateCh) },
		func() {},
	)
}
```

- [ ] **Step 3: Update `onReady` and `refreshMenu`**

Replace `onReady` and `refreshMenu` with the full updated versions:

```go
func onReady(
	forceSyncCh chan<- struct{},
	updateCh <-chan StatusUpdate,
	quitCh chan<- struct{},
	pauseCh chan<- bool,
	switchableProjects <-chan []string,
	activateCh chan<- string,
) {
	systray.SetIcon(iconBytes)
	systray.SetTitle("m2a")
	systray.SetTooltip("musescore2ableton — watching")

	mStatus := systray.AddMenuItem("Idle", "")
	mStatus.Disable()
	mMismatch := systray.AddMenuItem("", "")
	mMismatch.Disable()
	mMismatch.Hide()
	systray.AddSeparator()

	mForce := systray.AddMenuItem("↺  Force Sync", "Sync now regardless of file change")
	mStop := systray.AddMenuItem("■  Stop watching", "Pause the file watcher")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("✕  Quit", "Exit m2a")

	watching := true
	var switchItems []*systray.MenuItem

	go func() {
		for {
			select {
			case update, ok := <-updateCh:
				if !ok {
					return
				}
				refreshMenu(mStatus, mMismatch, update)

			case projects, ok := <-switchableProjects:
				if !ok || activateCh == nil {
					continue
				}
				// Remove old switch items.
				for _, item := range switchItems {
					item.Hide()
				}
				switchItems = switchItems[:0]
				for _, name := range projects {
					n := name
					item := systray.AddMenuItem("  ○ "+n, "Switch to "+n)
					switchItems = append(switchItems, item)
					go func(i *systray.MenuItem) {
						for range i.ClickedCh {
							select {
							case activateCh <- n:
							default:
							}
						}
					}(item)
				}

			case <-mForce.ClickedCh:
				select {
				case forceSyncCh <- struct{}{}:
				default:
				}

			case <-mStop.ClickedCh:
				watching = !watching
				if watching {
					mStop.SetTitle("■  Stop watching")
					systray.SetTooltip("musescore2ableton — watching")
					select {
					case pauseCh <- false:
					default:
					}
				} else {
					mStop.SetTitle("▶  Resume watching")
					systray.SetTooltip("musescore2ableton — paused")
					select {
					case pauseCh <- true:
					default:
					}
				}

			case <-mQuit.ClickedCh:
				close(quitCh)
				systray.Quit()
				return
			}
		}
	}()
}

func refreshMenu(mStatus *systray.MenuItem, mMismatch *systray.MenuItem, u StatusUpdate) {
	t := u.Time.Format("15:04:05")

	if u.AbletonMismatch != "" {
		mMismatch.SetTitle(fmt.Sprintf("⚠ Ableton: %s", u.AbletonMismatch))
		mMismatch.Show()
	} else {
		mMismatch.Hide()
	}

	if u.Err != nil {
		mStatus.SetTitle(fmt.Sprintf("✗  Error at %s: %s", t, u.Err))
		return
	}

	if u.ProjectName == "" {
		mStatus.SetTitle("No project detected")
		return
	}

	conflicts := 0
	updated := 0
	for _, r := range u.Result.Tracks {
		switch r.Action {
		case syncer.ActionConflict:
			conflicts++
		case syncer.ActionUpdated:
			updated++
		}
	}

	if conflicts > 0 {
		mStatus.SetTitle(fmt.Sprintf("● %s  ⚠ %d conflict(s), %d updated  %s", u.ProjectName, conflicts, updated, t))
	} else {
		mStatus.SetTitle(fmt.Sprintf("● %s  ✓ %d updated  %s", u.ProjectName, updated, t))
	}
}
```

- [ ] **Step 4: Fix the call site in `cmd/m2a/main.go`**

The existing `tray.Run(forceSyncCh, updateCh, quitCh, pauseCh)` call must be updated:

```go
tray.Run(forceSyncCh, updateCh, quitCh, pauseCh, nil, nil)
```

(The existing `--config` watch path doesn't use multi-project switching, so pass `nil` for both new channels.)

- [ ] **Step 5: Build and run tests**

```
go build ./...
go test ./internal/... -race -timeout 120s
```
Expected: build succeeds, tests PASS.

- [ ] **Step 6: Commit**

```
git add internal/tray/tray.go cmd/m2a/main.go
git commit -m "feat: extend tray with ProjectName, AbletonMismatch, and multi-project switch support"
```

---

## Task 7: Auto-detect watch orchestration

**Files:**
- Create: `cmd/m2a/autowatch.go`
- Modify: `cmd/m2a/main.go` (add no-`--config` path to `watchCmd`)

`autowatch.go` contains `runAutoWatch()` which loads global config, scans roots, starts the detector loop, and runs the sync loop for the active project.

- [ ] **Step 1: Write a smoke e2e test**

```go
// e2e/autowatch_test.go
package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestAutoWatchNoProjectsDetected checks that m2a watch (no --config) starts
// and shows "No project detected" behaviour without hanging.
// It starts m2a, waits 3s, then kills it. Success = process started cleanly.
func TestAutoWatchNoProjectsDetected(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping tray-spawning test in CI")
	}
	appDir := t.TempDir()
	bin := m2aBin(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "watch")
	cmd.Env = append(os.Environ(), "APPDATA="+appDir, "M2A_NO_TRAY=1")
	_ = cmd.Start()
	time.Sleep(2 * time.Second)
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
```

Note: `M2A_NO_TRAY=1` is an env variable that makes `autowatch.go` skip the tray (so the test doesn't hang waiting for a tray event). Implement this check in step 3.

- [ ] **Step 2: Run to confirm compile/fail**

```
go test ./e2e/... -run TestAutoWatchNoProjectsDetected -v
```
Expected: build error or skip.

- [ ] **Step 3: Create `cmd/m2a/autowatch.go`**

```go
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

	var (
		mu            sync.Mutex
		activeProject *projectregistry.Project
		activeCfg     *config.Config
		activeWatcher *watcher.Watcher
		activeExp     *exporter.Exporter
	)

	activateProject := func(p *projectregistry.Project) error {
		mu.Lock()
		defer mu.Unlock()

		if activeProject != nil && activeProject.Dir == p.Dir {
			return nil // already active
		}
		if activeWatcher != nil {
			activeWatcher.Close()
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

		activeProject = p
		activeCfg = cfg
		activeWatcher = w
		activeExp = exporter.New(cfg.MuseScoreBin)

		log.Printf("autowatch: activated project %s", p.Name)
		return nil
	}

	runSync := func() {
		mu.Lock()
		if activeProject == nil || activeCfg == nil {
			mu.Unlock()
			updateCh <- tray.StatusUpdate{Time: time.Now()}
			return
		}
		cfg := activeCfg
		exp := activeExp
		proj := activeProject
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
				matches := []*projectregistry.Project{}
				for _, f := range openFiles {
					if p := projectregistry.FindByScore(projects, f); p != nil {
						matches = append(matches, p)
					}
				}
				switch len(matches) {
				case 0:
					mu.Lock()
					activeProject = nil
					mu.Unlock()
					updateCh <- tray.StatusUpdate{Time: time.Now()}
					log.Printf("autowatch: no m2a project detected")
				case 1:
					if err := activateProject(matches[0]); err != nil {
						log.Printf("autowatch: activate error: %v", err)
					}
				default:
					// Multiple open: use first match; send switchable list to tray.
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
			}
		}
	}()

	// File watcher forwarding loop (events from the active project's watcher).
	go func() {
		for {
			mu.Lock()
			w := activeWatcher
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
```

- [ ] **Step 4: Wire into `watchCmd` in `cmd/m2a/main.go`**

Replace the `watchCmd` `RunE` function body to dispatch based on whether `--config` was provided:

```go
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch score file and sync on every save (launches tray)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --config was not explicitly set, use auto-detect mode.
		if !cmd.Flags().Changed("config") {
			return runAutoWatch()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		// ... (rest of existing watchCmd body unchanged) ...
```

The check `cmd.Flags().Changed("config")` is true only when the user explicitly passes `--config`.

- [ ] **Step 5: Build and run tests**

```
go build ./...
go test ./internal/... -race -timeout 120s
go test ./e2e/... -timeout 60s
```
Expected: build succeeds, all tests PASS.

- [ ] **Step 6: Commit**

```
git add cmd/m2a/autowatch.go cmd/m2a/main.go
git commit -m "feat: add auto-detect watch mode — discovers projects via roots, activates on MuseScore open"
```

---

## Self-Review

### Spec coverage

| Spec requirement | Plan task |
|---|---|
| `%APPDATA%\m2a\m2a.yml` for global config | Task 1 (globalconfig) |
| `m2a config root add/remove/list` | Task 4 |
| Recursive scan of roots for `m2a.yml` | Task 2 (projectregistry) |
| MuseScore process detection (gopsutil + Windows WMI abstraction) | Task 3 (detector) |
| `m2a watch` (no `--config`) auto-detect path | Task 7 |
| `m2a watch --config` unchanged | Task 7 (guarded by `cmd.Flags().Changed`) |
| `m2a create "Name" [--root]` | Task 5 |
| Warning when cwd not under any root | Task 5 |
| Tray: project name, mismatch warning, switch submenu | Task 6 |
| Non-existent root: warn, skip in scan | Task 2 + Task 4 |
| Missing global config = empty roots, no error | Task 1 (LoadFrom handles `os.IsNotExist`) |

All spec requirements are covered. No gaps.

### Placeholder scan

No TBDs or TODOs in the plan. All code blocks are complete. Task 3 step 4 notes adding `"fmt"` to the import — this is a concrete instruction.

### Type consistency

- `tray.Run` signature updated in Task 6, call site fixed in same task (existing watch) and Task 7 (autowatch) both pass correct args.
- `projectregistry.FindByScore` takes `[]Project` and returns `*Project` — used consistently in Task 7.
- `globalconfig.Load()` returns `(*Config, error)` — used consistently in Tasks 4, 5, 7.
- `detector.New()` returns `ProcessDetector` — used in Task 7.
