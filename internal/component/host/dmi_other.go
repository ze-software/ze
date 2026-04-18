// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectDMI on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectDMI() (*DMIInfo, error) {
	return nil, ErrUnsupported
}
