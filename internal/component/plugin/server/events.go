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
	"github.com/ze-software/ze/internal/component/plugin/registry"
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

// declaresSessionReady reports whether a REGISTERED process declares that it
// reports `plugin session ready`, under either route to that declaration: the
// Stage-1 declare-registration an external plugin sends, or the compile-time
// Registration an in-tree plugin files at init().
//
// The two routes are both needed because the caller asks by PROCESS name, and a
// process name is not a registry key. `plugin { internal rs { use bgp-rs } }`
// gives the operator's alias to the process and files the registration under
// `bgp-rs`, so a lookup by "rs" finds nothing and the peer that attaches it
// sends its End-of-RIB while bgp-rs is still replaying (RFC 4724 Section 4).
// plugin.RegistryNames resolves the alias, and it is the resolution
// claimsForPlugin already uses for the same reason (startup_claims.go).
//
// Installed as registry.SetRuntimeSessionReady so the reactor reaches the answer
// without importing this package. It is consulted only for a name the
// compile-time registry does not hold, which is every alias and every external
// process; a plugin attached under its own registry name is answered by that
// registration before this method is reached.
//
// False for a name the process manager does not hold: nothing was ever started
// under it, so no report can arrive, and waiting for one would hold every peer
// that attaches it to the api-sync timeout. A process that WAS registered and
// has since died still answers true, because the answer is read off the config
// and the declaration rather than off the process state. That direction is the
// loud one: the peer waits out apiSyncTimeout and logs the silent process,
// where the other direction would send a marker claiming an initial routing
// update nobody produced.
func (s *Server) declaresSessionReady(process string) bool {
	pm := s.procManager.Load()
	if pm == nil {
		return false
	}
	proc := pm.GetProcess(process)
	if proc == nil {
		return false
	}
	if reg := proc.Registration(); reg != nil && reg.SignalsSessionReady {
		return true
	}

	for _, name := range plugin.RegistryNames(proc.Config()) {
		if impl := registry.Lookup(name); impl != nil && impl.SignalsSessionReady {
			return true
		}
	}
	return false
}
