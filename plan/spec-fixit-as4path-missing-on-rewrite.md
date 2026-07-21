# Spec: fixit-as4path-missing-on-rewrite

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
3. `rfc/short/rfc6793.md` - the governing RFC (Section 4.2.2 MUSTs)
4. `internal/component/bgp/wireu/aspath_rewrite.go`, `aspath_transcode.go`, `aspath_as4.go`

## Task

`RewriteASPath` / `RewriteASPathDual` substitute AS_TRANS (23456) for a non-mappable
(>65535) ASN when encoding AS_PATH for a peer that did not negotiate the four-octet AS
capability, but never emit the AS4_PATH attribute that RFC 6793 Section 4.2.2 MUSTs
alongside it. The real ASN is then irrecoverable: the receiver sees AS_TRANS and has
nothing to reconstruct from.

Blast radius is the **normal eBGP forward path**, not a corner case. Fix at the source,
sharing the AS4_PATH rule with the sibling that already implements it correctly.

## Required Reading

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6793.md` - the governing RFC
  → Constraint: Section 4.2.2 "The NEW BGP speaker MUST also send the AS path information
    in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case
    where all of the AS path information is composed of mappable four-octet AS numbers
    only." (`rfc/short/rfc6793.md:267`)
  → Constraint: Section 4.2.2 "In this case, the NEW BGP speaker MUST NOT send the
    AS4_PATH attribute." — the MUST NOT half (`rfc/short/rfc6793.md:269`)
  → Constraint: Section 4.1 "The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be
    carried in an UPDATE message between NEW BGP speakers." (`rfc/short/rfc6793.md:263`)
  → Constraint: Section 4.2.2 confed segments MUST be excluded from AS4_PATH
    (`rfc/short/rfc6793.md:271`)
  → Decision: the RFC's own "Generating UPDATE to OLD Speaker" algorithm
    (`rfc/short/rfc6793.md:343-383`) sets `has_non_mappable` **only inside the non-confed
    branch**. The mappability scan must therefore skip confed segments.
  → Constraint: Section 4.2.3 reconstruction is driven by
    `count(AS_PATH) - count(AS4_PATH)`; anything prepended to AS_PATH must also be
    prepended to a forwarded AS4_PATH or the delta shifts and our hop reconstructs as
    AS_TRANS (`rfc/short/rfc6793.md:191-201`).

### Rules
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`
  → Constraint: this is the hot forward path. `WriteTo(buf, off) int`, no `append`, no
    `make`. A fix that allocates on the fast path is not a fix.
- [ ] `ai/rules/rfc-compliance.md`
  → Constraint: every enforced MUST needs `// RFC NNNN Section X.Y: "quoted requirement"`
    directly above the enforcing code.

**Key insights:**
- The exact condition is narrow: **dstASN4 == false AND the outgoing (post-prepend) AS
  path contains an ASN > 65535 in a non-confederation segment.** Emitting AS4_PATH
  anywhere else is itself a defect (Section 4.1 MUST NOT / Section 4.2.2 MUST NOT).
- The sibling `TranscodeASPath` already implements this rule correctly. The correct shape
  is to share it, not to write a third copy.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - prepend + transcode for eBGP
  → Constraint: three write paths: `rewriteInsertASPath` (no AS_PATH),
    `tryDirectPrepend` (zero-alloc fast path, `srcASN4 == dstASN4` only),
    `rewritePrependASPathFull` (everything else). None emitted AS4_PATH.
- [ ] `internal/component/bgp/wireu/aspath_transcode.go` - transcode-only, RS-client
  → Decision: `:149` already gated AS4_PATH correctly and `:273-278` appended it. This is
    the reference implementation to share.
- [ ] `internal/core/bgp/attribute/aspath.go` - `ASPath`, `Prepend`, `WriteToWithASN4`
  → Constraint: `Prepend` (`:269`) never lets a segment exceed 255, so `AS4Path.Len`/
    `WriteTo` (which do not split) stay safe on a prepended path.
