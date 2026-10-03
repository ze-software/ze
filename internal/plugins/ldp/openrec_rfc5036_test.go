// Design: docs/architecture/ldp/mpls-ldp.md -- active session initialization.
// RFC: rfc/short/rfc5036.md -- peer acceptance and OPENREC message handling.
// Related: establishment_rfc5036_test.go -- live session and wire assertions.
package ldp

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// RFC requirement: RFC5036-2.5.3-8 positive -- the peer's KeepAlive confirms our
// proposed parameters: after the Initialization exchange it makes the live session operational.
// RFC requirement: RFC5036-2.5.3-9 positive -- an acceptable Initialization followed
// by the peer's KeepAlive makes the active session operational.
// RFC requirement: RFC5036-2.5.4-1 negative -- KeepAlive in OPENREC is accepted,
// advances to OPERATIONAL, and produces neither a Notification nor a transport close.
// RFC requirement: RFC5036-2.5.4-2 negative -- acceptable Initialization in OPENSENT
// receives KeepAlive, advances to OPENREC, and permits establishment without a NAK.
func TestRFC5036OpenReceivedKeepAliveEstablishesSession(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	exchangeInitialization(t, remote)
	if sess.State() != StateOpenReceived {
		t.Fatalf("state = %s, want open-received before the peer's KeepAlive", sess.State())
	}
	// RFC 5036 Section 2.5.3 item 2.d.
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer KeepAlive: %v", err)
	}
	awaitOperational(t, sess)
	expectSilence(t, remote, 200*time.Millisecond, "the accepted KeepAlive produced a Notification or closed the session")
}

// RFC requirement: RFC5036-2.5.3-8 negative -- without the peer's KeepAlive, an
// accepted Initialization does not confirm the peer accepted our proposal.
// RFC requirement: RFC5036-2.5.3-9 negative -- the Initialization exchange alone
// leaves the active session in OPENREC rather than operational.
func TestRFC5036OpenReceivedWaitsForPeerKeepAlive(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	exchangeInitialization(t, remote)
	expectSilence(t, remote, 200*time.Millisecond, "no message is due before the peer's KeepAlive")
	if sess.State() != StateOpenReceived {
		t.Fatalf("state = %s, want open-received until the peer's KeepAlive", sess.State())
	}
}

// RFC requirement: RFC5036-2.5.3-9 negative -- a peer KeepAlive without an
// acceptable peer Initialization cannot establish the active session.
func TestRFC5036KeepAliveWithoutPeerInitializationNeverEstablishes(t *testing.T) {
	rec := recordSessionUps(t)
	local, remote := sessionPipe(t)
	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", hdr.Type)
	}
	// RFC 5036 Section 2.5.3 item 2.d requires both peer messages.
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer KeepAlive: %v", err)
	}
	status, referID, referType := readNotificationStatus(t, remote)
	if status != 0x8000000A {
		t.Errorf("status = %#08x, want fatal Shutdown 0x8000000a", status)
	}
	if referID != 9 || referType != MsgTypeKeepAlive {
		t.Errorf("reference = %d/%#x, want 9/%#x", referID, referType, MsgTypeKeepAlive)
	}
	expectClosed(t, remote)
	stop()
	if got := rec.recorded(); len(got) != 0 {
		t.Fatalf("premature KeepAlive published SessionUp: %+v", got)
	}
	if sess.State() == StateOperational {
		t.Fatal("KeepAlive without Initialization established the session")
	}
}

// RFC requirement: RFC5036-2.5.4-1 positive -- every other message in OPENREC
// receives an Error Notification with the fatal Shutdown status and the transport
// closes without making the session operational or applying a mapping/address.
// Unknown types exercise both U bits. RFC 5036 Section 3.5.1.1: "When an LSR
// receives a Shutdown message during session initialization, it SHOULD transmit
// a Shutdown message and then close the transport connection."
func TestRFC5036OpenReceivedOtherMessageRejected(t *testing.T) {
	testInitializationOtherMessageRejected(t, StateOpenReceived)
}

// TestRFC5036OpenSentOtherMessageRejected drives every non-Initialization type
// through the live active session, including the initialization Shutdown reply.
// RFC requirement: RFC5036-2.5.4-2 positive -- every non-Initialization message in
// OPENSENT draws fatal Shutdown with the offending reference, then EOF, without
// publishing SessionUp or applying a mapping/address; both unknown U bits are tried.
func TestRFC5036OpenSentOtherMessageRejected(t *testing.T) {
	testInitializationOtherMessageRejected(t, StateOpenSent)
}

