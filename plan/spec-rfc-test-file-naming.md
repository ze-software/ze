# Spec: rfc-test-file-naming

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-10-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Bucket: `plan/`. No operator meets this and the release ships without it (`plan/README.md`).

## Task

Make a Go test file's name and its `RFC requirement:` tags agree, in both directions, and enforce it.

| Part | Owner decision (2026-10-03) |
|------|-----------------------------|
| (a) | A test file named `rfcNNNN_<topic>_test.go` MUST carry at least one tag for RFC NNNN, or a fixed marker comment saying why it carries none |
| (b) | A test file whose tags cite exactly one RFC MUST be named `rfcNNNN_<topic>_test.go`: full convention, existing files renamed (chosen over grandfathering them) |
| (c) | Enforced by a check inside `./le rfc check` |
| (d) | A rule point states the convention |
| (e) | The tree is repaired so the check arms green: untagged RFC-named files, name/tag mismatches, and the renames |

The owner rejected mass tagging of the untagged files: each is renamed after what it covers or given the marker.

## Research (2026-10-03, session ff3776cb)

Counts measured by the research agent; lists in `tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/scratch/` (`rfc-rename-candidates.txt`, `rfc-naming-flags.txt`, and `naming.py`, the script that produced both).

| Set | Count |
|-----|-------|
| Single-stem tagged files not named for their stem | 509 (462 rfcN, 41 draft/sflow, 6 mismatches) |
| Multi-RFC tagged files | 183, out of scope for (b) |
| Untagged RFC-named files | 39 |
| Name/tag mismatches | 6 |
| Rename collisions needing a hand-chosen topic | 26 |

The candidates file has six tab-separated columns: path, RFC stem, proposed path, collision (`exists` or `dup-target`), discrimination records, audit verdicts.

## Required Reading

### Producers (read 2026-10-03)

- [ ] `internal/le/rfc/discriminate.go` `discriminationOwedTags`; `internal/le/rfc/check_baseline.go` `readCommittedTags`, `coversOfTags`
  -> Constraint: a cover is keyed by RID, polarity and `<path>::<Func>`; the HEAD^ baseline is built per path with no rename detection, so every gated tag in a renamed file reads as newly added and owes a discrimination record (about 787 covers have none). Renames are blocked until the baseline follows a git rename whose body is unchanged.
  → Constraint: `discriminationOwedTags` skips a cover that is in the baseline OR proven by a verifying record. A renamed cover WITH a record is therefore safe once the record's `unit` is rewritten; the baseline fix is needed for the covers with no record.
  → Constraint: `coversAt` is the one place a baseline cover set is minted, for both `priorRevision` (HEAD^) and `backlogRevision` (origin/main). Rename following goes there once, so both baselines get it.
- [ ] `internal/le/rfc/check_baseline.go` `baselineTaggedAt`, `gitCatBlobs`, `gitOutput`
  → Constraint: `baselineTaggedAt` lists paths with `git grep -l -F` over `testRoots` and keeps only what `CarrierFor` holds; every reader answers false when git cannot be read, and a false baseline accuses nobody. The rename map follows the same rule.
- [ ] `internal/le/rfc/freshness.go` `unitIdentity`, `keyFile`
  -> Constraint: audit verdicts are keyed (file, unit-sha), "the identity a RENAME does not preserve"; 808 verdicts in 117 audit files go STALE_UNIT on rename and `reseal` refuses them. Both hashes are path-independent, so rewriting the keys keeps them FRESH.
- [ ] `internal/le/rfc/audit.go` `VerdictRecord`
  → Constraint: a verdict carries three fingerprint maps, `tests`, `units` and `code`, each keyed `<path>::<Func>`. The JSON key for the requirement fingerprint is `requirement-sha`. All three maps are rewritten when their file part is the old path.
- [ ] `internal/le/rfc/discriminate.go` `DiscriminationRecord`, `unitKeyAt`, `behaviorSHA`, `removable`
  → Constraint: a record carries `unit`, `unit-sha`, `claim-sha`, `producer`, `producer-sha`, `break`. `behaviorSHA(path, text)` uses the path only to pick the comment grammar, so a `_test.go` to `_test.go` rename keeps `unit-sha`. A record whose unit is gone reads `removable`, not stale: a record left at the old path after a rename is reported for removal and its cover is owed again.
- [ ] `internal/le/rfc/check_ratchets.go` `discriminationWithdrawnErrors`
  → Constraint: a HEAD record that is gone while its tag stands is refused. After a rename the old-path cover has no tag in the tree, so the rewritten record does not trip it.
- [ ] `internal/le/rfc/check.go` `check`; `internal/le/rfc/check_core.go` `checkSuperseded`
  -> Constraint: checks are called in fixed order and appended to findings; the new check follows `checkSuperseded` as a whole-tree structural check. `scanDir` skips files without the literal tag, so part (a) needs its own walk of `rfc*_test.go` names.
  → Decision: `checkTestFileNames` is appended immediately after the `checkSuperseded` line in `check`, outside the `intersects(...)` block, so it runs on every tree.
- [ ] `internal/le/rfc/carriers.go` `ScanTree`, `scanDir`, `CarrierFor`, `skipDirs`
  → Constraint: `CarrierFor` refuses `test/draft/` and `internal/le/` (except `internal/le/interoplab/`), and `scanDir` skips `.git`, `vendor`, `testdata`. The naming check judges exactly that population, restricted to `_test.go` files with the `go` reader.
- [ ] `internal/le/rfc/tags.go` `goTagRE`, `parseTagRest`, `scanGoTags`
  → Constraint: `goTagRE` matches a `//` comment line holding `RFC requirement:`. The marker text MUST NOT contain that phrase.
- [ ] `internal/le/rfc/rfc.go` `Prefix`, `hasRIDStem`, `testRoots`, `tagMarker`
  -> Constraint: draft stems contain hyphens, so a tag maps to its stem through the requirement lookup, never a regex.
  → Decision: the tag to stem map is `Requirement.RID` to `Requirement.RFC` from `collected.Requirements`; a RID with no requirement falls back to `hasRIDStem` over the summary stems, and a tag that resolves to no stem is ignored by the naming check (another check already refuses an unknown id).
- [ ] `internal/le/rfc/reseal.go` `ResealWithProof`, `reseal`, staging
  → Constraint: the one writer of `rfc/audit/` stages a rewrite and swaps it in atomically; a concurrent run's staging directory is protected by age. The rename's JSON rewrites use the same atomic write.
- [ ] `internal/le/rfc/actions.go` action table, `resealAnswer`, `auditStampAnswer`
  → Constraint: the action table is the single source for dispatch, help, listings and grammar. A new verb is one `leaction.Action` entry with `Writes: true` and keyword `Parameters`; tests reach it through `Answer` with `ZE_REPO_ROOT` set (`actions_test.go`).
- [ ] `internal/le/go/module/rename.go` `Rename`, `applyRename`
  → Decision: precedent for a repository rename: plan first, preflight every move, then apply, then re-stamp audits through `rfc.ResealWithProof`. `./le rfc rename` does not need reseal, because it rewrites keys instead of re-stamping shas.
- [ ] `internal/le/commit/rfcchange.go` `rfcChangeProblems`, `internal/le/test/weakened/audit.go` `auditDiff`
  -> Constraint: both pair renames at similarity >= 50, so each rename MUST be byte-pure, landed with plain `mv` plus `remove` of the old path.
  → Constraint: `auditDiff` calls `rfc.ChangedTags(path, oldText, newText)` on the paired texts; identical texts report nothing, so a byte-pure rename needs no `RFC-approved:` trailer.
- [ ] `internal/le/doc/citation/citation.go` `Paths`, `internal/le/doc/check/links.go` `sweepTracked`
  → Constraint: the link sweep reads a backticked path out of ANY tracked file, Go comments included (`internal/le/rfc/render.go` `summaryRelOf` says so). The rename reuses this grammar instead of writing a second one. `go list -deps ./internal/le/doc/check` holds no `internal/le/rfc`, so `internal/le/rfc` may import it without a cycle.
- [ ] `internal/le/hookruntime/lifecycle.go` `validateSpecText`
  → Constraint: the sections this file must carry.

### Docs and rules

- [ ] `docs/contributing/rfc-conformance-gates.md` - the gate's page; the new section and the rename following land here
  → Constraint: "The discrimination record" and "Producing a record" sections describe the obligation billed against HEAD^; they must say a byte-pure rename is followed.
- [ ] `docs/architecture/core-design.md` - declared by every `internal/le/rfc` file's `// Design:` header
  → Constraint: its paragraph naming `./le rfc reseal` and `./le rfc audit-stamp` as Go writers whose edits no hook sees, so `./le rfc index-update` follows them, must name `./le rfc rename` too.
- [ ] `docs/contributing/rule-authoring.md` - how a point is added
  → Constraint: write the point under a free slug, add the slug under its section in the manifest, then run `./le ai rules render-update`, `condensed-update`, `index-update`, `lint`, in that order.
- [ ] `ai/rules/points/testing/manifest.md` - the `rfc-tagged-tests-blocking` section lists four points
  → Decision: the new slug `name-a-single-rfc-test-file-for-its-rfc` goes after `reindex-after-moving-a-tagged-test` (checked free on 2026-10-03).
- [ ] `ai/rules/rfc-compliance.md` - a tag added owes a discrimination record in the same change
  → Constraint: Phase 3 adds a tag only where the test truly covers the requirement, and then records the proof with `./le rfc discriminate-record` in the same commit.
- [ ] `ai/rules/spec-no-code.md`, `ai/rules/testing.md`, `ai/rules/planning.md`, `ai/rules/writing.md`

