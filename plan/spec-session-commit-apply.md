# Spec: session-commit-apply — leaf-list set persists and session commit reaches the daemons

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-ui-integrity (deadlock fix, committed de3b46e93) |
| Phase | 1/5 |
| Updated | 2026-06-10 |

> Implementation target: **Anthropic Fable 5** via `/ze-implement`.
> Authored by a diagnostic session that reproduced both faults on real hardware.
> The commit deadlock (F1/F2) is a SEPARATE issue (`spec-web-ui-integrity.md`);
> its fix LANDED at commit `de3b46e93` (`WriteGuard.Has`, `deleteEditFileGuard`).
> Do NOT revert it. Lock-discipline invariant it established: inside a held
> WriteGuard, never call `store.Exists`/`store.Remove` (non-reentrant blob mutex
> self-deadlocks) — use `guard.Has`/`guard.Remove`. See "Relationship to the
> deadlock fix" below.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - per-session commit protocol
4. `internal/component/cli/editor_draft.go` (writeThroughSet), `internal/component/cli/editor_commit.go` (CommitSession/CommitSessionCandidate apply loops), `internal/component/config/tree.go` (Set vs SetSlice/AppendValue/multiValues), `internal/component/config/serialize_set.go` (ValueOrArrayNode case), `cmd/ze/hub/session_factory.go` + `cmd/ze/hub/main.go:644` (reloadAfterCommit wiring)

## Task

Two coupled faults make leaf-list configuration unusable in the session editor on
the appliance, observed live (2026-06-10, real hardware, SSH config editor):

1. **Leaf-list `set` never persists (Bug B).** `set system name-server 8.8.8.8`
   reports `Session committed: 0 change(s) applied` while the editor still shows
   the value as a pending `+` diff. The value is stored in the Tree's scalar map
   but every leaf-list serializer reads the multi-value map, so the change file
   written to disk is empty and commit applies nothing. There is currently NO
   working way to add a leaf-list value in a session.

2. **Session commit does not reach the daemons.** Even once a change persists, an
   SSH/session commit writes `config.conf` but never reloads the running daemon:
   `session_factory.go` builds the editor with no reload notifier, so
   `cmdCommitSession` takes the bare `CommitSession` branch instead of the
   transactional `CommitSessionCandidate` + `NotifyReload` path. The committed
   change does not take effect until restart.

**Goal:** a session user can `set`/`delete` leaf-list members (add-member
semantics), `commit`, and have the change (a) persist to the blob store and
(b) immediately reach the running daemons (e.g. `name-server` written to
resolv.conf and visible to the internal resolver).

### Design decisions already made by the user (do not relitigate)

| Decision | Value |
|----------|-------|
| Leaf-list `set` semantics | **Add member (JunOS-style).** `set system name-server X` appends X to the leaf-list; `delete system name-server X` / `deactivate ... X` removes one member. Each `set` adds; it does not replace the whole list. Verify and match what NON-session `set` already does for leaf-lists (see Current Behavior). |
| Commit meaning | **Commit = apply + propagate.** A session commit MUST reach the running daemons via the existing reload path, not merely write `config.conf`. |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `docs/architecture/config/yang-config-design.md` - per-session commit protocol (change file → draft → committed)
  → Decision: change-tracking is per-session; each set records a SessionEntry with one Value per path
  → Constraint: commit holds the store write lock across read-merge-write; lock is NOT reentrant (already enforced via WriteGuard methods after the deadlock fix)
- [ ] `docs/architecture/web-interface.md` - commit hook vs no-hook editor wiring
  → Decision: editors with a reload hook use the transactional candidate path; editors without one write config.conf directly
- [ ] `ai/rules/qemu-testing.md` - Phase 2 daemon-reload verification needs QEMU (linux-only resolv.conf write)
  → Constraint: linux-only behavior (resolv.conf, daemon reload effect) must be verified in QEMU, never skipped as "needs hardware"
- [ ] `ai/rules/no-partial-completion.md`, `ai/rules/wiring-completeness.md`
- [ ] `ai/rules/hook-mapping.md` - which checks fire on the changed files

### RFC Summaries (MUST for protocol work)
- Not applicable. No wire-protocol change. (resolv.conf/DNS behavior is local, not on-wire.)

**Key insights:**
- The Tree keeps TWO disjoint maps: `values` (scalar, `Set`/`Get`) and
  `multiValues` (leaf-list, `SetSlice`/`AppendValue`/`GetSlice`). A YANG
  `leaf-list` with no `ze:syntax` compiles to `ValueOrArrayNode`
  (`yang_schema.go:333-340`), which serializes ONLY from `multiValues`
  (`serialize_set.go:164-166`). Storing a leaf-list value via scalar `Set` is
  silently dropped on every serialize path.
- The session change model (`config.SessionEntry`, one `Value` per path) is
  single-valued throughout: storage, conflict detection, draft round-trip, and
  the commit merge loops all assume one value per leaf. Add-member leaf-lists
  need a multi-value representation across all of these.
- `reloadAfterCommit` (`main.go:644`) is the daemon-reaching reload function. It
  is already in scope where the SSH session factory is wired (`main.go:745`), so
  threading it to the session editor is a wiring change, not a new mechanism.

## Current Behavior (MANDATORY)

**Source files read:** (all read during diagnosis)
- [ ] `internal/component/cli/editor_commands.go` (SetValue 181-192; DeleteValue 195-206; deleteEditFile 48-52; InsertLeafListValue 593-611 refuses session mode `errInsertNotSupportedInSessionMode`; DeactivateLeafListValue 614-632 refuses session mode)
  → Constraint: SetValue session branch calls writeThroughSet for ALL leaves; the dedicated multi-value methods all early-return in session mode
- [ ] `internal/component/cli/editor_draft.go` (writeThroughSet 48-108; writeThroughDelete 152; writeThroughCreate 112-148; writeThroughRename 212; readChangeFile 308; SaveDraft 327; AdoptSession 732)
  → Constraint: writeThroughSet:71 stores via `changeTarget.Set(key, value)` (scalar) in the change tree, and :101 `target.Set` in the in-memory tree. No leaf-list branch.
- [ ] `internal/component/cli/editor_commit.go` (CommitSession 23-197 apply loop 129-146 uses `target.Set`/`Delete`; CommitSessionCandidate 205-345 twin loop 307-324; conflict detection getValueAtPath at 105 and 284; DiscardSessionPath)
  → Constraint: commit apply loops use scalar `target.Set(leafName, value)`. Even a correctly-stored leaf-list entry would be re-dropped into the committed tree's `values`.
