// Design: docs/architecture/wire/attributes.md -- RFC 8097 Origin Validation State Extended Community unit tests
// Related: origin_validation_state.go -- NewOriginValidationStateExtCommunity, DecodeOriginValidationStateExtCommunity, ResolveOriginValidationState, SanitizeOriginValidationStateAttr
// Related: extcomm_hex.go -- ParseExtendedCommunityHex
// Related: community.go -- ExtendedCommunities.WriteTo
// RFC 8097 Section 2 -- see rfc/short/rfc8097.md

package attribute

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8097HelperReservedFieldZeroOnSend verifies the package-level attribute
// helpers for RFC 8097 Section 2 (end-to-end UPDATE send tests live in message/).
func TestRFC8097HelperReservedFieldZeroOnSend(t *testing.T) {
	// Positive 1: typed constructor sets [0]=0x43, [1]=0x00, [2..6]=0, [7]=state.
	for _, state := range []OriginValidationState{OriginValidationValid, OriginValidationNotFound, OriginValidationInvalid} {
		ec, err := NewOriginValidationStateExtCommunity(state)
		require.NoError(t, err)
		require.Equal(t, ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, uint8(state)}, ec)
	}
	_, err := NewOriginValidationStateExtCommunity(OriginValidationState(3))
	require.Error(t, err)

	// Positive 2: keyword parser sets Reserved bytes to 0 and round-trips through String().
	for _, tc := range []struct {
		text  string
		state OriginValidationState
		canon string
	}{
		{"origin-validation-state:valid", OriginValidationValid, "origin-validation-state:valid"},
		{"origin-validation-state:not-found", OriginValidationNotFound, "origin-validation-state:not-found"},
		{"origin-validation-state:invalid", OriginValidationInvalid, "origin-validation-state:invalid"},
		{"origin-validation-state:2", OriginValidationInvalid, "origin-validation-state:invalid"},
	} {
		ec, matched, err := ParseExtendedCommunityKeyword(tc.text)
		require.True(t, matched)
		require.NoError(t, err)
		require.Equal(t, ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, uint8(tc.state)}, ec)
		require.Equal(t, tc.canon, ec.String())
	}

	// Positive 3: valid hex form with zero Reserved octets succeeds.
	hexEC, err := ParseExtendedCommunityHex("0x4300000000000001")
	require.NoError(t, err)
	require.Equal(t, ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 1}, hexEC)

	// Positive 4: ExtendedCommunities.WriteTo zeroes dirty Reserved bytes [2..6] on 0x43/0x00
	// while leaving non-0x43/0x00 extended communities (such as RFC 9012 Color 0x03/0x0b) untouched.
	dirtyOV := ExtendedCommunity{0x43, 0x00, 0xde, 0xad, 0xbe, 0xef, 0x55, 0x02}
	color := ExtendedCommunity{0x03, 0x0b, 0x3f, 0xa5, 0x00, 0x00, 0x00, 0x2a}
	comms := ExtendedCommunities{dirtyOV, color}
	buf := make([]byte, comms.Len())
	n := comms.WriteTo(buf, 0)
	require.Equal(t, 16, n)
	require.Equal(t, []byte{0x43, 0x00, 0, 0, 0, 0, 0, 0x02}, buf[0:8], "WriteTo must emit zero Reserved octets for RFC 8097")
	require.Equal(t, color[:], buf[8:16], "non-RFC-8097 extended communities must be emitted verbatim")

	// Negative 1: ParseExtendedCommunityHex rejects non-zero Reserved byte in each of the 5 positions (bytes 2..6).
	for _, badHex := range []string{
		"0x4300010000000000", // byte 2 non-zero
		"0x4300000100000000", // byte 3 non-zero
		"0x4300000001000000", // byte 4 non-zero
		"0x4300000000010000", // byte 5 non-zero
		"0x4300000000000100", // byte 6 non-zero
		"0x4300000000000003", // state > 2
	} {
		_, err := ParseExtendedCommunityHex(badHex)
		require.Error(t, err, "ParseExtendedCommunityHex(%q) must refuse non-zero Reserved or state > 2", badHex)
	}

	// Negative 2: non-0x4300 extended community with arbitrary bytes [2..6] still parses cleanly.
	other, err := ParseExtendedCommunityHex("0x430bdeadbeef5502")
	require.NoError(t, err)
	require.Equal(t, ExtendedCommunity{0x43, 0x0b, 0xde, 0xad, 0xbe, 0xef, 0x55, 0x02}, other)
}

