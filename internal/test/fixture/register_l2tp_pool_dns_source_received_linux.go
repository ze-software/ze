//go:build linux

// Design: docs/features/dns-resolver.md -- the subscriber pool's dns-source list falls through to received
// Related: l2tp_pool_dns_source.go -- poolDNSNegotiate, the subscriber this scenario drives
// Related: dns_source_router_follows_lease_linux.go -- leaseNegotiate and leaseDNSServer, the WAN lease reused here

package fixture

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// poolReceivedLNSPort is the LNS port test/pppoe/l2tp-pool-dns-source-received.ci
// configures. The test runs in its own network namespace, so a fixed port
// collides with nothing.
const poolReceivedLNSPort = 1701

func init() {
	// spec-home-cpe-23b AC-10, the received half: a pool with no dns-primary
	// and no dns-source offers the resolver a WAN lease supplied.
	Register("pppoe/l2tp-pool-dns-source-received",
		observer02("l2tp-pool-dns-source-received-test", poolDNSReceived))
}

// poolDNSReceived brings up a PPPoE lease whose IPCP names leaseDNSServer,
// then opens subscriber sessions on the LNS until one is offered that
// server: the pool learns the record asynchronously, so a session opened
// before the record arrived is offered none and the next one is tried.
func poolDNSReceived(ctx context.Context, _ *sdk.Plugin) error {
	runCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()

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

	want := poolDNSWant{primary: leaseDNSServer}
	var last error
	if Poll(runCtx, 10, 2*time.Second, func() bool {
		last = poolDNSNegotiate(runCtx, poolReceivedLNSPort, want)
		return last == nil
	}) {
		// Stderr: a plugin's stdout carries the plugin protocol.
		return poolDNSVerdict(want).StdErr()
	}
	return fmt.Errorf("no subscriber session was offered the WAN lease's resolver %s: %w", leaseDNSServer, last)
}
