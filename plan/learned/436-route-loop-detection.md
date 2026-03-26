# 436 -- Route Loop Detection

## Context

Ze had no loop detection for received BGP UPDATE messages. Routes with the local ASN
in the AS_PATH, or reflected routes with matching ORIGINATOR_ID or CLUSTER_LIST, would
pass through validation and reach plugins/RIB. This violates RFC 4271 Section 9 and
RFC 4456 Section 8, and could cause routing loops in iBGP/route-reflector topologies.

## Decisions

- Chose a single `detectLoops` function walking attributes once, over three separate
  functions (spec originally planned `detectASLoop`, `detectOriginatorIDLoop`,
  `detectClusterListLoop`). One pass is more efficient and the call site is simpler.
- Placed loop detection after RFC 7606 validation and before prefix limit counting.
  Looped routes should not count toward prefix limits (they are treated as withdrawn).
- Used Router ID as default Cluster ID per RFC 4456 Section 7, over adding explicit
  cluster-id YANG configuration (deferred to a future spec if needed).
- Compared ORIGINATOR_ID as uint32 directly from wire bytes, over converting through
  `attribute.ParseOriginatorID` (which returns `netip.Addr`). Avoids unnecessary
  type conversion since RouterID in PeerSettings is already uint32.
- AS loop detection applies to both eBGP and iBGP (RFC 4271 makes no distinction).
  ORIGINATOR_ID and CLUSTER_LIST checks apply only to iBGP (RFC 4456 scope).

## Consequences

- Routes with loops are now silently dropped before reaching plugins or RIB.
- Future `allow-own-as N` configuration can be added by modifying the AS loop check
  to count occurrences rather than returning on first match.
- Future explicit `cluster-id` configuration requires adding a ClusterID field to
  PeerSettings and using it instead of RouterID in the cluster-list check.
- Extended ze-peer `RouteToSend` with ASPath, OriginatorID, ClusterList fields for
  functional tests. This infrastructure is reusable for future route-reflection tests.

## Gotchas

- Session without negotiated capabilities defaults to `asn4=false` (2-byte ASN parsing).
  Test data must match: build AS_PATH with 2-byte ASNs when session has no ASN4 negotiation.
- The `parseEventTypeList` function referenced in `event_monitor.go` is undefined (pre-existing
  build break from another session's uncommitted work). This blocks `make ze-lint` but not
  the reactor package tests.

## Files

- `internal/component/bgp/reactor/session_validation.go` -- `detectLoops` function
- `internal/component/bgp/reactor/session_read.go` -- pipeline integration
- `internal/component/bgp/reactor/session_validate_test.go` -- 12 unit tests
- `internal/test/peer/message.go` -- extended RouteToSend
- `internal/test/peer/expect.go` -- extended send-route parser
- `rfc/short/rfc4456.md` -- RFC summary
- `test/plugin/loop-{as,originator-id,cluster-list}.ci` -- functional tests
- `docs/features.md` -- Route Loop Detection section
- `docs/architecture/route-selection.md` -- validation reasons 8-10
