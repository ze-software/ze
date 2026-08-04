# Spec: fixit-send-community-suppress-ignored

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 2/2 |
| Deferral shard | - |
| Updated | 2026-08-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`session { community { send ... } }` set to anything other than `all` (or left
unset) is SILENTLY DISCARDED whenever the community attribute is present on the
route. Every peer, both forward rails. An operator who configures
`community { send none; }` still leaks every COMMUNITY, EXTENDED_COMMUNITY and
LARGE_COMMUNITY value to that peer.

The goal is to make the configured suppression reach the wire, and to close the
identical blind spot in the OTC handler before a producer rediscovers it.

### Second item, HOMED HERE on 2026-08-04: LOCAL_PREF crosses to external peers

A second egress defect of the SAME class is in scope for this spec, and it was
homed here rather than opened as its own spec for three reasons: it is the same
producer (`applyFacts*` on `reactor/peer_forward_facts.go`), it is the same
defect class (an attribute the egress rail must remove and does not), and
fixing the two together costs the forward hot path one pass instead of two.

RFC 4271 Section 5.1.5: "A BGP speaker MUST NOT include this attribute in UPDATE
messages it sends to external peers, except in the case of BGP Confederations
[RFC3065]." Two rails obeyed it and one did not:

| Rail | Producer | Before |
|------|----------|--------|
| Announce (API / batch) | `buildAnnounceUpdate` (`reactor_api_batch.go`) | strips, correct |
| Stored-RIB replay | `writeRIBRoutes` (`peer_rib_routes.go`) | strips, correct |
| Wire builder | `writeAnnounceUpdateWithPlan` (`reactor_wire.go`) | writes only under iBGP, correct |
| **Forward / relay** | `forwardUpdateCore` (`reactor_api_forward.go`), `reactorForwardRS` (`forward_rs.go`) | **no strip at all** |

So a route LEARNED from an internal peer and RELAYED to an external one carried
the internal preference across the AS boundary, while the same prefix originated
locally did not. Every `LOCAL_PREF` reference under `reactor/forward*.go` was in
a `_test.go` file, and `prependDefaultFilters` (`bgp/config/peers.go`) prepends
only loop-detection entries and only to `ImportFilters`, so no auto-added export
filter covered it either.

Confederations are the RFC's own exception. Ze has no confederation member-AS in
`PeerSettings` or the YANG tree, so the exception is constantly false; the
predicate that owns it is named below so a future member-AS grows it in ONE
place.

