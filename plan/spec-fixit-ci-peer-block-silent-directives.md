# Spec: fixit-ci-peer-block-silent-directives -- a reject= directive inside a stdin=peer block asserts nothing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-16 |

Deferral holder created at the closure of the wire-edit-5 fan-out dedup spec on 2026-08-02
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

**The site count here is the 2026-08-02 one and it is STALE. The list re-derived
on 2026-08-15 is in Risks & Assumptions, A-1: 15 dropped directives in 12 files,
not 3.** Three sites of the original shape, kept because they show the three
shapes the defect takes:

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
That is a real assertion with no directive behind it. **The wire-edit-5 fan-out
dedup spec recorded its AC-1 and AC-2 as inexpressible with today's directives
for this exact reason**, so the choice is either to implement `reject=bgp:` in ze-peer or to
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
| A-1 | The three known sites are the only ones; a tree-wide scan finds no fourth. | Review scan of 2026-08-02 over `test/**/*.ci`, re-run the same day: two `reject=bgp:` and one `reject=stderr:` inside a peer block. | The audit list grows; the guard is unchanged. | grep the corpus again immediately before implementing | **broken** (2026-08-15) |
| A-2 | No currently-green `.ci` depends on a `reject=` inside a peer block being ignored. | The directive asserts a negative, so dropping it can only ever have widened what passes. | A test goes red on the guard and must be fixed, not exempted. | run the full functional suite once the guard fires | confirmed (2026-08-15) |
| A-3 | A negative wire assertion is implementable in ze-peer. | `expect=bgp:` already matches wire bytes per connection, so the machinery for comparison exists. | AC-4 takes its second branch and the two sites are rewritten around a positive assertion. | read `internal/test/peer/expect.go` before choosing the branch | confirmed (2026-08-15) |

**A-1 broke, and the count it broke by is the reason this spec's shape changed.**
Re-derived on 2026-08-15 over every stdin block a `ze-peer` reads: 11 `reject=bgp:`
in 9 files, 1 `reject=stderr:`, 1 `option=mode:`, and 2 `expect=json:` in blocks
named something other than `peer` (`test/plugin/forward-mpreach-nexthop-self-two-peer.ci`),
so 15 dropped directives in 12 files rather than 3. Two further classes are
dropped and lose no assertion: `option=timeout` (446 lines) and `cmd=api` (235).
Patching a list of sites was never going to hold, which is why the deliverable is
the derived guard.

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
| AC-5 | AC-1 and AC-2 of the closed wire-edit-5 fan-out dedup spec | Recorded as either now expressible (with the test that proves them) or permanently out of the harness's reach (with the reason), in `docs/architecture/bgp/fanout-dedup.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestPeerBlockRefusesUnclaimedDirective | `internal/test/runner/peer_block_directive_test.go` | AC-1 | pass |
| TestPeerBlockAcceptsRunnerDirective | `internal/test/runner/peer_block_directive_test.go` | AC-3 | pass |
| TestPeerBlockAcceptsPeerDirective | `internal/test/runner/peer_block_directive_test.go` | AC-4 | pass |
| TestPeerBlockRejectNeedsDelivery | `internal/test/runner/peer_block_directive_test.go` | AC-4 | pass |
| TestPeerBlockGuardCoversEveryPeerBlock | `internal/test/runner/peer_block_directive_test.go` | AC-3 | pass |
| TestCIPeerBlockCorpusParses | `internal/test/runner/peer_block_directive_test.go` | AC-3 | pass |
| TestSplitRejectRules | `internal/test/peer/reject_test.go` | AC-4 | pass |
| TestCheckerRejectedIsByteAligned | `internal/test/peer/reject_test.go` | AC-4 | pass |
| TestCheckerRejectedIsPerConnection | `internal/test/peer/reject_test.go` | AC-4 | pass |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the 9 files carrying `reject=bgp:` | `test/plugin/` | each negative fires when the behaviour it names is broken | pass; each shown red |
| `logging-level-filter.ci` | `test/plugin/` | the `reject=stderr:` inside the peer block now fails the test when DEBUG appears | pass; shown red at `value=debug` |
| `forward-mpreach-nexthop-self-two-peer.ci` | `test/plugin/` | the two `expect=json` in `stdin=peer1`/`peer2` are enforced | pass |

