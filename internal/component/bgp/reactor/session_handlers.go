// Design: docs/architecture/core-design.md — BGP message type handlers
// Overview: session.go — BGP session struct, constructor, accessors, run loop

package reactor

import (
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// handleUnknownType handles unknown message types (exabgp-compatible).
func (s *Session) handleUnknownType(msgType message.MessageType) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	// ExaBGP format: Message Header Error (1), subcode 0, text message.
	errMsg := fmt.Sprintf("can not decode update message of type \"%d\"", msgType)
	_ = s.sendNotification(conn,
		message.NotifyMessageHeader,
		0, // ExaBGP uses subcode 0
		[]byte(errMsg),
	)
	_ = s.fsm.Event(fsm.EventBGPHeaderErr)
	s.closeConn()

	return fmt.Errorf("%w: unknown type %d", ErrInvalidMessage, msgType)
}

// handleOpen processes a received OPEN message.
func (s *Session) handleOpen(body []byte) error {
	open, err := message.UnpackOpen(body)
	if err != nil {
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		return fmt.Errorf("unpack OPEN: %w", err)
	}

	// Validate version.
	if open.Version != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		_ = s.sendNotification(conn,
			message.NotifyOpenMessage,
			message.NotifyOpenUnsupportedVersion,
			[]byte{4}, // We support version 4
		)
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
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
		var notif *message.Notification
		if errors.As(err, &notif) {
			_ = s.sendNotification(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Validate OPEN pair via plugins (e.g., RFC 9234 Role validation).
	// Called BEFORE negotiation — saves work if rejected.
	s.mu.RLock()
	localOpen := s.localOpen
	s.mu.RUnlock()

	if s.openValidator != nil && localOpen != nil {
		if err := s.openValidator(s.settings.Address.String(), localOpen, open); err != nil {
			s.mu.RLock()
			valConn := s.conn
			s.mu.RUnlock()

			// Check for OpenValidationError with specific NOTIFICATION codes.
			var valErr interface{ NotifyCodes() (uint8, uint8) }
			notifyCode := message.NotifyOpenMessage
			notifySubcode := message.NotifyOpenRoleMismatch
			if errors.As(err, &valErr) {
				code, sub := valErr.NotifyCodes()
				notifyCode = message.NotifyErrorCode(code)
				notifySubcode = sub
			}

			_ = s.sendNotification(valConn, notifyCode, notifySubcode, nil)
			return fmt.Errorf("open validation failed: %w", err)
		}
	}

	// Parse capabilities from both OPENs for negotiation.
	var localCaps, peerCaps []capability.Capability
	if localOpen != nil {
		localCaps = capability.ParseFromOptionalParams(localOpen.OptionalParams)
	}
	peerCaps = capability.ParseFromOptionalParams(open.OptionalParams)

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
			_ = s.sendNotification(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required families not negotiated: %v", ErrInvalidState, missing)
		}
	}

	// RFC 5492 Section 3: Validate required/refused capability codes.
	if err := s.validateCapabilityModes(conn, neg, requiredCaps, refusedCaps); err != nil {
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

// handleKeepalive processes a received KEEPALIVE message.
func (s *Session) handleKeepalive() error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

	state := s.fsm.State()
	if state == fsm.StateOpenConfirm {
		// Start keepalive timer for sending our keepalives.
		s.timers.StartKeepaliveTimer()
	}

	return s.fsm.Event(fsm.EventKeepaliveMsg)
}

// handleUpdate processes a received UPDATE message.
// RFC 4760 Section 6: validates AFI/SAFI in MP_REACH/MP_UNREACH against negotiated.
// RFC 7606 validation is done earlier in processMessage() via enforceRFC7606().
// Accepts WireUpdate for zero-copy processing.
func (s *Session) handleUpdate(wu *wireu.WireUpdate) error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

	// Get raw payload for validation (zero-copy slice)
	body := wu.Payload()

	// Validate address families in UPDATE.
	if err := s.validateUpdateFamilies(body); err != nil {
		return err
	}

	return s.fsm.Event(fsm.EventUpdateMsg)
}

// handleNotification processes a received NOTIFICATION message.
func (s *Session) handleNotification(body []byte) error {
	notif, err := message.UnpackNotification(body)
	if err != nil {
		_ = s.fsm.Event(fsm.EventNotifMsgVerErr)
		return fmt.Errorf("unpack NOTIFICATION: %w", err)
	}

	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventNotifMsg)
	s.closeConn()

	return fmt.Errorf("%w: %s", ErrNotificationRecv, notif.String())
}

// handleRouteRefresh processes a received ROUTE-REFRESH message.
// RFC 2918 Section 3: "A BGP speaker that is willing to receive the
// ROUTE-REFRESH message from its peer SHOULD advertise the Route Refresh
// Capability to the peer using BGP Capabilities advertisement."
// RFC 2918 Section 4: The receiver SHOULD ignore ROUTE-REFRESH for AFI/SAFI
// that were not advertised in the OPEN message.
// RFC 7313: Enhanced Route Refresh with BoRR/EoRR markers.
func (s *Session) handleRouteRefresh(body []byte) error {
	// RFC 7313 Section 5: "If the length... is not 4, then the BGP speaker
	// MUST send a NOTIFICATION message with Error Code 'ROUTE-REFRESH Message Error'
	// and subcode 'Invalid Message Length'."
	if len(body) != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		_ = s.sendNotification(conn,
			message.NotifyRouteRefresh,
			message.NotifyRouteRefreshInvalidLength,
			body,
		)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("%w: ROUTE-REFRESH invalid length %d", ErrInvalidMessage, len(body))
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
	family := capability.Family{AFI: capability.AFI(rr.AFI), SAFI: capability.SAFI(rr.SAFI)}
	if !s.negotiated.SupportsFamily(family) {
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
func (s *Session) shouldIgnoreFamily(family capability.Family) bool {
	for _, f := range s.settings.IgnoreFamilies {
		if f.AFI == family.AFI && f.SAFI == family.SAFI {
			return true
		}
	}
	return false
}
