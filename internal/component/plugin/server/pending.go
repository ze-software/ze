// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// MaxPendingPerProcess limits pending requests to prevent memory exhaustion.
const MaxPendingPerProcess = 100

// ErrPendingUnknownSerial says the serial names no in-flight request: it
// completed, timed out, or died with its process before this delivery.
var ErrPendingUnknownSerial = errors.New("pending request: unknown serial")

// ErrPendingUndeliverable says the waiting caller did not take the response
// within the request's own timeout. The response was NOT delivered, so a
// producer that gets it MUST treat the row as unsent and MUST NOT count it.
var ErrPendingUndeliverable = errors.New("pending request: caller is not reading its responses")

// PendingRequest represents an in-flight plugin command request.
type PendingRequest struct {
	Serial   string                // Alpha serial (a, b, bcd, ...)
	Command  string                // Matched command name
	Process  *process.Process      // Target process
	Timeout  time.Duration         // Timeout for response
	RespChan chan *plugin.Response // Channel to deliver response
	timer    *time.Timer           // Timeout timer
}

// PendingRequests tracks in-flight plugin command requests.
// Thread-safe for concurrent access.
type PendingRequests struct {
	mu        sync.RWMutex
	next      atomic.Uint64                        // Next serial number
	requests  map[string]*PendingRequest           // serial → pending
	byProcess map[*process.Process]map[string]bool // process → serials (for limit)
}

// newPendingRequests creates a new pending requests tracker.
func newPendingRequests() *PendingRequests {
	return &PendingRequests{
		requests:  make(map[string]*PendingRequest),
		byProcess: make(map[*process.Process]map[string]bool),
	}
}

// Add registers a new pending request and starts the timeout timer.
// Returns the assigned alpha serial, or empty string if limit exceeded.
// If limit exceeded, sends error response to RespChan.
func (p *PendingRequests) Add(req *PendingRequest) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check per-process limit
	if procSerials, ok := p.byProcess[req.Process]; ok {
		if len(procSerials) >= MaxPendingPerProcess {
			// This runs under p.mu, so the delivery cannot wait for room. The
			// caller learns of the refusal from the empty serial as well, and
			// the log says so when its channel had no room for the reason.
			if !tryDeliver(req, &plugin.Response{
				Status: plugin.StatusError,
				Error:  "too many pending requests",
			}) {
				slog.Warn("plugin pending: refusal undelivered, caller is not reading",
					"command", req.Command, "limit", MaxPendingPerProcess)
			}
			return ""
		}
	}

	// Generate alpha serial
	n := p.next.Add(1) - 1
	serial := encodeAlphaSerial(n)
	req.Serial = serial

	// Start timeout timer
	req.timer = time.AfterFunc(req.Timeout, func() {
		p.timeout(serial)
	})

	// Register
	p.requests[serial] = req

	// Track by process
	if p.byProcess[req.Process] == nil {
		p.byProcess[req.Process] = make(map[string]bool)
	}
	p.byProcess[req.Process][serial] = true

	return serial
}

// Complete delivers a final response and removes the request.
//
// It returns nil when the response reached the waiting caller,
// ErrPendingUnknownSerial when no request holds that serial, and
// ErrPendingUndeliverable when the caller did not take the response within the
// request's timeout. The last one means the response was NOT delivered.
func (p *PendingRequests) Complete(serial string, resp *plugin.Response) error {
	p.mu.Lock()
	req, ok := p.requests[serial]
	if !ok {
		p.mu.Unlock()
		return ErrPendingUnknownSerial
	}

	// Stop timer
	if req.timer != nil {
		req.timer.Stop()
	}

	// Remove from maps
	delete(p.requests, serial)
	if procSerials, exists := p.byProcess[req.Process]; exists {
		delete(procSerials, serial)
		if len(procSerials) == 0 {
			delete(p.byProcess, req.Process)
		}
	}
	p.mu.Unlock()

	return deliver(req, resp)
}

