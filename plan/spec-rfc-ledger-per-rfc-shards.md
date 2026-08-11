# Spec: rfc-ledger-per-rfc-shards

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | - |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/RFC-REQUIREMENTS.md` is 1,072,928 bytes over 6,206 lines. The head is 552 lines
(34,633 bytes): the preamble, "Evidence kinds", the "Coverage by RFC" rollup, "Audit
coverage" and "Extraction sign-off". The other 97 percent is 177 sections, one per RFC
stem, each a table of that RFC's requirement rows.

A reader who wants one RFC's coverage must open one megabyte. A session that moves one
tagged test rewrites the whole file, and several sessions share this checkout, so the
same file is rewritten by more than one of them at a time.

Goal: keep `ai/RFC-REQUIREMENTS.md` as the index, and write each RFC's requirement table
to its own file under `rfc/requirements/`. Add a `--show` mode so a caller names a stem
instead of a path. No requirement text, no test link, and no count changes value.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - governs the line numbers this ledger carries
  → Constraint: a line number in a document is allowed only where a generator maintains it, and a file earns that by declaring `GENERATED ... do not edit` in its first ten lines. Each shard must carry that banner.
  → Decision: the rule's own text names `ai/RFC-REQUIREMENTS.md` as the worked example of derived `file.go:line` entries. After the split those entries live in the shards, so the rule point that states it must be corrected.
- [ ] `ai/rules/repo-maintenance.md` - discovery surfaces
  → Constraint: two discovery rows send a reader to `ai/RFC-REQUIREMENTS.md` for per-requirement test coverage. Both must name the shard path, or the discovery surface points at a file that no longer holds the answer.
- [ ] `ai/rules/simplicity.md` - shape of the change
  → Constraint: the split cuts a file in two along a seam that already exists. It adds no abstraction, no option and no second source of truth.

### RFC Summaries (Scope: protocol)
Not applicable. This spec changes no protocol code and no RFC obligation.

**Key insights:** (minimal context to resume after compaction)
- The seam already exists in the renderer: the head sections are separate helpers, the per-RFC body is one inline loop.
- Only one file reads the ledger's bytes for meaning: `scripts/dev/testing_health.py`, and it reads the rollup alone.
- Whole-file freshness becomes many-file freshness. That is where the risk sits, not in the consumers.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/rfc_requirements.py` - `render_ledger` returns one string. It emits the title and preamble, then `_render_evidence_legend`, `_render_rollup`, `_render_audit_coverage` and `render_extraction_table`; then it builds the per-requirement audit verdicts and emits the 177 `## <STEM>` sections in an inline loop with no helper; then `_render_status_backlog` and the "Summaries declaring no MUST-level requirement" table. `run_write` collects once through `_collect_for_check`, renders, and writes `LEDGER_FILE`. `check_ledger_fresh` re-renders and compares the whole string, returning one message. `main` matches flags by membership in the argument list, and only `--extract-skeleton` reads a value, by index.
- [ ] `scripts/dev/testing_health.py` - `collect_rfc` reads the ledger as one blob and matches `RFC_ROW` against the rollup table. It reads nothing else from the file, and it fails closed when the header moves, when no row parses, when no row carries the enrolled marker, or when the gated total is zero.
- [ ] `scripts/dev/check_doc_links.py` - `sweep_tracked` reads every tracked file and checks each cited path exists. Its only exclusions are `SWEEP_EXCLUDE` and `CITATION_EXCLUDE_PREFIXES`, and it has no banner exemption. `check_baseline_growth` refuses any baseline pair HEAD does not hold, so a new dead citation cannot be absorbed.
- [ ] `scripts/dev/ste_check.py` - `EXCLUDE_DIRS` begins with the `rfc/` prefix, applied by `excluded` at every entry point. The writing checker reads nothing under `rfc/`.
- [ ] `scripts/dev/line_refs.py` - `targets` walks `ROOTS`, which is `ai`, `docs`, `plan` and `.claude`. The `rfc/` tree is outside the sweep.
- [ ] `.claude/hooks/pretool-writeedit.py` - `_in_prose_root` returns false for a file declaring `GENERATED ... do not edit`, and its roots exclude `rfc/`. The hook cannot block a shard.
- [ ] `scripts/dev/verify_wiring_docs.py` - `is_doc_source` fires for four named scripts, the `Makefile`, the `mk/` tree, and `docs/` markdown carrying a source anchor. Neither the ledger nor a shard selects a target.
- [ ] `scripts/dev/rfc_requirements_gate_test.go` - `TestRFCLedgerFresh` shells `--check-fresh` and fails on a non-zero exit. It asserts nothing about the file's shape.
- [ ] `Makefile` - `ze-rfc-index` runs the writer, and its comment names one output file. `ze-rfc-check` runs the selftest and the full check.

**Behavior to preserve:**
- The rollup table's header and row shape, exactly as `testing_health.py` pins them.
- Every requirement row's text, including its evidence kind and tier, its audit marker, its nightly-only marker and its annotation.
- The stable sort of requirements by id, and of test links by file then line, which is what makes the render byte-stable across machines.
- `make ze-rfc-check` and `make ze-doc-test` staying the gates that catch a stale ledger.
- The header counts the weekly update reads.

