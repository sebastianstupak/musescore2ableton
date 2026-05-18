# musescore2ableton — Design Spec

**Date:** 2026-05-18  
**Status:** Approved

---

## Overview

A Go CLI tool that watches a MuseScore Studio project file (`.mscz`) and syncs changes into a running Ableton Live session on every save. Each MuseScore instrument maps to a named MIDI track in Ableton. The tool detects conflicts (when both sides have diverged since the last sync) and creates a labeled conflict track rather than silently overwriting Ableton edits.

Compiles to a single static binary. No runtime dependencies required beyond AbletonOSC installed as a MIDI Remote Script in Live.

---

## Goals

- Sync on every explicit MuseScore save (Ctrl+S), not autosave
- Initial sync scope: MIDI notes, tempo changes, time signature changes
- Designed to be extended: dynamics, articulations, score metadata (title, composer, etc.) can be added later without restructuring
- Support Ableton Live 11 Suite and Live 12 Suite (same API surface)
- Windows 11 primary target; architecture is cross-platform

---

## Non-Goals

- Audio export / audio sync
- Two-way sync (Ableton → MuseScore)
- MuseScore plugin; this is a standalone external tool
- Real-time note-by-note streaming

---

## Tech Stack

| Concern | Library |
|---|---|
| Language | Go |
| File watching | `github.com/fsnotify/fsnotify` |
| MIDI parsing | `gitlab.com/gomidi/midi/v2` |
| OSC messaging | `github.com/hypebeast/go-osc` |
| Config parsing | `gopkg.in/yaml.v3` |
| JSON state | `encoding/json` (stdlib) |
| MuseScore export | `os/exec` → `MuseScore4.exe -o` |
| System tray | `github.com/getlantern/systray` |

---

## Configuration — `m2a.yml`

Placed in the working directory or passed via `--config` flag.

```yaml
score: "C:/path/to/MySong.mscz"
musescore_bin: "C:/Program Files/MuseScore4/bin/MuseScore4.exe"
ableton_osc_host: "127.0.0.1"
ableton_osc_port: 11000
debounce_ms: 500
```

All fields except `score` have sensible defaults so a minimal config is just the score path.

---

## Data Flow

```
MuseScore file saved (Ctrl+S)
  → watcher detects write event on .mscz (fsnotify)
  → debounce 500ms (MuseScore may flush in multiple writes)
  → exporter: MuseScore4.exe -o %TEMP%/m2a_sync.mid score.mscz
  → parser: m2a_sync.mid → []Track
  → state: load .m2a_state.json (last-synced snapshot)
  → for each Track:
      → ableton: read current clip notes via AbletonOSC
      → diff A: new MIDI track vs last-synced snapshot  (MuseScore delta)
      → diff B: current Ableton clip vs last-synced snapshot (Ableton delta)
      → conflict? (both A and B non-empty)
          → create new track above existing, name = original + " (CHANGES)", color red
          → write new MuseScore notes into new conflict track
      → no conflict, MuseScore changed:
          → clear existing Ableton clip
          → write new notes into existing clip
      → no change on either side: skip
  → state: save new snapshot to .m2a_state.json
  → log: print sync summary (tracks updated, conflicts, skipped)
```

---

## Module Layout

```
cmd/musescore2ableton/
  main.go                ← CLI entry: parse flags, load config, wire modules, start watch loop

internal/
  watcher/
    watcher.go           ← fsnotify wrapper with debounce; emits on channel when score file changes
  exporter/
    exporter.go          ← runs MuseScore4.exe via os/exec, returns temp MIDI path
  parser/
    parser.go            ← parses MIDI file → []Track
    types.go             ← Track, Note, TempoEvent, TimeSigEvent structs
  state/
    state.go             ← load/save SyncState JSON, compute Track diffs
    types.go             ← SyncState struct (map[trackName]TrackSnapshot)
  ableton/
    client.go            ← AbletonOSC UDP client, wraps go-osc
    tracks.go            ← read track list, find track by name, create track
    clips.go             ← read clip notes, clear clip, write notes, set color/name
  config/
    config.go            ← load m2a.yml, apply defaults, validate
  tray/
    tray.go              ← system tray icon, menu, status updates, force sync trigger
```

---

## Core Types

