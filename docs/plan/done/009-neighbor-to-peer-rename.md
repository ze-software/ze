# Neighbor→Peer Rename + Template Restructuring

**Status:** Phase 6 Complete ✅ (All Phases Done)
**Created:** 2025-12-21
**Updated:** 2025-12-21
**Depends on:** `config-migration-system.md` (migration infrastructure)

## Implementation Progress

| Phase | Status | Commit |
|-------|--------|--------|
| Phase 1: Add New Syntax | ✅ Complete | `e0eb357` |
| Phase 2: Migration Infrastructure | ✅ Complete | - |
| Phase 3: Internal Refactoring | ✅ Complete | `04d4cb9` |
| Phase 4: Deprecate Old Syntax | ✅ Complete | `a9e4285` |
| Phase 5: Update Config Files | ✅ Complete | `0a2633f` |
| Phase 6: Remove Old Syntax | ✅ Complete | `76e5b79` |

### Phase 1 Notes

- All v3 syntax implemented and tested (15 new tests)
- V2 backward compatibility preserved
- **Known limitation:** Multiple `inherit` not yet supported (single only)
- Key fix: v3 `template.match` uses config order, NOT specificity

### Phase 2 Notes

- Created `internal/config/migration/` package
- Added `Tree.Clone()` for safe mutation during migration
- Added `Tree.GetOrCreateContainer()`, `RemoveListEntry()`, `ClearList()` helpers
- Implemented version detection: `DetectVersion()` with `hasV2Patterns()`, `hasV3Patterns()`
- Implemented `MigrateV2ToV3()` with order-preserving transforms:
  - `neighbor <IP>` → `peer <IP>`
  - `peer <glob>` (root) → `template { match <glob> }`
  - `template { neighbor <name> }` → `template { group <name> }`
- 12 tests covering detection and migration (all pass)
- Migration is idempotent and doesn't mutate original tree

---

## Overview

Unify naming by renaming `neighbor` to `peer` for BGP sessions, and restructure templates to separate named templates (`group`) from glob patterns (`match`).

---

## Current Structure

```
peer * { ... }                           # Glob pattern (root level)
peer 192.168.*.* { ... }                 # Specific glob

template {
    neighbor <name> { ... }              # Named template
}

neighbor <IP> { ... }                    # Actual BGP session
```

**Problems:**
- `peer` at root = glob pattern
- `neighbor` = both template name AND actual session
- Inconsistent terminology

---

## Target Structure

```
template {
    group <name> { ... }                 # Named template (explicit: inherit <name>;)
    match * { ... }                      # Glob pattern (implicit: IP matching)
    match 192.168.*.* { ... }            # Specific glob
}

peer <IP> { ... }                        # Actual BGP session
```

**Benefits:**
- `peer` = always an actual BGP session
- `group` = named template (BGP "peer group" concept)
- `match` = glob pattern (clear intent)
- All template-related config under `template { }`

---

## Schema Changes

### Before

```go
// internal/config/bgp.go

schema.Define("neighbor", List(TypeIP, neighborFields()...))
schema.Define("peer", List(TypeString, neighborFields()...))  // glob
schema.Define("template", Container(
    Field("neighbor", List(TypeString, neighborFields()...)),
))
```

### After

```go
schema.Define("peer", List(TypeIP, peerFields()...))          // actual session
schema.Define("template", Container(
    Field("group", List(TypeString, peerFields()...)),        // named template
    Field("match", List(TypeString, peerFields()...)),        // glob pattern
))
// Remove root-level "peer" (glob) - now under template.match
```

---

## Inheritance & Precedence

### Configuration Syntax Rules

These are **documented rules** of the configuration syntax:

#### Rule 1: Match Order is Config Order

`match` blocks inside `template { }` are applied **in the order they appear in the configuration file**, not by specificity. This allows intentional general→specific or specific→general patterns.

```
template {
    match * { hold-time 90; }           # Applied first to all
    match 192.168.*.* { hold-time 60; } # Applied second, overrides for 192.168.x.x
}
```

#### Rule 2: Multiple Inheritance with Last-Wins

