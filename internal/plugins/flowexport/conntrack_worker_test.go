package flowexport

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/plugins/flowexport/conntrack"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: Refresh signals an out-of-band dump and coalesces -- repeated calls
// before the run loop consumes the signal leave at most one pending.
// PREVENTS: a burst of AttackDetected events queueing a backlog of full-table
// conntrack dumps (one per event) that would pile CPU/netlink load onto the box
// exactly during an attack.
func TestConntrackWorkerRefreshCoalesces(t *testing.T) {
	w := newConntrackWorker(nil, ConntrackConfig{ActiveTimeout: 60})

	if len(w.refreshCh) != 0 {
		t.Fatalf("new worker: refresh pending = %d, want 0", len(w.refreshCh))
	}
	w.Refresh()
	w.Refresh()
	w.Refresh()
	if got := len(w.refreshCh); got != 1 {
		t.Fatalf("after 3 Refresh calls: pending = %d, want 1 (coalesced)", got)
	}

	// Draining (as the run loop does) re-arms the signal.
	<-w.refreshCh
	w.Refresh()
	if got := len(w.refreshCh); got != 1 {
		t.Fatalf("after drain + Refresh: pending = %d, want 1", got)
	}
}

// VALIDATES: Refresh is a no-op on a nil worker.
// PREVENTS: a panic when the exporter delegates through refreshConntrack before
// the conntrack worker exists (conntrack disabled) or after teardown.
func TestConntrackWorkerRefreshNilSafe(t *testing.T) {
	var w *conntrackWorker
	w.Refresh() // must not panic
}

type flowStubPublisher struct {
	mu     sync.Mutex
	raises []sdk.Issue
	clears []struct{ code, subject string }
}

func (p *flowStubPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.raises = append(p.raises, issue)
	return nil
}

func (p *flowStubPublisher) ClearIssue(_ context.Context, code, subject string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clears = append(p.clears, struct{ code, subject string }{code: code, subject: subject})
	return nil
}

func (p *flowStubPublisher) raisedByCode(code string) []sdk.Issue {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []sdk.Issue
	for _, r := range p.raises {
		if r.Code == code {
			out = append(out, r)
		}
	}
	return out
}

func (p *flowStubPublisher) clearedByCode(code string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.clears {
		if c.code == code {
			n++
		}
	}
	return n
}

type stubConntrackReader struct{}

func (stubConntrackReader) Dump() ([]conntrack.FlowEntry, error) { return nil, nil }
func (stubConntrackReader) Close() error                         { return nil }

type stubDestroyListener struct {
	closed chan struct{}
	once   sync.Once
}

func newStubDestroyListener() *stubDestroyListener {
	return &stubDestroyListener{closed: make(chan struct{})}
}

func (s *stubDestroyListener) Read() ([]conntrack.FlowEntry, error) {
	<-s.closed
	return nil, errors.New("closed")
}

func (s *stubDestroyListener) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

type flowSyncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *flowSyncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *flowSyncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise verifies AC-2
// and AC-3 for flowexport: conntrackWorker.Start raises a ReportWarning when
// NewDestroyListener fails and clears it on Stop/recovery, and
// samplingWorker.Stop raises a ReportError only for interfaces where
// SetupSampling succeeded.
//
// VALIDATES: spec-dropped-error-4-system-plugins AC-2, AC-3 (flowexport).
// PREVENTS: conntrack dump-only degradation and tc sampling teardown failures
// dropping below WARN with no report-bus visibility.
func TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise(t *testing.T) {
	t.Run("conntrack-destroy-unavailable", func(t *testing.T) {
		origReader := newConntrackReaderFn
		origDestroy := newConntrackDestroyListenerFn
		origLogger := loggerPtr.Load()
		t.Cleanup(func() {
			newConntrackReaderFn = origReader
			newConntrackDestroyListenerFn = origDestroy
			loggerPtr.Store(origLogger)
		})

		var logBuf flowSyncBuf
		setLogger(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))

		newConntrackReaderFn = func() (conntrackReader, error) { return stubConntrackReader{}, nil }
		newConntrackDestroyListenerFn = func() (conntrackDestroyListener, error) {
			return nil, errors.New("operation not permitted")
		}

		pub := &flowStubPublisher{}
		q := issuequeue.Start(pub)
		t.Cleanup(q.Stop)

		w := newConntrackWorker(nil, ConntrackConfig{ActiveTimeout: 60})
		w.setIssueQueue(q)
		w.Start()
		q.Flush()

		warnOut := logBuf.String()
		for _, want := range []string{"level=WARN", codeDoctorFlowexportConntrackDestroyUnavailable, "report=queued"} {
			if !strings.Contains(warnOut, want) {
				t.Errorf("WARN log missing %q:\n%s", want, warnOut)
			}
		}

		raised := pub.raisedByCode(codeDoctorFlowexportConntrackDestroyUnavailable)
		if len(raised) != 1 {
			t.Fatalf("raised issues = %d, want 1", len(raised))
		}
		if raised[0].Severity != rpc.ReportWarning {
			t.Errorf("severity = %q, want %q", raised[0].Severity, rpc.ReportWarning)
		}
		if raised[0].Subject != "conntrack-destroy" {
			t.Errorf("subject = %q, want conntrack-destroy", raised[0].Subject)
		}
		if raised[0].Fix == "" {
			t.Error("raised issue has empty Fix")
		}

		// Overlapping reload while destroy listener stays unavailable: starting
		// the replacement worker before stopping the previous one (as register.go
		// configure does) must not let the old worker's Stop erase the new
		// worker's standing warning.
		wReload := newConntrackWorker(nil, ConntrackConfig{ActiveTimeout: 60})
		wReload.setIssueQueue(q)
		wReload.Start()
		q.Flush()

		w.Stop()
		q.Flush()
		if got := pub.clearedByCode(codeDoctorFlowexportConntrackDestroyUnavailable); got != 0 {
			t.Errorf("clear count while replacement worker still dump-only = %d, want 0", got)
		}

		wReload.Stop()
		q.Flush()
		if got := pub.clearedByCode(codeDoctorFlowexportConntrackDestroyUnavailable); got != 1 {
			t.Errorf("clear count after last dump-only worker stopped = %d, want 1", got)
		}

		// Recovery on a subsequent Start with a working destroy listener clears too.
		newConntrackDestroyListenerFn = func() (conntrackDestroyListener, error) {
			return newStubDestroyListener(), nil
		}
		w2 := newConntrackWorker(nil, ConntrackConfig{ActiveTimeout: 60})
		w2.setIssueQueue(q)
		w2.Start()
		q.Flush()
		w2.Stop()
		q.Flush()
		if got := len(pub.raisedByCode(codeDoctorFlowexportConntrackDestroyUnavailable)); got != 2 {
			t.Errorf("recovery Start raised unexpectedly: got %d, want 2", got)
		}
	})

	t.Run("sampling-teardown-failed", testSamplingTeardownWarnAndRaise)
}
