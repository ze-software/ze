# Spec: Split ze-hub-conf.yang Into Per-Subsystem Schemas

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/hub/schema/ze-hub-conf.yang` - current monolithic schema
4. `internal/config/environment.go` - Environment struct and envOptions table
5. `internal/hub/config.go` - hub config parser

## Task

The `ze-hub-conf.yang` schema is a monolithic `environment {}` block containing settings for multiple subsystems (daemon, logging, TCP, BGP, API, reactor, debug, chaos). Each container should live with its owning subsystem, not in the hub.

Split the schema so each subsystem owns its own `environment` leaves, and the hub aggregates them — matching how `ze-bgp-conf.yang` already owns BGP peer/template config separately from the hub.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline
  → Constraint: File → Tree → ResolveBGPTree() → map[string]any → PeersFromTree()
- [ ] `docs/architecture/hub-architecture.md` - hub coordination role
  → Constraint: hub is orchestrator, not owner of subsystem settings
- [ ] `docs/architecture/config/environment.md` - environment variable handling
  → Constraint: ze.bgp.section.option naming convention

**Key insights:**
- Hub parses `environment {}` block today as flat key-value pairs
- Each subsystem already has its own YANG schema for non-environment config (e.g., `ze-bgp-conf.yang` for peers)
- The `Environment` struct in `config/environment.go` mirrors the YANG containers exactly

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/hub/schema/ze-hub-conf.yang` (129L) — single YANG module with containers: daemon, log, tcp, bgp, cache, api, reactor, debug
- [ ] `internal/hub/schema/embed.go` (7L) — embeds ze-hub-conf.yang as string
- [ ] `internal/hub/config.go` (316L) — parses env/plugin/generic blocks from config file
- [ ] `internal/config/environment.go` (701L) — Environment struct, envOptions table, LoadEnvironmentWithConfig()
- [ ] `internal/config/environment_test.go` — tests for environment parsing
- [ ] `internal/plugins/bgp/schema/ze-bgp-conf.yang` — BGP peer/template config (separate from environment)
- [ ] `internal/yang/modules/ze-plugin-conf.yang` — plugin process config
- [ ] `internal/config/yang_schema.go` — schema loading/aggregation
- [ ] `internal/config/editor/validator.go` — config editor validation uses schemas

**Behavior to preserve:**
- Config file syntax: `environment { daemon { pid /var/run/ze.pid; } }` unchanged
- Environment variable format: `ze.bgp.section.option` and `ze_bgp_section_option`
- LoadEnvironmentWithConfig() API — section/option/value interface
- Config editor validation and autocompletion
- ExaBGP migration compatibility for environment blocks

**Behavior to change:**
- YANG schema ownership: containers move from hub to their owning subsystem
- Hub schema becomes either empty (aggregator only) or holds only truly hub-owned settings

## Data Flow (MANDATORY)

### Entry Point
- Config file → tokenizer → `ParseHubConfig()` in `hub/config.go`
- Environment variables → `getEnv()` → `loadFromEnvStrict()`

### Transformation Path
1. Config file parsed by hub tokenizer into `HubConfig.Env` map (flat key-value)
2. `Env` map passed to `LoadEnvironmentWithConfig()` as `map[string]map[string]string`
3. `SetConfigValue()` dispatches via `envOptions` table to typed setters
4. `Environment` struct populated, passed to subsystem startup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file → Hub | Tokenizer in hub/config.go | [ ] |
| Hub → Environment struct | LoadEnvironmentWithConfig() | [ ] |
| Environment → Subsystems | Struct fields read by reactor, FSM, etc. | [ ] |
| YANG → Config editor | Schema loaded for validation/autocomplete | [ ] |

### Integration Points
- `internal/config/yang_schema.go` — loads all YANG schemas, must discover new schema locations
- `internal/config/editor/validator.go` — uses YANG for validation
- `internal/config/editor/completer.go` — uses YANG for autocomplete

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Design

### Proposed YANG Split

