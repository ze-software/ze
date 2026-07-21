// Design: docs/architecture/core-design.md — BGP message read loop
// RFC: rfc/short/rfc7606.md — UPDATE error handling before plugin dispatch
// Overview: session.go — BGP session struct and lifecycle
// Related: session_write.go — wire write primitives and Send* methods
// Related: session_connection.go — session connect, accept, teardown
// Related: session_prefix.go — prefix limit check before plugin delivery

package reactor

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// ReadAndProcess reads and processes a single message.
// Exposed for testing.
func (s *Session) ReadAndProcess() error {
	// Capture conn + bufReader atomically. connectionEstablished writes
	// both under s.mu.Lock(); readers MUST take s.mu.RLock() to get a
	// consistent view. Without capturing bufReader here, the direct field
	// read inside readAndProcessMessage would race the locked write in
	// connectionEstablished.
	s.mu.RLock()
	conn := s.conn
	bufReader := s.bufReader
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	// Set read deadline.
	_ = conn.SetReadDeadline(s.clock.Now().Add(5 * time.Second))

	return s.readAndProcessMessage(conn, bufReader)
}

// readAndProcessMessage reads a message from the connection and processes it.
// Uses clean get/return pool pattern for buffer lifecycle.
//
// bufReader is passed as a parameter rather than read from s.bufReader so
// that the caller captures conn + bufReader together under a single RLock,
// making them a consistent pair relative to connectionEstablished's locked
// write. Reading s.bufReader directly here would be a data race with the
// locked write in session_connection.go.
func (s *Session) readAndProcessMessage(conn net.Conn, bufReader *bufio.Reader) error {
	// Get buffer from multiplexer.
	buf := s.getReadBuffer()
	if buf.Buf == nil {
		return errReadBufferExhaustedPoolAtMaximum
	}

	// Defer ensures buffer is returned even if processMessage panics.
	// Set to true when callback takes ownership (cache keeps the buffer).
	kept := false
	defer func() {
		if !kept {
			s.returnReadBuffer(buf)
		}
	}()

	// Read header -- through bufio.Reader to batch kernel read syscalls.
	_, err := io.ReadFull(bufReader, buf.Buf[:message.HeaderLen])
	if err != nil {
		// Handle connection close: EOF or connection reset by peer.
		// Clean close does not increment wireReadErrors (not an error).
		if errors.Is(err, io.EOF) || isConnectionReset(err) {
			s.handleConnectionClose()
			return ErrConnectionClosed
		}
		// Actual read error (timeout, network failure): count it.
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireReadErrors.With(s.addrLabel).Inc()
		}
		return err
	}

	// Mark that data was read. Used by hold timer congestion extension:
	// if the hold timer fires while recentRead is true, the daemon is
	// CPU-congested (data arrived but wasn't processed in time).
	s.recentRead.Store(true)
	if s.onRead != nil {
		s.onRead()
	}

	hdr, err := message.ParseHeader(buf.Buf[:message.HeaderLen])
	if err != nil {
		s.logFSMEvent(fsm.EventBGPHeaderErr)
		return fmt.Errorf("parse header: %w", err)
	}

	// RFC 8654: Validate message length against max (4096 or 65535 if extended).
	if err := hdr.ValidateLengthWithMax(s.extendedMessage); err != nil {
		// RFC 8654 Section 5: Send NOTIFICATION with Bad Message Length.
		var lengthBuf [2]byte
		binary.BigEndian.PutUint16(lengthBuf[:], hdr.Length)
		s.logNotifyErr(conn,
			message.NotifyMessageHeader,
			message.NotifyHeaderBadLength,
			lengthBuf[:],
		)
		s.logFSMEvent(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("message length %d exceeds max for %s: %w", hdr.Length, hdr.Type, err)
	}

	// Read body
	bodyLen := int(hdr.Length) - message.HeaderLen
	if bodyLen > 0 {
		_, err = io.ReadFull(bufReader, buf.Buf[message.HeaderLen:hdr.Length])
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
	}

	// Track wire bytes received.
	if s.prefixMetrics != nil {
		s.prefixMetrics.wireBytesRecv.With(s.addrLabel).Add(float64(hdr.Length))
	}

	// Process message - callback returns kept=true if it took buffer ownership
	var processErr error
	processErr, kept = s.processMessage(&hdr, buf.Buf[message.HeaderLen:hdr.Length], buf)

	return processErr
}

