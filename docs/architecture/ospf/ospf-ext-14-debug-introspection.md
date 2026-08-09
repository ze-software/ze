# OSPF debug and introspection

Deep LSDB decode, a shortest-path compute explanation, neighbor and interface
deep dumps, address-family-aware instance listing, an offline decode helper, and
gated crafted-LSA injection. This surface is a pure consumer of existing engine
snapshots plus ONE gated write. It adds no wire format, no LSA type and no
change to the computation.

## Decisions

- **A per-family decoder registry, keyed by Opaque Type for IPv4 and by the
  neutral LS type for IPv6.** Each consumer registers its typed decoder from its
  OWN file. Generic code spells NO consumer body format. The fallback is the
  generic opaque TLV and hex iterator for IPv4, and a scope-aware header with
  body hex for IPv6. An optional decoder resolves at runtime, so the surface
  ships before or without a consumer.
  <!-- source: internal/plugins/ospf/decode_view.go -- registerOpaqueDetailDecoder, lookupOpaqueDetailDecoder -->
  <!-- source: internal/plugins/ospf/decode_view_v3.go -- registerV3DetailDecoder, registerV3BaseDecoders -->
  <!-- source: internal/plugins/ospf/debug_wiring.go -- v6DatabaseDetail, v6DatabaseExtended -->
- **Injection is DOUBLE-GATED and fails closed.** The authorization profile
  denies the `debug` prefix, and an engine enablement flag defaults to off and
  is not persisted. Both are required. The enable toggle is itself
  `debug`-prefixed, so a read-only user cannot flip it. Injection is LOCAL only,
  through the EXISTING origination seams, and is never exposed on the web or SSE
  surface.
  <!-- source: internal/plugins/ospf/debug_enable.go -- setDebugInjectEnabled, debugInjectIsEnabled -->
  <!-- source: internal/plugins/ospf/inject.go -- debugInjectOpaque, parseOpaqueInject -->
  <!-- source: internal/plugins/ospf/inject_v3.go -- debugInjectV3, parseV3Inject -->
  <!-- source: internal/plugins/ospf/doctor_debug.go -- checkOSPFDebugEnabled -->
- **The compute explanation is READ-ONLY.** The computer retains its last
  candidate set and a run counter, both set only in the compute path. The
  snapshot copies under the lock, never recomputes, and leaves the route table
  and the run count unchanged.
  <!-- source: internal/plugins/ospf/spf/explain.go -- ExplainSnapshot -->
  <!-- source: internal/plugins/ospf/spf_explain_view.go -- spfExplainSnapshot -->
- **The IPv6 scope filter keys on the LS-type scope bits, not a flat OSPFv2
  numeric type.** The reserved scope value is rejected, and the link-local scope
  includes the per-interface Link-LSA store.
  <!-- source: internal/plugins/ospf/lsdb/native_view.go -- NativeLSAView, LSAViewsByType -->
- **The debug metric series register through a separate one-time path**, because
  the v6 series cannot sit under a v4-only namespace guard. No existing series
  was renamed.
  <!-- source: internal/plugins/ospf/debug_metrics.go -- setDebugMetrics -->
- **Existing nouns were reused rather than duplicated.** The tree already ships
  a TE database view and a segment-routing view, so their typed decoders were
  wired into the opaque-detail registry instead of adding parallel database
  nouns.
  <!-- source: internal/plugins/ospf/instance_view.go -- instanceListing -->
  <!-- source: internal/plugins/ospf/interface_detail.go -- interfaceDetailSnapshot -->
  <!-- source: internal/plugins/ospf/neighbor_detail.go -- neighborDetailSnapshot -->

## Traps

- **A security test can assert the wrong denial.** In production the inject
  command dispatches as a WRITE, so the denial that fires is the edit default,
  not the read-path deny rule. There is no bypass, because the deny rule, the
  enablement gate and the edit default each deny independently. A test that only
  exercises the read path stays green while a change to the edit default opens
  the write path. Assert both.
- **A hand-listed set of handlers does not stay complete.** The assertion that
  no web handler dispatches injection lists handlers by hand, so a new handler
  is not caught. Reflect over the handler methods instead.
- Injection input is validated: the v4 scope is 9, 10 or 11, the opaque id fits
  24 bits, the opaque type is in the private-use range, and the body fits the
  maximum LSA body. For v6 the reserved scope is rejected and the body is
  bounded. A malformed inject or decode is counted and never panics, and never
  wedges the LSDB lock.
- The plugin's own command switch is internal dispatch and disappears with the
  plugin. The central registry is the RPC registration. The v4 and v6 wire
  method names must not collide.
