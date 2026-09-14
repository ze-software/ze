//go:build linux

// Design: docs/features/interfaces.md -- the CAP_SYS_TIME probe the readiness check runs
// Overview: doctor.go -- checkNTPClockPrivilege's registration and its sibling check
// Related: clock_linux.go -- setClock and slewClock, the calls that need the capability

package ntp

import (
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// capSysTime is the bit CAP_SYS_TIME holds in the kernel's capability mask
// (linux/capability.h).
const capSysTime = 25

// The two probes checkNTPClockPrivilege runs: the process uid, and the
// /proc/self/status read that carries the effective capability mask. Each is a
// variable so a test stands in an unprivileged process; nothing else assigns
// them.
var (
	ntpCurrentUID  = os.Getuid
	ntpReadProcess = os.ReadFile
)

// checkNTPClockPrivilege warns when the client is enabled, the process is not
// root, and its effective capabilities lack CAP_SYS_TIME, so every clock
// adjustment the client attempts will fail. A status file it cannot read or
// parse is not evidence either way and reports nothing.
//
// A nil tree is the missing-config phase, which this check does not run in,
// and a context carrying anything else is a runner defect the runner's own
// type assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkNTPClockPrivilege(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if _, enabled := ntpEnabled(tree); !enabled {
		return nil
	}
	if ntpCurrentUID() == 0 {
		return nil
	}
	data, err := ntpReadProcess(kernelcap.ProcPath("self", "status"))
	if err != nil {
		return nil
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		hex, ok := strings.CutPrefix(line, "CapEff:\t")
		if !ok {
			continue
		}
		caps, parseErr := strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
		if parseErr != nil {
			return nil
		}
		if caps&(1<<capSysTime) == 0 {
			return []diagnostic.Diagnostic{{
				Code:     codeNTPClockPrivilege,
				Severity: diagnostic.SeverityWarning,
				Message:  "NTP: CAP_SYS_TIME not granted; clock adjustment will fail",
			}}
		}
		return nil
	}
	return nil
}
