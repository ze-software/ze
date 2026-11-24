// Design: docs/architecture/api/process-protocol.md -- validation reaches the same registered owner across both SDK rails.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The literal is the existing public command: tests-only compilation needs no
// production addition. The registered owner's configured name is target-plugin.
const batchValidationOwnerQuestionCommand = "request bgp adj-rib-in batch-validate"

func batchValidationSDKCaller(t *testing.T, s *Server, proc *process.Process, direct bool, requests int) *sdk.Plugin {
	t.Helper()
	client, engine := net.Pipe()
	connection := client
	if direct {
		bridge := rpc.NewDirectBridge()
		bridge.SetDispatchRPC(func(ctx context.Context, method string, input json.RawMessage) (json.RawMessage, error) {
			return s.dispatchPluginRPCDirect(ctx, proc, method, input)
		})
		for _, operation := range allEngineOps() {
			if operation.typedWire != nil {
				operation.typedWire(s, proc, bridge)
			}
		}
		bridge.SetReady()
		connection = rpc.NewBridgedConn(client, bridge)
	}
	caller := sdk.NewWithConn(proc.Name(), connection)
	done := make(chan struct{})
	if direct {
		close(done)
	} else {
		conn := plugipc.NewPluginConn(engine, engine)
		go func() {
			defer close(done)
			for range requests {
				request, err := conn.ReadRequest(s.ctx)
				if err != nil {
					return
				}
				s.dispatchPluginRPC(proc, conn, request)
			}
		}()
	}
	t.Cleanup(func() { _ = caller.Close(); _ = engine.Close(); <-done })
	return caller
}