Multiple `inherit` statements are allowed. Settings are applied in order, with **later values overriding earlier ones**. This enables a general→specific inheritance pattern.

```
peer 192.0.2.1 {
    inherit base-settings;    # Applied first
    inherit ibgp-defaults;    # Applied second, overrides base-settings
    inherit rr-client;        # Applied third, overrides ibgp-defaults
    hold-time 30;             # Applied last, overrides all inherited values
}
```

#### Rule 3: Match Only in Template

`match` blocks are **only valid inside `template { }`**. They automatically apply to peers whose IP matches the glob pattern. They cannot appear at root level or inside `peer { }`.

### Order of Application

1. **`template { match ... }`** - Applied in config file order to matching peers
2. **`template { group ... }`** - Applied via explicit `inherit` statements, in order listed
3. **`peer { ... }`** - Session-level settings override everything

### Example

```
template {
    # 1. Applied to ALL peers (first)
    match * {
        hold-time 90;
        rib { out { group-updates true; } }
    }

    # 2. Applied to 192.168.x.x peers (second, overrides match * for these peers)
    match 192.168.*.* {
        hold-time 60;
        rib { out { auto-commit-delay 50ms; } }
    }

    # Named groups - only applied when explicitly inherited
    group base {
        capability { asn4; }
    }

    group ibgp-rr {
        peer-as 65000;
        capability { route-refresh; }
    }
}

# Peer 192.168.1.1:
# - match * applies: hold-time=90, group-updates=true
# - match 192.168.*.* applies: hold-time=60, auto-commit-delay=50ms (hold-time overridden)
# - inherit base applies: asn4 capability
# - inherit ibgp-rr applies: peer-as=65000, route-refresh capability
# - peer-level: local-as=65000
# Final: hold-time=60, group-updates=true, auto-commit-delay=50ms, peer-as=65000, local-as=65000
peer 192.168.1.1 {
    inherit base;
    inherit ibgp-rr;
    local-as 65000;
}

# Peer 10.0.0.1:
# - match * applies: hold-time=90, group-updates=true
# - match 192.168.*.* does NOT apply (IP doesn't match)
# - No inherit statements
# - peer-level: local-as=65000, peer-as=65001
# Final: hold-time=90, group-updates=true, local-as=65000, peer-as=65001
peer 10.0.0.1 {
    local-as 65000;
    peer-as 65001;
}
```

---

## Config Migration

### Version Mapping

| Version | Format |
|---------|--------|
| v1 | ExaBGP main (RIB opts at neighbor level) |
| v2 | ZeBGP current (RIB opts in rib { } block) |
| **v3** | **This plan** (neighbor→peer, peer→template.match) |

### Migration: v2 → v3

| v2 Syntax | v3 Syntax |
|-----------|-----------|
| `neighbor <IP> { }` | `peer <IP> { }` |
| `peer * { }` | `template { match * { } }` |
| `peer 192.*.*.* { }` | `template { match 192.*.*.* { } }` |
| `template { neighbor <name> { } }` | `template { group <name> { } }` |

### Migration Implementation

```go
// internal/config/migration/v2_to_v3.go

var migrateV2ToV3 = Migration{
    From:        Version2,
    To:          Version3,
    Name:        "neighbor-to-peer",
    Description: "Rename neighbor→peer, move peer globs to template.match",
    Migrate:     doV2ToV3,
}

func doV2ToV3(tree *Tree) (*Tree, error) {
    result := tree.Clone()

    // 1. Rename "neighbor" → "peer" at root level
    for _, neighbor := range result.RemoveAll("neighbor") {
        result.Add("peer", neighbor.Key(), neighbor)
    }

    // 2. Move root "peer" (globs) → template.match
    template := result.GetOrCreate("template")
    for _, peerGlob := range result.RemoveAll("peer") {
        key := peerGlob.Key()
        // Check if it's a glob pattern (contains * or doesn't parse as IP)
        if isGlobPattern(key) {
            template.Add("match", key, peerGlob)
        }
        // Actual IPs stay as "peer" at root (already moved above)
    }

    // 3. Rename template.neighbor → template.group
    if tmpl := result.GetContainer("template"); tmpl != nil {
        for _, named := range tmpl.RemoveAll("neighbor") {
            tmpl.Add("group", named.Key(), named)
        }
    }

    return result, nil
}
```