**Key insights:**
- Two things break on a rename today, and both are path keys over path-independent hashes: the HEAD^ / backlog cover baselines, and the `unit` / `tests` / `units` / `code` keys in the JSON evidence.
- Byte purity is what lets every existing gate (weakened audit, RFC-change gate, discrimination freshness, audit freshness) read a rename as no change.
- The naming predicate is declared once and used twice: by `checkTestFileNames` and by `./le rfc rename` to refuse a bad target.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/check.go` - `check` collects tags, builds three committed baselines through `readCommittedTags`, runs the checks in a fixed order and appends findings
- [ ] `internal/le/rfc/check_core.go` - `checkSuperseded` is a whole-tree structural check over requirements
- [ ] `internal/le/rfc/check_baseline.go` - `readCommittedTags` reads HEAD, HEAD^ and origin/main; `coversAt` mints a baseline's covers from its own blobs, keyed by that revision's paths
- [ ] `internal/le/rfc/discriminate.go` - `Cover`, `unitKeyAt`, `behaviorSHA`, `discriminationOwedTags`, `removable`
- [ ] `internal/le/rfc/freshness.go` - `unitIdentity` compares verdicts by (file, sha) multiset
- [ ] `internal/le/rfc/audit.go` - `VerdictRecord` with `tests`, `units`, `code` maps
- [ ] `internal/le/rfc/tags.go` - `goTagRE`, `parseTagRest`, `scanGoTags`
- [ ] `internal/le/rfc/carriers.go` - `ScanTree`, `scanDir`, `CarrierFor`, `skipDirs`
- [ ] `internal/le/rfc/reseal.go` - the atomic staged writer of `rfc/audit/`
- [ ] `internal/le/rfc/actions.go` - the `./le rfc` action table
- [ ] `internal/le/rfc/check_ratchets.go` - `discriminationWithdrawnErrors`
- [ ] `internal/le/go/module/rename.go` - the module rename precedent
- [ ] `internal/le/commit/rfcchange.go` - the RFC-change commit gate pairs renames
- [ ] `internal/le/test/weakened/audit.go` - `auditDiff` pairs renames with `git diff --name-status -M`
- [ ] `internal/le/doc/citation/citation.go` - the backtick and link citation grammar
- [ ] `internal/le/doc/check/links.go` - `sweepTracked` reads citations out of tracked files

**Behavior to preserve:**
- Every existing check, its order, its message and its exit code in `./le rfc check`.
- A rename with ANY content change still bills every cover in the file as new against HEAD^.
- A baseline git cannot read still accuses nobody.
- A record whose unit is gone is still `removable`, never stale.
- `./le rfc reseal` and `./le go module rename` behave as today.
- Multi-stem tagged files are not constrained by part (b).
- Interop carriers and every non-`_test.go` carrier are not judged.

**Behavior to change:**
- `coversAt` follows a byte-identical rename between the baseline revision and HEAD, so a moved cover is not new.
- New verb `./le rfc rename`.
- New check `checkTestFileNames` in `./le rfc check` (armed in Phase 3).
- About 509 test files renamed, 39 untagged RFC-named files and 6 mismatches repaired.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`: no arguments; reads the checkout's summaries, tags, JSON evidence and git history.
- `./le rfc rename from <old> to <new>`, or `./le rfc rename plan <file>` where the file holds one `<old> <new>` pair per line: keyword-value arguments, paths relative to the checkout root.

