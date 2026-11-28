# Spec: YANG loader guards against the three goyang crash structures

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Three malformed YANG structures crash goyang with an unrecoverable stack overflow or nil-map panic inside `Loader.Resolve`, which runs at daemon start (`DefaultLoader`) and when joining external plugin YANG modules (`yang.Join`). Non-crashing RFC 7950 structural conformance checks moved to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` per `ai/META-DECISIONS-SITUATIONAL.md`. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze loads its embedded, registered, and external-plugin YANG modules through goyang (`internal/component/config/yang/loader.go::AddModuleFromText`, `Resolve`, and `join.go::Join`). Three invalid YANG structures crash goyang inside `Modules.Process()` instead of returning an error:

1. **Cyclic grouping (RFC 7950 Section 7.12):** a `grouping` that references itself directly (`uses g` inside `g`) or indirectly through a chain of `uses` or nested `grouping` statements (within one module, across included submodules, or across imported modules) overflows the Go stack in `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::ToEntry` (lines 698, 841, 952) because `ToEntry` writes `entryCache` only in its deferred return (lines 600-603).
2. **Cyclic identity (RFC 7950 Section 7.18.2):** an `identity` whose `base` references itself directly or indirectly through a chain of `identity` statements (within one module, across included submodules, or across imported modules) overflows the Go stack in `vendor/github.com/openconfig/goyang/pkg/yang/identity.go::addChildren` (lines 70-78), called from `resolveIdentities` (lines 180-189), because `addChildren` recurses over `r.Values` without tracking visited nodes.
3. **Augment targeting a non-augmentable node (RFC 7950 Section 7.17):** a top-level, cross-module, chained, or `uses`-local `augment` whose resolved target is a `leaf` or `leaf-list` panics with `assignment to entry in nil map` in `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::(*Entry).merge` (line 1729, `e.Dir[k] = v`) because `newLeaf` (lines 363-370) leaves `Entry.Dir` nil. RFC 7950 Section 7.17 permits augmenting only a `container`, `list`, `choice`, `case`, `input`, `output`, or `notification` node (`leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, and `action` are forbidden).

Because `yang.Join` calls `Loader.Resolve` on external plugin schemas at `ze start` and `ze config validate`, any malformed plugin or Ze module carrying one of these three structures crashes the entire process instead of being refused cleanly.

**Design of the fix:**
- Run pre-process cycle detection (`checkPreProcess` in `internal/component/config/yang/loader_structure.go`) inside `Loader.process()` (`internal/component/config/yang/loader.go`) immediately before calling `l.modules.Process()`. Using the existing `substatementFieldTable` AST walker and in-memory module/submodule lookup (never reading disk), detect direct and indirect `grouping` cycles (covering both `uses` edges and nested-`grouping` edges) and `identity` base cycles with iterative three-color DFS. When any cycle is found, return joined errors wrapping `ErrCyclicGrouping` or `ErrCyclicIdentity` naming the module, source location, offending symbol, and cycle chain, and skip `l.modules.Process()`.
- Patch `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::(*Entry).Augment` through a retained Apache-2.0 vendor patch (`internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch`, stacked after `goyang-uses-augment.patch` per `docs/contributing/patching-the-standard-library.md`). Right after resolving a non-nil `target` and before `target.merge(nil, namespace, a)`, verify that `target.Dir != nil` and that `target` is one of the seven RFC 7950 Section 7.17 augmentable node kinds (`container`, `list`, `choice`, `case`, `input`, `output`, or `notification`). Otherwise record an error on `e` via `e.errorf` naming the augment's source location, target path, actual target node kind, and RFC 7950 Section 7.17, increment `processed` so the augment is not retried, and skip `target.merge`. Both `ToEntry(m).GetErrors()` (for `uses`-local augments) and `sourcedModules.augmentErrors()` (for top-level augments) surface the error from `Loader.process()` and `Loader.Resolve()`.

**Historical scope record (2026-10-08 through 2026-11-26):** Earlier structural checks from this spec's original AC-2 and partial AC-1 work (`RFC7950-7.19-1` extension substatement grammar, `RFC7950-9.4.4-1` length order and restricted min/max bounds, `RFC7950-9.6.4-2` enum restriction subset/base values, `RFC7950-7.2.2-1` submodule `belongs-to` inclusion, and `RFC7950-9.6.4.2-1` enum assignment plus `goyang-enum-negative.patch`) have already landed at HEAD and are retired into `## Existing Mechanisms`. On 2026-11-26, per `ai/META-DECISIONS-SITUATIONAL.md` ("Full YANG (RFC 7950) coverage runs after the first release. Before it, YANG work is limited to what blocks something Ze needs."), the fifteen non-crashing RFC 7950 structural rows and the remaining RFC 7950 audit proof work split to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md`.

## Work Not Done

