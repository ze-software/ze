// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Overview: drop.go -- privilege package

//go:build linux

package privilege

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Linux capability bit positions.
const (
	capNetBindService = 10 // Bind to ports < 1024
	capNetRaw         = 13 // Raw sockets (ping, traceroute)
	capNetAdmin       = 12 // Network administration (FIB, interfaces, netlink)
)

type capCheck struct {
	bit  int
	name string
	desc string
}

var requiredCaps = []capCheck{
	{capNetBindService, "CAP_NET_BIND_SERVICE", "port 179"},
	{capNetRaw, "CAP_NET_RAW", "ping, traceroute"},
	{capNetAdmin, "CAP_NET_ADMIN", "FIB, interfaces"},
}

// CheckPrivileges returns warnings for missing capabilities.
// Returns nil if running as root or all capabilities are present.
func CheckPrivileges() []string {
	if os.Getuid() == 0 {
		return nil
	}

	eff, err := effectiveCaps()
	if err != nil {
		var tb textbuf.Buffer
		return []string{tb.Str("running without root; cannot read capabilities: ").Err(err).String()}
	}

	var missing []string
	for _, c := range requiredCaps {
		if eff&(1<<c.bit) == 0 {
			var tb textbuf.Buffer
			missing = append(missing, tb.Str("  ").Str(c.name).Str(" (").Str(c.desc).Byte(')').String())
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return append([]string{"running without root; missing capabilities:"}, missing...)
}

// effectiveCaps reads the effective capability set from /proc/self/status.
func effectiveCaps() (uint64, error) {
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