- [ ] `internal/component/config/tree.go` (Set 132-144 → values; SetSlice 160-165 / GetSlice 167-173 / AppendValue 147-151 / GetMultiValues 153-158 → multiValues; InsertMultiValue 618+; Clone copies both maps)
- [ ] `internal/component/config/serialize_set.go` (MultiLeafNode 142 reads values; BracketLeafListNode 153 reads values; ValueOrArrayNode 164-166 reads multiValues)
  → Constraint: ValueOrArrayNode (what a plain leaf-list compiles to) reads multiValues ONLY.
- [ ] `internal/component/config/change_file.go` (SerializeChangeFile 214-220 delegates to SerializeSetWithMeta) — confirms the change file uses the same ValueOrArrayNode multiValues path
- [ ] `internal/component/config/yang_schema.go` (333-340: leaf-list without ze:syntax → ValueOrArray(...); with ze:syntax: multi-leaf/bracket/value-or-array)
- [ ] `internal/component/config/system/yang/ze-system-conf.yang:28` (`leaf-list name-server { type zt:ip-address; }`) — the live repro field
- [ ] `internal/component/cli/model_commands_edit.go` (runActivation leaf-list branch 160-187 → Activate/DeactivateLeafListValue; cmdInsert 273-322 → InsertLeafListValue; both detect leaf-lists via `resolveLeafListValue`)
  → Decision: leaf-list detection already exists (`resolveLeafListValue` returns `isLeafList`); reuse it, do not invent a new detector.
- [ ] `cmd/ze/hub/session_factory.go` (buildSessionModelFactory 27-113: builds editor via NewEditorWithStorage:50, SetSession:57, NewModel:59; NEVER calls SetReloadNotifier)
  → Constraint: SSH session editor has no reload notifier → HasReloadNotifier() false → cmdCommitSession uses bare CommitSession.
- [ ] `internal/component/cli/model_commands_commit.go` (cmdCommitSession 255-346: transactional branch 276-281 chooses CommitSessionCandidate iff HasReloadNotifier())
- [ ] `cmd/ze/hub/main.go` (reloadAfterCommit 644-664 calls doReload + gnmiNotifier.NotifyConfigReload; wired to web 666/844, api 873, gnmi 947, managed 841; SSH session factory wired 745 with reloadAfterCommit IN SCOPE; setupInfraHook 424 wires session factory earlier via infra_setup.go BEFORE reloadAfterCommit exists)
  → Constraint: there are TWO session-factory wiring sites (infra_setup.go via setupInfraHook@424, and main.go:745). reloadAfterCommit is only in scope at the latter. Determine the authoritative one before threading.
- [ ] `internal/component/cli/editor.go` (SetReloadNotifier 253-258; HasReloadNotifier 260-264; NotifyReload 266+)
- [ ] `cmd/ze/hub/infra_setup.go` (setupInfraHook → sshSrv.SetSessionModelFactory(buildSessionModelFactory(sshSrv, params, recorder)))

**Behavior to preserve:**
- Scalar leaf set/commit (control case `set bgp router-id X` → Applied=1) — must keep working.
- The non-session (`-f` filesystem) leaf-list path (InsertMultiValue) — already works; match its add-member semantics.
- Full-daemon WEB commit path (CommitSessionCandidate + reloadAfterCommit) — verified working; do not regress.
- Conflict detection for scalar leaves (live + stale).
- The deadlock fix (WriteGuard.Has, deleteEditFileGuard) — do not revert.
- `.et` / `.ci` / `.wb` test formats and runners.

**Behavior to change:**
- Session-mode leaf-list `set`/`delete`/`activate`/`deactivate`/`insert` must work (add-member), persist, and commit.
- SSH/session commit must reload the running daemons.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- SSH config editor: `set system name-server <ip>` → `cmdSet`/model edit handler → `Editor.SetValue` (session mode) → `writeThroughSet`
- `commit` → `cmdCommitSession` → `CommitSession` (today, no notifier) or `CommitSessionCandidate` (after Phase 2)

### Transformation Path
1. Edit: `SetValue` → `writeThroughSet` writes the per-user change file (sparse tree) + in-memory tree, records a SessionEntry
2. Serialize: `SerializeChangeFile` → `SerializeSetWithMeta` → per-node case (ValueOrArrayNode reads multiValues)
3. Commit: `SaveDraft` (change → draft) → `CommitSession`/`CommitSessionCandidate` merges draft entries into the committed tree → writes config.conf (or candidate version)
4. Reload (Phase 2): `NotifyReload` → `reloadAfterCommit` → `doReload` → plugin-server diff + plugin reload → daemons see new config; resolv.conf written by the resolve/DNS path

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor ↔ Tree | Set (values) vs SetSlice/AppendValue (multiValues) — MUST match node kind | [ ] |
| Editor ↔ change/draft file | SerializeChangeFile/SerializeSetWithMeta + SessionEntry metadata | [ ] |
| Editor ↔ zefs store | WriteGuard methods only (post-deadlock-fix invariant) | [ ] |
| Session editor ↔ daemon | SetReloadNotifier(reloadAfterCommit) → NotifyReload → doReload | [ ] |

### Integration Points
- Phase 1 integrates in `writeThroughSet`/`writeThroughDelete` (storage), the two commit apply loops, conflict detection, and unblocking the leaf-list ops in session mode.
- Phase 2 integrates at `session_factory.go` (call SetReloadNotifier) + the wiring of `reloadAfterCommit` into the session-factory construction.

