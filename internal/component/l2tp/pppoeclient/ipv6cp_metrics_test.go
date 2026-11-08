// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP visibility.
// Related: metrics.go -- ze_pppoe_client_ipv6cp_opened; ipv6cp_client.go -- applyIPv6CP.

package pppoeclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ipv6cpMetricsRegistry binds a fresh exporter registry for one test and
// restores the previous binding after it.
func ipv6cpMetricsRegistry(t *testing.T) *metrics.PrometheusRegistry {
	t.Helper()
	previous := clientMetricsPtr.Load()
	t.Cleanup(func() { clientMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindClientMetrics(reg)
	return reg
}

// scrapeIPv6CPOpened fails the test unless the exporter shows want for name.
func scrapeIPv6CPOpened(t *testing.T, reg *metrics.PrometheusRegistry, name, want string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	line := `ze_pppoe_client_ipv6cp_opened{name="` + name + `"} ` + want + "\n"
	if !strings.Contains(recorder.Body.String(), line) {
		t.Fatalf("exporter missing %q: %s", line, recorder.Body.String())
	}
}

// TestIPv6CPOpenedMetric checks that the IPv6CP gauge follows the session's
// IPv6CP state: 1 once IPv6CP opened, 0 once the peer Protocol-Rejects it,
// 1 again when it opens. Method: drive the real automaton to Opened, apply
// each outcome through the session glue (applyIPv6CP), and scrape the
// exporter after each step.
// Break: delete either recordIPv6CPOpened call in applyIPv6CP; the scrape
// after that step reads the previous value and the test goes red.
func TestIPv6CPOpenedMetric(t *testing.T) {
	reg := ipv6cpMetricsRegistry(t)
	logger := slogutil.DiscardLogger()
	c, _, _ := openedIPv6CP(t)
	var v4, v6 []ppp.NPMode
	s := independenceSession(t, c, &v4, &v6)
	s.cfg.name = "wan-v6"

	if err := s.applyIPv6CP(ipv6cpOpened, logger); err != nil {
		t.Fatalf("apply opened: %v", err)
	}
	<-s.changes
	scrapeIPv6CPOpened(t, reg, "wan-v6", "1")

	if err := s.applyIPv6CP(c.protocolRejected(), logger); err != nil {
		t.Fatalf("apply rejected: %v", err)
	}
	<-s.changes
	scrapeIPv6CPOpened(t, reg, "wan-v6", "0")
}

// TestIPv6CPOpenedMetricSessionEnd checks that the gauge reads 0 once the
// session's worker ends, whatever IPv6CP state it ended in. Method: raise the
// gauge as an Opened IPv6CP does, run the real keepaliveLoop on a session that
// configures ipv6, stop it, and scrape.
// Break: delete the deferred recordIPv6CPOpened in keepaliveLoop; the scrape
// reads 1 and the test goes red.
func TestIPv6CPOpenedMetricSessionEnd(t *testing.T) {
	reg := ipv6cpMetricsRegistry(t)
	synctest.Test(t, func(t *testing.T) {
		var v6 []ppp.NPMode
		_, s, done := ipcpIndependence(t, true, &v6)
		s.cfg.name = "wan-end"
		recordIPv6CPOpened("wan-end", true)
		stop := make(chan struct{})
		go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
		synctest.Wait()
		close(stop)
		<-done
	})
	scrapeIPv6CPOpened(t, reg, "wan-end", "0")
}
