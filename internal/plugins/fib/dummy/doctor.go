// Design: docs/architecture/doctor-and-health-checks.md -- fib-dummy post-config doctor check.
// Related: register.go -- plugin registration and configRoot
// Related: register_doctor_codes.go -- codeDoctorHoldsRoutes registration

package fibdummy

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// doctorHoldsRoutesCheckName is the registered name of the post-config
	// doctor check reporting that `fib { dummy { } }` holds system RIB routes.
	doctorHoldsRoutesCheckName = "fib-dummy-holds-routes"

	// doctorHoldsRoutesOrder places the check right beside
	// plugin-dataplane-writer (719) in the post-config phase.
	doctorHoldsRoutesOrder = 720

	doctorHoldsRoutesFix = "delete fib dummy"
)

var holdsRoutesDoctorCheck = diagnostic.DoctorCheck{
	Name:         doctorHoldsRoutesCheckName,
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        doctorHoldsRoutesOrder,
	Component:    pluginName,
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeDoctorHoldsRoutes},
	Check:        checkFIBDummyHoldsRoutes,
}

// checkFIBDummyHoldsRoutes reports the consequence of `fib { dummy { } }` in
// `ze doctor` and `show doctor`: WARNING when the configuration selects any
// plugin declaring NeedsDataPlane (naming every such plugin), INFO when it
// selects none, and nothing when `fib { dummy { } }` is absent.
func checkFIBDummyHoldsRoutes(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	// refuse-by-default: guards holdsRoutesDoctorCheck -- a context holding a non-*Tree carries no parsed config
	if !ok {
		return nil
	}
	// refuse-by-default: guards holdsRoutesDoctorCheck -- missing-config phase supplies a nil tree
	if tree == nil {
		return nil
	}
	if !fibDummyPresent(tree) {
		return nil
	}

	producers := selectedDataPlaneProducers(tree, ctx)
	if len(producers) > 0 {
		var msg textbuf.Buffer
		msg.Str("fib dummy is configured, and this config loads ").
			Str(textbuf.Join(producers, ", ")).
			Str(", whose routes reach forwarding only through a data-plane writer: fib dummy holds those routes and writes them to no forwarding table")
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorHoldsRoutes,
			Severity: diagnostic.SeverityWarning,
			Message:  msg.String(),
			Fix:      doctorHoldsRoutesFix,
		}}
	}

	return []diagnostic.Diagnostic{{
		Code:     codeDoctorHoldsRoutes,
		Severity: diagnostic.SeverityInfo,
		Message: "fib dummy is configured: ze writes no route the system RIB selects to any forwarding table " +
			"(named-table static routes, policy routing, IPsec tunnel-source guards, and interface addresses are outside the system RIB and untouched)",
		Fix: doctorHoldsRoutesFix,
	}}
}

// fibDummyPresent reports whether tree holds an active `fib { dummy { } }` container.
func fibDummyPresent(tree *config.Tree) bool {
	fib := tree.GetContainer("fib")
	if fib == nil {
		return false
	}
	if fib.IsInactive() {
		return false
	}
	dummy := fib.GetContainer("dummy")
	if dummy == nil {
		return false
	}
	return !dummy.IsInactive()
}

// selectedDataPlaneProducers returns the sorted, deduplicated names of every
// plugin selected by tree or ctx.Plugins that declares NeedsDataPlane.
func selectedDataPlaneProducers(tree *config.Tree, ctx diagnostic.DoctorCheckContext) []string {
	seen := make(map[string]bool)
	var producers []string
	add := func(name string) {
		if name == "" {
			return
		}
		if seen[name] {
			return
		}
		if !registry.NeedsDataPlane(name) {
			return
		}
		seen[name] = true
		producers = append(producers, name)
	}

	for _, name := range registry.PluginsForConfigPaths(config.CollectContainerPaths(tree)) {
		add(name)
	}
	for i := range ctx.Plugins {
		add(ctx.Plugins[i].CompiledIdentity())
	}
	slices.Sort(producers)
	return producers
}
