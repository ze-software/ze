// Design: (none — new utility for binary-relative path resolution)

package paths

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})

// isBinDir returns true if the directory basename is a standard binary directory.
func isBinDir(name string) bool {
	return name == "bin" || name == "sbin"
}

// ConfigDirFromBinary returns the config directory for ze based on the binary path.
// The resolution follows GNU prefix conventions:
//
//   - /usr/bin/ze, /bin/ze, /sbin/ze, /usr/sbin/ze → /etc/ze
//   - /user/ze (gokrazy) → /perm/ze
//   - /opt/app/bin/ze → /opt/app/etc/ze
//   - ./bin/ze → etc/ze (relative)
//   - unknown layout → "" (caller must provide explicit config path)
//
// This is a pure mapping and deliberately ignores the ze.config.dir override: it
// answers "where would the config for THIS binary live", which the installers ask
// about a binary they are placing, not about the running process. To resolve the
// running process's config dir, call DefaultConfigDir, which applies the override.
func ConfigDirFromBinary(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(dir)

	// Gokrazy places binaries in /user/; persistent storage is /perm/.
	if base == "user" && filepath.Dir(dir) == "/" {
		return "/perm/ze"
	}

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

// DefaultConfigDir resolves the config directory, preferring the ze.config.dir
// override and falling back to the running binary's location.
// Returns "" if the override is unset and the binary location cannot be
// determined or doesn't match a known layout.
//
// The override must be honored here rather than at each call site: a caller
// that reached for the binary-relative path alone resolved a store that `ze
// init` had never written (`ze data` did exactly that).
func DefaultConfigDir() string {
	if dir := env.Get("ze.config.dir"); dir != "" {
		return dir
	}

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
