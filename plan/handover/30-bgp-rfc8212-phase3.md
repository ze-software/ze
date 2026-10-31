# RFC 8212 implementation: Phase 3 handover

## Handoff

**Spec:** `spec-bgp-rfc8212-ebgp-default-reject` (closed)
**Branch:** `main`
**Goal:** Finish the approved EBGP default-reject implementation, including reversible live policy removal without resetting the session or requiring an optional route store.
**Pause:** Owner requested stop and handover on 2026-10-28. Development stopped. All implementation agents parked; no half-applied source edit was reported.
**Observed HEAD before this documentation checkpoint:** `93c8f3d9c79e3a42126811071cfd43428a834f61`.

This checkpoint commits documentation only. Implementation files remain in the shared checkout, including changes other sessions have already committed. Do not restore, clean, delete or stage the whole tree. This handover is not a product-completion, whole-diff review or conformance claim.

### Resume first

1. Read the spec's **Implementation checkpoint**, then this handover. Keep status `in-progress`, Phase `3/7`. Update the specification at every phase/package handoff, as the owner requested.
2. Read the final entries of the canonical state below. They supersede older pending-test statements. Do not repeat passing checks merely to confirm them or rerun an observed red before fixing its producer.
3. Establish the new session's upstream check before edits. Use the named spec explicitly: the final `./le spec current` call returned `spec-homepage-first-interaction.md`, not this spec. Do not release or alter that other selection. Recover this spec's state by its explicit name using the documented spec workflow.
4. Delegate independent implementation packages, not the whole spec. The next source repairs are the deadline, final epoch gates, and mixed-owner compensation schedule. Keep shared files under one owner. The parent runs checks after a coherent snapshot; agents do not run checks or formatters mid-flight.

