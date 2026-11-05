# Filter wire umbrella handover

## Goal and owner instructions

Finish `spec-filter-wire-0-umbrella`, including its Review Gate and formal closure. **The umbrella is not closed.**

The owner stopped this session and requested this handover. Do not resume the implementation merely to finish this document. The owner explicitly objected to hours without visible progress: the next session should publish independently verified fixes promptly rather than hold them behind an expanding audit.

- Preserve all concurrent work. Do not undo, discard or overwrite another session's changes.
- No push was authorized.
- The owner supplied and accepted Mac benchmark results. No before/after comparison or further Mac run is currently needed.
- All session-owned running jobs and agents were cancelled. Shared LSP and other sessions' services were left untouched.

## Session and evidence locations

- Previous session: `01a1a5be-a56f-734c-beaf-07dd6a1d224a`.
- Native commit namespace: `e7d8704b`.
- In this document, **S** means `tmp/session/2026-11-04-111704`.
- Detailed state: `S/state/session-state-filter-wire-0-umbrella-01a1a5be-a56f-734c-beaf-07dd6a1d224a.md`.
- The detailed state's active-job notes are stale. **This handover supersedes them.**
- The umbrella claim was not released. Transfer ownership through the native claim route before proceeding.
- Source and test snapshots in manifests may precede formatting. Refresh pins for actual final contents without inventing another semantic audit.

## Committed work

These six commits were published after the owner asked for visible progress:

| Commit | Change |
|---|---|
| `18c9181e4c` | Default/stale writer boundaries, both-transport regression tests and docs |
| `32c04b56aa` | Inherited role imports in upstream checks |
| `591845be16` | Daemon-owned reject-ASN condition counters |
| `48af87f49e` | Retention verdicts bounded by owned peer lifetimes |
| `e5da45b0a2` | Original policy decision retained on generated follow withdrawals |
| `4b6aa0c719` | Both SDK startup examples answer the protocol version |

Scoped tests passed before publication. Required native committed-tree compilation passed in every flavor at both `18c9181e4c51` and `4b6aa0c71958`:

- `S/scratch/job-fw-writer-boundaries-committed-e6594260.log`
- `S/scratch/job-fw-finished-repairs-batch-committed-e6594260.log`

Earlier published repairs include `66ee5db01359`, `1d9bb0050a`, `3c71a346ec`, `47d95f9a63`, `19237e47d8`, `2e9a891ed7`, `ee4de6e09c`, `ee9eca67e5` and `942ca06d90`. Their exact scope/evidence is recorded in the detailed session state. Do not repeat their checks merely to reconfirm them.

## Interrupted verification: use the completed results

The last run, `bg_221`, was cancelled at the owner's request. Its log is:

`S/scratch/job-fw-closure-new-repairs-green-terminal-debt-red-23da58d9.log`

**The selected `internal/component/plugin/server` tests completed PASS before cancellation.** The complete package result is at log lines 128–129. Observed GREEN:

- Metrics UTF-8 label identity and deletion arity, all three vectors.
- Label admission, churn, restoration/recreation, full-capacity replacement, encoded batch boundary and declaration representability.
- Private/public inactive-intent diffs and metadata-only reconcile, including all six ancestor cases.
- Real startup-publication compensation: activation failure, actual SDK verifier refusal and successful activation.
- Stage-1 declaration readiness: allocated/unknown, completed-empty/known, completed-filter/known.

This is **not** a green result for the entire interrupted job. Results for the remaining packages were not observed. Do not rerun the completed server checks just for confirmation. Resume the unrun scoped packages/tests; the old runtime command can be recovered from the previous session if needed.

The interrupted run selected 55 exact tests plus the existing `TestLoadConfig*` family, with race, `-p 1`, `-timeout 8m`, and the canonical 35 feature tags plus core. It reached plugin/server, plugin, BGP/config, config, config/cli, cmd/ze/hub, BGP, RS, plugin/process, reactor, Adj-RIB-In and RIB. The new U18 test was deliberately expected to go RED against unchanged production, but it had not run before cancellation.

## Uncommitted: verified metrics repair — publish this first

Owned files:

- `pkg/plugin/sdk/metrics.go`
- `internal/component/plugin/server/dispatch_metrics_publisher_regression_test.go`
- `internal/component/plugin/server/dispatch_metrics_publisher_aggregate_test.go`
- `internal/component/plugin/server/dispatch_metrics_publisher_identity_test.go`
- **Only the forked-publisher subsection** of `docs/plugin-development/metrics.md`; daemon ownership prose was already committed in `591845be16`.

The independent final source/test/doc review was CLEAN. The patch covers bounded deletion tracking, label admission, frame chunking with ACK-prefix restoration, finite aggregate reservations, UTF-8 identity, deletion arity and capacity-before-allocation. Main's final allocation-order edit moved capacity refusal before cloned labels and both histogram arrays; existing-key lookup and scalar behavior are unchanged.

