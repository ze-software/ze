# Spec: feature-gate-3-web

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | plan/spec-feature-gate-0-umbrella.md (registry + tag-wiring pattern); learned 980 (lg registry), 981 (ssh seam) |
| Phase | 1/5 |
| Updated | 2026-06-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/980-feature-gate-1-lg.md` - the construction-registry pattern this mirrors
4. `plan/learned/981-feature-gate-2-ssh.md` - extract-then-gate, four-place tag wiring, dep_audit DISABLEABLE
5. `cmd/ze/hub/service_registry.go`, `service_lg.go`, `register_lg.go` - the registry template
6. `cmd/ze/hub/main_servers.go` (startWebServer), `listener_migrate.go`, `internal/component/web/server.go` (cert helpers)

## Task

Make the **web UI service compile-out-able** from the `ze` binary via a `ze_web`
build tag, for a smaller binary and a smaller attack surface. Web is child 3 of the
feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`), after the lg pilot
(child 1, learned 980) and ssh (child 2, learned 981).

Web fits the **construction registry** (lg's pattern), NOT a bespoke seam: it has no
reactor-lifecycle callbacks, but in the current daemon path it is built after the engine
and plugin server so the dispatcher and command metadata already exist. The complication
is that the web package today exports shared, non-UI utilities (self-signed TLS cert
generation and a listener-diff helper) that are consumed by code which builds with web
OFF: the appliance installer, the `init` plugin, and lg's gated factory. So web
requires **extract-then-gate** (the ssh approach): Phase 1 extracts those utilities to
an always-on home so nothing always-on imports the web package for them; Phase 2
registry-izes the web server behind `ze_web`.

User decision (SCOPE gate): one spec, two phases. User decision (DESIGN gate): the
extracted cert/TLS surface lands in a new always-on leaf `internal/core/selfcert`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/980-feature-gate-1-lg.md` - the construction-registry pattern
  → Constraint: registry lives in `package hub` (`service_registry.go`): `Service`
    (`Reconfigurable`+`Name`+`Shutdown`), `ServiceDeps` (generic deps, never a service
    type), `registerService`/`buildServices`/`registerBuiltService`. The feature factory
    + adapter live in `service_<x>.go` (`//go:build ze_<x>`); registration `init()` in
    `register_<x>.go`.
  → Constraint: four-place tag wiring -- `ZE_FEATURES` (Makefile), `.golangci.yml`
    build-tags, `TestBuildTags()` (`internal/test/runner/runner.go`), `featureTags`
    (generator). Missing the `TestBuildTags` entry is the trap: the functional-test
    runner builds its OWN `ze`, so every web `.ci` fails with "unknown field" until
    `ze_web` is added there.
  → Constraint: generator gates the YANG schema: `featureTags` maps `web/yang -> ze_web`,
    emits `all_ze_web.go` (`//go:build ze_web`), removes web/yang from the flat `all.go`.
  → Constraint: feature-only helpers must move INTO the gated file or a no-web build
    flags them U1000-unused.
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - extract-then-gate, DISABLEABLE gate
  → Constraint: `dep_audit.py` `DISABLEABLE` map (pkg -> tag); `--check` fails if any
    always-on (untagged, non-test) file imports a disableable package. Web cannot enter
    DISABLEABLE until Phase 1 removes every always-on import of `internal/component/web`.
  → Decision: ze-stripped keeps ssh as the management plane but is otherwise minimal;
    web is an optional UI, so ze-stripped (`ze_core ze_ssh`) drops web.
  → Constraint: a unit/functional test that requires the feature must be gated
    `//go:build ze_web` or skip-guarded with a `// test-relax:` comment under ze_core.
- [ ] `ai/rules/module-tiers.md` - tier placement for the new selfcert leaf
  → Constraint: `internal/core/*` is the leaf tier (no config/component deps inward).
    `selfcert` deps are stdlib crypto + `slogutil` only, so it sits correctly at core;
    `make ze-tier-check` must stay green.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with `ze_web` off, removing the web blank import must drop ALL web code;
    no web spelling may remain in generic/always-on packages.
- [ ] `docs/architecture/web-interface.md` - graceful listener migration
  → Constraint: `ListenerMigrator.ReloadListeners` drives web via the `Reconfigurable`
    contract; `SetWeb` must widen from `*zeweb.WebServer` to `Reconfigurable` (as `SetLG`
    already did) so always-on code never names the web server type.

### RFC Summaries (MUST for protocol work)
- N/A. Web compile-out is a composition/build-tag change; no wire-protocol behavior.

**Key insights:**
- The ~51 `zeweb.*` call-sites in `main_servers.go` are concentrated inside one
  self-contained function, `startWebServer()` (plus helpers `webOnlyDispatcher`,
  `withBGPDecode`, `wireEventRingToBroker`). That whole function moves into a gated
  `service_web.go` as a unit; the zefs-backed `blobCertStore` stays in an always-on hub
  file because lg still needs it when `ze_lg` is on and `ze_web` is off.
- The genuinely cross-cutting work is Phase 1 (utility extraction), not the registry-ize.
- `internal/component/web/server.go` holds three utility groups consumed with web off:
  cert generation (`GenerateWebCert*`, `addInterfaceIPs`), TLS config (`NewTLSConfig`),
  cert persistence (`CertStore`, `LoadOrGenerateCert`), and `ListenerDiff`. All but
  `ListenerDiff` move to `internal/core/selfcert`; `ListenerDiff` becomes a migrator-local
  copy (matching the private copies rest/grpc/lg/mcp already carry).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main_servers.go` - `startWebServer()` (~line 231) builds the entire web
  stack: cert load (`zeweb.LoadOrGenerateCert`), renderer, decorator registry, ASN
  decorator, `NewWebServer`, editor manager, session store, event broker, ~30 route
  handlers (`HandleWorkbench`, `HandleConfig*`, `HandleCLI*`, `HandleAdmin*`, L2TP/ISIS/OSPF
  handlers, portal), auth/CSRF middleware, then `ListenAndServe`. Helpers
  `webOnlyDispatcher`, `withBGPDecode`, and `wireEventRingToBroker` also live here.
  The `blobCertStore` adapter is shared with lg and must not move into a `ze_web`-only
  file.
  → Constraint: all web orchestration moves into the gated factory. No web logic is
    rewritten in Phase 2.
- [ ] `internal/component/web/server.go` - holds `WebServer` (gated target) AND the
  always-on-consumed utilities: `CertStore` (line 56), `WebConfig`/`NewWebServer` (uses
  `NewTLSConfig` at 145), `ListenerDiff` (264), `GenerateWebCert`/`WithAddr`/`WithNames`
  + `addInterfaceIPs` (452-566), `NewTLSConfig` (570), `LoadOrGenerateCert` (587).
  → Constraint: the cert/TLS surface depends only on stdlib crypto + the `web.server`
    logger; it extracts cleanly. `NewWebServer` calls `NewTLSConfig`, so after extraction
    the gated web package imports `selfcert` (gated → always-on is allowed).
- [ ] `cmd/ze/hub/listener_migrate.go` - always-on. `NewListenerMigrator(web *zeweb.WebServer)`
  (line 48) and `SetWeb(web *zeweb.WebServer)` (56) name the web type; `buildChange`
  (173) calls `zeweb.ListenerDiff`. The `web` field is already typed `Reconfigurable`.
  → Constraint: widen the constructor + `SetWeb` to `Reconfigurable` (mirror `SetLG`);
    replace the `zeweb.ListenerDiff` call with a migrator-local `listenerDiff`.
- [ ] `cmd/ze/hub/main.go` - always-on references to web: `RunWebOnly()` (the no-bgp,
  web-only daemon mode) builds web directly (~line 113-132); `zeweb.RegisterPortalService`
  (567); `var webEditorMgr *zeweb.EditorManager` (670); `blobCertStore` methods (1105-1113).
  → Constraint: `RunWebOnly` is a SECOND web construction path (like ssh's standalone
    path in main.go). It must be gated behind the seam so a no-web build returns a clean
    "web not compiled in" error, not a build break. The portal-registration and
    editor-manager references must move into the gated path too.
- [ ] `internal/appliance/cmd_init.go` (line 296), `cmd_cert.go` (92),
  `internal/plugins/init/main.go` (301) - call `zeweb.GenerateWebCertWithNames` to
  bootstrap TLS at install time, with web off.
  → Constraint: these are the release-critical install-path consumers; Phase 1 must
    repoint them to `selfcert` with identical behavior, validated by the functional suite.
- [ ] `cmd/ze/hub/service_lg.go` (line 76, `//go:build ze_lg`) - calls
  `zeweb.LoadOrGenerateCert` for lg's TLS cert.
  → Constraint: lg-with-web-off must keep cert generation; repoint to `selfcert`.
- [ ] `internal/component/plugin/all/all.go` (line 83) - blank-imports `web/yang`.
  → Constraint: must move into generated `all_ze_web.go` under `//go:build ze_web`.

**Behavior to preserve:**
- Default `ze` and `ze-appliance` keep the full web UI (ZE_FEATURES includes `ze_web`).
- The web UI's routes, auth, SSE, editor, and listener-migration behavior are byte-for-byte
  unchanged when web is compiled in.
- `ze init` / appliance TLS-cert bootstrap behavior is identical after the selfcert move
  (same SANs, same store semantics, same PEM output).
- lg's TLS cert behavior (when `ze_lg` is on, web off) is unchanged.
- `DiscoverListenerServices` / builtin listener-default behavior for the OTHER services.

**Behavior to change:**
- `internal/component/web` becomes a disableable feature: with `ze_web` off it is linked
  nowhere and dropped from the binary.
- `ze-stripped` (`ze_core ze_ssh`) loses the web UI (it gains no `ze_web`).
- Cert/TLS helpers move package: `internal/component/web` -> `internal/core/selfcert`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of the `ze_web` build tag passed to `go build`.
- Run time: the hub (`cmd/ze/hub`) building services via `buildServices(deps)`.

### Transformation Path
1. `service_web.go` (`//go:build ze_web`) `init()` (in `register_web.go`) calls
   `registerService("web", buildWebService)`.
2. The generator emits `all_ze_web.go` blank-importing `web/yang` only under `ze_web`.
3. At startup the hub resolves web listen addresses and generic web inputs into
   `ServiceDeps` (addresses, `InsecureWeb`, config path, dispatcher, resolvers,
   authorizer, audit recorder, commit hook, config users, event ring) and calls
   `buildServices(deps)`; the web factory builds and starts the server.
4. `registerBuiltService` routes the built web service into `ListenerMigrator.SetWeb`.
5. With `ze_web` off, the web package is unimported, uncompiled, and unlinked.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `all_ze_web.go` group | [ ] |
| Composition root ↔ registry | `register_web.go` init() → `registerService("web", ...)` | [ ] |
| Registry ↔ hub | `buildServices(deps)` iterates; no direct `web.NewWebServer` always-on | [ ] |
| Web service ↔ migrator | `registerBuiltService` → `SetWeb(Reconfigurable)` | [ ] |
| Cert helpers ↔ install path | appliance/init/lg import `selfcert`, never `web` | [ ] dep_audit |
| Disableable web ↔ always-on | MUST be registry-only; dep_audit DISABLEABLE enforces | [ ] audit |

### Integration Points
- `internal/core/selfcert` (new) - cert/TLS surface; imported by web, lg, appliance, init.
- `Reconfigurable` / `ListenerMigrator` (existing) - web routed via `SetWeb`.
- `scripts/dev/dep_audit.py` DISABLEABLE - `internal/component/web -> ze_web`.
- `scripts/codegen/plugin_imports.go` featureTags - `web/yang -> ze_web`.

### Architectural Verification
- [ ] No bypassed layers (web still built via the registry + Reconfigurable contract)
- [ ] No unintended coupling (selfcert is a pure leaf; the registry is the only new surface)
- [ ] No duplicated functionality (reuse 980 registry; selfcert centralizes cert gen)
- [ ] Zero-copy preserved (N/A - composition/build change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The cert/TLS surface extracts to selfcert with no behavior change | server.go cert fns use only stdlib crypto + a logger | install-time TLS bootstrap differs (SANs, store) | full functional suite + appliance/init cert path tests pass after Phase 1 | unvalidated |
| A-2 | After Phase 1, the ONLY always-on importers of internal/component/web are the hub's web-service construction | grep importers; appliance/init/lg/migrator are the known non-hub ones | dep_audit still flags an always-on importer after Phase 2 | re-grep importers of internal/component/web before adding to DISABLEABLE | unvalidated |
| A-3 | web fits the construction registry (no reactor-lifecycle callback), even though it starts after engine/plugin server today | Source: engine/plugin server start before web; web reconfigures only through migrator | web needs a bespoke seam like ssh | audit startWebServer deps; confirm all post-start inputs can be passed as generic ServiceDeps | confirmed |
| A-4 | A no-web build leaves config validation safe (web/yang not registered) | 980: schema gated by generator → clean "unknown field" | `web {}` config panics in a no-web build | build ze_core binary, feed `web {}` config | unvalidated |
| A-5 | listener discovery/default handling tolerates an absent web schema/service | listener services are schema-discovered; web still has a builtin listener default | parse-time conflict detection panics/misfires with web absent | build ze_core binary, run listener-conflict path and config with no web schema | unvalidated |
| A-6 | NewTLSConfig has no callers outside web that would also need selfcert | grep showed web-internal use at server.go:145 | a missed caller breaks compile after the move | grep all callers of NewTLSConfig before the move | confirmed |
| A-7 | RunWebOnly can be gated behind the seam without breaking the no-bgp daemon mode for OTHER services | main.go RunWebOnly builds web directly | RunWebOnly is a second hard-coded web path that breaks the no-web build | trace RunWebOnly; design a nil-able hook like ssh's standalone path | confirmed |
| A-8 | ze-stripped-surface.ci does not drive the web UI | 981: that .ci ssh-es in to drive the CLI | dropping web from ze-stripped breaks the functional gate | read test/ui/ze-stripped-surface.ci for any web/HTTP assertion | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Phase 1 changes install-time TLS bootstrap and breaks first-boot HTTPS, which the unit suite does not exercise | appliance/init functional tests fail; manual first-boot has no cert | keep Phase 1 a behavior-preserving move (identical bodies, package only); run full functional suite before any gating |
| R-2 | startWebServer carries a hidden dependency that only resolves post-reactor, forcing a bespoke seam | the factory cannot be built at buildServices time | audit deps in Phase 2 audit; if found, fall back to an ssh-style nil-able hook for that piece only |
| R-3 | RunWebOnly is a second construction path missed by the registry-ize | no-web build fails to compile main.go | treat RunWebOnly explicitly (gated hook); AC-9 covers it |
| R-4 | Dropping web from ze-stripped breaks ze-stripped-surface.ci | that .ci fails | A-8 validates first; if it drives web, either keep web in stripped or split the .ci (user decision) |
| R-5 | A missed always-on web symbol (RegisterPortalService, EditorManager) keeps web linked despite the tag | go tool nm shows web symbols in ze_core build | dep_audit DISABLEABLE + the nm symbol-count check in the absent test catch residuals before close |
| R-6 | selfcert misplaced (importing config/component) trips the tier check | make ze-tier-check fails | keep selfcert deps to stdlib + slogutil only |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_web'` | → | web factory registered; hub builds web | `TestBuildTag_Web_Present` (cmd/ze/hub) |
| `go build -tags ze_core` (web off) | → | web package not linked; daemon starts without web | `TestBuildTag_Web_Absent` (cmd/ze/hub) |
| registry has a registered web factory | → | hub builds web via `buildServices`, not `web.NewWebServer` | `TestServiceRegistry_BuildsWeb` (gated `ze_web`) |
| `dep_audit.py --check` over the tree | → | no always-on import of `internal/component/web` | `dep_audit` `--check` clean + `--selftest` |
| appliance/init/lg cert call | → | resolves through `internal/core/selfcert` | existing appliance/init functional tests pass |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Phase 1 complete | `internal/core/selfcert` holds `CertStore`, `GenerateWebCert*`, `addInterfaceIPs`, `NewTLSConfig`, `LoadOrGenerateCert`; appliance, init plugin, lg, and web's `NewWebServer` all import it; the web package no longer exports cert/TLS helpers to always-on consumers |
| AC-2 | Phase 1 complete, web still always-on (no `ze_web` yet) | `make ze-verify-changed` passes all stages; behavior identical (install TLS bootstrap, lg cert, listener migration) |
| AC-3 | Phase 2 complete | `dep_audit.py --check` with `internal/component/web -> ze_web` in DISABLEABLE is clean: no always-on (untagged, non-test) file imports `internal/component/web` |
| AC-4 | `go build` with `ze_web` ON | web compiled in, registered, started; existing web unit + functional tests pass; UI reachable |
| AC-5 | bare `go build -tags ze_core` (web OFF) | `go tool nm` shows zero `internal/component/web` server symbols; daemon starts without web; no error |
| AC-6 | no-web binary fed config containing `web { ... }` | clean "unknown field" validation handling, no panic |
| AC-7 | the generator runs | emits `all_ze_web.go` (`//go:build ze_web`) blank-importing `web/yang`; removes web/yang from `all.go`; `plugin_imports.go --check` passes |
| AC-8 | a no-web build exercises listener discovery/default handling | parse-time port-conflict detection for other services works; no panic from the absent web schema/service |
| AC-9 | a no-web binary invokes the web-only daemon mode (`RunWebOnly`) | returns a clean "web not compiled in" error; no build break, no panic |
| AC-10 | always-on hub code is inspected | `SetWeb` / `NewListenerMigrator` take `Reconfigurable`; no always-on file names `*zeweb.WebServer` |
| AC-11 | `make ze-stripped` (`ze_core ze_ssh`) and `make ze` are built | ze-stripped links no web symbols; `ze`/`ze-appliance` keep the full web UI |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without web (`go build -tags ze_core`) | tag off → web blank import dropped → package unlinked → no web listener | `TestBuildTag_Web_Absent` + `go tool nm` symbol check |
| 2 | builds a full `ze` with web (default) | tag on → factory registered → hub builds web via registry → UI listens | `TestBuildTag_Web_Present` + existing web functional tests |
| 3 | runs `ze init` on a no-web build to bootstrap TLS | init → `selfcert.GenerateWebCertWithNames` → cert written to store | existing init functional test (now via selfcert) |
| 4 | runs a no-web binary against a config with `web {}` | config load → web schema absent → clean unknown-field handling | `test/parse/web-absent-config.ci` or `TestBuildTag_Web_Absent` config assertion |
| 5 | reloads listener config on a web build | `ReloadListeners` → `SetWeb(Reconfigurable)` → web `Reconfigure` | existing listener-migration test (web routed via registry) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSelfCert_GenerateAndLoad` | `internal/core/selfcert/selfcert_test.go` | cert gen + LoadOrGenerateCert round-trip (moved tests) | |
| `TestSelfCert_SANsForListenAddr` | same | SANs include localhost/loopback + listen addr + extra names | |
| `TestSelfCert_NewTLSConfig` | same | TLS config keeps minimum TLS 1.2 and rejects invalid PEM material | |
| `TestInitPlugin_WebCertUsesSelfcert` | `internal/plugins/init/main_test.go` | `--web-cert` / `--web-cert-name` generate and store cert material via selfcert | |
| `TestServiceRegistry_BuildsWeb` | `cmd/ze/hub/service_web_test.go` (`//go:build ze_web`) | hub builds web via registry, not direct ctor | |
| `TestListenerDiff_MigratorLocal` | `cmd/ze/hub/listener_migrate_test.go` | migrator-local listenerDiff matches old behavior | |
| `TestBuildTag_Web_Present` | `cmd/ze/hub/build_tag_web_present_test.go` (`//go:build ze_web`) | web factory hooks non-nil | |
| `TestBuildTag_Web_Absent` | `cmd/ze/hub/build_tag_web_absent_test.go` (`//go:build !ze_web`) | web factory not registered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_web` | `cmd/ze/hub/build_tag_web_*_test.go` | web present with `ze_web`, absent without | |
| `web-absent-config` | `test/parse/web-absent-config.ci` (or absent-test assertion) | no-web binary handles `web {}` config safely | |
| appliance/init cert | existing appliance + init functional tests | install-time TLS bootstrap unchanged via selfcert | |

### Interop Tests (MANDATORY for protocol features)
- N/A. No wire-protocol behavior changes.

### Future (if deferring any tests)
- None. Web is fully in scope for this spec.

## Files to Modify
- `internal/component/web/server.go` - remove the cert/TLS surface (moved to selfcert);
  `NewWebServer` imports `selfcert.NewTLSConfig`; keep `ListenerDiff` for web-internal use
- `internal/appliance/cmd_init.go`, `cmd_cert.go` - import `selfcert` for cert gen
- `internal/plugins/init/main.go` - import `selfcert` for cert gen
- `cmd/ze/hub/service_lg.go` (`//go:build ze_lg`) - import `selfcert` for `LoadOrGenerateCert`
- `cmd/ze/hub/main_servers.go` - move `startWebServer` + web-only helpers out to `service_web.go`
- `cmd/ze/hub/main.go` - gate `RunWebOnly`, `RegisterPortalService`, `EditorManager` references behind the seam; remove web type names from always-on code
- `cmd/ze/hub/listener_migrate.go` - `SetWeb`/`NewListenerMigrator` take `Reconfigurable`; migrator-local `listenerDiff`
- `cmd/ze/hub/service_registry.go` - `ServiceDeps` gains web inputs: listen addrs, `InsecureWeb`, config path, dispatcher, resolvers, authorizer, audit recorder, commit hook, config users, event ring; `registerBuiltService` routes "web" → `SetWeb`
- `scripts/codegen/plugin_imports.go` - `featureTags["internal/component/web/yang"] = "ze_web"`
- `internal/component/plugin/all/all.go` - web/yang removed (generator)
- `scripts/dev/dep_audit.py` - `DISABLEABLE["internal/component/web"] = "ze_web"`
- `Makefile` - `ZE_FEATURES += ze_web` (ze/ze-appliance); ze-stripped unchanged (stays `ze_core ze_ssh`)
- `internal/test/runner/runner.go` - `TestBuildTags()` appends `ze_web`
- `.golangci.yml` - `build-tags` appends `ze_web`
- `cmd/ze/hub/main_servers_webonly_test.go`, `web_commit_hang_repro_test.go`, and any
  hub test naming web symbols - gate `//go:build ze_web` or skip-guard with `// test-relax:`
- `ai/rules/module-tiers.md`, `docs/features.md` - document `ze_web` + the selfcert leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | build-time, not CLI |
| Functional test | [ ] yes | `cmd/ze/hub/build_tag_web_*_test.go`, config-absent assertion |
| Doctor check | [ ] no | web owns its own doctor check; absent web = no check registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register `ze_web` in the build-tag set |
| YANG schema | [ ] no new | web/yang exists; only its blank import is gated |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavor) | `docs/features.md` (build-tag table: add `ze_web`) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md` (registry services), `ai/rules/module-tiers.md` (selfcert leaf) |
| 15 | Runtime inventory changed? | [ ] location only | note selfcert as the cert home if a plugin-overview doc references web cert gen |
| 16 | Source anchors reference changed files? | [ ] check | grep `docs/` for `source:` anchors at `web/server.go` cert fns; repoint to selfcert |
| others | - | [ ] assess | grep docs for web cert / TLS references |

## Files to Create
- `feature-gates.txt` (repo root) - SINGLE SOURCE OF TRUTH for compile-out-able
  features: `<tag> <pkg>` per line (ze_lg/ze_ssh/ze_web). The generator, Makefile
  `ZE_FEATURES`, `TestBuildTags`, and `dep_audit.py` DISABLEABLE all DERIVE from it;
  only `.golangci.yml` build-tags stays hand-edited (static YAML) and is drift-checked
  by `dep_audit.py --check`. Collapses the former five-place hand-wiring to one line.
- `ai/rules/feature-gate-registration.md` - the manifest workflow + the two
  registration shapes (construction registry vs seam) for agents.
- `internal/test/runner/manifest_test.go` - `TestBuildTags` reads gate tags from the manifest.
- `internal/core/selfcert/selfcert.go` - `CertStore`, `GenerateWebCert*`, `addInterfaceIPs`, `NewTLSConfig`, `LoadOrGenerateCert`, `certValidityDuration`
- `cmd/ze/hub/cert_store.go` - always-on zefs-backed `blobCertStore` adapter shared by web and lg
- `internal/core/selfcert/selfcert_test.go` - moved cert tests
- `cmd/ze/hub/service_web.go` (`//go:build ze_web`) - `buildWebService` factory + `webService` adapter + moved `startWebServer`/web-only helpers
- `cmd/ze/hub/register_web.go` (`//go:build ze_web`) - `init(){ registerService("web", buildWebService) }`
- `cmd/ze/hub/build_tag_web_present_test.go` (`//go:build ze_web`), `build_tag_web_absent_test.go` (`//go:build !ze_web`)
- `internal/component/plugin/all/all_ze_web.go` (generated, `//go:build ze_web`)
- `test/parse/web-absent-config.ci` (if expressible) - no-web `web {}` config safety

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior, Assumptions (validate A-2/A-6/A-7/A-8 first) |
| 3. Wiring | Wiring Test - registry + build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase 1: extract cert/TLS + ListenerDiff (behavior-preserving, NO gating)**
   - Create `internal/core/selfcert`; move the cert/TLS surface; repoint web, lg,
     appliance, init plugin. Give the migrator a local `listenerDiff`.
   - Tests: `TestSelfCert_*` (moved); existing appliance/init/web/lg tests stay green.
   - Verify: `make ze-verify-changed` passes; web still always-on; `internal/component/web`
     has no always-on importer outside the hub's web construction (re-grep, A-2).
2. **Phase 2: registry-ize web behind `ze_web`**
   - Move `startWebServer` + web-only helpers to `service_web.go`; keep `blobCertStore`
     in an always-on hub file. Add `webService` adapter + `buildWebService`;
     `register_web.go` registers it. Widen `SetWeb`/`NewListenerMigrator` to
     `Reconfigurable`; add the full generic web input set to `ServiceDeps`; route "web"
     in `registerBuiltService`. Gate `RunWebOnly`/`RegisterPortalService`/`EditorManager`
     behind the seam (nil-able hook for the web-only mode, A-7).
   - Tests: `TestServiceRegistry_BuildsWeb`, present/absent build-tag tests.
   - Verify: `ze_web` build identical to today; ze_core build drops web.
3. **Phase 3: tag wiring + schema gating + audit**
   - Four-place tag wiring (`ZE_FEATURES`, `TestBuildTags`, `.golangci.yml`, generator
     `featureTags`); regenerate `all_ze_web.go`; `dep_audit` DISABLEABLE += web.
   - Verify: generator `--check` clean; dep_audit `--check` + `--selftest` clean;
     `go tool nm` shows 0 web symbols in ze_core, N in ze_web.
4. **Phase 4: docs + stripped validation**
   - `docs/features.md`, `module-tiers.md`; validate A-8 (ze-stripped-surface.ci) and
     A-4/A-5 (no-web config safety).
5. **Full verification** - `make ze-verify-changed`; build + nm-measure ze_core vs ze_web.

### Failure Routing
| Failure | Route To |
|---------|----------|
| web not omitted with tag off | a residual always-on web import remains - find via dep_audit + nm (R-5) |
| appliance/init cert path breaks | the selfcert move changed behavior - Phase 1, A-1/R-1 |
| RunWebOnly fails to compile (web off) | the second construction path - A-7/R-3 |
| generator `--check` fails | the gated-group emission - `plugin_imports.go` |
| config panics in no-web build | schema/listener absence - A-4/A-5 |
| ze-stripped-surface.ci fails | web dropped from stripped - A-8/R-4 (user decision) |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Extraction fidelity | selfcert functions are byte-identical moves (no logic change); diff the bodies |
| No always-on web import | dep_audit `--check` clean; `grep -rl internal/component/web` shows only gated/test files |
| Symbol absence | `go tool nm` on ze_core binary lists zero web server symbols |
| Reconfigurable widening | no always-on file names `*zeweb.WebServer` |
| Tier placement | selfcert imports only stdlib + slogutil; `make ze-tier-check` green |
| Test gating | every web-requiring test is gated or skip-guarded; default ze_core unit suite passes |
| Rule: no-layering | old cert fns fully removed from web (not left as aliases that re-pin the import) |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/selfcert` package | `ls internal/core/selfcert/*.go`; `go test ./internal/core/selfcert/` |
| web factory + registration | `ls cmd/ze/hub/service_web.go register_web.go`; `grep registerService.*web` |
| dep_audit DISABLEABLE entry | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_web.go | `ls internal/component/plugin/all/all_ze_web.go`; `plugin_imports.go --check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm /tmp/ze-core` web count = 0 |
| present/absent tests | `go test -tags 'ze_core ze_web' -run TestBuildTag_Web` and `-tags ze_core` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| Cert gen integrity | selfcert generates the same key strength (ECDSA P-256), validity, and SANs as before; no weakening in the move |
| Key material handling | `CertStore.WriteKey` permission semantics preserved; no key logged |
| No auth bypass | gating web removes the UI auth surface entirely (smaller attack surface); a no-web build exposes no web endpoints |
| Insecure-web flag | `--insecure-web` path stays inside the gated web code; not reachable in a no-web build |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The web compile-out's real cost is not the 51 call-sites (they concentrate in one
  movable function) but the shared utilities trapped in the service package. The lesson
  generalizes: before gating a service, find every NON-lifecycle export it provides to
  always-on code and extract those first.
- `selfcert` is independently valuable: self-signed cert generation has no business
  living in the HTMX UI package; extracting it is a tier correction regardless of gating.
- The feature-gate "tag ↔ package" fact was hardcoded in five disconnected places
  (Makefile `ZE_FEATURES`, `.golangci.yml`, `TestBuildTags`, generator `featureTags`,
  `dep_audit.py` DISABLEABLE) across four languages. With five more gate specs queued
  (gnmi/mcp/api/monitoring/protocols), that hand-wiring (and its documented "missing
  the TestBuildTags entry is the trap") was scheduled to repeat. A single repo-root
  manifest `feature-gates.txt` now holds the fact once; every program-language consumer
  derives from it, and the one static consumer (`.golangci.yml`) is drift-gated. The
  runtime construction registry was already dynamic; this fixes the build-time half.

## Core Insight
A service is only compile-out-able once nothing always-on imports its package for ANY
reason - lifecycle OR a borrowed helper. Web borrowed cert gen to the install path, so
gating it required a utility extraction first; the registry-ize was the easy half.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Construction registry (like lg), not a bespoke seam (like ssh) | ssh-style nil-able hooks for the daemon path | web needs no reactor-lifecycle callbacks; it starts after engine/plugin server today and reconfigures via the migrator |
| Extract cert/TLS to `internal/core/selfcert` | reuse `internal/component/pki`; split web into web/cert subpkg | pki is config-driven (wrong tier); a web/cert subpkg forces a DISABLEABLE carve-out. selfcert is a clean stdlib-only core leaf |
| `ListenerDiff` → migrator-local copy | extract to a shared core package | rest/grpc/lg/mcp already carry private copies; a local copy matches convention with the smallest diff |
| One spec, two phases | two specs (extract first, gate second) | user decision (SCOPE gate); mirrors ssh; single learned summary keeps the umbrella tidy |
| ze-stripped drops web | keep web in stripped | web is an optional UI, not the management plane; ssh (kept) is the management plane (981) |

## Known Limitations
- Protocol compile-out (isis/ldp/ospf/rsvpte) is a separate child blocked on tiers-5 B-2.
- gnmi/mcp/api/monitoring compile-out are separate children (own specs).
- A no-web build has no HTMX UI; remote management is via ssh CLI + API only.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| web compile-out-able via `ze_web` | build-tag test + nm symbol check | `TestBuildTag_Web_Absent` passes; `go tool nm` shows 0 web symbols in ze_core build |
| install-path cert bootstrap unaffected | functional test | appliance + init functional tests pass with cert via selfcert |
| no always-on import of web | audit | `dep_audit.py --check` clean with web in DISABLEABLE |
| default flavors keep web | build | `ze`/`ze-appliance` link web; `ze-stripped` does not |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `ListenerDiff` lost its only cross-package caller when the migrator switched to a local `listenerDiff` (AC-10); left exported, inconsistent with the lowercase sibling copies | `internal/component/web/server.go:230` | FIXED: unexported to `webListenerDiff`; updated in-package caller + white-box test |
| 2 | NOTE | golangci build-tags parser would truncate on an inline `# comment` | `scripts/dev/dep_audit.py` `parse_golangci_build_tags` | FIXED: strips inline comments; selftest fixture added |
| 3 | NOTE | `ze-validate` reports 16 exported-symbol flags (hub `Set*`/`ServiceDeps`, `selfcert.GenerateWebCert`, init `RunWith*`, `runner.Timings`) | leaf-package exports | Confirmed PRE-EXISTING on HEAD (same count); not introduced by this spec; `ze-validate` is not in `ze-verify` |
| 4 | NOTE | manifest derivation couples to the `ze_` tag prefix and `GOLANGCI_BASE_TAGS={ze_core}` | `Makefile`, `dep_audit.py` | Documented convention in `ai/rules/feature-gate-registration.md`; accepted |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (Run 1's single ISSUE fixed; re-run clean)
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/selfcert/selfcert.go` + `_test.go` | yes | `go test ./internal/core/selfcert/` ok |
| `cmd/ze/hub/service_web.go` (ze_web) | yes | `buildWebService` + `webService` adapter + moved `startWebServer`/`runWebOnly` |
| `cmd/ze/hub/register_web.go` (ze_web) | yes | `registerService("web", buildWebService, SetWeb)` + `setWebStandalone(runWebOnly)` |
| `cmd/ze/hub/web_infra.go` (always-on) | yes | nil-able `webBuildStandalone` seam + `webPortalService` |
| `cmd/ze/hub/cert_store.go` (always-on) | yes | shared `blobCertStore` (web + lg) |
| `internal/component/plugin/all/all_ze_web.go` (generated, ze_web) | yes | blank-imports `web/yang`; `--check` green |
| `feature-gates.txt` (manifest, this session) | yes | single source of truth; all consumers derive |
| `ai/rules/feature-gate-registration.md` (this session) | yes | indexed (`make ze-rules-index`, 77 rules) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | selfcert holds cert/TLS; appliance/init/lg/web import it; web exports none to always-on | importers grep: appliance(cmd_cert,cmd_init), init, service_lg, service_web; `web/server.go` has no cert/TLS exports; 0 remaining `zeweb.{GenerateWebCert,LoadOrGenerateCert,NewTLSConfig,CertStore}` callers |
| AC-2 | Phase 1 behavior-preserving | `make ze-lint-changed` 0 issues; selfcert + init + web + hub suites green |
| AC-3 | dep_audit --check clean with web disableable | `dep_audit.py --check` exit 0 (web via manifest DISABLEABLE); only ze_web-gated `service_web.go` imports `internal/component/web` |
| AC-4 | web ON: built, registered, started; tests pass | `go test ./internal/component/web/` ok; `go test -tags 'ze_core ze_web' ./cmd/ze/hub/` ok; `TestServiceRegistry_BuildsWeb`, `TestBuildTag_Web_Present` pass |
| AC-5 | ze_core (web off): 0 web server symbols | `go tool nm` ze_core: 0 `internal/component/web.` symbols; `TestBuildTag_Web_AbsentBinaryDropsWebSymbols` passes |
| AC-6 | no-web binary + `web {}` config → clean unknown-field | `TestBuildTag_Web_AbsentRejectsWebConfig` passes (err contains "unknown field") |
| AC-7 | generator emits all_ze_web.go; web/yang out of flat all.go | `all.go` web/yang count 0; `all_ze_web.go` web/yang count 1; `plugin_imports.go --check` green |
| AC-8 | no-web listener discovery/default safe | hub web-off suite ok; absent config rejected (no web listener to conflict); no panic |
| AC-9 | no-web `RunWebOnly` → clean "web not compiled in" | `RunWebOnly` nil-guards `webBuildStandalone` → `webNotCompiledIn()` (exit 1); `TestBuildTag_Web_Absent` asserts |
| AC-10 | `SetWeb`/`NewListenerMigrator` take Reconfigurable; no always-on `*zeweb.WebServer` | `listener_migrate.go:47,55` Reconfigurable; migrator-local `listenerDiff` (no `zeweb.ListenerDiff`) |
| AC-11 | ze-stripped drops web; ze/ze-appliance keep it | `nm` ze-stripped: 0 web / 119 ssh; ze+features: 716 web symbols |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | selfcert is a behavior-preserving stdlib-only move; init/appliance cert tests + `TestInitPlugin_WebCertUsesSelfcert` pass |
| A-2 | confirmed | only ze_web-gated `service_web.go` imports `internal/component/web`; dep_audit --check clean |
| A-3 | confirmed | web fits the construction registry; `buildWebService` builds from generic `ServiceDeps` |
| A-4 | confirmed | `web {}` config in a ze_core build → "unknown field", no panic (test) |
| A-5 | confirmed | ze_core hub suite green; listener discovery tolerates absent web schema/service |
| A-6 | confirmed | `NewTLSConfig` only used inside web (now via selfcert); no missed caller |
| A-7 | confirmed | `RunWebOnly` gated via nil-able `webBuildStandalone` seam |
| A-8 | confirmed | ze-stripped builds and drops web (0 symbols); ssh management plane retained (119 symbols) |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 demonstrated
- [ ] `internal/core/selfcert` created; cert path repointed; behavior preserved (Phase 1)
- [ ] web compile-out-able; present/absent build-tag tests pass
- [ ] dep_audit DISABLEABLE clean; no always-on web import
- [ ] generator emits all_ze_web.go; `--check` passes
- [ ] `make ze-verify-changed` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe)
- [ ] Goal Validation table filled with concrete evidence
