# Spec: fixit-rs-community-strip-arity -- Route-server control communities leak when a route carries two or more

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-rs-community-strip-arity.md` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** A route-server client receives the route-server's own control
communities (the `0:<asn>` and `<rs-asn>:<asn>` forms used to steer per-peer
forwarding) whenever the route carries **two or more** of them. With exactly one,
the strip works.

**Cause.** An arity mismatch between two halves of the modification accumulator
contract, in a path with zero test coverage.

`StripControlCommunities` (`internal/component/bgp/wireu/community.go`) walks
the COMMUNITY attribute and accumulates **every** matching four-byte value into a
single slice at `:189`. Both route-server call sites hand that whole slice to the
accumulator as one Remove operation:

| Call site | Producer line |
|-----------|---------------|
| `internal/component/bgp/reactor/reactor_api_forward.go` builds it, `:635` emits it | `mods.Op(8, filterapi.AttrModRemove, communityStripBytes)` |
| `internal/component/bgp/reactor/forward_rs.go` builds it, `:342` emits it | `mods.Op(8, filterapi.AttrModRemove, communityStripBytes)` |

The consumer is `removeValues`
(`internal/component/bgp/plugins/filter_community/handler.go`), reached
through `communityAttrModHandler` and `genericCommunityHandler` (`:64`,
which calls `removeValues(data, 4, op.Buf)` at `:78`). Its first statement is a
size guard at `:119-122`:

if the removal buffer is not exactly `valueSize` bytes it returns the data
unchanged, with the comment "Size mismatch: caller bug, silently preserve data".

So a strip buffer holding two communities is eight bytes, the guard trips, and
**none** of them are removed. The route is forwarded with its control communities
intact.

**Why the other producers are unaffected.** They already split. The text-delta
path splits a Remove directive into `valueSize` chunks explicitly at
`internal/component/bgp/reactor/filter_delta.go`, and the community
plugin's own egress filter emits one operation per wire value at
`internal/component/bgp/plugins/filter_community/egress.go`. The one-value
rule is therefore a real contract, it is simply unwritten, and the two
route-server sites are the only violators.

**Two separate defects, one fix.**

1. The route-server strip does not happen (a behaviour bug on a live path).
2. The guard that catches the mismatch is fail-open: it preserves the data and
   says nothing, so the violation has been invisible since it was introduced.
   `ai/rules/evidence.md` requires a guard to fail closed or speak.

**Coverage.** There is no test at all. `StripControlCommunities` has exactly four
references in the tree (the two call sites, its definition, and its doc comment)
and none of them is a test. No `.ci` under `test/plugin/` exercises route-server
control-community stripping.

This spec is independent of the wire-edit-0 umbrella, which would make
the arity part of a typed structure. That is weeks away; this is a live leak.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/evidence.md` - a guard must fail closed or say something
  → Constraint: silently preserving data on a detected caller-contract violation is the exact fail-open shape this rule forbids. The mismatch branch must either handle the input or refuse loudly.
- [ ] `docs/architecture/core-design.md` - modification accumulator and the progressive build
  → Decision: filters accumulate `Op(code, action, valueBytes)` and the apply engine dispatches per attribute code to a registered handler. The value-buffer arity is part of that contract and belongs in its documentation.
