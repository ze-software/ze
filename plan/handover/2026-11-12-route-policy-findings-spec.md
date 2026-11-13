# Handover: route policy's three closure findings now have their own spec

Implementation continuation: [2026-11-13 route-policy handover](2026-11-13-route-policy-continuation.md).
It records the committed implementation, two unfinished teardown repairs,
interrupted daemon proof and the exact draft/proof archives for transfer.

Written 2026-11-12 by the session that continued
`plan/handover/2026-11-10-close-open-specs.md`. It replaces that handover's
route-policy section. Everything else in that handover (lint-tooling, the
unjournaled findings, the rule conflict) still stands there.

## Design revision on 2026-11-12

The owner rejected the internal-only proposal below and required correct
internal and external system RIB operation. The requirement is recorded in
`docs/architecture/rib/unified-locrib.md`, "Owner requirement: internal and
external system RIB".

`plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md` now
requires complete two-mode routing behavior, an authoritative daemon view,
distance publication, candidates, external FIB output, and lifecycle proof.
The owner approved implementation on 2026-11-12. It is now `in-progress`.
The proposed system RIB, VRRP, and AS112 commit refusals are withdrawn.

The first implementation claim was refused: 17 specs were already in progress
against a cap of 12. The owner then answered, "Yes, you can increase the number
of specs." This spec was admitted with cap 18; that exception does not change
the standing cap permanently.

Implementation is active in the main checkout. The current per-spec state is
`tmp/session/2026-11-12-205428/state/session-state-route-policy-1-gate-proof-and-forked-sysrib-01a1d178-88dd-7463-b4bb-ae4c147dd158.md`.
The generic startup/registration and public contract packages are written and
under review; the selected-view and projection packages are next. Only the
related selectable-cover LPM repair has landed (`f52a5c7dfd`), with both store
backends tested. This does not claim external system RIB parity or completion
of any later phase. Nothing has been pushed.

The umbrella closure instructions below remain a handoff for that separate
work, subject to checking the current tree. The old design decision table,
the new spec's old assumptions and criterion mapping, and Phase 3 below are
historical. Do not implement them. Read the revised spec for the current
design and acceptance criteria. The umbrella's pointer edits have not been
made by this revision.

## Original decisions waiting on the owner (superseded)

| # | Decision | What it unblocks | Recommendation |
|---|----------|------------------|----------------|
| 1 | Approve or change the design of `plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md` (its Key Design Decisions table) | The spec moves from `design` to `ready`, then `/ze-implement` | Approve. The central choice: the system RIB runs only in the daemon, and the commit refuses a config that forks it |
| 2 | The bucket and priority: `plan/release/fixes/`, P1 | Where it sits in the release inventory | Keep. A forked system RIB programs routes the import filter rejected, in code the release ships |

