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

---

## Implementation Summary

### What Was Implemented

- **Guard 1.** `acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`)
  consults `(*Checker).Completed` (`internal/test/peer/checker.go`) on the
  context-cancelled accept. An outstanding expectation now answers
  `Result{Success: false}` with an error naming cancellation before completion.
  A settled checker still answers success.
- **Guard 2.** `(*Runner).testBudgetEnv` (`internal/test/runner/runner_exec_util.go`)
  publishes `ze_test_budget`, the headroom-scaled wall clock the child races.
  `run_rs_observer` (`test/scripts/ze_api.py`) derives `eor_timeout` as 60% of
  it and `shutdown_timeout` as 25%. A share below 1.0 is reachable at every
  budget, which no constant can be.
- **Guard 3.** `Ze.rib_count` (`test/interop/interop.py`) raises on a failed
  command and on a response with no `routes-in` field, naming the container and
  the command. `ResolveCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) and
  `AbsoluteVerbPath` (`internal/component/cli/client/verb_tree.go`) repair the
  verb resolution that guard 3 uncovered.
- **Guard 4.** `scripts/dev/docker_exec_checked.py` derives the fail-open
  function set to a fixpoint and classifies every call site of any member.
  `make ze-docker-exec-check` runs it, `test/health/docker-exec-baseline.json`
  holds the floor at 171, and `is_docker_exec_source`
  (`scripts/dev/verify_wiring_docs.py`) routes it onto the changed-file path.

### Bugs Found/Fixed

- A cancelled run reported a pass it never earned. Covered by
  `TestCanceledAcceptReportsFailureUntilCheckerCompletes`
  (`internal/test/peer/peer_connmap_test.go`).
- `ZE-OBSERVER-FAIL` was unreachable at all 17 `.ci` call sites. Covered by
  `TestObserverBudget` (`test/scripts/ze_api_test.py`).
- `ze <verb> ...` resolved almost nothing from a shell argv. Covered by
  `TestDeclaredCommandsResolveFromArgv` (`cmd/ze/internal/cmdutil/cmdutil_test.go`).
- `_emptiness_tested` (`scripts/dev/docker_exec_checked.py`) matched a
  same-named variable anywhere in the function, so three live sites rode out on
  a guard belonging to an earlier call. The rule is positional now, and the
  floor rose from 168 to 171 because the detector got more correct.
- Guard 2's own proof was vacuous, found at the closure review. `TestObserverBudget`
  (`test/scripts/ze_api_test.py`) recomputed `budget * _EOR_BUDGET_SHARE` itself
  and never called `run_rs_observer`, so restoring the 30.0 constant inside the
  producer left every assertion green. That is the shape guard 2 exists to
  refuse, one level up. Covered by `TestObserverBudgetReachesTheWaits`.

### Documentation Updates

- `docs/architecture/testing/interop.md` already carries guard 3's contract for
  `Ze.rib_count`. Its citation of this spec is restated as the bare stem, because
  the committed tree no longer holds the file.
- `ai/rules/repo-maintenance.md` carries the `ze-docker-exec-check` gate row and
  `ai/INDEX.md` carries the tool entry. Both landed with the other session's
  regenerated points.
- `docs/functional-tests.md` needed no edit: it documents the `.ci` format and
  the observer's public arguments, and neither changed. The derivation replaced
  two defaults inside `run_rs_observer` and added no argument.
- `make ze-doc-test` result is in Pre-Commit Verification below.

### Deviations from Plan

| Planned | What landed | Why |
|---------|-------------|-----|
| `TestCancelledAcceptDoesNotReportSuccess` and `TestCancelledAcceptAfterCompletionStillPasses` | One table-driven `TestCanceledAcceptReportsFailureUntilCheckerCompletes` with subtests `expectation_outstanding` and `expectations_satisfied` | Both cases share one listener setup and one cancelled context. Two functions would have duplicated it |
| A functional `.ci` under `test/draft/plugin/`, promoted once green | None written; `TestObserverBudgetReachesTheWaits` (`test/scripts/ze_api_test.py`) written instead | AC-3 was met by derivation rather than by a second constant, so all 17 existing call sites became reachable at once and no `.ci` needed editing or adding. A `.ci` proves the diagnosis fires; this test proves the number that makes it fire reaches the wait, which is what the derivation changed |
| Guard 4 as a ruling to take | Guard 4 implemented in full | Thomas ruled on 2026-08-09 for the wider lint: every fail-open return value is checked, and `docker_exec_quiet` keeps its contract |
| AC-5 names 15 `connmap` `.ci` files | 17 | A miscount in the spec prose. `grep -rl connmap test/plugin/` answers 17 |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Guard 2 was to be fixed by picking a shorter `eor_timeout` constant | Any constant is either unreachable or larger than the run. The budget must be DERIVED from the deadline the runner enforces | The AC-4 survey: all 17 call sites sat under the 30.0s default, so no single constant could serve them | `(*Runner).testBudgetEnv` publishes the budget and the observer takes a share of it |
| approach | Guard 4's detector treated a membership test as a check | `if prefix in out` is False on `""`, so it is the fail-open shape rather than a guard against it | The survey's own worked true positive, `FRR.is_dis` (`test/interop/interop.py`), which reads `"DIS" in out` | The rule flags membership tests, and the floor moved from 114 to 168 |
| escalation | The guard 4 checker's own emptiness rule was position-blind | An emptiness test on a same-named variable marked EVERY assignment of that name checked, `FRR.route_count` among them | Review round 1 | The rule is positional, two tests pin it, and the floor rose to 171 |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A cancelled accept must not report success | Done | `acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`) | Consults `(*Checker).Completed` |
| A harness timeout must be reachable | Done | `(*Runner).testBudgetEnv` (`internal/test/runner/runner_exec_util.go`), `run_rs_observer` (`test/scripts/ze_api.py`) | Shares of the published budget |
| A failed RIB query must not answer 0 | Done | `Ze.rib_count` (`test/interop/interop.py`) | Raises on both failure modes |
| A mechanical guard against the swallowed call | Done | `scripts/dev/docker_exec_checked.py` | `make ze-docker-exec-check`, floor 171 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCanceledAcceptReportsFailureUntilCheckerCompletes/expectation_outstanding` | `acceptConnMapBatch` returns the named error |
| AC-2 | Done | same test, `/expectations_satisfied` | Success preserved |
| AC-3 | Done | `TestObserverBudgetReachesTheWaits` (`test/scripts/ze_api_test.py`), `(*Runner).testBudgetEnv` | Met by derivation, proven at the producer |
| AC-4 | Done | Survey of 17 call sites | The derivation fixes all 17 at once |
| AC-5 | Done | `make ze-plugin-test` 602/602 PASS, 2026-08-09 | The 17 `connmap` `.ci` are that suite |
| AC-6 | Done | Mutation run 2026-08-09: `result.Success = true, want false` | Test discriminates |
| AC-7 | Done | `Ze.rib_count` (`test/interop/interop.py`) | Two raise paths, both name container and command |
| AC-8 | Done | `TestDeclaredCommandsResolveFromArgv` (`cmd/ze/internal/cmdutil/cmdutil_test.go`) | |
| AC-9 | Done | Mutation: 305 paths red | Test discriminates |
| AC-10 | Done | `make ze-interop-test INTEROP_SCENARIO=05-routes-from-frr`, 2026-08-07 | 3 routes; `06` 3, `13` 1 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCancelledAcceptDoesNotReportSuccess` | Changed | `internal/test/peer/peer_connmap_test.go` | Landed as `TestCanceledAcceptReportsFailureUntilCheckerCompletes/expectation_outstanding` |
| `TestCancelledAcceptAfterCompletionStillPasses` | Changed | same file | Landed as the `/expectations_satisfied` subtest |
| An RS observer test with a reachable diagnostic | Changed | `test/scripts/ze_api_test.py` | `TestObserverBudgetReachesTheWaits` drives `run_rs_observer` and reads back each wait's timeout; no `.ci` was needed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/peer/peer_connmap.go` | Done | Guard 1 |
| `test/scripts/ze_api.py` | Done | Guard 2 |
| `docs/functional-tests.md` | Changed | No edit needed: no public argument and no `.ci` directive changed |
| Any `.ci` the AC-4 survey names | Changed | None needed editing; the derivation covers all 17 |
| `internal/test/peer/peer_connmap_test.go` | Done | Created |
| One `.ci` under `test/draft/plugin/` | Skipped | Superseded by the derivation, recorded in Deviations |
| `test/interop/interop.py`, `cmd/ze/internal/cmdutil/cmdutil.go`, `internal/component/cli/client/verb_tree.go`, `cmd/ze/internal/cmdutil/cmdutil_test.go`, `test/interop/scenarios/13-graceful-restart-frr/frr.conf` | Done | Guard 3 |

