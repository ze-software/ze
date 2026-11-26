# Spec: dropped-error-8-core-hub-appliance

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 11 errors across the DNS server core, the hub startup capability inventory, the managed client's doctor probe, self-update history persistence and the on-device installer's DHCP fallback are caught and logged only below WARN, leaving a DoH reply write failure uncounted, a self-update history save failure invisible at the default log level, and an installer fallback link-up failure hidden on the serial console |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Doors: `ai/doors/system.md`,
`ai/doors/core.md`, `ai/doors/hub-engine.md`, `ai/doors/appliance.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/core/`, `pkg/zefs/`,
the hub-engine door's paths (`cmd/ze`, `internal/component/{engine,hub,managed}`,
`pkg/fleet`, `pkg/ze`) and the appliance door's paths (`internal/appliance`,
`internal/component/gokrazy`, `internal/install`, `cmd/ze-installer`,
`cmd/ze-serial-shell`, `internal/component/config/system`, `gokrazy`).
At HEAD on 2026-11-26: **11 instances in 9 files** (`cmd/ze/hub` 1,
`internal/component/config/system` 1, `internal/component/managed` 1,
`internal/core/dnsserver` 7 across 5 files, `internal/install/disk` 1), plus 1
companion doctor-code registration file
(`internal/component/config/system/register_doctor_codes.go`).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. Source audit on 2026-11-26 confirmed zero Answer-1 instances remain in this child: `hubReachable` already returns `false` on dial and source-address errors (the flagged line is only `conn.Close()` after a successful TCP connect), and `Manager.bind` already returns the TCP bind error |
| 2 | Going on is right and an operator can act, or the per-event failure needs counting | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`), or count in an existing Prometheus counter on a per-query path where per-event WARN would flood. Under `doctor-coverage`'s `siteRoots` (`internal/component`, `internal/plugins`, `cmd/ze`), every new `Warn`/`Error` log MUST register and emit a doctor `code` in the owning package's `register_doctor_codes.go` (`diagnostic.Register` and, for transient events, `diagnostic.RegisterEventCode`) and raise the transient problem on `internal/core/report` so `ze show doctor` (`recent-events`) reports it and `./le arch doctor-coverage check` passes |
| 3 | Going on is right and nobody can act (teardown close, post-connect probe close, informational boot inventory, or a per-query failure already counted in metrics) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above, naming a visible dependent or guard symbol other than the enclosing function |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: every user-facing message names what, why and the exact fix; `pkg/` contracts and process boundaries do not change
- [ ] `ai/doors/core.md`
  → Constraint: `internal/core` imports no component or plugin; a WARN or metric in `internal/core/dnsserver` uses the logger and `metrics.Registry` the package already takes
- [ ] `ai/doors/hub-engine.md`
  → Constraint: the managed client and hub contract is `pkg/fleet`; `hubReachable`'s close-error annotation changes no envelope and no doctor check result
- [ ] `ai/doors/appliance.md`
  → Constraint: the initrd installer is one static binary with no external tools and links no component; a WARN in `fallbackDHCP` goes to its serial console log, and PID 1 never exits
  → Constraint: an image in the field is the contract; the self-update history key `meta/config/update-history` and JSON payload shape do not change
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every WARN names what went wrong, why it matters and the exact fix; every `Warn`/`Error` log under `internal/component`, `internal/plugins` or `cmd/ze` names a registered doctor `code` and raises transient failures on the report bus
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer" and `docs/contributing/go-style/zero-values.md`
  → Constraint: return, count, or WARN; the guard annotation names a visible symbol that is never the enclosing function
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Event codes"
  → Constraint: `internal/component/config/system/selfupdate.go` sits under `doctor-coverage`'s `siteRoots`, so its new WARN MUST carry a `code` registered in `internal/component/config/system/register_doctor_codes.go` via `diagnostic.Register` and `diagnostic.RegisterEventCode` and raise on `internal/core/report`
- [ ] `docs/architecture/system-architecture.md` - "Capabilities without root"
  → Decision: `logDaemonCapabilities` logs at Debug with no doctor code and no remedy, and `daemon-capabilities` shows the same read failure at `info` severity (`TestLogDaemonCapabilitiesNeverWarns`); elevating it to WARN is forbidden
- [ ] `docs/architecture/dns/server-harness.md` - "The recursion guard is enforced, not offered" and "What one TCP client can hold"
  → Decision: per-query reply write failures count in `ze_dns_reply_write_failure_total` and log at Debug to avoid per-query log floods; TCP cap refusals count in `ze_dns_tcp_connection_refused_total`
- [ ] `docs/architecture/appliance/self-update.md` - "Decisions"
  → Constraint: `selfupdate.go` builds behind `//go:build ze_distro`, so its tests run with `-tags ze_distro`
- [ ] `docs/architecture/appliance/on-device-installer.md` and `docs/architecture/appliance/installer-initrd.md`
  → Constraint: `internal/install/disk` runs in the initrd before Ze is installed, links no component, and is outside `doctor-coverage`'s `siteRoots`

