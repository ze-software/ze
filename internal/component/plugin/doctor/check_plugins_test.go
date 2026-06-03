package doctor

import (
	"testing"

	zeplugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func TestCheckPluginBinaries_MissingBinary(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "gone", Run: "/nonexistent/path/to/binary"},
	}
	diags := CheckPluginBinaries(plugins)
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for missing binary")
	}
	if diags[0].Code != "doctor-plugin-missing" {
		t.Errorf("code = %q, want doctor-plugin-missing", diags[0].Code)
	}
}

func TestCheckPluginBinaries_NoDiagForInternalPlugin(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "rib", Internal: true, Run: ""},
	}
	diags := CheckPluginBinaries(plugins)
	if len(diags) != 0 {
		t.Errorf("expected no diags for internal plugin, got %d", len(diags))
	}
}

func TestCheckPluginBinaries_ExternalBuiltin(t *testing.T) {
	builtins := zeplugin.AvailableInternalPlugins()
	if len(builtins) == 0 {
		t.Skip("no built-in plugins registered")
	}
	plugins := []zeplugin.PluginConfig{
		{Name: "test-ext", Run: builtins[0], Internal: false},
	}
	diags := CheckPluginBinaries(plugins)
	found := false
	for _, d := range diags {
		if d.Code == "doctor-plugin-external-builtin" {
			found = true
		}
	}
	if !found {
		t.Error("expected doctor-plugin-external-builtin diagnostic")
	}
}

func TestCheckPluginsRegistered(t *testing.T) {
	names := diagnostic.DoctorCheckNames()
	found := false
	for _, n := range names {
		if n == "plugin-binaries" {
			found = true
		}
	}
	if !found {
		t.Error("plugin-binaries doctor check not found in exported registry")
	}
}
