# Spec: fixit-private-asn-leak

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/fail-closed-guards.md`, `ai/rules/no-fabrication.md`
4. `internal/component/bgp/reactor/egress_inject_filter.go`, `filter_ordered.go`

## Task

Root-cause and fix a private-ASN leak on Ze's BGP egress path. With
`filter { export [ remove-private-as:STRIP ] }` configured on an EBGP peer, ze
delivered `AS_PATH [64496 64512 64497]` to that peer: RFC 6996 Private Use ASN
64512 was NOT removed. **RFC 6996 Section 4 is a MUST.** Reachable in production.

Origin: `plan/spec-fixit-redistribute-establishment-stall.md` rows B2.3/B2.4,
which MEASURED the leak (3/18 runs) but explicitly left the mechanism as an
unverified hypothesis.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/fail-closed-guards.md` - the leak is a textbook zero-value/permissive-no-op guard
  → Constraint: "A guard that neither denies nor speaks does not exist." The egress
    gate ran the filter chain, got a valid Modify answer, and returned "no change".
  → Constraint: "Drive the guard from the entry point that triggers it." A unit test on
    the chain helper proves nothing about whether the write path reaches it correctly.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md`
  → Constraint: fix at the owning layer. The owning layer is the egress gate, not the callers.
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - forward path is perf-critical
  → Constraint: the override slice must not alias the session `writeBuf` it is written back into.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6996.md` - the governing MUST
  → Constraint: S4 "Private Use ASNs MUST be removed from AS path attributes (including
    AS4_PATH ...) before being advertised to the global Internet."
  → Constraint: S5 ranges are 64512-65534 and 4200000000-4294967294 inclusive. In the
    fixture, 64512 is the ONLY private ASN; 64496/64497 are RFC 5398 documentation ASNs
    and MUST be retained.
  → Decision: the remove-vs-replace CHOICE is vendor policy, unspecified by RFC. Ze's
    `replace-with peer-as` is a design decision. The MUST is only "must not reach EBGP".
- [ ] `rfc/short/rfc4271.md` - S9.1.2 EBGP AS_PATH prepend (context only, see Known Limitations)

**Key insights:**
- The filter itself was never wrong. `rewritePrivateASSegments`
  (`filter_delta.go:645-669`) and `isRFC6996PrivateASN` (`:673`) implement RFC 6996
  exactly. The bug was that the filter's ANSWER was thrown away on one of the two
  egress paths, and on that same path the filter was never even shown the attributes.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - the single egress gate for
      every NON-forwarded outbound route. THE DEFECT SITE.
  → Constraint: pre-fix `:49-57` honored ONLY `PolicyReject` and a raw override, and
    returned `(false, nil)` for everything else. `(false, nil)` means "write the body
    unchanged". A `FilterModify` text delta therefore left no trace.
  → Constraint: pre-fix `:38` built the WireUpdate with ctxID **0**.
- [ ] `internal/component/bgp/reactor/filter_ordered.go:195-236` - `runEgressPolicyChain`,
      the FORWARDED-route twin. Handles the text delta correctly at `:217-234`
      (`textDeltaToModOps` + `ExtractRemovePrivateASOps` + `buildModifiedPayload`).
  → Decision: the two paths were independent copies of "run the export chain". They drifted.
    The fix collapses them onto one body rather than patching the copy.
- [ ] `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go:92`
  → Constraint: the plugin returns `Action: FilterModify, Update: delta` — a TEXT delta.
    It NEVER returns a raw override. `Raw: true` in its registration (`:60`) means it
    RECEIVES raw bytes, not that it emits them. So the pre-fix gate discarded 100% of
    this plugin's output. Same for every other text-delta filter (as-path prepend, ...).
- [ ] `internal/component/bgp/reactor/session_write.go:245,478` - `writeUpdate` / `SendAnnounce`
      call the gate; `peer_run.go:200` wires it.
- [ ] `internal/component/bgp/wireu/wire_update.go:95-109` - `Attrs()` builds
      `attribute.NewAttributesWire(attrBytes, u.sourceCtxID)` at `:106`.
  → Constraint: THE CONTEXT ID IS LOAD-BEARING. With ctxID 0 the attributes cannot be
    decoded, so `AppendUpdateForFilter` renders an attribute-less filter text.
