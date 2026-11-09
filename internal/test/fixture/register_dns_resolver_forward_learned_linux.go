//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: dns_resolver_forward_learned_linux.go -- the scenarios registered below
// Related: dns_resolver_forward_ppp_lease_linux.go -- the PPPoE lease scenario registered below

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
