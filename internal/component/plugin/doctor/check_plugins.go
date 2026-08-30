// Design: docs/features/ai-first.md -- plugin binary readiness check

package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func checkPlugins(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return CheckPluginBinaries(ctx.Plugins)
}

func CheckPluginBinaries(plugins []zeplugin.PluginConfig) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	for _, p := range plugins {
		if p.Run == "" {
			continue
		}
		parts := strings.Fields(p.Run)
		if len(parts) == 0 {
			continue
		}

		if p.Internal {
			continue
		}

		binary := parts[0]
		if filepath.IsAbs(binary) || strings.HasPrefix(binary, "./") || strings.HasPrefix(binary, "../") {
			if _, err := os.Stat(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codePluginMissing,
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not found: " + binary,
					Path:     binary,
				})
			}
		} else {
			if _, err := exec.LookPath(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codePluginMissing,
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not on PATH: " + binary,
				})
			}
		}
	}
	return diags
}
