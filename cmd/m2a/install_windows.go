//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const taskName = "m2a"

func runInstall(cmd *cobra.Command, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	// Build a PowerShell one-liner that registers the scheduled task.
	// - AtLogOn trigger: starts when the current user logs in
	// - ExecutionTimeLimit 0: no timeout (tray app runs indefinitely)
	// - RestartCount 3, RestartInterval 1 min: auto-recover from crashes
	// - MultipleInstances IgnoreNew: don't spawn a second instance if one is running
	// - Force: overwrite any existing task with the same name
	script := fmt.Sprintf(
		`$a = New-ScheduledTaskAction -Execute '%s' -Argument 'watch'; `+
			`$t = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME; `+
			`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit 0 -RestartCount 3 `+
			`-RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew; `+
			`$p = New-ScheduledTaskPrincipal -UserId $env:USERDOMAIN\$env:USERNAME -RunLevel Limited; `+
			`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger $t -Settings $s -Principal $p `+
			`-Description 'musescore2ableton auto-sync' -Force | Out-Null; `+
			`Start-ScheduledTask -TaskName '%s'`,
		exe, taskName, taskName,
	)

	out, err := runPS(script)
	if err != nil {
		return fmt.Errorf("failed to register task: %w\n%s", err, out)
	}

	fmt.Printf("Installed: m2a will start automatically on login.\n")
	fmt.Printf("Task: %s\n", taskName)
	fmt.Printf("Binary: %s\n", exe)
	fmt.Printf("Logs: %%LOCALAPPDATA%%\\m2a\\m2a.log\n")
	fmt.Println("m2a watch is now running in the background.")
	return nil
}

func runUninstall(cmd *cobra.Command, args []string) error {
	// Stop first (ignore error — task may not be running)
	_, _ = runPS(fmt.Sprintf("Stop-ScheduledTask -TaskName '%s' -ErrorAction SilentlyContinue", taskName))

	out, err := runPS(fmt.Sprintf(
		"Unregister-ScheduledTask -TaskName '%s' -Confirm:$false", taskName,
	))
	if err != nil {
		if strings.Contains(string(out), "cannot find") || strings.Contains(string(out), "No MSFT") {
			return fmt.Errorf("task '%s' not found — already uninstalled?", taskName)
		}
		return fmt.Errorf("failed to unregister task: %w\n%s", err, out)
	}

	fmt.Printf("Uninstalled: m2a will no longer start on login.\n")
	return nil
}

func runPS(script string) ([]byte, error) {
	return exec.Command("powershell", "-NonInteractive", "-NoProfile", "-Command", script).CombinedOutput()
}
