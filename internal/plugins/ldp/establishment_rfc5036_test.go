// RFC: rfc/short/rfc5036.md -- session establishment in the active role
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP plugin
// Related: rfc5036_test.go -- rfcTestSession, readLDPPDU, runSessionForTest, readNotificationStatus
//
// Ze dials every session it opens, so it always plays the active role of RFC 5036
// Section 2.5.3. These tests drive runSession, the function startSessionForAdj
// runs on the connection it dialed, and read what the peer end of the connection
// receives.
//
// VALIDATES: ze initiates the parameter negotiation with its Initialization and does
// not wait for the peer; ze answers an acceptable Initialization with a KeepAlive and
// sends none before it; ze answers an unacceptable Initialization with a Session
// Rejected Notification and closes the connection.
// PREVENTS: an establishment KeepAlive that signals acceptance of parameters ze has
// not yet received, and a rejected session whose connection stays open.
package ldp

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// runSessionToOperational drives sess through runSession to the operational state
// over remote. It reads ze's Initialization, answers with the peer's Initialization
// proposing peerKeepalive seconds, and reads the KeepAlive ze replies with. It
// returns the stop function of runSessionForTest.
func runSessionToOperational(t *testing.T, sess *Session, remote net.Conn, peerKeepalive uint16) func() {
	t.Helper()
	stop := runSessionForTest(t, sess)
	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		stop()
		t.Fatalf("first message = %#x, want Initialization (%#x)", msgHdr.Type, MsgTypeInitialize)
	}
	if _, err := remote.Write(encodeInitPDU(ldpVersion, peerKeepalive)); err != nil {
		stop()
		t.Fatalf("write peer Initialization: %v", err)
	}
	_, msgHdr, _ = readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeKeepAlive {
		stop()
		t.Fatalf("reply to the peer Initialization = %#x, want KeepAlive (%#x)", msgHdr.Type, MsgTypeKeepAlive)
	}
	return stop
}

// expectSilence fails the test when remote receives any octet inside window.
func expectSilence(t *testing.T, remote net.Conn, window time.Duration, what string) {
	t.Helper()
	if err := remote.SetReadDeadline(time.Now().Add(window)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var probe [1]byte
	_, err := remote.Read(probe[:])
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("read error = %v, want a deadline timeout: %s", err, what)
	}
}

