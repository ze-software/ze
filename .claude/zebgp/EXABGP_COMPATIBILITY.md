# ExaBGP Compatibility

## TL;DR

ZeBGP aims for **easy migration from ExaBGP**, not 100% compatibility.

| Principle | Description |
|-----------|-------------|
| **Config migration** | `zebgp config migrate` converts ExaBGP configs |
| **API differences OK** | Document differences, provide migration path |
| **No silent breakage** | Breaking changes must have migration or clear error |
| **Plural communities** | ZeBGP uses `communities` (plural), ExaBGP uses `community` |

## Compatibility Rules

### 1. Config Files
- ExaBGP configs MUST work with `zebgp config migrate`
- New ZeBGP-only features MAY use different syntax
- Deprecated ExaBGP syntax MUST be migrated automatically

### 2. API Output
- ZeBGP MAY use different JSON key names
- Differences MUST be documented

### 3. API Commands
- ZeBGP MAY extend command syntax
- Core commands SHOULD match ExaBGP
- New commands MAY use different patterns

## Known Differences

| Area | ExaBGP | ZeBGP | Notes |
|------|--------|-------|-------|
| Keyword | `neighbor` | `peer` | Migration converts |
| API output | `community` | `communities` | Plural for consistency |
| Config syntax | `api { processes [...] }` | `api <name> { }` | Named blocks |
| Output format | Version 6 default | Version 7 default | Can set `version 6;` |

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
   - Version flag: `version 6;`
   - Environment: `zebgp_api_legacy=true`

## Migration Tool

```bash
# Check what needs migration
zebgp config check old.conf

# Preview changes
zebgp config migrate --dry-run old.conf

# Migrate in place
zebgp config migrate --in-place old.conf
```

---

**Created:** 2026-01-01
