# Spec: Daemon runtime one-off defects (2) - Config TreeFromPluginMap unordered multi-entry lists

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Fixes silent zero-peer loading when `TreeFromPluginMap` reconstructs a keyed list holding two or more entries without an `@<list>` order key |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the one remaining open `internal/component/config/` journal row routed from the umbrella.

## Task

Fix `TreeFromPluginMap` (`internal/component/config/plugin_map.go`) for unordered multi-entry keyed lists (`zero-value-as-valid-answer`, 2026-10-30):

When `TreeFromPluginMap` receives a plugin config map whose keyed list container (such as `peer`) holds two or more active map entries and no `@<key>` (`configorder.OrderKey(key)`) order array, lines 72-97 populate `item.target.lists[key]` but only populate `item.target.listOrder[key]` when `ordered` is true OR `entries == 1`. Because `GetListOrdered` reads `listOrder[key]`, any downstream consumer (including the BGP reload pipeline) sees zero entries and returns a nil error instead of the configured entries or an error.

When `!ordered` and `allMaps` is true and `entries >= 1`, collect the active map keys, sort them deterministically with `slices.Sort`, and store them in `item.target.listOrder[key]`. When `@<key>` IS present (`ordered == true`), keep its explicit operator-defined order unchanged (and keep returning an error when `@<key>` is malformed or has no keyed list).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts and registration boundaries
- [ ] `ai/doors/config-yang.md` - config pipeline, plugin maps, and transaction protocol
- [ ] `docs/architecture/config/transaction-protocol.md` - delivered config sections and reconstruction

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `internal/component/config/plugin_map.go` - `TreeFromPluginMap` (L72-97) only writes `item.target.listOrder[key]` when `ordered` is true (`@<key>` present) or `entries == 1`. For `entries >= 2` without `@<key>`, `item.target.lists[key]` contains all entries while `item.target.listOrder[key]` stays empty, so `GetListOrdered(key)` returns an empty slice with nil error.

**Behavior to preserve:**
- Lists carrying `@<key>` (`configorder.OrderKey(key)`) retain their exact declared order from `configorder.Entries`, and invalid `@<key>` values still return an error.
- Inactive member metadata (`IsInactiveMemberMetadata`) continues to be excluded from active `listOrder` entries and restored afterward by `restoreRunningMembers`.
- Single-entry unordered lists continue to populate `listOrder[key]` with that one key.