```go
// parser/types.go
type Note struct {
    Pitch    uint8
    Velocity uint8
    StartBeat float64  // beats from clip start
    Duration  float64  // beats
}

type TempoEvent struct {
    Tick  uint32
    BPM   float64
}

type TimeSigEvent struct {
    Tick        uint32
    Numerator   uint8
    Denominator uint8
}

type Track struct {
    Name       string
    Notes      []Note
    Tempos     []TempoEvent
    TimeSigs   []TimeSigEvent
}

// state/types.go
type TrackSnapshot struct {
    Notes    []Note
    Tempos   []TempoEvent
    TimeSigs []TimeSigEvent
}

type SyncState struct {
    Version   int                       `json:"version"`
    ScorePath string                    `json:"score_path"`
    SyncedAt  time.Time                 `json:"synced_at"`
    Tracks    map[string]TrackSnapshot  `json:"tracks"`
}
```

---

## Conflict Detection

Two diffs are computed per track:

- **MuseScore delta:** `newMIDITrack` vs `lastSnapshot.Tracks[name]`
- **Ableton delta:** `currentAbletonClip` vs `lastSnapshot.Tracks[name]`

A diff is "non-empty" if any note's pitch, velocity, start, or duration changed, or if any tempo/timesig event changed.

| MuseScore delta | Ableton delta | Action |
|---|---|---|
| empty | empty | skip |
| non-empty | empty | update Ableton clip in place |
| empty | non-empty | skip (Ableton edit preserved) |
| non-empty | non-empty | **conflict** → create `"TrackName (CHANGES)"` track above, color red, write new MuseScore notes there |

---

## AbletonOSC Command Mapping

| Operation | OSC address |
|---|---|
| List tracks | `/live/song/get/track_names` |
| Get clip notes | `/live/clip/get/notes` |
| Create MIDI track | `/live/song/create_midi_track` |
| Create clip | `/live/clip_slot/create_clip` |
| Add notes to clip | `/live/clip/add/notes` |
| Remove all notes | `/live/clip/remove/notes` |
| Set track name | `/live/track/set/name` |
| Set track color | `/live/track/set/color` |
| Set song tempo | `/live/song/set/tempo` |

---

## State File

`.m2a_state.json` is written beside the `.mscz` file. It stores the last successfully synced state so both deltas can be computed on the next save. If the file is missing (first run), the tool treats it as a fresh sync — no conflict detection, just write all tracks.

---

## Error Handling

- MuseScore export fails (binary not found, parse error): log error, skip sync cycle, do not update state
- AbletonOSC unreachable: log error with hint ("Is Live running? Is AbletonOSC installed?"), retry on next save
- Partial sync failure (one track fails): log per-track error, continue remaining tracks, save partial state only for succeeded tracks
- Malformed state file: treat as missing (fresh sync), log warning

---

## System Tray UI

The binary runs as a system tray application (Windows notification area). No separate install — the tray is part of the same binary as the watcher.

**Tray menu:**
```
m2a  ·  Last sync: 14:32:01
─────────────────────────────
✓  Piano   — updated
✓  Violin  — updated
⚠  Cello   — CONFLICT
─────────────────────────────
↺  Force Sync
■  Stop watching
✕  Quit
```

**Tray icon states:**
- Grey — idle / watching
- Green pulse — sync in progress
- Green — last sync succeeded
- Orange — last sync had conflicts
- Red — last sync errored

**Force Sync:** runs the full sync pipeline immediately, identical to a file-save trigger. Useful when Ableton wasn't running during a save and needs to catch up.

The tray reads sync results from a shared in-memory channel fed by the sync pipeline. No status file needed.

---

## CLI Usage

```
# Start watching (launches tray icon)
m2a watch --config m2a.yml

# One-shot sync (no tray, no watch — useful for scripting)
m2a sync --config m2a.yml

# Reset state (forces fresh sync next time)
m2a reset --config m2a.yml
```

---

## Extensibility Notes

Adding new metadata (e.g. dynamics, articulations, title/composer) requires:
1. Extend `Track` struct in `parser/types.go`
2. Extend `parser.go` to extract from MIDI or MSCX XML
3. Extend `TrackSnapshot` in `state/types.go` for diffing
4. Extend `ableton/clips.go` to write the new data via OSC

The sync loop and conflict detection logic in `main.go` do not need to change.

---

## Out of Scope (future)

- `--dry-run` flag (log what would change without writing to Ableton)
- Watch multiple scores simultaneously
- Score metadata sync (title, composer → clip/track names)
- Dynamics → MIDI velocity curves
- Articulations → note length/velocity adjustments
- Desktop notification on conflict
