// Design: docs/research/l2tpv2-ze-integration.md -- kernel subsystem design
// Related: kernel_event.go -- event types consumed by the worker
// Related: genl_linux.go -- Generic Netlink message construction
// Related: pppox_linux.go -- PPPoL2TP socket and /dev/ppp operations

//go:build linux

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

var errRelayCallCancelled = errors.New("l2tp: relay call abandoned by the subscriber before the bridge completed")

// No "l2tp:" prefix: the subsystem Start wraps this with fmt.Errorf("l2tp: %w"),
// and the doubled prefix showed up verbatim on the appliance serial console.
var errL2tpFailedToLoadKernelModules = errors.New("failed to load kernel modules (tried l2tp_ppp, pppol2tp)")

// kernelOps holds function pointers for the actual kernel syscalls.
// Tests inject fakes via the struct fields. Production uses newKernelOps().
type kernelOps struct {
	// Generic Netlink operations.
	tunnelCreate  func(localTID, remoteTID uint16, socketFD int, peerAddr netip.AddrPort) (connFD int, err error)
	tunnelDelete  func(localTID uint16) error
	sessionCreate func(p sessionCreateParams) error
	sessionDelete func(tunnelID, localSID uint16) error

	// PPPoL2TP + /dev/ppp combined setup. Takes the setup event and
	// returns all fds needed for the PPP data plane.
	pppSetup func(ev kernelSetupEvent) (pppSessionFDs, error)

	// File descriptor close.
	closeFD func(fd int) error

	// Module probing.
	probeModules func() error
}

// newKernelOps returns a kernelOps with real syscall implementations.
func newKernelOps(genl *genlConn) kernelOps {
	return kernelOps{
		tunnelCreate:  genl.tunnelCreate,
		tunnelDelete:  genl.tunnelDelete,
		sessionCreate: genl.sessionCreate,
		sessionDelete: genl.sessionDelete,
		pppSetup:      pppSetupReal,
		closeFD:       unix.Close,
		probeModules:  probeKernelModules,
	}
}

// pppSetupReal performs the full PPPoL2TP socket + /dev/ppp setup sequence.
func pppSetupReal(ev kernelSetupEvent) (pppSessionFDs, error) {
	return pppSetupWith(ev, pppoxCreate, pppkernel.DevPPPSetup, unix.Close)
}

// pppSetupWith supplies the existing syscall boundaries for fault injection;
// production uses pppoxCreate, DevPPPSetup and unix.Close above.
func pppSetupWith(ev kernelSetupEvent,
	create func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (int, error),
	setup func(int, string) (int, int, int, error), closeFD func(int) error,
) (pppSessionFDs, error) {
	pppoxFD, err := create(ev.socketFD, ev.peerAddr,
		ev.localTID, ev.localSID, ev.remoteTID, ev.remoteSID)
	if err != nil {
		return pppSessionFDs{}, err
	}

	if ev.lnsMode {
		if serr := pppoxSetLNSMode(pppoxFD, true); serr != nil {
			return pppSessionFDs{}, pppkernel.RecordSetupCleanup(fmt.Errorf("l2tp: pppox set LNS mode: %w", serr), closeFD(pppoxFD))
		}
	}
	if ev.sequencing {
		if serr := pppoxSetSendSeq(pppoxFD, true); serr != nil {
			return pppSessionFDs{}, pppkernel.RecordSetupCleanup(fmt.Errorf("l2tp: pppox set send seq: %w", serr), closeFD(pppoxFD))
		}
		if serr := pppoxSetRecvSeq(pppoxFD, true); serr != nil {
			return pppSessionFDs{}, pppkernel.RecordSetupCleanup(fmt.Errorf("l2tp: pppox set recv seq: %w", serr), closeFD(pppoxFD))
		}
	}

	// An L2TP LNS and LAC name their subscriber sessions pppN: the name is
	// not set by config here, only the PPPoE client passes a configured
	// name (plan child-1).
	chanFD, unitFD, unitNum, err := prepareAndSetupPPP(ev, pppoxFD, setup)
	if err != nil {
		return pppSessionFDs{}, pppkernel.RecordSetupCleanup(err, closeFD(pppoxFD))
	}

	// spec-followup-l2tp-call A-4: a LAC-relayed incoming call bridges the
	// pppol2tp channel to the subscriber's PPPoE channel in the kernel; no
	// local PPP unit runs for it. A bridge error fails this kernel setup and
	// the session is torn down: no userspace frame relay exists. A realm call
	// first has its PPPoE side quiesce the subscriber's channel reader, so the
	// bridge is the channel's only consumer, and is told the bridge result.
	if ev.relay != nil {
		if qerr := quiesceRelaySubscriber(ev.relay); qerr != nil {
			return pppSessionFDs{}, rollbackPPPBridge([3]int{chanFD, unitFD, pppoxFD}, qerr, closeFD)
		}
	}
	bridged, berr := maybeBridgePPPoE(ev, chanFD)
	if ev.relay != nil {
		if berr == nil {
			if !ev.relay.markBridged(ev.localTID, ev.localSID) {
				return pppSessionFDs{}, rollbackPPPBridge([3]int{chanFD, unitFD, pppoxFD}, errRelayCallCancelled, closeFD)
			}
		}
	}

	if ev.relay != nil {
		// A realm call never takes the userspace relay fall-back: the PPPoE
		// side refuses the subscriber on a bridge error (Bridged(err)).
		// markBridged ran first, so a Cancel the PPPoE side sends once it
		// saw Bridged(nil) tears the bridged session down.
		if ev.relay.realm != nil {
			// quiesceRelaySubscriber checked this immutable handoff before bridging.
			handoff := ev.relay.handoff.(callsink.RealmHandoff) //nolint:errcheck,forcetypeassert // quiesceRelaySubscriber validated this immutable handoff before bridging
			countRealmBridge(berr)
			handoff.Bridged(berr)
		}
	}
	if berr != nil {
		// The kernel worker cannot close a local lease setup never published.
		return pppSessionFDs{}, fmt.Errorf("l2tp: LAC channel bridge: %w",
			rollbackPPPBridge([3]int{chanFD, unitFD, pppoxFD}, berr, closeFD))
	}

	return pppSessionFDs{
		pppoxFD: pppoxFD,
		chanFD:  chanFD,
		unitFD:  unitFD,
		unitNum: unitNum,
		bridged: bridged,
	}, nil
}

