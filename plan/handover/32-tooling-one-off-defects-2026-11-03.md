# Handoff: tooling one-off defects

**Next action: independent `/ze-review` of the uncommitted Phase 4A/4B browser candidate.**
Do not start more implementation or repeat completed tests before that review.
No independent browser review has occurred. Its author was
`ses_e63ce5a9affeBGOhGUrFDR8Lls`, which cannot supply the independent verdict.

## Context and goal

- Spec: `plan/post-release/engineering/spec-tooling-one-off-defects.md`.
- Historical evidence: `plan/post-release/engineering/tooling-one-off-evidence.md`.
- Branch: `main`, shared checkout at `/home/thomas/Code/github.com/ze-software/ze/main`.
- Parent identity: `CLAUDE_CODE_SESSION_ID=ses_e640edc6effeAAkQ560zO43PMs`.
- State: `tmp/session/2026-11-02-121815/state/session-state-tooling-one-off-defects-ses_e640edc6effeAAkQ560zO43PMs.md`.
- Assigned scratch: `/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-01-180915/scratch`.
- Many actual logs and later artifacts are under `tmp/session/2026-11-02-121815/scratch/`. Preserve their actual locations.

The owner requested “ok do it all” and approved the final design through the
question gate. The spec remains `in-progress`, phase `4/13`. Its early design
history is retained, but the top approval supersedes old pending-approval text.
Implement and prove every owned obligation without converting missing evidence
into a pass. This handoff is not closure or release authorization.

## Status

### Landed work

| Commit | Completed slice |
| --- | --- |
| `7698b02cde` | Approved reconciled design |
| `e8c5576aa8` | Phase 1 false-green refusals |
| `7a9da881db` | Editor dimensions and option validation |
| `4c8c4918e6` | Editor runtime, waits and timers |
| `93c53a73e6` | Owned editor child, IPC, reap and cleanup |
| `f2f048b34d` | Editor resource admission and capability uncertainty |
| `9549c1b106` | Union construction and interop cleanup verdict |
| `6c18acbb85` | Runner ownership, capture and peer linger |
| `a2bd71db82` | Six persistence fixtures select disk, plus guest guidance |
| `0aeab361ba` | LG seeded-certificate identity and persistence |
| `067bf0ad2e` | Connected retained-root, storage, actual-child and portable-launch repair |

These session commits were not pushed by this work. They do not close every
acceptance criterion in their phases. The state records each commit's selected
files, independent review, evidence and limits.

Phase 3C reviewer `ses_e5f03d901ffeLKxO3sV1TmPG2S` cleared the fixture/LG work,
retained-root and actual-descriptor repairs, portable compatibility, and final
OpenBlob cooperative lock-handoff fix. Storage APIs and consumers landed together.
The scope is owned confinement, not immutable bytes or trees against hostile
same-user writers.

All six active disk cases have recorded native greens. The three promoted kernel
cases ran in Ze's 7.3 guest. Their post-promotion proof was parent-observed tool
transcript output, not a separately retained complete guest log. Preserve that
limit and the pinned runner/DUT identities. Darwin/BSD evidence remains source
and typecheck only. Native launch/root conversion/two-child proof remains owed.
Foreign-owner tests requiring ownership changes skipped on the non-root host.

### Current uncommitted work: exact review population

The handoff census confirmed these paths in the dirty shared checkout. The spec
diff is only phase `3/13` to `4/13`. No browser path is part of the handoff commit.
Recheck ownership when later selecting commits because other sessions can edit
the same files.

```text
internal/component/web/testing/deadline.go
internal/component/web/testing/expect.go
internal/component/web/testing/parser.go
internal/component/web/testing/runner.go
internal/test/cli/cmd_web.go
internal/test/cli/cmd_web_discovery.go
internal/test/runner/server_process.go
internal/component/web/testing/deadline_test.go
internal/component/web/testing/deadline_paths_test.go
internal/component/web/testing/expect_retry_test.go
internal/component/web/testing/ownership_test.go
internal/component/web/testing/pipe_test.go
internal/component/web/testing/resource_test.go
internal/component/web/testing/runner_test.go
internal/test/cli/cmd_web_admission_test.go
internal/test/cli/cmd_web_auth_test.go
internal/test/cli/cmd_web_budget_test.go
internal/test/cli/cmd_web_cleanup_test.go
internal/test/cli/cmd_web_command_integration_linux_test.go
internal/test/cli/cmd_web_deadline_test.go
internal/test/cli/cmd_web_owned_test.go
internal/test/cli/cmd_web_resource_integration_linux_test.go
internal/test/runner/server_process_test.go
docs/architecture/testing/runner-architecture.md
plan/post-release/engineering/spec-tooling-one-off-defects.md
```

