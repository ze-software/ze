// Design: docs/architecture/core-design.md — wire write primitives and Send* methods
// Overview: session.go — BGP session struct and lifecycle
// Related: session_read.go — inbound message reading (symmetric counterpart)
// Related: session_connection.go — session connect, accept, teardown

package reactor

import (
	"net"
	"net/netip"
	"slices"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// controlWriteDeadlineMin is the minimum write deadline for control messages.
const controlWriteDeadlineMin = 10 * time.Second

// sendKeepalive sends a KEEPALIVE message.
func (s *Session) sendKeepalive(conn net.Conn) error {
	return s.writeMessage(conn, message.NewKeepalive())
}

// sendNotification sends a NOTIFICATION message.
// Increments the notification counter only after a successful write.
func (s *Session) sendNotification(conn net.Conn, code message.NotifyErrorCode, subcode uint8, data []byte) error {
	notif := &message.Notification{
		ErrorCode:    code,
		ErrorSubcode: subcode,
		Data:         data,
	}
	err := s.writeMessage(conn, notif)
	if err == nil && s.onNotifSent != nil {
		s.onNotifSent(uint8(code), subcode)
	}
	return err
}

// writeMessage writes a BGP message directly into the session buffer.
// Always flushes immediately -- used for KEEPALIVE, OPEN, NOTIFICATION.
// Sets a write deadline to prevent indefinite blocking on stuck TCP.
func (s *Session) writeMessage(conn net.Conn, msg message.Message) error {
	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Set write deadline to prevent indefinite blocking on stuck TCP.
	// Without this, a stuck connection holds writeMu forever, preventing
	// the forward worker from detecting the failure and triggering teardown.
	deadline := s.controlWriteDeadline()
	if err := conn.SetWriteDeadline(s.clock.Now().Add(deadline)); err != nil {
		return err
	}
	defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()

	// Zero-allocation: write directly into session buffer.
	// Message types don't use context for basic encoding
	// (KEEPALIVE, OPEN, NOTIFICATION have fixed formats).
	s.writeBuf.Reset()
	n := msg.WriteTo(s.writeBuf.Buffer(), 0, nil)

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireWriteErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}
	if err := s.bufWriter.Flush(); err != nil {
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireWriteErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}

	// Counts bytes staged into bufWriter then flushed. Both Write and Flush
	// succeeded, so for TCP sockets this reflects bytes delivered to the kernel.
	if s.prefixMetrics != nil {
		s.prefixMetrics.wireBytesSent.With(s.settings.Address.String()).Add(float64(n))
	}

	// Successful write -- reset RFC 9687 Send Hold Timer.
	s.resetSendHoldTimer()

	// Notify callback after successful send.
	// Body is data after the 19-byte header (16-byte marker + 2-byte length + 1-byte type).
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, msg.Type(), body, nil, s.sendCtxID, rpc.DirectionSent, BufHandle{}, nil)
	}

	return nil
}

// controlWriteDeadline returns the write deadline for control messages
// (KEEPALIVE, OPEN, NOTIFICATION). Bounded by min(holdTime/3, 30s)
// with a minimum of 10s.
func (s *Session) controlWriteDeadline() time.Duration {
	holdTime := s.settings.ReceiveHoldTime
	if holdTime <= 0 {
		return fwdWriteDeadlineDefault
	}
	d := max(holdTime/3, controlWriteDeadlineMin)
	return min(d, fwdWriteDeadlineDefault)
}

// sendHoldDuration returns the Send Hold Timer duration per RFC 9687.
// If SendHoldTime is explicitly configured (>0), use that value.
// Otherwise use the RFC 9687 recommendation: max(8 minutes, 2x ReceiveHoldTime).
func (s *Session) sendHoldDuration() time.Duration {
	if s.settings.SendHoldTime > 0 {
		return s.settings.SendHoldTime
	}
	return max(sendHoldTimerMin, 2*s.settings.ReceiveHoldTime)
}

// startSendHoldTimer starts the RFC 9687 Send Hold Timer.
// Called when session enters ESTABLISHED. The timer is reset on every
// successful write. On expiry, the session is torn down.
func (s *Session) startSendHoldTimer() {
	d := s.sendHoldDuration()
	s.sendHoldMu.Lock()
	defer s.sendHoldMu.Unlock()
	s.stopSendHoldTimerLocked()
	s.sendHoldTimer = s.clock.AfterFunc(d, s.sendHoldTimerExpired)
}

// resetSendHoldTimer resets the Send Hold Timer after a successful write.
// Stops the current timer and creates a new AfterFunc. Timer.Reset is unsafe
// for AfterFunc timers: it cannot guarantee the callback won't run concurrently
// with a previously-fired instance (Go docs: "Reset does not wait for prior f
// to complete"). Stop+AfterFunc ensures single-execution semantics.
func (s *Session) resetSendHoldTimer() {
	s.sendHoldMu.Lock()
	defer s.sendHoldMu.Unlock()
	if s.sendHoldTimer != nil {
		s.sendHoldTimer.Stop()
		s.sendHoldTimer = s.clock.AfterFunc(s.sendHoldDuration(), s.sendHoldTimerExpired)
	}
}

