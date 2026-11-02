// Design: docs/architecture/config/yang-config-design.md -- Module Loading
// Related: yang_schema.go -- loadYANGModules, the module set this reads

package config

import (
	"errors"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CheckFilterInsertionPaths judges the config condition path of each of a
// plugin's auto-insertions against the compiled schema
// (spec-filter-wire-6-stage-filters A-10, AC-2): the path, continuing a
// peer's entry (rpc.FilterInsertionSchemaPath), MUST be a node the daemon
// compiled, or no config could ever meet it.
//
// ownerYANG is the module text of a compiled-in registration, empty for a
// third-party plugin. For a compiled-in one the node the path names MUST be
// defined by that module, an augment included, so the insertion applies only
// where config the plugin owns is set, and that config's presence is what loads the
// plugin. A third-party plugin is loaded by the operator's `plugin external`
// entry, and its own YANG is not compiled (docs/plugin-development/schema.md),
// so it may name any compiled path.
func CheckFilterInsertionPaths(pluginName, ownerYANG string, insertions []rpc.FilterInsertion) error {
	if len(insertions) == 0 {
		return nil
	}
	owner := ""
	if ownerYANG != "" {
		name, err := YANGModuleName(ownerYANG)
		if err != nil {
			return insertionPathError(pluginName, "", "carries YANG that declares no module: "+err.Error())
		}
		owner = name
	}
	for i := range insertions {
		if err := checkFilterInsertionPath(pluginName, owner, &insertions[i]); err != nil {
			return err
		}
	}
	return nil
}

func checkFilterInsertionPath(pluginName, owner string, ins *rpc.FilterInsertion) error {
	// A session condition names no config path: registry.CheckFilterInsertions
	// judges its facts against the facts their owners registered.
	if len(ins.Config) == 0 {
		return nil
	}
	path := rpc.FilterInsertionSchemaPath(ins.Config)
	definer, err := SchemaNodeModule(path)
	if err != nil {
		return insertionPathError(pluginName, ins.Filter, "conditions on "+strings.Join(path, " ")+
			", and "+err.Error()+"; name a path of the peer config the daemon compiles")
	}
	if owner == "" {
		return nil
	}
	// The node the condition names is judged, not the first word: a plugin's
	// module may augment a node another module defines (bgp-gr's
	// long-lived-stale-time sits under bgp's session capability), and it is
	// the named node's presence that the plugin's config sets.
	if definer != owner {
		return insertionPathError(pluginName, ins.Filter, "conditions on "+strings.Join(path, " ")+
			", which module "+definer+" defines, not the plugin's own module "+owner+
			"; condition on config the plugin's YANG adds, so setting it loads the plugin")
	}
	return nil
}

func insertionPathError(pluginName, filter, rule string) error {
	var tb textbuf.Buffer
	tb.Str("plugin ").Quoted(pluginName)
	if filter != "" {
		tb.Str(" declares the filter insertion ").Quoted(filter)
	}
	return errors.New(tb.Str(" ").Str(rule).Str(" (docs/architecture/api/process-protocol.md, \"Filter Insertion\")").String())
}

// SchemaNodeModule answers the name of the YANG module that defines the node
// at path (one word per schema level, list keys omitted) in the compiled
// schema: for a node another module adds by augment, the augmenting module.
// A path the schema does not hold is an error naming the first word missing.
// The bound is the path's length.
func SchemaNodeModule(path []string) (string, error) {
	if len(path) == 0 {
		return "", errors.New("BUG: config: SchemaNodeModule asked for an empty path")
	}
	resolved, err := loadYANGModules(nil)
	if err != nil {
		return "", err
	}
	var entry *yang.Entry
	for _, name := range resolved.ConfModuleNames() {
		root := resolved.GetEntry(name)
		if root == nil {
			continue
		}
		if top, found := root.Dir[path[0]]; found {
			entry = top
			break
		}
	}
	for i, word := range path {
		if i > 0 {
			entry = entry.Dir[word]
		}
		if entry == nil {
			var tb textbuf.Buffer
			return "", errors.New(tb.Str("the schema holds no node ").Quoted(strings.Join(path[:i+1], " ")).String())
		}
	}
	module := yang.RootNode(entry.Node)
	if module == nil {
		return "", errors.New("BUG: config: schema node " + strings.Join(path, " ") + " has no defining module")
	}
	return module.Name, nil
}

// YANGModuleName answers the name a module's text declares (its first
// statement, `module <name>`), or an error when the text declares no module.
func YANGModuleName(text string) (string, error) {
	statements, err := yang.Parse(text, "module text")
	if err != nil {
		return "", err
	}
	if len(statements) == 0 {
		return "", errors.New("the YANG text declares no module")
	}
	if statements[0].Keyword != "module" {
		return "", errors.New("the YANG text declares no module")
	}
	return statements[0].Argument, nil
}
