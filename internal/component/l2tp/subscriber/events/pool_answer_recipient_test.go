// Design: docs/architecture/l2tp/subscriber-session-model.md -- undelivered pool answers

package events

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/report"
)

type poolAnswerSynchronousBus struct{ handler func(any) }

func (b *poolAnswerSynchronousBus) Emit(_, _ string, payload any) (int, error) {
	b.handler(payload)
	return 0, nil
}

func (b *poolAnswerSynchronousBus) Subscribe(_, _ string, handler func(any)) func() {
	b.handler = handler
	return func() {}
}

// TestPoolAnswerSynchronousReleaseDoesNotReportFailure proves a zero plugin-
// process recipient count does not report a failure after the engine subscriber
// received the exact allocation synchronously (the EventBus.Emit contract).
func TestPoolAnswerSynchronousReleaseDoesNotReportFailure(t *testing.T) {
	bus := &poolAnswerSynchronousBus{}
	var received *PoolAnswerUndeliveredPayload
	unsubscribe := PoolAnswerUndelivered.Subscribe(bus, func(payload *PoolAnswerUndeliveredPayload) {
		received = payload
	})
	defer unsubscribe()
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	ReleaseUndeliveredPoolAnswer(logger, bus,
		ppp.EventIPRequest{SubscriberID: "l2tp-1-2", TunnelID: 1, SessionID: 2}, ppp.IPResponseArgs{Accept: true, AllocationID: 3})
	if received == nil {
		t.Fatal("the synchronous subscriber did not receive the allocation release")
	}
	if received.TunnelID != 1 {
		t.Fatalf("release tunnel = %d, want 1", received.TunnelID)
	}
	if received.SessionID != 2 {
		t.Fatalf("release session = %d, want 2", received.SessionID)
	}
	if received.AllocationID != 3 {
		t.Fatalf("release allocation = %d, want 3", received.AllocationID)
	}
	if strings.Contains(output.String(), report.CodeInternalDefect) {
		t.Fatalf("successful synchronous release reported a defect: %s", output.String())
	}
}
