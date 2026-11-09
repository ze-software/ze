# Spec: rfc-record-derives-guest-root

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Tooling: `./le rfc record` in the guest; no shipped binary changes, and every currently tagged `/dev/ppp` unit already carries the directive |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le rfc record ... kernel <K>` runs a tagged unit in the QEMU guest as root
only when its doc comment carries `//ze:guest-root`
(`unitDeclaresGuestRoot`, `internal/le/rfc/discriminate_guest.go`; read by
`observationRunner.guestRunArgv`). Every other unit runs as the owner uid, and
`idMappings` (`internal/test/userns/userns_linux.go`) then maps only that uid
into the user namespace, so a unit that opens `/dev/ppp` (owned by real root)
fails with permission denied and its clean pass times out. 65044f40ad fixed
the one tagged unit that met it, by hand.

Eleven `userns.Enter` units that open `/dev/ppp` carry no directive today and
no RFC tag; each will fail the first record taken after it is tagged. The
journal row lists their areas.

The cleanest owner is the record tool, not a hand directive per test: derive
"runs as root in the guest" from what the unit needs (its `userns.Enter` call
plus a `/dev/ppp` open, or a declaration the `userns` helper or the PPP
channel opener exposes), so a newly tagged unit cannot miss it. Failing that,
the record must refuse up front with the cause and the directive to add,
never time out.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - how `./le rfc record` observes a tagged unit
- [ ] `ai/doors/le-tooling.md` - the tooling door

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/discriminate_guest.go` - `unitDeclaresGuestRoot` reads the doc directive; `guestRunArgv` adds the root switch only when `observationRunner.root` is set
- [ ] `internal/test/userns/userns_linux.go` - `idMappings` maps the full id range when the parent is root, and only the parent's own id otherwise, so the guest's `/dev/ppp` is reachable only from a root run (the host's `./le setup install` grant does not exist in the guest)

**Behavior to preserve:** a unit that means something only unprivileged keeps the owner uid; `requireRootInGuest` keeps refusing a root unit on the host.

**Behavior to change:** a unit that needs `/dev/ppp` in its namespace runs as root in the guest without a hand directive, or the record refuses it before running with the fix named.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | How this work treats it |
|-----------|-------|--------------------|-------------------------|
| `//ze:guest-root` and `unitDeclaresGuestRoot` | `internal/le/rfc/discriminate_guest.go` | The only input that sets `observationRunner.root` | Extended or replaced by a derived answer; no second path beside it (`ai/rules/no-layering.md`) |
| `guestRunArgv` root switch (`testqemu.KeywordRoot`) | same file | Runs the guest command under docker-lab's root switch | Kept as the effect |
| `requireRootInGuest` | same file | Refuses a root unit placed on the host | Kept; the derived answer must feed it too |
| `unitNeedsGuest` | `internal/le/rfc/discriminate_guest.go` | Derives guest placement from the unit's build context | The precedent: placement is derived, root should be too |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc record ... kernel <K>` on a tagged unit

### Transformation Path
1. `discriminate_observe.go` resolves the unit and sets `runner.root` from `unitDeclaresGuestRoot`
2. `guestRunArgv` builds the docker-lab command, with the root switch when `root` is set
3. The guest runs the unit; `userns.Enter` maps the invoking uid

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le host to QEMU guest | `le test qemu docker-lab` argv | No |

### Integration Points
- `requireRootInGuest` and `guestRunArgv` consume the root answer unchanged

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc record` on a `/dev/ppp` unit with no directive | → | root derivation in `internal/le/rfc/discriminate_guest.go` | `TestRecordRunsDevPPPUnitAsRootInGuest` (to write, `internal/le/rfc/discriminate_guest_test.go`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecordRunsDevPPPUnitAsRootInGuest` | `internal/le/rfc/discriminate_guest_test.go` | a unit calling `userns.Enter` and opening `/dev/ppp` gets the root switch in `guestRunArgv` | |

## Files to Modify
- `internal/le/rfc/discriminate_guest.go` - derive the root answer
- `docs/contributing/rfc-conformance-gates.md` - describe how root is decided

## Implementation Steps
1. Write the failing unit test above.
2. Decide the signal (static read of the unit, or a declaration the helper exposes) and implement it in place of the hand-only answer.
3. Retire the hand directives the derivation makes redundant, through `ze-test-judge` (they sit on RFC-tagged tests).

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output): `./le job run label unit-pkg command go test ./internal/le/rfc/`
- [ ] Boundary tests for all numeric inputs (N-A: no numeric input)
- [ ] Interop tests (N-A: tooling, no protocol peer)
- [ ] One tagged `/dev/ppp` unit recorded through `./le rfc record ... kernel <K>` with no directive; the guest run it drives is `./le test qemu docker-lab kernel <K> root command ...` (no `./le test unit` group covers `internal/le`, hence the scoped `go test` above)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population` | 2026-11-09 | `./le rfc record` runs a `/dev/ppp` unit as the owner uid unless it carries `//ze:guest-root` |
