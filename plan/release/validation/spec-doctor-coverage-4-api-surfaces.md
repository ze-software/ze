# Spec: doctor-coverage-4-api-surfaces

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | component |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md` (Goal and scope): `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of the parent makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the recent-events check and the gate, landed in commits ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes `doctor-internal-defect`, `doctor-plugin-*`, `doctor-audit-record-failed`, the event-code registration and the raise-carries-fix convention); its P0b (the report route for a forked plugin) is not needed here, because every surface in this spec runs in the daemon process and raises through `internal/core/report` directly, as `setNeighborAS` does today; `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` (what `cmd/ze/hub` decides at boot when a listener fails to bind, the split of the doctor `listeners` bind probe, and AC-7, the gNMI TLS refusal landed in 537b3a87f9); `plan/release/validation/spec-doctor-coverage-9-config-yang.md` for `doctor-config-password-weak` (AC-5) and the store row's code; `plan/release/validation/spec-doctor-coverage-8-commands-cli.md` keeps the SSH editor-storage row |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Inventory: `plan/audits/doctor-problem-inventory/api-surfaces.md` (slice A3 api-surfaces), plus the SSH rows of `plan/audits/doctor-problem-inventory/commands-cli.md` (listener, login failures, user list unreadable, public key invalid, audit record). The cross-child decision assigns those here: this child owns every per-protocol listener class (SSH, web, MCP, REST, gRPC, gNMI, LG, telemetry), the TLS material checks and the management auth failure classes. The commands-cli child keeps the SSH editor-storage row.

## Task

Doctor reports every problem class of the management surfaces (SSH, web, looking
glass, MCP, REST, gRPC, gNMI, the Prometheus exporter and its OS collectors)
with its cause in plain words and the command that fixes it, on every surface
through `show doctor`. Every Warn and Error log line, and every report-bus
raise, in the area names its doctor code (parent O-2), so the doctor-coverage
gate counts it covered. A Warn line that reports nothing wrong (a client that
went away, a refusal its caller already sees) drops to Info or Debug instead.
Three defects found by the audit are fixed here: an SSH public key that does not
parse passes commit; the web add-entry form drops a field that fails to set; gNMI
binds only the first configured listener. The fourth, gNMI serving plaintext
when its configured TLS material cannot be read, is fixed by child 11 AC-7
(`plan/release/validation/spec-doctor-coverage-11-hub-engine.md`), landed in
537b3a87f9 with this spec's doctor check that reports it (AC-10).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform constraints
  → Constraint: a check is registered by the package that owns the state it reads ("Shared machinery", readiness check row); no central list names a check
  → Constraint: every surface compiles out (`ze_ssh`, `ze_web`, `ze_lg`, `ze_mcp`, `ze_rest`, `ze_grpc`, `ze_gnmi`, `ze_telemetry`), so each surface's check lives in that surface's package and leaves with it
- [ ] `ai/doors/api-surfaces.md` - the area door
  → Constraint: the surfaces are thin adapters; the LG asks the dispatcher and never imports a RIB or peer package, so the LG backend check asks the command registry, never a plugin
  → Constraint: each surface carries its own authentication and TLS; a listener that serves without it is a security defect (gNMI TLS row)
  → Constraint: `cmd/ze/hub` reaches each surface through a registration seam (`cmd/ze/hub/service_registry.go`, `cmd/ze/hub/gnmi_infra.go`), never an always-on import
- [ ] `ai/doors/commands-cli.md` - SSH
  → Constraint: SSH public-key and password login are an outside contract; the verifier refuses only what login can never accept
- [ ] `ai/doors/hub-engine.md` - service construction and reload
  → Constraint: an optional service registers a factory in `cmd/ze/hub/service_registry.go`; `buildServices` logs and skips a failed build; the boot decision stays with the hub child (spec 11)
  → Constraint: one reload path (`doReload`); the gNMI listener set migrates on it like the others
- [ ] `ai/doors/config-yang.md` - verifiers
  → Constraint: `ze config validate` runs YANG checks and each `InProcessConfigVerifier`, never a live `OnConfigVerify`; the SSH key refusal must run on validate and commit alike, or validate accepts what commit refuses
  → Constraint: SSH is a component, not a plugin: it has no `registry.Registration` and so no `InProcessConfigVerifier`. A component's refusal registers with `config.RegisterTreeValidator` (`internal/component/config/tree_validators.go`), which `config.ValidateCustomSections` runs for `LoadConfig` (start and SIGHUP), `ze config validate` and the editor's commit, as `internal/component/config/system/register.go` and `internal/component/l2tp/register.go` do
- [ ] `ai/doors/core.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md` - doors of the paths this spec names
  → Constraint: new codes go in `internal/core/diagnostic/codes.go` and nowhere else; a registry is the only list
  → Constraint: `ze` never links `internal/le`; the gate reads codes from the registries
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Data Flow 1b, 2a (amended), 2b, 4b
  → Decision: a `runtime` check receives `Platform` only; it reads its owner's live state, never `Tree` or `ConfigDir`
  → Decision: a transient code counts as covered only when the check of the code's owner declares it; `recent-events` then reports the bus event under that code
- [ ] `plan/release/validation/spec-doctor-coverage-3-access-security.md` - AAA backend rows
  → Constraint: RADIUS, TACACS+ and authz failures are child 3's codes; a management login refused on any surface is this spec's `doctor-mgmt-auth-failed`, the one code for that class
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the runtime phase
  → Constraint: a `runtime` check applies nothing and only reads
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: the message states the problem and why; the fix is the command alone, on its own line, in `Fix`
- [ ] `ai/rules/spec-no-code.md`
  → Constraint: tables, no code

