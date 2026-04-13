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
