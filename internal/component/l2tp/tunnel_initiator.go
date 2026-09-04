// Design: docs/research/l2tpv2-implementation-guide.md -- S9 tunnel FSM, initiator half
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.1 (Message Type AVP first), 4.4.3 (Challenge/Response AVPs), 6.1 (SCCRQ), 6.2 (SCCRP), 6.3 (SCCCN)
// Related: tunnel_fsm.go -- the answering half (handleSCCRQ / handleSCCCN)
// Related: reactor.go -- the reactor dial event that drives initiate

package l2tp

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// initiate is the idle -> wait-ctl-reply transition on the LAC/initiator
// side (RFC 2661 Section 6.1). It stamps our local tunnel ID into the
// Assigned Tunnel ID AVP so the peer addresses subsequent messages to us,
// generates a random Challenge when a shared secret is configured (forcing
// mutual authentication, mirroring the answering side), includes the caller-
// supplied 8-byte Tie Breaker so simultaneous-open collisions resolve
// (RFC 2661 Section 4.4.3), enqueues the SCCRQ through the reliable engine
// (Ns=0, PeerTunnelID=0 because the peer has not assigned one yet), and
// transitions to wait-ctl-reply.
//
// Caller (reactor dial handler) MUST hold tunnelsMu and MUST have created
// the tunnel in the idle state with its peerAddr set to the dial target.
func (t *L2TPTunnel) initiate(now time.Time, defaults TunnelDefaults, tieBreaker []byte) []sendRequest {
	if t.state != L2TPTunnelIdle {
		t.logger.Debug("l2tp: initiate on non-idle tunnel ignored", "state", t.state.String())
		return nil
	}
	// Whenever a SharedSecret is configured we authenticate the peer by
	// always emitting our own Challenge; a peer cannot bypass auth by
	// omitting its Challenge Response. Symmetric with handleSCCRQ.
	if defaults.SharedSecret != "" {
		ours := make([]byte, 16)
		if _, err := rand.Read(ours); err != nil {
			t.logger.Warn("l2tp: unable to read random Challenge for SCCRQ; dial aborted", "error", err.Error())
			return nil
		}
		t.ourChallenge = ours
	}
	if len(tieBreaker) == 8 {
		t.tieBreaker = tieBreaker
	}

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeSCCRQBody(*bodyBuf, t.localTID, defaults, t.ourChallenge, t.tieBreaker)

	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now, false)
	if err != nil {
		t.logger.Warn("l2tp: SCCRQ enqueue failed; tunnel stays idle", "error", err.Error())
		return nil
	}
	t.transition(L2TPTunnelWaitCtlReply, "dial")
	// t.logger already binds local-tid and peer (newTunnel).
	t.logger.Info("l2tp: SCCRQ sent; tunnel now wait-ctl-reply",
		"challenge", t.ourChallenge != nil,
		"tie-breaker", t.tieBreaker != nil)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// handleSCCRP is the wait-ctl-reply -> established transition on the
