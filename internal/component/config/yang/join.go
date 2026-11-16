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

// ExternalModule is the YANG module one external plugin declares: the `schema`
// object of its Stage 1 declaration, with the name of the plugin that sent it.
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
	// augments is whether the module carries a top-level augment statement.
	augments bool
}

// Join answers the module set that holds every module of compiled and every
// external module, resolved and checked, or an operator error that names the
// plugin and the module it refuses. Every check Loader.Resolve runs is run
// over the joined set, and the join adds the refusals a plugin's module owes
// beyond them:
//
//   - the text does not parse or resolve, or holds anything but one module
//   - the declared module name or namespace differs from the text
//   - the module name is held by a module of compiled, at any revision, or by
//     an earlier external module
//   - the namespace is held the same way
//   - the module adds nothing to the config schema: its name does not end in
//     -conf and it augments nothing
//   - a top-level node of a -conf external module is already defined by a
//     -conf module of compiled or by an earlier external module
//   - the module names a ze:validate function validators does not hold, or a
//     ze:reference source no component registered
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
	joined, err := resolveJoined(set, declared)
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
			augments:  len(module.Augment) > 0,
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

// reloaded answers a new Loader holding every module text of the set, so a
// module can be added beside them and the whole resolved again: the set
// itself refuses every load once Resolve ran.
func (m *sourcedModules) reloaded() (*Loader, error) {
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
	return loader, nil
}

// resolveJoined resolves the modules of set with the declared modules added,
// and names the module a failure belongs to. The set resolved alone, so a
// failure comes from an external module. When several were added, each is
// resolved alone with the set to find the one that fails; that recursion is
// one level deep, because a call with one module never recurses.
func resolveJoined(set *sourcedModules, declared []declaredModule) (*Resolved, error) {
	loader, err := set.reloaded()
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
	for _, module := range declared {
		if _, aloneErr := resolveJoined(set, []declaredModule{module}); aloneErr != nil {
			return nil, aloneErr
		}
	}
	return nil, invalidTogetherRefusal(declared, resolveErr)
}

// refuseIdleModules refuses an external module that would add nothing to the
// config schema: the schema takes top-level nodes only from a config module
// (Resolved.ConfModuleNames), and any other module reaches it by an augment.
func refuseIdleModules(joined *Resolved, declared []declaredModule) error {
	configModules := joined.ConfModuleNames()
	for _, module := range declared {
		if slices.Contains(configModules, module.name) {
			continue
		}
		if module.augments {
			continue
		}
		return opererr.New(
			"Module "+quoted(module.name)+" of plugin "+quoted(module.external.Plugin)+" adds nothing to the config schema",
			"Ze reads top-level config nodes only from a module whose name ends in -conf, and this module augments no other module",
			"rename the module to "+quoted(module.name+"-conf")+" in the schema that plugin "+
				quoted(module.external.Plugin)+" declares, or augment an existing container, then restart ze",
		)
	}
	return nil
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
			if err := unheldRegistration(entry, owner, validators); err != nil {
				return err
			}
		}
	}
	return nil
}

// unheldRegistration answers the refusal for the first registration entry
// names that the daemon does not hold, or nil.
func unheldRegistration(entry *gyang.Entry, owner declaredModule, validators *ValidatorRegistry) error {
	for _, name := range SplitValidatorNames(GetValidateExtension(entry)) {
		if validators.Get(name) == nil {
			return unheldRegistrationRefusal(owner, entry, "ze:validate", name)
		}
	}
	source := GetReferenceExtension(entry)
	if source == "" {
		return nil
	}
	if _, held := referenceSources[source]; held {
		return nil
	}
	return unheldRegistrationRefusal(owner, entry, "ze:reference", source)
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
		"The schema that plugin "+plugin+" declares does not hold exactly one YANG module",
		"Ze joins one module for each plugin, and a submodule or a second module has no declared name",
		"write one module statement in the YANG text that plugin "+plugin+" declares, then restart ze",
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
