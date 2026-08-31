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
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/network"
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
// (peer_run.go) -- never sees the socket and cannot close it. So a sealed session
// refusing the dial is closed here. The accept rails are the other way round:
// their callers keep the connection, so connectionEstablished must not close it.
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
		if errors.Is(err, ErrSessionTearingDown) {
			closeConnQuietly(conn)
		}
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

// AcceptWithOpen accepts a connection and processes a pre-received OPEN.
// RFC 4271 §6.8: Used for collision resolution when we've already read the peer's OPEN.
func (s *Session) AcceptWithOpen(conn net.Conn, peerOpen *message.Open) error {
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

	// Process the pre-received OPEN
	return s.processOpen(peerOpen)
}

// processOpen handles a pre-parsed OPEN message.
// Used by AcceptWithOpen for collision resolution.
func (s *Session) processOpen(open *message.Open) error {
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
	if err := s.validateOpenPeerAS(open); err != nil {
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

	// Parse capabilities once from both OPENs.
	var localCaps []capability.Capability
	var err error
	if localOpen != nil {
		localCaps, err = capability.ParseFromOptionalParams(localOpen.OptionalParams, localOpen.ExtendedParams)
		if err != nil {
			return fmt.Errorf("parse local OPEN capabilities: %w", err)
		}
	}
	peerCaps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		return s.rejectOpenCapabilityError(err)
	}

	// Negotiate capabilities.
	s.negotiateWith(localCaps, peerCaps)

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
	if err := s.validateCapabilityModes(conn, neg, requiredCaps, refusedCaps); err != nil {
		return err
	}

	// Validate per-family ADD-PATH required/refused.
	if err := s.validateAddPathFamilyModes(conn, neg, s.settings.RequiredAddPathFamilies, s.settings.RefusedAddPathFamilies); err != nil {
		return err
	}

	// Update FSM
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE
	if err := s.sendKeepalive(conn); err != nil {
		return err
	}

	// Reset hold timer
	s.timers.ResetHoldTimer()

	return nil
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
				if network.IsIPTTLUnsupported(err) {
					sessionLogger().Debug("GTSM outbound TTL not supported on this platform", "peer", settings.Name, "err", err)
				} else {
					sysErr = err
					return
				}
			}
		}
		if settings.MinTTL != 0 {
			if err := network.SetIPMinTTL(intFD, addr.IP, settings.MinTTL); err != nil {
				if network.IsIPTTLUnsupported(err) {
					sessionLogger().Debug("GTSM inbound TTL gate not supported on this platform", "peer", settings.Name, "err", err)
				} else {
					sysErr = err
					return
				}
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

	// INVARIANT: s.conn, s.bufReader and s.bufWriter are assigned in the same
	// critical section so that readers capturing them under s.mu.RLock() (see
	// Run loop in session.go and ReadAndProcess in session_read.go) always see
	// a consistent triple. Do not split these assignments across lock
	// acquisitions -- doing so would let a reader see "conn != nil but
	// bufReader == nil" or similar and crash on io.ReadFull.
	//
	// s.writeMu is nested inside s.mu (lock ordering s.mu → s.writeMu, see
	// closeConn below) to serialize the s.bufWriter assignment with concurrent
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
	// an inbound conn for a CONFIGURED peer (acceptOrReject -> AcceptConnection
	// -> Accept), a conn a Listener had already accepted when Reactor.stop took
	// r.mu, and a dial that was in flight when the seal landed. Each would send
	// an OPEN the cancel then closes in silence (RFC 4271 Section 8.2.2,
	// ManualStop). Connect, Accept and AcceptWithOpen reach the wire only
	// through here, so one gate covers all three.
	//
	// The refusal does NOT close conn, because this function does not own it on
	// two of those three rails. Accept's caller is acceptOrReject, which buffers
	// the connection on ErrSessionTearingDown for a passive peer and offers it to
	// the next cycle (reactor_connection.go); AcceptWithOpen's caller is
	// acceptPendingConnection, which closes it itself. Closing here left both
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
	readBufSize := max(env.GetInt("ze.buf.read.size", 65536), 4096)
	writeBufSize := max(env.GetInt("ze.buf.write.size", 16384), 4096)
	s.bufReader = bufio.NewReaderSize(conn, readBufSize)
	s.bufWriter = bufio.NewWriterSize(conn, writeBufSize)
	s.writeMu.Unlock()
	s.mu.Unlock()

	// Signal FSM.
	if err := s.fsm.Event(fsm.EventTCPConnectionConfirmed); err != nil {
		return err
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
	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		s.logNotifyErr(conn, code, subcode, nil)
	}

	s.closeConn()
	s.logFSMEvent(fsm.EventOpenCollisionDump)

	return nil
}

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// RFC 4486 defines Cease subcodes: 1=MaxPrefixes, 2=AdminShutdown, 3=PeerDeconfigured,
// 4=AdminReset, 5=ConnectionRejected, 6=OtherConfigChange, 7=Collision, 8=OutOfResources.
// RFC 8203 specifies that subcodes 2/4 may include a shutdown communication message.
// If shutdownMsg is non-empty and subcode is 2 or 4, it is included per RFC 8203.
// If shutdownMsg is empty, the subcode name is used as a default message.
func (s *Session) Teardown(subcode uint8, shutdownMsg string) error {
	return s.teardown(subcode, shutdownMsg, fsm.EventManualStop)
}

// TeardownAutomatic is Teardown for a stop the LOCAL SYSTEM chose rather than
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
func (s *Session) TeardownAutomatic(subcode uint8, shutdownMsg string) error {
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
}

func (s *Session) teardown(subcode uint8, shutdownMsg string, stopEvent fsm.Event) error {
	// Seal first: a conn published after this point would carry a session the
	// caller believes it has just ended.
	s.seal()

	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		// Build data per RFC 8203: length byte + message for subcodes 2/4
		var data []byte
		if subcode == message.NotifyCeaseAdminShutdown || subcode == message.NotifyCeaseAdminReset {
			msg := shutdownMsg
			if msg == "" {
				msg = message.CeaseSubcodeString(subcode)
			}
			data = message.BuildShutdownData(msg)
		}

		s.logNotifyErr(conn,
			message.NotifyCease,
			subcode,
			data,
		)
	}

	// Set close reason BEFORE closing conn so the read loop can identify this
	// as a teardown (not just a connection reset) after ReadFull returns error.
	s.setCloseReason(ErrTeardown)
	s.closeConn()
	s.logFSMEvent(stopEvent)

	// Signal errChan so the cancel goroutine in Run() exits cleanly.
	// Non-blocking: channel may be full if cancel goroutine already consumed
	// a signal, or Run() may have already exited.
	select {
	case s.errChan <- ErrTeardown:
	default: // errChan full or closed — cancel goroutine already processed a signal
	}

	return nil
}