| Item split or deferred | Owning spec | Why |
|------------------------|-------------|-----|
| Fifteen non-crashing RFC 7950 structural rows (`5.1-1`, `5.5-1`, `5.6.5-1`, `6.2.1-1`, `6.3.1-1`, `7.1.4-1`, `7.3-1`, `7.9.2-1`, `7.9.3-1`, `7.15-1`, `7.16-1`, `7.21.2-1`, `9.2.4-2`, `9.6.4-1`, `9.12-2`) | `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` | Non-crashing full RFC 7950 conformance is post-release (`ai/META-DECISIONS-SITUATIONAL.md`, "Protocol and RFC scope") |
| Four non-crashing clauses of `RFC7950-7.17-1` (top-level absolute form, `uses` descendant form, duplicate node names, cross-module mandatory augment requiring `when`) and full RFC tagging / `{gap}` removal for `7.12-1`, `7.17-1`, `7.18.2-1` | `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` | Only the crashing target-kind panic and cycle overflows block v1.0; full RFC 7950 audit tagging is post-release |
| `RFC7950-9.6.4.2-1` audit upgrade (`weak` -> `enforced`) and `RFC7950-9.6.4-2` independent audit verdict (code and `goyang-enum-negative.patch` already landed at HEAD) | `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` and `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (owns the 2026-10-09 `coverPackages` vendor-producer journal row) | Landed code and test already fix the runtime bug; only RFC proof tooling and audit verdicts remain |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, third-party plugins, and dependency direction
  → Constraint: external third-party plugins feed YANG modules into `yang.Join` -> `Loader.Resolve`, so a loader crash is an external-plugin stability defect.
- [ ] `ai/doors/config-yang.md` - config/YANG boundaries, external plugin join path, and gates
  → Constraint: `DefaultLoader`, `Loader.Resolve`, and `yang.Join` are the single schema-resolution authority; every embedded Ze module must continue loading cleanly.
- [ ] `ai/doors/le-tooling.md` - rules for `internal/le` and vendor patches
  → Constraint: product packages never import `internal/le`; `internal/le/vendorpatch/patches/` holds diffs applied to `vendor/`.
- [ ] `docs/architecture/config/yang-config-design.md` - how Ze loads and resolves YANG modules and maintains goyang vendor patches
  → Decision: every module passes `Loader.Resolve`, which joins goyang's processing with Ze's checks and returns `*yang.Resolved` only when every check passes.
  → Constraint: retained goyang patches on `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` are documented in Section 1 and verified by `TestEveryVendorPatchIsApplied` and `TestPatchedVendorModulesMatchUpstream`.
- [ ] `docs/contributing/patching-the-standard-library.md`, "Patching a vendored module" - stamp, licence, and verification rules for vendor patches
  → Constraint: the patch against Apache-2.0 `github.com/openconfig/goyang` is Apache-2.0, carries `Ze-Reviewed-Against: github.com/openconfig/goyang v1.6.4-0.20251113201521-a80f2799d7b9`, and stacks cleanly on top of existing goyang patches.

### RFC Summaries
- [ ] `rfc/short/rfc7950.md` and `rfc/full/rfc7950.txt` Sections 7.12, 7.17, and 7.18.2
  → Constraint: Section 7.12 forbids direct and indirect grouping cycles; Section 7.17 restricts augment targets to container, list, choice, case, input, output, or notification; Section 7.18.2 forbids direct and indirect identity base cycles.

**Key insights:**
- Calling `checkPreProcess` inside `Loader.process()` protects `Loader.Resolve`, `DefaultLoader`, `Loader.StageProcess`, and `yang.Join` at once, while keeping `TestStagesRunEveryStageOfResolve` (`loader_stages_test.go`) satisfied with no new top-level `Resolve` stage.
- Checking augment target kinds inside goyang's `(*Entry).Augment` right before `target.merge` catches top-level, cross-module, chained, `uses`-expanded, and `uses`-local augments in one place without duplicating goyang's expansion engine.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD):**
- [ ] `internal/component/config/yang/loader.go` - `Resolve` calls `l.process()`, `l.checkExtensions()`, `l.checkPatterns()`, `l.checkStructure()`, joins errors, then calls `checkReferences(resolved)`; `process()` calls `l.modules.Process()` and then `l.modules.augmentErrors()` when `Process` returned no errors
- [ ] `internal/component/config/yang/loader_structure.go` - `checkStructure` runs post-`Process` checks (`moduleIncludeErrors`, `lengthErrors`, `enumErrors`, `moduleExtensionSubstatementErrors`), provides `substatementFieldTable` AST traversal and in-memory `includedSubmodule`
- [ ] `internal/component/config/yang/loader_stages.go` and `loader_stages_test.go` - `StageProcess` calls `l.process()`; `TestStagesRunEveryStageOfResolve` verifies every top-level stage called by `Resolve` or `resolveDefault` is also called by `loader_stages.go`
- [ ] `internal/component/config/yang/join.go` - `Join` resolves external plugin YANG modules through `resolveJoined` -> `Loader.Resolve`
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` - `ToEntry` (lines 588-738, 837-842, 950-957) recurses unconditionally on `*Uses` and nested `*Grouping` before deferring `setEntryCache`, overflowing the stack on any grouping cycle; `(*Entry).Augment` (lines 1168-1223) calls `target.merge` (line 1708-1732) without checking `target.Dir != nil` or target kind, panicking on `e.Dir[k] = v` when `target` is a `leaf` or `leaf-list`
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/identity.go` - `resolveIdentities` (lines 124-192) calls `addChildren` (lines 70-78) recursively over `r.Values` with no visited guard, overflowing the stack on any identity base cycle
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/modules.go` - `Modules.Process()` (lines 324-407) runs `ms.process()` (`resolveIdentities`), then `ToEntry(m)` for every module and submodule, then the `Augment` retry loop

**Behavior to preserve:**
- Every embedded and registered Ze YANG module loads through `DefaultLoader()` without error.
- Valid acyclic grouping chains (including cross-module and submodule groupings), valid acyclic identity hierarchies (including multiple bases forming a DAG), and valid augments targeting `container`, `list`, `choice`, `case`, `input`, `output`, and `notification` nodes (top-level, cross-module, chained, and `uses`-local) continue to load identically.
- All landed post-`Process` checks (`checkExtensions`, `checkPatterns`, `checkStructure`, `checkReferences`) and all existing goyang vendor patches keep working unchanged.

