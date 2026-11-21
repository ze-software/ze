// Design: docs/architecture/host/inventory.md -- hardware inventory detection

//go:build !linux

package host

import "io/fs"

// DetectMemory on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectMemory() (*MemoryInfo, error) {
	return nil, ErrUnsupported
}

// DetectKernelMemory on non-Linux platforms answers ErrUnsupported, wrapped in
// the *fs.PathError its Linux form answers for every failure, so the
// kernel-memory rule names the file it would have read.
func (d *Detector) DetectKernelMemory() (KernelMemory, error) {
	return KernelMemory{}, &fs.PathError{Op: "read", Path: "/proc/meminfo", Err: ErrUnsupported}
}
