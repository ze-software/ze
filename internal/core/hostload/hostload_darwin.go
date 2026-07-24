// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package hostload

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// readLoadAvg1 reads the 1-minute load average on macOS via sysctl.
func readLoadAvg1() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0
	}
	// Format: "{ 3.43 4.65 14.57 }"
	s := strings.TrimSpace(string(out))
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}
