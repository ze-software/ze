// Design: docs/research/l2tpv2-implementation-guide.md -- S10 session state machines
// Overview: tunnel_fsm.go -- handleMessage dispatches session-scoped messages here
// Related: session.go -- L2TPSession struct and tunnel helpers

package l2tp

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// CDN Result Codes (RFC 2661 S5.4.2). Distinct from StopCCN Result Codes.
const (
	cdnResultGeneralError uint16 = 2 // Call disconnected -- general error
	cdnResultNoResources  uint16 = 4 // Call disconnected -- no appropriate facilities
)

// dispatchToSession routes a session-scoped message to the appropriate
// handler. Called from handleMessage for message types ICRQ, ICRP, ICCN,
// OCRQ, OCRP, OCCN, CDN, WEN, and SLI.
//
// For session-creating messages (ICRQ, OCRQ) the header SessionID is 0;
// the handler allocates a new session. For all others, SessionID must
// match an existing session in the tunnel's map.
//
// RFC 2661 S24.12: unknown mandatory AVP in a session-scoped message
// tears down the session (CDN), not the tunnel (StopCCN).
func (t *L2TPTunnel) dispatchToSession(msgType MessageType, sessionID uint16, payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	if msgType == MsgICRQ {
		return t.handleICRQ(payload, now, logger)
	}
	if msgType == MsgOCRQ {
		return t.handleOCRQ(payload, now, logger)
	}
	if msgType == MsgCDN {
		return t.handleCDN(sessionID, payload, logger)
	}
	if msgType == MsgWEN {
		return t.handleWEN(sessionID, payload, logger)
	}
	if msgType == MsgSLI {
		return t.handleSLI(sessionID, payload, logger)
	}

	// ICRP, ICCN, OCRP, OCCN require an existing session.
	sess := t.lookupSession(sessionID)
	if sess == nil {
		// AC-16: unknown session ID in header -> drop with debug log.
		logger.Debug("l2tp: session-scoped message for unknown SID; dropped",
			"type", uint16(msgType), "session-id", sessionID)
		return nil
	}

	if msgType == MsgICRP {
		return t.handleICRP(sess, payload, logger)
	}
	if msgType == MsgICCN {
		return t.handleICCN(sess, payload, now, logger)
	}
	if msgType == MsgOCRP {
		return t.handleOCRP(sess, payload, logger)
	}
	if msgType == MsgOCCN {
		return t.handleOCCN(sess, payload, now, logger)
	}

	logger.Debug("l2tp: unhandled session message type", "type", uint16(msgType))
	return nil
}

// ---------------------------------------------------------------------------
// Incoming Call -- LNS side (RFC 2661 S10.2)
// ---------------------------------------------------------------------------

