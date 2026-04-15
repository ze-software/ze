// Design: docs/research/l2tpv2-implementation-guide.md -- S9 tunnel FSM + S4 AVP handling
// Related: tunnel.go -- L2TPTunnel value and state enum
// Detail: session_fsm.go -- session-scoped message handlers dispatched from handleMessage

package l2tp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"
)

// TunnelDefaults carries the local-side values stamped into outbound
// control messages. Phase 3 hardcodes most of them; phase 7 will route
// these through YANG.
type TunnelDefaults struct {
	HostName            string
	FramingCapabilities uint32 // RFC 2661 S4.4.3: bit 0 = async, bit 1 = sync
	BearerCapabilities  uint32
	RecvWindow          uint16
	// SharedSecret is the CHAP-MD5 tunnel authentication secret (RFC 2661
	// S4.2). Empty disables our end of authentication; when a peer sends a
	// Challenge AVP while SharedSecret is empty, we reject the tunnel with
	// StopCCN Result Code 4. Non-empty enables Challenge verification on
	// SCCCN (we compute Challenge Response for SCCRP with CHAP_ID=2 and
	// verify the peer's response for SCCCN with CHAP_ID=3).
	SharedSecret string
}

// sendRequest is one outbound datagram produced during Process. The
// reactor enqueues these, releases tunnelsMu, and then calls listener.
// Send for each -- avoiding holding the tunnel-map lock across a
// kernel-visible UDP write (which may block when the TX queue is full).
type sendRequest struct {
	to    netip.AddrPort
	bytes []byte
}

// Process ingests one already-parsed header + its AVP body for this
// tunnel. It hands the message to the reliable engine for sequencing,
// dispatches each in-order delivery through the FSM, and collects every
// outbound datagram (SCCRP, window-opened sends, ZLB ACK) into a slice
// the reactor will send AFTER releasing the tunnel-map lock.
//
// sccrq carries the pre-validated SCCRQ AVP contents when this packet
// is the initial SCCRQ for the tunnel; handleSCCRQ consumes it without
// re-parsing. For any other message (or retransmitted SCCRQ that the
// engine dedupes) sccrq is nil.
//
// The caller (reactor) holds tunnelsMu for the whole call; the returned
// bytes are heap-owned (engine.Enqueue return values and explicit
// clones of ZLB output) so they stay valid after the lock releases.
func (t *L2TPTunnel) Process(hdr MessageHeader, payload []byte, now time.Time, defaults TunnelDefaults, sccrq *sccrqInfo) []sendRequest {
	var out []sendRequest
	res := t.engine.OnReceive(hdr, payload, now)
	if len(res.Delivered) > 0 {
		// Update lastActivity only when the engine delivers at least
		// one new message. Duplicates and out-of-window packets do not
		// count as peer activity (a peer replaying old Ns values must
		// not prevent the HELLO timeout from firing).
		t.lastActivity = now
	}
	for _, d := range res.Delivered {
		out = append(out, t.handleMessage(d, now, defaults, sccrq)...)
		// Only the FIRST delivery can be the SCCRQ we validated at
		// reactor level; subsequent deliveries (gap-fill) are a later
		// phase's concern and should not consume the pre-parsed info.
		sccrq = nil
	}
	for _, wire := range res.NewSends {
		out = append(out, sendRequest{to: t.peerAddr, bytes: wire})
	}
	if t.engine.NeedsZLB() {
		zlbBuf := GetBuf()
		defer PutBuf(zlbBuf)
		n := t.engine.BuildZLB(*zlbBuf, 0)
		// Clone the ZLB bytes because the caller needs them after we
		// release the pool buffer. ~12 bytes; trivial allocation.
		out = append(out, sendRequest{to: t.peerAddr, bytes: bytes.Clone((*zlbBuf)[:n])})
	}
	return out
}

