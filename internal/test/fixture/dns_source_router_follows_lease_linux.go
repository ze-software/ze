//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: register_dns_source_router_follows_lease_linux.go -- the registration of the driver below
// Related: pppoe_client_renegotiation_linux.go -- the concentrator socket and credentials reused here
// Related: plugin_fixture_dns_stub.go -- the stub zone and the answer rendering reused here
// RFC 1877 Section 1.1 -- see rfc/short/rfc1877.md.

package fixture

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// leaseDNSServer is the server the concentrator offers in IPCP. The .ci
// gives it to a dummy link of the test's namespace (option=netns-link) and
// runs `le mock dns --port 53 --address` on it. It is not a loopback
// address because the PPPoE client adopts only a global unicast DNS server
// (adoptNakDNS, internal/component/l2tp/pppoeclient/session.go).
var leaseDNSServer = netip.MustParseAddr("192.0.2.53")

// leaseLookup is answered by the stub zone alone: .test is reserved
// (RFC 6761 Section 6.2), so no other resolver gives this record.
const leaseLookup = "show dns lookup web.example.test type A"

// leaseStubRecord is the stub zone's A record for web.example.test.
const leaseStubRecord = "203.0.113.10"

// leaseLookupAttempts bounds the wait for the daemon to follow the lease.
const leaseLookupAttempts = 40

// dnsSourceRouterFollowsLease proves spec-home-cpe-23's router consumer
// through the daemon: with no name-server configured, the internal resolver
// asks the server a PPPoE lease supplied. Before the lease, a lookup of the
// stub's name does not return the stub's record. The scripted concentrator
// then brings the session up and Naks the client's 0.0.0.0 Primary-DNS
// request with leaseDNSServer, and the same lookup must answer from the
// stub, which only that server reaches.
func dnsSourceRouterFollowsLease(ctx context.Context, plugin *sdk.Plugin) error {
	acCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// No lease exists yet: the client's discovery waits for Accept. An error
	// answer is acceptable here; only the stub's record is refused.
	var before map[string]any
	_, beforeErr := Dispatch(acCtx, plugin, leaseLookup, &before)
	if beforeErr == nil && dnsRecordsEqual(before, leaseStubRecord) {
		return fmt.Errorf("%s answered %s before any lease named the stub's server: the test cannot tell the lease's effect",
			leaseLookup, dnsAnswerLine(before))
	}

	fd, peer, err := renegotiationOpenAC()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last read is done; a close error changes no verdict.

	// RFC 2516 Section 5.
	session, err := peer.Accept(acCtx)
	if err != nil {
		return fmt.Errorf("access concentrator: discovery: %w", err)
	}
	if err := leaseNegotiate(acCtx, session); err != nil {
		return fmt.Errorf("access concentrator: negotiation: %w", err)
	}

	data, err := fixture06PollObject(acCtx, plugin, leaseLookup, leaseLookupAttempts,
		func(answer map[string]any) bool { return dnsRecordsEqual(answer, leaseStubRecord) })
	if err != nil {
		return fmt.Errorf("the daemon never resolved web.example.test through the lease's server %s: %w", leaseDNSServer, err)
	}

	var tb textbuf.Buffer
	return tb.Str("OK: web.example.test type A -> ").Str(dnsAnswerLine(data)).
		Str(" through the IPCP lease's server ").Str(leaseDNSServer.String()).Str("\n").StdErr()
}

// leaseNegotiate plays LCP with CHAP-MD5, the CHAP challenge, then IPCP
// offering leaseDNSServer as the only DNS server.
func leaseNegotiate(ctx context.Context, session *pppoeac.Session) error {
	// Wire: MRU (type 1, length 4), Authentication-Protocol CHAP with MD5
	// (type 3, length 5, 0xc223, 5), Magic-Number (type 5, length 6).
	options := []byte{1, 4, 0, 0, 3, 5, 0xc2, 0x23, 5, 5, 6, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(options[2:4], 1450)
	binary.BigEndian.PutUint32(options[11:15], 0x0a0b0c0d)
	// RFC 1661 Sections 4.1 and 5.1.
	if err := session.LCP(ctx, 70, options, false); err != nil {
		return fmt.Errorf("LCP: %w", err)
	}
	// RFC 1994 Sections 4.1 and 4.2.
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 80}, renegotiationUser, renegotiationSecret); err != nil {
		return fmt.Errorf("CHAP: %w", err)
	}
	// RFC 1332 Section 3.3, RFC 1877 Section 1.1.
	concentrator := netip.MustParseAddr("100.64.93.1")
	client := netip.MustParseAddr("100.64.93.2")
	if err := session.IPCPWithDNS(ctx, 90, concentrator, client, []netip.Addr{leaseDNSServer}); err != nil {
		return fmt.Errorf("IPCP: %w", err)
	}
	return nil
}
