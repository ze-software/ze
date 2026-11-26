// Design: docs/architecture/hub-architecture.md -- an external plugin's declaration changes only when ze restarts
// Related: reload.go -- the reload this refusal precedes
// Related: server_events.go -- raiseReloadRefused, the event the refusal raises
// Related: stage1_schema.go -- the same rule for the module a restarted plugin declares
// Related: ../../config/first_pass.go -- FirstPass, which reads both lists this file compares

package server

import (
	"slices"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// restartDaemonFix is the fix of a refusal whose cause is a plugin or a
// plugin's config schema that differs from what ze started with: ze reads
// both once, when it starts.
const restartDaemonFix = "restart ze"

// declarationsFixedWhy says why a reload cannot change the external plugins.
const declarationsFixedWhy = "ze starts the external plugins that the config and the command line declare once, when it starts, " +
	"and reads the config schema of each one then. A reload starts none, stops none and reads no schema again, " +
	"so ze refused the whole reload and keeps the config it was running. Every other part of the config changes without a restart"

// RefuseChangedDeclarations refuses a reload whose config declares other
// external plugins than the ones ze started with. started is the External
// list of the first pass (config.FirstPass) the daemon booted from, and
// candidate the External list of the first pass over the config the reload
// read, both with the daemon's command-line plugins.
//
// It answers nil when the two lists hold the same declarations, in any order.
// Otherwise it answers one operator error that names every plugin the config
// adds, removes or changes, and the restart of ze. It logs that refusal and
// raises it as doctor-config-reload-refused, so the caller only returns it.
//
// The caller MUST call it before the full parse of the reload's config: a
// config that adds a plugin and a section of that plugin is refused here with
// the restart, where the full parse would refuse the section as a name no
// schema defines. Safe for concurrent use.
func RefuseChangedDeclarations(started, candidate []plugin.PluginConfig) error {
	refusal := declarationChangeRefusal(started, candidate)
	if refusal == nil {
		return nil
	}
	logger().Warn("config reload: the external plugin declarations changed", "error", refusal,
		"fix", restartDaemonFix, "code", CodeDoctorConfigReloadRefused)
	raiseReloadRefused(reloadRefusalMessage(
		"the new configuration changes the external plugins ze runs, so ze keeps the configuration it was running",
		nil, refusal), refusal)
	return refusal
}

// declarationChangeRefusal is the operator error of RefuseChangedDeclarations,
// with no log line and no event: nil when candidate declares what started
// declares.
func declarationChangeRefusal(started, candidate []plugin.PluginConfig) error {
	changes := declarationChanges(started, candidate)
	if len(changes) == 0 {
		return nil
	}
	var problem textbuf.Buffer
	problem.Str("the new config changes the external plugins ze runs, which change only when ze restarts: ").
		Join(changes, "; ")
	return opererr.New(problem.String(), declarationsFixedWhy, restartDaemonFix)
}

// declarationChanges says how candidate differs from started, one phrase for
// each plugin: the added plugins by name, then the removed ones, then the
// changed ones with the leaves that differ. A plugin is matched by its name.
// Both lists are bounded by the number of plugins a config declares.
func declarationChanges(started, candidate []plugin.PluginConfig) []string {
	before := sortedDeclarations(started)
	after := sortedDeclarations(candidate)

	var changes []string
	for _, declaration := range after {
		if _, held := declarationNamed(before, declaration.Name); held {
			continue
		}
		changes = append(changes, declarationPhrase("it adds plugin ", declaration.Name, nil))
	}
	for _, declaration := range before {
		if _, held := declarationNamed(after, declaration.Name); held {
			continue
		}
		changes = append(changes, declarationPhrase("it removes plugin ", declaration.Name, nil))
	}
	for _, declaration := range after {
		previous, held := declarationNamed(before, declaration.Name)
		if !held {
			continue
		}
		if externalDeclarationsEqual(previous, declaration) {
			continue
		}
		changes = append(changes, declarationPhrase("it changes plugin ", declaration.Name,
			changedLeaves(previous, declaration)))
	}
	return changes
}

// sortedDeclarations answers a copy of declarations sorted by name, so the
// refusal names the plugins in one order whatever order the config lists
// them in.
func sortedDeclarations(declarations []plugin.PluginConfig) []plugin.PluginConfig {
	sorted := slices.Clone(declarations)
	slices.SortFunc(sorted, func(a, b plugin.PluginConfig) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return sorted
}

// declarationNamed answers the declaration of the plugin named name.
func declarationNamed(declarations []plugin.PluginConfig, name string) (plugin.PluginConfig, bool) {
	for _, declaration := range declarations {
		if declaration.Name == name {
			return declaration, true
		}
	}
	return plugin.PluginConfig{}, false
}

// externalDeclarationsEqual compares only the external process declaration
// fields (run, encoding, respawn, timeout, on-exit, delivery-queue-max). Bus
// send/receive filters live outside these fields so filter reloads proceed live
// without restarting ze.
func externalDeclarationsEqual(before, after plugin.PluginConfig) bool {
	if before.Run != after.Run {
		return false
	}
	if canonicalEncoding(before.Encoder) != canonicalEncoding(after.Encoder) {
		return false
	}
	if before.Respawn != after.Respawn {
		return false
	}
	if before.StageTimeout != after.StageTimeout {
		return false
	}
	if canonicalOnExit(before.OnExit) != canonicalOnExit(after.OnExit) {
		return false
	}
	return before.DeliveryQueueMax == after.DeliveryQueueMax
}

func canonicalEncoding(enc string) string {
	if enc == "" {
		return plugin.EncodingJSON
	}
	return enc
}

func canonicalOnExit(onExit string) string {
	if onExit == "" {
		return plugin.OnExitWithdraw
	}
	return onExit
}

// changedLeaves names the endpoint declaration leaves in which after differs
// from before. A difference in a field no leaf below names is still a changed
// declaration, and is said as such: the list is never empty for two
// declarations that differ.
func changedLeaves(before, after plugin.PluginConfig) []string {
	var leaves []string
	if before.Run != after.Run {
		leaves = append(leaves, "run")
	}
	if canonicalEncoding(before.Encoder) != canonicalEncoding(after.Encoder) {
		leaves = append(leaves, "encoding")
	}
	if before.Respawn != after.Respawn {
		leaves = append(leaves, "respawn")
	}
	if before.StageTimeout != after.StageTimeout {
		leaves = append(leaves, "timeout")
	}
	if canonicalOnExit(before.OnExit) != canonicalOnExit(after.OnExit) {
		leaves = append(leaves, "on-exit")
	}
	if before.DeliveryQueueMax != after.DeliveryQueueMax {
		leaves = append(leaves, "delivery-queue-max")
	}
	if len(leaves) == 0 {
		leaves = append(leaves, "another part of its declaration")
	}
	return leaves
}

// declarationPhrase writes one change: lead, the quoted plugin name, and the
// changed leaves in brackets when there are any.
func declarationPhrase(lead, name string, leaves []string) string {
	var phrase textbuf.Buffer
	phrase.Str(lead).Quoted(name)
	if len(leaves) > 0 {
		phrase.Str(" (").Join(leaves, ", ").Byte(')')
	}
	return phrase.String()
}