### Architectural Verification
- [ ] No bypassed layers (store access via guards; node-kind respected in BOTH change and in-memory trees)
- [ ] No unintended coupling (reuse resolveLeafListValue + Tree multi-value APIs; do not duplicate)
- [ ] No duplicated functionality (do not add a parallel leaf-list store; use multiValues)
- [ ] Zero-copy preserved where applicable (textbuf in render/serialize paths)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Session `set system name-server X` then `commit` | → | writeThroughSetMember stores to multiValues; commit apply loop via applySessionEntryToTree; Applied>0 | `TestSessionLeafListSetCommits` + `test/editor/session/leaflist-set-commit.et` — DONE |
| Session `set name-server X` then `set name-server Y` then `commit` | → | both members persist (add-member) | `TestSessionLeafListAddMember` + `test/editor/session/leaflist-add-member.et` — DONE |
| Session `delete system name-server X` | → | writeThroughDeleteMember removes one member (via DeleteByPath routing) | `TestSessionLeafListDeleteMember` + `test/editor/session/leaflist-delete-member.et` — DONE |
| Session `insert`/`deactivate`/`activate` on a leaf-list | → | writeThroughMemberOp structural ops, exact at commit | `TestSessionLeafListInsertDeactivate` + `test/editor/session/leaflist-insert-deactivate.et` — DONE |
| SSH session `commit` | → | editor has reload notifier → CommitSessionCandidate → NotifyReload | `TestSessionEditorHasReloadNotifier` (cmd/ze/hub) + `test/editor/session/leaflist-commit-reload.et` — DONE |
| `set system name-server X` → commit → resolv.conf | → | doReload → applyResolvConf → WriteResolvConf | `TestSessionCommitReloadWritesResolvConf` (`cmd/ze/hub/resolv_reload_integration_linux_test.go`, QEMU) — DONE |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session `set system name-server 8.8.8.8`, then `commit` | `Session committed: 1 change(s) applied`; change file non-empty; committed config contains `name-server` with 8.8.8.8 in the multi-value store; survives restart |
| AC-2 | Session `set name-server 8.8.8.8`, then `set name-server 1.1.1.1`, then `commit` | BOTH members present after commit (add-member, not replace) |
| AC-3 | Session `set name-server 8.8.8.8` twice | Idempotent: member not duplicated (match non-session behavior; verify) |
| AC-4 | Session `delete system name-server 8.8.8.8` (with two members), `commit` | Only that member removed; the other remains |
| AC-5 | Session `insert`/`deactivate`/`activate` on a leaf-list | Works in session mode (no `errInsertNotSupportedInSessionMode`); persists through commit |
| AC-6 | Scalar leaf control: `set bgp router-id 9.9.9.9`, `commit` | Still `Applied=1` (no regression) |
| AC-7 | Two sessions set different leaf-list members on the same path | Conflict detection treats add-member correctly (no false stale conflict for non-overlapping members; real conflict surfaced where it exists) |
| AC-8 | SSH session editor built by `buildSessionModelFactory` | `HasReloadNotifier()` is true |
| AC-9 | SSH `set system name-server 8.8.8.8` + `commit` on a running daemon (QEMU) | Daemon reloads; resolv.conf written with 8.8.8.8 WITHOUT restart |
| AC-10 | `commit` status message after a daemon reload | Reflects reload (status includes "and reloaded" per cmdCommitSession transactional branch) |
| AC-11 | Pending leaf-list diff shown in TUI vs committed result | Consistent: what the `+` diff shows is exactly what commit applies (no "shows value but applies 0") |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionLeafListSetCommits` | `internal/component/cli/editor_leaflist_test.go` | AC-1: leaf-list set stored to multiValues, change file non-empty, Applied=1 | PASS |
| `TestSessionLeafListAddMember` | same | AC-2: two sets → two members | PASS |
| `TestSessionLeafListSetIdempotent` | same | AC-3: duplicate set does not duplicate member | PASS |
| `TestSessionLeafListDeleteMember` | same | AC-4: delete removes one member only | PASS |
| `TestSessionLeafListInsertDeactivate` | same | AC-5: session-mode insert/deactivate/activate work | PASS |
| `TestScalarLeafStillCommits` | same | AC-6: scalar regression guard | PASS |
| `TestLeafListConflictDetection` | same | AC-7: add-member conflict semantics | PASS |
| `TestSessionEditorHasReloadNotifier` | `cmd/ze/hub/session_factory_test.go` | AC-8: SSH editor wired with notifier | PASS |
| `TestCommitSessionCandidateAppliesLeafList` | `internal/component/cli/editor_leaflist_test.go` | leaf-list survives the candidate (transactional) apply loop too | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none — no new numeric leaf; name-server is an ip-address leaf-list) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `leaflist-set-commit.et` | `test/editor/session/` | session set name-server, commit, value persists in committed config | PASS |
| `leaflist-add-member.et` | `test/editor/session/` | two sets accumulate both members | PASS |
| `leaflist-delete-member.et` | `test/editor/session/` | delete removes one member | PASS |
| `leaflist-insert-deactivate.et` | `test/editor/session/` | session insert/deactivate/activate through the TUI, exact order at commit | PASS |
| `leaflist-commit-reload.et` | `test/editor/session/` | commit with reload notifier reports "applied and reloaded" | PASS |
| `TestSessionCommitReloadWritesResolvConf` | `cmd/ze/hub/resolv_reload_integration_linux_test.go` | set name-server + commit → reload effect writes resolv.conf without restart (Phase 2, QEMU) | PASS |

### Interop Tests (MANDATORY for protocol features)
Not applicable — no wire-protocol behavior changes. resolv.conf/DNS effect is local; verified via QEMU functional test, not interop.

### Future (if deferring any tests)
- None planned. Deferral requires explicit user approval per `ai/rules/no-partial-completion.md`.

## Files to Modify

**Phase 1 — leaf-list session write (add-member):**
- `internal/component/cli/editor_draft.go` - writeThroughSet: detect leaf-list node (via schema/resolveLeafListValue) and store via AppendValue/SetSlice into BOTH the change tree (line 71) and in-memory tree (line 101); writeThroughDelete: leaf-list member removal
- `internal/component/cli/editor_commit.go` - both apply loops (CommitSession 129-146, CommitSessionCandidate 307-324): apply leaf-list entries via multi-value API, not scalar Set/Delete; conflict detection (105/284) compare leaf-list slices
- `internal/component/cli/editor_commands.go` - InsertLeafListValue (593-595) and DeactivateLeafListValue (614-616): route through the write-through protocol in session mode instead of refusing; SetValue/DeleteValue leaf-list awareness if needed
- `internal/component/config/` - if SessionEntry / change-file metadata needs a multi-value representation: the metadata model that records per-path entries (likely `meta_tree.go` / `set_parser.go` / wherever SessionEntry + MetaEntry live). Determine during Phase 1 audit; the single-Value-per-path model is the core constraint.
- `internal/component/config/serialize_set.go` / change_file.go - only if a metadata round-trip gap is found for multi-value entries (serialization of leaf-list VALUES already works via ValueOrArrayNode; the gap is per-value METADATA)

**Phase 2 — session commit reaches daemons:**
- `cmd/ze/hub/session_factory.go` - call `ed.SetReloadNotifier(reloadFn)` when building the session editor (after SetSession, before NewModel)
- `internal/component/bgp/config/infra_hook.go` - add a `ReloadConfig func() error` field to `InfraHookParams` (or equivalent) so the reload function reaches the factory
- `cmd/ze/hub/main.go` / `cmd/ze/hub/infra_setup.go` - populate the new field with `reloadAfterCommit`; resolve the two-wiring-sites sequencing (reloadAfterCommit only in scope at main.go:745; setupInfraHook@424 runs earlier — may need lazy/func-indirection or to wire at the later site)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No (name-server leaf-list already exists) | - |
| YANG validation constraints | [ ] No new leaves | - |
| YANG custom validators | [ ] No | - |
| CLI commands/flags | [ ] No new commands; existing set/delete/insert/deactivate gain session-mode leaf-list support | - |
| CLI grammar | [ ] N/A | - |
| Editor autocomplete | [ ] Verify leaf-list completion still works in session mode | model_commands_edit.go |
| Functional test for new behavior | [ ] Yes | test/editor/session/*.et, QEMU |
| Pipe completeness | [ ] N/A | - |
| Env var registration | [ ] No | - |
| Doctor check | [ ] No new runtime dep | - |
| Prometheus counters | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes — leaf-list editing in sessions (behavior fix + member ops) | Documented in `docs/architecture/config/yang-config-design.md` "Leaf-List Editing Semantics"; `docs/features.md` lists config editing generically (no per-node-kind rows — verified by grep, no leaf-list row to update) |
| 2 | Config syntax changed? | Yes — `delete <path> <member>` line form; `inactive <path> <member>` emission (parser already accepted it); change-file `insert-member`/`deactivate-member`/`activate-member` op lines | `docs/architecture/config/yang-config-design.md`; `docs/research/comparison/freertr/23-concurrent-editing.md` op table |
| 3 | CLI command added/changed? | No new commands; existing set/delete/insert/deactivate/activate gained session leaf-list support | covered by #1 docs; `docs/guide/command-reference.md:91` deactivate claim verified still accurate |
| 4 | API/RPC added/changed? | No (verified: no RPC/YANG-api files touched) | - |
| 12 | Internal architecture changed? | Yes — per-member session change-tracking; SSH commit reloads daemons; resolv.conf on reload | `docs/architecture/config/yang-config-design.md`, `docs/architecture/web-interface.md` (both updated) |
| 16 | Changed source referenced by doc anchors? | Yes — grepped `docs/` for `source:` anchors on all changed files; `web-interface.md` readChangeFile claim still valid; `23-concurrent-editing.md` change_file.go anchor updated with new ops; `command-reference.md` setparser.go anchor (structural-only set) unchanged; `configuration.md`/`core-design.md` main_system.go anchors (update checker) unchanged | done |
| 17 | Docs show leaf-list editing examples? | Yes — `docs/guide/configuration.md` name-server row updated (resolv.conf on commit/reload) | done |

(Other rows: No — no plugin SDK, wire format, RFC, comparison-table, test-infrastructure, metric, doctor, or env-var changes; verified by the diff file list.)
`make ze-doc-test` PASSED after updates.

## Files to Create
- `test/editor/session/leaflist-set-commit.et`, `leaflist-add-member.et`, `leaflist-delete-member.et`, `leaflist-insert-deactivate.et`, `leaflist-commit-reload.et`
- `cmd/ze/hub/resolv_reload_integration_linux_test.go` — QEMU test for the Phase 2 daemon effect (resolv.conf); replaces the originally planned `.ci` (see Deviations)
- `cmd/ze/hub/session_factory_test.go` for the reload-notifier wiring test
- `internal/component/cli/editor_leaflist.go`, `internal/component/cli/editor_leaflist_test.go`, `internal/component/config/leaflist_member_test.go`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan — confirm SessionEntry multi-value gap |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-13 | Per template |
| 14. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Audit the SessionEntry model (MANDATORY FIRST, no code)**
   - Determine exactly how the per-session change metadata stores values (one `Value` per path). Decide the add-member representation: per-value SessionEntry keyed by (path, value), OR a multi-value entry. This is the load-bearing design choice; record it in Key Design Decisions before writing code.
   - Confirm what NON-session leaf-list `set` does (replace vs add) so session mode matches. Read the cmdSet leaf-list branch (model edit handler) — `resolveLeafListValue` exists; find where plain `set` routes a leaf-list in non-session mode.

2. **Phase: Leaf-list storage in write-through (add-member)**
   - Tests: `TestSessionLeafListSetCommits`, `TestSessionLeafListAddMember`, `TestSessionLeafListSetIdempotent`
   - Files: editor_draft.go (writeThroughSet/writeThroughDelete), the SessionEntry metadata model
   - Verify: change file is non-empty and round-trips; in-memory tree and change tree agree; the TUI `+` diff matches what will commit (AC-11)

3. **Phase: Commit apply for leaf-lists**
   - Tests: `TestCommitSessionCandidateAppliesLeafList`, `TestScalarLeafStillCommits`, `TestLeafListConflictDetection`
   - Files: editor_commit.go (both apply loops + conflict detection)
   - Verify: Applied>0; both CommitSession and CommitSessionCandidate paths apply leaf-list members; scalar regression guard green

4. **Phase: Unblock session-mode leaf-list ops**
   - Tests: `TestSessionLeafListInsertDeactivate`, `TestSessionLeafListDeleteMember`
   - Files: editor_commands.go (InsertLeafListValue/DeactivateLeafListValue route through write-through instead of refusing)
   - Verify: insert/deactivate/activate/delete work in session and persist

5. **Phase: Session commit reaches the daemons (Phase 2)**
   - Tests: `TestSessionEditorHasReloadNotifier`, then the QEMU daemon-effect test
   - Files: session_factory.go, infra_hook.go (InfraHookParams), main.go/infra_setup.go wiring
   - Verify: HasReloadNotifier() true; cmdCommitSession takes the transactional branch; on QEMU, set name-server + commit writes resolv.conf without restart
   - Note: once the notifier is wired, SSH commits route through CommitSessionCandidate (already deadlock-free); the deadlock fix remains as defense-in-depth for the no-notifier web-only mode.

6. **Functional + QEMU tests** → `.et` suite green via the editor runner; QEMU test proves daemon effect
7. **Full verification** → `make ze-verify`
8. **Complete spec** → audit tables, learned summary, two commits per Spec Closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has implementation + file:line |
| Correctness | Node kind respected in BOTH trees and BOTH commit apply loops; add-member not replace; idempotency; delete removes one member |
| Naming | New tests follow Test* convention; .et names match scenario verbs |
| Data flow | Storage via Tree multi-value API; reload via SetReloadNotifier(reloadAfterCommit), not a new reload path |
| Rule: no-workarounds | Fix at writeThroughSet/commit-apply source; do not special-case name-server |
| Rule: plugin-self-containment | No system/resolve-specific spelling in generic editor/config code |
| Lock discipline | Any new in-guard existence check uses guard.Has (post-deadlock-fix invariant) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Leaf-list set commits | `go test ./internal/component/cli/ -run TestSessionLeafList` |
| Add-member semantics | two-set test green; committed config has both members |
| Session commit reloads daemon | QEMU: `set system name-server` + commit → resolv.conf shows value without restart |
| No scalar regression | `TestScalarLeafStillCommits` green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Leaf-list values validated by the ip-address YANG type as before; no new injection surface |
| Resource | Add-member must not allow unbounded duplicate growth (idempotency / dedup) |
| Reload safety | reloadAfterCommit already used by web/api; reusing it introduces no new privilege |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the introducing phase |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read Current Behavior sources |
| SessionEntry model can't represent multi-value cleanly | STOP, record options in Mistake Log, ask user |
| Reload wiring sequencing blocks (reloadAfterCommit not in scope) | Resolve via InfraHookParams func field; if structurally blocked, ask user |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Relationship to the deadlock fix (READ FIRST)

The commit deadlock (F1) and silent data-loss (F2) are a DIFFERENT bug, fixed
under `spec-web-ui-integrity.md` and committed at `de3b46e93`:
- `editor_commit.go:194` now uses `deleteEditFileGuard(guard)` (was a bare
  `e.store.Remove` under the guard → blob self-deadlock).
- `editor_commit.go` DiscardSessionPath now uses `guard.Has` (was
  `e.store.Exists` under the guard → same-class deadlock).
- `WriteGuard.Has` added to the interface; both implementers satisfy it.

This spec ASSUMES that fix is present. Do not revert it. Phase 2 (wiring the
reload notifier) makes SSH commits route through `CommitSessionCandidate`, which
releases the guard before reload and never hit the deadlock anyway — but the
deadlock fix is still required for web-only mode (no notifier) and as
defense-in-depth.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (diagnosis) "0 changes applied" was the deadlock-orphan (F2) | Independent leaf-list scalar/multiValues store mismatch (Bug B); reproduces with no deadlock | deterministic repro `SetValue(system,name-server,X)` → Applied=0 | corrected before this spec |
| F1 deadlock was web-only | SSH appliance editor hits the same CommitSession path (no notifier) | session_factory.go has no SetReloadNotifier | widened scope; motivates Phase 2 |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| (none yet) | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Scalar `Set` used for a leaf-list node | systemic: session write path, set parser, non-session SetValue (3 independent sites) | "leaf-list nodes MUST use the multi-value Tree API in every write/apply path" | DONE: rule added to `docs/architecture/config/yang-config-design.md` "Leaf-List Editing Semantics" (bold invariant); lint candidate left open |

## Design Insights
- The session change model's single-`Value`-per-path assumption is the root
  constraint behind Bug B. The serializer (multiValues) and the writer (values)
  were never reconciled for leaf-lists because no session test ever set one.
- "Commit" on a network appliance must mean "applied and live", not "written to
  a file". The SSH editor silently being the only commit path WITHOUT a reload
  notifier is the gap that made even a persisted change inert.

## Core Insight
A leaf-list config value typed over SSH on the appliance must (1) be stored where
the serializer reads it (multiValues, not values), (2) survive the commit merge
into the committed tree, and (3) reach the running daemons via the same reload
path the web/API editors already use. All three were missing; this spec restores
the full chain with add-member semantics.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add-member leaf-list `set` (user decision) | Replace-whole-list | Matches JunOS/network-OS convention and ze's existing `insert`; user-selected |
| Reuse `reloadAfterCommit` via SetReloadNotifier on the SSH editor | New SSH-specific reload path; SIGHUP-only | reloadAfterCommit already reaches the daemons and is used by web/api/gnmi; reuse avoids a second reload mechanism |
| Store leaf-list values in Tree multiValues in the write-through path | Keep scalar Set + change the serializer to read values | The serializer (ValueOrArrayNode) and the non-session/file path already standardize on multiValues; changing the serializer would break the loaded-config path |
| (RESOLVED Phase 1 audit) SessionEntry multi-value representation: per-(path,member) MetaEntry entries with new `Member string` field | multi-value entry (Values []string); new Op enum | One MetaEntry per member keeps SessionEntries/SaveDraft/commit/conflict machinery per-entry (no fan-out rewrite); add-member={Member:X,Value:X}, delete-member={Member:X,Value:""}; scalar entries (Member=="") behave exactly as today; serialized form derived from line shape (`set/delete <path> <member>` on a leaf-list node), no new metadata token |
| (Phase 1 audit) MetaTree.SetEntry replacement key becomes (SessionKey, Member) | separate AddMemberEntry method | Scalar behavior identical (Member "" == ""); same-member re-set idempotent (AC-3) |
| (Phase 1 audit) Set parser ValueOrArrayNode stores multiValues (idempotent member-merge) + values sync; `delete <path> <member>` parses as member removal; Tree.Delete clears multiValues | keep parser scalar, fix only writeThroughSet | The set parser itself drops leaf-lists on parse→serialize (same bug class in the committed-config path); fixing only the editor would leave config.conf round-trips lossy |
| (Phase 1 audit) Session insert/deactivate/activate = new StructuralOp types (insert-member + Position, deactivate-member, activate-member) | encode as member delete+add MetaEntries | Order must be exact at commit (`exact-or-reject`); delete+add loses position; structural-op machinery (filter/apply/serialize/conflict) already exists for rename/delete-entry |
| (Phase 1 audit) Phase 2 reload threading = hub-side: setupInfraHook(recorder, reloadFn) → infraSetup → buildSessionModelFactory param → SetReloadNotifier; reloadFn reads an atomic holder assigned where reloadAfterCommit is built | new InfraHookParams.ReloadConfig field through bgp/config + loader_create.go | Keeps reload wiring out of the BGP component entirely; holder resolves the two-wiring-sites sequencing (hook registered at main.go:424 before reloadAfterCommit exists at :644); main.go:745 no-BGP path uses the same wrapper |
| (Phase 1 audit) Non-session SetValue/DeleteByPath leaf-list routing also fixed (add-member / member delete) | session-only fix | Non-session plain `set` is broken the same way (scalar Set, serializer reads multiValues); user decision says session must match non-session JunOS add-member semantics — both route by node kind |

## Known Limitations
- Phase 2 daemon-effect verification is linux-only (resolv.conf); darwin cannot
  validate it — QEMU is mandatory per `ai/rules/qemu-testing.md` (done:
  `cmd/ze/hub` green in `make ze-qemu-integration-test`).
- The hierarchical serializer (`serialize.go` ValueOrArrayNode) still emits
  deactivated members as raw `inactive:` items; typed leaf-lists with
  deactivated members do not round-trip through the HIERARCHICAL format
  (pre-existing). The committed config format is set-meta, where this spec
  fixed the round-trip (`inactive <path> <member>` lines).
- Member-level partial discard (`discard system name-server 8.8.8.8`) is not
  supported; discard works at leaf level and above.
- A concurrent removal of an insert's before/after reference member surfaces
  at commit as an error ("apply structural ops: ... not found"), not as a
  ConflictStale entry (Review Gate NOTE 6).

## RFC Documentation
Not applicable — no protocol-behavior changes.

## Implementation Summary

### What Was Implemented
- **Per-member session change model**: `config.MetaEntry` gained a `Member` field; `MetaTree.SetEntry` replaces per (session, member) so members of one leaf-list coexist (`internal/component/config/meta.go`). `PendingChange` carries `Member`; member ops get their own conflict semantics (`pendingChangesConflict`, `pendingChangeKey`, `SessionChanges` dedup key).
- **Set parser member-merge**: `walkAndSet`/`walkAndSetWithMeta` for `ValueOrArrayNode` store members in `multiValues` (idempotent add-member merge) with the scalar map kept in sync; `delete <path> <member>` removes one member; `Tree.Delete` clears `multiValues` (`setparser.go`, `setparser_meta.go`). `ParseWithMeta` learned `inactive` lines; `DetectFormat` recognizes `inactive ` lines as set format.
- **Serializers**: per-member emission mode when member entries exist (`writeLeafListMemberLines`); deactivated members serialize as bare value + `inactive <path> <member>` line, never a raw `inactive:` item (`serialize_set.go`); orphan delete lines carry the member.
- **Tree member API**: `AddMultiValueMember`, `RemoveMultiValueMember`, `HasMultiValueMember`, `MultiValueMemberState` (`tree.go`).
- **Editor write-through**: `writeThroughSetMember`/`writeThroughDeleteMember`/`writeThroughMemberOp` in new `editor_leaflist.go`; `SetValue` routes leaf-lists to add-member in BOTH modes; `DeleteByPath`/`DeleteLeafListValue` member routing; the shared `applySessionEntryToTree` helper used by all four apply loops (save, both commits, discard replay); member entries skip stale-conflict checks.
- **Ordered member ops**: three new structural op types (`insert-member` with `Position`, `deactivate-member`, `activate-member`) parsed/serialized in change files, applied idempotently at commit (`change_file.go`, `applyMemberOp`); session-mode `insert`/`deactivate`/`activate` no longer refuse.
- **Phase 2 reload wiring**: `newSessionEditor` (hub) wires `SetReloadNotifier` into every SSH session editor; `setupInfraHook(recorder, reloadFn)` threads a late-bound `sessionReloadHolder` wrapper from `main.go`; the no-BGP path passes `reloadAfterCommit` directly; `doReload` gained `applyResolvConf` so a commit changing `system name-server` rewrites resolv.conf without restart.
- **Displays**: member shown in TUI change list, web diff, and conflict summaries; `contract.PendingChange.Member` + two new kinds.

### Bugs Found/Fixed
- Bug B (spec target): session leaf-list `set` stored via scalar `Set`, serializers read `multiValues` — empty change file, `Applied=0`.
- Set parser stored leaf-lists scalar too: ANY parse-serialize round-trip of a set-format config dropped leaf-list lines (committed config path).
- Non-session plain `set`/`delete` on leaf-lists was equally broken (scalar store); fixed via the same node-kind routing.
- `Tree.Delete` left `multiValues` populated: deleted leaf-lists resurrected on next serialize (also: missing mutex).
- Deactivated members serialized as raw `inactive:` items: reparse failed item validation for typed leaf-lists (e.g. ip-address).
- `DetectFormat` misclassified set files containing `inactive` lines as hierarchical.
- `ParseWithMeta` rejected `inactive` lines entirely.
- `pendingChangeKey` and `SessionChanges` dedup collapsed two members on one path into one entry.
- `doReload` never rewrote resolv.conf (name-server changes were inert until restart even for web commits).
- Drive-by: `internal/plugins/tftpserver/socket_integration_linux_test.go` referenced the renamed `defaultBlockSize` constant; the QEMU gate was broken before this work.

### Documentation Updates
- `docs/architecture/config/yang-config-design.md`: new "Leaf-List Editing Semantics" section (member commands, multi-value API invariant, per-member metadata, structural member ops) with source anchors.
- `docs/architecture/web-interface.md`: commit-hook wiring note (web AND SSH editors commit transactionally + reload).
- `docs/research/comparison/freertr/23-concurrent-editing.md`: three member ops added to the structural-op table; per-member MetaEntry note.
- `docs/guide/configuration.md`: name-server row notes resolv.conf rewrite on commit/reload.
- `make ze-doc-test` PASSED (1015 code paths, all references valid).

### Deviations from Plan
- Leaf-list unit tests live in new `internal/component/cli/editor_leaflist_test.go` (and `internal/component/config/leaflist_member_test.go`), not appended to `editor_draft_test.go`/`editor_commit_test.go`: keeps the feature's tests in one file per package.
- Phase 2 wiring did NOT add `InfraHookParams.ReloadConfig`: a hub-side closure (`setupInfraHook(recorder, reloadFn)` + atomic holder) keeps reload out of the BGP component entirely (recorded in Key Design Decisions).
- `test/ui/session-commit-reload.ci` replaced by `test/editor/session/leaflist-commit-reload.et` (TUI commit+reload path is .et territory) plus the QEMU test `cmd/ze/hub/resolv_reload_integration_linux_test.go` for the daemon effect.
- The QEMU daemon-effect test exercises the chain at the `applyResolvConf` seam (session commit, committed file, reload-path extraction, resolv.conf write) rather than booting the full daemon + SSH inside the VM; the commit-NotifyReload-doReload routing is covered by `TestSessionEditorHasReloadNotifier` and `leaflist-commit-reload.et`.
- The `activate`/`deactivate` verbs and `inactive` keyword naming kept as-is per user decision (2026-06-10: "keep as it is").

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Leaf-list `set` persists (Bug B) | Done | `editor_leaflist.go:22` writeThroughSetMember; `editor_commands.go` SetValue routing | change file non-empty, multiValues store |
| Session commit reaches daemons | Done | `session_factory.go` newSessionEditor; `main.go` sessionReloadHolder; `infra_setup.go` | HasReloadNotifier true; transactional branch |
| Add-member (JunOS) semantics | Done | `tree.go` AddMultiValueMember; `setparser.go` member merge | idempotent, never replaces |
| Commit = apply + propagate | Done | `main_reload.go` applyResolvConf; reload notifier wiring | resolv.conf rewritten on reload |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSessionLeafListSetCommits` (cli); `leaflist-set-commit.et` | Applied=1, change file non-empty, survives reparse |
| AC-2 | Done | `TestSessionLeafListAddMember`, `TestSessionLeafListAddMemberPreservesExisting`; `leaflist-add-member.et` | both members + committed seed preserved |
| AC-3 | Done | `TestSessionLeafListSetIdempotent`; `leaflist-add-member.et` (triple set) | no duplicate |
| AC-4 | Done | `TestSessionLeafListDeleteMember`; `leaflist-delete-member.et` | one member removed, other remains |
| AC-5 | Done | `TestSessionLeafListInsertDeactivate`; `leaflist-insert-deactivate.et` | insert position exact; deactivate/activate in place |
| AC-6 | Done | `TestScalarLeafStillCommits` | scalar regression guard green |
| AC-7 | Done | `TestLeafListConflictDetection` | non-overlapping members commit cleanly; set-vs-delete same member conflicts |
| AC-8 | Done | `TestSessionEditorHasReloadNotifier` (hub) | notifier wired + invoked |
| AC-9 | Done | `TestSessionCommitReloadWritesResolvConf` (QEMU, `cmd/ze/hub` ok in `make ze-qemu-integration-test`) | resolv.conf written on real Linux FS without restart |
| AC-10 | Done | `leaflist-commit-reload.et` | status "1 change(s) applied and reloaded" |
| AC-11 | Done | `TestSessionLeafListPendingDiffMatchesCommit` | pending count == Applied; per-member changes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSessionLeafListSetCommits | Done | internal/component/cli/editor_leaflist_test.go | |
| TestSessionLeafListAddMember | Done | same | + AddMemberPreservesExisting variant |
| TestSessionLeafListSetIdempotent | Done | same | |
| TestSessionLeafListDeleteMember | Done | same | |
| TestSessionLeafListInsertDeactivate | Done | same (planned file editor_commands_test.go; see Deviations) | |
| TestScalarLeafStillCommits | Done | same | |
| TestLeafListConflictDetection | Done | same | |
| TestSessionEditorHasReloadNotifier | Done | cmd/ze/hub/session_factory_test.go | + WithoutReloadFn variant |
| TestCommitSessionCandidateAppliesLeafList | Done | internal/component/cli/editor_leaflist_test.go | |
| (config layer, added) | Done | internal/component/config/leaflist_member_test.go | parser round-trip, member delete, MetaTree, change-file ops, inactive round-trip, PendingChange summary |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/component/cli/editor_draft.go | Modified | SaveDraft via applySessionEntryToTree; applyStructuralOps member cases; pendingChangesConflict |
| internal/component/cli/editor_commit.go | Modified | both apply loops, stale-skip, buildCommitMeta, discard replay |
| internal/component/cli/editor_commands.go | Modified | SetValue routing, DeleteLeafListValue, DeleteByPath, session unblock of insert/deactivate/activate |
| internal/component/cli/editor_leaflist.go | Created | member write-through methods (file-modularity split) |
| internal/component/cli/editor_walk.go | Modified | applySessionEntryToTree |
| internal/component/config/meta.go | Modified | MetaEntry.Member, SetEntry key |
| internal/component/config/setparser.go / setparser_meta.go | Modified | member merge, member delete, inactive lines |
| internal/component/config/serialize_set.go | Modified | member emission, inactive member lines, DetectFormat |
| internal/component/config/tree.go | Modified | member API |
| internal/component/config/change_file.go | Modified | 3 member op types + Position |
| cmd/ze/hub/session_factory.go | Modified | newSessionEditor + reload param |
| internal/component/bgp/config/infra_hook.go | NOT modified | hub-side closure instead (see Deviations) |
| cmd/ze/hub/main.go / infra_setup.go / main_reload.go / main_system.go | Modified | holder, threading, applyResolvConf |
| test/editor/session/leaflist-*.et (5) | Created | set-commit, add-member, delete-member, insert-deactivate, commit-reload |
| cmd/ze/hub/session_factory_test.go | Created | AC-8 |
| cmd/ze/hub/resolv_reload_integration_linux_test.go | Created | AC-9 (QEMU) |
| test/ui/session-commit-reload.ci | Replaced | by leaflist-commit-reload.et + QEMU test (see Deviations) |