- [ ] `internal/component/bgp/config/peers.go:156,167,202` - ExportFilters is resolved and
      canonicalized at CONFIG PARSE time, into PeerSettings, before any Peer exists.
- [ ] `internal/component/bgp/reactor/peer.go:513` - `refreshForwardFacts` at `setEncodingContexts`
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go:325-335` - the only late writer of
      `settings.ExportFilters` (`:332`), which already calls `refreshForwardFacts()` at `:335`.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go:130,280` + `server.go:386` -
      bgp-rs re-advertises via `plugin.UpdateRoute(... "update text ...")`, i.e. through the
      ORIGINATED path, NOT through `forwardUpdateCore`.
- [ ] `internal/component/bgp/reactor/forward_build.go:296-310` - `buildModifiedPayload` with
      a nil pool returns a freshly allocated slice (`:307`), not a view into the input.

**Behavior to preserve:**
- `runEgressPolicyChain`'s signature and semantics for the forwarded path (unchanged).
- The forwarded path's asn4 source (the update's SOURCE encoding context).
- Zero-cost when a peer has no export filters (the common case): unchanged early return.
- EOR exemption and the already-filtered `writeRawUpdateBody` path stay excluded.

**Behavior to change:**
- The originated/injected/replayed egress path now applies the FULL export chain result
  (text delta included), with the attributes actually rendered.

## Data Flow (MANDATORY)

### Entry Point
An outbound route for a peer with a non-empty export filter chain. Two disjoint classes:
1. **Forwarded/reflected** — `forwardUpdateCore` → `runEgressPolicyChain`. Correct pre-fix.
2. **Everything else** — API/plugin injection, redistribute, static routes, configured
   `update{}` blocks, bgp-adj-rib-in replay, **and bgp-rs `update text` re-advertisement**
   → `Session.writeUpdate` / `SendAnnounce` → `exportFilterForBody`. **Leaked pre-fix.**

### Transformation Path (class 2, pre-fix — the leak)
1. `session_write.go:245` `writeUpdate` calls `s.egressRouteFilter(body)`.
2. `peer_run.go:200-202` routes that to `Reactor.exportFilterForBody(peer, body)`.
3. `egress_inject_filter.go:38` (pre-fix) `wireu.NewWireUpdate(body, 0)`.
4. `wire_update.go:106` builds AttributesWire with ctxID 0 → attributes undecodable.
5. `AppendUpdateForFilter` renders `"nlri ipv4/unicast add 10.0.0.0/24"` — **no attributes**.
   (MEASURED: ZZTRACE `textIn="nlri ipv4/unicast add 10.0.0.0/24"`.)
6. The remove-private-as plugin looks for `as-path ` (`filter_remove_private_as.go:96`),
   finds none, and returns **FilterAccept**. It is not even asked the question.
7. Even had it answered Modify, `egress_inject_filter.go:49-57` (pre-fix) honored only
   Reject and raw, and returned `(false, nil)` = write body unchanged.
8. `writeUpdate` writes the original body. **64512 goes on the wire.**

→ Decision: the leak is TWO independent fail-open defects stacked in one function.
  Fixing only the delta handling still leaks (MEASURED: it did — the wire was unchanged
  until the ctxID was also fixed). Both are necessary; neither alone is sufficient.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session write ↔ export chain | `egressRouteFilter` closure | [ ] measured via functional test |
| Reactor ↔ filter plugin | text update over PolicyFilterChain/RPC | [ ] measured (ZZTRACE textIn/textOut) |
| Wire ↔ attributes | ctxID-carrying `AttributesWire` | [ ] measured (mutation A) |

### Integration Points
- `Session.writeUpdate` / `SendAnnounce` (`session_write.go:245,478`) — the only callers of the
  egress gate, via the `egressRouteFilter` closure installed at `peer_run.go:200-202`.
- `forwardUpdateCore` (`reactor_api_forward.go:488`) — the other consumer of the shared chain,
  via `runEgressPolicyChain`. Unchanged in behavior.
- `bgp-rs` (`plugins/rs/server.go:386`) — reaches the gate indirectly through `UpdateRoute`.

