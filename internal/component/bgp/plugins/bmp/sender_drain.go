// RFC: rfc/short/rfc7854.md
// Design: docs/architecture/core-design.md -- BMP sender transmit path
//
// Overview: sender.go -- the sender session this drains for
// Related: txqueue.go -- the bounded byte queue itself
//
// The producer half (enqueueLocked) and the consumer half (drainLoop) of the
// BMP sender's transmit path, split out of sender.go because they are one
// concern: getting a finished BMP message from whichever goroutine produced it
// onto the collector socket without that goroutine ever touching the socket.

package bmp

// enqueueLocked copies a pre-encoded BMP message into the session's transmit
// queue and returns immediately. The drain goroutine writes it to the socket.
//
// This function NEVER blocks on I/O and never partially enqueues a message. It
// returns errNotConnected when there is no collector connection (the message is
// dropped, as it was before there was a queue), and errQueueOverflow when the
// message would take the queue past its byte bound -- in which case the session
// is reset here and the message is not queued.
//
// Caller MUST hold writeMu: data is almost always a slice of the shared scratch
// buffer, and the lock is what keeps this message's bytes contiguous against the
// other producer goroutines described on senderSession.
func (ss *senderSession) enqueueLocked(data []byte) error {
	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()

	if c == nil {
		return errNotConnected
	}

	q := ss.ensureDrain()
	if q.push(data) {
		return nil
	}

	// RFC 7854 has no way to tell a collector "slow down", and dropping a
	// Route Monitoring message would silently corrupt the collector's view of
	// the RIB. Reset the session instead: the reconnect re-sends Initiation,
	// every Peer Up and a full fresh dump, which is the only resynchronization
	// BMP offers. Same policy as BIRD (proto/bmp/bmp.c:1197-1215).
	logger().Error("bmp: collector connection stalled, resetting session",
		"collector", ss.name,
		"queued-bytes", q.bytesPending(),
		"limit-bytes", q.limit,
		"message-bytes", len(data),
		"remedy", "collector is not reading its TCP stream; check the collector or reduce monitored churn",
	)
	// Bare TCP close, deliberately NO Termination message: RFC 7854 Section 4.5
	// Termination says why the ROUTER is closing the session, and BIRD defines
	// BMP_TERM_REASON_OOR but never sends it (proto/bmp/bmp.c:159,981). Writing
	// one here would also mean queueing on a queue that just proved it is full.
	closeLog(c, "sender-queue-overflow")
	ss.clearConnAndResetIf(c, q)
	return errQueueOverflow
}

// ensureDrain returns the session's transmit queue, starting the drain
// goroutine on first use.
//
// Lazily rather than in newSenderSession because a session that never connects
// never enqueues, and because it keeps the queue and its one consumer created
// together: there is no window in which messages accumulate with nothing to
// write them. The goroutine runs until stopCh closes; run() waits for it.
func (ss *senderSession) ensureDrain() *txQueue {
	ss.drainMu.Lock()
	defer ss.drainMu.Unlock()

	if ss.txq == nil {
		ss.txq = newTxQueue(ss.txLimit)
	}
	if !ss.drainStarted {
		ss.drainStarted = true
		ss.drainDone = make(chan struct{})
		go ss.drainLoop(ss.txq)
	}
	return ss.txq
}

// queue returns the transmit queue, or nil when the session never enqueued.
func (ss *senderSession) queue() *txQueue {
	ss.drainMu.Lock()
	defer ss.drainMu.Unlock()
	return ss.txq
}

// resetQueue drops every queued byte. No-op when nothing was ever queued.
func (ss *senderSession) resetQueue() {
	if q := ss.queue(); q != nil {
		q.reset()
	}
}

// drainLoop is the session's single socket writer: it moves queued bytes to the
// collector until the session stops.
//
// A write failure ends the connection rather than being retried: the collector
// has seen a prefix of a BMP message stream it can no longer be synchronized
// with, so run() redials and starts a fresh session with Initiation and a full
// dump. MUST be started only by ensureDrain, exactly once per session.
func (ss *senderSession) drainLoop(q *txQueue) {
	defer close(ss.drainDone)

	for {
		if ss.isStopping() {
			return
		}

		buf := q.peek()
		if buf == nil {
			if !q.wait(ss.stopCh) {
				return
			}
			continue
		}

		ss.connMu.Lock()
		c := ss.conn
		ss.connMu.Unlock()
		if c == nil {
			// Not connected. Park rather than reset: run() owns dropping the
			// previous session's bytes (clearConn + resetQueue), and by the time
			// this goroutine is scheduled again the queue may already hold the
			// NEXT connection's priming messages -- resetting here would throw
			// those away while the producer was told they were queued.
			if !q.wait(ss.stopCh) {
				return
			}
			continue
		}

		if err := ss.writeRaw(c, buf); err != nil {
			logger().Info("bmp: sender write failed, resetting session",
				"collector", ss.name, "queued-bytes", q.bytesPending(), "error", err)
			closeLog(c, "sender-drain-write-failed")
			// Compare-and-clear, and drop the queue ONLY if this write was
			// still the current connection's. A write can block for up to
			// writeTimeout, which is long enough for run() to tear this
			// connection down, redial, publish a new one and prime it with a
			// Peer Up for every established peer. Nilling that connection would
			// leave run() holding a socket no producer can reach; resetting the
			// queue would silently discard those primed Peer Ups, and the
			// collector would then receive Route Monitoring for peers it was
			// never told about (RFC 7854 Section 4.10) with nothing logged and
			// nothing to recover it. advance() is guarded the same way, by the
			// queue's own inFlight marker (txqueue.go).
			//
			// Clear-and-reset under ONE connMu hold: done as two steps, run()
			// could publish and prime the next connection in between and the
			// reset would throw those primed Peer Ups away.
			ss.clearConnAndResetIf(c, q)
			continue
		}
		q.advance(len(buf))
	}
}

// waitDrain blocks until the drain goroutine has exited. Caller MUST have
// closed stopCh first (stop() does). No-op when no drain was ever started.
func (ss *senderSession) waitDrain() {
	ss.drainMu.Lock()
	done := ss.drainDone
	q := ss.txq
	ss.drainMu.Unlock()

	if done == nil {
		return
	}
	if q != nil {
		q.wake() // in case it is parked waiting for bytes
	}
	<-done
}
