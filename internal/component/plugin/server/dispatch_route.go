// Design: plan/learned/1070-forked-route-install.md -- forked route install via Loc-RIB RPC
// Related: dispatch.go -- the plugin->engine RPC switch these handlers hang off
//
// A forked (external, out-of-process) route-installing plugin (OSPF, IS-IS) cannot
// reach the engine's process-wide Loc-RIB singleton: locrib.Default() returns nil
// in a subprocess (default.go, guarded on ze.plugin.hub.token). These handlers are
// the cross-process bridge: the plugin ships its computed routes as a batch, the
// engine applies them to locrib.Default() -- the REAL singleton in the engine
// process -- and sysrib's OnChange subscription programs the kernel exactly as it
// does for an in-process installer. See plan/learned/639-rib-unified.md for the
// locrib->sysrib->fibkernel path this reuses unchanged.
//
// The engine tracks each plugin's installed routes (installedByPlugin) so a
// disconnect withdraws them (AC-8): a forked plugin that dies without withdrawing
// must not leave stale routes in the kernel.

package server

import (
	"encoding/json"
	"fmt"
	"net/netip"

	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// maxProtocolNameLen caps an accepted redistevents protocol name. A forked plugin
// names its own protocol ("ospf", "isis"); a longer value is malformed input.
const maxProtocolNameLen = 64

// routeKey is the (family, prefix, source, instance) identity of one Loc-RIB path
// a forked plugin installed. It is comparable, so it keys the per-plugin tracking
// set that disconnect cleanup withdraws (AC-8).
type routeKey struct {
	fam      family.Family
	prefix   netip.Prefix
	source   redistevents.ProtocolID
	instance uint32
}

// handleRouteInstallRPC handles ze-plugin-engine:route-install from a forked
// route-installing plugin: it applies the batch to the engine's Loc-RIB and records
// the routes under the plugin name for disconnect cleanup.
//
// the input/apply/output types differ and merging them via generics hurts clarity.
//
//nolint:dupl // route-install and route-remove are deliberately parallel handlers;
func (s *Server) handleRouteInstallRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.RouteInstallInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		s.routeCounter().With(proc.Name(), "install", "error").Inc()
		var tb textbuf.Buffer
		s.sendRouteErr(conn, proc, req.ID, tb.Str("invalid route-install params: ").Str(err.Error()).String())
		return
	}
	keys, err := applyRouteInstall(locrib.Default(), input)
	if err != nil {
		s.routeCounter().With(proc.Name(), "install", "error").Inc()
		s.sendRouteErr(conn, proc, req.ID, err.Error())
		return
	}
	s.recordInstalled(proc.Name(), keys)
	s.routeCounter().With(proc.Name(), "install", "ok").Inc()
	if sendErr := conn.SendResult(s.ctx, req.ID, &rpc.RouteInstallOutput{Installed: uint32(len(keys))}); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleRouteRemoveRPC handles ze-plugin-engine:route-remove: it withdraws the
// batch from the engine's Loc-RIB and drops the routes from the plugin's tracking.
//
//nolint:dupl // parallel with handleRouteInstallRPC by design; see the note there.
func (s *Server) handleRouteRemoveRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.RouteRemoveInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		s.routeCounter().With(proc.Name(), "remove", "error").Inc()
		var tb textbuf.Buffer
		s.sendRouteErr(conn, proc, req.ID, tb.Str("invalid route-remove params: ").Str(err.Error()).String())
		return
	}
	keys, err := applyRouteRemove(locrib.Default(), input)
	if err != nil {
		s.routeCounter().With(proc.Name(), "remove", "error").Inc()
		s.sendRouteErr(conn, proc, req.ID, err.Error())
		return
	}
	s.unrecordInstalled(proc.Name(), keys)
	s.routeCounter().With(proc.Name(), "remove", "ok").Inc()
	if sendErr := conn.SendResult(s.ctx, req.ID, &rpc.RouteRemoveOutput{Removed: uint32(len(keys))}); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

func (s *Server) sendRouteErr(conn *plugipc.PluginConn, proc *process.Process, id uint64, msg string) {
	if err := conn.SendError(s.ctx, id, msg); err != nil {
		logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", err)
	}
}

