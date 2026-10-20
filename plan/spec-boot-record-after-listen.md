# Spec: boot-record-after-listen

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | - |

## Task

Under fsync load ze stalls before its first listener (plan/journal/startup-wait-expires-under-load.md,
rows 2026-10-09 and 2026-10-17). Owner decision (Thomas, 2026-10-17), three commits:

- A. Record the boot config in the store AFTER the listeners start, not before. The owner lock,
  the SSH host key write and `recoverFileCommit` stay before serving. Every config-change path
  waits for the record (bounded) or is refused with a user-facing error. A failed record is
  reported. A crash between serving and the record is repaired by the next start.
- B. The functional-test DUT uses the in-memory store (`storage.CreateMemory`); tests that
  exercise the disk store keep it through an explicit per-test option.
- C. Trim redundant fsyncs in the storage write path (staging-folder sync in `installBytes`,
  the folder sync of an existing lock in `lockStoreFile`, the `file/active` mirror sync), each
  verified at its producer and against docs/architecture/storage-backends.md.

## Required Reading

### Architecture Docs
- [ ] docs/architecture/storage-backends.md, "Durable publication", "Version pointers and recovery"
  -> Constraint: a durable version precedes its pointer; object before entry.
- [ ] docs/architecture/hub-architecture.md, explicit-file commit authority
- [ ] ai/rules/config.md: env-only is allowed for a bootstrap value read before config parses.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/config_source.go` - initializeConfigSource records the boot config; promote, recover, runtime commit
- [ ] `cmd/ze/hub/main.go` - startup order: recoverFileCommit, clear stale candidate, load, record, listeners, ready file
- [ ] `internal/component/config/storage/source.go` - configStore binding every consumer borrows
- [ ] `internal/component/config/storage/pointer.go` - candidate, promote, active pointer API
- [ ] `internal/component/bgp/config/register.go` - reactor reads candidate then active at start
- [ ] `cmd/ze/ze_core_start.go` - openExplicitStore creates an absent store
- [ ] `internal/component/config/storage/memory.go` - CreateMemory

**Behavior to preserve:** owner lock, host key and recoverFileCommit before serving; ordering of
durable writes; every config commit semantics after the record.

**Behavior to change:** the record moves after the listeners (A); DUT store (B); syncs (C).

- `cmd/ze/hub/main.go` runYANGConfig calls `initializeConfigSource` (config_source.go) after
  config load and kernelcap, before any listener. In file mode it writes candidate + promotes
  (object, entry, candidate pointer, rollback, active pointer, mirror, candidate removal), each
  durable write syncing file and directories.
- Readers of the recorded version during startup: `bgp/config/register.go` (ReadCandidateConfig,
  then ReadActiveConfig) when the reactor starts.
- Writers: SSH editor (`cli/editor_commands.go`, `cli/editor.go`), web editor (`web/editor.go`),
  `commitRuntimeConfig`, `request reload`/SIGHUP (`main_reload.go`), managed (`managed.go`),
  confirm window, data restore. All reach pointer.go through the bound `configStore`.
- `openExplicitStore` (`cmd/ze/ze_core_start.go`) creates an absent store with `storage.Create`.
- Functional-test DUTs written as `exec=ze ... -` with a stdin block are NOT storeless: the
  runner rewrites them into `ze start <WorkDir>/stdin-<name>.conf` (`internal/test/runner/parsing.go`,
  the `daemonCfgIdx` branch; the `stdinRouteDaemonConfig` branch in `runner_exec.go`), so they open a live
  store through openExplicitStore and pay the boot record. test/plugin/filter-irr.ci is one.
  -> Decision: an earlier handoff said otherwise; corrected 2026-10-17.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`ze start <file>` / `ze <file>` -> openExplicitStore -> BindConfigSource -> hub.Run.

### Transformation Path
A: runYANGConfig arms a boot record on the bound handle (`storage.DeferBootRecord`) where it
used to record synchronously; after `writeReadyFile` the record runs once in a lifecycle
goroutine against an ungated view of the same handle. pointer.go functions consult the gate.

### Boundaries Crossed
hub -> config/storage (gate lives on `configStore`, the handle every consumer borrows).

### Integration Points
`configStore` (source.go); pointer.go `WriteCandidateVersion*`, `PromoteCandidate`,
`ClearCandidate`, `EnsureActiveVersion`, `ReadActiveConfig`, `ReadCandidateConfig`.

### Architectural Verification
- [ ] No consumer holds an unbound handle (only hub calls BindConfigSource; every consumer borrows it).

## Risks & Assumptions

### Assumptions
- The boot bytes ARE the accepted config for this run; making them durable is the only work the
  record adds. So a read while the record is pending is answered from those bytes.

### Risks
- A writer reaching the store through a handle that is not the bound one bypasses the gate.

## Blast Radius
Every daemon start; every config commit in the first moments after start.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start <file>` boot, then an SSH/web/API commit | → | `storage.DeferBootRecord`, `BootRecord.Run`, pointer.go gate | `TestBootRecordWriteWaitsThenLands`, `test/plugin/explicit-file-restart.ci` |
| `ze` zetest DUT with `ze.test.storage=memory` | → | `openExplicitStore` selector | `TestStoreSelector*` |