## Files to Modify
- `internal/test/runner/peer_contract.go` - `validatePeerBlockDirectives`, the guard, and `validatePeerBlockRejects`, the non-vacuity rule
- `internal/test/runner/record_parse.go` - the old `expect=`/`action=`-only loop deleted; the guard called once the `cmd=` lines are known
- `internal/test/peer/expect.go` - `ClaimLine` and `Claim`, ze-peer's own answer about a line; `consumes` gains `reject=bgp`
- `internal/test/peer/reject.go` - `reject=bgp`: parse, per-connection match, enforcement in the linger loop
- `docs/architecture/testing/ci-format.md`, `ai/patterns/functional-test.md` - the peer-block contract and the new directive
- `docs/architecture/bgp/fanout-dedup.md` - AC-5
- `test/plugin/rest-peer-set-delete-lifecycle.ci` - the dropped `option=mode`
- `test/plugin/logging-level-filter.ci` - a comment; its `reject=stderr` is now live where it stands

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

---

## Implementation Summary

### What Was Implemented

- **The guard.** `validatePeerBlockDirectives` / `validateOnePeerBlock`
  (`internal/test/runner/peer_contract.go`) fails a `.ci` at PARSE time when a
  line inside a ze-peer stdin block is read by neither ze-peer nor the runner.
  The accepted set is DERIVED: `peer.ClaimLine` (`internal/test/peer/expect.go`)
  reports what ze-peer's own parser did with the line, and the runner's own
  `parseLine` answers for the rest. No third list exists to drift.
  `peerBlockNames` scopes it to every block a ze-peer reads, not to the block
  literally named `peer`, and returns them sorted so a file with two bad blocks
  names the same one on every run.
- **The directive.** `reject=bgp:conn=N:pattern=<hex>` is a real per-connection
  ze-peer directive (`internal/test/peer/reject.go`). It is never consumed:
  `Checker.rejection` is re-checked against every frame the message loop reads,
  the `option=linger` loop included. `peer.ParseRejectRule` is the ONE parser for
  the line and the runner's guard calls it, so a malformed needle fails the FILE
  rather than the peer.
- **Non-vacuity.** `validatePeerBlockRejects` refuses a rejection with no
  `expect=bgp:conn=N` delivery on the same connection, and refuses one in a block
  handed to a non-check peer. `peer.New` refuses the same thing at run time.
- **The old drop is deleted.** `record_parse.go`'s expect/action-only peer-block
  loop is gone (`ai/rules/no-layering.md`); `parseReject` gains a `bgp` case that
  names the peer block rather than answering "unknown reject type".
- **The seven orphaned blocks are repaired.** Each
  `test/plugin/redistribution-*.ci` now starts the ze-peer its `stdin=peer` block
  was written for, so every directive in that block decides the verdict. Three
  changes travel with the `cmd=background` line. A `family` block on each peer,
  because a peer that declares none negotiates none and `sendInitialRoutes`
  (`reactor/peer_initial_sync.go`) then emits no End-of-RIB for the block to
  assert. A callback pump before any poll, because only `API.read_line`
  (`test/scripts/ze_api.py`) answers ze's filter-verdict request. And a second
  `ze-peer --bind 127.0.0.2` on the two export fixtures, because with one peer
  nothing is forwarded and an export filter is never consulted. Every
  `INFO: filter not called` fall-through became `runtime_fail`.

### Bugs Found/Fixed

- `parseOptionConfig` (`expect.go`) failed soft on a non-numeric `asn` /
  `tcp_connections` and on an `open=` / `update=` value with no branch. Each was
  the same silent drop one level down. All four now error.
- `TestParseAndAdd_OptionTimeoutInsidePeerBlockPasses`
  (`record_parse_test.go`) pinned `option=update:value=inspect-update-message`,
  a value no branch of ze-peer has ever had, and asserted a peer block accepts
  it. The fixture now uses a real value and the comment's two false claims about
  who reads `option=timeout` are corrected.