**Behavior to change:**
- Cyclic groupings, cyclic identities, and augments targeting non-augmentable nodes (`leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, `action`) are refused with descriptive errors naming the module, location, offending statement, and reason instead of overflowing the stack or panicking on a nil map.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `Loader.Resolve` and `Loader.process` | `internal/component/config/yang/loader.go::Resolve`, `process`, `augmentErrors` | Runs `l.modules.Process()`, collects `augmentErrors()` when `Process` returned none, then joins `checkExtensions`, `checkPatterns`, `checkStructure`, and `checkReferences` | Extends: `process()` runs `checkPreProcess` before `l.modules.Process()` so grouping and identity cycles return an error before goyang can overflow the stack, and `augmentErrors()` automatically surfaces errors recorded on `e` by the patched `(*Entry).Augment` |
| Structural checks and AST walker | `internal/component/config/yang/loader_structure.go::checkStructure`, `moduleIncludeErrors`, `lengthErrors`, `enumErrors`, `moduleExtensionSubstatementErrors`, `substatementFieldTable`, `includedSubmodule` | Landed AC-2 / §7.2.2 checks (`RFC7950-7.19-1`, `9.4.4-1`, `9.6.4-2`, `7.2.2-1`, `9.6.4.2-1`) plus cached reflection-free-per-node AST field table and in-memory submodule lookup | Extends: reuses `substatementFieldTable` and `includedSubmodule` for the pre-process grouping and identity cycle walks; retains all landed post-`Process` checks unchanged |
| Enum assignment and goyang negative-enum patch | `internal/component/config/yang/enum_assignment.go::parseEnumAssignment`, `assignEnumValues`; `internal/le/vendorpatch/patches/goyang-enum-negative.patch` | Landed AC-2 fix assigning RFC 7950 §9.6.4.2 enum values (including after negative values) across `checkStructure`, schema nodes, and command arguments | Does not apply: already landed and tested at HEAD (`rfc7950_enum_value_test.go`, `goyang_enum_numbering_test.go`); retained unchanged |
| Section 14 ABNF extension grammar | `internal/component/config/yang/loader_abnf.go`, `loader_grammar.go`, `loader_source.go`, `rfc7950.abnf` | Landed AC-2 check enforcing RFC 7950 Section 14 grammar under extension usages (`RFC7950-7.19-1`, verdict `enforced`) | Does not apply: already landed and enforced at HEAD; retained unchanged |
| Goyang vendor patch stack | `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::(*Entry).Augment`; `internal/le/vendorpatch/patches/goyang-deviate-add-presence.patch`, `goyang-entry-dup-rpc.patch`, `goyang-enum-negative.patch`, `goyang-uses-augment.patch`; `internal/le/vendorpatch/stamp.go::Patches` | Retained Apache-2.0 diffs over upstream goyang applied in strict lexicographical order (`slices.Sort(paths)`) and verified automatically by `TestEveryVendorPatchIsApplied`, `TestPatchedVendorModulesMatchUpstream`, and `./le go patch-stamps check` | Extends: adds `goyang-z-augment-target-kind.patch` (following `govpp-z-pinned-peer.patch` so it sorts after `goyang-uses-augment.patch`) to refuse non-augmentable targets before `target.merge` |
| External plugin schema join | `internal/component/config/yang/join.go::Join`, `resolveJoined` | Joins external plugin YANG modules with the compiled-in set via `Loader.Resolve` | Does not apply directly: already calls `Loader.Resolve`, so external plugin modules inherit all three crash guards automatically |
| Staged schema benchmark | `internal/component/config/yang/loader_stages.go::StageProcess`; `loader_stages_test.go::TestStagesRunEveryStageOfResolve` | Measures each stage of `Resolve` separately and fails if `Resolve` adds an unmeasured top-level stage | Does not apply directly: calling `checkPreProcess` inside `Loader.process()` keeps `StageProcess` protected and preserves the exact stage list |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "Ze is not a closed program whose every caller sits in this repository." | agrees | External third-party plugins supply YANG schemas that `yang.Join` passes through `Loader.Resolve`; refusing cyclic groupings, cyclic identities, and invalid augment targets prevents a third-party module from crashing the daemon |
| `ai/doors/config-yang.md` | Who depends on it from outside | the door covering `internal/component/config/yang/loader.go`, `loader_structure.go`, `loader_crash_guards_test.go` | "An external plugin's own YANG is joined and published at `ze start`, before the full parse, so config under its own root loads in the daemon." | agrees | All three crash guards run inside `Loader.process()` / `(*Entry).Augment`, so external plugin YANG modules joined at `ze start` and `ze config validate` are refused with an operator error instead of crashing |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` | "A shipped `ze` never links an `internal/le` package." | agrees | The new patch file lives under `internal/le/vendorpatch/patches/` as a build/vendor maintenance artifact applied to `vendor/github.com/openconfig/goyang/pkg/yang/entry.go`; no product package imports `internal/le` |
| `docs/architecture/config/yang-config-design.md` | 3. Module Loading | `// Design:` page of `loader.go`, `loader_structure.go`; anchored to `loader.go`, `loader_structure.go`, `entry.go`; modified by this spec | "`DefaultLoader()` runs both phases and `Resolve()`, and it is strict: nothing is best-effort. A registered module that does not parse, an import that no linked package registers, an undeclared extension, an uncompilable pattern and a refused structure each come back, joined, and `DefaultLoader` then answers no `Resolved`." | agrees | Cyclic groupings, cyclic identities, and invalid augment targets are refused strictly inside `Loader.Resolve()` and `DefaultLoader()`, returning no `*Resolved`; `docs/architecture/config/yang-config-design.md` is updated to list the three guards and `goyang-z-augment-target-kind.patch` |
| `docs/architecture/api/commands.md` | Every route validates | anchored to `internal/component/config/yang/loader.go` | "`Loader.Resolve` and `DefaultLoader` refuse a module holding a pattern that translation cannot compile, wrapping `ErrUncompilablePattern`, so no argument is ever built with its pattern missing." | agrees | `checkPatterns` and `ErrUncompilablePattern` are untouched; the new pre-process cycle checks and augment target-kind guard only add earlier/safer refusals for modules that previously crashed before `checkPatterns` could even run |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `internal/component/config/yang/loader.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec modifies only YANG module-load crash guards and adds or alters no CLI command paths, arguments, or pipe operations |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `internal/component/config/yang/loader.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec does not touch command registration, pipe overrides, or answer shapes |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `internal/component/config/yang/loader.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec does not touch pipe execution or command behavior |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- YANG module text enters via `Loader.AddModuleFromText`, `Loader.AddModuleFromFile`, `Loader.LoadEmbedded`, `Loader.LoadRegistered`, or `yang.Join`, then reaches `Loader.Resolve` (or `Loader.StageProcess`).

### Transformation Path
1. Module text -> goyang AST (`*yang.Module` in `sourcedModules.Modules` and `SubModules`).
2. `Loader.process()` -> `l.checkPreProcess()` (`loader_structure.go`): iterative 3-color DFS checks grouping `uses`/nesting graphs (`ErrCyclicGrouping`) and identity `base` graphs (`ErrCyclicIdentity`) across all loaded modules and submodules without reading disk. If any cycle is found, `process()` returns the joined error immediately without invoking `l.modules.Process()`.
3. `Loader.process()` -> `l.modules.Process()`: goyang resolves includes/imports, identities, typedefs, `ToEntry` (including `uses`-local augments), top-level augments, choices, and deviations. Inside `(*Entry).Augment` (`vendor/github.com/openconfig/goyang/pkg/yang/entry.go`), before `target.merge`, non-augmentable target kinds record an error on `e` and skip `merge`.
4. `Loader.process()` -> `l.modules.augmentErrors()` collects any errors recorded on module entries when `Process()` returned none.
5. `Loader.Resolve()` -> `checkExtensions`, `checkPatterns`, `checkStructure`, joins errors, then `checkReferences(resolved)` -> `*yang.Resolved`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| External plugin Stage 1 declaration -> `yang.Join` -> `Loader.Resolve` | External plugin YANG text strings enter `Join` and are resolved through `Loader.Resolve`; refusals become operator errors naming the plugin and module | Yes |
| Ze loader -> vendored goyang `Modules.Process` | In-process Go call inside `internal/component/config/yang/loader.go::process` | Yes |

### Changed-Wire Census

**Search evidence:** No wire format, IPC protocol message, config file syntax, CLI command grammar, or persisted data format changes. Verified across `internal/component/config/yang/loader.go`, `loader_structure.go`, `join.go`, and `vendor/github.com/openconfig/goyang/pkg/yang/entry.go`: all changes are in-process Go schema-loading validation guards that return Go errors where goyang previously overflowed the stack or panicked on a nil map; no encoded bytes, RPC definitions, or cross-process payloads change.

### Integration Points
- `Loader.process()` (`internal/component/config/yang/loader.go`) - called by both `Loader.Resolve()` (used by `DefaultLoader()` and `yang.Join()`) and `Loader.StageProcess()`, so every schema-loading path gets the pre-process cycle guards and augment target-kind guard.
- `internal/le/vendorpatch/patches/` - `vendorpatch.Patches` discovers `goyang-z-augment-target-kind.patch` automatically; `TestEveryVendorPatchIsApplied`, `TestPatchedVendorModulesMatchUpstream`, and `./le go patch-stamps check` enforce it with no test-list edit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | `checkPreProcess` runs inside `Loader.process()` right before `l.modules.Process()`, and the augment target guard runs at the single `(*Entry).Augment` merge point; no caller can obtain a `*yang.Resolved` without passing both |
| No unintended coupling | Yes | Cycle checks live in `internal/component/config/yang/loader_structure.go`; the vendor patch lives in `internal/le/vendorpatch/patches/` and modifies only `vendor/github.com/openconfig/goyang/pkg/yang/entry.go`; product code never imports `internal/le` |
| No duplicated functionality | Yes | Grouping and identity cycle checks reuse `substatementFieldTable` and `includedSubmodule` in `loader_structure.go`; augment target-kind checking runs inside goyang's own `(*Entry).Augment` rather than duplicating `uses` expansion and multi-pass augment path resolution in Ze |
| Zero-copy preserved where applicable | Yes | Schema load time only; no packet or wire path touched |
| Registration over hardcoding, outbound | Yes | `vendorpatch.Patches` discovers the new `.patch` file from the directory; no central patch list edited |
| Registration over hardcoding, inbound | Yes | `substatementFieldTable` derives AST child traversal from goyang's struct tags; augmentable node kinds match RFC 7950 Section 7.17's closed seven-kind specification |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every embedded and registered Ze YANG module has acyclic groupings, acyclic identities, and augments targeting only container, list, choice, case, input, output, or notification nodes | `git grep 'augment ' -- '*.yang'` checked shipped modules; `DefaultLoader()` currently succeeds at HEAD | A shipped Ze module is refused at daemon start | `DefaultLoader()` and `internal/component/config/yang` package tests | unvalidated |
| A-2 | `checkPreProcess` can resolve cross-module and submodule grouping/identity references strictly from in-memory `l.modules.Modules` and `l.modules.SubModules` before `Modules.Process()` populates `Import.Module` and `Include.Module` | Read `vendor/github.com/openconfig/goyang/pkg/yang/modules.go` and `loader_structure.go::includedSubmodule`: `Parse` populates `Modules` and `SubModules` maps by name and `name@revision`, and `mod.Import`/`mod.Include`/`mod.BelongsTo`/`mod.Prefix` are populated at parse time | Cross-module or submodule cycles would be missed before `Process()` | Unit tests for cross-module and cross-submodule grouping and identity cycles | unvalidated |
| A-3 | Recording an error via `e.errorf(...)` and incrementing `processed++` in `(*Entry).Augment` surfaces from `Loader.Resolve()` for both top-level augments and `uses`-local augments without causing an infinite retry loop | Read `vendor/github.com/openconfig/goyang/pkg/yang/modules.go:356-388`, `entry.go:725-735`, and `loader.go:422-463`: `augmentErrors()` collects `ToEntry(mod).GetErrors()` for every held module after `Process()` | A top-level invalid augment target could be silently ignored | Unit tests asserting non-nil error from `Loader.Resolve()` for top-level, cross-module, chained, and `uses`-local augments targeting a leaf | unvalidated |
| A-4 | `goyang-z-augment-target-kind.patch` sorts last under `vendorpatch.Patches` (`slices.Sort`) and stacks cleanly after `goyang-deviate-add-presence.patch`, `goyang-entry-dup-rpc.patch`, `goyang-enum-negative.patch`, and `goyang-uses-augment.patch` | Read `internal/le/vendorpatch/stamp.go::Patches`, `internal/le/vendorpatch/patches/goyang-uses-augment.patch`, and `docs/contributing/patching-the-standard-library.md` | `TestEveryVendorPatchIsApplied` or `TestPatchedVendorModulesMatchUpstream` fails | `internal/le/vendorpatch` unit tests and `./le go patch-stamps check` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Diamond DAGs (two branches referencing the same grouping or an identity inheriting from two bases that share an ancestor) falsely flagged as cycles | Valid diamond grouping or multi-base identity test fails | Use standard 3-color DFS (unvisited / visiting / done): only an edge to a `visiting` node is a cycle; a `done` node is skipped |
| R-2 | Shadowed local grouping inside a container or grouping resolved to the wrong outer grouping during pre-process lookup | False cycle or missed cycle on nested groupings | Follow RFC 7950 Section 5.5 lexical scope resolution upward from the `uses` node's `ParentNode()` chain before checking module/submodule top-level groupings |
| R-3 | Duplicate module or submodule map keys (`name` and `name@revision`) in goyang's `Modules`/`SubModules` cause duplicate cycle errors or missed pointers | Duplicate error lines in test output | Deduplicate modules by `*yang.Module` pointer (as `sourcedModules.held()` does) and sort deterministically by module name before walking |
| R-4 | A `choice` shorthand case or implicit `input`/`output` entry in goyang uses a `Kind` or `Node` type not matched by the augment target check | A valid augment into `choice`, `case`, `input`, or `output` is wrongly refused | Test all seven permitted RFC 7950 Section 7.17 target node kinds (`container`, `list`, `choice`, `case`, `input`, `output`, `notification`) explicitly in unit tests |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false positive would refuse a valid Ze or external-plugin YANG module at daemon start (`DefaultLoader`) or plugin join (`yang.Join`) |
| How is it reverted? | Single commit revert removing the `checkPreProcess` call/helpers and the `goyang-z-augment-target-kind.patch` vendor patch hunk |
| Who else touches this path? | `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` and `plan/post-release/engineering/spec-validated-construction-and-state-types.md` (both post-release and depend on this spec landing first) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Loader.AddModuleFromText` -> `Loader.Resolve` and `yang.Join` | → | `loader.go::process` -> `loader_structure.go::checkPreProcess` (grouping cycle) | `TestLoaderRefusesCyclicGrouping` |
| `Loader.AddModuleFromText` -> `Loader.Resolve` and `yang.Join` | → | `loader.go::process` -> `loader_structure.go::checkPreProcess` (identity cycle) | `TestLoaderRefusesCyclicIdentity` |
| `Loader.AddModuleFromText` -> `Loader.Resolve` and `yang.Join` | → | `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::(*Entry).Augment` (target kind guard) | `TestLoaderRefusesNonAugmentableTarget` |
| `DefaultLoader` (daemon start) | → | `Resolve` over every embedded and registered Ze module | `TestPublishedExtensionUsagesLoad` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A module set carrying a direct self-using grouping, an indirect grouping cycle (same module, across included submodules, or across imported modules), or a grouping whose nested grouping uses the outer grouping; alongside an acyclic grouping chain, a diamond grouping DAG, and `DefaultLoader()` | `Loader.Resolve` (and `yang.Join` when supplied as an external plugin module) returns a non-nil error wrapping `ErrCyclicGrouping` naming the module, source location, grouping, and cycle before goyang runs; the acyclic chain, diamond DAG, and `DefaultLoader()` resolve and return a non-nil `*Resolved` | Non-crashing grouping conformance clauses (such as top-level duplicate grouping names if goyang accepts them) remain in `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` because full RFC 7950 conformance is post-release (`ai/META-DECISIONS-SITUATIONAL.md`) |
| AC-2 | A module set carrying a direct self-based identity or an indirect identity base cycle (same module, across included submodules, or across imported modules); alongside an acyclic identity chain, a multi-base diamond identity DAG, and `DefaultLoader()` | `Loader.Resolve` (and `yang.Join` when supplied as an external plugin module) returns a non-nil error wrapping `ErrCyclicIdentity` naming the module, source location, identity, and cycle before goyang runs; the acyclic chain, diamond DAG, and `DefaultLoader()` resolve and return a non-nil `*Resolved` | Undefined local base identity references are already refused by goyang (`identity.go::findIdentityBase`, pinned in `rfc7950_loader_test.go`); no cyclic identity survives |
| AC-3 | A module set carrying a top-level, cross-module, chained, `uses`-expanded, or `uses`-local `augment` targeting a `leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, or `action`; alongside valid augments targeting each of the seven RFC 7950 Section 7.17 node kinds (`container`, `list`, `choice`, `case`, `input`, `output`, `notification`) and `DefaultLoader()` | `Loader.Resolve` (and `yang.Join`) returns a non-nil error naming the augment source location, target path, target node kind, and RFC 7950 Section 7.17 with zero panic; valid augments into all seven permitted target kinds and `DefaultLoader()` resolve and return a non-nil `*Resolved` containing the augmented child nodes | The remaining four non-crashing clauses of RFC 7950 Section 7.17 (top-level absolute form, `uses`-local descendant form, duplicate node names, cross-module mandatory augment requiring `when`) remain in `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` because they do not crash goyang and full RFC 7950 conformance is post-release |
| AC-4 | The checkout with `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` applied to `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` | `TestEveryVendorPatchIsApplied` and `TestPatchedVendorModulesMatchUpstream` pass across the full goyang patch stack, `./le go patch-stamps check` succeeds, `TestStagesRunEveryStageOfResolve` passes, and `DefaultLoader()` succeeds over every embedded and registered Ze module | None; every vendor patch and every shipped Ze module must pass with zero exemptions |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An external plugin declares a YANG module containing a self-referencing grouping, cyclic identity, or augment targeting a leaf, and the operator runs `ze start` or `ze config validate` | `yang.Join` -> `Loader.Resolve` -> `Loader.process` -> `checkPreProcess` / `(*Entry).Augment` returns a structured refusal naming the plugin, module, and fault instead of crashing the process | `TestLoaderRefusesCyclicGrouping`, `TestLoaderRefusesCyclicIdentity`, `TestLoaderRefusesNonAugmentableTarget` |
| 2 | The daemon starts with all shipped Ze YANG modules | `DefaultLoader` -> `Loader.Resolve` -> `Loader.process` -> all checks pass and return `*Resolved` | `TestPublishedExtensionUsagesLoad` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoaderRefusesCyclicGrouping` | `internal/component/config/yang/loader_crash_guards_test.go` | AC-1: direct self-use, indirect cycle (same module, submodule, cross-module import), nested grouping using outer grouping, and `yang.Join` refusal; acyclic chain and diamond DAG accepted | |
| `TestLoaderRefusesCyclicIdentity` | `internal/component/config/yang/loader_crash_guards_test.go` | AC-2: direct self-base, indirect cycle (same module, submodule, cross-module import), and `yang.Join` refusal; acyclic chain and multi-base diamond DAG accepted | |
| `TestLoaderRefusesNonAugmentableTarget` | `internal/component/config/yang/loader_crash_guards_test.go` | AC-3: top-level, cross-module, chained, `uses`-expanded, and `uses`-local augments targeting `leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, `action` refused without panic (including via `yang.Join`); valid augments into `container`, `list`, `choice`, `case`, `input`, `output`, `notification` accepted with children present | |
| `TestEveryVendorPatchIsApplied` & `TestPatchedVendorModulesMatchUpstream` | `internal/le/vendorpatch/` | AC-4: `goyang-z-augment-target-kind.patch` applies cleanly, reverses to upstream with the rest of the goyang stack, and carries a valid review stamp | |
| `TestStagesRunEveryStageOfResolve` | `internal/component/config/yang/loader_stages_test.go` | AC-4: `Loader.Resolve` stage census stays intact | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No numeric input fields are added or modified; graph traversal depth is bounded by the finite number of groupings and identities in the loaded module set via iterative 3-color DFS | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | - | Embedded Ze modules are valid and never carry these faults; external-plugin YANG schema loading is proven through `yang.Join` in `loader_crash_guards_test.go` and existing `test/plugin/config-validate-agrees-with-boot.ci` | N/A |

## Files to Modify
- `internal/component/config/yang/loader.go` - call `l.checkPreProcess()` inside `Loader.process()` immediately before `l.modules.Process()`
- `internal/component/config/yang/loader_structure.go` - add `ErrCyclicGrouping`, `ErrCyclicIdentity`, and `checkPreProcess` with iterative 3-color grouping and identity cycle detection
- `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` - guard `(*Entry).Augment` before `target.merge` against non-augmentable target node kinds (`leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, `action`)
- `docs/architecture/config/yang-config-design.md` - document the cyclic grouping, cyclic identity, and augment target-kind guards and `goyang-z-augment-target-kind.patch`

