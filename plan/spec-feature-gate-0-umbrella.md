# Spec: feature-gate-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | plan/spec-tiers-0-umbrella.md (B-2 codec extraction unblocks protocol compile-out) |
| Phase | 0 (umbrella ready; next: /ze-implement the ssh pilot = child 1) |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/853-build-tag-split.md` - the build-tag model this extends
4. `plan/learned/452-ssh-server.md` - ssh is a `ze.Subsystem` (pilot target)
5. `plan/learned/554-named-service-listeners.md` - service binders + runtime `enabled` leaf
6. `cmd/ze/hub/infra_setup.go` (ssh ctor), `cmd/ze/hub/main_servers.go` (web/gnmi ctors)
7. `cmd/ze/setup_features_setup.go` - the per-flavor blank-import pattern
8. `ai/rules/module-tiers.md` + `scripts/dev/dep_audit.py` - the placement gate to extend

## Task

Make any optional **service** (web, ssh, gnmi, mcp, looking-glass, monitoring) and
optional **protocol** (isis, ldp, ospf, rsvpte, ...) **compile-out-able** from the
`ze` binary via a per-feature `ze_<feature>` build tag, for a smaller binary and a
smaller attack surface. Organizing principle: **disableable = edge feature reached
ONLY via build-tag-gated registration** -- the same axis as the module-tier rule's
axis B (nothing always-compiled depends on it). A direct functional `import` from
always-on code pins a package into the binary; only a blank registration import can
be dropped by a build tag.

This is an **umbrella**. Child 1 proves the whole pattern end-to-end on `ssh` (the
cleanest case: one ctor call, daemon-only). Later child specs generalize to `web`
(bespoke, ~30 hub call-sites), `gnmi`, `mcp`, `lg`, monitoring, and the optional
protocols (already edge plugins; they need only tag-gated blank-imports once B-2
lets a protocol's codec come in without its engine).

Reuses, does NOT reinvent: `ze.Subsystem` lifecycle (452); the 853 build-tag model
(positive tags, default-minimal, `setup_features_*.go` groups, per-flavor tests),
adding finer `ze_<feature>` granularity; the 554 listener binders + runtime `enabled`
leaf (runtime disable exists; this adds compile-time removal).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/853-build-tag-split.md` - the established build-tag model
  → Constraint: positive tags only (`ze_distro`/`ze_appliance`/`ze_setup`), default
    (no tag) = minimal; new feature code is EXCLUDED unless explicitly tagged.
  → Constraint: per-flavor feature sets live in `cmd/ze/setup_features_<flavor>.go`
    as blank-import lists; each has a `build_tag_<flavor>_test.go` asserting
    presence/absence with compound constraints (`ze_distro && !ze_appliance && ...`).
  → Constraint: `_linux.go` filename suffix is an implicit GOOS=linux constraint --
    never name a tag file `*_linux.go`; use the tag in the file body.
  → Decision: 853 deliberately chose COARSE flavor tags over fine-grained ones; the
    per-feature granularity here multiplies the build-tag test matrix -- the matrix is
    the main cost (see R-1).
- [ ] `plan/learned/452-ssh-server.md` - ssh subsystem
  → Constraint: ssh is a `ze.Subsystem` (Start/Stop/Reload managed by Engine);
    constructed via `ssh.NewServer(cfg)` at `internal/component/ssh/ssh.go:166`.
  → Constraint: schema registration is a blank import of `ssh/yang` in `all.go`; the
    ze-ssh-conf schema is loaded in `YANGSchemaWithPlugins()`. Compiling ssh out must
    also drop its schema registration, or config referencing `ssh {}` mis-validates.
- [ ] `plan/learned/554-named-service-listeners.md` - service binders + enabled leaf
  → Constraint: web/ssh/lg/mcp/telemetry/api share a listener-binder shape with
    `Addresses()`/`Address()`; each has a runtime `enabled` leaf and an entry in
    `knownListenerServices` (config port-conflict table). Compiling a service out must
    not break `knownListenerServices` parse-time conflict detection for the others.
  → Decision: runtime disable already exists (`enabled` leaf). This umbrella is ONLY
    about compile-time removal (smaller binary / attack surface), a distinct axis.
