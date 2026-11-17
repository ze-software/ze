// Design: docs/architecture/config/yang-config-design.md — joining an external plugin's module
// Related: join.go — Join, which calls refuseCommandConflicts after the config checks
// Related: command.go — BuildCommandTree, whose merge rule these refusals state
// Related: rpc_publish.go — PublishedRPCs, the ze:rpc and wire method checks every module obeys

package yang

import (
	"errors"
	"maps"
	"slices"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/opererr"
)

// commandHolder is the module that declares one command path or one wire
// method, as a refusal names it.
type commandHolder struct {
	module string
	// plugin is the plugin that declared the module, and empty for a module of
	// the compiled-in set.
	plugin string
}

// describe names the holder in a refusal.
func (h commandHolder) describe() string {
	if h.plugin == "" {
		return "module " + quoted(h.module) + ", compiled into ze"
	}
	return "module " + quoted(h.module) + " of plugin " + quoted(h.plugin)
}

// refuseCommandConflicts judges the commands and rpcs of the external modules
// by the rules every module obeys, compiled-in or not, and refuses what an
// external module changes only because it arrives after the set it joins:
//
//   - a ze:command on a path where another module already declares one:
//     BuildCommandTree keeps the first ze:command a node meets, so the second
//     would be dropped in silence, or would take the command of the first
//   - a ze:command wire method another module already declares: the wire
//     method names one command
//   - what PublishedRPCs refuses over the joined set and not over the
//     compiled-in one (a ze:rpc that names an rpc its module does not declare,
//     a pointer on a node with no ze:command, a malformed wire method), and a
//     ze:rpc whose module no module of the joined set is: a compiled-in pointer
//     may name a module the build left out (RPCPublication.Unlinked), and
//     nothing leaves out a module of an external plugin but the plugin itself
//
// Containers are shared as BuildCommandTree shares them, by every module.
// The modules of compiled are not judged against each other.
func refuseCommandConflicts(compiled, joined *Resolved, declared []declaredModule) error {
	owners := make(map[*gyang.Module]declaredModule, len(declared))
	for _, module := range declared {
		owners[joined.GetModule(module.name)] = module
	}
	paths := map[string]commandHolder{}
	methods := map[string]commandHolder{}
	for _, name := range compiled.moduleNamesBySuffix(cmdModuleSuffix) {
		recordCompiledCommands(compiled.GetEntry(name), paths, methods)
	}
	for _, name := range joined.moduleNamesBySuffix(cmdModuleSuffix) {
		err := walkEntries(joined.GetEntry(name), func(entry *gyang.Entry) error {
			method := GetCommandExtension(entry)
			if method == "" || entry.Node == nil {
				return nil
			}
			owner, external := owners[gyang.RootNode(entry.Node)]
			if !external {
				return nil
			}
			return claimCommand(owner, entry, method, paths, methods)
		})
		if err != nil {
			return err
		}
	}
	return refuseRPCLinks(compiled, joined, declared)
}

// recordCompiledCommands records the path and the wire method of each
// ze:command a compiled-in -cmd module declares, the first holder kept: the
// compiled-in modules share aliases and are not judged against each other.
func recordCompiledCommands(module *gyang.Entry, paths, methods map[string]commandHolder) {
	walkEntries(module, func(entry *gyang.Entry) error { //nolint:errcheck // the visit never fails, so the walk answers nil
		method := GetCommandExtension(entry)
		if method == "" {
			return nil
		}
		holder := commandHolder{module: definingModule(entry)}
		path := commandWords(entry)
		if _, held := paths[path]; !held {
			paths[path] = holder
		}
		if _, held := methods[method]; !held {
			methods[method] = holder
		}
		return nil
	})
}