// handleMessage dispatches one delivered message to the matching FSM
// transition. Phase 4 implements SCCRQ and SCCCN; other message types
// are logged and dropped for phase 5 to wire. Returns the outbound
// datagrams produced by the handler.
func (t *L2TPTunnel) handleMessage(entry RecvEntry, now time.Time, defaults TunnelDefaults, sccrq *sccrqInfo) []sendRequest {
	msgType := MessageType(entry.MessageType)
	if msgType == MsgSCCRQ {
		return t.handleSCCRQ(now, defaults, sccrq)
	}
	if msgType == MsgSCCRP {
		t.logger.Debug("l2tp: SCCRP received (LAC-side handling lands in a later phase)")
		return nil
	}
	if msgType == MsgSCCCN {
		return t.handleSCCCN(now, defaults, entry.Payload)
	}
	if msgType == MsgStopCCN {
		return t.handleStopCCN(now, entry.Payload)
	}
	if msgType == MsgHello {
		// Peer HELLO: no action needed beyond ACK. The engine's
		// NeedsZLB path already schedules the ZLB ACK. The inbound
		// message resets lastActivity via the reactor's Process caller.
		t.logger.Debug("l2tp: Hello received")
		return nil
	}
	// Session-scoped messages: ICRQ, ICRP, ICCN, OCRQ, OCRP, OCCN, CDN, WEN, SLI.
	// Dispatched to session_fsm.go handlers via dispatchToSession.
	if msgType == MsgICRQ || msgType == MsgICRP || msgType == MsgICCN ||
		msgType == MsgOCRQ || msgType == MsgOCRP || msgType == MsgOCCN ||
		msgType == MsgCDN || msgType == MsgWEN || msgType == MsgSLI {
		return t.dispatchToSession(msgType, entry.SessionID, entry.Payload, now, t.logger)
	}
	t.logger.Debug("l2tp: unsupported message type ignored", "type", uint16(msgType))
	return nil
}

