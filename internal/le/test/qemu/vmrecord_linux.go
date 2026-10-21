//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le

package testqemu

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// processStartTime answers when pid started, in clock ticks since boot, from
// field 22 of /proc/<pid>/stat, or errProcessGone when no process holds it.
// The fields are counted after the last ')', because field 2, the command
// name, may itself hold spaces and parentheses.
func processStartTime(pid int) (int64, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return 0, errProcessGone
	}
	if err != nil {
		return 0, fmt.Errorf("read the process table entry of pid %d: %w", pid, err)
	}
	text := string(data)
	fields := strings.Fields(text[strings.LastIndexByte(text, ')')+1:])
	// Field 3, the state, is fields[0], so field 22 is fields[19].
	if len(fields) < 20 {
		return 0, fmt.Errorf("the process table entry of pid %d has %d fields after its name, want at least 20", pid, len(fields))
	}
	start, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("the start time of pid %d is not a number: %w", pid, err)
	}
	return start, nil
}