**Behavior to change:**
- The per-RFC sections leave `ai/RFC-REQUIREMENTS.md` and become one file per stem under `rfc/requirements/`.
- `_render_status_backlog` and the "Summaries declaring no MUST-level requirement" table move up, because the body they used to follow is gone.
- Freshness compares the index and every shard, and refuses a shard the render did not produce.
- `run_write` deletes a shard whose stem no longer renders.
- A `--show` mode prints one shard.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-rfc-index` for the write path, `make ze-doc-test` and `make ze-rfc-check` for the freshness path, and a human or agent naming a stem to `--show`.
- Input format: the summaries in `rfc/short/`, the `RFC requirement:` tags scanned from the test tree, `rfc/enrolled.txt`, `rfc/extraction/`, `rfc/audit/` and `docs/features/rfc-status.md`.

### Transformation Path
1. `_collect_for_check` parses every summary and scans the test tree once. Unchanged.
2. The render splits: an index render for the head sections, and a per-stem render for one RFC's table. Both read the collection already in memory.
3. `run_write` writes the index, writes one file per rendered stem, and deletes any other markdown file in the shard directory.
4. `check_ledger_fresh` renders the same set and compares the index and every shard, then reports any shard on disk that the render did not produce.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Generator ↔ health page | The rollup table stays in the index, header and row shape untouched | No |
| Generator ↔ doc-links gate | Every path cited in a shard must exist when the shard is written | No |
| Generator ↔ git | 177 new tracked files, one deleted megabyte, in one commit | No |

### Integration Points
- `scripts/dev/testing_health.py` `collect_rfc` - reads the rollup from the index, unchanged.
- `scripts/dev/rfc_requirements_gate_test.go` `TestRFCLedgerFresh` - follows whatever `check_ledger_fresh` compares, unchanged.
- `mk/inventory.mk` `ze-doc-test` - already runs the freshness mode, unchanged.

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
| A-1 | No consumer parses the per-RFC sections | Tree-wide search for the ledger path: every other hit names it in prose, and `collect_rfc` reads the rollup alone | A silent consumer reads an empty file after the split | Repeat the search over the whole tree, then run `make ze-verify` | confirmed |
| A-2 | The shard directory is invisible to the summary scanner | `rfc_requirements.py` discovers summaries through `SUMMARY_DIR`, which is the `rfc/short` tree, and `scan_tree` walks the test roots | The generator reads its own output and requirement counts double | A test that runs the write twice and asserts the counts are equal | confirmed |
| A-3 | The 177 shards add no writing-checker or line-reference cost | `ste_check.py` `EXCLUDE_DIRS` starts with the `rfc/` prefix; `line_refs.py` `targets` walks four roots that exclude it | Every shard enters a prose sweep it was never written for | Run `make ze-doc-test` and the writing checker over the new tree | confirmed |
| A-4 | `commit_helper.py` can express a 178-file commit | It takes one file flag per path and rejects ignored paths; nothing caps the count | The commit cannot be prepared through the helper | Prepare the script and read what it prints before running it | confirmed |

A-1 confirmed: the tree-wide search returns one programmatic reader, `testing_health.py`
`collect_rfc`, which matches `RFC_ROW` against the rollup; `line_refs.py` `GENERATED` names
the ledger to exempt it from the line-number sweep; `rfc_requirements_gate_test.go`
`TestRFCLedgerFresh` shells the mode. Every other hit is prose.
A-2 confirmed: `summary_stems` lists `SUMMARY_DIR` (`rfc/short`) alone, `scan_tree` walks
`TEST_ROOTS` (`internal`, `pkg`, `test`), and the other `rfc/` readers name `rfc/full`,
`rfc/drafts`, `rfc/audit` and `rfc/extraction`. None reaches `rfc/requirements`.
A-3 confirmed: `ste_check.py` `EXCLUDE_DIRS` opens with `rfc/`, applied by `excluded`;
`line_refs.py` `ROOTS` is `ai`, `docs`, `plan`, `.claude`.
A-4 confirmed: `commit_helper.py` `main` reads `args.file` as an appended list with no cap.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Freshness silently weakens: an edited shard is caught, an ORPHAN shard is not, and a deleted RFC leaves a stale page that reads as current | A shard whose stem is absent from the summaries survives a write | Orphan detection is an acceptance criterion, with its own test, and the writer prunes rather than only reporting |
| R-2 | The migration loses or reorders a requirement row, and the loss is invisible inside a diff that deletes a megabyte | The rendered rows differ from the rows the old ledger held | A migration test asserts every requirement id in the pre-split ledger appears in exactly one shard with identical row text |
| R-3 | Another session regenerates the ledger while the split lands. `plan/journal/concurrent-rfc-gate-stale.md` records this happening twice on 2026-07-30 | `make ze-rfc-check` reports the ledger stale for edits this session did not make | Regenerate immediately before preparing the commit script, and attribute any red before writing an unverified reason |
| R-4 | A shard cites a test path that no longer exists, and check 5 of the doc-links gate fails on a file nobody edited by hand | `make ze-doc-links` names a shard as the citer | The condition is unchanged from today: the same citations already sit in the swept ledger. Regenerating fixes it, as it does now |
| R-5 | The index cites the shard path with a placeholder, and the doc-links sweep reads the placeholder as a dead path | `make ze-doc-links` fails on the index | The index names a real shard as its example, never a placeholder |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. A wrong landing costs the RFC evidence ledger its freshness guarantee, which is what tells a reader the coverage claim is current |
| How is it reverted? | Single commit revert, then `make ze-rfc-index` |
| Who else touches this path? | Every session that moves a tagged test regenerates this file. `plan/spec-followup-rfc-enrollment.md` and `plan/spec-fixit-rfc-row-level-and-anchor-drift.md` both read the rollup as their work list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-index` | → | `run_write` writes the index and one file per stem | `TestShardWrite.test_write_emits_one_file_per_stem` |
| `make ze-doc-test` | → | `run_check_fresh` compares index and shards | `TestShardFreshness.test_edited_shard_is_stale` |
| The `--show` mode with a stem | → | `run_show` prints one shard | `TestShardShow.test_show_prints_shard` |
| `make ze-verify` | → | `TestRFCLedgerFresh` shells the freshness mode | `TestRFCLedgerFresh` (existing test, unchanged) |