// stopSendHoldTimer stops the Send Hold Timer.
func (s *Session) stopSendHoldTimer() {
	s.sendHoldMu.Lock()
	defer s.sendHoldMu.Unlock()
	s.stopSendHoldTimerLocked()
}

func (s *Session) stopSendHoldTimerLocked() {
	if s.sendHoldTimer != nil {
		s.sendHoldTimer.Stop()
		s.sendHoldTimer = nil
	}
}

// sendHoldTimerExpired is called when the RFC 9687 Send Hold Timer expires.
// This means the local side has been unable to send any data for the
// configured duration. Try to send NOTIFICATION, then close.
func (s *Session) sendHoldTimerExpired() {
	sessionLogger().Warn("send hold timer expired (RFC 9687)",
		"peer", s.settings.Address,
		"duration", s.sendHoldDuration(),
	)

	// Stop the timer before attempting NOTIFICATION. Otherwise the
	// NOTIFICATION write (if it succeeds) resets the timer via
	// writeMessage -> resetSendHoldTimer, creating a new timer that
	// closeConn immediately stops.
	s.stopSendHoldTimer()

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	// Try to send NOTIFICATION (may fail if TCP is stuck -- that's expected).
	if conn != nil {
		s.logNotifyErr(conn,
			message.NotifySendHoldTimerExpired,
			0, // No subcode defined by RFC 9687
			nil,
		)
	}

	s.mu.Lock()
	s.logFSMEvent(fsm.EventHoldTimerExpires)
	s.mu.Unlock()

	s.setCloseReason(ErrSendHoldTimerExpired)
	s.closeConn()

	select {
	case s.errChan <- ErrSendHoldTimerExpired:
	default: // errChan full -- cancel goroutine already has a signal
	}
}

// TriggerHoldTimerExpiry triggers the hold timer expiry event.
// Exposed for testing.
func (s *Session) TriggerHoldTimerExpiry() {
	s.timers.StopAll()
	s.logFSMEvent(fsm.EventHoldTimerExpires)
	s.closeConn()
}

// writeUpdate writes an UPDATE to bufWriter without locking or flushing.
// Caller must hold writeMu.
func (s *Session) writeUpdate(update *message.Update) error {
	s.writeBuf.Reset()
	n := update.WriteTo(s.writeBuf.Buffer(), 0, nil) // nil ctx: UPDATE already has wire bytes

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireWriteErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}

	if s.prefixMetrics != nil {
		s.prefixMetrics.wireBytesSent.With(s.settings.Address.String()).Add(float64(n))
	}

	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		sessionLogger().Debug("SendUpdate", "peer", s.settings.Address, "direction", "sent", "ctxID", s.sendCtxID, "msgLen", n)
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, rpc.DirectionSent, BufHandle{}, s.sentMeta)
	}

	return nil
}

// writeRawUpdateBody writes a raw UPDATE body to bufWriter without locking or flushing.
// Caller must hold writeMu. Fires sent event callback with route metadata.
func (s *Session) writeRawUpdateBody(body []byte) error {
	totalLen := message.HeaderLen + len(body)
	s.writeBuf.Reset()
	buf := s.writeBuf.Buffer()

	copy(buf[:message.MarkerLen], message.Marker[:])
	buf[16] = byte(totalLen >> 8)
	buf[17] = byte(totalLen)
	buf[18] = byte(message.TypeUPDATE)
	copy(buf[message.HeaderLen:], body)

	if _, err := s.bufWriter.Write(buf[:totalLen]); err != nil {
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireWriteErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}

	if s.prefixMetrics != nil {
		s.prefixMetrics.wireBytesSent.With(s.settings.Address.String()).Add(float64(totalLen))
	}

	if s.onMessageReceived != nil {
		sessionLogger().Debug("SendRawUpdateBody", "peer", s.settings.Address, "direction", "sent", "ctxID", s.sendCtxID, "bodyLen", len(body))
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, rpc.DirectionSent, BufHandle{}, s.sentMeta)
	}

	return nil
}

// flushWrites flushes the bufWriter. Caller must hold writeMu.
// Increments wireWriteErrors on flush failure (TCP write error).
func (s *Session) flushWrites() error {
	if err := s.bufWriter.Flush(); err != nil {
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireWriteErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}
	return nil
}

// appendFwdDirty tracks a destination session that has unflushed RS fast path
// writes. Deduplicates so each session appears at most once per batch.
// Only called from this session's read goroutine (no synchronization needed).
func (s *Session) appendFwdDirty(dst *Session) {
	if slices.Contains(s.fwdDirty, dst) {
		return
	}
	s.fwdDirty = append(s.fwdDirty, dst)
}

