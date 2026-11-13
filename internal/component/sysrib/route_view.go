// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view_test.go -- selected-view session contract.

package sysrib

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type routeViewVisitor func(*rpc.RouteViewEntry) error

// routeViewAuthority is the owner registry; its zero value has no session.
// Safe for concurrent use. The authority lock admits requests and serializes
// cleanup with replacement; requests never hold it while inspecting the RIB.
type routeViewAuthority struct {
	mu             sync.Mutex
	active         *routeViewSession
	cleanupFailure error
	distanceTable  map[string]int
	// Scan and beforeWait are controlled interleaving seams, nil in production.
	scan        func(context.Context, *locrib.RIB, routeViewVisitor) error
	beforeWait  func()
	maintenance chan routeViewMaintenance
}

// routeViewSession owns one generation's subscription and pinned values. Only
// open constructs it; its zero value is uninitialized and has no usable token.
// Callback capture uses only its bounded channel and atomics, never requestMu.
// Requests and the generation watcher serialize reconciliation and expiry.
// Cleanup cancels before joining both, then releases all retained values.
type routeViewSession struct {
	owner            *process.Process
	rib              *locrib.RIB
	conn             *ipc.PluginConn
	processDone      <-chan struct{}
	engineDone       <-chan struct{}
	watchDone        chan struct{}
	ctx              context.Context
	cancel           context.CancelFunc
	token            string
	epoch            string
	snapshotID       string
	requestMu        sync.Mutex
	users            sync.WaitGroup
	unsubscribe      func()
	capture          chan routeViewDirty
	wake             chan struct{}
	overflow         atomic.Bool
	snapshot         []rpc.RouteViewEntry
	snapshotOffset   int
	snapshotDeadline time.Time
	journal          []routeViewRecord
	journalBytes     int
	assigned         uint64
	consumed         uint64
	ready            bool
	localReader      bool
	resetReason      error
	terminalReason   error
	publicationReset *routeViewPublicationReset
	fenceFailure     error
	fence            func() error
	beforeWait       func()
	beforeReconcile  func()
	distances        routeViewDistanceState
	output           *routeViewOutput
	candidates       *routeViewCandidateSnapshot
	maintenance      chan routeViewMaintenance
}

// routeViewMaintenance is the controlled clock input used by lifecycle tests.
// Production authorities leave its channel nil; the real ticker uses maintain.
type routeViewMaintenance struct {
	at   time.Time
	done chan struct{}
}

func (authority *routeViewAuthority) open(ctx context.Context, owner *process.Process, rib *locrib.RIB, input rpc.RouteViewOpenInput) (rpc.RouteViewOpenOutput, error) {
	if err := routeViewAuthorize(owner); err != nil {
		return rpc.RouteViewOpenOutput{}, err
	}
	localReader := false
	switch input.Reader {
	case "", rpc.RouteViewReaderSelectedFeed:
	case rpc.RouteViewReaderLocalAuthority:
		if !owner.Config().Internal {
			return rpc.RouteViewOpenOutput{}, routeViewRefusal("The local routing reader is unavailable to this external process", "External system RIBs receive selected values through the routing feed", "Call ze-sysrib:route-view-open with reader selected-feed")
		}
		localReader = true
	default:
		return rpc.RouteViewOpenOutput{}, routeViewRefusal("Requested routing reader is unknown", "Only declared reader choices can establish a session", "Call ze-sysrib:route-view-open with reader selected-feed")
	}
	if rib == nil {
		return rpc.RouteViewOpenOutput{}, routeViewRefusal("The daemon's selected routing table is unavailable", "A complete selected view is required before forwarding", "Call route-view-open on the daemon's authenticated plugin connection")
	}
	if err := ctx.Err(); err != nil {
		return rpc.RouteViewOpenOutput{}, routeViewStopped(err, owner)
	}
	authority.mu.Lock()
	if authority.active != nil {
		if authority.active.ctx.Err() == nil {
			fix := "ze request plugin restart " + authority.active.owner.Name()
			authority.mu.Unlock()
			return rpc.RouteViewOpenOutput{}, routeViewRefusal("A selected routing view already has an owner", "Close the previous owner's session before opening another", fix)
		}
		// A generation cleanup hook runs once. A later owner therefore retries
		// its stopped predecessor's exact ledger before claiming replacement.
		if err := authority.cleanupLocked(authority.active); err != nil {
			authority.mu.Unlock()
			return rpc.RouteViewOpenOutput{}, err
		}
	}
	// The configure context bounds this scan only. The session must outlive it.
	lifetime, cancel := context.WithCancel(context.Background())
	session := &routeViewSession{
		owner: owner, rib: rib, conn: owner.Conn(), ctx: lifetime, cancel: cancel,
		processDone: owner.Done(), engineDone: owner.EngineDone(), watchDone: make(chan struct{}),
		token: rand.Text(), epoch: rand.Text(), snapshotID: rand.Text(),
		beforeWait: authority.beforeWait, maintenance: authority.maintenance,
		localReader: localReader, ready: localReader, unsubscribe: func() {},
	}
	if !localReader {
		session.capture = make(chan routeViewDirty, rpc.RouteViewRecordsMax)
		session.wake = make(chan struct{}, 1)
		session.snapshotDeadline = time.Now().Add(routeViewSnapshotLifetime)
		session.unsubscribe = rib.OnChange(session.offer)
	}
	go session.watchGeneration()
	session.users.Add(1)
	authority.active = session
	scan := authority.scan
	authority.mu.Unlock()
	if scan == nil {
		scan = routeViewScan
	}
	session.requestMu.Lock()
	err := session.checkRequest(ctx)
	if !localReader {
		work, stop := context.WithDeadline(ctx, session.snapshotDeadline)
		err = session.scanSnapshot(work, scan)
		stop()
	}
	session.requestMu.Unlock()
	session.users.Done()
	if err != nil {
		if cleanupErr := authority.cleanupSession(session); cleanupErr != nil {
			return rpc.RouteViewOpenOutput{}, cleanupErr
		}
		return rpc.RouteViewOpenOutput{}, routeViewStopped(err, owner)
	}
	return rpc.RouteViewOpenOutput{Session: session.token, Epoch: session.epoch, Snapshot: session.snapshotID, Cursor: 0}, nil
}

