// Design: docs/functional-tests.md -- fixture decisions have unit controls.
// Related: tunnel_fixture_l2tp_slow_accounting.go -- the decisions under test;
// the daemon path is test/l2tp/slow-accounting-control.ci.
package fixture

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// slowAccountingTestSecret is the shared secret the fixture's server verifies
// every Accounting-Request with, the .ci's testing123.
const slowAccountingTestSecret = "testing123"

// slowAccountingTestAdminReset is Acct-Terminate-Cause 6, Admin Reset
// (RFC 2866 Section 5.10), the cause ze reports for the LAC's CDN.
const slowAccountingTestAdminReset = uint32(6)

// slowAccountingStopShape is one Accounting-Request the tests encode. omit
// names attribute types left out of the datagram.
type slowAccountingStopShape struct {
	code       uint8
	identifier uint8
	userName   string
	status     uint32
	sessionID  string
	cause      uint32
	delay      uint32
	omit       map[uint8]bool
}

// slowAccountingOriginalStop is the session's Stop as Client.exchange first
// sends it: Acct-Delay-Time zero, appended last as setAcctDelayTime does.
func slowAccountingOriginalStop() slowAccountingStopShape {
	return slowAccountingStopShape{
		code:       radius.CodeAccountingReq,
		identifier: 7,
		userName:   "lac-user",
		status:     radius.AcctStatusStop,
		sessionID:  "0000002a",
		cause:      slowAccountingTestAdminReset,
	}
}

// slowAccountingDelayedRetry is the client's retransmission of the original
// Stop: a new Identifier, and a delay counted from the first send.
func slowAccountingDelayedRetry() slowAccountingStopShape {
	shape := slowAccountingOriginalStop()
	shape.identifier = 8
	shape.delay = 5
	return shape
}

// slowAccountingWire encodes shape with the radius package's own encoder and
// signs it with the RFC 2866 Section 3 Request Authenticator under secret.
func slowAccountingWire(t *testing.T, shape slowAccountingStopShape, secret string) []byte {
	t.Helper()
	attrs := []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString(shape.userName)},
		{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(shape.status)},
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString(shape.sessionID)},
		{Type: radius.AttrAcctTerminateCause, Value: radius.AttrUint32(shape.cause)},
		{Type: radius.AttrAcctDelayTime, Value: radius.AttrUint32(shape.delay)},
	}
	kept := attrs[:0]
	for _, attr := range attrs {
		if !shape.omit[attr.Type] {
			kept = append(kept, attr)
		}
	}
	packet := radius.Packet{Code: shape.code, Identifier: shape.identifier, Attrs: kept}
	return slowAccountingSign(t, &packet, secret)
}

// slowAccountingSign encodes packet and writes its Accounting-Request
// authenticator over the encoded bytes.
func slowAccountingSign(t *testing.T, packet *radius.Packet, secret string) []byte {
	t.Helper()
	buffer := make([]byte, radius.MaxPacketLen)
	length, err := packet.EncodeTo(buffer, 0)
	if err != nil {
		t.Fatal(err)
	}
	authenticator := radius.AccountingRequestAuth(buffer, length, []byte(secret))
	copy(buffer[4:radius.HeaderLen], authenticator[:])
	return buffer[:length]
}

