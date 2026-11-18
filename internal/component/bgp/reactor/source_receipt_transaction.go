// Design: docs/architecture/api/process-protocol.md -- reversible source grants.
// Related: source_receipt.go -- original facts, leases and final-write guards.
// Related: source_lease.go -- authenticated public acquire/retain/release.
package reactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// sourceGrantKey isolates a transaction from another selecting incarnation or
// another configured Peer at the same address. It contains no route identity.
type sourceGrantKey struct {
	peer  *Peer
	owner plugin.SelectedOwner
}

type sourceGrantPhase uint8

const (
	sourceGrantPhaseUnspecified sourceGrantPhase = iota
	sourceGrantPhasePrepared
	sourceGrantPhaseApplied
	sourceGrantPhaseRestored
)

// sourceGrantPin preserves existing authority, never authority inferred from an
// identifier supplied by a plugin. retained is the decision captured at prepare.
type sourceGrantPin struct {
	lease    *sourceReceiptLease
	retained bool
}

// sourceReceiptGrant holds one actual config transaction's session facts.
// Its nil and zero values are invalid. Safe for concurrent use through the
// phase handles below; store.mu serializes transitions and rejects stale
// aliases of a consumed handle. No lock survives a join or external callback.
type sourceReceiptGrant struct {
	store *sourceReceiptStore
	key   sourceGrantKey
	pins  []sourceGrantPin
	phase sourceGrantPhase
}

// preparedSourceGrant may apply the captured revocation or abort before apply.
// Its nil and zero values are invalid. The caller MUST end it through apply or
// abort, and MUST preserve its real transaction's deadline and cleanup path.
type preparedSourceGrant struct{ grant *sourceReceiptGrant }

// appliedSourceGrant owns permanently revoked old guards and rollback pins.
// Its nil and zero values are invalid. The caller MUST finish via finalize or
// restore; a failed apply join still returns this handle for that cleanup.
type appliedSourceGrant struct{ grant *sourceReceiptGrant }

// restoredSourceGrant owns the pins until the plugin has acknowledged restoring
// its candidates and fresh selection revisions. Its nil and zero values are
// invalid. The caller MUST call finish after that settlement, including errors.
type restoredSourceGrant struct{ grant *sourceReceiptGrant }

