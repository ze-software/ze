# 1226 -- test-health-dashboard

Built a generated report that answers "would a regression be caught?" about ze's
own test suite, plus two ratchets in `ze-verify`: an AST gate that finds tests
which cannot fail and test files no `go test` target builds, and a per-metric
floor for the higher-is-better ratios. The report is `docs/features/test-health.md`
(committed, mirrored to the website at `quality/health/`).

## What was dark and why it mattered

Three separate green summaries were each true in isolation and collectively
misleading:

1. The site advertised "121,500+ unit tests" -- 76% of which were
   `gokrazy/modcache` third-party dependency tests. Real figure ~19,900.
2. `ai/RFC-REQUIREMENTS.md` headlined "0 MUST-level requirements still owe work",
   which reads as 100% compliant. Only 35.7% are proven by a test PAIR; the rest
   are annotated `{not-applicable}` / `{gap}` / `{single-polarity}`.
3. An all-green `ze-verify` says nothing about whether the tests that ran could
   have failed.

No count of tests distinguishes "large suite" from "sensitive suite". The whole
feature exists to decompress those green numbers into what they actually assert.

## Decisions worth keeping

- **The ratchet must not depend on the report.** `ze-test-sensitivity-check`
  (stage 10) reads only `sensitivity-baseline.json` and the tree. This is what
  let a later design change (below) ungate the report's volume counters without
  weakening any guarantee.
- **Gate events, publish churn.** A byte-exact staleness gate over the whole
  report charged a regenerate-and-commit to ~60% of commits, because every added
  test moves a denominator. That is the *"advisory gate permanently red"* failure
  the report itself is built to detect: a check that fires that often for
  cosmetic reasons gets routed around, not read. `do_check` now compares only
  STRUCTURAL facts (orphan list, unproven RFCs, metric statuses); counters are
  published and refreshed by `make ze-regen`.
- **Status as a regression signal, not a target.** Three bare thresholds
  (50/75/40) put five rows in the attention table permanently -- the same
  green-wall failure from the other side. Replaced with a `quality-baseline.json`
  that ratchets UP: a higher-is-better metric warns only when it drops below its
  locked-in best. The absolute number stays visible in the metric's card.
- **The report is a pure function of committed state**, from `git ls-files`, so
  an untracked scratch test does not move it -- but file CONTENTS are read from
  the working tree, so an uncommitted edit does. State both limits; do not
  overclaim "reproducible from any checkout".
- **Detectors in Go, aggregation in Python**, matching the two existing
  conventions (`scripts/checks/*.go` with `--selftest`, `scripts/dev/*.py` for
  report generation). AST parsing beats regex for "does this test assert".

## Traps hit, recorded so the next agent does not repeat them

- **A "fix" can over-correct into the opposite bug.** Narrowing a
  negative-test regex that over-matched setup guards, I narrowed it so far it
  missed the project's plain-Go house style (`if err == nil { t.Fatal }`) and
  halved the figure (418 vs 828), publishing "0/31" for subsystems with 13 files
  of rejection tests. Over-correction is still a correctness bug.
- **A detector heuristic keyed on a NAME is porous.** `isAssertionImport`
  matching `"/is"` as a substring exempted all of `internal/plugins/isis/**`
  (143 tests). `failureSelectors` matching a method name with no receiver check
  credited `fmt.Errorf(...)`. Match the final path element; discriminate on the
  receiver.
- **A guard tested only via its helper is not tested.** Several "fails closed"
  tests called `tighten_baseline` directly and passed, while the real entry point
  crashed with a traceback on the same input because a DIFFERENT collector read
  the file first. Drive fail-closed guards from the entry point.
- **Recording a fix you did not verify is worse than no record.** Two `str.replace`
  edits silently no-opped after a formatter reflowed their targets, and I wrote
  "fixed" into the Review Gate without re-grepping. A false review record stops
  the next reviewer looking. Grep for the new text before recording any scripted
  edit as done. (Escalation candidate -- this is a process trap, not a code one.)
- **A ratchet that never lowers cannot un-lock a mistaken high floor.** A wrong
  `quality-baseline.json` value can only be corrected by deleting the file and
  re-bootstrapping. Same trade as any ratchet; document it.

## Risks & observations carried forward

- The assert-nothing detector has documented blind spots (helper chains > 1 deep,
  cross-package helpers, `var _ int`, dead-code `panic`). The escape annotation is
  unratcheted. These are recorded in the spec's Known Limitations, not fixed.
- The 34 site-renderer tests skip in CI (`.woodpecker/verify.yml` checks out only
  `main`). Inherent to the two-repo split; they run on a developer's machine.

## Reviewed

Five review rounds (four adversarial subagent passes plus one clean-room),
recorded in the spec's Review Gate: ~58 findings, including 6 that were defects in
a previous round's fixes, and one blocker in the clean-room pass. The numbers on
the page were independently recomputed and all checked out; the failures were in
the prose around them, which is why the review effort went where it did.