// TestSlowAccountingRequestWireReadsASignedStop proves the fixture's reader
// of an Accounting-Request. Goal: it keeps every attribute octet but the
// Acct-Delay-Time value, reads the delay, the Identifier and the Request
// Authenticator, and refuses a datagram it cannot vouch for. Method: encode
// real requests with radius.Packet.EncodeTo and radius.AccountingRequestAuth,
// compare the accepted one with the same Stop encoded at delay zero, then
// feed each defect and name the refusal it must get.
func TestSlowAccountingRequestWireReadsASignedStop(t *testing.T) {
	delayed := slowAccountingDelayedRetry()
	datagram := slowAccountingWire(t, delayed, slowAccountingTestSecret)
	request, err := slowAccountingRequestWire(datagram)
	if err != nil {
		t.Fatal(err)
	}
	if request.delay != delayed.delay {
		t.Fatalf("delay = %d, want %d", request.delay, delayed.delay)
	}
	if request.identifier != delayed.identifier {
		t.Fatalf("identifier = %d, want %d", request.identifier, delayed.identifier)
	}
	if !bytes.Equal(request.authenticator[:], datagram[4:radius.HeaderLen]) {
		t.Fatal("the authenticator read is not the datagram's")
	}
	undelayed := delayed
	undelayed.delay = 0
	if want := slowAccountingWire(t, undelayed, slowAccountingTestSecret)[radius.HeaderLen:]; !bytes.Equal(request.immutable, want) {
		t.Fatalf("immutable = %x, want the attributes with the delay cleared %x", request.immutable, want)
	}

	cases := []struct {
		name     string
		datagram func(t *testing.T) []byte
		want     string
	}{
		{"short", func(_ *testing.T) []byte { return datagram[:radius.HeaderLen-1] }, "short original Stop datagram"},
		{"access-request", func(t *testing.T) []byte {
			shape := slowAccountingOriginalStop()
			shape.code = radius.CodeAccessRequest
			return slowAccountingWire(t, shape, slowAccountingTestSecret)
		}, "original Stop is not an Accounting-Request"},
		{"trailing-octet", func(_ *testing.T) []byte { return append(append([]byte(nil), datagram...), 0) }, "original Stop length does not describe its full datagram"},
		{"bad-signature", func(t *testing.T) []byte {
			return slowAccountingWire(t, slowAccountingOriginalStop(), "not-the-secret")
		}, "original Stop Request Authenticator does not verify"},
		{"attribute-past-datagram", func(t *testing.T) []byte {
			// The encoder cannot write a lying attribute length, so the last
			// attribute's length octet is raised and the datagram re-signed.
			packet, err := radius.Decode(datagram)
			if err != nil {
				t.Fatal(err)
			}
			wire := slowAccountingSign(t, packet, slowAccountingTestSecret)
			wire[len(wire)-5]++
			authenticator := radius.AccountingRequestAuth(wire, len(wire), []byte(slowAccountingTestSecret))
			copy(wire[4:radius.HeaderLen], authenticator[:])
			return wire
		}, "original Stop has an attribute past its datagram"},
		{"two-octet-delay", func(t *testing.T) []byte {
			packet := radius.Packet{Code: radius.CodeAccountingReq, Identifier: 7, Attrs: []radius.Attr{
				{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(radius.AcctStatusStop)},
				{Type: radius.AttrAcctDelayTime, Value: []byte{0, 5}},
			}}
			return slowAccountingSign(t, &packet, slowAccountingTestSecret)
		}, "the original Stop's Acct-Delay-Time is not a four-octet measurement"},
		{"missing-delay", func(t *testing.T) []byte {
			shape := slowAccountingOriginalStop()
			shape.omit = map[uint8]bool{radius.AttrAcctDelayTime: true}
			return slowAccountingWire(t, shape, slowAccountingTestSecret)
		}, "original Stop does not carry exactly one Acct-Delay-Time"},
		{"two-delays", func(t *testing.T) []byte {
			packet, err := radius.Decode(datagram)
			if err != nil {
				t.Fatal(err)
			}
			packet.Attrs = append(packet.Attrs, radius.Attr{Type: radius.AttrAcctDelayTime, Value: radius.AttrUint32(9)})
			return slowAccountingSign(t, packet, slowAccountingTestSecret)
		}, "original Stop does not carry exactly one Acct-Delay-Time"},
	}
	for _, refusal := range cases {
		t.Run(refusal.name, func(t *testing.T) {
			_, err := slowAccountingRequestWire(refusal.datagram(t))
			if err == nil {
				t.Fatalf("accepted, want %q", refusal.want)
			}
			if err.Error() != refusal.want {
				t.Fatalf("refused with %q, want %q", err, refusal.want)
			}
		})
	}
}

