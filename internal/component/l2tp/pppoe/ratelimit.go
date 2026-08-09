// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PADI rate limiting

package pppoe

import "sync"

// nowNano returns the current time in nanoseconds. Tests override this
// via export_test.go for deterministic behavior.
var nowNano = func() int64 {
	return nowUnix() * 1_000_000_000
}

const nanoPerSec = int64(1_000_000_000)

// PADILimiter enforces per-interface PADI rate limiting and per-MAC
// deduplication. Modeled after accel-ppp's check_padi_limit.
type PADILimiter struct {
	mu          sync.Mutex
	limit       int
	recent      map[[EthALen]byte]int64
	count       int
	windowStart int64
}

// NewPADILimiter creates a limiter that allows at most limit PADIs per
// second per interface. A limit of 0 or negative disables rate limiting.
func NewPADILimiter(limit int) *PADILimiter {
	return &PADILimiter{
		limit:  limit,
		recent: make(map[[EthALen]byte]int64),
	}
}

// Check returns true if the PADI from mac should be processed. Returns
// false if the rate limit is exceeded or if this MAC already sent a
// PADI within the current 1-second window (dedup).
func (l *PADILimiter) Check(mac [EthALen]byte) bool {
	if l.limit <= 0 {
		return true
	}

	now := nowNano()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.windowStart == 0 || now-l.windowStart >= nanoPerSec {
		l.windowStart = now
		l.count = 0
		l.sweepLocked(now)
	}

	if l.count >= l.limit {
		return false
	}

	if lastSeen, exists := l.recent[mac]; exists && now-lastSeen < nanoPerSec {
		return false
	}

	l.recent[mac] = now
	l.count++
	return true
}

// sweepLocked removes MAC entries older than 1 second. Called at
// window transitions to prevent unbounded map growth.
func (l *PADILimiter) sweepLocked(now int64) {
	for mac, ts := range l.recent {
		if now-ts >= nanoPerSec {
			delete(l.recent, mac)
		}
	}
}
