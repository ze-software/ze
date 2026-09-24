// Design: docs/guide/rpki.md -- RTR data expiry independent of connection lifetime
// RFC: rfc/short/rfc8210.md -- Section 6, lifetime of successfully fetched data
package rpki

import (
	"sync"
	"time"
)

// rtrDataLease belongs to the published payloads, not to a TCP connection. It
// survives polling, cache failover and config/credential rotation. One timer
// expires the data even while a cache stalls or route revalidation blocks.
// Session publication takes its own mu before this mu; callbacks run after both
// are released and inspect current data, never an obsolete captured verdict.
type rtrDataLease struct {
	mu           sync.Mutex
	deadline     time.Time
	generation   uint64
	timer        *time.Timer
	stopped      bool
	callbacks    sync.WaitGroup
	cache        *ROACache
	aspaCache    *aSPACache
	onROAChange  func()
	onASPAChange func([]uint32)
}

func newRTRDataLease(cache *ROACache, aspa *aSPACache, roaChanged func(), aspaChanged func([]uint32)) *rtrDataLease {
	return &rtrDataLease{cache: cache, aspaCache: aspa, onROAChange: roaChanged, onASPAChange: aspaChanged}
}

// renewLocked is called only after all payloads in an End of Data response
// have been applied. Section 6 starts this countdown at that End of Data.
func (l *rtrDataLease) renewLocked(received time.Time, interval time.Duration) {
	l.deadline = received.Add(interval)
	delay := time.Until(l.deadline)
	if l.timer == nil {
		l.timer = time.AfterFunc(delay, func() { l.expire(time.Now()) })
	} else {
		l.timer.Reset(delay)
	}
}

func (l *rtrDataLease) current(generation uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.stopped && !l.deadline.IsZero() && generation == l.generation && time.Now().Before(l.deadline)
}

// expire also serves the config handoff: it checks the old data's deadline
// before retained routes can be revalidated after a long disabled interval.
func (l *rtrDataLease) expire(now time.Time) {
	l.mu.Lock()
	if l.stopped || l.deadline.IsZero() || now.Before(l.deadline) {
		l.mu.Unlock()
		return
	}
	l.callbacks.Add(1)
	l.deadline = time.Time{}
	l.generation++
	l.cache.Clear()
	var changed []uint32
	if l.aspaCache != nil {
		changed = l.aspaCache.Replace(nil)
	}
	if metrics := rpkiMetricsPtr.Load(); metrics != nil {
		metrics.vrpsCached.Set(0)
		metrics.sessionsActive.Set(0)
	}
	l.mu.Unlock()
	defer l.callbacks.Done()
	if len(changed) != 0 && l.onASPAChange != nil {
		l.onASPAChange(changed)
	}
	if l.onROAChange != nil {
		l.onROAChange()
	}
}

func (l *rtrDataLease) stop() {
	l.mu.Lock()
	l.stopped = true
	if l.timer != nil {
		l.timer.Stop()
	}
	l.mu.Unlock()
	// expire registers before releasing mu and cannot register after stopped,
	// so no new Add races this Wait during plugin shutdown.
	l.callbacks.Wait()
}

// prepareQuery rejects an obsolete serial base before encoding the query.
// Expiry during a query is checked again at End of Data, before publication.
func (s *RTRSession) prepareQuery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dataLease != nil && !s.dataLease.current(s.dataGeneration) {
		s.serial = 0
	}
	s.fullSync = s.serial == 0
}

func (s *RTRSession) dataLeaseLocked() *rtrDataLease {
	if s.dataLease == nil {
		s.dataLease = newRTRDataLease(s.cache, s.aspaCache, func() {
			if s.onROAChange != nil {
				s.onROAChange()
			}
		}, func(changed []uint32) {
			if s.onASPAChange != nil {
				s.onASPAChange(changed)
			}
		})
	}
	return s.dataLease
}
