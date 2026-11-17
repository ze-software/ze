# IPsec checkpoint for continuation, 2026-11-13

Branch: `main`. Both specs remain `in-progress`:

- `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
- `plan/release/fixes/spec-rfc4301-architecture-gaps.md`

The owner asked to commit as much existing work as possible and then report so
he can decide whether to continue here or move machines. This is a checkpoint,
not closure. No push was ordered. Dual-form AC-4 is unchanged: RFC4301-5.2-2
must lose `{partial}` only when the broader obligation is actually proven.

## Current continuation, 2026-11-17

Continue from the dual-form spec's **Current continuation checkpoint, 2026-11-17**. It
records landed guard commit `bde4a49a6347`, the separate eleven-test raw and packet PASS
on d87/d2eb with matching 43a1 image, and scoped host/Linux engine lint PASS. Older
unrun and source-only statements below are historical. Both specs remain open; no push
was made by this session.

The sound IPv6 queue witness is now accepted. The missing-field RED in run 21799 on the
preceding normal kernel retains its TDD scope. Normal build 43301 succeeded with C patch
`e3ecc903`; actual B1 kernel SHA256
`f82654a4d1458db3094449274348cf5268e0f09e912776a95b52263f27816536` and matching image
`d9efda5bec4fd7f7` passed capability and counter tests in first boot run 36055. All
eight input and Colima holds were released after its VM stopped. Independently approved
helper source `08e8a270` observes `XfrmFrag6Queued` with negative rejection; IPv4
parsing and lifetime assertions remain unchanged. Formatting 84982 and Linux compilation
96345 passed; binary `b1a1994c` then passed exact lifetime run 55413 in 7.08 seconds,
covering both families and arrival orders. Separate run 59439 passed the three raw tests
and eight packet tests: eleven top-level PASS results on the B1/f826 kernel and matching
d9 image. Both runs had no skips, zero exits, interrupted false and stopped VMs. Scoped
engine lint 60545 passed host and Linux integration with zero issues. Owning docs are
`3601b075`; this counter and helper phase landed in
`84d13c03ee526447bc06646b4fb652fb35dd9bd1`. The dual-form checkpoint names the exact
logs. Next is the remaining proof work. The first native renewal recorded manifest order
58 after its selected clean PASS and producer-halt FAIL. Main also renewed host-positive
orders 9, 15 and 44, runs 94797, 40175 and 17887, each exact selected clean PASS then
native installChildSA halt FAIL, with only selected producer seals changed to
`17aa3b0ab62f6479`. The dual-form checkpoint names the logs. These remain dependency
proofs of partial tagged claims. The final eight-revert batch leaves all 79 current
renewals RECORDED and zero pending. Eight scoped reports verify all 79 exact
roster keys, with zero missing/stale/wrong-seal/no-break matches; bounded legacy
renewal/fingerprint debt is complete, without whole-RFC or C credit. All four historical records
now have current VERIFIED source/fingerprint verdicts from native selected reports;
the CE ID retains six other unproven tags. This supplies no new packet observation,
whole-ID or full-RFC credit. Partial function-dependency proof gives no C-enforcement or
full-RFC credit. Both rejected tasks and their owner question remain pending. Full AC-4,
compliant-fragment RED and mapped-return work remain open. The generic C-proof route has
independent schema, native-image and observer-manifest reviews CLEAR (draft `2438aa68`),
but remains an UNAPPROVED P3 design proposal with A-R2 and A-R4 unvalidated and seven
later RFC792 rows preserved. The design prose landed in
`467baad97515e924378d0e7a58a8e56735046f98`; that commit grants no design approval. Its
separate owner design question (prepare pairs separately versus recorder builds) is
PENDING and does not approve either rejected safety action. No tooling implementation or
C-proof credit is established.

The [dual-form current checkpoint](../release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md#current-continuation-checkpoint-2026-11-17)
owns the completed host and eight guest batches' native evidence and proof
limits. It also records report repair `3d6aa6c86f`, AC-30's existing lint
limitation and four current VERIFIED historical records; those bounded
conclusions and pending approvals apply here.

## Landed work

| Commit | Change and evidence limit |
|--------|---------------------------|
| `8b3c7a34cd`, `2db581e7d6` | Earlier RFC 4301 implementation checkpoints |
| `abeb7d6535` | Dual-form implementation; the old journal refusal was resolved. Its incorrect `Spec-work: deployment-user-journeys` metadata must not hide its files from final review |
| `bd8e21b487` | Receiver diagnostics, ESP GRO doctor check, and the three judged test corrections |
| `14bec40b17` | Independent source directories for concurrent kernel workers; named regression and scoped race package passed; all five committed build configurations compiled |
| `352f231413` | Netlink selector-family and state-scope work, repaired forward-application headers |
| `b43120b371`, `c9ce6ba81d` | Ordered netlink series and upstream-style tests; see the library handover |
| `5c063bc8d6` | K1 exact state-scope and K2 fragment-context source checkpoint, with no native C compilation or packet proof at that checkpoint |
| `2ae4f0f3f4` | K3 draft preserved at `gokrazy/kernel/proposals/xfrm-fragment-reassembly.patch`; unenrolled and unproven |
| `d5459aae36` | Correct the ESP receiver's token-refill rate |
| `66825526f2` | Start the external IKE engine through the shared CLI and promote both runtime-doctor tests |
| `5ab68bb5fa` | Keep an attached childless SA through the owner handoff during an ESP-group reload, with its regression and AC-21 |
| `afd59c50b9` | Preserve reader failure events and replacement backoff through retirement, with the companion architecture paragraph |
| `2d25927741` | Hide retained device failures after the final configured peer is removed, with unit coverage and companion documentation |
| `f457c7c22d` | Land the qualified internal/external peer-removal daemon tests, their test backend and same-answer observer |
| `671c5926e2` | Correct only the capability-rejection test request's encoding, with independent judgment. No kernel or vendor code changed |
| `cb5e4a6044` | Reject empty and whitespace-only Ze XFRM snapshots in the form-change checker, with focused and full checker-package passes before helper wiring |
| `7d49a5e3f9` | Land the 30-file peer-local helper, image staging, same-SA form-switch checker and tests, with scoped repair validation. Proof metadata is prepared separately; parent-package lint remains red |
| `927f93fafe` | Land nine reviewed namespace/permission repair files after eight exact-capability namespace tests, affected guest population and both packages passed. Rebuilt-source firewall clean/fault/clean subsequently passed its intended discrimination sequence; lint retains bounded outside-repair findings |

Go caller checkpoint `466ab09fd8` landed 55 files, including the callers,
selector repairs, diagnostics, regression probes and companion documentation.
The required `./le repo compiles check` passed at `466ab09fd8b9` for all five
configurations: distro, appliance, setup, host and installer. This checks
committed production code and dependencies; it does not compile test files
or prove the kernel C patches. No fresh tests ran for this checkpoint.

The Go checkpoint carries immutable scope construction, exact projection and
capability admission, single-pair inbound state binding, protocol/MH refusal,
FWD-default ownership and doctor reporting, and the saved packet regressions.
It does not enable full scope installation on a partial kernel receipt.

## Evidence already obtained

The logs named in this section were under
`tmp/session/2026-11-10-225434/scratch/`, which exists on neither the Mac nor
the `claude` host (checked 2026-11-15). The names stay as the record of what
ran; none of the logs can be read now.

- `job-ipsec-dualform-host-redwalks-aa95f75a.log`: all sixteen remaining host
  mutations, 27, 31–33, 37 and 39–49, returned native red verdicts with each
  clean own-test pass and broken own-test failure. Guest mutant 20 is not run.
- `job-unit-pkg-58fce57a.log`: targeted corrected dataplane package run passed.
  `job-ipsec-reopen-schedule-e00a01c8.log`: narrow race/count-20 scheduling
  run passed. These are not the final guest population.
- `job-ipsec-binding-doctor-units-419db7fc.log`: named receiver doctor and
  binding fixture tests passed, including ESP offload findings.
- `job-ipsec-scope-params-family-green-67381781.log`: targeted immutable scope,
  capability admission, VPP refusal, projection and selector-family units
  passed after observed semantic failures. This proves Go behavior only.
- `job-ipsec-kernel-workspace-package-81a675f5.log`: kernelbuilder race package
  passed; its optional Docker cancellation test skipped because
  `KERNEL_CONTAINER_PROOF_IMAGE` was unset.

## Current receiver corrections, 2026-11-16

Both specs remain open, including dual-form AC-4 and its broader RFC 4301 obligation.

| Correction | Producing code | State |
|------------|----------------|-------|
| Refill the complete-path token bucket at 5000 tokens per second. An empty bucket gets 5000 tokens after one second and 7500 after 1.5 seconds. | `dataplane/espform.go` `espFormLimiter.allow` | Committed |
| Stop reporting a retained TUN-open failure after the last configured peer is removed. The receiver keeps its failure record. | `engine/doctor_esp_form_fast_path.go` `espFormDeviceFindings` | Product correction committed. Both applicability-guard cuts discriminated. Both 80-run qualifications passed and the CIs were promoted unchanged. The full 50-case Linux suite passed. Qualification support includes the test backend and observer |
| Report a recorded reader failure once across replacement, preserve the replacement's backoff, and serialize failure publication with retirement. Intentional retirement stays silent. | `dataplane/espform_open_linux.go` `read`, `revive`; `dataplane/espform_linux.go` `retireLocked` | Committed. Controlled-schedule and final unmodified-source Linux tests passed. Isolated stale-backoff cut produced the intended test failure |
| Start the registered external IKE engine through the shared plugin CLI and its authenticated TLS connection. | `engine/register.go` `init`; `plugin/cli/cli.go` `RunPlugin`, `connFromEnv` | Committed |
| Let an attached childless SA reach its owner before an ESP-group-only reload requires a restart. Keep the existing two-second admission deadline. | `engine/reconcile.go` `retargetESPGroup`, `adoptESPGroup` | Committed |

Paths in this table are under `internal/component/ike/`, except
`plugin/cli/cli.go`, which is under `internal/component/`.

The main thread observed these results in the main checkout:

| Command or proof | Observed result and limit |
|------------------|---------------------------|
| Host `internal/component/ike/dataplane/` package with current feature tags, `zetest` and `CGO_ENABLED=0` | PASS in 0.471 seconds. |
| Host `internal/test/fixture/` package through native `test unit pkg` with `CGO_ENABLED=0` | PASS, one package. Receipt: `job-ipsec-fixture-package-6994a9c9.log`. |
| Final unmodified dataplane binary through `./le --name session test qemu docker-lab` root command mode | 35 distinct outer top-level tests PASS, with no skipped or failed nested tests. All 29 rewrite cases, TUN node/errno/user-namespace cases, eight migration tests, legacy/bitmap/ESN controls and both capability-helper markers ran. This includes all six final reader tests without a scheduling overlay. Ze kernel `7.3.0-ze`, check/command/lab exits zero, interrupted false, VM stopped. |
| Five selected engine packet/context/receipt tests through the same root-command guest route | Four packet/context tests passed in the initial run, including all 16 fragment-context observations across both arrival orders. The receipt-rejection test first failed. After the judged request-encoding correction, that isolated test passed on the identical kernel. The repaired run had no skips, zero check/command/lab exits, no interruption and a stopped VM. |
| Eight selected trafficusage tests through the same root-command guest route | All eight passed without skips. The four L4 boundary subcases, ingress/egress clsact controls and capability-helper marker ran. Same Ze kernel, zero check/command/lab exits, no interruption and a stopped VM. |
| `./le --name session test unit pkg path ./internal/component/ike/engine/` | Latest run: PASS, one package, 162.552 seconds, after the childless handoff correction. `CGO_ENABLED=0`. Receipt `job-childless-engine-package-94d132fd.log` distinguishes this run from the earlier result at the reused native log path. |
| Race-enabled selection of `TestESPGroupReloadWaitsForChildlessOwnerHandoff` and four existing reload/refusal/schedule tests | All five top-level tests and their subtests passed without skips. The new test first failed against the old nil-owner guard. Independent source review was CLEAN. |
| `./le --name session test ipsec -p 1 -v ipsec-childless-ike-sa-recovers` | Main reported PASS, 33.2 seconds, with all 15 VERIFY STEP rows passing. The unchanged CI checks recovery under the same IKE SPI and rejects `RECONNECTED`. |
| `./le --name session test ipsec --draft -p 1 -v ipsec-esp-form-fast-path-doctor` | PASS through the internal engine. The recording backend's cause, fix, severity and complete-path cost reached the doctor answer. |
| The same command selecting `ipsec-esp-form-fast-path-doctor-external` | PASS through the actual forked `ze plugin ike` daemon under test and its TLS connection. |
| `test/draft/ipsec/ipsec-esp-form-doctor-peer-removal.ci`, initial own-system run | PASS in 3.7 seconds. One selected case passed, one phase ran and none skipped. Ze kernel `7.3.0-ze`. Check, command and lab exits were zero, interrupted was false, and the VM stopped. |
| `test/draft/ipsec/ipsec-esp-form-doctor-peer-removal-external.ci`, initial own-system run | PASS in 6.2 seconds. One selected case passed, one phase ran and none skipped. Same Ze kernel, zero check/command/lab exits, no interruption and a stopped VM. |
| Paired peer-removal applicability-guard cuts | Both selected tests reached their own no-peer `ZE-OBSERVER-FAIL` with `device=true retained-cause=true counter-warning=true`. Internal: 46.8 seconds. External: 56.1 seconds. Each executed one case and one phase without skips on the same Ze kernel. Check/lab exits were zero, command exit was one, interrupted was false, and both VMs stopped. |
| Native cached-pair boot after the carrier repair | The explicit `6727e257` kernel and `54adcae1c6c63cd2` image booted successfully. `uname -r` answered `7.3.0-ze`, with kernel SHA256 `3c13600db1d2c6fb8028e8fdd16420e5b376a8f9315d008313edd027ab43502e`. All exits were zero, interrupted was false and the VM stopped. |
| Native peer-removal qualification, internal and external | Each completed and executed all 80 requested invocations, with no skipped phase. Each raw capture contains 80 own-test PASS rows, 80 zero-exit invocation headers and 320 passing VERIFY STEP rows. Both guests used the proven kernel, returned zero check/command/lab exits and stopped without interruption. |
| Full affected Linux IPsec suite | All 50 live CI names passed exactly once, with 50 executed single-case selections and 50 zero-skip phase reports. No failure or skip markers occurred. This includes the 13 cases skipped on the earlier host run, same-SPI childless recovery and both promoted peer-removal cases. Same kernel SHA256 `3c13600db1d2c6fb8028e8fdd16420e5b376a8f9315d008313edd027ab43502e`, zero check/command/lab exits, no interruption and a stopped VM. |
| Selected-CI output-capture attempt | The runner rejected `-save` before it executed a test. This supplies no product or fault-proof result. The CI tooling owner was informed. |
| `./le --name session test stress-repro run suite "ipsec --draft -p 1" test ipsec-esp-form-fast-path-doctor any-failure iterations 80 parallel 2 burners 1` | All 80 invocations exited 0. The retained tails contain 240 passing VERIFY STEP rows for this test. The harness returned 1 to report that it reproduced no failure. |
| The same stress command selecting `ipsec-esp-form-fast-path-doctor-external` | All 80 invocations exited 0, with 240 passing retained VERIFY STEP rows naming the external test. The harness returned 1 for no reproduced failure. This completed after the VPP owner restored a coherent source checkpoint. |
| Native runtime-doctor row-removal redwalk | Clean selected test PASS. Removing its runtime owner made the same test fail at its required diagnostic assertion, with `ZE-OBSERVER-FAIL`. |
| Native external-entrypoint redwalk | Clean external test PASS. The broken DUT compiled, then timed out after the shared-runner cut prevented TLS connect-back. |
| AC-2 helper, checker and staging | Wiring/four subtests and five generic staging consumers passed. Full helper and checker packages then passed in `job-ipsec-peerform-checker-packages-green-2d1ebed9.log` (0.242s and 10.829s), including missing/same-form controls, 12 direction/phase stalls, malformed receipts and the outbound-only template regression. Linux helper cross-build passed in `job-ipsec-peerform-linux-build-81b69d7b.log`; native Linux helper tests passed without skips in `job-ipsec-peerform-linux-units-dcd7f143.log`, including projection, link, GETSA and migration framing. Current kernel/image pair, zero exits and stopped VM. |
| Repaired live `esp-form-change`, clean run | PASS in 8s, `job-ipsec-live-form-switch-clean-de5e18d6.log`, immutable Ze image `32eec714a8eabfb4679bcd7c1ca9df3f070c0a5e030a76d744f8326588371c48`, strongSwan image `8c2dda87d201730eee7714c34150e9b93d30b259f71f821f2272a7a1eb275b05`. The 40-row checker recording is `scratch/interop-recordings/ipsec/esp-form-change-YLWTMXUXDJM7FC6X42RQZD7MBR.jsonl`. It records bare probe sequence 1, UDP acknowledgement 1, UDP probe sequence 2, bare acknowledgement 2 and bare probe sequence 3, all on helper SPI `0x2bfcb9ba`, reqid 1. The real checker verified four counters and identities. Check/command/lab exits zero, no interruption, no resources left, VM stopped. The as-used binding cut then failed at its first actual bare reply in 16s; post-clean passed in 8s, as detailed below. The later comment-only input remains unexecuted. |
| Four fresh native RFC records | RFC4301 positive and negative, RFC7296 ECN positive and RFC5082 Unknown-SYN positive recorded clean own-test PASS and intended packet-row RED on current kernel SHA256 `c3583fe57d83e971a6c24c7d6e93d5b4e682a5797dbb0d15dad42d642f1c8798`, matching image `45cb736f9a13fd62`. No skips; VMs stopped. AC-4 stays partial because the single-pair bare-ESP proofs do not establish general per-SA scope installation and isolation. |
| Affected live IPsec suite before the handoff fix | 48 cases: 34 PASS, one FAIL (`ipsec-childless-ike-sa-recovers`), 13 Linux skips. Both promoted doctor cases passed on the unmodified DUT. The later isolated childless PASS does not establish a full-suite PASS. |
| `./le --name session doc check all` | This turn's direct run exited 1 and initially reported 389 documentation-drift issues. The remaining output was truncated; no new log path was retained. Earlier retained run `job-ipsec-doc-check-f119ecea.log` separately reports 389 drift issues, mostly published wiki/gh-pages CLI drift, plus 12 source-anchor findings outside IPsec paths. Neither receipt supplies an owned-docs PASS. |

The channel-ordered regression proves the handoff defect at `retargetESPGroup`.
The earlier functional failure's elided runtime trace does not prove that attribution.
The unchanged functional rerun separately proves recovery with the correction.

The publication regression first failed under a single-file Go overlay that
added `runtime.Gosched` between the stop check and failure publication.
`TestESPFormReceiverConcurrentFailureAndRetirementReportsEveryRecordedFailure`
recorded a failure with zero events on iteration 1. This was a controlled
scheduling reproduction. The shared source carried no scheduler yield.

After per-life serialization, the same yield ran inside the critical section.
All six selected tests executed and passed on Ze kernel `7.3.0-ze`, with zero
skips and the VM stopped after the run:

```text
TestESPFormReceiverReportsFailureRetiredBeforeRevive
TestESPFormReceiverIntentionalRetirementHasNoFailureEvent
TestESPFormReceiverRetiredLifeCannotReopen
TestESPFormReceiverMatureRetiredLifeKeepsBackoff
TestESPFormReceiverConcurrentFailureAndRetirementReportsEveryRecordedFailure
TestESPFormReceiverReopensAStoppedReaderByItself
```

The main thread used `./le --name session test qemu docker-lab` in root command
mode. The guest ran the rebuilt binary with `-test.v -test.count=1`, a
three-minute test timeout, and count-mode coverage. The binary SHA256 was
`78d5ad4f803811088816dcf0869a9712141cf92074ec92365c3cf79f1ca6571d`.
The kernel SHA256 was
`3c13600db1d2c6fb8028e8fdd16420e5b376a8f9315d008313edd027ab43502e`.
Check, command and lab exits were all zero.

Measured coverage counts were 67 intentional-read returns and 1090 failure
publications, 278 stale-life returns and 812 current-life continuations in
`revive`, and 1 nil-life return against 1158 retirements. The nested stop arm
observed 106 recorded deaths and 67 intentional stops. These counts prove
those outcomes ran under the controlled schedule. The final unmodified-source
dataplane run separately passed with binary SHA256
`0309b7b65ed086db18f3051fb4023cb959fd8c630b0e9548c384ddd25ea818cd`
on the same kernel.

The isolated stale-backoff cut then made
`TestESPFormReceiverMatureRetiredLifeKeepsBackoff` fail: replacement backoff
was `0s`, with `30s` required. The private overlay disabled the early
reader-identity guard in `revive` and also omitted a G101 annotation comment.
The shared source stayed unchanged. Main recorded the broken binary SHA256 as
`908c6809135854584e8fc24589ba05c212a9540d3e0f9a0aa66b165505ebff01`.
The guest used the same kernel. Check and lab exits were zero, command exit
was one, interrupted was false, and the VM stopped.

The current session state owns the raw logs, overlay paths and their cleanup
conditions. Its root is `tmp/session/2026-11-15-130223/`. This durable receipt
records the outcomes needed after those temporary files are removed.
The three daemon inputs in `scratch/doctor-current-guest-bin/` kept the tested
bytes stable while other sessions refreshed shared staging paths. Their hashes
are in the parent state. After `f457c7c22d` preserved the qualification receipts,
main checked active processes and references and removed those completed copies,
recovering 231 MiB. Main also removed the completed 50-case driver, the superseded
unrun 13-case driver and the completed f457/cb5 native commit scripts. The earlier
standalone packet, negative-test and private cut-DUT binaries were already removed
after their receipts committed. Keep the small qualification and fault logs through
the spec's closure review. Their owning state records the removal conditions.

The later native cache trim (`job-ipsec-safe-cache-trim-c2992b33.log`) freed
5 GiB while preserving the three-hour young-cache floor. The 13 MiB Linux
helper test binary, `scratch/ipsec-peerform-linux.test`, had SHA256
`5ed06e4c4916b2a8554f7b0b5081dfbf75aecb8f9c652a0ae8adf129f61fd702`
after the lint-repair rebuild (`job-ipsec-peerform-lint-fix-linux-build-1732f37d.log`).
After `7d49a5e3f9` landed, main confirmed no open files or active owning processes
and removed the binary. Main also removed the obsolete unused `a-c54a88` native
script/message, completed `b-9f2ab9` script and used phase body/list. Small proof logs,
private cuts for remaining trials and the live checker recording
remain through closure review.

## Remaining work, in order

1. Done: the private-ABI move landed in `8afd1ff560` (2026-11-13), and
   `plan/post-release/engineering/spec-netlink-xfrm-upstream-series.md` records
   it as AC-5 and AC-6 met. That spec's closure owes the scope install and
   read-back integration tests on Ze's kernel, which the move did not run.
2. The historical guest mutant 20 remains unrecovered. Its definition was
   entry 20 of `b9-mutants.json` in the lost session tree (see "Transfer and
   recovery"). The four fresh clause-scoped candidates below do not assert
   that identity. All four fresh native records below are observed. The older
   transport-positive/negative and tunnel-positive/negative covers were renewed
   by dependency breaks. The partial audit was independently cleared and natively
   stamped, and the current check reports no audit/discrimination freshness
   finding for this exact requirement. Of the 83 old-record inputs, four are
   observed and 79 remain unobserved, with details and retention in the state.
   Regenerate changed mutation
   coordinates rather than accepting stale-hunk failures as red evidence.

   Main ran the five changed-line architecture checks. Operator-error passed;
   compound-guard, refuse-by-default, error-wrap and doctor-coverage reported
   findings outside IKE, routed to their live owners. The parent state records
   each receipt. These are not whole-gate green claims. Scoped IKE and diagnostic
   lint findings remain to resolve. The bounded lint repairs have independent
   CLEAR review and fresh main-thread package PASS in
   `job-ipsec-lint-repair-packages-4419bc1f.log` (native
   `job-unit-pkg-cf26cae2.log`): checker 12.442s, helper 1.304s. The rebuilt
   helper passed all 31 top-level Linux tests, including fuzz seeds, without
   skips (`job-ipsec-peerform-lint-fix-linux-units-bad03c34.log`), on the current
   kernel/image pair with all zero exits and stopped VM. The post-repair scoped
   lint (`job-ipsec-helper-checker-lint-after-repair-41608712.log`) reported
   14 Darwin and 16 Linux findings before the final import-group correction.
   Helper-only lint then reported zero Linux issues and one Darwin
   `encapsulation` unused finding, for a field consumed by Linux
   (`job-ipsec-peerform-final-import-lint-25898e43.log`). The code phase landed
   in `7d49a5e3f9`. Existing parent findings and the counter-test modernization
   note remain. No overall lint-green claim is made.
   A lint or gate red is owed at release through CI and holds no commit.
3. Qualification support at this checkpoint includes the last-peer removal/re-addition
   carriers, their test-only backend and the same-answer doctor observer.
   The carriers completed qualification at these draft paths:
   `test/draft/ipsec/ipsec-esp-form-doctor-peer-removal.ci` and
   `test/draft/ipsec/ipsec-esp-form-doctor-peer-removal-external.ci`.
   Both initial cases, paired applicability cuts and 80-run qualifications are
   observed. The CIs were promoted unchanged to
   `test/ipsec/ipsec-esp-form-doctor-peer-removal.ci` and
   `test/ipsec/ipsec-esp-form-doctor-peer-removal-external.ci`.
   Their hashes remain `a5a9953b990d2cb5e2845409023949d3ec1ede874471b55962d05d1be96c7849`
   and `e34c9c78169900ced34ed67e77e434b59600527490ae4c4ed62d2916efb32854`.
   The affected 50-case Linux suite also passed. These synthetic retained-failure
   tests claim no packet delivery. The separate
   `internal/le/interoplab/ipsec/espform_change_snapshot_test.go` regression
   belongs to the checker repair. Its empty and whitespace cases produced the
   expected RED in `job-ipsec-form-change-missing-state-red-57ce2d65.log`.

   The earlier current-runtime preflight refusal is resolved for an explicit
   cached kernel/image pair, proven by the successful native boot and qualification.
   Native RFC records still take only `kernel` and omit `image`. They do not
   select that cached pair through this repair. H41 published the current `e8763657`
   runtime kernel, SHA256
   `c3583fe57d83e971a6c24c7d6e93d5b4e682a5797dbb0d15dad42d642f1c8798`.
   The matching image is published at
   `/Users/thomas/.cache/ze/docker-host/arm64/45cb736f9a13fd62/docker-host.img`.
   H41 reported a native boot with dockerd ready. This session's four fresh
   native RFC records ran on that pair. All named namespace-phase final interop
   runs have now completed, with their qualified receipts below.
   No duplicate build or optional public image feature was added.
4. Namespace-phase live validation is complete; remaining scope is unchanged.
   `checkESPFormChange` in `internal/le/interoplab/ipsec/checkers.go` now rejects
   empty and whitespace-only Ze SA snapshots. Both new missing-state cases
   first failed (`job-ipsec-form-change-missing-state-red-57ce2d65.log`, native
   `job-unit-pkg-7f9efd7e.log`). After the one-branch correction, both new
   top-level tests and their empty/whitespace subtests passed, together with
   `TestESPFormChangeClaimsThreeDirections`
   (`job-ipsec-form-change-missing-state-green-3e1c452a.log`). The full checker
   package also passed, one package OK
   (`job-ipsec-interop-checker-package-a60a1a60.log`). This small
   fix is AC-23 of the existing dual-form spec and landed in `cb5e4a6044`.
   Those focused and package results preceded the subsequent helper wiring.
   The helper and checker repair now has full portable and Linux helper package
   passes and a clean live bare/UDP/bare PASS, as recorded above. The new
   outbound-only template regression first failed at its own assertion
   (`job-ipsec-outbound-template-regression-red-69e830c3.log`); the checker then
   used the directed inbound-state reader, received CLEAN source review and
   passed its full package. The counter-fixture replacement was independently
   approved. The native production-receive goal-break trial is now observed:
   clean PASS in 8s, the as-used binding cut's first bare probe failed in 16s
   with egress=true and reply=false after IKE and Child establishment, then
   post-clean PASS in 8s. The cut receipt is
   `job-ipsec-live-form-switch-binding-cut-de5e18d6.log`; post-clean is
   `job-ipsec-live-form-switch-clean-after-cut-de5e18d6.log` with fresh recording
   `esp-form-change-73ZQMZSGXWCCXBQQR2WCLACH4B.jsonl`. Its AS-USED source SHA256
   is `a47c67ee90560536a43531ebbedc8ed7ad54283870b809d8add1469e80a64e77`.
   The later mutation-marker comment-only revision, SHA256
   `0af0356d5c7ca16099e2a750c856d9f88cd94d678d93a098f3aebcf8c89ca937`,
   is unexecuted. The as-used trial validates the AC-2 goal. Its missing marker
   was procedural and is corrected for future inputs, with no freshness rerun.
   Namespace and permission-diagnostic repairs then landed in `927f93fafe`.
   The rebuilt-source firewall clean/fault/clean sequence is observed:
   `job-ipsec-firewall-namespace-clean-c729df7b.log` PASS 17s,
   `job-ipsec-firewall-gate-open-cut-c729df7b.log` intended packet FAIL 14s,
   `job-ipsec-firewall-clean-after-gate-cut-c729df7b.log` PASS 17s.
   Final independent lifecycle review CLEAR, bounded to the IPv4 single tunnel.
   Each clean loses all three judged probes, moves the drop counter 4→7 and
   strongSwan 328→580 bytes/4→7 packets, and keeps Ze inbound/mismatch zero.
   The established-SA/finding-preserving cut delivers all three probes and
   fails the loss assertion before post-probe drop/mismatch reads, so those
   post-fault deltas are not claimed. All VMs stopped without interruption or
   leftover resources. Receipt `eb52f67f57` landed; main confirmed no open/process
   references and removed the accepted private firewall Go/JSON inputs.
   FRR's own named scenario passed in 5s
   (`job-ipsec-frr-redistribute-current-029e809f.log`), recording
   `ipsec-bgp-redistribute-frr-7GW5D6CTZCNUGK4QGU4RVLZ3ZK.jsonl`. All exits were
   zero, interrupted false, VM stopped and left-behind none on native image `45cb736f9a13fd62`.
   Independent acceptance CLEAR for BGP learning/withdrawal: valid/best
   `10.200.0.0/24` from `172.28.0.2`, AS path `65001`, next hop `172.28.0.2`;
   strongSwan stop removes the prefix while BGP stays Established with one
   connection, dropped zero and accepted zero. Actual JSON `fibInstalled: false`
   supplies no FRR FIB-installation or payload-forwarding credit.
   Post-fix clean `esp-form-change` passed its own named scenario in 8s
   (`job-ipsec-form-switch-after-namespace-89683801.log`), recording
   `esp-form-change-YMJRYRZ2L4LFC4DBJTXCVALXSL.jsonl`. All exits zero,
   interrupted false, stopped VM, left-behind none, native image `45cb736f9a13fd62`. Independent
   lifecycle review is CLEAR. All four directed counters advance 0→46→92→138
   bytes with unchanged identities, bare 1 → UDP 2 → bare 3 wire phases and
   zero mismatch/socket drops.
   Final ICMP clean also passed in 10s
   (`job-ipsec-icmp-after-namespace-98342191.log`), recording
   `icmp-error-payload-check-3QRH2IBDU7IY5M23OU5CFPVIGM.jsonl`, independently CLEAR.
   Consistent errors generate three, inbound ESP 0→336 and Time Exceeded 0→3;
   inconsistent errors generate three more, inbound 336→672, Time Exceeded
   stays three and no error is printed. The temporary peer policy is removed.
   All exits zero, interrupted false, no leftovers, stopped VM, same image.
   No live run remains owed for this namespace phase. Existing discrimination
   for unchanged producers keeps its original credit; no new cut is claimed.
   AC-4 and the 79 renewals remain unchanged. The owner later selected the full
   three-point kernel route, as recorded in the current kernel section below.
   AC-1 is now observed for the current IPv4 single-pair tunnel: clean PASS 10s,
   flag-cut consistent-error FAIL 8s, recovery clean PASS 10s, policy-only PASS 10s,
   state-only PASS 10s, combined-source inconsistent-error FAIL 8s, final clean
   PASS 10s. Logs share `job-ipsec-icmp-` and suffix `-98342191.log`; the exact
   seven stems are `clean-current`, `flag-cut-current`, `clean-after-flag-cut`,
   `policy-only-cut`, `state-only-cut`, `combined-selectors-cut` and
   `clean-after-selector-cuts`. Proof (c) decrypted 336 ESP octets without
   delivering Time Exceeded. Proof (d) delivered three inconsistent errors
   after its consistent control and generation/carriage guards passed.
   Independent final review CLEAR, unchanged production XFRM source, reviewed
   private hashes matched, all VMs stopped without interruption or leftovers.
   Fault lab exits 1/check-command 0; passes all 0. ICMP no longer needs another
   final run. These local-delivery proofs give no general scope, multiple-pair
   or protected inner-fragment credit. AC-4 remains open, and 79 renewals remain owed.
   The installed-endpoint MTU scenario also passed its own named check in 4s
   (`job-ipsec-mtu-installed-endpoint-current-dcac5194.log`), recording
   `mtu-nat-installed-endpoint-4ON24PX74JNHTY3SGYHJZFDUNY.jsonl`. Check/command/lab
   exits were zero, interrupted false, VM stopped and left-behind none, on
   native image `45cb736f9a13fd62`. MTU is removed from current owed runs.
5. The four fresh native records are now observed in
   `rfc/discrimination/rfc4301/RFC4301-5.2-2.json` and
   `rfc/discrimination/rfc7296/RFC7296-2.24-2.json`, plus
   `rfc/discrimination/rfc5082/RFC5082-3-4.json`. They cover the tagged
   claims and leave the broader RFC 4301 partial status unchanged:
   - RFC4301-5.2-2 positive: unit
     `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go::TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`;
     change the final `return total` in `writeESPForm`
     (`internal/component/ike/dataplane/espform.go`) to `return 0`.
     Source review found that the old raw-socket exemption cut reaches a
     prerequisite before the packet rows. The replacement
     produced clean PASS in 5.54s and the intended fragmented-delivery row 3
     FAIL in 4.59s (`job-ipsec-rfc4301-positive-record-6923ef2c.log`).
   - RFC4301-5.2-2 negative: the same unit; change the `0` that `want()`
     returns for `espFormTUNDisablePolicy`
     (`internal/component/ike/dataplane/espform_tun_linux.go`) to `1`.
     Clean PASS in 5.27s became row 4 outside-selector fragmented delivery,
     FAIL in 14.22s (`job-ipsec-rfc4301-negative-record-651671a3.log`).
   - RFC7296-2.24-2 positive: unit
     `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE` (same file); change
     `buf[1] = tos` (`internal/component/ike/dataplane/espform.go`) to
     `buf[1] = 0`. Clean PASS became fragmented TOS 0x02 instead of CE 0x03,
     FAIL in 3.21s (`job-ipsec-rfc7296-ecn-record-364024cc.log`).
   - Observed RFC5082-3-4 re-record: unit
     `internal/component/gtsm/rfc5082_listener_integration_linux_test.go::TestGTSMListenerUnknownNotDropped`,
     producer `listenPortTerm` (`internal/component/gtsm/gtsm.go`); remove the
     `firewall.MatchSourceAddress{Prefix: source}` match from it.
     Clean PASS in 8.91s across all four subcases became three Unknown-SYN
     no-SYN-ACK failures in 19.50s, while the other-port control passed
     (`job-ipsec-rfc5082-unknown-record-239f49a4.log`).

   All four observed records used the same current kernel/image pair named
   above, without skips and with stopped VMs. The subsequent native RFC check
   (`job-ipsec-rfc-record-check-4fea72ad.log`) reported 367 violations, including
   four stale older SAD-selector covers for RFC4301-5.2-2. The transport-positive
   cover was then renewed (`job-ipsec-sad-transport-positive-record-f7512802.log`):
   clean PASS 3.78s, producer-halt RED 0.01s. That is dependency evidence,
   distinct from a packet-assertion break. The transport-negative cover was also
   renewed (`job-ipsec-sad-transport-negative-record-59c8ab79.log`): clean PASS
   3.34s, producer-halt RED 0.01s, again dependency evidence. Both tunnel covers
   were renewed in `job-ipsec-sad-tunnel-positive-record-8cbbeb8e.log` and
   `job-ipsec-sad-tunnel-negative-record-3ca64234.log`: each clean PASS 3.80s,
   producer-halt RED 0.01s, same current kernel/image pair, no skips and stopped
   clean/broken VMs. These remain dependency proofs.

   Independent review read canonical RFC text, all nine covers across six
   functions, and both tunnel logs, and cleared the partial audit. Main natively
   stamped `rfc/audit/rfc4301/RFC4301-5.2-2.json` from the existing pending input.
   The audit keeps `installChildDataplane` as the missing scope producer and
   gives no whole-requirement credit. The subsequent check
   (`job-ipsec-rfc4301-partial-refresh-check-4fea72ad.log`) reports 451 corpus
   findings, with no audit/discrimination freshness finding for this exact ID.
   Its only occurrence is the normal partial report. Other claims on the same
   tunnel unit remain among the 79 unobserved manifest renewals.

   Phase 4 also owes its 13 mutation walks, four
   port-field records and affected audit reseals. They were prepared in
   `phase4-validation.txt` and `phase4-proofs-run.py`, which were lost with
   the session trees, so they are prepared again from the spec.

The owner selected the full kernel route on 2026-11-16: complete exact per-SA
permissions, fragment-context isolation and fragment port-rule enforcement.
No refusal-only route or scope reduction is selected. The earlier receipt,
single-pair bare-ESP and legacy fragment-context passes keep their bounded
identities; they do not establish the new full implementation. AC-4 and the
79 renewals remain open, with scoped checks, RFC refresh and closure still owed.

The engine writer is now authored in `child_scope.go` and `child.go`, with
tests: exact immutable inbound/outbound scopes from expanded directional
policies are validated before either state publishes, transport addresses are
canonical, the bound is 64 and noninitial permission stays false. Its actual
RED/GREEN is recorded in distinct outer logs sharing suffix `d7fcdddc`;
the reused native inner log now holds GREEN 1.501s. Full host packages passed
before the doctor addition: engine 131.808s, dataplane 3.453s. AC-26's fresh
runtime scope check then went RED/GREEN with its own independent CLEAR review;
its broader Go checks remain separately owed. Actual daemon findings are
recorded in the current kernel section below.

## Kernel work still open

The frozen active series now includes `0005-xfrm-fragment-guard.patch`, with
the reviewed transport-boundary correction, conditional ready bit-3
advertisement and guard enabled in runtime config/require. The old proposal
is retained only until native commit removal completes the migration; never
delete it manually or repoint historical plan paths. Core and lifecycle
reviews are CLEAR. Main serializes recipe activation and keeps the patch
directory frozen because native cache identity reads every file there.

The corrected packet probe's old-kernel RED is
`job-ipsec-transport-boundary-packet-red-23bf7d7f.log`: four direct/Destination
Options initial/tail cases had ModeError 0 versus 1 and thread-namespace
ReasmReqds advances. Whole, atomic and outer-ciphertext controls, plus the
separate nested control, passed. Instrumentation uses thread-local SNMP6;
the ciphertext control observes AF_PACKET and exact body, not ReasmOKs.
The complete receipt also correctly failed on old c358/image45cb at features
0x7 versus 0xf. Neither RED proves the new kernel works.

Main's cold native kernel/matching-guest job `ipsec-k123-kernel-guest` completed
with exit 0 (`job-ipsec-k123-kernel-guest-f67febab.log`), new cache
`/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-fe40244f-5d7f6651/vmlinuz`.
No old image was supplied: native runtime-tree/image machinery built the
matching image `47a4c8b74c98c558`. Kernel SHA256 is
`9740ef22ce9ad40cde536908394d33a4d71dede15955113c15c839fe11f62cb9`.
The native Linux 7.3 arm64 build includes modules and guard/IPv6/defrag options.
`TestRFC4301KernelAdvertisesCompleteStateScope` passed its full 0xf receipt
in 0.01s, no skips, all zero exits, stopped VM and interrupted false.
The same old-RED transport binary then passed
`TestRFC4301TransportRejectsAuthenticatedIPv6PlaintextFragments` in 4.92s
and `TestRFC4301NestedTransportKeepsTunnelFragments` in 0.00s
(`job-ipsec-transport-boundary-new-kernel-fcb2493d.log`), including the body's
counter/whole/atomic/ciphertext controls. No skips, all zero exits, stopped VM.
Main's full active-patch/source re-read found no additional issue and hashes
remain unchanged. Full bits 0–3 admission stays intact.

The first 9740/image47a4 profile passed eleven scope-population tests without
skips (`466ea81f`), including all eight ANY rows, K1 readback, multipair/rekey/
bounds and clear-forward. The private K1 port cut produced four wrong-SA
token failures with eight controls (`2459e7a5`), independently CLEAR. This
is whole-packet proof only: NONINITIAL=false remains explicit, but widening
ports broadens effective fragment permission. IPv6 raw-port acceptance passed
all forms/phases in 0.42s (`0a6f6cb6`); old-kernel RED (`26098b15`) stops at
permitted WHOLE send EPERM, with no per-fragment RED claim. Ten PMTU tests
passed without skips (`a8a7fb27`), all exits zero and stopped VM.

Actual daemon probes `d20bb5e4` complete-old and `0c1dd1de` new had one
configured peer, running true, zero SAs/empty SAD: exact old 0x7/0xf cause/fix,
scope code absent on the full profile, all exits zero and daemons/VMs stopped.
Other store findings were ready=false. The earlier old attempt failed its
BusyBox SAD read and supplies no complete-probe credit. Three doctor CIs
passed without skips: internal fast 2.9s, external fast 3.4s, internal removal
4.4s. Fourth external removal passed in 4.0s after its cold 88c98f02 runtime
build (`job-ipsec-doctor-removal-external-k123-7fad9c53.log`): one selected case,
zero skipped phases, all exits zero, interrupted false and VM stopped. Current
kernel SHA256 was `89a57fd6746aa444a1a453feab7c0eece000075f00276fb545cb62312038beb2`, at
`/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-88c98f02-5d7f6651/vmlinuz`;
matching image is `a3933863bf7976cd`. The direct complete-receipt test also passed
in 0.01s (`job-ipsec-current-profile-complete-caps-d4265cb4.log`), with no skips,
all exits zero, interrupted false and VM stopped. Current runtime.config SHA256
is `2cf19c2ccdad059fd27d4e1bb9b8a42725f7c285865de13d43db0a9d032172d2`;
runtime.require is `b56687bcd5990d39d3a23dd86b48a3b52fb5cea7051e74470a80bd0b6d83e136`.

Approved AC-27 lint fixes preserve assertions, packet bytes, tags, claims and
public refusal identity/cause. Earlier host packages (`70092bc3`) passed before
those fixes: engine 119.345s, dataplane 2.334s, diagnostic 1.283s. The distinct
post-style outer `job-ipsec-post-style-host-packages-70092bc3.log` and inner
`job-unit-pkg-0d6c7694.log` passed engine 123.593s, dataplane 2.521s and diagnostic
1.291s. Main observed successful Linux engine `b93836ee` and dataplane `668c4a79`
builds; silent retained build logs do not alone establish their exit status.
The bounded nine-path AC-27 cleanup landed in `a62375a383` (native script
session 32163 exit 0), 22 insertions and 21 deletions, carrying the ordinary
`engine.espPolNoPols` weakening approval. Main verified the commit and clean
targeted status; two other RFC approval rows remain unspent. Scoped lint left
only the independently owned diagnostic formatting finding, whose owner's
targeted retry passed on host and Linux; capability/compile-out stages had no
findings. No wider lint-green or spec closure is claimed. Other outstanding
checks remain owed. Canonical docs retain partial narrative only.

Post-style Linux dataplane units passed 31 top-level tests without skips
(`job-ipsec-post-style-dataplane-linux-units-3641106f.log`): exact scope
capability/parameters/wire, SPD recording, ten forwarding-default cases and
full receipt. Binary `bdaf3541` ran on 89a/a393; check/command/lab exits zero,
interrupted false and VM stopped. This is bounded Go/unit evidence and gives
no runtime credit to the new C continuation repair.

The reached helper population (`job-ipsec-style-reached-linux-population-a644e963.log`)
ran 16 tests: ten PASS, six FAIL. Five cleartext/forwarded-ICMP zero-counter
failures used the shared wrong-namespace `sadSelStat` observer. Exact patch
`fe6c7813` was independently CLEAR for 16 tagged consumers, with native e729
approval rows. The author confirmed application and actual source hash
observer intermediate `711a26fd` was read back. A later authorized comment-only
header correction yields current `cfeea450`, with functions/tags/assertions
unchanged. Main observed formatter `34367ae3` and full-manifest Linux
integration/zetest compile `61adff27` exit 0. Fresh `engine-counter.test`
(`880a1f6d`) passed all five former counter failures in
`job-ipsec-thread-counter-five-linux-39fb9b9a.log`: cleartext 5.85s, four
forwarded-ICMP cases 0.18/0.20/0.20/0.24s. The 16 tagged consumers had 14 PASS
and two FAIL (`job-ipsec-thread-counter-tagged-linux-f6906673.log`): known
compliant-fragment delivery 3.30s, plus ordered inbound admit cleanup ENOENT
at line 53 after all packet checks passed, 1.01s. Noncompliant fragments
passed with exact counter deltas (1,0,0). Both guests used historical 89a/a393,
without skips, and stopped. Five-test check/command/lab exits were zero; tagged
check/lab zero and command one. Native handle 74728 was closed on collection;
no outer exit is inferred. The CI owner's independently approved full-selector
cleanup and shared counter correction are now committed in `714970f5b0`, exactly
two files `cfeea450`/`8b36277e`, both clean in targeted status. Main's native
script exited 0 and carried five RFC/helper trailers; other approvals remain
unconsumed. This session has not pushed. External CI-main receipt
`job-ci-ordered-cleanup-linux-52ac831c.log` passes both ordered tests
11.26/0.51s, packet vectors intact, no skips, all guest exits zero, stopped
89a/a393. This is external CI execution, not our runtime run. Task/AC-28 records
the bounded checkpoint; the known compliant-fragment contract remains red and
safety-blocked. Binary `880a1f6d` predates cleanup, with no fresh-current-binary
claim. The counter-phase commit has landed; main owes ownership/reference/process
checks and cleanup of superseded artifacts, retaining minimum unresolved evidence.

Main reports the exact peer-form source-classification row added and reviewed
by author/main, with existing TUN/reqid rows verified. CI owns coordinated
registry publication; actual commit/receipts remain pending, with no runtime change.

The mapped-return wire failure is separate: protected=0/clear=0 reproduced in
12.05s when isolated (`job-ipsec-mapped-return-isolated-e9eb39b7.log`), same
89a/a393, no skips, check zero/command one/lab zero, interrupted false and VM
stopped. Its expectation stays protected=true and no cleartext. This matches
the existing architecture phase-3 `LL-XFRM-ICMP-TRANSIT` obligation, rather than
showing a new C bug from style changes; locally generated controls passed.

The private fault kernel is unbuilt/unrun. Automatic review rejected follow-up
composite scope-key/transport-output input preparation for possible cybersecurity
risk. Read-only inspection found partial edits in the existing
`scratch/k2-fragment-proof/inputs/patches/`: 0004 clears the scope key; 0005
bypasses the output helper for transport mode in both families. Do not build or
use those partial inputs, retry/rephrase/reassign the rejected preparation, or
use the earlier pure K2 input as an alternate retry. That rejected preparation
changed no production patches; the separate normal repair below is authorized.
Retain the existing blocked artifact only through a safety decision
or approved removal, with ownership/reference/process checks before removal.
The 79 original renewals and freshness of the four earlier renewals must be verified after
the writer change. Neither these passes nor the doc edits complete all three
points, change RFC classification or meet AC-4.

The post-style seven-test packet matrix (`e4293794`) used 89a/88c, image a393
and engine binary `2285ba7c`: five PASS, two FAIL. IPv4 ANY contexts, IPv6 raw
port policies, nested transport/tunnel and both transport-output families
passed. IPv6 ANY lost four permitted same-SA Destination Options rows and
transport receive lost its initial whole control. Isolated receive passed in
5.16s (`b6c76ed5`). Isolated ANY repeated those four failures in 8.68s (`324011f1`),
with all eight base rows, authenticated counts and whole controls correct.
The same-process pair had ANY FAIL 10.19s and receive PASS 2.04s (`a015d255`).
No guest skipped or was interrupted; all stopped, check/lab exits were zero,
and command exits reflected their assertions. The batch receive loss remains
unlocalized: later passes do not prove a fix or causal cross-test leak.

The repeatable Options failure is a normal production defect at
`xfrm6_fragment_prerouting`'s receive continuation. Active 0005 now carries
the first-fragment/control-block cursor repair, preserving processed
Hop-by-Hop state and leaving LOCAL_OUT unchanged, SHA256
`80cf2492bac5b88c302a915fc0fa4b07986add231ad777dcc222202827b36a00`.
Independent C review is CLEAR; the additive first-fragment Hop-by-Hop
regression also has independent CLEAR review. The additive baseline passed in
3.30s (`job-ipsec-hop-options-old-normal-79f077b6.log`), all six both/initial-only/
tail-only Hop-by-Hop arrival-order cases with whole-chain/following controls
and exact SA counts. No skips, check/command/lab exits zero, interrupted false
and VM stopped on 89a/a393, binary `127cef6f`. Main observed formatting
`09ed7233` and test build `ecdb71b4` PASS. This is a preservation PASS, not a
mutation proof or repaired-C acceptance. Normal kernel job 72568
(`job-ipsec-normal-cursor-kernel-6fc0be2f.log`) ended exit 1. The worker compiled
the kernel, installed modules and printed `done: /out/vmlinuz`; the final host
`enforceKernelRequirements` then rejected missing built-in `CONFIG_QFMT_V2`.
Shared base config/require and `kernelreq.go` changed at 17:26:56 during the
17:04:51 build, in `faa6297d78`. Worker validation reads live requirements before
make; host validation rereads them after build. Base `2bef4e61`/`57692fbb` became
`be2894ce`/`6990de48`; runtime `2cf19c2c`/`b56687bc` and patch `80cf2492` did not
change. The quota/QFMT dependency is valid, with no requirement weakening.
Native cleanup removed rejected `tmp/kernel/runtime-build-2183217670`; target
cache 265a is absent. That attempt produced no recoverable/published repaired
kernel or image. Original Options remains natural production RED on historical
pre-repair 89a/88c.

The subsequent two-job `9c693ee4`/session 82844 completed exit 0 and published
accepted cache 6a87, kernel SHA256
`1b3b839f751e6ab9f69a896d15847c07c9f8b7e2a24f07f934cf7fb029076ab3`.
QUOTA/QFMT_V2/XFRM_FRAGMENT_GUARD read back as built in; modules `7.3.0-ze`
and kernel.version match Linux 7.3 runtime/arm64/Docker. Matching image
`913df5db17ebb20a` first boot (`c6c943c2`) passed complete receipt 0.01s,
original IPv6 ANY 16-row matrix 6.68s (all four former Options positives true,
all mixed-SA false), and HBH six cases 0.15s with whole/following controls and
SA counts. The eight-test packet matrix (`4e681aeb`) passed all eight using
engine `880a1f6d` and dataplane `bdaf3541`. Both guests had no skips, all exits
zero, interrupted false and stopped VMs on that accepted pair. This accepts
the cursor repair, not full three-point conformance or AC-4 closure. The original
seven-test population then passed five times in one process: 35 top-level PASS
results and no FAIL or SKIP (`job-ipsec-corrected-rx-stability-c25ff001.log`,
main session 13208 exit 0). The same accepted pair, count 5, shuffle off,
original eight-minute test timeout and all packet deadlines/assertions were
preserved; check/command/lab exits were zero, interrupted false and VM stopped.
The earlier RX cause remains unlocalized; these passes do not establish its
cause or a fix. Both safety blockers, 79 unexecuted renewals, four historical
records' unknown current freshness and phase-3 mapped-return gap remain.

The dual-form spec's Task/AC-29 now records ordinary preservation regressions
for the existing captured raw-sender authority contract in
`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, under Decisions,
“Fragment gathering”: `xfrm_raw_route_capture`,
`xfrm_policy_receipt_capture` and `inet_frag_raw_authority`.
`engine/raw_socket_fragment_policy_integration_linux_test.go` is authored
and independently reviewed CLEAR. It covers socket-policy presence/miss/absence,
route authority, generation and lifetime with whole and same-authority controls.
Formatter session 6402 exited 0 (`e9d970e8`), source `71d4` unchanged. Linux
build session 80197 exited 0 (`af85e020`, silent log; terminal supplied by main),
producing static arm64 binary SHA256
`b73f7299f689254045b7fb67fd32519e5bd29c5e9f2efa0ce738e562348a5145`.
Guest session 85260 exited 1 (`job-ipsec-raw-authority-packets-be4fca5b.log`):
IPv4 PolicyAuthority's matching whole send and RouteAuthority's admitted-mark
whole send returned `EPERM` with fallback BLOCK. All three IPv6 subtests and
IPv4 PolicyLifetime passed. No SKIP; check 0/command 1/lab 0, interrupted false,
VM stopped on exact kernel `1b3b839f`/matching image `913df5db17ebb20a`.