**Symptom:** the attribute is re-emitted unchanged. Nothing is logged, no
counter moves, and the route is correctly excluded from zero-copy passthrough,
so the daemon does extra work to produce the byte-identical wrong answer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - attribute modification handler dispatch
  → Constraint: a handler PLANS and writes no bytes; it must finish with exactly
    one of `Emit`, `EmitExtended`, `Drop`, `Fail` (`filterapi/editset.go`, `AttrPlan`).
  → Decision: a suppressed attribute is a `Drop` slot, never an empty `Emit`.
    An attribute with a zero-length value is not the same thing as no attribute
    (`AttrPlan.ValueLen` doc comment).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1997.md` - COMMUNITY, 4-octet values
- [ ] `rfc/short/rfc4360.md` - EXTENDED_COMMUNITY, 8-octet values
- [ ] `rfc/short/rfc8092.md` - LARGE_COMMUNITY, 12-octet values
  → Constraint: none of the three attributes is well-known mandatory, so omitting
    one entirely is legal. Suppression has no RFC obstacle.
- [ ] `rfc/short/rfc9234.md` - OTC (Section 5)
  → Constraint: [RFC9234-5-6] [MUST] "Once the OTC Attribute has been set, it
    MUST be preserved unchanged." This is a GATED MUST. It decides the OTC fix:
    preservation wins over a Suppress op, so the handler must REFUSE the
    suppression and say so, never honor it.

**Key insights:**
- The last-Set-or-Suppress-wins rule already exists for generic codes
  (`reactor/filter_delta_handlers.go`, `lastSetOrSuppress`). The community and
  OTC handlers predate it and never adopted it.
- Suppression must stay its own action. Expressing it as an empty `AttrModSet`
  re-overloads the exact ambiguity `AttrModSuppress` exists to remove.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler`
      scans ops for `AttrModSet` ONLY. There is no `AttrModSuppress` branch.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - `lastSetOrSuppress`,
      `genericAttrSetHandler`, `genericAttrCodes`, `attrModHandlersWithDefaults`.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeSendCommunity`,
      `applyFactsSendCommunity` (the producer).
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `applySendCommunityFilter`
      (the second producer, same three codes).
- [ ] `internal/component/bgp/plugins/role/otc.go` - `otcAttrModHandler`.
- [ ] `internal/component/bgp/filterapi/editset.go` - `AttrPlan` contract.

**The mechanism, verified at each link:**

| # | Link | Evidence |
|---|------|----------|
| 1 | `precomputeSendCommunity` sets `scMask` for anything but `all`/unset | `peer_forward_facts.go` |
| 2 | `applyFactsSendCommunity` emits `mods.Op(8\|16\|32, AttrModSuppress, nil)` | `peer_forward_facts.go` |
| 3 | Both rails call it: `forward_rs.go` and `reactor_api_forward.go` | grep, 2 non-test call sites |
| 4 | `genericCommunityHandler` finds no Set, so `setIdx` stays -1 | `filter_community/handler.go` |
| 5 | No Remove op names any value, so every source value is `Keep`-ed | same |
| 6 | `ValueLen() != 0`, so `emitCommunity` re-emits the attribute intact | same |
| 7 | The Suppress-aware `genericAttrSetHandler` never runs for 8/16/32: `genericAttrCodes` omits them and `attrModHandlersWithDefaults` fills only nil slots | `filter_delta_handlers.go` |
| 8 | `filter_community/register.go` claims all three at `init()`, blank-imported from the generated composition root | `register.go` |
| 9 | `otcAttrModHandler` returns `KeepAll()` whenever `Source() != nil`, before any op is inspected | `role/otc.go` |

**Behavior to preserve:**
- `EmitExtended` for every community attribute: the 4-byte header class must not
  change for any community attribute Ze forwards.
- An empty `AttrModSet` buffer keeps dropping the attribute (existing behavior,
  not extended to new callers).
- Remove/Add semantics, the malformed-Remove refusal, and its counter.
- OTC preservation when the source already carries it (RFC9234-5-6).
- The 12 transforms pinned by `TestGoldenBytesUnchangedTier1`.

**Behavior to change:**
- A community attribute whose last Set-or-Suppress op is a Suppress is DROPPED.
- A Suppress op on OTC with a source attribute present is REFUSED OUT LOUD
  (warn + preserved), rather than consumed in silence.

## Data Flow (MANDATORY)

### Entry Point
- Config leaf `session { community { send <list> } }` -> `PeerSettings.SendCommunity`.

### Transformation Path
1. `precomputeSendCommunity` folds the list into `peerForwardFacts.scMask`.
2. `applyFactsSendCommunity` records one `AttrModSuppress` op per suppressed code
   on the per-destination `ModAccumulator`.
3. `buildModifiedPayload` (`reactor/forward_build.go`) groups ops by code and
   calls `planAttr` -> the registered `AttrModHandler` for that code.
4. The handler plans a slot; the writer materializes the attribute section.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| reactor -> filter_community plugin | `filterapi.AttrModHandlers()` registry, populated at `init()` | Yes |
| reactor -> role plugin | same registry, code 35 | Yes |
| handler -> wire | `AttrPlan` slot (`Emit`/`Drop`), materialized by the edit-set writer | Yes |

### Integration Points
- `filterapi.AttrOp` / `AttrModSuppress` - the action the handlers must honor.
- `lastSetOrSuppress` - the existing rule to reuse, not re-invent.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The fix is inside the registered handlers, the layer that already owns the decision |
| No unintended coupling | Yes | The shared rule moves to `filterapi`, the leaf package both `reactor` and the plugins already import |
| No duplicated functionality | Yes | One definition of last-Set-or-Suppress-wins replaces one definition plus two blind spots |
| Zero-copy preserved | Yes | Drop plans no fragment; Keep/Op paths are untouched |
| Registration over hardcoding | Yes | No new registration, no per-feature field in a shared package |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | No production producer emits `AttrModSuppress` for code 35 today | grep of `AttrModSuppress` producers: codes 8, 16, 32, 40 only | The OTC change would be wire-visible rather than latent | grep (done) | confirmed |
| A-2 | Moving `lastSetOrSuppress` into `filterapi` changes no emitted byte | It is a pure fold over ops with no I/O | Goldens move | `TestGoldenBytesUnchangedTier1` + variants | confirmed (12/12 transforms unmoved) |
| A-3 | The 3 community codes have exactly one registered handler each, from `filter_community` | `register.go` `init()`; `attrModHandlersWithDefaults` fills only nil slots | A second registration could mask the fix | grep of `RegisterAttrModHandler` | confirmed (4 production registrations total: codes 8, 16, 32, 35) |
| A-4 | A reactor unit test can reach the REAL community handlers through the registry | The plugin registers at `init()`; the test binary must link it | The test would assert a handler it built itself, which is the vacuity that hid this defect | build + run the new test | confirmed; the new test file carries its OWN blank import rather than inheriting one |
| A-5 | Only the community handlers and OTC read `AttrModSet` alone | The task's diagnosis named those two | Other handlers keep the same fail-open | audit every handler in `attrModHandlersWithDefaults` | **broken** -- `clusterListHandler` (code 10) and `originatorIDHandler` (code 9) had the identical blind spot. Both fixed. See Design Insights |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new test passes without the fix (vacuous) | Mutation check does not go red | Revert the Suppress branch and require RED with the attribute still on the wire |
| R-2 | Honoring Suppress on OTC would break a gated RFC MUST | RFC9234-5-6 | Preservation wins; the handler warns instead of dropping |
| R-3 | A golden moves because `EmitExtended` or the empty-Set drop was disturbed | `TestGoldenBytesUnchangedTier1` | Stop and report, per the task's instruction |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Communities are stripped from routes an operator wanted them on, or keep leaking to peers configured to receive none. Wire-visible on every forwarded route carrying a community. |
| How is it reverted? | Single commit revert. No config migration, no state. |
| Who else touches this path? | The six `spec-wire-edit-*` specs are closing concurrently; this spec touches the same handlers but not those spec files. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `PeerSettings.SendCommunity` -> `applyFactsSendCommunity` -> `buildModifiedPayload` | → | `genericCommunityHandler` Suppress branch | `TestSendCommunitySuppressEmittedBytes` |
| `mods.Op(35, AttrModSuppress, nil)` -> `buildModifiedPayload` | → | `otcAttrModHandler` Suppress branch | `TestOTCSuppressRefusedAndPreserved` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `community { send none; }`, route carries COMMUNITY, EXTENDED_COMMUNITY and LARGE_COMMUNITY | The rebuilt attribute section contains NONE of codes 8, 16, 32 |
| AC-2 | `community { send standard; }`, route carries all three | Section contains code 8 with its original values, and neither 16 nor 32 |
| AC-3 | `community { send all; }` or unset, route carries all three | Section is byte-identical to the source (no ops recorded at all) |
| AC-4 | A Set op follows a Suppress op for the same community code | The Set wins: the attribute is emitted with the Set value |
| AC-5 | A Suppress op follows a Set op for the same community code | The Suppress wins: the attribute is absent |
| AC-6 | Suppress op on code 35 while the source carries OTC | OTC is preserved unchanged AND a warning names the refusal (RFC9234-5-6) |
| AC-7 | Suppress op on code 35 with no source OTC | No OTC attribute is emitted |
| AC-8 | Every existing transform pinned by `TestGoldenBytesUnchangedTier1` | Byte-identical output |
| AC-9 | A Suppress op whose `Buf` is NOT empty, on a community code | The attribute is dropped. The ACTION decides, never the buffer length |
| AC-10 | Suppress op on CLUSTER_LIST (code 10) | The attribute is dropped (Optional Non-transitive, no preservation clause) |
| AC-11 | Suppress op on CLUSTER_LIST alongside a Prepend op | Suppress wins, matching `aspathHandler` |
| AC-12 | Suppress op on ORIGINATOR_ID (code 9) with a source value present | Preserved unchanged (RFC4456-8-4) and the refusal is logged; with no source value, no attribute is created |
| AC-13 | A route carrying LOCAL_PREF, learned from an internal peer, relayed to an EXTERNAL peer | The forwarded UPDATE carries NO LOCAL_PREF (RFC4271-5.1.5-2), on both forward rails |
| AC-14 | The same route relayed to an INTERNAL peer | LOCAL_PREF is preserved byte-identical, and no operation is recorded, so the route stays on the zero-copy path |
| AC-15 | A route with NO LOCAL_PREF, relayed to an external peer | No operation is recorded. Nothing to strip costs nothing |
| AC-16 | An egress filter records `AttrModSet` on code 5 for an external destination | The strip runs after the filter pass and wins. The prohibition is not a policy a filter may override |
| AC-17 | Every egress rail's answer to "may this destination receive LOCAL_PREF" | Comes from ONE predicate, `localPrefAllowedTo`, which carries the RFC 3065 confederation exception |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `community { send none; }` on a peer and forwards a route with communities | config -> `PeerSettings.SendCommunity` -> `precomputeSendCommunity` -> `applyFactsSendCommunity` -> `buildModifiedPayload` -> `genericCommunityHandler` -> wire | `TestSendCommunitySuppressEmittedBytes` |
| 2 | Sets `community { send standard; }` and forwards a route with large + extended | same | `TestSendCommunitySuppressEmittedBytes` subset case |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendCommunitySuppressEmittedBytes` | `internal/component/bgp/reactor/forward_send_community_test.go` | AC-1, AC-2 on EMITTED BYTES via `buildModifiedPayload` with the registered handlers | pass |
| `TestSendCommunityAllKeepsZeroCopy` | same file | AC-3 | pass |
| `TestCommunitySetSuppressLastWins` | same file | AC-4, AC-5, AC-9 | pass |
| `TestClusterListAndOriginatorIDSuppress` | same file | AC-10, AC-11, AC-12 | pass |
| `TestOTCSuppressRefusedAndPreserved` | `internal/component/bgp/plugins/role/otc_test.go` | AC-6, AC-7 | pass |
| `TestForwardLocalPrefStrippedToExternalPeer` | `internal/component/bgp/reactor/forward_local_pref_test.go` | AC-13, AC-14, AC-15 on EMITTED BYTES | pass; mutation-verified |
| `TestForwardLocalPrefStripBeatsAFilterSet` | same file | AC-16 | pass; mutation-verified |
| `TestLocalPrefAllowedToIsTheOnlyAnswer` | same file | AC-17 | pass |
| `TestPayloadHasLocalPref` | same file | the presence check reads the attribute SECTION, not the payload bytes | pass |
| `TestGoldenBytesUnchangedTier1` (existing) | `internal/component/bgp/reactor/forward_modify_failure_test.go` | AC-8 | pass, 12/12 unmoved |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A (no new numeric input; value sizes 4/8/12 are existing constants) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `send-community-suppress` | `test/plugin/send-community-suppress.ci` | One source, two destinations: the `send none` peer receives the route with NO community, the `send standard` peer still receives it. Same received UPDATE, two different wires | pass; mutation-verified at the socket |
| `local-pref-strip-ebgp` | `test/plugin/local-pref-strip-ebgp.ci` | An internal source announces a route carrying LOCAL_PREF 200; the external destination's wire is asserted byte for byte and holds none | pass; mutation-verified at the socket (`+ LOCAL_PREF: 200 (unexpected)`) |

