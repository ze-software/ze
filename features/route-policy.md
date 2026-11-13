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
| Interop | bgp/isis-import-policy-frr, bgp/isis-import-policy-frr-forked, bgp/ospf-import-policy-bird, bgp/ospf-import-policy-bird-forked, bgp/isis-export-policy-frr, bgp/ospf-export-policy-frr |
| Docs | docs/guide/route-policy.md, docs/architecture/rib/unified-locrib.md, docs/architecture/api/process-protocol.md |
| Doc review | 2026-11-13: source/central admission in locrib/policy.go admit, both readers in sysrib/route_view_client.go openRouteViewMonitoredClient, full candidates in route_view_candidates.go readCandidates and acknowledged output in route_view_output.go publish match the linked pages. |
| Defect review | 2026-11-13: plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md remains open for paired daemon/kernel, lifecycle, peer, discrimination and load proof. No Supported claim or recorded green feature run is made. |

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
acknowledgment. This feature remains Experimental while paired daemon, kernel,
lifecycle and peer validation is incomplete. Listed scenarios identify the
proof paths, not completed runs. Functional drafts do not count as retained
real-path evidence until promotion and a recorded run.
<!-- source: internal/component/sysrib/route_view_client.go -- openRouteViewMonitoredClient -->
<!-- source: internal/component/sysrib/route_view_worker.go -- consume -->
<!-- source: internal/component/sysrib/route_view_output.go -- publish -->