| Current container | Owner | New schema location | Rationale |
|-------------------|-------|---------------------|-----------|
| `daemon` | Engine/supervisor | `internal/hub/schema/ze-hub-conf.yang` (keep) | Process lifecycle is hub-owned |
| `log` | Engine/supervisor | `internal/hub/schema/ze-hub-conf.yang` (keep) | Cross-cutting, engine-owned |
| `debug` | Engine/supervisor | `internal/hub/schema/ze-hub-conf.yang` (keep) | Cross-cutting diagnostics |
| `tcp` | BGP subsystem | `internal/plugins/bgp/schema/ze-bgp-conf.yang` (extend) | TCP is BGP's transport |
| `bgp` | BGP subsystem | `internal/plugins/bgp/schema/ze-bgp-conf.yang` (extend) | BGP protocol settings |
| `cache` | BGP subsystem | `internal/plugins/bgp/schema/ze-bgp-conf.yang` (extend) | Attribute caching is BGP-specific |
| `reactor` | BGP subsystem | `internal/plugins/bgp/schema/ze-bgp-conf.yang` (extend) | Reactor tuning |
| `api` | Plugin infrastructure | `internal/yang/modules/ze-plugin-conf.yang` (extend) | Plugin communication settings |
| `chaos` | Engine/supervisor | `internal/hub/schema/ze-hub-conf.yang` (keep) | Chaos is engine-level fault injection |

### Impact on Go Code

The `Environment` struct and `envOptions` table in `config/environment.go` are **not affected** by the YANG split — they're runtime config, not schema-driven. The YANG split only affects:

1. Schema files (which `.yang` file contains which leaves)
2. Schema loading (yang_schema.go must find schemas in new locations)
3. Config editor validation/autocomplete (automatic if YANG loading is correct)

### Open Questions

1. Should `environment {}` remain a single config block in the file, or should BGP environment settings move under `bgp {}`?
   - Keeping as single block: simpler migration, less user disruption
   - Splitting in config syntax too: cleaner ownership, but breaking change
2. Should the `Environment` struct be split to match? Or keep monolithic for backward compatibility?

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with `environment { tcp { port 1179; } }` | → | YANG validation accepts tcp under bgp schema | `TestYANGValidationTCPInBGPSchema` |
| Config editor autocomplete for `environment.tcp.` | → | Completer finds tcp leaves from bgp schema | `TestEditorAutocompleteTCPFromBGPSchema` |
| `ze config check` with environment block | → | All schemas aggregated, validation passes | `test/parse/environment-split.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-hub-conf.yang` loaded | Contains only daemon, log, debug, chaos containers |
| AC-2 | `ze-bgp-conf.yang` loaded | Contains environment.tcp, environment.bgp, environment.cache, environment.reactor |
| AC-3 | `ze-plugin-conf.yang` loaded | Contains environment.api settings |
| AC-4 | Config file with all environment sections | Parses and validates correctly against aggregated schemas |
| AC-5 | Config editor autocomplete | All environment leaves still discoverable |
| AC-6 | `LoadEnvironmentWithConfig()` | Unchanged behavior — all sections still work |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestYANGHubSchemaContainers` | `internal/hub/schema_test.go` | AC-1: hub schema has only daemon/log/debug/chaos | |
| `TestYANGBGPSchemaEnvironment` | `internal/plugins/bgp/schema_test.go` | AC-2: bgp schema has tcp/bgp/cache/reactor under environment | |
| `TestYANGPluginSchemaEnvironment` | `internal/yang/modules_test.go` | AC-3: plugin schema has api under environment | |
| `TestEnvironmentLoadUnchanged` | `internal/config/environment_test.go` | AC-6: existing tests still pass | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `environment-split-validation` | `test/parse/environment-split.ci` | Config with all environment sections validates | |

## Files to Modify
- `internal/hub/schema/ze-hub-conf.yang` — remove tcp, bgp, cache, reactor, api containers
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` — add environment.tcp, environment.bgp, environment.cache, environment.reactor
- `internal/yang/modules/ze-plugin-conf.yang` — add environment.api
- `internal/config/yang_schema.go` — verify schema aggregation finds all schemas
- `internal/config/editor/validator.go` — verify validation still works with split schemas

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | YANG files above |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test | [x] | `test/parse/environment-split.ci` |

## Files to Create
- None — extending existing files

## Implementation Steps

1. **Write schema tests** for expected container ownership → Review: correct containers?
2. **Run tests** → Verify FAIL
3. **Move YANG containers** between schema files
4. **Run tests** → Verify PASS
5. **Verify config editor** still autocompletes all environment leaves
6. **Functional test** → config with all environment sections validates
7. **Verify all** → `make test-all`
8. **Critical Review** → all checks pass

### Failure Routing

| Failure | Route To |
|---------|----------|
| Schema aggregation misses containers | Fix yang_schema.go loading order |
| Editor autocomplete breaks | Fix schema discovery path |
| Environment parsing breaks | envOptions table unchanged — investigate schema-to-runtime mismatch |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
