# Spec: interop-suite-red -- resolve the recorded interop failures and their evidence gaps

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Resolve the failures recorded on 2026-08-05 and the additional obligations from
the 2026-08-07 investigation. The original five-red count is a dated baseline,
not the current suite result. The runner is now native Go under
`internal/le/interoplab/bgp`; the retired Python runner is historical evidence.
Every surviving failure requires a repair and passing evidence. A root-cause
row records diagnosis and cannot satisfy this repair milestone.

### Current obligation set (reconciled 2026-09-19)

| Scenario or surface | Latest evidence available here | Remaining obligation |
|---------------------|--------------------------------|----------------------|
| `bgp-routes-from-frr`, `bgp-routes-from-bird` | Recorded PASS on 2026-08-07 after CLI/listener/helper repairs; native `checkers.go` now requires three received routes through JSON fields | Current regression evidence; do not repeat the disproved missing-plugin diagnosis |
| `bgp-route-withdrawal-frr`, `bgp-ipv6-ebgp-frr`, `bgp-addpath-frr` | Original failure record; native checker/process producers now own these scenarios | Establish present verdict, repair any survivor and retain the withdrawal, IPv6 and ADD-PATH assertions |
| `bgp-srv6-frr` | Added Active/config failure on 2026-08-07; native checker exists | Establish present verdict and resolve any surviving session or config defect |
| `bgp-routes-gobgp` | Added retired-helper failure; native `opGoBGPRoute` now queries GoBGP directly and the checker requires Ze's received routes | Current route-exchange proof; a changed helper is not a passing scenario |
| `ospf-gr-frr`, `ospf-gr-fib-retention`, `ospfv3-gr-frr`, `ospfv3-gr-fib-retention` | Shared config failure recorded on 2026-08-07. Current `ospf-gr-frr` checks FRR adjacency/database and restart recovery, not a BGP count | Prove the configs start and the required OSPF route-retention behaviour; adjacency alone cannot discharge the retained route-retention obligation |
| `bgp-graceful-restart-frr` | Recorded PASS on 2026-08-07 after adding the missing advertised prefix | Preserve that repair with current regression evidence |
| Fail-closed route-count reader | `runOperation` propagates query errors; `requireJSONFields` rejects non-JSON, absent or nonnumeric fields | Prove those failures remain failures through the native runner and name the command/scenario |

No current scenario run was made in this reconciliation. All rows remain owned
here until their required evidence is recorded; no failure has been transferred
or waived.

### Historical baseline

**Measured 2026-08-05**, each run twice, once with the working tree's
`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> and once with HEAD's, with identical results:

| Scenario | Symptom |
|----------|---------|
| `bgp-routes-from-frr` | `Ze RIB has 0 received routes (expected >= 3)` |
| `bgp-routes-from-bird` | `Ze RIB has 0 received routes (expected >= 3)` |
| `bgp-route-withdrawal-frr` | route absent |
| `bgp-ipv6-ebgp-frr` | session stuck Active for the whole 90s budget |
| `bgp-addpath-frr` | plugin dies ~6s in on a rejected token; **root cause found, see below** |

`06` was re-run directly by the main thread and reproduced: BIRD established the
session and exported 3 routes, and ze reported 0.

### 2026-08-07: the RIB reds are fixed, and the fail-open helper with them

`05` and `06` shared one cause after all (A-2 **confirmed**), and it was three
faults stacked behind the single `0` this spec predicted would hide them:

| Fault | Producer | Fix |
|-------|----------|-----|
| The verb form resolved nothing, for ANY verb. 56 of the 63 `ze show` subcommands answered `unknown command` on the HOST, and `clear` / `monitor` / `request` / `set` / `delete` with them | `RunCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) walked the verb-RELATIVE tree from `cli.BuildVerbCommandTree` with the verb-INCLUSIVE argv | `ResolveCommand` aligns the words; `cli.AbsoluteVerbPath` (`internal/component/cli/client/verb_tree.go`) rebuilds every absolute form |
| The interop daemon started no SSH listener, so no CLI client could reach it whatever the verb | `infraSetup` (`cmd/ze/hub/infra_setup.go`) starts one only when the config asks, and no scenario `ze.conf` asked | `ZE_CLI_CONFIG` appended to every RENDERED `ze.conf`, queried through `ze cli -c`, as `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> already did |
| `Ze.rib_count` returned 0 on failure, which is what made the two above indistinguishable | `Ze.rib_count` (`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) | raises, naming the command and the container (AC-1 **met**) |

It was NOT the `use bgp-rib` producer this spec named as the first thing to read.
The plugin loads; nothing could ask it anything.

`05` PASS (3 routes), `06` PASS (3), `bgp-graceful-restart-frr` PASS (1). `13` was
not on the list above and was red for its own reason: its `frr.conf` advertised
nothing at all, so `ze.rib_received(1)` could not pass on any run. It now
advertises one prefix.

