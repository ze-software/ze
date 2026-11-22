// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting reads the original network through its capture.
// Related: acct_counter_source_test.go -- the questions this fixture serves
package l2tpauthradius

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// acctUnitCapture stands in for one network-qualified capture. Counters reads
// the original PPP unit now through read; TerminalCounters waits for the
// measurement PPP takes when the network ends, which the test supplies with
// measure. Embedding CapturedSession makes any other method a test reaches
// fail loudly rather than answer a fake success.
type acctUnitCapture struct {
	l2tp.CapturedSession
	read     func() (ppp.SessionCounters, error)
	mu       sync.Mutex
	reads    int
	terminal l2tpevents.TrafficSample
	measured chan struct{}
	once     sync.Once
	// awaiting closes when the first TerminalCounters call begins.
	awaiting     chan struct{}
	awaitingOnce sync.Once
	// defect makes TerminalCounters panic, as a Ze defect would.
	defect atomic.Bool
}

func newAcctUnitCapture(read func() (ppp.SessionCounters, error)) *acctUnitCapture {
	return &acctUnitCapture{read: read, measured: make(chan struct{}), awaiting: make(chan struct{})}
}

// fixedAcctUnit answers one reading for every live read.
func fixedAcctUnit(counters ppp.SessionCounters, err error) func() (ppp.SessionCounters, error) {
	return func() (ppp.SessionCounters, error) { return counters, err }
}

func (c *acctUnitCapture) Counters() (ppp.SessionCounters, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	return c.read()
}

func (c *acctUnitCapture) liveReads() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

func (c *acctUnitCapture) TerminalCounters(ctx context.Context) (l2tpevents.TrafficSample, error) {
	c.awaitingOnce.Do(func() { close(c.awaiting) })
	if c.defect.Load() {
		panic("boom")
	}
	select {
	case <-c.measured:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.terminal, nil
	case <-ctx.Done():
		return l2tpevents.TrafficSample{}, ctx.Err()
	}
}

// measure publishes the network's terminal sample once, as PPP does before it
// closes the original unit.
func (c *acctUnitCapture) measure(sample l2tpevents.TrafficSample) {
	c.once.Do(func() {
		c.mu.Lock()
		c.terminal = sample
		c.mu.Unlock()
		close(c.measured)
	})
}

// bindAcctUnit binds capture to sess as the accounting task would.
func bindAcctUnit(sess *acctSession, capture *acctUnitCapture) {
	sess.capture.Store(&acctCaptureRef{session: capture})
}

func unitCounters(rxBytes, txBytes, rxPackets, txPackets uint64) ppp.SessionCounters {
	return ppp.SessionCounters{RxBytes: rxBytes, TxBytes: txBytes, RxPackets: rxPackets, TxPackets: txPackets}
}

func measuredSample(rxBytes, txBytes, rxPackets, txPackets uint64) l2tpevents.TrafficSample {
	return l2tpevents.MeasuredTrafficCounters(l2tpevents.TrafficTotals{
		RxBytes: rxBytes, TxBytes: txBytes, RxPackets: rxPackets, TxPackets: txPackets,
	})
}

// useAcctCaptures answers every accounting capture with bind until t ends.
// Install it before the accountant, so it is restored after the tasks join.
func useAcctCaptures(t *testing.T, bind func(l2tp.SessionSelector) (l2tp.CapturedSession, error)) {
	t.Helper()
	saved := acctCaptureSession
	acctCaptureSession = bind
	t.Cleanup(func() { acctCaptureSession = saved })
}

func firstNetworkLifetime(t *testing.T) l2tpevents.NetworkLifetime {
	t.Helper()
	network, err := l2tpevents.NetworkLifetime{}.Next()
	if err != nil {
		t.Fatal(err)
	}
	return network
}
