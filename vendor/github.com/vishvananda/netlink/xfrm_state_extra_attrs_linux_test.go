package netlink

import (
	"bytes"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestXfrmStateExtraAttributes checks that the state writer appends each extra
// attribute unchanged and in order, and that the state reader returns an
// attribute it has no definition for, with its own copy of the value, while it
// keeps an attribute it defines out of ExtraAttrs.
func TestXfrmStateExtraAttributes(t *testing.T) {
	first := uint16(nl.XFRMA_MAX + 8)
	extra := []XfrmAttr{
		{Type: first, Value: []byte{1, 2, 3, 4, 5}},
		{Type: first + 1, Value: []byte{6, 7, 8, 9}},
	}
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_NEWSA, 0)
	addXfrmExtraAttrs(req, extra)
	// Each attribute is a u16 length that excludes padding and a u16 type,
	// then its value padded to four bytes.
	want := make([]byte, 20)
	native.PutUint16(want[0:], 4+5)
	native.PutUint16(want[2:], first)
	copy(want[4:], []byte{1, 2, 3, 4, 5})
	native.PutUint16(want[12:], 4+4)
	native.PutUint16(want[14:], first+1)
	copy(want[16:], []byte{6, 7, 8, 9})
	if got := req.Serialize()[unix.SizeofNlMsghdr:]; !bytes.Equal(got, want) {
		t.Fatalf("request attributes = %x, want %x", got, want)
	}

	msg := nl.XfrmUsersaInfo{Family: nl.FAMILY_V4}
	m := append(msg.Serialize(), nl.NewRtAttr(int(first), []byte{1, 2, 3, 4, 5}).Serialize()...)
	m = append(m, nl.NewRtAttr(nl.XFRMA_LTIME_VAL, make([]byte, nl.SizeofXfrmLifetimeCur)).Serialize()...)
	state, err := parseXfrmState(m, FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.ExtraAttrs) != 1 {
		t.Fatalf("ExtraAttrs = %+v, want only attribute %d", state.ExtraAttrs, first)
	}
	if state.ExtraAttrs[0].Type != first || !bytes.Equal(state.ExtraAttrs[0].Value, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("ExtraAttrs[0] = %+v, want attribute %d with value 0102030405", state.ExtraAttrs[0], first)
	}
	for i := range m {
		m[i] = 0
	}
	if !bytes.Equal(state.ExtraAttrs[0].Value, []byte{1, 2, 3, 4, 5}) {
		t.Fatal("ExtraAttrs shares the message buffer")
	}
}