### Audit Summary
- **Total items:** 4 requirements, 11 ACs, 10 test groups, 17 file entries
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 4 (documented in Deviations: test file locations, hub-side reload threading, .ci→.et replacement, QEMU test seam)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Leaf-list set persists (add-member) | unit + .et functional | `TestSessionLeafListSetCommits`/`AddMember`/`Idempotent` green (`go test ./internal/component/cli/ -run TestSessionLeafList`); `bin/ze-test editor -p leaflist` pass 5/5; committed config re-parsed into multiValues |
| Session commit reaches the daemons | unit + .et + QEMU | `TestSessionEditorHasReloadNotifier` green; `leaflist-commit-reload.et` asserts "1 change(s) applied and reloaded"; QEMU `TestSessionCommitReloadWritesResolvConf` green (`cmd/ze/hub` ok in ze-qemu-integration-test, resolv.conf contains `nameserver 8.8.8.8`) |
| No scalar/commit regression | unit + suites | `TestScalarLeafStillCommits` green; full cli/config/web/hub packages green; all 158 editor .et tests pass (157 pre-existing + new) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Session-mode insert/deactivate/activate had no functional test through the TUI dispatch (AC-5 unit-only) | test/editor/session/ | Added `leaflist-insert-deactivate.et` (exercises cmdInsert + runActivation + commit) |
| 2 | ISSUE | `SessionChanges` dedup key (SessionKey\|Path) collapsed two members of one leaf-list | editor.go:754 | Key now includes Member |
| 3 | ISSUE | Commit error wrap said "apply rename" though the loop now applies member ops too | editor_commit.go:132/307/459, editor_draft.go:353 | Reworded to "apply structural ops" |
| 4 | ISSUE | `errInsertNotSupportedInSessionMode` left unused after unblocking insert | editor_commands.go:21 | Removed (caught by ze-lint-changed) |
| 5 | ISSUE | Contested-leaf serializer re-emitted a committed leaf-list annotation's joined Value as one quoted token (reparse failure) | setparser_meta.go committed-annotation branch | Committed annotations no longer store Value; tree is source of truth (covered by TestLeafListConflictDetection) |
| 6 | NOTE | Insert whose before/after ref was removed by a concurrent commit surfaces as a commit ERROR ("apply structural ops: ... not found"), not a ConflictStale | editor_draft.go applyMemberOp | Accepted: message is actionable; conflict-shaped reporting left as future polish |
| 7 | NOTE | Hierarchical serializer (serialize.go) still emits raw `inactive:` members | serialize.go:117 | Pre-existing; committed config format is set-meta where this is fixed; recorded in Known Limitations |
| 8 | NOTE | `cmdInsert` does not validate the inserted value against the YANG item type (pre-existing, both modes) | model_commands_edit.go cmdInsert | Pre-existing gap, unchanged by this spec; reported |

