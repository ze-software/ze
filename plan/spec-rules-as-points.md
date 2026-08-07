# Spec: rules-as-points

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/rules-as-points.md` |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The unit of enforcement is one rule FILE, and it needs to be one instruction.**

Every hook refusal, review finding and digest entry names a file. `performance.md`
is 902 lines and nothing in the repository can name the sentence inside it that a
check enforces. Three problems follow from that one fact, and none is fixable
without changing the unit:

| Problem | Why the file granularity causes it |
|---------|-----------------------------------|
| We cannot tell which instructions have a machine behind them | `Enforces` in the Hook-to-Rule Mapping table says `performance.md`. 44 checks against 27 files says nothing about which of the roughly 1,469 instructions are gated |
| We cannot count what any instruction ever prevented | A refusal cites a file, so every refusal in a 902-line rule aggregates to one counter |
| A reworded instruction silently keeps its gate | The check still names the file. The sentence it was written against can be deleted and nothing goes red |

**Goal.** One instruction becomes one checked-in file whose PATH is its id. The
rendered `ai/rules/<rule>.md` becomes generated from those files, so agents keep
reading exactly the file they read today. Then the 44 checks declare which points
they enforce, and a gate reports gated, ungated and dangling.

Source of the design: `plan/handover/07-rules-as-points.md`, decisions D1 to D6.

## Required Reading

### Architecture Docs

- [ ] `plan/handover/07-rules-as-points.md` - the design this spec implements
  → Decision: the path is the id. No content hash. Change detection is `git log` on the point file, so a reworded instruction shows as a one-file diff in review rather than a dangling reference
  → Constraint: D6 makes the byte-identical round trip the first acceptance criterion and it is not negotiable. If tables or nested lists cannot round-trip, STOP and report rather than accept a lossy split
- [ ] `plan/learned/1228-rule-format-condensed-eager-load.md` - why the current format exists
  → Constraint: directives and rationale interleave WITHIN paragraphs, which is why `## Directives` is recommended and not required. A mechanical split at sentence granularity is a semantic rewrite. The unit must be a BLOCK
  → Constraint: a new sibling file under `ai/rules/` silently joins every `*.md` glob. `CONDENSED.md` appeared as a phantom 90th rule this way
- [ ] `plan/learned/1328-rule-corpus-merge-and-line-ref-strip.md` - the consolidation to the present corpus
  → Constraint: `CORE.md` membership is DERIVED from the precedence ladder, never listed. Read `make ze-rules-router-report` after any change to corpus size, and do not read a smaller core as a loss
- [ ] `ai/rules/rule-format.md` - the single-file format this work replaces
  → Constraint: ~~two of its directives are invalidated by this work and must be rewritten, not merely extended. "Edit them directly" is contradicted by D3, and "keep each bullet on ONE physical line" is a constraint a per-point file removes~~
  → Corrected 2026-08-07, phase 5: only ONE directive is invalidated. "Edit them directly" is contradicted by D3, as stated. **"Keep each bullet on ONE physical line" SURVIVES and must stay.** `condense_body` in `scripts/dev/rules_condensed.py` reads the RENDERED rule, never the points, so the constraint is unchanged by the split. Measured: in a section that already carries a prose paragraph, which every section in this corpus does, a wrapped bullet's continuation line falls to the prose branch and is DROPPED, because only the first paragraph per section survives. A per-point file makes the physical line easier to author; it does not make it safe to wrap
- [ ] `ai/rules/repo-maintenance.md` - the generator contract and the mapping table
  → Constraint: adding a generator is a five-edit pattern, and the fifth is a Go test asserting an exact set. Discovery updates are owed in the same work
- [ ] `ai/rules/spec-no-code.md` - no code snippets in this spec
  → Constraint: formats are described as tables, never as file listings

### RFC Summaries (Scope: protocol)

Not applicable. Scope is tooling. No protocol surface is touched.

**Key insights:** (minimal context to resume after compaction)

- The split must be a pure LINE PARTITION. Every byte of a rendered rule comes from exactly one point body or from the manifest header. Byte-identity then holds by construction, not by luck.
- `load_rules` in `scripts/dev/rules_condensed.py` is already lossy (it drops the H1 line, the metadata lines and the blank lines around them, and `splitlines()` discards line endings). The splitter must re-read the file. Reuse the shape of its `parse`, never its output.
- A splitter that classifies a line starting `|` as a table row without fence state destroys `ai/rules/planning.md`, which carries markdown tables INSIDE fenced blocks.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)

- [ ] `scripts/dev/rules_condensed.py` - `load_rules` returns one flat dict per rule (`name`, `stem`, `path`, `title`, `meta` as ordered pairs, `body` as lines, `trigger`, `severity`). `condense_body` filters by line SHAPE and assigns no identity to any obligation. `ARTIFACTS` is a two-entry tuple emitting `TRIGGERS.md` and `CORE.md`; `main` walks it, and `--check` compares text and exits 1 on mismatch
- [ ] `scripts/dev/rules_lint.py` - `CANON_KEYS` is closed at `When`, `Severity`, `Related`. `check_rule` validates the H1 title, the contiguous metadata block, key order and severity vocabulary. `check_trigger` validates the trigger string's grammar. Directive TEXT is never validated
- [ ] `scripts/dev/rule_coverage.py` - a THIRD consumer of `ai/rules/`, unrelated to the digests: it reads session transcripts and reports blocking rules whose trigger matched touched files but whose file was never Read. `list_rules` uses a non-recursive `glob("*.md")`, so a `points/` subdirectory is invisible to it. It has no make target
- [ ] `.claude/hooks/pretool-writeedit.py` - `CHECKS` is a module-level tuple of 44 function objects in dispatch order. There is no decorator and no registry: a `c_*` function absent from `CHECKS` never runs. `c_generated_files` matches `CLAUDE.md` and `AGENTS.md` by REALPATH against `PROJECT_DIR`, deliberately, so it cannot catch a same-named file in another checkout
- [ ] `scripts/status/verify_run_test.go` - `regenCheckPrereqs` and `generatorChecks` are asserted by `TestRegenCheckReadonlyCoversGenerators` as exact sets in both directions. A missing entry and an undocumented extra both fail
- [ ] `ai/rules/repo-maintenance.md` - "Sync Flows" has two rows. "Rule Placement" asserts `ai/rules/*.md` are originals edited directly, with two Exception bullets. "Banned Actions" has five rows. "Hook-to-Rule Mapping" has three sub-tables with columns Check, Enforces, Triggers on, What it does
- [ ] `ai/rules/rule-format.md` - 132 lines mandating title, metadata block, trigger grammar and a body budget for the single-file format
- [ ] `scripts/dev/rfc_requirements.py` - the model to copy: an authored id on the join key, everything else derived at check time, an unknown id reported as dangling, and a monotonic ratchet against a git HEAD baseline

**The grammar a byte-identical round trip must survive.** Measured across all 27 rules.

| Present, must be handled | Where the awkward cases live |
|--------------------------|------------------------------|
| Markdown tables inside fenced code blocks | `planning.md` |
| Fenced blocks in six languages, all balanced | 19 files |
| Backticked and backslash-escaped pipes in table cells | 22 and 11 files |
| HTML comments, both on their own line and trailing a content line | 5 and 10 files, including comments inside table rows |
| Blockquotes, including a bare marker line | `architecture.md`, `cli.md`, `repo-maintenance.md` |
| `####` headings | 6 files |
| List-continuation indented prose, and bullets nested under a numbered item | 7 files, and `writing.md` |
| Tabs, inside a fence | `config.md` |
| Non-ASCII: arrows, section marks, em dashes, emoji, comparison operators | 12 files |
| Task-list syntax, strikethrough inside a table cell, backslash line continuation inside a fence | 4 files, `platform-linux.md`, 2 files |
| A wrapped sentence whose line begins with an angle bracket | `git-safety.md` |

| Absent, need not be handled | Consequence |
|-----------------------------|-------------|
| Trailing whitespace, tabs outside fences, CRLF | no normalization step is needed |
| Consecutive blank lines, blank line at EOF, missing trailing newline | every block gap is exactly one blank line, so the renderer can join with one blank line and needs no recorded separator |
| Horizontal rules, setext headings, a second H1, `#####` or deeper | a triple-dash line is free to serve as a frontmatter delimiter |
| Asterisk and plus bullet markers, indented table rows, ragged cell counts, rows without a trailing pipe | the block parser needs no tolerance for malformed tables |

**Behavior to preserve:**

- `ai/rules/<rule>.md` stays byte-identical at every commit. Agents Read the same path and get the same bytes.
- `TRIGGERS.md` and `CORE.md` stay byte-identical. `rules_condensed.py` is not modified, so the always-loaded session payload cannot regress.
- `rules_lint.py` keeps passing over all 27 rendered rules, unchanged.
- `rules_router.py` and `rules_condensed.load_task_corpus` have a two-way dependency. Neither is touched.

**Behavior to change:**

- `ai/rules/<rule>.md` stops being canonical and becomes generated. The only observable difference to a reading agent is that an EDIT to it is refused.
- `ai/rules/repo-maintenance.md` stops claiming rule files are edited directly.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- An author edits one point file under `ai/rules/points/<rule>/`, or its manifest.
- Format at entry: a delimited metadata header followed by a verbatim block body.

### Transformation Path

1. `make ze-rules-render` reads each rule's manifest for the title, the metadata block and the ordered point list.
2. It reads each named point, strips the header, and concatenates the bodies with one blank line between them.
3. It writes `ai/rules/<rule>.md`.
4. `make ze-rules-condensed` parses those rendered files exactly as it does today and writes `TRIGGERS.md` and `CORE.md`.
5. `make ze-rules-gate-map` reads the binding comments in the three hook dispatchers, joins them against the point paths on disk, and reports three sets.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Points → rendered rule | one generator, `ze-rules-render` | No |
| Rendered rule → session payload | unchanged, `rules_condensed.py` | No |
| Hook check → point | a line comment naming a point path | No |

### Integration Points

