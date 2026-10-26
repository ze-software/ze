// Design: docs/architecture/testing/ci-format.md — generated UPDATE attributes.
// Related: message.go — unicast and labeled route builders.

package peer

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// TestSendRouteLocalPrefFollowsSession checks complete wire attribute blocks for
// unicast and labeled routes, preserving LOCAL_PREF=100 only on iBGP.
// It also checks message lengths and NLRI so omitting the attribute cannot leave
// stale framing or consume route bytes. Raw hex injection is not involved.
func TestSendRouteLocalPrefFollowsSession(t *testing.T) {
	for _, session := range []struct {
		name      string
		senderAS  uint32
		asPath    string
		localPref string
	}{
		{
			name:      "ibgp",
			asPath:    "40020602010000fde9",
			localPref: "40050400000064",
		},
		{
			name:     "ebgp",
			senderAS: 65002,
			asPath:   "40020a02020000fdea0000fde9",
		},
	} {
		for _, family := range []struct {
			name    string
			labels  []uint32
			nextHop string
			mpReach string
			nlri    string
		}{
			{
				name:    "unicast",
				nextHop: "400304c0000201",
				nlri:    "180a0001",
			},
			{
				name:    "labeled",
				labels:  []uint32{100},
				mpReach: "900e001000010404c000020100300006410a0001",
			},
		} {
			t.Run(session.name+"/"+family.name, func(t *testing.T) {
				route := RouteToSend{
					Prefix: "10.0.1.0/24", NextHop: "192.0.2.1", OriginAS: 65001,
					SenderAS: session.senderAS, Labels: family.labels,
				}
				// RFC 4271 Sections 4.3 and 5.1.5; RFC 8277 Section 2.2.
				msg, err := buildRouteMsg(route)
				if err != nil {
					t.Fatal(err)
				}
				if len(msg) < HeaderLen+4 {
					t.Fatalf("truncated UPDATE: %x", msg)
				}
				if got := int(binary.BigEndian.Uint16(msg[16:18])); got != len(msg) {
					t.Fatalf("message length = %d, actual = %d", got, len(msg))
				}
				if msg[18] != MsgUPDATE {
					t.Fatalf("message type = %d, want UPDATE", msg[18])
				}
				if got := binary.BigEndian.Uint16(msg[HeaderLen:]); got != 0 {
					t.Fatalf("withdrawn length = %d, want 0", got)
				}
				wantAttrs, err := hex.DecodeString("40010100" + session.asPath + family.nextHop + session.localPref + family.mpReach)
				if err != nil {
					t.Fatal(err)
				}
				attrLen := int(binary.BigEndian.Uint16(msg[HeaderLen+2:]))
				if attrLen != len(wantAttrs) {
					t.Fatalf("attribute length = %d, want %d; UPDATE = %x", attrLen, len(wantAttrs), msg)
				}
				attrEnd := HeaderLen + 4 + attrLen
				if attrEnd > len(msg) {
					t.Fatalf("attributes exceed UPDATE: %x", msg)
				}
				if got := msg[HeaderLen+4 : attrEnd]; !bytes.Equal(got, wantAttrs) {
					t.Errorf("attributes = %x, want %x", got, wantAttrs)
				}
				if got := hex.EncodeToString(msg[attrEnd:]); got != family.nlri {
					t.Errorf("legacy NLRI = %s, want %s", got, family.nlri)
				}
			})
		}
	}
}