### Fixes applied
- Findings 1-5 fixed in this change set (each with a test: the .et for 1, dedup behavior exercised by TestSessionLeafListPendingDiffMatchesCommit + SessionChanges path for 2, conflict test for 5; 3 and 4 are message/lint-level with no behavior to regress).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | re-run after fixes: no new BLOCKER/ISSUE findings | full diff re-read + suites green | clean |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (3: findings 6-8)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| internal/component/cli/editor_leaflist.go (+_test.go) | yes | git status (new) |
| internal/component/config/leaflist_member_test.go | yes | git status (new) |
| cmd/ze/hub/session_factory_test.go | yes | git status (new) |
| cmd/ze/hub/resolv_reload_integration_linux_test.go | yes | git status (new) |
| test/editor/session/leaflist-{set-commit,add-member,delete-member,insert-deactivate,commit-reload}.et | yes (5) | `bin/ze-test editor -p leaflist` lists 5 tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..7, AC-11 | unit tests green | `go test ./internal/component/cli/ ./internal/component/config/ -count=1` ok (2026-06-10) |
| AC-5, AC-1..4, AC-10 | .et functional green | `bin/ze-test editor -p leaflist` pass 5/5 (2026-06-10) |
| AC-8 | hub wiring green | `go test ./cmd/ze/hub/ -count=1` ok |
| AC-9 | QEMU green | `make ze-qemu-integration-test`: `ok codeberg.org/thomas-mangin/ze/cmd/ze/hub 2.025s` (resolv.conf test in package) |

