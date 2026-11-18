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

	// Preparation validates the actual role before any subscriber allocation.
	// Its returned plan owns setup; tests substitute this complete operation.
	pppSetup  func(ev kernelSetupEvent) (kernelPPPSetup, error)
	pppConfig func(*kernelPPPOwners, pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error)
	closePPP  func(*kernelPPPOwners, *os.File, [3]int) [3]error

	// File descriptor close.
	closeFD func(fd int) error

	// Module probing.
	probeModules func() error
}

// newKernelOps returns a kernelOps with real syscall implementations.
func newKernelOps(genl *genlConn, backend pppkernel.InterfaceOperations) kernelOps {
	return kernelOps{
		tunnelCreate:  genl.tunnelCreate,
		tunnelDelete:  genl.tunnelDelete,
		sessionCreate: genl.sessionCreate,
		sessionDelete: genl.sessionDelete,
		pppSetup: func(ev kernelSetupEvent) (kernelPPPSetup, error) {
			return prepareKernelPPPSetup(ev, backend)
		},
		pppConfig: originalKernelPPPConfig,
		closePPP: func(owners *kernelPPPOwners, transport *os.File, labels [3]int) [3]error {
			return closeOriginalKernelPPP(owners, transport, (*os.File).Close)
		},
		closeFD:      unix.Close,
		probeModules: probeKernelModules,
	}
}

// pppSetupReal performs the full PPPoL2TP socket + /dev/ppp setup sequence.
func pppSetupReal(ev kernelSetupEvent, creator *pppkernel.UnitCreator) (pppSessionFDs, error) {
	return pppSetupWith(ev, creator, kernelPPPSetupOperations{
		create:   openKernelPPPTransport,
		setup:    pppkernel.DevPPPSetup,
		channel:  pppkernel.CreatePPPChannel,
		snapshot: snapshotOriginalKernelPPP,
		bridge:   bridgeOriginalKernelPPP,
		close: func(owners *kernelPPPOwners, transport *os.File, labels [3]int) [3]error {
			return closeOriginalKernelPPP(owners, transport, (*os.File).Close)
		},
	})
}

