# Test-Health Reporting and the Sensitivity Ratchets

How ze answers "would a regression actually be caught?" about its own test
suite, as opposed to "how many tests are there?". Those are different questions:
a suite can grow without limit while the share of behaviour a change would break
red keeps falling, and no test count shows that.

The user-facing report is [`../../features/test-health.md`](../../features/test-health.md)
(generated). This document is the architecture behind it: what is measured, what
is enforced, and where each number comes from.

## Two independent mechanisms

The feature is two things that share collectors but not enforcement.

| | Ratchets | Report |
|---|---|---|
| Question | "Did this commit make sensitivity worse?" | "What is the state of the suite?" |
| Enforced by | `./le test-sensitivity check`, stage 10 of `./le verify current mode full` | `./le test-health check`, inside `./le repository generated-check` |
| Reads | `test/health/sensitivity-baseline.json` + the working tree | the committed report vs the tree |
| Source | `internal/le/testsensitivity.Answer` | `internal/le/testhealth.Answer` |
<!-- source: internal/le/verify/run.go -- Run, RunMode -->

The ratchets do NOT depend on the report. `./le test-sensitivity check` reads only
the baseline and the tree, so a stale or wrong report cannot weaken the
guarantee that you cannot add an inert test or strand a test file.

## The sensitivity detectors

`./le test-sensitivity check` runs the native AST detectors in
`internal/le/testsensitivity`. Its `selftest` action proves each detector on
known-bad fixtures before the live-tree check is trusted.

### assert-nothing

A `Test` function with no reachable failure call: no `t.Error`/`Fatal`/`Fail` on
a testing receiver, no assertion-library call, no compile-time `var _ T = ...`
assertion, no `panic`. It executes code, moves coverage, and passes
unconditionally. Deleting the body of the function under test would not turn it
red.

Benchmarks and fuzz targets are exempt: a benchmark measures, and a fuzz target
delegates its oracle to the engine. Assertion helpers are followed one level;
import aliases are resolved so `require` under a different name still counts.

### tag-orphan

A `_test.go` file whose `//go:build` constraint is unsatisfiable under every
native test action is a tag orphan. The tag universe is derived from the Go
action tables and feature manifest, so a new feature gate cannot silently
orphan its tests. The satisfiability search handles negated and compile-out
constraints such as `!linux` and `ze_core && !ze_web`.
<!-- source: internal/le/testsensitivity/tags.go -- TagUniverse -->

### Detector pitfalls

Four traps hit while building these detectors, recorded because each produced a
green or a plausible number that was wrong:

- **A name-shaped heuristic is porous.** Matching `"/is"` as a substring for an
  assertion-library import exempted all of `internal/plugins/isis/**`, which is
  143 tests. Match the final path element. A method-name match with no receiver
  check credited `fmt.Errorf(...)` as a failure call. Discriminate on the
  receiver.
- **A correction can over-shoot into the opposite defect.** Narrowing a
  negative-test regular expression that over-matched setup guards narrowed it
  past the house style `if err == nil { t.Fatal }`, which halved the figure
  (418 against 828) and published `0/31` for subsystems holding 13 files of
  rejection tests.
- **A guard driven only through its helper is not tested.** Several
  fail-closed tests called the helper directly and passed, while the real entry
  point crashed on the same input because a different collector read the file
  first. Drive a fail-closed guard from its entry point.
- **A scripted edit is not done until it is grepped.** Two string replacements
  silently matched nothing after a formatter reflowed their targets, and the
  review record said "fixed". A false review record stops the next reviewer
  looking.

### The floors

Committed in `test/health/sensitivity-baseline.json`. Counts may only go DOWN,
following the `test/.ci-sleep-baseline` convention: lower the floor in the same
change that improves the number. A missing file or key is an error, not a fresh
floor, so deleting the baseline cannot launder a regression; first-time creation
is the explicit `--bootstrap-baseline`.

The ratchet scans the WORKING TREE, so an inert test is caught by the
`./le verify current mode full` run that precedes its commit, not blamed on the next one.

## The per-commit weakening record

