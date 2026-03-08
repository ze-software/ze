// Design: docs/architecture/core-design.md — wire write primitives and Send* methods
// Overview: session.go — BGP session struct and lifecycle
// Related: session_read.go — inbound message reading (symmetric counterpart)
// Related: session_connection.go — session connect, accept, teardown

package reactor

import (
	"net"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// sendKeepalive sends a KEEPALIVE message.
func (s *Session) sendKeepalive(conn net.Conn) error {
	return s.writeMessage(conn, message.NewKeepalive())
}

// sendNotification sends a NOTIFICATION message.
func (s *Session) sendNotification(conn net.Conn, code message.NotifyErrorCode, subcode uint8, data []byte) error {
	notif := &message.Notification{
		ErrorCode:    code,
		ErrorSubcode: subcode,
		Data:         data,
	}
	return s.writeMessage(conn, notif)
}

// writeMessage writes a BGP message directly into the session buffer.
// Always flushes immediately — used for KEEPALIVE, OPEN, NOTIFICATION.
func (s *Session) writeMessage(conn net.Conn, msg message.Message) error {
	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Zero-allocation: write directly into session buffer.
	// Message types don't use context for basic encoding
	// (KEEPALIVE, OPEN, NOTIFICATION have fixed formats).
	s.writeBuf.Reset()
	n := msg.WriteTo(s.writeBuf.Buffer(), 0, nil)

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		return err
	}
	if err := s.bufWriter.Flush(); err != nil {
		return err
	}

	// Notify callback after successful send.
	// Body is data after the 19-byte header (16-byte marker + 2-byte length + 1-byte type).
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, msg.Type(), body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// TriggerHoldTimerExpiry triggers the hold timer expiry event.
// Exposed for testing.
func (s *Session) TriggerHoldTimerExpiry() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventHoldTimerExpires)
	s.closeConn()
}

// writeUpdate writes an UPDATE to bufWriter without locking or flushing.
// Caller must hold writeMu.
func (s *Session) writeUpdate(update *message.Update) error {
	s.writeBuf.Reset()
	n := update.WriteTo(s.writeBuf.Buffer(), 0, nil) // nil ctx: UPDATE already has wire bytes

	if _, err := s.bufWriter.Write(s.writeBuf.Buffer()[:n]); err != nil {
		return err
	}

	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		sessionLogger().Debug("SendUpdate", "peer", s.settings.Address, "direction", "sent", "ctxID", s.sendCtxID, "msgLen", n)
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// writeRawUpdateBody writes a raw UPDATE body to bufWriter without locking or flushing.
// Caller must hold writeMu.
func (s *Session) writeRawUpdateBody(body []byte) error {
	totalLen := message.HeaderLen + len(body)
	s.writeBuf.Reset()
	buf := s.writeBuf.Buffer()

	copy(buf[:message.MarkerLen], message.Marker[:])
	buf[16] = byte(totalLen >> 8)
	buf[17] = byte(totalLen)
	buf[18] = byte(message.TypeUPDATE)
	copy(buf[message.HeaderLen:], body)

	_, err := s.bufWriter.Write(buf[:totalLen])
	return err
}

// flushWrites flushes the bufWriter. Caller must hold writeMu.
func (s *Session) flushWrites() error {
	return s.bufWriter.Flush()
}

// SendUpdate sends a BGP UPDATE message.
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
	return s.flushWrites()
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

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
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

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
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
	return s.flushWrites()
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
		return s.bufWriter.Flush()
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
	return s.bufWriter.Flush()
}
