// Design: plan/learned/1007-cp-survival-3-egress-cs6-sched.md -- DSCP filter selector tests

//go:build linux

package trafficnetlink

import (
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/traffic"
)

func TestTranslateFilterDSCPHasSelector(t *testing.T) {
	f := traffic.TrafficFilter{Type: traffic.FilterDSCP, Value: 48} // CS6
	filters, err := translateFilter(f, 5, 0x10000, 0x10001)
	if err != nil {
		t.Fatalf("translateFilter: %v", err)
	}
	if len(filters) == 0 {
		t.Fatal("translateFilter returned no filters for FilterDSCP")
	}

	var foundV4 bool
	for _, filter := range filters {
		u32, ok := filter.(*netlink.U32)
		if !ok {
			continue
		}
		if u32.Attrs().Protocol != 0x0800 {
			continue
		}
		foundV4 = true
		if u32.Sel == nil {
			t.Fatal("IPv4 DSCP filter has nil Sel (the bug: no u32 match selector)")
		}
		if len(u32.Sel.Keys) == 0 {
			t.Fatal("IPv4 DSCP filter Sel has no keys")
		}
		key := u32.Sel.Keys[0]
		// CS6 = 48, TOS byte = 48<<2 = 0xC0. In big-endian 32-bit at offset 0:
		// Val = 0x00C00000, Mask = 0x00FC0000 (top 6 bits of TOS byte)
		wantVal := uint32(0x00C00000)
		wantMask := uint32(0x00FC0000)
		if key.Val != wantVal {
			t.Errorf("IPv4 key.Val = 0x%08X, want 0x%08X", key.Val, wantVal)
		}
		if key.Mask != wantMask {
			t.Errorf("IPv4 key.Mask = 0x%08X, want 0x%08X", key.Mask, wantMask)
		}
		if key.Off != 0 {
			t.Errorf("IPv4 key.Off = %d, want 0", key.Off)
		}
		if u32.ClassId != 0x10001 {
			t.Errorf("IPv4 ClassId = 0x%X, want 0x10001", u32.ClassId)
		}
	}
	if !foundV4 {
		t.Fatal("no IPv4 (ETH_P_IP 0x0800) DSCP filter found")
	}
}

func TestTranslateFilterDSCPv6Selector(t *testing.T) {
	f := traffic.TrafficFilter{Type: traffic.FilterDSCP, Value: 48} // CS6
	filters, err := translateFilter(f, 5, 0x10000, 0x10001)
	if err != nil {
		t.Fatalf("translateFilter: %v", err)
	}

	var foundV6 bool
	for _, filter := range filters {
		u32, ok := filter.(*netlink.U32)
		if !ok {
			continue
		}
		if u32.Attrs().Protocol != 0x86DD {
			continue
		}
		foundV6 = true
		if u32.Sel == nil {
			t.Fatal("IPv6 DSCP filter has nil Sel")
		}
		if len(u32.Sel.Keys) == 0 {
			t.Fatal("IPv6 DSCP filter Sel has no keys")
		}
		key := u32.Sel.Keys[0]
		// CS6 = 48. In IPv6 traffic class at offset 0:
		// DSCP bits are at positions 27-22 of the 32-bit word.
		// Val = 48 << 22 = 0x0C000000, Mask = 0x0FC00000
		wantVal := uint32(0x0C000000)
		wantMask := uint32(0x0FC00000)
		if key.Val != wantVal {
			t.Errorf("IPv6 key.Val = 0x%08X, want 0x%08X", key.Val, wantVal)
		}
		if key.Mask != wantMask {
			t.Errorf("IPv6 key.Mask = 0x%08X, want 0x%08X", key.Mask, wantMask)
		}
		if key.Off != 0 {
			t.Errorf("IPv6 key.Off = %d, want 0", key.Off)
		}
		if u32.ClassId != 0x10001 {
			t.Errorf("IPv6 ClassId = 0x%X, want 0x10001", u32.ClassId)
		}
	}
	if !foundV6 {
		t.Fatal("no IPv6 (ETH_P_IPV6 0x86DD) DSCP filter found")
	}
}

