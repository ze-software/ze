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

	// RFC 5072 Section 5, RFC 4861 Section 4.2: the link-local source the RA
	// sender and the DHCPv6 server answer from exists before either starts.
	if err := svc.installLinkLocal(); err != nil {
		return nil, err
	}

	raStop, err := startRASender(cfg.Ifname, logger)
	if err != nil {
		svc.removeLinkLocal()
		return nil, err
	}

	dhcpStop, err := startDHCPv6Server(cfg.Ifname, svc, serverID, allocPrefix, logger)
	if err != nil {
		raStop()
		svc.removeLinkLocal()
		return nil, err
	}

	svc.stop = func() {
		dhcpStop()
		raStop()
	}
	return svc, nil
}