// testInitializationOtherMessageRejected checks the RFC 5036 Section 2.5.4
// initialization type guard before any body can change session or label state.
func testInitializationOtherMessageRejected(t *testing.T, state SessionState) {
	t.Helper()
	cases := []struct {
		name string
		kind uint16
	}{
		{"hello", MsgTypeHello},
		{"initialization", MsgTypeInitialize},
		{"keepalive", MsgTypeKeepAlive},
		{"shutdown-notification", MsgTypeNotification},
		{"address", MsgTypeAddress},
		{"address-withdraw", MsgTypeAddressWithdraw},
		{"label-mapping", MsgTypeLabelMapping},
		{"label-request", MsgTypeLabelRequest},
		{"label-withdraw", MsgTypeLabelWithdraw},
		{"label-release", MsgTypeLabelRelease},
		{"label-abort-request", MsgTypeLabelAbortReq},
		{"unknown-u-clear", 0x1234},
		{"unknown-u-set", 0x9234},
	}
	for _, tc := range cases {
		if state == StateOpenSent && tc.kind == MsgTypeInitialize {
			continue
		}
		if state == StateOpenReceived && tc.kind == MsgTypeKeepAlive {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			rec := recordSessionUps(t)
			local, remote := sessionPipe(t)
			sess := rfcTestSession(local)
			stop := runSessionForTest(t, sess)
			defer stop()

			if state == StateOpenReceived {
				exchangeInitialization(t, remote)
			} else {
				if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeInitialize {
					t.Fatalf("first message = %#x, want Initialization", hdr.Type)
				}
			}
			var pdu [256]byte
			// Use a complete valid Mapping as the dispatch control. The other
			// messages need only a complete header: initialization rejects the type
			// before its body can be decoded or its U bit can authorize ignoring it.
			n := encodeLabelMapping(pdu[ldpHeaderLen:], labelMappingMessage{
				MessageID: 17,
				FEC:       FECElement{Type: FECPrefix, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
				Label:     100,
			})
			if tc.kind != MsgTypeLabelMapping {
				n = encodeMessageHeader(pdu[ldpHeaderLen:], MessageHeader{
					Type: tc.kind, Length: 4, MessageID: 17,
				})
			}
			switch tc.kind {
			case MsgTypeInitialize:
				n = EncodeInit(pdu[ldpHeaderLen:], initMessage{
					MessageID: 17, ProtocolVersion: ldpVersion, KeepaliveTime: 30, MaxPDULength: 4096,
				})
			case MsgTypeNotification:
				n = encodeNotification(pdu[ldpHeaderLen:], notificationMessage{
					MessageID: 17, Status: 0x8000000A,
				})
			case MsgTypeAddress, MsgTypeAddressWithdraw:
				// RFC 5036 Section 3.5.5: Address List TLV with one IPv4 address.
				body := pdu[ldpHeaderLen+ldpMsgHdrLen:]
				binary.BigEndian.PutUint16(body[0:2], TLVTypeAddressList)
				binary.BigEndian.PutUint16(body[2:4], 6)
				binary.BigEndian.PutUint16(body[4:6], AFIIPv4)
				copy(body[6:10], []byte{192, 0, 2, 2})
				n = ldpMsgHdrLen + 10
				binary.BigEndian.PutUint16(pdu[ldpHeaderLen+2:], uint16(n-4))
			}
			encodePDUHeader(pdu[:], PDUHeader{
				Version: ldpVersion, PDULength: uint16(n + 6), LSRID: [4]byte{10, 0, 0, 2},
			})
			if _, err := remote.Write(pdu[:ldpHeaderLen+n]); err != nil {
				t.Fatalf("write other message: %v", err)
			}
			status, referID, referType := readNotificationStatus(t, remote)
			if status != 0x8000000A {
				t.Errorf("status = %#08x, want fatal Shutdown 0x8000000a", status)
			}
			if referID != 17 || referType != tc.kind {
				t.Errorf("reference = %d/%#x, want 17/%#x", referID, referType, tc.kind)
			}
			expectClosed(t, remote)
			stop()
			if got := rec.recorded(); len(got) != 0 {
				t.Fatalf("unexpected message published SessionUp: %+v", got)
			}
			if sess.State() == StateOperational {
				t.Fatal("unexpected message established the session")
			}
			if binding, ok := sess.lib.LookupRemote(netip.MustParsePrefix("192.0.2.0/24"), "10.0.0.2:0"); ok {
				t.Errorf("Mapping applied before establishment: %+v", binding)
			}
			if got := sess.peerAddresses(); len(got) != 0 {
				t.Errorf("Address applied before establishment: %v", got)
			}
		})
	}
}
