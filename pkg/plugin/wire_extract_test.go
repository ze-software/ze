package plugin

import (
	"encoding/hex"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractRawAttributes verifies attribute extraction from UPDATE payload.
//
// VALIDATES: Raw attribute bytes can be extracted for pool storage.
// PREVENTS: Incorrect offset calculation leading to corrupt data.
func TestExtractRawAttributes(t *testing.T) {
	tests := []struct {
		name    string
		payload string // hex-encoded UPDATE payload (after header)
		want    string // hex-encoded expected attributes
		wantErr bool
	}{
		{
			name: "simple_ipv4_update",
			// Withdrawn: 0 bytes, Attrs: ORIGIN(IGP)+AS_PATH+NEXT_HOP+MED, NLRI: 10.0.0.0/24
			payload: "0000" + // withdrawn len = 0
				"0015" + // attrs len = 21
				"40010100" + // ORIGIN: IGP (4 bytes)
				"400200" + // AS_PATH: empty (3 bytes)
				"4003040a000001" + // NEXT_HOP: 10.0.0.1 (7 bytes)
				"80040400000064" + // MED: 100 (7 bytes)
				"180a0000", // NLRI: 10.0.0.0/24 (4 bytes)
			want: "40010100" + "400200" + "4003040a000001" + "80040400000064",
		},
		{
			name: "with_withdrawn",
			// Withdrawn: 10.1.0.0/24, Attrs: small, NLRI: 10.0.0.0/24
			payload: "0004" + // withdrawn len = 4
				"180a0100" + // Withdrawn: 10.1.0.0/24
				"000b" + // attrs len = 11
				"40010100" + // ORIGIN: IGP
				"4003040a000001" + // NEXT_HOP: 10.0.0.1
				"180a0000", // NLRI: 10.0.0.0/24
			want: "40010100" + "4003040a000001",
		},
		{
			name: "empty_attrs",
			// Withdrawn: 0, Attrs: empty, NLRI: 10.0.0.0/24
			// Empty attrs = nil, which is correct behavior
			payload: "0000" + // withdrawn len = 0
				"0000" + // attrs len = 0
				"180a0000", // NLRI: 10.0.0.0/24
			want: "", // nil expected - empty string means nil
		},
		{
			name:    "truncated_payload",
			payload: "00", // Too short
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := hex.DecodeString(tt.payload)
			require.NoError(t, err)

			wu := NewWireUpdate(payload, 0)
			got, err := ExtractRawAttributes(wu)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.want == "" {
				assert.Nil(t, got)
			} else {
				want, _ := hex.DecodeString(tt.want)
				assert.Equal(t, want, got)
			}
		})
	}
}