N/A for `.ci` coverage: this spec changes no daemon code, so its functional surface is
the Python driving test named above.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-rfc-index` on a clean tree | `ai/RFC-REQUIREMENTS.md` holds no per-RFC requirement section, and one file exists under `rfc/requirements/` for every stem that renders a section |
| AC-2 | The index after a write | It holds the preamble, "Evidence kinds", the rollup, "Audit coverage", "Extraction sign-off", the status backlog and the no-MUST-summary table, and is under 60 KB |
| AC-3 | `collect_rfc` in `testing_health.py` run against the new index | It parses the same RFC count, gated count and enrolled count as it does against the pre-split ledger |
| AC-4 | Any shard's first ten lines | They declare `GENERATED` and `do not edit`, and name `make ze-rfc-index` as the producer |
| AC-5 | One byte edited in one shard, then the freshness mode | Exit is non-zero and the message names that shard's path |
| AC-6 | A markdown file placed in the shard directory whose stem renders no section, then the freshness mode | Exit is non-zero and the message names the orphan |
| AC-7 | The same orphan present, then `make ze-rfc-index` | The orphan file is deleted and the exit is zero |
| AC-8 | The `--show` mode given a stem in lower case, then the same stem in upper case | Both print that stem's shard to standard output and exit zero |
| AC-9 | The `--show` mode with no stem, or with a stem that has no shard | Exit is 2 and the message names `make ze-rfc-index` |
| AC-10 | Every requirement id in the pre-split ledger | It appears in exactly one shard, and its row text is identical to the row the old ledger held |
| AC-11 | `make ze-rfc-check`, `make ze-doc-test`, `make ze-doc-links`, `make ze-test-pkg PKG=./scripts/dev` | All pass |
| AC-12 | Two consecutive `make ze-rfc-index` runs | The second writes identical bytes and deletes nothing, proving the shard directory is not read as an input |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIndexRender.test_index_has_no_per_rfc_section` | `scripts/dev/rfc_requirements_test.py` | AC-1: the head render emits no per-RFC section | |
| `TestIndexRender.test_rollup_header_unchanged` | `scripts/dev/rfc_requirements_test.py` | AC-3: the rollup header and row shape the health page pins are untouched | |
| `TestShardWrite.test_write_emits_one_file_per_stem` | `scripts/dev/rfc_requirements_test.py` | AC-1: one shard per rendered stem | |
| `TestShardWrite.test_write_prunes_orphan_shard` | `scripts/dev/rfc_requirements_test.py` | AC-7: the write deletes a shard the render did not produce | |
| `TestShardWrite.test_second_write_is_a_no_op` | `scripts/dev/rfc_requirements_test.py` | AC-12: the shard directory is not an input | |
| `TestShardBanner.test_shard_declares_generated` | `scripts/dev/rfc_requirements_test.py` | AC-4: the banner sits in the first ten lines | |
| `TestShardFreshness.test_edited_shard_is_stale` | `scripts/dev/rfc_requirements_test.py` | AC-5: a one-byte edit fails the gate | |
| `TestShardFreshness.test_missing_shard_is_stale` | `scripts/dev/rfc_requirements_test.py` | AC-5: a deleted shard fails the gate | |
| `TestShardFreshness.test_orphan_shard_is_stale` | `scripts/dev/rfc_requirements_test.py` | AC-6: an extra shard fails the gate | |
| `TestShardShow.test_show_prints_shard` | `scripts/dev/rfc_requirements_test.py` | AC-8: the stem resolves to its file | |
| `TestShardShow.test_show_accepts_uppercase_stem` | `scripts/dev/rfc_requirements_test.py` | AC-8: the stem is matched without regard to case | |
| `TestShardShow.test_show_unknown_stem_exits_two` | `scripts/dev/rfc_requirements_test.py` | AC-9: the failure names the regenerate command | |
| `TestShardShow.test_show_refuses_a_separator_in_the_stem` | `scripts/dev/rfc_requirements_test.py` | Security row: a stem cannot reach outside the shard directory | |
| `TestShardMigration.test_every_requirement_row_survives` | `scripts/dev/rfc_requirements_test.py` | AC-10: no row is lost or altered by the split | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Rendered stems per write | 0 to the summary count | 177 | N/A | N/A |
| Requirement rows in one shard | 1 to the largest RFC | 65 rows, the count `rfc1661` renders | 0 rows renders no shard | N/A |

The zero case is the one that matters: a summary declaring no MUST-level requirement
renders no section today, so it must render no shard, and the orphan rule must not then
delete a file it never wrote.

### AC-10 migration proof (one-time measurement, 2026-08-11)

The pre-split ledger was captured to this session's scratch before the write path changed.
Scratch is not repeatable, so this comparison cannot become a test. It ran once, and these
are its numbers.

| Comparison | Result |
|------------|--------|
| 1. The captured pre-split ledger against the shards the current generator renders | 177 stems and 4,702 requirement rows on each side. 0 ids lost, 0 gained, 0 in more than one shard, 0 in a different stem. 20 rows differ in text |
| 2. HEAD `render_ledger` against the current `render_shards`, both over the tree as it stands | 4,702 rows on each side, 0 differences of any kind |
| 3. The captured pre-split ledger against HEAD `render_ledger` over the same tree | the same 20 rows differ |

