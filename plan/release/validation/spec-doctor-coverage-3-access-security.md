# Spec: doctor-coverage-3-access-security

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md` (Goal and scope): `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of the parent makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, the `Fix` field, the recent-events check and the gate, landed in commits ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` for the rows it owns (Existing Mechanisms) and the L2TP per-peer record it adds; `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a for `doctor-plugin-stopped`, `doctor-plugin-crashed`, `doctor-plugin-external-unsupported` and `doctor-internal-defect` (and the transient-code registration), and its AC-35 (phase P6) for `doctor-env-invalid-value` (AC-54); `plan/release/validation/spec-doctor-coverage-5-appliance.md` for `doctor-dns-source-host-only-dropped` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Inventory: `plan/audits/doctor-problem-inventory/access-security.md` (slices A1 IKE, AAA and PKI; A2 L2TP, PPP, PPPoE and the subscriber plugins).

## Task

Doctor reports every problem class of the access-security area (IKEv2/IPsec,
AAA, RADIUS, TACACS+, authz, PKI, L2TP, PPP, PPPoE and the subscriber plugins)
with its cause in plain words and the command that fixes it, on every surface
through `show doctor`. Every Warn and Error log line, and every report-bus
raise, in the area names its doctor code (parent O-2), so the doctor-coverage
gate counts it covered. The three IKE checks that read engine state from a
config phase move to `runtime`, so the local route never reads empty engine
state as healthy (parent AC-9).

Rows owned by child 2 stay there and are not respecified here.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform constraints
  → Constraint: a check is registered by the package that owns the state it reads ("Shared machinery", readiness check row); no central list names a check
  → Constraint: a plugin can run forked; the subscriber plugins and the IKE engine declare plugin doctor checks (`registry.DoctorCheckDef`), which reach `show doctor` through `collectPluginDoctorChecks` whichever way they run
- [ ] `ai/doors/access-security.md` - the area door
  → Constraint: readers of IKE state outside `ike/engine` use `ipsecinventory` and `ikeprobe`, never the engine package
  → Constraint: a backend that cannot build degrades, it does not abort (`aaa/types.go` `Build`); the doctor row reports the degradation and never turns it into an abort
  → Constraint: one subscriber auth handler, last write wins (`l2tp/subscriber/handler_registry.go`); doctor reports which handler answers and does not redesign the slot (open trap, `plan/journal/design-claim-never-measured.md` 2026-09-03)
  → Constraint: subscriber auth, pool and shaper reach a session only through the `l2tp/subscriber` handler registries
- [ ] `ai/doors/core.md`, `ai/doors/hub-engine.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md` - doors of the paths this spec names
  → Constraint: a core registry holds value types and is the only list; new codes go in `internal/core/diagnostic/codes.go` and nowhere else
  → Constraint: L2TP and PPPoE boot through `Engine.RegisterSubsystem` (`registerBNGSubsystems`); a check that reads the subsystem runs in the daemon
  → Constraint: a field name in `rpc` is wire; this spec adds no field, only codes
  → Constraint: `ze` never links `internal/le`; the gate reads codes from the registries
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Data Flow 1b, 2a (amended), 4b
  → Decision: a `runtime` check receives `Platform` only; it reads its owner's live state, never `Tree` or `ConfigDir`
  → Decision: a transient code counts as covered only when the check of the code's owner declares it; the `recent-events` check then reports the bus event under that code when its metadata is registered
  → Constraint: health rows take no code; each is mapped to a doctor check by hand
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - rows it owns
  → Constraint: `ipsec-udp-encap` and `ipsec-virtual-ip-pool` move to `runtime` in child 2; the implicit discard, the XFRM exemptions, the VTI peer with no Child SA, the L2TP unresponsive peer and stuck tunnel and session, and the three PPPoE client rows are child 2's
  → Decision: child 2's A-3 asks for an IKE per-peer last failure; this spec builds it (phase P1), and child 2 reads it
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the runtime phase
  → Constraint: a `runtime` check applies nothing and only reads
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: the message states the problem and why; the fix is the command alone, on its own line, in `Fix`
- [ ] `ai/rules/spec-no-code.md`
  → Constraint: tables, no code

**Key insights:**
- `doctor-ipsec-listen` has metadata in `internal/core/diagnostic/codes.go` and no producer: no check emits it, so `ze show diagnostic doctor-ipsec-listen` describes a report doctor never makes
- The IKE check for `vpn ipsec interface` emits `doctor-ipsec-iface` (`CodeDoctorIPsecIface`), not `doctor-ipsec-interface` as slice A1 wrote; the code stays as registered
- PKI raises its expiry warnings under `cert-expiry` and `cert-expired` (`pki/health.go` `reportCodeExpiry`, `reportCodeExpired`); CODES.md maps both to `doctor-pki-cert`; no other reader of those two strings exists in the tree
- The `l2tp` health row is registered from `init()` with `health.Register`, while the subsystem is constructed only when L2TP is enabled or a listener is configured (`cmd/ze/hub/register_l2tp.go` `registerBNGSubsystems`), so an unconfigured `ze_l2tp` build reports `l2tp` degraded "subsystem not running"

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/register.go` - `init()` registers the `ike` plugin with ten `DoctorChecks`, all `rpc.DoctorPhasePostConfig`; `runEngine` logs "dataplane load failed, SA installation disabled" and keeps running
- [ ] `internal/component/ike/engine/doctor_interface_address.go` - `checkIPsecInterfaceAddress` ignores its context and reads `interfaceWaitPtr`, nil outside the running engine
- [ ] `internal/component/ike/engine/doctor_application_override.go` - `checkIPsecApplicationOverride` reads `overrideRefusal`, `appliedOverride`, `dataplane.Get()`; `checkIPsecSPDWriters` returns nothing while `appliedOverride` is nil
- [ ] `internal/component/ike/engine/health.go` - `checkIPsecHealth`: down when no `ActiveTable`, degraded for no or some established SAs, drift, or an unreadable dataplane; no code, no fix
- [ ] `internal/component/ike/transport/udp.go` - `UDPTransport.Run` logs "inbound queue full, dropping packet" and the refusal-queue drop, with no counter
- [ ] `internal/component/ike/transport/udp_linux.go` - `drainErrorQueue` logs a failed drain
- [ ] `internal/component/ike/dataplane/espform_linux.go` - `espFormReceiver.run` logs "stopped reading refused ESP" and returns; no run state kept
- [ ] `internal/core/diagnostic/codes.go` - the code constants; `CodeDoctorIPsecListen` with metadata and no producer
- [ ] `internal/component/pki/health.go` - `checkHealth` over the loaded store; `RaiseExpiryWarnings` raises `cert-expiry` / `cert-expired` on source `pki`
- [ ] `internal/component/pki/ca.go` - `NewEphemeralRoot` logs "using ephemeral local CA" and keeps no flag a check can read
- [ ] `internal/component/pki/doctor.go` - `caRootDoctorCheck` (`pki-ca-root`, pre-config) reads the store, not the daemon's root
- [ ] `internal/component/aaa/types.go` - `Build` drops a backend that fails to build, logs at Error, returns the error beside the bundle; nothing keeps the dropped list after boot
- [ ] `internal/component/aaa/doctor.go` - `aaa-no-local-fallback` (post-config) over the config tree
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `aaaBundle` holds the live bundle; `swapAAABundle` installs boot and reload results
- [ ] `internal/component/authz/authz.go` - the authorization store denies with "no applicable profile" at Warn, no code
- [ ] `internal/component/l2tp/register.go` - `init()` calls `health.Register("l2tp", checkHealth)` unconditionally
- [ ] `internal/component/l2tp/health.go` - `checkHealth` reads `LookupService()` and `kernelDataPlane()`; "subsystem not running" when no service is published
- [ ] `cmd/ze/hub/register_l2tp.go` - `registerBNGSubsystems` constructs the L2TP subsystem only for `Enabled` or a listener; PPPoE always
- [ ] `internal/component/l2tp/subscriber/handler_registry.go` - one auth, one pool and one shaper handler, each last write wins, with getters
- [ ] `internal/component/l2tp/pppoeclient/session.go`, `internal/component/l2tp/pppoeclient/ipcp_independent.go` - LCP codes logged under the `code` key
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` (runtime) renders `report.Errors` and `report.Warnings`, one row per (kind, source, code, subject), under the event's own code when `diagnostic.Lookup` knows it

