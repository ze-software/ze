# Ze: correctness and recovery review

## Decisions

| Rank | Improvement | Why it comes first | Evidence |
|---|---|---|---|
| 1 | Repair the established SSH special-action permission bypass. | A restricted, authenticated login reached all four privileged callback paths in the experiment. This is a contained defect with a direct correction. | SEC-01, reproduced. SEC-02 is a separate credential-reload finding retained from the completed review. |
| 2 | Preserve downstream route truth through queueing, filtering, and write failure. | A peer can receive the wrong final route state. Other paths lose source identity or count failed writes as send progress. | ROUTE-01 reproduced; ROUTE-05 and ROUTE-06 source-confirmed; ROUTE-04 is an unexecuted reconnect risk. |
| 3 | Give a configuration change one owner and one accepted generation, including restart. | Verification, compensation, durable promotion, and boot do not consistently use the same state. A storage failure reproduced an unreadable active configuration. | CFG-01 through CFG-06; CFG-03 reproduced and CFG-05 is a concurrency hypothesis. |
| 4 | Make queued-byte ownership and overload accounting cover the same memory. | The normal queue can borrow bytes that cache expiry returns. Overflow can allocate bytes outside the utilization measure that controls teardown. | ROUTE-02 and ROUTE-03, source-confirmed. No RSS or exhaustion measurement. |
| 5 | Make one lifecycle owner observe exit, clean a generation, and complete recovery. | Internal exits can miss cleanup, ordinary external exits do not invoke configured respawn, and an incremental launch failure can stop healthy plugins. | LIFECYCLE-1 through LIFECYCLE-3, source-confirmed. |

These are improvement priorities, not five additional frameworks. Several corrections are small. Keep them separate from the configuration and lifecycle work that needs a broader ownership decision.

The final instruction is to include everything already established, but investigate security no further. Existing security findings and evidence remain here. No further security investigation was performed after that instruction.

**Inventory: 17 findings — three reproduced, eleven independently source-checked by the reviewer and Main, one additional reviewer-source-confirmed finding, and two unexecuted hypotheses.** Findings remain open: this commission authorized review artifacts, not product fixes.

## What the architecture needs

The strongest common pattern is that a local guarantee stops before the workflow it is meant to protect ends:

- Plugin verification is ordered before plugin apply, but BGP verification itself installs runtime state.
- The plugin transaction lock ends before the hub finishes the operation an administrator calls a commit.
- An individual pointer write is atomic, but its error does not identify whether durable acceptance already happened.
- A queue preserves FIFO after insertion, but duplicate replacement has already changed chronology.
- A cache retain count names a borrower, but forced eviction can still return its storage.
- A process has an exit signal, but the bridge runtime owner waits on a different signal.

The useful architectural change is to make each guarantee cover its full lifetime. More helper layers will not do that. Define the owner, acceptance point, generation, and failure outcome at these boundaries.

### Configuration ownership

```text
operator intent -> candidate bytes -> hub reload
                                    -> plugin verification/apply
                                    -> provider and subsystem reload
                                    -> listener and system effects
                                    -> durable promotion -> accepted state

file-backed BGP preparation --------> reads candidate/active independently
late compensation ------------------> supplies old tree, but can re-read candidate
boot -------------------------------> chooses authority by storage backend
```

`runReloadContext` in `cmd/ze/hub/main_reload.go` owns the outer operation. `Server.reloadConfig` in `internal/component/plugin/server/reload.go` owns an inner transaction. `createReloadFunc` in `internal/component/bgp/config/loader_create.go` supplies another loading and installation path.

Keep the plugin transaction protocol as a participant mechanism. Put admission, candidate identity, preparation, acceptance, and compensation under the outer owner. Pass prepared old/new generations to participants. Do not ask a compensation operation to rediscover which generation it means.

The durable pointer must answer the same question at runtime and boot: which configuration was accepted? A raw-file mirror cannot be both a best-effort copy during commit and the authority on restart.

### Route ownership

```text
received UPDATE -> ingress filters -> cached update
                                     |-> RS direct writes
                                     |-> destination worker channel
                                     |-> owned overflow copies
                                     -> destination wire

source identity, byte lifetime, operation order, and destination generation
must remain valid across every branch
```

`notifyMessageReceiver`, `TryDispatch`, `dispatchOverflow`, and `fwdBatchHandler` are the important handoffs. Do not replace the buffer-first design. Make the ownership contract survive the existing fast and queued paths.

The destination's final route state is the observable result. A callback firing, a cache reference releasing, or a queue draining is not evidence that the destination received that state. Similarly, successful buffering is not successful flushing.

### Plugin lifecycle ownership

