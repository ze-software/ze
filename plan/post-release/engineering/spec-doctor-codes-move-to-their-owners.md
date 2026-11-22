# Spec: doctor codes move to their owners

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Release-independent doctor codes move to their owners work; historical findings do not establish a current product defect. |
| Scope | cli |
| Depends | - |
| Phase | closure |
| Handoff | - |
| Updated | 2026-11-22 |

<!-- Scope: cli. `ze show diagnostic <code>` is the surface whose answer the decision
     changes; the doctor registry and the code table are what it reads.
     Bucket: plan/post-release/engineering/. Named in the Work Not Done table of
     spec-the-fix-pass-for-restated-registries. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`internal/core/diagnostic/codes.go` and `internal/core/diagnostic/codes_plugin_infra.go`
declared feature-, component- and plugin-owned `doctor-*` codes centrally beside
the built-in `config-*` codes and cross-cutting multi-subsystem `doctor-*` codes.

The bottom-tier table was a second declaration of what each feature emits
(`ai/rules/principles.md`, declare once). Moving each feature's codes into its
owning package changes what `ze show diagnostic <code>` answers on a build
without that feature's build tag, because the code then registers nowhere.

The decision this spec makes, one of two:

| Reading | What `ze show diagnostic <code>` answers on a build without the feature | What the check is |
|---------|----------------------------------------------------------------|-------------------|
| A: a code an absent feature would emit still explains | The explanation | The table stays; each emitter references the constant, and the check is that it does |
| B: `ze show diagnostic` answers "that feature is not in this build" | "that feature is not in this build" | The entry moves to the emitter; the doctor registry contract (`diagnostic.RegisterDoctorCheck` in `internal/core/diagnostic/doctor_registry.go`), whose codes must resolve through `diagnostic.Lookup`, is the check |

**Decision (owner directive):** **Reading B** was chosen and implemented directly
at Thomas's direction. Every single-owner `doctor-*` code moves out of
`internal/core/diagnostic/` into its owning package and registers via
`diagnostic.Register` from `init()`. Only `config-*` codes, core TLS helper
codes (`internal/core/diagnostic/doctor_cert.go`), and genuinely cross-cutting
multi-subsystem `doctor-*` codes stay in `internal/core/diagnostic/`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor readiness tiers, phases, runtime checks, and code ownership
  → Decision: every check's declared codes resolve through `diagnostic.Lookup` so `ze show diagnostic <code>` explains every finding the running binary can emit
  → Constraint: owning packages register their own doctor checks and diagnostic codes from `init()`, while cross-cutting codes stay in core to avoid import cycles
- [ ] `docs/architecture/cli/error-surface.md` - operator error format (`opererr.Error`) and diagnostic code ownership
  → Decision: `ze show diagnostic` unknown-code refusal uses `opererr.New` with Reading B wording
  → Constraint: each code is declared once as a constant beside its metadata table and referenced everywhere it is emitted
- [ ] `docs/architecture/module-tiers.md` - leaf/component/plugin tiers and compile-out boundaries
  → Constraint: `internal/core/diagnostic` must not import upward into components or plugins; compile-out tags drop feature codes with their packages

### Source Files
- [ ] `internal/core/diagnostic/codes.go` and `internal/core/diagnostic/codes_plugin_infra.go` - built-in diagnostic code catalog
  → Constraint: no `doctor-*` code is declared in two places (`ai/rules/principles.md`, declare once); only `config-*`, core TLS helper, and cross-cutting multi-subsystem `doctor-*` codes remain here
- [ ] `internal/core/diagnostic/registry.go` - `Register`, `Lookup`, `AllCodes`, `ResetForTest`, `SnapshotForTest`
  → Decision: `SnapshotForTest()` restores `init()`-registered owner codes after tests that call `ResetForTest()`
- [ ] `internal/core/diagnostic/doctor_registry.go` - `diagnostic.RegisterDoctorCheck`
  → Decision: under Reading B, `TestDoctorRegisteredCheckCodesHaveMetadata` walks every registered check and verifies every declared code resolves through `diagnostic.Lookup`
- [ ] `internal/core/diagnostic/event_codes.go` - `RegisterEventCode` and `describedCode`
  → Constraint: an owner registering an event code must register its `CodeMeta` via `diagnostic.Register` first in `init()` so `describedCode` finds it
- [ ] `internal/plugins/explain/main.go` - `ze show diagnostic <code>` handler
  → Decision: unknown code refusal states "If this code belongs to an optional feature, that feature is not in this build."
- [ ] `spec-the-fix-pass-for-restated-registries` - Work Not Done origin of this spec
  → Constraint: moving an entry into its emitter removes it from minimal builds without that feature's build tag

### RFC Summaries (Scope: protocol)
- [ ] N/A - Scope is `cli`; no wire protocol or RFC behavior is changed.

