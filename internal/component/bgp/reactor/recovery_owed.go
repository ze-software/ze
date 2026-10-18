// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: session_ownership.go -- the writer that owes a key instead of withdrawing it
// Related: relay_recovery.go -- the lookup transports and the admission item
// Related: adj_rib_out.go -- the owed set beside the destination's routes
package reactor

import (
	"context"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// recoveryLookupKeysMax bounds one RIB lookup, so a burst of owed keys costs
// several bounded lookups rather than one unbounded document.
const recoveryLookupKeysMax = 500

// recoveryCommandName is the RIB's recovery command. Its registration is the
// one answer to "is a recovery owner registered".
const recoveryCommandName = "request bgp rib recovery"

var errRecoveryOwnerGone = errors.New("source-DOWN recovery owner left while keys were owed")

// recoveryWorker decides the keys one destination session owes: one
// long-lived goroutine per destination session, never one per event.
//
// Lifecycle: Session.startRecovery creates it and starts run, before the
// session publishes Established. stop is idempotent, never waits, and is safe
// from any goroutine, the worker's own included. join MUST be called after
// stop, from a goroutine that holds neither writeMu nor the forward worker,
// and waits for run to return. Safe for concurrent use.
type recoveryWorker struct {
	adapter *reactorAPIAdapter
	peer    *Peer
	session *Session
	ctx     context.Context
	cancel  context.CancelFunc
	// wake has one slot: the owed set is the queue, so a skipped send loses
	// nothing.
	wake     chan struct{}
	done     chan struct{}
	ownerLog sync.Once
}

// recoveryOwedKey is one key of a round's snapshot, with the record it read.
type recoveryOwedKey struct {
	key    adjOutKey
	record *adjOutOwed
}

// startRecovery MUST be called once per session, before the session publishes
// Established, so a key owed by the first write already has its worker. A
// session sealed before the worker existed stops it at once.
func (s *Session) startRecovery(peer *Peer) {
	if peer.reactor == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := &recoveryWorker{adapter: &reactorAPIAdapter{r: peer.reactor}, peer: peer, session: s,
		ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{})}
	s.recovery.Store(worker)
	go worker.run()
	if s.tearingDown.Load() {
		worker.stop()
	}
}

// stopRecovery is the stop half of the worker's lifecycle. It never waits,
// so every teardown path may call it, under writeMu included.
func (s *Session) stopRecovery() {
	if worker := s.recovery.Load(); worker != nil {
		worker.stop()
	}
}

// joinRecovery stops the worker, then waits for it. It MUST NOT be called
// under writeMu or from the destination's forward worker, which an admission
// in flight waits on.
func (s *Session) joinRecovery() {
	worker := s.recovery.Load()
	if worker == nil {
		return
	}
	worker.stop()
	<-worker.done
}

// wakeRecovery wakes the worker without blocking.
func (s *Session) wakeRecovery() {
	worker := s.recovery.Load()
	if worker == nil {
		return
	}
	select {
	case worker.wake <- struct{}{}:
	default: // A wake is already pending; it covers this owe.
	}
}

// recoveryOwed reports whether an owner withdrawal at this session is left to
// recovery: the session has a worker and a recovery owner is registered.
func (s *Session) recoveryOwed() bool {
	worker := s.recovery.Load()
	if worker == nil {
		return false
	}
	return worker.owner() != nil
}

func (w *recoveryWorker) stop() {
	w.cancel()
}

// owner is the registered recovery command, or nil when none is: then no
// process retains a route to elect, and RFC 4271 Section 9.2 owes the bare
// withdrawal. After Freeze the lookup reads an immutable snapshot.
func (w *recoveryWorker) owner() *pluginserver.RegisteredCommand {
	api := w.adapter.r.api
	if api == nil {
		return nil
	}
	command := api.Dispatcher().Registry().Lookup(recoveryCommandName)
	if command == nil {
		w.logOwnerless()
		return nil
	}
	if command.Process == nil {
		w.logOwnerless()
		return nil
	}
	return command
}

func (w *recoveryWorker) logOwnerless() {
	w.ownerLog.Do(func() {
		fwdLogger().Warn("owner withdrawal sent without election: no RIB recovery owner is registered",
			"peer", w.peer.Settings().Address,
			"fix", "load the bgp-rib plugin beside bgp-rs or bgp-rr")
	})
}

