// Design: docs/architecture/diagnostics/crash-capture.md -- the relay queue that never blocks a writer
// Related: stderr.go -- the pump that fills it and the relay that drains it

package crashlog

import (
	"io"
	"strconv"
	"sync"
)

// stderrQueue is a bounded byte queue between the stderr pump and the relay.
//
// Write never blocks: a byte that does not fit under the limit is dropped and
// counted, and the count is written into the stream at the point the bytes went
// missing, as soon as there is room again or the queue closes. Read blocks until
// there is data or the queue is closed.
type stderrQueue struct {
	mu    sync.Mutex
	ready *sync.Cond
	buf   []byte
	head  int // buf[head:] is unread
	limit int

	dropped int64 // bytes dropped since the last drop notice
	closed  bool
	err     error // the pump's read error, nil at EOF
}

func newStderrQueue(limit int) *stderrQueue {
	q := &stderrQueue{limit: limit}
	q.ready = sync.NewCond(&q.mu)
	return q
}

// Write queues what fits under the limit and drops the rest. It always reports
// the whole of p as written, because the pump has nobody to hand a short write
// to: the drop is reported in the stream instead.
func (q *stderrQueue) Write(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.compact()
	room := q.limit - len(q.buf)
	if room > 0 && q.dropped > 0 {
		q.appendDropNotice()
	}
	keep := min(len(p), max(room, 0))
	q.buf = append(q.buf, p[:keep]...)
	q.dropped += int64(len(p) - keep)
	q.ready.Signal()
	return len(p), nil
}

// Read copies queued bytes into p. It returns io.EOF, or the pump's error, only
// once the queue is closed and every queued byte has been read.
func (q *stderrQueue) Read(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.head == len(q.buf) && !q.closed {
		q.ready.Wait()
	}
	if q.head < len(q.buf) {
		n := copy(p, q.buf[q.head:])
		q.head += n
		return n, nil
	}
	if q.err != nil {
		return 0, q.err
	}
	return 0, io.EOF
}

// close ends the queue. A drop not yet reported is reported before the end.
func (q *stderrQueue) close(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.dropped > 0 {
		q.compact()
		q.appendDropNotice()
	}
	q.closed = true
	q.err = err
	q.ready.Broadcast()
}

// compact moves the unread bytes to the front, so the limit counts only bytes
// the relay has still to write.
func (q *stderrQueue) compact() {
	if q.head == 0 {
		return
	}
	n := copy(q.buf, q.buf[q.head:])
	q.buf = q.buf[:n]
	q.head = 0
}

// appendDropNotice writes the drop count into the stream on a line of its own.
// The notice may take the queue past its limit by its own length, because a drop
// that is never reported reads as output that was never written.
func (q *stderrQueue) appendDropNotice() {
	q.buf = append(q.buf, "\ncrashlog: "...)
	q.buf = strconv.AppendInt(q.buf, q.dropped, 10)
	q.buf = append(q.buf, " bytes of stderr dropped: the relay fell behind its output\n"...)
	q.dropped = 0
}