// initiator side (RFC 2661 Section 6.2). The peer's SCCRP carries the
// tunnel ID it assigned for us (Assigned Tunnel ID AVP), which becomes the
// TunnelID field of every message we send from now on. Authentication is
// symmetric with the answering side:
//
//   - If we sent a Challenge (shared secret configured), the SCCRP MUST
//     carry a valid Challenge Response computed with the SCCRP CHAP ID; a
//     missing or wrong response tears the tunnel down with StopCCN RC=4.
//   - If the SCCRP carries the peer's own Challenge, we answer it in the
//     SCCCN with the SCCCN CHAP ID; if we have no shared secret we cannot
//     answer and tear down with RC=4.
//
// On success the SCCCN is enqueued and the tunnel is established.
func (t *L2TPTunnel) handleSCCRP(now time.Time, defaults TunnelDefaults, payload []byte) []sendRequest {
	if t.state != L2TPTunnelWaitCtlReply {
		t.logger.Debug("l2tp: SCCRP on non-wait-ctl-reply tunnel ignored", "state", t.state.String())
		return nil
	}
	sccrp, err := parseSCCRP(payload)
	if err != nil {
		t.logger.Warn("l2tp: malformed SCCRP; sending StopCCN RC=1", "error", err.Error())
		return t.teardownStopCCN(now, resultGeneralError, l2tpevents.TerminateCauseNASError)
	}

	// Adopt the peer's assigned tunnel ID for every subsequent outbound
	// header (SCCCN, HELLO, session-scoped messages, and any StopCCN we may
	// emit below on an auth failure).
	t.remoteTID = sccrp.AssignedTunnelID
	t.engine.setPeerTunnelID(sccrp.AssignedTunnelID)
	t.peerHostName = sccrp.HostName
	t.peerFraming = sccrp.FramingCapabilities
	t.peerBearer = sccrp.BearerCapabilities
	t.peerRecvWindow = sccrp.RecvWindow

	// An initiator dialing a specific remote uses that remote's secret; fall
	// back to the reactor's global default for a dial with no per-remote
	// secret (Phase 1 tunnel-level tests take this branch).
	secret := defaults.SharedSecret
	if t.initiatorSecret != "" {
		secret = t.initiatorSecret
	}

	// Authenticate the peer to us: verify its response to our Challenge.
	if t.ourChallenge != nil {
		if !sccrp.ChallengeResponsePresent {
			t.logger.Warn("l2tp: SCCRP missing Challenge Response; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized, l2tpevents.TerminateCauseNASError)
		}
		if !VerifyChallengeResponse(ChapIDSCCRP, []byte(secret), t.ourChallenge, sccrp.ChallengeResponseValue) {
			t.logger.Warn("l2tp: SCCRP Challenge Response did not verify; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized, l2tpevents.TerminateCauseNASError)
		}
	}

	// Authenticate us to the peer: answer its Challenge in the SCCCN.
	var ourResponse []byte
	if sccrp.ChallengePresent {
		if secret == "" {
			t.logger.Warn("l2tp: SCCRP Challenge AVP present but shared-secret is unset; sending StopCCN RC=4")
			return t.teardownStopCCN(now, resultNotAuthorized, l2tpevents.TerminateCauseNASError)
		}
		resp := ChallengeResponse(ChapIDSCCCN, []byte(secret), sccrp.ChallengeValue)
		ourResponse = resp[:]
	}

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeSCCCNBody(*bodyBuf, ourResponse)

	wire, enqErr := t.engine.Enqueue(0, (*bodyBuf)[:n], now, false)
	if enqErr != nil {
		t.logger.Warn("l2tp: SCCCN enqueue failed; tunnel stays wait-ctl-reply", "error", enqErr.Error())
		return nil
	}
	t.transition(L2TPTunnelEstablished, "SCCRP received")
	// Seed the dead-peer liveness clock (see handleSCCCN for rationale).
	t.lastLiveness = now
	// Release the challenge; no consumer reads it past established.
	t.ourChallenge = nil
	t.logger.Info("l2tp: tunnel now established (initiator)",
		"peer-host", strconv.Quote(sccrp.HostName),
		"peer-tid", t.remoteTID,
		"challenge", ourResponse != nil)
	return []sendRequest{{to: t.peerAddr, bytes: wire}}
}

// writeSCCRQBody writes the AVP body of an SCCRQ into buf starting at offset
// 0 and returns the byte length written. RFC 2661 Section 6.1 required AVPs:
// Message Type, Protocol Version, Host Name, Framing Capabilities, Assigned
// Tunnel ID, Receive Window Size; Bearer Capabilities is required for a LAC
// that may place calls. A non-empty challenge appends a mandatory Challenge
// AVP; a non-empty 8-byte tieBreaker appends an optional Tie Breaker AVP.
// RFC 2661 Section 4.4.3 defines both. Caller supplies a pooled buffer;
// no `append` or `make`.
//
// Uses `off += Write*` because ze's L2TP wire helpers return bytes written,
// NOT a new offset (see writeSCCRPBody).
func writeSCCRQBody(buf []byte, localTID uint16, d TunnelDefaults, challenge, tieBreaker []byte) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, protocolVersionValue[:])
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, d.FramingCapabilities)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, d.BearerCapabilities)
	off += WriteAVPString(buf, off, true, AVPHostName, d.HostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, localTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, d.RecvWindow)
	if len(challenge) > 0 {
		off += WriteAVPBytes(buf, off, true, 0, AVPChallenge, challenge)
	}
	if len(tieBreaker) == 8 {
		off += WriteAVPBytes(buf, off, false, 0, AVPTieBreaker, tieBreaker)
	}
	return off
}

// writeSCCCNBody writes the AVP body of an SCCCN into buf starting at offset
// 0 and returns the byte length written. RFC 2661 Section 6.3: Message Type,
// plus a Challenge Response AVP when the peer challenged us in its SCCRP.
func writeSCCCNBody(buf, challengeResponse []byte) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCCN))
	if len(challengeResponse) > 0 {
		off += WriteAVPBytes(buf, off, true, 0, AVPChallengeResponse, challengeResponse)
	}
	return off
}