- [ ] `internal/core/bgp/attribute/as4.go` - `AS4Path`, `ParseAS4Path`
  → Constraint: `Len` (`:50`) and `WriteTo` (`:64`) already skip confed segments.

**Behavior to preserve:**
- `tryDirectPrepend` stays allocation-free for the ASN4→ASN4 and mappable-local-ASN cases.
- A received AS4_PATH is never dropped except when replaced by one we construct.
- All existing `aspath_rewrite_test.go` and `aspath_transcode_test.go` expectations.

**Behavior to change:**
- Emit AS4_PATH on the rewrite path exactly when RFC 6793 Section 4.2.2 requires it.

## Verified Chain (producers cited)

| # | Claim | Producer (file:line) | Verified |
|---|-------|---------------------|----------|
| 1 | AS_TRANS substituted on the fast path | `wireu/aspath_rewrite.go:292` (`asn = 23456`) | yes |
| 2 | AS_TRANS **also** substituted on the full path — the brief named only (1) | `core/bgp/attribute/aspath.go:193` (`as16 = 23456`), reached from `aspath_rewrite.go:396` `WriteToWithASN4(dst, n, dstASN4)` | yes (correction) |
| 3 | AS4_PATH never emitted by the rewrite | `grep -c AttrAS4Path aspath_rewrite.go` = 0; its AS4 hits are all `AttrAS4Aggregator` | yes |
| 4 | Sibling emits AS4_PATH correctly | `wireu/aspath_transcode.go:149,273-278` | yes |
| 5 | Normal eBGP forward path caller | `reactor/received_update.go:138` inside `EBGPWire`; the result is the wire sent, cached per `dstASN4` | yes |
| 6 | API forward path callers | `reactor/reactor_api_forward.go:293,295` **and `:365,367`** — the brief missed the second pair | yes (correction) |
| 7 | RS caller | `reactor/forward_rs.go:195` (dual) **and `:197`** — the brief named only `:197` | yes (correction) |
| 8 | No caller compensates downstream | The only other AS4_PATH producer in the forward direction is `reactor/filter_delta.go:585-593`, which **modifies an existing** AS4_PATH for remove-private-as and never creates one. RIB-side `plugins/rib/storage/attrparse.go:198` is a RIB-local canonicalization, not on the wire path. | yes |

**Conclusion:** the bug is real, the blast radius is as stated (and slightly wider: 7 call
sites, not 4), and nothing downstream compensates.

## Data Flow

### Entry Point
Received UPDATE wire bytes (`ReceivedUpdate.WireUpdate.Payload()`), or an API-originated
UPDATE body.

### Transformation Path
1. `ReceivedUpdate.EBGPWire(localASN, srcASN4, dstASN4)` — `received_update.go:115`
2. `wireu.RewriteASPath(dst.Buf, payload, ...)` — `received_update.go:138`
3. `rewriteASPathPrepend` → one of `rewriteInsertASPath` / `tryDirectPrepend` /
   `rewritePrependASPathFull`
4. Result wrapped as a `WireUpdate` and cached in the per-`dstASN4` slot; sent verbatim.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ wireu | `RewriteASPath(dst, payload, localASN, srcASN4, dstASN4) (int, error)` — caller-owned pooled buffer, byte slices only | [ ] |
| wireu ↔ attribute (core) | `ParseASPath` / `ParseAS4Path` in, `WriteToWithASN4` / `AS4Path.WriteTo(buf, off) int` out | [ ] |
| wireu ↔ wire (peer socket) | resulting `WireUpdate` bytes sent verbatim; no re-encode downstream | [ ] |

### Integration Points
- `attribute.AS4Path` (`internal/core/bgp/attribute/as4.go`) - existing type; `Len`/`WriteTo`
  already implement the Section 3 confed exclusion, so the new code adds no encoder
