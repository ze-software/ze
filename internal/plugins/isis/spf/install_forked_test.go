// VALIDATES: the IS-IS installer prefers the local Loc-RIB in-process (no RPC) and
// ships to the remote sink when forked (nil local Loc-RIB), for both address
// families -- AC-4 plus the forked install path.
// PREVENTS: a forked IS-IS silently dropping routes (the original bug), or an
// in-process installer needlessly emitting route-install RPCs.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

type remoteSpy struct {
	inserts []locrib.Path
	removes int
	flushes int
}

func (s *remoteSpy) InsertForward(_ family.Family, _ netip.Prefix, p locrib.Path) {
	s.inserts = append(s.inserts, p)
}

func (s *remoteSpy) Remove(_ family.Family, _ netip.Prefix, _ redistevents.ProtocolID, _ uint32) {
	s.removes++
}

func (s *remoteSpy) Flush() { s.flushes++ }

func forkedRoute(pfx, nh string) RouteEntry {
	return RouteEntry{
		Prefix: netip.MustParsePrefix(pfx), Metric: 10, Level: Level1,
		NextHops: []NextHop{{Addr: netip.MustParseAddr(nh)}},
	}
}

// TestISISInstallerInProcessSkipsRemoteSink: AC-4 -- with a local Loc-RIB the
// installer writes it directly and never touches the remote sink.
func TestISISInstallerInProcessSkipsRemoteSink(t *testing.T) {
	loc := locrib.NewRIB()
	spy := &remoteSpy{}
	in := NewInstaller(loc)
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{forkedRoute("10.40.0.0/24", "10.0.0.2")})
	if len(spy.inserts) != 0 {
		t.Errorf("remote sink InsertForward called %d times in-process; want 0", len(spy.inserts))
	}
	if _, ok := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.40.0.0/24")); !ok {
		t.Error("route not present in local Loc-RIB in-process")
	}
}

// TestISISInstallerForkedUsesRemoteSink: a forked IPv4 installer ships to the sink.
func TestISISInstallerForkedUsesRemoteSink(t *testing.T) {
	spy := &remoteSpy{}
	in := NewInstaller(nil)
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{forkedRoute("10.41.0.0/24", "10.0.0.2")})
	if len(spy.inserts) != 1 {
		t.Fatalf("remote sink got %d inserts when forked; want 1", len(spy.inserts))
	}
	if spy.inserts[0].Source != ProtocolID() {
		t.Errorf("shipped Source = %d, want IS-IS ProtocolID %d", spy.inserts[0].Source, ProtocolID())
	}
}

// TestISISInstallerV6ForkedUsesRemoteSink: the IPv6 installer (isis-12) also ships
// forked routes to the sink -- both families are covered.
func TestISISInstallerV6ForkedUsesRemoteSink(t *testing.T) {
	spy := &remoteSpy{}
	in := NewInstallerV6(nil)
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{{
		Prefix: netip.MustParsePrefix("2001:db8::/64"), Metric: 10, Level: Level1,
		NextHops: []NextHop{{Addr: netip.MustParseAddr("fe80::2")}},
	}})
	if len(spy.inserts) != 1 {
		t.Fatalf("v6 remote sink got %d inserts when forked; want 1", len(spy.inserts))
	}
}
