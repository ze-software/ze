// Design: docs/architecture/l2tp/subscriber-session-model.md -- undelivered pool answers

package events

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/ze"
)

const PoolAnswerUndeliveredEvent = "pool-answer-undelivered"

// PoolAnswerUndeliveredPayload names one answer's reservation, not a session
// lifetime. The allocating pool MUST release only a record with this exact
// AllocationID, so a delayed failure cannot release a successor's reservation.
type PoolAnswerUndeliveredPayload struct {
	SubscriberID string
	TunnelID     uint16
	SessionID    uint16
	AllocationID uint64
}

var PoolAnswerUndelivered = events.Register[*PoolAnswerUndeliveredPayload](Namespace, PoolAnswerUndeliveredEvent)

// ReleaseUndeliveredPoolAnswer returns an accepted answer's reservation to its
// owner through the same bus as subscriber teardown. Drains MUST call it when
// IPResponse fails; pools MUST match the AllocationID before releasing state.
// Refusals and answers that own no reservation require no release.
func ReleaseUndeliveredPoolAnswer(logger *slog.Logger, bus ze.EventBus, request ppp.EventIPRequest, answer ppp.IPResponseArgs) {
	if !answer.Accept {
		return
	}
	if answer.AllocationID == 0 {
		return
	}
	if err := subscriber.RequireID(logger, request.SubscriberID); err != nil {
		return
	}
	if bus == nil {
		reportPoolAnswerReleaseFailure(logger, "the subscriber event bus is unavailable", nil)
		return
	}
	if _, err := PoolAnswerUndelivered.Emit(bus, &PoolAnswerUndeliveredPayload{
		SubscriberID: request.SubscriberID,
		TunnelID:     request.TunnelID,
		SessionID:    request.SessionID,
		AllocationID: answer.AllocationID,
	}); err != nil {
		reportPoolAnswerReleaseFailure(logger, "the subscriber event bus could not deliver the reservation's release", err)
	}
}

func reportPoolAnswerReleaseFailure(logger *slog.Logger, reason string, cause error) {
	const problem = "A subscriber did not receive its address, and Ze could not return the reservation to its pool"
	const fix = "ze show errors"
	err := opererr.Wrap(cause, problem, reason, fix)
	logger.Error(err.Error(), "code", report.CodeInternalDefect)
	report.RaiseErrorFix("l2tp-pool", report.CodeInternalDefect, "undelivered address", problem+": "+reason, fix)
}
