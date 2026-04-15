// Design: docs/research/l2tpv2-ze-integration.md -- kernel subsystem design
// Related: kernel_event.go -- event types consumed by the worker
// Related: genl_linux.go -- Generic Netlink message construction
// Related: pppox_linux.go -- PPPoL2TP socket and /dev/ppp operations

//go:build linux

package l2tp

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"golang.org/x/sys/unix"
)

// kernelOps holds function pointers for the actual kernel syscalls.
// Tests inject fakes via the struct fields. Production uses newKernelOps().
type kernelOps struct {
	// Generic Netlink operations.
	tunnelCreate  func(localTID, remoteTID uint16, socketFD int) error
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
			unix.Close(pppoxFD)
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set LNS mode: %w", serr)
		}
	}
	if ev.sequencing {
		if serr := pppoxSetSendSeq(pppoxFD, true); serr != nil {
			unix.Close(pppoxFD)
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set send seq: %w", serr)
		}
		if serr := pppoxSetRecvSeq(pppoxFD, true); serr != nil {
			unix.Close(pppoxFD)
			return pppSessionFDs{}, fmt.Errorf("l2tp: pppox set recv seq: %w", serr)
		}
	}

	chanFD, unitFD, unitNum, err := devPPPSetup(pppoxFD)
	if err != nil {
		unix.Close(pppoxFD)
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

	eventCh chan any // receives kernelSetupEvent and kernelTeardownEvent
	errCh   chan<- kernelSetupFailed

	mu       sync.Mutex
	stopped  bool
	tunnels  map[uint16]*kernelTunnelState
	sessions map[sessionKey]*pppSessionFDs

	stop chan struct{}
	wg   sync.WaitGroup
}

// newKernelWorker constructs a kernel worker. The errCh is used to
// report setup failures back to the reactor. The worker does not own
// errCh; the reactor creates and reads it.
func newKernelWorker(ops kernelOps, errCh chan<- kernelSetupFailed, logger *slog.Logger) *kernelWorker {
	return &kernelWorker{
		ops:      ops,
		logger:   logger,
		eventCh:  make(chan any, 64),
		errCh:    errCh,
		tunnels:  make(map[uint16]*kernelTunnelState),
		sessions: make(map[sessionKey]*pppSessionFDs),
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
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	w.stopped = true
	w.mu.Unlock()

	close(w.stop)
	w.wg.Wait()
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
		if err := w.ops.tunnelDelete(tid); err != nil {
			w.logger.Warn("l2tp: kernel tunnel delete on shutdown",
				"tunnel-id", tid, "error", err.Error())
		}
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
		if err := w.ops.tunnelCreate(ev.localTID, ev.remoteTID, ev.socketFD); err != nil {
			w.logger.Error("l2tp: kernel tunnel create failed",
				"local-tid", ev.localTID, "error", err.Error())
			w.reportError(ev.localTID, ev.localSID, err)
			return
		}
		ts = &kernelTunnelState{localTID: ev.localTID}
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
		if derr := w.ops.sessionDelete(ev.localTID, ev.localSID); derr != nil {
			w.logger.Warn("l2tp: rollback session delete", "error", derr.Error())
		}
		ts.sessionCount--
		w.cleanupTunnelIfNew(ev.localTID, tunnelExisted)
		w.reportError(ev.localTID, ev.localSID, err)
		return
	}

	key := sessionKey{tunnelID: ev.localTID, sessionID: ev.localSID}
	w.sessions[key] = &fds

	w.logger.Info("l2tp: kernel session established",
		"tunnel-id", ev.localTID, "session-id", ev.localSID,
		"ppp-unit", fds.unitNum)
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
			if err := w.ops.tunnelDelete(ev.localTID); err != nil {
				w.logger.Warn("l2tp: kernel tunnel delete failed",
					"tunnel-id", ev.localTID, "error", err.Error())
			} else {
				w.logger.Info("l2tp: kernel tunnel deleted (last session)",
					"tunnel-id", ev.localTID)
			}
			delete(w.tunnels, ev.localTID)
		}
	}
}

// teardownSessionFDsLocked closes all fds and deletes the kernel session.
// RFC 2661 Section 24.25: strict reverse order.
// Caller MUST hold w.mu.
func (w *kernelWorker) teardownSessionFDsLocked(key sessionKey, fds *pppSessionFDs) {
	// Reverse order: unit fd, channel fd, pppox socket, then genl delete.
	if fds.unitFD >= 0 {
		w.ops.closeFD(fds.unitFD)
	}
	if fds.chanFD >= 0 {
		w.ops.closeFD(fds.chanFD)
	}
	if fds.pppoxFD >= 0 {
		w.ops.closeFD(fds.pppoxFD)
	}
	if err := w.ops.sessionDelete(key.tunnelID, key.sessionID); err != nil {
		w.logger.Warn("l2tp: kernel session delete failed",
			"tunnel-id", key.tunnelID, "session-id", key.sessionID,
			"error", err.Error())
	}
	delete(w.sessions, key)
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
		if err := w.ops.tunnelDelete(localTID); err != nil {
			w.logger.Warn("l2tp: rollback tunnel delete", "error", err.Error())
		}
		delete(w.tunnels, localTID)
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
func probeKernelModules() error {
	if err := exec.Command("modprobe", "l2tp_ppp").Run(); err == nil {
		return nil
	}
	if err := exec.Command("modprobe", "pppol2tp").Run(); err == nil {
		return nil
	}
	return fmt.Errorf("l2tp: failed to load kernel modules (tried l2tp_ppp, pppol2tp)")
}
