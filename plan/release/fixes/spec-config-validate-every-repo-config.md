# Spec: config-validate-every-repo-config

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P1 |
| Release reason | RFC 8212 correctness: configs the repository ships and drives (perf DUT, demos, interop scenarios, netlab goldens, `.ci` bodies) left EBGP directions closed and exchanged no routes, and nothing caught it. `ze config validate` now warns (e2af3bbac6); every config the repository uses must pass it |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5 - closed: runtime proof complete, Review Gate clean (see Review Gate) |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas asked: "we should have a test running validate on all the configs
generated / used". An RFC 8212 correctness defect decides that this runs (no
owner approval needed): the perf lab DUT config stopped carrying routes when
RFC 8212 enforcement landed, and no test loaded it to notice.

Already landed:

| Commit | What |
|--------|------|
| e2af3bbac6 | `ze config validate` warns `config-bgp-rfc8212-no-policy` for each EBGP direction RFC 8212 closes (exit stays 0) |
| 1611f99e41 | `TestRepositoryConfigsValidate` (`internal/component/config/cli/validate_corpus_test.go`), red at landing with 593 findings |
| d9a3f94bc5 | 13 standalone configs state their policy (593 to 556 findings) |
| 664be57056 | `test/perf/configs/ze.conf` states its import and export policy |

This spec finishes the work: the sweep's over-reports are removed by derived
rules (never a list), every real defect it names is fixed, and the sweep is
green with zero findings. The 556 findings of the last run are in the session
scratch `tmp/session/2026-10-31-204407/scratch/sweep-findings-556.txt`; every
one is accounted for in Acceptance Criteria (the per-bucket counts sum to 556).

### Implementation record (2026-11-02)

On November 1, `./le spec claim` refused the claim at the WIP cap. On
November 2, Thomas requested implementation and said "ignore spec limit".
The parent claimed this spec with `ZE_SPEC_WIP_CAP=24`; this is a one-spec
exception, not a change to the standing cap rule.

The next table preserves the November 1 record. Its outstanding-work statements
are superseded by the final proof record below. Historical export/worktree runs
remain historical evidence, not current main-checkout passes.

| AC | State | Commits | Evidence |
|----|-------|---------|----------|
| AC-1 | Linux: one finding left, not this spec's: `test/plugin/filter-wire-thirdparty-compiled-in.ci` (dabe5bd47a, the filter-wire session) has two EBGP peers with no export policy. macOS run owed | c7f5ce38b6, 5f65d1f35c | `go test -tags <full set> -run TestRepositoryConfigsValidate$ ./internal/component/config/cli/`: validated 2778 Ze configs and 241 migrated ExaBGP configs; 20 declared refused and asserted refused; 487 skipped by a non-zero exit expectation; 29 load sequences (71 steps); 80 legacy native fixtures logged with their reason; host verdicts skipped: none on Linux |
| AC-2 | Done | - | 664be57056 reverted in the working tree: the sweep names `test/perf/configs/ze.conf` four times with `config-bgp-rfc8212-no-policy` (perf-receiver and perf-sender, import and export); restored: the file is in no finding |
| AC-3 | Done (Agent B), extended to the daemon configs the sweep now reads | 6ca76b741b, 99839b2060, 86280b3a20, cf5d6b8c9e, 41f87f7d4a, 92ee85d979, f194d2760c, 3cb09bb0a9, ff15eecba0 | 302 `.ci` body findings, then 328 daemon-config findings, to 0; 3cb09bb0a9 carries the judged `expect=` of the three RFC 8212 subject tests |
| AC-4 | Template and goldens hand-written; `./le test netlab check` owed on a netlab host | 6414ab2180 | - |
| AC-5 | Done | 01e6f725b7, c7f5ce38b6 | `internal/test/exabgpnative` is the one classification; `TestParseAllConfigFiles` passes; three plugin-only `api-` fixtures (no bgp block, so no reactor) are their own class and validated by the sweep |
| AC-6 | Done on Linux; the Mac run is AC-1's | 5f65d1f35c | `TestValidateReportsNodeOfAnotherOS` red with `nodeOfAnotherOS` disabled, green with it; code `config-node-other-os` |
| AC-7 | Done on Linux; the Mac run is AC-1's | 5f65d1f35c | `TestValidateReportsHostBackendVerdict` red without the `IsHostVerdict` mapping, green with it; code `config-host-backend` |
| AC-8, AC-9 | Done | c782411791, c7f5ce38b6 | `internal/test/labconfig`; RFC-approved rows for `checkRFC2545NextHops` and `checkVPNFRR` |
| AC-10 | Done | c7f5ce38b6 | statement-shape rule; the three `vtysh.conf` return with it off (AC-22) |
| AC-11 | Done | c7f5ce38b6 | `validate_corpus_plugins_test.go` links `internal/test/plugins/all`; 17 findings return with it off |
| AC-12 | Done (Agent C, migration) | 794c3ac052, 7c85bafe4a, 88e7a340ab, ce9da8c565 | the two conf-flow and the bridge neighbor migrate outputs validate |
| AC-13 | Done | 84c0fc6cea, c7f5ce38b6 | tapes expand their Source chain; every demo fragment is judged loaded onto `showcase-lab/ze.conf` in tape order; only `irr/1-server.conf` needed the top-level `policy` root |
| AC-14 | Done | 3648c2c9d2 | - |
| AC-15 | Done | c7f5ce38b6 | the four `.ci` merge fragments validate merged through the editor's LoadMerge |
| AC-16 | Done | 698a7a547a, 6f31718a91, ece2daa9de | `refused=<code>` on tmpfs and stdin blocks; 20 bodies asserted refused; reload runs listed in the commits; `static-bfd-profile-reload-deleted`, `-passive` and the two `tx-iface-wireguard-*` need `./le test qemu all` |
| AC-17 | Done | 84c0fc6cea, 9e74e41c06 | parse and ui runs listed in 84c0fc6cea; `lg-graph-lab/ze-bgp.conf` had no reader and was removed |
| AC-18 | Done | 84c0fc6cea, c7f5ce38b6 | `policy-root-migrate.ci` `old.conf` validated as the `ze config migrate` output |
| AC-19 | Done | 5dc57599fb | `TestRecordPortExpansionRefusesUnreservedPort`; bgp-rpki-added-live, bgp-rpki-attach-live, ipsec-added-live, reload-listener-rejected, bmp-lg-http-routes pass (ze built from HEAD) |
| AC-20 | Done | d3345c8082 | - |
| AC-21 | `./le arch tier check`: no refusal names the sweep's imports; the check is red for `internal/component/bfd/api` imports, not this spec's. Scoped lint owed: `./le go lint run scope ...` over the changed packages stopped three times with "lint inputs changed during planning" while other sessions edited the shared tree | - | - |
| AC-22 | Done | c7f5ce38b6 | `TestSweepDerivesEachExclusion`: placeholders 40, ports 242, test plugins 17, declared refusal 1, load sequence 20, migrate input 1, native classification 80, statement shape 3, host verdict 11 (schema built as plan9) |

Test-judge: test/ui/cli-config-dump-{sensitive,json-sensitive,strip-private}.ci: (a) adding local ip auto and import/export final accept leaves every assertion (exit 0, router-id / $9$ / SECRET-DATA) and the masked md5 password in the dump; all three pass and their bodies now validate.

The AC-1 macOS run needs a macOS host and remains unproven. The owner's
standing release rule leaves macOS-only checks until after the first release.
The final proof record below accounts for the Linux selections individually.

Found, owned elsewhere: the exabgp-compat mock stops comparing an allowed case at its first divergent frame (`plan/release/validation/spec-exabgp-compat-allowance-checks-every-frame.md`, 660ec70b4b).

### Initial proof recovery (2026-11-02, superseded)

The November 1 table is historical evidence, not a verdict on the current
shared tree. `spec state latest` returned no file; the referenced October 31
session directory is absent. Commits recover the following additional record.
The old AC-1 filter-wire finding is no longer the same input:
`test/plugin/filter-wire-thirdparty-compiled-in.ci` now declares `export {
final accept; }` for both peers. This source correction does not establish a
zero-finding verdict for the rest of today's corpus.

| AC | Recovered or current evidence | Remaining action |
|----|-------------------------------|------------------|
| AC-1, AC-22 | The current `runSweep`, `checkBody`, `checkRefused` and `requireRuleTurnsFindingsAway` in `validate_corpus_test.go` retain the corpus, refusal and differential checks. The November 2 race run reached the corpus test but the tool's 120-second timeout terminated it (exit 143); no corpus verdict or exclusion-test run was produced | A dedicated proof agent runs these two tests to completion with an adequate tool timeout; report every finding and host-code count |
| AC-2 | The earlier table records the perf-config mutation red and restoration; its original scratch output is unavailable | Preserve the historical claim; do not call it a newly observed mutation proof |
| AC-4 | 28bd5b5c9c records `./le test netlab check` passing render and validation for all three nodes under netlab 26.10, and the independent golden-order judgment. Current `Checker.Run` calls `compare` and `validateGolden`; the latter still checks exit status, not warnings | This supersedes the old claim that the netlab check was never run; no repeat solely for a fresh timestamp |
| AC-6, AC-7 | Newly observed `--- PASS` for `TestValidateReportsNodeOfAnotherOS` and `TestValidateReportsHostBackendVerdict`, including marked and unmarked subtests, under the full feature tags and `-race` | Linux unit proof only; injected OS is not a macOS execution |
| AC-21 | f7d8710ee8 fixes six lint findings and records a clean scoped host/integration/darwin/freebsd/openbsd matrix. That commit says it ran in a worktree; it is historical evidence and does not satisfy today's main-checkout proof rule | Run only the spec's changed-package lint scope in main; retain the recorded tier result and investigate only a refusal of this spec's imports |
| Other ACs | The implementation commits and named runs in the November 1 table remain the recovered record; this audit did not re-execute them | Complete the Linux/QEMU population above and any affected-suite evidence not recorded in those commits |