// rollbackPPPBridge closes unpublished setup descriptors once each, retaining
// cleanup errors beside cause. Zero is owned; only negative values mean no
// allocation. The three-slot bound keeps alias detection allocation-free.
func rollbackPPPBridge(fds [3]int, cause error, closeFD func(int) error) error {
	for index, fd := range fds {
		if fd < 0 {
			continue
		}
		if slices.Contains(fds[:index], fd) {
			continue
		}
		if err := closeFD(fd); err != nil {
			cause = pppkernel.RecordSetupCleanup(cause, err)
		}
	}
	return cause
}

// kernelTunnelState tracks kernel-side resources for one L2TP tunnel.
type kernelTunnelState struct {
	localTID     uint16
	birth        subscriberid.L2TPBirth // first admission identifies this native tunnel's diagnostics
	connFD       int                    // dup'd connected socket fd; closed on tunnel delete
	sessionCount int                    // number of kernel sessions on this tunnel
	retiring     bool
	deleted      bool
	closeErr     error
	deleteErr    error
	released     bool
}

// sessionKey uniquely identifies a kernel session.
type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

// quiesceRelaySubscriber refuses a stopped call before PPPIOCBRIDGECHAN. A
// realm then joins its local PPP reader; a Service-Name call has no reader,
// and BeforeBridge has already released its unused local lease.
func quiesceRelaySubscriber(rc *relayCall) error {
	if rc.stopped() {
		return errRelayCallCancelled
	}
	// permission: guards pppSetupReal -- a Service-Name call has no local PPP reader; BeforeBridge owns its unused lease
	if rc.realm == nil {
		return nil
	}
	handoff, ok := rc.handoff.(callsink.RealmHandoff)
	if !ok {
		return errors.New("l2tp: realm relay has no PPP handoff")
	}
	if err := handoff.Quiesce(); err != nil {
		return fmt.Errorf("l2tp: realm %q subscriber could not be quiesced: %w", rc.realm.name, err)
	}
	return nil
}