The eight current metrics regression groups passed in the interrupted run. Earlier finite aggregate/failed-restore twelve-case GREEN is in `S/scratch/job-fw-closure-authority-identity-red-consumer-green-aac6c0f7.log`; the arithmetic/reservation logic was unchanged by the later UTF-8 and allocation-order edits.

Use native selective hunks where necessary; do not commit every dirty shared document wholesale.

## Uncommitted: authority/configuration patch

Implemented and frozen; independently source-reviewed without a confirmed defect.

- Exact 53-path ownership/hunks/exclusions: `S/scratch/fw-reactor-lifecycle-owned-manifest-v2.json`.
- Review: `S/scratch/fw-final-review-authority.json`.
- Includes inactive running-map fidelity, active-only public surfaces, captured terminal recipients, phase/process/bridge-owned configuration, whole-map construction, atomic Attach/expected-Detach and early existing compensation enrollment.
- Removes `BootPeersFunc`, bootstrap-byte authority and obsolete `SetReactor` callers, without a fallback/shim.
- Factory, coordinator, removal, normal-loader and other current-source runtime checks still need completion. The server compensation/private-diff subset just passed as described above.

Every exact old-test migration was independently judged. Native approval records for several of those decisions remain queued. Do not reissue the judgments; record them through the existing approval route under the new session as required by the gate.

The previously refused storage fixture is corrected: each candidate/accepted iteration freshly runs `ReadReloadConfig` then `ReadConfig` before supplying the selected map to the factory. It retains the original loose/boot bytes, promotion and both peer assertions. Actual promotion mirrors accepted bytes to the config file. The factory must not regain storage authority.

**Evidence gap:** the pre-change semantic-reference census for removed `BGPBootstrap.ConfigData` was missed. It is explicitly **UNOBSERVED**, not retroactively completed. A current ten-reference semantic census and every current consumer were inspected; log `S/scratch/job-fw-current-bootstrap-semantic-references-4b2994ff.log`. Preserve that distinction.

## Uncommitted: UPDATE forwarding receipts

Implemented/frozen. Current mandatory-field GREEN remains unobserved.

- Preserves opaque generic envelope metadata and snapshots forwarding receipts.
- Rejects missing/null peer, message ID, policy epoch and forwarding decision; explicit zero/false remain valid.
- Uses value receipts with explicit envelope presence, rather than per-field pointers or a returned receipt pointer.
- Main files include `internal/component/bgp/types/update_receipt.go`, `update_receipt_scalar.go`, `internal/component/bgp/server/event_update_receipt.go`, shared BGP parsing and RS/RR consumers.
- New regression: `TestRSFastReceiptRequiresExplicitFields` in `internal/component/bgp/plugins/rs/fastpath_receipt_required_test.go`.
- Earlier real direct/forked RS exactly-once and shared-parser identity tests passed before the mandatory-scalar/value-API change. Those changed paths need current verification.

Preserve retention and authority hunks in shared RR/server files. The old RS constructor migration already received an independent native approval.

## Uncommitted: declaration readiness and forked command roots

Main changed only `Server.declaredFilters`' readiness guard in `internal/component/plugin/server/server.go`:

`reg == nil` became `reg == nil || !reg.Done`.

An allocated process has a non-nil empty registration before Stage 1. Treating it as an authoritative empty declaration incorrectly refused valid built-in community insertions. This is incomplete validation data, not an excuse to bypass name/direction/read/set checks or add a static-name fallback. The guard is independently source-reviewed and its three controls just passed. It is **not yet proven through the real daemon**.

The existing untracked `server_declared_filters_test.go` was preserved unchanged, not authored here. Attribute it honestly if carried.

Before this fix, native forked reject-ASN draft 33 failed on `bgp-filter-community:blackhole-agreement` **before its policy-root subject**:

`S/scratch/job-fw-path-asn-forked-roots-red-dd97f453.log`

Therefore the forked command's `WantsConfig` root delivery remains original and must not be changed until the native case actually reaches/reproduces its intended subject.

## Uncommitted: event framing and unresolved UClose18

Eleven Go files plus three restricted documentation areas are implemented/frozen. Exact source population:

- `pkg/plugin/rpc/event_frame_size.go`
- `internal/core/bgp/ribevents/validation.go`
- `internal/component/bgp/plugins/adj_rib_in/{rib.go,rib_eligibility.go,rib_commands.go,rib_validation_config.go,source_policy.go,rib_validation_burst_test.go}`
- `internal/component/bgp/plugins/rib/{rib_event_sdk.go,rib_event_burst_test.go,rib_validation_burst_recovery_test.go}`

Restricted docs: event framing/validation handoff in `docs/architecture/api/process-protocol.md`, validation/publication sections of `docs/architecture/plugin/rib-storage-design.md`, and the event-copy-trigger row in `docs/architecture/buffer-architecture.md`.

The patch replaces `FitsFrames` with closed-schema `CanonicalPayloadBudget`, preserves small-path single marshals, encodes large batches once, retains metadata/order and transfers owning strings. Original frame limits, workloads and deadlines remain. Independent review found no additional confirmed framing, ownership or ACK-prefix defect. New framing, 170k validation and 180k recovery GREEN is still owed.

