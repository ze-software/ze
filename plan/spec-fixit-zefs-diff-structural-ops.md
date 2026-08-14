# Spec: fixit-zefs-diff-structural-ops

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-implement at stage 10.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/2 |
| Deferral shard | - |
| Updated | 2026-08-14 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** Under zefs blob storage the pending-change review UI under-reports
structural operations. After deleting a list entry in the web editor, `/config/diff`
renders `Review changes (1)` with an **empty** body. The identical operation under
filesystem storage correctly yields `- bgp peer london` and a count of 2. Measured
both ways on 2026-07-27.

**Producer.** `Editor.listChangeFiles` (`internal/component/cli/editor.go`)
calls `store.List(filepath.Dir(originalPath))`. For blob storage that resolves through
`blobStorage.List` -> `ReadDir(resolveKey(prefix))`
(`internal/component/config/storage/blob.go`), which does not surface the
per-user change file the structural ops live in.

**Why leaf edits look fine.** A leaf edit is ALSO recorded in the in-memory `e.meta`,
so the diff finds it by another route. Structural ops (`delete-entry`,
`delete-container`, `rename`) live ONLY in the change file, so they vanish from the
review UI while still applying correctly at commit.

**Blast radius.** `/config/rename/` reaches the same mechanism
(`writeThroughDeleteNamed`/rename) and is affected identically. The web editor is
ALWAYS in session mode (`internal/component/web/editor.go` calls `SetSession`
unconditionally), so every web user is exposed; the TUI in file mode is not.

**Severity.** The delete itself is correct -- tree, peer list and commit all land.
This is a review-surface defect: an operator reviewing pending changes before commit
is shown an incomplete picture of what they are about to apply. That is a
trust-in-the-review-step problem, not a data-loss one.

**Goal.** `/config/diff` under blob storage must show every pending structural op,
matching filesystem storage exactly.

**Provenance.** Found 2026-07-27 while fixing the web editor's list-entry delete
(handover 19 item 4). Deliberately not expanded into: it is a storage/editor-layer
defect with its own test surface, and it predates that change.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - the `Design:` anchor of both storage files
  → Decision: the blob is FLAT inside each namespace. `file/active/<basename>` holds
    every config file, whatever directory the caller's path names.
  → Constraint: a key is a namespace plus a basename, so the directory part of a
    caller path carries no information after `resolveKey`.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is config storage, no wire protocol.

**Key insights:** (minimal context to resume after compaction)
- A blob key is a FILE key or a DIRECTORY key, and `zefs.BlobStore.ReadDir` accepts
  only the second. `KeyEntry.Dir()` (`pkg/zefs/registry.go`) is the directory form.