Comparison 2 holds the inputs fixed and varies only the renderer. It is the measurement of
the split, and it says the split changed no row. Comparison 3 holds the renderer fixed and
varies the tree. It accounts for every difference comparison 1 reports. The 20 rows are
input drift since the capture on 2026-08-10 at 22:23. 16 belong to `rfc4552`, whose
annotation reasons lost their line numbers in commit 7ec29b6e6. 4 belong to `rfc2661`,
whose tagged L2TP tests moved lines in another session's uncommitted work.

**No requirement row changed in the migration.**

Method, for anyone who repeats it. Parse the per-RFC sections of the captured ledger. Render
the shards from the same `_collect_for_check` the gate uses. Compare row text by requirement
id. The HEAD renderer is loaded from `git show HEAD:scripts/dev/rfc_requirements.py` with its
`_HERE` pinned to `scripts/dev`, so both renderers read one tree.

The permanent half of AC-10 is `TestShardMigration.test_every_requirement_row_survives`,
which pins the row text as a literal and asserts a requirement id renders in exactly one
shard. It was driven against three mutations of `render_shards` (the evidence label dropped,
every shard carrying every stem's rows, one row deleted) and it fails on each.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestShardShow.test_show_prints_shard` | `scripts/dev/rfc_requirements_test.py` | An agent asks for one RFC's coverage and reads six kilobytes instead of one megabyte | |
| `TestRFCLedgerFresh` | `scripts/dev/rfc_requirements_gate_test.go` | A session moves a tagged test, forgets to regenerate, and the build refuses the commit | |

### Interop Tests (Scope: protocol)
Not applicable. This spec changes no wire-visible behavior.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - split the render, write and prune the shard directory, compare index and shards for freshness, add the `--show` mode, and add its line to the module usage text
- `scripts/dev/rfc_requirements_test.py` - retarget the ledger tests that patch the output path or search the rendered body, and add the tests above
- `ai/RFC-REQUIREMENTS.md` - regenerated as the index
- `Makefile` - the `ze-rfc-index` comment names one output file; it must name the index and the shard directory
- `ai/rules/points/evidence/no-fabrication/cite-a-line-number-only-when-a-generator-maintains-it.md` - the worked example moves to the shards
- `ai/rules/points/repo-maintenance/discovery-updates/the-discovery-surface-that-answers-each-need.md` - both discovery rows name the shard path for per-requirement coverage
- `ai/rules/points/testing/rfc-tagged-tests-blocking/what-to-do-in-each-tagged-test-situation.md` - the row that says a re-tagged test means regenerating and committing the ledger in the same commit. The `file:line` records move to the shards, so a session following that row commits the index alone and leaves the gate red. Found while mapping the prose surfaces, and it is the same defect as the two rule points above, so it is fixed here rather than specced separately
- `ai/skills/ze-rfc.md` - the rows that send a reader to the ledger for one requirement's tests
- `ai/skills/ze-rfc-audit.md` - step 4 tells a reader to open the `$ARGUMENTS` section of the ledger, which is now that stem's shard. The audit-coverage row beside it stays: that section stays in the index
- `ai/INDEX.md` - the RFC coverage row
- `.claude/hooks/pretool-writeedit.py` - the comment and the fix message that cite the ledger as the worked example
- `scripts/dev/line_refs.py` - the comment on its generated-file entry, for the same reason
- `rfc/extraction/README.md`, `docs/contributing/rfc-implementation-guide.md` - where they name the ledger as the place a per-requirement answer is read

## Files to Create
- `rfc/requirements/` - one generated markdown file per RFC stem, written by `make ze-rfc-index` <!-- doc-links: ignore (destination to create) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface: this is a developer tool |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | This is a developer script, not a `ze` subcommand. `ai/rules/cli.md` governs the daemon CLI |
| CLI grammar (keyword before value) | Yes | The new mode takes its stem the way `--extract-skeleton` does, the existing value-taking flag in `main` |
| Editor autocomplete | N-A | Not a config leaf |
| Functional test for new RPC/API | Yes | `scripts/dev/rfc_requirements_test.py`, the driving surface for this tool |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency: no path the daemon reads, no socket, no port. The shard directory is read by a developer script and by a human |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Developer tooling, not a product feature |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | N-A | No `ze` command |
| 4 | API/RPC added/changed? | N-A | No API |
| 5 | Plugin added/changed? | N-A | No plugin |
| 6 | Has a user guide page? | N-A | No operator-facing surface |
| 7 | Wire format changed? | N-A | No wire change |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement changes state. The rows move file, and AC-10 proves their text is identical |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` where it names the ledger |
| 11 | Affects daemon comparison? | N-A | No daemon behavior |
| 12 | Internal architecture changed? | No | The renderer's shape changes, its inputs and outputs do not |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Search `docs/` for a source anchor naming `scripts/dev/rfc_requirements.py` and correct each claim about one output file |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `make ze-rfc-index` example that names the output; correct each to the index plus the shard directory |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the write path reaches the shard directory
   - Tests: `TestShardWrite.test_write_emits_one_file_per_stem`
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/rfc_requirements_test.py`
   - Verify: the test fails because the write still produces one file
2. **Phase: Split the render** - a head render and a per-stem render, from the collection already in memory
   - Tests: `TestIndexRender.test_index_has_no_per_rfc_section`, `TestIndexRender.test_rollup_header_unchanged`, `TestShardBanner.test_shard_declares_generated`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: the status backlog and the no-MUST-summary table sit in the index, and the rollup is untouched
3. **Phase: Write and prune** - write the index and every shard, delete what the render did not produce
   - Tests: `TestShardWrite.test_write_prunes_orphan_shard`, `TestShardWrite.test_second_write_is_a_no_op`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: a second run writes identical bytes and deletes nothing
4. **Phase: Freshness over many files** - compare the index and every shard, and refuse an orphan
   - Tests: `TestShardFreshness.test_edited_shard_is_stale`, `TestShardFreshness.test_missing_shard_is_stale`, `TestShardFreshness.test_orphan_shard_is_stale`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: each of the three states fails the gate with a message naming the file
5. **Phase: The show mode** - one stem in, one shard out
   - Tests: `TestShardShow.test_show_prints_shard`, `TestShardShow.test_show_accepts_uppercase_stem`, `TestShardShow.test_show_unknown_stem_exits_two`, `TestShardShow.test_show_refuses_a_separator_in_the_stem`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: the usage text names the mode
6. **Phase: Migration proof** - the rows survive the move
   - Tests: `TestShardMigration.test_every_requirement_row_survives`
   - Files: `scripts/dev/rfc_requirements_test.py`
   - Verify: run against the pre-split ledger captured before step 3 writes over it
7. **Phase: Regenerate and correct the prose** - run `make ze-rfc-index`, then fix every surface that names the ledger as the file holding a per-requirement answer. Rule points are authored under `ai/rules/points/`, so render and re-condense them (`make ze-rules-render`, `make ze-rules-condensed`, `make ze-rules-lint`) and commit the digests with the point
   - Tests: `make ze-doc-test`, `make ze-doc-links`, `make ze-rules-lint`
   - Files: the prose files listed under Files to Modify
   - Verify: no gate names a dead citation, and no rule sends a reader to a file that no longer holds the answer

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Correctness | The orphan rule deletes only what the generator owns, and never a summary that legitimately renders no section |
| Correctness | The freshness comparison covers the index, every shard, and the set of files present, not only the ones the render produced |
| Data flow | The shard directory is written, never read as an input (AC-12) |
| Naming | A shard's filename is the summary's stem, so each summary in `rfc/short/` pairs with one file of the same name in the shard directory |
| Rule: `ai/rules/evidence.md` | Every shard declares `GENERATED ... do not edit` in its first ten lines |
| Rule: `ai/rules/simplicity.md` | No option, flag or configuration is added beyond the show mode, which the owner asked for |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One shard per rendered stem | Count the files in the shard directory against the section count of the pre-split ledger |
| An index a session can afford to read | Measure `ai/RFC-REQUIREMENTS.md`, which must be under 60 KB |
| Freshness that catches all three staleness states | The three freshness tests pass |
| The health page still reads the rollup | `make ze-test-pkg PKG=./scripts/dev` passes |
| No prose sends a reader to the wrong file | Search the tree for the ledger path and read each hit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The show mode takes a stem from the command line and turns it into a path. It must reject a stem carrying a separator or a parent reference, and resolve only inside the shard directory |
| Destructive operation | The prune deletes files. It must delete only markdown files directly inside the shard directory, never a subdirectory and never a path the render did not own |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Freshness gate red for another session's edits | Attribute the red before acting: regenerate, then read what changed |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The renderer already had the seam. The head sections are separate helpers and the per-RFC body is one inline loop, so the split is a change of assembly, not a rewrite.
- Placement decided the gate surface, not the file count. Under the `rfc/` tree the writing checker and both line-reference sweeps skip the shards; under `ai/` all three would have applied and each would have needed an exemption.
- A whole-file comparison carries deletion detection for free. Many files do not. That property, not the file count, is what this change has to re-earn.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shard per RFC stem | One shard per protocol family; a JSON file with no per-RFC markdown | A family boundary is a judgement nobody maintains, and its shards stay too big to read for one requirement. JSON reads well for a script and badly for the gap notes, which are prose written for a reader |
| Shards under `rfc/requirements/` | A directory under `ai/` | The derived page sits beside its authored source in `rfc/short/`, and the `rfc/` tree is outside the writing checker and both line-reference sweeps. Under `ai/` each of those needs an exemption, and an exemption is a thing to maintain |
| The show mode reads the shard from disk | Re-render the stem on every call | Reading is instant, which is the reason the mode exists. Freshness stays the gate's job, and the gate already runs in `ze-doc-test` and `ze-rfc-check` |
| The rollup keeps a bare stem in its RFC column | Make the cell a link to the shard | The health page matches the cell with a pinned pattern and fails closed. A link changes the cell and buys a reader nothing the path convention does not already give |
| The index names a real shard as its example | A placeholder path with an angle-bracket stem | The doc-links gate reads a cited path and requires it to exist, so a placeholder is a dead citation in a generated file |

## Known Limitations

- `verify_wiring_docs.py` selects no target for the ledger today, and selects none for a per-RFC file either. A change confined to those files does not re-run the freshness gate under `make ze-verify-changed`. This spec neither creates that gap nor closes it: the behavior is identical before and after, so nothing here is deferred.
- The landing commit deletes one megabyte and adds 177 files. The diff is large by construction. AC-10 is what makes it reviewable: it proves the rows are unchanged, so a reviewer reads the generator rather than the output.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol change)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

