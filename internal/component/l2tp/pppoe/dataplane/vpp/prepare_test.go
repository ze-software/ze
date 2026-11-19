// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared VPP access identity
// Related: prepare.go -- Prepare and accessPreparation.borrow
package vpp

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// Preparation preserves the actual namespace and CP discovery identity while
// session construction borrows the DP MAC/index and the very same API epoch.
func TestVPPPreparePublishesActualControlAndDataIdentities(t *testing.T) {
	fixture := newAccessPreparationFixture()
	access, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	if access.Device() != "Ethernet0.100" {
		t.Fatalf("DP device=%q", access.Device())
	}
	if access.ControlDevice() != "cp0.100" {
		t.Fatalf("CP device=%q", access.ControlDevice())
	}
	if access.Namespace() != "dataplane" {
		t.Fatalf("namespace=%q", access.Namespace())
	}
	if access.DiscoveryIfIndex() != 73 {
		t.Fatalf("Linux index=%d", access.DiscoveryIfIndex())
	}
	if access.HWAddr() != [6]byte{2, 1, 1, 1, 1, 1} {
		t.Fatalf("CP MAC=%v", access.HWAddr())
	}
	if access.MTU() != 1500 {
		t.Fatalf("effective MTU=%d", access.MTU())
	}
	if fixture.wire.bindings[11] != 42 {
		t.Fatal("published access has no control map")
	}
	preparation := fixture.preparations.records["wan"]
	snapshot, err := preparation.borrow(t.Context(), fixture.request())
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.release()
	if snapshot.api != fixture.connection {
		t.Fatal("session captured another API generation")
	}
	if snapshot.dpIndex != 11 {
		t.Fatalf("session index=%d", snapshot.dpIndex)
	}
	if snapshot.localMAC != [6]byte{2, 1, 1, 1, 1, 1} {
		t.Fatalf("DP MAC=%v", snapshot.localMAC)
	}
}

// A malformed resolved pairing or mismatched Linux readback cannot install
// a map. A constructor refusal after an add must release the confirmed owner.
func TestVPPPrepareRefusalsLeaveNoOwnedMapping(t *testing.T) {
	for _, stage := range []string{"resolve", "index", "same-pair", "control-name", "control-index", "control-mac", "data-mac", "mtu", "namespace-path"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newAccessPreparationFixture()
			want := errors.New("actual LCP lookup failed")
			switch stage {
			case "resolve":
				fixture.resolveError = want
			case "index":
				fixture.pair.DataplaneIndex = 99
			case "same-pair":
				fixture.pair.ControlPlaneIndex = 11
			case "control-name":
				fixture.control.name = "another"
			case "control-index":
				fixture.control.index = 99
			case "control-mac":
				fixture.control.mac = [6]byte{}
			case "data-mac":
				fixture.binding.OperMAC = "malformed"
			case "mtu":
				fixture.control.mtu = 0
			case "namespace-path":
				fixture.pair.Namespace = "../foreign"
			}
			access, err := fixture.prepare(t.Context())
			if err == nil {
				t.Fatal("invalid access published")
			}
			if access != nil {
				t.Fatal("refusal retained an access")
			}
			if stage == "resolve" {
				if !errors.Is(err, want) {
					t.Fatalf("resolution error lost: %v", err)
				}
			}
			if len(fixture.wire.bindings) != 0 {
				t.Fatalf("refusal leaked map %v", fixture.wire.bindings)
			}
			if len(fixture.preparations.records) != 0 {
				t.Fatal("refusal leaked prepared access")
			}
		})
	}
}

