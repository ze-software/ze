# Spec: fixit-functional-suite-pins-the-unshipped-backend

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Reconcile the remaining functional storage evidence with the current live-store
contract before commissioning further implementation. The original task was
returned to design on 2026-08-24 and its ACs were explicitly voided on
2026-08-28. Neither the original all-daemons-pinned claim nor the later
client-pin diagnosis describes the current runner.

`Runner.runTest` now supplies `ze.config.dir=rec.WorkDir`, and
`runOrchestrated` supplies an isolated `configDir` in
`internal/test/runner/runner_exec.go`. Neither launch environment contains the
old `ze.storage.blob=false` pin. `zeConfigFileName`
(`internal/test/runner/runner_config.go`) gives distinct daemon stdin blocks
distinct directories and reuses the assignment on restart.

The storage product changed too. `docs/architecture/storage-backends.md`
defines a live framed `database` tree. `detect` and `openLive`
(`internal/component/config/storage/open.go`) open that tree and refuse a
legacy `database.zefs` with an explicit import diagnostic. Blob artifacts are
seeds, backups and import inputs; restoring a blob-default switch or adding a
`storage` directive would implement a retired design.

The retained concern is evidence that functional tests exercise the storage
behaviour the product exposes, with isolated state for distinct daemons and
stable state for a restarted daemon. The inherited web pending-diff coverage
item remains to be assessed against current coverage. Existing
`test/web/edit-diff-modal.wb` and `workbench-bgp-pending-diff.wb` are candidate
carriers to inspect; their existence alone does not prove that they cover the
original structural-operation failure.

The August disposition classed this as elective coverage. No current product
defect or release blocker is established by this document. Before any new
runner or product change, decide whether existing implementation and evidence
satisfy the retained intent, or whether a specific coverage gap survives. This
planning correction neither closes the spec nor authorises a new storage mode.

## Historical corrections and measurements

### 2026-08-22 premise, superseded on 2026-08-24

The original task claimed every functional daemon used filesystem storage,
that the pin's reason was undocumented, and that neither `.ci` nor `.wb` could
see a blob-only defect. The re-cut established that the daemon pin was guarded
by `zeDaemonShouldForceFileStorage`, excluded web daemons and documented its
purpose: keeping tests out of the developer's shared active pointer. The web
runner used a separate environment producer and already ran blob storage.
The client pin was unconditional at that time.

`option=env:var=ze.storage.blob:value=...` was already an override; 17 fixtures
used it in the August survey. The `.et` runner defaulted to filesystem and
opted into blob, contrary to the original claimed precedent. Those are dated
facts about the former two-backend contract, not instructions for the current
live store.

### 2026-08-24 isolation experiment

At that time `resolve.Storage()` selected
`<DefaultConfigDir()>/database.zefs`, and the runner did not give each daemon
its own config directory. The shared active pointer explained why removing
both pins without isolation was unsafe.

| August tree | PASS | TIMEOUT | FAIL |
|-------------|------|---------|------|
| Then-HEAD | 42 | 0 | 1 (`mgmt-guard-reload-auth-rebuild`, attributed elsewhere) |
| Both pins removed | 11 | 19 | 8 |
| Pins removed, plus a per-test `ze.config.dir` | 25 | 15 | 3 |

The August record reported 31 formerly green reload tests regressing across
24 files. Six sampled cases passed alone with a fresh blob directory and
wrote blobs of about 2 KB. Seventeen failures remained unexplained with a
per-test directory; that experiment did not establish a finished isolation
design. The parse suite's 312/312 result was weak storage evidence because its
blob was a 37-byte header, while the reload suite's reached 12.6 KB.

These counts are preserved as history. They are not a measurement of the
current runner or of the framed-tree storage cutover.

### 2026-08-28 disposition

The old ACs were voided. In particular, old AC-3 asked every filesystem
selection to carry a justification even though the diagnosed need was
isolation. Adding backend declarations to silence unexplained failures would
have hidden the cause. The web structural-diff proof did not depend on a
runner backend change. The work was placed in the elective tooling backlog.

## Required Reading

- [ ] `docs/architecture/storage-backends.md`: live tree, owner lock and explicit
      blob import.
- [ ] `docs/architecture/zefs-format.md`: artifact format, distinct from the live store.
- [ ] `docs/architecture/testing/runner-architecture.md`: per-test execution and
      the separate web runner.
- [ ] `docs/architecture/testing/ci-format.md`: stable per-daemon directories and
      restart semantics.
- [ ] `docs/functional-tests.md`: what each suite exercises.

## Current Behavior (MANDATORY)

| Producer | Current fact | Evidence still owed |
|----------|--------------|---------------------|
| `internal/test/runner/runner_exec.go`, `runTest` | Client environment selects the test work directory and has no old blob pin | Identify current consumer-visible storage assertions |
| `internal/test/runner/runner_exec.go`, `runOrchestrated` | Child environment receives `configDir` | Confirm the existing tests cover distinct daemons and restart reuse |
| `internal/test/runner/runner_config.go`, `zeConfigFileName` | Distinct stdin blocks receive numbered directories; repeated blocks reuse their path | Map the existing `runner_config_test.go` cases to the intended isolation contract |
| `internal/test/cli/cmd_web.go` | Web daemon environment selects its temporary config directory | Determine whether existing `.wb` assertions cover structural pending changes |
| `internal/component/config/storage/open.go`, `detect` / `openLive` | Live storage is the framed tree; a legacy blob is refused | Use current storage behaviour in any retained regression case |

