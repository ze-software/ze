// Design: docs/architecture/traffic/cos-dynamic.md -- shared legacy map custody
//go:build ze_l2tp

package cos

import (
	"encoding/json"
	"errors"
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	sharedCoSTestInterface = "eth0.317"
	sharedCoSTestTunnel    = uint16(31317)
	sharedCoSTestFirst     = "shared-first"
	sharedCoSTestAlias     = "shared-alias"
	sharedCoSTestDifferent = "shared-different"
)

// These unit observations model the native backend boundary only. The separate
// unchanged Linux questions exercise actual netlink writes in a namespace.
type sharedCoSTestOwner struct {
	t          *testing.T
	bus        *testBus
	handler    *cosHandler
	original   coreCos.Profile
	installed  coreCos.Profile
	shared     coreCos.Profile
	reads      int
	writes     int
	failWrites int
}

func newSharedCoSTestOwner(t *testing.T) *sharedCoSTestOwner {
	t.Helper()
	original := coreCos.Profile{IngressMap: map[uint32]uint32{1: 3}, EgressMap: map[uint32]uint32{3: 1}}
	shared := coreCos.Profile{IngressMap: map[uint32]uint32{2: 5}, EgressMap: map[uint32]uint32{5: 2}}
	f := &sharedCoSTestOwner{t: t, bus: newTestBus(), original: original,
		installed: coreCos.Profile{IngressMap: maps.Clone(original.IngressMap), EgressMap: maps.Clone(original.EgressMap)},
		shared:    shared}
	oldProfiles := coreCos.All()
	coreCos.Replace(map[string]coreCos.Profile{
		sharedCoSTestFirst:     shared,
		sharedCoSTestAlias:     {IngressMap: map[uint32]uint32{0: 0, 2: 5}, EgressMap: map[uint32]uint32{0: 0, 5: 2}},
		sharedCoSTestDifferent: {IngressMap: map[uint32]uint32{4: 6}, EgressMap: map[uint32]uint32{6: 4}},
	})
	t.Cleanup(func() { coreCos.Replace(oldProfiles) })
	f.handler = newCosHandler(f.bus, func(name string, ingress, egress map[uint32]uint32) error {
		if name != sharedCoSTestInterface {
			t.Fatalf("map write changed access: %s", name)
		}
		f.writes++
		f.installed = coreCos.Profile{IngressMap: maps.Clone(ingress), EgressMap: maps.Clone(egress)}
		if f.failWrites > 0 {
			f.failWrites--
			return errors.New("native write outcome is not confirmed")
		}
		return nil
	}, nil)
	f.handler.readLink = func(name string) (*iface.InterfaceInfo, error) {
		if name != sharedCoSTestInterface {
			t.Fatalf("native read changed access: %s", name)
		}
		f.reads++
		return &iface.InterfaceInfo{Name: name, Index: 317, Type: "vlan",
			IngressQoSMap: maps.Clone(f.installed.IngressMap), EgressQoSMap: maps.Clone(f.installed.EgressMap)}, nil
	}
	t.Cleanup(f.handler.stop)
	return f
}

func (f *sharedCoSTestOwner) up(session uint16, profile string) {
	f.t.Helper()
	storeMetadataForTest(sharedCoSTestTunnel, session, profile)
	f.t.Cleanup(func() {
		clearMetadataForTest(sharedCoSTestTunnel, session)
		sessionStore.Delete(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session})
	})
	emitSessionUp(f.t, f.bus, &l2tpevents.SessionUpPayload{
		SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(sharedCoSTestTunnel), session),
		TunnelID:     sharedCoSTestTunnel, SessionID: session, AccessInterface: sharedCoSTestInterface,
	})
}

func (f *sharedCoSTestOwner) down(session uint16) {
	f.t.Helper()
	emitSessionDown(f.t, f.bus, &l2tpevents.SessionDownPayload{TunnelID: sharedCoSTestTunnel, SessionID: session})
}

func (f *sharedCoSTestOwner) requireMaps(want coreCos.Profile) {
	f.t.Helper()
	if !nativeCoSEffectiveMapsEqualForUnit(f.installed.IngressMap, want.IngressMap) {
		f.t.Fatalf("installed ingress=%v, want %v", f.installed.IngressMap, want.IngressMap)
	}
	if !nativeCoSEffectiveMapsEqualForUnit(f.installed.EgressMap, want.EgressMap) {
		f.t.Fatalf("installed egress=%v, want %v", f.installed.EgressMap, want.EgressMap)
	}
}

// Enumerate the eight supported keys independently of the production matcher.
func nativeCoSEffectiveMapsEqualForUnit(actual, want map[uint32]uint32) bool {
	for key := range uint32(8) {
		if actual[key] != want[key] {
			return false
		}
	}
	return true
}

func (f *sharedCoSTestOwner) command(operation string, session uint16, profile, token string) (string, error) {
	f.t.Helper()
	request := sessionTransactionRequest{Operation: operation, Token: token}
	if operation == "prepare" {
		request.TunnelID = sharedCoSTestTunnel
		request.SessionID = session
		request.Interface = sharedCoSTestInterface
		request.Profile = profile
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		f.t.Fatal(err)
	}
	status, data, err := f.handler.executeSessionTransaction([]string{"transaction", string(encoded)}, "192.0.2.31")
	if err != nil {
		return "", err
	}
	if status != rpc.StatusDone {
		f.t.Fatalf("transaction returned status=%s: %#v", status, data)
	}
	result, ok := data.(sessionTransactionResult)
	if !ok {
		f.t.Fatalf("transaction returned %T", data)
	}
	return result.Token, nil
}

