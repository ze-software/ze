// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectKernel on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectKernel() (*KernelInfo, error) {
	return nil, ErrUnsupported
}

// DetectHost on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectHost() (*HostInfo, error) {
	return nil, ErrUnsupported
}
