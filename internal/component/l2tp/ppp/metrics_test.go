package ppp

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// scrapeIdentifierRefusals renders reg through the same promhttp handler an
// operator's exporter serves, and returns the
// ze_ppp_ipv6cp_identifier_refusals_total line for reason, or the empty
// string when the series does not exist. Reading the rendered text rather
// than the collector proves an operator can see the count.
func scrapeIdentifierRefusals(t *testing.T, reg *metrics.PrometheusRegistry, reason string) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)

	prefix := `ze_ppp_ipv6cp_identifier_refusals_total{reason="` + reason + `"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// TestIdentifierRefusalCounterBindsWhenTheRegistryArrivesLast pins the same
// startup-order gap internal/component/l2tp/pppoe/metrics.go documents and
// closes for its own counter: runYANGConfig (cmd/ze/hub/main.go) runs
// engine.Start, which runs Driver.Start, before startStandaloneTelemetry
// creates the registry on an L2TP- or PPPoE-only daemon. Reading
// registry.GetMetricsRegistry from registerPPPMetrics instead would bind
// nothing and leave ze_ppp_ipv6cp_identifier_refusals_total absent for the
// process lifetime with no line saying so.
//
// This is the only test in the package that touches the process-global
// registry.InjectPluginMetrics bookkeeping (registry.SetMetricsRegistry):
// that bookkeeping is idempotent per plugin name and has no reset, so a
// second test doing the same dance would find registerPPPMetrics already
// configured and silently a no-op. Every other counter test in this
// package binds pppMetricsPtr directly with bindPPPMetrics instead.
func TestIdentifierRefusalCounterBindsWhenTheRegistryArrivesLast(t *testing.T) {
	previousMetrics := pppMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppMetricsPtr.Store(previousMetrics) })

	previousRegistry := registry.GetMetricsRegistry()
	t.Cleanup(func() { registry.SetMetricsRegistry(previousRegistry) })
	registry.SetMetricsRegistry(nil)

	registerPPPMetrics()
	if pppMetricsPtr.Load() != nil {
		t.Fatal("counters bound with no registry in existence")
	}
	// Must not panic, and there is nothing to replay once the registry
	// arrives: this count is simply lost, the same as any other metric
	// observed before the exporter exists.
	countIdentifierRefusal(reasonEventUnnegotiated)

	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)
	if pppMetricsPtr.Load() == nil {
		t.Fatal("the identifier-refusal counter stayed unbound after the telemetry exporter created the registry")
	}

	countIdentifierRefusal(reasonEventUnnegotiated)

	want := `ze_ppp_ipv6cp_identifier_refusals_total{reason="event-unnegotiated"} 1`
	if got := scrapeIdentifierRefusals(t, reg, reasonEventUnnegotiated); got != want {
		t.Errorf("scraped %q, want %q", got, want)
	}
}

// TestIdentifierRefusalCounterCountsAfterLCPOpenAndSessionEventRefusals
// binds pppMetricsPtr directly (bypassing registry.InjectPluginMetrics, see
// the note on TestIdentifierRefusalCounterBindsWhenTheRegistryArrivesLast),
// then drives one real session to IPv6CP Opened with no negotiated peer
// identifier -- the same scenario TestIPv6ServiceRefusesUnnegotiatedIdentifier
// (ipv6_service_test.go) proves is refused -- and reads both series the
// session's two independent refusal sites publish back off the rendered
// page: afterLCPOpenIPv6Service (session_run.go), which refuses to start
// the IPv6 service, and onNCPOpened (ncp.go), which withholds the session-up
// address. Driver.Start (manager.go) also calls registerPPPMetrics as
// production code always does; with no global registry ever set in this
// test, that call is a harmless no-op (ai/rules/testing.md: exercise the
// real call site, not a substitute).
func TestIdentifierRefusalCounterCountsAfterLCPOpenAndSessionEventRefusals(t *testing.T) {
	previous := pppMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindPPPMetrics(reg)

	w := &captureWriter{}
	logger := slog.New(slog.NewTextHandler(w, nil))
	td := newNCPTestDriverIPLogged(t, &StartSession{DisableIPCP: true}, autoAcceptIP, logger)
	defer td.cleanup()

	td.completeIPv6CPMissingOption(t)

	// onNCPOpened runs, and would send EventSessionIPAssigned, strictly
	// before afterLCPOpen sends EventSessionUp -- both write to the same
	// eventsOut channel (ipv6_service_test.go's
	// TestIPv6ServiceRefusesUnnegotiatedIdentifier makes the same
	// argument) -- so waiting for the session-up event is enough to know
	// both refusal sites have already run.
	ev := td.waitForEvent(t, 2*time.Second)
	if _, ok := ev.(EventSessionUp); !ok {
		t.Fatalf("first event after IPv6CP opened without a negotiated identifier = %#v, want EventSessionUp", ev)
	}

	wantService := `ze_ppp_ipv6cp_identifier_refusals_total{reason="service-unnegotiated"} 1`
	if got := scrapeIdentifierRefusals(t, reg, reasonServiceUnnegotiated); got != wantService {
		t.Errorf("afterLCPOpenIPv6Service refusal: scraped %q, want %q", got, wantService)
	}
	wantEvent := `ze_ppp_ipv6cp_identifier_refusals_total{reason="event-unnegotiated"} 1`
	if got := scrapeIdentifierRefusals(t, reg, reasonEventUnnegotiated); got != wantEvent {
		t.Errorf("onNCPOpened refusal: scraped %q, want %q", got, wantEvent)
	}
}