`scripts/dev/rfc_requirements.py` splits one render into two. `render_ledger` is deleted,
not wrapped. `render_index` emits the head sections. `render_shards` returns a stem to body
map, one entry per RFC that renders a requirement table. `shard_stems` is the one producer
of that set. `_prunable_shards` is the one producer of the orphan set, consumed by both
`prune_shards` and `check_ledger_fresh`, so the write and the gate cannot disagree about
what an orphan is. `run_write` writes the index, writes one file per stem under `SHARD_DIR`
(`rfc/requirements/`), then prunes. `check_ledger_fresh` compares the index, then each stem
for absence and for drift, then the orphans. `run_show` reads one file from disk and folds
case before `_validated_stem`.

The index fell from 1,073,168 bytes to 43,570.

### Bugs Found/Fixed

- **`run_write` deleted tracked files on a partial collection, and exited 0.**
  `_collect_for_check` catches a `ParseError` per summary and carries on, which is right
  for the gate. `run_write` discarded that slot, so a summary that failed to parse rendered
  nothing, `shard_stems` omitted its stem, and the prune removed that RFC's file. An absent
  `rfc/short/` did the same to every file at once. `run_write` now refuses, writing and
  deleting nothing, when a parse error is present or when the render produced no rows.
  Covered by `test_a_parse_error_refuses_the_write_and_deletes_nothing` and
  `test_an_empty_render_refuses_the_write_and_deletes_nothing`.
