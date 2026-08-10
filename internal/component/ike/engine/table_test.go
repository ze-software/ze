package engine

import "testing"

func TestSATableCreate(t *testing.T) {
	tbl := NewSATable()
	sa := &SA{
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
		PeerName:     "test-peer",
	}
	if !tbl.Insert(sa) {
		t.Fatal("Insert should succeed for new SA")
	}
	if tbl.Len() != 1 {
		t.Fatalf("expected 1 SA, got %d", tbl.Len())
	}
	got := tbl.Lookup(sa.InitiatorSPI, sa.ResponderSPI)
	if got != sa {
		t.Fatal("Lookup should return the inserted SA")
	}
}

func TestSATableDuplicateInsert(t *testing.T) {
	tbl := NewSATable()
	sa := &SA{
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
	}
	tbl.Insert(sa)
	if tbl.Insert(sa) {
		t.Fatal("duplicate Insert should return false")
	}
}

func TestSATableRemove(t *testing.T) {
	tbl := NewSATable()
	sa := &SA{
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
	}
	tbl.Insert(sa)
	removed := tbl.Remove(sa.InitiatorSPI, sa.ResponderSPI)
	if removed != sa {
		t.Fatal("Remove should return the removed SA")
	}
	if tbl.Len() != 0 {
		t.Fatalf("expected 0 SAs after remove, got %d", tbl.Len())
	}
	if tbl.Lookup(sa.InitiatorSPI, sa.ResponderSPI) != nil {
		t.Fatal("Lookup should return nil after Remove")
	}
}

func TestSATableLookupBySPI(t *testing.T) {
	tbl := NewSATable()
	sa1 := &SA{
		InitiatorSPI: [8]byte{1, 0, 0, 0, 0, 0, 0, 0},
		ResponderSPI: [8]byte{2, 0, 0, 0, 0, 0, 0, 0},
		PeerName:     "peer1",
	}
	sa2 := &SA{
		InitiatorSPI: [8]byte{3, 0, 0, 0, 0, 0, 0, 0},
		ResponderSPI: [8]byte{4, 0, 0, 0, 0, 0, 0, 0},
		PeerName:     "peer2",
	}
	tbl.Insert(sa1)
	tbl.Insert(sa2)

	got := tbl.Lookup(sa2.InitiatorSPI, sa2.ResponderSPI)
	if got != sa2 {
		t.Fatal("Lookup should find sa2 by its SPI pair")
	}

	got = tbl.Lookup([8]byte{99}, [8]byte{99})
	if got != nil {
		t.Fatal("Lookup should return nil for unknown SPI pair")
	}
}

func TestSATableLookupByInitiatorSPI(t *testing.T) {
	tbl := NewSATable()
	iSPI := [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0, 0, 0, 0}
	sa := &SA{
		InitiatorSPI: iSPI,
		ResponderSPI: [8]byte{}, // zero, not yet known
		PeerName:     "pending",
	}
	tbl.Insert(sa)

	got := tbl.lookupByInitiatorSPI(iSPI)
	if got != sa {
		t.Fatal("LookupByInitiatorSPI should find SA with matching initiator SPI")
	}

	got = tbl.lookupByInitiatorSPI([8]byte{0xff})
	if got != nil {
		t.Fatal("LookupByInitiatorSPI should return nil for unknown initiator SPI")
	}
}

func TestSATableUpdateKey(t *testing.T) {
	tbl := NewSATable()
	iSPI := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	oldRSPI := [8]byte{} // zero initially
	newRSPI := [8]byte{8, 7, 6, 5, 4, 3, 2, 1}

	sa := &SA{
		InitiatorSPI: iSPI,
		ResponderSPI: oldRSPI,
	}
	tbl.Insert(sa)

	sa.ResponderSPI = newRSPI
	tbl.UpdateKey(oldRSPI, newRSPI, sa)

	if tbl.Lookup(iSPI, oldRSPI) != nil {
		t.Fatal("old key should be removed after UpdateKey")
	}
	if tbl.Lookup(iSPI, newRSPI) != sa {
		t.Fatal("new key should find the SA after UpdateKey")
	}
}

func TestSATableAll(t *testing.T) {
	tbl := NewSATable()
	for i := byte(1); i <= 3; i++ {
		tbl.Insert(&SA{
			InitiatorSPI: [8]byte{i},
			ResponderSPI: [8]byte{i + 10},
		})
	}
	all := tbl.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 SAs, got %d", len(all))
	}
}