### Architectural Verification
- [ ] No bypassed layers: the gate now delegates to the same chain body the forward path uses.
- [ ] No duplicated functionality: the second copy of the chain is DELETED, not patched.
- [ ] Zero-copy preserved: unchanged early return when no export filters.

## Risks & Assumptions

### Assumptions
- **A1 (VERIFIED).** ExportFilters is populated at config parse, before peers exist
  (`config/peers.go:156,167,202`), so `refreshForwardFacts` (`peer.go:513`, at Established)
  always observes it for a statically configured peer.
- **A2 (VERIFIED).** `buildModifiedPayload` with a nil pool returns a caller-owned fresh
  slice (`forward_build.go:307`), so no aliasing with `writeBuf`. The fix copies anyway,
  because the raw-override branch has no such guarantee.
- **A3 (ASSUMED, not verified).** `facts.sendCtxID` is the context the body was encoded
  under for EVERY caller of the write gate. Verified for the API-injected path by
  measurement (the filter text renders correctly and the strip lands). NOT independently
  traced for redistribute / static / adj-rib-in replay.

### Risks
- **R1.** `r.api == nil` remains a silent fail-open in both `exportFilterForBody` and
  `runEgressPolicyChain` (`filter_ordered.go:196`): a peer WITH export filters and no API
  server sends unfiltered, silently. NOT addressed here; no evidence it is reachable while
  sessions are established, and inventing a fix for an untraced path is what
  `ai/rules/no-fabrication.md` forbids. **Left open, flagged for Thomas.**
  → **RULED 2026-07-16 (Thomas): trace reachability first, then decide fix vs fail-closed.**
    Homed in `plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md`
    (`Status | skeleton`). R1 stays a surviving risk of THIS spec (it is unfixed); it is no
    longer an unhomed one. Still correctly not fixed here: this spec's subject is the
    private-ASN MUST.
  → The homing trace found this spec's own citation is short. `:196` is exact for
    `runEgressPolicyChain`, but `exportFilterForBody`'s guard is at
    `egress_inject_filter.go:43` (with a second fail-open, `facts == nil`, fused in), and R1
    misses `filter_ordered.go:222` -- `runEgressPolicyChainASN4`, the shared body this spec
    extracted and the one `exportFilterForBody` actually reaches -- plus `:139` on ingress.
    **Five sites, not two.**
  → It also found the argument R1 was missing: reachability is the weaker case. The same
    `r.api == nil` condition already fails **closed, with a Warn**, at
    `filter_chain.go:368-371` (`policyFilterFunc`, marked `// fail-closed`) and
    `peer_initial_sync.go:718-722`. `policyFilterFunc` is only reached *through*
    `PolicyFilterChain` at `filter_ordered.go:147`/`:232`, both **after** the early returns
    at `:196`/`:222` -- so when api is nil, the correct guard is unreachable, pre-empted by
    the wrong one. One package, one condition, two loud denials and one silent accept.
    That contradiction needs no reachability proof to be worth fixing.
- **R2.** Widening the gate from "Reject/raw only" to the full chain means text-delta
  filters now actually take effect on originated routes. That is the POINT, but it is a
  behavior change for any deployment that had (unknowingly) been relying on the no-op.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| API `peer * update text as-path [.. 64512 ..]` on a peer with `filter { export [ remove-private-as:STRIP ] }` | → | `Session.writeUpdate` (`session_write.go:245`) → `Reactor.exportFilterForBody` → `runEgressPolicyChainASN4` | `remove-private-as-export-originated` |