// closeConn closes the TCP connection.
// Uses half-close (CloseWrite) when possible to send TCP FIN instead of RST.
// This ensures the remote side can read any pending data (e.g., NOTIFICATION)
// before the connection is fully closed.
func (s *Session) closeConn() {
	// Stop Send Hold Timer before acquiring s.mu to avoid lock ordering issues.
	s.stopSendHoldTimer()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		// Flush under writeMu to avoid racing with Send* methods that
		// also access bufWriter under writeMu. Lock ordering: s.mu → s.writeMu.
		if s.bufWriter != nil {
			s.writeMu.Lock()
			_ = s.bufWriter.Flush()
			s.writeMu.Unlock()
		}
		// Graceful close: send FIN (not RST) so the remote side can read
		// any pending NOTIFICATION before the connection is torn down.
		// A plain Close() sends RST when unread data is in the receive
		// buffer, which can cause the remote kernel to discard our
		// outbound data before the application reads it.
		if tcp, ok := s.conn.(*net.TCPConn); ok {
			if cwErr := tcp.CloseWrite(); cwErr == nil {
				// FIN sent. Drain unread data so Close() sends FIN instead of RST.
				_ = tcp.SetReadDeadline(s.clock.Now().Add(100 * time.Millisecond))
				if _, drainErr := io.Copy(io.Discard, tcp); drainErr != nil {
					// Drain failure is expected (timeout or reset) -- proceed to Close.
					_ = drainErr
				}
			}
			// If CloseWrite failed, socket is already broken -- skip drain.
		}
		_ = s.conn.Close()
		s.conn = nil
		// bufReader is NOT nilled here: Run() may have captured conn (non-nil)
		// before this lock and will call readAndProcessMessage next. The stale
		// bufReader wrapping the closed conn returns a proper read error,
		// which readAndProcessMessage handles as ErrConnectionClosed.
		// connectionEstablished() replaces bufReader and bufWriter on reconnection.
	}
}

// setCloseReason atomically stores why the connection is being closed.
// Only the first reason wins — subsequent calls are no-ops.
func (s *Session) setCloseReason(err error) {
	s.closeReason.CompareAndSwap(nil, &err)
}
