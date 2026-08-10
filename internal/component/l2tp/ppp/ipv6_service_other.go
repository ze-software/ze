// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service stub (non-Linux)
// Related: ipv6_service_linux.go -- real implementation

//go:build !linux

package ppp

import (
	"log/slog"
	"net/netip"
)

func startIPv6Service(_ iPv6ServiceConfig, _ DHCPv6DUID, _ func() (netip.Prefix, bool), _ *slog.Logger) (*IPv6Service, error) {
	return nil, errNotLinux
}
