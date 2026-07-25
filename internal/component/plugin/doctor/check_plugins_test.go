package doctor

import (
	"testing"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
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
