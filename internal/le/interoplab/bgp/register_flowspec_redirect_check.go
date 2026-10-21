// Design: docs/architecture/testing/interop.md -- a draft-simpson redirect-to-IP rule Ze must relay to FRR without its redirect action.
// Related: test/interop/scenarios/bgp-flowspec-redirect-check-frr/inject.msg -- the cover and the rule, as raw frames.
package bgp

import "time"

// scenarioFlowSpecRedirectCheckFRR proves draft-simpson-idr-flowspec-redirect-02
// Section 3 against another implementation: "the BGP speaker SHOULD discard
// the redirect-to-ip extended community (and not propagate it further with
// the flow-spec route) if the last AS in the AS_PATH or AS4_PATH attribute of
// the longest prefix match for X does not match the AS of the EBGP peer."
//
// The injector (AS 65004) sends a cover for X originated in AS 65009, then the
// rule with the 0x0800 community. FRR must hold the rule and must not hold
// the redirect action. FRR prints a 0x0800 community it decoded as
// "FS:redirect IP 0x<n>" in the path's "ecomlist" (bgpd/bgp_ecommunity.c,
// ecommunity_ecom2str; bgpd/bgp_flowspec_vty.c, route_vty_out_flowspec; the plain JSON form is the generic route printer and carries no extended communities).
//
// With the check disabled (the RIB never refuses the rule) Ze relays the
// community, FRR prints it, and the absence assertion fails.
const (
	scenarioFlowSpecRedirectCheckFRR = "bgp-flowspec-redirect-check-frr" // #nosec G101 -- a scenario directory name, not a credential.

	flowSpecRedirectCheckPrefix = "192.0.2.0/24"
	flowSpecRedirectCheckAction = "FS:redirect"
	frrShowFlowSpecDetailJSON   = "show bgp ipv4 flowspec detail json"
)

func init() {
	scenarioOperations[scenarioFlowSpecRedirectCheckFRR] = []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opWaitContains, peer: peerFRR,
			command:  []string{cmdVtysh, "-c", frrShowFlowSpecDetailJSON},
			contains: []string{flowSpecRedirectCheckPrefix},
			timeout:  60 * time.Second},
		// The rule and its community arrive in one UPDATE, so once FRR holds
		// the rule the community it carried is settled.
		{kind: opRequireAbsent, peer: peerFRR,
			command: []string{cmdVtysh, "-c", frrShowFlowSpecDetailJSON},
			absent:  []string{flowSpecRedirectCheckAction},
			proof:   []string{flowSpecRedirectCheckPrefix}},
		{kind: opFRRSession, argument: zeLabAddress},
	}
}
