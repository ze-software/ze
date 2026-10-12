// Design: docs/architecture/config/yang-config-design.md -- mandatory nodes and ze:required inheritance
// Related: validate_sections.go -- ValidateCustomSections appends these failures
// Related: required.go -- hasNestedMapValue, the presence test ze:required uses
// RFC: rfc/short/rfc7950.md

package config

import (
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
// whole bgp section, peer and group entries included, and counts a global value
// every BGP session inherits from its group or carries itself as present.
//
// Only the mandatory check is kept. The rest of the bgp walk is withheld for
// the reason knownUnwalkedValidatorSections records, so every other failure
// the walk reports, a custom validator's included, is dropped here.
//
// The inheritance answers for a global node alone, one whose path holds no
// list entry. A node missing inside a peer[...] or group[...] entry stays a
// failure. RFC 7950 Section 7.6.5: "Otherwise, the leaf MUST exist if the
// ancestor node exists in the data tree." The closest ancestor of such a leaf
// that is not a non-presence container is the list entry itself, and the
// entry exists, so the leaf MUST exist in it. ResolveBGPTree does deep-merge
// the bgp defaults and the group's fields into each peer it builds, so a value
// a group sets reaches its peers at run time, but this check does not count
// that: the owner ruling below covers the global router-id and local ASN only.
// A future mandatory peer-level leaf that only the group sets is therefore
// refused, and that case owes an owner ruling when the first such leaf lands.
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

	var found []SectionValidationError
	for _, ve := range validator.ValidateTreeAllModules(string(ConfigTypeBGP), data) {
		if ve.Type != yang.ErrTypeMissing {
			continue
		}
		// Guard: an entry path such as bgp/peer[p1]/required never matches
		// here. EverySessionCarries looks its first segment, peer[p1], up as a
		// field of each session, and no session holds a field named with a list
		// key, so a node missing inside an entry always stays a failure.
		// TestEverySessionCarriesNeverMatchesAnEntryPath holds that.
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
// group, a group with no peer, and a dynamic group whether or not it lists
// peers, because a dynamic group is the template its accepted sessions are
// built from. A section with no source answers false: nothing supplies the
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
		groupCarries := hasNestedMapValue(group, path)
		members := listEntries(group, "peer")
		// A dynamic group is a session source of its own whether or not it
		// lists static members: resolveDynamicGroup builds each accepted
		// session from the bgp defaults and the group's fields, never from a
		// member's, so a value only the members carry never reaches it.
		if len(members) == 0 || IsDynamicBGPGroup(group) {
			sources++
			if !groupCarries {
				return false
			}
		}
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

// IsDynamicBGPGroup reports whether a bgp group's fields, in the shape
// Tree.ToMap writes, declare it a dynamic group: connection remote ip is the
// literal "dynamic". ResolveBGPTree reads the same test to build the group's
// session template, so the mandatory check and the resolver agree on which
// groups are templates.
func IsDynamicBGPGroup(group map[string]any) bool {
	connection, ok := group["connection"].(map[string]any)
	if !ok {
		return false
	}
	remote, ok := connection["remote"].(map[string]any)
	if !ok {
		return false
	}
	ip, ok := remote["ip"].(string)
	return ok && ip == "dynamic"
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
