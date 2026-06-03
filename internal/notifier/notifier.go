package notifier

import "github.com/vitorhugo-java/organizerv2/internal/config"

// FileEvent carries information about a file that was organised.
type FileEvent struct {
	// Source is the original absolute path before moving.
	Source string
	// Destination is the absolute path after moving.
	Destination string
	// Category is the category folder the file was moved into.
	Category string
}

// Notifier sends user-visible notifications about organised files.
type Notifier interface {
	// Notify delivers a notification for the given event. It must not block the
	// caller for more than a short moment; implementations should do heavy work
	// in a goroutine.
	Notify(event FileEvent) error
	// Close releases any resources held by the notifier.
	Close() error
}

// NoopNotifier implements Notifier but does nothing. Use it in tests or when
// notifications are disabled.
type NoopNotifier struct{}

func (NoopNotifier) Notify(_ FileEvent) error { return nil }
func (NoopNotifier) Close() error             { return nil }

// New returns a platform-specific Notifier. If cfg.Enabled is false it returns
// a NoopNotifier. The platform-specific construction is in the build-tagged
// files notifier_linux.go and notifier_windows.go.
func New(cfg config.NotificationConfig) Notifier {
	if !cfg.Enabled {
		return NoopNotifier{}
	}
	return newPlatform(cfg)
}