- `Makefile` `ze-regen` and `ze-regen-check-readonly` - the render joins the regen set and its freshness gate.
- `scripts/status/verify_run_test.go` - `regenCheckPrereqs` and `generatorChecks` must gain entries or the exact-set assertion fails.
- `.claude/hooks/pretool-writeedit.py` - a new check refuses an edit to a rendered rule and names the point file.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every block gap in the corpus is exactly one blank line, so the renderer needs no recorded separator | Measured: consecutive blank lines absent across all 27 files | The renderer needs a `blank-after` field on each point and the format grows a field | `ze-rules-points-roundtrip` over all 27 | confirmed - re-measured 2026-08-07: zero consecutive blanks outside fences, zero blank line at EOF. A gap is therefore 0 or 1 blanks, and `block_ranges` makes a zero gap impossible by defining a block as a maximal non-blank run: 2471 runs, 46 of them heterogeneous, all kept whole. `split_rule` also raises when a gap is not exactly 1 |
| A-2 | A block-level partition with fence and indent state can express every rule file with no residue | The grammar table above; fences balanced, tables well formed | The split is lossy and the work does not land (D6) | `ze-rules-points-roundtrip` over all 27 | confirmed - all 27 round-trip byte-identical (2417 points). Fence state is load-bearing: the corpus carries 66 blank lines inside fences |
| A-3 | A triple-dash frontmatter delimiter is unambiguous, because horizontal rules are absent from the corpus | Measured: no horizontal rules in any of the 27 | A body opening with a triple-dash line truncates its own header | A splitter unit test on a body whose first line is a triple dash | confirmed - zero horizontal rules outside fences and zero `---` lines inside them. The delimiter is unambiguous by construction, not only by measurement: `_frontmatter` stops the header at the first `---` after line 1, which is always the one the writer emitted, so the body is never reached. `test_body_opening_with_delimiter` |
| A-4 | `ai/rules/points/` is invisible to every existing consumer of `ai/rules/`, because all three glob non-recursively | `list_rules` in `rule_coverage.py`, and the glob in `rules_condensed.py` and `rules_index.py` | A phantom 28th rule appears in `INDEX.md`, `TRIGGERS.md` or the coverage report, exactly as `CONDENSED.md` once did | `make ze-doc-test` plus a count assertion after the split lands | **broken** - the named consequence did NOT happen (27 rules in `INDEX.md`, 27 rows in `TRIGGERS.md`, 8 in `CORE.md`, all three byte-identical after the flip), because the three named globs really are non-recursive. The assumption is wrong in its SCOPE: "every existing consumer" is false. Four consumers reach `ai/rules/points/` by a recursive walk and one by a prefix test, all fixed in phase 3. `default_files` in `ste_check.py` globs `ai/**/*.md`, so it reviewed each point body a SECOND time: 951 of the 2417 lines its ratchet printed were duplicates of findings the rendered rule already reports (fixed: `ai/rules/points/` added to `EXCLUDE_DIRS`). `targets` in `line_refs.py` `rglob`s `ai/`, so `--apply` would have rewritten a GENERATED rule (fixed: every `*.md` directly in `ai/rules/` is skipped, so the sweep reaches the point instead). `read_transcript` in `rule_coverage.py` credited `os.path.basename` for any read under `ai/rules/`, and four real slugs equal a rule stem (`architecture`, `completion`, `git-safety`, `testing`), so opening one point falsely cleared a blocking rule (fixed: `_is_rule_path`, plus two tests). `c_line_number_ref` and `c_enforce_naming` in `.claude/hooks/pretool-writeedit.py` both reach points: measured zero strippable line refs in the corpus and every slug is lowercase kebab by `SLUG_SAFE`, so neither fires today. `lesson_worthy` in `commit_helper.py` treats the `ai/rules/` prefix as lesson-worthy, which is correct and already what closure owes |
| A-5 | Nothing outside the three dispatchers needs to name a point | The handover scopes STEP 4 to the 44 checks | The binding comment convention needs a second reader | Grep for other enforcement sites during STEP 4 | **broken** - two points are enforced at BOTH ends and only the hook end names one. `_model_refusal` in `scripts/dev/review_gate.py` refuses to record a review off Opus 5. `review_model_refusal` in `.claude/hooks/pretool-agent-skill.py` refuses to spawn one. `planning/review-still-runs-on-opus-5-and-that-half-is-unchanged` names both, and only the hook names it back. `testing/blocking-gate-check-ci-sleep-justification-in` is the same shape over `check_ci_sleep_justification` in `scripts/dev/verify_wiring_docs.py` and `c_ci_sleep_justification` in the hook. Two more families want a binding. `scripts/dev/commit_helper.py` (`lesson_worthy`, `structural_gate_reds`, `commit_gate_problems`, `review_gate_problems`) cites a rule FILE in a comment for each. The twelve `check_*` ratchets in `scripts/dev/rfc_requirements.py`, `check_enrolment` to `check_gap_count_agreement`, enforce a table that is itself points of `rfc-compliance.md`. `.claude/hooks/block-premature-stop.sh` enforces `completion.md` and `.claude/hooks/validate-spec.sh` enforces `planning.md`. Scope is NOT expanded. Known Limitations already scopes phase 4 to the dispatcher checks. `parse_bindings` reads whatever file list it is given, so a later spec adds readers, never a second convention |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The split loses content | `ze-rules-points-roundtrip` diff is not empty | Phase 2 is byte-identical or the work does not land. This is the go/no-go |
| R-2 | Roughly 1,469 files makes a rule harder to read at source | An author says they cannot find an instruction | The rendered file still exists and is still what agents Read. Only authors touch points |
| R-3 | Concurrent sessions editing rules | Merge conflicts | This IMPROVES on today. One writer per file is the property that made `plan/deferrals/` sharded; today any two rule edits collide in one file |
| R-4 | A point present on disk but absent from its manifest is silently dropped | A rendered rule loses an instruction with no gate going red | The renderer FAILS on any unlisted point and on any listed slug with no file. This is the cost of choosing a manifest over a numeric prefix and it must be paid in Phase 1 |
| R-5 | Most points carry no RFC 2119 level, so `level` measures little | The field is empty on most points after the split | Report the count. Classifying the corpus is separate work and is out of scope here |
| R-6 | STEP 5 makes part of `repo-maintenance.md` generated, and that file is itself a rule rendered from points | A generated point inside a rendered rule, written by a third generator | **resolved, and no deferral is owed. The table is AUTHORED, and a derived CHECK holds it to the bindings.** `hook_table_problems` in `scripts/dev/rules_points.py` runs inside `make ze-rules-gate-map`. `make ze-doc-test` therefore goes red when the `Check` or `Enforces` column disagrees with a `# ze point:` comment. It found 103 disagreements on first run. Generation was rejected on three grounds. First, a row is not separable: `Triggers on` and `What it does` are authored prose on the same row. A generator would have to rewrite two cells INSIDE an authored markdown row that carries escaped pipes and trailing HTML comments. Second, no other tool here reads inside a point body, and the byte-identical round trip (R-1, D6) rests on bodies staying verbatim. Third, a generator can fill only one short column, because a new check's prose is authored either way. Its value over a check is typing, not correctness, and both designs catch the same drift. Moving the table to `docs/` was rejected too. The trigger of `repo-maintenance.md` is "looking up which check enforces a rule", so the answer belongs in the rule, and the move relocates the merge problem rather than solving it |
| R-7 | The new generator trips the exact-set assertion in the Go test | `TestRegenCheckReadonlyCoversGenerators` reports an undocumented prerequisite | Add the `regenCheckPrereqs` and `generatorChecks` entries in the same phase as the make targets, never after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and nothing in the daemon. The failure mode is an agent-facing rule losing an instruction silently, which is why byte-identity is the gate rather than a review |
| How is it reverted? | Single commit revert per phase. Until Phase 3 the rendered rules stay canonical and the points are additive, so Phases 1 and 2 are revertible with no consequence at all |
| Who else touches this path? | Any session editing a rule. `RESUME-wire-edit-4-bird-interop.md` is untracked in this checkout and unrelated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rules-points-roundtrip` | → | `split` then `render` in `scripts/dev/rules_points.py` | `test_roundtrip_every_committed_rule` |
| `make ze-rules-render` | → | `render` writing `ai/rules/<rule>.md` | `test_render_matches_committed_rule` |
| `make ze-rules-render-check` | → | `render` in compare mode | `TestRegenCheckReadonlyCoversGenerators` |
| An Edit to `ai/rules/performance.md` | → | the new generated-rule check in `.claude/hooks/pretool-writeedit.py` | `hook-fixture-check.py` fixture `rendered-rule-edit-refused` |
| `make ze-rules-gate-map` | → | `coverage` in `scripts/dev/rules_points.py` | `test_gate_map_sets_and_exits` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `split` runs over each of the 27 committed rules into a scratch directory | Every line of the source file lands in exactly one point body or in the manifest header. No line is duplicated and none is dropped |
| AC-2 | `render` runs over the output of AC-1 | The result is byte-identical to the committed rule, for all 27. Any difference fails the gate |
| AC-3 | A point file exists on disk but is absent from its manifest | `render` exits non-zero and names the unlisted file. It never renders a partial rule |
| AC-4 | A manifest lists a slug with no file on disk, or lists the same slug twice | `render` exits non-zero and names the slug |
| AC-5 | A point body's first line is a triple dash | The header parse is unaffected and the body round-trips unchanged |
| AC-6 | The points are committed and `ai/rules/<rule>.md` is generated | `make ze-doc-test`, `make ze-rules-lint` and `make ze-rules-condensed --check` all pass unchanged, and `TRIGGERS.md` and `CORE.md` are byte-identical to their pre-split content |
| AC-7 | An agent issues an Edit or Write against `ai/rules/<rule>.md` | The hook refuses with exit 2 and names the point directory to edit instead |
| AC-8 | An agent issues an Edit against a file under `ai/rules/points/` | The hook permits it |
| AC-9 | `ze-rules-render` is added to `ze-regen` and `ze-rules-render-check` to `ze-regen-check-readonly` | `TestRegenCheckReadonlyCoversGenerators` passes, with entries in both `regenCheckPrereqs` and `generatorChecks` |
| AC-10 | A check in a hook dispatcher carries a binding comment naming a point that exists | `ze-rules-gate-map` lists it in the gated set and exits 0 |
| AC-11 | A check names a point that does not exist | `ze-rules-gate-map` lists it in the dangling set and exits non-zero |
| AC-12 | Points carry no binding at all | `ze-rules-gate-map` lists them in the ungated set and exits 0. The ungated count is a measurement and never a red |
| AC-13 | `ai/rules/points/` exists with roughly 1,469 files | `make ze-rules-index` still reports 27 rules, and `TRIGGERS.md` still carries 27 rows. No phantom rule appears |
| AC-14 | The corpus has been split and rendered | `make ze-rules-router-report` reports the same rule count and the same always-on core membership as before the split |

### Follow-on scope, added 2026-08-07 on owner instruction

<!-- The split gave every instruction an id. Nothing links an instruction to WHY
     it exists. Measured 2026-08-07: of 934 learned summaries, 426 are cited
     outside `plan/learned/` and outside the two generated indexes. Only 13 are
     cited by a rule or a point, while `.claude/` cites 1,247 and `internal/`
     1,180. The learned corpus is already the rationale layer for hooks and code,
     and is linked to rules barely at all. `parse_point` rejects any field outside
     `POINT_KEYS`, so a fourth key is a format change, not free text. -->

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-15 | A point declares an optional `rationale` frontmatter key naming a repo-relative path | It parses. A point without the key still parses, as `stage` does today. The rendered rule is byte-identical either way: rationale is a header field and never reaches the body |
| AC-16 | A point's `rationale` names a path absent from disk | `ze-rules-gate-map` reports it in its own failing set and exits non-zero, for the same reason a dangling binding does: it is the signal that the explanation moved out from under the instruction |
| AC-17 | Points carry rationale links | The gate map reports how many instruction points have one and exits 0 whatever the number. Coverage is a measurement, never a red, exactly as the ungated count is (handover D5) |
| AC-18 | The learned-staleness ceiling gains a drain policy | `learned_staleness.py` reads a start date and a rate, ships INERT at rate 0, and only the owner arms it. The mechanism mirrors `rfc/drain-budget.txt`, which carries policy only and may never gain a per-item row |
| AC-19 | A future session proposes retiring the staleness gate to end the tax | A directive point refuses it and names the alternative, and a learned summary carries the measurement, the rejected alternatives, and why citation count is the wrong relevance metric |
| AC-20 | A point declares an optional `excepted-by` frontmatter key naming one or more point ids, comma-separated | It parses, and the key is declared on the GENERAL point and names the EXCEPTION. A point without the key still parses, as `rationale` does. The rendered rule is byte-identical either way. A ref naming no point on disk, or naming the declaring point itself, fails `ze-rules-gate-map`, so deleting an exception point turns the general point's link dangling and the gate goes red |
| AC-21 | Points carry exception links | The gate map reports how many instruction points name an exception, and how many points are named, and exits 0 whatever the numbers. Coverage is a measurement, never a red, exactly as the rationale and ungated counts are |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_split_partitions_every_line` | `scripts/dev/rules_points_test.py` | AC-1, over all 27 real rules by summation over line indices | pass |
| `test_roundtrip_every_committed_rule` | `scripts/dev/rules_points_test.py` | AC-2 over all 27 real rules, not a fixture | pass |
| `test_roundtrip_covers_the_whole_corpus` | `scripts/dev/rules_points_test.py` | the corpus is 27; a narrowed corpus is not a pass | pass |
| `test_fenced_table_is_one_point` | `scripts/dev/rules_points_test.py` | a table inside a fence is not split as a table | pass |
| `test_nested_bullet_stays_with_parent` | `scripts/dev/rules_points_test.py` | list-continuation prose and nested bullets do not orphan | pass |
| `test_render_fails_on_unlisted_point` | `scripts/dev/rules_points_test.py` | AC-3 | pass |
| `test_render_fails_on_missing_and_duplicate_slug` | `scripts/dev/rules_points_test.py` | AC-4 | pass |
| `test_render_rejects_unsafe_slug` | `scripts/dev/rules_points_test.py` | Security Review: a manifest cannot escape its rule directory | pass |
| `test_body_opening_with_delimiter` | `scripts/dev/rules_points_test.py` | AC-5 | pass |
| `test_point_file_on_disk_opening_with_delimiter` | `scripts/dev/rules_points_test.py` | AC-5 through the filesystem, not only in memory | pass |
| `test_manifest_carries_title_and_metadata` | `scripts/dev/rules_points_test.py` | the manifest holds the spine `rules_lint` validates | pass |
| `test_every_point_declares_a_known_kind` | `scripts/dev/rules_points_test.py` | kind, level and slug vocabularies hold over all 27 | pass |
| `test_gate_map_sets_and_exits` | `scripts/dev/rules_points_test.py` | AC-10, AC-11, AC-12 | pass |
| `test_gate_map_reports_a_binding_that_gates_nothing` | `scripts/dev/rules_points_test.py` | a binding separated from its check by code is dangling, not dropped | pass |
| `test_gate_map_declared_none_needs_a_reason` | `scripts/dev/rules_points_test.py` | `none -- <why>` is unbound; a bare `none` and an empty payload are both dangling | pass |
| `test_gate_map_empty_result_is_never_success` | `scripts/dev/rules_points_test.py` | no points, or no bindings, fails rather than reporting a vacuous green | pass |
| `test_gate_map_over_the_real_dispatchers` | `scripts/dev/rules_points_test.py` | the committed tree has zero dangling, and every check names a point or declares `none` | pass |
| `test_hook_table_agrees_with_the_bindings` | `scripts/dev/rules_points_test.py` | a row per check, and an `Enforces` cell naming what the binding names | pass |
| `test_hook_table_flags_a_row_naming_no_check` | `scripts/dev/rules_points_test.py` | a row for a deleted function fails, and its check is then also reported rowless | pass |
| `test_hook_table_flags_a_check_with_no_row` | `scripts/dev/rules_points_test.py` | a check nobody documented must not pass by being absent | pass |
| `test_hook_table_flags_a_drifted_enforces_cell` | `scripts/dev/rules_points_test.py` | a cell naming the wrong rule fails. A `none` check can claim no rule. A rule outside the corpus is free text | pass |
| `test_hook_table_missing_or_empty_is_never_success` | `scripts/dev/rules_points_test.py` | no table, a table under no naming heading, and a duplicate row all fail | pass |
| `test_hook_table_over_the_real_tree` | `scripts/dev/rules_points_test.py` | the committed mapping table agrees with every committed binding | pass |
| `TestRegenCheckReadonlyCoversGenerators` | `scripts/status/verify_run_test.go` | AC-9, already exists, must stay green | pass |
| `VerifyPartitionTest` (3) | `scripts/dev/rules_points_test.py` | review N1: `_verify_partition` raises on an overlap and on a hole | pass |
| `RenderAllTest` (6) | `scripts/dev/rules_points_test.py` | review 3: `--check` never writes, names every drift, and refuses an empty tree, a rule with no points, and a `points/CORE/` directory | pass |
| `DispatcherRosterTest` (8) | `scripts/dev/rules_points_test.py` | review 4: the roster is derived from `.claude/settings.json` and pinned at three; a shrunk roster, a missing file and an unprefixed unbound check all fail | pass |
| `GatedRatchetTest` (4) | `scripts/dev/rules_points_test.py` | review 6: a point that lost every binding since HEAD fails; a deleted point does not; no baseline says so | pass |
| `point-overwrite-*` (6 fixtures) | `.claude/hooks/fixtures/` via `hook-fixture-check.py` | review 1: a `Write` over an existing point is refused, and a free slug, an `Edit` and another checkout are not | pass |
| `RationaleTest` (7) | `scripts/dev/rules_points_test.py` | AC-15, AC-16, AC-17 | pass |
| `ExceptedByTest` (10) | `scripts/dev/rules_points_test.py` | AC-20, AC-21, and the mutation: deleting an exception point reds the gate | pass |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| points per rule | 1 to unbounded | n/a | 0 points with a non-empty body | n/a |
| blank lines between blocks | exactly 1 | 1 | 0 | 2 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rendered-rule-edit-refused` | `.claude/hooks/fixtures/` | an agent tries to edit `ai/rules/performance.md` and is told to edit the point instead | |
| `point-file-edit-allowed` | `.claude/hooks/fixtures/` | an agent edits a point file and is not blocked | |

### Interop Tests (Scope: protocol)

Not applicable. No wire-visible behavior and no peer daemon.

## Files to Modify

- `Makefile` - `ze-regen` and `ze-regen-check-readonly` prerequisites, and the `ze-regen-check` diff list
- `mk/inventory.mk` - the new targets and the `.PHONY` line, and a `ze-doc-test` line
- `scripts/status/verify_run_test.go` - `regenCheckPrereqs` and `generatorChecks` entries. Also correct the stale `ze-rules-condensed-check` comment, which still names the `CONDENSED.md` deleted on 2026-08-03
- `.claude/hooks/pretool-writeedit.py` - a new check in `CHECKS`, plus binding comments on the 44 existing checks
- `.claude/hooks/pretool-bash.py`, `.claude/hooks/pretool-agent-skill.py` - binding comments
- `ai/rules/rule-format.md` - rewritten for the point format
- `ai/rules/repo-maintenance.md` - Sync Flows row, the Rule Placement bullet and its exceptions, a Banned Actions row, and the Hook-to-Rule Mapping note
- `ai/INDEX.md` - Dev Tools rows for the four new targets
- `docs/contributing/README.md` - the index row for `rule-authoring.md`

## Files to Create

- `scripts/dev/rules_points.py` - `split`, `render` and `coverage` subcommands
- `scripts/dev/rules_points_test.py` - the unit tests above
- `ai/rules/points/<rule>/` - one manifest and one file per point, for all 27 rules
- `docs/contributing/rule-authoring.md` - the author-facing page. Created: the layout, the five authoring tasks, the frontmatter fields, what the renderer refuses, the binding convention, the generator order, and the digest history moved out of `rule-format.md`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface. Tooling only |
| YANG validation constraints | No | As above |
| YANG custom validators | No | As above |
| CLI commands/flags | No | Make targets, not `ze` subcommands |
| CLI grammar (keyword before value) | No | No `ze` command added |
| Editor autocomplete | No | No YANG leaf added |
| Functional test for new RPC/API | No | No RPC. Hook fixtures cover the user-facing refusal |
| Pipe completeness | No | No `ze` command output |
| Env var registration | No | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No new runtime dependency. Python 3 and make are already required |
| Prometheus counters/metrics | No | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | No | No protocol surface |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent-facing tooling, not a Ze feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/contributing/rule-authoring.md` for rule authors |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | No test runner change |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | No `internal/` change |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` Dev Tools rows for the three new targets |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `rules_condensed.py` and `rule-format.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/rule-format.md` carries a skeleton rule that becomes wrong |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the round-trip harness before any real split
   - Tests: `test_roundtrip_every_committed_rule`, failing because `rules_points.py` does not exist
   - Files: `scripts/dev/rules_points.py`, `scripts/dev/rules_points_test.py`, `mk/inventory.mk` target `ze-rules-points-roundtrip`
   - Verify: the target exists, runs, and reports 27 failures rather than passing vacuously
2. **Phase: Splitter and renderer (THE GO/NO-GO)** -- block parser with fence and indent state, manifest, render
   - Tests: every unit test in the plan above
   - Files: `scripts/dev/rules_points.py`, `scripts/dev/rules_points_test.py`
   - Verify: all 27 round-trip byte-identical. **If they do not, STOP and report. Do not accept a lossy split, and do not narrow the corpus to the files that pass**
3. **Phase: Flip canonical** -- commit the points, generate the rules
   - Tests: `TestRegenCheckReadonlyCoversGenerators`, the two hook fixtures, `make ze-doc-test`
   - Files: `ai/rules/points/**`, `Makefile`, `mk/inventory.mk`, `scripts/status/verify_run_test.go`, `.claude/hooks/pretool-writeedit.py`, `ai/rules/repo-maintenance.md`
   - Verify: AC-6, AC-7, AC-8, AC-9, AC-13, AC-14. `TRIGGERS.md` and `CORE.md` unchanged byte for byte
4. **Phase: Bind the gates** -- one comment per check, and the join
   - Tests: `test_gate_map_sets_and_exits`
   - Files: the three dispatchers, `scripts/dev/rules_points.py`, `mk/inventory.mk`
   - Verify: AC-10, AC-11, AC-12. Seed from the `Enforces` column, which already answers this at rule granularity
5. **Phase: Generate the mapping table, and the discovery updates**
   - Tests: the render-check for the generated section
   - Files: `ai/rules/repo-maintenance.md`, `ai/rules/rule-format.md`, `ai/INDEX.md`, `docs/contributing/rule-authoring.md`
   - Verify: R-6 resolved. The table is authored and `hook_table_problems` holds it to the bindings inside `make ze-rules-gate-map`. No deferral is owed

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Vacuity | `test_roundtrip_every_committed_rule` must FAIL when one line is deleted from one point body. Prove it by deleting one and running it |
| Correctness | The partition is total: assert line counts sum, not just that render matches. A splitter that drops a line and a renderer that re-adds it would pass a diff-only test |
| Fence state | `planning.md` renders unchanged, tables inside fences intact |
| Fail closed | The renderer errors on an unlisted point rather than skipping it (`ai/rules/evidence.md`) |
| Data flow | `rules_condensed.py` is not modified. Confirm by diff, not by intention |
| Rule: `repo-maintenance.md` | All five generator edits made, including the Go-side exact-set entries |
| Rule: `evidence.md` | The gate-map dangling set exits non-zero. An empty result is never reported as success |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Splitter and renderer | `make ze-rules-points-roundtrip` exits 0 over all 27 |
| Points committed | `ls ai/rules/points/ | wc -l` reports 27 directories |
| Rules generated | `make ze-rules-render && git diff --quiet ai/rules/` |
| Payload unchanged | `git diff --quiet ai/rules/TRIGGERS.md ai/rules/CORE.md` after a full regen |
| Edit refusal | `python3 scripts/dev/hook-fixture-check.py --only rendered-rule` |
| Gate map | `make ze-rules-gate-map` prints three sets and exits 0 with no dangling |
| Corpus unchanged | `make ze-rules-router-report` reports 27 rules and the same core membership |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A point slug is a path component. Reject any slug containing a path separator, a leading dot, or a parent reference, so a manifest cannot make the renderer read outside its rule directory |
| Fail open | The new hook check must refuse when it cannot resolve the path, never permit. `c_generated_files` documents the same care and its reason |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Round trip not byte-identical | STOP. Report the file and the diff. Do not narrow the corpus |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `load_rules` looked like the splitter's front end and is not. It is lossy by design, because a digest generator has no reason to keep the header or the blank lines. The splitter shares its SHAPE and none of its output.
- Choosing a manifest over a numeric prefix moved a cost rather than removing one. The prefix costs renumber churn that breaks ids; the manifest costs a silent-drop failure mode. The second is closable by a hard error and the first is not, which is why the manifest wins.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reading order held by a per-rule manifest | `NNN-` numeric prefix in tens, as the handover recommended | D2 makes the path the id and STEP 4 binds gates to it. A numeric prefix couples the id to reading order, so every reorder rewrites ids and breaks every binding. The manifest decouples them, and its silent-drop weakness closes with a hard render error (R-4) |
| Connective prose is a `note` point | A per-rule `_intro.md` | Prose sits BETWEEN directives, not only above them. An intro file cannot express a paragraph at position 7 |
| Five point kinds, not the handover's three | `directive`, `table`, `note` only | Headings and fenced blocks are structural. Without their own kinds they inflate the ungated denominator that D5 exists to measure |
| `rules_condensed.py` is not modified | Rewrite it to read points directly | Today's output is identical either way, so the rewrite buys nothing now. Leaving it alone keeps the payload producer out of a change that alters the format, and gives a free second check on the round trip: if a payload digest moves, the render is lossy |
| The split is a line partition, tested by summation | Diff the render against the source | A splitter that drops a line and a renderer that re-adds it passes a diff. Asserting the partition is total catches that class |
| Target named `ze-rules-gate-map` | `ze-rules-coverage`, as the handover proposed | `scripts/dev/rule_coverage.py` already exists and measures something different. `ze-rules-coverage` would read as its target |
| Triple-dash frontmatter delimiter | A bold marker block, matching the rule metadata convention | The body must be recoverable byte-exactly, which needs an unambiguous terminator. Horizontal rules are absent from the corpus, so the delimiter cannot collide (A-3) |

## Known Limitations

- `stage` is authored but left empty throughout this work. It is what later lets a design-phase subagent skip implementation directives, and populating it is separate work.
- `level` will be empty on most points. The corpus spends 154 MUST, 20 MUST NOT and 4 SHOULD over 11,015 lines. Classifying the rest is separate work (R-5).
- STEP 4 narrows the `Enforces` answer from file to point for the 55 dispatcher checks only (47 bound, 8 declaring `none`). Enforcement that lives elsewhere (`commit_helper.py` gates, `review_gate.py`, the RFC ratchets) is not bound in this work.
- The Hook-to-Rule Mapping table's `Check` and `Enforces` columns are checked against the bindings, not generated from them. `Triggers on` and `What it does` stay authored and are checked by nothing, so a row's PROSE can still go stale while its two derived columns cannot.
- The published-table check covers the three PreToolUse dispatchers. The PostToolUse table, the `ze-verify-wiring-docs` table, the prose-gate table and the commit-time gate table in the same section are outside it, because their checks carry no binding comment.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rules-as-points-7545230e-2aee-4587-9c64-0412270cf78c.md`, and the same review under a second key, `tmp/review/learned-corpus-drain-over-archive-...md` |
| `review_gate.py check` | clean under both keys (`review_gate: OK (0 code files, clean, hashes match ...)`, exit 0) |
| Why two keys | `spec_closure_stem` in `scripts/dev/commit_helper.py` reads `add_paths` first and derives the stem from the learned summary's FILENAME, not from the removed spec. This spec's summary is `plan/learned/1356-learned-corpus-drain-over-archive.md`, and it cannot be renamed to the spec stem: a committed rule point names it in a `rationale:` key, so a rename would edit `ai/rules/` and re-render `ai/rules/planning.md` inside a closure commit. The second artifact carries the SAME verdict over the SAME nine files, and its hash block is byte-identical to the first. No file was re-reviewed |
| Reviewer lenses used | round 1: correctness/wiring, guards/security, tests/removed-behaviour. round 2: the same three over everything round 1 produced. round 3: one lens scoped to `7f03b3dfa` |
| Rounds | 3. Round 1: 1 BLOCKER, 7 ISSUE, 6 NOTE. Round 2: 4 BLOCKER, 3 ISSUE. Round 3: 0 BLOCKER, 2 ISSUE. Final state 0 BLOCKER, 0 ISSUE |
| Independence | the reviewers were subagents, none of them the author. The main thread re-probed every BLOCKER and both round-3 mutations against the real tree before it accepted the round |

Round 1's findings are recorded in full below. Round 2's four BLOCKER and three
ISSUE were fixed in `7f03b3dfa` ("close four gates that were green because they
could not fire"), and round 3's two ISSUE in `513b427c1` ("make the retirement
ledger checked, and let it free a check"). Each commit message carries the
finding it answers, so the fix and its reason stay together in `git log`.

### Round 1

| Field | Value |
|-------|-------|
| Scope | the WHOLE diff of phases 1 to 5 |
| Reviewers | three, independent of the author |
| Lenses | (1) the hook dispatchers and the edit-time refusals, (2) `scripts/dev/rules_points.py` and its tests, (3) the make wiring, the published claims and the docs |
| Verified | the main thread checked every finding against source before this round started. All eight are real, and none was re-litigated |
| Result | 1 BLOCKER, 7 ISSUE, 6 NOTE. All fixed in this round |

| # | Severity | Finding | Fix |
|---|----------|---------|-----|
| 1 | BLOCKER | `c_rendered_rules` returns `None` for every path whose dirname is not `ai/rules`, so a `Write` over an EXISTING point was permitted and its instruction was gone at write time. `write_split` refuses the same move and cites `ai/rules/never-destroy-work.md`. Phase 5 clobbered `check-enforces-triggers-on-what-it-does-4.md` and recovered it only from git | New SIBLING check `c_point_overwrite` in `.claude/hooks/pretool-writeedit.py`, registered in `CHECKS`, bound to `never-destroy-work/operation-scope-replacement`. A sibling rather than a branch: `c_rendered_rules` answers "is this GENERATED", and its dirname early return is what makes AC-8 true, so the opposite question needs its own function. `Edit` and `MultiEdit` stay permitted, and a `Write` to a free slug stays permitted |
| 2 | ISSUE | `ze-rules-points-roundtrip` appeared only in `.PHONY`, its own recipe and PROSE. Three documents claimed `ze-doc-test` ran it | Wired into the `ze-doc-test` recipe in `mk/inventory.mk`. The three claims are now true |
| 3 | ISSUE | `render_all` is the producer behind four gates and no test named it | `RenderAllTest`, six cases, in `scripts/dev/rules_points_test.py`. `--check` must never write and must name every drifted rule |
| 4 | ISSUE | `GATE_FILES` was a hand-typed 3-tuple with no pin, and `dispatcher_checks` read a `c_`/`check_` name prefix that two live gates do not carry | `dispatchers()` derives the roster from the PreToolUse entries in `.claude/settings.json` and cross-checks it against `.claude/hooks/pretool-*.py` on disk, so a shrunk roster and a fourth dispatcher both go red. `dispatcher_checks` now reads the module with `ast`: the `CHECKS` tuple where one exists, otherwise the top-level functions `main()` calls |
| 5 | ISSUE | `make -j ze-regen` could build the digests from pre-render text. GNU make honours prerequisite ORDER only serially | `ze-rules-condensed` and `ze-rules-index` now declare `ze-rules-render` as a PREREQUISITE. An order-only prerequisite would not have worked, because it orders a prerequisite against its own target and never two siblings of one target. `.NOTPARALLEL` would serialise every unrelated target in the file |
| 6 | ISSUE | The gated set had no ratchet. Deleting a binding AND the backticked stem from its published row left both sides agreeing on empty, and a point moved from gated to ungated with everything green | `gated_at_head` plus `gated_regressions` in `scripts/dev/rules_points.py`, following `check_coverage_ratchet` in `scripts/dev/rfc_requirements.py`. `report_gate_map` fails on the REGRESSED set. The UNGATED count stays exit 0, which is D5 |
| 7 | ISSUE | Two documents claimed a machine answers all three problems in the Task table. `gate_map` joins on `Binding.ref` alone and `Binding` carries no digest of the body, so a reword keeps its gate | `docs/contributing/rule-authoring.md` and `ai/rules/points/rule-format/one-instruction-one-file.md` now claim the first two and say the third is answered in REVIEW. No content hash was added: D2 rejected it and that decision stands. A body digest on the BINDING is a row in `plan/deferrals/rules-as-points.md` |
| 8 | ISSUE | `render_split` had zero callers repo-wide | Deleted |
| N1 | NOTE | `_verify_partition` was proven by nothing | `VerifyPartitionTest`: an overlap, a hole, and a total partition as the control |
| N2 | NOTE | `run_rendered_rule` called `mod.c_rendered_rules` directly while every other section used `subprocess` | Converted. Every behavioural assertion now runs the whole dispatcher through `_writeedit` |
| N3 | NOTE | `render_all` took its targets from `point_dirs()` with no name predicate, so `points/CORE/` would render OVER `ai/rules/CORE.md` | `render_all` now applies `rule_files`' own predicate on the write side, with a test |
| N4 | NOTE | `c_rendered_rules` took `base` from the raw path and decided from the resolved one | `base` now comes from `resolved`, so a symlinked refusal names the file to edit |
| N5 | NOTE | `c_rendered_rules` listed `NotebookEdit`, which `main()` can never deliver: it sets `fp` from `file_path` and NotebookEdit sends `notebook_path` | Dropped from the tuple, with the reason recorded in the docstring |
| N6 | NOTE | The comment above `DOC_RULE` claimed the columns cannot disagree with the bindings, one level stronger than the code | Corrected in both places: the `Enforces` comparison is at RULE granularity, so rebinding within one rule is invisible to it |

Every fix carries a mutation proof: the new test fails when the fix is reverted, and passes when it is restored. Findings 2, 3, 4 and 6 are gates that were not themselves gated, so the mutation proof is the deliverable rather than a supporting detail.

### Round 2

| Field | Value |
|-------|-------|
| Scope | everything produced after round 1, over `299334063` |
| Reviewers | three, independent of the author |
| Result | 4 BLOCKER, 3 ISSUE. All fixed in `7f03b3dfa` |
| Verified | the main thread re-probed every BLOCKER against the real tree before it accepted the round |

#### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R2-1 | BLOCKER | `c_point_overwrite` and `c_rendered_rules` compared realpath STRINGS. `realpath` resolves symlinks but not case, so on this case-insensitive filesystem a Write to `AI/rules/points/...` exited 0 and landed on the real file | `.claude/hooks/pretool-writeedit.py` | Identity asked of the filesystem through `os.path.samestat`, and each tail component's on-disk spelling read from the directory listing so a case-sensitive volume is not falsely blocked. Nine fixtures, one per varied segment. `normcase` is the identity on POSIX and is not the fix |
| R2-2 | BLOCKER | `render_dir` globbed `*.md` non-recursively inside a section and `points_on_disk` walked exactly `rule/section/point.md`, so a point one level too deep rendered nothing while render-check, roundtrip and the gate map all exited 0. `make ze-regen` runs render in write mode, so the instruction would have been deleted | `scripts/dev/rules_points.py` | Both walks made total, with a test. This is R-4 realized, immediately after six agents moved 1580 files |
| R2-3 | BLOCKER | The section mapping was not total. `planning.md` carried 27 fence-aware `##` headings against 26 section directories: `## Work Phases` sat inside a point BODY because no blank line preceded it, so every point beneath it carried an id naming a section the reader never sees | `scripts/dev/rules_points.py` | `block_ranges` breaks on a fence-aware `##` mid-block and `Section.tight` records the absent blank line, so the render stays byte-identical. `render_dir` re-derives the `##` set from the RENDERED bytes and refuses any heading the manifest does not name. 281/280 before, 281/281 after |
| R2-4 | BLOCKER | `head_sources` returned `{}` both when git answered for no file and when a dispatcher was renamed, and the caller read "not None" as "there is a baseline", printing `REGRESSED: 0` from a baseline holding nothing | `scripts/dev/rules_points.py` | HEAD probed once with `rev-parse --verify`, `None` means git could not answer, and a dispatcher absent at HEAD is named in the report rather than absorbed |
| R2-5 | ISSUE | Deleting a point and its manifest line together was invisible to every gate: points and rendered rule agree on the smaller corpus, so nothing goes red | `scripts/dev/rules_points.py` | Coverage compares each rule's point count against `git ls-tree HEAD`, and a drop must be declared in `ai/rules/points/RETIRED.md` |
| R2-6 | ISSUE | A rename whose dangling binding was relabelled `none -- <why>` laundered a lost gate into the UNBOUND set | `scripts/dev/rules_points.py` | A check that named a point at HEAD and declares `none` now is a regression, reported as DECLARED NONE |
| R2-7 | ISSUE | `RETIRED.md` prose did not say its rows were checked | `ai/rules/points/RETIRED.md` | Corrected in round 3, with the ledger |

### Round 3

| Field | Value |
|-------|-------|
| Scope | `7f03b3dfa` only, the code round 2 produced |
| Reviewers | one, independent of the author |
| Result | 0 BLOCKER, 2 ISSUE. Both fixed in `513b427c1` |
| Verified | the main thread re-ran both mutations the reviewer wrote against the real tree |

#### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R3-1 | ISSUE | `unbound_regressions` refused the intended transition. Retire an instruction, declare it, relabel the check, and the gate failed: the function read only the gate map and the baseline and never consulted `retired_since_head`, which landed in the same commit for that purpose | `scripts/dev/rules_points.py` | A declared retirement is exempt, and retiring part of what a check named still reports the live points. Mutation re-run: a declared retirement whose check declares `none` exits 0 |
| R3-2 | ISSUE | `retired_since_head` believed a lie. It read only the rule name out of each row, so declaring a fictional id bought a real deletion elsewhere in the same rule, and rewording a committed row minted another | `scripts/dev/rules_points.py` | `retired_rows_since` is now pure and validated: it refuses a malformed row, an id HEAD never carried, an id whose point is still on disk, and a duplicate declaration. Mutation re-run: deleting a real point while declaring `rule/nowhere/never-existed` exits 2 and names both the bogus row and the vanished point |

`report_gate_map` also prints the dispatchers absent at HEAD, which the round-2
fix collected and then dropped on the no-baseline branch.

### Follow-on: the 47 bound slugs, re-authored (owner-authorised, 2026-08-07)

`slugify` in `scripts/dev/rules_points.py` produced every point id in one pass
from the block's first line, so an id names how the sentence OPENED rather than
what the instruction obliges. Two ids came from a table header row
(`architecture/anti-pattern-instead`, `architecture/principle-rule`) and one
truncated at the word that carried the meaning
(`architecture/persist-runtime-state-through-the-managed-zefs-store-never`).

Scope is the 47 points a hook check binds, and nothing else. A hook comment is
the one place a human reads a slug RAW, with no tool rendering anything nicer.
The other 1541 instruction points reach a reader only through generated output,
so re-authoring their ids would churn 800 filenames and every `git log` trail for
no reader. They get an authored `title:` field instead, homed in
`plan/deferrals/rules-as-points.md`.

Each rename touched three places: the point file (plain `mv`, never `git mv` --
`ai/rules/points/` is untracked), the rule's `manifest.md` line in place, and
every `# ze point:` comment naming it (60 comments for 47 points, because 9
points are bound by more than one check). `docs/contributing/rule-authoring.md`
carries one binding comment as an example and moved with them.

| Rule | Old slug | New slug | Bound by |
|------|----------|----------|----------|
| `architecture` | `anti-pattern-instead` | `reuse-the-existing-pattern-before-adding-one` | `c_check_existing_patterns` |
| `architecture` | `before-any-design-decision-communication-mechanism-naming` | `load-ze-context-before-any-design-decision` | `c_utils_package` |
| `architecture` | `persist-runtime-state-through-the-managed-zefs-store-never` | `persist-runtime-state-in-zefs-not-a-loose-file` | `c_direct_fs_state` |
| `architecture` | `principle-rule` | `apply-these-design-principles-to-every-decision` | `c_and_functions`, `c_yagni` |
| `cli` | `all-json-keys-lowercase-kebab-case-never-camelcase-or-snake` | `name-json-keys-in-lowercase-kebab-case` | `c_json_kebab` |
| `cli` | `errors-to-stderr-fmt-fprintf-os-stderr-error-v-n-err` | `return-exit-codes-and-write-errors-to-stderr` | `c_os_exit` |
| `cli` | `when-a-skill-covers-the-task-ze-rfc-ze-review-ze-implement` | `use-the-skill-instead-of-a-raw-agent` | `verdict` |
| `commands` | `never-pipe-make-go-test-go-build-golangci-lint` | `never-pipe-an-expensive-command-read-the-log` | `check_pipe_tail` |
| `commands` | `prefer-make-targets-a-bare-go-test-omits-ze-s-feature-build` | `run-commands-through-make-and-never-poll` | `check_pipe_tail`, `check_poll_loop` |
| `config` | `config-content-must-be-manipulated-through-one-of-two` | `manipulate-config-only-by-the-two-approved-methods` | `c_silent_ignore`, `c_version_config` |
| `evidence` | `a-line-number-in-a-document-is-legitimate-only-when-a` | `cite-a-line-number-only-when-a-generator-maintains-it` | `c_line_number_ref` |
| `evidence` | `before-claiming-code-behaves-a-certain-way-or-recommending` | `read-the-producing-code-before-claiming-behavior` | `c_design_without_lsp` |
| `evidence` | `if-enumerated-data-has-a-canonical-source-registry-map` | `derive-every-string-from-the-canonical-registry` | `c_hardcoded_commands` |
| `evidence` | `the-design-without-lsp-check-in-claude-hooks-pretool` | `investigate-source-in-session-before-writing-a-spec` | `c_design_without_lsp` |
| `git-safety` | `see-ai-instructions-md-destructive-git-commands-are` | `never-run-a-destructive-git-verb` | `check_destructive_git` |
| `go-standards` | `a-says-b-must-say` | `keep-file-cross-references-bidirectional` | `c_require_related_refs` |
| `go-standards` | `all-go-source-files-non-test-non-generated-must-have-design` | `every-go-file-carries-a-design-comment` | `c_require_design_ref` |
| `go-standards` | `engine-slogutil-logger-subsystem` | `log-through-slog-never-printf` | `c_temp_debug` |
| `go-standards` | `every-exported-struct-field-that-reaches-json-output-must` | `tag-every-json-field-with-a-kebab-case-name` | `c_json_kebab` |
| `go-standards` | `external-tools-only-ze-exabgp-plugin-ze-config-migrate` | `keep-exabgp-awareness-out-of-engine-code` | `c_exabgp` |
| `go-standards` | `panic-for-error-handling-allowed-prefixes-enforced-by-block` | `never-write-these-forbidden-go-patterns` | `c_ignored_errors`, `c_init_register`, `c_legacy_log`, `c_panic` |
| `go-standards` | `when-renaming-deleting-a-go-file-search-for-detail-overview` | `update-cross-references-when-a-file-moves` | `c_require_related_refs` |
| `goroutine-lifecycle` | `all-goroutines-must-be-long-lived-workers-never-per-event` | `keep-every-goroutine-a-long-lived-worker` | `c_goroutine` |
| `never-destroy-work` | `operation-scope-replacement` | `ask-before-deleting-or-overwriting-user-work` | `c_point_overwrite` |
| `no-layering` | `forbidden-keep-old-add-new-hybrid-approach-gradual` | `never-keep-the-old-path-beside-the-new-one` | `c_layering` |
| `no-layering` | `when-replacing-x-with-y-delete-x-first-then-implement-y` | `delete-the-old-before-implementing-the-new` | `c_layering` |
| `performance` | `1-no-fmt-on-hot-paths-use-append-based-primitives-instead` | `never-use-fmt-or-string-on-a-hot-path` | `c_sprintf_new` |
| `performance` | `all-wire-encoding-must-write-into-pooled-bounded-buffers` | `write-wire-encoding-into-pooled-bounded-buffers` | `c_encoding_alloc`, `c_sprintf_new` |
| `performance` | `blocking-for-new-code-and-all-hot-paths-hook-enforced-at` | `build-strings-with-textbuf-never-with-plus` | `c_string_concat` |
| `performance` | `code-in-these-paths-must-not-use-any-fmt-function-or-string` | `apply-the-hot-path-ban-to-these-packages` | `c_format_alloc` |
| `performance` | `enforced-by-the-encoding-alloc-check-in-claude-hooks` | `audit-and-fix-encoding-allocations` | `c_encoding_alloc` |
| `performance` | `mistake-why-it-s-wrong-fix` | `fix-these-common-allocation-mistakes` | `c_fake_bufhandle` |
| `planning` | `event-status-change-phase-updated-when-exactly` | `update-spec-status-at-each-transition` | `c_source_edit_spec` |
| `planning` | `review-still-runs-on-opus-5-and-that-half-is-unchanged` | `run-every-review-on-opus-5` | `review_model_refusal` |
| `plugins` | `never-use-switch-case-to-dispatch-subcommands-all-command` | `dispatch-subcommands-by-registration-not-switch` | `c_switch_dispatch` |
| `quality` | `fix-lint-issues-never-disable-linters-only-exclusions` | `fix-lint-issues-never-disable-a-linter` | `c_lint_exclusions`, `c_nolint` |
| `repo-maintenance` | `before-editing-any-file-listed-in-the-generates-column` | `edit-the-canonical-source-not-the-generated-file` | `c_generated_files` |
| `repo-maintenance` | `canonical-source-generates-sync-command` | `sync-generated-files-from-their-canonical-source` | `c_generated_files`, `c_rendered_rules` |
| `repo-maintenance` | `project-wide-behavior-rules-workflow-rules-and-agent-rules` | `keep-shared-rules-in-ai-rules-and-render-them` | `c_rendered_rules` |
| `testing` | `a-test-carrying-an-rfc-requirement-id-polarity-tag-is-the` | `never-edit-an-rfc-tagged-test-to-match-the-code` | `_rfc_tagged_change_err` |
| `testing` | `blocking-gate-check-ci-sleep-justification-in` | `justify-every-sleep-in-a-ci-test` | `c_ci_sleep_justification` |
| `testing` | `detection-hook-c-observer-sys-exit-in-claude-hooks-pretool` | `fail-a-ci-observer-with-runtime-fail-not-sys-exit` | `c_observer_sys_exit` |
| `testing` | `each-test-subdir-has-its-own-runner-and-format-and-they-are` | `put-each-test-in-the-suite-that-runs-its-format` | `c_throwaway_tests` |
| `testing` | `tests-must-exist-and-fail-before-implementation` | `write-the-test-first-and-never-weaken-it` | `c_require_test_first`, `c_test_weakening`, `check_test_deletion` |
| `testing` | `use-project-tmp-gitignored-for-scratch-files-never-tmp` | `use-project-tmp-for-scratch-files` | `c_system_tmp_we`, `c_throwaway_tests`, `check_system_tmp` |
| `testing` | `when-a-test-fails-fix-the-code-to-make-the-test-pass-never` | `fix-the-code-when-a-test-fails-not-the-test` | `c_test_weakening` |
| `writing` | `detail-is-a-cost-the-reader-pays-not-proof-that-you-did-the` | `write-only-what-changes-the-next-action` | `c_line_number_ref` |

Invariants held. The 27 rendered `ai/rules/*.md` are byte-identical by
`shasum -a 256` before and after (a slug never reaches rendered output).
`make ze-rules-gate-map` still reports 47 gated and 0 dangling.
`make ze-rules-render-check`, `make ze-rules-points-roundtrip`,
`make ze-rules-lint`, `scripts/dev/rules_points_test.py` and
`scripts/dev/hook-fixture-check.py` all exit 0. No point BODY changed.

Two references were left alone on purpose. The A-5 and finding-1 rows above name
old slugs, and they are a RECORD of what was true when they were written; the
table here resolves them. `ai/rules/rule-format.md` shows a slug under a
hypothetical `points/buffer-first/` rule that does not exist, and editing it would
break the byte-identical invariant.

### Follow-on: corpus dedup (owner-authorised, 2026-08-07)

The corpus is now measurable, so the first measurement is what it says twice.
Normalising every point body and grouping by rule found exactly three pairs that
are the SAME sentence twice, all three in `completion`, all three the `## Directives`
abstract restated in a body section. This is not a corpus convention: the other
26 rules restate nothing verbatim. Points: 2428 before, 2422 after.

Byte-identity with HEAD does not apply here. It held for every earlier phase
because those phases changed the split, never the text. This one changes the
text on purpose, so `ze-rules-render-check` and `ze-rules-points-roundtrip` are
the invariant instead: the split and the render stay faithful to what the points
now say.

| Pair | Verdict | Reason |
|------|---------|--------|
| `completion/you-may-not-claim-work-is-done-...` + `-2` | MERGED, abstract kept | Same sentence. `## The Rule` now opens on a lead clause pointing at the ban, so the "Deferred does not mean done" vocabulary is not orphaned |
| `completion/before-changing-code-to-make-a-symptom-go-away-...` + `-2` | MERGED, abstract kept | Same sentence. `## Diagnosis Before Fix` now opens on `### The Diagnosis (write all five before any edit)`, a shape 46 other sections in the corpus already use |
| `completion/every-exported-function-type-or-constant-...` + `-2` | MERGED, abstract kept | Same sentence. The `### The Wiring Rule` heading went with it: a heading with nothing under it is a hole, and the rule is still stated in the abstract, in requirement 9 of "What Done Requires", and in `### Mechanical Check` |
| `completion/before-marking-any-spec-done-...` (two spellings) | MERGED, abstract kept | One article apart. The body point held a `Rationale:` pointer, so it survives as a pointer-only point, which nine other points in the corpus already are |
| `completion/every-feature-needs-at-least-one-end-to-end-test-...` | MERGED into the abstract line | The abstract already carries the sentence, and the Feature Type / Required Test table under it enumerates exactly what it asks for |
| `plugins/the-engine-s-text-mode-filter-protocol-inlines-nlri` | MERGED into the table | `ai/rules/writing.md` says to keep the table and delete the paragraph that draws the same cut. The two facts only the paragraph carried, `FilterUpdateInput.Raw` and "every future non-CIDR family", moved into the table cells |
| `completion/never-use-phrases-like-would-you-like-me-to-...` + `-2` | KEPT BOTH | The body copy is what the two exception blocks under `## Don't Ask, Do` attach to. Delete it and the section opens on "Exception:" with no rule stated, which is the partial-reader failure the restatement exists to prevent |
| `writing/project-text-is-us-english-...` + `write-what-changes-the-reader-s-next-action-...` | KEPT BOTH | Same partial-reader test. The first states two obligations, one of them the US English rule that carries the UK English exception for Thomas's prose; the second is the thesis of `## Detail Budget`. Neither restates the other in full |
| `performance/anti-pattern-fix-2` + `pattern-replacement` | KEPT BOTH | The pair is misdiagnosed. `anti-pattern-fix-2` restates rows from all four tables in `## Banned Patterns`, not from `pattern-replacement` alone, and holds two rows nothing else states. Fixing it means rebuilding four tables, which is a compression pass and needs its own scope |
| `cli/examples` + `cli/examples-2` | KEPT BOTH | Lead-in labels before two different example blocks. Merging orphans a block |

The test applied to every pair: does the second copy exist so a reader who sees
only the first is not misled? Where the answer is yes, repetition is the guard
and both copies stay.

Gates: `ze-rules-render-check`, `ze-rules-points-roundtrip`, `ze-rules-lint`,
`ze-rules-condensed-check`, `ze-rules-index-check`, `ze-rules-gate-map`
(47 gated, 0 dangling, 0 regressed), `ze-rules-router-report` (27 rules,
core membership unchanged), `rules_points_test.py` (52), `hook-fixture-check.py`
(213), `ze-doc-test`. `TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md` are
byte-identical after the regeneration: neither `completion` nor `plugins` is
always-on, so no directive of theirs reaches `CORE.md`, and no title, trigger or
severity changed.

### Follow-on: fixed depth two, a directory per section (owner-authorised, 2026-08-07)

`ai/rules/points/<rule>/` was flat, so a point id was two components and a `##`
heading was a one-line point FILE beside the instructions it introduced. The id
now carries the section: `ai/rules/points/<rule>/<section>/<slug>.md`, three
components for every one of the 2,142 points, and a `##` heading is the section
DIRECTORY. A `###` or `####` heading stays a point inside its section, which is
what holds the depth at two rather than letting it follow how a rule nests.

A directory name is a slug and cannot carry the heading's text, capitalisation,
punctuation or level, so the manifest carries the heading. That makes the
manifest the rule's full structural SPINE rather than only its reading order,
which is what it already was for `title`, `when`, `severity` and `related`.

| Manifest body line | Shape |
|--------------------|-------|
| Section | `<dir-slug> ## The Heading Line Verbatim` -- a slug holds no space, so the split is unambiguous and the heading is recovered byte-exactly |
| Point | the slug, indented by two spaces. No slug can start with whitespace, so the two shapes cannot be confused |
| Anything else | a hard error. A skipped line is an instruction that stops rendering with nothing going red (R-4) |

`render_dir` in `scripts/dev/rules_points.py` now fails on four more shapes,
each one a way to lose an instruction silently: a `*.md` sitting directly in a
rule directory (the old flat layout is exactly that shape), a section directory
no manifest lists, a listed section with no directory, and a section listing no
point. `c_point_overwrite` in `.claude/hooks/pretool-writeedit.py` tested ONE
depth, so after the move it permitted a Write over every point; it now answers
for both canonical shapes, the rule manifest and the section point.

| Measure | Before | After |
|---------|--------|-------|
| Rule directories | 27 | 27 |
| Section directories | 0 | 280 |
| Point files | 2,422 | 2,142 (the 280 `##` heading points became directories) |
| Point id components | 2 | 3 |
| `# ze point:` comments rewritten | -- | 61 (60 bindings in the three dispatchers, 1 example in `docs/contributing/rule-authoring.md`) |

The move is PURE: all 30 files under `ai/rules/` are byte-identical by
`shasum -a 256` before and after it. `ai/rules/rule-format.md` then changed on
purpose, and alone, because its points documented the flat layout and would
otherwise describe a tree that no longer exists (`ai/rules/stale-comments.md`).
`TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md` stayed byte-identical: the doc
edit touched bodies only, never a title, a trigger or a severity.

The gated ratchet reads 0 REGRESSED because a renamed id is a point that no
longer EXISTS under its old ref, which `gated_regressions` documents as out of
scope. The 47-point gated set is unchanged and 0 dangling is what proves every
binding was rewritten.

Two headings encode a count that a future edit invalidates,
`### The six habits to avoid` and `### Why two variants`, both in `writing.md`.
Both are `###`, so both stayed point FILES and no directory name inherited the
defect. Their slugs carried it before this work and still do. Not fixed here: it
is a content change and was not authorised.

### Follow-on: `excepted-by`, AC-20 and AC-21 (owner-authorised, 2026-08-07)

A general instruction must carry its own exception, or a reader who stops after
the general statement is misled. `ai/rules/writing.md` does that by hand-repeating
the UK English exception at three levels, and that repetition is load-bearing
while being invisible: during a dedup pass on 2026-08-07 an agent was about to
delete one copy and every gate would have stayed green. Only a mid-run warning
stopped it.

`excepted-by` is a fifth point key, optional, written only when it carries a
value, exactly as `rationale` is. It is declared on the GENERAL point and names
the EXCEPTION, because the general point is the one that misleads. A ref naming
no point on disk fails `ze-rules-gate-map`, so deleting an exception point turns
the general point's link dangling and the gate goes red. That is the whole
protection, and it costs one line per pair.

Nine candidate points were read in their rendered context. Seven are members of
a real pair and are linked; two are not pairs.

| General point (carries the key) | Exception it names | Why it is a pair |
|---------------------------------|--------------------|------------------|
| `writing/directives/the-project-language-is-us-english-...` | `writing/language-and-spelling/prose-written-in-thomas-s-voice-...` | The verified case. The rule-level abstract states the US English obligation and mentions the exception in one clause |
| `writing/language-and-spelling/this-section-picks-the-english-variant-...` | the same point | The section-level restatement. Second of the three levels the repetition covers |
| `cli/cli-grammar-keywords-before-values/the-first-token-after-the-noun-...` | `cli/.../peer-commands-are-the-explicit-exception-to-the-generic` | The normative MUST that peer commands are named as excepting. A reader who stops here types `show bgp peer name <n>` |
| `cli/cli-grammar-keywords-before-values/use-an-explicit-selector-kind-...` | the same point | The Typed Selectors restatement, which is the form the exception names by title |
| `cli/cli-patterns/every-user-facing-command-must-have-tab-completion-no` | `cli/cli-patterns/opt-out-set-hidden-true-on-a-commanddecl-...` | "No exceptions by default" is a general MUST whose one opt-out is stated four blocks later |
| `performance/banned-patterns/build-strings-with-textbuf-never-with-plus` | `.../the-only-exception-is-a-compile-time-constant-expression`, `.../existing-cold-path-concatenation-is-cleanup-on-touch-not-a` | The two-exception case the comma-separated value exists for: one carve-out for const folding, one for the ~300 legacy cold-path sites |
| `go-standards/no-backwards-compatibility/code-under-internal-is-not-...` | `go-standards/.../the-only-exception-is-the-plugin-api-the-surface-that` | "no shims, no deprecation layers" reads as the whole rule; the frozen plugin API contract is stated only in the next block |

| Candidate NOT linked | Reason |
|----------------------|--------|
| `config/config-surface-yang-config-vs-env-var/default-answer-yang-config-env-only-is-the-exception-not` | Not a pair. The point states its own qualifier inline, and env-only is a co-equal branch of the decision table above it with its own enumerating section. No single point is the exception, so any link would be invented |
| `config/yang-module-structure/rule-detail` | Not a pair. The exception is a ROW inside this same table point ("Standard admin-state words are the only exception"). General and exception are one point, and a point may not except itself |

One sibling was left unlinked on purpose:
`go-standards/no-backwards-compatibility/ze-has-never-been-released-no-users-...`
says "no compat anywhere, including the plugin API", which is TRUE today. The
plugin API exception applies post-release only, so linking it would claim an
exception that does not yet hold.

Invariants. All 30 files under `ai/rules/` are byte-identical by `shasum -a 256`
before and after the links: `excepted-by` is a header field and never reaches a
body. `ai/rules/rule-format.md` then changed on purpose, and alone, because its
`field-values-meaning` table documented three keys when there are five
(`ai/rules/stale-comments.md`). `TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md`
stayed byte-identical: no title, trigger or severity moved. `ze-rules-gate-map`
reports 47 gated, 0 dangling, 0 regressed, 0 missing rationale, 0 missing
exception, and EXCEPTED 7 of 1584 naming 6 points.

## RFC Documentation (Scope: protocol)

Not applicable. No protocol code is touched.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] Functional hook-fixture tests for the edit refusal
- [ ] Interop tests for protocol features (N-A: no protocol surface)

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

The code landed in three commits before this closure, so commit A carries no
code: the spec, the learned summary and the regenerated learned index only.

| Commit | What it carried |
|--------|-----------------|
| `299334063` | The split. 2199 files. `ai/rules/points/<rule>/<section>/<slug>.md` becomes canonical, `ai/rules/<rule>.md` becomes generated, the three dispatchers gain `# ze point:` bindings, and `scripts/dev/rules_points.py` plus `scripts/dev/rules_points_test.py` are created |
| `7f03b3dfa` | Review Gate round 2. Four gates that were green because they could not fire: a case-variant path that clobbered a point, a point one level too deep that rendered nothing, a `##` heading inside a point body that made the section mapping non-total, and a ratchet that read an empty baseline as a clean one |
| `513b427c1` | Review Gate round 3. `ai/rules/points/RETIRED.md` becomes a validated ledger: a declared retirement exempts a check from the unbound regression, and a row naming a fictional id, a live point, or a duplicate is refused |

`a30ad29be` sits before all three. It repaired another closure's dangling
references and is separable from this work.

Five surfaces outside `ai/rules/` had to learn about `ai/rules/points/`, all in
phase 3: `default_files` in `scripts/dev/ste_check.py`, `targets` in
`scripts/dev/line_refs.py`, `read_transcript` in `scripts/dev/rule_coverage.py`,
and `c_line_number_ref` plus `c_enforce_naming` in
`.claude/hooks/pretool-writeedit.py`. A-4 predicted none of them.

### Bugs Found/Fixed

- A `Write` over an EXISTING point was permitted, and phase 5 used it to clobber a point file. Fixed by the sibling check `c_point_overwrite` in `.claude/hooks/pretool-writeedit.py`, six fixtures in `.claude/hooks/fixtures/`.
- `c_point_overwrite` and `c_rendered_rules` compared realpath STRINGS, and realpath resolves symlinks but not case, so on a case-insensitive filesystem `AI/rules/points/...` landed on the real file with exit 0. Fixed by asking the filesystem for identity through `os.path.samestat`, nine fixtures.
- `render_dir` in `scripts/dev/rules_points.py` globbed `*.md` non-recursively, so a point one level too deep rendered nothing while every gate exited 0. `make ze-regen` runs render in write mode, so the instruction would have been deleted. That is R-4 realized.
- `head_sources` returned `{}` both when git answered for no file and when a dispatcher was renamed, so the ratchet printed `REGRESSED: 0` from a baseline holding nothing. HEAD is now probed once with `rev-parse --verify` and `None` means git could not answer.
- Deleting a point and its manifest line together was invisible to every gate. Fixed by comparing each rule's point count against `git ls-tree HEAD`, with the drop declared in `ai/rules/points/RETIRED.md`.
- `retired_rows_since` believed a fictional id, so declaring one bought a real deletion elsewhere in the same rule. It is now pure and validated.

### Documentation Updates

- `docs/contributing/rule-authoring.md` -- created. The layout, the five authoring tasks, the frontmatter fields, what the renderer refuses, the binding convention and the generator order.
- `docs/contributing/README.md` -- the index row for `rule-authoring.md`.
- `ai/INDEX.md` -- Dev Tools rows for `ze-rules-render` / `ze-rules-render-check`, `ze-rules-points-roundtrip` and `ze-rules-gate-map`.
- `ai/rules/rule-format.md` -- rewritten through its points for the point format.
- `ai/rules/repo-maintenance.md` -- Sync Flows row, the Rule Placement bullet, a Banned Actions row, and the Hook-to-Rule Mapping table now held to the bindings by `hook_table_problems`.
- `make ze-doc-test` result: red on ONE line, `WARNING: ai/LEARNED-FULL-INDEX.md is stale`. Every other stage passes, including the three new rules-points stages. The staleness is the shared checkout: another session holds `plan/learned/1358-dev-setup-cross-platform.md` untracked. See Documentation Verified below.

### Deviations from Plan

- **AC-6 says `TRIGGERS.md` stays byte-identical. It did not, by one row.** `299334063` changed the `rule-format.md` trigger in `ai/rules/TRIGGERS.md` and `ai/rules/INDEX.md`, from "authoring or editing any `ai/rules/*.md` rule file" to "authoring or editing any rule: a point file under `ai/rules/points/`, its manifest, or a check's binding comment". This is the deliberate rewrite the Files to Modify list requires, not a regression of the payload: a trigger is a manifest field, so rewriting the rule moves it. `ai/rules/CORE.md` is untouched by the split (`git log -- ai/rules/CORE.md` stops at `5d3e99d65`, before it).
- The corpus dedup, the depth-two move and the `excepted-by` key were added on owner instruction after the spec was written. Each is recorded in its own follow-on section above.
- Point count moved through the work: 2417 at the split, 2422 after the flat-layout re-render, 2142 after the depth-two move turned 280 `##` heading points into directories, 2143 today.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 claimed `ai/rules/points/` was invisible to EVERY existing consumer of `ai/rules/`, on the basis that all three named globs are non-recursive | The three named globs really are non-recursive, so the predicted phantom 28th rule never appeared. The SCOPE was wrong: five more consumers reach the tree. Four walk it recursively (`default_files` in `ste_check.py`, `targets` in `line_refs.py`, `read_transcript` in `rule_coverage.py`, and the naming check), one by a prefix test. `ste_check.py` reviewed every point body a second time, and 951 of the 2417 lines its ratchet printed were duplicates. `line_refs.py --apply` would have rewritten a GENERATED rule. `rule_coverage.py` credited `os.path.basename`, and four point slugs equal a rule stem, so opening one point falsely cleared a blocking rule | Phase 3, by grepping for consumers rather than trusting the enumeration. The assumption named three and stopped | All four fixed at source in phase 3, with two tests on `_is_rule_path`. The lesson: an assumption that names a closed set of consumers must be validated by a search, never by the list that produced it |
| assumption | A-5 claimed nothing outside the three dispatchers needs to name a point, on the basis that the handover scoped STEP 4 to the 44 checks | Two points are already enforced at BOTH ends and only the hook end names one: `_model_refusal` in `scripts/dev/review_gate.py` beside `review_model_refusal` in `.claude/hooks/pretool-agent-skill.py`, and `check_ci_sleep_justification` in `scripts/dev/verify_wiring_docs.py` beside `c_ci_sleep_justification`. Four more families want a binding: the `commit_helper.py` gates, the twelve RFC ratchets in `rfc_requirements.py`, `block-premature-stop.sh` and `validate-spec.sh` | Phase 4, by the grep the assumption's own "Validated by" column asked for | Scope NOT expanded, and Known Limitations already scoped phase 4 to the dispatcher checks. `parse_bindings` reads whatever file list it is given, so a later spec adds readers and never a second convention. The lesson: "the design scopes it here" answers where the WORK stops, never whether the claim is true |
| approach | Phase 5 renamed a point with a `Write` and destroyed `check-enforces-triggers-on-what-it-does-4.md`, recovering it only from git | `c_rendered_rules` returns `None` for any path whose dirname is not `ai/rules`, which is what makes AC-8 true, so nothing answered the opposite question. `write_split` refuses the same move and cites `ai/rules/never-destroy-work.md` | Review Gate round 1, finding 1 | New sibling check `c_point_overwrite`, six fixtures. A guard that answers "is this generated" cannot be asked "is this already written" by adding a branch |
| escalation | Four gates added by this work were green because they could not fire, and a fifth believed a declaration it never validated | The class is the same one round 1 found and fixed in its instances: a gate whose input is empty reports success. `head_sources` reading `{}` as a baseline is the cleanest instance | Review Gate rounds 2 and 3, three independent lenses then one | Fixed in `7f03b3dfa` and `513b427c1`. Every fix carries a mutation proof. `test_gate_map_empty_result_is_never_success` is the general form and is now a test rather than a habit |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One instruction becomes one checked-in file whose PATH is its id | Done | `ai/rules/points/<rule>/<section>/<slug>.md`, 2143 points, 27 rules, 280 sections | `split_rule` and `render_dir` in `scripts/dev/rules_points.py` |
| The rendered `ai/rules/<rule>.md` becomes generated, so agents read the same bytes | Done | `render_all` in `scripts/dev/rules_points.py`, `make ze-rules-render` | `make ze-rules-points-roundtrip`: all 27 round-trip byte-identical |
| The checks declare which points they enforce | Done | 68 `# ze point:` comments across the three PreToolUse dispatchers | 47 points named by 48 checks, 8 checks declaring `none -- <why>` |
| A gate reports gated, ungated and dangling | Done | `report_gate_map` in `scripts/dev/rules_points.py`, `make ze-rules-gate-map` | Nine sets, not three: gated, dangling, regressed, declared-none, shrunk, unbound, missing rationale, missing exception, ungated |
| Problem 1: we cannot tell which instructions have a machine behind them | Done | the gate map's GATED set | 47 named instructions, by path |
| Problem 2: we cannot count what any instruction ever prevented | Done | a refusal now cites a point, not a file | The counter's granularity is the point |
| Problem 3: a reworded instruction silently keeps its gate | Changed | answered in REVIEW, not by a machine | `gate_map` joins on `Binding.ref` alone. Handover D2 rejected a content hash and that decision stands; a digest on the BINDING is a separate design, homed in the deferral shard. `docs/contributing/rule-authoring.md` and `ai/rules/points/rule-format/rationale/one-instruction-one-file.md` say so rather than claiming three |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test_split_partitions_every_line` | asserted by summation over line indices, over all 27 real rules |
| AC-2 | Done | `test_roundtrip_every_committed_rule`, `make ze-rules-points-roundtrip` | "all 27 rules round-trip byte-identical" |
| AC-3 | Done | `test_render_fails_on_unlisted_point` | `render_dir` names the unlisted file and renders nothing |
| AC-4 | Done | `test_render_fails_on_missing_and_duplicate_slug` | plus `test_render_rejects_unsafe_slug` for the security row |
| AC-5 | Done | `test_body_opening_with_delimiter`, `test_point_file_on_disk_opening_with_delimiter` | in memory and through the filesystem |
| AC-6 | Changed | `make ze-rules-lint`, `ze-rules-condensed-check`, `ze-rules-index-check` all pass | `CORE.md` byte-identical. `TRIGGERS.md` and `INDEX.md` moved by ONE row, the deliberate `rule-format.md` trigger rewrite. See Deviations |
| AC-7 | Done | fixtures `rendered-rule-edit-refused`, `-write-refused`, `-index-refused`, `-triggers-refused`, `-core-refused`, `-relative-path-refused` | `c_rendered_rules` exits 2 and names the point directory |
| AC-8 | Done | fixtures `point-file-edit-allowed`, `point-manifest-edit-allowed`, `point-overwrite-edit-allowed` | |
| AC-9 | Done | `TestRegenCheckReadonlyCoversGenerators` | `make ze-test-pkg PKG=./scripts/status` ok |
| AC-10 | Done | `test_gate_map_sets_and_exits`, `test_gate_map_over_the_real_dispatchers` | GATED 47, exit 0 |
| AC-11 | Done | `test_gate_map_sets_and_exits` dangling branch | DANGLING 0 on the real tree, and non-zero exit when one is planted |
| AC-12 | Done | `test_gate_map_sets_and_exits`, real run | UNGATED 1538 of 1585, exit 0 |
| AC-13 | Done | `make ze-rules-index-check`, `ze-rules-condensed-check` | 27 rules in `INDEX.md`, 27 rows in `TRIGGERS.md`. No phantom rule |
| AC-14 | Done | `make ze-rules-router-report` | 27 rules, 5 always-on by precedence plus 3 no task surfaces, which is `CORE.md`'s 8 |
| AC-15 | Done | `RationaleTest` (7 cases) | `rationale` parses, its absence parses, and it never reaches a body |
| AC-16 | Done | `RationaleTest`, `rationale_problems` | MISSING RATIONALE: 0 on the real tree; a planted missing path exits non-zero |
| AC-17 | Done | real run | RATIONALE: 11 of 1585, exit 0 |
| AC-18 | Done | `DrainPolicyTest` (`scripts/dev/learned_staleness_test.py`), `plan/.learned-staleness-drain` | `start 2026-08-07`, `rate 0`. `ze-doc-test` prints "learned drain: INERT" |
| AC-19 | Done | `ai/rules/points/planning/writing-learned-summaries/the-staleness-ceiling-is-drained-never-removed.md`, `plan/learned/1356-learned-corpus-drain-over-archive.md` | the directive point refuses the retirement and names the drain; the summary carries the measurement and why citation count is the wrong metric |
| AC-20 | Done | `ExceptedByTest` (10 cases) | includes the mutation: deleting an exception point reds the gate |
| AC-21 | Done | real run | EXCEPTED: 7 of 1585 naming 6 points, MISSING EXCEPTION: 0, exit 0 |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every unit test named in the TDD plan | Done | `scripts/dev/rules_points_test.py` | 105 tests, `Ran 105 tests ... OK`. The plan named 31 entries; rounds 2 and 3 added the rest |
| `TestRegenCheckReadonlyCoversGenerators` | Done | `scripts/status/verify_run_test.go` | `ok github.com/ze-software/ze/scripts/status 1.920s` |
| `rendered-rule-edit-refused`, `point-file-edit-allowed` and 33 siblings | Done | `.claude/hooks/fixtures/` | `hook-fixture-check.py --only rendered-rule`: 35/35 passed |
| Whole hook fixture suite | Done | `.claude/hooks/fixtures/` | 232/232 passed |
| `DrainPolicyTest` | Done | `scripts/dev/learned_staleness_test.py` | 30 tests, OK |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rules_points.py` | Done | `split`, `render`, `coverage`, plus the retirement ledger and the published-table check |
| `scripts/dev/rules_points_test.py` | Done | 105 tests |
| `ai/rules/points/<rule>/` | Changed | depth two, not flat: `<rule>/<section>/<slug>.md`. 27 rule directories, 280 section directories, 2143 points |
| `docs/contributing/rule-authoring.md` | Done | |
| `Makefile`, `mk/inventory.mk` | Done | `ze-regen`, `ze-regen-check-readonly`, `ze-doc-test`, and the four new targets |
| `scripts/status/verify_run_test.go` | Done | `regenCheckPrereqs` and `generatorChecks` entries |
| `.claude/hooks/pretool-writeedit.py`, `-bash.py`, `-agent-skill.py` | Done | `c_rendered_rules`, `c_point_overwrite`, and 68 binding comments |
| `ai/rules/rule-format.md`, `ai/rules/repo-maintenance.md` | Done | through their points |
| `ai/INDEX.md`, `docs/contributing/README.md` | Done | |
| `ai/rules/points/RETIRED.md` | Changed | not in the plan. Added by round 3 as the declaration the shrink check reads |
| `plan/.learned-staleness-drain` | Changed | not in the original plan. Added by AC-18 |

### Audit Summary

- **Total items:** 61 (7 requirements, 21 AC, 5 test groups, 11 file rows, 17 documentation rows)
- **Done:** 55
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 6 (Problem 3, AC-6, and the four file rows above, each recorded in Deviations or in a follow-on section)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The repository can name which individual instructions have a machine behind them | functional, `make ze-rules-gate-map` | `gate map: 2143 points, 68 bindings, 56 checks` / `GATED: 47 point(s) named by 48 check(s)` / `DANGLING: 0` / `REGRESSED: 0` / `UNGATED: 1538 of 1585 instruction points`. Each gated line prints the point path and the checks that name it, for example `performance/three-rules/never-use-fmt-or-string-on-a-hot-path <- c_sprintf_new` |
| An agent keeps reading exactly the file it reads today | functional, byte comparison | `make ze-rules-points-roundtrip`: "all 27 rules round-trip byte-identical". `make ze-rules-render-check`: "27 rules are fresh". `git diff --quiet -- ai/rules/` is clean against HEAD |
| The always-loaded session payload cannot regress | functional | `make ze-rules-condensed-check`: TRIGGERS.md (27 rules) and CORE.md (8 rules) up to date, from a `rules_condensed.py` this work never modified. `ai/rules/CORE.md` is byte-identical across the split (`git log -- ai/rules/CORE.md` stops before `299334063`) |
| A dangling or lost gate is a red, never a silent pass | functional, mutation | `test_gate_map_empty_result_is_never_success`, `test_gate_map_reports_a_binding_that_gates_nothing`, `GatedRatchetTest` (4), and the round-3 mutations re-run against the real tree: deleting a real point while declaring a fictional retirement exits 2 and names both |
| An author can find and change one instruction | functional | `docs/contributing/rule-authoring.md`, and `ai/INDEX.md` Dev Tools rows for the four targets. `make ze-rules-render` refuses an unlisted point, a listed slug with no file, a duplicate slug and an unsafe slug, so a silent drop is not reachable |
| Coverage is a measurement, never a red (handover D5) | functional | The same run exits 0 and reports UNGATED 1538, RATIONALE 11 and EXCEPTED 7. Only DANGLING, REGRESSED, DECLARED NONE, SHRUNK, MISSING RATIONALE, MISSING EXCEPTION and PUBLISHED can fail it |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `DemoLockTest` in `demos/terminal/test_render.py` depends on `flock -F`, which the macOS flock(1) does not carry | done | Fixed outside this spec by `db493f22a`. The lock holders are now a Python child taking `fcntl.flock` and writing a ready marker, which is the fix the row named as preferred. `plan/learned/1357-flock-macos-is-not-util-linux.md` carries the reasoning. Verified: no `flock -F` remains in `demos/`, and `test_render.py` imports `fcntl` |
| `check_hook_names` in `scripts/dev/check_doc_links.py` cannot see a dead check name in a rule file, and stale `c_model_phase` prose survives it | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. Row kept live. Its two point paths were repaired at closure: the depth-two move renamed them to `planning/work-phases/implementation-carries-no-model-requirement.md` and `planning/work-phases/how-the-model-phase-gates-work-and-where-they-stop.md` |
| `c_check_existing_tests` in `.claude/hooks/pretool-writeedit.py` is `return None` under a comment promising a warning | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. Row kept live. The gate map now names it: `c_check_existing_tests: the function is a no-op that always returns None, so it enforces nothing` |
| An authored `title:` field for the roughly 1538 ungated points | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. Row kept live |
| A digest of the point BODY on the `Binding` | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. Row kept live. This is the third Task problem, and Implementation Audit records it as Changed rather than Done |
| `is_test_path` in `scripts/dev/audit-test-relaxation.py` cannot see a Python test | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. Row kept live. `scripts/dev/audit-test-relaxation.py` is being edited by a concurrent session, so this closure does not touch it |

**The shard SURVIVES this closure.** Five of its six rows are live and homed at
`plan/spec-fixit-test-harness-fail-open-guards.md`, which is a `skeleton` and has
not yet enumerated them. A shard holding a live row outlives its source spec, so
commit B removes the spec and nothing else. No FOREIGN shard was emptied: the
`flock` row is written down only here, and no other shard in `plan/deferrals/`
mentions it.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/rules_points.py` | Yes | `ls`: 93K, 2026-08-07 13:25 |
| `scripts/dev/rules_points_test.py` | Yes | `ls`: 96K, 2026-08-07 13:21 |
| `docs/contributing/rule-authoring.md` | Yes | `ls`: 14K, 2026-08-07 12:42 |
| `ai/rules/points/` | Yes | `ls -1d ai/rules/points/*/ \| wc -l` = 27 rule directories, plus `RETIRED.md` and the top-level manifest entries |
| `ai/rules/points/RETIRED.md` | Yes | `ls`: 2.2K, 2026-08-07 13:28 |
| `plan/.learned-staleness-drain` | Yes | read: `start 2026-08-07`, `rate 0` |
| `.claude/hooks/fixtures/` rendered-rule and point-overwrite fixtures | Yes | `hook-fixture-check.py --only rendered-rule` enumerates all 35 by name |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | the split is a total partition and renders back byte-identical | `make ze-rules-points-roundtrip` exit 0: "rules-points: all 27 rules round-trip byte-identical" |
| AC-3, AC-4, AC-5 | the renderer fails closed on an unlisted, missing, duplicate or unsafe slug, and a triple-dash body survives | `python3 scripts/dev/rules_points_test.py`: `Ran 105 tests in 2.007s` / `OK` |
| AC-6 | the rendered corpus still lints and the payload is fresh | `make ze-rules-lint` exit 0 ("27 rule file(s) conform"), `make ze-rules-condensed-check` exit 0, `make ze-rules-index-check` exit 0 |
| AC-7, AC-8 | an edit to a rendered rule is refused, an edit to a point is permitted | `python3 scripts/dev/hook-fixture-check.py --only rendered-rule` exit 0: "hook fixture check: 35/35 passed" |
| AC-9 | the generator is in `ze-regen` and its check in `ze-regen-check-readonly` | `make ze-test-pkg PKG=./scripts/status` exit 0: `ok github.com/ze-software/ze/scripts/status 1.920s` |
| AC-10, AC-11, AC-12 | gated is listed, dangling is a red, ungated is a measurement | `make ze-rules-gate-map` exit 0: GATED 47 / DANGLING 0 / REGRESSED 0 / UNGATED 1538 of 1585 |
| AC-13, AC-14 | no phantom rule, and the router sees the same corpus | `make ze-rules-index-check`: "checked 27 rules". `make ze-rules-router-report` exit 0: "rules: 27 (5 always-on core, 22 routed)" |
| AC-15, AC-16, AC-17 | `rationale` parses, a missing path reds, coverage never reds | `make ze-rules-gate-map`: "MISSING RATIONALE: 0" and "RATIONALE: 11 of 1585", exit 0 |
| AC-18, AC-19 | the drain ships inert and a point refuses the retirement | `python3 scripts/dev/learned_staleness_test.py` exit 0, `Ran 30 tests ... OK`. `make ze-doc-test` prints "learned drain: INERT (rate 0 per calendar month since 2026-08-07), floor 0" |
| AC-20, AC-21 | `excepted-by` parses and reds on a dangling ref, and its coverage never reds | `make ze-rules-gate-map`: "MISSING EXCEPTION: 0" and "EXCEPTED: 7 of 1585 instruction points ... naming 6 point(s)", exit 0 |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-rules-points-roundtrip` | `scripts/dev/rules_points_test.py`, `test_roundtrip_every_committed_rule` | Yes. The target runs the real `split` then `render` over the committed corpus and compares bytes, and it is wired into the `ze-doc-test` recipe in `mk/inventory.mk` (round-1 finding 2) |
| `make ze-rules-render` | `render_all` writing `ai/rules/<rule>.md` | Yes. `git diff --quiet -- ai/rules/` is clean after the render, so what the tree holds is what the points produce |
| `make ze-rules-render-check` | `TestRegenCheckReadonlyCoversGenerators` | Yes. The Go test asserts `regenCheckPrereqs` and `generatorChecks` as exact sets in both directions, and it is green |
| An Edit to `ai/rules/performance.md` | `.claude/hooks/fixtures/` fixture `rendered-rule-edit-refused` | Yes. Read: the fixture drives the whole `pretool-writeedit.py` dispatcher through `_writeedit` (round-1 note N2), not `c_rendered_rules` directly |
| `make ze-rules-gate-map` | `test_gate_map_sets_and_exits`, `test_gate_map_over_the_real_dispatchers` | Yes. The second reads the committed dispatchers, so a binding rewritten without its point is a red in the suite as well as in the target |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Re-measured 2026-08-07: zero consecutive blank lines outside fences, zero blank line at EOF. `block_ranges` defines a block as a maximal non-blank run, so a zero gap is unreachable, and `split_rule` raises when a gap is not exactly 1 |
| A-2 | confirmed | `make ze-rules-points-roundtrip`: all 27 byte-identical. Fence state is load-bearing: the corpus carries 66 blank lines inside fences |
| A-3 | confirmed | Zero horizontal rules outside fences and zero `---` lines inside them. `_frontmatter` stops the header at the first `---` after line 1, so the body is never reached. `test_body_opening_with_delimiter` |
| A-4 | **broken** | The predicted consequence did not happen (27 rules in `INDEX.md`, 27 rows in `TRIGGERS.md`, 8 in `CORE.md`), but the assumption's SCOPE was wrong: five consumers reach `ai/rules/points/`, four by a recursive walk and one by a prefix test. Four were fixed at source in phase 3. Mistake Log row 1 |
| A-5 | **broken** | Enforcement outside the three dispatchers exists and is already paired with a bound point: `_model_refusal` in `scripts/dev/review_gate.py`, and `check_ci_sleep_justification` in `scripts/dev/verify_wiring_docs.py`. Four more families want a binding. Scope was NOT expanded; `parse_bindings` takes a file list, so a later spec adds readers. Mistake Log row 2 |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #6 user guide: `docs/contributing/rule-authoring.md` | The page describes the depth-two layout, and `ls -1d ai/rules/points/*/ \| wc -l` = 27 with 280 section directories under them | Yes |
| #6 index row in `docs/contributing/README.md` | `grep -n rule-authoring docs/contributing/README.md` line 11 | Yes |
| #15 inventory: `ai/INDEX.md` Dev Tools rows | `grep -n` finds rows 231, 232, 233 for `ze-rules-render` / `-render-check`, `ze-rules-points-roundtrip` and `ze-rules-gate-map`, each naming `scripts/dev/rules_points.py` | Yes |
| #16 source anchors naming `rules_condensed.py` and `rule-format.md` | `rules_condensed.py` is unmodified by this work, so its anchors cannot have gone stale. `ai/rules/rule-format.md` was rewritten through its points and `make ze-rules-lint` passes over the result | Yes |
| #17 `ai/rules/rule-format.md` carried a skeleton rule that became wrong | Rewritten for the point format. `ai/rules/points/rule-format/` is its source, and `make ze-rules-render-check` reports it fresh | Yes |
| #1 to #5, #7 to #14 answered No | No `ze` command, RPC, plugin, YANG leaf, wire format or `internal/` change is in the three commits. `git show --stat` over `299334063`, `7f03b3dfa` and `513b427c1` shows `scripts/`, `.claude/`, `ai/`, `docs/`, `mk/`, `Makefile` and one `_test.go` only | Yes |
| `make ze-doc-test` | Red on ONE line: `WARNING: ai/LEARNED-FULL-INDEX.md is stale`. Every other stage passes, `make ze-rules-gate-map`, `ze-rules-index-check`, `ze-rules-condensed-check` and `ze-rules-points-roundtrip` among them. The staleness is a shared-checkout artifact: another session holds `plan/learned/1358-dev-setup-cross-platform.md` untracked in this tree, and the working-tree index carries ITS row rather than 1356's. Commit A carries the index regenerated against HEAD plus its own files, which is what `discovery_index_problems` in `scripts/dev/commit_helper.py` checks | Yes, attributed |

## Core Insight

**A gate whose input is empty reports success, and that is the failure mode this
work kept finding in itself.** Round 1 found it in instances. Round 2 found four
more and named the class. Round 3 found the round-2 fix believing a declaration
it never validated. The general form is now a test rather than a habit:
`test_gate_map_empty_result_is_never_success` asserts that no points and no
bindings both fail, and `check_new_summaries` in `scripts/dev/rfc_requirements.py`
is the same shape one domain over. The corollary is what made the count worth
having: a coverage number is only trustworthy when the machine that produces it
cannot reach that number by seeing nothing.
