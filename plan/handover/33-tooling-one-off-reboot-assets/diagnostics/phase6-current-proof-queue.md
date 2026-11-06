# Phase 6 current source and proof queue

Source-only audit on November 6, 2026, for MAIN. Scope: P-7, P-10,
P-18 and P-36 of `plan/post-release/engineering/spec-tooling-one-off-defects.md`.
No build, test, formatter, Go/gopls, interpreter, Docker, QEMU, network or runtime
command was run. Only repository source, documentation and the exact current
state/evidence pages were read. No production, test or documentation source was
edited. The separate Docker-lab harness changes were inspected and preserved.

## Evidence boundary

The spec's Phase 6 row is still open. Proposed carrier names in its tables are
requirements, not statements that the named test exists or ran. Current actual
paths supersede proposed paths: dispatch is `internal/le/verify/dispatch`,
not `internal/le/verifydispatch`; the native documentation action is
`doc yang-contract doc-drift`.

`tooling-one-off-evidence-recovery-2026-11-06.md` says the former November 5
session raw logs, overlays, profiles, captures and proof JSON are unavailable.
The durable evidence page still records the earlier observations. Its historical
wording that a volatile artifact is retained does not restore those bytes.
This audit did not retrieve or recreate any missing original. No Phase 6 result
was run after recovery by this agent. Before using old observations as current
retained proof, MAIN must bind available bytes, canonical source and command in
the private archive named by the recovery companion. Missing raw proof is a
reason for the required new proof, not a reason to repeat unrelated unchanged
checks for a newer timestamp.

Relevant recorded observations, with their limits:

| Recorded job/commit | What the evidence page says was observed | What it does not prove |
|---|---|---|
| `10c07359d9`, `56aa4530`, `7351afb1` | Kernel worker ownership implementation; actual Docker isolation/removal and scoped contracts | Actual kernel compilation or boot |
| `5b794f03`, `cd3474d5d2` | Selected native, hugepages and isolated catalog assertions pass; host process owners land | Real filesystem mount, PPP, image, kernel or VM behavior |
| `960411b6`, `5db6a848`, `325dad55` | Actual native compiler descendant leak, controlled reproductions and nonterminal group restoration | Every Phase 6 route |
| `50f3761b` invocation b | Native QEMU entry returns status 129, compiler socket EOF, pidfd-observed tool death, direct Go disappearance and unchanged inherited parent | Actual Go generation, image, guest boot or own-system execution; its tool-version probe was blocked |
| `d3ffb6b9`, `0fc494ea` | Job identity package passed; dispatcher cancel-slot originally failed before readiness for missing fixture HEAD, then only that repaired subcase passed | A product cancellation RED from the fixture failure |
| `f1a4edc8` | Actual standalone reclaim JSON agrees with exit/report; locked/dead directoryless registrations removed; live/outside sentinels retained; malformed/unreadable ownership refused | Any verification stage, six-route cancellation or retained immutable copy of that historical native image |
| `e36477e2`, `76804da0`, `472c77b4` | Kernel staging input/ownership RED and restoration; selected-root path controls and actual Run/gok-edit consumer | Real kernel/image/guest or attribution of the earlier identity refusal |
| `ca2c40c9`, `fc37fb3d`, `2694c76025` | Kernel-payload drift RED/restoration and private-payload publication | Six-route cancellation |
| `475f79dd`, `a4c53cfc` | Native kernel staging; real own-system command checks ordinary UID, exact PID 1, squashfs, runtime release, no Docker socket, readable parent token, exact scratch writeback and shutdown | Selected acceleration from the candidate string; actual guest admission without fingerprinting; cancellation of a running real guest |

The current state checkpoint separately records active MAIN jobs. MAIN must
finish its active CPU job before starting any item below.

## Implementations and source carriers

### P-7: dispatcher and reclaim

`internal/le/verify/dispatch/dispatch.go` `dispatch` receives the caller context,
refuses an already canceled root, creates the stage with that context and sets
the existing two-second output bound. `runStage` uses
`runner.StartServerProcess`, immediately defers checked `Close`, waits on the
owner, and joins its output reader. It does not call a verify action to test
this machinery.