**Key insights:**
- `generateToken` failing (`internal/component/web/auth.go` `createSession`) is unreachable on the tree's Go 1.27: `crypto/rand.Read` has not returned an error since Go 1.24. The line is a defect path, and the unseeded pool stays `random-seed` (`doctor-random-seed`)
- `logCommitWarnings` (`internal/component/api/config_session.go`) logs every commit advisory at Warn, so one site carries two classes (weak password, draft marker); each advisory is raised with its own code
- `lg-unreadable-as-number` is raised and cleared only in `internal/component/lg/handler_api.go` `setNeighborAS`; no other reader of the string exists in the tree
- the hub's MCP start path prints to stderr only (`cmd/ze/hub/service_mcp.go` `startMCPServer`), and gNMI's bind-pending path does too (`cmd/ze/hub/service_gnmi.go`, "gNMI server failed to bind"); gNMI's `Serve` refusal is a coded Error line since ad9fc4abe2 (`serveGNMI`). A log line still records no state, so the runtime check needs a serving state each surface keeps, never a log scrape
- MCP has no server type in its own package: `mcpServerHandle` and `startMCPServer` live in the gated hub file `cmd/ze/hub/service_mcp.go`, which binds the MCP listeners itself, so the MCP serving state lives in `internal/component/mcp` and that hub file writes it through the mcp package's setter
- the deprecated telemetry leaves (`prefix`, `interval` and `collector` directly under `telemetry prometheus`, `status deprecated` in `ze-telemetry-conf.yang`) are read as fallbacks by `extractNetdataConfig` and listed by `deprecatedNetdataAliases` (`internal/component/telemetry/exporter/server.go`); no release shipped them, and no config, `.ci` or doc example in the repository writes them (search of `test/`, `docs/`, `internal/`, `cmd/`, `website/`): only `server_test.go` exercises them and `docs/guide/monitoring.md` names them
- the gNMI token refusal (`authUnaryInterceptor`, `authStreamInterceptor`) neither logs nor audits today; it only answers `Unauthenticated`
- `internal/component/ssh/doctor.go` already holds `ssh-host-key` (post-config, order 2010) and `internal/component/ssh/register.go` registers it; the new SSH checks join those files

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/doctor_checks.go` - `listeners` (post-config, order 2015) declares `doctor-listen-unavailable` and the protocol listen codes; `random-seed` (pre-config)
- [ ] `internal/component/doctor/checks_listener.go` - `checkListeners` bind-probes every schema `ze:listener` endpoint plus the protocol extractors; on the daemon route the daemon holds those ports
- [ ] `internal/component/doctor/cmd/show.go` - `HandleDoctor` runs `doctor.Answer` with `RunRuntimeChecks` and the plugin checks
- [ ] `internal/component/doctor/registry.go` - the phase dispatch over the `internal/core/diagnostic` registry
- [ ] `internal/component/doctor/checks_recent_events.go` - `recent-events` (runtime, order 3000) renders the report bus under each event's code
- [ ] `internal/core/diagnostic/codes.go`, `internal/core/diagnostic/doctor_cert.go` - code metadata; `DoctorCertPair` (missing, invalid, expired) used by the TLS material checks
- [ ] `internal/core/report/report.go` - warnings dedup on (source, code, subject); errors are a bounded ring
- [ ] `internal/component/mcp/doctor.go` - `mcp-tls-material` (post-config) over `ExtractMCPConfig`; the same shape in `internal/component/web/doctor_material.go` (`web-tls-material`) and `internal/component/api/grpc/doctor.go` (`api-grpc-tls-material`)
- [ ] `internal/component/web/register.go`, `internal/component/lg/register.go` - `web-tls-certificate`, `lg-tls-certificate` registered from `init()`
- [ ] `internal/component/telemetry/exporter/doctor_linux.go` - `telemetry-procfs`
- [ ] `internal/component/web/server.go` - `serveOne` logs a `Serve` error and the address stays in `Addresses()`; `closeAllListeners` Warns on cleanup
- [ ] `internal/component/web/auth.go` - `validateToken` refuses an anchored session when `localUserDeclared` fails (Warn); `AuthMiddlewareWithAudit` and `LoginHandlerWithAudit` log failures at Warn and call `recordWebAuthFailure`, which logs a failed audit write at Warn; `createSession` failure at Error
- [ ] `internal/component/web/handler_config_entry.go` - `HandleConfigAddWithAuthorizer` logs a failed `SetValue` after `CreateEntry` at Warn and answers success
- [ ] `internal/component/web/handler_config_commit.go` - `recordWebAudit` logs a failed audit write at Warn
- [ ] `internal/component/lg/server.go` - `query` Warns on a dispatch error; `ask` Warns on empty, non-JSON and refused replies
- [ ] `internal/component/lg/handler_api.go` - `setNeighborAS` raises and clears `lg-unreadable-as-number`; `transformBMPProtocols` omission Warn
- [ ] `internal/component/gnmi/server.go` - `Serve` loads the configured cert and key paths (`loadTLSCredentials`, `tls.go`) before it listens and refuses a half pair, an unreadable file or an invalid pair (537b3a87f9, child 11 AC-7); `Address` is empty when not serving; `Status` reports the listener
- [ ] `internal/component/gnmi/subscribe.go` - `Notify` drops on a full client buffer with one Warn per drop, no count
- [ ] `internal/component/gnmi/metrics.go` - the `code` key there is a metric label, not a log attribute
- [ ] `internal/component/api/grpc/server.go`, `internal/component/api/rest/server.go` - per-listener `Serve` error at Error, close Warns, `recordAudit` failure Warn
- [ ] `internal/component/api/config_session.go` - `logCommitWarnings` Warns each weak-password advisory and the `MarkCommittedContent` failure
- [ ] `internal/component/telemetry/exporter/exporter.go` - `Start` Warns and returns nil when the metrics server fails to start; Warns each entry of `cfg.DeprecatedAliases`
- [ ] `internal/component/telemetry/exporter/server.go` - `extractTelemetryConfig` fills `DeprecatedAliases` through `extractNetdataConfig`, which falls back to the deprecated `prefix`, `interval` and `collector` beside the `netdata` container, and `deprecatedNetdataAliases`, which names each one present
- [ ] `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang` - the three `status deprecated` nodes under `prometheus`, and the `netdata` container description that names them
- [ ] `internal/component/telemetry/collector/collector.go` - `collectAll` Warns each failed collection, keeps no error
- [ ] `internal/component/ssh/pubkey.go` - `matchPublicKey` parses each configured key at login (`parseConfiguredKey`), logs and skips one that does not parse; `authenticatePublicKeyResult` Warns on unreadable users and on a failed match
- [ ] `internal/component/ssh/passwordauth.go` - Warn and `recordAuthFailure` on a refused password
- [ ] `internal/component/ssh/ssh.go` - `recordAuthFailure` Warns a failed audit write
- [ ] `internal/component/ssh/listener_start.go` - `serveListener` logs a `Serve` error at Error
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - `public-keys` under `system authentication user`; its description says a key that does not parse is logged and skipped
- [ ] `internal/component/config/infra/authz.go` - `extractPublicKeys` copies type and key with no parse
- [ ] `internal/component/config/tree_validators.go` - `RegisterTreeValidator`; `internal/component/config/validate_sections.go` `ValidateCustomSections` runs every registered validator for `LoadConfig` (start, SIGHUP), `ze config validate` and the editor's commit
- [ ] `cmd/ze/hub/service_gnmi.go` - `gnmiBuildImpl` binds only the first listener (Warn on the rest); it passes the pair from `zeconfig.GNMITLSPaths`, the reader the `gnmi-tls-material` check shares, to the server, which refuses to serve on material that does not load (537b3a87f9)
- [ ] `cmd/ze/hub/gnmi_infra.go` - `resolveGNMIListeners`, the one resolver of the gNMI address and token
- [ ] `cmd/ze/hub/service_mcp.go`, `cmd/ze/hub/service_web.go`, `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/service_registry.go` - each declines or fails a surface at boot with stderr or one log line

**Behavior to preserve:**
- every existing check name, code and message in the area (`web-tls-certificate`, `lg-tls-certificate`, `web-tls-material`, `api-grpc-tls-material`, `mcp-tls-material`, `telemetry-procfs`, `ssh-host-key`)
- the web server fails closed with no readable user list, and an anchored session is refused when the list cannot be read
- the LG public refusal classes (engine error, engine unavailable, unreadable reply) and their HTTP statuses (`docs/architecture/api/birdwatcher-compat.md`)
- gNMI drops a notification for a slow client rather than block the commit path

**Behavior to change:**
- each surface keeps a serving state and a login-failure record; a runtime check per surface reports them
- an SSH public key that does not parse is refused at commit and at `ze config validate`, and by the same walk at start and SIGHUP reload
- the web add-entry form is refused as a whole when a field fails to set
- the deprecated telemetry leaves are deleted and their fallback reads removed; a config that writes one is refused by the schema
- gNMI binds every configured listener; doctor reports unreadable or invalid gNMI TLS material (the refusal to serve on it is child 11 AC-7; both landed in 537b3a87f9)
- every gap class below gets a doctor check with code, severity, message and fix
- every Warn and Error line in the area carries a doctor code, or drops to Info or Debug where it reports nothing wrong

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Listener bind probe | `internal/component/doctor/doctor_checks.go` `listeners`, `internal/component/doctor/checks_listener.go` `checkListeners` | probes every configured endpoint, these surfaces' included, post-config | does not apply here as an owner: spec 11 splits it (AC-9 row); this spec's per-surface runtime checks answer for the management addresses on the daemon route (AC-6) |
| TLS material checks | `web-tls-material`, `api-grpc-tls-material`, `mcp-tls-material`, all over `diagnostic.DoctorCertPair` | report a missing, unparsable or expired pair under `doctor-tls-*` | extends: gNMI gained `gnmi-tls-material` over its pair, env overrides included (AC-10, landed in 537b3a87f9); it runs the server's own loader, so it agrees with the refusal child 11 AC-7 landed in the same commit |
| Certificate reference checks | `web-tls-certificate`, `lg-tls-certificate` | config references to `pki` certificates | does not apply: correct as local checks; kept |
| Procfs check | `telemetry-procfs` | `/proc` readability for every OS collector | does not apply: covers the procfs half; kept |
| SSH host key check | `internal/component/ssh/doctor.go` `ssh-host-key` | host key and certificate files | extends: the `ssh-public-keys` local check and the SSH runtime check join the same file and registration |
| Random seed check | `random-seed` | unseeded entropy pool | does not apply: the session-token row is a defect path on Go 1.27 (AC-14) |
| Surface address lists | `WebServer.Addresses`, `LGServer.Addresses`, `GRPCServer.Addresses`, `RESTServer.Addresses`, `mcpServerHandle.Addresses`, `gnmi.Server.Address` and `Status` | the bound set, or the configured set before bind; never cleared on a `Serve` return | extends: each surface keeps a serving state beside them (configured set, served set, cause); the address methods keep their answer. `mcpServerHandle` is in the gated hub file `cmd/ze/hub/service_mcp.go`, so for MCP that file writes the state through the `internal/component/mcp` setter |
| Audit recorder | `audit.Recorder` `Record`, `audit.ActionAuthFail`; `show audit` | stores login failures and commits per surface | extends: the login-failure record counts what each site already records; the fix line of `doctor-mgmt-auth-failed` is `show audit` |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `RaiseError`, `ClearWarning` | coded issues | extends: transient classes raise under their doctor code; the LG raise is re-coded (AC-3) |
| Recent-events check | `internal/component/doctor/checks_recent_events.go` `recentEventsCheck` | renders bus events under each code with metadata | extends: each transient code gets metadata and is declared by its owner's check |
| Doctor runtime phase | `diagnostic.DoctorPhaseRuntime`; `RunRuntimeChecks`; `HandleDoctor` | runs runtime checks in the daemon only | extends: one runtime check per surface registers here from the surface's own package |
| Fix field | `diagnostic.Diagnostic.Fix` | the fixing command alone | extends: every new diagnostic sets it |
| Diagnostic code registry | `internal/core/diagnostic/codes.go`; `ze show diagnostic <code>` | explains each code | extends: every new code gets metadata |
| Command registry | the dispatcher the LG holds (`internal/component/lg/server.go` `query`) and the registry it dispatches into | answers commands | extends: the LG backend check asks whether each command the LG issues is registered, without calling it |
| Config verifiers | `InProcessConfigVerifier` on a plugin's registration (`ai/doors/config-yang.md`, Traps); `config.RegisterTreeValidator` for a component (`internal/component/config/tree_validators.go`) | the plugin verifier runs at `ze config validate`; the tree validator runs in `ValidateCustomSections` for start, reload, validate and commit | extends: SSH is a component with no `registry.Registration`, so it registers the public-key validator with `config.RegisterTreeValidator` from `internal/component/ssh/register.go` (AC-13) |
| Deprecated telemetry aliases | `internal/component/telemetry/exporter/server.go` `extractNetdataConfig`, `deprecatedNetdataAliases`, `telemetryConfig.DeprecatedAliases`; three `status deprecated` nodes in `ze-telemetry-conf.yang` | read the old leaves when the `netdata` container lacks the setting, and Warn per alias | replaces: deleted outright, because no release shipped them and the pre-release rule converts a form no release shipped directly, with no compatibility path (`ai/META-DECISIONS.md`, "Ze is pre-release") (AC-21) |
| Plugin-infra codes | spec 12: `doctor-internal-defect`, `doctor-audit-record-failed` and their checks | declared by plugin-infra's checks | does not apply as a check here: the sites in this area carry those codes |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` | lists uncovered sites | extends: this spec's sites move to covered; the report over the area is the deliverable check |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show doctor` on every daemon surface; `ze doctor` locally (asks the daemon, or answers locally with `doctor-runtime-not-checked`)
- a failure in a surface (a bind, a `Serve` return, a login refused, a dropped notification) recorded in that surface's state and, for a transient class, raised on the report bus

