# RIB Configuration Design

**Status:** Done
**Created:** 2025-12-21
**Completed:** 2025-12-21

---

## Problem

1. Each peer has its own Adj-RIB-Out
2. Batching config (`group-updates`, `auto-commit-delay`, `max-batch-size`) should be per-peer
3. Global defaults should use existing template inheritance
4. Need backward compatibility with existing `group-updates` at neighbor level

---

## Proposed Config Syntax

### Per-Neighbor (Primary)

```
neighbor 192.0.2.1 {
    peer-as 65001;
    rib {
        out {
            group-updates true;
            auto-commit-delay 100ms;
            max-batch-size 1000;
        }
    }
}
```

### Global Defaults via Template

```
template {
    neighbor rib-defaults {
        rib {
            out {
                group-updates true;
                auto-commit-delay 100ms;
                max-batch-size 1000;
            }
        }
    }
}

neighbor 192.0.2.1 {
    inherit rib-defaults;
    peer-as 65001;
}
```

### Backward Compatibility (Deprecated)

```
neighbor 192.0.2.1 {
    peer-as 65001;
    group-updates true;  # Maps to rib.out.group-updates (deprecated)
}
```

---

## Data Model

### NeighborConfig Changes

```go
type NeighborConfig struct {
    // ... existing fields ...

    GroupUpdates bool         // DEPRECATED: Use RIBOut.GroupUpdates
    RIBOut       RIBOutConfig // Per-neighbor outgoing RIB config

    // ... remaining fields ...
}
```

### RIBOutConfig (unchanged)

```go
type RIBOutConfig struct {
    GroupUpdates    bool          // default: true
    AutoCommitDelay time.Duration // default: 0
    MaxBatchSize    int           // default: 0 (unlimited)
}
```

---

## Parsing Logic

In `parseNeighborConfig`:

```
1. Initialize RIBOut with defaults
   nc.RIBOut = DefaultRIBOutConfig()

2. Apply template rib.out (if inherited)
   if tmpl.rib.out exists:
       merge into nc.RIBOut

3. Apply template group-updates (backward compat)
   if tmpl.group-updates exists:
       nc.RIBOut.GroupUpdates = tmpl.group-updates

4. Apply neighbor rib.out (overrides template)
   if neighbor.rib.out exists:
       merge into nc.RIBOut

5. Apply neighbor group-updates (backward compat, overrides)
   if neighbor.group-updates exists:
       nc.RIBOut.GroupUpdates = neighbor.group-updates

6. Sync legacy field
   nc.GroupUpdates = nc.RIBOut.GroupUpdates
```

---

## Implementation Steps

### Step 1: Update NeighborConfig
- Add `RIBOut RIBOutConfig` field
- Keep `GroupUpdates` for backward compat

### Step 2: Update parseNeighborConfig
- Add helper: `parseRIBOutConfig(tree *Tree) RIBOutConfig`
- Implement parsing logic with template inheritance
- Map legacy `group-updates` to `RIBOut.GroupUpdates`

### Step 3: Update Tests
- Per-neighbor `rib { out { ... } }` parsing
- Template inheritance of rib config
- Backward compat with `group-updates`
- Override: neighbor rib.out overrides template

### Step 4: Remove Global RIB from BGPConfig
- Already done: removed `RIBOut` from `BGPConfig`
- Already done: removed global `rib` from schema

---

## Test Cases

1. **Defaults**: No rib config → defaults applied
2. **Explicit per-neighbor**: `neighbor { rib { out { ... } } }`
3. **Template inheritance**: `template { neighbor X { rib { out { ... } } } }` + `inherit X`
4. **Override**: neighbor rib.out overrides template rib.out
5. **Backward compat**: `neighbor { group-updates true; }` works
6. **Mixed**: template has rib.out, neighbor has group-updates → both applied

---

## Future: rib.in

The nested structure allows future extension:

```
neighbor 192.0.2.1 {
    rib {
        out {
            group-updates true;
        }
        in {
            # future incoming RIB config
            max-routes 10000;
        }
    }
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/config/bgp.go` | Add RIBOut to NeighborConfig, update parseNeighborConfig |
| `pkg/config/bgp_test.go` | Update tests for per-neighbor rib |

---

## Migration Path

1. **Now**: Both syntaxes work, `group-updates` deprecated
2. **Phase 4 (formatter)**: `zebgp fmt` migrates `group-updates` → `rib { out { group-updates } }`
3. **Future**: Remove `GroupUpdates` field from NeighborConfig
