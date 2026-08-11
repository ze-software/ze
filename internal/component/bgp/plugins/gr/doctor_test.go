package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestCheckGRInProcessFlagsAnExternalGR pins the diagnosis for the one
// operator-reachable arrangement that leaves the LLGR egress filter blind for
// the whole life of the daemon.
//
// VALIDATES: `plugin { external X { run "ze plugin bgp-gr"; } }` is reported.
// PREVENTS: the daemon silently withdrawing every stale route from every peer,
// LLGR-capable peers included, with nothing said until the first stale route
// reaches the filter.
//
// The config path is real: ExtractPluginsFromTree (config/loader.go) leaves
// PluginConfig.Internal false for a `run` command that is not the "ze.<name>"
// spelling, and Process.StartWithContext then takes startExternal, so
// RunGRPlugin -- the only caller of setEgressState -- runs in the child.
func TestCheckGRInProcessFlagsAnExternalGR(t *testing.T) {
	diags := checkGRInProcess(diagnostic.DoctorCheckContext{
		Plugins: []zeplugin.PluginConfig{
			{Name: "gr", Run: "ze plugin bgp-gr"},
		},
	})

	require.Len(t, diags, 1, "an out-of-process bgp-gr must be reported")
	assert.Equal(t, codeGROutOfProcess, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "bgp-gr")
}

// TestCheckGRInProcessAcceptsTheSupportedArrangements is the contrast: the
// check must stay silent for every configuration in which the filter's state
// is either loaded or correctly absent. Without this, "warn always" would
// satisfy the positive above and be useless.
func TestCheckGRInProcessAcceptsTheSupportedArrangements(t *testing.T) {
	for _, tc := range []struct {
		name   string
		plugin zeplugin.PluginConfig
	}{
		// `use bgp-gr`: RunGRPlugin runs in the daemon and stores the state.
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

// TestCheckGRInProcessAcceptsAnInternalGRBesideAnExternalOne pins the answer
// for the degenerate config that names bgp-gr twice.
//
// VALIDATES: an in-process bgp-gr silences the check for every sibling.
// PREVENTS: a false positive. setEgressState stores into a package-level
// pointer (gr_egress.go), so the daemon that loads the engine once answers for
// every destination, and the child copy costs the filter nothing. Judging each
// `run` line on its own told the operator the filter was blind when it held
// the whole state.
func TestCheckGRInProcessAcceptsAnInternalGRBesideAnExternalOne(t *testing.T) {
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
			assert.Empty(t, diags, "the daemon holds the egress state, so nothing is owed")
		})
	}
}

// TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess is the contrast that
// stops the pre-pass above from silencing the check for any internal plugin at
// all: only an in-process bgp-gr loads the egress state.
func TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess(t *testing.T) {
	diags := checkGRInProcess(diagnostic.DoctorCheckContext{
		Plugins: []zeplugin.PluginConfig{
			{Name: "rib", Internal: true, Run: "bgp-rib"},
			{Name: "gr", Run: "ze plugin bgp-gr"},
		},
	})

	require.Len(t, diags, 1, "an internal bgp-rib stores no LLGR egress state")
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
// `ze explain`: doctor's own TestDoctorRegisteredCheckCodesHaveMetadata fails
// otherwise, and an operator meeting the code has nowhere to read what it means.
func TestGRDoctorCodeIsExplainable(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeGROutOfProcess),
		"the code registered by register.go must carry metadata")
}