- `zefs.KeyEntry.Key()` builds the file form and is what `resolveKey` calls.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/storage/blob.go` - `List` resolved its prefix with
  `resolveKey`, which builds a FILE key: a directory prefix lost every path element
  but the last and was looked up under `file/active`. `resolvePathToKey` had two
  branches returning the same expression, and a doc comment stating the opposite of
  what both did.
- [ ] `internal/component/config/storage/storage.go` - `Storage.List(prefix)` takes a
  DIRECTORY and returns immediate children. `filesystemStorage.List` is `os.ReadDir`.
- [ ] `pkg/zefs/store.go`, `pkg/zefs/tree.go` - `ReadDir` walks the key as a path and
  returns `fs.ErrNotExist` when the node is missing or is a leaf.
- [ ] `internal/component/cli/editor.go` - `listChangeFiles` lists
  `filepath.Dir(originalPath)` and filters the result by the `<config>.change.` prefix.
- [ ] `internal/component/cli/editor_commands.go` - `writeThroughDeleteListEntry` and
  `writeThroughDeleteNamed` record a structural op in the change file ONLY.

**Behavior to preserve:**
- `List` returns full namespaced keys, so a result round-trips to `ReadFile`/`Remove`.
- An already-namespaced prefix (`file/active`, `file/draft`, `meta/ssh`) lists that
  directory. `cmdLsWithStorage` and `doSelectConfig` depend on it.
- Filesystem storage is untouched.

**Behavior to change:**
- A filesystem directory prefix now lists `file/active` instead of failing.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator deletes a list entry, container, or list in the config editor (web
  editor, or the TUI over SSH), then opens the pending-change review.
- The editor is in session mode, so the delete is a structural op.

### Transformation Path
1. `Editor.DeleteListEntry` calls `writeThroughDeleteListEntry`
   (`internal/component/cli/editor_commands.go`), which appends a `StructuralOp` to
   the per-user change file `<config>.change.<user>` and removes the entry in memory.
2. The operator asks for the review. `Editor.PendingChanges`
   (`internal/component/cli/editor.go`) merges in-memory session entries with what
   the change files hold.
3. `listChangeFiles` calls `Storage.List(filepath.Dir(originalPath))` to find those
   change files.
4. `blobStorage.List` (`internal/component/config/storage/blob.go`) resolves the
   prefix to a blob key and calls `zefs.BlobStore.ReadDir`.
5. `ReadDir` walks the key in the blob tree and returns the immediate children.
6. `PendingChanges` returns the merged list, which `/config/diff`
   (`internal/component/web/editor.go`) and `show | changes`
   (`internal/component/cli/model_commands_session.go`) render.

Stage 4 was the defect: the prefix resolved to a FILE key, stage 5 answered
`fs.ErrNotExist`, and stages 3 and 2 read that as "no change files".

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor ↔ Storage | `Storage.List(prefix)`, a directory path, returning full keys | Yes: `TestBlobStorageListFilesystemDirectory` |
| Storage ↔ zefs | `BlobStore.ReadDir(key)`, a directory key | Yes: `TestResolveDirKey` |

### Integration Points
- `zefs.KeyFileActive.Dir()` - the directory form of the namespace helper, already
  used by `cmdLsWithStorage`. `resolveDirKey` reuses it rather than spelling
  `"file/active"`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix is inside the storage backend that owns the path-to-key mapping. No caller learns a key shape |
| No unintended coupling (components stay isolated) | Yes | `internal/component/cli` is unchanged. The editor still passes a filesystem directory |
| No duplicated functionality (extends existing, does not recreate) | Yes | `resolveDirKey` reuses `pathToKey`, `isNamespaced`, and `zefs.KeyFileActive.Dir()` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No buffer or wire path |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | Yes | The namespace comes from the zefs key registry, not a literal |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every non-namespaced name written through blob storage lands under `file/active`, so that one directory holds everything a filesystem prefix could list | `resolveKey` and `migrateExistingFiles` in `internal/component/config/storage/blob.go` both build `file/active/<basename>` | Widening `List` would return files from a directory the caller did not ask about | Read both producers; `TestBlobStorageFilePrefix` asserts the written key | confirmed |
| A-2 | Only three non-test callers reach `Storage.List`, and two already pass a namespaced directory | `gopls references` on the interface method and on `blobStorage.List` | A caller relying on the old base-name reduction would break | `cmdLsWithStorage` passes `zefs.KeyFileActive.Dir()`, `cmdEditWithStorage` passes `file/active` to `selectConfig`, and `Editor.listChangeFiles` passes `filepath.Dir` | confirmed |
| A-3 | Nothing deletes files from a `List` result, so a wider result cannot destroy operator data | `listChangeFiles` results are read by `SessionChanges` and `PendingChanges` only; `DisconnectSession` rebuilds its path from `ChangePath` | A widened `List` could feed a delete | Read every consumer in `internal/component/cli/editor.go` and `editor_commit.go` | confirmed |
| A-4 | The `.et` runner could reach the defect | It builds editors with `cli.NewEditor`, which is filesystem-only | No `.et` can prove the fix, and the proof moves to `test/web/` | Read `newHeadlessModel` in `internal/component/cli/testing/headless.go` | broken: the runner needed the blob-storage option this spec adds |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `List` returning more entries changes `doSelectConfig`'s prompt for `ze config edit` | The config selection lists unexpected files | Not reached: `cmdEditWithStorage` passes `file/active`, which was already correct and is unchanged |
| R-2 | A caller lists a FILE path and now gets the whole `file/active` directory instead of an error | A caller treats a config path as a directory | No caller does. The blob discards the directory part on write, so the backend cannot tell the two apart; the behavior is stated in `resolveDirKey`'s comment |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The pending-change review. A `List` that returns too little hides operator changes (the defect); one that returns too much shows another config's files in the review. Neither reaches a delete path (A-3), so no stored data is at risk |
| How is it reverted? | Single commit revert. No data is rewritten: the change is read-side only, and the blob layout is untouched |
| Who else touches this path? | `ze config ls` and `ze config edit`'s config selection are the other two `Storage.List` callers, both already passing a namespaced directory |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `delete bgp peer <name>` in a session-mode editor on blob storage | → | `Editor.PendingChanges` → `listChangeFiles` → `blobStorage.List` → `resolveDirKey` | `test/editor/session/diff-structural-op-blob.et` |
| `Storage.List` with a filesystem directory prefix | → | `blobStorage.List` | `TestBlobStorageListFilesystemDirectory` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator deletes a list entry in a session-mode editor backed by blob storage, then reads the pending-change review | The review counts and shows that delete, as filesystem storage does |
| AC-2 | The same, read through the all-sessions view | The delete is attributed to its session and counted there too |
| AC-3 | `Storage.List` is called with a filesystem directory prefix on blob storage | It returns the config files, as full namespaced keys that round-trip to `ReadFile` |
| AC-4 | `Storage.List` is called with an already-namespaced directory prefix | It lists that directory, unchanged from today |
| AC-5 | An `.et` test asks for blob-backed config storage | The editor under test reads and writes a zefs blob, as the daemon does |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Deletes a BGP peer in the editor, then reviews pending changes before commit | editor delete -> change file -> `PendingChanges` -> `listChangeFiles` -> `blobStorage.List` -> `ReadDir` -> review | `test/editor/session/diff-structural-op-blob.et` |
| 2 | Reviews what every session has pending | `ActiveSessions` -> `PendingChanges` -> the same chain | `test/editor/session/diff-structural-op-blob.et`, the `show \| changes all` step |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBlobStorageListFilesystemDirectory` | `internal/component/config/storage/blob_test.go` | AC-3: a directory prefix lists the config files, returned as full keys | pass |
| `TestResolveDirKey` | `internal/component/config/storage/blob_test.go` | AC-3, AC-4: directory prefix, bare filename, absolute file path, and the three namespaced forms | pass |
| `TestResolveKeyIdempotent` | `internal/component/config/storage/storage_test.go` | The file-key mapping is unchanged by this spec | pass |
| `TestRunnerBlobStorage` | `internal/component/cli/testing/runner_test.go` | AC-5: the option gives the editor a blob-backed store | pass |
| `TestRunnerUnknownStorageBackendFails` | `internal/component/cli/testing/runner_test.go` | An unrecognized backend name stops the test rather than falling back | pass |
| `TestRunnerBlobStorageRefusesFileExpectation` | `internal/component/cli/testing/runner_test.go` | `expect=file:` with blob storage stops the test | pass |

