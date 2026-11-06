# Handover: full CI 41685742426

**Goal:** Resolve the failures from https://github.com/ze-software/ze/actions/runs/41685742426, pinned at `1a2ccaeef8c0da4dc9f3747ce70f4aeecb84ddfa`. Do not chase newer unrelated shared-checkout failures.
**Branch:** main, shared checkout. No push is authorized.
**History scope:** `spec-cli-history-blocks-ui`. The completed audit is preserved in git by the paired audit/removal closure. The user authorized admission of this named fix only; the one-off claim used cap 23, without changing the permanent cap.
**Outcome:** History closure and the authorized checker/evidence publication are complete. Full-CI green remains unobserved. The earlier unexplained startup timeouts and their completed diagnostic captures are preserved below.

## Publication continuation, 2026-11-05 and 2026-11-06

Thomas authorized this publication turn:

> Commit the pending checkers, align fingerprints with committed source, publish the records

The authorized publication task is complete. All commits have landed, and all 18 audits are fresh against committed source, covering the 17 checker-file references plus RFC2545-3-6.
No owned proof, judgment or metadata publication remains pending. The required working-tree RFC check reported 76 violations before the final updates, so no whole-corpus green result is claimed.
Do not repeat the already-passing checker or writer checks. History closure remains complete, with its earlier evidence preserved below. No push or full-CI green result is claimed.

Checker commit `71b1f3d799` adds strict FRR JSON installation checks and the required mixed-UPDATE/NSSA adapters, with their selected fixtures and contract test.
The two scoped BGP roots passed race in `scratch/job-ci-rfc-checker-predicate-race-9799407e.log`. The final installation-guard root passed race in `scratch/job-ci-rfc-installation-guard-race-final-6aeaadf1.log`.
All five committed build flavors passed in `scratch/job-ci-rfc-checkers-committed-compile-63ed8ad7.log`. Unrelated shared wire-checker rewrites were preserved.
The two previously held RFC2545 proof records were published in `0db1290f22`, preserving their other recorded covers.

The native audit writer initially lacked committed-source selection. `ce17a46ba6` extends the existing reseal and audit-stamp actions with one strict HEAD snapshot and exact requirement selection.
It preserves the working-tree defaults and writes fingerprints through the native readers, without manually replacing hashes.
Independent review was clean after the deleted-producer and assertion corrections. All 24 scoped roots passed in `scratch/job-ci-rfc-head-writer-tests-72e96da1.log`.
The ten new roots passed race in `scratch/job-ci-rfc-head-writer-race-00f135a8.log`, and all five committed build flavors passed in `scratch/job-ci-rfc-writer-committed-compile-63ed8ad7.log`.

Native HEAD reseal refreshed the three original held audits. Two required RFC7606/RFC3101 rejudgments used native HEAD audit-stamp, all pinned to `1875e80c5c3ba3b7837bd9a6798274daf1916ccf`.
The logs are `scratch/job-ci-rfc-held-audits-head-2ee9ad68.log`, `scratch/job-ci-rfc7606-dependent-head-73094511.log` and `scratch/job-ci-rfc3101-dependent-head-4bbab479.log`.
All five audit files were published in `d8472f0d86`, preserving the judged verdicts. Together with `0db1290f22`, this completes the five original holds.