The audit found NO pre-existing `.ci` covering `session { community { send } }`.
The only file naming it was `test/draft/plugin/wire-edit-fanout-dedup.ci` (owned
by `spec-wire-edit-5-fanout-dedup`), whose header records this defect as its
**Blocker 1**: "`session/community/send none` did not suppress the COMMUNITY
attribute for receiver-a -- the forwarded wire still carried 65001:100". That
blocker is now resolved: running the draft shows the forwarded UPDATE no longer
carries the attribute. Its remaining Blocker 2 is a `contains=` assertion-form
problem and is NOT touched here, because that spec is being closed concurrently.

The new test uses `community { send ... }` where `community-strip.ci` uses an
egress `community { strip }`, and asserts the SAME stripped wire bytes. Two
config surfaces converging on one output is a stronger check than either alone.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `52-send-community-suppress-frr` | `test/interop/scenarios/` | FRR + BIRD | The `send none` peer sees no community of any type; the `send standard` peer sees the standard one and neither other | pass; mutation-verified |
| `54-local-pref-strip-gobgp` | `test/interop/scenarios/` | GoBGP + FRR | A relayed route reaches an external peer with no LOCAL_PREF, and with its AS_PATH and NEXT_HOP intact | pass; mutation-verified (`[{Origin: i} {LocalPref: 200}]` at GoBGP without the fix) |

