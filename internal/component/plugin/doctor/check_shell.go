// Design: docs/architecture/doctor-and-health-checks.md -- the shell an external plugin start forks
// Related: check_plugins.go -- the check over the binary each run string names
// Related: ../shell.go -- Shell and ShellAvailable, the dependency reported here

package doctor

import (
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func checkPluginShell(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return diagnosePluginShell(ctx.Plugins, zeplugin.ShellAvailable())
}

// diagnosePluginShell reports the shell when the config names an external
// plugin and this host does not carry the shell that starts it. A config that
// names no external plugin forks nothing, so it depends on no shell and gets no
// diagnostic.
//
// One host fact makes one diagnostic, and it names every plugin the operator
// configured, because a shell that is absent stops all of them for one reason.
//
// The shell answer is a parameter so a test states it: no test can take /bin/sh
// away from the host it runs on.
func diagnosePluginShell(plugins []zeplugin.PluginConfig, shellErr error) []diagnostic.Diagnostic {
	if shellErr == nil {
		return nil
	}

	var named textbuf.Buffer
	count := 0
	for _, configured := range plugins {
		if configured.Internal {
			continue
		}
		if configured.Run == "" {
			continue
		}
		if count > 0 {
			named.Str(", ")
		}
		named.Str(configured.Name)
		count++
	}
	if count == 0 {
		return nil
	}

	var message textbuf.Buffer
	message.Err(shellErr).Str("; the plugin block starts ").Str(named.String()).
		Str(" with `").Str(zeplugin.Shell).Str(" -c <run>`")

	return []diagnostic.Diagnostic{{
		Code:      codePluginShellMissing,
		Severity:  diagnostic.SeverityError,
		Message:   message.String(),
		Path:      zeplugin.Shell,
		Help:      "Run the plugin as `internal` where a built-in plugin provides it, or install a shell on this host. A gokrazy appliance image carries no shell.",
		FixSafety: diagnostic.SafetyRequiresHumanReview,
	}}
}
