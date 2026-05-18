# Agent Guide — m2a

This file tells AI coding agents (Claude Code, Copilot, etc.) everything they need to work effectively in this repo.

---

## What this project is

`m2a` is a Go CLI tool that watches a MuseScore `.mscz` file and syncs its contents into a running Ableton Live session via [AbletonOSC](https://github.com/ideoforms/AbletonOSC) on every save. It detects conflicts when both sides have diverged since the last sync and creates a labeled conflict track rather than silently overwriting Ableton edits.

Single static binary. No runtime dependencies beyond AbletonOSC.

---

## Repo layout

```
cmd/m2a/main.go              CLI entry point — wires all modules, defines watch/sync/reset commands
internal/
  config/                    Load + validate m2a.yml; apply defaults
  watcher/                   fsnotify wrapper with debounce; emits on channel on .mscz write
  exporter/                  Runs MuseScore4.exe via os/exec; returns temp MIDI path
  parser/                    Parses MIDI file → []Track (notes, tempos, time sigs)
  state/                     Load/save .m2a_state.json; Track diff computation
  ableton/
    client.go                AbletonOSC UDP client (send + receive)
    tracks.go                List tracks, find by name, create MIDI track
    clips.go                 Read/clear/write clip notes, set color/name
  syncer/                    Orchestrates per-track sync logic and conflict detection
  tray/                      System tray icon and menu (fyne.io/systray)
    assets/icon.ico          Tray icon — ICO wrapping a 32×32 PNG (Windows requires ICO)
  testutil/
    fakeosc.go               Stateful fake AbletonOSC UDP server for tests
e2e/                         End-to-end tests (subprocess + integration against FakeServer)
```

---

## Build and test

```sh
# Build
go build ./...

# Install locally
go install ./cmd/m2a

# Build Windows GUI binary (no console window, logs to %LOCALAPPDATA%\m2a\m2a.log)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o m2a.exe ./cmd/m2a

# Run all tests
go test ./internal/... -race -timeout 120s
go test ./e2e/... -timeout 60s

# Lint
golangci-lint run
```

On Linux, install systray dependencies first:
```sh
sudo apt-get install -y gcc libgtk-3-dev libayatana-appindicator3-dev
```

---

## Key decisions

**Tray icon must be ICO format on Windows.** `fyne.io/systray` accepts ICO. The PNG alone causes `SetIcon` to silently fail (`ERROR: unable to set icon: The operation completed successfully`). `internal/tray/assets/icon.ico` wraps the PNG in an ICO container.

**`-H windowsgui` suppresses the console window** when running `m2a watch`. Without it, a console window opens alongside the tray icon. With it, all log output goes to `%LOCALAPPDATA%\m2a\m2a.log` via `setupFileLog()` in `main.go`.

**MuseScore path has a space.** The default path is `C:\Program Files\MuseScore 4\bin\MuseScore4.exe` — note the space between `MuseScore` and `4`. This is different from what you might expect.

**AbletonOSC install structure.** The entire AbletonOSC repo root (containing `__init__.py`, `abletonosc/`, and `pythonosc/`) must be placed as a folder named `AbletonOSC` under:
```
C:\Users\<user>\Documents\Ableton\User Library\Remote Scripts\AbletonOSC\
```
The `pythonosc/` sibling is required; copying only the `abletonosc/` subdirectory produces a `ValueError: attempted relative import beyond top-level package` at Ableton startup.

**FakeServer thread safety.** `testutil.FakeServer` exposes only mutex-protected setter/getter methods (`SetTrackNames`, `SetClipNotes`, `SetNextTrackIdx`, `GetNextTrackIdx`). Do not add public fields — the serve goroutine reads these under `s.mu`.

**WaitForReceived for async assertions.** OSC sends in `SyncTracks` (tempo, track names, notes) are UDP fire-and-forget. Tests must use `WaitForReceived(addr, timeout)` rather than `HasReceived` immediately after `SyncTracks` returns.

**golangci-lint v2 config.** `.golangci.yml` uses `version: "2"` schema. Run with `golangci-lint-action@v7` and `install-mode: goinstall` (builds from source to match the runner's Go version; avoids a panic when the binary was compiled with an older Go).

---

## Data flow

```
.mscz written (Ctrl+S in MuseScore)
  → watcher detects write, debounces 500ms
  → exporter: MuseScore4.exe -o /tmp/m2a_sync.mid score.mscz
  → parser: MIDI → []Track{Name, Notes, Tempos, TimeSigs}
  → state.Load → SyncState (last snapshot)
  → syncer.SyncTracks:
      for each track:
        musescore_delta = diff(newTrack, lastSnapshot)
        ableton_delta   = diff(currentAbletonClip, lastSnapshot)
        if both non-empty → conflict: create "Name (CHANGES)" track above, color red
        if only musescore changed → update clip in place
        if only ableton changed → skip (preserve user's edits)
        if neither changed → skip
  → state.Save (only for non-errored tracks)
  → tray: update status display
```

---

## OSC addresses used

| Operation | Address |
|---|---|
| List track names | `/live/song/get/track_names` |
| Get clip notes | `/live/clip/get/notes` |
| Create MIDI track | `/live/song/create_midi_track` |
| Create clip | `/live/clip_slot/create_clip` |
| Add notes | `/live/clip/add/notes` |
| Remove all notes | `/live/clip/remove/notes` |
| Set track name | `/live/track/set/name` |
| Set track color | `/live/track/set/color` |
| Set song tempo | `/live/song/set/tempo` |

---

## Config file (`m2a.yml`)

```yaml
score: C:\path\to\MySong.mscz   # required; path to the .mscz file to watch
musescore_bin: ...               # default: C:\Program Files\MuseScore 4\bin\MuseScore4.exe
ableton_osc_host: 127.0.0.1     # default
ableton_osc_port: 11000          # default (AbletonOSC listen port)
ableton_recv_port: 11001         # default (port m2a binds to receive replies)
debounce_ms: 500                 # default
```

---

## Testing notes

- Unit tests for `internal/` packages use `testutil.FakeServer` — no real Ableton or MuseScore needed
- E2E tests in `e2e/` spawn the real `m2a` binary as a subprocess and communicate with a `FakeServer`
- The race detector (`-race`) is required in CI — do not skip it
- `exporter_test.go` uses a fake MuseScore script; on Linux it must be `.sh`, on Windows `.bat`
- There are no tests that require a real Ableton or MuseScore instance; manual testing is needed for full integration

---

## CI / release

- `.github/workflows/ci.yml` — runs tests and lint on every push to main/master
- `.github/workflows/release.yml` — builds Windows/Linux/macOS binaries on tag push (`v*`)
- Both workflows set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true` to silence Node.js deprecation warnings
- golangci-lint version: **v2.1.6**, action: **golangci-lint-action@v7**, install-mode: **goinstall**
