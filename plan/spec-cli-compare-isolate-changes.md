# Spec: cli-compare-isolate-changes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - config display/diff design
4. Key source: `internal/component/config/prune.go`, `internal/component/cli/editor.go` (:386-418),
   `internal/component/cli/model_commands_show.go`, `internal/component/cli/model_render.go`,
   `internal/component/cli/model_load.go` (:946-977), `internal/component/command/pipe.go`,
   `internal/component/cli/model_keys.go`, `internal/core/env/env.go`

## Task

User report: "when sshing and configuration which just changed `show | compare` does not
isolate ONLY the part of the configuration which changed - it should".

`show | compare` renders the **entire** configuration with a marker column instead of only the
changed part. On a real config this buries one `+` line under hundreds of unchanged lines.

User design direction (verbatim): *"we should have `show | compare` limiting the data presented
to the part of the yang tree modified and `| format ..` for the format"*.

→ Decision: `compare` is a **data-selection** pipe (which part of the YANG tree), `format` is a
  **presentation** pipe (how it is serialized). This separation already exists in
  `ClassifyShowPipes` (`model_load.go:946-977`) and must be honored rather than extended.

Three workstreams:

1. **Isolation (the reported bug):** `compare` prunes the YANG tree to the modified part, then the
   existing `format` pipe serializes it.
2. **Marker suppression (found while tracing):** compare silently degrades to a plain `show` when
   the `changes` column is disabled.
3. **Cross-session leak (found while tracing, user chose to keep in scope):** `set cli format` is
   process-global and leaks into every concurrent session.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - referenced by the `// Design:` annotation on
      every cli file in scope (diff_tree.go, model_render.go, model_commands_show.go)
  → Constraint: display option settings and diff annotation are described here; changing compare
    output shape means this doc's claims must be re-checked.
- [ ] `ai/rules/design-context.md` - grep ze before proposing; never default to trained instincts
  → Constraint: this rule was violated twice during design (see Mistake Log). Both the config-home
    question and the pruning-layer question were already answered by existing code.
- [ ] `ai/rules/pipe-completeness.md` - pipe operator behavior
- [ ] `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` - serialization is textbuf-based

### RFC Summaries (MUST for protocol work)
N/A - CLI display only, no protocol work.

**Key insights:**
- Data-selection pipes in the config editor follow one shape: **clone the tree → prune it →
  serialize** (`ActiveContentAtPath`, editor.go:386-400). `compare` must follow it.
- Pruning at the tree level (not the rendered-line level) makes BOTH formats isolate for free,
  because `tree` and `config` are both serializers over the same tree.

## Current Behavior (MANDATORY)

**Source files read:** (all read before writing this spec)
- [ ] `internal/component/cli/model_load.go` - `ClassifyShowPipes` (:946-977) already separates
      presentation from data selection: `cmdFormat` → `opts.Format`, accepting ONLY `tree` or
      `config` (:951-955, error "unknown format: %s (use tree or config)"); `cmdCompare` →
      `opts.CompareTarget`, defaulting to `srcConfirmed` when empty (:956-960); `cmdActive`/
      `cmdInactive` → `opts.TreeFilter` (:961-962).
  → Decision: the grammar for this feature already exists. No new pipe, no new format, no new
    config leaf. `compare` selects data; `format` presents it.
- [ ] `internal/component/cli/model.go` - :237-240 `fmtTree = "tree"`, `fmtConfig = "config"`
      (exported as `FmtTree`/`FmtConfig`); :215 `cmdFormat`; :172 PipeFilter Type list.
- [ ] `internal/component/cli/editor.go` - `ActiveContentAtPath` (:386-400): `!e.treeValid` guard →
      `e.tree.Clone()` → `config.PruneInactive(clone, e.schema)` → `config.Serialize` at root or
      `walkPathWithSchemaFrom` + `config.SerializeSubtree` at a path.
      `InactiveContentAtPath` (:404-418) is the mirror using `config.PruneActive`.
  → Decision: THE pattern to mirror. `ChangedContentAtPath` is a sibling of these two.
  → Constraint: both fall back to full serialize when the path walk fails (:396-398, :414-416).
- [ ] `internal/component/config/prune.go` - `PruneInactive(tree, schema)` (:13-18) and
      `PruneActive(tree, schema)` (:23-28), both in-place over a `childProvider` walk
      (`pruneActiveNode`, :31). Doc: "The tree is modified in place. Call on a clone if the
      original must be preserved."
  → Decision: `PruneUnchanged(tree, baseline, schema)` belongs here as a sibling.
- [ ] `internal/component/cli/model_commands_show.go` - `cmdShowDisplayWithSource` (:63) compare
      branch at :85-96: `resolveCompareBaseline` (:216) then
      `viewportData{content, originalContent, hasOriginal:true}`. Does NOT set `forceChanges`.
      `renderShowContent` (:177-185) and `renderTreeAtPath` (:187-202) both branch on format:
      `fmtConfig` → `config.FilterSetByPath(config.SerializeSet(tree, schema))` (:192);
      `fmtTree` → `config.Serialize` / `config.SerializeSubtree` (:195, :201).
  → Constraint: the two formats use DIFFERENT serializers. Pruning rendered lines would fix only
    the tree format. Pruning the tree fixes both.
  → Constraint: `resolveCompareBaseline` (:216-241) returns rendered TEXT, not a tree. Tree-level
    pruning needs the baseline as a TREE — this function (or a sibling) must expose one.
  → Constraint: `NormalizeCompareTarget` (:245-257) falls through to "treat as username" for any
    unrecognized token. The style can never be a compare argument.
  → Constraint: :204 comment claims show columns come from "current DB preferences" — stale;
    they are in-memory session state (editor.go:51). Fix while here.
- [ ] `internal/component/cli/model_render.go` - `setViewportData` (:114) is the only gutter
      consumer. :122 `changesEnabled := data.forceChanges || !m.hasEditor() || m.editor.DiffGutterEnabled()`;
      :123-128 annotates only when `changesEnabled && hasOriginal && originalContent != content`.
      Tree diff at :125 requires `m.editor.schema != nil && len(m.contextPath) == 0`; LCS fallback :127.
  → Constraint: compare with the `changes` column off renders `data.content` verbatim = a plain
    `show`. This is bug 2.
  → Constraint: the gutter diffs the two TEXTS. If both texts are already pruned, the markers are
    correct with no change to the annotation layer.
- [ ] `internal/component/cli/diff_tree.go` - `computeTreeAnnotatedDiff` (:31) uses
      `config.NewParser(schema)` (:32) directly, NOT `parseConfigWithFormat`. `diffWalkChildren`
      (:95) emits `diffUnchanged` for every unchanged node (:150, :254, :256, :360, :362, :407,
      :409); `renderDiffLines` (:69-92) prints all of them and builds `lineMapping`.
  → Constraint: with `| format config` the content is SET-format text; `config.NewParser` cannot
    parse it, so `computeTreeAnnotatedDiff` errors and falls back to LCS (:56-59). Usable output,
    but the tree diff is bypassed for that format. UNVERIFIED whether deliberate — see A-5.
- [ ] `internal/component/cli/editor_draft.go` - `parseConfigWithFormat` (:946-962) uses
      `config.DetectFormat` and dispatches to `NewSetParser` (FormatSet/FormatSetMeta) or
      `NewParser` (FormatHierarchical).
- [ ] `internal/component/cli/model_commands_session.go` - `show changes` (:49-50) sets
      `view.forceChanges = true`. Existing precedent for forcing the gutter.
- [ ] `internal/component/cli/model_commands_show.go` - `cmdShowFiltered` (:111-136): the
      active/inactive data-selection pipe. Gets pruned content, applies text filters, returns
      `(no <filter> configuration)` when empty (:119-122).
  → Decision: the empty-result message pattern for AC-3 ("(no changes)").
- [ ] `internal/core/env/env.go` - `Set` (:111-119) writes a package-global `cache` under `cacheMu`
      AND calls `os.Setenv(canonical, value)`.
  → Constraint: bug 3. `set cli format json` in one session mutates process state, changing the
    default format for every other concurrent SSH and web CLI session.
- [ ] `internal/component/cli/model_keys.go` - `handleSetCLIFormat` (:711-740), dispatched at :432
      for `ModeOperational`. Bare form reports current (:720-728); validates against
      `validCLIFormats` (:730); `env.Set("ze.cli.format", rest)` (:736). Completion at :705.
- [ ] `internal/component/command/pipe.go` - :754 `env.MustRegister(ze.cli.format, Default "text")`;
      `configuredDefault()` (:759-775) maps env value → `pipeKind`. Called at :742
      (`ProcessPipesDetectLog`) and :796 (`ProcessPipesDefaultFormatChecked`). Both are
      package-level funcs taking only `input string` — no session context.
  → Constraint: this is WHY the format default is global. Fixing the leak means threading a
    session value into these entry points.
