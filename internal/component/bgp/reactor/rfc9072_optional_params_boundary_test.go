package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// sizedCapability is a well-formed capability TLV of exactly octets octets
// (code 200, a Private Use code no ze decoder claims), so a test can size the
// capability total to any value from 2 upward.
type sizedCapability struct{ octets int }

func (c sizedCapability) Code() capability.Code { return 200 }
func (c sizedCapability) Len() int              { return c.octets }
func (c sizedCapability) WriteTo(buf []byte, off int) int {
	buf[off] = 200
	buf[off+1] = byte(c.octets - 2) //nolint:gosec // G115: octets is at most 257 by construction
	for i := 2; i < c.octets; i++ {
		buf[off+i] = 0xAB
	}
	return c.octets
}

// capabilitiesTotaling returns sized capabilities whose Len sums to octets
// (0, or at least 2), each TLV at most 257 octets.
func capabilitiesTotaling(octets int) []capability.Capability {
	var caps []capability.Capability
	for octets > 0 {
		tlv := min(octets, 257)
		if octets-tlv == 1 {
			tlv--
		}
		caps = append(caps, sizedCapability{octets: tlv})
		octets -= tlv
	}
	return caps
}

// TestRFC9072OptionalParamsBoundary proves where ze's OPEN switches to the RFC
// 9072 encoding.
//
// VALIDATES: the OPEN buildOpen produces, written by Open.WriteTo, for
// capability totals 0, 2, 253, 254, 255, 256 and 300. Up to 253 octets of
// capabilities (255 octets of Optional Parameters, the type-2 header
// included) the OPEN carries the RFC 4271 form: Opt Parm Len 2 + capTotal, no
// 255/255 marker, a one-octet Parameter Length. From 254 the OPEN carries
// Non-Ext OP Len 255, Non-Ext OP Type 255, Extended Opt. Parm. Length 3 +
// capTotal, and one type-2 parameter whose two-octet Parameter Length is
// capTotal. A conforming decode returns every capability.
// PREVENTS: D1, the off-by-two that chose the one-octet Parameter Length while
// capTotal <= 255, so 254 and 255 wrapped a one-octet parameter in the
// extended envelope and a conformant receiver misframed it.
//
// Method: the parameters are walked with the framing the envelope announces,
// read from the octets, never from the encoder's flag. capTotal 1 is not
// constructible: a capability TLV is at least two octets.
//
// RFC requirement: RFC9072-2-1 positive -- capabilities of 254, 255, 256 and 300 octets give
// Optional Parameters above 255 octets, and the OPEN carries the 255/255 envelope with
// Extended Opt. Parm. Length 3 + capTotal and a two-octet Parameter Length of capTotal.
// RFC requirement: RFC9072-2-1 negative -- capabilities of 253 octets or fewer give Optional
// Parameters of 255 octets or fewer, and the OPEN carries no 255/255 envelope; the base form
// there is the RFC 9072 Section 2 SHOULD: "In the event that the length of the Optional
// Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP
// specification [RFC4271] SHOULD be used without alteration."
// RFC requirement: RFC9072-2-7 positive -- capabilities of 0, 2 and 253 octets give the RFC
// 4271 form: Opt Parm Len 2 + capTotal and a one-octet Parameter Length.
func TestRFC9072OptionalParamsBoundary(t *testing.T) {
	for _, capTotal := range []int{0, 2, 253, 254, 255, 256, 300} {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
		settings.DisableASN4 = true
		caps := capabilitiesTotaling(capTotal)
		s := NewSession(settings)

		open, err := s.buildOpen(settings, caps)
		require.NoError(t, err, "capTotal %d", capTotal)
		body := message.PackTo(open, nil)[message.HeaderLen:]

		switch {
		case capTotal == 0:
			require.Equal(t, byte(0), body[9], "no capabilities, no Optional Parameters")
			require.Len(t, body, 10)
		case capTotal <= 253:
			require.Equal(t, byte(2+capTotal), body[9], "capTotal %d: Opt Parm Len", capTotal) //nolint:gosec // G115: at most 255
			require.Equal(t, byte(2), body[10], "capTotal %d: Parm. Type", capTotal)
			require.Equal(t, byte(capTotal), body[11], "capTotal %d: one-octet Parameter Length", capTotal) //nolint:gosec // G115: at most 253
			require.Len(t, body, 12+capTotal)
		default:
			require.Equal(t, byte(255), body[9], "capTotal %d: Non-Ext OP Len", capTotal)
			require.Equal(t, byte(255), body[10], "capTotal %d: Non-Ext OP Type", capTotal)
			require.Equal(t, 3+capTotal, int(binary.BigEndian.Uint16(body[11:13])), "capTotal %d: Extended Opt. Parm. Length", capTotal)
			require.Equal(t, byte(2), body[13], "capTotal %d: Parm. Type", capTotal)
			require.Equal(t, capTotal, int(binary.BigEndian.Uint16(body[14:16])), "capTotal %d: two-octet Parameter Length", capTotal)
			require.Len(t, body, 16+capTotal)
		}

		decoded, err := message.UnpackOpen(body)
		require.NoError(t, err, "capTotal %d", capTotal)
		got, err := capability.ParseFromOptionalParams(decoded.OptionalParams, decoded.ExtendedParams)
		require.NoError(t, err, "capTotal %d", capTotal)
		require.Len(t, got, len(caps), "capTotal %d: every capability decodes", capTotal)
		for i := range caps {
			require.Equal(t, caps[i].Len(), got[i].Len(), "capTotal %d: capability %d", capTotal, i)
		}
	}
}
