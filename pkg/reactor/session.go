package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/trace"
)

// Session errors.
var (
	ErrNotConnected        = errors.New("not connected")
	ErrAlreadyConnected    = errors.New("already connected")
	ErrInvalidState        = errors.New("invalid FSM state")
	ErrNotificationRecv    = errors.New("notification received")
	ErrConnectionClosed    = errors.New("connection closed")
	ErrHoldTimerExpired    = errors.New("hold timer expired")
	ErrInvalidMessage      = errors.New("invalid message")
	ErrUnsupportedVersion  = errors.New("unsupported BGP version")
	ErrFamilyNotNegotiated = errors.New("address family not negotiated")
)

// Session manages a single BGP peer connection.
//
// It integrates the FSM, timers, and message I/O to drive the BGP
// state machine through the connection lifecycle.
type Session struct {
	mu sync.RWMutex

	settings   *PeerSettings
	fsm        *fsm.FSM
	timers     *fsm.Timers
	conn       net.Conn
	negotiated *capability.Negotiated

	// localOpen stores our OPEN for reference during negotiation.
	localOpen *message.Open

	// peerOpen stores the peer's OPEN for reference.
	peerOpen *message.Open

	// Read buffer (reused).
	readBuf []byte

	// Error channel for timer callbacks to signal errors.
	errChan chan error
}

// NewSession creates a new BGP session for a peer.
func NewSession(settings *PeerSettings) *Session {
	s := &Session{
		settings: settings,
		fsm:      fsm.New(),
		timers:   fsm.NewTimers(),
		readBuf:  make([]byte, message.MaxMsgLen),
		errChan:  make(chan error, 1),
	}

	// Configure FSM passive mode.
	s.fsm.SetPassive(settings.Passive)

	// Configure timers.
	s.timers.SetHoldTime(settings.HoldTime)

	// Wire up timer callbacks.
	s.timers.OnHoldTimerExpires(func() {
		s.mu.Lock()
		_ = s.fsm.Event(fsm.EventHoldTimerExpires)
		s.mu.Unlock()
		select {
		case s.errChan <- ErrHoldTimerExpired:
		default:
		}
	})

	s.timers.OnKeepaliveTimerExpires(func() {
		s.mu.Lock()
		conn := s.conn
		neg := s.negotiated
		s.mu.Unlock()

		if conn != nil {
			_ = s.sendKeepalive(conn, neg)
		}
	})

	return s
}

// State returns the current FSM state.
func (s *Session) State() fsm.State {
	return s.fsm.State()
}

// Conn returns the current connection (nil if not connected).
func (s *Session) Conn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}

// Negotiated returns the negotiated capabilities (nil until OPENCONFIRM).
func (s *Session) Negotiated() *capability.Negotiated {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.negotiated
}

// Start triggers the ManualStart event to begin the connection process.
func (s *Session) Start() error {
	return s.fsm.Event(fsm.EventManualStart)
}

// Stop triggers the ManualStop event.
func (s *Session) Stop() error {
	s.timers.StopAll()
	return s.fsm.Event(fsm.EventManualStop)
}

// Connect initiates an outgoing TCP connection.
func (s *Session) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	addr := net.JoinHostPort(s.settings.Address.String(), fmt.Sprintf("%d", s.settings.Port))

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		_ = s.fsm.Event(fsm.EventTCPConnectionFails)
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	return s.connectionEstablished(conn)
}

// Accept accepts an incoming TCP connection.
func (s *Session) Accept(conn net.Conn) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	return s.connectionEstablished(conn)
}

// connectionEstablished handles a new TCP connection (incoming or outgoing).
func (s *Session) connectionEstablished(conn net.Conn) error {
	s.mu.Lock()
	s.conn = conn
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
	neg := s.negotiated
	s.mu.Unlock()

	if conn != nil {
		// Send NOTIFICATION (Cease/Administrative Shutdown).
		_ = s.sendNotification(conn, neg,
			message.NotifyCease,
			message.NotifyCeaseAdminShutdown,
			nil,
		)
	}

	s.closeConn()
	_ = s.fsm.Event(fsm.EventManualStop)

	return nil
}

