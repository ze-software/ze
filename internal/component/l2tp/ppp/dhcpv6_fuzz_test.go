// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6-PD codec for BNG PPP sessions
// Related: dhcpv6.go -- parseDHCPv6, the parser under fuzz
// Related: ipv6_service.go -- handleDHCPv6, the server answer under fuzz

package ppp

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

// FuzzParseDHCPv6 drives arbitrary bytes through the DHCPv6-PD server path the
// PPP DHCPv6 reader runs: parseDHCPv6, then handleDHCPv6. The seeds are the
// malformed inputs behind accel-ppp-ng's DHCPv6 security fixes.
//
// Invariants: no panic; a reply answers only Solicit, Request, Renew or
// Release, carries the request's transaction-id, and is itself a message the
// parser accepts.
func FuzzParseDHCPv6(f *testing.F) {
	solicit := buildSolicit([3]byte{1, 2, 3},
		DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}, 1)
	opt := func(b []byte, code uint16, data []byte) []byte {
		var hdr [4]byte
		binary.BigEndian.PutUint16(hdr[:], code)
		binary.BigEndian.PutUint16(hdr[2:], uint16(len(data)))
		return append(append(append([]byte(nil), b...), hdr[:]...), data...)
	}
	relay := func(inner []byte) []byte {
		b := make([]byte, 34)
		b[0] = 12 // RELAY-FORW
		return opt(b, 9, inner)
	}
	nested := solicit
	for range 40 {
		nested = relay(nested)
	}
	seeds := [][]byte{
		nil,
		solicit,
		// accel-ppp-ng bd1806e (T8475): Relay-Forward nested past HOP_COUNT_LIMIT.
		nested,
		// accel-ppp-ng cc42de1 (T9325): inner Relay-Message shorter than a header.
		relay([]byte{1, 2}),
		// accel-ppp-ng ef78432 (T9325) and 24afb85 (T8549): short and unknown Status-Code.
		opt(solicit, D6OptStatusCode, nil),
		opt(solicit, D6OptStatusCode, []byte{0}),
		opt(solicit, D6OptStatusCode, []byte{0xFF, 0xFF}),
		// accel-ppp-ng 371e7f0 (T8473): two Rapid-Commit options.
		opt(opt(solicit, 14, nil), 14, nil),
		// accel-ppp-ng 10a3e72 (T8474): AFTR-Name with a compression pointer and an overrun label.
		opt(solicit, 64, []byte{0xC0, 0x0C, 0, 0}),
		opt(solicit, 64, []byte{0x3F, 'a', 0}),
		// accel-ppp-ng 8acbd5d (T9325): the largest DUID, to press the reply buffer.
		opt(opt([]byte{DHCPv6Solicit, 4, 5, 6}, D6OptClientID, append([]byte{0xFF, 0}, make([]byte, 128)...)),
			D6OptIAPD, make([]byte, 12)),
		// IA_PD sub-option overrun and a truncated option header.
		opt(solicit, D6OptIAPD, []byte{0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 26, 0xFF, 0xFF}),
		append(append([]byte(nil), solicit...), 0, 1),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := parseDHCPv6(data)
		if err != nil {
			return
		}
		svc, _ := newTestIPv6Service()
		pool := func() (netip.Prefix, bool) { return netip.MustParsePrefix("2001:db8:1::/56"), true }
		resp, err := svc.handleDHCPv6(msg, testServerID(), pool)
		if err != nil || len(resp) == 0 {
			return
		}
		switch msg.Type {
		case DHCPv6Solicit, DHCPv6Request, DHCPv6Renew, DHCPv6Release:
		default:
			t.Fatalf("message type %d drew a reply", msg.Type)
		}
		if len(resp) < 4 || !bytes.Equal(resp[1:4], msg.TransactionID[:]) {
			t.Fatalf("reply %x does not carry transaction-id %x", resp, msg.TransactionID)
		}
		if _, err := parseDHCPv6(resp); err != nil {
			t.Fatalf("reply %x does not parse: %v", resp, err)
		}
	})
}
