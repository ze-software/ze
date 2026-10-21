# ExaBGP Migration Guide

## TL;DR

Ze aims for **easy migration from ExaBGP**, not 100% compatibility.

| Principle | Description |
|-----------|-------------|
| **Config migration** | `ze config migrate` converts ExaBGP configs |
| **API differences OK** | Document differences, provide migration path |
| **No silent breakage** | Breaking changes must have migration or clear error |
| **Plural communities** | Ze uses `communities` (plural), ExaBGP uses `community` |

## Migration Rules

### 1. Config Files
- ExaBGP configs MUST work with `ze config migrate`
- New Ze-only features MAY use different syntax
- Deprecated ExaBGP syntax MUST be migrated automatically

### 2. API Output
- Ze uses different JSON structure for UPDATE events
- Differences documented below

### 3. API Commands
- Ze MAY extend command syntax
- Core commands SHOULD match ExaBGP
- New commands MAY use different patterns

## Key Differences

| Area | ExaBGP | Ze | Notes |
|------|--------|-------|-------|
| Keyword | `neighbor` | `peer` | Migration converts |
| API output | `community` | `communities` | Plural for consistency |
| Config syntax | `api [<name>] { processes [...] }` | `attach process <name> { }` | The name is optional and groups the settings for a reader. A peer can carry several api blocks, and ExaBGP unions them |
| Process selection | `processes-match [ <regex>... ]` | `attach process <name> { }` | The pattern is anchored at the start of the process name, as `re.match` anchors it. A pattern set that selects no process is refused, and the two lists are mutually exclusive |
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

### Ze Style (NEW)
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
2. **Matches the text event syntax:** `nlri ipv4/unicast add ... del ...`
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
   - If yes: Add to `internal/exabgp/migration/`
   - If no: Require user action with clear error

2. **Document the difference**
   - Add to this file
   - Update `docs/architecture/api/architecture.md` if API-related
   - Add to release notes

3. **Provide compatibility option (if practical)**
   - Config flag: `encoding legacy;`
   - Environment variable

## Migration Tools

### Config Migration

```bash
# Check what needs migration
ze config validate old.conf

# Preview changes
ze config migrate --dry-run old.conf

# Write the migrated config to a new file
ze config migrate -o new.conf old.conf
```
<!-- source: internal/component/config/cli/cmd_migrate.go -- cmdMigrate -->

A neighbor with `shutdown true` migrates to the peer `shutdown {}` container,
so the migrated peer stays held down as it was under ExaBGP, and `request peer
<selector> enable` releases it (`docs/guide/configuration.md`, "Holding a peer
administratively down").
<!-- source: internal/exabgp/migration/migrate.go -- copySimpleFields; internal/exabgp/migration/migrate_serialize.go -->

#### Extended next hop for an IPv4 route

ExaBGP 4.2 and 5.0 sent an IPv4 route with an IPv6 next hop without asking the
peer. RFC 8950 Section 4 forbids that: "A BGP speaker MUST only advertise the
IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker
has first ascertained via the BGP Capability Advertisement that the BGP peer
supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI
pair." ExaBGP 6.0 adds the capability at load, and the migration does the
same: for each IPv4 unicast, multicast, labelled (`nlri-mpls`) or `mpls-vpn`
route whose next hop is an IPv6 address, it adds `<family> ipv6` to the peer's
`session { capability { nexthop { ... } } }` when the config does not already
declare it, and prints one warning per family naming the first route that
needed it. The route is then sent only if the peer agrees to the capability.

A config with `capability { nexthop disable; }` turns the capability off, so
the migration refuses it and names the neighbor, the next hop and the family,
as ExaBGP 6.0 does at load.
<!-- source: internal/exabgp/migration/migrate_extended_nexthop.go -- addExtendedNextHopEntries, requireExtendedNextHop -->

### Running ExaBGP Plugins

Use `ze exabgp plugin` to run existing ExaBGP plugins with Ze:

```bash
# Command line
ze exabgp plugin /path/to/exabgp-plugin.py

# In Ze config
plugin {
    external exabgp-compat {
        run "ze exabgp plugin /path/to/plugin.py";
    }
}
```

The bridge translates bidirectionally:
- Ze JSON -> ExaBGP JSON (to plugin stdin)
- ExaBGP commands -> Ze commands (from plugin stdout)

See `internal/exabgp/` for the Go library implementation.

---

**Updated:** 2026-01-16
