//go:build !linux && !windows

package notifier

import "github.com/vitorhugo-java/organizerv2/internal/config"

func newPlatform(_ config.NotificationConfig) Notifier {
	return NoopNotifier{}
}
