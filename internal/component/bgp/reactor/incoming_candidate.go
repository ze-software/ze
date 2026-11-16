// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming Session ownership.
// Related: peer_connection.go -- bounded admission and collision reservations.
// Related: peer_run.go -- publication after the incumbent epoch has ended.
// RFC 4271 Sections 8.2.1 and 6.8 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// inboundConnection is one of the four ownership receipts the Peer loop can
// consume. Nil is an empty slot. A receipt MUST be consumed once or discarded.
type inboundConnection interface {
	connection() net.Conn
	inboundReceipt()
}

type incomingSocket struct{ conn net.Conn }

func (v *incomingSocket) connection() net.Conn { return v.conn }
func (*incomingSocket) inboundReceipt()        {}

// bufferedIncoming is an externally read OPEN on a socket not yet installed in
// a Session. The producer MUST stop using its complete original wire buffer.
type bufferedIncoming struct {
	conn net.Conn
	open *message.Open
	wire []byte
}

func (v *bufferedIncoming) connection() net.Conn { return v.conn }
func (*bufferedIncoming) inboundReceipt()        {}

// candidateOpen transfers the same incoming Session and a validated first OPEN
// that its next owner MUST advance once. It owns its complete original wire.
type candidateOpen struct {
	candidate *incomingCandidate
	*openAdmission
	wire []byte
}

func (v *candidateOpen) connection() net.Conn { return v.candidate.conn }
func (*candidateOpen) inboundReceipt()        {}

// candidateReady transfers an incoming Session that has processed its OPEN.
// Its actual FSM state, including any BFD wait, is retained rather than reset.
type candidateReady struct {
	candidate *incomingCandidate
	remoteID  uint32
}

func (v *candidateReady) connection() net.Conn { return v.candidate.conn }
func (*candidateReady) inboundReceipt()        {}

func inboundConn(inbound inboundConnection) net.Conn {
	if inbound == nil {
		return nil
	}
	return inbound.connection()
}

func discardInbound(inbound inboundConnection) {
	if candidate := inboundCandidate(inbound); candidate != nil {
		candidate.retire()
		return
	}
	if conn := inboundConn(inbound); conn != nil {
		closeConnQuietly(conn)
	}
}

func inboundCandidate(inbound inboundConnection) *incomingCandidate {
	switch v := inbound.(type) {
	case nil, *incomingSocket, *bufferedIncoming:
		return nil
	case *candidateOpen:
		return v.candidate
	case *candidateReady:
		return v.candidate
	}
	return nil
}

// pendingConnection distinguishes admission, a running candidate and retirement.
// Every variant is private and occupies the same single pending slot.
type pendingConnection interface {
	connection() net.Conn
	pendingReceipt()
}

type pendingSocket struct {
	conn    net.Conn
	started bool // guarded by Peer.mu; only beginIncoming may start its worker
}

func (v *pendingSocket) connection() net.Conn { return v.conn }
func (*pendingSocket) pendingReceipt()        {}

// incomingCandidate owns a Session until a typed inbound receipt transfers it.
// The immutable conn is its admission identity, even after Session.Conn becomes
// nil. One reader and one cancellation bridge serve this connection. Promotion
// MUST join the reader and bridge before installing publication callbacks;
// retirement MUST close the socket, stop timers and join the bridge instead.
type incomingCandidate struct {
	peer        *Peer
	conn        net.Conn
	session     *Session
	readDone    chan struct{}
	handoffDone chan struct{}
	bridgeStop  chan struct{}
	bridgeDone  chan struct{}
	progress    chan struct{}
	stopBridge  sync.Once
	retireOnce  sync.Once
}

func (v *incomingCandidate) connection() net.Conn { return v.conn }
func (*incomingCandidate) pendingReceipt()        {}

// closingIncoming retains admission after the protocol decision. Its owner
// MUST complete its refusal, retire and join the candidate, then clearIncoming.
// It no longer reports an active pending exchange, but admits no new Session.
type closingIncoming struct{ candidate *incomingCandidate }

func (v *closingIncoming) connection() net.Conn { return v.candidate.conn }
func (*closingIncoming) pendingReceipt()        {}

func (p *Peer) beginIncoming(conn net.Conn) (*pendingSocket, context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.incomingClosed {
		return nil, nil
	}
	if p.pending == nil {
		p.pending = &pendingSocket{conn: conn}
	}
	admission, ok := p.pending.(*pendingSocket)
	if !ok {
		return nil, nil
	}
	if admission.conn != conn {
		return nil, nil
	}
	if admission.started {
		return nil, nil
	}
	admission.started = true
	p.incomingWG.Add(1)
	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return admission, ctx
}

