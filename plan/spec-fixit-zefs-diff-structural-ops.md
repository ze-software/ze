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
| Phase | 2/2 |
| Deferral shard | - |
| Updated | 2026-08-19 |

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
| `TestRunnerBlobStorageWritesBlobNotFile` | `internal/component/cli/testing/runner_test.go` | AC-5: after a commit the blob holds the edit and the config file on disk does not. Added from the review; it is the assertion the three rows above cannot make | pass |

### Boundary Tests (numeric inputs)
No numeric input. The unit is a key string, and its cases are enumerated in
`TestResolveDirKey`.

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `diff-structural-op-blob` | `test/editor/session/diff-structural-op-blob.et` | AC-1, AC-2, AC-5: the operator deletes a peer on blob storage and the review counts it and shows it | pass |
| `diff-structural-op-filesystem` | `test/editor/session/diff-structural-op-filesystem.et` | AC-1: the same delete on filesystem storage, which is what "matching filesystem storage exactly" compares against. Added from the review | pass |

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
| A List prefix resolves to a directory key | `make ze-unit-pkg-test PKG=./internal/component/config/storage` |
| The operator's review shows a structural delete on blob storage | `make ze-functional-editor-test` |
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
- [ ] `make ze-precommit-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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

---

## Implementation Summary

### What Was Implemented

Landed in commit `85a79ac9b` (fix(config): show structural ops in the diff an
operator reviews):

- `internal/component/config/storage/blob.go` - `List` resolves its prefix with
  the new `resolveDirKey`, which returns an already-namespaced prefix unchanged
  and maps every other prefix to `zefs.KeyFileActive.Dir()`. `resolvePathToKey`
  lost the branch both halves of which returned the same expression, the comment
  that stated the opposite of what it did, and the parameter no branch read;
  `resolveKey` lost the same parameter.
- `internal/component/config/storage/pointer.go`,
  `internal/component/config/storage/storage_test.go` - the call sites of that
  parameter.
- `internal/component/cli/testing/headless.go` - both headless constructors take
  a `storage.Storage` and build the editor with `NewEditorWithStorage`.
- `internal/component/cli/testing/runner.go` - `option=storage:value=blob`
  creates one `storage.NewBlob` shared by every model the test builds, refuses
  `expect=file:` beside it, and fails on an unknown backend name.
- `test/editor/session/diff-structural-op-blob.et`,
  `internal/component/config/storage/blob_test.go` - the tests.
- `docs/functional-tests.md` and
  `ai/rules/points/testing/editor-tests-et-format/every-et-directive-and-what-it-does.md`
  - the new `.et` option, documented where the other options are.

Added on 2026-08-19, answering an independent review of that commit:

- `internal/component/cli/testing/runner.go` - `runTestCase` splits into a
  wrapper that owns the temp directory and `runTestCaseIn`, which runs the case
  in a directory the caller owns. Nothing else changed: the wrapper creates the
  directory, removes it, and delegates.
- `internal/component/cli/testing/runner_test.go` -
  `TestRunnerBlobStorageWritesBlobNotFile` commits an edit through the runner and
  then reads both stores, so the test says WHERE the edit landed.
- `test/editor/session/diff-structural-op-blob.et` - the review must SHOW the
  delete, not only count it: `expect=viewport:contains=peer peer1`.
- `test/editor/session/diff-structural-op-filesystem.et` - the counterpart run
  of the same delete on filesystem storage, so "matching filesystem storage
  exactly" rests on a test rather than on code symmetry.

### Bugs Found/Fixed

- `blobStorage.List` resolved a directory prefix to a FILE key, so `ReadDir`
  answered `fs.ErrNotExist` and `Editor.listChangeFiles` read that as "no change
  files". Every structural op (`delete-entry`, `delete-container`, `rename`)
  vanished from the pending-change review under blob storage, which is every web
  editor user. Covered by `TestBlobStorageListFilesystemDirectory`,
  `TestResolveDirKey` and `test/editor/session/diff-structural-op-blob.et`.
- The proof of AC-5 did not discriminate. `TestRunnerBlobStorage` asserted the
  run passed and the editor went dirty, and the `.et` passed under filesystem
  storage too, so dropping `configStore = blobStore` in `runTestCaseIn` left
  both green. Covered by `TestRunnerBlobStorageWritesBlobNotFile`, whose three
  assertions all redden under that mutation (see Pre-Commit Verification).

### Documentation Updates

- `docs/functional-tests.md` - the editor-test option table carries
  `option=storage:value=blob`. Committed in `85a79ac9b`; its source anchor is
  `internal/component/cli/testing/parser.go`, which still parses that option.
- `ai/rules/points/testing/editor-tests-et-format/every-et-directive-and-what-it-does.md`
  and the rendered `ai/rules/testing.md` - the same option.
- `docs/guide/config-editor.md` carries the operator-facing statement, under
  "Structural operations appear in the diff", with source anchors on
  `resolveDirKey` and `blobStorage.List`. It reached the tree through the
  doc-sync commit `ee449fdfa`, not through `85a79ac9b`.
- Nothing was owed for the 2026-08-19 additions: they add no directive and no
  option, and they change no symbol a doc anchor names.
  `grep -rn "option=storage" docs/ ai/rules/` returns the three locations above
  (the point file, its rendered rule, and the editor-test table), and none of
  them lists the individual `.et` files.

### Deviations from Plan

- A-4 broke. The `.et` runner could not reach the defect as it stood, so the
  spec's own Files to Modify list grew the runner and both headless
  constructors. The Mistake Log below carries the row.
- Files to Create gained `test/editor/session/diff-structural-op-filesystem.et`,
  which the plan did not name. AC-1 claims the blob review matches filesystem
  storage, and no test stated what filesystem storage does.
- `internal/component/cli/testing/runner.go` gained `runTestCaseIn`. The plan
  named the file for the storage option only. A test that proves which store the
  runner wrote has to read the store after the run, and `runTestCase` removes
  its temp directory on the way out.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 assumed the `.et` runner could reach the defect as it stood | `newHeadlessModel` built its editor with `cli.NewEditor`, which is filesystem-only, so no `.et` could run on a blob and 164 editor tests ran on a backend the daemon does not use | Read `newHeadlessModel` in `internal/component/cli/testing/headless.go` during step 3 assumption validation, before feature code | Gave both headless constructors a `storage.Storage` parameter and added `option=storage:value=blob` to the runner, rather than moving the proof to `test/web/`. Cost: the runner, both constructors and their call sites entered a spec scoped to a storage backend |
| approach | The first proof of AC-5 asserted that a blob-backed run passed and went dirty | A pass and a dirty flag are produced by the editor whichever store it holds. Both assertions, and the `.et` beside them, survive `configStore = blobStore` being dropped | An independent review of `85a79ac9b` read the test against the mutation | Added `TestRunnerBlobStorageWritesBlobNotFile`, which commits and then reads both stores. The mutation was made and all three assertions reddened |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The pending-change review under blob storage shows every pending structural op | Done | `resolveDirKey`, `(*blobStorage).List` in `internal/component/config/storage/blob.go` | The change file is found, so `Editor.PendingChanges` merges it |
| It matches filesystem storage exactly | Done | `test/editor/session/diff-structural-op-blob.et` and `test/editor/session/diff-structural-op-filesystem.et` | The same delete, the same three assertions, one option apart |
| `/config/rename/` reaches the same mechanism and is fixed with it | Done | `Editor.listChangeFiles` in `internal/component/cli/editor.go` | One producer serves both: the rename op is recorded in the same change file the fixed `List` now returns |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/editor/session/diff-structural-op-blob.et`, steps `show \| changes` | `expect=status:contains=1 pending change` counts it; `expect=viewport:contains=peer peer1` shows it, and the name can reach the viewport only as a removed line because the working config no longer holds it |
| AC-2 | Done | The same file, `show \| changes all` | `1 pending change across 1 sessions` |
| AC-3 | Done | `TestBlobStorageListFilesystemDirectory` | Full namespaced keys that round-trip to `ReadFile` |
| AC-4 | Done | `TestResolveDirKey` | The three namespaced forms pass through unchanged |
| AC-5 | Done | `TestRunnerBlobStorageWritesBlobNotFile`, `TestRunnerBlobStorage`, `TestRunnerUnknownStorageBackendFails`, `TestRunnerBlobStorageRefusesFileExpectation` | The first is the one that separates a blob-backed editor from a filesystem-backed one |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBlobStorageListFilesystemDirectory` | Done | `internal/component/config/storage/blob_test.go` | pass |
| `TestResolveDirKey` | Done | `internal/component/config/storage/blob_test.go` | pass |
| `TestResolveKeyIdempotent` | Done | `internal/component/config/storage/storage_test.go` | pass |
| `TestRunnerBlobStorage` | Done | `internal/component/cli/testing/runner_test.go` | pass |
| `TestRunnerUnknownStorageBackendFails` | Done | `internal/component/cli/testing/runner_test.go` | pass |
| `TestRunnerBlobStorageRefusesFileExpectation` | Done | `internal/component/cli/testing/runner_test.go` | pass |
| `TestRunnerBlobStorageWritesBlobNotFile` | Changed | `internal/component/cli/testing/runner_test.go` | Added after the plan, from the review. It is the discriminating proof of AC-5 |
| `diff-structural-op` (blob) | Done | `test/editor/session/diff-structural-op-blob.et` | pass |
| `diff-structural-op` (filesystem) | Changed | `test/editor/session/diff-structural-op-filesystem.et` | Added after the plan, from the review. It states what AC-1 compares against |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/storage/blob.go` | Done | `resolveDirKey` added; the dead branch, the false comment and the unread parameter removed |
| `internal/component/config/storage/pointer.go` | Done | Call site updated |
| `internal/component/config/storage/storage_test.go` | Done | Call sites updated |
| `internal/component/cli/testing/headless.go` | Done | Both constructors take the backend |
| `internal/component/cli/testing/headless_test.go` | Done | Call sites updated |
| `internal/component/cli/testing/runner.go` | Changed | The storage option, plus the `runTestCaseIn` seam the review's proof needs |
| `ai/rules/points/testing/editor-tests-et-format/...` and `ai/rules/testing.md` | Done | The directive row |
| `docs/functional-tests.md` | Done | The same row in the editor-test table |
| `internal/component/config/storage/blob_test.go` | Done | Two new unit tests |
| `test/editor/session/diff-structural-op-blob.et` | Done | Plus the viewport assertion added on 2026-08-19 |
| `test/editor/session/diff-structural-op-filesystem.et` | Changed | Not in the plan; added from the review |

