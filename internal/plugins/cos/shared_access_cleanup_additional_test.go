// Design: docs/architecture/traffic/cos-dynamic.md -- independent failed-owner progress
// Related: shared_access_cleanup_registered_test.go -- real registered SDK runner

//go:build ze_l2tp

package cos

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// One failed owner must not prevent another original access from recovering.
// Method: drive three original owners through the registered event/SDK route,
// retain two distinct native causes, and prove the third native restore happens.
func TestCoSLegacySharedCleanupKeepsAllCausesAndOtherProgress(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	names := []string{"eth0.317", "eth0.318", "eth0.319"}
	installed := make(map[string]coreCos.Profile, len(names))
	writes := make(map[string]int, len(names))
	failures := make(map[string]error, len(names))
	for _, name := range names {
		installed[name] = coreCos.Profile{
			IngressMap: maps.Clone(f.original.IngressMap), EgressMap: maps.Clone(f.original.EgressMap),
		}
	}
	f.handler.updateQoS = func(name string, ingress, egress map[uint32]uint32) error {
		if _, present := installed[name]; !present {
			return fmt.Errorf("cleanup selected an unrelated access %q", name)
		}
		writes[name]++
		installed[name] = coreCos.Profile{IngressMap: maps.Clone(ingress), EgressMap: maps.Clone(egress)}
		return failures[name]
	}
	f.handler.readLink = func(name string) (*iface.InterfaceInfo, error) {
		profile, present := installed[name]
		if !present {
			return nil, fmt.Errorf("cleanup read an unrelated access %q", name)
		}
		return &iface.InterfaceInfo{Name: name, Index: 317, Type: "vlan",
			IngressQoSMap: maps.Clone(profile.IngressMap), EgressQoSMap: maps.Clone(profile.EgressMap)}, nil
	}
	for index, name := range names {
		session := uint16(62100 + index)
		storeMetadataForTest(sharedCoSTestTunnel, session, sharedCoSTestFirst)
		t.Cleanup(func() {
			clearMetadataForTest(sharedCoSTestTunnel, session)
			sessionStore.Delete(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session})
		})
		if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
			SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(sharedCoSTestTunnel), session),
			TunnelID:     sharedCoSTestTunnel, SessionID: session, AccessInterface: name,
		}); err != nil {
			t.Fatal(err)
		}
		if !nativeCoSEffectiveMapsEqualForUnit(installed[name].IngressMap, f.shared.IngressMap) {
			t.Fatalf("healthy native admission did not install the policy on %s", name)
		}
		failures[name] = fmt.Errorf("native original restore %s is unconfirmed", name)
		registeredCoSCleanupReviewDown(t, bus, session)
		if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}); !retained {
			t.Fatalf("failed original restore on %s lost its session", name)
		}
	}
	delete(failures, names[1])
	before := maps.Clone(writes)
	err := executeCoSCleanupForReview(t, engine, nil)
	if err == nil {
		t.Fatal("the cleanup pass acknowledged two unconfirmed original restorations")
	}
	for _, name := range []string{names[0], names[2]} {
		if !strings.Contains(err.Error(), failures[name].Error()) {
			t.Fatalf("cleanup lost the independent original cause for %s: %v", name, err)
		}
	}
	for _, name := range names {
		if writes[name] != before[name]+1 {
			t.Fatalf("cleanup made %d attempts for %s, want one", writes[name]-before[name], name)
		}
	}
	if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 62101}); retained {
		t.Fatal("one owner's failure prevented a different original owner from recovering")
	}
	if !nativeCoSEffectiveMapsEqualForUnit(installed[names[1]].EgressMap, f.original.EgressMap) {
		t.Fatal("the independently recovered owner did not restore its original egress maps")
	}
	for _, session := range []uint16{62100, 62102} {
		if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}); !retained {
			t.Fatal("partial progress forgot a still unconfirmed original owner")
		}
	}
	clear(failures)
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatalf("healthy original retry failed: %v", err)
	}
	for _, name := range names {
		if !nativeCoSEffectiveMapsEqualForUnit(installed[name].IngressMap, f.original.IngressMap) {
			t.Fatalf("recovery on %s did not restore its original ingress maps", name)
		}
	}
	if writes[names[1]] != before[names[1]]+1 {
		t.Fatal("retry replayed an already confirmed original restoration")
	}
}

// Cleanup is a closed operator action. Extra arguments must refuse before a
// healthy or failed owner is touched, so a malformed command cannot do work.
func TestCoSLegacySharedCleanupArgumentsRefuseBeforeNativeWrite(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	registeredCoSCleanupReviewUp(t, bus, 62110)
	f.failWrites = 1
	registeredCoSCleanupReviewDown(t, bus, 62110)
	writes := f.writes
	err := executeCoSCleanupForReview(t, engine, []string{"unexpected"})
	if err == nil {
		t.Fatal("cleanup accepted an undeclared argument")
	}
	if f.writes != writes {
		t.Fatal("cleanup changed native maps before refusing its extra argument")
	}
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatalf("the valid command could not recover the original owner: %v", err)
	}
	f.requireMaps(f.original)
}
