// Design: docs/architecture/config/syntax.md — the child order of a block
// Related: yang_schema.go — the converter that builds every node in this order
//
// The order of a block's children, which every printer of the configuration
// follows because it ranges the Children() the converter built.

package config

import (
	"cmp"
	"fmt"
	"slices"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// namedChild pairs a child entry with its effective name under the parent
// node. A choice and its cases are transparent at the data layer, so a data
// node inside a case is named as a child of the block that holds the choice.
// The case is not lost: the entry keeps its parent chain through the case and
// the choice, and the converter records it (choiceMembers.record) so
// ApplyDefaults can tell which case is in effect.
type namedChild struct {
	name  string
	entry *gyang.Entry
}

// definitionOrder answers the data children of the block entry in YANG
// definition order (docs/architecture/config/syntax.md, "Child order"), every
// choice and case flattened to the data nodes they hold:
//
//   - a child declared in the block takes its statement's position;
//   - a `uses` is expanded at its position, in the grouping's own order, and a
//     `uses` inside the grouping the same way;
//   - a `choice` is expanded at its position, its cases in source order and
//     each case's nodes in source order;
//   - the children an `augment` added follow the block's own, augment by
//     augment in augmentsInOrder's order, each in its statement order.
//
// goyang's Entry.Dir is a map, so the order is rebuilt from the statements the
// entries keep: goyang leaves Entry.Uses empty unless ParseOptions.StoreUses is
// set, which Ze never sets, and a child merged through `uses` keeps the Node
// of its statement inside the grouping. Every fact the order reads is module
// text (statement order, module names), so two module sets order the children
// they share the same way, whatever was registered or joined.
//
// A child the walk cannot place, one no statement of the block, of a grouping
// it uses or of an augment of it declares, has no position to print at. It is
// recorded as a schema build error naming its path, and left out: the build
// that fails on it answers no schema at all.
func definitionOrder(entry *gyang.Entry, path string) []namedChild {
	walk := childOrderWalk{placed: make(map[*gyang.Entry]bool, len(entry.Dir))}
	walk.block(entry)
	unplaced := walk.unplaced(entry, nil)
	slices.Sort(unplaced)
	for _, name := range unplaced {
		recordSchemaBuildError(fmt.Errorf(
			"at %s: YANG node %q has no definition position: no statement of its block, "+
				"of a grouping the block uses or of an augment of the block declares it, "+
				"so the printers have no place for it; report the module that adds it, "+
				"since every node goyang builds from module text has one",
			AppendPath(path, name), name))
	}
	return walk.children
}

// childOrderWalk collects the children of one block in definition order. Not
// safe for concurrent use; each definitionOrder call owns its own.
type childOrderWalk struct {
	children []namedChild
	placed   map[*gyang.Entry]bool
	included map[*gyang.Module]bool // The submodules already walked.
}

// block places the children of a block (a module, a container, a list or a
// case): its own statements, then the statements of each augment of it.
// Recursion through nested choices is bounded by the nesting depth of the
// module text, which Ze ships or an operator installs, never a peer.
func (w *childOrderWalk) block(holder *gyang.Entry) {
	if holder.Node == nil {
		return // No statement to read: every child is reported unplaced.
	}
	w.statements(holder.Node.Statement(), holder.Node, holder)
	for _, augment := range augmentsInOrder(holder) {
		w.statements(augment.Node.Statement(), augment.Node, holder)
	}
}

// statements places, in source order, the children of holder that the
// substatements of stmt declare. scope is the node stmt belongs to: a `uses`
// resolves its grouping from it, as goyang's ToEntry resolved it from the
// `uses` node whose parent scope is. Recursion through `uses` is bounded by
// the grouping nesting of the module text: goyang refuses a grouping it
// cannot expand before the converter runs.
func (w *childOrderWalk) statements(stmt *gyang.Statement, scope gyang.Node, holder *gyang.Entry) {
	for _, sub := range stmt.SubStatements() {
		switch sub.Keyword {
		case "uses":
			grouping := gyang.FindGrouping(scope, sub.Argument, map[string]bool{})
			if grouping == nil {
				continue // goyang refused the module set already: unknown group.
			}
			w.statements(grouping.Statement(), grouping, holder)
		case "include":
			w.include(scope, sub.Argument, holder)
		case "choice":
			w.choice(holder.Dir[sub.Argument])
		case yangStatementLeaf, "leaf-list", "container", "list",
			"anydata", "anyxml", "action", "notification", "rpc":
			w.place(sub.Argument, holder.Dir[sub.Argument])
		}
	}
}

// include places, at the `include` statement's position, the top-level
// children the submodule name declares: goyang merges a submodule's data nodes
// into its module's entry, while their statements sit in the submodule's
// text. Each submodule is walked once, so an include cycle ends.
func (w *childOrderWalk) include(scope gyang.Node, name string, holder *gyang.Entry) {
	module, ok := scope.(*gyang.Module)
	if !ok {
		return // `include` is a module statement; elsewhere it names nothing.
	}
	for _, included := range module.Include {
		if included.Name != name {
			continue
		}
		if included.Module == nil {
			continue // goyang refused the module set already: unknown submodule.
		}
		if w.included[included.Module] {
			continue
		}
		if w.included == nil {
			w.included = map[*gyang.Module]bool{}
		}
		w.included[included.Module] = true
		w.statements(included.Module.Statement(), included.Module, holder)
	}
}

// choice places the data nodes of a choice: its own cases in source order,
// then the cases each augment of it added.
func (w *childOrderWalk) choice(choice *gyang.Entry) {
	if choice == nil {
		return // A choice the module set removed (deviation, feature).
	}
	if choice.Node == nil {
		return // No statement to read: its nodes are reported unplaced.
	}
	w.choiceStatements(choice.Node.Statement(), choice)
	for _, augment := range augmentsInOrder(choice) {
		w.choiceStatements(augment.Node.Statement(), choice)
	}
}

// choiceStatements places the nodes of the cases the substatements of stmt
// declare in choice, each case in source order. A data statement directly
// under the choice is a shorthand case of its own.
func (w *childOrderWalk) choiceStatements(stmt *gyang.Statement, choice *gyang.Entry) {
	for _, sub := range stmt.SubStatements() {
		member := choice.Dir[sub.Argument]
		if member == nil {
			continue // Not a case of this choice, or one the module set removed.
		}
		switch sub.Keyword {
		case "case":
			w.block(member)
		case "choice":
			w.choice(shorthandMember(member, sub.Argument))
		case yangStatementLeaf, "leaf-list", "container", "list", "anydata", "anyxml":
			w.place(sub.Argument, shorthandMember(member, sub.Argument))
		}
	}
}

// shorthandMember answers the node a shorthand case holds. Once goyang has
// processed the module set it wraps each one in a case of the same name
// (Entry.FixChoice), and before that the node sits in the choice directly.
func shorthandMember(member *gyang.Entry, name string) *gyang.Entry {
	if member.IsCase() {
		return member.Dir[name]
	}
	return member
}

// place appends child under name, once. A statement whose entry the module
// set removed (a deviation, a disabled feature) has no child to place.
func (w *childOrderWalk) place(name string, child *gyang.Entry) {
	if child == nil {
		return
	}
	if w.placed[child] {
		return
	}
	w.placed[child] = true
	w.children = append(w.children, namedChild{name: name, entry: child})
}

// unplaced appends to out the name of every child of holder the walk placed
// no statement for, descending through choices and cases as the walk does.
func (w *childOrderWalk) unplaced(holder *gyang.Entry, out []string) []string {
	for name, child := range holder.Dir {
		if child == nil {
			continue
		}
		if child.IsChoice() {
			out = w.unplacedOfChoice(child, out)
			continue
		}
		if !w.placed[child] {
			out = append(out, name)
		}
	}
	return out
}

// unplacedOfChoice appends the unplaced data nodes of each case of choice.
func (w *childOrderWalk) unplacedOfChoice(choice *gyang.Entry, out []string) []string {
	for name, member := range choice.Dir {
		if member == nil {
			continue
		}
		if member.IsCase() {
			out = w.unplaced(member, out)
			continue
		}
		if member.IsChoice() {
			out = w.unplacedOfChoice(member, out)
			continue
		}
		if !w.placed[member] {
			out = append(out, name)
		}
	}
	return out
}

// augmentsInOrder answers the augments goyang applied to target, ordered by
// augmentKey. goyang appends to Entry.Augmented one copy of each applied
// augment, whose Node is the `augment` statement, in its own processing
// order, which follows a map iteration (Modules.Process): that order is never
// used. An augment with no statement cannot be walked; the children it added
// are reported unplaced.
func augmentsInOrder(target *gyang.Entry) []*gyang.Entry {
	augments := make([]*gyang.Entry, 0, len(target.Augmented))
	keys := make(map[*gyang.Entry]augmentKey, len(target.Augmented))
	for _, augment := range target.Augmented {
		if augment.Node == nil {
			continue
		}
		augments = append(augments, augment)
		keys[augment] = augmentKeyOf(augment.Node)
	}
	slices.SortFunc(augments, func(a, b *gyang.Entry) int {
		return keys[a].compare(keys[b])
	})
	return augments
}

// augmentKey is the position of an `augment` statement in module text: the
// module that wrote it, then the module or submodule whose text holds it,
// then its place in that text.
type augmentKey struct {
	module   string // The augmenting module; a submodule's belongs-to module.
	source   string // The module or submodule whose text holds the augment.
	position []int  // statementPosition of the augment in that text.
}

// compare orders two keys by module name, then source name, then position.
func (k augmentKey) compare(other augmentKey) int {
	if c := cmp.Compare(k.module, other.module); c != 0 {
		return c
	}
	if c := cmp.Compare(k.source, other.source); c != 0 {
		return c
	}
	return slices.Compare(k.position, other.position)
}

// augmentKeyOf answers the key of the augment statement node. Its root is the
// module or submodule holding it (yang.RootNode), and a submodule's augment
// belongs to the module it names in `belongs-to`, as goyang's own module()
// answers. A node with no root module has empty names, which sort first:
// goyang builds every augment from a module, so it reaches here from no text.
func augmentKeyOf(augment gyang.Node) augmentKey {
	key := augmentKey{position: statementPosition(augment)}
	root := gyang.RootNode(augment)
	if root == nil {
		return key
	}
	key.source = root.Name
	key.module = root.Name
	if root.Kind() == "submodule" && root.BelongsTo != nil {
		key.module = root.BelongsTo.Name
	}
	return key
}

// statementPosition answers the index of each statement on the path from the
// root of n's text down to n, so two statements of one text compare in source
// order. Bounded by the nesting depth of the module text.
func statementPosition(n gyang.Node) []int {
	var reversed []int
	for parent := n.ParentNode(); parent != nil; n, parent = parent, parent.ParentNode() {
		reversed = append(reversed, slices.Index(parent.Statement().SubStatements(), n.Statement()))
	}
	slices.Reverse(reversed)
	return reversed
}
