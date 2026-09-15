package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

func TestExecuteRequestRoundTrip(t *testing.T) {
	caller := api.CallerIdentity{Username: "admin", RemoteAddr: "10.0.0.1:5000"}
	pb := &zepb.CommandRequest{
		Command: "peer",
		Params:  map[string]string{"name": "peer1"},
	}

	req, err := fromProtoExecuteRequest(pb, caller)
	require.NoError(t, err)
	assert.Equal(t, "peer name peer1", req.Command)
	assert.Equal(t, caller, req.Caller)

	// Convert result back to proto.
	result := &api.ExecResult{Status: api.StatusDone, Data: plugin.RawJSON(`{"result":"ok"}`)}
	resp := execResultToProto(result)
	assert.Equal(t, api.StatusDone, resp.Status)
	assert.Equal(t, "", resp.Error)
}

func TestFromProtoExecuteRequestBadParams(t *testing.T) {
	caller := api.CallerIdentity{Username: "admin"}
	pb := &zepb.CommandRequest{
		Command: "show",
		Params:  map[string]string{"bad key": "val"},
	}

	_, err := fromProtoExecuteRequest(pb, caller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "whitespace")
}

func TestFromProtoConfigSetRequest(t *testing.T) {
	pb := &zepb.ConfigSetRequest{
		SessionId: "abc123",
		Path:      "bgp.router-id",
		Value:     "10.0.0.1",
	}

	req := fromProtoConfigSetRequest(pb, "alice")
	assert.Equal(t, "alice", req.Username)
	assert.Equal(t, "abc123", req.SessionID)
	assert.Equal(t, "bgp.router-id", req.Path)
	assert.Equal(t, "10.0.0.1", req.Value)
}

func TestExecResultToProtoNil(t *testing.T) {
	resp := execResultToProto(nil)
	assert.Equal(t, api.StatusError, resp.Status)
	assert.Equal(t, "nil result", resp.Error)
}

func TestCommandMetaToProto(t *testing.T) {
	cmd := api.CommandMeta{
		Name:        "show bgp rib",
		ShortHelp:   "Show RIB routes",
		Description: "One row for each route the RIB holds.",
		ReadOnly:    true,
		Params: []api.ParamMeta{
			{Name: "family", Type: "string", ShortHelp: "Address family", Description: "The family whose routes are shown.", Required: false},
		},
	}

	info := commandMetaToProto(cmd)
	assert.Equal(t, "show bgp rib", info.Name)
	assert.Equal(t, "Show RIB routes", info.ShortHelp)
	assert.Equal(t, "One row for each route the RIB holds.", info.Description)
	assert.True(t, info.ReadOnly)
	require.Len(t, info.Params, 1)
	assert.Equal(t, "family", info.Params[0].Name)
	assert.Equal(t, "Address family", info.Params[0].ShortHelp, "the summary is short_help")
	assert.Equal(t, "The family whose routes are shown.", info.Params[0].Description, "the explanation is description")
}
