// Design: docs/architecture/bgp/egress-attribute-rules.md -- ordinary selected-route MED preservation before iBGP readvertisement.
// Related: register_process.go -- process helper registration.
// Related: check_rfc.go -- GoBGP MED attribute predicates.
package bgp

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioMEDIBGPOrdinaryPostSelectionRemovalGoBGP = "bgp-med-ibgp-ordinary-post-selection-removal-gobgp"
	medOrdinarySubjectPrefix                         = "10.63.0.0/24"
	medOrdinaryControlPrefix                         = "10.63.1.0/24"
)

func init() {
	specialCheckers[scenarioMEDIBGPOrdinaryPostSelectionRemovalGoBGP] = checkMEDIBGPOrdinaryPostSelectionRemovalGoBGP
	registerProcessHelper(scenarioMEDIBGPOrdinaryPostSelectionRemovalGoBGP, runRawMEDFilter)
}

// checkMEDIBGPOrdinaryPostSelectionRemovalGoBGP verifies ordinary bgp-rib
// selected-route iBGP readvertisement (no bgp-rs) against a foreign GoBGP peer
// when a raw export filter attempts to strip MED.
//
// FRR 10.7.1 bgp_route.c (redistribute static + route-map set metric / set
// community) originates both prefixes toward Ze; GoBGP global rib prints
// `Med: <n>` when MULTI_EXIT_DISC is present and omits `Med` when absent.
//
// RFC requirement: RFC4271-9.1.2.2-2 positive -- on ordinary bgp-rib transit
// (no bgp-rs), FRR's eBGP-learned 10.63.0.0/24 with MED 100 reaches internal
// GoBGP carrying Med: 100 even though raw-med-drop removed attribute 4 on
// export.
// RFC requirement: RFC4271-9.1.2.2-2 negative -- on the same ordinary bgp-rib
// transit and iBGP session, 10.63.1.0/24 whose MED was removed on import before
// selection reaches GoBGP with no Med attribute synthesized.
func checkMEDIBGPOrdinaryPostSelectionRemovalGoBGP(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name         = "bgp-med-ibgp-ordinary-post-selection-removal-gobgp"
		sourceAS     = "65005"
		sourceMetric = "100"
	)
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}

	zeAddr := networkHostAddress(check.Network, 2)

	sessions := []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opGoBGPSession, argument: zeLabAddress},
	}
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+1, err)
		}
	}

	arrivals := []operation{
		{kind: opGoBGPRoute, argument: medOrdinarySubjectPrefix, timeout: 120 * time.Second},
		{kind: opGoBGPRoute, argument: medOrdinaryControlPrefix, timeout: 120 * time.Second},
		{kind: opWaitLogContains, peer: "ze", contains: []string{"RAW-MED-DROP: removed MULTI_EXIT_DISC"}, timeout: 120 * time.Second},
	}
	for index := range arrivals {
		if err := runOperation(ctx, check.Network, check.Lab, &arrivals[index]); err != nil {
			return fail(index+3, err)
		}
	}

	subject, err := check.Lab.Query(ctx, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4, medOrdinarySubjectPrefix}, nil)
	if err != nil {
		return fail(6, err)
	}
	if err := requireGoBGPSourceBlock(subject, medOrdinarySubjectPrefix, sourceAS, zeAddr); err != nil {
		return fail(6, err)
	}
	if err := requireGoBGPAttributeValue(subject, medOrdinarySubjectPrefix, gobgpMEDAttribute, sourceMetric); err != nil {
		return fail(6, err)
	}

	control, err := check.Lab.Query(ctx, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4, medOrdinaryControlPrefix}, nil)
	if err != nil {
		return fail(7, err)
	}
	if err := requireGoBGPSourceBlock(control, medOrdinaryControlPrefix, sourceAS, zeAddr); err != nil {
		return fail(7, err)
	}
	if err := requireGoBGPAttributeAbsent(control, medOrdinaryControlPrefix, gobgpMEDAttribute); err != nil {
		return fail(7, err)
	}

	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+8, err)
		}
	}
	return nil
}
