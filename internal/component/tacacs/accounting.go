// Design: (none -- new TACACS+ component)
// Overview: client.go -- TACACS+ TCP client
// Related: authenticator.go -- auth bridge (sibling wrapper around client)

// TacacsAccountant implements aaa.Accountant for TACACS+ accounting.
// RFC 8907 Section 7. Sends START/STOP records for command execution.
package tacacs

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// acctMsg is an accounting request queued for the background worker.
type acctMsg struct {
	req *AcctRequest
}

// TacacsAccountant sends TACACS+ accounting records for command execution.
// It implements aaa.Accountant.
// Accounting failures are logged locally and never block command execution.
//
// Lifecycle: call Start() before use, Stop() on shutdown. Start() launches
// one long-lived worker goroutine that reads from the queue. Stop() is safe
// to call multiple times; enqueue after Stop is safe (the record is dropped
// silently, never panics on a closed channel). Stop signals the worker via
// stopCh and never closes the queue, so the send-path stays panic-free.
type TacacsAccountant struct {
	client   *TacacsClient
	logger   *slog.Logger
	taskSeq  atomic.Uint64 // monotonic task ID generator
	queue    chan acctMsg  // buffered channel for the worker (never closed)
	done     chan struct{} // closed when worker exits
	stopCh   chan struct{} // closed by Stop; checked by worker and enqueue
	stopOnce sync.Once     // guards close(stopCh) against double-Stop
}

// NewTacacsAccountant creates a TacacsAccountant.
// MUST call Start() to launch the worker before sending records.
func NewTacacsAccountant(client *TacacsClient, logger *slog.Logger) *TacacsAccountant {
	if logger == nil {
		logger = slog.Default()
	}
	return &TacacsAccountant{
		client: client,
		logger: logger,
		queue:  make(chan acctMsg, 64),
		done:   make(chan struct{}),
		stopCh: make(chan struct{}),
	}
}

// Start launches the background worker that sends accounting records.
// The worker runs until Stop() is called.
func (a *TacacsAccountant) Start() {
	go a.worker()
}

// Stop signals the worker to exit and blocks until it does.
// Idempotent: safe to call multiple times. Never closes the queue, so
// concurrent enqueue calls cannot panic.
func (a *TacacsAccountant) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
	<-a.done
}

// worker reads from the queue and sends each request to the TACACS+ server.
// Exits when stopCh is closed. Records still queued at stop time are dropped
// (accounting is best-effort) and at most one in-flight processOne call
// completes before the worker exits, bounding Stop()'s blocking window to
// a single TACACS+ client Timeout regardless of queue depth.
//
// One subtlety for tests that count "records sent": when Stop runs while
// the worker has just received a message from the queue, the message is
// consumed (removed from the channel) but dropped before reaching the
// server. A test that observes "N messages enqueued, expect N sent" must
// either avoid stopping mid-drain or tolerate dropped tail messages.
func (a *TacacsAccountant) worker() {
	defer close(a.done)
	for {
		select {
		case <-a.stopCh:
			return
		case msg := <-a.queue:
			// Re-check stopCh before calling processOne so a Stop that
			// fires during a drain of a long queue does not have to wait
			// out queue_size * Timeout worth of SendAccounting calls
			// before the worker notices.
			if a.isStopped() {
				return
			}
			a.processOne(msg)
		}
	}
}

// processOne sends a single accounting request and logs the outcome.
func (a *TacacsAccountant) processOne(msg acctMsg) {
	reply, err := a.client.SendAccounting(msg.req)
	if err != nil {
		a.logger.Warn("TACACS+ accounting failed",
			"user", msg.req.User, "error", err)
		return
	}
	if reply.Status != AcctStatusSuccess {
		a.logger.Warn("TACACS+ accounting rejected",
			"user", msg.req.User, "status", reply.Status)
	}
}

// enqueue adds an accounting request to the worker queue.
// Returns false if the queue is full (record dropped) or the accountant is
// stopped. Never panics: the queue is never closed, and a stopCh check
// drops records arriving after Stop.
//
// Concurrency note: a benign race exists between the isStopped() check and
// the channel send. If Stop runs between the two, the send still succeeds
// (queue is buffered, not closed) and the message sits in the queue until
// the accountant is garbage-collected. No panic, no observable leak beyond
// normal GC -- the worker has already exited so the message is never
// processed, but it was a best-effort record anyway.
func (a *TacacsAccountant) enqueue(req *AcctRequest) bool {
	if a.isStopped() {
		return false
	}
	select {
	case a.queue <- acctMsg{req: req}:
		return true
	default: // queue full -- drop record, logged by caller
		return false
	}
}

// isStopped returns true if Stop has been called.
// Non-blocking check against stopCh.
func (a *TacacsAccountant) isStopped() bool {
	select {
	case <-a.stopCh:
		return true
	default: // stopCh not yet closed; accountant is live
		return false
	}
}

// CommandStart sends an accounting START record. Returns a task ID for correlation.
// Never blocks: enqueues to the worker. Drops with a warning if the queue is full.
func (a *TacacsAccountant) CommandStart(username, remoteAddr, command string) string {
	taskID := fmt.Sprintf("%d", a.taskSeq.Add(1))

	req := &AcctRequest{
		Flags:         AcctFlagStart,
		AuthenMethod:  0x06, // TACACS+
		PrivLvl:       1,    // default user
		AuthenType:    0x01, // ASCII
		AuthenService: 0x01, // login
		User:          username,
		Port:          "ssh",
		RemAddr:       remoteAddr,
		Args: []string{
			"task_id=" + taskID,
			"service=shell",
			"cmd=" + command,
			"start_time=" + fmt.Sprintf("%d", time.Now().Unix()),
		},
	}

	if !a.enqueue(req) {
		a.logger.Warn("TACACS+ accounting queue full, dropping START",
			"username", username, "command", command)
	}

	return taskID
}

// CommandStop sends an accounting STOP record.
// Never blocks: enqueues to the worker. Drops with a warning if the queue is full.
func (a *TacacsAccountant) CommandStop(taskID, username, remoteAddr, command string) {
	req := &AcctRequest{
		Flags:         AcctFlagStop,
		AuthenMethod:  0x06, // TACACS+
		PrivLvl:       1,
		AuthenType:    0x01,
		AuthenService: 0x01,
		User:          username,
		Port:          "ssh",
		RemAddr:       remoteAddr,
		Args: []string{
			"task_id=" + taskID,
			"service=shell",
			"cmd=" + command,
			"stop_time=" + fmt.Sprintf("%d", time.Now().Unix()),
		},
	}

	if !a.enqueue(req) {
		a.logger.Warn("TACACS+ accounting queue full, dropping STOP",
			"username", username, "command", command)
	}
}