Canonical state (901 lines at pause):
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/state/session-state-bgp-rfc8212-ebgp-default-reject-644a4e78-0122-4eeb-8642-eb50638a1641.md`.

The same session directory's `scratch/` holds the logs below and the saved exact fixture patch. Preserve it until implementation and closure consume its evidence. Preserve `tmp/commit-weakening-approved-0cf1e0e8.md` and `tmp/commit-rfc-approved-0cf1e0e8.md`; they contain independent judgments, including other work. No temporary server or reference job remains owned by the paused C3 package.

### Status

| Phase | State at pause |
|---|---|
| 1: wiring | Finished; native drafts reached intended missing behavior |
| 2: session-aware verdict | Implemented; focused race/native checks passed; proof renewal and fixture migration remain |
| 3: reversible lifecycle | In progress; three known source defects below; actual owner lifecycle and coordinator not implemented |
| 4: reload and operations | Not started: D-1 refresh, D-2 integration, static/dynamic boundaries, status and warnings |
| 5: affected fixtures | Not started as a phase; several exact adaptations already judged/applied; RFC records and audit renewal owed |
| 6: ExaBGP migration | Implemented; migration race/native validation and actual warning smoke passed; compatibility scripts/wire proof owed |
| 7: interop and proof | Not started; FRR scenario, recorded breaks, support ledgers and final scoped checks owed |

The spec's AC-12 through AC-15 are not proven end to end. Closure, independent Review Gate and implementation commits remain `/ze-close`; `Handoff` is not `verify`.

### Observed results, not rerun for this handover

Log names below are relative to the saved session's `scratch/` directory.

| Result | Evidence |
|---|---|
| RED: stale direct/cached/pool-fallback announcements; policy-stale/replaced/unrecorded stored replay; stale exact-owner announcement. Ordinary reconnect replay PASS | `job-rfc8212-source-epoch-before-79abcafe.log` |
| RED: both bridge and wire owners commit after the earlier caller budget; owner deadline is two seconds later | `job-rfc8212-source-deadline-before-d8817e4b.log` |
| RED: removed dependent service is not actually serving after compensation, both transports | `job-rfc8212-removed-service-before-4a4d2804.log` |
| Earlier RED: retained dependent inverse runs before removed infrastructure recovery | `job-rfc8212-removed-infrastructure-before-24532dbd.log` |
| PASS: normal lifecycle application/failure/preflight, required fields and exact receipts, eight retained-replay cases, parsed/raw/full/summary epoch projection controls | `job-rfc8212-source-transport-controls-a2e07e28.log` |
| PASS: original journal, inner dependency undo, retained-root reconstruction, required completion decisions and captured process/connection lifetime repairs | Spec checkpoint lists each exact after-run log |
| PASS: migration package and revised semantic policy unit | `job-rfc8212-phase6-migration-20335d73.log`; `job-rfc8212-migration-policy-after-1e0f3c65.log` |
| PASS: actual per-peer explanatory warning smoke | `job-rfc8212-migration-warning-smoke-dddf9ca4.log`; `rfc8212-migration-warning-smoke/evidence.json` |

Independent `ReviewStoredEpochState` and `ReviewStoredEpochReplay` source reviews returned no findings in their assigned C3 storage and replay/dispatch scopes. They excluded final writer gates and owner lifecycle. The new retained-replay test has no observed pre-fix discrimination; never describe it as red/green proof. The parent has not read the entire combined C1/C2/C3 diff, so it is not called green.

### Files already handled

Read these digests before reopening wide source. Recheck the exact producer before editing; shared HEAD moved during this session.

- `internal/component/plugin/ipc/rpc.go::SendSourcePolicy`, `pkg/plugin/sdk/sdk.go::OnSourcePolicy`, `internal/component/plugin/server/source_policy.go`: captured-owner transport, strict prepare drain, required identity and exact receipts. The sender deadline defect below remains. State lines811–831 and860–867.
- `pkg/plugin/rpc/source_policy.go`: typed lifecycle operations and strict required-field validation. Current map/raw JSON repeated decoding was flagged as avoidable allocation; a typed pointer-presence decoder must retain required zero/missing/null distinctions.
- `internal/component/bgp/reactor/{received_update.go,forward_pool.go,session_ownership.go,session_write.go,relay_recovery.go}`: original epoch plumbing exists; final eligibility gates do not. State lines868–900 list every changed producer and exact remaining gate location.
- `internal/component/bgp/plugins/rib/` storage/replay, `adj_rib_in/`, `rs/`, `rr/`, `textparse/`, typed SDK update request and server dispatch: original epoch retained through equal-attribute replacement, clones, sent ownership, replay groups, inventories and JSON/text transport. State lines833–858 contain complete file/constructor/reference maps.
- `internal/component/plugin/server/reload_compensation.go::restoreReload`, `startup_autoload.go::autoLoadForNewConfigPaths`, `startup.go::selectedPluginGraph`; `internal/component/config/transaction/{executor.go,solver.go}`: original retained journals and coarse recovery are distinct. State lines878–887 give the paused third-repair contract.
- Documentation already read/updated includes `docs/architecture/api/{process-protocol.md,text-parser.md,json-format.md}`, `docs/architecture/plugin/{plugin-system.md,rib-storage-design.md}`, `docs/plugin-development/protocol.md`, and `docs/architecture/bgp/structural-forwarding.md`. Keep each page true in its owning implementation phase.

### Edit 1: effective lifecycle deadline

In `internal/component/plugin/ipc/rpc.go::SendSourcePolicy`, line596 at pause, insert immediately before:

```go
raw, err := pc.CallRPC(ctx, "ze-plugin-callback:source-policy", input)
```

The exact addition is:

```go
deadline, _ := ctx.Deadline()
input.DeadlineMS = deadline.UnixMilli()
```

The context immediately above is always created with `context.WithDeadline`; its deadline exists and is the earlier caller/request deadline. Do not lengthen the wait, change owner cancellation or weaken `TestSourcePolicyOwnerEarlierCallerDeadline`. This edit has not been applied. Review the repair independently and run the existing failed unit afterward.

### Next work packages after Edit 1

These are unfinished implementation packages, not fabricated patch-ready edits. Preserve all acceptance criteria.

**Final policy-epoch gates (C2).** Actual red is already recorded. Add a distinct original-policy predicate beside `forwardSourceCurrent`/`initialSourceCurrent`: local source nil remains allowed; received source requires nonzero original epoch equal to its owner's live epoch. In `session_ownership.go::allow`, test announcements before ownership binding or recovery staging. For `adjOutExpected`, first validate the exact revision, then test the original `entry.owner`. Do not reject authorized removals, current explicit withdrawals or GR expiry. Make `checkInitialWrite` reject unrecorded/stale received replay. Carry epoch into recovery candidates; `adjOutHolds` must not settle stale work as held. Keep stale recovery owed, including per-section PATHS-LIMIT admission, rather than reporting a suppressed replacement as written.

Epoch is globally nonzero/monotonic across Peer incarnations and stable across ordinary reconnect. It is independent of dispatch-session generation, message revision and revocation. Capture it at receive admission, never in a forwarding worker. Stored replay deliberately uses current dispatch generation with original policy epoch. Do not add a zero-to-current fallback or turn policy expiry into a session-cut exclusion.

The exact independently approved 13-file fixture patch is saved at:
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/approved-source-policy-writer-fixture-admission.patch`.
It remains **unapplied**. It adds fixture epoch beside real admission generation, not assertions. Additional recovery candidate fixtures still need original epoch and independent judgment; do not assume the patch covers them. The four-file DirectBridge arity patch and C3 helper adaptations are already applied. Never run the benchmark fixture on Linux.

C2 also flagged raw text UPDATE's added message ID as an incidental grammar change requiring correction or justification, and the empty text UPDATE's epoch contract as unresolved. Passing projection controls do not settle those API choices. Preserve the agreed reserved parsed UPDATE token outside user metadata.

