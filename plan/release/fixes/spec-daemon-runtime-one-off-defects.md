# Spec: Daemon runtime, plugin host and storage one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Umbrella retiring 24 journal rows (plus three un-journaled ACs) already fixed or superseded at HEAD and routing the 22 remaining open daemon-runtime defects into seven focused, single-door child specs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Set: this file is the umbrella of `spec-daemon-runtime-one-off-defects-<N>-<name>.md`, children 1 to 7, all in `plan/release/fixes/`. The children carry every remaining acceptance criterion, test, and code edit, each scoped to its own subsystem door (`ai/META-DECISIONS.md`, "How agents work"). This umbrella carries the 2026-11-26 producer audit at HEAD, the source-verified retirement of 24 already-fixed or superseded journal rows (plus AC-19, AC-20, and AC-21), and the routing of all 46 original journal rows with zero duplicate ownership; it edits no code itself.

## Task

**The original sweep (2026-10-23).** Forty-six open journal rows across core libraries, config parsing, the plugin host and SDK, hub startup and reload, external API surfaces, provisioning staging, and test mock tooling were grouped into this single spec.

**What producer re-verification at HEAD showed (2026-11-26).** Every acceptance criterion (AC-1 to AC-31) and all 46 owned journal rows were re-checked against HEAD source code and git history:

- **Twenty-four of the 46 journal rows (plus AC-19, AC-20, and AC-21) are already fixed or superseded at HEAD:** looking glass PKI TLS and rotation (`b243c9221b`, `fc01140865`, `cc0de04ced`), `answerValue`, `responseToDispatchOutput`, and `pluginAnswerRows`/`CompleteRecordAnswer` (`cdb4156509`), `TxCoordinator.collectRollbackAcks` timeout drain (`47dc081099`), arm64 `crashdump_test.go` (`9c5f23cf0b`), web TLS rotation asymmetry (superseded by always-TLS `WebServer` and unconditional `setLGTransport` in `b243c9221b`), `show system` `command-count` (`14308f0dfe`), `ServerEndpoint.Listen` IPv6 bracketing (`d794af6e2a`), `defaultHTTPGet` body-owned cancel (`8e3f2e8e80`), `(*Root).IssueLeaf` live renewal via `ServingLeaf` in both `NewHubAcceptor` and `managedCertificate` (`8788a29f8b`), `handshakeAt` typed errors (`7889671486`), per-config-name storage pointers (`ba67a8b89c`), hub reload exposure guard (`81447e1c4c`, `0a836dce95`), hub `--plugin` test contract (`8be1928d2b`), crashlog fd 2 deadlock and systemd early-SIGHUP ignore (`9c3ff48f8a`, `f71ef11f1c`), `env.normalize` hyphens (`c268fa58d4`), raw `BlobStore` single-process contract reconciled with live storage flock (`ba67a8b89c`), `ze signal restart` systemd `RestartForceExitStatus` (`b93ff713b3`), `VPPManager.Run` doc comment (`83b7c9c3c9`), `TestRegisterDoctorCheckAcceptsForeignNamespace` (`130c8c54d7`), `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` (`78f1c4bdd4`), `attach-process-reload.ci` and `config-edit-ssh.ci` (`eae2825926`, `7684b590f6`), doctor listener fallback (`238bdb62e9`, `f4a0df7781`), `set debug module` trailing-word refusal (`90adb048ca`), and `capture-raw start` refusal (`238bdb62e9`).
- **Twenty-two rows remain genuinely open across seven subsystem doors:** core logging, Loc-RIB metrics wiring, `textbuf` zero-value initialization, and `resolve.StoreDir` (Child 1), `TreeFromPluginMap` unordered multi-entry lists (Child 2), plugin dispatch error-path preservation, SDK recovered goroutines, plugin `Completable` Tab completion and dedicated completion RPC, and deletion of test-only `ipc.Authenticate` (Child 3), hub `ze.mcp.enabled` dormant settings under the 2026-11-26 "Ze always starts" guard, managed client `readLine` CR rule, SIGHUP queueing behind staged candidates and `-race` wakeup, and hub `ServesClient` data-RPC readiness (Child 4), MCP `tools/list` byte stability, readiness race, and seven `test/plugin` functional tests (Child 5), `provision` iPXE placeholder refusal (Child 6), and test mock `flag.ContinueOnError` crashlog flush (Child 7).

**Goal.** Split the 22 remaining open rows into seven narrow, single-door child specs (`spec-daemon-runtime-one-off-defects-1` through `-7`), retire the 24 already-landed rows with exact file, symbol, and commit evidence, and keep zero code edits in this umbrella.

## Children

