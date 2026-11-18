// Design: docs/architecture/config/yang-config-design.md — joining an external plugin's module
// Related: loader.go — Loader, Resolved and the checks Resolve runs
// Related: validator_registry.go — the ze:validate registry the join reads
// Related: reference_source.go — the ze:reference sources the join reads

package yang

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/opererr"
)

// ExternalModule is one YANG module an external plugin declares: one entry of
// the `schemas` list of its Stage 1 declaration, with the name of the plugin
// that sent it. A plugin that brings several modules is several of them.
// It is the raw declaration, and nothing in it is checked until Join reads it.
type ExternalModule struct {
	// Plugin is the name of the plugin that declared the module.
	Plugin string
	// Module is the module name the declaration states beside the text.
	Module string
	// Namespace is the namespace the declaration states beside the text.
	Namespace string
	// Text is the YANG text of the module.
	Text string
}

// declaredModule is an external module whose text parsed alone and whose
// declaration agrees with it. Only declaredModules builds one.
type declaredModule struct {
	external  ExternalModule
	name      string
	namespace string
	prefix    string
}

// Join answers the module set that holds every module of compiled and every
// external module, resolved and checked, or an operator error that names the
// plugin and the module it refuses. One Loader.Resolve judges the joined set:
// goyang's processing and the leafref check over every module, the extension,
// pattern and structure checks over the external modules, because every module
// of compiled passed them already (Resolved.reloaded). The join adds the
// refusals a plugin's module owes beyond them:
//
//   - the text does not parse or resolve, or holds anything but one module
//   - the declared module name or namespace differs from the text
//   - the module name is held by a module of compiled, at any revision, or by
//     an earlier external module
//   - the namespace is held the same way
//   - the module declares definitions no reader of ze takes: its name does not
//     end in -conf, it augments nothing, it is not a -cmd module that writes a
//     ze:command or a ze:help into the command tree, it holds no rpc, and no
//     other module of the set imports it (refuseIdleModules, the rule the
//     compiled-in modules are held to as well). A module that declares no
//     statement at all loses nothing and is joined, as the compiled-in set
//     holds such modules (EmptyModuleNames)
//   - a command path or wire method another module holds, or a ze:rpc or wire
//     method PublishedRPCs refuses (refuseCommandConflicts)
//   - a top-level node of a -conf external module is already defined by a
//     -conf module of compiled or by an earlier external module
//   - the module names a ze:validate function validators does not hold, or a
//     ze:reference source no component registered
//   - the module adds a node under a root ze reads before any plugin runs
//     (FirstPassRoots), where no config could ever use it
//
// The join does not judge ze:decorate and ze:backend: their registries live
// in packages that import this one, so a name ze does not hold is accepted
// here. A decorator name the web registry does not hold decorates nothing
// (web.DecoratorRegistry.Get answers none for it), and a backend name is read
// when a config uses the node (config backend_gate.go).
//
// The redefinition refusal judges external modules only: the modules of
// compiled may share a top-level container by a second definition, as six
// shipped modules share `service`.
//
// With no external module Join answers compiled itself and resolves nothing.
// A caller MAY rely on that identity: a start that declares no external plugin
// keeps the Resolved, and every cache keyed on it, that DefaultLoader answered.
//
// compiled is not changed: the joined set is resolved by a new Loader over the
// module texts compiled holds. validators MUST be the registry the validator is
// built with (config.RegisterValidators). Safe for concurrent use.
func Join(compiled *Resolved, externals []ExternalModule, validators *ValidatorRegistry) (*Resolved, error) {
	set := compiled.set()
	if validators == nil {
		panic("BUG: yang.Join was called with no validator registry; pass the registry the validator is built with")
	}
	if len(externals) == 0 {
		return compiled, nil
	}
	declared, err := declaredModules(set, externals)
	if err != nil {
		return nil, err
	}
	joined, err := resolveJoined(compiled, declared)
	if err != nil {
		return nil, err
	}
	if err := refuseIdleModules(joined, declared); err != nil {
		return nil, err
	}
	if err := refuseRedefinedRoots(compiled, joined, declared); err != nil {
		return nil, err
	}
	if err := refuseUnheldRegistrations(joined, declared, validators); err != nil {
		return nil, err
	}
	if err := refuseCommandConflicts(compiled, joined, declared); err != nil {
		return nil, err
	}
	return joined, nil
}

