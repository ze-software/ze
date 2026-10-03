// Design: docs/architecture/wire/qualifiers.md -- ingest rewrites on the receive path
// Related: session_validation.go -- publishBase, the receive step every UPDATE leaves through

package reactor

// RFC naming: untagged -- a red defect probe: nothing on the receive path discards a duplicate Prefix-SID TLV yet.

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// srv6L3ServiceTLV is one SRv6 L3 Service TLV (Prefix-SID TLV type 5): Reserved,
// then an SRv6 SID Information Sub-TLV (type 1, length 21) holding the SID,
// Service SID Flags 00, Endpoint Behavior 0x0013 (End.DT4) and Reserved.
func srv6L3ServiceTLV(sid string) string {
	return "05" + "0019" + "00" + "01" + "0015" + "00" + sid + "00" + "0013" + "00"
}

// VALIDATES: RFC 8669 Section 6, "if a recognized TLV appears more than once in
// a BGP Prefix-SID attribute while the specification only allows for a single
// occurrence, then all the occurrences of the TLV other than the first one SHALL
// be discarded". RFC 9252 Section 7 allows one SRv6 L3 Service TLV. A labeled
// unicast UPDATE whose Prefix-SID carries two of them goes through the real
// receive entry (enforceRFC7606, ending in publishBase): the update is still
// processed (no error, no RFC 7606 action), and the published Prefix-SID holds
// the first TLV only.
// PREVENTS: the duplicate staying in the attribute bytes Ze relays and renders
// (appendPrefixSIDJSON emits a second "l3-service" key, which a JSON reader
// that keeps the last key takes as THE service SID).
// Untagged and red on purpose: nothing on the receive path discards a
// duplicate Prefix-SID TLV today.
func TestRFC8669DuplicateServiceTLVDiscardedOnReceive(t *testing.T) {
	firstSID := "20010db8000000010000000000000000"
	secondSID := "20010db8000000020000000000000000"
	prefixSID := srv6L3ServiceTLV(firstSID) + srv6L3ServiceTLV(secondSID)
	attribute40 := "c028" + hex.EncodeToString([]byte{byte(len(prefixSID) / 2)}) + prefixSID

	body := labeledReachUpdate(t, safiLabeled, ipv4NextHop, "30"+"000641"+"0a0102")
	attrOctets := int(body[2])<<8 | int(body[3])
	attrOctets += len(attribute40) / 2
	withPrefixSID := "0000" + hex.EncodeToString([]byte{byte(attrOctets >> 8), byte(attrOctets)}) +
		hex.EncodeToString(body[4:]) + attribute40
	received, err := hex.DecodeString(withPrefixSID)
	require.NoError(t, err, "fixture hex")

	// An internal session: RFC 8669 Section 4 has an EBGP receiver discard the
	// whole attribute, which would hide the TLV-level rule this test drives.
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	wu, action, err := NewSession(settings).enforceRFC7606(wireu.NewWireUpdate(received, 0))

	require.NoError(t, err, "a duplicate TLV is discarded, never an error")
	require.Equal(t, message.RFC7606ActionNone, action, "the UPDATE continues to be processed")
	published := hex.EncodeToString(wu.Payload())
	require.True(t, strings.Contains(published, "c028"+"1c"+srv6L3ServiceTLV(firstSID)),
		"the published Prefix-SID is the first L3 Service TLV alone (length 0x1c), got %s", published)
	require.False(t, strings.Contains(published, secondSID),
		"the second L3 Service TLV is discarded, got %s", published)
}