R-1 fired exactly as written: making the helper fail closed revealed reds that a
masked zero had been carrying. Their rows are below, and none was silenced.

| Scenario | Symptom | Verified |
|----------|---------|----------|
| `bgp-srv6-frr` | Session never leaves Active over the 90s budget, so the RIB is never queried. Ze LISTENS (TCP connect to `172.30.0.2:179` from the FRR container succeeds) and logs nothing about the attempt, while FRR reports `remote router ID 0.0.0.0`. FRR also reports `Configuration file[/etc/frr/frr.conf] processing failure: 11`, so whether the fault is Ze's or the scenario's SRv6 config is OPEN | Reproduced with the config append disabled, so it predates this work and is not caused by it. The producer that should log an accepted connection has NOT been read |
| `bgp-routes-gobgp` | `route_json returned no data`: `GoBGP.route_json` (`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) answers None and the check raises BEFORE any RIB query | The failure is in the GoBGP-side helper, not the Ze-side one. `rib_count` is never reached |
| `ospf-gr-frr` | **Ze never starts: its own scenario config does not parse.** `ze validate config test/interop/scenarios/ospf-gr-frr/ze.conf` answers `line 11: expected ';' after support value, got WORD`. Line 11 is `restarter { support planned restart-interval 120 }`: two leaves on one line with no separator, which `p.errorf` (`internal/component/config/parser.go`) refuses. The same line is in `ospf-gr-fib-retention`, `ospfv3-gr-frr` and `ospfv3-gr-fib-retention` | Reproduced against the UNMODIFIED file with a host `ze` build, so it is neither the container nor this work. **Which side is wrong is a real question, not bookkeeping**: either the four configs are missing `;` or the parser should accept the compact form. Answer it before editing either |

**A trap waiting behind the `ospf-gr-frr` config fix.** Its check calls
`Ze().rib_received(0)`, and that scenario configures no BGP at all. The call was
a tautology (`count >= 0`) over a helper that answered 0 for a failed query, so
it passed without asking anything. `rib_count` raises now, so whoever fixes the
config meets a red on a BGP surface an OSPF scenario never configures. The
replacement is an OSPF route-retention assertion, not a deleted line: the
scenario's own comment already points at `ospf-gr-fib-retention` for the real
property.

## Historical reporting defect (2026-08-05, repaired in the August 7 account)

`Ze.rib_count` (`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) ends `return 0` when its command
produces no parseable output. So "the daemon does not answer this verb" and "the
daemon received no routes" are the same number, and every caller asserting a
lower bound reports the second.

**Its own docstring records that this already happened once**: the helper read
`show rib status` until 2026-08-04, the daemon answered `unknown command`, and it
returned 0 for every caller. The verb was corrected; the fail-open was not.
Measured again on 2026-08-05: `ze show bgp rib status` answers `unknown command`
in the scenario container, so the helper is masking a second fault the same way.

That shape is the same one that produced a BLOCKER in
`spec-wire-edit-4-api-origin-deferred-bird-interop` (closed 2026-08-07 in `2cc75ab5f`) round 3, where
`API.peer_counter` returned its default of 0 on an unreadable lookup and a guard
read 0 as permission to proceed. `ai/rules/evidence.md`: a zero value must never
be a valid-looking answer.

The initial investigation suspected plugin loading or command registration.
The 2026-08-07 producing-function account above disproved the missing-plugin
explanation. It is not a current first step.

## Historical ADD-PATH diagnosis (2026-08-05)

