// Design: docs/architecture/api/commands.md — command-owned pipe filters
// Related: pipe.go — generic pipe parsing and formatting
// Related: pipe_partition.go — the one partition that reads this registry
// Related: completer.go — pipe completion

package command

import (
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PipeFilter declares a pipe operation a specific command runs in its own
// handler: the engine hands it over in the declared list rather than running a
// generic operator over the answer. Generic operators such as json, table and
// resolve are not declared with this type; a declaration whose name is an
// overridable generic's (count, first, last, match) is an override, held to the
// generic's argument contract and answer shape.
type PipeFilter struct {
	Name        string
	Description string
	// Argument is the operation's argument contract, one of the closed
	// argument kinds of the catalog (ArgNone, ArgOptional, ArgText, ArgCount,
	// ArgFields, ArgPath). The partition validates the typed words against it
	// before the handler runs. An override MUST state the generic's own kind.
	Argument PipeArgKind
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
// A filter whose name is a generic operator's is an override. It is refused
// the same way when the catalog does not let a command override that operator
// (declaredFilterRefusal), or when its argument contract is not the generic's, and
// so is an argument kind outside the closed vocabulary. Each is a Ze defect in
// the declaration, found when it is recorded rather than when an operator
// first types the name.
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
		if refusal := declaredFilterRefusal(commands, set.filters[i]); refusal != "" {
			panic("BUG: " + refusal)
		}
		set.byName[name] = set.filters[i]
	}
	pipeFilterRegistry.declare(commands, set)
}

// declaredFilterRefusal answers why a declared operation cannot be recorded,
// or "" when it can: its argument kind is outside the closed vocabulary, or
// its name is a generic operator's that the catalog does not let a command
// override, or that it overrides with another argument contract. filter.Name
// is already lowercased.
func declaredFilterRefusal(commands []string, filter PipeFilter) string {
	var tb textbuf.Buffer
	paths := textbuf.Join(commands, ", ")
	if !filter.Argument.declarable() {
		return tb.Str("pipe filter ").Str(filter.Name).Str(" of ").Str(paths).
			Str(" states an argument kind outside none, optional, text, count, fields and path").String()
	}
	generic, isGeneric := lookupPipeOperatorByName(filter.Name)
	if !isGeneric {
		return ""
	}
	if !generic.Overridable() {
		return tb.Str("pipe filter ").Str(filter.Name).Str(" of ").Str(paths).
			Str(" is the generic pipe operator ").Str(generic.Name).
			Str(", which a command cannot override; give the operation a name of its own").String()
	}
	if filter.Argument != generic.Arg {
		return tb.Str("pipe filter ").Str(filter.Name).Str(" of ").Str(paths).
			Str(" overrides the generic ").Str(generic.Name).Str(" with argument ").Str(filter.Argument.String()).
			Str(", but the generic takes ").Str(generic.Arg.String()).String()
	}
	return ""
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
	return sortedPipeFilters(append([]PipeFilter(nil), set.filters...))
}

// sortedPipeFilters sorts filters by name in place and answers them, the
// order every published list of a command's operations takes.
func sortedPipeFilters(filters []PipeFilter) []PipeFilter {
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
	slices.Sort(names)
	return textbuf.Join(names, ", ")
}

func ResetPipeFiltersForTest() {
	pipeFilterRegistry.reset()
}

// EmptyPipeFiltersForTest empties the pipe-filter registry for one test and
// answers the function that restores what it held, which the test MUST call
// when it ends, typically through t.Cleanup. Unlike ResetPipeFiltersForTest it
// gives back the declarations init() made, so a later test of the same binary
// that links their package (plugin/all) still reads them. Not safe for
// concurrent use with another test that empties or resets the registry.
func EmptyPipeFiltersForTest() (restore func()) {
	return pipeFilterRegistry.emptyForTest()
}
