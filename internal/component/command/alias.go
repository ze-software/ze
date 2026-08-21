// Design: docs/architecture/api/commands.md — pipe aliases
// Related: pipe.go — the operators an alias expands into, and where it expands
// Related: pipe_filter.go — the command-owned filters an alias must not shadow
// Related: column_order.go — the command-path registry the per-command table reuses

package command

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
)

// Alias is a name an operator types in place of an operator chain.
//
// `show bgp | peers` says what `show bgp | display peers`
// says. An alias takes no argument and names no other alias, so what it stands
// for is fixed at registration and readable in one place.
type Alias struct {
	// Name is the word an operator types after the pipe character.
	Name string
	// Description is the line the completer shows beside the name.
	Description string
	// Expansion is the operator chain the name stands for, written the way an
	// operator would type it: "display peers", or "display state | count".
	Expansion string
}

// aliasEntry is one alias and the operators its expansion parses to. The
// expansion is parsed at registration, so no chain is parsed twice while a
// command runs.
type aliasEntry struct {
	alias Alias
	ops   []pipeOp
}

type aliasSet struct {
	byName map[string]aliasEntry
}

// globalPath is the command path of the global alias table, in a message a
// person reads. The table itself sits on no path at all.
const globalPath = "every command"

// aliasRegistry resolves a command to the aliases of its own, by the longest
// registered command path. It is the lookup the column-order and pipe-filter
// registries use.
var aliasRegistry = newCommandRegistry[aliasSet]()

// globalAliases holds the aliases every command answers to.
//
// It is a table of its own rather than a registration on the empty command
// path. commandRegistry.register skips an empty path. commandMatchesPrefix
// refuses an empty prefix against every non-empty command. A global registered
// that way would match nothing and report nothing.
//
// Safe for concurrent use.
var globalAliases = struct {
	mu     sync.RWMutex
	byName map[string]aliasEntry
}{byName: make(map[string]aliasEntry)}

// RegisterAliases declares pipe aliases for command paths. An empty commands
// slice registers them for every command.
//
// Four registrations are refused here rather than at use time, because each one
// produces an alias that does nothing and reports nothing:
//
//   - an alias with no name.
//   - a name a pipe operator already carries.
//   - a name a pipe filter of an overlapping command path already carries,
//     because a command's own filter resolves before anything generic.
//   - an expansion naming something other than a pipe operator, another alias
//     included.
//
// Each refusal is a panic("BUG:"). Only a registration in this repository can
// produce one, and no operator input reaches this function.
//
// Registering a command path replaces the aliases that path declared before.
// Registering globally adds to the global table.
func RegisterAliases(commands []string, aliases ...Alias) {
	set := aliasSet{byName: make(map[string]aliasEntry, len(aliases))}
	for _, alias := range aliases {
		entry := checkedAlias(commands, alias)
		set.byName[entry.alias.Name] = entry
	}

	if len(commands) == 0 {
		globalAliases.mu.Lock()
		defer globalAliases.mu.Unlock()

		maps.Copy(globalAliases.byName, set.byName)
		return
	}

	aliasRegistry.register(commands, set)
}

// PluginAlias is one alias and the command path it sits on, as a caller outside
// this repository declares it.
//
// The command path travels with the alias. One declaration message can carry
// aliases for several of the caller's commands, and the registry stores one set
// for each path.
type PluginAlias struct {
	// Command is the command path the alias sits on.
	Command string
	// Alias is the name, the description and the expansion.
	Alias Alias
}

// errAliasNoName is the refusal both entry points report for an alias with no
// name. It is the one refusal that names nothing else, because there is nothing
// to name.
var errAliasNoName = errors.New("pipe alias registered with no name")