// handleICRQ processes an Incoming-Call-Request on an established tunnel.
// Creates a new session and sends ICRP. AC-1, AC-4, AC-5, AC-6.
//
// RFC 2661 Section 10.2: idle -> (recv ICRQ, acceptable) -> send ICRP -> wait-connect.
func (t *L2TPTunnel) handleICRQ(payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	// AC-4: ICRQ on non-established tunnel -> drop.
	if t.state != L2TPTunnelEstablished {
		logger.Debug("l2tp: ICRQ on non-established tunnel; dropped", "state", t.state.String())
		return nil
	}

	info, err := parseICRQ(payload)
	if err != nil {
		logger.Warn("l2tp: malformed ICRQ; sending CDN RC=2", "error", err.Error())
		return t.sendCDNNoSession(0, cdnResultGeneralError, now, logger)
	}

	// AC-6: Assigned Session ID = 0 is invalid.
	if info.assignedSessionID == 0 {
		logger.Warn("l2tp: ICRQ Assigned Session ID = 0; sending CDN RC=2")
		return t.sendCDNNoSession(0, cdnResultGeneralError, now, logger)
	}

	// Dedup: reject if we already have a session with this peer SID.
	// A malicious peer could send two ICRQs with different Ns but the same
	// Assigned Session ID; without this check we'd create two sessions that
	// the peer considers one.
	if t.lookupSessionByRemote(info.assignedSessionID) != nil {
		logger.Warn("l2tp: ICRQ duplicate remote SID; dropped", "remote-sid", info.assignedSessionID)
		return nil
	}

	// AC-5: max sessions check.
	if t.maxSessions > 0 && uint16(t.sessionCount()) >= t.maxSessions {
		logger.Warn("l2tp: ICRQ rejected; max sessions reached",
			"max", t.maxSessions, "current", t.sessionCount())
		return t.sendCDNNoSession(info.assignedSessionID, cdnResultNoResources, now, logger)
	}

	// AC-15: allocate local session ID with collision retry.
	localSID := t.allocateSessionID()
	if localSID == 0 {
		logger.Warn("l2tp: session ID space exhausted")
		return t.sendCDNNoSession(info.assignedSessionID, cdnResultNoResources, now, logger)
	}

	sess := &L2TPSession{
		localSID:  localSID,
		remoteSID: info.assignedSessionID,
		state:     L2TPSessionWaitConnect,
	}
	t.addSession(sess)

	// Build ICRP: Message Type + Assigned Session ID (our local SID in the body).
	// Header Session ID = peer's SID (the recipient's assigned ID per RFC 2661).
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeICRPBody(*bodyBuf, localSID)

	wire, enqErr := t.engine.Enqueue(info.assignedSessionID, (*bodyBuf)[:n], now)
	if enqErr != nil {
		logger.Warn("l2tp: ICRP enqueue failed", "error", enqErr.Error())
		t.removeSession(localSID)
		return nil
	}

	logger.Info("l2tp: ICRP sent; session wait-connect",
		"local-sid", localSID, "remote-sid", info.assignedSessionID,
		"call-serial", info.callSerialNumber)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// handleICRP processes an Incoming-Call-Reply. LAC side: wait-reply -> established
// is deferred (requires LAC-initiated tunnel). For now only logs.
func (t *L2TPTunnel) handleICRP(sess *L2TPSession, _ []byte, logger *slog.Logger) []sendRequest {
	// LAC-initiated incoming calls are deferred to a later spec.
	logger.Debug("l2tp: ICRP received (LAC-side deferred)",
		"local-sid", sess.localSID, "state", sess.state.String())
	return nil
}

// handleICCN processes an Incoming-Call-Connected on an LNS-side session
// in wait-connect state. AC-2, AC-3, AC-17, AC-18, AC-19.
//
// RFC 2661 Section 10.2: wait-connect -> (recv ICCN, acceptable) -> established.
func (t *L2TPTunnel) handleICCN(sess *L2TPSession, payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	// AC-3: ICCN on wrong state.
	if sess.state != L2TPSessionWaitConnect {
		logger.Debug("l2tp: ICCN on non-wait-connect session; dropped",
			"local-sid", sess.localSID, "state", sess.state.String())
		return nil
	}

	info, err := parseICCN(payload)
	if err != nil {
		// AC-3: malformed ICCN -> CDN.
		logger.Warn("l2tp: malformed ICCN; sending CDN RC=2",
			"local-sid", sess.localSID, "error", err.Error())
		return t.teardownSession(sess, cdnResultGeneralError, now, logger)
	}

	// AC-2: transition to established. Capture fields for phase 6.
	sess.state = L2TPSessionEstablished
	sess.txConnectSpeed = info.txConnectSpeed
	sess.rxConnectSpeed = info.rxConnectSpeed
	sess.framingType = info.framingType
	sess.sequencingRequired = info.sequencingRequired

	// AC-17: proxy LCP AVPs.
	sess.proxyInitialRecvLCPConfReq = info.proxyInitialRecvLCPConfReq
	sess.proxyLastSentLCPConfReq = info.proxyLastSentLCPConfReq
	sess.proxyLastRecvLCPConfReq = info.proxyLastRecvLCPConfReq

	// AC-18: proxy auth AVPs.
	sess.proxyAuthenType = info.proxyAuthenType
	sess.proxyAuthenName = info.proxyAuthenName
	sess.proxyAuthenChallenge = info.proxyAuthenChallenge
	sess.proxyAuthenID = info.proxyAuthenID
	sess.proxyAuthenResponse = info.proxyAuthenResponse

	logger.Info("l2tp: session established (incoming LNS)",
		"local-sid", sess.localSID, "remote-sid", sess.remoteSID,
		"tx-speed", info.txConnectSpeed, "framing", fmt.Sprintf("0x%08x", info.framingType))
	return nil
}

// ---------------------------------------------------------------------------
// Outgoing Call -- LAC side (RFC 2661 S10.4)
// ---------------------------------------------------------------------------

// handleOCRQ processes an Outgoing-Call-Request on an established tunnel.
// LAC receives OCRQ from LNS, sends OCRP. AC-12.
//
// RFC 2661 Section 10.4: idle -> (recv OCRQ, acceptable) -> send OCRP -> wait-cs-answer.
func (t *L2TPTunnel) handleOCRQ(payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	if t.state != L2TPTunnelEstablished {
		logger.Debug("l2tp: OCRQ on non-established tunnel; dropped", "state", t.state.String())
		return nil
	}

	info, err := parseOCRQ(payload)
	if err != nil {
		logger.Warn("l2tp: malformed OCRQ; sending CDN RC=2", "error", err.Error())
		return t.sendCDNNoSession(0, cdnResultGeneralError, now, logger)
	}

	if info.assignedSessionID == 0 {
		logger.Warn("l2tp: OCRQ Assigned Session ID = 0; sending CDN RC=2")
		return t.sendCDNNoSession(0, cdnResultGeneralError, now, logger)
	}

	if t.lookupSessionByRemote(info.assignedSessionID) != nil {
		logger.Warn("l2tp: OCRQ duplicate remote SID; dropped", "remote-sid", info.assignedSessionID)
		return nil
	}

	if t.maxSessions > 0 && uint16(t.sessionCount()) >= t.maxSessions {
		logger.Warn("l2tp: OCRQ rejected; max sessions reached",
			"max", t.maxSessions, "current", t.sessionCount())
		return t.sendCDNNoSession(info.assignedSessionID, cdnResultNoResources, now, logger)
	}

	localSID := t.allocateSessionID()
	if localSID == 0 {
		logger.Warn("l2tp: session ID space exhausted for OCRQ")
		return t.sendCDNNoSession(info.assignedSessionID, cdnResultNoResources, now, logger)
	}

	sess := &L2TPSession{
		localSID:  localSID,
		remoteSID: info.assignedSessionID,
		state:     L2TPSessionWaitCSAnswer,
	}
	t.addSession(sess)

	// Build OCRP: Message Type + Assigned Session ID (our local SID in the body).
	// Header Session ID = peer's SID (the recipient's assigned ID per RFC 2661).
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeOCRPBody(*bodyBuf, localSID)

	wire, enqErr := t.engine.Enqueue(info.assignedSessionID, (*bodyBuf)[:n], now)
	if enqErr != nil {
		logger.Warn("l2tp: OCRP enqueue failed", "error", enqErr.Error())
		t.removeSession(localSID)
		return nil
	}

	logger.Info("l2tp: OCRP sent; session wait-cs-answer",
		"local-sid", localSID, "remote-sid", info.assignedSessionID)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// handleOCRP processes an Outgoing-Call-Reply. LNS side: wait-reply -> wait-connect.
// LNS-initiated outgoing calls are deferred (requires LAC-initiated tunnel).
func (t *L2TPTunnel) handleOCRP(sess *L2TPSession, _ []byte, logger *slog.Logger) []sendRequest {
	logger.Debug("l2tp: OCRP received (LNS-side outgoing deferred)",
		"local-sid", sess.localSID, "state", sess.state.String())
	return nil
}

// handleOCCN processes an Outgoing-Call-Connected. AC-13.
//
// RFC 2661 Section 10.4: wait-cs-answer -> (bearer answers, send OCCN) -> established.
// But from the LAC's perspective receiving OCCN doesn't happen -- the LAC *sends* OCCN.
// From the LNS's perspective: wait-connect -> (recv OCCN) -> established.
// Since LNS-initiated outgoing calls are deferred, this handles the general
// OCCN reception on any session in wait-connect state.
func (t *L2TPTunnel) handleOCCN(sess *L2TPSession, payload []byte, now time.Time, logger *slog.Logger) []sendRequest {
	if sess.state != L2TPSessionWaitConnect {
		logger.Debug("l2tp: OCCN on non-wait-connect session; dropped",
			"local-sid", sess.localSID, "state", sess.state.String())
		return nil
	}

	info, err := parseOCCN(payload)
	if err != nil {
		logger.Warn("l2tp: malformed OCCN; sending CDN RC=2",
			"local-sid", sess.localSID, "error", err.Error())
		return t.teardownSession(sess, cdnResultGeneralError, now, logger)
	}

	sess.state = L2TPSessionEstablished
	sess.txConnectSpeed = info.txConnectSpeed
	sess.rxConnectSpeed = info.rxConnectSpeed
	sess.framingType = info.framingType
	sess.sequencingRequired = info.sequencingRequired

	logger.Info("l2tp: session established (outgoing)",
		"local-sid", sess.localSID, "remote-sid", sess.remoteSID,
		"tx-speed", info.txConnectSpeed)
	return nil
}

// ---------------------------------------------------------------------------
// CDN -- Call Disconnect Notify (RFC 2661 S5.4)
// ---------------------------------------------------------------------------

// handleCDN processes a Call-Disconnect-Notify. AC-7, AC-8.
//
// CDN is valid in any non-idle session state. The header SessionID is our
// local SID. The CDN's Assigned Session ID AVP carries the sender's own SID.
func (t *L2TPTunnel) handleCDN(sessionID uint16, payload []byte, logger *slog.Logger) []sendRequest {
	// CDN with SessionID=0: the peer is tearing down a session before we
	// assigned our local SID. Parse the CDN to log the result code, but
	// there's no session to clean up.
	if sessionID == 0 {
		info, _ := parseCDN(payload)
		logger.Info("l2tp: CDN for SID=0 (session not yet assigned)",
			"result", info.result, "error-code", info.errorCode)
		return nil
	}

	sess := t.lookupSession(sessionID)
	if sess == nil {
		logger.Debug("l2tp: CDN for unknown session; dropped", "session-id", sessionID)
		return nil
	}

	info, err := parseCDN(payload)
	if err != nil {
		logger.Warn("l2tp: malformed CDN; destroying session anyway",
			"local-sid", sessionID, "error", err.Error())
	} else {
		logger.Info("l2tp: CDN received; session destroyed",
			"local-sid", sessionID, "remote-sid", sess.remoteSID,
			"result", info.result, "error-code", info.errorCode)
	}

	t.removeSession(sessionID)
	return nil
}

// ---------------------------------------------------------------------------
// WEN / SLI (RFC 2661 S19)
// ---------------------------------------------------------------------------

// handleWEN processes a WAN-Error-Notify. AC-10.
// RFC 2661 S19.1: LAC -> LNS, carries Call Errors AVP.
func (t *L2TPTunnel) handleWEN(sessionID uint16, payload []byte, logger *slog.Logger) []sendRequest {
	sess := t.lookupSession(sessionID)
	if sess == nil {
		logger.Debug("l2tp: WEN for unknown session; dropped", "session-id", sessionID)
		return nil
	}
	if sess.state != L2TPSessionEstablished {
		logger.Debug("l2tp: WEN on non-established session; dropped",
			"local-sid", sessionID, "state", sess.state.String())
		return nil
	}

	info, err := parseWEN(payload)
	if err != nil {
		logger.Warn("l2tp: malformed WEN; ignored", "local-sid", sessionID, "error", err.Error())
		return nil
	}

	sess.callErrors = info.callErrors
	logger.Debug("l2tp: WEN received",
		"local-sid", sessionID,
		"crc-errors", info.callErrors.CRCErrors,
		"framing-errors", info.callErrors.FramingErrors)
	return nil
}

// handleSLI processes a Set-Link-Info. AC-11.
// RFC 2661 S19.2: LNS -> LAC, carries ACCM AVP.
func (t *L2TPTunnel) handleSLI(sessionID uint16, payload []byte, logger *slog.Logger) []sendRequest {
	sess := t.lookupSession(sessionID)
	if sess == nil {
		logger.Debug("l2tp: SLI for unknown session; dropped", "session-id", sessionID)
		return nil
	}
	if sess.state != L2TPSessionEstablished {
		logger.Debug("l2tp: SLI on non-established session; dropped",
			"local-sid", sessionID, "state", sess.state.String())
		return nil
	}

	info, err := parseSLI(payload)
	if err != nil {
		logger.Warn("l2tp: malformed SLI; ignored", "local-sid", sessionID, "error", err.Error())
		return nil
	}

	sess.accm = info.accm
	logger.Debug("l2tp: SLI received",
		"local-sid", sessionID,
		"send-accm", fmt.Sprintf("0x%08x", info.accm.SendACCM),
		"recv-accm", fmt.Sprintf("0x%08x", info.accm.RecvACCM))
	return nil
}

// ---------------------------------------------------------------------------
// Session teardown helpers
// ---------------------------------------------------------------------------

// teardownSession sends a CDN for the given session and removes it from the
// tunnel's session map. Used when we detect an error during session setup.
func (t *L2TPTunnel) teardownSession(sess *L2TPSession, resultCode uint16, now time.Time, logger *slog.Logger) []sendRequest {
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeCDNBody(*bodyBuf, sess.localSID, resultCode)

	wire, err := t.engine.Enqueue(sess.remoteSID, (*bodyBuf)[:n], now)
	t.removeSession(sess.localSID)
	if err != nil {
		logger.Warn("l2tp: CDN enqueue failed", "local-sid", sess.localSID, "error", err.Error())
		return nil
	}
	logger.Info("l2tp: CDN sent; session destroyed",
		"local-sid", sess.localSID, "remote-sid", sess.remoteSID,
		"result-code", resultCode)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// sendCDNNoSession sends a CDN when we don't have a session yet (e.g.,
// ICRQ validation failed before session was created). The remoteSID is
// the peer's Assigned Session ID from the rejected message.
func (t *L2TPTunnel) sendCDNNoSession(remoteSID, resultCode uint16, now time.Time, logger *slog.Logger) []sendRequest {
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	// Our local SID is 0 since we never allocated one.
	n := writeCDNBody(*bodyBuf, 0, resultCode)

	wire, err := t.engine.Enqueue(remoteSID, (*bodyBuf)[:n], now)
	if err != nil {
		logger.Warn("l2tp: CDN (no-session) enqueue failed", "error", err.Error())
		return nil
	}
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

type icrqInfo struct {
	assignedSessionID uint16
	callSerialNumber  uint32
	bearerType        uint32
	calledNumber      string
	callingNumber     string
	subAddress        string
	physicalChanID    uint32
}

// parseICRQ extracts fields from an ICRQ body. RFC 2661 S7.6.
// Required: Message Type (10), Assigned Session ID, Call Serial Number.
func parseICRQ(payload []byte) (icrqInfo, error) {
	var info icrqInfo
	iter := NewAVPIterator(payload)
	first := true
	hasCallSerial := false
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return icrqInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				// AC-14: unknown mandatory AVP in session message.
				return icrqInfo{}, fmt.Errorf("l2tp: mandatory ICRQ AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				// AC-14: unknown mandatory vendor AVP.
				return icrqInfo{}, fmt.Errorf("l2tp: mandatory ICRQ vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return icrqInfo{}, errors.New("l2tp: first ICRQ AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return icrqInfo{}, fmt.Errorf("l2tp: read ICRQ message type: %w", rerr)
			}
			if MessageType(mt) != MsgICRQ {
				return icrqInfo{}, fmt.Errorf("l2tp: expected ICRQ (10), got %d", mt)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown are silently skipped per RFC
		case AVPAssignedSessionID:
			v, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return icrqInfo{}, fmt.Errorf("l2tp: read ICRQ assigned session id: %w", rerr)
			}
			info.assignedSessionID = v
		case AVPCallSerialNumber:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return icrqInfo{}, fmt.Errorf("l2tp: read ICRQ call serial: %w", rerr)
			}
			info.callSerialNumber = v
			hasCallSerial = true
		case AVPBearerType:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.bearerType = v
			}
		case AVPCalledNumber:
			info.calledNumber = string(value)
		case AVPCallingNumber:
			info.callingNumber = string(value)
		case AVPSubAddress:
			info.subAddress = string(value)
		case AVPPhysicalChannelID:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.physicalChanID = v
			}
		}
	}
	if first {
		return icrqInfo{}, errors.New("l2tp: empty ICRQ body")
	}
	if info.assignedSessionID == 0 {
		return icrqInfo{}, errors.New("l2tp: ICRQ missing Assigned Session ID AVP")
	}
	if !hasCallSerial {
		return icrqInfo{}, errors.New("l2tp: ICRQ missing Call Serial Number AVP (RFC 2661 S7.6)")
	}
	return info, nil
}

