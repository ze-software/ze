// Design: docs/architecture/api/commands.md — pipe aliases
// Related: pipe.go — the operators an alias expands into, and where it expands
// Related: pipe_filter.go — the command-owned filters an alias must not shadow
// Related: column_order.go — the command-path registry the per-command table reuses

package command

import (
	"errors"
	"fmt"
	"maps"
	"slices"
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
//
// It is the one registry that STORES rather than DECLARES, so the collision
// rule the other four carry does not reach it (declarationRegistry.declare in
// column_order.go). Two declarations of one path do not collide here: an alias
// set MERGES, which is what mergedAliases below does, so what this registry is
// asked to store is already the answer to the collision. The rule would also be
// reachable from outside this repository through RegisterPluginAliases, where a
// bad declaration is an operating error owed an error rather than a panic.
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

// pluginAliasPath is what one owner put on one command path: the names it added
// there, and whether the path carried no declaration at all before it
// registered.
//
// Created is what makes removal exact. A path the owner found undeclared is the
// owner's own, and it leaves when the owner leaves. A path that already carried
// a declaration keeps that declaration, because it belongs to somebody else:
// `show bgp rpki` carries the empty declaration the in-tree BGP command plugin
// puts on every child of `show bgp`, and removing the path would remove that
// barrier with it.
type pluginAliasPath struct {
	names   []string
	created bool
}

// pluginAliases records what each owner put into the alias registry.
//
// The registry stores one set for each command path, and one path holds the
// aliases of more than one owner, so what an owner registered cannot be read
// back out of the registry alone. This record is what removal reverses.
//
// The mutex also orders the read-modify-write that registration and removal
// both run: each one reads what a path holds, changes it, and writes it back.
//
// Safe for concurrent use.
var pluginAliases = struct {
	mu      sync.Mutex
	byOwner map[string]map[string]pluginAliasPath
}{byOwner: make(map[string]map[string]pluginAliasPath)}

// RegisterPluginAliases declares pipe aliases for a caller outside this
// repository. It reports a bad declaration rather than panicking on it.
//
// It refuses what RegisterAliases refuses, through the same checks. Only the
// answer differs. RegisterAliases states that no operator input reaches it, so
// a bad registration there is a Ze defect and a panic is the right report. The
// strings read here arrived over a socket. A bad one is an operating error, and
// the caller is told which declaration is wrong and why.
//
// Two more registrations are refused here than RegisterAliases refuses, and
// both are refusals an in-tree caller does not owe:
//
//   - a name the EXACT command path already carries, whoever registered it.
//     RegisterAliases replaces what a path declared before, which is how a
//     package restates its own table. A caller outside this repository holds no
//     such table, so a replacement there takes a name a command already answers
//     to.
//   - one name declared twice on one path in one message. The message is built
//     into one set for each path, so the later of the two would win in silence.
//
// Nothing is registered when any one declaration is refused. Every declaration
// is checked before the first is stored. A caller therefore never has to undo a
// partial registration it did not ask for.
//
// A declaration MUST name a command path. A caller outside this repository
// reaches no global alias. A global name reaches every command in the daemon,
// including commands whose answer cannot carry it.
//
// The owner is the name UnregisterPluginAliases takes back what this call
// registered under. It is opaque: nothing else in this package reads it.
//
// The commands are every command path the owner declares, and aliasBarriers
// reads them to stop a declared alias reaching a command below the one it sits
// on. They are not an authorization check. Whether an owner may name a path is
// decided before the declaration arrives here.
func RegisterPluginAliases(owner string, commands []string, declared []PluginAlias) error {
	pluginAliases.mu.Lock()
	defer pluginAliases.mu.Unlock()

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
		if _, twice := set.byName[entry.alias.Name]; twice {
			return fmt.Errorf("%s: pipe alias %s is declared twice in one message", command, entry.alias.Name)
		}
		if aliasOnPath(command, entry.alias.Name) {
			return fmt.Errorf("%s: pipe alias %s is already registered on that command path", command, entry.alias.Name)
		}
		set.byName[entry.alias.Name] = entry
	}

	put := pluginAliases.byOwner[owner]
	if put == nil {
		put = make(map[string]pluginAliasPath, len(paths))
	}
	for _, path := range paths {
		_, occupied := aliasRegistry.get(path)
		aliasRegistry.register([]string{path}, mergedAliases(path, byCommand[path]))

		record := put[path]
		record.created = record.created || !occupied
		for name := range byCommand[path].byName {
			record.names = append(record.names, name)
		}
		put[path] = record
	}
	for _, path := range aliasBarriers(commands, paths) {
		aliasRegistry.register([]string{path}, aliasSet{byName: make(map[string]aliasEntry)})

		record := put[path]
		record.created = true
		put[path] = record
	}
	pluginAliases.byOwner[owner] = put
	return nil
}

