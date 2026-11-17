// Design: docs/architecture/api/process-protocol.md -- applied event settlement.
// Related: delivery.go -- strict FIFO application remains a separate primitive.
package process

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SettleEventsApplied joins prior event application and declared deferred work.
// Callers MUST release every lock needed by plugin handlers before waiting and
// supply a deadline when settlement is declared. The captured process and
// connection MUST still be current when their receipt returns; a replacement
// cannot complete another lifetime's work. Safe for concurrent use after startup.
// The caller's budget also covers the failure reply; the worker MUST honor its
// earlier work deadline rather than consume that reserved reply time.
//
// The callback runs on the caller, outside deliveryLoop, so events and nested
// engine RPCs continue. Settlement failure is retryable and never becomes the
// sticky application failure that DrainEventsApplied retains in projectionErr.
func (p *Process) SettleEventsApplied(ctx context.Context) error {
	connection := p.Conn()
	if connection == nil {
		return p.eventSettlementRefusal(ErrConnectionClosed)
	}
	if err := p.DrainEventsApplied(ctx); err != nil {
		return err
	}
	if err := p.eventSettlementOwnerError(connection); err != nil {
		return p.eventSettlementRefusal(err)
	}
	declaration := p.Registration()
	if declaration == nil {
		return p.eventSettlementRefusal(errors.New("the process has no runtime declaration"))
	}
	if !declaration.EventSettlement {
		return nil
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return p.eventSettlementRefusal(errors.New("deferred event settlement requires a caller deadline"))
	}
	now := time.Now()
	if !now.Before(deadline) {
		return p.eventSettlementRefusal(context.DeadlineExceeded)
	}
	// Keep the established worker answer allowance, capped to half a short
	// caller budget so both the work and its failure receipt have time.
	replyBudget := min(rpc.CallbackAnswerWriteMax, deadline.Sub(now)/2)
	input := rpc.EventSettlementInput{DeadlineMS: deadline.Add(-replyBudget).UnixMilli()}
	if err := connection.SendEventSettlement(ctx, input); err != nil {
		return p.eventSettlementRefusal(err)
	}
	if err := p.eventSettlementOwnerError(connection); err != nil {
		return p.eventSettlementRefusal(err)
	}
	if err := ctx.Err(); err != nil {
		return p.eventSettlementRefusal(err)
	}
	if !time.Now().Before(deadline) {
		return p.eventSettlementRefusal(context.DeadlineExceeded)
	}
	return nil
}

// eventSettlementOwnerError observes the captured lifetime without invoking a
// handler. It preserves delivery's eventMu-to-mu lock order; no lock survives IPC.
func (p *Process) eventSettlementOwnerError(connection *ipc.PluginConn) error {
	p.eventMu.RLock()
	defer p.eventMu.RUnlock()
	if err := p.deliveryOwnerError(); err != nil {
		return err
	}
	if p.Conn() != connection {
		return ErrConnectionClosed
	}
	return nil
}

func (p *Process) eventSettlementRefusal(cause error) error {
	return opererr.Wrap(cause,
		"Plugin "+p.Name()+" did not finish its deferred event work",
		"The request cannot confirm completion for this plugin process; retry after its reported problem is resolved",
		"request quiesce")
}
