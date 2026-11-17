// Design: docs/architecture/plugin/rib-storage-design.md -- scoped source faults.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type sourceFailureFixture struct {
	r           *RIBManager
	bridge      *rpc.DirectBridge
	failRelease atomic.Bool
	failReport  atomic.Bool
	mu          sync.Mutex
	reports     []rpc.ReportIssueInput
	issues      chan rpc.ReportIssueInput
}

func newSourceFailureFixture(t *testing.T) *sourceFailureFixture {
	t.Helper()
	f := &sourceFailureFixture{bridge: rpc.NewDirectBridge(), issues: make(chan rpc.ReportIssueInput, 8)}
	client, engine := net.Pipe()
	require.NoError(t, engine.Close())
	p := sdk.NewWithConn("bgp-rib", rpc.NewBridgedConn(client, f.bridge))
	f.r = newRIBManager(p)
	f.r.sourceLeases = newRIBSourceLeases(f.r)
	f.bridge.SetDispatchNative(func(_ context.Context, method string, raw any) (any, bool, error) {
		if method != rpc.MethodSourceLease {
			return nil, false, errors.New("unexpected native method")
		}
		input, ok := raw.(rpc.SourceLeaseInput)
		if !ok {
			return nil, true, errors.New("source lease input has the wrong type")
		}
		if input.Action == rpc.SourceLeaseRelease {
			if f.failRelease.Load() {
				return nil, true, errors.New("controlled source release failure")
			}
		}
		return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
	})
	f.bridge.SetDispatchRPC(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodReportIssue {
			return nil, errors.New("unexpected RPC method")
		}
		if !f.r.peerMu.TryLock() {
			return nil, errors.New("source report held peerMu")
		}
		f.r.peerMu.Unlock()
		if !f.r.sourceLeases.mu.TryLock() {
			return nil, errors.New("source report held the lease registry lock")
		}
		f.r.sourceLeases.mu.Unlock()
		var input rpc.ReportIssueInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, err
		}
		f.mu.Lock()
		f.reports = append(f.reports, input)
		f.mu.Unlock()
		select {
		case f.issues <- input:
		default:
		}
		if f.failReport.Load() {
			return nil, errors.New("controlled source report failure")
		}
		return json.RawMessage(`{}`), nil
	})
	f.bridge.SetReady()
	t.Cleanup(func() { require.NoError(t, p.Close()) })
	return f
}

func TestRIBSourceLeaseFailureStaysScoped(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.111")
	sibling := netip.MustParseAddr("192.0.2.112")
	ref, err := s.acquire(t.Context(), peer, 701, 71)
	require.NoError(t, err)
	ref.Release()
	f.failRelease.Store(true)
	require.Error(t, s.settle(t.Context()))
	require.False(t, s.reportFailures(t.Context()))
	findings := s.doctorFailures()
	require.Len(t, findings, 1)
	require.Equal(t, diagnostic.CodeDoctorBGPSelectedOutput, findings[0].Code)
	require.Contains(t, findings[0].Message, peer.String())
	require.Contains(t, findings[0].Message, "controlled source release failure")
	_, err = f.r.advertiseSnapshot()
	require.NoError(t, err, "a source lease failure must not corrupt healthy captured selections")
	healthy, err := s.acquire(t.Context(), sibling, 702, 72)
	require.NoError(t, err, "another source remains usable")
	_, err = s.acquire(t.Context(), peer, 701, 71)
	require.Error(t, err, "a revoked lease awaiting release cannot be resurrected")
	f.failRelease.Store(false)
	require.NoError(t, s.settle(t.Context()))
	require.False(t, s.reportFailures(t.Context()))
	require.Empty(t, s.doctorFailures())
	require.Len(t, f.reports, 2)
	require.Equal(t, rpc.ReportRaise, f.reports[0].Action)
	require.Equal(t, rpc.ReportWarning, f.reports[0].Severity)
	require.Equal(t, sourceLeaseSubject(peer), f.reports[0].Subject)
	require.Equal(t, rpc.ReportClear, f.reports[1].Action)
	healthy.Release()
	require.NoError(t, s.settle(t.Context()))
	require.Empty(t, s.leases)
	require.Empty(t, s.reported)
}

