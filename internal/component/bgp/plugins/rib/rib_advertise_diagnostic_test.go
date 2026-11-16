// Design: docs/architecture/plugin/rib-storage-design.md -- selected capture health
// Related: rib_advertise_diagnostic.go -- runtime doctor answer and report issue
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestAdvertiseCaptureDoctorReportsFailure checks the owned runtime answer
// before and after a failed capture, including its declared code and repair.
func TestAdvertiseCaptureDoctorReportsFailure(t *testing.T) {
	r := newTestRIBManager(t)
	decl := advertiseDoctorDeclaration()
	require.Equal(t, rpc.DoctorPhaseRuntime, decl.Phase)
	require.Contains(t, decl.Codes, "doctor-bgp-selected-output")
	findings, err := r.advertiseDoctorCheck(decl.Name)
	require.NoError(t, err)
	require.Empty(t, findings, "healthy capture must not report a fabricated problem")
	r.failAdvertise(errors.New("retained attribute handle no longer resolves"))
	findings, err = r.advertiseDoctorCheck(decl.Name)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	require.Equal(t, "doctor-bgp-selected-output", findings[0].Code)
	require.Equal(t, "error", findings[0].Severity)
	require.Contains(t, findings[0].Message, "retained attribute handle no longer resolves")
	require.Contains(t, findings[0].Message, "selected routes")
	require.Equal(t, "ze request restart", findings[0].Fix)
}

// TestAdvertiseCaptureDoctorRefusesUnknownCheck distinguishes an unsupported
// callback name from a healthy result for the declared check.
func TestAdvertiseCaptureDoctorRefusesUnknownCheck(t *testing.T) {
	r := newTestRIBManager(t)
	_, err := r.advertiseDoctorCheck("not-this-rib-check")
	require.ErrorContains(t, err, "not-this-rib-check")
}

// TestAdvertiseCaptureReportsIssueOutsideLocks exercises the actual SDK
// report-issue request and proves repeated failures do not raise duplicates.
func TestAdvertiseCaptureReportsIssueOutsideLocks(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	require.NoError(t, engineEnd.Close())
	p := sdk.NewWithConn("capture-diagnostic-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, p.Close()) })
	r := newRIBManager(p)
	var issues []rpc.ReportIssueInput
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		require.Equal(t, rpc.MethodReportIssue, method)
		if !r.advertise.mu.TryLock() {
			t.Fatal("issue reporting held the selected capture lock")
		}
		r.advertise.mu.Unlock()
		if !r.peerMu.TryLock() {
			t.Fatal("issue reporting held the received peer lock")
		}
		r.peerMu.Unlock()
		var issue rpc.ReportIssueInput
		require.NoError(t, json.Unmarshal(params, &issue))
		issues = append(issues, issue)
		return json.RawMessage(`{}`), nil
	})
	bridge.SetReady()
	r.reportAdvertiseIssue(context.Background())
	require.Empty(t, issues)
	r.failAdvertise(errors.New("retained snapshot unavailable"))
	r.reportAdvertiseIssue(context.Background())
	require.Len(t, issues, 1)
	require.Equal(t, rpc.ReportRaise, issues[0].Action)
	require.Equal(t, rpc.ReportError, issues[0].Severity)
	require.Equal(t, "doctor-bgp-selected-output", issues[0].Code)
	require.Contains(t, issues[0].Message, "selected routes")
	require.Equal(t, "ze request restart", issues[0].Fix)
	r.failAdvertise(errors.New("a later failure must not replace the first cause"))
	r.reportAdvertiseIssue(context.Background())
	require.Len(t, issues, 1, "one latched failure raises one acknowledged issue")
}
