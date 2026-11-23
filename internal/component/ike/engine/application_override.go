// Design: docs/guide/ipsec.md -- application override control
// Related: apply.go -- applyConfig, which calls applyApplicationOverride after the catch-all
// Related: doctor_application_override.go -- the doctor checks that read what this records
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-8, the administrator's override control

package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"syscall"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/privilege"
)

// appliedOverride is the `application-override` value of the configuration the
// running engine applied, for the doctor checks. Nil until the first apply, and nil
// again after releaseSPD, so a doctor run with no engine reads no state rather than a
// default.
var appliedOverride atomic.Pointer[ipsec.ApplicationOverride]

// overrideRefusal is the refusal of the last apply that asked for deny and could not
// attach the guard, for the doctor check: a first apply that fails this way records
// no appliedOverride, so without it a host with no cgroup2 mounted, or a ze without
// CAP_BPF, would leave IPsec down with nothing in `ze doctor` saying why. Nil after an
// apply that put the guard in the state it asked for, and after releaseSPD.
var overrideRefusal atomic.Pointer[overrideGuardRefusal]

// overrideGuardRefusal is one refusal of the guard: the text the apply refused with, and
// the fix the doctor finding carries for its cause (overrideGuardFix). Never changed
// once stored.
type overrideGuardRefusal struct {
	text string
	fix  string
}

// The fixes of a guard that did not attach, besides the capability remedy.
const (
	// overrideGuardFixMount mounts the cgroup2 hierarchy the guard attaches to, where
	// no cgroup2 file system is mounted (dataplane.ErrNoCgroup2).
	overrideGuardFixMount = "sudo mount -t cgroup2 none /sys/fs/cgroup"
	// overrideGuardFixAllow is the configuration statement that accepts overrides, so
	// the apply no longer needs the guard: the one way out for a backend that holds
	// no guard, and for a cause Ze cannot name (a kernel before Linux 6.10, whose
	// verifier refuses the guard's bpf_get_ns_current_pid_tgid call, answers EINVAL like
	// other refusals do). The refusal's text names the risk it accepts.
	overrideGuardFixAllow = "set vpn ipsec application-override allow"
)

// overrideGuardFault classifies why the guard did not attach, which decides the cause
// the refusal names and its fix (overrideGuardFaultOf). It is closed: only
// overrideGuardFaultOf produces it.
type overrideGuardFault uint8

const (
	// overrideGuardFaultUnspecified is the zero value, which overrideGuardFaultOf never
	// answers.
	overrideGuardFaultUnspecified overrideGuardFault = iota
	// overrideGuardFaultCapability is a process without CAP_BPF or CAP_NET_ADMIN: EPERM.
	overrideGuardFaultCapability
	// overrideGuardFaultNoCgroup2 is a host with no cgroup2 mounted (dataplane.ErrNoCgroup2).
	overrideGuardFaultNoCgroup2
	// overrideGuardFaultUnknown is any other answer: Ze cannot tell which cause it is,
	// and names the ones it knows rather than guess one.
	overrideGuardFaultUnknown
)

// overrideGuardFaultOf classifies err, the error AttachOverrideGuard answered
// (dataplane/override_guard_linux.go, kernel/bpf/syscall.c, Linux master):
//
//   - EPERM: bpf_prog_load sets "err = -EPERM;" before its capability checks, and
//     refuses a program type other than a socket filter or cgroup skb "if (type !=
//     BPF_PROG_TYPE_SOCKET_FILTER && type != BPF_PROG_TYPE_CGROUP_SKB && !bpf_cap)",
//     where "bpf_cap = bpf_token_capable(token, CAP_BPF);", then "if
//     (is_net_admin_prog_type(type) && !bpf_token_capable(token, CAP_NET_ADMIN))",
//     whose list holds "case BPF_PROG_TYPE_CGROUP_SOCKOPT:". So a ze without CAP_BPF
//     or CAP_NET_ADMIN gets EPERM for the guard.
//   - dataplane.ErrNoCgroup2: cgroup2Root found no cgroup2 mounted, so the guard has
//     no root to attach to and the kernel was not asked.
//   - Anything else: a kernel before Linux 6.10 refuses the guard's
//     bpf_get_ns_current_pid_tgid call, and its verifier answers EINVAL as it does for
//     other programs it refuses; a security module's bpf hook answers EACCES, which
//     the verifier also answers for a program it refuses. No errno tells these apart,
//     so the class is Unknown.
func overrideGuardFaultOf(err error) overrideGuardFault {
	if errors.Is(err, syscall.EPERM) {
		return overrideGuardFaultCapability
	}
	if errors.Is(err, dataplane.ErrNoCgroup2) {
		return overrideGuardFaultNoCgroup2
	}
	return overrideGuardFaultUnknown
}

