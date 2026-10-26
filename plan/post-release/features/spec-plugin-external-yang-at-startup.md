# Spec: plugin-external-yang-at-startup

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Compile external plugins own YANG at startup: a new supported plugin-config surface, not a current compiled-in schema defect. |
| Scope | plugin |
| Depends | spec-config-yang-compiled-at-build-time |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner goal (Thomas, 2026-10-16): "I want people to be able to write their own
plugins in Go or any other language and load them at startup".

An external plugin (any language, a separate process speaking the plugin
protocol) declares a YANG module in its registration. Today Ze stores that text
and never compiles it, so the operator cannot write config under the plugin's
own root. This spec makes Ze compile the module at startup, link it into the
schema, parse the config against the combined schema, and deliver the plugin's
section to it.

Owner ruling (Thomas, 2026-10-16): "if a user use an external plugin the
security is on him". There is no trust sandbox. Two refusals still apply:

| Refusal | Why |
|---------|-----|
| Invalid YANG, naming the plugin and the module | A module that does not compile has no schema to parse against |
| A module whose name or namespace clashes with a compiled-in module | The schema would be ambiguous |

Trust design (signing, allow-lists, sandboxing) is out of scope.

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` - Data Flow step 3 and Key Design Decisions
  → Constraint: `Link` is non-mutating (path copy to each graft target, depth at most 8). This spec links each external module onto the compiled-in value with that same `Link`; it never writes the compiled-in literals.
  → Constraint: the compiler stays linked into `ze`; AC-7 of that spec proves a start with no external plugin calls `compiler.Compile` zero times, and this spec MUST keep that true.
  → Constraint: each compiled module keeps its `Source` text, so an external module's `import` of `ze-types`, `ze-extensions` or an augment target compiles against it.
- [ ] `docs/architecture/hub-architecture.md` - startup sequence and staged plugin protocol
- [ ] `docs/plugin-development/schema.md`, `docs/plugin-development/protocol.md` - the author-facing contract this spec makes true
- [ ] `docs/architecture/config/yang-config-design.md` - loader checks, refusal texts

### RFC Summaries (Scope: protocol)
- N-A: no wire protocol; YANG 1.1 (RFC 7950) checks are the compiler's, unchanged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/startup.go` - `handleRegistration` stores `reg.PluginSchema`; `deliverConfigRPC` builds sections from the reactor's tree
- [ ] `internal/component/plugin/server/subsystem.go` - `onRegistration` stores the schema for `ze schema`
- [ ] `internal/component/config/plugin_verify.go` - `BuildPluginConfigSections`, `ExtractConfigSubtree`
- [ ] `internal/component/config/loader.go` - `LoadConfig`, `parseTreeWithYANG`
- [ ] `internal/component/plugin/inprocess.go` - `CollectPluginYANG` (internal registrations only)
- [ ] `internal/component/config/yang_schema.go` - `PluginOnlySchema` (no caller)

Verified at the producer on 2026-10-16:

| Fact | Evidence |
|------|----------|
| External plugin YANG is stored and never parsed or validated | `internal/component/plugin/server/startup.go` `handleRegistration` sets `reg.PluginSchema` from `input.Schema.YANGText`; no non-test code reads `reg.PluginSchema`. `internal/component/plugin/server/subsystem.go` `onRegistration` stores it for `ze schema`, which prints it |
| Config reaches a plugin only under compiled-in roots | `Server.deliverConfigRPC` (`startup.go`) calls `config.BuildPluginConfigSections(configTree, reg.WantsConfigRoots)` over the reactor's tree, parsed by `config.LoadConfig` with the compiled-in schema |
| `LoadConfig`'s plugin YANG is internal only | `plugin.CollectPluginYANG(cliPlugins)` reads `registry.Lookup(name).YANG` of INTERNAL registrations, named by the CLI `--plugin` list |
| A config block under an external plugin's own root is refused | `ze config validate` on a config with `plugin { external acme-monitor { ... } }` and an `acme-monitor { ... }` block prints `line 7: unknown top-level keyword: acme-monitor` and exits 1 (current tree, `go run -tags ze_core,ze_distro ./cmd/ze`, 2026-10-16; log `tmp/session/2026-10-15-1b0ec3e6-5946-4d37-8431-e1065e283551/scratch/job-ze-extyang-validate2-94b6378f.log`). The daemon parses the config before any external plugin starts, so the plugin's YANG cannot arrive in time |
| `config.PluginOnlySchema` (a two-phase parse helper) has no caller | grep of non-test code |
| A third-party Go plugin runs in process only when its package is added to the generated composition root (`internal/component/plugin/all`) and `ze` is rebuilt; there is no runtime Go plugin loading | `plugin/all/all.go` is generated by `./le repo update` |
| Otherwise a plugin runs as an external process through the SDK (`pkg/plugin/sdk`, Go) or any language speaking the protocol, configured by `plugin { external <name> { run ... } }` | `ze-plugin-conf.yang` |
| `docs/plugin-development/schema.md` claimed a plugin's own schema works; corrected 2026-10-16 to mark it planned, pointing here | this spec's Task |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start <config>` with a `plugin { external <name> { run ... } }` entry whose plugin declares a module in `DeclareRegistrationInput.Schema.YANGText`.

### Transformation Path
To be designed. Recommended shape (Q8):

1. Parse the `plugin` block alone (`PluginOnlySchema` exists for this and has no caller).
2. Start each external plugin to stage 1; its declare carries `YANGText`.
3. Compile each module with `compiler.Compile` against the compiled-in modules' `Source` closure; refuse invalid YANG and name or namespace clashes.
4. `Link` each module onto the compiled-in value (non-mutating).
5. Parse the full config against the linked schema.
6. Stage 2: deliver each plugin's section under its own root.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process to hub | Stage 1 declare over the plugin socket carries the YANG text | No |
| Hub to compiler | `compiler.Compile` of the text against the compiled-in `Source` closure | No |
| Hub to plugin | Stage 2 configure carries the section under the plugin's root | No |

### Integration Points
- `Link` from `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` (non-mutating).
- `config.PluginOnlySchema` for the first parse phase.
- `Server.deliverConfigRPC` for delivery.

## Open Owner Questions

| # | Question | Recommendation |
|---|----------|----------------|
| Q8 | Ordering: how does the plugin's YANG reach the parser before the config that uses it? | Two-phase load (Data Flow above). Alternatives: a YANG file path named in the `plugin external` block, read before start; or deferring unknown top-level sections until the plugins declare |
| Q9 | Reload: when an external plugin is added, removed or restarts with a changed module, how is the schema relinked, and what does a reader holding the old value see? | Relink from the compiled-in value with the non-mutating `Link`; a reader keeps its old value until it asks again |
| Q10 | Startup budget: compiling an external module parses its import closure (`ze-extensions`, `ze-types`, plus any augment target such as `ze-bgp-conf`), estimated 5 to 30 ms per module, unmeasured. What budget? | Measure first, then the owner sets the number |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An external plugin can reach stage 1 before the full config is parsed | Staged protocol: declare precedes configure | Two-phase load needs another carrier for the YANG (Q8 alternative) | Research phase, startup sequence read | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Starting plugins before the config is parsed changes startup order for every external plugin | Existing `test/plugin` `.ci` tests change outcome | Only plugins that declare YANG need the first phase; decided at DESIGN |
| R-2 | This spec starts before `spec-config-yang-compiled-at-build-time` lands and builds on the runtime loader, which that spec replaces | Two producers of the schema | Depends row: this spec starts after that one closes |

## Wiring Test (MANDATORY -- NOT deferrable)

Draft rows, confirmed at DESIGN:

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` with `plugin { external <name> }` whose plugin declares YANG, and a config block under its root | → | two-phase load, compile, `Link`, config delivery | `test/plugin/external-yang-own-root.ci` |
| Same, with a module that does not compile | → | compile refusal naming plugin and module | `test/plugin/external-yang-invalid-refused.ci` |
| Same, with a module named like a compiled-in module | → | clash refusal | `test/plugin/external-yang-clash-refused.ci` |

