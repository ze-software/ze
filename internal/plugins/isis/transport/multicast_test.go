// Design: docs/architecture/isis/isis-3-l2-transport.md -- multicast MAC selection tests

package transport

import "testing"

func TestMulticastMACForLevel(t *testing.T) {
	// VALIDATES: AC-5 level L1 -> AllL1ISs, L2 -> AllL2ISs; bytes exact.
	// PREVENTS: sending Hellos to the wrong ISO multicast group (silent
	// adjacency failure with a real peer).
	cases := []struct {
		name string
		lvl  Level
		want [MACLen]byte
		ok   bool
	}{
		{"l1", Level1, [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}, true},
		{"l2", Level2, [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x15}, true},
		{"none", LevelNone, [MACLen]byte{}, false},
		{"invalid", Level(7), [MACLen]byte{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MulticastMACForLevel(tc.lvl)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("mac = %x, want %x", got, tc.want)
			}
		})
	}
}

func TestMulticastConstantsExact(t *testing.T) {
	// VALIDATES: AC-2/AC-5 the ISO multicast group bytes match ISO/IEC 10589.
	if AllL1ISs != [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14} {
		t.Errorf("AllL1ISs = %x", AllL1ISs)
	}
	if AllL2ISs != [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x15} {
		t.Errorf("AllL2ISs = %x", AllL2ISs)
	}
	if AllISs != [MACLen]byte{0x09, 0x00, 0x2b, 0x00, 0x00, 0x05} {
		t.Errorf("AllISs = %x", AllISs)
	}
}

func TestIsISMulticastMAC(t *testing.T) {
	// VALIDATES: AC-1 receive path accepts all three ISO groups, rejects others.
	accept := [][MACLen]byte{AllL1ISs, AllL2ISs, AllISs}
	for _, m := range accept {
		if !IsISMulticastMAC(m) {
			t.Errorf("IsISMulticastMAC(%x) = false, want true", m)
		}
	}
	reject := [][MACLen]byte{
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // broadcast
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, // unicast
		{0x01, 0x80, 0xc2, 0x00, 0x00, 0x00}, // STP, not IS-IS
	}
	for _, m := range reject {
		if IsISMulticastMAC(m) {
			t.Errorf("IsISMulticastMAC(%x) = true, want false", m)
		}
	}
}

func TestLevelString(t *testing.T) {
	if Level1.String() != "l1" || Level2.String() != "l2" || LevelNone.String() != "none" {
		t.Errorf("level strings: %q %q %q", Level1, Level2, LevelNone)
	}
}