## Acceptance Criteria

| AC | Statement |
|----|-----------|
| AC-1 | A daemon started from an explicit file serves before its boot config is durable in the store |
| AC-2 | A config write (candidate, promote, clear, ensure) arriving before the record completes waits for it, up to 30s, then fails with an error naming the cause and the retry |
| AC-3 | A read of the bound config name while the record is pending answers the boot bytes |
| AC-4 | A failed record is logged at error level with the fix, and every later config write returns that failure |
| AC-5 | The record lands: after it completes, the active pointer names the boot bytes |
| AC-6 | (B) A zetest DUT given `ze.test.storage=memory` and no existing store runs on CreateMemory; a shipped build has no such path |
| AC-7 | (B) Every disk-store .ci test keeps the disk store (listed below) |
| AC-8 | (C) Each trimmed sync carries a comment why, docs updated, ordering unchanged |

## B: tests on the disk store

`option=storage:value=disk` is carried by 80 .ci files, grouped by the first reason that applies. The selector is read in one place, `createAbsentStore` (cmd/ze/ze_core_start.go), which only a `ze start <file>` finding no store reaches; the runner turns `ze -:stdin=<block>` into `ze start <path>`, so that form reaches it too. Every other DUT that creates its store runs on the in-memory store.
The parse suite (`test/parse`) has its own runner, which never sets `ze.test.storage` and refuses the option (`TestEveryParseCIFileParses`), so parse DUTs stay on disk and carry no option.
-> Decision: `ze config edit` opening the store by path is correct product behaviour (owner, 2026-10-17), so its tests run on disk rather than the editor learning the daemon's handle.

### Drives `ze config edit` in another process (session-editor, storage/explicit-editor-authority, cli-commit drivers): the editor opens the store by path (`cmdEditWithStorage`) before it edits over SSH, so it cannot see a memory store (40)

- plugin: cli-commit-reject, cli-commit-transactional, login-on-tree, session-editor-commit-completion, session-editor-commit-confirmed-abort, session-editor-commit-confirmed-accept-keeps-candidate, session-editor-commit-confirmed-accept, session-editor-commit-confirmed-boundary, session-editor-commit-confirmed-commit-now-refused, session-editor-commit-confirmed-disconnect, session-editor-commit-confirmed-nested, session-editor-commit-confirmed-other-user, session-editor-commit-confirmed-reconnect, session-editor-commit-confirmed-restart, session-editor-commit-confirmed-system-host, session-editor-commit-confirmed-timeout, session-editor-commit-force-conflict, session-editor-commit-force, session-editor-commit-grammar, session-editor-commit-verify, session-editor-copy-rename-existing, session-editor-copy, session-editor-deactivate-activate, session-editor-load-conflict, session-editor-load-merge, session-editor-load-refused, session-editor-load-replace, session-editor-load-terminal-relative
- reload: bgp-bfd-added-live, bgp-bfd-profile-edit-live, bgp-bfd-profile-timer-edit-live, bgp-filter-irr-loaded-over-ssh, bgp-route-refresh-enabled-over-ssh, bgp-rpki-added-live, bgp-rpki-attach-live, ipsec-added-live, ospf-added-live, request-reload-promotes-operator-file, traffic-usage-added-live, vrrp-added-live (ipsec, ospf and traffic-usage are darwin-skipped and carry it because they drive the same session-editor fixture)

