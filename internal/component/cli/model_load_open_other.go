//go:build !linux && !darwin && !freebsd

// Design: docs/guide/config-editor.md -- refuse imports without a safe file opener.

package cli

import (
	"errors"
	"os"
)

func openLoadFile(_ string) (*os.File, error) {
	return nil, errors.New("safe daemon configuration file reads are unsupported on this OS")
}