## Files to Create
- `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` - Apache-2.0 vendor patch recording the `(*Entry).Augment` target-kind guard against `github.com/openconfig/goyang v1.6.4-0.20251113201521-a80f2799d7b9`
- `internal/component/config/yang/loader_crash_guards_test.go` - unit tests for AC-1, AC-2, and AC-3 (including `yang.Join` coverage)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No shipped YANG modules change |
| YANG validation constraints | No | Loader structural crash guards only, not config value validation |
| YANG custom validators | No | None added |
| CLI commands/flags | No | None added |
| CLI grammar (keyword before value) | No | None added |
| Editor autocomplete | No | Unchanged |
| Functional test for new RPC/API | No | No new RPC or API |
| Pipe completeness | No | No command output changed |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | Loader refusals are schema-load errors returned directly to `DefaultLoader`/`Join` callers (consistent with `ErrLengthOrder`, `ErrEnumValue`, `ErrIncludeBelongsTo`) |
| Prometheus counters/metrics | No | None added |
| BGP family surface | No | Config/YANG loader only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Internal loader crash guard fix |
| 2 | Config syntax changed? | No | Unchanged |
| 3 | CLI command added/changed? | No | Unchanged |
| 4 | API/RPC added/changed? | No | Unchanged |
| 5 | Plugin added/changed? | No | Unchanged |
| 6 | Has a user guide page? | No | Internal schema-loader behavior |
| 7 | Wire format changed? | No | Unchanged |
| 8 | Plugin SDK/protocol changed? | No | Unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Non-crashing RFC 7950 conformance and `{gap}` removal moved to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` per `ai/META-DECISIONS-SITUATIONAL.md`; no `rfc/short/` edit in this spec (note to main thread: 15 non-crashing `{gap}` rows in `rfc/short/rfc7950.md` still cite this spec path until repointed) |
| 10 | Test infrastructure changed? | No | Unchanged |
| 11 | Affects daemon comparison? | No | Unchanged |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (Section 1 goyang vendor patch list and Section 2/3 refused structures table) |
| 13 | Route metadata keys added/changed? | No | Unchanged |
| 14 | Prometheus counters added/changed? | No | Unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Unchanged |
| 16 | Changed source referenced by existing doc source anchors? | Yes | `docs/architecture/config/yang-config-design.md` (updated above); advisory pages anchored to `loader.go` (`docs/DESIGN.md`, `docs/architecture.md`, `docs/architecture/api/commands.md`, `docs/config-reference.md`) checked and unaffected because they describe high-level YANG config loading and `checkPatterns`, neither of which changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config/CLI/API syntax changes |

## Design presentation

### What changes

`Loader.Resolve` (and therefore `DefaultLoader`, `Loader.StageProcess`, and `yang.Join` for external plugin YANG schemas) refuses the three malformed YANG structures that currently crash goyang with a stack overflow or nil-map panic: cyclic `grouping` definitions (RFC 7950 Section 7.12), cyclic `identity` base chains (Section 7.18.2), and `augment` statements targeting non-augmentable nodes such as `leaf` or `leaf-list` (Section 7.17). Grouping and identity cycles are caught before goyang runs via iterative 3-color DFS in `loader_structure.go::checkPreProcess`; non-augmentable augment targets are caught inside goyang's `(*Entry).Augment` before `target.merge` via retained Apache-2.0 vendor patch `goyang-z-augment-target-kind.patch`. All fifteen non-crashing RFC 7950 structural conformance rows move to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md`, and the already-landed AC-2 checks retire into `Existing Mechanisms`.