Four diagnostic lenses cleared fixture, initialization and encoding. Main read
canonical Linux 7.3: `ip_route_output_key` calls `ip_route_output_flow` with a
null socket and performs XFRM lookup, so it is wrong for both intended ordinary
IPv4 route stages. The normal source repair replaces those two calls with
`__ip_route_output_key`, in `raw_sendmsg` and `xfrm_fragment_route`; final
captured lookup and all guards stay unchanged. Active 0005 SHA256 is now
`235e022ad1b9b1725db8e62852e955a7a3109dc35c2dc6b3492ef7d2cbf43854`, independently
reviewed CLEAR, with targeted runtime acceptance recorded below. Resolver session 72041
passed (`a5417483`) and names actual candidate d87a6908; its vmlinuz was absent
before launch. All eight roots acknowledged input and Colima compiler holds
before main launched normal two-job build 29078, label
`ipsec-ipv4-raw-route-kernel`, PID/PGID 23658 (`9c693ee4`). It completed exit 0
and published actual kernel SHA256
`d2eb6a14cd07c913deb1a0e9f7ff667230982b64c597ebf8fbee8bc3f80d8f65`.
Firstboot session 3966 exited 0 (`job-ipsec-ipv4-route-first-boot-631e2522.log`),
building and booting matching image `43a1d1b2ecccc59c`, module `7.3.0-ze`.
Complete receipt passed 0.01s; unchanged `b73f` PolicyAuthority/ipv4 passed
1.24s and RouteAuthority/ipv4 passed 0.94s, including former whole failures.
No SKIP; check/command/lab 0, interrupted false and stopped VM. This accepts
the two-call C route repair for those failed cases. Main released input and
Colima compiler holds to all eight roots after actual readback. Full three raw
tests, existing eight-test matrix and the new SA-lifetime regression remain
unrun on this pair. Earlier eight/35 passes retain bounded source/kernel
identities and do not close AC-29/full AC-4.

