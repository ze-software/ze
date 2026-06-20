package enrich

import (
	"net/netip"
	"testing"
)

func TestEnricherAtomicSwap(t *testing.T) {
	e := NewEnricher()

	_, ok := e.Lookup(netip.MustParseAddr("10.0.0.1"))
	if ok {
		t.Fatal("expected no match in empty enricher")
	}

	tree1 := NewRadixTree()
	tree1.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{AS: 100})
	e.UpdateTree(tree1)

	entry, ok := e.Lookup(netip.MustParseAddr("10.0.0.1"))
	if !ok || entry.AS != 100 {
		t.Fatalf("after swap to tree1: ok=%v, AS=%d", ok, entry.AS)
	}

	tree2 := NewRadixTree()
	tree2.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{AS: 999})
	e.UpdateTree(tree2)

	entry, ok = e.Lookup(netip.MustParseAddr("10.0.0.1"))
	if !ok || entry.AS != 999 {
		t.Fatalf("after swap to tree2: ok=%v, AS=%d", ok, entry.AS)
	}
}

func TestEnricherEnrich(t *testing.T) {
	e := NewEnricher()
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{
		AS:     100,
		ASPath: []uint32{64512, 100},
	})
	tree.Insert(netip.MustParsePrefix("192.168.0.0/16"), ASEntry{
		AS:        200,
		NextHop:   netip.MustParseAddr("203.0.113.1"),
		LocalPref: 150,
		ASPath:    []uint32{64512, 200},
	})
	e.UpdateTree(tree)

	result := e.Enrich(
		netip.MustParseAddr("10.1.2.3"),
		netip.MustParseAddr("192.168.1.1"),
	)

	if result.SrcAS != 100 {
		t.Errorf("SrcAS = %d, want 100", result.SrcAS)
	}
	if result.DstAS != 200 {
		t.Errorf("DstAS = %d, want 200", result.DstAS)
	}
	if result.NextHop != netip.MustParseAddr("203.0.113.1") {
		t.Errorf("NextHop = %v, want 203.0.113.1", result.NextHop)
	}
	if result.LocalPref != 150 {
		t.Errorf("LocalPref = %d, want 150", result.LocalPref)
	}
	if len(result.SrcASPath) != 2 || result.SrcASPath[1] != 100 {
		t.Errorf("SrcASPath = %v, want [64512 100]", result.SrcASPath)
	}
	if len(result.DstASPath) != 2 || result.DstASPath[1] != 200 {
		t.Errorf("DstASPath = %v, want [64512 200]", result.DstASPath)
	}
}

func TestEnricherEnrichPartial(t *testing.T) {
	e := NewEnricher()
	tree := NewRadixTree()
	tree.Insert(netip.MustParsePrefix("10.0.0.0/8"), ASEntry{AS: 100})
	e.UpdateTree(tree)

	result := e.Enrich(
		netip.MustParseAddr("10.1.2.3"),
		netip.MustParseAddr("172.16.0.1"),
	)

	if result.SrcAS != 100 {
		t.Errorf("SrcAS = %d, want 100", result.SrcAS)
	}
	if result.DstAS != 0 {
		t.Errorf("DstAS = %d, want 0 (no match)", result.DstAS)
	}
}

func TestEnricherNilTree(t *testing.T) {
	e := NewEnricher()
	e.UpdateTree(nil)

	_, ok := e.Lookup(netip.MustParseAddr("10.0.0.1"))
	if ok {
		t.Error("expected no match after nil tree update")
	}
}