**Key insights:**
- `hubReachable` (`internal/component/managed/doctor.go:539`) already returns `false` on `SetSourceAddress` and `DialContext` failures (proven by `TestHubDoctorCheckUnreachable` and `TestHubReachableProbe`); line 539 is only `conn.Close()` after a successful TCP connect, so it is Answer 3, not Answer 1.
- `Manager.dohHandler` (`internal/core/dnsserver/secure.go:627`) is a real observability gap: unlike `send` (`handler.go:138`) and `sendUnchanged` (`signed.go:150`), it drops `w.Write(out)` errors without calling `recordWriteFailure(transportStream)`.
- `recordsMiss` in `internal/le/arch/refusebydefault/refusebydefault.go` recognizes direct field/element writes and `++`, not helper calls such as `recordWriteFailure(...)`, so the three DNS reply write-failure sites (`handler.go:138`, `signed.go:150`, `secure.go:627`) all carry `// refuse-by-default: guards recordWriteFailure -- ...`.
- `selfupdate.go` is gated on `//go:build ze_distro` and lives under `internal/component/`, so its new `Warn` line requires both `register_doctor_codes.go` registration (`diagnostic.Register` + `diagnostic.RegisterEventCode`) and `report.RaiseErrorFixOnce`, and tests must run with `-tags ze_distro`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `caughtError`, `belowWarn`, `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `parseGuard`, `namesEnclosing`, `addPackageNames`, `annotationFor`
- [ ] `internal/le/arch/refusebydefault/refusebydefault.go` - `recordsMiss`, `mentions`
- [ ] `internal/le/arch/doctorcoverage/doctorcoverage.go` and `sites.go` - `siteRoots` (`internal/component`, `internal/plugins`, `cmd/ze`), `codeAttr`, `StatusCovered`
- [ ] `cmd/ze/hub/main.go` - `logDaemonCapabilities` (line 311): logs `privilege.CheckPrivileges()` read error at Debug and returns; `cmd/ze/hub/capabilities_log_test.go` (`TestLogDaemonCapabilitiesNeverWarns`) enforces that it never logs at Warn or Error
- [ ] `internal/component/config/system/selfupdate.go` - `selfUpdater.saveHistory` (line 1052): logs `statestore.Put(zefs.KeyConfigUpdateHistory.Pattern, data)` error at Debug with no doctor code and no report-bus event
- [ ] `internal/component/config/system/register_doctor_codes.go` - registers `system` doctor codes via `diagnostic.Register(meta)` in `init()`
- [ ] `internal/component/managed/doctor.go` - `hubReachable` (line 539): already returns `false` on `SetSourceAddress` and `DialContext` failures; line 539 logs `conn.Close()` failure at Debug after TCP connect succeeded and returns `true`
- [ ] `internal/core/dnsserver/connlimit.go` - `connLimitListener.Accept` (line 71): increments `hmetrics().tcpConnRefused.Inc()` on line 70, then logs `c.Close()` error on the shed connection at Debug
- [ ] `internal/core/dnsserver/handler.go` - `send` (line 138): calls `recordWriteFailure(transport)` on line 139, then logs `w.WriteMsg(msg)` error at Debug
- [ ] `internal/core/dnsserver/manager.go` - `Manager.bind` (line 237): logs UDP `pc.Close()` error at Debug after TCP listen fails, then returns `fmt.Errorf("tcp: %w", err)`; `Manager.Stop` (lines 343, 350): logs `dns.Server.ShutdownContext` and `http.Server.Shutdown` teardown errors at Debug while draining all listeners
- [ ] `internal/core/dnsserver/secure.go` - `Manager.dohHandler.func` (line 627): logs `w.Write(out)` failure at Debug and never increments `ze_dns_reply_write_failure_total`
- [ ] `internal/core/dnsserver/signed.go` - `sendUnchanged` (line 150): calls `recordWriteFailure(transportOf(w))` on line 151, then logs `w.Write(reply)` error at Debug
- [ ] `internal/install/disk/network.go` - `fallbackDHCP` (line 106): loops over `/sys/class/net` skipping `lo` and logs `linkUp(name)` errors at Debug (unlike pinned-NIC line 58 which logs at Warn)

**Per-instance classification (all 11 instances at HEAD on 2026-11-26):**

| # | File and line | Symbol | Resolution | Planned change |
|---|---------------|--------|------------|----------------|
| 1 | `cmd/ze/hub/main.go:311` | `logDaemonCapabilities` | Answer 3 | Add `// refuse-by-default: guards privilege.CheckPrivileges -- capability inventory at boot is informational before config loads; each configured feature checks its own capability and doctor daemon-capabilities reports the same read failure at info severity` |
| 2 | `internal/component/config/system/selfupdate.go:1052` | `selfUpdater.saveHistory` | Answer 2 | Register `codeUpdateHistoryPersistFailed` (`doctor-update-history-persist-failed`) in `register_doctor_codes.go` via `diagnostic.Register` and `diagnostic.RegisterEventCode`; log at `Warn` with what, why, fix (`ze show data check`) and `"code", codeUpdateHistoryPersistFailed`; raise transient event via `report.RaiseErrorFixOnce` (without logging URLs or credentials) |
| 3 | `internal/component/managed/doctor.go:539` | `hubReachable` | Answer 3 | Add `// refuse-by-default: guards diagnoseHubReachability -- TCP connect already proved the hub reachable; closing the throwaway probe socket cannot change reachability and nobody can act on a close error` |
| 4 | `internal/core/dnsserver/connlimit.go:71` | `connLimitListener.Accept` | Answer 3 | Add `// refuse-by-default: guards hmetrics -- connection-cap refusal is already counted in tcpConnRefused; closing the shed socket cannot be retried and a peer reset on close is not actionable` |
| 5 | `internal/core/dnsserver/handler.go:138` | `send` | Answer 3 | Add `// refuse-by-default: guards recordWriteFailure -- per-query reply write failures are counted in ze_dns_reply_write_failure_total; logging WARN per dropped query would flood the log` |
| 6 | `internal/core/dnsserver/manager.go:237` | `Manager.bind` | Answer 3 | Add `// refuse-by-default: guards bindListeners -- the TCP bind error is returned to the caller; closing the already-bound UDP socket is teardown cleanup nobody can act on` |
| 7 | `internal/core/dnsserver/manager.go:343` | `Manager.Stop` | Answer 3 | Add `// refuse-by-default: guards reconcile -- listener teardown continues across remaining servers after generation++ and any port left bound is reported by the next bindListeners` |
| 8 | `internal/core/dnsserver/manager.go:350` | `Manager.Stop` | Answer 3 | Add `// refuse-by-default: guards reconcile -- DoH listener teardown continues across remaining servers after generation++ and any port left bound is reported by the next bindListeners` |
| 9 | `internal/core/dnsserver/secure.go:627` | `Manager.dohHandler.func` | Answer 2 (count) + Answer 3 (guard) | Call `recordWriteFailure(transportStream)` in the `w.Write(out)` error branch and add `// refuse-by-default: guards recordWriteFailure -- per-query DoH reply write failures are counted in ze_dns_reply_write_failure_total; logging WARN per dropped query would flood the log` |
| 10 | `internal/core/dnsserver/signed.go:150` | `sendUnchanged` | Answer 3 | Add `// refuse-by-default: guards recordWriteFailure -- per-query pass-through reply write failures are counted in ze_dns_reply_write_failure_total; logging WARN per dropped query would flood the log` |
| 11 | `internal/install/disk/network.go:106` | `fallbackDHCP` | Answer 2 | Raise `slog.Debug` to `slog.Warn` with what, why and fix on the initrd console while continuing the loop over remaining NICs; extract `fallbackDHCPIn(sysnetDir string)` so unit tests cover it without real `/sys/class/net` or netlink |

**Behavior to preserve:**
- DNS wire behavior across UDP, TCP, DoT, DoH and TSIG/TKEY pass-through; the `pkg/fleet` contract; `hubReachable`'s reachability boolean and doctor output; `logDaemonCapabilities` staying below WARN (`TestLogDaemonCapabilitiesNeverWarns`); the installer's PID 1 non-exit behavior; and the self-update history key (`meta/config/update-history`) and JSON format.

