// Design: docs/architecture/decisions/001-pull-model-metrics.md -- serialized series lifetime
package metrics

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSeriesCustodySerializesLastDelete proves a successor admission cannot
// overtake the original final deletion. Channels hold deletion in flight; the
// mutex probe verifies the stated acquisition-lock contract without a sleep.
func TestSeriesCustodySerializesLastDelete(t *testing.T) {
	var custody SeriesCustody
	var physical atomic.Int64
	var removals atomic.Int64
	var deleteComplete atomic.Bool
	var deleteUnlocked atomic.Bool
	deleteEntered := make(chan struct{})
	allowDelete := make(chan struct{})
	var allowOnce sync.Once
	unblock := func() { allowOnce.Do(func() { close(allowDelete) }) }
	remove := func(...string) bool {
		if removals.Add(1) == 1 {
			// Inspect the private mutex, not a custody method: raw removal
			// itself MUST NOT call back into custody while this lock is held.
			if custody.mu.TryLock() {
				custody.mu.Unlock()
				deleteUnlocked.Store(true)
			}
			close(deleteEntered)
			<-allowDelete
		}
		physical.Store(0)
		deleteComplete.Store(true)
		return true
	}
	old, err := custody.Retain([]string{"alice"}, nil, remove)
	if err != nil {
		t.Fatal(err)
	}
	physical.Store(1)
	releaseDone := make(chan struct{})
	go func() {
		old.Release()
		close(releaseDone)
	}()
	t.Cleanup(func() {
		unblock()
		waitSeriesCustodySignal(t, releaseDone)
	})
	waitSeriesCustodySignal(t, deleteEntered)

	var successor SeriesLease
	var retainErr error
	retainStarted := make(chan struct{})
	retainDone := make(chan struct{})
	go func() {
		close(retainStarted)
		successor, retainErr = custody.Retain([]string{"alice"}, func(...string) error {
			if !deleteComplete.Load() {
				return errors.New("successor admitted before the original deletion completed")
			}
			physical.Store(2)
			return nil
		}, remove)
		close(retainDone)
	}()
	t.Cleanup(func() {
		unblock()
		waitSeriesCustodySignal(t, retainDone)
		successor.Release()
	})
	waitSeriesCustodySignal(t, retainStarted)
	unblock()
	waitSeriesCustodySignal(t, releaseDone)
	waitSeriesCustodySignal(t, retainDone)
	if deleteUnlocked.Load() {
		t.Fatal("original deletion ran outside the acquisition lock")
	}
	if retainErr != nil {
		t.Fatal(retainErr)
	}
	old.Release()
	if custody.Delete([]string{"alice"}, remove) {
		t.Fatal("ordinary Delete removed the retained successor")
	}
	if got := physical.Load(); got != 2 {
		t.Fatal("original deletion or stale release erased the successor", got)
	}
	successor.Release()
	if got := physical.Load(); got != 0 {
		t.Fatal("successor final release left its physical series", got)
	}
	if got := removals.Load(); got != 2 {
		t.Fatal("physical deletion did not run exactly once per generation", got)
	}
}

// TestPrometheusSeriesCustodyConcurrentCopies proves concurrent releases of
// one copied lease preserve another holder's actual exported measurement.
func TestPrometheusSeriesCustodyConcurrentCopies(t *testing.T) {
	reg := NewPrometheusRegistry()
	vector := reg.GaugeVec("custody_concurrent", "Concurrent series holders.", []string{"login"})
	retainer := metricTestSeriesRetainer(t, vector)
	old := retainMetricTestSeries(t, retainer, "alice")
	t.Cleanup(old.Release)
	current := retainMetricTestSeries(t, retainer, "alice")
	t.Cleanup(current.Release)
	vector.With("alice").Set(41)
	start := make(chan struct{})
	var startOnce sync.Once
	unblock := func() { startOnce.Do(func() { close(start) }) }
	const copies = 16
	var workers sync.WaitGroup
	workers.Add(copies)
	for range copies {
		go func(lease SeriesLease) {
			defer workers.Done()
			<-start
			lease.Release()
		}(old)
	}
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		unblock()
		waitSeriesCustodySignal(t, done)
	})
	unblock()
	waitSeriesCustodySignal(t, done)
	if vector.Delete("alice") {
		t.Fatal("concurrent copied releases erased the current holder")
	}
	if got := custodyScrape(t, reg); !strings.Contains(got, `custody_concurrent{login="alice"} 41`) {
		t.Fatal("concurrent copied releases lost the original measurement", got)
	}
	current.Release()
	if got := custodyScrape(t, reg); strings.Contains(got, `login="alice"`) {
		t.Fatal("current final release retained its series", got)
	}
}

// The timer is a deadlock bound, never evidence that an operation was blocked.
// Normal progress and the ordering assertions use only channel handshakes.
func waitSeriesCustodySignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-signal:
	case <-timer.C:
		t.Fatal("series custody operation did not finish")
	}
}
