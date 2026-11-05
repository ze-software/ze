# Spec: CLI history persistence must not block the update loop

| Field | Value |
|-------|-------|
| Status | done |
| Scope | cli |
| Priority | P1 |
| Release reason | An interactive session stops processing input, timers and delivered command results while persisting command history to a slow durable store |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-05 |

## Task

Move command-history publication off the CLI's serial update loop while preserving useful history, ordered session-local publication and graceful owner shutdown. Reuse the existing bounded keyed coalescer rather than building a queue or starting a goroutine for each command.

The reported full-run CI is run 41685742426, source commit `1a2ccaeef8c0da4dc9f3747ce70f4aeecb84ddfa`. A subsequent real disk-backed native reproduction captured the daemon in `tmp/session/2026-11-03-231620/scratch/session-stall-inv79-daemon-stacks.json`. Goroutine 552 is in `Model.Update` through `handleEnter`, `History.Save`, `store.WriteFile`, `treeEncoding.installBytes` and the destination-directory fsync. Goroutine 849 independently persists an editor draft. These are different runtime/configuration writer locks, not evidence of a lock cycle. The capture proves the UI history-I/O boundary is wrong; it does not prove that fixing it removes every native deadline failure or identifies why the disk is slow.

The main thread selected the session-scoped design on 2026-11-04 after source-verifying the coalescer and all three save branches. Preserve existing whole-snapshot behavior between concurrent sessions, including sessions of the same user. This work introduces no daemon-global merge or chronology contract. Before product implementation, the main thread must run the new old-API-compatible regression and record its actual pre-fix red. Existing tests may change only after independent test judgment.