// TestExtractRawNLRIByFamily verifies NLRI extraction per family.
//
// VALIDATES: Raw NLRI bytes extracted correctly for each address family.
// PREVENTS: Wrong family NLRI returned (IPv4 vs MP_REACH confusion).
func TestExtractRawNLRIByFamily(t *testing.T) {
	t.Run("ipv4_unicast_from_body", func(t *testing.T) {
		// IPv4 unicast NLRI comes from message body
		payload, _ := hex.DecodeString(
			"0000" + // withdrawn len = 0
				"000b" + // attrs len = 11
				"40010100" + // ORIGIN: IGP
				"4003040a000001" + // NEXT_HOP: 10.0.0.1
				"180a0000" + // NLRI: 10.0.0.0/24
				"100a01", // NLRI: 10.1.0.0/16
		)

		wu := NewWireUpdate(payload, 0)

		got, err := ExtractRawNLRI(wu, nlri.IPv4Unicast, false)
		require.NoError(t, err)

		// Should get the body NLRI bytes
		want, _ := hex.DecodeString("180a0000" + "100a01")
		assert.Equal(t, want, got)
	})

	t.Run("ipv6_unicast_from_mp_reach", func(t *testing.T) {
		// IPv6 unicast NLRI comes from MP_REACH_NLRI attribute
		// MP_REACH format: AFI(2) SAFI(1) NH_LEN(1) NH(var) Reserved(1) NLRI(var)
		// Build the MP_REACH_NLRI value bytes
		mpReachValue := "" +
			"0002" + // AFI: IPv6 (2)
			"01" + // SAFI: unicast (1)
			"10" + // NH len: 16
			"20010db8000000000000000000000001" + // NH: 2001:db8::1
			"00" + // Reserved
			"30" + // NLRI prefix len: 48
			"20010db80001" // NLRI bytes: 2001:db8:1::/48

		mpReachValueBytes, _ := hex.DecodeString(mpReachValue)

		// Build attributes: ORIGIN(4) + MP_REACH_NLRI(3+len)
		// MP_REACH header: flags(1) + code(1) + extended_len(2)
		originAttr := "40010100" // ORIGIN IGP
		mpReachAttr := "900e" +  // flags(90=optional+transitive+extended) code(14)
			hex.EncodeToString([]byte{byte(len(mpReachValueBytes) >> 8), byte(len(mpReachValueBytes))}) +
			mpReachValue

		attrsHex := originAttr + mpReachAttr
		attrsBytes, _ := hex.DecodeString(attrsHex)

		// Build UPDATE payload
		payload, _ := hex.DecodeString(
			"0000" + // withdrawn len = 0
				hex.EncodeToString([]byte{byte(len(attrsBytes) >> 8), byte(len(attrsBytes))}) + // attrs len
				attrsHex,
		)

		wu := NewWireUpdate(payload, 0)

		got, err := ExtractRawNLRI(wu, nlri.IPv6Unicast, false)
		require.NoError(t, err)

		// Should get MP_REACH NLRI bytes only (after NH + reserved)
		want, _ := hex.DecodeString("30" + "20010db80001")
		assert.Equal(t, want, got)
	})

	t.Run("family_not_present", func(t *testing.T) {
		// IPv4 update, but asking for IPv6
		payload, _ := hex.DecodeString(
			"0000" + // withdrawn len = 0
				"000b" + // attrs len = 11
				"40010100" + // ORIGIN: IGP
				"4003040a000001" + // NEXT_HOP
				"180a0000", // NLRI: 10.0.0.0/24
		)

		wu := NewWireUpdate(payload, 0)

		got, err := ExtractRawNLRI(wu, nlri.IPv6Unicast, false)
		require.NoError(t, err)
		assert.Nil(t, got) // Not present = nil, no error
	})
}

// TestExtractRawWithdrawn verifies withdrawn NLRI extraction.
//
// VALIDATES: Withdrawn routes extracted for storage.
// PREVENTS: Missing withdrawn routes on session replay.
func TestExtractRawWithdrawn(t *testing.T) {
	t.Run("ipv4_unicast", func(t *testing.T) {
		payload, _ := hex.DecodeString(
			"0008" + // withdrawn len = 8
				"180a0000" + // 10.0.0.0/24
				"180a0100" + // 10.1.0.0/24
				"0000", // attrs len = 0
		)

		wu := NewWireUpdate(payload, 0)

		got, err := ExtractRawWithdrawn(wu, nlri.IPv4Unicast, false)
		require.NoError(t, err)

		want, _ := hex.DecodeString("180a0000" + "180a0100")
		assert.Equal(t, want, got)
	})

	t.Run("ipv6_from_mp_unreach", func(t *testing.T) {
		// MP_UNREACH format: AFI(2) SAFI(1) Withdrawn(var)
		mpUnreachValue := "" +
			"0002" + // AFI: IPv6
			"01" + // SAFI: unicast
			"30" + // prefix len: 48
			"20010db80001" // 2001:db8:1::/48

		mpUnreachBytes, _ := hex.DecodeString(mpUnreachValue)

		// Build attributes: MP_UNREACH_NLRI(4+len) with extended length
		mpUnreachAttr := "900f" + // flags(90=optional+transitive+extended) code(15)
			hex.EncodeToString([]byte{byte(len(mpUnreachBytes) >> 8), byte(len(mpUnreachBytes))}) +
			mpUnreachValue

		attrsHex := mpUnreachAttr
		attrsBytes, _ := hex.DecodeString(attrsHex)

		// Build UPDATE payload
		payload, _ := hex.DecodeString(
			"0000" + // withdrawn len = 0
				hex.EncodeToString([]byte{byte(len(attrsBytes) >> 8), byte(len(attrsBytes))}) + // attrs len
				attrsHex,
		)

		wu := NewWireUpdate(payload, 0)

		got, err := ExtractRawWithdrawn(wu, nlri.IPv6Unicast, false)
		require.NoError(t, err)

		// Should get the withdrawn NLRI bytes only (after AFI/SAFI)
		want, _ := hex.DecodeString("30" + "20010db80001")
		assert.Equal(t, want, got)
	})
}
