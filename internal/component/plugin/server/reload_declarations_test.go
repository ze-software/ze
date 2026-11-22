package server

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// startedDeclarations is what a daemon booted with: two external plugins, as
// the first pass answers them.
func startedDeclarations() []plugin.PluginConfig {
	return []plugin.PluginConfig{
		{Name: "live-stages", Run: "le test fixture reload/live-stages stages.script", Encoder: "json"},
		{Name: "acme-monitor", Run: "./acme-monitor", Encoder: "json"},
	}
}

// TestDeclarationChangesNamesEachDifference: the comparison of the plugin
// declarations a daemon started with and the ones a reload read. Method: one
// candidate list for each way it can differ, and the phrases compared whole.
//
// VALIDATES: AC-12 at the comparison. An added, a removed and a changed
// plugin are each named, a changed one with the leaves that differ, in name
// order; the same declarations in another order are no change.
// PREVENTS: a reload refused for the order of the `plugin` block, and a
// changed `respawn` or `timeout` passing because only `run` was compared.
func TestDeclarationChangesNamesEachDifference(t *testing.T) {
	started := startedDeclarations()

	reordered := []plugin.PluginConfig{started[1], started[0]}
	assert.Empty(t, declarationChanges(started, reordered), "the same declarations in another order are a change")
	assert.Empty(t, declarationChanges(nil, nil))

	added := append(startedDeclarations(), plugin.PluginConfig{Name: "late-plugin", Run: "./late"})
	assert.Equal(t, []string{`it adds plugin "late-plugin"`}, declarationChanges(started, added))

	assert.Equal(t, []string{`it removes plugin "acme-monitor"`}, declarationChanges(started, started[:1]))

	otherRun := startedDeclarations()
	otherRun[1].Run = "./acme-monitor --verbose"
	assert.Equal(t, []string{`it changes plugin "acme-monitor" (run)`}, declarationChanges(started, otherRun))

	otherLeaves := startedDeclarations()
	otherLeaves[0].Encoder = "text"
	otherLeaves[0].Respawn = plugin.RespawnDeclined
	otherLeaves[0].StageTimeout = 9 * time.Second
	assert.Equal(t, []string{`it changes plugin "live-stages" (encoder, respawn, timeout)`},
		declarationChanges(started, otherLeaves))

	otherField := startedDeclarations()
	otherField[0].ReceiveUpdate = true
	assert.Equal(t, []string{`it changes plugin "live-stages" (another part of its declaration)`},
		declarationChanges(started, otherField))

	everything := []plugin.PluginConfig{otherRun[1], {Name: "late-plugin", Run: "./late"}}
	assert.Equal(t, []string{
		`it adds plugin "late-plugin"`,
		`it removes plugin "live-stages"`,
		`it changes plugin "acme-monitor" (run)`,
	}, declarationChanges(started, everything))
}

// TestRefuseChangedDeclarationsRefusesTheReload: the refusal a reload gets,
// and what doctor reads of it. Method: refuse a candidate that adds a plugin,
// then read the operator error and the report bus.
//
// VALIDATES: AC-12. The reload is refused with one operator error that names
// the plugin and the restart of ze on its first line, and the refusal is
// raised as doctor-config-reload-refused with the restart as its fix.
// PREVENTS: a refused reload the operator finds only in the daemon's stderr.
func TestRefuseChangedDeclarationsRefusesTheReload(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	started := startedDeclarations()
	added := append(startedDeclarations(), plugin.PluginConfig{Name: "late-plugin", Run: "./late"})

	err := RefuseChangedDeclarations(started, added)
	require.Error(t, err, "a reload that adds an external plugin was accepted")
	refusal, operator := errors.AsType[*opererr.Error](err)
	require.True(t, operator, "the refusal is no operator error: %v", err)
	assert.Equal(t, `the new config changes the external plugins ze runs, which change only when ze restarts: `+
		`it adds plugin "late-plugin"`, refusal.Problem())
	assert.Contains(t, refusal.Why(), "Every other part of the config changes without a restart")
	assert.Equal(t, "restart ze", refusal.Fix())

	var raised []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == CodeDoctorConfigReloadRefused {
			raised = append(raised, issue)
		}
	}
	require.Len(t, raised, 1, "the refusal raised no doctor-config-reload-refused event")
	assert.Equal(t, "restart ze", raised[0].Fix)
	assert.Contains(t, raised[0].Message, `it adds plugin "late-plugin"`)
}

// TestRefuseChangedDeclarationsAcceptsTheSamePlugins: a reload that leaves
// the plugin declarations as they were. Method: the started list against
// itself, and against an empty list on both sides.
//
// VALIDATES: AC-12's permitted survivor: equal declarations refuse nothing
// and raise nothing, whatever else the reload changes.
// PREVENTS: every reload of a daemon that runs an external plugin refused.
func TestRefuseChangedDeclarationsAcceptsTheSamePlugins(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	require.NoError(t, RefuseChangedDeclarations(startedDeclarations(), startedDeclarations()))
	require.NoError(t, RefuseChangedDeclarations(nil, nil))
	assert.Empty(t, report.Errors(0), "an accepted reload raised an event")
}