// claimCommand records the ze:command an external module declares at entry,
// or refuses it when another module holds its path or its wire method. The
// module's own aliases may share a wire method, as compiled-in aliases do.
func claimCommand(owner declaredModule, entry *gyang.Entry, method string, paths, methods map[string]commandHolder) error {
	plugin := quoted(owner.external.Plugin)
	path := commandWords(entry)
	if holder, held := paths[path]; held && holder.module != owner.name {
		return opererr.New(
			"Module "+quoted(owner.name)+" of plugin "+plugin+" declares command "+quoted(path)+
				", which "+holder.describe()+" already declares",
			"ze keeps the first module that declares a command, so the command of plugin "+plugin+" would never run",
			"rename container "+quoted(entry.Name)+" of command "+quoted(path)+" in module "+quoted(owner.name)+
				" to a name no other module uses, in the schemas that plugin "+plugin+" declares, then restart ze",
		)
	}
	if holder, held := methods[method]; held && holder.module != owner.name {
		return opererr.New(
			"Module "+quoted(owner.name)+" of plugin "+plugin+" declares wire method "+quoted(method)+" at "+
				quoted(path)+", which "+holder.describe()+" already declares",
			"a wire method names one command, so two modules cannot declare the same one",
			"change ze:command "+quoted(method)+" at "+quoted(path)+" in module "+quoted(owner.name)+
				" to a name no other module uses, in the schemas that plugin "+plugin+" declares, then restart ze",
		)
	}
	paths[path] = commandHolder{module: owner.name, plugin: owner.external.Plugin}
	methods[method] = commandHolder{module: owner.name, plugin: owner.external.Plugin}
	return nil
}

// refuseRPCLinks runs PublishedRPCs, the check of every ze:rpc pointer and
// wire method the schema publishes, over the joined set. A refusal it answers
// for the joined set and not for the compiled-in one comes from an external
// module; so does a pointer whose module no module of the joined set is.
//
// A compiled-in set that already fails PublishedRPCs does not hide what the
// join adds: the joined set is refused for each failure the compiled-in set
// does not have, and a failure both have is the compiled-in set's own, which
// no plugin is blamed for.
func refuseRPCLinks(compiled, joined *Resolved, declared []declaredModule) error {
	before, compiledErr := PublishedRPCs(compiled)
	after, joinedErr := PublishedRPCs(joined)
	plugins := commandPlugins(declared)
	if added := addedFailures(compiledErr, joinedErr); added != nil {
		return opererr.Wrap(added,
			"The commands that the YANG modules of plugin "+plugins+" declare do not match their rpcs",
			"ze publishes each command with the parameters of the rpc its ze:rpc names, under its ze:command "+
				"wire method, and it cannot publish a command whose pointer or wire method is wrong",
			"correct each ze:rpc and ze:command named below in the schemas that plugin "+plugins+" declares, then restart ze",
		)
	}
	for _, unlinked := range after.Unlinked {
		if slices.Contains(before.Unlinked, unlinked) {
			continue
		}
		return opererr.New(
			"A command that plugin "+plugins+" declares names an rpc in a module ze does not hold: "+unlinked,
			"ze reads the parameters of the command from that rpc, and no module the plugin or ze brings defines it",
			"add the module that defines the rpc to the schemas that plugin "+plugins+
				" declares, or remove the ze:rpc from the command, then restart ze",
		)
	}
	return nil
}

// addedFailures answers the failures of joined, the PublishedRPCs error of the
// joined set, that compiled, the error of the compiled-in set, does not hold,
// joined as one error, or nil when joined adds none. PublishedRPCs joins one
// error per failure (errors.Join), and a failure is the same when its text is:
// each text names the node, the method or the rpc it is about.
func addedFailures(compiled, joined error) error {
	if joined == nil {
		return nil
	}
	held := map[string]bool{}
	for _, failure := range failuresOf(compiled) {
		held[failure.Error()] = true
	}
	var added []error
	for _, failure := range failuresOf(joined) {
		if !held[failure.Error()] {
			added = append(added, failure)
		}
	}
	return errors.Join(added...)
}