- `attribute.ASPath.Prepend` - reused to prepend into a received AS4_PATH (shared segment
  type `[]ASPathSegment` between `ASPath` and `AS4Path`)
- `wireu.TranscodeASPath` - now consumes the same rule owner, replacing its local copy

### Architectural Verification
- [ ] Zero-copy preserved: fast path still byte-shifts with no allocation
- [ ] No duplicated functionality: the AS4_PATH rule has exactly one owner
  (`wireu/aspath_as4.go`), used by both egress paths

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Nothing downstream of `RewriteASPath` adds AS4_PATH | grep for `AttrAS4Path` producers; `filter_delta.go:585` only modifies an existing one | bug would be narrower | grep + read of every hit | confirmed |
| A-2 | The fast path can be guarded with a comparison, no allocation | `tryDirectPrepend` cannot grow the attribute section; falling through to the full path is the only option | would need a hot-path allocation → STOP and report | `BenchmarkRewriteASPath` before/after | confirmed (0 allocs both) |
| A-3 | `ASPath.Prepend` never produces a >255 segment, so `AS4Path.WriteTo` (no split) is safe | `attribute/aspath.go:272` overflow branch | AS4_PATH count byte would wrap to 0 → malformed | read of producer + `TestRewriteASPath_FullSequence255` | confirmed |
| A-4 | A received AS4_PATH reaches the rewrite (not stripped at ingress) | forwarding is zero-copy over received bytes; no ingress 4.2.3 merge exists on the wire path | the merge case would be dead code | grep for reconstruction; only RIB-local `attrparse.go` exists | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Prepending to a received AS4_PATH shifts the 4.2.3 count delta | receiver reconstructs the wrong path | Prepend the same N ASNs to **both** AS_PATH and AS4_PATH so the delta is invariant; `TestRewriteASPath_AS4PathPrependedToReceivedAS4Path` |
| R-2 | Emitting AS4_PATH where forbidden (to a NEW speaker, or all-mappable) | interop complaints, wasted bytes | Both MUST NOTs have dedicated tests (AC-5, AC-6) |
| R-3 | An all-confed path with a non-mappable ASN yields a zero-length AS4_PATH (malformed per Section 6: length must be >= 6) | receiver discards the attribute | mappability scan skips confed segments, matching the RFC algorithm, so this cannot arise. Latent in the old sibling code; fixed by sharing. |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| eBGP forward of a received UPDATE to a non-ASN4 peer (`received_update.go:138`) | → | `wireu.RewriteASPath` → `as4PathForRewrite` | `TestRewriteASPath_AS4PathWireBytes` |
| local-as dual prepend to a non-ASN4 peer (`forward_rs.go:195`, `reactor_api_forward.go:293,365`) | → | `wireu.RewriteASPathDual` → `as4PathForRewrite` | `TestRewriteASPathDual_AS4Path` |
| locally-originated route (no AS_PATH) to a non-ASN4 peer | → | `rewriteInsertASPath` → `as4PathForPath` | `TestRewriteASPath_AS4PathMappabilityBoundary` |
| RS-client transcode (`forward_rs.go:371`) | → | `wireu.TranscodeASPath` → `as4PathForPath` | `TestTranscodeASPath_4to2_NonMappable` (pre-existing, now routed through the shared owner) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `RewriteASPath(localASN=200000, srcASN4=true, dstASN4=false)`, AS_PATH=[64512,64513] | Exact wire bytes: AS_PATH=`40 02 08 02 03 5BA0 FC00 FC01`, AS4_PATH=`C0 11 0E 02 03 00030D40 0000FC00 0000FC01` |
| AC-2 | Non-mappable ASN in the **received path** (not the local ASN), dstASN4=false | AS4_PATH carries the full post-prepend path with real 4-byte ASNs |
| AC-3 | `srcASN4=false, dstASN4=false`, non-mappable local ASN (fast-path input) | AS4_PATH emitted; fast path defers to the full path |
| AC-4 | Received AS4_PATH present, non-mappable local ASN, srcASN4=false | Local ASN prepended to the received AS4_PATH; exactly one AS4_PATH attribute emitted |
| AC-5 | All ASNs mappable, dstASN4=false | No AS4_PATH (MUST NOT) |
| AC-6 | dstASN4=true with a non-mappable ASN | No AS4_PATH (MUST NOT between NEW speakers) |
| AC-7 | AS_CONFED_SEQUENCE in the path, dstASN4=false | AS4_PATH excludes confed segments; AS_PATH keeps them |
| AC-8 | `RewriteASPathDual(primary=200000, secondary=65000, dstASN4=false)` | AS_PATH=[AS_TRANS,65000,...], AS4_PATH=[200000,65000,...] |
| AC-9 | localASN 65535 / 65536 / 4294967295, dstASN4=false | AS4_PATH emitted iff ASN > 65535 |
| AC-10 | Hot path (ASN4→ASN4, or mappable local ASN) | 0 allocs/op, unchanged from baseline |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRewriteASPath_AS4PathWireBytes` | `wireu/aspath_rewrite_test.go` | AC-1 (exact bytes, RFC-derived) | pass |
| `TestRewriteASPath_AS4PathFromNonMappablePathASN` | same | AC-2 | pass |
| `TestRewriteASPath_AS4PathSameEncodingASN2` | same | AC-3 | pass |
| `TestRewriteASPath_AS4PathPrependedToReceivedAS4Path` | same | AC-4 | pass |
| `TestRewriteASPath_NoAS4PathWhenAllMappable` | same | AC-5 | pass |
| `TestRewriteASPath_NoAS4PathToNewSpeaker` | same | AC-6 | pass |
| `TestRewriteASPath_AS4PathExcludesConfedSegments` | same | AC-7 | pass |
| `TestRewriteASPathDual_AS4Path` | same | AC-8 | pass |
| `TestRewriteASPath_AS4PathMappabilityBoundary` | same | AC-9 | pass |
| `FuzzRewriteASPath` | same (pre-existing) | no panic on arbitrary input | pass (2.5M execs, 45s) |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Mappable ASN | 0-65535 | 65535 | N/A | 65536 (first non-mappable), 4294967295 (max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-asn4-transcode` | `test/plugin/bgp-rs-asn4-transcode.ci` | RS transcode to a 2-byte client | **pre-existing; cannot pass — harness limit, see below** |

