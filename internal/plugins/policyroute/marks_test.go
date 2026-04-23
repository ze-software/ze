package policyroute

import (
	"net/netip"
	"testing"
)

func TestMarkAllocation(t *testing.T) {
	a := newAllocator()

	m1, err := a.allocateMark("policy1:100")
	if err != nil {
		t.Fatalf("allocate mark 1: %v", err)
	}
	if m1 < fwmarkBase || m1 > fwmarkMax {
		t.Errorf("mark %d outside range [0x%x, 0x%x]", m1, fwmarkBase, fwmarkMax)
	}

	m2, err := a.allocateMark("policy1:200")
	if err != nil {
		t.Fatalf("allocate mark 2: %v", err)
	}
	if m1 == m2 {
		t.Errorf("marks should be unique, both are %d", m1)
	}

	m1again, err := a.allocateMark("policy1:100")
	if err != nil {
		t.Fatalf("re-allocate mark: %v", err)
	}
	if m1again != m1 {
		t.Errorf("same key should return same mark: got %d, want %d", m1again, m1)
	}
}

func TestTableAllocation(t *testing.T) {
	a := newAllocator()

	nh := netip.MustParseAddr("10.0.0.1")
	tbl1, isNew1, err := a.allocateTable(nh)
	if err != nil {
		t.Fatalf("allocate table: %v", err)
	}
	if !isNew1 {
		t.Error("first allocation should be new")
	}
	if tbl1 < autoTableBase || tbl1 > autoTableMax {
		t.Errorf("table %d outside range [%d, %d]", tbl1, autoTableBase, autoTableMax)
	}

	tbl2, isNew2, err := a.allocateTable(nh)
	if err != nil {
		t.Fatalf("re-allocate table: %v", err)
	}
	if isNew2 {
		t.Error("same next-hop should reuse table")
	}
	if tbl2 != tbl1 {
		t.Errorf("same next-hop: got table %d, want %d", tbl2, tbl1)
	}

	nh2 := netip.MustParseAddr("10.0.0.2")
	tbl3, isNew3, err := a.allocateTable(nh2)
	if err != nil {
		t.Fatalf("allocate different nh: %v", err)
	}
	if !isNew3 {
		t.Error("different next-hop should be new")
	}
	if tbl3 == tbl1 {
		t.Error("different next-hops should get different tables")
	}
}

func TestMarkAllocationReset(t *testing.T) {
	a := newAllocator()

	_, err := a.allocateMark("key1")
	if err != nil {
		t.Fatal(err)
	}

	a.reset()

	m, err := a.allocateMark("key1")
	if err != nil {
		t.Fatal(err)
	}
	if m != fwmarkBase {
		t.Errorf("after reset, mark should be base %d, got %d", fwmarkBase, m)
	}
}