// kernelWorker is a long-lived goroutine that processes kernel setup
// and teardown events. It owns the Generic Netlink connection and all
// kernel-side state. Communication with the reactor is exclusively via
// channels.
//
// Caller MUST call Stop after Start.
type kernelWorker struct {
	ops    kernelOps
	logger *slog.Logger

	// adoptSocket / releaseSocket hand the per-tunnel connected socket to the
	// listener so it keeps receiving that peer's CONTROL frames after the kernel
	// takes over the tunnel. Set once by setSocketHooks before Start; nil in
	// tests that do not exercise the transport. See UDPListener.adoptTunnelSocket
	// for why this is required rather than optional.
	adoptSocket   func(tid uint16, fd int) error
	releaseSocket func(tid uint16)

	eventCh   chan any // receives kernelSetupEvent and kernelTeardownEvent
	errCh     chan<- kernelSetupFailed
	successCh chan<- kernelSetupSucceeded

	// signalStop must interrupt blocked reporting without waiting for setup
	// or teardown to release the worker mutex.
	stopped atomic.Bool

	mu       sync.Mutex
	tunnels  map[uint16]*kernelTunnelState
	sessions map[sessionKey]*pppSessionFDs

	stop chan struct{}
	wg   sync.WaitGroup
}

// newKernelWorker constructs a kernel worker. The errCh and successCh
// are used to report setup outcomes back to the reactor. The worker does
// not own either channel; the reactor creates and reads them.
//
// successCh may be nil for tests that exercise teardown / failure paths
// only; setupSession then runs the success branch without notifying the
// reactor.
func newKernelWorker(ops kernelOps, errCh chan<- kernelSetupFailed, successCh chan<- kernelSetupSucceeded, logger *slog.Logger) *kernelWorker {
	return &kernelWorker{
		ops:       ops,
		logger:    logger,
		eventCh:   make(chan any, 64),
		errCh:     errCh,
		successCh: successCh,
		tunnels:   make(map[uint16]*kernelTunnelState),
		sessions:  make(map[sessionKey]*pppSessionFDs),
	}
}

// setSocketHooks installs the listener callbacks used to keep receiving a
// tunnel's control frames once its connected socket belongs to the kernel.
// MUST be called before Start. Passing nil for either hook disables it.
func (w *kernelWorker) setSocketHooks(adopt func(tid uint16, fd int) error, release func(tid uint16)) {
	w.adoptSocket = adopt
	w.releaseSocket = release
}

// Start launches the worker goroutine.
func (w *kernelWorker) Start() {
	w.stop = make(chan struct{})
	w.wg.Add(1)
	go w.run()
}

// Stop signals the worker to exit and waits. Idempotent.
func (w *kernelWorker) Stop() {
	w.signalStop()
	w.wg.Wait()
}

// signalStop closes the worker's stop channel without waiting for the
// run goroutine. Idempotent. Separated from Stop so the subsystem can
// break reportSuccess/reportError out of their channel-send selects
// BEFORE teardownAll grabs w.mu -- otherwise a setupSession that was
// blocked on successCh (because the reactor already exited) would
// keep w.mu held indefinitely and deadlock teardownAll.
//
// Caller MUST still call Stop (or Wait) to reap the goroutine.
func (w *kernelWorker) signalStop() {
	if !w.stopped.CompareAndSwap(false, true) {
		return
	}
	close(w.stop)
}

// Enqueue sends an event to the worker. Blocks until the event is
// accepted or the worker stops.
func (w *kernelWorker) Enqueue(ev any) {
	select {
	case w.eventCh <- ev:
	case <-w.stop:
	}
}

// teardownAll cleans up all kernel resources. Called by the subsystem
// during Stop() before the reactor shuts down.
func (w *kernelWorker) teardownAll() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Teardown sessions first (reverse order), then tunnels.
	for key, fds := range w.sessions {
		w.teardownSessionFDsLocked(key, fds)
	}
	for tid := range w.tunnels {
		w.deleteTunnelLocked(tid)
	}
}

// run is the worker's main loop.
func (w *kernelWorker) run() {
	defer w.wg.Done()
	for {
		select {
		case ev := <-w.eventCh:
			w.handleEvent(ev)
		case <-w.stop:
			return
		}
	}
}

// handleEvent dispatches a single event.
func (w *kernelWorker) handleEvent(ev any) {
	switch e := ev.(type) {
	case kernelSetupEvent:
		w.setupSession(e)
	case kernelTeardownEvent:
		w.teardownSession(e)
	}
}