Preserve current storage ownership and explicit import, daemon isolation,
restart persistence, and backend-independent assertions. Do not restore
`ze.storage.blob` to make the old plan executable.

## Data Flow (MANDATORY)

### Entry Point

Existing `.ci` and `.wb` fixtures through their respective runners.

### Transformation Path

1. A fixture selects a daemon configuration and its isolated working directory.
2. The runner composes the environment and starts the selected binary.
3. The daemon opens the live store according to current storage ownership rules.
4. Functional assertions observe config operations, restart persistence or the
   pending-diff surface through the product entry point.

### Boundaries Crossed

| Boundary | Evidence to establish |
|----------|-----------------------|
| Runner -> daemon directory | Independent daemons do not share mutable state |
| Restart -> same daemon directory | Restart preserves the intended state |
| Pending changes -> web diff | The displayed content describes the structural operation, not only a nonzero count |

### Integration Points

The existing runners and storage API own these boundaries. No new backend
selector, environment registration or `.ci` directive is planned.

### Architectural Verification

The current directory allocation is existing implementation. Assess its
consumer-visible evidence before designing another isolation mechanism.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | Existing isolation implementation covers the former shared-store failures | Per-test and per-daemon directory producers now exist | Identify the remaining failing transition rather than re-add a backend pin | Map existing tests, then run current concurrent and restart cases | unvalidated |
| A-2 | Current web diff coverage may already exercise the inherited structural-operation case | Existing diff fixtures were found | A specific coverage item remains | Read their stimulus and assertions against the current diff producer | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Old measured failures are reported as current product defects | The report cites only August blob runs | Require a current fixture and producing path |
| R-2 | A test failure is hidden by an obsolete storage override | A proposed change restores the retired selector | Keep the live-store contract and investigate the actual failure |

## Blast Radius

The retained subject is functional evidence. Product behaviour changes require
a current reproduced defect; neither historical pin removal nor restoration of
a selectable live blob backend is an authorised implementation step.

## Wiring Test (MANDATORY)

Map existing isolation/restart tests and the current web diff fixtures before
selecting new carriers. A regression proof must reach the current product
surface and fail for the relevant behavioural defect.

## Acceptance Criteria

The old AC-1 through AC-5 remain void under the 2026-08-28 decision. Their
replacement implementation contract requires a decision at the design gate.
The reconciliation must answer each retained concern without inventing a new
feature or deleting evidence obligations:

| Retained concern | Required disposition before implementation |
|------------------|-------------------------------------------|
| The suite exercises the shipped storage contract | Map the current live-tree paths and existing assertions; name any actual gap |
| Tests do not share a live store accidentally | Map distinct-daemon isolation and restart reuse to evidence |
| Structural pending changes have functional diff proof | Identify an existing discriminating carrier or design the missing case against the current producer |
| Harness failures are not hidden as backend exceptions | Attribute each current failure before changing a fixture |

## End-to-End User Stories

A maintainer can tell which current storage behaviour a functional pass proves.
A user reviewing structural config changes gets the corresponding pending-diff
content. Neither story calls for a second live storage backend.

## TDD Test Plan

Read existing carriers first. If a retained gap survives, design a consumer-
visible regression for that gap and demonstrate its failure under a relevant
current producer break. Reverting the removed August blob implementation is
not a usable current mutation plan.

## Files to Modify

None selected before the remaining-evidence decision. Candidate inspection
surfaces are the existing runner environment/config allocation, its tests,
and current web diff fixtures. Documentation changes follow only a changed
contract or a demonstrated inaccurate coverage claim.

## Files to Create

None commissioned. The former `storage-backend-is-the-shipped-default.ci` and
`config-diff-structural-op.wb` were proposed names, not delivered files.

## Implementation Steps

1. Map current implementation and evidence to the retained concerns above.
2. Present the remaining design decision: close through the normal review path
   if existing evidence satisfies the intent, or retain a specifically named
   coverage gap and obtain approval of its revised ACs.
3. Only for approved remaining coverage, add the missing discriminating proof
   and fix any reproduced product defect that blocks it.
4. Complete the applicable independent review and worktree verification gates.

## Key Design Decisions

The August return-to-design pause remains in force. The choice is between
reviewing already-satisfied intent and approving a concrete surviving coverage
item. Reviving the removed live-blob selector is outside this task.

## Known Limitations

No current suite or reproduction was run during this reconciliation. Source
reading establishes changed ownership and removed pins; it does not establish
that all retained functional evidence is complete.

## Checklist

### Goal Gates (MUST pass)
- [ ] Retained concerns mapped to current evidence.
- [ ] Revised implementation ACs approved if a coverage gap survives.
- [ ] Every retained AC demonstrated without weakening existing assertions.
- [ ] Every assumption resolved.
- [ ] `./le verify worktree` passes before closure.

### Closure
- [ ] Complete `plan/TEMPLATE-CLOSURE.md` and independent review.
- [ ] Preserve the final evidence and decisions in commit A; remove this spec only in commit B.