// handleSCCRQ is the idle -> wait-ctl-conn transition. The reactor has
// already validated the SCCRQ body (via parseSCCRQ) and hands us the
// pre-parsed info, so this function cannot fail on malformed AVPs.
// Engine dedup guarantees this runs at most once per tunnel.
//
// Challenge flow (RFC 2661 S4.2 / S5.1.2):
//   - peer Challenge present + shared secret configured: compute peer's
//     CHAP-MD5 Response (CHAP_ID = SCCRP Message Type) and include it in
//     SCCRP; generate a 16-byte random Challenge of our own, store it on
//     the tunnel for SCCCN verification, and include it in SCCRP too.
//   - peer Challenge present + no shared secret: we cannot authenticate,
//     reject with StopCCN Result Code 4 (Not Authorized).
//   - peer Challenge absent: SCCRP is emitted without auth AVPs; SCCCN is
//     accepted unconditionally because the peer declined to request auth.
//
// Tie breaker: the parsed value (if present) is stored on the tunnel for
// the reactor to consult if a second SCCRQ collides with this one.
func (t *L2TPTunnel) handleSCCRQ(now time.Time, defaults TunnelDefaults, sccrq *sccrqInfo) []sendRequest {
	if t.state != L2TPTunnelIdle {
		t.logger.Debug("l2tp: SCCRQ on non-idle tunnel ignored", "state", t.state.String())
		return nil
	}
	if sccrq == nil {
		// The engine delivered an SCCRQ for which the reactor did not
		// pre-parse. Shouldn't happen -- reactor validates every
		// TunnelID=0 SCCRQ before creating the tunnel -- but fall back
		// to parsing rather than panicking, so a future code path that
		// delivers SCCRQ via a different route stays correct.
		info, err := parseSCCRQ(nil)
		if err != nil {
			t.logger.Warn("l2tp: SCCRQ delivered without pre-parsed info; refusing")
			return nil
		}
		sccrq = &info
	}
	t.peerHostName = sccrq.HostName
	t.peerFraming = sccrq.FramingCapabilities
	t.peerBearer = sccrq.BearerCapabilities
	t.peerRecvWindow = sccrq.RecvWindow
	if sccrq.TieBreakerPresent {
		t.tieBreaker = sccrq.TieBreakerValue
	}

	var peerResponse []byte
	if sccrq.ChallengePresent {
		if defaults.SharedSecret == "" {
			t.logger.Warn("l2tp: SCCRQ Challenge AVP present but shared-secret is unset; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized)
		}
		resp := ChallengeResponse(ChapIDSCCRP, []byte(defaults.SharedSecret), sccrq.ChallengeValue)
		peerResponse = resp[:]
		ours := make([]byte, 16)
		if _, err := rand.Read(ours); err != nil {
			t.logger.Warn("l2tp: unable to read random Challenge; sending StopCCN RC=4", "error", err.Error())
			return t.teardownStopCCN(now, resultNotAuthorized)
		}
		t.ourChallenge = ours
	}

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeSCCRPBody(*bodyBuf, t.localTID, defaults, t.ourChallenge, peerResponse)

	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now)
	if err != nil {
		t.logger.Warn("l2tp: SCCRP enqueue failed; tunnel stays idle", "error", err.Error())
		return nil
	}
	t.state = L2TPTunnelWaitCtlConn
	t.logger.Info("l2tp: SCCRP sent; tunnel now wait-ctl-conn",
		"peer-host", strconv.Quote(sccrq.HostName),
		"peer-tid", t.remoteTID,
		"peer-framing", fmt.Sprintf("0x%08x", sccrq.FramingCapabilities),
		"challenge", t.ourChallenge != nil)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// handleSCCCN is the wait-ctl-conn -> established transition. The payload
// is the already-in-order SCCCN body handed up by the engine. Returns the
// outbound datagrams produced (StopCCN RC=4 on auth failure; empty on
// success because the engine's NeedsZLB path schedules the ACK).
//
// Engine dedup makes this safe for duplicate SCCCNs: the engine delivers
// the message once and retransmits are ACKed from the engine's retention.
// A second SCCCN with a different Ns on an established tunnel is
// delivered; the state check below drops it with a debug log (defense
// against a malicious peer, not a normal protocol event).
func (t *L2TPTunnel) handleSCCCN(now time.Time, defaults TunnelDefaults, payload []byte) []sendRequest {
	if t.state != L2TPTunnelWaitCtlConn {
		t.logger.Debug("l2tp: SCCCN on non-wait-ctl-conn tunnel ignored", "state", t.state.String())
		return nil
	}
	scccn, err := parseSCCCN(payload)
	if err != nil {
		t.logger.Warn("l2tp: malformed SCCCN; sending StopCCN RC=4", "error", err.Error())
		return t.teardownStopCCN(now, resultNotAuthorized)
	}
	if t.ourChallenge != nil {
		if !scccn.ChallengeResponsePresent {
			t.logger.Warn("l2tp: SCCCN missing Challenge Response; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized)
		}
		if !VerifyChallengeResponse(ChapIDSCCCN, []byte(defaults.SharedSecret), t.ourChallenge, scccn.ChallengeResponseValue) {
			t.logger.Warn("l2tp: SCCCN Challenge Response did not verify; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized)
		}
	}
	t.state = L2TPTunnelEstablished
	// Release the challenge now that verification succeeded. No consumer
	// reads it past established; keeping it would be a 16-byte leak per
	// tunnel for the tunnel's lifetime.
	t.ourChallenge = nil
	t.logger.Info("l2tp: tunnel now established",
		"peer-host", strconv.Quote(t.peerHostName),
		"peer-tid", t.remoteTID)
	return nil
}

// teardownStopCCN encodes a StopCCN with the given Result Code into a
// pooled buffer, pushes it through the engine, transitions the tunnel to
// closed, and returns the resulting outbound datagram. Called from FSM
// handlers that decide to tear the tunnel down (bad Challenge Response,
// malformed SCCCN, etc.). RFC 2661 S4.4.2.
func (t *L2TPTunnel) teardownStopCCN(now time.Time, resultCode uint16) []sendRequest {
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeStopCCNBody(*bodyBuf, t.localTID, resultCode)

	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now)
	if err != nil {
		t.logger.Warn("l2tp: StopCCN enqueue failed", "error", err.Error())
		t.state = L2TPTunnelClosed
		t.engine.Close(now)
		return nil
	}
	t.state = L2TPTunnelClosed
	t.engine.Close(now)
	t.logger.Info("l2tp: StopCCN sent; tunnel closed", "result-code", resultCode)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// sccrqInfo collects the fields parseSCCRQ pulls out of the AVP stream.
type sccrqInfo struct {
	MessageType         MessageType
	ProtocolVersion     uint16
	FramingCapabilities uint32
	BearerCapabilities  uint32
	HostName            string
	AssignedTunnelID    uint16
	RecvWindow          uint16
	ChallengePresent    bool
	ChallengeValue      []byte
	TieBreakerPresent   bool
	TieBreakerValue     []byte
}

// parseSCCRQ walks the AVP stream of an SCCRQ body and collects the
// fields the FSM needs. Message Type AVP MUST be first per RFC 2661
// S4.1; Host Name and Assigned Tunnel ID AVPs MUST be present per S6.1.
// Vendor-ID != 0 with M=1 aborts the parse (RFC: unrecognized mandatory
// AVP => tear down).
func parseSCCRQ(payload []byte) (sccrqInfo, error) {
	var info sccrqInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return sccrqInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return sccrqInfo{}, fmt.Errorf("l2tp: mandatory AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return sccrqInfo{}, fmt.Errorf("l2tp: mandatory vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return sccrqInfo{}, errors.New("l2tp: first AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return sccrqInfo{}, fmt.Errorf("l2tp: read message type: %w", rerr)
			}
			info.MessageType = MessageType(mt)
			if info.MessageType != MsgSCCRQ {
				return sccrqInfo{}, fmt.Errorf("l2tp: expected SCCRQ (1), got %d", info.MessageType)
			}
			first = false
			continue
		}
		// Vendor ID = 0, well-formed header. Capture the fields we care
		// about; ignore everything else (RFC: unrecognized non-mandatory
		// AVPs are silently skipped).
		if attrType == AVPProtocolVersion && len(value) >= 2 {
			info.ProtocolVersion = binary.BigEndian.Uint16(value[:2])
			continue
		}
		if attrType == AVPFramingCapabilities {
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.FramingCapabilities = v
			}
			continue
		}
		if attrType == AVPBearerCapabilities {
			v, rerr := ReadAVPUint32(value)
			if rerr == nil {
				info.BearerCapabilities = v
			}
			continue
		}
		if attrType == AVPHostName {
			info.HostName = string(value)
			continue
		}
		if attrType == AVPAssignedTunnelID {
			v, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return sccrqInfo{}, fmt.Errorf("l2tp: read assigned tunnel id: %w", rerr)
			}
			if v == 0 {
				return sccrqInfo{}, errors.New("l2tp: Assigned Tunnel ID AVP must be non-zero")
			}
			info.AssignedTunnelID = v
			continue
		}
		if attrType == AVPReceiveWindowSize {
			v, rerr := ReadAVPUint16(value)
			if rerr == nil {
				info.RecvWindow = v
			}
			continue
		}
		if attrType == AVPChallenge {
			// RFC 2661 S5.12: Challenge is "at least one octet". An empty
			// Challenge AVP would make the peer's Response trivially
			// forgeable (MD5(chapID||secret)) and would fire the
			// ChallengeResponse panic guard in auth.go. Reject at the
			// edge so no tunnel state is allocated for the offender.
			if len(value) == 0 {
				return sccrqInfo{}, errors.New("l2tp: Challenge AVP must carry at least one octet (RFC 2661 S5.12)")
			}
			info.ChallengePresent = true
			info.ChallengeValue = append([]byte(nil), value...)
			continue
		}
		if attrType == AVPTieBreaker {
			// RFC 2661 S4.4.2: Tie Breaker is a fixed 8-byte value. Wrong
			// lengths would distort byte-wise comparison (a 1-byte value
			// always compares lower than any 8-byte value) and let a
			// misbehaving peer win every collision by underrunning.
			if len(value) != 8 {
				return sccrqInfo{}, fmt.Errorf("l2tp: Tie Breaker AVP must be 8 bytes (RFC 2661 S4.4.2), got %d", len(value))
			}
			info.TieBreakerPresent = true
			info.TieBreakerValue = append([]byte(nil), value...)
			continue
		}
		// Anything else (Firmware Revision, Vendor Name, etc.) is
		// optional and ignored for phase-3 purposes.
	}
	if first {
		return sccrqInfo{}, errors.New("l2tp: empty SCCRQ body")
	}
	if info.HostName == "" {
		return sccrqInfo{}, errors.New("l2tp: SCCRQ missing Host Name AVP (RFC 2661 S6.1)")
	}
	if info.AssignedTunnelID == 0 {
		return sccrqInfo{}, errors.New("l2tp: SCCRQ missing Assigned Tunnel ID AVP")
	}
	return info, nil
}