// setupSession creates all kernel resources for a newly established session.
// On failure, cleans up partial state and reports via errCh.
func (w *kernelWorker) setupSession(ev kernelSetupEvent) {
	w.mu.Lock()
	var ready *pppSessionFDs
	var setupError error
	defer func() {
		var snapshot pppSessionFDs
		if ready != nil {
			snapshot = *ready
		}
		w.mu.Unlock()
		// Claiming descriptors takes mu. Never hold it while reporting:
		// neither a full success queue nor a full error queue may block it.
		if ready != nil {
			w.reportSuccess(ev, snapshot, ready)
		} else if setupError != nil {
			w.reportError(ev, setupError)
		}
	}()
	if ev.session == nil {
		setupError = errors.New("l2tp: kernel setup requires its original session")
		return
	}
	if ev.session.removed.Load() {
		setupError = ErrSessionNotFound
		return
	}
	key := sessionKey{tunnelID: ev.localTID, sessionID: ev.localSID}
	if w.sessions[key] != nil {
		setupError = ErrSessionCleanupPending
		return
	}
	if ev.session.nativeOwner.Load() != nil {
		setupError = errors.New("l2tp: original session kernel setup was already consumed")
		return
	}
	owned := &pppSessionFDs{session: ev.session, worker: w, relay: ev.relay, pppoxFD: -1, chanFD: -1, unitFD: -1, deleted: true}
	ev.session.nativeOwner.Store(owned)

	// Step 1: ensure kernel tunnel exists.
	ts, tunnelExisted := w.tunnels[ev.localTID]
	if tunnelExisted {
		if ts.retiring {
			setupError = ErrSessionCleanupPending
			return
		}
	}
	if !tunnelExisted {
		connFD, err := w.ops.tunnelCreate(ev.localTID, ev.remoteTID, ev.socketFD, ev.peerAddr)
		if err != nil {
			w.logger.Error("l2tp: kernel tunnel create failed",
				"local-tid", ev.localTID, "remote-tid", ev.remoteTID,
				"socket-fd", ev.socketFD, "error", err.Error())
			setupError = err
			return
		}
		ts = &kernelTunnelState{localTID: ev.localTID, birth: ev.session.Birth(), connFD: connFD}
		owned.tunnel = ts
		w.tunnels[ev.localTID] = ts
		w.logger.Info("l2tp: kernel tunnel created",
			"local-tid", ev.localTID, "remote-tid", ev.remoteTID,
			"peer-addr", ev.peerAddr.String())

		// The connected socket now outranks the listener for every datagram from
		// this peer (see UDPListener.adoptTunnelSocket), so the listener must read
		// this socket or the tunnel's remaining control messages are lost. Fail
		// the setup if it cannot: a tunnel whose control plane is deaf is worse
		// than one that never came up, and rolling back here keeps the userspace
		// path working instead of stranding a half-live tunnel.
		if w.adoptSocket != nil {
			if adoptErr := w.adoptSocket(ev.localTID, connFD); adoptErr != nil {
				w.logger.Error("l2tp: cannot read kernel tunnel socket; rolling back",
					"local-tid", ev.localTID, "error", adoptErr.Error())
				w.deleteTunnelLocked(ev.localTID)
				setupError = adoptErr
				return
			}
		}
	}
	owned.tunnel = ts

	// Step 2: create kernel session.
	params := sessionCreateParams{
		tunnelID:  ev.localTID,
		localSID:  ev.localSID,
		remoteSID: ev.remoteSID,
		lnsMode:   ev.lnsMode,
		sendSeq:   ev.sequencing,
		recvSeq:   ev.sequencing,
	}
	if err := w.ops.sessionCreate(params); err != nil {
		w.logger.Error("l2tp: kernel session create failed",
			"tunnel-id", ev.localTID, "session-id", ev.localSID, "error", err.Error())
		w.cleanupTunnelIfNew(ev.localTID, tunnelExisted)
		setupError = err
		return
	}
	ts.sessionCount++
	owned.deleted = false
	w.sessions[key] = owned

	// Steps 3-4: PPPoL2TP socket + /dev/ppp setup.
	fds, err := w.ops.pppSetup(ev)
	if err != nil {
		w.logger.Error("l2tp: ppp setup failed",
			"tunnel-id", ev.localTID, "session-id", ev.localSID, "error", err.Error())
		owned.closeErr = pppkernel.SetupCleanupFailure(err)
		w.teardownSessionFDsLocked(key, owned)
		w.cleanupTunnelIfNew(ev.localTID, tunnelExisted)
		setupError = err
		return
	}

	owned.pppoxFD, owned.chanFD, owned.unitFD = fds.pppoxFD, fds.chanFD, fds.unitFD
	owned.unitNum, owned.bridged = fds.unitNum, fds.bridged

	w.logger.Info("l2tp: kernel session established",
		"tunnel-id", ev.localTID, "session-id", ev.localSID,
		"ppp-unit", fds.unitNum)

	ready = owned
}