**Behavior to change:**
- `selfUpdater.saveHistory` warns at `Warn` with `code` `doctor-update-history-persist-failed` (registered in `internal/component/config/system/register_doctor_codes.go`), names what/why/fix, and raises a bounded transient event on `internal/core/report` when `statestore.Put` fails.
- `Manager.dohHandler` counts failed HTTP response writes in the existing `ze_dns_reply_write_failure_total{transport="stream"}` counter via `recordWriteFailure(transportStream)`.
- `fallbackDHCP` logs `linkUp` failures at `Warn` with what, why and fix on the initrd console while continuing across remaining NICs.
- All 9 justified below-WARN branches carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists the 11 unannotated findings under this child's paths | Applies unchanged: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` `parseGuard`, `namesEnclosing` | Accepts `refuse-by-default: guards <Symbol> -- <why>` naming a visible dependent symbol other than the enclosing function | Applies unchanged: Answer 3 for the 9 justified below-WARN branches |
| Doctor-coverage gate and event-code registry | `internal/le/arch/doctorcoverage/doctorcoverage.go` `siteRoots`; `internal/component/config/system/register_doctor_codes.go` `init`; `internal/core/report` `RaiseErrorFixOnce` | Refuses any added `Warn`/`Error` under `internal/component`, `internal/plugins`, or `cmd/ze` whose `code` attribute is not declared by a doctor check; `recent-events` renders registered `EventCode` raises in `show doctor` | Extends: registers `doctor-update-history-persist-failed` in `internal/component/config/system/register_doctor_codes.go` (`diagnostic.Register` + `diagnostic.RegisterEventCode`) and raises it from `selfUpdater.saveHistory` |
| Managed hub reachability doctor check | `internal/component/managed/doctor.go` `hubDoctorCheck`, `checkHubReachable`, `diagnoseHubReachability`, `hubReachable`; `doctor_test.go` `TestHubDoctorCheckUnreachable`, `TestHubReachableProbe` | Already returns `false` on `SetSourceAddress` and `DialContext` failures and reports `doctor-hub-unreachable` with fix in `ze doctor` | Retired from old spec's Answer-1 claim: unreachable-hub reporting already exists and is tested; line 539 is only `conn.Close()` after a successful TCP connect and gets Answer 3 (`guards diagnoseHubReachability`) |
| Boot capability inventory log and doctor check | `cmd/ze/hub/main.go` `logDaemonCapabilities`; `cmd/ze/hub/capabilities_log_test.go` `TestLogDaemonCapabilitiesNeverWarns`; `internal/component/doctor/checks_daemon_capabilities.go` `daemonCapabilitiesCheck` | Logs capability read failures at Debug before config loads and reports them at `SeverityInfo` in `show doctor` under `doctor-daemon-capability-missing` | Preserves unchanged with Answer 3 (`guards privilege.CheckPrivileges`): elevating to WARN would violate `system-architecture.md` and break `TestLogDaemonCapabilitiesNeverWarns` |
| DNS server harness metrics | `internal/core/dnsserver/metrics.go` `recordWriteFailure`, `ze_dns_reply_write_failure_total`, `ze_dns_tcp_connection_refused_total` | Counts UDP/TCP/DoT reply write failures (by `transport`: `datagram` or `stream`) and TCP cap refusals | Extends: `Manager.dohHandler` calls `recordWriteFailure(transportStream)` so DoH write failures are counted under the existing `stream` label |
| Self-update backend | `internal/component/config/system/selfupdate.go` `selfUpdater.saveHistory` (`docs/architecture/appliance/self-update.md`) | Persists the 20-event update history circular buffer to `meta/config/update-history` via `statestore.Put` | Extends: surfaces a `statestore.Put` error at `Warn` with doctor code and report-bus raise instead of hiding it at `Debug` |
| Initrd installer network fallback | `internal/install/disk/network.go` `ensureNetwork`, `fallbackDHCP`, `ifaceForMAC`, `linkUp` (`docs/architecture/appliance/on-device-installer.md`, `installer-initrd.md`) | Brings up all non-loopback NICs via netlink and acquires DHCP when the install server is unreachable; pinned-NIC `linkUp` already logs at `Warn` | Extends: brings `fallbackDHCP` `linkUp` failure to `Warn` with what/why/fix on the serial console and adds a `sysnetDir` test seam (`fallbackDHCPIn`) matching `ifaceForMAC` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every newly raised WARN (`selfUpdater.saveHistory`, `fallbackDHCP`) states what failed, why it matters and the exact operator fix, and no `pkg/` contract or plugin boundary changes |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/dnsserver/{connlimit,handler,manager,secure,signed}.go` | "It imports neither `internal/component/` nor `internal/plugins/`." | agrees | All `dnsserver` changes stay inside `internal/core/dnsserver`, reusing `recordWriteFailure` and guard annotations with zero upward imports |
| `ai/doors/hub-engine.md` | Where it sits | the door covering `cmd/ze/hub/main.go` and `internal/component/managed/doctor.go` | "Managed configuration has two sides in this tree." | agrees | Neither the `pkg/fleet` contract nor hub startup/reload flow changes; both sites receive Answer-3 guard annotations only |
| `ai/doors/appliance.md` | Who depends on it from outside | the door covering `internal/component/config/system/{selfupdate,register_doctor_codes}.go` and `internal/install/disk/network.go` | "An image already in the field is the contract." | agrees | `meta/config/update-history` key, JSON format, and initrd PID 1 non-exit behavior are preserved; `fallbackDHCP` logs WARN to console without exiting PID 1 or invoking external tools |
| `ai/rules/repo-maintenance.md` | Directives | the `// Design:` page of `internal/component/managed/doctor.go` | "A private change owes none of that only when it changes no user or agent behavior, breaks no documented contract or invariant, makes no page stale, and sets no pattern later work follows." | agrees | The `managed/doctor.go` edit adds only a guard annotation on `conn.Close()` after a successful probe; `selfupdate.go` registers its new doctor code in `register_doctor_codes.go` |
| `docs/architecture/hub-architecture.md` | Startup Sequence | the `// Design:` page of and anchored to `cmd/ze/hub/main.go` | "The hub constructs the plugin server and binds its complete startup as the plugin manager's runner before Engine.Start." | agrees | `cmd/ze/hub/main.go` changes only a comment annotation above `if err != nil` in `logDaemonCapabilities`, leaving hub startup and reload sequences untouched |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | the `// Design:` page of `internal/component/config/system/register_doctor_codes.go`; anchored to `cmd/ze/hub/main.go` and `internal/component/managed/doctor.go` | "A site is covered when its code is one that a doctor check declares." | agrees | The only new `Warn` under `siteRoots` (`selfUpdater.saveHistory`) emits `doctor-update-history-persist-failed`, registered in `register_doctor_codes.go` via `diagnostic.Register` and `diagnostic.RegisterEventCode` (declared by `recent-events`) and raised on `internal/core/report` |
| `docs/architecture/system-architecture.md` | Capabilities without root | anchored to `cmd/ze/hub/main.go` | "The hub logs it at debug at start (`logDaemonCapabilities`), with no doctor code and no remedy, and the `daemon-capabilities` runtime check shows it in `show doctor` as an `info` row under `doctor-daemon-capability-missing` that never makes the box not ready" | agrees | `logDaemonCapabilities` keeps logging at Debug with no doctor code and no remedy, marked with `// refuse-by-default: guards privilege.CheckPrivileges` |
| `docs/architecture/appliance/self-update.md` | Decisions | the `// Design:` page of and anchored to `internal/component/config/system/selfupdate.go` | "History is a circular buffer of 20 events, published through the daemon's owning store under `meta/config/update-history`." | agrees | History buffer size, key, and best-effort continuation are preserved; only a `statestore.Put` error is elevated from Debug to Warn + doctor code + report-bus raise |
| `docs/architecture/dns/server-harness.md` | The recursion guard is enforced, not offered | the `// Design:` page of and anchored to `internal/core/dnsserver/{connlimit,handler,manager,secure,signed}.go` | "Owning the write also means owning its failure. `send` counts a refused write in `ze_dns_reply_write_failure_total`, labelled by transport, and logs one line at Debug." | agrees | `send` and `sendUnchanged` retain their Debug log and counter with guard annotations, and `Manager.dohHandler` joins them by calling `recordWriteFailure(transportStream)` |
| `docs/architecture/dns/secure-transports.md` | One answer policy across four transports | the `// Design:` page of `internal/core/dnsserver/secure.go`; anchored to `internal/core/dnsserver/{manager,secure}.go` | "With one handler behind all four transports, the `allow-from` decision, the client-IP selection and the authoritative and recursion guard are identical on cleartext UDP, cleartext TCP, DoT and DoH." | agrees | Counting DoH response write failures in `ze_dns_reply_write_failure_total{transport="stream"}` brings DoH into parity with cleartext TCP and DoT without changing HTTP status codes or answer policy |
| `docs/architecture/appliance/on-device-installer.md` | Decisions | the `// Design:` page of and anchored to `internal/install/disk/network.go` | "The installer registers as `ze install disk` under the existing subdispatch, not as a new binary." | agrees | `fallbackDHCP` stays inside `internal/install/disk`, elevating `linkUp` failure to `slog.Warn` on the serial console while continuing across remaining NICs |
| `docs/architecture/appliance/installer-initrd.md` | PID 1 cannot exit | anchored to `internal/install/disk/network.go` | "PID 1 exiting panics the kernel, so a fatal error routes to a recovery console instead." | agrees | Raising `linkUp` failure to `slog.Warn` in `fallbackDHCP` preserves non-fatal iteration across remaining NICs and never exits PID 1 |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Warnings are state-based (condition is currently problematic, may resolve). Errors are event-based (something already happened; no clear API). Producers pick deliberately; the bus does not auto-promote." | agrees | `selfUpdater.saveHistory` raises a failed persistence write as an event-based error via `report.RaiseErrorFixOnce`, respects every field bound (`Source`/`Code` <= 64 bytes, `Subject` <= 256, `Message` fitted via `report.FitMessage` <= 1024, `Fix` <= 256), and uses the existing bus unchanged |
| `docs/architecture/core-design.md` | 20. System Update Backend | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "The `ze-self-update` backend delegates to the existing passive `UpdateChecker` when only version checking is configured, and to `selfUpdater` when auto-apply or restart policy is configured. This preserves the existing download, verification, staging, restart, and history code path." | agrees | `selfUpdater.saveHistory` preserves the existing history key (`meta/config/update-history`), 20-event circular buffer, and continuation path, surfacing only `statestore.Put` failures at `Warn` with a doctor code and report-bus event |
| `docs/architecture/fleet-config.md` | Vision | anchored to `cmd/ze/hub/main.go` and `internal/component/managed/doctor.go` | "" | silent | Fleet config protocol and hub block layout are unaffected by the `conn.Close()` guard annotation in `hubReachable` and the `logDaemonCapabilities` guard annotation; the 2 design decisions on this page are judged individually below |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | anchored to `cmd/ze/hub/main.go` and `internal/core/dnsserver/{manager,secure}.go` | "" | silent | TLS certificate selection and fail-closed precedence are untouched by DoH response write counting and listener teardown annotations; the 3 design decisions on this page are judged individually below |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `cmd/ze/hub/main.go` | "" | silent | Pipe/bus architecture is unrelated to `logDaemonCapabilities` in `cmd/ze/hub/main.go`; the 8 design decisions on this page are judged individually below |
| `docs/architecture/api/commands.md` | Operational Report Bus (ze-cmd:show-warnings, ze-cmd:show-errors) | anchored to `cmd/ze/hub/main.go` | "Producers MUST pick the right severity. The bus does not auto-promote." | agrees | `selfUpdater.saveHistory` selects event-based error severity deliberately (`report.RaiseErrorFixOnce` carrying `Issue.Fix` = `ze show data check`), keeps every field within the bus bounds (`Source`/`Code` <= 64, `Subject` <= 256, `Message` fitted via `report.FitMessage` <= 1024, `Fix` <= 256), and leaves `ze-cmd:show-errors` / `ze-cmd:show-warnings` RPC shapes unchanged |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `cmd/ze/hub/main.go` | "" | silent | Plugin wire protocol is unrelated to `logDaemonCapabilities` in `cmd/ze/hub/main.go`; the 2 design decisions on this page are judged individually below |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | "" | silent | SSH configuration upload and remote operations are unrelated to the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `cmd/ze/hub/main.go` | "" | silent | Signal handling (SIGTERM/SIGHUP/SIGUSR1) is untouched by the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `cmd/ze/hub/main.go` | "" | silent | CLI Tab/`?` completion producers are untouched by the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/config/syntax.md` | Overview | anchored to `cmd/ze/hub/main.go` | "" | silent | Configuration syntax is unchanged by this spec |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `cmd/ze/hub/main.go` | "" | silent | YANG schema design and validation are unchanged by this spec |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go` | "" | silent | Panic and crash capture are untouched by the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | "" | silent | PKI store layout and key storage are untouched by the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to `cmd/ze/hub/main.go` | "" | silent | Store opening and ownership semantics are unchanged; `selfUpdater.saveHistory` continues to use `statestore.Put` and only surfaces its error at `Warn` |
| `docs/architecture/testing/ci-format.md` | Key Concepts | anchored to `cmd/ze/hub/main.go` | "" | silent | The `.ci` functional test harness format is unchanged by this spec |
| `docs/architecture/web-interface.md` | Component Structure | anchored to `cmd/ze/hub/main.go` | "" | silent | The web UI is untouched by the `logDaemonCapabilities` comment annotation in `cmd/ze/hub/main.go` |
| `docs/architecture/dns/as112.md` | The response code says where the query name sits | anchored to `internal/core/dnsserver/handler.go` | "" | silent | AS112 zone and response-code policy is unchanged by adding a guard annotation on `send`'s existing write-failure log line |
| `docs/architecture/dns/lan-resolver.md` | Request path | anchored to `internal/core/dnsserver/handler.go` | "" | silent | LAN resolver forwarding and caching policy is unchanged by adding a guard annotation on `send`'s existing write-failure log line |
| `docs/architecture/dns/geodns.md` | Server | anchored to `internal/core/dnsserver/manager.go` | "" | silent | GeoDNS snapshot and zone-matching behavior is unchanged by adding guard annotations on `Manager.bind` and `Manager.Stop` |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not touch config rendering or `\| implied` output |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec does not alter bus participant naming or registration |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec does not alter bus `send > to` routing |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec does not alter bus `receive > from` filtering |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec does not alter bus allow-list enforcement |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec does not alter participant filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec adds no role arbitration on the bus |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec does not change config or bus display surfaces |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds or modifies no CLI command paths or pipes |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec does not touch pipe operator overrides or command output shapes |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec does not alter any pipe operation or command behavior |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec does not modify engine operations or plugin identity authorization |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec does not touch engine-to-plugin event queues or overflow handling |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec does not touch VPP or kernel data-plane feature validation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not touch data-plane mismatch checks at start, commit, reload or validate |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch firewall or VPP verifiers |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces or modifies no data-plane selectors or backend names |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not touch BGP next-hop rewriting or export filter chains |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch `policychain` or BGP export next-hop syntax |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch BGP filter chain compilation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not touch BGP UPDATE forwarding or `wireOverride` |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not touch BGP import or export next-hop policy |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not touch BGP route origination or forwarding next-hop rules |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not touch implied filter publication or display |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Consistent: none of the 11 changes causes `ze start` to exit; `logDaemonCapabilities` keeps starting non-fatally and `selfUpdater.saveHistory` logs under a registered doctor code and raises on the report bus while continuing |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/config/system/selfupdate.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not touch kernel/VPP mismatch handling at start, commit, reload or validate |
| `docs/architecture/fleet-config.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/managed/doctor.go` | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | This spec does not touch managed listener certificate reload |
| `docs/architecture/fleet-config.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/component/managed/doctor.go` | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | This spec does not touch fleet web device configuration editing |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/core/dnsserver/{manager,secure}.go` | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | This spec does not touch looking-glass HTTP/HTTPS listener policy |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/core/dnsserver/{manager,secure}.go` | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | This spec does not touch `listenerTLSMaterial` or `managedCertificate` selection |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` and `internal/core/dnsserver/{manager,secure}.go` | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | This spec does not touch TLS listener startup or reload refusal policy |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed store persistence write in `selfUpdater.saveHistory`, a failed DoH HTTP response write in `Manager.dohHandler`, a failed netlink `linkUp` in `fallbackDHCP`, or one of the 9 justified below-WARN teardown/probe/inventory/counted-write error branches.

### Transformation Path
1. The error branch catches the non-nil error.
2. In `selfUpdater.saveHistory`: logs `Warn` with `code` `doctor-update-history-persist-failed`, what, why and `ze show data check`, and raises a bounded transient event via `report.RaiseErrorFixOnce` so `ze show doctor` (`recent-events`) and `ze show errors` surface it.
3. In `Manager.dohHandler`: increments the existing `ze_dns_reply_write_failure_total{transport="stream"}` counter via `recordWriteFailure(transportStream)` alongside its Debug log and guard annotation.
4. In `fallbackDHCP`: logs `slog.Warn` to the initrd serial console with what, why and the `ze.mac=<mac>` / NIC driver fix, then continues bringing up remaining NICs without exiting PID 1.
5. In the remaining 8 justified branches (`logDaemonCapabilities`, `hubReachable`, `connLimitListener.Accept`, `send`, `Manager.bind`, `Manager.Stop` x2, `sendUnchanged`): keeps the existing behavior and documents its guard with `// refuse-by-default: guards <Symbol> -- <why>`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ operator | `Warn` log with registered doctor `code`, `internal/core/report` transient event (`show doctor`, `show errors`), and Prometheus counter `ze_dns_reply_write_failure_total{transport="stream"}` | No |
| Initrd PID 1 ↔ operator | `slog.Warn` on the serial console | No |