// closeConn closes the TCP connection.
func (s *Session) closeConn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// Run is the main session loop. It processes messages until context is
// cancelled or an error occurs.
func (s *Session) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-s.errChan:
			s.closeConn()
			return err
		default:
		}

		// Check if we have a connection.
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			// No connection, nothing to do.
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Set read deadline to allow context checking.
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		err := s.readAndProcessMessage(conn)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) ||
				isTimeout(err) {
				continue // Timeout, check context and retry.
			}
			return err
		}
	}
}

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
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	return s.readAndProcessMessage(conn)
}

// readAndProcessMessage reads a message from the connection and processes it.
func (s *Session) readAndProcessMessage(conn net.Conn) error {
	// Read header.
	_, err := io.ReadFull(conn, s.readBuf[:message.HeaderLen])
	if err != nil {
		if errors.Is(err, io.EOF) {
			s.handleConnectionClose()
			return ErrConnectionClosed
		}
		return err
	}

	hdr, err := message.ParseHeader(s.readBuf[:message.HeaderLen])
	if err != nil {
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		return fmt.Errorf("parse header: %w", err)
	}

	// RFC 8654: Validate message length against max (4096 or 65535 if extended).
	// Get negotiated state to check if extended message is enabled.
	s.mu.RLock()
	neg := s.negotiated
	s.mu.RUnlock()

	extendedMessage := neg != nil && neg.ExtendedMessage
	if err := hdr.ValidateLengthWithMax(extendedMessage); err != nil {
		// RFC 8654 Section 5: Send NOTIFICATION with Bad Message Length.
		_ = s.sendNotification(conn, neg,
			message.NotifyMessageHeader,
			message.NotifyHeaderBadLength,
			[]byte{byte(hdr.Length >> 8), byte(hdr.Length)},
		)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("message length %d exceeds max for %s: %w", hdr.Length, hdr.Type, err)
	}

	// Read body.
	bodyLen := int(hdr.Length) - message.HeaderLen
	if bodyLen > 0 {
		_, err = io.ReadFull(conn, s.readBuf[message.HeaderLen:hdr.Length])
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
	}

	return s.processMessage(&hdr, s.readBuf[message.HeaderLen:hdr.Length])
}

// processMessage handles a received BGP message.
func (s *Session) processMessage(hdr *message.Header, body []byte) error {
	switch hdr.Type { //nolint:exhaustive // Unknown types handled in default
	case message.TypeOPEN:
		return s.handleOpen(body)
	case message.TypeKEEPALIVE:
		return s.handleKeepalive()
	case message.TypeUPDATE:
		return s.handleUpdate(body)
	case message.TypeNOTIFICATION:
		return s.handleNotification(body)
	default:
		return s.handleUnknownType(hdr.Type)
	}
}

