// RFC: rfc/short/rfc7854.md
// Design: docs/architecture/core-design.md -- BMP sender transmit queue
//
// Related: sender_drain.go -- the producer (enqueueLocked) and the drain (drainLoop)
//
// txQueue is the handoff between the goroutines that PRODUCE BMP messages (the
// bmp plugin's delivery loop, and whichever goroutine publishes a RIB
// best-change) and the one goroutine that WRITES them to the collector socket.
// Without it a producer did the socket write itself, so a wedged collector
// stalled a RIB publisher goroutine for up to writeTimeout per message --
// violating the EventBus contract that a subscriber "MUST NOT block on I/O"
// (pkg/ze/eventbus.go EventBus.Subscribe).
//
// Shape follows BIRD's BMP transmit queue (proto/bmp/bmp.c:222-226, bmp.h:79-82,
// master 02d082a7): a FIFO of pooled fixed-size pages with messages packed
// contiguously and split across page boundaries, bounded in BYTES rather than
// in messages, and reset wholesale when the bound is hit. Bytes rather than
// messages because BMP Route Monitoring messages vary from ~70 bytes to 64 KiB,
// so a message-count cap bounds nothing in particular.
//
// It is a page list rather than one fixed ring because the bound has to be
// large enough to absorb a full Loc-RIB dump (RFC 9069 initial dump) without
// tripping the overflow reset, and a ring that large would be resident for
// every configured collector from the first message onwards. Pages are taken
// from a package-level pool and returned on consumption, so a steady-state
// session holds one page and the BGP-UPDATE -> BMP Route Monitoring hot path
// stays allocation-free (see senderSession.scratch).

package bmp

import "sync"

const (
	// txPageSize is the queue's allocation unit. One page holds a whole
	// maxBMPMsgSize message, so any message spans at most two pages.
	txPageSize = maxBMPMsgSize + 1 // 65536

	// txQueueLimitBytes bounds the bytes a single collector session may have
	// queued but unwritten. Reaching it means the collector is not draining
	// its TCP window and the session is reset (RFC 7854 has no flow-control
	// mechanism to signal back-pressure with).
	//
	// This is NOT the defense against a wedged collector, and reading it as one
	// is the easy mistake: a collector that stops reading entirely never gets
	// near this number, because the drain's per-write deadline (writeTimeout,
	// 10s in sender.go) fails first and resets the session in seconds. The byte
	// bound bites for the other shape -- a collector that keeps reading, but
	// steadily slower than ze produces -- where every individual write succeeds
	// and the backlog is what grows.
	//
	// Sized to absorb a full Loc-RIB dump: ~1M IPv4 best paths at ~120 bytes
	// of Route Monitoring each is ~120 MB. BIRD's tx_pending_limit defaults to
	// 1 GiB (proto/bmp/config.Y:30) for the same reason; ze halves the order of
	// magnitude because it also runs as an appliance, and because a collector
	// that is 256 MiB behind is not going to catch up. A dump that genuinely
	// exceeds this settles into a connect -> dump -> overflow -> reconnect cycle
	// spaced by reconnectMin; that is a real failure mode, not a graceful
	// degradation, and the number is what decides how large a table triggers it.
	txQueueLimitBytes = 256 << 20
)

// txPagePool recycles queue pages across every collector session. Pages are
// pointer-typed so returning one to the pool does not allocate an interface box.
var txPagePool = sync.Pool{
	New: func() any {
		p := make([]byte, txPageSize)
		return &p
	},
}

// getTxPage takes a page from the pool. The pool's New always yields a
// *[]byte; the type check is belt-and-braces so a future pool change cannot
// hand the queue a nil page to copy into.
func getTxPage() *[]byte {
	p, ok := txPagePool.Get().(*[]byte)
	if !ok || p == nil || len(*p) != txPageSize {
		b := make([]byte, txPageSize)
		return &b
	}
	return p
}

func putTxPage(p *[]byte) { txPagePool.Put(p) }

// txQueue is a bounded FIFO byte queue with one producer set and one consumer.
//
// Producers call push; the single drain goroutine calls peek/advance. Safe for
// concurrent use: every field is guarded by mu, and the consumer holds NO lock
// while it writes peek's slice to the socket. That is sound because peek only
// ever returns queued (not free) bytes, and a producer only ever writes into
// free space, so the two never touch the same region.
type txQueue struct {
	mu       sync.Mutex
	pages    []*[]byte // FIFO: pages[0] is being drained, the last page is being filled
	readOff  int       // read offset into pages[0]
	writeOff int       // write offset into the last page
	pending  int       // queued bytes not yet consumed
	limit    int       // maximum pending bytes; push refuses beyond it

	// inFlight is the page peek() handed to the drain and advance() has not
	// released yet. The drain writes it to the socket WITHOUT holding mu, so a
	// concurrent reset() must not hand that page back to the shared pool: another
	// session would take it and overwrite bytes still being written to this one.
	// reset drops it instead and lets the garbage collector have it.
	inFlight *[]byte

	// signal wakes the drain goroutine. Buffered depth 1: it is an
	// edge notification, not a queue of its own.
	signal chan struct{}
}