// RegisterPluginAliases declares pipe aliases for a caller outside this
// repository. It reports a bad declaration rather than panicking on it.
//
// It refuses what RegisterAliases refuses, through the same checks. Only the
// answer differs. RegisterAliases states that no operator input reaches it, so
// a bad registration there is a Ze defect and a panic is the right report. The
// strings read here arrived over a socket. A bad one is an operating error, and
// the caller is told which declaration is wrong and why.
//
// Nothing is registered when any one declaration is refused. Every declaration
// is checked before the first is stored. A caller therefore never has to undo a
// partial registration it did not ask for.
//
// A declaration MUST name a command path. A caller outside this repository
// reaches no global alias. A global name reaches every command in the daemon,
// including commands whose answer cannot carry it.
func RegisterPluginAliases(declared []PluginAlias) error {
	byCommand := make(map[string]aliasSet, len(declared))
	paths := make([]string, 0, len(declared))

	for _, declaration := range declared {
		command := normalizeCommand(declaration.Command)
		if command == "" {
			return fmt.Errorf("pipe alias %s names no command path", declaration.Alias.Name)
		}

		entry, err := checkAlias([]string{command}, declaration.Alias)
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}

		set, seen := byCommand[command]
		if !seen {
			set = aliasSet{byName: make(map[string]aliasEntry)}
			byCommand[command] = set
			paths = append(paths, command)
		}
		set.byName[entry.alias.Name] = entry
	}

	for _, path := range paths {
		aliasRegistry.register([]string{path}, byCommand[path])
	}
	return nil
}

// checkedAlias returns the entry to store for one alias. It panics when the
// registration is one of the four RegisterAliases refuses.
func checkedAlias(commands []string, alias Alias) aliasEntry {
	entry, err := checkAlias(commands, alias)
	if err != nil {
		panic("BUG: " +
			err.Error())
	}
	return entry
}

// checkAlias returns the entry to store for one alias, or the reason the
// registration is refused. It is the one reading of the four refusals. The
// in-tree path and the plugin-facing path can therefore never disagree about
// which declarations are sound.
func checkAlias(commands []string, alias Alias) (aliasEntry, error) {
	name := strings.ToLower(strings.TrimSpace(alias.Name))
	if name == "" {
		return aliasEntry{}, errAliasNoName
	}
	if _, taken := knownPipeOps[name]; taken {
		return aliasEntry{}, fmt.Errorf("pipe alias %s is the name of a pipe operator", name)
	}
	if path, shadowed := filterShadowing(commands, name); shadowed {
		return aliasEntry{}, fmt.Errorf("pipe alias %s is the name of a pipe filter of %s", name, path)
	}

	ops := parsePipeOps(alias.Expansion)
	if len(ops) == 0 {
		return aliasEntry{}, fmt.Errorf("pipe alias %s expands to nothing", name)
	}
	for _, op := range ops {
		if op.kind != pipeUnknown {
			continue
		}
		// parsePipeOps builds an unknown op from at least one field, so the
		// first field is the word the expansion named.
		named := strings.Fields(op.arg)[0]
		if aliasRegistered(named) {
			return aliasEntry{}, fmt.Errorf(
				"pipe alias %s expands to the alias %s, and an alias may not name another alias", name, named)
		}
		return aliasEntry{}, fmt.Errorf("pipe alias %s expands to %s, which is not a pipe operator", name, named)
	}

	alias.Name = name
	return aliasEntry{alias: alias, ops: ops}, nil
}

// filterShadowing returns the command path of a registered pipe filter that
// would shadow this alias name.
//
// A command's own filter resolves before anything generic, so a command
// carrying both answers the filter and never reaches the alias. The two collide
// when their command paths overlap, which pathsOverlap decides.
func filterShadowing(commands []string, name string) (string, bool) {
	shadow := ""
	pipeFilterRegistry.each(func(path string, set pipeFilterSet) {
		if shadow != "" {
			return
		}
		if _, carries := set.byName[name]; !carries {
			return
		}
		if !pathsOverlap(commands, path) {
			return
		}
		shadow = path
	})
	return shadow, shadow != ""
}

