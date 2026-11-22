// Design: docs/architecture/plugin/rib-storage-design.md -- queued original validation.
// Related: rpki.go, retained_validation.go -- worker admission and terminal join.
package rpki

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The same production worker is held after it copied its coalesced batch and
// before dispatchMu. Cleanup is delivered while those old values are escaped.
// Actual SDK BatchValidate is observed on both typed and framed placements.
// MUTATION: omit validationSourceAdmitted to send old copied values after cleanup.
func TestRPKICopiedWorkerBatchHonorsSourceCleanupBothTransports(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		for _, transport := range []string{"typed", "framed"} {
			for _, cleanup := range []string{"terminal", "physical", "retained"} {
				t.Run(rail+"/"+transport+"/"+cleanup, func(t *testing.T) {
					rp, bridge := rpkiOwnerPlugin(t)
					old := rpkiSource{owner: 1701, connection: 41}
					fresh := rpkiSource{owner: 1701, connection: 43}
					sibling := rpkiSource{owner: 1702, connection: 42}
					retained := rpkiSource{owner: 1703, connection: 44}
					untracked := rpkiSource{owner: 1704, connection: 45}
					retain := func(source rpkiSource) {
						payload, err := retention.Encode(source.owner, source.connection, rpkiOwnerAddress, []family.Family{family.IPv4Unicast})
						require.NoError(t, err)
						retainedRPKIEvent(t, rp, retention.EventType, payload)
						rpkiOwnerDown(t, rp, rail, source, rpc.ReasonTCPFailure)
					}
					retainedRPKIUp(t, rp, rail, old)
					rpkiOwnerFeed(t, rp, rail, "old", old, 71)
					retain(old)
					retainedRPKIUp(t, rp, rail, fresh)
					rpkiOwnerFeed(t, rp, rail, "fresh", fresh, 73)
					retainedRPKIUp(t, rp, rail, sibling)
					rpkiOwnerFeed(t, rp, rail, "sibling", sibling, 72)
					retainedRPKIUp(t, rp, rail, retained)
					rpkiOwnerFeed(t, rp, rail, "retained", retained, 74)
					retain(retained)
					// The existing tracker cap can leave a live initial decision untracked.
					// An actual admitted UP must still permit that decision, never default it.
					retainedRPKIUp(t, rp, rail, untracked)
					rp.validateCh <- validationRequest{peerAddr: rpkiOwnerAddress, source: untracked, family: "ipv4/unicast",
						prefix: "10.0.0.0/24", msgID: 75, state: ValidationValid, generation: rp.validationGeneration.Load()}
					entered, release := make(chan struct{}), make(chan struct{})
					releaseWorker := sync.OnceFunc(func() { close(release) })
					var once sync.Once
					rp.validationDispatchHook = func() { once.Do(func() { close(entered); <-release }) }
					captured := rpkiOwnerDecisionTransport(t, rp, bridge, transport, 1)
					var workers sync.WaitGroup
					workers.Go(rp.validationWorker)
					t.Cleanup(func() { releaseWorker(); rp.closeStopCh(); workers.Wait() })
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					require.Empty(t, rp.validateCh, "the actual worker has copied the batch")
					generation := rp.validationGeneration.Load()
					want := map[rpkiSource]uint64{sibling: 72, retained: 74, untracked: 75}
					switch cleanup {
					case "terminal":
						rpkiOwnerDown(t, rp, rail, rpkiSource{owner: old.owner}, rpc.ReasonPeerRemoved)
					case "physical":
						rpkiOwnerDown(t, rp, rail, fresh, rpc.ReasonTCPFailure)
						want[old] = 71
					case "retained":
						payload, err := retention.EncodeCleanup(old.owner, old.connection, rpkiOwnerAddress, nil, false)
						require.NoError(t, err)
						retainedRPKIEvent(t, rp, retention.CleanupEventType, payload)
						want[fresh] = 73
					}
					require.Equal(t, generation, rp.validationGeneration.Load(), "source cleanup cannot invalidate healthy owners globally")
					releaseWorker()
					decisions := rpkiOwnerNext(t, captured)
					rpkiOwnerDecisionSources(t, decisions, want)
					for _, decision := range decisions {
						require.True(t, decision.Accept)
						require.Equal(t, ValidationValid, decision.ValState)
					}
					require.Empty(t, captured, "one coalesced actual SDK request supplies useful survivors")
				})
			}
		}
	}
}

