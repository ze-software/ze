//go:build !linux

// Design: rfc/short/rfc5082.md -- the transmit half of the related-message rule
// Overview: route_linux.go -- the implementation this stands in for

package gtsm

import (
	"errors"
	"log/slog"
	"net/netip"
)

// applyHopLimitRoutes reports that the hop-limit route is not installed here.
//
// The route metric is a Linux route attribute (RTAX_HOPLIMIT), and the
// non-Linux build reaches this only in a development environment: ze's daemon
// runs on Linux. It says so once for each peer rather than returning an error:
// an error would make SetPeers retry on every reconcile, and there is nothing
// a retry could install here.
func applyHopLimitRoutes(wanted, previous []Peer) error {
	_ = previous
	for _, p := range wanted {
		if p.HopLimit == 0 {
			continue
		}
		slog.Default().Warn("GTSM hop-limit route needs Linux, ICMP errors to this peer carry the system default TTL",
			"peer", p.Addr.String(), "hop-limit", p.HopLimit)
	}
	return nil
}

// errNeedsLinux says the hop-limit route is a Linux route attribute, so no
// kernel this build runs on can hold it.
var errNeedsLinux = errors.New("the GTSM hop-limit route needs Linux")

// hopLimitRouteInstalled answers false on every other platform: the route
// above is never installed, so a doctor run here reports what the daemon's
// own warning says.
func hopLimitRouteInstalled(Peer) (bool, error) { return false, nil }

// peerRouteResolvable answers the same reason: the route cannot be installed
// here whatever the kernel resolves.
func peerRouteResolvable(netip.Addr) error { return errNeedsLinux }
