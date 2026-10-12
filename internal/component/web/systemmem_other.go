// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build !darwin && !linux && !freebsd && !openbsd

package web

import (
	"errors"
	"runtime"
)

// totalSystemMemory has no source on this operating system, and says so rather
// than answering zero.
func totalSystemMemory() (uint64, error) {
	return 0, errors.New("total system memory is not implemented on " + runtime.GOOS)
}
