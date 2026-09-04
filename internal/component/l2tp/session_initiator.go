// Design: docs/research/l2tpv2-implementation-guide.md -- S10 session FSMs, initiator half
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 7.6 (ICRQ), 7.7 (ICRP), 7.8 (ICCN),
//      7.9 (OCRQ), 7.10 (OCRP)
// Related: session_fsm.go -- the answering half (handleICRQ / handleOCRQ / parsers)

package l2tp

import (
	"fmt"
	"log/slog"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// callParams carries the local-side attributes stamped into an initiator's
// ICRQ / OCRQ (and the Tx Connect Speed / Framing Type echoed back in the
// ICCN we send after ICRP). A LAC fills these from the relayed PPPoE session;
// an LNS fills them from the outgoing-call RPC. FramingType defaults to 1
// (synchronous) when left zero so the ICCN/OCRQ carry a valid Framing Type.
type callParams struct {
	callSerial     uint32
	bearerType     uint32
	framingType    uint32
	txConnectSpeed uint32
	rxConnectSpeed uint32
	minBPS         uint32
	maxBPS         uint32
	calledNumber   string
	callingNumber  string
	// pppoeChannelFD is the relayed PPPoE session's kernel PPP channel fd,
	// carried on a LAC incoming call so the reactor can bridge it to the
	// pppol2tp channel (A-4) when the session's kernel resources are created.
	// Zero for calls not originating from a PPPoE relay.
	pppoeChannelFD int
}

// framingOrDefault returns the framing type, defaulting to 1 (synchronous)
// so a caller that leaves it unset still emits a wire-valid Framing Type AVP.
func (p callParams) framingOrDefault() uint32 {
	if p.framingType == 0 {
		return 1
	}
	return p.framingType
}

// ---------------------------------------------------------------------------
// Call origination (initiator side). Caller MUST hold tunnelsMu and MUST have
// an established tunnel; both methods return the allocated local session ID
// plus the outbound request datagram (nil sends on failure).
// ---------------------------------------------------------------------------

// placeIncomingCall originates a LAC-side incoming call on an established
// tunnel (RFC 2661 Section 10.1): it allocates a local session ID, creates
// the session in wait-reply, and sends the ICRQ. The ICCN that follows the
// peer's ICRP echoes the Tx Connect Speed / Framing Type captured here.
func (t *L2TPTunnel) placeIncomingCall(now time.Time, p callParams, logger *slog.Logger) (uint16, []sendRequest) {
	sess, ok := t.newInitiatorSession(now, false, p, logger)
	if !ok {
		return 0, nil
	}
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeICRQBody(*bodyBuf, sess.localSID, p.callSerial, p.bearerType, p.calledNumber, p.callingNumber)
	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now, false)
	if err != nil {
		logger.Warn("l2tp: ICRQ enqueue failed", "local-sid", sess.localSID, "error", err.Error())
		t.removeSession(sess.localSID, l2tpevents.TerminateCauseNASError)
		return 0, nil
	}
	logger.Info("l2tp: ICRQ sent; session wait-reply (LAC incoming)",
		"local-sid", sess.localSID, "call-serial", p.callSerial)
	return sess.localSID, []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// placeOutgoingCall originates an LNS-side outgoing call on an established
// tunnel (RFC 2661 Section 10.4): it allocates a local session ID, creates
// the session in wait-reply with lnsMode true (ze is the LNS end), and sends
// the OCRQ. OCRP moves it to wait-connect; OCCN establishes it.
func (t *L2TPTunnel) placeOutgoingCall(now time.Time, p callParams, logger *slog.Logger) (uint16, []sendRequest) {
	sess, ok := t.newInitiatorSession(now, true, p, logger)
	if !ok {
		return 0, nil
	}
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeOCRQBody(*bodyBuf, sess.localSID, p.callSerial, p.minBPS, p.maxBPS,
		p.bearerType, p.framingOrDefault(), p.calledNumber)
	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now, false)
	if err != nil {
		logger.Warn("l2tp: OCRQ enqueue failed", "local-sid", sess.localSID, "error", err.Error())
		t.removeSession(sess.localSID, l2tpevents.TerminateCauseNASError)
		return 0, nil
	}
	logger.Info("l2tp: OCRQ sent; session wait-reply (LNS outgoing)",
		"local-sid", sess.localSID, "called", p.calledNumber)
	return sess.localSID, []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// newInitiatorSession is the shared allocation path for placeIncomingCall and
// placeOutgoingCall: it enforces the established-tunnel + max-sessions
// preconditions, allocates a non-zero local session ID, and inserts a
// wait-reply session carrying the local call attributes. lnsMode fixes the
// kernel role for the call's lifetime (preserved through ICCN/OCCN).
func (t *L2TPTunnel) newInitiatorSession(now time.Time, lnsMode bool, p callParams, logger *slog.Logger) (*L2TPSession, bool) {
	if t.state != L2TPTunnelEstablished {
		logger.Debug("l2tp: call origination on non-established tunnel; refused", "state", t.state.String())
		return nil, false
	}
	if t.maxSessions > 0 && uint16(t.sessionCount()) >= t.maxSessions {
		logger.Warn("l2tp: call origination refused; max sessions reached",
			"max", t.maxSessions, "current", t.sessionCount())
		return nil, false
	}
	localSID := t.allocateSessionID()
	if localSID == 0 {
		logger.Warn("l2tp: session ID space exhausted for call origination")
		return nil, false
	}
	sess := &L2TPSession{
		localSID:       localSID,
		state:          L2TPSessionIdle,
		createdAt:      now,
		fsmHistory:     newFSMHistoryRing(),
		lnsMode:        lnsMode,
		txConnectSpeed: p.txConnectSpeed,
		rxConnectSpeed: p.rxConnectSpeed,
		framingType:    p.framingOrDefault(),
		callingNumber:  p.callingNumber,
		pppoeChannelFD: p.pppoeChannelFD,
	}
	// Record the origination as an FSM transition so the initiator session's
	// history and metrics reflect it entered wait-reply on our request (AC-5),
	// rather than materializing already in wait-reply with an empty history.
	trigger := "outgoing call originated"
	if !lnsMode {
		trigger = "incoming call originated"
	}
	sess.transition(L2TPSessionWaitReply, trigger)
	t.addSession(sess)
	return sess, true
}

// ---------------------------------------------------------------------------
// Reply handlers (initiator side). These fill the former session_fsm.go stubs.
// ---------------------------------------------------------------------------

// handleICRP processes an Incoming-Call-Reply on a LAC-initiated session in
// wait-reply (RFC 2661 Section 10.1). It records the peer's Assigned Session
// ID, sends the ICCN, and moves the session to established with a kernel-setup
// request (lnsMode false: the LAC bridges frames, it does not run PPP).
func (t *L2TPTunnel) handleICRP(sess *L2TPSession, payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	if sess.state != L2TPSessionWaitReply {
		logger.Debug("l2tp: ICRP on non-wait-reply session; dropped",
			"local-sid", sess.localSID, "state", sess.state.String())
		return nil
	}
	info, err := parseICRP(payload)
	if err != nil {
		logger.Warn("l2tp: malformed ICRP; sending CDN RC=2",
			"local-sid", sess.localSID, "error", err.Error())
		return t.teardownSession(sess, cdnResultGeneralError, l2tpevents.TerminateCauseNASError, now, logger)
	}
	sess.remoteSID = info.assignedSessionID

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeICCNBody(*bodyBuf, sess.txConnectSpeed, sess.framingType)
	wire, enqErr := t.engine.Enqueue(sess.remoteSID, (*bodyBuf)[:n], now, false)
	if enqErr != nil {
		logger.Warn("l2tp: ICCN enqueue failed; tearing down session",
			"local-sid", sess.localSID, "error", enqErr.Error())
		return t.teardownSession(sess, cdnResultGeneralError, l2tpevents.TerminateCauseNASError, now, logger)
	}
	sess.transition(L2TPSessionEstablished, "ICRP received")
	sess.kernelSetupNeeded = true
	sess.lnsMode = false
	// AC-4: a blocking PlaceIncomingCallSync (if any) learns the call is up.
	sess.resolveCall(callOutcome{localSID: sess.localSID, remoteSID: sess.remoteSID})
	logger.Info("l2tp: session established (incoming LAC)",
		"local-sid", sess.localSID, "remote-sid", sess.remoteSID)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// handleOCRP processes an Outgoing-Call-Reply on an LNS-initiated session in
// wait-reply (RFC 2661 Section 10.4). It records the peer's Assigned Session
// ID and moves the session to wait-connect; the OCCN that follows establishes
// it. No datagram is produced here.
func (t *L2TPTunnel) handleOCRP(sess *L2TPSession, payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	if sess.state != L2TPSessionWaitReply {
		logger.Debug("l2tp: OCRP on non-wait-reply session; dropped",
			"local-sid", sess.localSID, "state", sess.state.String())
		return nil
	}
	info, err := parseOCRP(payload)
	if err != nil {
		logger.Warn("l2tp: malformed OCRP; sending CDN RC=2",
			"local-sid", sess.localSID, "error", err.Error())
		return t.teardownSession(sess, cdnResultGeneralError, l2tpevents.TerminateCauseNASError, now, logger)
	}
	sess.remoteSID = info.assignedSessionID
	sess.transition(L2TPSessionWaitConnect, "OCRP received")
	logger.Info("l2tp: OCRP received; session wait-connect (LNS outgoing)",
		"local-sid", sess.localSID, "remote-sid", sess.remoteSID)
	return nil
}

// ---------------------------------------------------------------------------
// Wire builders for initiator-side session messages (buffer-first; no make).
// ---------------------------------------------------------------------------

// writeICRQBody writes the AVP body of an ICRQ (LAC -> LNS incoming call).
// RFC 2661 Section 7.6 required AVPs: Message Type (10), Assigned Session ID,
// Call Serial Number. Optional Bearer Type and Called/Calling Number AVPs are
// appended when non-zero / non-empty. localSID is the session ID we assigned.
func writeICRQBody(buf []byte, localSID uint16, callSerial, bearerType uint32, calledNumber, callingNumber string) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, localSID)
	off += WriteAVPUint32(buf, off, true, AVPCallSerialNumber, callSerial)
	if bearerType != 0 {
		off += WriteAVPUint32(buf, off, false, AVPBearerType, bearerType)
	}
	if calledNumber != "" {
		off += WriteAVPString(buf, off, false, AVPCalledNumber, calledNumber)
	}
	if callingNumber != "" {
		off += WriteAVPString(buf, off, false, AVPCallingNumber, callingNumber)
	}
	return off
}

