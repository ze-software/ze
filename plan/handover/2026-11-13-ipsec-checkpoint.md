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
| Affected live IPsec suite before the handoff fix | 48 cases: 34 PASS, one FAIL (`ipsec-childless-ike-sa-recovers`), 13 Linux skips. Both promoted doctor cases passed on the unmodified DUT. The later isolated childless PASS does not establish a full-suite PASS. |
| `./le --name session doc check all` | FAIL: 389 generated website/wiki command-catalog drift findings and 12 source-anchor findings in interface, PPPoE and web pages. None of those anchors names an IPsec path. The foreign findings remain with their active owners. |

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
are in the parent state. After this receipt commits, main checks active processes
and references before removing those completed binary copies. The earlier
standalone packet, negative-test and private cut-DUT binaries were already removed
after their receipts committed. Keep the small qualification and fault logs through
the spec's closure review. Their owning state records the removal conditions.

## Remaining work, in order

1. Done: the private-ABI move landed in `8afd1ff560` (2026-11-13), and
   `plan/post-release/engineering/spec-netlink-xfrm-upstream-series.md` records
   it as AC-5 and AC-6 met. That spec's closure owes the scope install and
   read-back integration tests on Ze's kernel, which the move did not run.
2. The historical guest mutant 20 remains unrecovered. Its definition was
   entry 20 of `b9-mutants.json` in the lost session tree (see "Transfer and
   recovery"). The four fresh clause-scoped candidates below do not assert
   that identity. Their native records remain owed. Regenerate changed mutation
   coordinates rather than accepting stale-hunk failures as red evidence.

   Main ran the five changed-line architecture checks. Operator-error passed;
   compound-guard, refuse-by-default, error-wrap and doctor-coverage reported
   findings outside IKE, routed to their live owners. The parent state records
   each receipt. These are not whole-gate green claims. Scoped IKE and diagnostic
   lint findings remain to resolve; new source edits owe their scoped checks.
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
   H41 reported a native boot with dockerd ready. This session's native RFC
   records and interop have not run on that pair.
   No duplicate build or optional public image feature was added.
4. Finish the interop checker before the remaining live scenarios.
   `checkESPFormChange` in `internal/le/interoplab/ipsec/checkers.go` now rejects
   empty and whitespace-only Ze SA snapshots. Both new missing-state cases
   first failed (`job-ipsec-form-change-missing-state-red-57ce2d65.log`, native
   `job-unit-pkg-7f9efd7e.log`). After the one-branch correction, both new
   top-level tests and their empty/whitespace subtests passed, together with
   `TestESPFormChangeClaimsThreeDirections`
   (`job-ipsec-form-change-missing-state-green-3e1c452a.log`). The full checker
   package also passed, one package OK
   (`job-ipsec-interop-checker-package-a60a1a60.log`). This small
   fix is AC-23 of the existing dual-form spec and awaits its separate commit.
   The checker still lacks a mid-session form switch and still assumes three
   ESP directions and a rising mismatch counter. AC-2's form-switch and counter
   repair remain owed. Then run the scenarios
   and their deliberate breaks:
   `mtu-nat-installed-endpoint`, `icmp-error-payload-check` with bare ESP and
   both redwalks, `esp-form-change`, `esp-form-firewall-drop` with its redwalk,
   and `ipsec-bgp-redistribute-frr`, each with
   `IPSEC_INTEROP_SCENARIO=<name> ./le test integration interop-ipsec`, one at
   a time. The `esp-form-firewall-drop` red: make `publishFirewall` ignore a
   blocked verdict so the gate is forced open, expect the scenario's ESP-drop
   check to go red, then restore. Earlier success is not final-code evidence.
5. Produce RFC4301-5.2-2 positive and negative records, RFC7296-2.24-2
   positive, and refreshed RFC5082-3-4 positive. First take a fresh
   `./le rfc discriminate stem <stem> report <path>` for `rfc4301`, `rfc7296`
   and `rfc5082`, and regenerate line/column selectors from current source.
   The prepared candidates all remain unobserved:
   - RFC4301-5.2-2 positive: unit
     `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go::TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`;
     change the final `return total` in `writeESPForm`
     (`internal/component/ike/dataplane/espform.go`) to `return 0`.
     Source review found that the old raw-socket exemption cut reaches a
     prerequisite before the packet rows. The replacement
     and its expected fragmented-delivery failure remain unobserved.
   - RFC4301-5.2-2 negative: the same unit; change the `0` that `want()`
     returns for `espFormTUNDisablePolicy`
     (`internal/component/ike/dataplane/espform_tun_linux.go`) to `1`.
   - RFC7296-2.24-2 positive: unit
     `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE` (same file); change
     `buf[1] = tos` (`internal/component/ike/dataplane/espform.go`) to
     `buf[1] = 0`.
   - RFC5082-3-4 re-record: unit
     `internal/component/gtsm/rfc5082_listener_integration_linux_test.go::TestGTSMListenerUnknownNotDropped`,
     producer `listenPortTerm` (`internal/component/gtsm/gtsm.go`); remove the
     `firewall.MatchSourceAddress{Prefix: source}` match from it.

   Then `./le rfc check`. Phase 4 also owes its 13 mutation walks, four
   port-field records and affected audit reseals. They were prepared in
   `phase4-validation.txt` and `phase4-proofs-run.py`, which were lost with
   the session trees, so they are prepared again from the spec.

Section 7.4 release-enforcement choice remains open in
`plan/release/fixes/spec-rfc4301-architecture-gaps.md`, "Owner decisions open".
The selected kernel receipt and legacy fragment-context probes passed.
Broader SA-scope installation, kernel fragment work, strict ICMP return mapping and the
engine scope writer remain unfinished. Scoped review, RFC checks
and both spec-closure commits remain after the applicable implementation and proof.

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