### A fixture or a second process runs `ze init`, `ze data` or an offline `ze config` verb against the DUT's store (9)

- plugin: authz-recovery-admin, cli-credential-resolution, data-backup-live, history-repaired-store-start, init-from-blob, init-from-path, init-from-url
- ui: cli-data-config-dir, web-recovery-session-survives-commit

### Reads the store's files after the DUT (`expect=file:path=meta/...` or `expect=key:path=meta/...`) (5)

- plugin: api-config-commit-reject, audit-config-commit, concurrent-config-commit
- reload: commit-transactional, commit-verify-reject

### Starts ze twice in one work directory: the second start must find what the first recorded (3)

- plugin: api-peer-create-delete-rib, api-peer-save, iface-tunnel-restart

### A fixture owns the DUT and its store (storage, history, data, restart, zefs, rollback, repaired-store, archive and appliance fixtures) (23)

- appliance: serial-login
- plugin: bfd-auth-meticulous-persist, data-restore-config-live, drain-restart-kind, explicit-file-restart, firewall-irr-update, gr-marker-cold-start, history-rollback-object, mgmt-guard-api-env-started-settings-survive, policy-routes-show, resolve-rir-lookup, resolve-rir-refresh, start-auto-creates, statestore-on-tree, storage-history-restart, storage-rir-restart
- policy: policy-interface-list-counters
- static: static-rollback-keeps-committed
- traffic: traffic-reload-qdisc-tc
- ui: api-user-removed-by-reload, config-archive-prune, config-schema-stamp-recovers, web-commit-transactional

### The option dropped: nothing in the test reaches `createAbsentStore` (45)

The structural grep gave these the option and review found its reason does not hold, so the option was removed and each file is back at HEAD.
- Two `ze -` daemons, each in its own `daemon-N/` directory and each started once (17): ipsec-child-rekey, ipsec-clear-reestablish-peer, ipsec-clear-reestablish, ipsec-clear-sa, ipsec-cookie-challenge, ipsec-cookie-threshold-tolerates, ipsec-dpd-holds-tunnel, ipsec-dpd-timeout, ipsec-eap-md5-challenge, ipsec-eap-nak-unacceptable-type, ipsec-eap-tls-clienthello, ipsec-eap-tls13-ocsp-required, ipsec-eap-tls13-ocsp-stapling, ipsec-eap-tls13-resumption, ipsec-peer-show, ipsec-sa-installed, ipsec-sa-show
- One daemon, then an offline `ze config validate`, which opens no store (`internal/component/config/cli/cmd_validate.go`) (3): traffic-vpp-reject-hfsc, traffic-vpp-reject-mark, traffic-vpp-reject-prio
- No daemon at all: `ze completion`, `ze data registered`, `ze init`, `ze doctor`, an offline `ze config` verb, or a VM driven by `./le test qemu` (16): plugin data-registered-object, ze-init-piped; ui help-lists-every-local-command, cli-completion-keyword-shorthand, cli-config-dump-sensitive, cli-config-dump-strip-private, cli-zefs-blob-storage, cli-zefs-filesystem-override, config-fmt-write-stdout, config-history-stdin-refused, dash-stdio, dash-stdio-path-scope, doctor-without-store, doctor-store-refuses-legacy-blob; install qemu-full, qemu-iso
- `ze init` seeds the store first, and `ze start` then opens it (`openExplicitStore`) and never replaces it, or starts `--web-only` with no file (9): plugin mgmt-guard-web-dormant-insecure-warns, mgmt-guard-web-env-started-address-binds; ui test-show-system-sockets-keyword-filters, cli-argument-length-refused, cli-argument-refused-plugin-route, cli-transcript-redacts-credential, web-tool-decode, web-only-interrupt-force, web-only-interrupt-graceful