### Before and after

1. **Cyclic grouping in a module or external plugin schema:**
   - Before: loading `grouping g { uses g; }` (or an indirect cycle `g -> h -> g`) into `Loader.Resolve` or `yang.Join` terminates the process with `runtime: goroutine stack exceeds 1000000000-byte limit` / `fatal error: stack overflow` inside goyang's `ToEntry`.
   - After: `Loader.Resolve` returns an error wrapping `ErrCyclicGrouping` naming the module, source location, grouping name, and cycle (`g -> g` or `g -> h -> g`) before goyang runs; `yang.Join` surfaces an operator error naming the plugin and module.
2. **Cyclic identity base chain:**
   - Before: loading `identity i { base i; }` (or `a -> b -> a`) terminates the process with `fatal error: stack overflow` inside goyang's `addChildren`.
   - After: `Loader.Resolve` returns an error wrapping `ErrCyclicIdentity` naming the module, source location, identity name, and cycle before goyang runs.
3. **Augment targeting a leaf or other non-augmentable node:**
   - Before: loading `augment "/c/a" { leaf b { type string; } }` where `/c/a` is a `leaf` (top-level, cross-module, chained, or `uses`-local) panics with `panic: assignment to entry in nil map` inside goyang's `(*Entry).merge`.
   - After: `Loader.Resolve` returns an error naming the augment's source location, target path `/c/a`, target node kind `leaf`, and RFC 7950 Section 7.17; valid augments targeting `container`, `list`, `choice`, `case`, `input`, `output`, or `notification` continue to load unchanged.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Implement all three checks as pre-`Process` AST walks in Ze | Pre-`Process` AST walking cannot resolve `augment` targets created by `uses` expansion or earlier augments in goyang's multi-pass augment loop without duplicating goyang's expansion and augment-retry engine |
