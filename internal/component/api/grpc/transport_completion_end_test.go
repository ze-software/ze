// Design: docs/architecture/api/commands.md -- accepted actions outlive transport cancellation.
package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/stats"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestGRPCCompletionEndWithoutPayload drives the real Execute adapter and its
// registered stats owner. An accepted action MUST finish when the RPC ends,
// including a canceled response that never produced OutPayload.
func TestGRPCCompletionEndWithoutPayload(t *testing.T) {
	for _, terminalErr := range []error{nil, context.Canceled} {
		t.Run(terminalName(terminalErr), func(t *testing.T) {
			completed := 0
			engine := completionEndEngine(func() { completed++ }, true)
			handler := transportCompletionStatsHandler{}
			base, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := handler.TagRPC(completionEndCaller(base), &stats.RPCTagInfo{})
			response, err := (&zeServiceImpl{engine: engine}).Execute(ctx,
				&zepb.CommandRequest{Command: "request restart"})
			require.NoError(t, err)
			require.Equal(t, api.StatusDone, response.GetStatus())
			assert.Zero(t, completed, "the adapter still owns the accepted response")
			if terminalErr != nil {
				cancel()
			}
			handler.HandleRPC(ctx, &stats.End{Error: terminalErr})
			assert.Equal(t, 1, completed, "terminal RPC must release an accepted action without OutPayload")
			handler.HandleRPC(ctx, &stats.End{Error: terminalErr})
			assert.Equal(t, 1, completed, "a repeated terminal notification must not repeat the action")
		})
	}
}

// TestGRPCCompletionEndAfterPayload proves the fallback shares the existing
// once-only completion: the ordinary payload write remains the first owner.
func TestGRPCCompletionEndAfterPayload(t *testing.T) {
	completed := 0
	handler := transportCompletionStatsHandler{}
	ctx := handler.TagRPC(completionEndCaller(t.Context()), &stats.RPCTagInfo{})
	_, err := (&zeServiceImpl{engine: completionEndEngine(func() { completed++ }, true)}).
		Execute(ctx, &zepb.CommandRequest{Command: "request restart"})
	require.NoError(t, err)
	handler.HandleRPC(ctx, &stats.OutPayload{})
	assert.Equal(t, 1, completed)
	handler.HandleRPC(ctx, &stats.End{})
	assert.Equal(t, 1, completed)
}

// TestGRPCCompletionEndRejectedRequest proves a rejected command never installs
// an action for the terminal fallback to run.
func TestGRPCCompletionEndRejectedRequest(t *testing.T) {
	completed := 0
	handler := transportCompletionStatsHandler{}
	ctx := handler.TagRPC(completionEndCaller(t.Context()), &stats.RPCTagInfo{})
	response, err := (&zeServiceImpl{engine: completionEndEngine(func() { completed++ }, false)}).
		Execute(ctx, &zepb.CommandRequest{Command: "request restart"})
	require.Error(t, err)
	require.Nil(t, response)
	handler.HandleRPC(ctx, &stats.End{Error: err})
	assert.Zero(t, completed)
}

func completionEndEngine(action func(), allowed bool) *api.APIEngine {
	authorizer := completionEndPolicy()
	return api.NewAPIEngine(
		func(context.Context, api.CallerIdentity, string) (*plugin.Response, error) {
			response := plugin.NewResponse(plugin.StatusDone, plugin.Map{"accepted": true})
			response.OnTransportComplete(action)
			return response, nil
		},
		func() ([]api.CommandMeta, error) {
			return []api.CommandMeta{{Name: "request restart"}}, nil
		},
		func(username, command string) bool {
			if !allowed {
				return false
			}
			return authorizer.Authorize(username, "192.0.2.90:50051", command, false)
		}, nil,
	)
}

// completionEndCaller supplies the identity already authenticated by the
// transport interceptor. An absent identity intentionally means read-only in
// production and cannot stand in for an accepted restart administrator here.
func completionEndCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, callerKey, api.CallerIdentity{
		Username: "restart-admin", RemoteAddr: "192.0.2.90:50051", Surface: audit.GRPC,
		Authorizer: completionEndPolicy(),
	})
}

// completionEndPolicy permits the one mutation under test for its administrator.
// Other identities and all other mutations remain denied by the real policy.
func completionEndPolicy() authz.StoreAuthorizer {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "restart-admin",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{{Number: 10, Action: authz.Allow, Match: "request restart"}},
		},
	})
	store.AssignProfiles("restart-admin", []string{"restart-admin"})
	return authz.StoreAuthorizer{Store: store}
}

func terminalName(err error) string {
	if err != nil {
		return "canceled after acceptance"
	}
	return "successful end without payload"
}
