// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view.go -- session admission and generation cleanup.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	routeViewSnapshotLifetime = 2 * time.Minute
	routeViewJournalLifetime  = 30 * time.Second
	routeViewJournalBytesMax  = 16 * 1024 * 1024
)

type routeViewDirty struct {
	key   overflowKey
	since time.Time
}

type routeViewRecord struct {
	cursor uint64
	entry  rpc.RouteViewEntry
	bytes  int
	since  time.Time
}

// offer never waits, copies a path, encodes, or retains Change.Forward. A full
// capture invalidates the session; its next request fences output before reset.
func (session *routeViewSession) offer(change locrib.Change) {
	select {
	case <-session.ctx.Done():
		return
	default:
	}
	dirty := routeViewDirty{key: overflowKey{fam: change.Family, prefix: change.Prefix}, since: time.Now()}
	select {
	case session.capture <- dirty:
	default:
		session.overflow.Store(true)
	}
	select {
	case session.wake <- struct{}{}:
	default:
	}
}

func (session *routeViewSession) scanSnapshot(ctx context.Context, scan func(context.Context, *locrib.RIB, routeViewVisitor) error) error {
	selected := make(map[overflowKey]rpc.RouteViewEntry)
	err := scan(ctx, session.rib, func(entry *rpc.RouteViewEntry) error {
		if err := session.checkRequest(ctx); err != nil {
			return err
		}
		selected[routeViewEntryKey(entry)] = *entry
		return nil
	})
	if err != nil {
		return err
	}
	// Re-read keys, never queued values. A withdrawal after an old scan value
	// removes it; no buffered add can subsequently resurrect that value.
	for {
		if err := session.checkRequest(ctx); err != nil {
			return err
		}
		if session.overflow.Load() {
			return session.invalidate(session.restartRefusal("Selected routing changes exceeded the capture bound", "The previous view must be rebuilt before forwarding"))
		}
		keys := session.takeDirty()
		if len(keys) == 0 {
			break
		}
		for _, dirty := range keys {
			entry, err := routeViewInspect(session.rib, dirty.key)
			if err != nil {
				return err
			}
			if entry.Action == rpc.RouteViewActionWithdraw {
				delete(selected, dirty.key)
				continue
			}
			selected[dirty.key] = entry
		}
	}
	keys := make([]overflowKey, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, routeViewCompareKeys)
	session.snapshot = make([]rpc.RouteViewEntry, len(keys))
	for i, key := range keys {
		session.snapshot[i] = selected[key]
	}
	if err := session.checkRequest(ctx); err != nil {
		return err
	}
	// Completion gets its own marker only after all scan and paging corrections.
	// Revision is never used for this ordering.
	session.assigned = uint64(len(session.snapshot))
	return nil
}

// takeDirty drains the capture in arrival order. A key captured twice keeps
// its first position and its first capture time, the age the journal bound
// reads. This is the journal's record order and so a delta page's order: it
// never comes from a map, so two reads of the same mutations page alike.
func (session *routeViewSession) takeDirty() []routeViewDirty {
	// The bounded initial depth is the barrier: new captures may remain for the
	// next batch, while every mutation captured before this call is included.
	depth := len(session.capture)
	if depth == 0 {
		return nil
	}
	keys := make([]routeViewDirty, 0, depth)
	seen := make(map[overflowKey]struct{}, depth)
	for range depth {
		dirty := <-session.capture
		if _, exists := seen[dirty.key]; exists {
			continue
		}
		seen[dirty.key] = struct{}{}
		keys = append(keys, dirty)
	}
	return keys
}

