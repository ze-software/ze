# Route-policy implementation handover — 2026-11-13

## Handoff

**Spec:** [spec-route-policy-1-gate-proof-and-forked-sysrib](../release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md)  
**Branch:** main  
**Goal:** Complete correct internal and external system RIB operation, then close the spec through its independent Review Gate and two closure commits.

The owner requested transfer to another agent. This is an unfinished implementation handover, not closure. Do not mark the spec complete or verification. No push was requested or performed by this session.

## Status

- **Committed implementation:** d73a860caf8af26ffed1a2270f464ab0cbf79136, “fix: preserve in-progress system RIB process-boundary work”. It preserves 314 files, including runtime, regression, fixture, SDK, schema, docs and existing RFC proof changes.
- **Committed-tree builds:** native repo compiles check passed distro, appliance, setup, host and installer for d73a860caf. The check compiled what Git holds.
- **Current production fixes:** daemon admission/selection and one import gate; authenticated bounded selected/candidate views; shared forwarding for both placements; distance configure/apply/rollback; generation-owned output; local reader; metadata and recursive resolution; health/metrics; startup budget; replay repair; full FIB stop; connected devices; complete nonwinner explicit ECMP, including ECMP-only contributors.
- **Latest daemon matrix:** deliberately interrupted for this transfer after 18 named cases passed, with zero completed-case failures/skips. Both new nonwinner ECMP placements passed. The two-worker admission run completed/executed exactly 2/2. This is NOT a completed 30-case run.
- **Active work:** two teardown defects below. Their production and existing-fixture fixes are NOT applied.
- **New unverified regression files:** internal/component/sysrib/route_view_runtime_stop_race_test.go and internal/component/plugin/server/dispatch_native_closed_output_test.go. Formatted for transfer, but never compiled or run. Obtain actual RED before implementing the stop fix.
- **Remaining:** full final matrix, six peer interop scenarios, 27 deliberate-failure/restore proofs, measured count profiles, 80 successful load executions of each of 30 functional cases, test promotion/registration, feature evidence/docs, final independent closure review and two closure commits.
- **Not this spec:** another session's unfinished appliance config-push capability and the existing SRv6 inner-TTL defect. Their owners are spec-deployment-user-journeys-appliance and spec-srv6-headend-decrements-inner-ttl. Do not implement those features to make a global package report green.

The final handover commit preserves the two new unverified tests and the adjacent continuation bundle. The original 30 ignored drafts remain in place. Their exact bytes are also committed as .ci.txt archives; they have NOT been prematurely promoted.

## No route-policy proof run remains

The main proof job received SIGINT and joined its own teardown. Its report says check-exit 0, lab-exit 0, command-exit 130, interrupted true, and “VM stopped.” There was no applied mutation to restore.

The old spec claim is released after the handover commit. The successor should claim the spec using its own session identity. Do not export this predecessor's CLAUDE_CODE_SESSION_ID. The owner previously authorized a one-spec WIP-cap exception of 18; that was not a permanent project-wide cap change.

## Two defects to fix first

### 1. Normal internal stop can be retained as a routing failure

Producer: internal/component/sysrib/route_view_worker.go, routeViewWorker.run, approximately lines 72–87.

Process.Stop fences DirectBridge dispatch before callback closure lets SDK Run return. The SDK intentionally keeps the internal owner's signal context live. A queued publication can therefore observe rpc.ErrBridgeClosed before ctx is canceled, and worker.fail records it as a crash.

Independent source judgment from rp6_review_remaining_runtime_tests recommends the narrow rpc.ErrBridgeClosed exception in EDIT 1 below. Do not unconditionally suppress routeViewTransportStopped: an unexpected external EOF, closed pipe, net closure or mux loss must remain a failure while its context is live.

Important provenance: the production server.engineOpCallError normalizes live owner/evaluator errors into *rpc.RPCCallError text, without Unwrap. A live owner returning a closed-like cause cannot impersonate the bridge admission sentinel. The new server regression exercises that real registered adapter; the worker regression separately consumes its published refusal form. A raw test bridge override returning io/net sentinels was rejected as a false production counterexample.

The proposed new tests are still unverified. Preserve all their useful first-ACK, live-context, refusal, failure and doctor controls. Existing malformed-feed and genuine bridge-failure controls must still pass.

### 2. The new nonwinner fixture deletes live devices too early

Producer: internal/test/fixture/sysrib_nonwinner_ecmp_daemon_fixture.go, sysribNonwinnerECMPScenario. Its deferred deletions run when the scenario returns, before observeConfigured quiesces and requests daemon stop. Both native captures show missing-zesrnwc FIB replacement errors after the weighted-group success marker.

The two staged patches in the bundle are UNAPPLIED:

1. cleanup/rp6-nonwinner-cleanup-proof-regression.apply-patch adds a post-await-exit helper and a step in both CIs, leaving the old deletion defers in place. The helper first proves all three Ze-managed prefixes absent across all tables, while all three devices and proto-2 cover routes remain present; only then does it delete the links. The original six-prefix empty assertion remains afterward. Run both cases and observe the new semantic RED before changing lifetime.
2. cleanup/rp6-nonwinner-cleanup-lifetime-fix.apply-patch removes only the premature deletions after that RED. Rebuild and require both full cases GREEN, including pre-delete ownership checks and final emptiness.

Independent ze-test-judge rp6_nonwinner_cleanup_judge APPROVE (a) covers the exact two patches and all four existing files. It verified that every original input, weighted/candidate/control/reload/withholding/replay/placement/shutdown/empty assertion remains. For the fixture and draft-CI edits, retain these verdicts in the eventual commit body:

~~~text
Test-judge: test/draft/plugin/sysrib-nonwinner-explicit-ecmp.ci: APPROVE (a): preserves all original inputs and assertions and adds managed-route absence with surviving OS covers and devices before cleanup.
Test-judge: test/draft/plugin/sysrib-nonwinner-explicit-ecmp-forked.ci: APPROVE (a): preserves all original inputs and assertions and adds the same pre-delete ownership proof across forked placement.
Test-judge: internal/test/fixture/sysrib_nonwinner_ecmp_daemon_fixture.go: APPROVE (a): preserves all original scenario and final emptiness assertions and moves device deletion after joined shutdown and successful pre-delete ownership checks.
Test-judge: internal/test/fixture/register_sysrib_nonwinner_ecmp_daemon.go: APPROVE (a): retains existing registrations and StopsDaemon and adds only the post-stop cleanup helper.
~~~

## Edits

These are the next bounded edits, not permission to skip RED.

**EDIT 1 — internal/component/sysrib/route_view_worker.go:76, after the new regression actually fails.** Immediately after the existing line:

~~~go
if err := worker.consume(ctx); err != nil {
~~~

insert:

~~~go
if errors.Is(err, rpc.ErrBridgeClosed) {
    return
}
~~~

Keep the existing ctx.Err and routeViewTransportStopped checks and worker.fail path unchanged. First verify the new tests against the real adapter contract; this source proposal has not been implemented or tested.

**EDITS 2–5 — the four files in the committed cleanup proof patch.** The patch spells out exact old/new text for:

2. internal/test/fixture/sysrib_nonwinner_ecmp_daemon_fixture.go — add sysribNonwinnerECMPLinksCleanup after sysribNonwinnerECMPKernelEmpty.
3. internal/test/fixture/register_sysrib_nonwinner_ecmp_daemon.go — register plugin/sysrib-nonwinner-explicit-ecmp-links-cleanup.
4. test/draft/plugin/sysrib-nonwinner-explicit-ecmp.ci — insert the helper after daemon/await-exit and retain all existing expectations.
5. test/draft/plugin/sysrib-nonwinner-explicit-ecmp-forked.ci — the same explicit insertion, preserving forked placement.

Apply only the proof-regression patch first. Phase 2 is needed after these edits: obtain both native semantic REDs, then apply the separately committed lifetime-fix patch, update the source-anchored docs, and prove the unchanged scenarios and scoped tests GREEN.

## Then: commands and evidence rules

All development and execution stays in this main checkout. Main runs all builds/tests/formatters/native actions serially. Agents may read or edit source only. Never use a worktree or any le verify command. Use named builds and CGO_ENABLED=0; race-only test commands may use CGO_ENABLED=1.

First run the new regressions BEFORE EDIT 1:

~~~bash
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
GOMAXPROCS=4 CGO_ENABLED=1 ./le --name session job run label route-policy-stop-red quiet command go test -v -count=1 -race -timeout 2m -tags "$tags" -run '^(TestRouteViewRuntimeNativeDispatchStopJoinsBeforeContextCancellation|TestRouteViewRuntimeNativeLiveOutputFailuresRemainFatal|TestOwnerNativeClosedOutputFailuresRemainRPCRefusals)$' ./internal/component/sysrib ./internal/component/plugin/server
~~~

A compile/setup failure is not semantic RED. Read the actual named tests and controls. After the source repair, run that exact selection again and the reached full sysrib/plugin-server lifecycle populations.

Before daemon cases, rebuild both daemon and fixture binaries through the existing native builders:

~~~bash
GOMAXPROCS=4 CGO_ENABLED=0 ./le --name session job run label route-policy-guest-build quiet command ./le --name session test qemu guest-binaries
~~~

Use its returned ZE_BIN, ZE_STRIPPED_BIN and ZE_STRIPPED_FLOW_BIN assignments. The own-system command action builds fresh guest le when harness is named. Example for one case:

~~~bash
GOMAXPROCS=4 CGO_ENABLED=0 ./le --name session job run label route-policy-native quiet command ./le --name session test qemu docker-lab kernel /home/thomas/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-6727e257-c79de00c/vmlinuz harness timeout 3000s command 'ZE_BIN=tmp/qemu/linux-amd64/bin/ze ZE_STRIPPED_BIN=tmp/qemu/linux-amd64/bin/ze-stripped ZE_STRIPPED_FLOW_BIN=tmp/qemu/linux-amd64/bin/ze-stripped-flow tmp/qemu/linux-amd64/le test qemu all test test/draft/plugin/sysrib-nonwinner-explicit-ecmp.ci'
~~~

The last accepted kernel SHA was 19e1bdeee5d415cffb95a3d302140f838dfa166c7e09125a7c0ef88bdeceb16e, release 7.3.0-ze, image bb0789656639aba7. Native validation compares kernel bytes and image identity. Runtime inputs can change in this shared checkout: resolve the new matching tree if refused; never bypass identity validation.

Use native save with a fresh private path when full capture is needed. It cannot combine with stress. Require each native case's own PASS, one executed test, zero skips, all phases, positive controls and joined cleanup. A command list in a log is not evidence that its later commands ran.

The two-worker admission command already completed 2/2, but the final per-case load requirement remains:

~~~text
<returned guest binary assignments> tmp/qemu/linux-amd64/le test qemu all test test/draft/plugin/<literal-stem>.ci stress 80 stress-parallel 4
~~~

Require Completed=Executed=Iterations=80, no failure/skip/interruption, for EACH of the 30 cases. All 2,400 final executions remain unrun.

## Remaining proof, in order

1. Fix/prove the two teardown defects and refresh affected docs. Complete the final 30-case matrix on the final source; do not promote drafts early.
2. Run the six actual peer scenarios: isis-import-policy-frr, isis-import-policy-frr-forked, ospf-import-policy-bird, ospf-import-policy-bird-forked, isis-export-policy-frr and ospf-export-policy-frr. Use the registered docker-lab lab action with INTEROP_SCENARIO. Peer sources at the lab versions were already read (FRR10.7.1/BIRD3.4.0); verify pins before changing expectations. Hardware preflight has not run.
3. Measure count-mode coverage using the archived coverage recipe; it has NOT been executed. Update its old kernel path. Keep native gocoverdir propagation and netlink instrumentation, and read actual per-outcome counts rather than infer them from passing tests. Include the latest connected/ECMP-only/stop/checker branches.
4. Execute all 27 guarded discrimination pairs. The committed proof files are proposals with empty outcomes, not executed evidence. Copy the proof directory into the successor's own private scratch before running; the driver writes beside itself and must NOT write generated attempts under plan. Refresh source hashes/patch context after fixing route_view_worker.go, especially cut12. Do not remove a hash guard to proceed. Each mutation needs its marker, nonempty diff, actual semantic RED, exact guarded restoration and full same-carrier GREEN.
5. Run 80 successful loaded executions per final case; promote all 30 drafts into test/plugin only afterward. Apply/update the native namespace inventory registration for all30, run its declaration lint and scoped functional population, and inspect test-cost scope.
6. Add all30 promoted cases to features/route-policy.md, retain six interop declarations and record a real feature run. The current feature declaration remains Experimental. Never fabricate generated feature evidence or Supported status.
7. Complete source-anchored docs/schema/runtime inventory/generated docs, compare the feature row, check RFC freshness and repoint closing-spec citers. The umbrella's own closure is separate; do not close it.
8. Run one independent ze-close context, without its own subagents: deliverables, ACs, security/docs and final hash-pinned CLEAN Review Gate. Its final scope must include the implementation commit and all later fixes/proof/docs. Land Commit A and Commit B(spec removal) in ONE generated le commit script, then release the claim. No push without the owner's order.

No requirement has been dropped or homed into a new spec by this handover.

## Files already handled

The broad implementation diff was independently read in full: the original frozen review snapshot covered 320 paths/38,576 lines, followed by bounded reviews of all fixes. Those reads and findings are recorded in the local canonical state. Do not restart the 32k-line implementation from scratch; read the producing functions for the two open defects and review each new fix.

- docs/architecture/rib/unified-locrib.md — authority, transport, lifecycle and owner decision.
- docs/architecture/api/process-protocol.md and plugin SDK docs — published operations and startup/stop boundaries.
- internal/component/sysrib/route_view*.go, projection*.go — authority, consumers, configuration, output and health.
- internal/component/plugin/server/{dispatch_native,startup_engine_op,startup_budget,restart_controller}.go — authenticated shared infrastructure.
- internal/core/rib/locrib/{manager,entry,forwarding_members}.go — authority, originals, hidden LPM and full selected membership.
- internal/plugins/connected/{connected,locrib}.go — actual covering-device producer.
- internal/plugins/fib/kernel/{fibkernel_replay,fibkernel_stop,backend_replay_linux,nexthop_linux}.go and the versioned vendor patch — kernel identity/ownership replay and scope.
- the two new stop regression files — source-only checkpoint; their correctness and execution are still owed.
- the committed cleanup patches — independently judged, still unapplied.

The earlier review found and fixed diagnostics, replay admission/order, schema sentinels, ECMP membership, unchanged-insert allocation and source-anchor defects. The final bounded source review before the teardown discovery had no additional issue, but NO final CLEAN artifact exists.

## Actual evidence retained locally

All paths below are beneath tmp/session/2026-11-12-205428/scratch.

| Evidence | Actual result |
|---|---|
| job-rp6-committed-runtime-compiles-e6594260.log | All five committed binary flavors PASS for d73a860caf. |
| job-rp6-connected-locrib-sysrib-green-15edfbdc.log | Full connected/Loc-RIB/sysrib race PASS, including seven connected egress regressions. |
| job-rp6-ecmp-only-accepted-path-red-8cf87d78.log | NEW missing-member/notification/both-reader semantic RED before repair. |
| job-rp6-refusal-gate-regressions-red-1e7aa604.log | Checker and namespace RED before their fixes; ECMP-only/command regressions PASS. |
| job-rp6-ecmp-only-and-refusal-full-packages-green-b1cbc93e.log | Full errorinventory/Loc-RIB/sysrib PASS; QEMU timed out in the separate agent-client test. |
| job-rp6-agent-single-client-and-namespace-green-7a4fae44.log | Agent setup repair plus namespace failure/success PASS under race. |
| job-rp6-full-qemu-after-client-setup-green-8e067f0d.log | 319 roots:314PASS,5FAIL,0SKIP; all route-policy selection/namespace/stress/save controls PASS. |
| job-rp6-final-thirty-daemon-and-stress-admission-ca20a59b.log | 18 named daemon cases PASS, stress2 exactly2/2; owner-requested transfer interrupted the remainder and joined the VM. |
| rp6-final-source-30-matrix-1794577116686/<stem>/*/client-stderr.log | Complete individual captures, including both observed teardown defects. |

Completed cases are listed in the adjacent evidence/matrix-interrupted.json. Twelve cases were not completed; no 30-case or final post-teardown-fix claim follows.

Earlier valid scoped evidence remains in the canonical state: eleven final native FIB roots; actual slow-reader recovery and BGP metric callers; five bounded fuzz campaigns; eight existing RFC discrimination records and next-hop audit reseal; allocation comparisons and bounded retention. The unchanged-group insertion benchmark measured winner/nonwinner each0B/0alloc after repair. Report no timing/throughput number from these runs.

## Unrelated working-tree changes

Sessions share main. The predecessor deliberately excluded active foreign XFRM, appliance/SSH/config-upload, OSPF-default and LDP changes from its commit or used safe shared-file projections. Do not stage them under “all route-policy work.”

One foreign test setup was repaired to unblock the harness: repeated agent.NewClient on one connection creates competing pipeline readers. Independent judge approved reusing one client without losing any input/assertion; native approval was recorded before edit and the test passed. Its still-untracked test/producer belong to the deployment session and are excluded from this handover commit; the exact repair and judgment are preserved in related-agent-setup. The five remaining QEMU failures are unfinished appliance native-action/orchestration tests, not route-policy failures.

The existing full-FIB TTL64-vs63 failure belongs to spec-srv6-headend-decrements-inner-ttl. Global RFC/repo prechecks also have disclosed unrelated findings. Preserve their honest scope; do not claim the whole repository green.

## Continuation bundle and local state

The adjacent directory contains exact source archives, not runtime proof:
30 .ci.txt draft archives; all27 current unapplied cuts, source hashes and serial driver; cleanup patches; count-mode recipes; the independently judged foreign setup patch; and an interrupted-matrix summary. MANIFEST.json records source/archive paths and SHA256.

Current ignored drafts already exist in this checkout. On a fresh checkout recreate their original test/draft/plugin/*.ci paths from the archives with exact bytes, without overwriting a different draft. Recreate proof recipes in the successor's own private scratch. Run all project code through existing registered le actions.

The much longer local navigation/audit state remains at:

tmp/session/2026-11-12-205428/state/session-state-route-policy-1-gate-proof-and-forked-sysrib-01a1d178-88dd-7463-b4bb-ae4c147dd158.md

Read its final root checkpoints and phase handoffs if a specific earlier result needs reconstruction. Its predecessor session ID is provenance, not an identity to reuse.
