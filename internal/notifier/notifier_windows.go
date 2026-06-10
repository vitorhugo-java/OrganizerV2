//go:build windows

package notifier

import (
	"fmt"
	"log"
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
	if cfg.Actions.CopyPath {
		if err := clipboard.Init(); err != nil {
			log.Printf("[notifier] clipboard init failed: %v", err)
		} else {
			n.clipboardInit = true
		}
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
	}

	if n.cfg.Actions.OpenFile {
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      "protocol",
			Label:     "Open File",
			Arguments: event.Destination,
		})
	}
	if n.cfg.Actions.OpenLocation {
		// Button opens the parent folder (file:// URI); the auto-open below
		// also runs immediately with file selected via PowerShell.
		folderURI := "file:///" + filepath.ToSlash(filepath.Dir(event.Destination))
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      "protocol",
			Label:     "Open Folder",
			Arguments: folderURI,
		})
	}
	if n.cfg.Actions.Confirm {
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      "protocol",
			Label:     "OK",
			Arguments: "",
		})
	}

	if err := notification.Push(); err != nil {
		log.Printf("[notifier] toast push error: %v", err)
	}

	// Auto-open folder with the file selected right after the notification fires.
	if n.cfg.Actions.OpenLocation {
		n.openLocation(event.Destination)
	}

	if n.cfg.Actions.CopyPath && n.clipboardInit {
		clipboard.Write(clipboard.FmtText, []byte(event.Destination))
	}
}

func (n *windowsNotifier) openLocation(filePath string) {
	// Use PowerShell Start-Process so the path is passed as a proper argument,
	// which handles spaces and special characters correctly.
	ps := fmt.Sprintf(`Start-Process explorer.exe -ArgumentList '/select,"%s"'`, filePath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] explorer error: %v", err)
	}
}

func (n *windowsNotifier) Close() error { return nil }
