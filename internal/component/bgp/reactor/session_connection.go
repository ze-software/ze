// Design: docs/architecture/core-design.md — session connect, accept, teardown
// Overview: session.go — BGP session struct and lifecycle
// Related: session_read.go — inbound message reading
// Related: session_write.go — wire write primitives

package reactor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/report"
)

// socketRecvBufSize is the SO_RCVBUF size for BGP sessions (256KB).
// Sized at 4x the default bufio.Reader size (64KB) to absorb burst traffic
// while the application drains the kernel buffer.
const socketRecvBufSize = 262144

// socketSendBufSize is the SO_SNDBUF size for BGP sessions (64KB).
// Sized at 4x the default bufio.Writer size (16KB) to allow write batching
// without blocking on kernel buffer space.
const socketSendBufSize = 65536

// Connect initiates an outgoing TCP connection.
// If LocalAddress is configured, binds to it for outgoing connections.
// This ensures consistent source address for next-hop self resolution.
//
// This is the one rail that OWNS the connection it hands to
// connectionEstablished: it dialed it, and runOnce -- its only production caller
// (peer_run.go) -- never sees the socket and cannot close it. Every installation
// failure therefore releases it here. Accepted sockets remain owned by their
// callers until installation succeeds.
func (s *Session) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	addr := net.JoinHostPort(s.settings.Address.String(), strconv.Itoa(int(s.settings.Port)))

	conn, err := s.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		s.logFSMEvent(fsm.EventTCPConnectionFails)
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	if err := s.connectionEstablished(conn); err != nil {
		closeConnQuietly(conn)
		return err
	}
	return nil
}

// Accept accepts an incoming TCP connection.
//
// A sealed session is refused here rather than reused. The seal is one-way (see
// seal below), so this is the cheap early answer and connectionEstablished holds
// the authoritative one, under the lock that publishes s.conn.
//
// Refusing is not losing the connection, and that holds at BOTH answers: neither
// this check nor connectionEstablished's closes conn. The caller keeps it.
// acceptOrReject buffers it on ErrSessionTearingDown for a passive peer
// (reactor_connection.go) and runOnce offers it to the NEXT cycle's session,
// which is a new Session value (peer_run.go). That buffer is what reuse after a
// teardown runs on; the session value itself is never reused.
func (s *Session) Accept(conn net.Conn) error {
	if s.tearingDown.Load() {
		return ErrSessionTearingDown
	}

	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	// Drain any stale errors from previous teardown.
	// If a teardown was queued but a new connection arrives before Run() exits,
	// the old ErrTeardown would incorrectly terminate the new session.
	// Drain all buffered errors (channel has buffer size 2).
drainLoop:
	for {
		select {
		case <-s.errChan:
		default:
			break drainLoop
		}
	}

	err := s.connectionEstablished(conn)
	if err != nil {
		return err
	}

	// Drain errChan again after connection setup.
	// A concurrent Teardown() may have sent ErrTeardown between our first
	// drain and connectionEstablished(). This second drain catches it.
drainLoop2:
	for {
		select {
		case <-s.errChan:
		default:
			break drainLoop2
		}
	}

	return nil
}

// acceptWithOpen accepts a connection and processes a pre-received OPEN.
// RFC 4271 §6.8: Used for collision resolution when we've already read the peer's OPEN.
func (s *Session) acceptWithOpen(conn net.Conn, peerOpen *message.Open, wire []byte) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	// Establish connection (sends our OPEN)
	if err := s.connectionEstablished(conn); err != nil {
		return err
	}
	s.observeReceivedWire(wire)

	// Process the pre-received OPEN
	return s.processOpen(peerOpen)
}

