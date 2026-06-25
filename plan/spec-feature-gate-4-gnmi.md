# Spec: feature-gate-4-gnmi

| Field | Value |
|-------|-------|
| Status | done |
| Depends | plan/spec-feature-gate-0-umbrella.md; learned 980 (lg registry), 981 (ssh seam) |
| Phase | 4/4 |
| Updated | 2026-06-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/981-feature-gate-2-ssh.md` - the dedicated-seam pattern this mirrors
4. `cmd/ze/hub/ssh_infra.go` - the seam shape (opaque handle + nil-able hook vars + setter)
5. `internal/component/gnmi/server.go`, `subscribe.go` - the gNMI server + ChangeNotifier
6. `cmd/ze/hub/main.go` (gnmi build ~899-966, reload closure ~661), `main_servers.go` (serveGNMI/waitForGNMIBind)

## Task

Make the **gNMI service compile-out-able** from the `ze` binary via a `ze_gnmi` build
tag, for a smaller binary and a smaller attack surface. gNMI is a feature-gate umbrella
child (`plan/spec-feature-gate-0-umbrella.md`).

Unlike web/lg (listener services driven by the construction registry), gNMI uses a
**dedicated seam** (ssh's pattern, learned 981) for three reasons the registry's
`Service` contract cannot express: (1) its constructor needs richer deps than
`ServiceDeps` carries (a config-tree accessor, a `*api.ConfigSessionManager`, a YANG
loader, a `*ChangeNotifier`); (2) the always-on config-reload closure calls
`gnmiNotifier.NotifyConfigReload()`, a direct dependency on a gNMI type that must be
severed; (3) gNMI binds once and never live-migrates listeners, so it has no
`Reconfigure` and is not in `ListenerMigrator`. gNMI also requires gating every
owned blank import: `gnmi/yang` (config schema), the `gnmi` package itself (the
handler registration), and `internal/plugins/gnmi-cmd/yang` (the "show gnmi" command schema).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - the dedicated-seam pattern
  → Constraint: an always-on `*_infra.go` declares an opaque server handle, generic
    input structs (never a feature type), nil-able hook vars, and a `set<Feature>Infra`
    setter; the gated `service_<x>.go` (`//go:build ze_<x>`) holds the impl; `init()` in
    `register_<x>.go` installs the hooks. With the tag off the hooks are nil and the
    feature is skipped.
  → Constraint: only GENERIC types cross the seam (config values, accessors, registries);
    never a `zegnmi` type. The hub's always-on files must not import the gnmi package.
  → Constraint: tag wiring is single-source. `feature-gates.txt` is the only
    manifest edit; `ZE_FEATURES` (Makefile awk), `TestBuildTags()`
    (`internal/test/runner/runner.go`), `featureTags` (generator `loadFeatureTags`),
    and `dep_audit` `DISABLEABLE` (`load_feature_gates`) all DERIVE from it. Only
    `.golangci.yml` build-tags is hand-edited (static YAML); `dep_audit.py --check`
    flags its drift.
- [ ] `plan/learned/980-feature-gate-1-lg.md` - the generator schema-gating mechanism
  → Constraint: `featureTags` maps package dirs to a tag; the generator emits matching
    blank imports into `all_ze_<tag>.go` and removes them from the flat `all.go`.
    For gnmi this maps THREE dirs: `gnmi`, `gnmi/yang`, and `gnmi-cmd/yang`.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with `ze_gnmi` off, both gnmi blank imports drop and the package is
    unlinked; no gnmi spelling may remain in always-on hub files.

### RFC Summaries (MUST for protocol work)
- N/A. gNMI compile-out is a composition/build-tag change; the gNMI/gRPC wire behavior
  is unchanged when compiled in.

**Key insights:**
- gNMI's always-on importers were the generated `all.go` blank imports for
  `internal/component/gnmi/yang`, `internal/component/gnmi`, and the command-schema
  sidecar `internal/plugins/gnmi-cmd/yang`, PLUS the hub `main.go` which directly
  constructed the server (`NewServer`, `ChangeNotifier`, `RegisterGlobal`).
- The build block (`main.go:899-966`) is self-contained: config resolution
  (`zeconfig.ExtractGNMIConfig`), `NewChangeNotifier`, `NewServer`, `SetMetricsRegistry`,
  `RegisterGlobal`, then `go serveGNMI(...)` + `waitForGNMIBind(...)`; shutdown is
  `gnmiSrv.Stop()` (main.go:1042). It moves wholesale into the gated impl.
