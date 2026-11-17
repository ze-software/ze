// Design: (none — new utility for binary-relative path resolution)

package paths

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})

// dataDirKey overrides the data directory: where Ze keeps artifacts too large
// for the state store, such as container images, layers and volumes.
const dataDirKey = "ze.data.dir"

var _ = env.MustRegister(env.EnvEntry{Key: dataDirKey, Type: "string", Description: "Override default data directory (container images, layers and volumes)"})

// isBinDir returns true if the directory basename is a standard binary directory.
func isBinDir(name string) bool {
	return name == "bin" || name == "sbin"
}

// binaryLayout is where a binary sits, which decides both where its config
// lives and where its data lives. Zero is layoutUnknown: a binary in no known
// layout, for which the caller must name the directory.
type binaryLayout uint8

const (
	layoutUnknown binaryLayout = iota
	// layoutGokrazy is the appliance: gokrazy places binaries in /user/, and
	// its persistent storage is /perm/.
	layoutGokrazy
	// layoutSystem is a bin or sbin folder under /, /usr or /usr/local.
	layoutSystem
	// layoutRelative is a relative bin or sbin folder, such as ./bin/ze.
	layoutRelative
	// layoutPrefix is a bin or sbin folder under any other absolute prefix.
	layoutPrefix
)

// classifyBinary answers the layout binaryPath sits in, and for layoutPrefix
// the prefix itself. It is the one place the layouts are recognized, so the
// config directory and the data directory of one binary cannot disagree
// about where it sits.
func classifyBinary(binaryPath string) (binaryLayout, string) {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(dir)

	if base == "user" && filepath.Dir(dir) == "/" {
		return layoutGokrazy, ""
	}

	if !isBinDir(base) {
		return layoutUnknown, ""
	}

	prefix := filepath.Dir(dir)

	switch prefix {
	case "/", "/usr", "/usr/local":
		return layoutSystem, ""
	}

	if !filepath.IsAbs(binaryPath) {
		return layoutRelative, ""
	}

	return layoutPrefix, prefix
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
	layout, prefix := classifyBinary(binaryPath)
	switch layout {
	case layoutGokrazy:
		return "/perm/ze"
	case layoutSystem:
		return "/etc/ze"
	case layoutRelative:
		return "etc/ze"
	case layoutPrefix:
		return filepath.Join(prefix, "etc", "ze")
	case layoutUnknown:
		return ""
	}
	panic("BUG: classifyBinary answered a layout ConfigDirFromBinary does not know")
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

// DataDirFromBinary returns the data directory for ze based on the binary
// path, beside the config directory ConfigDirFromBinary answers, and ignores
// the ze.data.dir override for the same reason it does:
//
//   - /usr/bin/ze, /bin/ze, /sbin/ze, /usr/sbin/ze → /var/lib/ze
//   - /user/ze (gokrazy) → /perm/var/lib/ze
//   - /opt/app/bin/ze → /opt/app/var/lib/ze
//   - ./bin/ze → var/lib/ze (relative)
//   - unknown layout → "" (caller must provide ze.data.dir)
//
// The data directory is a folder of its own rather than one under the config
// directory: the config directory holds the live store and its credentials,
// and an image tree holds arbitrary owners, modes and set-user files.
func DataDirFromBinary(binaryPath string) string {
	layout, prefix := classifyBinary(binaryPath)
	switch layout {
	case layoutGokrazy:
		return "/perm/var/lib/ze"
	case layoutSystem:
		return "/var/lib/ze"
	case layoutRelative:
		return "var/lib/ze"
	case layoutPrefix:
		return filepath.Join(prefix, "var", "lib", "ze")
	case layoutUnknown:
		return ""
	}
	panic("BUG: classifyBinary answered a layout DataDirFromBinary does not know")
}

// DefaultDataDir resolves the data directory, preferring the ze.data.dir
// override and falling back to the running binary's location. Returns "" if
// the override is unset and the binary location cannot be determined or
// doesn't match a known layout.
func DefaultDataDir() string {
	if dir := env.Get(dataDirKey); dir != "" {
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

	return DataDirFromBinary(resolved)
}