func TestTranslateFilterDSCPBoundary(t *testing.T) {
	tests := []struct {
		name    string
		dscp    uint32
		wantV4  uint32
		wantErr bool
	}{
		{"BE(0)", 0, 0x00000000, false},
		{"CS6(48)", 48, 0x00C00000, false},
		{"EF(46)", 46, 0x00B80000, false},
		{"max(63)", 63, 0x00FC0000, false},
		{"invalid(64)", 64, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := traffic.TrafficFilter{Type: traffic.FilterDSCP, Value: tt.dscp}
			filters, err := translateFilter(f, 5, 0x10000, 0x10001)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error for out-of-range DSCP, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("translateFilter: %v", err)
			}
			for _, filter := range filters {
				u32, ok := filter.(*netlink.U32)
				if !ok || u32.Attrs().Protocol != 0x0800 {
					continue
				}
				if u32.Sel == nil || len(u32.Sel.Keys) == 0 {
					t.Fatal("missing selector")
				}
				if got := u32.Sel.Keys[0].Val; got != tt.wantV4 {
					t.Errorf("Val = 0x%08X, want 0x%08X", got, tt.wantV4)
				}
			}
		})
	}
}

func TestTranslateFilterMarkUnchanged(t *testing.T) {
	f := traffic.TrafficFilter{Type: traffic.FilterMark, Value: 0x10}
	filters, err := translateFilter(f, 5, 0x10000, 0x10001)
	if err != nil {
		t.Fatalf("translateFilter: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("got %d filters, want 1", len(filters))
	}
	fw, ok := filters[0].(*netlink.FwFilter)
	if !ok {
		t.Fatalf("got %T, want *netlink.FwFilter", filters[0])
	}
	if fw.ClassId != 0x10001 {
		t.Errorf("ClassId = 0x%X, want 0x10001", fw.ClassId)
	}
	if fw.Attrs().Handle != 0x10 {
		t.Errorf("Handle = 0x%X, want 0x10", fw.Attrs().Handle)
	}
}

func TestTranslateFilterProtocolHasSelector(t *testing.T) {
	f := traffic.TrafficFilter{Type: traffic.FilterProtocol, Value: 6} // TCP
	filters, err := translateFilter(f, 5, 0x10000, 0x10001)
	if err != nil {
		t.Fatalf("translateFilter: %v", err)
	}
	if len(filters) == 0 {
		t.Fatal("translateFilter returned no filters for FilterProtocol")
	}

	var foundV4 bool
	for _, filter := range filters {
		u32, ok := filter.(*netlink.U32)
		if !ok || u32.Attrs().Protocol != 0x0800 {
			continue
		}
		foundV4 = true
		if u32.Sel == nil {
			t.Fatal("IPv4 protocol filter has nil Sel")
		}
		if len(u32.Sel.Keys) == 0 {
			t.Fatal("IPv4 protocol filter Sel has no keys")
		}
		key := u32.Sel.Keys[0]
		// TCP = 6. Protocol byte at IPv4 header offset 8, byte 1:
		// Val = 0x00060000, Mask = 0x00FF0000
		if key.Val != 0x00060000 {
			t.Errorf("key.Val = 0x%08X, want 0x00060000", key.Val)
		}
		if key.Mask != 0x00FF0000 {
			t.Errorf("key.Mask = 0x%08X, want 0x00FF0000", key.Mask)
		}
		if key.Off != 8 {
			t.Errorf("key.Off = %d, want 8", key.Off)
		}
	}
	if !foundV4 {
		t.Fatal("no IPv4 protocol filter found")
	}
}

func TestTranslateFilterProtocolRejectsOutOfRange(t *testing.T) {
	f := traffic.TrafficFilter{Type: traffic.FilterProtocol, Value: 256}
	_, err := translateFilter(f, 5, 0x10000, 0x10001)
	if err == nil {
		t.Fatal("want error for protocol value 256, got nil")
	}
}