**Behavior to change:**
- Multi-entry unordered keyed lists (`allMaps && !ordered && entries >= 1`) populate `item.target.listOrder[key]` in deterministic ascending key order (`slices.Sort`) instead of leaving `listOrder[key]` empty.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin map to config tree reconstruction | `internal/component/config/plugin_map.go` `TreeFromPluginMap`; `internal/component/config/configorder/` `OrderKey`, `Entries` | Rebuilds `*config.Tree` from delivered plugin JSON maps; populates `listOrder` only when `@<key>` exists or `entries == 1` | Extends: populates sorted deterministic `listOrder` for any unordered `allMaps` keyed list with `entries >= 1` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "**Config is YANG-modeled, and `ConfigProvider` is its single authority** for the editor, the web UI, subsystems and plugins." | agrees | Repairs `TreeFromPluginMap` in place so delivered plugin maps reconstruct complete ordered lists without silently dropping entries. |
| `ai/doors/config-yang.md` | Boundaries | the door covering `internal/component/config/plugin_map.go` and `plugin_map_test.go` | "A plugin lowers its config with `Tree.ToPluginMap`, never `Tree.ToMap`" | agrees | Repairs the inverse helper `TreeFromPluginMap` so unordered multi-entry keyed lists reconstruct deterministically without silent zero-entry loss. |
| `docs/architecture/config/transaction-protocol.md` | 2. Transaction Phases | `// Design:` page of `internal/component/config/plugin_map.go` and `plugin_map_test.go` | "A config transaction has three runtime phases." | agrees | Ensures config maps reconstructed during verify, apply, or rollback never silently lose multi-entry keyed lists when `@<key>` metadata is absent. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/config/plugin_map.go` | silent (page covers IKEv2 exchange and engine decisions, not `TreeFromPluginMap` list-order reconstruction) | silent | IKEv2 config maps that carry `@<key>` keep their exact order, and unordered maps gain deterministic order instead of empty lists. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `config.TreeFromPluginMap(m map[string]any)` called by reload, PKI, BGP, and plugin config parsers reconstructing a `*config.Tree` from a plugin config map.

### Transformation Path
1. `TreeFromPluginMap` visits each `map[string]any` value in the explicit work queue.
2. When `allMaps` is true and `@<key>` is present, `configorder.Entries` fills `item.target.listOrder[key]` in declared order.
3. When `allMaps` is true and `@<key>` is absent (`entries >= 1`), active map keys (excluding `IsInactiveMemberMetadata`) are collected, sorted with `slices.Sort`, and stored in `item.target.listOrder[key]`.
4. Downstream `tree.GetListOrdered(key)` returns every active entry in deterministic order.

### Boundaries Crossed
- Stays entirely within `internal/component/config`. No upward or cross-component imports added.

### Integration Points
- `config.Tree.GetListOrdered`, `configorder.Entries`, and every plugin/BGP config consumer calling `TreeFromPluginMap`.

### Changed-Wire Census

No wire format, RPC payload, event schema, or external protocol changes in this spec. `TreeFromPluginMap` is an in-process map-to-tree helper inside `internal/component/config`.

### Architectural Verification
- [ ] Two-entry and multi-entry unordered keyed list maps reconstruct all active entries via `GetListOrdered` in deterministic sorted order
- [ ] Explicit `@<key>` order metadata still takes precedence over lexicographic sorting

## Risks & Assumptions

### Assumptions
- Callers that require operator-specified ordering (such as policy chains or ordered lists) already carry `@<key>` via `Tree.ToPluginMap` / `Tree.ToRunningMap`; deterministic lexicographic ordering applies only when `@<key>` was omitted.

### Risks
- Inactive member metadata keys must continue to be filtered out before sorting so inactive entries never leak into active `listOrder`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `TreeFromPluginMap` given a two-entry `peer` map without `@peer` | → | `TreeFromPluginMap` sorted `listOrder` fallback | `TestTreeFromPluginMapOrdersUnorderedMultiEntryList` (`internal/component/config/plugin_map_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Calling `TreeFromPluginMap` on a plugin map with two or more active keyed-list entries and absent `@<key>` order metadata (`zero-value-as-valid-answer`, 2026-10-30) | `TreeFromPluginMap` populates `listOrder[key]` in deterministic ascending key order so `GetListOrdered(key)` returns every active entry in sorted order and excludes inactive metadata entries | Inactive member metadata keys remain outside active `listOrder` and are restored through `restoreRunningMembers`, preserving inactive-config semantics |
| AC-2 | Calling `TreeFromPluginMap` on a plugin map carrying explicit `@<key>` order metadata (`zero-value-as-valid-answer`, 2026-10-30) | `TreeFromPluginMap` preserves the exact declared entry sequence from `@<key>` in `GetListOrdered(key)` and returns a non-nil error for malformed `@<key>` entries | Single-entry unordered keyed lists continue populating `listOrder[key]` with that one key, matching existing behavior |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestTreeFromPluginMapOrdersUnorderedMultiEntryList` | `internal/component/config/plugin_map_test.go` | AC-1, AC-2 |

### Functional Tests

| Test | AC |
|------|----|
| Covered by unit tests in `internal/component/config` (pure map-to-tree function) | AC-1, AC-2 |

## Files to Modify
- `internal/component/config/plugin_map.go`
- `internal/component/config/plugin_map_test.go`

## Files to Create
- None.

### Integration Checklist
- [ ] `TreeFromPluginMap` returns deterministic sorted entries for unordered multi-entry keyed lists
- [ ] Existing `@<key>` ordering and inactive-member restoration tests remain green

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/config/transaction-protocol.md` - note deterministic key-order fallback in `TreeFromPluginMap` if section documents map reconstruction

## Design presentation

### What changes
- Extends `TreeFromPluginMap` in `internal/component/config/plugin_map.go` so any unordered keyed list (`allMaps && !ordered && entries >= 1`) collects its active entry names, sorts them with `slices.Sort`, and writes `item.target.listOrder[key]`.

### Before and after
- **Before:** `TreeFromPluginMap` populated `listOrder[key]` only when `@<key>` was present or `entries == 1`; a two-entry unordered map returned a tree whose `GetListOrdered("peer")` yielded zero peers with nil error.
- **After:** An unordered multi-entry keyed list yields all active entries in deterministic sorted order; explicit `@<key>` metadata continues to preserve operator-declared order.

### Alternatives set aside
- **Refusing an unordered multi-entry keyed list with an error:** Rejected because hand-constructed Go maps in tests and external config JSON payloads without `@<key>` metadata (where order is not semantically significant, such as `peer` maps keyed by IP) would fail unnecessarily; sorting deterministically preserves both safety and compatibility while eliminating silent zero-entry loads.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Add `TestTreeFromPluginMapOrdersUnorderedMultiEntryList` in `internal/component/config/plugin_map_test.go` (covering two active entries without `@peer`, inactive metadata exclusion, and explicit `@peer` order preservation) and confirm red at HEAD.
2. Replace the `if entries == 1` branch in `TreeFromPluginMap` (`internal/component/config/plugin_map.go`) with sorted collection for any `entries >= 1` when `!ordered`.
3. Verify `go test -race ./internal/component/config/...` passes and mark the owned journal row `fixed:` in `plan/journal/zero-value-as-valid-answer.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer` | 2026-10-30 | `TreeFromPluginMap` unordered multi-entry list loads zero peers |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Child 1 and Children 3 through 7.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] Owned journal row marked `fixed:` in `plan/journal/zero-value-as-valid-answer.md`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label config-plugin-map command go test -race ./internal/component/config/...`
