// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- per-PID /proc inspection types and status parser
// Related: process_linux.go -- Linux /proc/<pid> reader
// Related: process_other.go -- non-Linux stub

package procfs

import (
	"strconv"
	"strings"
)

// ProcessInspect holds Linux /proc/<pid> diagnostic metrics for a process.
type ProcessInspect struct {
	State                    string `json:"state,omitempty"`
	Threads                  int    `json:"threads,omitempty"`
	VmRSSKB                  int    `json:"vm-rss-kb,omitempty"`
	VmSizeKB                 int    `json:"vm-size-kb,omitempty"`
	VmSwapKB                 int    `json:"vm-swap-kb,omitempty"`
	VmPeakKB                 int    `json:"vm-peak-kb,omitempty"`
	VmDataKB                 int    `json:"vm-data-kb,omitempty"`
	VmStackKB                int    `json:"vm-stack-kb,omitempty"`
	VoluntaryCtxtSwitches    int64  `json:"voluntary-ctxt-switches,omitempty"`
	NonvoluntaryCtxtSwitches int64  `json:"nonvoluntary-ctxt-switches,omitempty"`
	Wchan                    string `json:"wchan,omitempty"`
	FDCount                  *int   `json:"fd-count,omitempty"`
}

// parseProcessStatusLines parses key:value lines from Linux /proc/<pid>/status.
func parseProcessStatusLines(lines []string) ProcessInspect {
	var info ProcessInspect
	for _, line := range lines {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "State":
			info.State = value
		case "Threads":
			n, _ := strconv.Atoi(value)
			info.Threads = n
		case "VmRSS":
			info.VmRSSKB = parseStatusKB(value)
		case "VmSize":
			info.VmSizeKB = parseStatusKB(value)
		case "VmSwap":
			info.VmSwapKB = parseStatusKB(value)
		case "VmPeak":
			info.VmPeakKB = parseStatusKB(value)
		case "VmData":
			info.VmDataKB = parseStatusKB(value)
		case "VmStk":
			info.VmStackKB = parseStatusKB(value)
		case "voluntary_ctxt_switches":
			n, _ := strconv.ParseInt(value, 10, 64)
			info.VoluntaryCtxtSwitches = n
		case "nonvoluntary_ctxt_switches":
			n, _ := strconv.ParseInt(value, 10, 64)
			info.NonvoluntaryCtxtSwitches = n
		}
	}
	return info
}

func parseStatusKB(s string) int {
	s = strings.TrimSuffix(s, "kB")
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}
