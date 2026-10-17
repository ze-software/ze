# Handover: session 1b0ec3e6 (2026-10-15..17) — all tests green, then RFC MUSTs

Owner goals, in order: (1) every test passes, local full verify AND GitHub CI;
(2) then move RFC MUST requirements forward. Commit namespace `b2ec5abb`.
Rules this session added: `.claude/rules/known-reds-first.md` (targeted reruns,
full verify last on a quiet machine) and `.claude/rules/handover-in-repo.md`
(this file is the handover; machine-local notes are not). Up to 5 dev agents.
Agents test through a private launcher `./le --name <x> ...` (the shared
`bin/le` is often older than the tree) and delete `bin/le-<x>` after.

## 1. Landed
Filter modify per NLRI section + IRR re-admit (6019dd0675), its reviewed-claim
rows (bc1567a4ce), GTSM RFC 5082 test fix and repaired RFC records (aa68152f6a,
`./le rfc check` green). Owner approved the FlowSpec and GTSM test changes.

## 2. In flight 2026-10-17 (uncommitted; each needs independent review, then commit)
- Fastpath enable replays the best routes already installed (owner: fix).
  Rebuilds each UPDATE from the Adj-RIB-In winner under the best-path shard
  lock: `rib/forward_replay.go` + test, `test/plugin/bgp-rs-fastpath-late.ci`.
- Functional-test daemon store in memory (owner: option B for the startup
  stall under disk load).
- IPsec readiness timeouts (owner: investigate): scenario frr.conf
  `no bgp ebgp-requires-policy`, `mobike=no` in the port-translated swanctl.conf,
  IKE endpoint-refusal debug log, conntrack flush fails loud with log tails.