The native RFC2545-3-6 renewal completed with exit 0 in `scratch/job-ci-rfc2545-thirdparty-renew-19021b2f.log`.
The baseline scenario passed. Disabling `linkScope.thirdPartyLinkLocal` produced the intended assertion-4 failure through the injected producer panic, and the native writer accepted that observed red.
Only the third-party unit hash changed, from `ddbb2f9355e3a95a` to `9221fed271c01efb`. The other four recorded entries were preserved.
The broken run reported three Docker removal commands exceeding their 30-second cleanup bounds. Those cleanup warnings are retained caveats, separate from the passing baseline and intended assertion red.
Independent partial rejudgment and native default stamp completed for RFC2545-3-3 and RFC2545-3-6 in `scratch/job-ci-rfc2545-partial-rejudge-43271ac6.log`.
Both verdicts remain partial. Targeted HEAD reseal completed at `d8472f0d8649d66bfddd4bd82e37ff2748ea1779`: RFC2545-3-6 was fresh, RFC2545-3-3 was re-stamped, and zero records were refused.
The result is retained in `scratch/job-ci-rfc2545-partial-head-a2b49650.log`. The three dependent files were published in `7565a5b4b7`.
The executed script was `tmp/commit-2c5d4340-b-1b37c0.sh`, using message `tmp/commit-msg-2c5d4340-b-d0f86b.txt`.
Final native HEAD freshness checked all seven selected audits at `7565a5b4b7ecc7e90254e3a04a00e58871b884c2`, with nothing to reseal and zero refusals.
That result is retained in `scratch/job-ci-rfc-published-head-freshness-b39538e9.log`.
The final discrimination report passed in `scratch/job-ci-rfc-publication-discrimination-66d18b7d.log`, with no stale records and all five target interop covers current.
It still lists the inherited RFC2545-3-2 negative claim in `test/encode/new-v6.ci` as unproven. That claim is outside this publication task and was not represented as renewed evidence.
The required RFC check exited 1 with 76 violations in `scratch/job-ci-rfc-publication-check-f758acb7.log`.
It exposed RFC2545-3-2's stale audit binding to `checkRFC2545NextHops`, changed by `71b1f3d799`. This was an owned consequence omitted from the seven-audit selection above.
Independent `checker_test_judge` rejudged RFC2545-3-2 as enforced, and native HEAD stamp completed in `scratch/job-ci-rfc2545-length-head-rejudge-715ce8a6.log`.
The bounded census is retained at `scratch/rfc2545-checker-selection/check-rfc-audit-census-20261106.json`, comparing all 17 audit references against the parent of `71b1f3d799`, that commit and HEAD.
It found eight newly shifted records, additional checker-file drift in the already-shifted RFC4271-5.1.5-2, and the inherited shifted RFC9234-5-4.
Native exact-eight HEAD reseal re-stamped seven records and refused RFC4271-5.1.2-3, whose unit moved in concurrent `cd0b42213c` after the census.
That latest record was independently rejudged as enforced and stamped in `scratch/job-ci-rfc4271-concurrent-aspath-head-17688197.log`.
The separate exact-two reseal refreshed RFC4271-5.1.5-2 and RFC9234-5-4, with zero refusals.
Both operations used `b4215935acd576856c4ddfb9160d1d67a6860fd8`. Their logs are `scratch/job-ci-rfc-sibling-audits-head-8908b402.log` and `scratch/job-ci-rfc-inherited-sibling-head-961d212a.log`.
All eleven additional audit records landed in `bc121becf6`: nine mechanical reseals and the two independently rejudged enforced audits.
The executed script was `tmp/commit-2c5d4340-b-2cb2e2.sh`, using message `tmp/commit-msg-2c5d4340-b-c09a92.txt`.
Final exact-18 committed-source validation passed at `bc121becf614a805058c6ddabeb16aded510903e`: every audit is fresh, nothing needs resealing, and zero records were refused.
The result is retained in `scratch/job-ci-rfc-complete-checker-audit-bindings-c20cb4af.log`. No test rerun is owed for these metadata-only changes.
Main's read-only Docker check found no remaining containers or networks owned by the proof run, despite its earlier bounded cleanup timeouts.
The 76 check violations have not all been classified as unrelated.
The judge input is `scratch/checker-judge-rfc2545-partial-rejudge.json`. `scratch/rfc2545-checker-selection/partial-audit-consequence-comparison.json` verifies the judged working functions equal HEAD.
The missing-neighbor resolution and automatic re-advertisement gap must remain partial, with no whole-requirement credit.
Earlier controller-build and partial-writer compile failures stopped before tests. The coherent checker and writer sources have the passing evidence above.

## Continuation: captured commit wait, 2026-11-05

The prepared diagnostic now ran. Both forced commit and the corrected route-refresh draft reached required storage barriers when their output waits expired.
CLI history implementation, regression promotion and the independent closure review are complete. The publication continuation above supersedes this section's earlier RFC holds. Full-CI green remains unobserved.
Do not repeat the completed route-refresh source diagnosis. The current state is `tmp/session/2026-11-05-002734/state/session-state-cli-history-blocks-ui-codex-ci41685742426-followthrough-20261105.md`, with earlier chronology preserved below.
The earlier instruction to obtain the first live stack is satisfied. The authorized fixture correction and its completed native proof are recorded below.

