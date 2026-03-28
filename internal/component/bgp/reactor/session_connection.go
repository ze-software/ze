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
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
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
func (s *Session) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	addr := net.JoinHostPort(s.settings.Address.String(), fmt.Sprintf("%d", s.settings.Port))

	conn, err := s.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		s.logFSMEvent(fsm.EventTCPConnectionFails)
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	return s.connectionEstablished(conn)
}

// Accept accepts an incoming TCP connection.
func (s *Session) Accept(conn net.Conn) error {
	// Check if session is being torn down - reject if so.
	// This prevents accepting a connection on a session that's about to exit.
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

	// Reset tearing down flag for new connection.
	// This allows the session to be reused after a teardown.
	s.tearingDown.Store(false)

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

		var notif *message.Notification
		if errors.As(err, &notif) {
			s.logNotifyErr(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	s.mu.Lock()
	s.peerOpen = open
	localOpen := s.localOpen
	s.mu.Unlock()

	// Parse capabilities once from both OPENs.
	var localCaps []capability.Capability
	if localOpen != nil {
		localCaps = capability.ParseFromOptionalParams(localOpen.OptionalParams)
	}
	peerCaps := capability.ParseFromOptionalParams(open.OptionalParams)

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

// connectionEstablished handles a new TCP connection (incoming or outgoing).
func (s *Session) connectionEstablished(conn net.Conn) error {
	// Tune TCP socket for BGP:
	// - TCP_NODELAY: BGP messages are application-framed and flushed
	//   explicitly via bufio.Writer, so Nagle only adds latency.
	// - IP_TOS/IPV6_TCLASS = 0xC0 (DSCP CS6, Internet Control):
	//   RFC 4271 §5.1 recommends IP precedence for BGP. Network devices
	//   with QoS policies prioritize CS6 traffic over regular data,
	//   reducing hold timer expiry risk under network congestion.
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
		if addr, ok := tcp.RemoteAddr().(*net.TCPAddr); ok {
			if raw, err := tcp.SyscallConn(); err == nil {
				_ = raw.Control(func(fd uintptr) {
					if addr.IP.To4() != nil {
						_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TOS, 0xC0)
					} else {
						_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_TCLASS, 0xC0)
					}
					// Set socket buffers for BGP burst throughput.
					if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, socketRecvBufSize); err != nil {
						sessionLogger().Debug("SO_RCVBUF not set, using OS default", "err", err)
					}
					if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, socketSendBufSize); err != nil {
						sessionLogger().Debug("SO_SNDBUF not set, using OS default", "err", err)
					}
				})
			}
		}
	}

	s.mu.Lock()
	s.conn = conn
	readBufSize := max(env.GetInt("ze.buf.read.size", 65536), 4096)
	writeBufSize := max(env.GetInt("ze.buf.write.size", 16384), 4096)
	s.bufReader = bufio.NewReaderSize(conn, readBufSize)
	s.bufWriter = bufio.NewWriterSize(conn, writeBufSize)
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

	// Start hold timer (for waiting for peer's OPEN).
	s.timers.StartHoldTimer()

	return nil
}

// Close gracefully closes the session.
func (s *Session) Close() error {
	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		// Send NOTIFICATION (Cease/Administrative Shutdown).
		s.logNotifyErr(conn,
			message.NotifyCease,
			message.NotifyCeaseAdminShutdown,
			nil,
		)
	}

	s.closeConn()
	s.logFSMEvent(fsm.EventManualStop)

	return nil
}

// CloseWithNotification closes the session with a specific NOTIFICATION.
// RFC 4271 §6.8: Used for collision detection to close with Cease/Connection Collision.
func (s *Session) CloseWithNotification(code message.NotifyErrorCode, subcode uint8) error {
	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		s.logNotifyErr(conn, code, subcode, nil)
	}

	s.closeConn()
	s.logFSMEvent(fsm.EventManualStop)

	return nil
}

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// RFC 4486 defines Cease subcodes: 1=MaxPrefixes, 2=AdminShutdown, 3=PeerDeconfigured,
// 4=AdminReset, 5=ConnectionRejected, 6=OtherConfigChange, 7=Collision, 8=OutOfResources.
// RFC 8203 specifies that subcodes 2/4 may include a shutdown communication message.
// If shutdownMsg is non-empty and subcode is 2 or 4, it is included per RFC 8203.
// If shutdownMsg is empty, the subcode name is used as a default message.
func (s *Session) Teardown(subcode uint8, shutdownMsg string) error {
	// Mark session as tearing down to prevent accepting new connections
	s.tearingDown.Store(true)

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
	s.logFSMEvent(fsm.EventManualStop)

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
