# Spec: yang-enum-gated-marker-migration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Post-release engineering cleanup: migrates the remaining 76 `// enumeration: gated by TestX` annotations outside the reactor/`slogutil`/`diag` scope onto `// enumeration: yang <module>/<leaf-path>` and retires the transitional `gated by TestX` marker after `spec-yang-enum-vocabulary-agreement-gate.md` lands. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md` |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Problem.** After `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md` introduces `// enumeration: yang <module>/<leaf-path>` and migrates the reactor, `slogutil`, and `diag` (`capture_raw.go`) sites, 76 `// enumeration: gated by TestX` annotations remain across unrelated packages. `gated by TestX` checks only that a test function with that name exists, not that the test compares YANG and Go bidirectionally, and keeping both markers is a transitional hybrid (`ai/rules/no-layering.md`).

**Goal.** Migrate the remaining 76 `// enumeration: gated by TestX` annotations onto `// enumeration: yang <module>/<leaf-path>`, retire the per-package YANG value-set boilerplate tests superseded by the gate (keeping behavioral parse/round-trip assertions), and remove `gated by TestX` support from `internal/le/arch/enumeration/`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - bidirectional enum agreement gate
  → Constraint: every Go enum vocabulary mapped to a YANG enum leaf is checked bidirectionally by `./le arch enumeration check`
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`
  → Constraint: retire `gated by TestX` only once zero `// enumeration: gated by` markers remain in `internal/`, `cmd/`, and `pkg/`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/enumeration/enumeration.go` - supports `// enumeration: gated by TestX` beside `// enumeration: yang <module>/<leaf-path>` during transition

**Behavior to preserve:**
- Existing `// enumeration: exempt (...)` annotations and flat-registry checks continue working unchanged.

**Behavior to change:**
- Replace the remaining 76 `// enumeration: gated by TestX` markers with `// enumeration: yang <module>/<leaf-path>` and delete `gated by TestX` / `KindGated` support from `internal/le/arch/enumeration/`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le arch enumeration check` and `./le arch enumeration report`.

### Transformation Path
1. Scan every `// enumeration: yang <module>/<leaf-path>` site across `internal/`, `cmd/`, and `pkg/`.
2. Refuse any remaining `// enumeration: gated by` syntax once retired.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/le` → product YANG schema | `yang.DefaultLoader()` via `internal/component/plugin/all` | No |

### Integration Points
- `internal/le/arch/enumeration/` (`enumeration.go`, `report.go`, `enumeration_test.go`).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be designed | → | To be designed | To be designed |

## Files to Modify
- `internal/le/arch/enumeration/enumeration.go` - remove `gated by TestX` parsing and test-file scanning
- `internal/le/arch/enumeration/report.go` - remove `KindGated` rendering

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