The new command was `CGO_ENABLED=1 ./le job run label config-proof-recovery
quiet command go test -race -tags "$tags" -run
'^(TestRepositoryConfigsValidate|TestSweepDerivesEachExclusion|TestValidateReportsNodeOfAnotherOS|TestValidateReportsHostBackendVerdict)$'
-v ./internal/component/config/cli/`, with `tags` derived from
`feature-gates.txt` plus `ze_core` as documented in `running-commands.md`.
Only the two named host tests finished. The interrupted corpus run is not a
test failure or a passing package run.

At initial recovery, the two November 2 owned journal rows needed implementation.
Both repairs have since landed, as recorded below:

| Work package | Producer diagnosis | Required proof |
|--------------|--------------------|----------------|
| Runner helper exit contract and placeholder documentation | `Runner.runOrchestrated` in `internal/test/runner/runner_exec.go` checks `cmd.ExitCode` only in the quick-Ze arm. Both foreground helper arms ignore it; the non-final arm fails every nonzero `Wait`, and the final arm consults only file-level `ExpectExitCode`. The placeholder page says every text expands, but the execution sites cover tmpfs, exec, stdin, env and HTTP, not assertion values | Add regression cases for declared zero/nonzero exits and mismatches on both helper positions; preserve timeout handling and file-level semantics. Correct the documented expansion scope. Do not work around the defect by rearranging commands |
| Doctor reference diagnostic reachability | `doctor.runChecks` in `internal/component/doctor/doctor.go` returns on `config.ReadConfig` failure before post-config checks. `config.loadConfig` refuses invalid custom sections before returning a tree; `bgp/config.checkFilterReferences` requires that tree. The old journal path `doctor/checks_config.go` has moved to `internal/component/bgp/config/filter_reference_doctor.go` | Reproduce `test/ui/doctor-config-reference.ci` through its native runner, then repair the diagnostic path without weakening its undefined-reference assertion; a new passing fixture alone would not repair the reachability defect |

No closure review was performed. Phase 5 remains open.

### Final proof record and blocked handoff (2026-11-02)

This is the durable record of parent `ses_e631cb616ffe3yt3BVP45Yn2nO`'s
reconciled implementation evidence. No test or review ran for this handoff.
The earlier recovery instructions are historical, not a second work queue.
No owned implementation repair remains pending beyond findings a future review
can expose. At this handoff, required runtime proof and foreign gate reds
remained open; the closure evidence after the compatibility allowances below
supersedes that state.

#### Landed repairs included in the future review population

| Commit | Repair and retained proof |
|--------|---------------------------|
| `175a44e816` | Foreground helper `exit=` contract and placeholder documentation. `TestForegroundHelperCommandExit`, `TestForegroundHelperFileExit`, and `TestForegroundHelperExitTimeout` pass under race, including both helper positions, signals, timeouts and final-file conflicts. `test runner helper-command-exit` failed with the helper check disabled and passes after promotion. Stress: 80 invocations, parallel 4, burners 2, no reproduction. The package run still failed at `TestCIAcceptOnlyLint` below |
| `bb4fead1d61aea7206caa582bc6b7cf130c67e36` | Git-derived corpus discovery and explicit closed policy in both role-clear fixtures. Discovery regression red before the repair, then race PASS, including missing/unreadable source controls. Both role-clear twins pass after promotion. Their draft stress selection completed 80 invocations, parallel 4, burners 2, no reproduction. Independent judge `ses_e62c72805ffedG6YKudd35zgBr` approved fixtures and discovery/exclusion coverage |
| `5dcffd85d0c49d5c8152ebb1571714191c6c371f` | Doctor reference finding before validated-load refusal. Unchanged `test ui --pattern doctor-config-reference` red then PASS. Full-tag race packages doctor and BGP config PASS, with diagnostic's earlier PASS retained. `TestDoctorReferenceBeforeLoadRefusal` PASS through the local answer route. Coverage observes syntax refusal, parsed dispatch, load refusal and successful post-load paths. Parsed-phase sort comparison remains unexercised, not full branch coverage |
| `f90191eab4003dbbd0a54b36d59f83fcc593eee7` | Six direct lint findings cleared: one runner failure-label constant and five discovery path joins. Host/integration lint over runner and config/cli, and Darwin/FreeBSD/OpenBSD lint over runner, each exit 0 and `0 issues`. No automatic Spec-work trailer: include explicitly in review |
| `26da0eef6ee8c33834e85a67f39ef4bff8a45956` | Connected-distance fixture provides a reachable nonlocal next hop. Independent judge `ses_e62450d15ffeK9MS6utJRtoWW2` approved setup without assertion changes. Both distance polarities pass in fresh namespaces, including the unchanged proto-250 checks. No automatic Spec-work trailer: include explicitly in review |

#### Acceptance evidence retained at handoff

All November 2 Go proof used the full feature-tag set from `feature-gates.txt`
plus `ze_core`, through `./le job run` in main. Race runs used `CGO_ENABLED=1`.
The completed corpus command was `go test -race -tags "$tags" -timeout 20m
-run '^TestRepositoryConfigsValidate$|^TestSweepDerivesEachExclusion$/(statement_shape|host_verdict)$'
-v ./internal/component/config/cli/`. The earlier interrupted command is not
a passing package run. The later completed results supersede its missing verdict.

| AC | Final evidence and limit |
|----|--------------------------|
| AC-1 | Linux PASS: 2813 Ze configs, 241 migrated ExaBGP configs, 20 declared/asserted refusals, 489 exit-expectation skips, 29 load sequences/71 steps, 80 legacy native fixtures with reasons. Zero findings and zero host verdicts for every code, including kernel-capability, node-other-os and host-backend. macOS remains unproven and post-release. The Git-derived discovery no longer reads `plan/private/router.conf` and `plan/private/lns.conf`, which `.gitignore` ignores; they are treated as private files, not repository configs, until the owner answers |
| AC-2 | Preserve the recorded perf-config mutation: removing 664be57056 produces four RFC8212 findings for perf-receiver/perf-sender import/export, restoring it removes them. Original scratch is absent. This is historical mutation evidence, not a newly observed walk |
| AC-3 | Corpus findings cleared, including the later eight role-clear findings. Reconciled changed-carrier union: 349 files, not 349 new main passes. Historical export runs remain labeled below. At this handoff one Linux runtime selector was unrun: linklocal-only-multihop-withdraw, since run and passing (closure evidence below) |
| AC-4 | `28bd5b5c9c`: netlab 26.10 `./le test netlab check` PASS for all three nodes, including rendering and validation. Supersedes the initial claim that it was unrun |
| AC-5 | Shared classification and native parsing/config-direction proofs retained from `01e6f725b7`, plus the 80 reasoned legacy classifications in the completed corpus. No legacy fixture deleted |
| AC-6, AC-7 | Full-tag race named PASS for `TestValidateReportsNodeOfAnotherOS` and `TestValidateReportsHostBackendVerdict`, including marked/unmarked subtests. Linux corpus has no host-code skips. Injected plan9 schema is not macOS execution |
| AC-8, AC-9 | Shared labconfig and BGP/IPsec renderer race evidence from `c782411791` retained. Completed corpus validates rendered link-local and PKI inputs |
| AC-10, AC-11 | Completed corpus plus the exclusion differential results below prove statement classification and test-plugin composition |
| AC-12 | Migration/bridge race and DSCP regression evidence from `794c3ac052`/`7c85bafe4a`, plus migrated-config validation retained. Final old-4.2/old-5.0 conf-flow results are declared allowances, not wire-equivalence passes |
| AC-13 | Standalone and cumulative tape-order validation retained, including showcase-lab base and IRR fragment, within 29 sequences/71 steps. No live demo recording is owed |
| AC-14 | Both corrected interop scenarios pass through the main-built Ze guest: bgp-delayopen-frr and bgp-session-protection-frr. bgp-rfc2545-third-party-linklocal-frr also passes. Each reports 1 passed/0 failed, no skips |
| AC-15, AC-16 | Completed cumulative validation and 20 asserted refusals retained. All four formerly unrun Linux reload cases now pass, listed below |
| AC-17, AC-18 | Recorded parse/format/dump/completion/doctor-claims and policy-root-migrate passes in `84c0fc6cea` retained with corpus validation. Doctor reference reachability is now separately repaired and proved |
| AC-19 | `TestRecordPortExpansionRefusesUnreservedPort` and runner/CLI race evidence retained. Five reached consumers pass: bgp-rpki-added-live, bgp-rpki-attach-live, ipsec-added-live, reload-listener-rejected, bmp-lg-http-routes |
| AC-20 | `d3345c8082` retains the passing no-final control `TestUnknownRedistributeFamilyRefusesLoad` |
| AC-21 | NOT CLEAN. Completed main lint exposed six owned findings, now cleared, plus the foreign findings below. Historical tier result names bfd/api, not sweep imports. Historical worktree lint is not substituted for main evidence |
| AC-22 | All nine differential controls pass across completed runs: placeholders 40 restored findings, ports 243, test plugins 17, declared refusal 1, load sequence 20, migrate input 1, native classification 80, statement shape 3, host verdict 11 with plan9 schema. The two traversal-error cases passed after Git-derived discovery. Seven successful controls were not repeated |

Independent remaining-proof audit `ses_e61f2c7f6ffeQkj2f8iH8GipoV` rejected
the proposed blanket standalone smoke debt. AC-1/AC-13 require offline and
cumulative validation, which the retained corpus supplies. This applies to
`test/perf/configs-filter/ze.conf`, stress 01-bulk-ipv4/02-multi-peer/03-session-flap/05-profile-1m,
and demo cli-dashboard/config-graph/config-views/health-reports/rib-fib/zefs-config,
showcase-lab and the IRR fragment. Their runtime behavior is not proved by
the sweep, and no new runtime smoke obligation is invented for them.
`validate_corpus_test.go` `judgeSequence`, `load` and `checkBody` use editor
merge/replace and offline validation, not live demo execution.

#### Exact Linux runtime and package reconciliation