- [ ] `ai/rules/go-standards.md` - caller obligations belong in the function comment
  → Constraint: `ModAccumulator.Op` carries a caller obligation for list-valued Remove operations and does not state it. That omission is the root cause.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1997.md` - BGP communities
  → Constraint: a COMMUNITY attribute value is a list of four-octet values, so a subset removal is well defined on four-byte boundaries. The `valueSize` parameter is that width.
- [ ] `rfc/short/rfc7947.md` - Internet Exchange BGP route server
  → Constraint: the RFC mandates per-client import and export policy on each redistribution (RFC7947-x-4) but places **no** normative requirement on stripping control communities. The stripping is ze's own designed behaviour, stated in the `StripControlCommunities` doc comment at `internal/component/bgp/wireu/community.go` ("values that should be removed before forwarding"), not an RFC obligation. Do not describe the fix as an RFC compliance fix.
- [ ] `rfc/short/rfc8092.md` - large communities
  → Constraint: the twelve-byte width shares the same generic handler, so any arity change must hold for widths 4, 8 and 12.

**Key insights:** (minimal context to resume after compaction)
- Only the two route-server sites pass a multi-value Remove buffer; every other producer already splits.
- The forwarding **decision** is correct: `ParseCommunityPolicy` (`community.go`) reads the policy properly and `ShouldForwardTo` suppresses the right destinations. Only the stripping fails, so the impact is a leak of internal control tags to clients, not mis-forwarding.
- `filter_community` is gated by `ze_bgp` (`feature-gates.txt`), the same gate as the reactor, so the handler is always linked when this path can run. The absence of a default code-8 handler in `attrModHandlersWithDefaults` (`filter_delta_handlers.go`, whose `genericAttrCodes` list at `:82-98` deliberately excludes 8, 16 and 32) is therefore not reachable and is not part of this bug.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)

> Line numbers in this section and in Data Flow describe the tree as it was when
> the spec was written. Citations that still name live code were re-pointed on
> 2026-07-28 after commit `4730deb84` moved them. Citations that
> name code the fix DELETED (the `len(toRemove) != valueSize` size guard and its
> "silently preserve data" comment, cited below as `handler.go` and
> `handler.go`) are kept at their pre-fix positions on purpose: they are the
> record of what the defect was, and there is nothing left to point them at.
- [ ] `internal/component/bgp/wireu/community.go` - `StripControlCommunities`: walks the attribute section, and for a COMMUNITY attribute appends every four-byte value whose high half is 0 or the route server's low sixteen ASN bits into one result slice. Returns nil when nothing matches.
- [ ] `internal/component/bgp/wireu/community.go` - `ParseCommunityPolicy`: independent walk producing the blacklist, whitelist, blackhole and prepend-target policy. Correct today; unchanged by this fix.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the route-server branch of the destination loop: parses the policy once per UPDATE at `:614-616`, suppresses non-forward destinations at `:618`, then emits the single Remove operation at `:635`.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - the route-server fast-path rail: the identical sequence, strip buffer built at `:325`, single Remove operation at `:342`.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `communityAttrModHandler` delegates to `genericCommunityHandler` with `valueSize` 4.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler`: copies the source value into a fresh buffer at `:69-70`, applies Remove operations at `:76-80`, then Add at `:81-85`, then Set at `:86-91`, omits the attribute entirely when nothing remains at `:93-95`, and writes an always-extended-length header at `:110-113`.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `removeValues`: returns the input unchanged when the removal buffer length is not exactly `valueSize`; otherwise rebuilds the list with an allocating append loop at `:172`.
- [ ] `internal/component/bgp/plugins/filter_community/egress.go` - the plugin's own egress filter emits one Remove operation per configured wire value.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - the text-delta path splits a Remove directive into `valueSize` chunks before emitting.
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `func (a *ModAccumulator) Op(`: documents that repeated calls with the same code are allowed and reach the handler together, but states nothing about the value-buffer arity.
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` dispatch: groups operations by code and hands all operations for a code to its registered handler in one call.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - `attrModHandlersWithDefaults`: fills generic set handlers for the codes listed at `:82-98`, which exclude the community codes because the plugin registers specialised ones.

**Behavior to preserve:**
- The forwarding decision: `ShouldForwardTo` (`community.go`) must keep suppressing exactly the destinations it suppresses today. This fix touches stripping only.
- Single-value Remove operations from the text-delta path (`filter_delta.go`) and from the plugin egress filter (`egress.go`) must behave exactly as today.
- The Remove, then Add, then Set ordering inside `genericCommunityHandler`, and the "omit the attribute entirely when nothing remains" behaviour at `:93-95`.
- The same handler serves widths 4, 8 and 12; all three must keep working.
- Every existing expectation in `test/plugin/community-*.ci` and `test/plugin/bgp-rs-*.ci`.

**Behavior to change:**
- A Remove operation whose buffer holds N values removes all N, instead of silently removing none when N is greater than one.
- A Remove buffer whose length is not a whole multiple of the value width is refused loudly instead of being silently ignored.

## Data Flow (MANDATORY)

### Entry Point
- A route-server client peer sends an UPDATE whose COMMUNITY attribute carries two or more control values, for example `0:65001` and `0:65002`. The bytes arrive as a normal received UPDATE and are cached as a `ReceivedUpdate`.

### Transformation Path
1. The destination loop reaches a route-server client destination (`reactor_api_forward.go`, or `forward_rs.go` on the fast-path rail).
2. Once per UPDATE, `ParseCommunityPolicy` and `StripControlCommunities` each walk the payload (`reactor_api_forward.go`). The strip buffer now holds eight bytes, two communities.
3. `ShouldForwardTo` decides the destination is eligible. This is correct today.
4. One Remove operation carrying the eight-byte buffer is appended to the destination's accumulator.
5. `buildModifiedPayload` groups operations by code and calls the registered code-8 handler with them (`forward_build.go`).
6. `genericCommunityHandler` copies the source list and calls `removeValues(data, 4, eightByteBuffer)` (`handler.go`).
7. `removeValues` sees a length of eight against a value size of four, and returns the list unchanged (`handler.go`).
8. The unchanged list is written back into the output attribute (`handler.go`), and the route reaches the client carrying both control communities.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to registered attribute handler | `filterapi.AttrOp{Code, Action, Buf}` through `ModAccumulator`, dispatched by attribute code | No |
| Reactor to peer TCP | modified UPDATE body written by the forward pool | No |
| wireu to reactor | `StripControlCommunities` returns a concatenated wire-value buffer | No |

### Integration Points
- `filterapi.ModAccumulator.Op` (`filterapi.go`): the contract that gains the documented arity rule.
- `genericCommunityHandler` (`handler.go`): the single consumer for all three community widths.
- The two route-server rails, `forwardUpdateCore` and `reactorForwardRS`, which must stay behaviourally identical to each other.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | No | |

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
| A-1 | The two route-server sites are the only producers of a multi-value Remove buffer. | Full enumeration of `AttrModRemove` producers: `filter_delta.go` splits, `filter_community/egress.go` is per value, `reactor_api_forward.go` and `forward_rs.go` do not split. | Another producer means the same fix still applies; only the impact assessment widens. | `grep -rn "AttrModRemove" --include=*.go internal/` re-run at implementation, each producer inspected. | **VALIDATED 2026-07-28.** Re-run: the only other hit is `policy_dryrun.go`, which is a verb-name switch for dry-run output, not a producer or consumer. |
| A-2 | Widening `removeValues` cannot change behaviour for a single-value buffer. | One value is a whole multiple of the width, and the matching predicate is unchanged for that case. | Existing community `.ci` files fail, which is the detection. | `TestRemoveValuesSingleUnchanged` plus the existing `test/plugin/community-*.ci` suite. | **VALIDATED.** `TestRemoveValuesSingleUnchanged` passes; the full `bgp plugin` suite shows no community test newly failing. |
| A-3 | The forwarding decision is unaffected: only stripping is broken. | `ParseCommunityPolicy` (`community.go`) and `ShouldForwardTo` are independent of `StripControlCommunities` and of the handler. | A wider bug, and the spec scope grows. | `TestShouldForwardToUnaffected` and a `.ci` asserting the suppression set is unchanged. | **VALIDATED.** `TestShouldForwardToUnaffectedByStrip` (`wireu/community_test.go`); both `.ci` files carry a `65000:65002` whitelist tag and the route still reaches 65002. |
| A-4 | Widths 8 and 12 need no separate treatment. | All three share `genericCommunityHandler` with only `valueSize` differing (`handler.go`). | Per-width handling, a larger change. | Table-driven test over widths 4, 8 and 12. | **VALIDATED.** `TestRemoveValuesAllWidths` covers 4, 8 and 12, each with a multi-value removal and a non-multiple refusal. |
| A-5 | No `.ci` currently covers route-server control-community stripping, so no existing functional expectation encodes the broken behaviour. | `grep -rn "StripControlCommunities"` returns four hits, none in a test; no `test/plugin/bgp-rs-*.ci` mentions control communities. | An existing `.ci` pins the leak and must be corrected as part of this fix. | Re-run the grep and read every `test/plugin/bgp-rs-*.ci`. | **VALIDATED.** Four hits, none a test. No existing `.ci` needed correcting; the two new files are the first coverage. |
| A-6 | `session/rs-client true` is required for the strip path to run at all. | Not in the original spec; discovered while writing the functional tests. Both the RFC 7947 policy block (`reactor_api_forward.go`) and the strip emission are gated on `facts.rsClient`, which comes from `config.go`. | A `.ci` without the leaf exercises nothing and passes vacuously against a broken build. | Both new `.ci` files set it on both peers, with a comment saying why. | **VALIDATED 2026-07-28.** An early draft omitted it and forwarded all five communities intact even against a FIXED binary -- indistinguishable from the bug it was meant to catch. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Operators have built policy that depends on seeing the leaked control communities on the client side. | Complaint after upgrade. | The leak contradicts the stated intent in `community.go`. Record the behaviour change in the learned summary and the guide page added by this spec. |
| R-2 | The loud refusal fires in production for a producer the enumeration missed. | The new warning appears in soak logs. | The warning names the code, the expected width and the actual length, so the offending producer is identifiable from one log line. |
| R-3 | The route-server fast-path rail and the general forward rail drift, so the fix lands on one only. | The `.ci` runs one rail and passes while the other leaks. | The functional test must exercise both rails; `test/plugin/bgp-rs-reactor-fastpath.ci` and `bgp-rs-reactor-fastpath-fallback.ci` show how each is selected. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route-server clients either keep receiving internal control communities (no change from today) or, if the matching predicate is wrong, lose communities they should have kept. The second is worse: a client's own downstream policy could stop firing. |
| How is it reverted? | Single commit revert. No wire-format or configuration change, no persisted state. |
| Who else touches this path? | The wire-edit-0 umbrella replaces this contract with a typed structure; hot-path alloc round 4 removes the allocations in the same handler. Both must be rebased on this fix, not the other way round. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Route-server client sends a route tagged with two `0:<asn>` control communities | → | `StripControlCommunities` builds an eight-byte buffer, `genericCommunityHandler` accepts it through `wholeValues` and drops both | `test/plugin/bgp-rs-community-strip-multi.ci` |
| Same route on the route-server fast-path rail | → | `forward_rs.go` operation reaches the same handler | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` |
| Route tagged with exactly one control community | → | unchanged single-value path | `test/plugin/bgp-rs-community-strip-multi.ci` (single-value case in the same scenario) |
| Configured `community { egress strip NAME }` on an ordinary peer | → | per-value operations from `egress.go`, behaviour unchanged | existing `test/plugin/community-strip.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route-server client route carrying two control communities, forwarded to another client | Neither control community appears in the UPDATE the receiving client sees |
| AC-2 | Same, with five control communities interleaved with three ordinary ones | All five control communities are removed; all three ordinary communities are preserved, in their original order |
| AC-3 | Same, with exactly one control community | Behaviour is identical to today: the single community is removed |
| AC-4 | A route whose every community is a control community | The COMMUNITY attribute is omitted entirely from the forwarded UPDATE, matching the existing empty-list behaviour at `handler.go` |
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
| `TestGenericCommunityHandlerOmitsEmptyAttribute` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-4: an emptied list omits the attribute, preserving `handler.go` | |
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
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`: state the Remove-buffer arity obligation alongside the accumulator contract |
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
   - Files: `filterapi/filterapi.go`, `wireu/community.go`, `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`, `docs/guide/bgp-policy.md`, `docs/architecture/core-design.md`, `ai/rules/plugins.md`
   - Verify: both rails produce identical wire; the counter is registered and documented; `make ze-verify` passes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and both route-server rails are covered, not just the one the default configuration selects |
