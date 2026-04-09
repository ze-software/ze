# Spec: policy-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-08 |

## Task

Build a route policy framework for ze. Filters are specialized plugins configured under `bgp { policy { <type> <name> { settings } } }` and referenced by unique name in per-peer `filter { import/export [ <name> ... ] }`. Route redistribution controls which route sources flow per-peer via `redistribute { import/export [ ... ] }`.

Replaces the current `<plugin>:<filter>` redistribution format. Clean break (no-layering).

## Design Decisions

Resolved via `/ze-design` session 2026-04-08.

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | YANG structure | Plugins augment `bgp/policy` with `ze:filter` marked lists | Follows ze-role/ze-filter-community augment pattern. Core YANG has empty `policy` container, plugins own their schemas. |
| 2 | Name resolution timing | Parse-time in `PeersFromConfigTree` | `config-design.md` fail on unknown keys. Reactor receives validated chain, stays decoupled. |
| 3 | Default visibility | Always visible in config tree | Defaults (e.g., loop detection) auto-populated in peer filter chains. `show config` shows them. Documented in user guide. |
| 4 | Default control | `delete` on built-in = `deactivate`. `delete` on user-defined = remove. | User-friendly: single command, context-aware. `inactive:` prefix in config. `activate` to restore. |
| 5 | Loop-detection | Facade over in-process `LoopIngress` | Named filter is config only. Settings (allow-own-as, cluster-id) flow into `PeerSettings`. `LoopIngress` stays as wire-bytes filter. Zero-copy preserved. |
| 6 | `inactive:` leaf-list | Extend existing `inactive:` mechanism to leaf-list values | Currently works on containers/lists only. Required for `filter { import [ inactive:no-self-as ] }`. |
| 7 | External plugin filters | Keep working, name is just a name (no colon split) | YAGNI for external filter YANG settings. Future spec. |
| 8 | `ze:filter` extension | Yes -- marks filter type lists in YANG | Mechanical discovery, location-independent. Follows `ze:listener`/`ze:validate` pattern. |
| 9 | Config structure | `policy` = definitions, `filter` = per-peer chains, `redistribute` = route source/dest | Three separate concerns. Junos mixes source selection with filtering -- ze separates them. |
| 10 | `redistribute` scope | YANG with `ze:hidden`, BGP registers `ibgp`/`ebgp` | Schema-present for validation/autocomplete, hidden from config display. Visible when fully implemented. |
| 11 | `redistribute` validation | `ze:validate "redistribute-source"` with plugin-registered values | Follows `registered-families` pattern for autocomplete. |
| 12 | `ze:hidden` implementation | Implement properly in serializer -- hides from display, still saves | Extension exists but not enforced. Fixes existing broken uses on 2 leaves. Enables hidden redistribute. |
| 13 | `ze:ephemeral` extension | New -- not saved to config file, runtime only | Separate concern from `ze:hidden`. Schema-present for validation/autocomplete but values not persisted. |

## Required Reading

- [ ] `docs/architecture/core-design.md` -- filter pipeline architecture
- [ ] `docs/architecture/config/syntax.md` -- config parsing, YANG schema
- [ ] `docs/guide/redistribution.md` -- current filter documentation (to rewrite)
- [ ] `rfc/short/rfc4271.md` -- AS loop detection (Section 9)
- [ ] `rfc/short/rfc4456.md` -- route reflector loop detection (Section 8)

**Key insights:** see Design Decisions table above.

## Current Behavior (MANDATORY)

**Source files read:** see child specs for detailed file surveys.

Current state: `redistribution { import [ rpki:validate ] }` with `<plugin>:<filter>` format. Two independent filter chains (in-process wire-bytes, policy text-IPC). `DefaultImportFilters`/`applyOverrides` built but never populated. `ze:hidden` extension declared but not enforced.

