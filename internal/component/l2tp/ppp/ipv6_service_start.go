// Design: docs/research/l2tpv2-ze-integration.md -- session-owned IPv6 service start
// Related: ra_carrier.go -- solicitation and advertisement worker lifetime
// Related: dhcpv6_carrier.go -- DHCPv6 worker lifetime

package ppp

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
)

// startIPv6Service installs the negotiated link-local pair and the accepted
// Framed-IPv6-Routes, and reconciles them before acquiring the carrier. The service owns its carrier until DHCPv6 and RS readers join
// and the final Router Advertisement has been sent. A failed startup returns
// its service only when resource cleanup failed; the caller MUST retain that
// owner so Stop can retry before the session enters another network lifetime.
func startIPv6Service(cfg iPv6ServiceConfig, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), logger *slog.Logger) (*IPv6Service, error) {
	if cfg.Dataplane == nil {
		return nil, errors.New("PPP IPv6 service has no data-plane handle")
	}
	svc := newIPv6Service(cfg)
	if err := svc.installLinkLocal(); err != nil {
		return rollbackIPv6ServiceStart(svc, err)
	}
	// RFC 3162 Section 2.5: the routes resolve through the pair just
	// installed, and are in place before the carrier can forward IPv6.
	if err := svc.installFramedRoutes(); err != nil {
		return rollbackIPv6ServiceStart(svc, err)
	}
	// The initial channel connection precedes IPv6 service startup. The new
	// pair is staged only: reconcile after installation and before any carrier
	// can publish IPv6 traffic. Startup follows assignment acknowledgment.
	if err := cfg.Dataplane.Connect(); err != nil {
		return rollbackIPv6ServiceStart(svc, err)
	}
	carrier, err := cfg.Dataplane.OpenIPv6Carrier(context.Background())
	svc.carrier = carrier
	if err != nil {
		if carrier != nil {
			svc.stop = carrier.Close
		}
		return rollbackIPv6ServiceStart(svc, err)
	}
	if carrier == nil {
		return rollbackIPv6ServiceStart(svc, errors.New("PPP data plane returned no IPv6 carrier"))
	}
	identity := cfg.Dataplane.Identity()
	raStop := startRASender(carrier, identity, logger)
	dhcpStop := startDHCPv6Server(carrier, svc, serverID, allocPrefix, identity.InterfaceName, logger)
	svc.stop = func() error { dhcpStop(); return raStop() }
	return svc, nil
}

// rollbackIPv6ServiceStart keeps the existing cleanup owner when a withdrawal
// failed. Its caller MUST preserve a returned service even with a nonnil error.
func rollbackIPv6ServiceStart(service *IPv6Service, cause error) (*IPv6Service, error) {
	if cleanupErr := service.Stop(); cleanupErr != nil {
		return service, errors.Join(cause, cleanupErr)
	}
	return nil, cause
}
