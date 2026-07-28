# Spec: fixit-rs-community-strip-arity -- Route-server control communities leak when a route carries two or more

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-07-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** A route-server client receives the route-server's own control
communities (the `0:<asn>` and `<rs-asn>:<asn>` forms used to steer per-peer
forwarding) whenever the route carries **two or more** of them. With exactly one,
the strip works.

**Cause.** An arity mismatch between two halves of the modification accumulator
contract, in a path with zero test coverage.

`StripControlCommunities` (`internal/component/bgp/wireu/community.go:158`) walks
the COMMUNITY attribute and accumulates **every** matching four-byte value into a
single slice at `:189`. Both route-server call sites hand that whole slice to the
accumulator as one Remove operation:

| Call site | Producer line |
|-----------|---------------|
| `internal/component/bgp/reactor/reactor_api_forward.go:643` builds it, `:635` emits it | `mods.Op(8, filterapi.AttrModRemove, communityStripBytes)` |
| `internal/component/bgp/reactor/forward_rs.go:347` builds it, `:342` emits it | `mods.Op(8, filterapi.AttrModRemove, communityStripBytes)` |

The consumer is `removeValues`
(`internal/component/bgp/plugins/filter_community/handler.go:165`), reached
through `communityAttrModHandler` (`:19`) and `genericCommunityHandler` (`:64`,
which calls `removeValues(data, 4, op.Buf)` at `:78`). Its first statement is a
size guard at `:119-122`:

if the removal buffer is not exactly `valueSize` bytes it returns the data
unchanged, with the comment "Size mismatch: caller bug, silently preserve data".

So a strip buffer holding two communities is eight bytes, the guard trips, and
**none** of them are removed. The route is forwarded with its control communities
intact.

**Why the other producers are unaffected.** They already split. The text-delta
path splits a Remove directive into `valueSize` chunks explicitly at
`internal/component/bgp/reactor/filter_delta.go:221-224`, and the community
plugin's own egress filter emits one operation per wire value at
`internal/component/bgp/plugins/filter_community/egress.go:28-30`. The one-value
rule is therefore a real contract, it is simply unwritten, and the two
route-server sites are the only violators.

**Two separate defects, one fix.**

1. The route-server strip does not happen (a behaviour bug on a live path).
2. The guard that catches the mismatch is fail-open: it preserves the data and
   says nothing, so the violation has been invisible since it was introduced.
   `ai/rules/fail-closed-guards.md` requires a guard to fail closed or speak.

**Coverage.** There is no test at all. `StripControlCommunities` has exactly four
references in the tree (the two call sites, its definition, and its doc comment)
and none of them is a test. No `.ci` under `test/plugin/` exercises route-server
control-community stripping.

This spec is independent of `plan/spec-wire-edit-0-umbrella.md`, which would make
the arity part of a typed structure. That is weeks away; this is a live leak.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: silently preserving data on a detected caller-contract violation is the exact fail-open shape this rule forbids. The mismatch branch must either handle the input or refuse loudly.
- [ ] `docs/architecture/core-design.md` - modification accumulator and the progressive build
  → Decision: filters accumulate `Op(code, action, valueBytes)` and the apply engine dispatches per attribute code to a registered handler. The value-buffer arity is part of that contract and belongs in its documentation.
