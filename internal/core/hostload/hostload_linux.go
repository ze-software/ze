// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package hostload

import (
	"os"
	"strconv"
	"strings"
)

// readLoadAvg1 reads the 1-minute load average on Linux from /proc/loadavg.
func readLoadAvg1() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	// Format: "3.43 4.65 14.57 1/234 5678"
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}