### Audit Summary
- **Total items:** 24
- **Done:** 19
- **Partial:** 0
- **Skipped:** 1 (the draft `.ci`, superseded by AC-3's derivation)
- **Changed:** 4 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A cancelled run must not report success | functional (unit over the harness) | `TestCanceledAcceptReportsFailureUntilCheckerCompletes`. Mutating the guard to return success unconditionally turns it red: `result.Success = true, want false`. The test refuses to run at all if the two fixture checkers do not differ, so it cannot pass vacuously |
| The observer diagnostic must be reachable | functional (unit over the harness) | `TestObserverBudgetReachesTheWaits.test_the_waits_take_shares_of_the_published_budget` (`test/scripts/ze_api_test.py`) runs `run_rs_observer` over a stubbed session and asserts the number it hands `wait_rs_replayed` and `wait_for_shutdown`. Reverting either default to its old constant turns it red at `30.0 != 12.0`, measured 2026-08-14. `TestObserverBudget.test_an_unreadable_duration_raises_rather_than_reading_zero` refuses the zero that would make every derived wait expire at once |
| A failed RIB query must not answer 0 | interop | `make ze-interop-test INTEROP_SCENARIO=05-routes-from-frr` passes on 3 real received routes, and `06` on 3, `13` on 1. Before the fix, scenario 05 was red on `Ze RIB has 0 received routes` with three distinct faults behind that 0 |
| Nothing may write the swallowed call again | tooling gate | `make ze-docker-exec-check` refuses a rise over the 171 floor in `test/health/docker-exec-baseline.json`. `TestRepoRatchet` re-runs the real scan under `make ze-unit-test`, so the floor holds without the changed-file routing |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Changed-file routing for `ze-docker-exec-check` | done | Both are in HEAD. `is_docker_exec_source` is in `scripts/dev/verify_wiring_docs.py` and `DockerExecRoutingTest` is in its sibling test. The interleaving cleared when the other session committed its refactor. Verified by reading both files, not by a commit message |
| The `ze-docker-exec-check` gate row and its `ai/INDEX.md` entry | done | The gate row is in `ai/rules/repo-maintenance.md` and the tool entry is in `ai/INDEX.md`, both naming `make ze-docker-exec-check` |
| 171 unchecked fail-open call sites across 67 files | resolved | Homed at `plan/future/spec-fail-open-call-site-drain.md`. Not commissioned: the row has a destination so this shard can close, not a schedule. The floor may only go DOWN, so the count cannot grow while it waits |
| `plan/deferrals/fixit-interop-bare-verb-guard.md`, its one row | done | That row IS guard 4, and guard 4 shipped. The shard is now all-terminal residue, and this closure removes it (`ai/rules/planning.md`: the actor is the closer of the last spec that homed one of its rows) |

**Nine live rows in FOUR FOREIGN shards named this spec as their Destination,
and all nine are re-homed in commit A.** `ai/rules/planning.md` ("Closure
resolves the spec's deferral rows") makes this BLOCKING: "Every row naming it as
**Destination** MUST be resolved inside commit A", and its Banned table names
the alternative outright, "`git rm` a spec while a deferral row still names it
as Destination -- The row dangles forever."

| Shard | Rows | New Destination |
|-------|------|-----------------|
| `plan/deferrals/wire-edit-4-api-origin-deferred-bird-interop.md` | 2 | `plan/future/spec-harness-fail-open-guard-backlog.md` |
| `plan/deferrals/fixit-ospf-sr-missing-label-passes.md` | 1 | same |
| `plan/deferrals/fixit-firewall-concurrency-deadlock.md` | 1 | same |
| `plan/deferrals/rules-as-points.md` | 5 | same |

Each row keeps Status `deferred`, because the rule forbids closing a row on
filing: "a `done` row is never destination-checked again, so closing it on
filing is precisely how the work stops being watched."

**Why one destination rather than nine.** A survey on 2026-08-14 found a better
per-row home for each: two journal-class rows, three existing live specs, and
two new skeletons. Applying it would add Task items to
`plan/spec-interop-suite-red.md`,
`plan/spec-fixit-relax-audit-reports-the-wrong-token.md` and
`plan/spec-rules-situation-index.md`, which changes THEIR scope, and would
create two specs Thomas has not commissioned. A closure may not take that
decision. The survey is recorded in full in the backlog spec's own table, so the
routing work is preserved rather than repeated, and none of it is applied.

One more citation, in a row that was already terminal:
`plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` names this spec as
the Destination of guard 3's `done` row. It is restated as the bare stem, because
`check_tracked_citations` (`scripts/dev/check_doc_links.py`) sweeps every tracked
file and the path form goes dead when commit B lands.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-test-harness-fail-open-guards-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` |
| `review_gate.py check` | `review_gate: OK (3 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | Round 1, two parallel independent lenses: logic+wiring+evidence over the closure diff, and gate-correctness+bookkeeping-integrity over the deferral and citation moves. Round 2 re-reviewed the fixes |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Nine LIVE rows in four foreign shards named this spec as Destination. `ai/rules/planning.md` requires every one to be resolved inside commit A, and its Banned table names `git rm` of a spec a live row still points at | `plan/deferrals/` (wire-edit-4 ×2, ospf-sr ×1, firewall ×1, rules-as-points ×5) | All nine re-pointed at `plan/future/spec-harness-fail-open-guard-backlog.md`, Status kept `deferred`. The per-row survey is recorded in that file rather than applied, because applying it would expand three live specs' scope |
| 2 | ISSUE | AC-3's proof was vacuous. `TestObserverBudget` recomputed `budget * _EOR_BUDGET_SHARE` itself and never called `run_rs_observer`, so restoring the 30.0 constant in the producer left every assertion green. The Wiring Test table is marked NOT deferrable | `test/scripts/ze_api_test.py` | `TestObserverBudgetReachesTheWaits` drives `run_rs_observer` over a stubbed session and reads back the timeout each wait receives. Mutation measured 2026-08-14: the reverted constant fails it at `30.0 != 12.0` |
| 3 | ISSUE | The shard's Status cells carried prose (`done 2026-08-14: ...`, `homed ...`). `DEFERRAL_TERMINAL_STATUSES` (`scripts/dev/commit_helper.py`) matches the WHOLE cell, so all three read as live and the shard removal would have been refused | `plan/deferrals/fixit-test-harness-fail-open-guards.md` | Status cells reduced to `done`, `done`, `resolved`; the evidence moved to the Deferrals Resolved table above |
| 4 | ISSUE | Removing the spec would leave twelve dead path citations in tracked files, which `check_tracked_citations` (`scripts/dev/check_doc_links.py`) sweeps repo-wide. The first count of five missed every backticked Destination cell in `plan/deferrals/` | five source and doc files, plus seven Destination cells across three shards | The five restated as the bare stem; the seven fixed by finding 1's re-point, and the two terminal ones restated. `git ls-files | xargs grep` now finds the path form only in `scripts/dev/doc_citation_baseline.txt` |
| 5 | ISSUE | `plan/deferrals/fixit-interop-bare-verb-guard.md` held guard 4 as `deferred` after guard 4 shipped, so the shard asserted a defect the tree no longer has | that shard | Row set to `done`, header records the ruling, shard removed as residue |
| 6 | NOTE | The closure sections were appended between `## Provenance` and its own subsection, so `### Homed here 2026-08-07` came to sit under `## Core Insight` | this spec | The subsection moved back under `## Provenance` |
| 7 | NOTE | "171 unchecked sites across 66 files" is not reproducible from the tool | this spec, its shard, `plan/future/spec-fail-open-call-site-drain.md` | Re-derived from `docker_exec_checked.scan`: 171 sites across **67** files. Corrected in all three |
| 8 | NOTE | Guard 2 is inert on the non-orchestrated path: `runTest` (`internal/test/runner/runner_exec.go`) builds `clientEnv` without `testBudgetEnv`, so only `runOrchestrated` publishes the budget | `internal/test/runner/runner_exec.go` | Not fixed. All 17 observer callers carry `cmd=foreground` and so take `runOrchestrated`, so the claim holds today. A future observer `.ci` with no `cmd=` line would silently return to the 30.0s constant. Recorded here and in the journal row |
| 9 | ISSUE | `plan/future/spec-harness-fail-open-guard-backlog.md` holds nine subtasks in one file, and "Deferral Spec Naming (BLOCKING)" (`ai/rules/planning.md`) says "MUST use one subtask per file" under `plan/spec-<source>-deferred-<subtask>.md` | the new backlog spec | **Not fixed; a reading is taken and declared, per `ai/rules/rule-precedence.md`.** That naming rule governs a spec created to hold work deferred OUT of one source spec, and derives the filename from that single `<source>`. These nine rows were homed AT the closing spec by FOUR different sources, so there is no `<source>` to name. Nine skeletons would also contradict "Choosing the Destination Spec", whose step 1 prefers an existing spec and whose survey found one for five of the nine. The consequence is bookkeeping only: every row resolves, `deferral_destination_problem` (`scripts/dev/commit_helper.py`) answers `None`, and nothing dangles. Flagged for Thomas in the closure report |
| 10 | NOTE | `scripts/dev/doc_citation_baseline.txt` held a grandfathered dead-citation entry for this spec, which becomes residue once the spec is gone | that file | The one line removed. The file shrank 1338 to 1337, and the banned `--write-baseline` regeneration was not used |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/peer/peer_connmap_test.go` | Yes | Holds `TestCanceledAcceptReportsFailureUntilCheckerCompletes` |
| `scripts/dev/docker_exec_checked.py` | Yes | 22K, with sibling `docker_exec_checked_test.py` at 15K |
| `test/health/docker-exec-baseline.json` | Yes | `{"unchecked": 171}` |
| `plan/future/spec-fail-open-call-site-drain.md` | Yes | Created 2026-08-14, the destination of the third shard row |
| One `.ci` under `test/draft/plugin/` | No | Not created. Superseded by AC-3's derivation, recorded in Deviations |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The guard consults the checker | Read in `acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`): `if !p.checker.Completed()` returns `Success: false` with `errors.New("accept canceled before the checker completed its expectations")`, else `Success: true` |
| AC-3 | The budget is derived, not a constant | `(*Runner).testBudgetEnv` (`internal/test/runner/runner_exec_util.go`) emits `ze_test_budget`; `run_rs_observer` (`test/scripts/ze_api.py`) takes `_EOR_BUDGET_SHARE` 0.60 and `_SHUTDOWN_BUDGET_SHARE` 0.25. `TestObserverBudgetReachesTheWaits` proves the producer hands those values on, and goes red on the reverted constant |
| AC-4 | Every call site is covered | `grep -rl run_rs_observer test/plugin/` answers 16 `.ci`, 17 across `test/`, and none is edited: the derivation is inside the helper |
| AC-6, AC-9 | The tests discriminate | Mutation runs recorded 2026-08-09. Guard 1: `result.Success = true, want false`. `ResolveCommand`: 305 paths red |
| AC-7 | A failed RIB query raises | Read in `Ze.rib_count` (`test/interop/interop.py`): both the `RuntimeError` path and the missing-`routes-in` path raise, each naming `self.container` and the command |
| AC-8 | Declared commands resolve from argv | `TestDeclaredCommandsResolveFromArgv` (`cmd/ze/internal/cmdutil/cmdutil_test.go`) |
| AC-5, AC-10 | Suite and interop results | Recorded from the 2026-08-09 and 2026-08-07 runs. Not re-run in this closure: the interop scenarios need Docker, which this host does not run |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A batch accept cancelled before the checker completes | none; `internal/test/peer/peer_connmap_test.go` | Yes. The test drives `acceptConnMapBatch` with a closed listener and a cancelled context, which is the exact branch, and fails the run if the two fixture checkers do not differ |
| A batch accept cancelled after the checker completes | same file | Yes. The `/expectations_satisfied` subtest asserts success is preserved |
| An observer whose replay exceeds its budget | none; `test/scripts/ze_api_test.py` | Yes, over the producer rather than by a `.ci`. `TestObserverBudgetReachesTheWaits` drives `run_rs_observer` itself and reads back the timeout it gives each wait, so the assertion sits on the path the change altered. No `.ci` can hold the old defect, because the value is derived inside `run_rs_observer` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `make ze-plugin-test` 602/602 PASS, exit 0, 2026-08-09. That suite holds every `connmap` `.ci`, so it is the population `acceptConnMapBatch` can affect |
| A-2 | confirmed | `(*Checker).Completed` (`internal/test/peer/checker.go`) exists and `acceptConnMapBatch` calls it as `p.checker.Completed()` |
| A-3 | broken, in the direction that widened the fix | The mismatch was not one `.ci`: all 17 call sites ran 10s, 15s or 20s against a 30.0s default, so the diagnostic had never been reachable anywhere. Recorded in the Mistake Log |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Test infrastructure changed (checklist row 2) | `docs/architecture/testing/interop.md` states `Ze.rib_count` raises on failure, which matches `Ze.rib_count` (`test/interop/interop.py`) | Yes |
| `docs/functional-tests.md` needs no edit | `grep -n "eor_timeout\|run_rs_observer\|ZE-OBSERVER-FAIL\|shutdown_timeout" docs/functional-tests.md` matches nothing, so the page states no claim the derivation could make stale. The `.ci` format and `run_rs_observer`'s public arguments are unchanged | Yes |
| New user-facing feature, wire format, internal architecture (rows 1, 3, 4) | Harness only. No YANG leaf, no CLI verb and no wire byte changed | No for each |

## Core Insight

Two budgets chosen independently in two files will drift, and no third constant
repairs that. The fix is to publish the one the runner ENFORCES and let every
inner wait take a share below 1.0 of it. A share is reachable at every budget;
a constant is reachable at none or at all, and which one is an accident of the
`.ci` that happens to call it.

