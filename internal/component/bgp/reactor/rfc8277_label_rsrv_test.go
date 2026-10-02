// Design: docs/architecture/wire/qualifiers.md — the label field, Rsrv on relay
// Overview: rfc8277_label_rsrv.go — clearLabelRsrv

package reactor

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
)

// labeledReachUpdate builds an UPDATE body whose only route-bearing attribute is
// an MP_REACH_NLRI of AFI 1 and the given SAFI, next hop and NLRI, all in hex.
//
//	0000          Withdrawn Routes Length
//	LLLL          Total Path Attribute Length
//	40 01 01 00   ORIGIN IGP
//	40 02 00      AS_PATH, empty
//	80 0e LL      MP_REACH_NLRI: AFI 0001, SAFI, next hop length, next hop,
//	              reserved 00, NLRI
func labeledReachUpdate(t *testing.T, safi, nextHop, nlri string) []byte {
	t.Helper()
	nextHopOctets := len(nextHop) / 2
	mp := "0001" + safi + hex.EncodeToString([]byte{byte(nextHopOctets)}) + nextHop + "00" + nlri
	mpOctets := len(mp) / 2
	require.Less(t, mpOctets, 256, "the fixture uses a one-octet attribute length")
	attrs := "40010100" + "400200" + "800e" + hex.EncodeToString([]byte{byte(mpOctets)}) + mp
	attrOctets := len(attrs) / 2
	body, err := hex.DecodeString("0000" + hex.EncodeToString([]byte{byte(attrOctets >> 8), byte(attrOctets)}) + attrs)
	require.NoError(t, err, "fixture hex")
	return body
}

// payloadNLRI returns the hex of the last nlriOctets octets of the payload,
// which is where labeledReachUpdate puts the NLRI.
func payloadNLRI(payload []byte, nlriOctets int) string {
	return hex.EncodeToString(payload[len(payload)-nlriOctets:])
}

// noAddPath and allAddPath stand for the receive context of a session that
// negotiated ADD-PATH for no family, and for every family.
func noAddPath(family.Family) bool  { return false }
func allAddPath(family.Family) bool { return true }

const (
	ipv4NextHop = "0a000001"
	vpnNextHop  = "0000000000000000" + "0a000001"
	safiLabeled = "04"
	safiVPN     = "80"
)

// TestRFC8277RsrvClearedOnRelay hands the ingest step label fields whose Rsrv
// bits a peer set, and reads the NLRI octets every consumer then sees: the RIB
// keeps them, the forward rails copy them, and every rebuild starts from them.
//
// VALIDATES: RFC 8277 Section 2.2: "Rsrv: This 3-bit field SHOULD be set to zero
// on transmission and MUST be ignored on reception."
// PREVENTS: Ze relaying a peer's non-zero Rsrv bits onward.
//
// RFC requirement: RFC8277-2.2-3 negative -- received label fields with Rsrv
// 111, 101, 001 and 110 leave clearLabelRsrv with Rsrv 000 and every other
// octet unchanged: one label ("30 00064f 0a0102" -> "30 000641 0a0102"), a
// three-label stack followed by a second NLRI, an ADD-PATH NLRI whose path
// identifier is not read as a label, and a SAFI 128 NLRI whose Route
// Distinguisher is not read as a label. The received buffer is left as it was.
func TestRFC8277RsrvClearedOnRelay(t *testing.T) {
	cases := []struct {
		name, safi, nextHop, received, relayed string
		addPathFor                             func(family.Family) bool
	}{
		{"one-label", safiLabeled, ipv4NextHop,
			"30" + "00064f" + "0a0102",
			"30" + "000641" + "0a0102", noAddPath},
		{"stack-then-second-nlri", safiLabeled, ipv4NextHop,
			"60" + "00064e" + "000c8a" + "0012c3" + "0a0102" + "30" + "00190d" + "0a0103",
			"60" + "000640" + "000c80" + "0012c1" + "0a0102" + "30" + "001901" + "0a0103", noAddPath},
		{"add-path", safiLabeled, ipv4NextHop,
			"0e0e0e0e" + "30" + "00064f" + "0a0102",
			"0e0e0e0e" + "30" + "000641" + "0a0102", allAddPath},
		{"vpn", safiVPN, vpnNextHop,
			"70" + "00064f" + "0000fde90000000e" + "0a0102",
			"70" + "000641" + "0000fde90000000e" + "0a0102", noAddPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := labeledReachUpdate(t, tc.safi, tc.nextHop, tc.received)
			receivedHex := hex.EncodeToString(received)
			nlriOctets := len(tc.received) / 2

			out := clearLabelRsrv(wireu.NewWireUpdate(received, 0), tc.addPathFor)

			assert.Equal(t, tc.relayed, payloadNLRI(out.Payload(), nlriOctets), "Rsrv is zero in what Ze relays")
			assert.Equal(t, len(received), len(out.Payload()), "no octet is added or removed")
			assert.True(t, strings.HasPrefix(hex.EncodeToString(out.Payload()), receivedHex[:len(receivedHex)-2*nlriOctets]),
				"every octet before the NLRI is unchanged")
			assert.Equal(t, receivedHex, hex.EncodeToString(received), "the received buffer is not written")
		})
	}
}

// TestRFC8277RsrvClearedOnTheReceivePath drives the same UPDATE through the
// session's receive entry, enforceRFC7606, which ends in publishBase: the
// returned payload is what the RIB stores and both forward rails send.
//
// RFC requirement: RFC8277-2.2-3 negative -- a SAFI 4 NLRI received with Rsrv
// 111 ("30 00064f 0a0102") leaves enforceRFC7606 with no error and no RFC 7606
// action, carrying "30 000641 0a0102".
func TestRFC8277RsrvClearedOnTheReceivePath(t *testing.T) {
	body := labeledReachUpdate(t, safiLabeled, ipv4NextHop, "30"+"00064f"+"0a0102")
	s := rfc7311EBGPSession()

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err, "a non-zero Rsrv is ignored, never an error")
	require.Equal(t, message.RFC7606ActionNone, action, "a non-zero Rsrv draws no RFC 7606 action")
	assert.Equal(t, "30"+"000641"+"0a0102", payloadNLRI(wu.Payload(), 7), "the published NLRI carries Rsrv 000")
}

// TestRFC8277ZeroRsrvRelayedZeroCopy is the common case: a peer that already
// sends Rsrv 000 costs nothing, and a family that carries no label field is
// never read as one.
//
// RFC requirement: RFC8277-2.2-3 positive -- a SAFI 4 three-label stack with
// Rsrv 000 on every entry, and an IPv4 unicast MP_REACH NLRI whose prefix octets
// have the Rsrv bit positions set ("18 0a0e0f"), each leave clearLabelRsrv as
// the same WireUpdate with the same octets.
func TestRFC8277ZeroRsrvRelayedZeroCopy(t *testing.T) {
	cases := []struct{ name, safi, nlri string }{
		{"labeled-zero-rsrv", safiLabeled, "60" + "000640" + "000c80" + "0012c1" + "0a0102"},
		{"unicast-not-a-label", "01", "18" + "0a0e0f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := labeledReachUpdate(t, tc.safi, ipv4NextHop, tc.nlri)
			receivedHex := hex.EncodeToString(received)
			wu := wireu.NewWireUpdate(received, 0)

			out := clearLabelRsrv(wu, noAddPath)

			assert.Same(t, wu, out, "nothing to clear, so the received update is relayed as it came")
			assert.Equal(t, receivedHex, hex.EncodeToString(out.Payload()), "the octets are unchanged")
		})
	}
}
