//go:build ze_bgp && ze_cos

package all

import (
	"maps"
	"slices"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// The guard of this file runs where every module is registered, because a
// list's positional slots depend on modules the config package's own tests
// never link: a plugin that augments a list adds children at its end, and a
// second -conf module that defines a list merges its children in front of or
// behind the first module's (docs/architecture/config/syntax.md, "Child
// order"). Its build constraint names the two feature tags the reviewed table
// below depends on: ze_cos merges its copy of the interface lists ethernet,
// dummy, veth and bridge, and ze_bgp carries the five filter plugins that
// augment policy match. Every other entry comes from an always-built module.

// positionalMeaningChangeReview is the reviewed truth for one list whose
// positional one-line entry means something else under the definition order
// than under the rule before it: the slots a positional line reached before
// and reaches now, as positionalSlots spells them, and why the change is
// accepted, with what a line written for the old order does now.
type positionalMeaningChangeReview struct {
	slots  string
	reason string
}

// positionalMeaningChangesReviewed names every list of the full build whose
// positional one-line entry means something else under the definition order
// than under the rule before it. Each reason says what a line written for the
// old order does now: refused, because a value lands on a child whose type
// rejects it or that holds no value of its own, or accepted with another
// meaning, with the line and both readings. Each value is judged against the
// child it lands on (refuseInlineSlot and validateInlineListChildValue, from
// parseListInlineEntry and setInlineLastChild). A list whose slots change and
// is not named here fails TestPositionalListOrderChangesInFullBuild, and so
// does a name here whose slots no longer change or now change differently, so
// the table cannot go stale.
//
// Census of 2026-11-18 for every entry below: a search of test/, examples/,
// docs/, etc/, website/ and internal/ (testdata and Go strings included) for
// an entry of the list written `<list> <key> <value> ...` found no Ze config
// writing a positional line for any of them, so no file changes meaning. The
// hits were set-form paths (`set interface ethernet eth9 unit 0 ...`), which
// address each value by keyword, and other daemons' configs.
var positionalMeaningChangesReviewed = map[string]positionalMeaningChangeReview{
	"interface/bridge": {
		slots: "last:os-name -> class-of-service",
		reason: "ze-cos-conf defines bridge too and merges first, by module name, so the " +
			"list starts with cos's children: before, its own list unit, then " +
			"class-of-service from uses cos-interface-leaves by name; now " +
			"class-of-service, then unit, in source order. The interface module's " +
			"children follow, the last being os-name by name before and the container " +
			"offload in its grouping's order now. Refused: before, every plain value " +
			"landed on unit, a list, and only a bracket parsed, given to os-name " +
			"(bridge br0 [ br-lan ]; set os-name \"[ br-lan ]\"); the bracket now lands " +
			"on offload, which holds no value of its own. A line of one plain value, " +
			"refused before, now sets class-of-service (bridge br0 gold;)",
	},
	"interface/dummy": {
		slots: "last:os-name -> class-of-service",
		reason: "ze-cos-conf defines dummy too and merges first, by module name: " +
			"class-of-service now precedes cos's list unit, which the old rule put " +
			"first, and the interface module's children follow, ending on os-name " +
			"by name before and on the container offload in its grouping's order " +
			"now. Refused: before, every plain value landed on unit, a list, and only " +
			"a bracket parsed, given to os-name (dummy dum0 [ lab ];); the bracket now " +
			"lands on offload, which holds no value of its own. A line of one plain " +
			"value, refused before, now sets class-of-service (dummy dum0 gold;)",
	},
	"interface/ethernet": {
		slots: "last:os-name -> class-of-service",
		reason: "ze-cos-conf defines ethernet too and merges first, by module name: " +
			"class-of-service now precedes cos's list unit, which the old rule put " +
			"first, and the interface module's children follow, ending on os-name " +
			"by name before and on the container offload in its grouping's order " +
			"now. Refused: before, every plain value landed on unit, a list, and only " +
			"a bracket parsed, given to os-name (ethernet eth0 [ uplink ];); the " +
			"bracket now lands on offload, which holds no value of its own. A line of " +
			"one plain value, refused before, now sets class-of-service (ethernet " +
			"eth0 gold;)",
	},
	"interface/pppoe-client": {
		slots: "source-interface last:os-name -> source-interface last:disable",
		reason: "uses interface-common is the list's last statement, so the last child " +
			"is now the grouping's last leaf (disable) where name order ended on " +
			"os-name. Only a bracket reaches the last child: authentication, a " +
			"container, refuses a second plain value in both orders. Refused: " +
			"pppoe-client ppp0 eth0 [ wan0 ]; gave os-name the string \"[ wan0 ]\" and " +
			"now gives it to disable (type empty), which takes only true",
	},
	"interface/veth": {
		slots: "last:os-name -> class-of-service",
		reason: "ze-cos-conf defines veth too and merges first, by module name: " +
			"class-of-service now precedes cos's list unit, which the old rule put " +
			"first, and the interface module's children follow (peer first), ending " +
			"on os-name by name before and on the container offload in its " +
			"grouping's order now. Refused: before, every plain value landed on unit, " +
			"a list, and only a bracket parsed, given to os-name (veth veth0 [ lab ];); " +
			"the bracket now lands on offload, which holds no value of its own. A line " +
			"of one plain value, refused before, now sets class-of-service (veth veth0 " +
			"gold;)",
	},
	"interface/xfrm": {
		slots: "if-id dev description disable mtu os-name -> if-id dev description mtu os-name disable",
		reason: "uses interface-common at its position, after if-id and dev: disable " +
			"moves after mtu and os-name, as the grouping declares it, where name " +
			"order put it before them. Refused: an old line's fourth value was for " +
			"disable (type empty, which takes only true) and now lands on mtu (uint16 " +
			"68..16000), which refuses true (xfrm xfrm0 7 eth0 ipsec true 1400 " +
			"xfrm7;); a line of three values or fewer reads the same",
	},
	"bus/hub/server": {
		slots: "secret certificate last:port -> ip port secret certificate",
		reason: "uses zt:listener is declared before secret and certificate, so ip and " +
			"port take the first two values, where the old rule put the list's own " +
			"leaves first (secret, certificate, then the client list, which ends the " +
			"slots) and the grouping's by name after them; old last:port took only a " +
			"bracket, which port's uint16 refused. Refused for every usable old line " +
			"but one shape: its secret (32 characters at least, minTokenLength) now " +
			"lands on ip (zt:ip-address), and its certificate name on port (uint16). " +
			"Accepted with another meaning when the secret is itself an IPv6 literal " +
			"of 32 to 39 characters and the certificate, if written, a number up to " +
			"65535: server main 2001:0db8:0000:0000:0000:0000:0000:0001 443; read " +
			"secret 2001:0db8:0000:0000:0000:0000:0000:0001 and certificate 443, and " +
			"now reads ip 2001:0db8:0000:0000:0000:0000:0000:0001 and port 443 with no " +
			"secret. server main 10.0.0.1 1790; parses in both orders, but its old " +
			"reading (secret 10.0.0.1) was refused at load as shorter than 32 " +
			"characters",
	},
	"policy/match": {
		slots: "action family next-hop interface last:tag -> action family next-hop interface last:prefix-list",
		reason: "uses route-source-conditions is the list's last statement, and the " +
			"five BGP filter plugins augment match after it, by module name " +
			"(ze-filter-aspath, ze-filter-aspath-length, ze-filter-community-match, " +
			"ze-filter-path-asn, ze-filter-prefix), so the last child is now " +
			"ze-filter-prefix's prefix-list, a string leaf, where name order ended on " +
			"the leaf-list tag. Only a bracket reaches the last child: metric, a " +
			"container, refuses a fifth plain value in both orders, and the slots " +
			"action, family, next-hop, interface are unchanged. Accepted with another " +
			"meaning by the parse, refused at commit: match m1 [ 100 ]; gave tag the " +
			"member 100 and now gives prefix-list the text \"[ 100 ]\", which commit " +
			"refuses as naming no prefix-list (prefixListReference) unless a " +
			"prefix-list carries that name",
	},
	"static/table/route": {
		slots: "description metric distance tag blackhole last:reject -> description metric distance tag last:reject",
		reason: "choice action declares case forward (container next), then discard " +
			"(the presence container blackhole), then unreachable (the presence " +
			"container reject); the old rule expanded the cases by name, so blackhole " +
			"took the fifth value and next ended the slots. Under the definition order " +
			"next comes fifth and ends the slots after tag; reject is the last child in " +
			"both orders. Refused: an old line's fifth value set blackhole (route " +
			"10.0.0.0/8 lab 10 1 7 drop;) and now lands on next, a container, which " +
			"holds no value of its own; a line of four values or fewer, or one whose " +
			"bracket goes to reject, reads the same",
	},
}

// TestPositionalListOrderChangesInFullBuild: every keyed list of the full
// build, all of which accept the positional one-line entry
// (parseListInlineEntry), sends each value of such a line to the same child
// under the definition order as under the rule it replaced, or is named in
// positionalMeaningChangesReviewed with exactly the slots it changes. Method:
// walk the schema of every registered module beside the goyang entries that
// define it, rebuild each list's previous order from that rule
// (previousListOrder, across every module that defines the list), and compare
// the slots a positional line reaches in each order (positionalSlots), asking
// the parser which children take a value (slotProbe).
//
// VALIDATES: A-1, R-3, D-6 of spec-config-yang-definition-order, over the
// modules a full build carries.
// PREVENTS: a positional line silently read into other leaves, after this
// change or after a later YANG edit or a new augmenting plugin, with no list
// reviewed.
func TestPositionalListOrderChangesInFullBuild(t *testing.T) {
	probe := newSlotProbe(t)
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}
	resolved, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	var roots []*gyang.Entry
	for _, name := range resolved.ConfModuleNames() {
		if entry := resolved.GetEntry(name); entry != nil {
			roots = append(roots, entry)
		}
	}

	changed := map[string]string{}
	for _, name := range schema.Children() {
		probe.meaningChanges(schema.Get(name), definersOf(roots, name), name, changed)
	}
	for _, path := range slices.Sorted(maps.Keys(changed)) {
		review, named := positionalMeaningChangesReviewed[path]
		if !named {
			t.Errorf("%s: positional slots change (%s) and positionalMeaningChangesReviewed does not name it", path, changed[path])
			continue
		}
		if review.slots != changed[path] {
			t.Errorf("%s: positional slots change as %q, the review states %q", path, changed[path], review.slots)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(positionalMeaningChangesReviewed)) {
		// A reviewed list the YANG declares for another system is not judged
		// here: pppoe-client and xfrm carry ze:os "linux", so the schema of
		// any other system prunes them. Only that case is skipped, a path
		// Lookup misses and LookupDeclared still resolves; a path no module
		// declares falls through and fails below, so the table cannot go stale.
		if _, lookupErr := schema.Lookup(path); lookupErr != nil {
			if _, declaredErr := schema.LookupDeclared(path); declaredErr == nil {
				continue
			}
		}
		if _, found := changed[path]; !found {
			t.Errorf("%s: positionalMeaningChangesReviewed names it, but its positional slots no longer change", path)
		}
	}
}

// slotRefusal is the text refuseInlineSlot (config/parser_list.go) gives a
// one-line value that lands on a child holding no value of its own.
const slotRefusal = "which holds no value of its own"

// slotProbe asks the parser whether a one-line list entry may give a child a
// value, so the test reads that rule from its one producer
// (inlineSlotTakesValue, behind refuseInlineSlot) instead of a copy. Answers
// are cached per node. Not safe for concurrent use.
type slotProbe struct {
	t     *testing.T
	takes map[config.Node]bool
}

// newSlotProbe answers a probe whose reading of the parser is checked first:
// a plain container must be refused and a string leaf accepted, so a reworded
// refusal fails here instead of reading every child as taking a value.
func newSlotProbe(t *testing.T) *slotProbe {
	t.Helper()
	probe := &slotProbe{t: t, takes: map[config.Node]bool{}}
	if probe.takesValue("block", config.Container()) {
		t.Fatalf("a one-line value on a plain container is no longer refused with %q: refuseInlineSlot changed", slotRefusal)
	}
	if !probe.takesValue("text", config.Leaf(config.TypeString)) {
		t.Fatalf("a one-line value on a string leaf is refused with %q", slotRefusal)
	}
	return probe
}

// takesValue answers whether child, named name, takes the value of a one-line
// list entry: a one-child list holding it parses `probe k x;`, and the parser
// refuses x with slotRefusal exactly when the child holds no value of its own.
// A parse, or a refusal of x by the child's own type, is a slot that takes a
// value.
func (p *slotProbe) takesValue(name string, child config.Node) bool {
	if takes, known := p.takes[child]; known {
		return takes
	}
	schema := config.NewSchema()
	schema.Define("probe", config.List(config.TypeString, config.Field(name, child)))
	_, err := config.NewParser(schema).Parse("probe k x;")
	takes := err == nil || !strings.Contains(err.Error(), slotRefusal)
	p.takes[child] = takes
	return takes
}

// meaningChanges records, under its path, every keyed list at or under node
// whose positional slots differ between the previous order and Children().
// definers are the goyang entries that define node, in module-name order
// (more than one where two -conf modules define the same node). Recursion is
// bounded by the schema's depth.
func (p *slotProbe) meaningChanges(node config.Node, definers []*gyang.Entry, path string, changed map[string]string) {
	lister, ok := node.(interface {
		Children() []string
		Get(name string) config.Node
	})
	if !ok {
		return
	}
	if list, isList := node.(*config.ListNode); isList && list.KeyName != "" && len(definers) > 0 {
		before := p.positionalSlots(list, previousListOrder(definers, list))
		after := p.positionalSlots(list, list.Children())
		if !slices.Equal(before, after) {
			changed[path] = strings.Join(before, " ") + " -> " + strings.Join(after, " ")
		}
	}
	for _, name := range lister.Children() {
		p.meaningChanges(lister.Get(name), definersOf(definers, name), config.AppendPath(path, name), changed)
	}
}

// positionalSlots answers the children the values of a positional line land
// on, for a list whose children come in order, as parseListInlineEntry
// assigns them: one value to each child before the last while each takes a
// value (the first that does not refuses the line), and the rest, or a
// bracket, to the last child, marked "last:", when it takes a value. Two
// orders with equal slots read every positional line alike.
func (p *slotProbe) positionalSlots(list *config.ListNode, order []string) []string {
	if len(order) == 0 {
		return nil
	}
	var slots []string
	for _, name := range order[:len(order)-1] {
		if !p.takesValue(name, list.Get(name)) {
			break
		}
		slots = append(slots, name)
	}
	if last := order[len(order)-1]; p.takesValue(last, list.Get(last)) {
		slots = append(slots, "last:"+last)
	}
	return slots
}

// previousListOrder answers the child order the converter gave a list before
// the definition order. Each definer, in module-name order, contributed its
// children by that rule (the deleted yangChildOrder and flattenChoiceCases):
// the leaf, leaf-list, container, list, choice and anyxml statements of the
// list's own statement in source order, then every other child (from a `uses`
// or an `augment`) by name; the key left out; a choice expanded case by case
// by name, each case's nodes by name. A later definer's children followed
// those of the earlier ones, a name already present keeping its place, as
// Schema.Define merges them (mergeNode). Children the schema pruned (ze:os)
// are left out, as Children() leaves them.
func previousListOrder(definers []*gyang.Entry, list *config.ListNode) []string {
	var order []string
	for _, entry := range definers {
		for _, name := range previousDefinerOrder(entry) {
			if !slices.Contains(order, name) {
				order = append(order, name)
			}
		}
	}
	return slices.DeleteFunc(order, func(name string) bool { return list.Get(name) == nil })
}

// previousDefinerOrder answers the children one module's entry for a list
// contributed under the rule previousListOrder describes.
func previousDefinerOrder(entry *gyang.Entry) []string {
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
	return order
}

// previousChoiceOrder answers the data nodes of a choice as the deleted
// flattenChoiceCases did: cases by name, each case's nodes by name, a nested
// choice the same way. Recursion is bounded by the schema's depth.
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
// inside a case of one of its choices. Recursion is bounded by the schema's
// depth.
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
