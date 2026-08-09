// Design: docs/architecture/diagnostics/crash-capture.md -- fd2 redirect stub for non-unix

//go:build !unix

package crashlog

import (
	"errors"
	"os"
)

var errNoDup2 = errors.New("crashlog: stderr redirect not supported on this platform")

func dupStderr(_ int) error {
	return errNoDup2
}

func saveStderr() *os.File {
	return nil
}
