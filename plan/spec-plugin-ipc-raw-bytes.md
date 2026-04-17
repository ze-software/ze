# Spec: plugin-ipc-raw-bytes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fmt-0-append (completed, see plan/learned/614-fmt-0-append.md) |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/compatibility.md` -- plugin API backwards compatibility post-release
4. `pkg/plugin/rpc/types.go` -- `FilterUpdateInput.Update string` (current contract)
5. `internal/component/plugin/server/events.go` -- `formatCache` string consumers

## Task

Switch the plugin IPC payload for filter updates from the current
`FilterUpdateInput.Update string` (JSON envelope wrapping the text-form
update) to a length-prefixed raw-bytes wire format so the final
`string(scratch)` conversion at the IPC boundary (enumerated as edge 1 of
spec-fmt-0-append AC-9) can be dropped.

This is the last remaining allocation on the filter-dispatch hot path
after spec-fmt-0-append eliminated all internal `string` conversions.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/compatibility.md`
  -> Constraint: post-release, the plugin API contract is frozen. This
     change MUST be bundled with an SDK version bump and a migration note
     in the plugin authoring guide.
- [ ] `pkg/plugin/rpc/types.go` -- current IPC types
  -> Decision: `Update string` and `Raw string` are both strings today;
     raw-bytes format replaces both with `[]byte` or a single length-
     prefixed frame.
- [ ] `plan/learned/614-fmt-0-append.md` -- establishes that the
     string(scratch) boundary is the last remaining alloc.

### RFC Summaries
None -- this is an internal protocol change, not an external RFC.

**Key insights:**
- Pre-release (current state): ze has never shipped. Change freely.
- Post-release: every external plugin consuming the current JSON envelope
  must migrate. The SDK version bump is the coordination mechanism.
- Raw bytes on the wire make the filter path zero-alloc end-to-end.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` -- `FilterUpdateInput` struct, `Update string`
- [ ] `internal/component/bgp/reactor/filter_chain.go:policyFilterFunc` --
  assigns `input.Update = updateText` (string form of scratch buffer)
- [ ] `internal/component/plugin/server/events.go` -- JSON marshals the
  FilterUpdateInput to the plugin pipe
- [ ] External plugin SDKs in `pkg/plugin/sdk/` -- consumer side

**Behavior to preserve:**
- Filter semantics: accept/reject/modify actions.
- The text protocol content (names, spacing, brackets) once it IS decoded
  by the plugin; only the WIRE encapsulation changes.

**Behavior to change:**
- `FilterUpdateInput.Update` becomes `[]byte` (raw-bytes envelope).
- SDK version bumped; plugin authoring guide updated.

## Data Flow (MANDATORY)

### Entry Point
- Reactor filter dispatch (`reactor_notify.go`, `reactor_api_forward.go`,
  `peer_initial_sync.go`) -> `PolicyFilterChain` -> `policyFilterFunc` ->
  `api.CallFilterUpdate(ctx, pluginName, input)`.

### Transformation Path
1. Reactor builds UPDATE text into stack scratch (`AppendUpdateForFilter`).
2. Today: `input.Update = string(scratch)` -- allocation edge.
3. Future: `input.Update = scratch` directly; IPC framing handles the wire.
4. Plugin side: decode length-prefixed frame to `[]byte`, parse as text.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| reactor -> plugin pipe | length-prefixed raw bytes | [ ] |
| plugin -> reactor response | same envelope, `Update []byte` delta | [ ] |

### Integration Points
- SDK: `pkg/plugin/sdk/sdk_types.go` and aliases -- change type
- Plugin authoring guide in `docs/plugin-development/`
- Every external plugin in `internal/plugins/*` and
  `internal/component/bgp/plugins/*` that reads `FilterUpdateInput.Update`

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Text-mode filter plugin | -> | `rpc.FilterUpdateInput.Update []byte` | `test/plugin/prefix-filter-accept.ci` (adapted) |
| Raw-mode filter plugin | -> | `rpc.FilterUpdateInput.Raw []byte` | `test/plugin/prefix-filter-reject.ci` (adapted) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestFilterUpdateInput_RawBytes_RoundTrip` | `pkg/plugin/rpc/types_test.go` | Encode/decode preserves bytes |
| `TestFilterDispatch_ZeroAlloc` | `reactor/filter_dispatch_alloc_test.go` | End-to-end filter dispatch reports 0 allocs/op on warm scratch |

## Files to Modify

- `pkg/plugin/rpc/types.go` -- change `Update string` -> `Update []byte`
- `pkg/plugin/sdk/sdk_types.go` -- re-export updated type
- `internal/component/bgp/reactor/filter_chain.go:policyFilterFunc` --
  drop the `string(scratch)` conversion
- `internal/component/bgp/reactor/reactor_notify.go`,
  `reactor_api_forward.go`, `peer_initial_sync.go` -- pass scratch slice
  directly instead of converting
- Every plugin that reads `input.Update` -- migrate to `[]byte`
- `docs/plugin-development/*.md` -- migration note

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | n/a |
| CLI commands | [ ] No | n/a |
| Plugin SDK version bump | [ ] Yes | `pkg/plugin/sdk/version.go` |
| Plugin authoring docs | [ ] Yes | `docs/plugin-development/filters.md` |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/*.ci` adapted to raw-bytes contract |

## Implementation Steps

1. Design: pick wire framing (length-prefix vs existing JSON envelope with bytes field).
2. Bump SDK version. Update plugin authoring guide.
3. Change `FilterUpdateInput.Update` type.
4. Migrate internal plugins (`internal/component/bgp/plugins/*`).
5. Migrate reactor call sites -- drop `string(scratch)`.
6. Parity tests: existing `.ci` tests produce identical filter decisions.
7. Allocation benchmark: `BenchmarkFilterDispatch_EndToEnd` reports 0 allocs/op.
8. Verify: `make ze-verify-fast`, `make ze-race-reactor`.

## Checklist

### Goal Gates
- [ ] SDK version bumped
- [ ] Filter dispatch end-to-end reports 0 allocs/op on warm scratch
- [ ] All internal plugins migrated
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Plugin authoring guide updated with migration note

### Quality Gates
- [ ] Benchmarks show zero-alloc filter path end-to-end
- [ ] Legacy `Update string` field deleted

### Completion
- [ ] Learned summary written to `plan/learned/NNN-plugin-ipc-raw-bytes.md`
- [ ] Deferral entry in `plan/deferrals.md` closed