| # | Spec | What it fixes | Doors it names | Status |
|---|------|---------------|----------------|--------|
| 1 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-1-core.md` | Reuses stored `LevelVar` in `slogutil.Logger`, canonicalizes hyphens in `slogutil.PluginLogger`, wires `locrib.SetMetricsRegistry` in `policy` `ConfigureMetrics`, lazily initializes zero-value `textbuf.Buffer` to its inline array, and resolves bare config names in `resolve.StoreDir` against `ze.config.dir` | core | design |
| 2 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-2-config-plugin-map.md` | Populates deterministic `listOrder` in `TreeFromPluginMap` for multi-entry keyed lists missing `@<list>` order metadata so `GetListOrdered` never silently loads zero peers | config-yang | design |
| 3 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-3-plugin-infra.md` | Preserves non-nil `*plugin.Response` beside `dispatchErr` in `dispatchCommandPeerResponse`, adds a recovered goroutine starter to `pkg/plugin/sdk` and guards remaining raw plugin goroutines, exposes `Completable` in `commandRows` and adds a dedicated `complete-command` callback RPC, and deletes test-only `ipc.Authenticate` and `authenticateWithName` | plugin-infra | design |
| 4 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-4-hub-runtime.md` | Honors dormant `mcp` block settings under `ze.mcp.enabled` gated by per-listener exposure checks without exiting `ze start`, stops stripping `\r` in `managed` `readLine`, queues SIGHUP behind `storage.ErrCandidateExists` and fixes the `-race` wakeup, and gates managed `ServesClient` data RPC readiness at boot | hub-engine | design |
| 5 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-5-api-surfaces.md` | Fixes MCP `tools/list` byte stability, wires an explicit MCP readiness signal for `mcp-header-mismatch-rejected.ci`, and repairs the seven `test/plugin` functional tests (keeping LG `.ci` tests in `test/plugin/`) | api-surfaces | design |
| 6 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-6-provision-ipxe.md` | Refuses to stage the 28-byte ASCII iPXE placeholders in `internal/plugins/provision/staging.go` with an actionable `opererr.Error` naming the placeholder file and fetch instructions | system-plugins | design |
| 7 | `plan/release/fixes/spec-daemon-runtime-one-off-defects-7-mock-help-flush.md` | Switches the seven remaining mock servers under `internal/test/mock/` and `internal/test/cli/` from `flag.ExitOnError` to `flag.ContinueOnError` so `--help` flushes through `crashlog.Flush` | le-tooling | design |

Every child also names `ai/doors/system.md`. No child edits a file another child edits; they can be implemented in any order.

## Row routing

All 46 original rows of `## Owned journal rows` re-verified against HEAD on 2026-11-26:

| # | Class file | Date | Surface (short) | Outcome | Journal marker today |
|---|------------|------|-----------------|---------|----------------------|
| 1 | `second-logger-detaches-runtime-level` | 2026-10-28 | panic guards calling `slogutil.Logger` twice | Child 1 | open |
| 2 | `second-logger-detaches-runtime-level` | 2026-11-02 | forked BGP plugin logger ignores dotted `ze.log.<subsystem>` key | Child 1 | open |
| 3 | `zero-value-as-valid-answer` | 2026-10-30 | `TreeFromPluginMap` unordered multi-entry list loads zero peers | Child 2 | open |
| 4 | `unwired-feature` | 2026-10-30 | `locrib.SetMetricsRegistry` never called | Child 1 | open |
| 5 | `deferral-sized-from-the-visible-half` | 2026-09-04 | `spec-pki-full-chain` looking glass TLS | Retired (already fixed at HEAD: `b243c9221b`, `fc01140865`, `cc0de04ced`) | open |
| 6 | `enabled-gate-discards-settings` | 2026-08-10 | MCP `ze.mcp.enabled` | Child 4 | open |
| 7 | `error-path-discards-data-already-received` | 2026-09-05 | `answerValue` | Retired (already fixed at HEAD: `cdb4156509`) | open |
| 8 | `error-path-discards-data-already-received` | 2026-09-05 | `responseToDispatchOutput` | Retired (already fixed at HEAD: `cdb4156509`) | open |
| 9 | `error-path-discards-data-already-received` | 2026-09-08 | `(*Server).dispatchCommandResponse` | Child 3 | open |
| 10 | `error-path-discards-data-already-received` | 2026-10-15 | `TxCoordinator.collectRollbackAcks` | Retired (already fixed at HEAD: `47dc081099`) | open |
| 11 | `fast-path-type-used-without-its-initializer` | 2026-09-02 | `internal/core/textbuf` | Child 1 | open |
| 12 | `fixture-encodes-an-impossible-state` | 2026-09-08 | `crashdump_test.go` | Retired (already fixed at HEAD: `9c5f23cf0b`) | open |
| 13 | `guard-added-to-one-half-of-a-pair` | 2026-09-04 | hub TLS rotation, web listener | Retired (superseded at HEAD: always-TLS `WebServer`, unconditional `setLGTransport` in `b243c9221b`) | open |
| 14 | `guard-added-to-one-half-of-a-pair` | 2026-08-22 | fleet hub auth line reader | Child 4 | open |
| 15 | `invariant-enforced-by-an-absent-call-site` | 2026-08-18 | `show system` command count | Retired (already fixed at HEAD: `14308f0dfe`) | open |
| 16 | `ipv6-address-built-by-concatenation` | 2026-08-10 | `ServerEndpoint.Listen` | Retired (already fixed at HEAD: `d794af6e2a`) | open |
| 17 | `lifetime-ends-before-its-last-reader` | 2026-10-11 | `defaultHTTPGet` | Retired (already fixed at HEAD: `8e3f2e8e80`) | fixed |
| 18 | `material-expires-with-no-renewal-path` | 2026-09-03 | `(*Root).IssueLeaf` | Retired (already fixed at HEAD: `8788a29f8b`) | open |
| 19 | `nil-nil-return` | 2026-09-08 | `handshakeAt` | Retired (already fixed at HEAD: `7889671486`) | open |
| 20 | `output-lost-to-an-exit-past-the-flush` | 2026-09-06 | `ze-test <mock>` help | Child 7 | open |
| 21 | `output-not-byte-stable` | 2026-08-10 | mcp tools list | Child 5 | open |
| 22 | `placeholder-shipped-as-the-real-artifact` | 2026-08-28 | `tools/ipxe-binaries/ipxe.efi` | Child 6 | open |
| 23 | `pointer-shared-across-the-names-it-indexes` | 2026-09-02 | storage active pointer | Retired (already fixed at HEAD: `ba67a8b89c`) | open |
| 24 | `queued-request-waits-for-an-unrelated-trigger` | 2026-09-26 | `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` | Child 4 | open |
| 25 | `silent-fall-through` | 2026-08-12 | hub reload guard, functional | Retired (already fixed at HEAD: `81447e1c4c`, `0a836dce95`) | open |
| 26 | `silent-fall-through` | 2026-08-12 | hub `--plugin` | Retired (already fixed at HEAD: `8be1928d2b`) | open |
| 27 | `silent-fall-through` | 2026-09-24 | early SIGHUP, crashlog | Retired (already fixed at HEAD: `9c3ff48f8a`, `f71ef11f1c`) | open |
| 28 | `silent-fall-through` | 2026-09-24 | env `normalize` | Retired (already fixed at HEAD: `c268fa58d4`) | fixed |
| 29 | `silent-fall-through` | 2026-10-19 | `handleSIGHUPReload` | Child 4 | open |
| 30 | `silent-fall-through` | 2026-10-21 | `pluginAnswerRows` | Retired (already fixed at HEAD: `cdb4156509`) | open |
| 31 | `store-serializes-in-process-only` | 2026-09-03 | `BlobStore` | Retired (reconciled at HEAD: `ba67a8b89c`, `docs/architecture/zefs-format.md`) | open |
| 32 | `unguarded-goroutine-kills-the-process` | 2026-09-02 | plugin goroutines | Child 3 | open |
| 33 | `claim-outlives-the-evidence-it-cites` | 2026-10-13 | `ze signal restart` promised to restart; unit has `Restart=on-failure` | Retired (already fixed at HEAD: `b93ff713b3`) | open |
| 34 | `comment-describes-superseded-behaviour` | 2026-10-15 | `VPPManager.Run` says SIGTERM; `exec.CommandContext` sends SIGKILL | Retired (already fixed at HEAD: `83b7c9c3c9`) | open |
| 35 | `gate-red-where-nothing-blocks-on-it` | 2026-09-14 | `TestRegisterDoctorCheckAcceptsForeignNamespace` red with full tags | Retired (already fixed at HEAD: `130c8c54d7`) | open |
| 36 | `test-asserts-superseded-behaviour` | 2026-11-11 | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` wants "unknown backend", gets the compiled-out refusal | Retired (already fixed at HEAD: `78f1c4bdd4`) | open |
| 37 | `test-against-broken-path` | 2026-08-22 | `test/plugin/attach-process-reload.ci` | Retired (already fixed at HEAD: `eae2825926`, `7684b590f6`) | fixed |
| 38 | `test-against-broken-path` | 2026-08-23 | seven `test/plugin` tests red | Child 5 | open |
| 39 | `test-against-broken-path` | 2026-08-23 | `config-edit-ssh.ci` observer shutdown decode | Retired (already fixed at HEAD: `eae2825926`, `7684b590f6`) | fixed |
| 40 | `unwired-feature` | 2026-08-15 | mcp `tools/list` first line fails | Child 5 | open |
| 41 | `unwired-feature` | 2026-08-15 | mcp `tools/list` readiness, rerun from `git archive HEAD` | Child 5 | open |
| 42 | `unwired-feature` | 2026-11-11 | plugin `Completable` commands get no Tab completion | Child 3 | open |
| 43 | `field-carries-two-meanings` | 2026-11-15 | plugin `Completable` completion request is an `ExecuteCommand` | Child 3 | open |
| 44 | `production-function-only-tests-call` | 2026-11-11 | `ipc.Authenticate`, `authenticateWithName` test-only, still accept an empty expected token | Child 3 | open |
| 45 | `session-up-before-its-subscriber` | 2026-11-19 | hub start publishes the data RPC target before `ServesClient` | Child 4 | open |
| 46 | `supplied-path-outranks-the-configured-store` | 2026-11-19 | `resolve.StoreDir` resolves a bare config name to the working directory and ignores `ze.config.dir` | Child 1 | open |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, public `pkg/` API, registration over enumeration
- [ ] The subsystem door each child names in `## Children` (`ai/META-DECISIONS.md`, "How agents work")
- [ ] `docs/architecture/core-design.md` - `## Design decisions` (2026-11-26 "Ze always starts")
- [ ] `plan/journal/README.md` - owned journal rows and closure rules

## Current Behavior (MANDATORY)

