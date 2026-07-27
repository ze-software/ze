# Spec: fixit-isis-lsdb-entry-race

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-27 |

Retroactive spec (`ai/rules/planning.md` "Retroactive Specs"): the fix landed as
`7f3bfd338` (the atomics + the regression test) and `71f91c170` (the field-discipline
correction + the removal of a dead accessor) before the spec body was written. The
sections below were reconstructed from the committed code in the working tree on
2026-07-27, not from the commit messages.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/no-fabrication.md` - cite the producing `file:line`, not the caller
4. `internal/plugins/isis/lsdb/entry.go` (the FIELD DISCIPLINE struct doc),
   `internal/plugins/isis/lsdb/aging.go`, `internal/plugins/isis/lsdb/lsdb.go`

## Task

`TestISISDISElection` fails under `-race` with a genuine DATA RACE on an IS-IS LSDB
`Entry` field. Found incidentally by `make ze-verify-changed` while working an
unrelated BGP spec; it is NOT caused by that work (nothing under `internal/plugins/isis`
was touched, and the BGP packages in the same run pass).

**Diagnosis.** `Entry.lifetime` is mutated by the aging tick under the database lock:
`e.lifetime--` at `internal/plugins/isis/lsdb/aging.go:121`, reached via
`internal/plugins/isis/lsdb_wiring.go:96,78`. It is read WITHOUT any lock through the
plain accessor `func (e *Entry) Lifetime() types.RemainingLifetime { return e.lifetime }`
at `internal/plugins/isis/lsdb/entry.go:109`, reached from SNP generation at
`internal/plugins/isis/lsdb/snp.go:319,360` on the flooding goroutine
(`internal/plugins/isis/flooding_wiring.go:266,248`).

So the LSDB has a lock discipline that its own exported accessors bypass. `Lifetime()`
is unlikely to be the only one -- `Sequence()`, `Checksum()`, `LSPID()` and the
overload-bit reader sit beside it in the same file with the same shape, and any of them
read fields the aging or update paths write.

Reproducible, so this gets a spec rather than a `plan/known-failures/` shard
(`ai/rules/no-parking.md`: a deterministic failure has no recording path).

**Investigate and fix at the discipline, not the one field.** Decide whether `Entry`
accessors take the database lock, whether callers must hold it (and the accessors are
renamed `...Locked` to say so), or whether the mutable fields become atomics. A
per-field patch that silences the one reported race leaves the sibling accessors
equally unsafe.

Reproduce with:
`go test -race -run TestISISDISElection ./internal/plugins/isis/`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `plan/learned/932-isis-6-lsdb.md` - the `// Design:` target of every file changed here; the LSDB store, aging and snapshot model
  → Decision: the LSDB is a single-writer store behind one `sync.RWMutex`; writers (aging tick, origination, receive) take the write lock, snapshot readers take the read lock
  → Constraint: `Snapshot` hands out flat copies so no live pointer crosses the CLI/RPC boundary -- but `Lookup` does return the live pointer, which is what makes the field discipline load-bearing
- [ ] `ai/rules/no-fabrication.md` - the rule that the second commit applies to itself
  → Constraint: a code comment is its author's belief, not a decision record; read the producing function before asserting what a field does
- [ ] `ai/rules/fail-closed-guards.md` - why an "only these two fields" claim must be an enumeration, not an assertion
  → Constraint: state the condition, then enumerate against it; a claim that names the wrong set is worse than no claim