A third gate, on a different failure: a test that stopped proving something with
a written excuse attached.

The compiled weakening detector in `internal/le/weakened` refuses an edit that
deletes assertions, adds a `t.Skip`, drops an `expect=`, or introduces an
assertion that cannot fail. Its escape hatch is a row in `test/weakened.md`
naming the test the edit weakens.

| | |
|---|---|
| Refused at edit time by | `internal/le/hookruntime` calling `internal/le/weakened` |
| Refused at commit time by | `weakened_problems` (`internal/le/commit.Answer`) |
| Reads | `test/weakened.md` + the HEAD content of the paths the commit names |
| Source | `internal/le/weakened.Answer`, called by both gates |
| Parse gate | `./le test-weakened check`, in `./le verify current mode full` both modes |
<!-- source: internal/le/verify/run.go -- Run, RunMode -->

**The file is replaced per commit, and that shape is the whole design.** Delete
the rows of the last commit. Write the rows of this one. Git history holds every
past row beside the change it accepted. A record that cannot accumulate cannot
become unreadable, so no ceiling and no census are needed to cap it.

**The commit must CARRY the file**, not merely have the row in the working tree.
`internal/le/commit.Answer` refuses a weakening when `test/weakened.md` is not
one of the repeated `file <path>` values passed to `./le commit create`.

**The commit gate judges the paths the commit NAMES, never the working tree**,
which is where it differs from the sensitivity ratchet above. Several sessions
share this checkout, so a tree-wide count moves under whoever reads it. It went
751 to 752 to 755 within an hour on 2026-08-10, on edits by three sessions that
had never touched this gate. A gate that reds on another session's half-finished
work is a gate that gets switched off (`ai/rules/repo-maintenance.md`, "the two
changed-file checks"). Keying on the commit's own paths makes the BLOCK tier safe
by construction.

Until 2026-08-16 the justification was a `// test-relax:` comment in the test
file, capped by a ceiling that a census counted at HEAD. Two properties of that
corpus are why the storage moved, and both are recorded in `TEST-RELAX-AUDIT.md`.

Volume destroyed the mechanism. At 751 tokens across 466 files, reading them was
no longer possible. So nobody read them, writing one cost nothing, and more got
written.

A token never expired. Three `redistribute-l2tp-*.ci` tests carried a
`KNOWN ENGINE ISSUE` justification whose claim was refuted in-place on
2026-07-16, and whose tracking spec had been closed and deleted. Both statements
sat in the file and contradicted each other for four months.

## The report

`internal/le/testhealth.Answer` aggregates ten metrics, each assigned to one of
three questions (sensitivity / intent / integrity) and each stating the action
its degradation implies. Every number is derived from committed state; which
files count comes from `git ls-files`, so an untracked scratch test does not
move the figures.

### What is gated, and what is only published

`./le test-health check` compares only the STRUCTURAL facts against the tree: the
orphaned-test-file list, the unproven-RFC list, and every metric's status. Each
of those changing is an event, and a status flipping to `unknown` means a
collector stopped measuring, which is the sensor rot the report exists to
surface.

The volume counters (test counts, ratios, adoption buckets) are published, not
gated. Byte-comparing the whole report charged a regenerate-and-commit to ~60%
of commits, because every added test moves a denominator, and a check that fires
that often for cosmetic reasons gets routed around rather than read: the
"advisory gate permanently red" failure the report is built to expose. The
counters are refreshed by `./le repository generate` and the page discloses that they may
lag.
<!-- source: internal/le/testhealth/actions.go -- Answer -->

### The KPI series

`./le test-health record` appends one row to `test/health/history.ndjson`.
`./le mutation record-history` maintains the per-package mutation series. Both
skip an identical sample at the same commit, so trends do not overstate the
number of runs.

## Publication

The native `./le site build` action renders the website health page from
`test/health/latest.json`, `history.ndjson`, and the generated Markdown. The
site builder computes no new test-health facts.
<!-- source: internal/le/sitebuild/build.go -- Build -->
<!-- source: internal/le/testhealth/actions.go -- Actions -->

## Full operator reference

`test/health/README.md`.
