// Design: docs/features/ai-first.md — plugin readiness check registration
// Overview: registry.go — doctor check registry

package doctor

import "codeberg.org/thomas-mangin/ze/internal/core/diagnostic"

var _ = mustRegisterDoctorCheck(doctorCheck{
	Name:         "plugin-binaries",
	Phase:        doctorCheckPhasePostConfig,
	Order:        700,
	Component:    "plugin",
	Dependencies: []string{"external-binary"},
	Platforms:    []string{doctorCheckPlatformAny},
	Codes:        []string{"doctor-plugin-missing", "doctor-plugin-external-builtin"},
	Check:        checkRegisteredPlugins,
})

func checkRegisteredPlugins(ctx doctorCheckContext) []diagnostic.Diagnostic {
	return checkPlugins(ctx.Plugins)
}
