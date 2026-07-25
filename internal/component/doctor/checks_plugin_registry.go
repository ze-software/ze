// Design: docs/features/ai-first.md -- plugin doctor check bridge
// Overview: registry.go -- runDoctorChecks dispatches to this bridge

package doctor

import (
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func runPluginRegistryChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
	checks := pluginregistry.PluginDoctorChecks()

	var diags []diagnostic.Diagnostic
	for _, check := range checks {
		if string(check.Phase) != string(phase) {
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
			diags = append(diags, diagnostic.Diagnostic{
				Code:     d.Code,
				Severity: diagnostic.Severity(d.Severity),
				Message:  d.Message,
			})
		}
	}
	return diags
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
