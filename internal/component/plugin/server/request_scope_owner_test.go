// Design: docs/architecture/api/process-protocol.md -- registered owner cancellation.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOwnerCancellationScopeUsesRuntimeStore proves an owner declaration's
// authenticated resource reaches the existing request enrollment and join.
func TestOwnerCancellationScopeUsesRuntimeStore(t *testing.T) {
	const method = "ze-plugin:source-lease-scope-test"
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() {
		engineOperationMu.Lock()
		delete(engineOpTable, method)
		engineOperationMu.Unlock()
		registry.Restore(snapshot)
	})
	deadline := time.Now().Add(time.Second)
	require.NoError(t, registerEngineOp("github.com/ze-software/ze/internal/component/plugin", EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			return nil, nil //nolint:nilnil // this scope-only operation has no result payload
		},
		CancellationScope: func(*Server, *process.Process, json.RawMessage) (EngineRequestScope, error) {
			return NewEngineRequestScope(method, uint64(79), deadline), nil
		},
	}))
	op := lookupEngineOp(method)
	require.NotNil(t, op.cancellationScope)
	requests := newRuntimeRequests(t.Context(), func(op *engineOp, raw json.RawMessage) (requestScope, error) {
		value, err := op.cancellationScope(nil, nil, raw)
		return value, err
	})
	defer requests.cancelAll()
	defer requests.stop()
	admission, err := requests.admit(&rpc.Request{ID: 7, Method: method})
	require.NoError(t, err)
	require.Equal(t, uint64(7), requests.resources[requestResource{method: method, identity: uint64(79)}])
	work, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = requests.cancelAndJoin(work, 7, method)
	require.True(t, errors.Is(err, context.Canceled))
	require.ErrorIs(t, admission.ctx.Err(), context.Canceled)
	admission.finish()
	require.Empty(t, requests.resources)
	require.Empty(t, requests.active)
}
