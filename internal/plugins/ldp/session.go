// RFC: rfc/short/rfc5036.md -- Section 2.5 session establishment, Section 3.5.3 Initialization
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP session FSM
// Related: wire.go -- message encoding/decoding
// Related: discovery.go -- adjacency triggers session initiation
// Related: lib.go -- label bindings exchanged during session
//
// RFC 5036 Section 2.5: LDP session establishment uses TCP on port 646.
// The LSR with the higher transport address initiates the TCP connection.
// After TCP connect, Initialization messages are exchanged, then Keepalive.
package ldp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// RFC 5036 Section 2.5.3: Session states.
type SessionState uint8

const (
	StateNonExistent SessionState = iota
	StateInitialized
	StateOpenReceived
	StateOpenSent
	StateOperational
)

func (s SessionState) String() string {
	switch s {
	case StateNonExistent:
		return "non-existent"
	case StateInitialized:
		return "initialized"
	case StateOpenReceived:
		return "open-received"
	case StateOpenSent:
		return "open-sent"
	case StateOperational:
		return "operational"
	default:
		return "unknown"
	}
}

// RFC 5036 Section 2.5.3: Default keepalive.
const (
	DefaultKeepaliveTime = 60 * time.Second
	DefaultMaxPDULength  = 4096
)

var (
	errSessionClosed   = errors.New("ldp: session closed")
	errKeepaliveExpiry = errors.New("ldp: keepalive timer expired")
)

// Session represents a single LDP TCP session with a peer.
type Session struct {
	mu sync.Mutex

	state         SessionState
	conn          net.Conn
	peerAddr      netip.Addr
	localLSRID    [4]byte
	localLabelSpc uint16
	peerLSRID     [4]byte
	peerLabelSpc  uint16

	keepaliveTime time.Duration
	holdTime      time.Duration
	maxPDU        uint16

	nextMsgID atomic.Uint32

	// peerAddrs are the interface addresses the peer advertises via Address
	// messages (RFC 5036 Section 3.5.5), used to resolve a label binding to the IP
	// next hop on the shared link.
	peerAddrs []netip.Addr

	log      *slog.Logger
	lib      *LIB
	stopOnce sync.Once
	stopCh   chan struct{}
}

// addPeerAddresses records interface addresses learned from an Address message.
func (s *Session) addPeerAddresses(addrs []netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range addrs {
		if !a.IsValid() {
			continue
		}
		if !slices.Contains(s.peerAddrs, a) {
			s.peerAddrs = append(s.peerAddrs, a)
		}
	}
}

// removePeerAddresses drops addresses withdrawn via an Address Withdraw message.
func (s *Session) removePeerAddresses(addrs []netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range addrs {
		if i := slices.Index(s.peerAddrs, a); i >= 0 {
			s.peerAddrs = slices.Delete(s.peerAddrs, i, i+1)
		}
	}
}

// peerAddresses returns a copy of the peer's advertised interface addresses.
func (s *Session) peerAddresses() []netip.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]netip.Addr, len(s.peerAddrs))
	copy(out, s.peerAddrs)
	return out
}

// NewSession creates a session for the given adjacency. peerLSRID/peerLabelSpace
// come from the discovered neighbor's Hello: the Initialization message ze sends
// first must address the peer's LDP Identifier (RFC 5036 Section 3.5.3 "Receiver
// LDP Identifier"), or a compliant peer (e.g. FRR) rejects the session.
func NewSession(conn net.Conn, localLSRID [4]byte, localLabelSpace uint16, peerLSRID [4]byte, peerLabelSpace uint16, peerAddr netip.Addr, lib *LIB, log *slog.Logger) *Session {
	return &Session{
		state:         StateInitialized,
		conn:          conn,
		peerAddr:      peerAddr,
		localLSRID:    localLSRID,
		localLabelSpc: localLabelSpace,
		peerLSRID:     peerLSRID,
		peerLabelSpc:  peerLabelSpace,
		keepaliveTime: DefaultKeepaliveTime,
		// Initial hold time governs the wait for the peer's first Initialization
		// message; handleInit replaces it with the negotiated value. Without this
		// the first ReadLoop deadline is now+0 and times out before the peer's
		// Init can arrive, so the session never establishes (RFC 5036 Section 2.5.3).
		holdTime: 3 * DefaultKeepaliveTime,
		maxPDU:   DefaultMaxPDULength,
		lib:      lib,
		log:      log,
		stopCh:   make(chan struct{}),
	}
}

// currentHoldTime returns the session hold time under lock (handleInit updates it
// after keepalive negotiation while ReadLoop reads it each iteration).
func (s *Session) currentHoldTime() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.holdTime
}

// currentKeepalive returns the negotiated keepalive interval under lock. handleInit
// lowers it during the Initialization exchange, so the keepalive sender must read
// it each cycle rather than caching the pre-negotiation default.
func (s *Session) currentKeepalive() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keepaliveTime
}

