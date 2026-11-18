// Design: docs/architecture/api/process-protocol.md -- applied emission receipt.
package sdk

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An older host can understand emit-event and still acknowledge only queue
// admission. The applied producer MUST require explicit matching completion,
// including a null result and a receipt for another absolute deadline.
func TestEmitAppliedRejectsQueueOnlyOrWrongReceipt(t *testing.T) {
	for _, kind := range []string{"queue-only", "null", "wrong-deadline", "matching"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			client, engine := net.Pipe()
			p := NewWithConn("applied-receipt", client)
			joined := make(chan error, 1)
			go func() {
				wire := rpc.NewConn(engine, engine)
				request, err := wire.ReadRequest(ctx)
				if err != nil {
					joined <- err
					return
				}
				var input rpc.EmitEventInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					joined <- err
					return
				}
				var output any
				switch kind {
				case "queue-only":
					output = &rpc.EmitEventOutput{DeadlineMS: input.DeadlineMS, Delivered: 1}
				case "matching":
					output = &rpc.EmitEventOutput{Applied: true, DeadlineMS: input.DeadlineMS, Delivered: 1}
				case "wrong-deadline":
					output = &rpc.EmitEventOutput{Applied: true, DeadlineMS: input.DeadlineMS + 1, Delivered: 1}
				case "null":
					output = nil
				}
				joined <- wire.SendResult(ctx, request.ID, output)
			}()
			t.Cleanup(func() { require.NoError(t, p.Close()); require.NoError(t, engine.Close()) })
			count, err := p.EmitEventApplied(ctx, "test", "cleanup", "", "", "null")
			if kind == "matching" {
				require.NoError(t, err)
				require.Equal(t, 1, count)
			} else {
				require.Error(t, err)
				require.Zero(t, count)
			}
			select {
			case err := <-joined:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("receipt producer did not join", ctx.Err())
			}
		})
	}
}