- A peer-block `option=timeout` would have overridden the file-level one once the
  guard started parsing it (455 blocks carry a decorative one; two disagree with
  their file-level value). It is parsed into a throwaway record instead, so the
  guard proves it is not a typo and the block gains no authority over the test.

### Documentation Updates

- `docs/architecture/testing/ci-format.md` - "What a ze-peer block may carry"
  (the contract table) and "`reject=bgp` -- bytes a peer must never receive".
  Anchors: `peer_contract.go -- validatePeerBlockDirectives`,
  `expect.go -- ClaimLine`, `reject.go -- reject=bgp`.
- `ai/patterns/functional-test.md` - the directive row and the peer-block rule.
  Round 6 added "An observer plugin answers callbacks before it polls": the
  callback pump the seven repaired fixtures need, and the two barrier rules the
  round-6 finding produced (a per-peer counter is a lifetime total that already
  counts the End-of-RIB; `quiesce` is not a barrier for establishment).
  Anchors: `ze_api.py -- read_line, wait_peer_counter, wait_peer_eor_sent`,
  `reactor_notify.go -- IncrUpdatesSent on the sent branch`.
- `docs/architecture/bgp/fanout-dedup.md` - AC-5.
- `make ze-doc-test`: PASS at round 5. Re-run at round 6 is scoped to the two
  checks that read the edited files, because the full target is red at HEAD on
  `ai/DOCS-TO-CODE.md is stale`, which another session owns:
  `python3 scripts/dev/code_to_docs.py --check` exits 0 (2091 code paths, all
  references valid), and `python3 scripts/dev/validate.py --root .` reports the
  same 9 pre-existing issues as before the edit and none about an anchor.

### Deviations from Plan

- The `.ci` half of this spec is NOT in its own commit. Commit `06c95f65d`
  (another session, `attach process` grammar) absorbed
  `test/plugin/filter-family-export-flowspec.ci`, `logging-level-filter.ci` and
  `rest-peer-set-delete-lifecycle.ci` while this closure was running. HEAD
  therefore carries the corpus and not the engine that gives it meaning: every
  `reject=bgp` at HEAD is dropped exactly as before until this commit lands.
- AC-3's block-level residue was CLOSED on 2026-08-16, so no Partial AC remains
  and no owner approval is owed. The seven `test/plugin/redistribution-*.ci` each
  start the ze-peer their block was written for, and every directive in those
  blocks now decides the verdict. The journal row in
  `plan/journal/silent-fall-through.md` carries the repair. What is still open is
  the RUNNER-side refusal of a peer block no `cmd=` names: `blockPeerMode`
  (`internal/test/runner/peer_contract.go`) refuses only a `reject=` there, and
  the corpus reason for stopping short of refusing the whole block is gone now
  that the seven are repaired.
- The spec's Task names a third defect with no AC: a parse-suite helper script
  cannot invoke `ze`, because `runOneCommand` (`internal/test/runner/parsing.go`)
  builds the child environment with `childEnv(test.EnvVars...)` and never adds a
  PATH entry for `filepath.Dir(r.zePath)`. It is NOT implemented. It is recorded
  in `plan/deferrals/finish-ci-coverage.md` as live and unhomed, and it is the
  one open question this closure hands back.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: the three known sites were the whole population | 15 dropped directives in 12 files | tree-wide re-derivation on 2026-08-15 | the deliverable became the derived guard rather than a patch to a list of sites |
