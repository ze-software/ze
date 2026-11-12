// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_state.go -- the fast path's state, which reports the gate's verdict
// Related: espform_backend_linux.go -- the XFRM backend, which implements the holder
// Related: espform_fastpath_gate_linux.go -- the gate a hold shuts

package dataplane

// ESPFormFirewallHolder is implemented by a backend whose dual-form receiver has a fast
// path. The fast path rewrites bare ESP to ESP-in-UDP only while no nftables rule on
// the input path can drop ESP, and it learns of a foreign commit only after it landed.
// Ze's own firewall commits close that window: the firewall takes a hold before it
// commits and releases it after.
//
// A backend that does not implement it has no fast path, so there is nothing to hold.
type ESPFormFirewallHolder interface {
	// HoldESPFormFastPath shuts the fast path's gate before it returns, so no datagram
	// is rewritten while the commit lands. The release it answers MUST be called once,
	// after the commit returned, whether it succeeded or not; the fast path then
	// judges the new ruleset before it opens the gate again. It never blocks on the
	// fast path's start, stop or worker, and calls nothing of the firewall, so a
	// caller may hold its own reconcile lock. Safe for concurrent use.
	HoldESPFormFastPath() (release func())
}
