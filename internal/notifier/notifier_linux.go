//go:build linux

package notifier

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

// linuxNotifier uses notify-send for banners and xclip/xdotool/wl-copy for
// clipboard. Interactive action buttons are not supported by notify-send
// without a full D-Bus client; instead the notification body lists the
// destination path and the Copy Path action writes to the clipboard silently.
type linuxNotifier struct {
	cfg             config.NotificationConfig
	notifySendAvail bool
	clipboardAvail  bool
	clipboardCmd    string // "xclip", "xsel", or "wl-copy"
	mu              sync.Mutex
}

func newPlatform(cfg config.NotificationConfig) Notifier {
	n := &linuxNotifier{cfg: cfg}
	_, err := exec.LookPath("notify-send")
	n.notifySendAvail = err == nil
	if !n.notifySendAvail {
		log.Println("[notifier] notify-send not found; desktop notifications disabled")
	}
	// Detect clipboard tool (prefer Wayland, fall back to X11).
	for _, tool := range []string{"wl-copy", "xclip", "xsel"} {
		if _, err := exec.LookPath(tool); err == nil {
			n.clipboardCmd = tool
			n.clipboardAvail = true
			break
		}
	}
	if !n.clipboardAvail {
		log.Println("[notifier] no clipboard tool found (wl-copy/xclip/xsel); Copy Path will log instead")
	}
	return n
}

func (n *linuxNotifier) Notify(event FileEvent) error {
	go n.deliver(event)
	return nil
}

func (n *linuxNotifier) deliver(event FileEvent) {
	filename := filepath.Base(event.Destination)
	body := fmt.Sprintf("Moved to %s/", event.Category)

	// Copy to clipboard before blocking on the notification.
	if n.cfg.Actions.CopyPath {
		n.copyToClipboard(event.Destination)
	}

	if n.notifySendAvail && n.cfg.Enabled {
		args := []string{
			"OrganizerV2",
			fmt.Sprintf("%s\n%s", filename, body),
			"--icon=folder",
			"--expire-time=5000",
		}
		if n.cfg.Actions.OpenLocation {
			// --action requires notify-send ≥ 0.7.9 (libnotify). On older
			// daemons the notification still shows, just without the button.
			args = append(args, "--action=open-folder:Open Folder")
		}
		cmd := exec.Command("notify-send", args...)
		out, err := cmd.Output()
		if err != nil {
			log.Printf("[notifier] notify-send error: %v", err)
			return
		}
		if n.cfg.Actions.OpenLocation && strings.TrimSpace(string(out)) == "open-folder" {
			n.openLocation(event.Destination)
		}
	}
}

func (n *linuxNotifier) copyToClipboard(path string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.clipboardAvail {
		log.Printf("[notifier] destination path: %s", path)
		return
	}
	var cmd *exec.Cmd
	switch n.clipboardCmd {
	case "wl-copy":
		cmd = exec.Command("wl-copy", path)
	case "xclip":
		cmd = exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(path)
	case "xsel":
		cmd = exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(path)
	}
	if err := cmd.Run(); err != nil {
		log.Printf("[notifier] clipboard copy error: %v", err)
	}
}

func (n *linuxNotifier) openLocation(filePath string) {
	// Try the standard D-Bus FileManager1 interface first; it opens the file
	// manager and selects the specific file (supported by Nautilus, Thunar,
	// Dolphin, Nemo, etc.).
	uri := "file://" + filePath
	if _, err := exec.LookPath("dbus-send"); err == nil {
		cmd := exec.Command("dbus-send", "--session", "--print-reply",
			"--dest=org.freedesktop.FileManager1",
			"/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems",
			"array:string:"+uri,
			"string:",
		)
		if err := cmd.Run(); err == nil {
			return
		}
	}
	// Fall back to opening the parent directory without selection.
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return
	}
	cmd := exec.Command("xdg-open", filepath.Dir(filePath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] xdg-open error: %v", err)
	}
}

func (n *linuxNotifier) Close() error { return nil }
