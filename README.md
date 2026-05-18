# m2a — musescore2ableton

Watches a MuseScore Studio project file and syncs it into a running Ableton Live session on every save. Each MuseScore instrument becomes a named MIDI track in Ableton. Conflict detection prevents silent overwrites when you've edited both sides.

Compiles to a single static binary. No runtime dependencies beyond AbletonOSC.

---

## Requirements

- **MuseScore 4** — [musescore.org](https://musescore.org)
- **Ableton Live 11 or 12** with [AbletonOSC](https://github.com/ideoforms/AbletonOSC) installed as a MIDI Remote Script
- **Windows 10/11** (primary target; Linux and macOS build but are untested)

---

## Installation

### Download binary

Grab the latest release from [GitHub Releases](https://github.com/sebastianstupak/musescore2ableton/releases) and place `m2a.exe` somewhere on your `PATH`.

### Build from source

```sh
go install github.com/sebastianstupak/m2a/cmd/m2a@latest
```

Requires Go 1.24+. On Linux: `gcc libgtk-3-dev libayatana-appindicator3-dev` for the tray.

---

## Setup

### 1. Install AbletonOSC

Download [AbletonOSC](https://github.com/ideoforms/AbletonOSC/archive/refs/heads/master.zip), extract the zip, and copy the entire `AbletonOSC-master` folder (renamed to `AbletonOSC`) into your Ableton User Remote Scripts directory:

**Windows:**
```
C:\Users\<you>\Documents\Ableton\User Library\Remote Scripts\AbletonOSC\
```

The folder must contain `__init__.py`, `abletonosc\`, and `pythonosc\` at the top level.

### 2. Enable AbletonOSC in Ableton

1. Fully restart Ableton Live
2. Open **Preferences** (`Ctrl+,`) → **Link/Tempo/MIDI** tab
3. In the first **Control Surface** dropdown, select **AbletonOSC**
4. Ableton should show: *"AbletonOSC: Listening for OSC on port 11000"*

### 3. Create a config file

Create `m2a.yml` (or anywhere, pass path via `--config`):

```yaml
score: C:\path\to\MySong.mscz
```

All other fields are optional — defaults cover a standard local setup:

```yaml
score: C:\path\to\MySong.mscz              # required
musescore_bin: C:\Program Files\MuseScore 4\bin\MuseScore4.exe
ableton_osc_host: 127.0.0.1
ableton_osc_port: 11000
ableton_recv_port: 11001
debounce_ms: 500
```

---

## Usage

```sh
# Watch the score file and sync on every save (shows tray icon)
m2a watch --config m2a.yml

# One-shot sync without watching or tray
m2a sync --config m2a.yml

# Reset saved state (forces a full re-sync next time)
m2a reset --config m2a.yml
```

After `m2a watch` starts, a green **M** icon appears in the system tray (overflow `^` area on Windows 11). Right-click it for Force Sync, Stop watching, and Quit.

Logs are written to `%LOCALAPPDATA%\m2a\m2a.log`.

---

## How it works

On every save of your `.mscz` file:

1. MuseScore exports the score to a temp MIDI file
2. m2a parses the MIDI into tracks with notes, tempos, and time signatures
3. For each track, m2a compares the new state against the last synced snapshot and the current Ableton clip:

| MuseScore changed | Ableton changed | Result |
|---|---|---|
| No | No | Skip |
| Yes | No | Update Ableton clip in place |
| No | Yes | Skip (preserve your Ableton edits) |
| Yes | Yes | **Conflict** — new track `"Name (CHANGES)"` created above, colored red |

4. State is saved to `.m2a_state.json` beside the `.mscz` file

---

## Conflict resolution

When m2a detects a conflict (you edited both the score and the Ableton clip since the last sync), it creates a new MIDI track named `"TrackName (CHANGES)"` directly above the existing track and colors it red. Your original Ableton clip is left untouched. Review the two tracks, decide which to keep, then run `m2a reset` and sync again.

---

## Limitations

- Notes, tempos, and time signatures are synced. Dynamics, articulations, and score metadata are not yet supported.
- One-way only: MuseScore → Ableton. Changes in Ableton are never written back to the score.
- One score at a time per instance.

---

## License

MIT