### RFC Summaries (MUST for protocol work)
- [ ] ISO/IEC 10589 clause 7.3.16 / 7.3.17 (no `rfc/short/` summary: this is an ISO standard, referenced inline in the source)
  → Constraint: Remaining Lifetime decrements once per second; reaching 0 makes the LSP a purge that is re-flooded and RETAINED for `ZeroAgeLifetime`, not deleted -- which is why `lifetime` and `purged` are both mutated on an already-stored entry
  → Constraint: clause 7.3.16 also refreshes the held Remaining Lifetime from a DUPLICATE LSP, a second post-publication write to the same field (`internal/plugins/isis/lsdb/lsdb.go:190`)

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The lock is not the whole story: `Lookup` returns a live `*Entry`, so the caller holds a pointer with no lock. What decides safety per field is whether the field is written after the entry became reachable.
- Two independent conditions, not one. A field needs an atomic only when it is BOTH mutated post-publication AND read off-lock. Post-publication mutation alone is fine when nothing reads it off-lock; an off-lock read alone is fine when the field is written once before publication.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/isis/lsdb/entry.go:66` - the `Entry` struct: 14 fields, 9 exported accessors, and (post-fix) the FIELD DISCIPLINE doc at `entry.go:40`
- [ ] `internal/plugins/isis/lsdb/lsdb.go:216` - `replaceLocked` builds a FRESH `Entry` and publishes it with `store.entries[id] = e` at `lsdb.go:252`; every scalar metadata field is assigned before that line
- [ ] `internal/plugins/isis/lsdb/lsdb.go:295` - `Lookup` takes the read lock, returns the live `*Entry`, and releases; the caller then holds a pointer with no lock
- [ ] `internal/plugins/isis/lsdb/lsdb.go:190` - `Receive`'s `Equal` branch refreshes the held lifetime IN PLACE (clause 7.3.16), the second post-publication write to `lifetime`
- [ ] `internal/plugins/isis/lsdb/aging.go:121` - the tick decrement, and `aging.go:142` `markPurgedLocked` (sets lifetime 0, purged, deleteAt)
- [ ] `internal/plugins/isis/lsdb/snp.go:319` - `buildPSNP` reads `Lifetime`/`Sequence`/`Checksum` on a `Lookup` result with no lock held: the reader half of the reported race
- [ ] `internal/plugins/isis/spf_wiring.go:153` - SPF reads `IsPurged`/`Decode`/`IsOverloaded` off-lock
- [ ] `internal/plugins/isis/lsdb/aging_test.go:236` - `TestISISLSDBEntryAccessorsAreRaceFree`, the regression test

**Behavior to preserve:** (unless user explicitly said to change)
- The LSDB stays a single-writer store behind one `sync.RWMutex`; the atomics are for the benefit of unlocked READERS, not a replacement for the lock or a new write-ordering mechanism (`internal/plugins/isis/lsdb/entry.go:171`).
- Every aging, purge and freshness semantic is unchanged: 1s decrement, purge-not-delete at 0, `ZeroAgeLifetime` grace, received-purge vs local-expiry distinction, clause 7.3.16.1 purge tiebreak.
- `Lifetime()` keeps returning `types.RemainingLifetime` (a uint16 value) even though the backing store widens to `atomic.Uint32`: no caller sees a type change.
- `Snapshot`, `LSPEntries`, `LSPIDs` keep handing out copies, never live pointers.

**Behavior to change:** (only if user explicitly requested)
- None at runtime. The change is representational (two plain fields become atomics), documentary (the discipline and `Lookup`'s contract), and a dead-accessor removal. No wire-visible or operator-visible behavior changes.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
Two concurrent goroutines reach the same `lsdb.Entry`:
- the aging tick, `internal/plugins/isis/lsdb_wiring.go:78,96` -> `lsdb/aging.go:83,121`
- SNP generation on the flooding goroutine, `internal/plugins/isis/flooding_wiring.go:248,266`
  -> `lsdb/snp.go:360,319` -> `lsdb/entry.go:109`

### Transformation Path
1. The aging tick decrements `Entry.lifetime` under the database lock (`aging.go:121`),
   and may transition the entry to purged via `markPurgedLocked`.
2. Concurrently, SNP generation walks entries and reads `Entry.Lifetime()`
   (`entry.go:109`) with no lock held, to build the SNP lifetime field.
3. The two overlap on the same 4-byte field, which `-race` reports as a write/read race.
4. After the fix, step 1 writes through `setLifetime` (`entry.go:173`, an
   `atomic.Uint32` store) and step 2 reads through `Lifetime()` (`entry.go:167`, an
   atomic load), so the same overlap is a defined, race-free hand-off.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LSDB lock ↔ lock-free reader | `Lookup` (`lsdb.go:295`) returns the LIVE `*Entry` and releases the read lock; the caller reads fields afterwards with no lock | Yes -- every off-lock accessor call site enumerated in Design Insights |
| Aging goroutine ↔ flooding goroutine | both hold the same `*Entry`; the tick writes, SNP generation reads | Yes -- `TestISISLSDBEntryAccessorsAreRaceFree` drives exactly this pair |
| LSDB ↔ SPF / show / DIS | `spf_wiring.go:152`, `show.go:66`, `dis_wiring.go:330` each `Lookup` then read off-lock | Yes -- all three read only `IsPurged`, `Decode`, `IsOverloaded` |
| Entry ↔ CLI/RPC | `Snapshot` (`lsdb.go:498`) copies into flat `LSPSnapshot` values under the read lock; no pointer escapes | Yes -- unchanged by this work |

### Integration Points
- `internal/plugins/isis/lsdb/snp.go:319,427` (`buildPSNP`, `compareSNPEntry`) - the production off-lock reader of `Lifetime()`; unchanged, now reading an atomic.
- `internal/plugins/isis/lsdb/aging.go:121,146` - the production writer; converted from `e.lifetime--` / `e.lifetime = 0` to `setLifetime`.
- `internal/plugins/isis/lsdb/lsdb.go:190,230` - the two other lifetime write sites (duplicate refresh, fresh entry build).

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

Findings for the four rows above (this spec adds no command, view, family or handler,
so the registration row is N-A by construction):

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every write still happens under `d.mu`; the atomics change the read side only |
| No unintended coupling | Yes | the change is confined to `internal/plugins/isis/lsdb`; no caller signature changed |
| No duplicated functionality | Yes | `setLifetime` replaces the four scattered plain assignments rather than adding a parallel path |
| Zero-copy preserved | Yes | `Raw()` still returns the entry's owned slice, `Decode()` still parses lazily and aliases it |
| Registration over hardcoding | N-A | no command, view, family or handler is added |

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Sequence()`, `Checksum()`, `LSPID()` and `IsOverloaded()` are equally unsafe siblings of `Lifetime()` | the Task section's reasoning ("sit beside it in the same file with the same shape") | the fix would be four more atomics that cost reads and prove nothing | read every write site of each field | **broken** -- `replaceLocked` publishes a FRESH entry (`lsdb.go:252`), so those fields are written once before the entry is reachable |
| A-2 | `lifetime` and `purged` are the ONLY fields mutated after publication | the comment `7f3bfd338` added to `entry.go` | a future reader concludes any other field is safe to expose, and the next accessor is a race | grep every assignment to every `Entry` field | **broken** -- `recvPurgeReflooded` (`aging.go:113`) and `deleteAt` (`aging.go:148`, `lsdb.go:239`) are also post-publication, plus the three flag maps; corrected by `71f91c170` |
| A-3 | What forces an atomic is post-publication mutation AND an off-lock read, both required | the corrected discipline at `entry.go:40-65` | either half alone would over- or under-protect | enumerate both conditions across all 14 fields | **confirmed** -- see the field table in Design Insights; exactly `lifetime` and `purged` satisfy both |
| A-4 | Every off-lock read of an `Entry` field goes through an exported accessor | `Lookup` is the only producer of a live `*Entry` (`lsdb.go:295`) | an in-package off-lock read would be an unguarded race the accessor audit misses | grep every `*Entry` producer and every direct field access | **confirmed** for production code; one TEST reads a lock-only field off-lock (`aging_test.go:152`), benign because that test is single-goroutine -- recorded in Known Limitations |
| A-5 | `TestISISLSDBEntryAccessorsAreRaceFree` fails deterministically if either field reverts to plain | asserted in the test's own doc comment as committed | the regression is not actually gated and the AC evidence is vacuous | mutation-verify: revert each atomic, run the test repeatedly | **broken** -- 6/12 and 10/12 single runs. Fixed in this session; now 12/12. See Deviations |

### Risks
<!-- Things that could go wrong even if all assumptions hold. From /ze-spec Failure Mode Analysis. -->
<!-- Surviving risks copy forward to the Executive Summary "Risks & observations" and the learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A future field or accessor is added over a post-publication-mutated field, reintroducing the race | `-race` red in the isis suite, or none at all if the window is narrow | the FIELD DISCIPLINE doc (`entry.go:40`) states the test to apply, and the regression test now calls EVERY accessor so a new unsafe one fails |
| R-2 | The regression test degrades into a coin flip again if the writer stops writing mid-run | the test still passes with the atomics reverted | the writer re-originates purged LSPs so both post-publication writes keep flowing; the rationale and the measured 6/12 figure are recorded in the test doc |
| R-3 | Making `lifetime` atomic hides a torn multi-field read: `buildPSNP` (`snp.go:319-322`) reads lifetime, sequence and checksum as three separate off-lock loads | none -- it is not a race and `-race` will never flag it | benign: `sequence` and `checksum` are immutable for a given `Entry` object, so the tuple can only mix a fresher lifetime with that same entry's own sequence, and the SNP lifetime field is advisory. Recorded, not fixed |
| R-4 | `atomic.Uint32` for a uint16 value silently widens the store; a future writer could store a value above 65535 | a lifetime that never ages out | `setLifetime` takes `types.RemainingLifetime` (a uint16 type), so the narrowing is enforced by the parameter type, not by the field |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| aging tick concurrent with SNP generation | → | `lsdb.Entry` accessor lock discipline | `TestISISLSDBEntryAccessorsAreRaceFree` (`internal/plugins/isis/lsdb/aging_test.go:236`), run under `-race` |
| every exported `Entry` accessor | → | the same discipline | `TestISISLSDBEntryAccessorsAreRaceFree` reader loop (`aging_test.go:297-307`) calls all nine accessors while the tick runs |
| the originally reported symptom | → | `Entry.lifetime` read by DIS election | `TestISISDISElection` (`internal/plugins/isis/dis_wiring_test.go:115`) under `-race` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The aging tick runs concurrently with unlocked `Lifetime()` and `IsPurged()` reads on a live `*Entry` obtained from `Lookup` | `-race` reports no data race |
| AC-2 | Every OTHER exported `*Entry` accessor (`LSPID`, `Sequence`, `Checksum`, `IsOverloaded`, `IsOwn`, `Raw`, `Decode`) is called off-lock while the tick runs | `-race` reports no data race: each reads a field written once by `replaceLocked` before the entry is published |
| AC-3 | Either `lifetime` or `purged` is reverted to a plain (non-atomic) field | the regression test fails under `-race` on a single `-count=1` run, not merely on a repeated run |
| AC-4 | A developer adding a field or accessor consults the `Entry` struct doc | the doc states BOTH conditions that together require an atomic, and classifies every field as ATOMIC, LOCK-ONLY, or written-once-before-publication |
| AC-5 | A caller reads `Lookup`'s doc to decide what is safe on the returned pointer | it names the accessors that are safe off-lock and states that mutating anything, or reading another mutable field directly, is not |
| AC-6 | Repo-wide search for an exported `Entry` accessor over a field that is mutated after publication and not atomic | none exists (`IsReceivedPurge`, whose doc falsely claimed the engine read it, is deleted) |
| AC-7 | `TestISISDISElection`, the originally reported symptom, is run under `-race` | it passes |

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path. -->

