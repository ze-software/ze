// Design: docs/architecture/l2tp/subscriber-session-model.md -- undelivered pool answers

package events

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/ze"
)

type releaseFailureBus struct{ err error }

func (b releaseFailureBus) Emit(_, _ string, _ any) (int, error)    { return 0, b.err }
func (releaseFailureBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// TestPoolAnswerReleaseFailureIsReported proves an unavailable cleanup path is
// visible in the log and doctor error ring, rather than silently leaking state.
func TestPoolAnswerReleaseFailureIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		bus  ze.EventBus
	}{
		{name: "no-bus"},
		{name: "emit-failed", bus: releaseFailureBus{err: errors.New("injected bus failure")}},
		{name: "empty-error", bus: releaseFailureBus{err: errors.New("")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			ReleaseUndeliveredPoolAnswer(logger, tc.bus, ppp.EventIPRequest{SubscriberID: "l2tp-1-2", TunnelID: 1, SessionID: 2},
				ppp.IPResponseArgs{Accept: true, AllocationID: 3})
			if !strings.Contains(output.String(), report.CodeInternalDefect) {
				t.Fatalf("cleanup failure has no doctor code: %s", output.String())
			}
			if !strings.Contains(output.String(), "ze show errors") {
				t.Fatalf("cleanup failure has no next command: %s", output.String())
			}
			errors := report.Errors(1)
			if len(errors) != 1 {
				t.Fatalf("doctor errors = %d, want cleanup failure", len(errors))
			}
			if errors[0].Code != report.CodeInternalDefect {
				t.Fatalf("doctor code = %s, want %s", errors[0].Code, report.CodeInternalDefect)
			}
		})
	}
}