func (authority *routeViewAuthority) read(ctx context.Context, owner *process.Process, input rpc.RouteViewReadInput) (rpc.RouteViewReadOutput, error) {
	if input.Limit == 0 {
		return rpc.RouteViewReadOutput{}, routeViewRefusal("Selected-view read limit is zero", "Read between 1 and 4096 records per page", routeViewReadFix)
	}
	if input.Limit > rpc.RouteViewRecordsMax {
		return rpc.RouteViewReadOutput{}, routeViewRefusal("Selected-view read limit exceeds 4096", "Read between 1 and 4096 records per page", routeViewReadFix)
	}
	if input.WaitMillis > rpc.RouteViewWaitMillisMax {
		return rpc.RouteViewReadOutput{}, routeViewRefusal("Selected-view wait exceeds 1000 milliseconds", "Use a wait between 0 and 1000 milliseconds", routeViewReadFix)
	}
	var output rpc.RouteViewReadOutput
	err := authority.withSession(ctx, owner, input.Session, func(session *routeViewSession) error {
		if session.localReader {
			return routeViewRefusal("The local routing lease has no selected feed", "Its reader observes the daemon authority directly", "Close this routing view and call ze-sysrib:route-view-open with reader selected-feed before reading pages")
		}
		var err error
		output, err = session.readPage(ctx, input)
		return err
	})
	return output, err
}

// withSession lends exclusive access to the authenticated active session.
// The callback MUST NOT reenter the authority. Cleanup cancels requests before
// waiting, and request admission is counted under mu so Wait cannot race Add.
func (authority *routeViewAuthority) withSession(ctx context.Context, owner *process.Process, token string, fn func(*routeViewSession) error) error {
	if err := routeViewAuthorize(owner); err != nil {
		return err
	}
	authority.mu.Lock()
	session, err := authority.match(owner, token)
	if err != nil {
		authority.mu.Unlock()
		return err
	}
	session.users.Add(1)
	authority.mu.Unlock()
	defer session.users.Done()
	session.requestMu.Lock()
	defer session.requestMu.Unlock()
	if err := session.checkRequest(ctx); err != nil {
		return routeViewStopped(err, owner)
	}
	return fn(session)
}

func (authority *routeViewAuthority) match(owner *process.Process, token string) (*routeViewSession, error) {
	if authority.active == nil {
		return nil, routeViewRefusal("No selected routing view is open", "Open a view before reading or publishing routes", "Call ze-sysrib:route-view-open {} on this process's authenticated connection")
	}
	if authority.active.owner != owner {
		return nil, routeViewRefusal("Selected routing view belongs to another process generation", "A replacement cannot use its predecessor's session", "Call ze-sysrib:route-view-open {} on the replacement's authenticated connection")
	}
	if authority.active.token != token {
		return nil, routeViewRefusal("Selected routing view token does not match", "Use the token returned to this process by route-view-open", "Retry the operation with this process's own route-view-open session token")
	}
	return authority.active, nil
}

func (authority *routeViewAuthority) close(owner *process.Process, token string) (rpc.RouteViewCloseOutput, error) {
	if err := routeViewAuthorize(owner); err != nil {
		return rpc.RouteViewCloseOutput{}, err
	}
	authority.mu.Lock()
	session, err := authority.match(owner, token)
	authority.mu.Unlock()
	if err != nil {
		return rpc.RouteViewCloseOutput{}, err
	}
	if err := authority.cleanupSession(session); err != nil {
		return rpc.RouteViewCloseOutput{}, err
	}
	return rpc.RouteViewCloseOutput{Session: token}, nil
}

