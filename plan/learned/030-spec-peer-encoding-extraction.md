# 030 — Peer Encoding Cleanup

## Objective

Fix silent ORIGINATOR_ID/CLUSTER_LIST data loss in UPDATE building, add `BuildGroupedUnicast` to `UpdateBuilder`, and delete ~500 LOC of dead code from `peer.go`.

## Decisions

- `routeGroupKey` was missing `OriginatorID` and `ClusterList` fields, meaning routes with different reflector attributes could be silently merged — fixed by adding both to the format string.
- `buildStaticRouteUpdate` (old code) never encoded ORIGINATOR_ID/CLUSTER_LIST either; the bug predates the migration. Only `buildGroupedUpdate` got it right. New path inherits the fix.
- `RawAttributes` was deliberately NOT added to `routeGroupKey` — custom attributes are rare and typically unique, so the risk of incorrect grouping is very low.
- VPN grouping bug and LabeledUnicast-in-grouped-path bugs are explicitly deferred (documented as pre-existing limitations).
- AS4_AGGREGATOR (RFC 6793 Section 4.2.3) not implemented — risk assessed as very low (needs ASN4=false + AggregatorASN > 65535 + AGGREGATOR attribute, a rare combination).

## Patterns

- Wire compat tests migrated from "old vs new comparison" (both were broken!) to expected-bytes assertions captured from the correct implementation.
- `BuildGroupedUnicast` uses first route's attributes for the shared path attributes section; all routes contribute NLRI.

## Gotchas

- The comparison in `wire_compat_test.go` was old-vs-new, but BOTH were broken for reflector attrs. The test was passing but proving nothing useful.
- `VPN` and `LabeledUnicast` builders use `packAttributesNoSort()` assuming pre-ordered attributes — fragile but correct at time of writing.

## Files

- `internal/bgp/message/update_build.go` — `UnicastParams` new fields, `BuildGroupedUnicast`
- `internal/reactor/peer.go` — `routeGroupKey` fix, `toStaticRouteUnicastParams` fix, dead code deleted
- `internal/reactor/wire_compat_test.go` — migrated to expected-bytes tests
