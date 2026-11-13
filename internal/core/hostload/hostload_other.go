// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

//go:build !linux && !darwin

package hostload

import (
	"errors"
	"runtime"
)

// readLoadAverages has no source on platforms other than Linux and macOS, and
// says so. Snapshot turns that into the zero load its contract documents, so
// Contended() reports false there and the package still builds and runs
// everywhere the development tooling does.
func readLoadAverages() (Averages, error) {
	return Averages{}, errors.New("the load averages have no reader on " + runtime.GOOS)
}