// writeICCNBody writes the AVP body of an ICCN (LAC -> LNS incoming call
// connected). RFC 2661 Section 7.8 required AVPs: Message Type (12), Tx
// Connect Speed, Framing Type.
func writeICCNBody(buf []byte, txConnectSpeed, framingType uint32) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgICCN))
	off += WriteAVPUint32(buf, off, true, AVPTxConnectSpeed, txConnectSpeed)
	off += WriteAVPUint32(buf, off, true, AVPFramingType, framingType)
	return off
}

// writeOCRQBody writes the AVP body of an OCRQ (LNS -> LAC outgoing call).
// RFC 2661 Section 7.9 required AVPs: Message Type (7), Assigned Session ID,
// Call Serial Number, Minimum BPS, Maximum BPS, Bearer Type, Framing Type,
// Called Number. localSID is the session ID we assigned.
func writeOCRQBody(buf []byte, localSID uint16, callSerial, minBPS, maxBPS, bearerType, framingType uint32, calledNumber string) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgOCRQ))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, localSID)
	off += WriteAVPUint32(buf, off, true, AVPCallSerialNumber, callSerial)
	off += WriteAVPUint32(buf, off, true, AVPMinimumBPS, minBPS)
	off += WriteAVPUint32(buf, off, true, AVPMaximumBPS, maxBPS)
	off += WriteAVPUint32(buf, off, true, AVPBearerType, bearerType)
	off += WriteAVPUint32(buf, off, true, AVPFramingType, framingType)
	off += WriteAVPString(buf, off, true, AVPCalledNumber, calledNumber)
	return off
}

