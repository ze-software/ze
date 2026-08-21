// Design: docs/architecture/api/commands.md — per-command column order
// Related: pipe_filter.go — the other user of the command-path registry
// Related: pipe_table.go — the renderer that reads the declared order

package command

import (
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ColumnOrder is the column order of one record shape: the JSON keys of that
// record, in the order a person reads them.
//
// A command that renders more than one record shape declares one ColumnOrder
// per shape. `show bgp` renders an outer record and a list of peer
// rows. Both carry an "uptime" key in a different position, so one flat list
// cannot state both orders.
type ColumnOrder []string

// columnRegistry holds the column orders each command declares.
var columnRegistry = newCommandRegistry[[]ColumnOrder]()

// RegisterColumns declares the column order the table and text renderers use
// for command paths. Every name is a JSON key of the command's answer,
// verbatim, in lowercase kebab-case (ai/rules/cli.md).
//
// Rendering for a person is the only consumer. `| json`, `| ndjson` and
// `| yaml` keep their alphabetical keys, because a program reads them and
// column order carries no meaning for a program (owner directive, 2026-08-19).
//
// A declared name the answer does not carry is inert, and a key no order names
// still renders, after the declared ones, alphabetically. Ordering never adds a
// column and never drops one.
//
// Passing no order registers the command as declaring none, which stops a
// shorter registered command path from ordering it. RegisterPipeFilters uses
// the same convention for the same reason.
func RegisterColumns(commands []string, orders ...ColumnOrder) {
	stored := make([]ColumnOrder, 0, len(orders))
	for _, order := range orders {
		normalized := make(ColumnOrder, 0, len(order))
		for _, name := range order {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			normalized = append(normalized, name)
		}
		if len(normalized) == 0 {
			continue
		}
		stored = append(stored, normalized)
	}
	columnRegistry.register(commands, stored)
}

// ColumnsForCommand returns the column orders declared for a command, resolved
// by the longest registered command path that is a prefix of it. It returns nil
// when the command declares none, and the renderer then orders every column
// alphabetically.
func ColumnsForCommand(command string) []ColumnOrder {
	orders, ok := columnRegistry.lookup(command)
	if !ok {
		return nil
	}
	return orders
}

// ResetColumnsForTest clears every declared column order.
func ResetColumnsForTest() {
	columnRegistry.reset()
}

// commandRegistry maps a command path to one declaration and resolves a command
// to the declaration registered on the longest matching path.
//
// A command with no declaration of its own inherits the nearest registered
// ancestor's. A command that MUST NOT inherit registers an empty declaration of
// its own. That is why an empty declaration is stored rather than dropped:
// absent and empty are different answers.
type commandRegistry[T any] struct {
	mu        sync.RWMutex
	byCommand map[string]T
}

func newCommandRegistry[T any]() *commandRegistry[T] {
	return &commandRegistry[T]{byCommand: make(map[string]T)}
}

func (r *commandRegistry[T]) register(commands []string, value T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, command := range commands {
		command = normalizeCommand(command)
		if command == "" {
			continue
		}
		r.byCommand[command] = value
	}
}

func (r *commandRegistry[T]) lookup(command string) (T, bool) {
	command = normalizeCommand(command)

	r.mu.RLock()
	defer r.mu.RUnlock()

	var best T
	bestLen := -1
	found := false
	for prefix, value := range r.byCommand {
		if !commandMatchesPrefix(command, prefix) {
			continue
		}
		if len(prefix) > bestLen {
			best = value
			bestLen = len(prefix)
			found = true
		}
	}
	return best, found
}

// each calls visit once for every registered command path, in map order. The
// read lock is held for the walk, so visit MUST NOT call back into this
// registry.
//
// Registration reads the other registries through this. It refuses a name that
// two of them would carry, because such a name reaches nobody.
func (r *commandRegistry[T]) each(visit func(command string, value T)) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for command, value := range r.byCommand {
		visit(command, value)
	}
}

func (r *commandRegistry[T]) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byCommand = make(map[string]T)
}

func normalizeCommand(command string) string {
	return strings.ToLower(textbuf.Join(strings.Fields(command), " "))
}

// commandMatchesPrefix reports whether prefix is the command itself or an
// ancestor path of it. The word boundary stops "show bgp rib" from matching
// "show bgp ribbon".
func commandMatchesPrefix(command, prefix string) bool {
	if command == prefix {
		return true
	}
	if len(command) <= len(prefix) || command[len(prefix)] != ' ' {
		return false
	}
	return strings.HasPrefix(command, prefix)
}
