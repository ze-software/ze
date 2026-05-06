// Design: docs/research/l2tpv2-ze-integration.md -- kernel subsystem design
// Related: kernel_event.go -- event types consumed by the worker
// Related: genl_linux.go -- Generic Netlink message construction
// Related: pppox_linux.go -- PPPoL2TP socket and /dev/ppp operations

//go:build linux

package l2tp

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

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
	pppoxFD, err := pppoxCreate(ev.socketFD, ev.peerAddr,
		ev.localTID, ev.localSID, ev.remoteTID, ev.remoteSID)
	if err != nil {
		return pppSessionFDs{}, err
	}

	if ev.lnsMode {
		if serr := pppoxSetLNSMode(pppoxFD, true); serr != nil {
			unix.Close(pppoxFD) //nolint:errcheck // rollback path; primary error is serr
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set LNS mode: %w", serr)
		}
	}
	if ev.sequencing {
		if serr := pppoxSetSendSeq(pppoxFD, true); serr != nil {
			unix.Close(pppoxFD) //nolint:errcheck // rollback path; primary error is serr
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set send seq: %w", serr)
		}
		if serr := pppoxSetRecvSeq(pppoxFD, true); serr != nil {
			unix.Close(pppoxFD) //nolint:errcheck // rollback path; primary error is serr
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set recv seq: %w", serr)
		}
	}

	chanFD, unitFD, unitNum, err := devPPPSetup(pppoxFD)
	if err != nil {
		unix.Close(pppoxFD) //nolint:errcheck // rollback path; primary error is err
		return pppSessionFDs{}, err
	}

	return pppSessionFDs{
		pppoxFD: pppoxFD,
		chanFD:  chanFD,
		unitFD:  unitFD,
		unitNum: unitNum,
	}, nil
}

// kernelTunnelState tracks kernel-side resources for one L2TP tunnel.
type kernelTunnelState struct {
	localTID     uint16
	connFD       int // dup'd connected socket fd; closed on tunnel delete
	sessionCount int // number of kernel sessions on this tunnel
}

// sessionKey uniquely identifies a kernel session.
type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
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

	eventCh   chan any // receives kernelSetupEvent and kernelTeardownEvent
	errCh     chan<- kernelSetupFailed
	successCh chan<- kernelSetupSucceeded

	// stopped is an atomic flag so SignalStop can close w.stop without
	// acquiring w.mu. Locking would deadlock when setupSession holds
	// w.mu across a blocked successCh send after the reactor exited
	// (reactor drained before the worker, no reader for successCh).
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

// Start launches the worker goroutine.
func (w *kernelWorker) Start() {
	w.stop = make(chan struct{})
	w.wg.Add(1)
	go w.run()
}

// Stop signals the worker to exit and waits. Idempotent.
func (w *kernelWorker) Stop() {
	w.SignalStop()
	w.wg.Wait()
}

// SignalStop closes the worker's stop channel without waiting for the
// run goroutine. Idempotent. Separated from Stop so the subsystem can
// break reportSuccess/reportError out of their channel-send selects
// BEFORE TeardownAll grabs w.mu -- otherwise a setupSession that was
// blocked on successCh (because the reactor already exited) would
// keep w.mu held indefinitely and deadlock TeardownAll.
//
// Caller MUST still call Stop (or Wait) to reap the goroutine.
func (w *kernelWorker) SignalStop() {
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

// TeardownAll cleans up all kernel resources. Called by the subsystem
// during Stop() before the reactor shuts down.
func (w *kernelWorker) TeardownAll() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Teardown sessions first (reverse order), then tunnels.
	for key, fds := range w.sessions {
		w.teardownSessionFDsLocked(key, fds)
	}
	for tid := range w.tunnels {
		w.deleteTunnelLocked(tid)
	}
	w.sessions = make(map[sessionKey]*pppSessionFDs)
	w.tunnels = make(map[uint16]*kernelTunnelState)
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
	defer w.mu.Unlock()

	// Step 1: ensure kernel tunnel exists.
	ts, tunnelExisted := w.tunnels[ev.localTID]
	if !tunnelExisted {
		connFD, err := w.ops.tunnelCreate(ev.localTID, ev.remoteTID, ev.socketFD, ev.peerAddr)
		if err != nil {
			w.logger.Error("l2tp: kernel tunnel create failed",
				"local-tid", ev.localTID, "remote-tid", ev.remoteTID,
				"socket-fd", ev.socketFD, "error", err.Error())
			w.reportError(ev.localTID, ev.localSID, err)
			return
		}
		ts = &kernelTunnelState{localTID: ev.localTID, connFD: connFD}
		w.tunnels[ev.localTID] = ts
		w.logger.Info("l2tp: kernel tunnel created",
			"local-tid", ev.localTID, "remote-tid", ev.remoteTID)
	}

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
		w.reportError(ev.localTID, ev.localSID, err)
		return
	}
	ts.sessionCount++

	// Steps 3-4: PPPoL2TP socket + /dev/ppp setup.
	fds, err := w.ops.pppSetup(ev)
	if err != nil {
		w.logger.Error("l2tp: ppp setup failed",
			"tunnel-id", ev.localTID, "session-id", ev.localSID, "error", err.Error())
		// Rollback: delete kernel session, maybe delete tunnel.
		// Only decrement sessionCount when the kernel delete actually
		// succeeded. A failed delete means the kernel still holds the
		// session, so leaving the counter high keeps our Go-side view
		// conservative: cleanupTunnelIfNew then skips the tunnel delete
		// and we leak loudly rather than silently producing an
		// inconsistent "clean" state in Go while the kernel still has
		// state.
		if derr := w.ops.sessionDelete(ev.localTID, ev.localSID); derr != nil {
			w.logger.Error("l2tp: rollback session delete FAILED; kernel state leaked",
				"tunnel-id", ev.localTID, "session-id", ev.localSID, "error", derr.Error())
		} else {
			ts.sessionCount--
			w.cleanupTunnelIfNew(ev.localTID, tunnelExisted)
		}
		w.reportError(ev.localTID, ev.localSID, err)
		return
	}

	key := sessionKey{tunnelID: ev.localTID, sessionID: ev.localSID}
	w.sessions[key] = &fds

	w.logger.Info("l2tp: kernel session established",
		"tunnel-id", ev.localTID, "session-id", ev.localSID,
		"ppp-unit", fds.unitNum)

	w.reportSuccess(ev, fds)
}

