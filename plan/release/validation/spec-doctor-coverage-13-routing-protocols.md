# Spec: doctor-coverage-13-routing-protocols

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | Thomas's rule that `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it (`ai/META-DECISIONS.md`, Goal and scope), and owner decision O-3 of `spec-doctor-coverage-1-audit-and-gate.md`: every area child is release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the local route asking the daemon, the `recent-events` check, the doctor-coverage gate; landed in commits ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (cross-cutting codes `doctor-plugin-stopped`, `doctor-internal-defect`, the transient-code registration, a raise carrying its fix) and phase P0b (the plugin report route phase S2 uses); `plan/release/validation/spec-doctor-coverage-10-forwarding.md` for the MPLS writer-side codes: `doctor-fib-kernel-mpls-install` and `doctor-fib-kernel-mpls-remove` (its AC-83, AC-84), the failed install or removal seen by the writer, which AC-28 and AC-43 here do not duplicate: they compare the protocol's own table with the kernel; `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` adds a route field to `DoctorCheckContext` (its Required Reading), which AC-9c does not need because each owner answers from its own recorded bind result |
| Phase | ready |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Door: `ai/doors/routing-protocols.md`. Inventory rows: `plan/audits/doctor-problem-inventory/routing-protocols.md` (65 class rows: 54 gap or partial, 11 covered, none owned by child 2).

## Task

Doctor reports every problem class of the routing-protocols area (BFD, LDP,
RSVP-TE, OSPF, IS-IS) with its cause in plain words and the command that fixes
it, on every surface through `show doctor`. Every Warn and Error log line in the
area's packages names its doctor code (owner decision O-2 of the parent), so
`./le arch doctor-coverage report` counts it covered.

| # | Work | Why |
|---|------|-----|
| 1 | One acceptance criterion per gap class of the inventory, grouped by protocol | the inventory is the population; a class with no check is a problem doctor cannot report |
| 2 | The AC-9 rows: checks that read daemon state, or probe sockets the daemon holds, while registered before `runtime` | a local or daemon answer must never read a healthy daemon as broken, or an unseen one as healthy |
| 3 | Three defects found by the audit, each journaled and owned here | they change what doctor would report, so they are fixed with the checks |
| 4 | The "not problem classes" sites, each carrying the code its group names | O-2: no Warn or Error line without a code |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - process boundary, registration, one store
  → Constraint: a check that reads a plugin's live state is answered inside that plugin's process (`OnDoctorCheck`), never by a Go call into its package from the daemon, because every plugin may run forked
  → Constraint: a readiness check is registered by the package that owns the dependency ("Proximity"), so a protocol's port probe lives in the protocol, not in doctor's own listener extractor list
- [ ] `ai/doors/routing-protocols.md` - the engines, their boundaries and traps
  → Constraint: `api.GetService` answers nil in a forked plugin and in any process the bfd plugin does not run in, so a check reading it is only meaningful inside the plugin that would use the BFD client
  → Constraint: labels reach the kernel only through `(mpls-fib, entry)` events, and an installer with no sink drops every route with no error (both become doctor classes here)
- [ ] `ai/doors/plugin-infra.md` - the SDK and the engine-op registry
  → Constraint: one engine-op entry per RPC; a new plugin-to-daemon call is one `engineOps` entry that serves the DirectBridge and the RPC path alike
- [ ] `ai/doors/core.md` - `internal/core/diagnostic`, `internal/core/report`
  → Constraint: every code a check declares resolves through the diagnostic code registry (`internal/core/diagnostic/codes.go`), proven by `TestDoctorRegisteredCheckCodesHaveMetadata`
- [ ] `ai/doors/le-tooling.md` - the gate and the `.ci` runner
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux`; a scoped package test runs through `./le job run` with its feature tags
- [ ] `ai/doors/system-plugins.md` - redistribution consumers share `internal/plugins`
  → Constraint: every plugin can run in another process; reach the engine through the SDK, the bus, or a command
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Task, O-1 to O-3, AC-6, AC-9, Data Flow amendments
  → Decision: a `runtime` registry check receives `Platform` only (Data Flow 1b); a runtime check reads its owner's live state
  → Decision: a transient code counts as covered for the gate only when its owner's check declares it; the `recent-events` check then reports the report-bus event (Data Flow 2a, amended)
  → Decision: the local route asks the daemon first and answers locally only when no daemon answers (O-1)
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem first, why it matters, then the fixing command on a line of its own (`Diagnostic.Fix`)

**Key insights:**
- Three mechanisms register a doctor check in this area: `diagnostic.RegisterDoctorCheck` (OSPF, IS-IS, the OSPF and IS-IS transports), `registry.DoctorCheckDef` in a plugin's `Registration` (LDP `ldp-port`, RSVP-TE `rsvp-te-rawsock`), and the SDK's `DoctorChecks` declaration answered by `OnDoctorCheck` in the running plugin (OSPF `ospf-state`). Only the third reads live plugin state on both transports
- On the daemon route the post-config checks also run, in the daemon process, so a post-config bind probe of a port the daemon holds fails on every healthy box

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/doctor.go` - `ospfConfigDiagnostics` adds `doctor-ospf-bfd-plugin-absent` when `bfdEnabledInterfaceCount` is non-zero and `api.GetService()` is nil; `checkOSPFConfigSanity` runs it from the post-config `ospf-config-sanity` check, so a local `ze doctor` always reads nil and warns whenever BFD is configured
- [ ] `internal/plugins/ospf/register.go` - `OnDoctorCheck` answers `ospf-state` from `activeCfg` and `p.StateKeys`; the SDK `Registration.DoctorChecks` declares it `rpc.DoctorPhasePostConfig` with the one code `doctor-ospf-graceful-restart-nvs`
- [ ] `internal/plugins/ospf/instance.go` - `openInterfaces` logs a per-interface open failure and continues (the OSPF fix of the shape IS-IS still has)
- [ ] `internal/plugins/isis/server.go` - `openCircuits` returns the first `openCircuit` error, so the circuits after it are never opened and `refreshConnectedPrefixes` and `originate` never run
- [ ] `internal/plugins/isis/register.go` - `OnStarted` returns that error wrapped as "isis: opening circuits", which stops the whole IS-IS plugin; it logs "isis: no net configured, engine idle" at Warn with no code
- [ ] `internal/plugins/rsvpte/register.go` - `setupTunnel` sets the LSP to path-sent and, when `eng` is nil, increments `pathMsgSent` (`ze_rsvpte_path_sent_total`) although no PATH left the box; the `rsvp-te-rawsock` `registry.DoctorCheckDef` is post-config
- [ ] `internal/plugins/rsvpte/reservation.go` - the egress branch of the ResvErr handler logs "rsvp-te: receiver reservation rejected" at Warn with the RSVP ERROR_SPEC code under the attribute key `code`
- [ ] `internal/plugins/rsvpte/engine.go` - the PathErr handler logs "rsvp-te: PathErr received" at Info with the ERROR_SPEC code under the key `code`
- [ ] `internal/plugins/ldp/doctor.go` - `checkLDPPort` and `probePortBindable` bind UDP and TCP 646 with no socket options; its comment says LDP binds TCP 646
- [ ] `internal/plugins/ldp/register.go` - `listenDiscovery` opens a multicast UDP socket on 224.0.0.2:646 per interface; sessions are dialed through `ldpSessionDialer`; no TCP listener exists (`docs/architecture/ldp/mpls-ldp.md`: "Nothing listens on TCP port 646"); the `ldp-port` check is a post-config `registry.DoctorCheckDef`
- [ ] `internal/component/bfd/bfd.go` - `runtimeState.loopFor` returns an existing loop without comparing the requested device beyond a Warn, never compares `bindV6`, and returns the loop start error to its caller without storing it
- [ ] `internal/component/doctor/checks_listener.go` - `extractBFDListeners` adds one UDP 3784 probe with `CodeDoctorBFDPort`; `bindProbe` binds without the BFD transport's socket options; `checkListeners` reports every bind error as "cannot bind"
- [ ] `internal/core/diagnostic/codes.go` - metadata rows for `doctor-ldp-port-unavailable` (its description says UDP/TCP 646) and `doctor-rsvpte-rawsock-unavailable`; `CodeDoctorBFDPort`
- [ ] `internal/core/diagnostic/doctor_registry.go` - `DoctorCheckContext` carries no route field, so a check cannot tell the daemon route from the local one