The scenario sends `path-information` as a TOP-LEVEL token
(`test/interop/scenarios/bgp-addpath-frr/announce-addpath.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->). `ParseUpdateText`
(`internal/component/bgp/plugins/cmd/update/update_text.go`) rejects it there, and
the plugin dies on the `RuntimeError`.

**The error message advertises the keyword it is rejecting.** Its text lists
`path-information (info)` among the valid tokens, so an operator who reads it and
retries at the same position fails again. The keyword is real but belongs
elsewhere: `kwPathInfo` is consumed by the NLRI-section parser
(`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`), whose own
comment states "info/path-information is per-NLRI-section, not top-level".

So there are two defects, and they need separating before either is fixed:

| Defect | Where |
|--------|-------|
| The top-level error lists a keyword that is not valid at top level, pointing the reader at the mistake that produced it | the message in `ParseUpdateText` (`ai/rules/cli.md` governs error text) |
| The scenario places the token at top level | `bgp-addpath-frr/announce-addpath.py` |

The original repair obligation covered both the misleading error message and
the invalid scenario token placement. At pickup, trace the current native
process in `internal/le/interoplab/bgp/helper.go` and `ParseUpdateText` before
claiming either defect survives. Both observable obligations remain: valid
per-NLRI ADD-PATH input and truthful guidance for invalid input.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/evidence.md` - a guard or reader that fails open must say something
- [ ] `docs/architecture/testing/interop.md` and `internal/le/interoplab/bgp/check_engine.go` - native runner and fail-closed assertions
- [ ] `ai/rules/completion.md` - a red is fixed, not recorded; this spec is the home, not the resolution

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/le/interoplab/bgp/check_engine.go` - `runOperation` returns query errors; `requireJSONFields` rejects malformed or missing data
- [ ] `internal/le/interoplab/bgp/checkers.go`, `check_extras.go`, `helper.go` - current scenario assertions and compiled processes
- [ ] The scenario configs in the Current obligation set - compare with their producing parsers before changing syntax

**Behavior to preserve:** every scenario that passes today keeps passing. This
spec makes failures legible and fixes the reds; it does not relax an assertion to
reach green (`ai/rules/completion.md`).

## Data Flow (MANDATORY)

### Entry Point
`INTEROP_SCENARIO=<name> ./le integration interop`, or the nightly workflow.

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
| A-1 | The original failures were independent of the Python runner revision | Both Python versions failed alike on 2026-08-05 | That comparison cannot exclude a shared harness defect | The August 7 investigation found a shared fail-open reader and CLI setup defects | broken as a claim that harness faults were excluded |
| A-2 | The FRR and BIRD received-route failures shared one cause | August 7 producing-function account above | Separate defects would require separate repair | Recorded CLI/listener/reader repair and dated passes | confirmed in the 2026-08-07 record |
| A-3 | The current clean tree satisfies the full recorded obligation set | No current run recorded here | Repairs or evidence remain owed | Run the named population from an isolated clean revision, then the full suite | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing `rib_count` to fail closed turns other currently-green scenarios red, because they were passing on a masked zero. | More scenarios red after the helper change than before. | That is the helper working. Each newly visible red is a real fault and gets its own row here, never a revert of the fix. |
| R-2 | The reds are environmental on one machine. | They pass elsewhere. | A-3's clean-export run settles it before any fix is designed. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A route-count query fails or returns malformed/missing data | -> | native `runOperation` / `requireJSONFields` | Native helper refusal test and runner-level failure evidence, named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A route-count command fails or answers unparseably | The native runner fails and names the query/scenario; no synthetic zero is returned |
| AC-2 | Every scenario and behavioural obligation in Current obligation set | Each has passing evidence against the current revision after any required repair. A diagnosis row alone never satisfies this AC |
| AC-3 | Fail-closed checking reveals another red within this repair population | Diagnose and repair it without relaxing assertions; record its producing cause and passing evidence |
| AC-4 | The named population and full suite run from an isolated clean revision | Results distinguish historical failures from current survivors and rule out shared working-tree changes; no unresolved failure is presented as completed repair |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | AC-1 | |

### Functional Tests
<!-- Tooling scope: no daemon Go changes are known yet, so the driving surface is
     the runner. If the rib fault turns out to be daemon-side, this spec grows a
     .ci and this row is revisited. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Native runner over Current obligation set, then the full suite | `test/interop/` via `./le integration interop` | the recorded failures are repaired and all required scenario assertions pass | |

## Files to Modify
- `internal/le/interoplab/bgp/` and affected scenario configs - only current surviving defects identified at their producers; keep fail-closed helper behaviour
- `docs/architecture/testing/interop.md` - the harness contract, if helper behaviour changes

## Files to Create
- (fill during design)

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | **Yes** | `docs/architecture/testing/interop.md` |
| 1-9, 11-17 | - | Decided at design | The rib fault may prove to be daemon-side, which would reopen rows 4 and 12 |

## Implementation Steps

1. Establish the current clean-revision results for the full Current obligation set.
2. Inspect and repair each surviving producing defect, preserving every assertion.
3. Record passing evidence for every row, including OSPF route retention and invalid ADD-PATH guidance, then run the full suite and normal verification/review.

## Known Limitations

- Scope includes the original five scenarios, the August 7 additions and shared
  OSPF config/retention obligations, plus the route-count helper contract.
  A general sweep of unrelated fail-open readers remains outside this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated, not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `wire-edit-4-api-origin-deferred-bird-interop.md`, 2026-08-05

Deferred by spec-wire-edit-4-api-origin-deferred-bird-interop (adjacent defect, found at the round 4 review gate).

Five interop scenarios are red at HEAD and untracked: `bgp-routes-from-frr`, `bgp-routes-from-bird`, `bgp-route-withdrawal-frr`, `bgp-ipv6-ebgp-frr`, `bgp-addpath-frr`. Plus the harness helper that hides two of them: `Ze.rib_count` (`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) returns 0 when its verb does not resolve, so "unknown command" reports as "0 routes received"