Both plugin runs used `./le --name ci41685742426-stallcapture test stress-repro run suite 'plugin --draft' test 'session-editor-commit-force session-editor-commit-verify policy-final-disposition bgp-capture-replay attach-process-delivery-graph' any-failure iterations 80 parallel 2 burners 1 timeout 240 minutes 25`, with the prepared fixture overlay and `CGO_ENABLED=0`.
The plain diagnostic reproduced force step 12 on invocation 9. Its stress log ends `20261105-012203-3191593247.log` under `tmp/stress-repro/`.
`daemon-stacks-1296080553.json`, beside the overlay, captures `runCommitSession` waiting through `NotifyReload` and candidate promotion at the active pointer's destination-directory fsync.
The UI event loop is in `select`, and its history coalescer is idle. This capture locates durable commit work rather than another synchronous history write.

The same native selection, traced through `./le job run label ci41685742426-sync-trace quiet command strace -f -ttt -T -yy -e trace=fsync,fdatasync,renameat,renameat2`, reproduced force step 12 on invocation 3.
Its stress log ends `20261105-012905-3802947951.log`. The traced capture is `daemon-stacks-2145862107.json`, which catches pointer promotion reading a pointer.
The force daemon's identified threads spent 7.905979 seconds syncing the file-commit intent directory, then 10.699060 seconds syncing the explicit-source frame.
Those are serial waits in the forced commit. The trace's longest completed fsyncs, about 18 seconds each, belong to other identified threads.
`readPTYUntil04` bounds the whole expected-output wait at 20 seconds. These are observations from failed invocations, not host performance figures or a whole-commit duration estimate.

`installBytes` syncs each frame before rename and its destination directory afterward. `recoverFileCommit` durably records intent before publishing the explicit source.
No redundant barrier or incorrect ordering has been established, so removing a barrier has no correctness basis.
Thomas replied on 2026-11-05:

> ok you can

This followed the choice between VM storage investigation and a completion wait within the existing case timeout.
Main explicitly interpreted the reply as approval for that completion wait and proceeded under that reading.
This is a one-off override of the earlier prohibition on changing the separate output-wait deadline. The case's 60-second timeout, peer's 50-second timeout and stress settings remain unchanged.
Storage durability and backing remain unchanged.

Separate reload progress has landed in `405f472cd7`: both community reload tests were promoted with their required egress membership preserved.
That commit records independent test judgment, native draft and promoted passes, and 80 successful stress invocations of the pair.
The attempted three-draft run ending `20261105-013407-1358465072.log` stopped at `test "bgp-filter-community-reload" not found`, because that test had been promoted. It supplies no runtime proof and establishes no absence of the remaining drafts.
The live `bgp-route-refresh-enabled-over-ssh` run failed invocation 1 at its 60-second case timeout. Its log ends `20261105-013504-324635378.log`.
That timeout alone did not locate the failed script step. The disposable `route-refresh-pollcapture/overlay.json` records the first failed poll response without changing predicates or deadlines.
The instrumented live single-invocation run, with the same parallel and burner settings, failed step 8 waiting for `"route-refresh":true`. Its log ends `20261105-013845-3149552913.log`.
The actual UI reported `Session committed: 1 change(s) applied and reloaded`. The final peer JSON still reported `negotiation-complete:true`, `route-refresh:false`, one established connection and zero dropped connections.
The wire evidence contained only the initial EOR, with no Cease. The stack is retained at `tmp/session/2026-11-05-002734/scratch/route-refresh-pollcapture/daemon-stacks-3011966463.json`.
Source diagnosis resolved the live test's premise: `peerSettingsSwapPlan` and `negotiationOutcomeUnchanged` correctly keep the session when the peer offered no Route Refresh capability.
Enabling the local offer leaves that negotiation false, as permitted by `docs/guide/config-reload.md`. The live test incorrectly expected a restart and a true result.
The retained draft already adds an independent capability-2 offer from the peer. It predates this session and was hidden by the ignored-file search.
Independent judge `/root/route_refresh_judge` approved that exact retained diff through `tmp/commit-weakening-approved-2c5d4340.md`, preserving the initial false result and all Cease, EOR and final true assertions.