// reportSuccess sends a setup success to the reactor via successCh.
// successCh may be nil in tests that exercise teardown / failure paths
// only; in that case the success event is dropped silently.
func (w *kernelWorker) reportSuccess(ev kernelSetupEvent, fds pppSessionFDs) {
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
		proxyInitialRecvLCPConfReq: ev.proxyInitialRecvLCPConfReq,
		proxyLastSentLCPConfReq:    ev.proxyLastSentLCPConfReq,
		proxyLastRecvLCPConfReq:    ev.proxyLastRecvLCPConfReq,
	}:
	case <-w.stop:
	}
}

// teardownSession destroys kernel resources for a session.
func (w *kernelWorker) teardownSession(ev kernelTeardownEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := sessionKey{tunnelID: ev.localTID, sessionID: ev.localSID}
	fds, ok := w.sessions[key]
	if !ok {
		// Session might not have kernel resources (setup could have
		// failed or not yet completed).
		return
	}

	w.teardownSessionFDsLocked(key, fds)

	// If this was the last session, delete the kernel tunnel.
	ts, ok := w.tunnels[ev.localTID]
	if ok {
		ts.sessionCount--
		if ts.sessionCount <= 0 {
			w.deleteTunnelLocked(ev.localTID)
		}
	}
}

// teardownSessionFDsLocked closes all fds and deletes the kernel session.
// RFC 2661 Section 24.25: strict reverse order.
// Caller MUST hold w.mu.
func (w *kernelWorker) teardownSessionFDsLocked(key sessionKey, fds *pppSessionFDs) {
	// Reverse order: unit fd, channel fd, pppox socket, then genl delete.
	// Close failures on teardown are logged warnings at most; they do not
	// block the rest of the cleanup sequence.
	if fds.unitFD >= 0 {
		if err := w.ops.closeFD(fds.unitFD); err != nil {
			w.logger.Warn("l2tp: close unit fd", "fd", fds.unitFD, "error", err.Error())
		}
	}
	if fds.chanFD >= 0 {
		if err := w.ops.closeFD(fds.chanFD); err != nil {
			w.logger.Warn("l2tp: close chan fd", "fd", fds.chanFD, "error", err.Error())
		}
	}
	if fds.pppoxFD >= 0 {
		if err := w.ops.closeFD(fds.pppoxFD); err != nil {
			w.logger.Warn("l2tp: close pppox fd", "fd", fds.pppoxFD, "error", err.Error())
		}
	}
	if err := w.ops.sessionDelete(key.tunnelID, key.sessionID); err != nil {
		w.logger.Warn("l2tp: kernel session delete failed",
			"tunnel-id", key.tunnelID, "session-id", key.sessionID,
			"error", err.Error())
	}
	delete(w.sessions, key)
}

// deleteTunnelLocked deletes the kernel tunnel and closes its connected socket fd.
func (w *kernelWorker) deleteTunnelLocked(tid uint16) {
	ts, ok := w.tunnels[tid]
	if !ok {
		return
	}
	if err := w.ops.tunnelDelete(tid); err != nil {
		w.logger.Warn("l2tp: kernel tunnel delete failed",
			"tunnel-id", tid, "error", err.Error())
	} else {
		w.logger.Info("l2tp: kernel tunnel deleted", "tunnel-id", tid)
	}
	if ts.connFD >= 0 {
		w.ops.closeFD(ts.connFD) //nolint:errcheck // best-effort cleanup
	}
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
func (w *kernelWorker) reportError(localTID, localSID uint16, err error) {
	select {
	case w.errCh <- kernelSetupFailed{
		localTID: localTID,
		localSID: localSID,
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
	return fmt.Errorf("l2tp: failed to load kernel modules (tried l2tp_ppp, pppol2tp)")
}

func moduleBuiltIn(name string) bool {
	_, err := os.Stat("/sys/module/" + name)
	return err == nil
}

// newSubsystemKernelWorker constructs a kernel worker ready for wiring
// into a reactor. Resolves the L2TP Generic Netlink family and builds the
// real kernelOps. Returns nil if genl resolution fails (kernel integration
// stays disabled for this reactor; userspace control still works).
//
// Caller MUST call SetKernelWorker on the reactor before Start(), then
// Start() the worker after the reactor has its channels wired.
func newSubsystemKernelWorker(errCh chan<- kernelSetupFailed, successCh chan<- kernelSetupSucceeded, logger *slog.Logger) *kernelWorker {
	genl, err := resolveGenlFamily()
	if err != nil {
		logger.Warn("l2tp: genl family resolve failed; kernel integration disabled",
			"error", err.Error())
		return nil
	}
	return newKernelWorker(newKernelOps(genl), errCh, successCh, logger)
}
