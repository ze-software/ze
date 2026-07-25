// VALIDATES: the OSPF installer prefers the local Loc-RIB in-process (no RPC) and
// ships to the remote sink when forked (nil local Loc-RIB) -- AC-4 plus the forked
// install/withdraw path.
// PREVENTS: a forked OSPF silently dropping routes (the original bug), or an
// in-process installer needlessly emitting route-install RPCs.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

type spySink struct {
	inserts []locrib.Path
	removes int
	flushes int
}

func (s *spySink) InsertForward(_ family.Family, _ netip.Prefix, p locrib.Path) {
	s.inserts = append(s.inserts, p)
}

func (s *spySink) Remove(_ family.Family, _ netip.Prefix, _ redistevents.ProtocolID, _ uint32) {
	s.removes++
}

func (s *spySink) Flush() { s.flushes++ }

func forkedRoute(t *testing.T, pfx string) RouteEntry {
	t.Helper()
	return RouteEntry{
		AreaID: testArea(), Prefix: netip.MustParsePrefix(pfx), Metric: 10, Type: RouteIntraArea,
		Origin:   testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}
}

// TestOSPFInstallerInProcessSkipsRemoteSink: AC-4 -- with a local Loc-RIB the
// installer writes it directly and never touches the remote sink (no RPC emitted).
func TestOSPFInstallerInProcessSkipsRemoteSink(t *testing.T) {
	loc := locrib.NewRIB()
	spy := &spySink{}
	in := NewInstaller(loc)
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{forkedRoute(t, "10.30.0.0/24")})
	if len(spy.inserts) != 0 {
		t.Errorf("remote sink InsertForward called %d times in-process; want 0", len(spy.inserts))
	}
	if _, ok := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.30.0.0/24")); !ok {
		t.Error("route not present in local Loc-RIB in-process")
	}
}

// TestOSPFInstallerForkedUsesRemoteSink: a forked installer (nil local Loc-RIB)
// ships every route to the remote sink instead of dropping it (the fix).
func TestOSPFInstallerForkedUsesRemoteSink(t *testing.T) {
	spy := &spySink{}
	in := NewInstaller(nil) // forked: locrib.Default() was nil
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{forkedRoute(t, "10.31.0.0/24")})
	if len(spy.inserts) != 1 {
		t.Fatalf("remote sink got %d inserts when forked; want 1 (routes must not drop)", len(spy.inserts))
	}
	if spy.inserts[0].Source != ProtocolID() {
		t.Errorf("shipped Source = %d, want OSPF ProtocolID %d", spy.inserts[0].Source, ProtocolID())
	}
	if spy.flushes != 1 {
		t.Errorf("Apply flushed %d times; want exactly 1 batch per Apply (R-1)", spy.flushes)
	}
}

// TestOSPFInstallerForkedWithdrawsViaRemoteSink: withdrawing routes forked routes
// the Remove to the sink.
func TestOSPFInstallerForkedWithdrawsViaRemoteSink(t *testing.T) {
	spy := &spySink{}
	in := NewInstaller(nil)
	in.SetRemoteSink(spy)
	in.Apply([]RouteEntry{forkedRoute(t, "10.32.0.0/24")})
	in.Apply(nil) // withdraw all
	if spy.removes == 0 {
		t.Error("remote sink Remove not called on withdraw when forked")
	}
}

// TestOSPFInstallerNoSinkNoOp: neither local RIB nor remote sink -- snapshot still
// tracks but nothing is shipped (unwired test / pre-fix no-op behavior preserved).
func TestOSPFInstallerNoSinkNoOp(t *testing.T) {
	in := NewInstaller(nil) // no loc, no remote
	in.Apply([]RouteEntry{forkedRoute(t, "10.33.0.0/24")})
	if got := in.Installed(); len(got) != 1 {
		t.Errorf("snapshot tracked %d routes; want 1 even with no sink", len(got))
	}
}