// cleanup is generation-scoped; an old process can never close a replacement.
func (authority *routeViewAuthority) cleanup(owner *process.Process) error {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	if authority.active == nil {
		return nil
	}
	if authority.active.owner != owner {
		return nil
	}
	return authority.cleanupLocked(authority.active)
}

func (authority *routeViewAuthority) cleanupSession(session *routeViewSession) error {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	if authority.active != session {
		return nil
	}
	return authority.cleanupLocked(session)
}

func (authority *routeViewAuthority) cleanupLocked(session *routeViewSession) error {
	session.cancel()
	session.users.Wait()
	<-session.watchDone
	session.unsubscribe()
	session.snapshot = nil
	session.journal = nil
	session.releaseCandidates()
	if session.fence != nil {
		if err := session.fence(); err != nil {
			authority.cleanupFailure = err
			return err
		}
	}
	authority.cleanupFailure = nil
	authority.active = nil
	return nil
}

func routeViewAuthorize(owner *process.Process) error {
	if owner == nil {
		return routeViewRefusal("Selected routing view has no authenticated caller", "Only the configured system RIB may own forwarding state", "Authenticate the plugin connection before calling route-view-open")
	}
	if owner.RegistrationName() != configRootRIB {
		return routeViewRefusal("Selected routing view caller is not the configured system RIB", "Only a process configured to run rib may own forwarding state", "Configure this plugin with use rib before requesting a selected routing view")
	}
	return nil
}

func (session *routeViewSession) checkRequest(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.ctx.Err(); err != nil {
		return err
	}
	select {
	case <-session.processDone:
		return context.Canceled
	case <-session.engineDone:
		return context.Canceled
	default:
	}
	return nil
}

// watchGeneration owns cancellation independent of the borrowed configure
// request. Transport failure must cancel a scan before generic dispatch drain
// reaches cleanup; bridge delivery locks are acquired only in this worker.
// Cleanup MUST cancel and join watchDone. The worker takes requestMu only with
// TryLock, so an active scan cannot prevent transport-failure cancellation.
func (session *routeViewSession) watchGeneration() {
	defer close(session.watchDone)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var pending *routeViewMaintenance
	// One lifecycle worker observes liveness and retention until generation stop.
	// Transport state is always inspected outside request and shard locks.
	for {
		control := session.maintenance
		if pending != nil {
			control = nil
		}
		select {
		case <-session.ctx.Done():
			return
		case <-session.processDone:
			session.cancel()
			return
		case <-session.engineDone:
			session.cancel()
			return
		case <-ticker.C:
			if session.conn != nil {
				if session.conn.Err() != nil {
					session.cancel()
					return
				}
			}
			at := time.Now()
			if pending != nil {
				at = pending.at
			}
			if session.maintain(at) {
				if pending != nil {
					close(pending.done)
					pending = nil
				}
			}
		case request := <-control:
			if session.maintain(request.at) {
				close(request.done)
				continue
			}
			pending = &request
		}
	}
}

// maintain returns whether the tick was admitted. Busy requests defer work to
// the next tick rather than blocking the worker's generation-failure checks.
// Safe for concurrent use; all retained state is inspected under requestMu.
func (session *routeViewSession) maintain(at time.Time) bool {
	if !session.requestMu.TryLock() {
		return false
	}
	defer session.requestMu.Unlock()
	if session.ctx.Err() != nil {
		return true
	}
	session.expireCandidates(at)
	if session.resetReason != nil {
		return true
	}
	if !session.ready {
		if !at.Before(session.snapshotDeadline) {
			err := session.invalidate(session.restartRefusal("Selected routing snapshot expired", "Rebuild the snapshot before forwarding"))
			if err != nil {
				reportRouteViewFailure(session.owner, err)
			}
			return true
		}
	}
	if _, err := session.synchronize(session.ctx); err != nil {
		if session.fenceFailure != nil {
			reportRouteViewFailure(session.owner, session.fenceFailure)
			return true
		}
		if session.ctx.Err() != nil {
			// Generation cleanup owns the stopped session and its ledger.
			return true
		}
		reportRouteViewFailure(session.owner, err)
		return true
	}
	if len(session.journal) > 0 {
		if !at.Before(session.journal[0].since.Add(routeViewJournalLifetime)) {
			err := session.invalidate(session.restartRefusal("Selected routing journal expired", "Rebuild the snapshot before forwarding"))
			if err != nil {
				reportRouteViewFailure(session.owner, err)
			}
		}
	}
	return true
}

const routeViewReadFix = "Call ze-sysrib:route-view-read with the returned session and cursor, limit 1..4096, and wait-ms 0..1000"

func routeViewRefusal(problem, why, fix string) error {
	return opererr.New(problem, why, fix)
}

func routeViewStopped(err error, owner *process.Process) error {
	if _, operator := errors.AsType[*opererr.Error](err); operator {
		return err
	}
	return opererr.Wrap(err, "Selected routing request stopped before completion", "The request did not return a complete result", "ze request plugin restart "+owner.Name())
}