### Transformation Path
1. The surface package keeps its state: the serving state (configured addresses, served addresses, cause, time), the login-failure record, and the per-class counters the ACs name
2. The surface's server writes the state at bind, at `Serve` return and at reload; the hub writes the cause through the surface package's own setter when it declines to build the surface (the decision itself is spec 11's)
3. The surface's runtime check reads its own state and answers its rows; a local check reads config (TLS pair, public keys, deprecated paths)
4. `HandleDoctor` or the local route draws each diagnostic with its `fix` line

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub to surface | the gated `cmd/ze/hub/service_*.go` file calls the surface package's state setter | No |
| LG to command registry | a registry lookup through the dispatcher the LG already holds | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `test/`, `docs/`; terms `lg-unreadable-as-number`, `reportCodeUnreadableAS`, `"code",` on Warn and Error lines in the slice's packages, `parseConfiguredKey`, `public-keys`, `ExtractGNMIConfig`. Positive control: `internal/component/lg/handler_api.go` `setNeighborAS` writes the LG code; `internal/component/ssh/pubkey.go` `matchPublicKey` reads the keys. Findings: no reader of `lg-unreadable-as-number` outside that file; no log attribute named `code` in the slice's packages (the gNMI `code` is a metric label); `parseConfiguredKey` has one caller, at login

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| report bus code for an unreadable LG AS | writer | `internal/component/lg/handler_api.go` `setNeighborAS` | raises and clears `lg-unreadable-as-number` | change to `doctor-lg-unreadable-as-number` in both calls |
| report bus code for an unreadable LG AS | reader | `show warnings`, `recent-events`, API clients outside the repository | no in-tree reader | pre-release, no compatibility owed; the new code is registered |
| gNMI listen set | writer | operator config `environment gnmi server` and `ze.gnmi.listen`, resolved by `cmd/ze/hub/gnmi_infra.go` `resolveGNMIListeners` | the list is accepted whole | preserve: no config change |
| gNMI listen set | reader | `cmd/ze/hub/service_gnmi.go` `gnmiBuildImpl`; gNMI clients outside the repository (gnmic, controllers) | binds the first entry only | every entry is bound; a client reaching a second address now connects |
| `public-keys` entry | writer | operators and scripts that `set system authentication user <name> public-keys <key>`; repository configs under `test/` and `docs/` | accepted unchecked | an entry that does not parse is refused at commit; repository configs carrying one are converted in the same work |
| `public-keys` entry | reader | `internal/component/config/infra/authz.go` `extractPublicKeys`, `internal/component/ssh/pubkey.go` `matchPublicKey`, `config.ValidateCustomSections` (start, reload, `ze config validate`, commit) | parsed only at login | parsed by the SSH tree validator as well; the YANG description is rewritten |
| `telemetry prometheus prefix`, `interval`, `collector` (deprecated) | writer | operators; repository configs, `.ci` files and doc examples (none found, search in Key insights) | accepted, read as a fallback, Warned | deleted from the schema; a config that writes one is refused at parse; no migrate path, because no release shipped them (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| same | reader | `internal/component/telemetry/exporter/server.go` `extractNetdataConfig`, `deprecatedNetdataAliases`; `exporter.go` `Start`; `server_test.go`; `docs/guide/monitoring.md` | read when `netdata` lacks the setting | the fallback reads, the alias list, its Warn line and the doc sentence are removed; the alias assertions in `server_test.go` are converted to the `netdata` form |
| log levels of the Not-problem-class lines | writer | the sites of AC-24, AC-27, AC-28 | Warn today | Debug or Info |
| log levels of the Not-problem-class lines | reader | operator log searches and alerting outside the repository; the doctor-coverage gate | the gate counts Warn and Error only | no level contract is published for these lines; the gate stops counting them |
| `show doctor` JSON | writer and reader | unchanged format; new codes only | parent census | preserve |

### Integration Points
- each surface package registers its runtime check with `diagnostic.RegisterDoctorCheck` from its own registration file
- `internal/component/gnmi` registers the local `gnmi-tls-material` check the same way
- `internal/component/ssh` registers the public-key validator with `config.RegisterTreeValidator` from `register.go` (SSH has no plugin registration, so no `InProcessConfigVerifier`) and the local `ssh-public-keys` check in `internal/component/ssh/doctor.go`
- `internal/component/mcp` holds the MCP serving state and its setter; `cmd/ze/hub/service_mcp.go` (`startMCPServer`, `mcpServerHandle.Reconfigure`) writes it, because that gated file binds the MCP listeners

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every row reaches `show doctor` through the runtime runner or a local check |
| No unintended coupling (components stay isolated) | Yes | each check lives in its surface package; the hub writes state only through the gated service file |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; the gNMI TLS check (landed) reuses the server's `loadTLSCredentials` and `DoctorCertExpiry`; the refusal itself is child 11 AC-7, not rebuilt here |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path; records keep counts and the last occurrence |
| Registration over hardcoding, outbound | Yes | checks register from their owner |
| Registration over hardcoding, inbound | Yes | the LG backend check reads the registry, never a list of plugins; the gate derives codes from the registries |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every management surface runs in the daemon process, so a runtime check in its package reads live state | `ai/doors/api-surfaces.md` (wired in `cmd/ze/hub`) | a check reads an empty copy | `test/plugin/show-doctor-mgmt-listener-not-serving.ci` | unvalidated |
| A-2 | Spec 11 splits `listeners` so the daemon route skips the management addresses its own surfaces serve | cross-child decision | the daemon route reports a served address as `cannot bind` | the AC-6 test against spec 11's split | unvalidated |
| A-3 | `crypto/rand.Read` never returns an error on the tree's Go 1.27 | Go 1.24 release notes; `go.mod` | `createSession` can fail for a host reason | `TestSessionTokenFailureIsDefectPath` | unvalidated |
| A-4 | Every fix named in an AC exists as a command or YANG path | each AC names it by kind | a fix names nothing | each check's unit test resolves its fix against the command registry or YANG (parent AC-7) | unvalidated |
| A-5 | The command registry answers "is this path registered" without dispatching | `ai/doors/commands-cli.md` (dispatch keys are YANG paths) | the LG check must call the command | `TestLGBackendCheck` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A brute force floods the report bus and `show doctor` | thousands of auth rows | one row per surface with counts and the last user and source; the bus groups on (source, code, subject) with the surface as subject |
| R-2 | A row prints a password, token or key | review | messages name the user, the source address and the key name, never the value |
| R-3 | gNMI refusing to serve on unreadable TLS removes a listener an operator relied on | a box that served plaintext gNMI by accident | resolved: the refusal is child 11's AC-7 (`plan/release/validation/spec-doctor-coverage-11-hub-engine.md`), landed in 537b3a87f9 as the security fix; this spec's `gnmi-tls-material` check (AC-10, same commit) names the file and the TLS setting |
| R-4 | The public-key validator refuses a config that boots today | an existing config with a broken key | that key never authenticated; the refusal names user and key and the delete path; repository configs are converted in the same work. The tree validator runs in `LoadConfig` too, so a stored config holding such a key is refused at start and at SIGHUP, as `ze config validate` already says it will (one walk, `ValidateCustomSections`); `ssh-public-keys` names the key on the local route |
| R-5 | A slow gNMI client floods the log with one Warn per drop | many lines | the Warn fires on the first drop of a burst; the counter carries the total |
| R-6 | One unserved management address answers three rows on the daemon route: this spec's runtime row (AC-5), spec 11 AC-1's inverted `listeners` probe ("configured but nothing serves it", under `doctor-listen-unavailable`), and spec 11 AC-3's report-bus warning rendered by `recent-events` under `doctor-mgmt-listener-not-serving`; the same holds for `doctor-mgmt-auth-failed` (AC-11 row plus the bus raise of the same refusal) | duplicate rows in `TestSurfaceRuntimeChecks*` or the `.ci` | OPEN, a cross-spec decision for the main thread before P2: one class answers one row. Candidates: the hub records through the surface setter and raises nothing for these eight surfaces (spec 11 AC-3 then covers only services outside this list), and `listeners` on route `daemon` skips an address a surface reports in its serving state |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a false or missing doctor row; a refused SSH key commit; gNMI binding more addresses |
| How is it reverted? | one commit per phase |
| Who else touches this path? | spec 11 (hub boot decisions, the `listeners` split), spec 12 (plugin-infra codes), the config-yang child (weak-password and store codes), the commands-cli child (SSH editor storage) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with an MCP listener whose port another socket holds at boot | → | MCP runtime check over its serving state | `test/plugin/show-doctor-mgmt-listener-not-serving.ci` |
| `show doctor` after two refused SSH password logins | → | SSH login-failure record, `doctor-mgmt-auth-failed` | `test/plugin/show-doctor-mgmt-auth-failed.ci` |
| commit of a user `public-keys` entry whose key does not parse | → | SSH public-key verifier | `test/plugin/ssh-public-key-invalid-refused.ci` |
| `ze doctor --json config` with a stored config holding such a key | → | local `ssh-public-keys` check | `test/ui/doctor-ssh-public-key-invalid.ci` |
| gNMI with two `server` entries, a client on the second | → | `gnmiBuildImpl` binding every listener | `test/plugin/gnmi-multiple-listeners.ci` |
| gNMI with a configured cert path that does not exist | → | the hub refusal (child 11 AC-7) and `gnmi-tls-material` | `test/plugin/gnmi-tls-unreadable-cert-not-started.ci`, landed in 537b3a87f9 under child 11 AC-7 |
| web add-entry form with a field that fails to set | → | `HandleConfigAddWithAuthorizer` refusal | `test/web/config-add-entry-field-refused.wb` |
| `show doctor` with the LG enabled and no `bgp-rib` | → | LG runtime check over the command registry | `test/plugin/show-doctor-lg-backend-unavailable.ci` |
| `ze doctor --json`, no daemon | → | the surface runtime checks skipped, `doctor-runtime-not-checked` | `TestSurfaceRuntimeChecksAreRuntimePhase` |

## Acceptance Criteria

Expected Behavior names what doctor reports: code, severity, the state it
reads, route (local or runtime) and fix. The fix names a command or config
path, which the check's unit test resolves (A-4). A transient class also
raises on the report bus under the same code at the failure site, and the
owner's runtime check declares the code. "Each surface" means SSH, web, LG,
MCP, REST, gRPC, gNMI and the Prometheus exporter.

**Shared mechanisms**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Any surface binds, stops serving, is reloaded, or is declined at boot | the surface package keeps one serving state: configured addresses, served addresses, cause per unserved address (bind, TLS load, store unavailable, user list unreadable, no users, serve error, build error) and time; its server writes it at bind, on every `Serve` return that is not a stop, and at reload add or remove; the hub writes the cause through the surface's setter where it declines the surface | the boot decision is spec 11's; this spec provides the setter and the readers |
| AC-2 | A management login is refused on any surface | the surface keeps a login-failure record: count since start, last user, last source address, last time; written where the site already logs or audits the refusal (SSH password and public key, web form and basic auth, REST `recordAuthFailure`, gRPC `recordAuthFailure`, MCP `audit.go`), and at the gNMI token refusal (`authUnaryInterceptor`, `authStreamInterceptor`), which logs and audits nothing today, so the record is its first trace | none |
| AC-3 | The LG raises or clears the unreadable-AS warning | both calls use `doctor-lg-unreadable-as-number`, which has metadata; no reader of the old string remains | none |
| AC-4 | An audit record write fails on gRPC, REST, web login, web commit or SSH | each site raises on the bus and logs under `doctor-audit-record-failed`, declared by spec 12's check | the check is spec 12's |

**Listeners and TLS**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-5 | A configured address of any surface is not served: boot bind or TLS failure, a surface declined at boot, a `Serve` error after start, a listener a reload added that failed | the surface's runtime check reports `doctor-mgmt-listener-not-serving` (error) per unserved address: surface, address, cause, time; fix names the leaf or command the cause calls for (the listen address leaf, the TLS leaf, `ze init` for a missing store, the user config); the `Serve` Error lines of web, LG, REST and gRPC and the hub LG and service-build lines carry the code | an address a reload removed is not configured and not reported |
| AC-6 | AC-9 row: `listeners` bind-probes these surfaces' addresses on the daemon route, where the daemon holds them | spec 11 AC-1 makes the probe read its route from a field of `DoctorCheckContext`: on route `daemon` an address-in-use bind reads healthy and a bind that succeeds is the finding; on route `local` the verdict is today's. A healthy served management address therefore never reads as `cannot bind` on the daemon route, and this spec's runtime checks answer for the management addresses with their cause | the probe change is spec 11's (AC-1); this spec's test asserts the runtime row and the absence of a `doctor-listen-unavailable` row for a served address. The second row an unserved address then gets from the inverted probe is R-6, open |
| AC-7 | The Prometheus exporter fails to start (`internal/component/telemetry/exporter/exporter.go` `Start`) | the exporter's serving state carries the cause; its runtime check reports `doctor-mgmt-listener-not-serving` with the endpoint; the Warn line carries the code | none |
| AC-8 | The SSH server fails its config or `Start` at boot, or `serveListener` returns an error | the SSH serving state carries it; the SSH runtime check reports `doctor-mgmt-listener-not-serving`; the SSH Error line carries the code | the hub boot lines in `cmd/ze/hub/service_ssh.go` are spec 11's sites |
| AC-9 | `environment gnmi server` lists more than one entry (defect, Owned journal rows) | gNMI binds every entry, all or none, as REST and gRPC do, and reload migrates the set; the "extra listeners ignored" Warn line is removed; `doctor-gnmi-listener-ignored` is not registered, because the class no longer exists | none |
| AC-10 | gNMI's configured cert or key (`ze.gnmi.tls.cert` / `.key` or the config TLS leaves) is unreadable, or the pair does not load | a local check `gnmi-tls-material` in `internal/component/gnmi` reads the same pair the hub reads, env overrides included, and reports `doctor-gnmi-tls-material` for each case: an error for a half pair, a file that cannot be read (named, with the read error), material that does not parse, a pair that does not match, or a certificate outside its validity window; a warning for a certificate near expiry; fix names the TLS setting and `ze signal restart`; when the hub declines gNMI on that cause, the gNMI serving state carries it and AC-5 reports it. LANDED in 537b3a87f9: `TestGNMITLSMaterial`, `TestGNMITLSMaterialCheckRegistered` | the refusal to serve (plaintext before) is child 11's AC-7 (`plan/release/validation/spec-doctor-coverage-11-hub-engine.md`), landed in the same commit with its journal row in `silent-fall-through.md`; this spec owns the check and the code, which child 11 reuses |

**Authentication and users**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-11 | Logins refused on a surface since start (AC-2 record non-empty) | the surface's runtime check reports `doctor-mgmt-auth-failed` (warning): surface, count, last user, last source, last time; fix `show audit`; the refusal sites raise on the bus under the same code with the surface as subject; the refusal Warn lines in `internal/component/ssh/passwordauth.go`, `internal/component/ssh/pubkey.go` and `internal/component/web/auth.go` carry the code | a RADIUS or TACACS+ backend fault behind the refusal is child 3's code; the refused login itself is this code |
| AC-12 | The running configuration's user list cannot be read (web `localUserDeclared`, SSH `s.users()`) | the surface keeps the last read error and clears it on a successful read; runtime `doctor-mgmt-user-list-unreadable` (error): surface, error, every login anchored on the local list is refused; fix names the command the error's cause calls for (A-4); the two Warn lines carry the code | the web boot decline on the same cause is AC-5's cause |
| AC-13 | A user `public-keys` entry whose type and key do not parse (defect, Owned journal rows) | a tree validator the SSH component registers with `config.RegisterTreeValidator` (from `internal/component/ssh/register.go`; SSH has no plugin registration and so no `InProcessConfigVerifier`) parses each entry with `parseConfiguredKey` and refuses the commit, `ze config validate`, start and SIGHUP reload alike (`ValidateCustomSections`), naming user, key name and the parse error, with `delete system authentication user <name> public-keys <key>` as the fix; a local check `ssh-public-keys` reports `doctor-ssh-public-key-invalid` (error) for a stored config that holds one; the `matchPublicKey` Warn line carries the code; the YANG description of `public-keys` is rewritten | none |
| AC-14 | `createSession` fails (`generateToken`) | the Error line carries `doctor-internal-defect`, since `crypto/rand` cannot fail on Go 1.27 (A-3); `random-seed` stays the host check | none |

**Web, gNMI, LG**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-15 | A field of the web add-entry form, or a `ze:suggest` default, fails to set after `CreateEntry` (defect, Owned journal rows) | the handler removes the entry it created from the draft and answers `returnAddError` naming the field and the cause; the draft holds no half-made entry; the two Warn lines are removed | none |
| AC-16 | A gNMI subscriber's buffer is full and a notification is dropped | the notifier keeps a dropped count per subscriber; the gNMI runtime check reports `doctor-gnmi-subscriber-dropped` (warning): subscriber address, drops since it subscribed; fix names `show gnmi`; the Warn line fires on the first drop of a burst and carries the code | a subscriber that left is removed with its count |
| AC-17 | A command the LG issues is not registered (the plugin that serves it is not loaded), or the engine answers nothing | the LG runtime check asks the command registry for each command the LG issues and reports `doctor-lg-backend-unavailable` (error) per missing command, naming the LG endpoints that answer 502 or 503; fix names the command that lists loaded plugins; the `query` and `ask` empty-reply Warn lines carry the code | a configured plugin that is down is `doctor-plugin-stopped` (spec 12) |
| AC-18 | A peer row carries no readable AS | the LG runtime check declares `doctor-lg-unreadable-as-number`; `recent-events` shows the active bus warning under it; fix names the peer show command | none |
| AC-19 | BMP peers left out of `protocols/bmp` (no, duplicate or unroutable `rib-peer`) | the LG keeps the last omission counts; runtime `doctor-lg-bmp-peer-omitted` (warning) with the three counts; fix `show bmp peers`; the Warn line carries the code | none |

**Telemetry and API**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-20 | An OS collector's `Collect` fails | the manager keeps per collector the last error and the count of failures in a row, reset on success; runtime `doctor-telemetry-collector-failed` (warning) per collector whose last run failed: name, error, failures in a row; fix names the collector's `enabled` leaf; the Warn line carries the code | procfs unavailable stays `telemetry-procfs` |
| AC-21 | Config writes `prefix`, `interval` or `collector` directly under `telemetry prometheus` (the deprecated aliases of the `netdata` leaves) | the three `status deprecated` nodes are deleted from `ze-telemetry-conf.yang`, with the `netdata` container description that names them; `extractNetdataConfig` reads the `netdata` container only; `deprecatedNetdataAliases`, `telemetryConfig.DeprecatedAliases` and the `Start` Warn line that lists them are deleted; such a config is refused at parse, naming the leaf; every in-repo user is converted in the same work (the alias cases of `TestExtractTelemetryConfig_NetdataPrefix`, `_NetdataInterval` and `_NetdataCollectors` in `server_test.go`, and the "Deprecated compatibility aliases" sentence of `docs/guide/monitoring.md`); `doctor-telemetry-config-deprecated` is not registered, because the class no longer exists | none: no release shipped the aliases, so the pre-release rule converts them directly with no compatibility path and no migrate hint (`ai/META-DECISIONS.md`, "Ze is pre-release"). The `server_test.go` change removes assertions of a removed behaviour, so it goes to an independent `ze-test-judge` and its commit carries the `Weakening-approved:` line |
| AC-22 | A REST, gRPC or gNMI commit hashed a weak password, or committed and could not mark the draft committed | `logCommitWarnings` raises each advisory under its own code: `doctor-config-password-weak` (declared by the config-yang child, its AC-5) and, for the `MarkCommittedContent` failure (the draft cleanup after a commit that landed, `internal/component/cli/editor_commands.go`), the code the config-yang child registers for that class | the checks are the config-yang child's. OPEN: `spec-doctor-coverage-9-config-yang.md` names no code for the draft-cleanup failure today (its AC-1 list holds none), so P5 waits on that child naming one |

**Not problem classes and sweep**

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-23 | Listener close on cleanup (the 10 inventory sites in the gRPC, REST, LG and web server files) and the SSH reload close | each line carries `doctor-internal-defect` | the bind error reaches AC-5 |
| AC-24 | Response write or encode to a client failed (the 7 inventory sites in LG and web) | each line drops to Debug: the client went away, nothing is wrong | none |
| AC-25 | Page, fragment or template render failed (the 13 inventory sites in LG and web) | each line carries `doctor-internal-defect` | none |
| AC-26 | Engine or YANG contract broken (LG engine reply not JSON, gNMI capabilities loader, gNMI notification marshal) | each line carries `doctor-internal-defect` | none |
| AC-27 | A request refused and the refusal shown to its caller (LG engine refusal, web tools and L2TP disconnect) | each line drops to Info; `doctor-mgmt-request-refused` is not registered | none |
| AC-28 | MCP edge cases: elicitation build failed, input-required suppressed on a forbidden method, unsolicited `requestState` | the first two carry `doctor-internal-defect`; the third drops to Info (the client is refused) | none |
| AC-29 | Every Warn and Error line at the source sites of AC-1 to AC-22 | each carries its doctor code as the `code` attribute; the gate reports them covered | none |
| AC-30 | Every code this spec adds (`doctor-mgmt-listener-not-serving`, `doctor-mgmt-auth-failed`, `doctor-mgmt-user-list-unreadable`, `doctor-ssh-public-key-invalid`, `doctor-gnmi-tls-material`, `doctor-gnmi-subscriber-dropped`, `doctor-lg-backend-unavailable`, `doctor-lg-unreadable-as-number`, `doctor-lg-bmp-peer-omitted`, `doctor-telemetry-collector-failed`; `doctor-gnmi-tls-material` is already registered and declared, landed in 537b3a87f9) | `ze show diagnostic <code>` explains it; the declaring check lists it in `Codes` | none |
| AC-31 | `./le arch doctor-coverage report` after the last phase | the slice's packages and the SSH rows taken here show every site covered | sites of spec 11 in `cmd/ze/hub` are counted there |
| AC-32 | Every check added | its unit test asserts a non-empty fix naming a `ze` command or a `set`/`delete` path, and goes red when the detection is removed | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` after MCP did not come up at boot | hub decline -> MCP serving state -> MCP runtime check | `test/plugin/show-doctor-mgmt-listener-not-serving.ci` |
| 2 | runs `show doctor` on a box under a password guessing attack | SSH refusal -> login-failure record -> SSH runtime check | `test/plugin/show-doctor-mgmt-auth-failed.ci` |
| 3 | pastes a truncated public key | commit refuses, naming user and key | `test/plugin/ssh-public-key-invalid-refused.ci` |
| 4 | points a gNMI client at the second configured address | both listeners serve | `test/plugin/gnmi-multiple-listeners.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSurfaceRuntimeChecksAreRuntimePhase` | one `doctor_runtime_test.go` per surface package | every surface check registers at `runtime` | |
| `TestWebServingState`, `TestWebRuntimeCheck` | `internal/component/web/doctor_runtime_test.go` | AC-1, AC-5, AC-11, AC-12 for web | |
| `TestLGServingState`, `TestLGBackendCheck`, `TestLGUnreadableASCode`, `TestLGBMPOmission` | `internal/component/lg/doctor_runtime_test.go` | AC-1, AC-3, AC-5, AC-17 to AC-19 | |
| `TestMCPRuntimeCheck` | `internal/component/mcp/doctor_runtime_test.go` | AC-1, AC-5, AC-11 | |
| `TestRESTRuntimeCheck`, `TestGRPCRuntimeCheck` | `internal/component/api/rest/doctor_runtime_test.go`, `internal/component/api/grpc/doctor_runtime_test.go` | AC-1, AC-5, AC-11 | |
| `TestGNMIRuntimeCheck`, `TestGNMISubscriberDropped` | `internal/component/gnmi/doctor_test.go` | AC-5, AC-16 | |
| `TestGNMITLSMaterial`, `TestGNMITLSMaterialCheckRegistered` | `internal/component/gnmi/doctor_test.go` | AC-10; landed in 537b3a87f9 | |
| `TestGNMIBindsEveryListener` | `cmd/ze/hub/service_gnmi_test.go` | AC-9 (the refusal's own tests, `TestGNMIUnreadableCertNotStarted` and `TestGNMIReadableCertStarted`, are child 11 AC-7's, landed in 537b3a87f9) | |
| `TestSSHRuntimeCheck`, `TestSSHPublicKeyTreeValidator`, `TestSSHPublicKeysDoctorCheck` | `internal/component/ssh/doctor_runtime_test.go` | AC-8, AC-11 to AC-13 | |
| `TestSessionTokenFailureIsDefectPath` | `internal/component/web/auth_test.go` | AC-14 | |
| `TestAddEntryRefusesFailedField` | `internal/component/web/handler_config_entry_test.go` | AC-15 | |
| `TestExporterServingState` | `internal/component/telemetry/exporter/doctor_test.go` | AC-7 | |
| `TestDeprecatedTelemetryAliasesRemoved` | `internal/component/telemetry/exporter/server_test.go` | AC-21: a config writing `telemetry prometheus prefix` is refused at parse naming the leaf, and `extractTelemetryConfig` reads the `netdata` values only | |
| `TestCollectorFailureRecord` | `internal/component/telemetry/collector/collector_test.go` | AC-20 | |
| `TestCommitWarningsCarryCodes` | `internal/component/api/config_session_test.go` | AC-22 | |
| `TestAPISurfaceCodesHaveMetadata` | `internal/core/diagnostic/codes_test.go` | AC-30 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| login failures (AC-11) | 0 upward | 0: no row | N/A | 1: row |
| collector failures in a row (AC-20) | 0 upward | 0 after a success: no row | N/A | 1: row |
| gNMI dropped per subscriber (AC-16) | 0 upward | 0: no row | N/A | 1: row |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-mgmt-listener-not-serving` | `test/plugin/show-doctor-mgmt-listener-not-serving.ci` | MCP port held by another socket; the row names surface, address and cause | |
| `show-doctor-mgmt-auth-failed` | `test/plugin/show-doctor-mgmt-auth-failed.ci` | two refused SSH logins; the row names count and user | |
| `ssh-public-key-invalid-refused` | `test/plugin/ssh-public-key-invalid-refused.ci` | commit refused naming user and key | |
| `doctor-ssh-public-key-invalid` | `test/ui/doctor-ssh-public-key-invalid.ci` | local doctor over a stored config | |
| `gnmi-multiple-listeners` | `test/plugin/gnmi-multiple-listeners.ci` | a client on the second listener | |
| `gnmi-tls-unreadable-cert-not-started` | `test/plugin/gnmi-tls-unreadable-cert-not-started.ci` | no plaintext gNMI; child 11 AC-7's test, landed in 537b3a87f9, cited here because AC-10 reports the same cause | |
| `config-add-entry-field-refused` | `test/web/config-add-entry-field-refused.wb` | the form shows the field error, no entry in the draft | |
| `show-doctor-lg-backend-unavailable` | `test/plugin/show-doctor-lg-backend-unavailable.ci` | LG enabled with no `bgp-rib` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no protocol message changes: doctor reads state, the gNMI change binds more addresses (the plaintext refusal is child 11 AC-7, landed in 537b3a87f9), and the gNMI messages are unchanged (`ai/rules/interop-and-goal-validation.md`) | |

## Files to Modify
- `internal/core/diagnostic/codes.go` - every new code and its metadata
- `internal/component/web/server.go`, `internal/component/web/auth.go`, `internal/component/web/handler_config_entry.go`, `internal/component/web/handler_config_commit.go`, `internal/component/web/register.go`, and the web files of the Not-problem-class sites (named per site by `./le arch doctor-coverage report`)
- `internal/component/lg/server.go`, `internal/component/lg/handler_api.go`, `internal/component/lg/register.go`, and the LG files of the Not-problem-class sites
- `internal/component/mcp/doctor.go` (registration beside it), and the MCP files of AC-28
- `internal/component/gnmi/server.go`, `internal/component/gnmi/subscribe.go`, and the gNMI file of AC-26
- `internal/component/api/grpc/server.go`, `internal/component/api/rest/server.go`, `internal/component/api/config_session.go`
- `internal/component/telemetry/exporter/exporter.go`, `internal/component/telemetry/collector/collector.go`
- `internal/component/telemetry/exporter/server.go`, `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang`, `internal/component/telemetry/exporter/server_test.go`, `docs/guide/monitoring.md` (AC-21, the aliases deleted and their users converted)
- `internal/component/gnmi/doctor.go`, `internal/component/gnmi/doctor_test.go` (the gNMI runtime check beside the landed `gnmi-tls-material`)
- `internal/component/ssh/pubkey.go`, `internal/component/ssh/passwordauth.go`, `internal/component/ssh/ssh.go`, `internal/component/ssh/listener_start.go`, `internal/component/ssh/doctor.go`, `internal/component/ssh/register.go`, `internal/component/ssh/yang/ze-ssh-conf.yang` (description)
- `cmd/ze/hub/service_gnmi.go` and `cmd/ze/hub/gnmi_infra.go` (every listener, reload migration; the TLS refusal is child 11 AC-7, landed in 537b3a87f9); `cmd/ze/hub/service_mcp.go`, `cmd/ze/hub/service_web.go`, `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/service_ssh.go` (state setters only, at spec 11's decision points)
- `docs/architecture/doctor-and-health-checks.md`, `docs/guide/gnmi.md`, `docs/guide/web-interface.md`, `docs/guide/looking-glass.md`, `docs/guide/api.md`, `docs/guide/monitoring.md`, `docs/guide/authentication.md`, `docs/guide/health-checks.md`, `docs/architecture/web-interface.md`, `docs/architecture/api/architecture.md`

## Files to Create
- `internal/component/web/doctor_runtime.go`, `internal/component/lg/doctor_runtime.go`, `internal/component/mcp/doctor_runtime.go`, `internal/component/api/rest/doctor_runtime.go`, `internal/component/api/grpc/doctor_runtime.go`, `internal/component/ssh/doctor_runtime.go`, `internal/component/telemetry/exporter/doctor_runtime.go`, each with its test
- `internal/component/web/handler_config_entry_test.go`
- `internal/component/telemetry/exporter/doctor_test.go`
- `internal/core/diagnostic/codes_test.go`
- the eight functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | no config or RPC added; three deprecated telemetry nodes deleted (AC-21); one SSH description rewritten |
| YANG validation constraints | Yes | the public-key validator (AC-13) |
| YANG custom validators | Yes | the same validator, through `config.RegisterTreeValidator`, so start, reload, validate and commit agree |
| CLI commands/flags | No | `show doctor` exists |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | the eight functional tests |
| Pipe completeness | N-A | `show doctor` unchanged |
| Env var registration | No | `ze.gnmi.tls.cert` and `.key` are registered |
| Doctor check for runtime dependencies | Yes | this spec |
| Prometheus counters/metrics | No | counts live in the surface state; no metric added |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | gNMI serves every listener: `docs/guide/gnmi.md` (its TLS refusal section landed with child 11 AC-7 in 537b3a87f9) |
| 2 | Config syntax changed? | Yes | the three deprecated `telemetry prometheus` aliases are removed: `docs/guide/monitoring.md` (AC-21); the SSH key refusal is a validation |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | `show doctor` format unchanged |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md`, `docs/guide/looking-glass.md`, `docs/guide/api.md`, `docs/guide/monitoring.md`, `docs/guide/authentication.md`, `docs/guide/health-checks.md` list the doctor codes |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | reporting only |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (surface runtime checks, serving state); `docs/architecture/web-interface.md`; `docs/architecture/api/architecture.md` ("gNMI Transport": listeners and TLS) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per phase with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-4-api-surfaces.md`. Declared by the changed files' `// Design:` headers: `docs/architecture/system-architecture.md` (the SSH key verifier and checks: updated where it describes public-key login), `docs/architecture/mcp/overview.md` (MCP serving state: updated), `docs/architecture/core-design.md` (the report bus and the exporter: updated for the LG code and the exporter state), `docs/features/ai-first.md` (the code registry: unaffected, no claim about doctor coverage changes), `ai/rules/plugins.md` (the gNMI compile-out seam: unaffected, the seam keeps its shape) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | sample `show doctor` output in the guide pages is refreshed where a row changes |

## Implementation Steps

1. **Phase: Wiring** -- codes and metadata, empty runtime checks per surface, the functional tests failing
2. **Phase: Shared mechanisms** -- serving state, login-failure record, LG re-code
3. **Phase: Per sub-area checks and defect fixes**
4. **Phase: Lines carry codes, and docs**

### Implementation Phases

One agent per phase, about 60 tool calls each. Agents edit only; the main thread runs every test.

| Phase | Owns | ACs | Depends on | Runs with |
|-------|------|-----|------------|-----------|
| P0 wiring | `internal/core/diagnostic/codes.go` (every code up front), empty runtime checks registered in each surface package, the functional test drafts | AC-30 | - | - |
| P1 shared state | the serving state and its setter in each surface package; the login-failure record; the LG re-code | AC-1 to AC-4 | P0 | - |
| P2 listeners and gNMI | the runtime listener rows of every surface; the exporter and SSH halves; gNMI every listener (the TLS refusal is child 11 AC-7 and the AC-10 check, both landed in 537b3a87f9); the AC-6 test against spec 11's split | AC-5 to AC-10 | P1, spec 11's `listeners` split for AC-6 | ∥ P3 |
| P3 auth and SSH key | auth and user-list rows on SSH, web, REST, gRPC, gNMI, MCP; the public-key verifier and check; AC-14 | AC-11 to AC-14 | P1 | ∥ P2 |
| P4 web, gNMI, LG | add-entry refusal; gNMI subscriber drops; LG backend, AS and BMP rows | AC-15 to AC-19 | P1 | ∥ P5 |
| P5 telemetry and API | collector record; the deprecated aliases deleted and their users converted; commit warning codes | AC-20 to AC-22 | P0, the config-yang child's codes (the draft-cleanup code is not named there yet, AC-22) | ∥ P4 |
| P6 sweep and docs | the Not-problem-class lines; the line sweep; every page in the Documentation checklist; the AC-31 report | AC-23 to AC-29, AC-31, AC-32 | P2 to P5 | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC at file and symbol; `./le arch doctor-coverage report` shows the area covered |
| Correctness | no runtime check reads `Tree`; a served address never reads as unserved; the gNMI check reports the same refusal child 11 AC-7 enforces (landed in 537b3a87f9), through the same loader |
| Naming | codes kebab-case with the `doctor-` prefix; CODES.md names, except the three Design Insights explains |
| Data flow | the LG asks the registry, never a plugin; the hub writes state only through the surface's setter |
| Rule: user-facing-errors | every fix is a command alone in `Fix` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every new code explained | `TestAPISurfaceCodesHaveMetadata`; `ze show diagnostic <code>` |
| area covered | `./le arch doctor-coverage report` |
| tests | the unit and functional tests above pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | no row or log line prints a password, token, TLS key or public key body |
| Plaintext | gNMI never serves without the TLS its config names: child 11 AC-7, landed in 537b3a87f9; AC-10 reports the cause |
| Usernames and addresses | login-failure rows appear only in `show doctor`, behind authz |
| Resource use | records keep counts and the last occurrence; no per-attempt growth |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A class has no readable state | back to the inventory row; name the record the owner must keep |
| A fix names nothing that exists | DESIGN: verify against the command registry and YANG |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `doctor-gnmi-listener-ignored` (CODES.md) is not registered: AC-9 fixes the defect, so the class no longer exists
- `doctor-gnmi-tls-material` keeps its CODES.md name, because child 11 reuses it for the refusal it added (child 11 AC-7, landed in 537b3a87f9 with this code and the AC-10 check). Code split with spec 11: this spec registers `doctor-mgmt-listener-not-serving`, `doctor-mgmt-user-list-unreadable` and `doctor-gnmi-tls-material`, their metadata and the checks that declare them; spec 11's hub sites log and record under these exact names. Unlike `DoctorCertPair`, which web, gRPC and MCP use, the gNMI check also reports an unreadable file, the case that served plaintext before 537b3a87f9
- `doctor-mgmt-request-refused` (CODES.md) is not registered: a refusal its caller already sees reports nothing wrong, so those lines drop to Info (AC-27)
- The web session-token row is a defect path on Go 1.27 (A-3), so it carries `doctor-internal-defect` rather than a host code
- `doctor-telemetry-config-deprecated` (CODES.md) is not registered: the deprecated telemetry aliases are a compatibility path for a form no release shipped, so AC-21 deletes them under the pre-release rule (`ai/META-DECISIONS.md`, "Ze is pre-release") and the class no longer exists

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-10 | SSH user `public-keys` entry parsed only at login |
| `silent-fall-through.md` | 2026-11-10 | web add-entry handler drops a field that fails to set |
| `listener-set-partly-bound-reported-as-applied.md` | 2026-11-10 | gNMI binds only the first configured listener |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Serving state kept in each surface package, read by a runtime check there | a core registry of listeners; scraping the log | the check belongs to the package that owns the state and compiles out with it; a log scrape misses stderr-only failures |
| gNMI binds every listener | refuse a second `server` entry at verify | the YANG list promises several, and every sibling surface binds them all |
| gNMI refuses to serve on unreadable TLS (child 11 AC-7, landed in 537b3a87f9; this spec only reports it) | warn and serve plaintext (before that commit) | a listener without its configured security is a security defect (door) |
| SSH key refused at verify and reported by doctor | doctor only | the key can never authenticate; exact-or-reject; doctor still catches a stored config |
| SSH key refusal through `config.RegisterTreeValidator` | an `InProcessConfigVerifier` | SSH is a component with no plugin registration; the tree validator is the component mechanism and runs on every path `ValidateCustomSections` serves |
| Deprecated telemetry aliases deleted (AC-21) | report them in doctor | the pre-release rule converts a form no release shipped directly, with no compatibility path |
| One `doctor-mgmt-auth-failed` row per surface with counts | one row per user and source | bounded output under attack (R-1); `show audit` holds every attempt |

## Known Limitations
- The boot decision for each surface and the `listeners` split are spec 11's; the plugin-infra codes and their checks are spec 12's.

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
- [ ] AC-1..AC-32 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests under `./le job run` for each package in Files to Modify (with `ze_ssh`, `ze_web`, `ze_lg`, `ze_mcp`, `ze_rest`, `ze_grpc`, `ze_gnmi`, `ze_telemetry` as each needs), the named `.ci` files through `./le test functional plugin` and `./le test functional ui`, and the `.wb` file through `./le test functional web`, each selecting those tests; no agent runs any `./le verify` command
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
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` only, in the same `./le commit create` script
