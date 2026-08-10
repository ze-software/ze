# Spec: fixit-test-harness-fail-open-guards

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | guards 1, 2, 3 and 4 implemented; closure pending |
| Deferral shard | `plan/deferrals/fixit-test-harness-fail-open-guards.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Four guards in the test harness fail OPEN. One reports success for a run that
was cancelled. One can never fire, because a shorter timeout always wins first.
The third answers 0 for a query that failed, and 0 is a legitimate RIB size. The
fourth is missing outright: nothing stops a new scenario writing the same
swallowed call the third one hid behind.**

Guard 3 is DONE (2026-08-07). Guards 1 and 2 are DONE (2026-08-09). Guard 4 is
untouched and needs the ruling in the table above. This spec stays open for it.

Found on 2026-08-02 by the independent review of
`spec-rfcgate-2-deferred-rs-replay-evidence` (closed 2026-08-03 in `15dac5bc4`; written without its `plan/` path because `spec-citation-check.py` reads any such path as a LIVE citation and the file is gone. Its record was retired with the learned corpus), while closing
the rfcgate-1b RFC 7296 pilot spec.

A guard that fails open in the test harness is worse than no guard. It converts a
run that proved nothing into a green bar, and a green bar is what everyone reads.
`ai/rules/evidence.md` states the rule these two break: fail closed or say
something, and a zero value must never be a valid looking answer.

### Guard 1: a cancelled accept reports success

`acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`) accepts a batch of
connections. When `ln.Accept()` returns an error it checks whether the context is
done, and if it is, returns `Result{Success: true}`.

Nothing consults `checker.Completed()`. So a run cancelled before the checker
finished its assertions is indistinguishable from a run that passed them. The
cancellation path is the one taken on timeout and on operator interrupt, which are
exactly the cases where a false green is most costly.

### Guard 2: a diagnostic that can never fire

**Surveyed 2026-08-09, and it is worse than the one instance found. EVERY call
site is affected: all 17 `.ci` files invoking `run_rs_observer` set a foreground
timeout of 10s (3 files), 15s (9) or 20s (5). None exceeds the 30.0s
`eor_timeout` default, so `ZE-OBSERVER-FAIL` has never been reachable anywhere
and the harness has never once emitted the diagnosis it advertises.**

**The runner exports no timeout budget today.** `runner_exec.go` appends
per-process env (`ze_test_bgp_port`, `ze.log.backend`, `ZE_TEST_NETNS_HOST`,
and the `.ci`'s own `option=env:` knobs) and nothing carries the foreground
deadline, so AC-3's "derive from the `.ci` budget" branch needs a new variable
before the derivation can be written. That is guard 2's implementation.

`run_rs_observer` (`test/scripts/ze_api.py`) documents itself as failing closed:
when the replay does not complete before `eor_timeout` it reports the named
diagnostic `ZE-OBSERVER-FAIL`. Its default `eor_timeout` is 30.0 seconds.

`test/plugin/bgp-rs-relay-aspath-transparency.ci` runs with a foreground timeout of
20 seconds. The foreground timeout therefore always expires first, the process is
torn down, and the diagnostic the harness advertises never reaches anyone. The test
still denies a broken run, so the coverage is real, but the failure it reports is
"timed out" rather than the specific diagnosis the harness was written to give.

The same arithmetic applies to any `.ci` whose timeout is under 30 seconds. This
spec must survey them rather than fix the one instance found.

### Guard 3: a RIB query that failed answers 0 (DONE 2026-08-07)

Homed here by `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md`.

`Ze.rib_count` (`test/interop/interop.py`) ran its query through
`docker_exec_quiet`, which returns "" on any non-zero exit, then returned 0 when
the regex found nothing. A failed query and an empty RIB were the same number.

**This guard cost three days and hid three separate faults**, which is the
clearest evidence in the repo for why a fail-open reader is worse than none:

| Fault | Producer | Fixed by |
|-------|----------|----------|
| `ze <verb> ...` resolved almost nothing. 56 of the 63 `ze show` subcommands answered `unknown command` on the host, and every other YANG verb with them | `RunCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) walked the verb-RELATIVE tree from `cli.BuildVerbCommandTree` (`bgp rib status`) with the verb-INCLUSIVE argv (`show bgp rib status`) | `ResolveCommand` aligns the two and `cli.AbsoluteVerbPath` (`internal/component/cli/client/verb_tree.go`) rebuilds the absolute path the registries and the daemon are keyed on |
| The interop daemon started no SSH listener, so no CLI client could reach it whatever the verb | `infraSetup` (`cmd/ze/hub/infra_setup.go`) starts one only when the config asks, and no scenario `ze.conf` asked | `ZE_CLI_CONFIG` appended to every RENDERED `ze.conf`, queried through `ze cli -c` |
| The helper itself | `Ze.rib_count` | raises, naming the command and the container |