func (p *Peer) clearIncoming(conn net.Conn) {
	p.mu.Lock()
	if p.pending != nil {
		if p.pending.connection() == conn {
			p.pending = nil
		}
	}
	p.mu.Unlock()
}

// beginIncomingRetirement ends the pending exchange while retaining its one
// admission slot. The caller MUST retire/join this candidate before clearing
// the receipt; freeing it before a blocking refusal would admit more Sessions.
func (p *Peer) beginIncomingRetirement(candidate *incomingCandidate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == candidate {
		p.pending = &closingIncoming{candidate: candidate}
	}
}

func (r *Reactor) handlePendingCollision(peer *Peer, conn net.Conn) {
	admission, ctx := peer.beginIncoming(conn)
	if admission == nil {
		closeConnQuietly(conn)
		return
	}
	defer peer.incomingWG.Done()
	var candidate *incomingCandidate
	defer bgpPanics().read.Contain(peer.describeConnect, func(*taskguard.Panic) {
		if candidate != nil {
			candidate.session.closeConn()
		}
	})
	// Factory failures still own the admitted socket. Once constructed, the
	// candidate's cleanup below takes over the socket and all its workers.
	defer func() {
		if candidate == nil {
			peer.clearIncoming(conn)
			closeConnQuietly(conn)
		}
	}()
	session := peer.newIncomingSession()
	candidate = &incomingCandidate{
		peer: peer, conn: conn, session: session,
		readDone: make(chan struct{}), handoffDone: make(chan struct{}),
		bridgeStop: make(chan struct{}), bridgeDone: make(chan struct{}),
		progress: make(chan struct{}, 1),
	}
	transferred := false
	defer close(candidate.readDone)
	defer func() {
		if !transferred {
			defer close(candidate.handoffDone)
			peer.beginIncomingRetirement(candidate)
			candidate.retire()
			peer.clearIncoming(conn)
		}
	}()
	session.beforeNotification = func() { peer.beginIncomingRetirement(candidate) }
	session.onNotificationFailure = func(err error) bool {
		peer.mu.RLock()
		selected := peer.session == session
		peer.mu.RUnlock()
		if selected {
			return false
		}
		reportPendingRefusalFailure(conn, "write NOTIFICATION", err)
		return true
	}

	go candidate.watchFailure(ctx)
	candidate.session.fsm.SetCallback(func(_, _ fsm.State, _ fsm.Event) {
		select {
		case candidate.progress <- struct{}{}:
		default: // One wakeup already represents the latest FSM state.
		}
	})

	// The first-OPEN read budget is independent of the Session's write/timer
	// clock. An expired pending read must not corrupt the local OPEN budget.
	hold := peer.Settings().ReceiveHoldTime
	if hold == 0 {
		hold = 90 * time.Second
	}
	if err := conn.SetReadDeadline(r.clock.Now().Add(hold)); err != nil {
		return
	}
	// RFC 4271 Section 8.2.1: "For each incoming connection, a state machine MUST be instantiated."
	if err := candidate.session.startDamped(); err != nil {
		return
	}
	if err := candidate.session.Accept(conn); err != nil {
		return
	}
	peer.mu.Lock()
	if peer.pending != admission {
		peer.mu.Unlock()
		return
	}
	peer.pending = candidate
	peer.mu.Unlock()

	reader := candidate.session.bufReader
	initial, err := candidate.readOpen(reader)
	if err != nil {
		return
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return
	}
	open := initial.open
	decision := candidate.resolve(initial, open.BGPIdentifier)
	if decision == incomingTransferred {
		transferred = true
		return
	}
	if decision == incomingRejected {
		_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
		return
	}

	// RFC 4271 Section 6.8 does not arbitrate an OpenSent incumbent whose
	// remote Identifier is unknown outside the protocol. As BIRD does, keep
	// both independent FSMs while this incoming handshake progresses.
	if err := initial.openAdmission.advance(); err != nil {
		return
	}
	ready := &candidateReady{candidate: candidate, remoteID: open.BGPIdentifier}
	for {
		decision = candidate.resolve(ready, open.BGPIdentifier)
		if decision == incomingTransferred {
			transferred = true
			return
		}
		if decision == incomingRejected {
			_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
			return
		}
		if candidate.session.receivedType == msgtype.TypeKEEPALIVE {
			// A real KEEPALIVE can be held at the BFD hold-down boundary.
			// Stop reading before the next UPDATE; the winning Session's
			// original buffered reader must deliver it after promotion.
			select {
			case <-candidate.progress:
			case <-candidate.bridgeDone:
				return
			case <-ctx.Done():
				return
			}
			continue
		}
		if err := candidate.session.readAndProcessMessage(conn, reader); err != nil {
			return
		}
	}
}