### Transformation Path
1. Rename following: `readCommittedTags` asks git for the exact renames of carrier files between the baseline revision and HEAD (blob ids equal, not a similarity score), builds an old-path to new-path map, and `coversAt` rewrites the path part of each baseline cover's `Unit` through that map before the comparison in `discriminationOwedTags`.
2. Rename action: parse arguments; for each pair run every refusal (below) with nothing written; read every `rfc/discrimination/*.json` and `rfc/audit/*.json` that names the old path, and every tracked file whose citations (the `internal/le/doc/check` grammar) resolve to it; then move the file with a plain rename, rewrite the JSON keys and fields, rewrite the backtick citations, and report.
3. Naming check: walk `testRoots` with `skipDirs`, keep `_test.go` files that `CarrierFor` holds with the `go` reader; for each, derive the name stem (longest known stem prefix), the tag stems (proof and gap tags through the requirement lookup), and the marker; apply the one naming predicate; append a finding per violation after `checkSuperseded`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/le/rfc` ↔ git | `git diff` with exact-rename detection and `--raw` blob ids, read through `gitOutput` | No |
| `internal/le/rfc` ↔ `internal/le/doc/check` | exported citation function over one line of a tracked file | No |
| `internal/le/rfc` ↔ JSON evidence | `rfc/discrimination/*.json` and `rfc/audit/*.json` read, rewritten by atomic replace | No |

### Integration Points
- `coversAt` in `check_baseline.go` - gains the rename map; both HEAD^ and origin/main baselines use it.
- `check` in `check.go` - one appended call to `checkTestFileNames`.
- `actions` in `actions.go` - one `rename` entry.
- `Paths` in `internal/le/doc/citation/citation.go` - the grammar, moved out of `internal/le/doc/check` into a leaf package so the rename and the sweep read one grammar. Exporting it from `doc/check` made a test import cycle (`doc/wiring` test, `rfc`, `doc/check`, `doc/wiring`).
- The atomic writer used by `reseal.go` - reused for the JSON rewrites.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Rename following is added inside `coversAt`, the one cover minting point for baselines |
| No unintended coupling (components stay isolated) | Yes | `internal/le/rfc` imports `internal/le/doc/check`; `go list -deps ./internal/le/doc/check` holds no `internal/le/rfc` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Citation grammar reused, not copied; the naming predicate is one function used by the check and the rename |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Development tooling, no wire path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The verb is one entry in the `actions` table, which derives dispatch, help and grammar |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Stems derive from `rfc/short/` file names through `summaryStems`; tag stems from `collected.Requirements`; no list of RFC numbers or drafts is written anywhere. Searched: `actions.go` table (derives help), `testRoots`, `skipDirs`, `CarrierFor` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `unit-sha`, `claim-sha`, and the audit `tests`/`units`/`code` shas do not depend on the file path for a `_test.go` to `_test.go` rename | `freshness.go` `unitIdentity` comment; `behaviorSHA(path, text)` uses the path for comment grammar only | Rewritten records go stale and rewritten verdicts go STALE_UNIT | `TestRenameKeepsDiscriminationRecordsVerified`, `TestRenameKeepsAuditVerdictsFresh` | confirmed 2026-10-03: both tests green |
| A-2 | The commit gates read a byte-pure rename as unchanged | `rfcchange.go` and `weakened/audit.go` pair renames and compare texts | Every Phase 2 commit needs an `RFC-approved:` trailer or reports WEAKENED | First Phase 2 commit through `./le commit create` reports no RFC-change and no weakened finding | unvalidated |
| A-3 | Git's exact rename detection is not cut off by `diff.renameLimit` for 179 files in one commit | Git detects exact renames by blob id before the inexact pass the limit governs | Part of a component commit bills owed covers | The bgp Phase 2 commit: the exact-rename count git reports equals the number of moved files, and `./le rfc check` owes nothing new | unvalidated |
| A-4 | `internal/le/rfc` can import `internal/le/doc/check` without a cycle | `go list -deps ./internal/le/doc/check` run 2026-10-03 held no `internal/le/rfc` | The citation function moves to a leaf package both import | Phase 1 build | confirmed 2026-10-03: `rename.go` imports `doccheck` and the package builds |
| A-5 | No stem contains an underscore, and the stem to prefix map is injective over `rfc/short/` | `find rfc/short -name '*_*'` returned 0 on 2026-10-03; nine non-RFC stems, none a prefix of another after mapping | The inverse is ambiguous | `TestStemPrefixesAreInjectiveOverTheCorpus` | confirmed 2026-10-03: green over this checkout's `rfc/short/` |
| A-6 | The 13 backtick citations of candidate paths are the whole citation population | Research count | A dead citation fails `./le doc check links` after a phase | `./le rfc rename` report per component; `./le doc check links` before each Phase 2 commit | unvalidated |
| A-7 | Phase 2 needs no list that outlives this session | Owner decision 2026-10-03: Phase 2 input is `./le rfc rename propose`, derived from `checkTestFileNames` over the tree in hand; the scratch list was research evidence only | A later session would rename from a stale or missing list | AC-9 test of `propose` over a fixture tree; the per-component run in Phase 2 | validated by design |
| A-8 | No test reads its own file name or names a sibling test file in a string | No known case | A package test fails after a rename | The rename report lists plain mentions; each Phase 2 commit runs the package tests of its component | unvalidated |
| A-9 | A gap tag cites its stem for naming purposes just as a proof tag does | A gap tag demonstrates that RFC's requirement | Gap-only files are judged as untagged | `TestCheckTestFileNamesCountsGapTags` | confirmed 2026-10-03: green |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Another session edits a test file between planning and moving it | The source differs from its HEAD blob | `./le rfc rename` refuses a source whose bytes differ from HEAD or that is untracked; that pair waits for the next component pass |
| R-2 | Another session writes a discrimination record for the old path while the rename is in flight | The JSON file's bytes change between the rename's read and its write | The rename re-reads each JSON file immediately before its atomic replace and refuses that file if the bytes moved; a record written later for the old path reads `removable` and its cover owed, and `./le rfc check` names it |
| R-3 | A proposed target already exists, or two sources propose one target | Candidates column 4 (`exists`, `dup-target`), 26 rows | Hand-chosen topics in Phase 2 below; the rename refuses an existing target and two pairs with one target |
| R-4 | Another session has uncommitted hunks in an `rfc/discrimination/*.json` or `rfc/audit/*.json` the rename rewrites | `git diff` on that file before the commit | The rename edits only the path keys and leaves foreign hunks in place; the commit carries them under the git-safety rule after judging them against HEAD |
| R-5 | A 179-file rename commit buries a real edit in review | The commit's exact-rename count is below its file count | One commit per component; the commit body lists every pair; the reviewer compares the exact-rename count with the moved-file count |
| R-6 | A draft stem whose mapped prefix begins another stem's prefix makes a file name ambiguous | The injectivity test | Longest known prefix wins; the rename refuses a target whose inverse stem is not the file's single tag stem |
| R-7 | Other sessions commit new tagged files during the phases, so the armed check goes red for them | Phase 3 local run of `./le rfc check` shows names outside the lists | Arm in the Phase 3 commit after a whole-tree run; every finding names the exact `./le rfc rename from ... to ...` command or the marker to write |
| R-8 | A rename drops a build-constraint suffix (`_linux`, `_amd64`) and changes which platforms compile the test | The source and target differ in `go/build` file-name constraints | The rename refuses a target whose GOOS/GOARCH suffix set differs from the source's |
| R-9 | Another baseline keyed by path, not covered here, reads a rename as change | A finding appears after a byte-pure rename in the end-to-end fixture | `TestRenameEndToEndCheckFindingsUnchanged` compares the whole findings list before and after |
| R-10 | Adding a missing tag in Phase 3 stales an audit verdict, whose `tests` map changes | `./le rfc check` reports STALE for that requirement | Add a tag only where the test truly covers the requirement; then record the proof and re-judge the verdict with `./le rfc audit-stamp ... mode rejudge`; otherwise rename the file |
| R-11 | The new point shows as ungated in `./le ai rules gate-map-report`, because `// ze point:` bindings count only PreToolUse dispatchers | The report row | Accepted and recorded; no invented binding. `./le rfc check` is the enforcement |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A wrong baseline either owes too much (red gate) or too little (an unproven new cover passes). A wrong rename leaves a stale record or a dead citation, both of which `./le rfc check` and `./le doc check links` report |
| How is it reverted? | Phase 1: single commit revert. Phase 2: each component commit reverts on its own. Phase 3: single commit revert, which disarms the check |
| Who else touches this path? | Every session that tags tests, writes discrimination records or audit verdicts, or edits test files in the renamed packages |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` on a git fixture whose last commit is a byte-pure rename | → | `readCommittedTags` → `coversAt` rename map → `discriminationOwedTags` | `TestCheckOwesNothingForBytePureRename` |
| `./le rfc rename from <old> to <new>` through `Answer` with `ZE_REPO_ROOT` | → | `renameAnswer` → rename plan → move, JSON rewrite, citation rewrite | `TestRFCActionsRenameThroughAnswer` |
| `./le rfc check` on a fixture tree holding a misnamed tagged file | → | `check` → `checkTestFileNames` | `TestCheckReportsTestFileNameFindings` |
| Rename through the action, commit, then `./le rfc check` | → | rename + baseline + record + audit freshness together | `TestRenameEndToEndCheckFindingsUnchanged` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | HEAD renames a tagged carrier file with an identical blob; its gated covers have no record | `./le rfc check` reports no discrimination obligation for those covers |
| AC-2 | The unpushed commits include the same byte-pure rename | The published backlog does not count those covers as added since origin/main |
| AC-3 | HEAD renames a tagged file and changes one byte of it (above git's similarity threshold), or rewrites it below the threshold | Every gated cover in the file without a verifying record is owed, as today |
| AC-4 | `./le rfc rename from A to B` where A has discrimination records | B exists with A's exact bytes, A is gone, every record's `unit` (and `producer` where it named A) now names B, and every one of them verifies |
| AC-5 | Same, where A has audit verdicts | Every `tests`, `units` and `code` key naming A names B; those verdicts read FRESH |
| AC-6 | Tracked files cite A in backticks or markdown links, and also mention it in plain text | Each citation now names B; each plain mention is listed in the report as file and line and left unchanged |
| AC-7 | B exists; or B is in another directory; or B's build-constraint suffix differs; or A or B is not a `_test.go` file; or A is untracked or differs from HEAD | The action exits 2, names the refusal and the path, and writes nothing |
| AC-8 | B fails the naming rule for A's tags (B names a stem A carries no tag for, or A is single-stem and B is not named for that stem) | The action exits 2, names the stem it expected, and writes nothing |
| AC-9 | `plan <file>` with several pairs, one of which is refused, or two pairs naming one target; or `plan` given together with `from`/`to`, or neither | The action exits 2, names every refused pair, and writes nothing |
| AC-9b | `./le rfc rename propose` over a fixture tree holding one misnamed single-stem file, one whose proposed target exists, and one correctly named file | The plan file holds exactly one pair, the first file to its finding's target; the report names the colliding file and its taken target; the correct file appears nowhere |
| AC-10 | A successful rename | The report lists the move, each JSON file and key count rewritten, each citation rewritten, each plain mention, and names `./le rfc index-update` as the next step |
| AC-11 | A `_test.go` file named for stem S carries no tag for S and no marker | `./le rfc check` reports it, naming the file, S, the marker text, and a `./le rfc rename` command |
| AC-12 | Same file carries the marker with a non-empty reason | No finding |
| AC-13 | The marker has an empty reason, or sits in a file that carries a tag for its name stem, or sits in a file not named for any stem | One finding each, naming why the marker is wrong |
| AC-14 | A file whose proof and gap tags all cite one stem S is not named for S | `./le rfc check` reports it with the exact `./le rfc rename from <path> to <suggested path>` command |
| AC-15 | A file whose tags cite two or more stems | No part (b) finding whatever its name; a part (a) finding only when its name stem is not among its tag stems |
| AC-16 | Stem `rfc792`, stem `sflow-v5`, stem `draft-ietf-sidrops-8210bis` | Prefixes `rfc792_`, `sflow_v5_`, `draft_ietf_sidrops_8210bis_`; a file name maps back to the longest matching prefix; `rfcN_test.go` with no topic is named for `rfcN` |
| AC-17 | A misnamed file under `internal/le/`, `test/draft/`, `testdata/` or `vendor/`, or a non-`_test.go` carrier | No finding |
| AC-18 | Each Phase 2 commit | Byte-pure: the exact-rename count equals the moved-file count; `./le rfc check` owes no new record; the commit needs no `RFC-approved:` trailer |
| AC-19 | The tree after the Phase 3 commit | `./le rfc check` reports zero naming findings with `checkTestFileNames` wired; each of the 39 untagged files is renamed or carries a true marker; each of the 6 mismatches is renamed or tagged with a recorded proof |
| AC-20 | The rule point | `ai/rules/testing.md` renders the new directive under "RFC-Tagged Tests"; `./le ai rules lint` and `./le ai rules render-check` pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckOwesNothingForBytePureRename` | `internal/le/rfc/check_rename_baseline_test.go` | AC-1, wiring | |
| `TestCheckBacklogFollowsBytePureRename` | `internal/le/rfc/check_rename_baseline_test.go` | AC-2 | |
| `TestCheckOwesAgainWhenRenameChangesOneByte` | `internal/le/rfc/check_rename_baseline_test.go` | AC-3, above similarity | |
| `TestCheckOwesAgainWhenRenameRewritesTheFile` | `internal/le/rfc/check_rename_baseline_test.go` | AC-3, below similarity | |
| `TestCheckRenameMapUnreadableAccusesNobody` | `internal/le/rfc/check_rename_baseline_test.go` | a git failure in the rename read leaves the baseline unknown, not empty | |
| `TestRenameMovesFileAndRewritesDiscriminationUnits` | `internal/le/rfc/rename_test.go` | AC-4 | |
| `TestRenameKeepsDiscriminationRecordsVerified` | `internal/le/rfc/rename_test.go` | AC-4, A-1 | |
| `TestRenameKeepsAuditVerdictsFresh` | `internal/le/rfc/rename_test.go` | AC-5, A-1 | |
| `TestRenameRewritesCitationsAndListsPlainMentions` | `internal/le/rfc/rename_test.go` | AC-6 | |
| `TestRenameRefusesExistingTarget` | `internal/le/rfc/rename_test.go` | AC-7 | |
| `TestRenameRefusesDirectoryChange` | `internal/le/rfc/rename_test.go` | AC-7 | |
| `TestRenameRefusesBuildSuffixChange` | `internal/le/rfc/rename_test.go` | AC-7, R-8 | |
| `TestRenameRefusesNonTestFile` | `internal/le/rfc/rename_test.go` | AC-7 | |
| `TestRenameRefusesDirtyOrUntrackedSource` | `internal/le/rfc/rename_test.go` | AC-7, R-1 | |
| `TestRenameRefusesTargetFailingTheNamingRule` | `internal/le/rfc/rename_test.go` | AC-8 | |
| `TestRenamePlanRefusesWholeBatchOnOneBadPair` | `internal/le/rfc/rename_test.go` | AC-9 | |
| `TestRenamePlanRefusesTwoPairsOneTarget` | `internal/le/rfc/rename_test.go` | AC-9 | |
| `TestRenameRefusesPlanWithFromTo` | `internal/le/rfc/rename_test.go` | AC-9 | |
| `TestRenameProposeWritesOnePairPerFindingAndNamesCollisions` | `internal/le/rfc/rename_test.go` | AC-9b | |
| `TestRenameRefusesJSONChangedDuringRename` | `internal/le/rfc/rename_test.go` | R-2 | |
| `TestRenameReportNamesIndexUpdate` | `internal/le/rfc/rename_test.go` | AC-10 | |
| `TestRenameEndToEndCheckFindingsUnchanged` | `internal/le/rfc/rename_test.go` | AC-1, AC-4, AC-5, R-9 | |
| `TestRFCActionsCarryRenameVerb` | `internal/le/rfc/actions_test.go` | verb in table with its keywords | |
| `TestRFCActionsRenameThroughAnswer` | `internal/le/rfc/actions_test.go` | wiring | |
| `TestStemPrefixOfRFCHasNoPadding` | `internal/le/rfc/names_test.go` | AC-16 | |
| `TestStemPrefixOfDraftTurnsHyphensToUnderscores` | `internal/le/rfc/names_test.go` | AC-16 | |
| `TestStemOfFileNameTakesTheLongestKnownPrefix` | `internal/le/rfc/names_test.go` | AC-16, R-6 | |
| `TestStemOfFileNameAcceptsBareStemName` | `internal/le/rfc/names_test.go` | AC-16 | |
| `TestStemPrefixesAreInjectiveOverTheCorpus` | `internal/le/rfc/names_test.go` | A-5, reads this checkout's `rfc/short/` | |
| `TestRFCNamingMarkerIsNotATag` | `internal/le/rfc/names_test.go` | the marker never matches `goTagRE` | |
| `TestCheckTestFileNamesRefusesUntaggedStemNamedFile` | `internal/le/rfc/names_test.go` | AC-11 | |
| `TestCheckTestFileNamesAcceptsMarkerWithReason` | `internal/le/rfc/names_test.go` | AC-12 | |
| `TestCheckTestFileNamesRefusesBadMarker` | `internal/le/rfc/names_test.go` | AC-13, three cases | |
| `TestCheckTestFileNamesRefusesSingleStemFileNamedOtherwise` | `internal/le/rfc/names_test.go` | AC-14, message carries the command | |
| `TestCheckTestFileNamesJudgesMultiStemFilesByPartAOnly` | `internal/le/rfc/names_test.go` | AC-15 | |
| `TestCheckTestFileNamesCountsGapTags` | `internal/le/rfc/names_test.go` | A-9 | |
| `TestCheckTestFileNamesSkipsOutOfScopeFiles` | `internal/le/rfc/names_test.go` | AC-17 | |
| `TestCheckReportsTestFileNameFindings` | `internal/le/rfc/names_test.go` | wiring through `check` | |
| `TestCitedPathsMatchesTheLinkSweep` | `internal/le/doc/check/citation_test.go` | the link sweep reports, per line, exactly what `citation.Paths` answers | |
| `TestPathsAnswersEachCitationShape` | `internal/le/doc/citation/citation_test.go` | `citation.Paths` answers each citation shape, and nothing for a plain mention | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Bytes differing between the baseline blob and the renamed blob | 0 to file size | 0 (followed) | N/A | 1 (not followed: `TestCheckOwesAgainWhenRenameChangesOneByte`) |
| Marker reason length | 0 or more characters after the separator | 1 | 0 (refused) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Whole-tree `./le rfc check` | this checkout, Phase 3 commit | The armed check over the real tree reports zero naming findings; the output is pasted in the spec | |
| `./le rfc rename` on the real tree | each Phase 2 commit | The action moves a component's files and the commit gates accept them | |

No `.ci` suite drives `./le rfc` actions; the action is reached through `Answer`, which is what `le` dispatches, and the two real-tree runs above are the end-to-end proof.

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer, no wire-visible change (`ai/rules/interop-and-goal-validation.md` exemption).

## Files to Modify
- `internal/le/rfc/check_baseline.go` - rename map read from git; `coversAt` rewrites baseline cover paths through it; `readCommittedTags` passes the revision pair
- `internal/le/rfc/check.go` - Phase 3: one call to `checkTestFileNames` after `checkSuperseded`
- `internal/le/rfc/actions.go` - `rename` verb with `from`, `to`, `plan` keywords and `Writes: true`
- `internal/le/rfc/actions_test.go` - two tests
- `internal/le/doc/check/citation.go` - the grammar moves out; the ignore markers and path resolution stay
- `internal/le/doc/citation/citation.go` (new) - the citation grammar, `Paths`, as a leaf package
- `docs/contributing/rfc-conformance-gates.md` - "Test file names" section; rename following in "The discrimination record"; `./le rfc rename` beside the record writers
- `docs/contributing/rfc-implementation-guide.md` - one sentence where it says which carrier a tag lives in, pointing at the naming section
- `docs/architecture/core-design.md` - the Go-writer sentence names `./le rfc rename`
- `ai/rules/points/testing/manifest.md` - new slug
- `ai/rationale/testing.md` - why the name and the tags agree
- `ai/rules/testing.md`, `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md`, `ai/rules/INDEX.md` - regenerated, never hand-edited
- Phase 2: about 503 test files renamed (462 rfcN and 41 draft/sflow), `rfc/discrimination/*.json`, `rfc/audit/*.json`, and the tracked files citing them
- Phase 3: the 39 untagged and 6 mismatched files listed below, and any JSON and citations their renames touch

## Files to Create
- `internal/le/rfc/names.go` - stem prefix, inverse, marker constant, the naming predicate, `checkTestFileNames`
- `internal/le/rfc/names_test.go`
- `internal/le/rfc/rename.go` - the rename plan, refusals, move, JSON and citation rewrites, report
- `internal/le/rfc/rename_test.go`
- `internal/le/rfc/check_rename_baseline_test.go`
- `internal/le/doc/check/citation_test.go`, `internal/le/doc/citation/citation_test.go`
- `ai/rules/points/testing/rfc-tagged-tests-blocking/name-a-single-rfc-test-file-for-its-rfc.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Development tool, no config |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go` (`./le rfc rename`) |
| CLI grammar (keyword before value) | Yes | `from <path> to <path>` or `plan <path>`, keyword before value, as `audit-stamp` does |
| Editor autocomplete | N-A | `le` help and listings derive from the action table |
| Functional test for new RPC/API | N-A | No ze RPC; `TestRFCActionsRenameThroughAnswer` drives the dispatcher |
| Pipe completeness | N-A | `le` action output, not a ze CLI command |
| Env var registration | N-A | No new env var; tests use the existing `ZE_REPO_ROOT` |
| Doctor check for runtime dependencies | N-A | No runtime dependency added to ze; git is already required by `./le rfc check` |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Development tooling only |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | No | `./le` is not the ze CLI; `docs/guide/command-reference.md` covers ze. The verb is documented in `docs/contributing/rfc-conformance-gates.md` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | No wire change |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Evidence moves, no requirement changes state; derived ledgers are regenerated, not committed |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md` |
| 11 | Affects daemon comparison? | No | No behavior change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (Go writers of RFC inputs) |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `le` action only |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/spec-rfc-test-file-naming.md`: rerun at the start of Phase 1 and after each phase. `docs/architecture/core-design.md` is declared by the `internal/le/rfc` files' `// Design:` headers and is named above. Every doc citing a renamed test file in backticks is rewritten by the action. Advisory mentions of `internal/le/rfc/actions.go`, judged unaffected: `docs/features.md` (its interop row cites the file for the scenario list, which this spec does not change) and `docs/functional-tests.md` (its RFC-gate prose anchors `Answer` for the check, the re-stamp and the tagged-unit definition, none of which this spec changes; Phase 1d re-reads lines 1050 to 1150 and adds the rename beside the re-stamp if that prose lists the Go writers) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/rfc-conformance-gates.md` examples of `discriminate-record ... unit <file>::<Func>` stay valid; examples naming a renamed test path are rewritten |

## Implementation Steps

Three commits for Phases 1 and 3, and one commit per component for Phase 2. No commit waits behind another phase's review.

1. **Phase: Wiring (MANDATORY FIRST)** -- register the verb and write the failing wiring tests
   - Tests: `TestRFCActionsCarryRenameVerb`, `TestRFCActionsRenameThroughAnswer`, `TestCheckOwesNothingForBytePureRename`
   - Files: `internal/le/rfc/actions.go`, `internal/le/rfc/rename.go` (answer returns a refusal), `internal/le/rfc/check_rename_baseline_test.go`
   - Verify: the verb dispatches; the baseline test fails because the rename is billed
2. **Phase 1a: baseline follows a byte-pure rename** -- in `check_baseline.go`, read the exact renames between the baseline revision and HEAD (`git diff` with exact-rename detection, `--raw` so both blob ids are compared, `-z`, restricted to `testRoots` and to paths `CarrierFor` holds); a read git cannot answer leaves that baseline unknown. `coversAt` rewrites the path part of each cover `Unit` (the text before `::`, or the bare path for a file-scoped key) through the map. Both HEAD^ and origin/main use it.
   - Tests: the five `check_rename_baseline_test.go` tests
   - Verify: fail, implement, pass
3. **Phase 1b: the naming predicate** -- `names.go`: stem prefix (`rfcN` gives `rfcN_`; any other stem gives the stem with `-` turned into `_`, plus `_`); inverse (longest prefix among `summaryStems`; a base name beginning `rfc<digits>_` whose stem has no summary is still named for that stem, and can only be satisfied by the marker or a rename); the marker constant; the predicate over (file name, tag stems, marker), returning the violation and the suggested target. `checkTestFileNames` is written and unit-tested here and wired in Phase 3.
   - Tests: the `names_test.go` tests except `TestCheckReportsTestFileNameFindings`
   - Verify: fail, implement, pass
4. **Phase 1c: `./le rfc rename`** -- move the citation grammar into the leaf package `internal/le/doc/citation`; in `rename.go` build a plan for every pair, run every refusal (AC-7, AC-8, AC-9, R-8) before writing; then move each file with a plain rename, rewrite every record field and every audit map key whose file part is the old path, re-reading each JSON file just before its atomic replace (R-2), rewrite backtick and link citations in tracked files, and report (AC-10).
   - Tests: the `rename_test.go` tests and `TestCitedPathsMatchesTheLinkSweep`
   - Verify: fail, implement, pass; `TestRenameEndToEndCheckFindingsUnchanged` green
5. **Phase 1d: docs** -- `rfc-conformance-gates.md` (rename following, the verb, and a "Test file names" section describing the convention as it will arm), `rfc-implementation-guide.md` pointer, `core-design.md` sentence. Commit 1: code, tests, docs.
6. **Phase 2: renames, one commit per component** -- component is the directory under `internal/component/`, `internal/plugins/`, `internal/core/`, `internal/mrt/`, `cmd/ze/` (research counts: bgp 179, ospf 78, l2tp 51, isis 39, flowexport 35, rsvpte 21, ike 17, config 16, and 21 smaller). Input: the part (b) findings of `checkTestFileNames` over the tree in hand, never the research scratch list. Phase 1 adds `./le rfc rename propose`, which runs the unwired check and writes a `plan` file of one `from`/`to` pair per finding, each target being the exact rename the finding names; a target that collides is left out and named in its report, and takes the hand-chosen target from the table below. The 6 mismatches stay for Phase 3. Files other sessions add between phases are picked up because the list is recomputed per component. Run `./le rfc rename plan <file>` per component, then `./le doc check links`, the component's package tests, and `./le rfc check`; commit with `./le commit create`, naming each new path and passing each old path to `remove`. The body lists every pair.
   - Verify: AC-18 for each commit
7. **Phase 3: repair and arm** -- for each of the 39 untagged files: read it; rename it after what it covers, or add the marker with a true reason. For each of the 6 mismatches: read it; add the missing tag only if the test truly covers that requirement, and then record its proof with `./le rfc discriminate-record` and re-judge any audit verdict it stales (R-10); otherwise rename it for the stem it does tag. Run the whole-tree check locally with `checkTestFileNames` wired; repair any file other sessions added (R-7). Wire `checkTestFileNames` after `checkSuperseded` in `check`, write the point file, add its slug to the manifest, the rationale paragraph, run render, condensed, index, lint. Commit 3: the repairs, the wiring, the point and its generated files, the doc section update, together, so the check never arms red.
   - Tests: `TestCheckReportsTestFileNameFindings`
   - Verify: AC-19, AC-20; paste the whole-tree `./le rfc check` output

### Phase 2 collision targets (hand-chosen)

| Source | Target |
|--------|--------|
| `internal/component/bgp/plugins/filter_community/blackhole_rfc7999_test.go` | `rfc7999_blackhole_propagation_test.go` |
| `internal/component/bgp/plugins/filter_community/blackhole_test.go` | `rfc7999_blackhole_guard_test.go` |
| `internal/component/bgp/plugins/filter_remove_private_as/private_as_rfc6996_test.go` | `rfc6996_private_as_attributes_test.go` |
| `internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go` | `rfc6996_private_as_test.go` |
| `internal/component/config/schema_defaults_rfc7950_test.go` | `rfc7950_leaf_default_test.go` |
| `internal/component/config/schema_defaults_test.go` | `rfc7950_schema_defaults_test.go` |
| `internal/component/ike/crypto/rfc7296_proposal_negotiation_test.go` | `rfc7296_proposal_negotiation_test.go` |
| `internal/component/ike/engine/rfc7296_dpd_liveness_test.go` | `rfc7296_dpd_liveness_test.go` |
| `internal/component/ike/engine/rfc7296_notify_out_of_sa_test.go` | `rfc7296_notify_out_of_sa_test.go` |
| `internal/component/ike/engine/rfc7296_rekey_collision_test.go` | `rfc7296_rekey_collision_test.go` |
| `internal/component/l2tp/plugins/authradius/coa_test.go` | `rfc5176_coa_listener_test.go` |
| `internal/component/mcp/as_metadata_rfc8414_test.go` | `rfc8414_metadata_request_test.go` |
| `internal/component/mcp/as_metadata_test.go` | `rfc8414_as_metadata_test.go` |
| `internal/component/tacacs/rfc8907_accountant_test.go` | `rfc8907_accountant_test.go` |
| `internal/core/bgp/attribute/aigp_rfc7311_test.go` | `rfc7311_aigp_tlv_test.go` |
| `internal/core/bgp/attribute/aigp_test.go` | `rfc7311_aigp_test.go` |
| `internal/plugins/isis/bgpls_export_rfc9552_test.go` | `rfc9552_router_id_test.go` |
| `internal/plugins/isis/bgpls_export_test.go` | `rfc9552_bgpls_export_test.go` |
| `internal/plugins/isis/packet/checksum_test.go` | `rfc905_checksum_vectors_test.go` |
| `internal/plugins/isis/packet/tlv_ipv4_rfc5305_test.go` | `rfc5305_tlv_ipv4_presence_test.go` |
| `internal/plugins/isis/packet/tlv_ipv4_test.go` | `rfc5305_tlv_ipv4_test.go` |
| `internal/plugins/ospf/lsdb/flooding_test.go` | `rfc2328_flooding_decisions_test.go` |
| `internal/plugins/rsvpte/engine_rfc3209_test.go` | `rfc3209_engine_label_test.go` |
| `internal/plugins/rsvpte/engine_test.go` | `rfc3209_engine_test.go` |
| `internal/plugins/rsvpte/softstate_rfc2205_test.go` | `rfc2205_softstate_timeout_test.go` |
| `internal/plugins/rsvpte/softstate_test.go` | `rfc2205_softstate_test.go` |
| `internal/plugins/flowexport/sflow/flow_rfc_sflow_v5_test.go` | `sflow_v5_flow_sampling_test.go` |
| `internal/plugins/flowexport/sflow/flow_test.go` | `sflow_v5_flow_test.go` |
| `internal/component/bgp/plugins/nlri/mup/rfc_mup_session_test.go` | `rfc4760_mup_session_test.go` |
| `cmd/ze/hub/service_mcp_rfc9325_test.go` | `rfc9728_service_mcp_tls_suites_test.go` |
| `internal/component/bgp/reactor/config_paths_limit_test.go` | `draft_abraitis_idr_addpath_paths_limit_config_test.go` |
| `internal/component/bgp/reactor/session_paths_limit_test.go` | `draft_abraitis_idr_addpath_paths_limit_session_test.go` |

Each target stays in its source's directory. Every target was checked absent on 2026-10-03; the rename refuses one that has appeared since.

→ Decision (review of fd31c539ce, R-6, 2026-10-03): `rfc_mup_session_test.go` tags only RFC 4760 (its tests drive `capability.Negotiate` over the two MUP Multiprotocol capabilities), and `mup` is no word of `rfc4760`, so the tool keeps `rfc_mup` as topic and proposes `rfc4760_rfc_mup_session_test.go`. That target is free, so `propose` writes the pair rather than leaving it out: the component's plan MUST carry the row above instead.
→ Decision (review of 71b1772696, F-1, 2026-10-03): `service_mcp_rfc9325_test.go` tags only RFC 9728 (RFC9728-7.1-2, the BCP 195 cipher suites the MCP listener negotiates), and `rfc9325` is no spelling of `rfc9728`, so `propose` writes the free target `rfc9728_service_mcp_rfc9325_test.go`, which names a second RFC the file does not tag. The row above names its subject, the TLS suites, instead.
→ Decision (review of 71b1772696, F-7, 2026-10-03): `config_paths_limit_test.go` and `session_paths_limit_test.go` tag only the paths-limit draft, whose words `paths` and `limit` the tool keeps when they stand alone (R-2), so `propose` writes `draft_abraitis_idr_addpath_paths_limit_config_paths_limit_test.go`, the stem's words twice. The rows above keep the topic word: `config` (the knob that decides the OPEN limits) and `session`. The session row has the same shape as F-7 and was added with it.
→ Decision (D-1 recount, 2026-10-03): a fresh whole-tree `propose` with the inner stem dropped writes 439 pairs and leaves out 23 collisions and 6 name/tag mismatches. The 23 are the 21 rows above not yet landed (the ike and tacacs rows landed in e52260d978 and 489a1b53a5) plus the sflow `flow` pair, whose two sources now propose the same `sflow_v5_flow_test.go` and gained the last two rows. Every other hand-chosen row is still needed: each of its pairs still collides on its proposed name.

### Phase 3 untagged RFC-named files (39)

| Directory | Files |
|-----------|-------|
| `internal/component/bfd/engine/` | `rfc5881_link_local_test.go`, `rfc5882_join_test.go` |
| `internal/component/bgp/message/` | `rfc7606_addpath_test.go`, `rfc7606_bench_test.go`, `rfc7606_bgpls_nlri_fuzz_test.go`, `rfc7606_bgpls_nlri_test.go`, `rfc7606_withdraw_families_test.go` |
| `internal/component/bgp/plugins/filter_path_asn/` | `rfc6793_subject_test.go` |
| `internal/component/bgp/plugins/nlri/evpn/` | `rfc7606_test.go` |
| `internal/component/bgp/plugins/nlri/ls/` | `rfc8571_attr_reserved_test.go`, `rfc9514_srv6_descriptor_test.go` |
| `internal/component/bgp/plugins/nlri/srpolicy/` | `rfc9012_test.go` |
| `internal/component/bgp/plugins/rib/` | `rfc4364_vpn_bestchange_test.go`, `rfc8277_addpath_comparable_red_test.go` |
| `internal/component/bgp/plugins/rib/storage/` | `rfc6793_reconstruct_test.go` |
| `internal/component/bgp/reactor/` | `rfc6793_ingest_collapse_test.go`, `rfc7606_session_families_test.go`, `rfc7705_local_as_announce_test.go`, `rfc8669_duplicate_tlv_red_test.go`, `rfc8950_family_scope_test.go` |
| `internal/component/bgp/wireu/` | `rfc7606_split_test.go` |
| `internal/component/ike/crypto/` | `rfc5903_ecp_test.go` |
| `internal/component/ike/dataplane/` | `rfc7296_ecn_test.go` |
| `internal/component/ike/engine/` | `rfc7296_invalid_syntax_fatal_test.go`, `rfc7296_teardown_notify_test.go`, `rfc7427_algid_test.go`, `rfc9190_crl_wiring_test.go`, `rfc9190_ocsp_wiring_test.go`, `rfc9190_resumption_wiring_test.go` |
| `internal/component/ike/ipsec/` | `rfc4301_policy_order_test.go` |
| `internal/component/l2tp/plugins/authradius/` | `rfc5176_disconnect_service_stopped_test.go` |
| `internal/component/l2tp/ppp/` | `rfc1661_invalid_reply_test.go` |
| `internal/core/eap/` | `rfc7296_method_test.go`, `rfc9190_peer_indication_test.go` |
| `internal/mrt/` | `rfc8050_addpath_nlri_red_test.go` |
| `internal/plugins/fib/vpp/` | `rfc9252_srv6_encap_red_test.go` |
| `internal/plugins/geodns/` | `rfc1035_server_test.go` |
| `internal/plugins/ospf/` | `rfc5340_checksum_test.go` |
| `internal/plugins/ospf/spf/` | `rfc4577_test.go` |

### Phase 3 name/tag mismatches (6)

| File | Name stem | Tag stem |
|------|-----------|----------|
| `internal/component/bgp/message/rfc7606_aigp_test.go` | rfc7606 | rfc7311 |
| `internal/component/bgp/reactor/rfc7606_session_addpath_test.go` | rfc7606 | rfc7911 |
| `internal/component/bgp/reactor/rfc8050_wire_capture_test.go` | rfc8050 | rfc6396 |
| `internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go` | rfc3748 | rfc7296 |
| `internal/component/l2tp/plugins/authradius/rfc2548_mschap2_success_test.go` | rfc2548 | rfc2865 |
| `internal/component/l2tp/ppp/rfc1877_dns_options_test.go` | rfc1877 | rfc1661 |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each entry point (check, rename) is reached by a test through `check` or `Answer` |
| Correctness | Only identical blob ids are followed; a git failure in the rename read leaves the baseline unknown; the rename writes nothing on any refusal |
| Naming | The marker text never matches `goTagRE`; prefixes derive from stems, no literal list |
| Data flow | One cover-minting point (`coversAt`); one naming predicate shared by check and rename; one citation grammar shared with the link sweep |
| Rule: `ai/rules/principles.md` (silent wrong values) | A rename map git cannot read is reported as unknown, never as "no renames" |
| Rule: `ai/rules/rfc-compliance.md` | Every tag added in Phase 3 carries its discrimination record in the same commit |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Baseline follows byte-pure renames | `./le job run` of `go test ./internal/le/rfc/ -run 'TestCheck.*Rename'` |
| `./le rfc rename` verb | `./le rfc help` lists it; `TestRFCActionsRenameThroughAnswer` |
| Naming check armed | `grep -n checkTestFileNames internal/le/rfc/check.go`; whole-tree `./le rfc check` output |
| Phase 2 byte purity | per commit, the exact-rename count equals the moved-file count |
| Rule point | `ls ai/rules/points/testing/rfc-tagged-tests-blocking/name-a-single-rfc-test-file-for-its-rfc.md`; `./le ai rules lint` |
| Docs | `grep -n "le rfc rename" docs/contributing/rfc-conformance-gates.md docs/architecture/core-design.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `from`, `to` and plan paths are cleaned and must stay inside the checkout; `..` and absolute paths are refused |
| Destructive writes | The rename never overwrites an existing file and never deletes anything but the moved source; JSON is replaced atomically |
| Concurrency | A JSON file changed by another session between read and write is refused, not clobbered |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A Phase 2 commit owes records or needs a trailer | A-2 or A-3 broken: stop the phase, record it, fix the baseline or the gate before the next component |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A rename only looks like a change to the readers that key by path; every hash underneath is path-independent, so the fix is to translate keys, never to re-stamp.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Full convention (a)+(b), multi-stem files free (owner, 2026-10-03) | Grandfather existing names; enforce only (a) | The owner chose the full convention and renaming over grandfathering |
| Stem prefix `rfcN_` with no padding; other stems with `-` turned into `_` (owner) | Zero padding; keep hyphens | Matches `rfc/short/` names; hyphens are not idiomatic in Go file names |
| Tag to stem through the requirement lookup, file name to stem by longest known prefix (owner) | Regex over the id | Draft stems carry hyphens and digits, so a regex cannot split the id |
| Marker text `// RFC naming: untagged -- <reason>`, reason required | Free comment; no marker | A fixed text is greppable and cannot match `goTagRE` |
| Baseline follows only identical blobs, compared by blob id (owner) | Follow any `-M` rename | Byte purity is what makes every other gate read the rename as no change; a rename with an edit is reviewed as an edit |
| A native `./le rfc rename` rewrites keys (owner) | Mass re-record and re-audit; `./le rfc reseal` | The hashes are path-independent, so rewriting keys keeps the evidence verified with no re-judging |
| Rename reuses the link sweep's citation grammar | A second regex in `internal/le/rfc` | One declaration of what a citation is |
| Three phases, Phase 2 one commit per component, arming in the Phase 3 commit (owner) | One commit | Each commit stands alone; the check never arms red |
| Interop carriers out of scope (owner) | Name `.ci` and scenario files too | Scenario names are identities cited elsewhere (`interop-and-goal-validation.md`) |
| `checkTestFileNames` written in Phase 1, wired in Phase 3 | Write it in Phase 3 | The rename refuses a bad target with the same predicate, so the predicate must exist in Phase 1 |

→ Decision (Phase 1, 2026-10-03): `propose` takes its output path as its value, `propose <file>`, and an optional `under <dir>` narrows it to one component, because Phase 2 runs one plan per component. The output file is created with `O_EXCL`, never overwritten.
→ Decision (Phase 1): `propose` leaves out a file already named for ANOTHER stem (the name/tag mismatches), lists it in its report, and writes no pair for it, because Phase 3 reads each of those before anything moves.
→ Decision (Phase 1): R-2 is enforced batch-wide: `applyRename` re-reads every evidence and cited file it will rewrite before it links any target, and refuses the whole batch, writing nothing, when one moved. Targets are created by `os.Link`, so an existing file can never be overwritten.
→ Decision (review round 1, N-5, replaces the Phase 1 textual rewrite): the evidence rewrite is by field and in place. A JSON token walk (`evidenceStrings`) names each string by its key chain, and only a discrimination record's `unit` and `producer` and an audit requirement's `tests`, `units` and `code` keys are rewritten, when equal to the old path or opening with it then `::`. The bytes between those strings are copied, so formatting and foreign hunks survive (R-4), and a `break`, a note or a fingerprint value that spells the old path keeps its bytes. A matching string written with an escape is refused rather than rewritten in a second spelling. Chosen over a decode and re-encode, which would reformat every file it touched.
→ Decision (Phase 1): a moved file is never edited, even where it cites itself or another moved file, because the move must stay byte-pure; such a line is listed as a mention instead. A citation is an occurrence whose replacement removes one citation of the old path under `citation.Paths`, so a plain mention on the same line is left alone.
→ Decision (review round 1, I-1, replaces the Phase 1 suffix parse): `go/build` judges the suffix. For every pair `go tool dist list` names, `build.Context.MatchFile` (OpenFile answering a bare package clause, so only the name is read) answers whether the source name and the target name build there, and the rename is refused when the two answer sets differ. `go/build`'s own known-name list is wider than the ports (`_sparc`, `_zos`, `_hurd`, `_amd64p32`) and carries the implied names (`_linux` for android), so no list is copied and no parse of the rule is kept beside it.
→ Decision (review round 1, I-3, replaces the Phase 1 function-level proof): `coversAt` takes its rename reader as a parameter (`renameReader`, `exactRenamesSince` in both production callers), so `TestCheckRenameMapUnreadableAccusesNobody` drives a readable baseline with a failing reader and reaches the map guard, which no git fixture can, because `git diff` and `git grep` at one revision fail together.
→ Decision (review round 1, N-4): a write that fails after the first source is removed answers exit 2 WITH the report of what was written (`Stopped`, and `Linked` for a target whose source is still in place); a refusal before any write still answers no report.
→ Decision (review round 1, N-6): after the rewrite, a line holding a brace that still cites an old path under `citation.Paths` is listed as stale (`Stale`), and the tracked-file search also matches the source's directory, which is what a brace citation still spells.
→ Decision (review round 1, N-8): the rename applies the naming rule only to a target `CarrierFor` holds as a unit carrier, the population `testFileNameVerdicts` judges (AC-17).

## Known Limitations

- Multi-stem files keep any name (owner decision).
- Interop and other non-`_test.go` carriers are not judged (owner decision).
- Files under `internal/le/` and `test/draft/` are not judged, because `CarrierFor` does not count their tags as evidence.
- The point is enforced by `./le rfc check`, not by a PreToolUse hook, so `gate-map-report` shows it ungated (R-11).
- `./le rfc rename` lists no stale brace citation inside a moved file, nor one whose braces span a directory segment (review round 2, N-4); those are edited by hand.

## Review Gate

<!-- Filled by /ze-close through /ze-review. Run tables left empty at design time. -->

| Field | Value |
|-------|-------|
| Artifact | Phase 1, commit e87b8bb205 |
| `./le spec review check` | not run |
| Rounds | 5 (round 1 scope: e87b8bb205: baseline rename following, `./le rfc rename`, the naming predicate, the citation leaf package; round 2 scope: 150892523b; round 3 scope: 14d6771780; round 4 scope: fd31c539ce; round 5 scope: 71b1772696) |
| Reviewer lenses used | independent review of e87b8bb205 |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| I-1 | ISSUE | The suffix refusal knew only `go tool dist list` names, so `_sparc`, `_zos` and the other names `go/build` knows without a port were not refused | `internal/le/rfc/rename.go` `goPlatformSet.suffix` | `platformsBuilding`, `buildSuffixMoves` over `build.Context.MatchFile`; `TestRenameRefusesBuildSuffixChange` (`_sparc`, `_zos`), `TestBuildSuffixMovesFollowsGoBuild` |
| I-2 | ISSUE | No fixture reached the `::` branch of `followRename` | `internal/le/rfc/check_baseline.go` | `TestCheckFollowsBytePureRenameOfTaggedGoFunction` (HEAD^ and backlog) |
| I-3 | ISSUE | The unreadable-map test never reached the guard after the rename reader in `coversAt` | `internal/le/rfc/check_baseline.go` `coversAt` | injectable `renameReader`; `TestCheckRenameMapUnreadableAccusesNobody` |
| N-4 | NOTE | A failure mid-apply dropped the report of the writes already made | `internal/le/rfc/rename.go` `applyRename`, `renameAnswer` | `stopRename`; `TestRenameReportsPartialWritesWhenItStops` |
| N-5 | NOTE | The textual evidence rewrite also moved any other string spelling the old path | `internal/le/rfc/rename.go` `rewriteEvidencePaths` | field walk; `TestRenameEvidenceRewriteTouchesPathFieldsOnly` |
| N-6 | NOTE | A brace citation kept citing the old path, unlisted | `internal/le/rfc/rename.go` `planCitations` | `citesAnySource`, `Stale`; `TestRenameListsBraceCitationsLeftOnTheOldPath` |
| N-8 | NOTE | The naming rule was applied to targets no carrier judges | `internal/le/rfc/rename.go` `refusePair` | `CarrierFor` unit gate; `TestRenameJudgesTheNameOfUnitCarriersOnly` |
| N-9 | NOTE | No unit case for unequal blob ids | `internal/le/rfc/check_baseline.go` `parseExactRenames` | `TestParseExactRenamesSkipsUnequalBlobs` |

### Round 2 (scope: 150892523b and the sibling call sites it touched)

Verdict 0 BLOCKER, 1 ISSUE, 5 NOTE. Every round 1 finding was confirmed closed at its producer by a mutation that turned its test red.

| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| I-1 | ISSUE | The escaped-path refusal and the not-JSON branch were driven by no test | `internal/le/rfc/rename.go` `rewriteEvidencePaths` | Fixed. `TestRenameRefusesEscapedEvidencePath`, `TestRenameRefusesTruncatedEvidenceJSON`, `TestEvidenceStringsRefusesAllButOneValue`. The truncated case exposed a defect: `json.Decoder.Token` answers `io.EOF` between tokens at any depth and reads a stream, so `evidenceStrings` accepted a file cut after a token, an empty file and two values. It now refuses an EOF inside an open object or array and any count of top-level values but one. Each guard shown red by an overlay mutant |
| N-1 | NOTE | The `dir/` grep pattern makes the 494-pair plan about 3x slower (62 s against 22 s) and found no extra stale line | `rename.go` `trackedFilesMentioning` | Accepted: cold path, run by hand |
| N-2 | NOTE | The read-only-directory test gives a false red under root | `rename_test.go` `TestRenameReportsPartialWritesWhenItStops` | Fixed: skips when `os.Geteuid() == 0` |
| N-3 | NOTE | No test drives the `report.Linked` arm | `rename.go` `applyRename` | Accepted: reachable only through a race, now said in a comment on the arm |
| N-4 | NOTE | A brace citation inside a moved file, or one whose braces span a directory segment, is never listed as stale | `rename.go` `planCitations` | Documented: `docs/contributing/rfc-conformance-gates.md` "Moving a tagged test file", and Known Limitations |
| N-5 | NOTE | The verification-debt row for 150892523b cited lines 26 and 382; only line 26 matches the gate | `plan/verification-debt/1d41415e.md` | Row text corrected to line 26; its status is untouched, the owner's confirmation is still owed |

The commit gate's false positive on `rename_test.go` (the file-scope literal `renameTestSource` spelled a tag) is removed by respelling the literal as a concatenation, and recorded in `plan/journal/guard-blocks-its-own-authors-repair.md`.

### Round 3 (scope: 14d6771780 and the callers of `evidenceStrings` and `rewriteEvidencePaths`)

Verdict 0 BLOCKER, 0 ISSUE, 3 NOTE. Evidence: all 315 JSON files under `rfc/discrimination/` and `rfc/audit/` parse and rewrite reversibly, and the fixture passes at f9ca905e00. The Phase 1 review loop is closed.

| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| N-1 | NOTE | `TestRenameRefusesTruncatedEvidenceJSON` does not discriminate the open-object/array arm (the unit case `TestEvidenceStringsRefusesAllButOneValue` does), and its comment claims more than it shows | `internal/le/rfc/rename_test.go` | Accepted |
| N-2 | NOTE | The `start < written` half of the escape check is driven by no test; it is reachable only with a path holding U+FFFD | `internal/le/rfc/rename.go` `rewriteEvidencePaths` | Accepted |
| N-3 | NOTE | `auditDiff` never consults `RFC-approved:` trailers, so `./le commit audit base X` over 14d6771780 reports `rename_test.go` WEAKENED for good | `internal/le/test/weakened/audit.go` `auditDiff` | Recorded: `plan/journal/check-cannot-see-the-change-it-looks-for.md` |

### Round 4 (scope: fd31c539ce, the Phase 2 tool fixes D-1 to D-3)

| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| R-1 | ISSUE | A topic left empty whose bare `<stem>_test.go` was taken fell back to the old topic, which carried the stem twice | `internal/le/rfc/names.go` `judgeTestFileName` | Fixed: the bare target is kept and `propose` reports it as taken. `TestRenameProposeReportsTakenBareTarget`, red under an overlay of HEAD's `names.go` and `rename.go` (the plan held `rfc9999_rfc_rfc9999_test.go`) |
| R-2 | ISSUE | The legacy draft abbreviation `rfc_<word>` (`rfc_mup`) was no spelling of the draft stem, so the mup files proposed `draft_ietf_bess_mup_safi_rfc_mup_*` | `internal/le/rfc/names.go` `stemSpellingAt` | Fixed for the draft's own words; `rfc_mup_session_test.go` (RFC 4760) takes a hand-chosen topic. Two mup cases in `TestJudgeTestFileNameDropsTheInnerStem` (`rfc_mup_ingress`, `rfc_mup_safi`) go red with the new arm removed; the third, `rfc_mup_session` (RFC 4760), is a negative pin that stays green |
| R-3 | NOTE | `ai/INDEX.md` still named `citationExcludes`, which fd31c539ce moved | `ai/INDEX.md` | Fixed: it names `citation.Excluded`, `citation.Policed` and `citation.CorpusGlobs` in `internal/le/doc/citation/policed.go` |
| R-4 | NOTE | A suggested repair can change which platforms compile the file (`linux_test.go` would become `rfc5082_linux_test.go`); `./le rfc rename` refuses that rename, but the finding still suggests it | `internal/le/rfc/names.go` `judgeTestFileName` | Not done: `propose` drops a verdict with no target without a word, so refusing the target needs a new report category, past the 10-line bound the main thread set. The rename's `buildSuffixMoves` refusal still holds |
| R-5 | NOTE | No test killed the `buildSuffixLength` cap-1 mutant, nor the `existsIn`-always-false mutant | `internal/le/rfc/names.go` | Fixed: the case `foo_rfc_draft_linux_amd64_test.go` (a stem word at n-2 before an arch token) is red under the cap-1 and cap-0 mutants. `existsIn` is deleted by R-1; the taken path is now `proposeRenames`' own collision check, driven by `TestRenameProposeReportsTakenBareTarget` |
| R-6 | NOTE | Names the tool cannot repair alone | Phase 2 | Hand-chosen topics in Phase 2 (`rfc_mup_session_test.go` row added) |
| R-7 | NOTE | Rename skips exactly what the link sweep skips, except the sweep's own `links.go` and `links_test.go` (`sweepExcluded`), which rename still polices | `internal/le/rfc/rename.go` `planCitations` | Accepted: harmless, both read `citation.Excluded` and the corpus set |
| R-8 | NOTE | `TestRecordTreesAreExcludedFromCitationPolicing` stays in `links_test.go` calling `citation.Excluded`; it pins the list values and never proved the sweep consults the list (nor did the old version) | `internal/le/doc/check/links_test.go` | Accepted: sound, partly overlaps `TestPolicedKeepsCorpusFilesUnderRecordTrees` |

### Round 5 (scope: 71b1772696, the round 4 fixes)

Verdict 0 BLOCKER, 1 ISSUE, 6 NOTE.

| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| F-1 | ISSUE | `cmd/ze/hub/service_mcp_rfc9325_test.go` would be proposed as `rfc9728_service_mcp_rfc9325_test.go`, naming an RFC the file does not tag | Phase 2 | Fixed: hand row `rfc9728_service_mcp_tls_suites_test.go` |
| F-2 | NOTE | No test killed the mutant that moves the `rfc_<word>` arm before the draft gate | `internal/le/rfc/names.go` `stemSpellingAt` | Fixed: case `c/sf/foo_rfc_v5_test.go` (sflow-v5) wants `sflow_v5_foo_rfc_v5_test.go`; red under the overlay mutant (target `sflow_v5_foo_test.go`), green on the tree |
| F-3 | NOTE | No test killed the mutant `len(rest) >= 2` to `>= 1` in the `rfc_<word>` arm | `internal/le/rfc/names.go` `stemSpellingAt` | Fixed: case `c/x/foo_rfc_test.go` (a draft stem); red under the overlay mutant (index out of range), green on the tree |
| F-4 | NOTE | The armed check's message for a taken bare target needs its own wording | `internal/le/rfc/names.go` `checkTestFileNames` | Deferred to Phase 3 and folded with R-4, because the wording belongs to the armed check |
| F-5 | NOTE | Rounds still said 2, and the R-2 row said three mup cases go red | this section | Fixed |
| F-6 | NOTE | `bin/le` built before a commit ran without a stale warning | `./le` launcher | Recorded: one row in `plan/journal/` |
| F-7 | NOTE | `config_paths_limit_test.go` would be proposed as `draft_abraitis_idr_addpath_paths_limit_config_paths_limit_test.go` | Phase 2 | Fixed: hand rows for it and for `session_paths_limit_test.go`, which has the same shape |

The two test-only edits need no fixture re-seal, because the fixture digest skips `_test.go`.

## Phase 2 progress

| Component | Pairs | SHA | Deferred |
|-----------|-------|-----|----------|
| `internal/component/gtsm` | 1 | 0a32d2fa12 | none |
| `internal/component/iface` | 1 | 4c0b32a9f9 | none |
| `internal/component/resolve` | 1 | 3f179b497c | none |
| `internal/plugins/dhcpserver` | 1 | df534fa193 | none |
| `internal/plugins/tftpserver` | 1 | e85f2c8f66 | none |
| `internal/plugins/geodns` | 2 | b370d44b11 | none |
| `internal/plugins/mrt` | 2 | a47fb81e0a | none |
| `internal/plugins/fib` | 3 | 31636a09f5 | none |
| `internal/core/eap` | 5 | 9696844f4e | none |
| `internal/plugins/flowspec-firewall` | 1 | 052811c772 | none |
| `internal/plugins/iface` | 2 | e0854e9eb8 | none |
| `internal/component/tacacs` | 4 (1 from the collision table) | 489a1b53a5 | none |
| `internal/component/ike` | 16 (4 from the collision table) | e52260d978 | none |
| reseal of the verdicts the comment edits shifted | - | 3168d066aa | - |
| tool fix D-1 to D-3 (inner stem, record trees) | - | fd31c539ce | - |
| re-rename of the three doubled names | 3 | 6194de918a | none |
| round 4 review fixes R-1 to R-5 | - | 71b1772696 | - |
| round 5 record, F-2 and F-3 test cases | - | f37a0f3312 | - |
| `internal/component/bfd` | 6 | 9135d29dfc | none |
| `internal/component/mcp` | 6 (2 from the collision table) | 6f3a48c1d3 | none |
| `internal/plugins/ldp` | 6 | bc6a6d93a3 | none |
| `internal/plugins/vrrp` | 8 | 02bb8f13db | none |
| reseal (vrrp) | - | a70ab07289 | - |
| `cmd/ze/hub` | 4 (1 from the collision table) | efd82c0ee2 | none |
| `internal/core/bgp` | 9 (2 from the collision table) | f4f9ae8a09 | none |
| `internal/component/config` | 16 (2 from the collision table) | 1ce82bbf63 | none |
| reseal (core bgp, config) | - | a9bf78cb0f | - |
| `internal/component/plugin` | 1 | f0c371d371 | none |
| `internal/core/network` | 3 | be7b56a1bd | none |
| `internal/component/radius` | 6 | 3418bb3bf5 | none |
| `internal/plugins/rsvpte` | 21 (4 from the collision table) | d9e0b9595d | none |
| `internal/plugins/flowexport` | 34 (2 from the collision table) | 205417fd68 | none |
| reseal (network, rsvpte, flowexport) | - | 68c3b74651 | - |
| `internal/plugins/isis` | 39 (5 from the collision table) | 7768f39f9b | none |
| `internal/component/l2tp` | 49 (1 from the collision table) | 1f4e1e81f1 | none |
| reseal (isis, l2tp) | - | 1c08e1dbf9 | - |
| `internal/plugins/ospf` | 78 (1 from the collision table) | b980836600 | none |
| reseal (ospf) | - | 59531a714b | - |
| `internal/component/bgp/reactor` | 62 (2 from the collision table) | e36b833d2c | none |
| reseal (bgp reactor) | - | 15b956183b | - |
| `internal/component/bgp/plugins` | 89 (5 from the collision table) | e8ef205181 | none |
| reseal (bgp plugins) | - | c58abed1c5 | - |
| `internal/component/bgp`, the rest (cli, config, fsm, grmarker, message, rib, route, server, wireu) | 25 | 6dbb2272f6 | none |

Every move is R100 in `git show -M`, and no commit carries an `RFC-approved:` trailer. `./le rfc check` stood at 135 violations before the first rename and 94 after 3168d066aa, none new; it is 94 after 6dbb2272f6, none new. `./le doc check links` stayed at its 12 pre-existing broken references. **Phase 2 is complete:** a whole-tree `./le rfc rename propose` after 6dbb2272f6 writes 0 pairs and leaves out only the 6 name/tag mismatches of the Phase 3 table.

→ Constraint (Phase 2, bgp): three mentions stay on old names because the file is generated or history: `docs/features/test-health.md` and `test/health/latest.json` (`gr_egress_test.go`; `./le test health update` regenerates both) and `plan/learned/HOOK-FRICTION.md`. `internal/component/bgp/reactor/session_test.go` keeps `message/attr_discard_test.go` inside the tagged unit `TestSessionRFC7606AttributeDiscardContinues`; `test/plugin/dynamic-peer-gets-group-role-capability.ci`, `llgr-egress-state-unloaded.ci`, `bgp-rs-relay-aspath-transparency.ci`, `prefixsid-ebgp-egress-boundary.ci` and `test/reload/signal-stop-cease.ci` carry file-scoped tags and keep theirs.
→ Constraint (Phase 2, bgp): `internal/component/bgp/reactor` is red at HEAD c39f0b9c38 in 10 tests, identically before and after the renames (checked in a `git archive` export of HEAD): two RFC 2545 tests need `fd00::2` on the loopback, three are the retained defect probes of 699f4b296f, five are the startup race journalled in 7f56a16124. `plugins/rib` has one retained probe red (`rfc8277_addpath_comparable_red_test.go`).

→ Constraint (Phase 2, second batch): a `.ci` file whose tag is file-scoped is a tagged unit as a whole, so a comment edit anywhere in it stales the verdict. `test/l2tp/rfc2661-sccrq-mandatory-avp.ci` and `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` keep naming `reactor_sccrq_mandatory_avp_test.go` and `reactor_sccrq_zero_tid_test.go` in a comment, though both moved in 1f4e1e81f1.
→ Constraint (Phase 2, second batch): a plain mention reported for a moved base name is often a file of the same name in another directory (`config_test.go`, `fsm_test.go`, `packet/checksum_test.go`). Each mention was applied only where its path, or its directory when it gives none, is the moved file's.

→ Constraint (Phase 2): a comment edit in another tagged test file shifts its audit verdicts (`./le rfc reseal` re-stamps them), and one INSIDE a tagged unit stales the verdict, so a mention inside a tagged unit is left on the old name.
→ Constraint (Phase 2, fixed by D-2 and D-3): `./le rfc rename` rewrote citations in `test/weakened/<session>.md`, which `./le commit create` refuses to carry for another session and the link sweep exempts as history; those lines were restored to HEAD. It also rewrote journal rows over the 600-character cap, which `./le commit create` then refuses (`row-too-long`).
→ Constraint (Phase 2, fixed by D-1): `propose` kept an inner stem, so `gtsm_rfc5082_linux_test.go` became `rfc5082_gtsm_rfc5082_linux_test.go` (155 of the 494 targets). Three landed names carried it (`internal/component/gtsm`, `internal/component/iface`, `internal/plugins/dhcpserver`) and are re-renamed by the fixed tool in the commit after the fix.
→ Decision (D-1, main thread, 2026-10-03): `judgeTestFileName`'s repair topic drops every `_`-delimited spelling of the target stem (`topicForStem`): the stem itself, the legacy `rfc_<stem>`, and for a draft the legacy `rfc_draft_<word>` where the word is one of the draft name's own. The trailing elements `go/build` reads as a GOOS/GOARCH suffix are never removed, judged by `platformsBuilding` over a platform naming no OS and no arch, so no suffix list is copied. A topic left empty names the bare `<stem>_test.go` only when that path is free, and keeps the old topic otherwise. Test: `TestJudgeTestFileNameDropsTheInnerStem`, `TestBuildSuffixTokenFollowsGoBuild`.
→ Decision (review of fd31c539ce, R-1, replaces the bare-target fallback in the decision above): a topic left empty names the bare `<stem>_test.go` whether or not it exists. The fallback to the old topic put back the doubled stem it existed to remove (`rfc_draft_abraitis_test.go` would become `draft_abraitis_bgp_version_capability_rfc_draft_abraitis_test.go`). A taken bare target is now left out by `propose` as a collision and takes a hand-chosen topic. `judgeTestFileName` lost its `taken` parameter and `existsIn` was deleted, because nothing else asked. Test: `TestRenameProposeReportsTakenBareTarget`.
→ Decision (review of fd31c539ce, R-2): a draft's legacy `rfc_<word>` is a fourth spelling of its stem, where the word is one of the draft name's own after `draft` (`rfc_mup` for `draft-ietf-bess-mup-safi`). A lone draft word with no `rfc_` before it is not a spelling, as for every other stem, so `rfc_mup_safi_test.go` proposes `draft_ietf_bess_mup_safi_safi_test.go`. Accepted: the second `safi` is the file's topic (the SAFI value tests), not a repeat of the stem. Test: `TestJudgeTestFileNameDropsTheInnerStem` (the three mup files).
→ Decision (D-2 and D-3, main thread, 2026-10-03): the rename rewrites citations, and lists stale lines and plain mentions, only in a file the link sweep polices. The one declaration moved from `internal/le/doc/check/links.go` (`citationExcludes`, `markdownGlobs`) into the leaf `internal/le/doc/citation/policed.go` (`Excluded`, `CorpusGlobs`, `Policed`), which both the sweep and `planCitations` read; the sweep's behaviour is unchanged and its exclusion test stays in `links_test.go`, now reading `citation.Excluded`. Test: `TestRenameLeavesUnpolicedRecordsUntouched`, `TestPolicedKeepsCorpusFilesUnderRecordTrees`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling, the section is deleted)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: no `.ci` suite drives `./le rfc`; the real-tree runs in the Functional Tests table stand in)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-rfc-test-file-naming.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
