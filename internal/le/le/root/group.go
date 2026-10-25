// Design: docs/architecture/core-design.md -- le's command groups
// Overview: leroot.go -- the registration adapter that takes a group
//
// A group is what one le command is FOR, and le has five kinds of command
// behind one name. Help renders the group as each line's purpose tag.
//
// The group is declared at the registration site rather than derived, because
// only the tool knows why it exists. registry.Meta.Section is not that field.
// It carries ze's operator taxonomy, and every le tool files under
// SectionTest. A second meaning behind one name costs the reader a guess.
//
// Several commands answer to more than one of the five, so the choice is a
// LADDER rather than a judgement. Read it from the top and stop at the first
// row that matches.
//
//	1. a committed file is its product, and its check
//	   compares that file against the tree            GroupGenerate
//	2. it runs tests, proofs, or benchmarks           GroupSuite
//	3. it acts on your working state: git, the
//	   session, job admission, scratch, a build       GroupWorkflow
//	4. it judges the tree and answers a verdict       GroupGate
//	5. nothing above, so it answers a question        GroupReport
//
// Rung 1 is about the command's PURPOSE, not about whether some verb writes.
// A gate's repair verb and its ratchet baseline write too. `tier write-baseline`
// records what the tree currently is, and `rfc reseal` re-pins an audit. Those
// files serve a verdict rather than being the point of the command, so both
// areas are gates.
//
// `mutation` exists to hold the score history, so it is rung 1. `perf`
// runs the benchmark, so it stops at rung 2. internal/le/group_test.go checks
// rungs 1 and 5, and the pre-commit population against rungs 1, 2 and 4.

package leroot

import (
	"slices"
	"sync"
)

// ungroupedTag is the purpose tag a listing shows for a command registered
// without a group: it says so rather than borrowing another group's name.
const ungroupedTag = "ungrouped"

// Group is the part of le's surface one command belongs to.
type Group string

// The five groups. A command declares exactly one.
const (
	// GroupWorkflow is typed by a person to move their own work along.
	GroupWorkflow Group = "workflow"
	// GroupGate judges the tree and answers a verdict.
	GroupGate Group = "gate"
	// GroupGenerate owns a committed artifact and can rewrite it.
	GroupGenerate Group = "generate"
	// GroupSuite runs tests, proofs, and benchmarks by name.
	GroupSuite Group = "suite"
	// GroupReport reads the tree and answers what it found, gating nothing.
	GroupReport Group = "report"
)

// groupTable is the set Register accepts, in the order the ladder above reads
// them, each with the one line `le help ai groups` prints. A group absent from
// this table is a programming error at init.
var groupTable = []struct {
	group   Group
	meaning string
}{
	{GroupWorkflow, "typed by a person to move their own work along"},
	{GroupGate, "judges the tree and answers a verdict"},
	{GroupGenerate, "owns a committed artifact and can rewrite it"},
	{GroupSuite, "runs tests, proofs, and benchmarks by name"},
	{GroupReport, "reads the tree and answers what it found, gating nothing"},
}

// groupOrder is groupTable's groups, in its order.
var groupOrder = func() []Group {
	order := make([]Group, 0, len(groupTable))
	for _, row := range groupTable {
		order = append(order, row.group)
	}
	return order
}()

// groups holds the declared group of every registered command. Registration
// runs in init() and every read happens after it, but help runs on any
// goroutine, so the map is guarded. Safe for concurrent use.
var groups struct {
	sync.RWMutex
	byName map[string]Group
}

// KnownGroup reports whether a group is one le renders.
func KnownGroup(group Group) bool {
	return slices.Contains(groupOrder, group)
}

// Groups answers the render order.
func Groups() []Group { return append([]Group(nil), groupOrder...) }

// GroupOf answers the group a command declared, and whether it declared one. A
// command registered by any route other than Register has none.
func GroupOf(name string) (Group, bool) {
	groups.RLock()
	defer groups.RUnlock()
	group, ok := groups.byName[name]
	return group, ok
}

// setGroup records one command's group. Register calls it after it has
// validated the group, so an unknown value never reaches the map.
func setGroup(name string, group Group) {
	groups.Lock()
	defer groups.Unlock()
	if groups.byName == nil {
		groups.byName = make(map[string]Group, len(groupOrder))
	}
	groups.byName[name] = group
}
