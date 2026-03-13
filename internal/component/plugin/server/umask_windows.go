// Design: (none -- platform-specific umask stub for Windows)

//go:build windows

package server

// setUmask is a no-op on Windows (no umask concept).
// Socket permissions on Windows are handled differently.
func setUmask(_ int) int {
	return 0
}
