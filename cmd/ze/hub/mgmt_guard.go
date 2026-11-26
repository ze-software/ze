// Design: ai/rules/evidence.md -- boot-time management-listener exposure guard
//
// The unifying fix for the 2026-07-16 management-surface audit: a single,
// always-on, fail-closed guard that filters out any management listener bound
// to a non-loopback address without authentication, or without TLS where the
// service requires it (gNMI and MCP do). Each management surface feeds its
// resolved (address, auth) pair to filterMgmtListener; safe addresses
// (loopback or protected) keep starting, while each exposed address stays down
// and is reported through reportServiceNotStarted so ze itself always starts.
// It subsumes the API server's prior inline refusal (apiHasNonLoopback, folded
// into network.ListenAddrIsLoopback).
//
// See plan/spec-fixit-mgmt-listener-auth-guard.md and
// plan/records/ruling-ze-always-starts.md.

package hub

import (
	"slices"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// mgmtListener is one management surface's declaration for the boot-time
// exposure guard. Each surface constructs its own declaration next to its
// resolution code so filterMgmtListener stays generic (registration over
// hardcoding): the guard function names no service.
type mgmtListener struct {
	service       string   // human name for the refusal message (e.g. "gNMI")
	addrs         []string // resolved listen addresses ("host:port" or bare host)
	authenticated bool     // true when a token / users / real auth-mode gates every request
	remedy        string   // actionable fix printed on refusal (never a secret)
	requireTLS    bool     // true when non-loopback listeners MUST also configure TLS
	tlsConfigured bool     // true when TLS material is configured for the listener
	tlsRemedy     string   // actionable fix printed when requireTLS fails (never a secret)
}

// filterMgmtListener applies the management-listener exposure rule per address
// (network.Exposure.Plan). Loopback addresses (and every address when the
// listener satisfies authentication and TLS) are returned in allowed. Each
// breach becomes a serviceNotStarted record naming the offending address (when
// one was resolved) and carrying the operator error (problem, why, fix) from
// network.Exposure; the daemon leaves only the refused listener down and keeps
// running. The rule itself lives in internal/core/network so `ze config
// validate` and `ze doctor` apply the same one offline. The refusal never
// prints a token or other secret.
func filterMgmtListener(l mgmtListener) (allowed []string, down []serviceNotStarted) {
	plan := l.exposure().Plan()
	if len(plan.Breaches) == 0 {
		return plan.Allowed, nil
	}
	down = make([]serviceNotStarted, 0, len(plan.Breaches))
	for _, b := range plan.Breaches {
		var effect textbuf.Buffer
		effect.Str("the ").Str(l.service).Str(" listener")
		if b.Addr != "" {
			effect.Byte(' ').Str(b.Addr)
		}
		effect.Str(" is not started").Str(keepsRunning)
		down = append(down, serviceNotStarted{
			service: l.service,
			effect:  effect.String(),
			problem: b.Refusal,
		})
	}
	return plan.Allowed, down
}

// filterAPIConfig narrows each enabled API transport to the listen addresses
// the exposure guard allowed, disabling a transport whose addresses were all
// refused. A transport that was already disabled stays disabled. The boolean
// reports whether either transport still has a listener to bind.
func filterAPIConfig(cfg zeconfig.APIConfig, allowed []string) (zeconfig.APIConfig, bool) {
	if cfg.RESTOn {
		cfg.REST = filterAPIListen(cfg.REST, allowed)
		cfg.RESTOn = len(cfg.REST) > 0
	}
	if cfg.GRPCOn {
		cfg.GRPC = filterAPIListen(cfg.GRPC, allowed)
		cfg.GRPCOn = len(cfg.GRPC) > 0
	}
	return cfg, cfg.RESTOn || cfg.GRPCOn
}

// filterAPIListen returns the entries whose Listen() address is in allowed.
func filterAPIListen(entries []zeconfig.APIListenConfig, allowed []string) []zeconfig.APIListenConfig {
	var kept []zeconfig.APIListenConfig
	for _, ep := range entries {
		if slices.Contains(allowed, ep.Listen()) {
			kept = append(kept, ep)
		}
	}
	return kept
}

// exposure converts the declaration into the shared rule's input.
func (l mgmtListener) exposure() network.Exposure {
	return network.Exposure{
		Service:       l.service,
		Addrs:         l.addrs,
		Authenticated: l.authenticated,
		AuthFix:       l.remedy,
		RequireTLS:    l.requireTLS,
		TLSConfigured: l.tlsConfigured,
		TLSFix:        l.tlsRemedy,
	}
}

// mgmtListenerFor builds a guard declaration from a service's own declaration
// of the exposure rule's inputs (zeconfig.GNMIExposure, zeconfig.MCPExposure),
// so the guard copies no service's fixes or TLS requirement.
func mgmtListenerFor(e network.Exposure) mgmtListener {
	return mgmtListener{
		service:       e.Service,
		addrs:         e.Addrs,
		authenticated: e.Authenticated,
		remedy:        e.AuthFix,
		requireTLS:    e.RequireTLS,
		tlsConfigured: e.TLSConfigured,
		tlsRemedy:     e.TLSFix,
	}
}

// mcpMgmtListener builds the boot-time guard declaration for MCP from the
// listener as the daemon will bind it: addrs are the resolved addresses (flag,
// environment or config), and authentication follows the server's effective
// mode (mcpListenerAuthenticated). TLS comes only from the environment.mcp
// block, so a listener with no block (cfgOK false) has none. A non-loopback
// MCP listener without authentication or without TLS is filtered out and
// reported via doctor-service-not-started, as a gNMI one is
// (zeconfig.MCPExposure).
func mcpMgmtListener(addrs []string, cfgOK bool, cfg zeconfig.MCPListenConfig, token string) mgmtListener {
	tlsSet := false
	if cfgOK {
		tlsSet = tlsPathsConfigured(cfg.TLS.Cert, cfg.TLS.Key)
	}
	authenticated := mcpListenerAuthenticated(cfgOK, cfg.AuthMode, token)
	return mgmtListenerFor(zeconfig.MCPExposure(addrs, authenticated, tlsSet))
}

// mcpAuthModeAuthenticates reports whether a raw MCP auth-mode string gates
// every request. The set of modes is declared once, in
// zeconfig.MCPAuthModeAuthenticates, which the offline checks read too.
func mcpAuthModeAuthenticates(mode string) bool {
	return zeconfig.MCPAuthModeAuthenticates(mode)
}

// mcpListenerAuthenticated reports whether the MCP server, as it will actually
// be constructed, gates every request. It mirrors the server's effective-mode
// precedence EXACTLY (internal/component/mcp: mcpConfigToStreamable +
// NewStreamable): base.AuthMode starts unspecified and env supplies no
// auth-mode, so an explicit YANG auth-mode wins; only when the auth-mode is
// unspecified does a token (env or YANG, already folded into token) infer the
// single-shared-bearer mode. A token alongside an explicit "none" is IGNORED by
// the server (it builds the accept-all noneAuthenticator), so it must NOT read
// as authenticated here -- that mismatch is the fail-open this guard prevents.
func mcpListenerAuthenticated(cfgOK bool, authMode, token string) bool {
	if cfgOK && authMode != "" {
		return mcpAuthModeAuthenticates(authMode)
	}
	return token != ""
}

// serviceFactoryRegistered reports whether a construction-registry factory with
// the given name is compiled in. The guard consults it so a surface whose
// feature-gate is off (its ze_<feature> registration file was not compiled, so
// it can never bind) is not checked -- reporting an unbuilt surface as not
// started for exposure would be false noise for zero exposure reduction.
func serviceFactoryRegistered(name string) bool {
	for _, nf := range serviceFactories {
		if nf.name == name {
			return true
		}
	}
	return false
}
