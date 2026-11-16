// Design: docs/architecture/host/inventory.md -- hardware inventory detection

//go:build !linux

package host

// DetectStorage on non-Linux platforms returns ErrUnsupported.
func (d *Detector) DetectStorage() (*StorageInfo, error) {
	return nil, ErrUnsupported
}

// DetectPersistentStorage answers ErrNoPersistentStorage: an appliance runs
// Linux, so a system that does not mounts no persistent partition. That is the
// answer about this system, which is why it is not ErrUnsupported.
func (d *Detector) DetectPersistentStorage() (PersistentStorage, error) {
	return PersistentStorage{}, ErrNoPersistentStorage
}
