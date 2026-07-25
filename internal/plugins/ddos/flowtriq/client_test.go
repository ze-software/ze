package flowtriq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestClientOpenIncident(t *testing.T) {
	// VALIDATES: incident open sends correct payload to Flowtriq API
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/incidents" {
			t.Errorf("path: got %q, want /api/v1/agent/incidents", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth: got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Node-UUID") != "test-uuid" {
			t.Errorf("uuid: got %q", r.Header.Get("X-Node-UUID"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "inc-123"})
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/v1", "test-key", "test-uuid")
	event := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:    ddosevent.FamilyUDPFlood,
		PeakRxPps: 50000,
		PeakRxBps: 3200000,
		TopSources: []netip.Addr{
			netip.MustParseAddr("1.2.3.4"),
			netip.MustParseAddr("5.6.7.8"),
		},
	}
	uuid, err := c.openIncident(event)
	if err != nil {
		t.Fatalf("OpenIncident: %v", err)
	}
	if uuid != "inc-123" {
		t.Errorf("uuid: got %q, want inc-123", uuid)
	}
	if pps, ok := received["peak_pps"].(float64); !ok || pps != 50000 {
		t.Errorf("peak_pps: got %v", received["peak_pps"])
	}
	if family, ok := received["attack_family"].(string); !ok || family != "udp-flood" {
		t.Errorf("attack_family: got %v", received["attack_family"])
	}
}

func TestClientResolveIncident(t *testing.T) {
	// VALIDATES: incident resolve sends duration and peaks
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/incidents/inc-123/resolve" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/v1", "test-key", "test-uuid")
	err := c.resolveIncident("inc-123", 120.5, 75000, 4800000, 88)
	if err != nil {
		t.Fatalf("ResolveIncident: %v", err)
	}
	if dur, ok := received["duration_seconds"].(float64); !ok || dur != 120.5 {
		t.Errorf("duration_seconds: got %v", received["duration_seconds"])
	}
	if conf, ok := received["confidence"].(float64); !ok || conf != 88 {
		t.Errorf("confidence: got %v, want 88", received["confidence"])
	}
}

func TestClientHeartbeat(t *testing.T) {
	// VALIDATES: heartbeat sends baseline state
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/heartbeat" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/v1", "test-key", "test-uuid")
	err := c.heartbeat(true, 1000, 3000)
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if ready, ok := received["baseline_ready"].(bool); !ok || !ready {
		t.Errorf("baseline_ready: got %v", received["baseline_ready"])
	}
}

func TestCircuitBreakerTripsAfterFailures(t *testing.T) {
	// VALIDATES: circuit breaker trips after 5 consecutive failures
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/v1", "test-key", "test-uuid")
	for range 6 {
		_ = c.heartbeat(false, 0, 0)
	}
	if !c.cb.tripped() {
		t.Error("circuit breaker should be tripped after 5 failures")
	}
}