| Feature completeness | The `.ci` files fail before the fix and pass after; a mutation reinstating the size guard turns them red |
| Correctness | Only control communities are removed. An ordinary community whose high half coincidentally equals the route server's low sixteen ASN bits is a real ambiguity in the existing selection rule (`community.go`); confirm the fix does not widen it, and record it as a known limitation rather than silently changing it |
| Naming | The refusal warning names the attribute code, the expected width and the actual length, per `ai/rules/cli.md` |
| Data flow | The forwarding decision path is untouched; only the strip path changes |
| Rule: `ai/rules/evidence.md` | No branch preserves data on a detected contract violation without speaking |
| Rule: `ai/rules/go-standards.md` | The arity obligation is stated on `ModAccumulator.Op`, where the caller reads it, not only in the handler that enforces it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The leak is fixed on the general rail | `make ze-functional-test TEST=bgp-rs-community-strip-multi` |
| The leak is fixed on the fast-path rail | `make ze-functional-test TEST=bgp-rs-community-strip-multi-fastpath` |
| The tests genuinely catch the bug | reinstate the exact-length guard at `handler.go` and confirm both `.ci` files turn red |
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
(`filter_community.go`), so a helper that logged directly could only be tested
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
from `tmp/s/<session-id>/bin` FIRST (`FindPrebuiltDir`, `internal/test/sessionpath/sessionpath.go`).
A stale session-scoped binary was therefore under test for several iterations,
which read exactly like "the fix does not work on this path" and sent this
investigation looking for a third forwarding rail that does not exist
(`ForwardUpdatesDirect` delegates to `forwardUpdateCore` at
`reactor_api_forward_batch.go`). Running WITHOUT `ZE_TEST_NO_BUILD=1` lets the
runner build the DUT itself and avoids this entirely.

## Design Insights

- The defect is not in either half taken alone. `StripControlCommunities` legitimately returns every match, and `removeValues` legitimately defends against a buffer it cannot interpret. The defect is that the arity rule joining them was never written down, so two of four producers violated it and the guard chose silence over noise.
- The guard's own comment, "Size mismatch: caller bug, silently preserve data" (`handler.go`), names the caller bug correctly and then declines to report it. A guard that has already diagnosed the fault is the cheapest possible place to speak.
- Zero coverage is the enabling condition. `StripControlCommunities` has four references in the tree and not one is a test, and no `.ci` exercises route-server control-community stripping. A feature with no test and no documentation has no way to notice it stopped working.
- The forwarding decision and the strip are computed by two independent walks of the same bytes (`community.go` and `:141`), called back to back at `reactor_api_forward.go`. That the decision half works and the strip half does not is a direct consequence of their independence.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Widen the consumer to accept a whole number of values | split into per-value operations at the two producers | Splitting fixes the leak and leaves the fail-open guard, so the next violator fails silently too. Widening fixes both defects at one site and keeps every existing single-value producer working. |
| Refuse a non-multiple length loudly | keep preserving the data silently | `ai/rules/evidence.md`: a guard must fail closed or say something. The guard has already detected the fault; reporting it costs one log line on a path that is already an error. |
| Document the obligation on `ModAccumulator.Op` | document it only on the handler | `ai/rules/go-standards.md` puts caller obligations where the caller reads them. The handler is not what a filter author looks at when calling `Op`. |
| Fix now, independently of `spec-wire-edit-0-umbrella.md` | wait for the typed slot structure to make the arity a compile error | The umbrella is weeks of work; this is a live leak on the route-server path. The umbrella rebases onto this fix. |
| Add a refusal counter | log only | A log line in a soak run is easy to miss. A counter makes R-2 measurable before the next producer appears. |

## Known Limitations

- The selection rule in `StripControlCommunities` (`community.go`) matches on the community's high half alone, so an ordinary community whose high half happens to equal 0 or the route server's low sixteen ASN bits is indistinguishable from a control community and is stripped. This is pre-existing and unchanged by this fix; it is recorded here because the fix makes the stripping actually happen, which makes the ambiguity reachable for the first time in the multi-value case.
- Standard communities can only carry a sixteen-bit ASN in the high half. A route server with a four-octet ASN cannot express the `<rs-asn>:<asn>` form in a standard community at all; `parseCommunityAttr` documents this at `community.go` and directs operators to large communities. Large-community control values are parsed by `parseLargeCommunityAttr` but are **not** stripped: `StripControlCommunities` only inspects code 8. That gap is out of scope here and is a separate defect.
- The two independent attribute walks are not merged. That is the wire-edit-0 umbrella.

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
ze's own, stated at `internal/component/bgp/wireu/community.go`.

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

## Correction to the Implementation Summary (2026-07-28, session 63608781)