// synchronize assigns a frontier after all already-captured authoritative
// mutations. The caller MUST hold the session lease from withSession. Distance
// application calls this after Reselect, never substitutes locrib.Revision.
func (session *routeViewSession) synchronize(ctx context.Context) (uint64, error) {
	if err := session.checkRequest(ctx); err != nil {
		return 0, routeViewStopped(err, session.owner)
	}
	if session.resetReason != nil {
		if session.fenceFailure != nil {
			return 0, session.fenceFailure
		}
		return 0, session.resetReason
	}
	if session.localReader {
		return 0, nil
	}
	if session.overflow.Load() {
		err := session.restartRefusal("Selected routing changes exceeded the capture bound", "The previous view must be rebuilt before forwarding")
		if fenceErr := session.invalidate(err); fenceErr != nil {
			return 0, fenceErr
		}
		return 0, err
	}
	keys := session.takeDirty()
	if session.beforeReconcile != nil {
		session.beforeReconcile()
	}
	for _, dirty := range keys {
		key, since := dirty.key, dirty.since
		if err := session.checkRequest(ctx); err != nil {
			reason := routeViewStopped(err, session.owner)
			if fenceErr := session.invalidate(reason); fenceErr != nil {
				return 0, fenceErr
			}
			return 0, reason
		}
		if time.Since(since) > routeViewJournalLifetime {
			return 0, session.resetFailure("Selected routing changes expired before consumption")
		}
		entry, err := routeViewInspect(session.rib, key)
		if err != nil {
			if fenceErr := session.invalidate(err); fenceErr != nil {
				return 0, fenceErr
			}
			return 0, err
		}
		bytes := routeViewEntryBytes(&entry)
		if len(session.journal) == rpc.RouteViewRecordsMax {
			return 0, session.resetFailure("Selected routing journal exceeded 4096 records")
		}
		if bytes > routeViewJournalBytesMax-session.journalBytes {
			return 0, session.resetFailure("Selected routing journal exceeded its retained byte bound")
		}
		if session.assigned == math.MaxUint64 {
			return 0, session.resetFailure("Selected routing cursor reached its maximum value")
		}
		session.assigned++
		session.journal = append(session.journal, routeViewRecord{cursor: session.assigned, entry: entry, bytes: bytes, since: since})
		session.journalBytes += bytes
	}
	if session.overflow.Load() {
		return 0, session.resetFailure("Selected routing changes exceeded the capture bound")
	}
	if err := session.checkRequest(ctx); err != nil {
		reason := routeViewStopped(err, session.owner)
		if fenceErr := session.invalidate(reason); fenceErr != nil {
			return 0, fenceErr
		}
		return 0, reason
	}
	return session.assigned, nil
}

func (session *routeViewSession) resetFailure(problem string) error {
	err := session.restartRefusal(problem, "The previous view must be rebuilt before forwarding")
	if fenceErr := session.invalidate(err); fenceErr != nil {
		return fenceErr
	}
	return err
}

func (session *routeViewSession) invalidate(reason error) error {
	if _, rebuild := reason.(*routeViewRebuildReason); !rebuild {
		if session.terminalReason == nil {
			session.terminalReason = reason
		}
		session.publicationReset = nil
	}
	session.resetReason = reason
	session.unsubscribe()
	session.snapshot = nil
	session.journal = nil
	session.journalBytes = 0
	session.ready = false
	session.releaseCandidates()
	if session.fence != nil {
		session.fenceFailure = session.fence()
		return session.fenceFailure
	}
	return nil
}

// setFence installs the output owner's scoped withdrawal under the lease.
// It MUST finish before replacement. A reset that predates installation also
// fences immediately, so no late output attachment can revive an invalid view.
func (session *routeViewSession) setFence(fence func() error) error {
	if fence == nil {
		return routeViewRefusal("Selected routing output has no withdrawal fence", "The output owner must supply scoped cleanup before publication", "Attach the system-RIB output ledger before publishing forwarding routes")
	}
	if session.fence != nil {
		return routeViewRefusal("Selected routing output already has a withdrawal fence", "Replacing the fence would abandon the existing owner's forwarding ledger", "Keep the existing scoped output ledger until its session is closed")
	}
	session.fence = fence
	if session.resetReason != nil {
		session.fenceFailure = fence()
		return session.fenceFailure
	}
	return nil
}

func (session *routeViewSession) authoritativeRIB() *locrib.RIB { return session.rib }

func (session *routeViewSession) consumedCursor() (uint64, bool) {
	return session.consumed, session.ready
}

// resetStatus distinguishes a recoverable view reset from a failed withdrawal.
// The caller MUST hold the lease. A distance apply may acknowledge the former,
// but MUST NOT turn a fence error into successful authoritative application.
func (session *routeViewSession) resetStatus() (bool, error) {
	return session.resetReason != nil, session.fenceFailure
}

// resetBarrier exposes a completed distance apply whose view must be rebuilt.
// The caller MUST hold the lease and finish the reset fence before this call.
// Its marker forces even an already-current consumer to observe explicit Reset.
func (session *routeViewSession) resetBarrier(ctx context.Context) (uint64, error) {
	if err := session.checkRequest(ctx); err != nil {
		return 0, routeViewStopped(err, session.owner)
	}
	if session.resetReason == nil {
		return 0, routeViewRefusal("Selected routing reset barrier has no pending reset", "A healthy view uses its ordinary synchronization frontier", "Apply the authoritative distance table and use synchronize before acknowledging it")
	}
	if session.fenceFailure != nil {
		return 0, session.fenceFailure
	}
	if session.assigned == math.MaxUint64 {
		return 0, session.restartRefusal("Selected routing cursor reached its maximum value", "A fresh session epoch is required before acknowledging another operation")
	}
	session.assigned++
	return session.assigned, nil
}

