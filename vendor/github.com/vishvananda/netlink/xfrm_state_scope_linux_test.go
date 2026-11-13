// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- safe state readback.
// Related: xfrm_state_linux.go -- parseXfrmState.

package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestZeXfrmStateParserRejectsTruncatedHeader calls the actual SA parser for
// every short header length, including empty replies that must not panic.
func TestZeXfrmStateParserRejectsTruncatedHeader(t *testing.T) {
	var header [nl.SizeofXfrmUsersaInfo]byte
	for size := 0; size < len(header); size++ {
		if _, err := parseXfrmState(header[:size], FAMILY_ALL); err == nil {
			t.Fatalf("accepted a %d-byte SA header", size)
		}
	}
}
