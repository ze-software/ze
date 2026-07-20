# test/health

Committed artifacts behind `docs/features/test-health.md`, the generated page that
reports whether a regression would actually be caught.

The three data files are produced by `scripts/dev/testing_health.py` (this README
is hand-written). Do not hand-edit the data; edit the collectors instead.

| File | What it is | Written by |
|------|-----------|-----------|
| `latest.json` | Every metric, with each ratio's numerator and denominator | `make ze-test-health` |
| `history.ndjson` | Append-only KPI series, one row per recorded sample | `make ze-test-health-record` |
| `sensitivity-baseline.json` | Ratchet floors for the assert-nothing and tag-orphan counts (lower is better; ratchets DOWN) | `make ze-test-health` (tightens only) |
| `quality-baseline.json` | Locked-in best for the higher-is-better ratios (proof density, mutation kill, negative-test share); a metric warns only when it drops below its best, so the attention table shows regressions, not a permanent gap to an arbitrary target. Ratchets UP | `make ze-test-health` |

## What is gated, and what is only published

`make ze-test-health-check` compares the STRUCTURAL facts recorded in
`latest.json` against the tree: which test files no `go test` target can build,
which enrolled RFCs have no test pair, and every metric's status. Each of those
changing is an event worth stopping a commit for. A status flipping to `unknown`
in particular means a collector stopped measuring, and sensor rot is the failure
this whole report exists to make visible.

The volume counters are published, not gated. Byte-comparing the whole report
charged a regenerate-and-commit to roughly 60% of commits, because every added
test moves a denominator, and a check that fires that often for cosmetic reasons
gets routed around instead of read. The counters are refreshed by
`make ze-regen` and may lag the tree by a few tests; the page says so.

**The ratchets do not depend on any of this.** `make ze-test-sensitivity-check`
enforces them from the tree itself, reading only `sensitivity-baseline.json`, at
stage 10 of `ze-verify`. Report staleness cannot weaken them.

## Why the report is reproducible

Which files are counted comes from git's index, and the Markdown embeds no
wall-clock value and no host path, so an untracked scratch test does not move the
published numbers and regenerating at the same commit with a clean tree
reproduces the page byte for byte.

Two limits worth stating plainly, because the earlier wording overclaimed: file
CONTENTS are read from the working tree, so an uncommitted edit to a tracked test
does move the counts; and `git ls-files` reads the INDEX, so `git add` of a new
test moves them before any commit. Regenerate after staging.

The ratchet deliberately differs: `ze-test-sensitivity-check` scans the WORKING
TREE, so an inert test is caught by the verify run before its commit rather than
by the next, unrelated one. The two populations therefore differ by whatever is
currently uncommitted, which is intended. That is what
lets `make ze-test-health-check` gate the committed page for staleness, the way
every other generated file in this repository is gated. It also removes the
sensor-rot failure, where a collector broke weeks ago and the page still shows
its last green.

The consequence: a metric that needs a live test run cannot go straight onto the
page. `--record` appends it to `history.ndjson` (committed), and the page renders
trends from there. `scripts/dev/mutation_history.py` established the same pattern
for mutation scores.

## The ratchets

`sensitivity-baseline.json` holds floors, not facts. `make ze-test-sensitivity-check`
(stage 10 of `make ze-verify`, both modes) fails when a count rises above its floor
and names every offending file.

`make ze-test-health` only ever *lowers* a floor. Writing a higher one would let a
regression be laundered into the baseline by running the generator, so the floor
falls when the debt is paid and never rises. This follows `test/.ci-sleep-baseline`.

To resolve a failure, fix the test. To record that a test genuinely has no runtime
assertion (a must-not-panic smoke test, where the oracle is the absence of a
panic), annotate it:

```go
// test-asserts-nothing: the oracle is the absence of a panic
func TestCloseWithNoBundle(t *testing.T) { ... }
```

## Commands

| Command | Does |
|---------|------|
| `make ze-test-health` | Regenerate the page, `latest.json`, and the baseline |
| `make ze-test-health-check` | Fail if the committed page is stale (runs inside `ze-regen-check-readonly`) |
| `make ze-test-health-record` | Append one KPI sample, then regenerate the page |
| `make ze-test-sensitivity-check` | Enforce the ratchets (runs inside `ze-verify`) |
| `go run scripts/checks/inert_tests.go` | Human-readable list of every inert and orphaned test |

The website renders the same data at `quality/health/` via
`../gh-pages/tools/render-test-health.py`, which computes nothing of its own.