**Behavior to preserve:**
- every existing check name and code in the area: `ospf-config-sanity`, `ospf-segment-routing`, `ospf-debug-enabled`, `ospf-graceful-restart-nvs`, `ospf-state`, `ospf-raw-socket`, `ospfv3-raw-socket`, `ospfv3-ipsec`, `isis-raw-socket`, `isis-config-sanity`, `ldp-port`, `rsvp-te-rawsock`, and doctor's `doctor-bfd-port` code
- the `DoctorResult` keys and exit codes of the parent
- the OSPF and IS-IS "show" outputs; this spec adds no key to them

**Behavior to change:**
- every gap class of the inventory gets a finding with code, severity, message and fix
- the BFD half of `ospf-config-sanity` moves to the runtime `ospf-state` check; `ospf-state` is declared `runtime`
- the BFD and LDP port probes stop reporting a port the running daemon holds as unavailable; LDP stops probing TCP 646
- the three defects below are fixed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| OSPF config checks | `internal/plugins/ospf/doctor.go` `checkOSPFConfigSanity`; the `RegisterDoctorCheck` calls in `internal/plugins/ospf/register.go` | router-id, area binding, unnumbered donor, BFD absence | extends: gains the local halves of OSPF classes (key-chain expiry, redistribute family); loses the BFD half (AC-9a) |
| OSPF plugin-declared check | `internal/plugins/ospf/register.go` `OnDoctorCheck`, `Registration.DoctorChecks` `ospf-state` | GR restart-fact readability in the running plugin | extends: declared `runtime`; answers every OSPF runtime class from the engine's live state |
| IS-IS config checks | `internal/plugins/isis/doctor.go`, registered from `internal/plugins/isis/register.go` as `isis-config-sanity` | NET missing, system-id mismatch | extends: gains the IS-IS half of the redistribute-family class; a new plugin-declared runtime check `isis-state` answers the runtime classes |
| Raw-socket checks | `internal/plugins/ospf/transport/register.go`, `internal/plugins/ospf/v3/transport/register.go`, `internal/plugins/isis/transport/register.go`; RSVP-TE `rsvp-te-rawsock` | host probe of the raw socket | extends: `rsvp-te-rawsock` gains its runtime half (engine idle with router-id set) in `rsvp-te-state` |
| LDP port check | `internal/plugins/ldp/doctor.go` `checkLDPPort` | binds UDP and TCP 646 plainly | extends: probes UDP 646 only, as the engine binds, and yields to the runtime answer when the engine holds the port (AC-9c) |
| Doctor listeners | `internal/component/doctor/checks_listener.go` `extractBFDListeners` | one UDP 3784 probe for BFD | replaces (`ai/rules/no-layering.md`): the BFD extractor is deleted first, then the BFD-owned `bfd-ports` check probes every mode, the echo port and VRF devices; one owner for one fact |
| Doctor registry and runtime phase | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`, `DoctorPhaseRuntime`; `internal/component/doctor/registry.go` `RunRuntimeChecks` | runs checks per phase; runtime only on the daemon route | extends: the area's runtime classes register as runtime |
| Plugin doctor checks | `internal/component/plugin/registry/doctor.go` `DoctorCheckDef`, `validateDoctorCheckDef`; `internal/component/doctor/cmd/show.go` `collectDoctorChecksFrom`; `internal/component/plugin/server/startup.go` `refuseDoctorCheckDeclaredTwice` | config-time plugin checks (the registry refuses a `DoctorCheckDef` in the `runtime` phase, commit 149497ce12); SDK-declared checks answered by the running plugin, whatever their phase; Stage 1 refuses a declared check whose name the plugin's registry row already holds | extends: every runtime class is SDK-declared and answered by `OnDoctorCheck` in the protocol's own process, internal or forked; `ldp-port` and `rsvp-te-rawsock` stay post-config `DoctorCheckDef`s, so the new runtime `ldp-state` and `rsvp-te-state` take names of their own |
| Fix field | `internal/core/diagnostic/types.go` `Diagnostic.Fix`; `pkg/plugin/rpc/types.go` `DoctorCheckDiagnostic` | carries the command on its own line | extends: every finding here sets it |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` | metadata for `ze show diagnostic` | extends: one metadata row per new canonical code |
| Report bus and recent events | `internal/core/report/report.go` `RaiseError`, `RaiseWarning`; `internal/component/doctor/checks_recent_events.go` `recent-events` | coded events; recent rows in `show doctor` | extends: each transient class raises its event under its code, registered through `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a's transient-code registration, which `recent-events` reads; a forked plugin reaches the bus through child 12 phase P0b's plugin report route |
| Health registry | `internal/core/health/registry.go` | degraded rows with a reason | does not apply: the area registers no health row (inventory: 0) |
| Doctor-coverage gate | `./le arch doctor-coverage check` and `report` (`internal/le/arch/doctorcoverage`) | refuses an added Warn or Error site with no doctor code | extends: every site of the area gains its code; the report for the area's package trees reads zero uncovered |
| Protocol metrics | `internal/component/bfd/metrics.go`; `internal/plugins/ospf/transport/metrics.go`; `internal/plugins/isis/transport/metrics.go`; the metrics struct in `internal/plugins/rsvpte/register.go` | counters the checks can read (`ze_bfd_auth_failures_total`, `ze_ospf_packets_dropped_total`, `ze_isis_frames_dropped_total`) | extends: the checks read the owner's counters in process; missing counters are added in the owner (S3) |
| Operator error text | `internal/core/opererr` `New`, `LogAttrs` | problem, why, fix and a caller's `code` attribute | extends: a site that becomes an `opererr` log line keeps the `code` key for the doctor code only |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor [--json] [config <file>]` locally; `show doctor` on SSH, MCP, REST, gRPC and web

### Transformation Path
1. Config classes (no router-id, no lsr-id, unmapped admission, unbound BFD loop, IPv6 BFD not bound, redistribute family, key-chain expiry, missing interface devices) run post-config on both routes from the config tree
2. Runtime classes run on the daemon route: `collectDoctorChecksFrom` calls the SDK-declared check of each protocol (`bfd-state`, `ldp-state`, `rsvp-te-state`, `ospf-state`, `isis-state`), answered by `OnDoctorCheck` in the protocol's process from its live state and counters
3. Transient classes: the site logs at Warn with its code and raises the event on the report bus under that code; `recent-events` reports it; the owning protocol's check declares the code so the gate counts the site covered
4. Each finding carries `fix`; `show doctor` returns it on every surface and `ze doctor` draws it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol plugin to doctor | SDK `DoctorChecks` declaration and `OnDoctorCheck` (DirectBridge in process, RPC when forked) | Yes: `ospf-state` uses it today |
| Protocol plugin to report bus | child 12 phase P0b's plugin report route (one engine-op entry) | No: built by child 12, used here |
| Component to doctor (config classes) | `diagnostic.RegisterDoctorCheck` or `registry.DoctorCheckDef` in the daemon and the CLI process | Yes: `ospf-config-sanity`, `ldp-port` |

