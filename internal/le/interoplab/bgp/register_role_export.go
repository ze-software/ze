// Design: docs/architecture/testing/interop.md -- role export sets on roleless destinations.
package bgp

import "time"

const roleExportExcludedPrefix = "10.77.5.0/24"

// The paired sources distinguish insertion from observation: bypassing the
// egress member leaks BIRD's route, while failing to observe FRR's Customer
// OPEN withholds the injector's allowed route. Ze originates a third control.
// FRR frr-10.7.1 bgp_open_capability advertises any configured local_role;
// bgp_role_violation rejects missing remote Role only in strict mode.
// bgp_otc_filter (bgpd/bgp_route.c) accepts an unmarked route as Customer and
// sets its OTC locally. BIRD v3.4.0 bgp_update_attrs (proto/bgp/attrs.c) only
// adds OTC for local Provider, Peer or RS, so this unconfigured BIRD sends none.
func init() {
	scenarioOperations["bgp-role-export-roleless-destination-frr"] = []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRRoute, argument: "10.77.4.0/24", timeout: 60 * time.Second},
		{kind: opFRRRoute, argument: injectPrefixFirst, timeout: 60 * time.Second},
		{kind: opBIRDRoute, argument: roleExportExcludedPrefix, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: "ze", command: zeCommand("show bgp rib received"), contains: []string{injectPrefixFirst, roleExportExcludedPrefix}, timeout: 60 * time.Second},
		// recordDrop runs before the export-set rejection returns. FRR is the
		// only send-granted target, and BIRD the only excluded source. This is
		// a completed export decision, not the asynchronous received-RIB fence.
		// BIRD v3.4.0 bgp_fire_tx emits its initial EOR only after
		// bgp_create_update has no UPDATE left. The source worker's FIFO thus
		// cannot produce this fence from EOR ahead of BIRD's static route.
		{kind: opWaitLogContains, peer: "ze", contains: []string{"reason=export-set direction=egress peer=172.30.0.3 peer-role=customer"}, timeout: 60 * time.Second},
		{kind: opFRRRouteAbsent, argument: roleExportExcludedPrefix, timeout: 30 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	}
}