// ExternalModuleFromText answers the declaration of a module text that has no
// declaration beside it: the module name and the namespace are the ones the
// text states. It is for a Go caller that holds a text and no plugin's Stage 1
// declaration; origin takes the place of the plugin name in a refusal. A text
// that does not parse alone is refused here, as Join refuses it.
func ExternalModuleFromText(origin, text string) (ExternalModule, error) {
	external := ExternalModule{Plugin: origin, Text: text}
	module, err := parseExternal(external)
	if err != nil {
		return ExternalModule{}, err
	}
	external.Module = module.Name
	external.Namespace = module.Namespace.Name
	return external, nil
}

// DefinesConfig reports whether external, an external module, adds config to
// the schema: its name ends in -conf, so the config schema takes its top-level
// nodes (Resolved.ConfModuleNames), or it is neither a -cmd nor an -api module
// and carries an augment, which reaches a config node. A -cmd module and an
// -api module add commands and rpcs, never config. The name is the declared
// one, which Join refuses when it differs from the text, so only a module of
// another name is parsed. The error is a text that does not parse alone, as
// Join refuses it.
func DefinesConfig(external ExternalModule) (bool, error) {
	if strings.HasSuffix(external.Module, "-conf") {
		return true, nil
	}
	if strings.HasSuffix(external.Module, cmdModuleSuffix) {
		return false, nil
	}
	if strings.HasSuffix(external.Module, "-api") {
		return false, nil
	}
	module, err := parseExternal(external)
	if err != nil {
		return false, err
	}
	return len(module.Augment) > 0, nil
}

// declaredModules parses each external module alone and refuses the ones the
// module set cannot take before any of them is added to it: a declaration that
// differs from its text, a module name already held, a namespace already held.
func declaredModules(set *sourcedModules, externals []ExternalModule) ([]declaredModule, error) {
	namespaces := set.namespaceHolders()
	declared := make([]declaredModule, 0, len(externals))
	for _, external := range externals {
		module, err := parseExternal(external)
		if err != nil {
			return nil, err
		}
		next := declaredModule{
			external:  external,
			name:      module.Name,
			namespace: module.Namespace.Name,
			prefix:    modulePrefix(module),
		}
		if err := refuseDeclarationMismatch(next); err != nil {
			return nil, err
		}
		if err := refuseNameClash(set, declared, next); err != nil {
			return nil, err
		}
		if err := refuseNamespaceClash(namespaces, declared, next); err != nil {
			return nil, err
		}
		declared = append(declared, next)
	}
	return declared, nil
}

// parseExternal parses one external module text alone, with no other module
// loaded, and answers its one module. Imports are not resolved here: that is
// Resolve's work over the joined set. The module it answers always states a
// namespace, so a caller reads module.Namespace.Name with no nil check.
func parseExternal(external ExternalModule) (*gyang.Module, error) {
	scratch := gyang.NewModules()
	if err := scratch.Parse(external.Text, externalFile(external)); err != nil {
		return nil, invalidModuleRefusal(external, err)
	}
	// goyang stores a module that declares a revision under two keys, so the
	// count is of modules and not of keys.
	var parsed []*gyang.Module
	for _, module := range scratch.Modules {
		if !slices.Contains(parsed, module) {
			parsed = append(parsed, module)
		}
	}
	if len(scratch.SubModules) > 0 {
		return nil, moduleCountRefusal(external)
	}
	if len(parsed) != 1 {
		return nil, moduleCountRefusal(external)
	}
	module := parsed[0]
	if module.Namespace == nil {
		return nil, missingNamespaceRefusal(external, module.Name)
	}
	if module.Namespace.Name == "" {
		return nil, missingNamespaceRefusal(external, module.Name)
	}
	return module, nil
}

// externalFile is the file name an external module is parsed under, which the
// loader's error locations then carry.
func externalFile(external ExternalModule) string {
	return external.Plugin + "/" + external.Module + ".yang"
}

// modulePrefix answers the prefix a module declares for itself, or its name
// when it declares none, which Resolve then refuses.
func modulePrefix(module *gyang.Module) string {
	if module.Prefix == nil {
		return module.Name
	}
	return module.Prefix.Name
}

// HoldsModuleText reports whether the set holds a module named name that was
// parsed from exactly text, byte for byte: the module a caller holds the text
// of is then the one already in the set, embedded or registered. A held name
// with another text, and a name the set does not hold, both answer false.
func (r *Resolved) HoldsModuleText(name, text string) bool {
	set := r.set()
	module := set.Modules.Modules[name]
	if module == nil {
		return false
	}
	source := set.source(module)
	if !source.parsed {
		return false
	}
	return source.text == text
}

// holdsName reports whether the set holds a module or a submodule named name,
// at any revision: goyang keys the most recent revision under the bare name.
func (m *sourcedModules) holdsName(name string) bool {
	if m.Modules.Modules[name] != nil {
		return true
	}
	return m.Modules.SubModules[name] != nil
}

