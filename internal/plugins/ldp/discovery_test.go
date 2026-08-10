package ldp

import (
	"net/netip"
	"testing"
	"time"
)

func TestAdjacencyTableUpdate(t *testing.T) {
	table := newAdjacencyTable()
	pdu := PDUHeader{
		Version:    ldpVersion,
		LSRID:      [4]byte{10, 0, 0, 1},
		LabelSpace: 0,
	}
	hello := HelloMessage{
		HoldTime:      15,
		TransportAddr: netip.MustParseAddr("10.0.0.1"),
	}

	adj, isNew := table.Update(pdu, hello, "")
	if !isNew {
		t.Fatal("first Update should return isNew=true")
	}
	if adj.TransportAddr != hello.TransportAddr {
		t.Errorf("TransportAddr = %s, want %s", adj.TransportAddr, hello.TransportAddr)
	}
	if adj.HoldTime != 15*time.Second {
		t.Errorf("HoldTime = %v, want 15s", adj.HoldTime)
	}

	_, isNew2 := table.Update(pdu, hello, "")
	if isNew2 {
		t.Fatal("second Update should return isNew=false")
	}

	if table.Len() != 1 {
		t.Errorf("Len = %d, want 1", table.Len())
	}
}

func TestAdjacencyTableDefaultHoldTime(t *testing.T) {
	table := newAdjacencyTable()
	pdu := PDUHeader{LSRID: [4]byte{10, 0, 0, 1}}
	hello := HelloMessage{
		HoldTime:      0,
		TransportAddr: netip.MustParseAddr("10.0.0.1"),
	}

	adj, _ := table.Update(pdu, hello, "")
	if adj.HoldTime != DefaultHelloHoldTime {
		t.Errorf("HoldTime = %v, want %v (default)", adj.HoldTime, DefaultHelloHoldTime)
	}
}

func TestAdjacencyExpired(t *testing.T) {
	adj := &Adjacency{
		HoldTime: 15 * time.Second,
		LastSeen: time.Now().Add(-20 * time.Second),
	}
	if !adj.Expired(time.Now()) {
		t.Error("adjacency should be expired")
	}

	adj.LastSeen = time.Now()
	if adj.Expired(time.Now()) {
		t.Error("adjacency should not be expired")
	}
}

func TestAdjacencyTableExpireSweep(t *testing.T) {
	table := newAdjacencyTable()
	pdu := PDUHeader{LSRID: [4]byte{10, 0, 0, 1}}
	hello := HelloMessage{
		HoldTime:      1,
		TransportAddr: netip.MustParseAddr("10.0.0.1"),
	}
	table.Update(pdu, hello, "")

	key := AdjacencyKey(pdu.LSRID, pdu.LabelSpace)
	adj, _ := table.Get(key)
	if adj.HoldTime != 1*time.Second {
		t.Fatalf("unexpected hold time: %v", adj.HoldTime)
	}

	time.Sleep(1100 * time.Millisecond)

	expired := table.ExpireSweep()
	if len(expired) != 1 {
		t.Fatalf("ExpireSweep returned %d, want 1", len(expired))
	}
	if table.Len() != 0 {
		t.Fatalf("Len = %d after sweep, want 0", table.Len())
	}
}

func TestAdjacencyTableRemove(t *testing.T) {
	table := newAdjacencyTable()
	pdu := PDUHeader{LSRID: [4]byte{10, 0, 0, 1}}
	hello := HelloMessage{
		HoldTime:      15,
		TransportAddr: netip.MustParseAddr("10.0.0.1"),
	}
	table.Update(pdu, hello, "")

	key := AdjacencyKey(pdu.LSRID, pdu.LabelSpace)
	table.Remove(key)
	if table.Len() != 0 {
		t.Errorf("Len = %d after Remove, want 0", table.Len())
	}
}

func TestAdjacencyTableAll(t *testing.T) {
	table := newAdjacencyTable()
	for i := byte(1); i <= 3; i++ {
		pdu := PDUHeader{LSRID: [4]byte{10, 0, 0, i}}
		hello := HelloMessage{
			HoldTime:      15,
			TransportAddr: netip.AddrFrom4([4]byte{10, 0, 0, i}),
		}
		table.Update(pdu, hello, "")
	}
	all := table.All()
	if len(all) != 3 {
		t.Errorf("All returned %d, want 3", len(all))
	}
}

func TestAdjacencyKey(t *testing.T) {
	key := AdjacencyKey([4]byte{10, 0, 0, 1}, 0)
	if key != "10.0.0.1:0" {
		t.Errorf("AdjacencyKey = %q, want %q", key, "10.0.0.1:0")
	}
}
