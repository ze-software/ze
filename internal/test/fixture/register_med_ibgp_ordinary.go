// Design: docs/architecture/bgp/egress-attribute-rules.md -- ordinary selected-route MED preservation fixture registration.
// Related: med_ibgp_ordinary.go -- ordinary selected-route MED preservation fixture implementation.
package fixture

func init() {
	Register("plugin/med-ibgp-ordinary-post-selection-removal", medIBGPOrdinaryPostSelectionRemoval, StopsDaemon)
}
