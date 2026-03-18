// Design: docs/architecture/api/process-protocol.md — NLRI codec via plugin RPC
// Overview: server.go — Server struct and lifecycle

package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"
)

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family string, args []string) ([]byte, error) {
	if s.registry == nil || s.procManager == nil {
		return nil, fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
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
	if s.registry == nil || s.procManager == nil {
		return "", fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
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
