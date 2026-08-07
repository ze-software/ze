// Design: docs/architecture/core-design.md -- Firewall reconcile concurrency
// Related: backend_linux.go -- Apply, which binds the deadline and tags the error

//go:build linux

package firewallvpp

import (
	"errors"
	"fmt"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/core"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/env"
)

const (
	// defaultReplyTimeout bounds one VPP binary-API round-trip.
	//
	// govpp's own default is core.DefaultReplyTimeout, which is 0 and which
	// govpp documents as disabling the timeout. That is not a usable default
	// here: firewall.ApplyAll holds the process-wide reconcileMu across
	// Backend.Apply, so one unanswered reply stalls EVERY firewall owner
	// (copp, policy-routes, ddos-local, the firewall engine) for as long as
	// VPP stays silent. Matches the nft backend's netlink deadline.
	defaultReplyTimeout = 10 * time.Second
	// minReplyTimeout is the floor. Zero is deliberately NOT accepted: it is
	// govpp's spelling of "no deadline", the defect this removes.
	minReplyTimeout = 1 * time.Second
	// maxReplyTimeout is the shared contract ceiling, not a local choice: see
	// the same clamp in the nft backend and the histogram's last finite bucket,
	// all derived from firewall.MaxBackendDeadline.
	maxReplyTimeout = firewall.MaxBackendDeadline
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.firewall.vpp.reply-timeout",
	Type:        "duration",
	Default:     "10s",
	Description: "Bound on each VPP binary-API round-trip; a wedged dataplane fails the apply instead of stalling every firewall owner",
})

// vppReplyTimeout returns the per-request deadline, clamped to
// [minReplyTimeout, maxReplyTimeout]. An out-of-range or unparseable value
// clamps rather than disabling the bound, because an unbounded call is the
// failure mode this guard exists to prevent.
func vppReplyTimeout() time.Duration {
	d := env.GetDuration("ze.firewall.vpp.reply-timeout", defaultReplyTimeout)
	return min(max(d, minReplyTimeout), maxReplyTimeout)
}

// newGovppOps builds the ops facade over a channel and installs the reply
// deadline on it, in that order, before any request can be sent.
//
// The deadline is bound HERE rather than at the call site so it cannot be
// forgotten: every production request goes through a govppOps, and there is no
// way to construct one that skips the bound. A computed-but-uninstalled
// deadline is indistinguishable from having none, and govpp's default is none.
func newGovppOps(ch api.Channel) *govppOps {
	ch.SetReplyTimeout(vppReplyTimeout())
	return &govppOps{ch: ch}
}

// asDataplaneTimeout tags a request VPP accepted and never answered as
// firewall.ErrKernelTimeout, so an owner can tell "the dataplane is wedged"
// from "the ruleset was rejected". Any other error is returned unchanged.
//
// ONE cause qualifies: core.ErrReplyTimeout, a binary-API request that ran out
// its reply deadline.
//
// VPP being ABSENT is deliberately NOT tagged, though the connect phase also
// times out (waitConnector and WaitConnected each bound their wait). The
// sentinel drives two decisions and neither fits an absent dataplane:
// ze_firewall_apply_timeout_total counts wedged reconciles, and ddos-local
// SKIPS its rollback reconcile on this error because a wedged VPP would only
// burn a second deadline. With VPP down there is no ruleset to be behind and
// nothing to wedge: the reconcile failed because the dataplane is not running,
// which is a different operational condition with a different fix. "Not there"
// is not "not answering".
func asDataplaneTimeout(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, core.ErrReplyTimeout) {
		return fmt.Errorf("%w: %w", firewall.ErrKernelTimeout, err)
	}
	return err
}
