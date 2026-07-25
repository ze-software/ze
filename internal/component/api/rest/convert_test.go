package rest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
)

func TestFromRESTExecuteRequest(t *testing.T) {
	caller := api.CallerIdentity{Username: "admin", RemoteAddr: "127.0.0.1:8080"}
	params := map[string]any{"name": "peer1", "family": "ipv4"}

	req, err := fromRESTExecuteRequest(caller, "peer", params)
	require.NoError(t, err)
	assert.Equal(t, caller, req.Caller)
	assert.Contains(t, req.Command, "peer")
	assert.Contains(t, req.Command, "name peer1")
	assert.Contains(t, req.Command, "family ipv4")
}

func TestFromRESTExecuteRequestBadParams(t *testing.T) {
	caller := api.CallerIdentity{Username: "admin"}
	params := map[string]any{"bad key": "val"}

	_, err := fromRESTExecuteRequest(caller, "show", params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "whitespace")
}

func TestFromRESTExecuteRequestEmptyParams(t *testing.T) {
	caller := api.CallerIdentity{Username: "admin"}

	req, err := fromRESTExecuteRequest(caller, "show version", nil)
	require.NoError(t, err)
	assert.Equal(t, "show version", req.Command)
}

func TestFromRESTConfigSetRequest(t *testing.T) {
	req := fromRESTConfigSetRequest("alice", "sess1", "bgp.router-id", "10.0.0.1")
	assert.Equal(t, "alice", req.Username)
	assert.Equal(t, "sess1", req.SessionID)
	assert.Equal(t, "bgp.router-id", req.Path)
	assert.Equal(t, "10.0.0.1", req.Value)
}

func TestFromRESTConfigDiffRequest(t *testing.T) {
	req := fromRESTConfigDiffRequest("bob", "sess2")
	assert.Equal(t, "bob", req.Username)
	assert.Equal(t, "sess2", req.SessionID)
}
