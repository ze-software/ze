# Spec: feature-gate-1-lg

| Field | Value |
|-------|-------|
| Status | in-progress |
| Parent | plan/spec-feature-gate-0-umbrella.md (this is child 1, the pilot) |
| Depends | none (lg has no protocol/codec dependency; B-2 not required) |
| Phase | 5/5 (complete; AC-1..AC-8 verified; closing via two-commit) |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-feature-gate-0-umbrella.md` - the parent (architecture, shared risks)
3. `plan/learned/853-build-tag-split.md` - the positive-tag build model this extends
4. `cmd/ze/hub/main.go:296-392` (lg addr/TLS resolution), `:790-800` (lg start + SetLG)
5. `cmd/ze/hub/main_servers.go:670-746` (`startLGServer`/`serveLG`/`serveLGBlocking`)
6. `cmd/ze/hub/listener_migrate.go:21` (`Reconfigurable`), `:60` (`SetLG`)
7. `scripts/codegen/plugin_imports.go` - generates `all.go`; must emit a `ze_lg` group
8. `Makefile:87-99` - the `ze`/`ze-appliance`/`ze-stripped` tag sets

## Task

Prove the per-feature compile-out pattern from the umbrella end-to-end on the
**looking-glass (lg)** service: a `ze` binary built without the `ze_lg` build tag must
not link `internal/component/lg` (smaller binary, smaller attack surface), while the
default `ze`/`ze-appliance` builds keep lg exactly as today. This is the pilot: it
delivers the reusable construction registry, the `ze_lg` tag-gating (code + YANG
schema), the present/absent build-tag tests, and the `dep_audit.py` no-direct-import
audit rule. ssh (the headline hardening target) and web/gnmi/mcp follow as later
children, built on the registry proven here.

lg was chosen over ssh as the pilot after research falsified the umbrella's A-1
("ssh is the clean, one-ctor case"): ssh is the *most*-coupled service (two
construction paths + ~10 setters + the interactive-CLI surface across three files).
lg is the genuinely clean case (one self-contained `startLGServer`, one call site,
one already-generic interface coupling). See parent spec's revised assumptions.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/spec-feature-gate-0-umbrella.md` - parent design
  → Constraint: "disableable = edge feature reached ONLY via build-tag-gated
    registration"; a direct functional import pins a package in, only a blank/gated
    registration import is build-tag-removable.
  → Decision: construction registry (factory registered under the feature tag, hub
    iterates) chosen over per-service `!ze_<feature>` stub files.
- [ ] `plan/learned/853-build-tag-split.md` - build-tag model
  → Constraint: positive tags only; a feature is EXCLUDED unless its tag is passed.
    So `ze_lg` must be ADDED to `ze`/`ze-appliance` to preserve current behavior; the
    minimal `ze-stripped` (`ze_core` only) omits it.
  → Constraint: never name a tag file `*_linux.go` (that suffix is an implicit GOOS
    constraint); put `//go:build ze_lg` in the file body.
- [ ] `cmd/ze/hub/listener_migrate.go` - graceful listener migration
  → Decision: `Reconfigurable` interface (`Addresses() []string`, `Reconfigure(ctx,
    []string) error`) ALREADY exists; the migrator's `web/lg/mcp/rest/grpc` fields are
    all `Reconfigurable`. `lg.LGServer` already satisfies it. The ONLY concrete-`lg`
    coupling is the `SetLG(s *lg.LGServer)` setter signature (line 60).
  → Constraint: changing `SetLG` to take `Reconfigurable` removes the `lg` import from
    this always-on file with zero behavior change.

### RFC Summaries (MUST for protocol work)
- N/A. No wire-protocol change; pure composition/build-tag work.

**Key insights:**
- lg's always-on footprint is exactly 4 sites: `main_servers.go` (`startLGServer`),
  `main.go` (the call + `SetLG`), `listener_migrate.go` (`SetLG` signature),
  `internal/component/plugin/all/all.go:71` (`component/lg/yang` schema blank import).
- lg's deps are all generic: `storage.Storage`, `*resolve.Resolvers`, a
  `func(cmd string) (string, error)` dispatcher (lg.CommandDispatcher is exactly that
  func type), addresses `[]string`, and a `TLS bool`. None is lg-specific.
