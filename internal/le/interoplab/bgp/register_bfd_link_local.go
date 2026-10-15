// Design: docs/architecture/testing/interop.md -- BFD over link-local against FRR.
// Related: check_bfd_link_local.go -- the checker this file registers.
package bgp

func init() {
	specialCheckers[bfdLinkLocalTwoLinksScenario] = checkBFDLinkLocalTwoLinks
}