Rerun on the memory default (2026-10-17, darwin, private launcher): the 4 plugin and 18 ui tests above pass, and test-show-system-sockets-keyword-filters is darwin-skipped. The 17 ipsec tests, the 3 traffic tests and test-show-system-sockets-keyword-filters owe a targeted Linux rerun, and qemu-full and qemu-iso a VM run.

### Run on the memory store after all (14)

The structural grep first put these in the fixture group on the words "restart" or "rollback". Each starts ONE DUT, reloads it in process (a trigger fixture rewrites the file and sends SIGHUP, or the plugin restarts, or the transaction rolls back), reads no store file after it, and opens no store from another process, so the memory store holds everything the test reads. The darwin-runnable ones were rerun on memory one by one: every one passed except the two VPP tests (the race in the table below).
- plugin: api-commit-lifecycle, failure-policy-restart, prefix-count-installed-cross-family, iface-ensure-rollback (linux, not run here), iface-migrate (linux, not run here), vpp-loopback-reapply, vpp-loopback-reload-create
- reload: bgp-watchdog-pool-reload, config-apply-ordering-coarse-root (linux, not run here), config-apply-ordering-mixed-rollback (linux, not run here), geodns-reload-unbindable-listener-refused, reload-import-policy-no-bounce, tx-bgp-rollback, tx-protocol-rollback

### Reds of the plugin and reload suites that are not the store's

Full suites on the memory default (2026-10-17, private launcher): plugin 35 of 825 failed, reload 9 of 101. Every one of them was rerun after the disk options landed:

| Test | Result | Cause |
|------|--------|-------|
| the 30 plugin and 8 reload tests among the groups above that failed on memory (`no configuration store` from `ze config edit` or the fixture, or a store file missing after the DUT) | pass | needed the disk store |
| drain-budget-over-ceiling-refused | red in both full runs, pass in the reruns on memory | not the store: the fixture reports the expected refusal as `ZE-OBSERVER-FAIL`, so the verdict reads the relay order (plan/journal/assertion-reads-state-the-scheduler-owns.md, open) |
| vpp-loopback-reapply, vpp-loopback-reload-create | red 2 of 2 alone on memory ("VPP connector not ready" at the first apply, 55s), pass 2 of 2 on disk | a product race the memory store exposes, not a store defect: `applyConfig` Phase 1 (`internal/component/iface/config_apply.go`, the `cfg.Dummy` loop) fails the boot when `CreateDummy` answers `ErrBackendNotReady`, while the listing and Phase 3+4 defer on the same error and wait for `vppevents.EventConnected`. Nothing orders the interface plugin after the GoVPP handshake (its only dependency is `sysctl`), and the failing log shows the create 1 ms after the vpp component started connecting. The disk store's slower start hid it. The tests run on memory and stay red until the iface fix lands; that fix's session holds the journal row in plan/journal/transient-failure-treated-as-fatal.md and commits it with the fix |
| storage-rir-restart | red once in the parallel rerun ("cannot read config file"), pass alone | load, not the store |
| bgp-rib-maximum-paths-reload | red in all three reload suite runs and twice alone (`connection_refused`: the peer's 30 s dial retry never reaches the DUT's listener), once on memory and once with `option=storage:value=disk` | not the store, since it fails the same on disk; the build carried other sessions' uncommitted `internal/component/bgp/reactor` and `plugins/rib` hunks, and the cause is not traced (plan/journal/listener-refuses-the-peer-its-config-names.md, open) |
| rfc7999-blackhole-readvertise-agreement | red | the runner's pre-flight check, `loopback_address_missing` (no lo0 alias 127.0.0.6), before any store exists: environment |
| rpki-decorator-merge | red in the full run, pass in the rerun | known flake, plan/journal/gate-red-where-nothing-blocks-on-it.md |

Not run as whole suites on the memory default from here: the ui, parse, ipsec, firewall, traffic, policy, static, appliance and install suites (darwin), and every darwin-skipped test. Their disk options come from the structural grep, not from a red; the ui tests whose option was dropped were rerun one by one (above).

