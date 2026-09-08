# Spec: the announce build key IS the builder's argument set

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | in flight: `announceFacts` exists in the working tree, UNCOMMITTED |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from a journal row and
     skipped the spec step. Status is in-progress: the structural change exists
     in the working tree and closure has not run. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`AnnounceNLRIBatch` groups peers on a key, builds ONE UPDATE per group, and sends
it to every member. The key was a hand-written list of the per-peer facts that
make two peers equivalent, and the builder took MORE facts than the key named.
Two peers differing only in an unlisted fact hashed together, and the member
whose fact was ignored received another peer's bytes, decided by Go map
iteration order.

Nothing goes red: the UPDATE is well formed, the group is populated, and the
peer that builds first is correct.

Evidence is `plan/journal/key-omits-a-fact-the-builder-uses.md`. The measured
instance: the key carried a bare local AS where the builder takes the whole
local-as prepend, so an operator with one customer mid-migration under
`replace-as` and one without sent whichever AS_PATH built first to BOTH. Three of
the key's four original fields arrived as a FIX after the defect shipped. The row
is not restated here.

Goal: a new per-peer wire decision cannot reach the builder without entering the
key, and the compiler is what enforces it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the announce rail and grouped sending
  → Constraint: a group is one BUILD and one SEND, so a fact that changes only
    the send still has to partition the group.
- [ ] `ai/rules/principles.md` - registration over central enumeration
  → Constraint: "a new feature MUST register itself and be discovered; it MUST
    NOT require an edit to a switch, a case, a factory, a field list, or any
    other central enumeration". The old key was exactly that field list.
- [ ] `ai/rules/performance.md` - the announce path is hot
  → Constraint: the key must stay a cheap comparable map key: no pointer, no
    slice, no string.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Sections 5.1.2 and 5.1.5, the iBGP AS_PATH and LOCAL_PREF rules
  → Constraint: an internal peer must not be sent the prepend an external peer gets.
- [ ] `rfc/short/rfc7947.md` - Section 2.2.2.1, AS_PATH suppression for RS clients
- [ ] `rfc/short/rfc8669.md` - Section 8, Prefix-SID toward a peer outside the SR domain
- [ ] `rfc/short/rfc7911.md` - Section 3, the ADD-PATH path identifier before every NLRI
- [ ] `rfc/short/rfc6793.md` - Section 4.2.2, AS_TRANS and AS4_PATH toward an OLD speaker
- [ ] `rfc/short/rfc8654.md` - the extended message size, which is the SPLIT point
- [ ] RFC 7705 Section 3.3, the second AS number for a local-as override with no
  "Replace Old AS". No `rfc/short/rfc7705.md` exists; one is owed before closure.

**Key insights:**
- Every field in the key is an RFC-driven per-peer distinction. That is why the
  key is not an optimization detail: an omitted field is a conformance defect.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `announceFacts`, `buildBatchAnnounceUpdate`, and the grouping in `AnnounceNLRIBatch`
- [ ] `internal/component/bgp/reactor/forward_prefix_sid.go` - the four rails carrying the operator's Prefix-SID leaf

**Behavior to preserve:**
- One build per group and one send per group. The fan-out shape is unchanged.
- The per-BATCH inputs stay parameters of the builder: the attribute buffer, the
  NLRI buffer and the `NLRIBatch`. One pooled buffer pair is reused for every
  build, and every peer is offered the same batch, so none of the three can tell
  two peers apart.

**Behavior to change:**
- The key and the builder's argument set become ONE struct, `announceFacts`,
  passed to `buildBatchAnnounceUpdate` and used as the map key. A new field
  cannot be added to one without the compiler demanding it in the other.
- The bare local AS becomes the whole local-as prepend, so two peers sharing a
  local AS but differing on `replace-as` no longer share a build.
- `extended` is a field even though the BUILDER does not read it, because a group
  is one build AND one send, and `sendUpdateWithSplit` takes the extended size as
  its split point. It is passed to the builder unread rather than kept in a
  second list beside the key, because a second list is the defect being removed.

