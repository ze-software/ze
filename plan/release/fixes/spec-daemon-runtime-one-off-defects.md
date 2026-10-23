# Spec: Daemon runtime, plugin host and storage one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Each row is incorrect shipped behaviour in the hub, the plugin host and SDK, config transactions, storage, PKI, env, crash logging or appliance download |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 31 in the daemon runtime: hub
start, reload and SIGHUP, the plugin host dispatch and SDK, config transaction
rollback, blob storage, PKI leaf issue, env key lookup, crash logging, `ze
doctor`, debug and capture commands, MCP and the appliance downloader. Each row
is a claim until a red test reproduces it at HEAD.

Left to root-cause specs: `appendDataPlaneWriter`, `getConfigPathPlugins`,
`sendPostStartupTo` and the "not built reads as not configured" rows (S2).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the plugin, config and hub doors in `ai/doors/README.md`
- [ ] `docs/architecture/core-design.md`
- [ ] `ai/rules/goroutine-lifecycle.md`

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `ServerEndpoint.Listen` in
`internal/component/config/loader_extract.go` still joins host and port with a
bare colon (`Str(e.Host).Byte(':').Str(e.Port)`), so `ip ::1` yields `::1:9339`;
and `normalize` in `internal/core/env/env.go` still maps only dots to
underscores, keeping hyphens. Both rows are present.

- [ ] `pkg/plugin/sdk/sdk_engine.go` - `answerValue`
- [ ] `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput`, `dispatchCommandResponse`
- [ ] `internal/component/plugin/server/command.go` - `pluginAnswerRows`, `streamedPluginResponse`
- [ ] `internal/component/config/transaction/orchestrator.go` - `collectRollbackAcks`
- [ ] `internal/component/config/loader_extract.go` - `ServerEndpoint.Listen`
- [ ] `internal/component/config/storage/pointer.go` - `pointerPath`
- [ ] `pkg/zefs/store.go` - `BlobStore` write path
- [ ] `internal/component/pki/ca.go` - `IssueLeaf`
- [ ] `internal/core/env/env.go` - `normalize`
- [ ] `internal/core/crashlog/` - `Init`, `Flush`
- [ ] `internal/core/textbuf/` - call sites without initialiser
- [ ] `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` - MCP listener, `--plugin`, `handleSIGHUPReload`
- [ ] `internal/component/doctor/checks_listener.go` - `collectSchemaListeners`
- [ ] `internal/plugins/debug/` - `setModuleSetting`
- [ ] `internal/plugins/diag/cmd/capture_raw.go` - `captureRawStart`
- [ ] `internal/appliance/cache.go` - `defaultHTTPGet`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`ze start`, SIGHUP, an operator command dispatched to a plugin, a config commit,
a TLS listener start, `ze doctor`.

### Transformation Path
1. Hub start: env, crash log, storage, listeners.
2. Command dispatch to a plugin, answer collapse, terminator handling.
3. Transaction fan-out and rollback acknowledgement.

### Boundaries Crossed
Hub to plugin process (in-process or external); daemon to storage; daemon to network listeners.

### Integration Points
The plugin RPC answer format, the transaction orchestrator, the storage registry.

### Architectural Verification
- [ ] An answer's records and its error both reach the operator

## Risks & Assumptions

### Assumptions
- External plugins see the same answer format as in-process ones.

