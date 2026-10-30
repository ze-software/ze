// Design: docs/architecture/api/process-protocol.md -- source-policy transport receipts.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func sourcePolicyRequest(operation rpc.SourcePolicyOperation) rpc.SourcePolicyInput {
	input := rpc.SourcePolicyInput{
		TransactionID: "source-transaction", Operation: operation,
		Source:     rpc.SourcePolicyIdentity{Peer: "source", PeerOwner: 11, SessionGeneration: 12, PolicyEpoch: 13},
		DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli(),
	}
	if operation == rpc.SourcePolicyApply {
		input.NextPolicyEpoch = 14
	}
	if operation == rpc.SourcePolicyAbort {
		input.NextPolicyEpoch = 15
	}
	return input
}

func startSourcePolicyOwner(t *testing.T, wire bool, event sdk.EventHandler, handler func(context.Context, rpc.SourcePolicyInput) error) (*Server, *SourcePolicyOwner) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	s, _ := newLifecycleStartupServer(t)
	const name = "source-policy-test-owner"
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		if wire {
			conn = struct{ net.Conn }{conn}
		}
		p := sdk.NewWithConn(name, conn)
		p.OnConfigure(func([]sdk.ConfigSection) error { return nil })
		p.OnEvent(event)
		p.OnSourcePolicy(handler)
		defer func() { _ = p.Close() }() // Teardown: Run's result is what the fixture reports.
		if err := p.Run(s.Context(), &sdk.Registration{}); err != nil {
			return 1
		}
		return 0
	})
	if err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
		t.Fatal(err)
	}
	owners, err := s.CaptureSourcePolicyOwners([]string{name, name})
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) != 1 {
		t.Fatalf("captured %d owners, want one deduplicated owner", len(owners))
	}
	return s, owners[0]
}

// The production startup, strict event FIFO and both callback transports must
// settle a visible owner projection before granting a lifecycle receipt.
func TestSourcePolicyOwnerApplication(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			var visible, pinned atomic.Int64
			entered, release := make(chan struct{}), make(chan struct{})
			_, owner := startSourcePolicyOwner(t, wire, func(string) error {
				visible.Store(73)
				return nil
			}, func(ctx context.Context, input rpc.SourcePolicyInput) error {
				switch input.Operation {
				case rpc.SourcePolicyPrepare:
					if visible.Load() != 73 {
						return errors.New("prepare preceded prior event application")
					}
					pinned.Store(visible.Load())
				case rpc.SourcePolicyApply:
					close(entered)
					select {
					case <-release:
						visible.Store(0)
					case <-ctx.Done():
						return ctx.Err()
					}
				case rpc.SourcePolicyAbort:
					visible.Store(pinned.Swap(0))
				case rpc.SourcePolicyFinalize:
					pinned.Store(0)
				case rpc.SourcePolicyUnspecified:
					return errors.New("unspecified operation reached owner")
				default:
					return errors.New("unknown operation reached owner")
				}
				return nil
			})
			if !owner.proc.Deliver(process.EventDelivery{Output: `{"route":"retained"}`}) {
				t.Fatal("event admission refused")
			}
			if err := owner.Call(t.Context(), sourcePolicyRequest(rpc.SourcePolicyPrepare)); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- owner.Call(t.Context(), sourcePolicyRequest(rpc.SourcePolicyApply)) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("apply never reached owner")
			}
			select {
			case err := <-done:
				t.Fatalf("acknowledged before owner settlement: %v", err)
			default:
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if visible.Load() != 0 {
				t.Fatal("receipt preceded visible removal")
			}
			if err := owner.Call(t.Context(), sourcePolicyRequest(rpc.SourcePolicyAbort)); err != nil {
				t.Fatal(err)
			}
			if visible.Load() != 73 {
				t.Fatal("abort receipt preceded visible restoration")
			}
			if err := owner.Call(t.Context(), sourcePolicyRequest(rpc.SourcePolicyPrepare)); err != nil {
				t.Fatal(err)
			}
			if err := owner.Call(t.Context(), sourcePolicyRequest(rpc.SourcePolicyFinalize)); err != nil {
				t.Fatal(err)
			}
			if pinned.Load() != 0 {
				t.Fatal("finalize receipt preceded pin release")
			}
		})
	}
}

