package mvpn

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMVPNTypes verifies MVPN route types.
func TestMVPNTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, MVPNRouteType(1), MVPNIntraASIPMSIAD)
	assert.Equal(t, MVPNRouteType(2), MVPNInterASIPMSIAD)
	assert.Equal(t, MVPNRouteType(3), MVPNSPMSIAD)
	assert.Equal(t, MVPNRouteType(4), MVPNLeafAD)
	assert.Equal(t, MVPNRouteType(5), MVPNSourceActive)
	assert.Equal(t, MVPNRouteType(6), MVPNSharedTreeJoin)
	assert.Equal(t, MVPNRouteType(7), MVPNSourceTreeJoin)
}

// TestMVPNBasic verifies basic MVPN NLRI creation.
func TestMVPNBasic(t *testing.T) {
	t.Parallel()
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, []byte{1, 2, 3, 4})

	assert.Equal(t, MVPNIntraASIPMSIAD, mvpn.RouteType())
	assert.NotNil(t, mvpn.Bytes())
}

// TestMVPNFamily verifies MVPN address family.
func TestMVPNFamily(t *testing.T) {
	t.Parallel()
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, nil)

	assert.Equal(t, AFIIPv4, mvpn.Family().AFI)
	assert.Equal(t, SAFIMVPN, mvpn.Family().SAFI)
}

// TestMVPNWithRD verifies MVPN with Route Distinguisher.
func TestMVPNWithRD(t *testing.T) {
	t.Parallel()
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	mvpn := NewMVPNWithRD(AFIIPv6, MVPNIntraASIPMSIAD, rd, []byte{1, 2, 3, 4})

	assert.Equal(t, AFIIPv6, mvpn.Family().AFI)
	assert.Equal(t, rd, mvpn.RD())
}

// TestMVPNRoundTrip verifies encode/decode cycle.
func TestMVPNRoundTrip(t *testing.T) {
	t.Parallel()
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	original := NewMVPNWithRD(AFIIPv4, MVPNIntraASIPMSIAD, rd, []byte{10, 0, 0, 1})
	data := original.Bytes()

	parsed, remaining, err := ParseMVPN(AFIIPv4, data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.RouteType(), parsed.RouteType())
	assert.Equal(t, original.RD(), parsed.RD())
}

// TestMVPNParseErrors verifies error handling.
func TestMVPNParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated header", []byte{0x01}},
		{"truncated body", []byte{0x01, 0x10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseMVPN(AFIIPv4, tt.data)
			assert.Error(t, err)
		})
	}
}

// TestMVPNStringCommandStyle verifies command-style string representation.
//
// VALIDATES: MVPN String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestMVPNStringCommandStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mvpn     *MVPN
		expected string
	}{
		{
			name:     "mvpn without rd",
			mvpn:     NewMVPN(MVPNIntraASIPMSIAD, []byte{1, 2, 3, 4}),
			expected: "intra-as-i-pmsi-ad",
		},
		{
			name: "mvpn with rd",
			mvpn: func() *MVPN {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMVPNWithRD(AFIIPv4, MVPNSourceTreeJoin, rd, []byte{10, 0, 0, 1})
			}(),
			expected: "source-tree-join rd 0:65001:100",
		},
		{
			name: "mvpn s-pmsi-ad with rd",
			mvpn: func() *MVPN {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewMVPNWithRD(AFIIPv6, MVPNSPMSIAD, rd, nil)
			}(),
			expected: "s-pmsi-ad rd 1:10.0.0.1:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.mvpn.String())
		})
	}
}

// TestMVPNWriteToMatchesBytes verifies MVPN.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for MVPN NLRI.
// PREVENTS: Route type encoding errors, RD data loss.
func TestMVPNWriteToMatchesBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mvpn *MVPN
	}{
		{
			name: "basic mvpn",
			mvpn: NewMVPN(MVPNIntraASIPMSIAD, []byte{1, 2, 3, 4}),
		},
		{
			name: "mvpn with rd",
			mvpn: func() *MVPN {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMVPNWithRD(AFIIPv4, MVPNIntraASIPMSIAD, rd, []byte{10, 0, 0, 1})
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.mvpn.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.mvpn.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}