**GoBGP is the witness and FRR cannot be, which is a property of the RFC rather
than of FRR.** Section 5.1.5 also says a receiver MUST ignore a LOCAL_PREF that
arrives over eBGP (RFC4271-5.1.5-3), and FRR implements that by skipping the
attribute during parse, so a leak is invisible in its RIB, its JSON and its
debug log. Measured 2026-08-04 with the strip removed: FRR showed nothing while
GoBGP showed `{LocalPref: 200}` for the same route from the same UPDATE. FRR
stays in the lab for the other half of the job, that a conformant peer accepts
the stripped UPDATE and installs the route.

**Two properties of the lab decide which rail is under test, and both were
measured rather than assumed.** A scenario that announces BEFORE its
destinations are established delivers through `bgp-rib`'s replay, which is
`peer_rib_routes.go` and was already correct: an earlier draft of scenario 54
did exactly that and stayed GREEN with the forward rail's strip removed. The
injector therefore waits for the destinations first. Separately, the first
UPDATE after a long quiet session is dropped with `BUG: ForwardUpdatesDirect:
msgID missing from cache` (`reactor_api_forward_batch.go`), reproduced at 50 s
and at 70 s of idle, so the scenario re-announces. That drop is a SEPARATE
defect, reported and not fixed here.

## Files to Modify
- `internal/component/bgp/filterapi/filterapi.go` (or `editset.go`) - export the
  last-Set-or-Suppress-wins fold so one definition serves all three handlers.
