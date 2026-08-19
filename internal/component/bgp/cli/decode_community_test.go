package cli

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// buildCommunityUpdate returns a real BGP UPDATE message, header and all, that
// carries one of each community attribute plus the mandatory ORIGIN, AS_PATH
// and NEXT_HOP, and announces 10.0.1.0/24.
//
// The octets are assembled here rather than pasted as a hex literal so the
// reader can see which code carries which value. What the test feeds the
// decoder is still the wire form, which is the point: the defect these tests
// cover lives in the switch that reads the wire.
func buildCommunityUpdate(t *testing.T) string {
	t.Helper()

	attr := func(flags, code byte, value []byte) []byte {
		require.LessOrEqual(t, len(value), 255, "attribute %d needs the extended length form", code)
		return append([]byte{flags, code, byte(len(value))}, value...)
	}

	var attrs []byte
	// ORIGIN igp, AS_PATH [65001], NEXT_HOP 192.0.2.1.
	attrs = append(attrs, attr(0x40, 1, []byte{0x00})...)
	attrs = append(attrs, attr(0x40, 2, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9})...)
	attrs = append(attrs, attr(0x40, 3, []byte{192, 0, 2, 1})...)
	// COMMUNITIES (RFC 1997) 65001:100.
	attrs = append(attrs, attr(0xC0, 8, []byte{0xFD, 0xE9, 0x00, 0x64})...)
	// EXTENDED_COMMUNITIES (RFC 4360) target:100:1.
	attrs = append(attrs, attr(0xC0, 16, []byte{0x00, 0x02, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01})...)
	// IPV6_EXTENDED_COMMUNITIES (RFC 5701) route target 2001:db8::1:100.
	attrs = append(attrs, attr(0xC0, 25, []byte{
		0x00, 0x02,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x64,
	})...)
	// LARGE_COMMUNITIES (RFC 8092) 65001:1:2.
	attrs = append(attrs, attr(0xC0, 32, []byte{
		0x00, 0x00, 0xFD, 0xE9,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
	})...)

	return string(updateHex(t, attrs))
}

// updateHex wraps a path attribute block in an UPDATE body and a BGP header,
// announcing 10.0.1.0/24, and returns the whole message as hex.
func updateHex(t *testing.T, attrs []byte) []byte {
	t.Helper()

	body := []byte{0x00, 0x00} // Withdrawn Routes Length: none.
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, 24, 10, 0, 1) // NLRI 10.0.1.0/24.

	msg := make([]byte, 0, message.HeaderLen+len(body))
	for range 16 {
		msg = append(msg, 0xFF)
	}
	msg = binary.BigEndian.AppendUint16(msg, uint16(message.HeaderLen+len(body)))
	msg = append(msg, 2) // Type: UPDATE.
	msg = append(msg, body...)

	out := make([]byte, hex.EncodedLen(len(msg)))
	hex.Encode(out, msg)
	return out
}

// decodedAttributes runs one hex message through decodeHexPacket, the function
// cmdDecode calls, and returns the decoded `bgp.update.attr` object.
func decodedAttributes(t *testing.T, hexMsg string) map[string]any {
	t.Helper()

	output, err := decodeHexPacket(hexMsg, "", "", true)
	require.NoError(t, err, "decode failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result), "invalid JSON")

	bgp, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "no bgp object in %s", output)
	update, ok := bgp["update"].(map[string]any)
	require.True(t, ok, "no update object in %s", output)
	attrs, ok := update["attr"].(map[string]any)
	require.True(t, ok, "no attr object in %s", output)

	return attrs
}

// requireAttrValue returns the value inside one flag-annotated attribute object,
// and fails the test when the attribute is absent.
func requireAttrValue(t *testing.T, attrs map[string]any, key string) any {
	t.Helper()

	wrapped, ok := attrs[key].(map[string]any)
	require.True(t, ok, "attribute %q absent; decode produced %v", key, keysOf(attrs))

	value, ok := wrapped["value"]
	require.True(t, ok, "attribute %q carries no value", key)

	return value
}

func keysOf(attrs map[string]any) []string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	return keys
}