**Behavior to preserve:**
- every existing check name, code and message in the area; `doctor-ipsec-iface` keeps its spelling
- the AAA chain degrades on a backend build failure and never aborts (door boundary)
- the subscriber handler slots keep their single-slot semantics (this spec reports, it does not redesign)
- health row names `ipsec`, `pki`, `l2tp`

**Behavior to change:**
- the three IKE checks that read engine state run in the `runtime` phase
- `doctor-ipsec-listen` gains its producer
- the PKI expiry raises carry `doctor-pki-cert`
- the `l2tp` health row is registered only while the L2TP subsystem exists
- every gap class below gets a doctor check (local or runtime) with code, severity, message and fix
- every Warn and Error line in the area carries a doctor code; protocol codes logged under the `code` key move to a distinct key

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| IKE plugin doctor checks | `ike/engine/register.go` `DoctorChecks` (`ipsec-interface`, `ipsec-udp-encap`, `ipsec-cert-url`, `ipsec-cookie-threshold`, `ipsec-virtual-ip-pool`, `ipsec-interface-address`, `ipsec-application-override`, `ipsec-spd-writers`, `ipsec-no-pmtu-disc`) | config-phase checks; five read engine globals | extends: new IKE checks join this list; `ipsec-interface-address`, `ipsec-application-override`, `ipsec-spd-writers` move to `runtime` (AC-36); the two child 2 moves stay child 2's |
| AAA doctor checks | `aaa/doctor.go` `aaa-no-local-fallback`; `radius/doctor.go` `radius-admin-unreachable`; `tacacs/doctor.go` `tacacs-servers` | config and probe checks | extends: the RADIUS probe distinguishes a secret mismatch; new runtime checks join each package |
| PKI doctor checks | `pki/doctor.go` `pki-ca-root`; `pki/doctor_config_certs.go` `pki-configured-certificates` (`doctor-pki-cert`) | store and config material | extends: `pki-configured-certificates` already declares `doctor-pki-cert`, so the re-coded raise is covered by it; a runtime check reads the loaded store and the ephemeral root |
| L2TP and PPPoE kernel checks | `l2tp/register_kernelcap_linux.go` and the PPPoE kernelcap check (`doctor-l2tp-unavailable`, `doctor-l2tp-unknown`, `doctor-l2tp-ppp-*`, `doctor-pppoe-*`) | kernel support probes, gated on config | does not apply: correct as local checks; kept |
| authradius server check | `l2tp/plugins/authradius/register.go` `l2tp-auth-radius-servers` | probes configured servers | extends: the no-server case and the runtime rows join the same plugin's `DoctorChecks` |
| Doctor runtime phase | `pkg/plugin/rpc/types.go` `DoctorPhaseRuntime`; `doctor/registry.go` `RunRuntimeChecks`; `doctor/cmd/show.go` `HandleDoctor` | runs runtime checks only in the daemon; the local route asks the daemon or warns `doctor-runtime-not-checked` | extends: every runtime check here registers with this phase |
| Fix field | `rpc.DoctorCheckDiagnostic.Fix`, `diagnostic.Diagnostic.Fix` | the fixing command alone | extends: every new diagnostic sets it |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `RaiseError`, `ClearWarning` | coded issues: active warnings and a bounded error ring | extends: each transient class raises its event here under its doctor code |
| Recent-events check | `doctor/checks_recent_events.go` `recentEventsCheck` | renders bus events; a code with registered metadata shows as itself | extends: each transient code gets metadata in `codes.go` and is declared by its owner's check, so the event row carries it and the gate counts it |
| Diagnostic code registry | `internal/core/diagnostic/codes.go`; `ze show diagnostic <code>` | explains each code | extends: every new code gets metadata; `doctor-ipsec-listen` gains a producer |
| Health registry | `internal/core/health/registry.go` `Register`, `RegisterWhen` | degraded rows, no code | extends: `l2tp` moves to `RegisterWhen`; each of `ipsec`, `pki`, `l2tp` is mapped to a doctor check carrying the same states with code and fix |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` | lists uncovered sites, refuses added ones | extends: this spec's sites move to covered; the report over the area is the deliverable check |
| L2TP snapshot and child 2's per-peer record | `l2tp/snapshot.go` `Snapshot`; child 2 phase 3 (per-peer unanswered record) | live tunnels and sessions | extends: the per-peer record child 2 adds gains the tunnel-auth, version and protocol-error causes; no second record |
| PPP, PPPoE and RADIUS metrics | `ze_ppp_auth_failures_total`, `ze_pppoe_discovery_refusals_total`, `ze_radius_acct_sent_total`, `subscriber.RecordAuthResult` | counters | extends: the runtime checks read the same counters; a counter is added only where none exists (named per AC) |
| IPsec inventory | `internal/core/ipsecinventory` `Tunnels` | per-peer `Up`, `IfID` | extends: the Child SA rows for peers outside child 2's VTI scope read it |
| Plugin supervision codes | `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a: `doctor-plugin-stopped`, `doctor-plugin-crashed`, `doctor-plugin-external-unsupported`, `doctor-internal-defect` | declared by child 12's checks | does not apply as a check here: the sites in this area carry those codes; plugin-infra's checks declare them |
| Operator error | `internal/core/opererr` `New`, `LogAttrs` | problem, why, fix; caller-supplied `code` | extends: a site logged through it passes its doctor code the same way |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show doctor` on every daemon surface; `ze doctor` locally (asks the daemon, or answers locally with `doctor-runtime-not-checked`)
- a failure in the owner (an IKE negotiation, an L2TP refusal, a RADIUS reply) raised on the report bus

### Transformation Path
1. Owner state: each owner keeps the state its check reads (existing state, or the record or counter the AC names)
2. A local-phase check reads the config tree, the store, the host or a probe; a runtime check reads its owner's live state
3. A transient class: the failure site logs with its doctor code and raises on the report bus under that code; the owner's runtime check declares the code and reports the owner's record when it has one; `recent-events` renders the bus event under the same code
4. `HandleDoctor` or the local route draws each diagnostic with its `fix` line

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | IKE and subscriber plugins answer `doctor-check` callbacks with `fix`; phase `runtime` in the Stage 1 declaration | No |
| Core leaves | non-IKE readers use `ipsecinventory` and `ikeprobe` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `test/`, `docs/`; terms `cert-expiry`, `cert-expired`, `reportCodeExpiry`, and `"code",` on Warn and Error lines in `ike`, `l2tp`, `radius`, `tacacs`, `aaa`, `authz`, `pki`. Positive control: `pki/health.go` `raiseOrClearExpiry` writes both PKI codes; `ike/engine/fsm.go` logs the EAP packet code under `code`. Findings: no reader of `cert-expiry` or `cert-expired` outside `pki/health.go`; `code` holds a protocol code at `ike/engine/fsm.go` (EAP packet discarded, two sites), `l2tp/pppoeclient/session.go` and `l2tp/pppoeclient/ipcp_independent.go` (LCP code), `l2tp/plugins/authradius/handler.go` and `l2tp/plugins/authradius/coa.go` (RADIUS code), `radius/authenticator_eap.go` (RADIUS code), `l2tp/ppp/ncp.go` and `l2tp/ppp/session_run.go` (LCP code)

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| report bus code for PKI expiry | writer | `pki/health.go` `raiseOrClearExpiry` | raises `cert-expiry`, `cert-expired` | change to `doctor-pki-cert`; the clear calls use the same code |
| report bus code for PKI expiry | reader | `show warnings`, `show errors`, the `recent-events` check, API clients of those commands outside the repository | grep finds no in-tree reader of the old strings | the new code is a registered doctor code; an outside reader keyed on `cert-expiry` sees the new code (pre-release, no compatibility owed) |
| log attribute `code` | writer | the protocol-code sites in Search evidence | `code` holds an EAP, LCP or RADIUS code | rename the key to `eap-code`, `lcp-code` or `radius-code`; Debug and Info sites in the same package follow, for one spelling per package |
| log attribute `code` | reader | the doctor-coverage gate (`internal/le/arch/doctorcoverage`) reading `code` as a doctor code; operators' log searches | the gate reads a protocol code as code-dynamic (parent Data Flow 4b) | after the rename the gate sees the doctor code alone; the logging page names the new keys |
| `show doctor` JSON | writer and reader | unchanged format; new codes only | parent census | preserve |

### Integration Points
- `ike/engine/register.go` `DoctorChecks` - new and moved IKE checks
- each component's doctor file and `diagnostic.RegisterDoctorCheck` - new AAA, RADIUS, TACACS+, authz and PKI checks
- the L2TP and PPPoE subsystems register runtime checks with `diagnostic.RegisterDoctorCheck` from their own package (they run in the daemon, parent A-3)
- the subscriber plugins add to their `registry.Registration.DoctorChecks`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every row reaches `show doctor` through the runtime runner or the plugin bridge |
| No unintended coupling (components stay isolated) | Yes | each check lives in the owner package; IKE readers outside the engine use the core leaves |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; the L2TP per-peer record is child 2's, extended |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path; records keep the last failure only |
| Registration over hardcoding, outbound | Yes | checks register from the owner's `init()` or plugin registration |
| Registration over hardcoding, inbound | Yes | the gate and doctor derive codes from the registries; no list in `doctor` names an area check |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The IKE engine plugin runs in the daemon process (DirectBridge), so its `runtime` checks read live engine globals | parent A-3; `ai/doors/plugin-infra.md` Traps | a forked engine answers through the plugin callback, which reads the forked process's own state; still live | a runtime check unit test against engine state, plus `test/plugin/show-doctor-ipsec-auth-failed.ci` | unvalidated |
| A-2 | The L2TP and PPPoE subsystems run in the daemon, so a registry check in their package reads live state | `Engine.RegisterSubsystem` (`ai/doors/hub-engine.md`) | a check reads an empty copy | `test/plugin/show-doctor-l2tp-tunnel-auth.ci` | unvalidated |
| A-3 | An IKE per-peer last-failure record (cause, peer, time, message) is enough for every transient IKE class here | slice A1 rows share one shape | some class needs more (an SPI, a proposal) | `TestIPsecPeerLastFailureRecord` over each cause | unvalidated |
| A-4 | Every fix named in an AC exists as a command or YANG path | each AC names it by kind | a fix names nothing | each check's unit test resolves its fix against the command registry or YANG (parent AC-7) | unvalidated |
| A-5 | The subscriber plugins run internal; `authradius` refuses external | `l2tp/plugins/authradius/register.go` | a forked `authlocal`, `pool` or `shaper` reads its own state | plugin `DoctorChecks` answer through the callback either way | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Transient rows flood `show doctor` on a box with a flapping peer | many rows per peer | one record per peer (last failure) and the bus's per (source, code, subject) grouping |
| R-2 | `doctor-ipsec-spd-left` reads Ze's live SPD as leftover | a row on a running daemon | the check reports only when no IKE engine runs in the process answering doctor; on the daemon route the engine runs |
| R-3 | A check reports a normal setup phase (tunnel coming up, pool filling) | rows during boot | thresholds come from protocol timers or the configured size, never a new constant |
| R-4 | A row prints a secret (L2TP shared secret, RADIUS secret, TACACS+ key, PSK) | review | security checklist; every message names the peer and the leaf, never the value |
| R-5 | Renaming `code` log keys breaks an operator's log search | log query by `code` | the logging page names the new keys in the same commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a false or missing doctor row; a renamed log key; no forwarding or session change |
| How is it reverted? | one commit per phase |
| Who else touches this path? | child 2 (IKE check list, L2TP per-peer record, PPPoE client), `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0a plugin codes, AC-35 env code), `plan/release/validation/spec-doctor-coverage-5-appliance.md` (DNS source code) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with an IKE peer whose PSK is wrong | → | IKE runtime check reading the per-peer last failure | `test/plugin/show-doctor-ipsec-auth-failed.ci` |
| `show doctor` while `vpn ipsec interface` has no address | → | `ipsec-interface-address` in the `runtime` phase | `test/plugin/show-doctor-ipsec-interface-address.ci` |
| `ze doctor --json`, no daemon, same config | → | the local route skipping it and warning `doctor-runtime-not-checked` | `TestIPsecEngineStateChecksAreRuntimePhase` |
| `ze doctor --json config` with an EAP-TLS peer citing a missing certificate | → | IKE local check `doctor-ipsec-eap-tls-credential` | `test/ui/doctor-ipsec-config.ci` |
| `show doctor` with an AAA backend dropped at boot | → | AAA runtime check | `test/plugin/show-doctor-aaa-backend-dropped.ci` |
| `show doctor` with a certificate expiring in 10 days | → | PKI raise under `doctor-pki-cert`, recent-events row | `test/plugin/show-doctor-pki-cert-expiry.ci` |
| `show doctor` with an L2TP peer using a wrong shared secret | → | L2TP runtime check over the per-peer record | `test/plugin/show-doctor-l2tp-tunnel-auth.ci` |
| `show doctor` with an exhausted subscriber pool | → | pool plugin runtime check | `test/plugin/show-doctor-subscriber-pool.ci` |
| `show health` on a `ze_l2tp` build with no L2TP config | → | `l2tp` health row not applying | `TestL2TPHealthRowOnlyWhenSubsystemExists` |

