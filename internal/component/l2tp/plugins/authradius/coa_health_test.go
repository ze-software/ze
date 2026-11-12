package l2tpauthradius

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCoABackendFailureRemainsVisibleUntilRecovery asks the running RADIUS
// provider's real doctor callback after a partial backend failure, then retries
// the authenticated UDP request and requires the same problem to clear.
func TestCoABackendFailureRemainsVisibleUntilRecovery(t *testing.T) {
	ctx, provider := startAuthProvider(t, Name, "")
	addr, secret, backend := coaAppliedFixture(t)
	t.Cleanup(func() { report.ClearSource(Name) })
	backend.mu.Lock()
	backend.fail = true
	backend.mu.Unlock()
	attrs := []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	}
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, attrs)
	wantNAK(t, response, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	read := func() []rpc.DoctorCheckDiagnostic {
		t.Helper()
		params, err := json.Marshal(rpc.DoctorCheckInput{Name: "l2tp-radius-coa-apply"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := provider.SendCallback(ctx, "ze-plugin-callback:doctor-check", params)
		if err != nil {
			t.Fatal(err)
		}
		var result rpc.DoctorCheckOutput
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result.Diagnostics
	}
	findings := read()
	if len(findings) != 1 {
		t.Fatalf("failed backend runtime findings: got %+v, want one", findings)
	}
	if findings[0].Code != "doctor-radius-coa-apply" || findings[0].Fix == "" {
		t.Fatalf("backend failure lacks diagnostic code or recovery command: %+v", findings[0])
	}
	backend.mu.Lock()
	backend.fail = false
	backend.mu.Unlock()
	response = sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, attrs)
	if response.Code != radius.CodeCoAACK {
		t.Fatalf("backend recovery response: got %d, want CoA-ACK", response.Code)
	}
	if findings := read(); len(findings) != 0 {
		t.Errorf("successful retry retained the same backend failure: %+v", findings)
	}
}