Verified at the producing source files on 2026-11-26. This umbrella edits no code itself; each child carries the full producer reading for its own files. Every retired row below was confirmed landed or superseded at HEAD:

**Source files verified for retirements:**
- [ ] `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/register_lg.go`, `internal/component/web/server.go` - LG consumes `pki.ServerTLSMaterial` with per-entry TLS and unconditional `setLGTransport` (`b243c9221b`, `fc01140865`, `cc0de04ced`); `NewWebServer` always requires TLS (`--insecure-web` disables auth only).
- [ ] `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/server/command.go`, `internal/component/plugin/record_answer.go` - `answerValue`, `responseToDispatchOutput`, and `pluginAnswerRows`/`CompleteRecordAnswer` all preserve records, status, error, and warnings (`cdb4156509`).
- [ ] `internal/component/config/transaction/orchestrator.go` - `collectRollbackAcks` calls `drainArrivedRollbackAcks` on deadline timeout (`47dc081099`).
- [ ] `internal/component/config/system/crashdump_test.go` - `treeFrom` parses via `ParseTreeWithYANG` so arm64 does not hit the amd64 kexec validator (`9c5f23cf0b`).
- [ ] `internal/component/plugin/server/command_registry.go`, `internal/component/cmd/show/system.go`, `internal/component/plugin/server/system.go` - `CommandCountsByProcess` supplies real `command-count` values (`14308f0dfe`).
- [ ] `internal/component/config/loader_extract.go` - `ServerEndpoint.Listen` uses `tb.HostPort` (`d794af6e2a`).
- [ ] `internal/appliance/cache.go` - `defaultHTTPGet` transfers cancel to `downloadBody.Close` (`8e3f2e8e80`).
- [ ] `internal/component/plugin/acceptor.go`, `internal/component/plugin/server/managed_serve.go`, `internal/component/plugin/leaf_test.go` - both `NewHubAcceptor` and `managedCertificate` serve via `plugin.NewServingLeaf` (`8788a29f8b`); `handshakeAt` returns typed errors (`7889671486`).
- [ ] `internal/component/config/storage/pointer.go`, `pkg/zefs/store_test.go`, `docs/architecture/zefs-format.md` - `pointerPath` keys pointers per config name (`ba67a8b89c`); single-process raw `BlobStore` contract is reconciled with live storage flock (`ownership.go`, `open.go`).
- [ ] `cmd/ze/hub/listener_migrate.go`, `cmd/ze/hub/main_test.go`, `cmd/ze/hub/build_tag_vpp_absent_test.go` - `checkReloadExposure` / `checkServiceExposure` refuses unauthenticated non-loopback reload (`81447e1c4c`, `0a836dce95`); `TestRunPassesCLIPluginsToConfigLoad` pins the `--plugin` merge contract (`8be1928d2b`); `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` asserts the compiled-out refusal (`78f1c4bdd4`).
- [ ] `internal/core/crashlog/`, `internal/plugins/systemd/unit.go`, `internal/core/env/env.go`, `internal/core/diagnostic/doctor_registry.go` - crashlog leaves fd 2 real (`9c3ff48f8a`), systemd unit wraps `ExecStart` with `trap "" HUP` (`f71ef11f1c`) and sets `RestartForceExitStatus` (`b93ff713b3`), `env.normalize` maps hyphens (`c268fa58d4`), and `validateDoctorCheckCodesReg` accepts foreign-namespace codes (`130c8c54d7`).
- [ ] `internal/component/vpp/vpp.go`, `internal/component/doctor/checks_listener.go`, `internal/plugins/debug/debug.go`, `internal/plugins/diag/cmd/capture_raw.go` - `VPPManager.Run` doc matches SIGKILL wait (`83b7c9c3c9`), doctor listener fallback reports `doctor-listener-set-unknown` (`238bdb62e9`, `f4a0df7781`), `setModuleSetting` calls `refuseTrailingWords` (`90adb048ca`), and `captureRawStartRefusal` reports unstarted protocol capture (`238bdb62e9`).

**Behavior to preserve:**
- Every retired mechanism above and every child spec's preserved contracts.