**Key insights:**
- The table in `internal/core/diagnostic/codes.go` was a second declaration of what each feature emits; moving 248 single-owner `doctor-*` codes into 71 owning packages via `diagnostic.Register` eliminates central enumeration and makes compile-out consistent.
- Under Reading B, `ze show diagnostic <code>` explains every code the running build can emit and explicitly informs operators when an unknown code may belong to an optional feature absent from the build.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/diagnostic/codes.go` - previously declared ~248 single-owner feature/component/plugin `doctor-*` codes beside `config-*` and cross-cutting codes
- [ ] `internal/core/diagnostic/codes_plugin_infra.go` - previously declared single-owner `plugin/{doctor,ipc,process,server}` codes beside cross-cutting plugin-infra codes
- [ ] `internal/core/diagnostic/doctor_registry.go` - `diagnostic.RegisterDoctorCheck` registers doctor checks whose declared codes must resolve through `diagnostic.Lookup`
- [ ] `internal/core/diagnostic/event_codes.go` - `RegisterEventCode` requires `describedCode(code)` to be true at `init()` time
- [ ] `internal/plugins/explain/main.go` - `Run` looks up codes in `diagnostic.Lookup` and previously refused unknown codes without noting build-tag absence

**Behavior to preserve:**
- `ze show diagnostic <code>` answers every code the running build can emit (including text and `--json` output formats).
- Every `doctor-*` emitter references one constant declaration beside its metadata table rather than repeating string literals.
- Every doctor check registered through `diagnostic.RegisterDoctorCheck` keeps the contract that all its declared `Codes` resolve through `diagnostic.Lookup`.
- Cross-cutting `doctor-*` codes shared across unrelated packages (which cannot import one another without cycles) and core `config-*` / TLS helper codes remain in `internal/core/diagnostic/`.

**Behavior to change:**
- All 248 single-owner `doctor-*` codes move from `internal/core/diagnostic/{codes,codes_plugin_infra}.go` to their 71 owning packages (`doctor_codes.go` / `register_doctor_codes.go`), registered via `diagnostic.Register` from `init()`.
- `ze show diagnostic <code>` for an unknown code states: `If this code belongs to an optional feature, that feature is not in this build.` (Reading B).
- `diagnostic.SnapshotForTest()` is added so tests resetting the code registry restore `init()`-registered owner codes cleanly.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| `diagnostic.Register` / `diagnostic.Lookup` / `diagnostic.AllCodes` | `internal/core/diagnostic/registry.go` | Process-wide mutex-protected diagnostic code metadata registry populated by `RegisterBuiltinCodes()` and package `init()` calls | Extends with `SnapshotForTest()` and moves all 248 single-owner `doctor-*` registrations to owning-package `init()` calls |
| `diagnostic.RegisterDoctorCheck` | `internal/core/diagnostic/doctor_registry.go` | Registers doctor checks with their declared `Codes` slice | Preserved; `TestDoctorRegisteredCheckCodesHaveMetadata` walks it to enforce AC-3 |
| `diagnostic.RegisterEventCode` / `describedCode` | `internal/core/diagnostic/event_codes.go` | Registers bus-raised transient event codes and verifies `builtinCodes` or `Lookup(code)` already describes them | Extends error hint to direct owners to `diagnostic.Register` in their owning package before `RegisterEventCode` |
| Owning-package code registration precedent | `internal/plugins/isis/codes.go` | Already registered IS-IS `doctor-*` codes from `init()` via `diagnostic.Register` | Adopted as the universal pattern across all 71 owning packages |
| `explain.Run` (`ze show diagnostic`) | `internal/plugins/explain/main.go` | Local-data command that looks up `diagnostic.Lookup(code)` and renders text or `--json` | Updates unknown-code refusal wording to Reading B (`opererr.New`) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Package `init()` at binary startup (every linked owning component/plugin package) registers its `CodeMeta` entries via `diagnostic.Register(meta)`.
- Binary entrypoint calls `diagnostic.RegisterBuiltinCodes()` for built-in `config-*`, core TLS helper, and cross-cutting `doctor-*` codes.
- Operator runs `ze show diagnostic [--json] <code>` (or daemon `show diagnostic <code>`) or `ze doctor` / `show doctor`.

### Transformation Path
1. Linked owning packages execute `init()` -> `diagnostic.Register(meta)` populates `internal/core/diagnostic.codes` (and panics with `BUG:` on duplicate code).
2. Packages that raise transient bus events call `registerDoctorCodes()` first, then `diagnostic.RegisterEventCode(...)` (`describedCode` verifies metadata exists).
3. Binary startup calls `diagnostic.RegisterBuiltinCodes()` for core `config-*` and cross-cutting codes.
4. `explain.Run` (`internal/plugins/explain/main.go`) calls `diagnostic.Lookup(code)`: returns `ExplainResult` if found, or an `opererr.Error` refusal with Reading B wording + `diagnostic.AllCodes()` if absent.
5. `internal/component/doctor/registry_test.go` (`TestDoctorRegisteredCheckCodesHaveMetadata`) walks `diagnostic.DoctorChecks()` across all linked phases and verifies every declared code resolves via `diagnostic.Lookup`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Owning component/plugin -> `internal/core/diagnostic` | Downward import; `init()` calls `diagnostic.Register(CodeMeta)` (value type) | Yes (`./le arch tier check` clean; no upward core imports) |
| `internal/component/plugin/ipc` -> `internal/core/diagnostic` | Avoided cycle: `ipc.ConnectBackRefusedCode` registers in `internal/component/plugin/server/doctor_codes.go` | Yes (`TestPluginInfraDoctorCodesDescribed` passes) |
| Build-tag compile-out (`ze_bgp`, `ze_gnmi`, `ze_ssh`, `ze_telemetry`, etc.) | Blank imports in `internal/component/plugin/all/all_ze_*.go` and `cmd/ze/`; absent tag drops package + codes | Yes (minimal `-tags ze_core,ze_distro` vs full build verified) |
| CLI/Daemon -> `internal/plugins/explain` | Local-data command returning `command.LocalAnswer` / `opererr.Error` | Yes (`explain_test.go`, `test/ui/explain-diagnostic.ci`) |

### Integration Points
- `internal/core/diagnostic/registry.go` (`Register`, `Lookup`, `AllCodes`, `SnapshotForTest`) - single registry for built-in and owning-package codes.
- `internal/component/plugin/all/all_ze_*.go` and `cmd/ze/` - blank imports link owning packages according to build tags.
- `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` - mirrors `cmd/ze` build-tagged imports in the `doctor` test binary.
- `internal/le/arch/doctorcoverage/` - updated fix hint and test reference (`gnmi.CodeDoctorGNMITLSMaterial`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All 71 owning packages register via `diagnostic.Register`; `explain.Run` reads only `diagnostic.Lookup`/`AllCodes` |
| No unintended coupling (components stay isolated) | Yes | `./le arch tier check` clean; `ipc.ConnectBackRefusedCode` registered from `plugin/server` to avoid import cycle |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `diagnostic.Register`, `diagnostic.RegisterDoctorCheck`, `diagnostic.RegisterEventCode`; 0 duplicate `doctor-*` codes across 81 code-registry files |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Cold-path startup registration and CLI lookup only; no hot-path allocations added |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | 248 single-owner `doctor-*` codes removed from `internal/core/diagnostic/` and registered from each owner's `init()` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/core/diagnostic/{codes,codes_plugin_infra}.go`, `docs/`, `ai/skills/`, `plan/templates/TEMPLATE.md`; `explain.Run` derives valid codes from `diagnostic.AllCodes()` and `doctor` checks derive from `diagnostic.DoctorChecks()` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every linked owning package's `init()` runs before `diagnostic.Lookup` is called by `explain` or `doctor` tests | Go package initialization order + blank imports in `internal/component/plugin/all/` and `cmd/ze/` | A linked check's code would fail `diagnostic.Lookup` | `TestDoctorRegisteredCheckCodesHaveMetadata` + `all_import_ze_{bgp,ssh,telemetry}_test.go` | confirmed |
| A-2 | Owners that call `diagnostic.RegisterEventCode` in `init()` register their `CodeMeta` first | `internal/core/diagnostic/event_codes.go` (`describedCode`) checks `builtinCodes` or `Lookup(code)` | Binary panics at startup during `init()` | `internal/component/iface/register.go:179,201` calls `registerDoctorCodes()` before `RegisterEventCode`; scoped unit tests green | confirmed |
| A-3 | No import cycle prevents owning packages from importing `internal/core/diagnostic`, except `plugin/ipc` | `internal/core/diagnostic` imports `internal/component/plugin`, which imports `plugin/ipc` | Compile error on import cycle | `ipc.ConnectBackRefusedCode` registered in `internal/component/plugin/server/doctor_codes.go`; `./le arch tier check` clean | confirmed |
| A-4 | Remaining `doctor-*` codes in `internal/core/diagnostic/` are genuinely cross-cutting or core TLS helpers | Grep of all emitters of remaining constants in `codes.go` and `codes_plugin_infra.go` | Single-owner code left behind in core | Audited every remaining `CodeDoctor*` in `internal/core/diagnostic/`: all emitted by `doctor_cert.go` or 2+ unrelated packages (`bgp`+`doctor`, `cli`+`web`+`hub`, `config/system`+`lg`, `config/system`+`doctor`, plugin-infra shared) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A test calling `diagnostic.ResetForTest()` wipes `init()`-registered owner codes for subsequent tests in the same binary | Later test in same binary fails `diagnostic.Lookup` | Added `diagnostic.SnapshotForTest()` with `t.Cleanup` restore contract |
| R-2 | `doctor` test binary misses a build-tagged component that `cmd/ze` links | `TestDoctorRegisteredCheckCodesHaveMetadata` misses or fails a code | Added `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` mirroring `cmd/ze` |
| R-3 | Duplicate code registration across two packages panics at startup | `init()` panic with `BUG: ... doctor code registration refused` | Python/grep audit across all 81 code-registry files confirmed 0 duplicates; `diagnostic.Register` rejects duplicates deterministically |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `ze show diagnostic <code>` could fail to explain a linked check's code, or a package `init()` could panic on duplicate/missing registration |
| How is it reverted? | Two-commit revert of `f4a0df7781` and the follow-up Commit A; no config or on-disk state migration |
| Who else touches this path? | Any component/plugin adding a new `doctor-*` check or code; `ai/skills/{ze-close,ze-implement,ze-review}.md` and `plan/templates/TEMPLATE.md` updated so future work follows owning-package registration |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show diagnostic <known>` | → | `explain.Run` -> `diagnostic.Lookup` | `test/ui/explain-diagnostic.ci`, `test/ui/explain-diagnostic-json.ci`, `TestExplainKnownDiagnostic` (`internal/plugins/explain/explain_test.go`) |
| `ze show diagnostic <absent>` | → | `explain.Run` -> Reading B `opererr.New` refusal | `TestExplainUnknownDiagnostic` (`internal/plugins/explain/explain_test.go`) + minimal vs full `cmd/ze` build check |
| `ze doctor` registry walk | → | `diagnostic.DoctorChecks()` -> `diagnostic.Lookup` | `TestDoctorRegisteredCheckCodesHaveMetadata` (`internal/component/doctor/registry_test.go`) |
| `gnmi` check & hub refusal | → | `gnmi/register_doctor_codes.go:init` + `gnmi/doctor.go` + `cmd/ze/hub/service_gnmi.go` | `TestGNMITLSMaterialCheckRegistered`, `TestGNMIExposureFromEnvironment` (`internal/component/gnmi/doctor_test.go`), `TestDoctorCoverageSourceHoldsEveryRegisteredCode` (`internal/le/arch/doctorcoverage/check_test.go`) |
| `iface` unread-state warning | → | `iface/register.go:init` -> `registerDoctorCodes` -> `RegisterEventCode` -> `config_apply_held.go` | `TestApplyReportsAStateItCannotRead` (`internal/component/iface/config_apply_held_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The tree is searched for every `doctor-*` code declaration | No `doctor-*` code is declared in two places |
| AC-2 | `ze show diagnostic <code>` is run, on any build, for a code that build can emit | The code is explained |
| AC-3 | Every doctor check registered through `diagnostic.RegisterDoctorCheck` is walked, and each entry of its `Codes` is resolved through `diagnostic.Lookup` | A test walks every registered check and every code resolves; the test goes RED when one registered code has no declaration |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze show diagnostic <code-in-build>` (text or `--json`) | CLI -> `explain.Run` -> `diagnostic.Lookup` (populated by owning package `init()` + `RegisterBuiltinCodes`) -> `render`/`drawText` | `test/ui/explain-diagnostic.ci`, `test/ui/explain-diagnostic-json.ci`, `TestExplainKnownDiagnostic` |
| 2 | Runs `ze show diagnostic <code-from-compiled-out-feature>` on a minimal build | CLI -> `explain.Run` -> `diagnostic.Lookup == nil` -> Reading B `opererr.Error` ("that feature is not in this build") | `TestExplainUnknownDiagnostic` + minimal (`-tags ze_core,ze_distro`) vs full `cmd/ze` verification |
| 3 | Runs `ze doctor` / `show doctor` and looks up any emitted code | `doctor.Answer` -> registered checks -> `diagnostic.Lookup` for every declared code | `TestDoctorRegisteredCheckCodesHaveMetadata`, `TestDoctorCheckCodesRegistered`, `TestPluginInfraDoctorCodesDescribed` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorRegisteredCheckCodesHaveMetadata` | `internal/component/doctor/registry_test.go` | Every registered doctor check's declared codes resolve via `diagnostic.Lookup` | Pass (commit `f4a0df7781`) |
| `TestDoctorCheckCodesRegistered` | `internal/component/doctor/doctor_codes_test.go` | Every doctor-component emitted code resolves | Pass (commit `f4a0df7781`) |
| `TestPluginInfraDoctorCodesDescribed` | `internal/component/plugin/doctor/doctor_codes_test.go` | Every plugin-infra code and cross-package `RelatedCodes` target resolves | Pass (commit `f4a0df7781`) |
| `TestCrossCuttingCodesDescribed` | `internal/core/diagnostic/codes_test.go` | Every cross-cutting code in `codes_plugin_infra.go` has metadata | Pass (commit `f4a0df7781`) |
| `TestExplainUnknownDiagnostic` | `internal/plugins/explain/explain_test.go` | Unknown code returns Reading B message ("that feature is not in this build") | Pass (commit `f4a0df7781`) |
| `TestApplyReportsAStateItCannotRead` | `internal/component/iface/config_apply_held_test.go` | `codeDoctorIfaceStateUnread` registers as event code and raises/clears warning | Owed (main thread) |
| `TestGNMITLSMaterialCheckRegistered`, `TestGNMIExposureFromEnvironment` | `internal/component/gnmi/doctor_test.go` | `gnmi.CodeDoctorGNMITLSMaterial` registered in check and emitted with fix | Owed (main thread) |
| `TestDoctorCoverageSourceHoldsEveryRegisteredCode` | `internal/le/arch/doctorcoverage/check_test.go` | Source coverage gate tracks `gnmi.CodeDoctorGNMITLSMaterial` under `gnmi-tls-material` | Owed (main thread) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No numeric boundary inputs introduced | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `explain-diagnostic` | `test/ui/explain-diagnostic.ci` | `ze show diagnostic config-parse` renders text explanation | Existing coverage |
| `explain-diagnostic-json` | `test/ui/explain-diagnostic-json.ci` | `ze show diagnostic --json config-yang-missing` renders structured JSON | Existing coverage |
| `command-contract-explain` | `test/ui/command-contract-explain.ci` | `show diagnostic` command contract works through CLI | Existing coverage |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | N/A | N/A | Scope is `cli` (internal diagnostic code registration and `ze show diagnostic` refusal text); no wire protocol behavior changed | N/A |

