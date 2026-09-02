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

// reporterTestServer answers every incident call the reporter makes, so a test
// can drive the whole set of transitions without asserting on the payloads.
// The uuid is constant: what these tests are about is which goroutine may
// touch the state, not what the API is told.
func reporterTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "inc-race"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func reporterTestDetected() *ddosevent.AttackDetected {
	return &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:    ddosevent.FamilyUDPFlood,
		PeakRxPps: 50000,
		PeakRxBps: 3200000,
	}
}

// TestReporterStateIsOrderedAgainstAConfigApply drives the four detector
// callbacks against a configuration apply from separate goroutines, which is
// how they arrive in the daemon: the detector publishes from its rate tick and
// from the characterization goroutine, and the plugin SDK invokes
// OnConfigApply from its own reader loop. Nothing outside the reporter orders
// the two.
//
// VALIDATES: every writer of the incident state takes the reporter's lock, so
// an apply and a delivery cannot interleave on it.
// PREVENTS: the state the deferral row recorded as unguarded going back to
// closure variables in runEngine, where an apply could clear the incident
// between an Ongoing callback's "is there an incident" test and its post, and
// where the client pointer could be swapped inside a handler already using it.
//
// It is a RACE-DETECTOR test: it makes no assertion about the values, because
// the interleaving is what is under test. Under `-race` it fails if any of the
// five writers stops taking the lock.
func TestReporterStateIsOrderedAgainstAConfigApply(t *testing.T) {
	srv := reporterTestServer(t)
	var rep reporter
	rep.swapClient(newClient(srv.URL+"/api/v1", "test-key", "test-uuid"))

	const rounds = 50
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		for range rounds {
			rep.onDetected(reporterTestDetected())
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			rep.onCharacterized(&ddosevent.AttackCharacterized{Confidence: 75})
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			rep.onOngoing(&ddosevent.AttackOngoing{CurrentPps: 60000, CurrentBps: 4000000})
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			rep.onCleared(&ddosevent.AttackCleared{})
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			rep.swapClient(newClient(srv.URL+"/api/v1", "test-key", "test-uuid"))
		}
	}()

	wg.Wait()
}

// TestReporterApplyResolvesTheOpenIncident verifies that a configuration apply
// closes the incident it is about to orphan, and that the client it installs is
// the one the next attack reports through.
//
// VALIDATES: swapClient resolves before it swaps, so an incident is never left
// open on the API by a reload.
// PREVENTS: the reload path dropping the resolve, which leaves the remote side
// showing an attack that ended.
func TestReporterApplyResolvesTheOpenIncident(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "inc-1"})
	}))
	defer srv.Close()

	var rep reporter
	rep.swapClient(newClient(srv.URL+"/api/v1", "k", "n"))
	rep.onDetected(reporterTestDetected())

	// The apply arrives with the incident still open.
	rep.swapClient(newClient(srv.URL+"/api/v1", "k", "n"))

	mu.Lock()
	got := append([]string(nil), paths...)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("calls: got %v, want an open and a resolve", got)
	}
	if got[1] == got[0] {
		t.Errorf("the apply repeated the open instead of resolving: %v", got)
	}

	// The incident is forgotten, so a later Ongoing reports nothing.
	rep.onOngoing(&ddosevent.AttackOngoing{CurrentPps: 1, CurrentBps: 1})
	mu.Lock()
	after := len(paths)
	mu.Unlock()
	if after != 2 {
		t.Errorf("an update was posted for a resolved incident: %d calls", after)
	}
}
