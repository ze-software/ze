// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

import (
	"sync"
	"sync/atomic"
	"time"
)

// MaxPendingPerProcess limits pending requests to prevent memory exhaustion.
const MaxPendingPerProcess = 100

// PendingRequest represents an in-flight plugin command request.
type PendingRequest struct {
	Serial   string         // Alpha serial (a, b, bcd, ...)
	Command  string         // Matched command name
	Process  *Process       // Target process
	Timeout  time.Duration  // Timeout for response
	RespChan chan *Response // Channel to deliver response
	timer    *time.Timer    // Timeout timer
}

// PendingRequests tracks in-flight plugin command requests.
// Thread-safe for concurrent access.
type PendingRequests struct {
	mu        sync.RWMutex
	next      atomic.Uint64                // Next serial number
	requests  map[string]*PendingRequest   // serial → pending
	byProcess map[*Process]map[string]bool // process → serials (for limit)
}

// NewPendingRequests creates a new pending requests tracker.
func NewPendingRequests() *PendingRequests {
	return &PendingRequests{
		requests:  make(map[string]*PendingRequest),
		byProcess: make(map[*Process]map[string]bool),
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
			// Send error to channel
			if req.RespChan != nil {
				select {
				case req.RespChan <- &Response{
					Status: "error",
					Data:   "too many pending requests",
				}:
				default:
				}
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
// Returns true if the serial was found (response delivered).
func (p *PendingRequests) Complete(serial string, resp *Response) bool {
	p.mu.Lock()
	req, ok := p.requests[serial]
	if !ok {
		p.mu.Unlock()
		return false
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

	// Deliver response
	if req.RespChan != nil {
		select {
		case req.RespChan <- resp:
		default:
		}
	}

	return true
}

// Partial delivers a partial response (streaming) and resets the timeout.
// Returns true if the serial was found.
func (p *PendingRequests) Partial(serial string, resp *Response) bool {
	p.mu.Lock()
	req, ok := p.requests[serial]
	if !ok {
		p.mu.Unlock()
		return false
	}

	// Reset timer
	if req.timer != nil {
		req.timer.Stop()
		req.timer = time.AfterFunc(req.Timeout, func() {
			p.timeout(serial)
		})
	}
	p.mu.Unlock()

	// Deliver partial response
	if req.RespChan != nil {
		select {
		case req.RespChan <- resp:
		default:
		}
	}

	return true
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

	// Send timeout error
	if req.RespChan != nil {
		select {
		case req.RespChan <- &Response{
			Status: "error",
			Data:   "command timed out",
		}:
		default:
		}
	}
}

// CancelAll cancels all pending requests for a process (process death).
// Sends error response to all waiting clients.
func (p *PendingRequests) CancelAll(proc *Process) {
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

	// Send error responses
	for _, req := range toCancel {
		if req.RespChan != nil {
			select {
			case req.RespChan <- &Response{
				Status: "error",
				Data:   "process died",
			}:
			default:
			}
		}
	}
}

// Count returns the number of pending requests for a process.
func (p *PendingRequests) Count(proc *Process) int {
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