Review: `S/scratch/fw-final-review-event-burst.json`. It is **not CLEAN because U18 remains open**.

### UClose18: no production fix written

After a failed validation publication, automatic emission pauses but ordinary distinct announce/withdraw churn still grows retained owed keys indefinitely. Repeated-key coalescing is not a bound.

Ready, unrun first-RED test:

```sh
CGO_ENABLED=1 ./le --name session job run label validation-publisher-debt-red quiet command go test -race -p 1 -timeout 8m -run '^TestValidationPublisherFailureFencesDistinctReceiveChurn$' -v ./internal/component/bgp/plugins/adj_rib_in
```

Use the canonical feature tags if the environment requires them, as in the interrupted run. The file is `internal/component/bgp/plugins/adj_rib_in/rib_validation_failure_churn_test.go` (149 lines). It reaches the actual registered plugin, SDK publication and bridge input: primes a real ACK/eligible route, refuses its withdrawal, churns distinct identities and requires bounded/fenced intake, failure status, closed transport and no reopened validation. Its production baseline has not been exercised yet.

Blind SDK Close is not a complete fix:

- A nil validation lookup fails open for non-FlowSpec routes.
- SDK Close is nonjoining/self-handler-safe, but Run may return nil; the owner must preserve/report a terminal cause after drain.
- Existing atomic `RegisterValidationLookup` can install a non-manager false provider in one store. Terminal cleanup must not erase it with nil.
- SDK runtime `FailureFatal` already exists; it cannot be softened by respawn-ignore. No registry-policy field or pure-query extension is required.
- Ordinary fatal shutdown reaches the BGP owner's `Stop`/`Wait`, not GR restart. Actual process-manager to live-peer teardown proof is still required, not a shutdown-hook echo.
- **Intentional removal race is unresolved:** supervisor suppresses failure while removal is pending. Existing removal `done/err` records actual SendBye outcome, but recovery can retire/replace the generation before a watcher classifies the failure. Successful Bye must exclude later publication faults; failed/disconnected fatal-owner removal must be classified before losing that owner.
- Latest design candidate reuses existing validation publication serialization across mutation/publication/OnBye settlement, and existing removal outcome state. It is not production-authorized as complete. Do not add a cap, drop owed withdrawals, introduce a retry worker/new protocol, or assume Close alone is fail-closed.
- Declaring the receive-store owner fatal changes availability: **any unexpected current owner exit becomes fatal**, not just publisher failure. State this plainly if that design is adopted.

## Functional tests and final closure still owed

Both unchanged live community reload tests genuinely failed because config2 omitted `bgp-filter-community:egress`, retaining community 65000:1 instead of 65000:2:

- `S/scratch/job-fw-community-live-reload-red-0920ea77.log`
- `S/scratch/job-fw-export-next-update-live-red-7da41381.log`

Corrected drafts preserve all wire assertions and have independent native approvals:

- `test/draft/reload/bgp-filter-community-reload.ci`
- `test/draft/reload/bgp-filter-export-reload-next-update-only.ci`

They owe draft GREEN, required eighty-iteration stress, promotion and scoped live GREEN. The live originals remain untouched.

Also complete native forked-root proof and existing filter-wire workflow smokes: `filter-wire-dryrun`, `filter-wire-set-actions`, `filter-wire-intree-forked`, `filter-wire-undeclared-set`, `filter-wire-irr-two-transports`, and positive `redistribute-declare`.

Historical review covered nine slices, 1,004 live plus 60 removed code paths over 372 commits. Integrate post-frozen `b4b5375625ac...` role/RFC work and refresh the final prospective pins. Then finish native Review Gate and formal closure A+B. Do not claim closure early or release the claim before both closure commits.

## Five concrete next actions

1. Transfer the umbrella claim, then publish the independently reviewed, now-green metrics chunk using only its owned files/hunks.
2. Resume the unrun scoped runtime packages from the interrupted run; do not repeat the completed server checks merely to reconfirm them. Run the untouched U18 test against current production and observe its actual RED before any U18 production edit.
3. Finish the minimal independently admitted U18 ownership fix and prove admission, pending-removal classification and real peer teardown. Do not use the incomplete Close/Fatal proposal as a finished design.
4. Exercise the native declaration-readiness/forked-root workflow and the corrected reload drafts, including stress/promotion, plus the original filter-wire smoke workflows. Commit each verified logical chunk promptly.
5. Integrate exact review/approval/evidence pins, complete remaining closure documentation/security/bookkeeping and execute formal closure A+B.

## Operating constraints

- One CPU-intensive job at a time; main checkout only.
- No `./le verify`, broad suites or worktrees.
- Commit only through `./le commit create`; read generated messages and execute the exact printed script path. Do not supply `Spec-work:` manually; the claim writes it.
- After a generated script carrying Go, run the required native committed-tree compile.
- Preserve shared-file changes through native `hunks` selection; do not stage whole shared files indiscriminately.
- No push authorized; no additional Mac benchmark needed.
- The source was frozen at stop. No implementation, test or verification job should still be running from this session.