// terminalRPKIHeldTransport serves the two actual worker BatchValidate requests
// and the follow-up feed's EmitEvent. The first BatchValidate is held before
// answering; the useful follow-up is free. Closing connections and releasing
// the first request MUST precede the join.
func terminalRPKIHeldTransport(t *testing.T, rp *rPKIPlugin, bridge *rpc.DirectBridge, transport string) (<-chan []rpc.ValidationDecision, func(), <-chan struct{}) {
	t.Helper()
	captured := make(chan []rpc.ValidationDecision, 2)
	release := make(chan struct{})
	settled := make(chan struct{})
	releaseRPC := sync.OnceFunc(func() { close(release) })
	if transport == "typed" {
		var once sync.Once
		bridge.SetBatchValidate(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
			captured <- slices.Clone(decisions)
			once.Do(func() { <-release; close(settled) })
			return &rpc.BatchValidateResult{Accepted: len(decisions)}, nil
		})
		t.Cleanup(releaseRPC)
		return captured, releaseRPC, settled
	}
	client, engine := net.Pipe()
	plugin := sdk.NewWithConn("terminal-rpki", client)
	rp.plugin = plugin
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	finished := make(chan error, 1)
	go func() {
		connection := rpc.NewConn(engine, engine)
		batchCount := 0
		emitCount := 0
		for batchCount < 2 || emitCount < 1 {
			request, err := connection.ReadRequest(ctx)
			if err != nil {
				finished <- err
				return
			}
			switch request.Method {
			case rpc.MethodBatchValidate:
				var input rpc.BatchValidateInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					finished <- err
					return
				}
				captured <- input.Decisions
				if batchCount == 0 {
					select {
					case <-release:
					case <-ctx.Done():
						finished <- ctx.Err()
						return
					}
				}
				if err := connection.SendResult(ctx, request.ID, &rpc.BatchValidateResult{Accepted: len(input.Decisions)}); err != nil {
					finished <- err
					return
				}
				if batchCount == 0 {
					close(settled)
				}
				batchCount++
			case rpc.MethodEmitEvent:
				if err := connection.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: 1}); err != nil {
					finished <- err
					return
				}
				emitCount++
			default:
				finished <- fmt.Errorf("unexpected worker request %s", request.Method)
				return
			}
		}
		finished <- nil
	}()
	t.Cleanup(func() {
		releaseRPC()
		cancel()
		if err := plugin.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := <-finished; err != nil {
			if !t.Failed() {
				t.Error(err)
			}
		}
	})
	return captured, releaseRPC, settled
}

// terminalRPKICleanupCaller gives a test dispatcher two actual SDK command
// requests. The dispatcher invokes the unchanged production cleanup callback;
// its reply is written only when that callback returns. The first caller's
// live deadline can therefore distinguish an outstanding join from success.
// This is a bounded dispatcher harness, not a daemon command registration.
func terminalRPKICleanupCaller(t *testing.T, cleanup func() error, releaseValidation func()) (*sdk.Plugin, <-chan time.Time, <-chan error) {
	t.Helper()
	client, engine := net.Pipe()
	caller := sdk.NewWithConn("terminal-cleanup-observer", client)
	admitted := make(chan time.Time, 1)
	completed := make(chan error, 1)
	finished := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	go func() {
		connection := rpc.NewConn(engine, engine)
		for requestNumber := range 2 {
			request, err := connection.ReadRequest(ctx)
			if err != nil {
				finished <- err
				return
			}
			if request.Method != rpc.MethodDispatchCommand {
				finished <- fmt.Errorf("unexpected cleanup observer request %s", request.Method)
				return
			}
			var input rpc.DispatchCommandInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				finished <- err
				return
			}
			if input.Command != "terminal-cleanup-probe" {
				finished <- fmt.Errorf("unexpected cleanup observer command %s", input.Command)
				return
			}
			if requestNumber == 0 {
				admitted <- time.Now()
			}
			err = cleanup()
			if requestNumber == 0 {
				completed <- err
			}
			if err != nil {
				finished <- err
				return
			}
			if err := rpc.WriteDocumentAnswer(connection.AnswerWriter(ctx), request.ID, rpc.AnswerTail{}, json.RawMessage(`{}`)); err != nil {
				finished <- err
				return
			}
		}
		finished <- nil
	}()
	t.Cleanup(func() {
		releaseValidation()
		cancel()
		if err := caller.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := <-finished; err != nil {
			if !t.Failed() {
				t.Error(err)
			}
		}
	})
	return caller, admitted, completed
}