type iccnInfo struct {
	txConnectSpeed             uint32
	rxConnectSpeed             uint32
	framingType                uint32
	sequencingRequired         bool
	proxyInitialRecvLCPConfReq []byte
	proxyLastSentLCPConfReq    []byte
	proxyLastRecvLCPConfReq    []byte
	proxyAuthenType            uint16
	proxyAuthenName            string
	proxyAuthenChallenge       []byte
	proxyAuthenID              uint8
	proxyAuthenResponse        []byte
}

// parseICCN extracts fields from an ICCN body. RFC 2661 S7.8.
// Required: Message Type (12), Tx Connect Speed, Framing Type.
func parseICCN(payload []byte) (iccnInfo, error) {
	var info iccnInfo
	iter := NewAVPIterator(payload)
	first := true
	hasTxSpeed := false
	hasFraming := false
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return iccnInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return iccnInfo{}, fmt.Errorf("l2tp: mandatory ICCN AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return iccnInfo{}, fmt.Errorf("l2tp: mandatory ICCN vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return iccnInfo{}, errors.New("l2tp: first ICCN AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return iccnInfo{}, fmt.Errorf("l2tp: read ICCN message type: %w", rerr)
			}
			if MessageType(mt) != MsgICCN {
				return iccnInfo{}, fmt.Errorf("l2tp: expected ICCN (12), got %d", mt)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown skipped per RFC
		case AVPTxConnectSpeed:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return iccnInfo{}, fmt.Errorf("l2tp: read ICCN tx connect speed: %w", rerr)
			}
			info.txConnectSpeed = v
			hasTxSpeed = true
		case AVPRxConnectSpeed:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.rxConnectSpeed = v
			}
		case AVPFramingType:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return iccnInfo{}, fmt.Errorf("l2tp: read ICCN framing type: %w", rerr)
			}
			info.framingType = v
			hasFraming = true
		case AVPSequencingRequired:
			info.sequencingRequired = true
		// AC-17: Proxy LCP AVPs.
		case AVPInitialReceivedLCPConfReq:
			info.proxyInitialRecvLCPConfReq = append([]byte(nil), value...)
		case AVPLastSentLCPConfReq:
			info.proxyLastSentLCPConfReq = append([]byte(nil), value...)
		case AVPLastReceivedLCPConfReq:
			info.proxyLastRecvLCPConfReq = append([]byte(nil), value...)
		// AC-18: Proxy Auth AVPs.
		case AVPProxyAuthenType:
			v, rerr := ReadAVPUint16(value)
			if rerr == nil {
				info.proxyAuthenType = v
			}
		case AVPProxyAuthenName:
			info.proxyAuthenName = string(value)
		case AVPProxyAuthenChallenge:
			info.proxyAuthenChallenge = append([]byte(nil), value...)
		case AVPProxyAuthenID:
			v, rerr := ReadProxyAuthenID(value)
			if rerr == nil {
				info.proxyAuthenID = v.ChapID
			}
		case AVPProxyAuthenResponse:
			info.proxyAuthenResponse = append([]byte(nil), value...)
		}
	}
	if first {
		return iccnInfo{}, errors.New("l2tp: empty ICCN body")
	}
	if !hasTxSpeed {
		return iccnInfo{}, errors.New("l2tp: ICCN missing Tx Connect Speed AVP (RFC 2661 S7.8)")
	}
	if !hasFraming {
		return iccnInfo{}, errors.New("l2tp: ICCN missing Framing Type AVP (RFC 2661 S7.8)")
	}
	return info, nil
}

