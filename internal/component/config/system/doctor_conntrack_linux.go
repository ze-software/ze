//go:build linux

// Design: docs/features/ai-first.md -- the conntrack readiness check this component owns
// Overview: doctor.go -- the cross-platform checks and the table register.go installs
// Related: conntrack.go -- ConntrackSysctlKeys, the keys the hub writes under /proc/sys/net/netfilter
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. Every conntrack tuning this package computes is
// applied as a sysctl under /proc/sys/net/netfilter, so a tree that is absent
// or not writable is this package's question (ai/patterns/registration.md,
// "Doctor Check Registry"). register_linux.go installs it, apart from the
// table in doctor.go, because it is Linux-only: unix.Access is the probe, and
// off Linux there is no netfilter tree to ask about.

package system

import (
	"os"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeConntrackProcfs names a netfilter sysctl tree that is absent or not
// writable. internal/core/diagnostic/codes.go declares it, so `ze explain
// doctor-conntrack-procfs` answers.
const codeConntrackProcfs = "doctor-conntrack-procfs"

// The two probes checkConntrackProcfs runs: whether the netfilter tree exists,
// and whether nf_conntrack_max is writable. Each is a variable so a test stands
// in a kernel without the tree; nothing else assigns them.
var (
	conntrackStatPath   = os.Stat
	conntrackAccessPath = unix.Access
)

// conntrackDoctorCheck is the registration register_linux.go installs.
//
// Order 2070 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the sysctl check (2040,
// internal/component/sysctl/doctor.go) and before the policy-route netlink
// check (2080, internal/plugins/policyroute).
var conntrackDoctorCheck = diagnostic.DoctorCheck{
	Name:         "conntrack-procfs",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2070,
	Component:    doctorComponentSystem,
	Dependencies: []string{"procfs"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeConntrackProcfs},
	Check:        checkConntrackProcfs,
}

// checkConntrackProcfs warns when the config carries a conntrack block and the
// netfilter sysctl tree is absent, or nf_conntrack_max under it is not
// writable by this process.
func checkConntrackProcfs(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	if tree.GetContainerPath("system/conntrack") == nil {
		return nil
	}
	var tb textbuf.Buffer
	dir := kernelcap.ProcPath("sys", "net", "netfilter")
	if _, err := conntrackStatPath(dir); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeConntrackProcfs,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("conntrack: ").Str(dir).Str(" unavailable: ").Err(err).String(),
			Path:     dir,
		}}
	}
	key := kernelcap.ProcPath("sys", "net", "netfilter", "nf_conntrack_max")
	if err := conntrackAccessPath(key, unix.W_OK); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeConntrackProcfs,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str("conntrack: ").Str(key).Str(" is not writable: ").Err(err).String(),
			Path:     key,
		}}
	}
	return nil
}
