// Design: docs/architecture/plugin/plugin-system.md -- one plugin, two ways to reach the bus
// Related: stage1_checks.go -- the Stage 1 table, whose command-nodes-served entry calls refuseUnservedCommandNodes
// Related: stage1_schema.go -- refuseChangedSchema, which runs before it, so the published modules are the declared ones
// Related: command.go -- longestReach, the routing rule the dispatcher and this check share
// Related: command_registry.go -- commandNameRefusal, the name rule Register and this check share
// Related: ../../config/yang/join_command.go -- ModuleCommands, the ze:command nodes of a plugin's modules
// Related: ../../../le/doc/yangcontract/contract.go -- Validate, the same rule for the compiled-in modules

package server

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// unservedCommandWhy says why a ze:command node no declared command reaches is
// refused, and that a compiled-in module is held to the same rule.
const unservedCommandWhy = "ze sends a command of a plugin's module to the plugin through a command the plugin declares " +
	"at Stage 1, the longest registered command the command starts with, so a command of the module that no declared " +
	"command reaches would answer that it is not registered, or reach another plugin. A module compiled into ze is held " +
	"to the same rule when ze is built: every ze:command node has a handler"

// refuseUnservedCommandNodes refuses the Stage 1 declaration of the plugin
// named name when a ze:command node of a module it brought is not routed to it
// by the commands it declares. A line equal to the node's path goes where the
// dispatcher sends it: to the longest registered command it starts with on a
// word boundary (longestReach), among the names Register accepts
// (commandNameRefusal). site.routing holds the commands of the other plugins
// that decision also reads; a checker holds none, so it judges the plugin's
// own declaration alone.
//
// It is the rule `./le doc yang-contract command-contract` holds the
// compiled-in modules to when ze is built: every ze:command node has a
// handler. A compiled-in plugin's handler is a builtin RPC that forwards to its
// plugin command (RPCRegistration.PluginCommand); an external plugin has no
// builtin, so its declared command is the handler. Only the time differs,
// which is the transport: an external plugin's modules arrive at Stage 1.
//
// The converse is not refused, because a compiled-in plugin is not held to it:
// a declared command needs no ze:command node of its own (the kind "Plugin
// command (Stage 1 declaration)" of docs/architecture/api/commands.md, as
// `show vrrp interface`, whose node is `show vrrp interface name`).
//
// A plugin compiled into ze, and a plugin that brought no module, are not
// judged here. A module set the join refuses answers the join's own refusal,
// and no node is judged.
func refuseUnservedCommandNodes(name string, input *rpc.DeclareRegistrationInput, site stage1Site) error {
	if site.compiledIn {
		// A compiled-in plugin's modules are judged when ze is built.
		return nil
	}
	schema, modules, err := commandModules(name, input.Schemas)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		// The plugin brought no module, so no node of its own is to be served.
		return nil
	}
	for _, node := range yang.ModuleCommands(schema, modules) {
		if commandServed(node.Path, input.Commands, site.routing) {
			continue
		}
		return unservedCommandRefusal(name, node)
	}
	return nil
}

// commandModules answers the module set that holds the YANG modules the plugin
// named name brought, and the names of those modules. When the process
// published its external layer, that is the published set, which the daemon
// reads for every config and command: schema-unchanged runs first and refuses
// a declaration whose modules differ from it. When it published none, as in a
// checker that runs one program, it is the compiled-in set joined to the
// modules the declaration carries (config.JoinExternalModules), and a join
// refusal is answered as the join states it. No module answers no set.
func commandModules(name string, schemas []rpc.SchemaDecl) (*yang.Resolved, []string, error) {
	if published, fixed := yang.PublishedExternalModules(); fixed {
		return publishedCommandModules(name, published)
	}
	declared := DeclaredModules(name, schemas)
	if len(declared) == 0 {
		return nil, nil, nil
	}
	schema, err := config.JoinExternalModules(declared)
	if err != nil {
		return nil, nil, err
	}
	modules := make([]string, 0, len(declared))
	for _, module := range declared {
		modules = append(modules, module.Module)
	}
	return schema, modules, nil
}

// publishedCommandModules answers the set the process published and the names
// of the modules it published for the plugin named name: commandModules for a
// process that published its external layer.
func publishedCommandModules(name string, published []yang.ExternalModule) (*yang.Resolved, []string, error) {
	var modules []string
	for _, module := range published {
		if module.Plugin == name {
			modules = append(modules, module.Module)
		}
	}
	if len(modules) == 0 {
		return nil, nil, nil
	}
	schema, err := yang.DefaultLoader()
	if err != nil {
		return nil, nil, opererr.Wrap(err, "ze could not read the YANG modules it published for plugin "+quotedName(name),
			"ze checks the commands of the plugin's modules against the commands it declares, and cannot without them",
			restartDaemonFix)
	}
	return schema, modules, nil
}

// commandServed reports whether the dispatcher routes a line equal to path to
// the plugin that declares commands. The candidates are the names of commands
// Register accepts from the plugin (commandNameRefusal, against routing) and
// the commands routing holds for other plugins; the line goes to the longest
// one that reaches it (longestReach), and the node is served when that one is
// the plugin's. The line is compared in lower case, as the dispatcher
// compares a typed command.
func commandServed(path string, commands []rpc.CommandDecl, routing commandRouting) bool {
	type candidate struct {
		key string
		own bool
	}
	candidates := make([]candidate, 0, len(commands)+len(routing.commands))
	for i := range commands {
		if _, _, refused := commandNameRefusal(commands[i].Name, routing.builtins, routing.commands); refused {
			continue
		}
		candidates = append(candidates, candidate{key: strings.ToLower(commands[i].Name), own: true})
	}
	for key := range routing.commands {
		candidates = append(candidates, candidate{key: key})
	}
	matched := longestReach(strings.ToLower(path), len(candidates), func(i int) string { return candidates[i].key })
	if matched < 0 {
		return false
	}
	return candidates[matched].own
}

// unservedCommandRefusal is the refusal of node, a ze:command node of a module
// the plugin named name brought that none of its declared commands reaches.
func unservedCommandRefusal(name string, node yang.ModuleCommand) error {
	var problem textbuf.Buffer
	problem.Str("plugin ").Quoted(name).Str(" brings command ").Quoted(node.Path).Str(" in module ").
		Quoted(node.Module).Str(", and declares no command at Stage 1 that reaches it")
	var fix textbuf.Buffer
	fix.Str("add ").Quoted(node.Path).Str(" to the commands that plugin ").Quoted(name).
		Str(" declares at Stage 1, or remove its ze:command from module ").Quoted(node.Module).Str(", then restart ze")
	return opererr.New(problem.String(), unservedCommandWhy, fix.String())
}

// quotedName answers name in double quotes, as a refusal names a plugin.
func quotedName(name string) string {
	var text textbuf.Buffer
	text.Quoted(name)
	return text.String()
}

// DeclaresConfig reports whether a Stage 1 declaration of the plugin named name
// brings a module that adds config (yang.DefinesConfig): a plugin whose modules
// add only commands and rpcs defines no config section. The error is a module
// text that does not parse alone, which the join refuses before any caller
// asks this.
func DeclaresConfig(name string, schemas []rpc.SchemaDecl) (bool, error) {
	for _, module := range DeclaredModules(name, schemas) {
		definesConfig, err := yang.DefinesConfig(module)
		if err != nil {
			return false, err
		}
		if definesConfig {
			return true, nil
		}
	}
	return false, nil
}