| Same, `replace-with peer-as` | → | `exportFilterForBody` passing `facts.peerAS` as destPeerAS → `ExtractRemovePrivateASOps` | `remove-private-as-replace-originated` |
| Forwarded route on an export-filtered peer | → | `forwardUpdateCore` (`reactor_api_forward.go:488`) → `runEgressPolicyChain` → `runEgressPolicyChainASN4` | `community-strip`, `ze-plugin-test` suite (no regression) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route ORIGINATED via the API with `as-path [64496 64512 64497]` to an EBGP peer configured `export [ remove-private-as:STRIP ]` | Wire carries `AS_PATH [64496 64497]`; 64512 (0xFC00) absent. RFC 6996 S4 satisfied |
| AC-2 | Same, mode `replace-with peer-as` | Wire carries `AS_PATH [64496 65002 64497]`: the DESTINATION peer AS (0xFDEA), not the local AS (0xFDE8) |
| AC-3 | Code structure | Exactly ONE export-chain body (`runEgressPolicyChainASN4`), shared by both egress paths, so they cannot drift again |
| AC-4 | Each half of the fix reverted independently | Both functional tests go RED for each mutation, and GREEN only with both halves present |
| AC-5 | Full `make ze-plugin-test` + reactor unit tests | No regression on the forwarded path or elsewhere |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `export [ remove-private-as:STRIP ]` on an EBGP peer and originates/redistributes a route whose AS_PATH carries a private ASN | API/plugin `update text` -> `UpdateRoute` -> `Session.writeUpdate` -> `egressRouteFilter` -> `exportFilterForBody` -> `runEgressPolicyChainASN4` -> plugin text delta -> `buildModifiedPayload` -> wire | `remove-private-as-export-originated` |
| 2 | Same, but wants the private ASN replaced by the neighbour's AS rather than dropped | as above, `mode=peer-as`, substituting `facts.peerAS` | `remove-private-as-replace-originated` |
| 3 | Runs a route server (bgp-rs) with an export filter on a client | bgp-rs `UpdateRoute` (`rs/server.go:386`) -> same session write gate as story 1 | covered by story 1's path (same gate); see Known Limitations for the untraced replay variants |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none — justified below) | n/a | n/a | N/A |

NONE at this seam, and this is a deliberate, defensible choice, not a gap:
`Reactor.api` is a concrete `*pluginserver.Server` (`reactor.go:247`), not an interface,
so `exportFilterForBody` cannot be driven in a unit test without standing up a real plugin
server and a real filter plugin. `filter_ordered_property_test.go:6-8` already records this
conclusion for the same chain ("Standing up the full external policy chain (RPC/text
filters) in a unit property test is impractical"). Per `ai/rules/fail-closed-guards.md`
("drive the guard from the entry point that triggers it"), the functional test through the
real entry point is the STRONGER proof here, and it is fully deterministic (not flaky),
so it does not carry the usual "functional tests are racy" caveat.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RFC 6996 16-bit private ASN | 64512-65534 | 65534 | 64511 | 65535 |
| RFC 6996 32-bit private ASN | 4200000000-4294967294 | 4294967294 | 4199999999 | 4294967295 |

Boundary coverage already exists and is unchanged by this spec: `isRFC6996PrivateASN`
(`filter_delta.go:673`) is the single range predicate, covered by `filter_delta_test.go`.
This spec does not touch the predicate; it fixes whether its verdict is applied at all.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `remove-private-as-export-originated` | `test/plugin/*.ci` | Operator's export STRIP policy actually strips a private ASN from an originated route | PASS (RED pre-fix) |
| `remove-private-as-replace-originated` | `test/plugin/*.ci` | Operator's `peer-as` substitution uses the neighbour AS, on an originated route | PASS (RED pre-fix) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| n/a | n/a | n/a | Not required: this spec changes no wire ENCODING and adds no protocol feature. It fixes whether an already-correct, already-interop-tested rewrite is invoked. The wire format on both sides of the fix is ordinary RFC 4271 AS_PATH, asserted byte-exact by the two functional tests | N/A |

