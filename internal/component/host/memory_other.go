// Design: plan/learned/631-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectMemory on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectMemory() (*MemoryInfo, error) {
	return nil, ErrUnsupported
}
