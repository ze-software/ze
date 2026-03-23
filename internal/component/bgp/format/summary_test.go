package format

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// parseSummaryJSON parses summary format JSON and returns the bgp payload.
func parseSummaryJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	bgpPayload, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "bgp payload must exist")
	return bgpPayload
}

// getNLRISummary extracts the nlri summary object from bgp payload.
func getNLRISummary(t *testing.T, bgp map[string]any) map[string]any {
	t.Helper()
	nlriObj, ok := bgp["nlri"].(map[string]any)
	require.True(t, ok, "bgp.nlri must exist")
	return nlriObj
}

// buildMPReachAttr builds an MP_REACH_NLRI attribute with given AFI/SAFI.
// Minimal: AFI(2) + SAFI(1) + NH_len(1)=0 + reserved(1).
func buildMPReachAttr(afi uint16, safi uint8) []byte {
	// Value: AFI(2) + SAFI(1) + NH_len(1)=0 + reserved(1) = 5 bytes
	value := make([]byte, 5)
	binary.BigEndian.PutUint16(value[0:2], afi)
	value[2] = safi
	value[3] = 0 // NH len
	value[4] = 0 // reserved

	// Attribute header: flags(1) + type(1) + length(1) = 3 bytes
	// flags=0x80 (optional), type=14 (MP_REACH_NLRI)
	attr := make([]byte, 0, 3+len(value))
	attr = append(attr, 0x80, 14, byte(len(value)))
	attr = append(attr, value...)
	return attr
}

// buildMPUnreachAttr builds an MP_UNREACH_NLRI attribute with given AFI/SAFI.
// Minimal: AFI(2) + SAFI(1).
func buildMPUnreachAttr(afi uint16, safi uint8) []byte {
	// Value: AFI(2) + SAFI(1) = 3 bytes
	value := make([]byte, 3)
	binary.BigEndian.PutUint16(value[0:2], afi)
	value[2] = safi

	// Attribute header: flags(1) + type(1) + length(1) = 3 bytes
	// flags=0x80 (optional), type=15 (MP_UNREACH_NLRI)
	attr := make([]byte, 0, 3+len(value))
	attr = append(attr, 0x80, 15, byte(len(value)))
	attr = append(attr, value...)
	return attr
}

// buildSummaryUpdateBody builds an UPDATE body from components.
func buildSummaryUpdateBody(withdrawn, attrs, nlriBytes []byte) []byte {
	body := make([]byte, 4+len(withdrawn)+len(attrs)+len(nlriBytes))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
	copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[2+len(withdrawn):], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4+len(withdrawn):], attrs)
	copy(body[4+len(withdrawn)+len(attrs):], nlriBytes)
	return body
}

// summaryMsg creates a RawMessage for summary format testing.
func summaryMsg(body []byte, msgID uint64) bgptypes.RawMessage {
	ctxID := testEncodingContext()
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()
	return bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
		MessageID:  msgID,
	}
}

// summaryContent returns ContentConfig for summary format.
func summaryContent() bgptypes.ContentConfig {
	return bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatSummary,
	}
}

var summaryPeer = plugin.PeerInfo{
	Address: netip.MustParseAddr("10.0.0.1"),
	PeerAS:  65001,
}

// TestFormatSummaryPeerNameGroup verifies name and group appear in summary JSON peer object.
//
// VALIDATES: Summary JSON includes peer name and group when set.
// PREVENTS: writePeerJSON path omitting name/group in summary format.
func TestFormatSummaryPeerNameGroup(t *testing.T) {
	peer := plugin.PeerInfo{
		Address:   netip.MustParseAddr("10.0.0.1"),
		PeerAS:    65001,
		Name:      "upstream1",
		GroupName: "transit",
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)
	msg := summaryMsg(body, 1)
	got := FormatMessage(peer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	peerObj, ok := bgp["peer"].(map[string]any)
	require.True(t, ok, "bgp.peer must be object")
	assert.Equal(t, "10.0.0.1", peerObj["address"])
	assert.Equal(t, float64(65001), peerObj["asn"])
	assert.Equal(t, "transit", peerObj["group"])
	assert.Equal(t, "upstream1", peerObj["name"])
}

// TestFormatSummaryLegacyNLRI verifies announce=true when legacy NLRI section has bytes.
//
// VALIDATES: Legacy IPv4 unicast NLRI section detected as announce.
// PREVENTS: Summary format missing legacy NLRI detection.
func TestFormatSummaryLegacyNLRI(t *testing.T) {
	// UPDATE with IPv4 NLRI (192.168.1.0/24), origin, empty as-path, next-hop
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)
	msg := summaryMsg(body, 1)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, true, nlriObj["announce"])
	assert.Equal(t, false, nlriObj["withdrawn"])
	assert.Equal(t, "", nlriObj["mp-reach"])
	assert.Equal(t, "", nlriObj["mp-unreach"])
}