// One actual pre-first-write snapshot must survive aliases and both departures.
// MUTATION: capture per user or restore the first user's maps on nonlast Down.
func TestCoSLegacySharedCapturesNativeBaselineOnce(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(401, sharedCoSTestFirst)
	f.up(402, sharedCoSTestAlias)
	f.requireMaps(f.shared)
	if f.reads != 1 {
		t.Fatalf("native lifetime baseline reads=%d, want one", f.reads)
	}
	f.down(401)
	f.requireMaps(f.shared)
	f.down(402)
	f.requireMaps(f.original)
	if f.reads != 1 {
		t.Fatalf("departures recaptured the original maps: reads=%d", f.reads)
	}
}

// A failed final restore keeps its original owner and blocks a conflicting use;
// retrying that same departure must finish restoration without a new snapshot.
// MUTATION: delete session/owner before the native final restore succeeds.
func TestCoSLegacySharedRetainsFailedFinalRestore(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(411, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	f.failWrites = 1
	f.down(411)
	key := sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 411}
	if _, retained := sessionStore.Load(key); !retained {
		t.Fatal("failed final restore lost its original session custody")
	}
	writes := f.writes
	f.up(412, sharedCoSTestDifferent)
	if f.writes != writes {
		t.Fatal("uncertain final restore admitted a new map writer")
	}
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 412}); admitted {
		t.Fatal("uncertain final restore published a conflicting member")
	}
	f.down(411)
	f.requireMaps(f.original)
	if _, retained := sessionStore.Load(key); retained {
		t.Fatal("confirmed restore kept a departed member")
	}
	if f.reads != 1 {
		t.Fatalf("failed restore recaptured maps: reads=%d", f.reads)
	}
}

// A session with no initial Filter-Id joins the same original owner on first
// CoA; its later departure must not restore the other user's installed maps.
func TestCoSLegacySharedFirstCoAJoinsOriginalBaseline(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(421, sharedCoSTestFirst)
	f.up(422, "")
	token, err := f.command("prepare", 422, sharedCoSTestAlias, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"apply", "commit"} {
		if _, err := f.command(operation, 0, "", token); err != nil {
			t.Fatal(err)
		}
	}
	f.requireMaps(f.shared)
	f.down(421)
	f.requireMaps(f.shared)
	f.down(422)
	f.requireMaps(f.original)
}

// Rolling back a newly joined CoA member restores membership as well as maps:
// both original users survive, and no phantom member prevents final restore.
func TestCoSLegacySharedRollbackRemovesOnlyNewMember(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(431, sharedCoSTestFirst)
	f.up(432, sharedCoSTestAlias)
	t.Cleanup(func() { sessionStore.Delete(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 433}) })
	token, err := f.command("prepare", 433, sharedCoSTestAlias, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 433}); retained {
		t.Fatal("rollback retained the new member")
	}
	f.requireMaps(f.shared)
	f.down(431)
	f.requireMaps(f.shared)
	f.down(432)
	f.requireMaps(f.original)
}

// A prepared sole-user transition reserves this access until rollback; another
// user's compatible old maps may join only after that reservation is resolved.
func TestCoSLegacySharedPrepareProtectsReservation(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(441, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	token, err := f.command("prepare", 441, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	writes := f.writes
	f.up(442, sharedCoSTestAlias)
	if f.writes != writes {
		t.Fatal("legacy event wrote through a pending access reservation")
	}
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 442}); admitted {
		t.Fatal("legacy event published through a pending access reservation")
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.up(442, sharedCoSTestAlias)
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 442}); !admitted {
		t.Fatal("resolved reservation still refused a compatible user")
	}
	f.requireMaps(f.shared)
}

// A committed change remains compensatable. A newcomer cannot acquire maps
// whose later compensation would overwrite that newcomer's admitted policy.
func TestCoSLegacySharedCommittedCompensationProtectsAdmission(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(451, sharedCoSTestFirst)
	token, err := f.command("prepare", 451, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"apply", "commit"} {
		if _, err := f.command(operation, 0, "", token); err != nil {
			t.Fatal(err)
		}
	}
	different, found := coreCos.Lookup(sharedCoSTestDifferent)
	if !found {
		t.Fatal("healthy committed profile disappeared")
	}
	f.requireMaps(different)
	writes := f.writes
	f.up(452, sharedCoSTestDifferent)
	if f.writes != writes {
		t.Fatal("new legacy admission wrote through outstanding compensation")
	}
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 452}); admitted {
		t.Fatal("new user made the retained compensation incompatible")
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 451}); !admitted {
		t.Fatal("compensation lost the original admitted user")
	}
	f.up(452, sharedCoSTestAlias)
	if _, admitted := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 452}); !admitted {
		t.Fatal("completed rollback still refused a compatible new user")
	}
	f.requireMaps(f.shared)
}
