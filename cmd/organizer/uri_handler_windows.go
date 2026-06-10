//go:build windows

package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func init() {
	// When invoked as a URI handler (toast button click), handle and exit immediately
	// before Cobra parses any commands.
	if len(os.Args) >= 2 && strings.HasPrefix(os.Args[1], "organizerv2://") {
		handleURIInvocation(os.Args[1])
		os.Exit(0)
	}
	if err := ensureURIScheme(); err != nil {
		log.Printf("[uri] failed to register URI scheme: %v", err)
	}
}

func handleURIInvocation(rawURI string) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return
	}
	if u.Host != "open-location" {
		return
	}
	filePath := u.Query().Get("path")
	if filePath == "" {
		return
	}
	ps := fmt.Sprintf(`Start-Process explorer.exe -ArgumentList '/select,"%s"'`, filePath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	_ = cmd.Start()
}

// ensureURIScheme registers organizerv2:// in HKCU so toast protocol actions
// can invoke this executable with the URI as the first argument.
func ensureURIScheme() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`SOFTWARE\Classes\organizerv2`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("", "URL:OrganizerV2 Protocol"); err != nil {
		return err
	}
	if err := k.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER,
		`SOFTWARE\Classes\organizerv2\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmdKey.Close()
	return cmdKey.SetStringValue("", fmt.Sprintf(`"%s" "%%1"`, exe))
}