**Behavior to change:**
- Only what Children 1 to 7 specify.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Looking glass PKI TLS and web TLS rotation | `cmd/ze/hub/service_lg.go` `startLGService`; `cmd/ze/hub/register_lg.go`; `internal/component/web/server.go` `NewWebServer` | LG consumes `pki.ServerTLSMaterial` with per-entry TLS and unconditional `setLGTransport` (`b243c9221b`, `fc01140865`, `cc0de04ced`); `WebServer` always requires TLS (`--insecure-web` disables auth only) | Retired (already fixed / superseded at HEAD): original AC-1, AC-7, and rows 2026-09-04 are satisfied |
| Record answer preservation across SDK, dispatch output, and streaming | `pkg/plugin/sdk/sdk_engine.go` `answerValue`; `internal/component/plugin/server/dispatch.go` `responseToDispatchOutput`; `internal/component/plugin/server/command.go` `pluginAnswerRows`; `internal/component/plugin/record_answer.go` `CompleteRecordAnswer` | Preserves records, status, error, and warnings across completed record answers (`cdb4156509`) | Retired for `answerValue`, `responseToDispatchOutput`, and `pluginAnswerRows` (original AC-3a/3b, AC-25); Child 3 repairs the remaining `dispatchCommandPeerResponse` discard (original AC-3c) |
| Config transaction rollback ack drain | `internal/component/config/transaction/orchestrator.go` `collectRollbackAcks`, `drainArrivedRollbackAcks` | Drains arrived rollback acks on deadline timeout without restarting on shutdown (`47dc081099`) | Retired (already fixed at HEAD): original AC-4 and row 2026-10-15 are satisfied |
| Crash-dump tree extraction test | `internal/component/config/system/crashdump_test.go` `treeFrom` | Uses `ParseTreeWithYANG` so arm64 does not trip the amd64 kexec validator (`9c5f23cf0b`) | Retired (already fixed at HEAD): original AC-6 and row 2026-09-08 are satisfied |
| Plugin command count inventory | `internal/component/plugin/server/command_registry.go` `CommandCountsByProcess`; `internal/component/cmd/show/system.go`; `internal/component/plugin/server/system.go` | Reports actual registered command counts per plugin (`14308f0dfe`) | Retired (already fixed at HEAD): original AC-9 and row 2026-08-18 are satisfied |
| IPv6 endpoint formatting | `internal/component/config/loader_extract.go` `ServerEndpoint.Listen` | Uses `tb.HostPort` to bracket IPv6 hosts (`d794af6e2a`) | Retired (already fixed at HEAD): original AC-10 and row 2026-08-10 are satisfied |
| Appliance download context lifetime | `internal/appliance/cache.go` `defaultHTTPGet`, `downloadBody.Close` | Transfers request cancellation to response body close (`8e3f2e8e80`) | Retired (already fixed at HEAD): original AC-11 and row 2026-10-11 are satisfied |
| Local CA leaf live renewal and handshake test helper | `internal/component/plugin/leaf.go` `NewServingLeaf`; `acceptor.go` `NewHubAcceptor`; `server/managed_serve.go` `managedCertificate`; `leaf_test.go` `handshakeAt` | Both hub acceptor and managed server serve through `ServingLeaf` via `GetCertificate` (`8788a29f8b`); `handshakeAt` returns typed errors (`7889671486`) | Retired (already fixed at HEAD): original AC-12, AC-13, and rows 2026-09-03 / 2026-09-08 are satisfied |
| Storage per-config pointers and single-process blob contract | `internal/component/config/storage/pointer.go` `pointerPath`; `open.go`; `ownership.go`; `docs/architecture/zefs-format.md` "Single-process ownership" | Keys active/candidate/rollback/recovery pointers per config name and locks live storage with `unix.Flock` while raw `pkg/zefs` stays single-process (`ba67a8b89c`) | Retired (already fixed / reconciled at HEAD): original AC-17, AC-26, and rows 2026-09-02 / 2026-09-03 are satisfied |
| Hub reload exposure guard, `--plugin` merge, and VPP build-tag test | `cmd/ze/hub/listener_migrate.go` `checkReloadExposure`, `checkServiceExposure`; `cmd/ze/hub/main_test.go` `TestRunPassesCLIPluginsToConfigLoad`; `cmd/ze/hub/build_tag_vpp_absent_test.go` `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` | Refuses unauthenticated non-loopback reload (`81447e1c4c`, `0a836dce95`); verifies `--plugin` reaches `MergeCliPlugins` (`8be1928d2b`); verifies compiled-out VPP refusal (`78f1c4bdd4`) | Retired (already fixed at HEAD): original AC-22 and rows 2026-08-12 / 2026-11-11 are satisfied |
| Crashlog fd 2 safety, systemd SIGHUP and restart unit, env normalization, doctor foreign codes | `internal/core/crashlog/`; `internal/plugins/systemd/unit.go`; `internal/core/env/env.go` `normalize`; `internal/core/diagnostic/doctor_registry.go` `validateDoctorCheckCodesReg` | Crashlog uses `debug.SetCrashOutput` (`9c3ff48f8a`), systemd ignores early SIGHUP (`f71ef11f1c`) and sets `RestartForceExitStatus` (`b93ff713b3`), `env.normalize` maps hyphens (`c268fa58d4`), and foreign doctor codes are accepted (`130c8c54d7`) | Retired (already fixed at HEAD): original AC-23, AC-24, AC-28, AC-30, and rows 2026-09-14 / 2026-09-24 / 2026-10-13 are satisfied |
| VPP doc comment, doctor listener refusal, debug trailing words, capture-raw refusal, migrated `.ci` fixtures | `internal/component/vpp/vpp.go`; `internal/component/doctor/checks_listener.go`; `internal/plugins/debug/debug.go` `refuseTrailingWords`; `internal/plugins/diag/cmd/capture_raw.go` `captureRawStartRefusal`; `test/plugin/attach-process-reload.ci`; `test/plugin/config-edit-ssh.ci` | Doc updated (`83b7c9c3c9`), listener fallback reports `doctor-listener-set-unknown` (`238bdb62e9`, `f4a0df7781`), trailing debug words refused (`90adb048ca`), unstarted capture refused (`238bdb62e9`), retired `ze_api.py` `.ci` fixtures migrated (`eae2825926`, `7684b590f6`) | Retired (already fixed at HEAD): original AC-19, AC-20, AC-21, AC-29, and rows 2026-08-22 / 2026-08-23 / 2026-10-15 are satisfied |
| Child 1-7 mechanisms | Each child's `## Existing Mechanisms` table | `slogutil`, `locrib`, `textbuf`, `resolve`, `plugin_map`, `plugin/server`, `pkg/plugin/sdk`, `plugin/ipc`, `cmd/ze/hub`, `managed`, `mcp`, `provision`, `internal/test/mock` | Extended in Children 1 to 7 |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Before you write code in a subsystem | the door every change reads | "Open that subsystem's door (`ai/doors/README.md`). It names the pages, the digest, the pattern and the gates for that area." | agrees | Umbrella edits no code and splits the 22 remaining open rows into seven narrow child specs, each scoped to its own subsystem door with zero file overlap. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- None of its own. Each child names the entry point of its fix (`slogutil`/`locrib`/`textbuf`/`resolve` in Child 1, `TreeFromPluginMap` in Child 2, `plugin/server`/`sdk`/`ipc` in Child 3, `cmd/ze/hub`/`managed` in Child 4, `mcp`/`test/plugin` in Child 5, `provision` in Child 6, and `internal/test/mock`/`cli` in Child 7).