## Acceptance Criteria

Expected Behavior names what doctor reports: code, severity, the state it
reads, route (local or runtime) and fix. The fix names a command or config
path, which the check's unit test resolves (A-4). A transient class also
raises on the report bus under the same code at the failure site, and the
owner's runtime check declares the code.

**IKE / IPsec (`internal/component/ike/engine`, `internal/component/ike/dataplane`)**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An IKE UDP 500 listener for a configured address failed to bind | runtime check reads the engine's listener set and reports `doctor-ipsec-listen` (error) per unbound address with the bind error; fix names the `local-address` leaf; the code's existing metadata gains this producer | an address the config does not name is not checked |
| AC-2 | The kernel address-table watch is unavailable or ended (`watchAddresses`) | runtime `doctor-ipsec-address-watch` (warning) naming the cause and that address changes are followed only through the interface monitor; fix names the reload command | none |
| AC-3 | `dataplane.Load` failed in `runEngine` | the engine keeps the load error; runtime `doctor-ipsec-dataplane-load` (error): SA installation disabled for every peer, with the error; fix names the dataplane setting or the kernel support check | none |
| AC-4 | The IKE engine plugin exited with an error | the exit site in `register.go` carries `doctor-plugin-crashed`, reported by plugin-infra's check | the check itself is plugin-infra's |
| AC-5 | IKE_SA_INIT negotiation fails (no common proposal, KE group, offer rejected, either role) | the failure sets the per-peer last failure (cause negotiation) and raises `doctor-ipsec-negotiation-failed`; runtime check reports peer, the side that refused and the proposal (warning while the peer has no IKE SA); fix names the `ike-group` proposal leaf | cleared when an IKE SA to that peer establishes |
| AC-6 | IKE authentication fails (AUTH verify, AUTHENTICATION_FAILED received, chain refused, wrong identity) | per-peer last failure (cause auth) and `doctor-ipsec-auth-failed` (error while no SA): peer and the check that failed; fix names the PSK, certificate or `remote-id` leaf | cleared on establishment |
| AC-7 | EAP-TLS client credential unusable: certificate missing from the PKI store, no private key, no ca-certificate | local check reads config `vpn ipsec` EAP against `pki`: `doctor-ipsec-eap-tls-credential` (error) naming peer and the missing item; fix names the `pki certificate` or `ca-certificate` leaf | none |
| AC-8 | EAP exchange fails (method failure, EAP to a non-EAP peer, no EAP session, MSK AUTH failed) | per-peer last failure (cause eap) and `doctor-ipsec-eap-failed` (error while no SA); fix names the EAP method or credential leaf | cleared on establishment |
| AC-9 | Peer sends malformed or protocol-violating IKE messages (the 19 sites of the inventory row) | per-peer last failure (cause protocol) and `doctor-ipsec-peer-protocol-error` (warning): peer, message, violation; fix names `show vpn ipsec sa` | none |
| AC-10 | NAT traversal prohibited for the peer, or the peer rejects NAT on the auth path | per-peer last failure (cause nat) and `doctor-ipsec-nat-refused` (error while no SA); fix names the NAT-T leaf | none |
| AC-11 | IKE SA up, Child SA refused or failed (TS_UNACCEPTABLE, overlap, no UDP encap port, transport mode declined, responder TS missing) for a peer that is not VTI-bound | runtime check reads `ipsecinventory.Tunnels` and the per-peer last failure (cause child): `doctor-ipsec-child-sa-refused` (error): peer, reason; fix names the traffic-selector leaf | a VTI-bound peer (IfID non-zero) is child 2's `doctor-ipsec-xfrm-peer-no-child-sa` |
| AC-12 | A Child SA installed degraded: the dataplane refused its ESP state (`warnDegraded`, `ESPInstalled` false) | runtime `doctor-ipsec-child-sa-degraded` (error) per Child SA: peer, SPI pair, refusal; fix names the clear command for that peer's SA | none |
| AC-13 | A tunnel policy could not be removed or restored; its selectors stay reserved | the engine keeps the leftover list (`releaseInnerRecord`, inner guard registry); runtime `doctor-ipsec-policy-leftover` (error): selectors and the peers they block; fix names the clear or reload command | none |
| AC-14 | IKE SA lost to liveness or lifetime (DPD dead, unanswered request, rekey unanswered, message-id exhausted, hard lifetime, handshake timeout) | per-peer last teardown reason and `doctor-ipsec-sa-lost` (warning): peer, reason, time; fix names `show vpn ipsec sa` and the DPD or lifetime leaf | cleared on re-establishment |
| AC-15 | Rekey refused by the peer or by Ze (IKE or Child) | per-peer last failure (cause rekey) and `doctor-ipsec-rekey-failed` (warning); fix names the proposal leaf | the current SA stays; cleared on a successful rekey |
| AC-16 | An SA has no send path (`sendPath` nil): DPD, keepalive and requests dropped | runtime `doctor-ipsec-no-send-path` (error) per SA; fix names the listener or interface to bring up | none |
| AC-17 | A Delete was not sent because the request window was busy | bus event `doctor-ipsec-delete-not-sent` (warning) naming peer and SPI; declared by the IKE runtime check | none |
| AC-18 | MOBIKE update failed, SA closed or Child SA lost while migrating | per-peer last teardown reason (cause mobike) and `doctor-ipsec-mobike-failed` (warning); fix names `show vpn ipsec sa` | cleared on re-establishment |
| AC-19 | Authenticator certificate revoked after the tunnel came up (SA closed), or its revocation status not re-checked | runtime check reads the per-SA post-auth revocation status: `doctor-ipsec-cert-revoked` (error) or `doctor-ipsec-revocation-unchecked` (warning); fix names the `pki ca` CRL leaf | none |
| AC-20 | A certificate-auth peer whose chain holds no `pki ca` with a CRL | local `doctor-ipsec-no-crl` (warning) naming peer and CA; fix names the `pki ca` CRL leaf | none |
| AC-21 | A certificate-auth peer with no `remote-id` | local `doctor-ipsec-cert-no-remote-id` (warning): any certificate from that CA authenticates as this peer; fix names the `remote-id` leaf | none |
| AC-22 | An EAP method that derives no MSK is configured (RFC 7296 Section 2.16 SHOULD NOT) | local `doctor-ipsec-eap-no-msk` (warning) naming peer and method; fix names the method leaf | none |
| AC-23 | The peer lacks RFC 4739 multiple auth; the second-round credential is not sent | runtime `doctor-ipsec-multiple-auth-unsupported` (warning) per SA; fix names the second-auth leaf | none |
| AC-24 | A remote-access virtual IP pool named by a peer is not built from config | local check builds the pools from config as the engine does (`eap.NewPool`): `doctor-ipsec-virtual-ip-pool-missing` (error) naming pool and peers; fix names the pool leaf | none |
| AC-25 | A respond peer's `remote-address` name stops resolving; old addresses kept | runtime `doctor-ipsec-respond-name-unresolved` (warning): peer, name, addresses still used; fix names the name leaf or the resolver | none |
| AC-26 | The IKE bypass policy is not installed | runtime check compares the engine's bypass set with the kernel SPD: `doctor-ipsec-bypass-missing` (error); fix names the reload command | none |
| AC-27 | An operator `vpn ipsec policy` entry is not installed | runtime compares the applied SPD policy set with the kernel SPD: `doctor-ipsec-policy-not-installed` (error) naming the entry; fix names the reload command | none |
| AC-28 | The dataplane has no unbound catch-all (VPP) or no per-interface SPD, so `unmatched bypass` or the interface catch-all is not installed | runtime reads the dataplane capability (`catchAllUnsupported`) with the applied config: `doctor-ipsec-catch-all-unsupported` (error); fix names the dataplane or `unmatched` leaf | the implicit discard cutting own traffic is child 2's |
| AC-29 | A stale XFRM interface SPD entry could not be removed | the engine keeps the removal-failed entries; runtime `doctor-ipsec-xfrm-entry-stale` (error); fix names the reload command | missing exemptions are child 2's |
| AC-30 | Ze SPD entries and the catch-all left in the kernel with no IKE engine running | local check reads the kernel XFRM policy table for Ze's entries when no engine runs in the answering process: `doctor-ipsec-spd-left` (error); fix names the command that removes them | R-2: a running engine's entries are not leftovers |
| AC-31 | The tunnel-source guard is incomplete (no free table; rule, route or nft table not installed or not removed) | runtime compares the guard's held state with the desired one: `doctor-ipsec-source-guard` (error) naming the missing piece; fix names the reload command | none |
| AC-32 | An XFRM interface address is also a peer's `local-address`, so no tunnel-source guard is built for it | local `doctor-ipsec-source-guard-skipped` (warning) naming address and peer; fix names the `local-address` leaf | none |
| AC-33 | Inbound overload: owner queue, transport queue or PMTU refusal queue full | drop counters added per queue; runtime `doctor-ipsec-inbound-dropped` (warning) with counts since start; fix names `show vpn ipsec` | none |
| AC-34 | An ESP-form receiver stopped reading | the receiver keeps its run state; runtime `doctor-ipsec-espform-receiver` (error); fix names the reload command | none |
| AC-35 | Health row `ipsec` degraded or down | a runtime check reports the same states with code and fix: `doctor-ipsec-peer-not-established` (warning, per peer), `doctor-ipsec-dataplane-drift` (error), `doctor-ipsec-dataplane-unknown` (warning); "engine not running" is AC-4's code | the health row stays as it is |
| AC-36 | AC-9 rows: `ipsec-interface-address`, `ipsec-application-override`, `ipsec-spd-writers` | each registers at `rpc.DoctorPhaseRuntime`; on the local route with no daemon they do not run and `doctor-runtime-not-checked` appears; through `show doctor` they report as today | `ipsec-udp-encap` and `ipsec-virtual-ip-pool` move in child 2 |
| AC-37 | Every Warn and Error line at the source sites of AC-1 to AC-35, and of the covered rows (`doctor-ipsec-iface`, `doctor-ipsec-cookie-threshold`, `doctor-ipsec-cert-url`) | each carries its doctor code as the `code` attribute; the gate reports them covered | none |
| AC-38 | Not problem classes (IKE): 23 crypto or build failures, 9 event-bus emit failures, 2 unreachable group names, 4 transport close errors, 2 per-message build paths; 5 single-response send failures; the "accepted a longer key" line | the first five groups carry `doctor-internal-defect`; the send failures carry `doctor-ipsec-no-send-path`; the longer-key line becomes Info | the merge's assignment (`plan/audits/doctor-problem-inventory.md`) |

