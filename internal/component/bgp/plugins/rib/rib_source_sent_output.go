// Design: docs/architecture/plugin/rib-storage-design.md -- pending auxiliary source output.
// Related: rib_source_lease.go -- the existing lease worker owns retries and shutdown.
package rib

import (
	"context"
	"errors"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ribSourceSentOutput is one bounded cleanup batch on existing live leases.
// It owns immutable encoded commands and original destination/SENT receipts,
// never borrowed route/pool memory. It starts only at holdPhysicalSentOutput;
// its zero value is invalid. mu protects admission and the busy fence; the
// admitted busy caller alone mutates attempts until publishing completion.
// No mutex is held over engine I/O. No new goroutine, completed-source ledger or retry controller is
// created: the existing source-lease worker and command caller share this debt.
// An UNKNOWN original attempt retains its exact input/deadline and MUST NOT be
// retried or cleared on target DOWN, absence, cancellation ACK or another call.
type ribSourceSentOutput struct {
	leases   *ribSourceLeases // immutable original registry; never a later manager replacement
	mu       sync.Mutex
	busy     chan struct{}
	attempts []ribSourceSentAttempt // nil only after every original attempt joined and payload was cleared
}

type ribSourceSentAttempt struct {
	input   rpc.UpdateRouteInput
	outcome ribSourceSentOutcome
	failure error
}

type ribSourceSentOutcome uint8

const (
	ribSourceSentUnspecified ribSourceSentOutcome = iota
	ribSourceSentReady
	ribSourceSentJoined
	ribSourceSentUnknown
)

// holdPhysicalSentOutput MUST run before releasing the outer source output
// hold, with no peer/storage/election lock held. The supplied writes have passed
// physicalSentOutputReadyLocked and encode their own native wire identity.
// One shared batch is attached to the current physical leases, so memory is
// bounded by pending output plus those leases, rather than their product.
func (s *ribSourceLeases) holdPhysicalSentOutput(source ribPhysicalSource, receipts []uint64, writes []sentLifecycleWrite) *ribSourceSentOutput {
	if len(writes) == 0 {
		return nil
	}
	output := &ribSourceSentOutput{leases: s, attempts: make([]ribSourceSentAttempt, len(writes))}
	for index, write := range writes {
		output.attempts[index] = ribSourceSentAttempt{outcome: ribSourceSentReady, input: rpc.UpdateRouteInput{
			PeerSelector: write.peer, Command: write.command, Meta: sentLifecycleMeta(write),
			DestinationOwner: write.owner, DestinationSession: write.session,
		}}
	}
	s.mu.Lock()
	for _, receipt := range receipts {
		lease := s.leases[receipt]
		if lease == nil {
			continue
		}
		if lease.peerOwner != source.owner {
			continue
		}
		if lease.connectionID != source.connection {
			continue
		}
		lease.sentOutputs = append(lease.sentOutputs, output)
	}
	s.mu.Unlock()
	return output
}

// join serializes the original batch, outside all source/RIB locks. An absent
// batch means this cleanup captured no auxiliary writer. Each ready request
// gets one finite original deadline. Other writes still run after an unrelated
// failure; UNKNOWN never becomes a fresh retry with a new deadline.
func (output *ribSourceSentOutput) join(ctx context.Context, r *RIBManager) error {
	if output == nil {
		return nil
	}
	for {
		output.mu.Lock()
		if output.busy == nil {
			break
		}
		busy := output.busy
		output.mu.Unlock()
		select {
		case <-busy:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	output.busy = make(chan struct{})
	output.mu.Unlock()
	var refused error
	for index := range output.attempts {
		attempt := &output.attempts[index]
		// Closed internal enum: only this file constructs or changes outcomes.
		switch attempt.outcome {
		case ribSourceSentUnspecified:
			panic("BUG: source sent attempt was not constructed")
		case ribSourceSentJoined:
			continue
		case ribSourceSentUnknown:
			refused = errors.Join(refused, attempt.failure)
			continue
		case ribSourceSentReady:
		default:
			panic("BUG: source sent attempt outcome is invalid")
		}
		if err := ctx.Err(); err != nil {
			refused = errors.Join(refused, err)
			continue // This caller never dispatched; a later live caller may try.
		}
		work, cancel := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
		deadline, _ := work.Deadline()
		attempt.input.DeadlineMS = deadline.UnixMilli()
		err := r.joinPhysicalSentAttempt(work, attempt.input)
		cancel()
		if err == nil {
			attempt.outcome = ribSourceSentJoined
			continue
		}
		attempt.failure = err
		var notDispatched *rpc.CachedWriteNotDispatchedError
		var joined *rpc.CachedWriteJoinedError
		if errors.As(err, &notDispatched) {
			refused = errors.Join(refused, err)
			continue
		}
		if errors.As(err, &joined) {
			refused = errors.Join(refused, err)
			continue
		}
		attempt.outcome = ribSourceSentUnknown
		refused = errors.Join(refused, err)
	}
	output.mu.Lock()
	if refused == nil {
		// Every attempt passed its actual original final answer. Drop owned
		// commands/metadata before publishing completion to waiting callers.
		clear(output.attempts)
		output.attempts = nil
	}
	close(output.busy)
	output.busy = nil
	output.mu.Unlock()
	if refused == nil {
		output.leases.forgetPhysicalSentOutput(output)
	}
	return refused
}

// forgetPhysicalSentOutput detaches only this positively completed batch from
// all existing live leases, including untouched references that settlement
// intentionally skips. The caller MUST release output.mu first. This method
// takes only s.mu, performs no I/O and takes no output/RIB/storage lock. Thus
// the order is disjoint, never output.mu -> s.mu or s.mu -> output.mu. Ready,
// held and UNKNOWN batches keep their original payload and registry links.
func (s *ribSourceLeases) forgetPhysicalSentOutput(output *ribSourceSentOutput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lease := range s.leases {
		kept := lease.sentOutputs[:0]
		for _, pending := range lease.sentOutputs {
			if pending != output {
				kept = append(kept, pending)
			}
		}
		clear(lease.sentOutputs[len(kept):])
		lease.sentOutputs = kept
	}
}

// joinPhysicalSentAttempt consumes the original constrained SDK answer. The
// engine's bound maintenance path MUST synchronously join after the captured
// FIFO (reactor_api_batch.go:queueExpectedWrite/destinationBatchQueues). Zero
// withdrawals, nil output and any error do not prove the requested withdrawal.
// Existing hooks are test-only; the shipped path always calls the real SDK.
func (r *RIBManager) joinPhysicalSentAttempt(ctx context.Context, input rpc.UpdateRouteInput) error {
	if r.updateRefuseHook != nil {
		if err := r.updateRefuseHook(input); err != nil {
			return err
		}
	}
	if r.updateHook != nil {
		r.updateHook(input.Command, input.Meta)
		return nil
	}
	result, err := r.plugin.UpdateRouteRequestResult(ctx, input)
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("original auxiliary withdrawal has no final output answer")
	}
	if result.Withdrawn != 1 {
		return errors.New("original auxiliary withdrawal has no positive final withdrawal answer")
	}
	return nil
}

func joinPhysicalSentOutputs(ctx context.Context, r *RIBManager, outputs []*ribSourceSentOutput) error {
	var refused error
	for _, output := range outputs {
		refused = errors.Join(refused, output.join(ctx, r))
	}
	return refused
}

// physicalSentOutputReadyLocked refuses before input mutation when an existing
// SENT receipt has no original destination session. Current metadata is never a
// substitute. Caller MUST hold peerMu; live source leases were captured outside.
func (r *RIBManager) physicalSentOutputReadyLocked(source ribPhysicalSource, receipts []uint64) error {
	for _, destinations := range r.ribOut {
		for _, routes := range destinations {
			for _, entry := range routes {
				if !source.matchesSent(&entry) {
					continue
				}
				if entry.DestinationSession == 0 {
					return sourceCommandRefused("Original auxiliary output has no captured destination session")
				}
				if len(receipts) == 0 {
					return sourceCommandRefused("Original auxiliary output has no pending source lease")
				}
			}
		}
	}
	return nil
}

// joinPhysicalSourceOutput uses the same finite operation budget as the
// ordinary selected-source join. The existing lease worker owns later retries.
func (r *RIBManager) joinPhysicalSourceOutput(output *ribSourceSentOutput) error {
	parent := context.Background()
	if r.selectedDispatch != nil {
		parent = r.selectedDispatch.ctx
	}
	ctx, cancel := context.WithTimeout(parent, rpc.CancellableRequestTimeout)
	defer cancel()
	return output.join(ctx, r)
}
