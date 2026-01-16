# ExaBGP Migration Guide

## TL;DR

ZeBGP aims for **easy migration from ExaBGP**, not 100% compatibility.

| Principle | Description |
|-----------|-------------|
| **Config migration** | `zebgp config migrate` converts ExaBGP configs |
| **API differences OK** | Document differences, provide migration path |
| **No silent breakage** | Breaking changes must have migration or clear error |
| **Plural communities** | ZeBGP uses `communities` (plural), ExaBGP uses `community` |

## Migration Rules

### 1. Config Files
- ExaBGP configs MUST work with `zebgp config migrate`
- New ZeBGP-only features MAY use different syntax
- Deprecated ExaBGP syntax MUST be migrated automatically

### 2. API Output
- ZeBGP uses different JSON structure for UPDATE events
- Differences documented below

### 3. API Commands
- ZeBGP MAY extend command syntax
- Core commands SHOULD match ExaBGP
- New commands MAY use different patterns

## Key Differences

| Area | ExaBGP | ZeBGP | Notes |
|------|--------|-------|-------|
| Keyword | `neighbor` | `peer` | Migration converts |
| API output | `community` | `communities` | Plural for consistency |
| Config syntax | `api { processes [...] }` | `api <name> { }` | Named blocks |
| NLRI format | `announce`/`withdraw` with next-hop grouping | `<family>` array with `action: add/del` | See below |
| RD format | `65000:1` | `2:65000:1` (with type prefix) | Disambiguates Type 0 vs Type 2 |

## NLRI Format Change

### ExaBGP Style (OLD)
```json
{
  "announce": {
    "ipv4/unicast": {
      "192.0.2.1": [{"nlri": "10.0.0.0/24"}]
    }
  },
  "withdraw": {
    "ipv4/unicast": [{"nlri": "172.16.0.0/16"}]
  }
}
```

### ZeBGP Style (NEW)
```json
{
  "ipv4/unicast": [
    {
      "action": "add",
      "next-hop": "192.0.2.1",
      "nlri": ["10.0.0.0/24"]
    },
    {
      "action": "del",
      "nlri": ["172.16.0.0/16"]
    }
  ]
}
```

### Why Changed
1. **Correct semantics:** One UPDATE can have multiple next-hops for same family (IPv4 traditional + MP_REACH)
2. **Matches command syntax:** `nlri ipv4/unicast add ... del ...`
3. **Simpler parsing:** Family value is always an array
4. **Explicit action:** `add`/`del` instead of `announce`/`withdraw` keys

### Plugin Migration

Update JSON parsing to handle new structure:

```python
# OLD
for family, nhop_groups in event.get("announce", {}).items():
    for nhop, nlris in nhop_groups.items():
        for nlri in nlris:
            process(family, nhop, nlri["nlri"], "add")

# NEW
for key, ops in event.items():
    if "/" in key:  # Family key like "ipv4/unicast"
        for op in ops:
            action = op["action"]
            nhop = op.get("next-hop")
            for nlri in op["nlri"]:
                process(key, nhop, nlri, action)
```

## Adding New Differences

When introducing a breaking change:

1. **Check if migration is possible**
   - If yes: Add to `pkg/config/migration/`
   - If no: Require user action with clear error

2. **Document the difference**
   - Add to this file
   - Update `.claude/zebgp/api/ARCHITECTURE.md` if API-related
   - Add to release notes

3. **Provide compatibility option (if practical)**
   - Config flag: `encoding legacy;`
   - Environment variable

## Migration Tools

### Config Migration

```bash
# Check what needs migration
zebgp config check old.conf

# Preview changes
zebgp config migrate --dry-run old.conf

# Migrate in place
zebgp config migrate --in-place old.conf
```

### Running ExaBGP Plugins

Use `zebgp exabgp plugin` to run existing ExaBGP plugins with ZeBGP:

```bash
# Command line
zebgp exabgp plugin /path/to/exabgp-plugin.py

# In ZeBGP config
process exabgp-compat {
    run "zebgp exabgp plugin /path/to/plugin.py";
}
```

The bridge translates bidirectionally:
- ZeBGP JSON → ExaBGP JSON (to plugin stdin)
- ExaBGP commands → ZeBGP commands (from plugin stdout)

See `pkg/exabgp/` for the Go library implementation.

---

**Updated:** 2026-01-16