### Detection Heuristic

```go
func hasV2Patterns(tree *Tree) bool {
    // v2: has "neighbor" at root OR "peer" glob at root
    if len(tree.FindAll("neighbor")) > 0 {
        return true
    }
    for _, p := range tree.FindAll("peer") {
        if isGlobPattern(p.Key()) {
            return true  // peer glob at root = v2
        }
    }
    // v2: template.neighbor exists
    if tmpl := tree.GetContainer("template"); tmpl != nil {
        if len(tmpl.FindAll("neighbor")) > 0 {
            return true
        }
    }
    return false
}
```

---

## Internal Code Refactoring

### Struct Renames

| Current | New |
|---------|-----|
| `NeighborConfig` | `PeerConfig` |
| `NeighborReactor` | `PeerReactor` |
| `neighborFields()` | `peerFields()` |
| `parseNeighborConfig()` | `parsePeerConfig()` |

### Files Affected

```
internal/config/bgp.go           # Schema + parsing
internal/config/bgp_test.go      # Tests
internal/reactor/reactor.go      # Reactor uses neighbor terminology
internal/reactor/peer.go         # Peer management
internal/plugin/json.go             # API output (update to use "peer")
```

### API Changes

**ZeBGP API uses `peer` keyword consistently:**

```
peer * announce route 10.0.0.0/8 next-hop self
peer 192.0.2.1 announce route 10.0.0.0/8 next-hop 1.2.3.4
peer * withdraw route 10.0.0.0/8
```

**JSON output also uses `peer`:**

```go
// internal/plugin/json.go - Updated to use "peer"
msg["peer"] = peerSection(peer)  // ZeBGP native API
```

| Context | Keyword |
|---------|---------|
| Config file | `peer` |
| API commands | `peer` |
| JSON output | `peer` |

**Note:** ExaBGP API v6 compatibility layer will be added separately (not part of this plan).

---

## Implementation Plan

### Phase 1: Add New Syntax (Backward Compatible) ✅

| # | Task | Files | Status |
|---|------|-------|--------|
| 1.1 | Add `template.group` to schema (alongside `template.neighbor`) | `internal/config/bgp.go` | ✅ |
| 1.2 | Add `template.match` to schema | `internal/config/bgp.go` | ✅ |
| 1.3 | Parse `template.match` in config order (ordered iteration) | `internal/config/bgp.go` | ✅ |
| 1.4 | Add IPv6 glob pattern matching (`2001:db8::*`) | `internal/config/bgp.go` | ✅ |
| 1.5 | Add CIDR pattern matching (`10.0.0.0/8`, `2001:db8::/32`) | `internal/config/bgp.go` | ✅ |
| 1.6 | Add group name validation (alpha, alphanum/hyphen, no trailing hyphen) | `internal/config/bgp.go` | ✅ |
| 1.7 | Reject `inherit` inside `template { }` with clear error | `internal/config/bgp.go` | ✅ |
| 1.8 | Reject `match` at root level or inside `peer { }` | `internal/config/bgp.go` | ✅ |
| 1.9 | Tests for new syntax | `internal/config/bgp_test.go` | ✅ |
| 1.10 | Tests for error cases | `internal/config/bgp_test.go` | ✅ |

**After Phase 1:** Both old and new syntax work

```
# Old (still works)
peer * { }
template { neighbor mytmpl { } }
neighbor 192.0.2.1 { }

# New (also works)
template { match * { }; group mytmpl { } }
peer 192.0.2.1 { }
```

### Phase 2: Migration Infrastructure ✅

