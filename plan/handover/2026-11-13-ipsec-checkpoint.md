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
| `5c063bc8d6` | K1 exact state-scope and K2 fragment-context kernel patches; no native C compilation or packet proof |
| `2ae4f0f3f4` | K3 draft preserved at `gokrazy/kernel/proposals/xfrm-fragment-reassembly.patch`; unenrolled and unproven |

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

## Remaining work, in order

1. Done: the private-ABI move landed in `8afd1ff560` (2026-11-13), and
   `plan/post-release/engineering/spec-netlink-xfrm-upstream-series.md` records
   it as AC-5 and AC-6 met. That spec's closure owes the scope install and
   read-back integration tests on Ze's kernel, which the move did not run.
2. Compile and run fresh final guest binaries for dataplane, engine and
   trafficusage, and require each named test and required subtest to run.
   The guest set, every name checked in HEAD on 2026-11-15:
   - dataplane (`internal/component/ike/dataplane/`): `TestESPFormRewriteProgram`
     (`espform_program_integration_linux_test.go`),
     `TestESPFormFastPathGateFollowsTheKernelRuleset`
     (`espform_fastpath_gate_linux_test.go`), `TestESPFormNFTWatchReceiveBuffer`
     (`espform_firewall_linux_test.go`), the TUN node tests
     (`espform_tun_open_linux_test.go`), the unit-capability tests
     (`unit_capabilities_integration_linux_test.go`), every
     `TestESPFormFirewallStock*` (`espform_firewall_stock_linux_test.go`, five),
     the migrate, teardown and form-change integration tests
     (`xfrm_migrate_integration_linux_test.go`,
     `xfrm_teardown_integration_linux_test.go`,
     `encap_formchange_integration_linux_test.go`),
     `TestESPFormFastPathSecondSATakesOver` and
     `TestESPFormFastPathWarnsOfASharedSPIWhileAttached`
     (`espform_fastpath_integration_linux_test.go`);
   - engine (`internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go`):
     `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`,
     `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE`,
     `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff`;
   - trafficusage (`internal/plugins/trafficusage/`): `TestProgram_TruncatedPass`
     (`program_test.go`), the tests in `tcx_next_integration_linux_test.go`, and
     the unit-capability tests (`unit_capabilities_integration_linux_test.go`).

   Then run guest mutant 20. Its definition was entry 20 of
   `b9-mutants.json` in the session tree that no longer exists (see "Transfer
   and recovery"), so write it again against the current source before running
   it. Regenerate changed mutation coordinates rather than accepting
   stale-hunk failures as red evidence.

   Still owed on the final code for the spec's closure evidence: the dataplane
   package in its zetest flavour; the `./le arch` gates compound-guard,
   refuse-by-default, error-wrap, operator-error and doctor-coverage; the
   scoped lint on `./internal/component/ike/...` and
   `./internal/core/diagnostic/...`; gofmt on the edited files. A lint or gate
   red is owed at release through CI and holds no commit.
3. Run the doctor draft at
   `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, prove all 80 stress
   iterations with the documented selected-test harness, promote through
   `ze-test`, and run the promoted test. Final stress and promotion are owed.
   The draft is git-ignored and exists only in the checkout on the `claude`
   Linux host (confirmed 2026-11-15; absent on the Mac), so run it there or
   carry it by hand before the session that runs it. Steps 1 to 8 passed in
   batch 10. Step 9's regex expects the sorted keys `code`, `fix`, `message`,
   `severity`, because `applyJSON` re-encodes the answer; that alignment has
   not been run.
4. Run the remaining interop scenarios and their deliberate breaks:
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
   The mutants, each site checked in HEAD on 2026-11-15:
   - RFC4301-5.2-2 positive: unit
     `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go::TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`;
     replace the `espFormBypassInboundPolicy(conn)` call
     (`internal/component/ike/dataplane/espform_open_linux.go`) by `error(nil)`.
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

Phase 2 is needed after those five work packages: finish and prove the kernel
fragment/SA-scope work, strict ICMP return mapping, and engine scope writer;
then complete scoped review, RFC checks and the two spec-closure commits.

## Kernel work that remains source only

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