Main then ran `suite 'reload --draft' test 'bgp-route-refresh-enabled-over-ssh' any-failure iterations 80 parallel 2 burners 1 timeout 240 minutes 25` with the poll-capture overlay.
Its log ends `20261105-014430-210448681.log`: invocations 1 through 44 passed, and invocation 45 failed step 6 waiting for `Session committed:` at the unchanged 20-second output deadline.
The retained `route-refresh-pollcapture/daemon-stacks-383822974.json` captures goroutine 162 in the active mirror frame's fsync: `installBytes` through `promoteCandidateLocked`, `recoverFileCommit`, `NotifyReload` and `runCommitSession`.
This is another required storage publication. It establishes neither an 80-invocation pass nor promotion eligibility. No product or test source edits were made in this continuation, apart from the disposable diagnostic overlay.
The authorized fixture correction now uses `readPTYUntilContext04` for the session editor's first-frame and expected-output waits, through both `sessionEditorWaitFor` calls.
The shared matcher reads the case-owned process context. The legacy `readPTYUntil04` wrapper retains its 20-second bound and exact `output deadline expired` text.
Kill and quit still use the legacy wrapper. This correction changes the observer's completion boundary while preserving the case timeout and durability contract.
The scoped native race run, with the full registered tags, passed all selected roots in `tmp/session/2026-11-05-002734/scratch/job-ci-history-wait-race-a34ce998.log`.
Its four new roots cover PTY-close behavior, following-output preservation, caller-owned cancellation and EOF behavior. Existing kill and quit controls also passed, including quit's 20-second bound.
The five plugin drafts passed all 80 invocations under `--name ci41685742426-completion`, with the original stress settings and no diagnostic `GOFLAGS` overlay.
The exact log ends `20261105-101145-1929648125.log`. Native stress reported no reproduction in 80 invocations, with its expected exit 1 for that outcome.
`TestSessionEditorWaitForCancelsLiveSilentRead` also passed under race in `scratch/job-ci-history-live-cancel-race-2f36caf2.log`.
Independent review's second round was clean, including cancellation of a live silent read. The supplemental review of the foreign quit-test companion was also clean.
The scoped count-coverage run passed in `scratch/job-ci-history-wait-coverage-0ce41f48.log`, with profile `scratch/session-editor-wait.cover`.
It executed 6/6 statements in the legacy wrapper, 29/31 in the shared context reader and 8/8 in the session matcher. The unchanged poll-error and OS-deadline error guards were unexecuted.
All new context, deadline and poll paths have positive counts. These figures describe those functions only.
The observer correction landed in `fa795bf224`. Its committed compile check passed every flavor in `scratch/job-ci-history-wait-committed-compile-63ed8ad7.log`.
The corrected route-refresh draft then passed all 80 invocations in the log ending `20261105-104041-1192898260.log` and was copied into its live test path.
The five proven plugin drafts were also copied into their live paths. Four CI files change, because policy disposition is already in HEAD through `4007003754`.
Attach graph's CI delta is comments only, but a fresh HEAD check found its `deliveryGraph02` fixture repair still uncommitted.
The fixture companion accounts for the mandatory filter-owner state edges that HEAD's `communityInsertions` and `grantFilterOwnersState` already produce.
Independent `plugin_promotion_judge` approved that exact fixture companion. Its `Test-judge:` receipt belongs in the commit body, alongside the four CI changes' existing approval rows.

All promoted cases now have observed live passes, with separate startup failures retained. In the first normal four-plugin run, graph and replay passed, while force and verify reached their 180-second startup deadline without early stderr.
The full transcript is `scratch/live-plugin-promoted-20261105.log`. A lock-traced pair also timed out in `scratch/job-ci-history-live-startup-locks-56544725.log`.
`scratch/live-startup-processes.json` records separate case stores and staged frames, but no exact blocked producer was captured.
A later SIGQUIT diagnostic deliberately killed only the two owned daemons and caused an EIO failure. Its elided native output retained no useful full stack.
That diagnostic is recorded in `scratch/job-ci-history-live-startup-stacks-40534bca.log` and `scratch/live-startup-sigquit-processes.json`.
A subsequent normal pair passed both force and verify without signals in `scratch/job-ci-history-live-full-capture-7ffc9b69.log`, with raw streams under `scratch/live-startup-full`.
The promoted route-refresh test passed in `scratch/job-ci-history-route-promoted-6ad0aab8.log`, with raw streams under `scratch/live-route-refresh-promoted`.
The earlier startup timeout's cause remains unproved. If it recurs, use native `-s` capture and obtain a live stack from the owned PID before the hard deadline.
These passes establish no general startup fix or full-CI green result.