| # | Task | Files | Status |
|---|------|-------|--------|
| 2.1 | Add Version3 constant | `internal/config/migration/version.go` | ✅ |
| 2.2 | Add v2 detection heuristics | `internal/config/migration/detect.go` | ✅ |
| 2.3 | Implement v2→v3 migration | `internal/config/migration/v2_to_v3.go` | ✅ |
| 2.4 | Tests for migration | `internal/config/migration/v2_to_v3_test.go` | ✅ |

### Phase 3: Internal Refactoring

| # | Task | Files |
|---|------|-------|
| 3.1 | Rename `NeighborConfig` → `PeerConfig` | `internal/config/bgp.go` |
| 3.2 | Rename `neighborFields()` → `peerFields()` | `internal/config/bgp.go` |
| 3.3 | Update all references | `internal/config/*.go`, `internal/reactor/*.go` |
| 3.4 | Update API JSON output to use `peer` | `internal/plugin/json.go` |
| 3.5 | Update serializer to output v3 format | `internal/config/serialize.go` |
| 3.6 | Update `ze bgp config dump` command | `cmd/ze/bgp/config_dump.go` |
| 3.7 | Update tests | `*_test.go` |

### Phase 4: Deprecate Old Syntax

| # | Task | Files |
|---|------|-------|
| 4.1 | Log warning for `neighbor` keyword | `internal/config/loader.go` |
| 4.2 | Log warning for root-level `peer` glob | `internal/config/loader.go` |
| 4.3 | Log warning for `template.neighbor` | `internal/config/loader.go` |
| 4.4 | Update deprecation docs | `docs/` |

### Phase 5: Update Config Files

| # | Task | Files |
|---|------|-------|
| 5.1 | Migrate etc/ze/bgp/*.conf | `etc/ze/bgp/` |
| 5.2 | Migrate test/data/**/*.conf | `test/data/` |
| 5.3 | Update documentation examples | `docs/`, `.claude/` |

### Phase 6: Remove Old Syntax (Future Release)

| # | Task | Files |
|---|------|-------|
| 6.1 | Remove `neighbor` from schema | `internal/config/bgp.go` |
| 6.2 | Remove root-level `peer` glob | `internal/config/bgp.go` |
| 6.3 | Remove `template.neighbor` | `internal/config/bgp.go` |
| 6.4 | Error on old syntax | `internal/config/loader.go` |

---

## Example Configs

### Before (v2)

```
peer * {
    rib { out { group-updates true; } }
}

peer 192.168.*.* {
    rib { out { auto-commit-delay 50ms; } }
}

template {
    neighbor ibgp-rr {
        peer-as 65000;
        capability { route-refresh; }
    }
}

neighbor 192.168.1.1 {
    inherit ibgp-rr;
    local-as 65000;
}

neighbor 10.0.0.1 {
    local-as 65000;
    peer-as 65001;
}
```

### After (v3)

```
template {
    match * {
        rib { out { group-updates true; } }
    }

    match 192.168.*.* {
        rib { out { auto-commit-delay 50ms; } }
    }

    group ibgp-rr {
        peer-as 65000;
        capability { route-refresh; }
    }
}

peer 192.168.1.1 {
    inherit ibgp-rr;
    local-as 65000;
}

peer 10.0.0.1 {
    local-as 65000;
    peer-as 65001;
}
```

---

## CLI Commands

```bash
# Check config version
$ ze bgp config check myconfig.conf
Config version: v2 (ZeBGP pre-restructure)
Deprecated syntax found:
  - neighbor 192.168.1.1 { } → use: peer 192.168.1.1 { }
  - peer * { } at root → use: template { match * { } }
  - template { neighbor ibgp-rr { } } → use: template { group ibgp-rr { } }

# Upgrade config
$ ze bgp config upgrade myconfig.conf
Backup created: myconfig.conf.20251221-150000.bak
Upgraded myconfig.conf from v2 to v3
Applied migrations:
  - neighbor-to-peer

# Preview upgrade
$ ze bgp config upgrade --dry-run myconfig.conf
```

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing configs | Phased rollout: add new syntax first, deprecate later |
| ExaBGP script compatibility | Separate compatibility layer (not this plan) |
| Large refactoring scope | Use sed/agents for bulk renames |
| Migration bugs | Extensive test coverage, dry-run mode |

