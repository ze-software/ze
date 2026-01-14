package nlri

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMVPNTypes verifies MVPN route types.
func TestMVPNTypes(t *testing.T) {
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
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, []byte{1, 2, 3, 4})

	assert.Equal(t, MVPNIntraASIPMSIAD, mvpn.RouteType())
	assert.NotNil(t, mvpn.Bytes())
}

// TestMVPNFamily verifies MVPN address family.
func TestMVPNFamily(t *testing.T) {
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, nil)

	// IPv4 MVPN uses AFI 1, SAFI 5
	assert.Equal(t, AFIIPv4, mvpn.Family().AFI)
	assert.Equal(t, SAFIMVPN, mvpn.Family().SAFI)
}

// TestMVPNWithRD verifies MVPN with Route Distinguisher.
func TestMVPNWithRD(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	mvpn := NewMVPNWithRD(AFIIPv6, MVPNIntraASIPMSIAD, rd, []byte{1, 2, 3, 4})

	assert.Equal(t, AFIIPv6, mvpn.Family().AFI)
	assert.Equal(t, rd, mvpn.RD())
}

// TestMVPNRoundTrip verifies encode/decode cycle.
func TestMVPNRoundTrip(t *testing.T) {
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
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated header", []byte{0x01}},
		{"truncated body", []byte{0x01, 0x10}}, // claims 16 bytes but none
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseMVPN(AFIIPv4, tt.data)
			assert.Error(t, err)
		})
	}
}

// TestMVPNStringCommandStyle verifies command-style string representation.
//
// VALIDATES: MVPN String() outputs command-style format for API round-trip.
// Format: <type> [rd set <rd>].
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestMVPNStringCommandStyle(t *testing.T) {
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
			expected: "source-tree-join rd set 0:65001:100",
		},
		{
			name: "mvpn s-pmsi-ad with rd",
			mvpn: func() *MVPN {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewMVPNWithRD(AFIIPv6, MVPNSPMSIAD, rd, nil)
			}(),
			expected: "s-pmsi-ad rd set 1:10.0.0.1:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mvpn.String())
		})
	}
}

// TestVPLSBasic verifies basic VPLS NLRI creation.
func TestVPLSBasic(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	assert.Equal(t, uint16(100), vpls.VEBlockOffset())
	assert.Equal(t, uint16(200), vpls.VEBlockSize())
}

// TestVPLSFamily verifies VPLS address family.
func TestVPLSFamily(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{}, 0, 0, nil)

	// VPLS uses AFI 25, SAFI 65
	assert.Equal(t, AFIL2VPN, vpls.Family().AFI)
	assert.Equal(t, SAFIVPLS, vpls.Family().SAFI)
}

// TestVPLSBytes verifies VPLS wire format.
func TestVPLSBytes(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	data := vpls.Bytes()
	require.NotEmpty(t, data)
	// VPLS NLRI is 19 bytes: 2 len + 8 RD + 2 VE ID + 2 offset + 2 size + 3 label
	assert.Equal(t, 19, len(data))
}

// TestVPLSFull verifies full VPLS NLRI creation.
func TestVPLSFull(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	vpls := NewVPLSFull(rd, 1, 10, 20, 16000)

	assert.Equal(t, rd, vpls.RD())
	assert.Equal(t, uint16(1), vpls.VEID())
	assert.Equal(t, uint16(10), vpls.VEBlockOffset())
	assert.Equal(t, uint16(20), vpls.VEBlockSize())
	assert.Equal(t, uint32(16000), vpls.LabelBase())
}

// TestVPLSRoundTrip verifies encode/decode cycle.
func TestVPLSRoundTrip(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	original := NewVPLSFull(rd, 5, 100, 200, 16000)
	data := original.Bytes()

	parsed, remaining, err := ParseVPLS(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.RD(), parsed.RD())
	assert.Equal(t, original.VEID(), parsed.VEID())
	assert.Equal(t, original.VEBlockOffset(), parsed.VEBlockOffset())
	assert.Equal(t, original.VEBlockSize(), parsed.VEBlockSize())
	assert.Equal(t, original.LabelBase(), parsed.LabelBase())
}

// TestVPLSParseErrors verifies error handling.
func TestVPLSParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated length", []byte{0x00}},
		{"short length", []byte{0x00, 0x02}},                // claims 2 bytes
		{"too short", []byte{0x00, 0x11, 0, 0, 0, 0, 0, 0}}, // claims 17 but only 6
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseVPLS(tt.data)
			assert.Error(t, err)
		})
	}
}

// TestVPLSStringCommandStyle verifies command-style string representation.
//
// VALIDATES: VPLS String() outputs command-style format for API round-trip.
// Format: rd set <rd> ve-id set <id> label set <label>.
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestVPLSStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		vpls     *VPLS
		expected string
	}{
		{
			name: "basic vpls",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewVPLSFull(rd, 5, 0, 0, 16000)
			}(),
			expected: "rd set 0:65001:100 ve-id set 5 label set 16000",
		},
		{
			name: "vpls with type1 rd",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewVPLSFull(rd, 10, 0, 0, 500)
			}(),
			expected: "rd set 1:10.0.0.1:200 ve-id set 10 label set 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.vpls.String())
		})
	}
}