**Functional gate assessment (`ai/rules/functional-test-gate.md`).** This is changed BGP
wire behavior, so the table says a `.ci` is warranted. It is **blocked on test
infrastructure**, not skipped:

- `internal/test/peer/checker.go:400-411` consumes **one rule per message**: `matchRule`
  succeeds, splices that rule out of `c.messages`, and returns. Several `contains` rules
  scoped to the same `seq` can therefore never all match a single UPDATE.
- A meaningful AS4_PATH `.ci` needs at least two assertions on one UPDATE (AS_PATH
  re-encoded with AS_TRANS **and** AS4_PATH carrying the real ASN); asserting only one
  cannot distinguish the fix from the bug.
- `test/plugin/bgp-rs-asn4-transcode.ci` already hits this: three `contains` rules on
  `conn=2:seq=1`.
- **Not fixed here:** `internal/test/` is out of scope per the task, and the `.ci` belongs
  to a sibling agent.

**What a functional test would need:** either (a) multi-rule matching per message in
`checker.go` — a rule that matches does not consume the message, so every `contains` rule
for a `seq` is evaluated against it; or (b) a single full-hex `expect=bgp:` rule covering
the whole UPDATE, possible today but brittle. Recommend (a): it unblocks
`bgp-rs-asn4-transcode.ci` at the same time. Until then the wire bytes are gated by
`TestRewriteASPath_AS4PathWireBytes`, which asserts the **complete** output payload
byte-for-byte and is mutation-verified.

