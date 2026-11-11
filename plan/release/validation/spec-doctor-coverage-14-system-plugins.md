# Spec: doctor-coverage-14-system-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md` (Goal and scope): `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the doctor-coverage gate, the `recent-events` check: landed in commits ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` for the cross-cutting codes (`doctor-internal-defect`, `doctor-plugin-stopped`, `doctor-plugin-crashed`, `doctor-plugin-config-unapplied`, `doctor-plugin-setup-failed`, `doctor-plugin-external-unsupported`, `doctor-plugin-stop-cleanup-late`), the transient-code registration and a raise carrying its fix (phase P0a), the registration field that declares a plugin runs only in the daemon's process and its `doctor-plugin-external-unsupported` check (AC-24, phase P4; the code itself is registered in P0a), and the plugin-protocol route for a forked plugin to raise on the daemon's report bus (phase P0b); `plan/release/validation/spec-doctor-coverage-10-forwarding.md` for `doctor-firewall-drift` and `doctor-firewall-apply-behind`; `plan/release/validation/spec-doctor-coverage-5-appliance.md` for the `doctor-dns-source-*` check and the `ConsumerSelection` fields of its AC-19 (AC-31, AC-32) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Source: the A13, A14 and A15 slices of the doctor problem inventory, merged in `plan/audits/doctor-problem-inventory/system-plugins.md`, and the canonical code table of the merge.

## Task

Doctor reports every problem class of the system plugins (`internal/plugins/*` outside the routing protocols and the forwarding plugins) with its cause and the command that fixes it, on every surface through `show doctor`. Every Warn and Error log line of the area names a doctor code that a registered check declares (O-2), so `./le arch doctor-coverage report` counts the area covered. Five existing checks that read daemon state outside the `runtime` phase move to it (AC-9 of the parent).

| Sub-area | Packages | Gap rows | AC-9 moves | Defects owned |
|----------|----------|----------|------------|---------------|
| Anomaly and DDoS | `internal/plugins/anomaly/**`, `internal/plugins/ddos/**` | 11 | `ddos-detect-rate-feed` | - |
| DNS, DHCP, CoS, crashes, copp | `as112`, `dnsresolver`, `dhcpserver`, `cos`, `crashes`, `copp`, `connected` | 15 | `copp-input-chain` | - |
| ExaBGP bridge, flow export, FlowSpec firewall, kernel | `exabgp/**`, `flowexport`, `flowspec-firewall`, `kernel` | 16 | - | flow-export health row never clears |
| Image server, TFTP, MRT, NTP, geodns | `imageserver`, `tftpserver`, `mrt`, `ntp`, `geodns` | 16 | `ntp-clock-privilege`, `geodns-listen-capability` | NTP and MRT Warn floods |
| Static, redistribute | `static`, `redistribute` | 9 | `static-route-skipped` | - |
| Traffic usage, VRRP | `trafficusage`, `vrrp/**` | 15 | - | - |
| Provision, update server | `provision`, `update-cmd` | 2 | - | update server ends with no log line |
| Shared (every plugin above) | registrations | 1 | - | - |

No row of this area is owned by child 2 (`spec-doctor-coverage-2-home-cpe-batch.md`); its static-skipped mention is a cross-reference only.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the four questions; a plugin can run in another process
  → Constraint: a check registers from its owner's `init()` or its plugin `Registration`; no central list of codes, checks or plugins
  → Constraint: a plain Go call into another component's state works only in process; a runtime check of a plugin that can run forked is answered by the plugin itself
- [ ] `ai/doors/system-plugins.md` - two plugin shapes, the gated plugins, the traps
  → Constraint: every plugin here can run in another process, so a runtime check that reads a package global in the daemon is valid only for a plugin that declares it runs in the daemon's process
  → Constraint: a plugin owns its whole surface: its doctor checks and code metadata live in its folder
  → Constraint: always-on code never imports a gated plugin (`ntp`, `dhcpserver`, `tftpserver`, `imageserver`, `exabgp`)
- [ ] `ai/doors/plugin-infra.md` - Stage 1 declarations, both transports, `./le mock plugin-external`
  → Constraint: every feature works over both transports; a forked plugin never gets `DirectBridge`
- [ ] `ai/doors/forwarding.md` - the report bus and the firewall table registry are process-local
  → Constraint: a raise a forked plugin makes never reaches `show warnings` until child 12's route exists (AC-3)
- [ ] `ai/doors/core.md` - runtime state goes through the daemon's one store
  → Constraint: a persisted record uses `statestore` with a key registered in `pkg/zefs/keys.go`, or the SDK state RPCs in a plugin (AC-83)
- [ ] `ai/doors/config-yang.md`, `ai/doors/appliance.md`, `ai/doors/le-tooling.md` - the published selection in `internal/component/config/system`, the gate under `internal/le`
  → Constraint: a removed root arrives as `{}`; a check never reads nil config as "nothing configured" when the root is present
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the `runtime` phase
  → Constraint: a `runtime` check is handed the platform alone and reads its owner's live state, never a config tree (`RunRuntimeChecks`)
- [ ] `spec-doctor-coverage-1-audit-and-gate` - O-1, O-2, O-3, Data Flow 2a and 4b amendments
  → Decision (O-1): the local route asks a reachable daemon; the daemon's answer then runs every phase in the daemon process
  → Decision (O-2): every Warn and Error line carries a doctor code; a transient class is a report-bus raise shown by `recent-events`
  → Constraint: a health row takes no code; the audit maps each row to a check by hand (Data Flow 4b)
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - the rows it owns
  → Constraint: none of this area's rows is child 2's; `doctor-static-route-skipped` stays this area's code
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem, why, then the fixing command alone in the `fix` field
- [ ] `ai/rules/spec-no-code.md`, `ai/rules/planning.md`
  → Constraint: tables, no snippets; every open journal row is owned by a spec table

