package config

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// The tests of this file pin the child order of a block
// (docs/architecture/config/syntax.md, "Child order"): YANG definition order,
// with `uses`, `choice` and `augment` children at the positions the page
// states. Every fixture module under testdata/yang-order declares its
// children in an order that is not their name order, so a converter that
// sorts by name answers an order the fixture text does not hold.

// orderFixtureSchema answers the config schema schemaFromResolved builds over
// the fixture modules of testdata/yang-order named by files, loaded in the
// order given.
func orderFixtureSchema(t *testing.T, files ...string) *Schema {
	t.Helper()
	loader := yang.NewLoader()
	for _, file := range files {
		require.NoError(t, loader.AddModuleFromFile(filepath.Join("testdata", "yang-order", file)))
	}
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	schema, err := schemaFromResolved(resolved)
	require.NoError(t, err)
	return schema
}

// childLister is the part of a container or list node these tests read.
type childLister interface {
	Children() []string
}

// rootChildren answers the Children() of the top-level node name.
func rootChildren(t *testing.T, schema *Schema, name string) []string {
	t.Helper()
	node, ok := schema.Get(name).(childLister)
	require.True(t, ok, "%s must be a container or a list", name)
	return node.Children()
}

// TestContainerChildrenDefinitionOrder: a container's children follow the
// order its module declares them in, and so does what Serialize prints.
// Method: a container declaring remote, local, timers, against name order
// local, remote, timers.
//
// VALIDATES: AC-1.
// PREVENTS: every printer listing a block in an order nobody wrote.
func TestContainerChildrenDefinitionOrder(t *testing.T) {
	schema := orderFixtureSchema(t, "order-groupings.yang", "order-blocks-conf.yang")

	assert.Equal(t, []string{"remote", "local", "timers"}, rootChildren(t, schema, "direct"))

	tree, err := NewParser(schema).Parse("direct { local l; timers { hold 3; } remote r; }")
	require.NoError(t, err)
	printed := Serialize(tree, schema)
	remote := strings.Index(printed, "remote r")
	local := strings.Index(printed, "local l")
	timers := strings.Index(printed, "timers")
	require.NotEqual(t, -1, remote, printed)
	require.NotEqual(t, -1, local, printed)
	require.NotEqual(t, -1, timers, printed)
	assert.Less(t, remote, local, "remote is declared before local:\n%s", printed)
	assert.Less(t, local, timers, "local is declared before timers:\n%s", printed)
}

// TestUsesChildrenAtUsesPosition: the children a `uses` brings sit at the
// `uses` statement's position, in the grouping's own order, for a grouping
// that uses another and for a grouping imported from another module.
// Method: leaves declared around each `uses`, every grouping declaring its
// leaves against name order.
//
// VALIDATES: AC-2, AC-3.
// PREVENTS: grouping children sorted by name, or sent after the block's own.
func TestUsesChildrenAtUsesPosition(t *testing.T) {
	schema := orderFixtureSchema(t, "order-groupings.yang", "order-blocks-conf.yang")

	assert.Equal(t, []string{"a", "z", "y", "b"}, rootChildren(t, schema, "used"))
	assert.Equal(t, []string{"a", "o2", "i2", "i1", "o1", "m2", "m1", "b"},
		rootChildren(t, schema, "nested"))
}

// TestUsesAugmentChildrenAfterTheTargetsOwn: the children an `augment`
// written inside a `uses` adds to a node of the grouping follow that node's
// own children, augment by augment in source order, each in its statement
// order; the node itself keeps the `uses` statement's position in its block.
// Method: a `uses` whose two augments of one container declare u2, u1 and
// then a0, against a name order that would put a0 first.
//
// PREVENTS: a child added by a uses-local augment having no place, which
// refuses the schema build, or sorting by name among the node's children.
func TestUsesAugmentChildrenAfterTheTargetsOwn(t *testing.T) {
	schema := orderFixtureSchema(t, "order-uses-augment-conf.yang")

	assert.Equal(t, []string{"z", "box", "a"}, rootChildren(t, schema, "holder"))
	holder, ok := schema.Get("holder").(*ContainerNode)
	require.True(t, ok, "holder must be a container")
	box, ok := holder.Get("box").(childLister)
	require.True(t, ok, "box must be a container")
	assert.Equal(t, []string{"g2", "g1", "u2", "u1", "a0"}, box.Children())
}