// State returns the current session state.
func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// PeerAddr returns the peer's transport address.
func (s *Session) PeerAddr() netip.Addr {
	return s.peerAddr
}

// PeerLSRID returns the peer's LSR ID.
func (s *Session) PeerLSRID() [4]byte {
	return s.peerLSRID
}

// Stop terminates the session.
func (s *Session) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		_ = s.conn.Close()
		s.mu.Lock()
		s.state = StateNonExistent
		s.mu.Unlock()
	})
}

func (s *Session) stopped() bool {
	select {
	case <-s.stopCh:
		return true
	default: // non-blocking check
		return false
	}
}

// SendInit sends an Initialization message to the peer.
func (s *Session) SendInit() error {
	var buf [256]byte
	msgID := s.nextMsgID.Add(1)

	bodyLen := EncodeInit(buf[ldpHeaderLen:], initMessage{
		MessageID:          msgID,
		ProtocolVersion:    ldpVersion,
		KeepaliveTime:      uint16(s.keepaliveTime.Seconds()),
		MaxPDULength:       s.maxPDU,
		ReceiverLSRID:      s.peerLSRID,
		ReceiverLabelSpace: s.peerLabelSpc,
	})

	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      s.localLSRID,
		LabelSpace: s.localLabelSpc,
	})

	_, err := s.conn.Write(buf[:ldpHeaderLen+bodyLen])
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.state = StateOpenSent
	s.mu.Unlock()
	return nil
}

// SendKeepalive sends a KeepAlive message to the peer.
func (s *Session) SendKeepalive() error {
	var buf [64]byte
	msgID := s.nextMsgID.Add(1)

	bodyLen := encodeKeepalive(buf[ldpHeaderLen:], keepaliveMessage{
		MessageID: msgID,
	})

	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      s.localLSRID,
		LabelSpace: s.localLabelSpc,
	})

	_, err := s.conn.Write(buf[:ldpHeaderLen+bodyLen])
	return err
}

// SendLabelMapping sends a Label Mapping message for a FEC.
func (s *Session) SendLabelMapping(prefix netip.Prefix, label uint32) error {
	var buf [256]byte
	msgID := s.nextMsgID.Add(1)

	bodyLen := encodeLabelMapping(buf[ldpHeaderLen:], labelMappingMessage{
		MessageID: msgID,
		FEC: FECElement{
			Type:   FECPrefix,
			Prefix: prefix,
		},
		Label: label,
	})

	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      s.localLSRID,
		LabelSpace: s.localLabelSpc,
	})

	_, err := s.conn.Write(buf[:ldpHeaderLen+bodyLen])
	return err
}

// SendLabelWithdraw sends a Label Withdraw message for a FEC.
func (s *Session) SendLabelWithdraw(prefix netip.Prefix, label uint32) error {
	var buf [256]byte
	msgID := s.nextMsgID.Add(1)

	bodyLen := EncodeLabelWithdraw(buf[ldpHeaderLen:], labelWithdrawMessage{
		MessageID: msgID,
		FEC: FECElement{
			Type:   FECPrefix,
			Prefix: prefix,
		},
		Label:    label,
		HasLabel: true,
	})

	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      s.localLSRID,
		LabelSpace: s.localLabelSpc,
	})

	_, err := s.conn.Write(buf[:ldpHeaderLen+bodyLen])
	return err
}

// ReadLoop reads messages from the TCP connection and processes them.
// Blocks until the connection is closed or an error occurs. onOperational fires
// once, when the Initialization exchange completes and the session reaches the
// operational state, so the caller can advertise its local label mappings
// (RFC 5036 Section 2.5.3). It may be nil.
func (s *Session) ReadLoop(onLabel func(labelMappingMessage, [4]byte), onWithdraw func(labelWithdrawMessage, [4]byte), onOperational func()) error {
	var hdrBuf [ldpHeaderLen]byte
	for {
		if s.stopped() {
			return errSessionClosed
		}

		if err := s.conn.SetReadDeadline(time.Now().Add(s.currentHoldTime())); err != nil {
			return err
		}

		if _, err := io.ReadFull(s.conn, hdrBuf[:]); err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return errKeepaliveExpiry
			}
			return err
		}

		pdu, err := decodePDUHeader(hdrBuf[:])
		if err != nil {
			return err
		}

		bodyLen := int(pdu.PDULength) - 6
		if bodyLen <= 0 {
			continue
		}

		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(s.conn, body); err != nil {
			return err
		}

		if err := s.processMessages(body, pdu.LSRID, onLabel, onWithdraw, onOperational); err != nil {
			return err
		}
	}
}