Thomas authorized a one-off admission for this named fix on 2026-11-04, choosing “Authorize this named fix only.” The native claim admitted it with `ZE_SPEC_WIP_CAP=23` scoped to that one invocation; the permanent cap remains unchanged. Main read the actual fifteen-line pre-fix log `tmp/session/2026-11-03-231620/scratch/job-ci-history-pre-fix-red-834e15a4.log`: all three Enter branches reached the held writer and failed useful input/timer/result progress, with no secondary cleanup/key/payload failure. Native patched-toolchain `go test -v -p=1 -count=1 -timeout=2m` used the full registered tags and only `TestModelHistoryBlockedWriterKeepsUpdateProgress`; exit 1 was the intended product red. Before phase-two Go edits, HistoryCompletionJudge independently approved the exact six history-root and two model-root completion-boundary proposals and recorded all eight native receipts in `tmp/commit-weakening-approved-21df5267.md`. Only those exact existing-test edits were applied. No post-fix test, race, native runtime or formatter result is claimed by the authoring phase.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md`, `ai/doors/le-tooling.md`.
  → Constraint: preserve the command registry and storage boundary; use the existing long-lived worker mechanism and the owning model's cleanup path.
- [ ] `docs/contributing/ze-go-style.md`, in full; `ai/rules/go-standards.md`, `ai/rules/goroutine-lifecycle.md`, `ai/rules/testing.md`, `ai/rules/planning.md`.
  → Constraint: no per-event worker, unbounded queue, new dependency, unchecked mutable snapshot ownership or unexecuted red claim. A copied Model must share its resource owner.
- [ ] `docs/features/cli-commands.md`, Configuration Management and Command history persistence.
  → Constraint: keep daemon-owned history, per-user/per-mode keys, consecutive dedup, default 100/max 10000, restart persistence and nil-store operation. The page does not require result delivery to wait for history fsync.
- [ ] `docs/architecture/storage-backends.md`, Opening and ownership, Keys and reads, Durable publication.
  → Constraint: keep all framing, locks, descriptor-relative publication, permission checks, frame/ancestor/directory syncs and storage error propagation. History borrows its store and never closes it.
- [ ] `docs/architecture/zefs-format.md`, Key Registry.
  → Constraint: preserve the registered history namespace and existing line-delimited payload; this is not content-addressed configuration history.
- [ ] `docs/architecture/config/yang-config-design.md`, Module Loading; `docs/guide/config-editor.md`, Usage.
  → Constraint: retain real editor construction, schema-driven behavior and daemon ownership. No new YANG, grammar or validation behavior is required.
- [ ] `ai/digests/cli-editor.md`, flow and state-through-result invariant; `plan/learned/DESIGN-HISTORY.md`, CLI load-bearing invariants.
  → Decision: history publication is independent of config command turns; command results still flow through Update, and no storage I/O is moved into a UI callback.
- [ ] `plan/README.md`, `plan/templates/TEMPLATE.md`, `ai/rules/repo-maintenance.md`, `ai/rules/spec-no-code.md`.
  → Decision: this is a release lifecycle fix, not an optional performance feature. The spec carries the source census, explicit surviving behavior and discovery obligations.
- [ ] `../wiki/cli.md`, Interactive features.
  → Constraint: clarify nonblocking publication and graceful drain alongside its existing across-session persistence statement; do not promise crash-proof queued history.

### RFC Summaries

N-A: no protocol behavior or RFC claim changes.

**Key insights:**
- A persistent History owns UI entries and browsing state, but borrows the store. Only an immutable bounded snapshot may reach a background handler.
- Mode restoration changes the same History's entries. Capture mode and payload before queuing, not when the writer eventually runs.
- The shared Coalescer already handles nonblocking latest-per-key work, per-key exclusion and drain/join. One History needs one worker and at most two pending mode snapshots.
- Closing config dispatch cancels queued turns; it is not a history drain. The history drain must finish without any final Update/result delivery.
- An editor draft can still block in its own fsync and delay command completion. Do not claim all CI stalls solved from the history unit test.

## Current Behavior

This census records the audited pre-fix baseline and preservation constraints.
The phase-two source integration section below records the current written cutover.

**Source files read:**
- [ ] `internal/component/cli/history.go`: `NewHistory`, `Max`, `Entries`, `Append`, `Up`, `Down`, `resetBrowsing`, `snapshot`, `restore`, `Load`, `Save`.
  → Constraint: retain key/payload bytes, bounds, normalization, dedup and browsing semantics. Nil-store Save currently returns before trimming; preserve it.
- [ ] `internal/component/cli/model_keys.go`: `handleEnter`, `handleKeyMsg`, `handleSetCLIFormat`, history navigation.
  → Constraint: all three Save sites change together: config `run ...`, operational `set cli format`, ordinary dispatch. Auto-config commands save after switching; `run ...` saves in config mode.
- [ ] `internal/component/cli/model.go`: constructors, `Update`, `handleCommandResult`, `Close`, `SetHistory`, `SetStartMode`, `handleDraftPoll`.
  → Constraint: model copies share one history/lifetime. Preload both modes before running; neither result application nor timer handling waits for history publication.
- [ ] `internal/component/cli/model_mode.go`: `SetMode`, `modeState`, `executeOperationalCommand`.
  → Constraint: switching remains memory-only; operational result execution stays independent of history work.
- [ ] `internal/component/cli/model_commands.go`: `dispatchQueue`, `executeCommand`; `internal/component/cli/model_timer.go`: `TimerMsg.Wait`, `tick`.
  → Constraint: keep config ordering/result-application turns and existing timer context/clock. A stopped program cannot be required to apply a result to drain history.
- [ ] `internal/core/syncutil/coalescer.go`: `NewCoalescer`, `Enqueue`, `Await`, `run`, `Stop`.
  → Decision: use one worker, replacement merging and bounded real-mode keys. Do not introduce a second scheduler abstraction.
- [ ] `cmd/ze/hub/session_factory.go`: both persistent History installation branches; `internal/component/ssh/ssh.go`: `teaHandler`, `sessionCloseMiddleware`, `Stop`.
  → Constraint: SSH already registers the initial value model as closer; closing that copy must drain work submitted by later copies.
- [ ] `internal/component/cli/client/main.go`: `newAttachedModel`, `runInteractiveWithDispatch`; `internal/component/config/cli/cmd_edit.go`: `runBackupModel`, `runEditor`, `runEditorProgram`.
  → Constraint: attached console and local editor owners currently omit Model.Close. Add cleanup on normal, error, canceled and failed-start paths before their underlying resources close. Avoid starting an unused persistent worker before attached-model creation succeeds.
- [ ] `internal/component/cli/sshclient/terminal.go`: `RunInteractive`.
  → Constraint: this is a byte transport only. It must not create, save or close daemon-owned history.
- [ ] `internal/component/cli/testing/headless.go`: constructors, command/timer processing, `close`; `internal/component/cli/testing/runner.go`: history wiring, restarts, model/store cleanup.
  → Constraint: retain model-before-editor-before-store cleanup and close-before-restart; do not add headless-only history flush behavior.
- [ ] `internal/component/cli/history_test.go`, `model_test.go`, `model_mode_test.go`, `model_dispatch_race_test.go`, `model_timer_lifetime_test.go`.
  → Constraint: preserve existing roundtrip, max, per-mode, preload, browsing, dedup and dispatch lifetime proofs. Immediate-persist assertions need independently judged drain-boundary changes, not assertion deletion.

### Complete source census

| Role | Actual sites | Disposition |
|------|--------------|-------------|
| History saves | `handleEnter`: config run, operational format, ordinary command | Queue all three, preserving their current mode and accepted-command population |
| History installation | Model constructors with nil store; `SetHistory`; SSH factory's editor/command-only branches; attached console; headless `wireHistory` | Keep construction/preloads; establish one clear History owner per model |
| Mode state | `SetMode` calls `History.snapshot/restore`; `SetStartMode` delegates to it | Preserve UI ownership and memory-only switching |
| History reads/navigation | `NewHistory` max read; `SetHistory` current/opposite Load; `handleHistoryUp/Down`; reset on typing | Keep synchronous construction reads outside Update and existing navigation semantics |
| Normal/abrupt SSH terminal and daemon end | `teaHandler`, `sessionCloseMiddleware`, `Server.Start/Stop`, `modelSessions`, `modelSession` | Each channel owns its initial model closer; Stop seals its captured generation and joins admitted factory/model cleanup before store teardown, even after network shutdown error |
| Hook-built daemon SSH ownership | `runYANGConfig` immediately deferred `sync.OnceFunc(apiServer.Stop)`; `setupInfraHook` captures actual owner and registers SSH Stop with existing OnStop; `sshServer.Stop(context.Context)` | Normal and startup-error returns join the same owner; private constructor gate/seal joins in-flight construction and synchronously closes a just-built handle whose registration is refused; no callback-order assumption |
| Listener startup admission | `Server.Start`, `serveListener`, `registeredListener.Accept` | Every primary/extra listener has actually reached vendor registration before Start returns; a pre-Accept Serve error completes startup without a missed notification |
| Attached console | `runInteractiveWithDispatch` | Add owner cleanup, including TTY-open and program failure |
| Local loose-file/backup terminal | `runEditor`, `runBackupModel`, `runEditorProgram` | Close every constructed model on all paths, before editor/store release |
| Headless models/restarts | `headlessModel.close`, runner deferred model loop and restart branch | Existing ownership stays; new history drain follows the same path |
| Remote client | `sshclient.RunInteractive` | Intentionally unchanged: no History is owned here |
| Save-dependent unit callers | `history_test.go`: load/save, rolling, custom max, per-mode, nil, newline; `model_test.go`: both persistence-on-Enter tests | Await explicit completion before reload; preserve exact resulting content and limits |
| Existing functional readers | `test/editor/commands/cmd-history-persist.et`, `cmd-history-persist-command.et`, `cmd-history-max.et` | Keep expectations and restart route unchanged |

Search scope was `internal/`, `cmd/`, editor tests and plan work-item/journal folders; terms included NewHistory, SetHistory, history.Save, history methods, Model constructors, Close and NewProgram. Positive controls were handleEnter's captured Save and both real SSH factory producers. Main later obtained LSP references before export edits: seven history-test and three production Save callers, plus SetHistory and Model.Close references. Default-build LSP omits tagged SSH/hub, so the explicit producer census remains necessary. Phase two read the actual local, SSH and headless producers rather than treating that default reference set as exhaustive.

**Behavior to preserve:** existing command acceptance/modes, per-user/per-mode whole-snapshot bytes, rolling bounds/dedup/newline normalization, preload/navigation, nil-store behavior, best-effort history errors, storage durability, config-command sequencing, and all disk-backed force/verify expectations.

**Behavior to change:** history WriteFile no longer executes on the Update goroutine; graceful model shutdown drains accepted history publications; all actual model owners discharge Close.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Keyed nonblocking coalescer | `internal/core/syncutil/coalescer.go`: Enqueue, run, Stop | One waiting value per key, serial handling for that key, draining shutdown | Extends its use with one History-owned worker and newest-snapshot replacement |
| History bounds and encoding | `history.go`: Save, Load, historyMaxDefault/historyMaxCeiling | Per-user/mode line payload, durable whole-snapshot write, newest-entry trimming | Extends Save to capture an immutable bounded publication, preserving bytes |
| Config-command turn ordering | `model_commands.go`: dispatchQueue | Reserves in Update, finishes after result application, cancels at close | Does not apply to history writes: neither bounds pending history nor drains without Update |
| Shared model timer lifetime | `model_timer.go`: TimerMsg; `model.go`: Close | Copies share cancellation and clock | Preserves it; Close additionally drains the independent history owner |
| SSH model cleanup | `ssh.go`: sessionCloseMiddleware | Closes the initial model after inner program completion | Extends the resource contract through Model.Close, not another middleware |
| Headless lifetime cleanup | `headless.go`: close; `runner.go`: restart/deferred cleanup | Closes model before editor/store and before replacement | Reuses unchanged lifecycle order for history drain |
| Durable storage protocol | `docs/architecture/storage-backends.md`: Durable publication | Frames, syncs, publishes and syncs destination directories | Preserves every barrier; slow I/O runs inside the worker |

## Data Flow

### Entry Point

An accepted Enter key reaches Model.Update and one of handleEnter's three existing history-save branches. Empty input, completion acceptance, configure, exit/quit and stop/restart retain their existing history inclusion/exclusion behavior.

### Transformation Path
1. Append performs existing normalization/dedup on the serial UI owner.
2. Save resolves the current real mode, trims persistent entries as before, and captures detached immutable publication content and its existing user/mode destination.
3. Enqueue submits that value to the one existing Coalescer instance. A waiting snapshot of the same mode is replaced by the newer complete snapshot; input processing does not wait for storage.
4. The worker performs the unchanged borrowed historyRW.WriteFile call. Only immutable publication values cross to it; it never reads entries, browsing state or modeStates.
5. Model.Close cancels its existing command/timer scope and drains/joins the History owner independently of UI result delivery. It does not close the borrowed store. Parent resource cleanup follows after this returns.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| UI state to background work | Detached bounded snapshot with resolved mode/destination | Source-derived design; regression and snapshot tests required |
| History to storage | Existing historyRW WriteFile | Existing interface and storage contract; no new encoding |
| Program to resource owner | Existing Model.Close and SSH/headless cleanup, completed for local owners | Source census; ownership tests required |

### Changed-Wire Census

No wire, storage key or payload format changes. Searches for history.Save, NewHistory, SetHistory, meta/history and option=history identified the Go producers/readers and embedded editor/native script consumers listed above. Preserve the existing key prefix, config/operational mode spelling, line separator and store framing. The SSH session-editor scripts continue to request and assert the same statuses and committed files. No shell/Python history serializer exists in the searched population; none is introduced.

### Integration Points

| Point | Integration |
|-------|-------------|
| History.Save | Nonblocking publication admission with immutable ownership |
| Model.Close | Drain/join History, preserve cancellation and transcript cleanup |
| Model.SetHistory | Setup-time replacement only; do not orphan a started owner |
| Attached/local owners | Close successfully constructed models on every exit path |
| Existing persistent tests | Explicit completion before reload, following independent test judgment |

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes, by design | Worker calls existing historyRW; storage publication unchanged |
| No unintended coupling | Yes, by design | Session-owned History; no daemon-global user registry or storage-specific branch |
| No duplicated functionality | Yes, by design | Existing syncutil.Coalescer does scheduling/merging/draining |
| Necessary ownership copies only | Yes, by design | Mutable UI entries never shared with writer; compare immutable encoded payload versus detached entry snapshot and choose the representation avoiding redundant encoding/copies |
| Outbound registration | N-A | No command, view, family or handler introduced |
| Inbound registration | N-A | Existing mode and history callsites only; no parser/runner/completion feature-name list introduced |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | No existing contract requires command results to await history fsync | History.Save ignores write errors; feature docs promise persistence, not result/fsync ordering; main thread selected this boundary | Async behavior would require a different product decision | Parent source verification and documentation contract check | Confirmed by main selection |
| A-2 | Session-local ordering, not a global same-user chronology/merge, is the preservation requirement | Separate NewHistory per model in SSH factory; main thread's explicit selected scope | Per-History worker would not satisfy a global chronology promise | Same-user session preservation case and source census | Confirmed by main selection |
| A-3 | A History's two real modes bound pending work | EditorMode.String returns exactly the existing config or operational storage name, even for other enum values | Arbitrary queue keys would defeat the bound | TestHistoryModeProjectionBoundsPublicationKeys and TestModelHistoryModeSnapshotsDrainInOrder passed under race and count instrumentation | Confirmed |
| A-4 | Slow draft I/O can independently delay command results | Captured draft worker is also in destination-directory fsync under a separate guard | History responsiveness alone would not resolve all native failures | Later live stacks and syscall traces captured independent required commit/draft syncs. The approved observer follows completion within the unchanged case bound. | Confirmed. No disk-latency fix is claimed. |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Writer observes later mode's entries or aliases a reused backing slice | Cross-mode payload contamination while held | Resolve mode and detach payload synchronously before enqueue; restore/append isolation test |
| R-2 | Older write overwrites newer session snapshot | Final reload returns an earlier entry | One worker, serial per-key handling, newest pending replacement; exact write sequence test |
| R-3 | Work grows with Enter count while disk is held | Pending snapshot/call count follows burst size | At most two waiting mode snapshots plus one active write; no per-event goroutine |
| R-4 | Shutdown discards queued history or waits for a stopped Update | Close returns early or never finishes after writer release | Coalescer drain/join independent of dispatch cancellation/result application; copied-model Close regression |
| R-5 | Attached/local error paths leak workers or write after store closes | Late write after owner return | Lazy start and cleanup for each constructed model before editor/store release; owner tests |
| R-6 | History closes a borrowed daemon store | Other sessions receive closed-store errors | History never calls store.Close; borrowed-store reuse test |
| R-7 | Abrupt process death loses queued command history | Last typed commands absent after a forced kill | Document this timing consequence; graceful close drains; do not weaken config/draft durability or add a retry/deadline |
| R-8 | Existing immediate-read tests are weakened instead of preserving persistence proof | Assertions removed or reload no longer exercised | Independent ze-test-judge before applying proposed completion-boundary edits; retain exact contents/max/mode checks |
| R-9 | Constructor failure starts an unowned worker | Persistent History created but never attached | Start lazily and establish cleanup before possible program/TTY failure |
| R-10 | A blocked store keeps graceful Close waiting | Close remains blocked while gate is held | Expected durable drain behavior; test that release completes it. No invented cancellable fsync or timeout |
| R-11 | Fix is announced as resolving disk slowness or all native deadlines | Claim exceeds unit/native evidence | Report responsive UI proof separately from original full-run/native rerun results |
| R-12 | SSH network shutdown finishes before disconnected TUI model cleanup | Server.Stop returns while sessionCloseMiddleware is still draining history | Seal a per-run model-owner barrier before network shutdown; force network closure on its error, then join model cleanup unconditionally before store teardown; do not join synchronous exec stop callbacks |
| R-13 | SSH channels share one connection context and overwrite the same closer slot | Two TUI sessions on one SSH connection close the same last model, orphaning another history worker | Bind each model closer to a private per-session owner cell, not a connection-context slot or a cumulative per-session context-key map |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Lost/reordered command history, mode contamination, session worker leaks, late store writes, or canceled CLI timers/commands |
| How is it reverted? | Revert the focused implementation change; stored format remains identical and needs no migration |
| Who else touches this path? | Shared main checkout contains concurrent work; preserve foreign hunks, especially model timers/headless ownership and session fixture code |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Real Model.Update Enter in config show, config run and operational format | → | handleEnter's three History.Save calls | TestModelHistoryBlockedWriterKeepsUpdateProgress |
| Real mode changes with held history writer | → | SetMode, snapshot/restore, Save | TestModelHistoryModeSnapshotsDrainInOrder |
| Close through an earlier Model copy | → | Model.Close, History drain | TestModelHistoryCloseDrainsCopiedOwner |
| SSH program and no-PTY completion | → | sessionCloseMiddleware | Existing TestSSHPTYClosesModelAtSessionEnd and TestSSHNoPTYClosesModelAtSessionEnd; new TestSSHPTYDrainsHistoryAtSessionEnd and TestSSHNoPTYDrainsHistoryAtSessionEnd |
| Disconnected model cleanup, overlapping channels and daemon Stop/restart | → | actual sessionCloseMiddleware, teaHandler, Model.Close, sealed modelSessions | TestSSHStopDrainsDisconnectedModelBeforeStoreClose; TestSSHChannelsSharingConnectionCloseTheirOwnModels; causal transcript, generation, factory and exec controls |
| Immediate SSH Start then Stop, including extra listeners | → | actual vendor Serve/trackListener admission and registeredListener | TestSSHServeReadinessFollowsListenerAdmission; TestSSHListenerReadinessReportsEarlyServeFailure; TestSSHStartStopImmediatelyJoinsEveryListener |
| Actual daemon normal return or engine-start refusal after hook-built authenticated SSH accepts history | → | runYANGConfig OnceFunc owner, setupInfraHook OnStop and actual Server.Stop | TestInfraSSHOwnerDrainsBeforeStoreCloseOnStartupFailure; TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown |
| Attached/local program completion or refusal | → | runInteractiveWithDispatch, runEditorProgram and constructors | TestAttachedModelOwnerClosesHistory, TestEditorProgramOwnerClosesHistory |
| Headless restart | → | headlessModel.close then runner re-creation | Existing cmd-history-persist.et and cmd-history-persist-command.et |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A real model submits config show, config run or operational format while history WriteFile is held | Enter returns with cleared input; a following rune reaches the prompt; declared timer expiry advances its real handler; an independent command result is applied while history is still held; exact history entry is eventually written | Config command/draft I/O can independently wait; this fix removes history as an Update prerequisite, not every possible I/O stall |
| AC-2 | The first publication is held while many further commands in both modes are accepted | Browsing reflects accepted commands immediately; pending history is at most one newest snapshot per mode plus one active write; after release/drain each mode reloads the latest retained entries in session order | Superseded waiting snapshots need not be written because a newer whole snapshot contains the retained history |
| AC-3 | A mode changes before queued writes execute, including run and auto-config behavior | Each actual mode's history reloads only its own commands, preserving the existing mode attribution and exact user prefix | Different sessions of the same user keep existing independent whole-snapshot/last-publication behavior; no global merge is promised |
| AC-4 | Program ends with accepted publications queued, including an earlier Model copy closing it | Close waits while storage is held, then returns after all final mode snapshots finish; repeated close adds no writes and no worker survives; editor/store resources close afterward | A permanently blocked storage call can keep graceful shutdown waiting because the existing interface cannot cancel fsync |
| AC-5 | SSH (including overlapping channels and daemon Stop), attached console, local loose/backup editor, headless restart, and constructor/TTY/program refusal | Every successfully constructed owner closes its own model; history drains before borrowed-resource release. SSH seals model admission and joins admitted factory/model cleanup independently of network shutdown error; restart cannot admit old-generation handlers | Remote SSH client owns only terminal transport; nil-store models start no persistence worker. A permanently blocked history write can hold graceful Stop beyond its network deadline; that deadline cannot discard accepted history |
| AC-6 | Default/custom/clamped maximum, duplicates, newline input, preload, Up/Down and nil store | Existing exact bounded persistent contents, consecutive dedup, normalized bytes, browsing and nil-store memory behavior remain useful and unchanged | Nil-store Save preserves its current no-trim behavior; the bound here is pending persistence work, not a new nil-store policy |
| AC-7 | Original disk-backed force/verify native scripts and editor restart cases | Existing statuses, pending-change counts, committed-file assertions and restart recall remain correct with no expectation/deadline/polling changes | A remaining timeout with a draft worker in fsync is reported as remaining evidence, never silently treated as fixed by this spec |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Enters another command while history storage is slow | Enter to detached snapshot to worker; next key/timer/result through Update | TestModelHistoryBlockedWriterKeepsUpdateProgress |
| 2 | Switches config/operational mode then disconnects and reconnects | Mode snapshots to ordered publication; owner Close; new model preload | TestModelHistoryModeSnapshotsDrainInOrder and existing history restart .et cases |
| 3 | Uses disk-backed session commit verify/force | Existing SSH model, config turn, draft/commit and status route | Both native cases retain validation and state proof after independently judged input/observer repairs. |

## TDD Test Plan

### Unit Tests — plan and phase-two record

The Status column preserves the phase-two handoff. The final executed results are in Implementation Audit and Pre-Commit Verification below.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestModelHistoryBlockedWriterKeepsUpdateProgress | internal/component/cli/model_history_blocked_test.go | Old-API deterministic red through all three Save branches and useful input/timer/result progress | Main executed actual pre-fix red and post-fix race PASS; installed regression unchanged |
| TestModelHistoryModeSnapshotsDrainInOrder | internal/component/cli/model_history_async_test.go | 129 real accepted commands, max-three retention, immediate browsing, detached backing-slice isolation and exact three-write sequence | Main race PASS after bubble setup correction; actual Clip alias mutant fails submitted poison payloads, restored Clone passes |
| TestModelHistoryCloseDrainsCopiedOwner | internal/component/cli/model_history_async_test.go | Held-writer Close through old model copy, dispatch ended before drain, unapplied config result, repeated close and borrowed-store reuse | Main race PASS in corrected bubble population |
| TestHistoryIdleAndNilStoreStayWorkerFree; TestModelSetHistoryDrainsReplacedOwner; TestModelHistoryAutoConfigAttribution; TestHistorySessionsKeepIndependentSnapshots; TestHistoryPublicationRespectsMaxBoundaries; TestHistoryModeProjectionBoundsPublicationKeys | internal/component/cli/model_history_async_test.go | Idle/nil no-worker/no-trim behavior, replacement drain, real auto-config/run attribution, no global merge, exact bounded reload and two-key projection | Main race PASS, exact root names in history-cli-affected-results.json |
| Existing history roundtrip/max/per-mode/nil/navigation tests | internal/component/cli/history_test.go | Existing exact payload and useful retained entries after explicit completion | Exact judged proposal retained; Main's earlier observed passing population, listed in affected-results skipped-root census |
| TestModelHistoryPersistOnEnter and TestCommandModelHistoryPersistOnEnter | internal/component/cli/model_test.go | Real-store persistence after owner completion and exact reload contents | Exact judged proposal retained; both Main race PASS in affected-results census |
| TestAttachedModelOwnerClosesHistory; TestAttachedModelConstructorRefusalOwnsNoHistory | internal/component/cli/client/history_owner_test.go | Actual attached owner with real Update/Enter, borrowed terminal outcomes (normal/TTY refusal/program failure/cancel), real schema constructor refusal | Both Main race PASS in affected-results census |
| TestEditorProgramOwnerClosesHistory; TestEditorConstructorRefusalReleasesOwnedStore | internal/component/config/cli/history_owner_test.go | Real loose/backup editor construction, production program-owner drain across terminal outcomes, both actual local owners release editor-owned stores on schema refusal | Program owner Main race PASS; constructor-refusal root does not appear in that selected census and remains explicitly owed |
| TestSSHPTYDrainsHistoryAtSessionEnd; TestSSHNoPTYDrainsHistoryAtSessionEnd | internal/component/ssh/history_owner_test.go | Authenticated real SSH PTY/no-PTY session, actual initial Model closer and later Update copy; eventual owner completion and exact reload | Both retained with assertions unchanged; causal ordering belongs to the separate actual-middleware regression |
| TestSSHStopDrainsDisconnectedModelBeforeStoreClose | internal/component/ssh/history_shutdown_test.go | Canceled network shutdown cannot overtake disconnected model's exact operational+config publications before real store Close | Main useful old-Stop red, then actual PASS before the separate listener race; affected rerun after startup source correction still owed |
| TestSSHSessionCleanupDrainsBeforeTranscriptClose; TestSSHRestartKeepsOldModelAdmissionSealed; TestSSHStopJoinsFactoryAlreadyAdmitted; TestSSHExecStopCallbackDoesNotJoinItself | internal/component/ssh/history_shutdown_test.go | Causally admitted actual Model.Close/transcript order, stopped/restarted generation refusal, explicit missing-wrapper refusal, in-flight factory/getter cleanup and synchronous exec callback bypass | Full post-cutover execution owed; prior causal-transcript attempt exposed actual listener startup race rather than a pass |
| TestSSHChannelsSharingConnectionCloseTheirOwnModels | internal/component/ssh/history_channel_owner_test.go | Two actual models/transcripts over the same connection context close independently; exact separate publications and unchanged last-whole-snapshot behavior | Main useful old-slot red, then actual PASS before separate listener race; source-affected rerun still owed |
| TestHeadlessHistoryOwnerDrainsBeforeEditorClose; TestHeadlessHistoryRestartDrainsBeforeReload | internal/component/cli/testing/history_owner_test.go | Actual headless close/store-owned editor order under held write and actual runner restart recall in both modes | Main observed both PASS, in earlier/remaining affected-result populations respectively |
| TestSSHServeReadinessFollowsListenerAdmission; TestSSHListenerReadinessReportsEarlyServeFailure; TestSSHStartStopImmediatelyJoinsEveryListener | internal/component/ssh/listener_start_test.go | Real pre-registration entropy gate, genuine vendor signer failure with closed/no-Accept listener, actual primary/extra immediate Start/Stop | Authored; post-source runtime proof owed |
| TestInfraSSHOwnerDrainsBeforeStoreCloseOnStartupFailure | cmd/ze/hub/infra_history_owner_test.go | Actual Run→installed hook→real builder/authenticated PTY/Model→engine refusal→caller store Close with exact useful key/payload | Main actually executed useful old-owner red after correcting explicit source binding and waiting for foreign launcher cutover; source integration written, post-fix pass owed |
| TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown | cmd/ze/hub/infra_history_owner_test.go | Same actual Run/hook/SSH/Model/store assertions on real accepted shutdown request and normal exit 0 | Authored using the same assertions/fixture; actual post-fix execution owed |
| Existing dispatch/timer lifetime tests and SSH close tests | Existing cli and ssh test files | Unchanged cancellation/result-ordering and transcript lifetime proof | Existing unchanged population; main executes affected tests |

### Deterministic blocked-writer method

Create a real editor-backed Model and install only a channel-gated historyRW test double. Under testing/synctest, one serialized driver sends Enter and the following actual input, timer and command-result updates. The test waits until goroutines are durably blocked and checks progress by nonblocking channel observation, never an elapsed sleep. Writer entry is itself asserted, so dropping all history cannot pass. Keep its release gate held until the progress assertions; release and join on every failure path. Verify the actual resulting mode/key and exact line payload after release/Close. Before the fix the driver is blocked inside History.Save, so useful UI progress is absent at the deterministic observation point. The main thread records actual red output before any product edit.

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Persistent history maximum | 1..10000, default 100 | Existing 1 and 10000 cases retained | Existing zero/negative clamp-to-1 cases | Existing above-ceiling clamp-to-10000 case |
| Pending mode keys | Exactly config and operational | Both pending while one write is active | N-A | Other keys are not admitted to the bounded worker |
| Pending snapshots | At most two waiting plus one active | Burst across both modes | N-A | Further same-mode submissions replace the waiting snapshot |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| cmd-history-persist.et | test/editor/commands/cmd-history-persist.et | Config history survives owner close/restart | Existing, unchanged |
| cmd-history-persist-command.et | test/editor/commands/cmd-history-persist-command.et | Operational history survives owner close/restart | Existing, unchanged |
| cmd-history-max.et | test/editor/commands/cmd-history-max.et | History recall survives restart | Existing, unchanged; actual numeric rolling bound remains unit-tested |
| session-editor-commit-force.ci | test/plugin/session-editor-commit-force.ci | SSH warnings/force/confirmed status and real committed bytes | Judged repair retains the original case bound and exact state assertions. Loaded and live PASS. |
| session-editor-commit-verify.ci | test/plugin/session-editor-commit-verify.ci | SSH candidate validation and unchanged running config | Judged repair retains the original case bound and exact state assertions. Loaded and live PASS. |

Interop tests are N-A: no protocol or wire behavior changes. Main runs scoped native unit/race and relevant editor/plugin tests after integration; this authoring phase runs none.

## Files to Modify

- `internal/component/cli/history.go`: immutable bounded asynchronous publication and owned drain; preserve borrowed store and format.
- `internal/component/cli/model.go`: history ownership in Close and setup-only SetHistory contract; shared copied-model lifetime.
- `internal/component/cli/model_keys.go`: migrate all three Save calls together if typed mode admission changes their signature.
- `internal/component/cli/client/main.go`: attached-console owner cleanup on every exit, including pre-program failures.
- `internal/component/config/cli/cmd_edit.go`: local editor model cleanup, including failures before runEditorProgram.
- `internal/component/cli/testing/headless.go`, `cmd/ze/hub/session_factory.go`: discharge editor construction ownership when model creation refuses; preserve successful SSH/headless ownership order.
- `internal/component/ssh/ssh.go`: serialize Start/Stop generations, seal and join actual admitted model cleanup before borrowed-store teardown, preserve network shutdown error/force closure, and bind each channel's own closer without shared context state.
- `internal/component/cli/history_test.go`, `model_test.go`, existing client/config CLI/SSH-close test files: independently approved completion-boundary changes and ownership coverage.
- `docs/features/cli-commands.md`, `docs/guide/config-editor.md`, `docs/guide/cli.md`, `ai/digests/cli-editor.md`, `../wiki/cli.md`: nonblocking history timing, graceful draining, unchanged config durability and abrupt-death limitation. Main owns the separate wiki checkout prerequisite/edit.

## Files to Create

- `internal/component/cli/model_history_blocked_test.go`: focused deterministic old-API regression; no production changes in its authoring phase.
- `internal/component/cli/model_history_async_test.go`, `internal/component/cli/client/history_owner_test.go`, `internal/component/config/cli/history_owner_test.go`, `internal/component/ssh/history_owner_test.go`, `internal/component/cli/testing/history_owner_test.go`: phase-two deterministic snapshot/drain and real owner/restart regressions.
- `internal/component/ssh/model_lifetime.go`: sealed per-run model admission/join and per-channel delegated Session closer cell; no alternate history queue or registry.
- `internal/component/ssh/history_shutdown_test.go`, `internal/component/ssh/history_channel_owner_test.go`: old-producer-compatible causal reds and shutdown/channel/factory/restart/exec controls.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No RPC, config or leaf introduced |
| YANG validation constraints | N-A | No modeled input changes |
| YANG custom validators | N-A | No value validation changes |
| CLI commands/flags | No | Existing Enter/history behavior only |
| CLI grammar | No | Commands, inclusion rules and parser unchanged |
| Editor autocomplete | No | Completion source unchanged |
| Functional test for new RPC/API | N-A | No new RPC/API; existing native SSH and editor cases retained |
| Pipe completeness | No | Command/result and pipe paths unchanged |
| Env var registration | N-A | No setting introduced |
| Doctor check for runtime dependencies | N-A | No new runtime dependency; existing store unchanged |
| Prometheus counters/metrics | N-A | No telemetry introduced |
| BGP family surface | N-A | No BGP protocol change |

### Documentation Update Checklist

| # | Question | Applies? | File to update / reason |
|---|----------|----------|-------------------------|
| 1 | Feature scope/evidence/level changed? | No | features/cli-commands.md remains experimental/partial with the same scope and declared gaps |
| 2 | Config syntax changed? | No | No parser/YANG change |
| 3 | CLI command added/changed? | No | Grammar and output unchanged; timing belongs in history documentation |
| 4 | API/RPC added/changed? | No | No declaration or payload change |
| 5 | Plugin added/changed? | No | No plugin change |
| 6 | Has a user guide page? | Yes | docs/guide/config-editor.md and ../wiki/cli.md explain graceful publication timing |
| 7 | Wire format changed? | No | Existing newline history and store frames retained |
| 8 | Plugin SDK/protocol changed? | No | No SDK edit |
| 9 | RFC behavior changed/proven? | N-A | CLI lifecycle only |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, Driving the SSH session editor from a script, now distinguishes case-owned output waits from the separate 20-second quit drain. Landed in `fa795bf224`. |
| 11 | Affects daemon comparison? | No | No support/feature claim expansion |
| 12 | Internal architecture changed? | Yes | ai/digests/cli-editor.md and docs/guide/config-editor.md document worker ownership and drain |
| 13 | Route metadata changed? | N-A | No route data |
| 14 | Prometheus counters changed? | N-A | No metrics |
| 15 | Registration/inventory changed? | No | Existing History and modes only |
| 16 | Source annotations/anchors? | Yes | history.go declares docs/architecture/zefs-format.md: unaffected because keys/format unchanged; model.go and model_keys.go declare docs/architecture/config/yang-config-design.md: unaffected because schema/editor grammar unchanged; client/main.go declares docs/architecture/core-design.md: unaffected because composition roots/component boundaries remain unchanged; cmd_edit.go declares docs/architecture/config/syntax.md: unaffected because parsing/serialization remain unchanged. docs/guide/config-editor.md is updated for owner/drain behavior. Main runs the native citation-anchor inventory before landing |
| 17 | Existing examples? | Yes | Check history feature description, editor restart .et examples and wiki interactive paragraph; no command spelling should change |

### Discovery Mechanical Checklist

| Question | Answer |
|----------|--------|
| Where does a future agent look first? | ai/INDEX.md command/editor entries to docs/features/cli-commands.md, docs/guide/config-editor.md and the living cli-editor digest |
| What prevents regression? | Deterministic blocked-writer test, bounded/latest snapshot tests, real-owner cleanup tests and unchanged restart/native scenarios |
| What registry/inventory prevents drift? | Existing zefs history key registry and EditorMode; no second name list or queue implementation |
| What verification proves it? | Main's executed pre-fix red, scoped post-fix unit/race run, editor restart suite and original disk-backed native scenarios |

## Implementation Steps

1. **Phase: Wiring and discriminating red.** Author only the new old-API-compatible Model.Update regression and this focused spec. Main executes the exact test before product changes and independently judges proposed existing-test edits. A failure must name blocked useful UI progress, not an unrelated setup/build failure.
2. **Phase: Session history cutover and ownership.** After the red and independent design gate, use the existing Coalescer for immutable mode-resolved snapshots; extend Model.Close and every real local owner; preserve SSH/headless ownership. Add bounded/order/isolation/close tests, apply only judged existing-test edits, and update the named pages with the behavior change.
3. **Phase: Main verification and independent review.** Main runs affected unit/race, existing editor restart and disk-backed native tests once integration is complete. Review covers every save site, mode transition and owner. Report any remaining draft-I/O timeout separately; no deadlines, expected statuses, fixtures or storage barriers are weakened.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three Save branches and SSH/attached/local/headless owners covered |
| Snapshot correctness | Worker cannot read live entries, mutable modeStates or a later mode |
| Bounds | One worker, at most two waiting real-mode snapshots and one active write |
| Shutdown | Copied-model Close drains independently of result delivery; SSH Stop seals/join models without s.mu or exec self-join, even on network error; old generations stay sealed and each channel closes its own actual model |
| Preservation | Same key bytes, line payload, maxima, dedup, nil-store behavior and concurrent-session semantics |
| Evidence | Pre-fix red actually executed; native pass never inferred from a unit test |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Responsive real Update under held history writer | Main runs TestModelHistoryBlockedWriterKeepsUpdateProgress pre/post fix |
| Latest ordered mode history and bounded work | Main runs focused snapshot/order/saturation/close tests under race |
| Ownership coverage | Main runs client/config CLI/SSH/headless cleanup tests |
| Existing useful persistence | Main runs history roundtrip/max tests and unchanged editor restart cases |
| Native behavior | Main executes the disk-backed force/verify behavior with independently judged input/observer repairs and unchanged case bounds. |
| Accurate documentation | Main checks changed source anchors/pages and wiki changes |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Resource exhaustion | Bursts cannot create per-event workers or an event-count-sized pending queue |
| Borrowed resources | No store close, use-after-close, orphaned worker or caller-owned mutable data crosses the worker boundary |
| Credentials/keys | No new path construction, logging, telemetry, redaction or authentication behavior |
| Storage guarantees | No removed sync, lock, frame validation, descriptor or permission protection |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Regression fails before useful progress assertion | Fix only the new test setup; preserve old API compatibility |
| Existing test needs a new completion boundary | Independent ze-test-judge before edit |
| Snapshot ownership/order bug | Fix the worker integration, not expected history contents |
| Native timeout remains in draft fsync | Main retains actual failure evidence and continues root-cause work; do not expand this change by weakening durability |
| Design cannot meet bounded owner lifecycle | Return to main architecture gate, not a parallel queue or timeout fallback |

## Design Insights

The captured history and draft writers use separate keyspace guards. Moving history to the existing coalescer repairs a demonstrated serial-UI boundary without diagnosing the underlying filesystem latency. A canceled config turn and a drained history publication are deliberately different lifecycle operations.

## Owned journal rows

| Class file | Date | Surface |
|------------|------|---------|

No existing open journal row was found owning this defect; no journal artifact is created merely to close this spec.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| [source] Existing Coalescer with session-scoped History ownership | Reuse config turns for the write | Turns are canceled rather than drained and depend on final Update/result delivery; inline format/run branches do not all own turns |
| One worker and latest complete snapshot per real mode | Multiple workers or every-event queued snapshots | Complete snapshots supersede waiting older ones; one worker preserves order without overlapping store writes or event-count work |
| Preserve existing session-local semantics | Daemon-global same-user history merge or chronology | Main explicitly rejected scope expansion; existing producers independently load/overwrite whole snapshots |
| Immutable mode/payload at admission | Closure over live History entries | SetMode restores that same mutable History before a slow writer may run |
| [workaround] Reject deadline/poll/fixture/storage weakening | Longer native deadlines, memory-only disk tests, skipped fsync, exit-only save | None repairs responsive real UI plus eventual ordered durable history |

## Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Extends existing keyed scheduler and model owner; no new global manager, setting, queue or command protocol |
| Uniformity | Uses the existing nonblocking coalescer/drain and the existing shared Model lifetime/owner cleanup contracts |
| Performance | Work is bounded by real modes, not command count; start lazily; transfer one detached immutable representation and avoid encoding a coalesced snapshot repeatedly when a cheaper safe representation fits |

## Known Limitations

- A process killed before queued command history is published can lose that queued history. Graceful owner shutdown waits; configuration/draft/commit durability is unchanged.
- Two sessions of the same user retain existing independent whole-snapshot behavior. This spec intentionally introduces neither chronological merging nor a daemon-global history authority.
- A slow draft fsync can still delay a config command's own result. This spec makes no unsupported claim that history responsiveness fixes every full-run CI timeout.

## Historical phase-two integration and verification

The entries below preserve what each phase knew when it reported. Their outstanding-work statements are historical. The closure audit records the final evidence and remaining limits.

`History.Save(EditorMode)` now clones only the trimmed string slice and admits it under `EditorMode.String()`'s existing storage name. Its lazy `syncutil.Coalescer` owns one worker; waiting snapshots replace by mode. `History.publish` encodes only snapshots actually taken, using the existing pooled text buffer and unchanged borrowed WriteFile. `History.Close` refuses later publication, drains and joins idempotently without dispatch/result dependencies; it never closes the borrowed store. Nil-store Save still returns before trimming.

`Model.Close` ends dispatch/timers, drains the shared history, then closes the transcript. `SetHistory` drains the previous owner and documents setup-only replacement. All three `handleEnter` sites use the typed current mode. Attached history/transcript acquisition now follows successful model construction, and its program owner closes the latest model on normal/TTY/program/cancel exits. Local loose/backup owners build their command tree before creating a model, transfer transcript ownership into the model, and run through the draining program owner before editor/store release. SSH retains initial-copy closer registration; headless retains model→editor→store and close-before-restart ordering. SSH/headless failed-model construction also discharges its already-created editor.

AC-1 through AC-6 now have source integration, including hook-built daemon SSH ownership and primary/extra listener startup registration, plus the named regressions. This is not an overall post-fix pass: only the explicit Main-exercised results below and in the unit census are green. Main still owns the new SSH/hub populations, local constructor-refusal root missed by the selected name filter, unchanged editor restart cases, original disk-backed force/verify scenarios, coverage/load, citation inventory, independent phase-three review and separate wiki work. No storage barrier, status, deadline, poll, grammar, key, wire, SDK, YANG or config syntax changed. Storage/format/schema/core-boundary source-header pages remain intentionally unchanged; inline feature/editor/CLI docs and living digests state actual producer ownership.

### Phase-two SSH ownership correction — source integrated

Confirmed source diagnosis: `Server.Stop` waits only on vendored `ssh.Server.Shutdown`; vendor `server.go:268-287` joins listeners/connections, but `:448-470` spawns channel handlers outside that join. Standalone SSH stop can therefore return to store teardown while an already-disconnected session is still inside real `Model.Close` history drain. Holding `s.mu` across that wait also conflicts with an in-flight model factory's existing Server getters. [workaround] trust connection completion or permit the network timeout to drop publications; [source] serialize Start/Stop separately, seal a fresh per-run model-owner admission before network shutdown and unconditionally join admitted model cleanup afterward, forcing network closure on shutdown error while preserving the original error. Main independently admitted that source design, with exec bypass to avoid synchronous lifecycle callback self-join and stale-generation rejection required. Main's actual old-Stop run `job-ci-history-ssh-shutdown-causal-red-b88c62c6.log` exited 1 at the intended early-Stop and exact missing-before-store-publication assertions; the complete twelve-line log was read. An earlier vendor-channel/synctest setup fatal is not counted as this red.

The new `history_shutdown_test.go` contains actual-middleware/real-Model regressions for Stop-before-store ordering, causally admitted transcript cleanup, stopped/restarted generation rejection, explicit unowned-callback refusal, in-flight factory/getter cleanup and synchronous exec callback self-join prevention. Its context adapter observes the real canceled Shutdown error branch before closing a real tracked idle connection. The vendor's lazy done channel is primed outside synctest while that connection remains alive; no deadline, polling or production notification is added.

Main independently admitted the per-session owner-cell repair after checking vendor `DefaultSessionHandler`, `Session.Context`, Wish bubbletea and activeterm. Its actual old-slot run `job-ci-history-ssh-channel-pre-fix-red-019e0a62.log` exited 1 because A's admitted cleanup waited on B's held history and A's real transcript remained open after A's publication; all thirteen lines were read before product edits. LSP found exactly the old key declaration/setter/getter before deletion; explicit source census accounted for Start's callback and all seven then-existing new-test calls through actual owner middleware. The additional direct unowned-callback control asserts honest refusal before factory invocation; its post-cutover execution remains owed.

The written source now has a fresh `modelSessions` barrier captured by each running server's existing middleware, and a private `modelSession` closer cell for each admitted channel. `Server.Stop` seals admission, preserves the network error while forcing connection closure if necessary, then unconditionally joins admitted factory/model cleanup before clearing the server fields and allowing restart. Start/Stop use a separate lifecycle mutex; getters retain s.mu, which is not held across shutdown/drain. Exec remains outside model admission so its synchronous lifecycle callback cannot join itself. The obsolete shared connection-context key/setter/getter are deleted; Session/context/authz/PTY methods delegate unchanged, and teaHandler explicitly refuses a missing owned wrapper. No global registry, cumulative context-key map, alternate queue or silent cleanup fallback exists. This continuation ran no checks; all post-cutover proof is main-owned.

### Phase-two listener startup and daemon composition continuation

Main's `job-ci-history-ssh-ownership-restored-2b538741.log` reached a fixture-only panic after the channel body: WaitGroup Add occurred inside synctest but the final outer cleanup Wait ran outside. Main moved the actual final Server.Stop into the owning bubble for the channel and causal-transcript cases; assertions and exact payloads remain unchanged. The subsequent `job-ci-history-ssh-ownership-bubble-joined-2b538741.log` actually passed the channel and disconnected-Stop roots, then reported a product race and timed out in immediate Shutdown: vendor Serve's listenerWG Add could happen after Shutdown's Wait and listener snapshot. This is actual startup/teardown producer evidence, not storage latency or an admitted passing fixture.

Source repair now makes each existing Serve lifecycle goroutine notify startup once, after its actual first Accept boundary (vendor trackListener already added it), or with its actual pre-Accept Serve error. Start holds its lifecycle mutex until the primary and every successfully bound extra have completed this handshake. A primary early error clears its network ownership and returns the error without recursive Stop; failed extras retain existing best-effort logging/skip semantics. No vendor patch, polling, new goroutine, per-Accept allocation or deadline change is introduced.

New `listener_start_test.go` roots are `TestSSHServeReadinessFollowsListenerAdmission` (actual ensureHostSigner entropy Read held before registration, then actual registered-listener Shutdown), `TestSSHListenerReadinessReportsEarlyServeFailure` (genuine vendor signer entropy failure through serveListener, exact error/closed-listener/no-Accept assertions), and `TestSSHStartStopImmediatelyJoinsEveryListener` (actual production Start/Stop with primary and extra listeners). The helper-only error echo and incidental log gate were rejected before execution and removed. Since Go 1.26, documented cryptocustomrand compatibility and a serial/restored crypto/rand.Reader drive these genuine RSA paths; restoration follows joined serving goroutines. These new roots remain unexercised by this worker.

The BGP audit found the old setupInfraHook discarded infraSetup's SSH return, the old seam exposed only Address, and the main deferred SSH stop covered only !hasBGPBlock. The actual loader invokes infra.Run before the BGP plugin injects params.APIServer, so that provider could not own early failure. Main's actual corrected `job-ci-history-hub-owner-after-shared-cutover-f064baf9.log` exited 1 after real listen/authentication/admin session and engine refusal: Run released the store while the real history write remained held; store Close saw [] instead of exact meta/history/operator/operational = show daemon-owned. All 14 lines were read before hub source edits; rescue Stop added no setup/cleanup failure. Main then explicitly granted the admitted OnStop/OnceFunc cutover.

Earlier `job-ci-history-hub-owner-pre-fix-red-f064baf9.log` failed before the hook because the outer test interceptor hid explicit source binding; it was setup failure, not product red. The fixture now binds its outer interceptor with existing BindConfigSource(..., ConfigSourceFile), passes that same borrowed handle to Run/the hook, and closes through the real interception/store afterward. No source guard was waived.

The subsequent source-bound attempt (Main bg171) stopped before test execution on concurrent foreign BGP Coordinator/factory/RPC interface cutovers. Main read changed actual consumers and the new CanonicalPayloadBudget producer before the one changed-source retry that produced the useful red above. This worker did not edit foreign migration code, use a stale launcher/copy, or count a compile refusal as runtime product evidence.

Main reports the remaining four CLI-side affected packages pass with 65 completed roots (`job-ci-history-unexercised-affected-race-ff78ebd6.log`, exact census `scratch/history-cli-affected-results.json`), both corrected raw-editor admission controls pass (`scratch/raw-editor-native-admission-restored.json`), and unchanged native exact-path cmd-history-persist, cmd-history-persist-command and cmd-history-max all pass (`job-ci-history-native-editor-exact-paths-618cff67.log`). These do not establish the original disk-backed native force/verify, complete SSH population, load/coverage or whole-spec green.

Current daemon source captures the real freshly constructed API server, immediately owns it with sync.OnceFunc plus defer, and uses the same owner at all six explicit stop sites. setupInfraHook registers actual SSH Stop callbacks with that server's existing OnStop and publishes a private constructor-admission gate/seal first. Registration refused after Stop has taken its unordered callbacks synchronously closes/joins the just-built SSH server before the gate unlocks; the seal therefore joins in-flight construction regardless of callback order, and later calls refuse before building. No new manager/list/registry, engine/config feature enumeration or generic Server.Stop change was added. The always-on sshServer seam now requires Stop(context.Context); the exact DD4C auth fixture change was independently approved category (a), unchanged Test units/no native approval command appropriate, and adds a BUG panic rather than no-op cleanup.

Concrete stop-caller census found only main's shared OnceFunc for the captured hub API. Reactor abortStartup and cleanup stop API only under !externalServer, while the actual production factory calls CreateReactor(..., false). Coordinator/adapter Stop delegates to that reactor; registry.PluginServerAccessor exposes no Stop. REST/gRPC/gNMI/web cleanup wrappers close their own concrete transport servers, not this API; RPC/SSH lifecycle requests only signal the daemon wait loop. The actual standard-library OnceFunc→Once.Do producer guarantees every concurrent caller waits for the one operation to finish.

The startup-failure regression retains every actual transport/Model/store assertion through a shared fixture; `TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown` adds the real accepted shutdown request and normal exit-0 path with the same exact held-write/store-release proof. New/affected SSH/hub runtime proof, local constructor refusal, citation inventory and full native force/verify/load remain owed to Main. No post-source runtime pass is fabricated here.

Main's final `job-ci-history-final-owned-runtime-37b38b45.log` actually passes all 33 selected SSH roots under race, including every new listener/model owner control and real PTY/no-PTY sessions. The subsequent hub/config packages did not run: shared Go cache entries disappeared during vet/import resolution. Cache-device capacity is available; the documented missing-entry recovery is one rebuild of only those two failed packages, not a rerun of passing SSH.

Final independent fix-only lifetime and behavior reviews found no remaining confirmed blocker in the daemon OnStop/OnceFunc owner, unordered constructor seal, per-channel drain or listener admission. Their source verdicts are separate from the still-pending hub runtime result. The spec remains open; original disk-backed/load/coverage and final closure evidence are not claimed.

The documented single cache recovery completed with exit 0 in `job-ci-history-hub-owner-cache-recovery-25969fc6.log`: both real daemon startup-failure/normal-shutdown history-owner regressions pass under race, as do the selected infrastructure/authentication/accounting/lifecycle consumers and explicit loose/backup constructor-refusal case. Main read the full 91-line output. This completes the bounded patch's runtime proof, not AC-7's original disk-backed stress/load or formal spec closure.

Implementation landed as `aa34d659d7`. A single native run of the current draft counterparts passes both force/verify cases (`job-ci-history-original-force-verify-named-874110ea.log`). The separate live-carrier run `job-ci-history-original-live-force-verify-c26c4bee.log` still fails: force's step 16 waits for “force commits over warnings, never errors”; verify's step 18 waits for the prior role/filter diagnostic. Both expire their output deadline. This is unresolved live-CI failure; neither the draft pass nor the committed UI fix closes AC-7.




## Design and implementation review history

### Design admission

Main admits the session-scoped design after independently reading `syncutil.Coalescer` and checking its contract against every positive AC and permitted survivor: its single worker excludes same-key overlap, Enqueue replaces waiting work without storage waits, and Stop drains/joins without an Update or dispatch context. The History integration must admit only the two real modes and transfer detached immutable values; the coalescer alone does not provide those constraints. The source census covers all three Enter branches and copied-model, SSH, attached, local and headless owners. AC-1’s old-API-compatible held-writer regression must actually fail before implementation; AC-2 through AC-6 require ordering, bound, mode, owner and useful-persistence proof. AC-7 remains a separate disk-backed native obligation: neither the captured stack nor a unit pass proves every timeout repaired. Existing-test completion-boundary edits require independent judgment before application. This is the main thread’s harness-model admission, not a fabricated automated check.

| Run | Scope | Artifact | Result |
|-----|-------|----------|--------|

### Implementation review

| Run | Scope | Artifact | Result |
|-----|-------|----------|--------|

## Checklist

- [x] Main records actual pre-fix red of the focused regression before any product edit.
- [x] Main independently judges existing immediate-persist/completion-boundary test changes before they are applied (independent judge and all eight native receipts recorded before edits).
- [x] All ACs and useful end-to-end stories demonstrated through executed scoped tests, with the approved deviations recorded below.
- [x] Every model owner and Save caller migrated, no unused parallel mechanism retained.
- [x] Named docs/wiki and source anchors updated with the behavior change.
- [x] Independent Review Gate clean, with 38 current file hashes checked.
- [x] Closure sections appended on entry. The paired audit/removal commits use one native script.

---

## Implementation Summary

### What Was Implemented
- `aa34d659d7` moved the three Enter history saves to one lazy session worker, using the existing keyed coalescer and detached bounded snapshots.
- `Model.Close`, the attached/local owners, headless cleanup, SSH channel ownership and daemon stop ownership drain accepted history before borrowed resources close.
- SSH startup now waits for actual listener registration. Stop seals model admission and joins each admitted channel or factory, including network shutdown failure.
- `fa795bf224` makes the session observer wait for output within the original case lifetime. Legacy PTY callers and quit drains retain their 20-second bounds.
- `35fdc9b9f0` and `4603e2da52` publish the independently judged native regressions after their loaded proofs.

### Bugs Found/Fixed
- `TestModelHistoryBlockedWriterKeepsUpdateProgress` failed before the fix at all three useful-progress assertions, then passed under race.
- The SSH stop, shared-channel and daemon startup regressions each had an observed causal red against their old producer, then passed after the owner repairs.
- The listener-registration race exposed by the SSH race run is covered by the actual Serve and immediate Start/Stop tests.
- Native force/verify expected an error from a role input that the current plugin correctly accepts. Their judged replacements drive a real mandatory-member error and retain refusal, warning, pending/running and committed-state proof.
- Closure found missing entry-point proof of the hook constructor seal. `dec7fc60c7` adds both guard regressions. Their baseline race/count runs pass, and all three targeted source cuts fail at the intended assertions.

### Documentation Updates
- `docs/features/cli-commands.md`, Command history persistence, and `docs/guide/cli.md`, Interactive features, describe background publication and graceful drain. Their anchors name `History`, `Save`, `Close` and the actual SSH/hub owners.
- `docs/guide/config-editor.md`, Usage, names the copied-model, channel, listener and hook lifetimes with anchors to all affected producers. `ai/digests/cli-editor.md` and `ai/digests/hub-engine.md` match those owners. These edits landed in `aa34d659d7`.
- Wiki commit `ae2fbe6` updates the Interactive features paragraph and regenerated `llms-full.txt`. It preserves the abrupt-death limitation and separate config durability.
- `docs/functional-tests.md` anchors the observer lifetime to `sessionEditorRun`, `sessionEditorWaitFor`, both PTY helpers and `runOrchestrated` (`fa795bf224`).
- Scoped source-anchor and producer checks were performed. No aggregate documentation, repository or full verification result is claimed. The owner's scoped-evidence rule takes precedence over this skill's whole-tree command.

### Deviations from Plan
- Actual owner testing required the SSH generation/channel barrier, real listener-registration handshake and retained daemon hook owner. The original history worker alone could leave a disconnected model writing after store release.
- AC-7's original input and timing wording did not survive unchanged. Independent judgments approved obsolete force/verify input corrections. The owner authorized following completion within the original native case timeout, recorded verbatim in the handover. No case timeout or storage durability barrier was changed.
- Replay assertions now sit under their actual producing commands. The graph fixture includes mandatory implicit owner edges and still checks every explicit grant. Route Refresh uses an independently capable peer so its negotiated result actually changes. These are judged test repairs, not new protocol implementation.
- Coverage is measured, not asserted complete. The original count selection executed 73 of 106 blocks intersecting production additions. It does not prove every Boolean branch or cleanup outcome.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The initial live force/verify inputs expected an obsolete role refusal. | The role plugin inserts its mandatory checks. Explicitly naming a required member supplies the real error case. | Live failures and `requiredInsertionRefusal` source inspection | Preserve refusal and state assertions with independently judged inputs. |
| approach | A separate observer deadline expired during required durable writes. | Completion can legitimately follow that observer limit while the original case is still live. | Owned daemon stacks and identified syscall trace | Follow caller lifetime after the owner's authorization. Keep durable syncs and case bounds. |
| approach | Ordinary daemon shutdown tests left the constructor/stop guards unproved. | Those guards need a construction interleaving through the installed hook. | Independent closure guard audit and zero counters | `dec7fc60c7` adds entry-point proof. Three source cuts produce useful reds. |
| approach | The first hook regression used synctest quiescence while actual Stop waited on a mutex. | Go does not classify mutex contention as durable blocking. | Main captured the owned stalled test stack and read the standard library contract. | Correct only the test's scheduling observation. Keep actual OnStop refusal and every owner-order assertion. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove history I/O from serial Update | Done | `history.go`, `Save` and `publish`; `model_keys.go`, `handleEnter` | All three Save sites queue detached snapshots. |
| Preserve useful history, bounded work and session order | Done | `History.Save`, `EditorMode.String`, `Coalescer.Enqueue/run` | Two real keys, one worker, newest waiting snapshot per key. |
| Graceful owner shutdown before borrowed resource release | Done | `Model.Close`, local/headless owners, SSH `Stop`, hub `setupInfraHook` | Existing real owner proofs and new constructor/stop guard discrimination pass. |
| Reuse existing scheduler | Done | `internal/core/syncutil/coalescer.go` | No queue, per-command goroutine or global history authority was added. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestModelHistoryBlockedWriterKeepsUpdateProgress`: all three subcases PASS | The actual pre-fix red failed useful input, timer and delivered-result progress while the writer was held. |
| AC-2 | Done | `TestModelHistoryModeSnapshotsDrainInOrder`, `TestHistoryModeProjectionBoundsPublicationKeys`: PASS | 129 real accepts, exact three-write sequence, immediate browsing and real reload. The alias mutant was observed red before restored Clone passed. |
| AC-3 | Done | `TestModelHistoryAutoConfigAttribution`, `TestHistorySessionsKeepIndependentSnapshots`: PASS | Exact config/run/format attribution and last-whole-snapshot semantics. |
| AC-4 | Done | `TestModelHistoryCloseDrainsCopiedOwner`, replacement and headless owner roots: PASS | Old model copy drains independently of unapplied results. Borrowed store remains usable. |
| AC-5 | Done | Attached/local/headless, authenticated SSH, channel, shutdown, restart, factory, actual daemon and hook-guard roots: PASS | The new tests discriminate omitted cleanup, detached cleanup and post-stop construction. |
| AC-6 | Done | Existing roundtrip/max/per-mode tests, boundary roots and unchanged restart stories: PASS | Default/custom/clamped maxima, dedup, normalized bytes and nil-store behavior preserved. |
| AC-7 | Changed, proof executed | Both judged force/verify cases passed 80 loaded invocations and later normal live runs. Three unchanged editor restart cases passed. | Approved input/observer deviations are above. Earlier unrepeatable startup timeouts remain disclosed below, with attempts and next step. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Blocked-writer useful progress | Done | `model_history_blocked_test.go` | All three old-API subcases observed red then race PASS. |
| Mode snapshots, copied owner, idle/nil worker, replacement, auto-config, independent sessions, maxima and enum projection | Done | `model_history_async_test.go` | All eight named roots PASS. No useful payload assertion removed. |
| Existing history roundtrip/max/per-mode/nil/navigation and both Enter persistence tests | Done | `history_test.go`, `model_test.go` | Judged completion boundaries retain exact reloads. Existing navigation population passed in the affected race census. |
| Attached owner and constructor refusal | Done | `cli/client/history_owner_test.go` | Command/editor owners and normal/TTY/program/cancel outcomes PASS. |
| Local program owner and constructor refusal | Done | `config/cli/history_owner_test.go` | Loose/backup modes, exact reload and owned-store release PASS. |
| Headless owner/restart | Done | `cli/testing/history_owner_test.go` | Both roots PASS. Restart exercises the actual `.et` runner in both modes. |
| Authenticated PTY/no-PTY history drain | Done | `ssh/history_owner_test.go` | Both actual-transport roots PASS. |
| Disconnected stop, transcript order, generation rejection, admitted factory and exec self-join | Done | `ssh/history_shutdown_test.go` | All named roots PASS under race and count instrumentation. |
| Channels sharing one connection | Done | `ssh/history_channel_owner_test.go` | Actual private closer and exact separate publications PASS. |
| Listener admission, early Serve error and immediate Start/Stop | Done | `ssh/listener_start_test.go` | All three roots PASS. Primary and extra listener closure is asserted. |
| Actual daemon startup failure and normal shutdown | Done | `cmd/ze/hub/infra_history_owner_test.go` | Both actual Run/hook/SSH/Model/store roots PASS under race and count instrumentation. |
| Hook constructor seal and refused registration | Done | `hub/infra_hook_owner_test.go` | Both exact roots pass under race/count. All three verified one-file source cuts produce intended assertion failures. |
| Observer matching, caller lifetime, live silent cancellation and EOF/EIO | Done | `fixture/session_editor_wait_test.go`, `session_editor_wait_linux_test.go` | Five roots PASS under race. Existing kill and bounded quit tests PASS. |
| Existing dispatch/timer/session lifetime population | Done | Original affected package tests | Scoped affected race runs passed. No whole-suite claim. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cli/history.go`, `model.go`, `model_keys.go` | Done | Typed immutable publication and shared close ownership. |
| `cli/client/main.go`, `config/cli/cmd_edit.go` | Done | Real local owner defers and pre-construction refusal ordering. |
| `cli/testing/headless.go`, `hub/session_factory.go` | Done | Failed-model editor cleanup. Existing successful owner order retained. |
| `ssh/ssh.go`, `model_lifetime.go`, `listener_start.go` | Done | Generation/channel join and actual listener registration. |
| `hub/infra_setup.go`, `main.go`, `ssh_infra.go` | Done | Actual retained stop owner and compile-out seam. |
| `cli/model_history_blocked_test.go`, `model_history_async_test.go` | Done | Responsive UI, bounded snapshots, mode and close proofs. |
| `cli/history_test.go`, `model_test.go` | Done | Exact approved completion boundaries. |
| `cli/client/history_owner_test.go`, `config/cli/history_owner_test.go`, `cli/testing/history_owner_test.go` | Done | Every planned local/headless owner population exists and passed. |
| `ssh/history_owner_test.go`, `history_shutdown_test.go`, `history_channel_owner_test.go`, `listener_start_test.go` | Done | Every planned SSH owner/listener population exists and passed. |
| `hub/infra_history_owner_test.go`, `infra_setup_auth_test.go` | Done | Actual daemon ownership and judged seam adaptation. |
| `docs/features/cli-commands.md`, `docs/guide/config-editor.md`, `docs/guide/cli.md` | Done | Timing and owner claims match current source anchors. |
| `ai/digests/cli-editor.md`, `ai/digests/hub-engine.md`, `../wiki/cli.md` | Done | Current flow and user timing contract published. |
| Native observer and promoted plugin/reload files | Done | Committed in `fa795bf224`, `35fdc9b9f0`, `4603e2da52`; exact review population retained by the Review Gate. |

### Audit Summary
- **Total items:** seven acceptance criteria, the Task requirements and every planned test/file group above.
- **Done:** AC-1 through AC-6, with executed AC-7 behavior under the approved test repairs.
- **Partial:** no scope reduction approved or taken.
- **Skipped:** none.
- **Changed:** AC-7's approved test inputs and observer boundary. Its behavior and original case bounds are proved by executed native tests.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Input, timers and delivered results progress while history storage waits | Causal regression | All three `TestModelHistoryBlockedWriterKeepsUpdateProgress` subcases failed at the intended pre-fix assertion, then passed under race. |
| Useful bounded history survives graceful completion in its actual mode | Unit and functional | Exact snapshot/max/reload tests PASS. Native `cmd-history-persist.et`, `cmd-history-persist-command.et`, `cmd-history-max.et`: PASS 3/3. |
| Resource owners join accepted publication before borrowed storage closes | Actual owner integration | Copied/local/headless owners and authenticated SSH/daemon tests PASS. New hook-guard tests pass and fail under all three intended source cuts. |
| Original native commit behavior remains useful on durable storage | Loaded functional | Five plugin drafts, including force/verify: no failure in 80 invocations with parallel 2, burner 1, timeout 240. Both promoted commit cases subsequently PASS with actual file assertions. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|

No in-scope implementation item remains. Broader original-CI metadata publication is outside this CLI lifecycle spec and remains in the followthrough handover. No full-CI green result is claimed.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-history-blocks-ui-codex-ci41685742426-followthrough-20261105.md` |
| `./le spec review check` | OK: 38 code files, clean, hashes match |
| Rounds | 4 |
| Reviewer lenses used | Logic and wiring; security and edge cases; lifecycle, preservation and test discrimination |