→ ADOPTED (2026-07-17): recommendation (a) — multi-rule matching per message in
`checker.go`, where a matched rule does not consume the message so every `contains`
rule for a `seq` is evaluated against the same UPDATE — is the chosen approach; it
also unblocks `bgp-rs-asn4-transcode.ci`. The harness change itself stays out of
scope for this spec (owned by a sibling agent, per above); this only records the
decision so an implementer need not re-litigate (a) vs (b). Thomas: override if wrong.

## Files to Modify
- `internal/component/bgp/wireu/aspath_rewrite.go` - emit AS4_PATH in the insert and full
  paths; guard the fast path
- `internal/component/bgp/wireu/aspath_transcode.go` - route through the shared owner;
  delete its local `hasNonMappableASN`
- `internal/component/bgp/wireu/aspath_rewrite_test.go` - AC-1..AC-9

## Files to Create
- `internal/component/bgp/wireu/aspath_as4.go` - single owner of the AS4_PATH rule

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | No | AS4_PATH wire format is already specified in `rfc/short/rfc6793.md:47-81`; ze emits the standard encoding, no new format |
| 9 | RFC behavior implemented, changed, or newly proven? | No edit needed | `docs/features/rfc-status.md:14` already claims "RFC 6793 ... Supported ... AS4_PATH ... No tracked gap in current source anchors". That claim was **false for the eBGP forward path** before this fix and is **true after it**. The row needs no change; the code was brought up to the claim |
| 16 | Changed source referenced by doc source anchors? | No | `grep -rn "aspath_rewrite\|aspath_transcode" docs/` returns nothing |

## Implementation Steps

### Implementation Phases

1. **Phase: Failing tests (TDD red)** — encode the RFC condition as wire-byte assertions
   - Tests: AC-1..AC-9 in `wireu/aspath_rewrite_test.go`
   - Verify: expectations derived from RFC 6793 / RFC 4271 **before** observing ze output
   - Observed: 6 red (`tmp/as4-red.log`); the two MUST NOT tests pass vacuously (the
     feature is absent, so it cannot over-emit) — expected and noted
2. **Phase: Shared rule owner** — extract the concern that drifted
   - Files: `wireu/aspath_as4.go` (new)
   - Verify: `hasNonMappableASN` has exactly one definition in the package
3. **Phase: Route the sibling through it** — prove the owner is shared, not parallel
   - Files: `wireu/aspath_transcode.go`
   - Verify: the 19 pre-existing transcode tests stay green
4. **Phase: Fix the rewrite** — fast-path guard, full path, insert path
   - Files: `wireu/aspath_rewrite.go`
   - Verify: red → green; existing rewrite tests unaffected
5. **Phase: Mutation-verify** — break it, confirm red, restore
   - Verify: `as4PathForPath` → nil reds 11 tests across both files; guard → `if false`
     reds exactly the 2 fast-path tests
6. **Phase: Hot-path proof** — benchmark against pristine HEAD
   - Verify: `BenchmarkRewriteASPath` allocs/op identical before and after
7. **Phase: Gates** — `make ze-lint-changed`, `make ze`, fuzz

### Failure Routing

