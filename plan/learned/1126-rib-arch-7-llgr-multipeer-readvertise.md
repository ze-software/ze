# 1126 -- rib-arch-7: LLGR Egress Filter on the Readvertise Rail

## Context

RFC 9494 LLGR readvertisement to non-LLGR peers was BROKEN, not just untested. The premise
"add a multi-peer `.ci`" was wrong: `LLGREgressFilter` (`gr/gr_egress.go`) ran ONLY on the
ForwardUpdate rail (`safeEgressFilter`, `reactor_api_forward.go`), while the LLGR
readvertise trigger (`onLLGREntryDone` -> `clear bgp rib out` -> `outboundResend` ->
`resendRoutesWithCursor`, `rib_replay.go`) flows through `AnnounceNLRIBatch`, which ran NO
egress chain and dropped `ctx.Meta` at `DispatchNLRIGroups` (`cmd/update/update_text.go`).
So stale routes reached non-LLGR peers unmodified.

## Decisions

- **Wire the filter into the batch rail, scoped to stale batches.** The common
  `AnnounceNLRIBatch` path groups peers and builds one UPDATE per group -- incompatible with the
  per-peer LLGR decision. But LLGR readvertise is a RARE (GR-expiry) event, so a per-peer
  filtered branch that fires ONLY when `batch.Stale > 0` leaves the hot common path byte-unchanged.
- **Registration opt-in, not a hardcoded plugin name.** Added `filterapi.Filter.Readvertise`
  (mirrors `EnableRSForwarding`'s "plugin declares, reactor discovers" seam) so the reactor runs
  ONLY `ReadvertiseEgressFuncs()` on stale batches -- not the full egress chain (which would
  re-stamp OTC / re-apply community/policy that already ran at the original announce). Only
  `gr` opts in. The reactor never spells "bgp-gr" (plugin-self-containment).
- **`NLRIBatch.Stale` threads the level; reuse the forward rail's apply.** `DispatchNLRIGroups`
  populates `Stale` from `ctx.Meta["stale"]`. `sendStaleReadvertise` -> `decideStaleReadvertise`
  runs the filters and maps mods to: `buildBatchWithdrawUpdate` (non-LLGR eBGP withdraw),
  `buildModifiedPayload` -> `sendBodyWithSplit` (non-LLGR iBGP depreference), unchanged send
  (LLGR-capable). `pp=nil` on `buildModifiedPayload` returns a safe-to-retain copy (no pool
  release), which the cursor's 4000-byte NLRI cap (`rib_replay.go`) keeps within one UPDATE.
- **`decodePeerUp` fix is NOT here** -- that was rib-arch-5. Unrelated.

## Consequences

- Divergence covered by `TestStaleReadvertiseWireOutput` (reactor wire-output, deterministic).
- The live multi-peer BGP `.ci` remains a tracked test-infra follow-up: it needs a
  source-attributed stale route in the OTHER peers' rib-out (the source's route propagated in
  first), and the attempt hit an upstream test-harness limit (multi-peer BGP OPEN not completing).
  Scope reduction was user-approved.

## Gotchas

- **rib-out is populated by SENT updates** (`rib_structured.go`, `sourcePeer =
  se.SourcePeerStr`), and `mark-stale` (`rib_commands.go`) marks ONLY rib-out entries
  sourced from the restarting peer. So a working multi-peer readvertise `.ci` needs the source's
  route FORWARDED into the other peers' rib-out (ForwardUpdate rail sets SourcePeerStr) -- the
  RS fast path bypasses rib-out, and RS replay-on-peer-up needs `bgp-adj-rib-in` loaded.
- **ze-peer's OPEN ASN defaults to 0 = mirror ze's local AS** (`internal/test/peer/peer.go`);
  set it explicitly with `option=asn:value=<N>` for an eBGP peer (else ze rejects the AS mismatch).
- **The community AttrModHandler emits EXTENDED LENGTH** even for a 4-byte value: NO_EXPORT on
  the wire is `D0 08 00 04 FF FF FF 01` (flags 0xD0, 2-byte len), NOT the compact
  `C0 08 04 FF FF FF 01`. The never-run `.wip` fixture guessed the compact form and would have
  failed; the reactor test asserts the true bytes.

## Files

- `internal/component/bgp/filterapi/filterapi.go` -- `Filter.Readvertise` + `ReadvertiseEgressFuncs`
- `internal/component/bgp/plugins/gr/register.go` -- `Readvertise: true`
- `internal/component/bgp/types/types.go` -- `NLRIBatch.Stale`
- `internal/component/bgp/plugins/cmd/update/update_text.go` -- `staleLevelFromMeta` + populate
- `internal/component/bgp/reactor/reactor.go` -- `readvertiseEgressFilters` built at construction
- `internal/component/bgp/reactor/reactor_api_batch.go` -- `sendStaleReadvertise`/`decideStaleReadvertise`
- `internal/component/bgp/reactor/peer_send.go` -- `sendBodyWithSplit`
- `internal/component/bgp/reactor/reactor_stale_readvertise_test.go` -- decision + wire-output tests
