//go:build windows

package main

import (
	"os"
	"syscall"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
)

var (
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	freeConsole = kernel32.NewProc("FreeConsole")
)

func init() {
	// Disable Cobra's built-in mousetrap message so we control the UX.
	cobra.MousetrapHelpText = ""

	if !mousetrap.StartedByExplorer() {
		return
	}

	if hasArg("--show-terminal") {
		// Terminal requested explicitly: run normally with full output.
		return
	}

	// Launched by double-click without --show-terminal: detach from the
	// console so no window appears. The process continues running silently
	// in the background (useful for the file-watcher daemon).
	freeConsole.Call()
}

// hasArg reports whether flag is present in os.Args.
func hasArg(flag string) bool {
	for _, a := range os.Args[1:] {
		if a == flag {
			return true
		}
	}
	return false
}