The "Not done" note above states that the refusal counter is NOT implemented and
is "left for an owner decision". That is superseded: **the counter shipped in the
same commit** that carries this spec. `filterapi.RecordRemoveBufferRefused` is
defined at `internal/component/bgp/filterapi/metrics.go`, registered as
`ze_bgp_attr_mod_remove_buffer_refused_total` at
`internal/component/bgp/filterapi/metrics.go`, wired from the reactor's
metrics-enable block at `internal/component/bgp/reactor/reactor.go`, called
at `internal/component/bgp/plugins/filter_community/handler.go`, and covered
by four tests in `internal/component/bgp/filterapi/metrics_test.go`.

The objection recorded in that note (this package has no metrics surface) was
resolved rather than accepted: the counter lives in `filterapi`, which already
owns the accumulator contract being violated, not in `filter_community`. The
reasoning is recorded in the code at
`internal/component/bgp/filterapi/metrics.go` and mirrored at
`internal/component/bgp/reactor/reactor.go`, and it is load-bearing:
the AttrMod handlers register at init() and run during the progressive build
whether or not the owning plugin is running, so a plugin `ConfigureMetrics` hook
would leave the counter dead in exactly the configuration where the violation is
reachable.

## Review Follow-Up (2026-07-28, session 63608781)

Three INDEPENDENT reviewers (logic, security, tests) were run over the shipped
commit plus a candidate follow-up fix. RF-2 and RF-3 were fixed at the time.

**RF-1 IS FIXED. The section below is the record of how it stood on 2026-07-28,
kept because the measurements in it are what the fix had to beat.** A first fix
was written, the reviewers found it traded one peer-driven cost for another, and
it was reverted on owner direction because this code was expected to be replaced
wholesale by spec-wire-edit-2-edit-apply. That spec CLOSED on 2026-08-02 without
touching this code, so the contract stood and the defect was fixed here instead:
`newRemovalSet` on 2026-08-05, and its allocation residue (RF-1a) on 2026-08-10.
Read this section as history, and the Findings-fixed table as the current state.

### RF-1 (BLOCKER, FIXED 2026-08-05): the arity fix removed an accidental O(1) short-circuit and exposed a peer-controlled quadratic

`removeValues` called `containsValue` once per retained value, a nested scan over
two operands that are BOTH peer-controlled on the route-server path: `data` is
the peer's own COMMUNITY attribute, and `toRemove` is derived from that same
attribute by `wireu.StripControlCommunities`
(`internal/component/bgp/wireu/community.go`), not from local configuration.
A BGP attribute value reaches 65535 octets, so each side reaches 16383 four-byte
values.

The old `len(toRemove) != valueSize` guard returned immediately for exactly the
multi-value input that now does the work, so the defect this spec fixed was also,
accidentally, the cost cap. Removing it exposed the quadratic.

**This is a live property of the code as committed.** Two reviewers measured it
independently, per destination peer, for a 16383-value all-control COMMUNITY
attribute (65,532 octets, the RFC 4271 ceiling):

| Peer-chosen input | shipped code | candidate set-based fix |
|-------------------|--------------|-------------------------|
| 16383 distinct `0:X` | 874-889 ms, ~0 B | 2.7-6.2 ms, 1,004,810 B, 16,449 allocs |
| 16383 identical `0:0` | **118 us, 0 B** | **1,948 us, 1,004,865 B** |

Reachability was verified, not assumed. `StripControlCommunities` matches on
`high == 0` (`internal/component/bgp/wireu/community.go`), so a repeated
`0:0` is a trivially constructible all-control payload, and it leaves
`BlacklistASNs` empty (`community.go`) so `ShouldForwardTo` returns true for
every client and the fan-out is maximal. No cap exists anywhere on received
community count. The 4 KB message case needs no capability negotiation.

**Why the candidate fix was reverted.** It is 326x better against an attacker who
picks the worst shape, but 16.5x worse and 0 to 1 MB on the duplicate-heavy shape,
because `newValueSet` sized and populated the map from the raw byte length rather
than the distinct count. The reviewers also proved its regression guard was
decorative: reordering `valueSet.contains` to scan the raw bytes first restores
the quadratic with the whole suite green, because the struct held both
representations at once despite a comment claiming it held one. The benchmark
that was supposed to catch this is never executed by any gate
(`mk/alloc-gate.mk` covers only `./internal/component/bgp/reactor/...` and
enforces only names registered in `internal/perf/allocgate.go`).

The work is preserved at `backups/work-20260728-valueset-fix.patch`.

**What the replacement must do** (written for spec-wire-edit-2-edit-apply, which closed without doing it; the requirements below now stand against `handler.go` as it is):
deduplicate at the producer, since `StripControlCommunities` already walks the
payload once per UPDATE and duplicates are what make the map pathological; hoist
the set construction out of the per-destination loop, since
`communityStripBytes` is fan-out-invariant (computed once at
`reactor_api_forward.go`) while `buildModifiedPayload` runs per peer at
`reactor_api_forward.go`; and bound the threshold on `min(|data|, |set|)`
rather than on set size alone.

### RF-2 (ISSUE): eleven stale citations in the commit's own artefacts

The commit moved lines in six files and left citations at pre-move positions in
three artefacts it authored: the `removeValues` doc comment and both `.ci`
headers. The `.ci` ones are re-pointed here (`community.go` to `:158`,
`reactor_api_forward.go` to `:643`, `forward_rs.go` to `:347`, and
`reactor_api_forward.go` to `:642` in four places). `community.go`
and the learned summary were already correct.

The three inside the `handler.go` doc comment are gone: the rewrite of
`genericCommunityHandler` cites `wireu.StripControlCommunities` and the two
route-server rails by symbol, with no line number, so there is nothing left to
drift. The five that survived in `handler_test.go` are converted to symbol
citations in the same pass that closes the findings below. Two of them
(`reactor_api_forward.go`, `forward_rs.go`) had already drifted onto
unrelated code: the real call sites are at `:501` and `:248` today, which is the
argument for not writing the number at all.

No gate catches this class: `scripts/dev/spec-citation-check.py` scans only
`plan/` and `plan/learned/`, and `scripts/dev/check_doc_links.py --design-only`
follows only `// Design:` references. Comments in `.go` and `.ci` files are
unchecked. Recorded as a gap, not fixed here.

### RF-3 (ISSUE): the new metric was never documented

`ze_bgp_attr_mod_remove_buffer_refused_total` was registered but appeared nowhere
under `docs/`, despite row 14 of this spec's own Documentation Update Checklist
naming `docs/plugin-development/metrics.md`. Added to the Full Inventory with a
source anchor pointing at `internal/component/bgp/filterapi/metrics.go`.

### Other findings the reviewers recorded, now closed

All five were carried forward when the main fix landed. They are closed here,
against the rewritten `genericCommunityHandler`. Every test named below was
mutation-verified: the mutation is stated with the result.

