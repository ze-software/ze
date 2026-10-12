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
			*errs = append(*errs, missingMandatory(childPath, name))
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
func missingMandatory(path, name string) ValidationError {
	var tb textbuf.Buffer
	return ValidationError{
		Path: path,
		Type: ErrTypeMissing,
		Message: tb.Str("mandatory field ").Quoted(name).Str(" is missing: the YANG schema marks ").
			Str(path).Str(" mandatory (RFC 7950 Section 7.6.5); add it with `set ").
			Str(setCommandWords(path)).Str(" <value>`").String(),
	}
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
// words of a `set` command: bgp peer london session.
func setCommandWords(path string) string {
	return strings.NewReplacer("/", " ", "[", " ", "]", "").Replace(path)
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
