// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6-PD codec for BNG PPP sessions
// Related: dhcpv6.go -- parseDUID and the DUID length bounds under test

package ppp

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
)

// duidTestOption appends one DHCPv6 option to b.
func duidTestOption(b []byte, code uint16, data []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint16(hdr[:], code)
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(data)))
	return append(append(b, hdr[:]...), data...)
}

// VALIDATES: the largest DUID RFC 9915 allows (a 2-octet type code plus 128
// identifier octets, 130 in all) is parsed, fits the reply buffer and is
// echoed whole; one octet more is refused at parse, and so is a DUID that
// carries a type code and no identifier.
//
// Source: accel-ppp-ng 8acbd5d (T9325), "fix reply buffer bounds and relay
// length encoding": accel-ppp wrote reply options without checking the
// buffer.
//
// RFC 9915 Section 11.1: "The length of the DUID (not including the type
// code) is at least 1 octet and at most 128 octets."
func TestNGDHCPv6LargestDUIDFitsReply(t *testing.T) {
	duid := func(n int) []byte {
		b := make([]byte, n)
		binary.BigEndian.PutUint16(b, 0xFF00) // an unregistered type: opaque ID
		return b
	}
	build := func(clientID []byte) []byte {
		b := []byte{DHCPv6Solicit, 4, 5, 6}
		b = duidTestOption(b, D6OptClientID, clientID)
		return duidTestOption(b, D6OptIAPD, make([]byte, 12))
	}

	msg, err := parseDHCPv6(build(duid(duidOctetsMax)))
	if err != nil {
		t.Fatalf("Solicit with a %d-octet DUID: %v", duidOctetsMax, err)
	}
	svc, _ := newTestIPv6Service()
	pool := func() (netip.Prefix, bool) { return netip.MustParsePrefix("2001:db8:1::/56"), true }
	resp, err := svc.handleDHCPv6(msg, testServerID(), pool)
	if err != nil || len(resp) == 0 {
		t.Fatalf("Solicit with a %d-octet DUID: reply %x err %v, want an Advertise", duidOctetsMax, resp, err)
	}
	reply, err := parseDHCPv6(resp)
	if err != nil {
		t.Fatalf("Advertise does not parse: %v", err)
	}
	if reply.ClientID == nil || len(reply.ClientID.ID) != duidOctetsMax-2 {
		t.Fatalf("Advertise Client Identifier = %+v, want the %d-octet DUID echoed", reply.ClientID, duidOctetsMax)
	}

	if _, err := parseDHCPv6(build(duid(duidOctetsMax + 1))); !errors.Is(err, errDHCPv6DUIDTooBig) {
		t.Fatalf("Solicit with a %d-octet DUID: error %v, want errDHCPv6DUIDTooBig", duidOctetsMax+1, err)
	}
	if _, err := parseDHCPv6(build(duid(duidTypeLen))); !errors.Is(err, errDHCPv6BadDUID) {
		t.Fatalf("Solicit with a type-only DUID: error %v, want errDHCPv6BadDUID", err)
	}
	if _, err := parseDHCPv6(build(duid(duidOctetsMin))); err != nil {
		t.Fatalf("Solicit with a %d-octet DUID: %v", duidOctetsMin, err)
	}
}