| Finding | Outcome | Evidence |
|---------|---------|----------|
| The refusal-message test is vacuous | Fixed | `TestGenericCommunityHandlerWarnsOnNonMultiple` now asserts `buffer-length=3`, `attribute-code=8` and `value-size=4`, never a bare digit. Mutation: delete `"buffer-length", len(ops[i].Buf)` from the handler. Before: green. After: red on the `buffer-length=3` assertion. |
| No discriminating coverage at widths 8 and 12 | Fixed | `TestRemoveValuesAllWidths` now builds two value families per width: one differing in the LAST byte only, one in the FIRST byte only, six values, a three-value non-contiguous Remove, and a near-miss value. Mutation: narrow `containsValue` to the leading 4 bytes -- red at widths 8 and 12. Same for the trailing 4 bytes -- red at both. |
| The keep loop drops a trailing partial value | Fixed | The handler refuses a source value that is not a whole number of values: warn plus `p.Fail()`, which `forward_build.go` turns into `modifyFailureHandlerFault` (already logged and counted by `recordModifyFailure`), so the route is suppressed rather than an attribute the peer never sent being emitted. `TestGenericCommunityHandlerRefusesMalformedSourceValue` at all three widths. Mutation: remove the guard -- red (at width 4 the plan does not merely normalize, it DROPS the attribute). The reviewer's premise was wrong: `message.validateCommunityAttr` (registered in `attrValidators`, reached from `Session.enforceRFC7606` through `message.ValidateUpdateRFC7606AddPath`) classifies a code-8 length of 0 or 4k+r as treat-as-withdraw per RFC 7606 Section 7.8, and `SynthesizeWithdrawFamilies` carries no COMMUNITY into the forward path. Codes 16 and 32 have the same rule. Local origination writes 4 bytes per community (`reactor.writeCommunitiesAttr`). The guard is therefore unreachable today and exists so a NEW producer is heard on its first route. |
| `data = data[:65535]` truncates mid-value | Already resolved by the rewrite | No truncation exists. The cap is `attrValueMax` in `internal/component/bgp/filterapi/editset.go`, and every site that meets it REFUSES: `AttrPlan.appendFragment`, `AttrPlan.New` and `AttrPlan.NewByte` each call `p.Fail()`. The plugin's own ingress path (`filter.go`) likewise returns nil rather than truncating. |
| Empty `toRemove` is untested | Fixed | `TestGenericCommunityHandlerEmptyRemoveIsSilentNoOp` pins nil and `[]byte{}`: the attribute is forwarded unchanged, nothing is logged, the refusal counter is not touched. Mutation: `wholeValues` rejects the empty buffer -- red on both the log and the counter assertions. |

## Implementation Summary (closure, 2026-08-10)

The dated section above is the 2026-07-28 record of the first landing and is
kept as history. This is the shipped state at closure.

### What Was Implemented

- `wholeValues` and a rewritten `genericCommunityHandler`
  (`internal/component/bgp/plugins/filter_community/handler.go`). A Remove
  buffer holding N whole values removes all N. A buffer whose length is not a
  whole multiple of the attribute width is refused per operation, logged with
  `attribute-code`, `value-size` and `buffer-length`, and counted as
  `ze_bgp_attr_mod_remove_buffer_refused_total`. The attribute's other
  operations still apply. The helper the spec named, `removeValues`, is gone:
  the handler plans retained runs itself.
- `newRemovalSet` in the same file: the membership representation is chosen ONCE
  per attribute, above the loop over source values, scanning below
  `removalIndexThreshold` and answering from a map above it, thresholded on
  `min(source values, removal values)`. It carries no size hint and reads each
  value before inserting it, so the index cost tracks DISTINCT values.
- `filterapi.ModAccumulator.Op` states the arity obligation where the caller
  reads it, by symbol and never by line
  (`internal/component/bgp/filterapi/filterapi.go`).
- The counter and its wiring: `filterapi.RecordRemoveBufferRefused`
  (`internal/component/bgp/filterapi/metrics.go`), enabled from the reactor's
  metrics block, in `filterapi` rather than in the plugin because the AttrMod
  handlers register at `init()` and run whether or not the owning plugin runs.
- Tests: 26 in `filter_community/handler_test.go`, 6 in `wireu/community_test.go`,
  4 in `filterapi/metrics_test.go`, and both route-server rails end to end in
  `test/plugin/bgp-rs-community-strip-multi.ci` and its `-fastpath` sibling.
- The contract written down where the next author meets it: three points under
  `ai/rules/points/plugins/modification-accumulator-buffer-arity/`, rendered into
  `ai/rules/plugins.md`, and the "Buffer arity, list-valued attributes"
  subsection of `docs/architecture/core-design.md`.

### Bugs Found/Fixed

- The leak itself: a route carrying two or more control communities kept all of
  them. Covered by both `.ci` files, mutation-verified (restoring the old guard
  turns both red with the leak printed).
- The fail-open guard that had hidden it since introduction. Covered by
  `TestRemoveValuesNonMultipleRefusedLoudly` and
  `TestGenericCommunityHandlerWarnsOnNonMultiple`.
- RF-1, a peer-controlled quadratic the arity fix exposed (874 to 889 ms per
  destination peer). Covered by `TestRemovalSetIndexesOnlyAboveThreshold`.
- RF-1a, its residue: an index SIZED by the raw value count, 939,312 bytes for
  one repeated value with one entry stored. Covered by
  `TestRemovalSetIndexAllocatesByDistinctValues` (272 bytes) and, on the other
  side of the trade, `TestRemovalSetIndexBoundsAllDistinctValues`.
- A malformed source value would have emitted an attribute length no peer sent.
  The handler refuses it and fails the plan:
  `TestGenericCommunityHandlerRefusesMalformedSourceValue` at all three widths.
- Three test-quality defects the reviewers found: a vacuous refusal-message
  assertion, no discriminating coverage at widths 8 and 12, and an untested
  empty `toRemove`. All three are fixed and each fix is mutation-verified.
- One find NOT fixed here, and recorded rather than carried: the whole-value
  check covers the Remove buffer and the source value, but not the Add buffer.
  It is unreachable today (all three Add producers allocate exactly
  `len(tokens)*width` or a fixed array), so it is a row in
  `plan/journal/gate-excludes-part-of-its-population.md` naming the fix it
  needs, not a change in this commit.

### Documentation Updates

- `docs/guide/bgp-policy.md`: the route-server control-community convention.
  Landed with the first commit.
- `docs/plugin-development/metrics.md`: the counter, with a source anchor on
  `internal/component/bgp/filterapi/metrics.go` (RF-3). Landed with the first
  commit.
- `ai/rules/plugins.md` and the three points under
  `ai/rules/points/plugins/modification-accumulator-buffer-arity/`
  (Documentation checklist row 8). In this commit.
- `docs/architecture/core-design.md`, "Buffer arity, list-valued attributes
  (caller obligation)", with source anchors on
  `filter_community.wholeValues`, `wireu.StripControlCommunities` and
  `filterapi.RecordRemoveBufferRefused` (row 12). In this commit.
- Anchor evidence: `python3 scripts/dev/code_to_docs.py --check` reports
  "checked 2043 code paths, 500 packages", `ai/CODE-TO-DOCS.md` up to date, "all
  references valid". `make ze-doc-test` was NOT run in this closure session: the
  machine carries a temporary memory limit that bars every test suite, and the
  session was instructed to name any suite it could not run instead of running
  it.

