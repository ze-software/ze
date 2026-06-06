// Design: docs/architecture/testing/ci-format.md — ze-test shared helpers

//go:build ze_test

package main

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	errTestsFailed         = errors.New("tests failed")
	errCouldNotFindGoModIn = errors.New("could not find go.mod in parent directories")
)

func findBaseDir() (string, error) {
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

	return "", errCouldNotFindGoModIn
}