func (s *Session) processMessages(body []byte, peerLSRID [4]byte, onLabel func(labelMappingMessage, [4]byte), onWithdraw func(labelWithdrawMessage, [4]byte), onOperational func()) error {
	off := 0
	for off < len(body) {
		if len(body[off:]) < ldpMsgHdrLen {
			break
		}
		msgHdr, err := decodeMessageHeader(body[off:])
		if err != nil {
			return err
		}
		msgEnd := off + ldpTLVHdrLen + int(msgHdr.Length)
		if msgEnd > len(body) {
			break
		}
		msgBody := body[off+ldpMsgHdrLen : msgEnd]

		switch msgHdr.Type {
		case MsgTypeKeepAlive:
			s.log.Debug("ldp: keepalive received")
		case MsgTypeInitialize:
			initMsg, err := DecodeInit(msgHdr.MessageID, msgBody)
			if err != nil {
				return err
			}
			// RFC 5036 Section 3.5.3: the Protocol Version in the Common Session
			// Parameters TLV is 1. A session proposing any other version is rejected
			// (Bad LDP Version) rather than negotiated: returning the error ends the
			// read loop, so no further message from this peer is processed.
			if initMsg.ProtocolVersion != ldpVersion {
				return fmt.Errorf("%w: initialization protocol version %d", errBadVersion, initMsg.ProtocolVersion)
			}
			if s.handleInit(initMsg, peerLSRID) && onOperational != nil {
				// Fire after handleInit has released s.mu so the callback can
				// safely send on the session without re-entering the lock.
				onOperational()
			}
		case MsgTypeLabelMapping:
			lm, err := decodeLabelMapping(msgHdr.MessageID, msgBody)
			if err != nil {
				return err
			}
			if onLabel != nil {
				onLabel(lm, peerLSRID)
			}
		case MsgTypeLabelWithdraw:
			lw, err := s.decodeLabelWithdraw(msgHdr.MessageID, msgBody)
			if err != nil {
				return err
			}
			if onWithdraw != nil {
				onWithdraw(lw, peerLSRID)
			}
		case MsgTypeAddress:
			am, err := decodeAddressList(msgHdr.MessageID, msgBody)
			if err != nil {
				return err
			}
			s.addPeerAddresses(am.Addresses)
			s.log.Debug("ldp: peer addresses learned", "count", len(am.Addresses))
		case MsgTypeAddressWithdraw:
			am, err := decodeAddressList(msgHdr.MessageID, msgBody)
			if err != nil {
				return err
			}
			s.removePeerAddresses(am.Addresses)
			s.log.Debug("ldp: peer addresses withdrawn", "count", len(am.Addresses))
		case MsgTypeNotification, MsgTypeLabelRequest, MsgTypeLabelRelease, MsgTypeLabelAbortReq:
			s.log.Debug("ldp: unhandled message type", "type", msgHdr.Type)
		default:
			s.log.Warn("ldp: unknown message type", "type", msgHdr.Type)
		}

		off = msgEnd
	}
	return nil
}

func (s *Session) decodeLabelWithdraw(msgID uint32, msgBody []byte) (labelWithdrawMessage, error) {
	lw := labelWithdrawMessage{MessageID: msgID}
	for bOff := 0; bOff < len(msgBody); {
		tlv, n, err := DecodeTLV(msgBody[bOff:])
		if err != nil {
			return lw, err
		}
		switch tlv.Type {
		case TLVTypeFEC:
			fec, err := decodeFECElement(tlv.Value)
			if err != nil {
				return lw, err
			}
			lw.FEC = fec
		case TLVTypeGenericLabel:
			if len(tlv.Value) >= 4 {
				lw.Label = binary.BigEndian.Uint32(tlv.Value[:4])
				lw.HasLabel = true
			}
		default:
			s.log.Debug("ldp: unknown TLV in label-withdraw", "type", tlv.Type)
		}
		bOff += n
	}
	return lw, nil
}

// handleInit applies a received Initialization message and advances the FSM.
// It returns true when this message transitions the session into the operational
// state, so the caller can advertise its local label mappings.
func (s *Session) handleInit(msg initMessage, peerLSRID [4]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.peerLSRID = peerLSRID
	negotiatedKA := msg.KeepaliveTime
	localKA := uint16(s.keepaliveTime.Seconds())
	if negotiatedKA > localKA {
		negotiatedKA = localKA
	}
	s.keepaliveTime = time.Duration(negotiatedKA) * time.Second
	// RFC 5036: hold time is typically 3x keepalive.
	s.holdTime = time.Duration(negotiatedKA) * time.Second * 3

	if msg.MaxPDULength > 0 && msg.MaxPDULength < s.maxPDU {
		s.maxPDU = msg.MaxPDULength
	}

	switch s.state {
	case StateOpenSent:
		s.state = StateOperational
		return true
	case StateInitialized:
		s.state = StateOpenReceived
	case StateNonExistent, StateOpenReceived, StateOperational:
		s.log.Warn("ldp: init received in unexpected state", "state", s.state.String())
	}
	return false
}
