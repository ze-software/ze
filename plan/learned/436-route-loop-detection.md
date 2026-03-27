# 436 -- Route Loop Detection

## Context

Ze had no loop detection for received BGP UPDATE messages. Routes with the local ASN
in the AS_PATH, or reflected routes with matching ORIGINATOR_ID or CLUSTER_LIST, would
pass through validation and reach plugins/RIB. This violates RFC 4271 Section 9 and
RFC 4456 Section 8, and could cause routing loops in iBGP/route-reflector topologies.

## Decisions

- Implemented as an ingress filter (`reactor/filter/loop.go`) registered via the plugin
  registry, over hardcoding in the session pipeline. This follows the OTC pattern and
  makes `allow-own-as` a matter of not injecting the filter rather than special-case code.
- Established `reactor/filter/` as the location for protocol-mandated ingress filters,
  with `filter/` subfolder convention matching `schema/` in plugins.
- Extended `PeerFilterInfo` with `LocalAS`, `RouterID`, `ASN4` so ingress filters can
  make per-peer decisions without importing reactor internals.
- Single `LoopIngress` function walking attributes once, over three separate functions.
  One pass is more efficient and the call site is simpler.
- Used Router ID as default Cluster ID per RFC 4456 Section 7, over adding explicit
  cluster-id YANG configuration (deferred to a future spec if needed).
- Compared ORIGINATOR_ID as uint32 directly from wire bytes, over converting through
  `attribute.ParseOriginatorID` (which returns `netip.Addr`). RouterID is already uint32.
- AS loop detection applies to both eBGP and iBGP (RFC 4271 makes no distinction).
  ORIGINATOR_ID and CLUSTER_LIST checks apply only to iBGP (RFC 4456 scope).

## Consequences

- Routes with loops are silently dropped by the ingress filter chain before reaching
  plugins or RIB. They do count toward prefix limits (ingress filters run after prefix
  counting in the current pipeline).
- Future `allow-own-as N`: don't register the loop filter for that peer, or modify the
  filter to count occurrences rather than rejecting on first match.
- Future `cluster-id` config: add ClusterID to PeerFilterInfo, use instead of RouterID.
- `reactor/filter/` is the home for future protocol-mandated filters (AS_PATH length
  limits, next-hop validation, etc.). Policy filters stay in their own plugins.
- OTC's filter functions should eventually move to `plugins/role/filter/` for consistency.

## Gotchas

- PeerFilterInfo.ASN4 must be populated from the peer's negotiated capabilities at the
  ingress filter call site (`reactor_notify.go`). Without it, AS_PATH parsing defaults
  to 2-byte ASNs and misses 4-byte ASN loops.
- Test data must match ASN4 setting: build AS_PATH with 2-byte ASNs when PeerFilterInfo
  has ASN4=false (the default for sessions without negotiated capabilities).

## Files

- `internal/component/bgp/reactor/filter/loop.go` -- LoopIngress filter function
- `internal/component/bgp/reactor/filter/loop_test.go` -- 12 unit tests
- `internal/component/bgp/reactor/filter/register.go` -- init() registration
- `internal/component/plugin/registry/registry.go` -- PeerFilterInfo extended
- `internal/component/bgp/reactor/reactor_notify.go` -- populates new PeerFilterInfo fields
- `internal/component/plugin/all/all.go` -- imports reactor/filter
- `internal/test/peer/message.go` -- extended RouteToSend
- `internal/test/peer/expect.go` -- extended send-route parser
- `rfc/short/rfc4456.md` -- RFC summary
- `test/plugin/loop-{as,originator-id,cluster-list}.ci` -- functional tests
- `docs/features.md` -- Route Loop Detection section
- `docs/architecture/route-selection.md` -- validation reasons 8-10
