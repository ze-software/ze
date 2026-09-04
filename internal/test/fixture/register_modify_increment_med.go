// Design: docs/architecture/bgp/egress-attribute-rules.md -- the metric a route carries onward

package fixture

// The observer for test/plugin/modify-increment-med-from-route-value.ci. It is
// rsObserver09, the same replay barrier med-removal-configured.ci uses: two
// peers finish their initial sync and 10.0.0.0/24 is forwarded, then the daemon
// is asked to stop. The metric itself is asserted on the wire by the .ci.
func init() {
	Register("plugin/modify-increment-med-from-route-value",
		rsObserver09("modify-increment-med-from-route-value", 2, "10.0.0.0/24"))
}
