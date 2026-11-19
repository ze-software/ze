// Design: docs/architecture/api/birdwatcher-compat.md -- Section 9, producer-shaped fixtures
// Related: server_test.go -- mockDispatch, the handler golden's command answers
// Related: summary_producer_test.go -- the registered overview producer's wire contract

package lg

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestMockSummaryRouteCountsAreProducerValues rejects quoted counts and a
// pre-policy/filtered split that mergeRibRouteCounts cannot produce.
func TestMockSummaryRouteCountsAreProducerValues(t *testing.T) {
	summary := mockSummaryAnswer(t)
	peers := summaryPeers(summary)
	if len(peers) != 1 {
		t.Fatalf("fixture peers = %d, want 1", len(peers))
	}
	peer, ok := peers[0].(map[string]any)
	if !ok {
		t.Fatalf("fixture peer is %T, want object", peers[0])
	}
	for _, field := range []string{"routes-received", "routes-accepted", "routes-sent"} {
		if _, ok := peer[field].(float64); !ok {
			t.Errorf("fixture %s is %T, want a JSON number", field, peer[field])
		}
	}
	if peer["routes-received"] != peer["routes-accepted"] {
		t.Errorf("fixture received = %v, accepted = %v; both must be Adj-RIB-In size", peer["routes-received"], peer["routes-accepted"])
	}
	if _, present := peer["routes-filtered"]; present {
		t.Error("fixture carries routes-filtered, which mergeRibRouteCounts never emits")
	}
}

// TestAPIProtocolsMockPreservesProducerNumbers exercises the public handler
// chain used by the goldens and requires useful numbers to survive translation.
func TestAPIProtocolsMockPreservesProducerNumbers(t *testing.T) {
	rec := apiRequest(t, mockDispatch(), "/api/looking-glass/protocols/bgp")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var answer struct {
		Protocols map[string]map[string]any `json:"protocols"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode protocols: %v", err)
	}
	if len(answer.Protocols) != 1 {
		t.Fatalf("protocols = %#v, want exactly peer1", answer.Protocols)
	}
	peer, exists := answer.Protocols["peer1"]
	if !exists {
		t.Fatal("fixture's named peer did not survive the public endpoint")
	}
	for field, want := range map[string]float64{
		"neighbor_as": 65001, "routes_received": 100, "routes_imported": 100,
		"routes_exported": 50, "routes_filtered": 0, "uptime": 3600,
	} {
		got, ok := peer[field].(float64)
		if !ok {
			t.Errorf("%s = %#v, want JSON number %v", field, peer[field], want)
			continue
		}
		if got != want {
			t.Errorf("%s = %#v, want JSON number %v", field, peer[field], want)
		}
	}
	if peer["neighbor_address"] != "10.0.0.1" {
		t.Errorf("neighbor_address = %#v, want 10.0.0.1", peer["neighbor_address"])
	}
	if peer["routes_counts_available"] != true {
		t.Errorf("routes_counts_available = %#v, want true", peer["routes_counts_available"])
	}
	routes, ok := peer["routes"].(map[string]any)
	if !ok {
		t.Fatalf("routes = %#v, want object", peer["routes"])
	}
	for field, want := range map[string]float64{"imported": 100, "filtered": 0, "exported": 50, "preferred": 100} {
		got, ok := routes[field].(float64)
		if !ok {
			t.Errorf("routes.%s = %#v, want JSON number %v", field, routes[field], want)
			continue
		}
		if got != want {
			t.Errorf("routes.%s = %#v, want JSON number %v", field, routes[field], want)
		}
	}
}

// mockSummaryAnswer decodes the same response bytes LGServer.query consumes,
// so RawJSON and typed answers are checked at their shared wire boundary.
func mockSummaryAnswer(t *testing.T) map[string]any {
	t.Helper()
	response, err := mockDispatch()(t.Context(), plugin.CallerIdentity{}, cmdBGPOverview)
	if err != nil {
		t.Fatalf("mock summary: %v", err)
	}
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("encode mock summary: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatalf("decode mock summary: %v", err)
	}
	return summary
}
