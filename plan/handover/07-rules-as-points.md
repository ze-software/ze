# 07 -- Rules as points: one file per instruction

Handover for work not yet started. No code exists. Written 2026-08-06.

## RATIONALE (verify this matches what we agreed)

- **D1. The unit of enforcement is one instruction, not one rule file.** Every
  hook message, review finding and digest entry today names a file
  (`performance.md`, 902 lines). Nothing can name the sentence. That single fact
  is what makes three separate problems unfixable: we cannot tell which rules
  have a machine behind them, we cannot count what any rule ever prevented, and
  a reworded instruction silently keeps its gate. -> STEP 1
- **D2. One file per point, checked in, canonical. The PATH is the id.** This
  REPLACES the content-hash id proposed earlier in the session. A hash was only
  needed because the id was being derived from a file we did not control. An
  authored filename plus git history does both jobs the hash was doing (name the
  instruction, notice when it changes) and does them better: the change shows up
  as a one-file diff in review instead of as a dangling reference. -> STEP 1, STEP 2
- **D3. The rendered `ai/rules/<rule>.md` becomes GENERATED from the points.**
  Agents keep reading exactly the file they read today. It stops being canonical,
  which is the only change they can observe. -> STEP 3
- **D4. A table ROW is not a point. A whole table is one point.** Measured over
  the 27 rules: 3,099 directive-shaped lines outside `Rationale`/`Examples`,
  of which 1,923 are table rows spread over 293 distinct tables, 754 are
  bullets, 422 are bold lines. A row without its header means nothing, so the
  corpus is about **1,469 points**, not 3,099. -> STEP 1
- **D5. Gate binding runs bottom-up: the checks name their points.** The 44
  checks in the write/edit dispatcher each declare what they enforce. Nothing
  ever demands that all 1,469 points acquire a gate. The ungated count is a
  measurement, not a red. -> STEP 4
- **D6. The round trip is the first acceptance criterion and it is not
  negotiable.** Split the 27 files into points, render them back, and the result
  must be byte-identical before anything else is built. -> STEP 2

If any bullet is wrong, STOP and fix this handover before starting STEP 1.

## OPEN (decide before STEP 1)

| # | Question | Options | Recommended |
|---|----------|---------|-------------|
| O1 | Where does connective prose live? Rules carry paragraphs that are not instructions (the "two readings, and the one that governs" passages) | a `note` point kind, or a per-rule `_intro.md` | `note` kind, so ordering lives in one place |
| O2 | How is reading order held? | `NNN-` numeric prefix like `plan/learned/`, or a per-rule manifest file | numeric prefix, in tens (010, 020, 030) so an insert needs no renumber |
| O3 | Do `TRIGGERS.md` / `CORE.md` change? | regenerate from points, or keep reading the rendered rule files | keep reading the rendered files. This work then lands invisible to every session, and the payload cannot regress |

## FILES ALREADY HANDLED (do not re-read, do not re-measure)

| File | What was established |
|------|----------------------|
| the retired `scripts/dev/rules_condensed.py` (current producer: `internal/le/`) | `ARTIFACTS` emits exactly two markdown files, `TRIGGERS.md` and `CORE.md`. No JSON path exists. `condense_body` keeps lines by SHAPE and assigns no identity to any obligation. `load_rules` already builds the per-rule structured dict the splitter needs |
| the retired `scripts/dev/rules_lint.py` (current producer: `internal/le/`) | `CANON_KEYS` is closed at `When`/`Severity`/`Related`; a fourth key is a violation. `check_rule` and `check_trigger` validate the title, metadata block and trigger line. Directive TEXT is never validated |
| `ai/rules/repo-maintenance.md` | The "Hook-to-Rule Mapping" section already records which RULE each check enforces, in prose. Nothing parses it. It is the seed for STEP 4 |
| `.claude/hooks/pretool-writeedit.py` | 44 `c_*` check functions. `pretool-bash.py` and `pretool-agent-skill.py` hold the rest |
| the retired `scripts/dev/rfc_requirements.py` (current producer: `internal/le/rfc/rfc.go`) | The model to copy. `parse_checklist_line` (authored id, derived everything else), `scan_go_tags` (bind a requirement to a test by comment), `evaluate` (the join), `check_coverage_ratchet` (monotonic proof), `verdict_freshness` (text moved under a recorded verdict) |
| Corpus measurements | 27 rules, 11,015 lines. 3,099 directive lines: 754 bullets, 422 bold, 1,923 table rows in 293 tables. Session payload 18,982 tokens against a 40,000 budget, of which `CORE.md` is 11,818. Router over 1,042 past tasks: mean 2.5 rules surfaced, 3 blocking rules surfaced by no task |

## STEP 1 -- point format and splitter

Create the retired `scripts/dev/rules_points.py` (current producer: `internal/le/rules/points.go`) with two subcommands, `split` and `render`.

Point path: `ai/rules/points/<rule-slug>/<NNN>-<slug>.md`

