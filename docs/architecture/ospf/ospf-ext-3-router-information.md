# Router Information LSA

RFC 7770 Router Information for both address families. OSPFv2 carries it as an
Opaque type-4 LSA, a consumer of the opaque carrier. OSPFv3 carries it as a
NATIVE LSA with function code 12, because RFC 5340 carries extensions as native
LSAs. The TLV byte layout is in `docs/architecture/wire/ospf.md`.

## Decisions

- **One shared body builder for both address families**, so the v2 opaque LSA
  and the v3 native LSA carry identical TLV bytes.
  <!-- source: internal/plugins/ospf/ri.go -- buildRIInstances -->
  <!-- source: internal/plugins/ospf/origination_v6_ri.go -- v6OriginateRI, v6RILSType -->
  <!-- source: internal/plugins/ospf/packet/ri_tlv.go -- EncodeRITLVs, DecodeRITLVStream -->
- **A TLV registration hook lets Segment Routing inject its TLVs without the RI
  code naming SR.** The hook is UNEXPORTED, because the consumer is in-package.
  It is keyed by TLV type. The Informational Capabilities TLV is emitted first,
  then the registered builders in ascending type order, each panic-recovered.
  <!-- source: internal/plugins/ospf/ri_registry.go -- registerRITLV -->
- **Graceful-restart capability bits come from an injectable engine seam**, so
  the RI code does not depend on the GR implementation landing first.
- **The OSPFv3 RI LSA sets the U-bit**: wire types `0x800C` link, `0xA00C` area
  and `0xC00C` AS, not the literal `0x200C`, `0x400C` and `0x600C`. RFC 7770
  Section 2.2 requires U=1 so that an unknown router still floods it, and RFC
  5340 Section 4.4.1 confines a U=0 unknown LSA to link-local scope. Reception
  recognizes RI by function code whatever the U-bit, so either encoding is
  accepted.
  <!-- source: internal/plugins/ospf/v3/types/lsa.go -- LSTypeRouterInformationArea -->

## Constraints on callers

- A new capability bit or SR TLV attaches through the shared builder and the TLV
  hook. It needs no new LSA plumbing.
- **`ASWide()` asks "does this flood AS-wide?" and `ASExternal()` asks "is this
  function code 5?".** Use the first for store, flood and stub-suppression
  scope, and the second only for the literal Type-5 question. Before the split,
  one broad predicate would have fed an AS-scope RI LSA into the external
  computation and produced false ASBR detection.
  <!-- source: internal/plugins/ospf/types/lstype.go -- ASWide, ASExternal -->
- OSPFv3 RI reception works through scope-based store routing, not a
  known-type gate.

## Traps

- **The default RI scope is area and AS.** The AS branch also emits an
  area-scoped RI into each attached NSSA, because a Type-11 LSA cannot flood
  into an NSSA (RFC 7770 Section 2.7). Guard that fallback on the area scope
  being absent, or the NSSA is emitted twice and the origination counter
  double-counts.
