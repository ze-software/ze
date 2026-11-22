// Design: ai/rules/evidence.md -- boot-time management-listener exposure guard
//
// The unifying fix for the 2026-07-16 management-surface audit: a single,
// always-on, fail-closed guard that refuses to start any management listener
// bound to a non-loopback address without authentication, or without TLS where
// the service requires it (gNMI and MCP do). Each management
// surface appends its own mgmtListener declaration next to the code that
// resolves its (address, auth) pair; checkMgmtListeners iterates the slice and
// names no service. It subsumes the API server's prior inline refusal
// (apiHasNonLoopback, folded into network.ListenAddrIsLoopback).
//
// See plan/spec-fixit-mgmt-listener-auth-guard.md.

package hub

import (
	"fmt"
	"os"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/opererr"
)

// mgmtListener is one management surface's declaration for the boot-time
// exposure guard. A surface appends its own declaration next to its resolution
// code so checkMgmtListeners stays generic (registration over hardcoding):
// the guard function names no service.
type mgmtListener struct {
	service       string   // human name for the refusal message (e.g. "gNMI")
	addrs         []string // resolved listen addresses ("host:port" or bare host)
	authenticated bool     // true when a token / users / real auth-mode gates every request
	remedy        string   // actionable fix printed on refusal (never a secret)
	requireTLS    bool     // true when non-loopback listeners MUST also configure TLS
	tlsConfigured bool     // true when TLS material is configured for the listener
	tlsRemedy     string   // actionable fix printed when requireTLS fails (never a secret)
}

// checkMgmtListeners refuses to start when any declared management surface
// breaches the exposure rule (network.Exposure.Refusals): a non-loopback bind
// without authentication, or a non-loopback bind without TLS when requireTLS is
// set. It prints one operator error per breach (problem, why, fix) and returns
// true (refused); the caller returns a non-zero exit code before anything
// binds. The rule itself lives in internal/core/network so `ze config
// validate` and `ze doctor` apply the same one offline. The refusal never
// prints a token or other secret.
func checkMgmtListeners(listeners []mgmtListener) bool {
	refused := false
	// Declared with its type, and assigned by the range below rather than
	// defined there, so each line printed is visibly an operator error.
	var refusal *opererr.Error
	for _, l := range listeners {
		for _, refusal = range l.exposure().Refusals() {
			fmt.Fprintln(os.Stderr, refusal.Error())
			refused = true
		}
	}
	return refused
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
// MCP listener without authentication or without TLS refuses daemon start,
// as a gNMI one does (zeconfig.MCPExposure).
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
// it can never bind) is not declared -- refusing to boot on config a binary
// cannot serve would break working deployments for zero exposure reduction.
func serviceFactoryRegistered(name string) bool {
	for _, nf := range serviceFactories {
		if nf.name == name {
			return true
		}
	}
	return false
}
