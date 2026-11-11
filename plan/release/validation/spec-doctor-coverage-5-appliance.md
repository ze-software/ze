# Spec: doctor-coverage-5-appliance

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate`: the `runtime` doctor phase, `Diagnostic.Fix`, the `doctor-recent-events` check and the doctor-coverage gate, landed in commits ce58dddf53 and fc96556110. `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes, `doctor-internal-defect` among them; the event-code registration; the report-bus fix convention) and phase P0b (the plugin report route): this spec uses them and builds neither. `plan/release/validation/spec-doctor-coverage-14-system-plugins.md` AC-31 and AC-32 fill the two selection fields this spec adds (AC-19). `plan/release/validation/spec-doctor-coverage-15-system.md` AC-3 renames `self-update` and `update-available` to their doctor codes (this spec's AC-1 and AC-2 build on the renamed codes), and its AC-27 refuses the invalid self-update config (immediate with time, a bad HH:MM) before it reaches `startBackend`: this spec builds neither. `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-25 and AC-26 raise under the dns-source codes this spec declares (AC-14, AC-17) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Inventory: `plan/audits/doctor-problem-inventory/appliance.md` (19 class rows: 16 gap, 3 covered).

## Task

`ze doctor` reports every problem class of the appliance area, with its cause
and the command that fixes it: the system update backends (self-update, the
update checker, gokrazy A/B), the system resolver (resolv.conf, the dns-source
registry and its consumers), conntrack settings and the serial console, all in
`internal/component/config/system` and the parts of `cmd/ze/hub` that apply
them. Every Warn and Error line, and every report-bus raise in the area, names
its doctor code (O-2), so the doctor-coverage gate counts it covered.

Where doctor does not run, and why that is not a gap:

| Code | Where it runs | Doctor's reach |
|------|---------------|----------------|
| `internal/install`, `cmd/ze-installer` | the initrd, as PID 1, before any ze daemon exists | none: no `ze doctor` runs there, and the gate's scan roots exclude it. The audit found no record the installer leaves on `/perm` for the booted system to read (Known Limitations) |
| `internal/appliance` (`ze appliance build`, `iso`, `push`, `rekey`) | the build host, one command at a time | the existing pre-config `appliance-*` checks read the build host; each command line fails its own command or warns its own operator, and leaves no state on the device (Known Limitations) |
| `cmd/ze-serial-shell` | the serial login gate | no Warn or Error site in the slice |

An update, a rekey or an install failure that leaves the booted box degraded is
in scope. On the device, only the update path leaves such state: a staged
binary that is not running, a binary installed without verification. `rekey`
changes the build host's passphrase store and nothing on the device. The device
side of a push is covered in the hub-engine child (pushed config, health revert).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/appliance.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md`, `ai/doors/config-yang.md`, `ai/doors/forwarding.md`, `ai/doors/system-plugins.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md`
  → Constraint: a readiness check is registered by the package that owns the dependency (system door, "Shared machinery"), so every check here registers from `internal/component/config/system`, never from `cmd/ze/hub` or `internal/component/doctor`
  → Constraint: an operator relies on "what `ze config validate` accepts is what boots" (hub-engine door); AC-22 repairs one place where it does not hold (the self-update times are the second, repaired by `plan/release/validation/spec-doctor-coverage-15-system.md` AC-27), and `test/plugin/config-validate-agrees-with-boot.ci` is the existing proof shape (config-yang door, "Gates")
  → Constraint: runtime state persists through `statestore` under a registered zefs key (core door, "Boundaries"); the update history already does, under `meta/config/update-history`, and AC-8 and AC-9 add result values to it rather than a new key
  → Constraint: an image in the field is the contract for a `/perm/ze/` path or a zefs key the image writes (appliance door); no key is added or renamed here
  → Constraint: kernel tunables go through `sysctl`, config over transient over plugin defaults (forwarding door); the conntrack sysctl finding (AC-21) skips a key the operator's own sysctl config sets
  → Constraint: the report bus is process-local (forwarding door); every raise here is in the daemon process, and a forked consumer's dns-source line reaches the bus only through child 12's P0b route
  → Constraint: a dns-source consumer plugin may run forked, so `dnssource.ConsumerSelection` is a self-contained JSON value (system door, plugin-infra door)
- [ ] `docs/architecture/doctor-and-health-checks.md` - "The runtime phase", "Every finding names the command that fixes it", "The phase decides what a check may read", "Every problem site names a doctor code", "Recent events: the report bus in show doctor"
  → Decision: a runtime check reads its owner's live state and is given only `Platform` (parent Data Flow 1b)
  → Constraint: a report-bus code counts as covered only when its owner declares it (parent Data Flow 2a, amended); the owner declares through child 12's event-code registration (P0a)
- [ ] `docs/architecture/appliance/self-update.md` - "Decisions"
  → Constraint: history is a circular buffer of 20 events, read back at start; manual apply proceeds without a SHA-256 and warns
- [ ] `docs/features/dns-resolver.md` - "Where the router's servers come from", "The `dns-source` list and the learned record"
  → Constraint: the dns-source selection checks are runtime checks over `ProcessResolvers()`; this spec extends that check, never a second one
- [ ] `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` - Task rows 1 to 3, AC-1 to AC-3
  → Decision: `doctor-internal-defect` and the event-code registration are child 12's; this spec registers only appliance codes through them
- [ ] `plan/release/validation/spec-doctor-coverage-14-system-plugins.md` AC-31, AC-32; `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-25, AC-26
  → Decision: the dns-source check owned here declares `doctor-dns-source-host-only-dropped`, `doctor-dns-source-over-limit` and `doctor-dns-source-consumer-stale`, which the consumer and RA publisher lines of children 14 and 10 carry
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` class 3
  → Constraint: resolver selection states (no usable server, one of several, learned not followed) are child 2's; this spec covers the file write and the registry's refusals, not the selection. Child 2's three `doctor-system-resolver-*` codes join the declaration of the same `dns-source-selection` check (child 2 AC-4), so AC-18 lists only this spec's codes and both specs edit one `Codes` list

**Key insights:**
- The update path keeps its state in `selfUpdater` (`downloadStatus`, `stagedVersion`, `serverPaused`, `status.LastError`) and in the persisted history; `show system update` shows it and doctor shows nothing
- One report-bus warning per (source, code, subject) is active at a time; today every self-update state raises under the one code `self-update`, so the message changes and the code says nothing
- A failed exec into a staged binary leaves the process running the old version, with an active warning still saying "restarting into"

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/doctor.go` - four post-config checks (`dns-resolvers`, `update-check-url`, `update-backend-config`, `resolv-conf-path`); none sets `Fix`
- [ ] `internal/component/config/system/doctor_dns_source.go` - the runtime check `dns-source-selection` over `Resolvers.Snapshot` and `Resolvers.Consumers`, three codes, fixes in `Fix`
- [ ] `internal/component/config/system/doctor_conntrack_linux.go` - `conntrack-procfs` (post-config): netfilter tree present, `nf_conntrack_max` writable; no `Fix`
- [ ] `internal/component/config/system/register.go` - registers the checks and the tree validators `system-host-name`, `system-dns-source`
- [ ] `internal/component/config/system/selfupdate.go` - `selfUpdater.check`, `download`, `attemptStage`, `handleRestart`, `waitForRestartTime`, `manualDownload`, `manualApply`, `recordEvent`, `loadHistory`, `saveHistory`; raises under `selfUpdateCode` (`self-update`); Warn lines with no code
- [ ] `internal/component/config/system/update.go` - `UpdateChecker.check`: Warn on fetch failure, raise and clear under `updateCode` (`update-available`)
- [ ] `internal/component/config/system/selfupdate_validate.go` - `validateSelfUpdateConfig` (called only from the two backend factories), `warnConfigConflicts` (Warn, restart time outside the window)
- [ ] `internal/component/config/system/backend.go` - `UpdateBackend`, `registerBackend` (two `slog.Error` lines before a panic), `NewBackend`, `ActiveBackend`, `ActiveExtendedUpdateStatus`
- [ ] `internal/component/config/system/backend_gokrazy.go` - `gokrazyBackend.Status` sets `LastError` "gokrazy management unavailable" when `probe` fails; nothing logs or reports it
- [ ] `internal/component/gokrazy/gokrazy.go` - `handlerWithAuth`: the proxy's error handler logs Warn "gokrazy proxy error" and answers 502
- [ ] `internal/component/config/system/resolv.go` - `Resolvers` (fields `path`, `written`, `last`, `snapshot`, `consumers`), `Configure`, `expire`, `PublishOn`, `SetPublication`, `emitDNSSnapshot`, `refresh`, `parseServers`; Warn lines with no code
- [ ] `internal/component/config/system/resolv_host.go` - `readHost`: Warn on an unreadable host resolv.conf and on each rejected server text
- [ ] `internal/component/config/system/system.go` - `ExtractSystemConfig` logs Error when `RouterDNSSource` refuses; `acceptedDNSSECValidation` logs two Warn fallbacks
- [ ] `internal/component/config/system/console.go` - `ValidConsoleDeviceName`; `extractConsole` and `ExtractConsoleFromMap` drop an invalid name with no log line
- [ ] `internal/component/config/system/console_linux.go` - `ApplyConsole` returns `ConsoleResult` (applied, skipped with reason, errors); `applyTermios` logs Warn when the fd close fails
- [ ] `internal/component/config/system/conntrack.go` - `ConntrackConfig.ValidateModules` checks names against the model
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `update-check maintenance-window start`/`end` and `update-check restart time` are `type string`, and their descriptions say "Ze refuses" a value that is not HH:MM and both `immediate` and `time`; console `device name` is `type string` length 1..32 with no pattern, its description asking for the bare name
- [ ] `internal/plugins/update-cmd/cmd/show.go` - `handleShowSystemUpdate` (status from `ActiveExtendedUpdateStatus`), `handleShowSystemUpdateHistory` (prints each event's `result` as text)
- [ ] `internal/core/diagnostic/codes.go` - the `CodeDoctor*` constants, among them the three `doctor-dns-source-*` codes
- [ ] `cmd/ze/hub/main_system.go` - `applyConsole`, `applyConsoleFromMap` (Warn per skip and error), `applyResolvConf` (Warn), `applyConntrackConfig` (three Warn lines), `startBackend` (Warn on platform detection, Warn "invalid config" and no backend)
- [ ] `cmd/ze/hub/main.go` - startup `Configure` of `ProcessResolvers`, Warn "resolv.conf write failed"
- [ ] `internal/appliance/diskverify.go` - `verifyE2fsck` and `tryLoopbackVerify` warn and skip on the build host
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck`: one warning row per (kind, source, code, subject), under the event's own code when its metadata is registered

**Behavior to preserve:**
- `show system update` and `show system update history` output; the history JSON key names
- The update history zefs key `meta/config/update-history` and its 20-event bound
- The existing doctor codes and check names of this package, and the `doctor-dns-source-*` rows proven by `test/plugin/doctor-dns-source.ci`
- Manual download and apply still proceed without a SHA-256

**Behavior to change:**
- Every update state raises under a doctor code that names the state, with its fix; one state is active at a time
- A failed restart, an unverified install, a resolv.conf not written, a refused publication and a skipped console device leave a record a runtime check reads
- The console device name is refused at validate, commit and boot (the self-update times are `plan/release/validation/spec-doctor-coverage-15-system.md` AC-27)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Post-config checks of this package | `internal/component/config/system/doctor.go` `systemDoctorChecks`; `doctor_conntrack_linux.go` `conntrackDoctorCheck` | Five local checks over config and host | extends: each gains `Fix` (AC-24); new local checks join the same table (AC-11, AC-12, AC-21) |
| Runtime dns-source check | `doctor_dns_source.go` `dnsSourceDoctorCheck`, `dnsSourceFindings` | Three selection findings over the live registry | extends: declares the resolver-file, dropped-server, host-unreadable, consumer-stale, host-only and over-limit codes (AC-14 to AC-19); no second dns check |
| Doctor phase set and registry | `pkg/plugin/rpc/types.go` `DoctorCheckPhase`; `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck` | Phases including `runtime` | extends: new checks register here; no phase added |
| `Diagnostic.Fix` | `internal/core/diagnostic/types.go` `Diagnostic` | The command alone, drawn on its own line | extends: every finding here fills it |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `ClearWarning`, `Warnings`, `Errors` | Active warnings keyed by source, code, subject | extends: update states raise under their own doctor codes, with child 12's fix field (P0a) |
| Event-code registration (child 12 P0a) | `internal/core/diagnostic`, symbol named in child 12 AC-2 | An owner declares a bus code with metadata, so `doctor-recent-events` renders it and the gate counts it | extends: config/system registers its six update-state codes through it (AC-1); nothing rebuilt |
| `doctor-internal-defect` (child 12 P0a) | `internal/core/diagnostic/codes.go` | The code for a Ze defect reached at run time | does not apply as a new mechanism: lines here carry it (AC-20, AC-21, AC-25) |
| Recent-events check | `internal/component/doctor/checks_recent_events.go` `recentEventsCheck` | Renders bus events and active warnings as doctor rows | extends: shows the update-state warnings under their codes; no update check of this spec reads the same state a second time |
| Update backend status and history | `backend.go` `UpdateBackend.Status`, `History`; `selfupdate.go` `recordEvent`, `saveHistory` | Status for `show system update`; 20 persisted events | extends: two result values (`failed-restart`, `applied-unverified`); the runtime check reads `History` (AC-10) |
| Config tree validators | `internal/component/config/tree_validators.go` `RegisterTreeValidator`, `validateOriginalTree`; `register.go` | Run before a tree is published, so validate, commit, boot and doctor's config read refuse together | extends: the `system-console-device` validator (AC-22); the self-update times are refused by `plan/release/validation/spec-doctor-coverage-15-system.md` AC-27 |
| Platform detection check | `doctor-platform-detect` (pre-config) | Reports an undetected platform | does not apply: inventory row covered |
| Host name check | `system-host-name` (post-config, appliance build), `doctor_hostname.go` | Kernel name against `system host` | does not apply: inventory row covered |
| Build-host checks | `internal/appliance/doctor_checks.go` (`doctor-appliance-*`, pre-config) | Tools and artifacts the image build needs | does not apply: inventory row covered; the loopback half is a Known Limitation |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage` | Refuses an added uncovered problem site | does not apply: unchanged; this spec moves the area's sites to covered |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show doctor` on any daemon surface (runtime checks), `ze doctor` locally (local checks; runtime through the daemon when it answers, O-1)
- `ze config validate`, commit and boot for the two new validators

### Transformation Path
1. The update loop (`selfUpdater.check` and its successors, `UpdateChecker.check`) sets the update state through one helper that raises that state's doctor code on the bus, clears the other update-state codes for subject `firmware`, and carries the fix (child 12's fix field)
2. A failed exec and an install without SHA-256 append a history event (`failed-restart`, `applied-unverified`), persisted as today
3. `doctor-recent-events` renders the active update-state warning under its code; the runtime check `system-update-history` reads `ActiveBackend().History()` for an unverified running binary
4. `Resolvers` records its last file-write error, its dropped server texts, the host read error, refused publications and the last failed snapshot emit; the extended `dns-source-selection` runtime check reads them with the snapshot
5. The hub's console apply stores its last `ConsoleResult` through a config/system setter; the runtime check `serial-console` reads it
6. Local checks `self-update-schedule`, `gokrazy-management` and `conntrack-config` read the config tree and the host

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Producer to report bus | `report.RaiseWarning` and `ClearWarning` in-process (config/system runs in the daemon) | No |
| dns-source consumers to the registry | `dnssource.PublicationEvent` on the EventBus; a consumer plugin may run forked, so the payload is JSON | No |
| Daemon to local `ze doctor` | the O-1 ask over SSH; runtime checks never run in the local process | No |

### Changed-Wire Census

**Search evidence:** scope `internal`, `cmd`, `pkg`, `test`; terms `"self-update"`, `"update-available"`, `selfUpdateCode`, `updateCode`, `KeyConfigUpdateHistory`, `UpdateEvent`, `ConsumerSelection`. Positive control: `selfupdate.go` `selfUpdateCode` found as the writer. Findings: the bus codes have no reader outside the bus and `update_test.go` (the `"update-available"` hits in `internal/plugins/update-cmd/cmd/show.go` and `firmware.go` are JSON keys of the status answer, unchanged); no health row keys on either code.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| update-state bus code | writer | `internal/component/config/system/selfupdate.go` raises under `selfUpdateCode`; `update.go` under `updateCode` | grep of both constants | change: six doctor codes (AC-1); no release shipped the old names |
| update-state bus code | reader | `internal/component/doctor/checks_recent_events.go` `groupRecentEvents`; `show warnings`; `internal/component/config/system/update_test.go` | grep | change: the test follows the rename; the check reads any code |
| update history blob | writer | `selfupdate.go` `recordEvent`, `saveHistory` under `meta/config/update-history` | read | change: two new `Result` values; field names unchanged |
| update history blob | reader | `selfupdate.go` `loadHistory`; `internal/plugins/update-cmd/cmd/show.go` `handleShowSystemUpdateHistory`; `internal/test/fixture/storage_consumer_restart.go` | read, grep | preserve: readers print `Result` as text; the fixture compares a `blocked-minimum-version` event, unchanged |
| dns-source selection payload | writer | `internal/plugins/dhcpserver`, `internal/plugins/dnsresolver` (child 14 fills the new fields), `internal/component/iface` RA publisher (`ra_dns_source.go`), `internal/component/l2tp/plugins/pool` (`dns_source.go`; `plan/release/validation/spec-doctor-coverage-3-access-security.md` AC-84 fills the dropped list), each building `dnssource.ConsumerSelection` | child 14 AC-31, AC-32; child 3 AC-84 | change: two kebab-case JSON fields added (AC-19); a publisher that omits them publishes zero, which the check reads as nothing dropped and no limit |
| dns-source selection payload | reader | `resolv.go` `SetPublication`; `doctor_dns_source.go` `dnsSourceFindings`; `show system dns` (`internal/plugins/system-dns-cmd`) | read, grep | change: the check reads both fields; `show system dns` prints them |

### Integration Points
- `report.RaiseWarning` / `ClearWarning` with child 12's fix field - update states
- `diagnostic.RegisterDoctorCheck` - new checks, two extended
- child 12 event-code registration - six update-state codes
- `config.RegisterTreeValidator` - two validators

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | update states reach doctor through the bus and `doctor-recent-events`; the runtime checks read the owner's live state |
| No unintended coupling (components stay isolated) | Yes | every check registers from `internal/component/config/system`; the hub stores the console result through a config/system setter, never a doctor import |
| No duplicated functionality (extends existing, does not recreate) | Yes | one dns check extended; the update states are bus warnings, not a second status reader |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding |
| Registration over hardcoding, outbound | Yes | checks, codes and validators register from `init()` in config/system |
| Registration over hardcoding, inbound | Yes | searched `internal/component/doctor` (runner, recent-events), `internal/le/arch/doctorcoverage` and `internal/core/diagnostic/codes.go` for appliance names: the runner and the gate read the registries; `codes.go` is the declared metadata home |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every published config tree passed `validateOriginalTree`, so the `RouterDNSSource` refusal in `ExtractSystemConfig` is reached only by a defect | `tree_validators.go` `validateOriginalTree`; `register.go` registers `system-dns-source` | the line is a reachable class, owed a doctor finding instead of `doctor-internal-defect` | read every `ExtractSystemConfig` caller's tree source at P5 | unvalidated |
| A-2 | Child 12's P0a lands the event-code registration and the bus fix field before this spec's P1 | child 12 "Implementation Phases" | P1 has no registration to call | `Depends`; P1 starts by reading the P0a symbols | unvalidated |
| A-3 | The conntrack module name is constrained by the YANG model, so `ValidateModules` failing at the hub is a defect | `conntrack.go` `ValidateModules` ("against the model") | the hub line is a class owed a finding | a `ze config validate` unit over an unknown module at P4 | unvalidated |
| A-4 | `ze.gokrazy.socket` is the only socket path setting, default `/run/gokrazy-http.sock` | `internal/component/config/environment.go` registration | the check probes another path than the backend | read at P2 | unvalidated |
| A-5 | Opening a serial device from doctor could change line state (DTR) on attached hardware, so doctor never opens one | `console_linux.go` `applyTermios` opens the device read-write | none: the design reads the daemon's recorded apply result instead | design choice | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two update-state warnings active at once (a stale state code left behind) | `show warnings` lists two `firmware` rows | the one helper clears every other update-state code on each raise; a unit test asserts one active row per transition |
| R-2 | Child 10 AC-25 names `doctor-ra-rdnss-not-published` for the RDNSS publish class the merge put under `doctor-dns-source-consumer-stale` | two codes for one class | resolved: child 10 AC-25 now reports under `doctor-dns-source-consumer-stale`, which this spec declares; `doctor-ra-rdnss-not-published` is registered nowhere |
| R-3 | A new history result breaks a reader that matches results | `show system update history` test red | readers print the text; the census found no matcher besides the fixture's own value |
| R-4 | The consumer-stale record grows with refusals | memory growth | one record per consumer (last refusal and time), bounded by the consumer count |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false or missing doctor row; two validators could refuse a config that boots today (both forms the YANG text already calls refused or wrong) |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | children 2 (resolver selection), 10 (RDNSS publisher), 14 (dhcpserver, dnsresolver), 12 (P0a, P0b) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a self-update manifest server paused | → | update-state raise under `doctor-self-update-waiting`, rendered by `doctor-recent-events` | `test/plugin/doctor-self-update-waiting.ci` |
| `show doctor` after a manifest checksum mismatch | → | `doctor-update-download-failed` raise | `test/plugin/doctor-update-download-failed.ci` |
| `show doctor` with the update server down | → | `doctor-update-fetch-failed` raise | `test/plugin/doctor-update-fetch-failed.ci` |
| `ze config validate` with console device `/dev/ttyS0` | → | `system-console-device` tree validator | `test/parse/console-device-bare-name.ci` |
| `ze doctor` with a restart time outside the window | → | `self-update-schedule` check | `test/ui/doctor-self-update-schedule.ci` |
| `show doctor` with a resolv.conf path that cannot be written | → | `dns-source-selection` resolver-file finding | `test/plugin/doctor-resolv-conf-stale.ci` |
| `show doctor` with a console device absent from `/dev` | → | `serial-console` runtime check | `test/plugin/doctor-serial-console.ci` (`option=needs-linux`) |
| `ze doctor` with a conntrack module the kernel has not loaded | → | `conntrack-config` check | `test/ui/doctor-conntrack-config.ci` (`option=needs-linux`) |

## Acceptance Criteria

Every row names severity, code, the fix line, the state read and the route.
"Bus warning" means an active report-bus warning, rendered in `show doctor` by
`doctor-recent-events` under its own code with its own fix. New codes are
registered with metadata in `internal/core/diagnostic/codes.go` (check codes) or
through child 12's event-code registration (bus codes), so `ze show diagnostic
<code>` answers each.

### Shared mechanisms (P1)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze show diagnostic` for `doctor-update-available`, `doctor-self-update-waiting`, `doctor-update-blocked`, `doctor-update-fetch-failed`, `doctor-update-download-failed`, `doctor-update-restart-failed` | each answers title, description and example; config/system declares them from `init()` through child 12's event-code registration; the rename of the two existing codes `self-update` and `update-available` to `doctor-self-update-waiting` and `doctor-update-available` is `plan/release/validation/spec-doctor-coverage-15-system.md` AC-3, and this spec declares the four new codes beside them; the gate counts every update raise covered | none: no release shipped the old codes |
| AC-2 | the update loop moves from one state to another (available, waiting, blocked, failed, staged) | exactly one update-state warning for source `system`, subject `firmware` is active, under the code of the current state; a raise clears the other five codes; each raise carries its fix (child 12 AC-3); a check that finds no update clears all six | the update checker (no self-update) uses only `doctor-update-available` and `doctor-update-fetch-failed`, because it has no other state |

### Update backends (P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-3 | the manifest or version fetch fails (self-update and update checker) | bus warning `doctor-update-fetch-failed`: the redacted URL, the error and the time of the last good check; fix `update system firmware check`; cleared by the next good fetch; the two fetch Warn lines carry the code | the local `update-check-url` check keeps reporting reach from the doctor's own host, a different state |
| AC-4 | a newer image is available and auto-apply is off, or the update checker sees a newer version | bus warning `doctor-update-available`: remote and running versions; fix `update system firmware apply` (self-update) or `show system update` (checker only) | none |
| AC-5 | the update waits: paused by the server, inside the spread delay, outside the maintenance window, staged with a manual restart, restart scheduled or about to run | bus warning `doctor-self-update-waiting`, the message naming the reason and, where one exists, the remaining delay, the window or the restart time; fix `update system firmware apply` (paused, spread, window) or `update system firmware restart` (staged, scheduled) | none |
| AC-6 | auto-apply finds no SHA-256 in the manifest, or the manifest's minimum version is newer than the running one | bus warning `doctor-update-blocked`, naming which, and the remote, running and minimum versions; fix `update system firmware check`, the message saying what the server must publish first | none |
| AC-7 | resolving the download URL, the free-space check, the download, the hash, the checksum comparison or the stage fails | bus warning `doctor-update-download-failed` naming the step and cause: for free space, the needed and free bytes and the target directory; for a mismatch, expected and actual sums; fix `update system firmware download`; the six Warn lines carry the code; history events unchanged | none |
| AC-8 | the exec into a staged binary fails (immediate, scheduled or manual apply) | history event `failed-restart` (persisted); bus warning `doctor-update-restart-failed` naming staged and running versions and the exec error, replacing the "restarting into" warning; fix `update system firmware restart`; the two exec Warn lines carry the code | a successful exec replaces the process, and the next start reads the history success, so no finding remains |
| AC-9 | a manual download or apply installs a binary with no SHA-256 | history event `applied-unverified` for the new version; the two Warn lines carry `doctor-update-unverified-binary` | none |
| AC-10 | `show doctor` while the running version's newest history event is `applied-unverified` | runtime check `system-update-history` answers warning `doctor-update-unverified-binary` naming the version and the install time; fix `update system firmware apply`, the message saying it installs a verified build once the server publishes a SHA-256 | a later verified install for the running version clears it; the local route does not run the check (runtime phase) |
| AC-11 | an `update-check restart time` outside `update-check maintenance-window` | local post-config check `self-update-schedule`: warning `doctor-self-update-restart-outside-window` naming the time and the window; fix `set system update-check restart time <HH:MM>`; the Warn line in `warnConfigConflicts` carries the code | a restart time inside the window, or no window, gives no row: the YANG text permits the combination, and this row explains its effect |
| AC-12 | gokrazy platform, management socket (`ze.gokrazy.socket`) answers neither `/update/features` nor `/` | local check `gokrazy-management`: warning `doctor-gokrazy-management-unreachable` naming the socket path and the error, and that A/B status and the web `/gokrazy/` page are unavailable; fix names the `ze.gokrazy.socket` setting; the proxy's Warn line carries the code | a non-gokrazy platform gives no row |

### System resolver (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-14 | resolv.conf cannot be written at start, reload or lease expiry | `Resolvers` keeps the last write error, its time and the servers it failed to write; the `dns-source-selection` runtime check answers warning `doctor-resolv-conf-stale` naming the path, the servers selected, the servers the file holds and the error; fix `set system dns resolv-conf-path <path>`; the three Warn lines (`expire`, hub start, hub reload) carry the code | a write that succeeds clears the record; child 10 AC-26 (a learned resolver missing from `/etc/resolv.conf`) uses this code for its lease-producer lines, under that row's own condition "when the system-DNS child declares a code for the same state" |
| AC-15 | a server text from a lease, the host resolv.conf or `system name-server` is not an IP address | `Resolvers` keeps the dropped texts per origin; the check answers warning `doctor-dns-source-server-dropped` naming origin, interface or path, and text; fix `show system dns`; the two Warn lines carry the code | the next lease from that origin replaces its record |
| AC-16 | the host resolv.conf exists and cannot be read | the check answers warning `doctor-dns-source-host-unreadable` naming the path and the error; fix `show system dns`; the Warn line in `readHost` carries the code | an absent file gives no row, as today |
| AC-17 | a consumer publication is refused by `SetPublication`, or a snapshot emit fails | `Resolvers` keeps the last refusal per consumer and the last failed emit sequence; the check answers warning `doctor-dns-source-consumer-stale` naming the consumer and the cause; fix `show system dns`; the Warn lines in `PublishOn` and `emitDNSSnapshot` carry the code | a later accepted publication or delivered snapshot clears it; the consumer-side lines are children 14's and 10's, under the same code |
| AC-18 | the check's declaration | `dns-source-selection` declares `doctor-resolv-conf-stale`, `doctor-dns-source-server-dropped`, `doctor-dns-source-host-unreadable`, `doctor-dns-source-consumer-stale`, `doctor-dns-source-host-only-dropped` and `doctor-dns-source-over-limit` with its three existing codes | none |
| AC-19 | a published selection carries host-only servers dropped, or more servers than the consumer uses | `dnssource.ConsumerSelection` gains the dropped list and the consumer's limit (kebab-case JSON); the check answers `doctor-dns-source-host-only-dropped` and `doctor-dns-source-over-limit`, naming the consumer, the servers and the limit; fix `show system dns`; child 14 (AC-31, AC-32) fills the fields for dhcpserver and dnsresolver, and child 3 (AC-84) fills the dropped list for the l2tp pool publisher | a publisher that leaves the fields zero gives no row: zero means nothing dropped and no limit |
| AC-20 | `ExtractSystemConfig` meets a refused `system dns-source` | the Error line carries `doctor-internal-defect` | none: the `system-dns-source` tree validator refuses the list before a tree is published (A-1) |

### Conntrack and serial console (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-21 | Linux, `system conntrack` configures a module the kernel has not loaded, or a sysctl value the netfilter tree does not hold | local post-config check `conntrack-config`: warning `doctor-conntrack-module-not-loaded` naming the module, fix `delete system conntrack module <name>`, the message naming the kernel that lacks it; warning `doctor-conntrack-sysctl-not-applied` naming key, configured and kernel values, fix the reload command; the load-failed and emit-failed Warn lines carry these codes, and the module-validation Warn line carries `doctor-internal-defect` (A-3) | a key the operator's sysctl config sets is skipped, because that value wins by design |
| AC-22 | a console `device name` that is not a bare name (the owned journal row) | tree validator `system-console-device` refuses it at validate, commit and boot, naming the device and the bare form | none: today the name commits and is dropped with no line |
| AC-23 | the daemon applied the console devices at start or reload, and a device was skipped (getty active) or failed (open, termios) | the hub stores the last `ConsoleResult` through a config/system setter; runtime check `serial-console` answers warning `doctor-serial-console-getty-conflict` (fix `delete system console device <name>`) or `doctor-serial-console-not-applied` naming device and error (fix `delete system console device <name>`, the message naming the device path to check); the four Warn lines carry these codes | doctor never opens a serial device (A-5); the invalid-name skip carries `doctor-internal-defect` after AC-22 |

### Existing checks and lines that are not problem classes (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-24 | each finding of `dns-resolvers`, `update-check-url`, `update-backend-config`, `resolv-conf-path`, `conntrack-procfs` | carries a `Fix` naming a `ze` command or a `set`/`delete` path, asserted by each check's unit test | none |
| AC-25 | the programmer-error lines: backend registration nil and duplicate factory (`backend.go` `registerBackend`), the two dnssec-validation fallbacks (`system.go` `acceptedDNSSECValidation`), the invalid restart time at schedule (`selfupdate.go` `waitForRestartTime`), and the serial fd close (`console_linux.go` `applyTermios`); `startBackend`'s "invalid config" line is `plan/release/validation/spec-doctor-coverage-15-system.md` AC-27 | each line carries `doctor-internal-defect` | none |

### AC-9 (parent): checks reading daemon state outside runtime

None in this area. Checked: `dns-resolvers`, `update-check-url`,
`update-backend-config`, `resolv-conf-path`, `conntrack-procfs`,
`system-host-name` (post-config) read the config or the host; `dns-source-selection`
is already `runtime`; the `appliance-*` checks read the build host. The new
checks keep the rule: `system-update-history`, `serial-console` and the
extended dns check are `runtime`; `self-update-schedule`, `gokrazy-management`
and `conntrack-config` read only config and host.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator's box stops pulling updates because the server is down | fetch fails, bus warning, recent-events row, `show doctor` | `test/plugin/doctor-update-fetch-failed.ci` |
| 2 | an operator sees why a staged update has not run | staged state, bus warning, `show doctor` names `update system firmware restart` | `test/plugin/doctor-self-update-waiting.ci` |
| 3 | an operator writes a console device as a path | validate refuses it | `test/parse/console-device-bare-name.ci` |
| 4 | an operator's resolver file sits on a read-only path | write fails, record, runtime check row | `test/plugin/doctor-resolv-conf-stale.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateStateOneActiveWarning` | `internal/component/config/system/selfupdate_test.go` | AC-2: each transition leaves one active code | |
| `TestSelfUpdateFetchFailedRaise` | `internal/component/config/system/selfupdate_test.go` | AC-3 | |
| `TestUpdateCheckerCodes` | `internal/component/config/system/update_test.go` | AC-1, AC-3, AC-4 for the checker | |
| `TestSelfUpdateWaitingReasons` | `internal/component/config/system/selfupdate_test.go` | AC-5, each reason and fix | |
| `TestSelfUpdateBlocked` | `internal/component/config/system/selfupdate_test.go` | AC-6 | |
| `TestSelfUpdateDownloadFailedSteps` | `internal/component/config/system/selfupdate_test.go` | AC-7, six steps | |
| `TestSelfUpdateRestartFailed` | `internal/component/config/system/selfupdate_test.go` | AC-8 with a failing `restartFunc` | |
| `TestManualApplyUnverifiedRecorded` | `internal/component/config/system/selfupdate_test.go` | AC-9 | |
| `TestDoctorUpdateHistoryUnverified` | `internal/component/config/system/doctor_update_test.go` | AC-10 and its survivor | |
| `TestDoctorSelfUpdateSchedule` | `internal/component/config/system/doctor_update_test.go` | AC-11, a window crossing midnight included | |
| `TestDoctorGokrazyManagement` | `internal/component/config/system/doctor_update_test.go` | AC-12 against an unanswering socket | |
| `TestDNSSourceResolvConfStale` | `internal/component/config/system/doctor_dns_source_test.go` | AC-14 | |
| `TestDNSSourceServerDropped`, `TestDNSSourceHostUnreadable` | `internal/component/config/system/doctor_dns_source_test.go` | AC-15, AC-16 | |
| `TestDNSSourceConsumerStale` | `internal/component/config/system/doctor_dns_source_test.go` | AC-17 | |
| `TestDNSSourceHostOnlyAndLimit` | `internal/component/config/system/doctor_dns_source_test.go` | AC-18, AC-19 | |
| `TestDoctorConntrackConfig` | `internal/component/config/system/doctor_conntrack_linux_test.go` | AC-21 with stubbed module and procfs reads | |
| `TestValidateConsoleDeviceTree` | `internal/component/config/system/console_test.go` | AC-22 | |
| `TestDoctorSerialConsole` | `internal/component/config/system/doctor_console_test.go` | AC-23 from a stored result | |
| `TestSystemDoctorChecksCarryFix` | `internal/component/config/system/doctor_test.go` | AC-24 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| console `device name` length | 1-32 | 32 | 0 | 33 (YANG) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-self-update-waiting` | `test/plugin/` | local manifest server with `paused`, `show doctor` row and fix | |
| `doctor-update-download-failed` | `test/plugin/` | manifest sum differs from the served file | |
| `doctor-update-fetch-failed` | `test/plugin/` | update URL pointing at a closed local port | |
| `console-device-bare-name` | `test/parse/` | validate refuses `/dev/ttyS0` | |
| `doctor-self-update-schedule` | `test/ui/` | restart time outside the window | |
| `doctor-resolv-conf-stale` | `test/plugin/` | resolv-conf-path under a read-only directory | |
| `doctor-serial-console` | `test/plugin/` | device absent from `/dev` (`option=needs-linux`) | |
| `doctor-conntrack-config` | `test/ui/` | module not loaded (`option=needs-linux`) | |

The update `.ci` files reuse the local manifest server of
`test/plugin/storage-history-restart.ci` and `ze.test.update.running.version`
(`docs/architecture/appliance/self-update.md`).

### Interop Tests (Scope: protocol)
N-A: tooling scope; no wire protocol with a peer changes.

## Files to Modify
- `internal/component/config/system/selfupdate.go` - one update-state helper, codes, fix, `failed-restart` and `applied-unverified` events, code attributes
- `internal/component/config/system/update.go` - codes, fetch-failed raise
- `internal/component/config/system/selfupdate_validate.go` - `warnConfigConflicts` code (the refusal of `validateSelfUpdateConfig` at validate and commit is child 15 AC-27)
- `internal/component/config/system/backend.go` - code attributes
- `internal/component/config/system/resolv.go`, `resolv_host.go` - records of write error, dropped texts, host read error, refusals, failed emit; code attributes
- `internal/component/config/system/doctor_dns_source.go` - new findings and codes
- `internal/component/config/system/doctor.go`, `doctor_conntrack_linux.go` - `Fix`; new local checks in the table
- `internal/component/config/system/register.go` - two tree validators, new check registrations, event-code registrations
- `internal/component/config/system/system.go` - code attributes
- `internal/component/config/system/console.go`, `console_linux.go` - tree validator, stored apply result, code attribute
- `internal/component/config/system/yang/ze-system-conf.yang` - descriptions match the validators (no type change)
- `internal/component/gokrazy/gokrazy.go` - code attribute on the proxy Warn
- `internal/core/dnssource` - `ConsumerSelection` dropped list and limit
- `internal/core/diagnostic/codes.go` - metadata for the new check codes
- `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go` - store the console result; code attributes
- `internal/plugins/system-dns-cmd` - print the two new selection fields
- `internal/component/config/system/update_test.go` - the four new update codes (the rename is child 15 AC-3)
- `docs/architecture/appliance/self-update.md`, `docs/features/dns-resolver.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/appliance.md`, `docs/architecture/config/system-update.md`, `docs/architecture/config/syntax.md` - see the Documentation checklist (`docs/architecture/hub-architecture.md` judged unaffected there)

## Files to Create
- `internal/component/config/system/doctor_update.go` - `system-update-history` (runtime), `self-update-schedule`, `gokrazy-management`
- `internal/component/config/system/doctor_console.go` - `serial-console` (runtime)
- `internal/component/config/system/doctor_conntrack_config_linux.go` - `conntrack-config`
- `internal/component/config/system/doctor_update_test.go`, `doctor_console_test.go`
- the eight `.ci` files of the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf added; descriptions corrected only |
| YANG validation constraints | No | HH:MM and the bare name are checked by tree validators, as `system-host-name` is; a `pattern` on the leaves is an alternative the implementer may take if it gives the same message |
| YANG custom validators | Yes | `system-console-device` through `config.RegisterTreeValidator`; the self-update validator is child 15 AC-27 |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf added |
| Functional test for new RPC/API | N-A | no RPC; doctor rows proven by the `.ci` files above |
| Pipe completeness | N-A | no new output command |
| Env var registration | No | `ze.gokrazy.socket` already registered |
| Doctor check for runtime dependencies | Yes | the checks of this spec; codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | records are last-state values, not counters |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor rows only |
| 2 | Config syntax changed? | Yes | the YANG descriptions; `docs/guide/configuration.md` where it shows a restart time or console device |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` (update states in doctor) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | the dnssource payload gains two fields; named in `docs/features/dns-resolver.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/self-update.md` (one update-state warning, two history results); `docs/architecture/doctor-and-health-checks.md` "Where each owner registers its check" |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-5-appliance.md`; known so far: `docs/features/dns-resolver.md` (declared by `doctor_dns_source.go` and `resolv_host.go`), `docs/architecture/appliance/self-update.md` (declared by `selfupdate_validate.go` and `show.go`), `docs/features/ai-first.md` (declared by `doctor.go` and `register.go`: unaffected unless its check list names these checks); `docs/architecture/config/system-update.md` (declared by the update files: updated, the update-state codes and the two history results); `docs/architecture/config/syntax.md` (declared by `console.go`: updated, the console device takes the bare name and a path is refused); `docs/architecture/hub-architecture.md` (declared by the hub files: unaffected, the startup order and the apply calls keep their place) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/appliance.md` self-update examples checked against the validator |

## Implementation Steps

### Implementation Phases

Each phase is sized for one agent of about 60 tool calls.

| Phase | Work | ACs | Needs | Order |
|-------|------|-----|-------|-------|
| **P1 update-state helper and codes** | the event-code registrations, the one-active helper, `update_test.go` | AC-1, AC-2 | child 12 P0a; child 15 P1 (the AC-3 rename) | first |
| **P2a update raises** | fetch, available, waiting, blocked, download, restart, unverified record; three update `.ci` files | AC-3 to AC-9 | P1 | after P1 |
| **P2b update checks and validator** | `system-update-history`, `self-update-schedule`, `gokrazy-management`; the ui `.ci` file | AC-10 to AC-12 | P2a (history results) | after P2a |
| **P3 system resolver** | the `Resolvers` records, the extended check, the selection fields and `show system dns` | AC-14 to AC-20 | P1 (code registration only) | ∥ P2a |
| **P4 conntrack and console** | `conntrack-config`, console validator, stored apply result, `serial-console` | AC-21 to AC-23 | - | ∥ P2a, P3 |
| **P5 fixes and defect lines** | `Fix` on the five existing checks, `doctor-internal-defect` lines, the A-1 read | AC-24, AC-25 | child 12 P0a | last |

Each phase is wiring first: register the check or validator, write its failing
wiring test, then the behavior. Each phase ends with `./le arch doctor-coverage
report` showing every site the phase owns covered.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has its implementation and test named |
| Correctness | one active update-state warning after every transition; each record is cleared by the success that ends it |
| Naming | codes match the audit's canonical names where one exists; new codes start `doctor-`; JSON fields kebab-case |
| Data flow | every runtime check reads its owner's live state, never `ctx.Tree`; doctor never opens a serial device |
| Rule: no-layering | the dns check extended, not doubled; the update state lives on the bus, not also in a second check reading `downloadStatus` |
| Rule: user-facing-errors | each fix line is a command alone; each message says what is wrong and why |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every site of the slice covered | `./le arch doctor-coverage report` lists no uncovered site in `internal/component/config/system`, `internal/component/gokrazy` or the named hub files |
| new codes explained | `ze show diagnostic <code>` for each code AC-1, AC-9 to AC-12, AC-14 to AC-19, AC-21 and AC-23 name |
| validators | the two `test/parse/` files |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | HH:MM and device names validated before use; a device name with `/` or `..` refused at commit |
| Error leakage | the update URL is redacted (`redact.URL`) in every message and record |
| Resource exhaustion | records are one per origin or consumer, bounded |
| Fail-open | an unverified install is reported until a verified one replaces it |

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

- One active update-state warning keyed by its state code gives `show warnings`, `show doctor` and the gate the same answer. A runtime check reading `downloadStatus` beside the bus raises would report each state twice.
- The unverified-binary state survives the exec that creates it, so it lives in the persisted history, the only update record that crosses a restart.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/validate-accepts-what-start-refuses.md` | 2026-11-10 | console device name not bare commits, dropped silently at apply (AC-22) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Update states as one active bus warning per state code | a runtime check over `ExtendedUpdateStatus` | the bus already carries the state and `doctor-recent-events` renders it; a second reader doubles each row (`ai/rules/no-layering.md`) |
| Unverified install as a history result | a new zefs key | the history key already persists across the exec; no field contract is added (appliance door) |
| Console state from the daemon's recorded apply result, at runtime | doctor opens the device to read termios | opening a serial port can change line state on attached hardware; doctor applies nothing |
| Extend `dns-source-selection` | a resolver-file check of its own | one check over the one registry |
| Tree validators for the times and the device name | YANG `pattern` | same place as `system-host-name`; refuses with a message naming the form |
| The selection fields added here, filled by child 14 | child 14 adds them | the check that reads them defines them; one owner for the core type |

## Known Limitations
- `internal/install` and `cmd/ze-installer` run as initrd PID 1, outside the gate's scan roots, where no `ze doctor` runs. The audit found no state the installer leaves on `/perm` for the booted system's doctor to read, so there is no AC. Whether the installer should leave such a record is an owner decision the inventory records ("Shared mechanisms", item 5); it is not decided here.
- The 10 `internal/appliance` sites run inside one `ze appliance` invocation on the build host and leave no state; they sit outside the gate's scan roots. The loopback-mount half of the `/perm` image verification (no private mount directory, a refused loop mount, a non-root build) is reported or skipped by `ze appliance build` itself: it is a property of one build run, not of a box doctor can read. If the scan roots grow, the inventory names the codes (`doctor-appliance-e2fsprogs`, `doctor-internal-defect`).
- `ze appliance rekey` changes the build host's passphrase store only, and `push` failures are build-host command errors; the device side of a push is the hub-engine child's.
- Rollback and manual restart answer the calling command with an error and leave no state, so they are no class.

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
- [ ] AC-1..AC-25 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label system command go test -race ./internal/component/config/system/` (and `./internal/core/dnssource/`, `./internal/plugins/system-dns-cmd/`), then the named `.ci` files through `./le test functional plugin`, `./le test functional parse` and `./le test functional ui`, the `needs-linux` ones in the QEMU VM. No agent runs any `./le verify` command
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
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-5-appliance.md` only, in the same `./le commit create` script
