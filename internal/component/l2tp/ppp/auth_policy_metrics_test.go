package ppp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// TestAuthMethodRefusalCounter drives the real LCP refusal and scrapes the
// operator-facing exporter, including a stale retransmitted Nak control.
func TestAuthMethodRefusalCounter(t *testing.T) {
	previous := pppMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindPPPMetrics(reg)
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodCHAPMD5
	s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5}
	s.authRequired = true
	if !s.sendConfigureRequest() {
		t.Fatal("send request failed")
	}
	request := lastLCPConfigureRequest(t, rec)
	nak := LCPPacket{Code: LCPConfigureNak, Identifier: request.Identifier, Data: optStream(authProtoOpt(authProtoPAP))}
	s.handleLCPPacket(nak)
	s.handleLCPPacket(nak)
	recorder := httptest.NewRecorder()
	requestHTTP := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, requestHTTP)
	want := `ze_ppp_auth_failures_total{reason="auth-method-refused"} 1`
	if !strings.Contains(recorder.Body.String(), want+"\n") {
		t.Fatalf("export lacks %q: %s", want, recorder.Body.String())
	}
	if !strings.Contains(s.authRefusalReason, "(pap); allowed [chap-md5]") {
		t.Fatalf("refusal does not identify peer and policy: %s", s.authRefusalReason)
	}
	if !strings.Contains(s.authRefusalReason, "`set l2tp authentication allow [ chap-md5 pap ]`") {
		t.Fatalf("refusal lacks exact L2TP operator fix: %s", s.authRefusalReason)
	}
	if !strings.Contains(s.authRefusalReason, "`set pppoe authentication-allow [ chap-md5 pap ]`") {
		t.Fatalf("refusal lacks exact PPPoE operator fix: %s", s.authRefusalReason)
	}
}