// sccrpInfo collects the fields parseSCCRP pulls out of an SCCRP AVP stream.
type sccrpInfo struct {
	MessageType              MessageType
	ProtocolVersion          uint16
	FramingCapabilities      uint32
	BearerCapabilities       uint32
	HostName                 string
	AssignedTunnelID         uint16
	RecvWindow               uint16
	ChallengePresent         bool
	ChallengeValue           []byte
	ChallengeResponsePresent bool
	ChallengeResponseValue   []byte
}

// parseSCCRP walks the AVP stream of an SCCRP body and collects the fields
// the initiator FSM needs. Message Type AVP MUST be first per RFC 2661
// Section 4.4.1; Assigned Tunnel ID MUST be present and non-zero (RFC 2661
// Section 6.2) because it becomes the TunnelID of every message we send.
// Mirrors parseSCCRQ's structure and mandatory-AVP handling.
func parseSCCRP(payload []byte) (sccrpInfo, error) {
	var info sccrpInfo
	iter := NewAVPIterator(payload)
	first := true
	for {
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			if err := iter.Err(); err != nil {
				return sccrpInfo{}, err
			}
			break
		}
		if flags&FlagReserved != 0 {
			if flags&FlagMandatory != 0 {
				return sccrpInfo{}, fmt.Errorf("l2tp: mandatory SCCRP AVP type %d with reserved bits set", attrType)
			}
			continue
		}
		if skip, err := skipHiddenAVP("SCCRP", attrType, flags); err != nil {
			return sccrpInfo{}, err
		} else if skip {
			continue
		}
		if vendorID != 0 {
			if flags&FlagMandatory != 0 {
				return sccrpInfo{}, fmt.Errorf("l2tp: mandatory SCCRP vendor %d AVP not recognized", vendorID)
			}
			continue
		}
		if first {
			if attrType != AVPMessageType {
				return sccrpInfo{}, errors.New("l2tp: first SCCRP AVP must be Message Type (RFC 2661 S4.4.1)")
			}
			mt, rerr := readAVPUint16(value)
			if rerr != nil {
				return sccrpInfo{}, fmt.Errorf("l2tp: read SCCRP message type: %w", rerr)
			}
			info.MessageType = MessageType(mt)
			if info.MessageType != MsgSCCRP {
				return sccrpInfo{}, fmt.Errorf("l2tp: expected SCCRP (2), got %d", info.MessageType)
			}
			first = false
			continue
		}
		switch attrType { //nolint:exhaustive // only known AVPs handled; unknown skipped per RFC
		case AVPProtocolVersion:
			if len(value) >= 2 {
				info.ProtocolVersion = binary.BigEndian.Uint16(value[:2])
			}
		case AVPFramingCapabilities:
			if v, rerr := readAVPUint32(value); rerr == nil {
				info.FramingCapabilities = v
			}
		case AVPBearerCapabilities:
			if v, rerr := readAVPUint32(value); rerr == nil {
				info.BearerCapabilities = v
			}
		case AVPHostName:
			info.HostName = string(value)
		case AVPAssignedTunnelID:
			v, rerr := readAVPUint16(value)
			if rerr != nil {
				return sccrpInfo{}, fmt.Errorf("l2tp: read SCCRP assigned tunnel id: %w", rerr)
			}
			if v == 0 {
				return sccrpInfo{}, errors.New("l2tp: SCCRP Assigned Tunnel ID AVP must be non-zero")
			}
			info.AssignedTunnelID = v
		case AVPReceiveWindowSize:
			if v, rerr := readAVPUint16(value); rerr == nil {
				info.RecvWindow = v
			}
		case AVPChallenge:
			// RFC 2661 S4.4.3: "The Challenge is one or more octets of
			// random data". An empty one would make the response trivially
			// forgeable and trip the ChallengeResponse panic guard.
			if len(value) == 0 {
				return sccrpInfo{}, errors.New("l2tp: SCCRP Challenge AVP must carry at least one octet (RFC 2661 S4.4.3)")
			}
			info.ChallengePresent = true
			info.ChallengeValue = append([]byte(nil), value...)
		case AVPChallengeResponse:
			info.ChallengeResponsePresent = true
			info.ChallengeResponseValue = append([]byte(nil), value...)
		}
	}
	if first {
		return sccrpInfo{}, errors.New("l2tp: empty SCCRP body")
	}
	if info.AssignedTunnelID == 0 {
		return sccrpInfo{}, errors.New("l2tp: SCCRP missing Assigned Tunnel ID AVP (RFC 2661 S6.2)")
	}
	return info, nil
}
