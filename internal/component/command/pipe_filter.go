// Design: docs/architecture/api/commands.md — command-owned pipe filters
// Related: pipe.go — generic pipe parsing and formatting
// Related: completer.go — pipe completion

package command

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PipeFilter declares a pipe segment accepted by a specific command.
// Generic pipes such as match, json, table, and resolve do not use this type.
type PipeFilter struct {
	Name        string
	Description string
	TakesArg    bool
	Leading     bool
}

type pipeFilterSet struct {
	filters []PipeFilter
	byName  map[string]PipeFilter
}

// pipeFilterRegistry resolves a command to its filter set by the longest
// registered command path, the same lookup the column-order registry uses.
var pipeFilterRegistry = newDeclarationRegistry("pipe filter set",
	func(set pipeFilterSet) bool { return len(set.filters) == 0 })

// RegisterPipeFilters registers command-specific pipe filters for command paths.
// Passing no filters registers the command as having no command-specific pipes,
// which prevents a shorter filtered command prefix from matching it.
//
// A filter whose name a pipe alias of an overlapping command path already
// carries is refused with a panic("BUG:") naming both. The filter would resolve
// first and the alias would never be reached, which nothing reports at use
// time. RegisterAliases refuses the same pair from the other side, so init
// order decides which of the two reports it.
//
// Two packages declaring two DIFFERENT filter sets for one command path is
// refused the same way, and an empty declaration never overrides a non-empty
// one (declarationRegistry.declare).
func RegisterPipeFilters(commands []string, filters ...PipeFilter) {
	if name, path, shadowed := aliasShadowing(commands, filters); shadowed {
		panic("BUG: pipe filter " +
			name +
			" is the name of a pipe alias of " +
			path)
	}

	set := pipeFilterSet{filters: append([]PipeFilter(nil), filters...), byName: make(map[string]PipeFilter, len(filters))}
	for i := range filters {
		name := strings.ToLower(strings.TrimSpace(filters[i].Name))
		if name == "" {
			continue
		}
		set.filters[i].Name = name
		set.byName[name] = set.filters[i]
	}
	pipeFilterRegistry.declare(commands, set)
}

func lookupPipeFilters(command string) (pipeFilterSet, bool) {
	return pipeFilterRegistry.lookup(command)
}

// PipeFiltersForCommand returns command-specific pipe filters registered for a command.
func PipeFiltersForCommand(command string) []PipeFilter {
	set, ok := lookupPipeFilters(command)
	if !ok || len(set.filters) == 0 {
		return nil
	}
	filters := append([]PipeFilter(nil), set.filters...)
	sort.SliceStable(filters, func(i, j int) bool { return filters[i].Name < filters[j].Name })
	return filters
}

// String names the filters in the set, so a conflict report between two
// declarations of one command path reads as the names an operator types.
func (set pipeFilterSet) String() string { return set.filterNames() }

func (set pipeFilterSet) filterNames() string {
	if len(set.filters) == 0 {
		return ""
	}
	names := make([]string, 0, len(set.filters))
	for _, filter := range set.filters {
		if filter.Name != "" {
			names = append(names, filter.Name)
		}
	}
	sort.Strings(names)
	return textbuf.Join(names, ", ")
}

func filterSuggestions(command string) []Suggestion {
	set, ok := lookupPipeFilters(command)
	if !ok || len(set.filters) == 0 {
		return nil
	}
	items := append([]PipeFilter(nil), set.filters...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	suggestions := make([]Suggestion, 0, len(items))
	for _, filter := range items {
		suggestions = append(suggestions, Suggestion{Text: filter.Name, Description: filter.Description, Type: "pipe"})
	}
	return suggestions
}

func ResetPipeFiltersForTest() {
	pipeFilterRegistry.reset()
}