type ocrqInfo struct {
	assignedSessionID uint16
	callSerialNumber  uint32
	minimumBPS        uint32
	maximumBPS        uint32
	bearerType        uint32
	framingType       uint32
	calledNumber      string
	subAddress        string
}

// parseOCRQ extracts fields from an OCRQ body. RFC 2661 S7.9.
// Required: Message Type (7), Assigned Session ID, Call Serial Number,
// Minimum BPS, Maximum BPS, Bearer Type, Framing Type, Called Number.
func parseOCRQ(payload []byte) (ocrqInfo, error) {
	var info ocrqInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return ocrqInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return ocrqInfo{}, fmt.Errorf("l2tp: mandatory OCRQ AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return ocrqInfo{}, fmt.Errorf("l2tp: mandatory OCRQ vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return ocrqInfo{}, errors.New("l2tp: first OCRQ AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return ocrqInfo{}, fmt.Errorf("l2tp: read OCRQ message type: %w", rerr)
			}
			if MessageType(mt) != MsgOCRQ {
				return ocrqInfo{}, fmt.Errorf("l2tp: expected OCRQ (7), got %d", mt)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown skipped per RFC
		case AVPAssignedSessionID:
			v, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return ocrqInfo{}, fmt.Errorf("l2tp: read OCRQ assigned session id: %w", rerr)
			}
			info.assignedSessionID = v
		case AVPCallSerialNumber:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return ocrqInfo{}, fmt.Errorf("l2tp: read OCRQ call serial: %w", rerr)
			}
			info.callSerialNumber = v
		case AVPMinimumBPS:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.minimumBPS = v
			}
		case AVPMaximumBPS:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.maximumBPS = v
			}
		case AVPBearerType:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.bearerType = v
			}
		case AVPFramingType:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.framingType = v
			}
		case AVPCalledNumber:
			info.calledNumber = string(value)
		case AVPSubAddress:
			info.subAddress = string(value)
		}
	}
	if first {
		return ocrqInfo{}, errors.New("l2tp: empty OCRQ body")
	}
	if info.assignedSessionID == 0 {
		return ocrqInfo{}, errors.New("l2tp: OCRQ missing Assigned Session ID AVP")
	}
	return info, nil
}

