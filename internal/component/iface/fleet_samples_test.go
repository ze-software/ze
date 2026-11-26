// Design: docs/architecture/fleet-config.md — unit tests for interface fleet metric sampling

package iface

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/fleet"
)

func ifaceReadingMap(t *testing.T, snap *fleet.SourceSnapshot) map[string]fleet.SourceReading {
	t.Helper()
	if err := fleet.ValidateSourceSnapshot(fleet.SampleOwnerIface, snap); err != nil {
		t.Fatalf("ValidateSourceSnapshot failed: %v", err)
	}
	out := make(map[string]fleet.SourceReading, len(snap.Readings))
	for _, r := range snap.Readings {
		out[r.Figure] = r
	}
	return out
}

// VALIDATES: physical interface filtering excludes loopback, bridge, vlan,
// vrf, wireguard, tun/tap, ppp/pppoe, macvlan, dummy, veth, and sub-interfaces
// while retaining physical NICs; first tick omits traffic rates and second
// tick computes bps across stable membership.
// PREVENTS: double-counting sub-interfaces/bridges in physical traffic or
// emitting synthetic traffic rates before a baseline exists.
func TestFleetSampleTrackerPhysicalFilterAndTrafficRate(t *testing.T) {
	baseTime := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	now := baseTime

	ticks := [][]InterfaceInfo{
		{
			{Name: "lo", Index: 1, Type: "device", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 9999, TxBytes: 9999}},
			{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 1, Stats: &InterfaceStats{RxBytes: 1000, TxBytes: 2000}},
			{Name: "eth1", Index: 3, Type: "device", State: LinkStateDown, CounterGeneration: 1, Stats: &InterfaceStats{RxBytes: 500, TxBytes: 100}},
			{Name: "eth0.100", Index: 4, Type: "vlan", ParentIndex: 2, VlanID: 100, State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 1000, TxBytes: 2000}},
			{Name: "br0", Index: 5, Type: "bridge", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 5000, TxBytes: 5000}},
			{Name: "wg0", Index: 6, Type: "wireguard", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 500, TxBytes: 500}},
			{Name: "vrf0", Index: 7, Type: "vrf", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
			{Name: "tun0", Index: 8, Type: "tun", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
			{Name: "ppp0", Index: 9, Type: "device", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
			{Name: "dummy0", Index: 10, Type: "dummy", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
			{Name: "veth0", Index: 11, Type: "veth", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
			{Name: "mv0", Index: 12, Type: "macvlan", ParentIndex: 2, Alias: "ze:owned:vrrp", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 10, TxBytes: 10}},
		},
		{
			{Name: "lo", Index: 1, Type: "device", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 99999, TxBytes: 99999}},
			{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 1, Stats: &InterfaceStats{RxBytes: 2000, TxBytes: 4000}},
			{Name: "eth1", Index: 3, Type: "device", State: LinkStateUp, CounterGeneration: 1, Stats: &InterfaceStats{RxBytes: 750, TxBytes: 600}},
			{Name: "br0", Index: 5, Type: "bridge", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 99999, TxBytes: 99999}},
		},
	}
	tickIdx := 0

	tracker := newFleetSampleTracker(
		func() ([]InterfaceInfo, error) {
			out := ticks[tickIdx]
			tickIdx++
			return out, nil
		},
		func() time.Time { return now },
	)

	// Tick 1: 2 physical interfaces (eth0 up, eth1 down); no traffic rate yet.
	snap1, err := tracker.Sample()
	if err != nil {
		t.Fatalf("tick 1 error: %v", err)
	}
	m1 := ifaceReadingMap(t, &snap1)
	if got := m1[fleet.FigureInterfacesConfigured].Value; got != 2 {
		t.Errorf("tick 1 interfaces-configured = %v, want 2", got)
	}
	if got := m1[fleet.FigureInterfacesUp].Value; got != 1 {
		t.Errorf("tick 1 interfaces-up = %v, want 1", got)
	}
	if _, hasIn := m1[fleet.FigureTrafficIn]; hasIn {
		t.Fatalf("tick 1 emitted traffic-in, want omitted")
	}
	if _, hasOut := m1[fleet.FigureTrafficOut]; hasOut {
		t.Fatalf("tick 1 emitted traffic-out, want omitted")
	}

	// Tick 2 (+10s): deltaRx = 1000 + 250 = 1250 bytes -> 1000 bps;
	// deltaTx = 2000 + 500 = 2500 bytes -> 2000 bps.
	now = baseTime.Add(10 * time.Second)
	snap2, err := tracker.Sample()
	if err != nil {
		t.Fatalf("tick 2 error: %v", err)
	}
	m2 := ifaceReadingMap(t, &snap2)
	if got := m2[fleet.FigureInterfacesConfigured].Value; got != 2 {
		t.Errorf("tick 2 interfaces-configured = %v, want 2", got)
	}
	if got := m2[fleet.FigureInterfacesUp].Value; got != 2 {
		t.Errorf("tick 2 interfaces-up = %v, want 2", got)
	}
	if math.Abs(m2[fleet.FigureTrafficIn].Value-1000.0) > 1e-9 {
		t.Errorf("tick 2 traffic-in = %v, want 1000 bps", m2[fleet.FigureTrafficIn].Value)
	}
	if math.Abs(m2[fleet.FigureTrafficOut].Value-2000.0) > 1e-9 {
		t.Errorf("tick 2 traffic-out = %v, want 2000 bps", m2[fleet.FigureTrafficOut].Value)
	}
}

