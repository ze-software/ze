// Design: plan/learned/650-static-routes.md -- interface-only next-hop readiness check
// Related: register.go -- doctor check registration (static-interface-nexthop-backend)
// Related: inject.go -- routeManager.skipped + activeRouteManager the route-skipped check reads
// Related: backend_linux.go -- resolveNexthopIndex, the runtime resolve this pre-flights

package static

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorCodeInterfaceNexthopNoBackend is emitted when the config has a static
// route with an interface-only next-hop but no `interface { backend ... }`
// stanza, so the runtime resolve (iface.Resolve) will fail with "no backend
// loaded". Registered in internal/core/diagnostic/codes.go so `ze explain`
// can describe it.
const doctorCodeInterfaceNexthopNoBackend = "doctor-static-interface-nexthop-no-backend"

// doctorCodeRouteSkipped is emitted when the running static plugin has skipped
// one or more routes the backend could not program (per-route isolation,
// spec-fixit-static-per-route-isolation). Registered in
// internal/core/diagnostic/codes.go so `ze explain` can describe it.
const doctorCodeRouteSkipped = "doctor-static-route-skipped"

// staticDoctorChecks declares the static plugin's doctor readiness checks. The
// interface-only next-hop check is the config-time backstop for the runtime
// dependency an interface next-hop has on a loaded iface backend
// (spec-fixit-static-interface-nexthops D-2 = (a)+(b), ai/rules/repo-maintenance.md).
// The route-skipped check surfaces routes the running plugin isolated at apply
// time so a skip is never a silent no-op (spec-fixit-static-per-route-isolation
// AC-3, ai/rules/evidence.md).
func staticDoctorChecks() []registry.DoctorCheckDef {
	return []registry.DoctorCheckDef{
		{
			Name:         "static-interface-nexthop-backend",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        720,
			Dependencies: []string{"static"},
			Platforms:    []string{"any"},
			Codes:        []string{doctorCodeInterfaceNexthopNoBackend},
			Check:        checkInterfaceNexthopBackend,
		},
		{
			Name:         "static-route-skipped",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        721,
			Dependencies: []string{"static"},
			Platforms:    []string{"any"},
			Codes:        []string{doctorCodeRouteSkipped},
			Check:        checkRouteSkipped,
		},
	}
}

// checkRouteSkipped reports the routes the running static plugin could not
// program and skipped (per-route isolation). It reads the live route manager
// (activeRouteManager); when nil -- the offline `ze doctor <config>` path with
// no running daemon, or an external forked static plugin -- there is no runtime
// skip state to report and it stays silent (the WARN logs and `static show`
// remain the always-on surfaces). It is a WARNING, not an error: the daemon is
// running as designed with the good routes programmed; the operator is told
// which prefixes are unrouted and why so a skip is never silent.
func checkRouteSkipped(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	rm := activeRouteManager.Load()
	if rm == nil {
		return nil
	}
	skipped := rm.skippedRoutes()
	if len(skipped) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("static routes skipped (rest of the section kept programmed): ")
	for i, sk := range skipped {
		if i > 0 {
			tb.Str("; ")
		}
		tb.Str(sk.route.Prefix.String()).Str(" (").Str(sk.reason).Byte(')')
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     doctorCodeRouteSkipped,
		Severity: "warning",
		Message:  tb.String(),
	}}
}

// checkInterfaceNexthopBackend warns when a static route forwards over an
// interface-only next-hop but no iface backend is configured. Such a route
// cannot resolve its next-hop interface to an ifindex at runtime
// (iface.Resolve -> "no backend loaded"), and the whole static section fails.
// It is a WARNING, not an error: the check cannot see whether the interface is
// externally created, and the runtime path (C-2) remains the authoritative
// backstop -- this only surfaces the dependency before the daemon starts.
func checkInterfaceNexthopBackend(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if !hasInterfaceOnlyNextHop(tree) {
		return nil
	}
	if ifaceBackendConfigured(tree) {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     doctorCodeInterfaceNexthopNoBackend,
		Severity: "warning",
		Message: "a static route forwards over an interface-only next-hop but no " +
			"`interface { backend ... }` stanza is configured; the next-hop interface " +
			"cannot be resolved at runtime and the static section will fail to load",
	}}
}

// hasInterfaceOnlyNextHop reports whether any static route declares a next-hop
// under `next > interface` (the address-less, interface-only form). It walks
// static > table > route > next > interface.
func hasInterfaceOnlyNextHop(tree *config.Tree) bool {
	static := tree.GetContainer("static")
	if static == nil {
		return false
	}
	for _, table := range static.GetListOrdered("table") {
		for _, route := range table.Value.GetListOrdered("route") {
			next := route.Value.GetContainer("next")
			if next == nil {
				continue
			}
			if len(next.GetListOrdered("interface")) > 0 {
				return true
			}
		}
	}
	return false
}

// ifaceBackendConfigured reports whether the config declares a non-empty
// `interface { backend ... }` leaf, which is the only thing that loads an iface
// backend (iface/register.go OnConfigure). Mirrors the tree walk doctor's
// kernel-module check uses.
func ifaceBackendConfigured(tree *config.Tree) bool {
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return false
	}
	backend, _ := ifaceBlock.Get("backend")
	return backend != ""
}
