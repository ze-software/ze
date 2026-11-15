package flowtriq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// TestClientRefusesRedirect drives a real POST at a server that answers with a
// redirect to a second server, for each redirect status a client could follow.
//
// VALIDATES: the client follows no redirect. The second server receives no
// request, so neither the bearer token nor the incident body reaches it; the
// call fails with an operator error naming the status, the redacted Location
// and the api-base to configure; and the failure is counted on the circuit
// breaker, so a redirecting endpoint trips it like any other failure.
// PREVENTS: a return to Go's default redirect policy, which re-sends the POST
// body on a 307 or 308 to any host and keeps the Authorization header on a
// same-host https to http redirect, and to the "status below 400 is success"
// check that read an unfollowed 3xx as a delivered incident.
func TestClientRefusesRedirect(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"302 found", http.StatusFound},
		{"307 temporary redirect", http.StatusTemporaryRedirect},
		{"308 permanent redirect", http.StatusPermanentRedirect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var targetHits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetHits.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer target.Close()
			targetHost := strings.TrimPrefix(target.URL, "http://")

			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// A password in the Location proves the error redacts it.
				w.Header().Set("Location", "http://admin:s3cret@"+targetHost+r.URL.Path)
				w.WriteHeader(tc.status)
			}))
			defer origin.Close()

			c := newClient(origin.URL+"/api/v1", "test-key", "test-uuid")
			err := c.heartbeat(true, 1000, 3000)
			if err == nil {
				t.Fatalf("heartbeat answered with %d succeeded; a redirect must fail", tc.status)
			}
			if hits := targetHits.Load(); hits != 0 {
				t.Errorf("the redirect target received %d request(s); it must receive none", hits)
			}
			message := err.Error()
			if !strings.Contains(message, strconv.Itoa(tc.status)) {
				t.Errorf("error does not name status %d:\n%s", tc.status, message)
			}
			if !strings.Contains(message, targetHost+"/api/v1/agent/heartbeat") {
				t.Errorf("error does not name the Location it was redirected to:\n%s", message)
			}
			if strings.Contains(message, "s3cret") {
				t.Errorf("error carries the Location's password unredacted:\n%s", message)
			}
			if !strings.Contains(message, "fix: set ddos flowtriq api-base ") {
				t.Errorf("error does not end with the api-base command that fixes it:\n%s", message)
			}
			c.cb.mu.Lock()
			failures := c.cb.failures
			c.cb.mu.Unlock()
			if failures != 1 {
				t.Errorf("circuit breaker counted %d failure(s), want 1", failures)
			}
		})
	}
}

// TestClientRedirectFixNamesTheFinalAPIBase proves the fix line of a redirect
// refusal is a command the operator can type: when an https Location keeps the
// request path, the api-base it names is that Location with the path removed.
//
// VALIDATES: the fix derives the final api-base from the Location.
// PREVENTS: a fix that names the full endpoint URL, which Ze would append the
// path to a second time.
func TestClientRedirectFixNamesTheFinalAPIBase(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://api.flowtriq.test/v2/agent/heartbeat")
		w.WriteHeader(http.StatusPermanentRedirect)
	}))
	defer origin.Close()

	c := newClient(origin.URL+"/api/v1", "test-key", "test-uuid")
	err := c.heartbeat(true, 1000, 3000)
	if err == nil {
		t.Fatal("heartbeat answered with 308 succeeded; a redirect must fail")
	}
	want := "fix: set ddos flowtriq api-base https://api.flowtriq.test/v2"
	if !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error does not end with %q:\n%s", want, err.Error())
	}
}

// TestClientRedirectWithoutLocationFails proves a 3xx that names nowhere is
// still a failure: only a 2xx is a delivered request.
//
// VALIDATES: a 3xx with no Location fails and is counted on the breaker.
// PREVENTS: a status check that reads any answer below 400 as success.
func TestClientRedirectWithoutLocationFails(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer origin.Close()

	c := newClient(origin.URL+"/api/v1", "test-key", "test-uuid")
	err := c.heartbeat(true, 1000, 3000)
	if err == nil {
		t.Fatal("heartbeat answered with 304 succeeded; only a 2xx is success")
	}
	if !strings.Contains(err.Error(), "status 304") {
		t.Errorf("error does not name status 304: %v", err)
	}
	c.cb.mu.Lock()
	failures := c.cb.failures
	c.cb.mu.Unlock()
	if failures != 1 {
		t.Errorf("circuit breaker counted %d failure(s), want 1", failures)
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