// writeSCCRPBody writes the AVP body of an SCCRP into buf starting at
// offset 0 and returns the byte length written. When ourChallenge is
// non-nil, a Challenge AVP carrying our 16-byte random value is appended;
// when peerResponse is non-nil, a Challenge Response AVP carrying the
// peer's expected response is appended. Both are mandatory (M=1) AVPs
// per RFC 2661 S4.4.3. Caller supplies a pooled buffer; no `append` or
// `make`.
//
// Uses `off += Write*` because ze's L2TP wire helpers return bytes
// written, NOT new offset. Mixing `=` for one and `+=` for another
// corrupts the buffer silently.
func writeSCCRPBody(buf []byte, localTID uint16, d TunnelDefaults, ourChallenge, peerResponse []byte) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRP))
	// Protocol Version AVP carries 2 bytes: ver=1, rev=0 -> 0x01 0x00.
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, protocolVersionValue[:])
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, d.FramingCapabilities)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, d.BearerCapabilities)
	off += WriteAVPString(buf, off, true, AVPHostName, d.HostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, localTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, d.RecvWindow)
	if len(ourChallenge) > 0 {
		off += WriteAVPBytes(buf, off, true, 0, AVPChallenge, ourChallenge)
	}
	if len(peerResponse) > 0 {
		off += WriteAVPBytes(buf, off, true, 0, AVPChallengeResponse, peerResponse)
	}
	return off
}