// overrideGuardCause answers, in the operator's words, why the guard did not attach
// with err. Safe for concurrent use.
func overrideGuardCause(err error) string {
	return overrideGuardCauseOf(overrideGuardFaultOf(err))
}

// overrideGuardCauseOf is overrideGuardCause for one fault.
func overrideGuardCauseOf(fault overrideGuardFault) string {
	switch fault {
	case overrideGuardFaultCapability:
		return "ze lacks CAP_BPF or CAP_NET_ADMIN, which the kernel asks of a process that loads a cgroup setsockopt program"
	case overrideGuardFaultNoCgroup2:
		return "the guard has no cgroup2 hierarchy to attach to"
	case overrideGuardFaultUnknown:
		return "Ze cannot tell the cause from the kernel's answer: a kernel before Linux 6.10 refuses the guard's " +
			"bpf_get_ns_current_pid_tgid call, and a security module (SELinux or AppArmor) can refuse the program"
	case overrideGuardFaultUnspecified:
		panic("BUG: engine: overrideGuardFaultOf answered overrideGuardFaultUnspecified")
	}
	panic("BUG: engine: undeclared overrideGuardFault")
}

// overrideGuardFix answers the fix for a guard that did not attach with err: the
// capability remedy for an EPERM, worded for this process by
// privilege.CapabilityRemedy, so a ze that systemd did not start from ze.service is
// never told to reinstall the unit; the mount for a host with no cgroup2; otherwise
// the statement that accepts overrides. Safe for concurrent use.
func overrideGuardFix(err error) string {
	return overrideGuardFixOf(overrideGuardFaultOf(err))
}

// overrideGuardFixOf is overrideGuardFix for one fault.
func overrideGuardFixOf(fault overrideGuardFault) string {
	switch fault {
	case overrideGuardFaultCapability:
		return privilege.CapabilityRemedy("CAP_BPF", "CAP_NET_ADMIN")
	case overrideGuardFaultNoCgroup2:
		return overrideGuardFixMount
	case overrideGuardFaultUnknown:
		return overrideGuardFixAllow
	case overrideGuardFaultUnspecified:
		panic("BUG: engine: overrideGuardFaultOf answered overrideGuardFaultUnspecified")
	}
	panic("BUG: engine: undeclared overrideGuardFault")
}

// errOverrideUnspecified refuses a configuration that names no override value. The
// parser and EmptyConfig always write one, so only a configuration built by hand can
// carry the zero value.
var errOverrideUnspecified = errors.New("vpn ipsec application-override names no value: the configuration was not built by ipsec.ParseIPsecConfig or ipsec.EmptyConfig")