### Future (if deferring any tests)
- None deferred.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/reactor/egress_inject_filter.go` | delegate to the shared chain; pass `facts.sendCtxID` and `facts.sendASN4` |
| `internal/component/bgp/reactor/filter_ordered.go` | split `runEgressPolicyChain` into a thin ctx-resolving wrapper + shared `runEgressPolicyChainASN4(…, asn4)` |

## Files to Create

| File | Purpose |
|------|---------|
| `test/plugin/remove-private-as-export-originated.ci` | AC-1 |
| `test/plugin/remove-private-as-replace-originated.ci` | AC-2 |
| `plan/spec-fixit-private-asn-leak.md` | this spec |

## Implementation Steps

### Implementation Phases

| # | Phase | Files | Done when |
|---|-------|-------|-----------|
| 1 | Refute or confirm the originating spec's `refreshForwardFacts` hypothesis at the producer | (read-only) `peer_forward_facts.go`, `forward_rs.go`, `config/peers.go`, `reactor_dynamic.go` | Hypothesis refuted on timing AND byte grounds; recorded in Mistake Log |
| 2 | Get a DETERMINISTIC reproducer through a real entry point | scratch `.ci` | Leak reproduces 100%, byte-identical to the measured 3/18 leak |
| 3 | Extract the shared chain body | `filter_ordered.go` | `runEgressPolicyChainASN4` exists; `runEgressPolicyChain` is a thin ctx-resolving wrapper; forwarded path behavior unchanged |
| 4 | Delegate the originated gate to it, with the peer's send context | `egress_inject_filter.go` | Reproducer goes green; both defects (dropped delta, ctxID 0) closed |
| 5 | Promote the reproducer to two regression tests, mutation-test each half | `test/plugin/remove-private-as-{export,replace}-originated.ci` | Each mutation independently kills both tests |
| 6 | Regression + lint | - | `make ze-lint-changed`, `make ze`, `make ze-plugin-test`, reactor unit tests |

### Critical Review Checklist (/implement stage 6)
- [ ] The fix removes the duplicate chain rather than patching it (else it recurs)
- [ ] The override slice cannot alias the session `writeBuf`
- [ ] `asn4` is correct for BOTH callers (source ctx for forwarded, send ctx for originated)
- [ ] Zero-cost early return preserved for peers with no export filters
- [ ] Every claim in this spec cites a producing `file:line`, or is labelled ASSUMED

### Deliverables Checklist (/implement stage 10)
- [ ] `egress_inject_filter.go` delegates; no second chain implementation remains
- [ ] Two functional tests, both RED pre-fix, GREEN post-fix
- [ ] Both mutations independently kill both tests
- [ ] Open questions (EBGP prepend on originate; `r.api == nil` fail-open) flagged, not silently fixed

## Core Insight

The export policy chain existed twice. The forwarded copy was complete; the originated copy
handled only the two outcomes its author happened to need (Reject, raw) and silently
dropped the one outcome every text filter actually returns. A second, independent fail-open
sat in front of it: the body was parsed without the peer's encoding context, so the filter
was handed a text with no attributes and truthfully answered "nothing to change."

Both defects are the same shape: **a permissive default standing in for an answer nobody
asked for.** The `(false, nil)` return and the attribute-less filter text are both zero
values that downstream reads as a legitimate "no change" (`ai/rules/fail-closed-guards.md`,
the zero-value trap).

## Key Design Decisions

1. **Collapse, don't patch.** `exportFilterForBody` now calls the same body as
   `forwardUpdateCore`. Patching the copy would have fixed remove-private-as and left every
   other text filter broken on the same path, and left the drift free to recur.
2. **asn4 is the only legitimate difference** between the two callers, so it is the only
   thing parameterized. Forwarded wires are in the SOURCE peer's encoding; originated
   bodies are already in the DESTINATION's send encoding.
3. **Copy the override unconditionally.** `buildModifiedPayload`'s nil-pool path already
   allocates fresh, but the raw branch does not, and `writeRawUpdateBody` stages through
   the same `writeBuf` the body may alias. Depending on which branch produced the slice
   would be a latent aliasing bug.

## Known Limitations

- **The originated path does not prepend the local AS for EBGP peers.** The leaked wire
  had no 65000, and post-fix it still has none. For an API-injected route with an explicit
  `as-path`, that is arguably the operator's declared path, and `attributes.ci` shows the
  same verbatim-as-path behavior. Whether RFC 4271 S9.1.2 prepend SHOULD apply to
  API-originated routes on an EBGP session is a **separate open question, deliberately not
  answered here** — it is orthogonal to the private-ASN MUST and was not traced to a
  producer. Flagged for Thomas.
  → **ANSWERED 2026-07-16 (Thomas), with a design rather than a yes/no:** *"we should have
    an 'auto-added' filter added to the chain of ebgp peer to add our ASN to the peer when
    sending and another one to remove local pref."* So: yes prepend, but not as a
    special case -- as an auto-added entry on the EBGP peer's export chain, alongside a
    second auto entry stripping LOCAL_PREF. Homed in
    `plan/spec-fixit-private-asn-leak-deferred-ebgp-auto-filters.md` (`Status | skeleton`).
  → **That design cashes in this spec's Goal Gate.** "one shared chain body; a future
    outcome added to the chain is honored by both paths automatically" (Goal Validation,
    row 3) is exactly the invariant an auto chain entry needs to reach the originated path.
    This spec built the mechanism the ruling spends.
  → **Two corrections the homing trace found, both against this bullet:**
    1. "The originated path" is **two paths that disagree**. The `update text`/`BuildPlugin`
       path does not prepend (`message/update_build_plugin.go:58-69`), but the rib-commit
       path **does**: `rib/commit.go:444-464` prepends `c.ctx.LocalASN()` for non-IBGP,
       preserving verbatim only when `c.isIBGP()` (`:436-442`). This bullet is true of the
       first and false of the second.
    2. `attributes.ci` does not support the "same verbatim-as-path behavior" claim for the
       EBGP case: its peer1 is **IBGP** (`local 1`/`remote 1`, `attributes.ci:66-67`), so
       its verbatim AS_PATH and its `400504000000C8` LOCAL_PREF are the IBGP baseline. It
       says nothing about EBGP either way.
  → **And the trace found a live RFC MUST NOT violation next door**, which the second auto
    filter closes: RFC 4271 S5.1.5 ("MUST NOT include LOCAL_PREF in UPDATE messages sent to
    external peers") is enforced on ingress (`message/rfc7606.go:442-450`) and on
    origination (`update_build_plugin.go:97-105`, `rib/commit.go:341-344`) but **nowhere on
    the forwarded egress path** -- `grep -i localpref internal/component/bgp/wireu/*.go`
    returns zero non-test hits. An **IBGP-sourced route carrying LOCAL_PREF, forwarded to an
    EBGP destination, keeps LOCAL_PREF on the wire today.** Ingress discard hides this for
    EBGP-sourced routes only. Not this spec's bug and not fixed here; recorded because this
    spec's trace is what surfaced it.
- **R1 above:** `r.api == nil` fail-open, unaddressed here; ruled and homed 2026-07-16 (see
  Risks).
- The two pre-existing `.ci` files (`remove-private-as-export.ci`,
  `remove-private-as-replace-peer.ci`) are UNTOUCHED and remain red for their two unrelated
  reasons (the LOCAL_PREF/ATTR_DISCARD question, and the check-mode-peer sequential-accept
  defect). Closing them is not in this spec's scope.

## RFC Documentation

RFC 6996 S4 (the MUST), S5 (the ranges). Summary: `rfc/short/rfc6996.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: A1/A2 verified; A3 labelled ASSUMED; R1/R2 surviving and flagged

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)

```
# Pre-fix / mutation A (ctxID 0 restored) -- the filter never sees the attributes:
3.7s     2/2  FAIL  367  remove-private-as-replace-originated
3.8s     1/2  FAIL  364  remove-private-as-export-originated
fail  0/2  0.0%  3.8s  failed 2 [364, 367]

# Mutation B (Reject/raw-only restored) -- the text delta is discarded:
3.1s     2/2  FAIL  367  remove-private-as-replace-originated
3.3s     1/2  FAIL  364  remove-private-as-export-originated
fail  0/2  0.0%  3.3s  failed 2 [364, 367]

# Leaked wire at HEAD, byte-identical to the 3/18 leak measured by B2.3:
msg  recv  FFFF...FFFF:0037:02:0000001C4001010040020E02030000FBF00000FC000000FBF140030401010101180A0000
           = AS_PATH [64496 64512 64497] -- private 64512 present on an EBGP session
```

- [ ] Tests PASS (paste output)

```
3.0s     2/2  PASS  367  remove-private-as-replace-originated
3.2s     1/2  PASS  364  remove-private-as-export-originated
pass  2/2  100.0%  3.2s

# Post-fix wire:
msg  recv  FFFF...FFFF:0033:02:000000184001010040020A02020000FBF00000FBF140030401010101180A0000
           = AS_PATH [64496 64497] -- 64512 (0xFC00) removed. RFC 6996 S4 satisfied.

# Determinism: 20 consecutive runs of both tests = 40/40 green, 0 flakes.
40 test-instances over 20 runs: pass=20 fail=0
```

→ Decision: the reproducer is DETERMINISTIC (100% leak pre-fix, 100% green post-fix), not
  the 3/18 race the originating spec measured. The race was in the originating spec's
  TOPOLOGY (which of two egress paths a forwarded route happened to take), not in the
  defect. Driving the leaking path directly makes it a certainty. That is why the fix is
  proven against a deterministic test rather than a 17%-flaky one.

**Regression / no-collateral evidence.** `make ze-plugin-test` was run twice on the SAME
working tree, differing ONLY in whether my two files were at HEAD or fixed (HEAD versions
extracted with `git show`, swapped in with `cp`). Both runs: **32 failures out of 488**.
Diff of the failing test NAMES:

```
red at HEAD -> green with fix:   remove-private-as-export-originated
                                 remove-private-as-replace-originated
                                 bmp-locrib, forward-overflow-two-tier, show-l2tp-statistics
green at HEAD -> red with fix:   api-rib-clear-in, api-rib-show-in, bgp-summary-route-counts,
                                 show-l2tp-session-detail, show-l2tp-tunnel-detail
```

→ Decision: the ONLY attributable movement is my two tests. The other five each way are
  pre-existing load-flakes in a suite that is broadly red in the current working tree
  (many concurrent sessions' WIP). Evidence they are flakes, not collateral: the counts are
  identical (32 = 32); the movers churn in BOTH directions; the l2tp/bmp ones share no code
  path with the BGP export chain; and the three BGP-adjacent ones (`api-rib-clear-in`,
  `api-rib-show-in`, `bgp-summary-route-counts`) pass **3/3 in isolation with the fix
  applied**. I cannot fully exclude a load-dependent interaction, so this is stated as
  evidence, not proof.

- [ ] Boundary tests for all numeric inputs (pre-existing on `isRFC6996PrivateASN`; unchanged)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A — justified in TDD Test Plan)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval (R1 and the EBGP-prepend question were
      FLAGGED OPEN, not silently dropped — **both ruled by Thomas 2026-07-16 and homed**:
      R1 -> `plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md` (trace
      reachability first, then fix vs fail-closed); EBGP-prepend ->
      `plan/spec-fixit-private-asn-leak-deferred-ebgp-auto-filters.md` (auto-added export
      chain entries: prepend our ASN, strip LOCAL_PREF). Both recorded in
      `plan/deferrals.md`. This gate no longer waits on Thomas.)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Goal Validation (BLOCKING)

| Goal | Evidence | Status |
|------|----------|--------|
| A configured export remove-private-as policy actually removes the private ASN before an EBGP advertisement (RFC 6996 S4 MUST) | `remove-private-as-export-originated` asserts the wire byte-exact; RED pre-fix, GREEN post-fix; both mutations kill it | MET |
| `peer-as` substitution uses the destination peer AS | `remove-private-as-replace-originated` asserts 0xFDEA (65002) not 0xFDE8 (65000) | MET |
| The leak cannot recur through path drift | one shared chain body; a future outcome added to the chain is honored by both paths automatically | MET |
| The mechanism is verified, not hypothesised | every step of the Data Flow cites a producing `file:line`; the two defects were each confirmed by an independent mutation | MET |

## Implementation Summary

### What Was Implemented
The two-line-shaped, two-defect fix in `exportFilterForBody`, plus the extraction of
`runEgressPolicyChainASN4` as the single shared chain body, plus two functional tests.

### Bugs Found/Fixed
1. **Dropped text delta** (`egress_inject_filter.go:49-57` pre-fix) — the private-ASN leak.
2. **ctxID 0** (`egress_inject_filter.go:38` pre-fix) — the filter never saw the attributes.
   Found only because fixing (1) alone did NOT stop the leak; the measurement contradicted
   the story and the story lost.

### Deviations from Plan

- **Nothing planned was dropped.** The two deviations are both *additions* to the record,
  not reductions in scope: R1 and the EBGP-prepend question were flagged open during
  implementation and ruled by Thomas on 2026-07-16, after the code was complete.
- **R1 (`r.api == nil` silent fail-open) — deferred, homed, still unfixed.**
  Destination: `plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md`
  (`Status | skeleton`). Ruling: trace reachability first, then decide fix vs fail-closed.
  Correctly out of scope here (this spec's subject is the RFC 6996 private-ASN MUST), and
  fixing an untraced path blind is what `ai/rules/no-fabrication.md` forbids. Recorded in
  `plan/deferrals.md`. Surviving risk R1 stays on this spec's books.
- **EBGP-prepend-on-originate — answered with a design, deferred, homed.**
  Destination: `plan/spec-fixit-private-asn-leak-deferred-ebgp-auto-filters.md`
  (`Status | skeleton`). Thomas's ruling turns the yes/no question into a design: EBGP peers
  get auto-added export chain entries (prepend our ASN; strip LOCAL_PREF). Recorded in
  `plan/deferrals.md`.
- **Two claims in this spec's own Known Limitations were found wrong while homing the
  above** (the `update text` vs `rib/commit` prepend disagreement, and `attributes.ci` being
  an IBGP baseline rather than EBGP evidence). Corrected in place there rather than deleted,
  so the destination spec inherits the correction and not the error.
- **A live RFC 4271 S5.1.5 violation was surfaced but NOT fixed here** (LOCAL_PREF survives
  IBGP-source -> EBGP-destination forwarding). Out of scope, no scope reduction: it was
  never in the plan. It is AC-1 of the auto-filters spec.

### Mistake Log / Wrong Assumptions

- **The originating spec's hypothesis was WRONG, and is refuted here on two grounds.**
  B2.3/B2.4 suspected `refreshForwardFacts` had not re-run after filter refs resolved,
  leaving `exportFilters` empty so `forward_rs.go:107-114` failed to divert the peer.
  (a) TIMING: ExportFilters is config-parse-time (`config/peers.go:202`), long before the
  Peer exists; the only late writer (`reactor_dynamic.go:332`) already refreshes at `:335`.
  (b) BYTES: had the snapshot been empty, the peer would NOT have been skipped at
  `forward_rs.go:111` and would have gone through `:350-357 getEBGPWire`, which PREPENDS
  the local AS. The observed leak had NO prepend. An empty-exportFilters snapshot cannot
  produce the observed bytes. The hypothesis was self-consistent and false —
  `ai/rules/no-fabrication.md`, "a coherent story is a hypothesis".
- **My own first fix was wrong for the same reason.** I fixed the dropped delta, was
  confident, and the wire was byte-identical to the leak. Only instrumentation
  (`textIn=` with no attributes) exposed the second defect. The lesson is the rule's:
  do not trust a fix you have not watched work.

## Review Gate

### Run 1 (closure — independent adversarial review, 2026-07-21)

Independent subagent review of the full changeset (`egress_inject_filter.go`,
`filter_ordered.go`, `remove-private-as-{export,replace}-originated.ci`) against AC-1..AC-5.

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| NOTE | Working-tree comment-only doc-ref repoint (`spec-fixit-private-asn-leak.md` -> `plan/learned/1231-...`) | `egress_inject_filter.go:36` | Intended closure edit; no behavior change |
| NOTE | `r.api == nil` case now fails CLOSED (child commit `1fb231afb`), improving on this spec's "recorded, not fixed" prose | `egress_inject_filter.go:62-65` | Improvement, correctly homed to the nil-api child spec; not a defect |

**Verdict: CLEAN — 0 BLOCKER, 0 ISSUE, 2 NOTE.** ACs supported by producing code:
AC-1 `filter_ordered.go:250` / `filter_delta.go:645-668` (STRIP); AC-2 `egress_inject_filter.go:76`
/ `filter_delta.go:657` (peer-as = destination AS); AC-3 `runEgressPolicyChainASN4`
(`filter_ordered.go:221`) is the sole shared body; AC-4 both halves sit on the byte-exact
asserted path (mutation-killable); AC-5 forwarded path preserves source-ctx `asn4`.
`p.asn4()` (encode) and `facts.sendASN4` (parse/filter) read the identical `p.sendCtx`, so
the originated body's octet-width and the filter's parse context cannot diverge.
Artifact: `tmp/review/fixit-private-asn-leak-<session>.md` (verdict clean).

Gate satisfied: last run 0 BLOCKER, 0 ISSUE.
