// Design: docs/architecture/traffic/cos-dynamic.md -- bounded cleanup pass counts
// Related: shared_access_cleanup_registered_test.go -- actual registered SDK route

//go:build ze_l2tp

package cos

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// Goal: a partially failed pass reports both counts through its actual SDK
// error, then success counts only remaining owners, and an empty pass is zero.
// Method: retain three distinct modeled native owners, allow only the middle
// one to restore, and read every response from engine.Execute itself.
// MUTATION: omit either failed-pass count, or count an already recovered owner.
func TestCoSLegacySharedCleanupCountsPartialFinalAndEmptyPasses(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	names := []string{"eth0.327", "eth0.328", "eth0.329"}
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
		return &iface.InterfaceInfo{Name: name, Index: 327, Type: "vlan",
			IngressQoSMap: maps.Clone(profile.IngressMap), EgressQoSMap: maps.Clone(profile.EgressMap)}, nil
	}
	for index, name := range names {
		session := uint16(63200 + index)
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
			t.Fatalf("healthy policy was not installed on %s", name)
		}
		if !nativeCoSEffectiveMapsEqualForUnit(installed[name].EgressMap, f.shared.EgressMap) {
			t.Fatalf("healthy egress policy was not installed on %s", name)
		}
		failures[name] = fmt.Errorf("original native restore %s is unconfirmed", name)
		registeredCoSCleanupReviewDown(t, bus, session)
		if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}); !retained {
			t.Fatalf("failed Down lost original owner %s before the count question", name)
		}
	}
	delete(failures, names[1])
	before := maps.Clone(writes)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := engine.Execute(ctx, cleanupCommandForReview, nil)
	if err == nil {
		t.Fatal("partial cleanup acknowledged two unconfirmed restorations")
	}
	for _, label := range []string{`\battempted=3\b`, `\brecovered=1\b`} {
		if !regexp.MustCompile(label).MatchString(err.Error()) {
			t.Fatalf("actual SDK failure lost count %s: %v", label, err)
		}
	}
	for _, name := range []string{names[0], names[2]} {
		if !strings.Contains(err.Error(), failures[name].Error()) {
			t.Fatalf("counted failure lost original cause for %s: %v", name, err)
		}
	}
	for _, name := range names {
		if writes[name] != before[name]+1 {
			t.Fatalf("partial pass made %d attempts on %s, want one", writes[name]-before[name], name)
		}
	}
	clear(failures)
	ctxFinal, cancelFinal := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelFinal()
	data, err := engine.Execute(ctxFinal, cleanupCommandForReview, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCoSCleanupCountDocumentForReview(t, data, 2, 2)
	if writes[names[1]] != before[names[1]]+1 {
		t.Fatal("final pass replayed the owner already counted as recovered")
	}
	for _, name := range names {
		if !nativeCoSEffectiveMapsEqualForUnit(installed[name].IngressMap, f.original.IngressMap) {
			t.Fatalf("final counted recovery did not restore ingress on %s", name)
		}
		if !nativeCoSEffectiveMapsEqualForUnit(installed[name].EgressMap, f.original.EgressMap) {
			t.Fatalf("final counted recovery did not restore egress on %s", name)
		}
	}
	before = maps.Clone(writes)
	ctxEmpty, cancelEmpty := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelEmpty()
	data, err = engine.Execute(ctxEmpty, cleanupCommandForReview, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCoSCleanupCountDocumentForReview(t, data, 0, 0)
	if !maps.Equal(writes, before) {
		t.Fatal("zero-count pass still wrote native maps")
	}
}

// Zero-valued missing JSON fields must not pass the empty-pass observation.
func requireCoSCleanupCountDocumentForReview(t *testing.T, data json.RawMessage, attempted, recovered int) {
	t.Helper()
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("actual SDK cleanup document is invalid JSON %q: %v", data, err)
	}
	for name, want := range map[string]int{"attempted": attempted, "recovered": recovered} {
		raw, present := document[name]
		if !present {
			t.Fatalf("actual SDK cleanup document omitted %s: %s", name, data)
		}
		var got int
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("actual SDK cleanup count %s=%s is invalid: %v", name, raw, err)
		}
		if string(raw) == "null" {
			t.Fatalf("actual SDK cleanup count %s is null", name)
		}
		if got != want {
			t.Fatalf("actual SDK cleanup count %s=%d, want %d", name, got, want)
		}
	}
}
