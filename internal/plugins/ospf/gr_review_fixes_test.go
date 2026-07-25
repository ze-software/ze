// VALIDATES: the spec-ospf-ext-9 review follow-up fixes.
//   - FIX 1 (AC-12, RFC 3623 sec 2.2 trigger 1): the restarter's all-adjacencies-re-Full exit
//     fires through the PRODUCTION neighbor onFull sink (neighborEventSinkValue), not only via a
//     direct noteAdjacencyFull call.
//   - FIX 2: exitRestart snapshots m.cfg / m.reason under the lock, so a concurrent configure()
//     cannot race the post-unlock exit actions (run this file under -race to exercise it).
//   - FIX 3 (NOTE-3): the `request ospf graceful-restart` operator command drives prepareRestart
//     on the live engine (Grace-LSAs originated + graceful-stop suppression entered), and is
//     refused when graceful-restart is disabled.
//   - FIX 4 (NOTE-5, RFC 3623 sec A grace clock): the IPv4 helper honors the received Grace-LSA
//     LS age, so the grace window is GracePeriod - LSAge and a higher-age retransmit does not
//     reset it; the opaque delivery seam threads the LS age into opaqueReceived.
//
// PREVENTS: a wired-but-never-called restarter exit trigger, a data race on graceful exit, an
// untriggerable planned restart, and a grace clock that ignores the LS age.
package ospf

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRestarterExitViaProductionOnFullPath (FIX 1, AC-12): an active restarter with two
// pre-restart adjacencies exits early -- before the grace timer -- once BOTH re-reach Full via
// the production neighbor event sink (NeighborUp -> onFull -> grNeighborFull -> noteAdjacencyFull),
// exercising the wiring rather than calling noteAdjacencyFull directly.
func TestRestarterExitViaProductionOnFullPath(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	a := ospftypes.RouterID{10, 0, 0, 2}
	b := ospftypes.RouterID{10, 0, 0, 3}
	// A long grace window: only the adjacency trigger, not grace expiry, can end this restart.
	e.gr.enterRestart(now.Add(3600*time.Second), grReasonReload, []ospftypes.RouterID{a, b})

	sink := e.neighborEventSinkValue()
	sink.NeighborUp(ospfneighbor.Snapshot{Interface: "eth0", RouterID: a.String(), State: "full"})
	if !e.gr.inRestart() {
		t.Fatalf("one pre-restart adjacency still outstanding; restarter must stay in restart")
	}
	sink.NeighborUp(ospfneighbor.Snapshot{Interface: "eth0", RouterID: b.String(), State: "full"})
	if e.gr.inRestart() {
		t.Fatalf("both pre-restart adjacencies re-Full via the production onFull path; restarter must exit before grace expiry")
	}
}

// TestExitRestartConfigSnapshotNoRace (FIX 2): exitRestart reads the restart interval and reason
// it feeds to grOriginateGraceLSAs; those must be snapshotted under m.mu, not read after Unlock.
// Under `go test -race` a concurrent configure() (which rewrites m.cfg) would flag the old
// post-unlock read. With the fix the reads are inside the locked section, so no race is reported.
func TestExitRestartConfigSnapshotNoRace(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 300 {
			cfg := grTestConfig()
			cfg.RestartInterval = uint16(100 + (i % 200))
			e.gr.configure(cfg)
		}
	})
	for range 300 {
		e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, nil)
		e.gr.exitRestart(grExitGraceExpiry)
	}
	wg.Wait()
}

// grPrepareEngine builds a GR-enabled IPv4 engine with a router-id and one running interface, so
// prepareRestart originates a real Grace-LSA into the eth0 link store.
func grPrepareEngine(t *testing.T, now time.Time) *engine {
	t.Helper()
	e := grEnableEngine(t, false, now)
	rid := ospftypes.RouterID{10, 0, 0, 1}
	e.mu.Lock()
	e.cfg.RouterID = rid
	e.mu.Unlock()
	e.lsdb.SetSelfRouterID(rid)
	e.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkPointToPoint}
	return e
}

