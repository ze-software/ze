# 1227 -- fixit-verify-stage-ssot

Made the `make ze-verify` stage list a single source of truth: deleted the dead
duplicate Makefile stage lists, replaced the 4-name subset guard with an ordered
golden over BOTH mode branches, wired the never-running generated-file staleness
check into verify as a WRITE-SAFE variant, gave the unwired `ze-yang-glue-check`
a feeder, and dropped a `commit_helper.py` gate entry that could never match.

## What was dark and why it mattered

Four copies of "what verify runs" existed, and they had already diverged:

1. `stagesForMode` (`scripts/status/verify_run.go`) -- the LIVE list. Both
   `make ze-verify` and `make ze-verify-changed` shell out to this runner, and
   `.woodpecker/verify.yml`'s only step is `make ze-verify`. A gate absent here
   runs NOWHERE.
2. `_ze-verify-impl` / `_ze-verify-changed-impl` (Makefile) -- zero callers, but
   they read exactly like the stage list, so they were the natural-looking place
   to add a gate. Both still listed `ze-cli-grammar-check`, which the live list
   has never run.
3. `STRUCTURAL_GATES` (`commit_helper.py`) -- matched against the `stage` field
   of `tmp/ze-verify-failures.json`, which verify_run.go fills from
   `stagesForMode`. Its `ze-cli-grammar-check` entry could therefore never match:
   a dead entry that read as a live safety net.

The guard that was supposed to catch this,
`TestStagesForModeIncludesStaticAnalysisGates`, was a positive subset check of 4
names. A subset check cannot see a dropped stage, and cannot see the two hand
-duplicated mode branches drifting apart.

Separately `ze-yang-glue-check` and `ze-regen-check` were defined-but-unwired.
The yang one is the nastier of the two: a `.yang` added without `make generate`
leaves a stale `register.go`, the module is silently never wired, and nothing
errors -- the schema simply is not there.

## Key decisions / gotchas

- **The golden is a hand-maintained literal, deliberately.** `R-2` in the spec
  suggested deriving it from `stagesForMode` to avoid churn; that would compare
  the function to itself and assert nothing. Its whole job is to force a
  two-place, deliberate edit. Comparison is ORDERED, which is stronger than the
  AC's set-equality: stage order is load-bearing (cheap static gates before the
  expensive test stages, so a red surfaces fast).

- **`ze-regen-check` cannot be wired into verify, and the reason is not
  obvious.** It reads like a check, but it is `ze-regen-check: ze-regen` -- the
  prerequisite REWRITES every generated file and only then runs `git diff
  --quiet`. Putting it in verify makes `make ze-verify` leave a dirty working
  tree whenever anything is stale. The wired stage is a new write-safe twin,
  `ze-regen-check-readonly`, composed only of the generators' own `--check`
  modes (regenerate in memory, diff, never write). Proven write-safe by
  snapshotting `git status --porcelain` either side of a run.

- **Two generators were covered ONLY by the mutating `git diff`.** Rebuilding
  the check read-only is not a matter of copying the recipe's existing `--check`
  lines: `feature_tags.go` (`.golangci.yml`, `gokrazy/ze/config.json`,
  `docs/guide/quickstart.md`) and `fuzz-targets.py` (`mk/test-fuzz-targets.mk`)
  had `--check` modes that the recipe never called -- the `git diff` covered
  their outputs instead. Dropping the diff without adding those two calls would
  have silently reduced coverage. The generator -> output -> check map is now
  written out above the target so the next edit can be checked against it.

- **The new stage found a real red on its first run**: `plan/learned/.counter`
  was 1221 with the highest summary at 1221 (must be >= highest+1). Nothing had
  ever run that check in CI.

- **A gitignored output cannot be a CI gate.** The first cut wired
  `skill_sync.sh --check` into the verify stage. Every one of its targets
  (`CLAUDE.md`, `AGENTS.md`, the three skill mirrors) is gitignored, so on the
  fresh checkout CI runs `make ze-verify` against, they do not exist and the
  check exits 1 -- it would have reddened every CI run from the first push. The
  general rule: before wiring a `--check` into verify, ask whether its target is
  in `git ls-files`. If it is not, nothing committed can drift and CI has nothing
  to catch; the guard belongs in a local/session hook, which is exactly where
  this one already lived.
  Reproducing this needs care: `skill_sync.sh` starts with
  `cd "$(git rev-parse --show-toplevel)"`, so a `git archive HEAD` tree unpacked
  *inside* the repo resolves back to the real root and the test passes,
  falsely. `git init` in the extracted tree first. My first repro said "fine";
  the reviewer's said "broken"; the reviewer was right.