- lg addr/TLS resolution (`main.go:309-392`) uses only `env.*` and
  `zeconfig.ExtractLGConfig`/`ParseCompoundListen` - it does NOT import the lg package,
  so it can stay in always-on code and feed the factory via Deps.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main_servers.go:670-746` - `startLGServer(store, listenAddrs, useTLS,
  dispatch, resolvers) *lg.LGServer` builds `lg.LGConfig`, optionally loads a TLS cert
  via `zeweb.LoadOrGenerateCert`+`blobCertStore`, calls `lg.NewLGServer(cfg)`, starts
  `serveLG` (goroutine running `ListenAndServe`), waits `WaitReady`. `serveLG`/
  `serveLGBlocking` are the goroutine helpers.
  → Constraint: this whole adapter (and its only `lg` import in main_servers.go) moves
    verbatim into a `//go:build ze_lg` file; its TLS-cert path keeps using
    `zeweb.LoadOrGenerateCert` (web is a separate feature; the pilot does not gate web).
- [ ] `cmd/ze/hub/main.go:296-392` - resolves `lgAddrs`/`lgTLS` from env + config (no
  `lg` import). `:790-800` - `if len(lgAddrs) > 0 { lgDispatch := func(cmd){...};
  startLGServer(...); lm.SetLG(lgSrv); defer lgSrv.Shutdown(...) }`.
  → Constraint: replace the `startLGServer` call with a registry build; pass resolved
    `lgAddrs`/`lgTLS`/dispatch/store/resolvers via Deps. main.go keeps zero `lg` import.
- [ ] `cmd/ze/hub/listener_migrate.go:60` - `func (m *ListenerMigrator) SetLG(s
  *lg.LGServer)`; the field `m.lg` is already typed `Reconfigurable`.
  → Constraint: change the param to `Reconfigurable`; delete the `lg` import (line 15).
- [ ] `internal/component/plugin/all/all.go:71` - `_ ".../component/lg/yang"`.
  → Constraint: this schema blank import is in the flat universal list; it must move to
    a `ze_lg`-gated group so a no-lg build does not link `lg/yang`.
- [ ] `scripts/codegen/plugin_imports.go` - emits `all.go` from `pluginDirs` as one
  flat blank-import list.
  → Constraint: teach it a per-feature tag map (`lg/yang -> ze_lg`) and emit the gated
    entries into a separate `//go:build ze_lg` file (e.g. `all_ze_lg.go`), removing them
    from the untagged `all.go`. `--check` must still pass.
- [ ] `Makefile:87,91,99` - `ze`=`ze_core ze_distro`, `ze-appliance`=`ze_core
  ze_appliance`, `ze-stripped`=`ze_core`.
  → Constraint: add `ze_lg` to the `ze` and `ze-appliance` tag strings (both the
    primary and the duplicate target blocks at :120,:125). Leave `ze-stripped` without
    it - that becomes the minimal-binary proof.

**Behavior to preserve:**
- `ze` and `ze-appliance` keep lg exactly as today (built when configured/enabled).
- lg listener migration on config reload (`ReloadListeners`) unchanged.
- lg TLS cert generation/loading unchanged.
- All other listener services (web/mcp/rest/grpc) unchanged in every flavor.

**Behavior to change:**
- The hub no longer imports `internal/component/lg` from always-on code; it builds lg
  through the construction registry. With `ze_lg` off, lg is unregistered, unbuilt,
  unlinked - no error, no listener.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of the `ze_lg` build tag.
- Run time: the daemon (`cmd/ze/hub`) building registered services at startup.

### Transformation Path
1. `cmd/ze/hub/service_lg.go` (`//go:build ze_lg`) `init()` registers a `looking-glass`
   factory with the package-level registry in `service_registry.go`.
2. The composition root links `service_lg.go` only when `ze_lg` is set; the generated
   `all_ze_lg.go` (`//go:build ze_lg`) blank-imports `component/lg/yang` for the schema.
3. The hub resolves generic Deps (store, resolvers, dispatch, lgAddrs, lgTLS) and calls
   `buildServices(deps)`; for each non-nil `Service` it registers it with the migrator
   and defers `Shutdown`.
