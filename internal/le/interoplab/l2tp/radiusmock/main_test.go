package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
)

// VALIDATES: the Go peer decodes the same Access-Request fields and emits a valid Access-Accept shape.
// PREVENTS: replacing the independent peer with a logger that never participates on the RADIUS wire.
func TestHandleAccessRequest(t *testing.T) {
	packet := make([]byte, packetHeaderOctets)
	packet[0] = codeAccessRequest
	packet[1] = 7
	for index := 4; index < packetHeaderOctets; index++ {
		packet[index] = byte(index)
	}
	packet = appendAttribute(packet, attributeUserName, []byte("alice"))
	packet = appendAttribute(packet, attributeNASPortID, []byte("lns1:12.34"))
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	response, description, ok := handlePacket(packet, []byte("testing123"))
	if !ok {
		t.Fatal("Access-Request was not handled")
	}
	if got := hex.EncodeToString(response[:4]); got != "02070026" {
		t.Fatalf("Access-Accept header = %s, want code 2, identifier 7, length 38", got)
	}
	if response[20] != attributeMessageAuthenticator || response[21] != messageAuthenticatorAttributeOctets {
		t.Fatalf("first attribute = type %d length %d, want Message-Authenticator (80, 18)", response[20], response[21])
	}
	var requestAuthenticator [radius.AuthenticatorLen]byte
	copy(requestAuthenticator[:], packet[4:20])
	if !radius.VerifyResponseAuth(response, requestAuthenticator, []byte("testing123")) {
		t.Fatalf("Ze rejects the Response Authenticator of %s", hex.EncodeToString(response))
	}
	if !strings.Contains(description, "User-Name=alice") || !strings.Contains(description, "NAS-Port-Id=lns1:12.34") {
		t.Fatalf("description = %q", description)
	}
}

// VALIDATES: Ze's RADIUS client at its defaults, which discard an unsigned
// answer to an Access-Request (BlastRADIUS), accepts the mock's Access-Accept.
// PREVENTS: scenario 04 failing every subscriber login with "the answer
// carried no Message-Authenticator", as the mock did before it signed.
// Method: one Exchange through radius.Client against a loopback socket served
// by handlePacket, so both authenticators are judged by Ze's own verifier.
func TestAccessAcceptSatisfiesZesDefaultClient(t *testing.T) {
	secret := []byte("testing123")
	connection, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	}()
	served := make(chan error, 1)
	go serveOne(connection, secret, served)

	client, err := radius.NewClient(radius.ClientConfig{Timeout: 2 * time.Second, Retries: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Logf("client close: %v", closeErr)
		}
	}()
	authenticator, err := radius.RandomAuthenticator()
	if err != nil {
		t.Fatalf("RandomAuthenticator: %v", err)
	}
	request := &radius.Packet{
		Code:          radius.CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: authenticator,
		Attrs:         []radius.Attr{{Type: radius.AttrUserName, Value: []byte("alice")}},
	}
	answer, err := client.Exchange(t.Context(), request, radius.Server{Address: connection.LocalAddr().String(), SharedKey: secret})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if answer.Code != radius.CodeAccessAccept {
		t.Fatalf("answer code %d, want Access-Accept %d", answer.Code, radius.CodeAccessAccept)
	}
	if err := <-served; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// serveOne answers one datagram through handlePacket and reports the outcome.
func serveOne(connection *net.UDPConn, secret []byte, served chan<- error) {
	packet := make([]byte, packetOctetsMax)
	octets, peer, err := connection.ReadFromUDP(packet)
	if err != nil {
		served <- err
		return
	}
	response, _, ok := handlePacket(packet[:octets], secret)
	if !ok {
		served <- errors.New("handlePacket declined the Access-Request")
		return
	}
	_, err = connection.WriteToUDP(response, peer)
	served <- err
}

// VALIDATES: Accounting-Start reports the negotiated IPv4 address and stable port identity.
// PREVENTS: scenario 04 passing on an Access-Accept while accounting attributes are unread.
func TestDescribeAccountingRequest(t *testing.T) {
	attributes := []attribute{
		{kind: attributeAccountingStatus, value: []byte{0, 0, 0, 1}},
		{kind: attributeFramedIPAddress, value: []byte{10, 100, 0, 2}},
		{kind: attributeNASPortID, value: []byte("lns1:12.34")},
	}
	got := describe(codeAccountingRequest, attributes)
	for _, field := range []string{"Acct-Status-Type=Start", "Framed-IP-Address=10.100.0.2", "NAS-Port-Id=lns1:12.34"} {
		if !strings.Contains(got, field) {
			t.Errorf("description %q missing %q", got, field)
		}
	}
}

// VALIDATES: the peer decodes the four session attributes every subscriber
// Accounting-Request carries, so the checker can assert on them.
// PREVENTS: an attribute reaching the wire and going unread, which would let
// scenario 04 pass over a record the server never decoded.
func TestDescribeSessionAttributes(t *testing.T) {
	attributes := []attribute{
		{kind: attributeAccountingStatus, value: []byte{0, 0, 0, 2}},
		{kind: attributeCallingStationID, value: []byte("+441632960123")},
		{kind: attributeAccountingDelay, value: []byte{0, 0, 0, 3}},
		{kind: attributeTerminateCause, value: []byte{0, 0, 0, 6}},
		{kind: attributeEventTimestamp, value: []byte{0x68, 0xb8, 0x1e, 0x00}},
	}
	got := describe(codeAccountingRequest, attributes)
	for _, field := range []string{
		"Acct-Status-Type=Stop", "Calling-Station-Id=+441632960123",
		"Acct-Delay-Time=3", "Acct-Terminate-Cause=6", "Event-Timestamp=1756896768",
	} {
		if !strings.Contains(got, field) {
			t.Errorf("description %q missing %q", got, field)
		}
	}
}

// VALIDATES: malformed RADIUS attribute lengths stop parsing within the packet bound.
// PREVENTS: a peer-controlled attribute length panicking the mock and false-failing the interop scenario.
func TestParseAttributesRejectsMalformedLength(t *testing.T) {
	if got := parseAttributes([]byte{attributeUserName, 1}); len(got) != 0 {
		t.Fatalf("parsed malformed attributes: %#v", got)
	}
	if got := parseAttributes([]byte{attributeUserName, 8, 'a'}); len(got) != 0 {
		t.Fatalf("parsed truncated attributes: %#v", got)
	}
}

func appendAttribute(packet []byte, kind byte, value []byte) []byte {
	packet = append(packet, kind, byte(len(value)+2))
	return append(packet, value...)
}