// TestDecodeUpdateNamesEveryCommunityAttribute drives an UPDATE carrying all
// four community attributes through the decode path an operator invokes.
//
// VALIDATES: renderAttributeZe files COMMUNITIES (code 8, RFC 1997),
// EXTENDED_COMMUNITIES (16, RFC 4360), IPV6_EXTENDED_COMMUNITIES (25, RFC 5701)
// and LARGE_COMMUNITIES (32, RFC 8092) under their own JSON key, each carrying
// the decoded community text.
//
// PREVENTS: an attribute code reaching no arm of the switch and no default, so
// that the attribute leaves the decode with no trace at all. Codes 8, 25 and 32
// did exactly that: an operator inspecting a capture was told nothing about a
// community the peer had sent, rather than being given its text or an error.
//
// The message is built as wire octets and fed to decodeHexPacket. A test that
// constructed the attribute structs and called a renderer would not have caught
// this, because the defect was in the switch that reads the wire.
func TestDecodeUpdateNamesEveryCommunityAttribute(t *testing.T) {
	attrs := decodedAttributes(t, buildCommunityUpdate(t))

	assert.Equal(t, []any{"65001:100"}, requireAttrValue(t, attrs, "community"))
	assert.Equal(t, []any{"000220010db80000000000000000000000010064"},
		requireAttrValue(t, attrs, "ipv6-extended-community"))
	assert.Equal(t, []any{"65001:1:2"}, requireAttrValue(t, attrs, "large-community"))

	extComms, ok := requireAttrValue(t, attrs, "extended-community").([]any)
	require.True(t, ok, "extended-community is not a list")
	require.Len(t, extComms, 1)
	extComm, ok := extComms[0].(map[string]any)
	require.True(t, ok, "extended community is not an object")
	assert.Equal(t, "target:100:1", extComm["string"])

	// The attributes that already worked keep working beside the new ones.
	assert.Equal(t, "igp", requireAttrValue(t, attrs, "origin"))
	assert.Equal(t, []any{float64(65001)}, requireAttrValue(t, attrs, "as-path"))
}

// TestDecodeUpdateGivesAnUnnamedAttributeItsOctets drives an UPDATE carrying
// AS4_PATH (code 17, RFC 6793), which this decoder does not name.
//
// VALIDATES: an attribute code with no arm in renderAttributeZe is filed under
// "attr-<code>" with its octets as hex, the same spelling appendAttributeJSON
// (internal/component/bgp/format/text_json.go) gives it on the receive path.
//
// PREVENTS: the next unnamed attribute code repeating the community defect.
// Adding three cases would have fixed three codes; the default arm is what
// stops a fourth from vanishing in silence.
func TestDecodeUpdateGivesAnUnnamedAttributeItsOctets(t *testing.T) {
	// ORIGIN igp, so the update is well formed, then AS4_PATH.
	attrs := []byte{
		0x40, 1, 1, 0x00,
		0xC0, 17, 6, 0x02, 0x01, 0x00, 0x01, 0x00, 0x02,
	}

	decoded := decodedAttributes(t, string(updateHex(t, attrs)))

	assert.Equal(t, "020100010002", requireAttrValue(t, decoded, "attr-17"))
}

// TestDecodeUpdateGivesAMalformedCommunityAttributeItsOctets drives an UPDATE
// whose COMMUNITIES attribute is not a whole number of communities.
//
// VALIDATES: parseCommunities refuses a length that is not a multiple of four
// (RFC 1997), and renderAttributeZe then files the attribute under its raw
// form rather than under a truncated community list.
//
// PREVENTS: a malformed attribute reaching the operator as a shorter, valid
// looking list, or as nothing at all. Half a community names nothing, and a
// silently dropped octet is the failure that made this decoder unreliable.
func TestDecodeUpdateGivesAMalformedCommunityAttributeItsOctets(t *testing.T) {
	// ORIGIN igp, then COMMUNITIES with five octets: one community and a stray.
	attrs := []byte{
		0x40, 1, 1, 0x00,
		0xC0, 8, 5, 0xFD, 0xE9, 0x00, 0x64, 0xFF,
	}

	decoded := decodedAttributes(t, string(updateHex(t, attrs)))

	assert.Equal(t, "fde90064ff", requireAttrValue(t, decoded, "attr-8"))
	assert.NotContains(t, decoded, "community")
}

// TestFormatUpdateHumanNamesEveryCommunityAttribute checks the default output
// mode, which is what an operator gets without --json.
//
// VALIDATES: formatAttributesHuman writes a line for each of the four
// community attributes, reading the Go types renderAttributeZe puts in the map.
//
// PREVENTS: the JSON path being fixed while the default path stays silent. The
// human formatter asserted []any on both community keys, and neither producer
// ever returned one, so every community was dropped there too.
func TestFormatUpdateHumanNamesEveryCommunityAttribute(t *testing.T) {
	output, err := decodeHexPacket(buildCommunityUpdate(t), "", "", false)
	require.NoError(t, err, "decode failed")

	assert.Contains(t, output, "community")
	assert.Contains(t, output, "65001:100")
	assert.Contains(t, output, "target:100:1")
	assert.Contains(t, output, "000220010db80000000000000000000000010064")
	assert.Contains(t, output, "65001:1:2")
}
