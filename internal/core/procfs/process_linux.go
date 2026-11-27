//go:build linux

// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- Linux /proc/<pid> process inspector
// Related: process.go -- ProcessInspect type and status parser
// Related: process_other.go -- non-Linux stub

package procfs

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

var errInvalidPID = errors.New("procfs: pid must be positive")

// ReadProcessInspect reads /proc/<pid>/status, /proc/<pid>/wchan, and
// /proc/<pid>/fd count for pid. Reading wchan and fd count is best-effort;
// failing to read /proc/<pid>/status returns an error.
func ReadProcessInspect(pid int) (ProcessInspect, error) {
	if pid <= 0 {
		return ProcessInspect{}, errInvalidPID
	}

	base := "/proc/" + strconv.Itoa(pid)
	statusLines, err := ReadFileLines(base + "/status")
	if err != nil {
		return ProcessInspect{}, err
	}
	info := parseProcessStatusLines(statusLines)

	if wchanLines, wchanErr := ReadFileLines(base + "/wchan"); wchanErr == nil && len(wchanLines) > 0 {
		info.Wchan = strings.TrimSpace(wchanLines[0])
	}

	if entries, fdErr := os.ReadDir(base + "/fd"); fdErr == nil {
		count := len(entries)
		info.FDCount = &count
	}

	return info, nil
}
