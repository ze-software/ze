# 1050 - OSPF Segment Routing SR-MPLS (RFC 8665 IPv4 + RFC 8666 IPv6)

## Context

SR-MPLS for OSPF, both address families: advertise SRGB/SRLB + SR-Algorithm
(shared RI capabilities), originate node Prefix-SIDs and per-adjacency Adj-SIDs,
decode remote peers' SR state, and after each SPF run install the MPLS
push/swap/pop forwarding entries into the mpls-fib bus. IPv4 rides the RFC 7684
Extended Prefix/Link Opaque LSAs (a consumer of ext-1's opaque carrier + ext-3
Router-Information + ext-4 prefix/link sub-TLV hooks); IPv6 rides the RFC 8666
Extended-LSA TLVs. The control-plane state machines are SHARED (AF-neutral
engine, plan/learned/972); only the SR TLV wire carriage is AF-specific.

## Decisions

- Label maths: `sr.SRGB.Label(index)` = `base+index` across the ordered range
  list; `srlb` is a bounded allocator for Adj-SIDs. Shared RI capabilities
  (SRGB/SRLB/algorithms) are stored once per router (RFC 8666 §4); per-AF
  Prefix-SID prefix sets are stored separately.
- FORWARDING LABEL (the load-bearing correctness point): the out-label for a
  Prefix-SID is computed from the SPF **NEXT-HOP** router's advertised SRGB
  (RFC 8665 §5: `SRGB(next-hop).Label(index)`), and the NP/E/M PHP/Explicit-NULL
  flags (`OutgoingActionFor`) apply ONLY at the penultimate hop
  (`nh.Router == rs.Originator`); at any transit hop the label is swapped
  unconditionally (`ActionKeep`). This required threading the first-hop router
  ID through SPF: `spf.NextHop` gained a `Router types.RouterID` field, set at
  `nextHopsForP2P`/`nextHopsForNetwork` only for root-attached hops and inherited
  by deeper vertices via `inheritedHops`, propagating through
  inter-area/external/transit route decoration (all struct-copy paths).
- Dual-stack Prefix-SID origination: `srWireStore` keeps the shared config in
  `caps` and a per-IPv6 override in `v6caps`; `getAF(router, isV6)` reads the v6
  override when present, else the shared block. The override is populated only
  when BOTH families configure SR.
- Received SRGB/SRLB ranges are bounds-checked in the single `DecodeRangeValue`
  path: reject base `< 16` (reserved), base `> 2^20-1`, or `base+size-1 >
  MaxLabel`; a bad range is skipped (counted) without discarding the rest.
- The OSPFv3 grace/extended-LSA TLV iterator tolerates a missing FINAL pad
  (clamps the advance to end-of-region) to match the OSPFv2 SR sub-TLV iterator,
  so both AFs accept the same wire; a truncated header or an overrunning length
  is still rejected.

## Consequences

- `spf.NextHop` gaining `Router` is a backward-compatible struct-field addition;
  every next-hop merge/cap/decorate path already copies the struct, so the
  first-hop router ID reaches inter-area, external, and transit routes for free.
- mpls-fib SR producers: OSPF-SR is producer 3, OSPFv3-SR is producer 4
  (RSVP-TE=1, LDP=2). Kernel validates labels `<= 2^20-1`.
- `newEngineWithCodecAF(t, codec, af)` (ext-15) is the SR engine ctor seam;
  the v6 SR paths run under the per-AF v6 engine set.

## Gotchas

- LANDMINE (spec was wrong): the spec mandated computing the label from the
  ORIGINATOR's SRGB and applying PHP at every hop. That blackholes any
  heterogeneous-SRGB deployment (works only when every node shares one SRGB, the
  common/tested case). Correct SR-MPLS uses the NEXT-HOP's SRGB and PHP only at
  the penultimate hop. When a spec's forwarding rule contradicts the RFC, follow
  the RFC and record the deviation.
- LANDMINE (stale per-AF override): `srWireStore.set`'s ENABLED branch does NOT
  touch `v6caps` (only the disabled branch clears it). So `applySRConfig` must
  EXPLICITLY clear the v6 override (`setV6(router, sr.SRConfig{})`) whenever the
  IPv6 SR block is absent while SR stays enabled -- otherwise removing the IPv6
  SR block (keeping IPv4) leaves a stale override and the withdrawn IPv6 node
  Prefix-SID keeps originating. `srTestReset` clears the store between cases and
  MASKS this: the regression test must reconfigure IN PLACE (two `applySRConfig`
  calls, no reset) to catch it.
- A next-hop that advertised no SRGB is not SR-capable: skip the install, never
  push a garbage label.
- Received SRGB base `< 16` is a reserved-label push if not rejected before use.

## Files

- `internal/plugins/ospf/sr/{srgb,srlb,codec,codec_v6,config,install}.go` (+ tests)
- `internal/plugins/ospf/sr_{install,adjsid,config,fib,snapshot,origination_v6,reception_v6,interarea_v6,metrics,doctor}.go`, `sr.go`, `register_sr.go` (+ tests)
- `internal/plugins/ospf/spf/spf.go` (`NextHop.Router`, set at the two first-hop resolution points)
- `internal/plugins/ospf/v3/packet/{tlv,lsa_extended}.go`, `v3/types/lsa.go` (Extended-LSA constants)
- `rfc/short/rfc8665.md`, `rfc/short/rfc8666.md`, `docs/guide/ospf.md`
- `test/ospf/ospf-sr-*.ci`, `test/ospfv3/ospfv3-sr-*.ci`, `test/interop/scenarios/ospf-sr-*frr/`
