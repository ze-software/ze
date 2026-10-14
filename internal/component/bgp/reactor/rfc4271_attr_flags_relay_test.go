// Design: docs/architecture/wire/attributes.md -- the reserved flag bits on send
// Overview: session_egress_normalize.go -- normalizeTransmittedUpdate

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestRFC4271RelayedLowNibbleClearedOnBothRails receives an UPDATE whose ORIGIN,
// AS_PATH and NEXT_HOP are flagged 0x4F and whose unrecognized optional
// transitive attribute 241 is flagged 0xCF, and relays it over the general rail
// to a normal peer and over the route-server rail, through each rail's writer.
//
// VALIDATES: AC-7: the received reserved bits draw no RFC 7606 action; AC-1a: on
// both destinations' wire ORIGIN and NEXT_HOP are 0x40 and 241 is 0xE0 (the
// Partial bit is the RFC 4271 Section 5 rule each rail already applies, not
// asserted here beyond the general rail); the value octets equal the received
// ones on both.
// PREVENTS: a relay copying the received bytes repeating the peer's reserved bits.
// RFC requirement: RFC4271-4.3-4 positive -- reserved flag bits on ORIGIN, AS_PATH, NEXT_HOP and an unrecognized transitive attribute are ignored on receipt: the validator takes no action and the route is relayed on both rails.
// RFC requirement: RFC4271-4.3-3 positive -- the relayed UPDATE leaves the general rail and the route-server rail with the four lower-order bits of every flags octet zero.
func TestRFC4271RelayedLowNibbleClearedOnBothRails(t *testing.T) {
	received := lowNibbleAttrs()[:len(lowNibbleAttrs())-6] // without attribute 242
	result := message.ValidateUpdateRFC7606(received, true, false, true)
	require.Equal(t, message.RFC7606ActionNone, result.Action, result.Description)
	in := egressBody(received, egressNLRI)
	want := egressAttrs(t, received)
	for _, routeServer := range []bool{false, true} {
		name := map[bool]string{false: "general", true: "route-server"}[routeServer]
		t.Run(name, func(t *testing.T) {
			out := egressWrite(t, "relay", rfc8669RelayUpdate(t, routeServer, NextHopUnchanged, in))
			sections, err := wire.ParseUpdateSections(out)
			require.NoError(t, err)
			got := map[byte]egressAttr{}
			for _, a := range egressAttrs(t, sections.Attrs(out)) {
				require.Zero(t, a.flags&0x0F, "attribute %d sent with reserved bits %#x", a.code, a.flags)
				got[a.code] = a
			}
			for _, w := range want {
				a, ok := got[w.code]
				require.True(t, ok, "attribute %d not relayed", w.code)
				if w.code != 2 { // AS_PATH gains Ze's AS towards an eBGP peer
					require.Equal(t, w.value, a.value, "attribute %d value", w.code)
				}
			}
			require.Equal(t, byte(0xC0), got[241].flags&0xD0, "attribute 241 Optional, Transitive, Extended Length")
			if !routeServer {
				require.Equal(t, byte(0xE0), got[241].flags)
			}
			require.Equal(t, byte(0x40), got[1].flags)
			require.Equal(t, byte(0x40), got[3].flags)
			require.Equal(t, egressNLRI, sections.NLRI(out))
		})
	}
}
