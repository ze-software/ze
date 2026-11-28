# Spec: single-owner-reload-reconcile-4-web-form-atomic-save

| Field | Value |
|-------|-------|
| Status | design |
| Scope | web |
| Priority | P1 |
| Release reason | A web configuration form save applies fields one by one with no rollback, so when the third field is refused the first two remain applied in both the user's working tree and durable session change file. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-single-owner-reload-reconcile.md` (Family S3, part 2 of 3).

## Task

**Root cause.** `(*EditorManager).ApplyFormEdits` (`internal/component/web/editor.go`) holds the per-user mutex across all fields of a form save, but loops over each field calling `us.editor.SetValue` or `us.editor.DeleteValue` directly. Each call mutates `cli.Editor`'s working tree and writes through to the durable per-user change file (`writeThroughSet` / `writeThroughDelete` in `internal/component/cli/editor_draft.go`). On the first refused field, `ApplyFormEdits` returns that field and its error while leaving every earlier field applied in memory and persisted on disk.

**Goal.** Add an atomic batch-edit method `ApplyFieldEdits(edits []contract.FieldEdit) (int, error)` to `contract.Editor` (`internal/component/cli/contract/contract.go`) and implement it on `cli.Editor` (`internal/component/cli/`). All edits validate and apply against cloned working-tree and sparse-change-file copies under a single draft lock; only when every edit succeeds does `cli.Editor` write the updated change file once and swap the new working tree into place. `EditorManager.ApplyFormEdits` delegates the whole form save to `us.editor.ApplyFieldEdits` after pre-filtering untouched secrets and absent deletes, so a refusal on any field leaves both the working tree and durable session change file byte-for-byte unchanged.

**Why it matters for shipping.** Operators editing a multi-field web form expect a rejected save to leave their candidate untouched rather than silently persisting a half-applied subset of the form.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, boundaries
- [ ] `ai/doors/api-surfaces.md` - web UI adapter boundaries, `EditorManager`
- [ ] `ai/doors/commands-cli.md` - `cli.Editor`, `cli/contract` leaf boundary
- [ ] `ai/doors/le-tooling.md` - unit test conventions
- [ ] `docs/architecture/web-interface.md` - `## Per-User Editor` (edited in this same work)

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26.

**Source files read:**
- [ ] `internal/component/web/editor.go` - `ApplyFormEdits` (lines 253-306) holds `us.mu` across the loop, calls `judgeValue`, then `us.editor.SetValue` / `us.editor.DeleteValue` per field, returning on the first error with earlier fields still applied.
- [ ] `internal/component/cli/contract/contract.go` - `Editor` interface (lines 171-259) exposes only single-field `SetValue` and `DeleteValue` and holds no batch/atomic edit method.
- [ ] `internal/component/cli/editor_commands.go` - `SetValue` (line 170) and `DeleteValue` (line 376) validate, normalize, decode `$9$` secrets, and call `writeThroughSet` / `writeThroughDelete` when a session is attached.
- [ ] `internal/component/cli/editor_draft.go` - `writeThroughSet` (line 52) acquires `e.draftLock()`, opens the sparse change file (`openChangeFile`), applies one leaf to `changeTree` and `changeMeta`, writes the change file (`writeChangeFile`), and updates `e.workingTree`.

**Behavior to preserve:**
- `ApplyFormEdits` continues holding `us.mu` across the entire save so concurrent readers (`/config/diff`, commit) never observe a partial state (`TestConfigFormSaveIsOneEdit`).
- Untouched secret placeholders (`config.SecretDataPlaceholder` on `config.LeafHoldsSecret`) remain no-ops and never clobber stored secrets.
- Cleared fields whose leaf is not present in the working tree remain no-ops.
- Exact error framing (`FramedKeyRefusal`, `judgeValue` schema error text) and the returned failing `configFormField` stay unchanged so the web form highlights the exact offending field.
- `internal/component/cli/contract` stays a true leaf with zero imports from `internal/component/*`.

