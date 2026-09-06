// Design: docs/architecture/diagnostics/crash-capture.md -- crash file listing for CLI

package crashlog

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// CrashSummary describes one crash file for CLI display. Kind names what
// produced it: KindPanic for a Go panic this daemon caught, KindKernel for a
// kernel fault recovered from the reserved region on the following boot.
type CrashSummary struct {
	Name string
	Size int64
	Kind string
}

// ListCrashes returns crash file summaries, newest first.
// Returns nil if crashDir is not set or unreadable.
func ListCrashes() []CrashSummary {
	if crashDir == "" {
		return nil
	}
	names := listCrashFileNames(crashDir)
	if len(names) == 0 {
		return nil
	}

	result := make([]CrashSummary, 0, len(names))
	for _, name := range slices.Backward(names) {
		info, err := os.Stat(filepath.Join(crashDir, name))
		if err != nil {
			continue
		}
		result = append(result, CrashSummary{
			Name: name,
			Size: info.Size(),
			Kind: CrashKind(name),
		})
	}
	return result
}

// LatestCrash returns the full content of the most recent crash file.
// Returns empty string if no crash files exist.
func LatestCrash() string {
	if crashDir == "" {
		return ""
	}
	names := listCrashFileNames(crashDir)
	if len(names) == 0 {
		return ""
	}
	return ReadCrash(names[len(names)-1])
}

// ReadCrash returns the full content of a crash file by name.
// Returns empty string if the file does not exist or is unreadable.
func ReadCrash(name string) string {
	if crashDir == "" {
		return ""
	}
	if !strings.HasPrefix(name, crashFilePrefix) || !strings.HasSuffix(name, crashFileSuffix) {
		return ""
	}
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(crashDir, name)) //nolint:gosec // name validated above
	if err != nil {
		return ""
	}
	return string(data)
}

// CrashListFields renders the stored artifacts as the structured rows every
// surface carries, newest first. The daemon RPC, the offline fallback and the
// support bundle all build their answer from this one call, so a row means the
// same thing and carries the same key names wherever it is read.
func CrashListFields() []map[string]any {
	summaries := ListCrashes()
	rows := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		rows = append(rows, map[string]any{
			"name": s.Name,
			"size": s.Size,
			"kind": s.Kind,
		})
	}
	return rows
}

// CrashKind names what produced the artifact called name. The kind is derived
// from the name rather than stored beside it, so one directory holds both kinds
// and one listing tells them apart.
func CrashKind(name string) string {
	if strings.HasSuffix(name, kernelFileSuffix) {
		return KindKernel
	}
	return KindPanic
}

// SetCrashDirForTest points the listing API at dir and returns the call that
// restores the previous value. Test use only.
//
// The crash directory is resolved once per process by Init, which is correct for
// a daemon and unreachable for a test in another package: the surfaces that
// build a listing payload live in internal/plugins/crashes, and they have no way
// to reach the resolved directory otherwise.
func SetCrashDirForTest(dir string) (restore func()) {
	// Init runs first, and on purpose. It resolves the directory inside a
	// sync.Once, so a later first call from the code under test would overwrite
	// what this function just set. Consuming the Once here removes an ordering
	// hazard every caller would otherwise have to know about.
	Init()

	previous := crashDir
	crashDir = dir
	return func() { crashDir = previous }
}

// CrashDir returns the resolved crash directory path, or empty if none.
func CrashDir() string {
	return crashDir
}