// TestSlowAccountingRetryAcceptsOnlyADelayedRetransmission proves the
// server's test of "a real delayed retry of the original Stop". Goal: the
// withheld Stop is answered only for a datagram carrying the same attributes
// apart from Acct-Delay-Time, a larger delay, a new Identifier and Request
// Authenticator, and a valid signature. Method: run each encoded candidate
// through the two steps the server runs, slowAccountingRequestWire then
// slowAccountingRetry against the original, and name the refusal each defect
// must get, so a red says which guard stopped deciding.
func TestSlowAccountingRetryAcceptsOnlyADelayedRetransmission(t *testing.T) {
	first, err := slowAccountingRequestWire(slowAccountingWire(t, slowAccountingOriginalStop(), slowAccountingTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	judge := func(datagram []byte) error {
		request, err := slowAccountingRequestWire(datagram)
		if err != nil {
			return err
		}
		return slowAccountingRetry(first, request)
	}

	if err := judge(slowAccountingWire(t, slowAccountingDelayedRetry(), slowAccountingTestSecret)); err != nil {
		t.Fatalf("the delayed retry was refused: %v", err)
	}

	cases := []struct {
		name  string
		shape func() slowAccountingStopShape
		key   string
		want  string
	}{
		{"identical-retransmission", slowAccountingOriginalStop, slowAccountingTestSecret,
			"the Stop candidate is not a delayed retry of the original exchange"},
		{"same-identifier", func() slowAccountingStopShape {
			shape := slowAccountingDelayedRetry()
			shape.identifier = slowAccountingOriginalStop().identifier
			return shape
		}, slowAccountingTestSecret, "the delayed Stop retry reused the first Identifier"},
		{"changed-attribute", func() slowAccountingStopShape {
			shape := slowAccountingDelayedRetry()
			shape.userName = "lac-usex"
			return shape
		}, slowAccountingTestSecret, "the Stop retry changed the original immutable accounting attributes"},
		{"zero-delay", func() slowAccountingStopShape {
			shape := slowAccountingDelayedRetry()
			shape.delay = 0
			return shape
		}, slowAccountingTestSecret, "the Stop candidate is not a delayed retry of the original exchange"},
		{"missing-delay", func() slowAccountingStopShape {
			shape := slowAccountingDelayedRetry()
			shape.omit = map[uint8]bool{radius.AttrAcctDelayTime: true}
			return shape
		}, slowAccountingTestSecret, "original Stop does not carry exactly one Acct-Delay-Time"},
		{"bad-signature", slowAccountingDelayedRetry, "not-the-secret",
			"original Stop Request Authenticator does not verify"},
	}
	for _, refusal := range cases {
		t.Run(refusal.name, func(t *testing.T) {
			err := judge(slowAccountingWire(t, refusal.shape(), refusal.key))
			if err == nil {
				t.Fatalf("accepted, want %q", refusal.want)
			}
			if err.Error() != refusal.want {
				t.Fatalf("refused with %q, want %q", err, refusal.want)
			}
		})
	}

	// A signed datagram with a new Identifier and delay cannot repeat the
	// first MD5 authenticator, so the last guard is reached by giving a
	// decoded retry the first request's authenticator.
	retry, err := slowAccountingRequestWire(slowAccountingWire(t, slowAccountingDelayedRetry(), slowAccountingTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	retry.authenticator = first.authenticator
	want := "the delayed Stop retry reused the first Request Authenticator"
	if err := slowAccountingRetry(first, retry); err == nil || err.Error() != want {
		t.Fatalf("a reused authenticator gave %v, want %q", err, want)
	}
}

// TestSlowAccountingStopReadsTheDecodedLine proves which Accounting-Requests
// the server treats as the session's Stop. Goal: a Start or Interim-Update is
// not a Stop, a Stop answers its Acct-Session-Id, and a Stop without that
// identity or without Admin Reset as its cause is refused. Method: encode
// each request with the radius package, render it with tunnelRadiusDescribe
// as the server does, and judge the line with slowAccountingStop.
func TestSlowAccountingStopReadsTheDecodedLine(t *testing.T) {
	stop := slowAccountingOriginalStop()
	line := tunnelRadiusDescribe(slowAccountingWire(t, stop, slowAccountingTestSecret))
	owner, isStop, err := slowAccountingStop(line)
	if err != nil {
		t.Fatal(err)
	}
	if !isStop {
		t.Fatalf("the Stop %q was not read as a Stop", line)
	}
	if owner != stop.sessionID {
		t.Fatalf("owner = %q, want %q", owner, stop.sessionID)
	}

	for _, status := range []uint32{radius.AcctStatusStart, radius.AcctStatusInterimUpdate} {
		shape := slowAccountingOriginalStop()
		shape.status = status
		line := tunnelRadiusDescribe(slowAccountingWire(t, shape, slowAccountingTestSecret))
		owner, isStop, err := slowAccountingStop(line)
		if err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		if isStop {
			t.Fatalf("status %d was read as a Stop", status)
		}
		if owner != "" {
			t.Fatalf("status %d answered owner %q", status, owner)
		}
	}

	cases := []struct {
		name  string
		shape func() slowAccountingStopShape
		want  string
	}{
		{"no-status", func() slowAccountingStopShape {
			shape := slowAccountingOriginalStop()
			shape.omit = map[uint8]bool{radius.AttrAcctStatusType: true}
			return shape
		}, "an Accounting-Request has no decoded status"},
		{"no-session-id", func() slowAccountingStopShape {
			shape := slowAccountingOriginalStop()
			shape.omit = map[uint8]bool{radius.AttrAcctSessionID: true}
			return shape
		}, "a Stop has no original accounting identity"},
		{"no-cause", func() slowAccountingStopShape {
			shape := slowAccountingOriginalStop()
			shape.omit = map[uint8]bool{radius.AttrAcctTerminateCause: true}
			return shape
		}, "the Stop carries no Acct-Terminate-Cause"},
		{"user-request-cause", func() slowAccountingStopShape {
			shape := slowAccountingOriginalStop()
			shape.cause = 1 // RFC 2866 Section 5.10: 1 User Request
			return shape
		}, "the Stop reports Acct-Terminate-Cause 1, not 6 (Admin Reset) for the LAC's administrative CDN"},
	}
	for _, refusal := range cases {
		t.Run(refusal.name, func(t *testing.T) {
			_, _, err := slowAccountingStop(tunnelRadiusDescribe(slowAccountingWire(t, refusal.shape(), slowAccountingTestSecret)))
			if err == nil {
				t.Fatalf("accepted, want %q", refusal.want)
			}
			if err.Error() != refusal.want {
				t.Fatalf("refused with %q, want %q", err, refusal.want)
			}
		})
	}
}

// TestSlowAccountingHelloAckNamesTheHealthyTunnel proves the check that ze
// acknowledged the healthy tunnel's HELLO. Goal: only a control ZLB for
// tunnel slowAccountingPeerTunnel, session 0, with Nr one past the HELLO's Ns
// is the ACK; a ZLB with an earlier Nr is read past, and anything else is
// refused. Method: build each datagram with tunnelL2TPControl, the fixture's
// own header writer, and judge it with slowAccountingHelloAck.
func TestSlowAccountingHelloAckNamesTheHealthyTunnel(t *testing.T) {
	const helloNS = uint16(2)
	wantNR := helloNS + 1
	acked, err := slowAccountingHelloAck(tunnelL2TPControl(slowAccountingPeerTunnel, 0, 1, wantNR, nil), wantNR)
	if err != nil {
		t.Fatal(err)
	}
	if !acked {
		t.Fatal("the ZLB acknowledging the HELLO was not read as its ACK")
	}

	acked, err = slowAccountingHelloAck(tunnelL2TPControl(slowAccountingPeerTunnel, 0, 1, helloNS, nil), wantNR)
	if err != nil {
		t.Fatalf("the SCCCN's ZLB was refused: %v", err)
	}
	if acked {
		t.Fatal("the ZLB acknowledging the SCCCN was read as the HELLO's ACK")
	}

	cases := []struct {
		name     string
		datagram func() []byte
		want     string
	}{
		{"not-zlb", func() []byte {
			return tunnelL2TPControl(slowAccountingPeerTunnel, 0, 1, wantNR, tunnelL2TPAVP(true, 0, tunnelL2TPU16(6)))
		}, "the healthy tunnel received a 20-octet response, not a ZLB"},
		{"data-header", func() []byte {
			zlb := tunnelL2TPControl(slowAccountingPeerTunnel, 0, 1, wantNR, nil)
			binary.BigEndian.PutUint16(zlb[0:2], 0x4802) // T bit clear: a data message
			return zlb
		}, "the healthy tunnel received a response that is not a control ZLB"},
		{"other-tunnel", func() []byte {
			return tunnelL2TPControl(slowAccountingPeerTunnel-1, 0, 1, wantNR, nil)
		}, "the healthy ACK named another tunnel"},
		{"nonzero-session", func() []byte {
			return tunnelL2TPControl(slowAccountingPeerTunnel, 41, 1, wantNR, nil)
		}, "the healthy ACK named a session"},
	}
	for _, refusal := range cases {
		t.Run(refusal.name, func(t *testing.T) {
			acked, err := slowAccountingHelloAck(refusal.datagram(), wantNR)
			if err == nil {
				t.Fatalf("accepted (acked %t), want %q", acked, refusal.want)
			}
			if err.Error() != refusal.want {
				t.Fatalf("refused with %q, want %q", err, refusal.want)
			}
		})
	}
}
