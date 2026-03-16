// Design: docs/architecture/system-architecture.md — daemon privilege dropping
// Overview: drop.go — user/group resolution

//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

package privilege

// Drop is a no-op on platforms without Unix privilege model.
func Drop(_ DropConfig) error {
	return nil
}