- [ ] `ai/rules/api-contracts.md` - caller obligations belong in the function comment
  → Constraint: `ModAccumulator.Op` carries a caller obligation for list-valued Remove operations and does not state it. That omission is the root cause.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1997.md` - BGP communities
  → Constraint: a COMMUNITY attribute value is a list of four-octet values, so a subset removal is well defined on four-byte boundaries. The `valueSize` parameter is that width.
- [ ] `rfc/short/rfc7947.md` - Internet Exchange BGP route server
  → Constraint: the RFC mandates per-client import and export policy on each redistribution (RFC7947-x-4) but places **no** normative requirement on stripping control communities. The stripping is ze's own designed behaviour, stated in the `StripControlCommunities` doc comment at `internal/component/bgp/wireu/community.go:138-140` ("values that should be removed before forwarding"), not an RFC obligation. Do not describe the fix as an RFC compliance fix.
- [ ] `rfc/short/rfc8092.md` - large communities
  → Constraint: the twelve-byte width shares the same generic handler, so any arity change must hold for widths 4, 8 and 12.

**Key insights:** (minimal context to resume after compaction)
- Only the two route-server sites pass a multi-value Remove buffer; every other producer already splits.
- The forwarding **decision** is correct: `ParseCommunityPolicy` (`community.go:51`) reads the policy properly and `ShouldForwardTo` (`:21`) suppresses the right destinations. Only the stripping fails, so the impact is a leak of internal control tags to clients, not mis-forwarding.
- `filter_community` is gated by `ze_bgp` (`feature-gates.txt:261`), the same gate as the reactor, so the handler is always linked when this path can run. The absence of a default code-8 handler in `attrModHandlersWithDefaults` (`filter_delta_handlers.go:468`, whose `genericAttrCodes` list at `:82-98` deliberately excludes 8, 16 and 32) is therefore not reachable and is not part of this bug.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/community.go:158` - `StripControlCommunities`: walks the attribute section, and for a COMMUNITY attribute appends every four-byte value whose high half is 0 or the route server's low sixteen ASN bits into one result slice (`:186-191`). Returns nil when nothing matches.
- [ ] `internal/component/bgp/wireu/community.go:51` - `ParseCommunityPolicy`: independent walk producing the blacklist, whitelist, blackhole and prepend-target policy. Correct today; unchanged by this fix.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:610-636` - the route-server branch of the destination loop: parses the policy once per UPDATE at `:614-616`, suppresses non-forward destinations at `:618`, then emits the single Remove operation at `:635`.
- [ ] `internal/component/bgp/reactor/forward_rs.go:320-343` - the route-server fast-path rail: the identical sequence, strip buffer built at `:325`, single Remove operation at `:342`.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:19` - `communityAttrModHandler` delegates to `genericCommunityHandler` with `valueSize` 4.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:64` - `genericCommunityHandler`: copies the source value into a fresh buffer at `:69-70`, applies Remove operations at `:76-80`, then Add at `:81-85`, then Set at `:86-91`, omits the attribute entirely when nothing remains at `:93-95`, and writes an always-extended-length header at `:110-113`.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:165` - `removeValues`: returns the input unchanged when the removal buffer length is not exactly `valueSize`; otherwise rebuilds the list with an allocating append loop at `:121-126`.
- [ ] `internal/component/bgp/plugins/filter_community/egress.go:28-30` - the plugin's own egress filter emits one Remove operation per configured wire value.
- [ ] `internal/component/bgp/reactor/filter_delta.go:221-224` - the text-delta path splits a Remove directive into `valueSize` chunks before emitting.
- [ ] `internal/component/bgp/filterapi/filterapi.go:153` - `func (a *ModAccumulator) Op(`: documents that repeated calls with the same code are allowed and reach the handler together, but states nothing about the value-buffer arity.
- [ ] `internal/component/bgp/reactor/forward_build.go:199-223` - `buildModifiedPayload` dispatch: groups operations by code and hands all operations for a code to its registered handler in one call.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:468` - `attrModHandlersWithDefaults`: fills generic set handlers for the codes listed at `:82-98`, which exclude the community codes because the plugin registers specialised ones.

**Behavior to preserve:**
- The forwarding decision: `ShouldForwardTo` (`community.go:21`) must keep suppressing exactly the destinations it suppresses today. This fix touches stripping only.
- Single-value Remove operations from the text-delta path (`filter_delta.go:221-224`) and from the plugin egress filter (`egress.go:28-30`) must behave exactly as today.
- The Remove, then Add, then Set ordering inside `genericCommunityHandler` (`:76-91`), and the "omit the attribute entirely when nothing remains" behaviour at `:93-95`.
- The same handler serves widths 4, 8 and 12; all three must keep working.
- Every existing expectation in `test/plugin/community-*.ci` and `test/plugin/bgp-rs-*.ci`.

**Behavior to change:**
- A Remove operation whose buffer holds N values removes all N, instead of silently removing none when N is greater than one.
- A Remove buffer whose length is not a whole multiple of the value width is refused loudly instead of being silently ignored.

## Data Flow (MANDATORY)

### Entry Point
- A route-server client peer sends an UPDATE whose COMMUNITY attribute carries two or more control values, for example `0:65001` and `0:65002`. The bytes arrive as a normal received UPDATE and are cached as a `ReceivedUpdate`.

### Transformation Path
1. The destination loop reaches a route-server client destination (`reactor_api_forward.go:611`, or `forward_rs.go:320` on the fast-path rail).
2. Once per UPDATE, `ParseCommunityPolicy` and `StripControlCommunities` each walk the payload (`reactor_api_forward.go:614-616`). The strip buffer now holds eight bytes, two communities.
3. `ShouldForwardTo` decides the destination is eligible (`:618`). This is correct today.
4. One Remove operation carrying the eight-byte buffer is appended to the destination's accumulator (`:635`).
5. `buildModifiedPayload` groups operations by code and calls the registered code-8 handler with them (`forward_build.go:199-211`).
6. `genericCommunityHandler` copies the source list and calls `removeValues(data, 4, eightByteBuffer)` (`handler.go:78`).
7. `removeValues` sees a length of eight against a value size of four, and returns the list unchanged (`handler.go:119-122`).
8. The unchanged list is written back into the output attribute (`handler.go:110-113`), and the route reaches the client carrying both control communities.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to registered attribute handler | `filterapi.AttrOp{Code, Action, Buf}` through `ModAccumulator`, dispatched by attribute code | No |
| Reactor to peer TCP | modified UPDATE body written by the forward pool | No |
| wireu to reactor | `StripControlCommunities` returns a concatenated wire-value buffer | No |

### Integration Points
- `filterapi.ModAccumulator.Op` (`filterapi.go:132`): the contract that gains the documented arity rule.
- `genericCommunityHandler` (`handler.go:64`): the single consumer for all three community widths.
- The two route-server rails, `forwardUpdateCore` and `reactorForwardRS`, which must stay behaviourally identical to each other.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Chosen Fix

Widen the contract at the consumer rather than splitting at the two producers.

| Option | Effect | Verdict |
|--------|--------|---------|
| A: split into one operation per value at the two route-server sites | Fixes the leak. Leaves the fail-open guard in place, so the next producer to violate the rule fails silently again. Two call sites to keep in step. | Rejected as the whole fix; the guard is the durable half of the defect. |
| B: accept a Remove buffer of one or more values, refuse a non-multiple length loudly | Fixes the leak at the single consumer, makes the rule "a whole number of values", and keeps every existing single-value producer working unchanged since one value is a multiple. | **Chosen.** |
| C: reject multi-value buffers at `Op` time | Turns a silent leak into a loud one, but still requires the producers to change and adds a per-operation check to a hot path. | Rejected. |

Option B, in three parts:

1. `removeValues` treats its removal buffer as a set of whole values: any value in
   the input list that matches any value in the buffer is dropped. A buffer whose
   length is not a whole multiple of the value width is a caller-contract
   violation and is reported (a warning naming the code, the expected width and
   the actual length), not silently ignored.
2. `ModAccumulator.Op` documents the obligation: for list-valued Remove
   operations the buffer must be a whole number of wire values of the attribute's
   width.
3. The two route-server sites keep passing the concatenated buffer, which is now
   the documented, working form.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The two route-server sites are the only producers of a multi-value Remove buffer. | Full enumeration of `AttrModRemove` producers: `filter_delta.go:221-224` splits, `filter_community/egress.go:29` is per value, `reactor_api_forward.go:635` and `forward_rs.go:342` do not split. | Another producer means the same fix still applies; only the impact assessment widens. | `grep -rn "AttrModRemove" --include=*.go internal/` re-run at implementation, each producer inspected. | **VALIDATED 2026-07-28.** Re-run: the only other hit is `policy_dryrun.go:251`, which is a verb-name switch for dry-run output, not a producer or consumer. |
| A-2 | Widening `removeValues` cannot change behaviour for a single-value buffer. | One value is a whole multiple of the width, and the matching predicate is unchanged for that case. | Existing community `.ci` files fail, which is the detection. | `TestRemoveValuesSingleUnchanged` plus the existing `test/plugin/community-*.ci` suite. | **VALIDATED.** `TestRemoveValuesSingleUnchanged` passes; the full `bgp plugin` suite shows no community test newly failing. |
| A-3 | The forwarding decision is unaffected: only stripping is broken. | `ParseCommunityPolicy` (`community.go:51`) and `ShouldForwardTo` (`:21`) are independent of `StripControlCommunities` and of the handler. | A wider bug, and the spec scope grows. | `TestShouldForwardToUnaffected` and a `.ci` asserting the suppression set is unchanged. | **VALIDATED.** `TestShouldForwardToUnaffectedByStrip` (`wireu/community_test.go`); both `.ci` files carry a `65000:65002` whitelist tag and the route still reaches 65002. |
| A-4 | Widths 8 and 12 need no separate treatment. | All three share `genericCommunityHandler` with only `valueSize` differing (`handler.go:19-31`). | Per-width handling, a larger change. | Table-driven test over widths 4, 8 and 12. | **VALIDATED.** `TestRemoveValuesAllWidths` covers 4, 8 and 12, each with a multi-value removal and a non-multiple refusal. |
| A-5 | No `.ci` currently covers route-server control-community stripping, so no existing functional expectation encodes the broken behaviour. | `grep -rn "StripControlCommunities"` returns four hits, none in a test; no `test/plugin/bgp-rs-*.ci` mentions control communities. | An existing `.ci` pins the leak and must be corrected as part of this fix. | Re-run the grep and read every `test/plugin/bgp-rs-*.ci`. | **VALIDATED.** Four hits, none a test. No existing `.ci` needed correcting; the two new files are the first coverage. |
| A-6 | `session/rs-client true` is required for the strip path to run at all. | Not in the original spec; discovered while writing the functional tests. Both the RFC 7947 policy block (`reactor_api_forward.go:611`) and the strip emission (`:642`) are gated on `facts.rsClient`, which comes from `config.go:266`. | A `.ci` without the leaf exercises nothing and passes vacuously against a broken build. | Both new `.ci` files set it on both peers, with a comment saying why. | **VALIDATED 2026-07-28.** An early draft omitted it and forwarded all five communities intact even against a FIXED binary -- indistinguishable from the bug it was meant to catch. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Operators have built policy that depends on seeing the leaked control communities on the client side. | Complaint after upgrade. | The leak contradicts the stated intent in `community.go:138-140`. Record the behaviour change in the learned summary and the guide page added by this spec. |
| R-2 | The loud refusal fires in production for a producer the enumeration missed. | The new warning appears in soak logs. | The warning names the code, the expected width and the actual length, so the offending producer is identifiable from one log line. |
| R-3 | The route-server fast-path rail and the general forward rail drift, so the fix lands on one only. | The `.ci` runs one rail and passes while the other leaks. | The functional test must exercise both rails; `test/plugin/bgp-rs-reactor-fastpath.ci` and `bgp-rs-reactor-fastpath-fallback.ci` show how each is selected. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route-server clients either keep receiving internal control communities (no change from today) or, if the matching predicate is wrong, lose communities they should have kept. The second is worse: a client's own downstream policy could stop firing. |
| How is it reverted? | Single commit revert. No wire-format or configuration change, no persisted state. |
| Who else touches this path? | `plan/spec-wire-edit-0-umbrella.md` replaces this contract with a typed structure; `plan/spec-hotpath-alloc-round-4.md` removes the allocations in the same handler. Both must be rebased on this fix, not the other way round. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Route-server client sends a route tagged with two `0:<asn>` control communities | → | `StripControlCommunities` builds an eight-byte buffer, `removeValues` drops both | `test/plugin/bgp-rs-community-strip-multi.ci` |
| Same route on the route-server fast-path rail | → | `forward_rs.go:342` operation reaches the same handler | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` |
| Route tagged with exactly one control community | → | unchanged single-value path | `test/plugin/bgp-rs-community-strip-multi.ci` (single-value case in the same scenario) |
| Configured `community { egress strip NAME }` on an ordinary peer | → | per-value operations from `egress.go:29`, behaviour unchanged | existing `test/plugin/community-strip.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route-server client route carrying two control communities, forwarded to another client | Neither control community appears in the UPDATE the receiving client sees |
| AC-2 | Same, with five control communities interleaved with three ordinary ones | All five control communities are removed; all three ordinary communities are preserved, in their original order |
| AC-3 | Same, with exactly one control community | Behaviour is identical to today: the single community is removed |
| AC-4 | A route whose every community is a control community | The COMMUNITY attribute is omitted entirely from the forwarded UPDATE, matching the existing empty-list behaviour at `handler.go:93-95` |
| AC-5 | A Remove operation whose buffer length is not a whole multiple of the value width | The operation is refused and a warning is logged naming the attribute code, the expected width and the actual buffer length. The remaining operations for that attribute still apply |
| AC-6 | The same scenario driven through the route-server fast-path rail | Identical wire output to the general forward rail |
| AC-7 | Extended communities (width 8) and large communities (width 12) with a multi-value Remove buffer | All listed values are removed, proving the fix is width-independent |
| AC-8 | The per-destination forwarding decision for a route carrying control communities | Unchanged: the same destinations are suppressed and the same destinations receive the route as before the fix |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs ze as an IXP route server; a client tags a route with `0:65001 0:65002` to hide it from two peers | wire, policy parse, per-destination suppression, control-community strip, wire to the remaining clients | `test/plugin/bgp-rs-community-strip-multi.ci` |
| 2 | Same deployment with the route-server fast path active | wire, fast-path rail, same strip, wire | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` |
| 3 | Tags a route with control communities plus its own ordinary communities | wire, strip removes only the control values | `test/plugin/bgp-rs-community-strip-multi.ci` (mixed case) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoveValuesMultiValueBuffer` | `internal/component/bgp/plugins/filter_community/handler_test.go` | a buffer of N whole values removes all N; the current code removes none for N greater than one | |
| `TestRemoveValuesSingleUnchanged` | `internal/component/bgp/plugins/filter_community/handler_test.go` | A-2: the single-value case is byte-identical to today | |
| `TestRemoveValuesNonMultipleRefusedLoudly` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-5: a non-multiple length is refused and reported, not silently ignored | |
| `TestRemoveValuesAllWidths` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-7: table-driven over widths 4, 8 and 12 | |
| `TestGenericCommunityHandlerOmitsEmptyAttribute` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-4: an emptied list omits the attribute, preserving `handler.go:93-95` | |
| `TestStripControlCommunitiesMultiValue` | `internal/component/bgp/wireu/community_test.go` | the producer emits every matching value concatenated; pins the buffer shape the consumer must accept | |
| `TestStripControlCommunitiesPreservesOrdinary` | `internal/component/bgp/wireu/community_test.go` | ordinary communities are never selected for removal | |
| `TestShouldForwardToUnaffected` | `internal/component/bgp/wireu/community_test.go` | A-3: the forwarding decision is independent of the strip | |
| `TestRSStripEndToEndBothRails` | `internal/component/bgp/reactor/forward_rs_test.go` | AC-6: both rails produce identical wire for the same input | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Remove buffer length, width 4 | 0, 4, 8, ... | any multiple of 4 | N/A | any non-multiple, for example 5, refused loudly |
| Remove buffer length, width 8 | 0, 8, 16, ... | any multiple of 8 | N/A | any non-multiple, for example 12, refused loudly |
| Remove buffer length, width 12 | 0, 12, 24, ... | any multiple of 12 | N/A | any non-multiple, for example 8, refused loudly |
| Control communities per route | 0-n | bounded by the attribute's 65535-byte value ceiling | N/A | N/A |
| Community count after removal | 0-n | 0 (attribute omitted) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-community-strip-multi` | `test/plugin/bgp-rs-community-strip-multi.ci` | route-server client receives a route with zero, one, two and five control communities; none leak, ordinary communities survive | |
| `bgp-rs-community-strip-multi-fastpath` | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` | the same scenario driven through the route-server fast-path rail | |
| `community-strip` (existing) | `test/plugin/community-strip.ci` | the configured per-peer egress strip is unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The control-community convention is ze's own forwarding behaviour, not a wire-format change, and no peer daemon implements the same convention to interoperate against. The `.ci` files above observe the emitted wire directly, which is the same evidence an interop run would give. | N-A |

## Files to Modify
- `internal/component/bgp/plugins/filter_community/handler.go` - `removeValues` accepts a whole number of values and refuses a non-multiple loudly
- `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator.Op` documents the Remove-buffer arity obligation
- `internal/component/bgp/wireu/community.go` - `StripControlCommunities` doc comment states that it returns a concatenated multi-value buffer
- `internal/component/bgp/reactor/reactor_api_forward.go` - comment at the emission site naming the arity contract it relies on
- `internal/component/bgp/reactor/forward_rs.go` - the same comment on the fast-path rail
- `docs/guide/bgp-policy.md` - document the route-server control-community convention, which has no user-facing documentation today

## Files to Create
- `internal/component/bgp/plugins/filter_community/handler_test.go` - unit tests above, if the file does not already exist
- `internal/component/bgp/wireu/community_test.go` - producer-side unit tests, if the file does not already exist
- `test/plugin/bgp-rs-community-strip-multi.ci` - the leak scenario on the general rail
- `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` - the leak scenario on the fast-path rail

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No configuration surface changes; the strip is unconditional route-server behaviour |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; the two new `.ci` files cover the behaviour |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_attr_mod_op_refused_total`, labelled by attribute code, incremented on the non-multiple refusal so AC-5 is observable in production, not only in tests |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The feature exists; it was broken and undocumented |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if the arity obligation is visible to plugin authors writing filters |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md`: the route-server control-community convention (`0:<asn>` and `<rs-asn>:<asn>`) has no documentation anywhere under `docs/`, which is part of why the bug survived |
| 7 | Wire format changed? | No | The emitted wire changes, but no format changes |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugin-design.md`: state the Remove-buffer arity obligation alongside the accumulator contract |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7947 places no requirement on control-community stripping; see the Required Reading constraint |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: record the accumulator Remove-buffer arity in the modification-accumulator section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for `bgp_attr_mod_op_refused_total` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `community.go`, `handler.go` and `filterapi.go` and correct any stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | There are no examples for this area, which is the gap row 6 fills |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the leak from the user-visible entry point
   - Tests: `test/plugin/bgp-rs-community-strip-multi.ci` and `test/plugin/bgp-rs-community-strip-multi-fastpath.ci`, written to assert the correct behaviour
   - Files: the two new `.ci` files
   - Verify: both fail on the current tree, with the two control communities visible in the received UPDATE. Paste the failure output. A `.ci` that passes before the fix is not testing the bug
