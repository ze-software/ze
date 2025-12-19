package reactor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// Session errors.
var (
	ErrNotConnected       = errors.New("not connected")
	ErrAlreadyConnected   = errors.New("already connected")
	ErrInvalidState       = errors.New("invalid FSM state")
	ErrNotificationRecv   = errors.New("notification received")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrHoldTimerExpired   = errors.New("hold timer expired")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrUnsupportedVersion = errors.New("unsupported BGP version")
)

// Session manages a single BGP peer connection.
//
// It integrates the FSM, timers, and message I/O to drive the BGP
// state machine through the connection lifecycle.
type Session struct {
	mu sync.RWMutex

	neighbor   *Neighbor
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

// NewSession creates a new BGP session for a neighbor.
func NewSession(neighbor *Neighbor) *Session {
	s := &Session{
		neighbor: neighbor,
		fsm:      fsm.New(),
		timers:   fsm.NewTimers(),
		readBuf:  make([]byte, message.MaxMsgLen),
		errChan:  make(chan error, 1),
	}

	// Configure FSM passive mode.
	s.fsm.SetPassive(neighbor.Passive)

	// Configure timers.
	s.timers.SetHoldTime(neighbor.HoldTime)

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

	addr := fmt.Sprintf("%s:%d", s.neighbor.Address, s.neighbor.Port)

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
		return fmt.Errorf("%w: unknown type %d", ErrInvalidMessage, hdr.Type)
	}
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
		return ErrUnsupportedVersion
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Negotiate capabilities.
	s.negotiate()

	// Update FSM.
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE to confirm.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	s.mu.RUnlock()

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
func (s *Session) handleUpdate(_ []byte) error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

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
		s.neighbor.LocalAS,
		s.peerOpen.ASN4,
	)

	// Negotiate hold time (minimum of both, but at least 3s if non-zero).
	localHold := s.neighbor.HoldTime
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
	// Build capabilities.
	var caps []capability.Capability

	// Always add ASN4 if AS > 65535 or if configured.
	caps = append(caps, &capability.ASN4{ASN: s.neighbor.LocalAS})

	// Add configured capabilities.
	caps = append(caps, s.neighbor.Capabilities...)

	// Build optional parameters (capabilities).
	optParams := buildOptionalParams(caps)

	// Determine AS to put in header (AS_TRANS if > 65535).
	myAS := uint16(s.neighbor.LocalAS) //nolint:gosec // Truncation intended for AS_TRANS
	if s.neighbor.LocalAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	open := &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(s.neighbor.HoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  s.neighbor.RouterID,
		ASN4:           s.neighbor.LocalAS,
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
func buildOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}

	// Pack all capabilities.
	var capBytes []byte
	for _, c := range caps {
		capBytes = append(capBytes, c.Pack()...)
	}

	// Wrap in optional parameter type 2 (capabilities).
	optParams := make([]byte, 2+len(capBytes))
	optParams[0] = 2 // Parameter type: Capabilities
	optParams[1] = byte(len(capBytes))
	copy(optParams[2:], capBytes)

	return optParams
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
