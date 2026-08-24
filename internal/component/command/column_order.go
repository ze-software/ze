// Design: docs/architecture/api/commands.md — per-command column order
// Related: pipe_filter.go — the other user of the command-path registry
// Related: pipe_table.go — the renderer that reads the declared order

package command

import (
	"fmt"
	"reflect"
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
var columnRegistry = newDeclarationRegistry("column order",
	func(orders []ColumnOrder) bool { return len(orders) == 0 })

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
//
// Two packages declaring two DIFFERENT orders for one command path is a Ze
// defect and panics, and an empty declaration never overrides a non-empty one
// (declarationRegistry.declare).
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
	columnRegistry.declare(commands, stored)
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

// commandRegistry maps a command path to one value and resolves a command to
// the value registered on the longest matching path.
//
// A command with no declaration of its own inherits the nearest registered
// ancestor's. A command that MUST NOT inherit registers an empty declaration of
// its own. That is why an empty declaration is stored rather than dropped:
// absent and empty are different answers.
//
// register stores what its caller computed, so the alias registry is its one
// user: RegisterPluginAliases reads what a path holds, merges the caller's
// aliases into it, and stores the result, and UnregisterPluginAliases stores
// what survives a removal. A stored value that differs from the held one is the
// point there rather than a defect. A registry holding ONE package's
// declaration about a path embeds this one and adds declare below.
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

// declarationRegistry holds ONE declaration for each command path: what a
// package states that path's answer holds. The shape, column-order,
// address-field and pipe-filter registries are the four.
//
// It differs from commandRegistry in the collision rule alone, and the reason
// is where the value comes from. A declaration is written by hand in one
// package, so two packages writing two different ones for a path is a
// disagreement neither of them can win. A stored value is computed from what
// the path already holds, so a difference is expected.
type declarationRegistry[T any] struct {
	commandRegistry[T]

	// name is what a conflict report calls this registry, in a message a person
	// reads: "answer shape", "column order".
	name string

	// isEmpty reports whether a value declares nothing, which makes it a floor
	// rather than a claim. T is not uniformly a slice, because a pipe-filter set
	// is a struct, so each registry states its own emptiness rather than this
	// one testing it generically.
	isEmpty func(value T) bool
}

func newDeclarationRegistry[T any](name string, isEmpty func(value T) bool) *declarationRegistry[T] {
	return &declarationRegistry[T]{
		commandRegistry: commandRegistry[T]{byCommand: make(map[string]T)},
		name:            name,
		isEmpty:         isEmpty,
	}
}

// declare records what one package states a command path holds, and refuses two
// packages stating two different things about one path.
//
// Four cases, and the reason for each:
//
//   - The path holds nothing. Store the value.
//   - The value is EMPTY and the path holds something. Keep what it holds. An
//     empty declaration is a FLOOR: it stops a shorter path being inherited and
//     says nothing about what the answer holds, so it never overrides a value
//     that does say. The BGP peer command plugin blanks every direct child of
//     `show bgp`, and the rib command plugin declares `tab` for `show bgp rib`.
//     Package initialization order decided that path's answer before this rule.
//   - The two values are equal. A package restating its own declaration is a
//     no-op, so equality decides rather than identity.
//   - The two values differ and neither is empty. Panic. Every caller declares
//     from init(), so only a Ze defect reaches this state, which is what
//     panic("BUG:") is for (docs/contributing/ze-style.md). Nothing outside this
//     repository reaches a declaration registry: a caller's aliases are merged
//     into the alias registry, which stores rather than declares, and a bad
//     declaration there is answered with an error.
//
// reflect.DeepEqual reads every value type these four registries hold, which is
// why each one states its emptiness and none states its equality. Declaration
// runs at init alone, so the cost is paid once for each path.
func (r *declarationRegistry[T]) declare(commands []string, value T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	empty := r.isEmpty(value)
	for _, command := range commands {
		command = normalizeCommand(command)
		if command == "" {
			continue
		}
		held, found := r.byCommand[command]
		if !found {
			r.byCommand[command] = value
			continue
		}
		if empty {
			continue
		}
		if r.isEmpty(held) {
			r.byCommand[command] = value
			continue
		}
		if reflect.DeepEqual(held, value) {
			continue
		}
		// The message is built with textbuf and joined over two lines, which is
		// the form checkedAlias (alias.go) uses and for the same two reasons:
		// fmt.Sprintf is a banned format primitive here
		// (ai/rules/performance.md), and the panic argument has to OPEN with the
		// "BUG:" literal. Rendering the two values is fmt.Sprint's job, since T
		// is any and a buffer cannot serialize an unknown type.
		var tb textbuf.Buffer
		tb.Str("two packages declare a different ").Str(r.name).
			Str(" for ").Quoted(command).Str(": ").
			Str(fmt.Sprint(held)).Str(" and ").Str(fmt.Sprint(value))
		panic("BUG: " +
			tb.String())
	}
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