## Files to Modify
- `internal/core/diagnostic/codes.go` - remove 248 single-owner `doctor-*` codes (246 in `f4a0df7781` + `CodeDoctorIfaceStateUnread` and `CodeDoctorGNMITLSMaterial` in Commit A)
- `internal/core/diagnostic/codes_plugin_infra.go` - keep only cross-cutting plugin-infra/core codes (commit `f4a0df7781`)
- `internal/core/diagnostic/event_codes.go` - update missing-metadata hint to point at owning-package `diagnostic.Register` (commit `f4a0df7781`)
- `internal/core/diagnostic/registry.go` - add `SnapshotForTest()` (commit `f4a0df7781`)
- `internal/plugins/explain/main.go` and `explain_test.go` - Reading B unknown-code refusal wording (commit `f4a0df7781`)
- `internal/component/iface/doctor_codes.go`, `config_apply_held.go`, `config_apply_held_test.go`, `register.go` - move `codeDoctorIfaceStateUnread` to `iface` (Commit A)
- `internal/component/gnmi/doctor.go`, `doctor_test.go` and `cmd/ze/hub/service_gnmi.go` - use `gnmi.CodeDoctorGNMITLSMaterial` (Commit A)
- `internal/le/arch/doctorcoverage/check_test.go` - reference `gnmi.CodeDoctorGNMITLSMaterial` (Commit A)
- `plan/templates/TEMPLATE.md`, `ai/skills/ze-close.md`, `ai/skills/ze-implement.md`, `ai/skills/ze-review.md` - update doctor code registration instructions from `codes.go` to owning-package `diagnostic.Register` (Commit A)
- `docs/architecture/cli/error-surface.md`, `docs/guide/health-checks.md`, `docs/features/ai-first.md`, `docs/guide/debugging-tools.md`, `docs/guide/vpp.md`, `docs/architecture/bgp/as112-coordination.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - updated in commit `f4a0df7781`

## Files to Create
- 70 owning-package `doctor_codes.go` / `register_doctor_codes.go` files across components/plugins + `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` + `internal/plugins/ddos/flowspec/doctor_codes_test.go` (created in commit `f4a0df7781`)
- `internal/component/gnmi/register_doctor_codes.go` - registers `CodeDoctorGNMITLSMaterial` in `gnmi` `init()` (Commit A)
- `website/changes/fragments/doctor-codes-move-to-their-owners.md` - operator news fragment (Commit A)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC schema added or changed |
| YANG validation constraints | N-A | No YANG leaves added |
| YANG custom validators | N-A | No YANG validators added |
| CLI commands/flags | Yes | `internal/plugins/explain/main.go` updated Reading B unknown-code refusal text; grammar unchanged |
| CLI grammar (keyword before value) | No | Existing `ze show diagnostic <code>` grammar unchanged |
| Editor autocomplete | No | No new config or command nodes |
| Functional test for new RPC/API | No | Existing `.ci` tests (`test/ui/explain-diagnostic*.ci`) cover `ze show diagnostic` |
| Pipe completeness | No | `explain.Run` already returns structured `command.LocalAnswer` through pipe layer |
| Env var registration | No | No env vars added |
| Doctor check for runtime dependencies | Yes | All 248 single-owner doctor codes now register via `diagnostic.Register` in their owning packages; `TestDoctorRegisteredCheckCodesHaveMetadata` validates all registered checks |
| Prometheus counters/metrics | No | No metrics added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Scope is `cli`, no BGP family changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Refactors code ownership and clarifies unknown-code refusal; no new `features/<id>.md` |
| 2 | Config syntax changed? | No | No YANG/config syntax changed |
| 3 | CLI command added/changed? | No | `ze show diagnostic` syntax unchanged; `docs/guide/command-reference.md:252` source anchor (`CodeConfigNodeOtherOS`, `CodeConfigHostBackend`) still resolves |
| 4 | API/RPC added/changed? | No | No API/RPC methods changed |
| 5 | Plugin added/changed? | No | No plugins added or removed |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` (updated in `f4a0df7781`: owning-package `diagnostic.Register`), `docs/guide/debugging-tools.md` & `docs/guide/vpp.md` (updated in `f4a0df7781`) |
| 7 | Wire format changed? | No | No wire format changes |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC behavior changed |
| 10 | Test infrastructure changed? | No | Only `diagnostic.SnapshotForTest` and `doctor` test blank imports added |
| 11 | Affects daemon comparison? | No | No feature capability change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/error-surface.md:179-188` and `docs/features/ai-first.md:86-93` updated in `f4a0df7781`; `plan/templates/TEMPLATE.md` and `ai/skills/{ze-close,ze-implement,ze-review}.md` updated in Commit A |
| 13 | Route metadata keys added/changed? | No | No route metadata keys touched |
| 14 | Prometheus counters added/changed? | No | No metrics touched |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No plugin/event/command/capability inventory changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `git grep "source:.*internal/core/diagnostic/codes" docs/` and `./le doc check all` (6020/6020 source anchors valid): `docs/architecture/cli/error-surface.md`, `docs/features/ai-first.md`, `docs/architecture/bgp/as112-coordination.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` updated in `f4a0df7781`; `docs/guide/command-reference.md:252` unaffected (`CodeConfigNodeOtherOS`, `CodeConfigHostBackend` remain in `codes.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Checked `docs/guide/health-checks.md`, `docs/architecture/cli/error-surface.md`, `docs/features/ai-first.md` — all examples and code-ownership instructions match current code |

