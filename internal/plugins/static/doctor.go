// Design: docs/architecture/static-routes.md -- interface-only next-hop readiness check
// Related: register.go -- doctor check registration (static-interface-nexthop-backend)
// Related: inject.go -- routeManager.skipped + activeRouteManager the route-skipped check reads
// Related: backend_linux.go -- resolveNexthopIndex, the runtime resolve this pre-flights

package static

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/routingtable"
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

// doctorCodeNoFIBWriter is emitted when the config declares a main-table static
// route and NO FIB plugin can program it: the operator declared none and the
// registry holds no writer for the data plane the config selects. Registered in
// internal/core/diagnostic/codes.go so `ze explain` can describe it.
const doctorCodeNoFIBWriter = "doctor-static-no-fib-writer"

// staticDoctorChecks declares the static plugin's doctor readiness checks. The
// interface-only next-hop check is the config-time backstop for the runtime
// dependency an interface next-hop has on a loaded iface backend
// (spec-fixit-static-interface-nexthops D-2 = (a)+(b), ai/rules/repo-maintenance.md).
// The route-skipped check surfaces routes the running plugin isolated at apply
// time so a skip is never a silent no-op (spec-fixit-static-per-route-isolation
// AC-3, ai/rules/evidence.md).
// anyPlatform is the platform selector for a check that runs everywhere. Every
// static check does: the config-time ones read a tree, and the runtime one reads
// the plugin's own skip state.
const anyPlatform = "any"

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
			Name:         "static-fib-writer",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        719,
			Dependencies: []string{pluginName},
			Platforms:    []string{anyPlatform},
			Codes:        []string{doctorCodeNoFIBWriter},
			Check:        checkFIBWriter,
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

// checkFIBWriter reports a config whose main-table static routes can reach no
// data plane at all.
//
// A main-table static route is a Loc-RIB Path: the system RIB arbitrates it
// against every other protocol offering the prefix, and a FIB plugin writes the
// winner. The operator does not have to ask for that plugin. The engine loads
// the writer that declares the data plane `interface { backend }` selects, so a
// config with static routes and no `fib { ... }` block starts and programs them
// (owner decision, 2026-09-06; appendDataPlaneWriter in
// internal/component/plugin/server/startup_autoload.go performs it).
//
// What is left to report is the state auto-loading cannot repair: no plugin
// programs the selected data plane. The routes then reach arbitration and stop
// there, which is why this stays an ERROR rather than a warning. It is silent
// when the operator declared a `fib { ... }` backend of their own, because that
// block loads its plugin whatever this check thinks of it.
func checkFIBWriter(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if !hasMainTableRoute(tree) {
		return nil
	}
	if fibBackendConfigured(tree) {
		return nil
	}
	return fibWriterDiagnostics(iface.BackendNameFromTree(tree.ToMap()))
}

// fibWriterDiagnostics judges one resolved data-plane name, and answers nothing
// for a build that supplies none.
//
// The empty name is never something an operator wrote. BackendNameFromTree
// (internal/component/iface/register.go) falls back to defaultBackendName for a
// config that names no backend, and that constant is "netlink" on Linux and ""
// on every other platform (internal/component/iface/default_other.go). Reading
// the empty name as a data plane nothing programs made this check answer about
// the MACHINE running `ze doctor` instead of the config in front of it: the
// same static config was clean on the Linux host that serves it and reported
// unroutable on a developer's macOS checkout
// (plan/journal/gate-verdict-depends-on-the-machine.md).
//
// Silence is the accurate answer rather than a missing one, and it is the whole
// of what this guard decides. A build with no data plane at all holds no
// opinion about which data plane a config should select, so there is nothing
// here to tell an operator. Where a build HAS one, the name is non-empty and
// every config reaches the lookup below.
func fibWriterDiagnostics(dataPlane string) []rpc.DoctorCheckDiagnostic {
	if dataPlane == "" {
		return nil
	}
	if _, resolved := registry.PluginForDataPlane(dataPlane); resolved {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("a static route is declared in the main table, and no FIB plugin programs the ")
	tb.Str(dataPlane).Str(" data plane this config selects at `interface { backend }`; ")
	tb.Str("the system RIB selects the route and nothing writes it, so it would never reach ")
	tb.Str("the data plane. Name a backend Ze programs, or add the `fib { ... }` block for ")
	tb.Str("the one you want")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     doctorCodeNoFIBWriter,
		Severity: "error",
		Message:  tb.String(),
	}}
}

// fibBackendConfigured reports whether the config declares a backend under
// `fib { ... }`. An EMPTY `fib { }` block declares no backend, so it selects no
// plugin and leaves the choice to the engine, which is what makes this a
// question about the containers under fib rather than about fib itself.
func fibBackendConfigured(tree *config.Tree) bool {
	fib := tree.GetContainer("fib")
	if fib == nil {
		return false
	}
	return len(fib.ContainerNames()) > 0
}

// hasMainTableRoute reports whether any static route is declared in the main
// table. The table name "default" and an omitted table both mean the main table;
// every other name resolves through the routing-table registry to a named table
// static programs directly.
func hasMainTableRoute(tree *config.Tree) bool {
	static := tree.GetContainer("static")
	if static == nil {
		return false
	}
	for _, table := range static.GetListOrdered("table") {
		if table.Key != "" && table.Key != routingtable.MainTableName {
			continue
		}
		if len(table.Value.GetListOrdered("route")) > 0 {
			return true
		}
	}
	return false
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