// run is the worker's loop. It runs for the session's lifetime and ends when
// stop cancels its context. The first round runs without a wake, so a key owed
// before the worker existed is never stranded.
func (w *recoveryWorker) run() {
	defer close(w.done)
	for {
		again, err := w.round()
		if w.ctx.Err() != nil {
			return
		}
		if err != nil {
			w.retire(err)
			return
		}
		if again {
			continue
		}
		select {
		case <-w.ctx.Done():
			return
		case <-w.wake:
		}
	}
}

// retire ends a destination session whose owed keys cannot be decided. A key
// left owed would leave the client with a route its source withdrew.
func (w *recoveryWorker) retire(err error) {
	fwdLogger().Warn("recovery could not decide owed routes; resetting the session",
		"peer", w.peer.Settings().Address, "err", err)
	// The session is retired whatever the NOTIFICATION's fate.
	_ = w.session.teardownAutomatic(message.NotifyCeaseOutOfResources, "")
}

// round decides every key owed when it started, one family and one bounded
// chunk at a time. It reports again when a chunk must be looked up again.
func (w *recoveryWorker) round() (bool, error) {
	owed := w.snapshot()
	again := false
	for fam, keys := range owed {
		for start := 0; start < len(keys); start += recoveryLookupKeysMax {
			chunk := keys[start:min(start+recoveryLookupKeysMax, len(keys))]
			retry, err := w.decide(fam, chunk)
			if err != nil {
				return false, err
			}
			again = again || retry
		}
	}
	return again, nil
}

// snapshot copies the owed set of this session under the table mutex.
func (w *recoveryWorker) snapshot() map[family.Family][]recoveryOwedKey {
	table := &w.peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	if table.session != w.session {
		return nil
	}
	if len(table.owed) == 0 {
		return nil
	}
	owed := make(map[family.Family][]recoveryOwedKey)
	for key, record := range table.owed {
		if record.session != w.session {
			continue
		}
		owed[record.family] = append(owed[record.family], recoveryOwedKey{key: key, record: record})
	}
	return owed
}

// decide runs one lookup and one admission for a chunk of owed keys of one
// family. The order is a constraint: the source cuts are captured after the
// snapshot, so an owe that lands after the snapshot is either drained here or
// decided by the next round.
func (w *recoveryWorker) decide(fam family.Family, chunk []recoveryOwedKey) (bool, error) {
	session, sequence := recoverySnapshot(w.peer)
	if session != w.session {
		return false, nil // The table reset with the session; nothing is owed.
	}
	sources := w.adapter.recoverySources()
	if err := drainRecoverySources(w.ctx, sources); err != nil {
		return false, err
	}
	command := w.owner()
	if command == nil {
		return false, errRecoveryOwnerGone
	}
	if err := command.Process.DrainEventsApplied(w.ctx); err != nil {
		return false, err
	}
	request := ribevents.RecoveryRequest{Destination: w.peer.Settings().Address, Family: fam,
		Keys: make([]ribevents.RecoveryKey, len(chunk))}
	for i := range chunk {
		// An owner withdrawal or a lifecycle removal excludes no source: the
		// withdrawn path is already gone from the RIB, and the source's other
		// paths stay eligible.
		request.Keys[i].NLRI = chunk[i].record.nlri
	}
	outcomes, err := w.adapter.lookupRecovery(w.ctx, &request)
	if err != nil {
		return false, err
	}
	for i := range outcomes {
		// The destination holds the record's advertisement, in its own framing.
		outcomes[i].Sent = [][]byte{chunk[i].record.nlri}
	}
	sender := plugin.ProcessSender(command.Process.Name())
	retry, err := w.adapter.admitRecovery(w.ctx, outcomes, fam, w.peer, session, sequence, sender, sources)
	if err != nil {
		return false, err
	}
	if retry {
		return true, nil
	}
	w.settle(chunk)
	return false, nil
}

// settle removes the records a written admission decided. A record replaced
// by a later owe stays: the next round decides it.
func (w *recoveryWorker) settle(chunk []recoveryOwedKey) {
	table := &w.peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	for i := range chunk {
		if table.owed[chunk[i].key] == chunk[i].record {
			delete(table.owed, chunk[i].key)
		}
	}
}