- **Check mode that never reads the file it generates is a fail-open guard.**
  `code_to_docs.py --check` built the output in memory and then validated only
  that the anchor paths existed -- it never compared against
  `ai/CODE-TO-DOCS.md`. It printed "all references valid", exit 0, while the file
  was stale by 24 code paths (1439 recorded vs 1463 live), and had been for who
  knows how long, because the only gate that would have caught it
  (`ze-regen-check`'s `git diff`) has no callers. The tell was in the output all
  along: every sibling check prints "`<file> up to date`"; this one printed
  counts and never named its own output. When auditing a `--check`, grep for a
  read of the output path; if there is none, it is checking something else.

- **`go test` caching can serve a stale PASS for a codegen feeder.** A `.yang`
  file is not an input of `internal/component/plugin/all`, so after adding a
  stray `.yang` the feeder returned a cached `ok`; only `-count=1` went red. The
  full-verify stage is `ze-unit-test-cached`, so this is not theoretical. The
  feeder is the fast LOCAL signal; the uncached backstop is the
  `ze-regen-check-readonly` make recipe running the same `--check`. This applies
  equally to the pre-existing `TestGeneratedPluginImportsCurrent`. Prove any
  codegen feeder non-vacuous with `-count=1`, or you are testing the cache.

- **Where the Python assertion had to live.** `scripts/dev/*_test.py` are run
  under `go test` by `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`),
  so `commit_helper_test.py` IS wired -- but that is worth checking before
  putting a gate in a `.py` test, because nothing else in the repo executes
  them. It derives the live stage names by parsing `mk("...")` out of
  `stagesForMode` rather than restating the list, and asserts the parse found
  >10 stages so an empty parse cannot pass vacuously.

- **Deleting a Makefile target has doc-gate fallout.** Two
  `<!-- source: Makefile -- _ze-verify-impl -->` markers in
  `docs/contributing/documentation-testing.md` pointed at the deleted targets;
  repointed to `scripts/status/verify_run.go -- stagesForMode`. `code_to_docs.py
  --check` validates the path, not the symbol, so it would not have caught the
  stale symbol on its own.

- **A per-branch golden cannot detect divergence between branches.** Two
  independent reviewers found this separately, and one proved it: with one golden
  per mode, wiring a gate into only `ze-verify` fails exactly one sub-test, and
  the obvious fix (update that golden) ships a fast dev loop that silently skips
  the gate. The failure message made it worse by saying "update the golden if
  deliberate". A golden detects *change*; detecting *divergence* needs an
  explicit cross-branch assertion plus a documented allowlist of the stages that
  are legitimately mode-specific -- and the allowlist itself needs a check that
  it has not rotted.

- **Two tests, two cache domains.** A `.py` test run through `go test` sees the
  Go files it reads only as runtime data, never as cache inputs. The Python
  `STRUCTURAL_GATES` check reads `verify_run.go`, so editing the stage list alone
  serves it a cached PASS (and `changed-pkgs.sh` maps a `*.go` edit to
  `./scripts/status`, never `./scripts/dev`). The fix is a Go twin in
  `scripts/status` that calls `stagesForMode` in-process. Neither test is
  redundant: an edit to `commit_helper.py` invalidates `./scripts/dev` and only
  the Python one re-runs. Pair them and say so in both comments.

- **A fix to a fail-open guard can itself be a fail-CLOSED-forever guard.** The
  round-1 fix to `code_to_docs.py --check` compared the freshly-built index
  against the file on disk. But `stale` is populated only in check mode and fed a
  `## Stale References` section into that same string, so whenever any doc anchor
  was broken, check-mode content could never equal what generate mode writes:
  the gate said "run: make ze-doc-index", and running it did not help. An
  unfixable loop on a commit-blocking gate, and it short-circuited before the
  `MISSING: <path>` report that was the tool's original purpose. Rule of thumb:
  if you add a `generated == on-disk` comparison, first check that the generated
  side is computed identically in both modes. Any `if check_mode:` branch above
  the content builder is a red flag.

