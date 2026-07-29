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

These ExaBGP extensions are recognized during migration, but Ze does not
implement their behavior:

| Syntax | Result |
|--------|--------|
| `capability { multi-session; }` | Warning. Ze uses standard BGP session handling. |
| `capability { operational; }` | Warning. ExaBGP operational messages are not implemented. |
| `operational { ... }` under a peer | Warning. The block is not applied. |

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