The verb fault was invisible to every other gate: `make ze-cli-grammar-check`
checks how commands are DECLARED, and the `.ci` suite and the interactive CLI
both resolve through the daemon dispatcher, which was never broken.
`TestDeclaredCommandsResolveFromArgv`
(`cmd/ze/internal/cmdutil/cmdutil_test.go`) is the gate that was missing: it
drives `ResolveCommand` with a real argv for every declared `ze:command`. Mutating
the alignment back fails it on 305 paths.

### Guard 4: nothing stops a new scenario writing the swallowed call again

Homed here on 2026-08-08 from `plan/deferrals/ad-hoc-2026-08-08-031d68b3.md`,
while `ze show host` was repaired (`internal/plugins/host-cmd/yang/ze-host-cmd.yang`).

Guard 3 raised `Ze.rib_count`. It did not close the CLASS. `docker_exec_quiet`
(`test/interop/interop.py`) still returns `""` on any non-zero exit, and 80 call
sites read that return value. A scenario that runs the bare `ze <verb> ...` form
inside a container gets `no credentials` from `readCredentials`
(`internal/core/ssh/client/client.go`), which is turned into `""` and then into a
passing assertion over nothing. Nineteen instances were fixed BY HAND on
2026-08-07. Nothing refuses the twentieth.

**A mechanical guard is what closes it.** The shape is a `scripts/dev/` lint with
a sibling `_test.py`, wired into a make target and routed onto the verify path by
`scripts/dev/verify_wiring_docs.py` when a scenario file changes. It reads the
same population `ci_dispatch_commands` reads for `.ci` emitters
(`scripts/checks/ci_dispatch_commands.go`), which is the precedent to follow
rather than reinvent: fail closed on a string it cannot evaluate statically.

Two things it must decide, and neither is obvious enough to leave to the
implementer without a ruling:

| Question | Why it needs an answer before code |
|----------|------------------------------------|
| Does the lint ban the bare `ze <verb>` form inside a container, or does it require every `docker_exec_quiet` return value to be checked? | The first is narrow and catches today's nineteen. The second catches the class and touches all 80 sites |
| Does `docker_exec_quiet` keep its fail-open contract at all? | Its docstring promises "empty string on failure", so callers are written against it. Making it raise is a one-line change with an 80-site blast radius, and it may be the correct fix rather than the lint |

This is new tool + make target + docs + rules wiring, which is why it was homed
here rather than folded into the `ze show host` repair.

### Guard 4: the ruling, and what the tree actually holds (2026-08-09)

**Thomas ruled: the lint requires every return value to be checked.** Not the
narrow ban on the bare `ze <verb>` form. `docker_exec_quiet` keeps its fail-open
contract; the lint is what closes the class.

**The population is bigger than this spec assumed, measured with an AST walk over
every `*.py` outside `tmp/` and `.claude/worktrees/`:**

| Measure | Count |
|---------|-------|
| Fail-open FUNCTIONS, not one | 20 |
| Call sites across that set | 255 |
| Unchecked call sites | 114, across 46 files |
| Unchecked sites inside `test/interop/interop.py` alone | 36 |

The set is transitive and that is the load-bearing part: a function whose body
`return`s a fail-open call is itself fail-open, so `docker_exec_quiet` drags in
`_vtysh_quiet`, `_birdc_quiet`, `_gobgp_quiet`, `_swanctl`, `frr_log`, `vtysh`,
`xfrm_state`, `xfrm_policy`, `ze_xfrm_state`, `ze_xfrm_policy`, `ze_ppp_addr`,
`accel_show_sessions`, `list_sas`, `ospf_summary`, `ospf6_summary`,
`isis_summary`, `link_lsa_dump`, `inter_area_prefix_dump` and `_wait_bird_route`.
Scenarios call the WRAPPERS, not the seed. A lint that knew only the seed name
would report a handful of sites and miss the class it exists to close.

