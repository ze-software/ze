# 575 -- IPv6 MP_REACH UPDATE Forwarding

## Context

IPv6 UPDATEs with MP_REACH_NLRI were not being forwarded to destination peers in iBGP route-reflection scenarios. The wire-level next-hop rewriting was proven correct by existing unit tests, but end-to-end forwarding from receive through to the destination peer was broken. The `.ci` test showed peer-dst receiving 0 UPDATEs (only EOR) and timing out after 10s.

## Decisions

- Created a new `bgp-rr` plugin (Route Reflector, RFC 4456) over reusing `bgp-rs` (Route Server, RFC 7947), because they are semantically different protocols. The reactor already handles all RFC 4456 mechanics (ORIGINATOR_ID, CLUSTER_LIST, client/non-client filtering), so the plugin is a thin forwarding trigger.
- Used `cache forward *` (forward to all peers) over explicit target selection, because the reactor's ForwardUpdate already implements the RFC 4456 forwarding matrix.
- Asserted ORIGINATOR_ID presence (`800904`) in the `.ci` test over NLRI-only check, because ORIGINATOR_ID is never in the source UPDATE and proves route reflection is active.

## Consequences

- `bgp-rr` includes replay on peer-up (via adj-rib-in) with convergent delta loop. EOR is sent only after non-empty replay; the reactor handles initial EOR on empty sessions.
- `bgp-rr` includes withdrawal tracking on peer-down via per-source-peer NLRI map (same pattern as bgp-rs). Wire path only; text-path (fork-mode fallback) does not track withdrawals.
- `applyNextHopMod` now emits both legacy NEXT_HOP (type 3) and MP_REACH_NLRI (type 14) ops for IPv4 local addresses. The MP_REACH op uses IPv4-mapped IPv6 (::ffff:a.b.c.d). The reverse case (IPv6 local, IPv4 routes) still needs a paired address config.

## Gotchas

- The root cause was none of the three hypotheses in the spec. The `.ci` test simply had no forwarding plugin loaded. Without `bgp-rr` or `bgp-rs`, the reactor receives UPDATEs but nothing triggers forwarding.
- ze-peer mirrors the OPEN it receives with last byte of router-id incremented. Connection ordering determines which peer gets which mirrored router-id, making exact ORIGINATOR_ID/CLUSTER_LIST values unpredictable in tests.
- Each `expect=bgp:conn=N:seq=M:contains=` line in `.ci` tests consumes a separate message. Multiple `contains=` checks on a single-message flow cause timeouts.
- Replay on state-up must not send EOR with empty replay (replayed==0). The reactor sends initial EOR at session establishment. Sending EOR from the plugin before the peer reaches Established causes "invalid FSM state" errors. Only send EOR after non-empty replay.

## Files

- `internal/component/bgp/plugins/rr/register.go` -- bgp-rr registration (RFC 4456)
- `internal/component/bgp/plugins/rr/rr.go` -- route reflector plugin with replay + withdrawal
- `internal/component/bgp/plugins/rr/withdrawal.go` -- NLRI tracking and peer-down withdrawal
- `internal/component/bgp/reactor/reactor_api_forward.go` -- mixed-family next-hop fix
- `internal/component/bgp/reactor/filter_delta_handlers_test.go` -- applyNextHopMod unit tests
- `internal/component/plugin/all/all.go` -- added rr import
- `test/plugin/nexthop-self-ipv6-forward.ci` -- bgp-rr config + ORIGINATOR_ID assertion