| Selection | Retained result |
|-----------|-----------------|
| `test bgp reload static-bfd-profile-reload-deleted static-bfd-profile-reload-passive tx-iface-wireguard-invalid-bad-public-key tx-iface-wireguard-invalid-no-private-key` | Four named PASS, no skips, fresh user/network/mount namespaces on Linux 7.2.9-ze |
| `test static static-distance-beats-ebgp static-distance-loses-to-ebgp static-kernel-distance-bgp-wins static-kernel-distance-reload static-kernel-distance-route-override static-kernel-distance-static-wins static-kernel-igp-cost-reselect` | Seven named PASS, no skips, same isolated native route |
| `test firewall copp-bgp copp-over-limit-drop copp-trusted copp-unknown-source-drop` | Four named PASS across runs. Over-limit and unknown-source initially collided on the shared ze_copp table, then passed with `--parallel 1` in isolation. Do not claim the initial concurrent run passed |
| `test bgp plugin --parallel 1 dns-stub-lookup dns-stub-answer-change connected-distance-arbitration connected-distance-raised-loses iface-tunnel-kinds tunnel-ttl-default` | All six have named PASS, no skips. Raised-distance passed only after 26da0eef6ee8, with its affected twin also passing |
| `test bgp plugin --parallel 1 rpki-aspa-disabled rpki-aspa-invalid rpki-aspa-policy-logonly rpki-aspa-policy-reject rpki-aspa-policy-unknown-reject rpki-aspa-unknown rpki-aspa-valid` | Seven named PASS, no skips, managed Ze-system guest kernel 7.3.0-ze, runner-provisioned namespaces/links and daemon UID/GID 1000 |
| `test bgp plugin --parallel 1 ddos-announce-rate-limit ddos-direction ddos-incident-confidence ddos-policy` | All four have named PASS across runs, no skips. PERFMON resolved the BPF verifier environment refusal for three. Confidence passed separately after accounting/tracking provision below |
| `test firewall --parallel 1 gtsm-listener-floor gtsm-related-icmp` | Both named PASS, no skips, managed Ze guest |
| `test qemu docker-lab lab 'test integration interop' env 'INTEROP_SCENARIO=<scenario>' timeout 20m` | Each of bgp-delayopen-frr, bgp-session-protection-frr and bgp-rfc2545-third-party-linklocal-frr PASS. Fresh main-built binaries, FRR 10.7.1, Ze guest. Existing assertions passed, not a new discrimination walk or a proven collision-arbitration order |
| `go test -race -tags "$tags" -timeout 5m -run '^TestValidateBGPPeersWithPolicyRunsPeerValidatorAlone$' -v ./internal/component/config/infra` | Named PASS, exit 0. No missing infra selector remains |

Confidence environment proof used an independently approved fresh container
namespace inside the disposable Ze guest (`ses_e620b5f80ffeOEPB5z7WJGtTCf`).
Accounting readback was 1, with an accept-only output ct-state tracking rule
before traffic. Runner UID/EUID/GID were 1000, with effective/ambient mask
`0xc001003400` (NET_ADMIN, NET_RAW, NET_BIND_SERVICE, BPF, SYS_RESOURCE, PERFMON).
`ZE_TEST_NETNS` was unset, so the runner used that prepared namespace.
Only the unchanged `ddos-incident-confidence` ran: named PASS 1/1.
The earlier empty recent-flow ring was an environment prerequisite failure,
not a product repair. Managed runs report VM stopped and left-behind none.

Historical package evidence remains in `5f65d1f35c` (config/host verifiers),
`01e6f725b7` (native classification/reactor direction), `e2af3bbac6` (RFC8212),
`698a7a547a`, `5dc57599fb` and `c7f5ce38b6` (runner/CLI/tmpfs/fixture).
Historical export-based functional claims are `6ca76b741b` (55 parse),
`99839b2060` (39 UI), `86280b3a20` (22 plugin), `cf5d6b8c9e` (five reload),
`41f87f7d4a` (plugin), `f194d2760c` (mixed) and `3cb09bb0a9` (RFC subjects).
Their completed runs were not repeated to change provenance. Final evidence
reconciliation found no further unrun Linux functional or package selector.

#### Final compatibility allowances

| Command through the named launcher and job admission | Final result |
|-----------------------------------------------------|--------------|
| `./le --name config-phase5-proof test exabgp old-4.2 conf-flow` | Exit 0, conf-flow SKIP, 0 passing cases, 1 allowed difference |
| `./le --name config-phase5-proof test exabgp old-5.0 conf-flow` | Exit 0, conf-flow SKIP, 0 passing cases, 1 allowed difference |

Both final reasons name third-redirect as the first divergence. These are
declared allowances after `88e7a340ab`, not wire-equivalence PASS and not proof
of later frames. The allowance-every-frame spec named above retains that defect.

### Closure evidence (2026-11-02, supersedes the blocked handoff)

The handoff above recorded `linklocal-only-multihop-withdraw` as blocked on the
execution environment (a read-only procfs in the container, no capabilities
over guest SSH). That reading was wrong: the native unprivileged user-namespace
route on this Ze-kernel host had never been tried. Logs are under
`tmp/session/2026-11-02-215330/scratch/`.

| AC | Fresh evidence |
|----|----------------|
| AC-1 | `go test -tags "$tags" -count=1 -run '^TestRepositoryConfigsValidate$' -v ./internal/component/config/cli/` (tags: `ze_core` plus every `ze_` gate in `feature-gates.txt`), through `./le job run`, `close-sweep-v.log`: `validated 2827 Ze configs and 241 migrated ExaBGP configs; 20 declared refused and asserted refused; 489 skipped as refused by an exit expectation; 29 load sequences (71 steps); 80 legacy native fixtures skipped; host verdicts skipped: []`, `--- PASS: TestRepositoryConfigsValidate`, zero findings. The earlier full-tag race run (`job-phase5-ac1-8ac66f81.log`, 2825 configs) found six `config-bgp-rfc8212-no-policy` findings in `test/reload/reload-add-peer-role-capability.ci`, added by another session in bb50677cb4; bf8162f81a states the policy in both bodies, after which the sweep passes |
| AC-2 | Freshly observed (`job-phase5-ac2-mutation-9171615d.log`): with 664be57056 reverted the sweep names `test/perf/configs/ze.conf` four times with `config-bgp-rfc8212-no-policy` (perf-receiver and perf-sender, import and export); restored, the file diff is empty and the file is in no finding |
| AC-3 | `linklocal-only-multihop-withdraw` PASS 1/1, no skip (`job-linklocal-native-3bd4fba5.log`), through `./le job run ... unshare --user --map-root-user --net --mount sh -c 'mount -t tmpfs tmpfs /run/netns && ip link set lo up && exec ./le test bgp plugin --parallel 1 linklocal-only-multihop-withdraw'` |
| AC-3, AC-19 | `test/parse` 426/426 under the new `exit=` enforcement (`job-phase5-parse-016b9385.log`). After 99baaf4948, `TestCIAcceptOnlyLint`, `TestParseCICorpusReadsUnderTheGenericParser` and `TestPeerASDerivationReachesEveryCIFile` PASS (`job-runner-ci-corpus-v-324fc113.log`) |
| AC-21 | `./le go lint run scope` over config/cli, test/runner, test/fixture, doctor, core/diagnostic, bgp/config, pkg/plugin/rpc, pkg/plugin/sdk, test/labconfig, test/exabgpnative and exabgp/migration (`close-lint.log`): host and Linux integration builds report two findings, darwin, freebsd and openbsd 0 issues. Both findings are in files carrying another live session's uncommitted hunks: goconst `rfc` in `runLE`'s caller in `internal/test/fixture/ui_fixture_le_rfc_answers.go`, and embedlit on the `netlink.Veth` literal in `internal/test/fixture/register_sflow_agent_address_linux.go`. Owner: `plan/release/validation/spec-lint-backlog-3-test-tooling.md`. A red holds a release, not a commit (`ai/rules/pre-release.md`) |
| AC-22 | All nine controls PASS in `job-phase5-ac1-8ac66f81.log`: placeholders 40, ports 244, test plugins 17, declared refusal 1, load sequence 20, migrate input 1, native classification 80, statement shape 3, host verdict 11; `TestSweepRefusesAnEmptyCarrierKind` and both `TestSweepSourceDiscovery*` PASS |

Count deltas against the November 1 table (2778 to 2827 configs, 487 to 489
exit-expectation skips, 242 to 244 port findings restored): between c7f5ce38b6
and HEAD, 51 `.conf`/`.ci`/`.tape` files were added and 18 deleted, and
bb4fead1d6 replaced the directory walk by Git discovery, which now admits two
tracked `vendor/` `.conf` files (classified non-Ze) and every directory named
`draft` outside the runner's draft tree. No per-file reconciliation was made,
so the split of each delta between corpus growth and discovery is unexplained.

`TestPluginObserversWaitForTheScriptedPeer` is red before this spec's work:
7e68ebce69 records it as a known red, owned by
`plan/release/validation/spec-observer-peer-wait-backlog.md`.

Owner question pending, not answered here: whether the gitignored
`plan/private/router.conf` and `plan/private/lns.conf` belong in the sweep. The
provisional reading leaves them out (AC-1 row of the final proof record).

### Remaining foreign reds and proof limits

These are the last recorded verdicts, not a fresh audit of their owners' trees,
except where the closure evidence above re-ran a check.