// namespaceHolders answers, for each namespace a module of the set declares,
// the name of the module that holds it. When two modules share a namespace the
// first by name is the one named.
func (m *sourcedModules) namespaceHolders() map[string]string {
	names := m.moduleNames()
	slices.Sort(names)
	holders := make(map[string]string, len(names))
	for _, name := range names {
		module := m.Modules.Modules[name]
		if module.Namespace == nil {
			continue
		}
		if _, held := holders[module.Namespace.Name]; held {
			continue
		}
		holders[module.Namespace.Name] = name
	}
	return holders
}

// reloaded answers a new Loader holding every module text of r, so a module
// can be added beside them and the whole resolved again: the set itself
// refuses every load once Resolve ran. Every module it reloads is marked
// judged with r's patterns, because each one passed checkExtensions,
// checkPatterns and checkStructure when r was resolved: only a successful
// Resolve builds a Resolved, and a joined one holds compiled-in modules that
// passed them before and external modules its join judged. The Resolve of the
// new loader then judges the modules added after this, the external ones.
//
// A judged module's verdict cannot change in the joined set. Each of the
// three checks reads one module's statements, its text, and the modules its
// prefix, imports, includes and belongs-to name, and goyang resolves a type
// of the module from the same (types.go, Type.resolve, through RootNode and
// the module's imports). declaredModules refuses an external module whose
// name any module of r holds, at any revision, and parseExternal one that
// holds a submodule, so none of those names reaches another module. goyang
// resolves every type a module declares while it processes that module,
// groupings included (entry.go, ToEntry: "We just want to parse the grouping
// to collect errors"), so an external module that uses a grouping resolves no
// type that was left unresolved. What does depend on the other modules, the
// leafref paths, augments and every refusal the join adds, is judged over
// the whole joined set.
func (r *Resolved) reloaded() (*Loader, error) {
	m := r.set()
	held := m.held()
	modules := make([]*gyang.Module, 0, len(held))
	for module := range held {
		modules = append(modules, module)
	}
	slices.SortFunc(modules, func(a, b *gyang.Module) int {
		return cmp.Compare(a.FullName(), b.FullName())
	})
	loader := NewLoader()
	loaded := make(map[string]bool, len(modules))
	for _, module := range modules {
		source := m.source(module)
		if !source.parsed {
			panic("BUG: resolved YANG module " + module.Name + " has no recorded text; every module is loaded through Loader, which records it")
		}
		if loaded[source.text] {
			continue
		}
		loaded[source.text] = true
		if err := loader.AddModuleFromText(module.Name+".yang", source.text); err != nil {
			return nil, fmt.Errorf("reload YANG module %s: %w", module.Name, err)
		}
	}
	loader.modules.markJudged(r.patterns)
	return loader, nil
}

// resolveJoined resolves the modules of compiled with the declared modules
// added, in one resolve, and names the module a failure belongs to
// (failingModule). compiled resolved alone, so a failure comes from an
// external module.
func resolveJoined(compiled *Resolved, declared []declaredModule) (*Resolved, error) {
	loader, err := compiled.reloaded()
	if err != nil {
		return nil, err
	}
	for _, module := range declared {
		if err := loader.AddModuleFromText(externalFile(module.external), module.external.Text); err != nil {
			return nil, invalidModuleRefusal(module.external, err)
		}
	}
	joined, resolveErr := loader.Resolve()
	if resolveErr == nil {
		return joined, nil
	}
	if len(declared) == 1 {
		return nil, invalidModuleRefusal(declared[0].external, resolveErr)
	}
	if failing, found := failingModule(loader, declared, resolveErr); found {
		return nil, invalidModuleRefusal(failing.external, resolveErr)
	}
	// No error of the one resolve belongs to a declared module: goyang wrote
	// it with no location (Modules.include: "no such module: <name>";
	// Entry.ApplyDeviate: "cannot find target node to deviate"), or at a
	// location in a compiled-in file. Only then is each module resolved alone
	// with the set, the first that fails is named, and when none fails the
	// modules are refused together. The recursion is one level deep, because a
	// call with one module never reaches here.
	for _, module := range declared {
		if _, aloneErr := resolveJoined(compiled, []declaredModule{module}); aloneErr != nil {
			return nil, aloneErr
		}
	}
	return nil, invalidTogetherRefusal(declared, resolveErr)
}

