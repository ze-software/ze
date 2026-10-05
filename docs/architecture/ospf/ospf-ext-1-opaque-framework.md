# OSPFv2 opaque-LSA carrier

The carrier for opaque LSAs (types 9, 10 and 11, RFC 5250): scope-correct
flooding, the Link State ID split, O-bit negotiation, aligned TLV helpers and a
consumer registry. The carrier interprets NO opaque body, and an opaque LSA
never becomes an SPF vertex (RFC 5250 Section 3).

## Decisions

- **Consumers register process-globally at `init()`, not per engine.** Opaque
  types are owned globally (RFC 5250 Section 9), and a consumer stays
  self-contained. The registration function is UNEXPORTED on purpose: every
  consumer lives in the same package, and an exported name would invite an
  out-of-package registration the carrier is not designed for.
  <!-- source: internal/plugins/ospf/opaque_registry.go -- registerOpaqueConsumer -->
  <!-- source: internal/plugins/ospf/lsdb/opaque_as.go -- OpaqueDelivery, SetOpaqueDelivery -->
- **The Link State ID split is a codec-layer pair, not a field on the types
  leaf.** The 32-bit LS ID splits into an 8-bit Opaque Type and a 24-bit Opaque
  ID at the codec.
  <!-- source: internal/plugins/ospf/packet/lsa_opaque.go -- OpaqueTypeOf, OpaqueIDOf, OpaqueLinkStateID -->
- **Origination is a PULL model.** A consumer returns the full desired set on
  each self-LSA pass, and an unchanged return floods nothing. This reuses the
  existing self-origination sequencing instead of adding a second origination
  path.
- **The three existing LSDB stores carry opaque LSAs by scope.** Type 9 goes to
  the link store, Type 10 to the per-area store, and Type 11 to a new AS-wide
  opaque store beside the AS-external store. No new LSDB key type was added.
  <!-- source: internal/plugins/ospf/lsdb/opaque_as.go -- OpaqueOriginateInput, OriginateOpaque -->
- **The O-bit is a Database Description signal only**, and not part of the Hello
  E-bit and N-bit match (RFC 5250 Section 3.1). After either master/slave
  negotiation succeeds, the peer's DD options determine whether its outgoing
  summary includes Types 9, 10 and 11. A non-opaque peer receives only the
  non-opaque headers; Ze still advertises its own O-bit in its DDs. The candidate
  database summary before negotiation stays scope-complete. Exact OSPFv2 type
  matching leaves OSPFv3's full-width types and internal Grace sentinel intact.
  <!-- source: internal/plugins/ospf/neighbor/dd.go -- handleExStartDDLocked -->
  <!-- source: internal/plugins/ospf/neighbor/rfc5250_dd_capability_test.go -- TestDDNegotiatedOpaqueCapabilityControlsAdvertisedSummary, TestDDOpaqueCapabilityPreservesV3NeutralSummary -->
- **Type 11 reachability looks up the originator's ASBR routing table entry**
  (RFC 5250 Section 5), the same entries the Type 5 calculation uses: a
  router SPF reaches without the Router-LSA E-bit has none, so its Type 11 LSAs
  are not usable. Types 9 and 10 are always reachable.
  <!-- source: internal/plugins/ospf/opaque.go -- spfASBRReachable -->
- **A Type 11 originator advertises itself an ASBR** (RFC 5250 Section 5 (1)),
  which is what makes the lookup above succeed for Ze's own LSAs: while the
  router originates a non-purged Type 11 LSA, its Router-LSA carries the E-bit,
  and every Type 11 LSA header it originates carries the Options E-bit, as RFC
  2328 Section 12.1.2 sets it in every AS-external-LSA. Without the Router-LSA
  E-bit, another Ze router finds no ASBR entry and ignores the LSA.
  <!-- source: internal/plugins/ospf/lsdb/origination.go -- selfIsASBRLocked -->
  <!-- source: internal/plugins/ospf/lsdb/opaque_as.go -- OriginateOpaque -->
- **A consumer judges Type 11 usability when it reads an entry, never when the
  LSA arrives.** RFC 5250 Section 5 requires discontinuing every Opaque LSA of an
  originator once it is detected unreachable, and no new LSA arrives to say so.
  The TED and the Extended Prefix receiver both store the scope and ask the
  reachability seam at read time, so an entry delivered usable is shown unusable
  as soon as SPF no longer reaches its originator.
  <!-- source: internal/plugins/ospf/ext_prefix.go -- extPrefixUsable -->
  <!-- source: internal/plugins/ospf/te_ted.go -- usableLocked -->
- **TLV helpers are generic and 4-byte aligned**, with buffer-first emit and
  bound-checked iteration. Every consumer builds on them.
  <!-- source: internal/plugins/ospf/packet/opaque_tlv.go -- opaqueTLVIterator, DecodeOpaqueTLVs -->

## Constraints on callers

- A consumer registers its opaque type and owns its TLV body. The carrier names
  no consumer. Removing a consumer removes its registration and all its
  behaviour.
- **Any new AS-wide opaque store is added to BOTH the aging tick and the
  self-refresh pass**, or Type-11 self-LSAs never refresh and never purge.

## Traps

- **Per-neighbour opaque capability was NEW plumbing, not a read.** The JSON
  snapshot and the flooding neighbour view did not carry the options field. Only
  the internal neighbor struct did.
- **Type 11 is AS-wide, and the AS-external predicate is FALSE for it.** Before
  the carrier, Type-11 LSAs misrouted into the per-area store. Scope routing is
  explicit in the store selection.
- The area-drop filter returned false for opaque types, so Type 11 was not
  dropped in a stub or NSSA area until the Section 3.1 rule was added.