- The reload closure (`main.go:661`) calls `gnmiNotifier.NotifyConfigReload()`. The
  notifier must live inside the gated code; the closure calls a nil-able
  `gnmiReloadNotify` hook (no-op when gnmi is off).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/gnmi/server.go` - `NewServer(cfg, treeFn, sessions, loader,
  notifier) *Server`; `Serve(ctx)` (130), `Stop()` (183), `Address()` (196),
  `SetMetricsRegistry(reg)` (123); `RegisterGlobal(s)` (44) / `LookupServer()` (51) for
  show-command access.
  → Constraint: lifecycle is Serve/Stop/Address (singular), NOT Reconfigurable; gnmi is
    not a listener-migration participant.
- [ ] `internal/component/gnmi/subscribe.go` - `ChangeNotifier` (24); `NewChangeNotifier()`
  (30), `NotifyConfigReload()` (78), `Notify`/`Subscribe`/`Unsubscribe`.
  → Constraint: the hub declares `var gnmiNotifier *zegnmi.ChangeNotifier` (main.go:651)
    and calls `NotifyConfigReload()` in the reload closure. This naming of a gnmi type in
    always-on code is the coupling the seam must remove.
- [ ] `cmd/ze/hub/main.go` (899-966 build; 651 notifier var; 661 reload notify; 1042
  Stop) and `cmd/ze/hub/main_servers.go` (671 `serveGNMI`, 678 `waitForGNMIBind`),
  `cmd/ze/hub/api.go` (449 `buildGNMISessionManager`).
  → Constraint: all gnmi construction/lifecycle lives in the hub; severing the direct
    gnmi import from always-on hub files + gating two blank imports compiles gnmi out.
- [ ] `internal/component/plugin/all/all.go` (65 `gnmi/yang`, 267 `gnmi` package).
  → Constraint: BOTH blank imports must move into generated `all_ze_gnmi.go`.

**Behavior to preserve:**
- Default `ze`/`ze-appliance` keep gNMI (ZE_FEATURES includes `ze_gnmi`).
- gNMI server behavior (Get/Set/Subscribe, TLS, token auth, config-session integration,
  metrics) byte-for-byte unchanged when compiled in.
- `NotifyConfigReload` still fires on config reload when gnmi is compiled in (the reload
  closure calls the hook).
- The "show gnmi" CLI command works when gnmi is compiled in.

**Behavior to change:**
- `internal/component/gnmi` becomes a disableable feature: `ze_gnmi` off => unlinked.
- `ze-stripped` drops gNMI (it gains no `ze_gnmi`).
- The reload closure calls a nil-able hook instead of naming `*zegnmi.ChangeNotifier`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of `ze_gnmi`.
- Run time: the hub `run()` building gNMI via the seam hook.

### Transformation Path
1. `register_gnmi.go` (`//go:build ze_gnmi`) `init()` calls
   `setGNMIInfra(gnmiBuildImpl, gnmiReloadNotifyImpl)`.
2. The generator emits `all_ze_gnmi.go` blank-importing `gnmi/yang` AND `gnmi` only
   under `ze_gnmi`.
3. The hub resolves the generic build inputs (tree accessor, session manager, loader,
   metrics registry, store, config path) into `gnmiBuildInputs` and calls
   `if gnmiBuild != nil { gnmiSrv = gnmiBuild(&inputs) }`.
4. The gated impl extracts the gnmi config, builds the notifier + server, starts it, and
   stores the notifier in a package var for `gnmiReloadNotifyImpl`.
5. The reload closure calls `if gnmiReloadNotify != nil { gnmiReloadNotify() }`.
6. With `ze_gnmi` off, both hooks are nil, both blank imports drop, the package unlinks.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `all_ze_gnmi.go` (two imports) | [ ] |
| Composition root ↔ seam | `register_gnmi.go` init() → `setGNMIInfra(...)` | [ ] |
| Seam ↔ hub build | `gnmiBuild(&gnmiBuildInputs)` (generic inputs only) | [ ] |
| Reload closure ↔ gnmi | `gnmiReloadNotify()` hook, never a `*zegnmi.ChangeNotifier` | [ ] |
| Disableable gnmi ↔ always-on | dep_audit DISABLEABLE enforces no direct import | [ ] audit |