type occnInfo struct {
	txConnectSpeed     uint32
	rxConnectSpeed     uint32
	framingType        uint32
	sequencingRequired bool
}

// parseOCCN extracts fields from an OCCN body. RFC 2661 S7.11.
// Required: Message Type (9), Tx Connect Speed, Framing Type.
func parseOCCN(payload []byte) (occnInfo, error) {
	var info occnInfo
	iter := NewAVPIterator(payload)
	first := true
	hasTxSpeed := false
	hasFraming := false
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return occnInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return occnInfo{}, fmt.Errorf("l2tp: mandatory OCCN AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return occnInfo{}, fmt.Errorf("l2tp: mandatory OCCN vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return occnInfo{}, errors.New("l2tp: first OCCN AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return occnInfo{}, fmt.Errorf("l2tp: read OCCN message type: %w", rerr)
			}
			if MessageType(mt) != MsgOCCN {
				return occnInfo{}, fmt.Errorf("l2tp: expected OCCN (9), got %d", mt)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown skipped per RFC
		case AVPTxConnectSpeed:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return occnInfo{}, fmt.Errorf("l2tp: read OCCN tx connect speed: %w", rerr)
			}
			info.txConnectSpeed = v
			hasTxSpeed = true
		case AVPRxConnectSpeed:
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.rxConnectSpeed = v
			}
		case AVPFramingType:
			v, rerr := ReadAVPUint32(value)
			if rerr != nil {
				return occnInfo{}, fmt.Errorf("l2tp: read OCCN framing type: %w", rerr)
			}
			info.framingType = v
			hasFraming = true
		case AVPSequencingRequired:
			info.sequencingRequired = true
		}
	}
	if first {
		return occnInfo{}, errors.New("l2tp: empty OCCN body")
	}
	if !hasTxSpeed {
		return occnInfo{}, errors.New("l2tp: OCCN missing Tx Connect Speed AVP (RFC 2661 S7.11)")
	}
	if !hasFraming {
		return occnInfo{}, errors.New("l2tp: OCCN missing Framing Type AVP (RFC 2661 S7.11)")
	}
	return info, nil
}

