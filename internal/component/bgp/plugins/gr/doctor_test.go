package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestCheckGRInProcessFlagsAnExternalGR pins the diagnosis for the
// operator-reachable arrangement in which bgp-gr refuses to start, so the
// daemon does not start.
//
// VALIDATES: `plugin { external X { run "ze plugin bgp-gr"; } }` is reported
// as an error, from the config alone.
// PREVENTS: the operator meeting the refusal only when the daemon fails to
// start, with nothing in `ze doctor` to say so beforehand.
//
// The config path is real: ExtractPluginsFromTree (config/loader.go) leaves
// PluginConfig.Internal false for a `run` command that is not the "ze.<name>"
// spelling, and Process.StartWithContext then takes startExternal, so
// runGRPlugin runs in the child, where it refuses (gr.go).
func TestCheckGRInProcessFlagsAnExternalGR(t *testing.T) {
	diags := checkGRInProcess(diagnostic.DoctorCheckContext{
		Plugins: []zeplugin.PluginConfig{
			{Name: "gr", Run: "ze plugin bgp-gr"},
		},
	})

	require.Len(t, diags, 1, "an out-of-process bgp-gr must be reported")
	assert.Equal(t, codeGROutOfProcess, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "bgp-gr")
}

// TestCheckGRInProcessAcceptsTheSupportedArrangements is the contrast: the
// check must stay silent for every configuration in which bgp-gr runs in the
// daemon or is not named at all. Without this, "warn always" would
// satisfy the positive above and be useless.
func TestCheckGRInProcessAcceptsTheSupportedArrangements(t *testing.T) {
	for _, tc := range []struct {
		name   string
		plugin zeplugin.PluginConfig
	}{
		// `use bgp-gr`: runGRPlugin runs in the daemon and stores the state.
		{"internal use", zeplugin.PluginConfig{Name: "gr", Internal: true, Run: "bgp-gr"}},
		// `run "ze.bgp-gr"`: MarkInternalPlugin resolves it to the same thing.
		{"internal run", zeplugin.PluginConfig{Name: "gr", Internal: true, Run: "ze.bgp-gr"}},
		// Another plugin entirely, external: not this check's business.
		{"other external", zeplugin.PluginConfig{Name: "obs", Run: "ze plugin bgp-watchdog"}},
		// An external program whose command line merely mentions the name.
		{"name in an argument", zeplugin.PluginConfig{Name: "obs", Run: "./observer.run --label bgp-gr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := checkGRInProcess(diagnostic.DoctorCheckContext{
				Plugins: []zeplugin.PluginConfig{tc.plugin},
			})
			assert.Empty(t, diags, "no diagnostic is owed for this arrangement")
		})
	}
}

// TestCheckGRInProcessFlagsAnExternalGRBesideAnInternalOne pins the answer
// for the config that names bgp-gr twice.
//
// VALIDATES: an in-process bgp-gr does not silence the report on a forked
// sibling.
// PREVENTS: a false negative. The forked copy refuses to start whatever runs
// beside it (runGRPlugin, gr.go), so the daemon does not start with this
// config, and the check must say so.
func TestCheckGRInProcessFlagsAnExternalGRBesideAnInternalOne(t *testing.T) {
	for _, tc := range []struct {
		name     string
		internal zeplugin.PluginConfig
	}{
		{"use", zeplugin.PluginConfig{Name: "grin", Internal: true, Run: "bgp-gr"}},
		{"run ze.bgp-gr", zeplugin.PluginConfig{Name: "grin", Internal: true, Run: "ze.bgp-gr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := checkGRInProcess(diagnostic.DoctorCheckContext{
				Plugins: []zeplugin.PluginConfig{
					tc.internal,
					{Name: "grout", Run: "ze plugin bgp-gr"},
				},
			})
			require.Len(t, diags, 1, "the forked sibling refuses to start, so it is reported")
			assert.Equal(t, codeGROutOfProcess, diags[0].Code)
			assert.Contains(t, diags[0].Message, "plugin grout:")
		})
	}
}

// TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess pins that an
// internal plugin of another name does not silence the report.
func TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess(t *testing.T) {
	diags := checkGRInProcess(diagnostic.DoctorCheckContext{
		Plugins: []zeplugin.PluginConfig{
			{Name: "rib", Internal: true, Run: "bgp-rib"},
			{Name: "gr", Run: "ze plugin bgp-gr"},
		},
	})

	require.Len(t, diags, 1, "an internal bgp-rib does not stop the forked bgp-gr refusing")
	assert.Equal(t, codeGROutOfProcess, diags[0].Code)
}

// TestCheckGRInProcessMatchesAPathQualifiedLaunch covers the spelling the
// engine itself uses when ze is not on PATH: startExternal runs the command
// through /bin/sh, and a deployment can name the binary by path.
func TestCheckGRInProcessMatchesAPathQualifiedLaunch(t *testing.T) {
	diags := checkGRInProcess(diagnostic.DoctorCheckContext{
		Plugins: []zeplugin.PluginConfig{
			{Name: "gr", Run: "/usr/local/bin/ze plugin bgp-gr"},
		},
	})

	require.Len(t, diags, 1, "a path-qualified launch of the same plugin is the same arrangement")
	assert.Equal(t, codeGROutOfProcess, diags[0].Code)
}

// TestGRDoctorCodeIsExplainable keeps the reported code answerable by
// `ze show diagnostic`: doctor's own TestDoctorRegisteredCheckCodesHaveMetadata fails
// otherwise, and an operator meeting the code has nowhere to read what it means.
func TestGRDoctorCodeIsExplainable(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeGROutOfProcess),
		"the code registered by register.go must carry metadata")
}