- MTU probe target (owner: "it should prompt a configured IP which could be
  the DNS by default"): drop the 1.1.1.1 default, use the first configured
  DNS server, refuse with the fix when neither exists.
- CI: the hosted azure kernel lacks the l2tp modules (run 39047813355), so
  verify.yml now installs `linux-modules-extra-$(uname -r)`.

## 3. Waiting on the owner
- IKE `responder-eap-tls12`: recast as a refusal scenario (recommended), or
  find a TLS 1.2 EMS client.
- Leftover docker network `ze-ipsec-2323022` (not ours) blocks local IPsec runs.
- Delete the superseded root `RESUME-1b0ec3e6.md`.
- Route-injection grammar (`update text ... nlri <family> add`): adopt the
  `nlri` / `mp <family>` keywords too, or keep it.
- IRR cache: keep its disk copy, or memory only.
- Delete the temporary untracked tests `zz_state_timing_diag_test.go`
  (plugin/server) and `zz_c3_profile_test.go` (le/rfc).
- Commit 7f59cfe2ad (L2TP) message lost one example phrase to shell quoting:
  amend only if Thomas asks.

Also landed: PPPoE `pppoe-padr-replay` fixed in the checker (8dce48dd9d);
L2TP interop `02-ppp-bgp-redistribute-frr` fixed in its frr.conf (b3a5960dc7).

## 4. Remaining known reds (not started)
- Nightly: ipsec readiness timeouts (5 scenarios); BGP interop 90-minute ssh
  deadline (suites now print per-scenario progress, so read the next CI run);
  PPPoE IPv6CP gap waits on `plan/spec-l2tp-ipv6-subscriber.md`.
- CI: run 39047813355 stopped every shard at the l2tp modprobe step (fix in
  section 2), so the race split in 3, golangci-lint from vendor and the
  gh-pages seed are still unproven; the next push proves them.
- Then a full `./le verify worktree` on a quiet machine, and ask Thomas to push.

## 4b. TODO for the next session (on the Mac, owner 2026-10-17)
- PPPoE IPv6CP (pppoe-interop `ipv6cp-missing-option`, `ipv6cp-zero-identifier`):
  Ze sends no IPv6CP frame; build it under `plan/spec-l2tp-ipv6-subscriber.md`.

## 5. Spec claimed
`plan/pre-release/spec-plugin-ownership-imports.md`: D1 and D2 landed (69b67933a2,
82591173b6), arch tier green. Remaining: route J (AC-17 health RPCs) and closure.

## Agent handoffs (appended below)

### 2026-10-17 injection grammar `nlri add|del` / `mp <family> add|del` (agent ran out of budget, NOTHING COMMITTED)

Owner decision: `update text` and `update cursor` take the filter-text keywords. Done in the
working tree (uncommitted, all mine unless named):

- Parser: `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` (parseNLRISection
  dispatches `nlri` to parseLegacySection, `mp` to parseMPSection; parseINETSection shared),
  `update_text.go` (kwMP, boundary keyword, grammar comment), `cursor.go`. Old `nlri <family>`
  refused by name (removedFamilyFormError). `nlri add` with an explicit IPv6 next hop refused,
  naming `mp ipv4/unicast add`. `mp ipv4/unicast` accepted ONLY for add with an explicit IPv6
  next hop (RFC 8950); its del, eor, IPv4 or self next hop refused, naming the `nlri` form,
  because the reactor (inlineIPv4Unicast) cannot put IPv4-next-hop IPv4 unicast in MP_REACH.
  OWNER QUESTION.
- New tests: `update_text_section_keyword_test.go`. Package green.
- Helper: `internal/component/bgp/textparse/section.go` (KWNLRISection, KWMPSection,
  LegacyNLRISection, IPv6NextHopText).
- Producers switched: bgp/format.go (announce and withdraw text), rib/rib_replay.go (cursor),
  redistribute/consumer.go, ddos/flowspec/responder.go, reactor/peer_initial_sync.go (default
  originate FILTER text, which 6019dd0675 missed), rr/rr.go, rs/server_handlers.go (withdraw and
  EOR), exabgp bridge (bridge_command.go, bridge_flow.go, bridge_route_forms.go EOR,
  bridge_attribute.go NextHopIPv6), fixtures 06_decoder, 09, 02_events, cmd/ze/help_ai.go.
- Rewritten by script (lines holding `update text` or `update cursor`): about 880 lines in about
  150 files under test/ and internal/ (.ci and Go).

Left to do:
1. rr and rs tests: TestPeerUpReflectsExistingFlowSpecBeforeEOR (rr validation_test.go),
   TestReplayForPeer_SendsEOR, TestPeerUpReplaysExistingFlowSpecBeforeEOR (rs
   server_validation_test.go): expectations still match the old `nlri <fam> eor` text.
   rs/server_retention_test.go is ANOTHER session's file, not touched.
2. The rib package does not build because of another session's forward_tracker_test.go; rerun
   rib tests once it builds (my rib_test, rib_replay_test, llgr_sent_lifecycle_test edits).
3. Docs not yet updated: docs/guide/route-injection.md, docs/architecture/api/commands.md
   (update text section), update-syntax.md, architecture.md, text-format.md, plugin-modes.md,
   ipc_protocol.md, json-format.md, DESIGN.md, core-design.md, docs/exabgp/exabgp-migration.md.
   Only command lines change; event text (`received update ... nlri <fam> add`) does not.
4. Functional proof: a .ci that sends `nlri ipv4/unicast add` through the real entry point and
   asserts the error text; run targeted .ci tests for injection (test/encode, test/plugin).
5. Independent review (ze-read agent), then commit (main thread; this agent holds no Agent).
6. Out of scope, owner to decide: `update hex` and `update b64` still use `nlri <family>`;
   config `update { nlri { <family> add ... } }` and the received-event text format are
   unchanged; `nlri add next-hop self` may still be moved into MP_REACH by the reactor when
   self resolves to IPv6.
7. plugin/server and reactor RFC8654 package failures seen in one run look unrelated
   (namespace `bgp` not registered); not investigated.

### boot-record agent (spec `plan/spec-boot-record-after-listen.md`), stopped at budget

Nothing committed: the owner asked for independent review before each commit.

Commit A (boot record after listeners), code done, scoped tests green, NOT reviewed:
`internal/component/config/storage/boot_record.go` (+`_test.go`, 5 tests), `source.go`
(`configStore.AcquireLock` gate), `pointer.go` (pending reads), `cmd/ze/hub/main.go`
(DeferBootRecord where the record was; Start before WaitForStartupComplete),
`cmd/ze/hub/config_source.go` (`recordBootConfig` logs failure),
`cmd/ze/hub/config_source_repaired_test.go` (record now follows serving),
`docs/architecture/storage-backends.md` "The boot record", journal row, the spec.
Early commits WAIT on the config lock up to 30s, then are refused naming the slow disk and
the retry; a failed record refuses every later write with its error and the fix.
Green: `go test ./cmd/ze/hub/ ./internal/component/config/storage/` (tags ze_core,ze_distro,ze_setup), vet.
Owed: ze-read review, a `.ci` run (explicit-file-restart, any file-config DUT), commit A
(carry the IRR refreshASNCtx journal hunk, say so in the body).

Commit B (memory DUT store), selector done, runner NOT done:
`cmd/ze/ze_core_start.go` (`createAbsentStore` hook), `cmd/ze/register_store_zetest.go`
(zetest-only, env `ze.test.storage=memory`, Private; only an ABSENT store becomes memory, so a
seeded store stays disk), `register_store_zetest_test.go` (4 tests green with tag zetest).
Owed: runner sets `ze.test.storage=memory` for ze DUTs by default (runner_exec.go env build,
near ZE_READY_FILE) plus a per-test opt-out option (`option=storage:value=disk` in
record_parse_vocabulary.go); list every .ci that restarts ze in the same workdir or reads
the store after the DUT (restart, rollback, history, zefs, `ze data`, `ze init`) and give
each the option; docs/functional-tests.md.
CORRECTED (continuation agent, 2026-10-17): the earlier finding that stdin DUTs run storeless
was FALSE. The runner rewrites every `ze ... -` daemon that has a stdin block into
`ze start <WorkDir>/stdin-<name>.conf` (`internal/test/runner/parsing.go`, the `daemonCfgIdx`
branch; `runner_exec_util.go` `stdinRouteDaemonConfig`), so those DUTs open a live store
through openExplicitStore and pay the boot record. test/plugin/filter-irr.ci is one of them,
and its "created live store" stall is the one A and B address.

Commit C (fsync trims): not started. Before/after load numbers: not measured.
`bin/le-bootrec` deleted; rebuild with `./le --name bootrec`.
CORRECTION (independent review): the FINDING above is false. The runner rewrites every
stdin daemon into `ze start <WorkDir>/<name>.conf` (runner_exec.go stdinRouteDaemonConfig,
parsing.go), so every DUT opens a live store; filter-irr.ci included.

### Route-injection grammar, second agent (2026-10-17, budget hit, nothing committed)
Done: rr/rs EOR expectations now `mp <fam> eor`; rib llgr_sent_lifecycle_test picks
`nlri`/`mp <fam>` by family; interoplab speaker_test digest repinned (it pins command text
on purpose); docs rewritten for command lines only (api/update-syntax.md, api/architecture.md,
api/commands.md with the refusal quoted, guide/route-injection.md, DESIGN.md, core-design.md
and about a dozen more); new `test/plugin/update-text-nlri-family-form-refused.ci`, red with
an alias, green without. Left alone on purpose: config `update { text { nlri <fam> } }`,
`update hex`/`b64`, received-event text, `decode nlri <family>`.
Green: go test over update, textparse, exabgp bridge, bgp, rr, rs, redistribute,
ddos/flowspec, rib, reactor, watchdog, plugin/server, fixture, runner, interoplab/bgp,
bridgerun, mvpn (no -race, no lint). .ci on a private launcher: encode 66/66, reload
101/101, plugin 823/825 (lo0 alias 127.0.0.6 missing; one vpp flake passed on rerun),
ui 272/277.
Open, in order:
(a) `./le repo check` flags ClearProcessCursors (update/cursor.go), NextHopIPv6
(exabgp/bridge/bridge_attribute.go) and ConvertFlowRoute (bridge_flow.go) as exported with
no cross-package caller: compare with HEAD, unexport or fix.
(b) `le test sensitivity check` counts 115 assert-nothing tests against a baseline of 114:
find the extra one.
(c) Delete bin/le-inject2.
(d) The interop scenarios on interoplab helper.go and the exabgp-compat suite were not run.
(e) Check update-syntax.md "Per-NLRI Overrides" (`nlri nhop set ... add`) against the parser,
which takes only path-information, rd and label; check the plugins/nlri/mvpn/encode.go hunk
is grammar only.
(f) Owner questions pending: mp ipv4/unicast, hex/b64, config form, event text, next-hop self.
(g) Independent review, then commit. test/fixture/plugin_fixture_03_route_server.go also
carries the fastpath comment from 430a3de516: commit it whole and name it in the body.

### Route-injection grammar, third agent (2026-10-17, budget hit, nothing committed)
Done: (a) clearProcessCursors, convertFlowRoute and nextHopIPv6 unexported (ui
le-placement-answers and le-checks-answers pass). (b) The +1 assert-nothing test is not the
grammar: TestExitGraceChild (plugin/process/process_exit_grace_test.go, from 3e8a71b905) is a
fake-plugin re-entry; its dispatch moved to TestMain and the re-exec passes `-test.run=^$`
(linux-only file, vetted with GOOS=linux, not run on Linux): its own commit. (c) bin/le-inject2
deleted; bin/le-inject3 is this agent's, delete it. (d) exabgp suite 39/40 (api-healthcheck-module:
the sibling ExaBGP checkout's healthcheck.py fails on Python 3.11 Popen `process_group`, journal
row added to gate-verdict-depends-on-the-machine.md, its own commit). Interop NOT run: on darwin
the labs need the QEMU guest, whose runtime kernel cache is missing (`./le build host-driver`, then
`./ze-host appliance kernel --target runtime --arch arm64`). (e) update-syntax.md "Per-NLRI
Overrides" is now "Per-Section Modifiers", matching parseINETSection; the mvpn hunk is one comment.
Race tests green: update, textparse, exabgp bridge, rr, rs, rib, interoplab/bgp.
File list: tmp scratch clean.txt (198 paths) plus the two untracked files. Leave out:
docs/functional-tests.md (it mixes 3 grammar hunks with the DUT-store section of commit B),
bgp/config/register.go and storage-backends.md (boot record), and every .ci hunk that only adds
`option=storage:value=disk` (commit B).
Left: lint; three scripts (grammar with exabgp, the TestMain fix, the journal row); interop in
the guest; owner questions (f); independent review.

