// Design: docs/architecture/ospf/bfd-client.md -- per-session subscriber worker, stop+done handshake
// Related: bfd_client.go -- the two release paths these tests drive
package ospf

import (
	"bytes"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// lockedLog is a log sink the subscriber goroutine writes while the test reads it.
// Safe for concurrent use.
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// errForeignHandleText is the text of the BFD engine's only release refusal.
const errForeignHandleText = "bfd: foreign SessionHandle"

// refusedReleaseEngine opens a BFD session for the test neighbor on a service that
// refuses every release, with the engine's log in a WARN sink and its raises recorded.
func refusedReleaseEngine(t *testing.T) (*engine, *fakeBFDService, *lockedLog, *recordedIssues) {
	t.Helper()
	raised := recordIssues(t)
	svc := newFakeBFDService()
	svc.releaseErr = errors.New(errForeignHandleText)
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	sink := &lockedLog{}
	e.log = slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	return e, svc, sink, raised
}

// assertReleaseWarning checks the one WARN line a refused release writes and the one
// error event it raises through the SDK publisher.
func assertReleaseWarning(t *testing.T, log string, raised *recordedIssues) {
	t.Helper()
	issues := raised.withCode(diagnostic.CodeDoctorInternalDefect)
	if len(issues) != 1 {
		t.Fatalf("internal-defect raises = %d, want 1: %+v", len(issues), issues)
	}
	issue := issues[0]
	if issue.Severity != rpc.ReportError {
		t.Errorf("raise severity = %q, want %q", issue.Severity, rpc.ReportError)
	}
	if want := bfdTestIface + " " + bfdTestPeerRID; issue.Subject != want {
		t.Errorf("raise subject = %q, want %q", issue.Subject, want)
	}
	if want := "OSPF could not release the BFD session of neighbor " + bfdTestPeerRID + " on interface " + bfdTestIface; issue.Message != want {
		t.Errorf("raise message = %q, want %q", issue.Message, want)
	}
	if issue.Fix != showBFDSessionsFix {
		t.Errorf("raise fix = %q, want %q", issue.Fix, showBFDSessionsFix)
	}
	var refused []string
	for _, line := range warnLines(log) {
		if strings.Contains(line, "could not release the BFD session") {
			refused = append(refused, line)
		}
	}
	if len(refused) != 1 {
		t.Fatalf("refused-release WARN lines = %d, want 1:\n%s", len(refused), log)
	}
	for _, part := range []string{
		`msg="OSPF could not release the BFD session of neighbor ` + bfdTestPeerRID + ` on interface ` + bfdTestIface + `"`,
		`why="` + bfdReleaseRefusedWhy + `"`,
		`fix="` + showBFDSessionsFix + `"`,
		"code=doctor-internal-defect",
		"interface=" + bfdTestIface,
		errForeignHandleText,
		"report=queued",
	} {
		if !strings.Contains(refused[0], part) {
			t.Errorf("WARN line lacks %s:\n%s", part, refused[0])
		}
	}
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a release the BFD service refuses when the
// neighbor leaves Full (stopBFDSession) is reported at WARN naming the neighbor, the
// interface, why and the command, under the internal-defect doctor code, and raised as
// an error event through the SDK publisher.
// Method: open a session on a service that refuses every release, stop it, read the log
// and the raises a recording publisher kept.
// PREVENTS: a BFD session left held for a neighbor OSPF dropped, recorded only at debug;
// and a raise a forked OSPF would keep in its own process-local report bus.
func TestOSPFBFDStopReleaseRefusalIsWarned(t *testing.T) {
	e, _, sink, raised := refusedReleaseEngine(t)

	e.stopBFDSession(bfdClientKey{iface: bfdTestIface, router: ridMust(t, bfdTestPeerRID)})

	assertReleaseWarning(t, sink.String(), raised)
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a release the BFD service refuses when the
// subscriber detaches itself on a BFD Down (detachBFDClient) is reported the same way.
// Method: open a session on a service that refuses every release, emit a Down, wait for
// the line.
// PREVENTS: the subscriber's own release path hiding the failure the stop path reports.
func TestOSPFBFDDetachReleaseRefusalIsWarned(t *testing.T) {
	_, svc, sink, raised := refusedReleaseEngine(t)
	h := svc.handleFor(t, svc.firstRequest(t).Key())

	h.emit(t, packet.StateDown, packet.DiagControlDetectExpired)

	// warnBFDReleaseRefused raises before it logs, so the line proves the raise was made.
	waitFor(t, func() bool { return strings.Contains(sink.String(), "could not release the BFD session") })
	assertReleaseWarning(t, sink.String(), raised)
}
