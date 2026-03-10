// Design: (none — new utility for binary-relative path resolution)

package paths

import (
	"os"
	"path/filepath"
)

// isBinDir returns true if the directory basename is a standard binary directory.
func isBinDir(name string) bool {
	return name == "bin" || name == "sbin"
}

// ConfigDirFromBinary returns the config directory for ze based on the binary path.
// The resolution follows GNU prefix conventions:
//
//   - /usr/bin/ze, /bin/ze, /sbin/ze, /usr/sbin/ze → /etc/ze
//   - /opt/app/bin/ze → /opt/app/etc/ze
//   - ./bin/ze → etc/ze (relative)
//   - unknown layout → "" (caller must provide explicit config path)
func ConfigDirFromBinary(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(dir)

	if !isBinDir(base) {
		return ""
	}

	prefix := filepath.Dir(dir)

	// System prefixes: /, /usr, /usr/local → config in /etc/ze.
	switch prefix {
	case "/", "/usr", "/usr/local":
		return "/etc/ze"
	}

	// Relative path (e.g., ./bin/ze or bin/ze) → etc/ze relative.
	if !filepath.IsAbs(binaryPath) {
		return "etc/ze"
	}

	// Arbitrary prefix (e.g., /opt/myapp/bin/ze → /opt/myapp/etc/ze).
	return filepath.Join(prefix, "etc", "ze")
}

// DefaultConfigDir resolves the config directory from the running binary's location.
// Returns "" if the binary location cannot be determined or doesn't match a known layout.
func DefaultConfigDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}

	return ConfigDirFromBinary(resolved)
}