| Failure | Route To |
|---------|----------|
| Hot-path allocation unavoidable | STOP. Report to Thomas — an RFC/performance tradeoff is his call, not the agent's |
| Existing transcode test reds after sharing | The shared rule diverged from the sibling's behavior — re-check the RFC condition, not the test |
| Functional `.ci` cannot assert two facts on one UPDATE | Harness limit (`checker.go:400-411`); report, do not fix (out of scope) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Share the AS4_PATH rule in a new `aspath_as4.go` used by both egress paths | (a) duplicate the transcode block into the rewrite; (b) merge `RewriteASPath` and `TranscodeASPath` into one core with a `prepend []uint32` parameter | (a) is what produced today's private-ASN leak (`egress_inject_filter.go` vs `filter_ordered.go`) — a third copy would drift the same way. (b) is the right altitude in the abstract but rewrites two 300-line functions with different bounds-checking and a fast path, for a bug fix: unjustified regression risk. Extracting the *concern that drifted* (the rule) rather than the *control flow* gets the anti-drift benefit at a fraction of the risk. Mutation-verified: disabling `as4PathForPath` reds 11 tests across **both** files, proving one owner |
| Fast path defers to the full path for a non-mappable prepend | Grow the attribute section inside `tryDirectPrepend` | The fast path exists to shift bytes with no allocation; adding an attribute means re-encoding. One comparison (`!dstASN4 && asns[0] > 0xFFFF`) keeps the hot case free. Benchmark: 0 allocs/op before and after |
| Prepend to a **received** AS4_PATH rather than rebuild it from AS_PATH | Rebuild AS4_PATH from the 2-octet AS_PATH | With `srcASN4=false`, AS_PATH holds AS_TRANS placeholders whose real values exist only in the received AS4_PATH. Rebuilding from AS_PATH would write AS_TRANS *into* AS4_PATH, destroying upstream information. Prepending the same ASNs to both keeps the Section 4.2.3 count delta invariant |
| Mappability scan skips confederation segments | Scan every segment (the old sibling behavior) | RFC 6793's own generation algorithm (`rfc/short/rfc6793.md:353-371`) sets `has_non_mappable` only in the non-confed branch. Scanning confed segments would emit AS4_PATH for a path whose only non-mappable ASN cannot legally be carried in it — and, when every segment is confed, a zero-length AS4_PATH, malformed under Section 6. Latent bug in the pre-existing sibling, fixed by sharing |
| Never drop a received AS4_PATH except to replace it | Always drop and re-derive (what the sibling does) | Keeps the change minimal: the diff only *adds* AS4_PATH where the RFC MUSTs it. The all-mappable-received-AS4_PATH input is spec-violating upstream; today's forward-verbatim behavior is preserved rather than newly changed |

## Known Limitations
- The zero-alloc fast path forwards a received AS4_PATH verbatim without inspecting it.
  For a spec-violating input (an all-mappable AS4_PATH, or one received from a NEW
  speaker) it is passed through rather than dropped. Dropping would require parsing on the
  hot path. The full path handles these correctly.
- RFC 6793 Section 4.2.3 ingress reconstruction (merging a received AS4_PATH into AS_PATH
  for an ASN4 peer) remains out of scope, as documented at `aspath_transcode.go:19-23`.

## RFC Documentation

`// RFC 6793 Section X.Y: "<quoted requirement>"` comments added above the enforcing code:
- `aspath_as4.go` `hasNonMappableASN` (Section 4.2.2 confed exclusion; Section 6 min length)
- `aspath_as4.go` `as4PathForPath` (Section 4.1 MUST NOT; Section 4.2.2 MUST / MUST NOT)
- `aspath_as4.go` `as4PathForRewrite` (Section 4.1 MUST NOT between NEW speakers)
- `aspath_as4.go` `writeAS4PathAttr` (Section 3: optional transitive, type 17)
- `aspath_rewrite.go` `tryDirectPrepend` guard (Section 4.2.2 MUST)
- `aspath_rewrite.go` `rewriteInsertASPath` (Section 4.2.2 MUST)
- `aspath_rewrite.go` `rewritePrependASPathFull` (Section 6 malformed-AS4_PATH discard)

## Implementation Summary

### What Was Implemented
- `internal/component/bgp/wireu/aspath_as4.go` (new): `hasNonMappableASN` (moved from the
  sibling, now confed-aware), `as4PathForPath`, `as4PathForRewrite`, `as4PathWireSize`,
  `writeAS4PathAttr`.
