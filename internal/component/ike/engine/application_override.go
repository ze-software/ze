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

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// appliedOverride is the `application-override` value of the configuration the
// running engine applied, for the doctor checks. Nil until the first apply, and nil
// again after releaseSPD, so a doctor run with no engine reads no state rather than a
// default.
var appliedOverride atomic.Pointer[ipsec.ApplicationOverride]

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
// Under deny a guard that cannot be attached fails the apply, with the reason and the
// three ways out: exact or refuse (ai/rules/architecture.md). The caller MUST call it
// with the catch-all already installed on the first apply, so no IKE socket opens
// before the guard, and MUST call releaseApplicationOverride after the catch-all
// leaves (releaseSPD).
func applyApplicationOverride(dp dataplane.Dataplane, override ipsec.ApplicationOverride) error {
	switch override {
	case ipsec.ApplicationOverrideDeny:
		if err := attachOverrideGuard(dp); err != nil {
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
		return fmt.Errorf("vpn ipsec application-override deny: Ze could not attach its setsockopt guard to the cgroup2 root (%w). "+
			"Without it any process holding CAP_NET_ADMIN can exempt its sockets from the IPsec policy. "+
			"Run ze with CAP_BPF and CAP_NET_ADMIN, mount cgroup2 at /sys/fs/cgroup, "+
			"or set `vpn ipsec application-override allow` to accept that risk", err)
	}
	return nil
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
	if err := detachOverrideGuard(dp); err != nil {
		log.Warn("ike: the application override guard did not detach; it closes when ze exits", "error", err)
	}
}