2. **Phase: Producer and consumer unit coverage** -- pin the contract at both ends
   - Tests: `TestStripControlCommunitiesMultiValue`, `TestStripControlCommunitiesPreservesOrdinary`, `TestShouldForwardToUnaffected`, `TestRemoveValuesMultiValueBuffer`, `TestRemoveValuesSingleUnchanged`, `TestRemoveValuesAllWidths`, `TestGenericCommunityHandlerOmitsEmptyAttribute`
   - Files: `wireu/community_test.go`, `plugins/filter_community/handler_test.go`
   - Verify: the multi-value tests fail, the single-value tests pass. This separates the regression guard from the fix
3. **Phase: Fix the consumer** -- widen `removeValues`, refuse non-multiples loudly
   - Tests: all of phase 2, plus `TestRemoveValuesNonMultipleRefusedLoudly`
   - Files: `plugins/filter_community/handler.go`
   - Verify: all unit tests pass; both `.ci` files pass; `test/plugin/community-*.ci` unchanged
4. **Phase: State the contract** -- document the obligation everywhere it is relied on
   - Tests: `TestRSStripEndToEndBothRails`
   - Files: `filterapi/filterapi.go`, `wireu/community.go`, `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`, `docs/guide/bgp-policy.md`, `docs/architecture/core-design.md`, `ai/rules/plugin-design.md`
   - Verify: both rails produce identical wire; the counter is registered and documented; `make ze-verify` passes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and both route-server rails are covered, not just the one the default configuration selects |
