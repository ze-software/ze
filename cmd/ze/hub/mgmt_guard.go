// Design: ai/rules/fail-closed-guards.md -- boot-time management-listener exposure guard
//
// The unifying fix for the 2026-07-16 management-surface audit: a single,
// always-on, fail-closed guard that refuses to start any management listener
// bound to a non-loopback address without authentication. Each management
// surface appends its own mgmtListener declaration next to the code that
// resolves its (address, auth) pair; checkMgmtListeners iterates the slice and
// names no service. It subsumes the API server's prior inline refusal
// (apiHasNonLoopback, folded into listenAddrIsNonLoopback here).
//
// See plan/spec-fixit-mgmt-listener-auth-guard.md.

package hub

import (
	"fmt"
	"net"
	"net/netip"
	"os"
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
}

// listenAddrIsNonLoopback reports whether a single listen address binds to a
// non-loopback address. Fail-closed: an unparseable host (a DNS name like
// `localhost`, an empty host, or garbage) and any unspecified address
// (0.0.0.0, ::) classify as non-loopback, so an operator cannot smuggle remote
// reachability past the guard through a name or a wildcard bind. This preserves
// the exact semantics of the two prior producers it replaces
// (apiHasNonLoopback in api.go and MCPListenConfig.AnyListenerNonLoopback).
func listenAddrIsNonLoopback(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return true
	}
	return !ip.IsLoopback()
}

// checkMgmtListeners refuses to start when any declared management surface binds
// non-loopback without authentication. It prints one error per offending
// listener (service, address, remedy) and returns true (refused); the caller
// returns a non-zero exit code before anything binds. An authenticated surface
// is passed regardless of address; a loopback-only surface is passed regardless
// of auth. The refusal message never prints a token or other secret.
func checkMgmtListeners(listeners []mgmtListener) bool {
	refused := false
	for _, l := range listeners {
		if l.authenticated {
			continue
		}
		// An unauthenticated surface with no resolved addresses cannot be shown
		// to be loopback-only, so it must not pass by iterating zero times --
		// that is the empty-set trap in ai/rules/fail-closed-guards.md, and it
		// is exactly how an insecure web server reached 0.0.0.0:3443 past this
		// guard (the builder filled the default afterwards). Every declaration
		// site resolves its addresses first and declares only when the surface
		// will actually bind, so reaching here means the declaration is wrong.
		if len(l.addrs) == 0 {
			fmt.Fprintf(os.Stderr,
				"error: refusing to start %s: declared unauthenticated with no resolved listen address\n",
				l.service)
			fmt.Fprintf(os.Stderr,
				"  BUG: resolve the address before declaring it, or do not declare a surface that binds nothing\n")
			refused = true
			continue
		}
		for _, addr := range l.addrs {
			if !listenAddrIsNonLoopback(addr) {
				continue
			}
			fmt.Fprintf(os.Stderr,
				"error: refusing to start %s on non-loopback listener %q without authentication\n",
				l.service, addr)
			if l.remedy != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", l.remedy)
			}
			refused = true
		}
	}
	return refused
}

// mcpAuthModeAuthenticates reports whether a raw MCP auth-mode string gates
// every request. "" and "none" select the accept-all noneAuthenticator
// (internal/component/mcp/bearer.go), so they do NOT authenticate; bearer,
// bearer-list, and oauth do. Kept as literals (not a config import) so the
// always-on guard names no service package.
func mcpAuthModeAuthenticates(mode string) bool {
	switch mode {
	case "bearer", "bearer-list", "oauth":
		return true
	default:
		return false
	}
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