| Patch goyang for all three bugs (including `identity.go` and `ToEntry` grouping recursion) | Grouping and identity cycles are clean, self-contained AST graph checks in Ze's `loader_structure.go` that yield typed Ze sentinel errors (`ErrCyclicGrouping`, `ErrCyclicIdentity`) while keeping the `vendor/` patch surface minimal |
| Keep all fifteen non-crashing RFC 7950 structural rows in this v1.0 release fix spec | Contradicts `ai/META-DECISIONS-SITUATIONAL.md` ("Full YANG (RFC 7950) coverage runs after the first release. Before it, YANG work is limited to what blocks something Ze needs.") |
| Catch `panic` with `recover()` around `l.modules.Process()` | A Go `recover()` cannot catch a stack overflow (fatal runtime crash), and recovering a nil-map panic leaves goyang's shared entry cache partially mutated rather than reporting the exact offending augment |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring and failing regression tests (MANDATORY FIRST)** -- create `internal/component/config/yang/loader_crash_guards_test.go` with tests covering AC-1 (`TestLoaderRefusesCyclicGrouping`), AC-2 (`TestLoaderRefusesCyclicIdentity`), and AC-3 (`TestLoaderRefusesNonAugmentableTarget`), plus stub `ErrCyclicGrouping`, `ErrCyclicIdentity`, and `checkPreProcess` wired from `Loader.process()` in `loader.go`.
   - Tests: `TestLoaderRefusesCyclicGrouping`, `TestLoaderRefusesCyclicIdentity`, `TestLoaderRefusesNonAugmentableTarget`
   - Files: `internal/component/config/yang/loader.go`, `internal/component/config/yang/loader_structure.go`, `internal/component/config/yang/loader_crash_guards_test.go`
   - Verify: tests reproduce the crashes / fail red before the guards are implemented (note: run stack-overflow tests cautiously or assert stub refusal first so the test binary does not hang).
2. **Phase 2: Pre-process grouping and identity cycle guards (AC-1, AC-2)** -- implement iterative 3-color DFS in `loader_structure.go::checkPreProcess` for grouping dependency cycles (lexical scope lookup for `uses` plus nested-`grouping` edges, across modules, submodules, and imports, in-memory only) and identity `base` cycles (in-memory module/submodule identity index).
   - Tests: `TestLoaderRefusesCyclicGrouping`, `TestLoaderRefusesCyclicIdentity`, `TestStagesRunEveryStageOfResolve`, `TestPublishedExtensionUsagesLoad`
   - Files: `internal/component/config/yang/loader.go`, `internal/component/config/yang/loader_structure.go`
   - Verify: AC-1 and AC-2 pass with `-race` and zero regressions across `internal/component/config/yang`.
3. **Phase 3: Goyang augment target-kind vendor patch (AC-3, AC-4)** -- add the RFC 7950 Section 7.17 target-kind check in `vendor/github.com/openconfig/goyang/pkg/yang/entry.go::(*Entry).Augment` right before `target.merge`, and generate the Apache-2.0 vendor patch `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` with `Ze-Reviewed-Against: github.com/openconfig/goyang v1.6.4-0.20251113201521-a80f2799d7b9`.
   - Tests: `TestLoaderRefusesNonAugmentableTarget`, `TestEveryVendorPatchIsApplied`, `TestPatchedVendorModulesMatchUpstream`, `./le go patch-stamps check`
   - Files: `vendor/github.com/openconfig/goyang/pkg/yang/entry.go`, `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch`
   - Verify: AC-3 and AC-4 pass, including all seven permitted target kinds and `uses`-local/chained/cross-module invalid targets.
4. **Phase 4: Documentation update** -- update `docs/architecture/config/yang-config-design.md` with the three guards, their error sentinels, and `goyang-z-augment-target-kind.patch`, and verify `./le doc check all`.
   - Files: `docs/architecture/config/yang-config-design.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-4 each have a producer at file:line and passing positive + negative unit tests |
| Correctness | Iterative DFS handles diamond DAGs without false positives, resolves lexical grouping scopes accurately, never reads files from disk during pre-process lookup, and reports deterministic errors |
| Vendor patch hygiene | `goyang-z-augment-target-kind.patch` is Apache-2.0, stamped against the vendored goyang version, and passes `TestPatchedVendorModulesMatchUpstream` |
| Error quality | Every refusal names the module (or source location), offending grouping/identity/augment target, and why it is refused |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Grouping & identity cycle guards in `loader_structure.go` wired from `loader.go::process` | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefusesCyclic'` |
| Augment target-kind guard in `entry.go` + `goyang-z-augment-target-kind.patch` | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefusesNonAugmentableTarget'` and `./le test unit pkg path ./internal/le/vendorpatch/` |
| Patch stamps valid and shipped YANG modules load | `./le go patch-stamps check` and `./le test unit pkg path ./internal/component/config/yang/` |
| Architecture doc updated and anchors valid | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted / third-party plugin YANG input | External plugins supply YANG text through `yang.Join`; iterative 3-color DFS bounds traversal to O(V+E) with no unbounded recursion stack, no disk reads during pre-process lookup, and no nil-map panic on invalid augment targets |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A shipped Ze module is refused | Verify whether the module violates RFC 7950 or the check has a false positive (e.g., diamond DAG or valid `choice`/`case`/`input`/`output` augment) |
| Vendor patch stack fails upstream round-trip | Regenerate `goyang-z-augment-target-kind.patch` cleanly against the patched stack per `docs/contributing/patching-the-standard-library.md` |
| 3 fix attempts failed | STOP, report all three approaches, ask the user |

## Design Insights
- Calling `checkPreProcess` inside `Loader.process()` keeps `TestStagesRunEveryStageOfResolve` green while protecting `Loader.Resolve`, `DefaultLoader`, `Loader.StageProcess`, and `yang.Join`.
- Because goyang's `ToEntry` eagerly expands nested `grouping` declarations inside an outer `grouping` (`entry.go:837-842`), a nested grouping that uses its enclosing grouping overflows the stack even if the outer grouping never `uses` the nested one; the dependency graph must include both `uses` edges and enclosing-to-nested `grouping` edges.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Check grouping and identity cycles in Ze (`loader_structure.go::checkPreProcess`) before `l.modules.Process()` | Patch goyang's `entry.go` and `identity.go` | Pure AST cycle checks avoid widening the `vendor/` patch surface and produce typed Ze sentinel errors (`ErrCyclicGrouping`, `ErrCyclicIdentity`) |
| Check augment target node kind inside goyang's `(*Entry).Augment` via `goyang-z-augment-target-kind.patch` | Walk raw ASTs before `Process` in Ze | `(*Entry).Augment` already has the resolved target after `uses` expansion and multi-pass chained augments, covering every augment form in one place with zero duplicated resolution logic |
| Split the 15 non-crashing RFC 7950 structural rows and remaining RFC audit work to a post-release skeleton | Keep full RFC 7950 conformance in v1.0 | Follows `ai/META-DECISIONS-SITUATIONAL.md`: full RFC 7950 coverage runs post-release; v1.0 fixes only crash and real-defect bugs |

## Known Limitations
- Non-crashing RFC 7950 structural conformance checks and remaining RFC 7950 audit verdicts are tracked in `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md`.

## Implementation Summary