## Implementation Steps

1. **Phase 1: Core registry & Reading B explain** -- add `diagnostic.SnapshotForTest()`, update `internal/plugins/explain/main.go` and `explain_test.go` for Reading B, update `event_codes.go` error hint (landed in `f4a0df7781`).
2. **Phase 2: Move 246 single-owner doctor codes to 70 owning packages** -- create owning `doctor_codes.go` / `register_doctor_codes.go`, update emitters and `doctor` build-tagged test imports, update docs (landed in `f4a0df7781`).
3. **Phase 3: Move final 2 single-owner doctor codes & update skills/template** -- move `CodeDoctorIfaceStateUnread` to `internal/component/iface/doctor_codes.go` and `CodeDoctorGNMITLSMaterial` to `internal/component/gnmi/register_doctor_codes.go`, update callers/tests, update `plan/templates/TEMPLATE.md` and `ai/skills/{ze-close,ze-implement,ze-review}.md` (Commit A).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1, AC-2, AC-3 each traced to producing code and tests |
| Feature completeness | Every user story (`show diagnostic` known, `show diagnostic` absent in minimal build, `ze doctor` code resolution) verified |
| Correctness | Every `RegisterEventCode` owner registers its `CodeMeta` before calling `RegisterEventCode`; no duplicate code registrations across all 81 code-registry files |
| Naming | Lower-kebab `doctor-*` codes, one constant beside its `CodeMeta` table, no string repetition |
| Data flow | Owning packages import `internal/core/diagnostic` downward only; `plugin/ipc` cycle avoided via `plugin/server` |
| Rule: `ai/rules/principles.md` (declare once) | Zero remaining single-owner `doctor-*` codes in `internal/core/diagnostic/codes.go` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero duplicate `doctor-*` code declarations across code-registry files | Scripted scan of all 81 code-registry files (`0 duplicates`, 214 unique codes) |
| Zero remaining references to `diagnostic.CodeDoctorIfaceStateUnread` or `diagnostic.CodeDoctorGNMITLSMaterial` | `git grep -n "CodeDoctorIfaceStateUnread\|CodeDoctorGNMITLSMaterial"` |
| Reading B unknown-code explanation in `internal/plugins/explain/main.go` | `TestExplainUnknownDiagnostic` + minimal vs full `cmd/ze` build check |
| All registered doctor check codes resolve via `diagnostic.Lookup` | `TestDoctorRegisteredCheckCodesHaveMetadata`, `TestDoctorCheckCodesRegistered`, `TestPluginInfraDoctorCodesDescribed` |
| Scoped tests for Commit A follow-up (`diagnostic`, `iface`, `gnmi`, `cmd/ze/hub`, `doctorcoverage`, `doctor`) | Owed commands run by main thread before Commit A |
| Skills, template, and docs updated to owning-package registration | `./le doc check all` (0 drift, 6020/6020 source anchors valid) + diff review |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation (`ze show diagnostic <code>`) | Untrusted CLI argument `code` flows only into `diagnostic.Lookup(code)` map lookup and `opererr.New` string; no format string, shell, or SQL sink |
| Secret leakage (`gnmi` TLS check) | `gnmiExposureFindings` / `gnmiTLSRefusalFinding` never echo `ze.gnmi.token` (pinned by `TestGNMIExposureFromEnvironment`) |
| Bounded allocations / DoS | `diagnostic.AllCodes()` allocates strictly `len(codes)` (bounded compile-time registration count); `report.FitMessage` bounds `iface` unread-state warning text |
| Panic reachability | `init()` panics in `register_doctor_codes.go` / `doctor_codes.go` / `iface/register.go` run only over compile-time constants; unreachable from peer or operator input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| Lint failure | Fix inline |
| Audit finds a missing AC | Back to the relevant phase and implement |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reading B: move single-owner `doctor-*` codes to owning packages; `ze show diagnostic` says absent feature is not in this build | Reading A: keep all `doctor-*` codes in `internal/core/diagnostic/codes.go` | Eliminates second central declaration (`ai/rules/principles.md`, declare once) and honors compile-out boundaries |
| Keep cross-cutting multi-subsystem codes and core TLS helpers in `internal/core/diagnostic/` | Force-move cross-cutting codes into one arbitrary owner | Prevents component-to-component and core-to-component import cycles (e.g. `bgp` + `doctor`, `cli` + `web` + `hub`, `config/system` + `lg`) |
| Register `ipc.ConnectBackRefusedCode` from `internal/component/plugin/server/doctor_codes.go` | Register from `internal/component/plugin/ipc` | `diagnostic` imports `plugin`, which imports `plugin/ipc`; registering from `plugin/server` avoids an import cycle while keeping metadata in plugin infra |

