// Design: docs/architecture/fleet-config.md — heartbeat liveness detection
// Related: client.go — runConnection uses Heartbeat for liveness
// Related: reconnect.go — backoff used after heartbeat timeout

package managed

import (
	"sync"
	"sync/atomic"
	"time"
)

// Heartbeat monitors connection liveness by counting missed pong intervals.
// When missedMax consecutive intervals pass without a RecordPong call,
// the onTimeout callback fires (once). Safe for concurrent use.
//
// Caller MUST call Stop after Start to prevent goroutine leak.
type Heartbeat struct {
	interval  time.Duration
	missedMax int32
	missed    atomic.Int32
	onTimeout func()
	stop      chan struct{}
	once      sync.Once
}

// NewHeartbeat creates a heartbeat that fires onTimeout after missedMax
// consecutive intervals without RecordPong. Call Start() to begin.
func NewHeartbeat(interval time.Duration, missedMax int, onTimeout func()) *Heartbeat {
	return &Heartbeat{
		interval:  interval,
		missedMax: int32(missedMax),
		onTimeout: onTimeout,
		stop:      make(chan struct{}),
	}
}

// Start begins the heartbeat ticker in a background goroutine.
func (h *Heartbeat) Start() {
	go h.run()
}

// Stop cancels the heartbeat ticker.
func (h *Heartbeat) Stop() {
	h.once.Do(func() { close(h.stop) })
}

// RecordPong resets the missed counter. Called when a pong is received.
func (h *Heartbeat) RecordPong() {
	h.missed.Store(0)
}

func (h *Heartbeat) run() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if h.missed.Add(1) >= h.missedMax {
				h.onTimeout()
				return
			}
		case <-h.stop:
			return
		}
	}
}
