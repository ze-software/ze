// RFC: rfc/short/rfc9552.md — unknown Link-State NLRI types are opaque (§5.2)
//
// RFC 9552 Section 5.2 goes further than RFC 7752: an implementation MUST treat
// an unknown Link-State NLRI type as an opaque object and MUST preserve and
// propagate it, overriding the RFC 7606 Section 5.4 instinct to discard what
// cannot be parsed. Ze meets that structurally. The BGP-LS NLRI framing used on
// every propagation path is bgpLSNLRISize (chunk_mp_nlri.go:348), which reads
// the 2-octet Total NLRI Length and never looks at the NLRI Type, so a type ze
// has no decoder for is carved out and carried like any other.

package message

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// bgpLSWireNLRI frames a BGP-LS NLRI: type, total length, then a body of
// bodyLen octets filled with fill.
func bgpLSWireNLRI(nlriType uint16, bodyLen int, fill byte) []byte {
	out := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint16(out[0:], nlriType)
	binary.BigEndian.PutUint16(out[2:], uint16(bodyLen)) //nolint:gosec // test fixture
	for i := 4; i < len(out); i++ {
		out[i] = fill
	}
	return out
}

// TestRFC9552UnknownBGPLSNLRITypeIsOpaque proves an NLRI type ze cannot name is
// framed, chunked and handed on with its bytes intact. Type 65000 is in the
// RFC 9552 Section 5.2 Private Use range, so an IANA allocation cannot turn this
// test green for the wrong reason.
//
// VALIDATES: ChunkMPNLRI over AFI 16388 / SAFI 71 preserves an unknown NLRI
// type both when everything fits in one message and when the chunker must split
// on element boundaries.
// PREVENTS: a type switch being introduced into the BGP-LS framing path and
// dropping NLRIs a downstream Consumer understands.
func TestRFC9552UnknownBGPLSNLRITypeIsOpaque(t *testing.T) {
	// RFC requirement: RFC9552-5.2-8 positive -- an unknown Link-State NLRI type is framed by its Total NLRI Length alone, kept whole, and propagated byte-identically (§5.2)
	node := bgpLSWireNLRI(1, 30, 0xA1)        // known: Node NLRI
	unknown := bgpLSWireNLRI(65000, 40, 0xB2) // Private Use, no decoder in ze
	link := bgpLSWireNLRI(2, 30, 0xC3)        // known: Link NLRI
	data := append(append(append([]byte{}, node...), unknown...), link...)

	chunks, err := ChunkMPNLRI(data, family.AFIBGPLS, 71, false, len(data), nil)
	require.NoError(t, err, "an unknown NLRI type is not a framing error")
	require.Len(t, chunks, 1)
	assert.Equal(t, data, chunks[0], "the whole NLRI field, unknown type included, rides through")

	// Force a split: the chunker must respect the unknown NLRI's boundary and
	// never cut it in half.
	chunks, err = ChunkMPNLRI(data, family.AFIBGPLS, 71, false, len(node)+len(unknown), nil)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	assert.Equal(t, append(append([]byte{}, node...), unknown...), chunks[0],
		"the unknown NLRI is kept whole alongside the known one")
	assert.Equal(t, link, chunks[1])

	var rejoined []byte
	for _, c := range chunks {
		rejoined = append(rejoined, c...)
	}
	assert.Equal(t, data, rejoined, "nothing is dropped and nothing is rewritten")

	// SplitMPNLRI, used when forwarding to a peer with a smaller message limit,
	// splits at the same boundary.
	fitting, remaining, err := SplitMPNLRI(data, family.AFIBGPLS, 71, false, len(node)+len(unknown))
	require.NoError(t, err)
	assert.Equal(t, append(append([]byte{}, node...), unknown...), fitting)
	assert.Equal(t, link, remaining)
}

// TestRFC9552MalformedBGPLSNLRIRefused is the counter-case: opacity is about the
// TYPE, not about the framing. An NLRI whose Total NLRI Length runs past the end
// of the field cannot be carved out, so it is refused rather than carried as an
// opaque blob of whatever bytes remain.
//
// VALIDATES: the element iterator reports a truncated BGP-LS NLRI
// (iter.Elements.Next, internal/iter/iter.go:50) and ChunkMPNLRI surfaces it.
// PREVENTS: "unknown types are opaque" being widened into "any bytes are
// opaque", which would propagate a corrupted NLRI field.
func TestRFC9552MalformedBGPLSNLRIRefused(t *testing.T) {
	// RFC requirement: RFC9552-5.2-8 negative -- an NLRI whose Total NLRI Length overruns the field is refused, so opaque handling of unknown types does not extend to broken framing (§5.2)
	truncated := bgpLSWireNLRI(65000, 40, 0xB2)[:20] // header claims 40 body octets, 16 present

	_, err := ChunkMPNLRI(truncated, family.AFIBGPLS, 71, false, 4096, nil)
	require.Error(t, err, "a truncated unknown NLRI is not chunked")

	_, _, err = SplitMPNLRI(truncated, family.AFIBGPLS, 71, false, 8)
	require.Error(t, err, "a truncated unknown NLRI is not split")

	// A header too short to hold the Total NLRI Length is malformed too.
	_, err = ChunkMPNLRI([]byte{0xFD, 0xE8, 0x00}, family.AFIBGPLS, 71, false, 4096, nil)
	assert.ErrorIs(t, err, ErrNLRIMalformed)
}
