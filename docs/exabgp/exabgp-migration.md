# ExaBGP Migration Guide

## TL;DR

Ze aims for **easy migration from ExaBGP**, not 100% compatibility.

| Principle | Description |
|-----------|-------------|
| **Config migration** | `ze exabgp migrate` converts ExaBGP configs |
| **API differences OK** | Document differences, provide migration path |
| **No silent breakage** | Breaking changes must have migration or clear error |
| **Plural communities** | Ze uses `communities` (plural), ExaBGP uses `community` |

## Migration Rules

### 1. Config Files
- ExaBGP configs MUST work with `ze exabgp migrate`
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
| RD format | `65000:1` | `0:65000:1` (with type prefix) | Disambiguates Type 0 vs Type 2 |

The differences that remain between the ExaBGP bridge and ExaBGP itself are
listed in [exabgp-differences.md](exabgp-differences.md).

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
   - A config leaf
   - An environment variable

## Migration Tools

### Config Migration

```bash
# Preview what would be done
ze exabgp migrate --dry-run exabgp.conf

# Write the migrated config (to stdout)
ze exabgp migrate exabgp.conf > ze-bgp.conf

# Migrate an ExaBGP INI environment file
ze exabgp migrate --env exabgp.env
```
<!-- source: internal/plugins/exabgp/main.go -- the migrate command's flags and examples -->

A capability the migration does not translate (`multi-session`, `operational`,
or any other keyword) is dropped with one warning per peer naming each one, and
the rest of the config is converted. Ze supports neither BGP multi-session nor
the OPERATIONAL message (`ai/META-DECISIONS.md`).
<!-- source: internal/exabgp/migration/migrate_unimplemented.go -- untranslatedCapabilities, untranslatedCapabilityWarning -->

A process block's `on-exit` leaf (`keep` or `withdraw`) is carried as written,
and left unset when the ExaBGP block leaves it unset. The bridge then decides
what becomes of an exited script's routes as ExaBGP 6.0 does
(`docs/architecture/exabgp-bridge.md`).
<!-- source: internal/exabgp/migration/migrate.go -- exabgpOnExit -->

A neighbor with `shutdown true` migrates to the peer `shutdown {}` container,
so the migrated peer stays held down as it was under ExaBGP, and `request peer
<selector> enable` releases it (`docs/guide/configuration.md`, "Holding a peer
administratively down").
<!-- source: internal/exabgp/migration/migrate.go -- copySimpleFields; internal/exabgp/migration/migrate_serialize.go -->

#### Hold time and Graceful Restart time

ExaBGP offers a hold time of 180 seconds when a neighbor states none, in 4.2,
5.0 and 6.0 alike. Ze's own `timer { receive-hold-time }` defaults to 90. So the
migration writes `timer { receive-hold-time 180 }` for every neighbor that
states no `hold-time`, and the migrated peer offers what the ExaBGP one did. A
stated `hold-time` is carried as it is.

ExaBGP enables Graceful Restart without a time (a bare `graceful-restart;`, or
`graceful-restart 0;`) by advertising the neighbor's hold time as the Restart
Time, capped at 4095, the most the twelve-bit field holds. Ze's `restart-time`
defaults to 120, so the migration writes that time explicitly:
`graceful-restart 180` for a neighbor with no `hold-time`. A stated time is
carried as it is.
<!-- source: internal/exabgp/migration/migrate_defaults.go -- exabgpHoldTimeDefault, exabgpGracefulRestartTime, migratedCapabilityValue -->
<!-- source: internal/exabgp/migration/migrate.go -- copySimpleFields, migrateCapability -->

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

#### IPv4 next hop for an IPv6 route

An IPv6 route whose next hop is an IPv4 address migrates with the next hop
written IPv4-mapped, `::ffff:<ipv4>`, as ExaBGP 6.0 sends it. A VPN route is
mapped too, and the engine writes the zero RD before it. ExaBGP 4.2 and 5.0
sent the four octets, which RFC 2545 Section 3 gives no form: the Length of
Next Hop Network Address field "shall be set to 16, when only a global address
is present, or 32 if a link-local address is also included in the Next Hop
field".
<!-- source: internal/exabgp/migration/migrate_routes.go -- mapIPv6RouteIPv4NextHop -->

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

**Updated:** 2026-10-22
