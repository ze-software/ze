// Design: docs/research/l2tpv2-implementation-guide.md -- S9 tunnel FSM + S4 AVP handling
// Related: tunnel.go -- L2TPTunnel value and state enum

package l2tp

import (
	"bytes"
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
// transition. Phase 3 implements SCCRQ; other message types are logged
// and dropped for later phases to wire. Returns the outbound datagrams
// produced by the handler.
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
		t.logger.Debug("l2tp: SCCCN received (phase 4 completes the handshake)")
		return nil
	}
	if msgType == MsgStopCCN {
		t.logger.Debug("l2tp: StopCCN received (teardown lands in phase 5)")
		return nil
	}
	if msgType == MsgHello {
		t.logger.Debug("l2tp: Hello received (keepalive lands in phase 5)")
		return nil
	}
	t.logger.Debug("l2tp: unsupported message type ignored", "type", uint16(msgType))
	return nil
}

// handleSCCRQ is the idle -> wait-ctl-conn transition. The reactor has
// already validated the SCCRQ body (via parseSCCRQ) and hands us the
// pre-parsed info, so this function cannot fail on malformed AVPs.
// Engine dedup guarantees this runs at most once per tunnel.
//
// Phase 3 does not handle peer-side Challenge authentication. If the
// peer sent a Challenge AVP we log a WARN so operators see why the
// peer will eventually close the tunnel; phase 4 adds the MD5
// Challenge Response path.
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

	if sccrq.ChallengePresent {
		t.logger.Warn("l2tp: SCCRQ carries Challenge AVP; phase 3 does not authenticate, peer will close the tunnel")
	}

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	n := writeSCCRPBody(*bodyBuf, t.localTID, defaults)

	wire, err := t.engine.Enqueue(0, (*bodyBuf)[:n], now)
	if err != nil {
		t.logger.Warn("l2tp: SCCRP enqueue failed; tunnel stays idle", "error", err.Error())
		return nil
	}
	t.state = L2TPTunnelWaitCtlConn
	t.logger.Info("l2tp: SCCRP sent; tunnel now wait-ctl-conn",
		"peer-host", strconv.Quote(sccrq.HostName),
		"peer-tid", t.remoteTID,
		"peer-framing", fmt.Sprintf("0x%08x", sccrq.FramingCapabilities))
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
			info.ChallengePresent = true
			info.ChallengeValue = append([]byte(nil), value...)
			continue
		}
		if attrType == AVPTieBreaker {
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
// offset 0 and returns the byte length written. It omits Challenge and
// Challenge Response AVPs; phase 4 adds those when tunnel authentication
// lands. Caller supplies a pooled buffer; no `append` or `make`.
//
// Uses `off += Write*` because ze's L2TP wire helpers return bytes
// written, NOT new offset. Mixing `=` for one and `+=` for another
// corrupts the buffer silently.
func writeSCCRPBody(buf []byte, localTID uint16, d TunnelDefaults) int {
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRP))
	// Protocol Version AVP carries 2 bytes: ver=1, rev=0 -> 0x01 0x00.
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, protocolVersionValue[:])
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, d.FramingCapabilities)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, d.BearerCapabilities)
	off += WriteAVPString(buf, off, true, AVPHostName, d.HostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, localTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, d.RecvWindow)
	return off
}

// protocolVersionValue is the 2-byte Protocol Version AVP value (v1 rev0)
// per RFC 2661 S4.4.2. Shared across all outbound control messages.
var protocolVersionValue = [2]byte{0x01, 0x00}