### Wiring Verified (end-to-end)
| Entry Point | .ci/.et File | Verified |
|-------------|----------|----------|
| TUI `set system name-server X` + commit | test/editor/session/leaflist-set-commit.et | pass |
| TUI two sets + commit | test/editor/session/leaflist-add-member.et | pass |
| TUI `delete system name-server X` + commit | test/editor/session/leaflist-delete-member.et | pass |
| TUI `insert`/`deactivate`/`activate` + commit | test/editor/session/leaflist-insert-deactivate.et | pass |
| Commit with reload notifier ("and reloaded") | test/editor/session/leaflist-commit-reload.et | pass |
| Reload writes resolv.conf (linux) | cmd/ze/hub/resolv_reload_integration_linux_test.go (QEMU) | pass |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Leaf-list editing semantics section | anchors on tree.go/setparser.go/meta.go/change_file.go/serialize_set.go symbols (all exist) | `make ze-doc-test` PASSED |
| Commit-hook wiring (web + SSH) | anchors on session_factory.go newSessionEditor, main.go sessionReloadHolder | `make ze-doc-test` PASSED |
| Structural-op table (3 member ops) | change_file.go tokens match doc table | `make ze-doc-test` PASSED |
| name-server resolv.conf rewrite on reload | main_reload.go applyResolvConf call site | `make ze-doc-test` PASSED |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-11 all demonstrated (Implementation Audit table)
- [x] Wiring Test table complete — every row has a concrete test name, all DONE
- [x] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE; 3 NOTEs)
- [x] `make ze-test` passes (lint + all ze tests, via the `make ze-verify` final gate run 2026-06-10; log tmp/ze-verify.log)
- [x] Feature code integrated (`internal/component/{cli,config,web}`, `cmd/ze/hub`)
- [x] Integration completeness proven end-to-end (5 .et + QEMU daemon effect)
- [x] Documentation Update Checklist answered Yes/No with source evidence
- [x] Architecture docs updated (yang-config-design.md, web-interface.md)
- [x] Critical Review passes (Correctness/Simplicity/Consistency/Completeness/Quality/Tests — see Critical Review Checklist results in Review Gate)