The seven production files implement one case deadline before startup, scheduler
headroom once, parsed admission snapshots, resource scheduling and owned cleanup.
Tests cover command dispatch, capability states, cross-process leases, retained
pipes, startup failures and cleanup verdicts. Treat these as the author's claims
until the independent reader checks the producers and evidence.

Shared status also contained appliance, BGP, policy, IKE, plugin, fixture, tooling,
RFC and other plan changes, plus `.claude/settings.local.json`, `ai/INDEX.md`,
`more-todo` and `plan/roadmap.md`. These are not this candidate's ownership list.
Do not stage, repair, delete or attribute them from status alone.

### Existing exact-change judge approvals

Read `tmp/commit-weakening-approved-4f95a2b5.md`. Browser rows are already recorded
and applied. Do not recreate approvals or broaden their meaning.

- `ses_e5de6e5d3ffe2A8UGIymoALeyt`: old canonical
  `testing.TestWBTimeoutTakesTheVerifyHeadroom`, renamed to
  `TestWBTimeoutTakesTheSchedulerHeadroom`, and the five `testing.TestRetry*` rows.
- `ses_e5dccf917ffeg7K7Nu1o9yTDBM`: `testing.TestBrowserCloseResetsDaemonStart`
  and old `testing.TestBrowserNoSessionClosesAll`, renamed to
  `TestBrowserStandaloneClosesOwnSession`.
- `ses_e5dbd270fffeiWUC1bglgYj1dE`: `cli.TestZeTestWebAuth`, exactly the retained
  `phase4b-auth-judge.patch` under the 2026-11-02 scratch directory.

Prior fixture approvals were consumed or retained by their scoped commits.
The owner permitted the earlier two flowspec draft corrections and the audit
draft update. Keep `test/draft/plugin/audit-config-commit.ci` ignored and draft.
No force-add or promotion follows from its approval.

## Independent review brief

Use a context independent of the browser author, with at least two review lenses.
Read the system/subsystem doors and runner architecture before the code. Carry the
spec, parent identity, both scratch locations, exact paths and evidence limits in
the brief. Do not commission another broad design census.

1. Check deadline inheritance from discovery and scheduler through startup,
   readiness, viewport, locale, open, actions and retries. Verify factor 3 only
   for concurrent cases, factor 1 for serial, with no verify-mode policy or reset.
2. Check ParseFailed precedence, immutable snapshots and capability
   absent/present/unknown/malformed cases before effects. Check same-group leases
   through success/failure/cancellation cleanup and different-group progress.
3. Check sole Wait ownership, primary/auxiliary joins before directory/port/group
   release, and cleanup errors preserving browser steps and earlier failures.
   Explicitly judge `internal/test/runner/process_owner.go`, `ownedProcess.retire`:
   it discards the final `groupSignal` error. The handoff read confirmed this.
   Do not silently convert that limit into universal cleanup success.
4. Compare reachable new outcomes with measured statement blocks. Distinguish real
   children/syscalls, injected signal verdicts and the storage-stage Context
   observer. Decide whether any unexecuted migrated server-kind path needs proof.
5. Compare auth/close/headroom/retry migrations with the exact approvals. Judge
   any new weakening independently before an edit, not after a gate refusal.

Synchronous bcrypt and storage operations are not hard-interruptible. The current
proof shows cancellation refusal afterward, not a hard elapsed startup bound.
Separate cleanup and pipe bounds can extend beyond the authored execution budget.
No close-all, foreign-process sweep, subreaper, Rust pipeline, quarantine or
universal detached-Chrome death guarantee is part of this repair.

## Evidence to read, not rerun

Start with state lines 2540–2627, headed “Phase 4A/4B checkpoint”. Lines 2402–2450
hold the real-browser trace. Earlier sections explain superseded failures. Avoid
reading all 2627 lines unless a specific evidence question requires it.

Job log basenames below are in `tmp/session/2026-11-02-121815/scratch/`.
Resolve profile locations from the recorded commands rather than moving files.

- Final targeted race: `job-phase4ab-final-targeted-race-52c56649.log` and
  `job-phase4ab-last-boundaries-race-952cb799.log`.
