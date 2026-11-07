package l2tp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/core/metrics"
)

// scrapeRelayTotal renders reg through the handler an operator's exporter
// serves and returns the value text of ze_l2tp_relay_total for kind and
// outcome, or the empty string when the series does not exist.
func scrapeRelayTotal(t *testing.T, reg *metrics.PrometheusRegistry, kind, outcome string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	prefix := `ze_l2tp_relay_total{kind="` + kind + `",outcome="` + outcome + `"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return value
		}
	}
	return ""
}

// TestRelayTotalCountsRealmDecisions -- the ze_l2tp_relay_total counter of
// spec-home-cpe-14.
//
// VALIDATES: a realm no binding names counts kind="realm",outcome="no-match";
// a matched realm whose call cannot start counts outcome="refused"; the
// bridge result counts "bridged" or "bridge-failed"; a Service-Name with no
// binding counts kind="service",outcome="no-match". Read from the rendered
// exposition an operator scrapes.
// PREVENTS: a relay decision an operator cannot see.
func TestRelayTotalCountsRealmDecisions(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	s := &Subsystem{
		params: Parameters{
			Remotes:     []Remote{{Name: "lns-a"}},
			RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}},
		},
		logger: slog.Default(),
	}
	sink := &relaySink{s: s}
	auth := callsink.ProxyAuth{Kind: callsink.AuthKindCHAP}

	_, accepted, err := sink.RelayRealm(callsink.RealmRequest{Realm: "other.example", Handoff: &fakeRealmHandoff{}, Auth: auth})
	if err != nil || accepted {
		t.Fatalf("unbound realm: accepted=%t err=%v, want neither", accepted, err)
	}
	if got := scrapeRelayTotal(t, reg, "realm", "no-match"); got != "1" {
		t.Fatalf(`realm no-match = %q, want "1"`, got)
	}

	// No listener: the matched realm's call cannot start.
	if _, _, err := sink.RelayRealm(callsink.RealmRequest{Realm: "example.net", Handoff: &fakeRealmHandoff{}, Auth: auth}); !errors.Is(err, errRelayNoListener) {
		t.Fatalf("matched realm with no listener: err=%v, want errRelayNoListener", err)
	}
	if got := scrapeRelayTotal(t, reg, "realm", "refused"); got != "1" {
		t.Fatalf(`realm refused = %q, want "1"`, got)
	}

	countRealmBridge(nil)
	countRealmBridge(errors.New("PPPIOCBRIDGECHAN: invalid argument"))
	if got := scrapeRelayTotal(t, reg, "realm", "bridged"); got != "1" {
		t.Fatalf(`realm bridged = %q, want "1"`, got)
	}
	if got := scrapeRelayTotal(t, reg, "realm", "bridge-failed"); got != "1" {
		t.Fatalf(`realm bridge-failed = %q, want "1"`, got)
	}

	if accepted, err := sink.Relay(callsink.Request{Service: "unbound"}); err != nil || accepted {
		t.Fatalf("unbound service: accepted=%t err=%v, want neither", accepted, err)
	}
	if got := scrapeRelayTotal(t, reg, "service", "no-match"); got != "1" {
		t.Fatalf(`service no-match = %q, want "1"`, got)
	}
}