| Finding | Owner and next action |
|---------|-----------------------|
| Fixture lint: errorlint `%q` in plugin_fixture_06_events.go; goconst `veth` in plugin_fixture_rfc2545_joint_subnet_linux.go and register_sflow_agent_address_linux.go; goconst `update` in ui_fixture_le_docvalid_answers.go, ui_fixture_le_placement_answers.go and ui_fixture_le_vendor_web_answers.go; goconst `rfc` in ui_fixture_le_rfc_answers.go; embedlit in register_sflow_agent_address_linux.go; unparam in misc_fixture_runner_derived_debt.go and plugin_fixture_13_peer_lifecycle.go | All under `internal/test/fixture/`. Package owner: `plan/release/validation/spec-lint-backlog-3-test-tooling.md`. 59ff106163 and b760ebbab9 cleared the errorlint, `veth`/`update` goconst and unparam findings; the closure lint run leaves only goconst `rfc` and embedlit, both in files holding that session's uncommitted hunks |
| Setup lint: `could not import go/parser (Config.Importer.Import(go/parser) returned nil but no error)` in `internal/le/test/weakened/scope_go.go` | Same tooling owner. Analyzer importer failure, not a missing import. Diagnose before retry |
| Four misspell findings in `internal/component/doctor/answer.go`: marshalled twice, coloured, colour | Provenance command-surface-parity (`0b403ea3d3`, `5a91877b02`), no live spec of that name. `plan/release/validation/spec-lint-backlog-0-umbrella.md` coordinates routing. Direct child ownership remains unresolved |
| `TestCIAcceptOnlyLint` names `test/parse/graceful-restart-llgr-forked.ci` | Owner filter-wire-0-umbrella, introducing path commit `efa0d0957e`. Resolved: `TestCIAcceptOnlyLint` PASS after 99baaf4948 (`job-runner-ci-corpus-v-324fc113.log`) |
| Doc gate: shipping wiki/live `clear debug` mode mismatch, plus five stale anchors in commands.md/core-design.md (`runIngressPolicyChain`), meta/role.md (`broadcastValidateOpen`), verify-freshness-scope.md (`IsProse`), route-policy.md (`MustRegisterAttachment`) | Parent owns routing to the command catalog, BGP/filter and verification-tooling owners. Direct assignments were not established. Discovery/doctor pages had no anchor findings. DOCS-TO-CODE stale was a warning, not another passing gate |
| Historical tier refusal in `internal/component/bfd/api` | Foreign BFD ownership, not a sweep-import refusal. Preserve the recorded distinction |
| macOS corpus and `test ui doctor-bgp-md5` | Unproven, post-release by the owner's standing macOS rule. Owned by `plan/post-release/engineering/spec-config-sweep-macos-proof.md`. The doctor case declares `skip-os=linux,freebsd`. Darwin lint and injected-OS Linux units do not discharge either execution |
| Independent full Review Gate | Done at closure: see Review Gate |

The blocked handoff was cleared by the closure evidence above.

### Handoff enumeration (nothing dropped)

The previous agent's handoff (`tmp/session/2026-10-31-204407/scratch/handoff-config-validate-sweep.md`)
plus the brief that commissioned this spec carry 18 items. Each maps to ACs.

| # | Item | Source | AC |
|---|------|--------|----|
| 1 | Interop `@NAME@` placeholders | handoff sweep 1 | AC-8 (4 findings, not 6), plus AC-9 (36 PKI placeholder findings the handoff missed) |
| 2 | Host verdicts: firewall default backend, gokrazy interface commit | handoff sweep 2 | AC-7 (26 findings) |
| 3 | FRR `vtysh.conf` misread because `service` is a Ze root | handoff sweep 3 | AC-10 (3 findings) |
| 4 | Reload `.ci` with a deliberately refused config and exit 0 | handoff sweep 4 | AC-16 (22 findings) |
| 5 | `tmpfsForRecord` `$PORT3` expansion bug, journal row owed | handoff sweep 5 | AC-19, owned journal row |
| 6 | 302 RFC 8212 findings in `.ci` bodies | handoff fixes | AC-3 |
| 7 | netlab generator for `contrib/netlab/golden/r1.conf`, `r3.conf` | handoff fixes | AC-4 |
| 8 | 80 parse errors in `test/exabgp-compat/native/*.conf` | handoff fixes, corrected by the brief | AC-5 |
| 9 | Demo step fragments need validation in sequence | handoff fixes | AC-13 (21 findings) |
| 10 | `interface xfrm` unknown (far-end.conf, 4-xfrm.conf) | handoff fixes | AC-6: corrected, it is a host verdict, not a config defect |
| 11 | `bgp-delayopen-frr` missing prefix maximum | handoff fixes | AC-14 |
| 12 | `bgp-session-protection-frr` unknown `bgp filter` | handoff fixes | AC-14 |
| 13 | Three refused migrate outputs | handoff fixes | AC-12 |
| 14 | "The rest" (plugin-verify, yang-missing, bgp-peer, ...) | handoff fixes | AC-11, AC-15, AC-17, AC-18 (62 findings, each named) |
| 15 | Scoped lint on `./internal/component/config/cli/...` | brief | AC-21 |
| 16 | `./le arch tier check` for the sweep's import of `internal/exabgp/migration` | brief | AC-21 |
| 17 | `TestUnknownRedistributeFamilyRefusesLoad` red, unrelated, journal row | brief | AC-20, owned journal row |
| 18 | Sweep green with zero findings, and the mutation proof against 664be57056 | brief | AC-1, AC-2 |

Corrections to the handoff, verified at the producer:

| Handoff claim | What the code says |
|---------------|--------------------|
| Nothing reads `test/exabgp-compat/native/*.conf`, delete needs consent | False. `TestParseAllConfigFiles` (`internal/component/bgp/config/loader_test.go`) parses them under `curatedNativeExampleFixtures` and `legacyExampleFixtureReason`; `internal/component/config/serialize_test.go` and `internal/component/bgp/reactor/config_direction_test.go` read them too. No file is deleted |
| `interface xfrm` is unknown (a config defect) | `list xfrm` and `list pppoe-client` (and `container mirror`) carry `ze:os "linux"` in `internal/component/iface/yang/ze-iface-conf.yang`; `yangToNode` (`internal/component/config/yang_schema.go`) prunes a node whose `ze:os` differs from `runtime.GOOS`. On macOS the parser then says "unknown field". A host verdict, and a misleading operator message |
| 6 placeholder findings | 4 `@ZE_LINK_LOCAL@`/`@FRR_LINK_LOCAL@` findings, plus 36 `%%PKI_B64:<file>%%` findings in 18 `test/interop-ipsec` scenarios |
| `$PORT3` in 4 `.ci` bodies | 5 files: `test/plugin/reload-listener-rejected.ci`, `test/plugin/bmp-lg-http-routes.ci`, `test/reload/bgp-rpki-added-live.ci`, `test/reload/bgp-rpki-attach-live.ci`, `test/reload/ipsec-added-live.ci`. Runner expansion of `$PORT2`/`$PORT` only also applies to exec strings, stdin and env (`internal/test/runner/runner_exec.go`) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/le-tooling.md`, `ai/doors/system-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/appliance.md`, `ai/doors/core.md` - the subsystems this spec names
  -> Constraint: `ze config validate` never calls a live `OnConfigVerify`; it runs YANG checks and each `InProcessConfigVerifier` (config door, Traps)
  -> Constraint: a test that mutates a package registry restores it (core door, Traps); the sweep linking test-only plugins must not leave registrations behind
  -> Constraint: never import `internal/le` from product code (le-tooling door); a helper both sides need lives outside `internal/le`
- [ ] `docs/architecture/config/yang-config-design.md` - validation layers, where a validator runs, `ze:os` and `ze:backend`
  -> Decision: `ze:os` prunes at schema-build time because GOOS is fixed for the binary (`yang_schema.go`, comment above `getBackendExtension`)
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` block grammar, where a refusal marker would be declared
- [ ] `docs/architecture/testing/interop.md` - scenario rendering and placeholders
- [ ] `docs/guide/command-reference.md` - `ze config validate` codes the operator sees

### RFC Summaries (Scope: config, RFC-driven)
- [ ] `rfc/short/rfc8212.md` - the default-reject rule the warning reports
  -> Constraint: an EBGP direction with no explicit policy passes no routes (RFC 8212 Section 3)
- [ ] `rfc/full/rfc8955.txt` Section 4.2.2.11 - DSCP component (applies to IPv6 by RFC 8956 Section 3)
  -> Constraint: "The six least significant bits contain the DSCP value.  All other bits SHOULD be treated as 0." Only a SHOULD, so Ze refusing ExaBGP's `traffic-class 101` is stricter than ExaBGP with no MUST behind it

**Key insights:**
- The sweep calls `runValidation`, the exact path `ze config validate` takes, so a finding is what an operator would see.
- Over-reports fall in four classes, each removable by a rule derived from a fact already declared: the host (ze:os, OS default backend), the harness (placeholders, port expansion, test-only plugins), the carrying test (deliberate refusal, migrate input, merge fragments) and the classification (non-Ze files).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/cli/validate_corpus_test.go` (976L) - `TestRepositoryConfigsValidate`: takes its sources from `sweepSourceFiles` (`validate_corpus_discovery_test.go`), which lists `*.conf`, `*.ci` and `*.tape` with `git ls-files --cached --others --exclude-standard` (tracked files plus untracked files the ignore rules permit), drops the runner's draft tree (`runner.SuiteDir` with draft set), and refuses an incomplete listing; it reads every `.conf` and the `.ci` bodies `collectCorpusCarriers` yields, classifies a body by `firstConfigKeyword` against the schema and `registry.GatedConfigRootRequirements`, migrates non-Ze bodies with `migration.ParseExaBGPConfig`/`MigrateFromExaBGP`/`SerializeTree`, skips a `.ci` matching `refusedExit` (non-zero exit), excuses an RFC 8212 warning when the `.ci` has `expect=` naming 8212 or the scenario directory name holds 8212, counts a code as a host verdict through `isHostVerdict`, which reads `diagnostic.Lookup(code).HostVerdict` (`internal/core/diagnostic/codes.go` marks `config-kernel-capability`, `config-node-other-os` and `config-host-backend`), and expands ports through `runner.ExpandPorts`. At the spec's start it held a `hostVerdictCodes` list (only `config-kernel-capability`) and a copy of the runner's port rule in `expandRecordPorts`; both are gone
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation`: YANG, `kernelcap.Refuse` reported as `config-kernel-capability` (documented as a host verdict), MCP, gNMI, `config.VerifyPluginConfig` reported as `config-plugin-verify`, BGP, `addRFC8212Warning`
- [ ] `internal/component/bgp/config/loader_test.go` - `TestParseAllConfigFiles`, `curatedNativeExampleFixtures` (8 files), `legacyExampleFixtureReason` (prefix and name rules)
- [ ] `internal/component/config/yang_schema.go` - `yangToNode` returns nil for a node whose `ze:os` is not `runtime.GOOS`
- [ ] `internal/component/config/parser_freeform.go` - "unknown field in %s: %s" for a name the schema lacks
- [ ] `internal/component/config/backend_gate.go` - `validateBackendFeatures`, "backend feature gate: no backend configured at <path>"
- [ ] `internal/component/iface/register.go`, `internal/component/firewall/engine.go`, `internal/component/traffic/register.go` - "no backend configured and no OS default available"
- [ ] `internal/component/vpp/cpuset.go` - CPU inventory read from `/sys/devices/system/cpu/online`
- [ ] `internal/test/runner/runner_exec.go` - `tmpfsForRecord` and four other sites expand `$PORT2` then `$PORT`, nothing else
- [ ] `internal/le/interoplab/bgp/prepare.go` - `zeLinkLocalPlaceholder`, `frrLinkLocalPlaceholder`, rendered from the lab's IPv6 prefix
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `pkiPlaceholder` (`%%PKI_B64:<file>%%`)
- [ ] `internal/le/test/netlab/netlab.go` - `Checker.validateGolden` runs `ze config validate` on each golden and reads only the exit code, so a warning passes
- [ ] `contrib/netlab/ze/bgp.j2` - the template that renders the BGP block of each golden
- [ ] `internal/component/bgp/config/loader_create_test.go` - `TestUnknownRedistributeFamilyRefusesLoad`, red: the redistribute filter validator refuses unknown filter "V4" before `ExtractRedistributeRules` reaches its "no final" path
- [ ] `demos/terminal/topics/ipsec/configure.tape` - steps `load file absolute merge` each numbered fragment in order

