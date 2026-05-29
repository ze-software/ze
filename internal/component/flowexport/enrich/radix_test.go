package enrich

import (
	"net/netip"
	"testing"
)

func TestRadixTreeLookup(t *testing.T) {
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{AS: 100})
	tree.Insert(netip.MustParsePrefix("10.1.0.0/16"), ASEntry{AS: 200})
	tree.Insert(netip.MustParsePrefix("192.168.1.0/24"), ASEntry{AS: 300})

	tests := []struct {
		addr   string
		wantAS uint32
		wantOK bool
	}{
		{"10.2.3.4", 100, true},
		{"10.1.2.3", 200, true},
		{"10.1.0.1", 200, true},
		{"192.168.1.50", 300, true},
		{"192.168.2.1", 0, false},
		{"172.16.0.1", 0, false},
	}

	for _, tc := range tests {
		addr := netip.MustParseAddr(tc.addr)
		entry, ok := tree.Lookup(addr)
		if ok != tc.wantOK {
			t.Errorf("Lookup(%s): ok=%v, want %v", tc.addr, ok, tc.wantOK)
			continue
		}
		if ok && entry.AS != tc.wantAS {
			t.Errorf("Lookup(%s): AS=%d, want %d", tc.addr, entry.AS, tc.wantAS)
		}
	}
}

func TestRadixTreeNoMatch(t *testing.T) {
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{AS: 100})

	_, ok := tree.Lookup(netip.MustParseAddr("172.16.0.1"))
	if ok {
		t.Error("expected no match for 172.16.0.1")
	}

	_, ok = tree.Lookup(netip.MustParseAddr("::1"))
	if ok {
		t.Error("expected no match for ::1 in empty v6 tree")
	}
}

func TestRadixTreeIPv6(t *testing.T) {
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("2001:db8::/32"), ASEntry{AS: 400})
	tree.Insert(netip.MustParsePrefix("2001:db8:1::/48"), ASEntry{AS: 500})

	tests := []struct {
		addr   string
		wantAS uint32
		wantOK bool
	}{
		{"2001:db8::1", 400, true},
		{"2001:db8:1::1", 500, true},
		{"2001:db8:2::1", 400, true},
		{"2001:db9::1", 0, false},
	}

	for _, tc := range tests {
		addr := netip.MustParseAddr(tc.addr)
		entry, ok := tree.Lookup(addr)
		if ok != tc.wantOK {
			t.Errorf("Lookup(%s): ok=%v, want %v", tc.addr, ok, tc.wantOK)
			continue
		}
		if ok && entry.AS != tc.wantAS {
			t.Errorf("Lookup(%s): AS=%d, want %d", tc.addr, entry.AS, tc.wantAS)
		}
	}
}

func TestRadixTreeDelete(t *testing.T) {
	tree := NewRadixTree()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	tree.Insert(pfx, ASEntry{AS: 100})

	entry, ok := tree.Lookup(netip.MustParseAddr("10.1.2.3"))
	if !ok || entry.AS != 100 {
		t.Fatal("expected match before delete")
	}

	tree.Delete(pfx)

	_, ok = tree.Lookup(netip.MustParseAddr("10.1.2.3"))
	if ok {
		t.Error("expected no match after delete")
	}
}

func TestRadixTreeASPath(t *testing.T) {
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{
		AS:        100,
		NextHop:   netip.MustParseAddr("192.0.2.1"),
		LocalPref: 200,
		ASPath:    []uint32{64512, 100},
	})

	entry, ok := tree.Lookup(netip.MustParseAddr("10.5.5.5"))
	if !ok {
		t.Fatal("expected match")
	}
	if entry.NextHop != netip.MustParseAddr("192.0.2.1") {
		t.Errorf("NextHop = %v, want 192.0.2.1", entry.NextHop)
	}
	if entry.LocalPref != 200 {
		t.Errorf("LocalPref = %d, want 200", entry.LocalPref)
	}
	if len(entry.ASPath) != 2 || entry.ASPath[0] != 64512 || entry.ASPath[1] != 100 {
		t.Errorf("ASPath = %v, want [64512 100]", entry.ASPath)
	}
}