// readOpen dispatches every non-OPEN through the ordinary Session FSM. A
// complete OPEN is admitted by the same semantic constructor before collision;
// its retained wire owns a copy because the normal pool handle returns here.
func (c *incomingCandidate) readOpen(reader *bufio.Reader) (*candidateOpen, error) {
	for {
		frame, err := c.session.readFrame(c.conn, reader)
		if err != nil {
			return nil, err
		}
		if frame.header.Type != msgtype.TypeOPEN {
			if err := c.session.dispatchFrame(frame); err != nil {
				return nil, err
			}
			continue
		}
		return c.admitFrame(frame)
	}
}

func (c *incomingCandidate) admitFrame(frame *receivedFrame) (*candidateOpen, error) {
	s := c.session
	kept := false
	defer func() {
		if !kept {
			s.returnReadBuffer(frame.buffer)
		}
	}()
	if err := s.beginAdmission(); err != nil {
		return nil, err
	}
	defer s.endAdmission()
	wire := bytes.Clone(frame.wire())
	s.receivedType = msgtype.TypeOPEN
	s.mu.RLock()
	ctxID := s.recvCtxID
	s.mu.RUnlock()
	err, owned := s.deliverReceivedMessage(msgtype.TypeOPEN, frame.body(), nil, ctxID, frame.buffer)
	kept = owned
	if err != nil {
		return nil, err
	}
	admitted, err := s.admitOpen(wire[message.HeaderLen:])
	if err != nil {
		return nil, err
	}
	return &candidateOpen{candidate: c, openAdmission: admitted, wire: wire}, nil
}

// resolve rechecks liveness under the candidate OPEN owner after validators
// return. A slow plugin cannot promote a socket its timer already retired.
func (c *incomingCandidate) resolve(arrival inboundConnection, remoteID uint32) incomingDecision {
	c.session.openMu.Lock()
	defer c.session.unlockOpen()
	if c.session.tearingDown.Load() {
		return incomingRejected
	}
	if c.session.socketClosedIs(c.conn) {
		return incomingRejected
	}
	if c.session.Conn() != c.conn {
		return incomingRejected
	}
	// RFC 4271 Section 8.2.2, Event10: original expiry precedes arbitration.
	if c.session.rejectExpiredHold() {
		return incomingRejected
	}
	decision := c.peer.resolveIncoming(c, arrival, remoteID)
	if decision == incomingRejected {
		// Peer.mu may have remained held past the earlier budget check.
		// Its commit-side refusal left expiry to this openMu owner, outside
		// the Peer lock, where the terminal writer may report it safely.
		c.session.rejectExpiredHold()
	}
	return decision
}

func (c *incomingCandidate) watchFailure(ctx context.Context) {
	defer close(c.bridgeDone)
	defer bgpPanics().cancel.Contain(c.session.describeRead, func(*taskguard.Panic) { c.session.closeConn() })
	var reason error
	select {
	case <-ctx.Done():
		reason = ctx.Err()
	case reason = <-c.session.errChan:
	case <-c.bridgeStop:
		return
	}
	c.session.setCloseReason(reason)
	c.session.closeConn()
}

func (c *incomingCandidate) joinBridge() {
	c.stopBridge.Do(func() { close(c.bridgeStop) })
	<-c.bridgeDone
}

func (c *incomingCandidate) retire() {
	c.retireOnce.Do(func() {
		c.session.closeConn()
		c.joinBridge()
		// A read EOF/deadline on an unselected connection has no Run loop
		// to deliver Event 18. Retire its independent FSM and retry history.
		c.session.handleConnectionClose()
		c.session.stopSessionTimers()
		c.peer.releaseSessionRouterIDClaim(c.session)
		c.peer.stopCapture(c.session.captureWriter)
		close(c.session.done)
	})
}

// acquire joins the reader, collision close and cancellation bridge. Timers
// keep their original deadlines; no lock spans runOnce's deferred cleanup.
func (c *incomingCandidate) acquire() *Session {
	<-c.readDone
	<-c.handoffDone
	c.joinBridge()
	select {
	case <-c.session.done:
		return nil
	default: // The receipt still owns a live Session.
		return c.session
	}
}

// retireUnpublishedSession closes a candidate refused at the selected epoch's
// publication gate, or the freshly constructed ordinary Session. Neither can
// reach Run's finalizer on this exit.
func (p *Peer) retireUnpublishedSession(candidate *incomingCandidate, session *Session) {
	if candidate != nil {
		candidate.retire()
		return
	}
	session.closeConn()
	session.stopSessionTimers()
	p.stopCapture(session.captureWriter)
}