// ---------------------------------------------------------------------------
// Parsers for reply messages ze now receives as an initiator.
// ---------------------------------------------------------------------------

// icrpInfo collects the fields parseICRP extracts from an ICRP body.
type icrpInfo struct {
	assignedSessionID uint16
}

// ocrpInfo collects the fields parseOCRP extracts from an OCRP body.
type ocrpInfo struct {
	assignedSessionID uint16
}

// parseICRP extracts the Assigned Session ID from an ICRP body (RFC 2661
// Section 7.7). Message Type MUST be first (S4.4.1); Assigned Session ID is
// required and non-zero because it becomes the header Session ID of the
// ICCN we send next.
func parseICRP(payload []byte) (icrpInfo, error) {
	sid, err := parseAssignedSessionIDReply(payload, MsgICRP, "ICRP")
	if err != nil {
		return icrpInfo{}, err
	}
	return icrpInfo{assignedSessionID: sid}, nil
}

// parseOCRP extracts the Assigned Session ID from an OCRP body (RFC 2661
// Section 7.10). Same shape as ICRP.
func parseOCRP(payload []byte) (ocrpInfo, error) {
	sid, err := parseAssignedSessionIDReply(payload, MsgOCRP, "OCRP")
	if err != nil {
		return ocrpInfo{}, err
	}
	return ocrpInfo{assignedSessionID: sid}, nil
}