### Run 1 scope

Review the complete history and lifetime implementation in `aa34d659d7`, the observer correction in `fa795bf224`, and the associated native fixture and CI repairs in `35fdc9b9f0` and `4603e2da52`. Compare their actual current producers and tests with AC-1 through AC-7. Foreign shared-checkout changes are not the review population. No full-tree verification is run.

### Run 2 scope and result

Review only the new `infra_hook_owner_test.go` entry-point tests and the existing hook/stop producers they drive. Source review found no changed product behavior or weakened assertion. Main's first execution stalled before the new proof: synctest waited while actual Stop was blocked on `builders`' mutex. An owned SIGQUIT supplied that exact test stack. This is a test scheduling defect, not an observed product red. The proof remains open.

### Run 3 scope

Review only the hook test's scheduling correction and its cleanup/order observations. Preserve actual `infra.Run`, actual owner stop/registration refusal, held-constructor and held-resource checks, exact cleanup counts, and first post-stop refusal. Run only the corrected roots, their focused count profile and discrimination controls.

The first corrected run reached the test's return, then failed in deferred context cancellation. The observer deadline's Done channel was first materialized inside synctest, so its outer cancel tried to close a bubble-owned channel. This is another fixture ownership error, not a product assertion failure.

### Run 4 scope

Review the observer context's ownership correction: materialize its Done channel before entering synctest, then preserve the actual registration-refusal condition, cleanup joins and original assertions. Re-run only the two corrected roots and their discrimination controls. No new product code or broader test population is introduced.