`cancel_group_test.go` `TestDispatchCancellationReapsInflightStage` executes the
registered child boundary in three modes: TERM-ignoring group, escaped pipe
holder and occupied admission. It checks direct Wait completion, interrupted
result, bounded return, group-child disappearance in group mode, foreign sleeper
survival and no later stage. `TestDispatchCancellationBeforeStart` covers the
already canceled root. The escaped child is deliberately cleaned by the fixture;
bounded pipe closure does not claim to kill/reap it. The cancel-slot helper
intentionally has a Background root inside the stage; killing the stage is its
outer cancellation mechanism.

Reclaim is registered separately in `internal/le/reclaim/register.go` and calls
the existing sweep. Current carriers are `TestStandaloneReclaimWorktrees`,
`TestReclaimDirectorylessOwnerAndInterruptedRemoval`,
`TestReclaimRefusesUnknownOwnerAndPreservesLiveOwner`,
`TestReclaimRemovesLockedDirectorylessRegistration`,
`TestReclaimPreservesUnownedRegistrations`, and
`TestReclaimPreservesWorktreeDuringAdd`. The standalone carrier checks the
registered handler, no Verify result and no stage directory. Native `f1a4edc8`
is the recorded stronger entry proof. Do not run actual checkout reclamation to
renew it; use an authored private Git administrative fixture only.

### P-10: cache, KVM, mounts, cwd and unknown probe

`internal/le/test/qemu/run.go` `scratchShares`, `cacheShare` and `virtfsArgs`
resolve the supported cache symlink into a separate share. Exports still use
`security_model=none`; no source-based reason to change that policy was found.
`TestACacheLinkIsOneShare` covers linked/real/absent/dangling layouts.
`TestGuestBootstrapContracts` executes the generated setup in a confined shell
fixture with failed workspace and cache mounts and rejects later curl/tar/Go.
This tests shell ordering, not real 9p xattr handling. The historical cache
symlink xattr ELOOP cause remains unattributed, as the current QEMU page states.

`Run.acceleration` returns `kvm:tcg` for native Linux arm64 in the non-hardware-only
branch. `TestNativeARM64LinuxUsesKVM` checks that planning answer. It is not
evidence that an arm64 guest actually selected KVM, and amd64 replay cannot
prove the owner's arm64 case. The rule requires native-host architecture, so
do not cross-build/emulate another kernel to manufacture that evidence.

`internal/appliance/gok_execute.go` `ExecuteGok` wraps actual gok execution in
`keepWorkingDir`, which captures/restores the invocation cwd even on failure.
`cmd_build.go` `runGokBuild` makes the image path absolute before gok; the later
injection uses its caller paths after cwd restoration. Existing
`TestKeepWorkingDirRestoresAfterAChdir` and `TestBuildUsesGokBuildFn` cover those
pieces. `internal/le/build/gokrazy/root_ownership_test.go` covers selected-root
preparation/preflight. The proposed `TestRelativeGokInputsSurviveCwd` was not
found under the proposed deployment path; its exact end-to-end contract,
relative `ZE_APPLIANCE_DIR` through changed gok cwd into injection, still needs
a direct consumer witness. The recorded gok-edit smoke is related but does not
by itself prove that injection contract.

`internal/le/interoplab/kernelcheck.go` `checkDockerKernel` distinguishes unknown
rows before prescribing missing-feature remediation, preserving row reason and
exact rerun command. `TestDockerKernelUnknownIsProbeFailure` covers generic and
Ze releases using scripted Docker output, preserving positive diagnostic
assertions. It proves the consumer mapping, not a real failed kernel probe.

### P-18: kernel worker and terminal installer

`internal/appliance/kernelbuilder/driver.go` `runContainer` records a full
daemon-issued ID after detached bounded create, attaches under the original
root, and removes that exact ID with a detached bounded context before return.
`runDocker` refuses ownership repair/reclaim if removal is unconfirmed.
`TestKernelContainerCancellationJoinsBeforeRepair` is an actual Docker carrier:
TERM-ignoring heartbeat worker, repair checks that heartbeat stopped, no surviving
owned container and a foreign container remains running. It needs an explicit
locally cached `KERNEL_CONTAINER_PROOF_IMAGE`; unset means SKIP and cannot close
the obligation. This is lifecycle proof, not real kernel compilation.
`container_lifecycle_test.go` separately covers create cancellation, unknown
create/removal refusal and worker outcomes.

