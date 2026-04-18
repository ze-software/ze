// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build !linux

package host

// DetectStorage on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectStorage() (*StorageInfo, error) {
	return nil, ErrUnsupported
}
