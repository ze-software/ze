# Spec: bmp-receiver-bind-all-or-none

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A BMP receiver listen address that fails to bind is only logged while the other addresses serve, so start and reload report success with the receiver half-bound |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`(*BMPPlugin).applyReceiverConfig` (`internal/component/bgp/plugins/bmp/receiver.go`)
closes the listeners no longer wanted, then calls `startListenerLocked` once per
new address. `startListenerLocked` logs `bmp: listener bind failed` and returns,
and `applyReceiverConfig` returns nothing. Its three callers in `bmp.go` (the
Stage-2 configure handler, `OnConfigApply` and `OnConfigRollback`) therefore
cannot refuse: the receiver serves on the addresses that bound and the commit
or start reports success.

Make the receiver bind its wanted set all or none, as SSH now does: bind every
added address before closing any removed one, close what this apply bound on a
failure so the previous set keeps serving, and return an error that names the
address, why it failed and the fix to the config transaction, so the commit or
the start fails visibly (`ai/rules/user-facing-errors.md`).

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`
- [ ] `docs/guide/bmp.md` - the receiver listeners the operator configures
- [ ] `docs/architecture/config/transaction-protocol.md` - what an apply error does to the transaction

## Current Behavior (MANDATORY)

- [ ] `internal/component/bgp/plugins/bmp/receiver.go` - `applyReceiverConfig` closes removed listeners first, then `startListenerLocked` binds each added address alone; a bind failure is logged at Error and skipped, and nothing is returned
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - the Stage-2 handler, `OnConfigApply` and `OnConfigRollback` call `applyReceiverConfig` and never see a failure
- [ ] `internal/component/ssh/ssh.go` - `bindListeners` and `(*Server).Reconfigure`, the all-or-none shape home-cpe child 7 gave SSH (7ff7614e5f)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| SSH all-or-none bind | `bindListeners`, `serveListeners`, `(*Server).Reconfigure`, `internal/component/ssh/ssh.go` (home-cpe child 7, 7ff7614e5f) | Binds every address or none, closes the partial set on failure, binds additions before closing removals so a failed reload keeps the old set | Extends the same shape to the BMP receiver. The code stays in the BMP plugin, because a plugin never imports another component's implementation |
| BMP receiver apply | `(*BMPPlugin).applyReceiverConfig`, `startListenerLocked`, `listenerIsCurrent`, `receiver.go` | Diffs wanted against bound addresses, closes removed ones first, then binds each new one alone and logs a failure | Replaced by an apply that binds additions first, returns an error, and leaves the previous set serving on failure |
| Config transaction callbacks | `OnConfigApply`, `OnConfigRollback` and the Stage-2 handler in `bmp.go`; `docs/architecture/config/transaction-protocol.md` | The receiver apply's result never reaches the transaction | Extends: the bind error is returned through the existing callbacks, so the commit refuses and rolls back |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- `environment bmp server` entries in the config, delivered to the BMP plugin at Stage 2 or as a reload's verify and apply.

### Transformation Path

1. `parseReceiverConfig` reads the `environment` section.
2. `wantedListeners` derives the address set.
3. `applyReceiverConfig` reconciles the bound set against it (to be designed: bind additions, then close removals, all or none).

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | config-apply RPC answer carries the error | No |

### Integration Points

- The SDK config callbacks in `bmp.go`; no new registration.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| reload adding a BMP server on a port already held | → | `applyReceiverConfig` | to be designed: a `.ci` in `test/plugin/` beside `bmp-receiver-session.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Start with two BMP servers, one address already held | The plugin's configure fails with an error naming the address and the fix; no receiver listener stays bound | none |
| AC-2 | Reload adds a server whose bind fails | The commit is refused with that error; the previously bound set still accepts | none |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be designed | `internal/component/bgp/plugins/bmp/receiver_test.go` | AC-1, AC-2 | |

## Files to Modify

- `internal/component/bgp/plugins/bmp/receiver.go`
- `internal/component/bgp/plugins/bmp/bmp.go`
- `docs/guide/bmp.md`

## Implementation Steps

1. Write the failing tests, then make the apply all or none and return its error.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix
- [ ] Scoped package test: `./le job run label bmp command go test -tags ze_bgp,ze_bmp ./internal/component/bgp/plugins/bmp/`
- [ ] Functional: `./le test functional plugin` for the new `.ci`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `listener-set-partly-bound-reported-as-applied.md` | 2026-11-09 | BMP receiver `startListenerLocked` logs a failed bind and serves on the rest |