// reportSuccess sends a setup success to the reactor via successCh.
// successCh may be nil in tests that exercise teardown / failure paths
// only; in that case the success event is dropped silently.
func (w *kernelWorker) reportSuccess(ev kernelSetupEvent, fds pppSessionFDs, owner *pppSessionFDs) {
	if w.successCh == nil {
		return
	}
	select {
	case w.successCh <- kernelSetupSucceeded{
		localTID:                   ev.localTID,
		localSID:                   ev.localSID,
		lnsMode:                    ev.lnsMode,
		sequencing:                 ev.sequencing,
		fds:                        fds,
		owner:                      owner,
		session:                    ev.session,
		proxyInitialRecvLCPConfReq: ev.proxyInitialRecvLCPConfReq,
		proxyLastSentLCPConfReq:    ev.proxyLastSentLCPConfReq,
		proxyLastRecvLCPConfReq:    ev.proxyLastRecvLCPConfReq,
	}:
	case <-w.stop:
	}
}

// takePPPDescriptors transfers the channel and unit exactly once. The worker
// retains the PPPoX transport, including responsibility for disconnecting it.
func (w *kernelWorker) takePPPDescriptors(event kernelSetupSucceeded) (int, int, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fds := w.sessions[sessionKey{event.localTID, event.localSID}]
	if event.session == nil {
		return -1, -1, false
	}
	if event.session.removed.Load() {
		return -1, -1, false
	}
	if fds != nil {
		if fds.session != event.session {
			return -1, -1, false
		}
	}
	if fds == nil || fds != event.owner || fds.bridged || fds.chanFD < 0 || fds.unitFD < 0 {
		return -1, -1, false
	}
	channel, unit := fds.chanFD, fds.unitFD
	fds.chanFD, fds.unitFD = -1, -1
	return channel, unit, true
}

// teardownSession destroys kernel resources for a session.
func (w *kernelWorker) teardownSession(ev kernelTeardownEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if ev.session == nil {
		return
	}
	ev.session.removed.Store(true)
	key := sessionKey{tunnelID: ev.localTID, sessionID: ev.localSID}
	fds, ok := w.sessions[key]
	if !ok {
		// Session might not have kernel resources (setup could have
		// failed or not yet completed).
		return
	}

	// A delayed removal cannot target a replacement using the same pair.
	if fds.session != ev.session {
		return
	}
	if fds.relay != ev.relay {
		return
	}
	w.teardownSessionFDsLocked(key, fds)

	// If this was the last session, delete the kernel tunnel.
	ts, ok := w.tunnels[ev.localTID]
	if ok {
		if ts.sessionCount == 0 {
			w.deleteTunnelLocked(ev.localTID)
		}
	}
}

// originalCleanupStatus reads the immutable original receipt under its worker
// lock. It never selects an owner from a reusable tunnel/session pair.
func (w *kernelWorker) originalCleanupStatus(session *L2TPSession) error {
	if !w.mu.TryLock() {
		return ErrSessionCleanupPending
	}
	defer w.mu.Unlock()
	return w.originalCleanupStatusLocked(session)
}

func (w *kernelWorker) pendingCleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var errs []error
	for _, owned := range w.sessions {
		errs = append(errs, owned.deleteErr, owned.closeErr)
	}
	for _, tunnel := range w.tunnels {
		errs = append(errs, tunnel.deleteErr, tunnel.closeErr)
	}
	if len(w.sessions) != 0 {
		errs = append(errs, ErrSessionCleanupPending)
	}
	if len(w.tunnels) != 0 {
		errs = append(errs, ErrSessionCleanupPending)
	}
	return errors.Join(errs...)
}

func (w *kernelWorker) originalCleanupStatusLocked(session *L2TPSession) error {
	owned := session.nativeOwner.Load()
	if owned == nil {
		// Removal revoked every future setup before this lock was acquired.
		if session.removed.Load() {
			return nil
		}
		return ErrSessionCleanupPending
	}
	if owned.worker != w {
		return ErrSessionCleanupPending
	}
	if !owned.deleted {
		return errors.Join(ErrSessionCleanupPending, owned.deleteErr, owned.closeErr)
	}
	if owned.closeErr != nil {
		return owned.closeErr
	}
	ts := owned.tunnel
	if ts != nil {
		if ts.retiring {
			if !ts.released {
				return errors.Join(ErrSessionCleanupPending, ts.deleteErr, ts.closeErr)
			}
		}
	}
	return nil
}