The preparer's metadata-only stop adds no owner restriction: main may separately
execute the existing 13 ordinary host reverts and 11 existing host mutants
after its phase boundary, retaining selected clean PASS/attributable RED
requirements. The actual two safety rejections remain. All 79 renewals remain
unexecuted/four freshness UNKNOWN. The 55 guest commands still name d87a6908
and remain UNEXECUTED; the 6a87 observations remain historical. The owner's
question about both rejected tasks is still PENDING, not approval.

Existing AC-4 queue lifetime now has ordinary source regression
`TestFragmentSAReplacementKeepsQueuedIdentity` in
`engine/fragment_sa_lifetime_integration_linux_test.go`: same-public-identity/key
replacement cannot complete the old queue, with whole/fresh-generation controls,
both families/orders, old totals 2/6, first request increment +1 and B counts
0/1/2/4. Source `d030` is independently CLEAR but formatting/build/runtime are
owed; these are assertions, not observations or new RFC claims.
Native C-discrimination remains unsupported; the existing P3/post-release
kernel-route design needs REWORK and owner design admission before tooling or
C-proof credit. Read-only research is not implementation, an escape or a Go
dependency proof of the C primitive.

The earlier space blocker was resolved after all roots acknowledged a quiet period; main's
supported coordinated cleaner completed with exit 0, freeing 67.4G of checkout
Go cache and 0.4G of lint cache, with 80.1G free
(`scratch/ipsec-coordinated-cache-clean.log`). Valid patched GOROOTs were kept,
and the built pre-repair 88c kernel and a393 image remain preserved. Main
released all roots after completion. Retain those native artifacts for the unresolved
current-profile proof through their owning cleanup mechanisms. The private
fault-preparation safety blocker remains unchanged; no fault kernel has built
or executed. The existing state retains the minimum cleaner receipt through
the related phase commit, then reference/ownership cleanup. That 80.1G is the
first historical cleanup snapshot. Second supported cleanup
(`scratch/ipsec-coordinated-cache-clean-second.log`) reports 55.0G checkout and
0.3G lint freed, with 74.9G free at cleanup. Its execution 72928 was already
closed when recovered; no exit code was recovered or inferred. Main sent RELEASE
to all eight roots. The current-floor host driver passed with code 0 on Darwin
arm64/CGO disabled (`job-ipsec-current-floor-host-driver-ae60c218.log`). A temporary
unrelated BGP compilation blocker was corrected by its owner. Main's fresh
admission report then passed: one of four slots occupied, no waiters, 25.6GiB
available and 52.1GiB disk free. Normal kernel session 81417 then ended exit 1
(`job-ipsec-current-floor-cursor-kernel-799a7dd1.log`): gcc `cc1` was killed
compiling Intel `ice_common.o`, and make exited 2. Guard/quota objects compiled
earlier; no completed/published 6a87 kernel or image exists. Main's fresh Docker
info reports 2 CPUs and 2,054,840,320 bytes of memory (about 1.9GiB), which is the
builder budget the host admission figures did not measure. Main selected four
jobs too aggressively; builder-VM dmesg confirms OOM killed `cc1` in this build's
owned container. Use two jobs for the next normal retry, as the earlier
worker compilation did. The old hold was released; root job 82844/PID 75336
completed exit 0 as native key `9c693ee4`; its retained
`job-ipsec-two-job-cursor-kernel-9c693ee4.log` ends with ready cache 6a87.
Accepted kernel/image receipts are above. All eight roots received RELEASE of
input and Colima compiler holds. Main's exact-ID listing
found neither old owned container.
Main verified output `tmp/kernel/runtime-build-2858380835` was empty and removed
it with `rmdir`, exit 0. That accepted 6a87 build used patch/base hashes
`80cf2492`/`be2894ce`/`6990de48`; the later raw-route source repair is recorded
above and is not yet built.
Owner namespace dd6e maps to a shared Rex fallback session; unique intervening
writer attribution remains unconfirmed.