The history coverage run passed 37 package/root instances, representing 36 unique root names across six packages, without skips, in `scratch/job-ci-history-owned-coverage-29d35418.log`.
Analysis is complete in `scratch/history-owned-coverage-analysis.md` and `.json`, using `scratch/history-owned-count.cover`.
After summing duplicate counters from the six profiles, 73 of 106 blocks intersecting original production additions have nonzero counts. These are measured block observations, not complete branch coverage.
Plugin promotion landed in `35fdc9b9f0`, including the graph-fixture companion. Its committed source compiled in all five flavors in `scratch/job-ci-history-promotions-committed-compile-63ed8ad7.log`.
Route-refresh promotion landed separately in `4603e2da52`. The five plugin cases and all three reload cases have their required 80-invocation proof and promoted live paths.
Main verified the scoped documentation anchors against the actual helper functions. The native wiring dry run selected aggregate documentation/index checks, which have not been claimed as executed.
The observer correction contains no diagnostic instrumentation. Independent `/root/history_closure` completed the source, guard, preservation and style review. Its one missing-proof finding produced `dec7fc60c7`, which adds constructor/stop guard tests through actual `infra.Run` and plugin-server ownership. Both roots pass race and count instrumentation. Omitted cleanup, detached cleanup and removed post-stop admission each produce the intended assertion red under verified one-file diagnostic overlays. A final canonical control passed with GOFLAGS unset. Canonical source/test hashes stayed unchanged. The committed test also passed all five build flavors.

The focused count profile observes the stopped-admission return, refused-registration cleanup and cleanup body once each, and the sealing callback twice. The initial fixture attempts exposed a nondurable mutex wait and lazy Done channel ownership; those were corrected without changing product code or assertions. Their deliberate diagnostic SIGQUIT is not a product red.

The latest normal startup capture also passed both force/verify cases without signals (`scratch/job-ci-history-startup-causal-capture-8e791a9f.log`, raw streams `scratch/live-startup-causal-capture`). Its first attempt stopped at an unrelated shared inventory compile cutover; the cases actually ran after that source completed. The earlier startup timeouts remain unrepeatable and unexplained, with the next capture step above.

The two DocSource holds are satisfied by inventory foundation `5a6bd238c1` and documentation correction `d5efcf8bd3`. Main verified the current HEAD anchors against their committed producers, so no duplicate patch is owed.
Before the publication turn above, the five RFC metadata records were held against their exact source bindings. Main's comparison against `4603e2da52` found two different cases:

- RFC2545-3-2 and RFC2545-3-3 bind actual checker-function changes absent from that HEAD: `checkRFC2545NextHops` and, for the second record, `checkRFC2545ThirdPartyLinkLocal`.
- Audits RFC4271-5.1.4-1, RFC4659-3.2.1.1-3 and RFC7911-2-2 already match HEAD in every unit and code binding. Only their `tests` whole-file fingerprints for `check_rfc.go` differ. Do not describe those functions as unpublished.

`taggedUnitSHAs` hashes the whole file, while `unitSHAs` hashes the named function. `verdictFreshness` treats tests-only drift as `SHIFTED` when the other bindings match. No native freshness verdict was claimed from that source comparison. At that time, `reseal` and `auditStamp` read the working tree through `Collect` and `newSourceReader`, without committed-source selection. The comparison and exact fingerprints remain in `scratch/rfc-publication-bindings-20261105.md`. The authorized writer change and completed publications are recorded above.

Retain `tmp/session/2026-11-05-002734/scratch/forced-commit-sync.strace`, `job-ci41685742426-sync-trace-0cfab569.log` and `commit-stall-trace-findings.md` for storage diagnosis.
Retain both live stack captures beside the prepared overlay. Remove diagnostic-only artifacts after the cause is settled and no further diagnostic run needs them, preserving useful failure evidence.
The history lifecycle scope and authorized publication task are complete. The required working-tree RFC check's 76-violation snapshot remains disclosed, and full-CI green remains unobserved.

## Start here

