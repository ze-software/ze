// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: ra_rdnss.go -- raRDNSS.withdrawPPPoESession, the hook under test
// Related: pppoe_client.go -- PPPoEClient.endSessionLeases, which calls it

package iface

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/config/system"
)

// recordsOf answers how many of the registry's records carry origin.
func recordsOf(r *system.Resolvers, origin string) int {
	count := 0
	records := r.Snapshot().Records
	for i := range records {
		if records[i].Origin == origin {
			count++
		}
	}
	return count
}

// TestRARDNSSWithdrawnWhenPPPoESessionEnds checks that a pppoe-client's RA
// record leaves the registry when its session ends, with the session's IPCP
// lease: the servers were reached over the session device, which the session
// takes with it, so keeping them leaves resolv.conf naming a server behind a
// link that no longer exists. Method: a client wired as runEngine wires it
// (sessionEnded is the producer's withdrawPPPoESession) dials a fake session
// that leases one IPCP server, an RDNSS option of infinite Lifetime arrives on
// its device, then the session's Done closes and both records are read back.
// The infinite Lifetime means no expiry can be what removed the record.
//
// VALIDATES: spec-home-cpe-23 Phase 2a, a pppoe-client session's RA record
// ends with the session.
// PREVENTS: an RA server learned on a dead PPPoE session staying selected
// until a commit or a Lifetime 0 that never comes.
func TestRARDNSSWithdrawnWhenPPPoESessionEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		cfg := pppoeRATargets()
		p.setTargets(cfg)

		done := make(chan struct{})
		sess := pppoeTestSession(done)
		sess.PrimaryDNS = netip.MustParseAddr("192.0.2.53")
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		client := NewPPPoEClient(pppoeClientConfigFrom(cfg.PPPoEClient[0]), &fakeDialer{sess: sess}, nil, log)
		client.resolvers = resolvers
		client.sessionEnded = p.withdrawPPPoESession
		client.Start()
		t.Cleanup(client.Stop)
		synctest.Wait()

		p.handle(userOptEvent(t, "pppoe0", rdnssOption(t, 0xffffffff, "2001:db8::53")))
		if got := recordsOf(resolvers, "ra"); got != 1 {
			t.Fatalf("ra records %d while the session is up, want 1", got)
		}
		if got := recordsOf(resolvers, "ppp"); got != 1 {
			t.Fatalf("ppp records %d while the session is up, want 1", got)
		}

		close(done)
		synctest.Wait()
		if got := recordsOf(resolvers, "ppp"); got != 0 {
			t.Fatalf("ppp records %d after the session ended, want 0", got)
		}
		if got := recordsOf(resolvers, "ra"); got != 0 {
			t.Fatalf("ra records %d after the session ended, want 0: the RA server outlived its link", got)
		}
	})
}