- `internal/component/bgp/reactor/filter_delta_handlers.go` - delegate to it.
- `internal/component/bgp/plugins/filter_community/handler.go` - Suppress branch.
- `internal/component/bgp/plugins/role/otc.go` - refuse-and-say-so branch.
- `internal/component/bgp/reactor/reactor_api_forward.go` - call it, over the
  export wire override when a policy chain produced one.
- `internal/component/bgp/reactor/forward_rs.go` - call it.
- `internal/component/bgp/reactor/reactor_api_batch.go`,
  `internal/component/bgp/reactor/peer_rib_routes.go`,
  `internal/component/bgp/reactor/reactor_wire.go` - ask `localPrefAllowedTo`
  instead of re-deriving `isIBGP`, so one site owns the confederation exception.
- `test/plugin/bgp-rs-fastpath-ebgp-shared.ci`,
  `test/plugin/remove-private-as-replace-peer.ci` - both described the discard
  marker as `C0FD0405010000` / "ATTR_DISCARD". The producer writes
  `attribute.AttrTombstone` = 252 = 0xFC, named "ATTR_TOMBSTONE". Comments only;
  no assertion or input changed.

## Files to Create
- `internal/component/bgp/reactor/forward_send_community_test.go` - emitted-byte test.
- `internal/component/bgp/reactor/forward_local_pref.go` - `localPrefAllowedTo`,
  `payloadHasLocalPref`, `modsTouchLocalPref`, `applyFactsLocalPref`. Its own
  file rather than a block inside `peer_forward_facts.go` beside its `applyFacts*`
  siblings: that file carried another session's uncommitted work when this landed,
  and a shared checkout cannot stage half a file.
