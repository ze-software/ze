// Design: docs/architecture/core-design.md -- FIB kernel plugin
// Related: fibkernel.go -- processEvent (withdraw refusal), flushRoutes (installed gauge)

package fibkernel

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

// withOutcomeMetrics registers a fresh metrics registry for one test and
// restores the previous fibMetricsPtr afterwards, the seam
// TestFIBKernelReplayRefusalPreservesSuccessfulForwarding uses.
func withOutcomeMetrics(t *testing.T) *metrics.PrometheusRegistry {
	t.Helper()
	previous := fibMetricsPtr.Load()
	t.Cleanup(func() { fibMetricsPtr.Store(previous) })
	registry := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(registry)
	return registry
}

// scrapeOutcomeMetrics answers the registry's Prometheus text, the body an
// operator's scraper reads.
func scrapeOutcomeMetrics(t *testing.T, registry *metrics.PrometheusRegistry) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody))
	return recorder.Body.String()
}

// Goal: a withdraw the kernel refuses is reported and keeps its ownership.
// Method: a backend whose delete fails receives an install and then the
// withdraw of the same prefix, through processEvent.
// Expected: one fib-sync-failure error for the prefix with operation delete,
// the delete-failure metric at 1, and the prefix still owned, because the
// route is still in the kernel. TestFIBSyncFailure and
// TestFIBSyncFailureOnReplace cover the add and replace arms; nothing drove
// the remove arm before.
func TestFIBSyncFailureOnDelete(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	registry := withOutcomeMetrics(t)

	backend := &failingBackend{mockBackend: *newMockBackend(), failDel: true}
	f := newFIBKernel(backend)
	prefix := netip.MustParsePrefix("10.0.7.0/24")

	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("192.168.7.1"), Protocol: "bgp"},
	}))
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Withdraw, Prefix: prefix},
	}))

	raised := 0
	for _, e := range report.Errors(0) {
		if e.Code != reportCodeFIBSyncFailure {
			continue
		}
		if e.Subject != prefix.String() {
			t.Errorf("fib-sync-failure names %q, want %q", e.Subject, prefix)
			continue
		}
		raised++
		if got := e.Detail[keyOperation]; got != "delete" {
			t.Errorf("refused withdraw reported operation %v, want delete", got)
		}
		if !strings.Contains(e.Message, "no such process") {
			t.Errorf("refused withdraw message %q does not carry the kernel's error", e.Message)
		}
	}
	if raised != 1 {
		t.Fatalf("refused withdraw raised %d fib-sync-failure errors, want 1", raised)
	}
	if got, owned := f.installed[prefix.String()]; !owned || got != "192.168.7.1" {
		t.Errorf("refused withdraw dropped ownership of a route still in the kernel: installed=%q owned=%v", got, owned)
	}
	body := scrapeOutcomeMetrics(t, registry)
	if !strings.Contains(body, "ze_fibkernel_errors_total{operation=\"delete\"} 1\n") {
		t.Errorf("refused withdraw did not count one delete failure:\n%s", body)
	}
	if !strings.Contains(body, "ze_fibkernel_routes_installed 1\n") {
		t.Errorf("refused withdraw changed the installed gauge:\n%s", body)
	}
}

// Goal: the flush leaves the installed-routes gauge at zero.
// Method: install two routes through processEvent, check the gauge reads 2,
// then flush.
// Expected: the gauge reads 0 after the flush, the state the kernel is left
// in, rather than the count from before it.
func TestFIBKernelFlushZeroesTheInstalledGauge(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	registry := withOutcomeMetrics(t)

	backend := newMockBackend()
	f := newFIBKernel(backend)
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.8.0/24"), NextHop: netip.MustParseAddr("192.168.8.1"), Protocol: "bgp"},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.9.0/24"), NextHop: netip.MustParseAddr("192.168.9.1"), Protocol: "bgp"},
	}))
	if body := scrapeOutcomeMetrics(t, registry); !strings.Contains(body, "ze_fibkernel_routes_installed 2\n") {
		t.Fatalf("control: two installs did not set the gauge to 2:\n%s", body)
	}

	f.flushRoutes()

	if len(backend.deleted) != 2 {
		t.Fatalf("flush deleted %v, want both installed prefixes", backend.deleted)
	}
	if body := scrapeOutcomeMetrics(t, registry); !strings.Contains(body, "ze_fibkernel_routes_installed 0\n") {
		t.Errorf("flush left the installed gauge at its old value:\n%s", body)
	}
}

// Goal: an update that installs a route which was not installed before (its
// first add was refused by the kernel) counts it in the installed gauge at
// once. Method: a backend that refuses the add, one Add, the backend then
// accepting, one Update for the same prefix, then read the gauge: 1.
func TestFIBKernelUpdateAfterRefusedAddCountsTheRoute(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	registry := withOutcomeMetrics(t)

	backend := &failingBackend{mockBackend: *newMockBackend(), failAdd: true}
	f := newFIBKernel(backend)
	prefix := netip.MustParsePrefix("10.0.12.0/24")
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("192.168.12.1"), Protocol: "bgp"},
	}))
	if body := scrapeOutcomeMetrics(t, registry); !strings.Contains(body, "ze_fibkernel_routes_installed 0\n") {
		t.Fatalf("control: a refused add left the gauge off zero:\n%s", body)
	}

	backend.failAdd = false
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Update, Prefix: prefix, NextHop: netip.MustParseAddr("192.168.12.2"), Protocol: "bgp"},
	}))

	if _, installed := f.installed[prefix.String()]; !installed {
		t.Fatalf("the update after a refused add did not install %s", prefix)
	}
	if body := scrapeOutcomeMetrics(t, registry); !strings.Contains(body, "ze_fibkernel_routes_installed 1\n") {
		t.Errorf("the update installed the route but the gauge does not count it:\n%s", body)
	}
}
