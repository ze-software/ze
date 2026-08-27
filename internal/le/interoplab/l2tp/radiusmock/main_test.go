package main

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// VALIDATES: the Go peer decodes the same Access-Request fields and emits a valid Access-Accept shape.
// PREVENTS: replacing the Python peer with a logger that never participates on the RADIUS wire.
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
	if got := hex.EncodeToString(response); got != "0207001442e691424943f17320626011bd24bf47" {
		t.Fatalf("Access-Accept bytes = %s", got)
	}
	if !strings.Contains(description, "User-Name=alice") || !strings.Contains(description, "NAS-Port-Id=lns1:12.34") {
		t.Fatalf("description = %q", description)
	}
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
