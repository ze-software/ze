# Spec: feature-gate child 9 -- VRRP compile-out (ze_vrrp)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-07-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/feature-gate-registration.md` - the compile-out mechanism (Plugin compile-out shape)
4. `plan/learned/995-feature-gate-8-protocols.md` - closest precedent (plugin blank-import partitioning)
5. `feature-gates.txt` - the single source of truth for gated packages
6. `internal/plugins/vrrp/register.go`, `internal/plugins/vrrp/transport/register.go` - the two discovered registration units

## Task

Make the **VRRP first-hop-redundancy plugin compile-out-able** via a `//go:build ze_vrrp`
build tag, so a hardened / minimal `ze` binary can be built without any VRRP code
(smaller binary, smaller attack surface). This is a mechanical **enrollment into the
existing feature-gate system** (`feature-gates.txt` + `scripts/codegen/plugin_imports.go`),
NOT a new mechanism. The precedent is the routing-protocol children
(`plan/learned/995-feature-gate-8-protocols.md`): VRRP is a self-registering plugin, so
gating is pure **blank-import partitioning** -- no source-level `//go:build` tags on the
plugin's `.go` files, no `register_<x>.go`, no seam.

VRRP matches the **simplest** precedent shape (ldp / rsvpte): a single plugin with a
`transport` sidecar, whose CLI is registered through the plugin registry's `CLIHandler`
(NOT a programmatic `cli` dispatch package), so -- unlike isis / ospf -- it needs **no**
`cmd/ze/dispatch_vrrp.go` companion.

**Default-on**: adding `ze_vrrp` to `feature-gates.txt` puts it in `ZE_FEATURES`, so
`make ze` and `ze-appliance` keep shipping VRRP unchanged; only `ze-stripped` and bare
`ze_core` builds drop it. (User goal: "so that code *can* be compiled out" => on by
default, optionally out -- matching all 12 existing gates.)

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/feature-gate-registration.md` - the whole procedure; the "Plugin compile-out (routing protocols)" section is the exact shape VRRP uses.
  → Decision: gating a self-registering plugin is blank-import partitioning only; the plugin `.go` files carry NO `//go:build` tag.
  → Constraint: `feature-gates.txt` is the ONE declaration point; every other consumer (Makefile, runner, generator, dep_audit) DERIVES from it. `.golangci.yml` is the only non-deriving consumer and is drift-checked.
  → Constraint: the one invariant -- nothing always-on (untagged, non-test) may import a gated package. Verified below (A-1).
- [ ] `ai/rules/plugin-self-containment.md` - VRRP already satisfies "delete the folder, feature vanishes"; the gate is the compile-time projection of that.
  → Constraint: no plugin spelling belongs in a generic/central package. Exception already in tree: the `ze explain` catalogue (see Known Limitations).
- [ ] `ai/rules/module-tiers.md` - disable-ability + the gate; VRRP is an `internal/plugins/` edge engine (`sdk.NewWithConn`), like the protocols.
  → Constraint: a gated edge engine's blank import living in the generated `all_<tag>.go` is registration-only, not a tier violation (dep_audit already models this).

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md`, `rfc/short/rfc3768.md` - VRRPv3 / VRRPv2. Referenced only for context; this spec changes **no** protocol behavior, only build wiring.
  → Constraint: none new. No wire-format, FSM, or config-semantics change is in scope.

### Learned Summaries (precedent)
- [ ] `plan/learned/995-feature-gate-8-protocols.md` - isis/ldp/ospf/rsvpte plugin compile-out.
  → Decision: gate the plugin whole (codec + engine + transport) by gating blank imports; `nm` proves zero symbols in a bare-core build.
  → Decision: one plugin = several manifest lines (one per discovered dir under the shared tag); `<pkg>/yang` rides the plugin line automatically.
  → Constraint: absent build-tag tests MUST live in `cmd/ze/hub` (only that package is compiled with bare `ze_core` by `GO_TEST_CORE`); a `//go:build !ze_<x>` test elsewhere is silently skipped.
- [ ] `plan/learned/983-feature-gate-manifest-ssot.md` - the derive-from-manifest refactor that makes step 3 the only declaration point.