// failingModule answers the first declared module, in the order declared, that
// an error of the joined resolve belongs to, and whether there is one, reading
// where each producer records the module at fault and never the text an error
// quotes:
//
//   - a goyang error belongs to the module whose file it begins with: goyang
//     writes the location of the node it was building first (entry.go
//     newError and errorf; Entry.merge begins with the augment or uses that
//     adds a node already held, then quotes the node it met). Each declared
//     module is parsed under a file name of its own (externalFile);
//   - a leafref error belongs to the module whose statement wrote the leaf
//     (leafrefError, yang.RootNode), which for a leaf augmented into a
//     compiled-in container is the augmenting module;
//   - an extension, pattern or structure error belongs to the module those
//     checks refuse when they judge it alone over the processed set
//     (moduleCheckErrors), since they judged the declared modules only.
func failingModule(loader *Loader, declared []declaredModule, resolveErr error) (declaredModule, bool) {
	goyangErrs, leafOwners := resolveErrorOrigins(resolveErr)
	for _, module := range declared {
		if leafOwners[module.name] {
			return module, true
		}
		file := externalFile(module.external) + ":"
		for _, err := range goyangErrs {
			if strings.HasPrefix(err.Error(), file) {
				return module, true
			}
		}
		parsed := loader.modules.Modules.Modules[module.name]
		if parsed == nil {
			panic("BUG: yang.failingModule: declared module " + module.name + " is not in the loader that parsed it")
		}
		if loader.moduleCheckErrors(parsed) != nil {
			return module, true
		}
	}
	return declaredModule{}, false
}

// resolveErrorOrigins walks the error tree of a resolve and answers goyang's
// errors (processError) and the name of each module that wrote a leaf whose
// leafref was refused (leafrefError). The walk is an explicit stack bounded by
// the errors one resolve joins.
func resolveErrorOrigins(resolveErr error) (goyangErrs []error, leafOwners map[string]bool) {
	leafOwners = map[string]bool{}
	pending := []error{resolveErr}
	for len(pending) > 0 {
		err := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		// The walk judges each node of the tree itself, so a type assertion and
		// not errors.As, which would answer the first match of the whole tree.
		if goyang, isGoyang := err.(*processError); isGoyang { //nolint:errorlint // one node of the walk, see above
			goyangErrs = append(goyangErrs, goyang.errs...)
			continue
		}
		if leafref, isLeafref := err.(*leafrefError); isLeafref { //nolint:errorlint // one node of the walk, see above
			if leafref.leaf.Node != nil {
				leafOwners[gyang.RootNode(leafref.leaf.Node).Name] = true
			}
			continue
		}
		if joined, isJoined := err.(interface{ Unwrap() []error }); isJoined {
			pending = append(pending, joined.Unwrap()...)
		}
	}
	return goyangErrs, leafOwners
}

// moduleUse is what one module of a set gives ze, by the one rule every
// module obeys, compiled-in or external. The zero value is uninitialized:
// moduleUseOf never answers it, and every reader compares with a named value,
// so a zero matches no use and refuses or names no module.
type moduleUse uint8

const (
	// The zero value is no use, so an unclassified module is never taken for
	// one that adds, is empty or is idle.
	_ moduleUse = iota
	// moduleUseAdds: a reader of ze takes something the module declares.
	moduleUseAdds
	// moduleUseEmpty: the module declares no statement at all, only its
	// header, imports and revisions, so it brings ze nothing but a name and a
	// namespace.
	moduleUseEmpty
	// moduleUseIdle: the module declares definitions no reader of ze takes,
	// so ze would drop them in silence.
	moduleUseIdle
)

// refuseIdleModules refuses an external module that adds nothing to ze: one
// whose definitions ze would drop in silence (moduleUseIdle), and one that
// declares no statement at all (moduleUseEmpty). The config schema takes
// top-level nodes only from a config module (Resolved.ConfModuleNames), any
// other module reaches it by an augment, the command tree takes nodes only
// from a -cmd module (BuildCommandTree), which reads a ze:command and, on a
// navigation node, a ze:help summary, and the rpcs that describe command
// parameters are read from every module (PublishedRPCs). A module another
// module of the set imports adds what its importers take from it (a typedef,
// a grouping, an extension), as the compiled-in ze-types and ze-ppp-lcp do.
//
// It is the rule every module obeys: IdleModuleNames and EmptyModuleNames
// apply it to a whole set, and the test of the shipped modules holds the
// compiled-in set to it (TestNoShippedModuleAddsNothing), so neither set
// carries an idle or an empty module. Only the time differs, which is the
// transport: a compiled-in module is judged when ze is built, an external one
// when ze joins it at start.
func refuseIdleModules(joined *Resolved, declared []declaredModule) error {
	imported := importedModules(joined)
	for _, module := range declared {
		use, commands := moduleUseOf(joined, module.name, imported)
		switch use {
		case moduleUseAdds:
		case moduleUseEmpty:
			return emptyModuleRefusal(module)
		case moduleUseIdle:
			return idleModuleRefusal(module, commands)
		default:
			panic("BUG: yang.moduleUseOf answered no use for module " + module.name + "; it answers a named moduleUse for every module of the set")
		}
	}
	return nil
}

