// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectCPU on non-Linux platforms returns ErrUnsupported. Callers of
// the package-level Detect() receive an Inventory with CPU=nil and no
// entry in Errors (Detect treats ErrUnsupported as "section not
// available" rather than a detection failure).
func (d *Detector) DetectCPU() (*CPUInfo, error) {
	return nil, ErrUnsupported
}
