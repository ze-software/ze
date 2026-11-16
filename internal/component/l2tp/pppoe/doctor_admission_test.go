// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session admission failures
// Related: server.go -- provider selection and failed session construction

package pppoe

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/report"
)

// A real PADR that cannot acquire its provider must reach the operator's
// retained events and runtime doctor, while an irrelevant request must not.
func TestPPPoEProviderAdmissionReachesDoctor(t *testing.T) {
	const factoryRefused = "factory refused"
	for _, outcome := range []string{"provider absent", factoryRefused, "irrelevant request"} {
		t.Run(outcome, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			local := [EthALen]byte{2, 0, 0, 0, 0, 1}
			peer := [EthALen]byte{2, 0, 0, 0, 0, 2}
			cause := errors.Join(errors.New("native session creation refused"), errors.New("rollback receipt retained"))
			server := &InterfaceServer{
				ifName: "access70001", device: "control70001", ifIndex: 70001,
				hwAddr: local, cookieTimeout: time.Minute,
				sessions: newSessionTable("access70001", 10), maxSessionsPerMAC: 10,
				serviceNames: []string{"internet"}, logger: slog.Default(),
			}
			factoryCalls := 0
			if outcome != "provider absent" {
				server.openSession = func(context.Context, dataplane.Request) (dataplane.OpenedSession, error) {
					factoryCalls++
					return nil, cause
				}
			}
			sent := 0
			server.sendFrameFn = func([]byte) { sent++ }
			cookie := GenerateCookie(server.cookieKey, local[:], peer[:], nil)
			if outcome == "irrelevant request" {
				cookie = nil
			}
			server.handlePADR(&Packet{Code: CodePADR, SrcMAC: peer, DstMAC: local, Tags: []Tag{
				{Type: TagServiceName, Value: []byte("internet")},
				{Type: TagACCookie, Value: cookie},
			}})
			wantCalls := 0
			if outcome == factoryRefused {
				wantCalls = 1
			}
			if factoryCalls != wantCalls {
				t.Fatalf("factory calls = %d, want %d", factoryCalls, wantCalls)
			}
			if sent != 0 {
				t.Fatalf("unsuccessful admission sent %d discovery replies", sent)
			}
			if len(server.sessions.Sessions()) != 0 {
				t.Fatal("unsuccessful admission retained a session")
			}
			issues := report.Errors(0)
			if outcome == "irrelevant request" {
				if len(issues) != 0 {
					t.Fatalf("irrelevant request invented an admission failure: %+v", issues)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("provider refusal events = %+v, want one", issues)
			}
			issue := issues[0]
			if issue.Code != "doctor-ppp-session-admission" {
				t.Fatalf("admission code = %q", issue.Code)
			}
			if issue.Detail["access-index"] != 70001 {
				t.Fatalf("full access identity lost: %+v", issue.Detail)
			}
			if issue.Detail["session-id"] != uint16(1) {
				t.Fatalf("session identity lost: %+v", issue.Detail)
			}
			if issue.Detail["control-device"] != server.device {
				t.Fatalf("control device lost: %+v", issue.Detail)
			}
			if issue.Detail["access-interface"] != server.ifName {
				t.Fatalf("access interface lost: %+v", issue.Detail)
			}
			if outcome == factoryRefused {
				if issue.Detail["error"] != cause.Error() {
					t.Fatalf("complete factory/rollback cause lost: %+v", issue.Detail)
				}
			}
			for _, finding := range doctor.RunRuntimeChecks(nil, nil) {
				if finding.Code == issue.Code {
					if finding.Fix != "ze show errors" {
						t.Fatalf("doctor lost next command: %+v", finding)
					}
					if !strings.Contains(finding.Message, "70001") {
						t.Fatalf("doctor lost access identity: %+v", finding)
					}
					return
				}
			}
			t.Fatal("runtime doctor omitted the actual provider refusal")
		})
	}
}
