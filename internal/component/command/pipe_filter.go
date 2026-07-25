// Design: docs/architecture/api/commands.md — command-owned pipe filters
// Related: pipe.go — generic pipe parsing and formatting
// Related: completer.go — pipe completion

package command

import (
	"sort"
	"strings"
	"sync"

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

var pipeFilterRegistry = struct {
	sync.RWMutex
	byCommand map[string]pipeFilterSet
}{byCommand: make(map[string]pipeFilterSet)}

// RegisterPipeFilters registers command-specific pipe filters for command paths.
// Passing no filters registers the command as having no command-specific pipes,
// which prevents a shorter filtered command prefix from matching it.
func RegisterPipeFilters(commands []string, filters ...PipeFilter) {
	pipeFilterRegistry.Lock()
	defer pipeFilterRegistry.Unlock()

	set := pipeFilterSet{filters: append([]PipeFilter(nil), filters...), byName: make(map[string]PipeFilter, len(filters))}
	for i := range filters {
		name := strings.ToLower(strings.TrimSpace(filters[i].Name))
		if name == "" {
			continue
		}
		set.filters[i].Name = name
		set.byName[name] = set.filters[i]
	}
	for _, command := range commands {
		command = normalizeCommand(command)
		if command == "" {
			continue
		}
		pipeFilterRegistry.byCommand[command] = set
	}
}

func lookupPipeFilters(command string) (pipeFilterSet, bool) {
	command = normalizeCommand(command)
	pipeFilterRegistry.RLock()
	defer pipeFilterRegistry.RUnlock()

	var best pipeFilterSet
	bestLen := -1
	found := false
	for prefix, set := range pipeFilterRegistry.byCommand {
		if !commandMatchesPrefix(command, prefix) {
			continue
		}
		if len(prefix) > bestLen {
			best = set
			bestLen = len(prefix)
			found = true
		}
	}
	return best, found
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

func normalizeCommand(command string) string {
	return strings.ToLower(textbuf.Join(strings.Fields(command), " "))
}

func commandMatchesPrefix(command, prefix string) bool {
	return command == prefix || strings.HasPrefix(command, prefix+" ")
}

func ResetPipeFiltersForTest() {
	pipeFilterRegistry.Lock()
	defer pipeFilterRegistry.Unlock()
	pipeFilterRegistry.byCommand = make(map[string]pipeFilterSet)
}
