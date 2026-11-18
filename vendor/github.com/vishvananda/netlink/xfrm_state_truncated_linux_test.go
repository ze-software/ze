package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmStateParseTruncatedHeader hands the state parser every length short
// of a complete xfrm_usersa_info, the empty reply included, and then a complete one.
func TestXfrmStateParseTruncatedHeader(t *testing.T) {
	var header [nl.SizeofXfrmUsersaInfo]byte
	for size := 0; size < len(header); size++ {
		if _, err := parseXfrmState(header[:size], FAMILY_ALL); err == nil {
			t.Fatalf("accepted a %d-byte state header", size)
		}
	}
	if _, err := parseXfrmState(header[:], FAMILY_ALL); err != nil {
		t.Fatalf("refused a complete state header: %v", err)
	}
}