// IdleModuleNames answers, sorted, the modules of schema whose definitions no
// reader of ze takes, by the rule refuseIdleModules refuses an external module
// by: not a config module, no augment, no ze:command or ze:help the command
// tree reads under a -cmd name, no rpc, imported by no other module of schema,
// and at least one statement declared.
func IdleModuleNames(schema *Resolved) []string {
	return moduleNamesOfUse(schema, moduleUseIdle)
}

// EmptyModuleNames answers, sorted, the modules of schema that declare no
// statement at all: a header, imports and revisions only, and imported by no
// other module of schema. The join refuses one from an external plugin
// (refuseIdleModules), and the compiled-in set is held to ship none
// (TestNoShippedModuleAddsNothing).
func EmptyModuleNames(schema *Resolved) []string {
	return moduleNamesOfUse(schema, moduleUseEmpty)
}

// moduleNamesOfUse answers, sorted, the modules of schema whose use is want.
func moduleNamesOfUse(schema *Resolved, want moduleUse) []string {
	imported := importedModules(schema)
	var names []string
	for _, name := range schema.ModuleNames() {
		if use, _ := moduleUseOf(schema, name, imported); use == want {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// moduleUseOf answers what the module name of schema gives ze, and whether it
// declares a ze:command the tree would read under a -cmd name. imported holds
// the modules another module of schema imports.
func moduleUseOf(schema *Resolved, name string, imported map[string]bool) (use moduleUse, commands bool) {
	if slices.Contains(schema.ConfModuleNames(), name) {
		return moduleUseAdds, false
	}
	if imported[name] {
		return moduleUseAdds, false
	}
	module := schema.GetModule(name)
	if module == nil {
		panic("BUG: yang.moduleUseOf was asked about module " + name + ", which the set does not hold; callers pass a name of the set")
	}
	if len(module.Augment) > 0 {
		return moduleUseAdds, false
	}
	entry := schema.GetEntry(name)
	commands = declaresTreeCommand(entry)
	if strings.HasSuffix(name, cmdModuleSuffix) && writesTreeNode(entry) {
		return moduleUseAdds, commands
	}
	if len(module.RPC) > 0 {
		return moduleUseAdds, commands
	}
	if declaresNothing(module) {
		return moduleUseEmpty, commands
	}
	return moduleUseIdle, commands
}

// declaresNothing reports whether module holds no statement any reader could
// take: no data node, augment, uses, rpc, notification, typedef, grouping,
// identity, extension, feature, deviation, include or extension statement.
// What remains is its header, its imports and its revisions.
func declaresNothing(module *gyang.Module) bool {
	counts := []int{
		len(module.Anydata), len(module.Anyxml), len(module.Augment), len(module.Choice),
		len(module.Container), len(module.Deviation), len(module.Extension), len(module.Extensions),
		len(module.Feature), len(module.Grouping), len(module.Identity), len(module.Include),
		len(module.Leaf), len(module.LeafList), len(module.List), len(module.Notification),
		len(module.RPC), len(module.Typedef), len(module.Uses),
	}
	for _, count := range counts {
		if count > 0 {
			return false
		}
	}
	return true
}

// importedModules answers the name of each module that another module of
// schema imports.
func importedModules(schema *Resolved) map[string]bool {
	imported := map[string]bool{}
	for _, name := range schema.ModuleNames() {
		module := schema.GetModule(name)
		if module == nil {
			continue
		}
		for _, imp := range module.Import {
			if imp.Name != name {
				imported[imp.Name] = true
			}
		}
	}
	return imported
}

// idleModuleRefusal is the refusal of a module that adds nothing, with the one
// change that makes it add what it holds: commands is whether it declares a
// command the tree would read under a -cmd name.
func idleModuleRefusal(module declaredModule, commands bool) error {
	plugin := quoted(module.external.Plugin)
	problem := "Module " + quoted(module.name) + " of plugin " + plugin + " adds nothing to the config schema, the commands or the rpcs"
	why := "Ze reads config nodes only from a module whose name ends in -conf or that augments another module, " +
		"commands only from a module whose name ends in -cmd that declares a ze:command or a ze:help, rpcs from rpc statements, " +
		"and typedefs and groupings only through a module that imports them"
	switch {
	case commands:
		return opererr.New(problem, why,
			"rename the module to "+quoted(module.name+cmdModuleSuffix)+" in the schemas that plugin "+plugin+" declares, then restart ze")
	case strings.HasSuffix(module.name, cmdModuleSuffix):
		return opererr.New(problem, why,
			"add `config false;` and a ze:command statement to each command container of module "+quoted(module.name)+
				" in the schemas that plugin "+plugin+" declares, then restart ze")
	default:
		return opererr.New(problem, why,
			"rename the module to "+quoted(module.name+"-conf")+" in the schemas that plugin "+plugin+" declares, then restart ze")
	}
}

// emptyModuleRefusal is the refusal of a module that declares no statement at
// all. No rename makes it add anything, so the fix is to drop it from the
// declaration or give it the definitions it was meant to carry.
func emptyModuleRefusal(module declaredModule) error {
	plugin := quoted(module.external.Plugin)
	return opererr.New(
		"Module "+quoted(module.name)+" of plugin "+plugin+" adds nothing to the config schema, the commands or the rpcs: "+
			"it declares no statement, only its header, imports and revisions",
		"ze joins a module for the definitions it carries, and an empty one would hold a module name and a namespace "+
			"that no config node, command or rpc uses",
		"remove module "+quoted(module.name)+" from the schemas that plugin "+plugin+
			" declares, or add the definitions it is meant to carry, then restart ze")
}

// rootHolder is the module that defines one top-level config node.
type rootHolder struct {
	module string
	prefix string
	// plugin is the plugin that declared the module, and empty for a module of
	// the compiled-in set.
	plugin string
}

// refuseRedefinedRoots refuses an external config module whose top-level node
// a config module of compiled, or an earlier external module, already defines.
// The modules of compiled are not judged against each other.
func refuseRedefinedRoots(compiled, joined *Resolved, declared []declaredModule) error {
	holders := map[string]rootHolder{}
	for _, name := range compiled.ConfModuleNames() {
		holder := rootHolder{module: name, prefix: modulePrefix(compiled.GetModule(name))}
		for node := range compiled.GetEntry(name).Dir {
			if _, held := holders[node]; !held {
				holders[node] = holder
			}
		}
	}
	configModules := joined.ConfModuleNames()
	for _, module := range declared {
		if !slices.Contains(configModules, module.name) {
			// Its top-level nodes do not enter the config schema.
			continue
		}
		for _, node := range slices.Sorted(maps.Keys(joined.GetEntry(module.name).Dir)) {
			holder, held := holders[node]
			if held {
				return redefinedRootRefusal(module, node, holder)
			}
			holders[node] = rootHolder{module: module.name, prefix: module.prefix, plugin: module.external.Plugin}
		}
	}
	return nil
}

// refuseUnheldRegistrations refuses an external module that names a daemon-side
// registration the daemon does not hold: a ze:validate function absent from
// validators, or a ze:reference source no component registered. Each name is
// judged against the registry its own extension reads. The walk covers every
// entry of the joined set and keeps the ones an external module wrote, so a
// leaf augmented into a compiled-in container is judged too.
func refuseUnheldRegistrations(joined *Resolved, declared []declaredModule, validators *ValidatorRegistry) error {
	owners := make(map[*gyang.Module]declaredModule, len(declared))
	for _, module := range declared {
		owners[joined.GetModule(module.name)] = module
	}
	names := joined.ModuleNames()
	slices.Sort(names)
	for _, name := range names {
		// An explicit stack, not recursion: an external module chooses the depth.
		pending := []*gyang.Entry{joined.GetEntry(name)}
		for len(pending) > 0 {
			entry := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if entry == nil {
				continue
			}
			for _, child := range slices.Sorted(maps.Keys(entry.Dir)) {
				pending = append(pending, entry.Dir[child])
			}
			if entry.Node == nil {
				continue
			}
			owner, external := owners[gyang.RootNode(entry.Node)]
			if !external {
				continue
			}
			if err := refuseFirstPassAddition(entry, owner); err != nil {
				return err
			}
			if err := unheldRegistration(entry, owner, validators); err != nil {
				return err
			}
		}
	}
	return nil
}

// firstPassRoots names the config roots ze reads before any plugin runs, over
// the compiled-in modules alone, and strictly: `plugin`, which says which
// plugins to run, and `environment`. It is the one declaration of that set:
// the first pass of the config load derives its schema from it
// (FirstPassRoots).
var firstPassRoots = []string{"plugin", "environment"}

// FirstPassRoots answers the config roots ze reads before any plugin runs.
func FirstPassRoots() []string {
	return slices.Clone(firstPassRoots)
}

// refuseFirstPassAddition refuses a node an external module adds under a root
// of the first pass. Ze reads those roots before it starts a plugin, when the
// module is not known yet, and reads them strictly, so a config that used the
// added name would be refused as unknown before the plugin that declares it
// ran: the name could never be configured. Refusing the module at the join
// says so once, at start, in place of a refusal of the operator's config that
// names no module.
//
// entry MUST be a node owner's module wrote. A root of that name the module
// defines itself never reaches here: refuseRedefinedRoots refused it.
func refuseFirstPassAddition(entry *gyang.Entry, owner declaredModule) error {
	// The climb is bounded by the depth of entry, which the walk that found
	// it already traversed.
	top := entry
	for top.Parent != nil {
		if top.Parent.Parent == nil {
			break
		}
		top = top.Parent
	}
	if top == entry {
		return nil
	}
	if !slices.Contains(firstPassRoots, top.Name) {
		return nil
	}
	plugin := quoted(owner.external.Plugin)
	return opererr.New(
		"Module "+quoted(owner.name)+" of plugin "+plugin+" adds "+entry.Path()+" under the config root "+quoted(top.Name)+
			", which ze reads before it starts a plugin",
		"ze reads the "+strings.Join(firstPassRoots, " and ")+" blocks before any external plugin runs, from the schema compiled into ze, "+
			"so a config that used the added name would be refused before plugin "+plugin+" declared it",
		"move "+quoted(entry.Name)+" out of "+quoted(top.Name)+" into a config root module "+quoted(owner.name)+
			" defines, then restart ze",
	)
}

// unheldRegistration answers the refusal for the first registration entry
// names that the daemon does not hold, or nil. unheldName judges the name, as
// it does for a compiled-in module (RefuseUnheldCompiledRegistrations); this
// refusal names the plugin whose module wrote entry.
func unheldRegistration(entry *gyang.Entry, owner declaredModule, validators *ValidatorRegistry) error {
	extension, name := unheldName(entry, validators)
	if name == "" {
		return nil
	}
	return unheldRegistrationRefusal(owner, entry, extension, name)
}

// refuseDeclarationMismatch refuses a declaration whose module or namespace
// field differs from what the text states.
func refuseDeclarationMismatch(module declaredModule) error {
	plugin := quoted(module.external.Plugin)
	if module.external.Module != module.name {
		return opererr.New(
			"Plugin "+plugin+" declares module "+quoted(module.external.Module)+" and its YANG text is module "+quoted(module.name),
			"the module field and the text must name one module, so Ze reports the module the plugin brings",
			"set the module field of the schema that plugin "+plugin+" declares to "+quoted(module.name)+", then restart ze",
		)
	}
	if module.external.Namespace != module.namespace {
		return opererr.New(
			"Plugin "+plugin+" declares namespace "+quoted(module.external.Namespace)+" for module "+
				quoted(module.name)+" and its YANG text states namespace "+quoted(module.namespace),
			"the namespace field and the text must state one namespace, so Ze reports the namespace the plugin brings",
			"set the namespace field of the schema that plugin "+plugin+" declares to "+quoted(module.namespace)+", then restart ze",
		)
	}
	return nil
}

// refuseNameClash refuses a module whose name the compiled-in set holds, at
// any revision, or that an earlier external module holds.
func refuseNameClash(set *sourcedModules, declared []declaredModule, next declaredModule) error {
	plugin := quoted(next.external.Plugin)
	fix := "rename module " + quoted(next.name) + " in the schema that plugin " + plugin + " declares, then restart ze"
	if set.holdsName(next.name) {
		holder := "a module compiled into ze"
		if registrar, registered := ModuleRegistrar(next.name); registered {
			holder += " (registered by " + registrar + ")"
		}
		return opererr.New(
			"Module "+quoted(next.name)+" of plugin "+plugin+" has the name of "+holder,
			"two modules with one name make the config schema ambiguous, whatever revision each declares",
			fix,
		)
	}
	for _, earlier := range declared {
		if earlier.name != next.name {
			continue
		}
		return opererr.New(
			"Module "+quoted(next.name)+" is declared by plugin "+quoted(earlier.external.Plugin)+" and by plugin "+plugin,
			"two modules with one name make the config schema ambiguous, whatever revision each declares",
			fix,
		)
	}
	return nil
}

// refuseNamespaceClash refuses a module whose namespace a module of the
// compiled-in set holds, or that an earlier external module holds.
func refuseNamespaceClash(compiledHolders map[string]string, declared []declaredModule, next declaredModule) error {
	plugin := quoted(next.external.Plugin)
	fix := "change the namespace of module " + quoted(next.name) + " in the schema that plugin " + plugin + " declares, then restart ze"
	if holder, held := compiledHolders[next.namespace]; held {
		return opererr.New(
			"Namespace "+quoted(next.namespace)+" of module "+quoted(next.name)+" of plugin "+plugin+
				" is the namespace of module "+quoted(holder)+", compiled into ze",
			"a namespace identifies one module, so two modules cannot state the same one",
			fix,
		)
	}
	for _, earlier := range declared {
		if earlier.namespace != next.namespace {
			continue
		}
		return opererr.New(
			"Namespace "+quoted(next.namespace)+" of module "+quoted(next.name)+" of plugin "+plugin+
				" is the namespace of module "+quoted(earlier.name)+" of plugin "+quoted(earlier.external.Plugin),
			"a namespace identifies one module, so two modules cannot state the same one",
			fix,
		)
	}
	return nil
}

// invalidModuleRefusal is the refusal of a module the loader does not accept.
// cause is the loader's error, which carries the location in the text.
func invalidModuleRefusal(external ExternalModule, cause error) error {
	plugin := quoted(external.Plugin)
	return opererr.Wrap(cause,
		"The YANG module "+quoted(external.Module)+" that plugin "+plugin+" declares is not valid",
		"Ze cannot read config under a module that does not compile",
		"correct the YANG text in the schema that plugin "+plugin+" declares, then restart ze",
	)
}

// invalidTogetherRefusal is the refusal of several external modules that each
// resolve alone and fail together, so no one of them can be named.
func invalidTogetherRefusal(declared []declaredModule, cause error) error {
	plugins := make([]string, 0, len(declared))
	for _, module := range declared {
		plugins = append(plugins, quoted(module.external.Plugin))
	}
	list := strings.Join(plugins, ", ")
	return opererr.Wrap(cause,
		"The YANG modules that plugins "+list+" declare are not valid together",
		"each module compiles alone, and Ze cannot read config under a set of modules that does not compile",
		"correct the YANG text in the schemas that plugins "+list+" declare, then restart ze",
	)
}

// moduleCountRefusal is the refusal of a text that holds no module, several
// modules, or a submodule.
func moduleCountRefusal(external ExternalModule) error {
	plugin := quoted(external.Plugin)
	return opererr.New(
		"A YANG text that plugin "+plugin+" declares does not hold exactly one YANG module",
		"each entry of the schemas a plugin declares names one module, so a submodule or a second module in its text has no declared name",
		"write one module statement in each YANG text that plugin "+plugin+" declares, one entry for each module, then restart ze",
	)
}

// missingNamespaceRefusal is the refusal of a module that states no namespace.
func missingNamespaceRefusal(external ExternalModule, module string) error {
	plugin := quoted(external.Plugin)
	return opererr.New(
		"Module "+quoted(module)+" of plugin "+plugin+" states no namespace",
		"a namespace identifies the module, and Ze refuses a module it cannot tell from another",
		"add a namespace statement to module "+quoted(module)+" in the schema that plugin "+plugin+" declares, then restart ze",
	)
}

// redefinedRootRefusal is the refusal of a top-level node another module
// already defines, with the augment that reaches the same node.
func redefinedRootRefusal(module declaredModule, node string, holder rootHolder) error {
	plugin := quoted(module.external.Plugin)
	definer := "module " + quoted(holder.module) + ", compiled into ze"
	if holder.plugin != "" {
		definer = "module " + quoted(holder.module) + " of plugin " + quoted(holder.plugin)
	}
	return opererr.New(
		"Module "+quoted(module.name)+" of plugin "+plugin+" defines top-level node "+quoted(node)+", which "+definer+" already defines",
		"a second definition of a config root is refused: an external module extends an existing root with augment",
		"in module "+quoted(module.name)+" of plugin "+plugin+", replace the definition of "+quoted(node)+
			" with: import "+holder.module+" { prefix "+holder.prefix+"; } augment \"/"+holder.prefix+":"+node+"\" { ... }",
	)
}

// unheldRegistrationRefusal is the refusal of an extension that names a
// registration the daemon does not hold.
func unheldRegistrationRefusal(owner declaredModule, entry *gyang.Entry, extension, name string) error {
	plugin := quoted(owner.external.Plugin)
	return opererr.New(
		"Module "+quoted(owner.name)+" of plugin "+plugin+" names "+extension+" "+quoted(name)+" at "+entry.Path()+", which ze does not hold",
		"an extension that names a registration the daemon lacks would check nothing",
		"remove "+extension+" "+quoted(name)+" from module "+quoted(owner.name)+
			" and check the value in the config verify step of plugin "+plugin+", then restart ze",
	)
}

// quoted answers s between double quotes, as a name reads in a refusal.
func quoted(s string) string {
	return `"` + s + `"`
}