### Audit Summary
- **Total items:** 23 (3 requirements, 5 ACs, 9 tests, 11 files, counted once each where a row repeats)
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations: the extra `.et`, the extra unit test, the `runTestCaseIn` seam)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The pending-change review under blob storage shows every pending structural op | functional | `test/editor/session/diff-structural-op-blob.et`: an operator deletes `bgp peer peer1` on blob storage, `show \| changes` answers `1 pending change` and the viewport carries the removed entry. Run through `make ze-unit-pkg-test PKG=./internal/component/cli/testing`. It discriminates: forcing `changesEnabled` false in `(*Model).setViewportData` (`internal/component/cli/model_render.go`) reddens it with `viewport does not contain "peer peer1"` |
| It matches filesystem storage exactly | functional | `test/editor/session/diff-structural-op-filesystem.et`: the same delete, the same three assertions, and the only difference between the two files is `option=storage:value=blob`. Both redden together under the same mutation |
| An `.et` can ask for blob-backed config storage, so a blob-only editor defect is testable at all | unit | `TestRunnerBlobStorageWritesBlobNotFile`: after a commit the config file in the run directory still holds `1.2.3.4` and the blob holds `5.6.7.8`. Dropping `configStore = blobStore` in `runTestCaseIn` reverses both and reddens all three assertions (output in Pre-Commit Verification) |
| A directory prefix lists the config files on blob storage | unit | `TestBlobStorageListFilesystemDirectory` and `TestResolveDirKey`, `ok github.com/ze-software/ze/internal/component/config/storage 155.017s` (uncached, 2026-08-19) |

