# Spec: home-cpe-lab-guide-construct-coverage

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation: the home-CPE lab proves the mapping guide only for the constructs its synthetic VyOS source carries, and that source omits constructs the guide maps, so a guide row can be wrong with every lab scenario green |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `spec-home-cpe-0-umbrella` (closed). Found at the closure
of `spec-home-cpe-11-lab`, whose assumption A-2 ("a synthetic VyOS
configuration covering every umbrella construct exercises the guide") was
broken when measured.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `conversion` topology scenario (`checkConversion`,
`internal/le/interoplab/topology/check_conversion.go`) proves that the record
names every line of `test/interop-topology/cpe/vyos.conf`
(`conversionRecordCovers`), that every home-CPE scenario runs the rendered
conversion, and that Ze validates and applies it. Nothing checks the other
direction: that every construct the mapping guide
(`docs/guide/migrating-from-vyos.md`, "The mapping" and "Dropped
constructs") lists appears in the source.

Measured 2026-11-08 by grepping each guide row's construct in `vyos.conf`:
absent are `firewall group address-group` and `port-group`, `firewall
global-options ip-src-route`, `default-action reject`, `state established`,
`ip adjust-mss clamp-mss-to-pmtu`, `ipv6 adjust-mss`, `service
router-advert`, `service ntp server`, `system config-management
commit-archive` and `commit-revisions`, and the dropped rows `service https`,
`container`, `policy prefix-list`. The list is the starting point, not the
population: the check below derives it.

The goal: the synthetic source carries every construct the guide maps or
drops, the conversion and its record are redone by the model for the added
lines, and a check refuses a guide row whose construct the source does not
carry, so the inventory cannot drift again.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Conversion scenario | `internal/le/interoplab/topology/check_conversion.go` `checkConversion`, `conversionRecordCovers` | validates the converted file, starts Ze with it, and checks the record covers every source line | extends: a second clause checks the guide's constructs against the source |
| Synthetic VyOS source and record | `test/interop-topology/cpe/vyos.conf`, `conversion-record.md`, `ze-cpe.conf` | the invented CPE configuration, its record and the model's conversion | extends: lines added for the absent constructs, converted by the model |
| The mapping guide | `docs/guide/migrating-from-vyos.md`, "The mapping", "Dropped constructs" | the construct inventory | does not change, except where a newly exercised row proves wrong |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/migrating-from-vyos.md`
- [ ] `docs/architecture/testing/interop.md`, the home-CPE lab section

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/topology/check_conversion.go` - `checkConversion`, `conversionRecordCovers`
- [ ] `test/interop-topology/cpe/vyos.conf`

**Behavior to preserve:** every home-CPE lab scenario green on the converted file.

**Behavior to change:** the source covers the guide, and a check proves it.

## Data Flow (MANDATORY)

### Entry Point
- `./le test integration interop-topology conversion`

### Transformation Path
1. The checker reads the guide's construct rows and the VyOS source.
2. It refuses a row whose construct no source line carries.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| guide page ↔ lab source | text read by the checker | No |

### Integration Points
- The conversion record, and every home-CPE scenario that mounts the converted file.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-topology conversion` | → | guide coverage clause | scenario `conversion` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | every guide construct row | a `vyos.conf` line carries it, and the record maps or drops it |
| AC-2 | a guide row whose construct is removed from the source | the `conversion` scenario is red naming the row |
| AC-3 | the extended source, converted | every home-CPE lab scenario green in one run |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| guide coverage verdict | `internal/le/interoplab/topology/check_conversion_test.go` | AC-1, AC-2 from fixtures | |

## Files to Modify
- `internal/le/interoplab/topology/check_conversion.go`, `test/interop-topology/cpe/vyos.conf`, `conversion-record.md`, `ze-cpe.conf`

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |

## Implementation Steps

1. The coverage clause first, red over today's source; then the source, the model's conversion and the record.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 demonstrated
- [ ] The targeted tests pass: `./le job run label unit-pkg command go test -count=1 ./internal/le/interoplab/topology/` and `./le test integration interop-topology conversion`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
