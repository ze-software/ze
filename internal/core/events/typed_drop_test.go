package events

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// TestTypedDropRaisesInternalDefect proves a typed event dropped because its
// payload is not the registered type is logged under doctor-internal-defect
// and raised on the report bus under it, so the recent-events doctor check
// reports the drop. Method: an AsString wrapper handed a non-string payload,
// then the log and the bus are read.
//
// VALIDATES: AC-42 (spec-doctor-coverage-12-plugin-infra), the drops half.
// PREVENTS: a payload mismatch only a debug reader of the log would ever see.
func TestTypedDropRaisesInternalDefect(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	wrap := AsString(func(string) {})
	// Red: the raiseDrop call in AsString's wrong-type branch.
	wrap(&asStringPayload{N: 1})

	if !strings.Contains(buf.String(), "code="+report.CodeInternalDefect) {
		t.Errorf("drop logged without its code: %q", buf.String())
	}
	var found *report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == report.CodeInternalDefect && issue.Source == eventBusSource {
			found = &issue
		}
	}
	if found == nil {
		t.Fatalf("no internal-defect event on the bus: %v", report.Errors(0))
	}
	if !strings.Contains(found.Message, "asStringPayload") {
		t.Errorf("event message %q does not name the payload type", found.Message)
	}
}

// handlerBus is a ze.EventBus that keeps the one handler subscribed to it, so
// a test calls the handler with the payload it chooses.
type handlerBus struct{ handler func(any) }

func (b *handlerBus) Emit(string, string, any) (int, error) { return 0, nil }

func (b *handlerBus) Subscribe(_, _ string, handler func(any)) func() {
	b.handler = handler
	return func() {}
}

// TestTypedSubscribeDropRaisesOncePerType proves the registered-type branch
// of the drop, which TestTypedDropRaisesInternalDefect leaves to AsString: a
// payload that is not the registered type reaches no handler, is logged and
// raised under doctor-internal-defect naming the event and both types, and
// is raised once for each distinct dynamic type however many arrive, so a
// drifting publisher cannot wrap the error ring. Method: an Event[T] handle
// subscribed on a bus that hands back its handler, which is then called with
// the wrong type five times and with a second wrong type once.
//
// VALIDATES: AC-42 (spec-doctor-coverage-12-plugin-infra), the Subscribe
// branch, and its once-per-type bound.
// PREVENTS: one error event per dropped payload, which evicts every other
// owner's events from the ring.
func TestTypedSubscribeDropRaisesOncePerType(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	// The handle is built directly so the test registers nothing globally.
	handle := &Event[*asStringPayload]{namespace: "typed-drop-test", eventType: "subscribe-mismatch"}
	bus := &handlerBus{}
	delivered := 0
	handle.Subscribe(bus, func(*asStringPayload) { delivered++ })
	if bus.handler == nil {
		t.Fatal("Subscribe registered no handler on the bus")
	}

	// Red: the raiseDrop call in Subscribe's mismatch branch, or its bound.
	for range 5 {
		bus.handler("a string where a *asStringPayload is registered")
	}
	bus.handler(42)
	bus.handler(&asStringPayload{N: 1})

	if delivered != 1 {
		t.Errorf("the handler ran %d times, want only for the one payload of the registered type", delivered)
	}
	if !strings.Contains(buf.String(), "code="+report.CodeInternalDefect) {
		t.Errorf("drop logged without its code: %q", buf.String())
	}
	var drops []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == report.CodeInternalDefect && issue.Source == eventBusSource {
			drops = append(drops, issue)
		}
	}
	if len(drops) != 2 {
		t.Fatalf("%d internal-defect events, want 2, one per distinct wrong type: %v", len(drops), drops)
	}
	for _, issue := range drops {
		if issue.Subject != "typed-drop-test subscribe-mismatch" {
			t.Errorf("subject %q does not name the event", issue.Subject)
		}
		if !strings.Contains(issue.Message, "asStringPayload") {
			t.Errorf("message %q does not name the registered type", issue.Message)
		}
	}
}
