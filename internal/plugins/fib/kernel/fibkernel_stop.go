// Design: docs/architecture/core-design.md -- FIB shutdown admission.
// Related: fibkernel.go -- run owns cancellation and processEvent owns writes.

package fibkernel

// beginStop MUST close forwarding admission without waiting for the current
// backend operation. The shutdown worker MUST call it before joining readers.
func (f *fibKernel) beginStop() {
	f.stopped.Store(true)
}