type cdnInfo struct {
	result            uint16
	errorCode         uint16
	message           string
	assignedSessionID uint16
	q931Cause         *Q931CauseValue
}

// parseCDN extracts fields from a CDN body. RFC 2661 S7.12.
// Required: Message Type (14), Result Code, Assigned Session ID.
func parseCDN(payload []byte) (cdnInfo, error) {
	var info cdnInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return cdnInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return cdnInfo{}, fmt.Errorf("l2tp: mandatory CDN AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return cdnInfo{}, fmt.Errorf("l2tp: mandatory CDN vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return cdnInfo{}, errors.New("l2tp: first CDN AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return cdnInfo{}, fmt.Errorf("l2tp: read CDN message type: %w", rerr)
			}
			if MessageType(mt) != MsgCDN {
				return cdnInfo{}, fmt.Errorf("l2tp: expected CDN (14), got %d", mt)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown skipped per RFC
		case AVPResultCode:
			rc, rerr := ReadResultCode(value)
			if rerr != nil {
				return cdnInfo{}, fmt.Errorf("l2tp: read CDN result code: %w", rerr)
			}
			info.result = rc.Result
			if rc.ErrorPresent {
				info.errorCode = rc.Error
			}
			info.message = rc.Message
		case AVPAssignedSessionID:
			v, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return cdnInfo{}, fmt.Errorf("l2tp: read CDN assigned session id: %w", rerr)
			}
			info.assignedSessionID = v
		case AVPQ931CauseCode:
			v, rerr := ReadQ931Cause(value)
			if rerr == nil {
				info.q931Cause = &v
			}
		}
	}
	if first {
		return cdnInfo{}, errors.New("l2tp: empty CDN body")
	}
	return info, nil
}