### Boundary Tests (numeric inputs)
No numeric input. The unit is a key string, and its cases are enumerated in
`TestResolveDirKey`.

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `diff-structural-op-blob` | `test/editor/session/diff-structural-op-blob.et` | AC-1, AC-2, AC-5: the operator deletes a peer on blob storage and the review counts it | pass |

### Interop Tests (Scope: protocol)
Not applicable. Nothing wire-visible changes; this is config storage.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/config/storage/blob.go` - `List` resolves a DIRECTORY key;
  new `resolveDirKey`; `resolvePathToKey` loses its dead branch, its false comment,
  and the parameter no branch read; `resolveKey` loses the same parameter
- `internal/component/config/storage/pointer.go` - one `resolvePathToKey` call site
- `internal/component/config/storage/storage_test.go` - `resolveKey` call sites
- `internal/component/cli/testing/headless.go` - both headless constructors take the
  storage backend and build the editor with `NewEditorWithStorage`
- `internal/component/cli/testing/headless_test.go` - the constructor call sites
- `internal/component/cli/testing/runner.go` - `option=storage:value=blob` creates one
  blob store, shared by every model the test builds
- `ai/rules/points/testing/editor-tests-et-format/every-et-directive-and-what-it-does.md`
  and the rendered `ai/rules/testing.md` - the new directive
- `docs/functional-tests.md` - the same directive in the editor-test table

## Files to Create
- `internal/component/config/storage/blob_test.go` - has the two new unit tests
  (the file already existed)
- `test/editor/session/diff-structural-op-blob.et` - the operator-altitude test

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface changes. The fix is inside a storage backend |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No command, flag, or exit code changes |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new value source |
| Functional test for new RPC/API | Yes | `test/editor/session/diff-structural-op-blob.et` |
| Pipe completeness | No | `show \| changes` already routes through the pipe machinery, unchanged |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | No | No new file path, socket, port, or binary. The blob store and its location are unchanged |
| Prometheus counters/metrics | No | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A defect fix: the review surface now shows what it always claimed to show |
| 2 | Config syntax changed? | No | No syntax change |
| 3 | CLI command added/changed? | No | `show \| changes` behavior is corrected, not changed |
| 4 | API/RPC added/changed? | No | No API change |
| 5 | Plugin added/changed? | No | No plugin involved |
| 6 | Has a user guide page? | No | The review UI is documented by behavior, and the documented behavior is what now happens |
| 7 | Wire format changed? | N-A | No wire surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs config storage |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and the `.et` directive point file both carry `option=storage:value=blob` |
| 11 | Affects daemon comparison? | No | No feature claim changes |
| 12 | Internal architecture changed? | No | `docs/architecture/zefs-format.md` describes the flat namespace, which this fix relies on rather than alters |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/functional-tests.md` anchors `internal/component/cli/testing/parser.go`; the `.et` option table there is updated. No anchor names `blob.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The `.et` examples in the docs stay valid; the new option is additive |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- make the operator's surface reachable from a test
   - Tests: `test/editor/session/diff-structural-op-blob.et`, `TestBlobStorageListFilesystemDirectory`
   - Files: `internal/component/cli/testing/headless.go`, `runner.go`, `headless_test.go`,
     `internal/component/config/storage/blob_test.go`
   - Verify: the `.et` runs the editor on a real blob store and FAILS on the review
     count, reproducing what the operator sees
2. **Phase: Fix the key resolution** -- resolve a List prefix to a directory key
   - Tests: the two above, plus `TestResolveDirKey`
   - Files: `internal/component/config/storage/blob.go`, `pointer.go`, `storage_test.go`
   - Verify: both tests pass; reverting `List` to `resolveKey` returns them to red

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a named test in the TDD plan, and each one passes |
| Feature completeness | Both user stories run through the `.et`, no stubbed step |
| Correctness | `List` returns full namespaced keys that round-trip to `ReadFile`; an already-namespaced prefix still lists that directory and nothing else |
| No wider result than the write mapping | The directory a filesystem prefix lists is the one `resolveKey` writes into, and nothing else |
| Data flow | `internal/component/cli` learns no blob key shape. The whole fix lives in the storage backend |
| Rule: `ai/rules/stale-comments.md` | No comment describes the removed branch or the removed parameter, and `resolveKey`'s comment states what it now does |
| Rule: `ai/rules/testing.md` | The `.et` goes red when the fix is reverted, and the option it needs is documented where the other `.et` options are |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| A List prefix resolves to a directory key | `make ze-test-pkg PKG=./internal/component/config/storage` |
| The operator's review shows a structural delete on blob storage | `make ze-editor-test` |
| `.et` tests can ask for blob-backed storage | `grep -n "option=storage" test/editor/session/diff-structural-op-blob.et docs/functional-tests.md ai/rules/testing.md` |
| No dead branch or false comment survives in the key resolution | Read `resolvePathToKey` and `resolveKey` in `internal/component/config/storage/blob.go` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Path traversal through a List prefix | A prefix reaching `ReadDir` is either an already-namespaced key or the fixed `file/active` constant. A caller-supplied `../` never reaches the blob tree, because a non-namespaced prefix is discarded, not joined |
| Namespace escape | A prefix starting `meta/` still lists metadata, exactly as before this change. `List` is read-only and exposes no value, only key names |
| Cross-user disclosure | `List` returns every change file, as it does on filesystem storage. `listChangeFiles` filters to this config, and the review already shows other sessions' pending changes by design (`show \| changes all`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- A blob key has two shapes, and one function was serving both. `resolveKey` builds a
  FILE key; `ReadDir` needs a DIRECTORY key. The registry already carried both forms
  (`KeyEntry.Key` and `KeyEntry.Dir`), so the defect was a missing distinction in the
  storage backend, not a missing capability in zefs.
- Every existing test of `blobStorage.List` passed an already-namespaced prefix, which
  is the one shape the code got right. The defect lived under complete-looking
  coverage because no test used the shape the production caller uses.
- The whole `.et` suite ran on filesystem storage while the daemon runs on a blob.
  A defect reachable only under blob storage was invisible to 164 editor tests.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix in `blobStorage.List` | Fix in `Editor.listChangeFiles` by passing a key the blob understands | The editor would have to know blob key shapes. The path-to-key mapping belongs to the storage backend, and the same wrong prefix would still fail for the next caller |
| Fix in `blobStorage.List` | Change `resolvePathToKey` so a directory survives | Every read and write flows through it. A directory-shaped key under `file/active` would create nested keys the blob layout does not use, and would change where files are stored |
| A filesystem prefix maps to `file/active` | Reject a non-namespaced prefix with an error | The three callers include one that legitimately passes a filesystem directory, so an error would leave the defect in place |
| `option=storage:value=blob` in the `.et` runner | A Go test in `internal/component/cli` driving the editor directly | The operator meets this through the editor, and `ai/rules/testing.md` requires the functional test to run through the entry point. The option is also what lets any future blob-only editor defect be tested at all |
| Both headless constructors take a storage backend | A third constructor for the blob case | Three constructors with the same body is machinery; passing `NewFilesystem()` keeps one shape per model kind |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- A `List` prefix naming a FILE returns the `file/active` directory instead of an
  error. The blob discards the directory part of a path on write, so the backend
  cannot tell a file path from a directory path. No caller does this.
- An `.et` using `option=storage:value=blob` cannot assert with `expect=file:`. The
  editor writes the blob, so the temp directory keeps its migrated state and such an
  expectation would assert against content nothing wrote. The runner refuses the pair
  rather than let a test prove the wrong thing
  (`TestRunnerBlobStorageRefusesFileExpectation`).

## RFC Documentation (Scope: protocol)

Not applicable. No RFC governs ze's config storage.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