**A worked true positive, so the lint is designed against a real defect rather
than a category.** `FRR.is_dis` (`test/interop/interop.py`) binds
`out = self._vtysh_quiet("show isis interface detail")` and returns
`"DIS" in out or "Designated" in out`. When the command fails, `out` is `""`,
both membership tests are False, and the method answers "no Designated IS
elected" -- indistinguishable from a real negative answer, which is exactly the
zero-is-a-valid-looking-answer shape `ai/rules/evidence.md` names. Its neighbours
`has_isis_route` and `has_isis_route_v6` have the identical shape.

**A worked FALSE positive, which is why the lint needs an opt-out.** The same
class prints `self._vtysh_quiet("show isis neighbor")[:500]` in the failure path
of `wait_adjacency`, immediately after `log_fail` and immediately before it
raises. It is diagnostic output on a run that has already failed, and requiring a
check there buys nothing.

### Guard 4: the design

Follows `scripts/checks/ci_dispatch_commands.go`, which the Required Reading
already names as the precedent: a checker over test call sites that resolves
against the real thing, fails closed on what it cannot read statically, offers an
explicit marker for the genuinely dynamic case, and ships a `--selftest` and a
sibling test.

| Piece | Decision |
|-------|----------|
| Detector | `scripts/dev/docker_exec_checked.py`. Computes the transitive fail-open set to a fixpoint, then classifies every call site of any member |
| Checked | The value is bound and the bound name is tested for emptiness in the same function, or the call is the function's own `return` (the obligation moves to its callers, who are themselves call sites and get classified) |
| Unchecked | Bound and never tested, or used inline with no test: `f(x)[:5]`, `"s" in f(x)`, `json.loads(f(x))` |
| Discarded | A bare-statement call is fire-and-forget and not flagged. It asserts nothing, so it cannot pass an assertion over nothing |
| Opt-out | `# fail-open-ok: <reason>` on or above the line, auditable with one grep, same shape as `// test-relax:` and `rfc-test-change-approved:` |
| Turning it on | A committed count in `test/health/docker-exec-baseline.json` that may only go DOWN, following `test/health/sensitivity-baseline.json`. This refuses the twentieth site on the day it lands, which is what guard 4 is FOR, without a 114-site edit in one commit |

### Guard 4: what landed, and why the floor is 168 rather than 114

`scripts/dev/docker_exec_checked.py` reproduces the survey exactly on the two
DERIVED numbers: 20 fail-open functions and 255 call sites. It differs on the
verdict split, and the difference is one rule.

**A membership test is not an emptiness test.** `if prefix in out` is False on
`""`, so it is the fail-open shape rather than a guard against it. That is not a
refinement of the survey, it is what the survey's own worked true positive
demands: `FRR.is_dis` reads `"DIS" in out or "Designated" in out` and must be
flagged. Applying the rule consistently reclassifies 54 sites the survey counted
as checked, so the measured split is 55 checked, 32 discarded, 168 unchecked.

The floor committed in `test/health/docker-exec-baseline.json` is therefore 168.
It is a measurement, not a target, and it may only fall.

**Landed at 171, not 168 or 114, and the number is a measurement of the
CHECKER.** Review round 1 found `_emptiness_tested` walked the whole function
with no position comparison, so any emptiness test on a same-named variable
marked EVERY assignment of that name checked. Three live sites rode out on a
guard belonging to an earlier call, and the worst was `FRR.route_count`
(`test/interop/interop.py`): its JSON call is guarded, its text fallback is not,
`splitlines()` on `""` is empty, and it answers 0 prefixes for a vtysh that
FAILED. That is guard 3's `Ze.rib_count` shape surviving inside guard 4, which is
the one outcome this checker may not have. The rule is now positional, two tests
pin it, and the floor rose 168 to 171 because the detector got more correct.