// expectClosed fails the test unless the connection ze holds has been closed: a
// read on the peer end returns end of file, not a timeout and not data.
func expectClosed(t *testing.T, remote net.Conn) {
	t.Helper()
	if err := remote.SetReadDeadline(time.Now().Add(ldpReadTimeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var probe [1]byte
	n, err := remote.Read(probe[:])
	if !errors.Is(err, io.EOF) {
		t.Fatalf("read after the rejection = %d octets, error %v, want end of file: the connection was not closed", n, err)
	}
}

// RFC requirement: RFC5036-2.5.1-1 positive -- on the connection ze dialed, which
// makes ze the active LSR, runSession initiates the negotiation: with a peer that
// sends nothing, the first PDU the peer receives is ze's Initialization, carrying
// ze's LDP Identifier in its PDU header and the peer's in its Common Session
// Parameters. Ze does not wait for the peer to initiate.
func TestRFC5036ActiveRoleInitiatesNegotiation(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	pdu, msgHdr, body := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization (%#x)", msgHdr.Type, MsgTypeInitialize)
	}
	if pdu.LSRID != [4]byte{10, 0, 0, 1} || pdu.LabelSpace != 0 {
		t.Errorf("PDU LDP Identifier = %v:%d, want ze's 10.0.0.1:0", pdu.LSRID, pdu.LabelSpace)
	}
	initMsg, err := DecodeInit(msgHdr.MessageID, body)
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if initMsg.ReceiverLSRID != [4]byte{10, 0, 0, 2} || initMsg.ReceiverLabelSpace != 0 {
		t.Errorf("receiver LDP Identifier = %v:%d, want the peer's 10.0.0.2:0",
			initMsg.ReceiverLSRID, initMsg.ReceiverLabelSpace)
	}
}

// RFC requirement: RFC5036-2.5.3-5 positive -- ze, in the active role, receives an
// acceptable Initialization and replies with a KeepAlive: the next PDU after the
// peer's Initialization is a KeepAlive, and the session is operational.
func TestRFC5036ActiveRoleAcceptableInitAnsweredWithKeepAlive(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	if _, err := remote.Write(encodeInitPDU(ldpVersion, 30)); err != nil {
		t.Fatalf("write peer Initialization: %v", err)
	}
	_, msgHdr, _ = readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeKeepAlive {
		t.Fatalf("reply to an acceptable Initialization = %#x, want KeepAlive (%#x)", msgHdr.Type, MsgTypeKeepAlive)
	}
	if sess.State() != StateOperational {
		t.Errorf("state = %s, want operational", sess.State())
	}
}

// RFC requirement: RFC5036-2.5.3-5 negative -- the KeepAlive is a reply to an
// ACCEPTABLE Initialization and to nothing else: before the peer's Initialization
// arrives ze sends nothing after its own, and an unacceptable Initialization (a
// KeepAlive Time of 0) is answered with a Notification, not a KeepAlive.
func TestRFC5036ActiveRoleNoKeepAliveWithoutAcceptableInit(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	expectSilence(t, remote, 300*time.Millisecond,
		"ze sent a PDU after its Initialization before any Initialization from the peer")

	if _, err := remote.Write(encodeInitPDU(ldpVersion, 0)); err != nil {
		t.Fatalf("write peer Initialization: %v", err)
	}
	_, msgHdr, _ = readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeNotification {
		t.Fatalf("reply to an unacceptable Initialization = %#x, want Notification (%#x), never KeepAlive",
			msgHdr.Type, MsgTypeNotification)
	}
	if sess.State() == StateOperational {
		t.Error("session went operational on an unacceptable Initialization")
	}
}

// RFC requirement: RFC5036-2.5.3-6 positive -- ze, in the active role, receives an
// Initialization whose session parameters are unacceptable, sends a Session Rejected
// Notification for the parameter at fault, and closes the connection. Two
// unacceptable parameters are driven: a KeepAlive Time of 0 draws Session
// Rejected/Bad KeepAlive Time, and a Protocol Version other than 1 draws Bad
// Protocol Version (RFC 5036 Section 3.5.3), both with the E bit set and both
// naming the Initialization they answer.
func TestRFC5036ActiveRoleUnacceptableInitRejectedAndClosed(t *testing.T) {
	tests := []struct {
		name      string
		version   uint16
		keepalive uint16
		status    uint32
	}{
		{"keepalive-time-zero", ldpVersion, 0, statusSessionRejectedBadKeepaliveTime},
		{"protocol-version-2", 2, 30, statusBadProtocolVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = remote.Close() }()

			sess := rfcTestSession(local)
			stop := runSessionForTest(t, sess)
			defer stop()

			_, msgHdr, _ := readLDPPDU(t, remote)
			if msgHdr.Type != MsgTypeInitialize {
				t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
			}
			if _, err := remote.Write(encodeInitPDU(tt.version, tt.keepalive)); err != nil {
				t.Fatalf("write peer Initialization: %v", err)
			}
			status, referID, referType := readNotificationStatus(t, remote)
			if status != tt.status {
				t.Errorf("status code = %#08x, want %#08x", status, tt.status)
			}
			if referID != 7 || referType != MsgTypeInitialize {
				t.Errorf("status refers to message %d type %#x, want the Initialization, id 7 type %#x",
					referID, referType, MsgTypeInitialize)
			}
			expectClosed(t, remote)
			if sess.State() == StateOperational {
				t.Error("session went operational on an unacceptable Initialization")
			}
		})
	}
}

// RFC requirement: RFC5036-2.5.3-6 negative -- an Initialization whose parameters
// are acceptable draws no Notification and the connection stays open: the reply is a
// KeepAlive, and a second KeepAlive follows inside the negotiated KeepAlive Time.
func TestRFC5036ActiveRoleAcceptableInitKeepsConnection(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	stop := runSessionToOperational(t, sess, remote, 1)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeKeepAlive {
		t.Fatalf("message after the reply = %#x, want KeepAlive (%#x): no Notification, no close",
			msgHdr.Type, MsgTypeKeepAlive)
	}
	if sess.State() != StateOperational {
		t.Errorf("state = %s, want operational", sess.State())
	}
}
