# Spec: fixit-mgmt-listener-auth-guard

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

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
| A-4 | Out-of-scope listeners are safe to exclude: SSH (authenticated by protocol + AAA, `main.go`), plugin hub (secrets enforced, min length 32, `loader_extract.go`), telemetry/Prometheus (metrics read-only, default loopback `checks_listener.go`), managed server (hub secrets) | cited producers read 2026-07-16 | A surface is exposed unauthenticated | re-audit rows in Known Limitations; doctor bind probes still cover them | confirmed |
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
- `internal/component/config/loader_extract.go` - add `GNMIListenConfig.Validate`; flip `ExtractLGConfig` TLS default to true (absent leaf reads true); extract LG token leaf
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
9. **Complete spec** -- audit tables, release-notes entry (R-1/R-2), `plan/learned/NNN-<name>.md`, two-commit closure.

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
- gNMI token-over-plaintext (token set, no TLS) still boots: the guard enforces authentication, not transport secrecy. TLS-required-for-token is a possible follow-up (open question 4, resolved 2026-07-17: NOTED FOLLOW-UP, not this spec).
- Auth-mode changes on SIGHUP reload do not take effect (servers are built once); AC-7 only prevents the address from moving into exposure. Full reload-time auth rebuild is out of scope.
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
- [ ] Release-notes entry drafted for the behavior change (boot refusal on upgrade R-1; LG TLS default R-2)

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

Mutation verification: 13 mutations, each reverting one guard, wiring, or
default, and each confirmed to turn its own test red before restore.

## Notes
- Skeleton captured from the 2026-07-16 repository audit; verifier V1 corrected both earlier passes (MCP guard exists but does not run at boot; the insecure-web YANG path is clamped, only the env path bypasses). Deepened to design 2026-07-16: all citations re-verified against the working tree; the doctor claim was corrected (MCP is present in the hardcoded fallback at `checks_listener.go`; the schema path already discovers gNMI; the actual gaps are the missing "gnmi" listener default, the missing gNMI semantic Validate, and bind-probe-vs-exposure semantics).
- Open questions for Thomas: (1) confirm AC-7 (reload-migration gate) stays in this spec vs a follow-up; (2) confirm LG's exemption from the unauth refusal (public looking glass) plus TLS-default-on is the intended posture for finding 4; (3) whether existing clamp paths (web YANG insecure, MCP no-bind-remote, --insecure-web flag) should eventually converge on hard-fail too -- this spec keeps them clamping, which the guard then passes; (4) gNMI token-over-plaintext follow-up (Known Limitations).
  - → AUTONOMOUS DEFAULT (2026-07-17): (1) AC-7 (reload-migration gate) STAYS IN THIS SPEC. Rationale: `ReloadListeners` (`cmd/ze/hub/listener_migrate.go`, verified 2026-07-17 -- extracts new addrs and reconfigures web/lg/mcp/rest/grpc with no auth re-check) can move a running unauthenticated listener non-loopback after boot; a guard a SIGHUP can bypass fails open (`ai/rules/evidence.md`). The fail-closed choice keeps the reload gate in-scope. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (2) LG STAYS EXEMPT from the unauth refusal (intentionally public, read-only, birdwatcher-compatible surface) and instead gets TLS-default-on + optional bearer token. Rationale: refusing unauthenticated LG would break its primary public use case; TLS-default + opt-in token addresses the MEDIUM finding (Task finding 4, Key Design Decisions row) at its severity, without over-reaching to hard-fail. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (3) existing clamp paths (web YANG insecure `loader_extract.go`, MCP loopback clamp `:323-331`, `--insecure-web` flag `ze_core_start.go`) KEEP CLAMPING; the guard then sees only loopback and passes them. Converging them on hard-fail is a NOTED FOLLOW-UP, not this spec. Rationale: smaller self-contained scope, and no fail-open gap remains -- the clamps run BEFORE the guard (Behavior to preserve, above) so a clamped path never presents non-loopback. Thomas: override if wrong.
  - → AUTONOMOUS DEFAULT (2026-07-17): (4) gNMI token-over-plaintext hard-fail (token set, no TLS) is a NOTED FOLLOW-UP (Known Limitations), not this spec. Rationale: this spec's guard enforces authentication, not transport secrecy; no regression vs today (the token still authenticates), and TLS-required-for-token is additive/reversible, deferrable without leaving a fail-open gap. Thomas: override if wrong.
- Shared-file coordination: the in-flight bcrypt spec also edits `cmd/ze/hub/service_web.go` (R-3). This spec does not modify `startWebServer` internals.
- Feeds `plan/spec-release-audit-1-surface-inventory.md` as verified evidence for the management-surface inventory.
