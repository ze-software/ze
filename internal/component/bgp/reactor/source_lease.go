// Design: docs/architecture/api/process-protocol.md -- authenticated original source leases.
// Related: source_receipt.go -- bounded lease and final-write lifetimes.
package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const sourceLeaseRole = "bgp-selected-output"

var sourceLeaseClock clock.Clock = clock.RealClock{}

// decodeSourceLease keeps both transports on the same bounded scalar shape.
// Unknown fields cannot smuggle a claimed peer, process name or retained facts.
func decodeSourceLease(raw json.RawMessage) (rpc.SourceLeaseInput, error) {
	var input rpc.SourceLeaseInput
	if len(raw) > rpc.MaxMessageSize {
		return input, errors.New("source-lease request exceeds the frame bound; send one receipt and action")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return input, errors.New("source-lease request contains trailing JSON; send one complete input object")
	}
	return input, input.Validate(sourceLeaseClock.Now())
}

func sourceLeaseJSON(server *pluginserver.Server, ctx context.Context, proc *process.Process, raw json.RawMessage) (any, error) {
	input, err := decodeSourceLease(raw)
	if err != nil {
		return nil, err
	}
	return sourceLeaseApply(server, ctx, proc, input)
}

func sourceLeaseNative(server *pluginserver.Server, ctx context.Context, proc *process.Process, value any) (any, error) {
	input, valid := value.(rpc.SourceLeaseInput)
	if !valid {
		return nil, errors.New("source-lease input has the wrong type; send SourceLeaseInput")
	}
	return sourceLeaseApply(server, ctx, proc, input)
}

func sourceLeaseCancellation(server *pluginserver.Server, proc *process.Process, raw json.RawMessage) (pluginserver.EngineRequestScope, error) {
	input, err := decodeSourceLease(raw)
	if err != nil {
		return pluginserver.EngineRequestScope{}, err
	}
	if _, err := sourceLeaseAdmission(server, proc, input); err != nil {
		return pluginserver.EngineRequestScope{}, err
	}
	return pluginserver.NewEngineRequestScope(rpc.MethodSourceLease, input.SourceReceipt, time.UnixMilli(input.DeadlineMS)), nil
}

// sourceLeaseAdmission authenticates before connection-local request enrollment.
// The operation rechecks it before mutation. Release disposes of this owner's
// authority only; its absence is a settled result, never permission to acquire.
// It needs no now-removed receive grant, so terminal cleanup can finish.
func sourceLeaseAdmission(server *pluginserver.Server, proc *process.Process, input rpc.SourceLeaseInput) (plugin.SelectedOwner, error) {
	if err := input.Validate(sourceLeaseClock.Now()); err != nil {
		return plugin.SelectedOwner{}, err
	}
	if proc == nil {
		return plugin.SelectedOwner{}, errors.New("source-lease process is missing; use the authenticated selecting connection")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return plugin.SelectedOwner{}, err
	}
	if err := sourceReceipts.checkPeerOwner(input.SourceReceipt, input.PeerOwner, input.Action == rpc.SourceLeaseRelease); err != nil {
		return plugin.SelectedOwner{}, err
	}
	if input.Action == rpc.SourceLeaseRelease {
		return owner, nil
	}
	peer, err := sourceReceipts.originalPeer(input.SourceReceipt)
	if err != nil {
		return plugin.SelectedOwner{}, err
	}
	if peer.stopping.Load() {
		return plugin.SelectedOwner{}, errors.New("source-lease original peer was removed; reconcile the current source snapshot")
	}
	resolved := deliveryPeersFromSettings([]*PeerSettings{peer.settingsSnapshot()})
	if err := server.CheckDisseminationSource(proc, resolved[0], "bgp", sourceLeaseRole); err != nil {
		return plugin.SelectedOwner{}, err
	}
	if input.Action == rpc.SourceLeaseRetain {
		if _, err := sourceReceipts.ownedLease(input.SourceReceipt, owner); err != nil {
			return plugin.SelectedOwner{}, err
		}
	}
	return owner, nil
}

