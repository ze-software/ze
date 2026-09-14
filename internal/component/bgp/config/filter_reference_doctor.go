// Design: docs/architecture/config/syntax.md -- BGP filter chain references
// Overview: peers.go -- filterInstanceName and the policy registry lookup this check mirrors
// Related: register.go -- the doctor-check registration from init()
// Related: bfd_strict_doctor.go -- the sibling readiness check this package owns
//
// A filter chain that names a policy the configuration never defines.
//
// The reference resolves at peer build time, where an unknown name leaves the
// chain short of the filter the operator asked for. This check answers the same
// question from the tree alone, before the daemon starts, so the operator reads
// the name they typed rather than a silently shorter chain.
//
// It lives here because the names it judges are BGP policy names, and because
// filterInstanceName -- the rule that strips a reference's prefix -- is
// declared in peers.go. Doctor held a second copy of that rule until this file
// moved the check onto the one declaration; the two had already drifted, the
// copy cutting at the LAST colon where this one cuts at the first.
package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeFilterReferenceUndefined is raised when a filter chain names a policy
// that bgp/policy does not define. internal/core/diagnostic/codes.go declares
// it, so `ze explain doctor-config-reference` answers.
const codeFilterReferenceUndefined = "doctor-config-reference"

// filterReferenceDoctorCheck describes the check. register.go registers it.
//
// Order 842 keeps it where the runner used to call it: after the certificate
// and host-key checks, before the reachability probes.
var filterReferenceDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-filter-references",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        842,
	Component:    "bgp",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeFilterReferenceUndefined},
	Check:        checkFilterReferences,
}

// checkFilterReferences reports every filter chain entry, at any level, that
// names a policy the configuration does not define.
func checkFilterReferences(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}

	bgpBlock := tree.GetContainer("bgp")
	if bgpBlock == nil {
		return nil
	}

	// The policy lists (prefix-list, as-path, and the rest) are added by
	// plugins through a YANG augment, so the defined set is every key of every
	// list rather than a fixed set of list names.
	defined := make(map[string]bool)
	if policy := bgpBlock.GetContainer("policy"); policy != nil {
		collectFilterNames(policy.ToMap(), defined)
	}

	var diags []diagnostic.Diagnostic
	if filter := bgpBlock.GetContainer("filter"); filter != nil {
		diags = append(diags, undefinedFilterRefs(filter, defined, "bgp/filter")...)
	}

	var tb textbuf.Buffer
	for _, group := range bgpBlock.GetListOrdered("group") {
		groupPath := tb.Reset().Str("bgp/group/").Str(group.Key).Str("/filter").String()
		if filter := group.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, undefinedFilterRefs(filter, defined, groupPath)...)
		}
		for _, peer := range group.Value.GetListOrdered("peer") {
			peerPath := tb.Reset().Str("bgp/group/").Str(group.Key).Str("/peer/").Str(peer.Key).Str("/filter").String()
			if filter := peer.Value.GetContainer("filter"); filter != nil {
				diags = append(diags, undefinedFilterRefs(filter, defined, peerPath)...)
			}
		}
	}

	for _, peer := range bgpBlock.GetListOrdered("peer") {
		peerPath := tb.Reset().Str("bgp/peer/").Str(peer.Key).Str("/filter").String()
		if filter := peer.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, undefinedFilterRefs(filter, defined, peerPath)...)
		}
	}

	return diags
}

// collectFilterNames reads the policy container's map view and collects every
// second-level key as a defined filter instance name. The shape is
// {"prefix-list": {"customers": {...}}, "as-path": {"as1234": {...}}}.
func collectFilterNames(policy map[string]any, defined map[string]bool) {
	for _, list := range policy {
		entries, ok := list.(map[string]any)
		if !ok {
			continue
		}
		for name := range entries {
			defined[name] = true
		}
	}
}

// undefinedFilterRefs reports the import and export entries of one chain that
// name a policy the defined set does not hold.
//
// It reads the structural member view, so a deactivated reference is judged
// too: a deactivated entry pointing at a name nothing defines is still wrong,
// and the operator meets it the moment they activate the line.
func undefinedFilterRefs(filter *config.Tree, defined map[string]bool, path string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, direction := range []string{"import", "export"} {
		for _, member := range filter.GetMultiValuesState(direction) {
			if defined[filterInstanceName(member.Value)] {
				continue
			}
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeFilterReferenceUndefined,
				Severity: diagnostic.SeverityError,
				Message: tb.Reset().Str(path).Byte('/').Str(direction).
					Str(": references undefined filter '").Str(member.Value).Byte('\'').String(),
			})
		}
	}
	return diags
}