### Changed-Wire Census

**Search evidence:** the doctor JSON shape (`schema-version`, `ready`, `diagnostics` with `code`, `severity`, `message`, `fix`) is unchanged; this spec adds diagnostics and codes, not keys. The SDK `DoctorCheckDecl` phase value `runtime` already exists (`pkg/plugin/rpc/types.go` `DoctorPhaseRuntime`). The plugin-to-daemon report raise is a new wire owned and censused by `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0b; this spec only calls it, so it carries no row for it. AC-86 adds LDP's TCP 646 listener, a peer-facing change proved by its interop scenario. Search scope `pkg/plugin`, `internal/component/plugin`; terms `RaiseError`, `RaiseWarning`, `engineOps`. Positive control: `internal/component/plugin/process/manager.go` is the only non-test caller of a report raise under the plugin tree, and it runs in the daemon. The log attribute `code` on the two RSVP-TE lines is a log field, not a wire: its readers are the gate (`internal/le/arch/doctorcoverage`) and operators reading logs.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show doctor` JSON | writer | `internal/component/doctor` `Answer` | unchanged keys | preserve |
| `show doctor` JSON | reader | `.ci` expectations under `test/plugin/` and `test/ui/` | new `.ci` files only | preserve: additions only |

### Integration Points
- the registrations in `internal/plugins/ospf/register.go`, `internal/plugins/isis/register.go`, `internal/plugins/ldp/register.go`, `internal/plugins/rsvpte/register.go` and `internal/component/bfd/bfd.go`: `DoctorChecks` declarations and `OnDoctorCheck` handlers
- `internal/core/diagnostic/codes.go`: metadata rows

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | runtime state is read in the owning plugin's process; MPLS label checks read the kernel table through the existing `internal/component/mpls` dump |
| No unintended coupling (components stay isolated) | Yes | no protocol imports another; OSPF reads `api.GetService` as its BFD client already does |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; doctor's BFD extractor is deleted, not paralleled |
| Zero-copy preserved where applicable (refs, not copies) | Yes | checks read snapshots at doctor time only |
| Registration over hardcoding, outbound | Yes | every check registers from its owner |
| Registration over hardcoding, inbound | Yes | deleting `extractBFDListeners` removes a protocol name from doctor's `collectAllListeners`; no doctor file learns a protocol name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A plain UDP bind of 0.0.0.0:646 fails with EADDRINUSE while LDP's multicast socket holds 224.0.0.2:646 | audit note (not run); Go's `ListenMulticastUDP` sets SO_REUSEADDR, the probe does not | the LDP false positive does not occur, and AC-9c's LDP half reduces to removing the TCP probe | `TestLDPPortProbeWhileEngineHolds` under `needs-linux` | unvalidated |
| A-2 | Every config path a fix line names exists in the YANG schema | YANG modules of the five protocols | a fix line names a path the parser refuses | each check's unit test parses its fix path against the schema (`config.YANGSchema`) | unvalidated |
| A-3 | The daemon restart command named by restart-needed fixes is the one Ze's own messages already name | the plugin server's circular-dependency error names `systemctl restart ze` | the fix names the wrong command | implementer reads the restart path and asserts the command in the unit tests | unvalidated |
| A-4 | The forked-plugin raise route is child 12 phase P0b's (`plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`, AC-4) | child 12 Task row 4 | a transient class of a forked plugin has no route | phase S2 starts only after child 12 P0b lands, and builds nothing | unvalidated |
| A-5 | Each counter a check reads is held by the plugin's own process | metrics registries are per process | a forked plugin's check reads zero | one runtime check per protocol run with the plugin forked (`./le mock plugin-external`) | unvalidated |
| A-6 | IS-IS `HandleLinkUp` errors for a missing device, as the audit read | `internal/plugins/isis/server.go` `openCircuit` | the IS-IS defect needs another trigger | `TestISISOneCircuitOpenFailureKeepsOthers` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A runtime check reports a session or adjacency "down" during a normal start | rows on every boot | thresholds come from each protocol's own timers (BFD detection time, LDP KeepAlive hold, RSVP refresh period, OSPF dead interval, IS-IS hold time), never a new constant |
| R-2 | Two area children both build a plugin report-raise path | two engine ops for one fact | resolved: the one route is child 12 phase P0b, and this spec builds none (`ai/rules/no-layering.md`); A-4 |
| R-3 | A transient class fills the report ring | `show errors` dominated by one subject | the owner raises once per (code, subject) per protocol timer interval; the ring is bounded already |
| R-4 | Moving the BFD half of `ospf-config-sanity` to runtime removes a row from the local route | an operator sees fewer rows with no daemon | the parent's `doctor-runtime-not-checked` row names `show doctor` |
| R-5 | The MPLS label check races label installation | rows during a reconvergence | report an entry missing only after one refresh of the owner's state with no change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | false or missing doctor rows; the IS-IS start fix changes start behavior on a box with a missing circuit device (the plugin keeps running) |
| How is it reverted? | one commit per protocol phase |
| Who else touches this path? | `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0b report raise, P0a plugin-stopped codes), `plan/release/validation/spec-doctor-coverage-10-forwarding.md` (fib-kernel MPLS codes), `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` (same packages) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json config` with `ldp` and no `lsr-id`, `rsvp-te` and no `router-id` | → | `ldp-config`, `rsvp-te-config` post-config checks | `test/ui/doctor-routing-protocols-config.ci` |
| `show doctor` with a BFD session whose peer never answers | → | `bfd-state` runtime check via `OnDoctorCheck` | `test/plugin/show-doctor-bfd-state.ci` (`option=needs-linux`) |
| `show doctor` with an LDP interface naming an absent device | → | `ldp-state` runtime check | `test/plugin/show-doctor-ldp-state.ci` (`option=needs-linux`) |
| `show doctor` with a configured tunnel and no RSVP neighbor | → | `rsvp-te-state` runtime check | `test/plugin/show-doctor-rsvpte-state.ci` (`option=needs-linux`) |
| `show doctor` with BFD enabled on an OSPF interface and no bfd plugin | → | `ospf-state` runtime check (AC-9a) | `test/plugin/show-doctor-ospf-state.ci` (`option=needs-linux`) |
| `show doctor` with an IS-IS circuit on an absent device | → | `isis-state` runtime check, IS-IS still running | `test/plugin/show-doctor-isis-state.ci` (`option=needs-linux`) |
| a transient class event (RSVP PathErr refusal) then `show doctor` | → | report-bus raise, `recent-events` row | `test/plugin/show-doctor-rsvpte-recent-event.ci` (`option=needs-linux`) |

## Acceptance Criteria

Every check below sets `Fix` to the command alone (parent AC-7), states the
cause in plain words, and has a unit test that goes red when the detection is
removed. "Runtime" means the protocol's SDK-declared check answered in its own
process; "local" means a post-config check on both routes; "transient" means a
Warn site carrying the code plus a report-bus raise under it, reported by
`recent-events`, with the code declared by the owner's runtime check.