`internal/le/setup/docker/kernel/dockerkernel.go` owns the terminal exception.
`installSignalContext` stops interception before canceling the first root;
`installCommand` retains `os.Stdin` and sends the direct program TERM rather
than moving sudo into a background group. `runKernelInstallPlan` reports only
completed steps and names every unfinished step, final GRUB selection and
reboot-sensitive state. `TestInstallSecondSignalEndsWaitingEntry` enters the
actual signal wrapper and `runInstallStep`, with an owned subprocess that
acknowledges and ignores TERM; a second SIGINT must end le. Its fixture explicitly
retains then cleans its own step. It does not claim sudo exit reaped a privileged
tree. `TestInstallPlanProgress` exercises pre-first-step, failed first step,
failed/interrupted copy after actual temporary module removal and final GRUB-read
failure. `TestInstallUnstartedStepDoesNotClaimChanges` covers pre-start refusal.
No real host installation or reboot is needed or appropriate for these proofs.

### P-18: six-route coverage census

| Actual entry | Current producer and source coverage | Remaining proof boundary |
|---|---|---|
| `test deployment l2tp-ppp` | `actions.go` `signalRooted` -> `L2TPPPP.Run(ctx)` -> kernel probe, native daemon build, namespace setup, `observe`. `startWatched` uses one shared owner; `closeNative` joins owner/output before collector join. `TestNativeHostBuildCancellationJoinsDescendants` enters actual build producer; `TestNativeListenerStartupCancellationJoinsDescendants` enters actual listener startup and checks no peer/verdict. Shared descendant fixture ignores TERM. | Real registered native action canceled during build/setup/listener/session with foreign namespace/process retention and no later work. Current listener fixture replaces privileged namespace commands; no real PPP/kernel test is proved by it. No admission queue is introduced by this route. |
| `test deployment gokrazy-l2tp-ppp` | `signalRooted` -> `gokrazyL2TP.Run` -> `proveAppliance`; rooted build, setup, observe; deferred uncanceled lab removal follows owned process joins. `TestGokrazyBuildCancelEndsTheGrandchildren` checks group teardown; `TestNativeAppliancePhaseCancellationJoinsDescendants` enters database and boot command producers with TERM-ignoring descendant and live foreign socket. `TestGokrazyStoppedRunPrintsNoVerdict` uses stand-in stages. | Actual image/lab/VM/PPP cancellation. Host fixture processes are not a booted image. Need actual setup/peer/session cleanup and foreign-resource assertion. |
| `test deployment l2tp-scale` | `runL2TPScaleHere` signal root -> actual scenario startup -> `realL2TPScaleSystem.Start/Run/Sleep`. Start and simulator use shared process owner; deferred Stop joins it. `TestScaleEntryCancellationStopsBeforeSimulation` invokes registered Answer in a subprocess, signals it after real daemon command readiness, requires ordinary failure and dead daemon; `TestScaleStopJoinsActualDescendants` adds TERM-ignore and foreign socket; `TestScaleSimulatorCancellationJoinsActualDescendants` covers simulator producer. | Native composition-root action with actual daemon and simulator cancellation, no later scenario, real output verdict and owned process receipts. Entry startup fixture does not exercise an actual simulator; separate simulator fixture does not join both into one native route proof. There is no build/admission phase in this action. |
| `test qemu vpp-hugepages` | `guestContext` -> `Hugepages.Run` -> native host build, appliance init/build, boot/query. Shared owners in build/appliance and boot/SSH leaves; cancellation returns no payload. `TestHugepagesVerbRootReachesEveryOwnedPhase` covers host-build/init/image-build/kernel/memory using semantic tools; review carriers check actual payload and normal cooperative VM stop. | Real image/kernel/VM reservation and cancellation. Current root descendant cooperates with TERM/INT; it is not a TERM-ignore carrier. Current root/review assertions do not supply a separate live foreign process sentinel. This action has no new admission wait. |
| `test qemu docker-lab` | `runDockerLabHere` root covers image acquisition, `BuildLinuxZe` admission, boot/readiness, host le or guest command. `TestDockerLabBuildCancellationLeavesOccupiedAdmission` checks actual queue plus unchanged foreign holder inode/bytes and no compiler. `TestDockerLabStartupCancellationStopsOnlyOwnedVM` enters lab and command branches with real fixture QEMU process, recorded ownership and foreign-file retention. Existing `TestDockerLabForwardsOneInterruptThenStopsTheVM`/guest-command tests cover transport/teardown policies. | Actual image build, occupied wait, guest startup and running lab/guest cancellation with immutable receipts. Startup QEMU is a fixture and foreign sentinel there is a file. Preserve the separate current harness work; do not modify it or mistake its tests for already executed proof. |
| `doc yang-contract doc-drift` | `runDrift` roots SIGINT/TERM; `BuildDaemon` and `runCommandCatalog` derive bounded contexts; `runCatalogProcess` defers shared-owner Close and waits. `TestCatalogActionCancellation` calls real Answer with actual Go build or freshly built daemon gated at external-command boundary; checks nonzero result, both recorded owned processes dead, unrelated sleeper alive and no next stage. Normal carrier releases same gate and parses the actual catalog. | Retained current native composition-root proof and producer-cut RED/restoration. Its gate child is ordinary sleep, not a TERM-ignoring child. Isolated symlink root deliberately excludes sibling publication/wiki corpus, so it is not full drift/publication coverage. No admission queue exists here. |