## Data Flow (MANDATORY)

### Entry Point
- An API or plugin announce reaching `AnnounceNLRIBatch`. Entry format is an
  `NLRIBatch` plus the set of peers to send it to.

### Transformation Path
1. For each peer, the rail resolves its per-peer facts into an `announceFacts`.
2. Peers are grouped by that value, which is the map key.
3. `buildBatchAnnounceUpdate` is called once per group, taking the same
   `announceFacts` as its argument.
4. `sendUpdateWithSplit` cuts the built UPDATE at the group's message size and
   sends it to every member.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| reactor ↔ wire | the built UPDATE bytes | partly; see What Remains |
| peer ↔ peer, inside one group | every member receives identical bytes | that is the property this spec exists to make true |
| operator config ↔ the key | each field is an operator-visible leaf or a negotiated capability | Yes, each field carries its RFC citation in the type |

### Integration Points
- `internal/component/bgp/reactor/forward_prefix_sid.go` - the Prefix-SID rails
  carry the operator's leaf rather than the resolved answer, matching the key.
- The withdraw rail, which shares the framing decision through `nlriUnitLen`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the group key and the builder argument are one value |
| No unintended coupling | Yes | confined to the reactor's announce rail |
| No duplicated functionality | Yes | the parallel list is DELETED, not kept beside the struct (`ai/rules/no-layering.md`) |
| Zero-copy preserved where applicable | Yes | `announceFacts` holds no pointer, slice or string, so it is a cheap comparable key on a hot path |
| Registration over hardcoding | Yes, by construction | a new per-peer wire decision reaches the builder only by becoming a field, and a field is in the key automatically |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every per-peer fact the builder reads is now a field | read at `buildBatchAnnounceUpdate` during the change | a peer receives another peer's bytes again | reading the builder; the compiler enforces it going forward | confirmed for today's tree |
| A-2 | No per-peer fact that changes only the SEND is left outside the key | `extended` was the one such fact and it is a field | two peers are cut into a different number of messages from one build | asserted; only `extended` and `groupUpdates` were found | UNVALIDATED |
| A-3 | `announceFacts` stays comparable as fields are added | Go refuses a map key holding a slice at compile time | the key silently becomes expensive, or fails to compile | the compiler | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A field is added that is NOT a per-peer wire distinction, splitting groups needlessly | announce throughput drops | each field carries its RFC citation, so a field with no citation is the tell |
| R-2 | The Prefix-SID field carries the resolved answer rather than the operator's leaf | two internal peers build the same bytes twice | deliberate and documented: the leaf has no meaning on an internal session, and one extra build is cheaper than a key that says something other than what the builder is given |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a peer receives another peer's AS_PATH, or another peer's NLRI framing. Wire-visible and silent |
| How is it reverted? | not yet landed; a single commit revert once it is |
| Who else touches this path? | the withdraw rail, which shares `nlriUnitLen`, and every future per-peer wire decision |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `AnnounceNLRIBatch` over two peers on one local AS differing on `replace-as` | → | the grouping over `announceFacts` (`internal/component/bgp/reactor/reactor_api_batch.go`) | `internal/component/bgp/reactor/announce_facts_partition_test.go` |
| `AnnounceNLRIBatch` over two peers differing on `group-updates` | → | `nlriUnitLen` and the framing | `internal/component/bgp/reactor/group_updates_framing_test.go` |
| `AnnounceNLRIBatch` over two peers differing on RFC 8654 extended size | → | `sendUpdateWithSplit` | `internal/component/bgp/reactor/zzprobe_announce_extended_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | two peers on one local AS, one with `replace-as` and one without | they land in DIFFERENT groups, and each receives the AS_PATH its own configuration calls for |
| AC-2 | two peers differing only on ADD-PATH, asn4, iBGP, RS-client, next hop, or the Prefix-SID leaf | different groups, one per distinct value |
| AC-3 | two peers differing only on the RFC 8654 extended message size | different groups, because the SPLIT point differs even though the build does not |
| AC-4 | two peers differing only on `group-updates` | different groups, because one batch leaves as a different number of frames |
| AC-5 | two peers identical in every field | ONE group and ONE build, so the grouping still does its job |
| AC-6 | a new per-peer fact is added to the builder | the code does not compile until it is a field of `announceFacts` |
| AC-7 | any group, any configuration | every member receives byte-identical UPDATEs, whatever the map iteration order |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures two eBGP customers on one `local-as`, one with `replace-as` and one without, and announces a prefix to both | API announce → `AnnounceNLRIBatch` → grouping on `announceFacts` → two builds → two sends | `announce_facts_partition_test.go`, plus the interop scenario named below |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the partition over each field | `internal/component/bgp/reactor/announce_facts_partition_test.go` | AC-1, AC-2, AC-5 | written, uncommitted |
| framing under `group-updates` | `internal/component/bgp/reactor/group_updates_framing_test.go` | AC-4 | written, uncommitted |
| extended-size split partition | `internal/component/bgp/reactor/zzprobe_announce_extended_test.go` | AC-3 | written, uncommitted |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| message size (RFC 8654) | 4096-65535 | 65535 | 4095, refused at capability negotiation | 65536, refused |
| local AS | 1-4294967295 | 4294967295 | 0, refused by config validation | N/A |
| peers per group | 1..n | n | 0, which produces no build | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an announce to two peers on one `local-as` differing on `replace-as` | `test/` `.ci`, not written | the operator sees the right AS_PATH on each session | MISSING. See "What Remains" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `local-as-replace-as-partition` | `test/interop/scenarios/` | FRR or BIRD, two sessions | each peer's received AS_PATH matches its own `replace-as` setting, so no peer receives the other's bytes | NOT WRITTEN. Named here as `ai/rules/interop-and-goal-validation.md` requires |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_batch.go` - `announceFacts` replaces the key and the argument list
- `internal/component/bgp/reactor/forward_prefix_sid.go` - the leaf the rails carry

