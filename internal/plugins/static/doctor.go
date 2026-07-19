// Design: plan/learned/650-static-routes.md -- interface-only next-hop readiness check
// Related: register.go -- doctor check registration (static-interface-nexthop-backend)
// Related: backend_linux.go -- resolveNexthopIndex, the runtime resolve this pre-flights

package static

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// doctorCodeInterfaceNexthopNoBackend is emitted when the config has a static
// route with an interface-only next-hop but no `interface { backend ... }`
// stanza, so the runtime resolve (iface.Resolve) will fail with "no backend
// loaded". Registered in internal/core/diagnostic/codes.go so `ze explain`
// can describe it.
const doctorCodeInterfaceNexthopNoBackend = "doctor-static-interface-nexthop-no-backend"

// staticDoctorChecks declares the static plugin's doctor readiness checks. The
// interface-only next-hop check is the config-time backstop for the runtime
// dependency an interface next-hop has on a loaded iface backend
// (spec-fixit-static-interface-nexthops D-2 = (a)+(b), ai/rules/doctor-checks.md).
func staticDoctorChecks() []registry.DoctorCheckDef {
	return []registry.DoctorCheckDef{{
		Name:         "static-interface-nexthop-backend",
		Phase:        rpc.DoctorPhasePostConfig,
		Order:        720,
		Dependencies: []string{"static"},
		Platforms:    []string{"any"},
		Codes:        []string{doctorCodeInterfaceNexthopNoBackend},
		Check:        checkInterfaceNexthopBackend,
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