### What Was Implemented
- `internal/component/config/yang/loader.go`: called `l.checkPreProcess()` inside `Loader.process()` immediately before `l.modules.Process()`, and short-circuited `Loader.Resolve()` on `ErrCyclicGrouping` / `ErrCyclicIdentity` (separate `if` guards) so post-`Process` checks never run on unprocessed ASTs.
- `internal/component/config/yang/loader_structure.go`: exported `ErrCyclicGrouping` and `ErrCyclicIdentity`, implemented `checkPreProcess` with iterative 3-color DFS across loaded modules/submodules/imports strictly in memory (lexical scope + nested grouping + `uses`-augment edges), and wrapped cycle refusals in `cycleError{module, err}`. Left `includedSubmodule` byte-identical to HEAD so `RFC7950-7.2.2-1` stays fresh.
- `internal/component/config/yang/join.go`: updated `resolveErrorOrigins` and `failingModule` so multi-module `yang.Join` attributes `*cycleError` to its owning module and skips `loader.moduleCheckErrors` on unprocessed ASTs.
- `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` + `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch`: added Apache-2.0 vendor patch (stamped against `github.com/openconfig/goyang v1.6.4-0.20251113201521-a80f2799d7b9`, sorted last) guarding `(*Entry).Augment` before `target.merge` against non-augmentable node kinds (`leaf`, `leaf-list`, `anydata`, `anyxml`, `rpc`, `action`) while allowing all seven RFC 7950 Section 7.17 kinds (`container`, `list`, `choice`, `case`, `input`, `output`, `notification`).
- `internal/component/config/yang/loader_crash_guards_test.go`: added `TestLoaderRefusesCyclicGrouping`, `TestLoaderRefusesCyclicIdentity`, and `TestLoaderRefusesNonAugmentableTarget` covering direct/indirect/submodule/cross-module/nested/`uses`-augment cycles, all 6 forbidden and 7 permitted augment target kinds, `StageProcess`, unprocessed-AST short-circuiting, single- and multi-module `yang.Join`, diamond DAGs, and `DefaultLoader`.
- `docs/architecture/config/yang-config-design.md`: documented all three guards, sentinels, and `goyang-z-augment-target-kind.patch` with source anchors.
- `rfc/short/rfc7950.md`: repointed all 18 remaining `{gap}` pointers from this closing spec to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md`.

### Bugs Found/Fixed
- Round 1 review caught `Loader.Resolve()` falling through to `checkExtensions`/`checkPatterns`/`checkStructure` on unprocessed ASTs after `checkPreProcess` returned a cycle error -> fixed in `loader.go` with regression tests in `loader_crash_guards_test.go`.
- Round 2 review caught the mirror path in `join.go::failingModule` calling `loader.moduleCheckErrors` on an unprocessed AST during multi-module `yang.Join` (blaming an innocent valid sibling plugin with `typedef` + `length "min..10"`) -> fixed with `cycleError` + `resolveErrorOrigins` + `skipPostProcess` in `join.go`/`loader_structure.go` and multi-module `yang.Join` regression tests in `loader_crash_guards_test.go`.

### Documentation Updates
- `docs/architecture/config/yang-config-design.md` (Section 1 vendor patch list, Section 2/3 refused structures, source anchors `Augment, isAugmentTarget, augmentTargetKind`, `Resolve, process`, `checkPreProcess`): updated; `./le doc ste check file docs/architecture/config/yang-config-design.md` OK.
- `rfc/short/rfc7950.md`: repointed 18 `{gap}` rows to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` so no dangling links remain after closure.