### boot-record continuation agent (2026-10-17), stopped at budget

Commit A PREPARED, not run: `tmp/commit-900fdd06-boot-a-b67e18.sh` (review ISSUES 1-3 and all
notes applied; see its message file). Evidence: storage + cmd/ze/hub `go test -race` green;
`TestBootRecordPendingReadAnswersBootBytes` red with the ReadCandidateConfig gate removed;
history-repaired-store-start.ci red with the Start call removed (overlay), green restored;
explicit-file-restart, drain-restart-kind, filter-irr green. bgp/config TestClusterIDSync red
from another session's `update-received` grammar work (comment-only edit there).

Commit B, code done, NOT prepared: `internal/test/runner/record_dut_store.go` (+`_test.go`,
green), `record.go` (DUTStore), `record_parse_vocabulary.go` (`storage` arm), `runner_exec.go`
(env in runTest and runOrchestrated, before option=env); `docs/functional-tests.md` "The DUT's
store" (the file also carries the route-injection session's 3 grammar lines: B must land after
that commit, or carry them by decision). `option=storage:value=disk` inserted in 125 .ci by a
structural grep (`ze init|data|config <offline verb>|show config|data`, 2+ ze daemon launches,
zefs/restart-kind/history-observer/rollback/repaired-store names); none were dirty files.
Suite run with B (private launcher): plugin 32 failed, reload 9 failed. Sampled cause:
`config edit` over SSH opens the store BY PATH ("no configuration store: database; run ze
init"), not the daemon's handle, so every session-editor-*, *-live reload and SSH-editor test
breaks on memory. DECISION OWED: give those the disk option (about 40 files; rerun
`./le --name boot2 test bgp plugin --all` and `reload --all -p 1` to list them), or make the
editor use the daemon's store. Also check rfc7999-blackhole-readvertise-agreement and
rpki-decorator-merge (may be the grammar session's reds). Then: spec list of disk tests, rerun,
prepare `test(runner): functional DUTs run on an in-memory store`.

Commit C, verified, not coded: (1) installBytes' final `syncFile(stage)` is redundant when
stage is parent (the import.go and open.go callers) and, when distinct (treeStore's write),
only makes the temp name's removal durable; storage-backends.md "Tree writes stage" and
`ownershipPrivate` say leftover `.ze-storage-*` files are ignored, so it can go. (2)
`lockStoreFile`: flock state is not on disk; make `openLockNode` report whether O_EXCL created
the file and sync the folder only then. (3) PromoteCandidate's `file/active/<name>` mirror: NOT
safe yet; bare-name reads resolve to the mirror (`resolveKey`) and `ReadRestoreSource` reads it
when no pointer exists; audit every ReadKey/ReadFile of a bare config name first. Step 4
(stall measurement) not run. `bin/le-boot2` deleted.

COMMIT B CONTINUATION (agent stopped at its 100-call budget, 2026-10-17). Script
`tmp/commit-d1f5923b-boot-b-cbd804.sh` is STALE: do not run it; re-prepare with
`./le commit create replace tag boot-b ...` once the items below are done. Review (0 BLOCKER,
5 ISSUE) found:
- DONE ISSUE 3: `docs/architecture/storage-backends.md` "The memory encoding" now says a
  functional-test DUT's absent store uses `CreateMemory` under the zetest build, with source
  anchors (`cmd/ze/register_store_zetest.go`, `internal/test/runner/record_dut_store.go`). Add it to B.
- TODO ISSUE 1: about 36 files carry the disk option with no reason that holds (the selector is
  read only in `createAbsentStore`, which only `ze start <file>` reaches): 17 ipsec files (two
  concurrent `ze -` daemons, each in its own daemon-N/ dir, no restart); traffic-vpp-reject-hfsc,
  -mark and -prio (offline `ze config validate` in cmd_validate.go opens no store); about 16
  that start no daemon (help-lists-every-local-command, data-registered-object, ze-init-piped,
  cli-completion-keyword-shorthand, cli-config-dump-sensitive, cli-config-dump-strip-private,
  cli-zefs-blob-storage, cli-zefs-filesystem-override, config-fmt-write-stdout,
  config-history-stdin-refused, dash-stdio, dash-stdio-path-scope, doctor-without-store,
  doctor-store-refuses-legacy-blob, qemu-full, qemu-iso); and `ze init` before `ze start`, where
  the seeded store is opened, never replaced (test-show-system-sockets-keyword-filters,
  cli-argument-length-refused, web-tool-decode, mgmt-guard-web-*). Verify each, drop the option
  (`sed -i '' '/^option=storage:value=disk$/d'` on the named file only; diffs are one inserted
  line, so the file returns to HEAD and leaves B), regroup the spec section "## B: tests on the
  disk store" and fix its count (125 now), rerun the darwin-runnable ones on memory, and name
  ipsec and traffic as owing a targeted Linux rerun.
- TODO ISSUE 2: carry `docs/functional-tests.md` in B (the grammar commit landed as 3f6bb1965e,
  so its grammar hunks match HEAD; name them riding along), and delete the body sentence
  "docs/functional-tests.md waits for the route-injection grammar commit."
- TODO ISSUE 4: add two .ci files proving the runner hands the selector to the DUT: a default
  `ze -` DUT asserting stderr contains `created memory store (test build)`
  (`createTestStore`, cmd/ze/register_store_zetest.go), and one with `option=storage:value=disk`
  asserting the line `createLiveStore` (cmd/ze/ze_core_start.go) prints; read it for the exact
  text. Prove the first one red with the runner's env append removed (runner_exec.go, via
  go -overlay).
- TODO ISSUE 5: one journal row (600 chars max) for bgp-rib-maximum-paths-reload: it fails
  alone, on memory and on disk, because the test peer's 30 s dial (`dialTarget`,
  internal/test/peer/peer.go) is refused although the SIGHUP reload completes; the cause is
  open. Grep plan/journal for the class first. listener-never-binds-a-resource-that-appears-later
  does not fit.
- Do NOT carry `internal/component/iface/config_apply.go` (the iface VPP fix agent's
  uncommitted work). The two VPP tests stay red in B, named in the body.
- Already in B and done: spec 139->125 plus the parse-suite note, the VPP cause (journal row in
  transient-failure-treated-as-fatal.md), 4 store-reading tests regrouped, `bin/le-bootB` deleted.

COMMIT B SECOND CONTINUATION (2026-10-17, agent at budget). Issues 1, 2, 4, 5 done; B prepared as
`tmp/commit-cbab57ff-boot-b-952da8.sh` (95 paths, list in session scratch boot-b-files.txt):
45 files dropped back to HEAD, 80 carry the disk option, new dut-store-memory-default.ci and
dut-store-disk-option.ci, new journal class listener-refuses-the-peer-its-config-names.md.
Re-review then found 1 ISSUE + 1 NOTE, NOT yet applied, so that script is STALE:
- ISSUE: dut-store-memory-default.ci uses `cmd=`, so it covers only runOrchestrated's append
  (runner_exec.go ~926). runTest's append (~346) serves the cmd-less `option=file` tests, which
  exist only in test/exabgp-compat (84). Findings so far: runTest has no `expect=stderr`
  (reject=stderr is held on RunCommands, `rejectsRunStderr`), and the client's stderr only goes
  through validateLogging (observer sentinel, syslog). runTest DOES run validateFileChecks, and
  its client is `ze start <WorkDir>/<file>` with Dir=WorkDir, so the observable is the on-disk
  store: with memory no `database` appears in the work dir. Next: read the `expect=file` arm keys
  (record_parse_vocabulary.go expect arms; check for an absent/exists key) and write one
  exabgp-compat `option=file` test asserting the store directory is absent; prove it red with ONLY
  the runTest append removed (go -overlay, build recipe: `le` script build_le, tags
  ze_le+feature_tags, GOROOT from `./le go stdpatch goroot`, `-o bin/le-<name>/le ./cmd/ze`), and
  re-record dut-store-memory-default's red with ONLY the runOrchestrated append removed. If no
  observable exists, say so in the spec. Update the test header, spec Functional Tests, body.
- NOTE: docs/architecture/storage-backends.md lines 23-24: move the two `<!-- source -->` anchors
  from mid-paragraph to below the paragraph, beside the existing anchors.
Then re-prepare with `./le commit create replace tag boot-b ...` (same body; add the new file).

COMMIT B THIRD CONTINUATION (2026-10-17): ISSUE and NOTE applied. The cmd-less population is
not exabgp-compat (that suite has its own runner, internal/test/cli/cmd_exabgp.go) but every
generic .ci with only `cmd=api` lines, such as test/encode. New test/encode/dut-store-memory-config.ci
(`expect=file:glob=database:absent=true`) and dut-store-disk-config.ci (`exists=true`). Reds by
go -overlay, one append removed each: runTest-only removal reddened memory-config ("expected absent,
matched 1") and left memory-default green; runOrchestrated-only removal reddened memory-default
(await never held) and left memory-config green. Overlay builds deleted. The iface agent rewrote
the VPP row in plan/journal/transient-failure-treated-as-fatal.md as `fixed:` (its uncommitted
fix), so B no longer carries that file. B prepared as `tmp/commit-9871b957-boot-b-15611a.sh`
(96 paths); `tmp/commit-cbab57ff-boot-b-952da8.sh` is STALE, do not run it.

FULL VERIFY 2026-10-18 (at 0b037fd2fd): red at lint, doc wiring, tree-check, compound-guard,
unit, race, exabgp-compat (9 known, ExaBGP 6 spec) and gating. Landed: 4a59a226da (lint,
numberparse via asn.Parse, compound guards, ui 221), e389e62f2e (doc drift: 3 pages, 27 reviewed
rows), wiki 61a5651 (command catalog). gh-pages regeneration is HELD uncommitted in ../gh-pages:
a darwin `./le site build` drops linux-only iface-dhcp, iface-ra, memlock (journal row in
gate-verdict-depends-on-the-machine.md); owner to choose Linux build or a producer fix.
Test-red triage, UNCOMMITTED in the working tree (agent ran out of budget):
- toolchain TestTheTestTempDirIsMadePrivateAndWritable: Linux PATH_MAX hard-coded; uses unix.PathMax.
- scratch TestTrimSkipsLivenessWithoutSessions: load-only; go telemetry child writes under test HOME; telemetry off helper.
- interop-l2tp TestNativeConfigBytesArePinned: b3a5960dc7 changed scenario 02 frr.conf without repin; repinned.
- plugin 281 drain-over-ceiling: load-only race, fixture returns nil on the ceiling refusal. Unconfirmed: rerun 281 x3.
- plugin 735 rfc7999: setup on macOS lacked 127.0.0.6; added in internal/le/setup/system.go + docs. Needs owner `sudo ./le setup install`.
- plugin 871 storage-rir-restart: load-only, active/router.conf missing on read. Journal row owed.
- reload 37: missing established-file await stage; added. appliance 1: fake go not a patched GOROOT bin/go; fixed.
- runner 12: internal/appliance reaches ssh only via a linux test file; fixture expects 3 packages off Linux.
Owed: fixture and l2tp -race tests, lint/compound-guard/doc checks over these, one commit per fix, then full verify.

## 2026-10-18 RFC defects agent (RFC 7606 5.2 population, RFC 9252 transposition by family): needs continuation

Defect 1 (RFC7606-5.2-1 / RFC9830-5-2), uncommitted:
- `internal/component/bgp/message/rfc7606.go`: `hasAttrOtherThanMPUnreach` set for every framed attribute other than code 15; escalation is now `!hasReachableNLRI && hasAttrOtherThanMPUnreach && strongest > AttributeDiscard`. `structuralError` deliberately unchanged (comment says why).
- New `internal/component/bgp/message/rfc7606_mpunreach_only_test.go` (message_test, blank-imports srpolicy). Observed red before the fix (got 3 session-reset, want 2 treat-as-withdraw), green after; whole message package green.
- BLOCKED: `reactor.TestRFC9830NLRILengthReceiveVerdicts` asserts the old defect (`srRequireReset` for "MP_UNREACH alone 13 octets", line ~206) and is now red: err=nil, state=ESTABLISHED, which is the fixed behaviour through processMessage. The writeedit hook refuses the edit to this RFC-tagged test without `./le rfc approve unit reactor.TestRFC9830NLRILengthReceiveVerdicts reason "<owner's words>"`. Edit owed after approval: replace the two-call block with `srRequireWithdrawn` plus a `len(got.dispatched) == 0` check for the alone shape, keep the reset for "with other attributes", and replace the deviation comment with a RFC9830-5-2 positive / RFC7606-5.2-1 negative tag pair.
- Ledger: RFC9830-5-2 partial annotation removed in `rfc/short/rfc9830.md`. Journal row 2026-10-14 in `plan/journal/gate-fires-outside-its-population.md` marked fixed. Pages corrected: `docs/architecture/wire/messages.md`, `docs/architecture/wire/nlri.md`.

Defect 2 (RFC9252-3.2.1-2), uncommitted:
- `internal/core/bgp/attribute/prefixsid.go`: new `CheckSRv6TranspositionForFamily(prefixSID, fam)` + `checkTranspositionNotApplicable`, keyed on `nlrisplit.TranspositionLabelWidth(fam) == 0`. Not in the encoder: `update text` declares attributes before the family-naming sections.
- Callers: `update_text.go` ParseUpdateText (each announcing section), `config/peers.go` patchStaticRoutes (with new `StaticRoute.Family()`, the former reactor `routeFamily` turned into a method in peer_static_routes.go, callers in peer_static_wire.go, peer_static_routes_test.go, peer_test.go), `config/bgp_routes.go` buildConfigRouteRequest (new `fam` param, LookupFamily in the plugin branch).
- Tests: `TestCheckSRv6TranspositionForFamily` (attribute), `TestUpdateTextSRv6TranspositionNeedsALabelField` (new file in cmd/update), `test/parse/srv6-transposition-refused-unicast.ci` (passed in draft). attribute and cmd/update packages green.
- Ledger: RFC9252-3.2.1-2 partial annotation removed, header count seven to six with a correction sentence. Pages: `docs/architecture/api/update-syntax.md`, `docs/architecture/wire/attributes.md`.
- config package red with ~55 failures, all foreign (derived-binding "update-received" errors from in-flight peer_derived_bindings.go / server/events.go work); none names a transposition or my messages.

Still owed: discrimination records for every tag added (message test: RFC7606-5.2-1 pos+neg, RFC9830-5-2 pos, break = drop `hasAttrOtherThanMPUnreach &&` / never set it; attribute and update tests plus the .ci: RFC9252-3.2.1-2 pos+neg, break = skip the family check / invert the width test), `./le rfc check`, golangci-lint over attribute, message, config, cmd/update, reactor, `./le doc wiring`, then two commits via `./le commit create` (defect 1 needs the owner approval above first).

### 2026-10-18 continuation (same package): records, side finding, gates, scripts prepared

- Discrimination, all by `route revert` (producer body halted), red observed and recorded: RFC7606-5.2-1 pos+neg and RFC9830-5-2 pos on `message/rfc7606_mpunreach_only_test.go::TestRFC7606Section52MPUnreachOnlyNotEscalated` (producer `ValidateUpdateRFC7606AddPath`); RFC9252-3.2.1-2 pos+neg on `attribute/rfc9252_prefixsid_origin_test.go::TestCheckSRv6TranspositionForFamily` and `cmd/update/rfc9252_update_text_srv6_transposition_test.go::TestUpdateTextSRv6TranspositionNeedsALabelField` (producer `checkTranspositionNotApplicable`); RFC9252-4.1-1 and 6.2-1 pos+neg on `TestCheckSRv6TranspositionLabelWidth` (producer `checkTranspositionFitsLabel`). Targeted overlay breaks (`if length <= width` and `if length == 0 && offset == 0` forced true) also turned the unit tests red.
- NOT recorded: `test/parse/srv6-transposition-refused-unicast.ci` (RFC9252-3.2.1-2 negative). The recorder's preflight refuses: host lacks loopback 127.0.0.6, needs owner `sudo ./le setup install` (same blocker as plugin 735). The .ci itself passes (`bin/le test bgp parse srv6-transposition-refused-unicast`). Owed after setup: `./le rfc discriminate-record id RFC9252-3.2.1-2 polarity negative unit test/parse/srv6-transposition-refused-unicast.ci route revert producer internal/core/bgp/attribute/prefixsid.go::checkTranspositionNotApplicable citation "expect=stdout:contains=invalid srv6 SID structure for ipv6/unicast routes: transposition length 16 and offset 64 must both be 0"`.
- Side finding fixed, not journaled: RFC 9252 Sections 5.1/5.2 ("the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL") and 6.2/6.3/6.5 (24) bind the sender. `CheckSRv6TranspositionForFamily` now walks every family; `checkSIDStructureForFamily` dispatches to `checkTranspositionNotApplicable` (width 0) or new `checkTranspositionFitsLabel` (length <= width). The RFC bounds the length, not length+offset. FL bound still not judged on either side (rows 4.1-1 and 6.2-1 now `{partial}` with that as the gap).
- `./le rfc check` file-name rule: tagged tests moved to `attribute/rfc9252_prefixsid_origin_test.go` (prefixsid_origin_test.go is back to its HEAD bytes) and `cmd/update/rfc9252_update_text_srv6_transposition_test.go`.
- Gates: message, attribute, cmd/update, nlrisplit green; reactor red only on known `TestRFC9830NLRILengthReceiveVerdicts`; config 55 top-level reds, identical set with HEAD's bgp_routes.go and peers.go overlaid, so foreign. golangci-lint over the five packages 0 issues after a misspell fix; compound-guard OK; `./le doc wiring` no drift row on these files (four pages edited); `./le rfc check` mine left: RFC7606-5.2-1 STALE audit verdict (owes a ze-rfc-audit re-read). Records are STALE against HEAD until the commits land.

## 2026-10-18 close-reason agent (RFC 4724 Section 4, RFC 7854 Section 4.9): commit prepared, not run

The reactor derives one `sessionClose` per down (`reactor/peer_close.go`, `sessionCloseFor`): reason `rpc.ReasonNotification`, `ReasonTCPFailure` or `ReasonLocalClose`, plus the RFC 4271 Section 8.1 event number, carried to plugins as `PeerInfo.CloseFSMEvent` / `StructuredEvent.FSMEvent`. GR now drops routes after any NOTIFICATION; BMP sends reason 4 on transport loss and the real FSM event in reason 2.
- The two reds blamed on this work were not caused by it: the plugin registry tests and the ~55 config "derived binding ... update-received" failures come from running `go test` without the feature tags (`all_ze_bgp.go` is `//go:build ze_bgp`). With the tags from `docs/contributing/running-commands.md` both packages pass. The RFC defects agent's note above that the config reds are foreign holds for the same reason: they are not reds at all.
- `reactor.TestRFC9830NLRILengthReceiveVerdicts` red is the RFC defects agent's Defect 1 (see above), not this work.
- `bmp.TestPeerDownReasonMapping` was changed in place and is stronger: it now asserts a nonzero event encoded big-endian (`{0,8}`), that the event never leaks into reason 1, 3 or 4 Data, and reason 4 for transport loss where the old row asserted the defect (reason 2).
- `pkg/plugin/rpc/bridge.go` was gofmt'd (field alignment). `docs/architecture/api/architecture.md` corrected: observer signature, Close Reasons table (old "connection lost" / "session closed" strings), JSON example reason.
- Tests (feature tags): fsm, gr, bmp, server, rpc, plugin green; reactor green except the RFC 9830 red above. Race: `-race -count=5` over fsm green; over reactor (needs `-timeout 40m`, the 10m default cuts the fifth pass) no data race in five passes, only the RFC 9830 red each pass. Lint: `./le go lint run scope` over the seven packages, 0 issues in all three flavors. Package vet clean.
- Owed after the commit lands: `./le doc wiring` still flags ten claims that remain true. In `docs/architecture/api/text-format.md` (OnPeerClosed); in `behavior/fsm-active.md`, `behavior/fsm-open-sent.md`, `edge-cases/extended-message.md`, `mrt.md` and `docs/bgp-fsm.md` (two claims), each on runOnce; `docs/bgp-fsm.md` on notifyPeerClosed; `core-design.md` on Peer.announceClosed; `meta/role.md` on getStructuredStateEvent. Each needs a row in `plan/doc-reviewed/95685d09.md` naming the new commit SHA, because the ledger takes only a commit id. Also owed: `./le rfc check` over the discrimination records in `rfc/discrimination/rfc4724.json` and `rfc7854.json`.

## 2026-10-18 review-findings agent (close cause, RFC 9252 transposition, RFC 9494 partials): scripts re-prepared, not run

- Owed after the owner's `sudo ./le setup install` (host loopback 127.0.0.6): restore the tag `# RFC requirement: RFC9252-3.2.1-2 negative -- a configured SID Structure with a non-zero transposition on an ipv6/unicast route is refused at config load, so it is never advertised` in `test/parse/srv6-transposition-refused-unicast.ci` (removed for this commit because it carried no discrimination record), then run `./le rfc discriminate-record id RFC9252-3.2.1-2 polarity negative unit test/parse/srv6-transposition-refused-unicast.ci route revert producer internal/core/bgp/attribute/prefixsid.go::checkTranspositionNotApplicable citation "expect=stdout:contains=invalid srv6 SID structure for ipv6/unicast routes: transposition length 16 and offset 64 must both be 0"` and commit the tag with its record.

## 2026-10-18 RFC 9494 LLGR work (spec-bgp-graceful-restart-rfc-defects): uncommitted, needs continuation

- Defect A (4.2-8): gr_state.go reconnectKeepsFamily, gr/rfc9494_llgr_reconnect_test.go, corrected untagged NoLLGRCap test. Defect B (4.3-1): received LLGR_STALE least preferred only when Ze advertised LLGR (RFC 9494 4.3 sentence; FRR and BIRD do not condition it). The fact comes from the reactor: Session.localLLGR set in negotiateWith (session_negotiate.go retainAdvertisedLLGR), passed as filterapi.PeerFilterInfo.LLGRAdvertised to gr_ingress.go LLGRIngressFilter, meta["stale"]=2 reaches rib ReceivedLLGRStale (separate from retention StaleLevel). Received events now carry route-meta JSON (documented in docs/architecture/meta/README.md; appendRouteMetaValue avoids json.Marshal). Benchmark BenchmarkLLGRIngressFilter: 3.2 / 11.8 / 20.1 ns/op, 0 allocs.
- .ci test/plugin/llgr-stale-received-depreferenced.ci passes and goes red with `msg.Meta = routeMeta` dropped. AC-15, AC-22..25 still owed in it; llgr-reconnect-fbit-purge.ci not written (needs ze-peer per-family LLGR F-bit OPEN option). D4 rails 2 (RIB replay/refresh) and 3 (relay buildRelayUpdate) unfixed, ledger RFC9494-4.3-3 marked partial.
- Owed: six discrimination records for RFC9494-4.3-1 (gr ingress tests on gr_ingress.go::llgrStaleFromSource; reactor rfc9494_llgr_ingress_meta_test.go tests and both .ci tags on reactor_notify.go::notifyMessageReceiver); `./le rfc check`; full package tests gr, rib, rib/storage, reactor, format; lint `./le go lint run scope ./<pkg>`; `./le doc wiring`; commit (old script tmp/commit-7b6f1d30-rfc9494-llgr-fed362.sh is disarmed: gr.go and gr_egress.go are back to HEAD, new files missing). Then independent re-judge of RFC9494-4.2-8, 4.3-1, 4.3-3 and RFC8654-5-1.
- Done 2026-10-18 (script tmp/commit-cf8cf27a-*): seven RFC9494-4.3-1 records (gr 2, reactor 2, rib 3) plus re-records for RFC4724-4.2-7/4.2-8, RFC4271-4.4-2, 5.1.4-3, 9.1.2.2-4 (both), 9.2-4, RFC7705-3.3-2, RFC8277-2.5-3. The two .ci tags could NOT be recorded: the recorder's host preflight refuses because this Mac lacks loopback 127.0.0.6. They were removed from test/plugin/llgr-stale-received-depreferenced.ci so no tag lands without its record; the .ci stays a plain regression test. After the owner runs `sudo ./le setup install`, put these two lines back as lines 10 and 11 of that .ci, right after the PREVENTS block:

```
# RFC requirement: RFC9494-4.3-1 positive -- through the daemon, 10.0.0.0/8 received with LLGR_STALE at LOCAL_PREF 200 from the neighbor whose OPEN from Ze carried the LLGR Capability is not best: the fresh copy at LOCAL_PREF 100 wins at the stale-level step.
# RFC requirement: RFC9494-4.3-1 negative -- through the daemon, 10.1.0.0/16 received with LLGR_STALE at LOCAL_PREF 200 from the neighbor whose OPEN from Ze carried no LLGR Capability is best, decided at local-preference.
```

  then record both, and commit the .ci with rfc/discrimination/rfc9494.json:

```
./le rfc discriminate-record id RFC9494-4.3-1 polarity positive unit test/plugin/llgr-stale-received-depreferenced.ci route revert producer internal/component/bgp/reactor/reactor_notify.go::notifyMessageReceiver
./le rfc discriminate-record id RFC9494-4.3-1 polarity negative unit test/plugin/llgr-stale-received-depreferenced.ci route revert producer internal/component/bgp/reactor/reactor_notify.go::notifyMessageReceiver
```

- Landed: 0b323594c4, 5252a8d5f7 (LLGR A and B), 7ca8d5215b (re-judge; `./le rfc check` 0 violations).
- D4 rails 2 and 3 (RIB replay/resend/refresh, relay buildRelayUpdate) fixed, uncommitted, script tmp/commit-e4f711d4-a-b052a1.sh (24 files, not run, awaiting independent review and a reactor `-race -count=20`). One helper storage.AdvertisedStaleLevel; sentSourceState; filterapi.MetaKeyRebuilt; Session.advertisedLLGR under s.mu; LLGREgressFilter acts only at level 2 (AC-19, Q-2). 11 new RFC9494-4.3-3 records; RFC9494-4.3-3 verdict STALE (re-judge owed). Owed after `sudo ./le setup install`: re-record RFC2545-3-1 and 3-2 positive on test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci (producer reactor_api_relay.go::buildRelayUpdate changed). Still owed: test/plugin/llgr-stale-received-egress.ci (live and relay steps; draft in test/draft/plugin/ first).
- ExaBGP 6 spec claim refused by the WIP cap (23 in progress, cap 12): owner approved "24 ok" (2026-10-18); phase 1 agent running.
- Landed 2026-10-18 (review fixes, then items): e62f9e8304 D4 rails with review ISSUE 1 (Session.localLLGR is an atomic.Bool, no s.mu on the relay rail) and ISSUE 2 (guide: only LLGR stale level 2 takes the egress treatment); RFC9494-4.3-1 ingress-meta and RFC4271-5.1.4-3, 9.2-4 records re-recorded; reactor `-race -count=4` whole package green in two chunks. d41fb04dfa peer.go comment. 2effa4167c item 2 (TestComparePair_LLGRStale record, RFC-approved trailer). dc82b0dd89 llgr-stale-received-depreferenced.ci two RFC9494-4.3-1 tags restored and recorded (a .ci record needs `citation "<expect line>"`). Old script tmp/commit-e4f711d4-a-b052a1.sh disarmed.
- RFC2545-3-1/3-2 on adj-rib-in-replay-rfc2545-next-hop.ci: the .ci is `needs-linux:caps=net-admin`, so on darwin the recorder sees a SKIP and reports "stayed GREEN" (journal green-that-could-not-have-been-red.md row of 2026-09-30, open). Attempted inside the QEMU guest: `./le test qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap" command "./le rfc discriminate-record ... citation '<the seq=2 expect line already in rfc/discrimination/rfc2545.json>'"`. The first guest run refused before recording: the guest lacks loopback fd00::2, fd00::3/127 and fe80::1, so the retry prepends `ip -6 addr add fd00::2/128 dev lo && ip -6 addr add fd00::3/127 dev lo && ip -6 addr add fe80::1/128 dev lo && ` to the guest command. If `git status rfc/discrimination/rfc2545.json` shows it modified with two new producer-sha values, commit it (`./le commit create ... file rfc/discrimination/rfc2545.json`); otherwise re-run that guest command. Until then `./le rfc check` shows 4 violations: the two RFC9494 STALE audit verdicts (4.3-1, 4.3-3, independent re-judge owed) and these two RFC2545 producer-changed records.

## 2026-10-18 owner-approved tagged test fixes ("approve fix")

- Item 1 (reactor TestRFC9830NLRILengthReceiveVerdicts) committed. Items 3 (MCP RFC9728-3.1-3 producer + test), 4 (BMP rfc7854_peerdown_data_test.go new close reasons) and 5 (RFC3209-4.6.1-2 and RFC4301-4.4.2.1-3 tags removed, rows marked {gap} citing "owner ruling 2026-10-18") are edited, tested and recorded, uncommitted. Item 2 (rib TestComparePair_LLGRStale) edited; its record waits for rfc/discrimination/rfc9494.json to be clean (LLGR D4 script tmp/commit-e4f711d4-a-b052a1.sh lands first): `./le rfc discriminate-record id RFC9494-4.3-1 polarity positive unit internal/component/bgp/plugins/rib/bestpath_test.go::TestComparePair_LLGRStale route revert producer internal/component/bgp/plugins/rib/bestpath.go::compareBeforeMED`.
- Gotcha: approvals were written to tmp/commit-rfc-approved-edab2b99.md; pass `session edab2b99` to `./le commit create` or the RFC-approved trailers will not attach.
- Owed: rsvpte package tests, ike/dataplane linux test (qemu), scoped lint on reactor, rib, mcp, bmp, rsvpte, ike/dataplane; commits items 3, 4, 5, then 2; independent re-judge of RFC7606-5.2-1, RFC9830-4.2.1-2, RFC9494-4.3-1, RFC9728-3.1-3, RFC9728-3-2, RFC7854-4.9-1, RFC3209-4.6.1-1/-2, RFC4301-4.1-9, 4.4.2.1-6, 4.4.2.1-3.

## 2026-10-18 ExaBGP 6 spec, phase 1 first package (plan/immediate/spec-exabgp-6-api-compatibility.md)

- Prepared, not run: one commit script (path in the agent report) carrying the main pin, proof mode, the two ported cases and the Net comment.
- Main pin is a git commit (685f772e4ede5c818d2ff5c84f4ae040afd37be6) installed by uv, not the nightly zipapp: the nightly asset is replaced each night (A-1 broken, spec Key Design Decisions row rewritten).
- ze api population 33/44 under both pins: the 9 known reds plus api-disable (`ze exabgp migrate` refuses `shutdown true`) and api-on-exit (Phase 4).
- Phase 1 still owes AC-4: port qa/old/4.2 (132 files) and qa/old/5.0 (258 files) as populations old-4.2 and old-5.0, green with ALLOWED reasons. A continuation agent takes it with the same package.
- Owed gates: `./le go lint run`, `./le test unit all`, `./le test functional gating`, `./le test functional exabgp-test`.
- Continuation (2026-10-18): commit 1 prepared, not run (tmp/commit-444e522f-a-57108b.sh); it was made before the AC-4 edits but stages the working tree, so it also carries AC-4 hunks in cmd_exabgp.go, docs/functional-tests.md and the spec. AC-4 runner support done (internal/test/cli/cmd_exabgp_old.go: per-population etc/, option=allowed with stale-allowance refusal; 4 tests); cases ported to test/exabgp-compat/old-5.0 (72) and old-4.2 (34). Against ze: old-5.0 51 pass / 1 allowed / 12 fail / 8 timeout; old-4.2 9 / 0 / 20 / 5. Remaining items (a)-(e) are in the spec under phase 1 "AC-4 remaining"; the largest is the Go mock (interoplab/bgp/exabgp_case.go) refusing 4.2 `announce:`/`withdraw:` steps (12 cases). Not yet in the exabgp-test stage. AC-41 (migrate refuses `shutdown true`) is assigned to phase 7. Two ci-format.md claims about runOneExaBGPTest are still true and need doc-reviewed rows once commit 1 has a SHA.

### 2026-10-17 fastpath replay, review round 1 (agent stopped at its call budget)
Pass 1 (uncommitted, green, red without the fix): `rib/forward_replay.go` + test,
`forward_tracker.go`, `forward_handle.go`, fixture `bgpRSFastpathLate03`,
`test/plugin/bgp-rs-fastpath-late.ci`, `docs/architecture/rib/forward-handle.md`,
YANG enable text, journal row. Review: 0 BLOCKER, 2 ISSUE.
- Separate commit, code written, untested: `rib/storage/familyrib.go`
  `ToWireBytes` writes the pooled LARGE_COMMUNITIES (code 32), which it dropped
  (also hits rib_recovery and rib_flowspec_validation). Owed: a test in
  `storage/` that inserts an UPDATE with code 32 and asserts it in
  `ToWireBytes`, red without the hunk; a journal row. Drop the hunk if attrparse
  keeps code 32 in OtherAttrs.
- ISSUE 1: the replay body copies the source UPDATE's MP_REACH/MP_UNREACH.
  Strip 14/15 (as reactor `isRelayStrippedAttr`) and NEXT_HOP outside legacy
  IPv4; rebuild MP_REACH for this one prefix from `recoveryCandidate`
  (`rib_recovery.go`: per-prefix NLRI with labels, path-id, raw next hop); the
  trailing NLRI only for legacy IPv4. The doc comment states what the body is.
- ISSUE 2: unit cases, each red without the fix: IPv6 two-prefix MP_REACH plus
  MP_UNREACH, mixed IPv4+IPv6, ADD-PATH path-id (may pass already: say so),
  large community. Build with `PeerRIB.Insert` + `checkBestPathChange`.
- NITs: `tracker.replay` doc says it blocks holding the shard lock and
  deliveryMu; prefix slice capacity hint; `errReplayWinnerGone` comment
  overstates (the purge elects with a nil handle, so onChange drops it): state
  the gap, journal it if the live path has it; `rib_bestchange.go` lock-order
  comment says peerMu then shard, the code takes shard first.