// routeCounter lazily builds ze_route_install_rpc_total from the configured metrics
// registry (a nop when metrics are disabled or the server has no config, e.g. a
// unit-test zero-value Server).
func (s *Server) routeCounter() metrics.CounterVec {
	s.routeMetricOnce.Do(func() {
		if s.config == nil || s.config.MetricsRegistry == nil {
			s.routeInstallRPCs = metrics.NopRegistry{}.CounterVec("", "", nil)
			return
		}
		s.routeInstallRPCs = s.config.MetricsRegistry.CounterVec(
			"ze_route_install_rpc_total",
			"Forked-plugin route-install/route-remove RPCs handled by the engine, by plugin, op, and result.",
			[]string{"plugin", "op", "result"},
		)
	})
	return s.routeInstallRPCs
}

// installOp is one validated, parsed route ready to insert into the Loc-RIB.
type installOp struct {
	fam    family.Family
	prefix netip.Prefix
	path   locrib.Path
}

// applyRouteInstall validates and applies a route-install batch to rib. It builds
// every op FIRST (so a malformed entry fails the whole batch before any partial
// apply), then inserts. The protocol NAME is re-resolved to THIS process's
// ProtocolID via redistevents.RegisterProtocol -- register-on-demand, because the
// forked plugin's own init (which registered "ospf"/"isis") ran in the subprocess,
// so the engine may not have seen the name yet. Returns the keys applied.
func applyRouteInstall(rib *locrib.RIB, input rpc.RouteInstallInput) ([]routeKey, error) {
	if rib == nil {
		return nil, fmt.Errorf("route-install: engine has no Loc-RIB")
	}
	ops := make([]installOp, 0, len(input.Routes))
	cache := make(map[string]redistevents.ProtocolID)
	for i := range input.Routes {
		e := &input.Routes[i]
		id, err := resolveProtocol(e.Protocol, cache)
		if err != nil {
			return nil, err
		}
		prefix, err := netip.ParsePrefix(e.Prefix)
		if err != nil {
			return nil, fmt.Errorf("route-install: bad prefix %q: %w", e.Prefix, err)
		}
		var nextHop netip.Addr
		if e.NextHop != "" {
			nextHop, err = netip.ParseAddr(e.NextHop)
			if err != nil {
				return nil, fmt.Errorf("route-install: bad next-hop %q: %w", e.NextHop, err)
			}
		}
		var backup netip.Addr
		if e.BackupNextHop != "" {
			backup, err = netip.ParseAddr(e.BackupNextHop)
			if err != nil {
				return nil, fmt.Errorf("route-install: bad backup-next-hop %q: %w", e.BackupNextHop, err)
			}
		}
		ops = append(ops, installOp{
			fam:    family.Family{AFI: family.AFI(e.AFI), SAFI: family.SAFI(e.SAFI)},
			prefix: prefix,
			path: locrib.Path{
				Source:             id,
				Instance:           e.Instance,
				NextHop:            nextHop,
				AdminDistance:      e.AdminDistance,
				Metric:             e.Metric,
				Labels:             e.Labels,
				IsEBGP:             e.IsEBGP,
				BackupNextHop:      backup,
				BackupRepairLabels: e.BackupRepairLabels,
			},
		})
	}
	keys := make([]routeKey, 0, len(ops))
	for i := range ops {
		rib.InsertForward(ops[i].fam, ops[i].prefix, ops[i].path, nil)
		keys = append(keys, routeKey{fam: ops[i].fam, prefix: ops[i].prefix, source: ops[i].path.Source, instance: ops[i].path.Instance})
	}
	return keys, nil
}