- [ ] `ai/rules/module-tiers.md` - the placement gate
  → Constraint: the gate (dep_audit.py) is Path C engine-only; B-1 made the advisory
    reuse composition-root wiring. The new "no direct import from always-on code into
    a disableable feature" audit extends this advisory/gate.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: a disableable feature must be self-contained; removing its blank
    import (under its build tag) removes ALL its code from the binary.

### RFC Summaries (MUST for protocol work)
- N/A for the umbrella + ssh pilot (no wire-protocol change). Protocol-compile-out
  child specs reference their protocol's existing RFC summaries; no new RFC behavior.

**Key insights:**
- Two composition roots today: generated `all.go` (universal, `//go:build ze_core`,
  via `cmd/ze/ze_core_dispatch.go`) and hand-written `setup_features_*.go` (per-flavor).
- The blocker is the daemon's DIRECT construction imports (`ssh.NewServer`,
  `web.NewWebServer`, gnmi `NewServer`), not the registration imports.
- The fix is a construction registry: a service registers a factory under its
  `ze_<feature>` tag; the hub builds services by iterating the registry, never a
  direct `New*` call.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/infra_setup.go` - builds the ssh server via `zessh.NewServer(cfg)`
  (a single ctor call at ~line 162); ssh is daemon-only (no other importer).
  → Constraint: ssh's only always-on importer is the hub; severing one ctor call +
    gating one blank import compiles it out. This is why ssh is the pilot.
- [ ] `cmd/ze/hub/main_servers.go` - builds web via ~30 `zeweb.*` calls
  (`NewWebServer`, `NewRenderer`, `NewDecoratorRegistry`, handlers, broker,
  `NewEditorManager`, session store, cert) and starts gnmi.
  → Constraint: web is deeply wired -- its registry factory must encapsulate ALL this
    construction behind one entry point. Bespoke, deferred to the web child spec.
- [ ] `cmd/ze/ze_core_dispatch.go` - blank-imports `plugin/all` under `//go:build ze_core`.
  → Constraint: `all.go` is the universal plugin set; per-feature compile-out needs
    the composition root partitioned by `ze_<feature>` tags (generator change).
- [ ] `cmd/ze/setup_features_setup.go` - `//go:build ze_setup` blank-import group.
  → Constraint: the precedent shape for a tag-gated blank-import group; the generator
    should emit analogous per-feature groups instead of hand maintenance.
- [ ] `scripts/codegen/plugin_imports.go` - generates `all.go` from `pluginDirs`.
  → Constraint: the generator emits ONE flat blank-import list; per-feature gating
    requires it to emit tag-annotated groups (or per-feature files).
- [ ] `internal/component/config/listener_defaults.go` / `listener.go` -
  `RegisterListenerDefault` + `knownListenerServices`.
  → Constraint: a compiled-out service's listener-default registration vanishes; the
    conflict table and defaults must tolerate an absent service (no panic, no false
    conflict).

**Behavior to preserve:**
- The default `make build` binaries (`ze`, `ze-appliance`, `ze-setup`, `ze-stripped`)
  keep their current feature sets unless a tag explicitly removes a feature.
- `ze.Subsystem` Start/Stop/Reload semantics for every service.
- Runtime `enabled` leaf behavior (554) and named-listener multi-bind.
- Config validation for a service's YANG when the service IS compiled in.

**Behavior to change:**
- The daemon stops constructing optional services via direct `New*` imports; it builds
  them through a construction registry. A service absent from the build (its tag off)
  is simply not registered and not started -- no error.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: the set of `//go:build ze_<feature>` tags passed to `go build`.
- Run time: the daemon (`cmd/ze/hub`) starting registered services.

### Transformation Path
1. A service package's `init()` (under `//go:build ze_<feature>`) registers a factory
   with the new construction registry.
2. The composition root blank-imports the service package only inside a
   `ze_<feature>`-gated group (generator-emitted), so an off tag drops the import.
3. At startup the hub iterates the construction registry and builds/starts each
   registered service via the `ze.Subsystem` lifecycle -- no direct `New*` import.
4. With the tag off, the package is unimported; Go does not compile or link it (its
   transitive deps drop too) -- the compile-out.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `ze_<feature>` import groups | [ ] |