// writeStopCCNBody writes the AVP body of a StopCCN into buf starting at
// offset 0 and returns the byte length written. Body layout per RFC 2661
// S4.4.2:
//   - Message Type AVP (value = StopCCN = 4)
//   - Assigned Tunnel ID AVP (our local TID, so the peer can correlate
//     the teardown with the tunnel in its own map)
//   - Result Code AVP (compound; Result = resultCode, no Error sub-field,
//     no advisory message -- phase 4 only emits the minimum structure).
func writeStopCCNBody(buf []byte, localTID, resultCode uint16) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgStopCCN))
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, localTID)
	off += WriteAVPResultCode(buf, off, true, ResultCodeValue{Result: resultCode})
	return off
}

// scccnInfo collects the fields parseSCCCN extracts from an SCCCN body.
type scccnInfo struct {
	MessageType              MessageType
	ChallengeResponsePresent bool
	ChallengeResponseValue   []byte
}

// parseSCCCN walks the AVP stream of an SCCCN body and collects the
// fields the FSM needs. Message Type AVP MUST be first (RFC 2661 S4.1);
// Challenge Response is optional on the wire but required when the
// caller established a Challenge during SCCRP emission -- that check is
// the caller's concern.
func parseSCCCN(payload []byte) (scccnInfo, error) {
	var info scccnInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return scccnInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return scccnInfo{}, fmt.Errorf("l2tp: mandatory SCCCN AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return scccnInfo{}, fmt.Errorf("l2tp: mandatory SCCCN vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return scccnInfo{}, errors.New("l2tp: first SCCCN AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return scccnInfo{}, fmt.Errorf("l2tp: read SCCCN message type: %w", rerr)
			}
			info.MessageType = MessageType(mt)
			if info.MessageType != MsgSCCCN {
				return scccnInfo{}, fmt.Errorf("l2tp: expected SCCCN (3), got %d", info.MessageType)
			}
			first = false
			continue
		}
		if attrType == AVPChallengeResponse {
			info.ChallengeResponsePresent = true
			info.ChallengeResponseValue = append([]byte(nil), value...)
			continue
		}
		// Any other AVP (including unexpected ones) is ignored; SCCCN
		// carries only the Challenge Response AVP in the phase-4 scope.
	}
	if first {
		return scccnInfo{}, errors.New("l2tp: empty SCCCN body")
	}
	return info, nil
}

// protocolVersionValue is the 2-byte Protocol Version AVP value (v1 rev0)
// per RFC 2661 S4.4.2. Shared across all outbound control messages.
var protocolVersionValue = [2]byte{0x01, 0x00}

// StopCCN Result Codes (RFC 2661 S4.4.2).
const (
	resultGeneralError  uint16 = 1 // "General request to clear control connection"
	resultNotAuthorized uint16 = 4 // "Requester is not authorized to establish a control connection"
)

