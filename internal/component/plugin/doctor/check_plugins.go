// Design: docs/features/ai-first.md -- plugin binary readiness check

package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	zeplugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func checkPlugins(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return CheckPluginBinaries(ctx.Plugins)
}

func CheckPluginBinaries(plugins []zeplugin.PluginConfig) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	builtins := zeplugin.AvailableInternalPlugins()
	builtinSet := make(map[string]bool, len(builtins))
	for _, name := range builtins {
		builtinSet[name] = true
	}

	for _, p := range plugins {
		if p.Run == "" {
			continue
		}
		parts := strings.Fields(p.Run)
		if len(parts) == 0 {
			continue
		}

		if !p.Internal || strings.HasPrefix(p.Run, "ze.") {
			for _, name := range matchExternalBuiltinTokens(parts, builtinSet) {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-plugin-external-builtin",
					Severity: diagnostic.SeverityWarning,
					Message:  "plugin " + p.Name + ": command " + p.Run + " matches built-in " + name + "; use plugin { internal " + p.Name + " { use " + name + " } } for in-process execution",
				})
			}
		}

		if p.Internal {
			continue
		}

		binary := parts[0]
		if filepath.IsAbs(binary) || strings.HasPrefix(binary, "./") || strings.HasPrefix(binary, "../") {
			if _, err := os.Stat(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-plugin-missing",
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not found: " + binary,
					Path:     binary,
				})
			}
		} else {
			if _, err := exec.LookPath(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-plugin-missing",
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not on PATH: " + binary,
				})
			}
		}
	}
	return diags
}

func matchExternalBuiltinTokens(tokens []string, builtins map[string]bool) []string {
	seen := make(map[string]bool)
	var matched []string
	for _, token := range tokens {
		name := strings.TrimPrefix(token, "ze.")
		name = filepath.Base(name)
		if builtins[name] && !seen[name] {
			seen[name] = true
			matched = append(matched, name)
		}
	}
	return matched
}
