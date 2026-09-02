// Design: docs/architecture/api/process-protocol.md — Stage 1 declaration, Stage 2 configure
// Overview: startup.go — the startup phases and the peer-start signal this resolution is ordered against
// Related: startup_autoload.go — how the prospective plugin set is assembled

package server

import (
	"errors"
	"slices"
	"strings"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// errUnbackedClaim reports that a plugin stood its own default behavior down for
// a claimant that never reached Running. It is NOT fatal and is never returned:
// it exists to give that consequence one stable, greppable phrase in the ERROR
// log verifyAdvertisedClaims emits (which records why fatal was rejected).
var errUnbackedClaim = errors.New("plugin startup: exclusive role claimed by a plugin that is not running")

// advertiseClaims returns the exclusive runtime roles claimed by OTHER plugins
// in this daemon's startup set, for delivery to forPlugin on its Stage-2
// configure callback (rpc.ConfigureInput.Claims).
//
// Why here, and not from a runtime callback. A plugin that must stand its own
// default behavior down for another plugin has to know BEFORE it can receive
// its first runtime event. The post-startup callback cannot carry that: it is
// fanned out on detached goroutines and deliberately not waited on
// (sendPostStartupToAll in startup.go, which records why waiting deadlocks), and
// signalStartupComplete calls SignalPluginStartupComplete -> StartPeers right
// after. So a decision taken there races the first session establishment. Stage
// 2 is part of the sequential handshake driven by runStartupHandshake: it has
// returned before the plugin sends Stage 5 ready, which is before the phase
// completes, which is before peers start. There is no window.
//
// The claimant does not have to have handshaked yet -- that is the point.
// bgp-adj-rib-in is configured in an earlier dependency tier than bgp-rs
// (bgp-rs declares it as an OptionalDependency, so TopologicalTiers orders the
// dependency first), so a fact sourced from bgp-rs's own registration RPC would
// arrive too late for bgp-adj-rib-in. The claim is read from the plugin's
// static registration instead, which is populated by init() before any startup
// phase runs. runPluginPhase marks every plugin of a phase loaded before the
// tier handshake begins (startup.go, step (a)), and explicitly configured
// plugins are known from the config, so the prospective set is complete and
// stable at every plugin's Stage 2.
//
// forPlugin is excluded from its own answer: a claim expresses "I take this
// role over from another plugin's default", never "from my own".
func (s *Server) advertiseClaims(forPlugin string) []string {
	var tokens []string
	seen := make(map[string]bool)

	for _, pp := range s.prospectivePlugins() {
		if pp.process == forPlugin {
			continue
		}
		for _, token := range s.claimsForPlugin(pp) {
			if token == "" {
				continue
			}
			s.recordAdvertisedClaim(token, pp.process)
			if seen[token] {
				continue
			}
			seen[token] = true
			tokens = append(tokens, token)
		}
	}

	slices.Sort(tokens)
	return tokens
}

// prospectivePlugin is one plugin the engine knows will be part of this daemon,
// under both the names it answers to.
//
// The two differ whenever the operator renames an implementation:
// `plugin { internal rs { use bgp-rs } }` yields PluginConfig{Name: "rs",
// Run: "bgp-rs"}, so the ProcessManager and every log line say "rs" while the
// compile-time registration is filed under "bgp-rs". Looking a claim up by the
// process name alone silently finds nothing, and -- worse -- checking that a
// claimant is running by its registry name finds no process and would report a
// live claimant as unbacked.
type prospectivePlugin struct {
	process  string   // the name ProcessManager.GetProcess keys on
	registry []string // candidate names registry.ClaimsFor keys on
}

// prospectivePlugins returns every plugin the engine already knows will be part
// of this daemon: explicitly configured ones (known from the config before any
// phase runs), ones marked loaded by a startup phase (marked before that phase's
// handshake begins), and ones with a spawned process. The three sources overlap;
// the union is deduplicated by process name, merging registry-name candidates.
func (s *Server) prospectivePlugins() []prospectivePlugin {
	index := make(map[string]int)
	var out []prospectivePlugin

	add := func(process string, registryNames ...string) {
		if process == "" {
			return
		}
		i, ok := index[process]
		if !ok {
			index[process] = len(out)
			out = append(out, prospectivePlugin{process: process, registry: []string{process}})
			i = len(out) - 1
		}
		for _, rn := range registryNames {
			if rn != "" && !slices.Contains(out[i].registry, rn) {
				out[i].registry = append(out[i].registry, rn)
			}
		}
	}

	// One rule answers "which registry row does this process config name", and
	// plugin.RegistryNames is it: the process name plus the implementation its
	// run/use spelling resolves to (`use bgp-rs`, `run ze.bgp-rs`,
	// `run ze plugin bgp-adj-rib-in`).
	addConfig := func(p plugin.PluginConfig) {
		add(p.Name, plugin.RegistryNames(p)...)
	}

	if s.config != nil {
		for _, p := range s.config.Plugins {
			addConfig(p)
		}
	}

	// Auto-loaded plugins are registered under their own name (getConfigPathPlugins
	// builds PluginConfig{Name: <registry name>} with no Run), so process name and
	// registry name coincide.
	s.loadedPluginsMu.Lock()
	for name := range s.loadedPlugins {
		add(name)
	}
	s.loadedPluginsMu.Unlock()

	if pm := s.procManager.Load(); pm != nil {
		for _, proc := range pm.AllProcesses() {
			if proc != nil {
				addConfig(proc.Config())
			}
		}
	}

	slices.SortFunc(out, func(a, b prospectivePlugin) int {
		return strings.Compare(a.process, b.process)
	})
	return out
}

// claimsForPlugin returns the exclusive roles a plugin declares. A runtime
// declaration (Stage 1 declare-registration) wins when the plugin has already
// handshaked, because an external plugin has no compile-time registry row at
// all; otherwise the static registration is used under every candidate name,
// which is what makes the answer available before the claimant has started.
func (s *Server) claimsForPlugin(pp prospectivePlugin) []string {
	if pm := s.procManager.Load(); pm != nil {
		if proc := pm.GetProcess(pp.process); proc != nil {
			if reg := proc.Registration(); reg != nil && len(reg.Claims) > 0 {
				return reg.Claims
			}
		}
	}

	var out []string
	for _, name := range pp.registry {
		for _, token := range registry.ClaimsFor(name) {
			if !slices.Contains(out, token) {
				out = append(out, token)
			}
		}
	}
	return out
}

// recordAdvertisedClaim remembers that the engine told some plugin that token
// is claimed, and by whom, so verifyAdvertisedClaims can check the claimant
// actually came up.
func (s *Server) recordAdvertisedClaim(token, claimant string) {
	s.advertisedClaimsMu.Lock()
	defer s.advertisedClaimsMu.Unlock()
	if s.advertisedClaims == nil {
		s.advertisedClaims = make(map[string]map[string]bool)
	}
	if s.advertisedClaims[token] == nil {
		s.advertisedClaims[token] = make(map[string]bool)
	}
	s.advertisedClaims[token][claimant] = true
}

// advertisedClaimants copies the advertised-claim index out from under its
// lock, as token -> claimant process names. It returns nil when nothing was
// ever advertised. That is the common case, and it keeps the two readers below
// off the lock on a daemon that runs no claiming plugin.
//
// Safe for concurrent use.
func (s *Server) advertisedClaimants() map[string][]string {
	s.advertisedClaimsMu.Lock()
	defer s.advertisedClaimsMu.Unlock()

	if len(s.advertisedClaims) == 0 {
		return nil
	}
	advertised := make(map[string][]string, len(s.advertisedClaims))
	for token, claimants := range s.advertisedClaims {
		for claimant := range claimants {
			advertised[token] = append(advertised[token], claimant)
		}
	}
	return advertised
}

// UnheldRoles returns the advertised exclusive-role tokens that NO process in
// procs holds, sorted. It is the engine RETRACTING, for one event, a promise it
// made before any peer started.
//
// advertiseClaims tells a plugin at Stage 2 that a role is claimed, and that
// plugin stands its own default behavior down for it. The promise is necessarily
// about the DAEMON, because Stage 2 runs before any session exists. Two things
// make it wrong for one peer, and the plugin that stood down can see neither:
//
//   - The claimant takes no delivery of this peer's events. Delivery is
//     per-peer: an `attach process <name> { receive ... }` block grants it, and
//     PeerScopedProcs takes the overlap of that grant with what the plugin
//     subscribed to. Take a peer that gives `state` to bgp-adj-rib-in and not
//     to bgp-rs. The claimant cannot act on it, and the claim keeps the other
//     plugin's peer-up replay off. Nobody serves that peer.
//   - The claimant never reached Running. verifyAdvertisedClaims says so and
//     can do nothing else, because the Stage-2 handshake is over by the time it
//     is known.
//
// procs is the set the event is being delivered to, never the delivery graph's
// edges alone: a process that is granted the event but has not subscribed to it
// takes no delivery and will not act on it either.
//
// Safe for concurrent use.
func (s *Server) UnheldRoles(procs []*process.Process) []string {
	advertised := s.advertisedClaimants()
	if len(advertised) == 0 {
		return nil
	}

	var unheld []string
	for token, claimants := range advertised {
		if procsHoldRole(procs, claimants) {
			continue
		}
		unheld = append(unheld, token)
	}

	slices.Sort(unheld)
	return unheld
}

// procsHoldRole reports whether any process in procs is one of the claimants of
// a role.
func procsHoldRole(procs []*process.Process, claimants []string) bool {
	for _, proc := range procs {
		if slices.Contains(claimants, proc.Name()) {
			return true
		}
	}
	return false
}

// unbackedClaims returns the advertised role tokens whose every claimant failed
// to reach Running, sorted. An empty result means every advertisement is
// honored (or nothing was ever advertised, the overwhelmingly common case).
func (s *Server) unbackedClaims() []string {
	advertised := s.advertisedClaimants()
	if len(advertised) == 0 {
		return nil
	}

	pm := s.procManager.Load()
	var unbacked []string
	for token, claimants := range advertised {
		backed := false
		for _, claimant := range claimants {
			if pm == nil {
				break
			}
			proc := pm.GetProcess(claimant)
			if proc != nil && proc.Running() && proc.Stage() >= plugin.StageRunning {
				backed = true
				break
			}
		}
		if !backed {
			unbacked = append(unbacked, token)
		}
	}

	slices.Sort(unbacked)
	return unbacked
}

// verifyAdvertisedClaims reports, loudly, that a plugin was told a role is
// claimed but the claimant never reached Running.
//
// Advertising a claim at Stage 2 is a promise about a plugin that has not
// started yet, and the plugin that received it has already stood its own
// default behavior down. If the claimant then fails startup, nobody performs
// the role -- for peer-up replay, routes learned before a peer establishes are
// never replayed to it.
//
// It does NOT fail startup. Making it fatal was tried (2026-07-25) and is
// disproportionate: a claimant reaches Running only if its whole startup phase
// succeeded, so ANY unrelated plugin failure in that phase -- an unknown plugin
// name in the operator's config, a plugin dying at Stage 1 -- left bgp-rs short
// of Running and killed a daemon that previously survived. It turned 25 passing
// functional tests red (bgp-redistribute-*, fib-*, forward-*), none of which
// has anything to do with replay ownership. The daemon is already degraded in
// this case; removing it entirely is a worse outcome than the missing replay.
//
// So this is a guard that cannot deny and therefore must speak
// (ai/rules/evidence.md). It used to leave a residual gap: the stood-down plugin
// stayed stood down for the whole process lifetime. UnheldRoles closes that gap
// for every peer-scoped event by retracting the advertisement on the event
// itself. A claimant that never reached Running takes delivery of nothing, so it
// holds no role for any peer. The retraction rides the event instead of arriving
// before StartPeers, which keeps it off the racing post-startup fan-out. This
// line stays because it names the daemon-wide degradation in one place.
//
// Called from signalStartupComplete, after every phase has settled (failed
// plugins are already rolled back) and before SignalPluginStartupComplete, so
// the verdict is reached with complete knowledge of which plugins are running.
func (s *Server) verifyAdvertisedClaims() {
	for _, token := range s.unbackedClaims() {
		logger().Error("exclusive plugin role claimed but no claimant is running; "+
			"the plugin that stood down for it will not perform the role",
			"role", token, "consequence", errUnbackedClaim.Error())
	}
}
