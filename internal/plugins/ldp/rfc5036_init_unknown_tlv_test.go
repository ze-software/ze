// RFC: rfc/short/rfc5036.md -- RFC5036-3.3-1, an unknown TLV with the U bit clear in an Initialization
// Related: wire.go -- DecodeInit, session.go -- Session.ignoreUnknownTLV

package ldp

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
)

// TestLDPInitUnknownTLVUClearNotifies sends an Initialization whose second TLV
// is a type no RFC registers with the U bit clear. DecodeInit must refuse it
// with unknownTLVError, and the session must answer an Unknown TLV
// Notification naming that Initialization and ignore the whole message: the
// session does not leave OpenSent and processMessages returns nil. The U=1 half
// stays with TestRFC5561UnknownCapabilityWithUBitSetIsIgnored.
//
// RFC 5036 Section 3.3: "Upon receipt of an unknown TLV, if U is clear (=0), a
// notification MUST be returned to the message originator and the entire
// message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently
// ignored and the rest of the message processed as if the unknown TLV did not
// exist."
//
// RFC requirement: RFC5036-3.3-1 negative -- an Initialization carrying an
// unknown TLV with the U bit clear is refused by DecodeInit, answered with an
// Unknown TLV Notification referring to message 7 of type Initialization, and
// ignored whole: the session stays in OpenSent (wire.go DecodeInit, session.go
// Session.ignoreUnknownTLV).
func TestLDPInitUnknownTLVUClearNotifies(t *testing.T) {
	pdu := encodeInitPDUWithCapability(tlvTypeUnassigned, 30)
	unknownOff := ldpHeaderLen + ldpMsgHdrLen + ldpTLVHdrLen + 14
	binary.BigEndian.PutUint16(pdu[unknownOff:unknownOff+2], tlvTypeUnassigned)

	_, err := DecodeInit(7, pdu[ldpHeaderLen+ldpMsgHdrLen:])
	var unknown *unknownTLVError
	if !errors.As(err, &unknown) {
		t.Fatalf("DecodeInit error = %v, want unknownTLVError", err)
	}

	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()
	rx := rfcTestSession(local)
	rx.state = StateOpenSent
	done := make(chan error, 1)
	go func() {
		done <- rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0, nil, nil, nil)
	}()

	status, referID, referType := readNotificationStatus(t, remote)
	if status != statusUnknownTLV {
		t.Errorf("status code = %#08x, want %#08x (Unknown TLV)", status, statusUnknownTLV)
	}
	if referID != 7 {
		t.Errorf("status refers to message %d, want the Initialization message id 7", referID)
	}
	if referType != MsgTypeInitialize {
		t.Errorf("status refers to message type %#x, want Initialization (%#x)", referType, MsgTypeInitialize)
	}
	if err := <-done; err != nil {
		t.Errorf("processMessages error = %v, want nil: the message is ignored, not fatal", err)
	}
	if rx.State() != StateOpenSent {
		t.Errorf("state = %s, want OpenSent: the Initialization must be ignored whole", rx.State())
	}
}
