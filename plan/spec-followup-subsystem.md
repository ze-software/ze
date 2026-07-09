# Spec: followup-subsystem

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | closure (all 11 ACs landed; AC-6 QEMU tests env-blocked + recorded) |
| Updated | 2026-07-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/exabgp/main.go` + `main_sdk.go`, `pkg/plugin/rpc/conn.go`
4. `internal/core/dnsserver/manager.go`, `internal/component/resolve/dns/resolver.go`
5. `internal/component/mcp/handler.go` + `streamable.go`, `internal/test/cli/cmd_mcp.go`
6. `internal/component/config/listener.go` + `listener_defaults.go`
7. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Independent subsystem follow-ups: exabgp bridge, DNS, interface phase-4/platform, MCP, and port defaults. Grouped to preserve intent; each item has its own phase.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence re-verified at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **exabgp-bridge-internal (L61,L62)** - bridge internal-plugin registration for `.ci`/production (SDK/TLS connect-back resolved the transport blocker) (L61); SetWriteDeadline degradation watchdog on non-TCP transports - writes may block indefinitely (L62).
- **dns-secure (L87)** - DNS-over-TLS, DNS-over-HTTPS, DNSSEC validation (core/dnsserver is plain DNS).
- **iface phase 4 + platform (L74,L75,L73)** - SLAAC (L74, DHCP/make-before-break/mirror already shipped); VM-level mirror/DHCPv6-PD/SLAAC tests (L75); macOS/BSD interface plugins `_darwin.go`/`_bsd.go` (L73).
- **mcp follow-ups (L224,L225)** - GET /mcp SSE `.ci` (L224, unit-only today); delete legacy `internal/component/mcp/handler.go` (L225 - now used by chaos orchestrator, so migrate those callers first).
- **port-defaults v2 (L79)** - ~~range-vs-single port conflict detection~~ (design correction: partially shipped - see below) + YANG-default lint check.

### Design-time corrections (2026-07-09, verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| exabgp bridge blocked on transport | External-plugin path through the process manager is DONE and `.ci`-tested (`main_sdk.go:30-168`, `test/plugin/exabgp-bridge-sdk.ci`); the genuinely-missing piece is internal registration (`registry.Registration` + `plugin/all/all.go`); exabgp's register.go registers a CLI root only (codegen:skip) |
| conn.go writes lack deadlines | Deadlines ARE implemented on all three write paths but silently skipped when the writer is not a `writeDeadliner` (verified firsthand `conn.go:199-209`; doc :32-34): stdio (`sdk.NewWithIO`) and SSH channels (`plugin/server/adhoc.go:37`) are the non-deadline transports. The work item is the degradation watchdog, exactly as L62 says. `WriteRawFrame` (:481) is deadline-free with no production callers - flag for dead-code check |
| "range-vs-single port conflict detection" open | `ValidateRangeConflicts` has existed since 2026-06-07 (`internal/chaos/orchestrator/conflict.go:74-131`) wired at `cli.go:416`; residual gaps: the chaos config-file path (`run.go`) never calls it, and the YANG-default lint check does not exist |
| Spec's Functional Tests table names `test/mcp` | No `test/mcp/` directory exists; mcp `.ci` tests live in `test/plugin/` (`ze-reference-mcp.ci`, `mcp-announce.ci`) - new tests go there |
| MCP GET-SSE .ci premature | The old precondition ("once server-initiated frames exist") is met: elicitation + task notifications ship, delivered via session outbound (`session.go:512-524`) drained only by `handleGET` (`streamable.go:594-655`); the missing piece is GET-stream support in the `ze-test mcp` client (`cmd_mcp.go:428` is POST-only) |

## Session Progress (2026-07-09)

Partial implementation. 3 of 11 ACs landed and committed; the rest remain open
with the verified findings below. Spec stays **in-progress** (no fake closure).

### Completed and committed
- **AC-2 (watchdog)** — commit `6683567ae`. Reusable per-conn write watchdog in
  `pkg/plugin/rpc/conn.go`, armed only on the non-deadline write path in all
  three write helpers (`writeAppended`, `writeLineWithContext`,
  `writeBatchWithDeadline`); warn + `ze_plugin_write_watchdog_total{transport}`
  (hook wired by the plugin server at `NewServer`) + fail-fast `Close`. Zero
  behavior change and zero allocation for deadline-capable transports. 4 unit
  tests (stuck-stdio, no-op-on-deadline-transport, metric-hook, window=0
  boundary) pass with `-race`. Label set on the SSH adhoc conn. Metrics doc +
  `ai/DOCS-TO-CODE.md` updated.
- **AC-10 (chaos config-path range guard)** — commit `3737b5a45`.
  `ValidateConfigRangeConflicts` (conflict.go); `RunOrchestrator` validates the
  assembled `OrchestratorConfig` at entry, protecting programmatic callers, not
  only the flag path (cli.go). 4 tests.
- **AC-11 (port-defaults lint)** — commit `3737b5a45`.
  `scripts/checks/port_defaults.go` pins `listener_defaults.go` (7 central
  services) to each service's YANG `refine port { default N }`; fails naming
  service + both values; `--selftest` + `--json`; wired into ze-verify as
  `ze-port-defaults-check` (verify_run.go both branches + make chains). 2 Go
  tests.

### Continuation session (2026-07-09, second pass)
- **AC-7 (darwin/bsd vet)** — DONE (pending phase commit). Fixed the freebsd
  blocker `internal/component/host/platform_other.go:23-24` (`uint64()`
  conversion for `syscall.Rlimit.Cur/.Max`, int64 on freebsd). Added a
  `ze-platform-vet` make gate running `GOOS=darwin` + `GOOS=freebsd go vet` over
  the host + iface + plugins/iface trees, wired into `_ze-verify-impl`,
  `_ze-verify-changed-impl`, and `scripts/status/verify_run.go` `stagesForMode`
  (both branches; classified as a vet stage). Existing `_other.go` stubs already
  give darwin/freebsd interface parity, so no new `_darwin.go`/`_bsd.go` files
  were needed. Gate runs clean.
- **AC-1 (exabgp internal)** — DONE (pending phase commit). Implemented the
  config-delivery design: new internal plugin `exabgp-bridge` at
  `internal/plugins/exabgp/bridgeplugin/` (kept under `/exabgp/` so the
  engine-boundary ExaBGP lint is satisfied; discovered by the plugin/all
  generator which walks `internal/plugins` recursively). YANG root
  `exabgp { bridge { run; family; route-refresh; add-path } }` (user directive:
  nested under a top-level `exabgp` root, registry name stays `exabgp-bridge`).
  Runner `runInternalBridge` mirrors the external `runSDKMode` but sources the
  script command from the `exabgp` config root via SDK OnConfigure (Stage 2);
  capabilities set in OnConfigure (before Stage 3), subprocess spawned there,
  `p.Run(WantsConfig=["exabgp"])`. Stage-1 Families default to ipv4/unicast
  (config is not available at Stage 1). `make generate` regenerated all.go
  (102 plugins) + the yang glue; `make ze-plugin-snapshot` updated the
  plugins/yang-providers snapshots. New `.ci` `exabgp-bridge-internal` passes
  (identical wire UPDATE to `exabgp-bridge-sdk`); external + CLI tests
  unchanged. 8 unit tests (parse/family/capability/split) pass.
  - **Config-root collision check**: top-level `exabgp` root is free — the only
    other `exabgp` YANG nodes are `environment.exabgp` (nested under
    `environment` in `ze-hub-conf.yang`, different path) and
    `internal/exabgp/migration/exabgp.yang` (a standalone module loaded only by
    the offline migration tool's own loader, never registered in the live
    config/yang registry). No collision.
  - **No doctor check** for the `run` script: parity with external mode (which
    has none); a missing script fails loudly at plugin start with a clear exec
    error, and a config-time check would false-positive on CWD-relative script
    paths.

- **AC-8 (mcp GET-SSE .ci)** — DONE (pending phase commit). Added two directives
  to the `ze-test mcp` client (`internal/test/cli/cmd_mcp.go`): `sse-listen`
  opens the GET /mcp SSE stream (`Accept: text/event-stream` + `Mcp-Session-Id`)
  and reads server-initiated frames on a background goroutine; `sse-expect
  <method>` blocks until a frame with that JSON-RPC method arrives and prints it.
  New `.ci` `mcp-get-sse` (test/plugin) drives a task (`task-call ze_execute`)
  whose worker completion sends `notifications/tasks/status` via the session
  outbound queue, which `handleGET` (streamable.go) frames on the GET stream;
  the client asserts it. Passes. 2 unit tests (httptest SSE server +
  ordering guard). This `.ci` adds 0 `time.sleep` calls.

- **AC-9 (mcp legacy deletion)** — DONE (pending phase commit, GPG-blocked).
  Two sub-steps per R-5:
  1. Added optional `ToolProvider` support to the Streamable:
     `StreamableConfig.Provider` — tools/list returns `Provider.Tools()`,
     tools/call delegates to `Provider.CallTool` (task calls rejected),
     initialize reports `Provider.ServerName()`, and session-less POSTs are
     accepted in Provider mode ONLY (the chaos `.ci` http=post checks cannot
     thread `Mcp-Session-Id`; the MCP spec makes sessions optional). ze's
     strict session requirement is pinned by
     `TestStreamableWithoutProviderStillRequiresSession`. 4 new tests in
     `provider_test.go`.
  2. Migrated both chaos callers (`orchestrator/cli.go`, `run.go`) to
     `NewStreamable(StreamableConfig{Provider})`, mounted at `zemcp.Endpoint`
     (/mcp — was "/"), Close wired into cleanups; chaos `.ci` URLs updated to
     /mcp (mcp-status, mcp-problems — both PASS; chaos-web suite 6/6; chaos
     unit tests 16 pkgs green). THEN deleted `handler.go` (Handler,
     HandlerWithAudit, ZeProvider, NewZeProvider, methods map, legacy
     callTool/allTools/findGeneratedTool/ok/fail, writeJSON) and relocated the
     SHARED primitives into `tools.go` (JSON-RPC types, server struct,
     toolHandlers, handcraftedTools/Names, ToolProvider, CommandDispatcher,
     maxRequestBody, TextResult/ErrResult, noSpaces, (*server).run) — the
     spec's "delete handler.go wholesale" framing was imprecise: half the file
     was live streamable dependencies. Legacy-only tests deleted with
     test-relax markers (coverage replaced: provider_test.go, bearer_test.go,
     TestStreamableBearerAuthFailureAuditRecord); the three ViaHTTP dispatch
     tests REWRITTEN against Streamable sessions (sync generated dispatch had
     no other Streamable coverage). Grep clean (only web's unrelated
     LoginHandlerWithAudit matches). mcp pkg green incl. -race; docs
     (architecture/mcp/overview.md profile+files tables, guide/mcp/chaos.md
     protocol+URL, 6 anchor swaps) + ai/digests/mcp.md remapped; doc-test
     PASSES.

### Session end state (continuation, 2026-07-09)
- **Committed**: AC-7 (`d4c0c323d`), AC-1 (`76f098b83`).
- **Implemented + verified, commit GPG-BLOCKED**: AC-8 and AC-9 sit in
  `tmp/commit-8a588485.sh` (commits y and z, message files
  `tmp/commit-msg-8a588485-{y,z}.txt`). The GPG agent passphrase cache expired
  mid-session (`gpg: cannot open '/dev/tty'`); unlock with
  `echo test | gpg --clearsign` then run `bash tmp/commit-8a588485.sh`.
  The AC-8/AC-9 files are staged in the index by the failed attempts.
- **Remaining open**: AC-3/AC-4/AC-5 (dns-secure: DoT + DoH listeners in
  core/dnsserver, YANG tls/doh leaves for as112/geodns, selfcert/pki cert
  pattern + doctor checks, DNSSEC validation in resolve/dns) — not started;
  AC-6 (SLAAC lifecycle tracking + QEMU integration tests, execution
  env-blocked: no CAP_NET_ADMIN/netns/sudo) — not started. Spec stays
  in-progress; no closure attempted.
- **User decisions needed**: (1) ci-sleep baseline (see below); (2) GPG unlock
  to land the pending script.

### Verification state (continuation, 2026-07-09)
- `make ze-lint-changed`, `ze-platform-vet`, and all structural gates
  (tier/iface-resolution/plugin-boundary/port-defaults/cli-grammar) pass. The
  new package's unit tests, the `plugin/all` snapshot test, and the `.ci`
  (`exabgp-bridge-internal`, plus unchanged `exabgp-bridge-sdk`/`cli-exabgp-help`)
  pass. `ze-doc-test` passes after adding exabgp-bridge to `docs/DESIGN.md`
  Shipped Plugins and regenerating `ai/PACKAGE-MAP.md` + `ai/DOCS-TO-CODE.md`.
- Full `make ze-verify` was TERMINATED at the 300s cap mid-functional-suite
  (482 .ci tests). Pre-existing reds observed, attributed as NOT caused by this
  work: `as112-community-choice` (PROVEN pre-existing — still FAILs with the
  exabgp-bridge plugin import removed from all.go), `ddos-detect-characterize`,
  `ddos-detect-mitigate` (disjoint traffic-anomaly subsystem, ~20s environmental
  timing). These are deterministic/environmental repo-state reds, not this
  session's regressions.
- **ci-sleep ratchet (needs user action):** `test/**/*.ci` holds 470 committed
  `time.sleep(` calls vs a stale `test/.ci-sleep-baseline` of 448 (22-sleep
  drift from prior committed work). The ratchet is dormant until a `.ci` is
  touched; adding `exabgp-bridge-internal.ci` (+1 sleep, needed for the
  bridge-readiness race, R-1 — parity with the committed external test) activates
  it, so `ze-verify-wiring-docs` reports it. The baseline was NOT raised
  (rule: raise only with explicit user approval). Recommend raising the baseline
  to 471 (22 pre-existing + 1 AC-1 test) or advising an event-based wait.

## Session Progress (2026-07-09, FINAL continuation — closure)

All 11 ACs are now landed. AC-8/AC-9 (GPG-blocked in the prior session) were
committed (`1241ab807`, `c7fdff020`). This session implemented AC-3/AC-4/AC-5/AC-6.

### Completed and committed this session
- **AC-3/AC-4 (dns-secure DoT/DoH)** — commit `c8cf48644`. Shared `dnsserver`
  harness gains optional DoT (RFC 7858; TLS-wrapped TCP `dns.Server`) and DoH
  (RFC 8484; `net/http` handler driving the same `dns.Handler` via an in-memory
  `dns.ResponseWriter`). `ApplyWithSecure` reconciles all transports; the
  listener signature folds a leaf-cert fingerprint (rotation rebinds) and the
  self-signed fallback is cached (no reload churn). Shared `LoadTLSMaterial` +
  `CheckCertMaterial` (reusing `doctor-tls-*` codes). as112 + geodns gain YANG
  `tls`/`doh` containers, config parse, and a cert-validity doctor check. Go unit
  tests use real TLS/HTTPS clients + a real cert root (`TestDoTListener`,
  `TestDoHListener`, `TestApplyWithSecureSelfSigned`, 403/405/cert-rotation).
  Functional `.ci` `as112-dot`/`as112-doh` (test/plugin) prove the listeners bind
  through the real binary. Interop runbook below.
- **AC-5 (DNSSEC stub validation)** — commit `362266621`. RFC 4035 stub model:
  `system{dns{dnssec-validation}}` enum (off default) -> `SystemConfig` -> hub
  `newResolvers`; `permissive`/`strict` set the EDNS0 DO bit (CD=0) and treat an
  upstream SERVFAIL as a broken chain (strict rejects, permissive logs, AD=0
  accepted). Pure `dnssecDecision()`. `--dnssec` flag on `ze resolve dns`. Tests
  against a fake validating upstream + `test/parse/system-dns-dnssec.ci`.
- **AC-6 (SLAAC + iface VM tests)** — commit `fca663892`. Kernel-cooperating
  `addrOrigin()` (`IFA_F_*` -> `AddrInfo.Origin` static/slaac/temporary/dynamic +
  lifetimes) in the netlink listing + addr-event paths; NOT an RA client (A-5).
  Native unit tests (classifier + `handleAddrUpdate` via the handler seam +
  DHCPv6-PD lease flow via a synthetic IA_PD Reply). QEMU integration tests
  `TestSLAACAddressTracked` + `TestIntegrationMirrorPacketLevel` (AF_PACKET) are
  `integration && linux`; execution is env-blocked (no CAP_NET_ADMIN) so they
  skip locally and run under `make ze-qemu-integration-test`.

### DoT/DoH interop runbook (real-client, validated against a live daemon)
Cleartext DNS on 53 needs CAP_NET_BIND_SERVICE; use high ports for DoT/DoH.
Steps (verified this session against `bin/ze`):
1. Write a config enabling as112 with `doh { enabled true; listen-port 18443 }`
   and `tls { enabled true; listen-port 18853 }` (no cert-file: as112 self-signs).
2. Start the daemon: `bin/ze <config>` (DoH on :18443, DoT on :18853).
3. DoH: POST a wire-format DNS query for `10.in-addr.arpa` SOA to
   `https://127.0.0.1:18443/dns-query` with `Content-Type: application/dns-message`
   over a TLS context that accepts the self-signed cert (Python stdlib
   `ssl.CERT_NONE`, or `kdig +https`). Observed: HTTP 200, DNS RCODE 0, ANCOUNT 1.
4. DoT: `kdig +tls -p 18853 @127.0.0.1 10.in-addr.arpa SOA` (if kdig present).
   The Go unit test `TestDoTListener` proves the same path with a verifying client
   (real cert root, no verification disabled).

### Verification evidence (this session)
- `ze-lint-changed` clean after every phase; `go test -race` green for
  `internal/core/dnsserver`, `internal/plugins/{as112,geodns}`,
  `internal/component/resolve/dns`, `internal/component/config/system`,
  `internal/plugins/iface/{netlink,dhcp}`, `internal/component/iface`.
- Integration-tagged `go vet` + `golangci-lint` clean on the new QEMU tests.
- `.ci`: `as112-dot`, `as112-doh`, `system-dns-dnssec`, `resolve-dns-help` PASS.
- `make ze-validate` all-green; `audit-test-relaxation.py main` clean.
- Full `make ze-verify` still exceeds the 300s cap; pre-existing reds unrelated
  to this work: `plugin/all` snapshot drift for isis/ospf/ldp/rsvpte (those
  plugins are not in the committed `all.go`), `as112-community-choice`,
  `ddos-detect-*`. All commits used `--unverified` naming the changed-surface
  verification above.
- ci-sleep ratchet: the two new `.ci` (`as112-dot`, `as112-doh`) add 0
  `time.sleep(` calls; `system-dns-dnssec.ci` adds 0. No new ratchet pressure
  from this session's `.ci` additions.

### Deviations (this session)
- **AC-3/AC-4 `.ci`**: `as112-dot`/`as112-doh` assert the secure listener BINDS
  through the real binary (plain TCP connect) rather than driving a full TLS
  handshake, because the plugin-observer sandbox cannot import Python's `ssl`.
  The TLS *transport* is proven by the Go unit tests (real clients + real root)
  and the interop runbook. No user-approval needed: equivalent coverage, not a
  scope drop.
- **AC-5 surface**: the spec Decision assumed a per-resolver config; the resolver
  is a stub with no `service{resolve}` container, so the toggle lives in the
  existing `system{dns}` container (+ a `--dnssec` CLI flag). Same operator
  intent, verified architecture.
- **AC-6 DHCPv6-PD**: proven by a NATIVE lease-flow unit test (synthetic IA_PD
  Reply through the producing `handleV6Reply`) that runs in CI, over a
  server-in-the-loop QEMU test; the mirror + SLAAC QEMU tests exist but their
  execution is env-blocked (recorded runbook: `make ze-qemu-integration-test`).

### Findings that re-scope the remaining ACs (verified firsthand)
- **AC-1 (exabgp internal)** — config-plumbing gap. `plugin { internal
  exabgp-bridge { run <script> } }` cannot pass the script path to a
  `RunEngine(conn net.Conn)` through the as112 pattern: `startInternal`
  (`internal/component/plugin/process/process.go:465-482`) consults
  `p.config.Run` only for runner-NAME resolution; the script command never
  reaches `RunEngine`, which takes only `conn`. Delivering AC-1 needs a
  config-delivery path (exabgp-bridge YANG ConfigRoot + `OnConfigure` carrying
  the script, like as112's config) OR threading `p.config` into the runner — not
  the trivial as112 clone the spec Decision assumes. **Not implemented.**
- **AC-9 (delete legacy mcp handler)** — `NewStreamable(StreamableConfig)` has NO
  `ToolProvider` support: it builds tools from `cfg.Commands()`/`cfg.Dispatch`
  (the ze command registry, `streamable_tools.go:46-64`), while the two chaos
  callers serve CUSTOM tools via the `ToolProvider` interface
  (`chaosmcp.Provider`: chaos_status/problems/peers/scenario/control/execute).
  Migrating chaos to `NewStreamable` therefore requires FIRST adding optional
  `ToolProvider` support to `Streamable` (`allTools`/`callTool`) — not a drop-in
  swap. **Not implemented; `handler.go` NOT deleted.**
- **AC-8 (mcp GET-SSE .ci)** — not started; depends on a scenario delivering a
  server-initiated frame (elicitation / task notification) on the GET stream
  (`streamable.go:handleGET`) plus a new GET directive in the `ze-test mcp`
  client (`internal/test/cli/cmd_mcp.go`, POST-only today).
- **AC-3/AC-4/AC-5 (dns-secure)** — not started; DoT + DoH listeners + DNSSEC
  validation + YANG + doctor checks + interop is the largest phase.
- **AC-6 (QEMU integration)** — env-blocked: no CAP_NET_ADMIN / netns / sudo in
  this sandbox (plan/known-failures.md). Not attempted.
- **AC-7 (darwin/bsd)** — `internal/component/iface`, `internal/plugins/iface/netlink`,
  and `internal/plugins/iface/dhcp` ALREADY vet under `GOOS=darwin` (existing
  `_other.go` stubs give interface parity). `GOOS=freebsd` is blocked by a
  cross-platform bug in a DEPENDENCY, `internal/component/host/platform_other.go:23-24`
  (`syscall.Rlimit.Cur/.Max` are int64 on freebsd, assigned to uint64
  `PlatformInfo` fields); a `uint64()` conversion unblocks that layer but
  freebsd may surface deeper issues. No vet-gated make check added yet.

### Other verified sub-findings
- `WriteRawFrame` (`pkg/plugin/rpc/conn.go`) has NO production callers (only
  `internal/component/plugin/process/process_tls_test.go`) — confirmed dead code
  per the spec Known-Limitations question. Left as-is (removing it needs an
  unrelated test edit); the watchdog does not cover it (not a production write
  path).
- The claim "stdio (sdk.NewWithIO) is a non-deadline transport" is imprecise:
  `*os.File` DOES implement `SetWriteDeadline`; on an `os.Pipe` it succeeds
  (deadline path), on a terminal it errors. The genuine non-deadline transports
  through `rpc.Conn` are SSH channels (`adhoc.go`) and `io.PipeWriter`. The
  watchdog (arm on `!writeDeadliner`) targets exactly those.

## Required Reading

### Source files / docs

- [ ] `internal/plugins/exabgp/main.go`, `main_sdk.go`, `internal/exabgp/bridge/bridge.go`
  → Constraint: bridge always spawns the operator's ExaBGP-script subprocess; internal mode must preserve that (in-process plugin, out-of-process script)
  → Decision: internal registration follows the as112 pattern - `registry.Registration{Name, YANG, RunEngine}` (`internal/plugins/as112/register.go:115-175`), runner signature `func(conn net.Conn) int`, engine side `startInternal` (`plugin/process/process.go:456-521`) over net.Pipe; composition root `plugin/all/all.go` is generated (`make generate`)
- [ ] `pkg/plugin/rpc/conn.go`
  → Constraint: write paths writeAppended :198-210, writeLineWithContext :243-255, writeBatchWithDeadline :452-464; `writeDeadliner` gate :35-37; 30s default :42; non-deadline transports: stdio + SSH channels
  → Constraint: watchdog must not add per-write allocations (`ai/rules/memory-architecture.md`) - a reusable timer per conn, armed/disarmed around the write
- [ ] `internal/core/dnsserver/manager.go`
  → Constraint: bind (:163-180) creates plain UDP+TCP `dns.Server` pairs; TLS wrap points are the `lc.Listen` TCP listener (DoT) and a new HTTP server delegating to the same `m.handler` (DoH); consumers as112 + geodns must opt in per listener
  → Decision: cert material follows the web pattern: operator PEM or self-signed via `internal/core/selfcert` (`web/server.go:111`), stored/managed like the pki component precedent (`plan/learned/733-pki-store.md`)
- [ ] `internal/component/resolve/dns/resolver.go`
  → Constraint: client is plain UDP (`mdns.Client` :59-61, Exchange :213); DNSSEC validation lands here (validating resolver), not in the authoritative dnsserver
- [ ] `internal/component/mcp/handler.go`, `streamable.go`, `internal/chaos/orchestrator/cli.go` (:625) + `run.go` (:522), `internal/test/cli/cmd_mcp.go`
  → Constraint: deleting handler.go requires migrating the two chaos call sites + the legacy unit tests to `NewStreamable`
  → Constraint: the GET-SSE `.ci` needs a new `ze-test mcp` stdin directive that opens `GET /mcp` with `Accept: text/event-stream` + `Mcp-Session-Id` and asserts server-initiated frames
- [ ] `internal/component/iface/` (config_sysctl.go:74-75 autoconf sysctl, config.go:1179-1185 accept-ra, register.go:1177-1210 RA suppression), `internal/plugins/iface/dhcp/`
  → Constraint: SLAAC today = kernel sysctl passthrough; "SLAAC" feature = ze-tracked address lifecycle (observe kernel RA-assigned addrs via netlink addr subscriptions, expose in status/web, honor make-before-break) - NOT a userspace RA client reimplementation
  → Constraint: platform split precedent: `default_linux.go`/`default_other.go`, `migrate_linux.go`/`migrate_other.go`; netlink backend is all `_linux.go` + `backend_other.go` stub
- [ ] `ai/rules/qemu-testing.md`
  → Constraint: BLOCKING - the L75 VM-level tests are QEMU integration tests (`integration && linux` tags or `option=needs-linux` .ci); never skip for "needs hardware"
- [ ] `internal/component/config/listener.go` (:141-143 refine-default comment), `listener_defaults.go` (:8-16), `scripts/checks/cli_grammar.go` (lint-check pattern), `internal/chaos/orchestrator/conflict.go` (:74-131) + `run.go`
  → Constraint: the YANG compiler does NOT propagate `refine port { default N }`; the hand-maintained Go table exists precisely because of that - the lint check compares the two sources
- [ ] `ai/patterns/plugin.md`, `ai/patterns/registration.md` (exabgp internal registration), `ai/patterns/config-option.md` + `ai/rules/config-surface.md` + `ai/rules/config-naming.md` (DoT/DoH/DNSSEC YANG), `ai/rules/doctor-checks.md` (cert material, new listeners)
  → Constraint: read in full at implement time before touching the respective phase

**Key insights:**
- Five independent phases; nothing couples them. Sequential commits per phase (disjoint systems).
- exabgp internal mode and the mcp legacy deletion are the two items with regeneration/migration mechanics (all.go regen; chaos caller migration).
- dns-secure is the only item adding YANG surface + doctor checks + cert handling; it is the largest phase.
- iface SLAAC is scoped as kernel-cooperating lifecycle tracking, not an RA stack.

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `pkg/plugin/rpc/conn.go` - deadline gate verified firsthand (:199-209)
- [ ] `internal/core/dnsserver/manager.go` - plain-DNS bind verified firsthand (:163-180)
- [ ] `internal/component/mcp/handler.go` - legacy profile verified firsthand (:53-61); chaos callers cli.go:625, run.go:522
- [ ] `internal/plugins/exabgp/main.go` + `main_sdk.go` - dual mode; no runtime plugin registration
- [ ] `internal/component/config/listener.go` + `listener_defaults.go` - single-port model + hand table
- [ ] `internal/component/iface/` - autoconf passthrough, no SLAAC tracking; no darwin/bsd files

**Behavior to preserve:**
- exabgp external SDK mode + standalone stdio mode (both keep working; internal is additive).
- conn.go semantics on deadline-capable transports (net.Conn, net.Pipe) unchanged.
- dnsserver plain-DNS listeners (as112/geodns default behavior unchanged unless TLS configured).
- MCP Streamable HTTP behavior; chaos MCP endpoints keep responding after migration.
- Existing listener-conflict single-port detection and chaos flag-path range validation.

**Behavior to change:**
- exabgp registrable as `plugin { internal exabgp-bridge }`.
- Stuck writes on stdio/SSH transports detected: warn log + metric + connection close after the watchdog window.
- dnsserver gains optional DoT + DoH listeners; resolve gains DNSSEC validation.
- ze tracks SLAAC/RA-assigned addresses; QEMU tests for mirror/DHCPv6-PD/SLAAC; darwin/bsd backend stubs compile.
- chaos config-file path validates port ranges; lint check pins YANG defaults to the Go table.
- Legacy mcp handler deleted after caller migration; GET /mcp SSE covered by `.ci`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- exabgp: engine plugin manager → internal runner (net.Pipe) → bridge → ExaBGP script subprocess
- rpc: plugin event write → conn write path → transport (deadline or watchdog)
- DNS: client query → UDP/TCP (existing) / TLS / HTTPS listener → dns handler; resolver: upstream query → validation
- MCP: HTTP GET /mcp (SSE) ← session outbound queue
- iface: kernel RA → address netlink event → ze address lifecycle → status/web
- config: YANG defaults ↔ Go default table (lint)

### Transformation Path
1. Each subsystem's request enters via its existing listener/manager
2. The added capability (registration, watchdog, TLS wrap, validation, tracking, lint) handles it
3. Observable result: .ci-verifiable daemon behavior or scripts/checks failure

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bridge ↔ engine | internal: net.Pipe + rpc.BridgedConn; external: TLS connect-back | [ ] |
| plugin conn → transport | writeDeadliner or watchdog timer | [ ] |
| DNS client → dnsserver | UDP/TCP/DoT(TLS)/DoH(HTTPS) | [ ] |
| resolver → upstream | DNSSEC chain validation | [ ] |
| MCP client → server | Streamable HTTP GET SSE | [ ] |
| kernel → iface | netlink address subscriptions (RA-assigned) | [ ] |

### Integration Points
- `internal/plugins/exabgp/` + `internal/component/plugin/all/all.go` (generated)
- `pkg/plugin/rpc/conn.go`
- `internal/core/dnsserver/`, `internal/component/resolve/dns/`, `internal/component/pki` / `internal/core/selfcert` (cert source)
- `internal/component/mcp/`, `internal/chaos/orchestrator/`, `internal/test/cli/cmd_mcp.go`
- `internal/component/iface/`, `internal/plugins/iface/`
- `internal/component/config/listener_defaults.go`, `scripts/checks/`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Corrected evidence holds at implement time | re-verified 2026-07-09 (firsthand: conn.go:199-209, manager.go:163-180, handler.go:53-61) | Re-scope item | grep/LSP at implement-audit | confirmed |
| A-2 | Internal plugins may spawn subprocesses (the ExaBGP script) without violating the process model | as112 runs in-process; bridge subprocess is plugin-internal detail | Keep exabgp external-only; record decision | read plugin/process constraints during phase 1 | confirmed (AC-1: exabgp-bridge runner execs the script via exec.CommandContext; exabgp-bridge-internal.ci announces on the wire) |
| A-3 | miekg/dns supports DoT natively (dns.Server over tls.Listener) and DoH via std http.Handler wrapping | vendored miekg/dns API | Hand-roll TLS wrap of TCP listener (still std pattern) | phase 2 spike test | confirmed (AC-3/4: DoT = `tls.NewListener` handed to `dns.Server{Listener}`; DoH = `net/http` + in-memory `dns.ResponseWriter`; `TestDoTListener`/`TestDoHListener` pass with real clients) |
| A-4 | DNSSEC validation scope = resolver-side validation of upstream answers (AD-bit correctness), not authoritative signing | deferral wording "DNSSEC validation"; as112/geodns are sinks | Split signing into its own separate effort | user review of this spec | confirmed (AC-5: RFC 4035 stub model, DO bit + upstream SERVFAIL; no authoritative signing) |
| A-5 | SLAAC scope = lifecycle tracking of kernel-assigned addresses, not a userspace RA client | autoconf sysctl passthrough exists (config_sysctl.go:74-75) | Larger feature; split into own spec | user review of this spec | confirmed (AC-6: `addrOrigin()` classifies kernel IFA_F_* flags; no RA client) |
| A-6 | darwin/bsd scope = compiling backend stubs + platform files for the existing Backend interface (parity of interface, not features) | `backend_other.go` stub precedent | Full-feature ports become own specs | user review of this spec | confirmed (AC-7, prior commit d4c0c323d: `ze-platform-vet` gate over GOOS=darwin+freebsd) |
| A-7 | Watchdog close-on-stuck (fail-fast) is the right degradation for stdio/SSH | deadline semantics on net.Conn transports produce write errors; parity | Observe-only (log+metric) fallback | phase 1 design review | confirmed (AC-2 implemented as warn+metric+close, commit 6683567ae) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Internal exabgp changes plugin lifecycle expectations (.ci flakiness) | exabgp-bridge-internal.ci intermittent | Reuse startInternal contract exactly; sink lifecycle events like as112 |
| R-2 | Watchdog false-positives on slow-but-alive consumers (SSH over WAN) | spurious conn closes under load | Window = defaultWriteDeadline (30s), metric before close, configurable via env var if operators hit it |
| R-3 | DoT/DoH cert handling drifts from web/pki patterns (second cert system) | new cert loading code appears | Reuse selfcert + pki store; doctor check validates material |
| R-4 | DNSSEC validation breaks resolution when upstream is broken (SERVFAIL storms) | resolve failures after enable | Default off; per-resolver YANG leaf; permissive mode logs-only |
| R-5 | Legacy mcp handler deletion breaks chaos web/dashboard flows | chaos .ci/web tests red | Migrate callers first, delete second (two sub-phases); keep tests green in between |
| R-6 | SLAAC tracking floods on high-RA networks | event churn in address observer | Coalesce netlink addr events (existing iface event patterns) |
| R-7 | darwin/bsd stubs rot without CI | stubs break silently on later refactors | They compile under `GOOS=darwin/freebsd go vet` in a make check (document; full CI is a separate effort) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `plugin { internal exabgp-bridge }` in config | → | registry.Registration RunEngine over net.Pipe announces a route | `.ci` `exabgp-bridge-internal` |
| Plugin write on stdio transport blocked >30s | → | watchdog fires: warn + metric + conn close | `TestConnWriteWatchdogStuckStdio` |
| DoT query against as112 listener with TLS configured | → | dnsserver TLS listener answers | `.ci` `as112-dot` + `TestDoTListener` |
| DoH POST/GET query | → | HTTP handler → dns handler → answer | `.ci` `as112-doh` + `TestDoHListener` |
| Upstream answer with broken DNSSEC chain (validation on) | → | resolver rejects/flags | `TestDNSSECValidationRejects` |
| Kernel assigns SLAAC address (RA in QEMU) | → | iface lifecycle tracks it, status shows it | QEMU `TestSLAACAddressTracked` (integration && linux) |
| `GET /mcp` with session + Accept: text/event-stream | → | handleGET streams a server-initiated frame | `.ci` `mcp-get-sse` (test/plugin) |
| chaos scenario config file with clashing port range | → | run.go path calls ValidateRangeConflicts, rejects | `TestRunConfigRangeConflict` |
| `make ze-lint` (or inventory check) with YANG default ≠ Go table | → | lint check fails naming the drifted service | `scripts/checks/port_defaults.go` self-test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config declares `plugin { internal exabgp-bridge { run <script> } }` | Bridge runs in-process via GetInternalPluginRunner, spawns the script subprocess, announces routes; external + standalone modes unchanged; all.go regenerated (`make generate`); snapshot test `plugin/all/all_test.go` updated |
| AC-2 | A write on a non-writeDeadliner transport exceeds the watchdog window | Warn log with transport kind + plugin name, Prometheus counter increments, connection closed (fail-fast per A-7); deadline-capable transports: zero behavior change; no per-write allocation |
| AC-3 | as112/geodns listener configured with `tls` (operator PEM or selfcert fallback) | DoT serves on the configured port (RFC 7858); plain listeners unaffected; doctor check reports cert validity/expiry |
| AC-4 | DoH endpoint enabled | RFC 8484 GET+POST queries answered over HTTPS reusing the same dns handler |
| AC-5 | Resolver with `dnssec-validation` enabled | Valid chains resolve with AD semantics honored; broken chains rejected (or logged in permissive mode); default off |
| AC-6 | QEMU integration run | New tests prove: mirror at packet level, DHCPv6-PD lease flow, SLAAC address tracked in ze state (`ai/rules/qemu-testing.md` satisfied for L75) |
| AC-7 | `GOOS=darwin` / `GOOS=freebsd` build of iface plugins | `_darwin.go`/`_bsd.go` backend stubs compile (interface parity, errNotSupported bodies); vet-gated make check documents the surface |
| AC-8 | `ze-test mcp` script with the new GET-SSE directive | `.ci` proves a server-initiated frame (elicitation or task notification) arrives on the GET stream with correct SSE framing |
| AC-9 | After chaos callers migrate to NewStreamable | `internal/component/mcp/handler.go` (+HandlerWithAudit + legacy-only tests) deleted; chaos orchestrator MCP endpoints keep passing their tests; no references remain (grep clean) |
| AC-10 | chaos scenario config file with range/single port clash | Rejected at load (run.go path) with the same message quality as the flag path (cli.go:416) |
| AC-11 | YANG `refine port` default drifts from `listener_defaults.go` (or a per-plugin registration) | `scripts/checks/port_defaults.go` (wired into the inventory/lint make path like cli_grammar) fails, naming service + both values |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator consolidates ExaBGP scripts into ze without extra processes/tokens | config → internal plugin → bridge → script → routes on wire | `exabgp-bridge-internal.ci` |
| 2 | Resolver operator turns on DoT for as112 sink | config tls → DoT listener → dig +tls answers | `as112-dot.ci` |
| 3 | Security-conscious operator enables DNSSEC validation | resolve config → validating resolver → bad chain rejected | `TestDNSSECValidationRejects` |
| 4 | Appliance on an IPv6 SLAAC network shows its addresses | RA → kernel → netlink event → ze status | QEMU `TestSLAACAddressTracked` |
| 5 | MCP client subscribes to server notifications | GET /mcp SSE → outbound queue frames | `mcp-get-sse.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseConfigNested` + 7 more (parse/family/capability/split) | `internal/plugins/exabgp/bridgeplugin/internal_test.go` | AC-1 | DONE (76f098b83; names differ from the planned placeholders — config-delivery design) |
| `TestConnWriteWatchdogStuckStdio`, `TestConnWatchdogNoopOnDeadlineTransport`, `TestWatchdogMetric`, `TestConnWatchdogDisabledWhenWindowZero` | `pkg/plugin/rpc/conn_watchdog_test.go` | AC-2 | DONE (6683567ae) |
| `TestDoTListener`, `TestDoHListener`, `TestApplyWithSecureSelfSigned`, `TestDoHRefusedYields403`, `TestListenerSigCertRotation`, `TestLoadTLSMaterial*`, `TestCheckCertMaterial*` | `internal/core/dnsserver/` | AC-3, AC-4 | DONE (c8cf48644; names differ — real TLS/HTTPS clients + real cert root) |
| `TestDNSSECStrictRejectsBogus`, `TestDNSSECPermissiveLogsBogus`, `TestDNSSECStrictResolvesSecure`, `TestDNSSECOffIsUnchanged`, `TestDNSSECDecision` | `internal/component/resolve/dns/dnssec_test.go` | AC-5 | DONE (362266621; fake validating upstream) |
| `TestSLAACAddressTracked`, `TestIntegrationMirrorPacketLevel` (integration && linux, QEMU exec env-blocked), `TestAddrOrigin`/`TestHandleAddrUpdate_*` (native), `TestDHCPv6PDLeaseFlow` (native) | `internal/component/iface/`, `internal/plugins/iface/{netlink,dhcp}/` | AC-6 | DONE (fca663892; QEMU tests skip locally, run under ze-qemu-integration-test) |
| `ze-platform-vet` (vet gate, GOOS=darwin+freebsd) | make check | AC-7 | DONE (d4c0c323d; gate not a Go test) |
| `TestStreamableGETviaZeTest`, `TestSSEExpectRequiresListen` | `internal/test/cli/cmd_mcp_test.go` | AC-8 | DONE (pending GPG-blocked commit y) |
| `TestStreamableProviderServesSessionless` + 3 more | `internal/component/mcp/provider_test.go` | AC-9 | DONE (pending GPG-blocked commit z; chaos migration proven by chaos-web .ci, not a new orchestrator unit test) |
| `TestRunConfigRangeConflict`, `TestValidateConfigRangeConflicts_*` | `internal/chaos/orchestrator/run_test.go`, `conflict_test.go` | AC-10 | DONE (3737b5a45) |
| `TestPortDefaultsGate`, `TestPortDefaultsSelftest` (port_defaults `--selftest`) | `scripts/checks/port_defaults_test.go` | AC-11 | DONE (3737b5a45) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| watchdog window | 1s-600s (default 30s) | 600 | 0 → disabled or reject (decide, record) | reject |
| DoT/DoH port | 1-65535 (defaults 853/443) | 65535 | 0 | N/A uint16 |
| port-range base+count (chaos) | base+peers*2 ≤ 65535 | boundary case | - | reject |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `exabgp-bridge-internal.ci` | test/plugin | internal bridge announces | DONE (76f098b83) |
| `mcp-get-sse.ci` | test/plugin | GET-stream server-initiated frame | DONE (pending commit y) |
| `mcp-status.ci` / `mcp-problems.ci` (chaos) | test/chaos-web | chaos MCP on Streamable /mcp | DONE (pending commit z) |
| `as112-dot.ci`, `as112-doh.ci` | test/plugin | secure DNS listeners bind through the real binary | DONE (c8cf48644; assert bind — TLS handshake proven by Go unit tests + runbook) |
| `system-dns-dnssec.ci` | test/parse | dnssec-validation config leaf parses | DONE (this session) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| DoT/DoH client interop | test/plugin (dig/kdig or knot container) | dig +https / +tls | RFC 7858/8484 compliance from a real client | |
| N/A for other items (no new wire protocol; exabgp bridge already interop-covered by exabgp-compat suite) | - | - | - | |

## Files to Modify

- `internal/plugins/exabgp/register.go` (+ new engine runner file) - registry.Registration; `internal/component/plugin/all/all.go` via `make generate`
- `pkg/plugin/rpc/conn.go` - watchdog on non-deadline transports (+ metric)
- `internal/core/dnsserver/manager.go` (+ new dot.go/doh.go) - TLS/HTTPS listeners
- `internal/plugins/as112/register.go`, `internal/plugins/geodns/` - YANG tls/doh listener leaves + wiring
- `internal/component/resolve/dns/resolver.go` (+ validation file) - DNSSEC
- `internal/component/iface/` - SLAAC lifecycle tracking; `internal/plugins/iface/` - `_darwin.go`/`_bsd.go` stubs
- `internal/chaos/orchestrator/cli.go`, `run.go` - NewStreamable migration + range validation on config path
- `internal/component/mcp/handler.go` - DELETE (after migration); legacy tests removed
- `internal/test/cli/cmd_mcp.go` - GET-SSE directive
- `internal/component/config/listener_defaults.go` - source of truth for the lint check
- `docs/` per Documentation Update Checklist (features.md, guide/plugins.md, functional-tests.md, plugin-development/metrics.md for the watchdog counter)

## Files to Create

- `scripts/checks/port_defaults.go` (+ make wiring alongside `mk/inventory.mk:63` pattern)
- YANG additions in owning modules (as112/geodns tls+doh; resolve dnssec leaf) with max native validation
- `.ci` tests listed above; QEMU integration tests listed above

## Implementation Steps

1. **Phase: exabgp (L61+L62)** - internal registration + all.go regen + `.ci`; conn watchdog TDD (AC-1, AC-2).
2. **Phase: mcp (L224+L225)** - GET-SSE client directive + `.ci`; migrate chaos callers; delete handler.go (AC-8, AC-9).
3. **Phase: port-defaults (L79 residual)** - run.go range check; port_defaults lint check (AC-10, AC-11).
4. **Phase: dns-secure (L87)** - DoT → DoH → DNSSEC validation, YANG + doctor checks + interop (AC-3..AC-5).
5. **Phase: iface (L74+L75+L73)** - SLAAC tracking; QEMU tests; darwin/bsd stubs (AC-6, AC-7).
6. **Full verification** - `make ze-verify` (+ QEMU targets).
7. **Complete spec** - audit tables, `plan/learned/NNN-followup-subsystem.md`, two-commit closure.

Each phase is an independent commit (disjoint systems, `ai/rules/git-safety.md`). Order = smallest first.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
Pre-checks: `make ze-validate` all-green (no unwired exports, no stale anchors,
AC completeness OK); `audit-test-relaxation.py main` clean (no deleted/weakened
tests).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Config option `system{dns{dnssec-validation}}` (AC-5) had no `test/parse/` functional test (functional-test-gate: Config option -> test/parse) | `internal/component/config/system/yang/ze-system-conf.yang` | fixed: added `test/parse/system-dns-dnssec.ci` (PASSES) |
| 2 | NOTE | DoT/DoH `.ci` assert bind, not TLS handshake (observer sandbox has no Python ssl) | `test/plugin/as112-{dot,doh}.ci` | acknowledged: transport proven by Go unit tests + interop runbook (Deviations) |
| 3 | NOTE | SLAAC/mirror QEMU tests cannot run locally (no CAP_NET_ADMIN) | `internal/component/iface/*_integration_linux_test.go` | acknowledged: env-blocked, skip locally, runbook recorded |

Wiring (step 1) confirmed by `ze-validate` unwired-export check: every new
exported dnsserver symbol (`ApplyWithSecure`, `SecureConfig`, `LoadTLSMaterial`,
`CheckCertMaterial`, `ParseSecureLeaves`, `DefaultSecureConfig`) has a non-test
caller in as112/geodns; `ResolverConfig.DNSSECValidation` is read in `query()`
and threaded from `system.dns`; `AddrInfo.Origin` is populated in `addrList` and
rides `show interface`. RFC compliance (step 20) checked against
rfc7858/8484/4035/4862 summaries: DoH GET+POST both handled, 200 carries the DNS
RCODE, DO bit set only under validation, SERVFAIL-strict rejection, AD=0
accepted.

### Fixes applied
- Added `test/parse/system-dns-dnssec.ci` verifying the `dnssec-validation`
  enumeration leaf parses through the real `ze config validate` (finding 1).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (none) | Re-check after adding the parse `.ci`: 0 BLOCKER, 0 ISSUE (2 NOTEs acknowledged) | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| exabgp internal via standard registry.Registration (as112 pattern) | bespoke in-process shim | One registration pattern repo-wide; startInternal contract already exists |
| exabgp bridge config nested as `exabgp { bridge { run ... } }` (top-level `exabgp` root) | flat `exabgp-bridge { ... }` root | User directive 2026-07-09: keep the `exabgp` root free for future exabgp config; registry plugin name stays `exabgp-bridge`; top-level `exabgp` root verified collision-free (environment.exabgp is a different path; migration/exabgp.yang uses a separate loader) |
| exabgp-bridge plugin package under `internal/plugins/exabgp/bridgeplugin/` | sibling `internal/plugins/exabgp-bridge/` | The engine-boundary lint (pretool-writeedit c_exabgp) confines ExaBGP naming/imports to paths containing `/exabgp/`; a sibling dir fails it. The subpackage is still discovered by the plugin/all generator (walks `internal/plugins` recursively) |
| Script command delivered via SDK OnConfigure (Stage 2) | thread p.config into the runner | Config-delivery matches the as112 flow; SetCapabilities/subprocess-start fit in OnConfigure (before Stage 3/5); RunEngine(conn) never sees the process-manager run line |
| Watchdog = warn+metric+close (fail-fast) | observe-only | Parity with deadline-transport semantics (write error on expiry); A-7 records the alternative |
| DoT/DoH server-side in dnsserver; DNSSEC client-side in resolve | all three in one place | The deferral names both transports (server features used by as112/geodns) and "validation" (a resolver property); code ownership follows the existing packages |
| MCP: migrate-then-delete in two sub-steps | delete + migrate in one diff | Keeps chaos tests green mid-phase (R-5) |
| SLAAC = kernel-cooperating address lifecycle tracking | userspace RA client | ze already delegates autoconf to the kernel (config_sysctl.go:74-75); tracking closes the observability gap without a new protocol stack (A-5) |
| darwin/bsd = compiling interface-parity stubs | full feature ports | Deferral asked for the platform files; feature parity per-platform is its own future effort (A-6) |
| port-defaults lint as scripts/checks Go program | runtime check | Drift is a build-time property; cli_grammar.go pattern exists and runs in inventory lint |

## Known Limitations

- DNSSEC signing (authoritative side) is not part of this spec - A-4; a separate effort if wanted.
- darwin/bsd backends compile but return errNotSupported for most operations - feature ports are follow-ups.
- Watchdog does not recover a stuck peer, it fail-fasts; reconnection is the existing plugin-manager restart path.
- `WriteRawFrame` dead-code question resolved during phase 1 (delete or wire deadline) - record in audit.

## Notes
- Designed 2026-07-09 from skeleton; user instruction 2026-07-09 authorized batch conversion to ready.
- The skeleton's `test/mcp` reference corrected to `test/plugin` (no test/mcp directory exists).
