//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: dns_resolver_forward_ppp_lease_linux.go -- the PPPoE lease scenario this one starts the resolver after
// Related: dns_source_router_follows_lease_linux.go -- leaseNegotiate and leaseDNSServer, the IPCP lease reused here
// Related: dns_source_router_merge_linux.go -- dnsSourceReadResolvConf, the resolv.conf reader reused here
// Related: reload_fixture_live_stages.go -- liveStagesReload, the reload that starts the resolver

package fixture

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// forwardPPPReplayListener is the LAN resolver's listener in
// test/pppoe/dns-resolver-forward-ppp-replay.ci, on the per-test namespace's
// loopback.
var forwardPPPReplayListener = netip.MustParseAddrPort("127.0.23.93:53")

// forwardPPPReplayResolvConf is the resolv.conf the daemon writes in its
// working directory, which is the fixture's working directory too.
const forwardPPPReplayResolvConf = "dns-resolver-forward-ppp-replay.resolv.conf"

// dnsResolverForwardPPPReplay proves spec-home-cpe-23b AC-13 through the
// daemon. The boot config has no `service` block at all, so the dns-resolver
// plugin, whose config root is `service`, is not running. The scripted access
// concentrator brings up a PPPoE lease naming leaseDNSServer, and only once
// the daemon's resolv.conf lists that server, which the registry writes from
// the same ppp record, does reloadSource add `service dns-resolver`. The
// plugin then starts after the record exists and learns it only from the
// replay it requests once started. Its first answer MUST already use it: a
// reply that is not NOERROR, or another record, fails the run at once, where
// a timeout or a refused connection only means the listener is not bound yet.
// A starting plugin is the restart case: its learned list is empty until the
// replay, exactly as after a respawn.
func dnsResolverForwardPPPReplay(ctx context.Context, plugin *sdk.Plugin, reloadSource string) error {
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
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
	if err := forwardPPPReplayAwaitRecord(runCtx); err != nil {
		return err
	}

	if err := liveStagesReload(runCtx, plugin, reloadSource); err != nil {
		return fmt.Errorf("reload %s: %w", reloadSource, err)
	}
	var first error
	answered := Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		got, err := forwardAnswer(runCtx, forwardPPPReplayListener)
		if err == nil {
			if got != forwardLearnedRecord {
				first = fmt.Errorf("the resolver's first answer was %s, not %s from the lease's server %s",
					got, forwardLearnedRecord, leaseDNSServer)
			}
			return true
		}
		// forwardAnswer names a DNS reply that is not NOERROR this way; any
		// other error is a socket that does not answer yet.
		if strings.Contains(err.Error(), "answered rcode") {
			first = fmt.Errorf("the resolver started after the lease answered before it applied the replay: %w", err)
			return true
		}
		return false
	})
	if !answered {
		return fmt.Errorf("the resolver started by %s never answered on %s", reloadSource, forwardPPPReplayListener)
	}
	if first != nil {
		return first
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns-resolver started after the PPPoE lease -> first answer from the replayed lease server ").
		Str(leaseDNSServer.String()).Str("\n").StdErr()
	return nil
}

// forwardPPPReplayAwaitRecord waits until the daemon's resolv.conf lists
// leaseDNSServer. The registry writes that file from its records, so the
// ppp record the replay must carry exists once it does.
func forwardPPPReplayAwaitRecord(ctx context.Context) error {
	var listed []netip.Addr
	var readErr error
	if Poll(ctx, dnsSourceFileAttempts, dnsSourceFileDelay, func() bool {
		listed, readErr = dnsSourceReadResolvConf(forwardPPPReplayResolvConf)
		return readErr == nil && slices.Contains(listed, leaseDNSServer)
	}) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("the daemon's %s never listed the lease's server %s: %w",
			forwardPPPReplayResolvConf, leaseDNSServer, readErr)
	}
	return fmt.Errorf("the daemon's %s lists %v, never the lease's server %s",
		forwardPPPReplayResolvConf, listed, leaseDNSServer)
}
