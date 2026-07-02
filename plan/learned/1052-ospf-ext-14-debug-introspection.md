# 1052 - OSPF Debug & Introspection Tooling (both AFs)

## Context

First-class operational debugging for the unified `ospf` engine across both
address families: deep LSDB decode (typed TLV/field rendering with a generic
fallback), an SPF compute explain (why a route won), neighbor/interface deep
dumps, AF-aware instance listing, an offline `ze` decode helper, and -- behind an
explicit gate -- crafted-LSA injection (the useful half of FRR's `ospfclient`,
in-process). A PURE introspection consumer of existing engine snapshots plus ONE
authz-gated write (inject); NO new wire format, NO new LSA type, NO SPF change.
Depends on ext-1 (v4 opaque carrier) + ext-3/4/5/9 (v4 SR/RI/Extended/Grace
decoders) + the native v6 LSA model; all optional decoders resolve at runtime via
the per-family registry, so the surface ships before/without a consumer,
degrading to generic rendering.

## Decisions

- Per-family decoder registry: keyed by Opaque Type (IPv4) and by neutral
  `types.LSType` / function code (IPv6). Each landed consumer registers its typed
  decoder from its OWN file (`te.go`, `ri.go`, `ext.go`, `gr.go` for IPv4; the
  base-8 + Grace + RFC 8362 extended for IPv6 via `registerV3BaseDecoders`).
  Generic code (`decode_view.go`) spells NO consumer body format. Fallback: the
  ext-1 generic opaque TLV/hex iterator (IPv4), a scope-aware header + body-hex
  (IPv6).
- Inject is DOUBLE-GATED, fail-closed: (1) authz `deny "debug"` in
  `BuiltinReadOnlyProfile`; (2) an engine debug-enablement `atomic.Bool`, OFF by
  default, not persisted. Both required; the enable toggle is itself `debug`-
  prefixed so a read-only user cannot flip it. LOCAL-only via the EXISTING
  origination seams (`lsdb.OriginateOpaque` v4; `OriginateSelf`/`OriginateLinkSelf`
  + `WithdrawSelf`/`WithdrawLinkSelf` v6). NEVER surfaced on web/SSE.
- SPF-explain is READ-ONLY: `Computer` retains `lastCandidates` + a `runs` counter
  (additive, set only in the compute path); `ExplainSnapshot` copies under lock,
  never recomputes, leaves the route table + run-count unchanged; AF-tagged for v6.
- IPv6 scope filter keys on the LS Type S2/S1 bits (`v3ScopeBits` mask), NOT a flat
  OSPFv2 numeric type; reserved scope (S2/S1 = 11) rejected; the link-local scope
  includes the per-interface Link-LSA store.
- Six metrics `ze_ospf_debug_*` + `ze_ospfv3_debug_*` via a SEPARATE `setDebugMetrics`
  path (a `sync.Once`): the v6 series legitimately cannot sit under the `ze_ospf_`-
  only guard in `metrics_test.go` (see plan/learned/970). No existing series renamed.

## Consequences

- Inject validation: v4 scope in {9,10,11}, opaque-id <= 24 bits, opaque-type in
  Private-Use (128-255), body <= max LSA body; v6 S2/S1 != 11, body <= 65515, LS-ID
  32-bit. Malformed inject/decode is caught (recover wrapper on the typed decoder;
  the generic `DecodeOpaqueTLVs` is bound-checked by construction), counted in
  `..._debug_decode_errors_total`, never panics, never wedges the LSDB lock.
- AC-5/AC-6 deviation (accepted): the tree already ships `show ospf te-database`
  (ext-2) and `show ospf segment-routing` (ext-5); ext-14 wired the TE/SR typed
  decoders into the new opaque-detail registry and reused those nouns rather than
  adding duplicate `show ospf database te`/`... segment-routing` nouns. Substance
  delivered (TE decode asserted; SR verified transitively via the identical registry
  path); the exact noun differs from the spec text.

## Gotchas

- SECURITY TEST FIDELITY (review NOTE-1, folded into the coverage sweep): in
  production the inject command dispatches with `readOnly == FALSE` because
  `IsReadOnlyPath("debug ...")` is false, so the denial that actually fires is
  `Edit.Default = Deny`, NOT the `deny "debug"` Run-section rule. There is NO bypass
  (the deny rule, the enablement gate, and `Edit.Default` all independently deny),
  but `TestInjectDeniedReadOnly` asserts via `Authorize(cmd, true)` (the Run path),
  which is not the production condition. Harden by ALSO asserting
  `Store.Authorize(cmd, false) == Deny` so a future `Edit.Default` change cannot
  silently open the inject path while the test stays green.
- `assertNoInjectDispatch` (review NOTE-3) hand-lists four web handlers; a future
  inject handler on the struct would not be auto-caught. Reflect over the handler
  methods instead (durability for the no-web-inject invariant).
- The engine-internal `OnExecuteCommand` switch (`register.go`) is the plugin's OWN
  dispatch (deleted with the plugin); the central registry is `pluginserver.RegisterRPCs`
  in `cmd_show.go`. Both v4 `ze-show:ospf-*` and v6 `ze-show:ospfv3-*` methods must be
  distinct (no wire-method or command-noun collision, AC-26).

## Files

- `internal/plugins/ospf/{debug_enable,debug_metrics,debug_wiring,decode_view,decode_view_v3,doctor_debug,inject,inject_v3,instance_view,interface_detail,neighbor_detail,spf_explain_view}.go`, `spf/explain.go` (+ tests)
- Modified: `cmd_show.go`, `register.go`, `te.go`, `ri.go`, `ext.go`, `gr.go`, `packet/opaque_tlv.go` (DecodeOpaqueTLVs), `lsdb/{native_view,origination}.go`, `neighbor/{neighbor,table}.go`, `iface/iface.go`, `spf/computer.go`, `cli/decode.go`, `internal/component/authz/authz.go` (deny debug), `internal/component/web/handler_ospf.go`, `cmd/ze/hub/service_web.go`, `yang/ze-ospf-cmd.yang`
- `test/ospf/ospf-debug-*.ci` + `ospfv3-debug-*.ci` (16), `test/interop/scenarios/{ospf,ospfv3}-debug-*-frr/` (4)
- `docs/{features,guide/ospf,guide/command-reference,guide/plugins,comparison,functional-tests}.md`, `docs/architecture/wire/ospfv3.md`