// prepareGrant captures only this exact source/owner's existing leases.
// The coordinator MUST first cut its receive grant and settle admitted receive
// delivery. It MUST then pair success with abort or apply. The pins keep only
// immutable Session facts, bounded by current leases and one actual transaction
// per exact Peer/owner; candidates and GR policy remain in the selecting plugin.
func (store *sourceReceiptStore) prepareGrant(peer *Peer, owner plugin.SelectedOwner) (*preparedSourceGrant, error) {
	if peer == nil {
		return nil, errors.New("source grant: original peer is missing")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if peer.stopping.Load() {
		return nil, errors.New("source grant: original peer was permanently removed")
	}
	key := sourceGrantKey{peer: peer, owner: owner}
	if store.grants[key] != nil {
		return nil, errors.New("source grant: this original peer and selecting owner already have an open transaction")
	}
	grant := &sourceReceiptGrant{store: store, key: key, phase: sourceGrantPhasePrepared}
	// The scan is bounded by current delivery, lease, write and transaction
	// references. A revoked lease is not transferable restoration authority.
	for _, record := range store.entries {
		if record.facts.source.info.peer != peer {
			continue
		}
		lease := record.leases[owner]
		if lease == nil {
			continue
		}
		select {
		case <-lease.revoked:
			continue
		default:
		}
		grant.pins = append(grant.pins, sourceGrantPin{lease: lease, retained: lease.retained})
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if store.grants == nil {
		store.grants = make(map[sourceGrantKey]*sourceReceiptGrant)
	}
	store.grants[key] = grant
	for _, pin := range grant.pins {
		// Each active key owns at most one pin of a record. The count is
		// bounded by the in-memory transaction map, not a session counter.
		pin.lease.record.pins++
	}
	return &preparedSourceGrant{grant: grant}, nil
}

// apply cancels every captured old lease before joining any writer. Success
// MUST be followed by finalize or restore; an error with a nonnil applied
// handle MUST follow the same cleanup path. No peer-wide epoch is changed.
func (prepared *preparedSourceGrant) apply(ctx context.Context) (*appliedSourceGrant, error) {
	if prepared == nil {
		return nil, errors.New("source grant: prepared transaction is missing")
	}
	grant := prepared.grant
	if grant == nil {
		return nil, errors.New("source grant: prepared transaction is uninitialized")
	}
	grant.store.mu.Lock()
	if err := grant.require(sourceGrantPhasePrepared); err != nil {
		grant.store.mu.Unlock()
		return nil, err
	}
	grant.phase = sourceGrantPhaseApplied
	for _, pin := range grant.pins {
		pin.lease.invalidate()
	}
	grant.store.mu.Unlock()
	applied := &appliedSourceGrant{grant: grant}
	err := applied.join(ctx)
	return applied, err
}

// abort releases prepare-only pins without changing lease eligibility. It MUST
// end a successful prepare that was not applied; a consumed alias is refused.
func (prepared *preparedSourceGrant) abort() error {
	if prepared == nil {
		return errors.New("source grant: prepared transaction is missing")
	}
	grant := prepared.grant
	if grant == nil {
		return errors.New("source grant: prepared transaction is uninitialized")
	}
	grant.store.mu.Lock()
	defer grant.store.mu.Unlock()
	if err := grant.require(sourceGrantPhasePrepared); err != nil {
		return err
	}
	grant.release()
	return nil
}

// join settles old guards only. The caller MUST retain the applied handle after
// a canceled join and retry cleanup under its rollback/terminal budget.
func (applied *appliedSourceGrant) join(ctx context.Context) error {
	if applied == nil {
		return errors.New("source grant: applied transaction is missing")
	}
	grant := applied.grant
	if grant == nil {
		return errors.New("source grant: applied transaction is uninitialized")
	}
	grant.store.mu.Lock()
	if err := grant.require(sourceGrantPhaseApplied); err != nil {
		grant.store.mu.Unlock()
		return err
	}
	joins := make([]sourceLeaseJoin, 0, len(grant.pins))
	for _, pin := range grant.pins {
		joins = append(joins, sourceLeaseJoin{peer: grant.key.peer, idle: pin.lease.idle})
	}
	grant.store.mu.Unlock()
	var result error
	for _, join := range joins {
		if err := waitSourceReceiptWrites(ctx, join.idle); err != nil {
			result = errors.Join(result, fmt.Errorf("source grant: original peer %s writers did not settle: %w", join.peer.addrString, err))
		}
	}
	return result
}

// restore issues fresh lease contexts only for the original authenticated
// Process under its restored exact explicit source grant. It MUST follow a
// successful old-writer join; it never waits while holding a peer lock. The
// caller MUST restore candidate pins and publish fresh selection revisions,
// settle output, then call finish. An old admitted guard is never revived.
func (applied *appliedSourceGrant) restore(server *pluginserver.Server, proc *process.Process) (*restoredSourceGrant, error) {
	if applied == nil {
		return nil, errors.New("source grant: applied transaction is missing")
	}
	grant := applied.grant
	if grant == nil {
		return nil, errors.New("source grant: applied transaction is uninitialized")
	}
	if server == nil {
		return nil, errors.New("source grant: authenticating server is missing")
	}
	if proc == nil {
		return nil, errors.New("source grant: authenticated selecting process is missing")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return nil, err
	}
	if !owner.SameIncarnation(grant.key.owner) {
		return nil, errors.New("source grant: another process incarnation cannot restore the original owner")
	}
	peer := grant.key.peer
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	if peer.stopping.Load() {
		return nil, errors.New("source grant: original peer was permanently removed")
	}
	resolved := deliveryPeersFromSettings([]*PeerSettings{peer.settings})
	if err := server.CheckDisseminationSource(proc, resolved[0], bgpevents.Namespace, SelectedDisseminationRole); err != nil {
		return nil, err
	}
	grant.store.mu.Lock()
	defer grant.store.mu.Unlock()
	if err := grant.require(sourceGrantPhaseApplied); err != nil {
		return nil, err
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if peer.stopping.Load() {
		return nil, errors.New("source grant: original peer was permanently removed")
	}
	// Preflight every record before replacing any authority. The receive grant
	// stays locked through this leaf mutation, as it does for public Acquire.
	for _, pin := range grant.pins {
		if len(pin.lease.writes) != 0 {
			return nil, errors.New("source grant: old final writers have not joined")
		}
		if grant.store.entries[pin.lease.record.id] != pin.lease.record {
			return nil, errors.New("source grant: original transaction-pinned facts are unavailable")
		}
		if current := pin.lease.record.leases[owner]; current != nil {
			if err := current.current(); err != nil {
				return nil, err
			}
		}
	}
	for _, pin := range grant.pins {
		// A transient live guard has joined. Abort restores configured grants,
		// never completed write authority or old Session history.
		if pin.lease.transient {
			continue
		}
		record := pin.lease.record
		current := record.leases[owner]
		if current == nil {
			current = newSourceReceiptLease(grant.store, record, owner)
			record.leases[owner] = current
		}
		if pin.retained {
			current.retained = true
		}
	}
	grant.phase = sourceGrantPhaseRestored
	return &restoredSourceGrant{grant: grant}, nil
}

// finalize accepts the applied removal after joining its old guards and drops
// the transaction pins. It MUST end an accepted applied transaction; a failed
// join retains the pins for a subsequent terminal cleanup attempt.
func (applied *appliedSourceGrant) finalize(ctx context.Context) error {
	if err := applied.join(ctx); err != nil {
		return err
	}
	grant := applied.grant
	grant.store.mu.Lock()
	defer grant.store.mu.Unlock()
	if err := grant.require(sourceGrantPhaseApplied); err != nil {
		return err
	}
	grant.release()
	return nil
}

// finish drops restoration pins after the plugin's abort settlement. It MUST
// follow restore, whether candidate/output settlement succeeded or failed.
func (restored *restoredSourceGrant) finish() error {
	if restored == nil {
		return errors.New("source grant: restored transaction is missing")
	}
	grant := restored.grant
	if grant == nil {
		return errors.New("source grant: restored transaction is uninitialized")
	}
	grant.store.mu.Lock()
	defer grant.store.mu.Unlock()
	if err := grant.require(sourceGrantPhaseRestored); err != nil {
		return err
	}
	grant.release()
	return nil
}

// require needs store.mu. Runtime phase checks protect copied old handles:
// the compiler cannot revoke an alias after a type-state transition.
func (grant *sourceReceiptGrant) require(phase sourceGrantPhase) error {
	if grant.store.grants[grant.key] != grant {
		return errors.New("source grant: transaction has ended")
	}
	if grant.phase != phase {
		return errors.New("source grant: transaction handle belongs to another phase")
	}
	return nil
}

// release needs store.mu and a successful require. Only an actual transaction
// owns these pins; releasing it records no historical receipt or phase key.
func (grant *sourceReceiptGrant) release() {
	delete(grant.store.grants, grant.key)
	for _, pin := range grant.pins {
		pin.lease.record.pins--
		grant.store.dropUnused(pin.lease.record)
	}
	grant.pins = nil
}

// newSourceReceiptLease requires store.mu and already authenticated original
// facts/owner. Public acquisition and trusted rollback are its only producers.
// The caller MUST publish it in record.leases and pair it with revoke.
func newSourceReceiptLease(store *sourceReceiptStore, record *sourceReceiptRecord, owner plugin.SelectedOwner) *sourceReceiptLease {
	lease := &sourceReceiptLease{store: store, record: record, owner: owner,
		revoked: make(chan struct{})}
	if lifetime, bound := owner.ProcessContext(); bound {
		lease.lifetime, lease.cancel = context.WithCancel(lifetime)
	} else {
		// Private primitive/test owners have no Process context. Their
		// explicit revocation remains the sole lifetime of this lease.
		lease.lifetime, lease.cancel = context.WithCancel(context.Background())
	}
	return lease
}
