// Design: docs/architecture/api/process-protocol.md — NLRI codec via plugin RPC
// Overview: server.go — Server struct and lifecycle

package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
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
func (s *Server) DecodeNLRI(family, hexData string) (string, error) {
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

	jsonResult, err := conn.SendDecodeNLRI(ctx, family, hexData)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}

// declaresSessionReady reports whether a RUNNING process declared
// PluginRegistration.SignalsSessionReady at Stage 1, which is the only route an
// external plugin has to that declaration.
//
// Installed as registry.SetRuntimeSessionReady so the reactor reaches the answer
// without importing this package. It is consulted only for a name the
// compile-time registry does not hold, so an in-tree plugin's answer always comes
// from its own Registration and this method never contradicts it.
//
// False for a process that is not running: a process the server cannot see sends
// no report, and waiting for one would hold every peer that attaches it to the
// api-sync timeout.
func (s *Server) declaresSessionReady(process string) bool {
	pm := s.procManager.Load()
	if pm == nil {
		return false
	}
	proc := pm.GetProcess(process)
	if proc == nil {
		return false
	}
	reg := proc.Registration()
	return reg != nil && reg.SignalsSessionReady
}