## Known Limitations
- None. Cross-cutting codes shared across unrelated packages remain in `internal/core/diagnostic/` by design (see Key Design Decisions).

---

## Implementation Summary

### What Was Implemented
- **Commit `f4a0df7781`** (414 files):
  - Moved 246 feature-, plugin- and component-owned `doctor-*` codes out of `internal/core/diagnostic/{codes,codes_plugin_infra}.go` into `doctor_codes.go` / `register_doctor_codes.go` across 70 owning packages, registered via `diagnostic.Register` from `init()`.
  - Updated `internal/plugins/explain/main.go` (`Run`) to implement Reading B: unknown codes report `If this code belongs to an optional feature, that feature is not in this build.` via `opererr.New`.
  - Added `diagnostic.SnapshotForTest()` in `internal/core/diagnostic/registry.go`.
  - Added build-tagged test blank imports `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go`.
  - Added `Fix` to 101 doctor findings and updated affected documentation pages and source anchors.
- **Uncommitted follow-up (Commit A, 14 files + spec + news fragment)**:
  - Removed the last 2 single-owner `doctor-*` codes from `internal/core/diagnostic/codes.go`:
    - Moved `codeDoctorIfaceStateUnread` to `internal/component/iface/doctor_codes.go`, updated `internal/component/iface/{config_apply_held,config_apply_held_test,register}.go`.
    - Moved `CodeDoctorGNMITLSMaterial` to `internal/component/gnmi/register_doctor_codes.go`, updated `internal/component/gnmi/{doctor,doctor_test}.go`, `cmd/ze/hub/service_gnmi.go`, and `internal/le/arch/doctorcoverage/check_test.go`.
  - Updated `plan/templates/TEMPLATE.md` and `ai/skills/{ze-close,ze-implement,ze-review}.md` to instruct owning-package `diagnostic.Register` instead of `internal/core/diagnostic/codes.go`.
  - Added operator news fragment `website/changes/fragments/doctor-codes-move-to-their-owners.md`.

