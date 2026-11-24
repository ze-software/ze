# Route Policy

## Meta

| Field | Value |
|-------|-------|
| Name | Route Policy |
| Page | docs/guide/route-policy.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/policy, internal/core/rib/locrib, internal/component/sysrib, internal/plugins/redistribute |
| Real-path tests | test/plugin/connected-import-filter-hides.ci, test/plugin/connected-import-filter-hides-forked.ci, test/plugin/rib-import-central-bogons.ci, test/plugin/rib-import-central-bogons-forked.ci, test/plugin/rib-import-tag-internal-internal.ci, test/plugin/rib-import-tag-internal-forked.ci, test/plugin/rib-import-tag-forked-internal.ci, test/plugin/rib-import-tag-forked-forked.ci, test/plugin/static-import-filter-distance.ci, test/plugin/static-import-filter-distance-forked.ci, test/plugin/sysrib-bgp-metric-callers.ci, test/plugin/sysrib-bgp-metric-callers-forked.ci, test/plugin/sysrib-distance-reload.ci, test/plugin/sysrib-distance-reload-forked.ci, test/plugin/sysrib-distance-reload-configured.ci, test/plugin/sysrib-distance-reload-configured-forked.ci, test/plugin/sysrib-ecmp-replay.ci, test/plugin/sysrib-ecmp-replay-forked.ci, test/plugin/sysrib-execution-health.ci, test/plugin/sysrib-execution-health-forked.ci, test/plugin/sysrib-execution-parity.ci, test/plugin/sysrib-execution-parity-forked.ci, test/plugin/sysrib-feed-restart.ci, test/plugin/sysrib-feed-restart-forked.ci, test/plugin/sysrib-forked-producer-kernel.ci, test/plugin/sysrib-nonwinner-explicit-ecmp.ci, test/plugin/sysrib-nonwinner-explicit-ecmp-forked.ci, test/plugin/sysrib-recursive-nexthop.ci, test/plugin/sysrib-recursive-nexthop-forked.ci, test/plugin/sysrib-slow-reader-reset.ci |
| Interop | bgp/isis-import-policy-frr, bgp/isis-import-policy-frr-forked, bgp/ospf-import-policy-bird, bgp/ospf-import-policy-bird-forked, bgp/isis-export-policy-frr, bgp/ospf-export-policy-frr |
| Docs | docs/guide/route-policy.md, docs/architecture/rib/unified-locrib.md, docs/architecture/api/process-protocol.md |
| Doc review | 2026-11-14: source/central admission in locrib/policy.go admit, both readers in sysrib/route_view_client.go openRouteViewMonitoredClient, the worker in route_view_worker.go consume, candidates through route_view_worker_operations.go Candidates and route_view_candidates.go readCandidates, reset in reopen, cleanup in route_view.go cleanupLocked, health in doctor_route_view.go findings and acknowledged output in route_view_output.go publish match docs/guide/route-policy.md ("Seeing the hidden paths", "Internal and external system RIB") and the transport paragraphs of unified-locrib.md; process-protocol.md "System routing views" matches route_view.go routeViewAuthorize and match, the 4096-record and 1000 ms bounds (rpc RouteViewRecordsMax, RouteViewWaitMillisMax), route_view_output.go withdraw and provePublicationReset. |
| Defect review | 2026-11-14: spec-route-policy-1-gate-proof-and-forked-sysrib completed its paired daemon, kernel, lifecycle, peer, discrimination and load proof. Of the forwarding-path rows it feeds, refused-kernel-add retry was fixed by `spec-fib-depth` (`20d642984c`), and backup takeover on link failure moved to `plan/release/fixes/spec-fib-lfa-backup-takeover.md`; none names a Components path. |

## Description

One policy library supplies source import chains for connected, static, OSPF and
IS-IS routes and a central `rib` import chain for every Loc-RIB source. Rejected
paths remain hidden candidates, visible through `show rib candidates`. A policy
reload re-judges stored producer values. Named static tables bypass these
Loc-RIB attachments.
<!-- source: internal/core/rib/locrib/policy.go -- admit, PublishPolicy -->
<!-- source: internal/plugins/static/locrib.go -- applyProgrammed -->

BGP keeps peer import/export chains. Redistribution applies a destination/source
import chain before the destination's IS-IS or OSPF export chain. Each attachment
checks which fields its routes carry and which values its filters can set.
<!-- source: internal/component/policy/fields.go -- CheckChain -->
<!-- source: internal/plugins/redistribute/register.go -- init -->
<!-- source: internal/plugins/isis/redistribute/consumer.go -- InjectRoute -->
<!-- source: internal/plugins/ospf/redistribute/consumer.go -- InjectRoute -->

The system RIB runs internally or externally with one daemon authority and the
same forwarding worker. The external selected projection runs no policy or
cross-protocol selection. Distance updates and forwarding output require daemon
acknowledgment. The real-path tests run each case once with an internal and
once with an external system RIB, against a real kernel in a network namespace
of their own; the four `rib-import-tag` cases also cross the static
producer's placement. A recorded green run, not the list, is what counts toward
a level.
<!-- source: internal/component/sysrib/route_view_client.go -- openRouteViewMonitoredClient -->
<!-- source: internal/component/sysrib/route_view_worker.go -- consume -->
<!-- source: internal/component/sysrib/route_view_output.go -- publish -->