// pppSetupWith supplies complete private operation ports for fault injection.
// Production ports retain the actual files; a test's logical labels supply no
// public SessionConfig grant. Every returned partial receipt remains rooted.
func pppSetupWith(ev kernelSetupEvent, creator *pppkernel.UnitCreator, ops kernelPPPSetupOperations) (owned pppSessionFDs, err error) {
	owned.pppoxFD, owned.chanFD, owned.unitFD, owned.unitNum = -1, -1, -1, -1
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, kernelPPPFailure(failure))
		}
		if err == nil {
			return
		}
		labels := [3]int{owned.unitFD, owned.chanFD, owned.pppoxFD}
		owners := owned.ppp
		transport := owned.transport
		owned.retiredTransport = transport
		owned.transport = nil
		owned.ppp, owned.retiredPPP = nil, owners
		owned.unitFD, owned.chanFD, owned.pppoxFD = -1, -1, -1
		failures := ops.close(owners, transport, labels)
		if kernelPPPBridgePending(owners) {
			owned.ppp, owned.transport = owners, transport
			owned.retiredPPP, owned.retiredTransport = nil, nil
			owned.unitFD, owned.chanFD, owned.pppoxFD = labels[0], labels[1], labels[2]
			owned.bridgeErr = errors.Join(failures[:]...)
			err = errors.Join(err, owned.bridgeErr)
			return
		}
		err = pppkernel.RecordSetupCleanup(err, errors.Join(failures[:]...))
	}()
	owned.transport, owned.pppoxFD, err = ops.create(ev.socketFD, ev.peerAddr, ev.localTID, ev.localSID, ev.remoteTID, ev.remoteSID)
	if err != nil {
		return owned, err
	}
	if ev.lnsMode {
		if err := withKernelPPPTransport(owned.transport, func(fd int) error { return pppoxSetLNSMode(fd, true) }); err != nil {
			return owned, fmt.Errorf("l2tp: pppox set LNS mode: %w", err)
		}
	}
	if ev.sequencing {
		if err := withKernelPPPTransport(owned.transport, func(fd int) error { return pppoxSetSendSeq(fd, true) }); err != nil {
			return owned, fmt.Errorf("l2tp: pppox set send seq: %w", err)
		}
		if err := withKernelPPPTransport(owned.transport, func(fd int) error { return pppoxSetRecvSeq(fd, true) }); err != nil {
			return owned, fmt.Errorf("l2tp: pppox set recv seq: %w", err)
		}
	}
	channel, unit, setupErr := prepareAndSetupPPP(ev, owned.transport,
		func(file *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
			if !ev.lnsMode {
				if ev.channel != nil {
					channel, err := ops.channel(file)
					return channel, nil, err
				}
			}
			return ops.setup(creator, file, name)
		})
	owned.ppp = &kernelPPPOwners{channel: channel, unit: unit}
	if setupErr != nil {
		return owned, setupErr
	}
	if unit != nil {
		config, err := ops.snapshot(owned.ppp)
		if err != nil {
			return owned, err
		}
		owned.chanFD, owned.unitFD, owned.unitNum = config.ChanFD, config.UnitFD, config.UnitNum
	}
	if ev.relay != nil {
		if err := quiesceRelaySubscriber(ev.relay); err != nil {
			return owned, err
		}
	}
	owned.bridged, err = ops.bridge(ev, owned.ppp)
	if ev.relay != nil {
		if err == nil {
			if !ev.relay.markBridged(ev.localTID, ev.localSID) {
				return owned, errRelayCallCancelled
			}
		}
		if ev.relay.realm != nil {
			// quiesceRelaySubscriber validated the immutable realm handoff.
			handoff := ev.relay.handoff.(callsink.RealmHandoff) //nolint:errcheck,forcetypeassert // validated before native bridge
			countRealmBridge(err)
			handoff.Bridged(err)
		}
	}
	if err != nil {
		return owned, fmt.Errorf("l2tp: LAC channel bridge: %w", err)
	}
	return owned, nil
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
	// The original admission is verified before capability refusal or any
	// allocation. A bridge preflight deliberately needs no local unit creator.
	setup, err := w.ops.pppSetup(ev)
	if err != nil {
		setupError = err
		return
	}
	if setup == nil {
		setupError = errors.New("l2tp: kernel setup returned no prepared operation")
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
	fds, err := setup()
	owned.pppoxFD, owned.chanFD, owned.unitFD = fds.pppoxFD, fds.chanFD, fds.unitFD
	owned.transport = fds.transport
	owned.retiredTransport = fds.retiredTransport
	owned.bridgeErr = fds.bridgeErr
	owned.unitNum, owned.bridged = fds.unitNum, fds.bridged
	owned.ppp, owned.retiredPPP = fds.ppp, fds.retiredPPP
	if err != nil {
		w.logger.Error("l2tp: ppp setup failed",
			"tunnel-id", ev.localTID, "session-id", ev.localSID, "error", err.Error())
		owned.closeErr = pppkernel.SetupCleanupFailure(err)
		// A pending bridge has not consumed its transport or channel files. Its
		// retryable cleanup cause belongs to that exact handle, not closeErr.
		if kernelPPPBridgePending(owned.ppp) {
			owned.closeErr = nil
		}
		w.teardownSessionFDsLocked(key, owned)
		w.cleanupTunnelIfNew(ev.localTID, tunnelExisted)
		setupError = err
		return
	}

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

// preparePPPTransfer snapshots only the exact live original pair. A stale
// receipt is an unowned no-op and MUST NOT report another admission's failure.
func (w *kernelWorker) preparePPPTransfer(event kernelSetupSucceeded, backend pppkernel.InterfaceOperations) (kernelPPPTransfer, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fds, current := w.currentPPPTransferOwnerLocked(event)
	if !current {
		return kernelPPPTransfer{}, false, nil
	}
	config, err := w.ops.pppConfig(fds.ppp, backend)
	if err != nil {
		return kernelPPPTransfer{}, true, err
	}
	return kernelPPPTransfer{owner: fds, resources: fds.ppp, config: config}, true, nil
}

// takePPPConfig transfers the same original pair once. Callers MUST pass the
// preparation from this worker and stop touching its files after NewSession.
func (w *kernelWorker) takePPPConfig(event kernelSetupSucceeded, transfer kernelPPPTransfer) (pppkernel.SessionConfig, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fds, current := w.currentPPPTransferOwnerLocked(event)
	if !current {
		return pppkernel.SessionConfig{}, false
	}
	if transfer.owner != fds {
		return pppkernel.SessionConfig{}, false
	}
	if transfer.resources != fds.ppp {
		return pppkernel.SessionConfig{}, false
	}
	fds.ppp = nil
	fds.chanFD, fds.unitFD = -1, -1
	return transfer.config, true
}

func (w *kernelWorker) currentPPPTransferOwnerLocked(event kernelSetupSucceeded) (*pppSessionFDs, bool) {
	if event.session == nil {
		return nil, false
	}
	if event.session.removed.Load() {
		return nil, false
	}
	fds := w.sessions[sessionKey{event.localTID, event.localSID}]
	if fds == nil {
		return nil, false
	}
	if fds.session != event.session {
		return nil, false
	}
	if fds != event.owner {
		return nil, false
	}
	if fds.bridged {
		return nil, false
	}
	if fds.ppp == nil {
		return nil, false
	}
	return fds, true
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
		errs = append(errs, owned.deleteErr, owned.closeErr, owned.bridgeErr)
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
		return errors.Join(ErrSessionCleanupPending, owned.deleteErr, owned.closeErr, owned.bridgeErr)
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
	// Close owns exact Unbridge followed by both claim closes. Even a failed
	// Bridge may need that cleanup, so handle presence, not bridged, decides.
	// Reverse order: unit fd, channel fd, pppox socket, then genl delete.
	// A failed Close does not block the remaining cleanup operations, but its
	// retained uncertainty prevents this original owner from being retired.
	owned := [3]int{fds.unitFD, fds.chanFD, fds.pppoxFD}
	resources := fds.ppp
	transport := fds.transport
	if transport != nil {
		fds.retiredTransport = transport
	}
	fds.transport = nil
	fds.ppp = nil
	if resources != nil {
		fds.retiredPPP = resources
	}
	// Consume recyclable numbers before calling Close. A failed Close is
	// uncertainty, never permission to retry that number after reuse.
	fds.unitFD, fds.chanFD, fds.pppoxFD = -1, -1, -1
	failures := w.ops.closePPP(resources, transport, owned)
	if kernelPPPBridgePending(resources) {
		fds.ppp, fds.transport = resources, transport
		fds.retiredPPP, fds.retiredTransport = nil, nil
		fds.unitFD, fds.chanFD, fds.pppoxFD = owned[0], owned[1], owned[2]
		fds.bridgeErr = errors.Join(failures[:]...)
		w.reportKernelCleanupFailure(kernelCleanupChannelRelease,
			key.tunnelID, key.sessionID, kernelCleanupBirth(fds), owned[1], fds.bridgeErr)
		return
	}
	fds.bridged, fds.bridgeErr = false, nil
	clearKernelBridgeWarning(key, fds, owned[1])
	for index, err := range failures {
		if err != nil {
			w.reportKernelCleanupFailure(kernelCleanupSessionClose,
				key.tunnelID, key.sessionID, kernelCleanupBirth(fds), owned[index], err)
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
func newSubsystemKernelWorker(errCh chan<- kernelSetupFailed, successCh chan<- kernelSetupSucceeded, logger *slog.Logger, prepare kernelBackendPreparation) (*kernelWorker, error) {
	genl, err := resolveGenlFamily()
	if err != nil {
		return nil, fmt.Errorf("resolve the l2tp generic netlink family: %w", err)
	}
	if prepare == nil {
		return nil, &kernelBackendPreparationFailure{cause: errors.New("l2tp: native worker has no backend preparation")}
	}
	backend, err := prepare()
	if err != nil {
		return nil, &kernelBackendPreparationFailure{cause: err}
	}
	return newKernelWorker(newKernelOps(genl, backend), errCh, successCh, logger), nil
}
