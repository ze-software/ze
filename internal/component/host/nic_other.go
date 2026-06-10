// Design: plan/learned/631-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectNICs on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectNICs() ([]NICInfo, error) {
	return nil, ErrUnsupported
}
