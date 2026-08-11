# Spec: fixit-mgmt-listener-auth-guard

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-08-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/evidence.md` - a guard must fail closed
4. `cmd/ze/hub/main.go` (boot order), `cmd/ze/hub/service_registry.go` (factory registry), `cmd/ze/hub/service_gnmi.go` (gNMI resolution to hoist)
5. Source files in Current Behavior below

## Task

Every management listener except the API server can be published unauthenticated on a
routable address by a config file or env var that reaches `ze start` without passing
through the config editor. Establish a single startup-time fail-closed guard that refuses
to serve any management listener bound non-loopback without authentication, mirroring the
API server's existing precedent (`cmd/ze/hub/main.go`, helper `apiHasNonLoopback`
at `cmd/ze/hub/api.go`). Verified by the 2026-07-16 audit (verifier V1); every
citation below re-verified against the working tree on 2026-07-16 during design.

1. **[BLOCKER] gNMI unauthenticated read+write on `0.0.0.0`.** Once enabled (env
   `ze.gnmi.enabled` at `cmd/ze/hub/service_gnmi.go`, or YANG block at `:60-61`),
   with no listen override the bind defaults to `0.0.0.0:9339`, the token flows
   through with no guard, auth interceptors are installed only when a token is
   set (`internal/component/gnmi/server.go`), and `checkAuth` short-circuits to
   allow when the token is empty. `Set` is a full config mutation
   (`internal/component/gnmi/set.go`, session entered at `:41`). No
   `GNMIListenConfig.Validate` exists (`internal/component/config/loader_extract.go`
   defines the struct and extractor only), so neither `ze config validate` nor `ze doctor`
   can flag the exposure, and there is no boot-path refusal.
2. **[HIGH] MCP fail-closed guard does not run at daemon startup.** The guard exists --
   `MCPListenConfig.Validate` rejects `BindRemote && auth-mode in {"", none}`
   (`internal/component/config/loader_extract.go`, error var at `:19`) and the
   loopback clamp is skipped for bind-remote -- but its only callers are
   `ValidateSemantics` (`internal/component/config/validate_semantic.go`, reached by
   doctor via `internal/component/doctor/checks_config.go`) and `ze config
   validate` (`internal/component/config/cli/cmd_validate.go`). `ze start` runs
   neither: `LoadConfig` does no semantic validation
   (`internal/component/config/loader.go`) and `cmd/ze/hub/main.go` feeds
   `ExtractMCPConfig`'s result straight into `mcpServiceDeps` and on to
   `buildMCPService` (`cmd/ze/hub/service_mcp.go`). A `bind-remote true; auth-mode
   none` config on disk, or a bare `ze.mcp.listen=<routable>` env var with no token
   (`cmd/ze/hub/main.go`), boots with the accept-all `noneAuthenticator`
   (`internal/component/mcp/bearer.go` default branch; accept-all impl `:37-43`).
3. **[HIGH] `--insecure-web` loopback clamp is bypassable via the `ze.web.insecure` env
   var.** The flag and the YANG path both clamp to loopback
   (`cmd/ze/ze_core_start.go` and `:148-150`;
   `internal/component/config/loader_extract.go`), but `ze.web.insecure=true`
   (`cmd/ze/hub/main.go`) does not rewrite the address, so with `ze.web.listen` at
   its `0.0.0.0:3443` default (`cmd/ze/hub/service_web.go`, again `:240-242`) the
   unauthenticated `InsecureMiddleware` (`internal/component/web/auth.go`, selected
   at `cmd/ze/hub/service_web.go`) serves on all interfaces.
4. **[MEDIUM] Looking Glass is unauthenticated with TLS optional.** The LG registers every
   route on a bare mux with no auth middleware
   (`internal/component/lg/server.go`), TLS applies only when `LGTLS` is set
   (`cmd/ze/hub/service_lg.go`), and it exposes `routes/filtered` and
   `routes/noexport` (`internal/component/lg/server.go`). Default LG TLS on and
   offer an optional auth gate.

5. **[RESOLVED 2026-07-29 -- FIXED, not inherited] MCP auth settings were silently
   discarded when the block was not `enabled`, so a CLI/env-started listener ran
   accept-all.** It was fixed in `spec-mcp2026-1-stateless-core` rather than handed to
   this spec. `extractMCPBlock` now reports "block exists" and "block asks for a
   listener" separately, and `ExtractMCPSettings` returns auth and TLS for any block.
   And `cmd/ze/hub/main.go` takes addresses only from a listener-asking config, and it
   takes settings from any block. Four unit tests in
   `internal/component/config/loader_extract_test.go` are mutation-verified, and so is
   `test/plugin/mcp-cli-listener-honors-config-auth.ci`, which returns `status=200` with
   the full tool list when the fix is reverted.

   **This spec inherits nothing from it.** The row is kept for two reasons. The
   boot-path guard this spec builds must still cover the case. And the original
   analysis below explains why the exposure was bounded.

   Original analysis, added 2026-07-29 from `spec-mcp2026-1-stateless-core`. That spec
   found it with a stronger `test/plugin/task-identity-scope.ci`. The old test asserted
   per-principal task isolation while Alice and Bob were in fact the same anonymous
   principal, so its title claim was untested. The stronger version mutation-fails when
   it is reverted to the old config, which is what proved both the vacuity and the
   defect.

   `ExtractMCPConfig` returns `ok=false` unless the block sets `enabled true`
   (`internal/component/config/loader_extract.go`) **and** carries a server with a
   non-empty port. `cmd/ze/hub/main.go` then skips the whole
   `if mcpCfgOK` block, and `cmd/ze/hub/service_mcp.go` skips
   `mcpConfigToStreamable` entirely, so `AuthMode`, `BearerList` and `OAuth` stay zero.
   `NewStreamable`'s mode inference (`internal/component/mcp/streamable.go`)
   then selects `AuthNone`, and `noneAuthenticator` accepts every request with a zero
   `Identity` (`internal/component/mcp/bearer.go`). So an operator who writes
   `environment { mcp { auth-mode bearer; token secret; } }` and starts
   `ze --mcp 9718 <config>` gets an **unauthenticated** listener, and they believe they
   configured bearer auth.

   **Bounded, and the bound is why this is MEDIUM rather than HIGH:**
   `mcpListenerAuthenticated` (`cmd/ze/hub/mgmt_guard.go`) uses
   `token != ""` when `cfgOK` is false. It therefore reports *unauthenticated*
   correctly, and the non-loopback guard this spec builds still refuses to publish it.
   The exposure is local-only. It is still wrong: a config that asks for authentication
   must not silently produce none (`ai/rules/protocol.md`), and the operator gets
   no diagnostic at all.

   The fix belongs with the unifying guard below. The `enabled` gate answers "start a
   listener from config", which is a different question from "how does this service
   authenticate". So either apply the auth settings independently of that gate, or
   refuse to start when auth settings were parsed and then discarded.

The unifying fix: a boot-time guard, run unconditionally at ONE point in
`cmd/ze/hub/main.go` after all listener resolution and before any management bind, that
inspects every management listener's (address, auth-mode/token) and refuses to start
(hard-fail, per the API precedent -- Thomas confirmed this default 2026-07-16) when a
listener is non-loopback and unauthenticated. Add gNMI semantic validation and the missing
gNMI doctor entries so `ze doctor` and `ze config validate` also flag the exposure.

Skeleton claim corrected during design: `internal/component/doctor/checks_listener.go`
(`collectHardcodedListeners`) does NOT omit MCP -- it probes MCP at `:83-87` -- and it is
only the fallback used when schema discovery fails. The primary path
(`collectSchemaListeners`, `:48-73`) already schema-discovers gNMI via the `ze:listener`
mark (`internal/component/gnmi/yang/ze-gnmi-conf.yang`). The real doctor gaps are:
(a) `internal/component/config/listener_defaults.go` registers no "gnmi" default, so
a gNMI block enabled with no explicit `server` entry produces no probe endpoint
(`CollectListeners` skips default-only services per
`internal/component/config/listener.go`; defaults are filled only for registered
names, `:351-404`); (b) all these checks are bind-availability probes
(`doctor-listen-unavailable`), not auth-exposure checks; (c) gNMI has no `Validate`, so
`ValidateSemantics` says nothing about it, while MCP exposure is already flagged there.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - guards fail closed; drive the guard's test from its entry point
  → Constraint: a zero value (empty token / unset auth-mode) must never read as a valid answer.
  → Constraint: an unparseable listen host must classify as non-loopback (both existing helpers already do: `cmd/ze/hub/api.go`, `internal/component/config/loader_extract.go`).
- [ ] `ai/rules/config.md` - env var vs YANG; how listen/auth settings reach the daemon
  → Constraint: the guard must cover the env-var path and the on-disk-config path, not only the editor path.
- [ ] `ai/rules/config.md` - naming for the new LG leaves
  → Constraint: env leaf segment matches YANG leaf exactly: `looking-glass { token }` pairs with `ze.looking-glass.token`; register in `internal/component/config/environment.go` like `ze.gnmi.token` (`:84`, Secret: true).
- [ ] `ai/rules/repo-maintenance.md` - doctor check ownership and code registration
  → Constraint: new diagnostic codes register in `internal/core/diagnostic/codes.go` (precedent: `config-mcp-invalid` at `:64`); config-semantic checks flow through `ValidateSemantics`, which doctor already calls.
- [ ] `ai/rules/platform-linux.md` - boot-refusal tests that only validate config/refusal run natively; only kernel-touching tests need `option=needs-linux`
  → Constraint: the QEMU appliance boot test exercises the guard on the gokrazy image path (Linux boot), see Implementation Phases.
- [ ] `ai/rules/plugins.md` - compile-out seams; resolution stays always-on, only building is gated
  → Constraint: guard code is always-on hub code; it must not import gnmi/mcp/web/lg packages. The MCP precedent (resolution always-on, gated factory consumes plain values) is documented at `cmd/ze/hub/main.go` and is the model for hoisting gNMI resolution.

**Key insights:**
- The API server already does this correctly at `cmd/ze/hub/main.go`; the fix generalizes that check to every management listener, moved to a single earlier boot point.
- Both existing non-loopback classifiers fail closed on unparseable hosts; the shared helper must preserve that exact semantic.
- Auth mode is fixed when a service is built; a SIGHUP listener migration can move a running listener to a new address without any auth re-check (`cmd/ze/hub/listener_migrate.go`), so the guard classification must also gate `ReloadListeners`.

## Current Behavior (MANDATORY)

**Source files read:** (all re-verified 2026-07-16 against the working tree)
- [ ] `cmd/ze/hub/main.go` - boot order in `runYANGConfig`: env resolution for web/LG/MCP, YANG fill-in, engine start, plugin server start, SSH standalone bind, `buildServices` builds AND starts web/LG/MCP, API env+YANG resolution, API auth-mode report, `apiHasNonLoopback` refusal, REST/gRPC bind, gNMI seam build. MCP default loopback bind on env-enable.
  → Constraint: today no single point sees all five surfaces' (address, auth) pairs; web/LG/MCP resolve before `buildServices`, API and gNMI resolve after web/LG/MCP have already bound.
- [ ] `cmd/ze/hub/service_registry.go` - construction registry: `registerService`, `buildServices` iterates factories; `ServiceDeps` carries resolved plain values
  → Constraint: factories return nil for not-configured; a factory absent (build tag off) means the service can never bind. The guard can consult registered factory names to skip compiled-out services.
- [ ] `cmd/ze/hub/service_gnmi.go` - gNMI enable/bind/token resolution lives INSIDE the ze_gnmi-gated builder, then binds
  → Constraint: this resolution must be hoisted to always-on main.go (MCP precedent) so the guard can see it before anything binds. `ExtractGNMIConfig` is already always-on (`internal/component/config/loader_extract.go`); the `ze.gnmi.*` env keys are registered always-on (`internal/component/config/environment.go`).
- [ ] `cmd/ze/hub/gnmi_infra.go` - `gnmiBuild` seam hook nil when ze_gnmi absent
- [ ] `internal/component/gnmi/server.go` - interceptors only when token set; `checkAuth` allows on empty token
- [ ] `internal/component/config/loader_extract.go` - `MCPListenConfig.Validate` (`:197-258`, bind-remote rule `:204-206`), MCP loopback clamp unless bind-remote, web insecure clamp, `AnyListenerNonLoopback` fail-closed classifier, `GNMIListenConfig` with no Validate, `ExtractLGConfig` TLS default false, `ExtractAPIConfig`
- [ ] `internal/component/config/loader.go` - `LoadConfig` does no semantic validation
- [ ] `internal/component/config/validate_semantic.go` - `ValidateSemantics` runs MCP Validate only; no gNMI entry
- [ ] `internal/component/config/cli/cmd_validate.go` - `ze config validate` MCP semantic check; no gNMI entry
- [ ] `internal/component/mcp/bearer.go` - `noneAuthenticator` accept-all, selected as default fall-through
- [ ] `cmd/ze/hub/service_web.go` - default `0.0.0.0:3443`; the `!insecureWeb && no-users` fail-closed disable; insecure warning branch; `InsecureMiddleware` selection. SHARED FILE with the in-flight bcrypt spec -- see R-3.
- [ ] `internal/component/web/auth.go` - `InsecureMiddleware` injects username without auth
- [ ] `cmd/ze/hub/service_lg.go`, `internal/component/lg/server.go` - LG factory optional TLS (`service_lg.go`), routes on bare mux with no auth (`server.go`), all-or-nothing multi-bind (`server.go`)
- [ ] `internal/component/lg/yang/ze-lg-conf.yang` - `tls` leaf default false
- [ ] `internal/component/doctor/checks_listener.go` - schema-discovered bind probes, hardcoded fallback INCLUDING mcp (`:75-108`, mcp at `:83-87`), no gnmi in the fallback
- [ ] `internal/component/config/listener_defaults.go` - builtin defaults registry, no "gnmi" entry
- [ ] `internal/component/config/listener.go` - `ze:listener` schema discovery, defaults only for registered names
- [ ] `cmd/ze/hub/listener_migrate.go` - SIGHUP address migration for web/lg/mcp/rest/grpc with no auth re-check
- [ ] `cmd/ze/ze_core_start.go` - `--insecure-web` flag clamps webListenAddr to loopback (`:138-140`; web-only variant `:148-150`)
- [ ] `test/plugin/family-no-plugin-failure.ci` - existing boot-refusal .ci pattern (`expect=exit:code=1` + `expect=stderr:contains=`), per-command `env=` supported (`test/plugin/forward-write-deadline.ci`)
- [ ] `test/ui/doctor-listeners.ci` - existing doctor .ci pattern (`ze doctor --json <config>` + `expect=stdout:contains=<code>`)

**Behavior to preserve:**
- The API server's existing refusal semantics and message ("refusing to start API on non-loopback listener without authentication", `cmd/ze/hub/main.go`); the guard's API row keeps the condition `len(apiUsers) == 0 && apiCfg.Token == ""` exactly.
- gNMI/MCP/LG/web remaining fully usable on loopback, and on a routable address WHEN authenticated.
- Loopback defaults where they already exist (MCP default bind loopback, `cmd/ze/hub/main.go`; MCP YANG clamp `loader_extract.go`; web YANG insecure clamp `:119-127`; `--insecure-web` flag clamp `ze_core_start.go`). These clamps run BEFORE the guard, so a clamped config never presents non-loopback to the guard: no behavior change on those paths.
- Web's no-users fail-closed disable (`service_web.go`) stays; the guard adds refusal only for the insecure+non-loopback combination.
- LG's all-or-nothing bind and reconfigure semantics.
- A looking glass on a box with no blob storage keeps serving. TLS-on-by-default is a hardening default, not an operator instruction, so it degrades to plaintext with a warning rather than failing the build. An explicit `tls true` is an instruction and is still refused when it cannot be honored (`ai/rules/protocol.md`).

**Behavior to change:**
- Add the boot-time refusal for non-loopback + unauthenticated management listeners (gNMI, MCP, web-insecure, API kept as-is via the shared guard).
- Run `MCPListenConfig.Validate` (and the new `GNMIListenConfig.Validate`) on the `ze start` path so config-level inconsistencies hard-fail at boot with the existing precise messages.
- Default LG TLS on (YANG default flip + extraction default + env-enable path default); add optional LG bearer-token auth gate.
- Add "gnmi" to the doctor listener defaults and to the hardcoded fallback; add gNMI semantic validation so doctor and `ze config validate` flag the exposure.
- Refuse SIGHUP listener migrations that would move an unauthenticated listener non-loopback.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start` reaches `runYANGConfig` (`cmd/ze/hub/main.go`) with the parsed tree; env vars are read via `internal/core/env`; CLI flags arrive as parameters (`webListenAddr`, `insecureWeb`, `mcpAddr`, `mcpToken`).