// UnregisterPluginAliases removes every alias the owner declared, and every
// barrier that declaration derived. It removes nothing else.
//
// Removal is by ENTRY. One command path holds the aliases of more than one
// owner, and a path a caller declares on carries the in-tree declarations
// beside them, so removing the PATH would take another owner's names with it. A
// path this owner registered from nothing is the one exception: it goes once
// the last of its entries is gone, because the barrier it also became is the
// owner's.
//
// It reports nothing. A caller tears an owner down without knowing whether that
// owner ever reached this registry, so an owner that declared no alias is not
// an error.
func UnregisterPluginAliases(owner string) {
	pluginAliases.mu.Lock()
	defer pluginAliases.mu.Unlock()

	put, registered := pluginAliases.byOwner[owner]
	if !registered {
		return
	}
	delete(pluginAliases.byOwner, owner)

	for path, record := range put {
		set, declared := aliasRegistry.get(path)
		if !declared {
			continue
		}

		kept := aliasSet{byName: make(map[string]aliasEntry, len(set.byName))}
		for name, entry := range set.byName {
			if slices.Contains(record.names, name) {
				continue
			}
			kept.byName[name] = entry
		}

		if record.created && len(kept.byName) == 0 {
			aliasRegistry.remove(path)
			continue
		}
		aliasRegistry.register([]string{path}, kept)
	}
}

// aliasBarriers returns the command paths that MUST carry an empty declaration,
// so a declared alias stops at the command it was declared for.
//
// lookupAlias resolves the LONGEST registered command path that is a prefix of
// the command, and never falls back to a shorter one. A command that declares
// nothing therefore answers the nearest declared ancestor's aliases. An owner
// declaring `show x` and `show x rows`, with an alias on `show x`, would have
// `show x rows` answer that alias over a payload that cannot carry it.
//
// The barrier is the empty declaration that stops the inheritance, and it is
// derived from the owner's own command list. Knowing the resolution rule is not
// a thing a declaring author owes.
//
// A path that already carries a declaration needs no barrier and gets none. It
// stops the inheritance already, and the declaration is somebody's to keep.
func aliasBarriers(commands, aliasPaths []string) []string {
	barriers := make([]string, 0, len(commands))
	for _, command := range commands {
		path := normalizeCommand(command)
		if path == "" {
			continue
		}
		if _, declared := aliasRegistry.get(path); declared {
			continue
		}
		for _, aliasPath := range aliasPaths {
			if path == aliasPath || !commandMatchesPrefix(path, aliasPath) {
				continue
			}
			barriers = append(barriers, path)
			break
		}
	}
	return barriers
}

// aliasOnPath reports whether the EXACT command path already carries this alias
// name, whoever registered it.
//
// The population is that one path and nothing above it. lookupAlias reads the
// set on the longest registered prefix and never falls back to a shorter one,
// so an alias of this name on a shorter path, or in the global table, is
// SHADOWED for this path rather than in conflict with it. That shadowing is the
// mechanism a longer path uses to answer a name of its own, and it is what lets
// `show bgp rpki` carry `summary` while `show bgp` carries one.
func aliasOnPath(command, name string) bool {
	set, registered := aliasRegistry.get(command)
	if !registered {
		return false
	}
	_, carries := set.byName[name]
	return carries
}

// mergedAliases returns what a command path holds once the declared set is
// added to what it already held.
//
// A registration MUST NOT drop an alias another registration put on the same
// path. `show bgp rpki` already carries the empty declaration the in-tree BGP
// command plugin puts on every child of `show bgp`, so a declaring caller
// registers onto an occupied path in the ordinary case. Every declared name is
// checked against the same set before this runs, so nothing merged here
// replaces anything.
func mergedAliases(command string, declared aliasSet) aliasSet {
	current, registered := aliasRegistry.get(command)
	if !registered {
		return declared
	}

	merged := aliasSet{byName: make(map[string]aliasEntry, len(current.byName)+len(declared.byName))}
	maps.Copy(merged.byName, current.byName)
	maps.Copy(merged.byName, declared.byName)
	return merged
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
		suggestions = append(suggestions, Suggestion{Text: alias.Name, Description: alias.Description, Type: SuggestionPipe})
	}
	return suggestions
}

// ResetAliasesForTest clears every registered alias, the global table and the
// record of what each owner declared included. A record that survived the
// registry it describes would remove another test's entries.
func ResetAliasesForTest() {
	aliasRegistry.reset()

	pluginAliases.mu.Lock()
	pluginAliases.byOwner = make(map[string]map[string]pluginAliasPath)
	pluginAliases.mu.Unlock()

	globalAliases.mu.Lock()
	defer globalAliases.mu.Unlock()

	globalAliases.byName = make(map[string]aliasEntry)
}