## End-to-End User Stories
An operator starts ze on a busy disk; SSH answers at once; a commit typed in the first second
waits for the record and lands.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBootRecordPendingReadAnswersBootBytes` | `internal/component/config/storage/boot_record_test.go` | AC-3 | |
| `TestBootRecordWriteWaitsThenLands` | same | AC-2, AC-5 | |
| `TestBootRecordWaitBound` | same | AC-2 bound | |
| `TestBootRecordFailureRefusesWrites` | same | AC-4 | |
| `TestStoreSelector*` | `cmd/ze` | AC-6 | |

### Boundary Tests (numeric inputs)
- Wait bound: write released at bound minus epsilon proceeds; past the bound refuses.

### Functional Tests
- Existing `test/plugin/explicit-file-restart.ci`, `drain-restart-kind.ci` keep disk (B).
The runner hands the selector to the DUT in two places, so each has its own test. `runOrchestrated` (runner_exec.go) serves every `.ci` with a program `cmd=` line; `runTest` serves a `.ci` with none (at most `cmd=api` peer lines), such as the `test/encode` population. runTest holds no `expect=stderr` arm, so its tests observe the store directory instead: a disk store lands in `database` beside the config (`StoreDir`), a memory store writes nothing.
- New `test/plugin/dut-store-memory-default.ci` (AC-6, runOrchestrated): a default DUT prints `created memory store (test build)`. Red 2026-10-17 with ONLY runOrchestrated's store env append removed by go -overlay (the await never held, the DUT printed `created live store`); the same build left dut-store-memory-config green. Green restored.
- New `test/plugin/dut-store-disk-option.ci` (AC-7, runOrchestrated): `option=storage:value=disk` makes the DUT print `created live store`, never the memory line.
- New `test/encode/dut-store-memory-config.ci` (AC-6, runTest): a cmd-less config-file DUT leaves no `database` in its work directory (`expect=file:glob=database:absent=true`). Red 2026-10-17 with ONLY runTest's store env append removed by go -overlay (`expected absent, matched 1`, the DUT printed `created live store`); the same build left dut-store-memory-default green. Green restored.
- New `test/encode/dut-store-disk-config.ci` (AC-7, runTest): with `option=storage:value=disk` the same DUT writes `database` (`exists=true`), which also proves the absence check looks where a disk store lands.

### Interop Tests (Scope: protocol)
Not protocol.

## Files to Modify
- cmd/ze/hub/main.go, cmd/ze/hub/config_source.go
- internal/component/config/storage/source.go, pointer.go
- cmd/ze/ze_core_start.go (B); internal/test/runner (B)
- internal/component/config/storage/tree.go, ownership.go (C)
- docs/architecture/storage-backends.md, docs/architecture/hub-architecture.md, docs/functional-tests.md

## Files to Create
- internal/component/config/storage/boot_record.go, boot_record_test.go
- cmd/ze/register_store_zetest.go (B)

### Integration Checklist
- [ ] gate consulted by every pointer.go entry point

### Documentation Update Checklist (BLOCKING)
- [ ] storage-backends.md "Version pointers and recovery": the boot record and its gate
- [ ] journal rows updated

## Implementation Steps
1. A: boot_record.go + tests; wire hub; docs; commit.
2. B: selector, runner default, per-test disk option, list; commit.
3. C: trims with sync-count test; commit.

### Critical Review Checklist
- [ ] early commit semantics, failure reporting, shutdown waits for the record

### Deliverables Checklist
- [ ] three commits

### Security Review Checklist
- [ ] memory selector unreachable in a shipped build

### Failure Routing
Red .ci after B: check whether it needs the disk option before touching code.

## Design Insights
Reads need not wait: the boot bytes are the accepted config before they are durable.

## Key Design Decisions
- Early writes WAIT (bounded 30s) rather than refuse at once: the record normally takes
  milliseconds, so an immediate refusal would make every early commit (and every test that
  commits right after ready) flaky; past the bound the operator gets a refusal naming the cause.
- The gate lives in the storage handle, not at each hub entry point: seven writers in four
  packages reach pointer.go, and only the handle is common to all of them.
- A failed record does not stop a running daemon: it keeps routing on the config it loaded,
  and refuses config changes with the record's error until restarted.

## Known Limitations
- Raw-key readers (`ze data cat`, backup) see the store as it is while the record is pending.
- The gate lives in the daemon's handle. Read-only readers in another process that open the
  store themselves (`withRuntimeStore` in `config/cli/config_data.go`, `cmd_history.go`,
  `cmd_diff.go`) read the previous active configuration, or none on a first start, until the
  record lands.
- The record starts right after `writeReadyFile` (moved there by 9ee692f99a: started before the
  plugin wait, its lock held bgp's Stage 2 GR-marker read behind slow syncs and failed the
  startup barrier). A config write of the bound name arriving earlier, such as a SIGHUP during
  startup, waits for it, at most 30s.

## RFC Documentation (Scope: protocol)
Not protocol.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] design decided by the owner 2026-10-17

### Goal Gates (MUST pass)
- [ ] stall measured before/after under a concurrent fsync writer
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] /ze-close

## Closure attempt 2026-10-20 (agent bootrec): stopped, not closed

Goal measurement, one 192MB `conv=fsync` dd writer looped on the checkout disk (ext4, /dev/sda2),
`./le --name bootrec test stress-repro run suite plugin test <t> parallel 1 burners 1 timeout 120 any-failure`,
host load average 31-41 throughout (shared machine). before = `ze` built from `git archive 37823314a5^`,
after = `ze` built from `git archive 5aff23f38e` (HEAD), both with the DUT tag set, pinned by `ZE_BIN`.

| Run | Writer | Result |
|-----|--------|--------|
| filter-irr, before | none | 2/2 pass (control) |
| filter-irr, after | none | 2/2 pass (control) |
| filter-irr, before | fsync | invocation 1 FAIL, timeout 21.4s, ze printed `created live store` and nothing after (the stall) |
| filter-irr, after | fsync | 8/8 pass, 29s for all eight (memory DUT store) |
| explicit-file-restart, before | fsync | 3 pass, invocation 4 FAIL 70.3s: ze printed `created live store` and the capability warnings, never `Starting ze`; the fixture killed it (the stall) |
| explicit-file-restart, after | fsync | invocation 1 FAIL 77.3s: ze printed `Starting ze`, `Ze running`, `storage probe authenticated`, then on stop `Stopping: finishing config changes...`, `Ze stopped.`, and was still killed by the fixture |
| explicit-file-restart, after | none | 1/1 PASS 17.6s (`./le job run` over `le test bgp plugin -v explicit-file-restart`, `LE_TEST_NO_BUILD=1`) |

-> Finding (open, blocks closure): the startup stall is gone at HEAD (the disk-store DUT now serves
under the writer), but explicit-file-restart, a disk-store test, still fails under the same writer at
HEAD, now in shutdown: after `Ze stopped.` the process does not exit before the fixture's kill. The
cause is not traced. Suspect: `defer bootRecord.Wait()` in runYANGConfig (cmd/ze/hub/main.go), which
this spec added, so a stop now waits for the record's syncs; or the store close's syncs. Next step: a
SIGQUIT goroutine dump of the stopping ze under the writer (the fixture's kill deadline in
`storage/tree`), then decide whether shutdown waits on the record with a bound or the fixture's
deadline is wrong.

Review of 37823314a5, dd3eaf7773, 4c1db99ae2 (read in full, plus 9ee692f99a which moved the record
start after `writeReadyFile`): no BLOCKER or ISSUE in the diffs themselves. Every pointer.go writer
(`WriteCandidateVersion`, `StageSourceCandidate`, `EnsureActiveVersion`, `ClearCandidate`,
`PromoteCandidate`, `RestoreConfig`) takes `store.AcquireLock(configPath)`, which the bound
`configStore.AcquireLock` gates; `promoteConfigCandidate` writes its intent only after a candidate
exists, so after the gate. NOTE fixed: Known Limitations said the record starts before the plugin
wait. Not recorded with `./le spec review record` because the closure stopped at the finding above.