| Composition root ↔ construction registry | blank import → `init()` → register factory | [ ] |
| Construction registry ↔ daemon | hub iterates registry, builds via factory iface | [ ] |
| Disableable feature ↔ always-on code | MUST be registry-only, never a direct import | [ ] audit rule |

### Integration Points
- `ze.Subsystem` (existing) - the factory returns one; Engine manages its lifecycle.
- `knownListenerServices` / `RegisterListenerDefault` - tolerate absent services.
- `scripts/dev/dep_audit.py` - new audit rule enforcing no-direct-import.

### Architectural Verification
- [ ] No bypassed layers (services still run via `ze.Subsystem`/Engine)
- [ ] No unintended coupling (the registry is the only new always-on surface)
- [ ] No duplicated functionality (reuse 853 tags, 554 binders, 452 subsystem)
- [ ] Zero-copy preserved (N/A - composition/wiring change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ssh has exactly one always-on importer (the hub) so it is a clean pilot | grep: only `cmd/ze/hub/*` imports `internal/component/ssh` | pilot is not representative; more severing needed | re-grep importers of ssh before coding | unvalidated |
| A-2 | the generator can emit `ze_<feature>` tag-gated blank-import groups without breaking `--check` | `plugin_imports.go` owns all.go generation | generator rewrite larger than scoped | spike: emit one gated group, run `--check` | unvalidated |
| A-3 | a service compiled out leaves config validation safe (schema not registered) | 452: schema is a separate blank import in all.go | `ssh {}` config mis-validates or panics in a no-ssh build | build no-ssh binary, feed `ssh {}` config | unvalidated |
| A-4 | `knownListenerServices` tolerates an absent service | 554: static table keyed by name | parse-time conflict detection panics/misfires | build no-ssh binary, run listener-conflict path | unvalidated |
| A-5 | the daemon can build services via a factory iface without a direct import | hub currently calls `ssh.NewServer` directly | the iface cannot express web's deep wiring | prototype the iface against ssh first | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | per-feature tags multiply the build-tag test matrix (853 already found this painful at 4 flavors) | combinatorial `build_tag_*_test.go` explosion | one focused test per feature (compiles-in / compiles-out), not full cross-product; a single `ze_min` umbrella tag for "all optional off" |
| R-2 | web's ~30-call construction does not fit a generic factory iface | web child spec stalls | iface designed against ssh first; web factory may keep a bespoke internal builder, exposing only one entry point |
| R-3 | compiling a service out breaks an always-on consumer that imported it for a type/helper, not lifecycle | build break in a minimal binary | the audit rule catches direct imports BEFORE the tag is added; convert those to registry/iface or move the shared type to core |
| R-4 | dead-code elimination does not actually shrink the binary if a transitive dep is still referenced | binary size unchanged with tag off | measure binary size in the build-tag test; assert the symbol is absent |
| R-5 | protocol compile-out needs B-2 (codec/engine un-fusing) first | protocol tag pulls in unintended engine code | sequence protocol child specs AFTER tiers B-2; pilot only services first |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build` with `ze_ssh` ON | → | ssh registers a factory; hub starts it | `TestBuildTag_SSH_Present` (cmd/ze) |
| `go build` with `ze_ssh` OFF (default) | → | ssh package not linked; no ssh start | `TestBuildTag_SSH_Absent` (cmd/ze) |
| construction registry has a registered ssh factory | → | hub builds ssh via the registry, not `ssh.NewServer` | `TestServiceRegistry_BuildsSSH` |
| `dep_audit.py` over a tree with a direct import of a disableable feature | → | audit reports the violation | `dep_audit` selftest `TestNoDirectImportOfDisableable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A construction registry exists; services register a factory via `init()` | hub builds/starts services by iterating the registry, with no direct `New*` import of a registered service |
| AC-2 | `ze_ssh` build tag ON | ssh server compiled in, registered, started; existing ssh tests pass |
| AC-3 | `ze_ssh` build tag OFF (default minimal) | the `internal/component/ssh` package is NOT linked (symbol absent); daemon starts without ssh; no error |
| AC-4 | a no-ssh binary is fed config containing `ssh { ... }` | config handling is safe (documented behavior: ignored-with-warning or clean validation error), no panic |
| AC-5 | `knownListenerServices` / listener defaults in a no-ssh build | parse-time conflict detection for OTHER services still works; no panic on the absent ssh entry |
| AC-6 | the generator runs | it emits the composition root with `ze_<feature>`-gated blank-import groups; `plugin_imports.go --check` passes |
| AC-7 | a disableable feature is directly imported by always-on code | `dep_audit.py` reports it (advisory in this umbrella; gated once the category is enumerated) |
| AC-8 | child specs add web/gnmi/mcp/lg/protocol tags | each independently compile-out-able with its own `ze_<feature>` tag + present/absent test |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without ssh (`go build` minimal) | tag off → ssh blank import dropped → package unlinked → no ssh listener | `TestBuildTag_SSH_Absent` + binary symbol check |
| 2 | builds a full `ze` with ssh | tag on → factory registered → hub builds ssh subsystem → ssh listens | `TestBuildTag_SSH_Present` + existing ssh functional test |
| 3 | runs a no-ssh binary against a config with `ssh {}` | config load → ssh schema absent → safe handling | `test/parse/ssh-absent-config.ci` (child 1) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestServiceRegistry_BuildsSSH` | `cmd/ze/hub/service_registry_test.go` | hub builds ssh via registry, not direct ctor | |
| `TestServiceRegistry_AbsentFeatureNoOp` | same | a feature with no registered factory is skipped cleanly | |
| `TestNoDirectImportOfDisableable` | `scripts/dev/dep_audit.py --selftest` | audit flags a direct import of a disableable feature | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_ssh` | `cmd/ze/build_tag_ssh_test.go` | ssh present with `ze_ssh`, absent without | |
| `ssh-absent-config` | `test/parse/ssh-absent-config.ci` | no-ssh binary handles `ssh {}` config safely | |

### Interop Tests (MANDATORY for protocol features)
- N/A for the umbrella + ssh pilot (no wire-protocol change). Protocol-compile-out
  child specs re-run that protocol's EXISTING interop suite to prove the on-build
  behavior is unchanged.

### Future (if deferring any tests)
- web/gnmi/mcp/lg/monitoring and protocol compile-out: each its own child spec with
  its own present/absent build-tag test. Deferred by the umbrella structure (user-approved).

## Files to Modify
- `scripts/codegen/plugin_imports.go` - emit `ze_<feature>`-gated blank-import groups
- `cmd/ze/hub/infra_setup.go` - build ssh via the construction registry (pilot)
- `cmd/ze/hub/main.go` / `main_servers.go` - iterate the registry to start services
- `scripts/dev/dep_audit.py` - add the no-direct-import audit rule + selftest fixture
- `ai/rules/module-tiers.md` - document the disable-ability rule
- `internal/component/config/listener.go` - tolerate absent services (if needed)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | - (build-time, not CLI) |
| Functional test | [ ] yes | `cmd/ze/build_tag_ssh_test.go`, `test/parse/ssh-absent-config.ci` |
| Doctor check | [ ] no | services already own their doctor checks; absence = no check registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register the new audit rule + build tags |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavors) | `docs/features.md` (build-tag table) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md`, `docs/architecture/core-design.md` |
| 15 | Runtime inventory changed? | [ ] location only | `docs/plugin-overview.md` if a service moves |
| others | - | [ ] assess per child spec | grep docs for the affected service |

## Files to Create
- `cmd/ze/hub/service_registry.go` - the construction registry + factory iface (pilot)
- `cmd/ze/hub/service_registry_test.go`
- `cmd/ze/build_tag_ssh_test.go`
- `test/parse/ssh-absent-config.ci`
- child specs (real numbering; pilot pivoted to lg, ssh became child 2):
  - `spec-feature-gate-1-lg.md` -- DONE (learned 980), `spec-feature-gate-2-ssh.md` -- DONE (learned 981)
  - `spec-feature-gate-3-web.md` -- ready (registry; Phase 1 extracts cert/TLS to internal/core/selfcert)
  - `spec-feature-gate-4-gnmi.md` -- DONE (learned 986; dedicated seam, three owned blank imports + reload-notify coupling)
  - `spec-feature-gate-5-mcp.md` -- ready (registry; MCPServerHandle already Reconfigurable)
  - `spec-feature-gate-6-api.md` -- ready (seam; rest+grpc combined ze_api; parent api stays always-on)
  - `spec-feature-gate-7-monitoring.md` -- ready (gate metrics exporter only; registry stays always-on; recommend last)
  - `spec-feature-gate-8-protocols.md` -- ready (per-protocol; B-2 dependency CONDITIONAL on A-1 codec-consumer grep)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior |
| 3. Wiring | Wiring Test - registry + build-tag tests first |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase: construction registry (pilot infra)** - define the factory interface +
   registry; hub iterates it. No service moved yet; existing direct ctors still run.
   - Tests: `TestServiceRegistry_AbsentFeatureNoOp`
   - Verify: registry exists, hub builds nothing new yet, all tests green.
2. **Phase: ssh through the registry** - ssh registers a factory under `//go:build ze_ssh`;
   hub builds ssh via the registry; remove the direct `zessh.NewServer` import from the hub.
   - Tests: `TestServiceRegistry_BuildsSSH`, existing ssh tests
   - Verify: `ze_ssh` build identical behavior; default build omits ssh.
