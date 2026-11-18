//go:build integration && linux && ze_l2tp

// Design: docs/architecture/traffic/cos-dynamic.md -- shared access arbitration.
// Related: handler.go and session_transaction.go -- existing legacy entry points.
package cos

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These questions exercise the existing legacy CoS owner with actual native
// map writes and readback. Session metadata/events are existing legacy inputs;
// they do not mint a PPP network, original native command grant, rate outcome,
// receiver retirement receipt or authenticated forked-process authority.
type nativeCoSSharedFixture struct {
	t        *testing.T
	backend  iface.Backend
	bus      *testBus
	handler  *cosHandler
	device   string
	original coreCos.Profile
	shared   coreCos.Profile
	writes   int
}

func newNativeCoSSharedFixture(t *testing.T) *nativeCoSSharedFixture {
	t.Helper()
	resetCoSReportsForTest(t)
	if iface.GetBackend() != nil {
		t.Fatal("isolated shared CoS namespace has an unrelated backend")
	}
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Errorf("close shared CoS backend: %v", err)
		}
	})
	backend := iface.GetBackend()
	if backend == nil {
		t.Fatal("selected shared CoS backend is absent")
	}
	if err := backend.CreateDummy("cosshare0"); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateVLAN(iface.VLANSpec{
		Parent: "cosshare0", VLANID: 317,
		IngressQoSMap: map[uint32]uint32{1: 3},
		EgressQoSMap:  map[uint32]uint32{3: 1},
	}); err != nil {
		t.Fatal(err)
	}
	device := "cosshare0.317"
	before, err := backend.GetLink(device)
	if err != nil {
		t.Fatal(err)
	}
	if before == nil {
		t.Fatal("created shared CoS VLAN is absent")
	}
	f := &nativeCoSSharedFixture{t: t, backend: backend, bus: newTestBus(), device: device,
		original: coreCos.Profile{IngressMap: maps.Clone(before.IngressQoSMap), EgressMap: maps.Clone(before.EgressQoSMap)},
		shared:   coreCos.Profile{IngressMap: map[uint32]uint32{2: 5}, EgressMap: map[uint32]uint32{5: 2}},
	}
	coreCos.Replace(map[string]coreCos.Profile{
		"shared-first": f.shared,
		"shared-alias": {IngressMap: maps.Clone(f.shared.IngressMap), EgressMap: maps.Clone(f.shared.EgressMap)},
		"conflicting":  {IngressMap: map[uint32]uint32{4: 6}, EgressMap: map[uint32]uint32{6: 4}},
	})
	t.Cleanup(func() { coreCos.Replace(nil) })
	f.handler = newCosHandler(f.bus, func(name string, ingress, egress map[uint32]uint32) error {
		f.writes++
		return backend.UpdateVLANQoSMap(name, ingress, egress)
	}, discardCoSReportForTest)
	f.handler.readLink = backend.GetLink
	captureTarget := observeCreatorCoSTargets(f)
	f.handler.captureTarget = func(name string) (originalCoSTarget, error) {
		if name != device {
			t.Fatalf("static map resolution changed native access: %q", name)
		}
		return captureTarget(name)
	}
	t.Cleanup(func() { closeNativeCoSTestTargets(t, f.handler) })
	return f
}

func (f *nativeCoSSharedFixture) up(session uint16, profile string) {
	f.t.Helper()
	storeMetadataForTest(31, session, profile)
	f.t.Cleanup(func() {
		clearMetadataForTest(31, session)
		sessionStore.Delete(fixtureCoSKey(31, session))
	})
	emitSessionUp(f.t, f.bus, &l2tpevents.SessionUpPayload{
		Birth: fixtureCoSKey(31, session).birth, SubscriberID: fixtureCoSID(31, session),
		TunnelID: 31, SessionID: session, AccessInterface: f.device,
	})
}

func (f *nativeCoSSharedFixture) requireMaps(want coreCos.Profile) {
	f.t.Helper()
	link, err := f.backend.GetLink(f.device)
	if err != nil {
		f.t.Fatal(err)
	}
	if link == nil {
		f.t.Fatal("shared CoS access disappeared")
	}
	if !nativeCoSEffectiveMapsEqual(link.IngressQoSMap, want.IngressMap) {
		f.t.Fatalf("shared CoS ingress: got %v, want %v", link.IngressQoSMap, want.IngressMap)
	}
	if !nativeCoSEffectiveMapsEqual(link.EgressQoSMap, want.EgressMap) {
		f.t.Fatalf("shared CoS egress: got %v, want %v", link.EgressQoSMap, want.EgressMap)
	}
}

// A missing entry and an explicit zero select the same kernel default. Read
// both maps so every unexpected or missing nonzero value remains visible.
func nativeCoSEffectiveMapsEqual(actual, want map[uint32]uint32) bool {
	for from, to := range actual {
		if to != want[from] {
			return false
		}
	}
	for from, to := range want {
		if to != actual[from] {
			return false
		}
	}
	return true
}