**Mixed removed/retained compensation (A).** The current producer is the second, autoload-first repair. The third repair has **not started**. Global original-inverses-first fails when removed infrastructure must exist first; global recovery-first fails when a removed service binds before retained infrastructure is restored. A lenient TFTP boot records old config despite failed binding, and `sdk.ConfigTx.Apply` then skips identical config. Asserting old config alone would miss the failure; the regression asserts actual service availability.

Use a bounded server-owned dependency schedule, not a new generic planner. Preserve retained inverse order exactly, in reverse batch/operation order. Reuse alias-aware `selectedPluginGraph`, including Dependencies, OptionalDependencies, StartAfter and external-program semantics; validate the complete expanded recovery set before execution. `autoLoadForNewConfigPaths` currently combines selection/execution and can secretly expand dependencies. Do not call it per root under an incomplete schedule. Reuse conservative addressing-before-binder constraints; reversing coarse nodes alone is wrong (`solver.go::placeSectionNodes`). Identify actions by transaction, captured lifetime and operation position, not operation name alone.

Use one rollback context and the existing single-operation acknowledgment driver in `executor.go::rollbackApplied`; calling public Rollback once per action renews the deadline. Failed journal inverses are consumed: preserve attempted/success/failed outcomes, never retry them as fresh. First resolve `runPluginRecoveryPhase` calling `runPluginPhase` without caller context (`startup.go:400` at pause), so startup itself obeys the shared deadline. The removed-infrastructure regression currently lacks declared dependency metadata; submit an exact additive correction to an independent test judge before editing it. Prove both arrangements, cycle and partial-failure behavior. Two simplistic staging repairs were attempted; a third failed repair invokes the repository's three-fix rule.

**Actual owners and coordinator.** None of the following exists yet: independent import eligibility; accepted-reference undo pins; worker/downstream settlement; whole-reload prepare/apply/abort/finalize; static/dynamic peer join boundaries. Preserve spec sections on D-2 and AC-12–15. Capture transaction, peer incarnation/session/epoch/template generation and owner lifetimes. Pause admission under work and hard completion deadlines derived from the remaining HoldTimer reserve. Strict application drain is not downstream flush proof. Abort only after old filters are acknowledged, restoring still-owned pinned references under a fresh epoch and resuming once. Acceptance releases pins and resumes default rejection. No synthetic DOWN, required optional RIB, permanent denied-route store or copied full UPDATE per route.

Actual withdrawals, GR expiry and ownership replacement invalidate restoration authority even when bytes remain pinned. Keep existing external RS/RR/GR refusals; supported external RIB is a separate required proof. Existing SDK source-policy callbacks run synchronously on the callback loop: an owner waiting for downstream recovery that calls back into external RIB can deadlock. Resolve bounded asynchronous lifecycle execution before owner integration; internal DirectBridge can hide this issue. Do not reuse OnDrain's one-shot stop state.

### Remaining phases and exclusions

After Phase3, complete Phase4 reload/status/warnings, Phase5 affected fixtures and RFC discrimination/audit renewal, Phase7 FRR interop and scoped verification. Phase6 still owes old-4.2 and old-5.0 `api-announce`/`api-check`, current `api api-announce`/`api-check`, `encoding conf-ebgp`, and the affected ExaBGP functional suite. Native migration draft promotion and its required repeated proof remain owed. Use Ze as the migrated DUT, not `--dut exabgp`.

No requirement was waived for this handover. Arbitrary filter-definition re-evaluation and a new desired-state route store remain outside this spec. One-shot API/ExaBGP injections require producer reannouncement. Route-server pre-policy retention belongs to `plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md`; it does not supply general import re-evaluation. No new spec owns the unfinished lifecycle: it remains this spec's work.

### Then: targeted verification after repairs

Run through the native job action, with the new session's identity, from the repository root. Allow 300 seconds for a focused race job; inspect its returned log, never poll it or start a full suite.

```sh
CGO_ENABLED=0 ./le --name session job run label rfc8212-source-deadline-after quiet command env CGO_ENABLED=1 go test -v -race -tags ze_core,ze_bgp,ze_web ./internal/component/plugin/server -run '^TestSourcePolicyOwnerEarlierCallerDeadline$'
```

For C2, use the same job form with package `./internal/component/bgp/reactor` and selector:
`^(TestRFC8212ImportEpochFences|TestRFC8212StoredReplayEpochFences|TestRFC8212ExpectedOwnerEpochFence)$`.
For compensation, package `./internal/component/plugin/server`, selector:
`^(TestReloadRecoversRemovedInfrastructureBeforeOriginalUndo|TestReloadRecoversRemovedServiceAfterOriginalInfrastructureUndo)$`.

After the repairs, run the affected package/behavior controls and independently review the changed producers. C3's broader package coverage, full combined diff reading, real entry-point smoke, RFC proof records, component/functional scopes and changed-commit verification remain owed. No full verification, full unit suite, full functional suite or performance/allocation run is authorized on this Linux host.
