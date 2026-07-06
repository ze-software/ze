// Design: docs/architecture/api/process-protocol.md — plugin RPC dispatch (rs-fastpath-3)
// Related: dispatch.go — the plugin->engine RPC switch these handlers hang off
//
// Cache-consumer fast path (rs-fastpath-3): a plugin that consumes cached UPDATEs
// forwards or releases them by id via forward-cached / release-cached, bypassing the
// update-route tokenise path. Extracted from dispatch.go to keep that file within
// the file-modularity budget.

package server

import (
	"encoding/json"
	"errors"
	"net/netip"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// handleForwardCachedRPC handles ze-plugin-engine:forward-cached from a plugin
// (pipe path). rs-fastpath-3.
func (s *Server) handleForwardCachedRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.ForwardCachedInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		var tb textbuf.Buffer
		if sendErr := conn.SendError(s.ctx, req.ID, tb.Str("invalid forward-cached params: ").Str(err.Error()).String()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if err := s.forwardCached(proc, input.IDs, input.Destinations); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if sendErr := conn.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleForwardCachedDirect is the bridge (no-socket-I/O) variant of
// handleForwardCachedRPC. rs-fastpath-3.
func (s *Server) handleForwardCachedDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	var input rpc.ForwardCachedInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid forward-cached params: ").Err(err).String()}
	}
	if err := s.forwardCached(proc, input.IDs, input.Destinations); err != nil {
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return nil, nil
}

// handleReleaseCachedRPC handles ze-plugin-engine:release-cached from a plugin
// (pipe path). rs-fastpath-3.
func (s *Server) handleReleaseCachedRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.ReleaseCachedInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		var tb textbuf.Buffer
		if sendErr := conn.SendError(s.ctx, req.ID, tb.Str("invalid release-cached params: ").Str(err.Error()).String()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if err := s.releaseCached(proc, input.IDs); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if sendErr := conn.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleReleaseCachedDirect is the bridge variant of handleReleaseCachedRPC.
// rs-fastpath-3.
func (s *Server) handleReleaseCachedDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	var input rpc.ReleaseCachedInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid release-cached params: ").Err(err).String()}
	}
	if err := s.releaseCached(proc, input.IDs); err != nil {
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return nil, nil
}

// forwardCached is the shared implementation of the forward-cached RPC. It
// parses destination strings into netip.AddrPort values and delegates to the
// reactor's ForwardUpdatesDirect cache-consumer fast path. Malformed
// destinations are logged and dropped; the remaining destinations proceed.
// rs-fastpath-3.
func (s *Server) forwardCached(proc *process.Process, ids []uint64, destinations []string) error {
	if len(ids) == 0 {
		return nil
	}
	rc, ok := s.reactor.(plugin.ReactorCacheCoordinator)
	if !ok {
		return errors.New("forward-cached: no reactor available")
	}
	addrPorts := parseForwardDestinations(proc.Name(), destinations)
	return rc.ForwardUpdatesDirect(ids, addrPorts, proc.Name())
}

// releaseCached is the shared implementation of the release-cached RPC.
// rs-fastpath-3.
func (s *Server) releaseCached(proc *process.Process, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	rc, ok := s.reactor.(plugin.ReactorCacheCoordinator)
	if !ok {
		return errors.New("release-cached: no reactor available")
	}
	return rc.ReleaseUpdates(ids, proc.Name())
}

// parseForwardDestinations converts "addr" or "addr:port" strings to
// netip.AddrPort values. Malformed destinations are logged at WARN and
// dropped -- they cannot correspond to a real peer, so silently forwarding
// to "nothing" would mask caller bugs. Bare IPs produce port 0, which the
// reactor expands to every peer with that address.
//
// If the input is non-empty but EVERY entry is malformed, a summary WARN
// fires so the operator sees "all destinations dropped" as a distinct
// signal from the per-entry drops. The caller propagates the reactor's
// ErrNoDestinations back to the plugin.
func parseForwardDestinations(plugin string, dst []string) []netip.AddrPort {
	if len(dst) == 0 {
		return nil
	}
	out := make([]netip.AddrPort, 0, len(dst))
	for _, s := range dst {
		if s == "" {
			continue
		}
		if ap, err := netip.ParseAddrPort(s); err == nil {
			out = append(out, ap)
			continue
		}
		if addr, err := netip.ParseAddr(s); err == nil {
			out = append(out, netip.AddrPortFrom(addr, 0))
			continue
		}
		logger().Warn("forward-cached: dropping malformed destination",
			"plugin", plugin, "destination", s)
	}
	if len(out) == 0 {
		logger().Warn("forward-cached: every destination was malformed, refusing to broadcast",
			"plugin", plugin, "count", len(dst))
	}
	return out
}