**What this evidence does NOT cover.** The Task states the symptom on
`/config/diff`, the web review surface. No test in this spec drives that route.
`us.editor.PendingChanges(sid)` in `internal/component/web/editor.go` is the
producer the web handler reads, and it is the producer the two `.et` files drive
through the CLI, so the fix reaches both; the web route's own rendering is
unproven here.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard. The metadata table declares `Deferral shard: -` | done | Nothing was deferred: every AC has code and a passing test, and the two review findings were fixed in place rather than recorded |

## Review Gate

<!-- NOT FILLED BY THIS SESSION. The 2026-08-19 changes above (the
     runTestCaseIn seam, TestRunnerBlobStorageWritesBlobNotFile, the viewport
     assertion, the filesystem counterpart .et) were authored in this context,
     and ai/rules/planning.md forbids the authoring context from judging its own
     work. An independent pass records the artifact with
     scripts/dev/review_gate.py and fills the table. -->

| Field | Value |
|-------|-------|
| Artifact | not recorded |
| `review_gate.py check` | not run |
| Rounds | not run |
| Reviewer lenses used | not run |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The spec carried no closure half: no Implementation Summary, Audit, Goal Validation, Review Gate, Pre-Commit Verification or Mistake Log | `plan/spec-fixit-zefs-diff-structural-ops.md` | The sections above |
| 2 | BLOCKER | A-4 is recorded `broken` and owed a Mistake Log row and a Deviations entry, and neither existed | The same file | The Mistake Log row and the Deviations entry above |
| 3 | ISSUE | AC-5's proof was mutation-blind: `TestRunnerBlobStorage` and the `.et` both stay green with `configStore = blobStore` dropped | `internal/component/cli/testing/runner_test.go` | `TestRunnerBlobStorageWritesBlobNotFile`, on the `runTestCaseIn` seam |
| 4 | ISSUE | AC-1 claimed the review counts AND shows the delete, and nothing asserted the rendered entry; "matching filesystem storage" rested on code symmetry | `test/editor/session/diff-structural-op-blob.et` | `expect=viewport:contains=peer peer1`, plus the new `diff-structural-op-filesystem.et` counterpart |