// VALIDATES: membership change, CounterGeneration change, counter rollback,
// and non-advancing elapsed time each reset the traffic baseline and omit
// traffic-in/traffic-out for that tick, then resume on the next stable tick.
// PREVENTS: false traffic spikes when interfaces are hot-plugged, replaced, or reset.
func TestFleetSampleTrackerResetsOnDiscontinuity(t *testing.T) {
	baseTime := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	now := baseTime
	var current []InterfaceInfo

	tracker := newFleetSampleTracker(
		func() ([]InterfaceInfo, error) { return current, nil },
		func() time.Time { return now },
	)

	// Baseline tick.
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 10, Stats: &InterfaceStats{RxBytes: 1000, TxBytes: 1000}},
	}
	if _, err := tracker.Sample(); err != nil {
		t.Fatalf("baseline error: %v", err)
	}

	// Non-advancing clock -> omit traffic.
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 10, Stats: &InterfaceStats{RxBytes: 2000, TxBytes: 2000}},
	}
	snapSameTime, err := tracker.Sample()
	if err != nil {
		t.Fatalf("same-time error: %v", err)
	}
	mSame := ifaceReadingMap(t, &snapSameTime)
	if _, ok := mSame[fleet.FigureTrafficIn]; ok {
		t.Fatal("non-advancing time emitted traffic-in")
	}

	// CounterGeneration change -> reset baseline and omit traffic.
	now = now.Add(10 * time.Second)
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 11, Stats: &InterfaceStats{RxBytes: 3000, TxBytes: 3000}},
	}
	snapGen, err := tracker.Sample()
	if err != nil {
		t.Fatalf("gen-change error: %v", err)
	}
	mGen := ifaceReadingMap(t, &snapGen)
	if _, ok := mGen[fleet.FigureTrafficIn]; ok {
		t.Fatal("CounterGeneration change emitted traffic-in")
	}

	// Counter rollback -> reset baseline and omit traffic.
	now = now.Add(10 * time.Second)
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 11, Stats: &InterfaceStats{RxBytes: 500, TxBytes: 4000}},
	}
	snapRollback, err := tracker.Sample()
	if err != nil {
		t.Fatalf("rollback error: %v", err)
	}
	mRollback := ifaceReadingMap(t, &snapRollback)
	if _, ok := mRollback[fleet.FigureTrafficIn]; ok {
		t.Fatal("counter rollback emitted traffic-in")
	}

	// Membership change (eth1 added) -> reset baseline and omit traffic.
	now = now.Add(10 * time.Second)
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 11, Stats: &InterfaceStats{RxBytes: 600, TxBytes: 4100}},
		{Name: "eth1", Index: 3, Type: "device", State: LinkStateUp, CounterGeneration: 20, Stats: &InterfaceStats{RxBytes: 100, TxBytes: 200}},
	}
	snapMember, err := tracker.Sample()
	if err != nil {
		t.Fatalf("membership-change error: %v", err)
	}
	mMember := ifaceReadingMap(t, &snapMember)
	if _, ok := mMember[fleet.FigureTrafficIn]; ok {
		t.Fatal("membership change emitted traffic-in")
	}

	// Next stable tick (+8s): deltaRx = 100 + 300 = 400 B -> 400 bps; deltaTx = 200 + 200 = 400 B -> 400 bps.
	now = now.Add(8 * time.Second)
	current = []InterfaceInfo{
		{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, CounterGeneration: 11, Stats: &InterfaceStats{RxBytes: 700, TxBytes: 4300}},
		{Name: "eth1", Index: 3, Type: "device", State: LinkStateUp, CounterGeneration: 20, Stats: &InterfaceStats{RxBytes: 400, TxBytes: 400}},
	}
	snapStable, err := tracker.Sample()
	if err != nil {
		t.Fatalf("stable tick error: %v", err)
	}
	mStable := ifaceReadingMap(t, &snapStable)
	if math.Abs(mStable[fleet.FigureTrafficIn].Value-400.0) > 1e-9 {
		t.Errorf("post-reset traffic-in = %v, want 400", mStable[fleet.FigureTrafficIn].Value)
	}
	if math.Abs(mStable[fleet.FigureTrafficOut].Value-400.0) > 1e-9 {
		t.Errorf("post-reset traffic-out = %v, want 400", mStable[fleet.FigureTrafficOut].Value)
	}
}