// Equal effective maps permit both profile names. The survivor keeps its maps;
// only the final departure restores the actual pre-first-user native snapshot.
// MUTATION: restoring per-session static maps on the first departure loses the survivor.
func TestCoSLegacySharedNativeAccessPreservesSurvivor(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeCoSSharedFixture(t)
	f.up(41, "shared-first")
	f.up(42, "shared-alias")
	f.requireMaps(f.shared)
	if _, found := sessionStore.Load(fixtureCoSKey(31, 42)); !found {
		t.Fatal("equal effective maps did not admit the second legacy user")
	}
	writes := f.writes
	emitSessionDown(t, f.bus, &l2tpevents.SessionDownPayload{Birth: fixtureCoSKey(31, 41).birth, SubscriberID: fixtureCoSID(31, 41), TunnelID: 31, SessionID: 41})
	f.requireMaps(f.shared)
	if f.writes != writes {
		t.Fatal("first legacy departure wrote shared native maps")
	}
	if _, found := sessionStore.Load(fixtureCoSKey(31, 42)); !found {
		t.Fatal("first departure removed the permitted survivor")
	}
	emitSessionDown(t, f.bus, &l2tpevents.SessionDownPayload{Birth: fixtureCoSKey(31, 42).birth, SubscriberID: fixtureCoSID(31, 42), TunnelID: 31, SessionID: 42})
	f.requireMaps(f.original)
}

// A conflicting initial legacy class cannot replace maps used by an admitted
// survivor. The handler's event API supplies no ACK; this makes no ACK claim.
// MUTATION: admitting conflicting effective maps writes over the first user.
func TestCoSLegacySharedNativeAccessRejectsConflictingInitialClass(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeCoSSharedFixture(t)
	f.up(41, "shared-first")
	f.requireMaps(f.shared)
	writes := f.writes
	f.up(42, "conflicting")
	f.requireMaps(f.shared)
	if f.writes != writes {
		t.Fatal("conflicting initial legacy class attempted a native write")
	}
	if _, found := sessionStore.Load(fixtureCoSKey(31, 42)); found {
		t.Fatal("conflicting initial legacy class acquired shared ownership")
	}
	if _, found := sessionStore.Load(fixtureCoSKey(31, 41)); !found {
		t.Fatal("conflict removed the permitted existing user")
	}
}

// The existing acknowledged CoA transaction must arbitrate the same users as
// initial legacy admission. Equal maps remain a positive prepare/apply/commit
// control; a different map must refuse prepare before any native write.
// MUTATION: preparing CoA without survivor compatibility returns a token for conflict.
func TestCoSLegacySharedNativeAccessRejectsConflictingCoAPrepare(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeCoSSharedFixture(t)
	f.up(41, "shared-first")
	f.up(42, "shared-alias")
	f.requireMaps(f.shared)
	call := func(request sessionTransactionRequest) (string, any, error) {
		wire, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return f.handler.executeSessionTransaction([]string{"transaction", string(wire)}, "192.0.2.31")
	}
	status, answer, err := call(sessionTransactionRequest{
		Operation: "prepare", SubscriberID: fixtureCoSID(31, 41), TunnelID: 31, SessionID: 41, Interface: f.device, Profile: "shared-alias",
	})
	if err != nil {
		t.Fatalf("equal-map CoA prepare: %v", err)
	}
	if status != rpc.StatusDone {
		t.Fatalf("equal-map CoA prepare status: %q", status)
	}
	prepared, valid := answer.(sessionTransactionResult)
	if !valid {
		t.Fatalf("equal-map CoA prepare returned %T", answer)
	}
	if prepared.Token == "" {
		t.Fatal("equal-map CoA prepare returned no transaction token")
	}
	for _, operation := range []string{"apply", "commit"} {
		status, _, err := call(sessionTransactionRequest{Operation: operation, Token: prepared.Token})
		if err != nil {
			t.Fatalf("equal-map CoA %s: %v", operation, err)
		}
		if status != rpc.StatusDone {
			t.Fatalf("equal-map CoA %s status: %q", operation, status)
		}
	}
	f.requireMaps(f.shared)
	writes := f.writes
	status, _, err = call(sessionTransactionRequest{
		Operation: "prepare", SubscriberID: fixtureCoSID(31, 41), TunnelID: 31, SessionID: 41, Interface: f.device, Profile: "conflicting",
	})
	if err == nil {
		t.Fatalf("conflicting CoA prepare returned status %q without refusal", status)
	}
	if status != rpc.StatusError {
		t.Fatalf("conflicting CoA prepare status: %q", status)
	}
	if f.writes != writes {
		t.Fatal("conflicting CoA prepare attempted a native write")
	}
	f.requireMaps(f.shared)
}

// Effective maps compare forwarding values, including the kernel's retained
// zero/default entries. A nonzero extra mapping must remain a real mismatch.
// This oracle has no native side effect and supplies no shared-owner proof.
func TestNativeCoSSharedEffectiveMapOracle(t *testing.T) {
	cases := []struct {
		name   string
		actual map[uint32]uint32
		want   map[uint32]uint32
		equal  bool
	}{
		{"retained removed zero", map[uint32]uint32{3: 0, 5: 2}, map[uint32]uint32{5: 2}, true},
		{"explicit wanted zero", map[uint32]uint32{5: 2}, map[uint32]uint32{3: 0, 5: 2}, true},
		{"unexpected nonzero", map[uint32]uint32{3: 1, 5: 2}, map[uint32]uint32{5: 2}, false},
		{"changed value", map[uint32]uint32{5: 6}, map[uint32]uint32{5: 2}, false},
		{"missing nonzero", nil, map[uint32]uint32{5: 2}, false},
		{"only actual defaults", map[uint32]uint32{3: 0}, nil, true},
		{"only wanted defaults", nil, map[uint32]uint32{3: 0}, true},
	}
	for _, check := range cases {
		t.Run(check.name, func(t *testing.T) {
			equal := nativeCoSEffectiveMapsEqual(check.actual, check.want)
			if equal != check.equal {
				t.Fatalf("effective maps equal=%t, want %t: actual=%v wanted=%v", equal, check.equal, check.actual, check.want)
			}
		})
	}
}