### Changed-Wire Census

No wire format, storage key schema, RPC envelope, or external API payload changes in this spec. Source verification across all 10 files in `Files to Modify`: `internal/core/dnsserver/{connlimit,handler,manager,secure,signed}.go` leave DNS wire packing, HTTP status/headers, and `ze_dns_reply_write_failure_total` metric/label names untouched (`recordWriteFailure(transportStream)` reuses the existing `stream` label); `internal/component/config/system/selfupdate.go` keeps `zefs.KeyConfigUpdateHistory.Pattern` and `UpdateEvent` JSON encoding unchanged and raises into the existing in-process `internal/core/report` bus (`report.Issue` / `ze-cmd:show-errors` wire shape unchanged, `Message` fitted via `report.FitMessage`); `internal/component/managed/doctor.go` keeps `pkg/fleet` envelopes and `hubDoctorCheck` diagnostics unchanged; `cmd/ze/hub/main.go` adds only a comment; and `internal/install/disk/network.go` changes only a console log level and adds a test seam parameter.

### Integration Points
- `internal/component/config/system/register_doctor_codes.go` (`diagnostic.Register`, `diagnostic.RegisterEventCode`) and `internal/core/report` (`RaiseErrorFixOnce`) for `selfUpdater.saveHistory`.
- `internal/core/dnsserver/metrics.go` (`recordWriteFailure(transportStream)`) for `Manager.dohHandler`.
- `internal/install/disk/network.go` (`slog.Warn`, `fallbackDHCPIn`) for initrd DHCP fallback.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Uses existing `slogutil.Logger`, `diagnostic.Register`/`RegisterEventCode`, `report.RaiseErrorFixOnce`, `dnsserver.recordWriteFailure`, and `slog.Warn` |
| No unintended coupling (components stay isolated) | Yes | Zero new cross-tier imports; `internal/core/dnsserver` stays a leaf and `internal/install/disk` links no component |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `ze_dns_reply_write_failure_total{transport="stream"}`, `register_doctor_codes.go`, and `report.RaiseErrorFixOnce` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Error and control paths only; DoH write-failure path increments an atomic counter only on error |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | New doctor code `doctor-update-history-persist-failed` registers in `internal/component/config/system/register_doctor_codes.go` via `diagnostic.Register` and `diagnostic.RegisterEventCode` and is discovered by `recent-events` automatically |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/doctor/checks_recent_events.go` and `internal/le/arch/doctorcoverage/codes.go`; both derive covered event codes dynamically from `diagnostic.EventCode` registrations with no hardcoded list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 11 findings across 9 files are the current population under these three doors | `./le arch refuse-by-default report` run on 2026-11-26 | Work from the fresh report before the first edit | Re-run `./le arch refuse-by-default report '|' json` filtered to this child's paths at wiring | unvalidated |
| A-2 | `recordWriteFailure(transportStream)` is safe to call from `Manager.dohHandler` without adding a new label or changing metric cardinality | `internal/core/dnsserver/metrics.go` (`transportStream = "stream"`, `hmetrics()` lazy Nop default) | Would register an unexpected label series | `TestDoHHandlerWriteFailureCountsMetric` asserting `ze_dns_reply_write_failure_total{transport="stream"}` | unvalidated |
| A-3 | Registering `codeUpdateHistoryPersistFailed` via `diagnostic.Register` + `diagnostic.RegisterEventCode` in `internal/component/config/system/register_doctor_codes.go` satisfies `./le arch doctor-coverage check` | `internal/le/arch/doctorcoverage/codes.go` and `internal/component/managed/register_doctor_codes.go` (`CodeDoctorManagedReportFailed`) | Commit gate refuses the new `Warn` site in `selfupdate.go` | `./le arch doctor-coverage check` and `TestSelfUpdateHistoryDoctorCodeRegistered` | unvalidated |
| A-4 | `internal/install/disk/network.go` is outside `doctor-coverage`'s `siteRoots` so its console `slog.Warn` requires no doctor-code registration | `internal/le/arch/doctorcoverage/doctorcoverage.go:59` (`siteRoots = []string{"internal/component", "internal/plugins", "cmd/ze"}`) | Commit gate refuses `network.go` | `./le arch doctor-coverage check` after editing `network.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per DNS query under a client that drops connections floods the daemon log | DNS `.ci` or unit test log grows per dropped query | Never raise `send`, `sendUnchanged`, `connLimitListener.Accept`, or `dohHandler` to WARN; count via existing Prometheus metrics (`ze_dns_reply_write_failure_total`, `ze_dns_tcp_connection_refused_total`) and annotate with `guards` |
| R-2 | `selfUpdater.saveHistory` runs behind `//go:build ze_distro`, so an untagged `go test` skips it | `go test ./internal/component/config/system/` reports no run of `TestSelfUpdate*` | Run package tests with `-tags ze_distro` |
| R-3 | `selfUpdater.saveHistory` fails repeatedly and floods `report` error ring | Multiple events within one minute | Use `report.RaiseErrorFixOnce` with 1-minute deduplication window, matching `cli/history_diagnostic.go` |
| R-4 | A guard annotation names the enclosing function or an invisible symbol | `./le arch refuse-by-default check` reports `annotation-names-no-guard` | Every guard symbol is verified in the file or package siblings (`privilege.CheckPrivileges`, `diagnoseHubReachability`, `hmetrics`, `recordWriteFailure`, `bindListeners`, `reconcile`) and never matches the enclosing function |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise on self-update store errors or installer fallback NIC bring-up, or a commit-gate refusal from `doctor-coverage` or `refuse-by-default` |
| How is it reverted? | Single commit revert |
| Who else touches this path? | DNS, hub startup, self-update, and appliance installer specs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` over this child's door paths | → | `droppedError` + `annotations` across all 9 source files | `TestRefuseByDefaultRefusesEachRuleFixture` and `TestRefuseByDefaultReportListsTree` (`internal/le/arch/refusebydefault/refusebydefault_test.go`) plus filtered `./le arch refuse-by-default report` |
| `selfUpdater.recordEvent` when `statestore.Put` fails | → | `selfUpdater.saveHistory` (`internal/component/config/system/selfupdate.go`) + `register_doctor_codes.go` | `TestSelfUpdateSaveHistoryFailureWarnsAndRaisesEvent` (`internal/component/config/system/selfupdate_test.go`) |
| DoH HTTP request when `http.ResponseWriter.Write` fails | → | `Manager.dohHandler` → `recordWriteFailure(transportStream)` (`internal/core/dnsserver/secure.go`) | `TestDoHHandlerWriteFailureCountsMetric` (`internal/core/dnsserver/secure_test.go`) |
| `ensureNetwork` → `fallbackDHCP` when `linkUp` fails on one NIC | → | `fallbackDHCPIn` (`internal/install/disk/network.go`) | `TestFallbackDHCPWarnsOnLinkUpFailure` (`internal/install/disk/network_test.go`) |
| `./le arch doctor-coverage check` over changed files | → | `internal/component/config/system/register_doctor_codes.go` + `selfupdate.go` | `TestSelfUpdateHistoryDoctorCodeRegistered` (`internal/component/config/system/selfupdate_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's door paths (`internal/core/`, `pkg/zefs/`, `cmd/ze/`, `internal/component/{engine,hub,managed}/`, `pkg/fleet/`, `pkg/ze/`, `internal/appliance/`, `internal/component/gokrazy/`, `internal/install/`, `cmd/ze-installer/`, `cmd/ze-serial-shell/`, `internal/component/config/system/`, `gokrazy/`) | Report returns a valid JSON payload whose `findings` list has count 0 for rule `error-dropped-below-warn` under those paths and whose `annotated` list carries all 9 annotated sites with non-empty `guard` fields | The 9 annotated sites (`cmd/ze/hub/main.go:311`, `managed/doctor.go:539`, `dnsserver/connlimit.go:71`, `dnsserver/handler.go:138`, `dnsserver/manager.go:237,343,350`, `dnsserver/secure.go:627`, `dnsserver/signed.go:150`) remain in `annotated` because each names a verified guard for a justified below-WARN branch |
| AC-2 | `selfUpdater.recordEvent` runs while `statestore.Put` returns an error (behind `-tags ze_distro`) | `selfUpdater.saveHistory` emits one `Warn` log entry carrying `code=doctor-update-history-persist-failed`, what failed, why it matters, and fix `ze show data check`, and raises one bounded transient event under `doctor-update-history-persist-failed` on `internal/core/report` while keeping the event in memory; `TestSelfUpdateHistoryDoctorCodeRegistered` proves `diagnostic.Lookup("doctor-update-history-persist-failed")` and `diagnostic.EventCodes()` both contain the code, and `./le arch doctor-coverage check` exits 0 | Repeated persistence failures inside the 1-minute `RaiseErrorFixOnce` window suppress duplicate report-bus events by design to avoid flooding `show errors` |
| AC-3 | A valid DoH request hits `Manager.dohHandler` and `http.ResponseWriter.Write` returns an error | `Manager.dohHandler` increments `ze_dns_reply_write_failure_total{transport="stream"}` by 1 and keeps the handler returning cleanly without panicking or emitting per-query `Warn` logs | The Debug log line remains for diagnostic verbosity, guarded by `recordWriteFailure` |
| AC-4 | `fallbackDHCPIn` iterates a sysfs directory containing a failing NIC and a healthy NIC while `linkUp` returns an error for the failing NIC | `fallbackDHCPIn` emits one `Warn` log entry naming the interface, what failed, why it matters, and the fix (`ze.mac=<mac>` / NIC driver check), continues bringing up the remaining NIC, and acquires DHCP without exiting PID 1 | Unplugged or failing secondary NICs still proceed through the loop so a working install NIC can obtain a lease |
| AC-5 | Every guard annotation added by this spec | `./le arch refuse-by-default check` exits 0 with zero `annotation-names-no-guard` findings, and existing behavioral tests (`TestLogDaemonCapabilitiesNeverWarns`, `TestHubDoctorCheckUnreachable`, `TestHubReachableProbe`, `TestManager_TCPConnMaxRefusesPastTheCap`) pass | None: every guard annotation must resolve to a real non-enclosing symbol |
| AC-6 | Scoped package tests and functional suites reached by `./le repo changed scope print both` run by the main thread in the main checkout | All scoped package tests (`./internal/core/dnsserver/`, `./cmd/ze/hub/`, `./internal/component/managed/`, `-tags ze_distro ./internal/component/config/system/`, `./le test unit installer`), `./le arch doctor-coverage check`, `./le arch tier check`, and reached functional suites pass | None: all tests reached by this change set must pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSelfUpdateSaveHistoryFailureWarnsAndRaisesEvent` | `internal/component/config/system/selfupdate_test.go` | AC-2: `saveHistory` logs `Warn` with `code`, what/why/fix, raises `doctor-update-history-persist-failed` on `report`, and never leaks update URL/credentials | |
| `TestSelfUpdateHistoryDoctorCodeRegistered` | `internal/component/config/system/selfupdate_test.go` | AC-2: `doctor-update-history-persist-failed` is registered in `diagnostic` metadata and `diagnostic.EventCodes()` | |
| `TestDoHHandlerWriteFailureCountsMetric` | `internal/core/dnsserver/secure_test.go` | AC-3: failing `http.ResponseWriter.Write` in `Manager.dohHandler` increments `ze_dns_reply_write_failure_total{transport="stream"}` by 1 | |
| `TestFallbackDHCPWarnsOnLinkUpFailure` | `internal/install/disk/network_test.go` | AC-4: `fallbackDHCPIn` logs `Warn` with interface, what/why/fix when `linkUp` fails on one NIC and continues to lease the healthy NIC | |
| Existing regression guards (`TestLogDaemonCapabilitiesNeverWarns`, `TestHubDoctorCheckUnreachable`, `TestHubReachableProbe`, `TestManager_TCPConnMaxRefusesPastTheCap`) | `cmd/ze/hub/capabilities_log_test.go`, `internal/component/managed/doctor_test.go`, `internal/core/dnsserver/connlimit_test.go` | AC-5: existing capability-inventory, unreachable-hub doctor, and TCP-cap contracts stay green | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated `error-dropped-below-warn` findings under this child's paths | 0 | 0 | N/A | 1 |
| `ze_dns_reply_write_failure_total{transport="stream"}` increment per failed DoH write | 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Scoped `.ci` suites selected by `./le repo changed scope print both` (DNS, managed, self-update history restart, installer) | `test/` | Unchanged DNS, managed doctor, and installer behavior; self-update history restart remains green | |

## Files to Modify
- `cmd/ze/hub/main.go` - 1: `logDaemonCapabilities` 311 (Answer 3 guard annotation `guards privilege.CheckPrivileges`)
- `internal/component/config/system/selfupdate.go` - 1: `selfUpdater.saveHistory` 1052 (Answer 2: `Warn` with `code`, what/why/fix + `report.RaiseErrorFixOnce`)
- `internal/component/config/system/register_doctor_codes.go` - register `codeUpdateHistoryPersistFailed` (`doctor-update-history-persist-failed`) via `diagnostic.Register` and `diagnostic.RegisterEventCode`
- `internal/component/managed/doctor.go` - 1: `hubReachable` 539 (Answer 3 guard annotation `guards diagnoseHubReachability`)
- `internal/core/dnsserver/connlimit.go` - 1: `connLimitListener.Accept` 71 (Answer 3 guard annotation `guards hmetrics`)
- `internal/core/dnsserver/handler.go` - 1: `send` 138 (Answer 3 guard annotation `guards recordWriteFailure`)
- `internal/core/dnsserver/manager.go` - 3: `Manager.bind` 237 (`guards bindListeners`); `Manager.Stop` 343, 350 (`guards reconcile`)
- `internal/core/dnsserver/secure.go` - 1: `Manager.dohHandler.func` 627 (Answer 2 count via `recordWriteFailure(transportStream)` + Answer 3 guard annotation `guards recordWriteFailure`)
- `internal/core/dnsserver/signed.go` - 1: `sendUnchanged` 150 (Answer 3 guard annotation `guards recordWriteFailure`)
- `internal/install/disk/network.go` - 1: `fallbackDHCP` 106 (Answer 2: `slog.Warn` with what/why/fix + `fallbackDHCPIn(sysnetDir string)` test seam)

Lines are those of HEAD on 2026-11-26; `./le arch refuse-by-default report` is the current list.

## Files to Create
- No new source file; unit tests added to existing `*_test.go` files (`internal/component/config/system/selfupdate_test.go`, `internal/core/dnsserver/secure_test.go`, `internal/install/disk/network_test.go`).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency (`hubReachable` already reports unreachable hubs via `hubDoctorCheck`) |
| Doctor code per problem class | Yes | `doctor-update-history-persist-failed` registered in `internal/component/config/system/register_doctor_codes.go` (`diagnostic.Register` + `diagnostic.RegisterEventCode`), emitted on `selfUpdater.saveHistory`'s `Warn` log and raised via `report.RaiseErrorFixOnce`, reported by `recent-events` (`runtime` phase). `internal/install/disk` is the standalone initrd binary outside `doctor-coverage`'s `siteRoots` |
| Prometheus counters/metrics | Yes | Reuses existing `ze_dns_reply_write_failure_total` (`transport="stream"`) in `internal/core/dnsserver/secure.go` (`Manager.dohHandler`); no new metric family or label |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Reuses existing `ze_dns_reply_write_failure_total` with existing `transport="stream"` label in `docs/architecture/dns/server-harness.md`; no new counter or label |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `ai/rules/repo-maintenance.md` (unaffected: comment-only annotation in `managed/doctor.go`), `docs/architecture/appliance/on-device-installer.md` (unaffected: `fallbackDHCP` console log level only), `docs/architecture/appliance/self-update.md` (unaffected: history key and 20-event circular buffer unchanged; persistence failure now logs at Warn with doctor code), `docs/architecture/dns/secure-transports.md` (unaffected: DoH answer policy and HTTP status unchanged), `docs/architecture/dns/server-harness.md` (unaffected: existing `ze_dns_reply_write_failure_total{transport="stream"}` counter extended to DoH without adding a new metric), `docs/architecture/doctor-and-health-checks.md` (unaffected: follows existing `EventCode` registration pattern in `register_doctor_codes.go`), `docs/architecture/hub-architecture.md` (unaffected: comment-only annotation in `cmd/ze/hub/main.go`). Re-verify with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-8-core-hub-appliance.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

This spec resolves all 11 `error-dropped-below-warn` findings currently reported under the `core`, `hub-engine`, and `appliance` doors so `./le arch refuse-by-default report` has zero unannotated findings on these paths. Three sites gain real operator visibility: `selfUpdater.saveHistory` upgrades a `statestore.Put` failure from `Debug` to `Warn` under a newly registered doctor event code (`doctor-update-history-persist-failed` in `internal/component/config/system/register_doctor_codes.go`) and raises a bounded transient event on the report bus; `Manager.dohHandler` counts failed DoH HTTP response writes in the existing `ze_dns_reply_write_failure_total{transport="stream"}` counter alongside TCP and DoT; and `fallbackDHCP` in the initrd installer upgrades `linkUp` failures from `Debug` to `Warn` on the serial console with a clear fix while still scanning remaining NICs without exiting PID 1. The remaining 8 sites (plus `dohHandler`'s helper-counted branch) carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations because their failures are already returned (`Manager.bind`), already counted (`send`, `sendUnchanged`, `connLimitListener.Accept`), informational by design (`logDaemonCapabilities`), or non-actionable teardown/post-connect closes (`hubReachable`, `Manager.Stop`).

### Before and after

1. Self-update history persistence failure (`internal/component/config/system/selfupdate.go`, `selfUpdater.saveHistory`):
   - Before: logs `"history persist failed"` at `Debug` (invisible at default `WARN` level) with no doctor code and no report-bus event.
   - After: logs at `Warn` with `code=doctor-update-history-persist-failed`, naming the `meta/config/update-history` key, why it matters (recent update events will not survive a daemon restart), and the fix (`ze show data check`), and raises a bounded transient event on `internal/core/report` so `ze show doctor` (`recent-events`) and `ze show errors` display it.
2. DNS-over-HTTPS response write failure (`internal/core/dnsserver/secure.go`, `Manager.dohHandler`):
   - Before: logs `"dnsserver: doh response write"` at `Debug` and increments no metric, leaving DoH write failures invisible at default log level.
   - After: calls `recordWriteFailure(transportStream)` to increment `ze_dns_reply_write_failure_total{transport="stream"}` (matching `send` and `sendUnchanged`) and annotates the Debug line with `guards recordWriteFailure`.
3. On-device installer DHCP fallback `linkUp` failure (`internal/install/disk/network.go`, `fallbackDHCP`):
   - Before: logs `"network: linkUp failed"` at `Debug`, hiding which NIC failed to come up behind a later `"no NIC carrier detected"` error on the serial console.
   - After: logs at `Warn` on the serial console naming the interface, why it matters (cannot detect carrier or acquire an install lease), and the fix (check NIC hardware/driver or pin a working interface with `ze.mac=<mac>`), while continuing across remaining NICs and never exiting PID 1.
4. Justified below-WARN branches (`cmd/ze/hub/main.go`, `internal/component/managed/doctor.go`, `internal/core/dnsserver/{connlimit,handler,manager,signed}.go`):
   - Before: unannotated `Debug` log lines flagged by `./le arch refuse-by-default report`.
   - After: unchanged runtime behavior, each annotated with `// refuse-by-default: guards <Symbol> -- <why>` naming a verified non-enclosing symbol, bringing unannotated findings to zero.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Return `false` from `hubReachable` when `conn.Close()` fails | The TCP connect already proved the management hub reachable; returning `false` on a throwaway probe close error would falsely report a healthy hub as unreachable |