// parseSingleAVPMessage is a shared helper for messages that carry exactly
// one required compound AVP after the Message Type (WEN and SLI). It
// validates the Message Type AVP, scans for the target AVP, and returns
// the raw value bytes. The caller decodes the compound value.
func parseSingleAVPMessage(payload []byte, expectedMsg MessageType, targetAVP AVPType, msgName, avpName string) ([]byte, error) {
	iter := NewAVPIterator(payload)
	first := true
	var found []byte
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return nil, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return nil, fmt.Errorf("l2tp: mandatory %s AVP type %d with reserved bits set", msgName, attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return nil, fmt.Errorf("l2tp: mandatory %s vendor %d AVP not recognized", msgName, vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return nil, fmt.Errorf("l2tp: first %s AVP must be Message Type (RFC 2661 S4.1)", msgName)
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return nil, fmt.Errorf("l2tp: read %s message type: %w", msgName, rerr)
			}
			if MessageType(mt) != expectedMsg {
				return nil, fmt.Errorf("l2tp: expected %s (%d), got %d", msgName, expectedMsg, mt)
			}
			first = false
			continue
		}
		if attrType == targetAVP {
			found = value
		}
	}
	if first {
		return nil, fmt.Errorf("l2tp: empty %s body", msgName)
	}
	if found == nil {
		return nil, fmt.Errorf("l2tp: %s missing %s AVP", msgName, avpName)
	}
	return found, nil
}

