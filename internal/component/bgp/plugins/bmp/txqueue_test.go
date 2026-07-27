package bmp

import (
	"bytes"
	"testing"
)

// drainAll consumes the whole queue and returns the bytes in FIFO order.
func drainAll(q *txQueue) []byte {
	var out []byte
	for {
		buf := q.peek()
		if buf == nil {
			return out
		}
		out = append(out, buf...)
		q.advance(len(buf))
	}
}

func TestTxQueuePreservesMessageOrderAndBytes(t *testing.T) {
	// VALIDATES: the queue is a FIFO byte stream -- what the drain reads back is
	// exactly the concatenation of what the producers pushed, in push order.
	// PREVENTS: a reordered or corrupted BMP stream, which desynchronizes the
	// collector's framing for every message after it.
	q := newTxQueue(1 << 20)

	msgs := [][]byte{
		bytes.Repeat([]byte{0xA1}, 10),
		bytes.Repeat([]byte{0xB2}, 300),
		bytes.Repeat([]byte{0xC3}, 1),
	}
	var want []byte
	for _, m := range msgs {
		if !q.push(m) {
			t.Fatalf("push(%d bytes) refused with %d/%d pending", len(m), q.bytesPending(), 1<<20)
		}
		want = append(want, m...)
	}
	if got := q.bytesPending(); got != len(want) {
		t.Errorf("bytesPending = %d, want %d", got, len(want))
	}

	got := drainAll(q)
	if !bytes.Equal(got, want) {
		t.Errorf("drained %d bytes, want %d (content mismatch)", len(got), len(want))
	}
	if p := q.bytesPending(); p != 0 {
		t.Errorf("bytesPending after full drain = %d, want 0", p)
	}
}

func TestTxQueueSplitsMessagesAcrossPages(t *testing.T) {
	// VALIDATES: a message that does not fit in the current page is packed
	// across the page boundary and read back whole (BIRD bmp.c:222-226).
	// PREVENTS: silently truncating at the page boundary, or refusing a message
	// the byte budget had room for.
	q := newTxQueue(4 * txPageSize)

	// Fill most of the first page, then push a message that must straddle it.
	head := bytes.Repeat([]byte{0x11}, txPageSize-100)
	if !q.push(head) {
		t.Fatal("push of head message refused")
	}
	straddle := bytes.Repeat([]byte{0x22}, 500)
	if !q.push(straddle) {
		t.Fatal("push of straddling message refused")
	}

	want := append(append([]byte{}, head...), straddle...)
	if got := drainAll(q); !bytes.Equal(got, want) {
		t.Errorf("straddling message did not survive the page boundary: got %d bytes, want %d", len(got), len(want))
	}
}

func TestTxQueueRefusesWholeMessageAtLimit(t *testing.T) {
	// VALIDATES: the bound is in BYTES, and a message that would cross it is
	// refused ENTIRELY -- nothing is copied, and the queue still holds exactly
	// what it held before.
	// PREVENTS: BIRD's mid-message return (bmp.c:307-312), which leaves a
	// partially copied message in the queue; and a message-count cap, which
	// bounds nothing when Route Monitoring messages range from 70B to 64KiB.
	q := newTxQueue(1000)

	accepted := bytes.Repeat([]byte{0x33}, 900)
	if !q.push(accepted) {
		t.Fatal("push within the limit was refused")
	}
	if q.push(bytes.Repeat([]byte{0x44}, 101)) {
		t.Fatal("push past the byte limit was accepted")
	}
	if got := q.bytesPending(); got != len(accepted) {
		t.Errorf("bytesPending after refused push = %d, want %d (partial copy)", got, len(accepted))
	}
	// The refused bytes must not appear anywhere in the queue.
	got := drainAll(q)
	if !bytes.Equal(got, accepted) {
		t.Errorf("queue content changed on a refused push: %d bytes, want %d", len(got), len(accepted))
	}

	// Exactly-at-limit still fits: the bound is "> limit", not ">= limit".
	q2 := newTxQueue(1000)
	if !q2.push(bytes.Repeat([]byte{0x55}, 1000)) {
		t.Error("a message of exactly limit bytes must be accepted")
	}
	if q2.push([]byte{0x56}) {
		t.Error("one byte past a full queue must be refused")
	}
}

