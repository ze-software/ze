# 1044 - OSPF Graceful Restart (RFC 3623 IPv4 + RFC 5187 IPv6)

## Context

Non-stop forwarding across an OSPF control-plane restart, both address families,
both roles (restarter + helper). The restarter floods link-scope Grace-LSAs,
retains its FIB, suppresses self-LSA origination, and re-syncs without flapping;
helpers hold the adjacency at Full for the grace period. The control-plane state
machines are SHARED (AF-neutral engine, plan/learned/972); only the Grace-LSA
wire carriage differs: IPv4 = link Type-9 opaque (Opaque Type 3, a consumer of
ext-1); IPv6 = native link-scope v3 LSA (function code 11).

## Decisions

- IPv4 Grace-LSA rides the ext-1 opaque carrier (`registerOpaqueConsumer(3, OpaqueScopeLink, ...)`), 3 TLVs; IPv6 Grace-LSA is a native v3 LSA (new `v3/packet/lsa_grace.go` + a v3 4-octet TLV codec), 2 TLVs, routed through the link store.
- FIB retention: `spf/install.go` Apply/RemoveAll no-op while `suppressInstall()` (= restarting || gracefulStop); a graceful stop sets `gracefulStop` so `Computer.Stop` skips `RemoveAll` and kernel routes survive. Self-LSA origination gated at the shared `originateSelfLSAs` chokepoint (covers both AFs).
- The restarter exits on 3 triggers: all pre-restart adjacencies re-Full, an inconsistent LSA, or grace expiry. The re-Full trigger is wired through the production neighbour `onFull` sink; the grace timer is the always-armed backstop.
- Grace period is measured by the Grace-LSA LS age vs the Grace Period TLV (age starts 0, not reset on retransmit, DoNotAge unset).

## Consequences

- The ext-1 carrier gained an additive `Age` (LS age) field on `opaqueReceived`/`OpaqueDelivery` so the IPv4 helper honours the grace clock (the v6 native path already reads the LSA header age). Other opaque consumers ignore it.
- Operator trigger for planned GR: `ospf graceful-restart prepare` (object-rooted command) calls `prepareRestart`.
- The NVS restart-fact blob (via `openBootCountStore`) carries {restarting, grace-end, reason, IPv6 §3.2 Interface-ID map, §3.1 prefix->LSA-ID map}; a doctor check guards the NVS path.

## Gotchas

- LANDMINE: OSPFv3 Grace function code 11 (0x000B) equals OSPFv2 Opaque-AS Type 11 (0x000B) in the AF-neutral `types.LSType`. Naively broadening `isLinkLSAType` for 0x000B would hijack OSPFv2 Type-11 opaque (inter-AS TE) routing. Use a distinct internal sentinel `LSTypeGraceV6 = 0x800B` mapped ONLY at the v6 codec seam (never emitted on the wire); `IsOpaque()`/`ASWide()` are false for it.
- Exit triggers must have PRODUCTION wiring, not just test callers: wire `noteAdjacencyFull` into the real `onFull` event sink or the restarter only ever exits at grace-expiry (risking the fib-kernel sweep, ~30s, well under a 120s grace = the exact black hole GR prevents).
- `exitRestart` runs on the grace-timer goroutine: snapshot mutex-guarded fields (cfg, reason) INSIDE the lock before the post-unlock work.
- `noteInconsistentLSA` (AC-13) is deferred: reliably distinguishing a truly inconsistent received LSA from a benign change needs a topology diff vs the pre-restart Router-LSA (QEMU-level); wiring it to the generic content-change observer would cause FALSE early exits.

## Files

- `internal/plugins/ospf/{gr,gr_restarter,gr_helper,gr_nvs,gr_preserve,gr_lsa,gr_show}.go` (+ tests)
- `internal/plugins/ospf/packet/grace_lsa.go`, `v3/packet/{tlv,lsa_grace}.go`, `v3/types/lsa.go`, `types/lstype.go` (LSTypeGraceV6)
- `internal/plugins/ospf/{instance,config,register,cmd_show,doctor,spf_wiring,codec_v6,encoder_v6,bfd_client,opaque,opaque_registry}.go`, `spf/{install,computer}.go`, `lsdb/{lsdb,link_scope,origination,flooding,opaque_as}.go`, `yang/ze-ospf-{conf,cmd}.yang`
- `internal/core/diagnostic/codes.go` (GR doctor code)
- `test/ospf/ospf*-gr-*.ci`, `test/interop/scenarios/{ospf-gr-frr,ospf-v6-gr-frr,ospf-gr-fib-retention,ospf-v6-gr-fib-retention}/`
- `rfc/short/rfc3623.md`, `rfc/short/rfc5187.md`, `docs/guide/ospf.md`