| approach | The guard read the `conn=` out of a `reject=bgp` line with its own `SplitSeq(tail, ":")` | `ci.ParseKVPairs` treats `pattern=` as a complex key that swallows the remainder, so `reject=bgp:pattern=AA:conn=1` satisfied the guard and then failed inside ze-peer, surfacing as a bind timeout | review round 1, lens C | `peer.ParseRejectRule` exported as the one parser; `connOf` deleted |
| approach | `Checker.rejection` keys on `currentConnection`, which is the expectation sequence's connection | only check mode reads connections in turn; sink and echo run them concurrently against one checker, so `conn=` selects nothing there | review round 1, lens C | refused at parse time (`validatePeerBlockRejects`) and at run time (`peer.New`) |
| approach | The AC-3 repair replaced a blind sleep with `updates-sent >= 1` on peer2 and called it a barrier proving the forward was written | `updates-sent` counts the initial-sync End-of-RIB as well, so 1 is reached by establishment alone. The repair swapped a sleep for a barrier that waits for nothing, in the two fixtures whose whole subject is what peer2 does or does not receive | review round 6 | threshold raised to 2, the count peer2 actually reaches, with the mechanism stated in the comment. The class is journalled in `plan/journal/false-synchronization-claim.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A `reject=` in a peer block must stop being a silent no-op | Done | `peer_contract.go` `validateOnePeerBlock` | derived from both parsers |
| `reject=bgp:` must exist or be documented unavailable | Done | `internal/test/peer/reject.go` | implemented, not documented away |
| Audit the sites once the guard fires | Done | 15 directives, 12 files | see AC-3 |
| `tmpfs=...:mode=` is dropped | Changed | `parsing.go` `setupWorkDir` | parse suite fixed in `dc591ec72`; the orchestrated half is journalled in `plan/journal/helper-bypassed-by-an-open-coded-copy.md` |
| A parse-suite helper cannot invoke `ze` | **Skipped** | `parsing.go` `runOneCommand` | no AC covered it; live row in `plan/deferrals/finish-ci-coverage.md`; NEEDS the owner's answer |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPeerBlockRefusesUnclaimedDirective`, `TestPeerBlockRefusesMalformedReject` | error names block, line and directive |
| AC-2 | Done | `filter-family-export-flowspec.ci` rewritten, `logging-level-filter.ci` reject now live, `rest-peer-set-delete-lifecycle.ci` `option=mode` removed with its reason | all three landed in `06c95f65d` |
| AC-3 | Done | `TestPeerBlockGuardCoversEveryPeerBlock`, `TestPeerBlockRefusesRejectNoPeerReads`, `TestCIPeerBlockCorpusParses`, and the seven repaired `test/plugin/redistribution-*.ci` | 15 LINE-level drops repaired across 12 files, and the guard refuses a new one. The BLOCK-level twin is now closed too: each of the seven starts `ze-peer --port $PORT` on its own block, and each assertion in that block was made real rather than kept. `make ze-plugin-test`: the seven pass 4 of 4 runs, 3.7s to 6.1s each. Each was shown red by breaking the behaviour it names, never by editing its own expectation |
| AC-4 | Done | `reject.go` + `reject_test.go` + 9 corpus fixtures | implemented, not deferred |
| AC-5 | Done | `docs/architecture/bgp/fanout-dedup.md` | recorded as expressible now, and NOT yet expressed |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the 9 planned unit tests | Done | `peer_block_directive_test.go`, `reject_test.go` | all pass |
| 6 added in review round 1 | Done | same two files | timeout scope, malformed reject, non-check peer, sorted names, `New` refusal, pass-through |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `peer_contract.go`, `record_parse.go`, `expect.go`, `reject.go` | Done | |
| `ci-format.md`, `functional-test.md`, `fanout-dedup.md` | Done | |
| the three `.ci` files | Done | committed by `06c95f65d`, not by this closure |
| the seven `test/plugin/redistribution-*.ci` | Done | AC-3's block-level residue, repaired 2026-08-16 |