| Raise `logDaemonCapabilities` from `Debug` to `Warn` | Violates `docs/architecture/system-architecture.md` ("Capabilities without root"), breaks `TestLogDaemonCapabilitiesNeverWarns`, and contradicts `ai/rules/user-facing-errors.md` because a missing capability is informational before config is read |
| Raise DNS per-query write failures (`send`, `sendUnchanged`, `dohHandler`, `connLimitListener.Accept`) to `Warn` | A hostile or disconnected client would flood the daemon log at one WARN per query or connection; existing Prometheus counters (`ze_dns_reply_write_failure_total`, `ze_dns_tcp_connection_refused_total`) are the designed operator alert surface |
| Add a new `transport="doh"` label to `ze_dns_reply_write_failure_total` | `metrics.go` deliberately bounds `writeFailures` to two transport values (`datagram` and `stream`), and `handler.go` already treats TCP, DoT, and DoH alike as stream transports |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm the fresh `./le arch refuse-by-default report` population under this child's paths; write failing unit tests (`TestSelfUpdateSaveHistoryFailureWarnsAndRaisesEvent`, `TestSelfUpdateHistoryDoctorCodeRegistered`, `TestDoHHandlerWriteFailureCountsMetric`, `TestFallbackDHCPWarnsOnLinkUpFailure`).
2. **Phase: Answer 2 visibility fixes and doctor-code registration** -- register `codeUpdateHistoryPersistFailed` (`doctor-update-history-persist-failed`) in `internal/component/config/system/register_doctor_codes.go` (`diagnostic.Register` + `diagnostic.RegisterEventCode`), update `selfUpdater.saveHistory` to log `Warn` with `code`/what/why/fix and raise via `report.RaiseErrorFixOnce`, add `recordWriteFailure(transportStream)` in `Manager.dohHandler`, and raise `fallbackDHCP` `linkUp` failure to `slog.Warn` with what/why/fix behind `fallbackDHCPIn(sysnetDir string)`. Verify new tests pass.
3. **Phase: Answer 3 guard annotations** -- add the 9 verified `// refuse-by-default: guards <Symbol> -- <why>` annotations across `cmd/ze/hub/main.go`, `internal/component/managed/doctor.go`, and `internal/core/dnsserver/{connlimit,handler,manager,secure,signed}.go`.
4. **Phase: Prove** -- verify AC-1 through AC-6: filtered refuse-by-default report at 0 findings, `./le arch doctor-coverage check`, `./le arch tier check`, scoped package tests (including `-tags ze_distro` for `internal/component/config/system/` and `./le test unit installer`), and scoped functional tests from `./le repo changed scope print both` run by the main thread in the main checkout.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 11 instances at HEAD has its verified answer at `file:line` |
| Correctness | The installer never exits PID 1 on `linkUp` failure; `hubReachable` still returns `true` after a successful TCP connect; `logDaemonCapabilities` never logs at `Warn` or `Error` |
| Operator errors and doctor codes | `selfUpdater.saveHistory` logs `Warn` with `code=doctor-update-history-persist-failed` registered via `diagnostic.Register` and `diagnostic.RegisterEventCode` in `internal/component/config/system/register_doctor_codes.go`, raises on `internal/core/report` with fix `ze show data check`, and passes `./le arch doctor-coverage check`; `fallbackDHCP` logs `Warn` with what, why and fix on the initrd console |
| Guard annotations | Every `// refuse-by-default: guards <Symbol> -- <why>` names a visible symbol other than the enclosing function and passes `./le arch refuse-by-default check` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated `error-dropped-below-warn` findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered to this child's paths and `./le arch refuse-by-default check` |
| Doctor-coverage and tier checks clean | `./le arch doctor-coverage check` and `./le arch tier check` |
| Scoped package tests pass (main thread) | `./le job run label core command go test -race ./internal/core/dnsserver/`, `./le job run label hub command go test -race ./cmd/ze/hub/ ./internal/component/managed/`, `./le job run label system command go test -race -tags ze_distro ./internal/component/config/system/`, and `./le test unit installer` |
| Scoped functional suites pass (main thread) | Suites printed by `./le repo changed scope print both` run in the main checkout |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | `selfUpdater.saveHistory`'s `Warn` log and `report.RaiseErrorFixOnce` event MUST NOT include `su.url`, download URLs, or update credentials; `fallbackDHCP`'s `Warn` MUST NOT leak rescue auth or mirror URL credentials |
| Log-flood DoS | Per-query DNS write failures (`send`, `sendUnchanged`, `dohHandler`) and per-connection TCP cap refusals (`connLimitListener.Accept`) stay at `Debug` and count in Prometheus metrics rather than emitting `Warn` per packet/connection |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint or gate failure (`doctor-coverage`, `refuse-by-default`, `tier`) | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One child for three small doors | One child per door | 11 instances across 9 files; reading the three doors once is the cheaper split |
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct, violates `system-architecture.md` on `logDaemonCapabilities`, and creates per-query log floods on DNS handlers |
| Count DoH write failures in existing `ze_dns_reply_write_failure_total{transport="stream"}` | Add a new metric or `doh` label, or leave DoH uncounted | Closes a real observability gap in `Manager.dohHandler` while preserving `metrics.go`'s bounded two-label design |
| Register `doctor-update-history-persist-failed` as a transient event code in `internal/component/config/system/register_doctor_codes.go` | Log WARN without a doctor code, or add a dedicated polling check | Satisfies `doctor-coverage` and `ai/rules/user-facing-errors.md`; `saveHistory` is a transient write error best surfaced via `report.RaiseErrorFixOnce` into `recent-events` |
| Retire the 2026-11-02 `hubReachable` Answer-1 claim into Existing Mechanisms | Rewrite `hubReachable` to return `false` on `conn.Close()` failure | Source verification showed `hubReachable` already returns `false` on dial/source errors (`TestHubDoctorCheckUnreachable`, `TestHubReachableProbe`); line 539 is only post-connect socket close |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the arch checks (`refuse-by-default`, `doctor-coverage`, `tier`), and the functional suites `./le repo changed scope print both` reaches
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-8-core-hub-appliance.md` only, in the same `./le commit create` script
