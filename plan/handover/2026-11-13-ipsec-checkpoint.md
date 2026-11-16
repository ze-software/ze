# IPsec checkpoint for continuation, 2026-11-13

Branch: `main`. Both specs remain `in-progress`:

- `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
- `plan/release/fixes/spec-rfc4301-architecture-gaps.md`

The owner asked to commit as much existing work as possible and then report so
he can decide whether to continue here or move machines. This is a checkpoint,
not closure. No push was ordered. Dual-form AC-4 is unchanged: RFC4301-5.2-2
must lose `{partial}` only when the broader obligation is actually proven.

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
   native RFC records ran on that pair. Final interop remains owed.
   No duplicate build or optional public image feature was added.
4. Finish the remaining live scenarios.
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
   leftover resources. Retain the private cut until receipt commit and checked cleanup.
   Run `ipsec-bgp-redistribute-frr` and post-fix clean `esp-form-change` and
   `icmp-error-payload-check` lifecycle validation, each with
   `IPSEC_INTEROP_SCENARIO=<name> ./le test integration interop-ipsec`, one at
   a time. FRR is running under `ipsec-frr-redistribute-current`, with no result
   yet. The post-fix clean runs do not repeat the unchanged seven-run ICMP cuts.
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

Section 7.4 release-enforcement choice remains open in
`plan/release/fixes/spec-rfc4301-architecture-gaps.md`, "Owner decisions open".
The selected kernel receipt and legacy fragment-context probes passed.
Broader SA-scope installation, kernel fragment work, strict ICMP return mapping and the
engine scope writer remain unfinished. Scoped review, RFC checks
and both spec-closure commits remain after the applicable implementation and proof.

The remaining RFC4301-5.2-2 producer is `child.go::installChildDataplane` under
`internal/component/ike/engine/`. In tunnel mode it binds `inbound.Sel` only
when `tunnelStateSelectorSupported` accepts one concrete same-family policy.
It never sets `inbound.Scope`, and the engine has no `NewSAStateScope` caller.
The new bare-ESP records prove single-pair whole and outer-fragmented receipt
under the production SPD. General multi-pair and overlapping per-SA isolation
remain unfinished, with the full feature-bits 0-3 guard intact and K3 unenrolled.
The partial summary now names that gap instead of claiming discrimination records
are missing. Its RFC quote and tested/gap selector spans are unchanged.

## Kernel work still open

The active kernel series contains 0001–0004. K1/K2 advertise only feature
bits 0–2. Bit 3 and the full-guard runtime configuration remain disabled.
Do not claim full capability from these patches or from the Go unit tests.
Freeze the patch directory during a kernel build: the native cache identity
hashes every file there, not only enrolled series entries.

K3's receipt/reference/accounting review found no issue, but the route review
found a source-reachable transport defect: authenticated IPv6 transport ESP
can carry plaintext Fragment headers and reach reassembly. Correct the current
authenticated transport-state boundary before reassembly and output transform.
Preserve legitimate outer ciphertext fragmentation and distinguish atomic
fragments. Do not reject every earlier transport entry in a nested secpath.
No fix or runtime probe for this finding has been implemented.

The source-only audit identifies the next boundaries: K3's
`xfrm_state_noninitial_allowed` ignores mode, and its gather checks inspect
noninitial offsets. It adds no GETDEFAULT bit-3 advertisement, and neither
`gokrazy/kernel/runtime.config` nor `runtime.require` enables
`CONFIG_XFRM_FRAGMENT_GUARD`. The current capability-probe test requires only
bits 0 and 1, so its pass is a partial receipt. The cached upstream archive
`tmp/qemu/build/aarch64/linux-7.3.tar.xz` is now present; earlier absence claims
remain dated history. This audit supplies no C compilation or packet proof.

Keep one author on the whole K3 proposal file. The engine scope writer belongs
in `child_scope.go`, `child.go` and their tests: derive inbound and outbound
scopes from expanded directional `childPolicies`, validate before the first
state publication, retain the 64-record bound and noninitial=false, and keep
the full bits 0–3 guard. An independent author owns packet tests; main serializes
series/config activation. The Section 7.4 route decision remains unchanged.

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
