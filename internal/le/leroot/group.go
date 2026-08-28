// Design: docs/architecture/core-design.md -- le's command groups
// Overview: leroot.go -- the registration adapter that takes a group
//
// A group is what one le command is FOR, and le has five kinds of command
// behind one name. Help renders the groups in the order declared here, so a
// reader meets the commands a person types before the ones a gate runs.
//
// The group is declared at the registration site rather than derived, because
// only the tool knows why it exists. registry.Meta.Section is not that field.
// It carries ze's operator taxonomy, and every le tool files under
// SectionTest. A second meaning behind one name costs the reader a guess.

package leroot

import "sync"

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

// groupOrder is the order help renders the groups in, and the set Register
// accepts. A group absent from this table is a programming error at init.
var groupOrder = []Group{GroupWorkflow, GroupGate, GroupGenerate, GroupSuite, GroupReport}

// GroupTitle answers the heading a group prints under. The parenthetical says
// who runs the command, which is the fact that picks the group. An unknown
// group has no title, and that empty answer is what KnownGroup refuses.
func GroupTitle(group Group) string {
	switch group {
	case GroupWorkflow:
		return "Workflow (you type these while working)"
	case GroupGate:
		return "Gates (judge the tree, answer a verdict)"
	case GroupGenerate:
		return "Generated artifacts (check one, or rewrite it)"
	case GroupSuite:
		return "Suites (tests, proofs and benchmarks, by name)"
	case GroupReport:
		return "Reports (read the tree, gate nothing)"
	default:
		return ""
	}
}

// groups holds the declared group of every registered command. Registration
// runs in init() and every read happens after it, but help runs on any
// goroutine, so the map is guarded. Safe for concurrent use.
var groups struct {
	sync.RWMutex
	byName map[string]Group
}

// KnownGroup reports whether a group is one le renders.
func KnownGroup(group Group) bool {
	return GroupTitle(group) != ""
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
