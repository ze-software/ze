// Design: docs/architecture/appliance/on-device-installer.md -- no project quota off Linux
// Overview: quota.go -- the package and its states
// Related: quota_linux.go -- the kernel calls this file stands in for

//go:build !linux

package projquota

// Probe answers StateUnknown: no project quota can be read off Linux.
func Probe(_ string) Status {
	return Status{State: StateUnknown, Reason: ErrUnsupported.Error()}
}

// Enable answers ErrUnsupported.
func Enable(_ string) error {
	return ErrUnsupported
}

// AssignProject answers ErrUnsupported.
func AssignProject(_ string, _ uint32) error {
	return ErrUnsupported
}

// SetLimit answers ErrUnsupported.
func SetLimit(_ string, _ uint32, _ uint64) error {
	return ErrUnsupported
}

// ReadUsage answers ErrUnsupported.
func ReadUsage(_ string, _ uint32) (Usage, error) {
	return Usage{}, ErrUnsupported
}
