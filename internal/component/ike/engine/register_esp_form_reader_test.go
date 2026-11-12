package engine

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// espFormRecordedIssues records each issue raised through it and answers err.
type espFormRecordedIssues struct {
	issues []sdk.Issue
	err    error
}

func (r *espFormRecordedIssues) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	r.issues = append(r.issues, issue)
	return r.err
}

// TestESPFormReaderStoppedSinkRaisesEachStopOnce proves the sink runEngine registers with
// the dataplane raises one reader stop once, as an error event under
// doctor-ipsec-esp-form-reader-stopped, a code registered as an event code so the daemon
// accepts the raise, with the stop's error and time in its message and the receiver's
// warnings as its fix.
//
// MUTATION: in espFormReaderStoppedSink, drop the RaiseIssue call: nothing is raised.
// MUTATION: in espFormReaderStoppedIssue, raise rpc.ReportWarning: the event is a warning
// nothing clears.
func TestESPFormReaderStoppedSinkRaisesEachStopOnce(t *testing.T) {
	raiser := &espFormRecordedIssues{}
	sink := espFormReaderStoppedSink(raiser, slogutil.DiscardLogger())
	at := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)

	sink(dataplane.ESPFormReaderStopped{Err: errors.New("test: read ip4 0.0.0.0: bad file descriptor"), At: at})

	if len(raiser.issues) != 1 {
		t.Fatalf("one stop raised %d issues, want one", len(raiser.issues))
	}
	issue := raiser.issues[0]
	if issue.Severity != rpc.ReportError {
		t.Errorf("the stop is raised as %q, want the error event %q", issue.Severity, rpc.ReportError)
	}
	if issue.Code != diagnostic.CodeDoctorIPsecESPFormReaderStopped {
		t.Errorf("the stop is raised under %q, want %q", issue.Code, diagnostic.CodeDoctorIPsecESPFormReaderStopped)
	}
	if issue.Fix != dataplane.ESPFormFixLog {
		t.Errorf("the stop's fix is %q, want %q", issue.Fix, dataplane.ESPFormFixLog)
	}
	for _, want := range []string{"bad file descriptor", "2026-11-12T09:30:00Z"} {
		if !strings.Contains(issue.Message, want) {
			t.Errorf("the stop's message does not carry %q:\n%s", want, issue.Message)
		}
	}
	if !slices.Contains(diagnostic.EventCodes(), diagnostic.CodeDoctorIPsecESPFormReaderStopped) {
		t.Error("the stop's code is not a registered event code, so the daemon refuses the raise")
	}
}
