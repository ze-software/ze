// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package hostload

import (
	"context"
	"fmt"
	"os/exec"
)

// readLoadAverages reads the load averages on macOS from `sysctl -n
// vm.loadavg`, which prints "{ 3.43 4.65 14.57 }".
func readLoadAverages() (Averages, error) {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return Averages{}, fmt.Errorf("sysctl -n vm.loadavg: %w", err)
	}
	return parseLoadAverages(string(out))
}
