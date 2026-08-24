// VALIDATES: AC-6, AC-7, AC-8 (CoS enricher detail/brief/noop)
// PREVENTS: enricher crash on missing session state

//go:build ze_l2tp

package cos

import (
	"testing"

	coreCos "github.com/ze-software/ze/internal/core/cos"
)

func TestCoSSubscriberEnricherDetail(t *testing.T) {
	sessionStore.Store(sessionKey{tunnelID: 1, sessionID: 2}, sessionCoSState{
		profileName:   "residential",
		staticIngress: map[uint32]uint32{0: 0},
		staticEgress:  map[uint32]uint32{0: 0},
	})
	t.Cleanup(func() { sessionStore.Delete(sessionKey{tunnelID: 1, sessionID: 2}) })

	coreCos.Register("residential", coreCos.Profile{
		IngressMap: map[uint32]uint32{0: 1, 1: 2},
		EgressMap:  map[uint32]uint32{1: 0, 2: 1},
	})
	t.Cleanup(coreCos.Clear)

	base := map[string]any{
		"id":         "s1",
		"tunnel-id":  uint16(1),
		"session-id": uint16(2),
	}

	enrichSubscriberDetail(base)

	cosData, ok := base["cos"].(map[string]any)
	if !ok {
		t.Fatalf("expected cos map, got %T: %v", base["cos"], base["cos"])
	}
	if cosData["profile"] != "residential" {
		t.Fatalf("expected profile=residential, got %v", cosData["profile"])
	}
	if cosData["ingress"] == nil {
		t.Fatal("expected ingress map in cos data")
	}
	if cosData["egress"] == nil {
		t.Fatal("expected egress map in cos data")
	}
}

func TestCoSSubscriberEnricherBrief(t *testing.T) {
	sessionStore.Store(sessionKey{tunnelID: 3, sessionID: 4}, sessionCoSState{
		profileName: "business",
	})
	t.Cleanup(func() { sessionStore.Delete(sessionKey{tunnelID: 3, sessionID: 4}) })

	base := map[string]any{
		"id":         "s2",
		"tunnel-id":  uint16(3),
		"session-id": uint16(4),
	}

	enrichSubscriberBrief(base)

	if base["cos-profile"] != "business" {
		t.Fatalf("expected cos-profile=business, got %v", base["cos-profile"])
	}
}

func TestCoSSubscriberEnricherNoSession(t *testing.T) {
	base := map[string]any{
		"id":         "s3",
		"tunnel-id":  uint16(99),
		"session-id": uint16(99),
	}

	enrichSubscriberDetail(base)

	if _, ok := base["cos"]; ok {
		t.Fatal("expected no cos key when session has no CoS state")
	}
}

func TestCoSSubscriberEnricherNoTunnelID(t *testing.T) {
	base := map[string]any{
		"id": "s4",
	}

	enrichSubscriberDetail(base)

	if _, ok := base["cos"]; ok {
		t.Fatal("expected no cos key when tunnel-id is absent")
	}
}
