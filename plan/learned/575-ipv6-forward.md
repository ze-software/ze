# 575 -- IPv6 MP_REACH UPDATE Forwarding

## Context

IPv6 UPDATEs with MP_REACH_NLRI were not being forwarded to destination peers in iBGP route-reflection scenarios. The wire-level next-hop rewriting was proven correct by existing unit tests, but end-to-end forwarding from receive through to the destination peer was broken. The `.ci` test showed peer-dst receiving 0 UPDATEs (only EOR) and timing out after 10s.

## Decisions

- Created a new `bgp-rr` plugin (Route Reflector, RFC 4456) over reusing `bgp-rs` (Route Server, RFC 7947), because they are semantically different protocols. The reactor already handles all RFC 4456 mechanics (ORIGINATOR_ID, CLUSTER_LIST, client/non-client filtering), so the plugin is a thin forwarding trigger.
- Used `cache forward *` (forward to all peers) over explicit target selection, because the reactor's ForwardUpdate already implements the RFC 4456 forwarding matrix.
- Asserted ORIGINATOR_ID presence (`800904`) in the `.ci` test over NLRI-only check, because ORIGINATOR_ID is never in the source UPDATE and proves route reflection is active.

## Consequences

- `bgp-rr` is a minimal plugin (~300 lines). It lacks withdrawal tracking on peer-down and replay on peer-up (which `bgp-rs` has). These would be needed for production RR deployments with peer churn.
- AC-2 (next-hop rewrite for IPv6) is partially met: `applyNextHopMod` with an IPv4 local address only rewrites legacy NEXT_HOP (type 3), not MP_REACH_NLRI (type 14). This is a known limitation documented at `reactor_api_forward.go:758-765`. A proper fix requires paired IPv4/IPv6 local addresses in peer config.

## Gotchas

- The root cause was none of the three hypotheses in the spec. The `.ci` test simply had no forwarding plugin loaded. Without `bgp-rr` or `bgp-rs`, the reactor receives UPDATEs but nothing triggers forwarding.
- ze-peer mirrors the OPEN it receives with last byte of router-id incremented. Connection ordering determines which peer gets which mirrored router-id, making exact ORIGINATOR_ID/CLUSTER_LIST values unpredictable in tests.
- Each `expect=bgp:conn=N:seq=M:contains=` line in `.ci` tests consumes a separate message. Multiple `contains=` checks on a single-message flow cause timeouts.

## Files

- `internal/component/bgp/plugins/rr/register.go` -- bgp-rr registration (RFC 4456)
- `internal/component/bgp/plugins/rr/rr.go` -- route reflector plugin
- `internal/component/plugin/all/all.go` -- added rr import
- `test/plugin/nexthop-self-ipv6-forward.ci` -- updated with bgp-rr config + ORIGINATOR_ID assertion