Final result: 0 BLOCKER, 0 ISSUE. Both roots pass race/count. All three verified source cuts fail their intended assertions. The final canonical control passes both roots with GOFLAGS unset. The changed-Go style pass covered peer reachability of panic, bounds, names, paired lifetime obligations, deliberate copies, valid state and return shape. The new invariant panic is confined to fresh-owner setup, not peer input. Scoped wiring/source-anchor checks and exact approval trailers were checked. Aggregate repository/documentation checks and a weakened audit spanning foreign shared commits were not run under the owner's scoped-evidence rule.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The constructor seal and rejected late stop registration lacked entry-point discrimination tests. Both new guards protect AC-5. | `cmd/ze/hub/infra_setup.go`, `setupInfraHook` | `dec7fc60c7`: both new roots pass under race/count. Omitted cleanup, detached cleanup and removed admission guard each produce their intended assertion failure. |

### Diagnosis for finding 1

1. **Symptom:** the two daemon owner tests pass, but neither drives an infrastructure constructor racing the actual stop-hook snapshot or a first construction attempted after stop.
2. **Root cause:** `setupInfraHook` installs a constructor gate and seal, then synchronously stops a server whose late `owner.OnStop` registration is refused. Existing tests construct SSH before stop starts. The count profile confirms these guard outcomes were not executed.
3. **Owning layer:** hub composition. `infra.Run` invokes the installed hook, and the plugin server's `stopHooks.run` marks registration stopped before calling its unordered snapshot.
4. **Alternatives:** [workaround] infer the race contract from ordinary shutdown passes. [source] add deterministic tests through `infra.Run` and the actual plugin-server owner, holding construction across stop admission and holding resource cleanup until the test releases it.
5. **Decision:** use the source proof. A normal shutdown pass cannot distinguish a missing constructor seal or a dropped refused registration. The post-stop case must attempt its first construction, so the separate AAA single-build guard cannot hide a missing admission check.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `model_history_blocked_test.go`, `model_history_async_test.go` | Yes | `ls` returned both canonical CLI paths. |
| Client/config CLI/headless `history_owner_test.go` files | Yes | `ls` returned all three canonical paths. |
| SSH `history_owner_test.go`, `history_shutdown_test.go`, `history_channel_owner_test.go`, `listener_start_test.go` | Yes | `ls` returned all four canonical paths. |
| SSH `model_lifetime.go`, `listener_start.go`; hub `infra_history_owner_test.go` | Yes | `ls` returned all three canonical paths. |
| Three history editor stories and force/verify `.ci` files | Yes | `ls` returned all five canonical paths named in Functional Tests. |
| Hook guard regression | Yes | `cmd/ze/hub/infra_hook_owner_test.go` contains both exact new root names. |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | All three accepted Enter branches progress while storage is held | Count log has PASS for config command, config run and operational format. Source has exactly three `history.Save(m.mode)` calls. |
| AC-2 | Waiting work stays bounded and latest mode snapshots reload exactly | Count log has PASS for both snapshot-order and enum-projection roots. `Coalescer.Enqueue` replaces waiting values and `run` uses one worker. |
| AC-3 | Actual mode attribution and independent-session behavior remain | Auto-config and independent-snapshot roots PASS. `Save` captures the String projection and detached entries before enqueue. |
| AC-4 | Initial model copies drain before store release | Copied-owner, replacement and headless owner roots PASS. `Model.Close` calls dispatch end, history Close, then transcript Close. |
| AC-5 | Every owner discharges the same lifetime | Existing attached/local/headless/SSH/hub roots and both new hook-constructor guards PASS. Three verified source cuts produce useful reds. |
| AC-6 | Exact bounded persistence and useful browsing survive | Count log includes existing roundtrip/max/per-mode tests and exact maxima boundary subcases. Three unchanged native restart stories PASS. |
| AC-7 | Durable native statuses and file assertions pass under load | Both force/verify cases PASS in the five-case 80-invocation run, then from promoted live paths. Native output names both cases and their file-check steps. |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Actual Enter, following rune, declared timer and delivered result | `model_history_blocked_test.go` | Real Update driver asserts all useful progress before writer release, then exact key/payload. |
| Held writer, mode change and old copied owner | `model_history_async_test.go` | Exact three-write sequence, poisoned admission slices and reload assertions remain. |
| Authenticated PTY and no-PTY completion | `ssh/history_owner_test.go` | Real transport and actual registered Model closer, followed by exact reload. |
| Disconnection, overlapping channels, stop/restart and admitted factory | SSH shutdown/channel roots | Actual middleware and model drain. Old generation refusal and synchronous exec bypass are asserted. |
| Immediate primary/extra listener start-stop | `ssh/listener_start_test.go` | Actual Start/Serve/Shutdown, with each bound listener checked closed. |
| Actual daemon startup refusal and accepted normal shutdown | `hub/infra_history_owner_test.go` | Real Run and installed hook construct authenticated SSH. Store Close captures exact accepted history. |
| Constructor racing actual stop and first construction after stop | `hub/infra_hook_owner_test.go` | Final source review clean. Race/count baselines PASS. Actual source cuts fail on omitted cleanup, detached cleanup and post-stop work. |
| Attached/local terminal outcome | Client/config CLI owner roots | Real Model/Enter and production owner, with borrowed runner results and exact reload. |
| Headless restart | Three `test/editor/commands/cmd-history-*.et` files | Read actual history-store options, restart commands and exact Up/Down expectations. Native PASS 3/3. |
| Durable warnings, errors, force and confirmed commit | `test/plugin/session-editor-commit-force.ci`, `session-editor-commit-verify.ci` | Real SSH editor and persisted-file assertions. Judged inputs retain warning/error and pending/running-state proof. |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Confirmed | Existing best-effort `WriteFile` error treatment is unchanged. History docs now state publication timing separately from result delivery. |
| A-2 | Confirmed | Separate model factories still construct per-session History. The same-user snapshot test asserts replacement instead of merge. |
| A-3 | Confirmed | Actual `EditorMode.String` yields config or operational. Both-mode saturation and enum-cast projection tests PASS. |
| A-4 | Confirmed | Actual daemon stacks and identified syscall trace show independent required draft/commit syncs. The authorized observer waits within the unchanged case bound. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Background publication and abrupt-death limitation | `History.Save/publish/Close`, `Coalescer.Enqueue/run/Stop` | Feature, CLI/editor guide and wiki claims match. |
| Shared model and local/headless resource ordering | `Model.Close`, `runInteractiveWithProgram`, `runEditorProgramWith`, `headlessModel.close` | Editor guide anchors and living digest match. |
| SSH generation/channel ownership and listener registration | `sessionCloseMiddleware`, `Stop`, `modelSessions`, `modelSession`, `registeredListener.Accept`, `serveListener` | New claims match actual vendor trackListener/Accept/Shutdown order. |
| Retained hook-built SSH | `setupInfraHook`, `runYANGConfig`, actual `stopHooks.run` | Source claim supported and finding 1's missing guard discrimination is repaired. |
| Native observer and quit drain | `sessionEditorRun`, `sessionEditorWaitFor`, `readPTYUntilContext04`, `readPTYUntil04`, `runOrchestrated` | Original case owns process cancellation. Matching/error rules and separate legacy/quit bound remain. |
| Unchanged syntax, commands, API/SDK, wire and storage format | Owned diffs contain no schema, declaration, payload or key-format changes. `NewHistory`, `Append` and `publish` preserve registered keys and newline bytes. | No matching category update required. |
| Feature level, comparison, metrics and doctor dependencies | No owned declaration, capability, telemetry or runtime dependency was added. | No support expansion or new readiness prerequisite claimed. |
| Source-header anchors | Checked owned changed-file references in `docs/`, with `docs/guide/config-editor.md` as positive control. Schema, syntax, store-format and core-composition claims are unchanged. | Scoped source-aware check only. No aggregate doc/repository gate claimed. |

