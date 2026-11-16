// Design: docs/architecture/config/yang-config-design.md — publishing the modules external plugins declare
// Related: join.go — Join, which checks the modules a publish takes
// Related: loader.go — DefaultLoader, which answers the published set
// Related: command_argdefs.go — the argument definitions a publish rebuilds

package yang

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
)

// publishedModules is the external layer of the process's module set: the
// modules external plugins declared, as PublishExternalModules took them. Only
// PublishExternalModules builds one, after the join accepted every module, so
// holding one means the process's set is fixed. Its modules are never written
// after construction.
type publishedModules struct {
	// modules is the declared modules in the caller's order, which is the
	// order the join judged them in. Empty when the process declared none.
	modules []ExternalModule
	// validators is the registry the join judged ze:validate against. It is
	// kept for the one case that joins again: a test that changes the
	// registered modules after a publish (resolveProcessSet).
	validators *ValidatorRegistry
}

// PublishExternalModules makes the compiled-in module set joined to externals
// the set DefaultLoader answers for the rest of the process. externals is what
// Join takes: the module each external plugin declared, in config order.
//
// It calls Join. On a refusal it publishes nothing and returns Join's error
// unchanged: DefaultLoader keeps answering the compiled-in set, and a later
// call may publish. With no external module it joins and resolves nothing:
// DefaultLoader keeps answering the Resolved it answered before, and the
// process's set is recorded as holding no external module.
//
// The set is published at most once per process, because a plugin, and so its
// module, changes only when ze restarts. A second call with the same modules,
// in any order, changes nothing and returns nil. A second call with other
// modules is refused with an operator error that names the restart.
//
// The caller MUST publish before the process builds anything it keeps from
// the module set for its whole life. Every such value says so with
// KeepsModuleSet, and a publish that would change the set after one of them
// read it is refused, naming each: the alternative is a value that answers
// without the published modules, in silence. The command argument definitions
// (SeedCommandArgDefs) are not one of them: a publish rebuilds them.
//
// A reader that already holds a *Resolved keeps the set it took: a Resolved is
// immutable, and the publish swaps which one DefaultLoader answers. A reader
// that asks after the publish gets the joined set, and config.YANGSchema and
// config.YANGValidator, which are keyed on the Resolved, follow it.
//
// validators MUST be the registry the validator is built with
// (config.RegisterValidators). Safe for concurrent use: the swap happens under
// the lock DefaultLoader reads under, so a concurrent DefaultLoader answers
// the set before the publish or the set after it, never a mix.
func PublishExternalModules(externals []ExternalModule, validators *ValidatorRegistry) error {
	if validators == nil {
		panic("BUG: yang.PublishExternalModules was called with no validator registry; pass the registry the validator is built with")
	}
	joined, changed, err := swapModuleSet(slices.Clone(externals), validators)
	if err != nil {
		return err
	}
	if changed {
		// After swapModuleSet released the lock: commandArgDefs holds its own
		// lock while it asks DefaultLoader, so taking the two in the other
		// order could deadlock.
		rebuildCommandArgDefs(joined)
	}
	return nil
}

// swapModuleSet is the part of PublishExternalModules that runs under the
// lock DefaultLoader reads under. It answers the joined set and changed=true
// when it swapped the set DefaultLoader answers, and changed=false when that
// set is the one it was: a publish of no module, or a second publish of the
// same modules.
func swapModuleSet(declared []ExternalModule, validators *ValidatorRegistry) (joined *Resolved, changed bool, err error) {
	defaultSchema.mu.Lock()
	defer defaultSchema.mu.Unlock()
	if published := defaultSchema.external; published != nil {
		if sameModules(published.modules, declared) {
			return nil, false, nil
		}
		return nil, false, republishRefusal(published.modules, declared)
	}
	if len(declared) == 0 {
		defaultSchema.external = &publishedModules{validators: validators}
		return nil, false, nil
	}
	if keptErr := refuseKeptModuleSet(declared); keptErr != nil {
		return nil, false, keptErr
	}
	compiled, err := moduleSetLocked()
	if err != nil {
		return nil, false, err
	}
	joined, err = Join(compiled, declared, validators)
	if err != nil {
		return nil, false, err
	}
	defaultSchema.external = &publishedModules{modules: declared, validators: validators}
	// moduleSetLocked left the entry on the registry's generation, so the
	// joined set replaces the compiled-in one for that same generation.
	defaultSchema.entry.schema = joined
	return joined, true, nil
}

