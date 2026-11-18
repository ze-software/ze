// Design: docs/architecture/api/process-protocol.md — NLRI codec via plugin RPC
// Overview: server.go — Server struct and lifecycle

package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
)

var errServerNotConfiguredForPlugins = errors.New("server not configured for plugins")

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family string, args []string) ([]byte, error) {
	pm := s.procManager.Load()
	if s.registry == nil || pm == nil {
		return nil, errServerNotConfiguredForPlugins
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("plugin %s not running", pluginName)
	}

	conn := proc.Conn()
	if conn == nil {
		return nil, fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	hexStr, err := conn.SendEncodeNLRI(ctx, family, args)
	if err != nil {
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode plugin hex response: %w", err)
	}
	return data, nil
}

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin via RPC.
// Returns the JSON representation of the decoded NLRI.
// Returns error if no plugin registered or plugin not running.
// addPath states whether each NLRI in hexData carries a 4-octet Path Identifier
// ahead of it (RFC 7911 Section 3).
// withdraw selects MP_UNREACH withdrawal semantics; false means announcement.
func (s *Server) DecodeNLRI(family, hexData string, addPath, withdraw bool) (string, error) {
	pm := s.procManager.Load()
	if s.registry == nil || pm == nil {
		return "", errServerNotConfiguredForPlugins
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return "", fmt.Errorf("plugin %s not running", pluginName)
	}

	conn := proc.Conn()
	if conn == nil {
		return "", fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	jsonResult, err := conn.SendDecodeNLRI(ctx, family, hexData, addPath, withdraw)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}

// declaresSessionReady projects readiness from the same current Process
// declaration used by the live-forward fence. A same-named compiled row cannot
// override an external program's accepted Stage-1 value.
func (s *Server) declaresSessionReady(name string) bool {
	ready, _ := s.sessionReplayDeclarations(name)
	return ready
}

// processRegistryNames returns the registry names the running process named
// process uses, resolving an operator's alias (`internal rs { use bgp-rs }`).
// Installed as registry.SetRuntimeRegistryNames. Nil for a name the process
// manager does not hold. The registry calls it outside its lock.
func (s *Server) processRegistryNames(process string) []string {
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}
	proc := pm.GetProcess(process)
	if proc == nil {
		return nil
	}
	return plugin.RegistryNames(proc.Config())
}
