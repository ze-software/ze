// RFC: rfc/short/rfc5561.md -- gated MUST coverage for a Capability Parameter ze
// does not support
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP plugin
// Related: rfc5036_test.go -- rfcTestSession; sessionparams_rfc5036_test.go --
// expectNoPDU
//
// VALIDATES: an Initialization message carrying a Capability Parameter whose U-bit
// is 1 is processed as if the parameter were absent (RFC 5561 Section 6): the
// Common Session Parameters beside it are read, the session goes operational, and
// no Notification answers it. The Dynamic Capability Announcement TLV of Section 9
// is one of the two code points tried.
// PREVENTS: a decoder that stops at the first TLV it does not know, and a session
// that NAKs an Initialization over an optional parameter the peer marked ignorable.
package ldp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// tlvTypeDynamicCapability is the Dynamic Capability Announcement TLV code point
// (RFC 5561 Section 9), which ze neither sends nor reads.
const tlvTypeDynamicCapability uint16 = 0x0506

// capabilityCases are the Capability Parameter code points a peer can put in its
// Initialization: the one well-known capability of RFC 5561 and one that no
// document assigns, so the result cannot rest on a code point ze recognizes.
var capabilityCases = []struct {
	name      string
	codePoint uint16
}{
	{name: "dynamic-capability-announcement", codePoint: tlvTypeDynamicCapability},
	{name: "unassigned-code-point", codePoint: 0x0520},
}

// encodeInitPDUWithCapability builds a complete Initialization PDU: the Common
// Session Parameters TLV proposing keepalive, then one Capability Parameter TLV of
// the given code point with the U-bit set, the F-bit clear, and a one-octet value
// carrying S=1 (RFC 5561 Section 3).
func encodeInitPDUWithCapability(codePoint, keepalive uint16) []byte {
	var value [14]byte
	binary.BigEndian.PutUint16(value[0:2], ldpVersion)
	binary.BigEndian.PutUint16(value[2:4], keepalive)
	binary.BigEndian.PutUint16(value[6:8], 4096)
	copy(value[8:12], []byte{10, 0, 0, 1})

	var buf [128]byte
	off := ldpHeaderLen
	off += encodeMessageHeader(buf[off:], MessageHeader{Type: MsgTypeInitialize, MessageID: 7})
	off += EncodeTLV(buf[off:], TLV{Type: TLVTypeCommonSession, Length: 14, Value: value[:]})
	off += EncodeTLV(buf[off:], TLV{
		Type:   tlvUnknownBit | codePoint,
		Length: 1,
		Value:  []byte{0x80},
	})
	bodyLen := off - ldpHeaderLen
	binary.BigEndian.PutUint16(buf[ldpHeaderLen+2:ldpHeaderLen+4], uint16(bodyLen-ldpTLVHdrLen))
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(bodyLen + 6),
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	return buf[:off]
}

// --------------------------------------------------------------------------
// RFC5561-6-4 -- a Capability Parameter with U=1 is ignored and the session is
// established
// --------------------------------------------------------------------------

// RFC requirement: RFC5561-6-4 positive -- an Initialization carrying a Capability
// Parameter ze does not support, with its U-bit set, is decoded as if the parameter
// were absent: the Common Session Parameters beside it are read, and the session
// goes operational with the keepalive the peer proposed.
func TestRFC5561UnknownCapabilityWithUBitSetIsIgnored(t *testing.T) {
	for _, tc := range capabilityCases {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = remote.Close() }()

			pdu := encodeInitPDUWithCapability(tc.codePoint, 30)
			msg, err := DecodeInit(7, pdu[ldpHeaderLen+ldpMsgHdrLen:])
			if err != nil {
				t.Fatalf("DecodeInit: %v", err)
			}
			if msg.KeepaliveTime != 30 {
				t.Errorf("keepalive = %d, want 30: the Common Session Parameters were not read", msg.KeepaliveTime)
			}
			if msg.ReceiverLSRID != [4]byte{10, 0, 0, 1} {
				t.Errorf("receiver LSR ID = %v, want 10.0.0.1", msg.ReceiverLSRID)
			}

			rx := rfcTestSession(local)
			rx.state = StateOpenSent
			if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0, nil, nil, nil); err != nil {
				t.Fatalf("processMessages: %v", err)
			}
			if rx.State() != StateOperational {
				t.Errorf("state = %s, want operational: a U=1 Capability Parameter MUST allow the session to be established", rx.State())
			}
			if got := rx.currentKeepalive(); got != 30*time.Second {
				t.Errorf("keepalive = %v, want 30s: the Capability Parameter disturbed the negotiation", got)
			}
		})
	}
}

// RFC requirement: RFC5561-6-4 negative -- an Initialization carrying a Capability
// Parameter ze does not support, with its U-bit set, is answered by no Notification:
// processMessages returns no error and nothing is written to the peer.
func TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification(t *testing.T) {
	for _, tc := range capabilityCases {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = remote.Close() }()

			pdu := encodeInitPDUWithCapability(tc.codePoint, 30)
			rx := rfcTestSession(local)
			rx.state = StateOpenSent
			if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0, nil, nil, nil); err != nil {
				t.Fatalf("processMessages returned %v: the Initialization was refused", err)
			}
			expectNoPDU(t, remote, "a Notification was sent for a Capability Parameter whose U-bit is 1")
		})
	}
}