- `internal/component/bgp/reactor/forward_local_pref_test.go` - emitted-byte test.
- `test/plugin/local-pref-strip-ebgp.ci` - the wire at the socket.
- `test/interop/scenarios/54-local-pref-strip-gobgp/` - the wire at a real peer.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | The `send` leaf-list already exists and parses correctly (ruled out in diagnosis) |
| YANG validation constraints | No | Unchanged |
| YANG custom validators | No | Unchanged |
| CLI commands/flags | No | No command surface change |
| CLI grammar | N-A | No command added |
| Editor autocomplete | No | Unchanged enum |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No new output |
| Env var registration | No | No `environment/` leaf |
| Doctor check | N-A | No new runtime dependency |
| Prometheus counters | No | The existing remove-buffer-refused counter is untouched |
| BGP family surface | N-A | No new SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Existing documented feature that did not work |
| 2 | Config syntax changed? | No | Unchanged |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Behavior fix inside an existing plugin |
| 6 | Has a user guide page? | audit | Check `docs/guide/` for a send-community claim to re-verify |
| 7 | Wire format changed? | No | The fix makes the wire match the documented config |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented/changed/newly proven? | audit | RFC9234-5-6 gains an explicit refusal path; check `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command/capability changed? | No | |
| 16 | Changed source file referenced by doc source anchors? | audit | Grep `docs/` for anchors on the four modified files |
| 17 | Existing docs show examples for this area? | audit | Verify any `send-community` example |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the handler
   - Tests: `TestSendCommunitySuppressEmittedBytes` written FIRST, asserting emitted bytes
   - Files: `internal/component/bgp/reactor/forward_send_community_test.go`
   - Verify: the test FAILS with the community attribute still present on the wire
2. **Phase: Suppress branch** -- one shared rule, three handlers
   - Tests: the above, plus `TestOTCSuppressRefusedAndPreserved`
   - Files: `filterapi`, `filter_delta_handlers.go`, `filter_community/handler.go`, `role/otc.go`
   - Verify: tests pass; goldens unchanged; mutation-verify both directions

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named assertion |
| Test drives production dispatch | The test obtains handlers from the REGISTRY, never by constructing a handler in the test body. This is the exact vacuity that hid the defect |
| Assertion target | The test asserts EMITTED BYTES, not recorded ops and not a handler return |
| Correctness | Last-Set-or-Suppress-wins is applied identically in all three handlers |
| Suppression stays its own action | No caller expresses suppression as an empty `AttrModSet` |
| Drop, not empty emit | A suppressed attribute plans a `Drop` slot; `ValueLen()==0` never reaches `Emit` |
| Header class | `EmitExtended` still used for every community attribute that IS emitted |
| RFC 9234 | The OTC path preserves a present OTC (RFC9234-5-6) and warns rather than silently discarding |
| Fail-closed guard | The OTC refusal SPEAKS (`ai/rules/evidence.md`): a guard that neither denies nor speaks does not exist |
| Goldens | `TestGoldenBytesUnchangedTier1` and its PooledBuffer variants unmoved |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Suppress branch in `genericCommunityHandler` | `grep -n AttrModSuppress internal/component/bgp/plugins/filter_community/handler.go` |
| Suppress branch in `otcAttrModHandler` | `grep -n Suppress internal/component/bgp/plugins/role/otc.go` |
| Emitted-byte test | `go test -run TestSendCommunitySuppressEmittedBytes ./internal/component/bgp/reactor/` |
| Goldens unmoved | `go test -run TestGoldenBytesUnchanged ./internal/component/bgp/reactor/` |
| No data race | `make ze-race-reactor` |
| Package green | `make ze-test-bgp` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information disclosure | This IS the security-relevant half: internal communities (route-server tags, customer policy signals) leak to peers configured to receive none. The fix stops that leak |
| Fail-closed | An unrecognized op action must never re-emit an attribute the operator asked to suppress |
| Peer-controlled input | The source attribute value is peer-controlled; the Drop path plans no fragment over it, so no new bound is introduced |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test passes before the fix | The test is vacuous. Re-target it at emitted bytes |
| Golden moves | STOP and report (task instruction) |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

- The defect survived because both existing tests were shaped to avoid it:
  `TestGenericAttrSetHandler_Suppress` builds `genericAttrSetHandler` for code 8
  INSIDE the test, a handler production never dispatches for that code, and
  `TestPrecomputeSendCommunity` asserts recorded ops rather than rebuilt bytes.
  A test that constructs its own handler proves the handler, never the dispatch.
- Five handlers implemented "last op wins" independently and FOUR of them forgot
  AttrModSuppress: the three community codes, OTC, CLUSTER_LIST and
  ORIGINATOR_ID. Only `genericAttrSetHandler` and `aspathHandler` had it. The
  rule belongs in the shared leaf package, once.
- The sibling audit (`ai/rules/architecture.md`) is what found the last
  two. Grepping `RegisterAttrModHandler` finds only 4 production registrations;
  the other handlers are installed by `attrModHandlersWithDefaults` and are
  invisible to that grep. Audit the INSTALLER, not just the registrations.
- `mpReachNextHopHandler` ignores Suppress DELIBERATELY and says so in its doc
  comment: suppressing MP_REACH_NLRI would strip the route, which is a withdraw
  and is expressed through `ModAccumulator.SetWithdraw`. Left alone.
- The first mutation probe was masked: disabling the Suppress branch alone still
  passed, because a Suppress op carries a nil `Buf` and fell into the handler's
  separate "empty Set value" drop. That coincidence would have re-established
  "empty Set means suppress" as the working spelling. AC-9 exists to kill it: a
  Suppress carrying bytes is the one input only the action check answers, and
  without the branch those bytes are emitted as the community value.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add a Suppress branch to the community handler | Express suppression as an empty `AttrModSet` in the caller | An empty Set re-overloads the exact ambiguity `AttrModSuppress` exists to remove, and leaves the next producer to rediscover the fail-open |
| OTC: preserve and WARN | Honor Suppress and drop OTC | RFC9234-5-6 is a gated MUST: "Once the OTC Attribute has been set, it MUST be preserved unchanged." Honoring Suppress would violate it. Warning closes the silence without breaking conformance |
| One shared fold in `filterapi` | Copy `lastSetOrSuppress` into each plugin | Three copies is how two of them came to disagree |

## Known Limitations
- The OTC Suppress path is latent: no production producer emits it today (A-1).
  Its test drives the handler through the same registry dispatch, so it proves
  the branch rather than the plumbing.

## RFC Documentation (Scope: protocol)

`// RFC 9234 Section 5: "Once the OTC Attribute has been set, it MUST be
preserved unchanged."` above the OTC refusal branch.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken
- [ ] Deferral shard resolved (none opened)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A, recorded above)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