// Partial delivers one response of a stream and resets the timeout, so an
// answer that arrives row by row does not expire between its rows.
//
// It returns what Complete returns, and the same rule holds: a row that comes
// back ErrPendingUndeliverable did NOT reach the caller. A producer of records
// MUST stop the walk and report a short answer rather than count that row,
// because a count that includes a row nobody received is worse than a slow
// answer (R-9).
func (p *PendingRequests) Partial(serial string, resp *plugin.Response) error {
	p.mu.Lock()
	req, ok := p.requests[serial]
	if !ok {
		p.mu.Unlock()
		return ErrPendingUnknownSerial
	}

	// Reset timer
	if req.timer != nil {
		req.timer.Stop()
		req.timer = time.AfterFunc(req.Timeout, func() {
			p.timeout(serial)
		})
	}
	p.mu.Unlock()

	return deliver(req, resp)
}

// deliver hands resp to the caller waiting on req and reports whether it
// arrived. A full channel makes the producer wait rather than lose the
// response: the wait is bounded by the request's own timeout, which is how long
// its caller already said it would wait for this answer.
//
// The caller MUST NOT hold p.mu, because this waits.
//
// A response that cannot be delivered is reported, never discarded. Silence
// loses a row while the terminator still counts it, which makes the answer
// wrong rather than slow.
func deliver(req *PendingRequest, resp *plugin.Response) error {
	if req.RespChan == nil {
		return nil
	}
	if tryDeliver(req, resp) {
		return nil
	}

	wait := time.NewTimer(req.Timeout)
	defer wait.Stop()

	select {
	case req.RespChan <- resp:
		return nil
	case <-wait.C:
		slog.Warn("plugin pending: response undelivered, caller is not reading",
			"serial", req.Serial, "command", req.Command, "timeout", req.Timeout)
		return ErrPendingUndeliverable
	}
}

// tryDeliver attempts the delivery a caller with room takes at once, and
// reports whether it happened. It is the path that costs no timer.
func tryDeliver(req *PendingRequest, resp *plugin.Response) bool {
	if req.RespChan == nil {
		return true
	}
	select {
	case req.RespChan <- resp:
		return true
	default:
		return false
	}
}

// timeout handles a request timeout.
func (p *PendingRequests) timeout(serial string) {
	p.mu.Lock()
	req, ok := p.requests[serial]
	if !ok {
		p.mu.Unlock()
		return
	}

	// Remove from maps
	delete(p.requests, serial)
	if procSerials, exists := p.byProcess[req.Process]; exists {
		delete(procSerials, serial)
		if len(procSerials) == 0 {
			delete(p.byProcess, req.Process)
		}
	}
	p.mu.Unlock()

	// Send timeout error. The request's own timeout has just expired, so this
	// delivery waits no longer than that again before it says so.
	if err := deliver(req, &plugin.Response{
		Status: plugin.StatusError,
		Error:  "command timed out",
	}); err != nil {
		slog.Warn("plugin pending: timeout notice undelivered",
			"serial", serial, "command", req.Command, "error", err)
	}
}

// CancelAll cancels all pending requests for a process (process death).
// Sends error response to all waiting clients.
func (p *PendingRequests) CancelAll(proc *process.Process) {
	p.mu.Lock()
	procSerials, ok := p.byProcess[proc]
	if !ok {
		p.mu.Unlock()
		return
	}

	// Collect requests to cancel
	toCancel := make([]*PendingRequest, 0, len(procSerials))
	for serial := range procSerials {
		if req, exists := p.requests[serial]; exists {
			if req.timer != nil {
				req.timer.Stop()
			}
			delete(p.requests, serial)
			toCancel = append(toCancel, req)
		}
	}
	delete(p.byProcess, proc)
	p.mu.Unlock()

	// Send error responses. Each delivery waits at most that request's own
	// timeout, so one caller that stopped reading cannot hold up the rest.
	for _, req := range toCancel {
		if err := deliver(req, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "process died",
		}); err != nil {
			slog.Warn("plugin pending: process-death notice undelivered",
				"serial", req.Serial, "command", req.Command, "error", err)
		}
	}
}

// Count returns the number of pending requests for a process.
func (p *PendingRequests) Count(proc *process.Process) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byProcess[proc])
}

// Total returns the total number of pending requests.
func (p *PendingRequests) Total() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.requests)
}
