package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// These tests drive the canonical form the mup plugin's init() registers
// (mup_test.go links the plugin into this binary), through the receive path's RFC
// 7606 pass, whose output is what the Adj-RIB-In holds and every relay forwards.

// TestMUPT1ST02RelayedWithSourceLength proves a received draft-mpmz-bess-mup-safi-02
// Type 1 ST route, which ends at its Endpoint Address, leaves the receive path with
// the Source Address Length octet, 0, and the Length one larger.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.3.1, "The Source Address Length has
// a fixed length of 1 octet.", for an announcement and a withdrawal; a current-form
// sibling is kept byte for byte; an UPDATE with no -02 route is returned as received,
// not copied; an MP_REACH_NLRI the octets push past 255 gains the Extended Length bit.
// PREVENTS: the raw relay forwarding a -02 route as received, where ExaBGP f9bfb853b
// re-sends it with the octet (with_source_length, t1st.py).
func TestMUPT1ST02RelayedWithSourceLength(t *testing.T) {
	old := mupWireNLRI(3, t1stBody(1)...)
	current := mupWireNLRI(3, t1stBody(1, 0x00)...)
	sibling := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0, 0x02, 0xaa, 0xbb)...)

	t.Run("announcement", func(t *testing.T) {
		s := nlriTypeTestSession()
		nlri := append(append([]byte{}, old...), sibling...)
		wu, action, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, nlri), nil))
		assert.Equal(t, message.RFC7606ActionNone, action)

		kept, found := mpReachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, append(append([]byte{}, current...), sibling...), kept,
			"the -02 route leaves with Source Address Length 0, the sibling as received")
		requireDecodable(t, mupFam, kept)
	})

	t.Run("withdrawal", func(t *testing.T) {
		s := nlriTypeTestSession()
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, old), nil))

		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, current, withdrawn, "the -02 withdrawal leaves in the current form")
	})

	t.Run("current form not copied", func(t *testing.T) {
		s := nlriTypeTestSession()
		body := makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, current...), sibling...)), nil)
		wu, _, _ := applyRFC7606Body(t, s, body)
		assert.Equal(t, body, wu.Payload())
		assert.Same(t, &body[0], &wu.Payload()[0], "an UPDATE with no -02 route is relayed from the received octets")
	})

	t.Run("growth past 255 octets", func(t *testing.T) {
		s := nlriTypeTestSession()
		// MP_REACH value: AFI, SAFI, next hop (1+4), reserved = 9 octets, then
		// 9 routes of 27 octets: 252 octets, and 261 once each gains its octet.
		var nlri, want []byte
		for i := range 9 {
			nlri = append(nlri, mupWireNLRI(3, t1stBody(byte(i+1))...)...)
			want = append(want, mupWireNLRI(3, t1stBody(byte(i+1), 0x00)...)...)
		}
		require.Equal(t, 243, len(nlri))
		wu, action, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, nlri), nil))
		assert.Equal(t, message.RFC7606ActionNone, action)

		kept, found := mpReachNLRIOf(t, wu.Payload())
		require.True(t, found, "the rewritten MP_REACH_NLRI frames")
		assert.True(t, bytes.Equal(want, kept), "every -02 route gains its octet")
		requireDecodable(t, mupFam, kept)
	})
}