## P-36: observational admission and guest identity

`internal/le/job/registry.go` `scanAndClaim` counts unreadable or liveness-unknown
holders as occupied, ignores demonstrably gone/zombie capacity without removing
foreign files, and never signals by silence. `reportBusy` reports measured quiet
time. The misleading historical test names are preserved intentionally:
`TestAStalledHolderIsBrokenOnItsSilence` now asserts the opposite behavior,
including a real sleeper, repeated actual waiting banners, cancellation of only
the waiter, unchanged foreign entry/log and eventual admission only after the
fixture owner ends the holder. `TestAnUnreadableEntryIsDroppedOnceItIsOlderThanTheStallWindow`
now asserts occupied refusal and preservation. Other carriers include
`TestAQuietHolderOccupiesExactlyOneSlot`,
`TestProcessQueryFailureKeepsCapacityOccupied`,
`TestCancellationInterruptsAContendedRegistryLock`,
`TestAWaiterPreservesOldForeignReceipts` and
`TestUnreadableParentIdentityRefusesWork`.

`job.go` `admit` calls `insideParent` before directory creation/InputHash. An
unreadable/invalid LABEL refuses; an absent entry admits anew. Guest setup writes
a guest-local LABEL stand-in and exports `ZE_RUN_JOB`; it does not export the
host PID registry entry. `TestRunGuestCommandRunsInsideTheHostSlot` executes the
actual generated shell setup, then calls actual admission in a non-Git directory:
KindInside proves no fingerprint there. `TestAdmitQEMURunRunsInsideAParent` covers
host nesting; QEMU queue/root descendants have separate source carriers.

Important gap: `TestDockerLabCommandRunsInZeSystem` is a plain `command` invocation,
not `harness`, and only tests that `$ZE_RUN_JOB` is readable. Its recorded real
guest PASS cannot prove actual guest admission returns KindInside before
InputHash. Join the existing two halves in a native own-system harness witness:
actual guest le admission under the guest-local stand-in, with an input-reading
tripwire/non-Git admission root and preserved host holder receipts. A source cut
that makes admit fingerprint before recognizing the parent must turn this guest
carrier red. Do not manufacture a whole-tree 9p hash as the normal proof.

## MAIN execution queue

All commands below are proposals for MAIN, serialized in the main checkout.
Use the existing named native le and established all-feature/integration tags;
do not build another platform, copy/export source or run any verify command.
Retain native executable/source digests, exact argv/selection, stdout/stderr,
exit/assertion verdict, skips and owned cleanup receipts in the private archive.
An overall cleanup error remains an overall error even if assertions pass.

1. Recover/identify already available Phase 6 raw bytes at their exact archived
   locations before rerunning unchanged work. Do not search trimmed tmp trees.
2. Run only missing or source-changed focused host carriers, selected per package.
   Use the standard MAIN envelope:

   `"$LE" job run label <unique-label> quiet command go test -race -tags "$tooling_test_tags" -p=1 -run '<selector>' -count=1 -timeout=<bound> -v <package>`

   Suggested selections, each one job, not one broad package sweep:

