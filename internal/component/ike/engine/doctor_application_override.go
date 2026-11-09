// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Related: application_override.go -- the apply that attaches the guard these checks read
// Related: doctor.go -- the sibling ike readiness checks and their registration
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the administrator's control over application overrides

package engine

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// procRoot is where checkIPsecSPDWriters reads the host's processes. A test points it
// at a tree of its own.
var procRoot = "/proc"

// checkIPsecApplicationOverride reports what defeats the administrator's choice.
//
// RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
// whether or not a user or application can override (default) system policies."
//
// Under `allow` it reports nothing: the administrator accepted overrides. Under
// `deny` it reports a guard that is not attached (an error: any CAP_NET_ADMIN holder
// can then exempt its sockets), and every per-socket policy Ze cannot attribute to
// itself (a warning: one set before the guard attached, or while `allow` held, lives
// until its socket closes, and the kernel cannot remove it). An apply that asked for
// `deny` and could not attach the guard is an error too, carrying the apply's refusal,
// which names the cause and the three ways out.
func checkIPsecApplicationOverride(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	if refusal := overrideRefusal.Load(); refusal != nil {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorIPsecOverrideGuard,
			Severity: severityError,
			Message:  "The last IPsec apply was refused: " + *refusal,
		}}
	}
	applied := appliedOverride.Load()
	if applied == nil || *applied != ipsec.ApplicationOverrideDeny {
		return nil
	}
	dp := dataplane.Get()
	guard, ok := dp.(dataplane.OverrideGuard)
	if !ok {
		return []rpc.DoctorCheckDiagnostic{overrideGuardDiagnostic("the IPsec dataplane has no override guard, yet the apply recorded deny")}
	}
	state, err := guard.OverrideGuardState()
	// A backend that answers ErrNotSupported holds no kernel state to read (the
	// test-only noop backend; XFRM off Linux, where the apply already failed). This
	// check has nothing to judge there, and says nothing rather than guess.
	if errors.Is(err, dataplane.ErrNotSupported) {
		return nil
	}
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{overrideGuardDiagnostic("Ze could not read the guard: " + err.Error())}
	}
	switch state.Status {
	case dataplane.OverrideGuardNotApplicable:
		// VPP: its SPD has no per-socket policies.
		return nil
	case dataplane.OverrideGuardAttached:
		return socketPolicyDiagnostics(dp)
	case dataplane.OverrideGuardUnspecified, dataplane.OverrideGuardDetached:
	}
	return append([]rpc.DoctorCheckDiagnostic{overrideGuardDiagnostic("the guard reads " + state.Status.String())},
		socketPolicyDiagnostics(dp)...)
}

// overrideGuardDiagnostic is the error a guard that does not hold under deny raises.
func overrideGuardDiagnostic(why string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecOverrideGuard,
		Severity: severityError,
		Message: "The IPsec application-override guard is not attached (" + why + "), so applications can " +
			"override the SPD. Something detached it or Ze's load failed. Restart ze; if this persists, run " +
			"`ze doctor --json` and read the attach error.",
	}
}

// socketPolicyDiagnostics warns about every per-socket policy that is not Ze's own.
// Each message counts every socket policy present, Ze's own included, because the mark
// that names Ze's own can be copied by a process that set its policy before the guard
// attached (the spec's Known Limitations).
func socketPolicyDiagnostics(dp dataplane.Dataplane) []rpc.DoctorCheckDiagnostic {
	policies, err := dp.ListPolicies()
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorIPsecSocketPolicy,
			Severity: severityWarning,
			Message: "Ze could not list the IPsec policies (" + err.Error() + "), so it cannot tell whether a " +
				"process holds a per-socket policy that overrides the SPD. Run `show vpn ipsec dataplane policy`.",
		}}
	}
	total := 0
	for i := range policies {
		if policies[i].PerSocket {
			total++
		}
	}
	var out []rpc.DoctorCheckDiagnostic
	for i := range policies {
		p := &policies[i]
		if !p.PerSocket || p.Role == dataplane.PolicyRoleESPFormSocket {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("A process holds a per-socket IPsec policy (").Str(socketPolicyDirection(p.Dir)).Str(", ").
			Str(selectorString(p.Src)).Str(" to ").Str(selectorString(p.Dst)).
			Str(") that overrides the SPD for its socket (").Int(int64(total)).
			Str(" per-socket policies in all). The kernel cannot remove it and does not say which process owns it. " +
				"Restart the applications started before Ze: the policy goes with its socket, and with the guard " +
				"attached a restarted application cannot set it again.")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecSocketPolicy,
			Severity: severityWarning,
			Message:  tb.String(),
		})
	}
	return out
}

// socketPolicyDirection names a socket policy's direction for the operator.
func socketPolicyDirection(dir dataplane.SADir) string {
	switch dir {
	case dataplane.SADirIn:
		return "in"
	case dataplane.SADirOut:
		return "out"
	case dataplane.SADirFwd:
		return "fwd"
	}
	return "direction " + strconv.Itoa(int(dir))
}

// selectorString renders one side of a selector; an absent one matches anything.
func selectorString(n *net.IPNet) string {
	if n == nil {
		return "any"
	}
	return n.String()
}

// spdWriter is one process, other than Ze, that holds CAP_NET_ADMIN in Ze's network
// namespace.
type spdWriter struct {
	pid  int
	comm string
}

// capNetAdmin is CAP_NET_ADMIN's bit in the CapEff mask of /proc/<pid>/status.
const capNetAdmin = 12