func (session *routeViewSession) validateConsumed(cursor uint64) error {
	if session.resetReason != nil {
		if session.fenceFailure != nil {
			return session.fenceFailure
		}
		return session.resetReason
	}
	if session.overflow.Load() {
		return session.resetFailure("Selected routing changes exceeded the capture bound")
	}
	if !session.ready {
		return routeViewRefusal("Selected routing snapshot is incomplete", "Forwarding requires a complete selected view", "Continue ze-sysrib:route-view-read until a snapshot page returns complete true")
	}
	if cursor != session.consumed {
		return routeViewRefusal("Selected routing publication cursor was not consumed", "Publish only from the most recently consumed complete view", "Retry route-view-publish with the cursor from the last completed route-view-read")
	}
	return nil
}

func (session *routeViewSession) readPage(ctx context.Context, input rpc.RouteViewReadInput) (rpc.RouteViewReadOutput, error) {
	if input.Cursor != session.consumed {
		return rpc.RouteViewReadOutput{}, routeViewRefusal("Selected routing read cursor is stale or in the future", "Continue from the last successfully returned cursor", routeViewReadFix)
	}
	if session.resetReason != nil {
		if session.fenceFailure != nil {
			return rpc.RouteViewReadOutput{}, session.fenceFailure
		}
		return session.resetPage(), nil
	}
	if !session.ready {
		if time.Now().After(session.snapshotDeadline) {
			if err := session.invalidate(session.restartRefusal("Selected routing snapshot expired", "Rebuild the snapshot before forwarding")); err != nil {
				return rpc.RouteViewReadOutput{}, err
			}
			return session.resetPage(), nil
		}
		if session.overflow.Load() {
			if err := session.invalidate(session.restartRefusal("Selected routing changes exceeded the capture bound", "Rebuild the snapshot before forwarding")); err != nil {
				return rpc.RouteViewReadOutput{}, err
			}
			return session.resetPage(), nil
		}
		return session.snapshotPage(ctx, int(input.Limit))
	}
	if _, err := session.synchronize(ctx); err != nil {
		return session.readFailure(err)
	}
	if len(session.journal) == 0 {
		if input.WaitMillis > 0 {
			if err := session.wait(ctx, time.Duration(input.WaitMillis)*time.Millisecond); err != nil {
				return rpc.RouteViewReadOutput{}, routeViewStopped(err, session.owner)
			}
			if _, err := session.synchronize(ctx); err != nil {
				return session.readFailure(err)
			}
		}
	}
	if len(session.journal) > 0 {
		if time.Since(session.journal[0].since) > routeViewJournalLifetime {
			if err := session.invalidate(session.restartRefusal("Selected routing journal expired", "Rebuild the snapshot before forwarding")); err != nil {
				return rpc.RouteViewReadOutput{}, err
			}
			return session.resetPage(), nil
		}
	}
	return session.deltaPage(ctx, int(input.Limit))
}

func (session *routeViewSession) wait(ctx context.Context, duration time.Duration) error {
	if session.beforeWait != nil {
		session.beforeWait()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-session.ctx.Done():
		return session.ctx.Err()
	case <-session.processDone:
		return context.Canceled
	case <-session.engineDone:
		return context.Canceled
	case <-session.wake:
		return nil
	case <-timer.C:
		return nil
	}
}

func (session *routeViewSession) resetPage() rpc.RouteViewReadOutput {
	page := rpc.RouteViewReadOutput{Session: session.token, Epoch: session.epoch, Cursor: session.assigned, Phase: rpc.RouteViewPhaseReset}
	if proof := session.publicationReset; proof != nil {
		if proof.visible {
			page.RejectedPublication = proof.input.Operation
		}
	}
	return page
}

func (session *routeViewSession) readFailure(err error) (rpc.RouteViewReadOutput, error) {
	if session.fenceFailure != nil {
		return rpc.RouteViewReadOutput{}, session.fenceFailure
	}
	if errors.Is(err, context.Canceled) {
		return rpc.RouteViewReadOutput{}, err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return rpc.RouteViewReadOutput{}, err
	}
	if session.resetReason != nil {
		return session.resetPage(), nil
	}
	return rpc.RouteViewReadOutput{}, err
}

