package netlink

import (
	"bytes"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// struct xfrm_selector carries dport, dport_mask, sport and sport_mask as big
// endian u16 at these offsets.
const (
	xfrmSelDport     = 32
	xfrmSelDportMask = 34
	xfrmSelSport     = 36
	xfrmSelSportMask = 38
)

// TestXfrmSelectorPortMaskWrite checks the selector port masks the writer sends.
func TestXfrmSelectorPortMaskWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy XfrmPolicy
		dport  []byte
		dmask  []byte
		sport  []byte
		smask  []byte
	}{
		{
			name:   "exact port zero and a partial mask",
			policy: XfrmPolicy{Proto: 17, SrcPort: 0, SrcPortMask: 0xffff, DstPort: 0x1200, DstPortMask: 0xff00},
			dport:  []byte{0x12, 0x00},
			dmask:  []byte{0xff, 0x00},
			sport:  []byte{0x00, 0x00},
			smask:  []byte{0xff, 0xff},
		},
		{
			name:   "zero masks keep the derived ones",
			policy: XfrmPolicy{Proto: 17, SrcPort: 0, DstPort: 500},
			dport:  []byte{0x01, 0xf4},
			dmask:  []byte{0xff, 0xff},
			sport:  []byte{0x00, 0x00},
			smask:  []byte{0x00, 0x00},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sel nl.XfrmSelector
			selFromPolicy(&sel, &tc.policy)
			b := sel.Serialize()
			for _, field := range []struct {
				name string
				off  int
				want []byte
			}{
				{"dport", xfrmSelDport, tc.dport},
				{"dport_mask", xfrmSelDportMask, tc.dmask},
				{"sport", xfrmSelSport, tc.sport},
				{"sport_mask", xfrmSelSportMask, tc.smask},
			} {
				if got := b[field.off : field.off+2]; !bytes.Equal(got, field.want) {
					t.Errorf("%s = %x, want %x", field.name, got, field.want)
				}
			}
		})
	}
}

// TestXfrmSelectorPortMaskRead checks that a policy and a state's selector
// report the port masks the kernel holds.
func TestXfrmSelectorPortMaskRead(t *testing.T) {
	info := nl.XfrmUserpolicyInfo{}
	m := append([]byte(nil), info.Serialize()...)
	copy(m[xfrmSelDportMask:], []byte{0xff, 0x00})
	copy(m[xfrmSelSportMask:], []byte{0xff, 0xff})
	policy, err := parseXfrmPolicy(m, FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if policy.DstPortMask != 0xff00 || policy.SrcPortMask != 0xffff {
		t.Errorf("policy masks = %#x, %#x, want 0xff00, 0xffff", policy.DstPortMask, policy.SrcPortMask)
	}

	msg := nl.XfrmUsersaInfo{}
	m = append([]byte(nil), msg.Serialize()...)
	copy(m[xfrmSelDportMask:], []byte{0xff, 0x00})
	copy(m[xfrmSelSportMask:], []byte{0xff, 0xff})
	state, err := parseXfrmState(m, FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if state.Selector.DstPortMask != 0xff00 || state.Selector.SrcPortMask != 0xffff {
		t.Errorf("state selector masks = %#x, %#x, want 0xff00, 0xffff", state.Selector.DstPortMask, state.Selector.SrcPortMask)
	}
}
