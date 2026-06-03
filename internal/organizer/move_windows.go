//go:build windows

package organizer

import (
	"errors"
	"syscall"
)

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		// ERROR_NOT_SAME_DEVICE = 17
		return errno == 17
	}
	return false
}
