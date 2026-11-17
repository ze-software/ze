// Design: docs/architecture/config/yang-config-design.md -- config claim model
// Related: startup.go -- onRegistration records the roots of an accepted external declaration
// Related: register.go -- the process cleanup hook that forgets them
// Related: ../../doctor/checks_config_claims.go -- the doctor check that counts them as claims

package server

import (
	"slices"
	"sync"

	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// externalConfigRoots holds, for each external plugin this process runs, the
// config roots its accepted Stage 1 declaration wants. A plugin compiled into
// ze declares its roots in the registry, which every process can read; an
// external plugin's roots exist only in the declaration its program sent, so
// they are kept here for the reader that asks which config reaches a consumer.
//
// It holds one entry for each configured external plugin, whose name the
// config or the command line chose, and each entry is bounded by the size of
// one declaration (rpc.MaxMessageSize).
var externalConfigRoots struct {
	mu       sync.Mutex
	byPlugin map[string][]string
}

// recordExternalConfigRoots keeps roots as the config roots the external
// plugin cfg wants, replacing what an earlier start of it declared. A plugin
// compiled into ze is not recorded: the registry holds its roots.
// forgetExternalConfigRoots MUST run when the plugin's process ends.
func recordExternalConfigRoots(cfg plugin.PluginConfig, roots []string) {
	if !cfg.RunsExternalProgram() {
		return
	}
	externalConfigRoots.mu.Lock()
	defer externalConfigRoots.mu.Unlock()
	if len(roots) == 0 {
		delete(externalConfigRoots.byPlugin, cfg.Name)
		return
	}
	if externalConfigRoots.byPlugin == nil {
		externalConfigRoots.byPlugin = make(map[string][]string)
	}
	externalConfigRoots.byPlugin[cfg.Name] = slices.Clone(roots)
}

// forgetExternalConfigRoots drops the roots recorded for the plugin named
// name. It is the process cleanup hook paired with recordExternalConfigRoots:
// a plugin that no longer runs receives no config, so its roots stop counting
// as delivered. Safe for concurrent use.
func forgetExternalConfigRoots(name string) {
	externalConfigRoots.mu.Lock()
	defer externalConfigRoots.mu.Unlock()
	delete(externalConfigRoots.byPlugin, name)
}

// SetExternalConfigRootsForTest makes roots the recorded roots of the process,
// by plugin name, and returns the function that restores what it found. A
// test of a reader of ExternalConfigRoots calls it in place of running a
// plugin. MUST be paired with the returned restore, typically through
// t.Cleanup. Safe for concurrent use, but a test that sets the roots MUST NOT
// run in parallel with another test that reads them.
func SetExternalConfigRootsForTest(roots map[string][]string) (restore func()) {
	externalConfigRoots.mu.Lock()
	defer externalConfigRoots.mu.Unlock()
	saved := externalConfigRoots.byPlugin
	externalConfigRoots.byPlugin = make(map[string][]string, len(roots))
	for name, wanted := range roots {
		externalConfigRoots.byPlugin[name] = slices.Clone(wanted)
	}
	return func() {
		externalConfigRoots.mu.Lock()
		defer externalConfigRoots.mu.Unlock()
		externalConfigRoots.byPlugin = saved
	}
}

// ExternalConfigRoots answers, by plugin name, the config roots each running
// external plugin declared it wants, in the shape of
// registry.ConfigSectionRootsMap, which answers the same for the plugins
// compiled into ze. It is empty in a process that runs no external plugin, as
// a command with no daemon is. Safe for concurrent use.
func ExternalConfigRoots() map[string][]string {
	externalConfigRoots.mu.Lock()
	defer externalConfigRoots.mu.Unlock()
	roots := make(map[string][]string, len(externalConfigRoots.byPlugin))
	for name, wanted := range externalConfigRoots.byPlugin {
		roots[name] = slices.Clone(wanted)
	}
	return roots
}