**AAA, RADIUS, TACACS+, authz, PKI**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-39 | An AAA backend dropped from the chain at boot (`Build` error) | the hub keeps the dropped list of the running bundle; runtime `doctor-aaa-backend-dropped` (error): backend, error, its users cannot authenticate; fix names the backend's config leaf | a reload that refuses keeps the running chain and its list |
| AC-40 | RADIUS admin answers fail verification (bad Response Authenticator, undecodable answer) | the probe verdict tells it apart: local `doctor-radius-admin-secret-mismatch` (error): the server answers but its secret differs; fix names the server's secret leaf | an unanswered server stays `doctor-radius-admin-unreachable` |
| AC-41 | RADIUS Access-Accept unusable for admin login (unsupported Service-Type, no profile, reserved profile) | bus event `doctor-radius-admin-accept-unusable` (warning): user and reason; declared by the RADIUS runtime check; fix names the profile leaf | none |
| AC-42 | RADIUS admin EAP login ends with no response to the challenge | bus event `doctor-radius-admin-eap-failed` (warning); fix names the EAP method leaf | none |
| AC-43 | TACACS+ accounting records failed, rejected or dropped after stop | runtime reads the accountant's drops and last failure: `doctor-tacacs-accounting-failed` (warning); fix names the server leaf | none |
| AC-44 | TACACS+ returns a privilege level with no `tacacs-profile` mapping | bus event `doctor-tacacs-privilege-unmapped` (warning): user and level; fix names the profile mapping leaf | none |
| AC-45 | TACACS+ authorization returns an unsupported status (FOLLOW and others) | bus event `doctor-tacacs-authorization-unsupported` (warning); fix names the server-side setting | none |
| AC-46 | An authenticated user has no applicable authorization profile | bus event `doctor-authz-no-profile` (warning): user; declared by an authz runtime check; fix names the `system authorization` assignment | none |
| AC-47 | The daemon runs on an ephemeral local CA | `NewEphemeralRoot` marks the root ephemeral; runtime `doctor-pki-ca-ephemeral` (warning): the authority changes on restart; fix names the start command with a store | none |
| AC-48 | A PKI certificate or CA expired or near expiry, and the `pki` health row | the raise and clear use `doctor-pki-cert`; a runtime check over the loaded store reports the health row's states under `doctor-pki-cert`, fix naming the certificate leaf to renew | `pki-configured-certificates` keeps the config side |
| AC-49 | Every Warn and Error line at the source sites of AC-39 to AC-48 and of the covered rows (`doctor-radius-admin-unreachable`, `doctor-radius-admin-unsigned-response`, `doctor-tacacs-unreachable`) | each carries its code | none |
| AC-50 | Not problem classes (authz): empty identity and all profiles undefined | each line carries `doctor-internal-defect` | unreachable from config |

