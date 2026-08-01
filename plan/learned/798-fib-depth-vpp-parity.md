# 798 -- fib-depth-vpp-parity

## Context

The kernel FIB backend had a richRouteBackend interface supporting route type (blackhole/unreachable/prohibit), metric, per-route table ID, ECMP multipath, MPLS lwtunnel, and SRv6 seg6 encap. The VPP backend only supported single-path routes and ECMP via a separate addMultiPath code path, with no route type handling, no metric propagation, and single-path routes ignoring per-change TableID.

## Decisions

- Mirrored the kernel's richRouteBackend pattern with vppRichRoute struct and addRichRoute/delRichRoute/replaceRichRoute methods, keeping the same dispatch logic (hasRichFields triggers rich path) over extending the existing addMultiPath because addMultiPath lacked route type and metric handling
- Mapped RouteType to VPP FibPathType (blackhole->DROP, unreachable->ICMP_UNREACH, prohibit->ICMP_PROHIBIT) rather than setting NPaths=0, because VPP needs an explicit path type to generate correct ICMP responses
- Did not map Metric to FibPath.Weight because they are semantically different (kernel route.Priority is route selection priority; VPP FibPath.Weight is per-path ECMP weighting). ECMPPath.Weight flows through directly.
- Removed addMultiPath from the interface entirely (dead code after rich route subsumes its functionality) over keeping it as a legacy fallback
- Tracked installedRoute{nextHop, tableID} per prefix instead of just a string, because sysrib withdraw events carry TableID=0 even when the route was installed in a non-default table. The delete path looks up the stored tableID.
- Renamed SDK command "rib show" to "show rib" for action-before-identifier grammar consistency, adding "show nexthop-table" and "show ecmp-groups" following the same pattern

## Consequences

- VPP and kernel backends now handle the same BestChangeEntry fields (AC-6/7/8/9 parity), except VPP has no route priority equivalent (no kernel-like metric mapping)
- flushRoutes correctly deletes routes from non-default VPP tables on shutdown
- The show nexthop-table command exposes resolver tracking state for debugging recursive NH resolution issues without needing to inspect internal state
- Three existing functional tests (fib-sysrib.ci, fib-ecmp.ci, fib-metric.ci) updated for the renamed command

## Gotchas

- sysrib withdraw events only set Action and Prefix; TableID is always 0. The VPP delete path must look up the stored tableID from the installed map, not trust the incoming change.
- VPP route delete by prefix+table must match the table the route was installed in. A delete to the wrong table silently succeeds (VPP returns retval=0 for "route not found").
- richRouteAddDel can produce zero paths if RouteType is unicast, NextHop is invalid, and ECMPPaths is empty. This happens when hasRichFields triggers on Metric or TableID alone. Added a guard returning an error for zero paths.
- showNHTable calls nhResolver.Resolve() while holding nhResolver.mu.RLock(). Safe today because Resolve() only acquires Loc-RIB shard locks (different lock hierarchy) and the cascade worker uses a channel to avoid acquiring sysRIB.mu under the shard write lock.

## Files

- `internal/plugins/fib/vpp/backend.go` -- vppRichRoute, richRouteAddDel, routeTypeToVPP, mock recording types
- `internal/plugins/fib/vpp/fibvpp.go` -- hasRichFields dispatch, installedRoute tracking, delVPPRoute stored-tableID lookup
- `internal/plugins/fib/vpp/fibvpp_test.go` -- 7 new tests (route type, metric, table, stored-tableID delete, update)
- `internal/component/sysrib/sysrib.go` -- showNHTable(), showECMPGroups()
- `internal/component/sysrib/register.go` -- show rib/nexthop-table/ecmp-groups commands
- `test/plugin/fib-recursive.ci` -- functional test for recursive NH and all three show commands
- `docs/architecture/core-design.md` -- NH resolution layer and rich route backend docs
- `docs/comparison.md` -- recursive NH, IGP cost rows; ECMP Partial->Yes
- `docs/features.md` -- Route Installation updated with full FIB pipeline description
