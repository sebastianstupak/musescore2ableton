//go:build !windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func runInstall(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("m2a install is only supported on Windows (uses Task Scheduler)")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("m2a uninstall is only supported on Windows (uses Task Scheduler)")
}