- [ ] Callers of the pipe entry points (4 non-test): `web/cli_terminal.go:258` and
      `cli/model_mode.go:155` call `ProcessPipesDefaultFormatChecked`; `cli/model_traceroute.go:280`
      and `cli/model_ping.go:284` call `ProcessPipesDetectLog`.
- [ ] `cmd/ze/hub/session_factory.go` - :87 `cli.NewModel(ed)` per SSH session.
  → Constraint: the `Model` is per-session and is the correct owner for a session-scoped format.
- [ ] `internal/component/hub/yang/ze-hub-conf.yang` - `environment cli` (:78-102): `format default`
      enum text/table/json/yaml/ndjson default "text" (:80-93); `transcript` (:94-101).
  → Constraint: NOT extended by this spec. The compare style is a `format` concern, and `format`
    already has its home. Recorded here only to document why no new leaf is added.

**Behavior to preserve:**
- `show` (no compare) renders the full config with the gutter when `changes` is enabled. Only
  `compare` isolates. `show changes` unchanged. (AC-18)
- `format` accepts only `tree` and `config`, and its error text (`model_load.go:952-954`).
- `resolveCompareBaseline` target semantics: confirmed/committed/commit, saved, `rollback N`,
  username fall-through (:216-241) — including `resolveRollbackBaseline` bounds (:261-286).
- `NormalizeCompareTarget` username fall-through (:245-257), used by web (`cli_terminal.go:588`).
- `lineMapping` semantics: display line → working line; removed lines have no entry.
- LCS fallback for subtree context and parse failure.
- `PruneInactive`/`PruneActive` behavior and the active/inactive pipes.
- `environment cli format default` YANG leaf, `ze.cli.format` name, and config > built-in default.
- Existing `.et`: `compare-with-changes.et`, `show-pipe-compare.et` (assert `+` and `listen`
  appear), `compare-no-changes.et` (asserts no error), `show-compare-rollback.et`.

**Behavior to change:** (all user-requested)
1. `show | compare` limits presented data to the modified part of the YANG tree, in whichever
   format the `format` pipe selects.
2. `show | compare` always shows markers, regardless of the `changes` column.
3. `set cli format` becomes session-scoped instead of process-global.

## Data Flow (MANDATORY)

### Entry Point
- Operator types `show | compare [target] [| format tree|config]` in an SSH CLI session
  (`session_factory.go:87` builds the per-session `cli.Model`) or the web CLI (`cli_terminal.go`).

### Transformation Path
1. `ParsePipe` / `ClassifyShowPipes` (`model_load.go:946`) → `ShowPipeOpts{Format, CompareTarget}`.
2. `cmdShowPipe` → `cmdShowDisplayWithSource(format, compareTarget, source)` (`model_commands_show.go:63`).
3. **Changed:** resolve the baseline as a **TREE** (not text), then:
   - clone the working tree → `config.PruneUnchanged(clone, baselineTree, schema)`
   - clone the baseline tree → `config.PruneUnchanged(baselineClone, workingTree, schema)`
4. Serialize BOTH pruned trees in the requested format (`renderTreeAtPath` :187-202 — `fmtTree` →
   `Serialize`/`SerializeSubtree`; `fmtConfig` → `FilterSetByPath(SerializeSet(...))`).
5. `viewportData{content, originalContent, hasOriginal:true, forceChanges:true}`.
6. `setViewportData` (`model_render.go:114`) annotates the two pruned texts → markers.
7. `highlightValidationIssues` (:131) consumes `lineMapping`.

→ Constraint: pruning both sides symmetrically is what makes removals visible. A removed node is
  absent from the working tree but present in the pruned baseline, so the gutter renders it `-`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| cli → config component | `config.PruneUnchanged(tree, baseline, schema)`, sibling of PruneInactive | [ ] |
| Editor → serializer | `ChangedContentAtPath` mirrors `ActiveContentAtPath` (editor.go:386) | [ ] |
| CLI session → command package | session format threaded into `ProcessPipesDefaultFormatChecked` / `ProcessPipesDetectLog` | [ ] |
| web → cli | `cli_terminal.go:258` (pipes), `:588` (`NormalizeCompareTarget`) | [ ] |

### Integration Points
- `config/prune.go` — new `PruneUnchanged` beside `PruneInactive`/`PruneActive`.
- `editor.go` — new `ChangedContentAtPath` beside `ActiveContentAtPath`/`InactiveContentAtPath`.
- `model_commands_show.go` — compare branch (:85-96) uses them; sets `forceChanges`.
- `model_keys.go` / `command/pipe.go` — session-scoped format (workstream 3).