### Deviations from Plan
- Added `internal/component/config/yang/join.go` to the modified file list after Round 2 review revealed that multi-module `yang.Join` called `loader.moduleCheckErrors` on unprocessed ASTs when `checkPreProcess` skipped `Modules.Process()`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `Loader.Resolve()` and `join.go::failingModule` ran post-`Process` checks after `checkPreProcess` skipped `Modules.Process()` | Post-`Process` checks (`checkExtensions`, `checkStructure`) assume `typ.YangType != nil` and `Import.Module != nil` | Independent Review Gate Rounds 1 and 2 | Short-circuited `Resolve()` and `failingModule` on `ErrCyclicGrouping` / `ErrCyclicIdentity`, added `cycleError` ownership and regression tests |
| approach | Added a 3-line `mod.Modules == nil` check inside `includedSubmodule` | `RFC7950-7.2.2-1` has a func-scoped audit verdict pinned to `includedSubmodule` | `./le rfc check` | Restored `includedSubmodule` byte-identical to HEAD and placed the nil guard in `reachableModulesInMemory` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Pre-process cyclic grouping guard (`ErrCyclicGrouping`) | Done | `internal/component/config/yang/loader_structure.go` (`checkPreProcess`, `cyclicGroupingErrors`) | Wired from `loader.go::process`, iterative 3-color DFS, in-memory only |
| Pre-process cyclic identity guard (`ErrCyclicIdentity`) | Done | `internal/component/config/yang/loader_structure.go` (`checkPreProcess`, `cyclicIdentityErrors`) | Wired from `loader.go::process`, iterative 3-color DFS, in-memory only |
| Augment target-kind guard in goyang + Apache-2.0 vendor patch | Done | `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` (`(*Entry).Augment`, `isAugmentTarget`, `augmentTargetKind`), `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` | Refuses all 6 non-augmentable kinds without panic; permits all 7 RFC 7950 §7.17 kinds |
| Multi-module `yang.Join` blames cyclic plugin without false positives | Done | `internal/component/config/yang/join.go` (`failingModule`, `resolveErrorOrigins`) | Added after Round 2 review |
| Docs updated and `rfc/short/rfc7950.md` gap links repointed | Done | `docs/architecture/config/yang-config-design.md`, `rfc/short/rfc7950.md` | 0 dangling links to this spec |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLoaderRefusesCyclicGrouping` (`internal/component/config/yang/loader_crash_guards_test.go`) | Direct, prefixed, indirect, container, submodule, sibling-submodule, cross-module, nested-outer, local-container, `uses`-augment, `StageProcess`, `Resolve` skip, single- and multi-module `yang.Join`, acyclic/diamond/shadowed, `DefaultLoader` |
| AC-2 | Done | `TestLoaderRefusesCyclicIdentity` (`internal/component/config/yang/loader_crash_guards_test.go`) | Direct, prefixed, indirect, submodule, sibling-submodule, cross-module, `StageProcess`, `Resolve` skip, single- and multi-module `yang.Join`, acyclic/diamond, `DefaultLoader` |
| AC-3 | Done | `TestLoaderRefusesNonAugmentableTarget` (`internal/component/config/yang/loader_crash_guards_test.go`) | All 6 forbidden kinds across top-level/cross-module/chained/`uses`-expanded/`uses`-local/`yang.Join`; all 7 permitted RFC 7950 §7.17 kinds verified on `resolved.GetEntry` |
| AC-4 | Done | `internal/le/vendorpatch/` (`TestEveryVendorPatchIsApplied`, `TestPatchedVendorModulesMatchUpstream`), `./le go patch-stamps check`, `TestStagesRunEveryStageOfResolve`, `DefaultLoader` | All pass cleanly |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLoaderRefusesCyclicGrouping` | Done | `internal/component/config/yang/loader_crash_guards_test.go` | Passed |
| `TestLoaderRefusesCyclicIdentity` | Done | `internal/component/config/yang/loader_crash_guards_test.go` | Passed |
| `TestLoaderRefusesNonAugmentableTarget` | Done | `internal/component/config/yang/loader_crash_guards_test.go` | Passed |
| `TestEveryVendorPatchIsApplied` & `TestPatchedVendorModulesMatchUpstream` | Done | `internal/le/vendorpatch/` | Passed |
| `TestStagesRunEveryStageOfResolve` | Done | `internal/component/config/yang/loader_stages_test.go` | Passed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/yang/loader.go` | Done | `checkPreProcess` call + `Resolve` short-circuit |
| `internal/component/config/yang/loader_structure.go` | Done | Cycle guards + `cycleError` |
| `internal/component/config/yang/join.go` | Changed | Added in Round 2 review to protect multi-module `Join` |
| `internal/component/config/yang/loader_crash_guards_test.go` | Done | Created with AC-1..AC-3 + multi-module `Join` regression tests |
| `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` | Done | Augment target-kind guard |
| `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` | Done | Created, Apache-2.0, stamped |
| `docs/architecture/config/yang-config-design.md` | Done | Updated |
| `rfc/short/rfc7950.md` | Done | 18 `{gap}` rows repointed |

### Audit Summary
- **Total items:** 17
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`join.go` added per Round 2 review)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refuse cyclic groupings before goyang stack overflow | Unit tests (`Loader.Resolve`, `StageProcess`, `yang.Join`) | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefuses|TestStagesRunEveryStageOfResolve|TestJoin'` (`exit 0, 1 package ok, test time 8.671s`) |
| Refuse cyclic identities before goyang stack overflow | Unit tests (`Loader.Resolve`, `StageProcess`, `yang.Join`) | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefuses|TestStagesRunEveryStageOfResolve|TestJoin'` (`exit 0, 1 package ok, test time 8.671s`) |
| Refuse non-augmentable augment targets without nil-map panic and preserve vendor patch integrity | Unit tests + vendorpatch round-trip + patch-stamps check | `./le test unit pkg path ./internal/component/config/yang/ path ./internal/le/vendorpatch/` (`ok`) and `./le go patch-stamps check` (`every patch stamp matches what it patches`) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-yang-loader-structural-checks-c6b20002-2222-4000-8000-000000000002.md` |
| `./le spec review check` | `review_gate: OK` |
| Rounds | 3 |
| Reviewer lenses used | logic+wiring, multi-module `yang.Join` attribution, vendor patch hygiene, `go-standards.md`, RFC audit integrity, docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `Loader.Resolve()` ran post-`Process` checks on unprocessed ASTs after `checkPreProcess` returned a cycle error | `internal/component/config/yang/loader.go:409-415` | Added separate `ErrCyclicGrouping` / `ErrCyclicIdentity` early returns in `Resolve()` + regression tests |
| 2 | BLOCKER | `join.go::failingModule` called `loader.moduleCheckErrors` on unprocessed ASTs in multi-module `yang.Join`, blaming innocent valid sibling plugins | `internal/component/config/yang/join.go:400-450` | Wrapped cycle errors in `cycleError{module, err}`, extracted `cycle.module` in `resolveErrorOrigins`, skipped `moduleCheckErrors` on cycle errors in `failingModule`, added multi-module `Join` tests; Round 3 CLEAN |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/yang/loader.go` | Yes | `-rw-rw-r-- 1 thomas thomas 30498 Nov 27 23:34 internal/component/config/yang/loader.go` |
| `internal/component/config/yang/loader_structure.go` | Yes | `-rw-rw-r-- 1 thomas thomas 46139 Nov 27 23:52 internal/component/config/yang/loader_structure.go` |
| `internal/component/config/yang/join.go` | Yes | `-rw-rw-r-- 1 thomas thomas 41468 Nov 27 23:46 internal/component/config/yang/join.go` |
| `internal/component/config/yang/loader_crash_guards_test.go` | Yes | `-rw-rw-r-- 1 thomas thomas 26566 Nov 27 23:46 internal/component/config/yang/loader_crash_guards_test.go` |
| `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` | Yes | `-rw-rw-r-- 1 thomas thomas 57949 Nov 27 23:12 vendor/github.com/openconfig/goyang/pkg/yang/entry.go` |
| `internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` | Yes | `-rw-rw-r-- 1 thomas thomas 3666 Nov 27 23:13 internal/le/vendorpatch/patches/goyang-z-augment-target-kind.patch` |
| `docs/architecture/config/yang-config-design.md` | Yes | `-rw-rw-r-- 1 thomas thomas 154627 Nov 27 23:15 docs/architecture/config/yang-config-design.md` |
| `rfc/short/rfc7950.md` | Yes | `-rw-rw-r-- 1 thomas thomas 70677 Nov 27 23:35 rfc/short/rfc7950.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Cyclic groupings refused with `ErrCyclicGrouping` before goyang runs; valid chains/DAGs/`DefaultLoader` pass | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefuses|TestStagesRunEveryStageOfResolve|TestJoin'` (`exit 0, 1 package ok, 8.671s`); `./le go style check` (`OK`); 0 lint issues in touched files |
| AC-2 | Cyclic identities refused with `ErrCyclicIdentity` before goyang runs; valid chains/DAGs/`DefaultLoader` pass | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefuses|TestStagesRunEveryStageOfResolve|TestJoin'` (`exit 0, 1 package ok, 8.671s`) |
| AC-3 | All 6 non-augmentable kinds refused without panic; all 7 permitted RFC 7950 §7.17 kinds succeed | `./le test unit pkg path ./internal/component/config/yang/ run 'TestLoaderRefuses|TestStagesRunEveryStageOfResolve|TestJoin'` (`exit 0, 1 package ok, 8.671s`) |
| AC-4 | Vendor patch applies, round-trips to upstream, patch stamps match, `TestStagesRunEveryStageOfResolve` passes, darwin vet clean | `./le test unit pkg path ./internal/le/vendorpatch/` (`ok`), `./le go patch-stamps check` (`every patch stamp matches what it patches`), `./le go vet-platforms darwin` (PASS) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `Loader.AddModuleFromText` -> `Loader.Resolve` / `yang.Join` -> `checkPreProcess` / `(*Entry).Augment` | N/A (embedded Ze modules never carry crash faults; external plugin join path proven in `loader_crash_guards_test.go` via `yang.Join`) | Yes (`TestLoaderRefusesCyclicGrouping`, `TestLoaderRefusesCyclicIdentity`, `TestLoaderRefusesNonAugmentableTarget`, `DefaultLoader`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `DefaultLoader()` succeeds across every embedded and registered Ze module in `TestLoaderRefuses*` |
| A-2 | confirmed | Cross-module, included-submodule, and sibling-submodule cycle tests pass strictly in-memory before `Modules.Process()` |
| A-3 | confirmed | Top-level, cross-module, chained, `uses`-expanded, and `uses`-local invalid augments all return non-nil errors from `Loader.Resolve()` without retry loops |
| A-4 | confirmed | `goyang-z-augment-target-kind.patch` sorts last and passes `internal/le/vendorpatch` + `./le go patch-stamps check` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/config/yang-config-design.md` (Section 1 patch list, Section 2/3 refused structures) | `loader.go`, `loader_structure.go`, `entry.go`, `goyang-z-augment-target-kind.patch` | Yes (`./le doc ste check file` OK) |
| `rfc/short/rfc7950.md` 18 `{gap}` rows repointed | `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md` | Yes (`git grep` confirms 0 remaining `rfc/short/` links to this spec) |

## Checklist

- [x] Tests written (`internal/component/config/yang/loader_crash_guards_test.go`)
- [x] Tests FAIL before the fix
- [x] Tests PASS after the fix
- [x] Scoped verification run by main thread: `./le test unit pkg path ./internal/component/config/yang/`, `./le test unit pkg path ./internal/le/vendorpatch/`, `./le go patch-stamps check`, `./le go lint run scope "./internal/component/config/yang/..."`, `./le doc check all`
- [x] Closure: append `plan/templates/TEMPLATE-CLOSURE.md`, complete `/ze-review` gate, repoint any remaining `rfc/short/rfc7950.md` `{gap}` pointers to `plan/post-release/features/spec-config-yang-rfc7950-structural-conformance.md`, land Commit A + Commit B (`remove plan/release/fixes/spec-config-yang-loader-structural-checks.md`)