---

## Success Criteria

1. ✅ `peer <IP>` works for BGP sessions
2. ✅ `template { group <name> { } }` works for named templates
3. ✅ `template { match <glob> { } }` works for glob patterns
4. ✅ Precedence: match (config order) → group (inherit order) → peer
5. ✅ Migration v2→v3 correctly transforms configs (order preserved)
6. ✅ API uses `peer` consistently (commands + JSON output)
7. ✅ IPv6 glob patterns work (`2001:db8::*`)
8. ✅ CIDR patterns work (`10.0.0.0/8`, `2001:db8::/32`)
9. ✅ Static routes in `match`/`group` blocks work
10. ✅ `inherit` rejected inside `template { }` with clear error
11. ✅ Group name validation enforced
12. ✅ `ze bgp config dump` outputs valid v3 format
13. ✅ All tests pass
14. ✅ All example configs updated

---

## Design Decisions

| Question | Decision |
|----------|----------|
| inherit syntax | Keep `inherit <name>;` |
| Multiple inheritance | Yes, multiple `inherit` allowed; last-wins override |
| Match location | Only in `template { }`, not at root or in `peer { }` |
| Match ordering | Config file order, not specificity-based |
| inherit in template | **Not allowed** - `inherit` only valid inside `peer { }` blocks |
| Static routes in templates | Yes, `match` and `group` can contain `static { route ... }` |
| API commands | API uses `peer` keyword: `peer * announce route ...`, `peer 1.2.3.4 announce ...` |
| Hot reload | Config changes apply on SIGHUP or `reload` API command |

---

## Match Pattern Syntax

### IPv4 Patterns

```
*              # Match all IPv4 (and IPv6)
192.*.*.*      # Match 192.0.0.0 - 192.255.255.255
192.168.*.*    # Match 192.168.0.0 - 192.168.255.255
192.168.1.*    # Match 192.168.1.0 - 192.168.1.255
*.*.*.1        # Match any IP ending in .1
```

Each octet can be `*` (wildcard) or an exact number (0-255).

### IPv6 Patterns

```
*                      # Match all (IPv4 and IPv6)
2001:db8::*            # Match 2001:db8::/32 equivalent
2001:db8:abcd::*       # Match 2001:db8:abcd::/48 equivalent
*::1                   # Match any IP ending in ::1
```

Each segment can be `*` (wildcard) or an exact hex value. Trailing `*` matches remaining segments.

### CIDR Patterns

CIDR notation is also supported:

```
10.0.0.0/8             # Match 10.0.0.0 - 10.255.255.255
192.168.1.0/24         # Match 192.168.1.0 - 192.168.1.255
2001:db8::/32          # Match 2001:db8::0 - 2001:db8:ffff:...
```

### Pattern Matching Rules

1. `*` alone matches **both** IPv4 and IPv6 addresses
2. Pattern type (IPv4 vs IPv6) is detected from format
3. IPv4 pattern only matches IPv4 peers; IPv6 pattern only matches IPv6 peers
4. Both glob patterns (`*`) and CIDR notation supported
5. `match` and `group` support identical attributes (no match-only or group-only fields)

---

## Group Name Validation

Group names must follow these rules:

```
Valid:   [a-zA-Z][a-zA-Z0-9-]*[a-zA-Z0-9]
         or single letter: [a-zA-Z]

Rules:
- Must start with a letter (a-z, A-Z)
- May contain letters, numbers, hyphens
- Must NOT end with a hyphen
- Minimum length: 1 character
```

**Valid examples:**
```
group ibgp { }
group ibgp-rr { }
group rr-client-v4 { }
group a { }
group Route-Reflector-1 { }
```

**Invalid examples:**
```
group 123 { }           # Cannot start with number
group ibgp- { }         # Cannot end with hyphen
group -ibgp { }         # Cannot start with hyphen
group ibgp_rr { }       # Underscore not allowed
group "my group" { }    # Spaces/quotes not allowed
```

---

## Error Handling

### Parse-Time Errors

