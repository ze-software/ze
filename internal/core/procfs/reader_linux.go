// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- Linux /proc file reading
//
//go:build linux

package procfs

import (
	"os"
	"strings"
)

func ReadFileLines(path string) ([]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // paths are hardcoded constants, never user input
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}