N-A. This spec changes no user-facing operation: it is an internal concurrency and
documentation fix inside `internal/plugins/isis/lsdb`, with no config leaf, command,
event, or wire byte added or changed. The operator-visible surfaces that read the
affected fields (`show isis database` via `Snapshot`, CSNP/PSNP via `LSPEntries` and
`buildPSNP`) return identical values before and after.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISLSDBEntryAccessorsAreRaceFree` | `internal/plugins/isis/lsdb/aging_test.go:236` | AC-1, AC-2, AC-3 -- the tick and four reader goroutines share live entries; `-race` is the oracle | Done |
| `TestISISDISElection` | `internal/plugins/isis/dis_wiring_test.go:115` | AC-7 -- the originally reported symptom stays green under `-race` | Done (pre-existing test) |
| `TestISISLSDBAgeDecrement` | `internal/plugins/isis/lsdb/aging_test.go:29` | the 1s decrement is unchanged by routing the write through `setLifetime` | Done (pre-existing test) |
| `TestISISLSDBAgeToPurge` | `internal/plugins/isis/lsdb/aging_test.go:44` | purge-not-delete and the grace period are unchanged by the `purged` atomic | Done (pre-existing test) |
| `TestISISReceivedPurgeRefloodSurfaced` | `internal/plugins/isis/lsdb/aging_test.go:139` | the received-purge distinction still holds after `IsReceivedPurge` was deleted and the field is read directly | Done (pre-existing test, call sites changed) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Entry.lifetime` (stored in an `atomic.Uint32`, valued as `types.RemainingLifetime`, a uint16) | 0-65535 | 65535 | N/A (unsigned) | N/A -- unreachable: `setLifetime` takes `types.RemainingLifetime`, so the compiler rejects a wider value at every call site |

Existing lifetime boundary coverage is unchanged and lives in
`internal/plugins/isis/lsdb/boundary_test.go`; this spec adds no new numeric input.

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (none) | - | N/A -- no user-facing behavior change. The spec adds no config leaf, command, RPC, event or wire byte; `show isis database` and CSNP/PSNP output are byte-identical before and after. The user-visible property at stake (the daemon does not have a data race) is not observable from a `.ci` test and is proven by the `-race` unit test instead | N/A |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (none) | - | - | N-A per `ai/rules/interop-and-goal-validation.md` "When interop tests are NOT required": pure internal change with no wire-visible difference. The LSP, CSNP and PSNP bytes produced are unchanged; only the memory-access discipline behind them changed | N-A |

