//go:build !linux

// Design: rfc/short/rfc5082.md -- the transmit half of the related-message rule
// Overview: route_linux.go -- the implementation this stands in for

package gtsm

import "log/slog"

// applyHopLimitRoutes reports that the hop-limit route is not installed here.
//
// The route metric is a Linux route attribute (RTAX_HOPLIMIT), and the
// non-Linux build reaches this only in a development environment: ze's daemon
// runs on Linux. It says so once for each peer rather than returning an error,
// for the same reason the Linux path does, and so a config apply on a
// developer's machine is not refused for a kernel feature that machine has no
// equivalent of.
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