| Error | Message | When |
|-------|---------|------|
| Match at root | `match blocks only valid inside template { }` | `match * { }` at root level |
| Match in peer | `match blocks not valid inside peer { }` | `peer 1.2.3.4 { match * { } }` |
| Inherit in template | `inherit only valid inside peer { }, not in template` | `template { group x { inherit y; } }` |
| Invalid group name | `invalid group name "X": must start with letter, end with letter/number` | Name validation fails |
| Duplicate group | `duplicate group name "X"` | Same name defined twice |
| Invalid match pattern | `invalid match pattern "X": ...` | Pattern syntax error |

### Load-Time Errors

| Error | Message | When |
|-------|---------|------|
| Missing group | `peer 1.2.3.4: inherit "X" references undefined group` | Group doesn't exist |
| Invalid IP | `peer "X": invalid IP address` | Peer key not valid IP |

### Warnings (Non-Fatal)

| Warning | Message | When |
|---------|---------|------|
| Unused group | `group "X" defined but never inherited` | No peer uses it |
| Overlapping match | `match patterns "A" and "B" both match peer X` | Info only, not error |
| Deprecated syntax | `"neighbor" is deprecated, use "peer"` | v2 syntax detected |

---

## Serialization

`ze bgp config dump` outputs valid v3 config format:

```bash
$ ze bgp config dump
template {
    match * {
        hold-time 90;
        rib {
            out {
                group-updates true;
            }
        }
    }

    group ibgp-rr {
        peer-as 65000;
        capability {
            route-refresh;
        }
    }
}

peer 192.168.1.1 {
    inherit ibgp-rr;
    local-as 65000;
}
```

**Serialization rules:**
- Always output v3 format (even if loaded from v2)
- Preserve config order for `match` blocks
- 4-space indentation
- Alphabetical ordering within blocks (except `match` which preserves order)

---

## Static Routes in Templates

Both `match` and `group` blocks can contain static routes:

```
template {
    # Routes announced to ALL peers
    match * {
        static {
            route 10.0.0.0/8 next-hop self;
        }
    }

    # Routes announced only to 192.168.x.x peers
    match 192.168.*.* {
        static {
            route 172.16.0.0/12 next-hop self;
        }
    }

    # Routes announced only when explicitly inherited
    group customer-routes {
        static {
            route 203.0.113.0/24 next-hop 192.0.2.1;
        }
    }
}

peer 192.168.1.1 {
    inherit customer-routes;
    local-as 65000;
    peer-as 65001;
    # Gets: 10.0.0.0/8 (match *), 172.16.0.0/12 (match 192.168.*.*),
    #       203.0.113.0/24 (inherit customer-routes)
}
```

---

## Migration Order Preservation

The v2→v3 migration **must preserve config order** for `match` blocks:

```go
func doV2ToV3(tree *Tree) (*Tree, error) {
    result := tree.Clone()

    // 1. Move root "peer" globs → template.match (PRESERVE ORDER)
    template := result.GetOrCreate("template")
    for _, peerGlob := range result.RemoveAllOrdered("peer") {
        key := peerGlob.Key()
        if isGlobPattern(key) {
            template.AddOrdered("match", key, peerGlob)  // Order-preserving add
        }
    }

    // 2. Rename "neighbor" → "peer" at root level (PRESERVE ORDER)
    for _, neighbor := range result.RemoveAllOrdered("neighbor") {
        result.AddOrdered("peer", neighbor.Key(), neighbor)
    }

    // 3. Rename template.neighbor → template.group (PRESERVE ORDER)
    if tmpl := result.GetContainer("template"); tmpl != nil {
        for _, named := range tmpl.RemoveAllOrdered("neighbor") {
            tmpl.AddOrdered("group", named.Key(), named)
        }
    }

    return result, nil
}
```

**Critical:** `RemoveAllOrdered` and `AddOrdered` maintain insertion order because `match` blocks are applied in config order.

---

## References

- Current schema: `internal/config/bgp.go`
- Migration system: `docs/plan/config-migration-system.md`
- ExaBGP config: `../src/exabgp/configuration/`
