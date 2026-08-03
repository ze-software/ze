# Spec: fixit-ci-peer-block-silent-directives -- a reject= directive inside a stdin=peer block asserts nothing

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/learned/1321-wire-edit-5-fanout-dedup.md` on 2026-08-02
(`ai/rules/planning.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

A `reject=` directive written inside a `stdin=peer` block of a `.ci` test is a
SILENT NO-OP. Neither the runner's peer-block parser nor the peer expectation
reader consumes it, so the line parses into nothing while reading as a guard: the
test appears to check a negative it never checks.

**The diagnosis is WIDER than first recorded (re-verified 2026-08-02).** There
are two separate defects, and the second one is the reason the first is invisible.

| # | Defect | Evidence |
|---|--------|----------|
| 1 | `reject=bgp:` does not exist as a directive ANYWHERE. `parseReject` (`internal/test/runner/record_parse.go`) handles `stderr`, `syslog` and `stdout`, and its `default` returns `unknown reject type %q`. There is no `bgp` case. | read on 2026-08-02 |
| 2 | Inside a `stdin=peer` block the line never reaches `parseReject`. `consumes` (`internal/test/peer/expect.go`) returns true only for `expect=bgp` and for `action=` of type notification, send, rewrite, close, sighup or sigterm. `reject` is false, and `ConsumesLine`'s own doc comment names `reject=` as a runner-only directive. | read on 2026-08-02 |

Defect 2 MASKS defect 1. A `reject=bgp:` line outside a peer block would be a
hard parse error today. Every site that carries one carries it inside a peer
block, which is exactly why nobody has seen the error.

Three live sites carry a dropped `reject=` today, and they are not all the same
shape:

| Site | Directive | Which defect |
|------|-----------|--------------|
| `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | `reject=bgp:conn=2:pattern=6304DEADBEEF` | 1 and 2 |
| `test/plugin/filter-family-export-flowspec.ci` | `reject=bgp:conn=1:pattern=01180A010003` | 1 and 2 |
| `test/plugin/logging-level-filter.ci` | `reject=stderr:pattern=level=DEBUG` | 2 only: the type is real, the POSITION is wrong |

The third site is the sharpest illustration. The same file carries a comment
explaining that `option=env` "must live OUTSIDE the stdin=peer block" because the
runner consumes it, and then places a runner-consumed `reject=stderr:` inside the
block anyway. The author knew the rule and the file still shipped a dead line,
because nothing said so.

The fix is a runner GUARD that hard-errors on `reject=` inside a peer block,
matching the precedent already in place for `option=env:`, plus an audit of the
three sites once the guard fires. Patching the three call sites alone would leave
the next one silent, which is the failure this spec exists to stop
(`ai/rules/evidence.md`).

Whether `reject=bgp:` should EXIST is a second question this spec must answer.
The two sites that use it want to assert that a peer never received given bytes.
That is a real assertion with no directive behind it. **`plan/learned/1321-wire-edit-5-fanout-dedup.md`
recorded AC-1 and AC-2 as inexpressible with today's directives for this exact
reason**, so the choice is either to implement `reject=bgp:` in ze-peer or to
state that a negative wire assertion is out of the harness's reach and record how
those ACs are proven instead.

Found by an independent review of the wire-edit children on 2026-08-02. No RFC
claim rests on the dead lines: at each site the surrounding `expect=bgp:` framing
assertion still proves the behavior in the observed framing.

### Second directive of the same class: `tmpfs=<path>:mode=<octal>` is dropped

Added 2026-08-03 from `spec-finish-ci-coverage`, which met it while writing
`test/parse/cli-generate-wireguard-keypair.ci`. The syntax is documented in
`ai/patterns/functional-test.md` and `docs/architecture/testing/ci-format.md`,
`tmpfs.Parse` (`internal/test/tmpfs/tmpfs.go`) validates the octal and stores it
on `File.Mode`, and `Tmpfs.WriteTo` honours it. Nothing else does.

| Where the mode dies | Effect |
|---------------------|--------|
| `parsingRunner.setupWorkDir` (`internal/test/runner/parsing.go`) | writes EVERY tmpfs file `0o644`. No fixture in the parse suite can be executable, whatever the author declared |
| `runner_exec.go` -> `Tmpfs.AddFile` (`internal/test/tmpfs/tmpfs.go`) | re-derives the mode from the file EXTENSION via `defaultModeForPath`. A declared `mode=` is discarded; `.sh`/`.py`/`.run` happen to get `0o755`, and anything else, a fixture that must be named `wg` for a PATH lookup included, gets `0o644` |

The cause is upstream of both writers: the runner flattens the parsed files into
`map[string][]byte` (`Record.TmpfsFiles`, `parsingTest.TmpfsFiles`), so the mode
is gone before either writer runs. Same shape as the `reject=` defect above: the
author writes a directive, the parser accepts it, and it changes nothing.

A related second limit, found the same way and belonging with it: a helper script
run by the parse suite cannot invoke `ze` at all. `runOneCommand`
(`internal/test/runner/parsing.go`) rewrites a leading `ze ` in the `exec=` string
to the absolute binary path, but builds the child environment with `childEnv`,
which does NOT add `Runner.childPathEnv` the way `runner_exec.go` does. A `.sh`
helper therefore gets `ze: not found`, while the same helper works in every suite
on the orchestrated path.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/evidence.md` - a directive that neither denies nor speaks does not exist
- [ ] `ai/patterns/functional-test.md` - `.ci` directive vocabulary

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/test/runner/record_parse.go` - `parseReject` handles stderr, syslog and stdout; its `default` returns `unknown reject type`. The peer-block loop above it already refuses `option=env:`
- [ ] `internal/test/peer/expect.go` - `consumes` and `ConsumesLine`; both answer false for `reject`, and the doc comment says so deliberately

**Behavior to preserve:** every currently-passing `.ci` must keep passing once the guard fires; a site that genuinely wants a rejection assertion gets one that works, not a deleted line.

## Data Flow (MANDATORY)

### Entry Point
A `.ci` file containing `reject=` between `stdin=peer:` and its terminator.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three known sites are the only ones; a tree-wide scan finds no fourth. | Review scan of 2026-08-02 over `test/**/*.ci`, re-run the same day: two `reject=bgp:` and one `reject=stderr:` inside a peer block. | The audit list grows; the guard is unchanged. | grep the corpus again immediately before implementing | unvalidated |
| A-2 | No currently-green `.ci` depends on a `reject=` inside a peer block being ignored. | The directive asserts a negative, so dropping it can only ever have widened what passes. | A test goes red on the guard and must be fixed, not exempted. | run the full functional suite once the guard fires | unvalidated |
| A-3 | A negative wire assertion is implementable in ze-peer. | `expect=bgp:` already matches wire bytes per connection, so the machinery for comparison exists. | AC-4 takes its second branch and the two sites are rewritten around a positive assertion. | read `internal/test/peer/expect.go` before choosing the branch | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The guard fires on a site whose author intended the assertion, turning a quiet gap into a red suite. That is the point, but it must be fixed at the same time, not left red. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A `.ci` with `reject=` inside a peer block | -> | the runner's peer-block parser hard-errors | a fixture `.ci` under `test/draft/` that must fail to parse |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `.ci` with `reject=` inside a `stdin=peer` block | The runner refuses the file with an error naming the directive and the line |
| AC-2 | The three known sites | Each either carries a working rejection assertion or has the dead line removed with a stated reason |
| AC-3 | The whole `.ci` corpus | No other site carries a silently dropped directive |
| AC-4 | A `reject=bgp:` line anywhere | Either ze-peer implements it and a fixture proves it fails when the rejected bytes ARE sent, or the directive is documented as unavailable and the two sites using it are rewritten |
| AC-5 | `plan/learned/1321-wire-edit-5-fanout-dedup.md`'s AC-1 and AC-2 | Recorded as either now expressible (with the test that proves them) or permanently out of the harness's reach (with the reason), in `plan/learned/1321-wire-edit-5-fanout-dedup.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestPeerBlockRefusesRejectDirective | `internal/test/runner/record_parse_test.go` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| a draft `.ci` carrying `reject=` in a peer block | `test/draft/plugin/` | the runner refuses the file instead of dropping the directive | |

## Files to Modify
- `internal/test/runner/record_parse.go` - the guard that hard-errors on a dropped `reject=`
- `docs/architecture/testing/ci-format.md` - document that a peer block refuses `reject=`
- `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci`
- `test/plugin/filter-family-export-flowspec.ci`
- `test/plugin/logging-level-filter.ci`

## Implementation Steps

1. (fill during design)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both defects addressed: the missing `bgp` reject type AND the peer-block drop. Fixing only the second leaves `reject=bgp:` a hard parse error waiting for the first author who writes it outside a block |
| Correctness | The guard names the directive and the line, and it fires at PARSE time, before any process starts (`ai/rules/cli.md`) |
| Rule: `ai/rules/evidence.md` | A directive that neither denies nor speaks does not exist. `consumes` and the peer-block loop stay one decision, as the doc comment demands |
| Rule: `ai/rules/testing.md` | A dead line is removed only with a stated reason. It is never removed to quiet the new guard |
| Registration over hardcoding | The guard derives its accepted directive set from the parser, not from a second hand-written list that can drift from `consumes` |

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