func TestRIBSourceLeaseReportRetryIsBounded(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.113")
	ref, err := s.acquire(t.Context(), peer, 701, 73)
	require.NoError(t, err)
	ref.Release()
	f.failRelease.Store(true)
	f.failReport.Store(true)
	require.Error(t, s.settle(t.Context()))
	require.True(t, s.reportFailures(t.Context()))
	require.True(t, s.reportFailures(t.Context()))
	require.Len(t, f.reports, 1, "a failed raise is not retried before the report window")
	f.failReport.Store(false)
	s.reportMu.Lock()
	s.reportAfter[peer] = time.Now().Add(-time.Second)
	s.reportMu.Unlock()
	require.False(t, s.reportFailures(t.Context()))
	require.Len(t, f.reports, 2)
	f.failRelease.Store(false)
	require.NoError(t, s.settle(t.Context()))
	require.False(t, s.reportFailures(t.Context()))
	require.Len(t, f.reports, 3)
	require.Equal(t, rpc.ReportClear, f.reports[2].Action)
	require.Empty(t, s.reportAfter)
}

// An actual in-flight SDK release must be joined by a second settlement while
// an unrelated source can still acquire its lease. No sleep decides success.
func TestRIBSourceLeaseSettlementJoinsBusyRelease(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.114")
	sibling := netip.MustParseAddr("192.0.2.115")
	ref, err := s.acquire(t.Context(), peer, 701, 74)
	require.NoError(t, err)
	ref.Release()
	entered := make(chan struct{})
	finish := make(chan struct{})
	f.bridge.SetDispatchNative(func(ctx context.Context, method string, raw any) (any, bool, error) {
		input, ok := raw.(rpc.SourceLeaseInput)
		if !ok {
			return nil, true, errors.New("controlled lease handler received another input type")
		}
		if input.Action == rpc.SourceLeaseRelease {
			if input.SourceReceipt == 74 {
				close(entered)
				select {
				case <-finish:
				case <-ctx.Done():
					return nil, true, ctx.Err()
				}
			}
		}
		return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := make(chan error, 1)
	go func() { first <- s.settle(ctx) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("release did not enter its SDK call")
	}
	healthy, err := s.acquire(ctx, sibling, 702, 75)
	require.NoError(t, err, "blocked source release must not lock out another source")
	interrupted, stopWait := context.WithCancel(ctx)
	stopWait()
	require.ErrorIs(t, s.settle(interrupted), context.Canceled,
		"settlement must enter the busy lease's wait, not skip it and claim completion")
	select {
	case err := <-first:
		t.Fatalf("release returned before its final SDK operation completed: %v", err)
	default:
	}
	close(finish)
	require.NoError(t, <-first)
	healthy.Release()
	require.NoError(t, s.settle(ctx))
}

func TestRIBSourceLeaseWorkerRetriesAndClears(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.116")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ref, err := s.acquire(ctx, peer, 701, 76)
	require.NoError(t, err)
	f.failRelease.Store(true)
	ref.Release()
	s.start(ctx)
	defer s.stop()
	select {
	case issue := <-f.issues:
		require.Equal(t, rpc.ReportRaise, issue.Action)
	case <-ctx.Done():
		t.Fatal("source worker did not raise its failed release")
	}
	f.failRelease.Store(false)
	// No new event wakes the worker: its one retry timer must complete release.
	select {
	case issue := <-f.issues:
		require.Equal(t, rpc.ReportClear, issue.Action)
	case <-ctx.Done():
		t.Fatal("source worker did not retry and clear its failed release")
	}
	s.mu.Lock()
	require.Empty(t, s.leases)
	s.mu.Unlock()
}
