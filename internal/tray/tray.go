package tray

import (
	_ "embed"
	"fmt"
	"time"

	"fyne.io/systray"
	"github.com/sebastianstupak/m2a/internal/syncer"
)

//go:embed assets/icon.ico
var iconBytes []byte

// StatusUpdate carries sync results and project context to the tray for display.
type StatusUpdate struct {
	Time            time.Time
	Result          syncer.Result
	Err             error
	ProjectName     string // active project name; empty = no project detected
	AbletonMismatch string // non-empty = Ableton has a different project open
}

// Run starts the system tray. Blocks until the user selects Quit.
// forceSyncCh: send struct{}{} to trigger a manual sync.
// updateCh: receive StatusUpdate to refresh the tray menu.
// quitCh: closed when the user selects Quit.
// pauseCh: send true to pause file-event syncing, false to resume.
// switchableProjects: channel sending names of projects available to switch to; nil disables switch submenu.
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