// TestRTCBasic verifies basic RTC NLRI creation.
func TestRTCBasic(t *testing.T) {
	rt := RouteTarget{
		Type:  0,                                 // 2-byte ASN
		Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100}, // AS 65001 : 100
	}
	rtc := NewRTC(65001, rt)

	assert.Equal(t, uint32(65001), rtc.OriginAS())
}

// TestRTCFamily verifies RTC address family.
func TestRTCFamily(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{})

	// RTC uses AFI 1, SAFI 132
	assert.Equal(t, AFIIPv4, rtc.Family().AFI)
	assert.Equal(t, SAFIRTC, rtc.Family().SAFI)
}

// TestRTCBytes verifies RTC wire format.
func TestRTCBytes(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{
		Type:  0,
		Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
	})

	data := rtc.Bytes()
	require.NotEmpty(t, data)
	// Full RTC NLRI: 1 prefix-len + 4 origin AS + 8 RT = 13 bytes
	assert.Equal(t, 13, len(data))
}

// TestRTCDefault verifies default RTC (matches all RTs).
func TestRTCDefault(t *testing.T) {
	rtc := NewRTC(0, RouteTarget{})

	assert.True(t, rtc.IsDefault())
	assert.Equal(t, []byte{0}, rtc.Bytes())
}

// TestRTCRoundTrip verifies encode/decode cycle.
func TestRTCRoundTrip(t *testing.T) {
	rt := RouteTarget{
		Type:  0x0002,                            // 4-byte ASN
		Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 100}, // AS 65001 : 100
	}
	original := NewRTC(65001, rt)
	data := original.Bytes()

	parsed, remaining, err := ParseRTC(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.OriginAS(), parsed.OriginAS())
	assert.Equal(t, original.RouteTarget().Type, parsed.RouteTarget().Type)
}

// TestRTCParseDefault verifies parsing default RTC.
func TestRTCParseDefault(t *testing.T) {
	data := []byte{0} // prefix-length = 0

	parsed, remaining, err := ParseRTC(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.True(t, parsed.IsDefault())
}

// TestRTCParseErrors verifies error handling.
func TestRTCParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseRTC(tt.data)
			assert.Error(t, err)
		})
	}
}

// TestRouteTargetString verifies RT string formatting.
func TestRouteTargetString(t *testing.T) {
	tests := []struct {
		name     string
		rt       RouteTarget
		expected string
	}{
		{
			name: "2-byte ASN",
			// Type 0x0002 has high byte 0x00 = 2-byte ASN format
			// Value: 2-byte ASN (65001 = 0xFDE9) + 4-byte assigned (100 = 0x00000064)
			rt:       RouteTarget{Type: 0x0002, Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100}},
			expected: "65001:100",
		},
		{
			name: "4-byte ASN",
			// Type 0x0200 has high byte 0x02 = 4-byte ASN format
			// Value: 4-byte ASN (65001 = 0x0000FDE9) + 2-byte assigned (100 = 0x0064)
			rt:       RouteTarget{Type: 0x0200, Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 100}},
			expected: "65001:100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.String())
		})
	}
}

// TestRTCStringCommandStyle verifies command-style string representation.
//
// VALIDATES: RTC String() outputs command-style format for API round-trip.
// Format: default | origin-as set <asn> rt set <rt>.
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestRTCStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		rtc      *RTC
		expected string
	}{
		{
			name:     "default rtc",
			rtc:      NewRTC(0, RouteTarget{}),
			expected: "default",
		},
		{
			name: "rtc with 2-byte asn rt",
			rtc: NewRTC(65001, RouteTarget{
				Type:  0x0002,
				Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100}, // AS65001:100
			}),
			expected: "origin-as set 65001 rt set 65001:100",
		},
		{
			name: "rtc with 4-byte asn rt",
			rtc: NewRTC(65002, RouteTarget{
				Type:  0x0200,
				Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 200}, // AS65001:200
			}),
			expected: "origin-as set 65002 rt set 65001:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rtc.String())
		})
	}
}

// TestMUPTypes verifies MUP route types.
func TestMUPTypes(t *testing.T) {
	assert.Equal(t, MUPRouteType(1), MUPISD)
	assert.Equal(t, MUPRouteType(2), MUPDSD)
	assert.Equal(t, MUPRouteType(3), MUPT1ST)
	assert.Equal(t, MUPRouteType(4), MUPT2ST)
}

// TestMUPBasic verifies basic MUP NLRI creation.
func TestMUPBasic(t *testing.T) {
	mup := NewMUP(MUPISD, []byte{1, 2, 3, 4})

	assert.Equal(t, MUPISD, mup.RouteType())
	assert.Equal(t, MUPArch3GPP5G, mup.ArchType())
}

