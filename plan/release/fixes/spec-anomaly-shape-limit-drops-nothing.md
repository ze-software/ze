# Spec: anomaly-shape-limit-drops-nothing

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A shipped plugin's default response does nothing while reporting the source as armed |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`anomaly-shape { mode armed }` is meant to throttle each anomalous source to a
configured rate. Its default action, `limit`, builds the term
`ip saddr X counter limit rate R` and nothing else (`buildActions`,
`internal/plugins/anomaly/shape/match.go`). The nft lowering emits the limit as
a condition only (`lowerLimit`, `internal/plugins/firewall/nft/lower_linux.go`):
`Over` is false and the term carries no verdict. A packet under the rate
matches and continues, a packet over it does not match and continues, and both
reach the chain's `policy accept` (`mkTable`, same file as `buildActions`). So
an armed source is never throttled, while `show anomaly shape` lists it as
armed and the metrics count it.

Evidence: read, not run (the journal row this spec owns). The goal: a source
armed under the `limit` action has its packets above the configured rate
dropped, and its packets at or below the rate accepted.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/anomaly/anomaly-2-shape.md` - the responder and the term it installs
  → Constraint: the responder re-registers the whole owner term set on every change, under key `anomaly-shape`
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the table registry and its owners
  → Constraint: the tables keep their `ze_` names (`ze_anomaly-shape`, `ze_anomaly-shape6`)

## Current Behavior (MANDATORY)

- [ ] `internal/plugins/anomaly/shape/match.go` - `buildActions` returns a counter and a limit with `Over` false and no verdict; `mkTable` builds an input-hook chain with `policy accept`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerLimit` emits `expr.Limit` alone, carrying `Over` as given
- [ ] `internal/plugins/flowspec-firewall/rule_chains.go` - the `-excess` term: a limit over the rate, then a drop

## Existing Mechanisms

| Mechanism | Where | How this spec uses it |
|-----------|-------|-----------------------|
| The excess-and-drop term | `internal/plugins/flowspec-firewall/rule_chains.go`, the `-excess` term: `firewall.Limit` with `Over` set, then `firewall.Drop` | Extends: the anomaly-shape `limit` action takes the same shape, so the lowering needs no change |
| `firewall.Limit` and its lowering | `internal/component/firewall/model.go` `Limit`, `internal/plugins/firewall/nft/lower_linux.go` `lowerLimit` | Reused unchanged: `Over` already lowers to `limit rate over` |
| The `drop` fallback action | `buildActions`, `ActionDrop` (`internal/plugins/anomaly/shape/config.go`) | Unchanged |

## Data Flow

### Entry Point

An `anomaly-detect` incident for a source, with `anomaly-shape { mode armed }`
and the default `action limit`.

### Transformation Path

The incident reaches the responder (`internal/plugins/anomaly/shape/responder.go`),
which builds the term set (`buildTables`, `buildSourceTerm`, `buildActions`),
registers it (`firewall.RegisterTables`) and reconciles (`firewall.ApplyAll`);
the nft backend lowers each action and the kernel holds the rule.

### Boundaries Crossed

| Boundary | How |
|----------|-----|
| anomaly-shape plugin to the firewall component | `firewall.RegisterTables` and `firewall.ApplyAll`, in process |
| firewall component to the kernel | the nft backend's netlink batch |

### Integration Points

| Point | File | Change |
|-------|------|--------|
| Term actions | `internal/plugins/anomaly/shape/match.go` `buildActions` | The limit action matches the excess and drops it |
| Lowering | `internal/plugins/firewall/nft/lower_linux.go` `lowerLimit` | None: `Over` is already lowered |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an armed source under `action limit` | → | `buildActions` through `ApplyAll` to the kernel rule | `TestAnomalyShapeLimitDropsTheExcess` (QEMU guest-root kernel test: packets above the rate from the armed source are dropped, packets below it pass) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A source armed under `action limit` at rate R sends above R | The kernel drops the packets above R and accepts the packets at or below R, and the term's counter counts the drops | Packets within R and its burst pass, because the configured action is a throttle, not a block |
| AC-2 | `buildActions` under `action limit` | The term holds a limit with `Over` set, followed by a drop | The counter stays, because `show firewall ruleset` reads it |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildActionsLimitDropsTheExcess` | `internal/plugins/anomaly/shape/match_test.go` | AC-2: the limit action is a limit over the rate, then a drop | |
| `TestAnomalyShapeLimitDropsTheExcess` | `internal/plugins/anomaly/shape/limit_integration_linux_test.go` | AC-1: the kernel rule drops the excess and passes the rest | |

## Files to Modify

- `internal/plugins/anomaly/shape/match.go` - the limit action drops the excess
- `internal/plugins/anomaly/shape/match_test.go` - AC-2
- `docs/architecture/anomaly/anomaly-2-shape.md` - the term shape the action installs

## Files to Create

- `internal/plugins/anomaly/shape/limit_integration_linux_test.go` - AC-1 on a real kernel

## Implementation Steps

1. **Phase: Wiring** -- write `TestAnomalyShapeLimitDropsTheExcess`, red against today's term.
2. **Phase: Fix** -- `buildActions` emits the excess-and-drop shape; AC-2's unit test red then green; AC-1 green.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-12 | anomaly-shape default `limit` action has no verdict and drops nothing |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test unit pkg path ./internal/plugins/anomaly/shape/`