The original fixture/testing blocker remains: automated review stopped concrete
preparation of the existing fragment-context fixture correction, citing
"possible cybersecurity risk". No patch or approval exists and the test is
unchanged. The judge assessed design only: noninitial=false cannot support
those same-SA fragment positives under RFC 4301 Section 7.3. Do not retry,
rephrase or reassign that rejected task. Unaffected new tests and K3 work
continued; this blocker must stay explicit in the remaining proof population.

Old Ze-kernel probes also exposed mixed-protection fragment delivery and
general multi-pair/overlapping SA-scope leaks. Forwarded ICMP return under
`unmatched discard` remains red; the strict final-policy marker design was in
`rfc4301-icmp-final-marker-design.txt`, lost with the session trees, so it is
written again before that work starts. Do not replace it with an ambient ICMP
allow rule. The saved new probes must pass against the finished kernel.

## Uncommitted and separately owned work

The RFC spec's scope and publication reconciliation edits are committed, in
`6c287d1c9c` and `6e66c5b80a` (2026-11-14).

The seq-4 expected-error hunk in
`test/parse/ipsec-icmp-type-selector-refused.ci` landed in `34fbeadffe`.
Numerous other sessions' changes share this checkout. Do not stage, remove or
overwrite those changes.

Owed: reconcile AC-34 of
`plan/release/validation/spec-doctor-coverage-3-access-security.md`, which
still names `doctor-ipsec-espform-receiver` with a reload fix. The reader
reopen and its findings (`doctor-ipsec-esp-form-tun`,
`doctor-ipsec-esp-form-reader-stopped`) belong to the dual-form spec, as the
coordinator decided on 2026-11-12.