// cleanupOriginal serializes against in-flight setup and retries only deletes
// for its retained owner. A consumed raw descriptor number is never retried.
func (w *kernelWorker) cleanupOriginal(ctx context.Context, tid uint16, session *L2TPSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !w.mu.TryLock() {
		return ErrSessionCleanupPending
	}
	defer w.mu.Unlock()
	if !session.removed.Load() {
		return ErrSessionCleanupPending
	}
	owned := session.nativeOwner.Load()
	if owned == nil {
		return nil
	}
	if owned.worker != w {
		return ErrSessionCleanupPending
	}
	key := sessionKey{tid, session.localSID}
	if w.sessions[key] == owned {
		w.teardownSessionFDsLocked(key, owned)
	}
	ts := owned.tunnel
	if ts != nil {
		if w.tunnels[tid] == ts {
			if ts.sessionCount == 0 {
				w.deleteTunnelLocked(tid)
			}
		}
	}
	return w.originalCleanupStatusLocked(session)
}

// teardownSessionFDsLocked releases the worker-owned descriptors and deletes
// the kernel session. Transferred PPP descriptors are no longer worker-owned.
// Caller MUST hold w.mu.
func (w *kernelWorker) teardownSessionFDsLocked(key sessionKey, fds *pppSessionFDs) {
	// A LAC-bridged session must tear the kernel channel bridge down before
	// closing the channel fd (spec-followup-l2tp-call A-4), so the pppol2tp
	// and PPPoE channels can be released independently.
	if fds.bridged && fds.chanFD >= 0 {
		if err := unbridgeChannel(fds.chanFD); err != nil {
			w.reportKernelCleanupFailure(kernelCleanupChannelRelease,
				key.tunnelID, key.sessionID, kernelCleanupBirth(fds), fds.chanFD, err)
			fds.closeErr = errors.Join(fds.closeErr, err)
		}
		fds.bridged = false
	}
	// Reverse order: unit fd, channel fd, pppox socket, then genl delete.
	// A failed Close does not block the remaining cleanup operations, but its
	// retained uncertainty prevents this original owner from being retired.
	owned := [3]int{fds.unitFD, fds.chanFD, fds.pppoxFD}
	// Consume recyclable numbers before calling Close. A failed Close is
	// uncertainty, never permission to retry that number after reuse.
	fds.unitFD, fds.chanFD, fds.pppoxFD = -1, -1, -1
	for index, fd := range owned {
		if fd < 0 {
			continue
		}
		if slices.Contains(owned[:index], fd) {
			continue
		}
		if err := w.ops.closeFD(fd); err != nil {
			w.reportKernelCleanupFailure(kernelCleanupSessionClose,
				key.tunnelID, key.sessionID, kernelCleanupBirth(fds), fd, err)
			fds.closeErr = errors.Join(fds.closeErr, err)
		}
	}
	// Closing the pppox socket above already deletes the kernel session
	// (net/l2tp/l2tp_ppp.c pppol2tp_release calls l2tp_session_delete), and
	// that delete may finish before this request: genl then answers ENODEV,
	// which is the end state this delete asks for, not a failure.
	// TestLACRealmBridgeAfterLocalLCP met it on Ze's 7.3 kernel.
	if !fds.deleted {
		err := w.ops.sessionDelete(key.tunnelID, key.sessionID)
		if errors.Is(err, unix.ENODEV) {
			err = nil
		}
		fds.deleteErr = err
		if err != nil {
			w.reportKernelCleanupFailure(kernelCleanupSessionDelete,
				key.tunnelID, key.sessionID, kernelCleanupBirth(fds), -1, err)
		} else {
			fds.deleted = true
			// Confirmed original deletion MUST clear only its own delete warning.
			clearKernelSessionDeleteWarning(key, fds)
			if ts := w.tunnels[key.tunnelID]; ts != nil {
				ts.sessionCount--
			}
		}
	}
	if fds.deleteErr != nil {
		return
	}
	if fds.closeErr != nil {
		return
	}
	delete(w.sessions, key)
}