### Deviations from Plan

- The refusal is reported by the CALLER, not inside the helper the spec named.
  `removeValues` returned `(data, ok)` for that reason, and the handler rewrite
  then deleted the helper entirely; `wholeValues` is where the arity is judged
  today.
- The refusal counter shipped, contrary to the "Not done" note in the 2026-07-28
  summary, and it lives in `filterapi` rather than in `filter_community`.
- `TestRSStripEndToEndBothRails` was superseded by the two `.ci` files, which
  exercise both rails through the daemon rather than through a Go test.
- Nothing in the plan anticipated that removing a rejecting guard also removes a
  cost bound. RF-1 and RF-1a are entirely outside the original design, and they
  are what turned a one-file fix into four review passes.

## Mistake Log

| # | What happened | Root cause | Rule that would have caught it |
|---|---------------|------------|--------------------------------|
| 1 | The arity fix introduced a peer-controlled quadratic (RF-1). | The replacement was designed against the correctness defect only. The removed guard's second, accidental role as a cost cap was never asked about. | `ai/rules/planning.md`: the Review Gate is what surfaces this, and it was never run before the commit landed. |
| 2 | Eleven citations in the commit's own artefacts pointed at lines the same commit moved (RF-2). | Citations were written against the pre-edit file and not re-resolved after the edit. | `ai/rules/evidence.md` mandates the citation form, but no gate covers comments in `.go` or `.ci`. |
| 3 | The Documentation checklist promised a metrics entry that was never written (RF-3). | The checklist was filled at design time and never re-verified at closure. | The Pre-Commit Verification "Documentation Verified" table, unreached because the closure template was never appended. |
| 4 | Commit A landed without the closure template, so there was no Review Gate, Implementation Audit or Pre-Commit Verification section to fail. | The two-commit closure was started and abandoned mid-way; the session moved to another spec. | `ai/rules/planning.md` Spec Closure, enforced by `scripts/dev/spec-closure-check.py --spec` (exit 3), which was not run. |
| 5 | The RF-1 fix sized the map from the raw value count, so the fix for a CPU quadratic allocated 939,312 bytes for one repeated value and stored one entry (RF-1a). | A size hint reads as free, and the input whose SHAPE the fix exists to handle -- a duplicate-heavy buffer -- is exactly the input the hint sizes wrongly. | `ai/rules/performance.md`: a hint is a claim about the distinct count, and the review pass that measured it is what caught it. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A Remove buffer of N values removes all N | done | `handler.go` `wholeValues`, consumed by `genericCommunityHandler` | Option B as chosen. The `removeValues` helper the row used to name was deleted by the handler rewrite: the handler plans retained runs itself |
| A non-multiple length is refused loudly | done | `handler.go` refusal branch, warning plus counter | `filterapi/metrics.go` |
| The arity obligation documented on `ModAccumulator.Op` | done | `internal/component/bgp/filterapi/filterapi.go` | |
| The route-server convention documented | done | `docs/guide/bgp-policy.md` | |
| The refusal is observable | done | `ze_bgp_attr_mod_remove_buffer_refused_total`, now also in `docs/plugin-development/metrics.md` | RF-3 |
| No peer-controlled quadratic on the strip path | done | `newRemovalSet` in `internal/component/bgp/plugins/filter_community/handler.go`, pinned by `TestRemovalSetIndexesOnlyAboveThreshold` | RF-1. A first fix was written here and reverted; the shard `plan/deferrals/fixit-rs-community-strip-arity.md` re-homed it and the quadratic went on 2026-08-05. Its residue, a map sized by the raw value count, is RF-1a and is fixed here |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1, AC-2 | done | `test/plugin/bgp-rs-community-strip-multi.ci` | |
| AC-3 | done | `TestRemoveValuesSingleUnchanged`, `TestStripControlCommunitiesSingleValue` | |
| AC-4 | done | `TestRemoveValuesMultiRemovingEverythingOmitsAttribute` | |
| AC-5 | done | `TestRemoveValuesNonMultipleRefusedLoudly`, `TestGenericCommunityHandlerWarnsOnNonMultiple` | |
| AC-6 | done | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` | |
| AC-7 | done | `TestRemoveValuesAllWidths` | widths 4, 8, 12 |
| AC-8 | done | `TestShouldForwardToUnaffectedByStrip` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The nine unit tests named in the TDD plan | present | `wireu/community_test.go`, `filter_community/handler_test.go` | plus seven beyond the plan |
| `TestRSStripEndToEndBothRails` | superseded | both `.ci` files | the two functional tests cover both rails end to end, stronger than the planned Go test |
| `bgp-rs-community-strip-multi` | present | `test/plugin/` | |
| `bgp-rs-community-strip-multi-fastpath` | present | `test/plugin/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/filter_community/handler.go` | done | arity fix, then `newRemovalSet` for RF-1, then the RF-1a allocation fix |
| `internal/component/bgp/filterapi/filterapi.go` | done | `Op` documents the arity obligation |
| `internal/component/bgp/wireu/community.go` | done | doc comment states the multi-value return |
| `internal/component/bgp/reactor/reactor_api_forward.go` and `forward_rs.go` | done | comments naming the contract at both emission sites |
| `docs/guide/bgp-policy.md` | done | the convention documented |
| `docs/plugin-development/metrics.md` | done | RF-3 |
| `internal/component/bgp/filterapi/metrics.go` and `metrics_test.go` | done | beyond plan: the counter |
| `ai/rules/points/plugins/modification-accumulator-buffer-arity/` and the rendered `ai/rules/plugins.md` | done | Documentation checklist row 8 |
| `docs/architecture/core-design.md` | done | Documentation checklist row 12 |

### Audit Summary
Every AC is demonstrated. Two files were added beyond the plan (the metrics pair)
and one planned Go test was superseded by stronger functional coverage. The
review passes found two BLOCKER-severity defects on the same code path, RF-1 and
its residue RF-1a, and both are fixed and pinned by a test that goes red without
the fix. Every documentation row answered Yes is written.

## Goal Validation (BLOCKING)

| Goal | Evidence |
|------|----------|
| A route carrying two or more control communities has all of them stripped | Both `.ci` files pass, and the implementing session mutation-verified that restoring the original guard turns both red with the leak visible: `COMMUNITIES: 0:64998 65001:100 0:64999 65000:65002 65001:200` |
| The single-value path is unchanged | `test/plugin/community-strip.ci` and `TestRemoveValuesSingleUnchanged` green |
| A contract violation is visible in production | warning plus `ze_bgp_attr_mod_remove_buffer_refused_total`, documented |
| The strip cannot be turned into a denial of service by a peer | **MET on CPU, and the memory trade is stated, not hidden.** `newRemovalSet` (`internal/component/bgp/plugins/filter_community/handler.go`) chooses the membership representation ONCE per attribute, above the loop over source values, and answers from a map above `removalIndexThreshold`, so the 874 to 889 ms per destination peer is gone. `TestRemovalSetIndexesOnlyAboveThreshold` pins the representation on both sides of the boundary and on each operand independently, including the measured attack shape. **Allocation moved in both directions**, measured over 16383 four-byte values, the RFC 4271 attribute ceiling: an all-`0:0` buffer fell from 873,744 bytes to 272 (`TestRemovalSetIndexAllocatesByDistinctValues`), and an all-distinct `0:X` buffer ROSE from 939,264 to 1,812,792, 1.93x, because a hintless Go map grows geometrically and discards each intermediate table (`TestRemovalSetIndexBoundsAllDistinctValues`, ceiling 3 MiB). What was traded: a bounded rise in transient memory, freed when the attribute is written, for the removal of a peer-controlled CPU quadratic. Both halves now carry a byte ceiling, so neither can grow in silence. |

## Deferrals Resolved

The shard is `plan/deferrals/fixit-rs-community-strip-arity.md`, named in the
metadata table above. It carries three rows and all three are `done`, so it is
terminal and the closure commit removes it (`ai/rules/planning.md`: a shard
still holding a live row outlives its source spec; this one holds none). No row
in it names another spec's shard, so this closure empties no foreign shard.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-07-28, RF-1: the peer-controlled quadratic | done | RESOLVED 2026-08-05. `newRemovalSet` picks the representation once per attribute; `TestRemovalSetIndexesOnlyAboveThreshold`. The row's own residue, a map sized by the raw value count, is fixed here as RF-1a and measured by `TestRemovalSetIndexAllocatesByDistinctValues` |
| 2026-07-28, RF-2 remainder: three stale `file:line` citations in the `removeValues` doc comment | done | RESOLVED before 2026-08-05. `removeValues` and its comment no longer exist. The same class in this spec's other artefacts is re-anchored by SYMBOL here: both `.ci` headers, `wireu/community_test.go`, and the `ModAccumulator.Op` contract comment |
| 2026-07-28, reviewer NOTE: the refusal-message test was vacuous | done | RESOLVED before 2026-08-05. The assertions name `buffer-length=3`, `attribute-code=8` and `value-size=4` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-rs-community-strip-arity-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean. Exit 0, all six file hashes match, re-run immediately before the closure commit was prepared |
| Rounds | 4. Round 2 earned the extra passes and `--rounds-reason` names it: RF-1a, a peer-reachable 939,312-byte allocation in `newRemovalSet`, sized by the raw value count while storing one entry. Rounds 3 and 4 each re-read what the previous round's fix wrote |
| Reviewer lenses used | logic, security and tests over the shipped commit (2026-07-28, three independent reviewers); allocation and cost over the RF-1 fix (2026-08-05); measurement consistency across `handler.go`, `handler_test.go` and this spec (2026-08-10, final pass) |

The final pass returned 0 BLOCKER, 0 ISSUE, 3 NOTE. Its verdict is `clean`, it
holds no edit tools, and it re-read `newRemovalSet` from source: no size hint,
read before insert, both `wholeValues` guards present, threshold on
`min(valueCount, removals)`. It also verified that the `handler_test.go` diff
changes no assertion: it deletes exactly three comment lines and adds the rest.

**Why the earlier passes could not record the artifact.** Session 63608781 wrote
code in this spec (an RF-1 fix, since reverted), so its own pass did not qualify;
three independent reviewers ran instead and their findings are under Review
Follow-Up. The 2026-08-10 pass produced seven findings and its remediation round
wrote code, and the second remediation round for F1, F2 and F3 wrote code as
well. Each fix is new code that needs a fresh pass (`ai/rules/planning.md`), so
each of those rounds hands the artifact to the next pass rather than recording
its own. The pass recorded above wrote nothing.

`internal/component/bgp/filterapi/filterapi.go` also carries a second,
comment-only hunk from spec-fixit-egress-filter-non-decision-channel, in the
`EgressFilterFunc` doc comment. The two hunks are disjoint and every symbol
either one names resolves at HEAD, so whichever spec commits first carries a
correct, compiling comment for the other. This closure carried the file, so it
carried that hunk too, and the closure commit body says so.

### Notes recorded (record-only, no product defect)

The final pass called all three "worth correcting, not blocking". None is a
defect in shipped behaviour, and none earned another round
(`ai/rules/planning.md`: a finding in the record is not a finding in the
product). Each names the next touch of the file as its home, because editing a
hashed file after the artifact was written invalidates the gate.

| # | Note | Where it lives | Disposition |
|---|------|----------------|-------------|
| N-1 | The `-race` allocation figure is stated to the byte and is not run-stable. Three isolated `-race -count=1` runs measured 2,081,184 / 2,081,552 / 2,009,384 bytes: a 1.8% spread, none of them the recorded 2,072,584, and the derived 1.52x headroom is 1.51x at the worst value. The PLAIN figure is exact and reproducible: 1,812,792 on three isolated runs | `maxAllDistinctIndexBytes`'s comment in `internal/component/bgp/plugins/filter_community/handler_test.go` | This spec's copy is corrected to "about 2.07 MB" with the spread (F2 row above). The comment still states the byte figure and should read "about 2.07 MB (measured 2,009,384 to 2,081,552 over three isolated runs)" the next time that file is touched for another reason. The ceiling and its rationale are unaffected: 3 MiB still leaves 1.51x headroom over the worst observed value and still catches one further table doubling |
| N-2 | "Read the table above `newRemovalSet`" misdirects by one preposition. The two-shape table is INSIDE `newRemovalSet`, above the `set.index = make(...)` line; what sits above the function is the `removalSet` type comment. The symbol is right | the new paragraph in `internal/component/bgp/plugins/filter_community/handler_test.go` | Correct "above" to "inside" on the next touch of that file. Navigation only; the named symbol already leads the reader to the right place |
| N-3 | `removeValues` is written in the present tense in the "Implementation Summary (2026-07-28, session dd843d81)" section, and that helper no longer exists | this spec, dated-history section | Left as written, deliberately. The section is dated history of a superseded implementation and the audit tables above state explicitly that the helper was deleted by the handler rewrite, so a reader who reaches the heading is not misled |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| RF-1 | BLOCKER | peer-controlled quadratic on the strip path | `filter_community/handler.go`, the per-value membership test now in `removedByAny` | **FIXED 2026-08-05.** `newRemovalSet` picks the representation once per attribute, above the per-value loop, and answers from a map above `removalIndexThreshold`. Pinned by `TestRemovalSetIndexesOnlyAboveThreshold` |
| RF-1a | BLOCKER | the index was SIZED by the raw value count, so one repeated value allocated 939,312 bytes with one entry stored | `filter_community/handler.go` `newRemovalSet` | **FIXED 2026-08-10.** Size hint dropped, and each value is looked up before it is inserted, so the cost tracks distinct values: 272 bytes on the same input. `TestRemovalSetIndexAllocatesByDistinctValues` measures the bytes and goes red against the hint. The comment claiming the index "costs the map one entry rather than 16383" is corrected to say what the code does. What dropping the hint costs on the all-distinct shape is F1 and F2 below |
| RF-2 | ISSUE | eleven stale self-citations | `handler.go`, both `.ci` files | re-pointed to current positions |
| RF-3 | ISSUE | new metric undocumented | `docs/plugin-development/metrics.md` | inventory row plus source anchor |
| F1 | -- | the comment justifying the dropped size hint stated one shape as if it covered both: true for a duplicate-heavy buffer, false for a distinct one where the buffer length IS the distinct count | `filter_community/handler.go` `newRemovalSet` | **FIXED 2026-08-10 (round 2).** The comment now carries both shapes and their measured bytes, the 1.93x rise on the distinct shape, and the reason the trade was taken. The Goal Validation row says the same. No behaviour change: the hint stays dropped by decision |
| F2 | -- | only the improved shape had a byte ceiling, so the regressed one could grow in silence | `filter_community/handler_test.go` | **FIXED 2026-08-10 (round 2).** `TestRemovalSetIndexBoundsAllDistinctValues` bounds 16383 distinct values at `maxAllDistinctIndexBytes` (3 MiB), measured 1,812,792 bytes on a plain build (exact and reproducible over three isolated runs) and about 2.07 MB under `-race`, where the figure is not run-stable: 2,081,184 / 2,081,552 / 2,009,384 over three isolated runs, a 1.8% spread. See note N-1 below |
| F3 | -- | the byte-growth `PREVENTS:` line sat on `TestRemovalSetDeduplicatesIndexEntries`, whose entry-count assertion was green under the pre-fix code | `filter_community/handler_test.go` | **FIXED 2026-08-10 (round 2).** The line moved to the byte tests. The entry test now says what an entry count pins, and that it cannot see bytes |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/bgp-rs-community-strip-multi.ci` | yes | listing reports 7826 bytes |
| `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` | yes | listing reports 8051 bytes |
| `internal/component/bgp/plugins/filter_community/handler_test.go` | yes | 26 `func Test` |
| `internal/component/bgp/wireu/community_test.go` | yes | 6 `func Test` |
| `internal/component/bgp/filterapi/metrics.go` and `metrics_test.go` | yes | 4 `func Test` |
| `docs/plugin-development/metrics.md` | yes | inventory row present |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-6 | all control communities stripped on both rails | functional runner: `pass 2/2 100.0% 2.4s`, tests 86 and 87 |
| AC-3, AC-4, AC-5, AC-7 | single-value unchanged, empty omits, refusal loud, all widths | `go test ./internal/component/bgp/plugins/filter_community/` ok, 27 tests including all four |
| AC-8 | forwarding decision unaffected | `go test ./internal/component/bgp/wireu/` ok, includes `TestShouldForwardToUnaffectedByStrip` |
| RF-1, RF-1a | satisfied | `newRemovalSet` answers from a map above the threshold and is built once per attribute: `TestRemovalSetIndexesOnlyAboveThreshold`. Its cost tracks distinct values: `TestRemovalSetIndexAllocatesByDistinctValues` measures 272 bytes against 939,312 before the fix, and goes red against the size hint. The other side of that trade is bounded too: `TestRemovalSetIndexBoundsAllDistinctValues` measures 1,812,792 bytes for 16383 distinct values, up from 939,264 with the hint, under a 3 MiB ceiling |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| RS client route with several control communities, general rail | `test/plugin/bgp-rs-community-strip-multi.ci` | yes: read the file; it announces five interleaved communities covering both strip forms and asserts survivor order |
| Same, fast-path rail | `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` | yes: identical scenario with `rs-fast-path enable` on the source |
| Configured per-peer egress strip, unchanged | `test/plugin/community-strip.ci` | yes: existing test, still green |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | A tree-wide search for `AttrModRemove` over `internal/`: `filter_delta.go` splits, `filter_community/egress.go` is per value, only `reactor_api_forward.go` and `forward_rs.go` pass a multi-value buffer |
| A-2 | confirmed | `TestRemoveValuesSingleUnchanged` green; `test/plugin/community-strip.ci` unchanged and green |
| A-3 | confirmed | `TestShouldForwardToUnaffectedByStrip` green; the strip and the policy parse are independent walks |
| A-4 | confirmed | `TestRemoveValuesAllWidths` covers 4, 8 and 12 through the one shared handler |
| A-5 | confirmed | at authoring time a tree-wide search for `StripControlCommunities` returned four hits, none a test; neither `.ci` file existed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/bgp-policy.md` documents the control-community convention | 29 lines added by the commit | yes |
| `docs/plugin-development/metrics.md` lists the counter | inventory row plus a source anchor naming `internal/component/bgp/filterapi/metrics.go`; the doc-anchor validator resolves it | yes |
| No RFC compliance claim attached to the strip | a search for `RFC 7947` in `internal/component/bgp/plugins/filter_community/handler.go` is empty, as the spec's Required Reading required | yes |
| Row 9 answered No (no RFC behaviour changed) | RFC 7947 places no requirement on control-community stripping; its compliance checklist has no matching row | yes |
| Row 8: `ai/rules/plugins.md` states the Remove-buffer arity obligation | new section "Modification-Accumulator Buffer Arity (BLOCKING for filter plugin authors)", authored as three points under `ai/rules/points/plugins/modification-accumulator-buffer-arity/` and rendered by `make ze-rules-render`. It was answered Yes at design time and never written until 2026-08-10 | yes |
| Row 12: `docs/architecture/core-design.md` records the arity in the modification-accumulator section | new subsection "Buffer arity, list-valued attributes (caller obligation)" under the `AttrOp` table, with source anchors on `filter_community.wholeValues`, `wireu.StripControlCommunities` and `filterapi.RecordRemoveBufferRefused`. Answered Yes at design time and never written until 2026-08-10 | yes |
| Row 16: no doc anchor names a claim this change falsifies | `code_to_docs.py --check` resolves every anchor this change adds, and regenerating `ai/CODE-TO-DOCS.md` produced a zero diff. Its one reported failure is `docs/guide/ipsec.md:448` naming `vppUnsupportedSelector`, which belongs to a concurrent session editing `internal/component/ike/dataplane/vpp.go`. The `ModAccumulator.Op` contract comment and both `.ci` headers are re-anchored by SYMBOL, so no `file:line` remains to drift | yes |

## Core Insight

The arity rule joining `StripControlCommunities` to the COMMUNITY handler lived in a
comment inside one helper, and two of its four producers broke it silently for
months. Making the rule explicit fixed the leak, but the same edit removed a
guard that had been doing a second, undocumented job: bounding the cost. A guard
that rejects an input is also, by construction, a limit on the work that input
can cause, so replacing "reject" with "handle" transfers that limit onto the
handler. That is the general lesson for
spec-wire-edit-2-edit-apply, which was expected to replace this whole contract with a
typed slot: the type must carry the bound, not just the shape.