## Files to Create
- `internal/component/bgp/reactor/announce_facts_partition_test.go`
- `internal/component/bgp/reactor/group_updates_framing_test.go`
- `internal/component/bgp/reactor/zzprobe_announce_extended_test.go`
- `test/interop/scenarios/local-as-replace-as-partition/` - the interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | every field reads an existing leaf or a negotiated capability |
| CLI commands/flags | No | no command surface changed |
| Functional test for new RPC/API | Yes, MISSING | no `.ci` drives the two-peer partition |
| Prometheus counters | No | none added |
| BGP family surface | No | no new SAFI, capability or attribute |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | No | the bytes each peer is owed are unchanged; which peer receives which bytes is what is repaired |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 7705 Section 3.3 is newly correct on the announce rail, and `rfc/short/rfc7705.md` does not exist. It is owed, with the `docs/features/rfc-status.md` row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` is the `// Design:` anchor of `reactor_api_batch.go` and MUST state that the group key IS the builder's argument set. `docs/architecture/update-building.md` is the `// Design:` anchor of `forward_prefix_sid.go` and MUST state that a rail carries the operator's leaf rather than the resolved answer, because that is what the key holds |
| 1-6, 8, 10, 11, 13-15, 17 | - | No | no other surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - write the partition test for the
   `replace-as` pair and observe it red against the bare-AS key.
2. **Phase: The struct** - make `announceFacts` the builder's parameter and the
   map key, and DELETE the parallel list.
3. **Phase: The send-only fact** - move `extended` into the struct, passed to the
   builder unread, and prove the split partition.