- **`_prunable_shards` deleted `README.md`, and its docstring promised it did not.** It
  filtered on the `.md` suffix alone, so an authored `README.md` beside the generated files
  was an orphan, and a bare `.md` yielded an empty stem. It now also requires the stem to
  match `_STEM_RE`, the predicate `--show` already uses.
- **`test_stale_extraction_table_fails_check_fresh` was vacuous.** It patched `LEDGER_FILE`
  alone, so the gate walked the real `rfc/requirements/` and 177 files read as orphans. Any
  non-empty error list satisfied it, and it passed with the extraction table stubbed out
  entirely. It now patches `SHARD_DIR` too, asserts a control case reads fresh, and asserts
  exactly one error, naming the index.
- **The `isdir` guard was pinned by nothing.** Its fixture directory was named `sub`,
  already excluded by the `.md` test. Delete the guard and every test stays green. The
  fixture is now `sub.md`, which pins it.
- **`_render_evidence_legend` wrote a sentence the split made false**: "Every test link
  below carries a `kind/tier` cell". No test link is below it. It names the per-RFC files
  now, and cites no path, because a placeholder path in a generated file is a dead citation
  to the doc-links gate (risk R-5).
- **`plan/learned/RECURRING-PATTERNS.md`** quoted a symptom string the gate no longer
  prints and gave a cause naming one global file. Corrected, and it now records that the
  split limits the blast radius to the RFCs whose tests moved.

### Documentation Updates

`ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md`, `docs/functional-tests.md`,
`docs/contributing/rfc-implementation-guide.md`, the `scripts/dev/line_refs.py` comment,
and three rule points with their rendered rules. Gate results are in Pre-Commit
Verification.

### Deviations from Plan

- **The index reached HEAD in another session's commit.** `7ec29b6e6`, an unrelated IPsec
  fix, absorbed the split `ai/RFC-REQUIREMENTS.md` while the 177 per-RFC files stayed
  untracked, so HEAD published an index citing files no commit provided. This commit
  carries those files, which is the repair. History is not rewritten.
- **Four prose surfaces are not in this commit**, because each holds another session's
  uncommitted work in the same file and `--file` stages whole files. `Makefile` and
  `ai/INDEX.md` would break HEAD outright: both name `scripts/dev/relax-census.py`, which
  HEAD does not hold. `.claude/hooks/pretool-writeedit.py` carries a 218-line rework in
  flight. `docs/features/rfc-status.md` carries an RFC 5301 enrolment whose
  `rfc/enrolled.txt` and `rfc/short/rfc5301.md` are also uncommitted, so landing the status
  row alone would put the page and the summaries out of agreement. The corrections are
  written and sit in the working tree for the session that owns those files.
- **The rendered rule digests were reconciled rather than committed as they stood.** They
  render from the whole points tree, which currently holds two sessions' points. Each
  rendered rule in this commit was produced by `rules_points.py render` over HEAD's points
  plus this spec's three point files, so `rules_points.py render --check` reproduces it at
  HEAD from HEAD's own sources. `TRIGGERS.md`, `CORE.md` and `INDEX.md` regenerated to
  HEAD-identical bytes and are therefore not in the commit.
