// Design: docs/architecture/core-design.md -- sysctl plugin backend
// Detail: backend_linux.go -- Linux /proc/sys backend
// Detail: backend_darwin.go -- Darwin sysctlbyname backend
// Detail: backend_other.go -- no-op backend

package sysctl

// backend abstracts OS-specific sysctl read and write operations.
// Implementations are selected at compile time via build tags.
type backend interface {
	// read returns the current value of a sysctl key from the kernel.
	read(key string) (string, error)
	// write sets a sysctl key to the given value in the kernel.
	write(key, value string) error
}

// Read answers the kernel's current value of one sysctl key, trimmed, for a
// component that reports a tunable it does not manage. It is the one exported
// read: the key-to-path mapping stays declared once, in the platform backend,
// so a reader never writes a second copy of it. The value is what the kernel
// holds now, not the value this component last applied. A platform with no
// backend, or a key the kernel does not hold, answers an error rather than an
// empty string. Safe for concurrent use.
func Read(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	return newBackend().read(key)
}