// flushFwdDirty flushes all destination sessions that have pending RS fast
// path writes. Called when the source bufReader has no more data (natural
// batch boundary) or on read loop exit.
//
// For each dirty session: acquires writeMu via TryLock, sets write deadline,
// flushes bufWriter, clears deadline, resets send hold timer. Sessions where
// TryLock fails are retained for the next flush cycle.
func (s *Session) flushFwdDirty() {
	if len(s.fwdDirty) == 0 {
		return
	}
	deadline := s.clock.Now().Add(fwdWriteDeadline())
	kept := 0
	for _, dst := range s.fwdDirty {
		dst.mu.RLock()
		conn := dst.conn
		state := dst.fsm.State()
		dst.mu.RUnlock()
		if state != fsm.StateEstablished || conn == nil {
			continue
		}
		if !dst.writeMu.TryLock() {
			s.fwdDirty[kept] = dst
			kept++
			continue
		}
		_ = conn.SetWriteDeadline(deadline)
		_ = dst.flushWrites()
		_ = conn.SetWriteDeadline(time.Time{})
		dst.resetSendHoldTimer()
		dst.writeMu.Unlock()
	}
	for i := kept; i < len(s.fwdDirty); i++ {
		s.fwdDirty[i] = nil
	}
	s.fwdDirty = s.fwdDirty[:kept]
}

// SendUpdate sends a pre-built BGP UPDATE message.
// Returns ErrInvalidState if the session is not established.
//
// Uses zero-allocation path via Update.WriteTo and session write buffer.
// Concurrent calls are serialized by writeMu.
func (s *Session) SendUpdate(update *message.Update) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.writeUpdate(update); err != nil {
		return err
	}
	if err := s.flushWrites(); err != nil {
		return err
	}
	s.resetSendHoldTimer()
	return nil
}

// SendAnnounce sends a BGP UPDATE message for announcing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrInvalidState if the session is not established.
//
// RFC 4271 Section 4.3 - UPDATE Message Format.
// RFC 4760 Section 3 - MP_REACH_NLRI for IPv6 routes.
// RFC 7911 - ADD-PATH encoding when addPath is true.
// RFC 6793 - 4-byte AS encoding when asn4 is true.
//
// Concurrent calls are serialized by writeMu.
func (s *Session) SendAnnounce(route bgptypes.RouteSpec, localAS uint32, isIBGP, asn4, addPath bool) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := WriteAnnounceUpdate(s.writeBuf.Buffer(), 0, route, localAS, isIBGP, asn4, addPath)

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		return err
	}
	if err := s.flushWrites(); err != nil {
		return err
	}
	s.resetSendHoldTimer()

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, rpc.DirectionSent, BufHandle{}, nil)
	}

	return nil
}

// SendWithdraw sends a BGP UPDATE message for withdrawing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrInvalidState if the session is not established.
//
// RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn Routes for IPv4).
// RFC 4760 Section 4 - MP_UNREACH_NLRI for IPv6 withdrawals.
// RFC 7911 - ADD-PATH encoding when addPath is true.
//
// Concurrent calls are serialized by writeMu.
func (s *Session) SendWithdraw(prefix netip.Prefix, addPath bool) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := WriteWithdrawUpdate(s.writeBuf.Buffer(), 0, prefix, addPath)

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		return err
	}
	if err := s.flushWrites(); err != nil {
		return err
	}
	s.resetSendHoldTimer()

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, rpc.DirectionSent, BufHandle{}, nil)
	}

	return nil
}

// SendRawUpdateBody sends a pre-encoded UPDATE message body (without BGP header).
// Used for zero-copy forwarding when encoding contexts match.
// Prepends the BGP header with correct length.
// Returns ErrInvalidState if the session is not established.
// Concurrent calls are serialized by writeMu.
func (s *Session) SendRawUpdateBody(body []byte) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.writeRawUpdateBody(body); err != nil {
		return err
	}
	if err := s.flushWrites(); err != nil {
		return err
	}
	s.resetSendHoldTimer()
	return nil
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet (user provides marker+header+body).
// If msgType is non-zero, payload is message body only (we add the header).
// Concurrent calls are serialized by writeMu (when msgType != 0).
func (s *Session) SendRawMessage(msgType uint8, payload []byte) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	if msgType == 0 {
		// Full packet mode - send as-is through bufWriter
		s.writeMu.Lock()
		defer s.writeMu.Unlock()

		if _, err := s.bufWriter.Write(payload); err != nil {
			return err
		}
		if err := s.bufWriter.Flush(); err != nil {
			return err
		}
		s.resetSendHoldTimer()
		return nil
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Message body mode - write header + body into session buffer
	totalLen := message.HeaderLen + len(payload)
	s.writeBuf.Reset()
	buf := s.writeBuf.Buffer()

	// Marker (16 bytes of 0xFF)
	copy(buf[:message.MarkerLen], message.Marker[:])

	// Length (2 bytes, big-endian)
	buf[16] = byte(totalLen >> 8)
	buf[17] = byte(totalLen)

	// Type (1 byte)
	buf[18] = msgType

	// Body
	copy(buf[message.HeaderLen:], payload)

	if _, err := s.bufWriter.Write(buf[:totalLen]); err != nil {
		return err
	}
	if err := s.bufWriter.Flush(); err != nil {
		return err
	}
	s.resetSendHoldTimer()
	return nil
}
