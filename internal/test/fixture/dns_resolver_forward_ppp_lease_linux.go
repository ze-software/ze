//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: dns_resolver_forward_learned_linux.go -- forwardAwaitAnswer, forwardStartConfiguredStub and the records
// Related: dns_source_router_follows_lease_linux.go -- leaseNegotiate, the IPCP lease this scenario reuses

package fixture

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The LAN resolver's listener and its configured forward server in the
// PPP-lease scenario. Both sit on the per-test namespace's loopback.
var (
	forwardPPPListener   = netip.MustParseAddrPort("127.0.23.73:53")
	forwardPPPConfigured = netip.MustParseAddrPort("127.0.23.74:53")
)

// dnsResolverForwardPPPLease proves spec-home-cpe-23b AC-6 through the
// daemon: under `forward dns-source source [ received configured ]`, the LAN
// resolver forwards to the configured server while no lease exists, to the
// server a PPPoE lease supplied through IPCP once the session is up
// (leaseDNSServer, whose stub alone answers forwardLearnedRecord), and back
// to the configured server when the concentrator ends the session with a
// PADT, which withdraws the ppp record.
func dnsResolverForwardPPPLease(ctx context.Context, _ *sdk.Plugin) error {
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	configured, err := forwardStartConfiguredStub(forwardPPPConfigured)
	if err != nil {
		return err
	}
	defer configured.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	if !forwardAwaitAnswer(runCtx, forwardPPPListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("the LAN resolver on %s never forwarded to the configured server %s while no lease existed",
			forwardPPPListener, forwardPPPConfigured)
	}

	fd, peer, err := renegotiationOpenAC()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last read is done; a close error changes no verdict.

	// RFC 2516 Section 5.
	session, err := peer.Accept(runCtx)
	if err != nil {
		return fmt.Errorf("access concentrator: discovery: %w", err)
	}
	if err := leaseNegotiate(runCtx, session); err != nil {
		return fmt.Errorf("access concentrator: negotiation: %w", err)
	}
	if !forwardAwaitAnswer(runCtx, forwardPPPListener, forwardLearnedRecord, nil) {
		return fmt.Errorf("the LAN resolver never forwarded to the IPCP lease's server %s", leaseDNSServer)
	}

	// RFC 2516 Section 5.5: the PADT ends the session, and its lease with it.
	if err := session.SendPADT(); err != nil {
		return fmt.Errorf("access concentrator: PADT: %w", err)
	}
	if !forwardAwaitAnswer(runCtx, forwardPPPListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("the LAN resolver did not fall back to the configured server %s after the PPPoE session ended",
			forwardPPPConfigured)
	}

	var tb textbuf.Buffer
	return tb.Str("OK: dns-resolver forward -> configured, then the IPCP lease's server ").Str(leaseDNSServer.String()).
		Str(", then configured again after PADT\n").StdErr()
}
