# Spec: doctor-coverage-15-system

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate`: the `runtime` doctor phase, `Diagnostic.Fix`, the `recent-events` check and the doctor-coverage gate, landed in commits ce58dddf53 and fc96556110. `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes, the event-code registration through `doctor-recent-events`, the convention that a report-bus raise carries its own fix in an `Issue` fix field) and phase P0b (the report-bus raise route for a forked plugin): this spec builds none of them. `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`: the route field on `DoctorCheckContext`, the reload outcome record (its AC-15 reports that a system reload step failed; the step's own detail is here), and the conversion of the ad hoc `fix`, `remedy` and `check` log attributes of `cmd/ze/hub` (its AC-10). `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-72: the firewall IRR stale class keeps `doctor-firewall-irr-stale-data`, the firewall answer of this spec's AC-19. `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` phase P6 (AC-34, AC-36, AC-42, AC-43): the `filter_irr` runtime check (child 7 AC-42, AC-43, declared in the runner's `sdk.Registration` and answered by `OnDoctorCheck`) is the one this spec's AC-19 BGP row joins, so the plugin keeps one runtime check; whichever spec lands first creates it. Child 7's local rows (AC-34, AC-36) are config-phase and cannot share it. `plan/release/validation/spec-doctor-coverage-5-appliance.md` builds its update-state codes on this spec's AC-3 rename and cites AC-27 for the self-update validator; this spec does not depend on it |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the system section of the audit (`plan/audits/doctor-problem-inventory/system.md`, slice A4) and the index `plan/audits/doctor-problem-inventory.md` ("Canonical codes", AC-9 rows 13 and 14, "Not problem classes", "Defects found" 7, 9, 10 and 11).

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

`ze doctor` and `show doctor` report every problem class of the system area
(host resources, disk SMART, the system resolver, the IRR and RIR caches, the
policy IGP-import check, GTSM kernel state), each with its cause, severity,
doctor code and the command that fixes it. Every Warn and Error line and every
report-bus raise in the area names a doctor code that a registered check
declares (O-2), so the doctor-coverage gate counts it covered.

| # | Work | Rows |
|---|------|------|
| 1 | Report-bus codes that are not doctor codes are renamed to their canonical doctor codes and registered: storage `temp-rising`, `temp-high`, `temp-critical`, `smart-failing`; host `ecc-error`, `throttle`, `carrier-change`; config/system `self-update`, `update-available` | AC-1 to AC-3 |
| 2 | Storage: SMART health, temperature, enable failure, self-test, and SMART config that the commit accepts and the manager replaces by a default | AC-4 to AC-9 |
| 3 | Host: ECC, throttling, carrier, sensor alarm and unreadable inventory, tuning not in effect, and the host monitor started from the host side, not from BGP config | AC-10 to AC-16 |
| 4 | Resolve: DNSSEC failure in permissive mode, truncated answers, the IRR prefix cache and the stored RIR delegation table | AC-17 to AC-24 |
| 5 | AC-9 of child 1: the policy IGP-import check moves to `runtime`; the GTSM check splits | AC-25, AC-26 |
| 6 | The self-update config the commit accepts and the backend refuses | AC-27 |
| 7 | Lines whose class is covered elsewhere, and the not-problem group: each carries its code | AC-28, AC-29 |

Covered rows, no AC: config partition space and clock (`disk-space`,
`clock-skew`, `internal/component/doctor/doctor_checks.go`); configured DNS
server unreachable (`dns-resolvers`, `doctor-dns-resolver`,
`internal/component/config/system/doctor.go`; its two resolver lines get the
code in AC-28); policy plugin stopped (`doctor-plugin-stopped`, child 12).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the process boundary, registration, the doctor check as the readiness answer of the package that owns the dependency
  → Constraint: "A plain Go call into another component's package is the classic defect"; a check that reads a plugin's state (the IRR stores) is declared by that plugin, so it runs in the plugin's process on both transports
  → Constraint: a readiness check is "A doctor check registered by the package that owns the dependency"
- [ ] `ai/doors/core.md` - registries and runtime state
  → Constraint: "With no engine linked, it answers `ErrNotRegistered`, never an empty list"; every runtime accessor here answers "not published" when its owner never started, never an empty healthy set
  → Constraint: "Code that copies what a registry holds into a literal or a switch is refused"; the host event kinds raise their own constant codes, no code is rebuilt from a value
- [ ] `ai/doors/hub-engine.md` - boot and reload
  → Constraint: "Something built only at startup ignores every reload"; the host monitor and the SMART manager are started and reconfigured on the same path
  → Constraint: "A check that runs only at start passes validate"; the SMART and self-update config refusals move to verify
- [ ] `ai/doors/appliance.md` - the update backend
  → Constraint: "System update is one registered backend"; the self-update validation stays in `internal/component/config/system`, reached from verify
- [ ] `ai/doors/bgp-engine.md` - BGP compiles out with `ze_bgp`
  → Decision: a host feature started from `internal/component/bgp/config/loader_create.go` is absent from a build or a config without BGP; the start moves out of BGP
- [ ] `ai/doors/bgp-plugins.md`, `ai/doors/forwarding.md` - the two IRR store owners
  → Constraint: "The report bus is process-local too"; a forked IRR owner reports through its own doctor check or the child 12 P0b route, never a direct `report.RaiseWarning`
- [ ] `ai/doors/plugin-infra.md` - plugin doctor checks
  → Constraint: a plugin check over the plugin's live state (an IRR store's in-memory status) is `runtime`, declared in the `sdk.Registration` `DoctorChecks` its runner passes to `p.Run` and answered by `OnDoctorCheck`, which `show doctor` calls through `collectDoctorChecksFrom` on both transports (`internal/plugins/ddos/detect/doctor.go` is the reference); a `registry.DoctorCheckDef` carries a config phase only and runs in whichever process answers, and `validateDoctorCheckDef` refuses a `runtime` one (149497ce12). Neither IRR runner (`firewall/plugins/irr/irr.go`, `bgp/plugins/filter_irr/filter_irr.go`, their `p.Run` calls) declares a doctor check or sets `OnDoctorCheck` today
- [ ] `ai/doors/commands-cli.md` - the `show warnings` and `show errors` readers
  → Constraint: "A handler answers data, never text"; the renamed code reaches every surface through the same handlers
- [ ] `ai/doors/system-plugins.md`, `ai/doors/le-tooling.md` - `internal/plugins/update-cmd` (preserved JSON key) and the `.ci` files
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux`
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases and owners
  → Constraint: a `runtime` check receives only `Platform` and reads its owner's live state (child 1, Data Flow amendment 1b)
- [ ] `docs/architecture/storage/smart-health.md` - the SMART manager and its report-bus codes
  → Constraint: the page names `temp-high`; it is edited with the rename
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: the fix is the command alone, on its own line (`Diagnostic.Fix`)

**Key insights:**
- `locrib.Default()` builds an empty Loc-RIB in a process with no plugin hub token, so the local route reads an empty RIB and reports nothing: the policy check can only answer inside the daemon
- `gtsm` `checkKernelState` already answers two questions by process (`publishedPeers` `inDaemon`); the split puts each question in the phase whose route can answer it
- the host diff events carry a code chosen at run time (`DiffEvent.Code`), which the gate reads as code-dynamic
- the SMART YANG already bounds every numeric leaf; the extractor still refuses forms the YANG accepts (a one-digit bare interval, an interval past the integer range)
- the BGP `filter_irr` store is built with no persistence (`store.New(irrClient, pdb, nil)`), so only the firewall store persists entries

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/storage/manager.go` - `checkDevice` runs `enableOnce`, `checkTemperature`, `checkHealth`, `checkSelfTest`; raises `temp-rising`, `temp-high` (warnings), `temp-critical`, `smart-failing` (errors); `enableOnce` Warns "SMART enable failed" with no code; `checkSelfTest` drops the error of `smart.StartSelfTest` and records nothing
- [ ] `internal/component/storage/config.go` - `Config`, `TemperatureConfig`, `SelfTestSchedule`, `DefaultConfig`; no parser
- [ ] `internal/component/storage/doctor_linux.go` - `storage-smart` (post-config) declares `doctor-smart-sysfs`, `doctor-smart-access`: access only, no health, temperature or self-test row
- [ ] `internal/component/storage/yang/ze-storage-conf.yang` - `check-interval` range 60..86400; temperatures uint8 1..100 and 1..50; self-test `interval` pattern `[1-9][0-9]*[hdms]?` ("A bare number is read as seconds"); `time` pattern HH:MM; `day` enumeration; the `critical` description says critical at or under informational hides the informational warning
- [ ] `cmd/ze/hub/main_system.go` - `extractSmartConfig` Warns "invalid ..., using default" on nine leaves and keeps the default; `parseDuration` refuses a string shorter than two characters, so the one-digit bare interval the YANG accepts falls back to the default; `applyHostTuning` and `applyHostTuningFromMap` Warn "tuning failed" with no code; `startBackend` Warns "invalid config" and starts no update backend; `startStandaloneTelemetry` sets the metrics registry and starts no host metrics
- [ ] `internal/component/config/system/selfupdate_validate.go` - `validateSelfUpdateConfig` refuses immediate with time and a bad HH:MM; its only callers are the two `NewBackend` builds (`backend_ze_distro.go`, `backend_ze_appliance.go`)
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `restart time` is a bare string; both leaf descriptions say "Ze refuses a configuration that carries both immediate and time", which the commit does not do
- [ ] `internal/component/config/system/selfupdate.go`, `update.go` - `selfUpdateCode` `self-update`, `updateCode` `update-available` on the report bus
- [ ] `internal/component/host/metrics.go` - `CollectOnce` returns on a detect error with no record; `emitDiffEvents` raises `ecc-error` as an error and every other kind as an active warning with `e.Code`, never cleared
- [ ] `internal/component/host/diff.go` - `diffNICCarrier` (`carrier-change`, up and down alike), `diffECC` (`ecc-error`), `diffThrottle` (`throttle`)
- [ ] `internal/component/host/inventory.go` - `Inventory.Errors`, `ThermalInfo.Errors`, `SensorReading.Alarm`, `MemoryInfo` ECC counters
- [ ] `internal/component/host/register.go` - `init()` registers the doctor platform names only
- [ ] `internal/component/bgp/config/loader_create.go` - the only `host.RegisterMetrics` and `StartRefresh` call, inside the BGP loader, only when `metrics.StartExporter` returns a registry
- [ ] `internal/component/resolve/dns/resolver.go` - two Warns when a server fails ("did not answer", "answered a server failure"); `queryServer` Warns "truncated DNS response" and uses the answer; the permissive DNSSEC Warn
- [ ] `internal/component/resolve/irr/store/store.go` - stale Warns when a refresh learned nothing; PeeringDB failure Warns (cached AS-SET kept, or the ASN queried); `Open` skips an unreadable or undecodable entry silently and Warns on a name/key mismatch; persist encode and write Warns; purge removal Warn
- [ ] `internal/component/resolve/irr/stored.go` - `delegationTable` Warns "passed over"; `storedDelegationFile` Warns when the store cannot be opened or the key read
- [ ] `internal/component/firewall/plugins/irr/doctor.go` - `checkIRRDataFreshness` (`firewall-irr-data-freshness`, post-config, registered with `diagnostic.RegisterDoctorCheck` from `register.go`) builds its own store over a read-only open of the persisted storage, so it sees only what was persisted, never the running plugin's memory, and reports `doctor-firewall-irr-stale-data`, `-no-data`, `-oversized-data`; codes registered by the plugin (`irrDiagnosticCodes`)
- [ ] `internal/component/bgp/plugins/filter_irr/filter_irr.go` - one `store.PrefixStore` for the plugin's life, built with nil persistence; no doctor check
- [ ] `internal/component/policy/doctor.go` - `policy-igp-import-hides-nexthop` post-config reads `locrib.Default()`; its action text is in `Help`
- [ ] `internal/core/rib/locrib/default.go` - `Default` builds a RIB lazily in any process without the plugin hub token
- [ ] `internal/component/gtsm/doctor.go` - `gtsm-kernel-state` post-config; in the daemon `installedStateDiagnostics`, locally `resolvableRouteDiagnostics`; the restart command sits inside `Message`
- [ ] `internal/core/report/report.go` - `Issue` (no fix field yet), `RaiseWarning`, `ClearWarning`, `RaiseError`
- [ ] `internal/component/cmd/show/show.go` `handleShowWarnings`, `internal/component/doctor/checks_recent_events.go` `checkRecentEvents` - the in-tree readers of `report.Warnings` and `report.Errors`
- [ ] `internal/core/diagnostic/codes.go` - `CodeDoctorPolicyIGPImportHidesNextHop`, the DNS resolver codes, `doctor-dns-resolver`

**Behavior to preserve:**
- every existing check, its name and code; the SMART manager's poll and its raise/clear pairing; the `ze_host_*` Prometheus gauges and their names; the resolver's fallback order; the IRR store's keep-cached rule; the GTSM peer derivation; the policy check's verdict logic
- `show storage smart`, `show host`, `show firewall irr`, `show bgp irr` output

**Behavior to change:**
- the nine bus codes above take their doctor names
- a SMART or self-update config the backend cannot read is refused at commit, not replaced or dropped
- the host monitor runs on every box, BGP or not
- the area's problem states become doctor rows with a fix line

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `storage-smart` check | `internal/component/storage/doctor_linux.go` `smartDoctorCheck` | post-config SMART access rows | extends: adds the health and temperature rows from the same `smartDetect` walk (AC-4, AC-5) |
| SMART manager status | `internal/component/storage` `Manager`, `DeviceStatus`, `SetStorageManager` | live per-device state behind `show storage smart` | extends: a `runtime` check reads it through the published manager; `DeviceStatus` gains the last self-test start error (AC-6, AC-7) |
| SMART config type | `internal/component/storage/config.go` `Config`, `DefaultConfig` | the parsed form; the parser lives in the hub | extends: the parser moves beside the type, used by verify and the hub (AC-9) |
| Host inventory | `internal/component/host` `Detector.Detect`, `CachedDetector`, `SetGlobalCachedDetector` | sysfs inventory with per-section errors | extends: the local host checks read it (AC-10 to AC-14) |
| Host diff events | `internal/component/host/diff.go` `DiffInventory`, `metrics.go` `emitDiffEvents` | change events on the report bus | extends: constant doctor codes per kind; carrier down and throttle become ring events rather than never-cleared warnings (AC-2) |
| `dns-resolvers` check | `internal/component/config/system/doctor.go` | probes each configured server, `doctor-dns-resolver` | extends: the DNSSEC probe joins it (AC-17); the server-failure lines carry its code (AC-28) |
| Firewall IRR check | `internal/component/firewall/plugins/irr/doctor.go` `checkIRRDataFreshness` | per-reference stale, no-data and oversized rows | extends: declares and appends the corrupt store rows, which the persisted store shows (AC-21); its per-reference rows stay, and its stale row is the firewall answer of AC-19. The unpersisted rows (AC-20) live in the running plugin's store, so they come from a new runtime check the firewall `irr` runner declares in its `sdk.Registration` and answers with `OnDoctorCheck` |
| `rir-delegation-sources` check | `internal/component/resolve/cmd/doctor_rir.go` | configured source URLs refused | extends: a sibling row for the stored table (AC-23) |
| `policy-igp-import-hides-nexthop` | `internal/component/policy/doctor.go` | post-config scan of `locrib.Default()` | extends: moves to `runtime` (AC-25) |
| `gtsm-kernel-state` | `internal/component/gtsm/doctor.go` | one post-config check answering two questions by process | replaces: two checks, one per phase, each answering one question (AC-26, `ai/rules/no-layering.md`: the process branch is deleted) |
| Doctor phases and `runtime` | child 1, `internal/core/diagnostic/doctor_registry.go`, `RunRuntimeChecks` | daemon-only checks with `Platform` only | extends: AC-6, AC-7, AC-13, AC-25, AC-26 register here |
| `Diagnostic.Fix` | child 1, `internal/core/diagnostic/types.go` | the fix command per finding | extends: every row here fills it |
| Report bus and fix field | `internal/core/report/report.go`; child 12 P0a (`Issue` fix field) | coded active warnings and an error ring | extends: every raise here carries a doctor code and a fix (AC-1 to AC-3) |
| Event-code registration and `recent-events` | child 12 P0a; `internal/component/doctor/checks_recent_events.go` | a transient code registered by its owner shows as a recent-event row | extends: the transient codes here register through it (AC-1 to AC-3, AC-18, AC-22) |
| Plugin report route | child 12 P0b | a forked plugin raises on the daemon bus | extends: the IRR owners raise their store events through it when forked (AC-22) |
| Check-context route field | child 11 | a check knows whether it runs on the daemon or the local route | does not apply: the GTSM split removes the need, each half answers the same on both routes (AC-26) |
| Reload outcome record | child 11 AC-15 | a reload step that failed | extends: tuning and SMART reload steps return their failure to it; the per-step rows are here (AC-15, AC-9) |
| Doctor-coverage gate | child 1, `internal/le/arch/doctorcoverage` | refuses an added uncovered site | extends: every line here becomes covered; no gate change |
| Config verify | `docs/architecture/config/transaction-protocol.md`; `ai/rationale/exact-or-reject.md` | a backend that cannot apply a config refuses it at verify | extends: the SMART extractor runs at verify (AC-9); `validateSelfUpdateConfig` runs from a tree validator (`internal/component/config/tree_validators.go` `RegisterTreeValidator`), so validate, commit and boot refuse together (AC-27) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor` (local route: pre-config, parsed-config and post-config checks) and `show doctor` on every daemon surface (adds the `runtime` checks, the plugin checks and `recent-events`)
- the config commit (verify) for AC-9 and AC-27

### Transformation Path
1. A local check reads host state (sysfs, SMART, the stored zefs keys) and the config tree, and answers diagnostics with `Fix`
2. A `runtime` check reads its owner's published live state (SMART manager, host monitor record, GTSM published peers, the Loc-RIB); with nothing published it answers that the owner did not start, never an empty healthy set
3. An IRR store owner's plugin check (firewall `irr`, BGP `filter_irr`) asks its own store for a value-type status and appends the store's diagnostics, in the owner's process
4. A transient class raises on the report bus under its doctor code with its fix (child 12 P0a); `recent-events` renders it
5. Verify runs the SMART extractor and the self-update validator; a refusal names the leaf and the value

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to doctor (IRR owners, live store) | `sdk.Registration` `DoctorChecks` answered by `OnDoctorCheck`, called by `collectDoctorChecksFrom` over DirectBridge or RPC | No: neither IRR runner declares one yet |
| Firewall IRR persisted store to doctor | the existing `diagnostic.RegisterDoctorCheck` check reading the store read-only, in whichever process answers | Yes, `checkIRRDataFreshness` |
| Forked plugin to report bus | child 12 P0b | No |
| Hub to host and storage | the hub's system start calls the host monitor start and the SMART manager, as today for SMART | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms the nine bus code strings (`"temp-rising"`, `"temp-high"`, `"temp-critical"`, `"smart-failing"`, `"ecc-error"`, `"throttle"`, `"carrier-change"`, `"self-update"` as a code, `"update-available"` as a code), `selfUpdateCode`, `updateCode`, `report.Warnings()`, `report.Errors(`. Positive control: the raises in `storage/manager.go` and `host/diff.go`; the reader `handleShowWarnings`. Findings: no Go health filter or `HealthProbe` keys on any of the nine codes; the in-tree readers are `handleShowWarnings` and the errors handler (`internal/component/cmd/show/show.go`), `checkRecentEvents`, `internal/component/storage/manager_test.go` and `test/ui/doctor-gokrazy-update-check.ci`; the YANG descriptions in `ze-storage-conf.yang`, `docs/architecture/storage/smart-health.md`, the update pages under `docs/architecture/appliance/` and `docs/guide/self-update.md` name the codes in prose and are edited with the rename. `"update-available"` in `internal/plugins/update-cmd/cmd/show.go` and `firmware.go` is a JSON key of the show answer, not a bus code, and is preserved.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| report-bus `Issue.Code`, storage | writer | `internal/component/storage/manager.go` `checkTemperature`, `checkHealth`, `poll` | raise and clear literals | change to the `doctor-storage-*` codes, raise and clear together |
| report-bus `Issue.Code`, storage | reader | `handleShowWarnings` and the errors handler (`internal/component/cmd/show/show.go`), `checkRecentEvents`, `storage/manager_test.go`; API clients of `show warnings` and `show errors` outside the repository | they list `report.Warnings` and `report.Errors`; the test asserts the old literals | the test is converted; outside clients read the new value (pre-release, `ai/META-DECISIONS.md`, "Ze is pre-release"; no alias) |
| report-bus `Issue.Code`, host | writer | `internal/component/host/diff.go`, `metrics.go` `emitDiffEvents` | `DiffEvent.Code` | change to constant `doctor-host-*` codes per kind |
| report-bus `Issue.Code`, host | reader | `handleShowWarnings`, the errors handler, `checkRecentEvents`; outside API clients | the same readers | new value; no in-tree test asserts the old host literals |
| report-bus `Issue.Code`, update | writer | `internal/component/config/system/selfupdate.go` `selfUpdateCode`, `update.go` `updateCode` | constants | change to `doctor-self-update-waiting`, `doctor-update-available` |
| report-bus `Issue.Code`, update | reader | `handleShowWarnings`, `checkRecentEvents`, `test/ui/doctor-gokrazy-update-check.ci`; outside API clients | the `.ci` reads the update notice | the `.ci` is converted in P1 |

### Integration Points
- `cmd/ze/hub/main_system.go` system start and reload: starts the host monitor beside `startSmartManager`; returns step failures to child 11's reload record
- the two IRR owner plugins' doctor checks

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | checks run through the doctor registry and plugin checks; raises through the report bus |
| No unintended coupling (components stay isolated) | Yes | the host start leaves the BGP loader; IRR store rows reach doctor through their owners, not an import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; the GTSM process branch and the hub's SMART parser are deleted, not doubled |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path |
| Registration over hardcoding, outbound | Yes | each owner registers its check and code metadata from its own `init()` |
| Registration over hardcoding, inbound | Yes | searched `internal/component/doctor` and `internal/core/diagnostic/codes.go` for lists naming storage, host, irr or gtsm checks: none; new code metadata registers from its owner, as `irrDiagnosticCodes` does |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The storage component can register a verify of `storage smart` that the commit runs | `docs/architecture/config/transaction-protocol.md`, verify stage | AC-9 needs a different seam | read the storage root's config owner in P3 before code | unvalidated |
| A-2 | `internal/core/smart` can read the device self-test log (ATA and NVMe) | `smart.StartSelfTest`, `IsSelfTestInProgress` exist | AC-8 needs a reader added to `smart` | read `internal/core/smart` in P3 | unvalidated |
| A-3 | The BGP `filter_irr` plugin owns its own `store.PrefixStore`, with no persistence | `filter_irr.go`: `store.New(irrClient, pdb, nil)`, one store for the plugin's life | AC-19 and AC-22 name the wrong owner | read in this spec's research; re-read in P4 | unvalidated |
| A-4 | Child 12 P0a lands before P1 here | child 12 Depends row | the renames cannot register as event codes | check child 12 status before P1 | unvalidated |
| A-5 | A signed name with a known chain (the root zone) is answerable by any validating upstream | DNSSEC root trust anchor | the DNSSEC probe reads a false negative | probe against a validating and a non-validating resolver in the AC-17 test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Thermal throttle counters are non-zero on many healthy laptops and VMs | a Warning on every dev box | AC-11 reports the local count as Info and only a rise since the last refresh as an event |
| R-2 | The firewall per-reference rows and the store rows report one stale entry twice | duplicate rows | resolved in AC-19: one stale code per owner, each declared by one check; the firewall keeps `doctor-firewall-irr-stale-data` (child 10 AC-72) and gains no store stale row |
| R-3 | Moving the host monitor out of BGP changes the order gauges register | metrics test reds | the gauges register when a metrics registry is published; names unchanged |
| R-4 | Children 7 and 10 touch the IRR plugins | hunks in the same files | this spec edits only the doctor check of each owner; coordinate at P4 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | doctor rows and bus codes; a SMART or self-update config refused at commit that was accepted before |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | child 11 (`cmd/ze/hub/main_system.go`, `main_reload.go`), child 10 (firewall IRR), child 7 (BGP `filter_irr`), child 12 (report bus) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a SMART device reporting FAILING | → | `storage-smart` health row | `TestSmartDoctorReportsFailingHealth` |
| `show doctor` with SMART enable failed on a device | → | storage runtime check | `test/plugin/show-doctor-storage-smart-runtime.ci` |
| commit of `storage smart self-test short interval 5` | → | SMART extractor at verify | `test/parse/storage-smart-interval-one-digit.ci` |
| `ze config validate` with restart immediate and time | → | `system-self-update` tree validator running `validateSelfUpdateConfig` | `test/parse/self-update-restart-exclusive.ci` |
| `ze start` without a `bgp` block, then `show errors` after an ECC rise | → | host monitor | `TestHostMonitorStartsWithoutBGP` |
| `ze doctor` with an hwmon alarm | → | host sensor check | `TestHostDoctorReportsSensorAlarm` |
| `show doctor` with an IRR entry unpersisted | → | firewall `irr` check store rows | `TestIRRDoctorReportsUnpersistedEntry` |
| `show doctor` with an IGP import hiding a next hop | → | policy runtime check | `test/plugin/show-doctor-policy-igp-hidden.ci` |
| `ze doctor` and `show doctor` with a GTSM peer | → | the two GTSM checks | `TestGTSMChecksSplitByPhase` |

## Acceptance Criteria

Every row: severity, code, the fix line, the state read, and the route. "Event" means a report-bus raise rendered by `recent-events` under the named code, carrying its fix (child 12 P0a). A code marked (new) is registered with metadata by its owner from `init()`.

### Report-bus codes (P1)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The SMART manager raises or clears for temperature or health | the bus carries `doctor-storage-temp-rising`, `doctor-storage-temp-high` (warnings), `doctor-storage-temp-critical`, `doctor-storage-smart-failing` (errors), each with fix `show storage smart`; every clear names the same code as its raise; `ze show diagnostic <code>` explains each; the gate counts the raises covered | none: no release shipped the old names |
| AC-2 | The host monitor finds an ECC rise, a throttle rise, or a carrier going down | each diff kind raises its own constant code: `doctor-host-ecc-errors` (error event, fix `show host`), `doctor-host-thermal-throttle` (error-ring event, fix `show host`), `doctor-host-carrier-change` (error-ring event, fix `show interface <name>`); no code is passed as a run-time value; none is left as an active warning that nothing clears | a carrier going up raises nothing and logs at Info, because it reports nothing wrong |
| AC-3 | The self-update backend or the update checker raises its notice | the bus carries `doctor-self-update-waiting` and `doctor-update-available` (renamed from `self-update`, `update-available`), registered through the event-code registration, each with the fix its message already names; `test/ui/doctor-gokrazy-update-check.ci` and the update pages read the new names | the `update-available` JSON key of `show system update` stays: it is an answer field, not a code |

### Storage (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | `storage smart enabled`, a whole device whose SMART query answers health FAILING | `storage-smart` (post-config, local) reports Error "disk `<dev>`: SMART health status is FAILING, the disk may fail", code `doctor-storage-smart-failing`, fix `show storage smart`; the check declares the code | a device whose SMART is unavailable keeps the existing `doctor-smart-access` row |
| AC-5 | A device whose SMART temperature is at or above `informational` or `critical` | `storage-smart` reports Warning `doctor-storage-temp-high` or Error `doctor-storage-temp-critical` naming the device, the reading and the threshold, fix `show storage smart`; a temperature rising faster than `difference` between polls is the AC-1 event `doctor-storage-temp-rising`, declared by the storage runtime check of AC-6 | a reading of 0 (no sensor) reports nothing, as the manager does |
| AC-6 | The running SMART manager could not enable SMART on a device | runtime check `storage-smart-runtime` reads the published manager's `DeviceStatus` and reports Warning "disk `<dev>`: SMART could not be enabled: `<error>`", code `doctor-storage-smart-enable-failed` (new), fix `show storage smart`; the "SMART enable failed" Warn line carries the code; with no manager published while the running config enables SMART it reports Warning "the SMART manager is not running", same code, fix `ze signal reload` | SMART disabled in config reports nothing |
| AC-7 | A scheduled self-test did not start (`smart.StartSelfTest` failed) | the manager records the error and its time in `DeviceStatus` and logs Warn with code `doctor-storage-self-test-not-started` (new); `storage-smart-runtime` reports Warning naming the device, the test kind, the error and the time, fix `show storage smart` | a device already running a test is skipped, as today |
| AC-8 | The last completed self-test of a device failed | `storage-smart` (local) reads the device self-test log and reports Error "disk `<dev>`: the last `<kind>` self-test failed at `<time>`", code `doctor-storage-self-test-failed` (new), fix `show storage smart` | a device with no completed test reports nothing |
| AC-9 | A commit sets a `storage smart` leaf the YANG accepts and the extractor cannot read (a one-digit bare `interval`, an interval past the duration range) | one extractor in `internal/component/storage/config.go` reads every form the YANG accepts ("A bare number is read as seconds") and refuses at verify any value it cannot hold, naming the leaf and the value; the hub calls the same extractor, so the nine "invalid ..., using default" Warn lines are deleted; a reload whose SMART step fails returns it to child 11's reload record | `critical` at or under `informational` stays accepted, as the YANG description documents; `storage-smart` reports it Warning "the critical threshold hides the informational warning", code `doctor-storage-smart-config-invalid` (new), fix `set storage smart temperature critical <value above informational>` |

### Host (P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-10 | Host inventory shows ECC counters above zero | local check `host-hardware` reports Error for uncorrectable and Warning for correctable errors, the counts named, code `doctor-host-ecc-errors`, fix `show host`; a rise at run time is the AC-2 event | a host without ECC reports nothing |
| AC-11 | Host inventory shows thermal throttle counters above zero | `host-hardware` reports Info with the per-CPU counts, code `doctor-host-thermal-throttle`, fix `show host`; a rise at run time is the AC-2 event (R-1) | none |
| AC-12 | An hwmon sensor's alarm is set | `host-hardware` reports Warning "sensor `<name>` (`<device>`) alarm at `<temp>`", code `doctor-host-sensor-alarm` (new), fix `show host` | none |
| AC-13 | An inventory section or an hwmon channel cannot be read (`Inventory.Errors`, `ThermalInfo.Errors`) | `host-hardware` reports Warning naming the section or path and the error, code `doctor-host-inventory-unreadable` (new), fix `show host`; `CollectOnce` records a detect error (time, error) and runtime check `host-monitor` reports it under the same code; with no monitor published it reports Warning "the host monitor is not running", same code, fix `ze signal restart` | none |
| AC-14 | `system tuning` names a CPU governor, IRQ affinity or ethtool setting the host does not hold | local check `host-tuning` compares the configured values with `cpufreq/scaling_governor`, `/proc/irq/<n>/smp_affinity` and the ethtool read, and reports Warning per mismatch naming the setting, the wanted and the held value, code `doctor-host-tuning-not-applied` (new), fix `ze signal reload` | a setting the host has no file for (no cpufreq) reports Warning under the same code with the absent path, because the setting cannot take effect |
| AC-15 | Tuning apply fails at start or reload | the two "tuning failed" Warn lines carry `doctor-host-tuning-not-applied`; the reload step returns the failure to child 11's reload record | none |
| AC-16 | A box with no `bgp` block, or a build without `ze_bgp` or `ze_telemetry` | the host monitor (inventory refresh and diff events) starts from the hub's system start beside the SMART manager and is reconfigured by the same reload path; the `ze_host_*` gauges register whenever a metrics registry is published; the BGP loader no longer starts it | none: a host feature does not depend on BGP config |

### Resolve (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-17 | The system resolver validates DNSSEC (permissive or strict) and an upstream answers a signed name without the authenticated-data bit | `dns-resolvers` (local) sends one DO query for the root zone to each configured server and reports Warning "DNS server `<addr>` does not validate DNSSEC: permissive mode answers unvalidated data", code `doctor-dns-dnssec-unvalidated` (new), fix the `set` command of the DNSSEC mode leaf taken from the resolver's YANG path; the permissive Warn in `queryServer` carries the code | DNSSEC off reports nothing |
| AC-18 | A DNS answer arrives truncated | `queryServer` raises the event `doctor-dns-answer-truncated` (new) naming the name, type and server, fix `show dns cache`; the Warn line carries the code | none |
| AC-19 | An IRR store entry is stale (a refresh learned no prefixes, the cached ones kept) | the class has one code per owner, each declared and answered by exactly one check, because each owner holds its own `store.PrefixStore` (A-3). Firewall: the existing `doctor-firewall-irr-stale-data`, declared and answered only by the firewall `irr` check (`checkIRRDataFreshness` already reads `StaleSince` for every configured reference; `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-72 keeps it and codes the firewall's lines); this spec adds no firewall stale row. BGP: `doctor-bgp-irr-stale-data` (new), declared and answered only by the BGP `filter_irr` runtime check (in the runner's `sdk.Registration`, answered by `OnDoctorCheck`, shared with child 7 AC-42 and AC-43), because its store lives only in the plugin's memory, Warning "IRR `<name>` has kept cached prefixes since `<time>`", fix `update bgp irr as-set <name>`. The two shared store Warn lines (one per address family) leave `store.go`: the store reports the family that learned nothing in the entry it already returns, and each owner's refresh caller logs that Warn under its own code (firewall `irr.go` under `doctor-firewall-irr-stale-data`, BGP `filter_irr.go` under `doctor-bgp-irr-stale-data`), so no line in shared code names one owner's code | none: no `doctor-irr-cache-stale` code is created, and no store-level stale row is added (R-2) |
| AC-20 | A firewall IRR entry was not persisted (encode or write failed) or a purge did not remove the stored key | the store records each in its status; a runtime check the firewall `irr` runner declares in its `sdk.Registration` and answers with `OnDoctorCheck` reads that status and reports Warning naming the entry and the error, code `doctor-irr-cache-unpersisted` (new), fix `update firewall irr all`; the write and purge Warn lines carry the code | the encode failure of a fixed struct carries `doctor-internal-defect` (child 12), because it is a Ze defect; the BGP store persists nothing, so it reports no persist row |
| AC-21 | A persisted IRR entry is unreadable, undecodable or names another key | `Open` returns the skipped keys with their reason instead of a silent `continue`; the firewall `irr` check reports Warning per key, code `doctor-irr-cache-entry-corrupt` (new), fix `clear firewall irr as-set <name>`; the mismatch Warn carries the code | none |
| AC-22 | A PeeringDB lookup fails | the store raises the event `doctor-irr-peeringdb-lookup-failed` (new) naming the ASN and whether the cached AS-SET or the bare ASN was used, through the child 12 P0b route when the owner runs forked; the two Warn lines carry the code | none |
| AC-23 | The stored RIR delegation table cannot be opened, read or parsed | local check `rir-delegation-stored` (resolve) runs `preferStoredDelegation` over the stored key and reports Warning "the stored RIR delegation table cannot be used: `<reason>`; the shipped table answers", code `doctor-rir-delegation-stored-unusable` (new), fix `update resolve rir`; the three `stored.go` Warn lines carry it | no stored table reports nothing, because the seed answers by design |
| AC-24 | The resolve, storage and host code-metadata registrations | every code this spec names answers `ze show diagnostic <code>` with title, description and an example | none |

### AC-9 moves (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-25 | `policy-igp-import-hides-nexthop` | registered with phase `runtime`, so the local route never reads its own empty Loc-RIB as healthy and shows `doctor-runtime-not-checked` when no daemon answers; the row's fix is `show rib candidates prefix <prefix>` in `Fix`, the action text stays in `Help` | none |
| AC-26 | `gtsm-kernel-state` | splits into `gtsm-kernel-state` (`runtime`, installed state from `publishedPeers`, fix `ze signal restart` in `Fix`, removed from `Message`) and `gtsm-peer-route` (post-config, the configured peers' route resolvability, the same on both routes); the `inDaemon` branch is deleted; both declare `doctor-gtsm-kernel-state` | none |

### Self-update (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-27 | `ze config validate`, a commit or a boot with `restart immediate` and `restart time`, or a `restart time`, `maintenance-window start` or `end` that is not HH:MM | a `system-self-update` tree validator (`config.RegisterTreeValidator`, which runs before a tree is published, so validate, commit, boot and doctor's config read refuse together; a check only in a live verify passes `ze config validate`, config-yang door "Traps") runs `validateSelfUpdateConfig` and refuses, naming the leaf, the value and the HH:MM form, so the leaf descriptions' "Ze refuses" holds; `ze doctor` reports it through the config-tree check; `startBackend` keeps its "invalid config" line only under `doctor-internal-defect`, because a validated config cannot reach it. This AC also answers the appliance child's former self-update validator (`plan/release/validation/spec-doctor-coverage-5-appliance.md` cites it) | none |

### Lines whose class is covered, and not problem classes (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-28 | The two resolver server-failure Warns ("did not answer", "answered a server failure") | each carries `doctor-dns-resolver`, the covered class; the per-query fallback is not raised, because the `dns-resolvers` probe reports a failing server | none |
| AC-29 | The GTSM hop-limit route on a non-Linux build (`internal/component/gtsm/route_other.go`) | the line carries `doctor-gtsm-kernel-state` | none: production is Linux |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` on a box whose SMART enable failed | SMART manager, `storage-smart-runtime`, doctor answer | `test/plugin/show-doctor-storage-smart-runtime.ci` |
| 2 | commits a SMART interval `5` | verify, extractor | `test/parse/storage-smart-interval-one-digit.ci` |
| 3 | runs a box without BGP and sees an ECC event | hub system start, host monitor, report bus, `recent-events` | `TestHostMonitorStartsWithoutBGP` |
| 4 | runs `ze doctor` with no daemon on a box with GTSM peers | `gtsm-peer-route` only, runtime-not-checked row | `TestGTSMChecksSplitByPhase` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStorageRaisesDoctorCodes` | `internal/component/storage/manager_test.go` | AC-1 (existing assertions on the old names converted, not weakened) | |
| `TestHostDiffEventsUseConstantCodes` | `internal/component/host/metrics_test.go` | AC-2 | |
| `TestUpdateNoticesUseDoctorCodes` | `internal/component/config/system/selfupdate_test.go` | AC-3 | |
| `TestSmartDoctorReportsFailingHealth`, `TestSmartDoctorReportsTemperature` | `internal/component/storage/doctor_linux_test.go` | AC-4, AC-5 | |
| `TestSmartRuntimeCheck` | `internal/component/storage/doctor_runtime_test.go` | AC-6, AC-7 | |
| `TestSmartDoctorReportsFailedSelfTest` | `internal/component/storage/doctor_linux_test.go` | AC-8 | |
| `TestExtractSmartConfigReadsEveryYANGForm` | `internal/component/storage/config_test.go` | AC-9 | |
| `TestHostDoctorECCAndThrottle`, `TestHostDoctorReportsSensorAlarm`, `TestHostDoctorReportsUnreadableInventory` | `internal/component/host/doctor_test.go` | AC-10 to AC-13 | |
| `TestHostTuningDoctor` | `internal/component/host/doctor_tuning_test.go` | AC-14, AC-15 | |
| `TestHostMonitorStartsWithoutBGP` | `internal/component/host/monitor_test.go` | AC-16 | |
| `TestDNSResolversDNSSECProbe` | `internal/component/config/system/doctor_test.go` | AC-17 | |
| `TestResolverRaisesTruncated` | `internal/component/resolve/dns/resolver_test.go` | AC-18 | |
| `TestStoreStatusReportsStaleUnpersistedCorrupt`, `TestIRRDoctorReportsUnpersistedEntry` | `internal/component/resolve/irr/store/store_test.go`, `internal/component/firewall/plugins/irr/doctor_test.go` | AC-19 to AC-21 | |
| `TestFilterIRRDoctorReportsStale` | `internal/component/bgp/plugins/filter_irr/doctor_test.go` | AC-19, BGP owner | |
| `TestStoreRaisesPeeringDBFailure` | `internal/component/resolve/irr/store/store_test.go` | AC-22 | |
| `TestRIRStoredDelegationDoctor` | `internal/component/resolve/irr/doctor_stored_test.go` | AC-23 | |
| `TestSystemCodesHaveMetadata` | `internal/core/diagnostic/codes_test.go` | AC-24 | |
| `TestIGPImportCheckIsRuntime` | `internal/component/policy/doctor_test.go` | AC-25 | |
| `TestGTSMChecksSplitByPhase` | `internal/component/gtsm/doctor_test.go` | AC-26 | |
| `TestSelfUpdateRefusedAtVerify` | `internal/component/config/system/selfupdate_validate_test.go` | AC-27 | |
| doctor-coverage report over the area | `./le arch doctor-coverage report` | AC-28, AC-29, every line covered | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `self-test short interval` (bare seconds) | 1 to the largest duration | `1`, and the largest value the duration holds | `0` (YANG pattern) | one past the duration range (verify refusal) |
| `temperature critical` vs `informational` | 1..100 each | critical one above informational | critical equal to informational (doctor warning) | N/A |
| `update-check restart time` hour | 0-23 | 23 | N/A (no sign accepted) | 24 |
| `update-check restart time` minute | 0-59 | 59 | N/A | 60 |
| `update-check maintenance-window start`/`end` | same as `restart time` | 23:59 | N/A | 24:00 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-storage-smart-runtime` | `test/plugin/` | SMART enable failure and self-test not started appear in `show doctor` | |
| `storage-smart-interval-one-digit` | `test/parse/` | a one-digit interval is accepted and read as seconds; an out-of-range one is refused naming the leaf | |
| `self-update-restart-exclusive` | `test/parse/` | `ze config validate` refuses immediate with time | |
| `show-doctor-policy-igp-hidden` | `test/plugin/` | the hidden next-hop row appears in `show doctor`, not in local `ze doctor` | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible protocol change; the DNSSEC probe uses the existing resolver client.

## Files to Modify
- `internal/component/storage/manager.go`, `config.go`, `doctor_linux.go`, `register_linux.go` - codes, self-test error record, the shared extractor, health and temperature rows, runtime check registration (AC-1, AC-4 to AC-9)
- `internal/component/storage/yang/ze-storage-conf.yang` - descriptions name the new codes (AC-1)
- `cmd/ze/hub/main_system.go` - calls the storage extractor and deletes its own parser; host monitor start beside `startSmartManager`; tuning lines coded (AC-9, AC-15, AC-16)
- `internal/component/host/metrics.go`, `diff.go`, `register.go` - constant codes, error record, carrier up at Info, check registration (AC-2, AC-10 to AC-13)
- `internal/component/bgp/config/loader_create.go` - the host metrics start is deleted (AC-16)
- `internal/component/config/system/selfupdate.go`, `update.go`, `selfupdate_validate.go`, `doctor.go`, `yang/ze-system-conf.yang` - renames, verify, DNSSEC probe (AC-3, AC-17, AC-27)
- `internal/component/resolve/dns/resolver.go` - codes and the truncated event (AC-17, AC-18, AC-28)
- `internal/component/resolve/irr/store/store.go` - the learned-nothing family carried in the returned entry and its two per-family Warn lines removed (AC-19); status snapshot, skipped keys from `Open`, codes, event (AC-20 to AC-22)
- `internal/component/firewall/plugins/irr/irr.go` - the per-family kept-cached Warn under `doctor-firewall-irr-stale-data` in the refresh callers (AC-19); the runtime check declared in the runner's `sdk.Registration` and its `OnDoctorCheck` handler (AC-20)
- `internal/component/resolve/irr/stored.go` - codes (AC-23)
- `internal/component/firewall/plugins/irr/doctor.go`, `register.go` - declare and append the unpersisted and corrupt store rows (AC-20, AC-21)
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - declares the BGP owner's runtime check for its store in the runner's `sdk.Registration` (with `OnDoctorCheck`), declaring `doctor-bgp-irr-stale-data`, and logs the kept-cached Warn under it (AC-19)
- `internal/component/policy/doctor.go` - phase `runtime`, `Fix` (AC-25)
- `internal/component/gtsm/doctor.go`, `register.go`, `route_other.go` - the split, the not-problem line code (AC-26, AC-29)
- `internal/core/diagnostic/codes.go` - only where an owner cannot register its own metadata (AC-24)
- `docs/architecture/storage/smart-health.md`, `docs/architecture/appliance/self-update.md`, `docs/architecture/config/system-update.md`, `docs/guide/self-update.md`, `docs/architecture/host/inventory.md`, `docs/architecture/doctor-and-health-checks.md` - codes, the host monitor start, the GTSM split
- `test/ui/doctor-gokrazy-update-check.ci` - the renamed code (AC-3)

## Files to Create
- `internal/component/storage/doctor_runtime.go` (+ test) - `storage-smart-runtime`
- `internal/component/host/doctor.go`, `doctor_tuning.go`, `monitor.go` (+ tests) - `host-hardware`, `host-tuning`, `host-monitor`, the monitor start
- `internal/component/resolve/irr/doctor_stored.go` (+ test) - `rir-delegation-stored`
- `internal/component/bgp/plugins/filter_irr/doctor.go` (+ test) - the BGP owner's store check
- `test/plugin/show-doctor-storage-smart-runtime.ci`, `test/plugin/show-doctor-policy-igp-hidden.ci`, `test/parse/storage-smart-interval-one-digit.ci`, `test/parse/self-update-restart-exclusive.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf or RPC |
| YANG validation constraints | Yes | `ze-system-conf.yang` `restart time` gains the HH:MM pattern the validator enforces (AC-27) |
| YANG custom validators | Yes | the SMART and self-update verify (AC-9, AC-27) |
| CLI commands/flags | No | fix lines name existing commands |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC; the `.ci` files above cover the doctor and verify paths |
| Pipe completeness | N-A | no new command |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | Yes | every AC; codes registered by their owners |
| Prometheus counters/metrics | No | the `ze_host_*` gauges keep their names |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor rows only |
| 2 | Config syntax changed? | No | values the YANG already allows; `restart time` pattern tightens to what the backend reads |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/self-update.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | child 12 owns P0b |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/host/inventory.md` (monitor start), `docs/architecture/doctor-and-health-checks.md` (GTSM split) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the changed files' `// Design:` headers: `docs/architecture/firewall/firewall-irr.md` and `docs/architecture/bgp/filter-irr.md` (the store rows each owner's check adds, P4), `docs/architecture/resolve.md` (the IRR store status and the stored RIR check, P4), `docs/guide/route-policy.md` (the IGP-import row is answered by `show doctor` only, P5), `docs/architecture/hub-architecture.md` (the host monitor started by the hub's system start, P2); unaffected: `docs/architecture/config/syntax.md` (no syntax change: `restart time` takes the form the backend already reads) and `docs/features/ai-first.md` (`internal/component/host/register.go` keeps its platform-name registration unchanged); the rest derived at implementation with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-15-system.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/storage/smart-health.md`, the update pages: code names |

## Implementation Steps

### Implementation Phases

Sized for one agent of about 60 tool calls each.

| Phase | Work | ACs | Needs | Order |
|-------|------|-----|-------|-------|
| P1 codes and renames | the nine renames, constant host codes, code metadata from owners, the test and `.ci` that read the old names, docs | AC-1 to AC-3, AC-24 | child 12 P0a | first |
| P2 host | monitor start out of BGP (wiring first), error record, `host-hardware`, `host-tuning`, `host-monitor` | AC-10 to AC-16 | P1 | ∥ P3 |
| P3 storage | shared extractor and verify, self-test record, local and runtime checks, `.ci` files | AC-4 to AC-9 | P1 | ∥ P2 |
| P4 resolve | DNSSEC probe, truncated event, IRR store status and owners' checks, stored RIR check | AC-17 to AC-23 | P1; child 12 P0b for AC-22 forked | after P1 |
| P5 moves and lines | policy runtime, GTSM split, self-update verify, covered and not-problem lines, coverage report | AC-25 to AC-29 | P1 | last |

Each phase follows TDD and ends with the doctor-coverage report over the packages it touched.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | no runtime check answers an empty healthy set when its owner never started |
| Naming | every code starts `doctor-`; clears name the raise's code |
| Data flow | IRR store rows reach doctor only through the owners' plugin checks |
| Rule: no-layering | the GTSM process branch and the hub's SMART parser are deleted, not kept beside their replacements |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no old bus code string left | grep of the nine strings over `internal/`, `cmd/`, `test/`, `docs/` |
| every area line covered | `./le arch doctor-coverage report` lists no uncovered site in the area's packages |
| host monitor out of BGP | grep `host.RegisterMetrics` in `internal/component/bgp` answers nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the SMART and self-update values refused at verify name the leaf and the value, never a Go error chain alone |
| Error leakage | IRR and update URLs in messages keep their userinfo masked (`plan/journal/secret-echoed-to-the-client.md`) |

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

- The YANG for `storage smart` already bounds the values; the defect is the extractor refusing forms the YANG accepts, so the fix is one extractor that reads every accepted form and refuses at verify what it cannot hold.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-10 | `startBackend` drops an invalid self-update config the commit accepted |
| `plan/journal/validate-accepts-what-start-refuses.md` | 2026-11-10 | self-update restart and maintenance-window times checked only at backend start (AC-27; the A4 audit row, formerly listed by the appliance child) |
| `validate-accepts-what-start-refuses.md` | 2026-11-10 | `extractSmartConfig` replaces a SMART value the YANG accepts by a default |
| `silent-fall-through.md` | 2026-11-10 | `Manager.checkSelfTest` drops the self-test start error |
| `silent-fall-through.md` | 2026-11-10 | `HostMetrics.CollectOnce` drops a detect error |
| `unwired-feature.md` | 2026-11-10 | host monitor started only from the BGP config loader |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| IRR store rows reported by each owner's plugin check | one runtime check in the store package | the stores live in plugins that can run forked; a check in the daemon would read no store (`ai/doors/system.md`, the process boundary) |
| GTSM split into two checks | keep one check with the child 11 route field | each question has one phase that can answer it; the process branch goes (`ai/rules/no-layering.md`) |
| Host monitor started by the hub's system start | keep it in the BGP loader and add a second start for non-BGP boxes | one start path for a host feature; BGP compiles out |
| Carrier down and throttle as ring events | active warnings | they never clear today, so they stay active forever |
| The `cmd/ze/hub` ad hoc `fix`, `remedy` and `check` attributes are left to child 11 AC-10 | convert them here too | one owner per site; child 11 already lists all three in its census and AC-10 |
| Self-update renames here, the other update classes stay with the appliance audit rows | respec every update class here | the brief assigns the two bus codes and the invalid-config defect to this child; the remaining `config/system` update rows are the appliance area's (`plan/audits/doctor-problem-inventory/appliance.md`) |

## Known Limitations
- A truncated DNS answer is used as received; whether the resolver owes a TCP retry is a question for the resolver's RFC summary, outside this doctor spec. This spec reports it (AC-18).

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
- [ ] AC-1..AC-29 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests (`./le job run label <label> command go test -race <package>`, `-tags ze_bgp` for `filter_irr` and `bgp/config`), `./le test functional plugin` scoped to the named `test/plugin/*.ci`, `./le test functional parse` scoped to the named `test/parse/*.ci`, and `./le test functional ui` for `doctor-gokrazy-update-check.ci`. No agent runs any `./le verify` command
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
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-15-system.md` only, in the same `./le commit create` script
