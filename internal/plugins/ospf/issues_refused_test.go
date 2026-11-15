// Design: docs/architecture/ospf/ospf-4-component-config.md -- the problems OSPF raises on the daemon's report bus
// Related: issues.go -- internalDefectRaiseFailed and ipsecEntryLeftRaiseFailed, the callbacks these tests drive
package ospf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// errDaemonRefusedRaise is what refusingIssues answers to every raise.
var errDaemonRefusedRaise = errors.New("report-issue: the daemon refused the raise")

// refusingIssues is a publisher whose daemon refuses every raise, the way a daemon that
// is stopping or overloaded answers report-issue. Safe for concurrent use: it keeps
// nothing.
type refusingIssues struct{}

func (refusingIssues) RaiseIssue(context.Context, sdk.Issue) error      { return errDaemonRefusedRaise }
func (refusingIssues) ClearIssue(context.Context, string, string) error { return nil }

// warnLogger logs WARN and above to sink. Two goroutines write it here: the code
// that met the problem writes its WARN line, and the report queue's worker writes
// the refusal's line, so sink is a lockedLog (bfd_release_refused_test.go).
func warnLogger(sink *lockedLog) *slog.Logger {
	return slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a raise the daemon refuses after the queue
// took it reaches the operator anyway: the failed callback of each OSPF raise site
// writes one WARN line saying the bus does not list the problem, under the problem's
// own code, naming the subject, the daemon's error and the problem's fix, and the
// problem's own WARN line still reads report=queued, because the queue took it.
// Method: a report queue over a publisher that refuses every raise; one refused event
// emit (internalDefectRaiseFailed through warnEmitRefused) and one failed IPsec
// removal (ipsecEntryLeftRaiseFailed through warnRemoval); flush the queue, then read
// the WARN lines.
// PREVENTS: a refused raise vanishing, so show errors or ze doctor lacks the problem
// and no line says why.
func TestOSPFRefusedRaisesAreWarned(t *testing.T) {
	queue := useIssuePublisher(t, refusingIssues{})

	emitLog := &lockedLog{}
	previous := logger()
	loggerPtr.Store(warnLogger(emitLog))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	warnEmitRefused(EventNeighborUp, "neighbor 10.0.0.2 on interface eth0", showNeighborFix,
		errors.New("unknown event: ospf/"+EventNeighborUp))

	removalLog := &lockedLog{}
	inst := &ipsecInstaller{log: warnLogger(removalLog)}
	const entry = "dst fe80::2 proto esp spi 256"
	const deleteFix = "ip -6 xfrm state delete " + entry
	inst.warnRemoval("eth1", entry, opererr.Wrap(errors.New("xfrm: state del: operation not permitted"),
		"OSPF IPsec could not remove the kernel security association of interface eth1", ipsecRemovalWhy, deleteFix))

	queue.Flush()

	cases := []struct {
		site, log, refusal, code, subjectKey, subject, fix string
	}{
		{
			site: "warnEmitRefused", log: emitLog.String(),
			refusal: "ospf could not raise an internal defect on the daemon's report bus, so show errors does not list it",
			code:    diagnostic.CodeDoctorInternalDefect, subjectKey: "subject",
			subject: EventNeighborUp + " neighbor 10.0.0.2 on interface eth0", fix: showNeighborFix,
		},
		{
			site: "warnRemoval", log: removalLog.String(),
			refusal: "ospf could not raise an IPsec entry left in the kernel on the daemon's report bus, so ze doctor does not list it",
			code:    codeOSPFv3IPsecEntryLeft, subjectKey: "entry", subject: entry, fix: deleteFix,
		},
	}
	for _, tc := range cases {
		lines := warnLines(tc.log)
		if len(lines) != 2 {
			t.Fatalf("%s: WARN lines = %d, want the problem and its refused raise (2):\n%s", tc.site, len(lines), tc.log)
		}
		var problemLine, refusalLine string
		for _, line := range lines {
			if strings.Contains(line, tc.refusal) {
				refusalLine = line
				continue
			}
			problemLine = line
		}
		if refusalLine == "" {
			t.Fatalf("%s: no WARN line says the raise was refused:\n%s", tc.site, tc.log)
		}
		for _, want := range []string{
			"code=" + tc.code,
			fmt.Sprintf("%s=%q", tc.subjectKey, tc.subject),
			fmt.Sprintf("error=%q", errDaemonRefusedRaise.Error()),
			fmt.Sprintf("fix=%q", tc.fix),
		} {
			if !strings.Contains(refusalLine, want) {
				t.Errorf("%s: the refused raise's WARN line lacks %s:\n%s", tc.site, want, refusalLine)
			}
		}
		if !strings.Contains(problemLine, "report=queued") {
			t.Errorf("%s: the problem's WARN line does not say the queue took the raise:\n%s", tc.site, problemLine)
		}
	}
}
