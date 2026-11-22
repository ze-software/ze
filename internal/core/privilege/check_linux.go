// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Overview: drop.go -- privilege package
// Related: capabilities.go -- DaemonCapabilities, the set this inventories

//go:build linux

package privilege

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CheckPrivileges answers the generic capability inventory: the capabilities of
// DaemonCapabilities this process's effective set lacks, in the set's order, or
// the error that kept it from reading the set. A missing capability is not a
// problem by itself, since a config that uses nothing it serves runs without
// it: each feature that needs one checks for it and reports its own impact.
// A root process is read like any other, because root can run with a reduced
// set (a container, a unit's CapabilityBoundingSet=).
func CheckPrivileges() ([]Capability, error) {
	eff, err := effectiveCapabilities()
	if err != nil {
		return nil, err
	}
	var missing []Capability
	for _, c := range daemonCapabilities {
		if eff&(1<<c.Bit) == 0 {
			missing = append(missing, c)
		}
	}
	return missing, nil
}

// effectiveCapabilities reads this process's effective capability set from
// /proc/self/status, one bit per Capability.Bit.
func effectiveCapabilities() (uint64, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if hex, ok := strings.CutPrefix(line, "CapEff:\t"); ok {
			return strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
		}
	}
	return 0, fmt.Errorf("CapEff not found in /proc/self/status")
}
