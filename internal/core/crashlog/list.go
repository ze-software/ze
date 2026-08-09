// Design: docs/architecture/diagnostics/crash-capture.md -- crash file listing for CLI

package crashlog

import (
	"os"
	"path/filepath"
	"strings"
)

// CrashSummary describes one crash file for CLI display.
type CrashSummary struct {
	Name string
	Size int64
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
	for i := len(names) - 1; i >= 0; i-- {
		info, err := os.Stat(filepath.Join(crashDir, names[i]))
		if err != nil {
			continue
		}
		result = append(result, CrashSummary{
			Name: names[i],
			Size: info.Size(),
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

// CrashDir returns the resolved crash directory path, or empty if none.
func CrashDir() string {
	return crashDir
}