// newTxQueue returns an empty queue bounded at limit bytes. A limit of zero or
// less falls back to txQueueLimitBytes rather than silently accepting nothing.
func newTxQueue(limit int) *txQueue {
	if limit <= 0 {
		limit = txQueueLimitBytes
	}
	return &txQueue{limit: limit, signal: make(chan struct{}, 1)}
}

// push copies msg into the queue and wakes the drain. It enqueues the WHOLE
// message or nothing at all: on refusal the queue still holds exactly the
// messages that were already in it, so a drain that is mid-flight never emits a
// half message. Returns false when the message would take pending past limit --
// the caller MUST then reset the session (see senderSession.overflow); this
// queue never drops a message it accepted and never blocks the producer.
func (q *txQueue) push(msg []byte) bool {
	if len(msg) == 0 {
		return true
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pending+len(msg) > q.limit {
		return false
	}

	for off := 0; off < len(msg); {
		if len(q.pages) == 0 || q.writeOff == txPageSize {
			q.pages = append(q.pages, getTxPage())
			q.writeOff = 0
		}
		page := *q.pages[len(q.pages)-1]
		n := copy(page[q.writeOff:], msg[off:])
		q.writeOff += n
		off += n
	}
	q.pending += len(msg)

	select {
	case q.signal <- struct{}{}:
	default: // a wake-up is already pending; the drain will see the new bytes
	}
	return true
}

// peek returns the contiguous queued bytes at the front of the queue, or nil
// when the queue is empty. The slice is valid until the matching advance: it
// aliases a queue page, and only advance hands that page back to the pool.
//
// Caller MUST be the single drain goroutine, and MUST call advance with the
// number of bytes it consumed before calling peek again.
func (q *txQueue) peek() []byte {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pending == 0 {
		return nil
	}
	end := txPageSize
	if len(q.pages) == 1 {
		end = q.writeOff
	}
	q.inFlight = q.pages[0]
	return (*q.pages[0])[q.readOff:end]
}

// advance releases the first n queued bytes, recycling any page they emptied.
//
// It is a no-op when a reset landed between the matching peek and this call:
// the bytes this advance refers to are already gone, and consuming n bytes of
// whatever a producer has pushed since would silently drop a message that was
// never written. inFlight is the marker -- peek sets it, reset clears it.
func (q *txQueue) advance(n int) {
	if n <= 0 {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.inFlight == nil {
		return
	}
	if n > q.pending {
		n = q.pending
	}
	q.readOff += n
	q.pending -= n
	q.inFlight = nil

	if len(q.pages) == 0 {
		// reset() emptied the queue while this write was in flight; the page
		// those bytes lived in was deliberately not recycled.
		q.readOff, q.writeOff, q.pending = 0, 0, 0
		return
	}
	if len(q.pages) == 1 {
		// Rewind the single page when it drains completely, so a steady-state
		// session keeps reusing one page instead of rotating through the pool.
		if q.readOff >= q.writeOff {
			q.readOff, q.writeOff = 0, 0
		}
		return
	}
	if q.readOff >= txPageSize {
		putTxPage(q.pages[0])
		copy(q.pages, q.pages[1:])
		q.pages[len(q.pages)-1] = nil
		q.pages = q.pages[:len(q.pages)-1]
		q.readOff = 0
	}
}

// reset drops every queued byte and returns the pages to the pool. Called when
// the session is torn down: the queued messages belong to a BMP session that no
// longer exists, and BMP has no resynchronization short of a fresh dump.
func (q *txQueue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, p := range q.pages {
		if p != q.inFlight {
			putTxPage(p)
		}
		q.pages[i] = nil
	}
	q.pages = q.pages[:0]
	q.inFlight = nil
	q.readOff, q.writeOff, q.pending = 0, 0, 0
}

// bytesPending reports the queued bytes not yet written to the socket.
func (q *txQueue) bytesPending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending
}

// wait blocks until push signals new bytes or stop is closed. It reports
// whether the caller should keep draining (false means stop).
func (q *txQueue) wait(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return false
	case <-q.signal:
		return true
	}
}

// wake unblocks a waiting drain without queueing anything, so a session teardown
// does not have to wait out the drain's next signal.
func (q *txQueue) wake() {
	select {
	case q.signal <- struct{}{}:
	default: // already pending
	}
}