**Two files are written, tested, and HELD OUT of the landing commit.**
`scripts/dev/verify_wiring_docs.py` and its test carry the changed-file routing.
Another session's `plan/learned` to `plan/journal` refactor interleaved 12 lines
into that file between the implementation and the commit, so committing it would
carry their half-finished work. The floor is enforced without it: `TestRepoRatchet`
runs the real scan and `scripts/dev/python_tests_test.go` globs every `*_test.py`,
so `make ze-unit-test` already refuses a rise. Routing is a changed-file
optimisation, not the guard. Its row is in this spec's deferral shard.

**Why the ratchet rather than fixing all 114 now.** Guard 4's stated defect is
"nothing refuses the twentieth". A ratchet refuses it immediately. Landing 114
mechanical edits across 46 interop files in the same change would put a large
diff, most of it unrunnable without Docker, between the defect and its guard, and
the guard is the deliverable. The count is the backlog, in the open, and it can
only fall.

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/evidence.md` | The rule all four guards break, and the shape of the repair |
| `scripts/checks/ci_dispatch_commands.go` | Guard 4's precedent: a checker over test call sites that fails closed on what it cannot read |
| `test/interop/interop.py` | Guard 4's producing function, `docker_exec_quiet` |
| `ai/rules/testing.md` | Observer-exit antipattern, and how a harness hides a broken production path |
| `internal/test/peer/peer_connmap.go` | Guard 1's producing function |
| `test/scripts/ze_api.py` | Guard 2's producing function and its timeout contract |

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/peer/peer_connmap.go` - `acceptConnMapBatch` returns `Result{Success: true}` on a context-cancelled accept
- [ ] `test/scripts/ze_api.py` - `eor_timeout: float = 30.0`, with the fail-closed contract documented at `:1786`
- [ ] `test/plugin/bgp-rs-relay-aspath-transparency.ci` - foreground timeout of 20 seconds

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every currently passing functional test must still pass. This spec tightens a
  guard; it must not change what a correct run does.
- The `.ci` file format and the observer's public arguments.

