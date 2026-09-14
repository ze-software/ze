// Design: docs/architecture/mrt.md -- MRT file statistics
//
// Detail: the message-type words the MRT statistics count under are read from the
// msgtype table rather than spelled here, so this test reads the same table.

package analyze

import (
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestBGPMsgTypeNameReadsTheMessageTypeTable proves the statistics count a BGP
// message under the word the msgtype table gives its code, and under its
// number when the table gives none.
//
// PREVENTS: a message type renamed or added in msgtype that the statistics keep
// counting under an old word, so the statistics and `send bgp raw` disagree about
// what one message is called.
func TestBGPMsgTypeNameReadsTheMessageTypeTable(t *testing.T) {
	names := msgtype.TextNames()
	if len(names) == 0 {
		t.Fatal("msgtype.TextNames answered no name, so there is nothing to compare against")
	}
	for _, name := range names {
		code, ok := msgtype.FromText(name)
		if !ok {
			t.Fatalf("msgtype spells %q and FromText refuses it", name)
		}
		if got := bgpMsgTypeName(uint8(code)); got != name {
			t.Errorf("bgpMsgTypeName(%d) = %q, want the msgtype word %q", code, got, name)
		}
	}

	// A code the table does not name is counted by its number, so a malformed
	// dump is visible in the statistics rather than folded into a real type.
	for _, code := range []uint8{0, uint8(len(names) + 1), 255} {
		if _, ok := msgtype.FromText(bgpMsgTypeName(code)); ok {
			t.Fatalf("code %d is not in the msgtype table, and bgpMsgTypeName gave it a table word", code)
		}
		want := "type-" + strconv.Itoa(int(code))
		if got := bgpMsgTypeName(code); got != want {
			t.Errorf("bgpMsgTypeName(%d) = %q, want %q", code, got, want)
		}
	}
}