### Shared and AC-9

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch doctor-coverage report` over `internal/component/bfd`, `internal/plugins/ldp`, `internal/plugins/rsvpte`, `internal/plugins/ospf`, `internal/plugins/isis` after the last phase | every Warn or Error site is covered: it carries the `code` of its class in the tables below or of its "not problem class" group, and that code is declared by a registered check | none: O-2 has no marker |
| AC-2 | Every new code of this spec | has a metadata row in `internal/core/diagnostic/codes.go`, and `ze show diagnostic <code>` explains it; `TestDoctorRegisteredCheckCodesHaveMetadata` passes | none |
| AC-3 | A forked protocol plugin raises a transient class event | the daemon's report bus holds the event under the plugin's name and the class code, and `recent-events` reports it; in process the same call raises the same event | none: a transient class reported only when the plugin runs internal is the process-boundary defect |
| AC-9a | BFD enabled on an OSPF interface, no BFD service in the OSPF plugin's process, `show doctor` | `ospf-state` (runtime) answers `doctor-ospf-bfd-plugin-absent` (warning; fix: the `bfd` config that loads the bfd plugin); local `ze doctor` with no daemon answers the parent's `doctor-runtime-not-checked` row in its place; `ospf-config-sanity` reads only the config tree | none: the local process never runs the BFD plugin, so its nil is not evidence |
| AC-9b | `ospf-state`'s SDK declaration | declares phase `runtime` and every OSPF runtime code of this spec | none |
| AC-9c | `ldp` configured with the LDP engine running and holding UDP 646; `bfd` configured with BFD loops running; `show doctor` | each owner answers from its own recorded bind result: a port its running engine holds reads healthy, and a recorded bind failure reads `doctor-ldp-port-unavailable` or `doctor-bfd-port`; `ldp-port` probes UDP 646 only, as the engine binds, and the code's metadata says UDP | a port held by another process with no engine running still reports, its message naming "held by another process" and its fix `ze status` to confirm no ze daemon holds it |

### BFD

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-10 | A reload moved a pinned single-hop session to another interface on a live loop, or committed `bind-v6 true` after a loop exists | runtime `doctor-bfd-loop-restart-needed` (warning) naming VRF, mode, bound device and requested device, or the missing IPv6 socket, from `loopDevices` and the loop's transport family against the running config; fix: the restart command (A-3) | none |
| AC-11 | Default-VRF single-hop sessions name different interfaces, or some name none | local (pinned sessions in config) and runtime (client sessions in the loop) `doctor-bfd-loop-unbound` (warning) naming the interfaces; fix: the `interface` leaf that makes them one | a VRF whose single-hop sessions all name one interface is bound, so it is not reported |
| AC-12 | A BFD port (UDP 3784 single-hop, 4784 multi-hop, 3785 echo) cannot be bound, or a VRF device a session names is missing | local `bfd-ports` check, owned by `internal/component/bfd` and replacing doctor's `extractBFDListeners`, reports `doctor-bfd-port` per mode, port and VRF; runtime reports the stored `loopFor` start error under the same code; fix names the privilege or the VRF device config | a mode no session or client uses is not probed |
| AC-13 | An IPv6 BFD neighbor configured in bgp, ospfv3 or static while `bfd bind-v6` is false | local `doctor-bfd-ipv6-not-bound` (error) naming each client and peer; fix `set bfd bind-v6 true` (A-2) | none |
| AC-14 | A session whose last transmit failed (no interface, no matching subnet, socket error) | runtime `doctor-bfd-send-failed` (warning) naming session, last error and time, from a per-session last-send-error record the engine keeps (S3) | none |
| AC-15 | A configured or client-requested session not Up past its detection time | runtime `doctor-bfd-session-down` (warning) naming peer, state, local and remote diagnostic and the client; fix: the `show bfd` session command | a session younger than its detection time (R-1) |
| AC-16 | Received packets failing authentication for a session | runtime `doctor-bfd-auth-failure` (warning) naming session and count, from a per-session failure count (S3); fix names the session's key config | none |
| AC-17 | Packets dropped before the FSM (no session matches, TTL gate, ingress interface unnamed) | runtime `doctor-bfd-packets-unmatched` (warning) naming loop, reason and count, from a per-loop drop counter by reason (S3) | none |
| AC-18 | A state change dropped for a client whose channel is full | transient `doctor-bfd-state-change-dropped` naming the client and session | none |

### LDP

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-20 | `ldp` configured with no `lsr-id` | local `doctor-ldp-no-lsr-id` (error); fix `set ldp lsr-id <address>` | none |
| AC-21 | Committed LDP config not in effect: `lsr-id` added after an idle start, changed `lsr-id`, `transport-address`, hello timers or `hop-count-max` on a running interface, connected FECs not recomputed | runtime `doctor-ldp-reload-not-applied` (warning) naming each setting with running and committed value, from the engine's started values against the applied config; fix: the restart command (A-3) | none while the reload defect stands; the row clears when reload applies the value |
| AC-22 | A FEC that wants a local label while the label space is used up | runtime `doctor-ldp-label-space-exhausted` (error) listing each such FEC and the label space size | none |
| AC-23 | An LDP interface whose device is absent or whose iface backend is unavailable | local (config names against configured interfaces) and runtime (`iface.Resolve` result) `doctor-ldp-interface-unavailable` (warning) naming the interface; fix names the interface config | none |
| AC-24 | A discovery socket failed on an interface (multicast listen, egress pin, read error) | runtime `doctor-ldp-discovery-failed` (error) naming the interface and error, from a per-interface discovery worker state the manager keeps (S3) | none |
| AC-25 | A Hello adjacency with no operational session past the KeepAlive hold | runtime `doctor-ldp-session-not-operational` (warning) naming the peer, the last failure (connect, Initialization rejection, send, Notification, KeepAlive expiry) and retries, from a per-adjacency last-failure record (S3); the message names the active-role case when the peer's transport address is higher | an adjacency inside its first setup backoff (R-1) |
| AC-26 | A label mapping dropped as a loop (hop count above `hop-count-max`) | transient `doctor-ldp-loop-detected` naming peer and FEC | none |
| AC-27 | A peer sends an unknown message type or an unknown TLV with the U bit clear | transient `doctor-ldp-peer-message-ignored` naming peer and type | none |
| AC-28 | An LDP LIB binding absent from the kernel AF_MPLS table, or a withdrawn one still present | runtime `doctor-mpls-label-not-programmed` (error) from `ldp-state`, comparing LIB local bindings with the kernel MPLS dump; fix `show mpls forwarding` | an entry inside one refresh of its change (R-5). Where `fib-kernel` is not loaded nothing consumes `(mpls-fib, entry)` events (fib-vpp programs only labeled system-RIB routes, from `sysribevents.BestChange` in `internal/plugins/fib/vpp/register.go`), so every binding reads not programmed and the message names the missing label writer, not a failed install; where `fib-kernel` recorded the failed install, the message names child 10's `doctor-fib-kernel-mpls-install` row as the cause |

### RSVP-TE

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-30 | `rsvp-te` configured with no `router-id` | local `doctor-rsvpte-no-router-id` (error); fix `set rsvp-te router-id <address>` | none |
| AC-31 | A changed `router-id`, or one added after an idle start | runtime `doctor-rsvpte-restart-needed` (warning) naming running and committed value or the idle engine; fix: the restart command (A-3) | none while the reload defect stands |
| AC-32 | Router-id set and the engine idle because the raw socket did not open | runtime half of `doctor-rsvpte-rawsock-unavailable` (error) from `rsvp-te-state`; the local `rsvp-te-rawsock` probe stays | none |
| AC-33 | Several interfaces, some without an `address` prefix | local `doctor-rsvpte-admission-unmapped` (warning) naming them; fix names the `address` leaf | a single-interface config, where admission maps without it |
| AC-34 | The receive loop ended on a socket or poller error | runtime `doctor-rsvpte-receive-stopped` (error) from reader liveness the transport records (S3); fix: the restart command | none |
| AC-35 | A configured tunnel or bypass not Up past one refresh period | runtime `doctor-rsvpte-tunnel-down` (warning) naming tunnel, state and the last PathErr, ResvErr or send error, from a per-tunnel last-failure record (S3); fix: the `show rsvp-te` tunnel command | a tunnel inside its first refresh period (R-1) |
| AC-36 | A transit or egress LSP refused with a PathErr (ERO, MTU, policy, TTL, ADSPEC, RRO, unknown class) | transient `doctor-rsvpte-lsp-refused` naming session, sender and reason | none |
| AC-37 | Admission denied on an interface | runtime `doctor-rsvpte-admission-denied` (warning) naming interface, reserved and capacity, plus a refusal count (S3); fix `show rsvp-te interface` | none |
| AC-38 | A malformed or inconsistent message from a neighbor | transient `doctor-rsvpte-peer-message-invalid` naming neighbor and the missing object | none |
| AC-39 | A signaling message that could not be sent or relayed | transient `doctor-rsvpte-send-failed` naming message type, session and error, with a per-type send-failure counter (S3) | none |
| AC-40 | `transport.LocalAddresses()` failing | runtime `doctor-rsvpte-local-addresses` (error) with the error and time | none |
| AC-41 | A protected LSP with no Up bypass, or a failed local-repair FIB programming | runtime `doctor-rsvpte-frr-unprotected` (warning) naming LSP and bypass state; fix `show rsvp-te fast-reroute` | none |
| AC-42 | Interface-down events dropped (worker backlog, no bus subscription) | transient `doctor-rsvpte-link-event-dropped` naming the interface, with a drop counter (S3) | none |
| AC-43 | An RSVP `lspTable` label absent from the kernel AF_MPLS table, or a withdrawn one present | runtime `doctor-mpls-label-not-programmed` from `rsvp-te-state` | an entry inside one refresh of its change (R-5); the missing-writer and failed-install messages are AC-28's |

### OSPF

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-50 | A configured OSPF or OSPFv3 interface not running (device missing, no address, join or socket failure, receive path ended) | local (configured names against `/sys/class/net`) and runtime (transport enabled minus open) `doctor-ospf-interface-not-running` (error) naming interface, AF and cause | an interface whose link is down administratively by config |
| AC-51 | A neighbor not Full because a parameter disagrees (mask, hello or dead interval, E/N bits, Instance ID, AF bit, DD MTU, DD sequence, request limit) | runtime `doctor-ospf-neighbor-parameter-mismatch` (warning) naming interface, neighbor and the parameter with both values, from the drop counters by reason and the neighbor's recorded events | none |
| AC-52 | Received packets failing authentication | runtime `doctor-ospf-auth-failed` (warning) naming interface, reason and count from `ze_ospf_auth_failures_total` | none |
| AC-53 | The last key of an interface key chain expired | local and runtime `doctor-ospf-key-chain-expired` (warning) naming chain and interface; fix names the key-chain key to add | none |
| AC-54 | Cryptographic-ESN boot-count reservation failed | runtime `doctor-ospf-auth-sequence-unreserved` (error) naming the error and the refused-signing count (S3) | none |
| AC-55 | BFD configured for a Full neighbor with no BFD session | runtime `doctor-ospf-bfd-session-missing` (warning) naming neighbor and the `RegisterFailures` reason | none |
| AC-56 | BFD declared an OSPF neighbor down | transient `doctor-ospf-bfd-neighbor-down` naming interface and neighbor | none |
| AC-57 | An interface not LDP-synchronized past its hold-down after LDP was operational (RFC 5443 Section 3) | runtime `doctor-ospf-ldp-sync-stuck` (warning) naming interface and state | an interface inside its hold-down timer |
| AC-58 | An OSPFv3 IPsec interface or neighbor not protected (no link-local source, dataplane down, SA or policy install failed) | runtime `doctor-ospfv3-ipsec-not-installed` (error) naming interface, neighbor and the failure reason | none |
| AC-59 | An OSPFv3 IPsec SA or policy left in the kernel for a gone neighbor or interface | runtime `doctor-ospfv3-ipsec-state-leaked` (warning) comparing the XFRM dump with `ipsecInstaller.installed` | none |
| AC-60 | A GR restart fact that could not be cleared | runtime `doctor-ospf-graceful-restart-stale` (warning) from `ospf-state` | none during an active restart |
| AC-61 | An area or AS-scope LSDB at its store cap | runtime `doctor-ospf-lsdb-full` (error) naming scope, count and cap | none |
| AC-62 | A redistributed or default route not originated or not withdrawn | runtime `doctor-ospf-redistribute-not-originated` (warning) comparing consumer `offered` with engine imports and self Type-5/7 LSAs | none |
| AC-63 | A `redistribute` rule targeting OSPF for a family no OSPF AF originates | local `doctor-redistribute-family-unsupported` (warning) from `ospf-config-sanity` naming the rule | none |
| AC-64 | An SPF route change not delivered to redistribution consumers | transient `doctor-redistribute-source-undelivered` from the OSPF source | none |
| AC-65 | A BGP-LS snapshot not delivered | transient `doctor-bgpls-snapshot-undelivered` from the OSPF exporter | none |
| AC-66 | A BGP-LS inter-AS link withheld (remote ASBR with no unique Router-ID) | runtime `doctor-ospf-bgpls-inter-as-withheld` naming the link | none |
| AC-67 | SR label entries not delivered to the MPLS FIB | transient `doctor-ospf-sr-fib-undelivered` | none |
| AC-68 | Malformed RFC 7684 opaque LSAs received | transient `doctor-ospf-opaque-malformed` naming advertising router and TLV | none |
| AC-69 | SPF installer with no sink | runtime `doctor-igp-route-sink-missing` (error) from `ospf-state` | none |

### IS-IS

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-70 | A configured circuit not running (device missing, socket open failed, receive path ended) | local (config names against `/sys/class/net`) and runtime (transport enabled minus open) `doctor-isis-circuit-not-running` (error) naming circuit and cause | a circuit whose link is down by config |
| AC-71 | Hellos rejected so no adjacency forms (area, level, too many areas, table full, three-way state, ISH not point-to-point) | runtime `doctor-isis-hello-rejected` (warning) naming circuit and reason from `ze_isis_frames_dropped_total{reason}` | none |
| AC-72 | Another system uses this node's System ID | runtime `doctor-isis-system-id-duplicate` (error) from an own-ID conflict record (S3) naming the circuit or LSP ID | none |
| AC-73 | IS-IS packets failing authentication | runtime `doctor-isis-auth-failed` (warning) naming level, circuit and count | none |
| AC-74 | A level's LSDB at `MaxLSPsPerLevel` | runtime `doctor-isis-lsdb-full` (error) | none |
| AC-75 | Own state needing more than 256 fragments | runtime `doctor-isis-lsp-fragments-exhausted` (error) naming the count dropped, from a truncation record the encoder keeps (S3) | none |
| AC-76 | A redistributed route not originated | runtime `doctor-isis-redistribute-not-originated` | none |
| AC-77 | A `redistribute` rule targeting IS-IS for a family no IS-IS protocol carries | local `doctor-redistribute-family-unsupported` from `isis-config-sanity` | none |
| AC-78 | SPF route change or BGP-LS snapshot not delivered | transient `doctor-redistribute-source-undelivered`, `doctor-bgpls-snapshot-undelivered` from the IS-IS sources | none |
| AC-79 | SPF installer with no sink | runtime `doctor-igp-route-sink-missing` from `isis-state` | none |

### Defects owned here

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-80 | IS-IS with two circuits, the first on an absent device | IS-IS starts: the second circuit opens, connected prefixes are advertised and the own LSP is originated; the failed circuit logs at Warn with `doctor-isis-circuit-not-running` and stays enabled for the rescan, as OSPF's `openInterfaces` does | none: one circuit's failure is never fatal to the protocol |
| AC-81 | `rsvp-te` with a tunnel and no engine (`eng` nil) | `ze_rsvpte_path_sent_total` keeps its value; the tunnel's state reads not signaled, and AC-30 or AC-32 reports why | none |
| AC-82 | The ResvErr egress Warn and the PathErr Info lines | the RSVP ERROR_SPEC code is logged under `error-code`; the Warn line's `code` attribute holds `doctor-rsvpte-lsp-refused` | none: `code` names a doctor code only |
| AC-83 | A reload pins a live single-hop BFD loop's sessions to another interface, or commits `bind-v6 true` after a loop exists (`runtimeState.loopFor`, `internal/component/bfd/bfd.go`) | the reload applies it: the affected loop is rebuilt on the requested device and address family and its sessions move to it, or the commit is refused at verify with the reason and the restart command; an accepted reload never leaves the old binding running. AC-10 then reports only the window before the apply | none: an accepted reload that is not applied is a logic bug (journal `invariant-enforced-by-an-absent-call-site.md`, 2026-11-10 BFD row) |
| AC-84 | A reload adds `lsr-id` after an idle LDP start, or changes `lsr-id`, `transport-address`, hello or keepalive timers or `hop-count-max` on a running engine (`OnConfigApply`, `discoveryManager.reconcile`, `internal/plugins/ldp/register.go`) | the reload applies each setting: an idle engine starts, and discovery and sessions affected by a changed value restart with it; or the commit refuses at verify naming the setting. AC-21 then reports only the window before the apply | none: same defect class (journal 2026-11-10 LDP row) |
| AC-85 | A reload changes the RSVP-TE `router-id`, or adds one after an idle start (`OnConfigApply`, `internal/plugins/rsvpte/register.go`) | the reload applies it: an idle engine opens its transport and signals, and a changed id re-signals every LSP under the new id; or the commit refuses at verify naming `router-id`. The "keeping the running router-id" Warn is gone. AC-31 then reports only the window before the apply | none: same defect class (journal 2026-11-10 RSVP-TE row) |
| AC-86 | An LDP Hello adjacency with a peer whose transport address is higher than ze's, then one whose is lower | RFC 5036 Section 2.5.2: ze compares A1 and A2, dials only when it is active, and listens on TCP 646 and waits when it is passive, then runs the passive Initialization exchange of Section 2.5.3 (rows RFC5036-2.5.1-3, RFC5036-2.5.3-7); a session forms in both cases, proved against FRR in an interop scenario | none: an RFC gap of an implemented RFC (journal `mandatory-algorithm-absent.md`, 2026-11-10 ldp row) |

### Not problem classes

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-90 | Plugin exit sites of bfd, ldp, rsvpte, ospf, isis | carry `doctor-plugin-stopped` | none |
| AC-91 | Event-bus emit failures and missing-bus returns (LDP, RSVP-TE) | carry `doctor-internal-defect` and raise it on the report bus | none |
| AC-92 | Doctor probe close errors (LDP, RSVP-TE) | carry `doctor-internal-defect` | none |
| AC-93 | Programmer invariants (RSVP reservation rollback, ResvErr carrier limit, LDP keepalive range, LDP post-panic Notification, BFD MSG_CTRUNC) | carry `doctor-internal-defect` | none |
| AC-94 | Shutdown cleanup (BFD loop stop and ReleaseSession, RSVP transport close) | carry `doctor-internal-defect` | none |
| AC-95 | LDP Notification send failures while a session already closes | carry `doctor-ldp-session-not-operational` | none |
| AC-96 | BFD non-Linux source-address site | carries `doctor-bfd-send-failed` | none |
| AC-97 | OSPF, OSPFv3 and IS-IS socket and fd close errors on teardown, rescan and probe | carry `doctor-internal-defect` (the CODES merge of the per-protocol close codes) | none |
| AC-98 | OSPF and IS-IS Ze-defect sites (registration errors, self-built LSA normalise, metric style missing, undecodable iface event, malformed producer prefix, post-panic follow-ups) | carry `doctor-internal-defect` | none |
| AC-100 | The existing doctor findings of the area that set no `Fix` (13 on 2026-11-11, after commit 10c6647f6d made the gate judge them): `ospf/doctor.go` 4, `ospf/doctor_debug.go` 1, `ospf/doctor_ipsec.go` 1, `ospf/sr_doctor.go` 1, `ospf/unnumbered.go` 1, `ospf/transport/doctor.go` 1, `ospf/v3/transport/doctor.go` 1, `isis/doctor.go` 2, `isis/transport/doctor.go` 1 | each sets `Fix` to the command or config statement that clears it; a message that spells its fix inline drops that text, since the renderer draws `Fix` on its own line | none: a finding with no `Fix` is an uncovered site (AC-1) |
| AC-99 | "ospf: no ospf config" at plugin start | logged at Info: nothing is configured, so nothing is degraded | none: the line is not a problem, so it is not a Warn |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` on a box whose BFD peer never answers | `HandleDoctor` -> `collectDoctorChecksFrom` -> bfd `OnDoctorCheck` -> session snapshot | `test/plugin/show-doctor-bfd-state.ci` |
| 2 | runs `ze doctor` before committing `ldp` with no `lsr-id` | `doctor.Answer` -> `ldp-config` -> config tree | `test/ui/doctor-routing-protocols-config.ci` |
| 3 | runs `show doctor` with IS-IS on an absent device and sees IS-IS still up on the others | IS-IS start -> `isis-state` | `test/plugin/show-doctor-isis-state.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginReportRaiseBothTransports` | `pkg/plugin/sdk/report_raise_test.go` | AC-3 | |
| `TestDoctorBFDState`, `TestDoctorBFDConfig` | `internal/component/bfd/doctor_test.go` | AC-10, AC-11, AC-13 to AC-18, one case per code | |
| `TestDoctorBFDPorts` | `internal/component/bfd/doctor_ports_test.go` | AC-12, AC-9c BFD half, held-by-engine survivor | |
| `TestDoctorLDPConfig`, `TestDoctorLDPState` | `internal/plugins/ldp/doctor_test.go` | AC-20 to AC-28 | |
| `TestLDPPortProbeWhileEngineHolds` | `internal/plugins/ldp/doctor_linux_test.go` | AC-9c LDP half, A-1 | |
| `TestDoctorRSVPTEConfig`, `TestDoctorRSVPTEState` | `internal/plugins/rsvpte/doctor_test.go` | AC-30 to AC-43 | |
| `TestSetupTunnelNoEngineSendsNoPath` | `internal/plugins/rsvpte/register_test.go` | AC-81 | |
| `TestResvErrLogsErrorCodeKey` | `internal/plugins/rsvpte/reservation_test.go` | AC-82 | |
| `TestDoctorOSPFStateBFDAbsent` | `internal/plugins/ospf/doctor_test.go` | AC-9a, AC-9b; `ospfConfigDiagnostics` answers only config findings | |
| `TestDoctorOSPFState` | `internal/plugins/ospf/doctor_state_test.go` | AC-50 to AC-69, one case per code | |
| `TestDoctorISISState` | `internal/plugins/isis/doctor_test.go` | AC-70 to AC-79 | |
| `TestISISOneCircuitOpenFailureKeepsOthers` | `internal/plugins/isis/server_test.go` | AC-80, A-6 | |
| `TestDoctorRegisteredCheckCodesHaveMetadata` | `internal/component/doctor` (existing) | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IS-IS own fragments | 0-256 | 256 (no row) | N/A | 257 (AC-75 row) |
| OSPF and IS-IS LSDB entries | 0-cap | cap minus one (no row) | N/A | cap (AC-61, AC-74 row) |
| "not Up" age against the protocol timer | 0-timer | timer (no row) | N/A | timer plus one tick (row) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-routing-protocols-config` | `test/ui/` | local config classes for all five protocols | |
| `show-doctor-bfd-state` | `test/plugin/` | BFD runtime rows | |
| `show-doctor-ldp-state` | `test/plugin/` | LDP runtime rows | |
| `show-doctor-rsvpte-state` | `test/plugin/` | RSVP-TE runtime rows | |
| `show-doctor-rsvpte-recent-event` | `test/plugin/` | a transient class through the report bus, the plugin forked | |
| `show-doctor-ospf-state` | `test/plugin/` | OSPF runtime rows, BFD-absent moved to runtime | |
| `show-doctor-isis-state` | `test/plugin/` | IS-IS runtime rows, IS-IS survives a missing device | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (doctor ACs) | - | - | doctor reads state; the IS-IS start fix changes which circuits open, not what is sent on one, and the existing IS-IS scenarios against FRR in `test/interop/scenarios/` keep proving the wire | |
| `ldp-passive-role-frr` (new, AC-86) | `test/interop/scenarios/ldp-passive-role-frr/` | FRR (ldpd; read its role decision in the source before writing the check) | with FRR's transport address higher, FRR dials and ze, passive, accepts on TCP 646 and the session reaches OPERATIONAL; proved red with the listener reverted | |
| existing BFD, LDP and RSVP-TE scenarios (AC-83 to AC-85) | `test/interop/scenarios/`, `test/interop-rsvpte/scenarios/` | FRR, freeRouter | a reload that changes the BFD binding, LDP `lsr-id` or RSVP-TE `router-id` is applied on the wire; a reload step is added where the scenario lacks one | |

## Files to Modify

By package. The per-site list of each package is the inventory's source-site
column and `./le arch doctor-coverage report`; each phase reads the producing
function of every site before it edits it.

- `internal/core/diagnostic/codes.go` - metadata for every new code; `doctor-ldp-port-unavailable` description (UDP only)
- `internal/component/doctor/checks_listener.go` - delete `extractBFDListeners` and its `collectAllListeners` call
- `internal/component/bfd/bfd.go` - store loop start errors; compare `bindV6`; SDK `DoctorChecks` and `OnDoctorCheck`; codes on Warn sites
- the `internal/component/bfd/engine` and `internal/component/bfd/transport` packages, and BFD's metrics - per-session last send error and auth count, per-loop drop counter by reason, subscriber drop raise
- `internal/plugins/ldp/doctor.go` - UDP-only probe, engine-held yield; config checks
- `internal/plugins/ldp/register.go` and the rest of the `internal/plugins/ldp` package (discovery manager, session, FIB emit) - discovery worker state, per-adjacency last failure, runtime check, codes on sites
- `internal/plugins/rsvpte/register.go` - `setupTunnel` defect, config and runtime checks, codes
- `internal/plugins/rsvpte/engine.go`, `internal/plugins/rsvpte/reservation.go` and the rest of the `internal/plugins/rsvpte` package (reroute, FRR, confirmation, transport) - records of S3, `error-code` key, codes on sites, raises
- `internal/plugins/ospf/doctor.go`, `internal/plugins/ospf/register.go` - BFD half to `ospf-state`, `runtime` declaration, new codes
- `internal/plugins/ospf/instance.go` and the rest of the `internal/plugins/ospf` tree (BFD client, auth key store, LDP sync, IPsec installer, GR restarter, redistribution, BGP-LS export, SR FIB, opaque TLVs, iface, neighbor, lsdb, spf, transport, v3 transport) - state reads, codes on sites, raises
- `internal/plugins/isis/server.go`, `internal/plugins/isis/register.go` - `openCircuits` defect, `isis-state`, codes
- the rest of the `internal/plugins/isis` tree (own-LSP conflict, LSDB wiring, auth, BGP-LS export, redistribution, lsdb, spf, transport) - records, codes, raises
- `internal/plugins/ldp/register.go` - reload applies `lsr-id`, transport and timers (AC-84); the RFC 5036 Section 2.5.2 role comparison and TCP 646 listener (AC-86)
- `internal/plugins/rsvpte/register.go` - reload applies `router-id` (AC-85)
- `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/ldp/mpls-ldp.md`, `docs/architecture/bfd.md`, `docs/architecture/api/process-protocol.md`, `docs/guide/ospf.md`, `docs/guide/isis.md`, `docs/guide/mpls.md`, `docs/guide/rsvp-te.md`, `docs/guide/bfd.md` - the checks each owner registers, the S2 op

## Files to Create
- `internal/component/bfd/doctor.go`, `internal/component/bfd/doctor_test.go`, `internal/component/bfd/doctor_ports.go`, `internal/component/bfd/doctor_ports_test.go`
- `internal/plugins/ospf/doctor_state.go`, `internal/plugins/ospf/doctor_state_test.go`
- `internal/plugins/isis/doctor_state.go`
- `internal/plugins/rsvpte/doctor_state.go`, `internal/plugins/rsvpte/reservation_test.go`
- `internal/plugins/ldp/doctor_state.go`, `internal/plugins/ldp/doctor_linux_test.go`
- `test/ui/doctor-routing-protocols-config.ci`, `test/plugin/show-doctor-bfd-state.ci`, `test/plugin/show-doctor-ldp-state.ci`, `test/plugin/show-doctor-rsvpte-state.ci`, `test/plugin/show-doctor-rsvpte-recent-event.ci`, `test/plugin/show-doctor-ospf-state.ci`, `test/plugin/show-doctor-isis-state.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config leaf or RPC added |
| YANG validation constraints | N-A | none added |
| YANG custom validators | N-A | none added |
| CLI commands/flags | N-A | `show doctor` and `ze doctor` exist |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf added |
| Functional test for new RPC/API | No new RPC here (child 12 P0b owns it) | AC-3 still proved by `test/plugin/show-doctor-rsvpte-recent-event.ci` with the plugin forked |
| Pipe completeness | N-A | doctor output exists |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this spec is that work: owner checks, `internal/core/diagnostic/codes.go`, unit and functional tests |
| Prometheus counters/metrics | Yes | S3: `ze_bfd_packets_unmatched_total{mode,reason}`, `ze_bfd_state_changes_dropped_total{client}`, `ze_rsvpte_send_failures_total{type}`, `ze_rsvpte_link_events_dropped_total`, `ze_rsvpte_admission_denied_total{interface}`, `ze_ospf_auth_signing_refused_total{interface}`; names confirmed against each owner's metrics at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor rows extend an existing feature; grep `features/` for these checks at implementation |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | the raise RPC and its `docs/architecture/api/process-protocol.md` entry are child 12 phase P0b's |
| 5 | Plugin added/changed? | No | no plugin added |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`, `docs/guide/isis.md`, `docs/guide/mpls.md`, `docs/guide/rsvp-te.md`, `docs/guide/bfd.md`: what doctor reports for each |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | child 12 phase P0b documents the SDK raise in `docs/plugin-development/` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC-tagged behavior changes |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (BFD port check owner), `docs/architecture/ldp/mpls-ldp.md` (port check UDP only), `docs/architecture/bfd.md`, `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/ospf/ospf-4-component-config.md` and `docs/architecture/ospf/ospf-13-cli-diag-interop.md` (the OSPF doctor checks), `docs/architecture/isis/isis-4-component-config.md` (IS-IS start no longer stops on one circuit, `isis-state`). Declared by changed files and unaffected: `docs/features/ai-first.md` (names doctor as a surface, not these checks) and `rfc/short/rfc5880.md` (no BFD requirement or test claim changes) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | the owners' telemetry pages for the S3 counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration of those kinds |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-13-routing-protocols.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | doctor examples in the guides are checked against the new rows |

## Implementation Steps

### Implementation Phases

Each phase is sized for one agent of about 60 tool calls, lands as its own
commit once its scoped package tests pass, and builds the owner's records (S3)
before the checks that read them.

| Phase | Work | ACs | Needs | Parallel |
|-------|------|-----|-------|----------|
| P0 wiring | the five SDK-declared runtime checks and the config checks registered with their codes and empty bodies; the seven `.ci` files written failing; `codes.go` metadata for every code | AC-2 | - | - |
| S2 plugin report raise | no build here: the route is `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0b; this phase proves AC-3 by a forked protocol plugin raising through it | AC-3 | P0, child 12 P0b | ∥ P1 |
| P1 BFD | defect AC-83 first; S3 records (loop start error, per-session send error and auth count, drop counters, subscriber drop); delete `extractBFDListeners`, then `bfd-ports`, `bfd-state`, config checks; codes on every site | AC-9c (BFD), AC-10 to AC-18, AC-83, AC-90, AC-94, AC-96 | P0, child 12 P0b | ∥ P2 |
| P2 LDP | defect AC-84 first; discovery worker state, per-adjacency last failure; `ldp-port` UDP-only with the engine-held yield; `ldp-config`, `ldp-state`, MPLS label compare; codes | AC-9c (LDP), AC-20 to AC-28, AC-84, AC-91 to AC-93, AC-95 (LDP sites) | P0, child 12 P0b | ∥ P1 |
| P2b LDP passive role | RFC 5036 Section 2.5.2 role comparison, TCP 646 listener, passive Initialization exchange, FRR interop scenario; `ldp-port` then probes TCP 646 too | AC-86 | P2 | - |
| P3a RSVP-TE state | defects AC-81, AC-82 and AC-85 first; config checks; receive liveness, per-tunnel last failure, admission refusal count; `rsvp-te-state` | AC-30 to AC-35, AC-37, AC-40, AC-41, AC-43, AC-81, AC-82, AC-85 | P0 | ∥ P1, P2 |
| P3b RSVP-TE events | send-failure counter, link-event drop counter; transient raises; codes on the send sites and the remaining sites | AC-36, AC-38, AC-39, AC-42, AC-91 to AC-94 (RSVP-TE sites) | P3a, child 12 P0b | - |
| P4a OSPF config and adjacency | AC-9a, AC-9b; interfaces, neighbors, auth, key chain, BFD, LDP sync | AC-9a, AC-9b, AC-50 to AC-57 | P0, child 12 P0b | ∥ P5 |
| P4b OSPF databases and exports | IPsec, GR, LSDB, redistribution, BGP-LS, SR, opaque, sink; not-problem sites; AC-99 | AC-58 to AC-69, AC-90, AC-97, AC-98 (OSPF sites), AC-99 | P4a | - |
| P5 IS-IS | defect AC-80 first; `isis-state`; records for own-ID conflict and fragment truncation; codes | AC-70 to AC-80, AC-90, AC-97, AC-98 (IS-IS sites) | P0, child 12 P0b | ∥ P4a |
| P6 gate | `Fix` on the existing findings; `./le arch doctor-coverage report` over the area's package trees reads every site covered, findings included | AC-1, AC-100 | all | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a check, a code and a red-proven unit test |
| Process boundary | No runtime check reads plugin state by a Go call from the daemon; each is `OnDoctorCheck` in the protocol's process |
| Correctness | No row fires on a healthy start (R-1); no bind probe reports a port the running engine holds (AC-9c) |
| Naming | codes exactly as the canonical names in the inventory; log key `code` holds only doctor codes |
| Rule: no-layering | `extractBFDListeners` deleted before `bfd-ports` lands |
| Rule: user-facing errors | every `Fix` is the command alone, on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every code registered and declared | `TestDoctorRegisteredCheckCodesHaveMetadata`; `ze show diagnostic <code>` |
| Every site in the area covered | `./le arch doctor-coverage report` |
| IS-IS survives a missing circuit device | `TestISISOneCircuitOpenFailureKeepsOthers`, `test/plugin/show-doctor-isis-state.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Peer-controlled text in messages | neighbor-supplied names or TLV values in a finding are quoted and length-bounded |
| Report flooding | a peer sending malformed messages cannot fill the report ring (R-3) |
| Secrets | an auth finding names the key chain and key id, never key material |

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

- A post-config bind probe runs in the daemon on the daemon route, so it can only be right before the engine binds; once the engine runs, its own bind result is the answer (AC-9c)
- A check that reads `api.GetService` is only meaningful in the process whose BFD client would use it, which is the OSPF plugin's own process (AC-9a)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `one-failure-skips-independent-startup-work.md` | 2026-11-10 | IS-IS `openCircuits` returns the first circuit error from `OnStarted` |
| `counter-counts-the-wrong-packets.md` | 2026-11-10 | RSVP-TE `setupTunnel` counts a PATH sent with no engine |
| `field-carries-two-meanings.md` | 2026-11-10 | RSVP-TE ResvErr Warn logs the ERROR_SPEC code under `code` |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-11-10 | BFD `loopFor` accepts a reload that rebinds the device or `bind-v6` and applies neither (AC-83) |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-11-10 | LDP reload accepts `lsr-id`, transport and timer changes and applies only the interface set (AC-84) |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-11-10 | RSVP-TE reload ignores a changed or late `router-id` (AC-85) |
| `mandatory-algorithm-absent.md` | 2026-11-10 | LDP has no passive role: RFC 5036 Section 2.5.2 role comparison and TCP 646 listener absent (AC-86) |
| `lock-held-across-a-join.md` | 2026-11-13 | LDP `startSessionForAdj` holds `sessionsMu` across the 5s TCP dial |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Runtime classes are SDK-declared checks answered by `OnDoctorCheck` | `RegisterDoctorCheck` with `Phase: runtime` reading a package variable | the registry check runs in the daemon process and reads nothing when the plugin runs forked (`ai/doors/system.md`) |
| BFD's port probe moves into the BFD component | extend doctor's `extractBFDListeners` to more ports | proximity: the owner knows its modes, VRFs and whether its own loop holds the port |
| Transient classes go through the report bus with a plugin raise path | each protocol keeps its own recent-event record | the parent's agreed design reports transient classes through the bus; a second record would be a parallel mechanism |
| OSPF "no config" line demoted to Info | register `doctor-ospf-idle` | an unconfigured plugin is not a problem; a code for it would be a row reporting health |
| `doctor-mpls-label-not-programmed` is answered by LDP and RSVP-TE, each for its own labels | one check in fib-kernel | only the protocol knows which labels it holds |

## Known Limitations
- None. The reload defects behind AC-10, AC-21 and AC-31 are fixed here (AC-83 to AC-85), and the LDP passive role is owed here as an RFC 5036 gap (AC-86).

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests under `./le job run` for `internal/component/bfd`, `internal/plugins/ldp`, `internal/plugins/rsvpte`, `internal/plugins/ospf`, `internal/plugins/isis`, `internal/component/doctor` and `pkg/plugin/sdk`, and the named `.ci` files through `./le test bfd`, `./le test ldp`, `./le test rsvpte`, `./le test ospf`, `./le test isis` and `./le test ui` (one file each, never a whole suite). No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output of `./le test unit plugins` scoped to the changed packages, and of each named `.ci` file)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A with the reason above)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-13-routing-protocols.md` only, in the same `./le commit create` script
