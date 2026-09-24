# OSPFv2 Extended Prefix and Extended Link LSAs

RFC 7684 Opaque type 7 (Extended Prefix) and type 8 (Extended Link) as consumers
of the opaque carrier. These are TLV CONTAINERS only. They define no SID, no
label and no SRGB. Segment Routing attaches to them. The byte layout is in
`docs/architecture/wire/ospf.md`.

## Decisions

- **Two unexported sub-TLV registration hooks**, one for the prefix registry and
  one for the link registry. A registered codec carries build, receive and
  render callbacks. Origination passes a context: prefix and route type for the
  prefix registry, link type, id and data for the link registry. Type 0 and
  duplicates are rejected, and the callbacks are panic-recovered.
  <!-- source: internal/plugins/ospf/ext_subtlv.go -- registerPrefixSubTLV, registerLinkSubTLV -->
  <!-- source: internal/plugins/ospf/ext.go -- registerExtConsumers, refreshExtMetrics -->
- **Route Type is derived from the source LSA**: a stub link gives intra-area, a
  self summary gives inter-area, and a self external gives AS-external. The
  Extended Link TLV fields are copied verbatim from the matching decoded
  Router-LSA link.
  <!-- source: internal/plugins/ospf/ext_prefix.go -- extPrefixOnOriginate, extPrefixOnReceive -->
  <!-- source: internal/plugins/ospf/ext_link.go -- extLinkOnOriginate, extLinkOnReceive -->
- **An Extended Link Opaque LSA carries exactly one Extended Link TLV** (RFC
  7684 Section 3.1). Decode uses the first and counts the extras.
- **Cross-LSA dedup keeps the STRICTLY lower Opaque ID** (RFC 7684 Section 2).
  An equal id is a refresh and overwrites.

## Traps

- **A `<=` in the dedup comparison drops a same-Opaque-ID REFRESH.** The same
  LSA at a higher sequence then loses its updated flags, route type and
  usability, so an A-Flag transition or a Type-11 unusable-to-usable flip is
  discarded silently. The comparison is strictly lower, and an equal id falls
  through to the overwrite.
- **The link consumer keeps NO separate resolved-link store.** Its gauge is
  recomputed from the opaque store, so the WITHDRAW delivery path must also
  recompute it, or the gauge stays stale.
- **Render-codec panics are recovered but not counted**, because the render chain
  is free functions with no engine access. This is a show-path limitation.
- The router checks Extended Prefix/Link framing before the LSDB receive
  procedure takes any action on the LSA. `wireOpaqueDelivery` installs a typed
  receive validator, independent of the opaque and extended origination flags.
  It checks every container, including duplicates a consumer would ignore,
  without allocating decoded attributes. A TLV/sub-TLV overrun or undersized
  trailing header rejects the LSA before storage, acknowledgement, reflooding,
  self-originated fight-back or MaxAge handling, for every opaque scope.
  Rejections increment `ze_ospf_ext_malformed_total`. Other opaque applications
  retain their uninterpreted carrier behaviour; unknown TLVs within applications
  7 and 8 are skipped by their declared length.
  <!-- source: internal/plugins/ospf/opaque.go -- wireOpaqueDelivery -->
  <!-- source: internal/plugins/ospf/ext.go -- validateExtLSA -->
  <!-- source: internal/plugins/ospf/lsdb/lsdb.go -- ReceiveValidator, SetReceiveValidator -->
  <!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate -->
  <!-- source: internal/plugins/ospf/packet/ext_prefix.go -- ValidateExtLSABody -->
- A non-IPv4 address family in a prefix TLV can over-reject the whole LSA. Only
  address family 0 is defined today, so there is no live impact.
  <!-- source: internal/plugins/ospf/ext_render.go -- extOpaqueDecode -->
