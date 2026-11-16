// Design: docs/architecture/appliance/on-device-installer.md -- the project-quota kernel calls
// Overview: quota.go -- the package, its states and its not-implemented answer
// Related: quota_other.go -- the answers on a system that is not Linux

//go:build linux

package projquota

// Probe answers whether the filesystem that holds path enforces project quota.
//
// It is a wiring stub: it makes no kernel call and answers StateUnknown, with
// the reason that the probe is not implemented.
func Probe(_ string) Status {
	return Status{State: StateUnknown, Reason: ErrNotImplemented.Error()}
}

// AssignProject gives the empty folder dir the project id projectID, which new
// entries under it inherit.
//
// It is a wiring stub: it makes no kernel call and answers ErrNotImplemented.
func AssignProject(_ string, _ uint32) error {
	return ErrNotImplemented
}

// SetLimit sets limitBytes as the hard block limit of projectID on the
// filesystem that holds path.
//
// It is a wiring stub: it makes no kernel call and answers ErrNotImplemented.
func SetLimit(_ string, _ uint32, _ uint64) error {
	return ErrNotImplemented
}

// ReadUsage reads what projectID uses and may use on the filesystem that
// holds path.
//
// It is a wiring stub: it makes no kernel call and answers ErrNotImplemented.
func ReadUsage(_ string, _ uint32) (Usage, error) {
	return Usage{}, ErrNotImplemented
}