- `aspath_rewrite.go`: fast-path guard; AS4_PATH offsets captured in the slow-path scan;
  `rewritePrependASPathFull` parses a received AS4_PATH, constructs the outgoing one,
  skips the original when replacing it, and appends; `rewriteInsertASPath` emits AS4_PATH
  for locally-originated routes.
- `aspath_transcode.go`: routed through the shared owner; local `hasNonMappableASN` deleted.

### Bugs Found/Fixed
- Primary: AS4_PATH never emitted on the eBGP prepend path (7 call sites).
- Secondary (found while sharing): the sibling's `hasNonMappableASN` counted confed
  segments, so a confed-only non-mappable path would emit a zero-length AS4_PATH,
  malformed under RFC 6793 Section 6. Fixed by the confed-aware shared scan.
- Correction to the brief: AS_TRANS has **two** producers on this path, not one —
  `aspath_rewrite.go:292` (fast path) and `attribute/aspath.go:193` (full path).

### Deviations from Plan
- None.

## Goal Validation

| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| AS4_PATH emitted exactly when RFC 6793 4.2.2 requires | unit test, RFC-derived wire bytes | `TestRewriteASPath_AS4PathWireBytes`: expectation derived from RFC 6793 / RFC 4271 before observing ze; the AS_PATH half matched ze's pre-fix output byte-for-byte, isolating the missing AS4_PATH as the only difference |
| Not over-applied | unit test | `TestRewriteASPath_NoAS4PathWhenAllMappable`, `TestRewriteASPath_NoAS4PathToNewSpeaker` |
| Test genuinely gates | mutation | `as4PathForPath` → `return nil`: 11 tests red across both files. Fast-path guard → `if false`: exactly the 2 fast-path tests red. Both restored, green |
| No hot-path regression | benchmark | `BenchmarkRewriteASPath/ASN4_to_ASN4`: 20.33→20.77 ns/op, **0 B/op, 0 allocs/op** before and after |

## Review Gate

### Run 1 (closure — independent verification, 2026-07-21)

The AS4_PATH fix landed in commit `fb3e6f20b` ("fix(bgp): emit AS4_PATH when transcoding to
an old speaker"). An independent verification pass confirmed **all 10 ACs met** by the committed
code with producing `file:line` for each (AS4_PATH wire bytes `aspath_rewrite.go:552`
`writeAS4PathAttr`; shared owner `aspath_as4.go`; MUST-NOT-emit for all-mappable/AS4-capable
`aspath_as4.go:60/81`; confed excluded `:31-43`; dual rewrite `aspath_rewrite.go:420`; boundary
65535/65536/max `aspath_as4.go:37`; 0-alloc hot path `BenchmarkRewriteASPath`). The tests
genuinely gate the behavior (mutation: returning nil from `as4PathForPath` reds 16 tests across
BOTH the rewrite/`as4_rfc6793` AND `aspath_transcode` test files; the two MUST-NOT tests
correctly stay green — proving the rule is genuinely SHARED, not parallel). `make ze-rfc-check`
green (RFC6793-4.2.2-*/6-* tags resolve); `go vet` clean; 32 AS4 tests pass. The on-wire `.ci`
is legitimately deferred (harness one-rule-per-message limit) to the spec that owns the harness
contract; the behavior is proven byte-exact by unit tests instead.

**Verdict: CLEAN — 0 BLOCKER, 0 ISSUE.** Gate satisfied.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/bgp/wireu/`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (observed: 6 red, `tmp/as4-red.log`)
- [ ] Tests PASS (observed: `tmp/as4-green-v.log`, 27 PASS)
- [ ] Boundary tests for all numeric inputs
- [ ] Mutation-verified

### Known-red outside this change
`internal/component/bgp/reactor`: `TestPeerSettingsEqualDetects*` fail. Their test file
`peer_settings_reload_test.go` is **untracked** — a concurrent session's in-flight reload
work, unrelated to AS_PATH. Not introduced here; not chased here.