**L2TP, PPP, PPPoE and subscriber plugins (`internal/component/l2tp`)**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-51 | The kernel data plane is not programming sessions (worker unavailable, or `ze.l2tp.disable-kernel-dataplane=true`) | runtime reads `kernelDataPlane()`: `doctor-l2tp-dataplane` (error) with the cause; fix names the env setting or the kernel support check; maps the health row's degraded state | none |
| AC-52 | `ze.l2tp.skip-kernel-probe=true` in the daemon's environment | runtime `doctor-l2tp-kernel-probe-skipped` (warning); fix names unsetting the env value | none |
| AC-53 | L2TP enabled with no listener (never started) | local check over config and runtime check over the published service: `doctor-l2tp-no-listener` (error); fix names the listener leaf | none |
| AC-54 | Invalid `ze.l2tp.cqm.echo-interval`, fell back to 1s | the read is recorded by core `env` and reported as runtime `doctor-env-invalid-value` (warning) naming key, value, type and the default used, fix the corrected export; that class and its check are `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` AC-35 (phase P6), so this spec registers no L2TP env code and only proves the L2TP key reaches it | none |
| AC-55 | Committed config not in effect until restart (enabled, listener, cqm-enabled, max-logins, ring size, sample retention) | runtime compares the running parameters with the committed values the subsystem last received: `doctor-l2tp-restart-required` (warning) listing the leaves; fix names the daemon restart command | none |
| AC-56 | Dial refused: the listener cannot reach the remote (loopback listener, or listener moved by reload) | local check over listener and dial remotes, runtime check over the configured listen address against the bound socket: `doctor-l2tp-dial-unreachable` (error); fix names the listener leaf | none |
| AC-57 | max-tunnels reached or the tunnel ID space exhausted | a refusal counter per cause; runtime `doctor-l2tp-tunnel-limit` (warning at the limit, error with refusals since start); fix names the `max-tunnels` leaf | below the limit with no refusal, no row |
| AC-58 | max-sessions per tunnel reached or session ID space exhausted | refusal counter; runtime `doctor-l2tp-session-limit`; fix names the `max-sessions` leaf | as AC-57 |
| AC-59 | Tunnel authentication fails (secret unset on one side or mismatched, StopCCN result 4) | child 2's per-peer record gains cause tunnel-auth; runtime `doctor-l2tp-tunnel-auth-failed` (error while no tunnel to the peer): peer; fix names the `shared-secret` leaf | cleared on establishment |
| AC-60 | Peer speaks L2TPv3; every datagram dropped | per-peer record cause version; `doctor-l2tp-version-mismatch` (error); fix names the peer's configuration | none |
| AC-61 | Peer sends malformed, out-of-sequence or unknown-mandatory control messages | per-peer record cause protocol and bus event `doctor-l2tp-peer-protocol-error` (warning): peer, message, result code; fix names the show command for L2TP tunnels | none |
| AC-62 | Outgoing call refused: the peer advertises no matching bearer or framing | runtime reads the peer's bearer and framing capabilities against the requested call: `doctor-l2tp-call-capability` (error); fix names the call's bearer or framing leaf | none |
| AC-63 | Kernel tunnel, session or PPP channel programming failed; session torn down | failure counter and bus event `doctor-l2tp-kernel-program-failed` (error) with the netlink error; fix names the kernel support check | none |
| AC-64 | Kernel L2TP state leaked (rollback or delete failed, or a contained panic) | runtime compares the kernel genl tunnel and session dump with `Snapshot`: `doctor-l2tp-kernel-state-leaked` (error); fix names the command that clears it | none |
| AC-65 | No PPP driver: no interface backend, so L2TP sessions get no PPP and PPPoE refuses PADRs not bound to a relay | runtime reads the PPP driver in the L2TP and PPPoE subsystems: `doctor-l2tp-no-ppp-driver` (error); fix names `set interface backend netlink` | the config-only hint is child 2's `doctor-iface-no-backend` |
| AC-66 | No subscriber auth handler or no pool handler registered | runtime reads `subscriber.GetAuthHandler()` and `GetPoolHandler()` while L2TP or PPPoE runs: `doctor-subscriber-no-auth-handler`, `doctor-subscriber-no-pool-handler` (error); fix names the auth or pool config | none |
| AC-67 | The configured auth backend is not the one answering (single slot) | each auth plugin registers with its name; runtime compares the answering handler's name with the auth config: `doctor-subscriber-auth-backend-mismatch` (error); fix names the reload command | the slot design is the door trap, unchanged |
| AC-68 | Listener UDP send fails (control, CDN, StopCCN, HELLO) | send-error counter on the listener; runtime `doctor-l2tp-send-failed` (warning) with count and last error; fix names the listener leaf | none |
| AC-69 | A realm relay remote has no `shared-secret`, so PAP proxy authentication is not sent | local `doctor-l2tp-relay-no-secret` (warning); fix names the relay `shared-secret` leaf | none |
| AC-70 | Realm relay refused, failed, or left a relay session after the subscriber left | bus event `doctor-l2tp-relay-failed` (warning) and a runtime comparison of relay sink sessions with live sessions; fix names the show command for L2TP sessions | none |
| AC-71 | Subscriber consumers out of sync after an event delivery failure (pool lease, injected route, shaper, accounting or DNS held for a gone session, or missing for a live one) | runtime compares pool leases, redistribute routes and the dnssource registry with the L2TP and PPPoE session snapshots: `doctor-subscriber-consumer-stale` (error) naming consumer and session; fix names the clear command for the session | none |
| AC-72 | PPP authentication method refused by policy | runtime reads the auth-method refusal counter: `doctor-ppp-auth-method-refused` (warning) with the method offered; fix names `auth allow` | none |
| AC-73 | Subscribers failing PPP establishment (LCP or NCP timeout, auth reject, NCP refusal) | runtime reads `RecordAuthResult` and session-down causes: `doctor-ppp-establish-failed` (warning) by cause with counts; fix names the show command for L2TP sessions | none |
| AC-74 | IPv6 refused: peer MRU below 1280, IPv6CP disabled for the session | refusal counter; runtime `doctor-ppp-ipv6-mru` (warning); fix names the MRU leaf | none |
| AC-75 | Both IPCP and IPv6CP disabled in config | local `doctor-ppp-no-ncp` (error): sessions carry no IP; fix names the NCP enable leaf | none |
| AC-76 | IPv6CP opened with no peer interface identifier | runtime `doctor-ppp-ipv6-no-interface-id` (warning) per session | none |
| AC-77 | IPv6 service (RA, DHCPv6) failed to start on a session | per-session service state; runtime `doctor-ppp-ipv6-service-failed` (error); fix names the show command for L2TP sessions | none |
| AC-78 | PPPoE enabled and an access interface not started | local check over config interfaces and the link table, runtime over the subsystem's servers: `doctor-pppoe-interface-not-started` (error); fix names the interface leaf | none |
| AC-79 | PPPoE subscriber refused at discovery (session ID exhausted, add failed, PADO too large) | runtime reads `ze_pppoe_discovery_refusals_total` by reason: `doctor-pppoe-discovery-refused` (warning); fix by reason | none |
| AC-80 | PPPoE session socket or PPP channel setup failed at run time | failure counter and bus event `doctor-pppoe-session-setup-failed` (error); fix names the kernel support check | none |
| AC-81 | An IPv4 address pool exhausted | runtime per named pool: `doctor-subscriber-pool-exhausted` (error) with size; fix names the pool range leaf | none |
| AC-82 | An IPv6 PD pool exhausted, or IPv6 requested with no PD pool | runtime `doctor-subscriber-pd-pool-exhausted` (error); fix names the `ipv6-pd` leaf | none |
| AC-83 | RADIUS Framed-Pool names a pool not configured | bus event `doctor-subscriber-pool-unknown` (error): pool name and user; the silent reject logs too; fix names the pool leaf | none |
| AC-84 | Subscriber DNS from RADIUS or a host-only resolver not offered | the pool site carries `doctor-dns-source-host-only-dropped` (declared by `spec-doctor-coverage-5-appliance.md`'s dns-source check) for the host-only case; runtime `doctor-subscriber-dns-dropped` (warning) for the RADIUS case | the empty selection stays `doctor-dns-source-empty` |
| AC-85 | Shaper configured, no traffic backend loaded | runtime reads `traffic.GetBackend()`: `doctor-shaper-no-backend` (error); fix names the traffic backend leaf | none |
| AC-86 | TC apply, update or restore failed for a session | runtime compares the shaper's sessions with live sessions: `doctor-shaper-apply-failed` (error); fix names the reload command | none |
| AC-87 | RADIUS subscriber auth with no usable client (no server) | local `doctor-radius-no-server` (error): every subscriber rejected; fix names the server leaf | none |
| AC-88 | Access-Accept the NAS cannot honour (Service-Type, Framed-Protocol, missing MS-CHAP2-Success, no credential attribute, unexpected code) | bus event `doctor-radius-accept-unusable` (warning) per reply class; fix names the server-side attribute | none |
| AC-89 | RADIUS accounting not delivered (start, interim, stop) | a failure counter beside `ze_radius_acct_sent_total`; runtime `doctor-radius-accounting-failed` (warning); fix names the accounting server leaf | none |
| AC-90 | Accounting counters unavailable for a session | runtime reads the per-session invalid flag: `doctor-radius-accounting-counters` (warning) per session | none |
| AC-91 | CoA/Disconnect listener not running while RADIUS is configured | runtime reads the active CoA listener: `doctor-radius-coa-listener` (error) with the cause; fix names the CoA address leaf | none |
| AC-92 | CoA/Disconnect accepted and not carried out | bus event `doctor-radius-coa-failed` (error) | none |
| AC-93 | CoA client sends undecodable or unexpected requests, or responses not delivered | bus event `doctor-radius-coa-request-invalid` (warning) naming the client | none |
| AC-94 | `l2tp-auth-radius` configured external | the refusal site carries `doctor-plugin-external-unsupported` (plugin-infra) | the check is plugin-infra's |
| AC-95 | Local auth with no users | local `doctor-subscriber-local-no-users` (error); fix names the user leaf | none |
| AC-96 | MS-CHAPv2 allowed while local auth answers | local `doctor-subscriber-local-mschapv2` (warning); fix names `auth allow` | none |
| AC-97 | A subscriber plugin exited (authlocal, authradius, pool, shaper) | each exit site carries `doctor-plugin-crashed` (plugin-infra) | the check is plugin-infra's |
| AC-98 | `show health` on a box where the L2TP subsystem was not constructed | the `l2tp` row is registered with `health.RegisterWhen` and applies only while the subsystem exists, so no row appears; with the subsystem the row reads as today | fixes the journal row in Owned journal rows |
| AC-99 | Every Warn and Error line at the source sites of AC-51 to AC-97 and of the covered rows (kernelcap codes, `l2tp-auth-radius-servers` codes) | each carries its code | none |
| AC-100 | Not problem classes (L2TP): 6 BUG paths, 3 host entropy, 14 replies to a gone session, 8 close-at-teardown lines; 12 send-queue enqueue failures; 7 per-packet lines; 3 self-recovering negotiations; the `ipv6-pd delegation-length` default | the first four groups carry `doctor-internal-defect`; the enqueue and per-packet lines carry the peer classes' codes (child 2 `doctor-l2tp-peer-unresponsive`, AC-61); the self-recovering lines and the default become Info | the merge's assignment |

**Shared**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-101 | Every code this spec adds | `ze show diagnostic <code>` explains it (metadata in `codes.go`); the declaring check lists it in `Codes` | none |
| AC-102 | A protocol code logged under `code` on a Warn or Error line in the area | the key is `eap-code`, `lcp-code` or `radius-code`; `code` holds only a doctor code | none |
| AC-103 | `./le arch doctor-coverage report` after the last phase | the area's packages show every site covered; the three health rows are mapped (AC-35, AC-48, AC-51) | sites owned by child 2 are counted under child 2 |
| AC-104 | Every check added | its unit test asserts a non-empty fix naming a `ze` command or a `set`/`delete` path, and goes red when the detection is removed | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` after an IPsec peer stopped coming up | failure site -> per-peer record and bus -> runtime check -> `HandleDoctor` | `test/plugin/show-doctor-ipsec-auth-failed.ci` |
| 2 | runs `ze doctor` before committing an EAP-TLS peer | `doctor.Answer` -> IKE local check -> config | `test/ui/doctor-ipsec-config.ci` |
| 3 | runs `show doctor` on a BNG whose subscribers get no address | pool runtime check | `test/plugin/show-doctor-subscriber-pool.ci` |
| 4 | runs `show doctor` after an LAC with the wrong secret | L2TP per-peer record -> runtime check | `test/plugin/show-doctor-l2tp-tunnel-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPsecEngineStateChecksAreRuntimePhase` | `internal/component/ike/engine/doctor_runtime_test.go` | AC-36 | |
| `TestIPsecPeerLastFailureRecord` | `internal/component/ike/engine/peer_failure_test.go` | AC-5, AC-6, AC-8 to AC-10, AC-14, AC-15, AC-18: set, read, clear on establishment | |
| `TestIPsecRuntimeChecks` | `internal/component/ike/engine/doctor_runtime_test.go` | AC-1 to AC-3, AC-11 to AC-13, AC-16, AC-17, AC-19, AC-23, AC-25 to AC-29, AC-31, AC-33 to AC-35, one subtest per code | |
| `TestIPsecConfigChecks` | `internal/component/ike/engine/doctor_config_test.go` | AC-7, AC-20 to AC-22, AC-24, AC-30, AC-32 | |
| `TestAAABackendDroppedCheck` | `internal/component/aaa/doctor_test.go` | AC-39 | |
| `TestRadiusAdminProbeSecretMismatch`, `TestRadiusAdminTransientCodes` | `internal/component/radius/doctor_test.go` | AC-40 to AC-42 | |
| `TestTACACSRuntimeChecks` | `internal/component/tacacs/doctor_test.go` | AC-43 to AC-45 | |
| `TestAuthzNoProfileCode` | `internal/component/authz/doctor_test.go` | AC-46 | |
| `TestPKIEphemeralRootCheck`, `TestPKIExpiryRaisesDoctorCode` | `internal/component/pki/doctor_test.go` | AC-47, AC-48 | |
| `TestL2TPRuntimeChecks`, `TestRelayAndConsumerChecks` | `internal/component/l2tp/doctor_test.go` | AC-51 to AC-71 | |
| `TestL2TPHealthRowOnlyWhenSubsystemExists` | `internal/component/l2tp/health_test.go` | AC-98 | |
| `TestPPPRuntimeChecks` | `internal/component/l2tp/ppp/doctor_test.go` | AC-72 to AC-77 | |
| `TestPPPoERuntimeChecks` | `internal/component/l2tp/pppoe/doctor_test.go` | AC-78 to AC-80 | |
| `TestAuthRadiusChecks` | `internal/component/l2tp/plugins/authradius/doctor_test.go` | AC-87 to AC-93 | |
| `TestAuthLocalChecks` | `internal/component/l2tp/plugins/authlocal/doctor_test.go` | AC-95, AC-96 | |
| `TestPoolChecks` | `internal/component/l2tp/plugins/pool/doctor_test.go` | AC-81 to AC-84 | |
| `TestShaperChecks` | `internal/component/l2tp/plugins/shaper/doctor_test.go` | AC-85, AC-86 | |
| `TestAccessSecurityCodesHaveMetadata` | `internal/core/diagnostic/codes_test.go` | AC-101 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pool use (AC-81) | 0 to pool size | one address free: no row | N/A | none free: row |
| peer MRU (AC-74) | floor 1280 | 1280: no row | 1279: row | N/A |
| tunnel count (AC-57) | 0 to max-tunnels | max-tunnels minus one: no row | N/A | max-tunnels: row |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-ipsec-auth-failed` | `test/plugin/show-doctor-ipsec-auth-failed.ci` | two Ze IKE peers with different PSKs; `show doctor` names the peer and the PSK leaf | |
| `show-doctor-ipsec-interface-address` | `test/plugin/show-doctor-ipsec-interface-address.ci` | a peer waiting on an interface address, reported through the daemon | |
| `doctor-ipsec-config` | `test/ui/doctor-ipsec-config.ci` | the local config checks AC-7, AC-20 to AC-22 | |
| `show-doctor-aaa-backend-dropped` | `test/plugin/show-doctor-aaa-backend-dropped.ci` | a backend that fails to build at boot | |
| `show-doctor-pki-cert-expiry` | `test/plugin/show-doctor-pki-cert-expiry.ci` | a certificate expiring soon, row under `doctor-pki-cert` | |
| `show-doctor-l2tp-tunnel-auth` | `test/plugin/show-doctor-l2tp-tunnel-auth.ci` | an LAC with a wrong secret | |
| `show-doctor-subscriber-pool` | `test/plugin/show-doctor-subscriber-pool.ci` | an exhausted pool | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire-visible change: doctor reads state, and the failure paths keep their wire behavior (`ai/rules/interop-and-goal-validation.md`, reporting with no protocol change) | |

## Files to Modify
- `internal/core/diagnostic/codes.go` - every new code and its metadata
- `internal/component/ike/engine/register.go` - new checks, three phase moves, exit site code
- `internal/component/ike/engine/doctor_interface_address.go`, `internal/component/ike/engine/doctor_application_override.go` - runtime phase
- `internal/component/ike/engine/` - record writes, bus raises and codes on lines in fsm.go, responder.go, responder_eap.go, established.go, inbound.go, mobike.go, rebind.go, delete.go, dpd.go, probe.go, child.go, create_child.go, transport_mode.go, held_successor.go, reauth_receive.go, notify_error.go, sa_init_retry.go, apply.go, dynamic_wan.go, bypass.go, spd_policy.go, unmatched.go, unmatched_xfrm_interface.go, spd_left.go, source_guard.go, respond_names.go, virtual_ip.go, multiple_auth.go, postauth_revocation.go, remote_cert_checks.go, cert_payload.go, eap_auth.go, health.go
- `internal/component/ike/transport/udp.go`, `internal/component/ike/transport/udp_linux.go` - queue drop counter, listener state
- `internal/component/ike/dataplane/espform_linux.go` - receiver run state
- `internal/component/aaa/types.go`, `internal/component/aaa/doctor.go`, `cmd/ze/hub/aaa_lifecycle.go` - dropped list kept and checked
- `internal/component/radius/` - client.go, authenticator.go, authenticator_eap.go, doctor.go
- `internal/component/tacacs/` - accounting.go, authenticator.go, authorizer.go, client.go, doctor.go
- `internal/component/authz/authz.go`
- `internal/component/pki/health.go`, `internal/component/pki/ca.go`, `internal/component/pki/doctor.go`
- `internal/component/l2tp/` - register.go, health.go, subsystem.go, subsystem_reload.go, reactor.go, reactor_dial.go, reactor_kernel.go, tunnel_fsm.go, tunnel_initiator.go, session_fsm.go, session_initiator.go, kernel_linux.go, panic_guard.go, teardown.go, relay_sink.go, drain.go, route_observer.go, subscriber_bridge.go
- `internal/component/l2tp/ppp/` - auth.go, ncp.go, session_run.go, ra_linux.go, dhcpv6_linux.go, manager.go, relay_handoff.go
- `internal/component/l2tp/pppoe/` - server.go, subsystem.go, subsystem_reload.go, realm_relay.go, drain.go
- `internal/component/l2tp/subscriber/handler_registry.go` - handler names
- `internal/component/l2tp/plugins/authradius/`, `internal/component/l2tp/plugins/authlocal/`, `internal/component/l2tp/plugins/pool/`, `internal/component/l2tp/plugins/shaper/` - checks, codes, counters
- `internal/component/l2tp/pppoeclient/session.go`, `internal/component/l2tp/pppoeclient/ipcp_independent.go` - `code` key rename only
- `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/ike/ipsec-10-cli-diag.md`, `docs/architecture/l2tp/l2tp-10-metrics.md`, `docs/guide/ipsec.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/guide/radius.md`, `docs/guide/tacacs.md`, `docs/guide/authentication.md`, `docs/guide/health-checks.md`

## Files to Create
- `internal/component/ike/engine/peer_failure.go`, `internal/component/ike/engine/peer_failure_test.go` - per-peer last failure record
- `internal/component/ike/engine/doctor_runtime.go`, `internal/component/ike/engine/doctor_runtime_test.go`
- `internal/component/ike/engine/doctor_config.go`, `internal/component/ike/engine/doctor_config_test.go`
- `internal/component/l2tp/doctor.go`, `internal/component/l2tp/doctor_test.go`
- `internal/component/l2tp/ppp/doctor.go`, `internal/component/l2tp/ppp/doctor_test.go`
- `internal/component/l2tp/pppoe/doctor.go`, `internal/component/l2tp/pppoe/doctor_test.go`
- `internal/component/authz/doctor.go`, `internal/component/authz/doctor_test.go`
- `internal/component/l2tp/plugins/authlocal/doctor_test.go`, `internal/component/l2tp/plugins/pool/doctor_test.go`, `internal/component/l2tp/plugins/shaper/doctor_test.go`
- `internal/core/diagnostic/codes_test.go`
- the seven `.ci` files in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new config or RPC |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | `show doctor` exists |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | the seven `.ci` files |
| Pipe completeness | N-A | `show doctor` unchanged |
| Env var registration | No | the env values read are registered already |
| Doctor check for runtime dependencies | Yes | this spec |
| Prometheus counters/metrics | Yes | new counters: IKE queue drops (AC-33), L2TP refusals and send errors (AC-57, AC-58, AC-68), L2TP kernel programming failures (AC-63), PPPoE setup failures (AC-80), RADIUS accounting failures (AC-89); names set at implementation and listed in each owner's metrics page |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor rows diagnose existing features |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | `show doctor` format unchanged |
| 5 | Plugin added/changed? | Yes | subscriber plugins gain checks: `docs/guide/l2tp.md`, `docs/guide/radius.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/guide/radius.md`, `docs/guide/tacacs.md`, `docs/guide/authentication.md`, `docs/guide/health-checks.md` list the doctor codes |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | the parent spec did |
| 9 | RFC behavior implemented, changed, or newly proven? | No | reporting only |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (area checks, health row mapping); `docs/architecture/ike/ipsec-10-cli-diag.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/architecture/l2tp/l2tp-10-metrics.md` and the IKE and RADIUS metrics pages |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per phase with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-3-access-security.md`. Declared by the changed files' `// Design:` headers: `docs/architecture/ike/ipsec-7-ikev2-engine.md` and `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` (the failure record and the EAP and NAT failure paths: updated), `docs/architecture/l2tp/subscriber-session-model.md` (handler names: updated), `docs/architecture/core-design.md` (the `l2tp` and `pki` health rows: updated), `docs/architecture/l2tp/cpe-1-pppoe-client.md` (only the log key rename: updated where it names the key), `ai/patterns/registration.md` (the AAA registry shape: unaffected, no registry changes shape), `docs/features/ai-first.md` (unaffected: no claim about doctor coverage changes), `plan/release/validation/spec-ipsec-esp-dual-form-receive.md` (the ESP-form receiver's design spec: unaffected, the receiver keeps its behavior and gains a run state only) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | sample `show doctor` output in the guide pages is refreshed where a row changes |

## Implementation Steps

1. **Phase: Wiring** -- codes and metadata, empty runtime checks per owner, the `.ci` files failing
2. **Phase: Shared mechanisms** -- IKE per-peer last failure, `code` key renames, PKI re-code, `l2tp` health row
3. **Phase: Per sub-area checks** -- the packages below
4. **Phase: Lines carry codes, and docs** -- each sub-area ends with its AC-37, AC-49 or AC-99 sweep and its page edits

### Implementation Phases

One agent per package, about 60 tool calls each. Agents edit only; the main thread runs every test.

| Phase | Owns | ACs | Depends on | Runs with |
|-------|------|-----|------------|-----------|
| P0 wiring | `codes.go` (every code up front), empty runtime checks registered in each owner, the seven `.ci` drafts | AC-101 | - | - |
| P1 IKE shared | the per-peer failure record; the three AC-9 moves; the `code` renames in IKE and RADIUS admin | AC-36, AC-102 (IKE, RADIUS admin) | P0 | ∥ P5, P7 |
| P2 IKE transients | the failure sites in the negotiation, auth, EAP, established, inbound, MOBIKE, rebind, delete and multiple-auth files | AC-5, AC-6, AC-8 to AC-10, AC-14, AC-15, AC-17, AC-18 | P1 | - |
| P3 IKE runtime state | apply, dynamic WAN, register, child, bypass, SPD policy, unmatched, source guard, respond names, revocation, transport, ESP form, health | AC-1 to AC-4, AC-11 to AC-13, AC-16, AC-19, AC-23, AC-25 to AC-29, AC-31, AC-33 to AC-35 | P1 | ∥ P4 |
| P4 IKE config and sweep | the IKE config checks; the IKE line sweep | AC-7, AC-20 to AC-22, AC-24, AC-30, AC-32, AC-37, AC-38 | P2 | ∥ P3 |
| P5 AAA, PKI | aaa, radius, tacacs, authz, pki, the hub AAA lifecycle | AC-39 to AC-50 | P0 | ∥ P1 |
| P6 L2TP control plane | the L2TP subsystem, reactor, FSMs and health row; extends child 2's per-peer record | AC-52 to AC-62, AC-68, AC-98 | P0, child 2 phase 3 | - |
| P7 L2TP kernel, PPP, PPPoE | the kernel files, ppp, pppoe; the `code` renames in ppp and pppoeclient | AC-51, AC-63 to AC-65, AC-72 to AC-80, AC-102 (PPP) | P0 | ∥ P1 |
| P8 subscriber plugins | the subscriber handler names, authradius, authlocal, pool, shaper, relay, consumers | AC-66, AC-67, AC-69 to AC-71, AC-81 to AC-97, AC-102 (authradius) | P6 | - |
| P9 L2TP sweep and docs | the L2TP line sweep; every page in the Documentation checklist; the AC-103 report | AC-99, AC-100, AC-103, AC-104 | P2 to P8 | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC at file and symbol; `./le arch doctor-coverage report` shows the area covered |
| Correctness | no check reads healthy for state it could not see; runtime checks never read `Tree` |
| Naming | codes kebab-case with the `doctor-` prefix; `doctor-ipsec-iface` unchanged |
| Data flow | IKE readers outside the engine use the core leaves; subscriber state through the handler registries |
| Rule: user-facing-errors | every fix is a command alone in `Fix` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every new code explained | `TestAccessSecurityCodesHaveMetadata`; `ze show diagnostic <code>` |
| area covered | `./le arch doctor-coverage report` |
| tests | the unit and `.ci` tests above pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | no row prints a PSK, L2TP secret, RADIUS secret, TACACS+ key, password or EAP credential |
| Usernames | subscriber and operator usernames appear only in `show doctor`, which stays behind authz |
| Resource use | runtime checks read records and snapshots; the kernel dumps of AC-30 and AC-64 are bounded by Ze's own entries |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A class has no readable state | back to the inventory row; name the record the owner must keep |
| A fix names nothing that exists | DESIGN: verify against the command registry and YANG |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Slice A1 named the `vpn ipsec interface` code `doctor-ipsec-interface`; `codes.go` holds `doctor-ipsec-iface`, which stays
- `doctor-ipsec-listen` was registered with metadata and never produced; AC-1 gives it its producer
- The codes this spec adds are not in the merge's CODES.md table, because the slices proposed names only for rows that already had a code; every new name follows the `doctor-<area>-<problem>` form CODES.md uses

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-fires-outside-its-population.md` | 2026-11-10 | `l2tp` health row registered unconditionally |
| `zero-value-as-valid-answer.md` | 2026-11-10 | three IKE engine-state doctor checks at post-config |
| `normal-state-logged-at-error.md` | 2026-11-10 | pool `ipv6-pd delegation-length` default logged at Warn |
| `published-value-drifts-from-the-behavior-it-describes.md` | 2026-11-10 | `doctor-ipsec-listen` metadata with no producer |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One IKE per-peer last failure record for every transient IKE class | a record per class; read the report bus | the bus is process-local and evicts; one record per peer bounds memory and gives one "why is this peer down" answer; child 2 reads the same record |
| Extend child 2's L2TP per-peer record | a second record | one owner state per peer |
| Transient classes raise on the bus and the owner's check declares the code | a marker | parent O-2 |
| `l2tp` health row with `RegisterWhen` | keep it and filter in doctor | the row is false on an unconfigured box; the registration owns its applicability |

## Known Limitations
- The checks for plugin exit and external refusal are `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`'s (phase P0a); this spec puts their codes on its sites.

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
- [ ] AC-1..AC-104 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests under `./le job run` for each package in Files to Modify (with `ze_ike`, `ze_l2tp`, `ze_radius`, `ze_tacacs` as each needs), and the named `.ci` files through `./le test functional plugin` and `./le test functional ui` selecting those tests; no agent runs any `./le verify` command
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
- [ ] Interop tests for protocol features (N-A, reason in the Interop table)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-3-access-security.md` only, in the same `./le commit create` script