// PublishedExternalModules answers the external modules the process published
// and whether it published at all. published is false before the first
// successful PublishExternalModules; after a publish of no module it is true
// and the list is empty. The list is a copy. Safe for concurrent use.
func PublishedExternalModules() (modules []ExternalModule, published bool) {
	defaultSchema.mu.Lock()
	defer defaultSchema.mu.Unlock()
	if defaultSchema.external == nil {
		return nil, false
	}
	return slices.Clone(defaultSchema.external.modules), true
}

// ModulePublication is what the published set holds for one declaration: the
// answer PublishedModule gives a caller that compares a plugin's declaration
// with the set the process started with.
type ModulePublication uint8

const (
	// ModulePublicationUnspecified is the zero value and is never answered.
	ModulePublicationUnspecified ModulePublication = iota
	// ModuleSetUnpublished: the process has not published its external
	// modules, so no declaration can be compared yet.
	ModuleSetUnpublished
	// ModulePublished: the published set holds this declaration, with the
	// same plugin, module name, namespace and text.
	ModulePublished
	// ModuleChanged: the published set holds a module of this plugin, and it
	// differs from this declaration in its name, its namespace or its text.
	ModuleChanged
	// ModuleUndeclared: the published set holds no module of this plugin.
	ModuleUndeclared
)

// PublishedModule compares external, a plugin's declaration, with the set the
// process published. Safe for concurrent use.
func PublishedModule(external ExternalModule) ModulePublication {
	defaultSchema.mu.Lock()
	defer defaultSchema.mu.Unlock()
	if defaultSchema.external == nil {
		return ModuleSetUnpublished
	}
	modules := defaultSchema.external.modules
	if slices.Contains(modules, external) {
		return ModulePublished
	}
	ofPlugin := func(module ExternalModule) bool { return module.Plugin == external.Plugin }
	if slices.ContainsFunc(modules, ofPlugin) {
		return ModuleChanged
	}
	return ModuleUndeclared
}

// moduleSetKeepers names every value that was built once from the module set
// DefaultLoader answers and is kept for the life of the process
// (KeepsModuleSet). It is bounded by the number of call sites: each passes a
// constant name.
var moduleSetKeepers struct {
	mu    sync.Mutex
	names map[string]struct{}
}

// KeepsModuleSet records that keeper holds, for the rest of the process, a
// value it builds once from the module set DefaultLoader answers, or from
// anything built over that set (config.YANGSchema). keeper is a constant that
// names the value for a developer: it is printed in the refusal of a publish
// that comes too late.
//
// MUST be called before keeper reads the set, on every use and not only on the
// use that builds the value. A publish that had already looked at the keepers
// when the call was made holds the lock DefaultLoader reads under, so the read
// that follows the call waits for it and gets the published set. A publish
// that starts after the call finds keeper recorded and is refused
// (PublishExternalModules). Such a value is correct because the process
// publishes before it builds it; this call is what turns a wrong order into a
// refusal. Safe for concurrent use.
func KeepsModuleSet(keeper string) {
	if keeper == "" {
		panic("BUG: yang.KeepsModuleSet was called with no name for the value that keeps the module set")
	}
	moduleSetKeepers.mu.Lock()
	defer moduleSetKeepers.mu.Unlock()
	if moduleSetKeepers.names == nil {
		moduleSetKeepers.names = map[string]struct{}{}
	}
	moduleSetKeepers.names[keeper] = struct{}{}
}