**Key insights:**
- `collectDoctorChecksFrom` (`internal/component/doctor/cmd/show.go`, renamed from `collectPluginDoctorChecks` by commit 149497ce12) calls, on the daemon route, every check a running plugin declared at Stage 1, whatever its phase, and the plugin answers it in its own process through `OnDoctorCheck`; a `registry.DoctorCheckDef` check runs in the daemon process and sees nothing of a forked plugin, so the registry refuses one in the `runtime` phase (`validateDoctorCheckDef`), and Stage 1 refuses a declared check whose name the plugin's registry row already holds (`refuseDoctorCheckDeclaredTwice`)
- No report-bus raise and one health row (`flow-export`) exist in the area today, so most transient classes need both a record in the owner and a raise
- 18 plugins work only in the daemon's process and know it only in code; child 12 gives them one declared field
- dhcpserver already bounds its Warn lines (`warnLimited`); the NTP and MRT floods reuse that shape

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/flowexport/health.go` - `checkFlowExportHealth` counts a collector failing when its `errors` stat is above zero
- [ ] `internal/plugins/flowexport/sender.go` - `Sender.errors` is an `atomic.Uint64` that `Send` only adds to; `Stats` returns it; nothing resets it
- [ ] `internal/plugins/flowexport/register.go` - `flow-export-conntrack-tracking` (post-config); `runEngine` refuses to start forked with an Error line; the advisor loop logs a non-literal Warn message
- [ ] `internal/plugins/update-cmd/cmd/serve_daemon.go` - `updateServeListener.serve` discards the `Serve` error; `status` reads `ended()`; no log line anywhere when the server ends on its own
- [ ] `internal/plugins/ntp/ntp.go` - `syncWorker.run` phase 2 retries `doSync` every second; `doSync` Warns per failing server per try and "no reachable servers"
- [ ] `internal/plugins/ntp/doctor.go` - `ntp-client` and `ntp-clock-privilege` registered post-config through `diagnostic.RegisterDoctorCheck`
- [ ] `internal/plugins/ntp/doctor_linux.go` - `checkNTPClockPrivilege` reads `/proc/self/status` and `os.Getuid` of the process running the check
- [ ] `internal/plugins/mrt/async_writer.go` - `asyncWriter.Write` Warns per dropped record; `drain` Warns per failed write
- [ ] `internal/plugins/ddos/detect/doctor.go` - `rateFeedCheckDecl` declares phase post-config for a check only the running plugin answers
- [ ] `internal/plugins/copp/register.go` - `copp-input-chain` `registry.DoctorCheckDef` at post-config
- [ ] `internal/plugins/copp/doctor.go` - `readKernelTables` reads `firewall.GetBackend()`, nil in a process that never ran `ApplyAll`
- [ ] `internal/plugins/geodns/doctor.go` - `checkGeoDNSListenCapability` bind-probes each configured privileged port (`probeBindable`)
- [ ] `internal/plugins/static/doctor.go` - `static-route-skipped` post-config, reads `activeRouteManager`, nil locally and when static runs forked
- [ ] `internal/plugins/dhcpserver/register.go` - `warnUnserved`, `warnWithheld`, `warnLimited`: Warn through a function value, once per `unservedWarnInterval`
- [ ] `internal/plugins/anomaly/shape/register.go` - Warns when the legacy sweep (`firewall.LegacySweepPending`) did not run
- [ ] `internal/component/doctor/registry.go` - `RunRuntimeChecks`, daemon route only
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` shows bus events under their own code when the code has metadata
- [ ] `internal/component/doctor/cmd/show.go` - `collectDoctorChecksFrom` asks every running plugin for its Stage 1 declared checks on the daemon route
- [ ] `internal/core/diagnostic/codes.go` - code metadata, the existing static, ddos and flow-export codes among it
- [ ] `pkg/zefs/keys.go` - the registered zefs keys a persisted record joins

**Findings at the producer (2026-11-10):**

| Claim from the slice | Verdict | Evidence |
|----------------------|---------|----------|
| flow-export health row reads a counter that never resets | confirmed | `checkFlowExportHealth` tests `errors > 0`; `Sender.Send` only adds to `errors` |
| NTP Warn lines flood the log until the first sync | confirmed | `run` phase 2 loop with a one-second sleep; `doSync` Warns per server per try |
| MRT Warns once per dropped record | confirmed | `asyncWriter.Write` default arm; `drain` per failed write |
| update server can stop with no log line | confirmed | `serve` drops the `Serve` error under a `nolint` |
| `ddos-detect-rate-feed` declared post-config | confirmed | `rateFeedCheckDecl.Phase` |
| `copp-input-chain` reads a backend only the daemon loads | confirmed | `readKernelTables` answers `errNoFirewallBackend` when `GetBackend` is nil |
| `ntp-clock-privilege` reads the checking process | confirmed | `ntpCurrentUID`, `ntpReadProcess` of `self` |
| `geodns-listen-capability` bind-probes ports a daemon holds | confirmed | `probeBindable` per target; the message names "already in use" |
| `static-route-skipped` silent locally and forked | confirmed | `checkRouteSkipped` returns nil on a nil `activeRouteManager` |

**Behavior to preserve:**
- every existing check and code of the area: `anomaly-detect-feature-source`, `anomaly-shape-firewall`, `as112-listen-capability`, `dns-resolver-listen-capability`, `dns-resolver-upstream-reachable`, `copp-input-chain`, `crash-directory-writable`, `crash-capture-armed`, `crash-capture-pstore`, `ddos-detect-rate-feed`, `ddos-detect-flow-source`, `dhcp-listen-interface`, `flow-export-conntrack-tracking`, `geodns-listen-capability`, `geodns-tls-cert`, `ntp-client`, `ntp-clock-privilege`, `static-route-skipped`, `static-interface-nexthop-backend`, `traffic-usage-ebpf`, `vrrp-config-sanity`, `vrrp-raw-socket`, `systemd-service`, with their codes
- `static-fib-writer` (`doctor-static-no-fib-writer`) is not preserved here: `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` AC-34 removes it in favour of its generic `doctor-plugin-no-dataplane-writer` check, and this spec neither keeps nor edits it
- the `flow-export` health row name and its healthy, degraded and down meanings
- `update serve` answers `running` from the server goroutine