// TestRFC8097HelperReservedFieldIgnoredOnReceive verifies the package-level
// decoder helpers for RFC 8097 Section 2 (session receive tests live in reactor/).
func TestRFC8097HelperReservedFieldIgnoredOnReceive(t *testing.T) {
	for _, state := range []OriginValidationState{OriginValidationValid, OriginValidationNotFound, OriginValidationInvalid} {
		ec := ExtendedCommunity{0x43, 0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, uint8(state)}
		got, ok := DecodeOriginValidationStateExtCommunity(ec)
		require.True(t, ok, "non-zero Reserved must be ignored on receive for state %d", state)
		require.Equal(t, state, got)
		require.Equal(t, "origin-validation-state:"+state.String(), ec.String())

		resState, found, discarded := ResolveOriginValidationState(ExtendedCommunities{ec})
		require.True(t, found)
		require.Equal(t, state, resState)
		require.Empty(t, discarded)

		rewritten, changed, sanState, hasState, sanDiscarded := SanitizeOriginValidationStateAttr(ec[:], false, false)
		require.True(t, changed, "non-zero Reserved is canonicalized to 0 for downstream consumers")
		require.True(t, hasState)
		require.Equal(t, state, sanState)
		require.Empty(t, sanDiscarded)
		require.Equal(t, []byte{0x43, 0x00, 0, 0, 0, 0, 0, uint8(state)}, rewritten)
	}

	// Negative: wrong type/subtype is not decoded; state > 2 with zero Reserved is rejected.
	_, ok := DecodeOriginValidationStateExtCommunity(ExtendedCommunity{0x43, 0x01, 0, 0, 0, 0, 0, 0})
	require.False(t, ok)
	badVal, ok := DecodeOriginValidationStateExtCommunity(ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 3})
	require.False(t, ok)
	require.Equal(t, OriginValidationState(3), badVal)
}

// TestRFC8097HelperMultipleInstancesSelectGreatest verifies the package-level
// multi-instance resolution helpers (session receive tests live in reactor/).
func TestRFC8097HelperMultipleInstancesSelectGreatest(t *testing.T) {
	rt := ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01} // target:65000:1
	ov0 := ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 0}
	ov1 := ExtendedCommunity{0x43, 0x00, 0x11, 0x22, 0, 0, 0, 1}
	ov2 := ExtendedCommunity{0x43, 0x00, 0, 0, 0x33, 0, 0, 2}
	ov5 := ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 5}

	cases := []struct {
		name      string
		in        ExtendedCommunities
		wantState OriginValidationState
		wantFound bool
		wantDisc  []uint8
	}{
		{"0_then_2_then_1", ExtendedCommunities{ov0, ov2, ov1}, OriginValidationInvalid, true, nil},
		{"2_then_0_then_1", ExtendedCommunities{ov2, ov0, ov1}, OriginValidationInvalid, true, nil},
		{"0_then_1", ExtendedCommunities{ov0, ov1}, OriginValidationNotFound, true, nil},
		{"1_then_0", ExtendedCommunities{ov1, ov0}, OriginValidationNotFound, true, nil},
		{"valid_and_bogus_5_keeps_valid_greatest", ExtendedCommunities{ov0, ov1, ov5}, OriginValidationNotFound, true, []uint8{5}},
		{"only_bogus_yields_none", ExtendedCommunities{ov5}, 0, false, []uint8{5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotState, found, disc := ResolveOriginValidationState(tc.in)
			require.Equal(t, tc.wantFound, found)
			require.Equal(t, tc.wantState, gotState)
			require.Equal(t, tc.wantDisc, disc)

			raw := make([]byte, 0, (len(tc.in)+1)*8)
			raw = append(raw, rt[:]...)
			for _, ec := range tc.in {
				raw = append(raw, ec[:]...)
			}
			rewritten, changed, sanState, hasState, sanDisc := SanitizeOriginValidationStateAttr(raw, false, false)
			require.True(t, changed)
			require.Equal(t, tc.wantFound, hasState)
			require.Equal(t, tc.wantState, sanState)
			require.Equal(t, tc.wantDisc, sanDisc)
			if tc.wantFound {
				require.Len(t, rewritten, 16, "must preserve RT plus exactly ONE winning RFC 8097 instance")
				require.Equal(t, rt[:], rewritten[0:8])
				require.Equal(t, []byte{0x43, 0x00, 0, 0, 0, 0, 0, uint8(tc.wantState)}, rewritten[8:16])
			} else {
				require.Len(t, rewritten, 8, "all bogus RFC 8097 instances discarded, RT preserved")
				require.Equal(t, rt[:], rewritten[0:8])
			}
		})
	}
}
