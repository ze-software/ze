// Design: docs/architecture/testing/interop.md -- the Extended Prefix/Link scenario.
// Related: check_ospf_ext_prefix_link.go -- the checker it registers.
package bgp

func init() {
	specialCheckers[ospfExtPrefixLinkScenario] = checkOSPFExtPrefixLink
}