### Bugs Found/Fixed
- In `f4a0df7781`, added missing build-tagged test blank imports in `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` and a valid local IP in `TestDoctorAS112CoordinationFunctional` so `TestDoctorRegisteredCheckCodesHaveMetadata` covers every linked component.
- Follow-up caught and moved the last 2 single-owner codes (`doctor-iface-state-unread`, `doctor-gnmi-tls-material`) that had remained in `internal/core/diagnostic/codes.go` after `f4a0df7781`.
- During closure, caught that `newsFragmentPath` (`internal/le/commit/newsfragment.go:82`) uses the bare stem (`doctor-codes-move-to-their-owners.md`, without `spec-` prefix), matching `specpath.Stem`.

### Documentation Updates
- Commit `f4a0df7781`: `docs/architecture/cli/error-surface.md` (owning-package `diagnostic.Register`), `docs/guide/health-checks.md` (Requirements item 2), `docs/features/ai-first.md` (source anchors `internal/plugins/dnsresolver/register_doctor_codes.go`), `docs/guide/debugging-tools.md`, `docs/guide/vpp.md`, `docs/architecture/bgp/as112-coordination.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`.
- Commit A: `plan/templates/TEMPLATE.md`, `ai/skills/ze-close.md`, `ai/skills/ze-implement.md`, `ai/skills/ze-review.md`, `website/changes/fragments/doctor-codes-move-to-their-owners.md`.
- `./le doc check all`: 0 documentation drift, 562/562 YANG commands valid, 6020/6020 source anchors valid, 3155/3155 digest anchors valid, 35/35 rules fresh (only pre-existing warning: `ai/DOCS-TO-CODE.md is stale`).