// sourceLeaseApply is the shared typed/JSON evaluator. Receipt knowledge alone
// never grants source access, and cancellation cannot restore a revoked lease.
func sourceLeaseApply(server *pluginserver.Server, ctx context.Context, proc *process.Process, input rpc.SourceLeaseInput) (*rpc.SourceLeaseOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner, err := sourceLeaseAdmission(server, proc, input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The action is raw boundary input, validated above; unknown values still
	// refuse here instead of becoming an accidental successful no-op.
	switch input.Action {
	case rpc.SourceLeaseAcquire:
		err = sourceLeaseWithGrant(server, proc, input.SourceReceipt, func() error {
			_, acquireErr := sourceReceipts.acquire(input.SourceReceipt, owner)
			return acquireErr
		})
	case rpc.SourceLeaseRetain:
		err = sourceLeaseWithGrant(server, proc, input.SourceReceipt, func() error {
			lease, leaseErr := sourceReceipts.ownedLease(input.SourceReceipt, owner)
			if leaseErr != nil {
				return leaseErr
			}
			return lease.retain()
		})
	case rpc.SourceLeaseRelease:
		err = sourceReceipts.releaseOwned(ctx, input.SourceReceipt, owner)
	case rpc.SourceLeaseUnspecified:
		return nil, errors.New("source-lease action is missing; send acquire, retain or release")
	default:
		return nil, errors.New("source-lease action is unknown; send acquire, retain or release")
	}
	if err != nil {
		return nil, opererr.Wrap(err, "Source lease did not complete for process "+proc.Name(),
			"The original source cannot authorize new selected output until reconciliation succeeds", "ze request plugin restart "+proc.Name())
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
}

// checkPeerOwner binds open input to the captured original incarnation before
// authority changes. Terminal disposal may name an absent receipt, but a known
// receipt never accepts a different nonzero owner. This keeps no history.
func (store *sourceReceiptStore) checkPeerOwner(id, peerOwner uint64, release bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.entries[id]
	if record == nil {
		if release {
			return nil
		}
		return errors.New("source-lease receipt is unknown or released; reconcile the original source snapshot")
	}
	if release {
		if peerOwner == 0 {
			return nil
		}
	}
	if record.facts.peerOwner != peerOwner {
		return errors.New("source-lease peer owner does not match the original receipt; preserve its received peer-owner")
	}
	return nil
}

// sourceLeaseWithGrant holds the exact Peer's immutable attachment headers
// through the leaf-table mutation. A receive-only reload cannot remove its
// grant between checking it and creating/retaining authority. The callback MUST
// do only a leaf store mutation: no I/O, waits, Process or Peer lock acquisition.
func sourceLeaseWithGrant(server *pluginserver.Server, proc *process.Process, receipt uint64, mutate func() error) error {
	peer, err := sourceReceipts.originalPeer(receipt)
	if err != nil {
		return err
	}
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	resolved := deliveryPeersFromSettings([]*PeerSettings{peer.settings})
	if err := server.CheckDisseminationSource(proc, resolved[0], "bgp", sourceLeaseRole); err != nil {
		return err
	}
	return mutate()
}

// originalPeer returns only immutable identity, never a session replacement or
// forwarding authority. The caller MUST recheck the lease under store.mu.
func (store *sourceReceiptStore) originalPeer(id uint64) (*Peer, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.entries[id]
	if record == nil {
		return nil, errors.New("source-lease receipt is unknown or released; reconcile the original source snapshot")
	}
	return record.facts.source.info.peer, nil
}

func (store *sourceReceiptStore) ownedLease(id uint64, owner plugin.SelectedOwner) (*sourceReceiptLease, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := owner.Current(); err != nil {
		return nil, err
	}
	record := store.entries[id]
	if record == nil {
		return nil, errors.New("source-lease receipt is unknown or released; reconcile the original source snapshot")
	}
	lease := record.leases[owner]
	if lease == nil {
		return nil, errors.New("source-lease was not acquired by this process incarnation; acquire it before the original DOWN completes")
	}
	return lease, nil
}

// releaseOwned confirms terminal absence for one authenticated incarnation.
// A canceled join may finish and remove its lease before the caller retries;
// keeping that completed lease as history would violate the store's bound.
// An absent record or absent own lease therefore needs no mutation or wait.
// Foreign leases are never inspected or revoked. Safe for concurrent use.
func (store *sourceReceiptStore) releaseOwned(ctx context.Context, id uint64, owner plugin.SelectedOwner) error {
	store.mu.Lock()
	if err := owner.Current(); err != nil {
		store.mu.Unlock()
		return err
	}
	record := store.entries[id]
	if record == nil {
		store.mu.Unlock()
		return nil // No record means no authority or admitted writes for this owner.
	}
	lease := record.leases[owner]
	if lease == nil {
		store.mu.Unlock()
		return nil // This owner has no authority; other owners remain untouched.
	}
	lease.invalidate()
	idle := lease.idle
	store.mu.Unlock()
	return waitSourceReceiptWrites(ctx, idle)
}
