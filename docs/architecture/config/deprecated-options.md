# Configuration Syntax Changes

<!-- source: internal/component/config/cli/cmd_migrate.go -- migration implementation -->
<!-- source: internal/component/config/migration/migrate.go -- transformation registry -->

Ze has not shipped a stable configuration release yet. There is no public
deprecation lifecycle for old config versions, and Ze does not promise a
numbered generation ladder for config syntax.

Config compatibility is date-based. A config file records the schema stamp that
wrote it, and recovery chooses the newest rollback file that the running binary
can parse. For pre-release files or old examples, `ze config migrate` converts
known older shapes to the syntax expected by the current tree.

## Compatibility rules

| Case | What Ze does |
|------|--------------|
| Current config | `ze config validate <file>` checks it directly. |
| Older pre-release Ze syntax | `ze config migrate` applies named transforms, then emits current syntax. |
| ExaBGP-shaped syntax | The migrator converts supported shapes and reports unsupported extensions. |
| Newer schema stamp after downgrade | Startup tries rollback files newest-first and writes back the first one this binary can parse. |

The transform names are descriptive implementation names, not public config
version numbers. Use `ze config migrate --list` to see the current list in the
binary you are running.

## Older shapes the migrator recognizes

### Root-level `neighbor`

Older examples sometimes used root-level `neighbor` blocks:

```
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
```

Current native config puts BGP under `bgp {}` and separates transport from BGP
session state:

```
bgp {
    session { asn { local 65000; } }

    peer upstream1 {
        connection {
            remote { ip 192.0.2.1; }
        }
        session {
            asn { remote 65001; }
        }
    }
}
```

### Root-level peer globs

Older configs could use root-level peer globs for shared defaults:

```
peer * {
    hold-time 90;
}
```

Current config uses named groups and concrete peers:

```
bgp {
    group default {
        timer { receive-hold-time 90; }

        peer upstream1 {
            connection {
                remote { ip 192.0.2.1; }
            }
            session {
                asn { remote 65001; }
            }
        }
    }
}
```

### `template { neighbor }`

Older template neighbors became peer groups:

```
template {
    neighbor ibgp-rs {
        peer-as 65000;
    }
}
```

Current config expresses the same default as a BGP group:

```
bgp {
    group ibgp-rs {
        session { asn { remote 65000; } }
    }
}
```

## Unsupported ExaBGP extensions

Ze does not implement these two ExaBGP extensions, so it REFUSES a config that
asks for either one. It does not warn and continue. A migration that dropped
the capability would hand you a session that negotiates less than your ExaBGP
config asked for, and say nothing.

| Syntax | Result |
|--------|--------|
| `capability { multi-session; }` | `ze exabgp migrate` stops: `unsupported capability "multi-session": not implemented in ze` |
| `capability { operational; }` | `ze exabgp migrate` stops: `unsupported capability "operational": not implemented in ze` |
| Both in one peer | One error names both: `unsupported capabilities "multi-session", "operational": not implemented in ze` |
| `operational { ... }` under an ExaBGP neighbor | `ze exabgp migrate` stops at the parser: `line N: unknown field in neighbor: operational` |
| `capability { multi-session; }` in Ze syntax | `ze config migrate` stops at the parser: `line N: unknown field in capability: multi-session` |
| `operational { ... }` under a peer, in Ze syntax | `ze config migrate` stops at the parser: `line N: unknown field in peer: operational` |

To migrate such a config, delete the capability from the ExaBGP file first. The
sessions Ze then builds carry every other capability the file asked for.

Each extension comes from an IETF draft that expired without becoming an RFC:

| Extension | Draft | Expired | Codepoint |
|-----------|-------|---------|-----------|
| `multi-session` | draft-ietf-idr-bgp-multisession-07 | 2013-03-16 | Section 4 assigns capability code 68 |
| `operational` | draft-ietf-idr-operational-message-00 | 2012-09-01 | Section 9 requests a capability code and a BGP message type from IANA. Neither was allocated |

`operational` therefore has no assigned value to put on the wire. ExaBGP fills
both with private values of its own, capability `0xB9` and message type `0x06`.
Ze does not copy a private codepoint.

<!-- source: internal/exabgp/migration/migrate_unimplemented.go -- refuseUnimplementedCapabilities, the refusal and the two drafts -->

## Commands

```bash
# Check whether a file already matches the current binary
ze config validate config.conf

# Preview conversion without writing a new file
ze config migrate --dry-run config.conf

# Convert to stdout
ze config migrate config.conf

# Convert to a new file
ze config migrate -o config-current.conf config.conf

# List transforms in this binary
ze config migrate --list
```

See [Configuration Migration](../../config-migration.md) for command details.
