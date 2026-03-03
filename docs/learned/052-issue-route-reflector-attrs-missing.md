# 052 — Route Reflector Attributes Missing in MVPN/FlowSpec/MUP

## Objective

Document (as a low-priority issue) that ORIGINATOR_ID and CLUSTER_LIST (RFC 4456) are missing from MVPNParams, FlowSpecParams, and MUPParams, preventing their use with route reflector configurations.

## Decisions

- Low priority: RR deployments with MVPN/FlowSpec/MUP are rare; no existing tests require this
- Proposed fix: add `OriginatorID uint32` and `ClusterList []uint32` fields plus encoding in each Build* method — following the existing UnicastParams pattern
- RFC 4456 RR logic (prepend cluster ID, don't overwrite ORIGINATOR_ID) belongs in the reactor, not the builder

## Patterns

- MVPN's `BuildMVPN` takes a slice; it uses `routes[0]` for shared attributes — adding per-route reflector attrs here only works if all routes in the batch share the same attrs

## Gotchas

- MVPN/FlowSpec/MUP are part of a broader pattern: these advanced SAFIs are missing many attributes (AS_PATH, COMMUNITIES, AGGREGATOR, etc.) that unicast supports — the reflector attrs are just one symptom
- FlowSpec uses `CommunityBytes []byte` (raw) rather than `Communities []uint32` — any fix must be consistent with that pattern

## Files

- `internal/bgp/message/update_build.go` — MVPNParams, FlowSpecParams, MUPParams structs and Build* methods (fix not yet applied)
