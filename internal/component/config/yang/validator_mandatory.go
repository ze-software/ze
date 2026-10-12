// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator.go — walkTree calls the mandatory descent
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
// being walked, a present container, a list entry, or the section root that
// stands for the module's top level. The descent stops at a list, a presence
// container and a choice, because each is itself the closest ancestor of what
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
