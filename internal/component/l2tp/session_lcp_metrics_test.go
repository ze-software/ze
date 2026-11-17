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

	// These are consumer metadata cases, including a deliberate Unknown;
	// they do not claim that PAP/proxy setup produced every edited value.
	first := newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 100, sessionID: 41}, ipv6: false, subscriberID: ""})
	opened := first.opened
	opened.NegotiatedMRU, opened.ProxyLCP = 1492, ppp.ProxyLCPOutcomeRenegotiated
	opened.NegotiatedAuthMethod = ppp.AuthMethodNone
	r.handlePPPEvent(opened)
	r.handlePPPEvent(first.beginRestart(t))
	first.finishRestart(t)
	opened = first.opened
	opened.NegotiatedMRU, opened.ProxyLCP = 1492, ppp.ProxyLCPOutcomeRenegotiated
	opened.NegotiatedAuthMethod = ppp.AuthMethodNone
	r.handlePPPEvent(opened)
	for _, tc := range []struct {
		sid     uint16
		outcome ppp.ProxyLCPOutcome
	}{
		{42, ppp.ProxyLCPOutcomeAccepted}, {43, ppp.ProxyLCPOutcomeNone}, {44, ppp.ProxyLCPOutcomeUnknown},
	} {
		wire := newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 100, sessionID: tc.sid}, ipv6: false, subscriberID: ""})
		opened := wire.opened
		opened.NegotiatedMRU, opened.ProxyLCP = 1492, tc.outcome
		opened.NegotiatedAuthMethod = ppp.AuthMethodNone
		r.handlePPPEvent(opened)
	}
	missing := first.opened
	missing.SessionID, missing.NegotiatedMRU, missing.ProxyLCP = 999, 1492, ppp.ProxyLCPOutcomeAccepted
	missing.NegotiatedAuthMethod = ppp.AuthMethodNone
	r.handlePPPEvent(missing)

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
