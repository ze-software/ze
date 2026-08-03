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

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// opForwardCached is the shared handler for forward-cached (JSON + Direct),
// registered as the engineOp for rpc.MethodForwardCached. rs-fastpath-3.
func (s *Server) opForwardCached(proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.ForwardCachedInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid forward-cached params: ").Err(err).String()}
	}
	if err := s.forwardCached(proc, input.IDs, input.Destinations); err != nil {
		return nil, err
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// opReleaseCached is the shared handler for release-cached (JSON + Direct),
// registered as the engineOp for rpc.MethodReleaseCached. rs-fastpath-3.
func (s *Server) opReleaseCached(proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.ReleaseCachedInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid release-cached params: ").Err(err).String()}
	}
	if err := s.releaseCached(proc, input.IDs); err != nil {
		return nil, err
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// opRelayStoredRoute is the shared handler for relay-stored-route (JSON +
// Direct), registered as the engineOp for rpc.MethodRelayStoredRoute.
// spec-fixit-bgp-egress-rail-divergence.
func (s *Server) opRelayStoredRoute(_ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RelayStoredRouteInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid relay-stored-route params: ").Err(err).String()}
	}
	if err := s.relayStoredRoute(input.Destination, input.Routes); err != nil {
		return nil, err
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// relayStoredRoute is the shared implementation of the relay-stored-route RPC.
// It parses the destination once at this boundary (as forwardCached does) and
// delegates to the reactor's forward rail.
//
// A malformed or empty destination is an ERROR, not a dropped entry: unlike
// forward-cached (many destinations, drop the bad ones and proceed) this call
// targets exactly ONE peer, so a destination that does not parse means the whole
// replay has nowhere to go and must fail closed rather than silently relay
// nothing (ai/rules/evidence.md).
func (s *Server) relayStoredRoute(destination string, routes []rpc.StoredRoute) error {
	if len(routes) == 0 {
		return nil
	}
	rc, ok := s.reactor.(plugin.ReactorRelayCoordinator)
	if !ok {
		return errors.New("relay-stored-route: no reactor available")
	}
	addr, err := netip.ParseAddr(destination)
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("relay-stored-route: invalid destination ").Quoted(destination).Str(": ").Err(err).String())
	}
	return rc.RelayStoredRoute(addr, routes)
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