// adoptIncoming replaces only FSM publication, after the delivery worker and
// down finalizer exist. openMu orders the snapshot against all timer/BFD/OPEN
// transitions. The scoped defer releases it even if a publication panics.
func (s *Session) adoptIncoming(publish func(fsm.State, fsm.State, fsm.Event), leave *sessionLeave, ready bool) error {
	s.openMu.Lock()
	defer s.unlockOpen()
	state := s.State()
	if s.tearingDown.Load() {
		return ErrNotConnected
	}
	// RFC 4271 Section 8.2.2, Event10: joining old cleanup cannot renew it.
	if s.rejectExpiredHold() {
		return ErrHoldTimerExpired
	}
	if state == fsm.StateIdle {
		return ErrNotConnected
	}
	if s.Conn() == nil {
		return ErrNotConnected
	}
	s.selectNegotiationEpoch()
	s.fsm.SetCallback(publish)
	leave.selected = true
	if !ready {
		return nil
	}
	if s.Negotiated() != nil {
		// A processed OPEN may still be in a Connect/Active BFD wait.
		// Catch up the selected epoch's timer display without renegotiating.
		hold := s.timers.HoldTime()
		keepalive := s.timers.KeepaliveTime()
		if keepalive == 0 {
			keepalive = hold / 3
		}
		s.onNegotiated(uint32(hold/time.Second), uint32(keepalive/time.Second))
	}
	if state == fsm.StateOpenConfirm || state == fsm.StateEstablished {
		publish(fsm.StateOpenSent, fsm.StateOpenConfirm, fsm.EventBGPOpen)
	}
	if state == fsm.StateEstablished {
		publish(fsm.StateOpenConfirm, fsm.StateEstablished, fsm.EventKeepaliveMsg)
	}
	return nil
}

type incomingDecision uint8

const (
	incomingUnspecified incomingDecision = iota
	incomingRejected
	incomingContinues
	incomingTransferred
)

func (p *Peer) resolveIncoming(candidate *incomingCandidate, arrival inboundConnection, remoteID uint32) incomingDecision {
	var incumbent *Session
	var old inboundConnection
	decision := func() incomingDecision {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.pending != candidate {
			return incomingRejected
		}
		if p.stopping.Load() {
			return incomingRejected
		}
		if p.ctx != nil {
			if p.ctx.Err() != nil {
				return incomingRejected
			}
		}
		incumbent = p.session
		if incumbent == nil {
			return incomingRejected
		}
		accept, closeExisting := incumbent.detectCollision(remoteID)
		if incumbent.State() == fsm.StateOpenSent {
			if candidate.session.State() != fsm.StateEstablished {
				return incomingContinues
			}
			accept, closeExisting = true, true
		}
		if !accept {
			return incomingRejected
		}
		if !closeExisting {
			return incomingRejected
		}
		// RFC 4271 Section 8.2.2, Event10: reject at the actual publication
		// commit after waiting for Peer.mu and all collision-state reads.
		// The caller owns openMu and delivers expiry after this lock is free.
		if candidate.session.timers.HoldBudget().State == fsm.HoldBudgetExpired {
			return incomingRejected
		}
		p.pending = nil
		old = p.storeInboundLocked(arrival)
		p.collisionHandoff = true
		p.incomingWG.Add(1)
		return incomingTransferred
	}()
	if decision != incomingTransferred {
		return decision
	}
	if old != nil {
		discardInbound(old)
	}
	go p.finishIncomingHandoff(incumbent, candidate, remoteID)
	return incomingTransferred
}

func (p *Peer) finishIncomingHandoff(incumbent *Session, candidate *incomingCandidate, remoteID uint32) {
	defer p.incomingWG.Done()
	defer close(candidate.handoffDone)
	// The incoming FSM has already established without outside-Identifier
	// arbitration. Retire an incumbent still waiting for OPEN; if its own
	// OPEN won the race, re-run the existing OpenConfirm/Established policy.
	defer bgpPanics().connect.Contain(p.describeConnect, func(*taskguard.Panic) {
		incumbent.closeConn()
		candidate.session.closeConn()
	})
	retired := func() bool {
		incumbent.openMu.Lock()
		defer incumbent.unlockOpen()
		if candidate.session.State() == fsm.StateEstablished {
			if incumbent.State() == fsm.StateOpenSent {
				incumbent.closeCollision(message.NotifyCease, message.NotifyCeaseConnectionCollision, true)
				return true
			}
		}
		return false
	}()
	if retired {
		return
	}
	p.finishCollisionHandoff(incumbent, candidate.conn, remoteID)
}

func (p *Peer) selectedIncomingCandidate() *incomingCandidate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return inboundCandidate(p.inbound)
}