**Behavior to preserve:** in-process filter chain (OTC, LLGR), reactor filter execution, external plugin IPC.
**Behavior to change:** `<plugin>:<filter>` format replaced by named filters. See Migration table.

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { <type> <name> { settings } } }` defines filter instances
- Config: `peer { filter { import [ <name> ... ] } }` references filters
- Config: `peer { redistribute { import [ ibgp ] } }` selects route sources

### Transformation Path
1. YANG validation -- filter/redistribute containers parsed
2. Config tree -- instances stored, names collected into registry
3. `PeersFromConfigTree` -- names resolved, defaults auto-populated, chain assembled
4. Reactor startup -- per-peer settings (allow-own-as, cluster-id) control in-process filters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | Filter names resolved to `PeerSettings.ImportFilters`/`.ExportFilters` | [ ] |
| Config -> In-process filters | Loop-detection settings in `PeerFilterInfo` | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

Wiring tests are in child specs. Umbrella tracks overall coverage:

| Entry Point | -> | Feature Code | Test | Child Spec |
|-------------|---|--------------|------|------------|
| Config with `bgp { policy { loop-detection ... } }` | -> | YANG + config parse | TBD | spec-policy-1 |
| Config with `filter { import [ name ] }` | -> | Name resolution | TBD | spec-policy-3 |
| `delete` on built-in filter | -> | `inactive:` prefix | TBD | spec-policy-3 |
| Config with `redistribute { import [ ibgp ] }` | -> | Source validation | TBD | spec-policy-4 |

## Acceptance Criteria

Umbrella ACs -- child specs define detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Named filter defined and referenced | Filter runs in peer's chain |
| AC-2 | Unknown filter name in import list | Parse-time error |
| AC-3 | Duplicate filter name across types | Parse-time error |
| AC-4 | Default filter auto-populated | Visible in config, runs by default |
| AC-5 | `delete` on built-in default | Sets `inactive:` prefix, filter skipped |
| AC-6 | `activate` on inactive built-in | Removes `inactive:` prefix, filter runs |
| AC-7 | Loop-detection with allow-own-as | Own ASN allowed N times in AS_PATH |
| AC-8 | `redistribute { import [ ibgp ] }` | Validates, autocompletes, hidden from display |
| AC-9 | `ze:hidden` container | Not shown in `show config`, saved to file |
| AC-10 | `insert first/last/before/after` on ordered leaf-list | Entry at correct position |

## 🧪 TDD Test Plan

### Unit Tests
Defined in child specs.

| Test | File | Validates | Status |
|------|------|-----------|--------|

### Functional Tests
Defined in child specs.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify

See child specs for detailed file lists. Key files across the effort:

- `internal/component/config/yang/modules/ze-extensions.yang` -- `ze:filter`, `ze:ephemeral` extensions
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- `policy` and `filter` containers, `redistribute` with `ze:hidden`
- `internal/component/bgp/config/redistribution.go` -- replace `validateFilterRefs`, remove `DefaultImportFilters`/`applyOverrides`
- `internal/component/bgp/config/peers.go` -- name resolution, default auto-population
- `internal/component/bgp/reactor/filter/loop.go` -- allow-own-as, cluster-id
- `internal/component/config/serialize.go` -- `ze:hidden` enforcement

## Files to Create

See child specs. Key new files:

- `internal/component/bgp/config/filter_registry.go` -- named filter registry
- `internal/component/bgp/reactor/filter/loop_config.go` -- loop-detection YANG/config
- `internal/component/bgp/redistribute/registry.go` -- redistribute source registry

## Implementation Steps

Implementation via child specs in order: spec-policy-1 then spec-policy-2+3 (parallel) then spec-policy-4.

## Checklist

### Goal Gates (MUST pass)
- [ ] All child specs completed
- [ ] `make ze-verify` passes
- [ ] Migration complete (no `<plugin>:<filter>` format remaining)

### Completion (BLOCKING)
- [ ] All child specs have learned summaries
- [ ] Umbrella learned summary written
- [ ] Deferrals updated

## Config Model

### Filter definitions (global)

```
bgp {
    policy {
        loop-detection no-self-as {
            allow-own-as 0;
        }
        prefix-filter reject-bogons {
            ...
        }
    }
}
```

Each filter type is a YANG list under `bgp/policy`, added by plugin via `augment`. Marked with `ze:filter`. Names globally unique across all types.

### Per-peer filter chains

```
bgp {
    peer akamai {
        filter {
            import [ no-self-as reject-bogons ];
            export [ strip-internal ];
        }
    }
}
```

Order is user-controlled (execution order = config order). `ordered-by user` in YANG. CLI supports `insert first/last/before/after` and `delete`.

Built-in defaults (like `no-self-as`) auto-populate in the chain. `delete` on a built-in sets `inactive:` prefix instead of removing. `delete` on a user-defined entry removes it.

### Per-peer redistribution (route source/dest)

```
bgp {
    peer akamai {
        redistribute {
            import [ ibgp ];
            export [ ebgp ];
        }
    }
}
```

Hidden via `ze:hidden` -- schema-present for validation and autocomplete, not shown in config output. BGP registers `ibgp`/`ebgp` via `ze:validate "redistribute-source"`. Becomes visible when redistribution is fully implemented.

### Inactive entries

```
bgp {
    peer special {
        filter {
            import [ inactive:no-self-as reject-bogons ];
        }
    }
}
```

`inactive:` prefix on leaf-list values. Extension of existing `inactive:` mechanism (currently containers/lists only). Filter skipped at execution time.

## Spec Set

| Spec | Scope | Depends |
|------|-------|---------|
| `spec-policy-0-umbrella` (this file) | Design decisions, overall architecture | - |
| `spec-policy-1-framework` | `ze:filter` extension, `bgp { policy { } }` container, filter name registry, name validation in `PeersFromConfigTree`, `ze:hidden` serializer enforcement | - |
| `spec-policy-2-loop-detection` | Loop-detection filter type YANG, allow-own-as, cluster-id, facade over `LoopIngress`, default auto-population | spec-policy-1 |
| `spec-policy-3-filter-chain` | Per-peer `filter { import/export }`, `ordered-by user` leaf-lists, `inactive:` on leaf-list values, `insert first/last/before/after`, `delete` (deactivate for built-ins), CLI commands | spec-policy-1 |
| `spec-policy-4-redistribute` | `redistribute { import/export }` with `ze:hidden`, source registry, BGP `ibgp`/`ebgp` registration, `ze:validate "redistribute-source"` | spec-policy-1 |

Execution order: 1 then 2+3 (can parallel) then 4.

## Migration

The current `redistribution { import [ rpki:validate ] }` format and associated code is deleted (no-layering):

| Delete | Replacement |
|--------|-------------|
| `redistribution` YANG container (bgp/group/peer levels) | `filter { import/export }` per-peer + `redistribute` hidden |
| `validateFilterRefs` colon-format check | Name-exists-in-registry check |
| `DefaultImportFilters`/`DefaultExportFilters` package vars | Default auto-population in chain assembly |
| `applyOverrides` function | User controls chain directly, `delete` = deactivate for built-ins |
| `FilterRegistration.Overrides` field | Not needed -- user manages list |
| Colon-split in `PolicyFilterChain` | Name-based lookup |
| 6 existing redistribution .ci tests | Adapt to new format |

## Future Filter Types (separate specs)

| Type | Purpose |
|------|---------|
| prefix-filter | Block/allow prefix lists with exact/orlonger/range |
| prefix-length-filter | Block by prefix length range per family |
| as-path-filter | Match/reject by AS-path regex |
| community-tag | Add communities to routes |
| community-strip | Remove communities by pattern |
| local-pref | Set local-preference value |
| as-path-prepend | Prepend ASN to AS-path |
| next-hop-self | Rewrite next-hop to self |

## Deferred Items Received

| Source | Item | Destination |
|--------|------|-------------|
| spec-redistribution-filter AC-18 | Default named filter active by default | spec-policy-2-loop-detection |
| spec-redistribution-filter AC-19/20 | Override mechanism for default filters | spec-policy-3-filter-chain (delete = deactivate) |
| spec-redistribution-filter AC-21 | Mandatory filter cannot be overridden | OTC stays in-process, not in named system |
| spec-redistribution-filter | redistribution-override.ci functional test | spec-policy-3-filter-chain |
| spec-route-loop-detection | allow-own-as N configuration | spec-policy-2-loop-detection |
| spec-route-loop-detection | Explicit cluster-id configuration | spec-policy-2-loop-detection |