**Behavior to change:** (only what the user asked for)
- A cancelled accept must not report success unless the checker completed.
- A harness timeout must be reachable, or the harness must not advertise it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` file's `timeout=` directive, and the test runner's context deadline.
- A TCP listener accepting peer connections inside `internal/test/peer`.

### Transformation Path
1. The runner starts a `.ci` with a foreground timeout and a context.
2. `acceptConnMapBatch` accepts connections until the batch fills or the context ends.
3. On context end it returns a `Result`, which the runner renders as pass or fail.
4. In parallel, `run_rs_observer` waits up to `eor_timeout` for the replay and, on
   expiry, emits `ZE-OBSERVER-FAIL`.

The defect in step 3 is that the `Result` does not consult the checker. The defect in
step 4 is that the step 1 deadline is shorter than the step 4 deadline, so step 4's
branch is unreachable.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test runner ↔ peer harness | `Result` struct with a `Success` bool | No |
| `.ci` timeout ↔ observer timeout | Two independent numbers, never compared | No |

### Integration Points
- `checker.Completed()` - exists and reports whether assertions finished. Guard 1 must
  consult it.
- The `.ci` parser's `timeout=` handling - guard 2's repair needs to know the value.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|-----------|------------|--------|
| A-1 | No currently green test depends on the cancelled-accept success path | Tighten the guard, run the full functional suite | unvalidated |
| A-2 | `checker.Completed()` is reachable from `acceptConnMapBatch` | Read the call site and the type | unvalidated |
| A-3 | The 20s versus 30s mismatch is not unique to one `.ci` | Survey every `.ci` timeout against the observer default | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Tightening guard 1 turns load-sensitive tests red | Run the suite under parallelism before and after, and read `ai/rules/testing.md` |
| R-2 | Lowering `eor_timeout` makes a slow but correct replay fail | Derive the observer timeout from the `.ci` timeout rather than picking a second constant |

## Blast Radius

`internal/test/peer/peer_connmap.go`, `test/scripts/ze_api.py`, and any `.ci` whose
timeout is shorter than the observer default. No product code. A false green removed
here may expose tests that were passing for the wrong reason, which is the point.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A batch accept cancelled before the checker completes | → | `acceptConnMapBatch` | `TestCancelledAcceptDoesNotReportSuccess` |
| A batch accept cancelled after the checker completes | → | `acceptConnMapBatch` | `TestCancelledAcceptAfterCompletionStillPasses` |
| An observer whose replay exceeds its budget | → | `run_rs_observer` | a `.ci` whose timeout exceeds the observer timeout |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ln.Accept()` fails with the context done, and the checker has NOT completed | `Result.Success` is false and the error names cancellation before completion. **MET**, `acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`); `TestCanceledAcceptReportsFailureUntilCheckerCompletes/expectation_outstanding` |
| AC-2 | `ln.Accept()` fails with the context done, and the checker HAS completed | `Result.Success` is true, unchanged from today. **MET**, same test, `/expectations_satisfied` |
| AC-3 | `run_rs_observer` runs inside a `.ci` whose foreground timeout is shorter than `eor_timeout` | Either the observer timeout is derived from the `.ci` budget, or the mismatch is reported as a harness error rather than silently unreachable. **MET by DERIVATION**: `(*Runner).testBudgetEnv` (`internal/test/runner/runner_exec_util.go`) publishes `ze_test_budget`, the headroom-scaled deadline the child actually races, and `run_rs_observer` (`test/scripts/ze_api.py`) takes 60% of it for `eor_timeout` and 25% for `shutdown_timeout`. A share below 1.0 is reachable at every budget, which no constant can be |
| AC-4 | Every `.ci` invoking `run_rs_observer` | Surveyed, and none leaves the diagnostic unreachable. **MET**: surveyed 2026-08-09, all 17 call sites were 10s (3), 15s (9) or 20s (5) against a 30.0s default, so every one was unreachable. The derivation in AC-3 fixes all 17 at once, and no `.ci` needed editing. `shutdown_timeout` had the same defect at 15.0s and is fixed with it |
| AC-5 | The full functional suite, before and after | Same set of passing tests, or a named test whose green was false, with the evidence. **MET for guard 1**: `make ze-plugin-test` 602/602 PASS, exit 0, which is where all 15 `connmap` `.ci` files live and so is the population `acceptConnMapBatch` can affect |
| AC-6 | Guard 1 mutated to return success unconditionally | `TestCanceledAcceptReportsFailureUntilCheckerCompletes` turns red. **MET**, mutation run 2026-08-09: `result.Success = true, want false` |
| AC-7 | `show bgp rib status` fails, or answers without a `routes-in` field | `Ze.rib_count` raises and names the container and the command. It never returns 0. **MET** |
| AC-8 | Any `ze:command` a built-in declares, typed as a shell argv with its verb | resolves, and dispatches on that same absolute path. **MET**, `TestDeclaredCommandsResolveFromArgv` |
| AC-9 | The word alignment in `ResolveCommand` mutated back | `TestDeclaredCommandsResolveFromArgv` turns red. **MET**, 305 paths red on the mutation |
| AC-10 | `make ze-interop-test INTEROP_SCENARIO=05-routes-from-frr` | passes on a real received-route count. **MET**, 3 routes; `06` 3, `13` 1 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Interrupts a functional run partway | runner → context cancel → `acceptConnMapBatch` | `TestCancelledAcceptDoesNotReportSuccess` |
| 2 | Runs an RS test whose replay stalls | `.ci` → `run_rs_observer` → diagnostic | The `.ci` added under AC-3 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCancelledAcceptDoesNotReportSuccess` | `internal/test/peer/peer_connmap_test.go` | AC-1, AC-6 | |
| `TestCancelledAcceptAfterCompletionStillPasses` | `internal/test/peer/peer_connmap_test.go` | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `eor_timeout` versus the `.ci` foreground timeout | observer budget must be under the `.ci` budget | `.ci` timeout minus the teardown margin | 0, no time to observe | any value at or above the `.ci` timeout, which is the defect |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an RS observer test with a reachable diagnostic | `test/draft/plugin/` then promoted | A stalled replay reports `ZE-OBSERVER-FAIL` rather than a bare timeout | |

### Interop Tests (Scope: tooling)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | none | none | Harness only, no wire behavior and no peer daemon involved | |

## Files to Modify
- `internal/test/peer/peer_connmap.go` - guard 1
- `test/scripts/ze_api.py` - guard 2
- `docs/functional-tests.md` - the observer's timeout contract, once AC-3 settles how the two budgets relate
- any `.ci` the AC-4 survey names

Guard 3, done 2026-08-07:
- `test/interop/interop.py` - `Ze.rib_count` raises; `Ze.cli`; `ZE_CLI_CONFIG` appended to every rendered `ze.conf`; `docker_exec` takes `env`
- `cmd/ze/internal/cmdutil/cmdutil.go` - `ResolveCommand` aligns argv with the verb-relative tree
- `internal/component/cli/client/verb_tree.go` - the verb tree and `AbsoluteVerbPath`, split out of `main.go`
- `cmd/ze/internal/cmdutil/cmdutil_test.go` - `TestDeclaredCommandsResolveFromArgv` and three named cases
- `test/interop/scenarios/13-graceful-restart-frr/frr.conf` - a prefix for FRR to advertise

Note for a future reader: every file above lives in the test harness, and the
`validate-spec.sh` feature-integration check reads that as a tests-only spec. The
harness IS the product here. A guard that fails open turns a run that proved nothing
into a green bar, so the harness is exactly where this work belongs.

## Files to Create
- `internal/test/peer/peer_connmap_test.go` if it does not already exist
- one functional `.ci` under `test/draft/plugin/`, promoted once green

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | Test harness only |
| CLI commands/flags | No | None |
| Functional test for new RPC/API | Yes | The AC-3 `.ci` |
| Doctor check for runtime dependencies | No | No runtime dependency added |
| Prometheus counters/metrics | No | No observable daemon state |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Harness internals |
| 2 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the observer's timeout contract changes |
| 3 | Wire format changed? | No | None |
| 4 | Internal architecture changed? | No | None |

## Implementation Steps

1. Read `checker.Completed()` and confirm A-2.
2. Write the two guard 1 tests, watch them fail, then tighten the guard.
3. Run the full functional suite and compare the passing set against A-1.
4. Survey every `.ci` calling `run_rs_observer` against its own timeout (AC-4).
5. Derive the observer budget from the `.ci` budget, or make the mismatch an error.
6. Add the functional `.ci` for AC-3 and promote it once green.

## Design Insights

Two timeouts that must be ordered, chosen independently in two files, will drift. The
repair is to derive one from the other, not to pick better constants
(`ai/rules/evidence.md`).

## Key Design Decisions

Not yet taken.

## Known Limitations

Removing a false green can turn tests red that were passing for the wrong reason.
That is the intended outcome and must not be treated as a regression caused by this
spec.

## RFC Documentation (Scope: tooling)

No RFC behavior. This spec changes the test harness only. No requirement id is added,
retired or re-levelled, so `make ze-rfc-check` is unaffected.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features, or N-A with a reason

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Provenance

The independent review of `spec-rfcgate-2-deferred-rs-replay-evidence`,
2026-08-02, items 3 and 4 of its "left for someone else" list.

### Homed here 2026-08-07: a third harness claim that names the wrong producer

Found while verifying round 6 of
`spec-wire-edit-4-api-origin-deferred-bird-interop` (closed 2026-08-07 in `2cc75ab5f`), outside that round's
scope, so it is homed rather than fixed there.

`wait_peer_eor_sent` (`test/scripts/ze_api.py`) is the barrier every EOR-asserting
functional test holds, and its docstring says the `eor-sent` counter "is
incremented by `IncrEORSent` ..., which is called only from `sendInitialRoutes`".
`IncrEORSent` (`internal/component/bgp/reactor/peer_stats.go`) has FOUR non-test
callers: two in `(*Peer).sendInitialRoutes`
(`internal/component/bgp/reactor/peer_initial_sync.go`), one in
`(*reactorAPIAdapter).AnnounceEOR`
(`internal/component/bgp/reactor/reactor_api_forward.go`), and one in
`internal/component/bgp/reactor/reactor_api_batch.go`.

The barrier's own meaning survives, which is why this is not a fail-open guard:
non-zero still means a marker reached the wire, whichever producer sent it. What
does not survive is the attribution a reader derives from it. A test author
reasoning "eor-sent can only come from the initial sync" will mis-derive on any
peer whose route server or batch path also sends End-of-RIB. The same docstring
also carries a line-number citation, which `ai/rules/writing.md` no longer allows.