| Feature completeness | The `.ci` files fail before the fix and pass after; a mutation reinstating the size guard turns them red |
| Correctness | Only control communities are removed. An ordinary community whose high half coincidentally equals the route server's low sixteen ASN bits is a real ambiguity in the existing selection rule (`community.go:186-190`); confirm the fix does not widen it, and record it as a known limitation rather than silently changing it |
| Naming | The refusal warning names the attribute code, the expected width and the actual length, per `ai/rules/error-messages.md` |
| Data flow | The forwarding decision path is untouched; only the strip path changes |
| Rule: `ai/rules/fail-closed-guards.md` | No branch preserves data on a detected contract violation without speaking |
| Rule: `ai/rules/api-contracts.md` | The arity obligation is stated on `ModAccumulator.Op`, where the caller reads it, not only in the handler that enforces it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The leak is fixed on the general rail | `make ze-functional-test TEST=bgp-rs-community-strip-multi` |
| The leak is fixed on the fast-path rail | `make ze-functional-test TEST=bgp-rs-community-strip-multi-fastpath` |
| The tests genuinely catch the bug | reinstate the exact-length guard at `handler.go:119-122` and confirm both `.ci` files turn red |
| The single-value path is unchanged | `make ze-functional-test TEST=community-strip` |
| The refusal is observable | `grep -rn "bgp_attr_mod_op_refused_total" internal/` returns the registration and the increment site |
| The convention is documented | `grep -n "control communit" docs/guide/bgp-policy.md` |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The removal buffer is engine-produced, not peer-controlled, but the community list it is applied to is peer-controlled. The rebuilt list must stay within the 65535-byte attribute value ceiling and must never read past the source value bounds |
| Information disclosure | This is the defect itself: internal route-server control tags reaching client peers. Verify the fix removes every matched value, not merely the first |
| Resource exhaustion | The matching loop is O(list length times buffer length). A peer can make the list long and the route server's own policy makes the buffer long. Confirm the product stays bounded by the attribute ceiling and consider a set-based match if the measured cost is material |
| Fail-open risk | The refusal path must drop the operation and speak, never silently pass the unmodified attribute through |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| The new `.ci` passes before the fix | The test does not reproduce the bug. Back to phase 1; do not proceed |
| Test fails for the wrong reason | Fix the test assertion or setup |
| An existing community `.ci` breaks | A-2 is broken. Stop, record the Mistake Log row, and re-examine the single-value path |
| Only one rail is fixed | R-3. Back to phase 4 and cover both |
| Lint failure | Fix inline; if architectural → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Implementation Summary (2026-07-28, session dd843d81)