func TestTxQueueResetDropsEverything(t *testing.T) {
	// VALIDATES: reset empties the queue (BIRD frees the whole queue on session
	// down, bmp.c:1197-1215) and the queue is reusable afterwards.
	// PREVENTS: leaking the previous session's messages into the next BMP
	// session, whose collector has no context for them.
	q := newTxQueue(1 << 20)
	for range 50 {
		if !q.push(bytes.Repeat([]byte{0x66}, 1000)) {
			t.Fatal("push refused while filling")
		}
	}
	q.reset()
	if got := q.bytesPending(); got != 0 {
		t.Errorf("bytesPending after reset = %d, want 0", got)
	}
	if got := q.peek(); got != nil {
		t.Errorf("peek after reset returned %d bytes, want nil", len(got))
	}

	after := []byte{0x77, 0x78}
	if !q.push(after) {
		t.Fatal("push after reset refused")
	}
	if got := drainAll(q); !bytes.Equal(got, after) {
		t.Errorf("queue after reset drained %x, want %x", got, after)
	}
}

func TestTxQueueResetKeepsInFlightPageOutOfThePool(t *testing.T) {
	// VALIDATES: a reset that lands while the drain is writing does NOT recycle
	// the page those bytes live in, so the next session to take a page from the
	// pool cannot overwrite an in-flight socket write.
	// PREVENTS: cross-session corruption. The drain writes peek()'s slice with
	// no lock held (that is the point -- the producers must not wait on the
	// socket), and a session reset can come from a producer goroutine or from
	// the session loop at any moment during that write.
	q := newTxQueue(1 << 20)
	msg := bytes.Repeat([]byte{0x5A}, 400)
	if !q.push(msg) {
		t.Fatal("push refused")
	}

	inFlight := q.peek() // the drain is now "writing" these bytes
	q.reset()            // ... and the session is reset underneath it

	// Another session takes pages from the same pool and fills them.
	other := newTxQueue(1 << 20)
	for range 4 {
		if !other.push(bytes.Repeat([]byte{0xC7}, 400)) {
			t.Fatal("push on the second queue refused")
		}
	}

	if !bytes.Equal(inFlight, msg) {
		t.Errorf("the bytes being written were overwritten by another session: got %x..., want %x...",
			inFlight[:8], msg[:8])
	}
}

func TestTxQueueWaitReturnsOnPushAndStop(t *testing.T) {
	// VALIDATES: the drain wakes on a push and on stop, so a session teardown
	// never waits for traffic that will not come.
	// PREVENTS: a drain goroutine that outlives its session.
	q := newTxQueue(1 << 20)
	stop := make(chan struct{})

	q.push([]byte{0x01})
	if !q.wait(stop) {
		t.Error("wait after a push must report that draining should continue")
	}

	close(stop)
	if q.wait(stop) {
		t.Error("wait with a closed stop channel must report stop")
	}
}

func TestTxQueueSteadyStateIsAllocationFree(t *testing.T) {
	// VALIDATES: push+drain of a steady stream allocates nothing once the first
	// page is in hand -- pages come from txPagePool and are rewound in place.
	// PREVENTS: regressing to a per-message []byte copy, which is the whole
	// reason this is a pooled byte queue rather than a [][]byte channel
	// (ai/rules/memory-architecture.md, ai/rules/buffer-first.md).
	q := newTxQueue(1 << 20)
	msg := bytes.Repeat([]byte{0x99}, 160) // a typical Loc-RIB Route Monitoring

	allocs := testing.AllocsPerRun(500, func() {
		q.push(msg)
		for buf := q.peek(); buf != nil; buf = q.peek() {
			q.advance(len(buf))
		}
	})
	if allocs != 0 {
		t.Errorf("push+drain allocates %.2f objects per message, want 0", allocs)
	}
}
