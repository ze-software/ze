# Two supplemental producer cuts

Both proposals are UNAPPLIED, uncompiled and unobserved by this preparation author. Use the same fresh current-source copy and guarded restoration as the daemon cuts. They do not replace any of the 24 daemon cut populations. Wait for the current backend author's source-STABLE handoff before main compiles the shared package.

## 25: false/error replay normalization

Source: internal/plugins/fib/kernel/fibkernel_replay.go. SHA256: 47f3eb7829b956ef3eba6cd77636d34f4abc11d421047fb76e08ee6689883304. Patch: execution-v3/25-replay-unhandled-error.patch.

The only semantic edit is return true, err to return handled, err in tryReplay's nonnil-error arm. An adjacent // MUTATION-APPLIED comment marks the active cut. True/error refusal and false/nil fallback stay intact. The existing processEvent install arm then overwrites false/error with ordinary addChange's result.

Exact target: TestFIBKernelReplayUnhandledErrorDoesNotReplace. Its ordinary successful install is the first barrier. Its optional fake returns false/error; expected semantic RED is one forbidden full replacement and absent pending failure. The successful snapshot remains independently checked. Keep the other nine controls in the SAME selection; no test body changes.

A directly read current baseline exists in job-rp5-live-probe-and-device-scope-kernel-green-f808e804.log: this target and the other replay units named PASS. That is not a recorded break. Main still owes cut RED, guarded restoration and the same selection GREEN.

Use source-cycle.md's fresh copy/apply/diff/restore steps with this row. Replace its guest-binaries build/daemon run with the exact scoped race selection below. Run while cut is active, then after exact restoration with rp5_stage=green:

~~~bash
. ./feature-tags
rp5_tags="ze_core,$(feature_tags feature-gates.txt ,)"
rp5_replay_selection='^TestFIBKernel(ReplayReassertsUnchangedForwarding|ReplayChangedMetadataUsesFullReplacement|ReplayUncertainStateUsesFullProgramming|LiveChangesNeverUseReplayReassertion|ReplayUnhandledUsesFullReplacement|ReplayUnhandledErrorDoesNotReplace|ReplayReplaceKeepsFullProgramming|ReplayFailedReplacementNeedsFullRepair|ReplayRefusalPreservesSuccessfulForwarding|ReplayWithoutOptionalBackendStillRepairs)$'
rp5_stage=red
if CGO_ENABLED=1 ./le --name session job run label "rp5-replay-unhandled-error-$rp5_stage" quiet command go test -race -p 1 -v -count=1 -tags "$rp5_tags" -run "$rp5_replay_selection" ./internal/plugins/fib/kernel; then
  rp5_unit_exit=0
else
  rp5_unit_exit=$?
fi
printf 'stage=%s actual_exit=%s\n' "$rp5_stage" "$rp5_unit_exit"
~~~

Read each exact selected name from the quiet log. The target must fail semantically while the other controls retain their assertions. Package source changes invalidate RED's cache; -count=1 also requires the restored GREEN to execute instead of reusing the old baseline. -v exposes the selected names. This is a bounded discrimination pair, not a broad repeated gate. A compile failure or disappearance of every backend call is not this proof.

## 26: real privileged child cancellation

Source: internal/test/fixture/sysrib_process_executable_fixture.go. SHA256: 0b500847c8d8f00f463f75e08aadbaf718db24fddc9c92396ccbd1953d0e0f3d. Patch: execution-v3/26-privileged-cancellation.patch.

The only semantic edit is exec.CommandContext(ctx, ...) to exec.CommandContext(context.WithoutCancel(ctx), ...). An adjacent // MUTATION-APPLIED comment marks the active cut. Keep CommandContext itself, argv, bounded captures, WaitDelay, StartServerProcess and checked deferred Close. Using exec.Command instead fails setup when the owned runner installs Cancel, before the useful child exists.

