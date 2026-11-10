// Design: docs/features/ai-first.md -- plugin doctor check bridge
// Overview: registry.go -- runDoctorChecks dispatches to this bridge

package doctor

import (
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func runPluginRegistryChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
	checks := pluginregistry.PluginDoctorChecks()

	var diags []diagnostic.Diagnostic
	for _, check := range checks {
		if check.Phase != phase {
			continue
		}
		if !pluginCheckSupportsPlatform(check.Platforms, ctx) {
			continue
		}
		regCtx := pluginregistry.DoctorCheckContext{
			Tree:      ctx.Tree,
			ConfigDir: ctx.ConfigDir,
			Platform:  ctx.Platform,
		}
		for _, d := range check.Check(regCtx) {
			diags = append(diags, PluginDiagnostic(d))
		}
	}
	return diags
}

// PluginDiagnostic converts one diagnostic a plugin doctor check answered
// into the doctor's own record. It is the one mapping both plugin bridges use
// (runPluginRegistryChecks here, collectDoctorChecksFrom on the daemon
// route), so a field the plugin protocol carries, Fix included, cannot be
// dropped on one route and kept on the other. The severity is copied as the
// plugin wrote it: the set is open, and each bridge decides how to read a
// value outside it.
func PluginDiagnostic(d rpc.DoctorCheckDiagnostic) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     d.Code,
		Severity: diagnostic.Severity(d.Severity),
		Message:  d.Message,
		Fix:      d.Fix,
	}
}

func pluginCheckSupportsPlatform(platforms []string, ctx doctorCheckContext) bool {
	for _, allowed := range platforms {
		if allowed == doctorCheckPlatformAny {
			return true
		}
		if ctx.Platform != nil && allowed == ctx.Platform.Type.String() {
			return true
		}
	}
	return false
}