### Integration Points
- `cmd/ze/hub/gnmi_infra.go` (new, always-on) - the seam.
- `cmd/ze/hub/service_gnmi.go` (new, gated) - the impl + moved serveGNMI/waitForGNMIBind.
- `scripts/dev/dep_audit.py` DISABLEABLE - derives `internal/component/gnmi -> ze_gnmi`
  from `feature-gates.txt` via `load_feature_gates` (no hand-edit).
- `scripts/codegen/plugin_imports.go` featureTags - `loadFeatureTags` reads
  `feature-gates.txt` and gates `gnmi`, `gnmi/yang`, and `gnmi-cmd/yang`.

### Architectural Verification
- [ ] No bypassed layers (gnmi still built + started the same way, just behind the seam)
- [ ] No unintended coupling (only generic types cross the seam)
- [ ] No duplicated functionality (reuse ssh's seam shape)
- [ ] Zero-copy preserved (N/A - composition/build change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | gNMI's only always-on importers are all.go (two blank imports) + the hub build block | Explore map | a missed always-on importer keeps gnmi linked | `make ze-doc-test` found `internal/plugins/gnmi-cmd/yang` still registering `show gnmi` with the handler gated out; added a second `ze_gnmi` manifest row | broken, fixed |
| A-2 | the generator's featureTags can gate multiple dirs to one tag (gnmi, gnmi/yang, gnmi-cmd/yang) | 980: featureTags is a dir->tag map | the generator only handles one dir per tag | `loadFeatureTags` maps every manifest package and its `/yang` suffix; `byTag` groups all matches under `ze_gnmi` | confirmed |
| A-3 | the build inputs are all generic (no zegnmi type needed in always-on main.go) | ExtractGNMIConfig lives in zeconfig; sessions/loader/metrics are always-on | main.go still needs a zegnmi type | `gnmiBuildInputs` uses config tree, store, config path, reload hook, and generic accessors only; `main.go` uses opaque `gnmiServer` | confirmed |
| A-4 | a no-gnmi build leaves config validation safe (gnmi/yang not registered) | 980: schema gated => clean "unknown field" | `gnmi {}` config panics in a no-gnmi build | `TestBuildTag_GNMI_AbsentRejectsGNMIConfig` under `go test -tags ze_core -run TestBuildTag_GNMI ./cmd/ze/hub` | confirmed |
| A-5 | dropping the "show gnmi" RPC in a no-gnmi build is safe (command absent, not erroring) | the RPC is registered via the gated gnmi package import | the dispatcher panics on the missing command | `TestBuildTag_GNMI_AbsentBinaryDropsGNMISymbolsAndCommand` under `go test -tags ze_core -run TestBuildTag_GNMI ./cmd/ze/hub` | confirmed |
| A-6 | the ChangeNotifier has no other always-on referent besides the reload closure | Explore: used at main.go:651/661 | another always-on caller names the notifier | `search ChangeNotifier|NotifyConfigReload|gnmiNotifier` found always-on refs only in `cmd/ze/hub/main.go`; remaining refs are inside `internal/component/gnmi` tests/package | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the two-dir featureTags gating is unsupported and needs a generator change | generator `--check` fails or emits one import | A-2 spike first; extend the generator to accept multiple dirs per tag if needed |
| R-2 | a residual gnmi symbol (RegisterGlobal/LookupServer global, metrics) keeps it linked | go tool nm shows gnmi symbols in ze_core | dep_audit DISABLEABLE + nm symbol-count check in the absent test |
| R-3 | the reload-closure hook is missed and main.go still names the notifier type | no-gnmi build fails to compile main.go | the seam's nil-able `gnmiReloadNotify` is mandatory; AC-3 covers it |
| R-4 | "show gnmi" command registration leaks into always-on via a non-all.go path | grep finds a second registration site | grep RegisterRPCs for gnmi; ensure the only registration is the gated package import |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_gnmi'` | → | gnmi seam hooks installed; hub builds gnmi | `TestBuildTag_GNMI_Present` (cmd/ze/hub) |
| `go build -tags ze_core` (gnmi off) | → | gnmi package not linked; daemon starts without gnmi | `TestBuildTag_GNMI_Absent` (cmd/ze/hub) |
| reload closure on a gnmi build | → | `gnmiReloadNotify()` fires `NotifyConfigReload` | `TestGNMIReloadNotify` (gated `ze_gnmi`) |
| `dep_audit.py --check` | → | no always-on import of `internal/component/gnmi` | `dep_audit` `--check` clean + `--selftest` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build` with `ze_gnmi` ON | gnmi compiled in, built via the seam, started; existing gnmi tests pass; "show gnmi" works |
| AC-2 | bare `go build -tags ze_core` (gnmi OFF) | `go tool nm` shows zero `internal/component/gnmi` symbols; daemon starts without gnmi; no error |
| AC-3 | always-on hub code is inspected | no always-on file imports `internal/component/gnmi` or names `*zegnmi.ChangeNotifier`; the reload closure calls the nil-able hook |
| AC-4 | the generator runs | emits `all_ze_gnmi.go` (`//go:build ze_gnmi`) blank-importing BOTH `gnmi/yang` and `gnmi`; removes both from `all.go`; `--check` passes |
| AC-5 | `dep_audit.py --check` with gnmi in DISABLEABLE | clean: no always-on importer |
| AC-6 | no-gnmi binary fed config containing `gnmi { ... }` | clean "unknown field" validation, no panic |
| AC-7 | no-gnmi binary runs `show gnmi` | clean "unknown command" handling, no panic |
| AC-8 | `make ze-stripped` and `make ze` are built | ze-stripped links no gnmi symbols; `ze`/`ze-appliance` keep gNMI |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without gnmi | tag off → both blank imports dropped → package unlinked → no gnmi listener | `TestBuildTag_GNMI_Absent` + `go tool nm` symbol check |
| 2 | builds a full `ze` with gnmi (default) | tag on → seam hooks installed → hub builds gnmi → gRPC listens | `TestBuildTag_GNMI_Present` + existing gnmi functional test |
| 3 | reloads config on a gnmi build | reload closure → `gnmiReloadNotify()` → `NotifyConfigReload` → subscribers updated | `TestGNMIReloadNotify` |
| 4 | runs a no-gnmi binary against a config with `gnmi {}` | config load → gnmi schema absent → clean unknown-field handling | `test/parse/gnmi-absent-config.ci` or absent-test assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_GNMI_Present` | `cmd/ze/hub/build_tag_gnmi_present_test.go` (`//go:build ze_gnmi`) | gnmi seam hooks non-nil | PASS: `go test -tags 'ze_core ze_gnmi' -run 'TestBuildTag_GNMI|TestGNMIReloadNotify' ./cmd/ze/hub` |
| `TestBuildTag_GNMI_Absent` | `cmd/ze/hub/build_tag_gnmi_absent_test.go` (`//go:build !ze_gnmi`) | gnmi seam hooks nil | PASS: `go test -tags ze_core -run TestBuildTag_GNMI ./cmd/ze/hub` |
| `TestGNMIReloadNotify` | `cmd/ze/hub/service_gnmi_test.go` (`//go:build ze_gnmi`) | reload hook fires NotifyConfigReload | PASS: `go test -tags 'ze_core ze_gnmi' -run 'TestBuildTag_GNMI|TestGNMIReloadNotify' ./cmd/ze/hub` |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_gnmi` | `cmd/ze/hub/build_tag_gnmi_*_test.go` | gnmi present with `ze_gnmi`, absent without | PASS: present and absent tag tests |
| `gnmi-absent-config` | `cmd/ze/hub/build_tag_gnmi_absent_test.go` | no-gnmi binary handles `gnmi {}` config + `show gnmi` safely | PASS: config unknown-field and command unknown-command assertions |

### Interop Tests (MANDATORY for protocol features)
- N/A. gNMI wire behavior unchanged when compiled in; the existing gNMI interop/functional
  tests run under the `ze_gnmi` build and prove on-build behavior is unchanged.

### Future (if deferring any tests)
- None. gNMI is fully in scope for this spec.

## Files to Modify
- `cmd/ze/hub/main.go` - replace the inline gnmi build block + `var gnmiNotifier` with the
  seam call; reload closure calls `gnmiReloadNotify()`; remove the `zegnmi` import
- `cmd/ze/hub/main_servers.go` - move `serveGNMI`/`waitForGNMIBind` into the gated impl
- `cmd/ze/hub/api.go` - `buildGNMISessionManager` stays always-on (returns an `*api.ConfigSessionManager`, no gnmi type); passed in via the seam input
- `feature-gates.txt` - add two rows (`ze_gnmi internal/component/gnmi`,
  `ze_gnmi internal/plugins/gnmi-cmd`); this is the single manifest edit every
  derived consumer reads
- `scripts/codegen/plugin_imports.go` - extend `loadFeatureTags` to gate the
  direct `<pkg>` (for the show-command RPC) in addition to `<pkg>/yang`
- `internal/component/plugin/all/all.go` - all three gnmi blank imports removed (generator)
- `.golangci.yml` - `build-tags` appends `ze_gnmi` (the one hand-edited consumer;
  static YAML, drift-checked by `dep_audit.py --check`)
- DERIVED, NOT hand-edited (all read `feature-gates.txt`): `Makefile` `ZE_FEATURES`,
  `internal/test/runner/runner.go` `TestBuildTags()`, `scripts/dev/dep_audit.py`
  `DISABLEABLE`
- `ai/rules/module-tiers.md`, `docs/features.md` - document `ze_gnmi`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | build-time; "show gnmi" gated with the package |
| Functional test | [ ] yes | `cmd/ze/hub/build_tag_gnmi_*_test.go`, config/command-absent assertion |
| Doctor check | [ ] no | gnmi owns its own doctor check; absent gnmi = no check registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register `ze_gnmi` |
| YANG schema | [ ] no new | gnmi/yang exists; only its blank import is gated |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] yes (build flavor) | `docs/features.md`, `docs/guide/gnmi.md` |
| 12 | Internal architecture changed? | [x] yes | `docs/architecture/api/architecture.md`, `ai/rules/module-tiers.md`, `ai/rules/feature-gate-registration.md` |
| 15 | Runtime inventory changed? | [x] yes | "show gnmi" command/schema absent in no-gnmi build; `docs/guide/gnmi.md`, `docs/architecture/api/architecture.md` |
| others | - | [x] assessed | `make ze-doc-test` passed |

## Files to Create
- `cmd/ze/hub/gnmi_infra.go` (always-on) - opaque `gnmiServer` handle, `gnmiBuildInputs`, nil-able `gnmiBuild`/`gnmiReloadNotify`, `setGNMIInfra`
- `cmd/ze/hub/service_gnmi.go` (`//go:build ze_gnmi`) - `gnmiBuildImpl` + `gnmiReloadNotifyImpl` + moved `serveGNMI`/`waitForGNMIBind` + notifier package var
- `cmd/ze/hub/register_gnmi.go` (`//go:build ze_gnmi`) - `init(){ setGNMIInfra(...) }`
- `cmd/ze/hub/build_tag_gnmi_present_test.go` (`//go:build ze_gnmi`), `build_tag_gnmi_absent_test.go` (`//go:build !ze_gnmi`)
- `internal/component/plugin/all/all_ze_gnmi.go` (generated, `//go:build ze_gnmi`)
- `test/parse/gnmi-absent-config.ci` (if expressible)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior, Assumptions (validate A-1/A-2/A-3/A-6 first) |
| 3. Wiring | Wiring Test - seam + build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase 1: seam + impl move**
   - Add `gnmi_infra.go` (seam); move the build block + serveGNMI/waitForGNMIBind +
     notifier into `service_gnmi.go` (gated); `register_gnmi.go` installs hooks; main.go
     calls the seam + the reload hook; remove the `zegnmi` import from always-on main.go.
   - Tests: `TestBuildTag_GNMI_Present/Absent`, `TestGNMIReloadNotify`.
   - Verify: `ze_gnmi` build identical; ze_core build compiles without gnmi.
2. **Phase 2: schema + RPC gating + tag wiring**
   - generator `featureTags` gates the feature package plus owned sidecars; regenerate
     `all_ze_gnmi.go`; four-place tag wiring; dep_audit derives DISABLEABLE from
     `feature-gates.txt`.
   - Verify: generator `--check` clean; dep_audit clean; `go tool nm` 0 gnmi symbols in
     ze_core, N in ze_gnmi.
3. **Phase 3: docs + config/command safety**
   - `docs/features.md`, `module-tiers.md`; validate A-4 (`gnmi {}` config) + A-5
     (`show gnmi` command) on a ze_core binary.
4. **Full verification** - `make ze-verify-changed`; nm-measure ze_core vs ze_gnmi.

### Failure Routing
| Failure | Route To |
|---------|----------|
| gnmi not omitted with tag off | residual always-on import (notifier/global) - dep_audit + nm (R-2/R-3) |
| generator emits one import not two | the two-dir featureTags - R-1/A-2 |
| no-gnmi main.go fails to compile | the reload-closure notifier coupling - R-3 |
| config/command panics in no-gnmi build | schema/RPC absence - A-4/A-5 |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| No always-on gnmi import | dep_audit `--check` clean; `search internal/component/gnmi` shows only gated/test files |
| Notifier decoupled | no always-on file names `*zegnmi.ChangeNotifier`; reload uses the hook |
| Import gating | `all_ze_gnmi.go` blank-imports `gnmi`, `gnmi/yang`, and `gnmi-cmd/yang` |
| Symbol absence | `go tool nm` on ze_core binary lists zero gnmi symbols |
| Command safety | `show gnmi` on a no-gnmi build is a clean "unknown command", not a panic |
| Test gating | gnmi-requiring tests gated/skip-guarded; default ze_core unit suite passes |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| gnmi seam + impl + registration | `ls cmd/ze/hub/gnmi_infra.go service_gnmi.go register_gnmi.go` |
| dep_audit DISABLEABLE entry | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_gnmi.go (three imports) | `ls internal/component/plugin/all/all_ze_gnmi.go`; grep all three imports; `--check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm` gnmi count = 0 |
| present/absent tests | `go test -tags 'ze_core ze_gnmi' -run TestBuildTag_GNMI` and `-tags ze_core` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| No auth bypass | gating gnmi removes the gRPC management surface entirely; a no-gnmi build exposes no gNMI endpoint |
| Token/TLS handling | gnmi token + TLS config stays inside the gated code; not reachable in a no-gnmi build |
| Show-command exposure | "show gnmi" absent in no-gnmi build does not leak partial state |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| gNMI only had two blank imports to gate | `show gnmi` schema lives in `internal/plugins/gnmi-cmd/yang` and must be gated with the component and config schema | `make ze-doc-test` reported `ze-show:gnmi` schema with no handler after the handler package moved under `ze_gnmi` | Added a second `ze_gnmi` manifest row for the command-schema sidecar and regenerated `all_ze_gnmi.go` with three imports |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- gNMI shows the seam-vs-registry choice is about COUPLING SHAPE, not size: gnmi is small
  but its reload-closure notifier dependency + richer ctor deps + no-Reconfigure make a
  seam cleaner than the listener registry.
- A service can pin itself into the binary through MORE than its constructor: gnmi's
  "show gnmi" RPC is a second blank import that must be gated alongside the schema.

## Core Insight
Count every blank import a feature owns before gating it. gNMI has three owned
generated imports: component package registration, config schema, and command schema.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Dedicated seam (like ssh), not the construction registry | force gnmi into Service with a no-op Reconfigure | gnmi's reload-closure notifier coupling + richer ctor deps + no live-migration don't fit the registry's Reconfigurable+ServiceDeps contract |
| Gate gnmi, gnmi/yang, and gnmi-cmd/yang | gate only the config schema | the "show gnmi" RPC handler and command schema are registered by separate blank imports; gating only the config schema leaves the command surface inconsistent |
| Reload via nil-able `gnmiReloadNotify` hook | keep `*zegnmi.ChangeNotifier` in main.go | always-on code must name no gnmi type; the hook is the seam's reload entry |

## Known Limitations
- gNMI compile-out is independent of the other feature children.
- A no-gnmi build has no gNMI management interface; management is via ssh CLI / web / API.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| gnmi compile-out-able via `ze_gnmi` | build-tag test + nm symbol check | `TestBuildTag_GNMI_Absent` passes; `bin/ze-stripped: 0 gnmi symbols`; `bin/ze` and `bin/ze-appliance`: 65 gnmi symbols each |
| no always-on import of gnmi | audit | `dep_audit.py --check` clean; `internal/component/plugin/all/all.go` has no gnmi imports; `all_ze_gnmi.go` holds the gated imports |
| reload still notifies when compiled in | unit test | `TestGNMIReloadNotify` passes |
| default flavors keep gnmi | build | `make ze`, `make ze-appliance`, and `make ze-stripped` built; nm counts prove default/appliance keep gnmi and stripped drops it |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | (run /ze-review before implementation closure) | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/gnmi_infra.go` | yes | `read cmd/ze/hub/gnmi_infra.go:1-43` |
| `cmd/ze/hub/service_gnmi.go` | yes | `read cmd/ze/hub/service_gnmi.go:1-155` |
| `cmd/ze/hub/register_gnmi.go` | yes | `read cmd/ze/hub/register_gnmi.go:1-13` |
| `cmd/ze/hub/build_tag_gnmi_absent_test.go` | yes | `read cmd/ze/hub/build_tag_gnmi_absent_test.go:1-78` |
| `cmd/ze/hub/build_tag_gnmi_present_test.go` | yes | `read cmd/ze/hub/build_tag_gnmi_present_test.go:1-18` |
| `cmd/ze/hub/service_gnmi_test.go` | yes | `read cmd/ze/hub/service_gnmi_test.go:1-41` |
| `internal/component/plugin/all/all_ze_gnmi.go` | yes | `read internal/component/plugin/all/all_ze_gnmi.go:1-13` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `ze_gnmi` ON compiles gNMI through seam | `go test -tags 'ze_core ze_gnmi' -run 'TestBuildTag_GNMI|TestGNMIReloadNotify' ./cmd/ze/hub`; `go test -tags 'ze_core ze_gnmi' ./internal/component/gnmi`; `bin/ze: 65 gnmi symbols` |
| AC-2 | bare `ze_core` drops gNMI | `go test -tags ze_core -run TestBuildTag_GNMI ./cmd/ze/hub`; absent test builds `ze_core` and rejects all gNMI nm needles |
| AC-3 | always-on hub no longer names gNMI concrete types | `cmd/ze/hub/gnmi_infra.go:15-42` opaque seam; `cmd/ze/hub/main.go:615-616` reload hook; `cmd/ze/hub/main.go:828-839` build hook |
| AC-4 | generator gates all gNMI imports | `internal/component/plugin/all/all_ze_gnmi.go:9-12`; `all.go` has no gnmi import matches; `go run scripts/codegen/plugin_imports.go --check` passed |
| AC-5 | dep audit clean | `python3 scripts/dev/dep_audit.py --check` passed; `--selftest` passed |
| AC-6 | no-gnmi config safe | `TestBuildTag_GNMI_AbsentRejectsGNMIConfig` asserts clean unknown-field rejection |
| AC-7 | no-gnmi command safe | `TestBuildTag_GNMI_AbsentBinaryDropsGNMISymbolsAndCommand` asserts clean unknown-command rejection and no panic |
| AC-8 | default/appliance keep gnmi, stripped drops it | `make ze`, `make ze-appliance`, `make ze-stripped`; `go tool nm`: `bin/ze` 65, `bin/ze-appliance` 65, `bin/ze-stripped` 0 gNMI symbols |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, fixed | `make ze-doc-test` exposed the missing `internal/plugins/gnmi-cmd/yang` sidecar; `feature-gates.txt:33-34` now gates component and command-schema package |
| A-2 | confirmed | `scripts/codegen/plugin_imports.go:175-202` derives `<pkg>` and `<pkg>/yang`; generated `all_ze_gnmi.go:9-12` has all three imports |
| A-3 | confirmed | `cmd/ze/hub/gnmi_infra.go:21-29` uses generic inputs only; `cmd/ze/hub/main.go:828-839` passes only generic inputs |
| A-4 | confirmed | `TestBuildTag_GNMI_AbsentRejectsGNMIConfig` passed |
| A-5 | confirmed | `TestBuildTag_GNMI_AbsentBinaryDropsGNMISymbolsAndCommand` passed |
| A-6 | confirmed | `search ChangeNotifier|NotifyConfigReload|gnmiNotifier` found always-on references only through the seam |

## Checklist
### Goal Gates (MUST pass)
- [x] AC-1..AC-8 demonstrated
- [x] gnmi seam created; impl + reload hook moved; no always-on gnmi import
- [x] gnmi compile-out-able; present/absent build-tag tests pass
- [x] dep_audit DISABLEABLE clean
- [x] generator emits all_ze_gnmi.go (all gnmi imports); `--check` passes
- [x] `make ze-verify-changed` passes
- [x] `make ze-test` passes (lint + all ze tests)
- [x] Documentation Update Checklist answered with source evidence
- [x] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [x] Tests written
- [x] Tests FAIL: initial present test reported `ze_gnmi build: gNMI seam not installed`; initial absent test reported accepted gNMI config / retained gNMI symbol before gating
- [x] Tests PASS: present, absent, and reload tests pass under their build tags
- [x] Functional tests for end-to-end behavior (build-tag present/absent + config/command-safe)
- [x] Goal Validation table filled with concrete evidence
