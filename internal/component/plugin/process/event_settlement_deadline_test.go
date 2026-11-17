// Design: docs/architecture/api/process-protocol.md -- settlement reply reserve.
// Related: event_settlement.go -- the captured process owns one caller budget.
package process

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSettleEventsAppliedReservesFailureReply lets the work reach its deadline
// and requires its named failure before the original caller expires on both rails.
func TestSettleEventsAppliedReservesFailureReply(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprintf("typed=%t", typed), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			proc, _ := newEventSettlingProcess(t, typed, true, func(_ *Process, input rpc.EventSettlementInput) (rpc.EventSettlementOutput, error) {
				work, stop := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
				defer stop()
				<-work.Done()
				return rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS,
					Status: rpc.StatusError, Error: "destination 192.0.2.181 has an unsettled selected receipt"}, nil
			})
			err := proc.SettleEventsApplied(ctx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expired settlement lost its deadline cause: %v", err)
			}
			if !strings.Contains(err.Error(), "192.0.2.181") {
				t.Fatalf("expired settlement lost its named destination: %v", err)
			}
			if err := ctx.Err(); err != nil {
				t.Fatalf("failure was not answered within its caller budget: %v", err)
			}
		})
	}
}