### Transformation Path
1. Producer audit separates already-fixed rows (retired into `## Existing Mechanisms` and `## Rows found already fixed`) from genuinely open defects.
2. Each genuinely open defect is routed to the single child spec covering its subsystem door.

### Boundaries Crossed
- None in this umbrella; each child documents the exact boundaries its change crosses.

### Integration Points
- Seven child specs under `plan/release/fixes/` and the `plan/journal/` closure records.

### Changed-Wire Census

No wire format, RPC payload, event schema, or external protocol changes in this umbrella; it edits no code and holds only the producer audit, retirements, and child routing. Wire-touching children (Children 3, 4, and 5) carry their own Changed-Wire Census tables.

### Architectural Verification
- [ ] Every one of the 46 original journal rows has exactly one owner (retired in this umbrella or owned by exactly one child spec)
- [ ] No child spec edits a file another child spec edits

## Risks & Assumptions

### Assumptions
- All 24 retired rows were verified directly against HEAD source code and git history on 2026-11-26.

### Risks
- Retired rows whose `plan/journal/` marker still says `open:` remain in this umbrella's `## Owned journal rows` table until closure flips those markers to `fixed:`, preventing unowned open journal rows.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Umbrella audit and split | → | Children 1 to 7 | Each child's own Wiring Test table plus `./le spec design check` on umbrella and all seven children |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Children 1 to 7 (`spec-daemon-runtime-one-off-defects-1` through `-7`) complete implementation and verification | All seven child specs satisfy their own acceptance criteria, pass their Review Gate, and close cleanly in `plan/release/fixes/` | None; every remaining open defect routed from this umbrella must close through its owning child spec before umbrella closure |
| AC-2 | Umbrella closure audits the 20 retired rows still marked `open:` in `plan/journal/` | Every one of the 20 retired journal rows is updated from `open:` to `fixed:` citing the exact HEAD commit hashes and symbols recorded in `## Rows found already fixed` | The 4 retired rows already marked `fixed:` at HEAD (`defaultHTTPGet`, `env.normalize`, `attach-process-reload.ci`, `config-edit-ssh.ci`) stay untouched because their journal markers are already closed |
| AC-3 | Ownership audit across this umbrella and Children 1 to 7 (`## Owned journal rows` and `## Acceptance Criteria`) | Every one of the 46 original journal rows and every remaining acceptance criterion maps to exactly one owner across the umbrella (20 retired `open:` rows) and Children 1 to 7 (22 open rows) with disjoint file edit sets | None; duplicate ownership or overlapping file edit sets across child specs are forbidden |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| Defined per child spec (Children 1 to 7) | Per child | AC-1 |

### Functional Tests

| Test | AC |
|------|----|
| Defined per child spec (Children 1, 3, 4, 5, 6, 7) | AC-1 |

## Files to Modify
- None in this umbrella (documentation/journal closure updates at spec close only; all code edits live in Children 1 to 7).

## Files to Create
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-1-core.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-2-config-plugin-map.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-3-plugin-infra.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-4-hub-runtime.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-5-api-surfaces.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-6-provision-ipxe.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-7-mock-help-flush.md`

### Integration Checklist
- [ ] All seven child specs implemented and closed in any order

### Documentation Update Checklist (BLOCKING)
- [ ] Each child spec updates the design pages its `## Design documents` table names

## Design presentation

### What changes
- Retires 24 of the 46 owned journal rows (plus un-journaled AC-19, AC-20, and AC-21) verified already fixed or superseded at HEAD, and splits the 22 genuinely open rows into seven narrow, single-door child specs (`spec-daemon-runtime-one-off-defects-1` through `-7`). This umbrella edits no code itself.

### Before and after
- **Before:** One monolithic spec spanned twelve subsystem doors, mixed 24 already-landed fixes with 22 genuinely open defects, and carried stale claims (for example, `ServerEndpoint.Listen` bare colon and `env.normalize` hyphens) that were already fixed months earlier.
- **After:** The umbrella records the 2026-11-26 producer audit, exact commit-hash retirements, and row routing; seven focused single-door child specs each own a disjoint file set and disjoint journal rows.

