# Spec: prefix-sid-announce-rail-boundary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | bgp |
| Depends | - |
| Phase | - |
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
(`internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go`) states the
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
| API announce toward an external peer | -> | `buildBatchAnnounceUpdate` `plan.drop` | `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` |
| Stale readvertise toward an external peer | -> | the same drop, via `sendStaleReadvertise` | a `.ci` in `test/plugin/` |

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
| `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` | `internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go` | AC-1; RED at HEAD, and the file wants renaming once the deletion hook is answered | |
| (to write) the configured twin | the same file | AC-2 and AC-3, so the drop is confined | |
| (to write) the group split | the same file | AC-4: two destinations in one announce get opposite frames | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (to write) | `test/plugin/` | a plugin announce carrying a Prefix-SID reaches two eBGP peers with opposite outcomes | |

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

1. **Blocked until the owner writes the `test/rfc-changed.md` row.** Do not start.
2. Add the field to `announceBuildKey` and the parameter to
   `buildBatchAnnounceUpdate`; update the three production call sites.
3. Add the `plan.drop` beside the LOCAL_PREF drop, with the RFC 8669 Section 8
   quotation above it.
4. Widen the test call sites, the tagged one under the approval row.
5. Write the configured twin and the group-split case; drive the red test green.
6. Write the `test/plugin/` functional test.
7. Record the discrimination proof: `./le rfc discriminate-record`.
8. Take the `{gap}` off `RFC8669-8-1`, regenerate, and repair `docs/features/srv6.md`.

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