`Manager.spawnProcesses` reuses a shared `ProcessManager`. `Process` owns engine/process execution. `Server.handleSingleProcessCommandsRPC` owns runtime dispatch and reaches `cleanupProcess` on exit.

Those are useful separations when their completion signals agree. Currently they do not agree for every transport and failure path. Use one generation exit result to drive cleanup and the existing bounded restart policy. Readiness must distinguish a running process from a recovered stateful service.

## What to preserve

The review does not support a wholesale architecture rewrite. These mechanisms already address real failure modes:

- Registration keeps domain implementation out of generic engine dispatch. Preserve that dependency direction.
- Candidate creation rejects an existing candidate under the storage guard. Individual filesystem writes use temporary files and replacement.
- The plugin coordinator verifies participants before apply. It filters acknowledgments by transaction and has rollback paths.
- BGP apply journals changes made inside that apply. The defect is that outer compensation can bypass the intended old generation.
- Overflow owns its payload copies. Normal batching is FIFO, byte comparison checks hash collisions, and adjacent attribute merging stops at withdrawals.
- `cleanupProcess` already removes commands and subscriptions, releases cache-consumer accounting, and withdraws plugin-installed routes. Ensure all exits reach it instead of writing a second cleanup system.
- The engine has ordered subsystem startup and reverse shutdown. Keep recovery policy explicit rather than distributing it across every plugin.

The detailed independent reports contain additional protective mechanisms and the exact tests inspected. Their broader observations are retained as reviewer evidence, not silently promoted to runtime proof.

## Finding inventory

Stable IDs match [findings.json](findings.json). Each record contains the trigger, consequence, producer symbols, proposed correction, tradeoff, validation scenario, and independent report reference.

### Route correctness and resource ownership

| ID | Evidence | Finding |
|---|---|---|
| ROUTE-01 | Reproduced | `dispatchOverflow` replaces an earlier identical item in place across intervening operations. `announce → withdraw → announce` becomes `announce → withdraw`; the reverse sequence also ends incorrectly. |
| ROUTE-02 | Source-confirmed | Denial stops mux acquisition but not acceptance of heap-owned overflow. The controller's mux utilization measure excludes that continued growth. |
| ROUTE-03 | Source-confirmed | Gap expiry permits retained entries to be evicted. `evictLocked` returns buffers that normal-channel items can still reference. |
| ROUTE-04 | Hypothesis | Queued work carries a peer, while consumption reads its current session. A sufficiently delayed old item could reach a replacement session with different negotiation. |
| ROUTE-05 | Source-confirmed | An ingress payload rebuild restores MessageID but loses SourceID. ADD-PATH assignment uses SourceID, so modified announcements and unchanged withdrawals can name different paths. |
| ROUTE-06 | Source-confirmed | `flushFwdDirty` resets SendHoldTimer after a failed flush. Subsequent failed direct writes can return the same session to its dirty list. |

ROUTE-02 establishes an accounting hole, not a measured process-memory limit or a measured time to exhaustion. ROUTE-03 establishes the unsafe ownership paths; actual pool-slot reuse while a destination waits remains to be exercised. ROUTE-06 establishes erroneous timer reset; the persistent-session failure schedule remains to be exercised.

### Configuration acceptance and recovery

| ID | Evidence | Finding |
|---|---|---|
| CFG-01 | Source-confirmed | The file-backed BGP verification loader installs redistribution and dynamic groups. A later verification rejection can follow an already-stopped dynamic peer. |
| CFG-02 | Source-confirmed | Outer rollback supplies the old tree, but BGP reload reads the still-staged candidate. The provider and actual peer state can disagree after rejection. |
| CFG-03 | Reproduced | Promotion advances active, fails to clear candidate, then cleanup removes the version active references. Active becomes unreadable. |
| CFG-04 | Source-confirmed | Filesystem boot reads the raw configuration rather than active and omits blob startup's stale-candidate cleanup. Restart can select rejected or stale content. |
| CFG-05 | Hypothesis | A second reload can observe the first request's candidate, fail the inner lock, and clear that candidate. Whole-reload ownership is absent; the two-client schedule is unexecuted. |
| CFG-06 | Source-confirmed, medium severity | Resolver and other system effects occur before promotion. The outer rollback does not restore resolver contents after a later promotion rejection. |

CFG-01 and CFG-02 share a loader problem but describe different failures: verification must not install state, and compensation must apply the generation it was given. Fixing only one leaves the other reachable.

CFG-03 is a storage outcome problem, not just a cleanup error. Once active advances, treating every later failure as “nothing was accepted” is wrong. Garbage collection must also protect every referenced version.

