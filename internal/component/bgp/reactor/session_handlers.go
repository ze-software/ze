// Design: docs/architecture/core-design.md — BGP message type handlers
// Overview: session.go — BGP session struct, constructor, accessors, run loop
// Related: session_prefix.go — prefix limit enforcement (RFC 4486)

package reactor

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/wireu"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleUnknownType handles unknown message types (exabgp-compatible).
func (s *Session) handleUnknownType(msgType msgtype.MessageType) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	// ExaBGP format: Message Header Error (1), subcode 0, text message.
	errMsg := textbuf.StrIntStr("can not decode update message of type \"", int64(msgType), "\"")
	s.logNotifyErr(conn,
		message.NotifyMessageHeader,
		0, // ExaBGP uses subcode 0
		[]byte(errMsg),
	)
	s.logFSMEvent(fsm.EventBGPHeaderErr)
	s.closeConn()

	return fmt.Errorf("%w: unknown type %d", ErrInvalidMessage, msgType)
}

// handleOpen processes a received OPEN message.
func (s *Session) handleOpen(body []byte) error {
	// An OPEN is only legitimate while we are WAITING for the peer's OPEN. Once
	// we have one, a further OPEN on the SAME connection must not be processed:
	// without this gate handleOpen runs the whole path again on a live session,
	// overwriting s.peerOpen and calling negotiateWith, so a peer could silently
	// change the negotiated capability set (families, AddPath, ASN4) mid-session
	// on an already-established peering.
	//
	// RFC 4271 Section 8.2.2 never routes BGPOpen (Event 19) to the FSM-Error
	// branch: in Established that branch is scoped to "Events 9, 12-13, 20-22"
	// and in OpenConfirm to "Events 9, 12-13, 20, 27-28". Both states instead
	// send Event 19 through collision detection, whose termination action is
	// "sends a NOTIFICATION with a Cease". So Cease is the code, not FSM Error.
	//
	// Honest about the edge: Section 6.8 collision detection is written for two
	// DIFFERENT connections, so a second OPEN on one connection is not literally
	// a collision and RFC 4271 does not name an action for it. Cease is chosen
	// because it is what Section 8.2.2 associates with terminating on Event 19
	// in exactly these two states. What is NOT defensible either way is silently
	// re-negotiating, which is what happened before this gate.
	if state := s.fsm.State(); state == fsm.StateEstablished || state == fsm.StateOpenConfirm {
		// Counted at the refusal, not after the send: what an operator needs to
		// know is that this peer tried to re-negotiate mid-session, which is
		// true whether or not the Cease made it onto a socket the peer may
		// already have abandoned.
		if s.prefixMetrics != nil {
			s.prefixMetrics.openInEstablished.With(s.addrLabel).Inc()
		}
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		s.logNotifyErr(conn, message.NotifyCease, 0, nil)
		// MUST fire the FSM event, not just close the socket. Section 8.2.2's
		// action list for terminating on Event 19 is not only the NOTIFICATION:
		// it also "deletes all routes associated with this connection",
		// "releases all BGP resources" and "changes its state to Idle". In this
		// reactor all three hang off the FSM transition, not off closeConn:
		// peer_run.go's `from == fsm.StateEstablished` branch is what calls
		// stopBFDClient, raiseSessionDropped and notifyPeerClosed, and
		// notifyPeerClosed is the sole producer of the SessionStateDown that
		// makes adj_rib_in drop the peer's stored routes and clear peerUp.
		// Returning here without it would leave a dead peer marked up with its
		// routes retained, and replay them on reconnect. EventBGPOpen lands in
		// the Established/OpenConfirm default arm, which changes state to Idle
		// and returns the ErrFSMError sentinel that logFSMEvent records.
		s.logFSMEvent(fsm.EventBGPOpen)
		s.closeConn()
		return fmt.Errorf("%w: OPEN received in %s", ErrInvalidState, state)
	}

	if s.onOpenRecv != nil {
		s.onOpenRecv()
	}
	open, err := message.UnpackOpen(body)
	if err != nil {
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		return fmt.Errorf("unpack OPEN: %w", err)
	}

	// Validate version.
	if open.Version != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		s.logNotifyErr(conn,
			message.NotifyOpenMessage,
			message.NotifyOpenUnsupportedVersion,
			[]byte{4}, // We support version 4
		)
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return ErrUnsupportedVersion
	}

	// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values
	// of one or two seconds."
	if err := open.ValidateHoldTime(); err != nil {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		// Send NOTIFICATION with the error (already a *Notification).
		if notif, ok := errors.AsType[*message.Notification](err); ok {
			s.logNotifyErr(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	// RFC 6286 Section 2.2: reject a zero BGP Identifier, or this speaker's own identifier
	// from an internal peer, with OPEN Message Error / Bad BGP Identifier.
	if err := s.validateOpenIdentifier(open); err != nil {
		return err
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Validate OPEN pair via the peer callback (plugins such as RFC 9234 Role, plus the
	// RFC 6286 Section 2.1 AS-wide identifier claim).
	// Called BEFORE negotiation — saves work if rejected.
	if err := s.runOpenValidator(open); err != nil {
		return err
	}

	s.mu.RLock()
	localOpen := s.localOpen
	s.mu.RUnlock()

	// Parse capabilities from both OPENs for negotiation.
	var localCaps, peerCaps []capability.Capability
	if localOpen != nil {
		localCaps, err = capability.ParseFromOptionalParams(localOpen.OptionalParams)
		if err != nil {
			return fmt.Errorf("parse local OPEN capabilities: %w", err)
		}
	}
	peerCaps, err = capability.ParseFromOptionalParams(open.OptionalParams)
	if err != nil {
		return s.rejectOpenCapabilityError(err)
	}

	// Negotiate capabilities.
	s.negotiateWith(localCaps, peerCaps)

	// Validate required families and capabilities are negotiated.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	requiredCaps := s.settings.RequiredCapabilities
	refusedCaps := s.settings.RefusedCapabilities
	s.mu.RUnlock()

	if len(requiredFamilies) > 0 && neg != nil {
		if missing := neg.CheckRequired(requiredFamilies); len(missing) > 0 {
			// Required families not negotiated - send NOTIFICATION and reject.
			// RFC 5492 Section 3: Use Unsupported Capability subcode.
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

	// Update FSM.
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE to confirm.
	if err := s.sendKeepalive(conn); err != nil {
		return err
	}

	// Reset and restart hold timer with negotiated value.
	s.timers.ResetHoldTimer()

	return nil
}

func (s *Session) rejectOpenCapabilityError(err error) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	subcode := message.NotifyOpenUnsupportedOptParam
	data := []byte(nil)
	if errors.Is(err, capability.ErrInvalidLength) {
		subcode = message.NotifyOpenUnsupportedCapability
		// RFC 5492 Section 5: Unsupported Capability NOTIFICATION data
		// MUST list the capability TLV that caused the notification.
		data = capability.ErrorData(err)
	}

	s.logNotifyErr(conn, message.NotifyOpenMessage, subcode, data)
	s.logFSMEvent(fsm.EventBGPOpenMsgErr)
	s.closeConn()
	return fmt.Errorf("%w: parse peer OPEN capabilities: %w", ErrInvalidMessage, err)
}

// handleKeepalive processes a received KEEPALIVE message.
// RFC 4271 §8.2.2 Event 26: the HoldTimer restart is done by the FSM
// inside handleOpenConfirm / handleEstablished when EventKeepaliveMsg
// fires. This function only drives the OpenConfirm -> Established
// side-effects (start keepalive + send hold timers) that the FSM
// layer deliberately does not own.
func (s *Session) handleKeepalive() error {
	state := s.fsm.State()
	if state == fsm.StateOpenConfirm {
		// Start keepalive timer for sending our keepalives.
		s.timers.StartKeepaliveTimer()
		// Start RFC 9687 Send Hold Timer: detects when we cannot send
		// any data to the peer (stuck TCP). Resets on every successful write.
		s.startSendHoldTimer()
	}

	return s.fsm.Event(fsm.EventKeepaliveMsg)
}

// handleUpdate processes a received UPDATE message.
// RFC 4760 Section 6: validates AFI/SAFI in MP_REACH/MP_UNREACH against negotiated.
// RFC 7606 validation is done earlier in processMessage() via enforceRFC7606().
// Accepts WireUpdate for zero-copy processing.
//
// RFC 4271 §8.2.2 Event 27: the HoldTimer restart ("restarts its HoldTimer,
// if the negotiated HoldTime value is non-zero") is performed inside the
// FSM handler when EventUpdateMsg fires, not here. This gives the FSM
// event a real job and keeps the liveness rule in one place.
func (s *Session) handleUpdate(wu *wireu.WireUpdate) error {
	// Get raw payload for validation (zero-copy slice)
	body := wu.Payload()

	// Validate address families in UPDATE.
	if err := s.validateUpdateFamilies(body); err != nil {
		return err
	}

	// Prefix limits are checked in processMessage() BEFORE plugin delivery.
	// By the time handleUpdate runs, the UPDATE has already passed the prefix check.

	return s.fsm.Event(fsm.EventUpdateMsg)
}

// handleNotification processes a received NOTIFICATION message.
// RFC 8203: logs shutdown communication for Cease/Admin Shutdown and Admin Reset.
func (s *Session) handleNotification(body []byte) error {
	notif, err := message.UnpackNotification(body)
	if err != nil {
		s.logFSMEvent(fsm.EventNotifMsgVerErr)
		return fmt.Errorf("unpack NOTIFICATION: %w", err)
	}

	if s.onNotifRecv != nil {
		s.onNotifRecv(uint8(notif.ErrorCode), notif.ErrorSubcode)
	}

	// RFC 8203 Section 2: log shutdown communication message if present.
	if msg, msgErr := notif.ShutdownMessage(); msgErr == nil && msg != "" {
		sessionLogger().Info("peer shutdown communication",
			"peer", s.settings.Address,
			"subcode", message.CeaseSubcodeString(notif.ErrorSubcode),
			"message", msg,
		)
	} else if msgErr != nil {
		sessionLogger().Warn("invalid shutdown communication",
			"peer", s.settings.Address,
			"error", msgErr,
		)
	}

	s.timers.StopAll()
	s.logFSMEvent(fsm.EventNotifMsg)
	s.closeConn()

	return fmt.Errorf("%w: %s", ErrNotificationRecv, notif.String())
}

func (s *Session) validateRouteRefreshLength(body, notificationData []byte) error {
	// RFC 7313 Section 5: "If the length... is not 4, then the BGP speaker
	// MUST send a NOTIFICATION message with Error Code 'ROUTE-REFRESH Message
	// Error' and subcode 'Invalid Message Length'."
	if len(body) == 4 {
		return nil
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if notificationData == nil {
		notificationData = routeRefreshNotificationData(body)
	}
	s.logNotifyErr(conn,
		message.NotifyRouteRefresh,
		message.NotifyRouteRefreshInvalidLength,
		notificationData,
	)
	s.logFSMEvent(fsm.EventBGPHeaderErr)
	s.closeConn()
	return fmt.Errorf("%w: ROUTE-REFRESH invalid length %d", ErrInvalidMessage, len(body))
}

func routeRefreshNotificationData(body []byte) []byte {
	data := make([]byte, message.HeaderLen+len(body))
	header := message.Header{Length: uint16(len(data)), Type: msgtype.TypeROUTEREFRESH} //nolint:gosec // received BGP message length is uint16-bounded.
	header.WriteTo(data, 0)
	copy(data[message.HeaderLen:], body)
	return data
}

// handleRouteRefresh processes a received ROUTE-REFRESH message.
// RFC 2918 Section 3: "A BGP speaker that is willing to receive the
// ROUTE-REFRESH message from its peer SHOULD advertise the Route Refresh
// Capability to the peer using BGP Capabilities advertisement."
// RFC 2918 Section 4: The receiver SHOULD ignore ROUTE-REFRESH for AFI/SAFI
// that were not advertised in the OPEN message.
// RFC 7313: Enhanced Route Refresh with BoRR/EoRR markers.
func (s *Session) handleRouteRefresh(body []byte) error {
	if err := s.validateRouteRefreshLength(body, nil); err != nil {
		return err
	}
	if s.onRefreshRecv != nil {
		s.onRefreshRecv()
	}

	rr, err := message.UnpackRouteRefresh(body)
	if err != nil {
		return fmt.Errorf("unpack ROUTE-REFRESH: %w", err)
	}

	// Cannot process ROUTE-REFRESH before capabilities are negotiated.
	if s.negotiated == nil {
		sessionLogger().Debug("ignoring route-refresh before negotiation complete",
			"peer", s.settings.Address)
		return nil
	}

	// RFC 2918 Section 3: Only process ROUTE-REFRESH if the capability was negotiated.
	if !s.negotiated.RouteRefresh {
		sessionLogger().Debug("ignoring route-refresh from peer without capability",
			"peer", s.settings.Address)
		return nil
	}

	// RFC 2918 Section 4: Ignore ROUTE-REFRESH for AFI/SAFI not negotiated.
	fam := capability.Family{AFI: rr.AFI, SAFI: rr.SAFI}
	if !s.negotiated.SupportsFamily(fam) {
		sessionLogger().Debug("ignoring route-refresh for non-negotiated family",
			"peer", s.settings.Address, "afi", rr.AFI, "safi", rr.SAFI)
		return nil
	}

	// RFC 7313 Section 5: "When the BGP speaker receives a ROUTE-REFRESH message
	// with a 'Message Subtype' field other than 0, 1, or 2, it MUST ignore
	// the received ROUTE-REFRESH message."
	if rr.Subtype > 2 && rr.Subtype != 255 {
		sessionLogger().Debug("ignoring unknown route-refresh subtype", "peer", s.settings.Address, "subtype", rr.Subtype)
		return nil
	}

	// Subtype 255 is reserved - also ignore
	if rr.Subtype == 255 {
		sessionLogger().Debug("ignoring reserved route-refresh subtype", "peer", s.settings.Address, "subtype", 255)
		return nil
	}

	// Valid subtypes 0, 1, 2 are handled via onMessageReceived callback
	// which already forwarded the message to the API before this handler runs.
	// No additional action needed here - the API processes refresh/borr/eorr events.
	return nil
}

// shouldIgnoreFamily checks if UPDATE validation should be lenient for a family.
// Returns true if the family was configured with "ignore" mode.
func (s *Session) shouldIgnoreFamily(fam capability.Family) bool {
	for _, f := range s.settings.IgnoreFamilies {
		if f.AFI == fam.AFI && f.SAFI == fam.SAFI {
			return true
		}
	}
	return false
}