### Alternatives set aside
- **Keeping all 22 remaining fixes in one multi-door spec:** Rejected under `ai/META-DECISIONS.md` ("a spec whose edits need many doors is split into narrower child specs") and the precedent of `plan/release/fixes/spec-single-owner-reload-reconcile.md`.
- **Deleting the 20 retired-but-still-`open:` rows from `## Owned journal rows` without closing their `plan/journal/` entries:** Rejected because `plan/journal/` rows marked `open:` must remain owned by a spec until closure flips them to `fixed:`.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Implement and close Children 1 to 7 in any order (zero file overlap).
2. On umbrella closure, flip the 20 retired rows below from `open:` to `fixed:` in `plan/journal/` citing the HEAD commits and symbols recorded in `## Rows found already fixed`.

## Owned journal rows

Held here solely until umbrella closure flips their `plan/journal/` markers from `open:` to `fixed:` (all 20 are already verified fixed or superseded at HEAD; the 22 genuinely open rows are owned exclusively by Children 1 to 7):

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `deferral-sized-from-the-visible-half` | 2026-09-04 | `spec-pki-full-chain` looking glass TLS |
| `error-path-discards-data-already-received` | 2026-09-05 | `answerValue` |
| `error-path-discards-data-already-received` | 2026-09-05 | `responseToDispatchOutput` |
| `error-path-discards-data-already-received` | 2026-10-15 | `TxCoordinator.collectRollbackAcks` |
| `fixture-encodes-an-impossible-state` | 2026-09-08 | `crashdump_test.go` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | hub TLS rotation, web listener |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-18 | `show system` command count |
| `ipv6-address-built-by-concatenation` | 2026-08-10 | `ServerEndpoint.Listen` |
| `material-expires-with-no-renewal-path` | 2026-09-03 | `(*Root).IssueLeaf` |
| `nil-nil-return` | 2026-09-08 | `handshakeAt` |
| `pointer-shared-across-the-names-it-indexes` | 2026-09-02 | storage active pointer |
| `silent-fall-through` | 2026-08-12 | hub reload guard, functional |
| `silent-fall-through` | 2026-08-12 | hub `--plugin` |
| `silent-fall-through` | 2026-09-24 | early SIGHUP, crashlog |
| `silent-fall-through` | 2026-10-21 | `pluginAnswerRows` |
| `store-serializes-in-process-only` | 2026-09-03 | `BlobStore` |
| `claim-outlives-the-evidence-it-cites` | 2026-10-13 | `ze signal restart` promised to restart; unit has `Restart=on-failure` |
| `comment-describes-superseded-behaviour` | 2026-10-15 | `VPPManager.Run` says SIGTERM; `exec.CommandContext` sends SIGKILL |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-14 | `TestRegisterDoctorCheckAcceptsForeignNamespace` red with full tags |
| `test-asserts-superseded-behaviour` | 2026-11-11 | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` wants "unknown backend", gets the compiled-out refusal |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `env-key-read-but-never-registered` | 2026-09-07 | `resolve.go` reading `ze.storage.blob` | No `ze.storage.blob` read remains under `internal/core/resolve/` |
| `deferral-sized-from-the-visible-half` | 2026-09-04 | `spec-pki-full-chain` looking glass TLS | `b243c9221b`, `fc01140865`, `cc0de04ced`: `internal/component/lg/server.go`, `cmd/ze/hub/service_lg.go` `startLGService`, `register_lg.go` |
| `error-path-discards-data-already-received` | 2026-09-05 | `answerValue` | `cdb4156509`: `pkg/plugin/sdk/sdk_engine.go` `answerValue` preserves `Data`, `Status`, `Warnings`, and `RecordAnswer` alongside `answer.Failure()` |
| `error-path-discards-data-already-received` | 2026-09-05 | `responseToDispatchOutput` | `cdb4156509`: `internal/component/plugin/server/dispatch.go` `responseToDispatchOutput` preserves completed record data beside error |
| `error-path-discards-data-already-received` | 2026-10-15 | `TxCoordinator.collectRollbackAcks` | `47dc081099`: `internal/component/config/transaction/orchestrator.go` `drainArrivedRollbackAcks` drains queued acks on timeout |
| `fixture-encodes-an-impossible-state` | 2026-09-08 | `crashdump_test.go` | `9c5f23cf0b`: `internal/component/config/system/crashdump_test.go` `treeFrom` uses `config.ParseTreeWithYANG` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | hub TLS rotation, web listener | `b243c9221b`: `WebServer` (`internal/component/web/server.go`) always serves TLS (`--insecure-web` disables auth only) and `register_lg.go` installs `setLGTransport` unconditionally |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-18 | `show system` command count | `14308f0dfe`: `internal/component/plugin/server/command_registry.go` `CommandCountsByProcess`, `cmd/show/system.go`, `server/system.go` |
| `ipv6-address-built-by-concatenation` | 2026-08-10 | `ServerEndpoint.Listen` | `d794af6e2a`: `internal/component/config/loader_extract.go` `ServerEndpoint.Listen` uses `tb.HostPort` |
| `lifetime-ends-before-its-last-reader` | 2026-10-11 | `defaultHTTPGet` | `8e3f2e8e80`: `internal/appliance/cache.go` `downloadBody.Close` owns cancellation (journal already `fixed:`) |
| `material-expires-with-no-renewal-path` | 2026-09-03 | `(*Root).IssueLeaf` | `8788a29f8b`: both `NewHubAcceptor` (`acceptor.go`) and `managedCertificate` (`server/managed_serve.go`) renew via `plugin.NewServingLeaf` |
| `nil-nil-return` | 2026-09-08 | `handshakeAt` | `7889671486`: `internal/component/plugin/leaf_test.go` `handshakeAt` returns `errNotTLS` / `errNoPeerCertificate` |
| `pointer-shared-across-the-names-it-indexes` | 2026-09-02 | storage active pointer | `ba67a8b89c`: `internal/component/config/storage/pointer.go` `pointerPath` uses `zefs.KeyConfigActive.Key(name)` |
| `silent-fall-through` | 2026-08-12 | hub reload guard, functional | `81447e1c4c`, `0a836dce95`: `cmd/ze/hub/listener_migrate.go` `checkReloadExposure` / `checkServiceExposure` |
| `silent-fall-through` | 2026-08-12 | hub `--plugin` | `8be1928d2b`: `cmd/ze/hub/main_test.go` `TestRunPassesCLIPluginsToConfigLoad` replaced deleted-guard test |
| `silent-fall-through` | 2026-09-24 | early SIGHUP, crashlog | `9c3ff48f8a` (`debug.SetCrashOutput`, `TestSIGQUITDumpDrainsAfterInit`) and `f71ef11f1c` (`systemd/unit.go` `trap "" HUP`, `TestUnitFileStartsZeWithSIGHUPIgnored`) |
| `silent-fall-through` | 2026-09-24 | env `normalize` | `c268fa58d4`: `internal/core/env/env.go` `normalize` maps `.`, `-`, `_` (journal already `fixed:`) |
| `silent-fall-through` | 2026-10-21 | `pluginAnswerRows` | `cdb4156509`: `internal/component/plugin/server/command.go` `pluginAnswerRows` + `record_answer.go` `CompleteRecordAnswer` |
| `store-serializes-in-process-only` | 2026-09-03 | `BlobStore` | `ba67a8b89c`: reconciled in `docs/architecture/zefs-format.md` ("Single-process ownership"); live storage holds `unix.Flock` in `internal/component/config/storage/{open,ownership}.go` |
| `claim-outlives-the-evidence-it-cites` | 2026-10-13 | `ze signal restart` systemd unit | `b93ff713b3`: `internal/plugins/systemd/unit.go` sets `SuccessExitStatus` and `RestartForceExitStatus` to `rpc.DaemonRestartExitStatus` |
| `comment-describes-superseded-behaviour` | 2026-10-15 | `VPPManager.Run` doc comment | `83b7c9c3c9`: `internal/component/vpp/vpp.go` doc comment updated to match SIGKILL + wait + NIC unbind |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-14 | `TestRegisterDoctorCheckAcceptsForeignNamespace` | `130c8c54d7`: `internal/core/diagnostic/doctor_registry.go` `validateDoctorCheckCodesReg` removed the `doctor-` prefix restriction |
| `test-asserts-superseded-behaviour` | 2026-11-11 | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` | `78f1c4bdd4`: `cmd/ze/hub/build_tag_vpp_absent_test.go` asserts the compiled-out refusal |
| `test-against-broken-path` | 2026-08-22 | `test/plugin/attach-process-reload.ci` | `eae2825926`, `7684b590f6`: migrated off deleted `ze_api.py` (journal already `fixed:`) |
| `test-against-broken-path` | 2026-08-23 | `config-edit-ssh.ci` observer shutdown decode | `eae2825926`, `7684b590f6`: migrated off deleted `ze_api.py` (journal already `fixed:`) |
| (un-journaled AC-19) | 2026-10-24 | doctor listener fallback | `238bdb62e9`, `f4a0df7781`: `doctor-listener-set-unknown` in `internal/component/doctor/checks_listener.go` |
| (un-journaled AC-20) | 2026-10-23 | `set debug module ... flag x` | `90adb048ca`: `internal/plugins/debug/debug.go` `refuseTrailingWords` |
| (un-journaled AC-21) | 2026-10-23 | `show capture raw action start` | `238bdb62e9`: `internal/plugins/diag/cmd/capture_raw.go` `captureRawStartRefusal` |

## Work Not Done

All remaining implementation work is carried by the seven child specs:
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-1-core.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-2-config-plugin-map.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-3-plugin-infra.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-4-hub-runtime.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-5-api-surfaces.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-6-provision-ipxe.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-7-mock-help-flush.md`

## Review Gate

- [ ] Children 1 to 7 each passed their own Review Gate and closed
- [ ] All 46 original journal rows read `fixed:` in `plan/journal/`

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-3 satisfied by closure of Children 1 to 7 and journal cleanup
- [ ] Wiring Test table complete
- [ ] Each child's targeted tests pass in the main checkout (umbrella runs no tests of its own)

### TDD
- [ ] Per child (Children 1 to 7)
