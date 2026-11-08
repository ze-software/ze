package l2tp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/metrics"
)

// scrapeSessionLCPOpened renders reg through the handler an operator's
// exporter serves and returns the value text of
// ze_l2tp_session_lcp_opened_total for proxyLCP, or the empty string when the
// series does not exist.
func scrapeSessionLCPOpened(t *testing.T, reg *metrics.PrometheusRegistry, proxyLCP string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	prefix := `ze_l2tp_session_lcp_opened_total{proxy_lcp="` + proxyLCP + `"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return value
		}
	}
	return ""
}

// TestSessionLCPOpenedTotalCountsProxyLCPOutcome -- the `proxy_lcp` label of
// spec-home-cpe-15's Integration Checklist.
//
// VALIDATES: the reactor's EventLCPUp consumer counts each session once, on
// its first LCP open, under proxy_lcp="renegotiated", "accepted" or "none"
// as the PPP driver reported; an LCP reopen of the same session (after
// LCPDown) does not count again; an event for a session the reactor does not
// hold, or one with no outcome, counts nothing. Read from the rendered
// exposition an operator scrapes.
// PREVENTS: an operator unable to see whether an LNS renegotiated or
// accepted the LAC's proxy LCP, and a session counted on every renegotiation.
func TestSessionLCPOpenedTotalCountsProxyLCPOutcome(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	r := newReactorForSnapshot(t)
	insertEstablishedTunnel(t, r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"),
		&L2TPSession{localSID: 41, state: L2TPSessionEstablished},
		&L2TPSession{localSID: 42, state: L2TPSessionEstablished},
		&L2TPSession{localSID: 43, state: L2TPSessionEstablished},
		&L2TPSession{localSID: 44, state: L2TPSessionEstablished},
	)

	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 41, NegotiatedMRU: 1492, ProxyLCP: ppp.ProxyLCPOutcomeRenegotiated})
	r.handlePPPEvent(ppp.EventLCPDown{TunnelID: 100, SessionID: 41})
	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 41, NegotiatedMRU: 1492, ProxyLCP: ppp.ProxyLCPOutcomeRenegotiated})
	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 42, NegotiatedMRU: 1492, ProxyLCP: ppp.ProxyLCPOutcomeAccepted})
	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 43, NegotiatedMRU: 1492, ProxyLCP: ppp.ProxyLCPOutcomeNone})
	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 44, NegotiatedMRU: 1492})
	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 999, NegotiatedMRU: 1492, ProxyLCP: ppp.ProxyLCPOutcomeAccepted})

	for _, want := range []struct{ proxyLCP, value string }{
		{"renegotiated", "1"},
		{"accepted", "1"},
		{"none", "1"},
		{"unknown", ""},
	} {
		if got := scrapeSessionLCPOpened(t, reg, want.proxyLCP); got != want.value {
			t.Errorf(`ze_l2tp_session_lcp_opened_total{proxy_lcp=%q} = %q, want %q`, want.proxyLCP, got, want.value)
		}
	}
}