// Actual admitted validation remains in flight while cleanup is requested.
// The live bounded cleanup caller MUST expire rather than receive premature
// success. Release then permits the same cleanup and a fresh useful sibling
// UPDATE's validation on both actual worker transports. No sleep/poll oracle.
// MUTATION: remove dispatchMu from either cleanup producer to return success.
func TestRPKISourceCleanupJoinsAdmittedBatchValidateBothTransports(t *testing.T) {
	for _, transport := range []string{"typed", "framed"} {
		for _, kind := range []string{"terminal", "physical", "retained"} {
			t.Run(transport+"/"+kind, func(t *testing.T) {
				rp, bridge := rpkiOwnerPlugin(t)
				old := rpkiSource{owner: 1701, connection: 41}
				sibling := rpkiSource{owner: 1702, connection: 42}
				retainedRPKIUp(t, rp, "typed", old)
				rpkiOwnerFeed(t, rp, "typed", "old", old, 71)
				retainedRPKIUp(t, rp, "typed", sibling)
				rpkiOwnerFeed(t, rp, "typed", "sibling", sibling, 72)
				cleanup := func() error {
					rp.handlePeerDown(rpkiOwnerAddress, rpkiSource{owner: old.owner}, rpc.ReasonPeerRemoved)
					return nil
				}
				if kind == "physical" {
					cleanup = func() error { rp.handlePeerDown(rpkiOwnerAddress, old, rpc.ReasonTCPFailure); return nil }
				}
				if kind == "retained" {
					payload, err := retention.Encode(old.owner, old.connection, rpkiOwnerAddress, []family.Family{family.IPv4Unicast})
					require.NoError(t, err)
					retainedRPKIEvent(t, rp, retention.EventType, payload)
					rpkiOwnerDown(t, rp, "typed", old, rpc.ReasonTCPFailure)
					payload, err = retention.EncodeCleanup(old.owner, old.connection, rpkiOwnerAddress, nil, false)
					require.NoError(t, err)
					body, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: json.RawMessage(payload)})
					require.NoError(t, err)
					cleanup = func() error { _, err := rp.handleRetainedValidationEvent(string(body)); return err }
				}
				captured, releaseRPC, settled := terminalRPKIHeldTransport(t, rp, bridge, transport)
				var workers sync.WaitGroup
				workers.Go(rp.validationWorker)
				// Each caller cleanup also releases the held validation request before
				// joining its dispatcher. This owner closes and joins the actual worker.
				t.Cleanup(func() { releaseRPC(); rp.closeStopCh(); workers.Wait() })
				decisions := rpkiOwnerNext(t, captured)
				rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{old: 71, sibling: 72})
				caller, admitted, completed := terminalRPKICleanupCaller(t, cleanup, releaseRPC)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				// The work context is live when the SDK admits the real cleanup request.
				work, stop := context.WithTimeout(ctx, 250*time.Millisecond)
				defer stop()
				require.NoError(t, work.Err())
				result := make(chan error, 1)
				workers.Go(func() { _, _, err := caller.DispatchCommand(work, "terminal-cleanup-probe"); result <- err })
				deadline, hasDeadline := work.Deadline()
				require.True(t, hasDeadline)
				select {
				case at := <-admitted:
					require.True(t, at.Before(deadline), "cleanup was not admitted under a live context")
				case <-ctx.Done():
					t.Fatal("cleanup request was not admitted", ctx.Err())
				}
				select {
				case err := <-result:
					require.ErrorIs(t, err, context.DeadlineExceeded, "cleanup acknowledged while actual BatchValidate was held")
				case <-ctx.Done():
					t.Fatal("bounded cleanup caller did not return", ctx.Err())
				}
				releaseRPC()
				select {
				case <-settled:
				case <-ctx.Done():
					t.Fatal("actual held BatchValidate did not settle", ctx.Err())
				}
				select {
				case err := <-completed:
					require.NoError(t, err)
				case <-ctx.Done():
					t.Fatal("cleanup did not join after release", ctx.Err())
				}
				status, _, err := caller.DispatchCommand(ctx, "terminal-cleanup-probe")
				require.NoError(t, err)
				require.Equal(t, rpc.VerdictDone, status)
				require.Equal(t, 1, rp.originTracker.count())
				require.Equal(t, 1, rp.aspaTracker.count())
				require.False(t, rp.retentionVerdicts.Active(old.owner, old.connection))
				require.True(t, rp.retentionVerdicts.Active(sibling.owner, sibling.connection))
				rpkiOwnerFeed(t, rp, "typed", "sibling", sibling, 73)
				decisions = rpkiOwnerNext(t, captured)
				rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{sibling: 73})
				require.Len(t, decisions, 1)
				require.True(t, decisions[0].Accept)
				require.Equal(t, ValidationValid, decisions[0].ValState)
			})
		}
	}
}
