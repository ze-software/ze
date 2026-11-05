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
	for _, tc := range []struct {
		name  string
		pppoe bool
		path  string
		other string
	}{
		{"L2TP", false, "set l2tp authentication allow", "set pppoe authentication-allow"},
		{"PPPoE", true, "set pppoe authentication-allow", "set l2tp authentication allow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := pppMetricsPtr.Swap(nil)
			t.Cleanup(func() { pppMetricsPtr.Store(previous) })
			reg := metrics.NewPrometheusRegistry()
			bindPPPMetrics(reg)
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			s.pppoe = tc.pppoe
			t.Cleanup(func() {
				if s.restartTimer != nil {
					s.restartTimer.Stop()
				}
			})
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
			wantFix := "`" + tc.path + " chap-md5`, then `" + tc.path + " pap`"
			if !strings.Contains(s.authRefusalReason, wantFix) || strings.Count(s.authRefusalReason, "`") != 4 {
				t.Fatalf("refusal lacks exactly the executable per-member operator fixes %q: %s", wantFix, s.authRefusalReason)
			}
			if strings.Contains(s.authRefusalReason, tc.other) {
				t.Fatalf("refusal offers a repair for another transport: %s", s.authRefusalReason)
			}
		})
	}
}
