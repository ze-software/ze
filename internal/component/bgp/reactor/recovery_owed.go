// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: session_ownership.go -- the writer that owes a key instead of withdrawing it
// Related: relay_recovery.go -- the lookup transports and the admission item
// Related: adj_rib_out.go -- the owed set beside the destination's routes
package reactor

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
)

// recoveryLookupKeysMax bounds one RIB lookup, so a burst of owed keys costs
// several bounded lookups rather than one unbounded document.
const recoveryLookupKeysMax = 500

// recoveryCommandName is the RIB's recovery command. Its registration is the
// one answer to "is a recovery owner registered".
const recoveryCommandName = "request bgp rib recovery"

var errRecoveryOwnerGone = errors.New("source-DOWN recovery owner left while keys were owed")

// recoveryStallMax bounds the consecutive rounds that decide nothing because
// the read buffer pool is exhausted. The wait between them doubles from
// recoveryStallWaitMin to recoveryStallWaitMax, so the worker gives the pool
// about 25 seconds, plus the rounds' own time, before it retires the
// destination: a key left owed longer would leave the client holding a route
// its source withdrew.
const (
	recoveryStallMax     = 32
	recoveryStallWaitMin = 10 * time.Millisecond
	recoveryStallWaitMax = time.Second
)

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
	wake          chan struct{}
	done          chan struct{}
	ownerLog      sync.Once
	unattachedLog sync.Once
}