### Audit Summary
- **Total items:** 5 requirements, 5 ACs
- **Done:** 4 requirements, 5 ACs
- **Partial:** none
- **Skipped:** 1 (the parse-suite PATH limit; needs the owner's answer)
- **Changed:** 1 (tmpfs mode, fixed elsewhere and journalled)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A directive in a peer block that asserts nothing must fail the file | functional + unit | `TestPeerBlockGuardCoversEveryPeerBlock` goes red when `peerBlockNames` narrows back to `"peer"`; `make ze-plugin-test ZE_SUITE_TIMEOUT=1500s` 602 PASS / 44 SKIP / 2 FAIL over 648, both failures pre-recorded as non-deterministic (`plan/journal/false-synchronization-claim.md` for `cli-commit-reject`, `plan/journal/gate-verdict-depends-on-the-machine.md` for `concurrent-config-commit`) and both taking their budget from a file-level `option=timeout` at line 5, which this change does not touch |
| A peer must be able to assert bytes it never received | functional | the 9 corpus fixtures carrying `reject=bgp` all pass, `wellknown-no-advertise-egress`, `wellknown-no-export-egress`, `wellknown-no-export-withdraw-egress`, `control-community-withdraw-egress`, `rs-control-community-withdraw-egress`, `originated-nexthop-peer-own`, `rfc7606-54-discard-unrecognized-nlri`, `-mup-nlri` and `filter-family-export-flowspec` among them |
| The rejection must discriminate, not decorate | review + unit | round 1 lens B traced the delivery mechanism for each of the 9: fence-route-sent-last on one TCP stream, or same-frame (the needle is the NLRI excised from the very MP_REACH the expectation matches, and `p.rejected` runs before `ExpectedOrKeepalive`). `TestCheckerRejectedIsByteAligned` goes red if `indexByteAligned` becomes `strings.Index` |
| The accepted set must not be a third hand-written list | review | round 1 lens C found the one place it still was (`connOf`) and it is deleted; `peer.ParseRejectRule` is now the single parser both sides call |
| A peer block nobody starts must stop asserting nothing (AC-3, block level) | functional | the seven `test/plugin/redistribution-*.ci` each start `ze-peer --port $PORT` on their block and pass 4 of 4 runs. Each was shown red by breaking the PRODUCT behaviour it names, never its own expectation: `declare` by deleting the peer's `family` block, so ze negotiates no family and emits no End-of-RIB; `import-accept`, `import-reject` and `import-modify` by deleting the `filter { import [...] }` binding; `chain-order` by deleting the second filter from that list, giving `1 filter call(s), expected 2`; `export-reject` and `export-modify` by flipping the filter's own decision, which fires `reject=bgp:conn=1:pattern=180A0000` on the fence peer and drops `400504000000C8` from peer2's wire. Suite: `make ze-plugin-test ZE_SUITE_TIMEOUT=1500s` 603 of 604 and 602 of 604 over two runs, every red a load-dependent flake in a file this change does not touch, attributed in `plan/journal/gate-verdict-depends-on-the-machine.md`. Round 6 then corrected the two export fixtures' shutdown barrier, which waited for nothing, and re-measured the seven with `ze-test bgp plugin --pattern redistribution- -c 2`: 14 of 14, 5.4s to 9.3s each |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 3: `reject=bgp:` is not implemented ANYWHERE | done | this spec. Row updated in place with the producing functions |
| same shard, rows 2, 4, 5, 6, 7 | still live | homed at `spec-wire-edit-2-deferred-ci-substitution`, `spec-fixit-vacuous-functional-tests` (x2), `spec-wire-edit-3-deferred-ac9-dead-code`, `spec-fixit-fwdpool-backpressure-timing`. The shard is NOT removed |
| `plan/deferrals/finish-ci-coverage.md`, row of 2026-08-03, destined here | split | tmpfs `mode=`: parse suite fixed in `dc591ec72`, orchestrated half journalled. Parse-suite PATH: LIVE and UNHOMED, raised to the owner at this closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ci-peer-block-silent-directives-55b89662-ed70-484b-8728-361629e96dbc.md` |
| `review_gate.py check` | clean (exit 0, hashes match) |
| Rounds | 6 (rounds 4 and 5 earned by a PRODUCT defect round 3 found: a `reject=bgp` in a `.ci` with no `cmd=` lines reached no parser at all, because `writeExpectFile` builds ze-peer's input from `expect=`/`action=` only. Round 6 earned by a second one: the two export fixtures gated their shutdown on `updates-sent >= 1` for peer2, a threshold the initial-sync End-of-RIB satisfies by itself) |
| Reviewer lenses used | round 1: three parallel lenses (logic+wiring+removed-behaviour, test-discrimination+non-vacuity+coverage-regression, security+edge-cases+allocation+documentation). round 2: re-review of every round-1 fix. round 3: re-review of every round-2 fix. round 4: re-review of every round-3 fix, plus a final pass over the change as one unit. round 5: a narrow pass proving the ASD-STE100 prose edits to two files are comment-only and semantics-preserving, which the hash-pinned gate required. round 6: the AC-3 repair (the seven `test/plugin/redistribution-*.ci`), read hunk by hunk under four lenses -- does each fixture still assert its header's claim, is the `family` block a fixture fix or a mask over a product defect, can the stated mutation pass with the mechanism broken, and did the second peer change what the fixture proves |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The guard parsed a `reject=bgp` line with its own splitter while ze-peer used `ci.ParseKVPairs`, where `pattern=` swallows the remainder. `reject=bgp:pattern=AA:conn=1` passed the guard and failed inside ze-peer, reported as a bind timeout | `peer_contract.go` `connOf` | `peer.ParseRejectRule` exported as the one parser; `connOf` deleted; `connOfExpect` reads only `expect=bgp:` tails |
| 2 | ISSUE | A rejection could reach a sink/echo peer, which reads every connection concurrently against one checker, so `conn=` selected nothing | `reject.go` `Checker.rejection` | refused at parse time (`validatePeerBlockRejects` via `blockPeerMode`) and at run time (`peer.New`) |
| 3 | ISSUE | A malformed `reject=bgp` escaped parse time and killed the peer before it bound | `expect.go` `ClaimLine` | `ClaimLine` validates the payload with `ParseRejectRule` |
| 4 | ISSUE | A peer-block `option=timeout` silently overrode the file-level one (455 blocks carry one; 2 disagree) | `peer_contract.go` `validateOnePeerBlock` | parsed into a throwaway record; `TestPeerBlockTimeoutDoesNotOverrideTheFileLevelOne` |
| 5 | ISSUE | `parseOptionConfig` failed soft for a non-numeric `asn`/`tcp_connections` and for an `open=`/`update=` value with no branch | `expect.go` `parseOptionConfig` | each errors; the fixture pinning a non-existent `update` value corrected |
| 6 | ISSUE | `validatePeerBlockDirectives` ranged a map, so a file with two bad blocks reported a random one | `peer_contract.go` `peerBlockNames` | returns a sorted slice; `TestPeerBlockNamesAreSorted` |
| 7 | ISSUE | The guard's own comment and error claimed the delivery rule was sufficient for non-vacuity; three doc claims were false (byte alignment vs `contains=`, "anything else refused", "check, sink and echo mode") | `peer_contract.go`, `ci-format.md` | comment and error state necessary-not-sufficient and name the author's half; the three doc claims corrected |
| 8 | ISSUE | `fanout-dedup.md` said the draft fixture proves the AC-1/AC-2 negatives; it is a draft no gate runs and it carries no rejection | `fanout-dedup.md` | states expressible-but-not-yet-expressed |
| 9 | NOTE | `TestCIPeerBlockCorpusParses` claimed to validate AC-3 and would stay green with the guard deleted | `peer_block_directive_test.go` | comment says it is a corpus net and names the real discriminator |
| 10 | BLOCKER (round 2) | A `stdin=peer` block that no `cmd=...:exec=ze-peer` names is read by nothing, yet the guard validated it as if ze-peer would. `peerBlockNames`'s doc comment asserted the opposite and was false | `peer_contract.go` `peerBlockNames` | comment corrected against `writeExpectFile` (`runner_output.go`) and the `cmd=` loop; `blockPeerMode` now reports whether any peer READS the block and a `reject=bgp` there is refused (`TestPeerBlockRefusesRejectNoPeerReads`). The seven committed files carrying other peer directives in a dead block are journalled, and AC-3 is recorded Partial rather than claimed |
| 11 | ISSUE (round 2) | A bare `reject=bgp` with no `key=value` tail passed the file: `ParseRejectRule` answered "not a reject" while `consumes` answered true, so ze-peer took it, `parseExpectRule` failed inside `peer.New`, and the runner reported a bind timeout | `expect.go` `ClaimLine` | the `actionReject` arm errors on `isReject == false`; `bare` row in `TestPeerBlockRefusesMalformedReject` |
| 12 | ISSUE (round 2) | The four new `parseOptionConfig` error paths had no test, and the edited fixture removed the only input that reached one | `expect.go` `parseOptionConfig` | four rows added to `TestPeerBlockRefusesUnclaimedDirective` |
| 13 | ISSUE (round 2, found by the suite) | Discarding a peer-block `option=timeout` outright took the budget away from files whose ONLY timeout is in the block, and two llgr fixtures went red | `peer_contract.go` `validateOnePeerBlock` | the block value is adopted only when the file declares none, so a file-level value still wins; both halves tested in `TestPeerBlockTimeoutDoesNotOverrideTheFileLevelOne` |
| 14 | NOTE (round 2) | The scratch record burned a nick from the global counter, making operator-visible `ze-test bgp <nick>` ids non-contiguous | `peer_contract.go` | a bare `&Record{Extra: ...}` replaces `newRecord` |
| 16 | ISSUE (round 3) | `blockPeerMode` answered `read = true` for a `.ci` with no `cmd=` lines, but that path never hands the block to ze-peer: `writeExpectFile` (`runner_output.go`) builds the peer's input from `Record.Options` and `Record.Expects`, which the guard fills from `expect=`/`action=` only. A `reject=bgp` there is ClaimPeer, matches neither prefix, and reaches nothing | `peer_contract.go` `blockPeerMode` | `read` is now true only when a `cmd=...:exec=ze-peer ...:stdin=<name>` names the block. No committed file is affected: the files with no `cmd=` line carry no `stdin=peer` block |
| 17 | ISSUE (round 3) | `ci-format.md` said `option=timeout` is "parsed and discarded", contradicting the round-2 fix it documented | `ci-format.md` | says adopted only when the file declares none |
| 18 | NOTE (rounds 3-4) | Three stated counts were wrong. Two re-measurements were needed, because the first read `option=timeout` inside a `tmpfs=` block as file-level. Final, over `git ls-files test`: 450 lines inside a ze-peer stdin block across 418 files, and 43 tracked `.ci` with no `cmd=` line, 42 of them under `test/exabgp-compat/encoding` and none carrying a `stdin=peer` block | `peer_contract.go`, `peer_block_directive_test.go`, `ci-format.md` | corrected in all four places, and round 5 caught the second error |
| 15 | NOTE (round 2) | Two doc counts were wrong: the draft fixture does carry two `reject=stderr` crash guards, and the twelve dropped `reject=` are eleven `reject=bgp` in nine files plus a `reject=stderr` in a tenth | `fanout-dedup.md`, `ci-format.md` | both corrected |
| 19 | ISSUE (round 6) | The AC-3 repair gated both export fixtures on `api.wait_peer_counter('updates-sent', 1, peer='127.0.0.2')` and its comment claimed that threshold proves the forwarded UPDATE is on peer2's socket. It proves nothing: `writeUpdateGated` (`reactor/session_write.go`) notifies `onMessageReceived` for the initial-sync End-of-RIB too, and the sent branch of `reactor_notify.go` counts every `msgtype.TypeUPDATE` because `wireUpdate` is nil for a sent message. Establishment alone satisfies 1, so the shutdown could still beat the frame `expect=bgp:conn=1:seq=2` asserts. `wait_peer_counter`'s own docstring warns against an absolute threshold for this reason, and the one other caller (`bgp-redistribute-announce.ci`) uses `base + 1` | `redistribution-export-reject.ci`, `redistribution-export-modify.ci` | threshold raised to 2, which is what peer2 receives on this session (its EOR, plus the one forward the filter passes), with the reason written into the comment. `attempts=40` keeps the wait inside the fixture's own 15s budget so the barrier prints its message rather than the runner reporting a timeout. 14/14 green over two runs of all seven |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/peer/reject.go` | yes | `git status` reports it untracked, 178 lines |
| `internal/test/peer/reject_test.go` | yes | `go test -run TestSplitRejectRules` runs it |
| `internal/test/runner/peer_block_directive_test.go` | yes | `go test -run TestPeerBlock` runs 11 tests from it |
| `test/plugin/filter-family-export-flowspec.ci` | yes | at HEAD in `06c95f65d`; ran in `make ze-plugin-test` as test 221 |
| the seven `test/plugin/redistribution-*.ci` | yes | tracked and modified; ran as plugin ids 448-454 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the runner refuses the file, naming directive and line | `TestPeerBlockRefusesUnclaimedDirective` PASS over 5 shapes; `TestPeerBlockRefusesMalformedReject` PASS over 6 |
| AC-2 | each known site carries a working assertion or a stated reason | `logging-level-filter` PASS (295), `rest-peer-set-delete-lifecycle` PASS (472), `filter-family-export-flowspec` PASS (221) |
| AC-3 | no other site carries a dropped directive | `TestCIPeerBlockCorpusParses` PASS over the whole corpus; `TestPeerBlockGuardCoversEveryPeerBlock` PASS; the seven repaired `test/plugin/redistribution-*.ci` PASS 14 of 14 over two runs of `ze-test bgp plugin --pattern redistribution- -c 2`, taken after the round-6 barrier fix |
| AC-4 | `reject=bgp` fails when the rejected bytes ARE sent | `TestCheckerRejectedIsByteAligned` and `TestCheckerRejectedIsPerConnection` PASS; `TestNewRefusesRejectOutsideCheckMode` PASS |
| AC-5 | recorded in `fanout-dedup.md` | the section states expressible-now and not-yet-expressed, with the reason |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a `.ci` with `reject=` inside a peer block | the guard's fixtures are written and parsed in-process by `peerBlockCI` | yes: `parseAndAdd` is the runner's real entry point, not a helper |
| a live wire rejection through the daemon | `test/plugin/wellknown-no-advertise-egress.ci` | yes: read, 3 rejections across 2 peer blocks, each with a fence route delivered last on the same connection |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | 15 dropped directives in 12 files, not 3. Mistake Log row 1 |
| A-2 | confirmed | `make ze-plugin-test` 602 PASS / 44 SKIP with the guard live; the 2 reds are attributed above and neither touches a peer block's directives |
| A-3 | confirmed | `internal/test/peer/reject.go` implements it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| the ze-peer-owned `option=` list in `ci-format.md` | checked against `parseOptionConfig` case by case | yes, round 1 lens C |
| "byte-aligned, unlike `expect=bgp:contains=`" | `matchRule` uses `strings.Contains`; only `indexByteAligned` aligns | yes, corrected in round 1 |
| "check mode only" | `peer.New` refuses any other mode | yes |
| a peer-block `option=timeout` never overrules a file-level one | `validateOnePeerBlock` parses it into a bare `&Record{Extra: ...}` and copies it to the record only when `r.Extra["timeout"]` is empty | yes: both halves driven by `TestPeerBlockTimeoutDoesNotOverrideTheFileLevelOne` |
| feature list, user guide, config syntax, CLI reference, API/RPC, plugin SDK, wire format, RFC compliance, comparison table, architecture design | No for each: this change adds no config leaf, no CLI verb, no RPC, no wire behaviour and no RFC claim. `grep -rn "source: internal/test/peer\|source: internal/test/runner" docs/` names only `ci-format.md`, which is updated | yes |
| test infrastructure | Yes: `ci-format.md` and `ai/patterns/functional-test.md`, both updated | yes |

## Core Insight

A directive is only as real as the parser that answers for it, and "the parser"
must be ONE. This spec closed a drop caused by two parsers not covering a line
between them, and its own first implementation opened a smaller copy of the same
hole: the guard re-read `conn=` with a splitter that disagreed with
`ci.ParseKVPairs` about where a `pattern=` value ends. The rule that survives is
not "add a guard" but "make the guard CALL the parser it is guarding".