// handleUnknownType handles unknown message types (exabgp-compatible).
func (s *Session) handleUnknownType(msgType message.MessageType) error {
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	s.mu.RUnlock()

	// ExaBGP format: Message Header Error (1), subcode 0, text message.
	errMsg := fmt.Sprintf("can not decode update message of type \"%d\"", msgType)
	_ = s.sendNotification(conn, neg,
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
		neg := s.negotiated
		s.mu.RUnlock()

		_ = s.sendNotification(conn, neg,
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
		neg := s.negotiated
		s.mu.RUnlock()

		// Send NOTIFICATION with the error (already a *Notification).
		var notif *message.Notification
		if errors.As(err, &notif) {
			_ = s.sendNotification(conn, neg, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Negotiate capabilities.
	s.negotiate()

	// Validate required families are negotiated.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	s.mu.RUnlock()

	if len(requiredFamilies) > 0 && neg != nil {
		if missing := neg.CheckRequired(requiredFamilies); len(missing) > 0 {
			// Required families not negotiated - send NOTIFICATION and reject.
			// RFC 5492 Section 3: Use Unsupported Capability subcode.
			capData := buildUnsupportedCapabilityData(missing)
			_ = s.sendNotification(conn, neg,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required families not negotiated: %v", ErrInvalidState, missing)
		}
	}

	// Update FSM.
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE to confirm.
	if err := s.sendKeepalive(conn, neg); err != nil {
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
// RFC 7606: validates path attributes with revised error handling.
func (s *Session) handleUpdate(body []byte) error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

	// Validate address families in UPDATE.
	if err := s.validateUpdateFamilies(body); err != nil {
		return err
	}

	// RFC 7606: Validate path attributes with revised error handling.
	if err := s.validateUpdateRFC7606(body); err != nil {
		return err
	}

	return s.fsm.Event(fsm.EventUpdateMsg)
}

// validateUpdateRFC7606 performs RFC 7606 attribute validation.
// Returns nil for treat-as-withdraw (session stays up), error for session reset.
func (s *Session) validateUpdateRFC7606(body []byte) error {
	// Parse UPDATE structure
	if len(body) < 4 {
		return nil // Let other validation handle
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(body) {
		return nil
	}

	// RFC 7606 Section 5.3: Validate withdrawn routes NLRI syntax (IPv4)
	if withdrawnLen > 0 {
		withdrawn := body[2 : 2+withdrawnLen]
		if result := message.ValidateNLRISyntax(withdrawn, false); result != nil {
			trace.RFC7606TreatAsWithdraw(0, result.Description)
			return nil // treat-as-withdraw
		}
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return nil
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriLen := len(body) - (offset + attrLen)
	hasNLRI := nlriLen > 0

	// RFC 7606 Section 5.3: Validate NLRI syntax (IPv4)
	if nlriLen > 0 {
		nlri := body[offset+attrLen:]
		if result := message.ValidateNLRISyntax(nlri, false); result != nil {
			trace.RFC7606TreatAsWithdraw(0, result.Description)
			return nil // treat-as-withdraw
		}
	}

	// Validate path attributes per RFC 7606
	result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI)

	switch result.Action {
	case message.RFC7606ActionNone:
		// No error, continue normally
		return nil

	case message.RFC7606ActionTreatAsWithdraw:
		// RFC 7606: Log and continue (routes treated as withdrawn)
		trace.RFC7606TreatAsWithdraw(result.AttrCode, result.Description)
		return nil // Session stays up

	case message.RFC7606ActionAttributeDiscard:
		// RFC 7606: Log and continue (malformed attribute discarded)
		trace.RFC7606AttributeDiscard(result.AttrCode, result.Description)
		return nil // Session stays up

	case message.RFC7606ActionSessionReset:
		// RFC 7606: Session reset required (e.g., multiple MP_REACH)
		trace.RFC7606SessionReset(result.Description)
		return fmt.Errorf("RFC 7606 session reset: %s", result.Description)
	}

	return nil
}

// validateUpdateFamilies checks that AFI/SAFI in MP_REACH/MP_UNREACH were negotiated.
// RFC 4760 Section 6: "If a BGP speaker receives an UPDATE with MP_REACH_NLRI or
// MP_UNREACH_NLRI where the AFI/SAFI do not match those negotiated in OPEN,
// the speaker MAY treat this as an error.".
func (s *Session) validateUpdateFamilies(body []byte) error {
	// Need at least 4 bytes: withdrawn len (2) + attrs len (2)
	if len(body) < 4 {
		return nil // Let message parsing handle malformed
	}

	// Skip withdrawn routes
	withdrawnLen := binary.BigEndian.Uint16(body[0:2])
	offset := 2 + int(withdrawnLen)
	if offset+2 > len(body) {
		return nil
	}

	// Get path attributes
	attrLen := binary.BigEndian.Uint16(body[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(body) {
		return nil
	}
	pathAttrs := body[offset : offset+int(attrLen)]

	// Parse path attributes looking for MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}

		flags := pathAttrs[pos]
		code := attribute.AttributeCode(pathAttrs[pos+1])
		pos += 2

		// Determine length (1 or 2 bytes based on extended length flag)
		var attrDataLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			attrDataLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrDataLen = int(pathAttrs[pos])
			pos++
		}

		if pos+attrDataLen > len(pathAttrs) {
			break
		}

		attrData := pathAttrs[pos : pos+attrDataLen]
		pos += attrDataLen

		// Check MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
		if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
			if len(attrData) < 3 {
				continue // Malformed, let other validation catch it
			}

			afi := capability.AFI(binary.BigEndian.Uint16(attrData[0:2]))
			safi := capability.SAFI(attrData[2])
			family := capability.Family{AFI: afi, SAFI: safi}

			neg := s.Negotiated()
			if neg != nil && !neg.SupportsFamily(family) {
				// Family not negotiated - check if we should ignore
				shouldIgnore := s.settings.IgnoreFamilyMismatch || s.shouldIgnoreFamily(family)
				if shouldIgnore {
					// Lenient mode: log warning and skip
					trace.UpdateFamilyMismatch(uint16(afi), uint8(safi), true)
				} else {
					// Strict mode: return error
					trace.UpdateFamilyMismatch(uint16(afi), uint8(safi), false)
					return fmt.Errorf("%w: %s", ErrFamilyNotNegotiated, family)
				}
			}
		}
	}

	return nil
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

// handleConnectionClose handles TCP connection close.
func (s *Session) handleConnectionClose() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventTCPConnectionFails)
	s.closeConn()
}

// negotiate performs capability negotiation between local and peer OPEN.
func (s *Session) negotiate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.localOpen == nil || s.peerOpen == nil {
		return
	}

	// Parse capabilities from OPEN messages.
	localCaps := parseCapabilities(s.localOpen.OptionalParams)
	peerCaps := parseCapabilities(s.peerOpen.OptionalParams)

	// Negotiate.
	s.negotiated = capability.Negotiate(
		localCaps,
		peerCaps,
		s.settings.LocalAS,
		s.peerOpen.ASN4,
	)

	// RFC 8654: If extended message is negotiated, resize read buffer.
	// MUST be capable of receiving messages up to 65535 octets.
	if s.negotiated.ExtendedMessage && len(s.readBuf) < message.ExtMsgLen {
		s.readBuf = make([]byte, message.ExtMsgLen)
	}

	// Negotiate hold time (minimum of both, but at least 3s if non-zero).
	localHold := s.settings.HoldTime
	peerHold := time.Duration(s.peerOpen.HoldTime) * time.Second

	var negotiatedHold time.Duration
	if localHold == 0 || peerHold == 0 {
		negotiatedHold = 0
	} else {
		negotiatedHold = localHold
		if peerHold < negotiatedHold {
			negotiatedHold = peerHold
		}
		// RFC 4271: Minimum hold time is 3 seconds if non-zero.
		if negotiatedHold > 0 && negotiatedHold < 3*time.Second {
			negotiatedHold = 3 * time.Second
		}
	}

	s.negotiated.HoldTime = uint16(negotiatedHold / time.Second) //nolint:gosec // Hold time max 65535s
	s.timers.SetHoldTime(negotiatedHold)
}

// sendOpen sends an OPEN message.
func (s *Session) sendOpen(conn net.Conn) error {
	// Build capabilities in RFC-expected order:
	// 1. Multiprotocol (from settings.Capabilities)
	// 2. ASN4
	// 3. Other capabilities (FQDN, SoftwareVersion, etc.)
	var caps []capability.Capability
	var otherCaps []capability.Capability

	// Separate Multiprotocol capabilities from others.
	for _, c := range s.settings.Capabilities {
		if c.Code() == capability.CodeMultiprotocol {
			caps = append(caps, c)
		} else {
			otherCaps = append(otherCaps, c)
		}
	}

	// Add ASN4 unless disabled in config.
	if !s.settings.DisableASN4 {
		caps = append(caps, &capability.ASN4{ASN: s.settings.LocalAS})
	}

	// Add remaining capabilities.
	caps = append(caps, otherCaps...)

	// Build optional parameters (capabilities).
	optParams := buildOptionalParams(caps)

	// Determine AS to put in header (AS_TRANS if > 65535).
	myAS := uint16(s.settings.LocalAS) //nolint:gosec // Truncation intended for AS_TRANS
	if s.settings.LocalAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	open := &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(s.settings.HoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  s.settings.RouterID,
		ASN4:           s.settings.LocalAS,
		OptionalParams: optParams,
	}

	s.mu.Lock()
	s.localOpen = open
	s.mu.Unlock()

	return s.writeMessage(conn, nil, open)
}

// sendKeepalive sends a KEEPALIVE message.
func (s *Session) sendKeepalive(conn net.Conn, neg *capability.Negotiated) error {
	return s.writeMessage(conn, neg, message.NewKeepalive())
}

// sendNotification sends a NOTIFICATION message.
func (s *Session) sendNotification(conn net.Conn, neg *capability.Negotiated, code message.NotifyErrorCode, subcode uint8, data []byte) error {
	notif := &message.Notification{
		ErrorCode:    code,
		ErrorSubcode: subcode,
		Data:         data,
	}
	return s.writeMessage(conn, neg, notif)
}

// writeMessage packs and sends a BGP message.
func (s *Session) writeMessage(conn net.Conn, neg *capability.Negotiated, msg message.Message) error {
	if conn == nil {
		return ErrNotConnected
	}

	// Convert capability.Negotiated to message.Negotiated for packing.
	var msgNeg *message.Negotiated
	if neg != nil {
		msgNeg = &message.Negotiated{
			ASN4:            neg.ASN4,
			ExtendedMessage: neg.ExtendedMessage,
			LocalAS:         neg.LocalASN,
			PeerAS:          neg.PeerASN,
			HoldTime:        neg.HoldTime,
		}
	}

	data, err := msg.Pack(msgNeg)
	if err != nil {
		return fmt.Errorf("pack message: %w", err)
	}

	_, err = conn.Write(data)
	return err
}

// TriggerHoldTimerExpiry triggers the hold timer expiry event.
// Exposed for testing.
func (s *Session) TriggerHoldTimerExpiry() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventHoldTimerExpires)
	s.closeConn()
}

// parseCapabilities parses capabilities from OPEN optional parameters.
func parseCapabilities(optParams []byte) []capability.Capability {
	var caps []capability.Capability

	i := 0
	for i < len(optParams) {
		if i+2 > len(optParams) {
			break
		}
		paramType := optParams[i]
		paramLen := int(optParams[i+1])
		i += 2

		if i+paramLen > len(optParams) {
			break
		}

		if paramType == 2 { // Capabilities
			parsed, err := capability.Parse(optParams[i : i+paramLen])
			if err == nil {
				caps = append(caps, parsed...)
			}
		}

		i += paramLen
	}

	return caps
}

// buildOptionalParams builds optional parameters from capabilities.
// Each capability is wrapped in its own parameter (type 2) per RFC 5492.
func buildOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}

	var optParams []byte
	for _, c := range caps {
		capBytes := c.Pack()
		// Wrap each capability in its own parameter type 2.
		param := make([]byte, 2+len(capBytes))
		param[0] = 2 // Parameter type: Capabilities
		param[1] = byte(len(capBytes))
		copy(param[2:], capBytes)
		optParams = append(optParams, param...)
	}

	return optParams
}

