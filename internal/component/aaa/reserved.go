// Design: docs/architecture/core-design.md -- reserved AAA identity namespace
// Related: login_profiles.go -- login-resolved profiles carry the recovery name
// Related: ../authz/authz.go -- Store.Authorize recognizes these reserved names
//
// The reserved namespace is a contract shared by authentication (this package),
// authorization (authz.Store.Authorize), and the plugin RPC boundary
// (plugin/server). It lives here, in the interface tier both sides already
// import, so neither the authorizer nor the RPC server has to depend on the
// concrete authz package for it.

package aaa

import "strings"

// reservedPrefix marks identities and profile names that live OUTSIDE the
// configuration namespace. A config line is tokenized with strings.Fields, so a
// token can never contain the NUL byte this prefix carries: an operator cannot
// type a reserved name, and ValidateAuthzConfig rejects one defensively (fail
// closed). This is how a strict authorization default keeps reserved
// recovery/internal identities that cannot collide with a user-defined profile
// such as `admin` (spec-fixit-authz-admin-fallthrough R-8).
const reservedPrefix = "\x00ze:"

// ReservedInternalPrefix namespaces internal identities injected by the daemon
// (RPC and plugin dispatch boundaries). authz.Store.Authorize does NOT grant by
// this prefix: only the exact ReservedInternalRPCIdentity bypasses profiles,
// while identities under ReservedInternalPluginPrefix are evaluated against the
// participant's compiled bus command profile. The prefix is un-typeable, so no
// authenticated identity can spoof an internal caller.
const ReservedInternalPrefix = reservedPrefix + "internal:"

// ReservedInternalRPCIdentity is the sole internal identity authz.Store.Authorize
// allows without a profile, because it names no bus participant and has no
// filter to check against. Its sole production caller is
// internal/component/plugin/server/server.go:wrapHandler (plus internal
// post-gate dispatches such as update-route after its bgp:update/bgp:refresh
// bus check).
const ReservedInternalRPCIdentity = ReservedInternalPrefix + "rpc"

// ReservedInternalPluginPrefix namespaces internal identities injected for a
// bus participant (plugin endpoint) dispatching a CLI command. Every identity
// under this prefix is evaluated against the endpoint's compiled bus command
// profile (PluginCommandProfile) via authz's prefix matcher and fails closed
// when no profile allows the command. The endpoint name is preserved for
// accounting and audit.
const ReservedInternalPluginPrefix = ReservedInternalPrefix + "plugin:"

// ReservedSharedAPIUsername is the identity REST and gRPC inject after they
// classify a caller as either a validated shared-token request or a no-auth
// loopback request. Both modes share one identity because the transport marks
// no-auth callers read-only before command authorization, while a validated
// shared token retains write authority. Per-user authentication never emits
// this name and keeps the authenticated username.
const ReservedSharedAPIUsername = reservedPrefix + "shared-api"

// ReservedRecoveryProfile is the allow-all break-glass profile delivered to the
// `ze init` bootstrap admin through login-resolved profiles ONLY (never a config
// assignment, so it cannot flip an operator's RBAC posture).
// authz.Store.Authorize honors it regardless of the profiles the store defines,
// so a strict default can never lock an operator out of a box whose
// authorization config is wrong or partial.
const ReservedRecoveryProfile = reservedPrefix + "recovery"

// IsReservedName reports whether name lives in the reserved namespace and
// therefore may not appear in, or be referenced by, configuration.
func IsReservedName(name string) bool {
	return strings.HasPrefix(name, reservedPrefix)
}

// PluginEndpointFromIdentity extracts the plugin endpoint name from an internal
// plugin identity carrying ReservedInternalPluginPrefix. Returns ("", false)
// when username does not bear the prefix or carries an empty endpoint name.
func PluginEndpointFromIdentity(username string) (endpoint string, ok bool) {
	endpoint, ok = strings.CutPrefix(username, ReservedInternalPluginPrefix)
	if !ok {
		return "", false
	}
	if endpoint == "" {
		return "", false
	}
	return endpoint, true
}

// FilterReservedNames returns names with every reserved name removed. Apply it to
// profile names that arrive from an UNTRUSTED AAA server (a RADIUS Filter-Id, a
// TACACS+ priv-level mapping): a compromised or hostile server must not be able to
// name the break-glass recovery profile, or any reserved identity, over the wire.
// The only legitimate source of a reserved profile is the code-controlled local
// backend (cmd/ze/hub usersFromZefsDB). The input slice is not modified; a nil or
// all-reserved input yields an empty slice so the caller fails closed.
func FilterReservedNames(names []string) []string {
	filtered := make([]string, 0, len(names))
	for _, n := range names {
		if !IsReservedName(n) {
			filtered = append(filtered, n)
		}
	}
	return filtered
}
