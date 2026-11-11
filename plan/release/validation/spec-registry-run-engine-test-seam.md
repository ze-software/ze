# Spec: registry-run-engine-test-seam

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | The ipc subtests of two plugin-transport integration tests no longer run over IPC: their wrapped `RunEngine` lands on a registry copy, so the shipped IPC delivery path is unproven |
| RFC gaps | - |
| Depends | - |
| Phase | Defect found and traced at the producer; design owed |
| Handoff | - |
| Updated | 2026-11-11 |

## Task

The `ipc` subtests of `TestRecoveryQueuedDeliveryTransport` and
`TestRSFastReceiptRecipientExactlyOnce` drive their plugin over IPC again, and
any test that needs a wrapped `RunEngine` for one registration installs it
through a supported registry seam that honours the seal.

Since ae1aac228b, `registry.Lookup` (`internal/component/plugin/registry/registry.go`)
returns `cloneRegistration(reg)`. Both fixtures do
`reg := registry.Lookup(name); reg.RunEngine = wrapper`, which writes a private
copy. `GetInternalPluginRunner` (`internal/component/plugin/inprocess.go`) runs
the registry's own `RunEngine` with the plain bridged conn, `NewWithConn`
(`pkg/plugin/sdk/sdk.go`) finds the `rpc.Bridger`, and DirectBridge turns on:
"IPC fixture unexpectedly activated DirectBridge" and "route-server fixture did
not select its named transport".

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `registry.Snapshot` / `Restore` / `Reset` | `internal/component/plugin/registry/registry.go` | Test save and restore of the whole registry, seal state included | To decide: the new seam restores through these, or a per-registration test setter beside them |
| `registry.Lookup` clone | `registry.go` `Lookup`, `cloneRegistration` | Answers a copy that shares nothing with the registry | Kept: the seal and the clone stay; the fixtures stop writing into the copy |
| Internal runner | `internal/component/plugin/inprocess.go` `GetInternalPluginRunner` | Reads the registration and calls its `RunEngine` | Does not change unless the chosen seam lives here |

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md` ("A test that mutates the registry")
- [ ] `docs/architecture/plugin/plugin-system.md` - registration fields, the process boundary

## Current Behavior

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `Lookup` clones; `Snapshot`, `Restore`, `Reset`
- [ ] `internal/component/plugin/inprocess.go` - `GetInternalPluginRunner`
- [ ] `internal/component/plugin/process/recovery_delivery_integration_test.go` - `mode == "ipc"` block
- [ ] `internal/component/plugin/process/rs_fast_receipt_integration_test.go` - `ipc` subtest

**Behavior to change:**
- A wrapped `RunEngine` set on a `Lookup` result is never run.

## Data Flow

### Entry Point
- A test fixture that wraps one registration's `RunEngine` conn.

### Transformation Path
1. To design.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Test ↔ registry | To design: a test-only seam in `registry` | No |

### Integration Points
- `GetInternalPluginRunner` reads the registration the seam installed.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ipc subtest fixture | → | wrapped `RunEngine` through the seam | `TestRecoveryQueuedDeliveryTransport/ipc/*`, `TestRSFastReceiptRecipientExactlyOnce/ipc` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Both ipc subtests, full feature tags | The plugin runs over IPC (no DirectBridge) and the subtests pass with unchanged assertions | None |
| AC-2 | The seam used after the seal | It follows the seal rule chosen in design, and restores the registration at cleanup | None |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecoveryQueuedDeliveryTransport/ipc/*` | `internal/component/plugin/process/recovery_delivery_integration_test.go` | AC-1 | red |
| `TestRSFastReceiptRecipientExactlyOnce/ipc` | `internal/component/plugin/process/rs_fast_receipt_integration_test.go` | AC-1 | red |
| To design | `internal/component/plugin/registry/` | AC-2 | |

## Files to Modify
- To design: `internal/component/plugin/registry/registry.go`, the two test files above

## Implementation Steps
1. To design.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `test-writes-into-a-copy` | 2026-11-11 | ipc fixtures set `RunEngine` on a `registry.Lookup` clone, DirectBridge activates |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates
- [ ] Design approved
- [ ] Both ipc subtests pass over IPC under the full tag set: `./le test unit plugin` scoped to `./internal/component/plugin/process/` and `./internal/component/plugin/registry/`
