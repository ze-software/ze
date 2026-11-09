//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: dns_resolver_forward_learned_linux.go -- the scenarios registered below
// Related: dns_resolver_forward_ppp_lease_linux.go -- the PPPoE lease scenario registered below
// Related: dns_resolver_forward_dhcpv4_lease_linux.go -- the DHCPv4 lease scenarios registered below

package fixture

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	// The daemon creates each veth pair from the .ci config; the fixtures
	// advertise on its far end and create the stubs.
	Register("plugin/dns-resolver-forward-received",
		observer02("dns-resolver-forward-received-test", dnsResolverForwardReceived))
	Register("plugin/dns-resolver-forward-default", dnsResolverForwardDefaultDriver)
	// The scripted access concentrator brings up the PPPoE lease (AC-6).
	Register("pppoe/dns-resolver-forward-ppp-lease",
		observer02("dns-resolver-forward-ppp-lease-test", dnsResolverForwardPPPLease))
	// The daemon's own DHCP server leases to its own DHCP client across the
	// veth pair the .ci config creates (AC-20, AC-13).
	Register("plugin/dns-resolver-forward-dhcpv4-default",
		reloadingDriver("plugin/dns-resolver-forward-dhcpv4-default", "dns-resolver-forward-dhcpv4-default-test",
			dnsResolverForwardDHCPv4Default))
	Register("plugin/dns-resolver-forward-dhcpv4-replay",
		reloadingDriver("plugin/dns-resolver-forward-dhcpv4-replay", "dns-resolver-forward-dhcpv4-replay-test",
			dnsResolverForwardDHCPv4Replay))
}

// reloadingDriver builds the driver of a scenario that takes one argument,
// the config file it reloads, and observes as the plugin named plugin.
func reloadingDriver(name, plugin string,
	scenario func(context.Context, *sdk.Plugin, string) error,
) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return errors.New(name + " takes one argument: the config file it reloads")
		}
		return Observe(ctx, plugin, &sdk.Registration{},
			func(ctx context.Context, p *sdk.Plugin) error {
				return scenario(ctx, p, args[0])
			})
	}
}

// dnsResolverForwardDefaultDriver takes one argument, the config file the
// scenario reloads to add a forward server.
func dnsResolverForwardDefaultDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("plugin/dns-resolver-forward-default takes one argument: the config file that adds a forward server")
	}
	return Observe(ctx, "dns-resolver-forward-default-test", &sdk.Registration{},
		func(ctx context.Context, plugin *sdk.Plugin) error {
			return dnsResolverForwardDefault(ctx, plugin, args[0])
		})
}