// TestChoiceCasesDefinitionOrder: a choice's data nodes sit at the choice
// statement's position, its cases in source order and each case's nodes in
// source order, in a container and in a list, for a shorthand case and for a
// choice nested in a case. Method: case k2 declared before case k1.
//
// VALIDATES: AC-4.
// PREVENTS: case order sorted by name (R-7).
func TestChoiceCasesDefinitionOrder(t *testing.T) {
	schema := orderFixtureSchema(t, "order-groupings.yang", "order-blocks-conf.yang")

	assert.Equal(t, []string{"a", "q", "p", "b"}, rootChildren(t, schema, "chosen"))
	assert.Equal(t, []string{"a", "q", "p", "b"}, rootChildren(t, schema, "chosen-list"))
	assert.Equal(t, []string{"a", "s2", "k1", "n2", "n1", "s1", "b"},
		rootChildren(t, schema, "shorthand"))
}

// TestAugmentedChildrenByModuleThenSource: children other modules add through
// `augment` follow the block's own, ordered by augmenting module name, then
// by augment statement order within the module, then by each augment's own
// statement order, a `uses` inside an augment at its position. Method: module
// order-augmenting-alpha augments x twice and xl once with names that sort
// after order-augmenting-zulu's, and zulu is loaded first.
//
// VALIDATES: AC-5, A-5.
// PREVENTS: augmented children sorted by name, or by load order.
func TestAugmentedChildrenByModuleThenSource(t *testing.T) {
	schema := orderFixtureSchema(t,
		"order-augmenting-zulu.yang", "order-augmenting-alpha.yang", "order-augmented-conf.yang")

	assert.Equal(t, []string{"x2", "x1", "m9", "m8", "m2", "m3", "m1", "a0"},
		rootChildren(t, schema, "x"))
	assert.Equal(t, []string{"l2", "l1", "la", "a0"}, rootChildren(t, schema, "xl"))
}

// TestMergedTopLevelDefinitionOrder: two -conf modules defining one top-level
// container give it the first module's children, by module name, then the
// other's; the root lists modules by name and each module's roots in
// definition order. Method: order-merge-alpha-conf defines zeta then shared,
// order-merge-beta-conf defines shared then alpha.
//
// VALIDATES: AC-6.
// PREVENTS: a root or a shared container printed in name order (R-10).
func TestMergedTopLevelDefinitionOrder(t *testing.T) {
	schema := orderFixtureSchema(t, "order-merge-beta-conf.yang", "order-merge-alpha-conf.yang")

	assert.Equal(t, []string{"zeta", "shared", "alpha"}, schema.Children())
	assert.Equal(t, []string{"s9", "s8", "s5", "s4"}, rootChildren(t, schema, "shared"))
}

// TestMergedTopLevelDefinitionOrderThroughInclude: the top-level nodes a
// submodule declares sit at the module's `include` statement. Method: the
// include between two roots whose names sort against their source order.
//
// VALIDATES: AC-6 for a module split into submodules.
// PREVENTS: a submodule's roots refused as unplaced, or sorted by name.
func TestMergedTopLevelDefinitionOrderThroughInclude(t *testing.T) {
	schema := orderFixtureSchema(t, "order-included-part.yang", "order-included-conf.yang")

	assert.Equal(t, []string{"zz-first", "mm-sub", "aa-last"}, schema.Children())
}

