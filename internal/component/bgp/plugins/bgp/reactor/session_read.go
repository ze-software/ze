// Design: docs/architecture/core-design.md — BGP message read loop

package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
)

// ReadAndProcess reads and processes a single message.
// Exposed for testing.
func (s *Session) ReadAndProcess() error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	// Set read deadline.
	_ = conn.SetReadDeadline(s.clock.Now().Add(5 * time.Second))

	return s.readAndProcessMessage(conn)
}

// readAndProcessMessage reads a message from the connection and processes it.
// Uses clean get/return pool pattern for buffer lifecycle.
func (s *Session) readAndProcessMessage(conn net.Conn) error {
	// Get buffer from pool
	buf := s.getReadBuffer()

	// Read header — through bufio.Reader to batch kernel read syscalls.
	_, err := io.ReadFull(s.bufReader, buf[:message.HeaderLen])
	if err != nil {
		s.returnReadBuffer(buf)
		// Handle connection close: EOF or connection reset by peer
		if errors.Is(err, io.EOF) || isConnectionReset(err) {
			s.handleConnectionClose()
			return ErrConnectionClosed
		}
		return err
	}

	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	if err != nil {
		s.returnReadBuffer(buf)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		return fmt.Errorf("parse header: %w", err)
	}

	// RFC 8654: Validate message length against max (4096 or 65535 if extended).
	if err := hdr.ValidateLengthWithMax(s.extendedMessage); err != nil {
		s.returnReadBuffer(buf)
		// RFC 8654 Section 5: Send NOTIFICATION with Bad Message Length.
		var lengthBuf [2]byte
		binary.BigEndian.PutUint16(lengthBuf[:], hdr.Length)
		_ = s.sendNotification(conn,
			message.NotifyMessageHeader,
			message.NotifyHeaderBadLength,
			lengthBuf[:],
		)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("message length %d exceeds max for %s: %w", hdr.Length, hdr.Type, err)
	}

	// Read body
	bodyLen := int(hdr.Length) - message.HeaderLen
	if bodyLen > 0 {
		_, err = io.ReadFull(s.bufReader, buf[message.HeaderLen:hdr.Length])
		if err != nil {
			s.returnReadBuffer(buf)
			return fmt.Errorf("read body: %w", err)
		}
	}

	// Process message - callback returns kept=true if it took buffer ownership
	err, kept := s.processMessage(&hdr, buf[message.HeaderLen:hdr.Length], buf)

	// Return buffer to pool only if callback didn't keep it
	if !kept {
		s.returnReadBuffer(buf)
	}

	return err
}

// processMessage handles a received BGP message.
// Returns (error, kept) where kept indicates if callback took buffer ownership.
func (s *Session) processMessage(hdr *message.Header, body, buf []byte) (error, bool) {
	s.mu.RLock()
	ctxID := s.recvCtxID
	sourceID := s.sourceID
	s.mu.RUnlock()

	// For UPDATE: create WireUpdate once, use for callback and handler
	var wireUpdate *wireu.WireUpdate
	if hdr.Type == message.TypeUPDATE {
		wireUpdate = wireu.NewWireUpdate(body, ctxID)
		wireUpdate.SetSourceID(sourceID)

		// RFC 7606: Validate BEFORE dispatching to plugins.
		// Enforcement must happen before callback so malformed UPDATEs
		// are never delivered to plugins as valid routes.
		var action message.RFC7606Action
		var err error
		wireUpdate, action, err = s.enforceRFC7606(wireUpdate)
		if err != nil {
			// session-reset: error propagated, no dispatch
			return err, false
		}
		if action == message.RFC7606ActionTreatAsWithdraw {
			// RFC 7606 Section 2: "MUST be handled as though all of the routes
			// contained in an UPDATE message ... had been withdrawn"
			// Do not dispatch to plugins — the routes are treated as withdrawn.
			s.timers.ResetHoldTimer()
			// RFC 4271 Section 8.2.2: FSM must process Event 27 (UpdateMsg)
			// for any received UPDATE, even if treated as withdrawal.
			_ = s.fsm.Event(fsm.EventUpdateMsg)
			return nil, false
		}
		// ActionNone or ActionAttributeDiscard: continue to dispatch.
		// For attribute-discard, the malformed attributes are logged but the
		// UPDATE is still dispatched — the attribute bytes are still present
		// in the wire format, but plugins receiving this UPDATE should not
		// rely on the discarded attribute values for route selection.
	}

	// Notify callback for all message types.
	// Callback returns true if it took ownership of buf (e.g., cached it).
	var kept bool
	if s.onMessageReceived != nil {
		kept = s.onMessageReceived(s.settings.Address, hdr.Type, body, wireUpdate, ctxID, "received", buf)
	}

	var err error
	switch hdr.Type { //nolint:exhaustive // unknown in default
	case message.TypeUPDATE:
		err = s.handleUpdate(wireUpdate)
	case message.TypeOPEN:
		err = s.handleOpen(body)
	case message.TypeKEEPALIVE:
		err = s.handleKeepalive()
	case message.TypeNOTIFICATION:
		err = s.handleNotification(body)
	case message.TypeROUTEREFRESH:
		err = s.handleRouteRefresh(body)
	default:
		err = s.handleUnknownType(hdr.Type)
	}
	return err, kept
}

// handleConnectionClose handles TCP connection close.
func (s *Session) handleConnectionClose() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventTCPConnectionFails)
	s.closeConn()
}

// isConnectionReset checks if an error is a connection reset by peer.
// This happens when the remote side closes the connection abruptly,
// or when we close the connection ourselves (close-on-cancel pattern).
func isConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	// Check for common connection close errors
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	// Check error message for connection reset indicators
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "use of closed network connection")
}