// TestGRPrepareCommandTriggersPrepareRestart (FIX 3, NOTE-3): the `request ospf graceful-restart`
// command handler runs prepareRestart on the live engine -- it originates a Grace-LSA per
// interface and enters the graceful-stop suppression state so the FIB is retained.
func TestGRPrepareCommandTriggersPrepareRestart(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grPrepareEngine(t, now)

	res := e.grPrepare()
	if !res.Prepared || res.Error != "" {
		t.Fatalf("grPrepare on an enabled restarter = %+v, want Prepared=true, no error", res)
	}
	if res.Action != cmdGRPrepare {
		t.Fatalf("grPrepare action = %q, want %q", res.Action, cmdGRPrepare)
	}
	if !e.gr.suppressInstall() {
		t.Fatalf("prepare must enter the graceful-stop suppression state (suppressInstall true)")
	}
	lsas := e.lsdb.LinkLSAs("eth0")
	found := false
	for i := range lsas {
		if lsas[i].OpaqueType() == ospfpacket.GraceOpaqueType {
			found = true
		}
	}
	if !found {
		t.Fatalf("prepare must originate an Opaque Type 3 Grace-LSA on eth0; got %d link LSAs", len(lsas))
	}
}

// TestGRPrepareCommandRefusedWhenDisabled (FIX 3, AC-25): with graceful-restart not configured
// the command is refused (reported, not errored at the transport) and originates nothing.
func TestGRPrepareCommandRefusedWhenDisabled(t *testing.T) {
	e := newEngine(nil)
	res := e.grPrepare()
	if res.Prepared || res.Error == "" {
		t.Fatalf("grPrepare with GR disabled = %+v, want Prepared=false with a refusal message", res)
	}
	if e.gr.suppressInstall() {
		t.Fatalf("a refused prepare must not enter suppression")
	}
}

// TestGraceHelperClockHonorsLSAgeV4 (FIX 4, NOTE-5, RFC 3623 sec A): a v4 Grace-LSA received via
// the production graceOnReceive hook with LS age = N yields a helper grace window of
// (GracePeriod - N); a later retransmit at a HIGHER age reflects the smaller remaining grace and
// does not reset the window to a full period.
func TestGraceHelperClockHonorsLSAgeV4(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	key := helperKey{iface: "eth0", router: x}
	const period = uint32(120)

	// Seed an active helper session (bypassing the Full-adjacency entry gate) at age 0, so the
	// receive path exercises the grace-clock update, not entry.
	e.gr.helperEnter(key, graceReceived{iface: "eth0", advRouter: x, gracePeriod: period}, false, netip.Addr{}, 0)

	recv := func(age uint16) time.Time {
		e.graceOnReceive(opaqueReceived{
			OpaqueType:        ospfpacket.GraceOpaqueType,
			Interface:         "eth0",
			AdvertisingRouter: x,
			Body:              grV4Body(period, grReasonReload, [4]byte{}, false),
			Age:               age,
		})
		end, ok := e.gr.helperGraceEnd(key)
		if !ok {
			t.Fatalf("helper session vanished")
		}
		return end
	}

	const n1 = uint16(30)
	got1 := recv(n1)
	want1 := now.Add(time.Duration(period-uint32(n1)) * time.Second)
	if !got1.Equal(want1) {
		t.Fatalf("grace window with LS age %d = %v, want %v (GracePeriod - LSAge)", n1, got1, want1)
	}

	const n2 = uint16(60)
	got2 := recv(n2)
	want2 := now.Add(time.Duration(period-uint32(n2)) * time.Second)
	if !got2.Equal(want2) {
		t.Fatalf("grace window with LS age %d = %v, want %v (GracePeriod - LSAge)", n2, got2, want2)
	}
	if !got2.Before(got1) {
		t.Fatalf("a higher LS age (%d > %d) must shrink the window, not reset it: got2=%v got1=%v", n2, n1, got2, got1)
	}
}

// TestOpaqueDeliveryThreadsLSAge (FIX 4): the opaque reception seam copies the delivery's LS age
// into opaqueReceived.Age, so a consumer (the Grace-LSA helper) sees the received age.
func TestOpaqueDeliveryThreadsLSAge(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng := newEngine(transport.New(&fakeBackend{}))
	var got []opaqueReceived
	if err := registerOpaqueConsumer(42, OpaqueScopeArea, nil, func(r opaqueReceived) { got = append(got, r) }); err != nil {
		t.Fatalf("register: %v", err)
	}
	const lsAge = uint16(37)
	eng.deliverOpaque(ospflsdb.OpaqueDelivery{
		Scope:             ospftypes.LSTypeOpaqueArea,
		Area:              mustBackboneArea(t),
		AdvertisingRouter: mustRouterID(t, "2.2.2.2"),
		OpaqueType:        42,
		OpaqueID:          0x01,
		Body:              []byte{1, 2, 3, 4},
		Age:               lsAge,
	})
	if len(got) != 1 {
		t.Fatalf("delivered %d times, want 1", len(got))
	}
	if got[0].Age != lsAge {
		t.Fatalf("opaqueReceived.Age = %d, want %d (threaded from the delivery LS age)", got[0].Age, lsAge)
	}
}
