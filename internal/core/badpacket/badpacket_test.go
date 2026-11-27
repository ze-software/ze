package badpacket

import (
	"bytes"
	"encoding/hex"
	"sync"
	"testing"
)

// TestBadPacketRingBoundsAndFilter verifies payload boundary sizes (0, 4096,
// 4097), 16-slot ring eviction, newest-first ordering, case-insensitive
// protocol filtering, limit bounds, CopyBounded detachment, and race safety.
//
// VALIDATES: AC-6 -- MaxPayloadBytes (4096) truncation, MaxRecords (16) ring
// eviction, newest-first Snapshot ordering, case-insensitive protocol filter,
// CopyBounded isolation, and concurrent Record/Snapshot/Count safety.
// PREVENTS: unbounded memory growth under malformed packet floods, mutated
// backing buffers corrupting captured frames, or nil slices on empty snapshots.
func TestBadPacketRingBoundsAndFilter(t *testing.T) {
	t.Run("empty snapshot is non-nil", func(t *testing.T) {
		defaultRing.reset()
		t.Cleanup(defaultRing.reset)

		got := Snapshot(0, "")
		if got == nil {
			t.Fatal("Snapshot(0, \"\") = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("len(Snapshot) = %d, want 0", len(got))
		}
		if Count() != 0 {
			t.Fatalf("Count() = %d, want 0", Count())
		}
	})

	t.Run("payload size boundaries 0 4096 4097", func(t *testing.T) {
		defaultRing.reset()
		t.Cleanup(defaultRing.reset)

		id0 := Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "empty", nil)
		buf4096 := bytes.Repeat([]byte{0xab}, MaxPayloadBytes)
		id4096 := Record("bgp", "192.0.2.1", "UPDATE", "attribute-discard", "exact-max", buf4096)
		buf4097 := bytes.Repeat([]byte{0xcd}, MaxPayloadBytes+1)
		id4097 := Record("bgp", "192.0.2.2", "HEADER", "session-reset", "over-max", buf4097)

		if id0 != "pkt-1" || id4096 != "pkt-2" || id4097 != "pkt-3" {
			t.Fatalf("IDs = [%s, %s, %s], want [pkt-1, pkt-2, pkt-3]", id0, id4096, id4097)
		}
		if Count() != 3 {
			t.Fatalf("Count() = %d, want 3", Count())
		}

		snaps := Snapshot(0, "")
		if len(snaps) != 3 {
			t.Fatalf("len(snaps) = %d, want 3", len(snaps))
		}

		// Newest-first: pkt-3 (4097), pkt-2 (4096), pkt-1 (0).
		over := snaps[0]
		if over.ID != "pkt-3" || over.Length != MaxPayloadBytes+1 || !over.Truncated {
			t.Errorf("4097 record = {ID:%s Length:%d Truncated:%v}, want pkt-3 %d true",
				over.ID, over.Length, over.Truncated, MaxPayloadBytes+1)
		}
		if len(over.Hex) != MaxPayloadBytes*2 {
			t.Errorf("4097 Hex len = %d, want %d", len(over.Hex), MaxPayloadBytes*2)
		}
		if over.Hex != hex.EncodeToString(buf4097[:MaxPayloadBytes]) {
			t.Error("4097 Hex payload mismatch or not lowercase")
		}

		exact := snaps[1]
		if exact.ID != "pkt-2" || exact.Length != MaxPayloadBytes || exact.Truncated {
			t.Errorf("4096 record = {ID:%s Length:%d Truncated:%v}, want pkt-2 %d false",
				exact.ID, exact.Length, exact.Truncated, MaxPayloadBytes)
		}
		if len(exact.Hex) != MaxPayloadBytes*2 {
			t.Errorf("4096 Hex len = %d, want %d", len(exact.Hex), MaxPayloadBytes*2)
		}

		zero := snaps[2]
		if zero.ID != "pkt-1" || zero.Length != 0 || zero.Truncated || zero.Hex != "" {
			t.Errorf("0 record = %+v, want pkt-1 len=0 truncated=false empty hex", zero)
		}
	})

	t.Run("ring capacity 16 vs 17 evicts oldest", func(t *testing.T) {
		defaultRing.reset()
		t.Cleanup(defaultRing.reset)

		for i := range MaxRecords {
			Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "fill", []byte{byte(i)})
		}
		snaps16 := Snapshot(0, "")
		if len(snaps16) != MaxRecords {
			t.Fatalf("len(snaps16) = %d, want %d", len(snaps16), MaxRecords)
		}
		if snaps16[0].ID != "pkt-16" || snaps16[MaxRecords-1].ID != "pkt-1" {
			t.Fatalf("snaps16 ends = [%s .. %s], want [pkt-16 .. pkt-1]", snaps16[0].ID, snaps16[MaxRecords-1].ID)
		}

		// 17th record evicts pkt-1.
		id17 := Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "evict", []byte{0xff})
		if id17 != "pkt-17" {
			t.Fatalf("17th ID = %q, want pkt-17", id17)
		}
		if Count() != MaxRecords+1 {
			t.Fatalf("Count() = %d, want %d", Count(), MaxRecords+1)
		}
		snaps17 := Snapshot(0, "")
		if len(snaps17) != MaxRecords {
			t.Fatalf("len(snaps17) = %d, want %d", len(snaps17), MaxRecords)
		}
		if snaps17[0].ID != "pkt-17" || snaps17[MaxRecords-1].ID != "pkt-2" {
			t.Fatalf("snaps17 ends = [%s .. %s], want [pkt-17 .. pkt-2]", snaps17[0].ID, snaps17[MaxRecords-1].ID)
		}
	})

	t.Run("protocol filter and limit", func(t *testing.T) {
		defaultRing.reset()
		t.Cleanup(defaultRing.reset)

		Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "r1", []byte{0x01})
		Record("ospf", "192.0.2.2", "HELLO", "task-panic", "r2", []byte{0x02})
		Record("BGP", "192.0.2.3", "HEADER", "session-reset", "r3", []byte{0x03})

		bgpOnly := Snapshot(0, "BgP")
		if len(bgpOnly) != 2 {
			t.Fatalf("Snapshot(0, BgP) len = %d, want 2", len(bgpOnly))
		}
		if bgpOnly[0].ID != "pkt-3" || bgpOnly[1].ID != "pkt-1" {
			t.Errorf("Snapshot(0, BgP) IDs = [%s, %s], want [pkt-3, pkt-1]", bgpOnly[0].ID, bgpOnly[1].ID)
		}

		limited := Snapshot(1, "bgp")
		if len(limited) != 1 || limited[0].ID != "pkt-3" {
			t.Errorf("Snapshot(1, bgp) = %+v, want [pkt-3]", limited)
		}

		all16 := Snapshot(MaxRecords, "")
		if len(all16) != 3 {
			t.Errorf("Snapshot(16, \"\") len = %d, want 3", len(all16))
		}
	})

	t.Run("CopyBounded detaches and caps payload", func(t *testing.T) {
		if got := CopyBounded(nil); got != nil {
			t.Fatalf("CopyBounded(nil) = %v, want nil", got)
		}
		if got := CopyBounded([]byte{}); got != nil {
			t.Fatalf("CopyBounded(empty) = %v, want nil", got)
		}

		src := []byte{0xde, 0xad, 0xbe, 0xef}
		copied := CopyBounded(src)
		src[0] = 0x00
		if !bytes.Equal(copied, []byte{0xde, 0xad, 0xbe, 0xef}) {
			t.Fatalf("CopyBounded aliased caller buffer: %x", copied)
		}

		big := bytes.Repeat([]byte{0x42}, MaxPayloadBytes+50)
		capped := CopyBounded(big)
		if len(capped) != MaxPayloadBytes {
			t.Fatalf("len(CopyBounded(big)) = %d, want %d", len(capped), MaxPayloadBytes)
		}
	})

	t.Run("concurrent Record Snapshot Count are race-free", func(t *testing.T) {
		defaultRing.reset()
		t.Cleanup(defaultRing.reset)

		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go runConcurrentBadPacketWorker(&wg)
		}
		wg.Wait()
		if got := Count(); got != 160 {
			t.Fatalf("Count() = %d, want 160", got)
		}
	})
}

func runConcurrentBadPacketWorker(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 40 {
		Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "race", []byte{0x01, 0x02})
		_ = Snapshot(4, "bgp")
		_ = Count()
	}
}
