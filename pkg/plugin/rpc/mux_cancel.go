// Design: docs/architecture/api/ipc_protocol.md -- acknowledged request cancellation.
// Related: mux.go -- ordinary calls keep their existing cancellation contract.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// CallRPCCancellable is the opt-in request form whose recipient registers a
// request-scoped context. Cancellation waits for the recipient's join receipt;
// an ordinary CallRPC peer cannot satisfy this contract. Safe for concurrent
// use. A transport failure returns CancellationUnsettledError, never a claim
// that remote work finished. The caller MUST keep params immutable until return.
func (m *MuxConn) CallRPCCancellable(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := m.readerErr.Load(); err != nil {
		return nil, fmt.Errorf("mux conn read error: %w", *err)
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	id := m.conn.NextID()
	if id == 0 {
		return nil, errors.Join(errors.New("RPC request IDs exhausted; reconnect before sending more requests"), m.Close())
	}
	key := pendingKey(id)
	response := make(chan []byte, 1)
	m.pending.Store(key, response)
	defer m.pending.Delete(key)
	write := &cancellableWrite{}
	writeContext := context.WithValue(ctx, cancellableWriteKey{}, write)
	if err := m.conn.writeAppended(writeContext, func(buf []byte) []byte {
		return AppendRequest(buf, id, method, encoded)
	}); err != nil {
		if !write.begun.Load() {
			return nil, err
		}
		return nil, m.cancellationUnsettled(err, errors.New("request frame could not be completed"))
	}
	select {
	case body := <-response:
		return interpretResponse(body)
	case <-ctx.Done():
		return nil, m.cancelAndJoin(ctx, id, method)
	case <-m.done:
		select {
		case body := <-response:
			return interpretResponse(body)
		default:
			_, err := m.queuedResponse(response)
			return nil, m.cancellationUnsettled(err, errors.New("connection ended before final settlement"))
		}
	}
}

func (m *MuxConn) cancelAndJoin(ctx context.Context, id uint64, method string) error {
	join, cancel := context.WithTimeout(context.WithoutCancel(ctx), RequestCancelTimeout)
	defer cancel()
	encoded, err := m.CallRPC(join, MethodCancelRequest, CancelRequestInput{RequestID: id, Method: method})
	if err != nil {
		return m.cancellationUnsettled(ctx.Err(), err)
	}
	var output CancelRequestOutput
	if err := json.Unmarshal(encoded, &output); err != nil {
		return m.cancellationUnsettled(ctx.Err(), err)
	}
	if output.RequestID != id {
		return m.cancellationUnsettled(ctx.Err(), errors.New("cancellation receipt names another request"))
	}
	// Cancellation receipts are open wire input; unknown outcomes fail closed.
	switch output.Outcome {
	case CancelRequestSettled, CancelRequestNotActive:
		return ctx.Err()
	case CancelRequestUnspecified:
		return m.cancellationUnsettled(ctx.Err(), errors.New("cancellation outcome is missing"))
	default:
		return m.cancellationUnsettled(ctx.Err(), errors.New("cancellation outcome is unknown"))
	}
}

func (m *MuxConn) cancellationUnsettled(cause, settlement error) error {
	// A partial frame or unjoined request cannot share a reusable stream. Its
	// recipient cancels the connection's admitted contexts before joining them.
	if err := m.Close(); err != nil {
		settlement = errors.Join(settlement, err)
	}
	return &CancellationUnsettledError{cause: cause, settlement: settlement}
}