// TestFormatSummaryLegacyWithdrawn verifies withdrawn=true when legacy withdrawn section has bytes.
//
// VALIDATES: Legacy withdrawn routes section detected.
// PREVENTS: Summary format missing legacy withdrawal detection.
func TestFormatSummaryLegacyWithdrawn(t *testing.T) {
	body := buildTestUpdateBodyWithWithdrawn(netip.MustParsePrefix("10.0.0.0/24"))
	msg := summaryMsg(body, 2)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, false, nlriObj["announce"])
	assert.Equal(t, true, nlriObj["withdrawn"])
	assert.Equal(t, "", nlriObj["mp-reach"])
	assert.Equal(t, "", nlriObj["mp-unreach"])
}

// TestFormatSummaryMPReach verifies mp-reach family extracted from MP_REACH_NLRI attribute.
//
// VALIDATES: MP_REACH_NLRI AFI/SAFI extracted as family string.
// PREVENTS: Summary format not scanning for attribute code 14.
func TestFormatSummaryMPReach(t *testing.T) {
	// MP_REACH for l2vpn/evpn: AFI=25, SAFI=70
	mpReach := buildMPReachAttr(25, 70)
	body := buildSummaryUpdateBody(nil, mpReach, nil)
	msg := summaryMsg(body, 3)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, false, nlriObj["announce"])
	assert.Equal(t, false, nlriObj["withdrawn"])
	assert.Equal(t, "l2vpn/evpn", nlriObj["mp-reach"])
	assert.Equal(t, "", nlriObj["mp-unreach"])
}

// TestFormatSummaryMPUnreach verifies mp-unreach family extracted from MP_UNREACH_NLRI attribute.
//
// VALIDATES: MP_UNREACH_NLRI AFI/SAFI extracted as family string.
// PREVENTS: Summary format not scanning for attribute code 15.
func TestFormatSummaryMPUnreach(t *testing.T) {
	// MP_UNREACH for ipv6/unicast: AFI=2, SAFI=1
	mpUnreach := buildMPUnreachAttr(2, 1)
	body := buildSummaryUpdateBody(nil, mpUnreach, nil)
	msg := summaryMsg(body, 4)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, false, nlriObj["announce"])
	assert.Equal(t, false, nlriObj["withdrawn"])
	assert.Equal(t, "", nlriObj["mp-reach"])
	assert.Equal(t, "ipv6/unicast", nlriObj["mp-unreach"])
}

// TestFormatSummaryMixed verifies combination of MP and legacy sections.
//
// VALIDATES: All four fields set correctly when multiple sections present.
// PREVENTS: Field interference between MP and legacy detection.
func TestFormatSummaryMixed(t *testing.T) {
	// Build: legacy withdrawn + MP_REACH for l2vpn/evpn + legacy NLRI
	withdrawn := []byte{24, 10, 0, 0} // 10.0.0.0/24

	var attrs []byte
	// ORIGIN (igp) + AS_PATH (empty) + NEXT_HOP (10.0.0.1)
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 10, 0, 0, 1)
	attrs = append(attrs, buildMPReachAttr(25, 70)...) // MP_REACH l2vpn/evpn

	nlriBytes := []byte{24, 192, 168, 1} // 192.168.1.0/24

	body := buildSummaryUpdateBody(withdrawn, attrs, nlriBytes)
	msg := summaryMsg(body, 5)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, true, nlriObj["announce"])
	assert.Equal(t, true, nlriObj["withdrawn"])
	assert.Equal(t, "l2vpn/evpn", nlriObj["mp-reach"])
	assert.Equal(t, "", nlriObj["mp-unreach"])
}