**Key insights:** (minimal context to resume after compaction)
- VRRP = ldp/rsvpte shape: single plugin + `transport` sidecar, registry-`CLIHandler` (no `cli` dispatch pkg), so NO `dispatch_vrrp.go`.
- Two manifest lines needed (`internal/plugins/vrrp`, `internal/plugins/vrrp/transport`); `vrrp/yang` auto-derived from line 1; `fsm`/`packet` have no `register.go` and drop by dead-code elimination.
- Zero always-on importers of any vrrp package (grep-proven) => gate whole, no extract-then-gate.
- No code change to `dep_audit.py`, `Makefile`, or the test runner -- all derive from the manifest; the vrrp->transport intra-feature import is already handled by `_same_feature_importer`.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/vrrp/register.go` - `func init() { registerVRRP() }` registers plugin `"vrrp"` with `ConfigRoots: ["interface"]`, `Dependencies: ["interface"]`, `RunEngine`, `CLIHandler` (via `cli.RunPlugin`), doctor check `vrrp-config-sanity`, diagnostic codes, event namespace.
  → Constraint: CLI is registered through `reg.CLIHandler` (registry), NOT a `cmd/ze` dispatch-root import. Confirmed: `grep -n vrrp cmd/ze/*.go` is empty.
- [ ] `internal/plugins/vrrp/transport/register.go` - separate discovered registration unit (proto-112 sockets, GARP/NA). Imported by the main package (`register.go` line 33), so it is intra-feature.
- [ ] `internal/plugins/vrrp/yang/register.go` - registers `ze-vrrp-conf.yang` (config, augments interface units) and `ze-vrrp-cmd.yang` (command schema). Discovered as a schema package.
- [ ] `internal/component/plugin/all/all.go` (generated) - currently blank-imports `internal/plugins/vrrp` (plugins), `internal/plugins/vrrp/transport` (plugins), `internal/plugins/vrrp/yang` (schema). These three lines move into a new `all_ze_vrrp.go`.
- [ ] `scripts/codegen/plugin_imports.go` - `loadFeatureTags` reads `feature-gates.txt`; for each `<tag> <pkg>` line it gates `<pkg>` AND `<pkg>/yang`. `filterTagged` moves tagged imports into `all_<tag>.go` (`//go:build <tag>`).
- [ ] `feature-gates.txt` - manifest; VRRP is currently ABSENT (always-on).
- [ ] `internal/core/diagnostic/codes.go` - central `ze explain` catalogue (pure strings, zero plugin imports); catalogues `doctor-vrrp-raw-socket` alongside `doctor-isis-*`, `doctor-ospf-*`, `doctor-ldp-*`, `doctor-rsvpte-*`. Established pattern; stays always-on (see Known Limitations).
- [ ] `.golangci.yml` - static `build-tags:` list = `ze_core` + every gate tag; the one non-deriving consumer; currently ends at `ze_rsvpte`.
- [ ] `docs/features.md` - the VRRP row (line 31) has NO compile-out sentence yet; the isis/ospf/ldp rows do (the phrasing to mirror).

**Behavior to preserve:**
- Default `ze` / `ze-appliance` binaries keep VRRP: `pluginreg.Has("vrrp")` stays true; the shipped feature set is byte-unchanged except for the generated `all.go` / `all_ze_vrrp.go` split.
- VRRP protocol behavior, config grammar, FSM, dataplane, doctor checks, metrics -- all unchanged.
- `feature-gates.txt` remains the sole declaration point; no parallel hand-maintained list appears.

**Behavior to change:**
- With the `ze_vrrp` tag ABSENT, `internal/plugins/vrrp` (+ `transport`, `fsm`, `packet`, `yang`) is not linked, the plugin does not register, and a config that carries a `vrrp {}` block under an interface unit is rejected as an unknown field.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Build time: `go build -tags '<feature set>' ./cmd/ze`. The tag set decides which `all_<tag>.go` files compile in.
- Run time (unchanged when present): config file -> tree -> interface schema (augmented by `vrrp/yang`) -> in-process verifier -> `runVRRPEngine`.

### Transformation Path
1. `make generate` runs `scripts/codegen/plugin_imports.go`, which reads `feature-gates.txt`, matches the two `ze_vrrp` lines (+ derived `vrrp/yang`), and partitions those blank imports out of `all.go` into `all_ze_vrrp.go` (`//go:build ze_vrrp`).
2. `go build` with `ze_vrrp` present: `all_ze_vrrp.go` compiles, its blank imports trigger the vrrp `init()`s, the plugin registers -> identical to today.
3. `go build` with `ze_vrrp` absent: `all_ze_vrrp.go` is excluded; nothing references `internal/plugins/vrrp*`; the linker drops all vrrp packages (dead-code elimination), including the transitively-only `fsm` and `packet`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Manifest -> generator | `feature-gates.txt` two lines -> `loadFeatureTags` -> `all_ze_vrrp.go` | [ ] inspect generated file after `make generate` |
| Manifest -> Makefile / runner / dep_audit | `awk`/`load_feature_gates` derive `ze_vrrp` automatically | [ ] `ZE_FEATURES` includes `ze_vrrp`; `make ze-tier-check` green |
| Build tag -> registry | `all_ze_vrrp.go` blank import -> `init()` -> `registry.Register` | [ ] present/absent registration tests |
| Config schema -> parser | `vrrp/yang` gated => interface-unit `vrrp {}` augment absent | [ ] absent test parses a vrrp config and asserts unknown-field rejection |

### Integration Points
- `feature-gates.txt` (declaration), `scripts/codegen/plugin_imports.go` (derives, no code change), `Makefile ZE_FEATURES` (derives), `internal/test/runner TestBuildTags` (derives), `scripts/dev/dep_audit.py DISABLEABLE` (derives), `.golangci.yml build-tags` (hand-edit, drift-checked).

### Architectural Verification
- [ ] No bypassed layers (gating is at the composition root only; no plugin source edited)
- [ ] No unintended coupling (VRRP already imports only always-on packages; nothing always-on imports VRRP)
- [ ] No duplicated functionality (reuses the existing feature-gate machinery; adds zero new mechanism)
- [ ] Zero-copy preserved where applicable (N/A -- no runtime path changes)
- [ ] Registration over hardcoding -- VRRP is discovered by the generator; the gate is manifest-driven, no new switch/field/factory anywhere

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing always-on (untagged, non-test) imports any vrrp package, so VRRP can be gated whole with no extract-then-gate step. | `grep -rln internal/plugins/vrrp` returned only the vrrp tree itself + generated `all.go` (RESEARCH). | The gate would pin vrrp into every binary; would need extract-then-gate first. | `scripts/dev/dep_audit.py --check` (`make ze-tier-check`) after adding the manifest lines. | confirmed (audit grep empty; final proof via ze-tier-check) |
| A-2 | The vrrp->transport intra-feature import needs NO `dep_audit.py` code change; `_same_feature_importer` already skips same-tag importers generically (manifest-driven). | `dep_audit.py` uses `load_feature_gates(root)`, `_same_feature_importer(imp, tag, disableable)`, `manifest_tags = set(load_feature_gates(root).values())` -- no hardcoded tags. ospf has the identical plugin->transport shape and passes. | Would need a dep_audit fix (already generalized for protocols; unlikely). | `make ze-tier-check` after the change, with zero edits to `dep_audit.py`. | confirmed (structural: manifest-driven fns present; final proof via ze-tier-check) |
| A-3 | VRRP needs NO `cmd/ze/dispatch_vrrp.go` companion (it registers CLI via `reg.CLIHandler`, not a programmatic `cli` dispatch package). | `grep -n vrrp cmd/ze/*.go` empty; `register.go:81` sets `reg.CLIHandler`; only `dispatch_isis.go`/`dispatch_ospf.go` exist. | vrrp CLI symbols would leak into a stripped build via the dispatch root. | The absent `nm` symbol-drop test (AC-6) proves zero vrrp symbols across all composition roots. | confirmed (cmd/ze grep empty; final proof via absent nm test) |
| A-4 | `vrrp/yang` (both `ze-vrrp-conf.yang` and `ze-vrrp-cmd.yang`) rides the `internal/plugins/vrrp` manifest line via the generator's `<pkg>/yang` derivation. | `loadFeatureTags` adds `path.Join(pkg,"yang") -> tag` for every line; both YANG modules are registered by the single `vrrp/yang` package (`register.go`). | The command/config schema would stay always-on and reject nothing / leak. | Inspect generated `all_ze_vrrp.go` includes `internal/plugins/vrrp/yang`; absent test asserts config rejection. | confirmed (all_ze_vrrp.go includes vrrp/yang; `TestBuildTag_VRRP_AbsentRejectsVRRPConfig` passes) |
| A-5 | `fsm` and `packet` (no `register.go`, imported only transitively) drop by dead-code elimination when the plugin blank imports are gated out; they need no manifest line. | Generator discovers by `register.go`; `fsm`/`packet` have none (RESEARCH). Protocol codecs (`packet`/`types`) dropped the same way (995). | Residual vrrp symbols in a stripped build. | Absent `nm` test greps `internal/plugins/vrrp/` (covers `fsm`/`packet` subpaths). | confirmed (bare-`ze_core` `nm` finds zero `internal/plugins/vrrp` symbols) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `test/vrrp/*.ci` functional test is exercised against a stripped binary and fails (vrrp absent). | `make ze-functional-test` red on vrrp `.ci` after the change. | vrrp `.ci` tests run against the full `ze` binary (vrrp default-on); confirm no vrrp `.ci` is in the `ze-stripped` set. The `internal/test/cli/register.go` "vrrp" CI root is a string registration (no plugin import), unaffected. |
| R-2 | A composition root is missed and vrrp stays linked in a bare-core build. | Absent `nm` test finds a `internal/plugins/vrrp` symbol. | Lower risk than protocols (no dispatch root); the `nm` test is the backstop across both roots. |
| R-3 | The firewall `"vrrp": 112` protocol-name map (`internal/component/firewall/protocol.go`, `.../firewall/vpp/translate.go`) is mistaken for a plugin pin. | dep_audit flags it, or reviewer questions it. | It is a generic IP-protocol-number map (string->int), NOT an import of the vrrp package; it stays always-on and is correct. The `nm` needle is package-path-scoped (`internal/plugins/vrrp`), so a string literal elsewhere never matches. |
| R-4 | Regeneration rewrites unrelated lines in `all.go` (concurrent session drift). | `git diff internal/component/plugin/all/all.go` shows more than the 3 removed vrrp lines. | Expect the `all.go` diff to be exactly the 3 removed vrrp imports; if not, reconcile before committing (995 gotcha). |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ...ze_vrrp...'` (default set) | → | vrrp `init()` via `all_ze_vrrp.go` -> `registry.Register("vrrp")` | `TestBuildTag_VRRP_Present` (`//go:build ze_vrrp`) asserts `pluginreg.Has("vrrp")` |
| `go build -tags ze_core` (stripped) | → | vrrp NOT linked; registry lacks `"vrrp"`; `vrrp {}` config rejected | `TestBuildTag_VRRP_Absent` (`//go:build !ze_vrrp`) asserts registry non-empty, `!Has("vrrp")`, config rejection |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Add two lines to `feature-gates.txt`: `ze_vrrp internal/plugins/vrrp` and `ze_vrrp internal/plugins/vrrp/transport`. | Manifest declares the gate; `ZE_FEATURES` (Makefile awk) and `TestBuildTags` (runner) auto-include `ze_vrrp`. |
| AC-2 | Run `make generate`. | `internal/component/plugin/all/all_ze_vrrp.go` (`//go:build ze_vrrp`) is created blank-importing `internal/plugins/vrrp`, `internal/plugins/vrrp/transport`, and `internal/plugins/vrrp/yang`; `all.go` no longer imports those three. |
| AC-3 | Add `ze_vrrp` to `.golangci.yml` `build-tags`. | Lint covers the gated files; `dep_audit.py --check` reports no `.golangci.yml` drift. |
| AC-4 | Build `ze` with the default feature set and populate the plugin registry. | `pluginreg.Has("vrrp")` is true (VRRP still shipped by default). |
| AC-5 | Build with bare `ze_core` (vrrp absent) and (a) inspect the registry, (b) parse a config with a `vrrp {}` block under an interface unit. | (a) `pluginreg.Names()` non-empty and lacks `"vrrp"`; (b) parse fails with a clean unknown-field rejection. |
| AC-6 | `go tool nm` on a bare-`ze_core` `ze` binary. | Zero symbols matching `internal/plugins/vrrp` (covers `vrrp`, `transport`, `fsm`, `packet`, `yang`). |
| AC-7 | Run `make ze-tier-check` (`dep_audit.py --check`) after the change. | Passes with NO edit to `scripts/dev/dep_audit.py`, `Makefile`, or `internal/test/runner`. |
| AC-8 | `docs/features.md` VRRP row. | Gains a compile-out sentence (mirroring the isis/ospf/ldp rows) with `feature-gates.txt`, `all_ze_vrrp.go` source anchors; `ai/rules/feature-gate-registration.md` inventory line adds `ze_vrrp`. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds a hardened `ze` without VRRP (`make ze-stripped` or `go build -tags ze_core`). | manifest tags omit `ze_vrrp` -> `all_ze_vrrp.go` excluded -> vrrp packages unreferenced -> linker drops them | `TestBuildTag_VRRP_Absent` + the absent `nm` symbol-drop assertion |
| 2 | Feeds that stripped binary a config with a `vrrp {}` group under an interface unit. | `vrrp/yang` augment absent -> parser sees unknown field `vrrp` | `TestBuildTag_VRRP_AbsentRejectsVRRPConfig` |
| 3 | Builds and runs the default `ze` (VRRP included). | `ze_vrrp` in `ZE_FEATURES` -> `all_ze_vrrp.go` compiled -> plugin registers and runs exactly as today | `TestBuildTag_VRRP_Present`; existing `test/vrrp/*.ci` still green against full `ze` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_VRRP_Present` | `cmd/ze/hub/build_tag_vrrp_present_test.go` (`//go:build ze_vrrp`) | vrrp registered when the tag is set | |
| `TestBuildTag_VRRP_Absent` | `cmd/ze/hub/build_tag_vrrp_absent_test.go` (`//go:build !ze_vrrp`) | registry non-empty and lacks `"vrrp"` | |
| `TestBuildTag_VRRP_AbsentRejectsVRRPConfig` | `cmd/ze/hub/build_tag_vrrp_absent_test.go` | interface-unit `vrrp {}` config rejected as unknown | |
| `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` (extended) | `cmd/ze/hub/build_tag_protocols_absent_test.go` | add `&& !ze_vrrp` to the build constraint + `internal/plugins/vrrp` needles, so the single bare-`ze_core` build also proves vrrp symbol-drop | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no numeric inputs added | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `test/vrrp/*.ci` (unchanged) | `test/vrrp/` | VRRP still works in the default `ze` binary; no regression | reuse |

### Interop Tests (MANDATORY for protocol features)
Skipped with justification: this spec changes **no wire protocol behavior** -- only build-time
package inclusion. VRRP interop against keepalived already exists and is unaffected. No new
interop scenario is warranted.

### Future (if deferring any tests)
- None. All tests land in this spec.

## Files to Modify
- `feature-gates.txt` - add the two `ze_vrrp` lines (declaration; every derived consumer follows).
- `.golangci.yml` - `ze_vrrp` in `build-tags` (now GENERATED by `feature_tags.go`).
- `gokrazy/ze/config.json` - `ze_vrrp` in `GoBuildTags` (now GENERATED by `feature_tags.go`; the appliance image build). Enforced by `internal/appliance` `TestGokrazyConfigMatchesApplianceBuildTags` (`cmd_build_test.go:131-139`).
- `docs/guide/quickstart.md` - `ze_vrrp` in the `go install -tags '...'` command (now GENERATED by `feature_tags.go`).
- `scripts/dev/stress-repro.py` - `race_tags` DERIVED from `feature-gates.txt` via `_feature_gate_tags()` (was hardcoded; missed `ze_vrrp`).
- `Makefile` - `generate` runs `feature_tags.go`; new `ze-feature-tags-check` target.
- `ai/CODE-TO-DOCS.md` - regenerated (`make ze-doc-index`) for the `all_ze_vrrp.go` anchor.

**Files to Create (feature-tag SSOT):**
- `scripts/codegen/feature_tags.go` - the generator (`//go:build ignore`).
- `scripts/codegen/feature_tags_test.go` - `--check` currency + coverage + read-only tests.
- `internal/component/plugin/all/all.go` - regenerated (loses the 3 vrrp blank imports). **Generated; produced by `make generate`, never hand-edited.**
- `cmd/ze/hub/build_tag_protocols_absent_test.go` - extend build constraint (`&& !ze_vrrp`) + add vrrp needles so the existing single bare-core build also proves vrrp drop.
- `docs/features.md` - VRRP row: add the compile-out sentence + source anchors (mirror isis/ospf/ldp).
- `ai/rules/feature-gate-registration.md` - add `ze_vrrp` to the intro inventory of gated features (discovery-updates).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | none new -- `vrrp/yang` already exists; only its blank import moves |
| CLI commands/flags | [ ] No | vrrp CLI via `reg.CLIHandler`; no `cmd/ze` dispatch change (A-3) |
| Functional test for new RPC/API | [ ] No | no new RPC; existing `.ci` reused |
| Doctor check for runtime dependencies | [ ] No | no new runtime dependency; existing doctor checks ride the gated plugin |
| Prometheus counters/metrics | [ ] No | `ze_vrrp_*` metrics ride the gated plugin; absent when compiled out |
| dep_audit / Makefile / runner | [ ] No (derive from manifest) | `scripts/dev/dep_audit.py`, `Makefile`, `internal/test/runner` -- NO edits (A-2) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No (build option, not a runtime feature) | - |
| 3 | CLI command added/changed? | [ ] No | - |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [ ] Yes | `docs/features.md` VRRP row compile-out sentence; `ai/rules/feature-gate-registration.md` inventory line |
| 15 | Registered plugin / runtime inventory changed? | [ ] Yes (now compile-out-able) | `docs/features.md` VRRP row (source anchors: `feature-gates.txt`, `all_ze_vrrp.go`) |
| 16 | Changed source file referenced by doc source anchors? | [ ] Check | grep `docs/` for `source: feature-gates.txt` and vrrp anchors; update if stale |

## Files to Create
- `internal/component/plugin/all/all_ze_vrrp.go` - **generated** by `make generate` (`//go:build ze_vrrp`); blank-imports vrrp, vrrp/transport, vrrp/yang.
- `cmd/ze/hub/build_tag_vrrp_present_test.go` - `//go:build ze_vrrp`; `TestBuildTag_VRRP_Present`.
- `cmd/ze/hub/build_tag_vrrp_absent_test.go` - `//go:build !ze_vrrp`; `TestBuildTag_VRRP_Absent` + `TestBuildTag_VRRP_AbsentRejectsVRRPConfig`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Assumptions A-1..A-5 (validate by grep/read BEFORE coding) |
| 3. Wiring phase | Wiring Test table -- add manifest lines, write present/absent tests |
| 4. Implement | Phases below |
| 5. Full verification | `make generate` + `make ze-verify-changed` |
| 6-9. Review loop | Critical Review Checklist below |
| 10-12. Deliverables / security / docs | Checklists below |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases

1. **Phase: Manifest + generate (wiring first)** — declare the gate, regenerate.
   - Files: `feature-gates.txt` (+2 lines), then `make generate` -> `all_ze_vrrp.go` created, `all.go` updated.
   - Verify: `git diff all.go` shows exactly the 3 removed vrrp imports; `all_ze_vrrp.go` has vrrp + transport + yang; `plugin_imports.go --check` clean.
2. **Phase: Lint consumer** — `.golangci.yml` `build-tags` gains `ze_vrrp`.
   - Verify: `dep_audit.py --check` reports no golangci drift.
3. **Phase: Present/absent tests** — write the three test funcs in `cmd/ze/hub`; extend the consolidated `nm` test.
   - Verify: with default tags `Present` passes; with bare `ze_core` `Absent` + config-rejection + `nm` drop pass.
4. **Phase: Tier + verify** — `make ze-tier-check` (proves A-1, A-2, A-7 with zero dep_audit edits), `make ze-verify-changed`.
5. **Phase: Docs** — `docs/features.md` VRRP row + `ai/rules/feature-gate-registration.md` inventory; `make ze-doc-test`.
6. **Complete spec** — audit tables, learned summary `plan/learned/NNN-feature-gate-9-vrrp.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has file:line evidence |
| Correctness | `all.go` diff is exactly 3 removed lines; `all_ze_vrrp.go` lists all 3 vrrp packages incl. yang |
| No source tags | vrrp plugin `.go` files carry NO `//go:build ze_vrrp` (blank-import partitioning only) |
| No stray edits | `dep_audit.py`, `Makefile`, `internal/test/runner` UNCHANGED (derive from manifest) |
| No dispatch companion | No `cmd/ze/dispatch_vrrp.go` created; `grep vrrp cmd/ze/*.go` still empty (A-3) |
| Registration over hardcoding | gate is manifest-driven; no new switch/field/factory |
| Absent symbol-drop | bare-core `nm` has zero `internal/plugins/vrrp` symbols (incl. fsm/packet) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `feature-gates.txt` two lines | `grep ze_vrrp feature-gates.txt` |
| `all_ze_vrrp.go` generated | `ls internal/component/plugin/all/all_ze_vrrp.go`; `plugin_imports.go --check` |
| `.golangci.yml` tag | `grep ze_vrrp .golangci.yml` |
| present/absent tests | `go test -tags 'ze_core ze_vrrp' ./cmd/ze/hub -run VRRP` and `go test -tags ze_core ./cmd/ze/hub -run VRRP` |
| default build still has vrrp | build + `pluginreg.Has("vrrp")` |
| tier check clean, no dep_audit edit | `make ze-tier-check`; `git diff --stat scripts/dev/dep_audit.py Makefile internal/test/runner` empty |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Attack-surface reduction | a stripped build genuinely omits vrrp raw-socket / macvlan code (the point of the gate); `nm` proves it |
| No new input surface | no new parser/RPC/flag; config rejection of `vrrp {}` when absent is a clean unknown-field error, not a crash |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `all.go` diff larger than 3 lines | Reconcile concurrent regen drift (R-4) before committing |
| `nm` finds a vrrp symbol in bare core | Find the missed always-on importer / composition root; fix at source |
| dep_audit flags vrrp->transport | Re-check `_same_feature_importer` sees both under `ze_vrrp`; do NOT baseline it |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `.golangci.yml` is the ONLY hand-maintained (non-deriving) consumer of the gate tags (per `ai/rules/feature-gate-registration.md`, which listed only it). | There are FOUR full-tag-list consumers the rule never fully listed: `.golangci.yml`, `gokrazy/ze/config.json` `GoBuildTags`, `docs/guide/quickstart.md`'s `go install` command, and `scripts/dev/stress-repro.py` `race_tags`. | Full `make ze-verify` failed the appliance test on gokrazy; an independent review + a broader (all-extension) sweep found quickstart + stress-repro. My first canary grep missed them (only `.json/.yml/.go/.mk/.txt`, not `.md`/`.py`). | Root fix (user-directed): the three static files are now GENERATED from `feature-gates.txt` by `feature_tags.go`; stress-repro DERIVES at runtime. Hand-maintenance eliminated; drift is now impossible. |
### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- VRRP is the first **non-routing-protocol** plugin gated by the routing-protocol shape. It proves that shape generalizes to any self-registering `internal/plugins/` engine with a `transport` sidecar and registry-based CLI: the "protocol" framing in `995` was incidental; the mechanism is "self-registering plugin, no always-on importer".
- The absence of a `cli` dispatch package (VRRP uses `reg.CLIHandler`) is what removes the `dispatch_<x>.go` companion -- the single biggest simplification vs isis/ospf, and the reason VRRP is a ldp/rsvpte-class change.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Default-ON (`ze_vrrp` in `ZE_FEATURES`) | Default-off (opt-in) | Matches all 12 existing gates; user asked that code *can* be compiled out, i.e. shipped by default, dropped on request. `ze`/`ze-appliance` stay unchanged. |
| Gate the plugin **whole** via blank-import partitioning; NO source `//go:build` tags | Source-tag each vrrp `.go` file (telemetry's exporter shape) | Precedent 995: a self-registering plugin with zero always-on importers (A-1) is gated purely at the composition root; source tags would be noise and risk U1000-unused churn. |
| Two manifest lines (`vrrp`, `vrrp/transport`); `yang` auto-derived; `fsm`/`packet` unlisted | One line; or list all five dirs | Generator gates `<pkg>`+`<pkg>/yang` per line, and discovers only dirs with `register.go`. `transport` has its own `register.go` (needs a line); `fsm`/`packet` have none and drop by DCE. |
| No `cmd/ze/dispatch_vrrp.go` companion | Add one mirroring `dispatch_isis.go` | VRRP registers CLI through the registry `CLIHandler`, not a programmatic `cli` dispatch package; `grep vrrp cmd/ze/*.go` is empty (A-3). Adding one would be dead code. |
| Extend the consolidated `build_tag_protocols_absent_test.go` `nm` proof (add `!ze_vrrp` + vrrp needles) | A dedicated `build_tag_vrrp_absent_test.go` with its own second bare-core build | The consolidated test already does one bare-`ze_core` build; adding vrrp needles reuses it for free (the cost optimization `995` explicitly made). A second identical `go build` in the suite is pure waste. Registration/config-rejection tests stay in a dedicated vrrp file; only the binary `nm` proof is shared. |
| Leave `dep_audit.py` / `Makefile` / runner untouched | Edit them for vrrp | They all derive from `feature-gates.txt`; the vrrp->transport intra-feature import is already handled generically by `_same_feature_importer` (A-2). Editing them would reintroduce the hand-wiring `983` removed. |

## Known Limitations
- The `ze explain` catalogue `internal/core/diagnostic/codes.go` keeps `doctor-vrrp-raw-socket` (and the generic firewall `"vrrp": 112` proto-number map) always-on **by design** -- both are plugin-import-free string data. A stripped binary retains these strings (a harmless `ze explain` entry / proto lookup for an absent feature). This exactly mirrors the isis/ospf/ldp/rsvpte codes, which `995` also left catalogued. This spec deliberately does not change them: it is a separate, pre-existing self-containment matter unrelated to compile-out, and the always-on catalogue is the intended design (a stripped `ze explain` still resolves the code).
- This spec does not change any VRRP runtime behavior, config grammar, or the known `accept-mode` dataplane-enforcement gap (tracked with the VRRP feature, unrelated to compile-out).

## RFC Documentation
No enforcing code added; no RFC constraint comments in scope. VRRP RFC behavior is unchanged.

## Implementation Summary
### What Was Implemented
- `feature-gates.txt`: two `ze_vrrp` lines (`internal/plugins/vrrp`, `internal/plugins/vrrp/transport`); `vrrp/yang` auto-derived.
- `make generate`: created `internal/component/plugin/all/all_ze_vrrp.go` (`//go:build ze_vrrp`, 3 vrrp blank imports incl. yang); removed the 3 vrrp lines from `all.go` (diff = exactly 3 lines, R-4 clean).
- **Feature-tag SSOT generation (user-requested during implementation).** Instead of hand-maintaining the static tag lists + drift-gating them, they are now GENERATED from `feature-gates.txt` by a new `scripts/codegen/feature_tags.go` (+`_test.go`) wired into `make generate`: `.golangci.yml` `build-tags`, `gokrazy/ze/config.json` `GoBuildTags`, and `docs/guide/quickstart.md`'s `go install -tags '...'` command (surgical byte-stable edits). `scripts/dev/stress-repro.py` `race_tags` now derives at runtime via `_feature_gate_tags()`. So `ze_vrrp` reaches all four static/program consumers with zero hand-edits, and no future gate can drift. Rule `ai/rules/feature-gate-registration.md` updated: these are now generated/deriving consumers, not hand-maintained.
- Tests (`cmd/ze/hub`): `build_tag_vrrp_present_test.go` (`TestBuildTag_VRRP_Present`), `build_tag_vrrp_absent_test.go` (`TestBuildTag_VRRP_Absent`, `TestBuildTag_VRRP_AbsentRejectsVRRPConfig`); extended `build_tag_protocols_absent_test.go` (constraint `&& !ze_vrrp`, vrrp needles, generalized comment/message).
- Docs: `docs/features.md` VRRP row compile-out sentence + anchors; `ai/rules/feature-gate-registration.md` inventory + single-plugin/no-dispatch note; regenerated `ai/DOCS-TO-CODE.md`.
- NO edits to `scripts/dev/dep_audit.py`, `Makefile`, `internal/test/runner`, or any vrrp plugin `.go` file (all derive from the manifest; blank-import partitioning only).
### Bugs Found/Fixed
- None. No pre-existing behavior changed.
### Documentation Updates
- `docs/features.md` VRRP row: compile-out sentence, anchors `<!-- source: feature-gates.txt -- ze_vrrp -->`, `<!-- source: internal/component/plugin/all/all_ze_vrrp.go -- gated VRRP imports -->`.
- `ai/rules/feature-gate-registration.md`: `ze_vrrp` in the gated-feature inventory; note that a registry-`CLIHandler` plugin needs no dispatch companion.
- `ai/DOCS-TO-CODE.md`: regenerated (picks up the two new test files' `// Design:` headers).
- `make ze-doc-test`: PASSED.
### Deviations from Plan
- **Scope grew (user-directed).** The original spec assumed `.golangci.yml` was the only hand-maintained consumer (per the rule). Full `ze-verify` proved `gokrazy/ze/config.json` `GoBuildTags` was a second one (appliance test caught the missing `ze_vrrp`), and an independent review + a broader sweep found two more full-tag-list consumers the rule never mentioned: `docs/guide/quickstart.md`'s `go install` command and `scripts/dev/stress-repro.py` `race_tags`. Rather than hand-fix + document (the initial approach), the user directed a single-source-of-truth fix: GENERATE the three static files from `feature-gates.txt` (`scripts/codegen/feature_tags.go`, wired into `make generate`) and DERIVE the Python one at runtime. This eliminates the hand-maintained-consumer class entirely.
- Regenerated `ai/CODE-TO-DOCS.md` (an independent-review ISSUE: the `all_ze_vrrp.go` -> `docs/features.md` anchor was missing).
- Note (not a deviation): a concurrent session's IS-IS RFC5303 work (untracked `internal/plugins/isis/circuit/threeway_rfc5303_test.go`, modified `rfc/enrolled.txt`) made `ai/RFC-REQUIREMENTS.md` stale mid-session; regenerating it (via `make ze-rfc-index`) was needed for a green `ze-doc-test` but that file is NOT part of this spec's commit (it belongs to the isis session).

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| VRRP compile-out via `ze_vrrp` tag | Done | `feature-gates.txt`, `all_ze_vrrp.go` | blank-import partitioning |
| Default-on (ship in `ze`/`ze-appliance`) | Done | `ZE_FEATURES` awk includes `ze_vrrp` | byte-behavior preserved |
| No new mechanism | Done | manifest + generator only | dep_audit/Makefile/runner untouched |
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `feature-gates.txt` 2 lines | `grep ze_vrrp feature-gates.txt` |
| AC-2 | Done | `all_ze_vrrp.go` + `all.go` diff | `plugin_imports.go --check` exit 0 |
| AC-3 | Done | `.golangci.yml` `ze_vrrp` | drift check clean |
| AC-4 | Done | `TestBuildTag_VRRP_Present` | PASS |
| AC-5 | Done | `TestBuildTag_VRRP_Absent(+RejectsVRRPConfig)` | PASS |
| AC-6 | Done | `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` | PASS (nm) |
| AC-7 | Done | `make ze-tier-check` exit 0 | dep_audit/Makefile/runner diff empty |
| AC-8 | Done | `docs/features.md`, `ai/rules/feature-gate-registration.md` | `make ze-doc-test` PASS |
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|
| `TestBuildTag_VRRP_Present` | Pass | `cmd/ze/hub/build_tag_vrrp_present_test.go` | `ze_core ze_vrrp` |
| `TestBuildTag_VRRP_Absent` | Pass | `cmd/ze/hub/build_tag_vrrp_absent_test.go` | bare `ze_core` |
| `TestBuildTag_VRRP_AbsentRejectsVRRPConfig` | Pass | same file | unknown-field rejection |
| `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` (extended) | Pass | `cmd/ze/hub/build_tag_protocols_absent_test.go` | +vrrp needles, +`!ze_vrrp` |
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `feature-gates.txt` | Modified | +2 lines |
| `.golangci.yml` | Modified | +1 tag (non-deriving consumer #1) |
| `gokrazy/ze/config.json` | Modified | +ze_vrrp GoBuildTags (non-deriving consumer #2) |
| `internal/component/plugin/all/all.go` | Modified (generated) | -3 vrrp imports |
| `internal/component/plugin/all/all_ze_vrrp.go` | Created (generated) | 3 vrrp imports |
| `cmd/ze/hub/build_tag_vrrp_present_test.go` | Created | present test |
| `cmd/ze/hub/build_tag_vrrp_absent_test.go` | Created | absent + config-reject |
| `cmd/ze/hub/build_tag_protocols_absent_test.go` | Modified | vrrp nm coverage |
| `docs/features.md` | Modified | compile-out sentence |
| `ai/rules/feature-gate-registration.md` | Modified | inventory + note |
| `ai/DOCS-TO-CODE.md` | Modified (generated) | new test files |
### Audit Summary
- **Total items:** 8 ACs + 3 requirements + 4 tests + 10 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** none (no deviation from the approved design)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| VRRP can be compiled out of `ze` | functional (build-tag test) | `TestBuildTag_VRRP_Absent` PASS (bare `ze_core`): `!Has("vrrp")` + `vrrp {}` rejected unknown; `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` PASS: `go tool nm` on the bare-`ze_core` binary shows zero `internal/plugins/vrrp` symbols |
| Default `ze` still ships VRRP | functional (build-tag test) | `TestBuildTag_VRRP_Present` PASS under `ze_core ze_vrrp` and the full default set; `ZE_FEATURES` awk now emits `ze_vrrp` |
| No new mechanism / no hand-wiring | verification | `make ze-tier-check` exit 0 with zero edits to `dep_audit.py`/`Makefile`/`internal/test/runner`; `plugin_imports.go --check` exit 0 (13 gated groups) |

## Review Gate
### Run 1 (initial)
Pre-checks: `make ze-validate` exit 0 ("all checks passed"); `audit-test-relaxation.py` exit 0 ("clean; no tests deleted or weakened", 1 changed test file examined -> coverage expanded, not weakened).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | (none) | Wiring: gated import -> registry proven by present test; no new production symbols | `all_ze_vrrp.go` | n/a |
| - | (none) | Removed-behavior: 3 `all.go` lines moved to gated file; invariant preserved; nm test coverage expanded (+vrrp, old needles intact) | `all.go`, `build_tag_protocols_absent_test.go` | n/a |
| - | (none) | Golden snapshots unchanged (vrrp still registered under default set); snapshot test passes | `internal/component/plugin/all/testdata/` | n/a |
| - | (none) | Security/allocation/RFC/perf: N/A (no protocol/runtime-input/hot-path code) | - | n/a |

**Run 1 result: 0 BLOCKER, 0 ISSUE, 0 NOTE.**

### Fixes applied
- None required (clean on first pass).

### Run 2 (INDEPENDENT reviewer -- core VRRP enrollment)
Independent subagent built bare `ze_core` and `ze_vrrp` binaries, ran the tests, ran the generators + dep_audit, regenerated the doc index. Result: **0 BLOCKER, 0 ISSUE.**
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | quickstart `go install` command omits `ze_vrrp` (hardcoded list) | `docs/guide/quickstart.md:22` | FIXED (now generated) |
| 2 | ISSUE | `ai/CODE-TO-DOCS.md` stale (missing `all_ze_vrrp.go` anchor) | `ai/CODE-TO-DOCS.md` | FIXED (`make ze-doc-index`) |
| 3 | NOTE | `scripts/dev/stress-repro.py` `race_tags` omits `ze_vrrp` | `stress-repro.py:189` | FIXED (now derives) |

### Run 3 (INDEPENDENT reviewer -- feature-tag SSOT generation)
Independent subagent verified idempotence/byte-stability, CI wiring, order-consistency, manifest parse/dedupe, and swept for a fifth consumer (none). Result: **0 BLOCKER, 0 ISSUE; 6 NOTEs.**
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `rewriteGolangci` orphans a tag if a comment interrupts the list | `feature_tags.go` | ACK -- backstopped by `dep_audit --check` (fails loudly, not silent); no in-list comments today |
| 2 | NOTE | `rewriteGokrazy` appends `],` unconditionally (bad JSON if GoBuildTags were last key) | `feature_tags.go` | ACK -- backstopped by `TestGokrazyConfigMatchesApplianceBuildTags`; key is not last |
| 3 | NOTE | `ze-regen-check` git-diff list excluded the 3 new generated files | `Makefile:451` | FIXED (added the 3 paths) |
| 4 | NOTE | changed-scope verify can skip the quickstart drift gate | (process) | ACK -- full `ze-verify` (CI) always runs it |
| 5 | NOTE | generator/test comments said "two" static consumers, code handles three | `feature_tags.go`, `_test.go` | FIXED (comments say three) |
| 6 | NOTE | `feature-gates.txt` header still said "one consumer... drift-checked" | `feature-gates.txt:16` | FIXED (header describes generation + 3 consumers) |

### Run 4 (re-runs until clean)
Not needed -- BLOCKER/ISSUE already 0; NOTEs dispositioned above (3 fixed, 3 acknowledged/backstopped).
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  <!-- satisfied: Run 1 clean; template checkbox left unticked per spec-checkbox rule -->
- [ ] All NOTEs recorded above (or explicitly "none")  <!-- none -->>

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/all/all_ze_vrrp.go` | Yes | git status `??`; contains `//go:build ze_vrrp` + 3 imports |
| `cmd/ze/hub/build_tag_vrrp_present_test.go` | Yes | git status `??` |
| `cmd/ze/hub/build_tag_vrrp_absent_test.go` | Yes | git status `??` |
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | manifest lines | `grep ze_vrrp feature-gates.txt` -> 2 lines |
| AC-2 | codegen partition | `all.go` diff = 3 removed lines; `all_ze_vrrp.go` lists vrrp+transport+yang; `--check` exit 0 |
| AC-3 | golangci tag | `.golangci.yml` has `- ze_vrrp` |
| AC-4/5/6 | present/absent/nm | full `cmd/ze/hub` package PASS both tag-ways (present set + bare core, no `-short`) |
| AC-7 | no hand-wiring | `git diff --stat scripts/dev/dep_audit.py Makefile internal/test/runner` empty; `make ze-tier-check` exit 0 |
| AC-8 | docs | `make ze-doc-test` PASSED |
### Wiring Verified (end-to-end)
| Entry Point | .ci/test File | Verified |
|-------------|---------------|----------|
| build `ze_core ze_vrrp` -> registry | `build_tag_vrrp_present_test.go` | Yes -- `Has("vrrp")` true |
| build bare `ze_core` -> registry/schema | `build_tag_vrrp_absent_test.go` | Yes -- `!Has("vrrp")`, `vrrp {}` rejected unknown |
| build bare `ze_core` -> binary symbols | `build_tag_protocols_absent_test.go` | Yes -- nm zero vrrp symbols |
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | audit grep empty; `make ze-tier-check` exit 0 |
| A-2 | confirmed | `dep_audit.py` unedited; `make ze-tier-check` exit 0 with vrrp->transport intra-feature import |
| A-3 | confirmed | `grep vrrp cmd/ze/*.go` empty; no `dispatch_vrrp.go`; absent nm test passes |
| A-4 | confirmed | `all_ze_vrrp.go` includes `vrrp/yang`; `TestBuildTag_VRRP_AbsentRejectsVRRPConfig` PASS |
| A-5 | confirmed | bare-`ze_core` nm shows zero `internal/plugins/vrrp` symbols (covers fsm/packet) |
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| features.md VRRP compile-out sentence | anchors resolve (`feature-gates.txt`, `all_ze_vrrp.go` exist) | Yes -- `make ze-doc-test` PASS |
| feature-gate-registration.md inventory | `ze_vrrp` present; note accurate vs `register.go:81` CLIHandler | Yes |
| DOCS-TO-CODE.md | regenerated; diff = 2 new vrrp test rows | Yes -- `make ze-doc-test` PASS |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests); iterate with `make ze-verify-changed`
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none unvalidated)

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No speculative features (needed NOW?)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional/build-tag tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-feature-gate-9-vrrp.md`
- [ ] **Commit A:** manifest + golangci + generated all.go/all_ze_vrrp.go + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-feature-gate-9-vrrp.md` only
