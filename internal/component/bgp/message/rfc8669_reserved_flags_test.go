package message

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc8669SentPrefixSIDs encodes the Prefix-SID for every configuration shape Ze
// accepts: a bare label index at both ends of its range, and a label index with
// an Originator SRGB list.
func rfc8669SentPrefixSIDs(t *testing.T) map[string][]byte {
	t.Helper()
	sent := map[string][]byte{}
	for _, config := range []string{
		"0",
		"4294967295",
		"300, [( 800000,4096) ,( 1000000,5000)]",
		"4294967295, [(16777215,16777215)]",
	} {
		sid, err := attribute.EncodePrefixSID(config)
		require.NoError(t, err, config)
		sent[config] = sid
	}
	return sent
}

// TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception proves both
// halves of the three "clear on transmission and ignored on reception" rules.
//
// VALIDATES: every Prefix-SID Ze encodes, including the Label-Index TLV that
// leads an SRGB configuration and the all-ones extremes of index, base and
// range, carries a zero Label-Index Reserved octet, zero Label-Index Flags and
// zero SRGB Flags. A received Prefix-SID with those fields set is accepted
// exactly as the same attribute with them clear, and is left byte for byte.
// PREVENTS: an encoder that leaks configured bits into a reserved field, and a
// receiver that acts on, rewrites or refuses bits a newer sender may set.
//
// RFC requirement: RFC8669-3.1-4 positive -- every encoded Label-Index TLV, bare or leading an
// SRGB, has Reserved 0; a received one with Reserved 0 is accepted unchanged.
// RFC requirement: RFC8669-3.1-4 negative -- a received Label-Index TLV with Reserved 0xFF
// gets the same action as Reserved 0 (none), parses, and is left byte for byte; the
// all-ones configured index still encodes Reserved 0.
// RFC requirement: RFC8669-3.1-6 positive -- every encoded Label-Index TLV has Flags 0; a
// received one with Flags 0 is accepted unchanged.
// RFC requirement: RFC8669-3.1-6 negative -- a received Label-Index TLV with Flags 0xFFFF
// gets action none, parses, and is left byte for byte; the all-ones configured index still
// encodes Flags 0.
// RFC requirement: RFC8669-3.2-2 positive -- every encoded Originator SRGB TLV has Flags 0; a
// received one with Flags 0 is accepted unchanged.
// RFC requirement: RFC8669-3.2-2 negative -- a received Originator SRGB TLV with Flags 0xFFFF
// gets action none, parses, and is left byte for byte; all-ones base and range still encode
// SRGB Flags 0.
func TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception(t *testing.T) {
	for config, sid := range rfc8669SentPrefixSIDs(t) {
		require.GreaterOrEqual(t, len(sid), 10, config)
		require.Equal(t, byte(1), sid[0], "%s: Label-Index TLV first", config)
		require.Equal(t, byte(0), sid[3], "%s: Label-Index Reserved", config)
		require.Equal(t, []byte{0, 0}, sid[4:6], "%s: Label-Index Flags", config)
		if len(sid) == 10 {
			continue
		}
		require.Equal(t, byte(3), sid[10], "%s: Originator SRGB TLV second", config)
		require.Equal(t, []byte{0, 0}, sid[13:15], "%s: SRGB Flags", config)
	}

	received := map[string][]byte{
		"all clear":          append(rfc8669LabelIndexTLV(0, 0, 300), rfc8669SRGBTLV(0, 800000, 4096)...),
		"reserved set":       append(rfc8669LabelIndexTLV(0xFF, 0, 300), rfc8669SRGBTLV(0, 800000, 4096)...),
		"label flags set":    append(rfc8669LabelIndexTLV(0, 0xFFFF, 300), rfc8669SRGBTLV(0, 800000, 4096)...),
		"srgb flags set":     append(rfc8669LabelIndexTLV(0, 0, 300), rfc8669SRGBTLV(0xFFFF, 800000, 4096)...),
		"every field is set": append(rfc8669LabelIndexTLV(0xFF, 0xFFFF, 300), rfc8669SRGBTLV(0xFFFF, 800000, 4096)...),
	}
	for name, value := range received {
		pathAttrs := rfc8669Update(value[:10], value[10:])
		before := bytes.Clone(pathAttrs)
		result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "%s: %s", name, result.Description)
		require.Empty(t, result.DiscardEntries, name)
		require.Equal(t, before, pathAttrs, "%s: left byte for byte", name)
		_, err := attribute.ParsePrefixSID(value)
		require.NoError(t, err, name)
	}
}
