# 1151 -- rib-arch-3: RFC 5549 Extended Next-Hop for Injected Routes

## Context

`request bgp rib inject` (`internal/component/bgp/plugins/rib/rib_commands.go`
`injectRoute`) inserts a route into adj-rib-in with no live session, for testing the
forwarding path. It accepted an IPv6 next-hop but then **discarded** it ("IPv6 nhop
accepted but not stored in NEXT_HOP attr") because the legacy NEXT_HOP attribute (type 3)
is IPv4-only. So an injected IPv4 NLRI could not carry an RFC 5549/8950 extended next-hop
(IPv4 reachable via IPv6), and a native IPv6 NLRI inject also lost its next-hop. The
receive/parse side already supported RFC 5549; this was the symmetric inject/encode gap.

## Decisions

- Carry the IPv6 next-hop in an **MP_REACH_NLRI attribute** stored inside the injected
  route's attribute block, mirroring the receive path (`rib_structured.go`): the
  MP_REACH lives in the attribute bytes, the NLRI stays the separate storage key. Chosen
  over adding an MP_REACH method to `attribute.Builder` (which deliberately has none) --
  instead build it with the existing `attribute.NewMPReachNLRI` + `attribute.WriteAttrTo`
  and append the full attribute wire.
- Reused the existing send-side RFC 5549 encoder rather than writing a new one: the triage
  anchor (`PackContext.ExtendedNextHop`) was stale/gone, but `commit.go`
  `useTraditionalNLRI` already routes IPv4/unicast + non-IPv4 next-hop to
  `buildMPReachNLRI` (`:337`->`:484`). Once the stored route's recovered next-hop is IPv6,
  the forward path emits RFC 5549 automatically.
- Applied the RFC 8950 `ExtendedNextHop` capability check (`validateIPv6NextHop`) only for
  the cross-family case (`fam.AFI == IPv4`); a native IPv6 NLRI + IPv6 next-hop is ordinary
  MP-BGP and needs no such capability -- the pre-existing code wrongly required it there.

## Consequences

- Injected IPv4-over-IPv6 (RFC 5549) and native-IPv6 routes now forward with the correct
  next-hop. The extended-next-hop forwarding path is now exercisable end-to-end from inject.
- The next-hop is recovered from stored attributes (`bestCandidateNextHopAddr` ->
  `extractMPNextHopAddr`, `rib_bestchange.go`), not a stored scalar -- consistent with
  how received MP routes work.
- The "received" adj-rib-in show renders legacy NEXT_HOP only, not MP next-hops, so
  `show bgp rib received` shows no next-hop for these routes (received IPv6/MP routes
  already display the same way); `show bgp rib best` (which uses `extractMPNextHopAddr`)
  does render it.

## Gotchas

- `extractMPNextHopAddr` reads the storage's NORMALIZED `OtherAttrs` format
  (`[type][flags][len16][value]`), not raw BGP wire (`[flags][type][len]`). The inject wire
  MP_REACH is normalized by `ParseRouteEntry` into that format on Insert, so recovery works
  -- the same round-trip received MP routes rely on.
- `rib_commands.go` is now 1078 lines; it was already over the 1000-line modularity
  threshold before this ~30-line change (the change did not introduce the largeness).

## Files

- `internal/component/bgp/plugins/rib/rib_commands.go` -- `injectRoute` MP_REACH emission
- `internal/component/bgp/plugins/rib/rib_commands_test.go` -- 2 unit tests (RED->GREEN)
- `test/plugin/rib-inject-rfc5549.ci` -- functional test (PASS)
