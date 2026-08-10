// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service start (Linux)
// Related: ra_linux.go -- RA sender goroutine
// Related: dhcpv6_linux.go -- DHCPv6 server goroutine

//go:build linux

package ppp

import (
	"log/slog"
	"net/netip"
)

// startIPv6Service creates and starts the RA sender and DHCPv6 server
// goroutines for a PPP session. Returns the service (with stop
// function set) or an error.
func startIPv6Service(cfg iPv6ServiceConfig, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), logger *slog.Logger) (*IPv6Service, error) {
	svc := newIPv6Service(cfg)

	raStop, err := startRASender(cfg.Ifname, logger)
	if err != nil {
		return nil, err
	}

	dhcpStop, err := startDHCPv6Server(cfg.Ifname, svc, serverID, allocPrefix, logger)
	if err != nil {
		raStop()
		return nil, err
	}

	svc.stop = func() {
		dhcpStop()
		raStop()
	}
	return svc, nil
}
