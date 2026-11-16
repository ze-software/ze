// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: confinement_linux.go -- the Linux observation and judgment

//go:build !linux

package main

import (
	"errors"
	"fmt"
)

// observeConfinement needs Linux: a container runs only there.
func observeConfinement() (string, error) {
	return "", errors.New("the confinement facts are read on Linux only")
}

// confined needs Linux: a container runs only there.
func confined(_, _ string) bool {
	fmt.Println("probe: FAIL the confinement is judged on Linux only")
	return false
}
