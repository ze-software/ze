// Design: (none -- platform-specific umask for socket creation)

//go:build !windows

package server

import "syscall"

// setUmask sets the process umask and returns the previous value.
// Used to create Unix sockets with restrictive permissions from the start,
// eliminating the TOCTOU window between Listen() and Chmod().
func setUmask(mask int) int {
	return syscall.Umask(mask)
}
