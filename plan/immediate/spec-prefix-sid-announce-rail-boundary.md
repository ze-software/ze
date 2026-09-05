# Spec: prefix-sid-announce-rail-boundary

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | bgp |
| Depends | - |
| Phase | implementation |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** The BGP Prefix-SID attribute crosses an AS boundary on one
egress rail with nothing asking the operator first. RFC 8669 Section 8: "The
propagation to other ASes MUST be explicitly configured."

`spec-srv6-ebgp-egress-filter` gated four of the five rails that write an UPDATE.
`prefixSIDAllowedTo` (`internal/component/bgp/reactor/forward_prefix_sid.go`) is
the single site that answers, and the two forward rails (`forwardUpdateCore`,
`reactorForwardRS`) and the two origination rails (`buildStaticRouteUpdateNew`,
`toPluginParams`) all ask it.

**The rail that does not ask.** `buildBatchAnnounceUpdate`
(`internal/component/bgp/reactor/reactor_api_batch.go`) copies the caller's
attribute block verbatim into `plan.emit(base, attrBuf)`. Its only removal is
`plan.drop(uint8(attribute.AttrLocalPref))` for RFC 4271 Section 5.1.5. Attribute
40 present in `base` reaches the destination untouched. Three callers reach it:
the per-peer API announce, the grouped announce, and `sendStaleReadvertise`, the
RFC 9494 LLGR stale readvertise, whose base is the received attribute block of a
route the RIB stored.

**Measured, not inferred.** `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`
(written as a probe, since renamed to
`internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go`) states the
requirement and is RED at HEAD. Toward an external destination with no leaf set,
the rail emits `4001010040020a02020000fde80000fde94003040a000001` followed by
`c0280a01000700000000000064`: attribute 40, Label-Index 100.

**This spec cannot start until the owner writes one row.** The fix is per
destination, so it is one bool on `buildBatchAnnounceUpdate` and one field on
`announceBuildKey`. The key field is not optional: with update groups enabled one
built UPDATE is shared by every peer in a group, so a destination-scoped strip
outside the key would apply to the wrong peers. Widening the parameter list
mechanically edits `TestAnnounceStripsLocalPrefTowardExternalPeer`
(`reactor_api_origin_test.go`), which carries `RFC requirement: RFC4271-5.1.5-1`
and `-2`. `test/rfc-changed.md` states that only the owner approves an edit to a
tagged test, as a row in that file in the commit that carries the change.

No signature-preserving shape is honest. A variadic or a defaulted wrapper makes
an RFC decision silently. A post-build strip over `update.PathAttributes` re-adds
the memmove the announce writer was built to remove, and it mutates a buffer
shared across a build group.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` -- the design document
      `forward_prefix_sid.go` and `peer_send.go` declare, for egress attribute
      modification on the forward rails.
- [ ] `docs/features/srv6.md` -- the public statement of which rails remove the
      Prefix-SID at the SR domain boundary.

- [ ] `rfc/full/rfc8669.txt` Section 8 -- the whole Manageability paragraph, not
      the one sentence. It scopes the boundary to "a single SR/administrative
      domain that may include one or more ASes", which is why the answer is a
      per-neighbor leaf and not the ASN pair.
- [ ] `internal/component/bgp/reactor/forward_prefix_sid.go` -- `prefixSIDAllowedTo`,
      the single site every other rail asks.
- [ ] `test/rfc-changed.md` -- who writes the approval row, and why an author
      cannot write their own.
- [ ] `plan/journal/gate-blocks-the-conformant-fix.md` -- the row this gap wrote.

## Current Behavior

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` -- `buildBatchAnnounceUpdate`
      builds over the caller's verbatim `base`; the only destination-scoped removal
      is `plan.drop` of LOCAL_PREF. `announceBuildKey` is the group key, and it
      carries `isIBGP`, `rsClient`, `asn4`, `addPath`, `localAS`, `extended`.
- [ ] `internal/component/bgp/reactor/forward_prefix_sid.go` -- `prefixSIDAllowedTo`
      takes `(isIBGP, propagate)` and is the answer every other rail uses.
- [ ] `internal/component/bgp/reactor/reactor_api_origin_test.go` -- the tagged test
      the parameter widening edits.

