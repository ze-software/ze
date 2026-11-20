// Design: docs/architecture/api/commands.md — command-owned pipe filters
// Related: pipe_filter.go — the registry, and the in-tree entry point
// Related: alias.go — RegisterPluginAliases, the entry point this one is shaped like

package command

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PluginPipeFilter is one pipe operation a caller outside this repository
// declares on one of its own command paths: the path, and the operation as an
// in-tree command states one.
type PluginPipeFilter struct {
	// Command is the command path the operation sits on.
	Command string
	// Filter is the name, the description and the argument kind.
	Filter PipeFilter
}

// RegisterPluginPipeFilters declares the pipe operations of a caller outside
// this repository into pipeFilterRegistry, the one registry of a command's own
// operations, under the owner's name. It reports a bad declaration rather than
// panicking on it.
//
// It refuses what RegisterPipeFilters refuses, through the same checks, and
// only the answer differs: RegisterPipeFilters is called from init(), so a bad
// declaration there is a Ze defect and a panic, while these strings arrived
// over a socket, so a bad one is an operating error and the daemon keeps
// running. Each refusal names the operation and what it collides with:
//
//   - a generic operator's name the catalog does not let a command override,
//     an override whose argument kind is not the generic's, and a kind outside
//     the closed vocabulary (declaredFilterRefusal);
//   - a name a pipe alias carries on an overlapping command path, in-tree or
//     another plugin's, and a global alias's name (aliasShadowing): the
//     operation would resolve first and the alias would never be reached;
//   - a command path that already holds a non-empty operation set, an in-tree
//     package's or another owner's. A path holds one set, and replacing it
//     would take operations a command already answers to;
//   - one name declared twice on one path in one message, where the later of
//     the two would win in silence.
//
// Nothing is recorded when any one declaration is refused: every declaration
// is checked before the first is stored, and a write that still fails
// withdraws what this call wrote. A caller therefore never undoes a partial
// declaration it did not ask for.
//
// The commands are every command path the owner declares. The empty set that
// stops a declared set reaching a command below the one it sits on is derived
// from them, as aliasBarriers derives it for aliases, so a declaring author
// never has to know how a command resolves its operations. Whether the owner
// may name a path at all is decided before the declaration arrives here.
//
// The owner is the name UnregisterPluginPipeFilters takes the declaration back
// under. It is opaque: nothing else in this package reads it.
//
// The caller MUST serialize two owners' registrations against each other, as
// plugin startup does (startupRegistrationMu), because the checks and the
// writes take the registry's lock separately.
func RegisterPluginPipeFilters(owner string, commands []string, declared []PluginPipeFilter) error {
	if len(declared) == 0 {
		return nil
	}
	sets, paths, err := pluginPipeFilterSets(declared)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := pluginPipeFilterCollision(path, sets[path]); err != nil {
			return err
		}
	}

	for _, path := range paths {
		if err := pipeFilterRegistry.declareFor(owner, path, sets[path]); err != nil {
			UnregisterPluginPipeFilters(owner)
			return err
		}
	}
	holds := func(path string) bool {
		_, held := pipeFilterRegistry.get(path)
		return held
	}
	for _, path := range inheritanceBarriers(commands, paths, holds) {
		if err := pipeFilterRegistry.declareFor(owner, path, pipeFilterSet{}); err != nil {
			UnregisterPluginPipeFilters(owner)
			return err
		}
	}
	return nil
}

// UnregisterPluginPipeFilters takes back every operation set, and every
// barrier, the owner declared, so each path returns to what it held before:
// a path an in-tree package declared empty returns to that empty declaration,
// and a path the owner found undeclared becomes undeclared again
// (declarationRegistry.withdraw).
//
// It reports nothing. A caller tears an owner down without knowing whether
// that owner ever declared an operation.
//
// Safe for concurrent use.
func UnregisterPluginPipeFilters(owner string) {
	pipeFilterRegistry.withdraw(owner)
}

// pluginPipeFilterSets builds one set for each declared command path, in the
// order the paths first appear, refusing what no registry state can make
// right: a declaration that names no path or no operation, a name declared
// twice on one path, and a declaration RegisterPipeFilters would refuse.
func pluginPipeFilterSets(declared []PluginPipeFilter) (map[string]pipeFilterSet, []string, error) {
	sets := make(map[string]pipeFilterSet, len(declared))
	paths := make([]string, 0, len(declared))
	for _, declaration := range declared {
		path := normalizeCommand(declaration.Command)
		filter := declaration.Filter
		filter.Name = strings.ToLower(strings.TrimSpace(filter.Name))
		if path == "" {
			var tb textbuf.Buffer
			return nil, nil, errors.New(tb.Str("pipe operation ").Str(filter.Name).Str(" names no command path").String())
		}
		if filter.Name == "" {
			var tb textbuf.Buffer
			return nil, nil, errors.New(tb.Quoted(path).Str(": a pipe operation has no name").String())
		}
		if refusal := declaredFilterRefusal([]string{path}, filter); refusal != "" {
			return nil, nil, errors.New(refusal)
		}
		set, seen := sets[path]
		if !seen {
			set = pipeFilterSet{byName: make(map[string]PipeFilter)}
			paths = append(paths, path)
		}
		if _, twice := set.byName[filter.Name]; twice {
			var tb textbuf.Buffer
			return nil, nil, errors.New(tb.Quoted(path).Str(": pipe operation ").Str(filter.Name).
				Str(" is declared twice in one message").String())
		}
		set.filters = append(set.filters, filter)
		set.byName[filter.Name] = filter
		sets[path] = set
	}
	return sets, paths, nil
}

// pluginPipeFilterCollision answers why one path's declared set collides with
// what the registries already hold, or nil when it does not.
func pluginPipeFilterCollision(path string, set pipeFilterSet) error {
	if name, aliasPath, shadowed := aliasShadowing([]string{path}, set.filters); shadowed {
		var tb textbuf.Buffer
		tb.Quoted(path).Str(": pipe operation ").Str(name).Str(" is the name of ")
		if aliasPath == globalPath {
			tb.Str("a global pipe alias")
		} else {
			tb.Str("a pipe alias of ").Quoted(aliasPath)
		}
		return errors.New(tb.Str(", which it would shadow; give the operation a name of its own").String())
	}
	held, found := pipeFilterRegistry.get(path)
	if !found || len(held.filters) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	tb.Quoted(path).Str(" already holds the pipe operations ").Str(held.filterNames()).Str(" of ")
	if holder := pipeFilterRegistry.ownerOf(path); holder != "" {
		tb.Str("plugin ").Quoted(holder)
	} else {
		tb.Str("a compiled-in command")
	}
	return errors.New(tb.Str(", so it cannot also hold ").Str(set.filterNames()).String())
}