### What was implemented

| AC | Status | Evidence |
|----|--------|----------|
| AC-1, AC-2 | done | `test/plugin/bgp-rs-community-strip-multi.ci`: three control communities interleaved with two ordinary ones; the forwarded UPDATE carries `D0 08 0008` + the two ordinary values, in order |
| AC-3 | done | `TestRemoveValuesSingleUnchanged`, `TestStripControlCommunitiesSingleValue` |
| AC-4 | done | `TestRemoveValuesMultiRemovingEverythingOmitsAttribute` |
| AC-5 | done | `removeValues` returns `(data, ok)`; the caller logs and `continue`s. `TestRemoveValuesNonMultipleRefusedLoudly` (the signal) and `TestGenericCommunityHandlerWarnsOnNonMultiple` (the warning, and that sibling ops still apply) |
| AC-6 | done | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci`, identical but for `rs-fast-path enable` on the source |
| AC-7 | done | `TestRemoveValuesAllWidths` over widths 4, 8, 12 |
| AC-8 | done | `TestShouldForwardToUnaffectedByStrip`; both `.ci` files rely on the whitelist tag still routing to 65002 |

### Deviation from the Chosen Fix

The spec put the log line inside `removeValues`. It is in the CALLER instead.
`logger` here is `slogutil.LazyLogger` memoised behind a `sync.Once`
(`filter_community.go:26`), so a helper that logged directly could only be tested
through logging configuration. Returning `(data, ok)` makes the refusal a value
the unit test asserts on directly, and `genericCommunityHandler` emits the warning
and `continue`s -- which is also what AC-5's "the remaining operations for that
attribute still apply" requires. Same behaviour, testable without log capture.

### Not done

The "add a refusal counter" row in Key Design Decisions is NOT implemented. It is
not an AC, not in Files to Modify, and this package has no metrics surface at all
(no prometheus import anywhere under `filter_community/`), so it would mean
introducing one for a path that should never fire. The warning names the attribute
code, expected width and actual length, which satisfies R-2's stated mitigation.
Left for an owner decision.

### Verification

- Unit: `internal/component/bgp/{wireu,filterapi,plugins/filter_community}` green.
  Whole `./internal/component/bgp/...` tree green (81 packages) **with feature
  tags** -- a bare `go test` fabricates reds in `bgp/cli` and `bgp/config`
  ("unsupported family"), which is the tags trap, not a regression.
- Functional: both new `.ci` files pass, and pass inside the full
  `bgp plugin --all` run (498/514).
- Mutation, unit: original single-value guard -> multi-value tests red; silent
  refusal -> the warning test red; match only the first value in the set ->
  multi-value tests red.
- Mutation, functional: restoring the original `len(toRemove) != valueSize` guard
  turns BOTH `.ci` files red, each reporting
  `COMMUNITIES: 0:64998 65001:100 0:64999 65000:65002 65001:200` -- the leak
  itself, on both rails.
- The 15 suite failures are pre-existing and unrelated: the six privilege-blocked
  l2tp/kernel-log/teardown tests, five external-warns tests, and four
  load-sensitive ones (`concurrent-config-commit`, `flowspec-fw-withdraw`,
  `bfd-echo-handshake`, `role-otc-unicast-scope`) which each pass in isolation.

### Trap worth recording

Every rebuild during this work went to `bin/ze`, but the runner resolves the DUT
from `tmp/s/<session-id>/bin` FIRST (`FindPrebuiltDir`, `internal/test/sessionpath/sessionpath.go:107-132`).
A stale session-scoped binary was therefore under test for several iterations,
which read exactly like "the fix does not work on this path" and sent this
investigation looking for a third forwarding rail that does not exist
(`ForwardUpdatesDirect` delegates to `forwardUpdateCore` at
`reactor_api_forward_batch.go:148`). Running WITHOUT `ZE_TEST_NO_BUILD=1` lets the
runner build the DUT itself and avoids this entirely.

## Design Insights

- The defect is not in either half taken alone. `StripControlCommunities` legitimately returns every match, and `removeValues` legitimately defends against a buffer it cannot interpret. The defect is that the arity rule joining them was never written down, so two of four producers violated it and the guard chose silence over noise.
- The guard's own comment, "Size mismatch: caller bug, silently preserve data" (`handler.go:120`), names the caller bug correctly and then declines to report it. A guard that has already diagnosed the fault is the cheapest possible place to speak.
- Zero coverage is the enabling condition. `StripControlCommunities` has four references in the tree and not one is a test, and no `.ci` exercises route-server control-community stripping. A feature with no test and no documentation has no way to notice it stopped working.
- The forwarding decision and the strip are computed by two independent walks of the same bytes (`community.go:51` and `:141`), called back to back at `reactor_api_forward.go:614-616`. That the decision half works and the strip half does not is a direct consequence of their independence.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Widen the consumer to accept a whole number of values | split into per-value operations at the two producers | Splitting fixes the leak and leaves the fail-open guard, so the next violator fails silently too. Widening fixes both defects at one site and keeps every existing single-value producer working. |
| Refuse a non-multiple length loudly | keep preserving the data silently | `ai/rules/fail-closed-guards.md`: a guard must fail closed or say something. The guard has already detected the fault; reporting it costs one log line on a path that is already an error. |
| Document the obligation on `ModAccumulator.Op` | document it only on the handler | `ai/rules/api-contracts.md` puts caller obligations where the caller reads them. The handler is not what a filter author looks at when calling `Op`. |
| Fix now, independently of `spec-wire-edit-0-umbrella.md` | wait for the typed slot structure to make the arity a compile error | The umbrella is weeks of work; this is a live leak on the route-server path. The umbrella rebases onto this fix. |
| Add a refusal counter | log only | A log line in a soak run is easy to miss. A counter makes R-2 measurable before the next producer appears. |

## Known Limitations

- The selection rule in `StripControlCommunities` (`community.go:186-190`) matches on the community's high half alone, so an ordinary community whose high half happens to equal 0 or the route server's low sixteen ASN bits is indistinguishable from a control community and is stripped. This is pre-existing and unchanged by this fix; it is recorded here because the fix makes the stripping actually happen, which makes the ambiguity reachable for the first time in the multi-value case.
- Standard communities can only carry a sixteen-bit ASN in the high half. A route server with a four-octet ASN cannot express the `<rs-asn>:<asn>` form in a standard community at all; `parseCommunityAttr` documents this at `community.go:110-112` and directs operators to large communities. Large-community control values are parsed by `parseLargeCommunityAttr` (`:198`) but are **not** stripped: `StripControlCommunities` only inspects code 8. That gap is out of scope here and is a separate defect.
- The two independent attribute walks are not merged. That is `plan/spec-wire-edit-0-umbrella.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Where it applies here |
|-----|---------|-------------|----------------------|
| 1997 | - | a COMMUNITY attribute value is a sequence of four-octet values | the `valueSize` boundary in `removeValues`, which is why a removal must land on whole values |
| 8092 | - | a LARGE_COMMUNITY value is twelve octets | the width-12 case of the same handler |
| 4360 | - | an extended community value is eight octets | the width-8 case of the same handler |
| 7947 | 2.2.2.1 | a route server SHOULD NOT modify AS_PATH | unchanged by this fix; noted so the fix is not mistaken for an RS AS_PATH change |

Note for the implementer: RFC 7947 does **not** require control-community
stripping. Do not add an RFC citation above the strip code. The behaviour is
ze's own, stated at `internal/component/bgp/wireu/community.go:138-140`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-fixit-rs-community-strip-arity.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-rs-community-strip-arity.md` only (commit A preserves the spec in history)