// aliasShadowing returns the name of a registered alias that this filter set
// would shadow, and the command path that alias sits on.
//
// It is the other half of filterShadowing. Whichever of the two registrations
// runs second reports the collision, and package init order decides which one
// that is. The global table sits on no path, so it answers globalPath.
func aliasShadowing(commands []string, filters []PipeFilter) (name, path string, found bool) {
	for _, filter := range filters {
		filterName := strings.ToLower(strings.TrimSpace(filter.Name))
		if filterName == "" {
			continue
		}

		globalAliases.mu.RLock()
		_, global := globalAliases.byName[filterName]
		globalAliases.mu.RUnlock()
		if global {
			return filterName, globalPath, true
		}

		shadowed := ""
		carried := false
		aliasRegistry.each(func(aliasPath string, set aliasSet) {
			if carried {
				return
			}
			if _, carries := set.byName[filterName]; !carries {
				return
			}
			if !pathsOverlap(commands, aliasPath) {
				return
			}
			shadowed = aliasPath
			carried = true
		})
		if carried {
			return filterName, shadowed, true
		}
	}
	return "", "", false
}

// pathsOverlap reports whether a command under one of these paths can also
// resolve the other path. Resolution is by longest prefix, so two paths overlap
// when either one is a prefix of the other.
//
// An empty commands slice is the global table, which every command reaches.
func pathsOverlap(commands []string, path string) bool {
	if len(commands) == 0 {
		return true
	}
	for _, command := range commands {
		command = normalizeCommand(command)
		if command == "" {
			continue
		}
		if commandMatchesPrefix(command, path) || commandMatchesPrefix(path, command) {
			return true
		}
	}
	return false
}

// aliasRegistered reports whether the name belongs to an alias anywhere: on a
// command path, or in the global table. Registration refuses an expansion that
// names one, so this answers WHY a refusal happened. It never decides that one
// is owed.
func aliasRegistered(name string) bool {
	name = strings.ToLower(name)

	globalAliases.mu.RLock()
	_, global := globalAliases.byName[name]
	globalAliases.mu.RUnlock()
	if global {
		return true
	}

	registered := false
	aliasRegistry.each(func(_ string, set aliasSet) {
		if _, carries := set.byName[name]; carries {
			registered = true
		}
	})
	return registered
}

// lookupAlias resolves one alias name for one command. A command-specific alias
// wins over a global one of the same name, by the longest-prefix rule the
// column registry uses.
func lookupAlias(command, name string) (aliasEntry, bool) {
	name = strings.ToLower(name)

	if set, ok := aliasRegistry.lookup(command); ok {
		if entry, carries := set.byName[name]; carries {
			return entry, true
		}
	}

	globalAliases.mu.RLock()
	defer globalAliases.mu.RUnlock()

	entry, carries := globalAliases.byName[name]
	return entry, carries
}

// AliasesForCommand returns the aliases a command answers to, sorted by name.
// A command-specific alias replaces the global one of the same name, which is
// what lookupAlias answers for the same pair.
func AliasesForCommand(command string) []Alias {
	byName := make(map[string]Alias)

	globalAliases.mu.RLock()
	for name, entry := range globalAliases.byName {
		byName[name] = entry.alias
	}
	globalAliases.mu.RUnlock()

	if set, ok := aliasRegistry.lookup(command); ok {
		for name, entry := range set.byName {
			byName[name] = entry.alias
		}
	}
	if len(byName) == 0 {
		return nil
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	aliases := make([]Alias, 0, len(names))
	for _, name := range names {
		aliases = append(aliases, byName[name])
	}
	return aliases
}

// aliasSuggestions offers the alias names a command answers to, for the
// operator slot of a pipe segment. An alias takes no argument, so it has no
// sub-argument completion.
func aliasSuggestions(command string) []Suggestion {
	aliases := AliasesForCommand(command)
	if len(aliases) == 0 {
		return nil
	}
	suggestions := make([]Suggestion, 0, len(aliases))
	for _, alias := range aliases {
		suggestions = append(suggestions, Suggestion{Text: alias.Name, Description: alias.Description, Type: "pipe"})
	}
	return suggestions
}

// ResetAliasesForTest clears every registered alias, the global table included.
func ResetAliasesForTest() {
	aliasRegistry.reset()

	globalAliases.mu.Lock()
	defer globalAliases.mu.Unlock()

	globalAliases.byName = make(map[string]aliasEntry)
}
