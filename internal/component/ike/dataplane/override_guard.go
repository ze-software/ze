// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Detail: override_guard_linux.go -- the cgroup setsockopt program, its load, attach and detach
// Detail: override_guard_other.go -- the refusal off Linux
// Related: espform_linux.go -- Ze's own socket policy, which carries PriorityESPFormSocket
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-8, the administrator's override control

package dataplane

import (
	"errors"
	"strconv"
)

// PriorityESPFormSocket is the priority Ze writes into the per-socket policy of its
// ESP form reader (espFormBypassInboundPolicy). The kernel copies a socket policy's
// priority and never matches on it (net/xfrm/xfrm_user.c copy_from_user_policy,
// called from xfrm_compile_policy; xfrm_sk_policy_lookup reads no priority), so the
// value is a mark the read-back uses to name that row esp-form-socket rather than
// socket-override. It sits two places above the catch-all, a value no SPD entry Ze
// installs uses, so it never collides with Ze's own SPD ranks.
const PriorityESPFormSocket = PriorityUnmatched - 2

// ErrNoSocketPolicySPD is a backend's declaration that its SPD holds no per-socket
// policies at all, so no application can override it and there is nothing to guard.
// The VPP backend answers it: its SPD lives in VPP, not in the kernel's XFRM, and a
// setsockopt reaches only the kernel's.
var ErrNoSocketPolicySPD = errors.New("dataplane: this backend's SPD holds no per-socket policies")

// OverrideGuardStatus is the state of the application override guard.
type OverrideGuardStatus uint8

const (
	// OverrideGuardUnspecified is the zero value: nothing was read.
	OverrideGuardUnspecified OverrideGuardStatus = iota
	// OverrideGuardDetached means no guard program is in force: none was attached,
	// or the link Ze holds no longer reaches the cgroup it was attached to.
	OverrideGuardDetached
	// OverrideGuardAttached means the guard program is attached to the cgroup2 root.
	OverrideGuardAttached
	// OverrideGuardNotApplicable means the backend's SPD holds no per-socket
	// policies (ErrNoSocketPolicySPD), so no guard is needed.
	OverrideGuardNotApplicable
)

// overrideGuardStatusNames is the one declaration of each status's rendered name.
var overrideGuardStatusNames = [...]string{
	OverrideGuardUnspecified:   "unknown",
	OverrideGuardDetached:      "detached",
	OverrideGuardAttached:      "attached",
	OverrideGuardNotApplicable: "not-applicable",
}

// String answers the rendered name; a value outside the set names its number.
func (s OverrideGuardStatus) String() string {
	if int(s) < len(overrideGuardStatusNames) {
		return overrideGuardStatusNames[s]
	}
	return "invalid-guard-status-" + strconv.Itoa(int(s))
}

// OverrideGuardState is what a backend reports about its guard.
type OverrideGuardState struct {
	Status OverrideGuardStatus
	// Refusals counts the setsockopt calls the program refused since it was
	// loaded, summed over every CPU. Zero while no program is loaded.
	Refusals uint64
}

// OverrideGuard is implemented by a backend that can enforce `vpn ipsec
// application-override deny`: RFC 4301 Section 4.4.1, "the system administrator MUST
// be able to specify whether or not a user or application can override (default)
// system policies."
//
// A backend that does not implement it has declared nothing, and the engine treats
// that as unable: under deny its apply is refused, the way CatchAllInstaller treats
// a backend that answers nothing about the catch-all.
//
// The engine MUST call DetachOverrideGuard for every successful AttachOverrideGuard
// before it releases the backend, and AttachOverrideGuard MUST be idempotent: a
// reload that keeps deny calls it again. Both are safe for concurrent use.
type OverrideGuard interface {
	// AttachOverrideGuard puts the guard in force. It returns ErrNoSocketPolicySPD
	// when the backend has nothing to guard, and any other error when it could not
	// attach, in which case nothing stays attached.
	AttachOverrideGuard() error
	// DetachOverrideGuard removes the guard. Detaching a guard that is not attached
	// is not an error.
	DetachOverrideGuard() error
	// OverrideGuardState reads the guard's state, or an error when it cannot.
	OverrideGuardState() (OverrideGuardState, error)
}