type wenInfo struct {
	callErrors CallErrorsValue
}

// parseWEN extracts the Call Errors AVP from a WEN body. RFC 2661 S7.13.
func parseWEN(payload []byte) (wenInfo, error) {
	raw, err := parseSingleAVPMessage(payload, MsgWEN, AVPCallErrors, "WEN", "Call Errors")
	if err != nil {
		return wenInfo{}, err
	}
	v, rerr := ReadCallErrors(raw)
	if rerr != nil {
		return wenInfo{}, fmt.Errorf("l2tp: read WEN call errors: %w", rerr)
	}
	return wenInfo{callErrors: v}, nil
}

type sliInfo struct {
	accm ACCMValue
}

// parseSLI extracts the ACCM AVP from an SLI body. RFC 2661 S7.14.
func parseSLI(payload []byte) (sliInfo, error) {
	raw, err := parseSingleAVPMessage(payload, MsgSLI, AVPACCM, "SLI", "ACCM")
	if err != nil {
		return sliInfo{}, err
	}
	v, rerr := ReadACCM(raw)
	if rerr != nil {
		return sliInfo{}, fmt.Errorf("l2tp: read SLI ACCM: %w", rerr)
	}
	return sliInfo{accm: v}, nil
}

// ---------------------------------------------------------------------------
// Wire builders (buffer-first, no append, no make)
// ---------------------------------------------------------------------------

// writeICRPBody writes the AVP body of an ICRP. RFC 2661 S7.7.
// Required: Message Type (11), Assigned Session ID.
func writeICRPBody(buf []byte, localSID uint16) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgICRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, localSID)
	return off
}

// writeOCRPBody writes the AVP body of an OCRP. RFC 2661 S7.10.
// Required: Message Type (8), Assigned Session ID.
func writeOCRPBody(buf []byte, localSID uint16) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgOCRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, localSID)
	return off
}

// writeCDNBody writes the AVP body of a CDN. RFC 2661 S7.12.
// Required: Message Type (14), Result Code, Assigned Session ID.
// The Assigned Session ID is the SENDER's own SID (our local SID).
func writeCDNBody(buf []byte, localSID, resultCode uint16) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgCDN))
	off += WriteAVPResultCode(buf, off, true, ResultCodeValue{Result: resultCode})
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, localSID)
	return off
}