**Behavior to change:**
- `contract.Editor` gains `ApplyFieldEdits(edits []FieldEdit) (int, error)` (returning the failing index and error, or `-1, nil` on success).
- `cli.Editor.ApplyFieldEdits` validates and applies all edits to a cloned working tree (and, when a session is attached, to the in-memory `changeTree` / `changeMeta` / `changeOps` under a single `e.draftLock()`), writing the change file once and swapping `e.workingTree` only when every edit succeeds.
- `EditorManager.ApplyFormEdits` pre-validates every non-skipped edit with `judgeValue`, builds the `[]contract.FieldEdit` slice preserving field indices, and calls `us.editor.ApplyFieldEdits`. On any refusal, zero fields are applied in memory or persisted to storage.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-user web form save serialization | `internal/component/web/editor.go` `EditorManager.ApplyFormEdits`, `judgeValue`, `untouchedSecret` | Serializes a user's form save under `us.mu`, skips untouched secrets and absent deletes, then calls `SetValue`/`DeleteValue` one by one | Replaces the field-by-field loop with a single `us.editor.ApplyFieldEdits` call so refusal leaves the tree and change file unchanged |
| Write-through draft protocol | `internal/component/cli/editor_draft.go` `draftLock`, `openChangeFile`, `writeChangeFile`, `writeThroughSet` | Holds storage lock, reads sparse change file (`changeTree`, `changeMeta`, `changeOps`), mutates one leaf, writes change file, updates `e.workingTree` | Extends: `ApplyFieldEdits` acquires `draftLock` once, opens the sparse change file once, stages every edit onto clones of `changeTree`/`changeMeta`/`changeOps` and `e.workingTree`, and writes/swaps only if every edit succeeds |
| Editor contract interface | `internal/component/cli/contract/contract.go` `Editor` | Decouples `web` from `cli` with zero `internal/component/*` imports | Extends: adds value struct `FieldEdit` (`Path []string`, `Leaf string`, `Value string`, `Delete bool`) and `ApplyFieldEdits([]FieldEdit) (int, error)` |
| Working tree clone | `internal/component/config/tree.go` `(*Tree).Clone` | Deep-clones a `*config.Tree` (already used by `EditorManager.snapshotTree` and `viewTree`) | Extends: used by `cli.Editor.ApplyFieldEdits` to stage all edits on a throwaway clone before swapping into `e.workingTree` |

## Data Flow (MANDATORY)

### Entry Point
- Browser POST to config form handler -> `(*EditorManager).ApplyFormEdits(username, edits)`.

### Transformation Path
1. `ApplyFormEdits` acquires `us.mu` and verifies `us.editor.Tree()` is a non-nil `*config.Tree`.
2. For each `f` in `edits` (tracking original index):
   - If `f.delete`: skip if `f.path` / `f.leaf` is absent in `tree`; otherwise append `contract.FieldEdit{Path: f.path, Leaf: f.leaf, Delete: true}`.
   - Else: skip if `m.untouchedSecret(f.path, f.leaf, f.value)`; run `m.judgeValue(f.path, f.leaf, f.value)` and return `(f, err)` immediately if refused (zero mutations have occurred); otherwise append `contract.FieldEdit{Path: f.path, Leaf: f.leaf, Value: f.value}`.
3. If the effective edit list is empty, return `(configFormField{}, nil)`.
4. Call `failIdx, err := us.editor.ApplyFieldEdits(batch)`.
5. Inside `cli.Editor.ApplyFieldEdits`:
   - Clone `e.workingTree` into `stagedTree`.
   - If `e.session != nil`: acquire `e.draftLock()` once, open `changeTree`, `changeMeta`, `changeOps`, and `committedTree`.
   - Apply each `FieldEdit` sequentially to `stagedTree` (and to `changeTree`/`changeMeta`/`changeOps` when session-backed), running the exact same schema/secret/leaf-list checks as `SetValue` and `DeleteValue` (including `FramedKeyRefusal`).
   - On the first error at index `i`: release `draftLock` without calling `writeChangeFile` and without assigning `e.workingTree`, returning `(i, err)`.
   - When all edits succeed: if `e.session != nil`, call `e.writeChangeFile` once (returning error without swapping `e.workingTree` if disk write fails); then swap `e.workingTree = stagedTree`, mark `e.dirty.Store(true)`, and return `(-1, nil)`.