1. Read this handover and the completed audit in git history for `spec-cli-history-blocks-ui`. The canonical chronological state is `tmp/session/2026-11-03-231620/state/session-state-cli-history-blocks-ui-omp-ci-41685742426-20261103.md`; its early “missing/unexecuted” entries are superseded by later recorded results.
2. Original failure inventory: `tmp/session/2026-11-03-231620/scratch/ci-failure-inventory.json`. Original six downloaded shard logs: `tmp/session/2026-11-01-174538/scratch/ci-41685742426-shard{1..6}.log`.
3. Use the continuation above for the current next action. The forced-commit diagnosis and disposable capture instructions below preserve the earlier evidence chronology.

## Landed work and evidence

| Commit | Change |
|--------|--------|
| `aa34d659d7` | Async history publication, copied-model drain, all terminal/headless owner cleanup, per-channel SSH closer ownership, per-generation shutdown barrier, actual listener-registration handshake, and hook-built daemon SSH retained through existing plugin-server `OnStop`/`sync.OnceFunc` ownership; 32 files including source, regressions, docs and open spec |
| `ae2fbe6` in `../wiki` | Matching history timing paragraph and generated `llms-full.txt`; generator actually ran; `.source-commit` intentionally unchanged |
| `b7486a68f4` | Earlier honest history progress/handover, not closure |
| `5d5fb6d60c` | Two already-renewed native OSPF discrimination records published; other covers preserved |
| `6774450b8b` | Correct Active Event18 gap disclosure and source bindings; verdict stays unimplemented; existing AC-1e owns the actual retry defect; no runtime fix claimed |

Many earlier repairs from the pinned run are already committed. Important completed slices include `b4b5375625` (roleless-destination OTC export, actual FRR/BIRD positive/negative/restored proof), `73978cd75e` (32 observer progress barriers), `737e5494f6` (21 protocol maintenance paths), `c1631e20bd` and `d887e713bf` (audit records), `4abd85f6f9` (201 enum directives), `d1ade77a43` (allocation/AS-path fixes), `3e364229e1` (PID-port fixtures), and `5c5f2e6eb7` (13 safe DocSources). Do not redo these because the old CI log is still red.

### Observed history/owner verification

All logs named here live under `tmp/session/2026-11-03-231620/scratch/`.

- `job-ci-history-pre-fix-red-834e15a4.log`: all three real Enter branches fail useful input/timer/result progress while the history writer is held. `job-ci-history-post-fix-progress-c32438c2.log`: restored race PASS.
- `history-isolation-discrimination.json`: actual Clone-to-Clip overlay exposes submitted-slice poisoning; real Clone passes. Disposable alias overlay was removed.
- `job-ci-history-affected-race-c5bf8d4d.log`: 29 completed roots before a new fixture bubble deadlock; only gate construction was corrected. Three corrected controls pass in `job-ci-history-bubble-restoration-550cf70e.log`.
- `job-ci-history-unexercised-affected-race-ff78ebd6.log`: four CLI-side packages pass under race, 65 additional completed roots; exact population in `history-cli-affected-results.json`. Do not rerun these merely to reconfirm them.
- `job-ci-history-final-owned-runtime-37b38b45.log`: 33 selected SSH roots pass, including all new listener/model controls and actual PTY/no-PTY sessions. Subsequent hub/config compilation lost shared cache entries; it is not a product assertion failure.
- `job-ci-history-hub-owner-cache-recovery-25969fc6.log`: one documented recovery of only the failed packages passes both actual daemon startup-failure and normal-shutdown history-owner regressions, selected infrastructure/authentication/accounting/lifecycle consumers, and explicit loose/backup constructor refusal. Full 91-line output read.
- Raw native editor admission passed with no inherited `ZE_RUN_JOB`/`ZE_RUN_ATTACH`, patched GOROOT, local toolchain and patched bin first PATH. Exact argv/evidence: `raw-editor-native-admission-restored.json`.
- `job-ci-history-native-editor-exact-paths-618cff67.log`: unchanged `cmd-history-persist.et`, `cmd-history-persist-command.et`, `cmd-history-max.et` all pass.
- Final independent lifetime and behavior fix-only source reviews are clean. This is not the native pinned spec Review Gate or formal closure.

## Current failure: what is known and what is not

### Live CI carriers remain red

`job-ci-history-original-live-force-verify-c26c4bee.log` actually fails both live originals:

- `test/plugin/session-editor-commit-force.ci`: step 16 waits for “force commits over warnings, never errors”.
- `test/plugin/session-editor-commit-verify.ci`: step 18 waits for the prior “role peer requires a filter against a transit leak” error.

These original inputs no longer provoke that error: the role plugin automatically inserts the required checks and permits an empty upstream set. The independently approved drafts drive the real mandatory-member refusal by explicitly naming `bgp-role:otc-ingress`, retain exact warning/error counts and pending/running/persisted-state checks, and read the full refusal/remedy through `show | errors`. Producing refusal: `internal/component/bgp/config/filter_insertion.go::requiredInsertionRefusal`; auto-insertion is owned by the role plugin. Existing approvals are rows 11/12 in `tmp/commit-weakening-approved-21df5267.md`. Do not invent new approvals or silently drop the old error-proof coverage.

### Drafts pass once but fail under load

`job-ci-history-original-force-verify-named-874110ea.log` passes both current draft counterparts in one native run. That is not promotion proof.

The required five-case plugin load **reproduced invocation 3**:

`tmp/stress-repro/plugin-draft-session-editor-commit-force-session-editor-commit-verify-policy-final-disposition-b-61ebef89346daf5c7846b3ed3f662547b2d5f9fb5c6133119ad2d4aa8ed5e7ef-20261105-000914-3797433868.log`

- All five cases passed invocations 1 and 2.
- Invocation 3: force fails **step 12**, waiting for `commit now force: skipping 1 warning(s)`; verify, policy-final-disposition, bgp-capture-replay and attach-process-delivery-graph pass.
- The complete raw terminal capture at line 53 ends after typed `commit now force` input clears. **No final status bytes follow.** This is not demonstrated ANSI-delta matching failure; do not patch the matcher on that belief.
- Actual daemon-session producer: `internal/component/cli/model_commands_commit.go::runCommitSession` (read at lines 419–509). It validates, commits the session candidate, calls `NotifyReload`, marks committed content and records the config commit, then forms `WithSkippedWarnings`; archive notification can also precede the returned result. `handleCommandResult` publishes the status afterward. Current evidence does not isolate which stage remains pending.
- Original earlier stack capture `scratch/session-stall-inv79-daemon-stacks.json` proved history fsync blocked Update and independent draft fsync. That history bug is fixed; it does not identify this new pending forced-commit stage or the separate reload startup timeout.

## Prepared diagnostic: historical first action

Disposable files (not tracked, preserve for the next run):

- `tmp/session/2026-11-03-231620/scratch/stallcapture-FinishEditorCi-20261105/overlay.json`
- Same directory: `plugin_fixture_session_editor.go`.

The overlay changes only the failed-step branch of the existing session-editor observer. Before deferred cleanup kills the case, it runs the supported command `ze cli -c 'show system goroutines full | json'` with the existing authenticated client environment and a separate five-second bound. It writes exact returned output through `os.CreateTemp` as `daemon-stacks-*.json` outside case cleanup and returns the **original** failed assertion, adding the capture path and diagnostic errors. No product source, assertion or original wait/deadline changes. Main read the exact branch and mapping; the next session should confirm the overlay still matches the current shared fixture before reuse.

The unique diagnostic launcher had built/reached admission, but **the diagnostic test itself did not start**: it was queued behind another session's `cpe16-live-l2tp-nak` job. On the user's handover request, only our waiting diagnostic `bg_181` was cancelled. The other session's job/process/locks were not touched. No captured stack or post-overlay runtime result is claimed. Job IDs are process-local; do not expect `bg_181` in another session.

Run from the main checkout after normal claim/admission prerequisites:

```bash
env CLAUDE_CODE_SESSION_ID=<new-session-id> CGO_ENABLED=0 \
  GOFLAGS=-overlay=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-03-231620/scratch/stallcapture-FinishEditorCi-20261105/overlay.json \
  ./le --name ci41685742426-stallcapture test stress-repro run \
  suite 'plugin --draft' \
  test 'session-editor-commit-force session-editor-commit-verify policy-final-disposition bgp-capture-replay attach-process-delivery-graph' \
  any-failure iterations 80 parallel 2 burners 1 timeout 240 minutes 25
```

**Exit 0 means a failure was reproduced**, not PASS. Read the native outcome/log. If it fails, inspect the saved live daemon stack and fix the actual producing boundary. Do not widen deadlines, remove fsyncs, change disk backing, suppress errors, or declare single-pass success sufficient. Remove the disposable overlay and diagnostic-only named tool after the actual cause is captured and no further diagnostic run needs them; preserve useful captures/logs.

