// Design: plan/learned/631-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectThermal on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectThermal() (*ThermalInfo, error) {
	return nil, ErrUnsupported
}