// parseAssignedSessionIDReply is the shared parser for ICRP and OCRP, whose
// bodies carry only a Message Type AVP followed by the sender's Assigned
// Session ID (RFC 2661 S7.7 / S7.10). It validates the Message Type AVP is
// first and matches expectedMsg, applies the standard mandatory-AVP rules,
// and returns the non-zero Assigned Session ID.
func parseAssignedSessionIDReply(payload []byte, expectedMsg MessageType, msgName string) (uint16, error) {
	iter := NewAVPIterator(payload)
	first := true
	var assignedSID uint16
	seenSID := false
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return 0, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return 0, fmt.Errorf("l2tp: mandatory %s AVP type %d with reserved bits set", msgName, attrType)
			}
			continue
		}
		if skip, err := skipHiddenAVP(msgName, attrType, flags); err != nil {
			return 0, err
		} else if skip {
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return 0, fmt.Errorf("l2tp: mandatory %s vendor %d AVP not recognized", msgName, vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return 0, fmt.Errorf("l2tp: first %s AVP must be Message Type (RFC 2661 S4.4.1)", msgName)
			}
			mt, rerr := readAVPUint16(value)
			if rerr != nil {
				return 0, fmt.Errorf("l2tp: read %s message type: %w", msgName, rerr)
			}
			if MessageType(mt) != expectedMsg {
				return 0, fmt.Errorf("l2tp: expected %s (%d), got %d", msgName, expectedMsg, mt)
			}
			first = false
			continue
		}
		if attrType == AVPAssignedSessionID {
			v, rerr := readAVPUint16(value)
			if rerr != nil {
				return 0, fmt.Errorf("l2tp: read %s assigned session id: %w", msgName, rerr)
			}
			assignedSID = v
			seenSID = true
		}
	}
	if first {
		return 0, fmt.Errorf("l2tp: empty %s body", msgName)
	}
	if !seenSID || assignedSID == 0 {
		return 0, fmt.Errorf("l2tp: %s missing or zero Assigned Session ID AVP", msgName)
	}
	return assignedSID, nil
}