## Pre-Commit Verification

Both packages were re-run on 2026-08-19 through the make target, uncached
(`GOFLAGS=-count=1`) and race-instrumented, because a direct `go test` drops the
feature tags and the project build cache (`ai/rules/commands.md`):

```
make ze-unit-pkg-test PKG=./internal/component/config/storage
ok  	github.com/ze-software/ze/internal/component/config/storage	155.017s

MAY_ATTACH=0 make ze-unit-pkg-test PKG=./internal/component/cli/testing
ok  	github.com/ze-software/ze/internal/component/cli/testing	282.942s
```

`MAY_ATTACH=0` is not optional here. `scripts/dev/ze-run.sh` keys job sharing on
(label, tree hash), and `ze-unit-pkg-test` is one label for every PKG, so the
first attempt attached to another session's run of a different package and
returned that package's failure. The row is in
`plan/journal/stale-artifact-reused.md`.

`make ze-lint-changed` reports one issue, and it is in
`internal/plugins/flowspec-firewall/bridge_test.go`, a file this spec does not
touch and another session is editing (`handleUpdate` now takes `*bgp.Event` and
the test still passes a string). Every package this spec touches lints clean.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/storage/blob_test.go` | Yes | `-rw-rw-r-- 1 thomas thomas 5269 Aug 17 18:11 internal/component/config/storage/blob_test.go` |
| `test/editor/session/diff-structural-op-blob.et` | Yes | `-rw-rw-r-- 1 thomas thomas 1831 Aug 19 01:50 test/editor/session/diff-structural-op-blob.et` |
| `test/editor/session/diff-structural-op-filesystem.et` | Yes | `-rw-rw-r-- 1 thomas thomas 1698 Aug 19 01:50 test/editor/session/diff-structural-op-filesystem.et` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The review counts and shows the delete | `TestFunctionalETFiles/session/diff-structural-op-blob.et` and `.../diff-structural-op-filesystem.et` both pass; under the `changesEnabled` mutation both fail with `step 9 (expect viewport): viewport does not contain "peer peer1"` |
| AC-2 | The all-sessions view counts it too | The same two files assert `1 pending change across 1 sessions` after `show \| changes all` |
| AC-3 | A filesystem directory prefix lists the config files as full keys | `TestBlobStorageListFilesystemDirectory` passes in the uncached package run below |
| AC-4 | A namespaced prefix lists that directory, unchanged | `TestResolveDirKey` passes in the same run; its cases include `file/active`, `file/draft` and `meta/ssh` |
| AC-5 | An `.et` asking for blob storage gets a blob-backed editor | `TestRunnerBlobStorageWritesBlobNotFile` passes. Mutated (`configStore = blobStore` replaced by `_ = blobStore`) it reports: `"...set bgp router-id 5.6.7.8..." does not contain "1.2.3.4"`, `should not contain "5.6.7.8"`, and `"...router-id 1.2.3.4..." does not contain "5.6.7.8"` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `delete bgp peer <name>` in a session-mode editor on blob storage | `test/editor/session/diff-structural-op-blob.et` | Yes. Read the file: it types `delete bgp peer peer1`, then `show \| changes`, then `show \| changes all`, with `option=storage:value=blob` in force. The runner's blob branch is what serves that option |
| `Storage.List` with a filesystem directory prefix | `TestBlobStorageListFilesystemDirectory` | Yes. It writes through `blobStorage.WriteFile` and lists the directory the write mapped into |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `resolveKey` and `migrateExistingFiles` in `internal/component/config/storage/blob.go` both build `file/active/<basename>`; `resolveDirKey` maps a filesystem prefix to the same directory |
| A-2 | confirmed | `gopls references` on `Storage.List`: `cmdLsWithStorage`, `cmdEditWithStorage` (through `selectConfig`) and `Editor.listChangeFiles` |
| A-3 | confirmed | No consumer of a `List` result calls `Remove`; `DisconnectSession` rebuilds its path from `ChangePath` |
| A-4 | broken | `newHeadlessModel` built its editor with `cli.NewEditor`, which is filesystem-only. Recorded in the Mistake Log and in Deviations. The runner now takes the backend, so the assumption's consequence ("the proof moves to `test/web/`") did not follow |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `option=storage:value=blob` exists and behaves as the `.et` tables say | `internal/component/cli/testing/runner.go` parses `case "storage"` and serves `blob`, `filesystem` and `""`, and refuses anything else | Yes |
| `expect=file:` is refused beside blob storage | The same function returns that error before building a model; `TestRunnerBlobStorageRefusesFileExpectation` asserts it | Yes |
| The docs that anchor the changed files still describe them correctly | `grep -rn "resolveDirKey\|storage/blob.go" docs/` returns three anchors: `docs/guide/config-editor.md` ("Structural operations appear in the diff", anchored on `resolveDirKey` and `blobStorage.List`), `docs/architecture/zefs-format.md` (anchored on `NewBlob`'s corrupt-store self-heal) and `docs/architecture/fleet-config.md` (anchored on `SetWriteObserver`). All three symbols exist and behave as written; the 2026-08-19 changes touch none of them | Yes |

## Core Insight

A storage backend that answers "no files" and a storage backend that has no
files are indistinguishable to every caller. `blobStorage.List` returned an
empty result for a prefix shape it could not resolve, and three layers above it
that read as "this session has nothing pending". A lookup that cannot resolve
its input has to say so; returning the zero value in that case is what let the
defect live under complete-looking coverage, and it is the same shape
`ai/rules/evidence.md` names for a guard.
