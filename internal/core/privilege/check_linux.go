// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Overview: drop.go -- privilege package
// Related: capabilities.go -- DaemonCapabilities, the set this checks

//go:build linux

package privilege

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckPrivileges returns warnings for each capability of DaemonCapabilities
// this process lacks. Returns nil if running as root or all capabilities are
// present.
func CheckPrivileges() []string {
	if os.Getuid() == 0 {
		return nil
	}

	eff, err := effectiveCapabilities()
	if err != nil {
		var tb textbuf.Buffer
		return []string{tb.Str("running without root; cannot read capabilities: ").Err(err).String()}
	}

	var missing []string
	for _, c := range DaemonCapabilities() {
		if eff&(1<<c.Bit) == 0 {
			var tb textbuf.Buffer
			missing = append(missing, tb.Str("  ").Str(c.Name).Str(" (").Str(c.Use).Byte(')').String())
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return append([]string{"running without root; missing capabilities:"}, missing...)
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
