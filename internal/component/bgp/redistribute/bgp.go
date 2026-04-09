// Design: docs/architecture/core-design.md -- BGP redistribute source registration

package redistribute

// RegisterBGPSources registers BGP-specific redistribute sources (ibgp, ebgp).
// Called during startup after the redistribute registry is available.
func RegisterBGPSources() {
	RegisterSource(RouteSource{
		Name:        "ibgp",
		Protocol:    "bgp",
		Description: "iBGP learned routes",
	})
	RegisterSource(RouteSource{
		Name:        "ebgp",
		Protocol:    "bgp",
		Description: "eBGP learned routes",
	})
}
