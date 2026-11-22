// Design: docs/architecture/wire/attributes.md -- RFC 8097 send-path conformance tests
// Related: internal/core/bgp/attribute/origin_validation_state.go -- NewOriginValidationStateExtCommunity, ZeroOriginValidationStateReservedBytes
// Related: internal/core/bgp/attribute/builder_parse.go -- ParseSingleExtCommunity, parseExtCommunityTypedHex
// Related: internal/core/bgp/attribute/extcomm_hex.go -- ParseExtendedCommunityHex
// Related: internal/core/bgp/attribute/origin.go -- WriteAttributesOrdered
// Related: update_build.go -- UpdateBuilder.Build
// RFC 8097 Section 2 -- see rfc/short/rfc8097.md

package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC8097ReservedFieldZeroOnSend proves RFC 8097 Section 2 across all of
// Ze's real send and configuration/policy parsing entrypoints:
//   - typed constructor (NewOriginValidationStateExtCommunity)
//   - unified policy/route parser (ParseSingleExtCommunity: keyword, raw hex, AND typed-hex 0x4300:...)
//   - typed ExtendedCommunities.WriteTo
//   - real UPDATE wire builder (UpdateBuilder.Build via WriteAttributesOrdered)
//
// RFC requirement: RFC8097-2-1 positive -- NewOriginValidationStateExtCommunity, ParseSingleExtCommunity (keyword, raw hex, typed hex), ExtendedCommunities.WriteTo, and UpdateBuilder.Build all emit type 0x43, sub-type 0x00, five zero Reserved octets [2..6], and the validation state octet [7], zeroing dirty Reserved octets on 0x43/0x00 in real wire UPDATE PathAttributes without altering sibling extended communities.
// RFC requirement: RFC8097-2-1 negative -- ParseSingleExtCommunity (both raw hex 0x4300... and typed hex 0x4300:...) refuses any 0x43/0x00 value carrying a non-zero Reserved octet (bytes 2..6) or validation state > 2, while non-0x43/0x00 opaque extended communities (e.g. 0x430b) retain arbitrary payload octets verbatim in both parsing and UpdateBuilder.Build wire output.
func TestRFC8097ReservedFieldZeroOnSend(t *testing.T) {
	// Positive 1: typed constructor sets [0]=0x43, [1]=0x00, [2..6]=0, [7]=state.
	for _, state := range []attribute.OriginValidationState{
		attribute.OriginValidationValid,
		attribute.OriginValidationNotFound,
		attribute.OriginValidationInvalid,
	} {
		ec, err := attribute.NewOriginValidationStateExtCommunity(state)
		require.NoError(t, err)
		require.Equal(t, attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, uint8(state)}, ec)
	}
	_, err := attribute.NewOriginValidationStateExtCommunity(attribute.OriginValidationState(3))
	require.Error(t, err)

	// Positive 2: ParseSingleExtCommunity accepts keyword, raw hex, and typed-hex forms with zero Reserved octets.
	for _, tc := range []struct {
		input string
		state uint8
	}{
		{"origin-validation-state:valid", 0},
		{"origin-validation-state:not-found", 1},
		{"origin-validation-state:invalid", 2},
		{"0x4300000000000000", 0},
		{"0x4300000000000001", 1},
		{"0x4300000000000002", 2},
		{"0x4300:000000000000", 0},
		{"0x4300:000000000001", 1},
		{"0x4300:000000000002", 2},
	} {
		ec, err := attribute.ParseSingleExtCommunity(tc.input)
		require.NoError(t, err, "ParseSingleExtCommunity(%q)", tc.input)
		require.Equal(t, attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, tc.state}, ec)
	}

	// Positive 3: ExtendedCommunities.WriteTo zeroes dirty Reserved bytes [2..6] on 0x43/0x00.
	dirtyOV := attribute.ExtendedCommunity{0x43, 0x00, 0xde, 0xad, 0xbe, 0xef, 0x55, 0x02}
	color := attribute.ExtendedCommunity{0x03, 0x0b, 0x3f, 0xa5, 0x00, 0x00, 0x00, 0x2a}
	comms := attribute.ExtendedCommunities{dirtyOV, color}
	buf := make([]byte, comms.Len())
	require.Equal(t, 16, comms.WriteTo(buf, 0))
	require.Equal(t, []byte{0x43, 0x00, 0, 0, 0, 0, 0, 0x02}, buf[0:8])
	require.Equal(t, color[:], buf[8:16])

	// Positive 4: real UpdateBuilder.Build (which routes ExtCommunityBytes through rawAttribute
	// and WriteAttributesOrdered) zeroes dirty Reserved bytes [2..6] of 0x43/0x00 on wire emission
	// while keeping sibling extended communities verbatim and leaving caller input untouched.
	dirtyInput := make([]byte, 0, 16)
	dirtyInput = append(dirtyInput, dirtyOV[:]...)
	dirtyInput = append(dirtyInput, color[:]...)
	snapshot := append([]byte(nil), dirtyInput...)

	ub := GetUpdateBuilder(65000, true, true, false)
	defer PutUpdateBuilder(ub)
	upd, err := ub.BuildUnicast(&UnicastParams{
		Prefix:             netip.MustParsePrefix("198.51.100.0/24"),
		NextHop:            netip.MustParseAddr("192.0.2.1"),
		Origin:             attribute.OriginIGP,
		LocalPreference:    100,
		HasLocalPreference: true,
		ExtCommunityBytes:  dirtyInput,
	})
	require.NoError(t, err)
	_, _, wireExt, found := attribute.AttrFind(upd.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Len(t, wireExt, 16)
	require.Equal(t, []byte{0x43, 0x00, 0, 0, 0, 0, 0, 0x02}, wireExt[0:8], "UpdateBuilder wire output must zero RFC 8097 Reserved octets [2..6]")
	require.Equal(t, color[:], wireExt[8:16], "UpdateBuilder must keep non-RFC-8097 extended communities verbatim")
	require.Equal(t, snapshot, dirtyInput, "UpdateBuilder must not mutate caller ExtCommunityBytes slice")

	// Negative 1: ParseSingleExtCommunity refuses BOTH raw hex and typed hex when ANY Reserved byte [2..6] is non-zero or state > 2.
	for _, bad := range []string{
		"0x4300010000000000",  // raw hex: byte 2 non-zero
		"0x4300000100000000",  // raw hex: byte 3 non-zero
		"0x4300000001000000",  // raw hex: byte 4 non-zero
		"0x4300000000010000",  // raw hex: byte 5 non-zero
		"0x4300000000000100",  // raw hex: byte 6 non-zero
		"0x4300000000000003",  // raw hex: state > 2
		"0x4300:010000000000", // typed hex: byte 2 non-zero
		"0x4300:000100000000", // typed hex: byte 3 non-zero
		"0x4300:000001000000", // typed hex: byte 4 non-zero
		"0x4300:000000010000", // typed hex: byte 5 non-zero
		"0x4300:000000000100", // typed hex: byte 6 non-zero
		"0x4300:000000000003", // typed hex: state > 2
	} {
		_, err := attribute.ParseSingleExtCommunity(bad)
		require.Error(t, err, "ParseSingleExtCommunity(%q) must refuse non-zero Reserved or state > 2", bad)
	}

	// Negative 2: non-0x4300 opaque extended community (0x430b) with arbitrary bytes [2..6]
	// parses cleanly in both hex forms and is emitted verbatim by UpdateBuilder.Build.
	otherRaw, err := attribute.ParseSingleExtCommunity("0x430bdeadbeef5502")
	require.NoError(t, err)
	otherTyped, err := attribute.ParseSingleExtCommunity("0x430b:deadbeef5502")
	require.NoError(t, err)
	wantOther := attribute.ExtendedCommunity{0x43, 0x0b, 0xde, 0xad, 0xbe, 0xef, 0x55, 0x02}
	require.Equal(t, wantOther, otherRaw)
	require.Equal(t, wantOther, otherTyped)

	updOther, err := ub.BuildUnicast(&UnicastParams{
		Prefix:             netip.MustParsePrefix("198.51.100.0/24"),
		NextHop:            netip.MustParseAddr("192.0.2.1"),
		Origin:             attribute.OriginIGP,
		LocalPreference:    100,
		HasLocalPreference: true,
		ExtCommunityBytes:  wantOther[:],
	})
	require.NoError(t, err)
	_, _, wireOther, foundOther := attribute.AttrFind(updOther.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, foundOther)
	require.Equal(t, wantOther[:], wireOther)
}
