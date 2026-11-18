// Design: docs/architecture/api/process-protocol.md -- captured event settlement.
// Related: dispatch.go -- one producer for applied and queue-only emission.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// eventEmissionCapture exists for this one live request only. It never records
// completed process/source history and never resolves a successor by name.
type eventEmissionCapture struct{ receivers []eventEmissionReceiver }
type eventEmissionReceiver struct {
	process    *process.Process
	connection *ipc.PluginConn
	result     chan process.EventResult
}

func (c *eventEmissionCapture) receiver(proc *process.Process) (eventEmissionReceiver, error) {
	receiver := eventEmissionReceiver{process: proc, connection: proc.Conn(), result: make(chan process.EventResult, 1)}
	if err := receiver.liveError(); err != nil {
		return eventEmissionReceiver{}, err
	}
	return receiver, nil
}

// liveError reads the original Process and connection, never its configured
// name's current replacement. It holds no lock across a callback or RPC.
func (receiver eventEmissionReceiver) liveError() error {
	if !receiver.process.Running() {
		return eventAppliedRefusal(receiver.process.Name(), "its original process stopped", nil)
	}
	select {
	case <-receiver.process.Done():
		return eventAppliedRefusal(receiver.process.Name(), "its original process stopped", nil)
	case <-receiver.process.EngineDone():
		return eventAppliedRefusal(receiver.process.Name(), "its original process ended", nil)
	default:
	}
	if receiver.connection == nil {
		return eventAppliedRefusal(receiver.process.Name(), "its captured connection is absent", nil)
	}
	if receiver.process.Conn() != receiver.connection {
		return eventAppliedRefusal(receiver.process.Name(), "its original connection changed", nil)
	}
	if err := receiver.connection.Err(); err != nil {
		return eventAppliedRefusal(receiver.process.Name(), "its original connection ended", err)
	}
	return nil
}

func eventAppliedRefusal(name, why string, cause error) error {
	if cause == nil {
		cause = errors.New(why)
	}
	return opererr.Wrap(cause, "Applied event did not finish for "+name,
		why+"; partial application or deferred replay/output may still be owed",
		"ze request quiesce")
}

func eventAppliedSelfRefusal() error {
	return eventAppliedRefusal("the emitting process", "an applied event cannot wait on its own subscription; remove that subscription or move coordination outside the event handler", nil)
}

// eventEmissionInput is the shared admission/handler boundary. Ordinary
// queue-only inputs retain their existing behavior, including absent headers.
func eventEmissionInput(proc *process.Process, raw json.RawMessage) (rpc.EmitEventInput, error) {
	var input rpc.EmitEventInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, fmt.Errorf("invalid emit-event params: %w", err)
	}
	if err := eventEmissionHeader(proc, input.AwaitApplied, input.DeadlineMS); err != nil {
		return input, err
	}
	return input, nil
}

// eventEmissionHeader is the shared mode/deadline predicate. Admission decodes
// only these scalars so a large Event body is allocated only by evaluation.
func eventEmissionHeader(proc *process.Process, awaitApplied bool, deadlineMS int64) error {
	if !awaitApplied {
		return nil
	}
	if proc == nil {
		return eventAppliedRefusal("the emitter", "applied emission requires its authenticated process", nil)
	}
	if deadlineMS <= 0 {
		return eventAppliedRefusal(proc.Name(), "applied emission requires a positive absolute deadline-ms", nil)
	}
	return nil
}

func emitEventCancellationScope(_ *Server, proc *process.Process, raw json.RawMessage) (requestScope, error) {
	var header struct {
		AwaitApplied bool  `json:"await-applied"`
		DeadlineMS   int64 `json:"deadline-ms"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return requestScope{}, fmt.Errorf("invalid emit-event params: %w", err)
	}
	if err := eventEmissionHeader(proc, header.AwaitApplied, header.DeadlineMS); err != nil {
		return requestScope{}, err
	}
	if !header.AwaitApplied {
		return requestScope{}, nil
	}
	return rpc.NewRequestCancellationScope(rpc.MethodEmitEvent, time.UnixMilli(header.DeadlineMS)), nil
}

func (s *Server) emitEventApplied(ctx context.Context, emitter *process.Process, raw json.RawMessage) (*rpc.EmitEventOutput, error) {
	input, err := eventEmissionInput(emitter, raw)
	if err != nil {
		return nil, err
	}
	if !input.AwaitApplied {
		return s.emitEventInput(emitter, input)
	}
	// The bridge uses the same finite ceiling as the socket admission registry.
	ceiling, cancelCeiling := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
	defer cancelCeiling()
	work, cancel := context.WithDeadline(ceiling, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := work.Err(); err != nil {
		return nil, eventAppliedRefusal("the emitter", "its work deadline ended before publication", err)
	}
	var captured eventEmissionCapture
	delivered, err := s.deliverEventCaptured(work, emitter, input.Namespace, input.EventType, input.Direction, input.PeerAddress, input.Event, &captured)
	if err != nil {
		return nil, err
	}
	// Join every handler first. Consumers may publish revalidation events to
	// another captured consumer, so all those publications MUST precede the
	// second pass's FIFO/deferred-work join.
	for _, receiver := range captured.receivers {
		select {
		case <-work.Done():
			return nil, eventAppliedRefusal(receiver.process.Name(), "its event application was not joined", work.Err())
		case result := <-receiver.result:
			if result.Err != nil {
				return nil, eventAppliedRefusal(receiver.process.Name(), "its event handler refused application", result.Err)
			}
		}
		if err := receiver.liveError(); err != nil {
			return nil, err
		}
	}
	for _, receiver := range captured.receivers {
		if err := receiver.process.SettleEventsApplied(work); err != nil {
			return nil, eventAppliedRefusal(receiver.process.Name(), "its deferred replay/output did not settle", err)
		}
		if err := receiver.liveError(); err != nil {
			return nil, err
		}
	}
	// Earlier receivers can end while a later receiver settles. Completion is
	// one observation of this captured population, not each receiver's old success.
	for _, receiver := range captured.receivers {
		if err := receiver.liveError(); err != nil {
			return nil, err
		}
	}
	if err := work.Err(); err != nil {
		return nil, eventAppliedRefusal("the emitter", "its operation ended before completion", err)
	}
	return &rpc.EmitEventOutput{Applied: true, DeadlineMS: input.DeadlineMS, Delivered: delivered}, nil
}