### Architectural Verification
- [ ] No bypassed layers (prune before serialize, like every other data-selection pipe)
- [ ] No unintended coupling (`command` gains a parameter, not a cli import)
- [ ] No duplicated functionality (no second diff engine, no second renderer; reuses
      `PruneInactive`'s walk shape and the existing serializers)
- [ ] Zero-copy preserved where applicable (textbuf serializers unchanged)
- [ ] Registration over hardcoding — no new per-feature field on a core/shared struct beyond
      session state

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A baseline TREE can be obtained for every compare target | `resolveCompareBaseline` (model_commands_show.go:216-241) returns text but builds it from `renderRawSourceAtPath` (:161-173), which already parses a tree via `parseConfigWithFormat` | Tree-level pruning is impossible for some targets; would need to parse the rendered text back | Read :161-241; unit test per target (confirmed/saved/rollback/username) | **confirmed** — `renderRawSourceAtPath` parses the tree at :165 (`parseConfigWithFormat`) then serializes at :172. All four targets route through it. A sibling returning the tree instead of the text is a direct extraction. |
| A-2 | Pruning both trees symmetrically renders additions, removals and modifications correctly through the existing gutter | Gutter diffs two texts (`model_render.go:123-128`); removals survive in the pruned baseline | Removals or additions vanish from compare output | Unit tests AC-5/AC-6/AC-7 | **confirmed** — `TestPruneUnchangedRemovalIsVisibleOnlyViaBaseline` proves the deleted peer appears ONLY in the baseline direction; `TestPruneUnchangedAddedContainerKeptWhole` and `TestPruneUnchangedKeepsChangedLeaf` cover the other two. |
| A-3 | `PruneUnchanged` can reuse the `childProvider` walk shape of `pruneActiveNode` (prune.go:31) across all node kinds (leaf, multi-leaf, bracket-leaf, value-or-array, container, presence container, list, flex, freeform, inline-list) | `diff_tree.go:104-136` enumerates every kind that must be compared | Some node kinds prune incorrectly (silently dropped or always kept) | Unit test per node kind, mirroring diff_tree.go's dispatch | **broken** — see Mistake Log. The `childProvider` ITERATION (`Children()`/`Get()`, serialize.go:178-181) is reusable and is byte-identical to diff_tree.go's `nodeWalker` (:23-26). But `pruneNode`/`pruneActiveNode` only switch on `*ContainerNode` and `*ListNode` (prune.go:40-66, :85-116); leaves are handled out-of-band by the unexported `tree.pruneInactiveLeaves()` (:80). There is no per-kind value comparison to reuse. Superseded by A-9. |
| A-4 | `ContentWithoutUser` (editor.go:222-229) returning serialized text is adaptable to return a tree for the username target | It already clones + reverts on a tree then serializes (:227-229) | Username compare falls back to text-level pruning only | Read :218-241; unit test AC-17 | **confirmed** — :227-229 clones the tree, calls `revertUserChanges(clone, e.meta, username)`, then serializes. The tree exists before serialization; extracting it is a direct change. |
| A-5 | The tree diff being bypassed for `\| format config` (set text fails `config.NewParser`, falls to LCS at diff_tree.go:56-59) is acceptable, needing no change | `computeTreeAnnotatedDiff` (:31-32) vs `parseConfigWithFormat` (editor_draft.go:946) | `\| format config` markers are subtly wrong and this spec must fix the diff parser too | Functional test asserting `\| format config` compare output is correct | **confirmed** — `show-compare-format-config.et` asserts the marker survives the LCS fallback. First run failed on `contains=+`; the cause was the TEST, not the code: the fixture modifies a leaf, so the marker is `*` (`diff.go:24` `diffModified`), not `+`. Corrected and passing. Nearly recorded a Known Limitation that does not exist. |
| A-6 | Only 4 non-test callers of the pipe entry points, so threading a session default is tractable | grep: `web/cli_terminal.go:258`, `cli/model_mode.go:155`, `cli/model_traceroute.go:280`, `cli/model_ping.go:284` | Leak fix balloons | Re-grep at implementation; `make ze-build` | **confirmed** — re-grep at audit returns exactly those 4 non-test call sites and no others. |
| A-7 | `os.Setenv` in `env.Set` is not relied on by a child process for `ze.cli.format` | `env.go:111-119`; read in-process via `env.Get` (pipe.go:760) | Removing `env.Set` from the set-cli path breaks a subprocess | grep for subprocess spawn reading `ze.cli.format` / `ZE_CLI_FORMAT` | **confirmed** — every reference to `ze.cli.format` is in-process: `apply_env.go:53` (plumbing), `model_keys.go:721` (`env.Get`), `model_keys.go:736` (`env.Set`), `pipe.go:754` (register), `pipe.go:760` (`env.Get`). No `ZE_CLI_FORMAT` reader, no subprocess consumer. The `os.Setenv` side effect is unused. |
| A-8 | The web CLI has no `set cli format` equivalent and can pass "no session override" | `handleSetCLIFormat` dispatched only at `model_keys.go:432` | Web sessions lose the configured default | grep web for `set cli format`; functional test | **confirmed** — grep for `set cli format` / `handleSetCLIFormat` in `internal/component/web/` returns nothing. |
| A-9 | "Unchanged" can be defined as "serializes identically", removing the need for a per-node-kind comparison dispatch in `PruneUnchanged` | `diffContainerInline` (diff_tree.go:265-277) already compares `SerializeSubtree(origChild)` vs `SerializeSubtree(modChild)` as strings; `diffNodeFallback` (:416-431) + `serializeNodeText` (:518-532) do the same for unsupported kinds | Must hand-write a comparison per node kind, duplicating diff_tree.go's dispatch and risking divergence between "what prunes" and "what the gutter marks" | Unit test per node kind (`TestPruneUnchangedPerNodeKind`) asserting prune decisions agree with the diff markers | **confirmed** — `serializeNode` (serialize.go:256) switches on exactly the same nine kinds as `diffWalkNode` (diff_tree.go:104-136), and handles `FreeformNode`/`InlineListNode` properly where the diff walk calls them "unsupported — text fallback". So one comparison covers every kind, with strictly better coverage than the dispatch it replaces. `TestPruneUnchangedAgreesWithSerializer` + `TestPruneUnchangedDeactivationIsAChange` (deactivation is caught for free via the "inactive: " prefix) confirm. |
| A-10 | `PruneUnchanged` must live in the config package because leaf removal is package-internal | Removing a leaf touches `values`, `valuesOrder`, `multiValues`, `inactiveValues`, `inactiveMembers` — all unexported, all under `t.mu` (tree.go:26-42, :109-130). Exported removal covers only containers (:387), list entries (:562), leaf-list members (:788) | The prune would need a new exported `Tree` mutation method, widening the config API | Read tree.go:109-130 + the exported Remove* set | **confirmed** — `pruneInactiveLeaves` (tree.go:109-130) is the in-package template; no exported equivalent exists. This makes `config/prune.go` the required home, not merely the consistent one. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `PruneUnchanged` must handle every node kind `diff_tree.go` handles; missing one silently drops config from the compare view | A node kind never appears in compare output despite being changed | Unit test per kind, mirroring `diffWalkNode`'s dispatch (:104-136); table-driven |
| R-2 | Compare at a non-root context path: tree diff needs `len(m.contextPath) == 0` (model_render.go:124), but pruning happens before that | Compare inside `edit bgp` isolates but markers come from LCS | Pruning is independent of the annotation layer, so isolation still works; cover with `.et` and document |
| R-3 | Making `set cli format` session-scoped is a behavior change for anyone relying on it being global | Existing test asserting cross-session visibility; user surprise | User explicitly approved; document in learned summary + `docs/guide/command-reference.md` |
| R-4 | Existing `.et` tests assert on compare output | `.et` failures in `test/editor/` | They assert only that `+` and `listen` appear — both survive pruning; add `.et` asserting unchanged lines are ABSENT |
| R-5 | Threading a session format changes an exported `internal/component/command` signature used by web + cli | Build break in web | Only 4 call sites (A-6); change all in one phase |
| R-6 | Inactive/deactivated nodes interact with pruning (`setOrNop`, serialize_set.go:97-101; `memberDisplay`, diff_tree.go:201-216) | A deactivation change does not show in compare | Test a deactivation-only change (AC-19) |
| R-7 | Two clones per compare on a large config | Slow `show \| compare` | `ActiveContentAtPath` already clones per call (editor.go:390); same cost profile, no regression |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show \| compare` typed in CLI session | → | `ChangedContentAtPath` → `config.PruneUnchanged` | `test/editor/pipe/show-compare-isolates.et` |
| `show \| compare \| format config` | → | same prune, `SerializeSet` path | `test/editor/pipe/show-compare-format-config.et` |
| `show \| compare` with `option changes disable` | → | `forceChanges` on the compare view | `test/editor/pipe/show-compare-forces-markers.et` |
| `set cli format json` in session A | → | session-scoped format; session B unaffected | `TestSetCLIFormatIsSessionScoped` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One leaf added deep in the tree; `show \| compare` | Added line shown with `+`; unchanged leaves outside the changed containers ABSENT |
| AC-2 | Same as AC-1 | Enclosing container lines present (ancestors kept so the change is locatable and the output is valid config) |
| AC-3 | No changes; `show \| compare` | Reports no changes; no error, no full config dump |
| AC-4 | `option changes disable`, then `show \| compare` with a pending change | Markers still shown |
| AC-5 | Whole container added; `show \| compare` | Added container shown with `+`; unchanged siblings pruned |
| AC-6 | Whole container removed; `show \| compare` | Removed container shown with `-` (pruned baseline retains it) |
| AC-7 | Leaf value modified; `show \| compare` | Modified line shown with `*`; unchanged siblings pruned |
| AC-8 | Validation error present, `show \| compare` (pruned) | Compare never highlights the WRONG line: the pruned view suppresses validation styling rather than mis-positioning it. **Line position is not a compare concern** (user, 2026-07-16): compare answers "what changed", `show \| errors` answers "what is wrong". The original wording ("Highlighting lands on the correct line") imported a requirement that never belonged to compare — see Deviations. `show`, `show \| errors`, and the unpruned compare fallback are unaffected |
| AC-9 | `show \| compare \| format config` | Only changed parts, in `set` format |
| AC-10 | `show \| compare \| format tree` (and bare `show \| compare`) | Only changed parts, nested brace format (fmtTree is the default, model_load.go:947) |
| AC-11 | `show \| compare \| format bogus` | Existing error preserved: "unknown format: bogus (use tree or config)" |
| AC-12 | List entry added / removed / modified | Entry isolated correctly; unchanged sibling entries pruned |
| AC-13 | `show \| compare rollback 1` | Baseline resolution unchanged; isolation applies |
| AC-14 | `show \| compare saved` | Baseline resolution unchanged; isolation applies |
| AC-15 | `show \| compare <username>` | Baseline resolution unchanged; isolation applies |
| AC-16 | `set cli format json` in session A, command in session B | Session B unaffected (no leak) |
| AC-17 | `environment cli format default table` in config, no `set cli format` | Session uses `table` (config default still honored) |
| AC-18 | `show` (no compare), `changes` enabled | Full config with gutter — NOT isolated |
| AC-19 | A node deactivated (inactive) as the only change; `show \| compare` | Deactivation visible; not pruned away (R-6) |
| AC-20 | Compare at a nested context path (e.g. after `edit bgp`) | Isolation applies at that path (R-2) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | SSHes in, edits one leaf, runs `show \| compare` | session_factory → Model → ClassifyShowPipes → cmdShowDisplayWithSource → baseline tree → PruneUnchanged (both sides) → Serialize → setViewportData → gutter | `test/editor/pipe/show-compare-isolates.et` |
| 2 | Runs `show \| compare \| format config` | same, `fmtConfig` → FilterSetByPath(SerializeSet) | `test/editor/pipe/show-compare-format-config.et` |
| 3 | Runs `show \| compare` with `option changes disable` | cmdShowDisplayWithSource sets forceChanges → setViewportData annotates | `test/editor/pipe/show-compare-forces-markers.et` |
| 4 | Runs `show \| compare rollback 1` after a bad commit | resolveRollbackBaseline → baseline tree → prune → serialize | `test/editor/pipe/show-compare-rollback-isolates.et` |
| 5 | Two operators SSH in; one runs `set cli format json` | Model A session state only; Model B unaffected | `TestSetCLIFormatIsSessionScoped` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPruneUnchangedKeepsChangedLeaf` | `internal/component/config/prune_test.go` | AC-1 | |
| `TestPruneUnchangedKeepsAncestors` | `internal/component/config/prune_test.go` | AC-2 | |
| `TestPruneUnchangedDropsUnchangedSiblings` | `internal/component/config/prune_test.go` | AC-1 | |
| `TestPruneUnchangedEmptyWhenIdentical` | `internal/component/config/prune_test.go` | AC-3 | |
| `TestPruneUnchangedAddedContainer` | `internal/component/config/prune_test.go` | AC-5 | |
| `TestPruneUnchangedRemovedContainerSymmetry` | `internal/component/config/prune_test.go` | AC-6, A-2 | |
| `TestPruneUnchangedModifiedLeaf` | `internal/component/config/prune_test.go` | AC-7 | |
| `TestPruneUnchangedListEntries` | `internal/component/config/prune_test.go` | AC-12 | |
| `TestPruneUnchangedPerNodeKind` | `internal/component/config/prune_test.go` | R-1, A-3: table-driven over every kind in diff_tree.go:104-136 | |
| `TestPruneUnchangedInactiveNode` | `internal/component/config/prune_test.go` | AC-19, R-6 | |
| `TestChangedContentAtPathRoot` | `internal/component/cli/editor_test.go` | AC-1, AC-10 | |
| `TestChangedContentAtPathSubtree` | `internal/component/cli/editor_test.go` | AC-20 | |
| `TestChangedContentAtPathSetFormat` | `internal/component/cli/editor_test.go` | AC-9 | |
| `TestCmdShowDisplayCompareForcesChanges` | `internal/component/cli/model_commands_show_test.go` | AC-4 | |
| `TestCmdShowDisplayCompareNoChanges` | `internal/component/cli/model_commands_show_test.go` | AC-3 | |
| `TestCompareBaselineTreePerTarget` | `internal/component/cli/model_commands_show_test.go` | AC-13, AC-14, AC-15, A-1, A-4 | |
| `TestSetCLIFormatIsSessionScoped` | `internal/component/cli/model_commands_test.go` | AC-16, R-3 | |
| `TestConfiguredDefaultHonoursConfigWhenNoSessionOverride` | `internal/component/command/pipe_test.go` | AC-17 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no numeric inputs added. `compare rollback N` bounds are existing behavior, unchanged (`resolveRollbackBaseline`, model_commands_show.go:261-286) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-compare-isolates` | `test/editor/pipe/show-compare-isolates.et` | Operator sees only their change, not the whole config | |
| `show-compare-format-config` | `test/editor/pipe/show-compare-format-config.et` | Isolated diff in set format | |
| `show-compare-forces-markers` | `test/editor/pipe/show-compare-forces-markers.et` | Compare works with `changes` disabled | |
| `show-compare-rollback-isolates` | `test/editor/pipe/show-compare-rollback-isolates.et` | Isolation applies to rollback baseline | |
| `show-compare-nested-context` | `test/editor/pipe/show-compare-nested-context.et` | Compare at a nested context path (R-2) | |

### Interop Tests (MANDATORY for protocol features)
N/A — CLI display feature, no wire protocol surface touched.

### Future (if deferring any tests)
None. No deferrals.

## Files to Modify
- `internal/component/config/prune.go` - add `PruneUnchanged(tree, baseline, schema)` beside `PruneInactive`/`PruneActive`
- `internal/component/cli/editor.go` - add `ChangedContentAtPath` beside `ActiveContentAtPath` (:386)
- `internal/component/cli/model_commands_show.go` - compare branch (:85-96) prunes + sets `forceChanges`;
  baseline resolution exposes a tree; fix stale "DB preferences" comment (:204)
- `internal/component/cli/model.go` - `Model` gains session format (workstream 3)
- `internal/component/cli/model_keys.go` - `handleSetCLIFormat` (:711-740) off `env.Set`
- `internal/component/cli/model_mode.go` - pass session format into the pipe entry point (:155)
- `internal/component/cli/model_traceroute.go` (:280), `model_ping.go` (:284) - `ProcessPipesDetectLog` call sites
- `internal/component/command/pipe.go` - session-default parameter; keep `configuredDefault()` as fallback
- `internal/component/web/cli_terminal.go` - pass "no session override" (:258)
- `docs/architecture/config/yang-config-design.md` - `// Design:` target of every touched cli file
- `docs/guide/command-reference.md` - `show | compare` isolation; `set cli format` scoping

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A — no BGP protocol extension.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new config leaf. `compare` is data selection; `format` already has `environment cli format default` (ze-hub-conf.yang:80-93) |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No new command. `show \| compare` and `\| format` both exist |
| CLI grammar (action before identifier) | No | No new command |
| Editor autocomplete | No | `compare` and `format` completions already exist |
| Functional test for new RPC/API | Yes | `test/editor/pipe/*.et` above |
| Pipe completeness | Yes | `ai/rules/pipe-completeness.md` — `compare` must keep composing with `format`, `match`, `head`, `tail` (ClassifyShowPipes :946-977) |
| Env var registration | No | No new env var. `ze.cli.format` already registered (pipe.go:754) |
| Doctor check for runtime dependencies | No | No file path, socket, service, port, or cert material introduced |
| Prometheus counters/metrics | No | No observable runtime state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — compare isolates the changed subtree |
| 2 | Config syntax changed? | No | No new config leaf (verify with grep at completion) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `show \| compare` behavior; `set cli format` scoping |
| 4 | API/RPC added/changed? | [ ] | verify at implementation |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | [ ] | check `docs/guide/` for a config-editor page |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior? | No | - |
| 10 | Test infrastructure changed? | [ ] | only if `.et` needs a new directive |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` — compare isolation may be a row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` — diff/display |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered command/inventory changed? | No | - |
| 16 | Changed files referenced by doc source anchors? | [ ] | grep `docs/` for anchors on every touched file |
| 17 | Existing docs show examples for this area? | [ ] | verify compare output examples against the new isolated output |

## Files to Create
- `test/editor/pipe/show-compare-isolates.et`
- `test/editor/pipe/show-compare-format-config.et`
- `test/editor/pipe/show-compare-forces-markers.et`
- `test/editor/pipe/show-compare-rollback-isolates.et`
- `test/editor/pipe/show-compare-nested-context.et`

→ Decision: no new source file. `PruneUnchanged` joins `prune.go`, `ChangedContentAtPath` joins
  `editor.go`. Adding `diff_prune.go`/`diff_junos.go` was the superseded design (see Failed Approaches).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan; validate A-1..A-8 cheaply first |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint-changed && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — compare view forces markers and reaches a prune hook
   - Tests: `TestCmdShowDisplayCompareForcesChanges` (fails: `forceChanges` not set)
   - Files: `model_commands_show.go` (:85-96)
   - Verify: AC-4; compare reaches a stub prune; output still full
2. **Phase: PruneUnchanged (the reported bug)** — tree-level isolation
   - Tests: all `TestPruneUnchanged*`
   - Files: `internal/component/config/prune.go`
   - Verify: AC-1..AC-3, AC-5..AC-7, AC-12, AC-19; every node kind covered (R-1)
3. **Phase: ChangedContentAtPath** — editor surface mirroring `ActiveContentAtPath`
   - Tests: `TestChangedContentAtPath*`, `TestCompareBaselineTreePerTarget`
   - Files: `editor.go`, `model_commands_show.go` (baseline as tree)
   - Verify: AC-9, AC-10, AC-13..AC-15, AC-20; A-1/A-4 resolved
4. **Phase: Wire compare to the prune** — replace the full-render compare path
   - Tests: the five `.et` files
   - Files: `model_commands_show.go`
   - Verify: AC-1..AC-3, AC-8..AC-11, AC-18; existing `.et` still pass (R-4)
5. **Phase: Session scoping + leak fix** — `set cli format` off `env.Set`
   - Tests: `TestSetCLIFormatIsSessionScoped`, `TestConfiguredDefaultHonoursConfigWhenNoSessionOverride`
   - Files: `model_keys.go`, `model.go`, `model_mode.go`, `model_traceroute.go`, `model_ping.go`,
     `command/pipe.go`, `web/cli_terminal.go`
   - Verify: AC-16, AC-17; precedence session override > config/env > built-in default
6. **Functional tests** → all five `.et` files
7. **RFC refs** → N/A
8. **Full verification** → `make ze-verify`
9. **Complete spec** → audit tables, learned summary, two commits.
   → Constraint: compare isolation and the `set cli format` leak are disjoint concerns
     (planning.md:219). Use separate commits within the closure script.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-20 has implementation with file:line |
| Feature completeness | Every user story 1-5 has a working path and passing test |
| Correctness | `PruneUnchanged` handles every node kind `diffWalkNode` handles (diff_tree.go:104-136) — no silent drop (R-1) |
| Correctness | Both trees pruned symmetrically; removals survive in the baseline (A-2, AC-6) |
| Correctness | Ancestors of every changed node retained; output is valid parseable config (AC-2) |
| Correctness | Isolation applies to BOTH formats (AC-9, AC-10) — this is the whole point of pruning the tree, not the lines |
| Naming | `PruneUnchanged` matches `PruneInactive`/`PruneActive`; `ChangedContentAtPath` matches `ActiveContentAtPath` |
| Data flow | Prune before serialize; no format-specific pruning anywhere |
| Registration over hardcoding | No new per-feature field on a shared/core struct beyond session state |
| Rule: no-layering | Old always-full-render compare path fully replaced, not left beside the new one |
| Rule: no-workarounds | `ai/rules/no-workarounds-for-missing-behavior.md` — no weakening of existing `.et` assertions |
| Rule: buffer-first | `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` — textbuf, no fmt.Sprintf |
| Rule: pipe-completeness | `compare` still composes with `format`, `match`, `head`, `tail` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `PruneUnchanged` exists beside its siblings | `grep -n 'func Prune' internal/component/config/prune.go` shows Inactive, Active, Unchanged |
| `ChangedContentAtPath` exists beside its siblings | `grep -n 'ContentAtPath' internal/component/cli/editor.go` |
| No new YANG leaf added | `git diff --stat internal/component/hub/yang/ze-hub-conf.yang` is empty |
| No new source file for pruning/rendering | `git status` shows no `diff_prune.go` / `diff_junos.go` |
| All five `.et` files exist | `ls -la test/editor/pipe/show-compare-*.et` |
| No `env.Set` in the set-cli path | `grep -n 'env.Set' internal/component/cli/model_keys.go` returns nothing |
| Compare isolates in both formats | `make ze-functional-test` — `show-compare-isolates.et` + `show-compare-format-config.et` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Information disclosure | Compare baselines include other users' changes (`ContentWithoutUser`, editor.go:222). Pruning must only REMOVE nodes, never surface a node the unpruned view would not have shown |
| Information disclosure | Inactive/deactivated nodes must not leak into a view that would otherwise hide them (R-6) |
| Resource exhaustion | Pruning is a single bounded walk; two clones per compare, same profile as `ActiveContentAtPath` (R-7) |
| Input validation | `\| format` values still validated by `ClassifyShowPipes` (:951-955); compare targets unchanged |

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
| A CLI display preference does not belong in YANG (config-surface.md decision table answers "no" to all three YANG questions) | `environment cli format default` (ze-hub-conf.yang:82-92) already establishes `environment/` as the home for CLI presentation defaults, with env var + `set cli` override | User pointed to the existing `environment cli format default table` | Recommendation changed from a zefs per-user key to `environment cli compare default`. Then superseded entirely (below). Caught during design, before any code. |
| The compare style needed a NEW config surface (`environment cli compare default` + `set cli compare` + a junos renderer) | `ClassifyShowPipes` (model_load.go:946-977) already separates data selection (`compare`) from presentation (`format`), and `format` already accepts `tree` and `config`. The two "styles" I was going to build ARE those two existing formats | User: "we should have `show \| compare` limiting the data presented to the part of the yang tree modified and `\| format ..` for the format" | Deleted an entire workstream: no YANG leaf, no env var, no `set cli compare`, no `diff_junos.go`. Caught during design, before any code. |
| Pruning belonged at the rendered-diff-line level (`diff_prune.go` over `[]diffLine`) | Data-selection pipes prune the TREE then serialize (`ActiveContentAtPath` → `config.PruneInactive`, editor.go:386-400 / prune.go:13). Line-pruning would have fixed `\| format tree` and silently left `\| format config` rendering the whole config, because the two formats use different serializers (model_commands_show.go:191-195) | Followed the user's data-vs-format separation to its implementation and found `PruneInactive`/`PruneActive` | Wrong layer, wrong file, and a half-fix that would have passed a naive test. Caught during design, before any code. |
| (A-3) `PruneUnchanged` could mirror `pruneActiveNode`'s switch (prune.go:31-68) | That switch handles only `*ContainerNode` and `*ListNode`. Leaf-level pruning is out-of-band via the unexported `tree.pruneInactiveLeaves()` (prune.go:80 → tree.go:109-130), because inactivity is engine state on the parent Tree rather than a schema concern. `PruneUnchanged` compares VALUES, so it must handle every kind `diffWalkNode` handles (diff_tree.go:104-136) — about nine | Audit step 3: read prune.go in full before writing any code | Implementation shape changed, not the design: same API, same file, same data flow. Writing a nine-way comparison dispatch would have duplicated diff_tree.go and risked "what prunes" diverging from "what the gutter marks". Replaced by A-9 (serialize-and-compare), which needs no per-kind dispatch at all. Layer confirmed by A-10. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Persist the style per-user in zefs (`meta/option/{username}/compare`), following `KeyHistory` (pkg/zefs/keys.go:23) | zefs holds per-user session state (history), not presentation defaults | Superseded twice; ultimately no new setting at all |
| `option compare <style>` alongside author/date/source/changes | `option` toggles which metadata columns appear; it is not a formatting-default surface (editor.go:51) | No new setting |
| `environment cli compare default` + `ze.cli.compare` + `set cli compare` + `diff_junos.go` | The `format` pipe already IS the presentation surface. `\| format tree` = nested-with-context; `\| format config` = flat set lines (Ze's `display set` equivalent). A new style knob would have duplicated the pipe grammar in config | `show \| compare \| format tree\|config` — both already exist |
| `diff_prune.go` pruning the rendered `[]diffLine` list | Wrong layer: only fixes the tree format; `\| format config` uses a different serializer (model_commands_show.go:192) and would still dump the whole config | `config.PruneUnchanged(tree, baseline, schema)` in prune.go, before serialization |
| Per-invocation `show \| compare junos` | The compare pipe's argument slot is the BASELINE; `NormalizeCompareTarget` (:245-257) treats unknown tokens as usernames, so `\| compare junos` already means "diff against user junos" | Moot — style is a `format` concern |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Proposed a new surface (config home, then a new file/layer) without grepping for the existing sibling that already solves it. Happened TWICE in one design conversation: config home (`environment cli format default`), then pruning layer (`PruneInactive`/`PruneActive`) | 2 (this spec) | `ai/rules/design-context.md` already says "grep ze before proposing, never default to trained instincts". The rule exists and was not followed. The gap is not a missing rule but a missing TRIGGER: both misses were "where does this new thing go?" questions | Before proposing any new setting, file, or layer, grep for the nearest existing sibling of the same species and read it. Decide at completion whether `design-context.md` needs a worked example for "where does this go" questions specifically. |

## Design Insights
- **The existing compare functional tests were vacuous, which is part of why this bug survived.**
  `test/editor/commands/compare-with-changes.et` and `test/editor/pipe/show-pipe-compare.et` both
  build their "changed" config by adding `listen 0.0.0.0:179`. There is no `listen` leaf in
  `ze-bgp-conf.yang`. The parse therefore fails, `SetWorkingContent` (editor_commands.go:66-78)
  sets `treeValid = false`, and the editor drops into raw-text mode where the schema tree diff
  never runs. Their assertions (`contains=+`, `contains=listen`) pass anyway, because raw text
  still differs from the serialized baseline and still gets a gutter. So two tests named for
  compare have been exercising only the LCS fallback. Any `.et` touching tree-aware behavior MUST
  use leaves that exist in the schema; a passing compare test proves nothing otherwise.
  → Found by: the new isolation `.et` failing with `viewport should not contain "peer1"` while the
    equivalent unit test passed. The unit test used `SetValue` (which keeps the tree valid); the
    `.et` used `load file absolute replace` with an unparseable config.
- **Data-selection pipes prune the tree; presentation pipes serialize it.** `active`/`inactive`
  (`PruneInactive`/`PruneActive` → `ActiveContentAtPath`) are the template. `compare` is the third
  member of that family and was the only one not following it.
- Pruning before serialization is what makes `| format tree` and `| format config` both isolate
  with one implementation. Any fix applied after serialization can only ever fix one format.
- The `compare` pipe and the `format` pipe look symmetric but are not: `format`'s argument is the
  *kind*; `compare`'s argument is the *baseline*, with unknown tokens falling through to usernames
  (`NormalizeCompareTarget`, :245-257). No option can ever be added to `compare`'s argument slot.
- `env.Set` as a session-override mechanism is structurally wrong for anything per-session: it
  writes process-global state (`env.go:111-119`). `set cli format` is the existing instance.

## Core Insight
The feature was already designed — in the pipe grammar. `ClassifyShowPipes` separates data
selection from presentation, and `PruneInactive`/`PruneActive` show exactly how a data-selection
pipe is implemented: clone, prune, serialize. `compare` was the one data-selection pipe that never
pruned, so it selected *everything* and leaned on a marker column to imply selection. The fix is
not a new renderer, a new style knob, or a new config surface. It is making `compare` do what its
siblings already do.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `compare` prunes the YANG tree; `format` serializes it | Prune rendered diff lines; add a style knob + junos renderer | User's direction, and it matches `ClassifyShowPipes`'s existing split. Pruning the tree makes both formats isolate with one implementation; pruning lines fixes only `\| format tree`. |
| `PruneUnchanged(tree, baseline, schema)` in `config/prune.go` | A new `diff_prune.go` in the cli package | Sibling of `PruneInactive`/`PruneActive` (prune.go:13,23), same in-place-on-a-clone contract. Lives where the other prunes live — and per A-10 it MUST: leaf removal touches five unexported Tree fields under `t.mu` (tree.go:109-130) with no exported equivalent, so a cli-side prune is not possible without widening the config API. |
| "Unchanged" means "serializes identically" — compare child subtrees by serialization, no per-node-kind dispatch | A nine-way switch mirroring `diffWalkNode` (diff_tree.go:104-136) | A-3 broke: `pruneActiveNode`'s switch covers only Container+List, so there is nothing to reuse, and no tree-vs-tree comparison exists in the config package (`DiffMaps` works on `map[string]any`, `DiffBlocks` on `BlockState`, `PendingChanges` on the MetaTree). A hand-written dispatch would duplicate diff_tree.go and could diverge from it — pruning a node the gutter would have marked changed, or keeping one it would not. Serialization is already the comparison of record for inline containers (`diffContainerInline`, :265-277) and fallback kinds (`serializeNodeText`, :518). It also gives the semantically right rule: if two subtrees render identically, the operator has no reason to see them. |
| `ChangedContentAtPath` on Editor | Inline the prune in `cmdShowDisplayWithSource` | Mirrors `ActiveContentAtPath`/`InactiveContentAtPath` (editor.go:386-418) exactly, including the path-walk fallback. |
| Prune BOTH trees symmetrically | Prune only the working tree | A removed node exists only in the baseline. Pruning both keeps it, so the existing gutter renders it `-` with no change to the annotation layer. |
| No new YANG leaf, env var, or CLI command | `environment cli compare default` + `set cli compare` | `format` already owns presentation and already has a config home (`environment cli format default`). A compare-specific style knob would duplicate the pipe grammar in config. |
| Fix the `set cli format` leak in this spec | Split to its own spec | User chose to keep both. Disjoint concerns get separate COMMITS within the closure (planning.md:219). |

## Known Limitations
- **Validation errors are not highlighted in a pruned `show | compare` view** — by design, not by
  compromise. Line position is not a compare concern: compare answers "what changed", `show | errors`
  answers "what is wrong" (user, 2026-07-16). The view is also not the validated string, so its line
  numbers could not position them honestly anyway. `show` highlights as before.
  → Note for whoever revisits this: `show <path>` (subtree) has the SAME latent mismatch today and
    always has — it renders a subtree while errors are numbered against the full serialization
    (`runValidation` validates `ContentAtPath(nil)`, model_commands_commit.go:106). Pre-existing, out
    of scope here, and only fixable by keying validation errors on config PATH rather than line.
- No per-invocation style beyond `tree`/`config` — those are the only formats `ClassifyShowPipes`
  accepts (:951-955), and this spec does not add more.
- With `| format config`, the tree diff is bypassed: set-format text fails `config.NewParser` in
  `computeTreeAnnotatedDiff` (diff_tree.go:31-32) and falls back to the LCS line diff (:56-59).
  Isolation still works (pruning happens earlier) and the markers are correct — A-5 confirms it with
  `show-compare-format-config.et`. Nothing is deferred here: the fallback produces the right answer,
  so pointing `computeTreeAnnotatedDiff` at `parseConfigWithFormat` would be a tidy-up with no
  behavior change, not a fix.
- Compare at a non-root context path uses the LCS fallback for markers (`model_render.go:124`
  requires root). Isolation is unaffected. (R-2, AC-20)
- The session-scoped format applies to the CLI `Model`. The web CLI (`cli_terminal.go:258`) has no
  `set cli format` surface and continues to use the configured/env default (A-8).
- `option` columns (author/date/source/changes) remain session-only and unpersisted (editor.go:51,
  "In-memory show column preferences (sticky per session)"). Pre-existing and untouched by this
  spec — recorded only because `model_commands_show.go` carried a stale comment claiming they came
  from a DB, which is corrected here. Not deferred work: nothing was dropped from this scope.

## RFC Documentation
N/A — no protocol work.

## Implementation Summary

### What Was Implemented
- `config.PruneUnchanged(tree, baseline, schema)` in `config/prune.go`, beside `PruneInactive`/
  `PruneActive`, with the unexported `Tree.removeValue` it needs. "Unchanged" = "serializes
  identically" (`serializeNode`), so one rule covers all nine node kinds.
- `compareView` in `model_commands_show.go`: ONE compare path for every source (working / confirmed /
  saved), prunes both directions, honors the `format` pipe, forces the gutter, and opts out of
  validation highlighting. `showAlternateSource` lost its compare branch entirely.
- `compareViewUnpruned` preserves `renderRawSourceAtPath`'s "legacy files stay visible" contract when
  a tree cannot be parsed.
- `annotatedViewOf` so a pruned tree can still carry metadata columns.
- `set cli format` moved off `env.Set` onto per-session `Model.cliFormat`; `sessionFormat` threaded
  through `ProcessPipesDefaultFormat*`/`ProcessPipesDetectLog` (4 call sites). Precedence:
  session > config/env > text.
- 5 new `.et`, 2 de-vacuumed `.et`, 11 prune unit tests, 8 cli/command unit tests.

### Bugs Found/Fixed
- Bug 1 (reported): `show | compare` renders the entire config — `compare` never prunes, unlike its
  `active`/`inactive` siblings; `diff_tree.go` emits unchanged lines and `renderDiffLines` (:69-92)
  prints them all.
- Bug 2 (found): compare silently degrades to a plain `show` when the `changes` column is disabled —
  `model_render.go:122-123` gates the gutter and the compare path never sets `forceChanges`.
- Bug 3 (found): `set cli format` leaks across concurrent sessions — `env.Set` (env.go:111-119)
  writes a process-global cache and `os.Setenv`.
- Bug 4 (found, doc): `model_commands_show.go:204` claims show columns come from "DB preferences";
  they are in-memory session state (editor.go:51).

### Documentation Updates
- `docs/guide/config-editor.md:25` — `show | compare` row now states it shows only the parts that
  differ. This was the one doc making a claim the change invalidates.
- `docs/guide/command-reference.md:2250` — **no change needed, and this is evidence**: it already
  described `set cli format json` as a "(session override)", with a source anchor to
  `internal/component/cli/model_keys.go -- handleSetCLIFormat`. The documentation has claimed
  session scoping all along while `env.Set` made it process-global. The leak fix aligns the code to
  the documented contract rather than changing it; the doc was aspirational, not describing reality.
- `docs/features.md` — no compare claim to update (grep: no `compare` hit outside unrelated prose).
- `docs/architecture/config/yang-config-design.md` — no `compare` / `diff gutter` claim to update
  (grep returns nothing), despite being the `// Design:` target of the touched cli files.
- `docs/guide/command-catalogue.md:334,340` — rows describe WHICH command diffs config, not what it
  presents; still accurate.

**Not yet done (see Executive Summary "Not done"):** full 17-row Documentation Update Checklist
sign-off, `make ze-doc-test`.

### Deviations from Plan
- **`ChangedContentAtPath` on Editor → `comparePrunedViews` on Model.** The spec said to mirror
  `ActiveContentAtPath` (editor.go:386) exactly. That method is format-UNAWARE: it always calls
  `config.Serialize`/`SerializeSubtree`, which is why `show | active | format config` does not
  produce set format today. Compare must honor the format pipe (AC-9), and the format-aware
  serializer is `renderTreeAtPath` (model_commands_show.go:187-202), a Model method that also
  needs `m.contextPath`. Mirroring the Editor method exactly would have lost format support and
  broken AC-9, so the prune lives beside the format-aware serializer instead. `PruneUnchanged`
  itself is unaffected and stayed in `config/prune.go` as specced (required by A-10).
- **Tests for AC-9/AC-20 moved** from `editor_test.go` (`TestChangedContentAtPath*`) to
  `model_commands_show_test.go` (`TestCmdShowDisplayCompareFormatConfig`, etc.), following the
  code above.
- **`AnnotatedView` gained a tree-parameterized sibling** (`annotatedViewOf`, editor_annotated.go).
  Not in the spec's Files to Modify. Needed because `AnnotatedView` hardcodes `e.tree` (:19), so a
  pruned tree could not be annotated, and compare with metadata columns enabled would have silently
  lost its columns — a regression the spec's "Behavior to preserve" forbids.
- **Two failing pre-existing tests fixed at the fixture, not the assertion.**
  `TestCmdShowDisplayCompare` and `TestCmdShowPipeCompareRollback` asserted `originalContent` was
  non-empty while making NO change, so under AC-3 both sides now correctly prune to empty. Their
  intent ("compare must supply a baseline") is sound; only the fixture relied on the old
  dump-everything behavior. Each now makes a real edit and additionally asserts the baseline
  carries the pre-change value. Deleting the assertions would have been the workaround.
- **New test file** `internal/component/config/prune_unchanged_test.go` rather than extending
  `prune_test.go`, following the existing `prune_inactive_leaf_test.go` precedent of splitting
  prune tests by concern.
- **AC-8 CORRECTED (mis-specified; user signed off 2026-07-16: "the line does not matter for
  | compare").** Original: "Highlighting lands on the correct line." Delivered: the pruned compare
  view suppresses validation highlighting instead.
  **Why this is a correction, not a reduction:** line position is not something compare owes the
  operator. `compare` answers "what changed"; `show | errors` answers "what is wrong". The original
  AC imported a line-precision requirement from the plain `show` view, where it does belong. Nothing
  a user wants from compare is lost by dropping it — so the AC was wrong, and the code is right.
  **Why the original is not achievable as specced:** `runValidation` validates
  `m.editor.ContentAtPath(nil)` (model_commands_commit.go:106) — the full config serialized at root
  — with the stated intent "so that line numbers align with what the user sees" (:100-101).
  `highlightValidationIssues` maps display→working line via `lineMapping` and looks the error up by
  that number (model_render.go:223,230). Before this spec, the compare view's content WAS that exact
  validated string, so the numbers lined up. A pruned view is a different string, so they cannot.
  **Rejected alternative:** map pruned lines back to full-content line numbers by aligning the two
  serializations. Both are serializations of the same working tree, so every pruned line does appear
  verbatim in the full one — but identical lines (closing braces; the same leaf value under two
  peers) make a greedy or LCS alignment bind an error to the wrong parent's line. That reintroduces
  the exact failure mode (a wrong marker), so it is not a fix.
  **Reasoning:** a wrong highlight actively misleads — it marks an innocent line as erroneous and is
  worse than no marker. `show` and `show | errors` still highlight/report accurately, and the
  unpruned compare fallback keeps its correct mapping. Fail-safe over fail-wrong.
  **This is a REGRESSION introduced by this spec, caught by auditing AC coverage rather than by any
  test** — no test covered AC-8 before or after; the two added ones pin the new contract.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Isolation (reported bug) | done | `config/prune.go` PruneUnchanged; `model_commands_show.go` compareView | `show-compare-isolates.et` |
| 2. Marker suppression when `changes` off | done | `model_commands_show.go` forceChanges | `show-compare-forces-markers.et` |
| 3. Cross-session `set cli format` leak | done | `model.go` cliFormat, `model_keys.go`, `command/pipe.go` | `TestSetCLIFormatIsSessionScoped` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestPruneUnchangedDropsUnchangedSiblings`, `show-compare-isolates.et` | |
| AC-2 | done | `TestPruneUnchangedKeepsAncestors` (also asserts the output re-parses) | |
| AC-3 | done | `TestPruneUnchangedEmptyWhenIdentical`, `TestCmdShowDisplayCompareNoChanges` | |
| AC-4 | done | `TestCmdShowDisplayCompareForcesChanges`, `show-compare-forces-markers.et` | |
| AC-5 | done | `TestPruneUnchangedAddedContainerKeptWhole` | |
| AC-6 | done | `TestPruneUnchangedRemovalIsVisibleOnlyViaBaseline` | |
| AC-7 | done | `TestPruneUnchangedKeepsChangedLeaf` | |
| AC-8 | done | `TestCmdShowDisplayComparePrunedSuppressesValidationHighlight`, `TestSetViewportDataHonorsNoValidationHighlight` | AC text corrected (mis-specified); user signed off 2026-07-16 — line position is not a compare concern |
| AC-9 | done | `TestCmdShowDisplayCompareFormatConfig`, `show-compare-format-config.et` | |
| AC-10 | done | `show-compare-isolates.et` (fmtTree is the default) | |
| AC-11 | done | `TestCmdShowPipeFormatInvalid` (pre-existing, still passes) | |
| AC-12 | done | `TestPruneUnchangedListEntriesIsolated` | |
| AC-13 | done | `TestCmdShowPipeCompareRollback`, `show-compare-rollback.et` | |
| AC-14 | done | `TestCmdShowDisplayCompareAlternateSourceIsolates`, `show-compare-saved-source.et` | |
| AC-15 | done | `TestCmdShowCompareUsername` (pre-existing, passes) | |
| AC-16 | done | `TestSetCLIFormatIsSessionScoped` | |
| AC-17 | done | `TestSessionFormatOverridesConfiguredDefault` | |
| AC-18 | done | full `cli` suite green — `show` path untouched (compare branch is opt-in on compareTarget) | |
| AC-19 | done | `TestPruneUnchangedDeactivationIsAChange` | |
| AC-20 | done | `show-compare-nested-context.et` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPruneUnchanged*` (8 planned) | done | `config/prune_unchanged_test.go` | 11 written |
| `TestPruneUnchangedPerNodeKind` | **changed** | `TestPruneUnchangedAgreesWithSerializer` | A-9 removed the per-kind dispatch, so the per-kind test became a serializer-agreement test |
| `TestChangedContentAtPath{Root,Subtree,SetFormat}` | **changed** | `TestCmdShowDisplayCompare{,FormatConfig}`, `show-compare-nested-context.et` | Follows the code (Deviations: prune lives beside the format-aware serializer) |
| `TestCompareBaselineTreePerTarget` | **changed** | `TestCmdShowPipeCompareRollback`, `TestCmdShowCompareUsername`, `TestCmdShowDisplayCompareAlternateSourceIsolates` | Split across per-target tests, two pre-existing |
| `TestApplyEnvConfigCLICompare` | **skipped** | — | N/A: the `cli.compare` leaf was removed from the design (no new config surface) |
| `TestCmdShowDisplayCompareForcesChanges` | done | `model_commands_show_test.go` | |
| `TestSetCLIFormatIsSessionScoped` | done | `model_commands_test.go` | |
| `TestConfiguredDefaultHonoursConfigWhenNoSessionOverride` | done | `TestSessionFormatOverridesConfiguredDefault` | renamed |
| Functional `.et` (5 planned) | done | `test/editor/pipe/` | 5 written + 2 de-vacuumed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `config/prune.go`, `config/tree.go` | done | |
| `cli/model_commands_show.go`, `model.go`, `model_keys.go`, `model_mode.go`, `model_ping.go`, `model_traceroute.go` | done | |
| `command/pipe.go`, `web/cli_terminal.go` | done | |
| `cli/editor.go` (`ChangedContentAtPath`) | **changed** | Superseded: `comparePrunedViews` on Model (Deviations) |
| `cli/editor_annotated.go` | added | Not in plan; required so a pruned tree keeps metadata columns |
| `cli/model_render.go` | added | Not in plan; required for the AC-8 opt-out |
| `hub/yang/ze-hub-conf.yang`, `config/apply_env.go` | **skipped** | No new config surface — the `format` pipe already owns presentation |
| `docs/guide/config-editor.md` | done | |
| `docs/guide/command-reference.md` | not needed | Already documented the session override; the code moved to the doc |

### Audit Summary
- **Total items:** 20 ACs + 3 requirements + 9 test groups + 10 file groups
- **Done:** 20/20 ACs; 3/3 requirements; all file groups
- **Partial:** none
- **Skipped:** `TestApplyEnvConfigCLICompare`, `ze-hub-conf.yang`, `apply_env.go` — all consequences of the user-approved redesign (no new config surface), not omissions
- **Changed:** AC-8 text corrected (mis-specified; signed off 2026-07-16); 3 test names/locations following the code; `ChangedContentAtPath` → `comparePrunedViews`. All in Deviations. No AC left unmet.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `show \| compare` limits data to the modified part of the YANG tree (THE reported bug) | functional test | `show-compare-isolates.et` PASS: changes one peer's ip; asserts `10.9.9.9`+`alpha`+`bgp` present AND `beta`/`10.0.0.2`/`65002`/`65000`/`router-id` ABSENT. Every one of those appeared before the fix. |
| ...for every source, not just the working config | functional test | `show-compare-saved-source.et` PASS: `set` → `save` → `show saved \| compare`; asserts `alpha`/`10.0.0.1`/`65001`/`65000` absent. Caught by the /ze-review gate as a BLOCKER. |
| `\| format` controls presentation, compare controls data | functional test | `show-compare-format-config.et` PASS: `\| format config` shows `set bgp`+`10.9.9.9`+`*` marker, and NOT `65001`/`65000`/`router-id`. Proves the prune is format-agnostic (the line-pruning design would have failed exactly here). |
| Compare works with the `changes` column off | functional test | `show-compare-forces-markers.et` PASS: `option changes disable` then `show \| compare` still shows `+` and `maximum-paths`. |
| Isolation survives a nested context path | functional test | `show-compare-nested-context.et` PASS: `edit bgp` then compare; `beta`/`10.0.0.2`/`65002` absent. |
| One session's `set cli format` no longer changes another's output | unit test | `TestSetCLIFormatIsSessionScoped` PASS: asserts session B unaffected AND `env.Get("ze.cli.format") != "json"` (no process-global write). `TestSessionFormatOverridesConfiguredDefault` pins precedence session > config > text. |
| Compare never mis-positions a validation error | unit test | `TestCmdShowDisplayComparePrunedSuppressesValidationHighlight` + `TestSetViewportDataHonorsNoValidationHighlight` (flag is read, not decorative). AC-8 narrowed — see Deviations. |
| The prune rule cannot diverge from what the operator sees | unit test | `TestPruneUnchangedAgreesWithSerializer`; `serializeNode` (serialize.go:256) covers the same nine kinds as `diffWalkNode` (diff_tree.go:104-136). |
| Regression suites | test run | 4/4 packages lint 0 issues; `cli`/`config`/`command`/`web` unit suites PASS; **164/164** editor functional tests PASS (160 before, +4 new); `audit-test-relaxation.py` clean. |

## Review Gate

### Run 1 (initial)

Pre-checks: `audit-test-relaxation.py` reported one `[RELAXED]` in
`internal/component/bgp/plugins/rib/rib_replay_test.go` — **another session's file, not in
this diff** (that session committed during the run; the audit is clean at Run 2).
`make ze-validate` reported 26 `exported symbol has no cross-package non-test caller` ISSUEs.
All 26 verified pre-existing via `git grep <symbol> HEAD` — the validator flags them because the
FILE changed, not the symbol. Evidence: `ProcessPipesDefaultFormat` had no non-test caller at HEAD
either; `AnnotatedView` was already cli-package-internal at HEAD (`model_commands_show.go:107,179`).
The one new exported symbol, `PruneUnchanged`, is ABSENT from the list — the validator confirms its
cross-package non-test caller.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `show confirmed \| compare` and `show saved \| compare` still dumped the whole config — the reported bug was only half fixed. A SECOND compare path (`showAlternateSource`) took PRE-RENDERED text and so structurally could not prune. Reachable: `cmdShowPipe` lifts confirmed/saved out of `args[0]` (model_load.go:571-576) and passes BOTH source and CompareTarget (`:617`) | `model_commands_show.go:65-74` → `showAlternateSource:145-161` | fixed — compare unified into `compareView`, one path for every source |
| 2 | ISSUE | Unparseable baseline regressed from raw legacy text to EMPTY: `parseSourceTree` returns nil → `renderTreeAtPath(nil,…)` returns "" (`:189` guard), so a legacy/corrupt committed config diffed against nothing and rendered the entire config as added. The old path returned raw content (`renderRawSourceAtPath:164-166`, "so legacy files remain visible instead of blank"). Compounded by a FALSE CLAIM in the diff's own comment asserting it matched that contract | `model_commands_show.go:232-235` | fixed — `compareViewUnpruned` falls back to the unpruned text views |
| 3 | ISSUE | No functional test for `show saved\|confirmed \| compare`; all five new `.et` files used the working-config path only. This gap is why finding 1 survived | `test/editor/pipe/` | fixed — added `show-compare-saved-source.et` |
| 4 | NOTE | `resolveCompareBaseline` reduced to a single caller; would become dead if finding 1 were fixed by pruning that path too (no-layering) | `model_commands_show.go` | resolved — it is now the deliberate unpruned fallback for unparseable input (finding 2's fix), with a caller and a documented purpose |

### Fixes applied
- **Finding 1 (BLOCKER):** `compareView` is now the single compare path for every source, running
  BEFORE per-source rendering because compare needs TREES, not text. `showAlternateSource` lost its
  compare branch and its `compareTarget`/`format` params entirely — the second path no longer
  exists rather than being kept in step. `sourceTree` resolves working/confirmed/saved uniformly.
  Chose this over duplicating the prune into `showAlternateSource` (`[workaround]`): two compare
  paths is precisely what allowed the divergence.
  Regression test: `TestCmdShowDisplayCompareAlternateSourceIsolates` (RED first: asserted the full
  config came back containing `local 65000`).
- **Finding 2 (ISSUE):** `compareViewUnpruned` restores the unpruned text views when either tree is
  unparseable, preserving `renderRawSourceAtPath`'s visibility contract. The false comment is gone;
  the replacement states plainly that isolation is lost there and visibility is not.
  Regression test: `TestCmdShowDisplayCompareUnparseableBaselineStaysVisible` (RED first:
  `originalContent` was empty).
- **Finding 3 (ISSUE):** `test/editor/pipe/show-compare-saved-source.et` drives
  `set → save → show saved | compare` through the TUI and asserts the untouched peer is pruned.
- **Finding 4 (NOTE):** resolved as a side effect of finding 2's fix.

### Run 2 (after fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | — | none | — | — |

Pre-checks: `audit-test-relaxation.py` **clean** (no tests deleted or weakened).
`make ze-validate`: the same 26 pre-existing ISSUEs, none naming any symbol from this diff
(`grep -E 'compareView|sourceTree|sourceContent|PruneUnchanged|comparePruned|annotatedViewOf|removeValue'`
returns nothing).
Wiring: every new symbol has a non-test caller. Removed-behavior audit: each deleted branch's
invariant re-established and named (compare-on-alternate-sources → `compareView`; `forceChanges`
→ both branches; `"(empty configuration)"` → `compareViewUnpruned` via `sourceContent`).
Verification: 4/4 packages lint 0 issues; 4/4 package unit suites pass; 164/164 editor
functional tests pass.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `renderRawSourceAtPath` parses a tree before serializing; all four targets route through it. `TestCmdShowPipeCompareRollback`, `TestCmdShowCompareUsername` (pre-existing), `TestCmdShowPipeCompareSavedScopesPath` pass. |
| A-2 | confirmed | `TestPruneUnchangedRemovalIsVisibleOnlyViaBaseline`: the deleted peer appears ONLY in the baseline direction. |
| A-3 | **broken** | `pruneActiveNode` switches only on Container+List; leaves go via the unexported `pruneInactiveLeaves`. Nothing to reuse. Mistake Log row filed; superseded by A-9. Design unaffected (same API, file, data flow). |
| A-4 | confirmed | `ContentWithoutUser` clones + reverts on a tree before serializing; the tree is extractable. |
| A-5 | confirmed | `show-compare-format-config.et` asserts the marker survives the LCS fallback. The initial `contains=+` failure was the TEST (a modified leaf marks `*`, diff.go:24), not the code. |
| A-6 | confirmed | Re-grep at audit: exactly 4 non-test call sites, all updated; `go build ./...` clean. |
| A-7 | confirmed | Every `ze.cli.format` reference is in-process (`apply_env.go:53`, `model_keys.go:721`, `pipe.go:754,760`); no subprocess consumer, so the `os.Setenv` side effect was unused. |
| A-8 | confirmed | No `set cli format`/`handleSetCLIFormat` in `internal/component/web/`; the web terminal passes `""`. |
| A-9 | confirmed | `serializeNode` (serialize.go:256) covers the same nine kinds as `diffWalkNode` (diff_tree.go:104-136). `TestPruneUnchangedAgreesWithSerializer`, `TestPruneUnchangedDeactivationIsAChange`. |
| A-10 | confirmed | `pruneInactiveLeaves` clears five unexported fields under `t.mu`; exported removal covers only containers/list entries/leaf-list members. `config/prune.go` is the REQUIRED home. |

None left `unvalidated`. A-3 broke; recorded in the Mistake Log with its replacement (A-9).

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/prune_unchanged_test.go` | yes | `ls -la` → 11K |
| `test/editor/pipe/show-compare-saved-source.et` | yes | `ls -la` → 1.6K |
| `plan/learned/1160-cli-compare-isolate-changes.md` | yes | `ls -la` → 7.8K |
| `show-compare-{isolates,format-config,forces-markers,nested-context}.et` | yes | all 5 run green under `ze-test editor -p show-compare` |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| `show \| compare` (working) | `show-compare-isolates.et` | yes — `grep config.PruneUnchanged` → `model_commands_show.go:309,312`, non-test, cross-package. `ze-validate` does NOT flag `PruneUnchanged`, confirming a cross-package non-test caller |
| `show saved \| compare` | `show-compare-saved-source.et` | yes — `cmdShowPipe` → `cmdShowDisplayWithSource` → `compareView` (single path, all sources) |
| `show \| compare \| format config` | `show-compare-format-config.et` | yes |
| `option changes disable` + compare | `show-compare-forces-markers.et` | yes |
| `set cli format` isolation | `TestSetCLIFormatIsSessionScoped` | yes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/config-editor.md:25` — compare "shows only the parts that differ" | `compareView` → `config.PruneUnchanged` (model_commands_show.go:309,312); `show-compare-isolates.et` | yes |
| `docs/guide/command-reference.md:2250` — `set cli format` "(session override)" | Was ASPIRATIONAL (env.Set was process-global); now true via `Model.cliFormat` + `TestSetCLIFormatIsSessionScoped`. No edit needed — the code moved to the doc | yes |
| `docs/features.md` — no compare claim | `grep compare docs/features.md` → only unrelated IS-IS prose | yes |
| `docs/architecture/config/yang-config-design.md` — no compare/diff-gutter claim | `grep -n "compare\|diff gutter"` → no hits, despite being the `// Design:` target | yes |
| `docs/guide/command-catalogue.md:334,340` | Rows name WHICH command diffs, not what it presents; unaffected | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A)
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
- [ ] Boundary tests for all numeric inputs (N/A — no numeric inputs)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A — CLI display only)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-cli-compare-isolate-changes.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cli-compare-isolate-changes.md` only