4. **Phase: Framing** - `groupUpdates`, and the shared `nlriUnitLen`.
5. **Phase: Interop** - the named scenario, walked red against the reverted fix.
6. **Phase: RFC and documentation** - `rfc/short/rfc7705.md`, its status row, and
   `docs/architecture/core-design.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every builder read corresponds to a field, checked at `buildBatchAnnounceUpdate` |
| Correctness | the per-BATCH inputs stay parameters and are NOT fields; a field that cannot tell two peers apart splits groups for nothing |
| Naming | each field's comment names the RFC section that makes it a distinction |
| Data flow | the withdraw rail's framing reads the same `nlriUnitLen` |
| Rule: `ai/rules/performance.md` | the struct stays comparable and holds no pointer, slice or string |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one struct, no parallel list | `gopls symbols internal/component/bgp/reactor/reactor_api_batch.go` shows one `announceFacts` and no second key type |
| each partition holds | `go test ./internal/component/bgp/reactor/ -run AnnounceFacts` |
| the interop scenario discriminates | revert the fix, rebuild, observe RED, restore, observe GREEN, record it |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | more key fields mean more groups and more builds. A field with no per-peer wire distinction costs throughput for nothing |
| Information disclosure between peers | this IS the defect: one peer receiving another peer's AS_PATH |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A partition test passes before the fix | the test does not discriminate; force it red first |
| Interop scenario green before the fix | vacuous; rebuild the daemon binary so the revert takes effect |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- A key that is a hand-written list of "every fact that changes the bytes" is
  maintained by remembering, and the majority of this one's fields arrived after
  the defect they prevent had already shipped. Making the key BE the argument set
  moves the obligation from memory to the compiler.
- The failure is silent by construction: the UPDATE is well formed, the group is
  populated, and the peer that builds first is correct. Only the second peer is
  wrong, and only against a configuration nobody wrote a test for.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One struct for key and arguments | keep two lists and add a test that compares them | a test can only check the fields it knows about; the compiler checks the ones nobody thought of |
| `extended` is a field the builder does not read | keep it in a second list beside the key | a second list is the defect this type removes |
| The Prefix-SID field carries the operator's LEAF | carry the resolved answer | the leaf is what the four rails carry, and one extra build for two internal peers is cheaper than a key that disagrees with the builder's argument |

## Known Limitations
- Two internal peers differing only on the Prefix-SID leaf build the same bytes
  twice. Deliberate, and stated above.

## RFC Documentation (Scope: protocol)
Each field of `announceFacts` carries `// RFC NNNN Section X.Y` above it, naming
the requirement that makes it a per-peer distinction: RFC 4271 Sections 5.1.2 and
5.1.5, RFC 7947 Section 2.2.2.1, RFC 8669 Section 8, RFC 7705 Section 3.3,
RFC 7911 Section 3, RFC 6793 Section 4.2.2, and RFC 8654 for the split point.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Current Condition and What Remains

**IN FLIGHT.** `announceFacts` exists in the working tree with every field and
its RFC citation, and `reactor_api_batch.go` is uncommitted. Three test files are
new and untracked. No SHA can be cited.

| Item | State |
|------|-------|
| Structural change | in the working tree, UNCOMMITTED |
| Journal row | written, `plan/journal/key-omits-a-fact-the-builder-uses.md`. The local-AS field is called fixed in the announce-rail commit; the CLASS repair is this spec |
| PROVEN | the partition for the fields the three new unit tests cover, once they are run and observed red against the bare-AS key |
| ASSERTED, not proven | AC-6, which is a compiler property with no test naming it, and AC-7, which is a statement about every configuration rather than the ones tested. A-2 is unvalidated: only `extended` and `groupUpdates` were found as send-only facts, and nothing proves that set is complete |
| Remains | (1) the `.ci` over the two-peer `replace-as` case; (2) the `local-as-replace-as-partition` interop scenario, with the revert-rebuild-red walk `ai/rules/interop-and-goal-validation.md` requires; (3) `rfc/short/rfc7705.md`, which does not exist, and its `docs/features/rfc-status.md` row; (4) `docs/architecture/core-design.md`; (5) landing it; (6) closure sections |
