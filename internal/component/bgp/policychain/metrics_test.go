// Design: docs/plugin-development/metrics.md -- ze_policy_route_set_refusals_total
package policychain

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// scrape answers the Prometheus text reg exposes.
func scrape(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", nil))
	return rec.Body.String()
}

// TestRefusedSetCountedByFilter proves AC-6 and AC-20's "counted": a route
// failed by a refused set moves ze_policy_route_set_refusals_total for the
// filter that answered it, whichever on-error decides the route, and a member
// failure that is not a refused set (the plugin unreachable) does not.
// Method: a Prometheus registry wired into the package, three external
// members, each run once: one answering an undeclared set (fail-closed), one
// answering a wrong-shaped value (fail-open), one whose call errors; the
// scraped series name each refusing filter once and the failing one never.
// Not parallel: the registry is package state.
func TestRefusedSetCountedByFilter(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { chainMetricsPtr.Store(nil) })

	caller := &fakeCaller{filters: map[string]fakeFilter{
		"ext:undeclared": {
			decl:   ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject},
			answer: modify(set("med", rpc.FilterSetActionSet, "5")),
		},
		"ext:shape": {
			decl:   ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorAccept},
			answer: modify(set("local-preference", rpc.FilterSetActionSet, `"200"`)),
		},
		"ext:down": {
			decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return nil, errors.New("plugin ext is unavailable")
			},
		},
	}}
	for _, name := range []string{"undeclared", "shape", "down"} {
		chain := compileMembers(t, testAttachment, []Member{{Name: name, Plugin: "ext"}})
		runRoute(t, chain, caller, "import", localPref(100))
		if chain.Refused() != 1 {
			t.Fatalf("%s: refused = %d, want the member failure counted once", name, chain.Refused())
		}
	}

	text := scrape(t, reg)
	for _, want := range []string{
		`ze_policy_route_set_refusals_total{filter="ext:undeclared"} 1`,
		`ze_policy_route_set_refusals_total{filter="ext:shape"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("scrape lacks %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `filter="ext:down"`) {
		t.Errorf("a call failure was counted as a refused set:\n%s", text)
	}
}
