// Design: docs/architecture/api/process-protocol.md -- settlement work and reply budgets.
// Related: event_settlement.go -- a failure receipt can follow its work deadline.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEventSettlementSenderKeepsExpiredFailure receives the handler's failure
// after its work deadline, while the caller remains live, on both transports.
func TestEventSettlementSenderKeepsExpiredFailure(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			connection, _ := newEventSettlementSender(t, bridged, func(input rpc.EventSettlementInput) (json.RawMessage, error) {
				work, stop := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
				defer stop()
				<-work.Done()
				return json.Marshal(rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS,
					Status: rpc.StatusError, Error: "destination 192.0.2.181 has an unsettled selected receipt"})
			})
			err := connection.SendEventSettlement(ctx, rpc.EventSettlementInput{DeadlineMS: time.Now().Add(100 * time.Millisecond).UnixMilli()})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expired work lost its deadline cause: %v", err)
			}
			if !strings.Contains(err.Error(), "192.0.2.181") {
				t.Fatalf("expired work lost its named destination: %v", err)
			}
			if err := ctx.Err(); err != nil {
				t.Fatalf("reply consumed the caller's remaining budget: %v", err)
			}
		})
	}
}

// TestEventSettlementSenderRefusesExpiredSuccess gives the reply its separate
// budget but never lets that budget extend the time in which work can succeed.
func TestEventSettlementSenderRefusesExpiredSuccess(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			connection, _ := newEventSettlementSender(t, bridged, func(input rpc.EventSettlementInput) (json.RawMessage, error) {
				work, stop := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
				defer stop()
				<-work.Done()
				return json.Marshal(rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK})
			})
			err := connection.SendEventSettlement(ctx, rpc.EventSettlementInput{DeadlineMS: time.Now().Add(100 * time.Millisecond).UnixMilli()})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expired success = %v, want deadline refusal", err)
			}
		})
	}
}