// recoveryOwedKey is one key of a round's snapshot: the record it read, and
// the record's epoch and excluded sources as they stood.
type recoveryOwedKey struct {
	key      adjOutKey
	record   *adjOutOwed
	epoch    uint64
	excluded []netip.Addr
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

// recoveryOwedFor reports whether an owner withdrawal of a route source sent
// is left to recovery: recoveryOwed holds, and the recovery owner receives
// source's received UPDATEs. An owner the operator never attached to source
// holds none of its routes and is not fed its withdrawals, so its election
// for the key is not the forwarder's and its answer only delays the
// withdrawal behind every later UPDATE: the writer sends it as it stands.
// A departure keeps recoveryOwed alone: its keys come from the RIB's own
// command, so an owner is already engaged.
func (s *Session) recoveryOwedFor(source *Peer) bool {
	worker := s.recovery.Load()
	if worker == nil {
		return false
	}
	command := worker.owner()
	if command == nil {
		return false
	}
	return worker.ownerReceives(command, source)
}

// ownerReceives reports whether the delivery graph grants command's process
// source's received UPDATE events. The graph is the operator's attach blocks
// and an immutable snapshot, and the lookup allocates nothing.
func (w *recoveryWorker) ownerReceives(command *pluginserver.RegisteredCommand, source *Peer) bool {
	granted := w.adapter.r.api.DeliveryGraph().Receivers(events.LookupNamespaceID(bgpevents.Namespace),
		events.LookupEventTypeID(bgpevents.EventUpdate), events.DirReceived, source.addrString)
	if slices.Contains(granted, command.Process.Name()) {
		return true
	}
	w.unattachedLog.Do(func() {
		fwdLogger().Warn("owner withdrawal sent without election: the RIB recovery owner does not receive the source's UPDATEs",
			"peer", w.peer.Settings().Address, "source", source.addrString, "owner", command.Process.Name(),
			"fix", "attach the RIB process to the source peer with receive [ update-received ]")
	})
	return false
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
	stalls := 0
	for {
		again, err := w.round()
		if w.ctx.Err() != nil {
			return
		}
		if !errors.Is(err, errRecoveryBufferStalled) {
			stalls = 0 // Only consecutive rounds that decided nothing count.
		}
		if errors.Is(err, errRecoveryBufferPartial) {
			continue // The decided keys returned their buffers.
		}
		if errors.Is(err, errRecoveryBufferStalled) {
			stalls++
			if stalls < recoveryStallMax {
				if !w.stallWait(stalls) {
					return
				}
				continue
			}
		}
		if errors.Is(err, errRecoveryDestinationClosed) {
			// The session already left Established: its close drops the owed
			// set and stops this worker, so a reset here would only report a
			// failure that did not happen.
			fwdLogger().Debug("recovery stopped: destination session closed",
				"peer", w.peer.Settings().Address)
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

// stallWait waits before the round after the stalls-th consecutive round that
// decided nothing. It reports false when the worker was stopped meanwhile.
func (w *recoveryWorker) stallWait(stalls int) bool {
	wait := recoveryStallWaitMax
	if stalls < 8 {
		wait = min(recoveryStallWaitMin<<(stalls-1), recoveryStallWaitMax)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-w.ctx.Done():
		return false
	case <-timer.C:
		return true
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
// chunk at a time. It reports again when a key of its snapshot is still owed
// after its admission: left owed on a moved epoch, or on a candidate that
// withdrew the key, re-owed with a source excluded, or re-owed because an
// announcement reached the key since the snapshot. Each such retry needs a
// merge, an exclusion or a new write of that key by a source since the
// snapshot, so rounds are paced by those UPDATEs and cannot hot-loop on
// their own.
func (w *recoveryWorker) round() (bool, error) {
	owed := w.snapshot()
	decided := false
	for fam, keys := range owed {
		for start := 0; start < len(keys); start += recoveryLookupKeysMax {
			chunk := keys[start:min(start+recoveryLookupKeysMax, len(keys))]
			err := w.decide(fam, chunk)
			if errors.Is(err, errRecoveryBufferStalled) && decided {
				// An earlier chunk of this round was decided, so the round
				// is partial, not a stall.
				return false, errRecoveryBufferPartial
			}
			if err != nil {
				// A buffer shortage ends the round too: its undecided keys
				// stay owed, and run starts the next round.
				return false, err
			}
			decided = true
		}
	}
	return w.stillOwed(owed), nil
}

// snapshot copies the owed set of this session under the table mutex, and
// clears each record's movers and its announced mark: a write after this
// point is noted again. The excluded sources it asks with are the record's
// minus its current announcers (adjOutOwed.electableExcluded).
// It is taken before the source cuts are captured, so a withdrawal landing
// between the two is either drained or recorded.
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
		record.movers = record.movers[:0]
		record.announced = false
		owed[record.family] = append(owed[record.family], recoveryOwedKey{key: key, record: record,
			epoch: record.epoch, excluded: record.electableExcluded()})
	}
	return owed
}

// stillOwed reports whether any key of the snapshot is owed after the round.
func (w *recoveryWorker) stillOwed(owed map[family.Family][]recoveryOwedKey) bool {
	table := &w.peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	for _, keys := range owed {
		for i := range keys {
			if table.owed[keys[i].key] != nil {
				return true
			}
		}
	}
	return false
}

// decide runs one lookup and one admission for a chunk of owed keys of one
// family. The order is a constraint: the source cuts are captured after the
// snapshot, so an owe that lands after the snapshot is either drained here or
// decided by the next round. A chunk holds keys of one framing: a destination
// sends a family with ADD-PATH or without it.
func (w *recoveryWorker) decide(fam family.Family, chunk []recoveryOwedKey) error {
	if w.session.tearingDown.Load() {
		return nil // The table resets with the session; nothing stays owed.
	}
	sources := w.adapter.recoverySources()
	stopped, err := drainRecoverySources(w.ctx, sources)
	if err != nil {
		return err
	}
	command := w.owner()
	if command == nil {
		return errRecoveryOwnerGone
	}
	if err := command.Process.DrainEventsApplied(w.ctx); err != nil {
		return err
	}
	request := ribevents.RecoveryRequest{Destination: w.peer.Settings().Address, Family: fam,
		Keys: make([]ribevents.RecoveryKey, len(chunk))}
	admission := &recoveryAdmission{peer: w.peer, session: w.session, ctx: w.ctx,
		done: make(chan struct{}), sources: sources, keys: make([]recoveryAdmitKey, len(chunk))}
	for i := range chunk {
		// The snapshot's excluded set is a copy, so appending to it is safe.
		request.Keys[i] = ribevents.RecoveryKey{NLRI: chunk[i].record.nlri,
			Excluded: append(chunk[i].excluded, stopped...)}
		admission.keys[i] = recoveryAdmitKey{key: chunk[i].key, record: chunk[i].record, epoch: chunk[i].epoch}
	}
	if chunk[0].key.addPath {
		// A departure key at an ADD-PATH destination: the RIB names each of
		// the departed source's sent identifiers and runs no election. The
		// writer withdraws only an identifier no current session owns, which
		// is the reconnect guard: the RIB holds no session generation.
		request.SentAddPath = true
	}
	outcomes, err := w.adapter.lookupRecovery(w.ctx, &request)
	if err != nil {
		return err
	}
	sender := plugin.ProcessSender(command.Process.Name())
	return w.adapter.admitRecovery(admission, outcomes, fam, sender)
}