### Transformation Path
1. `LoadConfig` parses the tree; no semantic validation (`loader.go`).
2. Env/CLI/YANG resolution produces plain values per service: web (`main.go`, `:350-356`), LG, MCP. NEW: hoist API resolution and gNMI resolution (from `service_gnmi.go`, made always-on per the MCP precedent at `main.go`) into this same block.
3. NEW single guard point: build the `[]mgmtListener` declaration slice (service name, resolved addrs, authenticated flag, remedy text) for every surface whose factory/seam is compiled in, then call `checkMgmtListeners` once -- after resolution completes and BEFORE `eng.Start` (`main.go`), which precedes every management bind (SSH `:737`, buildServices `:746`, REST/gRPC `:872-910`, gNMI seam `:916-924`). Also call `mcpCfg.Validate()` and the new `gnmiCfg.Validate()` here when the YANG blocks are present.
4. On any non-loopback + unauthenticated listener: print one error per offending listener naming the service, the offending address, and the remedy; return exit code 1. Nothing has bound yet.
5. On pass: boot proceeds exactly as today; the existing API refusal at `:862-866` is subsumed by the guard (single implementation, same message).
6. SIGHUP reload: `ListenerMigrator.ReloadListeners` (`listener_migrate.go`) consults the same classification helper before applying a change; a migration that would take a service built without authentication to a non-loopback address is refused with an error (daemon keeps running on the old addresses).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config/env ↔ hub | extracted listen address + auth mode per service, all always-on plain values (MCP precedent `main.go`) | [ ] |
| Hub ↔ each service | resolution hoisted above the guard; builders receive already-guarded values | [ ] |
| Hub ↔ compile-out seams | declarations emitted only when the factory/seam exists (`serviceFactories` names, `gnmiBuild`/`restBuild`/`grpcBuild` non-nil) | [ ] |
| Doctor ↔ config | gNMI Validate wired into `ValidateSemantics`; "gnmi" listener default registered | [ ] |
| Reload ↔ guard | `ReloadListeners` reuses the classification helper | [ ] |

### Integration Points
- `cmd/ze/hub/main.go` `runYANGConfig` (guard call site, hoisted resolutions)
- `cmd/ze/hub/api.go` `apiHasNonLoopback` (folded into the shared helper)
- `internal/component/config/loader_extract.go` (`GNMIListenConfig.Validate`, LG extraction TLS default)
- `internal/component/config/validate_semantic.go`, `internal/component/config/cli/cmd_validate.go` (gNMI semantic wiring)
- `internal/component/config/listener_defaults.go`, `internal/component/doctor/checks_listener.go` (doctor coverage)
- `cmd/ze/hub/listener_migrate.go` (reload gate)
- `internal/component/lg/server.go` (optional token middleware), `internal/component/lg/yang/ze-lg-conf.yang` (tls default, token leaf)