## Remaining obligations

1. Completed plugin proof and promotion: all five selected plugin drafts passed 80 invocations. Policy disposition is committed in `4007003754`, and the other four CI repairs plus the graph fixture landed in `35fdc9b9f0`. The observer correction is `fa795bf224`. The unexplained startup timeouts remain retained evidence, with no general startup fix claimed.
2. Completed reload proof and promotion: both community cases passed their paired 80-invocation proof and landed in `405f472cd7`. Route-refresh passed its corrected 80-invocation proof and landed in `4603e2da52`. Every promoted case has an observed live pass. Earlier startup timeout causes were not proved identical to history.
3. Completed history closure: measured coverage, scoped source-anchor checks, independent review, constructor-guard discrimination and paired audit/removal commits. The audit preserves approved test deviations and unrepeatable startup evidence. Do not rerun passing checks to refresh timestamps.
4. Completed authorized publication: the original five holds are in `0db1290f22` and `d8472f0d86`, dependent RFC2545 proof/audits in `7565a5b4b7`, and eleven additional checker-audit consequences in `bc121becf6`. The five target interop covers are current, and all 18 selected audits are fresh against committed source with zero refusals. No owned proof, judgment or metadata publication remains pending. Six earlier ready paths already landed in `5d5fb6d60c` and `6774450b8b`. Do not publish any twice or call all 76 working-tree check violations unrelated.
5. Completed DocSource publication: inventory foundation `5a6bd238c1` and documentation correction `d5efcf8bd3` satisfy both earlier holds. Current HEAD anchors were verified against the committed producers. The older owner state under `tmp/session/2026-11-02-121815/state/` remains historical evidence, and no duplicate patch is owed.
6. Full-CI green is unobserved. Other sessions' nine config/reactor failures and their ongoing BGP/RPC authority/framing cutovers are not this pinned-CI repair's code to alter. The historical no-Python perf-script issue was recorded/owned by `52a9c2c7c9`, not fixed; never execute that archival script or delete the user's history to clear a gate. Do not widen into another audit.

## Files already handled

Do not restart the async-history design or owner census. `history.go`, Model/Enter/mode/timer/dispatch, local/attached/headless owners, `ssh.go`, `model_lifetime.go`, `listener_start.go`, hub `main.go`/`infra_setup.go`/`ssh_infra.go` and all new lifecycle regressions were read, source-verified and independently reviewed. Hunk-selected `main.go` excluded a foreign BGP ConfigData removal; preserve other sessions' work. Docs now describe async best-effort history, graceful drain and abrupt-loss limits without weakening draft/config/commit durability.

## Edits / operating constraints

No further product patch is prescribed before the diagnostic stack. At most these next five bounded changes are justified: actual stalled producer fix; its regression; proven plugin draft promotion; proven reload draft promotion; final spec/evidence closure. Later original metadata/doc publication waits for its recorded producers, not a speculative replacement.

- One CPU-heavy job at a time per session; agents may read/edit/review but do not launch a competing build/test/lint. All work/tests stay in main, no worktree/export/copy. No `./le verify` command.
- Commits only via `./le commit create`, read its message, run exact printed `script=` path. No direct git add/commit/rm/mv/push, no reset/restore/stash. No push was authorized.
- Before a fresh session's first edit, perform its own fetch/behind-zero prerequisite. Load Go style before its first Go edit and required system/subsystem doors before spec edits.
- Use native actions/full registered tags/patched toolchain. Raw unpatched Go previously failed before tests; exact raw-editor corrected setup is in `raw-editor-native-admission-restored.json`.
- BGP runner identities are basename names; the editor runner accepts full `test/editor/...et` paths. Earlier mismatched selections ran no test and are not product reds.
- `read` normal output truncates long log lines. Use `:raw` or JSON selectors for full terminal/stack content; do not parse the truncated displayed string as evidence. LSP missed some new files/feature-gated callers; known controls and actual source census resolved those omissions.
- Retained large passing update-init trace and extracted trace directory belong to this session; purpose/removal notes are in the canonical state. Preserve failed native logs, live stack JSON and useful comparisons. Do not prune shared caches/processes/containers to make room.
