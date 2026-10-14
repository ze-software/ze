// Design: docs/architecture/wire/nlri.md -- NLRI receive validation, the per-family syntax registry
// RFC: rfc/short/rfc9552.md -- Section 8.2.2; rfc/short/rfc7432.md -- Section 7.1
// Related: rfc7606_bgpls_nlri.go -- the BGP-LS Judge and Retain answers
// Related: evpn_nlri.go -- the EVPN Retain answer
//
// The BGP-LS and EVPN NLRI syntax rules live in this package because their walks
// do; they register in the same nlrisyntax registry every other family uses, so
// the receive path names neither family.

package message

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
)

func init() {
	bgpls := nlrisyntax.Rule{
		Citation:       "RFC 9552 Section 8.2.2",
		RemovedMessage: "RFC 9552 Section 8.2.2: discarded malformed Link-State NLRIs",
		Judge:          judgeBGPLSNLRISection,
	}
	registerNLRISyntax(family.Family{AFI: family.AFI(afiBGPLS), SAFI: family.SAFI(safiBGPLS)}, bgpls,
		func(section []byte, addPath bool) ([]byte, int, bool) {
			return RetainWellFormedNLRI(safiBGPLS, section, addPath)
		})
	registerNLRISyntax(family.Family{AFI: family.AFI(afiBGPLS), SAFI: family.SAFI(safiBGPLSVPN)}, bgpls,
		func(section []byte, addPath bool) ([]byte, int, bool) {
			return RetainWellFormedNLRI(safiBGPLSVPN, section, addPath)
		})
	registerNLRISyntax(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
		nlrisyntax.Rule{
			Citation:       "RFC 7432 Section 7.1",
			RemovedMessage: "RFC 7432 Section 7.1: discarded Ethernet A-D NLRIs",
		}, retainEVPNNLRI)
}

// registerNLRISyntax installs rule with retain as its Retain answer, and stops
// the process when the registry refuses it: a family with two rulings, or none
// where its walk exists, is a build defect no runtime path can resolve.
func registerNLRISyntax(fam family.Family, rule nlrisyntax.Rule, retain func([]byte, bool) ([]byte, int, bool)) {
	rule.Retain = retain
	if err := nlrisyntax.Register(fam, rule); err != nil {
		slog.Error("message: NLRI syntax rule registration failed", "family", fam, "error", err)
		os.Exit(1)
	}
}