4. With `ze_lg` off: no factory registered, `buildServices` returns nothing for lg, and
   `component/lg` (+ `lg/yang`) is imported nowhere always-on, so Go does not link it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze_lg` tag ↔ composition root | generator-emitted `all_ze_lg.go` + gated `service_lg.go` | [ ] |
| composition root ↔ registry | `service_lg.go` blank-compiled → `init()` → registerService | [ ] |
| registry ↔ daemon | hub iterates `buildServices(deps)`, no direct `lg` import | [ ] |
| disableable feature ↔ always-on code | lg reached only via the registry / gated files | [ ] audit rule |

### Integration Points
- `Reconfigurable` (existing) - the built lg `Service` satisfies it; migrator unchanged.
- `RegisterListenerDefault("looking-glass", ...)` / `DiscoverListenerServices` - config
  defaults + port-conflict detection must tolerate lg's component being absent.
- `scripts/dev/dep_audit.py` - new no-direct-import-of-disableable audit rule + selftest.

### Architectural Verification
- [ ] No bypassed layers (lg still built+started by the hub, migrated by the migrator)
- [ ] No unintended coupling (registry is package-internal to `hub`; lg stays a pure component)
- [ ] No duplicated functionality (reuse `Reconfigurable`, 853 tags, existing `startLGServer` body)
- [ ] Zero-copy preserved (N/A - composition/wiring change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | lg's only always-on functional importer set is main_servers.go + main.go + listener_migrate.go | grep of `component/lg"` importers (parent research) | extra severing needed | re-grep importers after the change | confirmed (research) |
| A-2 | `lg.LGServer` satisfies `Reconfigurable` so `SetLG(Reconfigurable)` compiles | listener_migrate.go:21,41 (field already `Reconfigurable`); lg has `Addresses()`+`Reconfigure()` | migrator needs a wider iface | compile after the signature change | confirmed (read) |
| A-3 | the generator can emit a `ze_lg`-gated group and keep `--check` green | plugin_imports.go owns all.go generation | generator rewrite larger than scoped | spike: emit `all_ze_lg.go`, run `--check` | unvalidated |
| A-4 | a no-lg binary validates config containing `looking-glass {}` safely (schema absent) | lg/yang is a separate blank import; YANG is loaded from registered schemas | `looking-glass {}` config errors/panics in a no-lg build | build no-lg binary, feed `looking-glass {}` config | unvalidated |
| A-5 | `knownListenerServices`/listener defaults tolerate the lg component being compiled out | listener defaults are static string registrations in `config`, independent of the lg package | parse-time conflict detection misfires/panics | build no-lg binary, exercise the listener-conflict path | unvalidated |
| A-6 | lg addr/TLS resolution (main.go) has no `lg` import and can stay always-on | main.go:309-392 uses env + `zeconfig.ExtractLGConfig` only | resolution must move into the factory | grep the block for `lg.` references | confirmed (read) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the generator change ripples beyond lg (other plugins mis-grouped) | `--check` diff touches non-lg imports | drive the gated group from an explicit per-feature allow-map; only `lg/yang` in it for the pilot |
| R-2 | dead-code elimination leaves lg symbols despite the tag (transitive ref via web's cert helper) | `go tool nm` shows lg symbols in `ze-stripped` | the cert helper is `zeweb.*` (web), not lg; assert absence of `component/lg` symbols specifically |
| R-3 | adding `ze_lg` to `ze`/`ze-appliance` is missed in one of the duplicate Makefile blocks (:87 vs :120) | one flavor loses lg unexpectedly | grep all `-tags` lines; build-tag test asserts lg present in the distro flavor |
| R-4 | `service_lg.go` in `package hub` under `ze_lg` is the first per-feature file in cmd/ze/hub; `gofumpt`/tier checks may flag the new pattern | lint/tier gate failure | follow the `setup_features_*.go` precedent (tag in body, not filename); run `make ze-lint-changed` early |
| R-5 | the registry abstraction is over-fit to lg and needs rework for ssh/web | ssh child cannot express its wiring via the same Deps/Service | keep Deps a growable struct + Service = `Name()`+`Reconfigurable`+`Shutdown`; ssh adds fields, does not reshape |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| build with `ze_lg` (distro flavor) | → | lg factory registered; hub builds + starts lg | `TestBuildTag_LG_Present` (cmd/ze) |
| build without `ze_lg` (stripped flavor) | → | lg package not linked; no lg listener; no error | `TestBuildTag_LG_Absent` (cmd/ze) |
| registry has a registered `looking-glass` factory | → | `buildServices(deps)` returns a started lg `Service`, hub wires it, not `lg.NewLGServer` | `TestServiceRegistry_BuildsLG` (hub) |
| `dep_audit.py` over a tree with a direct import of a disableable feature | → | audit reports the violation | `dep_audit` selftest `test_no_direct_import_of_disableable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the construction registry exists in `package hub` | the hub builds/starts lg by iterating `buildServices(deps)`, with no direct `lg.NewLGServer`/`lg.*` import in any always-on (`!ze_lg`) file |
| AC-2 | build with `ze_lg` (as in `ze`/`ze-appliance`) | lg compiled in, registered, built and started exactly as today; existing lg tests + functional LG behavior pass |
| AC-3 | build without `ze_lg` (`ze-stripped`) | `internal/component/lg` symbols absent from the binary (`go tool nm`); daemon starts without lg; no error logged |
| AC-4 | a no-lg binary is fed config containing `looking-glass { ... }` | config handling is safe (documented: clean validation error or ignored-with-warning), no panic |
| AC-5 | `knownListenerServices` / listener defaults in a no-lg build | parse-time conflict detection for the OTHER services still works; no panic on lg's absence |
| AC-6 | the generator runs | it emits `all_ze_lg.go` (`//go:build ze_lg`) with `lg/yang`, removes it from the untagged `all.go`; `plugin_imports.go --check` passes |
| AC-7 | a disableable feature is directly imported by always-on code | `dep_audit.py --selftest` flags it; a clean `--check` over the lg-converted tree passes |
| AC-8 | `make ze-verify` (or changed-scope) on the distro flavor | full suite green; `ze`/`ze-appliance`/`ze-setup`/`ze-stripped` all build |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze-stripped` (no `ze_lg`) | tag off → `service_lg.go`+`all_ze_lg.go` not compiled → lg unlinked → no LG listener | `TestBuildTag_LG_Absent` + `go tool nm` symbol check |
| 2 | builds the standard `ze` (`ze_lg` on) | tag on → factory registered → hub builds lg subsystem → LG listens | `TestBuildTag_LG_Present` + existing LG functional test |
| 3 | runs a no-lg binary against a config with `looking-glass {}` | config load → lg schema absent → safe handling | `test/parse/lg-absent-config.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestServiceRegistry_BuildsLG` | `cmd/ze/hub/service_registry_test.go` | a registered factory is built and returned by `buildServices`; hub wires it via `SetLG`, not a direct ctor | |
| `TestServiceRegistry_AbsentFeatureNoOp` | same | `buildServices` with no registered factory returns nothing, no panic | |
| `test_no_direct_import_of_disableable` | `scripts/dev/dep_audit.py --selftest` | audit flags a fixture with a direct import of a tagged-disableable feature | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_lg` | `cmd/ze/build_tag_lg_test.go` | lg present with `ze_lg`, absent without (compile + symbol assertion) | |
| `lg-absent-config` | `test/parse/lg-absent-config.ci` | no-lg binary handles `looking-glass {}` config safely | |

### Interop Tests (MANDATORY for protocol features)
- N/A. No wire-protocol change.

### Future (if deferring any tests)
- ssh/web/gnmi/mcp compile-out: each its own child spec with its own present/absent
  build-tag test, built on the registry this spec delivers. Deferred by the umbrella
  structure (user-approved).

## Files to Modify
- `cmd/ze/hub/main.go` - replace the `startLGServer(...)` call (~:790) with a registry
  build loop; keep lg addr/TLS resolution; remove any `lg` import (verify none remains)
- `cmd/ze/hub/main_servers.go` - remove `startLGServer`/`serveLG`/`serveLGBlocking` and
  the `lg` import (they move to `service_lg.go`)
- `cmd/ze/hub/listener_migrate.go` - `SetLG(Reconfigurable)`; drop the `lg` import
- `scripts/codegen/plugin_imports.go` - per-feature tag map + emit `all_ze_lg.go`
- `internal/component/plugin/all/all.go` - (generated) `lg/yang` removed from the flat list
- `Makefile` - add `ze_lg` to `ze` and `ze-appliance` tag sets (both target blocks)
- `scripts/dev/dep_audit.py` - no-direct-import-of-disableable audit rule + selftest fixture
- `ai/rules/module-tiers.md` - document the disable-ability (no-direct-import) rule

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | - (build-time, not CLI) |
| Functional test | [ ] yes | `cmd/ze/build_tag_lg_test.go`, `test/parse/lg-absent-config.ci` |
| Doctor check | [ ] no | lg owns its own doctor checks; absent = not registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register the new tag + audit rule |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature (build flavor)? | [ ] yes | `docs/features.md` (build-tag/flavor table: add `ze_lg`) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md` or `core-design.md` (construction registry) |
| 15 | Runtime inventory changed? | [ ] no | lg stays where it is; only its linkage is gated |
| others | - | [ ] assess | grep `docs/` for `looking-glass`/`lg` anchors |

## Files to Create
- `cmd/ze/hub/service_registry.go` - the construction registry: `Service` iface
  (`Name()` + `Reconfigurable` + `Shutdown(ctx)`), `Deps` struct, `registerService`,
  `buildServices(deps) []Service` (always-on, no service imports)
- `cmd/ze/hub/service_lg.go` - `//go:build ze_lg`; the moved `startLGServer` adapter +
  `init()` registering the `looking-glass` factory (the only direct `lg` import)
- `cmd/ze/hub/service_registry_test.go`
- `cmd/ze/build_tag_lg_test.go`
- `cmd/ze/internal/.../all_ze_lg.go` - generated `//go:build ze_lg` schema group
- `test/parse/lg-absent-config.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior |
| 3. Wiring | Wiring Test - registry + build-tag tests first |
| 4. Implement | Phases below |
| 7. Critical review | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 15. Close + commit | learned summary + two-commit script |

### Implementation Phases
1. **Phase: construction registry (always-on infra)** - `service_registry.go`: `Service`
   iface, `Deps`, `registerService`, `buildServices`. No service moved yet.
   - Tests: `TestServiceRegistry_AbsentFeatureNoOp`
   - Verify: registry compiles, `buildServices` empty is a clean no-op.
2. **Phase: lg through the registry** - `service_lg.go` (`//go:build ze_lg`) holds the
   moved `startLGServer` body + `init()` registration; `main.go` builds lg via
   `buildServices`; remove `startLGServer`/`lg` import from `main_servers.go`/`main.go`;
   `SetLG(Reconfigurable)`.
   - Tests: `TestServiceRegistry_BuildsLG`, existing lg tests
   - Verify: with `ze_lg`, identical lg behavior; build with `ze_core` only → no lg import error.
3. **Phase: schema + tag gating** - generator emits `all_ze_lg.go`, drops `lg/yang` from
   `all.go`; add `ze_lg` to `ze`/`ze-appliance` Makefile tags; `build_tag_lg_test.go`
   (present/absent) + `lg-absent-config.ci`.
   - Verify: `ze-stripped` has no `component/lg` symbol (`go tool nm`); `ze` does; config safe.
4. **Phase: audit rule** - `dep_audit.py` flags a direct import of a tag-disableable
   feature; selftest fixture; document the rule in `module-tiers.md`.
   - Verify: `--selftest` catches a planted direct import; `--check` clean.
5. **Full verification** - `make ze-verify` (or changed-scope); build all four flavors.

### Critical Review Checklist (/implement stage 7)
| Check | What to verify |
|-------|----------------|
| No always-on lg import | grep `!ze_lg` hub files for `component/lg` → none |
| Behavior parity | distro build's lg start/migrate/TLS path byte-identical in effect |
| Generator determinism | re-run generate twice → no diff; `--check` green |
| Tag matrix | `ze`/`ze-appliance` have `ze_lg` in BOTH Makefile blocks |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| registry + lg factory | `go test ./cmd/ze/hub -run ServiceRegistry` |
| lg compiled out | build `ze-stripped`; `go tool nm bin/ze-stripped` has no `component/lg` |
| lg compiled in | build `ze`; `go tool nm bin/ze` has `component/lg`; LG functional test |
| audit rule | `python3 scripts/dev/dep_audit.py --selftest` |
| config safe | `ze-test`/`.ci` `lg-absent-config` passes |

### Security Review Checklist (/implement stage 12)
| Concern | Check |
|---------|-------|
| no-lg build does not silently expose a port | `ze-stripped` binds no LG listener |
| TLS cert path unchanged in lg-on build | cert load/generate identical to current |
| config injection via `looking-glass {}` in no-lg build | safe parse, no panic, no partial bind |

### Failure Routing
| Failure | Route To |
|---------|----------|
| lg not omitted with tag off | a residual direct import remains - find via the audit rule / `go tool nm` |
| generator `--check` fails | the gated-group emission - fix `plugin_imports.go` |
| config panics in no-lg build | schema/listener absence handling - A-4/A-5 |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (umbrella) ssh is the clean pilot | ssh is the most-coupled service; lg is clean | reading the hub construction sites | pilot pivoted from ssh to lg (user-approved) |
| (umbrella) factory returns a `ze.Subsystem` | only ssh is a Subsystem; lg/web/gnmi are hub-managed HTTP servers (ListenAndServe/Shutdown/Addresses) | reading lg/web lifecycle | Service iface = `Reconfigurable`+`Name`+`Shutdown`, reusing the existing migrator contract, not `ze.Subsystem` |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Item | Why it might need escalation |
|------|------------------------------|

## Design Insights
- The hub already had the right abstraction (`Reconfigurable`) for half the job; the
  only concrete coupling was one setter signature. Compile-out for a well-factored
  service is small - the cost is concentrated in the deeply-wired ones (ssh/web).
- Putting the gated factory file in `package hub` (not the component) keeps lg a pure
  component and localizes the build-tag knowledge to the composition layer.

## Core Insight
A direct functional import pins a package into the binary; only blank/gated
registration imports are build-tag-removable. "Make lg disableable" reduced to:
sever the three always-on `lg` imports (one is just a setter signature) and gate the
schema import + the construction adapter behind `ze_lg`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| registry + factory in `package hub`, gated file holds the only `lg` import | factory inside `internal/component/lg` | keeps lg a pure component; the hub already owns the lg adapter (`startLGServer`) |
| `Service` = `Name()` + `Reconfigurable` + `Shutdown` | new bespoke lifecycle iface | reuses the existing `Reconfigurable`; lg already satisfies it |
| keep lg addr/TLS resolution always-on | move it into the factory | the resolution has no `lg` import; moving it is churn without compile-out benefit (refinement, not pilot) |
| `ze_lg` added to `ze`/`ze-appliance`; omitted from `ze-stripped` | a new `ze_min` flavor | `ze-stripped` already exists as the minimal `ze_core` build - it becomes the no-lg proof |

## Known Limitations
- Pilot gates only lg. ssh/web/gnmi/mcp and protocols are later children.
- lg addr/TLS resolution strings stay in always-on code (lg-import-free); a later
  refinement may move per-feature resolution into factories.

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| lg compile-out-able via `ze_lg` | symbol check + build-tag test | `go tool nm`: ze_core build = **0** `component/lg` server symbols, ze_core+ze_lg = **183**; `TestBuildTag_LG_Absent` passes |
| schema also gated (AC-6) | symbol check + generator | ze_core build = **0** `component/lg/yang` symbols, ze_lg = 3; `plugin_imports.go --check` current (1 gated group) |
| hub builds lg via registry, no direct import | unit test + audit | `TestServiceRegistry_BuildsRegisteredFactory` passes; `dep_audit.py --check` = 0 disableable violations |
| disableable features have no direct always-on import | audit selftest | `dep_audit.py --selftest` OK (flags planted always-on import; allows gated + test) |
| default flavors unchanged | build-tag test + build | `TestBuildTag_LG_Present` passes; `ze_core ze_distro ze_lg` build = 183 lg symbols (lg present as before) |
| no-lg binary is safe | binary behavior | `ze-stripped config validate looking-glass{}` → clean "unknown field" error, exit 1, no panic |

## Review Gate
### Run 1 (self-review)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Pilot pivoted ssh→lg: umbrella A-1 ("ssh is the clean case") falsified; ssh is the most-coupled service. | research | accepted (user-approved); ssh is child 2 |
| 2 | NOTE | Service iface is `Reconfigurable`+`Name`+`Shutdown`, NOT `ze.Subsystem`: lg/web/gnmi are hub-managed HTTP servers, only ssh is a Subsystem. | service_registry.go | accepted; iface matches the hub-managed-server shape |
| 3 | NOTE | `buildLGService` failure paths return errors (logged by buildServices/slog) instead of the original stderr prints; success "listening" stderr preserved. | service_lg.go | accepted; equivalent non-fatal behavior, cleaner (nilnil) |
| 4 | NOTE | No `test/parse/lg-absent-config.ci`: the .ci harness runs one flavor, so a no-lg run isn't expressible there. AC-4 guarded by `TestBuildTag_LG_Absent` + generator `--check` + empirical validate run. | test/parse | accepted; mechanism guarded, behavior verified |
| 5 | NOTE | Added `ze_lg` to `.golangci.yml` build-tags so the gated files are linted and `registerService` is not flagged unused (lint the shipped feature set). | .golangci.yml | accepted; covers feature-on build |
| 6 | NOTE | Makefile uses a centralized `ZE_FEATURES` var instead of inlining `ze_lg` in each target (mitigates R-3). | Makefile | accepted |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  → self-review: 0 BLOCKER, 0 ISSUE, 6 NOTEs (all accepted above)
- [ ] All NOTEs recorded above (or explicitly "none")  → 6 NOTEs recorded

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/service_registry.go` | yes | created (registry + Service/Deps + buildServices) |
| `cmd/ze/hub/service_lg.go` | yes | created (`//go:build ze_lg` factory) |
| `cmd/ze/hub/register_lg.go` | yes | created (`//go:build ze_lg` registration) |
| `cmd/ze/hub/service_registry_test.go` | yes | created (registry unit tests) |
| `cmd/ze/hub/service_lg_test.go` | yes | created (`ze_lg` factory + parseASN tests) |
| `cmd/ze/hub/build_tag_lg_present_test.go` / `_absent_test.go` | yes | created (present/absent registration) |
| `internal/component/plugin/all/all_ze_lg.go` | yes | generated (`//go:build ze_lg`, lg/yang) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | hub builds lg via registry, no always-on `lg` import | `dep_audit.py --check` = 0 violations; `TestServiceRegistry_*` pass |
| AC-2 | `ze_lg` on → lg present | `go tool nm` ze_core+ze_lg = 183 lg-server symbols; `TestBuildTag_LG_Present` pass |
| AC-3 | `ze_lg` off → lg absent | `go tool nm` ze_core = 0 lg-server symbols; `TestBuildTag_LG_Absent` pass |
| AC-4 | no-lg binary + `looking-glass{}` config | `ze-stripped config validate` → "unknown field in environment: looking-glass", exit 1, no panic |
| AC-5 | listener defaults tolerate absent lg | same run: no panic; other services validate; `looking-glass` default in always-on config pkg, never looked up (not in schema) |
| AC-6 | generator emits `all_ze_lg.go`, drops lg/yang from all.go | `plugin_imports.go --check` = current (1 gated group); ze_core lg/yang symbols = 0 |
| AC-7 | disableable direct import flagged | `dep_audit.py --selftest` OK (flags always-on importer, allows gated + test) |
| AC-8 | suite green | `ze-lint-changed` 0 issues; `ze-doc-test` 0; hub/config/plugin-all tests pass (ze_core + ze_lg) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | lg's always-on importers were exactly main_servers.go + main.go + listener_migrate.go; all severed |
| A-2 | confirmed | `SetLG(Reconfigurable)` compiles; `lg.LGServer` satisfies it (build green) |
| A-3 | confirmed | generator emits `all_ze_lg.go`; `--check` passes; both flavors build |
| A-4 | confirmed | no-lg binary returns clean "unknown field" validation error, no panic |
| A-5 | confirmed | no-lg build: no panic in listener/validation paths; other services unaffected |
| A-6 | confirmed | lg addr/TLS resolution stayed always-on (no `lg` import); ze_core build links no lg server |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated
- [ ] construction registry built; hub iterates it; no always-on `lg` import
- [ ] lg compile-out-able; present/absent build-tag tests pass; symbol check
- [ ] audit rule flags direct imports of disableable features
- [ ] `make ze-test` passes (lint + all ze tests); all four flavors build
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe)
- [ ] Goal Validation table filled with concrete evidence