## Acceptance Criteria

Draft, confirmed at DESIGN:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An external plugin declares module `acme-monitor`; the config holds an `acme-monitor { ... }` block | `ze start` accepts the config and the plugin receives the block in its stage 2 config section under root `acme-monitor` |
| AC-2 | The declared module does not compile | Startup refuses, naming the plugin and the module and today's compiler error |
| AC-3 | The declared module's name or namespace equals a compiled-in module's | Startup refuses, naming both |
| AC-4 | A config with no external plugin | `compiler.Compile` is called zero times (the compiled-YANG spec's AC-7 stays green) |
| AC-5 | A value out of the external module's declared range | Refused with the same validation message a compiled-in leaf gives |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To name at DESIGN | `internal/component/plugin/server/` | AC-1 to AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `external-yang-own-root` | `test/plugin/external-yang-own-root.ci` | An author's plugin in any language gets its own config | |
| `external-yang-invalid-refused` | `test/plugin/external-yang-invalid-refused.ci` | A broken module is refused with a named cause | |
| `external-yang-clash-refused` | `test/plugin/external-yang-clash-refused.ci` | A clashing module is refused | |

## Files to Modify
- `internal/component/plugin/server/startup.go` - two-phase start, compile and link of declared modules (to confirm at RESEARCH)
- `internal/component/config/loader.go` - parse against the linked schema
- `cmd/ze/hub/main.go` - startup order (to confirm at RESEARCH)
- `docs/plugin-development/schema.md` - remove the "planned" marking once AC-1 holds

## Files to Create
- the three `.ci` tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No Ze module changes; the plugin brings its module |
| YANG validation constraints | Yes | The compiler's checks apply to external modules unchanged |
| YANG custom validators | N-A | An external module's `ze:validate` names no registered Go function; behavior decided at DESIGN |
| CLI commands/flags | No | - |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | The editor reads the linked schema; decided at DESIGN whether it sees external modules |
| Functional test for new RPC/API | Yes | `test/plugin/external-yang-*.ci` |
| Pipe completeness | N-A | No new output |
| Env var registration | No | - |
| Doctor check for runtime dependencies | N-A | No new path, socket or binary |
| Prometheus counters/metrics | No | - |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/features/plugins.md` |
| 2 | Config syntax changed? | No | The plugin's module defines its own syntax |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/plugin-development/schema.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes if Q8 adds a stage | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/hub-architecture.md` sections 2 and 3 |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/schema.md` acme-monitor example |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three `.ci` tests, red
   - Tests: `external-yang-own-root`, `external-yang-invalid-refused`, `external-yang-clash-refused`
   - Files: `test/plugin/`
   - Verify: each red for the reason the AC names
2. **Phase: two-phase load, compile, link** -- designed after Q8, Q9, Q10 are answered

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No trust sandbox (owner ruling 2026-10-16) | Signed modules, allow-lists | "if a user use an external plugin the security is on him" |

## Known Limitations
- In-process Go plugins still need a rebuild with the package in `internal/component/plugin/all`; runtime Go plugin loading is not in scope.

## Work Not Done (homed elsewhere)

| Item | Home |
|------|------|
| Validating leafref values ("Ze validates that referenced values exist" was also removed from `docs/plugin-development/schema.md`) | `plan/post-release/features/spec-config-yang-type-leafref.md` |

## Effort

Estimate 6 to 9 agent-days: two-phase start, clash refusal, reload relink, docs, and a `.ci` with an external SDK plugin that declares YANG and receives its own config section.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-plugin-external-yang-at-startup.md` only, in the same `./le commit create` script