func (session *routeViewSession) snapshotPage(ctx context.Context, limit int) (rpc.RouteViewReadOutput, error) {
	if _, err := session.synchronize(ctx); err != nil {
		return session.readFailure(err)
	}
	page := rpc.RouteViewReadOutput{Session: session.token, Epoch: session.epoch, Snapshot: session.snapshotID, Phase: rpc.RouteViewPhaseSnapshot}
	base := session.snapshotOffset < len(session.snapshot)
	entries := session.snapshot[session.snapshotOffset:]
	if !base {
		entries = make([]rpc.RouteViewEntry, min(limit, len(session.journal)))
		for i := range entries {
			entries[i] = session.journal[i].entry
		}
	}
	count, err := routeViewPageEntries(&page, entries, limit)
	if err != nil {
		if fenceErr := session.invalidate(err); fenceErr != nil {
			return rpc.RouteViewReadOutput{}, fenceErr
		}
		return session.resetPage(), nil
	}
	if err := session.checkRequest(ctx); err != nil {
		return rpc.RouteViewReadOutput{}, routeViewStopped(err, session.owner)
	}
	// Catch changes captured while copying/encoding this page before declaring
	// completion. They remain snapshot upserts/tombstones in the staging view.
	if _, err := session.synchronize(ctx); err != nil {
		return session.readFailure(err)
	}
	page.Cursor = session.consumed + uint64(count)
	if base {
		clear(session.snapshot[session.snapshotOffset : session.snapshotOffset+count])
		session.snapshotOffset += count
	} else {
		for i := range count {
			session.journalBytes -= session.journal[i].bytes
		}
		clear(session.journal[:count])
		session.journal = session.journal[count:]
	}
	page.Complete = session.snapshotOffset == len(session.snapshot) && len(session.journal) == 0
	if page.Complete {
		if session.assigned == math.MaxUint64 {
			if err := session.invalidate(session.restartRefusal("Selected routing cursor reached its maximum value", "Rebuild the view with a fresh cursor before forwarding")); err != nil {
				return rpc.RouteViewReadOutput{}, err
			}
			return session.resetPage(), nil
		}
		session.assigned++
		page.Cursor++
		session.ready = true
		session.snapshot = nil
	}
	session.consumed = page.Cursor
	return page, nil
}

func (session *routeViewSession) deltaPage(ctx context.Context, limit int) (rpc.RouteViewReadOutput, error) {
	page := rpc.RouteViewReadOutput{Session: session.token, Epoch: session.epoch, Cursor: session.consumed, Phase: rpc.RouteViewPhaseDelta, Complete: true}
	// Only this bounded temporary slice shares journal headers. The committed
	// page receives the owned values before consumed journal cells are cleared.
	entries := make([]rpc.RouteViewEntry, min(limit, len(session.journal)))
	for i := range entries {
		entries[i] = session.journal[i].entry
	}
	count, err := routeViewPageEntries(&page, entries, limit)
	if err != nil {
		if fenceErr := session.invalidate(err); fenceErr != nil {
			return rpc.RouteViewReadOutput{}, fenceErr
		}
		return session.resetPage(), nil
	}
	if err := session.checkRequest(ctx); err != nil {
		return rpc.RouteViewReadOutput{}, routeViewStopped(err, session.owner)
	}
	page.Cursor = session.consumed
	for i := range count {
		page.Cursor = session.journal[i].cursor
		session.journalBytes -= session.journal[i].bytes
	}
	clear(session.journal[:count])
	session.journal = session.journal[count:]
	session.consumed = page.Cursor
	return page, nil
}

func routeViewPageEntries(page *rpc.RouteViewReadOutput, entries []rpc.RouteViewEntry, limit int) (int, error) {
	// The header is encoded off-lock once, reserving the widest possible cursor
	// and the entries property/comma overhead that omitempty leaves out here.
	page.Cursor = math.MaxUint64
	header, err := json.Marshal(page)
	if err != nil {
		return 0, err
	}
	remaining := rpc.RouteViewResultBudget() - len(header) - 16
	count := 0
	for i := range min(limit, len(entries)) {
		encoded, err := json.Marshal(&entries[i])
		if err != nil {
			return 0, err
		}
		if len(encoded)+1 > remaining {
			if count == 0 {
				return 0, routeViewRefusal("A selected routing entry exceeds the complete reply frame", "The entry cannot be transferred without losing forwarding metadata", "Reduce this route's ECMP or label metadata until its complete JSON entry fits the routing-view reply frame")
			}
			break
		}
		remaining -= len(encoded) + 1
		page.Entries = append(page.Entries, entries[i])
		count++
	}
	return count, nil
}

// routeViewRebuildReason marks bounded view retention and cursor failures.
// Only restartRefusal constructs it; its zero value is uninitialized.
// Output delivery and metadata failures MUST NOT acquire this marker.
type routeViewRebuildReason struct{ cause error }

func (reason *routeViewRebuildReason) Error() string { return reason.cause.Error() }
func (reason *routeViewRebuildReason) Unwrap() error { return reason.cause }

func (session *routeViewSession) restartRefusal(problem, why string) error {
	return &routeViewRebuildReason{cause: routeViewRefusal(problem, why, "ze request plugin restart "+session.owner.Name())}
}
