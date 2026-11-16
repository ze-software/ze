// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6 carrier worker
// Related: ipv6_service.go -- DHCPv6 codec and prefix lifetime

package ppp

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/ze-software/ze/internal/core/pacer"
)

type dhcpv6Reader interface {
	ReadDHCPv6(context.Context, []byte) (int, netip.AddrPort, error)
	WriteDHCPv6([]byte, netip.AddrPort) (int, error)
}

// startDHCPv6Server owns one worker. Its stop joins that worker without closing
// the carrier: RA still owes the final advertisement on the same lifetime.
func startDHCPv6Server(carrier IPv6Carrier, svc *IPv6Service, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), ifname string, logger *slog.Logger) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); dhcpv6ServerLoop(ctx, carrier, svc, serverID, allocPrefix, ifname, logger) }()
	return func() { cancel(); <-done }
}

func dhcpv6ServerLoop(ctx context.Context, carrier dhcpv6Reader, svc *IPv6Service, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), ifname string, logger *slog.Logger) {
	var payload [1500]byte
	var p pacer.Pacer
	// One worker lives until its session context ends.
	for {
		if ctx.Err() != nil {
			return
		}
		n, peer, err := carrier.ReadDHCPv6(ctx, payload[:])
		if err != nil {
			if normalIPv6CarrierStop(ctx, err) {
				return
			}
			reportIPv6Failure(logger, ifname, "DHCPv6 read", err)
			countReaderError(loopDHCPv6)
			if p.Wait(ctx.Done()) {
				return
			}
			continue
		}
		p.Succeed()
		if n < 4 {
			continue
		}
		if n > len(payload) {
			reportIPv6Failure(logger, ifname, "DHCPv6 payload", fmt.Errorf("carrier returned %d bytes into a %d-byte receive buffer", n, len(payload)))
			continue
		}
		message, err := parseDHCPv6(payload[:n])
		if err != nil {
			reportIPv6Failure(logger, ifname, "DHCPv6 parse", err)
			continue
		}
		response, err := svc.handleDHCPv6(message, serverID, allocPrefix)
		if err != nil {
			reportIPv6Failure(logger, ifname, "DHCPv6 handle", err)
			continue
		}
		if response == nil {
			continue
		}
		if _, err := carrier.WriteDHCPv6(response, peer); err != nil {
			if normalIPv6CarrierStop(ctx, err) {
				return
			}
			reportIPv6Failure(logger, ifname, "DHCPv6 write", err)
		}
	}
}
