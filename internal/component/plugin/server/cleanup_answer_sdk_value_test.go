// Design: docs/architecture/api/process-protocol.md -- command outcomes.
// Related: dispatch.go -- responseToDispatchOutput, actual legacy value slots.
// Related: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand[Args]Result.

package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The public SDK must retain completed data and the actual late failure on
// both string and exact-argument calls, on socket and registered bridge rails.
func TestCleanupAnswerSDKValueKeepsFailureProgress(t *testing.T) {
	cleanupAnswerSDKValueCases(t, func(t *testing.T, bridge, args bool, count int) {
		const message = "evaluation stopped before all native acknowledgements arrived"
		s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: message}, cleanupAnswerRows(count, 0, nil))
		client := cleanupAnswerSDKValueClient(t, s, bridge)
		result, err := cleanupAnswerSDKValueCall(t, client, args)
		require.Error(t, err)
		assert.Equal(t, message, err.Error())
		assert.Equal(t, rpc.VerdictAborted, result.Status)
		cleanupAnswerDocument(t, result.Data, count, 0)
	})
}

// A completed SDK value carries its original warning lines even when the
// daemon learned those lines only after forwarding the final streamed row.
func TestCleanupAnswerSDKValueKeepsHealthyWarning(t *testing.T) {
	cleanupAnswerSDKValueCases(t, func(t *testing.T, bridge, args bool, count int) {
		lines := []string{"cleanup remains owned until its acknowledgement", "retry the retained cleanup"}
		s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 0, nil))
		client := cleanupAnswerSDKValueClient(t, s, bridge)
		result, err := cleanupAnswerSDKValueCall(t, client, args)
		require.NoError(t, err)
		assert.Equal(t, rpc.VerdictWarning, result.Status)
		assert.Equal(t, lines, result.Warnings)
		cleanupAnswerDocument(t, result.Data, count, 0)
	})
}

// Refused rows outrank warnings, and neither their causes nor accompanying
// warning lines may disappear when a bridge holds the records as one value.
func TestCleanupAnswerSDKValueKeepsWarningsBesideFaults(t *testing.T) {
	cleanupAnswerSDKValueCases(t, func(t *testing.T, bridge, args bool, count int) {
		lines := []string{"one cleanup object still requires acknowledgement"}
		s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 1, nil))
		client := cleanupAnswerSDKValueClient(t, s, bridge)
		result, err := cleanupAnswerSDKValueCall(t, client, args)
		require.Error(t, err, "refused records were reported as successful SDK evaluation")
		assert.Equal(t, rpc.VerdictPartial, result.Status)
		assert.Equal(t, lines, result.Warnings)
		cleanupAnswerDocument(t, result.Data, count, 1)
	})
}

// A fault-only value still contains every refused row and reports a failure;
// an empty Error field cannot turn the actual error verdict into success.
func TestCleanupAnswerSDKValueKeepsFaultOnlyOutcome(t *testing.T) {
	cleanupAnswerSDKValueCases(t, func(t *testing.T, bridge, args bool, count int) {
		s := cleanupAnswerDispatcher(t, rpc.AnswerTail{}, cleanupAnswerRows(0, count, nil))
		client := cleanupAnswerSDKValueClient(t, s, bridge)
		result, err := cleanupAnswerSDKValueCall(t, client, args)
		require.Error(t, err, "all-refused evaluation returned a successful SDK value")
		assert.Equal(t, rpc.VerdictError, result.Status)
		cleanupAnswerDocument(t, result.Data, 0, count)
	})
}

// The matrix distinguishes each actual public API/transport/threshold route.
// It creates no parallel tests while the registry-backed recipient is active.
func cleanupAnswerSDKValueCases(t *testing.T, question func(*testing.T, bool, bool, int)) {
	t.Helper()
	for _, bridge := range []bool{false, true} {
		transport := "wire"
		if bridge {
			transport = "bridge"
		}
		for _, args := range []bool{false, true} {
			method := "string"
			if args {
				method = "arguments"
			}
			for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
				t.Run(transport+"/"+method+"/"+strconv.Itoa(count), func(t *testing.T) {
					question(t, bridge, args, count)
				})
			}
		}
	}
}

// This is a reader-only SDK client over the real registered engine-op slots.
// Its recipient uses genuine startup through cleanupAnswerDispatcher, while
// this synthetic caller deliberately proves no authentication or native grant.
// The socket rail invokes the production JSON serving wrapper rather than
// copying its answer grammar; the bridge uses the same engineOps registration.
func cleanupAnswerSDKValueClient(t *testing.T, s *Server, bridge bool) *sdk.Plugin {
	t.Helper()
	caller := process.NewProcess(plugin.PluginConfig{Name: "cleanup-value-reader"})
	caller.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{{Message: "cli:show cleanup answer"}},
	})
	reader, engine := net.Pipe()
	if bridge {
		client := sdk.NewWithConn(caller.Name(), rpc.NewBridgedConn(reader, pipeChainBridge(s, caller)))
		t.Cleanup(func() {
			assert.NoError(t, client.Close())
			assert.NoError(t, engine.Close())
		})
		return client
	}
	client := sdk.NewWithConn(caller.Name(), reader)
	connection := pluginipc.NewPluginConn(engine, engine)
	done := make(chan error, 1)
	t.Cleanup(func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, connection.Close())
		if err := <-done; err != nil {
			if t.Failed() {
				t.Logf("SDK reader cleanup: %v", err)
			} else {
				assert.NoError(t, err)
			}
		}
	})
	go func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		request, err := connection.ReadRequest(ctx)
		if err != nil {
			done <- err
			return
		}
		op := lookupEngineOp(request.Method)
		if op == nil {
			done <- fmt.Errorf("engine operation %q is not registered", request.Method)
			return
		}
		done <- s.serveEngineOpJSONContext(ctx, ctx, caller, connection, request, op)
	}()
	return client
}

func cleanupAnswerSDKValueCall(t *testing.T, client *sdk.Plugin, args bool) (sdk.DispatchResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if args {
		return client.DispatchCommandArgsResult(ctx, "show cleanup answer", nil, "")
	}
	return client.DispatchCommandResult(ctx, "show cleanup answer")
}