// applyApplicationOverride puts the guard in the state override asks for: attached
// under deny, detached under allow.
//
// RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
// whether or not a user or application can override (default) system policies."
//
// Under deny a guard that cannot be attached fails the apply, with the cause and its
// fix (overrideGuardFaultOf): exact or refuse (ai/rules/architecture.md). The caller MUST call it
// with the catch-all already installed on the first apply, so no IKE socket opens
// before the guard, and MUST call releaseApplicationOverride after the catch-all
// leaves (releaseSPD).
func applyApplicationOverride(dp dataplane.Dataplane, override ipsec.ApplicationOverride) error {
	switch override {
	case ipsec.ApplicationOverrideDeny:
		if err := attachOverrideGuard(dp); err != nil {
			overrideRefusal.Store(&overrideGuardRefusal{text: err.Error(), fix: overrideGuardFix(err)})
			return err
		}
	case ipsec.ApplicationOverrideAllow:
		if err := detachOverrideGuard(dp); err != nil {
			// The guard stays attached, which refuses more than allow asked for; the
			// reload is refused rather than reported applied.
			return fmt.Errorf("vpn ipsec application-override allow: Ze could not detach its setsockopt guard (%w). Restart ze to clear it", err)
		}
	case ipsec.ApplicationOverrideUnspecified:
		return errOverrideUnspecified
	default:
		return fmt.Errorf("vpn ipsec application-override holds %v: %w", override, errOverrideUnspecified)
	}
	value := override
	appliedOverride.Store(&value)
	overrideRefusal.Store(nil)
	return nil
}

// attachOverrideGuard attaches the guard, or answers the refusal the operator reads.
func attachOverrideGuard(dp dataplane.Dataplane) error {
	guard, ok := dp.(dataplane.OverrideGuard)
	if !ok {
		return errors.New("vpn ipsec application-override deny: this IPsec dataplane cannot refuse per-socket IPsec policies, " +
			"so any process holding CAP_NET_ADMIN could exempt its sockets from the IPsec policy. " +
			"Set `vpn ipsec application-override allow` to accept that risk")
	}
	err := guard.AttachOverrideGuard()
	if errors.Is(err, dataplane.ErrNoSocketPolicySPD) {
		// The backend's SPD holds no per-socket policies, so deny already holds.
		return nil
	}
	if err != nil {
		return fmt.Errorf("vpn ipsec application-override deny: Ze could not attach its setsockopt guard to the cgroup2 root: %s (%w). "+
			"Without it any process holding CAP_NET_ADMIN can exempt its sockets from the IPsec policy. "+
			"To fix it: %s",
			overrideGuardCause(err), err, overrideGuardRefusalFix(err))
	}
	return nil
}

// overrideGuardAllowAlternative is the way out the commit refusal offers for every
// cause: setting allow needs no guard, and accepts the risk the refusal names.
const overrideGuardAllowAlternative = "set `vpn ipsec application-override allow` to accept that risk"

// overrideGuardRefusalFix is the fix the commit refusal names for err: the cause's own
// fix (overrideGuardFix), then commit again, or overrideGuardAllowAlternative. The fix of
// an Unknown cause is that statement itself, so the refusal names it first and then the
// kernel Ze needs to keep deny.
func overrideGuardRefusalFix(err error) string {
	switch overrideGuardFaultOf(err) {
	case overrideGuardFaultCapability, overrideGuardFaultNoCgroup2:
		return overrideGuardFix(err) + ", then commit again, or " + overrideGuardAllowAlternative
	case overrideGuardFaultUnknown:
		return overrideGuardAllowAlternative + ", or keep deny on Linux 6.10 or later (Ze's own kernel)"
	case overrideGuardFaultUnspecified:
		panic("BUG: engine: overrideGuardRefusalFix read overrideGuardFaultUnspecified from overrideGuardFaultOf")
	}
	panic("BUG: engine: overrideGuardRefusalFix met an undeclared overrideGuardFault")
}

// detachOverrideGuard detaches the guard where the backend can hold one.
func detachOverrideGuard(dp dataplane.Dataplane) error {
	guard, ok := dp.(dataplane.OverrideGuard)
	if !ok {
		// refuse-by-default: guards applyApplicationOverride -- a backend that cannot hold a guard never attached one, so there is nothing to detach
		return nil
	}
	return guard.DetachOverrideGuard()
}

// releaseApplicationOverride detaches the guard when the engine leaves IPsec. It MUST
// be called after the catch-all leaves, so the guard is the last thing Ze holds.
func releaseApplicationOverride(dp dataplane.Dataplane, log *slog.Logger) {
	appliedOverride.Store(nil)
	overrideRefusal.Store(nil)
	if err := detachOverrideGuard(dp); err != nil {
		log.Warn("ike: the application override guard did not detach; it closes when ze exits", "error", err)
	}
}
