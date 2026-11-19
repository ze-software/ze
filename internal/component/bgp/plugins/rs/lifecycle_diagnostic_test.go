// Design: docs/architecture/doctor-and-health-checks.md -- current RS health and nonblocking SDK reports.
package rs

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRSLifecycleDoctorReadsOnlyCurrentDebt seeds two same-address originals and
// an unknown destination, then recovers only one obligation. A doctor read must
// name the remaining originals, mutate nothing and confer no completion.
func TestRSLifecycleDoctorReadsOnlyCurrentDebt(t *testing.T) {
	original, err := newWorkerKey("192.0.2.1", 701, 41)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.1", 702, 42)
	require.NoError(t, err)
	useful, err := newWorkerKey("192.0.2.3", 703, 43)
	require.NoError(t, err)
	target := rpc.ForwardTarget{Destination: "192.0.2.4", DestinationOwner: 704, DestinationSession: 144}
	rs := &routeServer{peers: map[workerKey]*PeerState{
		original: {replayFailure: errors.New("original page read refused"), Up: true},
		sibling:  {sourceReleaseOwed: true},
		useful:   {Up: true},
	}, unknownOutputs: map[rpc.ForwardTarget]error{target: errors.New("captured writer completion unknown")},
		settlementOwed: errors.New("original cleanup remains owed")}
	before := lifecycleQuestionStatus(t, rs.lifecycleStatus())
	rows, err := rs.lifecycleDoctorCheck(lifecycleDoctorName)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	var messages []string
	for _, row := range rows {
		require.Equal(t, lifecycleDoctorCode, row.Code)
		require.Equal(t, "error", row.Severity)
		require.Equal(t, lifecycleDoctorFix, row.Fix)
		messages = append(messages, row.Message)
	}
	combined := strings.Join(messages, "\n")
	require.Contains(t, combined, "owner 701 connection 41: original page read refused")
	require.Contains(t, combined, "owner 702 connection 42: the original source lease release has not completed")
	require.Contains(t, combined, "owner 704 session 144: captured writer completion unknown")
	require.Contains(t, combined, "original cleanup remains owed")
	require.NotContains(t, combined, useful.address.String())
	after := lifecycleQuestionStatus(t, rs.lifecycleStatus())
	require.ElementsMatch(t, before.sources, after.sources, "doctor must preserve every original source row and field")
	require.ElementsMatch(t, before.activeOutput, after.activeOutput, "doctor must preserve every captured active output")
	require.ElementsMatch(t, before.unknownOutput, after.unknownOutput, "doctor must preserve every unknown output row and field")
	require.Equal(t, before.settlementCause, after.settlementCause, "doctor must not discharge the original settlement cause")
	rs.mu.Lock()
	rs.peers[original].replayFailure = nil
	rs.mu.Unlock()
	rows, err = rs.lifecycleDoctorCheck(lifecycleDoctorName)
	require.NoError(t, err)
	require.Len(t, rows, 3, "successful replay clears only its own current failure")
	for _, row := range rows {
		require.NotContains(t, row.Message, "owner 701 connection 41")
	}
	rs.mu.Lock()
	rs.peers[sibling].sourceReleaseOwed = false
	rs.mu.Unlock()
	rs.outputMu.Lock()
	delete(rs.unknownOutputs, target)
	rs.settlementOwed = nil
	rs.outputMu.Unlock()
	rows, err = rs.lifecycleDoctorCheck(lifecycleDoctorName)
	require.NoError(t, err)
	require.Empty(t, rows, "completed work leaves no diagnostic history")
	_, err = rs.lifecycleDoctorCheck("unregistered-check")
	require.ErrorContains(t, err, "unregistered-check")
}