- Restored discrimination: `job-phase4a-inherited-routes-cut-ccf98848.log`,
  `job-phase4b-capability-cut-0dc5da50.log`,
  `job-phase4b-native-lease-cut-b226cc78.log`,
  `job-phase4b-native-cancel-lease-cut-72d55686.log`,
  `job-phase4b-pipe-red-ea76899e.log`, and
  `job-phase4b-callback-corrected-cut-990831b8.log`.
- Native command entry: `job-phase4b-command-native-2506737f.log`,
  `job-phase4b-command-counter-proof-ef4a6ed8.log`, and
  `job-phase4b-auth-expiration-count-f67fc4b6.log`.
- Authoritative aggregate: `phase4ab-outcome-counts.cover`. It includes the unique
  workers plus CLI, browser, boundary, native-command/auth, startup and seed-stage
  profiles listed at state lines 2594–2605.
- Unique workers: `phase4b-worker-counts/`, from
  `job-phase4b-worker-counts-unique-e95bc961.log`. Use generated
  `id-mode-suffix.cover` files matching `*-*-[0-9]*.cover`, not old `001-*.cover`
  basenames reused across subtests. The log names all 19 authoritative files.
- Effective instrumented executable: `scratch/le-phase4b-covered-main/le`, with
  `cmd/ze` included for exit counter hooks. The earlier two covered builds emitted
  no counters. Keep them as diagnostic evidence, not count proof.
  `phase4b-command-counters/` holds the per-exit artifacts.

All 70 statement blocks in the three new production files have recorded hits.
That is not all-branch or whole-package coverage. Existing server kinds and
impossible parsed-state defaults are not certified by that number.

Real agent-browser 0.26.0 navigation, nonsettling refusal and owned close passed in
`job-phase4a-native-traced-4584a6c9.log`,
`job-phase4a-native-direct-4584a6c9.log` and
`job-phase4b-native-owned-close-4584a6c9.log`. Tracing was removed from final code.
The initial unlogged-session open/close timeout remains unexplained. Its cleanup
is not retrospectively proven. Do not kill processes to compensate for that gap.

The author reported all source cuts restored and no active mutation markers at
the final inspection. This handoff performed no new mutation or runtime proof.
Keep existing logs, profiles, native builds and identified private browser trace
directories until review/landing no longer needs them. Preserve foreign artifacts.

## Known reds and remaining evidence

Browser candidate files have no reported remaining lint finding. The overall
scoped action was red. `job-phase4ab-owned-lint-fixed-a703f5e7.log` names older
runner identity/lifecycle/exec/caps files, `cmd_editor`/`cmd_bgp`, and non-Linux
process/loopback/rlimit issues. Some are this spec's earlier owned work, not foreign
work. Triage them as remaining spec defects/evidence, without waiving them.
`job-phase4ab-added-tests-lint-8256ad63.log` still names `cmd_editor.go:164` and
`cmd_bgp.go:568,574` outside the browser candidate.

The owned architecture page was checked directly against producers and tests.
The wiring dry-run selected global documentation checks, which were not run.
No whole-documentation green is claimed. Earlier Phase 3C LG embedlit and reactor
build failures blocked its remaining gates. Preserve those historical results,
the clear-debug catalog mismatch and four stale source anchors without assuming
their current cause or relabeling them green.

External blockers remain in the spec and evidence companion:

- AC-27's `ze-zeroed-segment.tar.gz`, `ze-zeroed-schema.tar.gz` and boundary traces
  from session `644a4e78-0122-4eeb-8642-eb50638a1641` remain unrecovered.
  A rebuild or generic integrity check cannot attribute or repair the incident.
- Original post-OSPF `ospf-closure-lint.log`: 93 host/104 Linux findings and related
  capability, uncovered-file, wiring/audit/doc populations remain incompletely known.
- June technique/fuzz identities, predicates and inventories remain missing.
- Authentic October naming log/final-proof archive remains missing. The bounded
  nine-finding reconstruction is not that archive.
- Four-peer GoBGP configs, OPENs and captures remain required for the wider reload
  claim. Remaining October raw contexts/load members and original performance
  timing files remain missing where the spec/companion names them.

No current green closes an unidentified historical member. Keep each obligation
with its exact AC/P-number and provenance.

## Decisions that must survive the next session

Owner-ruling interpretation is unchanged. The owner first said “no changes it
works as it is”, then required authorization from an interactive terminal.
The standing rule landed at `8e9e99805c`. Enforcement remains unimplemented work
for Phase 8, not a completed consequence of writing the rule.

