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

// remove drops what one command path declares, so the path is undeclared again
// and a command under it inherits from the nearest declared ancestor.
//
// It is not the reverse of register for a path that held a declaration before:
// register stores one value for each path, so what the path held is already
// gone. The one caller reads what the path holds, decides that nothing of it
// survives, and removes the path in place of writing an empty declaration.
// Empty and absent are different answers, and only absent restores the
// inheritance the declaration stopped.
func (r *commandRegistry[T]) remove(command string) {
	command = normalizeCommand(command)

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.byCommand, command)
}

// get returns what one command path declares itself, and nothing an ancestor
// declares. It answers a different question from lookup: lookup resolves a
// COMMAND to the nearest registered ancestor's declaration, and this reports
// what the PATH holds.
//
// A caller that must not read an inherited answer reads this one. Registration
// does, because a declaration on a shorter path is shadowed for this path
// rather than in conflict with it.
func (r *commandRegistry[T]) get(command string) (T, bool) {
	command = normalizeCommand(command)

	r.mu.RLock()
	defer r.mu.RUnlock()

	value, found := r.byCommand[command]
	return value, found
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
