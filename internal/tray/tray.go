package tray

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/getlantern/systray"
	"github.com/sebastianstupak/m2a/internal/syncer"
)

//go:embed assets/icon.png
var iconBytes []byte

// StatusUpdate carries sync results to the tray for display.
type StatusUpdate struct {
	Time   time.Time
	Result syncer.Result
	Err    error
}

// Run starts the system tray. Blocks until the user selects Quit.
// forceSyncCh: send struct{}{} to trigger a manual sync.
// updateCh: receive StatusUpdate to refresh the tray menu.
// quitCh: closed when the user selects Quit.
func Run(forceSyncCh chan<- struct{}, updateCh <-chan StatusUpdate, quitCh chan<- struct{}) {
	systray.Run(
		func() { onReady(forceSyncCh, updateCh, quitCh) },
		func() {},
	)
}

func onReady(forceSyncCh chan<- struct{}, updateCh <-chan StatusUpdate, quitCh chan<- struct{}) {
	systray.SetIcon(iconBytes)
	systray.SetTitle("m2a")
	systray.SetTooltip("musescore2ableton — watching")

	mStatus := systray.AddMenuItem("Idle", "")
	mStatus.Disable()
	systray.AddSeparator()

	mForce := systray.AddMenuItem("↺  Force Sync", "Sync now regardless of file change")
	mStop := systray.AddMenuItem("■  Stop watching", "Pause the file watcher")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("✕  Quit", "Exit m2a")

	watching := true

	go func() {
		for {
			select {
			case update, ok := <-updateCh:
				if !ok {
					return
				}
				refreshMenu(mStatus, update)

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
				} else {
					mStop.SetTitle("▶  Resume watching")
					systray.SetTooltip("musescore2ableton — paused")
				}

			case <-mQuit.ClickedCh:
				close(quitCh)
				systray.Quit()
				return
			}
		}
	}()
}

func refreshMenu(mStatus *systray.MenuItem, u StatusUpdate) {
	t := u.Time.Format("15:04:05")
	if u.Err != nil {
		mStatus.SetTitle(fmt.Sprintf("✗  Error at %s: %s", t, u.Err))
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
		mStatus.SetTitle(fmt.Sprintf("⚠  %s — %d conflict(s), %d updated", t, conflicts, updated))
	} else {
		mStatus.SetTitle(fmt.Sprintf("✓  %s — %d track(s) updated", t, updated))
	}
}
