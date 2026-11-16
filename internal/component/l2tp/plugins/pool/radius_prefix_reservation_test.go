// Design: docs/architecture/l2tp/subscriber-session-model.md -- exclusive ownership of subscriber prefixes

package l2tppool

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

func radiusPrefixPool(t *testing.T) *poolPlugin {
	t.Helper()
	first, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8:100::/61"), 64)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8:100:8::/61"), 64)
	if err != nil {
		t.Fatal(err)
	}
	return &poolPlugin{
		v6pool: first, v6namedPools: map[string]*ipv6PrefixPool{"other": second},
	}
}

func radiusPrefixRequest(t *testing.T, p *poolPlugin, session uint16, prefix netip.Prefix) l2tp.PrefixResult {
	t.Helper()
	l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session), &l2tp.AuthMetadata{
		DelegatedIPv6Prefix: prefix,
	})
	t.Cleanup(func() {
		l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session))
	})
	return p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: session})
}

// TestPoolRADIUSPrefixReservesAllOverlappingSlots requests one RADIUS /60 over
// two disjoint configured /61 blocks. Every covered /64 slot remains unavailable
// until that subscriber releases its prefix, across both pool names.
func TestPoolRADIUSPrefixReservesAllOverlappingSlots(t *testing.T) {
	p := radiusPrefixPool(t)
	prefix := netip.MustParsePrefix("2001:db8:100::/60")
	answer := radiusPrefixRequest(t, p, 10, prefix)
	if !answer.OK {
		t.Fatalf("RADIUS prefix refused: %s", answer.Reason)
	}
	if answer.Prefix != prefix {
		t.Fatalf("RADIUS prefix = %s, want %s", answer.Prefix, prefix)
	}
	for name, pool := range map[string]*ipv6PrefixPool{"default": p.v6pool, "other": p.v6namedPools["other"]} {
		_, used, free := pool.stats()
		if used != 8 {
			t.Fatalf("%s reserved slots = %d, want 8", name, used)
		}
		if free != 0 {
			t.Fatalf("%s free slots = %d, want 0", name, free)
		}
		request := l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 11}
		if name == "other" {
			request.PoolName = name
		}
		if next := p.handlePrefix(request); next.OK {
			t.Fatalf("%s assigned %s inside the RADIUS prefix", name, next.Prefix)
		}
	}
	p.releasePrefix(radiusTestTunnel, 10)
	for name, pool := range map[string]*ipv6PrefixPool{"default": p.v6pool, "other": p.v6namedPools["other"]} {
		_, used, free := pool.stats()
		if used != 0 {
			t.Fatalf("%s reserved slots after release = %d, want 0", name, used)
		}
		if free != 8 {
			t.Fatalf("%s free slots after release = %d, want 8", name, free)
		}
	}
	if next := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 12}); !next.OK {
		t.Fatalf("released default slots unavailable: %s", next.Reason)
	}
	if next := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 13, PoolName: "other"}); !next.OK {
		t.Fatalf("released named slots unavailable: %s", next.Reason)
	}
}

// TestPoolRADIUSPrefixOverlappingHeldIsRefused attempts both a covering prefix
// and a more-specific prefix against a live owner. Neither dynamic allocation
// nor an external RADIUS assignment may gain an overlapping second owner.
func TestPoolRADIUSPrefixOverlappingHeldIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dynamic   bool
		held      string
		requested string
	}{
		{name: "covering-dynamic", dynamic: true, requested: "2001:db8:100::/60"},
		{name: "inside-radius", held: "2001:db8:100::/60", requested: "2001:db8:100:1::/64"},
		{name: "outside-pools", held: "2001:db8:200::/60", requested: "2001:db8:200:1::/64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := radiusPrefixPool(t)
			if tc.dynamic {
				first := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 20})
				if !first.OK {
					t.Fatalf("dynamic prefix refused: %s", first.Reason)
				}
			} else {
				first := radiusPrefixRequest(t, p, 20, netip.MustParsePrefix(tc.held))
				if !first.OK {
					t.Fatalf("first RADIUS prefix refused: %s", first.Reason)
				}
			}
			requested := netip.MustParsePrefix(tc.requested)
			second := radiusPrefixRequest(t, p, 21, requested)
			if second.OK {
				t.Fatalf("RADIUS admitted overlapping prefix %s", second.Prefix)
			}
			if !strings.Contains(second.Reason, requested.String()) {
				t.Fatalf("refusal %q does not name requested prefix %s", second.Reason, requested)
			}
			if _, exists := p.sessionPrefixes.Load(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 21)); exists {
				t.Fatal("refused RADIUS prefix created a session record")
			}
			p.releasePrefix(radiusTestTunnel, 20)
			third := radiusPrefixRequest(t, p, 22, requested)
			if !third.OK {
				t.Fatalf("released prefix still refused: %s", third.Reason)
			}
		})
	}
}