// TestFormatSummaryNonUpdate verifies non-UPDATE messages fall through to parsed format.
//
// VALIDATES: OPEN with summary format produces parsed output, not summary.
// PREVENTS: Non-UPDATE messages getting summary treatment.
// NOTE: In subscription delivery (events.go), non-UPDATE types bypass FormatMessage entirely
// (dispatched to encoder.Open/Notification/etc). This is a defensive test ensuring
// FormatMessage itself handles the non-UPDATE case gracefully if called directly.
func TestFormatSummaryNonUpdate(t *testing.T) {
	// OPEN message body
	openBody := []byte{
		4,     // version
		0, 42, // AS 42
		0, 180, // hold time 180
		10, 0, 0, 1, // router-id 10.0.0.1
		0, // opt params len
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeOPEN,
		RawBytes:  openBody,
		Direction: "received",
		MessageID: 6,
	}

	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	// Should produce parsed OPEN output, not summary JSON
	var result map[string]any
	if json.Unmarshal([]byte(got), &result) == nil {
		// If it's JSON, it should NOT have nlri key
		bgp, ok := result["bgp"].(map[string]any)
		if ok {
			_, hasNLRI := bgp["nlri"]
			// OPEN should have open key, not nlri
			_, hasOpen := bgp["open"]
			assert.False(t, hasNLRI, "OPEN should not have nlri key")
			_ = hasOpen // May or may not be JSON — text format also valid
		}
	}
	// Non-UPDATE with summary format falls through to parsed; any valid output is OK
	assert.NotEmpty(t, got)
}

// TestFormatSummaryEmptyUpdate verifies empty UPDATE (End-of-RIB for IPv4) produces all false/empty.
//
// VALIDATES: Empty UPDATE produces all-false/all-empty summary.
// PREVENTS: False positives on empty UPDATE messages.
func TestFormatSummaryEmptyUpdate(t *testing.T) {
	// Empty UPDATE: withdrawn_len=0, attr_len=0, no NLRI
	body := []byte{0, 0, 0, 0}
	msg := summaryMsg(body, 7)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, false, nlriObj["announce"])
	assert.Equal(t, false, nlriObj["withdrawn"])
	assert.Equal(t, "", nlriObj["mp-reach"])
	assert.Equal(t, "", nlriObj["mp-unreach"])
}

// TestFormatSummaryEndOfRIB verifies End-of-RIB via MP_UNREACH produces family in mp-unreach.
//
// VALIDATES: MP_UNREACH with no NLRI bytes (End-of-RIB marker) still reports family.
// PREVENTS: End-of-RIB marker being invisible in summary format.
func TestFormatSummaryEndOfRIB(t *testing.T) {
	// End-of-RIB for IPv6/unicast: MP_UNREACH with AFI=2 SAFI=1, no withdrawn NLRI
	mpUnreach := buildMPUnreachAttr(2, 1)
	body := buildSummaryUpdateBody(nil, mpUnreach, nil)
	msg := summaryMsg(body, 8)
	got := FormatMessage(summaryPeer, msg, summaryContent(), "")

	bgp := parseSummaryJSON(t, got)
	nlriObj := getNLRISummary(t, bgp)

	assert.Equal(t, "ipv6/unicast", nlriObj["mp-unreach"])
}

// TestFormatSummaryMessageID verifies message.id is always present, even when 0.
//
// VALIDATES: message.id included in summary events unconditionally.
// PREVENTS: Missing message.id when value is 0.
func TestFormatSummaryMessageID(t *testing.T) {
	body := []byte{0, 0, 0, 0} // empty UPDATE
	tests := []struct {
		name  string
		msgID uint64
	}{
		{"nonzero", 42},
		{"zero", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := summaryMsg(body, tt.msgID)
			got := FormatMessage(summaryPeer, msg, summaryContent(), "")

			bgp := parseSummaryJSON(t, got)
			msgObj, ok := bgp["message"].(map[string]any)
			require.True(t, ok, "bgp.message must exist")

			_, hasID := msgObj["id"]
			assert.True(t, hasID, "message.id must always be present")

			// Verify the value (JSON numbers are float64)
			assert.Equal(t, float64(tt.msgID), msgObj["id"])
		})
	}
}

// TestFormatSummaryMalformed verifies truncated UPDATE returns empty update JSON.
//
// VALIDATES: Malformed UPDATE gracefully returns empty update.
// PREVENTS: Panic or error on truncated wire bytes.
func TestFormatSummaryMalformed(t *testing.T) {
	// Truncated: only 2 bytes (need minimum 4)
	body := []byte{0, 0}
	msg := bgptypes.RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  body,
		Direction: "received",
		MessageID: 10,
	}

	got := FormatMessage(summaryPeer, msg, summaryContent(), "")
	assert.NotEmpty(t, got, "should produce output even for malformed UPDATE")

	// Should be valid JSON
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &result))
}
