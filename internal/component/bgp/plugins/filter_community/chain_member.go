// Design: docs/guide/route-policy.md -- the community members an operator names in a peer's filter chain
// Overview: filter_community.go -- parseCommunityConfig, the verify and apply reader that calls this check
// Related: community_filter.go -- the members the check asks for

package filter_community

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errCommunityMemberMissing refuses community settings no chain member runs.
var errCommunityMemberMissing = errors.New("community settings are set but no filter chain runs them")

// errCommunityMemberOrder refuses a scrub that would run after the relation
// tag and delete the value the tag just derived.
var errCommunityMemberOrder = errors.New("the community relation tag runs before the scrub that would delete it")

// communityChains holds the import and export chain members of one peer's
// lineage: the bgp, group and peer levels, whose `filters` lists are run in
// order after the inherited ones (ze-bgp-conf.yang), so a member at any
// level runs for the peer.
type communityChains struct {
	imports, exports []string
}

// collectCommunityChains reads the chain members of every level the peer
// inherits from. A nil level holds none.
func collectCommunityChains(levels ...map[string]any) communityChains {
	var chains communityChains
	for _, level := range levels {
		filter, ok := level["filter"].(map[string]any)
		if !ok {
			continue
		}
		if block, ok := filter["import"].(map[string]any); ok {
			chains.imports = append(chains.imports, configvalue.LeafList(block["filters"])...)
		}
		if block, ok := filter["export"].(map[string]any); ok {
			chains.exports = append(chains.exports, configvalue.LeafList(block["filters"])...)
		}
	}
	return chains
}

// namesMember reports whether members holds the reference. A member the
// operator deactivated never reaches this list: the config tree lowers a
// leaf-list to its active members only (config.Tree toMap), so settings whose
// only member is deactivated are refused like settings with none.
func namesMember(members []string, reference string) bool {
	return slices.Contains(members, reference)
}

// checkCommunityMembers refuses a peer whose community settings name no
// member to run them (spec-filter-wire-6-stage-filters AC-11): since the
// members are ordinary chain members (D-3), a setting with no member would
// otherwise change nothing, in silence. The error names the subject, the
// setting, and the exact config line to add.
func checkCommunityMembers(subject string, fc filterConfig, chains communityChains) error {
	ingress := len(fc.ingressTag) > 0 || len(fc.ingressStrip) > 0 || fc.scrubEnabled() ||
		fc.blackholeGuardToken() != blackholeGuardNone
	if ingress && !namesMember(chains.imports, communityFilterRef(ingressFilterName)) {
		return memberMissing(subject, "filter ingress community", "import", ingressFilterName)
	}
	if fc.relationTagEnabled() && !namesMember(chains.imports, communityFilterRef(relationFilterName)) {
		return memberMissing(subject, "filter ingress community relation-tag", "import", relationFilterName)
	}
	// The scrub's keep-list never keeps the relation function, so a scrub run
	// after the relation tag deletes the value Ze derived (RFC 8195 Section
	// 3.2 tag). The members are the operator's to order, and this order is
	// the one that changes no route in silence.
	if fc.scrubEnabled() && fc.relationTagEnabled() &&
		slices.Index(chains.imports, communityFilterRef(relationFilterName)) <
			slices.Index(chains.imports, communityFilterRef(ingressFilterName)) {
		return memberOrder(subject)
	}
	egress := len(fc.egressTag) > 0 || len(fc.egressStrip) > 0
	if egress && !namesMember(chains.exports, communityFilterRef(egressFilterName)) {
		return memberMissing(subject, "filter egress community", "export", egressFilterName)
	}
	return nil
}

func memberMissing(subject, setting, direction, filter string) error {
	var tb textbuf.Buffer
	tb.Str(configLabel(subject)).Str(" sets ").Str(setting).Str(", and its ").Str(direction).
		Str(" chain does not run ").Str(communityFilterRef(filter)).
		Str(", so the setting would change no route. Add the member to the ").Str(configLabel(subject)).
		Str(" (or its group, or bgp): filter { ").Str(direction).Str(" { filters [ ").
		Str(communityFilterRef(filter)).Str(" ]; } }")
	return fmt.Errorf("%w: %s", errCommunityMemberMissing, tb.String())
}

func memberOrder(subject string) error {
	var tb textbuf.Buffer
	tb.Str(configLabel(subject)).Str(" scrubs its own communities and tags the relation, and its import chain runs ").
		Str(communityFilterRef(relationFilterName)).Str(" before ").Str(communityFilterRef(ingressFilterName)).
		Str(", so the scrub would delete the relation tag. Name ").Str(communityFilterRef(ingressFilterName)).
		Str(" first: filter { import { filters [ ").Str(communityFilterRef(ingressFilterName)).Str(" ").
		Str(communityFilterRef(relationFilterName)).Str(" ]; } }")
	return fmt.Errorf("%w: %s", errCommunityMemberOrder, tb.String())
}