**Behavior to preserve:**
- `TestParseAllConfigFiles`, `serialize_test.go` and `config_direction_test.go` behave as today over `test/exabgp-compat/native/`
- `ze config validate` exit codes; `config-bgp-rfc8212-no-policy` stays a warning
- The sweep's fatal guards: zero Ze configs or zero migrated configs fails the run (R-6)
- Every `.ci` keeps proving what it proved; an expectation it loses goes through `ze-test-judge`

**Behavior to change:**
- The sweep reports zero findings; its exclusions are each derived from a declared fact
- An operator validating a config on a host whose OS prunes a node is told the node exists only on the named OS, not "unknown field"
- A verifier verdict about the host (OS default backend, host inventory read) carries a code that says so
- The runner refuses or reserves `$PORT3` instead of silently producing `<port>3`
- The netlab template, demo fragments, interop scenarios, `.ci` fixtures and ExaBGP migration listed in the ACs are fixed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Offline validation path | `internal/component/config/cli/cmd_validate.go` `runValidation` | The whole `ze config validate` verdict | Extends: the sweep keeps calling it, and two host-verdict codes are added beside `config-kernel-capability` |
| Host verdict code | `cmd_validate.go` `kernelcap.Refuse`, `internal/core/diagnostic/codes.go` `config-kernel-capability` | A verdict documented as about THIS host | Extends: the same shape for an OS-pruned node and for an OS default backend or host inventory; the sweep's `hostVerdictCodes` keeps reading codes, never message text |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` | One entry per code with title, description, examples | Extends: new codes register here, so `ze show diagnostic <code>` explains them |
| `ze:os` pruning | `internal/component/config/yang_schema.go` `yangToNode`, `getOSExtension` | Drops the node from the schema on another OS | Extends: the schema remembers the pruned name and its OS so the parser can say why the name is refused |
| Backend feature gate | `internal/component/config/backend_gate.go` `validateBackendFeatures` | Refuses an empty backend and unsupported nodes | Extends: the empty-backend refusal, and the components' "no OS default" errors, are marked as host verdicts |
| Corpus carrier reader | `collectCorpusCarriers` (shared with `TestFormatCorpusRoundTrip`) | Yields each `.ci` tmpfs `.conf` and stdin=config body | Extends: carries the block's refusal marker and the test's load sequence |
| Runner port expansion | `internal/test/runner/runner_exec.go` `tmpfsForRecord` and four sibling sites | Expands `$PORT2` then `$PORT` | Replaces the sweep's copy (`expandRecordPorts`) with one exported expansion both use, which also answers for `$PORT3` |
| Native fixture classification | `internal/component/bgp/config/loader_test.go` `curatedNativeExampleFixtures`, `legacyExampleFixtureReason` | Curated fixtures must parse; legacy ones are named with a reason | Extends: moves to one declaration both `TestParseAllConfigFiles` and the sweep read; no second list |
| Interop placeholder rendering | `internal/le/interoplab/bgp/prepare.go` link-local placeholders; `internal/le/interoplab/ipsec/helpers.go` `pkiPlaceholder` | Renders scenario files before a lab run | Extends: the placeholder grammar and the renderer move to a leaf outside `internal/le` that both interoplab and the sweep call |
| Test-only plugin composition | `internal/test/plugins/all` | The plugin set the functional `ze` build links under `zetest` | Extends: the sweep links the same composition, so `fakeredist` and `fakedrain` resolve as they do in the `.ci` run |
| Gated root table | `registry.GatedConfigRootRequirements` | Names the tags a root needs | Does not change: the sweep keeps reporting a root its build lacks |
| `.ci` refused-exit rule | `validate_corpus_test.go` `refusedExit` | Skips a `.ci` whose command expects a non-zero exit | Extends: a block the test declares refused is asserted refused, not skipped |
| netlab golden check | `internal/le/test/netlab/netlab.go` `Checker.validateGolden` | Validates goldens by exit code | Does not change: the template fix makes goldens clean; the sweep catches the warning the exit code does not |
| ExaBGP migration | `internal/exabgp/migration` `ParseExaBGPConfig`, `MigrateFromExaBGP` | Turns an ExaBGP config into Ze config | Extends: `family { all; }` and an ExaBGP-accepted DSCP value migrate to a valid Ze config |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `go test -run TestRepositoryConfigsValidate` in `internal/component/config/cli` under the full tag set, and the `ze config validate` command for the operator-visible codes
- Input: repository files (`.conf`, `.ci` bodies, ExaBGP configs)

### Transformation Path
1. Walk the tree, collect carriers with what each carrying test declares (refused, RFC 8212 subject, migrate input, merge sequence)
2. Render harness placeholders through the shared renderer, expand ports through the runner's own function
3. Classify: Ze body, ExaBGP body (migrate), non-Ze body, native legacy fixture (from the shared classification)
4. Validate through `runValidation`; for a merge sequence, validate the cumulative config after each step
5. Judge each diagnostic: error, unexcused RFC 8212 warning, host verdict (counted and logged), declared refusal that did not refuse (finding)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test to product validation | `runValidation` in-package call | No |
| Sweep to interop harness | Shared leaf renderer outside `internal/le` | No |
| Sweep to functional runner | Exported port expansion in `internal/test/runner` | No |