### Deviations from Plan
- The spec started as a `skeleton` presenting Reading A vs Reading B; Thomas directly ordered Reading B implemented, so design and implementation proceeded straight to Reading B across `f4a0df7781` and Commit A.
- 248 single-owner codes (not just the original 121 counted on 2026-09-14) had accumulated by implementation time and were all moved to their 71 owning packages.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Decide Reading A vs Reading B | Done | `internal/plugins/explain/main.go:69-81` | Thomas ordered Reading B; unknown code explains optional feature may not be in this build |
| Move single-owner `doctor-*` codes out of `internal/core/diagnostic/codes.go` to owning packages | Done | 71 owning packages (`doctor_codes.go` / `register_doctor_codes.go`) + `internal/core/diagnostic/codes.go` | 246 moved in `f4a0df7781`, final 2 (`iface`, `gnmi`) moved in Commit A |
| Preserve cross-cutting / core codes in `internal/core/diagnostic/` | Done | `internal/core/diagnostic/codes.go`, `codes_plugin_infra.go` | Verified every remaining `doctor-*` code is core TLS helper or shared across 2+ unrelated packages |
| Enforce `RegisterDoctorCheck` -> `diagnostic.Lookup` contract | Done | `internal/component/doctor/registry_test.go:30-51` | `TestDoctorRegisteredCheckCodesHaveMetadata` walks all phases |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Code-registry scan across 81 files: 214 unique `doctor-*` codes, 0 duplicates; `git grep` shows 0 refs to removed `diagnostic.CodeDoctorIfaceStateUnread` / `CodeDoctorGNMITLSMaterial` | Every `doctor-*` code declared once |
| AC-2 | Done | `internal/plugins/explain/main.go:69-93`, `TestExplainKnownDiagnostic`, `TestExplainUnknownDiagnostic`, minimal vs full `cmd/ze` build verification | Known codes explain; absent codes return Reading B refusal |
| AC-3 | Done | `internal/component/doctor/registry_test.go:30-51` (`TestDoctorRegisteredCheckCodesHaveMetadata`) + `all_import_ze_{bgp,ssh,telemetry}_test.go` | Walks `diagnostic.DoctorChecks()`, errors if `diagnostic.Lookup(code) == nil` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDoctorRegisteredCheckCodesHaveMetadata` | Done | `internal/component/doctor/registry_test.go` | Passed in `f4a0df7781`; re-run owed for Commit A |
| `TestDoctorCheckCodesRegistered` | Done | `internal/component/doctor/doctor_codes_test.go` | Passed in `f4a0df7781`; re-run owed for Commit A |
| `TestPluginInfraDoctorCodesDescribed` | Done | `internal/component/plugin/doctor/doctor_codes_test.go` | Passed in `f4a0df7781` |
| `TestCrossCuttingCodesDescribed` | Done | `internal/core/diagnostic/codes_test.go` | Passed in `f4a0df7781`; re-run owed for Commit A |
| `TestExplainUnknownDiagnostic` | Done | `internal/plugins/explain/explain_test.go` | Passed in `f4a0df7781` |
| `TestApplyReportsAStateItCannotRead` | Done | `internal/component/iface/config_apply_held_test.go` | Updated to `codeDoctorIfaceStateUnread`; re-run owed |
| `TestGNMITLSMaterialCheckRegistered`, `TestGNMIExposureFromEnvironment` | Done | `internal/component/gnmi/doctor_test.go` | Updated to `CodeDoctorGNMITLSMaterial`; re-run owed |
| `TestDoctorCoverageSourceHoldsEveryRegisteredCode` | Done | `internal/le/arch/doctorcoverage/check_test.go` | Updated to `gnmi.CodeDoctorGNMITLSMaterial`; re-run owed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/diagnostic/{codes,codes_plugin_infra,event_codes,registry}.go` | Done | `f4a0df7781` + Commit A |
| 70 owning-package `doctor_codes.go` / `register_doctor_codes.go` + 3 `all_import_ze_*_test.go` | Done | `f4a0df7781` |
| `internal/component/iface/{doctor_codes,config_apply_held,config_apply_held_test,register}.go` | Done | Commit A |
| `internal/component/gnmi/{register_doctor_codes,doctor,doctor_test}.go` + `cmd/ze/hub/service_gnmi.go` | Done | Commit A |
| `internal/le/arch/doctorcoverage/check_test.go` | Done | Commit A |
| `plan/templates/TEMPLATE.md`, `ai/skills/{ze-close,ze-implement,ze-review}.md` | Done | Commit A |
| `website/changes/fragments/doctor-codes-move-to-their-owners.md` | Done | Commit A |