### Executed evidence and limits

Main's native `job run label ci-history-owned-coverage` ran the selected history/owner roots across `internal/component/cli`, `cli/client`, `cli/testing`, `config/cli`, `ssh` and `cmd/ze/hub`, with registered feature tags, `-p=1`, `-count=1`, `-timeout=8m` and count instrumentation. All six packages passed: 37 root instances, 36 distinct names, no skips. The profile repeats blocks once per test binary. Summing those duplicates yields 73 nonzero blocks of 106 intersecting the original production additions. This is scoped block coverage, not complete branch coverage. The new constructor proof has its own focused run.

The original profile leaves nil-history Close, several constructor/error cleanup outcomes, hook guard outcomes, five explicit hub error-stop sites, actual terminal adapters, and primary/extra Start error branches unexecuted. Existing real native stories exercise child processes outside this unit profile. Finding 1 specifically addresses the new hook admission guards. No exemption or 100-percent claim is inferred for the other zeros.

The focused `ci-history-hook-owner-count` run passed both new roots with `-covermode=count` and `-coverpkg=./cmd/ze/hub`. Its actual counters are 1 for the stopped-admission return, 1 for refused-registration synchronous cleanup, 1 for the cleanup body and 2 for the seal callback. The setup invariant panic and nil-constructor return remain zero. The corresponding focused race run passed both roots without skips or a race report.