The design departs from one sentence of `ai/doors/system.md` ("Code that
only works internal is broken"), through the exception mechanism that
already exists (`Registration.RunsInProcessOnly`). That is why it is put to
the owner and not moved to `ready` by the session.

## Handoff

**Spec:** two, in this order: `spec-route-policy-0-umbrella` (close it), then `spec-route-policy-1-gate-proof-and-forked-sysrib` (implement it once approved)
**Branch:** `main`, ahead of `origin/main`, nothing pushed. Thomas controls pushes.
**Goal:** close the umbrella now that its three findings have another owner, and take the new spec through approval and implementation.

### Status

- Done:
  - The new spec is written at Status `design` and passes the spec validator. It owns the umbrella's B-1 (missing daemon tests), B-2 (forked system RIB) and B-3 (feature declaration, comparison row).
  - Research for it, verified at the producers: a forked system RIB sees BGP only, ignores every import chain and `rib { distance }`, resolves no next hop, carries no ECMP siblings and refuses `show rib candidates`. It is reached only by an operator writing `plugin { external <name> { run "ze plugin rib"; } }`; no test, page or autoload path uses it.
  - The two missing tests are half written: `internal/test/fixture/rib_import_policy_fixture.go` and `register_rib_import_policy.go` are committed (drivers `plugin/rib-import-central-bogons`, `plugin/static-import-filter-distance`). Only the `.ci` files never left the git-ignored `test/draft/`.
  - The umbrella's A-4 benchmark is recorded and accepted (`plan/records/route-policy-0-a4-mac.bench.txt`).
- In progress: nothing. No claim is held, no job or agent is running.
- Remaining:
  - The five pointer edits to the umbrella below. They were not applied: the umbrella's write gate asks for 29 producer files and two doors to be read in the editing context, and the session that closes the umbrella reads the same set for its closure edits. Until they land, the umbrella still says it owns B-1 to B-3.
  - The umbrella's closure: review round 5, the review record, the news fragment, the citers, commits A and B.
  - The new spec: owner approval, then implementation in six phases.
- Not done, and the spec that owns each item:
  - Daemon tests for the central, static and connected import chains; the OSPF leg of the central chain on `ospf-import-policy-bird`: the new spec, AC-6 to AC-13.
  - The forked system RIB's ungated input, and the commit refusal for every in-process-only plugin (system RIB, VRRP, AS112): the new spec, AC-1 to AC-5.
  - `features/route-policy.md` and the `docs/comparison.md` row: the new spec, AC-14 and AC-15.

### Files Already Handled (don't re-read for the design; the gates still want them read before a spec edit)

- `internal/component/sysrib/sysrib.go` `run`, `processEvent`, `effectivePriority`, `reapplyAdminDistances`: two inputs; the unmarked batch branch and the arbitration serve only the forked input.
- `internal/component/sysrib/register.go`: registration sets no `RunsInProcessOnly`; `runSysRIBPlugin` wires `locrib.Default()`, nil when forked.
- `internal/component/sysrib/candidates.go`: `errNoLocRIB` already tells the operator the system RIB must run in the daemon.
- `internal/core/bgp/ribevents/ribevents.go`: `FromLocRIB` is `json:"-"`; removing it changes no payload.
- `internal/component/plugin/registry/registry.go`, `internal/component/plugin/doctor/check_external.go`: the declaration and the doctor check that reads it. Only the test plugin sets the field today.
- `internal/component/config/validators.go`, `internal/component/plugin/yang/ze-plugin-conf.yang`: commit validators sit on leaves; `external-plugin-name` is on `name`, and `run` has none. The new refusal attaches to `run` and must accept the `ze.<plugin>` form.
- `internal/plugins/vrrp/register.go`, `internal/plugins/as112/register.go`: both refuse a forked run at run time and declare nothing.
- `internal/test/fixture/rib_import_policy_fixture.go`: injects through BGP and static, requires the control selected before judging; decodes no `distance` or `tag`, reloads nothing.
- `internal/le/interoplab/bgp/check_import_policy.go`: the BIRD scenario proves a distance set by OSPF's own chain; it has no central chain.
- `test/plugin/doctor-plugin-external-unsupported.ci`, `test/parse/plugin-external-bad-name.ci`: the two patterns the new refusal follows.

### Edits

All five are in `plan/post-release/features/spec-route-policy-0-umbrella.md`.
Before the first one the gate requires, read in the editing context:
`ai/doors/api-surfaces.md`, `ai/doors/hub-engine.md` (and every other door,
if the context is fresh), plus the producers it names. On 2026-11-12 it
named 29: the Loc-RIB (`manager.go`, `candidate.go`, `entry.go`), the BGP
config and reactor filter files, the redistribute files, the IS-IS and OSPF
installers and consumers, `static/locrib.go`, `connected/locrib.go`, six
YANG modules, `pkg/plugin/rpc/types.go`, `dispatch_route.go`,
`redistevents/events.go`, `bgp/redistribute/producer.go` and
`internal/core/diagnostic/codes.go`. The refusal prints the exact list.

EDIT 1: line 5
- Replace: `| Status | blocked |` -> `| Status | verification |`

EDIT 2: lines 19-23
- Replace the paragraph that starts "The A-4 blocker of 2026-11-10 is cleared" and ends "back to `/ze-implement`:" with:

  Nothing blocks this spec now. The A-4 blocker of 2026-11-10 is cleared: the
  owner's matched Mac run is `plan/records/route-policy-0-a4-mac.bench.txt`,
  and he accepted its cost (Assumptions, A-4). The 2026-11-12 closure review
  (round 4, Review Gate below) found three ISSUEs, each needing
  implementation. The owner moved them out on 2026-11-12: "route-policy's
  three review findings get in a new spec". They are owned by
  `plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md`,
  which also carries what its research found beyond B-2 (a forked system RIB
  is wrong in five more ways) and the gap between the committed fixture
  drivers and AC-6 and AC-7 as written. The table below is kept as the record
  of what was handed over:

EDIT 3: lines 31-33
- Replace the paragraph that starts "The closure work that needs no implementation is done" and ends "It is the last round a session may spend." with:

  The closure work that needs no implementation is done (Implementation
  Audit, Pre-Commit Verification). What remains is closure alone: round 5,
  which checks that every item of B-1 to B-3 has an owner in the new spec and
  that this spec claims nothing they would have proved, then the review
  record, the news fragment and the two closing commits. Round 5 is the last
  round a session may spend.

EDIT 4: line 784, the Goal Validation row for "never install a bogon from any protocol"
- Replace the row's last two cells, `functional | Missing: `rib-import-central-bogons.ci` (B-1)`, with: `unit, with the daemon proof owed elsewhere | Here: `TestLocRIBCentralChainAllSources` and `TestRouteInstallTagReachesGate` prove the gate judges every source. The daemon proof is not claimed by this spec: `spec-route-policy-1-gate-proof-and-forked-sysrib` AC-6, AC-7 and AC-11 own it`

EDIT 5: line 792, the Work Not Done row
- Replace `| B-1, B-2, B-3 | Need implementation | This spec, handed back to `/ze-implement` |` with: `| B-1, B-2, B-3: the daemon tests for the central, static and connected import chains (AC-6, AC-7, AC-11 at the daemon, story 2); the forked system RIB's ungated input (A-2); the feature declaration and the comparison row (Documentation rows 1 and 11) | Need implementation; moved by the owner on 2026-11-12 | `plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md` |`

Phase 2 after these edits, the umbrella's closure:

1. Claim it above the cap for closure only: `ZE_SPEC_WIP_CAP=<in-progress count + 1> ./le spec claim plan/post-release/features/spec-route-policy-0-umbrella.md` (`./le spec wip` prints the count).
2. Review round 5, by an independent agent, over the five edits and the claims they touch. Three rounds are recorded and round 4 is done; round 5 is the last a session may spend. A sixth needs `owner-authorised "<words>"` typed by Thomas at a terminal.
3. Record the review. The population is 72 `Spec-work:` commits and 643 files, listed in `tmp/session/2026-11-12-014013/scratch/rp0-review-files.txt`. Re-derive it if a commit landed under that trailer since.
4. The news fragment `website/changes/fragments/spec-route-policy-0-umbrella.md`.
5. Repoint the live citers of the umbrella: the four `README.md` files under `test/interop/scenarios/` (`isis-import-policy-frr`, `isis-export-policy-frr`, `ospf-import-policy-bird`, `ospf-export-policy-frr`) and `plan/release/fixes/spec-single-owner-reload-reconcile.md`. The new spec's `Depends` row names the umbrella's path and must then name its closing commit.
6. Commit A (spec and citers), commit B (`remove` the spec), one `./le commit create` script. Release the claim.

Phase 3, the new spec, only after the owner approves its design: set Status
`ready`, claim it, and run `/ze-implement`. Its six phases are in its
Implementation Steps. Two things to carry in:

- Its assumptions A-1 to A-7 are all `unvalidated`; A-1 (nothing shipped runs the system RIB forked) and A-4 (doctor still reports a config the commit refuses) decide whether phases 1 and 3 hold as written.
- About 62 `processEvent` unit tests build a batch the deleted input produced. None may be deleted or weakened without a `ze-test-judge` verdict.

### Then

After the five edits the spec validator runs on each save; a refusal names
what is missing. Then:

```
./le spec status
./le doc check all
```

Both are quick. No test run is owed by the umbrella's edits: they are prose.
Nothing in this handover authorises a push.

## Rules that bit this session, so the next one does not pay twice

- A spec edit is refused until the editing context has read, with the Read tool, the producing range of every source file the spec cites, and every door covering a path it names. A research agent's report does not count. Batch spec edits into the context that has already read them.
- A criterion whose Expected Behavior states only an absence is refused from `design` onward. Lead with what is observed.
- A package-relative file name in a spec (`vrrp/register.go`) is refused when it matches two tracked files. Write the full path.
- Commits go only through `./le commit create` and the printed `script=` path. No agent runs `./le verify`, a whole-tree lint or a repo-wide compile. One CPU-intensive job at a time, in the main thread.