// Duplicate preparation of one logical access shares its immutable snapshot;
// neither repeated Close nor another lease can prematurely remove its map.
func TestVPPPrepareDuplicateLeaseRetainsOriginal(t *testing.T) {
	fixture := newAccessPreparationFixture()
	first, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	preparation := fixture.preparations.records["wan"]
	if fixture.wire.added != 1 {
		t.Fatalf("mapping adds=%d", fixture.wire.added)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := preparation.borrow(t.Context(), dataplane.Request{AccessName: "wan", Device: "wrong"}); err == nil {
		t.Fatal("session ignored prepared device identity")
	}
	var joined sync.WaitGroup
	for range 8 {
		joined.Go(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	joined.Wait()
	if fixture.wire.removed != 1 {
		t.Fatalf("mapping deletes=%d", fixture.wire.removed)
	}
	if _, err := preparation.borrow(t.Context(), fixture.request()); err == nil {
		t.Fatal("closed access admitted session construction")
	}
}

// A new API epoch or another data MAC cannot replace an active preparation
// under the same logical name, even when the control pair numbers are reused.
func TestVPPPrepareRefusesActiveIdentityReplacement(t *testing.T) {
	for _, change := range []string{"epoch", "data-MAC"} {
		t.Run(change, func(t *testing.T) {
			fixture := newAccessPreparationFixture()
			first, err := fixture.prepare(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			preparation := fixture.preparations.records["wan"]
			t.Cleanup(func() {
				if err := first.Close(); err != nil {
					t.Error(err)
				}
			})
			if change == "epoch" {
				next := fixture.wire.api()
				next.generation = fixture.connection.generation + 1
				fixture.connection = next
			} else {
				fixture.binding.OperMAC = "02:09:09:09:09:09"
				fixture.control.mac = [6]byte{2, 9, 9, 9, 9, 9}
			}
			access, err := fixture.prepare(t.Context())
			if err == nil {
				t.Fatal("active identity was replaced")
			}
			if access != nil {
				t.Fatal("replacement returned an access")
			}
			if fixture.wire.removed != 0 {
				t.Fatal("refusal removed the original map")
			}
			snapshot, err := preparation.borrow(t.Context(), fixture.request())
			if err != nil {
				t.Fatal(err)
			}
			snapshot.release()
		})
	}
}

// Every request coordinate comes from the prepared access. None can be
// replaced by an arbitrary CP device or namespace at the admission boundary.
func TestVPPPreparedLookupRefusesIdentityAndContextMismatch(t *testing.T) {
	fixture := newAccessPreparationFixture()
	access, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	preparation := fixture.preparations.records["wan"]
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, field := range []string{"access", "data", "control", "namespace", "canceled"} {
		t.Run(field, func(t *testing.T) {
			request := fixture.request()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch field {
			case "access":
				request.AccessName = "unknown"
			case "data":
				request.Device = "different"
			case "control":
				request.ControlDevice = "different"
			case "namespace":
				request.Namespace = "different"
			case "canceled":
				cancel()
			}
			snapshot, err := preparation.borrow(ctx, request)
			if err == nil {
				snapshot.release()
				t.Fatal("mismatched construction admitted")
			}
			if field == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("context error lost: %v", err)
				}
			}
		})
	}
}

// Closing an access must wait until its borrowed construction snapshot is
// released. Wait on actual writer contention, rather than a guessed sleep,
// before checking that the map and the constructor's lease remain intact.
func TestVPPPreparedConstructionPinsMap(t *testing.T) {
	fixture := newAccessPreparationFixture()
	access, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	preparation := fixture.preparations.records["wan"]
	snapshot, err := preparation.borrow(t.Context(), fixture.request())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(snapshot.release)
	closed := make(chan error, 1)
	go func() { closed <- access.Close() }()
	t.Cleanup(func() {
		snapshot.release()
		if err := <-closed; err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// Bounded by ctx; TryRLock observes the actual waiting close writer.
	for preparation.mu.TryRLock() {
		preparation.mu.RUnlock()
		select {
		case <-ctx.Done():
			t.Fatal("access close never reached its construction fence")
		default:
		}
		runtime.Gosched()
	}
	if fixture.wire.bindings[11] != 42 {
		t.Fatal("access close removed a map during construction")
	}
	select {
	case err := <-closed:
		closed <- err
		t.Fatal("access close returned before constructor release")
	default:
	}
}

type accessPreparationFixture struct {
	wire         *accessMapFixture
	connection   *binaryAPI
	preparations *accessPreparations
	maps         *controlMapOwners
	binding      iface.Binding
	pair         iface.LCPPair
	control      controlLink
	resolveError error
}

func newAccessPreparationFixture() *accessPreparationFixture {
	wire := newAccessMapFixture()
	return &accessPreparationFixture{wire: wire, connection: wire.api(),
		preparations: &accessPreparations{records: make(map[string]*accessPreparation)}, maps: newControlMapOwners(),
		binding: iface.Binding{OsName: "Ethernet0.100", Ifindex: 11, OperMAC: "02:01:01:01:01:01", MTU: 1600},
		pair:    iface.LCPPair{DataplaneIndex: 11, ControlPlaneIndex: 42, LinuxIndex: 73, ControlDevice: "cp0.100", Namespace: "dataplane"},
		control: controlLink{name: "cp0.100", index: 73, mac: [6]byte{2, 1, 1, 1, 1, 1}, mtu: 1500}}
}
func (f *accessPreparationFixture) request() dataplane.Request {
	return dataplane.Request{AccessName: "wan", Device: "Ethernet0.100", ControlDevice: "cp0.100", Namespace: "dataplane"}
}
func (f *accessPreparationFixture) prepare(ctx context.Context) (dataplane.Access, error) {
	return prepareAccess(ctx, "wan", accessPreparationOps{
		resolve: func(_ context.Context, name string) (iface.Binding, iface.LCPPair, error) {
			if name != "wan" {
				return iface.Binding{}, iface.LCPPair{}, errors.New("logical access name was changed")
			}
			return f.binding, f.pair, f.resolveError
		},
		control: func(_ context.Context, pair iface.LCPPair) (controlLink, error) {
			if pair != f.pair {
				return controlLink{}, errors.New("actual LCP pair was changed")
			}
			return f.control, nil
		},
		api: func(context.Context) (*binaryAPI, error) { return f.connection, nil },
	}, f.preparations, f.maps)
}