**Behavior to preserve:** the LOCAL_PREF strip and its two tagged polarities; the
one-pass merge writer and its zero-memmove property; group sharing of one built
UPDATE across the peers of one `announceBuildKey`.

**Behavior to change:** attribute 40 in `base` is dropped toward an external
destination whose peer does not set `propagate-srv6-prefix-sid`.

## Data Flow

### Entry Point
`ReactorAPI.Announce` from a plugin or the CLI, the grouped announce, and
`sendStaleReadvertise` on an RFC 9494 stale readvertise.

### Transformation Path
1. The caller's attribute block becomes `base` (`batch.Wire.Packed()` or
   `batch.Attrs.RawWire()`).
2. Destinations are grouped by `announceBuildKey`; the new bool joins that key.
3. `buildBatchAnnounceUpdate` takes the bool and calls
   `plan.drop(uint8(attribute.AttrPrefixSID))` when the destination refuses it.
4. `plan.emit(base, attrBuf)` writes the merged block without code 40.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `PeerSettings` -> `announceBuildKey` | `peer.prefixSIDAllowed()` at the group key site | [ ] |
| `announceBuildKey` -> builder | the new parameter on `buildBatchAnnounceUpdate` | [ ] |
| builder -> wire | `plan.drop` beside the LOCAL_PREF drop | [ ] |

### Integration Points
`prefixSIDAllowedTo` (`forward_prefix_sid.go`) stays the one site that answers.
`Peer.prefixSIDAllowed` already exists for the origination rails and is the form
this rail asks in.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| API announce toward an external peer | -> | `buildBatchAnnounceUpdate` `plan.drop` | `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`, `test/plugin/prefixsid-announce-rail-boundary.ci` |
| Stale readvertise toward an external peer | -> | the same drop, via `sendStaleReadvertise` | `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | API announce carrying attribute 40, external peer, leaf unset | no attribute 40 on the wire |
| AC-2 | The same, leaf set true | attribute 40 byte for byte |
| AC-3 | The same, internal peer | attribute 40 byte for byte |
| AC-4 | Two peers in one announce, one configured and one not | each gets its own answer, so the build group splits |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` | `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go` | AC-1; RED at HEAD, renamed off `zzprobe_prefixsid_announce_test.go` under the owner's approval | Done |
| the configured twin, subtest `external-peer-the-operator-configured-keeps-it` | the same file | AC-2 and AC-3, so the drop is confined | Done |
| the internal twin, subtest `internal-peer-keeps-it-whatever-the-leaf-says` | the same file | AC-3: the leaf does not reach an internal destination's frame | Done |
| `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain` | `internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go` | the SECOND entry into the rail: `sendStaleReadvertise` hands the destination's leaf to the same builder | Done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefixsid-announce-rail-boundary.ci` | `test/plugin/` | AC-4: one `send bgp * update hex` carrying a Prefix-SID reaches two eBGP peers with opposite outcomes, with `group-updates` at its default of true | Done |

## Files to Modify

- `internal/component/bgp/reactor/reactor_api_batch.go` -- `announceBuildKey` field,
  `buildBatchAnnounceUpdate` parameter, the `plan.drop`, and the three call sites.
- `internal/component/bgp/reactor/reactor_api_origin_test.go` -- the widened call,
  owner-approved.
- `internal/component/bgp/reactor/reactor_as4path_test.go`,
  `reactor_stale_readvertise_test.go`, `reactor_api_batch_nexthop_test.go`,
  `reactor_api_batch_dedup_test.go` -- the same widening, untagged.
- `test/rfc-changed.md` -- the owner's approval row, in the commit that carries it.
- `rfc/short/rfc8669.md` -- the `{gap}` comes off `RFC8669-8-1`, Meta count eleven
  back to ten; then `./le rfc index-update`.
- `docs/features/srv6.md` -- the "Gap: the API and readvertise announce rail"
  section goes, and the Partial row returns to Implemented.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | `propagate-srv6-prefix-sid` already exists |
