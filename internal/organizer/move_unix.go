//go:build !windows

package organizer

import (
	"errors"
	"syscall"
)

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EXDEV
	}
	return false
}