// processMessage handles a received BGP message.
// Returns (error, kept) where kept indicates if callback took buffer ownership.
func (s *Session) processMessage(hdr *message.Header, body []byte, buf BufHandle) (error, bool) {
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
		// enforceRFC7606 returns the UPDATE already rewritten for attribute-discard
		// (attributes tombstoned); treat-as-withdraw synthesis is handled below because it
		// is negotiation-aware and may produce more than one UPDATE.
		var err error
		var action message.RFC7606Action
		wireUpdate, action, err = s.enforceRFC7606(wireUpdate)
		if err != nil {
			// session-reset: error propagated, no dispatch
			return err, false
		}

		if action == message.RFC7606ActionTreatAsWithdraw {
			// RFC 7606 Section 2: "MUST be handled as though all of the routes contained in
			// an UPDATE message ... had been withdrawn". Turn the announced routes into
			// withdrawals so the malformed UPDATE removes them from the Adj-RIB-In instead of
			// leaving a previously-announced prefix installed and stale. Synthesis is
			// negotiation-aware (D-5) and splits two MP families across two UPDATEs (RFC 7606
			// Section 3.g: one MP_UNREACH per UPDATE; D-8).
			//
			// This branch used to return here without dispatching. That is NOT
			// treat-as-withdraw: the re-announced prefix kept its old entry and went stale.
			// RFC 4271 Section 8.2.2 Event 27 (UpdateMsg) is still satisfied: the primary
			// rides the normal path, whose FSM handler restarts the HoldTimer.
			bodies := message.SynthesizeWithdrawFamilies(wireUpdate.Payload(), s.mpFamilyDispatchable)
			if len(bodies) == 0 {
				// Nothing left to withdraw: an UPDATE whose only routes belong to a
				// non-negotiated MP family (RFC 7606 Section 3.j: nothing is installed to
				// remove) is dropped exactly as before treat-as-withdraw dispatch existed --
				// no NOTIFICATION, no teardown (D-5). The FSM handler still restarts the
				// HoldTimer per RFC 4271 Section 8.2.2 Event 27.
				s.logFSMEvent(fsm.EventUpdateMsg)
				return nil, false
			}
			// The primary body rides the unchanged normal dispatch path below (ctxID and
			// sourceID preserved so ADD-PATH decoding in the RIB is unaffected, D-6).
			primary := wireu.NewWireUpdate(bodies[0], wireUpdate.SourceCtxID())
			primary.SetSourceID(wireUpdate.SourceID())
			// Each further MP family rides its own withdraw-only UPDATE. It carries a
			// non-pool BufHandle (noPoolBufID sentinel) over its own heap-allocated body so
			// the receive path enters it in the recentUpdates forward cache exactly like the
			// primary (reactor_notify.go: the cache gate requires buf.Buf != nil). A route
			// server forwarding this body -- the reactor RS fast path, or the rs plugin's
			// ForwardCached -> ForwardUpdatesDirect -- then finds its cache entry instead of
			// missing it, which would log a false "BUG: ForwardUpdatesDirect: msgID missing
			// from cache" and silently drop the second family's withdrawal (D-8).
			//
			// Ownership: buildWithdrawBody makes each body a fresh allocation, never a slice
			// into the session pool buffer, so it cannot alias the primary's buffer. The
			// sentinel makes cache eviction's ReturnReadBuffer a no-op (the body is GC-owned,
			// not a pool slot), so there is no double-free and no pool slot is consumed (D-7).
			for _, extra := range bodies[1:] {
				if s.onMessageReceived != nil {
					extraWU := wireu.NewWireUpdate(extra, wireUpdate.SourceCtxID())
					extraWU.SetSourceID(wireUpdate.SourceID())
					s.onMessageReceived(s.settings.Address, message.TypeUPDATE, extra, extraWU,
						ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: extra}, nil, "")
				}
			}
			wireUpdate = primary
		}

		// ActionNone or ActionAttributeDiscard: continue to dispatch.
		// For attribute-discard, the malformed attributes are logged but the
		// UPDATE is still dispatched — the attribute bytes are still present
		// in the wire format, but plugins receiving this UPDATE should not
		// rely on the discarded attribute values for route selection.
		//
		// Loop detection (RFC 4271 S9, RFC 4456 S8) runs as an ingress filter
		// in the reactor's message receiver callback (plugins/loop package).
		if err := s.validateUpdateFamilies(wireUpdate.Payload()); err != nil {
			if errors.Is(err, ErrFamilyNotNegotiated) {
				s.mu.RLock()
				conn := s.conn
				s.mu.RUnlock()

				// RFC 4760 Section 7: "The session SHOULD be terminated with the
				// Notification message code/subcode indicating 'UPDATE Message Error'/
				// 'Optional Attribute Error'."
				s.logNotifyErr(conn, message.NotifyUpdateMessage, message.NotifyUpdateOptionalAttr, nil)
				s.logFSMEvent(fsm.EventUpdateMsgErr)
				s.closeConn()
			}
			return err, false
		}
	}

	// RFC 4486: Check prefix limits BEFORE delivering to plugins.
	// Over-limit UPDATEs must not reach the RIB or be forwarded.
	if hdr.Type == message.TypeUPDATE && wireUpdate != nil {
		prefixNotif, prefixDrop := s.checkPrefixLimits(wireUpdate)
		if prefixNotif != nil {
			// teardown=true: send NOTIFICATION and close session.
			s.mu.RLock()
			conn := s.conn
			s.mu.RUnlock()
			s.logNotifyErr(conn, prefixNotif.ErrorCode, prefixNotif.ErrorSubcode, prefixNotif.Data)
			s.logFSMEvent(fsm.EventNotifMsg)
			s.closeConn()
			return fmt.Errorf("%w: %w", ErrConnectionClosed, ErrPrefixLimitExceeded), false
		}
		if prefixDrop {
			// AC-27: teardown=false, exceeded. Skip plugin delivery but keep session.
			// Withdrawals were already counted. The UPDATE is consumed but not forwarded.
			// The FSM handler for EventUpdateMsg restarts the HoldTimer per §8.2.2 Event 27.
			s.logFSMEvent(fsm.EventUpdateMsg)
			return nil, false
		}
	}

	// Validate rejectable ROUTE-REFRESH wire shape before callback delivery.
	// RFC 7313 Section 5 requires invalid body lengths to produce a
	// ROUTE-REFRESH Message Error; malformed peer input must not reach plugins.
	if hdr.Type == message.TypeROUTEREFRESH {
		if err := s.validateRouteRefreshLength(body, routeRefreshNotificationData(body)); err != nil {
			return err, false
		}
	}

	// Notify callback after pre-delivery validation for rejectable messages.
	// Callback returns true if it took ownership of buf (e.g., cached it).
	var kept bool
	if s.onMessageReceived != nil {
		kept = s.onMessageReceived(s.settings.Address, hdr.Type, body, wireUpdate, ctxID, rpc.DirectionReceived, buf, nil, "")
	}

	// A policy filter on the import chain (e.g. filter_family tear-down) may have
	// requested a session teardown during the callback. Honor it here, on the
	// session read goroutine, before dispatching the UPDATE — mirroring the
	// family-not-negotiated teardown above (RFC 4760 §7). Short-circuits
	// handleUpdate so the FSM is not advanced for a session being closed.
	if req := s.takePolicyTeardown(); req != nil {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		s.logNotifyErr(conn, req.code, req.subcode, nil)
		s.logFSMEvent(fsm.EventUpdateMsgErr)
		s.closeConn()
		return nil, kept
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
	s.logFSMEvent(fsm.EventTCPConnectionFails)
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