- Files to Modify gained three rows during implementation, each the same defect at another
  call site: the tagged-test rule point, `ai/skills/ze-rfc-audit.md` step 4, and 29 rows of
  `docs/features/rfc-status.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Closure took `make ze-rules-lint` to be the gate that would go red if a rendered rule carried two sessions' points | `ze-rules-lint` checks only the `**When:**` / `**Severity:**` block. The gate at risk is `rules_points.py render --check`, which `ze-doc-test` runs | Read `mk/inventory.mk` and `scripts/dev/rules_lint.py` before acting on the assumption | Reconciled the render against HEAD's points, so HEAD reproduces it from its own sources |
| assumption | The files regenerated at 09:04 were taken as current | Another session moved a tagged test during the closure, staling one per-RFC file | The real-tree tests in `rfc_requirements_test.py` went red mid-closure | Regenerated immediately before preparing the commit script, which is what risk R-3 prescribes |
| approach | The write was treated as a pure render, so `run_write` discarded the parse-error slot | The write PRUNES, so an incomplete collection is a destructive input rather than a partial one | Independent review round 1 | Guarded at the entry point, fail-closed, with two tests driven from `run_write` |
| approach | The first test for that guard was written from the guard's CODE, so its fixture rendered nothing and both refusal branches fired at once | A test that cannot separate two guards pins neither. A weaker `parse_errs and not shards` would have kept it green | Independent review round 2 | Rebuilt the fixture from the DEFECT: many summaries render, one fails, and the file the prune would have deleted is asserted to survive |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Keep `ai/RFC-REQUIREMENTS.md` as the index | Done | `render_index` | 43,570 bytes |
| Write each RFC's table to its own file | Done | `render_shards`, `run_write` | 177 files under `rfc/requirements/` |
| Add a `--show` mode taking a stem | Done | `run_show`, `main` | Reads from disk, folds case before validating |
| No requirement text, test link or count changes value | Done | AC-10 measurement | 4,702 rows, 0 differences renderer to renderer |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestShardWrite.test_write_emits_one_file_per_stem`, `TestIndexRender.test_index_has_no_per_rfc_section` | 177 files; the index holds 7 `## ` headings, all head sections |
| AC-2 | Done | `stat` on the index | 43,570 bytes against a 61,440 limit |
| AC-3 | Done | `TestIndexRender.test_rollup_header_unchanged` | `collect_rfc` returns `1223 / 2963` and `36 / 169` against the new index |
| AC-4 | Done | `TestShardBanner.test_shard_declares_generated` | Banner on line 3, names `make ze-rfc-index` |
| AC-5 | Done | `TestShardFreshness.test_edited_shard_is_stale`, `test_missing_shard_is_stale` | The message carries the repo-relative path |
| AC-6 | Done | `TestShardFreshness.test_orphan_shard_is_stale` | |
| AC-7 | Done | `TestShardWrite.test_write_prunes_orphan_shard` | A non-markdown file, a `README.md`, a bare `.md` and a `sub.md` directory all survive |
| AC-8 | Done | `TestShardShow.test_show_prints_shard`, `test_show_accepts_uppercase_stem` | `--show RFC7606` exits 0 |
| AC-9 | Done | `TestShardShow.test_show_unknown_stem_exits_two` | An unknown stem and an absent stem both exit 2 |
| AC-10 | Done | `TestShardMigration.test_every_requirement_row_survives`, plus the one-time measurement above | 0 rows lost, gained, duplicated or moved |
| AC-11 | Partial | `ze-rfc-check`, `ze-doc-test`, `ze-test-pkg` pass; `ze-doc-links` red | Every `ze-doc-links` finding belongs to another session or is red at HEAD already. None names a file this work touched |
| AC-12 | Done | `TestShardWrite.test_second_write_is_a_no_op` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 14 named tests | pass | `scripts/dev/rfc_requirements_test.py` | 783 selftests, exit 0. Review round 1 added two more |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rfc_requirements.py` | Done | |
| `scripts/dev/rfc_requirements_test.py` | Done | |
| `ai/RFC-REQUIREMENTS.md` | Done | The split index reached HEAD in `7ec29b6e6`. This commit carries the legend correction |
| `Makefile`, `ai/INDEX.md`, `.claude/hooks/pretool-writeedit.py` | Changed | Edited, excluded from this commit. See Deviations |
| Three rule points, `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md`, `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `scripts/dev/line_refs.py` | Done | |
| `rfc/extraction/README.md` | Changed | Deliberately not edited. Both its sites name the extraction sign-off, which stayed in the index |
| `rfc/requirements/` | Done | 177 files |

### Audit Summary
- **Total items:** 12 acceptance criteria, 14 planned tests, 12 file rows
- **Done:** 11 acceptance criteria, 14 tests, 8 file rows
- **Partial:** AC-11, because `ze-doc-links` is red for findings this work did not produce
- **Skipped:** none
- **Changed:** 4 file rows, each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reader who wants one RFC's coverage no longer opens one megabyte | functional | `python3 scripts/dev/rfc_requirements.py --show RFC7606` prints 6,929 bytes and exits 0. The index is 43,570 bytes against 1,073,168 before |
| A session that moves one tagged test no longer rewrites the whole file | measured, live | An unrelated session moved a tagged test during this closure. The regeneration rewrote one file, `rfc/requirements/rfc4271.md`. Before the split the same one-line shift rewrote the entire ledger, which is the `concurrent-rfc-gate-stale` journal class |
| No requirement text, test link or count changes value | migration proof | 4,702 rows on each side, 0 lost, 0 gained, 0 duplicated, 0 in a different stem. HEAD `render_ledger` against the current `render_shards` over one tree: 0 differences |
| Freshness still catches staleness, now over many files | functional | Three tests, one per state: edited, missing, orphan. Each names the path and `make ze-rfc-index`. Proven live, by catching the foreign line shift above |
| The write never destroys what it does not own | negative tests | Four guards, each driven from `run_write` and each proven to discriminate by removing it: parse error, empty render, non-shard stem, subdirectory |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata declares no deferral shard, and `plan/deferrals/spec-rfc-ledger-per-rfc-shards.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-ledger-per-rfc-shards-4620e1aa-23ab-41c6-bb87-945c90f52cbe.md` |
| `review_gate.py check` | clean |
| Rounds | 3 |
| Reviewer lenses used | Round 1: logic, security, destructive operation and test discrimination over the generator; prose correctness, stale-pointer sweep, banner and STE over the documentation and the generated files. Round 2: re-judgement of every round-1 fix. Round 3: re-judgement of every round-2 fix |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `run_write` discarded the parse-error slot, so a summary that failed to parse let the prune delete that RFC's tracked file, exit 0. An absent `rfc/short/` deleted every file | `scripts/dev/rfc_requirements.py` `run_write` | Unpacked `parse_errs` and refused the run, before any write or delete, on a parse error or an empty render. Two tests driven from `run_write` |
| 2 | ISSUE | `_prunable_shards` deleted `README.md` and a bare `.md`, while its docstring promised a README survives | `scripts/dev/rfc_requirements.py` `_prunable_shards` | Required the stem to match `_STEM_RE`, and corrected the docstring to state the real rule |
| 3 | ISSUE | `test_stale_extraction_table_fails_check_fresh` patched `LEDGER_FILE` alone, so real-tree orphan noise satisfied it. It passed with the extraction table stubbed out | `scripts/dev/rfc_requirements_test.py` | Patched `SHARD_DIR` too, added a control case, and asserted exactly one error naming the index |
| 4 | ISSUE | The index legend read "Every test link below carries a `kind/tier` cell". Nothing is below it after the split | `scripts/dev/rfc_requirements.py` `_render_evidence_legend` | Named the per-RFC files, citing no path, then regenerated |
| 5 | ISSUE | `test_a_parse_error_refuses_the_write_and_deletes_nothing` did not model the defect it named. Its fixture rendered nothing, so the empty-render guard refused anyway and a weaker `parse_errs and not shards` guard would have kept it green | `scripts/dev/rfc_requirements_test.py` | The fixture now renders two stems, carries a parse error, and pre-seeds a file for a stem that does not render. Driven against the weaker guard, it fails |
| 6 | ISSUE | The `_prunable_shards` docstring justified leaving the render unguarded with "the convention is checked where summaries are authored". No such check exists, so the safety argument rested on a claim the code does not support | `scripts/dev/rfc_requirements.py` `_prunable_shards` | Said plainly that nothing enforces the convention, named the branches that still cover the case, and gave the real reason for leaving it: not-deleting is the safe direction |