| Field | Authored or derived | Content |
|-------|--------------------|---------|
| path | authored | the id. `<rule>/<NNN>-<slug>` |
| `kind` | authored | `directive`, `table`, `note` |
| `level` | authored | `MUST`, `MUST NOT`, `SHOULD`, `MAY`, or none. Today the corpus spends 154 MUST, 20 MUST NOT, 4 SHOULD over 11,015 lines, so most points will start with none. That gap is a finding to report, not a blocker |
| `stage` | authored, optional | `design`, `implement`, `review`, `commit`, `runtime`. Leave empty in this work; it is what later lets a design-phase subagent skip implementation directives |
| body | authored | the instruction verbatim, byte-for-byte as it appears in the rule today |

No hash field. Change detection is `git log` on the point file.

Tests in the retired `scripts/dev/rules_points_test.py` (current producer: `internal/le/`).

## STEP 2 -- round-trip gate (BLOCKING, before any real split lands)

`./le rules points-roundtrip-check`: for each of the 27 rules, split into points in
a scratch directory, render back, diff against the committed file. All 27 must
be byte-identical. Only when that is green does the split get committed.

This is the step that decides whether the whole idea is viable. If tables or
nested lists cannot round-trip, stop and report rather than accepting a lossy
split.

## STEP 3 -- flip canonical

| Change | File |
|--------|------|
| `ai/rules/<rule>.md` rendered by `./le rules render-update` | the retired `Makefile` (current producers: `internal/le/` native action tables), the retired `mk/` (current producer: `internal/le/`) |
| Added to the regen set and its freshness gate | `./le repository generate`, `ze-generated-files-reconcile` |
| An edit to a rendered rule is refused and names the point file | `c_generated_files` in `.claude/hooks/pretool-writeedit.py` |
| The sync-direction row | `ai/rules/repo-maintenance.md`, "Canonical Sources and Sync Direction" |

## STEP 4 -- bind the gates (the payload)

Each check gets one comment above it naming the points it enforces:
`# ze point: performance/040-no-sprintf`

`./le rules coverage-report` joins checks to points and prints three sets:

| Set | Exit |
|-----|------|
| gated: points with at least one check | 0 |
| ungated: points with none | 0. This is the measurement, never a red |
| dangling: a check naming a point that does not exist | non-zero. Deterministic, one-line fix, and it is the signal that a rule moved under its gate |

Seed the work from the `Enforces` column of the Hook-to-Rule Mapping table,
which already answers this at rule granularity. The job is narrowing each answer
from file to point, roughly an afternoon over 44 checks.

## STEP 5 -- the mapping table becomes generated

Render the Check and Enforces columns of "Hook-to-Rule Mapping" from the binding
comments. This retires a hand-maintained copy that is ALREADY stale, which is
the argument for doing it: see "Also fix while in here" below.

## Discovery updates owed (`ai/rules/repo-maintenance.md` requires these in the same work)

- `ai/INDEX.md`: Dev Tools rows for `./le rules points-roundtrip-check`, `./le rules render-update`, `ze-rules-coverage`
- `ai/rules/rule-format.md`: rewritten for the point format. It currently describes the single-file format this work replaces
- `ai/rules/repo-maintenance.md`: the Canonical Sources row and the Hook-to-Rule Mapping note
- A `docs/contributing/` page owning rule authoring, if the format change is user-visible

## Risks

| # | Risk | Early signal | Mitigation |
|---|------|--------------|------------|
| R1 | The split loses content | STEP 2 diff is not empty | STEP 2 is byte-identical or the work does not land |
| R2 | About 1,469 files makes a rule harder to read at source | An author complains they cannot find an instruction | The rendered file still exists and is still what agents Read. Only authors touch points |
| R3 | Concurrent sessions editing rules | Merge conflicts | This IMPROVES on today: one writer per file is the same property that made `plan/deferrals/` sharded. Today any two rule edits collide in one file |
| R4 | Insert churn renumbers a rule's points | A large rename diff | Number in tens (O2). An insert takes the gap |
| R5 | Most points start with no RFC 2119 level, so `level` carries little | The level field is empty on most points after the split | Report the count. Classifying the corpus is separate work and is not in scope here |

## THEN

```
./le rules points-roundtrip-check && ./le rules coverage-report && ./le doc check verify
```

## Already done 2026-08-06, no action owed

- `./le repository generate`. The generated `CLAUDE.md` was stale and its dispatch table
  named 32 rule paths that no longer exist after the consolidation to 27 rules.
  The canonical `ai/INSTRUCTIONS.md` was clean, so the regeneration was the whole
  fix.
- `ai/rules/repo-maintenance.md`: two stale claims corrected. It said
  the retired `scripts/dev/rules_condensed.py` (current producer: `internal/le/`) "generates THREE artifacts" (`ARTIFACTS`
  emits two; `CONDENSED.md` was deleted) and that `TRIGGERS.md` carries "All 97"
  rules (it carries 27). The rule-count copy is now a pointer to the generator's
  own printed count rather than a second number that can drift again.
  `./le rules condensed-update` produced no digest change, since the rule's
  `**When:**` trigger is what reaches `TRIGGERS.md` and it did not move.
  `./le rules lint` passes over all 27.

Both are uncommitted in the working tree at the time of writing.