### Plugin lifecycle

| ID | Evidence | Finding |
|---|---|---|
| LIFECYCLE-1 | Source-confirmed | Internal engine exit clears running and closes engineDone but does not cancel the process context. Bridge runtime cleanup waits on that context. |
| LIFECYCLE-2 | Source-confirmed | Ordinary external process exit cancels and cleans up but does not invoke configured respawn. LSP found one production Respawn caller: the rollback restart path. |
| LIFECYCLE-3 | Source-confirmed | `startConfigs` stops the shared manager when an incremental launch fails. A later startup phase can therefore kill earlier infrastructure and still signal startup completion without an error. |

Do not confuse deliberate degraded startup with LIFECYCLE-3. Continuing without an optional failed plugin is a policy. Stopping unrelated healthy generations while taking that path is the defect.

### Security findings already established

No new security investigation is requested or implied by this section.

| ID | Evidence | Finding and limit |
|---|---|---|
| SEC-01 | Reproduced at the SSH/dispatcher boundary | A mutation-denied login reached stop, restart, reboot, and plugin-protocol callbacks. A registered mutation was denied in the same experiment. Callbacks were harmless recorders; no actual daemon stop, reboot, or privileged protocol command was performed. |
| SEC-02 | Reviewer-source-confirmed | The completed reviewer traced MCP credential changes that retain the startup authenticator when the authenticated-mode boolean stays unchanged. No credential-rotation experiment was run. |

For SEC-01, the reviewer also traced ad-hoc protocol admission to reserved trusted-plugin identity. The callback experiment does not demonstrate the subsequent command-escalation chain. That chain remains separately identified as reviewer source evidence.

For SEC-02, Main inspected the constructor, address-only reload, and boolean comparison before security investigation stopped. Main did not independently recheck the token-digest producer. See the retained [security report](reviewers/ReviewSecurity-excluded.json) for the complete evidence and counterevidence.

## Executed experiments

| Experiment | Observed result | What it proves | What it does not prove |
|---|---|---|---|
| Route chronology | Both three-operation cases failed. The existing two-operation ordering control passed. | Production cached forwarding and destination writing produce the wrong final prefix state during held initial sync. | Whole-daemon behavior against an external BGP implementation, every family, and every forwarding branch. |
| Pointer promotion fault | Failure after active advanced, followed by cleanup, left active's version missing. Ordinary promotion passed on filesystem and blob backends. | The filesystem promotion/cleanup sequence can destroy the configuration active names. | Blob fault behavior, a full web commit, reboot recovery, or power-loss behavior. |
| SSH special actions, before investigation stopped | Four special callbacks were reached; the ordinary mutation control was denied. | Real loopback SSH plus the production dispatcher/profile store reaches the callback authorization defect. | Full production authentication backend integration or effects beyond the recorded callbacks. |

Logs: [route chronology](evidence/route-chronology.log), [pointer output excerpt](evidence/pointer-promotion-excerpt.log), and [prior SSH output excerpt](evidence/ssh-excluded-excerpt.log).

The commands returned exit status 1 because the review-only assertions exposed defects. No product test was changed, weakened, or installed. These were focused experiments, not a full verification gate. The passing controls were not rerun.

## How to implement the priorities without creating more machinery

### Established access defect

Make special-action authorization an explicit decision, independent of dispatch success or error spelling. The existing evidence is sufficient to explain the defect. Further security work is outside this review's continuing investigation.

### Route truth

Start with the small, separable corrections: remove nonadjacent duplicate replacement, preserve ingress source identity, and stop resetting send liveness on failed flush. Validate destination state, not queue statistics. Keep generation binding as a distinct reconnect correction with its own forced schedule.

Do not replace all forwarding paths at once. Require the same four facts at each handoff: source identity, destination generation, operation position, and byte ownership. Existing owned overflow and retained RIB handles provide local patterns.

### One accepted configuration generation

Use this order:

1. Acquire whole-operation admission and identify the candidate the request owns.
2. Prepare complete old/new configuration generations without installing them.
3. Apply participant changes and retain enough state for compensation.
4. Define exactly which durable write constitutes acceptance and which effects must precede it.
5. Report post-acceptance cleanup separately from rejection. Make boot load that same accepted generation.

Keep irreversible or best-effort system effects explicit. Either they participate with restoration, or they run after acceptance with a visible best-effort result. Do not describe a mixed state as a successful rollback.

### Owned, bounded backlog

Count outstanding payload storage across channels, overflow, transcodes, heap fallback, and in-flight writes. Cache visibility and buffer lifetime are different questions. Expiry can end lookup eligibility without returning memory still borrowed by a worker.