Main ran three verified one-file diagnostic overlays through native jobs, without changing canonical source. Omitting refused cleanup failed `TestInfraHookJoinsRefusedSSHOwner` with “refused SSH owner was not stopped.” Detaching cleanup failed that same root with “infra.Run during SSH cleanup returned before its owned cleanup joined.” Removing the post-stop guard failed `TestInfraHookRefusesFirstConstructionAfterStop` with both live-user resolutions and SSH constructions equal to 1 instead of 0. These are executed assertion reds, not compiler or setup failures. Canonical hook and test hashes stayed unchanged throughout.

The five-plugin native stress selection ran 80 invocations with parallel 2, one burner, timeout 240 and a 25-minute limit. It reported no reproduction, with the native any-failure mode's expected exit 1. The Route Refresh draft independently passed the same 80-invocation proof. Every promoted case has a live pass. Required durable syncs and disk backing remain.

Earlier normal and traced live force/verify runs timed out at startup without enough stack evidence to identify the producer. The SIGQUIT diagnostic deliberately ended its owned daemons and is not a product failure. Two later normal saved-output pairs passed, including `ci-history-startup-causal-capture`, without signals. The latest capture first encountered an unrelated shared inventory build cutover, then actually ran after its source completed. No general startup or disk-latency fix is claimed. If the startup red recurs, preserve native `-s` raw streams and capture the ownership-verified daemon/client stacks before the original hard case deadline.

The scoped race populations and three editor restart stories passed. The observer's focused race and live-silent-cancel roots passed. Committed-source checks for `fa795bf224` and `35fdc9b9f0` passed distro, appliance, setup, host and installer. No full-CI result or push is claimed.

The standalone hook-test commit `dec7fc60c7` also passed the required committed-source check in all five flavors. Closure adds prose and spec removal only. No post-closure verification is run.

The final canonical two-root control actually ran through `ci-history-hook-canonical-control` with GOFLAGS explicitly unset and passed without skips. The review record pins all 38 reviewable files from the complete Spec-work population, including `infra_hook_owner_test.go`. Native `spec review check` returned clean with matching hashes. The separate RFC metadata holds are not carried by the closure.
