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

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/wireu"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

	// Protocol event capture tee (capture_replay.go). Placed here, on the
	// complete wire message and BEFORE processMessage, because everything
	// downstream can rewrite or short-circuit it: RFC 7606 enforcement
	// tombstones attributes and synthesizes withdrawals, and the message
	// observer hook fires only after that. A capture exists to record what the
	// peer actually sent, which is exactly what the observer hook cannot see.
	s.teeCapture(uint8(hdr.Type), buf.Buf[:hdr.Length])

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
	if hdr.Type == msgtype.TypeUPDATE {
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
					s.onMessageReceived(s.settings.Address, msgtype.TypeUPDATE, extra, extraWU,
						ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: extra}, nil, "")
				}
			}
			wireUpdate = primary
		}

		// RFC 6793 Sections 4.1 and 4.2.3: the AS-path family is reconciled to
		// four-octet truth ONCE, here, and every consumer below reads that one
		// answer -- the ingress filters, the forward cache, the RIB and both
		// forward rails. It runs AFTER enforceRFC7606, which judges what the
		// PEER sent and must not judge ze's own rewrite, and BEFORE the import
		// policy chain, which reads the AS path and must read the truth.
		collapsed, collapseErr := s.collapseASPathFamily(wireUpdate)
		if collapseErr != nil {
			// The UPDATE is dropped, not answered with a NOTIFICATION: RFC 7606
			// has already ruled on these attributes and found them acceptable,
			// so the session survives exactly as it does for the family drop and
			// the prefix-limit drop below. What must not happen is dispatch: a
			// payload whose AS path is half rewritten reaches every consumer.
			// The FSM handler for EventUpdateMsg restarts the HoldTimer per RFC
			// 4271 Section 8.2.2 Event 27.
			sessionLogger().Error("dropped an UPDATE whose AS path family could not be reconciled",
				"peer", s.settings.Address, "error", collapseErr)
			s.logFSMEvent(fsm.EventUpdateMsg)
			return nil, false
		}
		wireUpdate = collapsed

		// ActionNone or ActionAttributeDiscard: continue to dispatch.
		// For attribute-discard, the malformed attributes are logged but the
		// UPDATE is still dispatched — the attribute bytes are still present
		// in the wire format, but plugins receiving this UPDATE should not
		// rely on the discarded attribute values for route selection.
		//
		// Loop detection (RFC 4271 S9, RFC 4456 S8) runs as an ingress filter
		// in the reactor's message receiver callback (plugins/loop package).
		famDrop, famErr := s.validateUpdateFamilies(wireUpdate.Payload())
		if famErr != nil {
			if errors.Is(famErr, ErrFamilyNotNegotiated) {
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
			return famErr, false
		}
		if famDrop {
			// The operator asked to ignore this family, so Section 7's other two
			// remedies apply instead of the session: no route of that AFI/SAFI is
			// taken, and the peering survives. Same shape as the prefix-limit drop
			// below, including the HoldTimer restart RFC 4271 Section 8.2.2
			// Event 27 owes a well-formed UPDATE.
			s.logFSMEvent(fsm.EventUpdateMsg)
			return nil, false
		}
	}

	// RFC 4486: Check prefix limits BEFORE delivering to plugins.
	// Over-limit UPDATEs must not reach the RIB or be forwarded.
	if hdr.Type == msgtype.TypeUPDATE && wireUpdate != nil {
		prefixNotif, prefixDrop := s.checkPrefixLimits(wireUpdate)
		if prefixNotif != nil {
			// teardown=true: send NOTIFICATION and close session.
			s.mu.RLock()
			conn := s.conn
			s.mu.RUnlock()
			s.logNotifyErr(conn, prefixNotif.ErrorCode, prefixNotif.ErrorSubcode, prefixNotif.Data)
			s.logFSMEvent(fsm.EventNotifMsg)
			s.closeConn()
			// The cause names the offending family. peer_run.go reads that
			// family's own idle-timeout, and the operator log says which
			// family stopped the session.
			return fmt.Errorf("%w: %w", ErrConnectionClosed, s.prefixTeardownCause()), false
		}
		if prefixDrop {
			// AC-27: teardown=false, exceeded. Skip plugin delivery but keep session.
			// The UPDATE is consumed but not forwarded. What its withdrawals did to
			// the count depends on the family's `count` mode: an `offered` family
			// keeps them, an `installed` family had them rolled back by
			// checkPrefixLimits, because this message reached no RIB.
			// The FSM handler for EventUpdateMsg restarts the HoldTimer per §8.2.2 Event 27.
			s.logFSMEvent(fsm.EventUpdateMsg)
			return nil, false
		}
	}

	// Validate rejectable ROUTE-REFRESH wire shape before callback delivery, so
	// malformed peer input never reaches a plugin. Which NOTIFICATION a bad body
	// length earns is RFC-scoped, and validateRouteRefreshLength
	// (session_handlers.go) owns that decision for both of its call sites. A body
	// RFC 7313 Section 5 obliges ze to ignore stops here too, without a
	// NOTIFICATION and without ending the session.
	if hdr.Type == msgtype.TypeROUTEREFRESH {
		ignore, refreshErr := s.validateRouteRefreshLength(body)
		if refreshErr != nil {
			return refreshErr, false
		}
		if ignore {
			return nil, false
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
		// Record the reason BEFORE closing: closeConn nils s.conn, and a Run
		// loop that reaches its conn == nil branch with no close reason set
		// sleeps 10 ms and retries forever (session.go:901-908). Returning the
		// sentinel is what actually exits Run on both read rails; the close
		// reason is the belt to that braces, and is what Run reports if the
		// cancel goroutine wins the race to closeConn first.
		s.setCloseReason(ErrPolicyTeardown)
		s.closeConn()
		return ErrPolicyTeardown, kept
	}

	var err error
	switch hdr.Type { //nolint:exhaustive // unknown in default
	case msgtype.TypeUPDATE:
		err = s.handleUpdate(wireUpdate)
	case msgtype.TypeOPEN:
		err = s.handleOpen(body)
	case msgtype.TypeKEEPALIVE:
		err = s.handleKeepalive()
	case msgtype.TypeNOTIFICATION:
		err = s.handleNotification(body)
	case msgtype.TypeROUTEREFRESH:
		err = s.handleRouteRefresh(body)
	default:
		err = s.handleUnknownType(hdr.Type)
	}
	return err, kept
}