// processOpen handles a pre-parsed OPEN message.
// Used by acceptWithOpen for collision resolution.
func (s *Session) processOpen(open *message.Open) error {
	s.openMu.Lock()
	defer s.unlockOpen()
	if s.onOpenRecv != nil {
		s.onOpenRecv()
	}
	// Validate version
	if open.Version != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		s.logNotifyErr(conn,
			message.NotifyOpenMessage,
			message.NotifyOpenUnsupportedVersion,
			[]byte{4},
		)
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return ErrUnsupportedVersion
	}

	// Validate hold time
	if err := open.ValidateHoldTime(); err != nil {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if notif, ok := errors.AsType[*message.Notification](err); ok {
			s.logNotifyErr(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	// RFC 7607 Section 2: the collision-winner rail refuses AS 0 exactly as the handleOpen
	// rail does -- a reserved AS is reserved whichever connection survived.
	peerCaps, err := s.validateOpenPeerAS(open)
	if err != nil {
		return err
	}

	// RFC 6286 Section 2.2: the collision-winner rail validates the identifier exactly as the
	// handleOpen rail does -- an invalid identifier is invalid whichever connection survived.
	if err := s.validateOpenIdentifier(open); err != nil {
		return err
	}

	s.mu.Lock()
	s.peerOpen = open
	localOpen := s.localOpen
	s.mu.Unlock()

	// Run the peer OPEN validator (plugins such as RFC 9234 Role, plus the RFC 6286 Section 2.1
	// AS-wide identifier claim). This rail used to skip it entirely, so a peer could bypass any
	// per-peer OPEN policy by winning connection collision resolution.
	if err := s.runOpenValidator(open); err != nil {
		return err
	}

	// Peer capabilities were validated before checking AS identity.
	var localCaps []capability.Capability
	if localOpen != nil {
		localCaps, err = capability.ParseFromOptionalParams(localOpen.OptionalParams, localOpen.ExtendedParams)
		if err != nil {
			return fmt.Errorf("parse local OPEN capabilities: %w", err)
		}
	}

	// Negotiate capabilities.
	s.negotiateWith(localCaps, peerCaps)
	// draft-ietf-idr-bgp-bfd-strict-mode Section 6: the BfdStrictNegotiated
	// attribute is settled by this exchange, and every clause below reads it.
	s.applyBFDStrictNegotiation()

	// Validate required families and capabilities.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	requiredCaps := s.settings.RequiredCapabilities
	refusedCaps := s.settings.RefusedCapabilities
	s.mu.RUnlock()

	if len(requiredFamilies) > 0 && neg != nil {
		if missing := neg.CheckRequired(requiredFamilies); len(missing) > 0 {
			capData := buildUnsupportedCapabilityData(missing)
			s.logNotifyErr(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			s.logFSMEvent(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required families not negotiated: %v", ErrInvalidState, missing)
		}
	}

	// RFC 5492 Section 3: Validate required/refused capability codes.
	if err := s.validateCapabilityModes(conn, neg, requiredCaps, refusedCaps, localCaps, peerCaps); err != nil {
		return err
	}

	// Validate per-family ADD-PATH required/refused.
	if err := s.validateAddPathFamilyModes(conn, neg, s.settings.RequiredAddPathFamilies, s.settings.RefusedAddPathFamilies, localCaps, peerCaps); err != nil {
		return err
	}

	// Update the FSM, send the KEEPALIVE and reset the hold timer to the
	// negotiated value -- unless draft-ietf-idr-bgp-bfd-strict-mode Section
	// 8.5.5 says to withhold the KEEPALIVE and wait for BFD. This rail is the
	// collision winner, and the draft draws no distinction between the two:
	// the wait belongs to the connection that survived (session_bfd_strict.go).
	return s.advanceAfterOpen(conn)
}

func (s *Session) tuneTCPConnection(tcp *net.TCPConn) error {
	return tuneTCPConnectionForSettings(tcp, s.settings)
}

func tuneTCPConnectionForSettings(tcp *net.TCPConn, settings *PeerSettings) error {
	_ = tcp.SetNoDelay(true)

	addr, ok := tcp.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		if settings.MinTTL != 0 {
			return fmt.Errorf("tcp raw conn: %w", err)
		}
		return nil
	}

	var sysErr error
	if err := raw.Control(func(fd uintptr) {
		intFD := int(fd)
		if addr.IP.To4() != nil {
			_ = syscall.SetsockoptInt(intFD, syscall.IPPROTO_IP, syscall.IP_TOS, 0xC0)
		} else {
			_ = syscall.SetsockoptInt(intFD, syscall.IPPROTO_IPV6, syscall.IPV6_TCLASS, 0xC0)
		}
		if settings.OutTTL != 0 {
			if err := network.SetIPTTL(intFD, addr.IP, settings.OutTTL); err != nil {
				if !network.IsIPTTLUnsupported(err) {
					sysErr = err
					return
				}

				sessionLogger().Debug("GTSM outbound TTL not supported on this platform", "peer", settings.Name, "err", err)
			}
		}
		if settings.MinTTL != 0 {
			if err := network.SetIPMinTTL(intFD, addr.IP, settings.MinTTL); err != nil {
				if !network.IsIPTTLUnsupported(err) {
					sysErr = err
					return
				}

				sessionLogger().Debug("GTSM inbound TTL gate not supported on this platform", "peer", settings.Name, "err", err)
			}
		}
		// Set socket buffers for BGP burst throughput.
		if err := syscall.SetsockoptInt(intFD, syscall.SOL_SOCKET, syscall.SO_RCVBUF, socketRecvBufSize); err != nil {
			sessionLogger().Debug("SO_RCVBUF not set, using OS default", "err", err)
		}
		if err := syscall.SetsockoptInt(intFD, syscall.SOL_SOCKET, syscall.SO_SNDBUF, socketSendBufSize); err != nil {
			sessionLogger().Debug("SO_SNDBUF not set, using OS default", "err", err)
		}
	}); err != nil {
		return fmt.Errorf("tcp raw conn control: %w", err)
	}
	if sysErr != nil {
		return sysErr
	}
	return nil
}

// connectionEstablished handles a new TCP connection (incoming or outgoing).
func (s *Session) connectionEstablished(conn net.Conn) error {
	s.openMu.Lock()
	defer s.unlockOpen()
	// Tune TCP socket for BGP:
	// - TCP_NODELAY: BGP messages are application-framed and flushed
	//   explicitly via bufio.Writer, so Nagle only adds latency.
	// - IP_TOS/IPV6_TCLASS = 0xC0 (DSCP CS6, Internet Control):
	//   RFC 4271 §5.1 recommends IP precedence for BGP. Network devices
	//   with QoS policies prioritize CS6 traffic over regular data,
	//   reducing hold timer expiry risk under network congestion.
	// - IP_MINTTL/IPV6_MINHOPCOUNT: RFC 5082 GTSM receive-side filtering
	//   drops packets below the configured minimum TTL or Hop Limit.
	if tcp, ok := conn.(*net.TCPConn); ok {
		if err := s.tuneTCPConnection(tcp); err != nil {
			return err
		}
	}

	addresses, err := network.InterfacePrefixes()
	if err != nil {
		return fmt.Errorf("read local addresses for NEXT_HOP validation: %w", err)
	}
	s.nextHopScope.Store(newReceiveNextHopScope(addresses, conn, s.settings))

	// INVARIANT: s.conn, s.bufReader and s.bufWriter are assigned in the same
	// critical section so that readers capturing them under s.mu.RLock() (see
	// Run loop in session.go and ReadAndProcess in session_read.go) always see
	// a consistent triple. Do not split these assignments across lock
	// acquisitions -- doing so would let a reader see "conn != nil but
	// bufReader == nil" or similar and crash on io.ReadFull.
	//
	// s.writeMu is nested inside s.mu (lock ordering s.mu → s.writeMu, see
	// clearClosedConn below) to serialize the s.bufWriter assignment with concurrent
	// senders that read s.bufWriter while holding s.writeMu. Without this
	// nesting, a sender inside writeMu racing a connectionEstablished would
	// race on the s.bufWriter field (writer-reader mismatch on two separate
	// locks). writeMu is released before sendOpen below because sendOpen
	// re-acquires writeMu.
	//
	// This assignment is one of the two places a conn becomes a session, the
	// other being p.session in Peer.runOnce (peer_run.go). A sealed session never
	// gets one, and the check sits INSIDE the critical section that publishes
	// the conn because that is what decides the order against a concurrent seal
	// rather than racing it. seal stores true and teardown then reads s.conn
	// under this same s.mu (see below), so either this section runs first --
	// teardown sees the conn and the Cease goes out on it -- or teardown's read
	// runs first, in which case its store happened-before this lock and the load
	// below returns true.
	//
	// The seal is one-way: seal is the only writer of s.tearingDown and only
	// ever writes true. Nothing clears it, and Session.Accept must not -- it did
	// until 2026-08-11, between its own entry check and this one, which made
	// this gate read a flag the accept rail had just erased.
	//
	// What that closes is every route a stopping daemon has to a live session:
	// an inbound conn for a CONFIGURED peer (acceptOrReject -> acceptConnection
	// -> Accept), a conn a Listener had already accepted when Reactor.stop took
	// r.mu, and a dial that was in flight when the seal landed. Each would send
	// an OPEN the cancel then closes in silence (RFC 4271 Section 8.2.2,
	// ManualStop). Connect, Accept and acceptWithOpen reach the wire only
	// through here, so one gate covers all three.
	//
	// The refusal does NOT close conn, because this function does not own it on
	// two of those three rails. Accept's caller is acceptOrReject, which buffers
	// the connection on ErrSessionTearingDown for a passive peer and offers it to
	// the next cycle (reactor_connection.go); acceptWithOpen's caller is the
	// Peer run loop, which closes refused sockets itself. Closing here left both
	// holding a dead socket -- the buffered one costs the peer a whole backoff
	// when the next cycle accepts it. Connect is the one rail that owns what it
	// hands in, and it closes its own dial on this error (see Connect above).
	s.mu.Lock()
	if s.tearingDown.Load() {
		s.mu.Unlock()
		return ErrSessionTearingDown
	}
	s.writeMu.Lock()
	s.conn = conn
	s.transport.Store(connectedTransport(conn))
	readBufSize := max(env.GetInt("ze.buf.read.size", 65536), 4096)
	writeBufSize := max(env.GetInt("ze.buf.write.size", 16384), 4096)
	s.bufReader = bufio.NewReaderSize(conn, readBufSize)
	s.wireWriter = &observedBGPWriter{writer: conn, session: s, transport: s.transport.Load()}
	s.bufWriter = bufio.NewWriterSize(s.wireWriter, writeBufSize)
	s.writeMu.Unlock()
	s.mu.Unlock()

	// Prepare the local proposal without publishing bytes while DelayOpen
	// holds. Both OPEN receive rails negotiate against this exact proposal.
	if s.settings.DelayOpen != 0 {
		if err := s.prepareOpen(); err != nil {
			s.closeConn()
			return err
		}
	}

	// Signal FSM.
	if err := s.fsm.Event(fsm.EventTCPConnectionConfirmed); err != nil {
		return err
	}
	if s.timers.IsDelayOpenTimerRunning() {
		return nil
	}

	// Send OPEN message.
	if err := s.sendOpen(conn); err != nil {
		s.closeConn()
		return err
	}

	// Arm the OPEN-wait bound. ze.bgp.openwait (default 120s) is the maximum
	// time we wait for the peer's OPEN message. RFC 4271 §8.2.2 suggests "a
	// large value" for the OpenSent hold timer; we borrow the hold-timer
	// plumbing to enforce openwait. processOpen/Established state resets the
	// hold timer to the negotiated value once the peer's OPEN arrives.
	openWait := env.GetDuration("ze.bgp.openwait", 120*time.Second)
	s.timers.SetHoldTime(openWait)
	s.timers.StartHoldTimer()

	return nil
}

// CloseWithNotification closes the session with a specific NOTIFICATION.
// RFC 4271 §6.8: Used for collision detection to close with Cease/Connection Collision.
//
// It fires RFC 4271 Event 23 (OpenCollisionDump), not Event 2 (ManualStop),
// because the collision resolution is the local system's decision and not the
// operator's. The two events reach Idle alike; they differ on the
// ConnectRetryCounter, which §8.2.2 has Event 23 INCREMENT and Event 2 zero
// (fsm/connect_retry_counter.go). A connection lost to a collision is an
// attempt that failed, so zeroing there would erase a real retry history.
func (s *Session) CloseWithNotification(code message.NotifyErrorCode, subcode uint8) error {
	s.openMu.Lock()
	defer s.unlockOpen()
	s.closeCollision(code, subcode, true)
	return nil
}

// closeCollision owns the shared Event 23 retirement. The caller MUST hold
// openMu; notify is false only for an OPEN-less Connect/Active connection.
func (s *Session) closeCollision(code message.NotifyErrorCode, subcode uint8, notify bool) {
	s.setCloseReason(ErrCollisionDump)

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if notify {
		if conn != nil {
			s.logNotifyErr(conn, code, subcode, nil)
		}
	}

	// RFC 4271 Section 8.2.2 (OpenSent, Event 23): "releases all BGP resources".
	// The reason is set before NOTIFICATION I/O: a failed write can close the
	// transport before closeConn runs. Run must still identify the collision,
	// including when it reaches the nil-connection branch between reads.
	s.logFSMEvent(fsm.EventOpenCollisionDump)
	s.timers.StopAll()
	s.closeConn()

	select {
	case s.errChan <- ErrCollisionDump:
	default: // errChan full: the cancel goroutine already holds a signal
	}
}

// ErrCollisionDump is the close reason of a session that collision resolution
// chose to close (CloseWithNotification, RFC 4271 Event 23). Unlike ErrTeardown
// it takes the ordinary reconnect backoff: the connection was a failed attempt.
var ErrCollisionDump = errors.New("session closed by collision resolution")

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// RFC 4486 defines Cease subcodes: 1=MaxPrefixes, 2=AdminShutdown, 3=PeerDeconfigured,
// 4=AdminReset, 5=ConnectionRejected, 6=OtherConfigChange, 7=Collision, 8=OutOfResources.
// RFC 8203 specifies that subcodes 2/4 may include a shutdown communication message.
// If shutdownMsg is non-empty and subcode is 2 or 4, it is included per RFC 8203.
// If shutdownMsg is empty, the subcode name is used as a default message.
func (s *Session) Teardown(subcode uint8, shutdownMsg string) error {
	return s.teardown(subcode, shutdownMsg, fsm.EventManualStop)
}

// teardownAutomatic is Teardown for a stop the LOCAL SYSTEM chose rather than
// the operator: a BFD session going down (peer_bfd.go), a forward-pool
// out-of-resources drop (forward_pool_congestion.go).
//
// It differs from Teardown in the FSM event alone, and that difference is
// RFC 4271 §8.2.2's: Event 8 (AutomaticStop) increments the
// ConnectRetryCounter where Event 2 (ManualStop) zeroes it. §8.1.2 defines
// Event 8 as "Local system automatically stops the BGP connection", and gives
// a prefix maximum as its example. Everything else about the teardown -- the
// Cease NOTIFICATION, the RFC 8203 shutdown communication, the close reason,
// the errChan signal -- is identical.
func (s *Session) teardownAutomatic(subcode uint8, shutdownMsg string) error {
	return s.teardown(subcode, shutdownMsg, fsm.EventAutomaticStop)
}

// seal is the ONLY writer of s.tearingDown, and it only ever writes true, so a
// sealed session stays sealed for the rest of its life. That is what lets
// connectionEstablished answer "no conn is published on this session" once,
// instead of once per rail that can reach it.
//
// Two callers seal: teardown, on its way to the wire, and Reactor.stop, which
// seals every live session as part of the stop's own seal (peer.go,
// sealSession). The second is not covered by the first -- StopForRestart
// notifies nobody, so nothing would tear those sessions down -- and the seal has
// to hold for that stop too.
func (s *Session) seal() {
	s.tearingDown.Store(true)
	// Every teardown path seals, so every one of them stops the recovery
	// worker; stop never waits. runOnce's defer joins it.
	s.stopRecovery()
}

func (s *Session) teardown(subcode uint8, shutdownMsg string, stopEvent fsm.Event) error {
	return s.teardownWith(ceaseNotice(subcode, shutdownMsg), stopEvent)
}

// teardownNotice is the NOTIFICATION a teardown sends: the code, the subcode
// and the Data field, already built. A queued teardown carries it whole, so
// the octets an operator asked for are the octets the drain sends.
type teardownNotice struct {
	code    message.NotifyErrorCode
	subcode uint8
	data    []byte
}

// ceaseNotice is the Cease every teardown Ze decides on itself sends.
// BuildCeaseData carries the Shutdown Communication only under subcodes 2 and
// 4 (RFC 9003 Section 2). When no message is given Ze sends the subcode's name
// as the communication: that is Ze's choice, so a peer's log says why the
// session ended; the RFC only says a zero length means no communication.
func ceaseNotice(subcode uint8, shutdownMsg string) teardownNotice {
	msg := shutdownMsg
	if msg == "" {
		msg = message.CeaseSubcodeString(subcode)
	}
	return teardownNotice{code: message.NotifyCease, subcode: subcode, data: message.BuildCeaseData(subcode, msg)}
}

// operatorNotice is the NOTIFICATION an operator's teardown names: any code
// and subcode, and data as the Data field, octet for octet, so nil means an
// empty Data field. Data that does not fit is refused, never cut: the command
// handler checks it first, and this is the second check. The notice keeps its
// own copy, because a queued teardown outlives the caller's slice.
func operatorNotice(code, subcode uint8, data []byte) (teardownNotice, error) {
	// RFC 4271 Section 4.1: "The maximum message size is 4096 octets." The
	// header and the two code octets leave MaxNotificationDataLen for the Data.
	if err := message.CheckNotificationData(data); err != nil {
		return teardownNotice{}, err
	}
	return teardownNotice{code: message.NotifyErrorCode(code), subcode: subcode, data: slices.Clone(data)}, nil
}

// teardownWith ends the session with notice as its NOTIFICATION.
func (s *Session) teardownWith(notice teardownNotice, stopEvent fsm.Event) error {
	return s.teardownSession(&notice, stopEvent)
}

// teardownSession owns cleanup for both notified closes and the silent
// Connect/Active strict-mode configuration-change clauses. A nil notice means
// the protocol explicitly calls for dropping TCP without a NOTIFICATION.
func (s *Session) teardownSession(notice *teardownNotice, stopEvent fsm.Event) error {
	conn := s.beginTeardown(stopEvent)

	if conn != nil && notice != nil {
		s.logNotifyErr(conn, notice.code, notice.subcode, notice.data)
	}

	// The close comes BEFORE the stop event. The event runs the Peer's leave
	// arm on this goroutine, and the arm can be held for as long as the
	// session's buffered UPDATEs take to publish; a close behind it would
	// leave the socket open and Run in its read for that long. The close holds
	// no session lock, so the read loop and Run can reach the FSM ahead of
	// this goroutine: they deliver the stop event beginTeardown recorded, never
	// Event 18 (endEvent), and the event below then finds Idle, where the FSM
	// ignores a stop (fsm.handleIdle).
	s.closeConn()
	s.endTeardown(stopEvent)
	return nil
}

// endEvent is the FSM event a goroutine delivers when it finds the session's
// connection gone and no teardown of its own: the read loop on a closed
// connection, and the Peer's session end when Run returned with the FSM still
// in Established. It is the stop event of the teardown that began, when one
// did, because that teardown is why the connection went, and Event 18
// (TcpConnectionFails) otherwise. Safe for concurrent use.
func (s *Session) endEvent() fsm.Event {
	if stop := s.stopEvent.Load(); stop != nil {
		return *stop
	}
	return fsm.EventTCPConnectionFails
}

// ceaseAfterPanic ends the session after the panic boundary of its read task
// (Session.Run) recovered a panic. It MUST NOT block: the write path may be
// held by a writer that is itself stuck, so it waits for writeMu no longer
// than holdExpiryNotifyDeadline (tryNotificationWithin), and when the path
// stays held the close routine closes the socket without the NOTIFICATION and
// says so (closeConnWithin).
func (s *Session) ceaseAfterPanic() {
	conn := s.beginTeardown(fsm.EventAutomaticStop)
	if conn == nil {
		s.endTeardown(fsm.EventAutomaticStop)
		return
	}

	// A recovered panic is a Ze defect, not one of the fatal errors of RFC 4271
	// Section 6, so the session ends with a Cease, as Event 8 (AutomaticStop).
	// RFC 4271 Section 6.7: "In the absence of any fatal errors (that are
	// indicated in this section), a BGP peer MAY choose, at any given time, to
	// close its BGP connection by sending the NOTIFICATION message with the
	// Error Code Cease."
	// RFC 4486 Section 4 names no subcode for an internal failure, so:
	// RFC 4271 Section 4.5: "If no appropriate Error Subcode is defined, then a
	// zero (Unspecific) value is used for the Error Subcode field."
	err := s.tryNotificationWithin(conn, message.NotifyCease, message.NotifyCeaseUnspecific,
		holdExpiryNotifyDeadline)
	// A busy write path is not logged here: the sender left its mark, and the
	// close below reports that no NOTIFICATION was sent and interrupts the
	// writer without waiting for the path again.
	if err != nil {
		if !errors.Is(err, errWritePathBusy) {
			sessionLogger().Warn("notification send failed",
				"peer", s.settings.Address,
				"code", message.NotifyCease,
				"subcode", notifySubcodeValue(message.NotifyCease, message.NotifyCeaseUnspecific),
				"error", err,
			)
		}
	}
	s.closeConnWithin(holdExpiryNotifyDeadline)
	s.endTeardown(fsm.EventAutomaticStop)
}

// beginTeardown seals the session, records stopEvent as the event that ends it
// and ErrTeardown as the close reason, and stops the timers, then answers the
// connection the NOTIFICATION goes to, nil when none is up. The stop event is
// recorded first, before anything can close the connection, so that whichever
// goroutine tells the FSM the session ended tells it this event (endEvent).
// Every teardown MUST call endTeardown with the same event after it.
func (s *Session) beginTeardown(stopEvent fsm.Event) net.Conn {
	// The first teardown wins, as the first close reason does.
	s.stopEvent.CompareAndSwap(nil, &stopEvent)

	// Seal first: a conn published after this point would carry a session the
	// caller believes it has just ended.
	s.seal()
	s.setCloseReason(ErrTeardown)

	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	return conn
}

// endTeardown delivers stopEvent to the FSM and wakes Run's cancel goroutine.
// It MUST be called after beginTeardown and after the connection is closed.
// The event finds Idle, and changes nothing, when the read loop or the Peer's
// session end delivered it first (endEvent).
func (s *Session) endTeardown(stopEvent fsm.Event) {
	s.logFSMEvent(stopEvent)

	// Signal errChan so the cancel goroutine in Run() exits cleanly.
	// Non-blocking: channel may be full if cancel goroutine already consumed
	// a signal, or Run() may have already exited.
	select {
	case s.errChan <- ErrTeardown:
	default: // errChan full or closed — cancel goroutine already processed a signal
	}
}

// closeClearWait is how long a close waits, once the socket is closed, for a
// write path it could not take: a writer blocked on that socket has failed and
// let go by then. Only a writer that waits on something else outlasts it.
const closeClearWait = time.Second

// closeFlushBudget is the write budget of a close's final flush: what
// bufWriter still holds, a NOTIFICATION included, has this long to leave.
const closeFlushBudget = time.Second

// closeDrainWait is how long a close reads what the peer still sends after
// the FIN, so that Close sends a FIN and no RST.
const closeDrainWait = 100 * time.Millisecond

// closeClaimWait is how long a close waits for another close of the same
// connection that holds the claim on its socket (awaitClaimedClose). It
// outlasts the longest a claimant legitimately takes: its flush and its drain
// when it had the write path, or closeClearWait when it dropped the socket and
// waits for the writer to let go, which is the shorter of the two. One more
// closeDrainWait covers the scheduling between the claimant's steps.
const closeClaimWait = closeFlushBudget + 2*closeDrainWait

// writePathHeldFix is what an operator does about a close behind a held write
// path. The usual cause is on the peer, so no ze command clears it. The other
// cause is a writer that waits on a plugin: no command lists how far behind a
// running plugin is, so the step names the doctor, whose plugin rows carry
// their own fix. It fits report.FixMax.
const writePathHeldFix = "on the peer, find why its BGP process stopped reading (CPU load, a stalled process); " +
	"ze connects to it again by itself. When the peer reads normally, a plugin is not taking its events: " +
	"run `ze show doctor`"

// closeConn closes the session's connection through the one close routine
// (closeConnWithin), waiting for the write path no longer than closeWait.
// Safe for concurrent use. A second close of one connection closes and
// reports nothing, and returns once the first has finished, so a close that
// returned means the connection is no longer installed. The one exception is
// a writer that keeps the path while it waits on a plugin: the fields stay set
// on a closed socket, and a later close of that connection returns at once. The caller MUST NOT hold s.mu or the write path.
func (s *Session) closeConn() {
	s.closeConnWithin(s.closeWait())
}

// closeWait is how long a close may wait for the write path: one second when
// a hold timer or a Send Hold Timer expiry ended the session, because the
// timer has already waited the peer out, and the control budget for every
// other close, so a healthy session whose write path is busy for a moment
// still gets its final flush.
func (s *Session) closeWait() time.Duration {
	if reason := s.closeReason.Load(); reason != nil {
		if errors.Is(*reason, ErrHoldTimerExpired) {
			return holdExpiryNotifyDeadline
		}
		if errors.Is(*reason, ErrSendHoldTimerExpired) {
			return holdExpiryNotifyDeadline
		}
	}
	return s.controlWriteDeadline()
}

// closeConnWithin is the one close routine of a session connection. No close
// waits on a writer that the close itself would unblock:
//
//  1. It stops the Send Hold Timer.
//  2. It reads the connection under s.mu and lets s.mu go. No connection is
//     nothing to do. One another close already claimed (claimSocket) is
//     nothing to close or report: this close only waits, no longer than
//     closeClaimWait, for the claimant to finish, and clears the fields if
//     the claimant could not (awaitClaimedClose). The close is
//     idempotent per connection, and it never seals the session, because an
//     unsealed Session takes a second connection (a passive peer's Run and
//     Accept).
//  3. It waits for the write path no longer than wait, with no session lock
//     held. It does not wait at all when the NOTIFICATION sent just before
//     already found the path held (notifyMetBusyPath): a holder of openMu
//     then waits once behind a writer that was already stuck when the
//     NOTIFICATION asked for the path.
//  4. The path came free: closeFlushed claims the socket, flushes what
//     bufWriter holds, sends the FIN, drains and closes, so the peer reads a
//     NOTIFICATION that was written. The path stayed held: the close claims
//     the socket, the log says what the peer was not sent, and dropConn fails
//     the writer's socket write and closes the socket. A close that finds the
//     claim taken, because another close of this connection got there while
//     it waited, reports nothing and closes nothing, and leaves as in step 2.
//  5. It clears the connection fields in the recorded order, s.mu then the
//     write path, taking the path under s.mu only when it is free. When the
//     path is still held it waits for it closeClearWait with no lock held and
//     tries once more. A writer that waits on a plugin outlasts that: the
//     fields then stay set, socketClosed names the connection, and every
//     reader of s.conn meets a closed socket.
//
// Safe for concurrent use. The caller MUST NOT hold s.mu or the write path.
func (s *Session) closeConnWithin(wait time.Duration) {
	// Run and standalone readers share this terminal-completion wake. It is
	// deferred first, so it runs last and no admission waiter wakes under a
	// session lock.
	defer s.stopAdmission()
	// The Send Hold Timer takes sendHoldMu, which no step below holds.
	s.stopSendHoldTimer()
	// Do not clear DelayOpen here: the cancel goroutine can close a failed
	// writer's socket before Event 18 is delivered. Its failure owner must
	// still see the running timer. Run's retireSessionTimers and the Peer's
	// stopSessionTimers stop all protocol timers when the Session finishes.

	// Taken whether or not a connection is installed, so the mark never
	// outlives the close it was left for.
	notifyMissed := s.notifyMetBusyPath.Swap(false)

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()
	if conn == nil {
		return
	}
	if s.socketClosedIs(conn) {
		// Another close claimed this socket: it closed it behind a held write
		// path, or it is closing it under the path now. This close reports
		// nothing and closes nothing, and it still does not return with the
		// connection installed: it waits for the claimant to finish.
		s.awaitClaimedClose(conn)
		return
	}

	pathWait := wait
	if notifyMissed {
		pathWait = 0
	}
	pathFree := s.writeMu.LockWithin(s.clock, pathWait)
	var claim *socketClaim
	if pathFree {
		// The close that closes the socket is the one that says a NOTIFICATION
		// was due and not sent. When another close claimed the socket first,
		// the peer's last-error record already holds it (notificationWritten).
		claim = s.closeFlushed(conn)
		if claim != nil {
			if notifyMissed {
				s.reportWritePathHeld(wait, notifyMissed)
			}
		}
	} else {
		// Claimed before anything is reported or closed. Another close can
		// have taken this connection while this one waited: it dropped the
		// socket, or it holds the path for its own flush and drain, which a
		// wait as short as holdExpiryNotifyDeadline does not outlast. Either
		// way the socket is that close's, the path was not held by a stuck
		// writer for this close to report, and one close gives one line and
		// one event. The read loop, which wakes on the closed socket and
		// closes too, finds the claim the same way.
		claim = s.claimSocketForDrop(conn)
	}
	if claim == nil {
		// Another close finished with this connection or holds its socket.
		s.awaitClaimedClose(conn)
		return
	}
	// Every other close of this connection waits for this one to finish, its
	// attempt to clear the fields included. Deferred, so a panic below still
	// lets them go.
	defer close(claim.done)

	if !pathFree {
		// RFC 4271 Section 8.2.2, Event 10: "sends a NOTIFICATION message
		// with the Error Code Hold Timer Expired,"
		// Behind a held write path that action is not performed, here and for
		// every other close that owes a NOTIFICATION. The sender waited for the
		// path (one second for a hold timer, the control budget otherwise) and
		// the writer that holds it is blocked on a peer that accepts no data,
		// or on a plugin, so no octet of the NOTIFICATION can be put on this
		// connection behind what that writer has half written. The actions
		// that follow do not wait for it: "drops the TCP connection," is
		// dropConn below.
		//
		// Reported before the socket closes: the line is the only record of
		// what the peer was not sent.
		s.reportWritePathHeld(wait, notifyMissed)
		dropConn(conn)
	}

	// A writer blocked on the socket this close just closed fails and lets go
	// within closeClearWait.
	s.clearClosedConnWithin(conn, closeClearWait)
}

// clearClosedConnWithin is the last step of the close that claimed conn's
// socket: it clears the connection fields that name conn, waiting for the
// write path no longer than wait when it is held. No session lock is held
// across the wait. Only the claimant calls it, so nothing else probes the path
// between its wait and its clear; every other close waits for the claimant
// (awaitClaimedClose). The claimant therefore returns with the connection no
// longer installed, with one exception: a holder of the path that outlasts
// wait. That is a writer waiting on something else than the socket (a plugin
// that does not take its events). The fields then stay set: the socket is
// closed, so every later write and read on it fails. The caller MUST NOT hold
// s.mu or the write path.
func (s *Session) clearClosedConnWithin(conn net.Conn, wait time.Duration) {
	if s.clearClosedConn(conn) {
		return
	}
	if !s.writeMu.LockWithin(s.clock, wait) {
		return
	}
	s.writeMu.Unlock()
	s.clearClosedConn(conn)
}

// closeFlushed closes conn the way that lets the peer read what was written
// to it: flush, FIN, drain, close. The caller MUST hold the write path and
// MUST NOT hold s.mu; closeFlushed releases the path, by defer, so a panic in
// the flush cannot leave it held for every later writer of this session.
//
// It answers the claim under which this call closed the socket, whose done
// channel the caller MUST close once it has cleared the fields. It answers
// nil, with nothing done, when another close finished with this connection or
// claimed it while this one waited for the write path (claimSocket). The claim is
// taken before the flush, so a close that gives up on the path while this one
// holds it for the flush and the drain finds the claim, reports no held path
// and does not cut the flush short (closeConnWithin).
func (s *Session) closeFlushed(conn net.Conn) *socketClaim {
	defer s.writeMu.Unlock()
	// s.conn is read here under writeMu, one of the two locks it is written
	// under.
	if s.conn != conn {
		// No claim: another close finished with conn. closeConnWithin reads a
		// nil claim as "not this close's socket" and reports nothing.
		return nil
	}
	claim := s.claimSocket(conn)
	if claim == nil {
		return nil
	}
	// The claim keeps every later close away from this socket, so this call
	// closes it on every way out, a panic in the flush included. Deferred
	// after the unlock above, it runs before it: the socket is closed under
	// the write path.
	// RFC 4271 Section 8.2.2: "drops the TCP connection"
	defer closeConnQuietly(conn)

	if s.bufWriter != nil {
		if s.bufWriter.Buffered() > 0 {
			// A socket that refuses the budget is closed: nothing can be
			// flushed to it. The flush is the last write before the FIN, and
			// the connection closes whatever it answers.
			if s.armWriteBudget(closeFlushBudget) == nil {
				_ = s.bufWriter.Flush()
			}
		}
		s.commitAIGPWrites(false)
	}
	// Graceful close: send FIN (not RST) so the remote side can read any
	// pending NOTIFICATION before the connection is torn down. A plain Close
	// sends RST when unread data is in the receive buffer, which can cause the
	// remote kernel to discard our outbound data before the application reads
	// it.
	if tcp, ok := conn.(*net.TCPConn); ok {
		if cwErr := tcp.CloseWrite(); cwErr == nil {
			// FIN sent. Drain unread data so Close sends FIN instead of RST.
			// A refused read deadline ends the drain at once.
			_ = tcp.SetReadDeadline(s.clock.Now().Add(closeDrainWait))
			// The drain ends on its deadline or on a reset: its error is how
			// it ends, and the close goes on either way.
			_, _ = io.Copy(io.Discard, tcp)
		}
		// If CloseWrite failed, the socket is already broken: skip the drain.
	}
	// The deferred close ends the socket, and with it the budget the flush
	// armed: there is no deadline left for releaseWrites to clear, and none
	// for the next taker to meet. A socket that a failed write already closed
	// answers an error to that close, and is closed either way.
	s.writeBudgetArmed = false
	// bufReader is NOT nilled: Run may have captured conn before this close
	// and will read next. The stale bufReader wrapping the closed conn returns
	// a read error, which readAndProcessMessage handles as ErrConnectionClosed.
	// connectionEstablished replaces bufReader and bufWriter on reconnection.
	return claim
}

// writeDeadlinePassed is a write deadline in the past whatever clock a session
// runs on: setting it fails a write blocked in the kernel on that socket.
var writeDeadlinePassed = time.Unix(1, 0)

// dropConn closes conn without the write path: the close step of a close
// that found the path held by another writer, and of a writer that gives up
// its own connection with a partial frame on it (sendUpdateCounted). It
// expires the socket's write deadline first, which fails a write blocked in
// the kernel on this socket, so that writer retires and lets the path go.
// Nothing is flushed and no NOTIFICATION is sent. Safe for concurrent use.
func dropConn(conn net.Conn) {
	// RFC 9687 Section 5: "If the local system does not send any BGP messages
	// within the period specified in SendHoldTime, then a NOTIFICATION message
	// with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP
	// connection MUST be closed."
	// The MAY is not taken here: the writer that holds the path is the reason
	// nothing can be sent, and the MUST does not wait for it. A socket that
	// refuses the deadline is closed already, which is the aim.
	_ = conn.SetWriteDeadline(writeDeadlinePassed)
	// RFC 4271 Section 8.2.2: "drops the TCP connection"
	closeConnQuietly(conn)
}

// claimSocket makes the caller the one close that closes conn's socket, and
// reports whether it did. It answers false when another close of this
// connection holds the claim: that close dropped the socket, or is flushing
// and closing it under the write path right now. One connection therefore
// gets one close step, one ERROR line and one event, however many closes race
// for it.
//
// The caller MUST hold s.mu or the write path and have read s.conn == conn
// under it, so the claim cannot land on a connection whose fields a finished
// close has cleared (clearClosedConn clears s.conn and the claim together,
// under both locks). The loop ends: a swap fails only when another close
// stored its claim in between, and the next pass reads that claim.
//
// It answers the claim it took, nil when another close holds it. The claimant
// MUST close the claim's done channel once it has finished with the
// connection, its attempt to clear the fields included: that is what every
// other close of the connection waits for (awaitClaimedClose).
func (s *Session) claimSocket(conn net.Conn) *socketClaim {
	claim := &socketClaim{conn: conn, done: make(chan struct{})}
	for {
		claimed := s.socketClosed.Load()
		if claimed != nil {
			if claimed.conn == conn {
				return nil
			}
		}
		if s.socketClosed.CompareAndSwap(claimed, claim) {
			return claim
		}
	}
}

// socketClaim is one close's claim on a connection's socket (claimSocket).
type socketClaim struct {
	conn net.Conn
	// done is closed by the claimant when it has finished with conn: the
	// socket is closed and the fields were cleared, or could not be because a
	// writer kept the write path past closeClearWait.
	done chan struct{}
}

// claimSocketForDrop is claimSocket for a close that does not hold the write
// path: it reads s.conn under s.mu. The caller MUST NOT hold s.mu.
func (s *Session) claimSocketForDrop(conn net.Conn) *socketClaim {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.conn != conn {
		// No claim: another close finished with conn. closeConnWithin reads a
		// nil claim as "not this close's socket" and reports nothing.
		return nil
	}
	return s.claimSocket(conn)
}

// awaitClaimedClose is what a close does with a connection whose socket
// another close has claimed. It closes and reports nothing. It waits for the
// claimant to finish, no longer than closeClaimWait and with no lock held, so
// that it does not return over a connection that is still installed: Accept,
// Connect and the Peer's next attempt read that (ErrAlreadyConnected,
// Conn() == nil). It waits on the claim and never on the write path: two
// closes that both probed the path could each find it taken by the other's
// probe and each leave the fields set.
//
// When the claimant has finished and the fields still name conn, a writer kept
// the path past the claimant's own wait (a writer that waits on a plugin). One
// more attempt to clear is made, and the fields stay set if it still holds
// the path: the socket is closed, so every later write and read on it fails.
// A claim whose close already finished costs no wait. The caller MUST NOT hold
// s.mu or the write path.
func (s *Session) awaitClaimedClose(conn net.Conn) {
	if claim := s.socketClosed.Load(); claim != nil {
		if claim.conn == conn {
			s.awaitClaim(claim)
		}
	}
	s.clearClosedConn(conn)
}

// awaitClaim waits for claim's close to finish, no longer than closeClaimWait.
// A finished claim costs no timer.
func (s *Session) awaitClaim(claim *socketClaim) {
	select {
	case <-claim.done:
		return
	default: // Still at work: wait for it below.
	}
	timer := s.clock.NewTimer(closeClaimWait)
	defer timer.Stop()
	select {
	case <-claim.done:
	case <-timer.C():
	}
}

// socketClosedIs reports whether a close already claimed conn's socket
// (claimSocket): the socket is closed, or is being closed under the write
// path. Safe for concurrent use.
func (s *Session) socketClosedIs(conn net.Conn) bool {
	closed := s.socketClosed.Load()
	if closed == nil {
		return false
	}
	return closed.conn == conn
}

// clearClosedConn clears the connection fields that name conn, whose socket is
// closed, in the recorded order: s.mu, then the write path. It never waits for
// the write path under s.mu: a writer holds that path for as long as its socket
// or a plugin takes, and a dynamic peer's writer may want s.mu meanwhile. It
// reports whether the fields no longer name conn. The caller MUST NOT hold
// s.mu or the write path.
func (s *Session) clearClosedConn(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != conn {
		return true
	}
	if !s.writeMu.TryLock() {
		return false
	}
	s.conn = nil
	s.transport.Store(nil)
	s.socketClosed.Store(nil)
	s.writeMu.Unlock()
	return true
}

// reportWritePathHeld tells the operator a close met a write path another
// writer held: one ERROR line and the same problem as an error event on the
// report bus, which `show doctor` and `show errors` print. notifyMissed says a
// NOTIFICATION was due and could not be sent for that reason; otherwise the
// close itself waited wait for the path and gave up.
func (s *Session) reportWritePathHeld(wait time.Duration, notifyMissed bool) {
	s.writePathHeldReported.Store(true)
	peerAddr := s.settings.Address.String()
	if notifyMissed {
		sessionLogger().Error("no NOTIFICATION was sent to the peer: another writer held the "+
			"session's write path, so the connection is closed without one",
			"peer", peerAddr,
			"fix", writePathHeldFix, "code", diagnostic.CodeDoctorBGPWritePathHeld)
		report.RaiseErrorFix(reportSourceBGP, diagnostic.CodeDoctorBGPWritePathHeld, peerAddr,
			"BGP session closed: no NOTIFICATION was sent to peer "+peerAddr+
				", another writer held the session's write path",
			writePathHeldFix, map[string]any{"notification": "not-sent"})
		return
	}
	sessionLogger().Error("BGP connection closed while another writer held the session's write path: "+
		"what that writer had not sent to the peer is dropped",
		"peer", peerAddr, "waited", wait,
		"fix", writePathHeldFix, "code", diagnostic.CodeDoctorBGPWritePathHeld)
	report.RaiseErrorFix(reportSourceBGP, diagnostic.CodeDoctorBGPWritePathHeld, peerAddr,
		"BGP session closed: another writer held the write path of the session to peer "+peerAddr+
			" for "+wait.String()+"; what it had not sent is dropped",
		writePathHeldFix, map[string]any{"waited": wait.String()})
}

// setCloseReason atomically stores why the connection is being closed.
// Only the first reason wins — subsequent calls are no-ops. A teardown owner
// must record its cause before NOTIFICATION I/O: a failed write retires the
// transport and wakes Run, which must not mistake that consequence for its cause.
// It reports whether err became the reason: a caller that reports the close to
// the operator does so only when its reason won (recordWriteFailure).
func (s *Session) setCloseReason(err error) bool {
	return s.closeReason.CompareAndSwap(nil, &err)
}

// sessionTransport keeps the connection's socket identity and actual local OPEN
// identity, never mutable configuration or the identity of a replacement epoch.
type sessionTransport struct {
	local                 netip.Addr
	localString           string
	localPort, remotePort uint16
	// sendOpen MUST publish the built OPEN's ASN before writing its first byte.
	// Observers MUST load this once-set value instead of configured LocalAS.
	localAS atomic.Uint32
}

// connectedLocalAddress answers the local endpoint of session's TCP connection,
// the address the peer reaches this speaker on, when a next hop can carry it.
// It answers the zero Addr, which every caller tests with IsValid, for a nil
// session, a session not yet connected, and an unspecified or multicast
// endpoint. Next-hop self reads it on the announce rail (resolveNextHop) and
// on both forward rails (precomputeNextHop), so the rails send one address.
func connectedLocalAddress(session *Session) netip.Addr {
	if session == nil {
		return netip.Addr{}
	}
	transport := session.transport.Load()
	if transport == nil {
		return netip.Addr{}
	}
	local := transport.local
	if !local.IsValid() {
		return netip.Addr{}
	}
	if local.IsUnspecified() {
		return netip.Addr{}
	}
	if local.IsMulticast() {
		return netip.Addr{}
	}
	return local
}

func connectedTransport(conn net.Conn) *sessionTransport {
	endpoints := &sessionTransport{}
	if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
		local := addr.AddrPort()
		endpoints.local = local.Addr().Unmap()
		endpoints.localString = endpoints.local.String()
		endpoints.localPort = local.Port()
	}
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		endpoints.remotePort = addr.AddrPort().Port()
	}
	return endpoints
}
