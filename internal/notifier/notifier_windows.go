//go:build windows

package notifier

import (
	"fmt"
	"log"
	"net/url"
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
		// Clicking this button invokes the app as a URI handler, which runs
		// PowerShell Start-Process explorer.exe /select,"<path>" so the file
		// is pre-selected in Explorer.
		actionURI := "organizerv2://open-location?path=" + url.QueryEscape(event.Destination)
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      "protocol",
			Label:     "Open Folder",
			Arguments: actionURI,
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

	if n.cfg.Actions.CopyPath && n.clipboardInit {
		clipboard.Write(clipboard.FmtText, []byte(event.Destination))
	}
}

func (n *windowsNotifier) Close() error { return nil }
