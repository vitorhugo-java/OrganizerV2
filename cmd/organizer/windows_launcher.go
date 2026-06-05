//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
)

func init() {
	// Disable Cobra's built-in mousetrap message so we control the UX.
	cobra.MousetrapHelpText = ""

	if mousetrap.StartedByExplorer() {
		fmt.Fprintln(os.Stderr, "OrganizerV2 is a command-line tool.")
		fmt.Fprintln(os.Stderr, "Open Command Prompt (cmd.exe) or PowerShell and run it from there.")
		fmt.Fprintln(os.Stderr, "\n  organizer --help   show available commands")
		fmt.Fprintln(os.Stderr, "  organizer start    start the file-watcher daemon")
		fmt.Fprintln(os.Stderr, "  organizer scan     one-shot scan and organize")
		fmt.Fprintln(os.Stderr, "\nPress Enter to exit...")
		fmt.Scanln()
		os.Exit(0)
	}
}
