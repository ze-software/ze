# Handover: session 1b0ec3e6 (2026-10-15..17) — all tests green, then RFC MUSTs

Owner goals, in order: (1) every test passes, local full verify AND GitHub CI;
(2) then move RFC MUST requirements forward. Commit namespace `b2ec5abb`.
Rules this session added: `.claude/rules/known-reds-first.md` (targeted reruns,
full verify last on a quiet machine) and `.claude/rules/handover-in-repo.md`
(this file is the handover; machine-local notes are not). Up to 5 dev agents.
Agents test through a private launcher `./le --name <x> ...` (the shared
`bin/le` is often older than the tree) and delete `bin/le-<x>` after.

## 1. Commit pending (do first)
Uncommitted and reviewed four times: the filter modify now applies per NLRI
section, and bgp-filter-irr re-admits held routes by ROUTE-REFRESH.
- Filter text format (owner decision): `nlri add|del <prefixes>` = legacy NLRI
  and Withdrawn Routes; `mp <family> add|del <prefixes>` = MP_REACH/MP_UNREACH,
  any family. No shim. Reactor: `filter_nlri_delta.go` (subset per section,
  path-ids and labels kept, refuses additions, reorder, duplicates and
  unrewritable families), `AttrModReplaceNLRI`, and the subject rendered from
  route keys (ADD-PATH and labeled NLRI no longer reach a filter as junk).
  Interop scenario `bgp-policy-modify-mp-frr` (raw injector to FRR) is green,
  and red with the fix stubbed.
- Commit subject: `fix(bgp): filter modify applies per NLRI section; IRR re-admits routes`.
  The file set is every uncommitted path under `internal/component/bgp/`, the
  fixtures `plugin_fixture_05.go` and `plugin_fixture_07*.go`, `internal/test/mock/irr`,
  the `.ci` files `filter-irr-readmit`, `prefix-filter-modify-mp`,
  `prefix-filter-modify-partial` and `dynamic-peer-applies-group-filters`, the
  scenario directory `test/interop/scenarios/bgp-policy-modify-mp-frr/`,
  `internal/le/interoplab/bgp/register_policy_modify_mp.go`, the touched docs and
  `plan/journal` rows, and `test/weakened/b2ec5abb.md`.
- **Blocked on the owner:** RFC-tagged `filter_prefix.TestPrefixPolicyFiltersFlowSpecDestination`
  (`rfc8955_flowspec_test.go`): its only change is the subject prefix `"nlri "` → `"mp "`.
  Needs Thomas's explicit words, then `./le rfc approve unit filter_prefix.TestPrefixPolicyFiltersFlowSpecDestination reason "<his words>"`.
- IRR items #6 (rewritable widens to MP CIDR blocks incl. mpls-label, ending
  the refresh loop for them) and #7 (marker blocks echoed) are done and
  reviewed; the final review items are applied (compound guards split,
  filter_prefix reads filtertext.CIDRFamily, marker-echo rule in
  process-protocol.md, filter-irr-modify-mp.ci proven red/green). New file
  set additions: `internal/component/bgp/filtertext/cidr*.go`,
  `filter_irr/testmain_test.go`, `test/plugin/filter-irr-modify-mp.ci`, two
  journal rows. Gates green: race tests, irr 6/6, filter 35/35, scoped lint,
  compound-guard. Only `./le doc wiring check` stays red on the five claims
  below, which need the post-commit rows.
- After the commit: add six reviewed-claim rows to `plan/doc-reviewed/b2ec5abb.md`
  naming the new SHA, one symbol each: `runIngressPolicyChain` and
  `runEgressPolicyChainASN4` (api/commands.md), `buildModifiedPayloadWithLimit`
  (edge-cases/extended-message.md), `parseFilterAttrsInto` (perf-round-3.md),
  `mpReachNextHopHandler` (update-building.md), `runEgressPolicyChainASN4`
  (wire/attributes.md). Then `./le doc wiring check` and commit.

## 2. Waiting on the owner
- GTSM RFC 5082-tagged tests (nightly integration): the fault is the TEST.
  Commit 7278126c3f moved nft rule counters after the matches, and the test
  helpers still assume a counter at the front of each rule. Proposed: helpers
  `awaitTermMatched`, `termsMatched` and `assertNoTermMatched`; the drop test
  asserts exactly the quoted-destination term matched; one assertion is lost
  (proof the chain evaluated a non-matching packet; the ICMP receive counter
  and the installed-table check remain). Proven green, and red under a mutant
  of the drop terms. Needs owner approval, then re-record RFC5082-3-2 and -3-4.
- Startup stall under disk load: about 36 fsyncs before the first listener.
  Options A (record the boot config after the listeners), B (functional DUT
  store in memory), C both (recommended).
- Fastpath enable ignores best routes already installed (journal row): fix or not.
- Route-injection grammar (`update text ... nlri <family> add`): adopt the
  `nlri` / `mp <family>` keywords too, or keep it.
- IRR cache: keep its disk copy, or memory only.
- Delete the temporary untracked tests `zz_state_timing_diag_test.go`
  (plugin/server) and `zz_c3_profile_test.go` (le/rfc).
- Commit 7f59cfe2ad (L2TP) message lost one example phrase to shell quoting:
  amend only if Thomas asks.

## 3. Done since this handover was first written
PPPoE `pppoe-padr-replay` fixed in the checker (8dce48dd9d); L2TP interop
`02-ppp-bgp-redistribute-frr` fixed in its frr.conf (b3a5960dc7). No agent
is running.

## 4. Remaining known reds (not started)
- Nightly: ipsec readiness timeouts (5 scenarios); BGP interop 90-minute ssh
  deadline (suites now print per-scenario progress, so read the next CI run);
  PPPoE IPv6CP gap waits on `plan/spec-l2tp-ipv6-subscriber.md`.
- CI: the next push proves userns, the l2tp modprobe step, the race split in 3,
  golangci-lint from vendor and the gh-pages seed.
- Then a full `./le verify worktree` on a quiet machine, and ask Thomas to push.

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