// TestMUPFamily verifies MUP address family.
func TestMUPFamily(t *testing.T) {
	mup := NewMUP(MUPISD, nil)

	// MUP uses AFI 1, SAFI 85
	assert.Equal(t, AFIIPv4, mup.Family().AFI)
	assert.Equal(t, SAFIMUP, mup.Family().SAFI)
}

// TestMUPFull verifies full MUP NLRI creation.
func TestMUPFull(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	mup := NewMUPFull(AFIIPv6, MUPArch3GPP5G, MUPT1ST, rd, []byte{1, 2, 3, 4})

	assert.Equal(t, AFIIPv6, mup.Family().AFI)
	assert.Equal(t, MUPArch3GPP5G, mup.ArchType())
	assert.Equal(t, MUPT1ST, mup.RouteType())
	assert.Equal(t, rd, mup.RD())
}

// TestMUPRoundTrip verifies encode/decode cycle.
func TestMUPRoundTrip(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	original := NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPISD, rd, []byte{10, 0, 0, 1})
	data := original.Bytes()

	parsed, remaining, err := ParseMUP(AFIIPv4, data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.ArchType(), parsed.ArchType())
	assert.Equal(t, original.RouteType(), parsed.RouteType())
	assert.Equal(t, original.RD(), parsed.RD())
}

// TestMUPParseErrors verifies error handling.
func TestMUPParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated header", []byte{0x01}},
		{"truncated body", []byte{0x01, 0x00, 0x01, 0x10}}, // claims 16 bytes but none
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseMUP(AFIIPv4, tt.data)
			assert.Error(t, err)
		})
	}
}

// TestMUPStringCommandStyle verifies command-style string representation.
//
// VALIDATES: MUP String() outputs command-style format for API round-trip.
// Format: <type> [rd set <rd>].
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestMUPStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		mup      *MUP
		expected string
	}{
		{
			name:     "mup without rd",
			mup:      NewMUP(MUPISD, []byte{1, 2, 3, 4}),
			expected: "isd",
		},
		{
			name: "mup with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPT1ST, rd, []byte{10, 0, 0, 1})
			}(),
			expected: "t1st rd set 0:65001:100",
		},
		{
			name: "mup dsd with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewMUPFull(AFIIPv6, MUPArch3GPP5G, MUPDSD, rd, nil)
			}(),
			expected: "dsd rd set 1:10.0.0.1:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mup.String())
		})
	}
}

// TestSAFIConstants verifies additional SAFI constants exist.
func TestSAFIConstants(t *testing.T) {
	assert.Equal(t, SAFI(5), SAFIMVPN)
	assert.Equal(t, SAFI(65), SAFIVPLS)
	assert.Equal(t, SAFI(85), SAFIMUP)
	assert.Equal(t, SAFI(132), SAFIRTC)
}

// TestFamilyVariables verifies P2 family variables.
func TestFamilyVariables(t *testing.T) {
	assert.Equal(t, AFIIPv4, IPv4MVPN.AFI)
	assert.Equal(t, SAFIMVPN, IPv4MVPN.SAFI)

	assert.Equal(t, AFIIPv6, IPv6MVPN.AFI)
	assert.Equal(t, SAFIMVPN, IPv6MVPN.SAFI)

	assert.Equal(t, AFIL2VPN, L2VPNVPLS.AFI)
	assert.Equal(t, SAFIVPLS, L2VPNVPLS.SAFI)

	assert.Equal(t, AFIIPv4, IPv4RTC.AFI)
	assert.Equal(t, SAFIRTC, IPv4RTC.SAFI)

	assert.Equal(t, AFIIPv4, IPv4MUP.AFI)
	assert.Equal(t, SAFIMUP, IPv4MUP.SAFI)

	assert.Equal(t, AFIIPv6, IPv6MUP.AFI)
	assert.Equal(t, SAFIMUP, IPv6MUP.SAFI)
}

// TestSAFIStrings verifies SAFI String() method.
func TestSAFIStrings(t *testing.T) {
	tests := []struct {
		safi     SAFI
		expected string
	}{
		{SAFIMVPN, "mvpn"},
		{SAFIVPLS, "vpls"},
		{SAFIMUP, "mup"},
		{SAFIRTC, "rtc"},
		{SAFIBGPLinkState, "bgp-ls"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.safi.String())
		})
	}
}

// TestFamilyParsing verifies family string parsing.
func TestFamilyParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Family
		ok       bool
	}{
		{"ipv4/mvpn", IPv4MVPN, true},
		{"ipv6/mvpn", IPv6MVPN, true},
		{"l2vpn/vpls", L2VPNVPLS, true},
		{"ipv4/rtc", IPv4RTC, true},
		{"ipv4/mup", IPv4MUP, true},
		{"ipv6/mup", IPv6MUP, true},
		{"unknown", Family{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			family, ok := ParseFamily(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, family)
			}
		})
	}
}
