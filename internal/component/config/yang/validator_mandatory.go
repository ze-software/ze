// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator.go — walkTree calls the mandatory checks
// RFC: rfc/short/rfc7950.md

package yang

import (
	"slices"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// requireMandatoryUnderAbsent reports every mandatory node reachable from the
// absent non-presence container at path through non-presence containers only.
//
// RFC 7950 Section 7.6.5: "If "mandatory" is "true", the behavior of the
// constraint depends on the type of the leaf's closest ancestor node in the
// schema tree that is not a non-presence container (see Section 7.5.1): o If
// no such ancestor exists in the schema tree, the leaf MUST exist. [...] o
// Otherwise, the leaf MUST exist if the ancestor node exists in the data tree."
//
// The caller has established that this closest ancestor exists: it is the node
// being walked, a present container, a list entry, the section root that
// stands for the module's top level, or a case with a node in the data. The
// descent stops at a list, a presence container and a choice, because each
// (for a choice, each of its cases) is itself the closest ancestor of what
// lies below it, and none of them exists here. Recursion depth is bounded by
// the depth of the compiled YANG schema; no configuration data drives it.
func requireMandatoryUnderAbsent(path string, container *yang.Entry, errs *[]ValidationError) {
	var tb textbuf.Buffer
	for name, child := range container.Dir {
		childPath := tb.Reset().Str(path).Byte('/').Str(name).String()
		if child.Mandatory == yang.TSTrue {
			*errs = append(*errs, missingMandatory(childPath, name, child))
			continue
		}
		if isNonPresenceContainer(child) {
			requireMandatoryUnderAbsent(childPath, child, errs)
		}
	}
}

// requireMandatoryChildren reports every mandatory node missing among the
// children of entry, the walked node, whose data is data. entry is a container,
// a list entry, the section root, or a case whose data sits in its parent's
// map. It also reports an absent leaf-list whose min-elements is above zero,
// and the mandatory nodes below an absent non-presence child. Recursion depth
// is bounded by the depth of the compiled YANG schema; no configuration data
// drives it.
func requireMandatoryChildren(path string, entry *yang.Entry, data map[string]any, errs *[]ValidationError) {
	var tb textbuf.Buffer
	for name, child := range entry.Dir {
		if child.IsChoice() {
			if child.Mandatory == yang.TSTrue && !mandatoryPresent(name, child, data) {
				*errs = append(*errs, missingMandatory(tb.Reset().Str(path).Byte('/').Str(name).String(), name, child))
			}
			requireMandatoryInPresentCases(path, child, data, errs)
			continue
		}
		if child.Mandatory == yang.TSTrue {
			if _, ok := data[name]; !ok {
				*errs = append(*errs, missingMandatory(tb.Reset().Str(path).Byte('/').Str(name).String(), name, child))
			}
			continue
		}
		// RFC 7950 Section 7.6.5: an absent non-presence container does not end
		// the mandatory check below it. The constraint is decided by the closest
		// ancestor that is not a non-presence container (Section 7.5.1), and for
		// everything under this child that ancestor is entry: the node being
		// walked, or a case with a node present. Either exists.
		if isNonPresenceContainer(child) {
			if _, ok := data[name]; !ok {
				requireMandatoryUnderAbsent(tb.Reset().Str(path).Byte('/').Str(name).String(), child, errs)
			}
		}
		// An ABSENT leaf-list with min-elements > 0 violates its bound just as
		// surely as an empty one, and NOTHING else catches it.
		//
		// The obvious assumption -- "absence is the mandatory check's business"
		// above -- is FALSE for a leaf-list, and provably so at the producer:
		// goyang's `case *LeafList:` (vendor/.../goyang/pkg/yang/entry.go)
		// synthesizes a Leaf WITHOUT copying Mandatory, and *LeafList has no
		// Mandatory field to copy (yang.go: it carries MaxElements/MinElements
		// but no mandatory). So Entry.Mandatory is TSUnset forever for a
		// leaf-list and the check above can never fire on one.
		//
		// Omitting the leaf entirely is also the shape an operator is most
		// likely to write, so checking only the explicitly-empty `foo [ ]` form
		// would leave the common case inert.
		if child.IsLeafList() && child.ListAttr != nil && child.ListAttr.MinElements > 0 {
			if _, ok := data[name]; !ok {
				checkCardinality(tb.Reset().Str(path).Byte('/').Str(name).String(), child, 0, errs)
			}
		}
	}
}

// requireMandatoryInPresentCases checks the children of every case of choice
// that has a data node in data. A case never appears in the data tree under
// its own name: its nodes sit in the map of the node that holds the choice, so
// the case's children are checked against that same map and reported under
// that node's path. A case with no node present is not checked, because
// nothing makes its mandatory nodes owed.
//
// RFC 7950 Section 7.6.5: "Otherwise, if this ancestor is a case node, the
// leaf MUST exist if any node from the case exists in the data tree."
//
// Recursion depth is bounded by the depth of the compiled YANG schema.
func requireMandatoryInPresentCases(path string, choice *yang.Entry, data map[string]any, errs *[]ValidationError) {
	for _, branch := range choice.Dir {
		if !holdsData(branch, data) {
			continue
		}
		requireMandatoryChildren(path, branch, data, errs)
	}
}

// isNonPresenceContainer reports whether entry is a container with no presence
// statement (RFC 7950 Section 7.5.1). goyang keeps that statement in
// Extra["presence"].
func isNonPresenceContainer(entry *yang.Entry) bool {
	if !entry.IsContainer() {
		return false
	}
	return len(entry.Extra["presence"]) == 0
}

// missingMandatory builds the error for a mandatory node absent from the data:
// what is missing, why it is required, and the command that supplies it. path
// is the slash-joined path the walk built, a list entry written as name[key].
//
// A choice gets its own message. It never appears in the data under its own
// name, so `set <parent> <choice> <value>` is a command the parser refuses;
// the operator types one of the case nodes instead, and the message lists each
// one as a command.
func missingMandatory(path, name string, entry *yang.Entry) ValidationError {
	var tb textbuf.Buffer
	parent := strings.TrimSuffix(path, "/"+name)
	if entry.IsChoice() {
		tb.Str("mandatory choice ").Quoted(name).Str(" is missing: the YANG schema requires one of its cases under ").
			Str(parent).Str(" (RFC 7950 Section 7.9.4); add one with")
		for i, node := range choiceDataNodes(entry) {
			if i > 0 {
				tb.Byte(',')
			}
			tb.Str(" `").Str(setCommandFor(parent, node)).Byte('`')
		}
		return ValidationError{Path: path, Type: ErrTypeMissing, Message: tb.String()}
	}
	return ValidationError{
		Path: path,
		Type: ErrTypeMissing,
		Message: tb.Str("mandatory field ").Quoted(name).Str(" is missing: the YANG schema marks ").
			Str(path).Str(" mandatory (RFC 7950 Section 7.6.5); add it with `").
			Str(setCommandFor(parent, entry)).Str("`").String(),
	}
}

// choiceDataNodes returns, sorted by name, the data nodes a choice's cases
// hold, the nodes of a nested choice included: the names an operator can type
// to satisfy the choice. Recursion depth is bounded by the depth of the
// compiled YANG schema.
func choiceDataNodes(choice *yang.Entry) []*yang.Entry {
	var nodes []*yang.Entry
	for _, branch := range choice.Dir {
		for _, node := range branch.Dir {
			if node.IsChoice() {
				nodes = append(nodes, choiceDataNodes(node)...)
				continue
			}
			nodes = append(nodes, node)
		}
	}
	slices.SortFunc(nodes, func(a, b *yang.Entry) int { return strings.Compare(a.Name, b.Name) })
	return nodes
}

// setCommandFor writes the `set` command that creates node under the walk path
// parent, shaped by what the node takes: an empty leaf and a presence
// container are complete alone, a non-presence container or a list takes more
// words, any other leaf takes a value.
func setCommandFor(parent string, node *yang.Entry) string {
	var tb textbuf.Buffer
	tb.Str("set ").Str(setCommandWords(parent)).Byte(' ').Str(node.Name)
	if node.IsLeaf() && node.Type != nil && node.Type.Kind == yang.Yempty {
		return tb.String()
	}
	if node.IsContainer() && !isNonPresenceContainer(node) {
		return tb.String()
	}
	if node.IsDir() {
		return tb.Str(" ...").String()
	}
	return tb.Str(" <value>").String()
}

// Sections returns, sorted and once each, every top-level container that a
// conf module of the validator's schema declares. A section several modules
// contribute to, as l2tp is, appears once. The set comes from the compiled
// schema, so a section a new module adds is in it without any edit here.
func (v *Validator) Sections() []string {
	seen := make(map[string]bool)
	for _, modName := range v.schema.ConfModuleNames() {
		entry := v.schema.GetEntry(modName)
		if entry == nil {
			continue
		}
		for name, child := range entry.Dir {
			if child.IsContainer() {
				seen[name] = true
			}
		}
	}
	sections := make([]string, 0, len(seen))
	for name := range seen {
		sections = append(sections, name)
	}
	slices.Sort(sections)
	return sections
}

// setCommandWords turns a walk path such as bgp/peer[london]/session into the
// words of a `set` command: bgp peer london session. A slash inside a list key,
// as in route[10.0.0.0/8], is part of the key and is kept.
func setCommandWords(path string) string {
	var tb textbuf.Buffer
	inKey := false
	for i := range len(path) {
		c := path[i]
		switch {
		case c == '[':
			inKey = true
			tb.Byte(' ')
		case c == ']':
			inKey = false
		case c == '/' && !inKey:
			tb.Byte(' ')
		default:
			tb.Byte(c)
		}
	}
	return tb.String()
}

// mandatoryPresent reports whether the mandatory node name is present in data.
// A leaf or container is present under its own name. A choice never appears
// in the data under its own name. RFC 7950 Section 7.9.4: "If "mandatory" is
// "true", at least one node from exactly one of the choice's case branches
// MUST exist." So a mandatory choice is present when any data node of any of
// its cases is, nested choices included. goyang keeps those nodes under the
// choice and case entries rather than in the parent's Dir. Recursion depth is
// bounded by the depth of the compiled YANG schema.
func mandatoryPresent(name string, entry *yang.Entry, data map[string]any) bool {
	if _, ok := data[name]; ok {
		return true
	}
	if !entry.IsChoice() {
		return false
	}
	for _, branch := range entry.Dir {
		for nodeName, node := range branch.Dir {
			if mandatoryPresent(nodeName, node, data) {
				return true
			}
		}
	}
	return false
}
