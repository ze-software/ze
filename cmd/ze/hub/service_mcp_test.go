// Design: ai/rules/plugins.md -- ze_mcp-gated MCP factory tests
//
//go:build ze_mcp

package hub

// VALIDATES: the ze_mcp-gated MCP service factory -- buildMCPService builds the
// server through the construction registry (not the old inline startMCPServer
// call in main.go), skips cleanly when not configured, and mcpCommandLister
// converts the neutral commandMeta into zemcp.CommandInfo faithfully (the
// task-support / params / ui-resource mapping).
// PREVENTS: a regression where the registry factory is bypassed, a not-configured
// build is treated as a failure, or the neutral->zemcp conversion drops fields.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestServiceRegistry_BuildsMCP proves the hub builds MCP via the construction
// registry's factory (buildMCPService) from generic serviceDeps -- the server
// binds and reports an "mcp"-named, Reconfigurable service, with no direct
// startMCPServer call from always-on code.
func TestServiceRegistry_BuildsMCP(t *testing.T) {
	port := allocEphemeralPorts(t, 1)[0]
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	svc, err := buildMCPService(serviceDeps{
		MCP: &mcpServiceDeps{
			Addrs: []string{addr},
			Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"status":"ok"}`)), nil
			},
			Commands: func() []commandMeta { return nil },
		},
	})
	require.NoError(t, err)
	require.NotNil(t, svc, "factory must build a service for a valid addr")
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if shutdownErr := svc.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Logf("Shutdown: %v", shutdownErr)
		}
	})

	assert.Equal(t, "mcp", svc.Name())
	assert.NotEmpty(t, svc.Addresses(), "built MCP service must report a bound address")
}

// TestBuildMCPService_NotConfigured proves the factory treats an absent/empty
// config as a skip (nil service, nil error), not a failure.
func TestBuildMCPService_NotConfigured(t *testing.T) {
	// Nil MCP deps -> skip.
	svc, err := buildMCPService(serviceDeps{})
	require.NoError(t, err)
	require.Nil(t, svc, "nil MCP deps must skip")

	// No listen addresses -> skip.
	svc, err = buildMCPService(serviceDeps{MCP: &mcpServiceDeps{
		Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		},
	}})
	require.NoError(t, err)
	require.Nil(t, svc, "no listen addresses must skip")
}

// TestMCPCommandLister proves the neutral commandMeta -> zemcp.CommandInfo
// conversion preserves every field, in particular the task-support mapping
// (required/forbidden/"" -> optional) that the always-on API path does not use.
func TestMCPCommandLister(t *testing.T) {
	src := func() []commandMeta {
		return []commandMeta{
			{
				Name:        "show bgp rib dump",
				Help:        "Dump RIB",
				ReadOnly:    true,
				TaskSupport: "required",
				Params: []commandParam{
					{Name: "peer", Type: "string", Description: "peer addr", Required: true},
				},
				UIResource: &commandUIResource{Path: "bgp/index.html", Permissions: "network", CSP: "default-src 'self'"},
			},
			{Name: "ping host", Help: "Ping", TaskSupport: "forbidden"},
			{Name: "show config dump", Help: "Dump config", TaskSupport: ""},
		}
	}

	infos := mcpCommandLister(src)()
	require.Len(t, infos, 3)

	assert.Equal(t, "show bgp rib dump", infos[0].Name)
	assert.Equal(t, "Dump RIB", infos[0].Help)
	assert.True(t, infos[0].ReadOnly)
	assert.Equal(t, zemcp.TaskSupportRequired, infos[0].TaskSupport)
	require.Len(t, infos[0].Params, 1)
	assert.Equal(t, "peer", infos[0].Params[0].Name)
	assert.Equal(t, "string", infos[0].Params[0].Type)
	assert.True(t, infos[0].Params[0].Required)
	require.NotNil(t, infos[0].UIResource)
	assert.Equal(t, "bgp/index.html", infos[0].UIResource.Path)
	assert.Equal(t, "network", infos[0].UIResource.Permissions)

	assert.Equal(t, zemcp.TaskSupportForbidden, infos[1].TaskSupport)
	assert.Nil(t, infos[1].Params)
	assert.Nil(t, infos[1].UIResource)

	// Empty task-support maps to the zero value (TaskSupportOptional), matching
	// the pre-gate plugin-command default.
	assert.Equal(t, zemcp.TaskSupportOptional, infos[2].TaskSupport)

	// A nil source yields a nil list (no panic).
	assert.Nil(t, mcpCommandLister(func() []commandMeta { return nil })())
}