// VALIDATES: zero physical interfaces up reports real zeros, nil Stats omits
// traffic rates without failing interface counts, and backend errors return an
// error instead of synthetic zeros. Also checks executeCommand dispatch and
// Hidden declaration in commandDecls.
// PREVENTS: absent/zero confusion or hidden command leaking into visible decls.
func TestFleetSampleTrackerEdgeCasesAndCommandWiring(t *testing.T) {
	t.Run("zero-physical-interfaces-up-reports-real-zero", func(t *testing.T) {
		tracker := newFleetSampleTracker(
			func() ([]InterfaceInfo, error) {
				return []InterfaceInfo{
					{Name: "eth0", Index: 2, Type: "device", State: LinkStateDown, Stats: nil},
				}, nil
			},
			time.Now,
		)
		snap, err := tracker.Sample()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := ifaceReadingMap(t, &snap)
		if m[fleet.FigureInterfacesConfigured].Value != 1 {
			t.Errorf("interfaces-configured = %v, want 1", m[fleet.FigureInterfacesConfigured].Value)
		}
		if m[fleet.FigureInterfacesUp].Value != 0 {
			t.Errorf("interfaces-up = %v, want 0", m[fleet.FigureInterfacesUp].Value)
		}
		if _, ok := m[fleet.FigureTrafficIn]; ok {
			t.Fatal("nil Stats emitted traffic-in")
		}
	})

	t.Run("backend-error-returns-error", func(t *testing.T) {
		wantErr := errors.New("netlink failed")
		tracker := newFleetSampleTracker(
			func() ([]InterfaceInfo, error) { return nil, wantErr },
			time.Now,
		)
		if _, err := tracker.Sample(); !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	})

	t.Run("command-decl-hidden-and-executeCommand", func(t *testing.T) {
		foundHidden := false
		for _, decl := range commandDecls() {
			if decl.Name == FleetSourceCommand {
				foundHidden = decl.Hidden
			}
		}
		if !foundHidden {
			t.Fatal("commandDecls missing hidden FleetSourceCommand")
		}

		_, cleanup := withStubBackend(t, []InterfaceInfo{
			{Name: "eth0", Index: 2, Type: "device", State: LinkStateUp, Stats: &InterfaceStats{RxBytes: 100, TxBytes: 200}},
		})
		defer cleanup()

		rt := newRateTracker()
		globalTracker.Store(rt)
		defer globalTracker.Store(nil)

		status, data, err := executeCommand(FleetSourceCommand, nil)
		if err != nil || status != "done" {
			t.Fatalf("executeCommand status=%q err=%v", status, err)
		}
		snap, ok := data.(fleet.SourceSnapshot)
		if !ok {
			t.Fatalf("data type = %T, want fleet.SourceSnapshot", data)
		}
		m := ifaceReadingMap(t, &snap)
		if m[fleet.FigureInterfacesConfigured].Value != 1 || m[fleet.FigureInterfacesUp].Value != 1 {
			t.Fatalf("unexpected counts: %+v", m)
		}
	})
}
