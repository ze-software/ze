// Design: docs/architecture/host/inventory.md -- hardware inventory detection

//go:build !linux

package host

// detectThermal on non-Linux platforms returns ErrUnsupported.
func (d *Detector) detectThermal() (*ThermalInfo, error) {
	return nil, ErrUnsupported
}
