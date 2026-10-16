// Design: docs/architecture/wire/nlri-bgpls.md — node descriptor sub-TLV decoding tests
// Related: types_descriptor.go — the descriptor parsers under test

package bgpls

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDescriptorBGPRouterID tests descriptor sub-TLV 516 round-trip.
//
// VALIDATES: BGP Router-ID in NodeDescriptor encodes and parses.
// PREVENTS: Missing TLV 516 in descriptor encoding/parsing.
func TestDescriptorBGPRouterID(t *testing.T) {
	nd := NodeDescriptor{
		ASN:         65001,
		BGPRouterID: 0x0A000001, // 10.0.0.1 as uint32
	}

	buf := make([]byte, nd.Len())
	nd.WriteTo(buf, 0)

	var parsed NodeDescriptor
	err := parseNodeDescriptorTLVs(buf, &parsed)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), parsed.ASN)
	assert.Equal(t, uint32(0x0A000001), parsed.BGPRouterID)
}

// TestDescriptorConfedMember tests descriptor sub-TLV 517 round-trip.
//
// VALIDATES: Confederation Member in NodeDescriptor encodes and parses.
// PREVENTS: Missing TLV 517 in descriptor encoding/parsing.
func TestDescriptorConfedMember(t *testing.T) {
	nd := NodeDescriptor{
		ASN:          65001,
		ConfedMember: 65100,
	}

	buf := make([]byte, nd.Len())
	nd.WriteTo(buf, 0)

	var parsed NodeDescriptor
	err := parseNodeDescriptorTLVs(buf, &parsed)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), parsed.ASN)
	assert.Equal(t, uint32(65100), parsed.ConfedMember)
}

// TestDescriptorSRv6SID tests the SRv6 SID Information TLV 518 round-trip.
//
// VALIDATES: an SRv6 SID NLRI encodes its SID (16-byte IPv6 address) as TLV 518
// after the Local Node Descriptors, and parsing returns it in the SRv6 SID
// Descriptor (RFC 9514 Section 6).
// PREVENTS: TLV 518 written inside the Node Descriptor or lost on parse.
func TestDescriptorSRv6SID(t *testing.T) {
	sid := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	nlri := NewBGPLSSRv6SID(ProtoISISL2, 0, NodeDescriptor{ASN: 65001}, SRv6SIDDescriptor{SRv6SID: sid})

	parsed, err := parseBGPLS(nlri.Bytes())
	require.NoError(t, err)
	srv6, ok := parsed.(*BGPLSSRv6SID)
	require.True(t, ok)
	assert.Equal(t, uint32(65001), srv6.LocalNode.ASN)
	assert.Equal(t, sid, srv6.SRv6SID.SRv6SID)
	assert.Equal(t, 8, srv6.LocalNode.Len(), "TLV 518 must not be read into the Node Descriptor")
}
