// Design: docs/architecture/core-design.md — administrative distance and the RIB
// Related: rib_bestchange.go — the stamp site
// Related: internal/core/rib/distance — the seam carrying the declaration to this stamp
// Related: internal/component/sysrib/sysrib.go — parseAdminDistanceConfig, which resolves it

package rib

// DefaultAdminDistanceEBGP and DefaultAdminDistanceIBGP are the distances BGP
// stamps on every locrib.Path it inserts, mirroring spf.DefaultAdminDistance in
// IS-IS (115) and OSPF (110). The values follow the Cisco and Juniper
// convention; RFC 4271 mandates none, and ze-rib-conf.yang says so.
//
// These are NOT the operator's knob. `rib { distance { } }` is the one
// declaration an operator writes, and sysrib applies it in effectivePriority
// before the FIB is programmed. BGP had a second container of its own until
// 2026-09-04, and which of the two decided depended on whether the rib block
// existed at all.
//
// They are the BOOTSTRAP value only, reachable before the first configure has
// run. From then on rib_bestchange.go stamps whatever the declaration says,
// read through internal/core/rib/distance. That indirection is not decoration:
// locrib.selectBest ranks paths on the stamped value and runs before sysrib
// sees the route, so a distance that reaches sysrib alone cannot change
// cross-protocol selection at all.
const (
	DefaultAdminDistanceEBGP uint8 = 20
	DefaultAdminDistanceIBGP uint8 = 200
)
