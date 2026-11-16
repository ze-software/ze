// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP ownership cleanup

package kernel

import (
	"errors"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A channel-close panic must not bypass the unit's release or erase either
// cleanup cause. Descriptor zero is fake here; no process descriptor is used.
func TestKernelSessionCloseChannelPanicReleasesUnit(t *testing.T) {
	cfg := sessionTestConfig()
	cfg.ChanFD = 0
	handle, streams := runtimeClosePanicSession(t, cfg)
	channelPanic := errors.New("channel close panicked")
	unitFailure := errors.New("unit close failed")
	streams[cfg.ChanFD].closePanic = channelPanic
	streams[cfg.UnitFD].closeErr = unitFailure
	for range 2 {
		err := runtimeClosePanicResult(t, handle)
		if !errors.Is(err, channelPanic) {
			t.Errorf("close lost channel panic: %v", err)
		}
		if !errors.Is(err, unitFailure) {
			t.Errorf("close lost unit cleanup failure: %v", err)
		}
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("descriptor %d closed %d times, want once", fd, stream.closes)
		}
	}
}

// A panic in the second resource must retain the first resource's failure and
// remain visible on every repeated Close, without retrying either descriptor.
func TestKernelSessionCloseUnitPanicRetainsChannelFailure(t *testing.T) {
	cfg := sessionTestConfig()
	handle, streams := runtimeClosePanicSession(t, cfg)
	channelFailure := errors.New("channel close failed")
	unitPanic := errors.New("unit close panicked")
	streams[cfg.ChanFD].closeErr = channelFailure
	streams[cfg.UnitFD].closePanic = unitPanic
	for range 2 {
		err := runtimeClosePanicResult(t, handle)
		if !errors.Is(err, channelFailure) {
			t.Errorf("close lost channel cleanup failure: %v", err)
		}
		if !errors.Is(err, unitPanic) {
			t.Errorf("close lost unit panic: %v", err)
		}
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("descriptor %d closed %d times, want once", fd, stream.closes)
		}
	}
}

// Concurrent closers must all receive the same retained cleanup result, even
// when both resource closers panic. Each fake descriptor remains single-owner.
func TestKernelSessionConcurrentCloseRetainsPanics(t *testing.T) {
	cfg := sessionTestConfig()
	handle, streams := runtimeClosePanicSession(t, cfg)
	channelPanic := errors.New("concurrent channel close panicked")
	unitPanic := errors.New("concurrent unit close panicked")
	streams[cfg.ChanFD].closePanic = channelPanic
	streams[cfg.UnitFD].closePanic = unitPanic
	type result struct {
		err        error
		panicValue any
	}
	results := make(chan result, 8)
	var closers sync.WaitGroup
	for range cap(results) {
		closers.Go(func() {
			var outcome result
			defer func() {
				outcome.panicValue = recover()
				results <- outcome
			}()
			outcome.err = handle.Close()
		})
	}
	closers.Wait()
	close(results)
	var retained error
	for outcome := range results {
		if outcome.panicValue != nil {
			t.Errorf("Close leaked a cleanup panic: %v", outcome.panicValue)
		}
		if !errors.Is(outcome.err, channelPanic) {
			t.Errorf("concurrent close lost channel panic: %v", outcome.err)
		}
		if !errors.Is(outcome.err, unitPanic) {
			t.Errorf("concurrent close lost unit panic: %v", outcome.err)
		}
		if retained == nil {
			retained = outcome.err
		}
		if outcome.err != retained { //nolint:errorlint // Every concurrent closer must receive the exact cached error object.
			t.Errorf("concurrent close returned a different retained result: %v", outcome.err)
		}
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("descriptor %d closed %d times, want once", fd, stream.closes)
		}
	}
}

func runtimeClosePanicSession(t *testing.T, cfg SessionConfig) (ppp.SessionHandle, map[int]*sessionTestStream) {
	t.Helper()
	streams := map[int]*sessionTestStream{
		cfg.ChanFD: {closed: make(chan struct{})},
		cfg.UnitFD: {closed: make(chan struct{})},
	}
	var released []int
	_, handle, err := newSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	return handle, streams
}

func runtimeClosePanicResult(t *testing.T, handle ppp.SessionHandle) (err error) {
	t.Helper()
	defer func() {
		if failure := recover(); failure != nil {
			t.Errorf("Close leaked a cleanup panic: %v", failure)
		}
	}()
	return handle.Close()
}
