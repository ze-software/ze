// Design: docs/architecture/fleet-config.md -- request-bound local device-scope projection
// Related: command.go -- CommandContext, Dispatcher.authorizer, isAuthorizedCommandArgs

package server

import (
	"github.com/ze-software/ze/internal/component/aaa"
)

type fleetScopeMode uint8

const (
	fleetScopeDenyAll fleetScopeMode = iota
	fleetScopeAllowAll
	fleetScopeEvaluate
)

// FleetDeviceScope projects per-device read visibility for one dispatched fleet
// query against the caller's authenticated policy without performing remote
// authorization per device. The zero value denies every device.
type FleetDeviceScope struct {
	mode      fleetScopeMode
	username  string
	scopeAuth aaa.LocalScopeAuthorizer
}

// ResolveFleetDeviceScope derives the request-bound device visibility scope for
// ctx. It resolves the effective authorizer in the same precedence order as
// Dispatcher.isAuthorizedCommandArgs: ctx.Authorizer first, then the server
// dispatcher's authorizer.
//
// When no authorizer is installed on either the request or the dispatcher
// (explicit policy-free installation), every non-empty device name is allowed.
// When an authorizer is installed and does not implement
// aaa.LocalScopeAuthorizer, the returned scope fails closed.
func ResolveFleetDeviceScope(ctx *CommandContext) FleetDeviceScope {
	if ctx == nil {
		return FleetDeviceScope{mode: fleetScopeDenyAll}
	}

	var effective aaa.Authorizer
	if ctx.Authorizer != nil {
		effective = ctx.Authorizer
	} else if d := ctx.Dispatcher(); d != nil {
		effective = d.authorizer
	}

	// refuse-by-default: guards FleetDeviceScope.AllowsDevice -- a dispatcher and
	// request with no installed authorizer operate in explicit no-RBAC mode,
	// matching Dispatcher.isAuthorizedCommandArgs.
	if effective == nil {
		return FleetDeviceScope{mode: fleetScopeAllowAll}
	}

	scopeAuth, ok := effective.(aaa.LocalScopeAuthorizer)
	if !ok {
		return FleetDeviceScope{mode: fleetScopeDenyAll}
	}

	return FleetDeviceScope{
		mode:      fleetScopeEvaluate,
		username:  ctx.Username,
		scopeAuth: scopeAuth,
	}
}

// AllowsDevice reports whether the caller represented by s may read fleet state
// for the named device. Empty names are always denied.
func (s FleetDeviceScope) AllowsDevice(name string) bool {
	if name == "" {
		return false
	}

	switch s.mode {
	case fleetScopeDenyAll:
		return false
	case fleetScopeAllowAll:
		return true
	case fleetScopeEvaluate:
		if s.scopeAuth == nil {
			return false
		}
		cmd, args := aaa.FleetDeviceReadTokens(name)
		return s.scopeAuth.AuthorizeLocalScope(s.username, cmd, args, true)
	}
	return false
}

// FilterDeviceNames returns the subset of names permitted by s, preserving
// input order. Empty names are omitted.
func (s FleetDeviceScope) FilterDeviceNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	if s.mode == fleetScopeDenyAll {
		return nil
	}
	allowed := make([]string, 0, len(names))
	for _, name := range names {
		if s.AllowsDevice(name) {
			allowed = append(allowed, name)
		}
	}
	return allowed
}

// AuthorizeFleetDevice reports whether ctx may read fleet state for the named
// device using local scope projection.
func AuthorizeFleetDevice(ctx *CommandContext, name string) bool {
	return ResolveFleetDeviceScope(ctx).AllowsDevice(name)
}