// TestRSLifecycleProblemReportsThroughActualSDK drives the real RS entry on both
// placements. A held report RPC must not hold event or doctor delivery. Invalid
// source state is reported and creates no actor; a useful attached peer survives.
func TestRSLifecycleProblemReportsThroughActualSDK(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, transport, RunRouteServer)
			reports := make(chan rpc.ReportIssueInput, 2)
			reportReturned := make(chan struct{})
			var reportReturnedOnce sync.Once
			release := make(chan struct{})
			var released sync.Once
			finish := func() { released.Do(func() { close(release) }) }
			t.Cleanup(finish)
			engine.SetRPCHandler(func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				if method != rpc.MethodReportIssue {
					return nil, pluginengine.ErrUnhandledRPC
				}
				defer reportReturnedOnce.Do(func() { close(reportReturned) })
				var input rpc.ReportIssueInput
				if err := json.Unmarshal(params, &input); err != nil {
					return nil, err
				}
				select {
				case reports <- input:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				select {
				case <-release:
					return struct{}{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
			require.NoError(t, engine.Deliver(ctx, "peer 192.0.2.1 remote as 65001 state up"))
			var report rpc.ReportIssueInput
			select {
			case report = <-reports:
			case <-ctx.Done():
				t.Fatal("real RS source refusal never reached the daemon report operation")
			}
			select {
			case <-reportReturned:
				t.Fatal("the admitted report RPC ended before its held-work controls began")
			default:
			}
			require.NoError(t, rpc.ValidateReportIssue(&report))
			require.Equal(t, rpc.ReportRaise, report.Action)
			require.Equal(t, rpc.ReportError, report.Severity)
			require.Equal(t, lifecycleDoctorCode, report.Code)
			require.Equal(t, lifecycleDoctorFix, report.Fix)
			require.Contains(t, report.Message, "state has no original Peer owner")
			require.True(t, slices.Contains(diagnostic.EventCodes(), report.Code), "the real daemon must accept this transient code")
			// The first report is still actually held in the engine evaluator.
			require.NoError(t, engine.Deliver(ctx, "peer 192.0.2.2 remote as 65002 state up"))
			require.NoError(t, engine.Deliver(ctx, "peer 192.0.2.3 remote as 65003 state up reason attached peer-owner 703 connection-id 43 destination-session 143 session-generation 1043"))
			rows, err := engine.DoctorCheck(ctx, lifecycleDoctorName)
			require.NoError(t, err)
			require.Empty(t, rows, "refused input must not manufacture current actors or output debt")
			peers, err := engine.Execute(ctx, "show bgp rs peers", nil)
			require.NoError(t, err)
			var snapshots struct {
				Peers []map[string]any `json:"peers"`
			}
			require.NoError(t, json.Unmarshal(peers, &snapshots))
			require.Len(t, snapshots.Peers, 1)
			require.Equal(t, "192.0.2.3", snapshots.Peers[0]["address"])
			require.Equal(t, "703", snapshots.Peers[0]["peer-owner"])
			require.Equal(t, "43", snapshots.Peers[0]["connection-id"])
			require.Contains(t, routeServerRegistration().DoctorChecks, lifecycleDoctorDeclaration())
			select {
			case <-reportReturned:
				t.Fatal("the report RPC ended before useful event, doctor and peer reads completed")
			default:
			}
			finish()
			select {
			case <-reportReturned:
			case <-ctx.Done():
				t.Fatal("the report evaluator did not finish after its explicit release")
			}
		})
	}
}

// TestRSLifecycleTransientReportWindow flushes the actual bounded report queue
// after each refusal. Two failures in one explicit window publish one event;
// opening the next window publishes again with its own current cause.
func TestRSLifecycleTransientReportWindow(t *testing.T) {
	publisher := &lifecycleIssueCapture{}
	queue := issuequeue.Start(publisher)
	defer queue.Stop()
	rs := &routeServer{issues: queue}
	rs.warnLifecycle("first source refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix)
	queue.Flush()
	publisher.mu.Lock()
	require.Len(t, publisher.issues, 1)
	require.Contains(t, publisher.issues[0].Message, "first source refused")
	publisher.mu.Unlock()
	rs.lifecycleReportMu.Lock()
	rs.lifecycleReportAt = time.Now().Add(time.Hour)
	rs.lifecycleReportMu.Unlock()
	rs.warnLifecycle("same-window source refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix)
	queue.Flush()
	publisher.mu.Lock()
	require.Len(t, publisher.issues, 1, "a repeated refusal must not flood the bounded daemon ring")
	publisher.mu.Unlock()
	rs.lifecycleReportMu.Lock()
	rs.lifecycleReportAt = time.Now().Add(-time.Hour)
	rs.lifecycleReportMu.Unlock()
	rs.warnLifecycle("next-window source refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix)
	queue.Flush()
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	require.Len(t, publisher.issues, 2)
	require.Contains(t, publisher.issues[1].Message, "next-window source refused")
	for _, issue := range publisher.issues {
		require.Equal(t, rpc.ReportError, issue.Severity)
		require.Equal(t, lifecycleDoctorCode, issue.Code)
		require.Equal(t, lifecycleDoctorFix, issue.Fix)
	}
}

type lifecycleIssueCapture struct {
	mu     sync.Mutex
	issues []sdk.Issue
}

func (capture *lifecycleIssueCapture) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	capture.mu.Lock()
	capture.issues = append(capture.issues, issue)
	capture.mu.Unlock()
	return nil
}

func (capture *lifecycleIssueCapture) ClearIssue(context.Context, string, string) error {
	return errors.New("an RS transient error must not be cleared as an active warning")
}

// lifecycleQuestionSnapshot preserves the complete status values and their Go
// types. Only the row order of the three map-derived collections is unordered.
type lifecycleQuestionSnapshot struct {
	sources         []map[string]any
	activeOutput    []rpc.ForwardTarget
	unknownOutput   []map[string]any
	settlementCause string
}

// lifecycleQuestionStatus checks the whole four-field producer contract, then
// leaves every value and every row field intact for exact multiset comparison.
func lifecycleQuestionStatus(t *testing.T, value any) lifecycleQuestionSnapshot {
	t.Helper()
	status, ok := value.(map[string]any)
	require.True(t, ok, "lifecycle status must retain its complete object shape")
	require.Len(t, status, 4, "the question must compare every lifecycle status field")
	sources, ok := status["sources"].([]map[string]any)
	require.True(t, ok, "source rows must preserve every field and its native type")
	active, ok := status["active-output"].([]rpc.ForwardTarget)
	require.True(t, ok, "active output must preserve the complete captured target values")
	unknown, ok := status["unknown-output"].([]map[string]any)
	require.True(t, ok, "unknown output rows must preserve every field and its native type")
	settlement, ok := status["settlement-owed"].(string)
	require.True(t, ok, "settlement must preserve its exact cause")
	return lifecycleQuestionSnapshot{sources: sources, activeOutput: active, unknownOutput: unknown, settlementCause: settlement}
}