// SendUpdate sends a BGP UPDATE message.
// Returns ErrInvalidState if the session is not established.
func (s *Session) SendUpdate(update *message.Update) error {
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	// Convert capability.Negotiated to message.Negotiated for packing.
	var msgNeg *message.Negotiated
	if neg != nil {
		msgNeg = &message.Negotiated{
			ASN4:            neg.ASN4,
			ExtendedMessage: neg.ExtendedMessage,
			LocalAS:         neg.LocalASN,
			PeerAS:          neg.PeerASN,
			HoldTime:        neg.HoldTime,
		}
	}

	data, err := update.Pack(msgNeg)
	if err != nil {
		return fmt.Errorf("pack UPDATE: %w", err)
	}

	_, err = conn.Write(data)
	return err
}

// isTimeout checks if an error is a timeout.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
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

// buildUnsupportedCapabilityData builds NOTIFICATION data for Unsupported Capability.
//
// RFC 5492 Section 3: The Data field contains one or more capability tuples.
// For Multiprotocol (code 1): AFI (2 bytes) + Reserved (1 byte) + SAFI (1 byte).
func buildUnsupportedCapabilityData(families []capability.Family) []byte {
	// Each Multiprotocol capability: code (1) + length (1) + AFI (2) + Reserved (1) + SAFI (1) = 6 bytes
	data := make([]byte, len(families)*6)
	offset := 0
	for _, f := range families {
		data[offset] = byte(capability.CodeMultiprotocol) // Capability code
		data[offset+1] = 4                                // Capability length
		binary.BigEndian.PutUint16(data[offset+2:], uint16(f.AFI))
		data[offset+4] = 0 // Reserved
		data[offset+5] = byte(f.SAFI)
		offset += 6
	}
	return data
}
