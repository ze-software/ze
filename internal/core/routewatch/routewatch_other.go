// Design: docs/architecture/core-design.md -- route subscription stub (non-Linux)

//go:build !linux

package routewatch

// platformState has no per-OS subscription state outside Linux.
type platformState struct{}

func newPlatformState() platformState { return platformState{} }

func (w *Watcher) captureNamespace() {}

func (w *Watcher) subscribe() {
	<-w.stopCh
}