### Risks
- The dispatch rows change what operators see on error; error text follows `ai/rules/user-facing-errors.md`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin answer with records and an error | → | `dispatchCommandResponse` | `.ci` asserting both reach the CLI |
| Config `ip ::1` on a server endpoint | → | `ServerEndpoint.Listen` | unit asserting `[::1]:9339` |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | looking glass TLS from PKI | The looking glass consumes `pki.ServerTLSMaterial` end to end, or its deferral is re-sized from the consumer side and owned |
| AC-2 | MCP dormant block | A configured MCP block's listener is honoured; `ze.mcp.enabled` does not replace it with `127.0.0.1:8080` |
| AC-3 | dispatch error path x3 | An answer's records, status and error sentence all reach the operator when a handler returns an error |
| AC-4 | `collectRollbackAcks` | Every queued ack is read; the `Rollback:` line names only plugins that did not answer |
| AC-5 | `textbuf` initialiser | Every `textbuf.Buffer` use goes through `New`, `Get` or `Reset`; a vet-style test refuses a bare zero value |
| AC-6 | crash-dump test on arm64 | The fixture sets a state valid on the running architecture |
| AC-7 | web TLS rotation | The web listener refuses a rotation handle it cannot serve, as the looking glass does |
| AC-8 | fleet hub auth CR | The auth line reader follows the owner rule: exactly one newline, no carriage-return stripping, on both halves |
| AC-9 | `show system` command count | The count is the number of commands the plugin serves, from its registry |
| AC-10 | `ServerEndpoint.Listen` | Host and port are joined with `net.JoinHostPort`; the mgmt guard reads IPv6 loopback correctly |
| AC-11 | `defaultHTTPGet` | The request context lives until the body is read; a multi-buffer body downloads |
| AC-12 | PKI leaf validity | A leaf is reissued before expiry while the daemon runs; a unit advances the clock past 24 hours |
| AC-13 | `handshakeAt` | The helper returns an error for each failure, never `(nil, nil)` |
| AC-14 | mock `--help` lost | Output written before exit is flushed through `crashlog.Flush` |
| AC-15 | MCP tools list order | `mcp-tools-list-deterministic-order.ci` passes: the tools list is byte-stable |
| AC-16 | iPXE placeholder | `ze install remote` refuses to stage a placeholder binary, naming the file and how to fetch the real one |
| AC-17 | blob storage active pointer | The active pointer is per config name |
| AC-18 | SIGHUP queue x2 | A SIGHUP during a transaction or an SSH or web commit is queued and runs once when it ends, as the doc says |
| AC-19 | doctor listener fallback | A schema error is reported, not replaced by the hardcoded listener list |
| AC-20 | `set debug module ... flag x` | Every keyword in one command is applied or refused |
| AC-21 | `show capture raw action start` | No capture started is reported as an error with the reason |
| AC-22 | mgmt guard reload, hub `--plugin` | `mgmt-guard-reload-refuses-nonloopback.ci` and `TestRunHubConfigRejectsCLIPlugins` green |
| AC-23 | early SIGHUP | A SIGHUP in the first 300 ms after `ze start` is logged and handled, not a silent exit 129 |
| AC-24 | env hyphen keys | `env.Get` reads a hyphenated key through its underscore spelling; all 34 hyphenated keys resolve |
| AC-25 | streamed plugin answer | A plugin's own terminator message and `wrn` reach the operator |
| AC-26 | `BlobStore` cross-process | Two daemons writing one store do not overwrite each other (file lock), or the second refuses to start |
| AC-27 | plugin goroutine panic | The SDK offers a recovered goroutine starter; a panic in it is logged and the plugin keeps running |
| AC-28 | `ze signal restart` | After `ze signal restart` ze is running again under systemd and gokrazy, or the pages stop promising it; a test reads the generated unit |
| AC-29 | VPP stop signal | VPP receives SIGTERM and a bounded wait on ctx cancel (`cmd.Cancel` set); a test observes the signal |
| AC-30 | doctor foreign-namespace test | `TestRegisterDoctorCheckAcceptsForeignNamespace` passes under the full feature tag set |
| AC-31 | plugin `.ci` reds x3 (`attach-process-reload.ci`, seven `test/plugin` tests, `config-edit-ssh.ci` shutdown decode), MCP `tools/list` readiness x2 | Each runs green with the cause named, and `mcp-header-mismatch-rejected.ci` waits on a readiness signal the daemon emits |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-27 |

### Functional Tests

| Test | AC |
|------|----|
| `.ci` under `test/plugin/` and `test/reload/` | AC-3, AC-15, AC-18, AC-22 |

## Files to Modify
- The producer files under Current Behavior.