The three separately named fix specs remain open:
`spec-privilege-drop-keeps-capabilities`,
`spec-anomaly-shape-limit-drops-nothing`, and
`spec-ike-peer-info-reads-child-after-unlock`.

## Transfer and recovery

Carry the committed branch through the owner's chosen transfer route; pushing
still needs his order. Git alone does not carry ignored proof artifacts.

The two session trees, `tmp/session/2026-11-10-225434/` and
`tmp/session/2026-11-09-222726/` (state files, mutant JSON,
`ipsec-close-validation.txt`, scratch scripts and raw logs), exist on neither
the Mac nor the `claude` host (checked 2026-11-15). The lists in "Remaining
work" above are now their only record. The state files were named
`session-state-ipsec-dual-form-receive-spd-bypass-c43b4620-2510-44dd-8936-84100b444bb5.md`
and `session-state-rfc4301-architecture-gaps-c43b4620-2510-44dd-8936-84100b444bb5.md`.
The main harness identity was `c43b4620-2510-44dd-8936-84100b444bb5`, commit
namespace `79a29bf3`. A resumed session uses its own identity and native claim;
inspect the generated `Spec-work` trailer because the old claim drifted.

The pinned archive is `tmp/qemu/build/x86_64/linux-7.3.tar.xz`, SHA-256
`87363c24f10e06321eb362765521094e94471f6bff8bce2d6b3e40f39f004953`.
The third-party work and immutable snapshots were under scratch paths beginning
`rfc4301-kernel-`; none is in this Mac checkout's `tmp/` (checked 2026-11-15),
and the archive is not here either. Where they survive on another host,
preserve the proposal plus manifests before any cleanup.
No deletion or cleanup was authorized by this checkpoint request.

Cached kernel/image and executable paths in the command file are machine
specific. The old baseline used Ze 7.3 on amd64 with KVM. Rebuild on the new
host's native architecture through the registered actions and record kernel
identity; copied old binaries do not prove the current source. Tests run in
the main checkout, one CPU-intensive job at a time, with no `./le verify` run.