| Package | Exact missing-proof selection to consider |
|---|---|
| `./internal/le/verify/dispatch` | `^(TestDispatchCancellationReapsInflightStage|TestDispatchCancellationBeforeStart)$` |
| `./internal/le/job` | `^(TestAStalledHolderIsBrokenOnItsSilence|TestAnUnreadableEntryIsDroppedOnceItIsOlderThanTheStallWindow|TestAQuietHolderOccupiesExactlyOneSlot|TestProcessQueryFailureKeepsCapacityOccupied|TestCancellationInterruptsAContendedRegistryLock|TestUnreadableParentIdentityRefusesWork)$` |
| `./internal/le/setup/docker/kernel` | `^(TestInstallSecondSignalEndsWaitingEntry|TestInstallPlanProgress|TestInstallUnstartedStepDoesNotClaimChanges)$` |
| `./internal/le/test/deployment` | `^(TestNativeHostBuildCancellationJoinsDescendants|TestNativeAppliancePhaseCancellationJoinsDescendants|TestNativeListenerStartupCancellationJoinsDescendants|TestScaleEntryCancellationStopsBeforeSimulation|TestScaleStopJoinsActualDescendants|TestScaleSimulatorCancellationJoinsActualDescendants|TestScaleActualProcessNormalCompletion)$` |
| `./internal/le/test/qemu` | `^(TestHugepagesVerbRootReachesEveryOwnedPhase|TestHugepagesCancellationReturnsNoPayload|TestHugepagesNormalCompletionCooperativelyStopsVM|TestDockerLabBuildCancellationLeavesOccupiedAdmission|TestDockerLabStartupCancellationStopsOnlyOwnedVM|TestRunGuestCommandRunsInsideTheHostSlot|TestGuestBootstrapContracts|TestACacheLinkIsOneShare|TestNativeARM64LinuxUsesKVM)$` |
| `./internal/le/doc/yangcontract` | `^(TestCatalogActionCancellation|TestCatalogActionNormalInvocation)$`; allow the carrier's existing compiler readiness bound, do not raise a product timeout |
| `./internal/le/interoplab` | `^TestDockerKernelUnknownIsProbeFailure$` |
| `./internal/appliance` | `^(TestKeepWorkingDirRestoresAfterAChdir|TestBuildUsesGokBuildFn|TestInjectionConsumerCancellationJoinsDescendants)$` |

3. If needed after source/retention assessment, actual Docker worker lifecycle:
   select only `^TestKernelContainerCancellationJoinsBeforeRepair$` in
   `./internal/appliance/kernelbuilder`, with `KERNEL_CONTAINER_PROOF_IMAGE`
   set to a confirmed local immutable shell-image ID. Preserve no-pull behavior;
   record SKIP as unmet, and no actual kernel build is implied.
4. Author the missing native signal observers before running them. Start the
   exact six native entries above under MAIN's admitted owner; use producer
   readiness/immutable process or resource receipts, signal only that entry,
   and observe owned teardown before any fixture fallback cleanup. Establish
   foreign process/resource sentinel readiness separately. Native source overlay
   fixtures can identify a phase but cannot replace real PPP/VM/mount claims.
   Every applicable phase needs its own row: already-done root, build, existing
   admission wait, startup and running. Do not add a queue to actions that have
   none. Keep installer terminal semantics separate.
5. Add the real guest-admission witness above; use the own-system route
   `test qemu docker-lab kernel <staged-native-runtime-vmlinuz> harness command
   "<exact-owned-guest-proof>" timeout <existing-bound>`. The existing integration
   root `^TestDockerLabCommandRunsInZeSystem$` is the normal identity/writeback
   control, not a replacement for that admission/cancellation witness.
6. Finish P-10 direct relative-appliance-dir/injection and cache-symlink xattr
   attribution. Record exact failed operation, filesystem/export/layout and
   positive control. Do not change security_model from a guessed ELOOP cause.
7. Perform the producer cuts below with unchanged carrier assertions, then
   restore exact source and prove the restoration. Archive before/after source
   bytes and logs. No cut was executed in this audit.

## Producer fault cutpoints

