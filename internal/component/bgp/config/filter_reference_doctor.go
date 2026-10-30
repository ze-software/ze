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
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeFilterReferenceUndefined is raised when a filter chain names a policy
// that the policy root does not define. internal/core/diagnostic/codes.go declares
// it, so `ze show diagnostic doctor-config-reference` answers.
const codeFilterReferenceUndefined = "doctor-config-reference"

// filterReferenceDoctorCheck describes the check. register.go registers it.
//
// Order 842 keeps it where the runner used to call it: after the certificate
// and host-key checks, before the reachability probes.
var filterReferenceDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-filter-references",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        842,
	Component:    doctorComponentBGP,
	Dependencies: []string{doctorDependencyConfigTree},
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

	// The filter types (prefix-list, as-path-list, and the rest) augment the
	// policy root and carry the ze:filter marker, so the defined set is the
	// policy registry's instances, the same namespace the peer build reads.
	// An entry of an unmarked list (policy route) is not a filter.
	schema, err := config.YANGSchema()
	if err != nil {
		return []diagnostic.Diagnostic{filterRegistryDiagnostic(err)}
	}
	defined, err := policyRegistryOf(tree, schema)
	if err != nil {
		return []diagnostic.Diagnostic{filterRegistryDiagnostic(err)}
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

// filterRegistryDiagnostic reports a policy root the check could not read, so
// the doctor never answers "no undefined reference" for a namespace it did not
// build.
func filterRegistryDiagnostic(err error) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     codeFilterReferenceUndefined,
		Severity: diagnostic.SeverityError,
		Message:  "policy: the filter namespace could not be built, so filter references were not checked: " + err.Error(),
	}
}

// undefinedFilterRefs reports the import and export entries of one chain that
// name a policy the defined set does not hold.
//
// It reads the structural member view, so a deactivated reference is judged
// too: a deactivated entry pointing at a name nothing defines is still wrong,
// and the operator meets it the moment they activate the line.
func undefinedFilterRefs(filter *config.Tree, defined *policy.Registry, path string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, direction := range []string{"import", "export"} {
		chain := filter.GetContainer(direction)
		if chain == nil {
			continue
		}
		for _, member := range chain.GetMultiValuesState("filters") {
			name := filterInstanceName(member.Value)
			if _, ok := defined.Lookup(name); ok {
				continue
			}
			location := tb.Reset().Str(path).Byte('/').Str(direction).Str("/filters").String()
			message := location + ": references undefined filter '" + member.Value + "'"
			if defined.IsNonFilterEntry(name) {
				// A policy routing rule is defined, just not a filter: say so in
				// the words the load refusal uses (policy.Registry.ValidateName).
				message = defined.ValidateName(name, location).Error()
			}
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeFilterReferenceUndefined,
				Severity: diagnostic.SeverityError,
				Message:  message,
			})
		}
	}
	return diags
}