// applyRouteRemove validates and applies a route-remove batch to rib, withdrawing
// each (Source, Instance) path. Builds all ops before removing (batch-atomic on
// validation). Returns the keys withdrawn.
func applyRouteRemove(rib *locrib.RIB, input rpc.RouteRemoveInput) ([]routeKey, error) {
	if rib == nil {
		return nil, fmt.Errorf("route-remove: engine has no Loc-RIB")
	}
	keys := make([]routeKey, 0, len(input.Routes))
	cache := make(map[string]redistevents.ProtocolID)
	for i := range input.Routes {
		e := &input.Routes[i]
		id, err := resolveProtocol(e.Protocol, cache)
		if err != nil {
			return nil, err
		}
		prefix, err := netip.ParsePrefix(e.Prefix)
		if err != nil {
			return nil, fmt.Errorf("route-remove: bad prefix %q: %w", e.Prefix, err)
		}
		keys = append(keys, routeKey{
			fam:      family.Family{AFI: family.AFI(e.AFI), SAFI: family.SAFI(e.SAFI)},
			prefix:   prefix,
			source:   id,
			instance: e.Instance,
		})
	}
	for i := range keys {
		rib.Remove(keys[i].fam, keys[i].prefix, keys[i].source, keys[i].instance)
	}
	return keys, nil
}

// resolveProtocol maps a wire protocol NAME to this process's ProtocolID via a
// LOOKUP -- never a registration. The engine binary contains every in-tree
// route-installing protocol, so their names are registered at package init (e.g.
// ospfProtocolID = redistevents.RegisterProtocol("ospf")) regardless of which
// plugins are loaded or forked; an unknown name is therefore malformed or foreign
// and is REJECTED. Rejecting (rather than register-on-demand) is load-bearing:
// registering arbitrary wire-supplied names would pollute the process-global
// redistevents registry and, once ~65535 distinct names accumulate, panic it
// (registry.go RegisterProtocol) -- crashing the engine from untrusted-ish plugin
// input. cache dedupes the lookup within one batch (a batch shares one protocol
// name in practice, so this is one lookup per batch, not per route).
func resolveProtocol(name string, cache map[string]redistevents.ProtocolID) (redistevents.ProtocolID, error) {
	if name == "" {
		return 0, fmt.Errorf("route-install: empty protocol name")
	}
	if len(name) > maxProtocolNameLen {
		return 0, fmt.Errorf("route-install: protocol name too long (%d > %d)", len(name), maxProtocolNameLen)
	}
	if id, ok := cache[name]; ok {
		return id, nil
	}
	id, ok := redistevents.ProtocolIDOf(name)
	if !ok {
		return 0, fmt.Errorf("route-install: unknown protocol %q (not registered in the engine)", name)
	}
	cache[name] = id
	return id, nil
}

// recordInstalled adds keys to the plugin's live-route set (AC-8 tracking).
func (s *Server) recordInstalled(pluginName string, keys []routeKey) {
	if len(keys) == 0 {
		return
	}
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	if s.installedByPlugin == nil {
		s.installedByPlugin = make(map[string]map[routeKey]struct{})
	}
	set := s.installedByPlugin[pluginName]
	if set == nil {
		set = make(map[routeKey]struct{})
		s.installedByPlugin[pluginName] = set
	}
	for _, k := range keys {
		set[k] = struct{}{}
	}
}

// unrecordInstalled drops keys from the plugin's live-route set (an explicit
// withdraw the plugin performed).
func (s *Server) unrecordInstalled(pluginName string, keys []routeKey) {
	if len(keys) == 0 {
		return
	}
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	set := s.installedByPlugin[pluginName]
	if set == nil {
		return
	}
	for _, k := range keys {
		delete(set, k)
	}
	if len(set) == 0 {
		delete(s.installedByPlugin, pluginName)
	}
}

// withdrawPluginRoutes removes every route a plugin installed from the engine
// Loc-RIB and clears its tracking. Called from cleanupProcess on disconnect (AC-8).
func (s *Server) withdrawPluginRoutes(pluginName string) {
	s.routeMu.Lock()
	set := s.installedByPlugin[pluginName]
	delete(s.installedByPlugin, pluginName)
	s.routeMu.Unlock()
	if len(set) == 0 {
		return
	}
	rib := locrib.Default()
	if rib == nil {
		return
	}
	for k := range set {
		rib.Remove(k.fam, k.prefix, k.source, k.instance)
	}
	logger().Debug("withdrew forked plugin routes on disconnect", "plugin", pluginName, "routes", len(set))
}