3. **Phase: build-tag gating + tests** - generator emits the `ze_ssh` group; add
   `build_tag_ssh_test.go` (present/absent) + `ssh-absent-config.ci`.
   - Verify: minimal build has no ssh symbol; full build does; config safe.
4. **Phase: audit rule** - `dep_audit.py` flags a direct import of a disableable feature;
   selftest fixture; document the rule in `module-tiers.md`.
   - Verify: selftest catches a planted direct import.
5. **Phase: child specs** - web, gnmi, mcp, lg, monitoring, protocols (each its own spec).
6. **Full verification** - `make ze-verify`; build + measure each flavor.

### Failure Routing
| Failure | Route To |
|---------|----------|
| ssh not omitted with tag off | a residual direct import remains - find via the audit rule |
| generator `--check` fails | the gated-group emission - fix `plugin_imports.go` |
| config panics in no-ssh build | schema/listener absence handling - A-3/A-4 |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- Compile-out-ability is the concrete payoff of the module-tier work: a feature can be
  removed from the binary exactly when nothing always-compiled imports it (axis B).
- Runtime disable (554 `enabled` leaf) and compile-time disable (this umbrella) are
  orthogonal: the former is an operational off-switch, the latter shrinks the binary
  and attack surface.

## Core Insight
A direct functional `import` pins a package into the binary; only blank
registration imports are build-tag-removable. So "make X disableable" always reduces
to "sever every direct import of X from always-on code, then gate its registration."

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Construction registry (factory `init()` registration) | per-service stub files under `!ze_<feature>` | stubs duplicate each service's API surface; a registry scales and matches the small-core "register, don't import" pattern |
| Per-feature `ze_<feature>` tags | coarse groups (853 model) | user requirement: independent compile-out per feature. Cost (test matrix) tracked as R-1 |
| ssh as the pilot | start with web | ssh has one ctor + one always-on importer; web is ~30 calls - prove the pattern cheaply first |
| Reuse `ze.Subsystem` | new service lifecycle | lifecycle already exists (452); only construction needs indirection |

## Known Limitations
- The umbrella delivers the registry + ssh pilot + audit rule + generator gating.
  web/gnmi/mcp/lg/monitoring and protocol compile-out are child specs.
- Protocol compile-out depends on tiers B-2 (codec/engine un-fusing).
- Per-feature granularity is a deliberate cost (build-tag test matrix, R-1).

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ssh compile-out-able via `ze_ssh` | build-tag test + binary symbol check | `TestBuildTag_SSH_Absent` passes; `go tool nm` shows no ssh symbols in minimal build |
| daemon builds services via registry | unit test | `TestServiceRegistry_BuildsSSH` |
| disableable features have no direct always-on import | audit | `dep_audit.py` selftest + clean run over the ssh-converted tree |

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated (umbrella: AC-1..AC-7 via ssh pilot; AC-8 per child spec)
- [ ] construction registry built; hub iterates it
- [ ] ssh compile-out-able; present/absent build-tag tests pass
- [ ] audit rule flags direct imports of disableable features
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe)
- [ ] Goal Validation table filled with concrete evidence