// TestDefinitionOrderStableAcrossModuleSets: the children two module sets
// share keep their relative order, whatever the set holds and whatever order
// its modules load in, as two builds with different feature tags or external
// plugins must print one tree alike. Method: the augment fixtures loaded in
// two orders, and without each augmenting module.
//
// VALIDATES: AC-11, A-4.
// PREVENTS: an order taken from registration, load or map iteration order
// (R-6).
func TestDefinitionOrderStableAcrossModuleSets(t *testing.T) {
	full := orderFixtureSchema(t,
		"order-augmenting-zulu.yang", "order-augmenting-alpha.yang", "order-augmented-conf.yang")
	reloaded := orderFixtureSchema(t,
		"order-augmented-conf.yang", "order-augmenting-alpha.yang", "order-augmenting-zulu.yang")
	withoutAlpha := orderFixtureSchema(t, "order-augmented-conf.yang", "order-augmenting-zulu.yang")
	withoutZulu := orderFixtureSchema(t, "order-augmenting-alpha.yang", "order-augmented-conf.yang")

	for _, name := range []string{"x", "xl"} {
		want := rootChildren(t, full, name)
		assert.Equal(t, want, rootChildren(t, reloaded, name), "%s across load orders", name)
		for label, subset := range map[string]*Schema{"without alpha": withoutAlpha, "without zulu": withoutZulu} {
			got := rootChildren(t, subset, name)
			shared := slices.DeleteFunc(slices.Clone(want), func(child string) bool {
				return !slices.Contains(got, child)
			})
			assert.Equal(t, shared, got, "%s %s keeps the full set's relative order", name, label)
		}
	}
}

// TestEveryChildHasDefinitionPosition: every child of every node of the full
// registered schema has a definition position. Method: build the schema of
// every module this binary registers; a child the order cannot place fails
// the build with an error naming its path.
//
// VALIDATES: AC-14, A-2.
// PREVENTS: a child goyang adds that the order walk does not model.
func TestEveryChildHasDefinitionPosition(t *testing.T) {
	_, err := YANGSchema()
	require.NoError(t, err)
}