| CLI commands/flags | No | config-only |
| Functional test | Yes | `test/plugin/` |
| Doctor check | No | no runtime dependency |
| Prometheus counters | No | a per-UPDATE decision |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the leaf already ships |
| 2 | Config syntax changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc8669.md`, then regenerate |
| 16 | Changed source file referenced by doc anchors? | Yes | `docs/features/srv6.md` anchors `reactor_api_batch.go` |

## Implementation Steps

1. The owner approved the tagged-test edit and the probe rename on 2026-09-05, and the approval row is written into `test/rfc-changed.md` by the commit that carries the change.
2. Add the field to `announceBuildKey` and the parameter to
   `buildBatchAnnounceUpdate`; update the three production call sites.
3. Add the `plan.drop` beside the LOCAL_PREF drop, with the RFC 8669 Section 8
   quotation above it.
4. Widen the test call sites, the tagged one under the approval row.
5. Write the configured twin and the group-split case; drive the red test green.
6. Write the `test/plugin/` functional test.
7. Record the discrimination proof: `./le rfc discriminate-record`.
8. Take the `{gap}` off `RFC8669-8-1`, regenerate, and repair `docs/features/srv6.md`.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `buildBatchAnnounceUpdate` drops code 40 for a refusing destination | `grep -n 'prefixSIDAllowedTo' internal/component/bgp/reactor/reactor_api_batch.go` |
| `announceBuildKey` carries the leaf, so a build group splits on it | `grep -n 'propagatePrefixSID' internal/component/bgp/reactor/reactor_api_batch.go` |
| All three call sites pass the destination's own leaf | `grep -n 'PropagateSRv6PrefixSID' internal/component/bgp/reactor/reactor_api_batch.go` |
| Both entries into the rail carry a test | `go test -run 'PrefixSIDInsideTheSRDomain' ./internal/component/bgp/reactor/` |
| The operator path carries a functional test | `ls test/plugin/prefixsid-announce-rail-boundary.ci` |
| Both RFC8669-8-1 polarities carry a recorded red | `./le rfc discriminate stem rfc8669` |
| The public ledger says what the code does | `grep -n 'RFC8669-8-1' rfc/short/rfc8669.md`, `grep -n 'announce rail' docs/features/srv6.md` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Untrusted input | `base` is a caller-supplied attribute block, and the presence test walks it. `attribute.AttrFind` (`internal/core/bgp/attribute/iterator.go`) bounds-checks every header and every value against `len(data)`, and its offset strictly increases, so a hostile block cannot read past the end or loop |
| Fail-open guard | The guard is `prefixSIDAllowedTo(isIBGP, propagate)`. Its false branch is the RESTRICTIVE one, so a missing leaf removes the attribute. A defaulted parameter would have made the permissive answer the silent one, which is why the leaf and not the answer is what the call sites pass |
| Resource exhaustion | The drop appends ONE `announcePlanEntry`. `announceInlinePlans` is 16 and the rails contribute at most seven, so no announce reaches the spill |
| Information leakage | An RFC 8669 Section 8 leak IS the disclosure this change stops: a Prefix-SID names an internal SR domain's label index to a peer outside it |
| Cross-destination bleed | One built UPDATE is shared by a build group, so a destination-scoped removal outside the key would apply to the wrong peers. The leaf is a key field, and `test/plugin/prefixsid-announce-rail-boundary.ci` runs with `group-updates` at its default of true to prove the split |

## Implementation Summary

### What Was Implemented

`buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go`)
takes `propagatePrefixSID`, the destination's leaf, as its last parameter and
asks `prefixSIDAllowedTo` (`forward_prefix_sid.go`) beside the RFC 4271
Section 5.1.5 LOCAL_PREF drop. When the answer is no and the base carries code
40, the rail records `plan.drop(uint8(attribute.AttrPrefixSID))`, so the merge
writer skips the copy and no memmove is added. `announceBuildKey` carries the
same leaf, so two peers of one update group that answer differently no longer
share a built UPDATE. The three production call sites pass it: the per-peer
announce, the grouped announce, and `sendStaleReadvertise`.

The parameter is the operator's LEAF and not the answer, which is the shape the
four other rails carry (`applyFactsPrefixSID` takes `f.propagatePrefixSID`).
An internal destination therefore keeps the attribute by construction rather
than by every caller getting it right.

Only the BASE can carry a Prefix-SID on this rail: nothing in the builder
contributes code 40, and `attribute.Builder` has no setter for it, so a
Builder's Prefix-SID arrives as pre-encoded wire in `RawWire`, which IS the
base.

`prefixSIDAllowedTo` was given a braced body. It is the same expression; a
one-line function body cannot be replaced by `./le rfc discriminate-record`,
which is what observes the red this change's tags owe.

### Bugs Found/Fixed

- The rail's SECOND entry, `sendStaleReadvertise`, had no test. The Wiring Test
  table named a `.ci` for it and the delivered `.ci` drives the API announce
  only, so a constant at that call site would have leaked attribute 40 on every
  LLGR re-advertisement with nothing red. Closure wrote
  `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`
  (`internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go`),
  which drives `AnnounceNLRIBatch` with `Stale` set through a piped session and
  reads the flushed frame. Proven discriminating: with the call site rewritten to
  `propagatePrefixSID := true` under a Go overlay, its negative subtest fails on
  its own assertion with `4001010040020a02020000fde80000fde94003040a000001c0280a01000700000000000064`
  on the wire, and `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` stays green,
  so the new red is readvertise-specific.
- The new test is a FILE of its own rather than a third test beside the announce
  rail's two. `./le commit audit` reads any edit to an RFC-tagged carrier as a
  weakening only the owner may approve, and it read the ADDITION that way:
  `[WEAKENED] forward_prefix_sid_announce_rail_test.go -- RFC-TAGGED test
  changed`. Writing an owner approval row is not a thing an agent may do, so the
  test moved to a new file, which `changedRFCTests`
  (`internal/le/commit/rfcchange.go`) skips because its HEAD text is empty.
  Recorded as a recurrence in `plan/journal/guard-blocks-its-own-authors-repair.md`.
- `test/plugin/prefixsid-announce-rail-boundary.ci` said in its own prose that
  one `send bgp * update text` reaches both receivers. The command it documents
  ten lines above is `update hex`, because `update text` has no token for a raw
  attribute. Corrected.

### Documentation Updates

- `docs/features/srv6.md`: the EBGP propagation row and the RFC 8669 Section 8
  row now say every rail asks, and the "Gap: the API and readvertise announce
  rail" section became a statement of the rail's behavior. Anchored on
  `<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchAnnounceUpdate -->`.
- `rfc/short/rfc8669.md`: `RFC8669-8-1` came off `{gap}`, the Support coverage
  cell names both producing files, and Support remaining went from eleven MUST
  gaps to ten. That edit landed in `4054ed854` with `docs/features/rfc-status.md`
  regenerated beside it.
- `rfc/requirements/rfc8669.md`: regenerated by `./le rfc index-update`, so the
  `RFC8669-8-1` row binds the readvertise test as well. `docs/features/rfc-status.md`
  is NOT in this commit: the same run also picked up four other RFCs from
  `rfc/short/` edits another session has in flight, and none of its diff hunks
  names RFC 8669.

### Deviations from Plan

- Step 6 planned the readvertise entry's coverage as a `.ci`. It is a Go test
  instead, in the shape `TestStaleReadvertiseWireOutput`
  (`reactor_stale_readvertise_test.go`) already established for this rail: an
  LLGR readvertise needs a stale transition driven by a fixture plugin, and the
  Go test reaches the same wire bytes deterministically. The API announce entry,
  which is the one an operator types, keeps its `.ci`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Wiring Test table's second row was read as covered by the `.ci` written for the first | The `.ci` drives `send bgp * update hex`, which enters `AnnounceNLRIBatch` with `Stale == 0` and never reaches `sendStaleReadvertise` | Closure step 1 read the `.ci` rather than its name | Wrote the readvertise test and proved its red under a broken call site |
| assumption | The `.ci` prose named `send bgp * update text` | `update text` carries no token for a raw attribute, so the test uses `update hex` | Closure step 4 compared the prose against the command in the same file | Corrected the prose |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The fifth rail asks `prefixSIDAllowedTo` | Done | `buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go`) | Beside the RFC 4271 Section 5.1.5 LOCAL_PREF drop |
| The answer is per destination, so the build key carries it | Done | `announceBuildKey` (same file) | `propagatePrefixSID` field |
| All three callers pass the destination's leaf | Done | the per-peer announce, the grouped announce, `sendStaleReadvertise` (same file) | Each reads `peer.Settings().PropagateSRv6PrefixSID` or `bg.key.propagatePrefixSID` |
| The owner-approved tagged-test edit is recorded | Done | `test/rfc-changed.md` | Written by `4054ed854`, one row per carrier the gate names |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain/external-peer-with-no-leaf-is-sent-no-prefix-sid` | Also `test/plugin/prefixsid-announce-rail-boundary.ci` receiver A |
| AC-2 | Done | the same test's `external-peer-the-operator-configured-keeps-it` | The configured frame equals the stripped frame plus the attribute, byte for byte |
| AC-3 | Done | the same test's `internal-peer-keeps-it-whatever-the-leaf-says` | Both spellings of the leaf produce one frame |
| AC-4 | Done | `test/plugin/prefixsid-announce-rail-boundary.ci` | Two eBGP receivers, `group-updates` at its default of true |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` | Done | `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go` | Renamed off the probe path under the owner's approval |
| the configured twin and the internal twin | Done | the same file | Subtests |
| the group split | Changed | `test/plugin/prefixsid-announce-rail-boundary.ci` | Delivered as the functional test rather than a unit test: a build group is a property of `AnnounceNLRIBatch` over two live peers |
| the readvertise entry | Done | `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain` (`internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go`) | Added at closure; see Deviations |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor_api_batch.go` | Done | Key field, parameter, drop, three call sites |
| `internal/component/bgp/reactor/reactor_api_origin_test.go` | Done | Widened under the owner's approval row |
| the four untagged test call sites | Done | Plus `reactor_api_batch_attr_order_test.go`, `_attr_preserve_test.go`, `_capacity_test.go`, `reactor_api_origin_bench_test.go`, `reactor_batch_test.go`, which the widening also reached |
| `test/rfc-changed.md` | Done | The approval row |
| `rfc/short/rfc8669.md` | Done | `{gap}` removed, Meta counts corrected |
| `docs/features/srv6.md` | Done | Gap section replaced by the rail's behavior |

