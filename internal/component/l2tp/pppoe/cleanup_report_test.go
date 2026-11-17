// Design: docs/guide/monitoring.md -- retained PPP cleanup failures
// Related: server.go -- access cleanup producers

package pppoe

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

type cleanupFailureHandle struct {
	fixtureSessionHandle
	failure error
}

func (handle *cleanupFailureHandle) Close() error { return handle.failure }

type cleanupFailureTransport struct{ failure error }

func (owner *cleanupFailureTransport) Close() error { return owner.failure }

// TestAccessCleanupFailureReachesOperator injects each actual access cleanup
// failure and reads both operator sinks. The created identity differs from the
// access identity, so a fabricated access index cannot satisfy the assertion.
func TestAccessCleanupFailureReachesOperator(t *testing.T) {
	for _, path := range []string{"admitted lease", "unpublished lease", "transport"} {
		t.Run(path, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			identity := ppp.SessionIdentity{InterfaceName: "created_ppp42", NativeIndex: 84}
			failure := errors.New("backend removal failed")
			handle := &cleanupFailureHandle{identity: identity}
			transport := &cleanupFailureTransport{}
			operation := ppp.CleanupSession
			if path == "transport" {
				operation = ppp.CleanupTransport
				transport.failure = failure
			} else {
				handle.failure = failure
			}
			opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, handle, transport)
			if err != nil {
				t.Fatal(err)
			}
			var log bytes.Buffer
			server := &InterfaceServer{ifName: "access99", ifIndex: 99,
				sessions: newSessionTable("access99", 10), logger: slog.New(slog.NewJSONHandler(&log, nil))}
			switch path {
			case "admitted lease":
				if err := server.sessions.Add(&Session{SID: 7}); err != nil {
					t.Fatal(err)
				}
				if !fixtureCurrentOpened(t, server, server.sessions.Lookup(7), opened) {
					t.Fatal("owner was not admitted")
				}
				server.closeAdmitted(server.sessions.Lookup(7))
			case "unpublished lease":
				server.closeSession(opened)
			case "transport":
				server.closeTransport(opened)
			}
			warnings := report.Warnings()
			if len(warnings) != 1 {
				t.Fatalf("cleanup warnings = %+v, want one", warnings)
			}
			issue := warnings[0]
			if issue.Code != diagnostic.CodeDoctorPPPCleanup {
				t.Errorf("warning code = %q", issue.Code)
			}
			if issue.Detail["interface"] != identity.InterfaceName {
				t.Errorf("created interface lost: %+v", issue.Detail)
			}
			if issue.Detail["native-index"] != identity.NativeIndex {
				t.Errorf("created index lost: %+v", issue.Detail)
			}
			if issue.Detail["operation"] != operation.String() {
				t.Errorf("cleanup operation lost: %+v", issue.Detail)
			}
			if issue.Detail["error"] != failure.Error() {
				t.Errorf("cleanup cause lost: %+v", issue.Detail)
			}
			var entry map[string]any
			if err := json.Unmarshal(log.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["code"] != diagnostic.CodeDoctorPPPCleanup {
				t.Errorf("log doctor code = %v", entry["code"])
			}
			if entry["fix"] != ppp.CleanupFailureFix {
				t.Errorf("log fix = %v", entry["fix"])
			}
		})
	}
}