// TestDefinitionPositionRefusesUnplacedChild: a child no statement places is
// refused at schema build, by name, and never printed at a guessed position.
// Method: a container entry built with no statement, holding one leaf.
//
// VALIDATES: AC-14, the refusal TestEveryChildHasDefinitionPosition relies on.
// PREVENTS: an unplaced child falling back to name order in silence.
func TestDefinitionPositionRefusesUnplacedChild(t *testing.T) {
	resetSchemaBuildErrors()
	block := &gyang.Entry{Name: "block", Kind: gyang.DirectoryEntry, Dir: map[string]*gyang.Entry{}}
	block.Dir["lost"] = &gyang.Entry{
		Name: "lost", Kind: gyang.LeafEntry, Parent: block, Type: &gyang.YangType{Kind: gyang.Ystring},
	}

	container := yangToContainer(block, "block")
	err := flushSchemaBuildErrors()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"lost" has no definition position`)
	assert.Empty(t, container.Children())
}

// TestPositionalListOrderChangesRoundTrip: a positional one-line entry
// assigns its values in definition order, a grouping's child at the `uses`
// position, and its printed form reads back to the same tree. Method: list
// positional declares `uses first-value` before leaf after.
//
// VALIDATES: AC-10, D-6.
// PREVENTS: a positional line read into the leaves in another order than the
// printer writes it.
func TestPositionalListOrderChangesRoundTrip(t *testing.T) {
	schema := orderFixtureSchema(t, "order-groupings.yang", "order-blocks-conf.yang")
	require.Equal(t, []string{"first", "after"}, rootChildren(t, schema, "positional"))

	tree, err := NewParser(schema).Parse("positional { k1 one two; }")
	require.NoError(t, err)
	entry := tree.GetList("positional")["k1"]
	require.NotNil(t, entry)
	first, ok := entry.Get("first")
	require.True(t, ok, "the first value lands on the grouping's leaf first")
	after, ok := entry.Get("after")
	require.True(t, ok, "the last value lands on leaf after")
	assert.Equal(t, "one", first)
	assert.Equal(t, "two", after)

	printed := Serialize(tree, schema)
	reparsed, err := NewParser(schema).Parse(printed)
	require.NoError(t, err, printed)
	assert.Equal(t, tree.ToMap(), reparsed.ToMap(), printed)
	assert.Equal(t, printed, Serialize(reparsed, schema))
}

// positionalMeaningChangesReviewed names every shipped list whose positional
// one-line entry means something else under the definition order than under
// the rule before it, each with the reason the change is accepted, and with
// what a line written for the old order does now: refused, because a value
// lands on a child whose type rejects it, or accepted with another meaning,
// with the line and both readings. Each value is judged against the type of
// the child it lands on (validateInlineListChildValue, from
// parseListInlineEntry and setInlineLastChild). A list whose positional
// meaning changes and is not named here fails
// TestPositionalListOrderChangesInShippedLists; so does a name here whose
// meaning no longer changes, so the list cannot go stale.
//
// Census of 2026-11-18 for every entry below: a search of test/, examples/,
// docs/, etc/, website/ and internal/ (testdata and Go strings included) for
// an entry of the list written `<list> <key> <value> ...` found no Ze config
// writing a positional line for any of them, so no file changes meaning.
var positionalMeaningChangesReviewed = map[string]string{
	"interface/bridge": "uses interface-l2 brings interface-common at its position: mtu " +
		"and os-name now sit between description and disable, where name order put " +
		"disable first. Refused: an old line's fourth value was for disable (type " +
		"empty, which takes only true) and now lands on mtu (uint16 68..16000), which " +
		"refuses true (bridge br0 true eth1 lan true;); a line of three values or " +
		"fewer reads the same",
	"interface/dummy": "uses interface-l2 brings interface-common at its position: " +
		"description, mtu, os-name, disable in the grouping's order instead of " +
		"description, disable by name. Refused: an old line's second value was for " +
		"disable (type empty, which takes only true) and now lands on mtu (uint16 " +
		"68..16000), which refuses true (dummy dum0 lab true;); a one-value line " +
		"reads the same",
	"interface/ethernet": "uses interface-l2 brings interface-common at its position: " +
		"description, mtu, os-name, disable in the grouping's order instead of " +
		"description, disable by name. Refused: an old line's second value was for " +
		"disable (type empty, which takes only true) and now lands on mtu (uint16 " +
		"68..16000), which refuses true (ethernet eth0 uplink true;); a one-value " +
		"line reads the same",
	"interface/pppoe-client": "uses interface-common is the list's last statement, so " +
		"the last child is now the grouping's last leaf (disable) where name order " +
		"ended on os-name. Only a bracket reaches the last child: authentication, a " +
		"container, refuses a second plain value in both orders. Refused: " +
		"pppoe-client ppp0 eth0 [ wan0 ]; gave os-name the string \"[ wan0 ]\" and " +
		"now gives it to disable (type empty), which takes only true",
	"interface/veth": "uses interface-l2 brings interface-common at its position: mtu " +
		"and os-name now sit between description and disable, where name order put " +
		"disable first. Refused: an old line's third value was for disable (type " +
		"empty, which takes only true) and now lands on mtu (uint16 68..16000), which " +
		"refuses true (veth veth0 veth1 lab true;); a line of two values or fewer " +
		"reads the same",
	"interface/xfrm": "uses interface-common at its position, after if-id and dev: " +
		"disable moves after mtu and os-name, as the grouping declares it, where name " +
		"order put it before them. Refused: an old line's fourth value was for disable " +
		"(type empty, which takes only true) and now lands on mtu (uint16 68..16000), " +
		"which refuses true (xfrm xfrm0 7 eth0 ipsec true 1400 xfrm7;); a line of " +
		"three values or fewer reads the same",
	"bus/hub/server": "uses zt:listener is declared before secret and certificate, " +
		"so ip and port take the first two values, where the old rule put the list's " +
		"own leaves first (secret, certificate, then the client list, which ends the " +
		"slots) and the grouping's by name after them; old last:port took only a " +
		"bracket, which port's uint16 refused. Refused for every usable old line but " +
		"one shape: its secret (32 characters at least, minTokenLength) now lands on " +
		"ip (zt:ip-address), and its certificate name on port (uint16). Accepted with " +
		"another meaning when the secret is itself an IPv6 literal of 32 to 39 " +
		"characters and the certificate, if written, a number up to 65535: server " +
		"main 2001:0db8:0000:0000:0000:0000:0000:0001 443; read secret " +
		"2001:0db8:0000:0000:0000:0000:0000:0001 and certificate 443, and now reads " +
		"ip 2001:0db8:0000:0000:0000:0000:0000:0001 and port 443 with no secret. " +
		"server main 10.0.0.1 1790; parses in both orders, but its old reading " +
		"(secret 10.0.0.1) was refused at load as shorter than 32 characters",
	"environment/looking-glass/server": "uses zt:listener is declared before tls, " +
		"so ip, port, last:tls take the slots where the old rule put the list's own " +
		"leaf first (tls, ip, last:port). Refused for every usable old line: its " +
		"first value was tls (boolean true/false) and now lands on ip (zt:ip-address), " +
		"which refuses booleans; a two-value line's second value (ip) lands on port " +
		"(uint16), which refuses an IP address. Census of 2026-11-25 confirmed no repo " +
		"config writes a positional server line under environment/looking-glass",
	"policy/match": "uses route-source-conditions sits at its position, the list's " +
		"last statement, so the last child is now the grouping's last leaf " +
		"(route-type) where name order ended on tag. Only a bracket reaches the last " +
		"child: metric, a container, refuses a fifth plain value in both orders, and " +
		"the slots action, family, next-hop, interface are unchanged. Refused: match " +
		"m1 [ 100 ]; gave tag the member 100 and now gives it to route-type, whose " +
		"enumeration refuses it. That is the schema this test loads; a build with " +
		"the BGP filter plugins places their augments of match after the grouping, by " +
		"module name, so it ends on ze-filter-prefix's prefix-list, a string leaf: " +
		"there the same line parses as prefix-list \"[ 100 ]\", and commit refuses it " +
		"as naming no prefix-list (prefixListReference)",
}

// TestPositionalListOrderChangesInShippedLists: every shipped keyed list,
// all of which accept the positional one-line entry (parseListInlineEntry),
// sends each value of such a line to the same child under the definition
// order as under the rule it replaced, or is named, with its reason, in
// positionalMeaningChangesReviewed. Method: walk the full registered schema
// beside the goyang entries that define it, rebuild each list's previous
// order from that rule (previousListOrder), and compare the slots a
// positional line reaches in each order (positionalSlots).
//
// VALIDATES: A-1, R-3, D-6.
// PREVENTS: a positional line silently read into other leaves, after this
// change or after a later YANG edit, with no list reviewed.
func TestPositionalListOrderChangesInShippedLists(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)
	resolved, err := yang.DefaultLoader()
	require.NoError(t, err)
	var roots []*gyang.Entry
	for _, name := range resolved.ConfModuleNames() {
		if entry := resolved.GetEntry(name); entry != nil {
			roots = append(roots, entry)
		}
	}

	changed := map[string]string{}
	for _, name := range schema.Children() {
		positionalMeaningChanges(schema.Get(name), definersOf(roots, name), name, changed)
	}
	// A reviewed list the YANG declares for another system is not judged here:
	// pppoe-client and xfrm carry ze:os "linux", so the schema of any other
	// system prunes them (yangToNode) and holds no list to compare. Only that
	// case is skipped, a path Lookup misses and LookupDeclared still resolves.
	// A reviewed path no module declares at all stays expected, so a list
	// renamed or removed fails here and the table cannot go stale.
	var reviewed []string
	for _, path := range slices.Sorted(maps.Keys(positionalMeaningChangesReviewed)) {
		if prunedForThisSystem(schema, path) {
			continue
		}
		reviewed = append(reviewed, path)
	}
	require.NotEmpty(t, reviewed, "no reviewed list is in this schema, so the comparison would prove nothing")
	assert.Equal(t, reviewed, slices.Sorted(maps.Keys(changed)),
		"lists whose positional meaning changes (slots before -> after): %v", changed)
}

// prunedForThisSystem reports whether path names a node the YANG declares and
// this system's schema left out (ze:os): Lookup misses it and LookupDeclared
// resolves it. A path neither resolves is not pruned, it is absent.
func prunedForThisSystem(schema *Schema, path string) bool {
	if _, err := schema.Lookup(path); err == nil {
		return false
	}
	_, err := schema.LookupDeclared(path)
	return err == nil
}

// positionalMeaningChanges records, under its path, every keyed list at or
// under node whose positional slots differ between the previous order and
// Children(). definers are the goyang entries that define node (more than one
// for a root two modules merge). Recursion is bounded by the schema's depth.
func positionalMeaningChanges(node Node, definers []*gyang.Entry, path string, changed map[string]string) {
	lister, ok := node.(interface {
		childLister
		Get(name string) Node
	})
	if !ok {
		return
	}
	if list, isList := node.(*ListNode); isList && list.KeyName != "" && len(definers) > 0 {
		before := positionalSlots(list, previousListOrder(definers[0], list))
		after := positionalSlots(list, list.Children())
		if !slices.Equal(before, after) {
			changed[path] = strings.Join(before, " ") + " -> " + strings.Join(after, " ")
		}
	}
	for _, name := range lister.Children() {
		positionalMeaningChanges(lister.Get(name), definersOf(definers, name), AppendPath(path, name), changed)
	}
}

// positionalSlots answers the children the values of a positional line land
// on, for a list whose children come in order, as parseListInlineEntry
// assigns them: one value to each child before the last while each takes a
// value (inlineSlotTakesValue; the first that does not refuses the line), and
// the rest, or a bracket, to the last child, marked "last:", when it takes a
// value. Two orders with equal slots read every positional line alike.
func positionalSlots(list *ListNode, order []string) []string {
	if len(order) == 0 {
		return nil
	}
	var slots []string
	for _, name := range order[:len(order)-1] {
		if !inlineSlotTakesValue(list.Get(name)) {
			break
		}
		slots = append(slots, name)
	}
	if last := order[len(order)-1]; inlineSlotTakesValue(list.Get(last)) {
		slots = append(slots, "last:"+last)
	}
	return slots
}

// previousListOrder answers the child order the converter gave a list before
// the definition order, rebuilt from that rule (yangChildOrder and
// flattenChoiceCases, deleted with it): the leaf, leaf-list, container, list,
// choice and anyxml statements of the list's own statement in source order,
// then every other child (from a `uses` or an `augment`) by name; the key
// left out; a choice expanded case by case by name, each case's nodes by name.
// Children the schema pruned (ze:os) are left out, as Children() leaves them.
func previousListOrder(entry *gyang.Entry, list *ListNode) []string {
	var names []string
	seen := map[string]bool{}
	if entry.Node != nil {
		for _, sub := range entry.Node.Statement().SubStatements() {
			if !slices.Contains([]string{"leaf", "leaf-list", "container", "list", "choice", "anyxml"}, sub.Keyword) {
				continue
			}
			if seen[sub.Argument] || entry.Dir[sub.Argument] == nil {
				continue
			}
			seen[sub.Argument] = true
			names = append(names, sub.Argument)
		}
	}
	var rest []string
	for name := range entry.Dir {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	slices.Sort(rest)
	var order []string
	for _, name := range append(names, rest...) {
		if name == entry.Key {
			continue
		}
		if child := entry.Dir[name]; child.IsChoice() {
			order = append(order, previousChoiceOrder(child)...)
			continue
		}
		order = append(order, name)
	}
	return slices.DeleteFunc(order, func(name string) bool { return list.Get(name) == nil })
}

// previousChoiceOrder answers the data nodes of a choice as the deleted
// flattenChoiceCases did: cases by name, each case's nodes by name, a nested
// choice the same way.
func previousChoiceOrder(choice *gyang.Entry) []string {
	var order []string
	for _, caseName := range slices.Sorted(maps.Keys(choice.Dir)) {
		member := choice.Dir[caseName]
		if !member.IsCase() {
			order = append(order, caseName)
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(member.Dir)) {
			if inner := member.Dir[name]; inner.IsChoice() {
				order = append(order, previousChoiceOrder(inner)...)
				continue
			}
			order = append(order, name)
		}
	}
	return order
}

// definersOf answers, for each block of blocks holding a child name, that
// child's entry, looking through choices and cases as the schema flattens
// them; a block that holds none is skipped.
func definersOf(blocks []*gyang.Entry, name string) []*gyang.Entry {
	var out []*gyang.Entry
	for _, block := range blocks {
		if child := childEntry(block, name); child != nil {
			out = append(out, child)
		}
	}
	return out
}

// childEntry answers the data node name of block, a direct child or one
// inside a case of one of its choices.
func childEntry(block *gyang.Entry, name string) *gyang.Entry {
	if child := block.Dir[name]; child != nil && !child.IsChoice() {
		return child
	}
	for _, member := range block.Dir {
		if !member.IsChoice() {
			continue
		}
		for _, inCase := range member.Dir {
			if inCase.IsCase() {
				if child := childEntry(inCase, name); child != nil {
					return child
				}
				continue
			}
			if inCase.Name == name {
				return inCase
			}
		}
	}
	return nil
}
