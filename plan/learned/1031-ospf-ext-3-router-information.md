# 1031 - OSPF Router Information LSA (RFC 7770)

## Context

Dual-address-family RI LSA advertising router-wide capabilities. OSPFv2: an
Opaque type-4 LSA (a consumer of the ext-1 opaque carrier). OSPFv3: a NATIVE LSA
with function code 12 (RFC 5340 carries extensions as native LSAs, not opaque).
One shared RI TLV codec + a `registerRITLV(tlvType, scope, build)` hook so ext-5
(Segment Routing) can inject SR-Algorithm/SRGB/SRLB TLVs into the same RI LSA
without the RI code naming SR.

## Decisions

- Single shared body builder (`buildRIInstances`) for both AFs, so the v2 opaque and v3 native RI carry identical TLV bytes (AC-11).
- The `registerRITLV` hook is UNEXPORTED (the SR consumer lives in-package), keyed by TLV type; type-1 Informational Capabilities TLV emitted first, then registered builders in ascending type order, panic-recovered.
- GR capability bits derived from an injectable `riGRState` engine seam (defaults false) because Ze has no Graceful Restart yet; ext-9 wires it.
- OSPFv3 RI uses U-bit-SET wire types 0x800C (link) / 0xA00C (area) / 0xC00C (AS), NOT the spec's literal 0x200C/0x400C. RFC 7770 §2.2 requires U=1 (unknown routers still flood it); RFC 5340 §4.4.1 confines a U=0 unknown LSA to link-local. `Known()` recognizes RI by function code regardless of the U-bit so either encoding is accepted on receive.

## Consequences

- Future RI capability bits and SR TLVs attach via the shared builder + registerRITLV; no new LSA plumbing.
- The `ASExternal()`/`ASWide()` split (below) is now the canonical way to ask "function code 5?" vs "floods AS-wide?" for every LS type; use `ASWide()` for store/flood/stub-suppression scope and `ASExternal()` only for the literal Type-5 / v3-external question.
- OSPFv3 RI reception works via scope-based store routing, not a `Known()` gate (v3 `Known()` has no production caller yet; the v2 codec is its only caller).

## Gotchas

- The broad pre-existing `ASExternal()` (true for Type 5 AND anything with the v3 AS scope bits) would have mis-fed an AS-scope RI LSA (0xC00C) into SPF external computation / false ASBR detection. Split into precise `ASExternal()` (function code 5 only) + new `ASWide()` (AS-wide store/flood semantics); migrate every AS-wide-semantics caller to `ASWide()`, keep function-code-5 callers on `ASExternal()`. Behavior-preserving for all pre-existing types.
- Default RI scope is [area, AS]. The AS-scope branch also emits an area-scoped RI into each attached NSSA (RFC 7770 §2.7, since Type-11 cannot flood into an NSSA). Guard that fallback with `!HasScope(area)`, else the NSSA is double-emitted (the Area-scope branch already covered it) and `ze_ospf_ri_originations_total{scope=area}` double-counts (benign: the carrier dedups the body, so no double-flood).
- rfc/short/rfc7770.md originally listed the U=0 hex values while asserting U=1; a future agent could "fix" the code to the wrong values. Corrected the doc to the U=1 values.

## Files

- `internal/plugins/ospf/ri.go`, `ri_registry.go`, `ri_show.go`, `origination_v6_ri.go`, `packet/ri_tlv.go`, `lsdb/native_view.go`
- `internal/plugins/ospf/{instance,origination_v6,config,cmd_show,register}.go`, `types/lstype.go` (ASExternal/ASWide split), `v3/types/lsa.go`, `lsdb/{lsdb,origination,flooding}.go`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-ri-*.ci`, `ospf6-ri-originate.ci`, `test/interop/scenarios/{ospf-ri-frr,ospf6-ri-frr}/`
- `rfc/short/rfc7770.md`, `docs/guide/ospf.md`
