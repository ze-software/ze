// Design: docs/architecture/core-design.md -- route subscription stub (non-Linux)

//go:build !linux

package routewatch

func (w *Watcher) subscribe() {
	<-w.stopCh
}