// checkIPsecSPDWriters names every other process Linux lets write the SPD.
//
// No cgroup hook covers an XFRM netlink write, and the kernel admits one from any
// CAP_NET_ADMIN holder in the namespace (xfrm_user_rcv_msg: "All operations require
// privileges, even GET"), so Linux treats each such holder as an administrator (spec
// D8-3). The check names them under either value of the leaf, because the leaf governs
// socket policies, not SPD writes. When /proc hides any process from Ze, it says Ze
// cannot tell, rather than reporting no writer.
func checkIPsecSPDWriters(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	if appliedOverride.Load() == nil {
		return nil
	}
	scan, err := spdWriters(procRoot, os.Getpid())
	// A scan that could not look is no answer: saying nothing would read as "no
	// other writer" on a host whose /proc hides them.
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{spdWritersUnknownDiagnostic("reading " + procRoot + " failed: " + err.Error())}
	}
	var out []rpc.DoctorCheckDiagnostic
	if len(scan.writers) > 0 {
		out = append(out, spdWritersDiagnostic(scan.writers))
	}
	if scan.unreadable > 0 {
		var why textbuf.Buffer
		why.Str("processes whose namespace or capabilities Linux hides under ").Str(procRoot).Str(": ").
			Int(int64(scan.unreadable))
		out = append(out, spdWritersUnknownDiagnostic(why.String()))
	}
	return out
}

// spdWritersDiagnostic is the warning that names each other process able to write the SPD.
func spdWritersDiagnostic(writers []spdWriter) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("Processes ")
	for i, w := range writers {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Int(int64(w.pid)).Str(" (").Str(w.comm).Str(")")
	}
	tb.Str(" hold CAP_NET_ADMIN in Ze's network namespace, so Linux lets them write the IPsec SPD directly. " +
		"Remove the capability from them or stop them.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecSPDWriter,
		Severity: severityWarning,
		Message:  tb.String(),
	}
}

// spdWritersUnknownDiagnostic is the warning raised when the scan could not see every
// process, so it cannot say whether another one can write the SPD.
func spdWritersUnknownDiagnostic(why string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecSPDWriterUnknown,
		Severity: severityWarning,
		Message: "Ze could not see which processes hold CAP_NET_ADMIN in its network namespace (" + why +
			"), so it cannot tell whether another process can write the IPsec SPD directly. Mount /proc " +
			"without hidepid (`mount -o remount,hidepid=0 /proc`, or `mount -t proc proc /proc` when it " +
			"is not mounted) and run ze as root with CAP_SYS_PTRACE, so it can read every process's namespace.",
	}
}

// spdScan is what the writer scan saw: the writers it named, and how many processes it
// could not judge because Linux refused to show their namespace or capabilities.
type spdScan struct {
	writers    []spdWriter
	unreadable int
}

// spdWriters lists the processes under root that share self's network namespace and
// hold CAP_NET_ADMIN in their effective set. A kernel thread (empty cmdline) is no
// writer an operator can act on and is skipped. A process whose namespace or status
// Linux refuses to show (hidepid, or no ptrace access to it) is counted as unreadable,
// never as no writer; one that exited during the scan is gone and is not counted. It
// answers an error when root itself, or self's own namespace, cannot be read.
func spdWriters(root string, self int) (spdScan, error) {
	own, err := os.Readlink(filepath.Join(root, strconv.Itoa(self), "ns", "net"))
	if err != nil {
		return spdScan{}, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return spdScan{}, err
	}
	var scan spdScan
	for _, e := range entries {
		pid, convErr := strconv.Atoi(e.Name())
		if convErr != nil {
			continue
		}
		if pid == self {
			continue
		}
		dir := filepath.Join(root, e.Name())
		ns, nsErr := os.Readlink(filepath.Join(dir, "ns", "net"))
		if nsErr != nil {
			scan.unreadable += hiddenProcess(nsErr)
			continue
		}
		if ns != own {
			continue
		}
		cmdline, cmdErr := os.ReadFile(filepath.Join(dir, "cmdline")) //nolint:gosec // dir is root plus a numeric pid entry, the leaf a constant
		if cmdErr != nil {
			continue
		}
		if len(cmdline) == 0 {
			continue
		}
		status, statusErr := os.ReadFile(filepath.Join(dir, "status")) //nolint:gosec // dir is root plus a numeric pid entry, the leaf a constant
		if statusErr != nil {
			scan.unreadable += hiddenProcess(statusErr)
			continue
		}
		if !statusHasCap(string(status), capNetAdmin) {
			continue
		}
		comm, commErr := os.ReadFile(filepath.Join(dir, "comm")) //nolint:gosec // dir is root plus a numeric pid entry, the leaf a constant
		name := strings.TrimSpace(string(comm))
		if commErr != nil || name == "" {
			name = "unknown command"
		}
		scan.writers = append(scan.writers, spdWriter{pid: pid, comm: name})
	}
	slices.SortFunc(scan.writers, func(a, b spdWriter) int { return a.pid - b.pid })
	return scan, nil
}

// hiddenProcess answers 1 when a read of a process's /proc entry failed because Linux
// hid it, and 0 when the process exited during the scan: a gone process writes nothing.
func hiddenProcess(err error) int {
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}
	return 1
}

// statusHasCap reads the CapEff line of a /proc/<pid>/status body and answers whether
// bit is set. A body with no parseable CapEff answers false.
func statusHasCap(status string, bit uint) bool {
	for line := range strings.SplitSeq(status, "\n") {
		mask, ok := strings.CutPrefix(line, "CapEff:")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(mask), 16, 64)
		return err == nil && v&(1<<bit) != 0
	}
	return false
}