// A failed event application cannot authorize prepare, and an owner failure
// cannot become success on either transport.
func TestSourcePolicyOwnerFailures(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, fault := range []string{"event", "handler", "deadline", "expired", "process", "connection"} {
			t.Run(fmt.Sprintf("wire=%t/%s", wire, fault), func(t *testing.T) {
				var calls atomic.Int64
				entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
				s, owner := startSourcePolicyOwner(t, wire, func(string) error {
					return errors.New("projection rejected")
				}, func(ctx context.Context, input rpc.SourcePolicyInput) error {
					calls.Add(1)
					defer close(returned)
					deadline, ok := ctx.Deadline()
					if !ok || deadline.UnixMilli() != input.DeadlineMS {
						return errors.New("owner deadline missing")
					}
					if fault == "handler" {
						return errors.New("downstream settlement failed")
					}
					close(entered)
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				input := sourcePolicyRequest(rpc.SourcePolicyPrepare)
				if fault == "event" {
					if !owner.proc.Deliver(process.EventDelivery{Output: `{"route":"bad"}`}) {
						t.Fatal("event admission refused")
					}
				}
				if fault == "deadline" {
					input.DeadlineMS = time.Now().Add(100 * time.Millisecond).UnixMilli()
				}
				if fault == "expired" {
					input.DeadlineMS = time.Now().Add(-time.Second).UnixMilli()
				}
				done := make(chan error, 1)
				go func() { done <- owner.Call(t.Context(), input) }()
				if fault == "process" || fault == "connection" {
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("owner callback never entered")
					}
					if fault == "process" {
						pm := s.procManager.Load()
						pm.AddProcess(owner.Name(), process.NewProcess(plugin.PluginConfig{Name: owner.Name()}))
						defer pm.AddProcess(owner.Name(), owner.proc)
					} else {
						owner.proc.SetConn(nil)
						defer owner.proc.SetConn(owner.conn)
					}
					close(release)
				}
				if err := <-done; err == nil {
					t.Fatal("failed owner operation acknowledged success")
				}
				if fault == "event" || fault == "expired" {
					if calls.Load() != 0 {
						t.Fatal("refused prepare invoked owner")
					}
				}
				if fault == "deadline" {
					select {
					case <-returned:
					case <-time.After(5 * time.Second):
						t.Fatal("bounded handler failed to return")
					}
				}
			})
		}
	}
}

// Ownership is independent of implementation capability: observers are omitted,
// while any unsupported selected owner refuses capture before invocation.
func TestSourcePolicyOwnerPreflight(t *testing.T) {
	var calls atomic.Int64
	s, owner := startSourcePolicyOwner(t, false, func(string) error { return nil }, func(context.Context, rpc.SourcePolicyInput) error {
		calls.Add(1)
		return nil
	})
	pm := s.procManager.Load()
	observer := process.NewProcess(plugin.PluginConfig{Name: "observer"})
	observer.SetRegistration(&plugin.PluginRegistration{})
	pm.AddProcess("observer", observer)
	unsupported := process.NewProcess(plugin.PluginConfig{Name: "unsupported"})
	unsupported.SetRegistration(&plugin.PluginRegistration{SourcePolicyOwner: true})
	pm.AddProcess("unsupported", unsupported)
	owners, err := s.CaptureSourcePolicyOwners([]string{"observer", owner.Name()})
	if err != nil || len(owners) != 1 {
		t.Fatalf("observer affected selection: %v, %d owners", err, len(owners))
	}
	if _, err := s.CaptureSourcePolicyOwners([]string{owner.Name(), "unsupported"}); err == nil {
		t.Fatal("unsupported owner did not refuse preflight")
	}
	if calls.Load() != 0 {
		t.Fatal("preflight invoked a handler")
	}
}

// A shorter caller budget must reach the real owner on both transports. Work
// becomes ready only after that budget expires; a cooperative handler must not
// commit it, and its actual context must end at the caller's effective deadline.
func TestSourcePolicyOwnerEarlierCallerDeadline(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			var commits atomic.Int64
			entered := make(chan time.Time, 1)
			workReady := make(chan struct{})
			returned := make(chan struct{})
			_, owner := startSourcePolicyOwner(t, wire, func(string) error {
				return nil
			}, func(ctx context.Context, _ rpc.SourcePolicyInput) error {
				defer close(returned)
				deadline, ok := ctx.Deadline()
				if !ok {
					return errors.New("owner received no deadline")
				}
				entered <- deadline
				select {
				case <-workReady:
					// Model an owner committing completed work only while its
					// assigned budget remains live. Check the clock as well as
					// cancellation so timer scheduling cannot admit late work.
					if time.Now().Before(deadline) && ctx.Err() == nil {
						commits.Add(1)
					}
				case <-ctx.Done():
				}
				<-ctx.Done()
				return ctx.Err()
			})
			input := sourcePolicyRequest(rpc.SourcePolicyApply)
			input.DeadlineMS = time.Now().Add(3 * time.Second).UnixMilli()
			callerDeadline := time.Now().Add(time.Second)
			ctx, cancel := context.WithDeadline(t.Context(), callerDeadline)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- owner.Call(ctx, input) }()
			var ownerDeadline time.Time
			select {
			case ownerDeadline = <-entered:
			case <-ctx.Done():
				t.Fatal("owner never entered before the caller deadline")
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("caller deadline acknowledged success")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("caller did not return at its deadline")
			}
			<-ctx.Done()
			close(workReady)
			select {
			case <-returned:
			case <-time.After(5 * time.Second):
				t.Fatal("owner did not return after its actual context ended")
			}
			if commits.Load() != 0 {
				t.Errorf("owner committed work after the caller budget expired: %d commits", commits.Load())
			}
			if ownerDeadline.UnixMilli() != callerDeadline.UnixMilli() {
				t.Errorf("owner deadline %d, want effective caller deadline %d",
					ownerDeadline.UnixMilli(), callerDeadline.UnixMilli())
			}
		})
	}
}
