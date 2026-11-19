// Design: docs/architecture/doctor-and-health-checks.md -- route refusal event codes
// Related: session_aigp.go -- recognized raw origination refuses a peer-own next hop
// RFC 4271 Section 5.1.3 -- see rfc/short/rfc4271.md.

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

const codeOriginatedPeerOwnNextHop = "doctor-bgp-originated-peer-own-next-hop"

const originatedPeerOwnNextHopFix = "set the UPDATE's NEXT_HOP to a reachable address other than the recipient's own address, " +
	"then send the corrected UPDATE again"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeOriginatedPeerOwnNextHop,
		Title:       "BGP route names its recipient as next hop",
		Description: "Ze withheld an originated announcement because its next hop is the receiving peer's own address. Correct the UPDATE's NEXT_HOP to name a reachable address distinct from that peer, then send the corrected UPDATE again.",
		Examples:    []string{"ze show errors", "ze show diagnostic " + codeOriginatedPeerOwnNextHop},
	}); err != nil {
		panic("BUG: bgp reactor: originated next-hop diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeOriginatedPeerOwnNextHop}); err != nil {
		panic("BUG: bgp reactor: originated next-hop event registration failed: " + err.Error())
	}
}
