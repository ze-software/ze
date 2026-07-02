# 1030 - OSPFv2 Traffic Engineering LSAs (RFC 3630 + RFC 5392)

## Context

The first consumer of the ext-1 opaque carrier. Registers Opaque type 1 (RFC 3630
TE) and type 6 (RFC 5392 Inter-AS-TE-v2) and implements the TE LSA body only:
Router-Address TLV, Link TLV, sub-TLVs 1-9, IEEE-754 float32 bytes/sec bandwidth,
the RFC 5392 inter-AS sub-TLVs (21/22/24). Reception parses into a passive
Traffic Engineering Database (TED); no SPF, no CSPF (RFC 3630 §1). Query via
`show ospf te-database` + inline decode under `show ospf database opaque-area`.

## Decisions

- Built the TE body codec on the ext-1 generic TLV builder/iterator; did NOT write a second TLV codec. Consumers live IN the `ospf`/`packet` packages, so the ext-1 carrier API is unexported (`registerOpaqueConsumer` etc.).
- The TED is engine-owned and unexported; the value-typed snapshot/lookup for a future `rsvpte` CSPF consumer stays intra-package (unexported `tedSnapshot`) until that consumer is actually wired (wiring-completeness forbids speculative exports; export it when rsvpte lands).
- Origination is pull-model via the carrier: `teOriginateType1/6(router) []opaqueOrigination`; a withdraw diff on unchanged config floods nothing.
- Three GENERIC (non-TE) additions to the ext-1 carrier were required and are sound: `opaqueReceived.Withdrawn`/`opaqueDelivery.Withdrawn` (a received MaxAge purge retains its body, so a flag is the only reliable withdraw signal), `opaqueOrigination.Scope` (per-link Type 10/11 override for inter-AS, since the carrier fixes one scope per Opaque type), and `LSDB.OpaqueLSAsByType` (for inline opaque-area decode).

## Consequences

- Self-originated TE links are upserted into the local TED at origination (so `show ospf te-database` and future CSPF see the full local topology; self LSAs are short-circuited before install, so they never arrive via reception).
- Router-Address LSA is originated once per area that has a TE link (Type 10 is area-local; one per area, Instance 0).
- Gauge series must be reset when a label population empties (a tracker records prior label tuples and Set(0)s vanished ones); `metrics.GaugeVec` has no Reset().
- Future opaque consumers (ext-3/4/9/14) follow this exact shape: register in `register.go`, body codec in `packet/`, engine glue reusing the ext-1 origination/reception hooks.

## Gotchas

- Type-11 (LSTypeOpaqueAS) is AS-wide but `ASExternal()` is false for it, so retransmit bookkeeping (`deletePurgedIfAcked`, `removeFromAllRetransmit`) treated it as area-scoped; fixed with an `isASWideType()` helper used at every AS-wide-or-area site.
- `floodLink` (the Type-9 opaque path) lacked the RFC 5250 §3.1 O-bit flood gate that `floodExcept` has; added, guarded by `IsOpaque()` so OSPFv3 Type-8 Link-LSAs still flood to all.
- ze-validate flags exported symbols with no cross-package non-test caller: unexport intra-package carrier/TLV/TED symbols; a type that is only the element of a cross-package method return (`OpaqueLSACount`) needs the consumer to name it (a typed helper param; `var x []T = expr` trips staticcheck ST1023).
- TE encode is allocation-heavy (make/append per origination) but it is the COLD origination/refresh path, a deliberate documented exception to buffer-first.
- Injected LSP/harness diagnostics again reported stale mid-edit undefined-symbol errors across concurrent trees; `go vet`/`go test` on the final tree are authoritative.

## Files

- `internal/plugins/ospf/te.go`, `te_originate.go`, `te_ted.go`, `te_config.go`, `te_show.go`
- `internal/plugins/ospf/packet/te_lsa.go`, `packet/te_interas.go`
- `internal/plugins/ospf/{config,register,cmd_show,instance}.go`, `lsdb/{flooding,link_scope}.go`, `opaque_registry.go`, `opaque.go`, `lsdb/opaque_as.go`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-te-*.ci`, `test/interop/scenarios/ospf-te-frr/`, `ospf-te-interas-frr/`
- `docs/guide/ospf.md`, `docs/guide/command-reference.md`, `docs/architecture/wire/ospf.md`, `rfc/short/rfc3630.md`, `rfc/short/rfc5392.md`