// handleStopCCN processes a peer-sent StopCCN on any tunnel state.
// The tunnel transitions to closed and the engine begins its retention
// window. During retention, the engine continues to ACK retransmitted
// StopCCNs via the NeedsZLB path. AC-14.
//
// RFC 2661 S4.4.2: upon receipt of a StopCCN, the tunnel and all
// sessions within it must be cleared.
func (t *L2TPTunnel) handleStopCCN(now time.Time, payload []byte) []sendRequest {
	if t.state == L2TPTunnelClosed {
		t.logger.Debug("l2tp: StopCCN on already-closed tunnel ignored")
		return nil
	}
	info, err := parseStopCCN(payload)
	if err != nil {
		t.logger.Warn("l2tp: malformed StopCCN; ignoring", "error", err.Error())
		return nil
	}
	// AC-9: cascade CDN to all active sessions before closing tunnel.
	// RFC 2661 S4.4.2: upon receipt of a StopCCN, the tunnel and all
	// sessions within it must be cleared.
	cleared := t.clearSessions()
	if len(cleared) > 0 {
		t.logger.Info("l2tp: StopCCN clearing sessions", "count", len(cleared))
	}
	t.state = L2TPTunnelClosed
	t.engine.Close(now)
	t.logger.Info("l2tp: peer StopCCN received; tunnel closed",
		"result", info.Result,
		"error-code", info.Error,
		"message", info.Message)
	return nil
}

// stopCCNInfo collects the fields parseStopCCN extracts from a StopCCN body.
type stopCCNInfo struct {
	AssignedTunnelID uint16
	Result           uint16
	Error            uint16
	Message          string
}

// parseStopCCN walks the AVP stream of a StopCCN body and collects the
// required fields. Message Type AVP MUST be first (RFC 2661 S4.1);
// Assigned Tunnel ID and Result Code are required per S6.1.
func parseStopCCN(payload []byte) (stopCCNInfo, error) {
	var info stopCCNInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return stopCCNInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return stopCCNInfo{}, fmt.Errorf("l2tp: mandatory StopCCN AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return stopCCNInfo{}, fmt.Errorf("l2tp: mandatory StopCCN vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return stopCCNInfo{}, errors.New("l2tp: first StopCCN AVP must be Message Type (RFC 2661 S4.1)")
			}
			mt, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return stopCCNInfo{}, fmt.Errorf("l2tp: read StopCCN message type: %w", rerr)
			}
			if MessageType(mt) != MsgStopCCN {
				return stopCCNInfo{}, fmt.Errorf("l2tp: expected StopCCN (4), got %d", mt)
			}
			first = false
			continue
		}
		if attrType == AVPAssignedTunnelID {
			v, rerr := ReadAVPUint16(value)
			if rerr != nil {
				return stopCCNInfo{}, fmt.Errorf("l2tp: read StopCCN assigned tunnel id: %w", rerr)
			}
			info.AssignedTunnelID = v
			continue
		}
		if attrType == AVPResultCode {
			rc, rerr := ReadResultCode(value)
			if rerr != nil {
				return stopCCNInfo{}, fmt.Errorf("l2tp: read StopCCN result code: %w", rerr)
			}
			info.Result = rc.Result
			if rc.ErrorPresent {
				info.Error = rc.Error
			}
			info.Message = rc.Message
			continue
		}
	}
	if first {
		return stopCCNInfo{}, errors.New("l2tp: empty StopCCN body")
	}
	return info, nil
}

// handleHelloTimer is called by the reactor when the HELLO interval
// has elapsed without peer activity on an established tunnel. It
// enqueues a HELLO control message (body = Message Type AVP only,
// MsgHello = 6) through the reliable engine. The peer's ZLB ACK will
// flow through the engine's NeedsZLB path; no FSM response is needed.
// AC-12.
//
// RFC 2661 S15: a Hello message is sent after a dead interval during
// which a control message has not been received.
func (t *L2TPTunnel) handleHelloTimer(now time.Time) []sendRequest {
	if t.state != L2TPTunnelEstablished {
		return nil
	}
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := WriteAVPUint16(*bodyBuf, 0, true, AVPMessageType, uint16(MsgHello))
	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now)
	if err != nil {
		t.logger.Warn("l2tp: HELLO enqueue failed", "error", err.Error())
		return nil
	}
	t.logger.Debug("l2tp: HELLO sent on silence timeout")
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}