NOTEs recorded and not fixed: `run_check_fresh` also discards `parse_errs`, so `ze-doc-test`
can name a command that then exits 2. It is self-terminating, because the write prints the
parse errors, and it loses nothing. It is the same discard this spec fixed at the
destructive call site, so its class is already journalled under
`discarded-error-becomes-destructive`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `rfc/requirements/` | yes | `ls rfc/requirements/*.md \| wc -l` prints 177 |
| `rfc/requirements/rfc7606.md` | yes | `head -6` shows the banner on line 3 |
| `ai/RFC-REQUIREMENTS.md` | yes | `stat -c%s` prints 43570 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | One file per rendered stem, no per-RFC section in the index | 177 files; `grep -c '^## ' ai/RFC-REQUIREMENTS.md` prints 7 |
| AC-2 | The index is under 60 KB | 43,570 bytes |
| AC-3 | The health page still parses the rollup | `testing_health.collect_rfc` returns `1223 / 2963` and `36 / 169` |
| AC-8, AC-9 | The show mode resolves and fails correctly | `--show RFC7606` exits 0; `--show nosuchstem` exits 2; `--show` with no stem exits 2 |
| AC-1 to AC-12 | Every named test passes | `python3 scripts/dev/rfc_requirements_test.py -q`: 783 tests, exit 0 |
| AC-11 | The gates pass | `make ze-rfc-check` exit 0: 783 selftests, 2963 gated MUST-level requirements across 170 enrolled RFCs, 3339 tags resolved. `make ze-doc-links` red, 17 findings, none naming a file this work touched |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-rfc-index` | n/a, a Python tool | Ran it. Wrote the index and 177 files, exit 0 |
| `make ze-doc-test`, `make ze-rfc-check` | n/a | `make ze-rfc-check` exit 0 |
| `--show <stem>` | n/a | Ran it against the real tree, exit 0 |
| `TestRFCLedgerFresh` | `scripts/dev/rfc_requirements_gate_test.go` | Passes inside `make ze-test-pkg PKG=./scripts/dev`, exit 0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `testing_health.py` `collect_rfc` is the one programmatic reader, reads the rollup alone, and still parses |
| A-2 | confirmed | `summary_stems` lists `SUMMARY_DIR` alone; `scan_tree` walks `internal`, `pkg`, `test`. `test_second_write_is_a_no_op` proves the directory is not an input |
| A-3 | confirmed | `ste_check.py` `EXCLUDE_DIRS` opens with `rfc/`; `line_refs.py` `ROOTS` excludes it. Neither sweep grew |
| A-4 | confirmed | `commit_helper.py` accepted the full `--file` list |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Item 10, test infrastructure | `docs/functional-tests.md`, three sites, each checked against `run_write` and `check_ledger_fresh` | yes |
| Item 16, source anchors naming the generator | An independent reviewer swept every `RFC-REQUIREMENTS` hit in the tree and judged each one | yes |
| Item 17, `make ze-rfc-index` examples | Every example names both outputs, except in the four files another session holds | partial, recorded in Deviations |
| Item 9, RFC behavior | No requirement changes state. AC-10 proves the row text is identical | yes |
| Sections that had to stay in the index | A reviewer confirmed "Extraction sign-off", "Audit coverage" and the header counts are still rendered by `render_index`, and that no pointer at them was repointed | yes |

## Core Insight

A whole-file comparison carries deletion detection for free. A many-file comparison does
not. That property, not the file count, is what a split has to re-earn, and building
`_prunable_shards` as the one producer consumed by both the write and the gate is what
makes the two incapable of disagreeing about an orphan.

The second half of that lesson is the one review found: a render that gains the power to
DELETE stops being a pure function of its inputs. It becomes a destructive operation, and a
destructive operation must fail closed on an input it cannot trust. The old code discarded
the parse-error slot safely for a decade, because a partial render only ever produced a
smaller file. The same discard, under a writer that prunes, deletes tracked evidence and
reports success.
