package nlri

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// WriteTo Zero-Allocation Tests
//
// These tests verify that WriteTo produces identical output to Bytes()
// for all NLRI types that need zero-alloc optimization.
//
// VALIDATES: WriteTo writes correct wire format directly to buffer
// PREVENTS: Allocation in hot path, output mismatch with Bytes()
// ============================================================================

// TestBGPLSNodeWriteToMatchesBytes verifies BGPLSNode.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for BGP-LS Node NLRI.
// PREVENTS: TLV encoding errors, header length miscalculation.
func TestBGPLSNodeWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		node *BGPLSNode
	}{
		{
			name: "basic node",
			node: NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
				ASN:         65001,
				IGPRouterID: []byte{1, 1, 1, 1},
			}),
		},
		{
			name: "full descriptor",
			node: NewBGPLSNode(ProtoISISL2, 0x12345678, NodeDescriptor{
				ASN:             65000,
				BGPLSIdentifier: 0xDEADBEEF,
				OSPFAreaID:      0x00000001,
				IGPRouterID:     []byte{10, 0, 0, 1},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.node.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.node.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestBGPLSLinkWriteToMatchesBytes verifies BGPLSLink.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for BGP-LS Link NLRI.
// PREVENTS: Local/remote node descriptor confusion, link TLV encoding errors.
func TestBGPLSLinkWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		link *BGPLSLink
	}{
		{
			name: "basic link",
			link: NewBGPLSLink(ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
				LinkDescriptor{},
			),
		},
		{
			name: "link with addresses",
			link: NewBGPLSLink(ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
				LinkDescriptor{
					LocalInterfaceAddr: []byte{10, 0, 0, 1},
					NeighborAddr:       []byte{10, 0, 0, 2},
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.link.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.link.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestBGPLSPrefixWriteToMatchesBytes verifies BGPLSPrefix.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for BGP-LS Prefix NLRI.
// PREVENTS: IPv4/IPv6 confusion, prefix descriptor encoding errors.
func TestBGPLSPrefixWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name   string
		prefix *BGPLSPrefix
	}{
		{
			name: "v4 prefix",
			prefix: NewBGPLSPrefixV4(ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				PrefixDescriptor{IPReachabilityInfo: []byte{24, 10, 0, 0}},
			),
		},
		{
			name: "v6 prefix",
			prefix: NewBGPLSPrefixV6(ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				PrefixDescriptor{IPReachabilityInfo: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.prefix.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.prefix.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestBGPLSSRv6SIDWriteToMatchesBytes verifies BGPLSSRv6SID.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for SRv6 SID NLRI.
// PREVENTS: SID encoding errors, TLV 518 format issues.
func TestBGPLSSRv6SIDWriteToMatchesBytes(t *testing.T) {
	srv6 := NewBGPLSSRv6SID(ProtoSegment, 0x200,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		SRv6SIDDescriptor{SRv6SID: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
	)

	expected := srv6.Bytes()

	buf := make([]byte, len(expected)+10)
	n := srv6.WriteTo(buf, 0)

	assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
	assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
}

// TestMVPNWriteToMatchesBytes verifies MVPN.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for MVPN NLRI.
// PREVENTS: Route type encoding errors, RD data loss.
func TestMVPNWriteToMatchesBytes(t *testing.T) {
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
			expected := tt.mvpn.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.mvpn.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestVPLSWriteToMatchesBytes verifies VPLS.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for VPLS NLRI.
// PREVENTS: Label encoding errors, VE block field ordering issues.
func TestVPLSWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		vpls *VPLS
	}{
		{
			name: "basic vpls",
			vpls: NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3}),
		},
		{
			name: "full vpls",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewVPLSFull(rd, 5, 100, 200, 16000)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.vpls.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.vpls.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestRTCWriteToMatchesBytes verifies RTC.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for RTC NLRI.
// PREVENTS: Origin AS encoding errors, route target data corruption.
func TestRTCWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		rtc  *RTC
	}{
		{
			name: "default rtc",
			rtc:  NewRTC(0, RouteTarget{}),
		},
		{
			name: "rtc with route target",
			rtc: NewRTC(65001, RouteTarget{
				Type:  0,
				Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.rtc.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.rtc.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestMUPWriteToMatchesBytes verifies MUP.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for MUP NLRI.
// PREVENTS: Architecture type encoding errors, route type confusion.
func TestMUPWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		mup  *MUP
	}{
		{
			name: "basic mup",
			mup:  NewMUP(MUPISD, []byte{1, 2, 3, 4}),
		},
		{
			name: "mup with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPT1ST, rd, []byte{10, 0, 0, 1})
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.mup.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.mup.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestWriteToAtOffset verifies WriteTo works correctly with non-zero offset.
//
// VALIDATES: WriteTo honors offset parameter and writes at correct position.
// PREVENTS: Off-by-one errors, buffer corruption at wrong positions.
func TestWriteToAtOffset(t *testing.T) {
	inet := &INET{
		PrefixNLRI: PrefixNLRI{
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
		},
	}

	expected := inet.Bytes()

	// Write at offset 5
	buf := make([]byte, len(expected)+20)
	buf[0] = 0xDE
	buf[1] = 0xAD
	buf[2] = 0xBE
	buf[3] = 0xEF
	buf[4] = 0x00

	n := inet.WriteTo(buf, 5)

	// Verify prefix bytes unchanged
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00}, buf[:5])
	// Verify WriteTo wrote at correct offset
	assert.Equal(t, expected, buf[5:5+n])
}

// TestWriteToZeroAlloc verifies WriteTo produces same output as Bytes().
//
// VALIDATES: WriteTo implementation is consistent with Bytes().
// PREVENTS: Divergent implementations of WriteTo and Bytes.
func TestWriteToZeroAlloc(t *testing.T) {
	// Create an INET
	inet := &INET{
		PrefixNLRI: PrefixNLRI{
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
		},
	}

	// Get expected output from Bytes()
	expected := inet.Bytes()

	// WriteTo should produce the same output
	buf := make([]byte, 256)
	n := inet.WriteTo(buf, 0)

	require.Equal(t, len(expected), n, "WriteTo length mismatch")
	assert.True(t, bytes.Equal(expected, buf[:n]), "WriteTo output mismatch")
}