// deleteTunnelLocked deletes the kernel tunnel and closes its connected socket fd.
func (w *kernelWorker) deleteTunnelLocked(tid uint16) {
	ts, ok := w.tunnels[tid]
	if !ok {
		return
	}
	if ts.sessionCount != 0 {
		return
	}
	// A failed/unknown child Close still reserves this original tunnel.
	for key := range w.sessions {
		if key.tunnelID == tid {
			return
		}
	}
	ts.retiring = true
	if !ts.deleted {
		ts.deleteErr = w.ops.tunnelDelete(tid)
		if errors.Is(ts.deleteErr, unix.ENODEV) {
			ts.deleteErr = nil
		}
		if ts.deleteErr != nil {
			w.reportKernelCleanupFailure(kernelCleanupTunnelDelete,
				tid, 0, ts.birth, -1, ts.deleteErr)
			return
		}
		ts.deleted = true
		// Confirmed original deletion MUST clear only its own delete warning.
		clearKernelTunnelDeleteWarning(ts)
		w.logger.Info("l2tp: kernel tunnel deleted", "tunnel-id", tid)
		// Stop the original listener reader before consuming its descriptor.
		if w.releaseSocket != nil {
			w.releaseSocket(tid)
		}
	}
	if ts.connFD >= 0 {
		fd := ts.connFD
		ts.connFD = -1
		ts.closeErr = w.ops.closeFD(fd)
		if ts.closeErr != nil {
			w.reportKernelCleanupFailure(kernelCleanupTunnelClose,
				tid, 0, ts.birth, fd, ts.closeErr)
		}
	}
	if ts.closeErr != nil {
		return
	}
	ts.released = true
	delete(w.tunnels, tid)
}

// cleanupTunnelIfNew deletes the kernel tunnel if it was freshly created
// for this setup attempt and has no sessions.
func (w *kernelWorker) cleanupTunnelIfNew(localTID uint16, tunnelExisted bool) {
	if tunnelExisted {
		return
	}
	ts, ok := w.tunnels[localTID]
	if !ok {
		return
	}
	if ts.sessionCount <= 0 {
		w.deleteTunnelLocked(localTID)
	}
}

// reportError sends a setup failure to the reactor via errCh.
func (w *kernelWorker) reportError(ev kernelSetupEvent, err error) {
	select {
	case w.errCh <- kernelSetupFailed{
		localTID: ev.localTID,
		localSID: ev.localSID,
		session:  ev.session,
		relay:    ev.relay,
		err:      err,
	}:
	case <-w.stop:
	}
}

// probeKernelModules loads the L2TP kernel module. Tries l2tp_ppp first,
// falls back to pppol2tp. Returns an error if both fail.
// RFC 2661 Section 24.23: fail startup if module probe fails.
//
// Each modprobe invocation gets its own 10s deadline so a hung first call
// does not starve the fallback.
func probeKernelModules() error {
	for _, mod := range [...]string{"l2tp_ppp", "pppol2tp"} {
		if moduleBuiltIn(mod) {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		// mod is bound to literals from the array above; not user input.
		err := exec.CommandContext(ctx, "modprobe", mod).Run() //nolint:gosec // mod is a compile-time constant
		cancel()
		if err == nil {
			return nil
		}
	}
	return errL2tpFailedToLoadKernelModules
}

func moduleBuiltIn(name string) bool {
	_, err := os.Stat("/sys/module/" + name)
	return err == nil
}

// newSubsystemKernelWorker constructs a kernel worker ready for wiring
// into a reactor. Resolves the L2TP Generic Netlink family and builds the
// real kernelOps. When genl resolution fails it returns a nil worker and the
// failure: kernel integration stays disabled for this reactor while userspace
// control still works, and the subsystem keeps the error so the l2tp health
// row reports the dead data plane (checkHealth).
//
// Caller MUST call SetKernelWorker on the reactor before Start(), then
// Start() the worker after the reactor has its channels wired.
func newSubsystemKernelWorker(errCh chan<- kernelSetupFailed, successCh chan<- kernelSetupSucceeded, logger *slog.Logger) (*kernelWorker, error) {
	genl, err := resolveGenlFamily()
	if err != nil {
		return nil, fmt.Errorf("resolve the l2tp generic netlink family: %w", err)
	}
	return newKernelWorker(newKernelOps(genl), errCh, successCh, logger), nil
}
