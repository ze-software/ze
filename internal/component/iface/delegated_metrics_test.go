// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_lease.go -- delegations.record and publishHeld under test

package iface

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// TestDelegatedPrefixesHeldMetric checks that ze_iface_delegated_prefixes_held
// follows the delegations a pppoe-client's lease holds: 1 once a prefix is
// bound, 2 with a second, back to 1 and then 0 as the client publishes each
// expiry (a lease time of zero). Method: feed the delegation table the
// records the delegation worker writes and scrape the exporter after each.
// Break: delete the deferred publishHeld in delegations.record; the gauge
// series never appears and the first scrape goes red. Moving the defer below
// the expiry branch leaves the gauge at 2 after the first expiry.
func TestDelegatedPrefixesHeldMetric(t *testing.T) {
	previous := ifaceMetricsPtr.Load()
	t.Cleanup(func() { ifaceMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindMetricsRegistry(reg)

	table := &delegations{
		held:  make(map[string]map[netip.Prefix]heldDelegation),
		inUse: make(map[string]map[netip.Prefix]LifetimeDeadline),
	}
	first := netip.MustParsePrefix("2001:db8:100::/56")
	second := netip.MustParsePrefix("2001:db8:200::/56")
	received := time.Now()

	steps := []struct {
		prefix netip.Prefix
		valid  int
		want   string
	}{
		{first, 7200, "1"},
		{second, 7200, "2"},
		{first, 0, "1"},
		{second, 0, "0"},
	}
	for _, step := range steps {
		table.record("pppoe0", step.prefix, received, step.valid, step.valid/2)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
		reg.Handler().ServeHTTP(recorder, request)
		line := `ze_iface_delegated_prefixes_held{name="pppoe0"} ` + step.want + "\n"
		if !strings.Contains(recorder.Body.String(), line) {
			t.Fatalf("after recording %s valid %d: exporter missing %q: %s",
				step.prefix, step.valid, line, recorder.Body.String())
		}
	}
}