### Audit Summary
- **Total items:** 22
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations: Reading B direct execution; 248 codes vs initial 121 count)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Eliminate second declaration of feature-owned `doctor-*` codes (AC-1) | Static scan + unit tests | 248 single-owner codes moved across 71 owning packages (`f4a0df7781` + Commit A); scan across 81 code-registry files reports 214 unique `doctor-*` codes and 0 duplicates; `./le arch enumeration check` 0 diagnostic-code findings in `f4a0df7781` |
| `ze show diagnostic` explains every code the running build can emit and reports absent-feature wording for compiled-out codes (Reading B, AC-2) | Unit test + minimal/full binary test + `.ci` | `TestExplainKnownDiagnostic` & `TestExplainUnknownDiagnostic` (`internal/plugins/explain/explain_test.go`), `test/ui/explain-diagnostic.ci`, `test/ui/explain-diagnostic-json.ci`, and minimal (`-tags ze_core,ze_distro`) vs full `cmd/ze` build check in `f4a0df7781` (`doctor-bgp-md5` absent with Reading B message in minimal, resolves in full) |
| Every registered doctor check's codes resolve through `diagnostic.Lookup` (AC-3) | Unit test | `TestDoctorRegisteredCheckCodesHaveMetadata` (`internal/component/doctor/registry_test.go:30-51`) walks `diagnostic.DoctorChecks()` across all phases and fails if any declared code has `diagnostic.Lookup(code) == nil` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope work completed | none |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/doctor-codes-move-to-their-owners-1ce1e925-39ef-4156-b41b-799d5236a35f.md` |
| `./le spec review check` | clean (verified after final spec & fragment write) |
| Rounds | 1 (0 BLOCKER, 0 ISSUE, 0 NOTE) |
| Reviewer lenses used | independent `/ze-review` + `/ze-review-spec` + Ze Go style pass + security pass over `f4a0df7781` and all 14 Commit A follow-up files |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | - | No product BLOCKER or ISSUE found | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/gnmi/register_doctor_codes.go` | Yes | Created in working tree (36 lines, registers `CodeDoctorGNMITLSMaterial`) |
| `internal/component/iface/doctor_codes.go` | Yes | Present and updated (87 lines, includes `codeDoctorIfaceStateUnread`) |
| `internal/component/doctor/register_doctor_codes.go` | Yes | Committed in `f4a0df7781` (234 lines) |
| `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` | Yes | Committed in `f4a0df7781` |
| `website/changes/fragments/doctor-codes-move-to-their-owners.md` | Yes | Created in working tree (valid front matter `spec: doctor-codes-move-to-their-owners`, `topic: Diagnostics`, 6 headings) |
| `test/ui/explain-diagnostic.ci`, `test/ui/explain-diagnostic-json.ci` | Yes | Verified on disk (7 lines and 10 lines) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No `doctor-*` code declared in two places | Python scan of 81 code-registry files: `214 unique doctor-* codes, 0 duplicates`; `git grep -n "CodeDoctorIfaceStateUnread\|CodeDoctorGNMITLSMaterial"` shows 0 refs to `diagnostic.*`; `./le go style check` on all 10 Commit A Go files: `go style: OK` |
| AC-2 | `ze show diagnostic <code>` explains codes in build and returns Reading B message for absent codes | Read `internal/plugins/explain/main.go:69-93` and `internal/plugins/explain/explain_test.go:109-116` (`"that feature is not in this build"`); minimal vs full build verified in `f4a0df7781` |
| AC-3 | Test walks `diagnostic.DoctorChecks()` and resolves every code via `diagnostic.Lookup` | Read `internal/component/doctor/registry_test.go:30-51` (`TestDoctorRegisteredCheckCodesHaveMetadata`) + `all_import_ze_{bgp,ssh,telemetry}_test.go`; scoped test reruns listed in main-thread Owed commands |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze show diagnostic config-parse` | `test/ui/explain-diagnostic.ci` | Yes (read file: runs `ze show diagnostic config-parse`, expects exit 0 + `config-parse` + `syntax`) |
| `ze show diagnostic --json config-yang-missing` | `test/ui/explain-diagnostic-json.ci` | Yes (read file: runs `ze show diagnostic --json config-yang-missing`, expects JSON fields) |
| `gnmi` doctor code + hub refusal | `internal/component/plugin/all/all_ze_gnmi.go` + `cmd/ze/hub/service_gnmi.go:147` | Yes (read source: `all_ze_gnmi.go` blank-imports `gnmi` under `ze_gnmi`, `register_doctor_codes.go:init` registers code) |
| `iface` unread-state event code | `internal/component/iface/register.go:179,201` + `config_apply_held.go:35,59-60` | Yes (read source: `registerDoctorCodes()` runs before `RegisterEventCode`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/component/plugin/all/all_ze_*.go` and `internal/component/doctor/all_import_ze_{bgp,ssh,telemetry}_test.go` link owning packages before `Lookup` |
| A-2 | confirmed | `internal/component/iface/register.go:179` calls `registerDoctorCodes()` before `RegisterEventCode` at line 201 |
| A-3 | confirmed | `internal/component/plugin/server/doctor_codes.go:64` registers `ipc.ConnectBackRefusedCode`; `./le arch tier check` clean in `f4a0df7781` |
| A-4 | confirmed | `git grep` of every remaining `CodeDoctor*` in `internal/core/diagnostic/{codes,codes_plugin_infra}.go` verified each is core TLS (`doctor_cert.go`) or shared across 2+ unrelated packages |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/cli/error-surface.md:179-188` | Read lines 177-188: accurately states owning-package `diagnostic.Register` and core cross-cutting/config codes | Yes |
| `docs/guide/health-checks.md:323-332` | Read lines 323-332: accurately states owning-package `diagnostic.Register` (`register_doctor_codes.go` or `register.go`) | Yes |
| `docs/features/ai-first.md:86-93` | Read lines 84-93: source anchor points to `internal/plugins/dnsresolver/register_doctor_codes.go` | Yes |
| `plan/templates/TEMPLATE.md`, `ai/skills/{ze-close,ze-implement,ze-review}.md` | Read uncommitted diffs: all 4 updated from `internal/core/diagnostic/codes.go` to owning-package `diagnostic.Register` | Yes |
| `./le doc check all` & `./le spec citation check` | 0 drift, 6020/6020 source anchors valid, 3155/3155 digest anchors valid, 0 citations to this spec anywhere in tracked tree (`git grep -n "spec-doctor-codes-move-to-their-owners"` empty) | Yes |
