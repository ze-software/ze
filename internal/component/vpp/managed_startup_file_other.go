//go:build !linux && !darwin

// Design: docs/guide/vpp.md -- explicit original-file capability refusal
package vpp

import (
	"errors"
	"os"
)

func openStartupCLIFile(path string) (startupCLIFile, error) {
	return nil, &os.PathError{Op: "open", Path: path, Err: errors.ErrUnsupported}
}
