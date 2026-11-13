// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

//go:build !linux && !darwin

package hostload

import (
	"errors"
	"runtime"
)

// readMemoryTotal has no source on platforms other than Linux and macOS, and
// says so.
func readMemoryTotal() (uint64, error) {
	return 0, errors.New("the total memory has no reader on " + runtime.GOOS)
}

// readMemoryAvailable has no source on platforms other than Linux and macOS,
// and says so.
func readMemoryAvailable() (uint64, error) {
	return 0, errors.New("the available memory has no reader on " + runtime.GOOS)
}
