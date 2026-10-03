package mrt_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype proves that the UPDATE in
// an add-path BGP4MP record is parsed with the Path Identifier its subtype
// signals, with no OPEN to say so.
//
// Method: one BGP4MP_MESSAGE_AS4_ADDPATH record (subtype 9) carries an UPDATE
// whose NLRI is Path Identifier 1 then 10.0.0.0/24. The record is read through
// ReadFrom and its message parsed through ParseBGPMessage, the path `le mrt
// show` and the content filter take. The announced prefixes must be exactly
// 10.0.0.0/24: a parser that ignored the subtype would read the Path
// Identifier octets as prefixes.
//
// RFC requirement: RFC8050-x-4 positive -- a BGP4MP_MESSAGE_AS4_ADDPATH record read through ReadFrom and ParseBGPMessage, with no OPEN, announces exactly 10.0.0.0/24 from NLRI holding Path Identifier 1 then that prefix.
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
