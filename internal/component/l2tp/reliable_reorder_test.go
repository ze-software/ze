package l2tp

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-6 in-order delivery after gap fill. When a later Ns
// arrives before the missing earlier one, it is stored and only
// delivered once the gap is closed. PREVENTS: the guide S11.4 trap where
// out-of-order messages are exposed to the upper layer before being
// reassembled.
func TestReorderQueueStoreAndPop(t *testing.T) {
	q := newReorderQueue(4)

	// Store Ns=3 when nextRecvSeq=1. Offset = 2. Capacity is 4, so
	// offsets 1..3 are reorder-buffered (offset 0 is in-order, handled by
	// the caller directly; offsets >= capacity are beyond the window).
	if !q.store(3, 1, []byte("msg3")) {
		t.Fatalf("store(3, 1) returned false, expected true")
	}

	// Storing again at the same Ns returns false (already buffered).
	if q.store(3, 1, []byte("dup")) {
		t.Fatalf("store(3, 1) duplicate returned true, expected false")
	}

	// Pop with nextRecvSeq=1 yields nothing (gap not filled).
	if got := q.popInOrder(1); len(got) != 0 {
		t.Fatalf("popInOrder(1) = %d entries, want 0", len(got))
	}

	// Store Ns=2. Now queue has [_, 2, 3].
	if !q.store(2, 1, []byte("msg2")) {
		t.Fatalf("store(2, 1) returned false")
	}

	// Pop with nextRecvSeq=2 (caller just advanced to 2) yields 2 then 3.
	got := q.popInOrder(2)
	if len(got) != 2 {
		t.Fatalf("popInOrder(2) = %d entries, want 2", len(got))
	}
	if !bytes.Equal(got[0].payload, []byte("msg2")) || got[0].ns != 2 {
		t.Errorf("got[0] = {%d, %q}, want {2, \"msg2\"}", got[0].ns, got[0].payload)
	}
	if !bytes.Equal(got[1].payload, []byte("msg3")) || got[1].ns != 3 {
		t.Errorf("got[1] = {%d, %q}, want {3, \"msg3\"}", got[1].ns, got[1].payload)
	}
}

// VALIDATES: AC-7 Ns beyond the advertised window is rejected. The ring
// buffer capacity is the receive window; an offset at or beyond it must
// not displace an existing entry.
func TestReorderQueueBeyondCapacity(t *testing.T) {
	q := newReorderQueue(4)

	// With nextRecvSeq=10, accepted offsets are 0..3 (i.e., Ns 10..13).
	// Ns=14 is offset 4 -- exactly at capacity, must be rejected.
	if q.store(14, 10, []byte("too-far")) {
		t.Errorf("store(14, 10) with capacity 4 returned true, expected false")
	}
	if q.store(100, 10, []byte("way-too-far")) {
		t.Errorf("store(100, 10) returned true, expected false")
	}

	// Ns=13 is offset 3 -- the last valid slot.
	if !q.store(13, 10, []byte("last-slot")) {
		t.Errorf("store(13, 10) at last slot returned false")
	}
}

// VALIDATES: AC-6 through AC-27 wraparound. A reorder queue spanning the
// 65535/0 boundary must behave correctly. PREVENTS: integer-overflow
// mis-indexing near wrap.
func TestReorderQueueWraparound(t *testing.T) {
	q := newReorderQueue(4)

	// nextRecvSeq=65535. Valid Ns range: 65535, 0, 1, 2 (offsets 0..3).
	if !q.store(0, 65535, []byte("ns0")) {
		t.Fatalf("store(0, 65535) returned false")
	}
	if !q.store(2, 65535, []byte("ns2")) {
		t.Fatalf("store(2, 65535) returned false")
	}

	// Pop after filling the gap. Simulate receiving 65535 (in-order) and
	// 1 (in-order after 65535->0). Caller advances nextRecvSeq to 1.
	// With {0, 2} queued, popInOrder(1) should yield 0 (since 0 is before
	// nextRecvSeq wraparound-aware -- actually 0 is the NEXT expected
	// after wrap). Expect got[0].ns = 0.
	got := q.popInOrder(0)
	if len(got) != 1 || got[0].ns != 0 {
		t.Fatalf("popInOrder(0) = %+v, want [{0, ns0}]", got)
	}

	// Now nextRecvSeq=1 (caller advanced after delivering Ns=0 in-order).
	// popInOrder(1) yields nothing because 1 was never stored (it would
	// also arrive as in-order when it shows up on the wire).
	got = q.popInOrder(1)
	if len(got) != 0 {
		t.Fatalf("popInOrder(1) with gap at 1 = %+v, want []", got)
	}

	// Peer retransmits Ns=1. Caller handles it as in-order (matches
	// nextRecvSeq=1, delivers directly), advances nextRecvSeq to 2, then
	// drains the reorder queue from 2. Ns=2 was buffered; it pops.
	got = q.popInOrder(2)
	if len(got) != 1 || got[0].ns != 2 {
		t.Fatalf("popInOrder(2) after gap fill = %+v, want [{2, ns2}]", got)
	}
}

// VALIDATES: payload copy semantics. The ring buffer owns its storage;
// mutating the caller's buffer after store must not corrupt queued data.
func TestReorderQueueDefensiveCopy(t *testing.T) {
	q := newReorderQueue(4)

	src := []byte("hello")
	q.store(2, 1, src)
	src[0] = 'X' // mutate after store

	// Pop with nextRecvSeq=2 (caller just handled Ns=1 in-order).
	got := q.popInOrder(2)
	if len(got) != 1 || !bytes.Equal(got[0].payload, []byte("hello")) {
		t.Errorf("payload was not defensively copied: got %q", got[0].payload)
	}
}