// Existing forked purge scenario is the full daemon RED. This focused question
// reaches the actual SDK, engine operation, registered owner and framed command
// without any in-daemon receive-store callback. The same question runs direct.
func TestBatchValidateReachesRegisteredReceiveOwner(t *testing.T) {
	previous := rpc.GetBatchValidator()
	t.Cleanup(func() { rpc.RegisterBatchValidator(previous) })
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf("direct=%t", direct), func(t *testing.T) {
			rpc.RegisterBatchValidator(nil)
			s := &Server{dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
			s.ctx, s.cancel = context.WithCancel(t.Context())
			t.Cleanup(s.cancel)
			callerProc := process.NewProcess(plugin.PluginConfig{Name: "validator-alias"})
			callerProc.SetRegistration(&plugin.PluginRegistration{Sends: []rpc.MessageSendDecl{{Message: busMsgBatchValidate}}})
			caller := batchValidationSDKCaller(t, s, callerProc, direct, 6)
			var got [][]string
			done := registerExecuteCommandTarget(t, s.dispatcher, batchValidationOwnerQuestionCommand, 4,
				func(index int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
					got = append(got, append([]string(nil), input.Args...))
					switch index {
					case 0:
						return &rpc.ExecuteCommandOutput{Status: rpc.StatusDone, Data: json.RawMessage(`{"accepted":1,"rejected":2,"early":1}`)}, nil
					case 1:
						return &rpc.ExecuteCommandOutput{Status: rpc.StatusDone, Data: json.RawMessage(`{"accepted":0,"rejected":0,"early":0}`)}, nil
					case 2:
						return &rpc.ExecuteCommandOutput{Status: rpc.StatusError, Data: json.RawMessage(`whole batch refused`)}, nil
					default:
						return &rpc.ExecuteCommandOutput{Status: rpc.StatusDone, Data: json.RawMessage(`null`)}, nil
					}
				})
			registered := s.dispatcher.Registry().Lookup(batchValidationOwnerQuestionCommand)
			require.NotNil(t, registered)
			require.Equal(t, "target-plugin", registered.Process.Name())
			intruder := process.NewProcess(plugin.PluginConfig{Name: "bgp-adj-rib-in"})
			duplicate := s.dispatcher.Registry().Register(intruder, []CommandDef{{Name: batchValidationOwnerQuestionCommand, ShortHelp: "duplicate receiver"}})
			require.Len(t, duplicate, 1)
			require.False(t, duplicate[0].OK)
			require.Equal(t, "target-plugin", duplicate[0].Owner)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			decisions := []rpc.ValidationDecision{
				{Accept: true, PeerAddr: "192.0.2.1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", PathID: 0, ValState: 1, MsgID: 9007199254741003, PeerOwner: 9007199254741005, ConnectionID: 9007199254741007},
				{Ineligible: true, PeerAddr: "192.0.2.1", Family: "ipv6/unicast", Prefix: "2001:db8::/32", PathID: 7, ValState: 3, MsgID: 81, PeerOwner: 51, ConnectionID: 61},
				{PeerAddr: "192.0.2.2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", PathID: 17, ValState: 2, MsgID: 82, PeerOwner: 52, ConnectionID: 62},
			}
			result, err := caller.BatchValidate(ctx, decisions)
			require.NoError(t, err, "the actual registered receive owner must be reached without a daemon-local validator")
			require.Equal(t, &rpc.BatchValidateResult{Accepted: 1, Rejected: 2, Early: 1}, result)
			require.Equal(t, []string{"a", "192.0.2.1", "ipv4/unicast", "10.0.0.0/24", "0", "1", "9007199254741003", "9007199254741005", "9007199254741007",
				"i", "192.0.2.1", "ipv6/unicast", "2001:db8::/32", "7", "3", "81", "51", "61",
				"r", "192.0.2.2", "ipv4/unicast", "10.0.1.0/24", "17", "2", "82", "52", "62"}, got[0])
			result, err = caller.BatchValidate(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, &rpc.BatchValidateResult{}, result)
			require.Empty(t, got[1])
			result, err = caller.BatchValidate(ctx, decisions)
			require.ErrorContains(t, err, "whole batch refused")
			require.Nil(t, result)
			result, err = caller.BatchValidate(ctx, decisions)
			require.Error(t, err)
			require.Nil(t, result, "null owner result is not an acknowledgement")
			require.NoError(t, <-done)
			invalid := append([]rpc.ValidationDecision(nil), decisions...)
			invalid[2].Accept = true
			invalid[2].Ineligible = true
			result, err = caller.BatchValidate(ctx, invalid)
			require.Error(t, err)
			require.Nil(t, result)
			require.Len(t, got, 4, "invalid last decision cannot send any part of the batch")
			var poisonMu sync.Mutex
			poisonCalls := 0
			rpc.RegisterBatchValidator(func([]rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
				poisonMu.Lock()
				poisonCalls++
				poisonMu.Unlock()
				return &rpc.BatchValidateResult{Accepted: 999}, nil
			})
			s.dispatcher.Registry().Unregister(registered.Process, []string{batchValidationOwnerQuestionCommand})
			result, err = caller.BatchValidate(ctx, decisions)
			require.Error(t, err)
			require.Nil(t, result, "an absent owner cannot fall back to another process-local manager")
			poisonMu.Lock()
			count := poisonCalls
			poisonMu.Unlock()
			require.Zero(t, count)
			require.Nil(t, s.dispatcher.Registry().Lookup(batchValidationOwnerQuestionCommand))
		})
	}
}

// A canceled SDK caller must not reach a process-local callback or start a
// receive-store command, even when such a legacy callback remains registered.
func TestBatchValidateCancelledContextDoesNotReachOwner(t *testing.T) {
	previous := rpc.GetBatchValidator()
	t.Cleanup(func() { rpc.RegisterBatchValidator(previous) })
	calls := 0
	rpc.RegisterBatchValidator(func([]rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
		calls++
		return &rpc.BatchValidateResult{}, nil
	})
	s := &Server{dispatcher: NewDispatcher()}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "canceled-validator"})
	caller := batchValidationSDKCaller(t, s, proc, true, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := caller.BatchValidate(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Zero(t, calls)
}