### Integration Points
- `runValidation` - unchanged entry; new host codes added inside
- `collectCorpusCarriers` - extended with declared facts
- `internal/test/plugins/all` - blank-imported by the sweep test

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, test tooling |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound: the sweep holds no list of files, directories or scenarios; every exclusion derives from a declaration (ze:os, verifier host code, renderer grammar, carrying test, shared classification) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every one of the 26 host-backend findings validates on Linux | the messages name the missing OS default or `/sys` | A real defect hides behind the host code | Run the sweep on the Linux host (scoped package test) | confirmed: on Linux the sweep reports no `config-host-backend`; every such body validates |
| A-2 | The 11 ze:os findings validate on Linux | `ze:os "linux"` on each pruned node | Same | Same run | confirmed: on Linux no `config-node-other-os`; built as plan9 the 11 findings are host verdicts |
| A-3 | The 22 AC-16 bodies are refused on purpose | test names and their expectations | Marking hides a defect | Read each `.ci` expectation; the sweep asserts the refusal | confirmed: each of the 20 declared bodies is read against its test's expectation; three red reload tests were stale, fixed under ze-test-judge (ece2daa9de) |
| A-4 | Adding explicit policy to a `.ci` body does not change what that test proves | most are parse and UI tests | An expectation changes | `ze-test-judge` on each changed expectation | confirmed: no expectation changed in AC-3; three RFC 8212 subject tests judged (3cb09bb0a9) |
| A-5 | A Ze config never opens with `service <WORD>` | the `service` root is a block | AC-10's shape rule misclassifies | Grep the Ze corpus | confirmed: the shape rule moved only the three vtysh.conf out of the Ze population |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Host codes become an escape hatch for real defects | a finding moves to a host code on Linux | AC-1 requires the Linux run to show no host-verdict skip outside `config-kernel-capability` |
| R-2 | netlab is not installed on the Mac, so goldens cannot be regenerated | `Checker.findNetlab` fails | Regenerate on a host with netlab; the template and goldens land in one commit |
| R-3 | Linking test-only plugins changes the schema the sweep sees | new findings | They are the functional `ze`'s schema, which is what the `.ci` run uses |
| R-4 | 126 `.ci` files edited by hand drift from their tests | functional reds | Run each touched suite (named under Goal Gates) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Test fixtures and demos; the operator-visible change is the two new `ze config validate` codes |
| How is it reverted? | Per-commit revert |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop` (interop link-local scenarios), RFC 8212 policy work in `bgp/reactor` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config with an OS-pruned node | → | `yangToNode` pruned-name record, parser refusal | `TestValidateReportsNodeOfAnotherOS` (`internal/component/config/cli`) |
| `ze config validate` with no backend and no OS default | → | verifier host-verdict marking in `runValidation` | `TestValidateReportsHostBackendVerdict` (`internal/component/config/cli`) |
| Repository files | → | `TestRepositoryConfigsValidate` | itself, plus `TestSweepDerivesEachExclusion` (one case per exclusion rule) |
| `.ci` with `$PORT3` | → | runner port expansion | `TestRecordPortExpansionRefusesUnreservedPort` (`internal/test/runner`) |

## Acceptance Criteria

Finding counts are from `sweep-findings-556.txt`; they sum to 556.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `TestRepositoryConfigsValidate` under the full tag set (`./le test unit config` tags), on macOS and on the Linux host | Zero findings on both. The log states how many configs were validated, migrated, declared refused, skipped as legacy native fixtures, and skipped per host-verdict code; on Linux the only host-verdict code that may appear is `config-kernel-capability` |
| AC-2 | Revert 664be57056's change to `test/perf/configs/ze.conf` | The sweep goes red naming `test/perf/configs/ze.conf` and `config-bgp-rfc8212-no-policy`; restoring it turns green. Both runs recorded in the spec |
| AC-3 | 302 RFC 8212 findings in 126 `.ci` files (`test/parse` 55, `test/ui` 40, `test/plugin` 23, `test/reload` 6, `test/firewall` 2) | Each body states its import and export policy explicitly, or the test is about RFC 8212 and carries an `expect=` naming 8212. Any expectation the edit changes is judged by `ze-test-judge` |
| AC-4 | `contrib/netlab/golden/r1.conf`, `r3.conf` (4 findings) | `contrib/netlab/ze/bgp.j2` renders explicit policy for EBGP peers; the goldens are regenerated from it and the netlab checker agrees |
| AC-5 | 80 findings in legacy `test/exabgp-compat/native/*.conf` (41 `api-*`, 35 `conf-*`, `example-healthcheck`, `extended-nexthop`, `parse-multisession`, `unknown-message`) | The sweep's treatment derives from the one classification `TestParseAllConfigFiles` uses, moved out of `loader_test.go` into a declaration both read: curated fixtures are validated, legacy ones are logged with their reason. No file is deleted; `TestParseAllConfigFiles`, `serialize_test.go` and `config_direction_test.go` behave as before |
| AC-6 | 11 findings on `ze:os "linux"` nodes (`xfrm`, `pppoe-client`, `mirror`): `demos/terminal/showcase-lab/far-end.conf`, `demos/terminal/topics/ipsec/4-xfrm.conf`, 2 `test/interop-ipsec` and 3 `test/interop-pppoe` scenarios, `test/parse/iface-netlink-accepts-pppoe-client.ci`, `test/parse/iface-vpp-accepts-mirror.ci`, `test/plugin/show-mtu-ike-probe.ci`, `test/plugin/show-mtu-oversized-tunnels.ci` | `ze config validate` on another OS reports a host-verdict code naming the node and the OS that carries it, with the fix (validate on that OS); the sweep counts it as a host verdict; on Linux all 11 validate |
| AC-7 | 26 host-backend findings: interface "backend feature gate: no backend configured" (gokrazy `ze.conf`, `test/install/bootstrap-config-valid.ci`, 2 demo vrrp topics, `test/parse/dhcp-set-format{,-multi}.ci`, 13 `test/reload` `.ci`), firewall "no OS default" (`test/parse/config-diff-list-order.ci` x2, `test/ui/doctor-firewall-nftables.ci`), traffic-control "no OS default" (`test/parse/traffic-empty-backend.ci`, `test/traffic/traffic-reload-{apply,qdisc-tc}.ci`), vpp CPU inventory (`test/parse/vpp-config-full.ci`) | Each verdict carries a host-verdict code the verifier sets, not a message match; the sweep counts it; on Linux all 26 validate |
| AC-8 | 4 link-local placeholder findings (`bfd-link-local-frr`, `bfd-link-local-two-links-frr`, `bgp-rfc2545-linklocal-nexthop-frr`, `bgp-vpn-frr`) | The sweep renders the placeholders through the renderer interoplab uses, from its single declaration, and the rendered configs validate |
| AC-9 | 36 PKI placeholder findings (18 `test/interop-ipsec` scenarios, `bgp-rpki` and `ike` each) | Same: `%%PKI_B64:<file>%%` is rendered from the scenario's own file by the shared renderer, and the configs validate |
| AC-10 | 3 FRR `vtysh.conf` findings (`test/interop`, `test/interop-ipsec`, `test/interop-l2tp`) | The Ze-or-not classification reads the first statement's shape, not only its keyword, so `service integrated-vtysh-config` is not a Ze body; no Ze config changes classification (A-5) |
| AC-11 | 7 findings naming test-only plugins (`test/plugin/drain-before-stop-internal.ci`, `redistribute-chain-narrow-reload.ci` x3, `redistribute-late-join-configadd.ci` x3) | The sweep links `internal/test/plugins/all`, the functional build's composition; the bodies validate |
| AC-12 | 3 refused migrate outputs: `test/exabgp-compat/old-4.2/etc/conf-flow.conf` and `old-5.0/etc/conf-flow.conf` (`traffic-class 101` exceeds 63), `internal/exabgp/bridge/testdata/exabgp-old/neighbor.conf` (`family { all; }`) | `ze exabgp migrate` output validates for all three. `all` migrates to the families ExaBGP means by it. For the DSCP value, ExaBGP's behaviour wins (RFC 8955 Section 4.2.2.11 states only a SHOULD), read from the ExaBGP source and quoted in Design Insights; the ExaBGP fixtures are not edited |
| AC-13 | 21 findings in demo topic fragments (`demos/terminal/topics/` bfd 3, ipsec 3, irr 7, ospf 3, rpki 2, vrrp 3) | Each topic's fragments are validated cumulatively in the order its tape loads them, onto the tape's base config; a step the tape shows refused is declared so in the tape and asserted refused; every other step validates (fragments fixed where they do not) |
| AC-14 | `test/interop/scenarios/bgp-delayopen-frr/ze.conf` (prefix maximum missing), `bgp-session-protection-frr/ze.conf` (`bgp filter` unknown, line 25) | Both configs fixed to today's grammar and validate; each scenario's assertion still proves its subject |
| AC-15 | 11 findings in `.ci` merge fragments: `test/reload/bgp-bfd-added-live.ci`, `bgp-bfd-profile-edit-live.ci`, `bgp-filter-irr-loaded-over-ssh.ci` (2 fragments), `bgp-rpki-attach-live.ci` | Each fragment is validated merged onto the config the test loaded before it, in the test's order; the merged configs validate |
| AC-16 | 22 findings in `.ci` bodies refused on purpose with exit 0: `test/plugin/bgp-update-delay-validation.ci`, `config-validate-agrees-with-boot.ci` (`shapes/unknown_internal.conf`), `session-editor-load-refused.ci`; `test/reload/bgp-bfd-profile-reload-deleted.ci`, `commit-verify-reject.ci`, `config-reload-invalid-validator.ci`, `mgmt-guard-reload-refuses-nonloopback.ci`, `pki-reference-reload-broken.ci`, `reload-bad-config.ci`, `static-bfd-profile-reload-deleted.ci` (2), `static-bfd-profile-reload-passive.ci`, `tx-iface-wireguard-invalid-bad-public-key.ci`, `tx-iface-wireguard-invalid-no-private-key.ci` (2), five `tx-ipsec-*-requires-*.ci`; `test/ui/doctor-gnmi-mcp-exposure.ci` (2) | The carrying test declares the refusal on the block, in the `.ci` format documented in `ci-format.md`; the sweep asserts each declared body IS refused, and a declared body that validates is a finding. A body found not deliberate is fixed instead |
| AC-17 | 23 findings in `.ci` fixtures: `test/parse/config-diff-list-order.ci` peers x3, `config-fmt-comments.ci`, `config-fmt-operator-order.ci`, `config-fmt-value-meaning.ci` x2; `test/ui/cli-completion-full-config.ci`, `cli-completion-value-reference.ci` x5, `cli-config-dump-json-sensitive.ci` x2, `cli-config-dump-sensitive.ci` x2, `cli-config-dump-strip-private.ci` x2, `doctor-config-claims.ci` x3; `test/plugin/lg-graph-lab/ze-bgp.conf` (`environment tcp`) | Each fixture validates, its test still asserting what it did (judged where an expectation changes), or, where the invalid config is the test's subject, it is declared refused as in AC-16 |
| AC-18 | `test/parse/policy-root-migrate.ci` `old.conf` (1) | A body the carrying test feeds to `ze config migrate` is validated as that command's output, derived from the test's own command line |
| AC-19 | `$PORT3` in 5 `.ci` files | The runner refuses an unreserved `$PORTn` with a message naming the file and the fix, or reserves it; the 5 tests use only reserved ports and pass; the sweep calls the runner's exported expansion and holds no copy |
| AC-20 | `TestUnknownRedistributeFamilyRefusesLoad` | Green, its config reaching the "no final" path (a defined filter with no final), so it proves the error path it names |
| AC-21 | Gates the previous agent left owed | Scoped `./le go lint` over `./internal/component/config/cli/...` (and every package this spec changes) is clean; `./le arch tier check` accepts the sweep's imports (`internal/exabgp/migration`, `internal/test/plugins/all`, the new leaf packages) |
| AC-22 | A sweep exclusion rule disabled one at a time | `TestSweepDerivesEachExclusion` shows each rule (host code, placeholder render, port expansion, test-only plugins, declared refusal, merge sequence, migrate input, legacy native classification, non-Ze shape) is what turns its findings away; with it removed they return |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-1..AC-18 | |
| `TestSweepDerivesEachExclusion` | `internal/component/config/cli/validate_corpus_test.go` | AC-22 | |
| `TestValidateReportsNodeOfAnotherOS` | `internal/component/config/cli/cmd_validate_host_test.go` | AC-6 code, message and fix text, with an injected OS | |
| `TestValidateReportsHostBackendVerdict` | `internal/component/config/cli/cmd_validate_host_test.go` | AC-7 code from the verifier, not the message | |
| `TestRecordPortExpansionRefusesUnreservedPort` | `internal/test/runner/runner_exec_test.go` | AC-19 | |
| `TestParseAllConfigFiles` | `internal/component/bgp/config/loader_test.go` | AC-5 unchanged behaviour over the shared classification | |
| `TestUnknownRedistributeFamilyRefusesLoad` | `internal/component/bgp/config/loader_create_test.go` | AC-20 | |
| migration tests for `family all` and DSCP | `internal/exabgp/migration/*_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| migrated `traffic-class` | decided by AC-12 from ExaBGP's behaviour | per decision | N/A | per decision |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| every `.ci` AC-3, AC-15..AC-19 touches | `test/parse`, `test/ui`, `test/plugin`, `test/reload`, `test/firewall`, `test/traffic`, `test/install` | Unchanged scenarios still pass | |

## Files to Modify
- `internal/component/config/cli/validate_corpus_test.go` - derived exclusions, cumulative validation, declared refusal assertion
- `internal/component/config/cli/cmd_validate.go` - map host verdicts to their codes
- `internal/component/config/yang_schema.go`, `internal/component/config/parser_freeform.go` - remember an OS-pruned name and refuse it with its OS
- `internal/component/config/backend_gate.go`, `internal/component/iface/register.go`, `internal/component/firewall/engine.go`, `internal/component/traffic/register.go`, `internal/component/vpp/cpuset.go` - mark host verdicts
- `internal/core/diagnostic/codes.go` - register the new codes
- `internal/component/bgp/config/loader_test.go` - read the shared native classification
- `internal/component/bgp/config/loader_create_test.go` - AC-20 fixture
- `internal/test/runner/runner_exec.go` - one exported expansion, `$PORT3`
- `internal/le/interoplab/bgp/prepare.go`, `internal/le/interoplab/ipsec/helpers.go` - call the shared renderer
- `internal/exabgp/migration` - `family all`, DSCP value
- `contrib/netlab/ze/bgp.j2`, `contrib/netlab/golden/r1.conf`, `contrib/netlab/golden/r3.conf`
- demo tapes and fragments under `demos/terminal/topics/` and `demos/terminal/showcase-lab/` as AC-13 finds
- `test/interop/scenarios/bgp-delayopen-frr/ze.conf`, `test/interop/scenarios/bgp-session-protection-frr/ze.conf`
- the `.ci` files named in AC-3, AC-15..AC-19
- `docs/architecture/testing/ci-format.md`, `docs/guide/command-reference.md`, `docs/architecture/config/yang-config-design.md`, `docs/functional-tests.md`

## Files to Create
- `internal/test/labconfig/` - placeholder grammar and renderer shared by interoplab and the sweep (name settled at implementation)
- `internal/test/exabgpnative/` - the native fixture classification, moved from `loader_test.go`
- `internal/component/config/cli/cmd_validate_host_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | none added |
| YANG custom validators | No | none added |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency; new codes register in `codes.go` |
| Prometheus counters/metrics | No | none |
| BGP family surface | No | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the two host-verdict codes of `ze config validate` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` (refusal marker, port placeholders) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (ze:os refusal) |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin, event, command changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | `docs/architecture/config/syntax.md` (declared by the config parser: update if the ze:os refusal changes what it says about unknown fields), `docs/architecture/core-design.md` (declared by changed files: section 14a gains the host-verdict code), `docs/features/ai-first.md` (declared by changed files: unaffected unless it describes `ze config validate` codes; check and say). Rerun `./le spec citation anchors spec plan/release/fixes/spec-config-validate-every-repo-config.md` at implementation |
| 17 | Docs show config examples for this area? | Yes | `contrib/netlab/README.md` if it shows a golden |

## Implementation Steps

1. **Phase: Wiring** - host-verdict codes in `runValidation` and `codes.go`, the two wiring unit tests failing first
2. **Phase: Sweep derivations** - shared classification, shared renderer, runner expansion, test-only plugins, statement-shape classification, declared refusal, merge sequence, migrate input; `TestSweepDerivesEachExclusion`
3. **Phase: Product fixes** - ExaBGP migration (AC-12), redistribute test fixture (AC-20)
4. **Phase: Config fixes** - netlab template and goldens, interop scenarios, demo fragments and tapes, `.ci` bodies (AC-3, AC-13..AC-18), each suite run as it is touched
5. **Phase: Proof** - AC-1 on macOS and the Linux host, AC-2 mutation walk, AC-21 gates

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and the bucket counts still sum to the finding total |
| Correctness | No exclusion matches message text; each reads a code, a declaration or a grammar |
| Data flow | The sweep holds no path, directory or scenario list |
| Rule: no-layering | `expandRecordPorts` and the native classification map are deleted, not kept beside the shared ones |
| Rule: testing | Every changed `.ci` expectation carries a `ze-test-judge` verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Sweep green | `./le job run label sweep command go test -tags <full tags> -run 'TestRepositoryConfigsValidate$' -v ./internal/component/config/cli/` on macOS and Linux |
| Mutation proof | revert and restore of `test/perf/configs/ze.conf`, both outputs pasted |
| No copy of the runner rule | grep for `expandRecordPorts` finds nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The host-verdict codes cannot be reached by a config error on the host it describes (a Linux-only node on Linux is never a host verdict) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The netlab checker validates goldens by exit code, and RFC 8212 is a warning with exit 0, so the checker could never have caught it; the sweep is the check that does.
- ExaBGP behaviour wins for `traffic-class` above 63, because RFC 8955 Section 4.2.2.11 states only a SHOULD: "The six least significant bits contain the DSCP value.  All other bits SHOULD be treated as 0." ExaBGP 4.2 and 5.0 accept up to 0xFFFF (`if number < 0 or number > 0xFFFF:`, `ClassValue`, lib/exabgp/bgp/message/update/nlri/flow.py@4.2.25:355; `MAX_TRAFFIC_CLASS = 0xFFFF`, src/exabgp/bgp/message/update/nlri/flow.py@5.0:37) and send the value as one unmasked octet (`return 1, character(value)`, flow.py@4.2.25:219; `return 1, bytes([value])`, flow.py@5.0:218), so 101 leaves as 0x65. ExaBGP 6.0 decodes that octet as `return NumericValue(int(_number(string)) & MAX_DSCP_VALUE)` (`_dscp`, src/exabgp/bgp/message/update/nlri/flow.py@main:695), which reads 37. The migration (`flowTrafficClassDSCP`) therefore writes each operand from 64 to 255 as `n & 0x3F` with its operator kept and warns naming the route: `traffic-class 101` becomes `traffic-class 37`, the same match on an RFC-following receiver. An operand of 256 or more is never sent by ExaBGP (`bytes([v])` raises), so Ze refuses it.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-10-31 | runner expands only `$PORT2`/`$PORT`, `$PORT3` becomes `<port>3` |
| `earlier-guard-hides-the-better-error.md` | 2026-10-31 | `TestUnknownRedistributeFamilyRefusesLoad` stopped at unknown filter before "no final" |
| `earlier-guard-hides-the-better-error.md` | 2026-11-02 | `doctor-config-reference.ci` refused by validation before doctor's filter-reference check |
| `unwired-feature.md` | 2026-11-02 | `.ci` runner ignores `exit=` on a non-final helper; `expect=` expansion scope undocumented (same `runner_exec.go` expansion sites) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Host verdicts carry a code set by the producer | Match message text in the sweep; run the sweep on Linux only | A hook or test judges what a thing is, never its wording; the operator on a Mac also deserves the honest code |
| The carrying test declares a deliberate refusal, and the sweep asserts it | A list of refused files; infer from test names | One declaration beside the test; an assertion keeps the marker honest |
| Native fixtures keep their existing classification, shared | Delete the legacy files; a second list in the sweep | Readers exist; one declaration (principles) |
| Placeholder renderer moves to a leaf outside `internal/le` | Import interoplab from the config test | The le-tooling door forbids product-side imports of `internal/le` and the tier gate judges direction |

## Known Limitations
- The legacy native fixtures stay "awaiting native conversion" as `legacyExampleFixtureReason` already records; converting them is outside Thomas's request, which is that every config the repository uses passes validation, and the sweep keeps them visible by logging each with its reason.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass: the scoped package tests of every changed package, and the functional suites each touched `.ci` belongs to (`./le test parse`, `ui`, `plugin`, `reload`, `firewall`, `traffic`, `install`), with `./le repo changed scope print both` naming the rest. The full verification is Thomas's release check
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change; the touched interop scenario configs are named for the main thread to run)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-validate-every-repo-config.md` only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- `ze config validate` reports host verdicts by code, set by the producer: `config-node-other-os` (`NodeOfAnotherOSError`, from the `ze:os` pruning in `yang_schema.go`) and `config-host-backend` (`diagnostic.HostVerdict` marking in the backend gate and the iface, firewall, traffic and vpp verifiers), beside `config-kernel-capability`; `codes.go` marks all three `HostVerdict`.
- `TestRepositoryConfigsValidate` sweeps every repository config with zero findings, each exclusion derived from a declaration: Git-derived discovery (`sweepSourceFiles`), the shared placeholder renderer (`internal/test/labconfig`), `runner.ExpandPorts`, the test-only plugin composition, `refused=` on `.ci` blocks, cumulative load sequences for tapes and `.ci` merges, `ze config migrate` input, the shared native classification (`internal/test/exabgpnative`), and the statement-shape rule. `TestSweepDerivesEachExclusion` disables each rule and sees its findings return; `emptyKinds` refuses a sweep in which a carrier kind produced nothing.
- Runner: `$PORT3` reserved and an unreserved `$PORTn` refused; every foreground helper honours its `exit=`; `exit=` refused where the arm never checks it (`classifyStepArm`, `checkExitAssertable`); the parse suite binds `exit=`.
- Product fixes: ExaBGP migration of `family { all; }` and of DSCP values above 63; doctor runs `parsed-config` checks before load validation so `doctor-config-reference` is reported, and the plugin protocol declares that phase once (`pkg/plugin/rpc`).
- Config fixes: hundreds of `.ci` bodies, the netlab template and goldens, two interop scenarios, demo fragments and tapes, as listed per AC in the records above.

### Bugs Found/Fixed
- Runner dropped `exit=` on non-final helpers and on daemon steps (`runOrchestrated`); covered by `TestForegroundHelperCommandExit`, `TestForegroundHelperFileExit`, `TestParseRefusesExitTheArmDrops`, `TestParseSuiteHonoursCommandExit`.
- `$PORT3` expanded to `<port>3`; covered by `TestRecordPortExpansionRefusesUnreservedPort`.
- Doctor's filter-reference check hidden by the load refusal; covered by `TestDoctorReferenceBeforeLoadRefusal` and `test/ui/doctor-config-reference.ci`.
- `parsed-config` absent from the plugin protocol's phase set; covered by `TestRunPluginRegistryChecks_ParsedConfig`.

### Documentation Updates
- `docs/guide/command-reference.md`: the `config-node-other-os` and `config-host-backend` rows.
- `docs/architecture/config/yang-config-design.md`: the `ze:os` row names `NodeOfAnotherOSError` and the code.
- `docs/architecture/testing/ci-format.md`: `refused=`, `$PORT3`, port expansion scope, Git discovery, `exit=` acceptance table.
- `docs/architecture/doctor-and-health-checks.md` ("The phase decides what a check may read") and `docs/guide/health-checks.md`.
- `./le doc check all` (`close-doccheck.log`): source anchors pass; the only red is the foreign `clear debug` wiki/live catalog mode mismatch.

### Deviations from Plan
- AC-4: the EBGP policy is rendered by `contrib/netlab/ze/ze.j2`, not `bgp.j2` as Files to Modify named.
- AC-1 runs on Linux only; the macOS run moves to `plan/post-release/engineering/spec-config-sweep-macos-proof.md` under the owner's macOS rule.
- Discovery became Git-derived (bb4fead1d6), so the sweep no longer reads the gitignored `plan/private/*.conf` (owner question pending).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The handoff recorded `linklocal-only-multihop-withdraw` as an environment blocker needing a privileged guest launcher | An unprivileged user namespace on the Ze-kernel host runs it; that route had never been tried | The parent's proof agent tried it | Recorded in Closure evidence |
| approach | 5dcffd85d0 added a doctor phase to `internal/core/diagnostic` only | The plugin protocol's phase set in `pkg/plugin/rpc` was a second declaration and refused it | Review | ebeecfbc66 declares it once |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Validate every config the repository generates or uses | Done | `validate_corpus_test.go` `TestRepositoryConfigsValidate` | Linux; macOS homed post-release |
| Over-reports removed by derived rules, never a list | Done | `TestSweepDerivesEachExclusion` | nine rules |
| Every real defect fixed, sweep green with zero findings | Done | `close-sweep-v.log` | 2827 configs |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done (Linux) | `close-sweep-v.log` summary line, zero findings | macOS: Work Not Done |
| AC-2 | Done | `job-phase5-ac2-mutation-9171615d.log` | fresh |
| AC-3 | Done | corpus zero findings; `linklocal-only-multihop-withdraw` PASS; parse 426/426 | |
| AC-4 | Done | 28bd5b5c9c netlab check; goldens carry `final accept` | `ze.j2` (Deviations) |
| AC-5 | Done | `internal/test/exabgpnative`, `TestParseAllConfigFiles` | |
| AC-6 | Done (Linux) | `TestValidateReportsNodeOfAnotherOS` | |
| AC-7 | Done (Linux) | `TestValidateReportsHostBackendVerdict` | |
| AC-8, AC-9 | Done | `internal/test/labconfig`, corpus | |
| AC-10, AC-11 | Done | statement shape 3, test plugins 17 | |
| AC-12 | Done | migration commits; Design Insights quotes ExaBGP source | |
| AC-13 | Done | 29 sequences, 71 steps | |
| AC-14 | Done | both interop scenarios PASS | |
| AC-15, AC-16 | Done | load sequence 20; 20 asserted refusals | |
| AC-17, AC-18 | Done | corpus; 84c0fc6cea runs | |
| AC-19 | Done | `TestRecordPortExpansionRefusesUnreservedPort`; five consumers pass | |
| AC-20 | Done | `TestUnknownRedistributeFamilyRefusesLoad` | |
| AC-21 | Done for this spec's code | `close-lint.log`: two foreign in-flight findings remain, owned by spec-lint-backlog-3-test-tooling; tier result names bfd/api only | |
| AC-22 | Done | nine controls PASS | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRepositoryConfigsValidate`, `TestSweepDerivesEachExclusion` | Done | `internal/component/config/cli/validate_corpus_test.go` | |
| `TestValidateReportsNodeOfAnotherOS`, `TestValidateReportsHostBackendVerdict` | Done | `internal/component/config/cli/cmd_validate_host_test.go` | |
| `TestRecordPortExpansionRefusesUnreservedPort` | Done | `internal/test/runner/runner_exec_test.go` | |
| `TestParseAllConfigFiles`, `TestUnknownRedistributeFamilyRefusesLoad` | Done | `internal/component/bgp/config/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/labconfig/`, `internal/test/exabgpnative/`, `cmd_validate_host_test.go` | Done | created |
| `contrib/netlab/ze/bgp.j2` | Changed | policy rendered in `ze.j2` |
| other Files to Modify | Done | per AC commits |

### Audit Summary
- **Total items:** 22 ACs
- **Done:** 22 (AC-1, AC-6, AC-7 on Linux)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-4 template file (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every config the repository generates or uses passes `ze config validate` | functional sweep over the real validation path (`runValidation`) | `close-sweep-v.log`: 2827 Ze configs, 241 migrated, zero findings |
| The sweep catches an RFC 8212 regression like the perf DUT config | mutation | `job-phase5-ac2-mutation-9171615d.log`: four findings with 664be57056 reverted, none restored |
| Each exclusion is what turns its findings away | differential | `TestSweepDerivesEachExclusion`, nine controls |
| Fixed configs still do their job at runtime | functional and interop | linklocal-only-multihop-withdraw, parse 426/426, the reload, static, firewall, plugin and interop selections in the final proof record |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-1 run on macOS, and `test ui doctor-bgp-md5` (`skip-os=linux,freebsd`) | Only a macOS host can run them; `ai/META-DECISIONS.md`: "Production is Linux. A check, test or defect that exists only on macOS waits until after the first release." | `plan/post-release/engineering/spec-config-sweep-macos-proof.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-validate-every-repo-config-cad30e56-065d-40be-a53c-322aefe738fa.md` (47 files: the population's 44 surviving files, both spec files and the citation repoint in `migrate_flow_traffic_class_test.go`) |
| `./le spec review check` | clean (`review_gate: OK`, hashes match) |
| Rounds | 1 |
| Reviewer lenses used | wiring and process boundary (doctor phase in `pkg/plugin/rpc`, SDK, every phase runner), logic and guards (runner `exit=` arms, discovery refusals, empty-kind guard), removed-behavior (deleted walk exclusions, deleted helper arms), docs against producers, style pass on every changed Go file |

Population: every `Spec-work: config-validate-every-repo-config` commit
(175a44e816, bb4fead1d6, 5dcffd85d0, 7b8707e23e, 59ff106163, 5068a67531,
cbc12349c2, ebeecfbc66, d16ef35b3a, b760ebbab9, 99baaf4948, 8b45e5bd2c,
0011ff954f, bf8162f81a) plus f90191eab4 and 26da0eef6e. Pre-checks:
`./le test weakened check base origin/main` names no file of this population
(`close-weakened.log`; its 20 findings are other sessions' l2tp, reactor,
filterapi, ospf and schema-helper tests); `./le repo check` names none either
(`close-repocheck.log`).

Run 1: 0 BLOCKER, 0 ISSUE. NOTEs:
1. `runOrchestrated` quick-exit ze arm reads a wait error that is not an `*exec.ExitError` as exit 0, while the helper arm now fails it as `exit_code_unreadable`. `awaitQuickZe` writes to in-memory builders, so no realistic input produces that error.
2. The helper arm's `exit_code_unreadable` path has no test, for the same reason.
3. `runChecks` names `diagnostic.DoctorPhaseParsedConfig` directly where the other phases use the `doctorCheckPhase*` aliases in `internal/component/doctor/registry.go`.
4. Git discovery now admits two tracked `vendor/` `.conf` files; they classify non-Ze and validate nothing.
5. The final helper accepts both `exit=` and `expect=exit:code=` with different values without a parse refusal (`record_parse.go`); the run then fails visibly, not silently.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none in this run; earlier review rounds fixed 5068a67531, cbc12349c2, ebeecfbc66, d16ef35b3a, 8b45e5bd2c, 99baaf4948 | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/labconfig/`, `internal/test/exabgpnative/` | Yes | `ls -d` at closure |
| `internal/component/config/cli/cmd_validate_host_test.go` | Yes | `ls -d` at closure |
| `test/runner/helper-command-exit.ci` | Yes | `ls -d` at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | sweep green | `close-sweep-v.log`: `--- PASS: TestRepositoryConfigsValidate`, summary line quoted in Closure evidence |
| AC-19 | no copy of the runner rule | `grep -rn expandRecordPorts` finds 0; the sweep calls `runner.ExpandPorts` |
| AC-21 | scoped lint | `close-lint.log`: two foreign findings only |
| AC-6, AC-7 | wiring tests exist | `func TestValidateReportsNodeOfAnotherOS(`, `func TestValidateReportsHostBackendVerdict(` in `cmd_validate_host_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` host verdicts | `TestValidateReportsNodeOfAnotherOS`, `TestValidateReportsHostBackendVerdict` drive `runValidation` | Yes |
| Repository files | `TestRepositoryConfigsValidate` | Yes |
| `.ci` with `$PORT3` | `TestRecordPortExpansionRefusesUnreservedPort` | Yes |
| `ze doctor` reference check | `test/ui/doctor-config-reference.ci` | Yes (5dcffd85d0, red then PASS) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | confirmed | Linux sweep: `host verdicts skipped: []` |
| A-3 | confirmed | 20 declared bodies asserted refused |
| A-4 | confirmed | 3cb09bb0a9 judged expectations |
| A-5 | confirmed | statement shape moves only the three `vtysh.conf` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| command-reference host codes | `internal/core/diagnostic/codes.go` marks `HostVerdict` on `config-node-other-os`, `config-host-backend` | Yes |
| ci-format `exit=` table | `steparm.go` `classifyStepArm`, `stepArm.assertsExit` | Yes |
| doctor phase order | `doctor.go` `runChecks`: pre-config, missing-config, parsed-config, post-config | Yes |
| yang-config-design `ze:os` | `NodeOfAnotherOSError` named in the `ze:os` row | Yes |
</content>
</invoke>
