package flowexport

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func TestIfDirectionFor(t *testing.T) {
	cases := map[string]uint32{
		"full":    IfDirectionFullDuplex,
		"half":    IfDirectionHalfDuplex,
		"":        IfDirectionUnknown,
		"unknown": IfDirectionUnknown,
	}
	for in, want := range cases {
		if got := ifDirectionFor(in); got != want {
			t.Errorf("ifDirectionFor(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestInterfaceCountersFromExtended verifies the extended sFlow if_counters
// fields: rx-multicast maps to IfInMulticastPkts, the sysfs Mbit/s speed scales
// to bit/s, and the duplex string maps to ifDirection. Guards the regression
// where these fields were left zero.
func TestInterfaceCountersFromExtended(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:    "eth0",
		Index:   3,
		Type:    "device",
		State:   "up",
		Promisc: true,
		Stats: &iface.InterfaceStats{
			RxBytes:     1000,
			RxPackets:   10,
			RxMulticast: 4,
			TxBytes:     2000,
			TxPackets:   20,
		},
	}

	ic := interfaceCountersFrom(info, 10000 /* Mbit/s */, "full")

	if ic.IfSpeed != 10_000_000_000 {
		t.Errorf("IfSpeed = %d, want 10000000000 (10 Gbit/s in bit/s)", ic.IfSpeed)
	}
	if ic.IfDirection != IfDirectionFullDuplex {
		t.Errorf("IfDirection = %d, want %d", ic.IfDirection, IfDirectionFullDuplex)
	}
	if ic.IfInMulticastPkts != 4 {
		t.Errorf("IfInMulticastPkts = %d, want 4", ic.IfInMulticastPkts)
	}
	if ic.IfType != 6 {
		t.Errorf("IfType = %d, want 6 (ethernetCsmacd)", ic.IfType)
	}
	if ic.IfStatus != IfStatusAdminUp|IfStatusOperUp {
		t.Errorf("IfStatus = %d, want %d (up)", ic.IfStatus, IfStatusAdminUp|IfStatusOperUp)
	}
	if ic.IfPromiscuousMode != 1 {
		t.Errorf("IfPromiscuousMode = %d, want 1", ic.IfPromiscuousMode)
	}
}

// TestInterfaceCountersFromUnknownSpeed verifies a virtual / down link whose
// sysfs speed and duplex are unknown leaves ifSpeed and ifDirection zero rather
// than reporting a bogus rate.
func TestInterfaceCountersFromUnknownSpeed(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:  "veth0",
		Index: 9,
		Type:  "veth",
		State: "down",
		Stats: &iface.InterfaceStats{},
	}
	ic := interfaceCountersFrom(info, 0 /* unknown speed */, "" /* unknown duplex */)
	if ic.IfSpeed != 0 {
		t.Errorf("IfSpeed = %d, want 0 for unknown speed", ic.IfSpeed)
	}
	if ic.IfDirection != IfDirectionUnknown {
		t.Errorf("IfDirection = %d, want %d (unknown)", ic.IfDirection, IfDirectionUnknown)
	}
	if ic.IfStatus != IfStatusAdminUp {
		t.Errorf("IfStatus = %d, want %d (admin up, oper down)", ic.IfStatus, IfStatusAdminUp)
	}
}

func TestIfTypeFor(t *testing.T) {
	cases := map[string]uint32{
		"device":    6,
		"bridge":    209,
		"vlan":      135,
		"veth":      53,
		"dummy":     53,
		"gre":       131,
		"wireguard": 131,
		"sit":       131,
		"":          1,
		"weirdkind": 1,
	}
	for in, want := range cases {
		if got := ifTypeFor(in); got != want {
			t.Errorf("ifTypeFor(%q) = %d, want %d", in, got, want)
		}
	}
}
