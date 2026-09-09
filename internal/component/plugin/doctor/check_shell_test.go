package doctor

import (
	"errors"
	"slices"
	"strings"
	"testing"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// errShellAbsent is the answer the probe gives on a host with no shell. The
// tests below pass it as an argument: no test can take /bin/sh away from the
// host it runs on, so the check takes the probe's answer rather than reading
// the filesystem itself.
var errShellAbsent = errors.New("the shell /bin/sh that starts an external plugin is absent: stat /bin/sh: no such file or directory")

// VALIDATES: an operator whose config names an external plugin is told the
// shell is missing, by name, before the daemon tries to start that plugin.
// PREVENTS: an appliance image with no shell reporting a plugin that "did not
// start" and never naming the dependency that is absent.
func TestPluginShellMissingWithAnExternalPlugin(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "collector", Run: "./collector.py"},
	}

	diags := diagnosePluginShell(plugins, errShellAbsent)

	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != codePluginShellMissing {
		t.Errorf("code = %q, want %q", diags[0].Code, codePluginShellMissing)
	}
	if diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("severity = %q, want %q", diags[0].Severity, diagnostic.SeverityError)
	}
	if !strings.Contains(diags[0].Message, zeplugin.Shell) {
		t.Errorf("the message must name the absent shell, and it says %q", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, "collector") {
		t.Errorf("the message must name the plugin the operator configured, and it says %q", diags[0].Message)
	}
}

// VALIDATES: a host that carries the shell reports nothing.
// PREVENTS: a check that fires on every host with an external plugin.
func TestPluginShellPresentIsSilent(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "collector", Run: "./collector.py"},
	}

	diags := diagnosePluginShell(plugins, nil)

	if len(diags) != 0 {
		t.Errorf("a host that carries the shell needs no diagnostic, and it reported %d", len(diags))
	}
}

// VALIDATES: a config that forks nothing needs no shell, so a host without one
// is ready. An internal plugin runs in the daemon's own process.
// PREVENTS: an appliance running only internal plugins, which is the supported
// arrangement, being reported as not ready.
func TestPluginShellUnusedWithoutAnExternalPlugin(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "rib", Internal: true},
		{Name: "gr", Internal: true, Run: "ze plugin bgp-gr"},
	}

	diags := diagnosePluginShell(plugins, errShellAbsent)

	if len(diags) != 0 {
		t.Errorf("a config that forks nothing needs no shell, and it reported %d diagnostics", len(diags))
	}
}

// VALIDATES: the check reaches `ze doctor` through the registry.
// PREVENTS: a check that exists and never runs.
func TestPluginShellCheckRegistered(t *testing.T) {
	if !slices.Contains(diagnostic.DoctorCheckNames(), "plugin-shell") {
		t.Error("plugin-shell doctor check not found in the exported registry")
	}
}
