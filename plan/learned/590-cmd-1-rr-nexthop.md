# 590 -- Route Reflection and Next-Hop Control

## Context

Ze's BGP peer configuration lacked route-reflector-client, cluster-id, and next-hop control (self/unchanged/auto/explicit IP). These are the two most critical missing features for production iBGP deployments. Without RR, iBGP requires a full mesh. Without next-hop control, operators cannot set next-hop self on RR clients (the most common iBGP pattern).

## Decisions

- YANG leaves placed in `session` container of `peer-fields` grouping, inheritable at bgp/group/peer levels.
- Next-hop as a union type (enum {auto, self, unchanged} + ip-address) over separate boolean flags, because the modes are mutually exclusive and the explicit-IP case needs a value.
- RR forwarding rules (RFC 4456) implemented in the reactor's `ForwardUpdate`, not in a separate plugin, because the forwarding decision matrix (client/non-client/eBGP) is a core routing concern.
- ORIGINATOR_ID and CLUSTER_LIST added via `AttrModHandler` registry (same pattern as AS-PATH prepend), over inline modification in the forwarding loop.
- IPv6 next-hop rewriting uses a type-14 `mpReachNextHopHandler` (approach 1: same ModAccumulator pattern), over a separate rewrite function.
- cluster-id defaults to router-id via `EffectiveClusterID()` method on PeerSettings, matching RFC 4456 Section 7.

## Consequences

- iBGP deployments can now use route reflection (full mesh no longer required).
- Operators can control next-hop per-peer for both IPv4 and IPv6 families.
- The `AttrModHandler` registry is now the established pattern for all egress attribute modification (ORIGINATOR_ID, CLUSTER_LIST, next-hop, community stripping, AS-override all use it).
- Wire-level forwarding .ci tests for RR are blocked by a bgp-rr replay timing issue when using single-ze-peer with multiple IPs. Config acceptance and RIB storage tests exist instead.

## Gotchas

- Phase 3b (cluster-id sync between session and loop-detection) referenced in the spec was never verifiable. The `loop-detection/cluster-id` leaf doesn't exist in YANG. The claim may be stale.
- The spec claimed 16 unit subtests but only ~10 were found. Subtest counts in specs are unreliable.
- IPv6 next-hop rewriting was listed as "What Remains" in the spec but was already fully implemented with `mpReachNextHopHandler`. Spec was stale for months.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- YANG leaves
- `internal/component/bgp/reactor/peersettings.go` -- PeerSettings fields
- `internal/component/bgp/reactor/config.go` -- parsePeerFromTree extraction
- `internal/component/bgp/reactor/reactor_api_forward.go` -- RR forwarding rules, applyNextHopMod
- `internal/component/bgp/reactor/filter_delta_handlers.go` -- originatorIDHandler, clusterListHandler, mpReachNextHopHandler
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- HandleBgpPeerDetail with RR/NH fields
- `test/parse/rr-nexthop-config.ci`, `test/plugin/rr-basic.ci`, `test/plugin/nexthop-self.ci`, `test/plugin/nexthop-unchanged.ci`