6. If `err != nil`, `ApplyFormEdits` maps `failIdx` back to the original `configFormField` and returns `(f, err)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `web` -> `cli` | Through leaf package `internal/component/cli/contract` (`FieldEdit`, `Editor.ApplyFieldEdits`) | Yes, zero new component imports |

### Changed-Wire Census

No network protocol wire format, RPC message, EventBus payload, YANG leaf, or persisted file format changes in this spec.

**Search evidence:** `contract.FieldEdit` and `Editor.ApplyFieldEdits` are in-process Go types/methods between `internal/component/web` and `internal/component/cli`; the sparse change file format written by `writeChangeFile` is unchanged.

### Integration Points
- `internal/component/cli/contract/contract.go`: `FieldEdit` struct and `Editor.ApplyFieldEdits` method.
- `internal/component/cli/editor_commands.go` & `editor_draft.go`: `(*Editor).ApplyFieldEdits` implementation.
- `internal/component/web/editor.go`: `(*EditorManager).ApplyFormEdits` delegates to `ApplyFieldEdits` (plus update test fakes implementing `contract.Editor`).
- `docs/architecture/web-interface.md`: update `## Per-User Editor` to document atomic form-save rollback semantics.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Form saves stay inside `EditorManager` -> `contract.Editor` -> `cli.Editor` write-through draft protocol |
| No unintended coupling | Yes | `cli/contract` remains a pure leaf with zero `internal/component/*` imports |
| No duplicated functionality | Yes | Reuses `Tree.Clone()`, `openChangeFile`/`writeChangeFile`, and `SetValue`/`DeleteValue` validation helpers |
| Zero-copy preserved where applicable | Yes | Interactive browser form save path only |
| Registration over hardcoding, outbound | Yes | Generic across every YANG leaf and form |
| Registration over hardcoding, inbound | Yes | No form or field names hardcoded |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `EditorManager.ApplyFormEdits` with 3 edits where edit 3 is refused by `cli.Editor` | -> | `cli.Editor.ApplyFieldEdits` clone-and-swap | `TestFormSaveRefusedFieldLeavesTreeUnchanged` (`internal/component/web/editor_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `EditorManager.ApplyFormEdits` receives 3 edits where the 3rd field is refused by `judgeValue` OR by `cli.Editor` (e.g. invalid path, unknown setting, or secret decode error) | Returns the 3rd `configFormField` and its refusal error, and preserves the user's working tree and persisted per-user change file byte-for-byte identical to before the save (fields 1 and 2 are not applied) | Prior uncommitted edits already present in the user's working tree and sparse change file before the refused save remain preserved; zero fields from the refused form save survive |
| AC-2 | `EditorManager.ApplyFormEdits` receives 3 valid edits (mix of sets, deletes, and untouched secret placeholders) | All non-skipped edits apply atomically in one `writeChangeFile` write, untouched secrets keep their stored values unchanged, and `/config/diff` shows every applied edit | Untouched secret placeholders (`config.SecretDataPlaceholder`) and cleared fields whose leaf is absent from the working tree remain permitted as no-ops so stored secrets are never clobbered and absent deletes stay idempotent |
| AC-3 | `writeChangeFile` fails during a session-backed `ApplyFieldEdits` | `ApplyFieldEdits` returns the storage error and preserves `e.workingTree` identical to its pre-call state (in-memory view never diverges from durable intent) | The prior committed and previously saved draft state in `e.workingTree` remains intact; none of the failed batch's staged mutations survive in memory |
| AC-4 | Concurrent readers call `Diff` or `snapshotTree` while `ApplyFormEdits` runs | Readers observe either the complete before-state or the complete after-state under `us.mu`, never a half-saved form | Other users' independent `EditorManager` sessions remain permitted to read and edit concurrently because serialization is scoped per user mutex (`us.mu`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormSaveRefusedFieldLeavesTreeUnchanged` | `internal/component/web/editor_test.go` | AC-1, AC-2 (exercises both `judgeValue` refusal and `cli.Editor` refusal on a real `cli.Editor` with storage) | |
| `TestEditorApplyFieldEditsRollbackOnFailure` | `internal/component/cli/editor_draft_test.go` | AC-1, AC-2, AC-3 (both standalone and session-backed write-through editors, verifying working tree and sparse change file bytes stay unchanged on refusal or write failure) | |
| `TestConfigFormSaveIsOneEdit` (existing) | `internal/component/web/editor_test.go` | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `len(edits)` | `0..N` | `0` (no-op, returns `-1, nil`), `1`, `N` | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `workbench-form-save` (existing `.wb`) | `test/web/workbench-form-save.wb` | End-to-end browser form save continues passing with atomic save | |

### Interop Tests (Scope: web)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | Web/CLI editor transaction only; no network protocol peer | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `FieldEdit` and `ApplyFieldEdits` to `contract.Editor`, update test mocks implementing `contract.Editor`, and add failing `TestFormSaveRefusedFieldLeavesTreeUnchanged` and `TestEditorApplyFieldEditsRollbackOnFailure`.
2. **Phase: Atomic `cli.Editor.ApplyFieldEdits`** -- implement staged clone-and-swap apply in `internal/component/cli/editor_commands.go` and `editor_draft.go` (single `draftLock`, single `writeChangeFile`, swap `e.workingTree` only on full success).
3. **Phase: `EditorManager.ApplyFormEdits`** -- delegate the save batch to `us.editor.ApplyFieldEdits`, preserving field-index error mapping and secret/absent-delete skipping.
4. **Phase: Docs** -- update `docs/architecture/web-interface.md` (`## Per-User Editor`) to state that `ApplyFormEdits` applies atomically and leaves the working tree and change file unchanged on refusal.

## Files to Modify
- `internal/component/cli/contract/contract.go`
- `internal/component/cli/editor_commands.go`
- `internal/component/cli/editor_draft.go`
- `internal/component/cli/editor_draft_test.go`
- `internal/component/web/editor.go`
- `internal/component/web/editor_test.go`
- Test stubs implementing `contract.Editor` (found by compile in `internal/component/web/` and `internal/component/ssh/`)
- `docs/architecture/web-interface.md`

## Files to Create
- None.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| concurrent-session-corruption | 2026-10-17 | `ApplyFormEdits` refused field leaves earlier fields applied |

## Work Not Done

Sibling children of `plan/release/fixes/spec-single-owner-reload-reconcile.md`:
- `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md` (reload-wide pre-autoload verification and registered conformance suite)
- `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md` (iface owed-apply guard, VPP learned-address proof, carrier resync epoch, DHCPv6 `IFLA_CARRIER_CHANGES`)
- `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md` (nft `NFTA_TABLE_USERDATA` ownership marker and restart reclaim)
- `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md` (IS-IS old-SystemID LSP purge on `SystemID` change)

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "**Config is YANG-modeled, and `ConfigProvider` is its single authority** for the editor, the web UI, subsystems and plugins." | agrees | `ApplyFieldEdits` validates every edit against the YANG schema on a cloned working tree inside `cli.Editor`, preserving `ConfigProvider` and `cli.Editor` as the single editor authority. |
| `docs/architecture/core-design.md` | 18. Config Transaction Protocol | the `// Design:` page of `internal/component/cli/contract/contract.go` | — | silent | Covers daemon-side config commit transactions; `ApplyFieldEdits` extends the candidate-editor `contract.Editor` interface without changing commit transactions. |
| `docs/architecture/config/change-file-structural-ops.md` | The shape | anchored to `contract.go`, `editor_draft.go` | "Metadata entry paths and structural-operation parents use `EditTokens`, not space-joined strings." | agrees | `ApplyFieldEdits` stages every field edit through the exact same `EditTokens` and change-file helpers (`mutateChangeFile` / `recordChangeEntry`) under one `draftLock`. |
| `ai/doors/commands-cli.md` | Traps | the door covering `internal/component/cli/contract/contract.go`, `editor_commands.go`, `editor_draft.go`, `editor_draft_test.go` | "One surface fixed, the other not." | agrees | Putting atomic batch staging on `cli.Editor` (`ApplyFieldEdits`) keeps all validation and write-through persistence logic in one shared editor producer rather than duplicating undo logic in `web`. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | the `// Design:` page of `editor_commands.go`, `editor_draft.go`; anchored to both | "YANG defines format. Extensions declare behavior. Implementation executes behavior." | agrees | `ApplyFieldEdits` preserves every YANG schema validation and extension check executed by `SetValue` / `DeleteValue` while making multi-field application all-or-nothing. |
| `docs/architecture/api/architecture.md` | Message Bus and Participant Wiring | anchored to `internal/component/cli/editor_commands.go` | — | silent | Covers REST/gRPC/gNMI transports and bus participant wiring; untouched by adding `ApplyFieldEdits` to the per-user web editor. |
| `docs/architecture/config/syntax.md` | Overview | anchored to `internal/component/cli/editor_commands.go` | — | silent | Covers Ze configuration file grammar; untouched by atomic web form staging. |
| `docs/architecture/storage-backends.md` | Durable publication | anchored to `internal/component/cli/editor_commands.go` | — | silent | Covers persistent storage backend publication; `ApplyFieldEdits` uses the existing `writeSparseChangeFile` atomic write path once per batch instead of once per field. |
| `docs/architecture/web-interface.md` | Per-User Editor | anchored to `editor_draft.go`; the `// Design:` page of `web/editor.go`; a page this spec modifies | "Each session has an isolated working tree, change tracking, and serialized access via per-user mutex. Operations: `SetValue`, `DeleteValue`, `ApplyFormEdits`, `Commit`, `Discard`, `Diff`, `ChangeCount`, `ReadTree`." | agrees | Updates `ApplyFormEdits` to delegate to `cli.Editor.ApplyFieldEdits` so a refused field never leaves earlier fields in the working tree or sparse change file, and updates this section to document the atomic guarantee. |
| `ai/doors/api-surfaces.md` | Where it sits | the door covering `internal/component/web/editor.go`, `editor_test.go` | "Config edits from the web editor (`internal/component/web` `EditorManager`) and from the REST and gRPC config sessions go through the config editor, not through commands." | agrees | `EditorManager.ApplyFormEdits` continues to go exclusively through `contract.Editor` (`cli.Editor`), extending it with `ApplyFieldEdits`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Kernel/VPP backend mismatch checks are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Superseded for daemon start by the 2026-11-26 row below; this spec adds no startup refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | VPP firewall verification is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Data-plane selector naming is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP export next-hop rewriting is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP export next-hop policy authoring is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Built-in BGP export chain compilation is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP export chain next-hop folding is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP explicit next-hop support is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP originated vs forwarded next-hop policy is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Implied filter seam is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | No startup refusal is added; web form edits operate on a running web editor session only. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Startup dataplane mismatch handling is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Implied filter display is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant naming and message registration are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus `send > to` semantics are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus `receive > from` semantics are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-list enforcement is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation and commit conflict checks are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Bus non-arbitration is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus filter display is untouched by this spec. |

## Design presentation

### What changes
- `contract.Editor` adds `FieldEdit` and `ApplyFieldEdits([]FieldEdit) (int, error)`.
- `cli.Editor.ApplyFieldEdits` stages all edits onto a cloned working tree and (when session-backed) sparse change file under one `draftLock`, writing once and swapping `e.workingTree` only when every edit succeeds.
- `EditorManager.ApplyFormEdits` delegates its validated field batch to `us.editor.ApplyFieldEdits`, so any refused field leaves both the in-memory working tree and durable change file unchanged.

### Before and after
- **Before:** Submitting a web form whose 3rd field failed validation inside `cli.Editor` left fields 1 and 2 applied in the user's working tree and written to the change file.
- **After:** If any field in the form save is refused (or if persisting the change file fails), neither the working tree nor the per-user change file changes.

### Alternatives set aside
- **Per-field undo inside `EditorManager.ApplyFormEdits`:** Rejected because `cli.Editor` writes through to the sparse change file on every `SetValue`/`DeleteValue`, so trying to undo from `web` via reverse `SetValue`/`DeleteValue` calls would leave stale metadata entries, fail to restore leaf-list member operations accurately, and risk leaving a torn change file if an undo write failed.
- **Validating only in `web` before looping `SetValue`:** Rejected because `cli.Editor` performs additional checks (`refuseUnknownSetting`, `decodeSecretValue`, `walkOrCreateIn`, storage write errors) that `judgeValue` does not cover.

### Documents departed from
None.

### Choices for Thomas
none

## Review Gate

- [ ] `/ze-review` passed on diff
- [ ] `/ze-review-spec` confirmed all acceptance criteria and files

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-4 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/component/cli/`, `./le test unit pkg path ./internal/component/web/`, and `./le test functional web`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.wb` test passing

### Closure
- [ ] Owned journal row marked `fixed:` with commit and test evidence