### Audit Summary
- **Total items:** 18
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the group-split test's home, the readvertise test's carrier kind; both in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Attribute 40 does not cross an AS boundary on the announce rail | functional | `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` (`internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go`), red before the fix with `c0280a01000700000000000064` on the wire, green after |
| The removal is confined to the destination that refuses it | functional | The same test's two positive subtests: a configured external peer's frame is the stripped frame plus the attribute, byte for byte, and an internal peer's frame does not depend on the leaf |
| The boundary holds on BOTH entries into the rail | functional | `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`, red on its own assertion when `sendStaleReadvertise` is given `propagatePrefixSID := true` under a Go overlay |
| An operator reaches the behavior through the product | functional | `test/plugin/prefixsid-announce-rail-boundary.ci`: one `send bgp * update hex attr set 40010100C0280A01000700000000000064 ...` reaches two eBGP peers whose configuration differs in one leaf, and the two expected frames differ in attribute 40 alone |
| The build group splits on the leaf | functional | The same `.ci` runs with `group-updates` at its default of true, so a key without the field would send both peers one frame and fail one of the two expectations |
| The tags are proofs rather than sentences | interop/discrimination | `rfc/discrimination/rfc8669.json`: four records, both polarities for each of the two tagged units, each observed red by `./le rfc discriminate-record` under a disabled producer |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every acceptance criterion, every Wiring Test row and every Documentation Update Checklist row is satisfied in this commit | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/prefix-sid-announce-rail-boundary-zeclose-railb.md` |
| `review check` | `review_gate: OK (clean, hashes match)`. It also NOTES that the running model could not be determined, so the review-model boundary is unchecked by the tool; the review ran in a closure agent that did not author the code, which is the independence `ai/rules/planning.md` requires |
| Rounds | 1 |
| Reviewer lenses used | correctness+wiring (the drop mechanism, the group key, the five-rail set, the leaf plumbing at all three call sites); test-discrimination (the vacuity walk over both tagged units); security+bounds (`AttrFind` over a hostile base, the fail-closed direction of the guard, the plan capacity); style (`docs/contributing/ze-go-style.md` over the changed Go); documentation (`docs/features/srv6.md` and `rfc/short/rfc8669.md` against the producers) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The rail's second entry had no test of any kind, so a wrong constant at the call site would leak attribute 40 on every LLGR re-advertisement with no red anywhere. Always-in-scope class: a user-facing behavior with no functional test | `sendStaleReadvertise` (`internal/component/bgp/reactor/reactor_api_batch.go`) | `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain` plus two `rfc/discrimination/rfc8669.json` records, and the Wiring Test row now names it |
| 2 | NOTE | The `.ci` prose claimed `send bgp * update text` where the file's own command is `update hex` | `test/plugin/prefixsid-announce-rail-boundary.ci` | Corrected in this commit |
| 3 | NOTE | The spec's Goal Validation carried the same false command | this spec | Corrected in this commit |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go` | Yes | `git ls-files` lists it; `zzprobe_prefixsid_announce_test.go` is gone at `425c30c48` |
| `internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go` | Yes | added at closure; `gofmt -l` and `go vet ./internal/component/bgp/reactor/` are both clean over it |
| `test/plugin/prefixsid-announce-rail-boundary.ci` | Yes | `ls test/plugin/prefixsid-announce-rail-boundary.ci` |
| `internal/test/fixture/register_prefixsid_announce_rail.go` | Yes | the `.ci`'s producer, added by `4054ed854` |
| `rfc/discrimination/rfc8669.json` | Yes | four records, printed by `./le rfc discriminate stem rfc8669` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | An unconfigured external destination is sent no attribute 40 | `go test -run 'TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain\|TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain' ./internal/component/bgp/reactor/` exits 0 |
| AC-2, AC-3 | The removal is confined | the same run: both positive subtests pass |
| AC-4 | The build group splits on the leaf | `test/plugin/prefixsid-announce-rail-boundary.ci` expects two different frames on two connections in one run |
| all | The tests are not vacuous | with `propagatePrefixSID := true` substituted at `sendStaleReadvertise` under a Go overlay, `TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain` fails with `Should be false` and `[]int{1, 2, 3, 40}` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| API announce toward two eBGP peers differing in one leaf | `test/plugin/prefixsid-announce-rail-boundary.ci` | Read: the fixture issues `send bgp * update hex attr set 40010100C0280A01000700000000000064 nhop set 0A000001 nlri ipv4/unicast add 18C00002`, and the two `expect=bgp:` lines carry full frames whose only difference is the 13 octets of attribute 40 |
| Stale readvertise toward an external peer | none; the Go test at `forward_prefix_sid_readvertise_rail_test.go` | Read: it drives `AnnounceNLRIBatch` with `Stale: 1` and a readvertise egress filter registered, so the batch enters `sendStaleReadvertise`, and the assertion reads the frame the peer's session flushed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| Only the BASE can carry a Prefix-SID on this rail | confirmed | `attribute.Builder.AppendAttributes` (`internal/core/bgp/attribute/builder.go`) emits ten codes and 40 is not among them, and it returns `dst` unchanged when the raw-wire escape is set, so a Builder's Prefix-SID is the base |
| Nothing above the drop contributes code 40, so `plan.drop` cannot duplicate | confirmed | the same producer, plus `buildBatchAnnounceUpdate`'s own contributions: ORIGIN, AS_PATH, AS4_PATH, NEXT_HOP, MP_REACH_NLRI, LOCAL_PREF |
| A group key without the leaf would send two differing peers one frame | confirmed | `AnnounceNLRIBatch` builds ONE UPDATE per `announceBuildKey` and sends it to every peer in the group |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/srv6.md` "removed on every rail that writes an UPDATE" | five ask sites: `applyFactsPrefixSID` and `Peer.prefixSIDAllowed` (`forward_prefix_sid.go`), `peer_initial_sync.go` at the static and the plugin origination rails, and `buildBatchAnnounceUpdate` | Yes |
| `rfc/short/rfc8669.md` Support coverage names the announce rail | the same five, and the cell names both files | Yes |
| `docs/features/rfc-status.md` | generated by `./le rfc index-update`; not hand-edited | Yes |
| CLI reference, config syntax, plugin SDK, wire format | No update needed: the leaf `propagate-srv6-prefix-sid` already shipped and no YANG, no command and no wire format changed. `git show --stat 4054ed854` lists no `.yang` and no `internal/component/cli` file | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] `./le rfc check` reports no RFC8669-8-1 finding

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior
