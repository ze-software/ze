# OSPFv2 Traffic Engineering LSAs

The first consumer of the opaque carrier: Opaque type 1 (RFC 3630 TE) and type 6
(RFC 5392 Inter-AS TE). Reception parses into a PASSIVE Traffic Engineering
Database. There is no SPF and no CSPF (RFC 3630 Section 1). The TLV byte layout
is in `docs/architecture/wire/ospf.md`.

## Decisions

- **The TE body codec is built on the carrier's generic TLV builder and
  iterator.** No second TLV codec exists.
  <!-- source: internal/plugins/ospf/packet/te_lsa.go -- TELSA, DecodeTELSA -->
  <!-- source: internal/plugins/ospf/packet/te_interas.go -- appendInterAsSubTLVs -->
- **The TE database is engine-owned and unexported.** The value-typed snapshot
  for a future CSPF consumer stays intra-package until that consumer is wired.
  Wiring completeness forbids a speculative export.
  <!-- source: internal/plugins/ospf/te_ted.go -- tedSnapshot -->
  <!-- source: internal/plugins/ospf/te.go -- registerTEConsumer, teOnReceive -->
- **Origination is pull-model through the carrier.** A withdraw diff on
  unchanged config floods nothing.
  <!-- source: internal/plugins/ospf/te_originate.go -- teOriginateType1 -->
- **Three GENERIC additions to the carrier were required and are sound**: a
  withdrawn flag on a received delivery, because a received MaxAge purge retains
  its body and a flag is the only reliable withdraw signal; a per-origination
  scope override, because the carrier fixes one scope per opaque type and
  inter-AS needs a per-link Type 10 or 11 choice; and a by-type opaque lookup
  for the inline database view.
- **A self-originated TE link is upserted into the local database at
  origination**, because a self-LSA is short-circuited before install and never
  arrives through reception.

## Traps

- **Type 11 is AS-wide while the AS-external predicate is false for it**, so
  retransmit bookkeeping treated it as area-scoped. Use the AS-wide predicate at
  every AS-wide-or-area site.
- **The link-scope flood path lacked the RFC 5250 Section 3.1 O-bit gate** that
  the area path had. The gate is guarded by the opaque test, so OSPFv3 Type-8
  Link-LSAs still flood to every neighbour.
- **A gauge series must be reset when a label population empties.** The metrics
  gauge vector has no reset, so a tracker records the prior label tuples and
  sets the vanished ones to zero.
  <!-- source: internal/plugins/ospf/te_show.go -- teDatabaseSnapshot -->
- **TE encode allocates per origination.** This is the COLD origination and
  refresh path, and it is a deliberate documented exception to the buffer-first
  rule.
- An exported type that is only the element of a cross-package method return
  still needs the consumer to name it, or the export check flags it.
