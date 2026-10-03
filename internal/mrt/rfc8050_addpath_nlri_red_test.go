package mrt_test

// RFC naming: untagged -- a red defect probe: ParseBGPMessage ignores the add-path BGP4MP subtype, so the requirement is unmet until the fix lands.

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype proves that the UPDATE in
// an add-path BGP4MP record is parsed with the Path Identifier its subtype
// signals.
//
// Method: one BGP4MP_MESSAGE_AS4_ADDPATH record (subtype 9) carries an UPDATE
// whose NLRI is Path Identifier 1 then 10.0.0.0/24. The record is read through
// ReadFrom and its message parsed through ParseBGPMessage, the path `le mrt
// show` and the content filter take. The announced prefixes must be exactly
// 10.0.0.0/24.
//
// Untagged and red on purpose (RFC 8050 Section 2, "In order to parse BGP
// messages that contain data structures that depend on the capabilities
// negotiated during the BGP session setup, the MRT subtypes are utilized."):
// ParseBGPMessage takes no subtype and parses the NLRI, MP_REACH_NLRI and
// MP_UNREACH_NLRI with addPath false, so the Path Identifier octets are read
// as prefixes and 0.0.0.0/0 is reported. The fix threads the subtype's
// add-path signal (IsAddPathBGP4MPSubtype) into ParseBGPMessage, ParseMPReach
// and ParseMPUnreach; then this unit is tagged RFC8050-x-4 positive.
func TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype(t *testing.T) {
	_, record := readRFC8050Message(t, rfc8050BGP4MPRecord(mrt.BGP4MPMessageAS4AP, rfc8050AS4Fields))

	parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
	if err != nil {
		t.Fatalf("ParseBGPMessage: %v", err)
	}
	if parsed.Update == nil {
		t.Fatal("ParseBGPMessage returned no UPDATE")
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	if !slices.Equal(parsed.Update.AnnouncedPrefixes, want) {
		t.Errorf("announced prefixes = %v, want %v", parsed.Update.AnnouncedPrefixes, want)
	}
}