The approved four issuance gates are existing RFC owner-ruling approval,
`commit create attest`, spec review with owner-authorised supplied at any round,
and push preparation. Gate before writes. Ordinary independent judges, ordinary
reviews, recorded consumers and prepared-script execution remain agent-usable.
TTY is an operational boundary, not identity proof. Add no `rfc authorise`, owner
ledger, provenance redesign or invented authorization.

The primary spec retains unfinished work rather than transferring it here.
Its broader detached-browser no-survivor guarantee belongs to
`spec-test-sandbox.md` AC-5. Root-cause classes T1–T6 remain with their existing
specs as stated in the primary Task. The evidence companion separately retains
the RS red probe under `plan/release/fixes/spec-bgp-one-off-defects.md`.
This handoff creates no scope reduction or new Work Not Done transfer.

## Next five actions

1. Load current rules/doors and commission the independent Phase 4A/4B review
   above. Read current source and retained evidence. Do not repeat passing tests
   merely to start a fresh session.
2. Delegate concrete findings to implementation contexts. Obtain an independent
   exact-change judge before any weakening. Reproduce and fix the producer, then
   run only new/failing/reached scoped tests. Review fixes in an independent context.
3. After a clean scoped verdict, inspect shared hunks and commit only the explicit
   browser candidate through `/ze-commit`. Check `Spec-work` and approval trailers,
   execute the exact returned script, and append SHA/evidence limits to existing state.
   Stop on an unapproved-unit gate. Never invent authority or omit its file.
4. Continue Phase 4C: shared authored site login/edit/save/diff/confirm/readback
   journey and capture settling. It has not started. Then follow phases 5–12 below.
5. Close only after the full criteria/evidence map and independent Review Gate
   are satisfied. Phase 13 remains blocked on named missing originals or attributable
   recurrence. Do not mark the whole spec done from slice commits.

| Phase | Remaining work, from the spec's Implementation phases table |
| --- | --- |
| 5 | Approval binding/publication, prospective committed review, canonical units, Git paths, removal/message/debt and identity |
| 6 | Owned reclamation, cancellation, guest/cache/KVM/bootstrap/cwd, installer second interrupt and all six platform routes |
| 7 | Hook consumers, direct-edit WIP, positive criteria, session attribution, wire-speaker census and canonical guidance |
| 8 | RFC levels/dispositions, explicit-kernel recording, committed RFC7084 text and the four terminal authorization gates |
| 9 | Fifteen live checker proofs, five RTR cases, IPsec/staging, real generated-peer OPEN matrix and bounded GoBGP reload |
| 10 | Complete committed inventory and derived CodeQL extraction/dependency population, with CI extraction evidence |
| 11 | Verified publication bytes, fixture/generator/workflow repairs, deterministic chaos/poster/theme, DNS judgment and truthful AI-first docs |
| 12 | Launcher/watchdog/stress identity, vendor/schema/l2tp inputs, empty derivations, report validity and bounded capture names |
| 13 | Historical June/lint/load populations and executable corruption attribution |

## Files already handled and execution rules

The spec's approved contracts, exact candidate list, judge ledger and state
checkpoint have been located. Use those locations instead of repeating a full
repository census. Independent review must still read the producers it judges.
There is no speculative code patch or immediate test command in this handoff:
the next artifact is the independent verdict, which determines any next edits.

- Fresh sessions load `AGENTS.md`, `ai/META-DECISIONS.md`, CORE/TRIGGERS and matching
  rules. Read `ai/doors/system.md` and applicable subsystem doors before briefs.
  Read `docs/contributing/ze-go-style.md` before the first Go edit.
- A fresh top-level session fetches and checks `git rev-list --count HEAD..@{u}`
  before editing. This handoff pass fetched and observed zero behind on `main`.
- Keep the parent identity and existing state/scratch. Use one CPU-intensive job
  at a time through job admission. Exact commands, tags and timeouts are in the
  named logs/state. Completed checks are not rerun without changed scope or failure.
- Develop/test only in main. No worktrees, exported source trees or copied-tree
  tests. Never run any `./le verify` command or substitute a whole-tree gate.
- No sudo is needed here. The standing stress-only exception is not general
  permission. Stop, signal or remove only resources proven to belong to this work.
- All Git staging/commits/removals/pushes use the supported `./le commit` route.
  Inspect the generated message and execute exactly its returned `script=` path.
  Preserve foreign work. No push without an owner order.

This handoff pass created and committed only this file. It ran no tests, builds,
review, agents or push, and changed no spec status or claim.