### Quality Gates (SHOULD pass — defer with user approval)
- [x] RFC constraint comments added (N/A — no protocol change)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed (rule landed in yang-config-design.md)

### Design
- [x] No premature abstraction (one shared apply helper, reused existing structural-op machinery)
- [x] No speculative features (member ops only for ValueOrArrayNode, the broken kind)
- [x] Single responsibility per component (member write-through isolated in editor_leaflist.go)
- [x] Explicit > implicit behavior (Member field, explicit op tokens)
- [x] Minimal coupling (reload threaded hub-side; BGP component untouched)

### TDD
- [x] Tests written first (editor_leaflist_test.go, leaflist_member_test.go, session_factory_test.go)
- [x] Tests FAIL first (compile failures: DeleteLeafListValue/MetaEntry.Member/newSessionEditor undefined; .et 0/4 on stale binary — logged in session)
- [x] Tests PASS (cli/config/hub/web packages ok; `bin/ze-test editor -p leaflist` pass 5/5; editor suite 100%)
- [x] Boundary tests for all numeric inputs (N/A — no new numeric leaf)
- [x] Functional tests for end-to-end behavior (5 .et + QEMU)
- [x] Interop tests for protocol features (N/A — no wire-protocol change, justified above)
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — documented in Review Gate
- [x] Partial/Skipped items have user approval (none; naming kept as-is per user 2026-06-10)
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/NNN-session-commit-apply.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump (user runs script)
- [ ] **Commit B:** `git rm plan/spec-session-commit-apply.md` only (same script)
