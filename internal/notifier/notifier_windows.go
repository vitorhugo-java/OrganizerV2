//go:build windows

package notifier

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-toast/toast"
	"github.com/vitorhugo-java/organizerv2/internal/config"
	"golang.design/x/clipboard"
)

type windowsNotifier struct {
	cfg           config.NotificationConfig
	clipboardInit bool
}

func newPlatform(cfg config.NotificationConfig) Notifier {
	n := &windowsNotifier{cfg: cfg}
	if err := clipboard.Init(); err != nil {
		log.Printf("[notifier] clipboard init failed: %v", err)
	} else {
		n.clipboardInit = true
	}
	return n
}

func (n *windowsNotifier) Notify(event FileEvent) error {
	go n.deliver(event)
	return nil
}

func (n *windowsNotifier) deliver(event FileEvent) {
	filename := filepath.Base(event.Destination)

	notification := toast.Notification{
		AppID:   "OrganizerV2",
		Title:   "OrganizerV2",
		Message: fmt.Sprintf("%s → %s/", filename, event.Category),
		Actions: n.buildActions(event),
	}

	if err := notification.Push(); err != nil {
		log.Printf("[notifier] toast push error: %v", err)
	}

	// Copy Path is executed immediately since toast action callbacks require a
	// COM activator registration that is complex to set up. The path is copied
	// to the clipboard as part of the notification delivery.
	if n.cfg.Actions.CopyPath && n.clipboardInit {
		clipboard.Write(clipboard.FmtText, []byte(event.Destination))
	}
}

func (n *windowsNotifier) buildActions(event FileEvent) []toast.Action {
	var actions []toast.Action
	if n.cfg.Actions.OpenFile {
		actions = append(actions, toast.Action{
			Type:      "protocol",
			Label:     "Open File",
			Arguments: event.Destination,
		})
	}
	if n.cfg.Actions.OpenLocation {
		actions = append(actions, toast.Action{
			Type:      "protocol",
			Label:     "Open Location",
			Arguments: fmt.Sprintf("file:///%s", filepath.Dir(event.Destination)),
		})
	}
	if n.cfg.Actions.Confirm {
		actions = append(actions, toast.Action{
			Type:      "background",
			Label:     "Confirm",
			Arguments: "confirm",
		})
	}
	return actions
}

// openWithDefault opens a file using the Windows shell default application.
// Not used directly (toast handles it via protocol action), kept for scan output.
func openWithDefault(path string) {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] open error: %v", err)
	}
}

func (n *windowsNotifier) Close() error { return nil }