Choose and expose an overload outcome at the destination boundary. Do not use reclamation underneath borrowers or accounting that excludes the fallback path. Measure fan-out, queue age, retained bytes, and RSS under sustained slow-destination load before tuning throughput.

### Generation-owned plugin recovery

Unify internal and external exit observation. Stop delivery, drain dispatch, run existing cleanup once, then decide whether the existing respawn policy permits replacement. Limit failed incremental startup cleanup to resources acquired by that batch.

For stateful plugins, a successful handshake is not sufficient recovery evidence. Define the snapshot/replay/reconciliation required before readiness. Prove it with an installed route, process failure, replacement, and withdrawal of a route that existed before failure.

## Additional observations and open design questions

These are not extra confirmed defects. They preserve the completed reviewers' findings without turning unexecuted observations into stronger claims.

- **Stateful restart reconstruction:** the lifecycle reviewer traced empty route-server peer/withdrawal maps after construction and future-only subscription installation. Whether a specific restart restores every required prior fact needs a focused replay experiment.
- **Replay readiness under lag:** the reviewer found catch-up watermarks and replay generations, but also an explicit catch-up budget after which incomplete replay can still lead to readiness/EOR. Define what “ready” means when convergence is incomplete; do not merely increase the timeout.
- **Persistence health:** the reviewer sampled shared-store state persistence and BFD sequence persistence. It observed best-effort absence/error handling and a coalesced write interval. No crash or protocol failure was reproduced. Define a recoverable loss window and an observable degraded state before making a stronger durability promise.
- **Parallel lifecycle status:** the reviewer found manager-level running state maintained separately from process running/stage state. Deriving status from the current generation would remove a second authority.
- **Documentation disagreements:** the plugin-manager wiring page describes process spawning more broadly than `Manager.StartAll` performs; config authority and MCP rotation descriptions also exceed the reviewed paths. These are reconciliation items, not reasons to enforce the documentation over the producer. Project documentation was not edited.
- **Test discrimination:** overflow replacement tests do not exercise three-operation chronology through replacement. Congestion tests supply denial and teardown ratios independently. Restart tests call restart directly. Boot-pointer tests exercise a helper rather than filesystem daemon boot. These are specific missing compositions, not evidence that the whole test suite is poor.

The full reports preserve additional inspected tests, protections, proposed experiments, and scope limits: [configuration](reviewers/ReviewConfig.json), [routes](reviewers/ReviewRoutes.json), [lifecycle](reviewers/ReviewLifecycle.json), and [completed access review](reviewers/ReviewSecurity-excluded.json).

## Protocol grounding

The following full RFC text was read. This is targeted grounding, not a complete RFC inventory or interoperability certification.

- RFC 4271, Section 9.2: “All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message.” Source: `rfc/full/rfc4271.txt`. The chronology experiment leaves the final advertised state wrong; it is not a claim that every redundant intermediate update must be sent.
- RFC 7911, Section 2: “However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor.” Section 5: “If a BGP speaker receives a message to withdraw a prefix with a Path Identifier not seen before, it SHOULD silently ignore it.” Source: `rfc/full/rfc7911.txt`.
- RFC 9687, Section 5: “If the local system does not send any BGP messages within the period specified in SendHoldTime, then a NOTIFICATION message with the \"Send Hold Timer Expired\" Error Code MAY be sent and the BGP connection MUST be closed.” Source: `rfc/full/rfc9687.txt`. The defective reset was source-checked; the prolonged connection scenario was not executed.

## Coverage and limits

This is a risk-selected project review, not a complete audit of every component or file. The sampled end-to-end boundaries are configuration reload and recovery, BGP forwarding and ownership, plugin lifecycle, and the already completed administrative-access slice.

Not performed: complete codec/RFC enumeration; external BGP interoperability; Linux/QEMU execution; kernel/FIB reconciliation across all routing protocols; durable-store power-loss testing; all-plugin restart validation; full-suite testing; throughput or allocation benchmarking; or further security investigation.

Main used producing code, independent reports, symbol references, and the three stated experiments. Reviewer-only observations are labeled. No product code, permanent tests, rules, project documentation, or shared index was modified.

Baseline HEAD: `4e220c76675f21ecbf31d8b2a1564850bd7d776c`. [Source fingerprints](evidence/source-sha256.json) cover cited producers; initially fingerprinted route producers did not change by synthesis. Fingerprints identify the reviewed content, not runtime correctness.

See [experiment instructions](experiments/INSTRUCTIONS.md) for commands, overlay limitations, and the distinction between observed and proposed validation. The `excluded` artifact filenames record an earlier temporary scope instruction; the final instruction includes their existing findings.
