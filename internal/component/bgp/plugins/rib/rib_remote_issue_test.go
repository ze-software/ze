// Design: docs/architecture/route-selection.md -- bounded forked next-hop lookups
// Related: rib_remote.go -- the existing worker coalesces and clears lookup issues
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRemoteMetricWorkerRaisesAndClearsBothTransports proves repeated admission
// failures produce one registered warning and a successful feed clears it. The
// engine callback takes both locks, so a report sent under either cannot pass.
func TestRemoteMetricWorkerRaisesAndClearsBothTransports(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			clientSide, engineSide := net.Pipe()
			issues := make(chan rpc.ReportIssueInput, 4)
			var manager *RIBManager
			handle := func(_ context.Context, method string, body json.RawMessage) (json.RawMessage, error) {
				switch method {
				case rpc.MethodRouteMetrics:
					var input rpc.RouteMetricsInput
					if err := json.Unmarshal(body, &input); err != nil {
						return nil, err
					}
					if len(input.Addresses) != 0 {
						return nil, errors.New("controlled next-hop lookup failure")
					}
					return json.Marshal(rpc.RouteMetricsOutput{})
				case rpc.MethodReportIssue:
					if !manager.peerMu.TryLock() {
						return nil, errors.New("report called while peer admission is locked")
					}
					manager.peerMu.Unlock()
					if !manager.forkRIB.mu.TryLock() {
						return nil, errors.New("report called while remote metrics are locked")
					}
					manager.forkRIB.mu.Unlock()
					var issue rpc.ReportIssueInput
					if err := json.Unmarshal(body, &issue); err != nil {
						return nil, err
					}
					select {
					case issues <- issue:
						return json.RawMessage(`{}`), nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				default:
					return nil, errors.New("unexpected engine request")
				}
			}
			conn := clientSide
			serverDone := make(chan struct{})
			var engine *rpc.MuxConn
			if direct {
				bridge := rpc.NewDirectBridge()
				bridge.SetDeliverEvents(func([]string) error { return nil })
				bridge.SetDispatchRPC(handle)
				bridge.SetReady()
				conn = rpc.NewBridgedConn(clientSide, bridge)
				close(serverDone)
			} else {
				engine = rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
				go func() {
					defer close(serverDone)
					for request := range engine.Requests() {
						result, err := handle(ctx, request.Method, request.Params)
						if err != nil {
							if engine.SendError(ctx, request.ID, err.Error()) != nil {
								return
							}
						} else if engine.SendResult(ctx, request.ID, json.RawMessage(result)) != nil {
							return
						}
					}
				}()
			}
			client := sdk.NewWithConn("bgp-rib", conn)
			manager = newRIBManager(client)
			manager.setupRemoteRIB()
			workerDone := make(chan struct{})
			t.Cleanup(func() {
				cancel()
				<-workerDone
				require.NoError(t, client.Close())
				if engine != nil {
					require.NoError(t, engine.Close())
				} else {
					require.NoError(t, engineSide.Close())
				}
				<-serverDone
			})
			go func() {
				defer close(workerDone)
				manager.runRemoteMetrics(ctx)
			}()
			path := locrib.Path{Source: bgpProtocolID, Instance: bgpLocRIBInstance,
				IsBGP: true, NextHop: netip.MustParseAddr("198.51.100.1")}
			for range 3 {
				distance := manager.forkRIB.prospectiveDistance(family.IPv4Unicast,
					netip.MustParsePrefix("10.48.0.0/24"), path)
				require.False(t, distance.Resolved)
			}
			for _, action := range []rpc.ReportAction{rpc.ReportRaise, rpc.ReportClear} {
				select {
				case issue := <-issues:
					require.Equal(t, action, issue.Action)
					require.Equal(t, codeRemoteMetricFailure, issue.Code)
					require.NotNil(t, diagnostic.Lookup(issue.Code))
					require.Contains(t, diagnostic.EventCodes(), issue.Code)
					if action == rpc.ReportRaise {
						require.Equal(t, rpc.ReportWarning, issue.Severity)
						require.Equal(t, "ze show doctor", issue.Fix)
					}
				case <-ctx.Done():
					t.Fatal("the metric worker did not raise and clear the lookup issue")
				}
			}
			select {
			case extra := <-issues:
				t.Fatalf("repeated failures produced an extra report: %+v", extra)
			default:
			}
		})
	}
}

// TestRemoteMetricRevisionFeedUnansweredRequestIsBounded leaves the mandatory
// poll unanswered on a live socket. The worker must retain its fatal behavior,
// report the error, and stop within an independently fixed response bound.
func TestRemoteMetricRevisionFeedUnansweredRequestIsBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	clientSide, engineSide := net.Pipe()
	client := sdk.NewWithConn("bgp-rib", clientSide)
	engine := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	manager := newRIBManager(client)
	manager.setupRemoteRIB()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		manager.runRemoteMetrics(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, client.Close())
		require.NoError(t, engine.Close())
		<-workerDone
	})
	poll, err := prospectiveMetricRequest(ctx, engine)
	require.NoError(t, err)
	require.Equal(t, rpc.MethodRouteMetrics, poll.Method)
	// No response to poll: a live connection must not disable the deadline.
	request, err := prospectiveMetricRequest(ctx, engine)
	require.NoError(t, err, "the unanswered poll must produce a bounded error report")
	require.Equal(t, rpc.MethodReportIssue, request.Method)
	var issue rpc.ReportIssueInput
	require.NoError(t, json.Unmarshal(request.Params, &issue))
	require.Equal(t, rpc.ReportRaise, issue.Action)
	require.Equal(t, rpc.ReportError, issue.Severity)
	require.Equal(t, codeRemoteMetricFailure, issue.Code)
	require.NoError(t, engine.SendOK(ctx, request.ID))
	select {
	case <-workerDone:
	case <-ctx.Done():
		t.Fatal("the mandatory feed did not stop after reporting its unanswered poll")
	}
}