| Obligation | Minimum source fault at producer | Required discriminating result |
|---|---|---|
| P-7 active stage | Detach the context passed by `dispatch` to spawn, or remove checked owner Close in `runStage` | The unchanged registered-stage TERM-ignore/readiness fixture fails bounded teardown or descendant death; foreign remains untouched. Do not force ActionResult failure/success. |
| P-7 reclaim | In `internal/le/verify/cleanup.go` `sweepAbandoned`, require a checkout directory before handling an owned directoryless marker; separately bypass owner/containment judgment in the registration path | Owned dead marker remains or protected registration is changed; actual standalone JSON/exit reflects failure. Never point the cut at real shared worktrees. |
| P-10 required mounts | In `Run.setupCommand`, restore an ungrouped optional OR branch after the required mount chain | Existing generated-shell test sees forbidden later download/unpack/build after mount failure. Do not alter its fake mount verdict. |
| P-10 arm64 | Change Linux arm64 selection in `Run.acceleration` to `hvf:tcg` | `TestNativeARM64LinuxUsesKVM` fails its plan; separate native-host acceleration observation remains owed. |
| P-10 cwd | Omit cwd restoration in `keepWorkingDir` during real gok consumer execution | Relative invocation input/injection fails or touches decoy paths. A callback-only equality check is insufficient. |
| P-10 unknown | Let unknown rows fall through to the missing-feature branch in `checkDockerKernel` | Exact named unknown-proof test fails diagnostic attribution on both release cases. Do not inject checker return. |
| P-18 native build/PPP | Detach `buildHostDaemon`/`nsCommand` from root, or omit `observe` deferred `closeNative` | Existing socket/worker/no-peer fixture fails; native phase observer must also detect live owned descendant/no later step. |
| P-18 gokrazy | Detach rooted `gokrazyBuildCommand` or database/boot command; omit owned close or `proveAppliance` cancellation check | Relevant database/build/boot fixture fails, not only a synthetic stage-return test; actual lab/VM cleanup witness still required. |
| P-18 scale | Detach Start/Run context, replace shared-owner Close with child-only teardown, or permit another scenario after root ends | TERM-ignoring daemon/simulator remains observable or later scenario starts. Keep normal actual process controls green. |
| P-18 hugepages | Detach one build/appliance/SSH command from root, omit its owner Close, or return canceled report payload in `runHugepages` | Named phase fails bounded stop/no-later-work or payload oracle. Add TERM-ignore and foreign controls without replacing existing normal cooperative stop. |
| P-18 Docker lab | Detach `BuildLinuxZe` admission context; detach readiness in `dockerLab`/`dockerLabCommand`; omit deferred owned VM stop; or omit interrupt forwarding in `runHostLe`/guest transport | Respectively compiler/queue, startup VM receipts or running child remains observable. Exercise each changed branch; no fake `report.Interrupted` cut. Preserve foreign harness hunks. |
| P-18 catalog | Detach context in `BuildDaemon` or `runCommandCatalog`, or omit checked Close in `runCatalogProcess` | Unchanged actual Answer carrier fails five-second cancellation, owned death/no-next-stage or foreign-sentinel assertion. Native entry still needed; forcing a report code is invalid. |
| P-18 kernel worker | Remove exact-ID teardown in `runContainer`, or allow `runDocker` repair after unconfirmed removal | Actual heartbeat/repair/liveness test goes red while foreign survives. Never widen ancestor/name cleanup to shared containers. |
| P-18 installer | Keep signal interception after first root cancellation in `cancelInstallOnSignal`; drop ctx precheck in `runInstallStep`; or corrupt completed index/unfinished reporting at `runKernelInstallPlan`/`installPlanFailure` | Second SIGINT is suppressed, an unstarted command writes, or exact completed/unfinished/reboot assertions fail. Do not change sudo process group/stdin or claim privileged-tree reaping. |
| P-36 silent holder | Let `scanAndClaim` treat live quiet/unreadable holder as free capacity, or reintroduce signaling/removal there | Real queue prematurely starts or holder/entry/log changes; actual owner-ending positive control still admits afterward. |
| P-36 inherited identity | Move InputHash before `insideParent`, accept unreadable/invalid entry, or omit guest stand-in export | Local non-Git admission control and new actual guest witness fail before child work; host foreign receipts remain exact. A readable token alone cannot distinguish this cut. |

No finding above closes a journal occurrence, Review Gate or phase. MAIN still
owns execution, independent review, retained evidence and any implementation
that the direct proof demonstrates is missing.
