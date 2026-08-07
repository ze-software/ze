// Design: docs/architecture/core-design.md -- nftables backend Linux implementation
// Related: backend_linux.go -- newBackend, which installs this deadline on every dial

//go:build linux

package firewallnft

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/nftables"
	"github.com/mdlayher/netlink"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/env"
)

const (
	// defaultNetlinkTimeout bounds one nftables netlink round-trip.
	//
	// This is load-bearing rather than cosmetic: firewall.ApplyAll holds the
	// process-wide reconcileMu across Backend.Apply, so an unbounded kernel call
	// stalls EVERY firewall owner (copp, policy-routes, ddos-local, the firewall
	// engine), not merely concurrent reconciles of the same owner.
	defaultNetlinkTimeout = 10 * time.Second
	// minNetlinkTimeout is the floor. Zero is deliberately NOT accepted: "no
	// deadline" is the defect this exists to remove, so it must not be
	// reachable by configuration.
	minNetlinkTimeout = 1 * time.Second
	// maxNetlinkTimeout is the shared contract ceiling, not a local choice:
	// the apply-latency histogram's last finite bucket is derived from the same
	// constant, so raising one here alone would silently push every
	// max-deadline timeout into +Inf (firewall.MaxBackendDeadline).
	maxNetlinkTimeout = firewall.MaxBackendDeadline
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.firewall.nft.netlink-timeout",
	Type:        "duration",
	Default:     "10s",
	Description: "Bound on each nftables netlink round-trip; a wedged kernel fails the apply instead of stalling every firewall owner",
})

// netlinkTimeout returns the per-operation netlink deadline, clamped to
// [minNetlinkTimeout, maxNetlinkTimeout]. An out-of-range or unparseable value
// clamps rather than disabling the bound, because an unbounded call is the
// failure mode this guard exists to prevent.
func netlinkTimeout() time.Duration {
	d := env.GetDuration("ze.firewall.nft.netlink-timeout", defaultNetlinkTimeout)
	return min(max(d, minNetlinkTimeout), maxNetlinkTimeout)
}

// withNetlinkDeadline builds the ConnOption that bounds every netlink
// round-trip.
//
// The deadline is per-DIAL, not per-Conn: ze's nftables.Conn is not lasting
// (AsLasting is never passed), so dialNetlink opens a fresh netlink socket for
// each operation and re-applies every SockOption to it
// (vendor/github.com/google/nftables/conn.go dialNetlink). The closure below
// therefore re-evaluates time.Now() on each dial, giving each operation a full
// deadline instead of one absolute instant that lapses after the first call.
func withNetlinkDeadline(d time.Duration) nftables.ConnOption {
	return nftables.WithSockOptions(netlinkDeadlineOption(d))
}

// netlinkDeadlineOption is the SockOption itself, split out so a test can apply
// the real thing to a real netlink socket rather than reimplementing it.
func netlinkDeadlineOption(d time.Duration) nftables.SockOption {
	return func(c *netlink.Conn) error {
		return c.SetDeadline(time.Now().Add(d))
	}
}

// asKernelTimeout tags a deadline-exceeded netlink error as
// firewall.ErrKernelTimeout so callers can tell "the kernel is wedged" from
// "the ruleset was rejected". Any other error is returned unchanged.
func asKernelTimeout(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("%w: %w", firewall.ErrKernelTimeout, err)
	}
	return err
}