// failuresOf answers the failures err joins, err itself when it joins none,
// and none for nil.
func failuresOf(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := errors.AsType[interface {
		error
		Unwrap() []error
	}](err); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

// ModuleCommand is one ze:command node of a module: the command it stands
// for, as an operator types it, and the module that wrote the node.
type ModuleCommand struct {
	Path   string
	Module string
}

// ModuleCommands answers each ze:command node that one of modules wrote in
// schema, sorted by path: the nodes BuildCommandTree reads, reached from a
// -cmd module through `config false` containers, a node a module adds by
// augment included. A module of modules that schema does not hold writes no
// node. The walk is an explicit stack, because an external module chooses the
// depth.
func ModuleCommands(schema *Resolved, modules []string) []ModuleCommand {
	var commands []ModuleCommand
	for _, name := range schema.moduleNamesBySuffix(cmdModuleSuffix) {
		root := schema.GetEntry(name)
		if root == nil {
			continue
		}
		pending := []*gyang.Entry{root}
		for len(pending) > 0 {
			entry := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			for _, child := range entry.Dir {
				if child.Config != gyang.TSFalse {
					continue
				}
				pending = append(pending, child)
				if GetCommandExtension(child) == "" {
					continue
				}
				writer := definingModule(child)
				if slices.Contains(modules, writer) {
					commands = append(commands, ModuleCommand{Path: commandWords(child), Module: writer})
				}
			}
		}
	}
	slices.SortFunc(commands, func(a, b ModuleCommand) int { return strings.Compare(a.Path, b.Path) })
	return commands
}

// commandPlugins names the plugins whose modules can carry a command or an
// rpc, as a refusal lists them: each plugin once, in declaration order.
func commandPlugins(declared []declaredModule) string {
	var plugins []string
	for _, module := range declared {
		name := quoted(module.external.Plugin)
		if !slices.Contains(plugins, name) {
			plugins = append(plugins, name)
		}
	}
	return strings.Join(plugins, ", ")
}

// declaresTreeCommand reports whether a module entry declares a command the
// tree reads: a ze:command on a container reached, as BuildCommandTree reaches
// it, through `config false` containers alone. The walk is an explicit stack,
// because an external module chooses the depth.
func declaresTreeCommand(module *gyang.Entry) bool {
	if module == nil {
		return false
	}
	pending := []*gyang.Entry{module}
	for len(pending) > 0 {
		entry := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, child := range entry.Dir {
			if child.Config != gyang.TSFalse {
				continue
			}
			if GetCommandExtension(child) != "" {
				return true
			}
			pending = append(pending, child)
		}
	}
	return false
}

// writesTreeNode reports whether a module entry writes a node the command tree
// takes something from, reached as declaresTreeCommand reaches one: a
// ze:command, or a ze:help summary, which BuildCommandTree gives a navigation
// node with no command. The second is how a verb-root anchor such as the
// compiled-in ze-cli-monitor-cmd gives the help of a branch other modules fill
// with commands. The walk is an explicit stack, because an external module
// chooses the depth.
func writesTreeNode(module *gyang.Entry) bool {
	if module == nil {
		return false
	}
	pending := []*gyang.Entry{module}
	for len(pending) > 0 {
		entry := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, child := range entry.Dir {
			if child.Config != gyang.TSFalse {
				continue
			}
			if GetCommandExtension(child) != "" {
				return true
			}
			if GetHelpExtension(child.Exts) != "" {
				return true
			}
			pending = append(pending, child)
		}
	}
	return false
}

// definingModule answers the name of the module that wrote entry: the module
// whose tree holds it, or the module of the augment that added it.
func definingModule(entry *gyang.Entry) string {
	if entry.Node != nil {
		if module := gyang.RootNode(entry.Node); module != nil {
			return module.Name
		}
	}
	top := entry
	for top.Parent != nil {
		top = top.Parent
	}
	return top.Name
}

// commandWords answers the command an entry stands for, as an operator types
// it: the names from the top-level container down to entry. The climb is
// bounded by the depth of entry, which the walk that found it traversed.
func commandWords(entry *gyang.Entry) string {
	var words []string
	for node := entry; node != nil && node.Parent != nil; node = node.Parent {
		words = append(words, node.Name)
	}
	slices.Reverse(words)
	return strings.Join(words, " ")
}

// walkEntries calls visit for every entry below module, in name order at each
// level. An explicit stack, because an external module chooses the depth.
func walkEntries(module *gyang.Entry, visit func(*gyang.Entry) error) error {
	if module == nil {
		return nil
	}
	pending := []*gyang.Entry{module}
	for len(pending) > 0 {
		entry := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if entry != module {
			if err := visit(entry); err != nil {
				return err
			}
		}
		names := slices.Sorted(maps.Keys(entry.Dir))
		slices.Reverse(names)
		for _, name := range names {
			pending = append(pending, entry.Dir[name])
		}
	}
	return nil
}