// moduleSetKeeperNames answers the recorded keepers, sorted.
func moduleSetKeeperNames() []string {
	moduleSetKeepers.mu.Lock()
	defer moduleSetKeepers.mu.Unlock()
	names := make([]string, 0, len(moduleSetKeepers.names))
	for name := range moduleSetKeepers.names {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// refuseKeptModuleSet refuses a publish that would change the module set
// after a once-built value read it. The cause is the order ze built things in,
// never the operator's config, so the refusal says so.
func refuseKeptModuleSet(declared []ExternalModule) error {
	keepers := moduleSetKeeperNames()
	if len(keepers) == 0 {
		return nil
	}
	return opererr.New(
		"Ze cannot join the YANG modules that external plugins "+pluginList(declared)+
			" declare: it already built "+strings.Join(keepers, ", ")+" without them",
		"each of those is built once for the life of the process and would keep answering without the plugins' modules; "+
			"this is a Ze defect, report it, and until it is corrected ze starts only without those plugins",
		"delete plugin external "+declared[0].Plugin,
	)
}

// republishRefusal is the refusal of a second publish whose modules differ
// from the published ones.
func republishRefusal(published, declared []ExternalModule) error {
	problem := "The YANG modules that external plugins declare differ from the ones ze joined when it started"
	if changed := changedPlugins(published, declared); len(changed) > 0 {
		problem += ": the declaration of plugin " + strings.Join(changed, ", ") + " is not the one ze started with"
	}
	return opererr.New(
		problem,
		"ze joins the modules of external plugins once, when it starts, and every reader of the config schema holds that set",
		"restart ze",
	)
}

// changedPlugins answers the quoted name of every plugin whose declaration is
// in one of the two lists and not in the other, sorted, once each. Both lists
// are bounded by the number of plugins the config declares.
func changedPlugins(published, declared []ExternalModule) []string {
	var names []string
	for _, module := range declared {
		if !slices.Contains(published, module) {
			names = append(names, quoted(module.Plugin))
		}
	}
	for _, module := range published {
		if !slices.Contains(declared, module) {
			names = append(names, quoted(module.Plugin))
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// pluginList answers the quoted names of the plugins that declared modules, in
// the order given.
func pluginList(modules []ExternalModule) string {
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, quoted(module.Plugin))
	}
	return strings.Join(names, ", ")
}

// sameModules reports whether two lists hold the same declarations, in any
// order: the config order decides which plugin a clash is reported against,
// never which set was published.
func sameModules(a, b []ExternalModule) bool {
	return slices.Equal(sortedModules(a), sortedModules(b))
}

// sortedModules answers a sorted copy of modules.
func sortedModules(modules []ExternalModule) []ExternalModule {
	sorted := slices.Clone(modules)
	slices.SortFunc(sorted, func(a, b ExternalModule) int {
		return cmp.Or(
			cmp.Compare(a.Plugin, b.Plugin),
			cmp.Compare(a.Module, b.Module),
			cmp.Compare(a.Namespace, b.Namespace),
			cmp.Compare(a.Text, b.Text),
		)
	})
	return sorted
}

// publishedModuleCountLocked answers how many external modules the process
// published: none before a publish and after a publish of no module. The
// caller MUST hold defaultSchema.mu.
func publishedModuleCountLocked() int {
	if defaultSchema.external == nil {
		return 0
	}
	return len(defaultSchema.external.modules)
}

// ResetPublishedModulesForTest puts the process back to before any publish:
// no external layer, DefaultLoader answering the compiled-in set, and no
// recorded keeper. It returns the function that restores what it found,
// the command argument definitions included, since a publish rebuilds them.
// A test that publishes calls it first.
// MUST be paired with the returned restore, typically through t.Cleanup. Not
// safe for concurrent use: a test that calls it MUST NOT run in parallel with
// another test that loads YANG.
func ResetPublishedModulesForTest() (restore func()) {
	defaultSchema.mu.Lock()
	savedEntry, savedExternal := defaultSchema.entry, defaultSchema.external
	if publishedModuleCountLocked() > 0 {
		// The entry holds a joined set: the compiled-in one is resolved again.
		defaultSchema.entry = moduleSetEntry{}
	}
	defaultSchema.external = nil
	defaultSchema.mu.Unlock()

	moduleSetKeepers.mu.Lock()
	savedKeepers := moduleSetKeepers.names
	moduleSetKeepers.names = nil
	moduleSetKeepers.mu.Unlock()

	savedArgDefs := heldCommandArgDefs()

	return func() {
		defaultSchema.mu.Lock()
		defaultSchema.entry, defaultSchema.external = savedEntry, savedExternal
		defaultSchema.mu.Unlock()

		moduleSetKeepers.mu.Lock()
		moduleSetKeepers.names = savedKeepers
		moduleSetKeepers.mu.Unlock()

		SeedCommandArgDefs(savedArgDefs)
	}
}

// heldCommandArgDefs answers the argument definitions commandArgDefs holds,
// nil when none were built or seeded.
func heldCommandArgDefs() map[string][]command.ArgDef {
	modelArgDefsMu.Lock()
	defer modelArgDefsMu.Unlock()
	return modelArgDefs
}

// rebuildCommandArgDefs replaces the argument definitions every dispatch
// route reads with the ones schema declares, when they were already built or
// seeded: they were built from the set before the publish. When none were
// built, nothing is done, and the first use builds them from the set
// DefaultLoader answers then, which is the published one.
func rebuildCommandArgDefs(schema *Resolved) {
	modelArgDefsMu.Lock()
	defer modelArgDefsMu.Unlock()
	if modelArgDefs == nil {
		return
	}
	modelArgDefs = PathToArgDefs(schema)
}
