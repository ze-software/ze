// Design: docs/architecture/testing/ci-format.md -- ze-test shared helpers

package cli

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrTestsFailed         = errors.New("tests failed")
	ErrCouldNotFindGoModIn = errors.New("could not find go.mod in parent directories")
)

func FindBaseDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrCouldNotFindGoModIn
}

func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}
