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

	if n.notifySendAvail && n.cfg.Enabled {
		args := []string{
			"OrganizerV2",
			fmt.Sprintf("%s\n%s", filename, body),
			"--icon=folder",
			"--expire-time=5000",
		}
		cmd := exec.Command("notify-send", args...)
		if err := cmd.Run(); err != nil {
			log.Printf("[notifier] notify-send error: %v", err)
		}
	}

	// Execute actions silently in the background.
	if n.cfg.Actions.CopyPath {
		n.copyToClipboard(event.Destination)
	}
	if n.cfg.Actions.OpenLocation {
		n.openLocation(filepath.Dir(event.Destination))
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

func (n *linuxNotifier) openLocation(dir string) {
	// xdg-open opens the folder in the default file manager.
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return
	}
	cmd := exec.Command("xdg-open", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] xdg-open error: %v", err)
	}
}

func (n *linuxNotifier) Close() error { return nil }
