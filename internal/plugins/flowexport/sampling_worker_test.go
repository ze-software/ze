// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sampling worker teardown warning tests

package flowexport

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/plugins/flowexport/sampling"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type stubPsampleReader struct {
	closed chan struct{}
	once   sync.Once
}

func newStubPsampleReader() *stubPsampleReader {
	return &stubPsampleReader{closed: make(chan struct{})}
}

func (s *stubPsampleReader) Read() (sampling.SampledPacket, error) {
	<-s.closed
	return sampling.SampledPacket{}, errors.New("closed")
}

func (s *stubPsampleReader) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

// testSamplingTeardownWarnAndRaise is driven by
// TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise.
//
// VALIDATES: spec-dropped-error-4-system-plugins AC-2, AC-3 (flowexport
// samplingWorker.Stop).
// PREVENTS: tc sample teardown failures dropping at Debug on installed
// interfaces, and spurious warnings on interfaces where SetupSampling never
// succeeded.
func testSamplingTeardownWarnAndRaise(t *testing.T) {
	origSetup := setupSamplingFn
	origRemove := removeSamplingFn
	origReader := newPsampleReaderFn
	origLogger := loggerPtr.Load()
	t.Cleanup(func() {
		setupSamplingFn = origSetup
		removeSamplingFn = origRemove
		newPsampleReaderFn = origReader
		loggerPtr.Store(origLogger)
	})

	var logBuf flowSyncBuf
	setLogger(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	const (
		installedIface   = "eth-installed0"
		uninstalledIface = "eth-uninstalled0"
	)
	setupSamplingFn = func(ifName string, _, _, _ uint32) error {
		if ifName == uninstalledIface {
			return errors.New("device not found")
		}
		return nil
	}
	var removed []string
	removeSamplingFn = func(ifName string) error {
		removed = append(removed, ifName)
		return errors.New("netlink permission denied")
	}
	newPsampleReaderFn = func() (psampleReader, error) {
		return newStubPsampleReader(), nil
	}

	pub := &flowStubPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	w := newSamplingWorker(nil, []SamplingConfig{
		{Interface: installedIface, Rate: 100, Group: 1, TruncSize: 128},
		{Interface: uninstalledIface, Rate: 100, Group: 1, TruncSize: 128},
	})
	w.setIssueQueue(q)
	w.Start()
	w.Stop()
	q.Flush()

	if len(removed) != 1 || removed[0] != installedIface {
		t.Fatalf("RemoveSampling called for %v, want only [%s]", removed, installedIface)
	}

	warnOut := logBuf.String()
	for _, want := range []string{"level=WARN", codeDoctorFlowexportSamplingTeardownFailed, installedIface, "report=queued"} {
		if !strings.Contains(warnOut, want) {
			t.Errorf("WARN log missing %q:\n%s", want, warnOut)
		}
	}

	raised := pub.raisedByCode(codeDoctorFlowexportSamplingTeardownFailed)
	if len(raised) != 1 {
		t.Fatalf("raised issues = %d, want 1", len(raised))
	}
	if raised[0].Severity != rpc.ReportError {
		t.Errorf("severity = %q, want %q", raised[0].Severity, rpc.ReportError)
	}
	if raised[0].Subject != installedIface {
		t.Errorf("subject = %q, want %q", raised[0].Subject, installedIface)
	}
	if raised[0].Fix == "" {
		t.Error("raised issue has empty Fix")
	}
}