- **`make -n` is not a dry run when the recipe contains `$(MAKE)`, and neither is
  `-t`.** This is the single nastiest thing in this spec. GNU make executes
  recipe lines containing `$(MAKE)` even under `-n`/`-t`/`-q` (that is how
  recursive make participates in those modes), while every OTHER recipe becomes a
  no-op that succeeds. So `make -n ze-verify` really starts the verify runner,
  all 21 stage sub-makes echo-and-pass, and the runner writes
  `tmp/ze-verify.status` with `exit=0` and the current tree hash --
  `verify-status.sh` then reports FRESH and the commit gate opens on a tree
  nothing verified. `-t` is worse because it is quieter: a `.PHONY` stage prints
  "Nothing to be done" and exits 0, with no echoed recipes to hint that nothing
  ran. Measured: with `MAKEFLAGS=t`, a `.PHONY` target whose recipe is `exit 9`
  exits 0.
  If a program writes a durable "this tree is verified" record, it MUST refuse to
  run in a no-execute mode. Detect it from `MAKEFLAGS`' first whitespace-separated
  field only (GNU make concatenates short flags there with no leading dash) and
  match `n`, `t`, `q`. Do NOT use `$(findstring n,$(MAKEFLAGS))`: measured, it
  matches `--no-print-directory` and would refuse every real verify.

- **Four review rounds, and rounds 2, 3 and 4 each earned their keep.** Round 1
  found 2 BLOCKERs in the original work. Round 2 found a BLOCKER *introduced by a
  round-1 fix*. Round 3 found a BLOCKER in a DOC LINE round 2 had added. Round 4
  found that round 3's guard covered `-n` but not `-t`, which forges the same
  green state more quietly. `ai/rules/planning.md`'s "every fix is new code
  that needs a fresh pass" is not ceremony: three of the four BLOCKERs were
  introduced by fixes, all on a path that gates every commit. Budget for the loop,
  not for one review.

- **Widening a parser finds work, and that is the point.** Deepening the coverage
  walk to descend into sub-target recipes immediately surfaced seven generator
  scripts that had never been mapped to a check. Each round of "make the guard
  see more" produced real unguarded surface, never just noise.

- **Verify the reviewer, and be ready to be wrong about the verification.** My
  first attempt to reproduce the `skill_sync.sh` BLOCKER said "works fine" and I
  nearly dismissed the finding. The repro was invalid (see the git-root note
  above). Re-check the harness before doubting the reviewer.

- **A floor is not a set.** The first coverage guard asserted
  `len(prereqs) >= 4` against 8 prerequisites, so half of them -- including the
  only guard for `ai/INSTRUCTIONS.md` -- could be deleted with every test green.
  If the point is "these exact things must be present", assert the exact set in
  both directions (missing, and undocumented-extra) and attach a reason to each
  entry so the next person knows what deleting it costs.

- **GNU make permits blank lines inside a recipe.** A `(?ms)^target:.*?\n\n`
  span regex therefore truncates at a stray blank line, silently hiding
  everything below it. Parse a recipe line by line and end it at the first line
  starting at column 0.

- **Prefer prerequisite targets to re-typed recipes.** The first cut spelled the
  twelve `--check` commands out in the new target's recipe, which made a fifth
  copy of "how to check yang glue" while `ze-yang-glue-check`,
  `ze-feature-tags-check` and `ze-fuzz-targets-check` sat there with zero callers.
  Depending on those targets instead is shorter, revives them, and makes the
  SSOT claim true of the check commands and not only of the stage list.

## Files

**SSOT + guards:** `scripts/status/verify_run.go` (`ze-regen-check-readonly` in
both branches, SSOT doc comment on `stagesForMode`),
`scripts/status/verify_run_test.go` (`TestStagesForModeMatchesGolden`,
`TestStagesForModeIncludesRegenCheck`).
**Feeder:** `internal/component/plugin/all/yang_glue_check_test.go`.
**Make:** `Makefile` (deleted `_ze-verify-impl` / `_ze-verify-changed-impl` and
their `.PHONY`; added `ze-regen-check-readonly` + the generator/output/check map).
**Commit gate:** `scripts/dev/commit_helper.py` (dropped `ze-cli-grammar-check`),
`scripts/dev/commit_helper_test.py` (`test_structural_gates_are_live_stages`).
**Fallout:** `docs/contributing/documentation-testing.md`, `plan/learned/.counter`.