### Integration Checklist
- [ ] Each fix reached through `ze start`, a signal, a command or config

### Documentation Update Checklist (BLOCKING)
- [ ] The reload page `ai/CODE-TO-DOCS.md` names for `main_reload.go` (AC-18)

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `deferral-sized-from-the-visible-half` | 2026-09-04 | `spec-pki-full-chain` looking glass TLS |
| `enabled-gate-discards-settings` | 2026-08-10 | MCP `ze.mcp.enabled` |
| `error-path-discards-data-already-received` | 2026-09-05 | `answerValue` |
| `error-path-discards-data-already-received` | 2026-09-05 | `responseToDispatchOutput` |
| `error-path-discards-data-already-received` | 2026-09-08 | `(*Server).dispatchCommandResponse` |
| `error-path-discards-data-already-received` | 2026-10-15 | `TxCoordinator.collectRollbackAcks` |
| `fast-path-type-used-without-its-initializer` | 2026-09-02 | `internal/core/textbuf` |
| `fixture-encodes-an-impossible-state` | 2026-09-08 | `crashdump_test.go` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | hub TLS rotation, web listener |
| `guard-added-to-one-half-of-a-pair` | 2026-08-22 | fleet hub auth line reader |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-18 | `show system` command count |
| `ipv6-address-built-by-concatenation` | 2026-08-10 | `ServerEndpoint.Listen` |
| `lifetime-ends-before-its-last-reader` | 2026-10-11 | `defaultHTTPGet` |
| `material-expires-with-no-renewal-path` | 2026-09-03 | `(*Root).IssueLeaf` |
| `nil-nil-return` | 2026-09-08 | `handshakeAt` |
| `output-lost-to-an-exit-past-the-flush` | 2026-09-06 | `ze-test <mock>` help |
| `output-not-byte-stable` | 2026-08-10 | mcp tools list |
| `placeholder-shipped-as-the-real-artifact` | 2026-08-28 | `tools/ipxe-binaries/ipxe.efi` |
| `pointer-shared-across-the-names-it-indexes` | 2026-09-02 | storage active pointer |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-09-26 | `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` |
| `silent-fall-through` | 2026-08-12 | hub reload guard, functional |
| `silent-fall-through` | 2026-08-12 | hub `--plugin` |
| `silent-fall-through` | 2026-09-24 | early SIGHUP, crashlog |
| `silent-fall-through` | 2026-09-24 | env `normalize` |
| `silent-fall-through` | 2026-10-19 | `handleSIGHUPReload` |
| `silent-fall-through` | 2026-10-21 | `pluginAnswerRows` |
| `store-serializes-in-process-only` | 2026-09-03 | `BlobStore` |
| `unguarded-goroutine-kills-the-process` | 2026-09-02 | plugin goroutines |
| `claim-outlives-the-evidence-it-cites` | 2026-10-13 | `ze signal restart` promised to restart; unit has `Restart=on-failure` |
| `comment-describes-superseded-behaviour` | 2026-10-15 | `VPPManager.Run` says SIGTERM; `exec.CommandContext` sends SIGKILL |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-14 | `TestRegisterDoctorCheckAcceptsForeignNamespace` red with full tags |
| `test-against-broken-path` | 2026-08-22 | `test/plugin/attach-process-reload.ci` |
| `test-against-broken-path` | 2026-08-23 | seven `test/plugin` tests red |
| `test-against-broken-path` | 2026-08-23 | `config-edit-ssh.ci` observer shutdown decode |
| `unwired-feature` | 2026-08-15 | mcp `tools/list` first line fails |
| `unwired-feature` | 2026-08-15 | mcp `tools/list` readiness, rerun from `git archive HEAD` |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `env-key-read-but-never-registered` | 2026-09-07 | `resolve.go` reading `ze.storage.blob` | no `ze.storage.blob` read remains under `internal/core/resolve/` |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/component/plugin/... ./pkg/plugin/... ./internal/core/env/...`, `./le test functional plugin`
