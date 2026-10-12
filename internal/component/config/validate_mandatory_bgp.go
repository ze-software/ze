// Design: docs/architecture/config/yang-config-design.md -- mandatory nodes and ze:required inheritance
// Related: validate_sections.go -- ValidateCustomSections appends these failures
// Related: required.go -- hasNestedMapValue, the presence test ze:required uses
// RFC: rfc/short/rfc7950.md

package config

import (
	"maps"
	"strings"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// bgpSessionPathOf names, for a global bgp leaf whose peer-level override sits
// at a different path, where a peer or a group carries that override. A global
// leaf not named here is overridden at the same relative path, as
// session/asn/local is. A mandatory global leaf added later with no entry here
// is checked at its own path: if no peer carries it there, the config is
// refused, so a missing entry fails closed.
var bgpSessionPathOf = map[string]string{
	"router-id": "session/router-id",
}

// bgpMandatoryFailures runs the RFC 7950 Section 7.6.5 mandatory check over the
// global level of the bgp section, and counts a value every BGP session
// inherits from its group or carries itself as present.
//
// Only the mandatory check runs here. The rest of the bgp walk is withheld for
// the reason knownUnwalkedValidatorSections records, and the peer and group
// lists are removed before the walk so their custom validators never run.
//
// RFC 7950 Section 7.6.5: "If no such ancestor exists in the schema tree, the
// leaf MUST exist." bgp, bgp/session and bgp/session/asn are non-presence
// containers, so bgp/router-id and bgp/session/asn/local are required whenever
// the bgp section exists. Ze's design lets a group or a peer supply both, and
// ResolveBGPTree layers global, group and peer values into each session, so the
// owner ruled on 2026-10-12 that a value every session resolves counts as
// present (plan/pre-release/spec-config-yang-when-unique-choice.md, AC-2).
func bgpMandatoryFailures(validator *yang.Validator, tree *Tree) []SectionValidationError {
	bgp := tree.GetContainer(string(ConfigTypeBGP))
	if bgp == nil {
		return nil
	}
	data := bgp.ToMap()
	global := maps.Clone(data)
	delete(global, "peer")
	delete(global, "group")

	var found []SectionValidationError
	for _, ve := range validator.ValidateTreeAllModules(string(ConfigTypeBGP), global) {
		if ve.Type != yang.ErrTypeMissing {
			continue
		}
		if EverySessionCarries(data, ve.Path) {
			continue
		}
		found = append(found, SectionValidationError{Section: string(ConfigTypeBGP), Err: ve})
	}
	return found
}

// EverySessionCarries reports whether every BGP session source in the bgp
// section resolves the global leaf at missingPath (bgp/<relative path>) from
// its own value or its group's. A source is a standalone peer, a peer inside a
// group, or a group with no peer, which is the template a dynamic group
// instantiates. A section with no source answers false: nothing supplies the
// value, so the global leaf stays required.
func EverySessionCarries(data map[string]any, missingPath string) bool {
	relative := strings.TrimPrefix(missingPath, string(ConfigTypeBGP)+"/")
	sessionPath := relative
	if p, ok := bgpSessionPathOf[relative]; ok {
		sessionPath = p
	}
	path := strings.Split(sessionPath, "/")

	sources := 0
	for _, peer := range listEntries(data, "peer") {
		sources++
		if !hasNestedMapValue(peer, path) {
			return false
		}
	}
	for _, group := range listEntries(data, "group") {
		members := listEntries(group, "peer")
		if len(members) == 0 {
			sources++
			if !hasNestedMapValue(group, path) {
				return false
			}
			continue
		}
		groupCarries := hasNestedMapValue(group, path)
		for _, peer := range members {
			sources++
			if groupCarries {
				continue
			}
			if !hasNestedMapValue(peer, path) {
				return false
			}
		}
	}
	return sources > 0
}

// listEntries returns the entries of the list named name in data, the shape
// Tree.ToMap writes: a map keyed by the list key, one map per entry.
func listEntries(data map[string]any, name string) []map[string]any {
	list, ok := data[name].(map[string]any)
	if !ok {
		return nil
	}
	entries := make([]map[string]any, 0, len(list))
	for _, v := range list {
		if entry, ok := v.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}
