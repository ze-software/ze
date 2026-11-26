# Spec: tombstone-forwarding-policy

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Restore the default eBGP Transitive clear on both forwarding rails (ordinary and route-server clients); optional policy selection is separate. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions

| Date | Topic | Decision |
|------|-------|----------|
| 2026-08-07 | D-1: "not forwarded" under `inherit` | Resolved as reading (b): clearing the Transitive bit at eBGP egress is the containment mechanism; non-transitive markers stay non-transitive without requiring physical removal or forcing a rebuild on the default path. |
| 2026-08-07 | D-2: Partial bit under `inherit` | Partial is not a forwarding lever under `inherit`: received Partial stays unchanged when forwarding (only Transitive clears on eBGP egress, while iBGP egress leaves Transitive unchanged). Locally generated markers keep their separate Section 4.2 Partial-clear rule. |
| 2026-11-24 | Route-server eBGP client Transitive-bit clear | Apply the default eBGP Transitive-bit clear to **route-server eBGP clients too** (not just ordinary eBGP peers); pay the per-destination buffer copy/edit only on UPDATEs that actually carry an `ATTR_TOMBSTONE` marker so normal marker-free UPDATEs preserve the zero-copy fast path. |

## Task

Restore the default `draft-mangin-idr-attr-tombstone-00` Section 5.3 `inherit` forwarding behavior on both live BGP forwarding rails (`forwardUpdateCore` / `forwardUpdateSection` in `reactor_api_forward.go` and `reactorForwardRS` / `reactorForwardRSSectionResult` in `forward_rs.go`) for both ordinary eBGP peers and route-server eBGP clients:

1. When forwarding an UPDATE carrying a transitive `ATTR_TOMBSTONE` (code 252, `attribute.AttrTombstone`) to any eBGP destination (ordinary eBGP peer or route-server eBGP client), clear the Transitive bit (`0x40`) in the destination's own rebuilt buffer without mutating shared received wire bytes.
2. Preserve the marker's received Partial bit (`0x20`), Optional bit (`0x80`), Extended Length header class (`0x10`), and every value byte verbatim, and preserve all unrelated attributes and bytes.
3. Also clear the Transitive bit when an eBGP egress edit itself generates an `ATTR_TOMBSTONE` (for example `wireu.ASPathEdit.recordAggregatorDiscard` discarding a malformed AGGREGATOR), while retaining the Section 4.2 Partial-clear rule on generated markers.
4. Preserve iBGP egress behavior (received Transitive and Partial bits unchanged) and preserve non-transitive received markers under `inherit` without physical removal or Partial changes.
5. Keep the zero-copy fast path untouched (no rebuild, no added allocation) for every UPDATE that carries no transitive `ATTR_TOMBSTONE` marker and generates none.
6. Update stale comments and design documentation (`docs/architecture/wire/attributes.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `internal/component/bgp/message/attr_discard.go`, and `test/plugin/prefixsid-ebgp-discard-single-walk.ci`) so code, tests, and architecture docs agree with the restored behavior.

This spec owns original AC-1, AC-6, AC-7, and the default-path half of AC-8 (`AC-8-default`). Configurable per-peer/per-group `inherit`/`strip`/`propagate` selection, YANG configuration, physical removal (`strip`), and `propagate` Transitive-set/Partial-clear belong to `plan/post-release/features/spec-tombstone-policy-selection.md`.

## Required Reading

- [ ] `ai/doors/system.md` - platform boundaries, buffer-first encoding, registration over hardcoding
  → Constraint: no cross-process/component coupling; reuse pooled per-destination buffers.
- [ ] `ai/doors/bgp-engine.md` - BGP engine boundaries, received-byte immutability, gates
  → Constraint: received wire bytes are borrowed/shared and MUST NEVER be mutated per destination; untagged tests do not link BGP (`-tags ze_bgp`).
- [ ] `ai/doors/le-tooling.md` - functional `.ci` and interop test placement, runner rules, and gates
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - sibling eBGP egress rules (`LOCAL_PREF`, `MULTI_EXIT_DISC`) and their shared cost constraint
  → Decision: scan attribute presence ONCE per UPDATE above the destination loop so marker-free traffic stays zero-copy.
- [ ] `docs/architecture/wire/attributes.md` - BGP path attribute header/flags layout and `ATTR_TOMBSTONE (Code 252)`
  → Decision: supersedes the 2026-09-09 retirement note in `ATTR_TOMBSTONE (Code 252)` and updates that section in the same change.
- [ ] `docs/architecture/bgp/structural-forwarding.md` - RS fast path, `rsBase`, export chain handling, and buffer ownership
- [ ] `docs/architecture/bgp/fanout-dedup.md` - per-fan-out edit-set deduplication (`fwdDedupTable`)
- [ ] `docs/architecture/buffer-architecture.md` - copy-on-modify triggers and `peerPool` ownership
- [ ] `docs/architecture/route-selection.md` - RFC 7606 attribute-discard escalation (`ATTR_TOMBSTONE` does not alter route selection)
- [ ] `rfc/short/rfc4271.md` - Section 4.3 attribute flags and Section 5 optional non-transitive/transitive handling
- [ ] `rfc/short/rfc7606.md` - attribute-discard error handling (generation is separate from Section 5.3 forwarding)
- [ ] `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Section 5.3 (READ ONLY — Thomas's IETF draft, never edit)

## Current Behavior

**Source files read (verified against HEAD on 2026-11-26):**
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateSection` (called by `forwardUpdateCore`) hoists once-per-UPDATE attribute scans (`srcHasLocalPref`, `srcHasPrefixSID`, `srcMED`, `srcNextHop`, `srcAIGP`, `srcWellKnown`), runs export policy (which may replace `peerBaseWire` with `exportWireOverride`), optionally partitions mixed NLRI sections, calls `aspathEdit.Record`, and materializes per-destination edits via `buildForwardPayload` and `fwdDedupTable`. Neither `forwardUpdateSection` nor any helper it calls clears Transitive on `ATTR_TOMBSTONE`.
- [ ] `internal/component/bgp/reactor/forward_rs.go` and `internal/component/bgp/reactor/forward_rs_chain.go` - `reactorForwardRSSectionResult` mirrors the same per-UPDATE hoists into `rsBase`, runs active export chains, calls `aspathEdit.Record`, and materializes per-destination edits via `buildForwardPayload` and `fwdDedupTable`. It also applies no `ATTR_TOMBSTONE` eBGP Transitive clear.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - `attrModHandlersWithDefaults` registers `tombstoneHandler()` for code 252 (`attribute.AttrTombstone`). Today `tombstoneHandler` handles `AttrModSuppress` and `AttrModSet` (for `wireu.ASPathEdit.recordAggregatorDiscard`), keeps an existing upstream marker verbatim via `p.KeepAll()` (even when a local `AttrModSet` also arrived), and has no flag-clear operation.
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` and `planAttr` already implement the exact-sized one-pass merge rebuild into per-peer `peerPool` buffers, reject duplicate type codes via `SpanIndex.Rebuild`, and return `(nil, 0, modifyFailureNone)` when `mods.HasModifications()` is false. No edits to `forward_build.go` are needed.
- [ ] `internal/component/bgp/reactor/forward_local_pref.go` and `internal/component/bgp/reactor/forward_prefix_sid.go` - define the sibling egress-rule pattern: once-per-UPDATE `payloadHasAttr` scan, per-destination `applyFacts*` function, and static `[...]byte` `AttrModRemove` payloads (as in `srv6ServiceTLVTypes`).
- [ ] `internal/component/bgp/reactor/forward_body.go` and `internal/core/bgp/attribute/flags_spec.go` - `coreAttributes` registers `AttrTombstone` (252) as recognized, so `forwardOpaquePayload` and `SetPartialOnUnrecognizedTransitive` never treat it as an unrecognized attribute.
- [ ] `internal/core/bgp/attribute/partial.go` and `internal/component/bgp/reactor/session_validation.go` - ingress normalization (`ClearPartialOnWellKnownAndNonTransitive`) clears Partial only on non-transitive optional attributes; received transitive `ATTR_TOMBSTONE` markers keep their received Partial bit intact into the forward rails.
- [ ] `internal/component/bgp/wireu/aspath_slot.go` - `ASPathEdit.recordAggregatorDiscard` records `AttrModSet` on `attribute.AttrTombstone` with a 2-byte `(code, reason)` value for malformed AGGREGATORs; `tombstoneFlags` derives `0xC0` (Transitive set) because AGGREGATOR is optional transitive.
- [ ] `internal/component/bgp/message/attr_discard.go` and `internal/component/bgp/wireu/tombstone.go` - receive-side and transcode-side generation compute flags with Optional set, Transitive and Extended Length preserved, and Partial cleared. `attr_discard.go` carries a comment saying Section 5.3's egress clear is not enforced anywhere.
- [ ] `test/plugin/prefixsid-ebgp-discard-single-walk.ci` - pins receive-time in-place marking toward an iBGP receiver, but its comment still cites the deleted `wireu.rewriteASPathPrepend` helper for Section 5.3 eBGP clearing.
- [ ] `test/plugin/remove-private-as-export.ci` - already carries the 2026-07-16 input-side LOCAL_PREF correction; no change needed.

**Behavior to preserve:**
- Zero-copy forwarding (no rebuild, no added allocation) for every UPDATE that carries no transitive `ATTR_TOMBSTONE` marker and generates none, on both ordinary and RS-client rails.
- Shared received wire immutability: per-destination eBGP flag clearing never mutates shared `WireUpdate` bytes, so iBGP peers in the same fan-out still receive the unmodified transitive marker.
- Received Partial bit preservation on forwarding (`0xC0` -> `0x80`, `0xE0` -> `0xA0` on eBGP; `0xC0` -> `0xC0`, `0xE0` -> `0xE0` on iBGP), and Section 4.2 Partial-clear on locally generated markers (`attrDiscardFlags`, `WriteTombstone`, `tombstoneFlags`).
- Non-transitive received markers remain non-transitive without physical removal or Partial changes.
- Extended Length header class (`0x10`) and all marker value bytes (including zero-padded in-place markers, 0-byte, 1-byte, and odd-length values per Section 5.2.3) are preserved verbatim.
- `test/plugin/remove-private-as-export.ci` remains unchanged.

**Behavior to change:**
- On both live forwarding rails (`forwardUpdateSection` and `reactorForwardRSSectionResult`), clear the Transitive bit on `ATTR_TOMBSTONE` markers (received, export-override, or egress-generated) for every eBGP destination, including both ordinary eBGP peers and route-server eBGP clients.
- Update `docs/architecture/wire/attributes.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `internal/component/bgp/message/attr_discard.go`, and `test/plugin/prefixsid-ebgp-discard-single-walk.ci` to match the restored behavior.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| General forward rail | `internal/component/bgp/reactor/reactor_api_forward.go` (`forwardUpdateSection`, `forwardUpdateCore`) | Hoists per-UPDATE attribute scans, runs export chain, handles partitioned sections, records `ASPathEdit`, and calls `buildForwardPayload` with `fwdDedupTable` | Extends: hoists `payloadHasTransitiveTombstone` once per UPDATE, re-checks on override/partition, and calls `applyFactsTombstone` on eBGP destinations after `aspathEdit.Record` |
| Route-server fast-path rail | `internal/component/bgp/reactor/forward_rs.go` (`reactorForwardRSSectionResult`) and `forward_rs_chain.go` (`rsBase`, `rsBaseOf`, `rsBaseOverride`) | Mirrors the general rail's per-UPDATE hoists and `ModAccumulator` rebuild for RS-eligible destinations | Extends: hoists `payloadHasTransitiveTombstone` into `rsBase` and calls `applyFactsTombstone` on eBGP destinations after `aspathEdit.Record` |
| One-pass attribute rebuild | `internal/component/bgp/reactor/forward_build.go` (`buildModifiedPayload`, `buildForwardPayload`, `planAttr`) | Materializes per-destination edits into `peerPool` buffers while leaving unmodified UPDATEs zero-copy and rejecting duplicate attribute codes | Reuses unchanged: `applyFactsTombstone` expresses the flag clear as a normal `ModAccumulator` operation handled by `tombstoneHandler` |
| Fan-out edit-set dedup | `internal/component/bgp/reactor/forward_dedup.go` (`fwdDedupTable`) and `filterapi/fingerprint.go` (`AppendEditDigest`) | Digests `ModAccumulator` operations so identical destinations in one fan-out rebuild once and copy into each destination's `peerPool` buffer | Reuses unchanged: `AttrModRemove` on code 252 with its static mask byte enters the digest automatically, amortizing rebuild cost across RS-client fan-outs |
| Tombstone rebuild handler | `internal/component/bgp/reactor/filter_delta_handlers.go` (`tombstoneHandler`, `tombstoneFlags`, `attrModHandlersWithDefaults`) | Handles `AttrModSet` and `AttrModSuppress` for code 252 (`attribute.AttrTombstone`); preserves upstream markers verbatim | Extends: applies `AttrModRemove` flag-clear bitmask (`FlagTransitive`) to both received markers (preserving Partial, Extended Length, and value bytes) and egress-generated `AttrModSet` markers |
| Sibling egress attribute rules | `internal/component/bgp/reactor/forward_local_pref.go` (`payloadHasAttr`, `applyFactsLocalPref`), `forward_prefix_sid.go` (`applyFactsPrefixSID`, `srv6ServiceTLVTypes`) | Establishes once-per-UPDATE section scanning, per-destination `applyFacts*`, and static `[...]byte` `AttrModRemove` payloads | Extends pattern: adds sibling `internal/component/bgp/reactor/forward_tombstone.go` following the exact same structure |
| Egress AGGREGATOR discard marker | `internal/component/bgp/wireu/aspath_slot.go` (`ASPathEdit.recordAggregatorDiscard`) | Records `AttrModSet` on `attribute.AttrTombstone` with `(7, TombstoneInvalidLength)` when discarding a malformed AGGREGATOR | Reuses unchanged: running `applyFactsTombstone` after `aspathEdit.Record` clears Transitive on that generated marker for eBGP destinations |
| Receive/transcode marker generation | `internal/component/bgp/message/attr_discard.go` (`attrDiscardFlags`, `ApplyAttrDiscard`), `wireu/tombstone.go` (`WriteTombstone`) | Generates code 252 markers per Section 4.2/5.1/5.7 with Partial cleared | Preserves behavior unchanged; updates only the stale doc comment on `attrDiscardFlags` |
| Recognized attribute registry and opaque treatment | `internal/core/bgp/attribute/flags_spec.go` (`coreAttributes`), `reactor/forward_body.go` (`forwardOpaquePayload`), `core/bgp/attribute/partial.go` | Marks `AttrTombstone` recognized so opaque-attribute treatment neither drops non-transitive markers nor stamps Partial on transitive markers | Reuses unchanged: ensures `ATTR_TOMBSTONE` reaches `applyFactsTombstone` with its received Partial bit intact |
| Retired 2026-07-16 LOCAL_PREF fixture fix | `test/plugin/remove-private-as-export.ci` | Already omits invalid eBGP `LOCAL_PREF` from its source frame per the 2026-07-16 ruling | Does not apply: already landed in HEAD; preserved unchanged |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | always | "`WriteTo(buf, off) int` into a pooled buffer, no `make` where a pool exists" | agrees | Marker flag clearing rides `ModAccumulator` and `buildModifiedPayload` into the destination's existing `peerPool` buffer; marker-free UPDATEs allocate nothing. |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `reactor_api_forward.go`, `forward_rs.go`, `forward_rs_chain.go`, `filter_delta_handlers.go`, `attr_discard.go`, `forward_tombstone.go`, `tombstone_forward_test.go` | "**Received bytes are borrowed.** A received UPDATE points into a reusable read buffer, so `RawMessage.IsAsyncSafe()` is false and anything that hands it to another goroutine copies first" | agrees | Clearing Transitive never mutates shared received wire bytes; eBGP destinations materialize into their own `peerPool` buffer while iBGP destinations keep the shared wire. |
| `ai/doors/le-tooling.md` | Where it sits | door covering `test/plugin/prefixsid-ebgp-discard-single-walk.ci`, `test/plugin/tombstone-policy-inherit-default.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner (`internal/test/runner`, `docs/architecture/testing/runner-architecture.md`)." | agrees | New `test/plugin/tombstone-policy-inherit-default.ci` and the comment correction in `test/plugin/prefixsid-ebgp-discard-single-walk.ci` live under `test/plugin/` and run through `./le test functional plugin`, while `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` runs through `./le test integration interop` with no compiled Go fixture regressions. |
| `docs/architecture/core-design.md` | Progressive Build (applyMods) | `// Design:` of `reactor_api_forward.go`, `filter_delta_handlers.go`; anchored to `reactor_api_forward.go`, `forward_rs.go`, `filter_delta_handlers.go` | "When `mods.Len() == 0` (common case: no role config or no stamping needed), the progressive build is skipped entirely -- zero allocation, zero copy." | agrees | `applyFactsTombstone` records no operation unless the destination is eBGP and a transitive marker is present or generated, keeping marker-free traffic at `mods.Len() == 0`. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This fix touches only BGP UPDATE egress flag handling in `reactor` and adds or changes no Linux-kernel/VPP backend or commit-gate rule. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This fix introduces no configuration leaf or data-plane mismatch check and leaves commit, reload, and validate verdicts unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This fix does not touch firewall verification, VPP, or YANG backend annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This fix introduces no data-plane selector or backend registration and leaves all plane names untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This fix does not alter egress next-hop rewriting or add any session-level next-hop configuration; it operates solely on `ATTR_TOMBSTONE` (code 252) flags. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This fix leaves `policychain.Sets` next-hop authoring forms and chain references unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This fix adds no export-chain step, plugin RPC, or `FilterInsertion`, and does not change `Policy.Configured()` or next-hop compilation. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This fix preserves raw-override next-hop provenance untouched and re-evaluates `payloadHasTransitiveTombstone` over `peerBaseWire` when a raw override replaces the source payload. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This fix does not modify `Sets.NextHop`, `NextHopProfile`, or import-chain validation. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This fix touches only forwarded routes on the two forward rails (`ATTR_TOMBSTONE` MUST NOT appear in freshly originated UPDATEs) and does not change next-hop rules. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This fix inserts no filter-chain member and does not alter `filterapi.ImpliedFilter` or `\| implied` display. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This fix adds no startup failure path or service configuration refusal; `ze start` behavior is unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision on `docs/architecture/core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This fix introduces no startup refusal or kernel/VPP mismatch check. |
| `docs/architecture/buffer-architecture.md` | When a copy is deliberate | `// Design:` of `reactor_api_forward.go` | "A copy on the wire path MUST match one of the triggers below. A copy that fits none of them is a defect until this table names the new trigger." | agrees | Clearing Transitive on marker-bearing UPDATEs uses the existing listed `buildModifiedPayload` per-destination attribute rewrite trigger into `peerPool`; marker-free UPDATEs make no copy. |
| `docs/architecture/api/architecture.md` | Overview | anchored to `reactor_api_forward.go` | - | silent | `api/architecture.md` defines the plugin bus, JSON/text API, and cache command protocol; this fix changes only internal reactor egress flag clearing on code 252 and leaves all bus, API, and cache contracts unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This fix introduces no implied config line or `\| implied` formatter change. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This fix touches no bus participant or message registration. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This fix touches no `send > to` bus filter or routing rule. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This fix touches no `receive > from` bus filter. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This fix touches no bus allow-list enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This fix touches no bus filter compilation or commit check. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This fix adds no bus arbitration or role exclusivity. |
| `docs/architecture/api/architecture.md` | Design decisions | decision on `docs/architecture/api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This fix changes no bus display or implied-default view. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `reactor_api_forward.go` | - | silent | `process-protocol.md` defines plugin startup, RPC framing, and event delivery; this fix operates purely inside the reactor's BGP wire egress path and changes no RPC or event contract. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision on `docs/architecture/api/process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This fix adds or modifies no engine RPC operation or plugin authorization profile. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision on `docs/architecture/api/process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This fix does not touch engine-to-plugin event delivery queues or overflow handling. |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `reactor_api_forward.go` | - | silent | `update-syntax.md` defines the operator/API UPDATE text and wire command grammar; this fix adds no command keywords or syntax changes. |
| `docs/architecture/bgp/egress-attribute-rules.md` | The cost constraint both rules obey | anchored to `reactor_api_forward.go`, `forward_rs.go`; modified by this spec | "The presence question is asked ONCE per UPDATE, not once per destination. Recording the operation unconditionally would be simpler and would put every route to every external peer on the payload-rebuild path, which is the cost the route-server fast path exists to avoid." | agrees | `payloadHasTransitiveTombstone` scans the attribute section once per UPDATE above both destination loops; marker-free and already non-transitive UPDATEs record no op and keep the zero-copy fast path. This spec updates `egress-attribute-rules.md` to document this sibling rule. |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `reactor_api_forward.go` | "The identity carries the BASE as well as the digest." | agrees | `applyFactsTombstone` records a standard `AttrOp` (`AttrModRemove` on code 252 with a static 1-byte mask) into `ModAccumulator`, which `AppendEditDigest` already encodes so identical eBGP destinations in a fan-out share one rebuild and copy into their own `peerPool` buffers. |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | `// Design:` of `forward_rs.go`, `forward_rs_chain.go`; anchored to `reactor_api_forward.go`, `forward_rs.go`, `forward_rs_chain.go`, `filter_delta_handlers.go` | "The fast path shares nothing a member's verdict could carry from one destination to another: each destination owns its edit set, a member's rewrite becomes that destination's base for every later edit, and both the shared encode (`fwdDedupIdentity`) and the withdrawal conversion (`fwdWithdrawal`) key on the base." | agrees | `hasTransitiveTombstone` is stored on `rsBase` (`rsBaseOf` / `rsBaseOverride`) alongside `hasLocalPref` and `hasPrefixSID`, so an export-chain rewrite re-evaluates marker presence over its own base and each destination owns its own edit set and buffer. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `reactor_api_forward.go` | - | silent | `encoding-context.md` covers per-peer negotiated capability contexts (`ASN4`, `ADD-PATH`, `ExtNH`); `ATTR_TOMBSTONE` flag clearing depends on eBGP session class, not encoding capabilities, and preserves `SourceCtxID` unchanged. |
| `docs/architecture/forward-congestion-pool.md` | Two-Tier Pool Model | anchored to `reactor_api_forward.go` | "Destination peers get an `Outgoing Peer Pool` for copy-on-modify output." | agrees | Marker-bearing eBGP rebuilds acquire buffers through the existing `peerPool` path in `buildForwardPayload` and release them on item completion/failure; congestion and overflow accounting are unchanged. |
| `docs/architecture/memory/lifetime-contracts.md` | The four contracts | anchored to `reactor_api_forward.go`, `forward_rs.go` | "Both rails MUST finish body construction before publishing a materialization to the per-fan-out dedup table. A failed body MUST abandon its candidate before returning the outgoing slot; otherwise later destinations can copy a different UPDATE that has reused that slot." | agrees | This fix uses the existing `buildForwardPayload` -> `dedup.commit` / `dedup.abandon` lifecycle without introducing any new buffer handle or cross-boundary borrow. |
| `docs/architecture/meta/role.md` | Ordering | anchored to `reactor_api_forward.go` | - | silent | `meta/role.md` governs RFC 9234 role metadata and OTC pipeline ordering; this fix runs strictly as a post-policy egress attribute rule on code 252 and does not touch role metadata or OTC handling. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `reactor_api_forward.go` | - | silent | `rib-storage-design.md` covers plugin RIB pool storage and best-path tracking; this fix runs on the reactor's egress forwarding rails and changes no RIB storage layout. |
| `docs/architecture/route-selection.md` | RFC 7606 Error Escalation | anchored to `forward_rs.go` | "Attribute-discard marks the specific attribute in-place (draft-mangin-idr-attr-tombstone-00) but the route itself continues." | agrees | Clearing the Transitive bit on eBGP egress does not alter receive-side discard, route selection, or route continuation. |
| `docs/architecture/update-building.md` | Three Paths for UPDATE Messages | anchored to `reactor_api_forward.go`, `filter_delta_handlers.go` | - | silent | `update-building.md` covers UPDATE construction paths and MP_REACH/next-hop builders; this fix does not change originated UPDATE construction or NLRI/next-hop builders. |
| `docs/architecture/update-building.md` | Design decisions | decision on `docs/architecture/update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | This fix does not touch FlowSpec MP_REACH next-hop emission or ExaBGP migration. |
| `docs/architecture/wire/attributes.md` | ATTR_TOMBSTONE (Code 252) | anchored to `filter_delta_handlers.go`; modified by this spec | "**Egress flag handling: the Section 5.3 clear is NOT implemented.** The generation flags are stamped at receive time, where the destination is not yet known, so a transitive discarded attribute yields a transitive marker (`0xC0`) and keeps it on every rail." | departs | Restores the Section 5.3 default eBGP Transitive-bit clear on both ordinary and route-server eBGP egress rails per Thomas's 2026-11-24 decision, superseding the 2026-09-09 retirement note; this spec updates `docs/architecture/wire/attributes.md` in the same change. |
| `docs/architecture/wire/messages.md` | 2. UPDATE Message (Type 2) | `// Design:` of `attr_discard.go` | - | silent | `wire/messages.md` specifies BGP message header and body framing; this spec updates only a doc comment in `attr_discard.go` and changes no message framing or header layout. |

## Data Flow

### Entry Point
- A received (or stored-relay / export-override) BGP UPDATE carrying an `ATTR_TOMBSTONE` (code 252) attribute, or a malformed `AGGREGATOR` that triggers `ASPathEdit.recordAggregatorDiscard`, entering either live forwarding rail (`forwardUpdateCore` -> `forwardUpdateSection` or `reactorForwardRS` -> `reactorForwardRSSectionResult`).

### Transformation Path
1. **Once-per-UPDATE presence scan (both rails):** Before the destination loop, `forwardUpdateSection` and `reactorForwardRSSectionResult` call `payloadHasTransitiveTombstone(payload)` (in new `internal/component/bgp/reactor/forward_tombstone.go`), which parses the UPDATE attribute section once with zero allocation using `wire.ParseUpdateSections` and `attribute.AttrFind` for `attribute.AttrTombstone` (252). If found with `FlagTransitive` set, it returns `true`. If found with `FlagTransitive` clear, it scans only the remaining attribute tail for any duplicate `AttrTombstone` carrying `FlagTransitive` so a malformed override with a trailing transitive duplicate cannot slip past the gate (it will enter `buildModifiedPayload` and fail closed on duplicate type code). If absent, it returns `false` immediately.
2. **Base tracking across overrides and partitions:**
   - On `reactorForwardRSSectionResult`, `rsBase` carries `hasTransitiveTombstone` populated by `rsBaseOf` (reusing the hoisted bool) and recomputed by `rsBaseOverride` whenever an export chain produces a `wireOverride`.
   - On `forwardUpdateSection`, inside the per-section `wires` loop (which handles both single-section UPDATEs and `partitioned` mixed-NLRI splits), `baseHasTransitiveTombstone` reuses `srcHasTransitiveTombstone` when `peerBaseWire == sourceWire`, and recomputes `payloadHasTransitiveTombstone(peerBaseWire.Payload())` whenever `peerBaseWire != sourceWire` (export `wireOverride` or partitioned section).
3. **Per-destination eBGP decision (after `aspathEdit.Record`):** On both rails, inside the `facts.isEBGP && !mods.IsWithdraw()` block immediately after `aspathEdit.Record(&mods, ...)`, call `applyFactsTombstone(facts, baseHasTransitiveTombstone, &mods)`.
   - Applies to **both** ordinary eBGP peers and route-server eBGP clients (`facts.isEBGP` is true for both; `!facts.isEBGP` skips iBGP destinations including RR clients and RFC 7705 internal migration peers).
   - Checks `baseHasTransitiveTombstone || tombstoneOpPresent(mods)` (where `tombstoneOpPresent` scans `mods.Ops()` for any `AttrModSet` on code 252, catching `ASPathEdit.recordAggregatorDiscard`).
   - If neither holds (no transitive marker on base and no `AttrModSet` on code 252), returns immediately without touching `mods`. Normal marker-free traffic and already non-transitive received markers add zero operations and remain on the zero-copy fast path.
   - Otherwise records `mods.Op(uint8(attribute.AttrTombstone), filterapi.AttrModRemove, tombstoneClearTransitiveMask[:])`, where `tombstoneClearTransitiveMask` is a package-level 1-byte array `[...]byte{byte(attribute.FlagTransitive)}` (zero allocation, mirroring `srv6ServiceTLVTypes` in `forward_prefix_sid.go`).
4. **One-pass materialization (`tombstoneHandler` in `filter_delta_handlers.go`):**
   - When `mods.HasModifications()` is true, `fwdDedupTable` digests `mods` (including the code-252 `AttrModRemove` op) so identical eBGP RS-clients in one fan-out rebuild once and copy into each destination's `peerPool` buffer.
   - `buildForwardPayload` -> `buildModifiedPayload` -> `planAttr` dispatches code 252 to `tombstoneHandler`.
   - `tombstoneHandler` computes `clearMask` by OR-ing every byte from `AttrModRemove` ops on code 252 (validating non-empty/known mask bits; refusing invalid ops fail-closed if ever malformed).
   - If `lastSetOrSuppress` ends in `suppress`: `p.Drop()`.
   - If `p.Source() != nil` (received marker, whether alone or with a local `AttrModSet` that keeps the upstream marker with a warning): reads `origFlags := p.Source()[0]`, computes `outFlags := origFlags &^ clearMask`.
     - If `outFlags == origFlags`: `p.KeepAll()` (coalesces verbatim with untouched attributes).
     - Otherwise: `p.Keep(0, len(p.Value()))` (valid even when `len(p.Value()) == 0`), then calls `p.EmitExtended(outFlags, byte(attribute.AttrTombstone))` if `origFlags & FlagExtLength != 0` else `p.Emit(outFlags, byte(attribute.AttrTombstone))`. Optional (`0x80`), Partial (`0x20`), Extended Length (`0x10`), and every value byte are preserved verbatim; only Transitive (`0x40`) clears.
   - If `p.Source() == nil` and `setIdx >= 0` (egress-generated marker from `ASPathEdit.recordAggregatorDiscard`): computes `outFlags := tombstoneFlags(p.Ops()[setIdx].Buf) &^ clearMask`, calls `p.Op(setIdx)` and `p.Emit(outFlags, byte(attribute.AttrTombstone))` (Partial stays 0 per Section 4.2, Transitive clears to 0 on eBGP egress).
   - If `p.Source() == nil` and `setIdx < 0`: `p.Drop()`.
5. **Wire dispatch:** The per-destination `peerPool` buffer goes on the wire through the existing `buildFwdBody` / `fwdPool` / direct-write path; the shared received `WireUpdate` bytes are never modified.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shared received wire ↔ per-destination `peerPool` buffer | `buildForwardPayload` / `buildModifiedPayload` copies modified bytes into destination's `peerPool` slot; iBGP destinations continue referencing shared wire | No |
| Forward rails ↔ `tombstoneHandler` | `ModAccumulator.Op(AttrTombstone, AttrModRemove, tombstoneClearTransitiveMask[:])` digested by `fwdDedupTable` and consumed by `tombstoneHandler` | No |
| `ASPathEdit.recordAggregatorDiscard` ↔ `applyFactsTombstone` | `applyFactsTombstone` runs immediately after `aspathEdit.Record` on both rails so generated markers receive the same eBGP Transitive clear | No |

### Integration Points
- `forwardUpdateSection` (`internal/component/bgp/reactor/reactor_api_forward.go`) — ordinary/cached forward rail, also used by `ForwardUpdatesDirect` and `RelayStoredRoute`.
- `reactorForwardRSSectionResult` (`internal/component/bgp/reactor/forward_rs.go`) and `rsBase` (`forward_rs_chain.go`) — reactor-native RS fast-path rail.
- `tombstoneHandler` (`internal/component/bgp/reactor/filter_delta_handlers.go`) — registered handler for code 252 in `attrModHandlersWithDefaults`.
- `fwdDedupTable` (`internal/component/bgp/reactor/forward_dedup.go`) — automatically deduplicates identical eBGP RS-client rebuilds for marker-bearing UPDATEs.

### Architectural Verification
- [ ] No bypassed layers (data flows through `ModAccumulator` -> `buildForwardPayload` -> `tombstoneHandler` on both rails)
- [ ] No unintended coupling (no plugin imports, no config/YANG changes, no `filterapi` ABI changes)
- [ ] No duplicated functionality (one `forward_tombstone.go` helper shared by both rails; reuses `buildModifiedPayload` and `fwdDedupTable`)
- [ ] Zero-copy preserved for marker-free UPDATEs and already non-transitive markers (zero `ModAccumulator` ops recorded)
- [ ] Shared received wire never mutated; iBGP destinations in the same fan-out receive unmodified bytes

## Risks & Assumptions

| ID | Assumption / Risk | Basis / Early signal | Validation / Mitigation | Status |
|----|-------------------|----------------------|-------------------------|--------|
| A-1 | Ze never sets Partial (`0x20`) on `ATTR_TOMBSTONE` at generation or opaque-attribute normalization | Verified in HEAD (`grep` over non-test `internal/component/bgp/` and `internal/core/bgp/`): `attrDiscardFlags` and `WriteTombstone` mask with `0x50`; `AttrTombstone` is registered in `coreAttributes` (`flags_spec.go`), so `Recognized()` is true and both `forwardOpaquePayload` and `SetPartialOnUnrecognizedTransitive` skip it; `ClearPartialOnWellKnownAndNonTransitive` clears Partial only on non-transitive optional attributes at ingress | Unit tests assert Partial=1 and Partial=0 received transitive markers preserve received Partial on both iBGP and eBGP egress, while locally generated markers keep Partial=0 | confirmed |
| A-2 | The old whole-payload prepend rail (`rewriteASPathPrepend`) is gone; the live seam is `ModAccumulator` + `tombstoneHandler` + `buildModifiedPayload` | Verified in HEAD: `aspath_slot.go` documents the 2026-09-09 deletion of `RewriteASPath` and both live rails rebuild via `buildForwardPayload` | All edits ride `ModAccumulator` and `tombstoneHandler`; `forward_build.go` needs no modification | confirmed |
| A-3 | Both ordinary eBGP and RS-client eBGP currently omit the Section 5.3 Transitive clear on both live rails | Verified in HEAD by reading `reactor_api_forward.go`, `forward_rs.go`, and `filter_delta_handlers.go` | TDD red phase fails on both rails for both ordinary eBGP and RS-client eBGP before the fix lands | confirmed |
| R-1 | Marker handling adds allocations or rebuilds to marker-free UPDATEs (regressing RS fast-path throughput) | Benchmark or alloc-count test shows >0 rebuilds or added allocs on marker-free UPDATEs | Hoist allocation-free `payloadHasTransitiveTombstone` once per UPDATE; record nothing on `ModAccumulator` when absent or already non-transitive; pin with `TestTombstoneNoMarkerNoRebuild` and allocation check | open |
| R-2 | Clearing Transitive on a received marker accidentally alters Partial, Extended Length header class, or value bytes, or mutates the shared received buffer | Wire diff between received and eBGP/iBGP outputs shows unintended byte changes | `tombstoneHandler` masks only `FlagTransitive`, preserves `FlagExtLength` via `EmitExtended`, keeps value via `p.Keep(0, len(p.Value()))`, and tests assert exact full-frame bytes and unchanged source buffer | open |
| R-3 | Egress-generated markers (`recordAggregatorDiscard`) or upstream-plus-local-set collisions bypass the eBGP Transitive clear | eBGP peer receives `0xC0` when malformed AGGREGATOR is discarded on egress | Run `applyFactsTombstone` after `aspathEdit.Record` on both rails and apply `clearMask` in `tombstoneHandler` on both the `p.Source() != nil` branch and the `p.Source() == nil` `AttrModSet` branch | open |
| R-4 | A non-recognizing peer (GoBGP) rejects or mishandles the optional non-transitive code 252 attribute | Interop scenario fails session or drops route | Verify GoBGP's optional non-transitive unknown attribute handling in source and prove end-to-end in `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` with wire capture | open |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Cached/general forward (`forwardUpdateCore` -> `forwardUpdateSection`) with transitive `ATTR_TOMBSTONE` to ordinary eBGP, RS-client eBGP, and iBGP | → | `payloadHasTransitiveTombstone` -> `applyFactsTombstone` -> `tombstoneHandler` | `TestForwardTombstoneInheritDefaultBothRails` (`internal/component/bgp/reactor/tombstone_forward_test.go`) and `test/plugin/tombstone-policy-inherit-default.ci` |
| Route-server fast path (`reactorForwardRS` -> `reactorForwardRSSectionResult`) with transitive `ATTR_TOMBSTONE` to RS-client eBGP, ordinary eBGP, and iBGP | → | `rsBaseOf`/`rsBaseOverride` -> `applyFactsTombstone` -> `tombstoneHandler` | `TestForwardTombstoneInheritDefaultBothRails` (`internal/component/bgp/reactor/tombstone_forward_test.go`) and `test/plugin/tombstone-policy-inherit-default.ci` |
| Egress `ASPathEdit.recordAggregatorDiscard` generating `ATTR_TOMBSTONE` toward eBGP | → | `applyFactsTombstone` (post-`aspathEdit.Record`) -> `tombstoneHandler` | `TestForwardTombstoneAggregatorDiscardClearsTransitiveOnEBGP` (`internal/component/bgp/reactor/tombstone_forward_test.go`) |
| Marker-free UPDATE on RS fast path and general rail | → | `payloadHasTransitiveTombstone` returns false -> `applyFactsTombstone` records no op -> zero-copy fast path | `TestForwardTombstoneNoMarkerZeroCopy` (`internal/component/bgp/reactor/tombstone_forward_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No policy configured, UPDATE carrying a transitive `ATTR_TOMBSTONE` (code 252) forwarded to ordinary eBGP and RS-client eBGP destinations through both live rails (`forwardUpdateCore` and `reactorForwardRS`), including export-override, partitioned-section, and egress-generated `recordAggregatorDiscard` markers | Transitive bit (`0x40`) is cleared on every eBGP destination's wire output (ordinary and RS-client alike); Optional (`0x80`), Extended Length (`0x10`) header class, marker value bytes, and all unrelated attributes remain unchanged; shared received wire bytes remain untouched. |
| AC-6 | Received transitive `ATTR_TOMBSTONE` under default `inherit`, with both Partial-set (`0xE0`) and Partial-clear (`0xC0`) controls, forwarded to iBGP, ordinary eBGP, and RS-client eBGP destinations | Received Partial stays unchanged (`0xE0` -> `0xA0` and `0xC0` -> `0x80` on both eBGP peer classes; `0xE0` -> `0xE0` and `0xC0` -> `0xC0` on iBGP), per the 2026-08-07 ruling; locally generated markers (`attrDiscardFlags`, `WriteTombstone`, `recordAggregatorDiscard`) retain their separate Section 4.2 Partial-clear rule (and clear Transitive on eBGP egress). |
| AC-7 | Received non-transitive `ATTR_TOMBSTONE` (`0x80` / `0x90`) under default `inherit`, forwarded to iBGP, ordinary eBGP, and RS-client eBGP destinations | Remains non-transitive without physical removal and without changing received Partial or value bytes, per D-1's resolved reading (b); triggers no unnecessary rebuild on RS-client eBGP when no other edit applies. |
| AC-8-default | UPDATE carrying no `ATTR_TOMBSTONE` marker (and generating none), forwarded on either live rail | Preserves the zero-copy fast path with zero rebuilds triggered by tombstone handling and zero added allocations per UPDATE/destination. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardTombstoneInheritDefaultBothRails` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-1: transitive marker (`0xC0`) clears to `0x80` for ordinary eBGP and RS-client eBGP on both `forwardUpdateCore` and `reactorForwardRS`; shared received wire is never mutated; fan-out dedup (`fwdMaterializations`) rebuilds once for identical RS-clients | |
| `TestForwardTombstoneInheritPreservesPartial` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-6 & AC-7: full 2x2 matrix of received `(Transitive={0,1}, Partial={0,1})` across iBGP, ordinary eBGP, and RS-client eBGP on both rails; verifies `0xE0 -> 0xA0`, `0xC0 -> 0x80` on eBGP, unchanged on iBGP, and non-transitive kept without removal | |
| `TestForwardTombstoneEdgeShapes` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-1 & Section 5.2.3: preserves Extended Length 4-byte header (`0xD0 -> 0x90`, `0xF0 -> 0xB0`), 0-byte, 1-byte, odd-length, and in-place zero-padded values; fails closed on malformed duplicate `ATTR_TOMBSTONE` override; handles partitioned mixed-NLRI UPDATEs and export `wireOverride` introducing a transitive marker | |
| `TestForwardTombstoneAggregatorDiscardClearsTransitiveOnEBGP` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-1 & AC-6: malformed `AGGREGATOR` discarded by `ASPathEdit.recordAggregatorDiscard` emits `ATTR_TOMBSTONE` with flags `0x80` (Transitive=0, Partial=0) on eBGP egress, and also clears Transitive when an upstream transitive marker coincides with a local discard | |
| `TestForwardTombstoneNoMarkerZeroCopy` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-8-default & AC-7: marker-free UPDATE and already non-transitive marker UPDATE record zero `ModAccumulator` ops on RS-client eBGP, trigger zero `fwdMaterializations`, and add zero heap allocations in `payloadHasTransitiveTombstone` and `applyFactsTombstone` | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ATTR_TOMBSTONE` value length (Section 5.2.3) | 0..65535 octets (all lengths valid; 0, 1, odd, 255, 256) | 65535 (bounded by UPDATE section ceiling) | N/A | >65535 (rejected by `buildModifiedPayload` `modifyFailureAttrLenRange`) |
| `ATTR_TOMBSTONE` flags octet | Optional (`0x80`) set; Transitive (`0x40`), Partial (`0x20`), ExtLen (`0x10`) combinations | `0xF0` -> `0xB0` on eBGP | N/A | Reserved bits cleared at final session normalization (`normalizeTransmittedUpdate`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tombstone-policy-inherit-default` | `test/plugin/tombstone-policy-inherit-default.ci` | Real `ze` daemon with one source and three destinations (ordinary eBGP, RS-client eBGP, iBGP) receiving an UPDATE with transitive `ATTR_TOMBSTONE` (both Partial=0 and Partial=1 controls) plus a non-transitive control and a marker-free control; exact wire-hex assertions prove Transitive clears on both eBGP classes, Partial is preserved, iBGP keeps Transitive, and shared wire is untouched | |
| `prefixsid-ebgp-discard-single-walk` (updated comment only) | `test/plugin/prefixsid-ebgp-discard-single-walk.ci` | Existing iBGP receive-time in-place marker test continues passing unchanged with its stale `rewriteASPathPrepend` comment corrected | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-tombstone-ebgp-inherit-gobgp` | `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` | GoBGP | Ze relays an UPDATE carrying a transitive `ATTR_TOMBSTONE` to an eBGP GoBGP peer (and an eBGP RS-client GoBGP peer) through a transparent wire-capture relay: the captured wire proves Transitive is cleared (`0x80` / `0xA0`), and GoBGP accepts the route and silently ignores the optional non-transitive code 252 attribute per RFC 4271 Section 5 without session reset or route drop. Discrimination check: reverting `applyFactsTombstone` causes the captured wire check to go red with `0xC0` | |

## Files to Modify

- `internal/component/bgp/reactor/reactor_api_forward.go` - hoist `srcHasTransitiveTombstone` once per UPDATE, re-check when `peerBaseWire != sourceWire` (export override or partitioned section), and invoke `applyFactsTombstone` on eBGP destinations after `aspathEdit.Record`.
- `internal/component/bgp/reactor/forward_rs.go` - hoist `srcHasTransitiveTombstone` once per UPDATE, pass into `rsBaseOf`, and invoke `applyFactsTombstone` on eBGP destinations after `aspathEdit.Record`.
- `internal/component/bgp/reactor/forward_rs_chain.go` - add `hasTransitiveTombstone` to `rsBase`, populated in `rsBaseOf` and recomputed in `rsBaseOverride`.
- `internal/component/bgp/reactor/filter_delta_handlers.go` - extend `tombstoneHandler` to apply `AttrModRemove` flag-clear masks (`FlagTransitive`) to both received markers (preserving Partial, Extended Length header class, and value bytes) and egress-generated `AttrModSet` markers, including when both an upstream marker and a local `AttrModSet` arrive together.
- `internal/component/bgp/message/attr_discard.go` - update the stale doc comment on `attrDiscardFlags` to name `applyFactsTombstone` on the live forward rails instead of claiming Section 5.3 is unenforced.
- `test/plugin/prefixsid-ebgp-discard-single-walk.ci` - correct the stale comment citing deleted `wireu.rewriteASPathPrepend` to name `applyFactsTombstone` on the live forward rails.
- `docs/architecture/wire/attributes.md` - update `## ATTR_TOMBSTONE (Code 252)` to document the restored Section 5.3 eBGP Transitive-bit clear on both live forwarding rails (superseding the 2026-09-09 retirement note).
- `docs/architecture/bgp/egress-attribute-rules.md` - document the `ATTR_TOMBSTONE` eBGP Transitive-bit clear rule alongside `LOCAL_PREF` and `MULTI_EXIT_DISC`, including its shared once-per-UPDATE presence scan and proof entries.

## Files to Create

- `internal/component/bgp/reactor/forward_tombstone.go` - `payloadHasTransitiveTombstone`, `tombstoneClearTransitiveMask`, `tombstoneOpPresent`, and `applyFactsTombstone`.
- `internal/component/bgp/reactor/tombstone_forward_test.go` - unit and zero-alloc regression tests covering AC-1, AC-6, AC-7, AC-8-default, edge shapes, dedup, and `recordAggregatorDiscard` on both rails.
- `test/plugin/tombstone-policy-inherit-default.ci` - functional test proving ordinary eBGP, RS-client eBGP, iBGP, Partial controls, non-transitive retention, and shared-wire isolation.
- `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` - GoBGP interop scenario with transparent wire capture proving eBGP Transitive clearing and non-recognizing peer acceptance.

### Integration Checklist
- [ ] YANG schema / CLI grammar / doctor checks: N/A — this fix restores the default `inherit` eBGP wire behavior with no new configuration surface; configurable policy selection belongs to `plan/post-release/features/spec-tombstone-policy-selection.md`.
- [ ] Functional test added and promoted (`test/plugin/tombstone-policy-inherit-default.ci`) and existing comment corrected (`test/plugin/prefixsid-ebgp-discard-single-walk.ci`).
- [ ] Interop scenario added (`test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/`) with GoBGP source verification and red-on-revert discrimination proof.

### Documentation Update Checklist
- [ ] `docs/architecture/wire/attributes.md` — updated (`departs` row: replaces the 2026-09-09 "Section 5.3 clear is NOT implemented" note in `## ATTR_TOMBSTONE (Code 252)` with the restored eBGP clear on both rails).
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` — updated (adds `ATTR_TOMBSTONE` eBGP Transitive-clear rule and proof rows alongside `LOCAL_PREF` and `MULTI_EXIT_DISC`).
- [ ] `docs/architecture/core-design.md` — verified unaffected: `Progressive Build (applyMods)` and `Forwarding Path` already describe generic registered `AttrModHandler`s and zero-copy when `mods.Len() == 0`; no pipeline architecture changes.
- [ ] `docs/architecture/buffer-architecture.md` — verified unaffected: `When a copy is deliberate` already lists `buildModifiedPayload` for per-destination attribute rewrites into `peerPool`, and marker-free UPDATEs remain zero-copy.
- [ ] `docs/architecture/bgp/structural-forwarding.md` — verified unaffected: fast-rail `rsBase`, `fwdDedupTable`, and direct-write contracts are unchanged; only `hasTransitiveTombstone` joins `hasLocalPref` and `hasPrefixSID` on `rsBase`.
- [ ] `docs/architecture/wire/messages.md` — verified unaffected: `attr_discard.go` receives only a doc-comment update; BGP message header and body framing are unchanged.

## Design presentation

### What changes
- Restore the default `draft-mangin-idr-attr-tombstone-00` Section 5.3 eBGP Transitive-bit clear on both live forwarding rails (`forwardUpdateSection` and `reactorForwardRSSectionResult`) for both ordinary eBGP peers and route-server eBGP clients, via a new sibling helper `forward_tombstone.go` and an `AttrModRemove` flag-clear extension in `tombstoneHandler`.
- Preserve the zero-copy fast path (zero rebuilds, zero added allocations) on all marker-free UPDATEs and already non-transitive markers by hoisting one allocation-free attribute-section scan per UPDATE above each destination loop and leveraging `fwdDedupTable` for multi-client RS fan-outs.
- Update stale comments and architecture docs (`docs/architecture/wire/attributes.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `internal/component/bgp/message/attr_discard.go`, `test/plugin/prefixsid-ebgp-discard-single-walk.ci`).

### Before and after
- **Before:** An UPDATE carrying a transitive `ATTR_TOMBSTONE` (flags `0xC0` or `0xE0`) received from upstream or stamped on ingress leaves Ze with `Transitive=1` (`0xC0` or `0xE0`) on both ordinary eBGP and RS-client eBGP sessions, allowing external non-recognizing peers to propagate the marker further across AS boundaries; `docs/architecture/wire/attributes.md` documents the Section 5.3 clear as unimplemented.
- **After:** The same UPDATE leaves Ze with `Transitive=0` (`0x80` when received Partial=0, `0xA0` when received Partial=1) on both ordinary eBGP and RS-client eBGP sessions, materialized into each destination's own `peerPool` buffer (deduplicated once per RS fan-out group), while iBGP destinations still receive the unmodified shared wire (`0xC0`/`0xE0`), non-transitive markers stay non-transitive without removal, and marker-free UPDATEs remain 100% zero-copy.

### Alternatives set aside
- **Exempting route-server eBGP clients to keep RS marker-bearing forwarding zero-copy:** Rejected by Thomas on 2026-11-24; eBGP RS clients are external peers and must clear Transitive too, while paying the copy/edit cost only on marker-bearing UPDATEs (and sharing one rebuild per fan-out via `fwdDedupTable`).
- **In-place mutation of the shared received wire buffer:** Rejected because one received `WireUpdate` is shared across iBGP and eBGP destinations; mutating it in place would corrupt iBGP output and violate buffer lifetime/immutability invariants.
- **Reviving the deleted whole-payload `wireu.rewriteASPathPrepend` rail or modifying `forward_build.go`:** Rejected as unnecessary layering; the live `ModAccumulator` + `tombstoneHandler` + `buildModifiedPayload` + `fwdDedupTable` pipeline already supports per-attribute planning and exact-sized pooling without changing `forward_build.go` or `filterapi`.
- **Adding a new `filterapi.AttrModAction` enum constant for flag clearing:** Rejected because `AttrModRemove` carrying a static 1-byte bitmask (`tombstoneClearTransitiveMask`) already matches the `prefixSIDNextHopHandler` (`srv6ServiceTLVTypes`) pattern, enters `AppendEditDigest` automatically, and avoids widening `filterapi` for one handler.

### Documents departed from
- `docs/architecture/wire/attributes.md` (`## ATTR_TOMBSTONE (Code 252)`):
  - **Before:** States `**Egress flag handling: the Section 5.3 clear is NOT implemented.**` and records that the clear stopped running after `wireu.ASPathEdit` replaced the old whole-payload AS_PATH rewrite on 2026-09-09.
  - **After:** Updated in the same change to document that both live forwarding rails (`forwardUpdateSection` and `reactorForwardRSSectionResult`) now clear the Transitive bit on `ATTR_TOMBSTONE` for ordinary and RS-client eBGP destinations via `applyFactsTombstone` and `tombstoneHandler`, preserving received Partial, Extended Length, value bytes, iBGP pass-through, and the marker-free zero-copy fast path.

### Choices for Thomas
none

## Implementation Steps

1. **Phase 1: Wiring and failing unit tests (TDD red).** Create `internal/component/bgp/reactor/tombstone_forward_test.go` exercising `forwardUpdateCore` and `reactorForwardRS` with ordinary eBGP, RS-client eBGP, and iBGP destinations for AC-1, AC-6, AC-7, AC-8-default, edge shapes, and `recordAggregatorDiscard`. Confirm tests fail against HEAD because eBGP destinations retain Transitive=1.
2. **Phase 2: Implement `forward_tombstone.go`, `tombstoneHandler`, and rail hooks (TDD green).**
   - Create `internal/component/bgp/reactor/forward_tombstone.go` with `payloadHasTransitiveTombstone`, `tombstoneClearTransitiveMask`, `tombstoneOpPresent`, and `applyFactsTombstone`, carrying verbatim `draft-mangin-idr-attr-tombstone-00` Section 5.3 quotes.
   - Extend `tombstoneHandler` in `internal/component/bgp/reactor/filter_delta_handlers.go` to apply `AttrModRemove` flag-clear masks while preserving Partial, Extended Length (`EmitExtended`), and value bytes on both received markers (including upstream-plus-local-set) and egress-generated `AttrModSet` markers.
   - Wire `payloadHasTransitiveTombstone` and `applyFactsTombstone` into `reactor_api_forward.go`, `forward_rs.go`, and `forward_rs_chain.go` (immediately after `aspathEdit.Record`).
   - Confirm all unit tests in `internal/component/bgp/reactor` pass with `-tags ze_bgp`.
3. **Phase 3: Functional test, interop scenario, and doc/comment updates.**
   - Create `test/plugin/tombstone-policy-inherit-default.ci` (drafting outside `test/`, proving under load, confirming red-on-revert before promoting per `/ze-test`).
   - Create `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` after checking GoBGP's optional non-transitive unknown attribute handling in source; prove red-on-revert discrimination.
   - Update `internal/component/bgp/message/attr_discard.go`, `test/plugin/prefixsid-ebgp-discard-single-walk.ci`, `docs/architecture/wire/attributes.md`, and `docs/architecture/bgp/egress-attribute-rules.md`.
4. **Phase 4: Scoped main-thread verification, independent review, and closure.**
   - Main thread runs scoped tests and gates:
     - `./le test unit pkg path internal/component/bgp/reactor`
     - `./le test unit pkg path internal/component/bgp/message`
     - `./le test functional plugin` (or targeted `tombstone-policy-inherit-default` and `prefixsid-ebgp-discard-single-walk`)
     - `./le test integration interop bgp-tombstone-ebgp-inherit-gobgp`
     - `./le go lint run scope "internal/component/bgp/reactor"`
     - `./le arch tier check` and `./le plugin boundary check`
   - Complete `/ze-review` and `/ze-review-spec` Review Gate, then close with `/ze-close` (Commit A + Commit B via `./le commit create`).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## RFC Documentation (Scope: protocol)

Add `// draft-mangin-idr-attr-tombstone-00 Section 5.3: "<quoted requirement>"` above enforcing code in `forward_tombstone.go` and `tombstoneHandler`, and `// RFC 4271 Section 5: "<quoted requirement>"` where optional non-transitive/Partial semantics are cited.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| `draft-mangin-idr-attr-tombstone-00` Section 5.3 (`inherit` default) | Unconfigured peer/group clears Transitive (`0x40`) on `ATTR_TOMBSTONE` toward ordinary eBGP and RS-client eBGP peers on both live rails; leaves received Partial unchanged; leaves iBGP markers unchanged; leaves non-transitive markers non-transitive without physical removal | Explicit per-peer/group `strip`/`propagate` selection is owned by `plan/post-release/features/spec-tombstone-policy-selection.md`; `tombstoneHandler` preserves `AttrModSuppress` support for future `strip` | `TestForwardTombstoneInheritDefaultBothRails`, `TestForwardTombstoneInheritPreservesPartial`, `tombstone-policy-inherit-default.ci`, `bgp-tombstone-ebgp-inherit-gobgp` | `TestForwardTombstoneNoMarkerZeroCopy`, `TestForwardTombstoneEdgeShapes`, `prefixsid-ebgp-discard-single-walk.ci` |
| RFC 4271 Section 5 (optional non-transitive and Partial handling) | Non-recognizing eBGP peer silently ignores optional non-transitive `ATTR_TOMBSTONE` (Transitive=0); recognizing forwarder preserves received Partial under `inherit` | `preserve-opaque-attributes` (`bgp-rs:transparency`) does not interfere because `AttrTombstone` is recognized | `bgp-tombstone-ebgp-inherit-gobgp`, `TestForwardTombstoneInheritPreservesPartial` | `TestForwardTombstoneInheritDefaultBothRails` (RS-client with transparency active) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill during implementation) | file:line | (fill during implementation) |

### Fixes applied
- (fill during implementation)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Default eBGP Transitive-bit clear restored on both live rails for ordinary eBGP and RS-client eBGP without mutating shared wire | Unit + functional wire-hex proof | (fill during implementation: `TestForwardTombstoneInheritDefaultBothRails`, `test/plugin/tombstone-policy-inherit-default.ci`) |
| Received Partial preserved on forwarding; iBGP and non-transitive controls unchanged; marker-free zero-copy fast path preserved | Unit + allocation + functional proof | (fill during implementation: `TestForwardTombstoneInheritPreservesPartial`, `TestForwardTombstoneNoMarkerZeroCopy`) |
| Non-recognizing external BGP peer accepts the route and silently ignores the non-transitive marker | Interop scenario + wire capture | (fill during implementation: `test/interop/scenarios/bgp-tombstone-ebgp-inherit-gobgp/` with red-on-revert record) |

## Checklist

- [ ] The 2026-08-07 (D-1/D-2) and 2026-11-24 (RS-client) owner rulings are reflected in the implementation and tests
- [ ] AC-1, AC-6, AC-7, and AC-8-default all demonstrated with fresh evidence
- [ ] Wiring Test table complete — every row has a concrete passing test, none deferred
- [ ] Tests written, observed FAIL before fix, observed PASS after fix
- [ ] Boundary, functional, and interop tests pass (with red-on-revert discrimination verified)
- [ ] Scoped verification run by main thread: `./le test unit pkg path internal/component/bgp/reactor`, `./le test unit pkg path internal/component/bgp/message`, `./le test functional plugin`, `./le test integration interop bgp-tombstone-ebgp-inherit-gobgp`, `./le go lint run scope "internal/component/bgp/reactor"`, `./le arch tier check`, `./le plugin boundary check`
- [ ] Documentation Update Checklist completed (`docs/architecture/wire/attributes.md` and `docs/architecture/bgp/egress-attribute-rules.md` updated)
- [ ] Goal Validation table filled with concrete evidence
- [ ] `/ze-review` and `/ze-review-spec` Review Gate clean (0 BLOCKER, 0 ISSUE)
- [ ] Closure: append `plan/templates/TEMPLATE-CLOSURE.md`, land Commit A + Commit B (`remove plan/release/fixes/spec-tombstone-forwarding-policy.md`) via `./le commit create`

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split separates default eBGP conformance repair (this spec) from configurable policy selection (post-release feature). Every test, documentation, and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1, AC-6, AC-7, and AC-8-default with no policy configured; default eBGP Transitive clear on both live rails (ordinary and RS-client eBGP) | `plan/release/fixes/spec-tombstone-forwarding-policy.md` |
| Original AC-2 through AC-5 and AC-8 under explicit policy; YANG per-peer/group `inherit`/`strip`/`propagate` selection, neighbor override, physical removal (`strip`), and `propagate` Transitive-set/Partial-clear | `plan/post-release/features/spec-tombstone-policy-selection.md` |

## Historical Combined Specification

Preserved provenance from the pre-split spec (`2026-07-16` through `2026-10-22`) for `plan/post-release/features/spec-tombstone-policy-selection.md`. Active ownership for this spec is defined strictly by the sections above.

### Historical draft ambiguities resolved 2026-08-07 (Thomas)

- **D-1 ("not forwarded" under `inherit`):** Section 5.3 says `"Non-transitive ATTR_TOMBSTONE: not forwarded."` and `"Non-transitive ATTR_TOMBSTONE markers are silently ignored by non-recognizing speakers per RFC 4271 Section 5 and are not forwarded by recognizing speakers under the default 'inherit' policy."` Thomas ruled on 2026-08-07 (*"it can be done by removing the transitive bit when going to an ebgp peer"*) that reading (b) governs: clearing Transitive at the eBGP boundary achieves containment without forcing a physical-removal rebuild on the default path. Physical removal belongs to the explicit `strip` policy (`spec-tombstone-policy-selection.md`), which MUST rebuild path attributes rather than merely masking Transitive.
- **D-2 (Partial bit under `inherit`):** Section 5.3's default list says `"Transitive ATTR_TOMBSTONE: forwarded to peers with Partial bit set (RFC 4271 Section 5)"` (describing an upstream non-recognizing speaker), while its `propagate` bullet says `"A recognizing speaker that explicitly forwards the attribute MUST clear the Partial bit (setting it to 0), even if it was set by an intermediate non-recognizing speaker"`. Thomas ruled on 2026-08-07 that Partial is not a lever pulled under `inherit`: origination (`attrDiscardFlags` / `WriteTombstone`) clears Partial per Section 4.2, and forwarding under `inherit` leaves received Partial untouched while clearing only Transitive at eBGP egress.
- **Draft wording note:** Revisions to `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` remain Thomas's own IETF work and are never edited by agents.

### Historical deferral provenance (2026-07-16 / 2026-08-30 / 2026-09-09)

- Originally deferred from `fixit-tombstone-ebgp-transitive.md` (2026-07-16) as an RS-client bypass plus configurable `inherit`/`strip`/`propagate` policy.
- Corrected 2026-08-30 and 2026-09-09: the old `wireu.rewriteASPathPrepend` helper that once held `clearTombstoneTransitiveInBody` was dead and deleted on 2026-09-09 when both live rails moved to `wireu.ASPathEdit` + `buildModifiedPayload`, leaving neither ordinary eBGP nor RS-client eBGP with a live Section 5.3 clear.
- On 2026-10-22, the configurable `inherit`/`strip`/`propagate` surface (original AC-2..AC-5 and explicit-policy AC-8) split into `plan/post-release/features/spec-tombstone-policy-selection.md`.
- On 2026-11-24, Thomas settled the remaining RS-client design question: apply the default eBGP Transitive-bit clear to route-server eBGP clients too, paying the per-destination buffer copy/edit only on UPDATEs that actually carry an `ATTR_TOMBSTONE` marker.
