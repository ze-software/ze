// Design: docs/architecture/rib/unified-locrib.md -- the Path next-hop model
// Related: ../locrib/candidate.go -- Path.ECMP, the equal-cost group built from these
// Related: ../../../component/sysrib/events/events.go -- ECMPPath, the FIB-facing form

// Package nexthop carries the one description of a forwarding target that
// crosses every RIB boundary: the Loc-RIB Path, the best-change event the BGP
// RIB emits, and the multipath group sysrib hands the FIB plugins.
//
// It is a leaf package for the same reason routetype and distance are. The
// Loc-RIB store and the best-change event contract both need the value, and
// neither may depend on the other: the contract is what a forked plugin decodes
// and the store is engine-only state. A type each package declared for itself
// would be two descriptions of one thing, disagreeing the first time one gained
// a field (ai/rules/principles.md).
package nexthop

import "net/netip"

// NextHop is one forwarding target: a gateway address, an outgoing interface,
// or both. One value carries the three facts the FIB needs about a next-hop,
// rather than three parallel slices that can differ in length.
//
// Comparable by ==, which the Loc-RIB relies on to tell a reweight or a device
// move from an unchanged group.
type NextHop struct {
	// Addr is the gateway address. An invalid Addr means Interface alone names
	// the next-hop, which is how an unnumbered or point-to-point device is
	// configured.
	Addr netip.Addr

	// Interface is the outgoing device name, empty when Addr alone names the
	// next-hop, which is every protocol-learned next-hop. The NAME rather than
	// the kernel ifindex crosses this boundary: the index is not stable across a
	// device replacement, and the resolver that maps one to the other lives in
	// the FIB plugin that programs the route.
	Interface string

	// Weight is this next-hop's share of a weighted multipath group. Zero means
	// the producer states no weight, and the FIB gives every member of the group
	// an equal share.
	//
	// The width is the kernel's: rtnh_hops in an RTA_MULTIPATH entry is one
	// octet, and VPP's fib_path weight is one octet too, so 255 is the largest
	// share either data plane can express.
	Weight uint8
}