**Behavior to change:**
- one check, or one more code on an existing check, per gap class below
- five checks move to the `runtime` phase, or split into a local and a runtime half
- every Warn and Error line of the area names its code; the NTP and MRT lines are bounded

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor check registry and phases | `diagnostic.RegisterDoctorCheck`; `RunRuntimeChecks` (`internal/component/doctor/registry.go`) | owner-registered checks per phase; `runtime` runs only in the daemon | extends: a new check of an in-process-only plugin registers here at `runtime`; local config and host checks at `post-config` |
| Plugin doctor checks, registry form | `registry.DoctorCheckDef` in a plugin `Registration`; `validateDoctorCheckDef` (`internal/component/plugin/registry/doctor.go`) | runs in the process that answers (the daemon, or the local process) beside the plugin registry; refused in the `runtime` phase since commit 149497ce12 | extends: used for config and host checks only |
| Plugin doctor checks, SDK form | Stage 1 `rpc.DoctorCheckDecl` answered by `OnDoctorCheck` (`ddos/detect/doctor.go`, `register.go`); called by `collectDoctorChecksFrom`; `refuseDoctorCheckDeclaredTwice` (`internal/component/plugin/server/startup.go`) | the running plugin answers, in process or forked; a name its registry row also holds is refused at Stage 1 | extends: every runtime check of a plugin, whether it can run forked or is declared in-process only, is declared here with phase `runtime` (AC-2), under a name its registry row does not hold |
| `Diagnostic.Fix` | `diagnostic.Diagnostic.Fix` (`internal/core/diagnostic/types.go`) | the fixing command on its own line | extends: every new diagnostic sets it |
| Report bus | `report.RaiseWarning`, `RaiseError`, `ClearWarning` (`internal/core/report/report.go`) | coded active warnings and an error ring, process-local | extends: each transient class raises under its doctor code; a forked plugin raises through child 12's plugin-protocol route (AC-3) |
| `recent-events` check | `recentEventsCheck` (`checks_recent_events.go`) | one row per bus event under the event's code when that code has metadata | extends: each transient code here has metadata and is declared by its owner's runtime check |
| Health registry | `health.Register` (`internal/core/health/registry.go`); `checkFlowExportHealth` | `flow-export` row from cumulative send errors | extends: the row judges a recent window (AC-41); no new row |
| Doctor-coverage gate | `./le arch doctor-coverage` (`internal/le/arch/doctorcoverage`) | counts a site covered when its code is declared by a registered check | extends: every site of the area becomes covered; the function-value and non-literal sites carry a code the scan resolves (AC-4) |
| In-process-only declaration | child 12's registration field (built in `spec-doctor-coverage-12-plugin-infra.md`) and its local check `doctor-plugin-external-unsupported` | does not exist today; each plugin hard-codes the knowledge (`IsInternal()` refusals in `as112`, `cos`, `ddos/detect`, `dhcpserver`, `flowexport`, `static`, `trafficusage`, `vrrp`; silent in `connected`, `kernel`, `mrt`, `flowspec-firewall`) | extends: this spec sets the field per plugin (AC-1); it builds no check of its own |
| Bounded Warn | `warnLimited` (`internal/plugins/dhcpserver/register.go`) | a Warn at most once per interval | extends: the NTP and MRT fixes use the same first-of-run shape with a count (AC-58, AC-53) |
| Existing area checks | the list under Behavior to preserve | config plus host state | extends: `dhcp-listen-interface` (AC-27), `flow-export-conntrack-tracking` (AC-36), `static-route-skipped` (AC-10, AC-61), `ntp-client` (AC-56); the five AC-9 checks move phase |
| Plugin status producers | `syncWorker.publishState` (NTP), `routeManager.skippedRoutes` (static), `pool.stats` (DHCP), `exporter.status` (flow export), `ze_vrrp_packet_errors_total` (VRRP), `updateServeOwner.status` | live state each `show` command already reads | extends: the runtime checks read these; where the slice says "not stored today", the owner keeps the record the AC names |
| Persisted runtime state | `statestore` with a key in `pkg/zefs/keys.go`; SDK `StateGet`/`StatePut` | owned state survives a restart | extends: the provision added-address record (AC-83) |
| Owner-registered code metadata | `internal/core/diagnostic/codes.go` | `ze show diagnostic <code>` explains a code | extends: each new code is registered with its cause and fix |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor [--json] [config <file>]` locally; `show doctor` on SSH, MCP, REST, gRPC and web
- `./le arch doctor-coverage check` and `report` for the line codes

### Transformation Path
1. Local checks (`post-config`) read the config tree plus host state (files, sysfs, procfs, tc, nft, netlink reads); they apply nothing
2. Runtime checks read the owner's live state and run only on the daemon route: every plugin, in-process-only or forkable, through its own `OnDoctorCheck` answer to a Stage 1 declaration at `runtime` (the registry refuses a `runtime` `DoctorCheckDef`); a component that is not a plugin (`internal/component/firewall`, AC-11) through `diagnostic.RegisterDoctorCheck` at `runtime`
3. A transient class raises on the report bus under its doctor code, from the daemon process or through child 12's plugin-protocol route; `recent-events` shows it with subject, count and time
4. Each diagnostic carries code, severity, message and `fix`; `show doctor` returns it on every surface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to doctor, forked or in process | Stage 1 `DoctorCheckDecl` and `OnDoctorCheck` over the plugin protocol | Yes: `ddos-detect-rate-feed` already crosses it |
| Plugin registry to doctor | `registry.DoctorCheckDef` in the daemon process | Yes: `copp-input-chain`, `static-route-skipped` |
| Forked plugin to report bus | child 12's raise route | No: built by child 12; AC-3 depends on it |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `test/`, `docs/`; terms `ddos-detect-rate-feed`, `DoctorPhasePostConfig` in `ddos/detect`, `flow-export` health row name, `updateServeStatus`, `checkFlowExportHealth`. Positive control: `rateFeedCheckDecl` carries phase post-config (read above). Found: the decl in `ddos/detect/doctor.go`, its reader `collectDoctorChecksFrom` (named `collectPluginDoctorChecks` before commit 149497ce12); the health row writer `health.go` and readers `show health` and `ze status`; the `update serve` answer writer `updateServeOwner.status`, read by its handler and API clients. No report-bus code of this area exists today (slices A13 to A15: no raise).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Stage 1 doctor declaration phase | writer | `rateFeedCheckDecl` (`ddos/detect/doctor.go`), and each new Stage 1 runtime declaration | phase field of `rpc.DoctorCheckDecl` | change: `runtime`; the value already exists in `pkg/plugin/rpc` |
| same | reader | `collectDoctorChecksFrom` (`doctor/cmd/show.go`); a third-party plugin reads nothing of another's declaration | calls every declared check whatever its phase | preserve: the reader already answers runtime declarations |
| `show doctor` JSON | writer | the new checks, through `diagnostic.Diagnostic` | keys unchanged | preserve: rows are added, no key changes |
| `show doctor` JSON | reader | `outputJSON`, `outputText` (doctor component); MCP, REST, gRPC, SSH clients; `test/plugin/*doctor*.ci` | rows are data | preserve: a reader that lists rows sees more rows |
| `flow-export` health row | writer | `checkFlowExportHealth` | status and reason | change of meaning only: failing means errors in the recent window; names unchanged |
| same | reader | `show health`, `ze status`, the health page | status words | preserve: same words, now correct |
| `update serve` answer | writer | `updateServeOwner.status` | `running`, `listen`, `paused` | change: one optional `last-error` key (kebab-case, declared in its YANG) when Serve ended on its own |
| same | reader | `handleUpdateServe`; API clients | JSON object | preserve: an added optional key breaks no reader |
| report-bus codes | writer | each transient AC's owner | new codes | new: no existing reader |
| report-bus codes | reader | `recent-events`, `show warnings`, `show errors` | code is data | preserve |

### Integration Points
- `diagnostic.RegisterDoctorCheck`, `Registration.DoctorChecks` and Stage 1 `DoctorChecks` in each owner
- `report.RaiseWarning` and `RaiseError` in each producer of a transient class
- child 12's in-process-only registration field in each plugin's `Registration`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | a forkable plugin answers its own runtime check; no check reads another package's globals across a process |
| No unintended coupling (components stay isolated) | Yes | each check lives in the plugin that owns the state; `copp` reads the firewall backend through the firewall registry as today |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; the external-unsupported class reuses child 12's one check |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path; the MRT and DHCP counters are atomic adds |
| Registration over hardcoding, outbound | Yes | every check and code registers from its owner |
| Registration over hardcoding, inbound | Yes | searched: no central list names these plugins; the in-process-only knowledge moves from per-plugin code into one declared field child 12 reads |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Child 12 delivers the in-process-only field, its local check and the forked raise route before P1 | brief from the main thread; CODES table owner column | AC-1 and AC-3 have nothing to set or call | P1 reads child 12's landed registration field; if absent, P1 stops and reports | unvalidated |
| A-2 | Every fix command an AC names exists with that grammar | the command registry and YANG | a fix line names a command that does not exist | each check's unit test resolves its fix through the command registry or the YANG schema | unvalidated |
| A-3 | The doctor runner can mark a check as answering only on the local route's fallback (no daemon answered) | child 10 R-3 relies on the same; child 1 O-1 fallback | the geodns, as112 and dnsresolver bind probes warn on a healthy daemon | P1 reads `local_route.go` and the runner; if absent, P1 adds the marker in `internal/core/diagnostic` | unvalidated |
| A-4 | The listener manager in `internal/core/dnsserver` can answer its bound set and each bind error | slice A13 row "DNS listener not bound" | AC-21 needs a producer first | P4 reads the manager; adds a status answer if missing | unvalidated |
| A-5 | `doctor-image-listener-down` is not in the canonical code table | CODES.md lists no runtime code for the imageserver listen row | the name collides with another child's | the merge agent adds it to the inventory table | unvalidated |
| A-6 | static, ntp, geodns, imageserver, tftpserver, dnsresolver, exabgp, redistribute and update-cmd can run forked; the 18 plugins in AC-1 cannot | `IsInternal()` refusals; slices A13 to A15 | a runtime check uses the wrong form | P1 lists each plugin's mode from its `Registration` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A runtime check reads a package global for a plugin that runs forked, and reads healthy | the plugin runs external in a test | A-6; the forkable form is the Stage 1 declaration; each such check is also proven under `./le mock plugin-external <name>` |
| R-2 | A recent-event code raised by a forked plugin never reaches the daemon's bus | `recent-events` shows nothing in an external-mode test | A-1; until the route lands, the owner keeps a counter its runtime check answers |
| R-3 | A "left behind" check counts state another owner installed | a row names a foreign TCX program or address | count only Ze's: Ze's eBPF program tag, the provision record, Ze's route protocol and table IDs |
| R-4 | A pending or failing check fires during normal convergence (NTP before first sync, VRRP during election) | rows on a healthy box at startup | thresholds are the owner's own timers (NTP poll interval, VRRP master-down interval), never a new constant |
| R-5 | Bounding the NTP and MRT lines hides a new error | a changed error not logged | Warn again on each change of a server's error, and at the end of a drop run with its count |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | false or missing doctor rows; the `flow-export` health row changes meaning; NTP and MRT log fewer lines; `update serve` gains one optional key |
| How is it reverted? | one commit per phase |
| Who else touches this path? | child 12 (plugin codes, field, raise route); child 10 (firewall drift and apply-behind codes); the appliance child (dns-source check); the doctor component's `listeners` check (bind probe of image and NTP ports, its own slice) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a static route whose next-hop the kernel refuses | → | static runtime checks through `OnDoctorCheck` | `test/static/show-doctor-static-route-skipped.ci` (`option=needs-linux`) |
| `ze doctor` locally, daemon running, `copp bgp` configured | → | the copp runtime half (AC-7), answered by copp's `OnDoctorCheck` on the daemon route | `test/plugin/doctor-copp-runtime.ci` (`option=needs-linux`) |
| `show doctor` with an NTP server that never answers | → | NTP runtime sync check | `test/plugin/show-doctor-ntp-not-synced.ci` |
| `show doctor` with geodns serving on a privileged port | → | geodns runtime listener check, no bind-probe row | `test/plugin/show-doctor-geodns-listener.ci` |
| `ze doctor --json` with `plugin { external mrt }` | → | child 12 check over the field mrt sets | `test/ui/doctor-plugin-external-unsupported-mrt.ci` |
| `show doctor` with a DHCP pool of one address leased | → | DHCP pool runtime check | `test/plugin/show-doctor-dhcp-pool-exhausted.ci` (`option=needs-linux`) |
| `show doctor` after the update server's listener is closed under it | → | update server runtime check | `TestUpdateServerEndedDoctor` |
| `show health` after one failed flow-export send and a recovery | → | `checkFlowExportHealth` recent window | `TestFlowExportHealthRecoversAfterWindow` |
| `./le arch doctor-coverage report` | → | every system-plugins site resolves | `TestSystemPluginsSitesCovered` |

## Acceptance Criteria

Every row below holds for every check it names: the message states the cause in plain words, `fix` holds the command alone, the row reaches every surface through `show doctor`, the unit test goes red when the detection is removed, and the check declares every code it emits. Route is `local` (`post-config`, both routes) or `runtime` (daemon only; a forkable plugin answers it through `OnDoctorCheck`, AC-2). A "recent event" code is raised on the report bus by its owner, has metadata, is declared by its owner's runtime check, and is shown by `recent-events`. Code names are the canonical ones of `plan/audits/doctor-problem-inventory/system-plugins.md`. Each Warn or Error line at a site the row lists carries the row's code.

### Shared mechanisms (before the checks that need them)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `plugin { external <name> }` for anomaly-detect, anomaly-observe, anomaly-shape, ddos-detect, ddos-observe, ddos-flowspec, ddos-local, ddos-flowtriq, as112, dhcpserver, cos, connected, flow-export, flowspec-firewall, kernel, mrt, vrrp or traffic-usage | each of these registrations sets child 12's in-process-only field, so child 12's local check reports `doctor-plugin-external-unsupported` (error) naming the plugin and why it cannot run forked, with the fix that removes the `external` entry; each plugin's own refusal line (the `IsInternal()` branches) carries the same code; the silent ones (connected, kernel, mrt, flowspec-firewall) refuse to start forked with that line instead of running inert | a plugin that works forked sets nothing and is never reported |
| AC-2 | A runtime check of a plugin that can run forked (static, ntp, geodns, imageserver, tftpserver, dnsresolver, exabgp, redistribute, update-cmd) | it is a Stage 1 `DoctorCheckDecl` with phase `runtime`, answered by the plugin's `OnDoctorCheck` from its live state, so it answers the same in process and forked; a unit test drives each through the callback, and one check per plugin is proven under `./le mock plugin-external <name>` | none: a plugin declared in-process only (AC-1) declares its runtime checks the same way, because the plugin registry refuses a `registry.DoctorCheckDef` in the `runtime` phase (`validateDoctorCheckDef`, commit 149497ce12); for it the forked proof under `./le mock plugin-external` does not apply |
| AC-3 | A transient class below raised by a plugin running forked | the raise reaches the daemon's report bus through child 12's plugin-protocol route, so `recent-events` shows it as for an in-process raise | none: a raise lost at the process boundary is the silent class this work closes |
| AC-4 | The Warn sites the gate's scan does not resolve: dhcpserver's `warnUnserved` and `warnWithheld` callers (`register.go`, `segment.go`) and the flow-export advisor line (`register.go`) | each carries a `code` attribute naming its row's code (AC-28, AC-27, AC-42), passed through `warnLimited`, so `./le arch doctor-coverage report` lists them covered | none |
| AC-5 | A Warn line of this area that reports nothing wrong | it logs at Info and carries no code; the commit that moves it names each line and why nothing is wrong | a line that reports a degraded outcome stays Warn with its code |

### Checks that move to the runtime phase (parent AC-9)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-6 | `ddos-detect-rate-feed` | `rateFeedCheckDecl` declares phase `runtime`; the local route never lists it as a config check; `doctor-ddos-detect-blind` (warning) unchanged in the daemon | none |
| AC-7 | `copp-input-chain` with `control-plane-protection bgp` configured | split: a `post-config` half reports a copp config that does not parse (`doctor-copp-missing`, error); a `runtime` half in the daemon reads the kernel tables through the backend `ApplyAll` loaded and reports a missing `ze_copp` table or unknown-source-drop term (`doctor-copp-missing`, error) with fix `ze signal restart`; the post-config half keeps the `copp-input-chain` `registry.DoctorCheckDef`, and the runtime half is a Stage 1 declaration under a name of its own answered by copp's `OnDoctorCheck`, because the registry refuses a `runtime` `DoctorCheckDef` and Stage 1 refuses a name the registry row already holds (`refuseDoctorCheckDeclaredTwice`); the local process never answers "kernel firewall tables could not be read" for a backend it never loaded | none |
| AC-8 | `ntp-clock-privilege` with the client enabled | the check runs in the process that adjusts the clock (the NTP plugin's `OnDoctorCheck`), reads that process's uid and CapEff, and reports `doctor-ntp-clock-privilege` (warning) when CAP_SYS_TIME is missing or the last slew, step or restore failed (the worker keeps the last set-clock error and time); fix names the service unit's capability setting; the post-config `diagnostic.RegisterDoctorCheck` registration in `ntp/doctor.go` is deleted (`ai/rules/no-layering.md`), so one check answers | none |
| AC-9 | `geodns-listen-capability` while the daemon serves geodns | split: the runtime half reads the dnsserver listener manager's bound set against `service geodns listener` and reports each configured listener not bound, `doctor-geodns-port-unavailable` when the bind error was a permission refusal and `doctor-dns-listener-down` (error) otherwise (AC-21); the bind probe answers only on the local route's fallback when no daemon answered (A-3) | the TLS certificate check `geodns-tls-cert` stays `post-config` |
| AC-10 | `static-route-skipped` | the check is a Stage 1 runtime declaration answered by the static plugin from its `routeManager`, so it reports `doctor-static-route-skipped` (warning) in process and forked; the `post-config` registration is deleted (`ai/rules/no-layering.md`) | none |

### Anomaly and DDoS

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-11 | `firewall.LegacySweepPending()` still true after the first reconcile | the firewall component's runtime check reports `doctor-firewall-legacy-tables-pending` (warning) naming the older build's tables still enforcing; fix `ze signal restart`; anomaly-shape's Warn line carries the code. Owner `internal/component/firewall`; this spec carries it because child 10 does not list it | none |
| AC-12 | The anomaly-shape responder refuses to arm because `armedCount` reached `BlastRadiusCap` | recent event `doctor-anomaly-shape-cap-reached` naming the source not rate-limited and the cap; fix names the blast-radius leaf | none |
| AC-13 | `kill-switch true` with `mode armed` | `doctor-anomaly-shape-kill-switch` (warning, local) saying nothing is enforced; fix names the delete of `kill-switch` | `mode shadow` with the kill switch enforces nothing by design and is not reported |
| AC-14 | ddos-detect's `show traffic usage` or `show flow recent` dispatch fails while its source is configured | the detector keeps the last outcome and time per command; `doctor-ddos-detect-flow-source-failing` (warning, runtime) naming the command, its error and the fallback (generic flood, coarse target); fix names the failing command | the config-only half stays `doctor-ddos-detect-no-flow-source` |
| AC-15 | An attack is detected with no victim resolved | recent event `doctor-ddos-no-victim` from the flowspec and local responders, naming the attack and the skip reason | none |
| AC-16 | The FlowSpec announce limiter refuses an announce | recent event `doctor-ddos-flowspec-rate-limited` naming the attack and the limit | none |
| AC-17 | The BGP engine refuses the FlowSpec announce during an attack | `doctor-ddos-flowspec-announce-failed` (error, runtime) from the responder's last dispatch error while `active` is false; fix names the BGP FlowSpec show command | none |
| AC-18 | The FlowSpec withdraw failed after the attack ended | `doctor-ddos-flowspec-withdraw-failed` (error, runtime) naming the rule still announced; fix names the withdraw command | none |
| AC-19 | Flowtriq API calls fail | the reporter keeps the last error and time per call kind; `doctor-ddos-flowtriq-unreachable` (warning, runtime) naming the call kind and incidents left open | none |
| AC-20 | ddos-local's drop rule is not installed during an attack (registry refused, apply failed, rollback failed) | `doctor-ddos-local-drop-failed` (error, runtime) naming the attack and the failure; the kernel-side lag stays child 10's `doctor-firewall-apply-behind` | none |

### DNS, DHCP, CoS, crashes

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-21 | A configured as112, dnsresolver or geodns listener is not bound in the running daemon | `doctor-dns-listener-down` (error, runtime) from the dnsserver listener manager (A-4) naming the address, the bind error and, for as112, that the covering prefixes stay withdrawn; fix names the listener leaf | the pre-start probes `as112-listen-capability` and `dns-resolver-listen-capability` answer only on the local fallback (A-3) |
| AC-22 | A RADIUS session or CoA names a CoS profile that is not configured | cos keeps the unknown names with a session count; `doctor-cos-profile-unknown` (warning, runtime) naming profile and sessions | none |
| AC-23 | A per-interface QoS map was not applied, or not reverted at session down | `doctor-cos-qos-apply-failed` (error, runtime) from `sessionStore` against the applied map, naming the interface | none |
| AC-24 | The crash directory holds a kernel crash record recovered from the previous boot | `doctor-crash-kernel-recovered` (warning, local) naming the record and its time; fix names the command that shows crash records | none |
| AC-25 | `HarvestKernelCrashes` left records in pstore | `doctor-crash-records-unharvested` (warning, local) naming the reason and the count | none |
| AC-26 | A configured DHCP listen interface is not served by the running daemon (bind failed, device appeared after start, nothing bound) | `doctor-dhcp-listener-down` (error, runtime) from the daemon's listener set; fix names `service dhcp-server listen-interface` | the local device check stays `doctor-dhcp-iface` |
| AC-27 | A subnet's `server-identifier` is not held on the listen interface | `dhcp-listen-interface` also reports `doctor-dhcp-server-identifier-not-held` (error, local) naming subnet and address; the withheld-reply line carries it (AC-4) | none |
| AC-28 | A relayed request whose giaddr lies in no subnet | the server keeps the last unserved giaddr and time; recent event `doctor-dhcp-relay-unserved`; the bounded Warn line carries it (AC-4) | none |
| AC-29 | A subnet's pool has no free address | `doctor-dhcp-pool-exhausted` (warning, runtime) from `pool.stats()` naming the subnet; fix names the pool range leaf | none |
| AC-30 | Replies fail to send (socket write error, Debug today) | the server counts send errors; the line moves to Warn with the code; recent event `doctor-dhcp-reply-send-failing` naming interface and count | none |
| AC-31 | Option 6 drops a host-only resolver | the dhcpserver publisher only FILLS the dropped-list field that `plan/release/validation/spec-doctor-coverage-5-appliance.md` AC-19 adds to `dnssource.ConsumerSelection` (this spec adds no field); the dhcpserver line carries `doctor-dns-source-host-only-dropped`, which child 5's runtime check declares | none |
| AC-32 | A resolver list is longer than option 6 or `forwardServersMax` uses | the dhcpserver and dnsresolver publishers only FILL the limit field that child 5 AC-19 adds to `dnssource.ConsumerSelection`; their lines carry `doctor-dns-source-over-limit`, declared by child 5's check | none |

### ExaBGP bridge, flow export, FlowSpec firewall, kernel

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-33 | A bridge `process <name> run` program is not found on the daemon's PATH or not executable | `doctor-exabgp-script-start-failed` (error): local from `exec.LookPath` and the exec bit, runtime from `script.child` nil after `Fleet.Start`, naming the process and the path tried | none |
| AC-34 | A respawning script hit `respawnMax` inside `respawnWindow`, or a restart failed | `doctor-exabgp-script-stopped` (error, runtime) naming the script and its last exit | none |
| AC-35 | A script does not read its stdin and events are dropped | `doctor-exabgp-script-lines-dropped` (warning, runtime) from `script.dropped` | none |
| AC-36 | conntrack accounting is off or the appliance tracking table is absent | `flow-export-conntrack-tracking` also reports `doctor-flowexport-conntrack-unavailable` (error, local) for `nf_conntrack_acct` 0 and for the missing tracking table, naming the cause | the module-absent case keeps its message |
| AC-37 | The conntrack reader or destroy listener did not open | `doctor-flowexport-conntrack-reader` (error, runtime) naming which reader and its error; the Info line moves to Warn with the code | none |
| AC-38 | Conntrack dump or destroy reads keep failing | the worker keeps consecutive errors and the last error per reader; `doctor-flowexport-conntrack-read-errors` (warning, runtime) | none |
| AC-39 | A collector's sends fail inside the recent window (AC-41) | `doctor-flowexport-collector-send-failed` (warning, runtime) naming each collector, its error count in the window and the last error | a collector with errors only before the window reads healthy |
| AC-40 | tc `sample` not installed on a configured sampling interface | `doctor-flowexport-sampling-setup` (error): local from the tc filters, runtime from `samplingWorker.idxToName` | none |
| AC-41 | One send fails, then sends succeed for the window | the `flow-export` health row returns to healthy; the sender keeps the last error time beside the cumulative count; journal row `ephemeral-record-stored-permanently` 2026-11-10 closes `fixed:` | the cumulative count stays in the flow-export show output |
| AC-42 | A collector config misses a protocol SHOULD | `doctor-flowexport-collector-advice` (warning, local) through `adviseProtocolCollectors`; the advisor line carries it (AC-4) | none |
| AC-43 | Selected FlowSpec rules are not in the kernel (`RegisterTables` or `ApplyAll` failed) | `doctor-flowspec-rules-not-applied` (error, runtime) naming the rule count behind | a whole table gone is child 10's `doctor-firewall-drift` |
| AC-44 | A selected FlowSpec route cannot be enforced (NLRI parse, nft expression) | `doctor-flowspec-rule-refused` (warning, runtime) from `countRuleRefused` by reason | none |
| AC-45 | The FlowSpec rule map reached `maxRulesDefault` | `doctor-flowspec-rule-limit` (error, runtime) naming the limit and refused count | none |
| AC-46 | The kernel route monitor failed or gave up | `routewatch` exposes running, gave-up and last error; `doctor-kernel-routewatch-down` (error, runtime) | none |

### Image server, TFTP, MRT, NTP

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-47 | A configured image-server `listen-interface` is not served (unresolved, bind failed, serve ended, appeared after apply) | `doctor-image-listener-down` (error, runtime, A-5) from `httpServers` against `ListenInterfaces`, naming interface and cause | the doctor component's port probe `doctor-image-listen` is its own slice's |
| AC-48 | The zefs database for the SSH user could not be built | the server keeps the last install error; `doctor-image-zefs-failed` (error, runtime; local half: temp dir writable and `buildZefsDB` over the config) | none |
| AC-49 | No `.img` in `image-directory`, or a served directory cannot be listed | `doctor-image-none` (warning, local) naming the directory | none |
| AC-50 | `build.json` is missing or names another image | `doctor-image-build-mismatch` (warning, local) naming both | none |
| AC-51 | TFTP enabled and not listening on some configured interfaces | `doctor-tftp-not-listening` (error, runtime) from the live `tftpListener` set | none |
| AC-52 | `root-directory` missing or unreadable | `doctor-tftp-root-unreadable` (error, local); the Debug "file not found" line moves to Warn with the code when the root itself fails | a request for a file that does not exist under a readable root stays Debug |
| AC-53 | The MRT write channel is full | the writer counts drops; the first drop of a run Warns with `doctor-mrt-records-dropped` and the run's end logs the count; `doctor-mrt-records-dropped` (warning, runtime) answers the count since start; journal row `warning-repeated-on-every-retry` (MRT) closes `fixed:` | none |
| AC-54 | Writing, rotating or closing an MRT file fails | `doctor-mrt-write-failed` (error): local for each output directory writable with free space, runtime from the writer's last error; the per-record Warn is bounded as in AC-53 | none |
| AC-55 | An mrt `routes` dump is configured with no RIB dump callback | `doctor-mrt-rib-dump-unavailable` (error, runtime; local half: an mrt `routes` stream and no RIB plugin configured) | none |
| AC-56 | The NTP clock is not synchronized (no server answers validly) | `ntp-client` gains a runtime declaration answered by the NTP plugin from `publishState` and each server's `Reach` and `LastError`: `doctor-clock-no-sync` (error on gokrazy, warning on a Linux host, as `clockNoSyncSeverity`) naming each server's error | before the first poll interval has passed since start, nothing is reported (R-4) |
| AC-57 | The client is enabled with no server configured and none learned from DHCP | `doctor-ntp-no-servers` (error): local from the config with no DHCP client, runtime from `syncWorker` | none |
| AC-58 | The NTP initial sync keeps failing | each server's first failure and each change of its error Warn with the code of AC-56; repeats are kept in `serverState`; "no servers configured" Warns once; journal row `warning-repeated-on-every-retry` (NTP) closes `fixed:` | none |
| AC-59 | The best server's offset exceeds `max-step` | `doctor-ntp-offset-rejected` (error, runtime) naming offset, server and `max-step` | none |
| AC-60 | Saving the time to zefs fails with `persist-path` set | the worker keeps the last persist error; the Debug line moves to Warn; `doctor-ntp-persist-failed` (warning, runtime) | none |

### Static, redistribute

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-61 | A reprogram after a BFD or link event failed | the route manager records it beside `skipped`; `doctor-static-route-stale` (error, runtime) naming route, kernel state and wanted next-hops | none |
| AC-62 | A removal or withdraw failed and the manager forgot the route | the manager keeps failed removals until a retry succeeds; `doctor-static-route-not-withdrawn` (error, runtime) naming the prefix and table | routes of another protocol in the same table are never counted (R-3) |
| AC-63 | Every next-hop of a configured route is down | `doctor-static-route-no-active-nexthop` (warning, runtime) naming the route and each next-hop's state; the silent withdraw logs at Warn with the code | none |
| AC-64 | A next-hop configured with BFD has no BFD session (`EnsureSession` refused or engine stopped) | `doctor-static-bfd-not-tracking` (error, runtime) | none |
| AC-65 | VPP is the data plane and the VPP channel did not open | `doctor-static-vpp-fallback` (error, runtime) saying routes went to the kernel FIB | none |
| AC-66 | static configured `external` with an interface next-hop | `doctor-static-external-interface-nexthop` (error, local) naming the route; fix removes the `external` entry | the no-backend case stays `doctor-static-interface-nexthop-no-backend` |
| AC-67 | Redistribution configured with no evaluator or no consumer registered | `doctor-redistribute-not-in-effect` (error, runtime) from `configredist.Global()` and `ConsumerNames()` | none |
| AC-68 | A replay to a destination was lost (emit failed or answer after the TTL) | the coordinator counts expiries and emit failures; recent event `doctor-redistribute-replay-lost` naming the destination | none |

### Traffic usage, VRRP

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-69 | A configured interface is not accounted (not found, down, attach failed) | `doctor-traffic-usage-not-attached` (warning, runtime) from `Monitor.attached` against the desired set; the silent skip logs at Warn | none |
| AC-70 | A counter read fails | the monitor keeps the last successful read time per interface; recent event `doctor-traffic-usage-read-failed` | none |
| AC-71 | Ze's eBPF program is attached on an interface Ze no longer accounts | `doctor-traffic-usage-left-attached` (warning): runtime against `Monitor.attached`, local fallback for any link carrying Ze's program (A-3) | links carrying another program are never counted (R-3) |
| AC-72 | A configured VRRP group never started | `doctor-vrrp-group-not-running` (error, runtime) from `engine.instances` against `extractGroupSpecs`, naming the cause | none |
| AC-73 | A running group kept its old parent | `doctor-vrrp-group-stale-parent` (error, runtime) naming both devices | none |
| AC-74 | A virtual router stopped because its parent is down or absent | `doctor-vrrp-parent-down` (warning, runtime) | none |
| AC-75 | A tracked interface name does not resolve | `doctor-vrrp-track-unresolved` (error, runtime) naming the name and the lowered priority | none |
| AC-76 | Master dataplane incomplete (VIP, accept-mode filter, owner filter, Solicited-Node join) | `doctor-vrrp-master-dataplane-incomplete` (error, runtime) naming each missing piece | a whole `ze_vrrp` table gone is child 10's `doctor-firewall-drift` |
| AC-77 | Backup or removed dataplane not withdrawn | `doctor-vrrp-backup-dataplane-not-withdrawn` (error, runtime) naming each leftover | none |
| AC-78 | VRRP sysctls not in effect, or not restored after the last group left | `doctor-vrrp-sysctl-unapplied` (error, runtime) naming key, wanted and kernel value | none |
| AC-79 | Master advertisement prepare or send failing | the transport keeps a tx error count and last success per instance; `doctor-vrrp-advert-send-failed` (error, runtime) | none |
| AC-80 | No IPv4 advertisement source address | `doctor-vrrp-no-source-address` (error, runtime) naming the parent | none |
| AC-81 | Another router advertises addresses this router owns | recent event `doctor-vrrp-owner-conflict` naming group and sender | none |
| AC-82 | Peer advertisements discarded | `doctor-vrrp-advert-discarded` (warning, runtime) from `ze_vrrp_packet_errors_total` by reason, naming group and reason | none |

### Provision, update server

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-83 | `ze provision` exited without removing the address it added | provision writes a record of each added address through `statestore` under a key registered in `pkg/zefs/keys.go`, and clears it on removal; `doctor-provision-address-left` (warning, local) naming interface and address | an address the operator configured is never counted (R-3) |
| AC-84 | The update server's `Serve` returned an error other than `http.ErrServerClosed` | `serve` logs it at Warn with `doctor-update-server-ended` and keeps it; `update serve` answers it as `last-error`; `doctor-update-server-ended` (warning, runtime) with fix `update serve start`; journal row `dropped-error-logged-below-warn` 2026-11-10 closes `fixed:` | a stop the operator or the daemon asked for is not reported |

### Not problem classes (each line carries the named code)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-85 | The "plugin failed" and `Run` error lines (A13: 14, A14: 8, A15: 6) | each carries `doctor-plugin-stopped`, or `doctor-plugin-setup-failed` when it fails before ready (child 12) | none |
| AC-86 | Engine and programmer faults: event-bus emit failures, `json.Marshal` of fixed structs, `trafficfeature` unavailable, SDK close at shutdown (A13); the 17 A14 sites (bridge event before configure, invalid event JSON, shutdown line encode, peer list refusal, missing encoder, malformed FlowSpec framing, kernel emit, MRT message and peer-index faults); the A15 invariants, static emit, VRRP queue-full, advert re-encode, teardown close and completion sites | each carries `doctor-internal-defect` (child 12) | none |
| AC-87 | bridgerun `fleet.go` kills a script still running when the shutdown wait ends | the line carries `doctor-plugin-stop-cleanup-late` (child 12) | none |
| AC-88 | `./le arch doctor-coverage report` over the door's packages | every site is listed covered, doctor findings included (AC-89); `TestSystemPluginsSitesCovered` | none |
| AC-89 | The existing `diagnostic.Diagnostic` findings of the area that set no `Fix` (26 on 2026-11-11, after commit 10c6647f6d made the gate judge them): `systemd/doctor.go` 12, `dhcpserver/doctor.go` 5, `crashes/doctor.go` 3, `ntp/doctor.go` 2, `ntp/doctor_linux.go` 1, `vrrp/doctor.go` 2, `vrrp/transport/doctor.go` 1; and the `routing-table plugin failed` Error line (`internal/plugins/routingtable/register.go`), which no slice listed | each finding sets `Fix` to the command or config statement that clears it, and a message that spells its fix inline drops that text, since the renderer draws `Fix` on its own line; the routingtable line carries `doctor-plugin-stopped` as AC-85's lines do | none: a finding with no `Fix` is an uncovered site (AC-88) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExternalUnsupportedDeclared` | `internal/component/plugin/registry/` (table over the 18 registrations) | AC-1 | |
| `TestForkableRuntimeChecksAnswerThroughCallback` | each forkable plugin's `doctor_test.go` | AC-2 | |
| `TestRateFeedDeclaredRuntime` | `internal/plugins/ddos/detect/doctor_test.go` | AC-6 | |
| `TestCoppConfigHalfLocalKernelHalfRuntime` | `internal/plugins/copp/doctor_test.go` | AC-7 | |
| `TestNTPClockPrivilegeReadsClockProcess`, `TestNTPSetClockErrorReported` | `internal/plugins/ntp/doctor_test.go` | AC-8 | |
| `TestGeoDNSListenerRuntime`, `TestGeoDNSProbeOnlyOnFallback` | `internal/plugins/geodns/doctor_test.go` | AC-9, AC-21 | |
| `TestStaticRouteSkippedForked` | `internal/plugins/static/doctor_test.go` | AC-10 | |
| one `Test<Code>` per AC-11..AC-84 | the owner's `doctor_test.go` | each detection goes red when removed; fix resolves (A-2) | |
| `TestFlowExportHealthRecoversAfterWindow` | `internal/plugins/flowexport/health_test.go` | AC-41 | |
| `TestNTPWarnsOncePerServerError` | `internal/plugins/ntp/ntp_test.go` | AC-58 | |
| `TestMRTDropRunWarnsOnceWithCount` | `internal/plugins/mrt/async_writer_test.go` | AC-53 | |
| `TestUpdateServerEndedDoctor` | `internal/plugins/update-cmd/cmd/serve_daemon_test.go` | AC-84 | |
| `TestSystemPluginsSitesCovered` | `internal/le/arch/doctorcoverage/` | AC-4, AC-85..AC-88 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| flow-export recent window (the exporter's own interval) | 1 interval | an error one interval old counts | N/A | an error older reads healthy |
| anomaly-shape `armedCount` against `BlastRadiusCap` | 0..cap | cap - 1 arms | N/A | cap raises AC-12 |
| FlowSpec `ruleMap` size against `maxRulesDefault` | 0..max | max - 1 accepted | N/A | max raises AC-45 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-static-route-skipped` | `test/static/` | operator sees an unprogrammed static route | |
| `doctor-copp-runtime` | `test/plugin/` | local `ze doctor` with a daemon reports copp from the daemon | |
| `show-doctor-ntp-not-synced` | `test/plugin/` | an unreachable server reports not synced, with one Warn line | |
| `show-doctor-geodns-listener` | `test/plugin/` | a serving geodns gives no bind-probe row | |
| `doctor-plugin-external-unsupported-mrt` | `test/ui/` | `external mrt` reported before start | |
| `show-doctor-dhcp-pool-exhausted` | `test/plugin/` | a full pool is reported | |
| `show-doctor-vrrp-group-not-running` | `test/plugin/` (`option=needs-linux`) | a group on a missing parent is reported | |
| `show-doctor-exabgp-script-start-failed` | `test/exabgp/` | a missing script path is reported | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | doctor reporting changes no wire a peer sees | |

## Files to Modify
- `internal/plugins/{anomaly/detect,anomaly/observe,anomaly/shape,ddos/detect,ddos/observe,ddos/flowspec,ddos/local,ddos/flowtriq}` - registrations (AC-1), records and checks
- `internal/component/firewall` - legacy-sweep runtime check (AC-11)
- `internal/plugins/{as112,dnsresolver,geodns}`, `internal/core/dnsserver` - listener status and checks
- `internal/plugins/dhcpserver` - listener, pool, relay, send records and checks; `warnLimited` code attribute
- `internal/component/config/system` - `ConsumerSelection` carries `Dropped` and the consumer limit (the check itself is the appliance child's)
- `internal/plugins/{cos,crashes,copp,connected}` - checks; copp split
- `internal/plugins/exabgp/bridgerun` - script start, stop, drop checks
- `internal/plugins/flowexport` - health row, sender, workers, checks
- `internal/plugins/{flowspec-firewall,kernel}`, `internal/core/routewatch` - checks and the routewatch status
- `internal/plugins/{imageserver,tftpserver,mrt,ntp}` - records, bounded lines, checks
- `internal/plugins/{static,redistribute}`, `internal/component/config/redistribute` - records and checks
- `internal/plugins/{trafficusage,vrrp,vrrp/transport}` - records and checks
- `internal/plugins/provision`, `pkg/zefs/keys.go` - the added-address record
- `internal/plugins/update-cmd/cmd` - the daemon update server and its YANG for `last-error`
- `internal/core/diagnostic/codes.go` - metadata for every new code
- `docs/guide/health-checks.md`, `docs/architecture/doctor-and-health-checks.md` - the new rows and the runtime moves

## Files to Create
- the `.ci` files of the Functional Tests table
- `doctor_test.go` in each owner that has none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `update serve` answer gains `last-error` in the update-cmd YANG |
| YANG validation constraints | N-A | no config leaf added |
| YANG custom validators | N-A | none |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf added |
| Functional test for new RPC/API | Yes | the `.ci` files above |
| Pipe completeness | N-A | `show doctor` already pipes |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this spec is that work; every code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | the new records are read by checks; existing metrics (`ze_vrrp_packet_errors_total`, FlowSpec refused) are read, none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `update serve` answer key, in its command page |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | child 12 owns the field and the raise route |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md`: the forkable runtime form (AC-2) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-14-system-plugins.md`. `docs/features/ai-first.md` is declared by a cited file: unaffected in meaning, it describes the code table and the `fix` field in general, which this spec uses unchanged; its dnsresolver code list is re-checked against `doctorChecks` when P4 lands |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/health-checks.md` examples |

## Implementation Steps

1. **Phase: Wiring** -- each owner registers its check returning nothing; the wiring tests fail
2. **Phase: the packages below**, in order of dependency; each lands as its own commit when its package tests pass

### Implementation Phases

Each phase is sized for one agent of about 60 tool calls. Agents edit; every test runs in the main thread, one at a time. `∥` marks phases that touch disjoint files.

| Phase | Owns | Covers | Depends on | Runs with |
|-------|------|--------|------------|-----------|
| P1 shared | the in-process-only field set in 18 registrations; the forkable runtime form; the local-fallback marker (A-3); `warnLimited` and advisor code attributes | AC-1..AC-5 | child 12 phases for the field, the codes and the raise route | - |
| P2 runtime moves | ddos rate feed, copp split, NTP privilege, geodns split, static skipped | AC-6..AC-10 | P1 | - |
| P3 anomaly and DDoS | legacy sweep, shape cap and kill switch, flow source failing, no victim, FlowSpec limiter, announce, withdraw, flowtriq, local drop | AC-11..AC-20 | P1 | ∥ P6 |
| P4 DNS and DHCP | listener status in dnsserver, DNS listener check; DHCP listener, server-identifier, relay, pool, send; dns-source publication | AC-21, AC-26..AC-32 | P2 (geodns half); the appliance child for AC-31, AC-32 | ∥ P6 |
| P5 CoS and crashes | unknown profile, QoS apply, kernel crash recovered, unharvested | AC-22..AC-25 | P1 | ∥ P7 |
| P6 ExaBGP bridge | script start, stopped, lines dropped | AC-33..AC-35 | P1 | ∥ P3 |
| P7 flow export | conntrack causes, reader, read errors, collector send, sampling, health window, advice | AC-36..AC-42 | P1 | ∥ P5 |
| P8 FlowSpec firewall and kernel | rules not applied, refused, limit; routewatch status and check | AC-43..AC-46 | P1 | ∥ P9 |
| P9 image server and TFTP | listener, zefs, no image, build mismatch, TFTP listening and root | AC-47..AC-52 | P1 | ∥ P8 |
| P10 MRT | drop counter and bounded line, write failed, RIB dump | AC-53..AC-55 | P1 | ∥ P11 |
| P11 NTP | not synced, no servers, bounded lines, offset, persist | AC-56..AC-60 | P2 (AC-8) | ∥ P10 |
| P12 static and redistribute | stale, not withdrawn, no active next-hop, BFD, VPP fallback, external interface next-hop, redistribute | AC-61..AC-68 | P2 (AC-10) | ∥ P13 |
| P13 traffic usage | not attached, read failed, left attached | AC-69..AC-71 | P1 | ∥ P12 |
| P14 VRRP group state | not running, stale parent, parent down, track unresolved, sysctl | AC-72..AC-75, AC-78 | P1 | ∥ P16 |
| P15 VRRP dataplane and adverts | master, backup, send, source, owner conflict, discarded | AC-76, AC-77, AC-79..AC-82 | P14 | - |
| P16 provision and update server | added-address record, update server ended | AC-83, AC-84 | P1 | ∥ P14 |
| P17 not problem classes | the remaining lines of the three slices | AC-85..AC-87 | child 12 codes; P3..P16 | - |
| P17b existing findings | `Fix` on the area's existing findings, the routingtable line | AC-89 | P2 (AC-8 moves the NTP privilege check first) | ∥ P17 |
| P18 coverage proof | `TestSystemPluginsSitesCovered`, the coverage report | AC-88 | all | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has its check, its code metadata and its test |
| Process boundary | every runtime check of a forkable plugin answers through `OnDoctorCheck`; none reads a package global from the daemon (R-1) |
| Correctness | no check reads a daemon-held port, cap or backend from the local process (AC-6..AC-10) |
| Ownership | "left behind" checks count only Ze's own state (R-3) |
| Naming | codes are the canonical names; JSON keys kebab-case |
| Rule: user-facing-errors | every message names problem and why; `fix` holds the command alone |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every system-plugins site covered | `./le arch doctor-coverage report` |
| every new code has metadata | `ze show diagnostic <code>` per code in the unit tests |
| four journal rows closed `fixed:` | `./le spec journal report` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | a doctor message names paths and addresses already in the config; flowtriq errors never carry the API key |
| Resource exhaustion | each new record is bounded (last error, a count, a set capped by config size) |

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

- A check in the plugin registry runs in the daemon process, so the registry now refuses one in the `runtime` phase; only the Stage 1 declaration answered by the plugin survives a fork. The phase a Stage 1 declaration carries is a label: `collectDoctorChecksFrom` calls it on the daemon route whatever it says, and the local route never calls it

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `ephemeral-record-stored-permanently.md` | 2026-11-10 | flow-export health row reads a send-error counter that never resets |
| `warning-repeated-on-every-retry.md` | 2026-11-10 | NTP initial sync Warns per server per one-second retry |
| `warning-repeated-on-every-retry.md` | 2026-11-10 | MRT Warns once per dropped or failed record |
| `dropped-error-logged-below-warn.md` | 2026-11-10 | daemon update server ends with no log line |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Runtime checks of forkable plugins answer through `OnDoctorCheck` | a `registry.DoctorCheckDef` at `runtime` reading package state | the registry form runs in the daemon and reads nothing of a forked plugin (system door) |
| External-unsupported is one field per registration, child 12's check | a list of plugin names in a check | a list is a central enumeration (`ai/rules/principles.md`) |
| Split copp and geodns into local and runtime halves | move whole to runtime | the config-parse half and the pre-start probe answer something true without a daemon |
| Bound the NTP and MRT lines by first-of-run plus count | a fixed interval as `warnLimited` | a change of error is new information and Warns at once |
| Health row judges a recent window | reset the counter on read | the cumulative count is useful in the show output; the window answers "failing now" |

## Known Limitations
- The TFTP server refuses write requests ("write not supported", `tftpserver/handler.go`) while `ai/META-DECISIONS.md` puts uploads in scope: an RFC gap, not a doctor class. It is homed in `plan/post-release/features/spec-tftp-server-uploads.md`; no AC here
- The doctor component's `listeners` check bind-probes the image and NTP ports a running daemon holds; that is the doctor component's slice, not this spec

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-89 all demonstrated
- [ ] Every user story has a working path and a passing test (Scope plugin: the stories are the Wiring Test rows)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests (one package at a time through `./le job run`), `./le test functional plugin`, `./le test functional static`, `./le test functional exabgp`, `./le test functional ui`, and the suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change a peer sees)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-14-system-plugins.md` only, in the same `./le commit create` script