// TestPoolRADIUSPrefixOutsideRangesIsPreserved keeps a unique external RADIUS
// prefix admissible even without a configured pool. Its ownership ends at
// release, after which another subscriber may receive the same prefix.
func TestPoolRADIUSPrefixOutsideRangesIsPreserved(t *testing.T) {
	p := &poolPlugin{}
	prefix := netip.MustParsePrefix("2001:db8:200::/60")
	first := radiusPrefixRequest(t, p, 30, prefix)
	if !first.OK {
		t.Fatalf("external RADIUS prefix refused: %s", first.Reason)
	}
	if first.Prefix != prefix {
		t.Fatalf("external prefix = %s, want %s", first.Prefix, prefix)
	}
	p.releasePrefix(radiusTestTunnel, 30)
	if _, remains := p.sessionPrefixes.Load(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 30)); remains {
		t.Fatal("released external RADIUS prefix retained its session record")
	}
	if second := radiusPrefixRequest(t, p, 31, prefix); !second.OK {
		t.Fatalf("released external prefix refused: %s", second.Reason)
	}
}

// TestPoolRADIUSPrefixCannotOverlapCarriedDelegation changes the pool's
// delegation length while a /60 remains live, then attempts to assign one of
// its /64 children through RADIUS. A reserved carried slot cannot be bypassed.
func TestPoolRADIUSPrefixCannotOverlapCarriedDelegation(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 60),
	})); err != nil {
		t.Fatal(err)
	}
	if first := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 40}); !first.OK {
		t.Fatal(first.Reason)
	}
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 64),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	prefix := netip.MustParsePrefix("2001:db8:100:1::/64")
	if second := radiusPrefixRequest(t, p, 41, prefix); second.OK {
		t.Fatalf("RADIUS bypassed carried delegation with %s", second.Prefix)
	}
	p.releasePrefix(radiusTestTunnel, 40)
	if third := radiusPrefixRequest(t, p, 42, prefix); !third.OK {
		t.Fatalf("released carried delegation still blocks RADIUS: %s", third.Reason)
	}
}

// TestPoolRADIUSDisjointPrefixesSharePoolSlot preserves two disjoint RADIUS
// /64s inside one allocatable /60. The coarse slot stays reserved until both
// subscribers release their distinct prefixes.
func TestPoolRADIUSDisjointPrefixesSharePoolSlot(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8:100::/60"), 60)
	if err != nil {
		t.Fatal(err)
	}
	p := &poolPlugin{v6pool: pool}
	for session, prefix := range map[uint16]string{50: "2001:db8:100::/64", 51: "2001:db8:100:1::/64"} {
		answer := radiusPrefixRequest(t, p, session, netip.MustParsePrefix(prefix))
		if !answer.OK {
			t.Fatalf("disjoint RADIUS prefix %s refused: %s", prefix, answer.Reason)
		}
	}
	if _, used, _ := pool.stats(); used != 1 {
		t.Fatalf("coarse slot reservations = %d, want 1", used)
	}
	p.releasePrefix(radiusTestTunnel, 50)
	if _, used, _ := pool.stats(); used != 1 {
		t.Fatalf("coarse slot was released while another RADIUS prefix remains: %d", used)
	}
	if next := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 52}); next.OK {
		t.Fatalf("coarse prefix %s overlaps the remaining RADIUS subscriber", next.Prefix)
	}
	p.releasePrefix(radiusTestTunnel, 51)
	if next := p.handlePrefix(l2tp.PrefixRequest{TunnelID: radiusTestTunnel, SessionID: 53}); !next.OK {
		t.Fatalf("coarse slot unavailable after final RADIUS release: %s", next.Reason)
	}
}
