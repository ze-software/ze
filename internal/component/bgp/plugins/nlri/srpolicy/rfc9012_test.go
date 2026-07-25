// VALIDATES: the Tunnel Encapsulation attribute the SR Policy originator builds is
// readable by ze's own attribute parser -- the TLV framing, the 6-octet Preference
// sub-TLV of RFC 9830 Section 2.4.1, and the 2-octet-length name sub-TLVs whose value is
// a reserved octet followed by the whole name.
// PREVENTS: an encoder and a decoder drifting into two different wire formats (ze once
// wrote a 6-octet Preference and read it back at offset 4 of an 8-octet value), and a
// name sub-TLV that loses its last character or leaves its reserved octet unwritten.

package srpolicy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// subTLVValue returns the value of the first sub-TLV of the given type.
func subTLVValue(t *testing.T, tlv attribute.TunnelTLV, styp uint8) []byte {
	t.Helper()
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	for i := range stlvs {
		if stlvs[i].Type == styp {
			return stlvs[i].Value
		}
	}
	t.Fatalf("sub-TLV type %d not found", styp)
	return nil
}

func TestSRPolicyTunnelEncapIsSelfDescribing(t *testing.T) {
	t.Parallel()

	cfg := "distinguisher 7 color 100 endpoint 10.0.0.1 preference 4242 " +
		"binding-sid mpls 24000 priority 9 " +
		"segment-list weight 3 segment type-a mpls 16001 " +
		"policy-name alpha candidate-path-name primary"
	pr, err := parseConfigRoute(cr(strings.Fields(cfg), "192.0.2.1", false))
	require.NoError(t, err)
	require.Len(t, pr.Attrs, 1)

	te, err := attribute.ParseTunnelEncap(pr.Attrs[0].Value)
	require.NoError(t, err, "the attribute ze builds must parse with ze's own parser")
	require.Len(t, te.TLVs, 1)
	assert.Equal(t, tunnelTypeSRPolicyCP, te.TLVs[0].TunnelType)

	pref, ok := te.TLVs[0].Preference()
	require.True(t, ok, "the Preference sub-TLV ze writes must be readable")
	assert.Equal(t, uint32(4242), pref)

	// RFC 9830 Sections 2.4.7 and 2.4.8: value = RESERVED(1) + name, 2-octet length.
	assert.Equal(t, append([]byte{0x00}, []byte("alpha")...),
		subTLVValue(t, te.TLVs[0], subTLVPolicyName))
	assert.Equal(t, append([]byte{0x00}, []byte("primary")...),
		subTLVValue(t, te.TLVs[0], subTLVCandidatePathNam))

	// Every sub-TLV ze wrote is walkable: the TLV ends exactly where its last sub-TLV does.
	stlvs, err := te.TLVs[0].SubTLVs()
	require.NoError(t, err)
	assert.Len(t, stlvs, 6, "preference, binding-sid, priority, segment-list, policy name, candidate-path name")
}