// collapseASPathFamily reconciles the AS-path family of a received UPDATE to
// four-octet truth, and answers the WireUpdate the rest of the receive path
// carries.
//
// RFC 6793 Section 4.2.3 constructs the AS path information from a received
// AS_PATH and AS4_PATH, and Section 4.1 forbids either AS4 attribute in an
// UPDATE between NEW BGP speakers. Running both once, here, is what lets every
// consumer downstream read one canonical path rather than resolve the pair for
// itself. FRR (aspath_reconcile_as4, from bgp_attr_parse) and BIRD
// (bgp_process_as4_attrs) both place it on the receive decode for the same
// reason.
//
// The answer is wireUpdate itself whenever nothing is owed, which is every
// UPDATE a NEW BGP speaker sends. That path allocates nothing and parses no
// AS_PATH, so a four-octet fleet pays nothing for the transition machinery.
//
// A rewritten payload is carried by a NEW WireUpdate, because Attrs FREEZES the
// attribute index of the one it replaces, and it is labeled with a context
// reporting four octets: the bytes are four-octet while the session's receive
// context still describes the wire, and every consumer reads the width from the
// payload's own context.
//
// The error means no canonical AS path exists. The caller MUST drop the UPDATE
// on it and MUST NOT dispatch: half of a rewritten AS path is a malformed
// attribute value toward every peer at once.
func (s *Session) collapseASPathFamily(wireUpdate *wireu.WireUpdate) (*wireu.WireUpdate, error) {
	// The width is read from the context the PAYLOAD carries, never from the
	// negotiated capability, so it describes the bytes rather than the session.
	// rib_structured.go reads the same field the same way, missing context
	// included: with no width to read there is nothing to reconcile against, so
	// the UPDATE travels on exactly as it arrived.
	srcCtx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	srcASN4 := srcCtx == nil || srcCtx.ASN4()

	if srcASN4 && !carriesAS4Attributes(wireUpdate) {
		return wireUpdate, nil
	}

	payload := wireUpdate.Payload()
	dst := make([]byte, wireu.CollapseAS4FamilySize(payload))
	n, discards, err := wireu.CollapseAS4Family(dst, payload, srcASN4)
	if err != nil {
		return nil, fmt.Errorf("reconcile the AS path family: %w", err)
	}

	for _, discard := range discards {
		// RFC 6793 Section 6: "The error SHOULD be logged locally for
		// analysis." The attribute package holds no logger and takes none, so
		// each caller writes the line under its own subsystem and this one is
		// the session's.
		sessionLogger().Warn("discarded a received AS4 attribute",
			"peer", s.settings.Address, "attribute", discard.Code, "reason", discard.Reason)
	}

	if n == 0 {
		return wireUpdate, nil
	}

	// Capped at n so nothing appends into the headroom CollapseAS4FamilySize
	// reserved for the widest possible reconciliation.
	collapsed := wireu.NewWireUpdate(dst[:n:n], fwdContextIDWithASN4(wireUpdate.SourceCtxID(), true))
	collapsed.SetSourceID(wireUpdate.SourceID())
	return collapsed, nil
}

// carriesAS4Attributes reports whether a received UPDATE holds an AS4_PATH or
// an AS4_AGGREGATOR, which is the only reason an UPDATE from a NEW BGP speaker
// owes the RFC 6793 reconciliation.
//
// It answers from the span index enforceRFC7606 already built, so the question
// costs two bit tests and no allocation. An attribute section that did not
// index answers false: enforceRFC7606 owns the verdict on such a section, and
// the reconciliation adds none of its own.
func carriesAS4Attributes(wireUpdate *wireu.WireUpdate) bool {
	attrs, err := wireUpdate.Attrs()
	if err != nil || attrs == nil {
		return false
	}
	hasAS4Path, err := attrs.Has(attribute.AttrAS4Path)
	if err != nil {
		return false
	}
	if hasAS4Path {
		return true
	}
	hasAS4Aggregator, err := attrs.Has(attribute.AttrAS4Aggregator)
	if err != nil {
		return false
	}
	return hasAS4Aggregator
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
