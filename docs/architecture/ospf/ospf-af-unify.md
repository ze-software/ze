# One OSPF engine for both address families

OSPFv3 began as a separate-plugin design. The owner chose ONE `ospf` engine with
address-family-aware seams. This is the decision every later OSPF change inherits.

## Decisions

- **One engine with a Transport seam, a Codec seam and an address-family prefix
  strategy, not a second engine.** The FSM, flooding, DR election and SPF are
  already address-family neutral.
  <!-- source: internal/plugins/ospf/codec.go -- Codec -->
  <!-- source: internal/plugins/ospf/codec_v6.go -- v6Codec -->
  <!-- source: internal/plugins/ospf/afstrategy_v6.go -- v6Strategy -->
  <!-- source: internal/plugins/ospf/spf/afstrategy.go -- AFPrefixStrategy -->
- **`internal/plugins/ospf/v3/{types,packet,transport}` are LEAVES the engine
  consumes.** The import guard and the one-way dependency preserve the module
  tiers.
- **Neighbor reachability is a `netip.Addr` and the next-hop source belongs to
  the strategy.** The OSPFv3 next-hop is the neighbour link-local address per
  interface, which the OSPFv2 4-byte link data cannot express.
- **The LSDB originates with caller-provided wire bytes.** Sequencing and
  flooding stay shared while the wire format stays address-family specific.
  <!-- source: internal/plugins/ospf/origination_v6.go -- v6OriginateSelf -->
  <!-- source: internal/plugins/ospf/encoder_v6.go -- v6Encoder -->

## Constraints on callers

- Future OSPFv3 work belongs in an engine strategy, in link-scope LSDB handling,
  or in the v3 codec and transport leaves. Do not create a second OSPFv3 engine.
- A scope-typed OSPFv3 LS Type is classified through the type helpers, never
  through an OSPFv2 numeric constant.
  <!-- source: internal/plugins/ospf/types/lstype.go -- ASExternal, ASWide -->
- Link-LSAs are interface-scoped. The DD summary, the LS Request lookup, the
  ack, the aging, the refresh, the release and the snapshots all need the
  arrival interface.
  <!-- source: internal/plugins/ospf/lsdb/link_scope.go -- isLinkLSAType, installLink -->
- A BGP redistribution source registers at package init. Registration after peer
  parsing is too late for config validation.

## Traps

- **An absent route at the peer looked like Router-LSA reachability and was
  not.** Advertising Exchange and Loading neighbors was the error. Link-LSAs
  have to take part in the database exchange so that Loading can reach Full.
- **OSPFv3 AS-External LSAs landed in per-area storage** because code compared
  `key.Type == 5`. Anywhere storage or flood scope matters, use the scope-aware
  type helpers.
- A final-adjacency failure needs the DD and LS Request drain and the LSDB
  acceptance check before the Router-LSA topology changes.
- A doc-test failure can surface as a Go compile error in a package the docs
  tooling imports, not as prose drift.