### Architectural Verification
- [ ] No bypassed layers (guard runs on the same boot path that builds the listeners, before any bind)
- [ ] No duplicated functionality (one classifier + one guard; `apiHasNonLoopback` folded in, not copied; `MCPListenConfig.AnyListenerNonLoopback` semantics reused)
- [ ] Registration over hardcoding -- the guard iterates a declaration slice; each surface appends its declaration next to its existing resolution code; the guard function itself names no service. A compile-time init() registry was considered and rejected: declarations are boot-resolved runtime values with a single collection point (`runYANGConfig`), and gated factories run only after bind decisions, too late to declare (`ai/rules/plugins.md` intent preserved: guard is generic; services declare)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Hard-fail (not warn-and-clamp) is the desired posture, matching the API precedent | `cmd/ze/hub/main.go`; Thomas confirmed the API-precedent default (2026-07-16) | Operator upgrade breakage | user confirmation captured 2026-07-16; QEMU boot test exercises the refusal and the authenticated pass | confirmed |
| A-2 | Every management listener's address+auth is knowable at one boot point | Builders enumerated: web `service_web.go`, LG `service_lg.go`, MCP `service_mcp.go`, REST/gRPC seams `main.go`, gNMI seam `main.go` impl `service_gnmi.go`. Web/LG/MCP resolve at `main.go` already; API resolution is pure env+tree reads and hoists cleanly; gNMI resolution (`service_gnmi.go`) uses only always-on inputs (`ExtractGNMIConfig`, `ze.gnmi.*` env) and hoists per the MCP precedent (`main.go`) | Guard cannot see a listener | source enumeration above (2026-07-16); wiring test drives each surface through the guard | confirmed |
| A-3 | Adding gNMI to the doctor listener set has no schema side effects | `checks_listener.go` is a static fallback slice builder; the schema path already discovers gNMI (`ze-gnmi-conf.yang`); only `listener_defaults.go` (name-keyed map, `listener.go`) and the fallback need entries | Doctor code drift | files read 2026-07-16; `TestDoctorCoverageCodesRegistered` after implementation | confirmed |
| A-4 | Out-of-scope listeners are safe to exclude: SSH (authenticated by protocol + AAA, `main.go`), plugin hub (secrets enforced, min length 32, `loader_extract.go`), telemetry/Prometheus (metrics read-only, default loopback), managed server (hub secrets) | cited producers read 2026-07-16; the Prometheus default re-verified 2026-08-10 against its real producer `extractTelemetryConfig` (`internal/component/telemetry/exporter/server.go`), which seeds every endpoint with `defaultTelemetryHost` = `127.0.0.1` | A surface is exposed unauthenticated | re-audit rows in Known Limitations; doctor bind probes still cover them | confirmed, with the residual case below |
| A-5 | The SIGHUP migration path can move a running listener's address without auth re-check, so a boot-only guard fails open on reload | `listener_migrate.go` extracts new addrs and reconfigures; auth mode fixed at build time (e.g. web `authWrap` chosen once, `service_web.go`) | Guard bypassed post-boot | producer read 2026-07-16; AC-7 test | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hard-fail bricks an existing deployment that relied on unauthenticated non-loopback gNMI/web-insecure/MCP-env | boot refusal on upgrade | Clear, actionable error naming the listener + remedy (set token / bind loopback / enable auth); release-notes entry drafted (Quality Gate); QEMU boot test covers both refusal and remedied boot |
| R-2 | Default LG TLS on breaks an existing plaintext LG consumer (birdwatcher API clients) | LG clients fail TLS handshake | Documented opt-out `environment { looking-glass { tls false } }` stays honored; release note; loopback plaintext unaffected only via explicit opt-out (TLS default applies to all binds -- simpler rule, stated in release note) |
| R-3 | Shared-file collision: the in-flight bcrypt spec also edits `cmd/ze/hub/service_web.go` (and both touch `cmd/ze/hub/main.go` vicinity) | merge conflict / double edit at implementation time | Coordinate at implementation: this spec's web changes are confined to the guard declaration (main.go) and do not alter `startWebServer` auth internals; rebase order agreed with Thomas before starting |
| R-4 | Hoisting gNMI/API resolution reorders startup log lines (auth-mode report currently prints at `main.go`) | test expectations on stderr ordering | Keep message text identical; only position moves; adjust any .ci expectations that assert ordering |
| R-5 | Env-only exposure is invisible to `ze doctor` (doctor validates a config file, not the daemon's env) | doctor green while env-booted daemon would refuse | Accepted: the boot guard is the authoritative gate; doctor covers the config-file paths; documented in Known Limitations |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| config binds gNMI `0.0.0.0` with no token | -> | boot guard refuses to start | `test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci` |
| `ze.mcp.listen=0.0.0.0:8080` env, no token | -> | boot guard refuses to start | `test/plugin/mgmt-guard-mcp-env-nonloopback-refused.ci` |
| `ze.web.insecure=true` with default `0.0.0.0` listen | -> | boot guard refuses to start | `test/plugin/mgmt-guard-web-insecure-env-refused.ci` |
| config `mcp { bind-remote true; auth-mode none }` | -> | boot-path `Validate` hard-fails with the existing message | `test/plugin/mgmt-guard-mcp-bind-remote-none-refused.ci` |
| loopback gNMI/MCP/web without auth | -> | starts normally | `test/plugin/mgmt-guard-loopback-allowed.ci` |
| gNMI non-loopback WITH token | -> | starts normally | `test/plugin/mgmt-guard-gnmi-token-allowed.ci` |
| `ze doctor --json` on exposing config | -> | `config-gnmi-invalid` / `config-mcp-invalid` emitted | `test/ui/doctor-gnmi-mcp-exposure.ci` |
| SIGHUP moves unauth listener non-loopback | -> | migration refused, daemon keeps old addrs | `test/reload/mgmt-guard-reload-refuses-nonloopback.ci` |
| `ze.gnmi.enabled` + dormant block naming a loopback `server` | -> | `resolveGNMIListeners` binds that address, not `0.0.0.0:9339` | `test/plugin/mgmt-guard-gnmi-env-started-address-survives.ci` |
| `ze.gnmi.enabled` + dormant block naming a `token` | -> | the guard reads that token, boot proceeds | `test/plugin/mgmt-guard-gnmi-env-started-token-survives.ci` |
| `ze.looking-glass.enabled` + dormant block naming a loopback `server` | -> | LG binds that address, not the env var's `0.0.0.0:8443` default | `test/plugin/mgmt-guard-lg-env-started-address-survives.ci` |
| `ze.looking-glass.token` leaf or env var | -> | `ServiceDeps.LGToken` -> `lg.LGConfig.Token` -> the bearer middleware | `test/plugin/lg-token-gate.ci` |
| `ze.web.enabled` + dormant block naming a loopback `server` | -> | web binds that address, not `0.0.0.0:3443` | `test/plugin/mgmt-guard-web-env-started-address-binds.ci` |
| `ze.web.enabled` + dormant block carrying `insecure true` | -> | the leaf is dropped, the daemon WARNs and boots authenticated | `test/plugin/mgmt-guard-web-dormant-insecure-warns.ci` |
| `ze.web.enabled` + `ze.web.insecure` + dormant block naming a routable `server` | -> | the refusal quotes that address | `test/plugin/mgmt-guard-web-env-started-address-survives.ci` |
| `ze.api-server.rest.enabled` + dormant block naming address and token | -> | REST binds that address and authenticates with that token | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | gNMI enabled (env or YANG), non-loopback bind (including the `0.0.0.0:9339` default), no token | Startup hard-fails, exit 1, stderr names gNMI, the offending address, and the remedy (set `ze.gnmi.token` / YANG token, or bind loopback); nothing binds |
| AC-2 | MCP routable listen with no effective auth: (a) YANG `bind-remote true` + `auth-mode none`/unset, (b) `ze.mcp.listen=<routable>` env with no token | (a) hard-fails with the existing `environment.mcp: bind-remote requires auth-mode != none` message at boot; (b) hard-fails via the guard naming MCP |
| AC-3 | `ze.web.insecure=true` while any web listen address is non-loopback | Startup hard-fails naming web-insecure and the remedy (`ze.web.listen=127.0.0.1:<port>` or drop the env var) |
| AC-4 | Any of the above but bound to loopback only | Starts normally (unchanged); guard logs nothing |
| AC-5 | LG enabled with no `tls` leaf and no `ze.looking-glass.tls` env | LG serves TLS (default flipped on); explicit `tls false` still serves plaintext; optional `token` leaf / `ze.looking-glass.token` gates every /api/ and /lg/ route with constant-time bearer auth when set. Certificates need blob storage: an EXPLICIT `tls true` without it is an error, while the inherited default yields to plaintext with a warning naming `ze init`, so a hardening default never removes a working looking glass |
| AC-6 | `ze doctor --json` (and `ze config validate`) on a config exposing gNMI non-loopback without token, or MCP bind-remote without auth | Emits `config-gnmi-invalid` / `config-mcp-invalid` diagnostics; gNMI default endpoint appears in the bind-probe set |
| AC-7 | Running daemon, SIGHUP reload moves a service built without authentication (e.g. insecure web) to a non-loopback address | `ReloadListeners` refuses that service's migration with an error naming it; daemon continues serving on the previous addresses |

AC-7 was added during design (fail-closed corollary of A-5: a boot-only guard fails open
on reload). ~~Flagged for Thomas's confirmation; the rest of the spec stands without it.~~
→ RESOLVED (2026-07-17, AUTONOMOUS DEFAULT): AC-7 stays in this spec (open question 1). A
reload path a boot-only guard cannot see fails open, so the reload gate is in-scope.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMgmtGuardRefusesNonLoopbackUnauth` | `cmd/ze/hub/mgmt_guard_test.go` | AC-1..AC-3 classification + per-service error text |DONE (`TestCheckMgmtListeners`) |
| `TestMgmtGuardAllowsLoopback` | `cmd/ze/hub/mgmt_guard_test.go` | AC-4 |DONE (`TestCheckMgmtListeners`) |
| `TestMgmtGuardAllowsAuthenticatedNonLoopback` | `cmd/ze/hub/mgmt_guard_test.go` | token/users present passes |DONE (`TestCheckMgmtListeners`) |
| `TestMgmtGuardUnparseableHostIsNonLoopback` | `cmd/ze/hub/mgmt_guard_test.go` | fail-closed classifier (hostname, garbage, empty host) |DONE (`TestListenAddrIsNonLoopback`) |
| `TestGNMIListenConfigValidate` | `internal/component/config/loader_extract_test.go` | non-loopback + empty token rejected; loopback + empty token allowed; token present allowed |DONE (`internal/component/config/gnmi_validate_test.go`) |
| `TestValidateSemanticsFlagsGNMI` | `internal/component/config/validate_semantic_test.go` | `config-gnmi-invalid` emitted through the semantic entry point (not only the helper) |DONE (`internal/component/config/validate_semantic_test.go`) |
| `TestExtractLGConfigTLSDefaultOn` | `internal/component/config/loader_extract_test.go` | absent leaf reads true; explicit false honored |DONE (`internal/component/config/lg_extract_test.go`) |
| `TestLGTokenMiddleware` | `internal/component/lg/server_test.go` | token set: 401 without/with-wrong bearer on /api/ and /lg/; no token: open |DONE (`internal/component/lg/auth_test.go`) |
| `TestReloadListenersRefusesUnauthNonLoopback` | `cmd/ze/hub/listener_migrate_test.go` | AC-7 |DONE (`cmd/ze/hub/mgmt_guard_test.go`) |
| `TestDoctorFlagsGnmiExposure` | `internal/component/doctor/checks_config_test.go` | AC-6 through the doctor check entry point |DONE (`internal/component/doctor/checks_config_test.go`) |
| `TestExtractGNMISettingsSurviveDisabledBlock` | `internal/component/config/gnmi_extract_test.go` | AC-1 -- token and TLS paths survive a block without `enabled true` |DONE (mutation-verified) |
| `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` | `cmd/ze/hub/gnmi_infra_test.go` | AC-1 -- the guard does not refuse a listener whose config named a token |DONE (red before the fix) |
| `TestResolveGNMIListenersKeepsTokenWithEnvListenAddress` | `cmd/ze/hub/gnmi_infra_test.go` | AC-1 -- an env-supplied loopback gNMI listener is authenticated by the config token |DONE (red before the fix) |
| `TestExtractAPISettingsSurviveDisabledTransports` | `internal/component/config/api_extract_test.go` | AC-4 -- token, addresses and the gRPC TLS pair survive a transport without `enabled true` |DONE (red before the fix) |
| `TestResolveAPIListenersKeepsSettingsFromDormantBlock` | `cmd/ze/hub/api_infra_test.go` | AC-4 -- the guard does not refuse an env-started API listener whose config named a token and a loopback address |DONE (red before the fix) |
| `TestResolveAPIListenersKeepsGRPCTLSFromDormantBlock` | `cmd/ze/hub/api_infra_test.go` | AC-4 -- an env-started gRPC transport serves the operator's certificate instead of plaintext |DONE (red before the fix) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listen address class | loopback (127.0.0.0/8, ::1) / routable / unparseable | 127.255.255.254 and ::1 allowed unauth | N/A | 128.0.0.1, 0.0.0.0, ::, `localhost` (unparseable = non-loopback) refused unauth |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mgmt-guard-gnmi-nonloopback-refused` | `test/plugin/*.ci` | operator enables gNMI on 0.0.0.0 with no token; daemon exits 1 with a clear error before binding |PASS |
| `mgmt-guard-gnmi-token-allowed` | `test/plugin/*.ci` | same config plus token boots normally |PASS |
| `mgmt-guard-mcp-env-nonloopback-refused` | `test/plugin/*.ci` | `ze.mcp.listen` routable + no token refused at boot (per-command `env=` directive) |PASS |
| `mgmt-guard-mcp-bind-remote-none-refused` | `test/plugin/*.ci` | YANG bind-remote + auth-mode none refused at boot with the Validate message |PASS |
| `mgmt-guard-web-insecure-env-refused` | `test/plugin/*.ci` | `ze.web.insecure` env with default 0.0.0.0 listen refused at boot |PASS |
| `mgmt-guard-loopback-allowed` | `test/plugin/*.ci` | loopback-bound unauthenticated gNMI+MCP+web boots, exit path clean |PASS |
| `doctor-gnmi-mcp-exposure` | `test/ui/*.ci` | `ze doctor --json` emits config-gnmi-invalid + config-mcp-invalid on an exposing config |PASS |
| `mgmt-guard-reload-refuses-nonloopback` | `test/reload/*.ci` | SIGHUP migration to non-loopback refused for unauth service, daemon keeps serving |PASS |
| `lg-tls-default-on` | `test/plugin/*.ci` | LG enabled with no tls leaf serves https (stderr banner `looking glass listening on https://`) |PASS |
| `mgmt-guard-web-env-started-address-survives` | `test/plugin/*.ci` | `ze.web.enabled` plus a block naming one address: the refusal quotes that address, never the 0.0.0.0:3443 default |PASS (red before the fix) |
| `mgmt-guard-api-env-started-settings-survive` | `test/plugin/*.ci` | `ze.api-server.rest.enabled` plus a dormant block: REST binds the address the block names and authenticates with its token |PASS (red before the fix) |
| `mgmt-guard-gnmi-env-started-address-survives` | `test/plugin/*.ci` | `ze.gnmi.enabled` plus a dormant block: gNMI binds the loopback address the block names, never `0.0.0.0:9339` |PASS (red before the fix: bound `[::]:9339`) |
| `mgmt-guard-gnmi-env-started-token-survives` | `test/plugin/*.ci` | `ze.gnmi.enabled` plus a dormant block naming a token: the daemon boots instead of refusing over a token the operator already wrote |PASS (red before the fix: boot refusal, exit 1) |
| `mgmt-guard-lg-env-started-address-survives` | `test/plugin/*.ci` | `ze.looking-glass.enabled` plus a dormant block: LG binds the block's loopback address, never the env var's `0.0.0.0:8443` default |PASS (red before the fix: bound `http://[::]:8443/`) |
| `mgmt-guard-web-env-started-address-binds` | `test/plugin/*.ci` | `ze.web.enabled` plus a dormant block: the web server reports binding the address the block names | **PASS, discrimination PROVEN 2026-08-10** |
| `mgmt-guard-web-dormant-insecure-warns` | `test/plugin/*.ci` | `ze.web.enabled` plus a dormant block carrying `insecure true`: the daemon names the dropped leaf in a WARN and serves authenticated | **PASS, discrimination PROVEN 2026-08-11** |
| `lg-token-gate` | `test/plugin/*.ci` | the configured looking-glass token reaches the running server: no bearer is refused, the right bearer is served |PASS (mutation-verified) |

**The two web rows above, measured by the main thread one process at a time,
`mgmt-guard-web-env-started-address-binds` on 2026-08-10 and
`mgmt-guard-web-dormant-insecure-warns` on 2026-08-11.**

`mgmt-guard-web-env-started-address-binds` is PROVEN. With the fix it passes and
the daemon logs `web server listening on https://127.0.0.1:18449/`. With the
three-line address fallback removed from `runYANGConfig` and the binary rebuilt,
it FAILS on `stderr does not contain "web server listening on
https://127.0.0.1:18449/"`, and the log shows `https://0.0.0.0:3443/`. Restored
byte-identical; passes again.

`mgmt-guard-web-dormant-insecure-warns` is PROVEN, on the second attempt.
Neutralising the guard to `if false` and rebuilding gives a binary in which
`strings ... | grep -c "insecure not honored"` returns **0** — the control,
checked BEFORE the run. Against it the test FAILS on `stderr does not contain
"environment.web insecure not honored"`. Restored: the count returns to 1 and the
test passes. `cmd/ze/hub/main.go` is byte-equivalent, `gofmt -l` empty.

**Why the first attempt reported a false result, because the trap is reusable.**
It built with `make <session-bin-path>/ze`, an explicit path target with no
Makefile rule. `make` did nothing and returned success, so the run used a stale
binary that still held the WARN, and the pass was read as "the test does not
discriminate". `make ze` is the phony target that actually builds.

**The lesson, stated for the next author: a discrimination run proves nothing
until the control is checked.** Confirm the artefact under test really changed —
here, `strings` on the binary — before reading the result. A build command that
exits 0 is not evidence that it built.

Note: no new suite directory. `test/plugin` boot-refusal tests follow
`test/plugin/family-no-plugin-failure.ci` (foreground `ze -`, `expect=exit:code=1`,
`expect=stderr:contains=`); doctor tests follow `test/ui/doctor-listeners.ci`;
reload tests join the existing `test/reload` suite (`ze-test bgp reload`). All run
natively (config/refusal only, no kernel features), so no `option=needs-linux`.

## Files to Modify
- `cmd/ze/hub/main.go` - hoist API and gNMI resolution into the resolution block ending at `:373`; assemble declarations; single `checkMgmtListeners` call before `eng.Start`; replace the inline API refusal with the guard's API declaration (message preserved); run MCP/gNMI Validate at boot
- `cmd/ze/hub/api.go` - fold `apiHasNonLoopback` into the shared classifier (delete or delegate)
- `cmd/ze/hub/service_gnmi.go` - consume pre-resolved gNMI values from `gnmiBuildInputs` instead of resolving internally
- `cmd/ze/hub/gnmi_infra.go` - extend `gnmiBuildInputs` with resolved listen/token/TLS plain values
- `cmd/ze/hub/service_lg.go` - pass the optional LG token through `lg.LGConfig`
- `cmd/ze/hub/service_registry.go` - add LG token field to `ServiceDeps` (generic string, no lg import)
- `cmd/ze/hub/listener_migrate.go` - gate `ReloadListeners` with the classifier (AC-7)
- `cmd/ze/hub/api_infra.go` - `resolveAPIListeners` (always-on API enable/address/token/TLS resolution, hoisted out of `runYANGConfig` so the guard and the gated builders read one resolver) and `apiGuardAddrs` (a dormant transport declares no address)
- `internal/component/config/loader_extract.go` - split `extractAPIBlock` out and add `ExtractAPISettings` so the token, the addresses and the gRPC TLS pair survive a transport without `enabled true`; add `GNMIListenConfig.Validate`; split `extractGNMIBlock` out and add `ExtractGNMISettings` so the token and TLS paths survive a block without `enabled true`; flip `ExtractLGConfig` TLS default to true (absent leaf reads true); extract LG token leaf
- `internal/component/config/validate_semantic.go` - wire gNMI Validate (code `config-gnmi-invalid`)
- `internal/component/config/cli/cmd_validate.go` - same wiring beside the MCP block
- `internal/component/config/listener_defaults.go` - register "gnmi" default `0.0.0.0:9339`
- `internal/component/doctor/checks_listener.go` - add gNMI to the hardcoded fallback
- `internal/core/diagnostic/codes.go` - register `config-gnmi-invalid` (precedent `config-mcp-invalid` at `:64`)
- `internal/component/config/environment.go` - register `ze.looking-glass.token` (Secret) and document the TLS default flip on `ze.looking-glass.tls`
- `internal/component/lg/server.go` - optional bearer-token middleware over the mux; constant-time compare per the gNMI model (`internal/component/gnmi/server.go`)
- `internal/component/lg/yang/ze-lg-conf.yang` - `tls` default true; new `token` leaf (`ze:sensitive`, description names the env override per `ai/rules/config.md`)

## Files to Create
- `cmd/ze/hub/mgmt_guard.go` - `mgmtListener` declaration type, fail-closed non-loopback classifier, `checkMgmtListeners`
- `cmd/ze/hub/mgmt_guard_test.go` - unit tests above
- `cmd/ze/hub/gnmi_infra_test.go` - `resolveGNMIListeners` address/settings split
- `cmd/ze/hub/api_infra_test.go` - `resolveAPIListeners` address/settings split, including the gRPC TLS pair
- `internal/component/config/gnmi_extract_test.go` - `ExtractGNMISettings` vs `ExtractGNMIConfig`
- `internal/component/config/api_extract_test.go` - `ExtractAPISettings` vs `ExtractAPIConfig`
- `test/plugin/mgmt-guard-*.ci`, `test/ui/doctor-gnmi-mcp-exposure.ci`, `test/reload/mgmt-guard-reload-refuses-nonloopback.ci`, `test/plugin/lg-tls-default-on.ci` - functional tests above

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- add `checkMgmtListeners` with the gNMI declaration only and a failing functional test: `mgmt-guard-gnmi-nonloopback-refused.ci` must refuse to start. Requires the gNMI resolution hoist (service_gnmi.go -> main.go, MCP precedent) so the guard can see the address before the seam builds.
2. **Phase: shared guard, all surfaces** -- fold `apiHasNonLoopback` in; add MCP (config + env paths, boot-time `Validate` call), web-insecure, API declarations; hoist API resolution; compiled-out surfaces never declare (factory-name / seam-nil checks). Tests: remaining refusal + allowed .ci rows, unit classifier tests.
3. **Phase: semantic validation parity** -- `GNMIListenConfig.Validate`, `ValidateSemantics` + `cmd_validate.go` wiring, `config-gnmi-invalid` code registration. Tests: `TestGNMIListenConfigValidate`, `TestValidateSemanticsFlagsGNMI`.
4. **Phase: doctor** -- "gnmi" listener default, hardcoded fallback entry, doctor functional test. Verify with the doctor coverage tests named in `ai/rules/repo-maintenance.md`.
5. **Phase: LG TLS default + optional auth gate** -- YANG default flip, extraction default, env default on the env-enable path, token leaf + env registration, LG token middleware. Tests: `TestExtractLGConfigTLSDefaultOn`, `TestLGTokenMiddleware`, `lg-tls-default-on.ci`.
6. **Phase: reload gate (AC-7, ~~pending Thomas's confirmation~~ CONFIRMED in-scope 2026-07-17, AUTONOMOUS DEFAULT)** -- classifier check in `ReloadListeners`; `test/reload` .ci.
7. **Functional + QEMU boot tests** -- run the new .ci suites natively; then `make ze-qemu-all-test` covers them in the Alpine VM, and the appliance boot path gets one QEMU scenario: boot a gokrazy-style config with gNMI exposed (expect refusal message on console) then the remedied config (expect ready) -- see `ai/rules/platform-linux.md`; no `needs-linux` marks needed since the tests are config/refusal-only.
8. **Full verification** -- `make ze-verify`.
9. **Complete spec** -- audit tables, upgrade-breaking entry for R-1/R-2 in `docs/guide/authentication.md` (this repository has no release-notes surface, and that page already documents the guard), journal row, two-commit closure. The `plan/learned/NNN-<name>.md` form this step asked for was retired: `ai/rules/planning.md` ("Writing Journal Rows") replaced it with a row in `plan/journal/<class>.md`, and the class here already exists as `enabled-gate-discards-settings.md`.

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Fail-closed | Empty token / unset auth-mode is treated as unauthenticated, never as "auth present"; unparseable host classifies as non-loopback |
| Coverage | Every management listener is declared to the guard (no silent bypass); compiled-out exclusion is keyed on the factory/seam actually being absent, not on config |
| Message safety | The refusal error does not leak secrets (never print tokens); it names the listener, the address class, and the remedy |
| Timing | LG token compare is constant-time over fixed-length digests (gNMI model, `gnmi/server.go`) |
| Reload | Address migration cannot resurrect the exposure the boot guard refused (AC-7) |

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-7 each have code + test |
| Registration over hardcoding | Guard iterates declarations; no per-service knowledge inside `checkMgmtListeners`; no copy-pasted per-service `if` (`ai/rules/plugins.md`) |
| Single classifier | Exactly one non-loopback classifier remains (`apiHasNonLoopback` and any new copies folded); `MCPListenConfig.AnyListenerNonLoopback` either delegates or documents why it stays |
| Doctor checks | gNMI default + fallback + `config-gnmi-invalid` per `ai/rules/repo-maintenance.md`; codes registered and explainable |
| YANG validation | New LG `token` leaf sensitive, described with env override; `tls` default change reflected in extraction (raw tree lacks YANG defaults, `listener.go`) |
| Compile-out purity | `mgmt_guard.go` and hoisted resolutions import no gnmi/mcp/web/lg package; `make` build-tag matrix tests still pass (`cmd/ze/hub/build_tag_*_test.go`) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Hard-fail at boot (exit 1) for non-loopback + unauthenticated | warn-and-clamp to loopback (the web-YANG/MCP-clamp behavior) | Thomas confirmed the API-precedent default (2026-07-16); a clamp silently changes where a service listens, a refusal is explicit and names the remedy (`ai/rules/evidence.md`, exact-or-reject) |
| One guard call site in `runYANGConfig`, before `eng.Start` (`main.go`) | per-builder checks inside each factory | Factories run after other services already bound (`buildServices` at `:746` precedes API/gNMI resolution today); a single pre-bind point is the only place "nothing has bound yet" holds |
| Hoist gNMI + API resolution always-on; declarations are runtime values collected in one function | init()-time registration registry for guard entries | Declarations depend on boot-resolved env/YANG/CLI values; the MCP feature-gate precedent already keeps resolution always-on (`main.go`); gated code cannot declare before it runs. The guard stays generic (iterates declarations), preserving registration-over-hardcoding intent |
| Compiled-out surfaces never declare (factory-name/seam-nil keyed) | guard evaluates resolved intent even when the service cannot build | A binary without ze_gnmi cannot expose gNMI; refusing to boot on config it cannot serve would break today-working deployments for zero exposure reduction |
| Boot also runs `MCPListenConfig.Validate` (+ new gNMI Validate) | guard-only coverage | Reuses the existing precise error messages; covers inconsistencies beyond the loopback rule (bearer without token, oauth without TLS) that the guard alone would miss |
| LG exempt from the unauth refusal; gets TLS-default-on + optional token instead | hard-fail LG like gNMI/MCP | A looking glass is an intentionally public, read-only surface (birdwatcher-compatible API); refusing unauthenticated LG would break its primary use case. TLS default + opt-in token addresses the audit finding at its severity (MEDIUM) |
| Classifier treats unparseable hosts as non-loopback | parse-or-skip | Both existing producers already fail closed (`api.go`, `loader_extract.go`); a DNS name must not smuggle remote reachability |
| Reload migrations gated by the same classifier (AC-7) | boot-only guard | `ReloadListeners` (`listener_migrate.go`) can move a running unauthenticated listener non-loopback; a guard that a SIGHUP can bypass fails open. ~~Pending Thomas's scope confirmation~~ CONFIRMED in-scope 2026-07-17 (AUTONOMOUS DEFAULT) |

## Known Limitations
- Env-var exposure (`ze.mcp.listen`, `ze.gnmi.*`, `ze.web.insecure`) is enforced by the boot guard only; `ze doctor` and `ze config validate` inspect a config file and cannot see another process's environment (R-5).
- The same blindness runs the other way for gNMI: a config that binds gNMI non-loopback while the token comes from `ze.gnmi.token` boots correctly and is still reported `config-gnmi-invalid`, because the file alone describes an exposed listener. The message names both token sources so the operator can tell the two cases apart. Making the offline check read the daemon's environment is not possible; making it silent would lose the exposure it exists to report.
- SSH, plugin hub, telemetry/Prometheus, and the managed server are out of the guard's declaration set by design (A-4): SSH authenticates by protocol, hub enforces min-32 secrets (`loader_extract.go`), Prometheus defaults loopback and is read-only metrics. Doctor bind probes still cover them.
- The Prometheus exclusion carries one residual case, and the earlier evidence for it named the wrong producer. The doctor probe list in `internal/component/doctor/checks_listener.go` is a bind-availability probe, not the default. The producer is `extractTelemetryConfig` (`internal/component/telemetry/exporter/server.go`): it seeds every endpoint with `defaultTelemetryHost` = `127.0.0.1` and synthesizes one such endpoint when the block names no server, so the DEFAULT is loopback. An operator who writes `server main { ip 0.0.0.0 }` gets a non-loopback metrics listener, and `extractBasicAuthConfig` leaves `BasicAuth.Enabled` false when the `basic-auth` block is absent, so that listener is unauthenticated and the guard never sees it. The exclusion stands (read-only metrics, loopback default), and this is the case it does not cover.
- gNMI token-over-plaintext (token set, no TLS) still boots: the guard enforces authentication, not transport secrecy. TLS-required-for-token is a possible follow-up (open question 4, resolved 2026-07-17: NOTED FOLLOW-UP, not this spec).
- Auth-mode changes on SIGHUP reload do not take effect (servers are built once); AC-7 only prevents the address from moving into exposure. Full reload-time auth rebuild is out of scope.
- An IPv6 literal written as `ip ::1` reaches the guard as `::1:9339`, because `ServerEndpoint.Listen` (`internal/component/config/loader_extract.go`) joins host and port with a bare colon and brackets nothing. **The strongest fact about this case, and the reason it earns a row: `listenAddrIsNonLoopback` (`cmd/ze/hub/mgmt_guard.go`) cannot `SplitHostPort` a bracket-less `::1:9339`, and the parse of the whole string fails too, so the function classifies IPv6 LOOPBACK as non-loopback. The refusal then tells the operator to bind `127.0.0.1/::1`, which is the address they already bound.** Two facts complete the picture. `net.Listen` rejects the same string, so `ip ::1` was an unusable config before this guard existed and the guard removes no working deployment. `docs/guide/api.md` advertises `::1:<port>` as a value to set, so the product itself names the form that reproduces the refusal. Fail-closed, no exposure, and shared by every listener rather than owned by the guard. Recorded in `plan/journal/ipv6-address-built-by-concatenation.md`; not fixed here.
- `cors-origin` is the other boundary-relaxing setting the settings/listener split moved outside the enable gate (`extractAPIBlock`, `internal/component/config/loader_extract.go`), and it needs no `insecure`-style exclusion because it is not auth-removing. `internal/component/api/rest/server.go` sets `Access-Control-Allow-Origin` (`:789`, `:811`, `:847`), `-Allow-Methods`, `-Allow-Headers` and `-Max-Age`, and never sets `Access-Control-Allow-Credentials`, so a cross-origin browser request carries no cookie the browser will attach. Every route that returns data is wrapped in `s.withAuth` (`:379-407`). The one route that is not is `mux.HandleFunc("OPTIONS /api/", s.handlePreflight)` (`:410`), and that exception is correct: a CORS preflight is what the browser sends BEFORE it is allowed to attach the `Authorization` header, so a preflight the server answers only when authenticated can never be answered at all. `handlePreflight` (`:842-852`) returns 204 with no body and reads nothing from the request, so it discloses only the CORS policy the operator configured. So the setting relaxes a browser boundary and cannot make an unauthenticated request succeed. Recorded here so the next reader does not re-derive it.
- `ze start --web-only` paths are unaffected: the flag clamp (`ze_core_start.go`) already forces loopback for insecure web-only, and `ze.web.insecure` is not consulted on that path (`RunWebOnly` -> `runWebOnly`, `service_web.go`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] QEMU boot-refusal test passes (or N/A with justification)
- [ ] Registration over hardcoding respected

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Upgrade-breaking entry written for the behavior change (boot refusal on upgrade R-1; LG TLS default R-2). This repository has no release-notes surface, so it lives in `docs/guide/authentication.md` under "Upgrading from a release without the guard", beside the guard it describes. Do not create a release-notes file for it

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for loopback vs routable vs unparseable
- [ ] Functional `.ci` tests for each refusal

## Review Gate

### Pass 1 (boot-guard core: AC-1..AC-4, AC-7)

Two independent reviewer subagents (security + correctness/wiring) reviewed the
diff from the guard entry point.

- BLOCKER: 0
- ISSUE: 2, both FIXED:
  - [security] MCP fail-open -- `auth-mode none` + token read as authenticated
    while the server builds the accept-all `noneAuthenticator` (producers:
    `internal/component/mcp/streamable.go`, `bearer.go`,
    `cmd/ze/hub/service_mcp.go`). Fixed by `mcpListenerAuthenticated`
    (mirrors server effective-mode precedence); regression test
    `TestMcpListenerAuthenticated` incl. the none+token case.
  - [correctness] `GNMIListenConfig.Validate` doc comment claimed
    validate/doctor/boot wiring that this phase defers (AC-6). Comment corrected.
- NIT: 2, ACCEPTED:
  - Guard call site is before every in-scope management bind but after
    `eng.Start`/SSH/dropPrivileges (not a fail-open; hoisting further is higher
    regression risk than the NIT warrants).
  - Non-loopback classifiers split across the hub and config packages (cannot
    share without a new dependency; identical fail-closed netip rule).

### Pass 2 (AC-5 looking glass, AC-6 offline parity, all functional tests)

Two independent reviewer subagents (security + correctness/wiring) reviewed the
whole diff. Every finding is FIXED; each fix carries a mutation-verified test.

- BLOCKER: 2, both FIXED:
  - [security] `ExtractLGConfig` returned before reading `tls` and `token`
    whenever the block lacked `enabled true`, so a looking glass started by
    `ze.looking-glass.enabled` or `ze.looking-glass.listen` ran PLAINTEXT and
    OPEN while the operator's config asked for TLS and a bearer token. This is
    the environment.mcp defect (Task finding 5) recurring on a second service.
    Fixed with the same split: `extractLGBlock` reports "block exists" and
    "block asks for a listener" separately, `ExtractLGSettings` serves the
    settings question, and `cmd/ze/hub/main.go` takes addresses from one and
    settings from the other. Test: `TestExtractLGSettingsSurviveDisabledBlock`.
  - [security] The web config form read the `tls` toggle from the raw tree,
    which carries no YANG defaults, so a config inheriting the new default-ON
    rendered TLS as OFF; the toggle template always posts a companion hidden
    `false`, so saving the Looking Glass form for any reason wrote `tls false`
    and downgraded HTTPS to plaintext. Fixed by `configValueOrDefault` applying
    the same default the extractor applies, and the missing `token` field was
    added (password type). Test: `TestBuildLookingGlassFormDataTLSDefaultsOn`.
- ISSUE: 7, all FIXED:
  - AC-5 was narrowed by an undocumented plaintext fallback when TLS is on by
    default and blob storage is absent. The behavior is correct (a hardening
    default must not remove a working looking glass) but was unwritten and
    untested. AC-5 and Behavior-to-preserve now state it; tests
    `TestBuildLGService_ExplicitTLSWithoutBlobStorageFails` and
    `TestBuildLGService_DefaultTLSWithoutBlobStorageServesPlaintext` pin both
    directions, so the fallback can never widen to an explicit `tls true`.
  - The looking-glass token was proven only by a unit test that built
    `LGConfig` directly; nothing proved the env/YANG value reached the server.
    Added `test/plugin/lg-token-gate.ci` (mutation-verified).
  - `RegisterListenerDefault("gnmi", ...)` was exercised only through the
    hardcoded fallback, which never consults the defaults map. Added
    `TestDoctorGnmiDefaultEndpointIsProbed`, which drives the schema path and
    also pins that the discovered service name really is `gnmi`.
  - The `ze config validate` gNMI leg had no test (doctor reaches the check by
    a different route). Added `internal/component/config/cli/cmd_validate_gnmi_test.go`.
  - Three non-loopback classifiers existed, two of them byte-identical inside
    `internal/component/config`. Folded into `anyEndpointNonLoopback`; both
    `AnyListenerNonLoopback` methods now delegate. One rule per package remains,
    with the hub's boot-guard copy documented as mirroring it.
  - The offline gNMI check reports a config whose token comes from
    `ze.gnmi.token`. The message now names both token sources, and Known
    Limitations records the direction.
  - Stale documentation from the TLS default flip and the new token, in
    `docs/features/looking-glass.md`, `docs/guide/looking-glass-howto.md`,
    `docs/guide/looking-glass.md`, `docs/architecture/config/environment.md`,
    `docs/architecture/web-interface.md`, `ai/patterns/web-endpoint.md`, and the
    `internal/component/lg` package doc.
- NIT: 3, 2 FIXED and 1 RECORDED:
  - The bearer scheme match was case-sensitive; RFC 7235 Section 2.1 makes the
    auth-scheme case-insensitive, so a conforming `bearer <token>` was refused.
    Fixed with `strings.EqualFold` over the scheme only; the token stays
    case-sensitive. Test: `TestLGTokenMiddleware/scheme_is_case-insensitive`.
  - A stale `startLGServer` doc anchor in `docs/guide/looking-glass.md`, fixed.
  - `TestDoctorDependencyInventory` (`internal/component/doctor/doctor_test.go`)
    has no `listener/gnmi` row. That file is owned by another concurrent session
    and was off-limits; the row and the `expectedTotal` bump are outstanding.
    → CLOSED 2026-08-10: the row is present and `TestDoctorDependencyInventory`,
    `TestDoctorCoverageCodesRegistered` and `TestDoctorGnmiDefaultEndpointIsProbed`
    pass.

Mutation verification: 13 mutations, each reverting one guard, wiring, or
default, and each confirmed to turn its own test red before restore.

### Pass 3 (independent gate over the whole spec) and its remediation

An independent Review Gate found the guard itself sound and AC-1..AC-7 met with
discriminating evidence. It raised four items, all FIXED 2026-08-10.

- BLOCKER: 1, FIXED. gNMI was still on a single-gate extractor: `ExtractGNMIConfig`
  returned `ok=false` unless `enabled == true`, so `resolveGNMIListeners`
  (`cmd/ze/hub/gnmi_infra.go`) read neither the token nor the servers of a block
  the operator had written. Two observable consequences. A block carrying
  `enabled false; token secret` plus `ze.gnmi.enabled` synthesized `0.0.0.0:9339`
  with an empty token, so the daemon refused to boot while telling the operator
  to set the token they had already written. The same block plus
  `ze.gnmi.listen=127.0.0.1:9339` booted an UNAUTHENTICATED gNMI `Set` surface.
  This is Task finding 5 (environment.mcp) and the Pass 2 looking-glass BLOCKER
  recurring on the third service, and the spec's own instruction was to apply the
  auth settings independently of the enable gate. Fixed with the same split:
  `extractGNMIBlock` reports "block exists" and "block asks for a listener"
  separately, `ExtractGNMISettings` serves the settings question, and
  `resolveGNMIListeners` takes addresses from one and the token from the other.
  `gnmiBuildImpl` (`cmd/ze/hub/service_gnmi.go`) takes the TLS cert and key paths
  from the settings question for the same reason. Tests:
  `TestExtractGNMISettingsSurviveDisabledBlock` (mutation-verified: restoring the
  enable gate turns it red) and `TestResolveGNMIListenersKeepsTokenFromDisabledBlock`
  / `TestResolveGNMIListenersKeepsTokenWithEnvListenAddress`, both red before the
  fix, the first printing the boot refusal the operator could not act on.
- ISSUE: 3, all FIXED.
  - `GNMIListenConfig.Validate` and `MCPListenConfig.Validate` cited
    `.claude/rules/exact-or-reject.md`, deleted when the rules merged in
    `ad809ea43`. Both now cite `ai/rules/protocol.md`.
  - `checkReloadExposure` (`cmd/ze/hub/listener_migrate.go`) reads only the
    addresses a change ADDS, so a reload that turns authentication OFF in place
    is not judged there. Its doc comment now names the three properties in other
    packages that close that case, and the condition that reopens it.
  - Assumption A-4 justified the Prometheus exclusion from the doctor probe list
    rather than from its producer. It now cites `extractTelemetryConfig`
    (`internal/component/telemetry/exporter/server.go`), and Known Limitations
    states the residual case honestly: an operator CAN configure a non-loopback
    metrics listener with `basic-auth` absent, and the guard does not see it.

**The Review Gate artifact is still owed and this round cannot supply it.** The
remediation above is new code written by its author, so `review_gate.py record`
over it would be the author reviewing himself, which `ai/rules/planning.md`
bans. A fresh independent pass runs over the remediation diff, and records the
artifact.

### Pass 4 (independent gate over the Pass 3 remediation) and its remediation

The gate found the guard sound and all five claims of the Pass 3 remediation
true. It raised two items inside that remediation diff, both FIXED 2026-08-10.

- BLOCKER: 1, FIXED. The settings/listener split gave `resolveGNMIListeners`
  (`cmd/ze/hub/gnmi_infra.go`) the token from the settings question while it
  still took the ADDRESS from the listener question, so a block that named
  `127.0.0.1` and did not say `enabled true` fell through to the hardcoded
  `0.0.0.0:9339`. Before Pass 3 that config refused to boot, so honoring the
  token made the wildcard bind newly REACHABLE: gNMI `Set`, a full
  config-mutation API, published on every interface where the operator wrote
  loopback. Fixed by reading the address as a SETTING, like the token and the
  TLS paths: only `enabled` says START. It strictly narrows, and it is a no-op
  for a block naming no server, because `extractServerList`
  (`internal/component/config/loader_extract.go`) synthesizes the same default.
  `cmd/ze/hub/gnmi_infra_test.go` pinned the wildcard deliberately; that
  assertion encoded the defect and was corrected with the code.
- BLOCKER: the SAME defect on the looking glass, FIXED in the same round.
  `cmd/ze/hub/main.go` applied the `0.0.0.0:8443` default of
  `ze.looking-glass.enabled` BEFORE reading the config, so it won over the
  address a dormant block named. Fixed the same way: the env var says START and
  says nothing about WHERE, so its default is applied after the settings
  question. A dormant block still starts nothing, because a non-empty `lgAddrs`
  is what starts the looking glass.
- ISSUE: 1, FIXED. Two comments claimed `ze.gnmi.listen` starts the gNMI server.
  It does not: `enabled` comes only from `env.IsEnabled("ze.gnmi.enabled")` or
  an `enabled true` block. The wording was copied from MCP and LG, where it is
  correct. Corrected in `internal/component/config/loader_extract.go`,
  `cmd/ze/hub/gnmi_infra.go`, and the third copy in
  `internal/component/config/gnmi_extract_test.go`.

The fixed gNMI path had no daemon-level test, which both sibling services have
(`test/plugin/mcp-cli-listener-honors-config-auth.ci`,
`test/plugin/lg-token-gate.ci`). Every other `mgmt-guard` scenario writes
`enabled true`, so nothing exercised an env-started listener whose credentials
come from a dormant block. Three `.ci` close that, each mutation-verified by
reverting its own fix:

| Test | Fix reverted | Observed before |
|------|--------------|-----------------|
| `test/plugin/mgmt-guard-gnmi-env-started-token-survives.ci` | token from settings | boot refusal, exit 1 |
| `test/plugin/mgmt-guard-gnmi-env-started-address-survives.ci` | address from settings | bound `[::]:9339`, wanted `127.0.0.1:19342` |
| `test/plugin/mgmt-guard-lg-env-started-address-survives.ci` | LG address from settings | bound `http://[::]:8443/`, wanted `http://127.0.0.1:18447/` |

**Files this round touched, and the whole scope of the next pass:**
`cmd/ze/hub/gnmi_infra.go`, `cmd/ze/hub/main.go`,
`cmd/ze/hub/gnmi_infra_test.go`,
`internal/component/config/loader_extract.go`,
`internal/component/config/gnmi_extract_test.go`, the three `.ci` above, and
this section.

**The Review Gate artifact stays owed.** This round is again new code written by
its author. A fifth independent pass runs over the scope above and records it.

### Pass 5 (independent gate over the Pass 4 remediation) and its remediation (round 6)

Verdict NOT CLEAN: 1 BLOCKER, 3 ISSUE, 4 NOTE. The pass verified the Pass 4 fix
by revert and found the SAME defect class open on two more services. All four
items FIXED 2026-08-10.

- BLOCKER: 1, FIXED. The sibling paths of the gNMI/LG defect were still open on
  `web` and `api-server`, and both carry the wildcard-default direction that made
  the gNMI case a BLOCKER. `extractWebBlock` gated `enabled`
  (`internal/component/config/loader_extract.go`), `ExtractWebConfig` returned
  `ok=false` for a dormant block, `webAddrs` stayed empty and `resolveWebListeners`
  (`cmd/ze/hub/main.go`) applied `defaultWebListen` `0.0.0.0:3443`.
  `ExtractAPIConfig` returned `APIConfig{}, false` when neither transport was
  enabled, dropping the token, the addresses and the gRPC TLS paths together,
  while `main.go` synthesized `0.0.0.0:8081` / `0.0.0.0:50051` for the two
  `ze.api-server.*.enabled` variables. A block carrying `token` beside a loopback
  `server` reproduced round 4's headline symptom exactly: a boot refusal telling
  the operator to set the token they had already written. Fixed with the same
  settings/listener split on both services. `extractServerList` synthesizes the
  identical `0.0.0.0:3443` default for a block that names no server, so the web
  change is a strict narrowing.
- ISSUE: 3, all FIXED.
  - Deterministic port collision: `test/plugin/mgmt-guard-loopback-allowed.ci`
    hardcoded MCP port 18086, which `test/plugin/rest-no-auth-readonly.ci` also
    binds, and that test is in no exclusive group. Moved to 18089.
  - IPv6 literals are newly reachable on one more path and are mishandled by a
    producer shared with every listener. Fail-closed, no exposure. Recorded in
    Known Limitations and in `plan/journal/ipv6-address-built-by-concatenation.md`.
  - The upgrade note did not say that a block's address now applies when an env
    var starts the service. Written into `docs/guide/authentication.md`.
- NOTE: 4, recorded. The MCP non-fix reasoning holds (Notes, three legs); the
  extra-listener warning in `gnmiBuildImpl` stays on the listener question;
  `mgmt-guard-gnmi-env-started-token-survives.ci` binds a wildcard on 9339, so
  two concurrent copies of the suite still collide; the one
  `audit-test-relaxation` finding belongs to another spec.

Round 6 also fixed a PRODUCT defect that the pass surfaced when it checked that
remediation. `extractAPIBlock` read the gRPC `tls-cert` / `tls-key` pair inside
the `enabled == configTrue` branch while it read `token` above it, so
`ze.api-server.grpc.enabled` built an AUTHENTICATED management gRPC server with
empty TLS paths (`cmd/ze/hub/service_grpc.go`) and served that token in clear.
Verified by revert: `TestResolveAPIListenersKeepsGRPCTLSFromDormantBlock` goes
red with the resolver pointed back at `ExtractAPIConfig`.

### Pass 6 (independent gate over the Pass 5 remediation) and its remediation (round 7)

Verdict NOT CLEAN: 0 BLOCKER, 3 ISSUE, 5 NOTE. Scope: the round-6 remediation
only. The gate found the guard itself sound, the web and api-server splits
correct, and the `insecure` exception right: `extractWebBlock` clamps every
server entry to `127.0.0.1` whenever `Insecure` is set, so a dormant insecure
block can only ever supply loopback, and `insecureWeb` never rises from one.
Evidence it ran: the three `resolveAPIListeners` tests go red under an overlay
revert and 7/7 pass unreverted; all 11 `mgmt-guard` `.ci` pass under
`make ze-plugin-test`; the 24 other failures and the 4
`audit-test-relaxation` findings are foreign to this scope.

- ISSUE: 3, all FIXED in round 7.
  - The web `.ci` proved only the REFUSAL branch, and its stated reason was
    false: `test/plugin/rbac-web-config-deny.ci` runs `ze init` in its own script
    and drives a web server on `https://127.0.0.1:18443`, so the positive-bind
    test both sibling services got was available here too. Written as
    `test/plugin/mgmt-guard-web-env-started-address-binds.ci`.
  - `resolveAPIListeners` (`cmd/ze/hub/api_infra.go`) returned errors naming
    `ze.api.rest.listen` / `ze.api.grpc.listen`. The registered keys are
    `ze.api-server.rest.listen` / `ze.api-server.grpc.listen`
    (`internal/component/config/environment.go`), so an operator who greps for
    the name the error gives finds nothing. Both strings corrected, and
    `TestResolveAPIListenersRejectsBadListen`, which had pinned the wrong
    spelling, now asserts the registered one.
  - The dropped `insecure` leaf was silent. `cmd/ze/hub/main.go` now logs a WARN
    naming `environment.web.insecure` and the condition under which the block
    decides that switch, and `docs/guide/authentication.md` names the exclusion
    in the upgrade section. The WARN fires on `webAuthFollowsConfig == false`,
    which covers both a dormant block and an enabled block whose address a flag
    or an environment variable supplied first; only the first was reported.
- NOTE: 5, recorded, none fixed. `cors-origin` is not auth-removing (Known
  Limitations); `ze doctor` gains a cert-pair diagnostic for a dormant gRPC
  transport whose sibling is enabled (`internal/component/doctor/checks_tls.go`),
  which is defensible because an env var can start that transport; the
  startup-failure slog stage is now `api-server listen` rather than the
  per-transport name, and the wrapped error still carries the label; the IPv6
  disposition is right and is now stated at full strength (Known Limitations);
  this Review Gate had no Pass 5 section, which these two sections repair.

**The Review Gate artifact stays owed.** Round 7 is again new code written by its
author, and the artifact on disk carries `verdict=findings rounds=6`. A seventh
independent pass runs over round 7's diff and records the artifact. Its scope:
`cmd/ze/hub/api_infra.go`, `cmd/ze/hub/api_infra_test.go`, `cmd/ze/hub/main.go`,
`test/plugin/mgmt-guard-web-env-started-address-binds.ci`,
`docs/guide/authentication.md`, and these two sections.

### Pass 7 (independent gate over the round-7 remediation) and its remediation (round 8)

The gate verified round 7 in full and found the code sound: the corrected env-var
names match `env.MustRegister` (`internal/component/config/environment.go`), the
new `mgmt-guard-web-env-started-address-binds.ci` asserts the bound address, and
the widened `insecure` WARN condition covers a real second case, because
`webAuthFollowsConfig` and `insecureWeb` are set together inside `if len(webAddrs)
== 0` (`cmd/ze/hub/main.go`), so an enabled block whose address came from
`ze.web.listen` skips both. It raised one ISSUE and three record defects.

- ISSUE: 1, FIXED. The round-7 WARN was new operator-visible output with nothing
  driving it. This spec set the precedent itself in Pass 2, when a looking-glass
  token proven only by a unit test earned `test/plugin/lg-token-gate.ci`. Written
  as `test/plugin/mgmt-guard-web-dormant-insecure-warns.ci`: `ze.web.enabled`
  plus a dormant block carrying `insecure true`. `extractWebBlock`
  (`internal/component/config/loader_extract.go`) clamps every server entry to
  `127.0.0.1` when `Insecure` is set, so the address stays loopback, the guard
  passes it, and the WARN fires over a RUNNING server rather than a refused boot.
  The test pins both halves, and rejects `WARNING: authentication disabled`
  (`cmd/ze/hub/service_web.go`), which is the line that appears if the dropped
  leaf is ever honored. The `https://` banner at `:665` prints in both modes and
  cannot tell them apart.
- RECORD: 3, all corrected.
  - Neither the Wiring Test table nor the Functional Tests table named the `.ci`
    of rounds 4-7. Six tests appeared only in Review Gate prose, so the Goal Gate
    "Wiring Test table complete" read as unmet for those entry points. Both
    tables now carry them, and the Wiring Test table also gained the two
    round-6/7 rows that had reached the Functional Tests table only.
  - The `cors-origin` row in Known Limitations said "The REST auth middleware
    still runs on every request". `mux.HandleFunc("OPTIONS /api/",
    s.handlePreflight)` (`internal/component/api/rest/server.go`) is not wrapped
    in `s.withAuth`. The conclusion is unchanged and the exception is correct;
    the row now states both.
  - The WARN was one 30-word sentence carrying a colon and a comma splice,
    against the 25-word cap for a description (`ai/rules/writing.md`). Split into
    two sentences of 7 and 16 words. No condition changed.

**The Review Gate artifact stays owed, and round 8 cannot supply it either.** Two
of the three record defects above are edits to this section's own neighbours, and
the `.ci` is new code by its author. An eighth independent pass runs over round
8's diff and records the artifact. Its scope:
`test/plugin/mgmt-guard-web-dormant-insecure-warns.ci`, the WARN string in
`cmd/ze/hub/main.go`, the two test tables, the `cors-origin` Known Limitations
row, and this section. `--rounds-reason` must name a PRODUCT defect: round 6's
`extractAPIBlock` TLS-in-clear finding is the one that qualifies. A reason drawn
from record defects is refused by design.

## Notes
- Skeleton captured from the 2026-07-16 repository audit; verifier V1 corrected both earlier passes (MCP guard exists but does not run at boot; the insecure-web YANG path is clamped, only the env path bypasses). Deepened to design 2026-07-16: all citations re-verified against the working tree; the doctor claim was corrected (MCP is present in the hardcoded fallback at `checks_listener.go`; the schema path already discovers gNMI; the actual gaps are the missing "gnmi" listener default, the missing gNMI semantic Validate, and bind-probe-vs-exposure semantics).
- Open questions for Thomas: (1) confirm AC-7 (reload-migration gate) stays in this spec vs a follow-up; (2) confirm LG's exemption from the unauth refusal (public looking glass) plus TLS-default-on is the intended posture for finding 4; (3) whether existing clamp paths (web YANG insecure, MCP no-bind-remote, --insecure-web flag) should eventually converge on hard-fail too -- this spec keeps them clamping, which the guard then passes; (4) gNMI token-over-plaintext follow-up (Known Limitations).
  - → AUTONOMOUS DEFAULT (2026-07-17): (1) AC-7 (reload-migration gate) STAYS IN THIS SPEC. Rationale: `ReloadListeners` (`cmd/ze/hub/listener_migrate.go`, verified 2026-07-17 -- extracts new addrs and reconfigures web/lg/mcp/rest/grpc with no auth re-check) can move a running unauthenticated listener non-loopback after boot; a guard a SIGHUP can bypass fails open (`ai/rules/evidence.md`). The fail-closed choice keeps the reload gate in-scope. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (2) LG STAYS EXEMPT from the unauth refusal (intentionally public, read-only, birdwatcher-compatible surface) and instead gets TLS-default-on + optional bearer token. Rationale: refusing unauthenticated LG would break its primary public use case; TLS-default + opt-in token addresses the MEDIUM finding (Task finding 4, Key Design Decisions row) at its severity, without over-reaching to hard-fail. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (3) existing clamp paths (web YANG insecure `loader_extract.go`, MCP loopback clamp `:323-331`, `--insecure-web` flag `ze_core_start.go`) KEEP CLAMPING; the guard then sees only loopback and passes them. Converging them on hard-fail is a NOTED FOLLOW-UP, not this spec. Rationale: smaller self-contained scope, and no fail-open gap remains -- the clamps run BEFORE the guard (Behavior to preserve, above) so a clamped path never presents non-loopback. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (4) gNMI token-over-plaintext hard-fail (token set, no TLS) is a NOTED FOLLOW-UP (Known Limitations), not this spec. Rationale: this spec's guard enforces authentication, not transport secrecy; no regression vs today (the token still authenticates), and TLS-required-for-token is additive/reversible, deferrable without leaving a fail-open gap. Thomas: override if wrong.
- The settings/listener split now covers five services and deliberately stops there. `web` and `api-server` (round 6) joined `mcp`, `looking-glass` and `gnmi` (round 5): each parses its whole block, and only the `enabled` question decides whether a listener starts.
- **MCP is exempt from that split, and the asymmetry is deliberate. Do NOT "finish" it later.** Three legs hold it up, all verified 2026-08-10. (a) `ze.mcp.enabled` synthesizes `127.0.0.1:8080` in `cmd/ze/hub/main.go`, which is STRICTLY NARROWER than any address a config block can name, so today's behavior is already a narrowing rather than a wildcard. (b) The MCP path was never gated behind a boot refusal, so honoring the block's settings would not remove one. (c) Copying the gNMI shape would let a dormant block MOVE MCP to a routable address, and the guard would pass it whenever that block also carries a token. Web and api-server fall on the other side of that line because their defaults are `0.0.0.0`. The residue MCP keeps -- a dormant block naming port 9000 gets 8080 -- is functional, not exposure, and is recorded in `plan/journal/enabled-gate-discards-settings.md`.
- Shared-file coordination: the in-flight bcrypt spec also edits `cmd/ze/hub/service_web.go` (R-3). This spec does not modify `startWebServer` internals.
- Feeds `plan/spec-release-audit-1-surface-inventory.md` as verified evidence for the management-surface inventory.
- **Closure mechanics: move the spec claim to THIS spec for both `commit_helper.py create` calls, and restore it after.** `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` is UNTRACKED and carries two rows, this spec's and `fixit-eap-tls-clienthello-race`'s. Committing an untracked file makes every row read as added, so `_journal_added_spec_stems` (`scripts/dev/commit_helper.py`) returns two stems and `spec_closure_stem` reaches its claim tie-break: `if len(stems) > 1` consults `claimed_spec(repo)`, and with the session claiming a different spec it falls through to `stems[0]`. That returns this spec only because its row happens to sort first in all three journal files the closure commit carries, and `_journal_added_spec_stems` reads them in path order. One appended row from the eap session breaks it, and the review gate then fires for a spec nobody is closing. The claim is the only input that makes the answer deliberate. Move it with `scripts/dev/spec-session.sh claim`, and restore the session's own claim once both commits are prepared.

### Integration Checklist

<!-- Answered at closure (/ze-close step 1). The spec predates the template row
     that carries this table, so it is filled here against the shipped diff. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/lg/yang/ze-lg-conf.yang`: `tls` default flipped to true, new `token` leaf (`ze:sensitive`). Landed in an earlier phase and already committed |
| YANG validation constraints | N-A | The leaves added are a boolean and an opaque secret string; no range, length or enumeration applies |
| YANG custom validators | Yes | `GNMIListenConfig.Validate` (`internal/component/config/loader_extract.go`) wired into `ValidateSemantics` (`internal/component/config/validate_semantic.go`) and `ze config validate` (`internal/component/config/cli/cmd_validate.go`) |
| CLI commands/flags | No | No flag added. `--insecure-web` keeps its existing clamp (`cmd/ze/ze_core_start.go`) and the guard reads the result |
| CLI grammar (keyword before value) | N-A | No new CLI verb or leaf-value pair |
| Editor autocomplete | Yes, automatic | `tls` is a YANG boolean and `token` a string, so the editor completes both from the schema with no `CompleteFn` |
| Functional test for new RPC/API | Yes | 13 `test/plugin/mgmt-guard-*.ci`, `test/plugin/lg-token-gate.ci`, `test/plugin/lg-tls-default-on.ci`, `test/ui/doctor-gnmi-mcp-exposure.ci`, `test/reload/mgmt-guard-reload-refuses-nonloopback.ci` |
| Pipe completeness | N-A | The guard writes one refusal line to stderr before any command surface exists |
| Env var registration | Yes | `ze.looking-glass.token` (Secret) in `internal/component/config/environment.go`, beside the existing `ze.gnmi.token`. Committed in an earlier phase |
| Doctor check for runtime dependencies | Yes | `config-gnmi-invalid` registered in `internal/core/diagnostic/codes.go`; `"gnmi"` listener default in `internal/component/config/listener_defaults.go`; fallback entry in `internal/component/doctor/checks_listener.go`. Proven by `TestDoctorFlagsGnmiExposure`, `TestDoctorGnmiListenerIsProbed`, `TestDoctorGnmiDefaultEndpointIsProbed` |
| Prometheus counters/metrics | No | The guard produces a boot decision, not observable running state. A refused boot leaves no process to scrape |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface is touched |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/authentication.md`, "Remote management listener guard" and "Upgrading from a release without the guard". Both written; the second is the R-1/R-2 upgrade-breaking entry the Quality Gate asks for |
| 2 | Config syntax changed? | No | No leaf was added or renamed in this phase. The settings/listener split changes WHEN a block's leaves are read, never how they are spelled. `grep -rn "loader_extract.go" docs/` returns no source anchor outside `docs/guide/authentication.md`, which this closure updated |
| 3 | CLI command added/changed? | No | No verb, flag or exit-code change. `ze start` keeps exit 1 for a refusal, the code the API precedent already used |
| 4 | API/RPC added/changed? | No | `resolveAPIListeners` moves WHERE the REST/gRPC listen address and token are resolved. No request, response or route changes; `docs/architecture/api/commands.md` carries no anchor at `cmd/ze/hub/api_infra.go` |
| 5 | Plugin added/changed? | No | The guard is always-on hub code and imports no gnmi/mcp/web/lg package |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md`, updated with four `<!-- source: -->` anchors naming `mgmt_guard.go`, `api_infra.go`, `service_lg.go` and `main.go` |
| 7 | Wire format changed? | N-A | No protocol encoding is touched |
| 8 | Plugin SDK/protocol changed? | No | `ServiceDeps` gained `LGToken`, an internal hub struct with no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs a vendor management listener's bind policy |
| 10 | Test infrastructure changed? | No | The new tests join `test/plugin`, `test/ui` and `test/reload` under existing runners. No new suite directory, no runner change |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares protocol and feature support, not management-listener hardening |
| 12 | Internal architecture changed? | Yes, recorded in the spec | The single-guard-point boot order is described in Data Flow above. `docs/architecture/core-design.md` describes component tiers, which are unchanged; the boot-order detail lives with the guard in `docs/guide/authentication.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | See the Integration Checklist row |

`make ze-doc-test` was not run by this closure agent: the instruction for this
run forbids every suite except one package's `go test`. The doc edits carry
`<!-- source: path -- symbol -->` anchors and each anchor's file exists in the
commit. The main thread owns the doc-gate run.

---

## Implementation Summary

### What Was Implemented

The whole spec shipped over nine phases; the last three rounds are what this
commit carries, and the earlier phases are already in git history.

- **The guard.** `checkMgmtListeners` and `listenAddrIsNonLoopback`
  (`cmd/ze/hub/mgmt_guard.go`) refuse a boot in which any management listener is
  both non-loopback and unauthenticated. One call site in `runYANGConfig`
  (`cmd/ze/hub/main.go`), after every resolution and before `eng.Start`, so
  nothing has bound when the refusal prints. `apiHasNonLoopback` was folded in,
  not copied.
- **Resolution hoisted always-on.** `resolveGNMIListeners`
  (`cmd/ze/hub/gnmi_infra.go`) and `resolveAPIListeners`
  (`cmd/ze/hub/api_infra.go`) resolve enable, address, token and TLS outside the
  build-tag-gated builders, so the guard sees every pair before a factory runs.
  `service_gnmi.go` consumes the resolved values instead of resolving its own.
- **The settings/listener split, five services.** `extractGNMIBlock`,
  `extractAPIBlock` and `extractWebBlock`
  (`internal/component/config/loader_extract.go`) now parse a whole block, and
  only the `enabled` question decides whether a listener starts.
  `ExtractGNMISettings`, `ExtractAPISettings` and `ExtractWebSettings` expose the
  settings half; `ExtractGNMIConfig` and `ExtractAPIConfig` keep the old
  enable-gated shape. MCP is deliberately exempt (Notes above).
- **Semantic parity.** `GNMIListenConfig.Validate` reached from
  `ValidateSemantics` and `ze config validate`; `config-gnmi-invalid` registered
  in `internal/core/diagnostic/codes.go`; `"gnmi"` added to the doctor listener
  defaults and to the hardcoded fallback.
- **Looking glass.** TLS default flipped on, optional bearer token over the mux
  with a constant-time compare, `ze.looking-glass.token` registered Secret.
- **Reload gate (AC-7).** `ReloadListeners` (`cmd/ze/hub/listener_migrate.go`)
  runs the same classifier before applying a migration, so a SIGHUP cannot move
  an unauthenticated listener onto a routable address.

### Bugs Found/Fixed

| Bug | Producer | Test that now covers it |
|-----|----------|-------------------------|
| MCP `auth-mode none` plus a token read as authenticated while the server built the accept-all `noneAuthenticator` | `internal/component/mcp/streamable.go`, `bearer.go` | `TestMcpListenerAuthenticated`, `TestMcpAuthModeAuthenticates` |
| `ExtractGNMIConfig` dropped the token and the TLS paths of a block without `enabled true`, so an env-started gNMI listener was refused over a token the operator had written | `internal/component/config/loader_extract.go` | `TestExtractGNMISettingsSurviveDisabledBlock`, `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` |
| `ExtractAPIConfig` read the gRPC `tls-cert`/`tls-key` pair inside the `enabled == configTrue` branch, so `ze.api-server.grpc.enabled` built an authenticated gRPC server with empty TLS paths and served the token in clear | `internal/component/config/loader_extract.go` | `TestResolveAPIListenersKeepsGRPCTLSFromDormantBlock` (red with the resolver pointed back at `ExtractAPIConfig`) |
| The same extractor returned not-ok when neither transport said `enabled true`, dropping the shared token and every address the block named | `internal/component/config/loader_extract.go` | `TestExtractAPISettingsSurviveDisabledTransports`, `TestResolveAPIListenersKeepsSettingsFromDormantBlock` |
| `extractWebBlock` gated the address on `enabled`, so `resolveWebListeners` published `0.0.0.0:3443` over the address the block named, and the refusal quoted an address written in no config file | `internal/component/config/loader_extract.go` | `TestResolveWebListenersClosesTheGuardHole`, `test/plugin/mgmt-guard-web-env-started-address-survives.ci`, `test/plugin/mgmt-guard-web-env-started-address-binds.ci` |
| `resolveAPIListeners` named unregistered env keys `ze.api.rest.listen` / `ze.api.grpc.listen` in its operator-facing errors | `cmd/ze/hub/api_infra.go` | the registered spellings now match `env.MustRegister` (`internal/component/config/environment.go`); read by the round-7 review |
| The dropped `environment.web insecure` leaf was silently discarded, so an operator who wrote it got authentication with no diagnostic | `cmd/ze/hub/main.go` | `test/plugin/mgmt-guard-web-dormant-insecure-warns.ci` |

### Documentation Updates

- `docs/guide/authentication.md`, new section "Upgrading from a release without
  the guard" (49 lines), carrying four `<!-- source: -->` anchors:
  `cmd/ze/hub/mgmt_guard.go`, `cmd/ze/hub/api_infra.go`,
  `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/main.go`. It is the R-1 and R-2
  upgrade-breaking entry; this repository has no release-notes surface.
- `make ze-doc-test` not run by this agent (suite runs forbidden for this run).
  The main thread owns that gate.

### Deviations from Plan

- **MCP is exempt from the settings/listener split.** The spec's Files to Modify
  implies parity across services. It is deliberate and must not be "finished"
  later: `ze.mcp.enabled` synthesizes `127.0.0.1:8080`, strictly narrower than
  any address a block can name, so copying the split would let a dormant block
  MOVE MCP to a routable address the guard passes whenever that block also
  carries a token. Recorded in Notes and in
  `plan/journal/enabled-gate-discards-settings.md`.
- **`plan/learned/NNN-<name>.md` was not written.** That form was retired;
  `ai/rules/planning.md` replaced it with journal rows, and three exist
  (below).
- **QEMU boot test.** The Alpine-VM pass over the new `.ci` runs through
  `make ze-qemu-all-test`, which this run is forbidden to start. The refusal and
  the remedied boot are proven natively by the `.ci` above; the tests are
  config-and-refusal only and carry no `option=needs-linux`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The guard was built first and the extractors were assumed to deliver every block's settings | Five extractors dropped auth and address settings at the `enabled` gate, so the guard refused boots over credentials the operator had written | Round 4 through round 7 review passes, each finding the next service with the same shape | Split every one of the five; recorded the class in `plan/journal/enabled-gate-discards-settings.md` so the sixth is recognised on sight |
| approach | A discrimination run for `mgmt-guard-web-dormant-insecure-warns` was read as "the test does not discriminate" | The build command was `make <session-bin-path>/ze`, an explicit path with no Makefile rule. `make` did nothing, exited 0, and the run used a stale binary that still carried the WARN | Re-run with `make ze`, plus `strings` on the binary as a control BEFORE the run | The control is now checked before the result is read. Stated for the next author in the Functional Tests section above |
| escalation | Two `test/plugin` tests hardcoded the same MCP port with no exclusive group | A parallel suite run failed one of them with `address already in use`, and a third bound the wildcard `0.0.0.0:9339` | A parallel run of the suite | Ports moved and the group joined; the deterministic-port allocator still does not reach `.ci` literals, recorded in `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. [BLOCKER] gNMI unauthenticated read+write on `0.0.0.0` | Done | `cmd/ze/hub/mgmt_guard.go` `checkMgmtListeners`; `cmd/ze/hub/gnmi_infra.go` `resolveGNMIListeners` | Boot refuses; `GNMIListenConfig.Validate` also reports it offline |
| 2. [HIGH] MCP fail-closed guard does not run at daemon startup | Done | `cmd/ze/hub/main.go` calls `MCPListenConfig.Validate` on the `ze start` path; `mcpListenerAuthenticated` (`cmd/ze/hub/mgmt_guard.go`) covers the env path | The existing precise message is reused, not re-worded |
| 3. [HIGH] `--insecure-web` clamp bypassable via `ze.web.insecure` | Done | `cmd/ze/hub/main.go` web declaration; `resolveWebListeners` | Non-loopback plus insecure refuses; a dormant block's `insecure` is dropped with a WARN naming the leaf |
| 4. [MEDIUM] Looking Glass unauthenticated with TLS optional | Done | `internal/component/lg/server.go` token middleware; `internal/component/lg/yang/ze-lg-conf.yang` `tls` default true | LG stays exempt from the unauth refusal by design (Key Design Decisions) |
| 5. [RESOLVED elsewhere] MCP auth settings discarded when the block was not enabled | Done, inherited nothing | fixed in `spec-mcp2026-1-stateless-core` | The guard still covers the case through `mcpListenerAuthenticated` |
| Unifying fix: one guard, one call site, before any management bind | Done | `cmd/ze/hub/main.go` `runYANGConfig` | Single classifier; `apiHasNonLoopback` folded in |
| gNMI semantic validation and doctor entries | Done | `internal/component/config/validate_semantic.go`, `internal/component/config/cli/cmd_validate.go`, `internal/component/config/listener_defaults.go`, `internal/component/doctor/checks_listener.go`, `internal/core/diagnostic/codes.go` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci`, `test/plugin/mgmt-guard-gnmi-token-allowed.ci`, `TestCheckMgmtListeners`, `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` | Includes the `0.0.0.0:9339` default |
| AC-2 | Done | `test/plugin/mgmt-guard-mcp-bind-remote-none-refused.ci` (a), `test/plugin/mgmt-guard-mcp-env-nonloopback-refused.ci` (b), `TestMcpListenerAuthenticated` | (a) keeps the existing `environment.mcp: bind-remote requires auth-mode != none` text |
| AC-3 | Done | `test/plugin/mgmt-guard-web-insecure-env-refused.ci`, `test/plugin/mgmt-guard-web-env-started-address-survives.ci`, `TestResolveWebListenersClosesTheGuardHole` | The refusal quotes the address the block named |
| AC-4 | Done | `test/plugin/mgmt-guard-loopback-allowed.ci`, `test/plugin/mgmt-guard-api-env-started-settings-survive.ci`, `TestCheckMgmtListeners` | Loopback boots; guard logs nothing |
| AC-5 | Done | `test/plugin/lg-tls-default-on.ci`, `test/plugin/lg-token-gate.ci`, `TestExtractLGConfigTLSDefaultOn`, `TestLGTokenMiddleware`, `TestLGWithoutTokenStaysOpen` | Explicit `tls true` without blob storage is an error; the inherited default warns and serves plaintext |
| AC-6 | Done | `test/ui/doctor-gnmi-mcp-exposure.ci`, `TestValidateSemanticsFlagsGNMI`, `TestDoctorFlagsGnmiExposure`, `TestDoctorGnmiDefaultEndpointIsProbed` | The `TestDoctorDependencyInventory` residual was closed 2026-08-07 (deferral row 3) |
| AC-7 | Done | `test/reload/mgmt-guard-reload-refuses-nonloopback.ci`, `TestReloadListenersRefusesUnauthNonLoopback`, `TestReloadListenersAllowsAuthenticatedNonLoopback` | Daemon keeps serving on the previous addresses |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestMgmtGuardRefusesNonLoopbackUnauth` / `...AllowsLoopback` / `...AllowsAuthenticatedNonLoopback` | Changed | `TestCheckMgmtListeners` (`cmd/ze/hub/mgmt_guard_test.go`) | One table-driven test replaced three; every case is present |
| `TestMgmtGuardUnparseableHostIsNonLoopback` | Changed | `TestListenAddrIsNonLoopback` (`cmd/ze/hub/mgmt_guard_test.go`) | Renamed to the function it drives |
| `TestGNMIListenConfigValidate` | Done | `internal/component/config/gnmi_validate_test.go` | Plus `TestGNMIListenConfigValidateMultiListener` |
| `TestValidateSemanticsFlagsGNMI` | Done | `internal/component/config/validate_semantic_test.go` | Driven from the semantic entry point, not the helper |
| `TestExtractLGConfigTLSDefaultOn` | Done | `internal/component/config/lg_extract_test.go` | |
| `TestLGTokenMiddleware` | Done | `internal/component/lg/auth_test.go` | Plus `TestLGWithoutTokenStaysOpen` |
| `TestReloadListenersRefusesUnauthNonLoopback` | Changed | `cmd/ze/hub/mgmt_guard_test.go` | Lives with the classifier it drives, not in `listener_migrate_test.go` |
| `TestDoctorFlagsGnmiExposure` | Done | `internal/component/doctor/checks_config_test.go` | |
| `TestExtractGNMISettingsSurviveDisabledBlock` | Done | `internal/component/config/gnmi_extract_test.go` | Mutation-verified |
| `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` / `...WithEnvListenAddress` | Done | `cmd/ze/hub/gnmi_infra_test.go` | Red before the fix |
| `TestExtractAPISettingsSurviveDisabledTransports` | Done | `internal/component/config/api_extract_test.go` | Red before the fix |
| `TestResolveAPIListenersKeepsSettingsFromDormantBlock` / `...KeepsGRPCTLSFromDormantBlock` | Done | `cmd/ze/hub/api_infra_test.go` | Red before the fix |
| 17 functional `.ci` (Functional Tests table) | Done | `test/plugin`, `test/ui`, `test/reload` | All present on disk; see Files Exist below |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/hub/main.go` | Done | Guard call site, hoisted resolutions, boot-time `Validate`, web settings branch and its WARN |
| `cmd/ze/hub/api.go` | Changed | `apiHasNonLoopback` folded into `listenAddrIsNonLoopback`; the resolution moved to the new `api_infra.go` rather than staying here |
| `cmd/ze/hub/service_gnmi.go`, `cmd/ze/hub/gnmi_infra.go` | Done | Resolution hoisted; the builder consumes plain values |
| `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/service_registry.go` | Done | LG token threaded as a plain string, no `lg` import |
| `cmd/ze/hub/listener_migrate.go` | Done | AC-7 gate |
| `cmd/ze/hub/api_infra.go` | Done | New file, not in the original Files to Create: `resolveAPIListeners` and `apiGuardAddrs` |
| `internal/component/config/loader_extract.go` | Done | Five extractor splits, `GNMIListenConfig.Validate`, LG TLS default and token |
| `internal/component/config/validate_semantic.go`, `.../cli/cmd_validate.go` | Done | gNMI wiring |
| `internal/component/config/listener_defaults.go`, `internal/component/doctor/checks_listener.go` | Done | `"gnmi"` default and fallback entry |
| `internal/core/diagnostic/codes.go` | Done | `config-gnmi-invalid` |
| `internal/component/config/environment.go` | Done | `ze.looking-glass.token`, Secret |
| `internal/component/lg/server.go`, `internal/component/lg/yang/ze-lg-conf.yang` | Done | Token middleware, `tls` default true |
| `cmd/ze/hub/mgmt_guard.go`, `mgmt_guard_test.go` | Done | Created |
| `cmd/ze/hub/gnmi_infra_test.go`, `api_infra_test.go` | Done | Created |
| `internal/component/config/gnmi_extract_test.go`, `api_extract_test.go` | Done | Created |
| `test/plugin/mgmt-guard-*.ci`, `test/ui/doctor-gnmi-mcp-exposure.ci`, `test/reload/mgmt-guard-reload-refuses-nonloopback.ci`, `test/plugin/lg-tls-default-on.ci` | Done | 13 `mgmt-guard-*` rather than the 6 the plan foresaw; each extra one pins a service the settings/listener split touched |

### Audit Summary
- **Total items:** 45 (7 requirements, 7 ACs, 13 test rows, 18 file rows)
- **Done:** 41
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (three test renames onto the function they drive, and `apiHasNonLoopback`'s resolution moving to a new file). Recorded in Deviations and in the tables above

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No management listener can be published non-loopback without authentication | functional | `test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci`, `mgmt-guard-mcp-env-nonloopback-refused.ci`, `mgmt-guard-web-insecure-env-refused.ci`, `mgmt-guard-mcp-bind-remote-none-refused.ci` -- each drives `ze` in the foreground and asserts `exit:code=1` plus the refusal text on stderr, so the proof is a refused daemon rather than a unit verdict |
| The refusal names the address the operator actually configured | functional, discrimination proven | `mgmt-guard-web-env-started-address-survives.ci` and `mgmt-guard-web-env-started-address-binds.ci`. The second was proven by removing the three-line address fallback from `runYANGConfig` and rebuilding: it fails on `stderr does not contain "web server listening on https://127.0.0.1:18449/"` and the log shows `https://0.0.0.0:3443/` |
| A config that asks for authentication never silently produces none | functional | `mgmt-guard-gnmi-env-started-token-survives.ci` (red before the fix: boot refusal, exit 1), `mgmt-guard-api-env-started-settings-survive.ci` (REST binds the block's address and authenticates with its token), `mgmt-guard-web-dormant-insecure-warns.ci` (discrimination proven 2026-08-11 with a `strings` control on the binary) |
| Loopback deployments keep working unchanged | functional | `mgmt-guard-loopback-allowed.ci`: unauthenticated gNMI, MCP and web on loopback boot and exit cleanly |
| The exposure is visible offline, not only at boot | functional | `test/ui/doctor-gnmi-mcp-exposure.ci`: `ze doctor --json` emits `config-gnmi-invalid` and `config-mcp-invalid` |
| A SIGHUP cannot resurrect the exposure the boot guard refused | functional | `test/reload/mgmt-guard-reload-refuses-nonloopback.ci`: the migration is refused and the daemon keeps serving the previous addresses |
| The looking glass is hardened without losing its public use case | functional | `test/plugin/lg-tls-default-on.ci` (stderr banner `looking glass listening on https://`), `test/plugin/lg-token-gate.ci` (mutation-verified: no bearer refused, right bearer served) |
| Operators can see the upgrade break before it hits them | documentation | `docs/guide/authentication.md`, "Upgrading from a release without the guard": the three breaking changes, each with its remedy, plus `ze config validate` as the pre-upgrade check |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-07-17: auth-mode changes on a SIGHUP reload do not take effect (servers built once) | done | Fixed 2026-08-07 by `spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild`. `AuthUpdatable` (`cmd/ze/hub/listener_migrate.go`) and `UpdateAuth` on the REST and gRPC servers rebuild credentials on a running server; both directions proven red with the change reverted |
| 2026-07-19: AC-5 LG TLS, AC-6 gNMI validate wiring, and the functional refusal tests deferred to CI | done | `lg-tls-default-on.ci`, `doctor-gnmi-mcp-exposure.ci` and the 13 `mgmt-guard-*.ci` all exist and pass |
| 2026-08-03: `TestDoctorDependencyInventory` needed a `listener/gnmi` row | done | Fixed 2026-08-07: the test now DERIVES the listener half from `config.DiscoverListenerServices`, so a new `ze:listener` service can no longer be missed by a hand-maintained count |

Every row in `plan/deferrals/fixit-mgmt-listener-auth-guard.md` is terminal, so
commit A removes the shard.

One row in a FOREIGN shard names this spec as its home and stays live:
`plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md`, 2026-08-08,
"resolve the API's per-user gate independently of the `environment { ssh { } }`
block". This spec did not take it and does not fix it. `apiUsers`
(`cmd/ze/hub/main.go`) is still `mergeAuthUsers(zefsUsers, sshCfg.Users)`, and
`infra.ExtractSSHConfig` still returns nothing without an `environment/ssh`
container. It is not remotely reachable, because `apiAuthed` is then false and
`checkMgmtListeners` refuses any non-loopback API listener. Closure repoints that
row at the bare stem so it reads as unhomed rather than as pointing at a file the
tree no longer holds; the advisory `deferral_unassigned_problems` warning is then
true and will surface it for a new home.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-mgmt-listener-auth-guard-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean -- exit 0, "OK, 18 code files, clean, hashes match" |
| Rounds | 8. Round 6 fixed the `extractAPIBlock` gRPC TLS-in-clear defect round 5 left open; round 7 fixed operator-facing errors naming unregistered `ze.api.*` env keys; round 8's findings were all record defects, which makes it the last round |
| Reviewer lenses used | security + correctness/wiring (pass 1-2), then a single independent adversarial gate per round (passes 3-8), each read-only over the round's diff |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | MCP fail-open: `auth-mode none` plus a token read as authenticated while the server built the accept-all authenticator | `internal/component/mcp/streamable.go`, `bearer.go` | `mcpListenerAuthenticated` mirrors the server's effective-mode precedence; `TestMcpListenerAuthenticated` covers the none+token case |
| 2 | ISSUE | `GNMIListenConfig.Validate` doc comment claimed validate/doctor/boot wiring that phase 1 had not built | `internal/component/config/loader_extract.go` | Comment corrected to what the phase shipped |
| 3 | BLOCKER | `ExtractGNMIConfig` dropped the token of a block without `enabled true`, so an env-started listener was refused over a token the operator had written | `internal/component/config/loader_extract.go` | `extractGNMIBlock` plus `ExtractGNMISettings`; `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` red before the fix |
| 4 | BLOCKER | `extractAPIBlock` read the gRPC `tls-cert`/`tls-key` pair inside the enabled branch, so an env-started gRPC transport served an authenticated management server in clear | `internal/component/config/loader_extract.go` | `ExtractAPISettings` consumed by `resolveAPIListeners`; `TestResolveAPIListenersKeepsGRPCTLSFromDormantBlock` red with the resolver pointed back at `ExtractAPIConfig` |
| 5 | ISSUE | `resolveAPIListeners` named unregistered env keys `ze.api.rest.listen` / `ze.api.grpc.listen` in operator-facing errors | `cmd/ze/hub/api_infra.go` | Corrected to the `ze.api-server.*` spellings `env.MustRegister` carries (`internal/component/config/environment.go`) |
| 6 | ISSUE | `extractWebBlock` gated the address on `enabled`, so the refusal quoted `0.0.0.0:3443` instead of the address the block named | `internal/component/config/loader_extract.go`, `cmd/ze/hub/main.go` | `ExtractWebSettings` branch in `runYANGConfig`; `mgmt-guard-web-env-started-address-survives.ci` and `...-binds.ci` |
| 7 | ISSUE | The round-7 `insecure not honored` WARN was new operator-visible output with no test driving it | `cmd/ze/hub/main.go` | `test/plugin/mgmt-guard-web-dormant-insecure-warns.ci`, discrimination proven with a `strings` control on the rebuilt binary |

NOTEs from the final round (record only, all corrected before this commit): the
Functional Tests table had prose inserted between two rows, orphaning the last
row; one date disagreed with the row it described; `### Integration Checklist`
and `### Documentation Update Checklist` were absent. All three are fixed above.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/mgmt_guard.go`, `mgmt_guard_test.go` | Yes | `ls` at closure, 2026-08-11 |
| `cmd/ze/hub/api_infra.go`, `api_infra_test.go`, `gnmi_infra.go`, `gnmi_infra_test.go` | Yes | `ls` at closure, 2026-08-11 |
| `internal/component/config/api_extract_test.go`, `gnmi_extract_test.go`, `gnmi_validate_test.go`, `lg_extract_test.go` | Yes | `ls` at closure, 2026-08-11 |
| 13 `test/plugin/mgmt-guard-*.ci` | Yes | `ls test/plugin/mgmt-guard-*.ci` returns 13 paths |
| `test/plugin/lg-token-gate.ci`, `test/plugin/lg-tls-default-on.ci` | Yes | `ls` at closure, 2026-08-11 |
| `test/ui/doctor-gnmi-mcp-exposure.ci`, `test/reload/mgmt-guard-reload-refuses-nonloopback.ci` | Yes | `ls` at closure, 2026-08-11 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | gNMI non-loopback without a token refuses the boot | `TestCheckMgmtListeners`, `TestResolveGNMIListenersKeepsTokenFromDisabledBlock` present in `cmd/ze/hub`; `make ze-test-pkg PKG=./cmd/ze/hub` green (main thread, 2026-08-11) |
| AC-2 | MCP bind-remote without auth, and env-listen without a token, both refuse | `TestMcpListenerAuthenticated`, `TestMcpAuthModeAuthenticates` present and green in the same run |
| AC-3 | web-insecure plus non-loopback refuses, quoting the configured address | `TestResolveWebListenersClosesTheGuardHole` present and green in the same run |
| AC-4 | loopback keeps booting; dormant blocks supply their own settings | `make ze-test-pkg PKG=./internal/component/config` exit 0, 2026-08-11 (covers `TestExtractAPISettingsSurviveDisabledTransports`, `TestExtractGNMISettingsSurviveDisabledBlock`, `TestExtractAPIConfigStillGatesOnEnabled`) |
| AC-5 | LG TLS default on, token gate optional | `TestExtractLGConfigTLSDefaultOn`, `TestExtractLGConfigTLSExplicitFlag`, `TestExtractLGConfigToken` in the same green `./internal/component/config` run |
| AC-6 | doctor and `ze config validate` report the exposure | `TestValidateSemanticsFlagsGNMI`, `TestGNMIListenConfigValidate`, `TestGNMIListenConfigValidateMultiListener` in the same green run |
| AC-7 | a reload cannot move an unauthenticated listener non-loopback | `TestReloadListenersRefusesUnauthNonLoopback`, `TestReloadListenersAllowsAuthenticatedNonLoopback` present in `cmd/ze/hub` and green in the main thread's run |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| config binds gNMI `0.0.0.0` with no token | `test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci` | Yes -- foreground `ze`, `expect=exit:code=1` plus the refusal text |
| `ze.mcp.listen` routable env, no token | `test/plugin/mgmt-guard-mcp-env-nonloopback-refused.ci` | Yes -- per-command `env=` directive |
| `ze.web.insecure` with a non-loopback listen | `test/plugin/mgmt-guard-web-insecure-env-refused.ci` | Yes |
| `mcp { bind-remote true; auth-mode none }` | `test/plugin/mgmt-guard-mcp-bind-remote-none-refused.ci` | Yes -- asserts the existing `Validate` message |
| loopback gNMI/MCP/web without auth | `test/plugin/mgmt-guard-loopback-allowed.ci` | Yes -- joins `option=exclusive:group=mgmt-guard` after the port collision |
| gNMI non-loopback WITH token | `test/plugin/mgmt-guard-gnmi-token-allowed.ci` | Yes |
| `ze doctor --json` on an exposing config | `test/ui/doctor-gnmi-mcp-exposure.ci` | Yes |
| SIGHUP moves an unauth listener non-loopback | `test/reload/mgmt-guard-reload-refuses-nonloopback.ci` | Yes |
| `ze.gnmi.enabled` plus a dormant block naming a loopback `server` | `test/plugin/mgmt-guard-gnmi-env-started-address-survives.ci` | Yes -- red before the fix, bound `[::]:9339` |
| `ze.gnmi.enabled` plus a dormant block naming a `token` | `test/plugin/mgmt-guard-gnmi-env-started-token-survives.ci` | Yes -- red before the fix, exit 1 |
| `ze.looking-glass.enabled` plus a dormant block naming a loopback `server` | `test/plugin/mgmt-guard-lg-env-started-address-survives.ci` | Yes -- red before the fix, bound `http://[::]:8443/` |
| `ze.looking-glass.token` leaf or env var | `test/plugin/lg-token-gate.ci` | Yes -- mutation-verified |
| `ze.web.enabled` plus a dormant block naming a loopback `server` | `test/plugin/mgmt-guard-web-env-started-address-binds.ci` | Yes -- discrimination proven 2026-08-10 |
| `ze.web.enabled` plus a dormant block carrying `insecure true` | `test/plugin/mgmt-guard-web-dormant-insecure-warns.ci` | Yes -- discrimination proven 2026-08-11, control checked with `strings` before the run |
| `ze.web.enabled` plus `ze.web.insecure` plus a routable `server` | `test/plugin/mgmt-guard-web-env-started-address-survives.ci` | Yes |
| `ze.api-server.rest.enabled` plus a dormant block naming address and token | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` | Yes -- red before the fix |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Hard-fail shipped; Thomas confirmed the API-precedent default 2026-07-16, and the refusal `.ci` prove exit 1 with nothing bound |
| A-2 | confirmed | Every surface's address and auth is knowable at the one guard point: `resolveGNMIListeners`, `resolveAPIListeners`, `resolveWebListeners` and the MCP/LG resolutions all complete before `checkMgmtListeners` runs in `runYANGConfig` |
| A-3 | confirmed | `"gnmi"` in `listener_defaults.go` and the doctor fallback caused no schema drift; `TestDoctorGnmiListenerIsProbed` and `TestDoctorGnmiDefaultEndpointIsProbed` are green |
| A-4 | confirmed, with the residual case recorded | SSH, plugin hub, telemetry and the managed server stay out of the declaration set. The Prometheus residual (`server main { ip 0.0.0.0 }` plus no `basic-auth` block) is stated in Known Limitations against its real producer, `extractTelemetryConfig` |
| A-5 | confirmed | `ReloadListeners` did move an address with no auth re-check; AC-7 now gates it, and `spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild` later added the rebuild |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "ze exits with status 1 and prints one line for each offending listener" | `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) returns the collected refusals and `runYANGConfig` exits before `eng.Start` | Yes |
| "An `environment` block without `enabled true` supplies the address, the token and the TLS settings" | `ExtractWebSettings` branch at `cmd/ze/hub/main.go:468`; `resolveAPIListeners` (`cmd/ze/hub/api_infra.go:50`); `resolveGNMIListeners` (`cmd/ze/hub/gnmi_infra.go:24`) | Yes |
| "`environment.web insecure` is the one exclusion" plus its WARN | The WARN string at `cmd/ze/hub/main.go:494`, fired from the `webEnabled && webSettings.Insecure && !insecureWeb && !webAuthFollowsConfig` condition | Yes |
| "The looking glass serves TLS by default ... an explicit `tls true` on such a box is an error" | `TestExtractLGConfigTLSDefaultOn` and `TestExtractLGConfigTLSExplicitFlag` (`internal/component/config/lg_extract_test.go`), green in the closure run | Yes |
| Category 2 (config syntax): no update needed | No leaf added or renamed this phase; `docs/` carries no source anchor at `internal/component/config/loader_extract.go` outside the page this closure edited | Yes |
| Category 9 (RFC status): not applicable | No RFC governs a vendor management listener's bind policy; `rfc/short/` holds no summary this change touches | Yes |

## Core Insight

**An `enabled` gate answers "start a listener", and it was silently answering
"how does this service authenticate" as well.** Five services carried the same
shape, and each one turned the guard into its own opposite: the daemon refused to
boot naming a credential the operator had already written, or served a management
port in clear because the certificate paths sat one branch too deep. The guard
did not create the class, it made it visible, because a fail-closed check is the
first reader that ever compares what a block SAYS with what the daemon RESOLVED.
The split that fixes it is mechanical -- parse the whole block, let `enabled`
decide only whether a listener starts -- and the reason MCP is exempt is the
same reason the class is dangerous: its `enabled` default is NARROWER than any
address a block can name, so honoring the block would move the listener outward.
