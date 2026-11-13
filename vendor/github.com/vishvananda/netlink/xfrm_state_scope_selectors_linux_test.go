// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact scope wire DTO.
// Related: xfrm_selector_family_linux_test.go -- the selector family cases.

package netlink

import (
	"bytes"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestZeXfrmStateScopeWritesSelectorFamilies checks that the private scope
// writer accepts each canonical selector of the family cases and writes it
// byte for byte, so a transport selector equals its scope record.
func TestZeXfrmStateScopeWritesSelectorFamilies(t *testing.T) {
	for _, tc := range xfrmSelectorFamilyCases {
		t.Run(tc.name, func(t *testing.T) {
			want := nl.XfrmSelector{
				Family: tc.family, Saddr: tc.saddr, Daddr: tc.daddr,
				PrefixlenS: uint8(tc.prefix), PrefixlenD: uint8(tc.prefix), Proto: 17,
				Sport: 0, SportMask: nl.Swap16(0xffff), Dport: nl.Swap16(0x1200), DportMask: nl.Swap16(0xff00),
				Ifindex: 9,
			}
			scope := XfrmStateScope{Selectors: []nl.XfrmSelector{want}}
			var payload [8 + nl.SizeofXfrmSelector]byte
			if _, err := scope.WriteTo(payload[:], 0); err != nil {
				t.Fatalf("canonical scope rejected: %v", err)
			}
			if !bytes.Equal(payload[8:], want.Serialize()) {
				t.Fatalf("scope record: got %x, want %x", payload[8:], want.Serialize())
			}
		})
	}
}
