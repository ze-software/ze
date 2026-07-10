// Design: docs/research/l2tpv2-implementation-guide.md -- S10 session FSMs, initiator half
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 7.6 (ICRQ), 7.7 (ICRP), 7.8 (ICCN),
//      7.9 (OCRQ), 7.10 (OCRP)
// Related: session_fsm.go -- the answering half (handleICRQ / handleOCRQ / parsers)

package l2tp

import (
	"fmt"
)

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
// Section 7.7). Message Type MUST be first (S4.1); Assigned Session ID is
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
				return 0, fmt.Errorf("l2tp: first %s AVP must be Message Type (RFC 2661 S4.1)", msgName)
			}
			mt, rerr := ReadAVPUint16(value)
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
			v, rerr := ReadAVPUint16(value)
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
