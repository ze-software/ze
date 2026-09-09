// Design: docs/architecture/cli/plugin-modes.md -- how a plugin process is started
// Related: process/process.go -- (*Process).startExternal, the live start that uses this
// Related: declarations.go -- the query start that uses the same composition
//
// childenv.go composes the environment a plugin child process is started with.
// It lives in the parent package because two callers start a plugin child and
// both owe the same PATH: the daemon's live start in process/, and the
// declaration query in declarations.go. One composition, stated once.

package plugin

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// EngineBinDir returns the directory of the running binary, preferring the
// invocation path over the resolved executable path.
//
// os.Args[0] is preferred over os.Executable() because the latter follows
// symlinks, which can resolve to a directory holding a different-architecture
// binary of the same name (a QEMU 9p mount carrying both the host and the guest
// build).
func EngineBinDir() string {
	if arg0 := os.Args[0]; filepath.IsAbs(arg0) {
		return filepath.Dir(arg0)
	} else if abs, err := filepath.Abs(arg0); err == nil {
		return filepath.Dir(abs)
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return ""
}

// ChildPathEnv composes the PATH a started plugin receives: the inherited PATH
// first, then the engine binary's own directory. It returns the whole
// `PATH=...` assignment, and an empty string when there is no directory to add.
//
// That directory is a FALLBACK, so a run command like "ze plugin bgp-rib"
// still finds ze when it is not installed system-wide, and it MUST NOT come
// first, because a cross-built checkout holds a host binary and a guest binary
// of the same name side by side. Under QEMU the engine runs as
// bin/ze-linux-arm64 next to a darwin bin/ze-test, and putting that directory
// first made every `run "ze-test ..."` in test/ resolve to the darwin binary,
// which the guest shell reads as a script.
func ChildPathEnv(binDir, inherited string) string {
	if binDir == "" {
		return ""
	}
	parts := make([]string, 0, 2)
	if inherited != "" {
		parts = append(parts, inherited)
	}
	parts = append(parts, binDir)
	var tb textbuf.Buffer
	return tb.Str("PATH=").Join(parts, string(os.PathListSeparator)).String()
}