### Future (if deferring any tests)
- None deferred.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/isis/lsdb/entry.go` - `lifetime` to `atomic.Uint32`, `purged` to `atomic.Bool`, `Lifetime()`/`IsPurged()` read via atomic load, new unexported `setLifetime`, the FIELD DISCIPLINE struct doc, per-field ATOMIC / LOCK-ONLY annotations, deletion of `IsReceivedPurge`
- `internal/plugins/isis/lsdb/aging.go` - tick decrement and `markPurgedLocked` write through `setLifetime` / `purged.Store`; in-package reads go through `IsPurged()` / `Lifetime()`
- `internal/plugins/isis/lsdb/lsdb.go` - `replaceLocked` and the clause 7.3.16 duplicate refresh write through `setLifetime` / `purged.Store`; `Lookup`'s doc rewritten from "callers that only read metadata are fine" to the precise safe/unsafe split
- `internal/plugins/isis/lsdb/aging_test.go` - the `-race` regression test; its two `IsReceivedPurge()` call sites read the field directly

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | no config or command surface changes |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | no new leaf |
| CLI commands/flags | No | no command added or renamed |
| CLI grammar (action before identifier) | No | no command added |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | No | no RPC or API added |
| Pipe completeness | No | no new command output |
| Env var registration | No | no `environment/` leaf added |
| Doctor check for runtime dependencies | No | no file path, socket, port, module, binary or certificate introduced |
| Prometheus counters/metrics | No | `ze_isis_purges_total` is unchanged; `markPurgedLocked` still bumps it exactly once per purge transition (`aging.go:149`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | none -- internal concurrency fix |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | the isis plugin's registration, commands and schema are untouched |
| 6 | Has a user guide page? | No | no behavior an operator can observe changed |
| 7 | Wire format changed? | No | LSP/CSNP/PSNP bytes are identical |
| 8 | Plugin SDK/protocol changed? | No | `Entry` is an internal type in `internal/plugins/isis/lsdb`, not part of `pkg/plugin` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | clause 7.3.16/7.3.17 behavior is preserved exactly, not extended; no `rfc/short/` summary or `docs/features/rfc-status.md` row changes state |
| 10 | Test infrastructure changed? | No | one new unit test in an existing file, using the existing helpers |
| 11 | Affects daemon comparison? | No | no capability gained or lost |
| 12 | Internal architecture changed? | No | the lock model is unchanged; the field-level discipline is documented in the code that owns it (`entry.go`), which is where a reader adding a field will be |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | none |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | verified by grep, see Pre-Commit Verification |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the changed symbols are unexported or internal-only; no doc shows them |

## Files to Create
- None. Every change lands in an existing file.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)**: a `-race` test that drives the real pair
   - Tests: `TestISISLSDBEntryAccessorsAreRaceFree`
   - Files: `internal/plugins/isis/lsdb/aging_test.go`
   - Verify: the test reproduces on demand, unlike `TestISISDISElection` which surfaced the race by scheduling luck and did not reproduce in 20 runs
2. **Phase: Enumerate before fixing**: establish which fields actually need protection
   - Tests: none (a reading pass)
   - Files: every write site and every accessor in `internal/plugins/isis/lsdb`
   - Verify: the two conditions are stated separately and every field is classified against both; A-1 and A-2 are resolved by evidence rather than by the Task section's guess
3. **Phase: The atomics**: `lifetime` and `purged` only
   - Tests: `TestISISLSDBEntryAccessorsAreRaceFree` goes green; `TestISISLSDBAgeDecrement`, `TestISISLSDBAgeToPurge` and `TestISISReceivedPurgeRefloodSurfaced` stay green
   - Files: `entry.go`, `aging.go`, `lsdb.go`
   - Verify: `go test -race` on the isis tree is clean
4. **Phase: The contract**: the FIELD DISCIPLINE doc and `Lookup`'s corrected doc
   - Tests: none (documentation)
   - Files: `entry.go`, `lsdb.go`
   - Verify: the doc names both conditions and every field; the sentence that invited the race is gone
5. **Phase: Dead accessor**: delete `IsReceivedPurge`
   - Tests: `TestISISReceivedPurgeRefloodSurfaced` reads the field directly (same package), so coverage is unchanged
   - Verify: repo-wide grep for the symbol returns nothing
6. **Full verification** → `make ze-verify`
7. **Complete spec** → fill audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation or evidence at `file:line` |
| Enumeration honesty | Any "only these fields" claim in a comment is an enumeration checked against every assignment, not an assertion. This is the defect the second commit exists to fix; do not reintroduce it |
| Correctness | Every former plain write to `lifetime`/`purged` goes through `setLifetime`/`Store`; a missed one is a silent half-fix that `-race` may not catch |
| Test discriminates | Reverting either atomic must turn the regression test RED on a SINGLE run. A test that only fails on repeat is a coin flip in a `-count=1` suite (`ai/rules/interop-and-goal-validation.md` vacuity traps) |
| Data flow | The lock is still the write mechanism; the atomics serve unlocked readers only. No write path was moved out from under `d.mu` |
| Naming | `setLifetime` stays unexported: an exported setter would invite a write from outside the lock |
| Rule: no-fabrication | Every claim about what a field does cites the producing assignment, not a comment |
| Rule: wiring-completeness | No exported accessor is left without a non-test caller |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `lifetime` and `purged` are atomics | `grep -n 'atomic\.' internal/plugins/isis/lsdb/entry.go` |
| No plain assignment to either field remains | `grep -rn 'lifetime *=\|purged *=' internal/plugins/isis/lsdb/*.go` shows no `Entry` field assignment |
| `IsReceivedPurge` is gone | repo-wide `grep -rn 'IsReceivedPurge'` returns nothing |
| The regression test exists and gates | mutation-verify: revert an atomic, confirm RED on a single run, restore |
| The isis tree is race-clean | `go test -race` on `./internal/plugins/isis/...` with the feature tags |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Unchanged: no new external input is parsed. `MaxLSPsPerLevel` and the codec validation that precede `Receive` are untouched |
| Memory safety | `Raw()` still returns the entry's single OWNED copy, never an alias of a reused receive buffer; `Decode()`'s TLV values still alias only that owned copy, and `packet.ReleaseTLVs` clears the TLV slice, not the entry bytes |
| Resource exhaustion | The regression test's re-origination loop is bounded by the test's own stop channel and reuses eight pre-built PDUs; it adds no production allocation |
| Torn reads | An `atomic.Uint32` load cannot tear; the multi-field `buildPSNP` read is consistency-relaxed but not unsafe (R-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `Sequence`, `Checksum`, `LSPID` and the overload reader are unsafe siblings of `Lifetime` (A-1, the Task section's own reasoning) | `replaceLocked` builds a FRESH `Entry` and publishes it at `lsdb.go:252`, so those fields are written once before the entry is reachable | reading every assignment to each field rather than reasoning from the accessors' shape | the fix is two atomics instead of six; four needless atomic loads on the SNP hot path avoided |
| `lifetime` and `purged` are the only fields mutated after publication (A-2, the comment `7f3bfd338` shipped) | four are: `recvPurgeReflooded` (`aging.go:113`) and `deleteAt` (`aging.go:148`) too, plus the three per-circuit flag maps | re-reading every access instead of trusting the just-written comment | the fix stayed correct (the conjunction still selects the same two fields) but the comment told a future reader that adding an accessor over `deleteAt` would be safe. Corrected by `71f91c170` |
| The regression test fails deterministically if either atomic is reverted (A-5, asserted in the test's own doc) | it caught the `lifetime` revert in 6 of 12 single runs and the `purged` revert in 10 of 12 | mutation-verifying the claim instead of reading it | the AC-3 evidence was vacuous half the time; fixed in this session by keeping the writer writing (see Deviations) |
| `IsReceivedPurge`'s doc: "the engine reads this to decide the distinct flooding behavior" | the engine reads the `receivedPurge` FIELD directly in `aging.go:112,119,125` under the lock; the accessor had only two test callers | repo-wide grep for the symbol | an exported accessor with no non-test caller was carrying a fabricated justification; deleted |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Make every `Entry` accessor take the LSDB lock | `Lookup` already returned the pointer, so the lock would be re-entered per field read on the SNP hot path, and a caller holding a stale pointer would still read a freed-from-the-map entry | atomics on the two fields that actually need them; the rest stay plain and lock-free |
| Rename the accessors `...Locked` and require callers to hold the lock | `Lookup` releases the lock before returning, so every existing off-lock caller (SPF, show, DIS, SNP) would have to be restructured to hold it across the read -- a much larger change to fix a two-field problem | same as above |
| Rely on `TestISISDISElection` as the regression test | it surfaced the race by scheduling luck and did not reproduce on demand in 20 runs | a dedicated test that drives the tick against four reader goroutines |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A comment asserting "only X and Y" without enumerating against every site | twice in this spec alone (A-2, and `IsReceivedPurge`'s "the engine reads this") | already covered by `ai/rules/no-fabrication.md` ("citing a code comment as the project's design intent") and `ai/rules/fail-closed-guards.md` | no new rule; the Critical Review Checklist above carries an "Enumeration honesty" row so the next reviewer of this area checks it |
| A `-race` test whose window closes early, so it passes with the bug present | once here; the same shape as the sleep-based flakes in `ai/rules/fix-dont-record.md` | mutation-verify every concurrency regression test, not just functional ones | `ai/rules/functional-test-gate.md` already mandates mutation-verification; this is a reminder that it applies to `-race` tests, where "the oracle is the race detector" makes it easy to skip |

## Design Insights

**The full field classification.** Every `Entry` field, against both conditions.
Condition 1 is "mutated after `store.entries[id] = e` (`lsdb.go:252`)"; condition 2 is
"read with no LSDB lock held". Only a field satisfying BOTH needs an atomic.

| Field | Mutated post-publication? | Read off-lock? | Storage | Evidence |
|-------|---------------------------|----------------|---------|----------|
| `lifetime` | Yes -- `aging.go:121` (tick decrement), `aging.go:146` (markPurgedLocked), `lsdb.go:190` (clause 7.3.16 duplicate refresh) | Yes -- `Lifetime()` from `snp.go:319,427` | `atomic.Uint32` | both conditions |
| `purged` | Yes -- `aging.go:147` (markPurgedLocked) | Yes -- `IsPurged()` from `spf_wiring.go:153`, `show.go:67`, `dis_wiring.go:331` | `atomic.Bool` | both conditions |
| `recvPurgeReflooded` | Yes -- `aging.go:113` | No -- no accessor, read only at `aging.go:112` under the write lock | plain | condition 1 only |
| `deleteAt` | Yes -- `aging.go:148`, `lsdb.go:239` | No -- no accessor, read only at `aging.go:100` under the write lock | plain | condition 1 only |
| `srm` | Yes -- `lsdb.go:385,419,472` | No -- only via `SetSRM`/`ClearSRM`/`SRM`/`ClearCircuit`, all of which take the lock | plain map | condition 1 only |
| `ssn` | Yes -- `lsdb.go:440,449,473` | No -- same, via the SSN methods | plain map | condition 1 only |
| `srmSent` | Yes -- `lsdb.go:386,404,409,474` | No -- only via `SetSRM`/`noteSRMTransmit`/`ClearCircuit` | plain map | condition 1 only |
| `sequence` | No -- assigned only in the `replaceLocked` literal (`lsdb.go:225`) | Yes -- `Sequence()` from `snp.go:321,421,423`, `flooding.go:308`, `lsdb_wiring.go:721` | plain | condition 2 only |
| `checksum` | No -- `lsdb.go:226` | Yes -- `Checksum()` from `snp.go:322` | plain | condition 2 only |
| `typeBlock` | No -- `lsdb.go:227` | Yes -- `IsOverloaded()` from `spf_wiring.go:162` | plain | condition 2 only |
| `own` | No -- `lsdb.go:228` | Reachable via `IsOwn()`, but that accessor has NO caller (see Known Limitations) | plain | condition 2 only, latently |
| `raw` | No -- `lsdb.go:224`, and never mutated in place afterwards | Yes -- `Raw()` from `flooding.go:386`, `Decode()` from `spf_wiring.go:156`, `show.go:70`, `lsdb_wiring.go:844` | plain | condition 2 only |
| `id` | No -- `lsdb.go:223` | Yes -- `LSPID()` | plain | condition 2 only |
| `receivedPurge` | No -- `lsdb.go:238`, before publication at `lsdb.go:252` | No production reader off-lock; read at `aging.go:112,119,125` under the lock | plain | neither |

**Why `replaceLocked` is the load-bearing fact.** The whole classification turns on one
implementation choice made elsewhere: a freshness replace does not update the stored
`Entry` in place, it builds a new one and swaps the map slot. That is what makes
"written once before the entry is reachable" true for eleven of the fourteen fields, and
it is why the Task section's expectation of four more unsafe siblings was wrong. If
`replaceLocked` were ever changed to mutate in place, `sequence`, `checksum`,
`typeBlock`, `own` and `raw` would all acquire condition 1 at once and every one of them
is read off-lock.

**Why the invitation mattered more than the bug.** The reported race was one field, but
its cause was a sentence: `Lookup`'s doc said "callers that only read metadata are
fine". A read races a concurrent write whether or not the reader also writes, so the doc
was actively wrong, and it was the reason the SNP path felt safe to write. Fixing the
field without fixing the sentence would have left the next accessor to make the same
mistake.

## Core Insight

The question "does this field need an atomic?" is not one question. It is a conjunction
of two independent ones -- is it written after it became reachable, and is it read
without the lock -- and conflating them fails in both directions. Assume condition 1
implies the answer and you make `deleteAt` and `recvPurgeReflooded` atomic for nothing.
Assume condition 2 implies it and you make `sequence`, `checksum` and `raw` atomic for
nothing. State the two conditions separately, enumerate every field against both, and
the answer is a two-row table rather than an argument.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two atomics on the fields that need them | lock every accessor; rename accessors `...Locked` and push the lock to callers | `Lookup` hands out a live pointer and releases the lock, so both alternatives require restructuring every off-lock caller (SPF, show, DIS, SNP) to fix a two-field problem. The atomics keep the read path lock-free, which is what the SNP hot path needs |
| `setLifetime` stays unexported | export a `SetLifetime` for symmetry with `Lifetime()` | an exported setter invites a write from outside the LSDB lock. The atomic exists for unlocked READERS; writes remain the lock's job, and the doc at `entry.go:171` says so |
| `atomic.Uint32` for a uint16 value | `atomic.Uint64`, or a `uint32` with manual `atomic.Load/StoreUint32` | `atomic.Uint32` is the narrowest typed atomic Go offers; `setLifetime`'s `types.RemainingLifetime` parameter keeps the value domain at uint16, so the widening is invisible to callers |
| Leave `recvPurgeReflooded` and `deleteAt` plain, and say why in the field doc | make them atomic too, "for consistency" | consistency would be the wrong lesson: it would teach that post-publication mutation alone forces an atomic, which is the misreading that produced the bad comment in the first place. The doc instead says they stay plain PRECISELY BECAUSE no accessor exposes them |
| Delete `IsReceivedPurge` rather than keep it for symmetry | keep it and correct its doc | an exported accessor with no non-test caller is dead code (`ai/rules/wiring-completeness.md`), and keeping it would put an accessor over a LOCK-ONLY field -- exactly the thing the discipline forbids. Its two test callers are in package `lsdb` and read the field directly |
| Test the whole accessor set, not just the two atomic fields | assert only `Lifetime()` and `IsPurged()` | the discipline is a claim about every accessor. Testing only the two that were fixed would leave a new unsafe accessor to be caught by nothing |

## Known Limitations
- **`IsOwn()` is dead exported code.** `internal/plugins/isis/lsdb/entry.go:184` has zero
  callers repo-wide, including tests. It is NOT a race (`own` is written once before
  publication), so it does not affect any AC, but it is the same
  `ai/rules/wiring-completeness.md` argument that justified deleting `IsReceivedPurge` in
  `71f91c170` -- applied to one accessor and not its neighbour. Deciding its fate
  (delete, or wire the `own` flag into `Snapshot`/`show`) is outside this spec's race
  scope and is flagged for the reviewer rather than actioned here.
- **The condition-1 enumeration in the struct doc lists four fields, not seven.**
  `entry.go:58-62` names `lifetime`, `purged`, `recvPurgeReflooded` and `deleteAt`; the
  three per-circuit flag maps (`srm`, `ssn`, `srmSent`) are also mutated post-publication
  and are addressed by the following sentence ("reachable only through LSDB methods that
  take the lock themselves") rather than by the enumeration. The conclusion is correct
  and the maps are covered, but the enumeration is not exhaustive -- the same shape of
  imprecision `71f91c170` was written to remove.
- **One test reads a LOCK-ONLY field off-lock.** `aging_test.go:152,159` read
  `Lookup(...).receivedPurge` directly. Benign -- that test is single-goroutine, so there
  is no concurrent writer -- but it is the exact shape the discipline warns against, and
  it is only legal because the test is in package `lsdb`.
- **Multi-field off-lock reads are consistency-relaxed, by design.** `buildPSNP`
  (`snp.go:319-322`) takes three separate off-lock loads. Not a race and not fixed: see
  R-3.

## RFC Documentation

The ISO/IEC 10589 clause references that constrain the two atomic fields are carried
inline on the fields and their writers: clause 7.3.16/7.3.17 on `lifetime` and `purged`
(`entry.go:79`, `entry.go:108`), clause 7.3.16 on the duplicate-refresh write
(`lsdb.go:189`), and clause 7.3.16.4/7.3.17 on the tick and the grace period
(`aging.go:72`, `aging.go:96`). No clause behavior changed, so no `rfc/short/` summary
and no `docs/features/rfc-status.md` row moves.

## Implementation Summary

### What Was Implemented
- `Entry.lifetime` is an `atomic.Uint32` and `Entry.purged` an `atomic.Bool`
  (`internal/plugins/isis/lsdb/entry.go:95,115`). `Lifetime()` and `IsPurged()` read
  through atomic loads (`entry.go:167,188`); a new unexported `setLifetime`
  (`entry.go:173`) is the single write path.
- All four post-publication write sites converted: the tick decrement (`aging.go:121`),
  `markPurgedLocked` (`aging.go:146-147`), `replaceLocked` (`lsdb.go:230,232`) and the
  clause 7.3.16 duplicate refresh (`lsdb.go:190`). All in-package plain reads of the two
  fields now go through the accessors.
- The `Entry` struct carries a FIELD DISCIPLINE doc (`entry.go:40-65`) stating the two
  conditions as a conjunction and classifying every field; `recvPurgeReflooded` and
  `deleteAt` carry explicit LOCK-ONLY "do NOT add an accessor" notes
  (`entry.go:128-137`).
- `Lookup`'s doc (`lsdb.go:277-294`) replaced "callers that only read metadata are fine"
  with the explicit safe/unsafe split and a note that the old sentence is what produced
  the race.
- `IsReceivedPurge` deleted; its two test callers read the field directly.
- `TestISISLSDBEntryAccessorsAreRaceFree` (`aging_test.go:236`) drives the aging tick
  against four reader goroutines; `-race` is the oracle.

### Bugs Found/Fixed
- **The reported race** -- `Entry.lifetime` written by the aging tick under the lock and
  read by SNP generation off-lock. Fixed by the atomic; covered by
  `TestISISLSDBEntryAccessorsAreRaceFree`.
- **A false claim in the fix's own comment** -- "one of only two fields mutated AFTER the
  entry is published" (four are). Fixed by `71f91c170`; no test can cover a comment, so
  the Critical Review Checklist carries an "Enumeration honesty" row instead.
- **A fabricated accessor justification** -- `IsReceivedPurge`'s doc claimed the engine
  read it; the engine reads the field. Fixed by deleting the accessor; covered by a
  repo-wide grep in Pre-Commit Verification.
- **The regression test did not reliably gate** (found 2026-07-27 while filling this
  spec, see Deviations). Fixed by keeping the writer writing for the whole run.

### Documentation Updates
- None under `docs/`. Every row of the Documentation Update Checklist is No; the grep
  proving no source anchor points at the changed files is in Pre-Commit Verification.
  `make ze-doc-test` was therefore not required and not run.

### Deviations from Plan
- **The Task section's expected blast radius was wrong, and narrowing it was the finding.**
  It predicted `Sequence()`, `Checksum()`, `LSPID()` and the overload reader would be
  "equally unsafe siblings". They are not: `replaceLocked` publishes a fresh entry, so
  those fields are written once before the entry is reachable. Two atomics, not six.
  Recorded as A-1 broken.
- **The fix's first comment was itself wrong and needed a second commit.** `7f3bfd338`
  stated one condition where two are required and named the wrong set of
  post-publication fields; `71f91c170` restated the discipline as a conjunction, verified
  by reading every access. Recorded as A-2 broken.
- **The regression test was strengthened during this spec-fill session (2026-07-27).**
  Mutation-verification showed the committed version caught a reverted `lifetime` atomic
  in only 6 of 12 single runs and a reverted `purged` in 10 of 12: all eight LSPs reached
  the purge state within the first few tick iterations, after which `markPurgedLocked`
  early-returns and the tick wrote no entry field for the remaining ~150 ms. The writer
  now re-originates the LSPs when the tick reports them purged, and the reader loop calls
  all nine exported accessors instead of four. Both mutations are now caught 12/12. The
  test's doc comment also still carried the "only lifetime and purged are mutated after
  publication" claim that `71f91c170` corrected in `entry.go` and `lsdb.go` but not in
  the test; that surviving copy is corrected. Recorded as A-5 broken.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The reported DATA RACE on `Entry.lifetime` no longer occurs | Done | `internal/plugins/isis/lsdb/entry.go:95,167,173` (atomic field, load, store) | Proven by `TestISISLSDBEntryAccessorsAreRaceFree`, which drives the tick against four reader goroutines under `-race` |
| Fix at the discipline, not the one field | Done | `internal/plugins/isis/lsdb/entry.go:40-65` (the two-condition FIELD DISCIPLINE doc) plus the per-field classification in Design Insights | Every one of the 14 fields is classified against both conditions; `purged` was found by the discipline, not by a failing test |
| Decide between locking accessors, `...Locked` renames, or atomics | Done (Changed vs the Task's framing) | Key Design Decisions row 1 | Atomics chosen; both alternatives require restructuring every off-lock caller because `Lookup` releases the lock before returning |
| `Sequence()`, `Checksum()`, `LSPID()`, overload reader are equally unsafe | Changed | Design Insights field table | The premise was false: `replaceLocked` (`lsdb.go:216-252`) publishes a fresh entry, so those fields are written once before reachability. Recorded as A-1 broken |
| A per-field patch must not leave sibling accessors unsafe | Done | `aging_test.go:297-307` | The regression test now calls all nine exported accessors, so a future accessor over a post-publication field fails here |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestISISLSDBEntryAccessorsAreRaceFree` (`aging_test.go:236`); reader loop reads `Lifetime()` at `aging_test.go:297` and `IsPurged()` at `:298` while the writer at `:272` runs `d.Tick()`. Green under `-race -count=20` | The producing code is `entry.go:167` (atomic load) and `entry.go:173` / `aging.go:147` (atomic stores) |
| AC-2 | Done | Same test, `aging_test.go:299-307`: `Sequence`, `Checksum`, `LSPID`, `IsOverloaded`, `IsOwn`, `Raw`, `Decode` all called off-lock in the reader loop. Green under `-race -count=20` | These seven accessors were NOT exercised by the committed version of the test; that coverage was added on 2026-07-27 (see Deviations). Their safety rests on `replaceLocked` publishing at `lsdb.go:252` after every assignment |
| AC-3 | Done | Mutation verification, 2026-07-27: `lifetime` reverted to plain `uint32` → 12/12 single runs RED; `purged` reverted to plain `bool` → 12/12 single runs RED. Output in Pre-Commit Verification | Was 6/12 and 10/12 before the test was strengthened. This AC is the reason the strengthening happened rather than being recorded |
| AC-4 | Done | `internal/plugins/isis/lsdb/entry.go:40-65` states both conditions as a numbered conjunction; `entry.go:81-94` (lifetime), `:112-114` (purged), `:128-130` (recvPurgeReflooded LOCK-ONLY), `:135-137` (deleteAt LOCK-ONLY) classify the fields | Verified by reading, not by a test -- a comment has no test. See Known Limitations for the one imprecision that remains (the four-field enumeration where seven qualify) |
| AC-5 | Done | `internal/plugins/isis/lsdb/lsdb.go:277-294`: names the safe accessors, states what is not safe, and records that the earlier "callers that only read metadata are fine" wording produced the race | grep for the old sentence returns only this explanatory reference, never as guidance |
| AC-6 | Done | Repo-wide `grep -rn 'IsReceivedPurge'` returns nothing (evidence in Pre-Commit Verification). No accessor exists over `recvPurgeReflooded`, `deleteAt`, `receivedPurge`, `srm`, `ssn` or `srmSent` | `IsOwn()` still exists over `own`, which is written once before publication, so AC-6 as stated holds. Its lack of callers is a separate wiring-completeness matter recorded in Known Limitations |
| AC-7 | Done | `go test -race -count=10 -run TestISISDISElection ./internal/plugins/isis/` → `ok ... 106.058s`. Output in Pre-Commit Verification | The originally reported symptom |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestISISLSDBEntryAccessorsAreRaceFree` | Done | `internal/plugins/isis/lsdb/aging_test.go:236` | Strengthened 2026-07-27: writer re-originates purged LSPs, reader calls all nine accessors. Mutation-verified 12/12 both fields |
| `TestISISDISElection` | Done | `internal/plugins/isis/dis_wiring_test.go:115` | Pre-existing; the symptom. Green at `-race -count=10` |
| `TestISISLSDBAgeDecrement` | Done | `internal/plugins/isis/lsdb/aging_test.go:29` | Pre-existing; proves the decrement semantics survived the `setLifetime` indirection |
| `TestISISLSDBAgeToPurge` | Done | `internal/plugins/isis/lsdb/aging_test.go:44` | Pre-existing; proves purge-not-delete and the grace period survived the `purged` atomic |
| `TestISISReceivedPurgeRefloodSurfaced` | Done | `internal/plugins/isis/lsdb/aging_test.go:139` | Pre-existing; its two assertions now read `receivedPurge` directly (`aging_test.go:152,159`) after `IsReceivedPurge` was deleted. Coverage unchanged: same package, same field |
| Functional `.ci` test | N/A | - | No user-facing behavior change; see TDD Test Plan → Functional Tests |
| Interop scenario | N-A | - | No wire-visible change; see TDD Test Plan → Interop Tests |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/isis/lsdb/entry.go` | Done | Both atomics, both accessors, `setLifetime`, the FIELD DISCIPLINE doc, LOCK-ONLY field notes, `IsReceivedPurge` removed |
| `internal/plugins/isis/lsdb/aging.go` | Done | `aging.go:95,116,121,122,143` read via accessors; `:146,147` write via `setLifetime`/`Store` |
| `internal/plugins/isis/lsdb/lsdb.go` | Done | `:190,230,232` write via `setLifetime`/`Store`; `:218,359,507,510` read via accessors; `:277-294` the corrected `Lookup` doc |
| `internal/plugins/isis/lsdb/aging_test.go` | Done | The regression test plus the two direct-field reads replacing `IsReceivedPurge()` |

### Audit Summary
- **Total items:** 21 (5 Task requirements, 7 ACs, 7 TDD-plan tests, 4 files -- 2 tests counted as N/A)
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the accessor-blast-radius requirement, narrowed from six fields to two; the mechanism choice, atomics over the Task's other two options -- both recorded in Deviations)

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `TestISISDISElection` no longer fails under `-race` on the LSDB `Entry` field | Unit test under the race detector | `go test -race -count=10 -run TestISISDISElection ./internal/plugins/isis/` → `ok github.com/ze-software/ze/internal/plugins/isis 106.058s` (2026-07-27) |
| The LSDB no longer has a lock discipline its own exported accessors bypass | Unit test calling EVERY exported accessor off-lock against a live writer | `TestISISLSDBEntryAccessorsAreRaceFree` (`aging_test.go:236`): reader loop `:297-307` calls all nine accessors while `:272` ticks. `-race -count=20` → `ok ... 4.674s` |
| The fix is at the discipline, not the one field | Enumeration of all 14 fields against both conditions, plus a second field found by it | Design Insights field table. `purged` was fixed because the discipline selected it, not because a test failed on it; `recvPurgeReflooded`, `deleteAt` and the three flag maps were deliberately left plain with the reason recorded at `entry.go:128-137` |
| The fix would fail if reverted (the test is not vacuous) | Mutation verification, both fields, single runs | `lifetime` → plain `uint32`: 12/12 single runs RED. `purged` → plain `bool`: 12/12 single runs RED. Full output in Pre-Commit Verification |
| No behavior regression in aging, purge or freshness | Full package suite under the race detector | Whole tree, 2026-07-27: `go test -race ./internal/plugins/isis/...` → all 12 packages `ok` (`isis 86.047s`, `lsdb 358.443s`, plus adjacency, circuit, cli, packet, redistribute, redistribute/events, spf, transport, types, yang). Re-run of the changed package after the test was strengthened: `go test -race ./internal/plugins/isis/lsdb/` → `ok ... 222.356s` |
| No exported accessor is left over an unsafe or uncalled-for field | Repo-wide grep | `grep -rn 'IsReceivedPurge'` → no matches (the symbol, its doc and both test call sites are gone) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->

All evidence below was produced on 2026-07-27 against the working tree, independently of
the audit tables. Feature tags used throughout are
`ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u)`, per
`ai/rules/bash-output.md` (a bare `go test` drops the tags and fabricates reds).

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> — paste output. -->
| File | Exists | Evidence |
|------|--------|----------|
| (Files to Create) | N/A | The spec creates no file; every change lands in an existing one |
| `internal/plugins/isis/lsdb/entry.go` | Yes | `-rw-r--r-- 1 thomas staff 14K Jul 27 15:58 internal/plugins/isis/lsdb/entry.go` (269 lines) |
| `internal/plugins/isis/lsdb/aging.go` | Yes | `-rw-r--r-- 1 thomas staff 6.6K Jul 27 15:59 internal/plugins/isis/lsdb/aging.go` (150 lines) |
| `internal/plugins/isis/lsdb/lsdb.go` | Yes | `-rw-r--r-- 1 thomas staff 23K Jul 27 15:59 internal/plugins/isis/lsdb/lsdb.go` (561 lines) |
| `internal/plugins/isis/lsdb/aging_test.go` | Yes | `-rw-r--r-- 1 thomas staff 12K Jul 27 15:56 internal/plugins/isis/lsdb/aging_test.go` (317 lines after the 2026-07-27 strengthening) |

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit — re-check. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | tick + unlocked `Lifetime()`/`IsPurged()` is race-free | `go test -race -count=20 -run TestISISLSDBEntryAccessorsAreRaceFree ./internal/plugins/isis/lsdb/` → `ok github.com/ze-software/ze/internal/plugins/isis/lsdb 4.674s`. The atomic that makes it true: `grep -n 'atomic\.' internal/plugins/isis/lsdb/entry.go` → `20: "sync/atomic"`, `95: lifetime atomic.Uint32`, `115: purged atomic.Bool` |
| AC-2 | all seven other accessors are read off-lock in the same test | `grep -n '_ = e\.' internal/plugins/isis/lsdb/aging_test.go` → lines 297-304 (`Lifetime`, `IsPurged`, `Sequence`, `Checksum`, `LSPID`, `IsOverloaded`, `IsOwn`, `Raw`) plus `305: if lsp, err := e.Decode(); err == nil`. Nine accessors; `grep -c 'func (e \*Entry) [A-Z]' internal/plugins/isis/lsdb/entry.go` → 9. Every exported accessor is covered |
| AC-3 | reverting either atomic turns the test RED on a single run | `lifetime` → plain `uint32`: 12 separate `-test.count=1` runs, `CAUGHT 12 / 12`. `purged` → plain `bool`: 12 separate runs, `CAUGHT 12 / 12`. Sample race report from the pre-strengthening measurement: `WARNING: DATA RACE / Write at 0x00c0003a9b44 by goroutine 12: (*Entry).setLifetime() entry.go:173 ← tickLevelLocked() aging.go:121 / Previous read at 0x00c0003a9b44 by goroutine 16: (*Entry).Lifetime() entry.go:168`. Production code restored after each mutation: `git diff --stat -- internal/plugins/isis/lsdb/entry.go internal/plugins/isis/lsdb/aging.go internal/plugins/isis/lsdb/lsdb.go` → empty |
| AC-4 | the struct doc states both conditions and classifies every field | `sed -n '40,65p' internal/plugins/isis/lsdb/entry.go` shows the numbered conjunction ("A field needs an atomic when BOTH of these are true", items 1 and 2) and the classification sentence naming which fields stay plain and why. `grep -n 'LOCK-ONLY' internal/plugins/isis/lsdb/entry.go` → 128 (`recvPurgeReflooded`), 135 (`deleteAt`) |
| AC-5 | `Lookup`'s doc names what is and is not safe | `sed -n '277,294p' internal/plugins/isis/lsdb/lsdb.go`: "What is safe to call on it without the LSDB lock: the metadata accessors ... What is NOT safe: mutating anything, and reading any other mutable field directly rather than through an accessor", and the record that the earlier wording "was the invitation that produced a DATA RACE" |
| AC-6 | no exported accessor over a non-atomic post-publication field | `grep -rn 'IsReceivedPurge' .` → no matches anywhere in the repository (source, tests, docs, plan). Accessor inventory cross-checked against the field table: the nine accessors read `id`, `sequence`, `lifetime`, `checksum`, `typeBlock`, `own`, `purged`, `raw` -- of which only `lifetime` and `purged` are post-publication, and both are atomic. No accessor reaches `recvPurgeReflooded`, `deleteAt`, `receivedPurge`, `srm`, `ssn` or `srmSent` |
| AC-7 | the reported symptom passes | `go test -race -count=10 -run TestISISDISElection ./internal/plugins/isis/` → `ok github.com/ze-software/ze/internal/plugins/isis 106.058s` |

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the test exist AND does it exercise the full path? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| aging tick concurrent with SNP generation | none (`.ci` is N/A -- no user-facing surface); the wiring test is `TestISISLSDBEntryAccessorsAreRaceFree`, `internal/plugins/isis/lsdb/aging_test.go:236` | Yes -- read the test body, not just its name: the writer goroutine (`:266-281`) calls `d.Tick()`, which is the same `LSDB.Tick` the engine drives from `lsdb_wiring.go:78,96`; the reader goroutines (`:285-312`) call `d.Lookup` then read the accessors after the read lock has been released, which is the same shape as `snp.go:313-322` |
| every exported `Entry` accessor | none (`.ci` N/A); `aging_test.go:297-307` | Yes -- all nine accessors are called in the reader loop; counted against `grep -c 'func (e \*Entry) [A-Z]' internal/plugins/isis/lsdb/entry.go` → 9 |
| the originally reported symptom | none (`.ci` N/A); `internal/plugins/isis/dis_wiring_test.go:115` | Yes -- `TestISISDISElection` exercises DIS election, which reaches `Entry` metadata through the engine's own wiring; it is the test whose `-race` failure opened this spec |

### Assumptions Resolved
<!-- For EVERY A-N row in Risks & Assumptions: final status with evidence. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | **broken** | `sed -n '216,258p' internal/plugins/isis/lsdb/lsdb.go`: `replaceLocked` assigns `id`, `raw`, `sequence`, `checksum`, `typeBlock`, `own` in the composite literal at `:222-229` and `receivedPurge`/`deleteAt` at `:238-239`, all before publication at `:252`. Grep confirms no other assignment to any of them: `grep -n '\.\(sequence\|checksum\|typeBlock\|own\|receivedPurge\)\b' internal/plugins/isis/lsdb/*.go` shows only reads outside `replaceLocked`. Mistake Log row 1, Deviations bullet 1 |
| A-2 | **broken** | `grep -n 'recvPurgeReflooded\|deleteAt' internal/plugins/isis/lsdb/*.go` → writes at `aging.go:113`, `aging.go:148`, `lsdb.go:239`, all after publication. Plus `srm`/`ssn`/`srmSent` mutated at `lsdb.go:385,419,440,449,472-474`. Seven fields satisfy condition 1, not two. Mistake Log row 2 |
| A-3 | **confirmed** | The full 14-row field table in Design Insights, built from `grep`-verified write sites and the nine-accessor inventory. Exactly `lifetime` and `purged` satisfy both conditions; exactly those two are atomic (`entry.go:95,115`) |
| A-4 | **confirmed (production), one benign test exception** | `grep -rn ') \*Entry\|(\*Entry,\|\[\]\*Entry' internal/plugins/isis/ --include='*.go'` excluding tests → one hit, `lsdb.go:295` (`Lookup`). So `Lookup` is the only producer of a live pointer. All production reads through it use exported accessors (call sites enumerated in Design Insights). The exception is `aging_test.go:152,159`, an in-package single-goroutine test read of `receivedPurge`; recorded in Known Limitations |
| A-5 | **broken** | Measured 2026-07-27 with the committed test and `lifetime` reverted to plain `uint32`: `CAUGHT 6 / 12` single runs. With `purged` reverted: `CAUGHT 10 / 12`. The doc claim "fails deterministically" was false. After strengthening: 12/12 for both. Mistake Log row 3, Deviations bullet 3 |

### Documentation Verified
<!-- For EVERY Yes in Documentation Update Checklist: verify the edited doc claim against source. -->
<!-- For EVERY No: paste the grep/source check that proves no doc update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No `docs/` page carries a source anchor pointing at any changed file | `grep -rn 'source:.*isis/lsdb/\(entry\|aging\|lsdb\)\.go' docs/` → no matches. Nothing under `docs/` claims anything about these three files, so no anchor can be stale | Yes |
| No doc mentions the deleted `IsReceivedPurge` accessor | `grep -rn 'IsReceivedPurge' .` → no matches repo-wide, which covers `docs/`, `ai/` and `plan/` as well as source | Yes |
| No RFC status row changes (checklist row 9 = No) | Clause 7.3.16/7.3.17 behavior is preserved, not extended: `TestISISLSDBAgeDecrement`, `TestISISLSDBAgeToPurge` and `TestISISReceivedPurgeRefloodSurfaced` all pass unchanged. The isis tree does carry many RFC-tagged tests, but none in the one test file this work touched: `grep -n 'RFC requirement:' internal/plugins/isis/lsdb/aging_test.go` → no matches, so no ledger `file:line` shifts. Confirmed mechanically: `make ze-rfc-check` → `rfc-requirements OK: 2720 gated MUST-level requirement(s) across 166 enrolled RFC(s); 2560 test tag(s) resolved` (exit 0), i.e. `ai/RFC-REQUIREMENTS.md` is not stale and needs no regeneration | Yes |
| No architecture doc describes the LSDB lock model (checklist row 12 = No) | The `// Design:` target of all three changed files is `plan/learned/932-isis-6-lsdb.md`, a learned summary, not a `docs/architecture/` page. The field-level discipline is documented in `entry.go` itself, which is where a developer adding a field reads | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-verify` passes (the pre-commit gate, `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