Exact target: TestSysribExecutablePrivilegedCancellationJoinsRootChild. It must first authenticate an actual root descendant's READY identity, owned group and procfs T state with a pinned pidfd, then explicitly cancel before the outer request expires. Expected semantic RED: "actual inspection runner did not join its stopped root child on cancellation". Unchanged failure cleanup cancels, reclaims that pinned child and unconditionally joins the request. A root-role SKIP, EPERM, missing READY, detached ancestry/group or Cancel-without-context setup error is not the target.

A directly read existing non-root baseline is job-rp5-kernel-startup-producer-and-root-child-proof-b08af07c.log, with the exact named PASS and cap_kill=ep. Cut RED and restored same-role GREEN remain owed. The ordinary root package pass cannot substitute for this role.

After source-cycle.md's fresh copy/apply/diff steps, build a NEW fixture ELF for each stage on the host. The example uses red; after guarded source restoration set green and repeat these exact build/native steps. Each binary has a different retained path.

~~~bash
. ./feature-tags
rp5_tags="ze_core,$(feature_tags feature-gates.txt ,),integration"
rp5_stage=red
rp5_fixture="$rp5_attempt/fixture-$rp5_stage.test"
CGO_ENABLED=0 ./le --name session job run label "rp5-privileged-cancel-build-$rp5_stage" quiet command env GOOS=linux go test -c -race=false -p 1 -tags "$rp5_tags" -o "$rp5_fixture" ./internal/test/fixture
sha256sum "$rp5_fixture"
printf -v rp5_fixture_argument '%q' "$rp5_fixture"
rp5_guest_command="set -e
mkdir -p /tmp/rp5-control-exec
mount -t tmpfs -o mode=0755,nodev tmpfs /tmp/rp5-control-exec
cp $rp5_fixture_argument /tmp/rp5-control-exec/fixture.test
chmod 0755 /tmp/rp5-control-exec/fixture.test
setcap cap_kill+ep /tmp/rp5-control-exec/fixture.test
getcap /tmp/rp5-control-exec/fixture.test
cd /tmp/rp5-control-exec
sudo -n -u ze env HOME=/tmp TMPDIR=/tmp /tmp/rp5-control-exec/fixture.test -test.v -test.count=1 -test.timeout=1m -test.run '^TestSysribExecutablePrivilegedCancellationJoinsRootChild$'"
if CGO_ENABLED=0 ./le --name session job run label "rp5-privileged-cancel-native-$rp5_stage" quiet command ./le --name session test qemu docker-lab kernel "$rp5_kernel" root timeout 45m command "$rp5_guest_command"; then
  rp5_native_exit=0
else
  rp5_native_exit=$?
fi
printf 'stage=%s actual_exit=%s\n' "$rp5_stage" "$rp5_native_exit"
~~~

Use the matching absolute rp5_kernel from carrier-commands.md/current image. The root command shares the host absolute checkout, not /workspace, so the fixture path above is absolute. The private executable tmpfs omits nosuid, has mode0755 and grants only CAP_KILL to the fixture ELF. The test runs as non-root ze; its real sudo child becomes root. Require the exact named restored GREEN with zero skips. Retain source digests, both ELF digests and both quiet/native logs. Existing VM teardown reclaims the private mount.

No performance figure is proposed or reported. Command durations are existing deadline settings.

## 27: unchanged effective offer retains latest producer originals

This additional UNAPPLIED unit proposal is documented in
`27-unchanged-original-values.md`, with patch
`27-unchanged-original-values.patch` and exact current source guard in
`27-unchanged-original-values.json`. It skips only `g.upsert(stored)` when
`sameEffective` is true. The fresh-copy, marked-apply, semantic-RED,
guarded-restore and identical-GREEN recipe keeps the other four new explicit
ECMP roots as useful controls. No source mutation or execution is claimed.

Use `../execution-v4/cuts.current.json` for the refreshed complete 27-proposal
population. Original 24 cut semantics are retained; manager and worker hashes
and their current patch contexts were refreshed after the ECMP and replay fixes.
The earlier current-v2 and execution-v3 proposals remain historical and unapplied.
