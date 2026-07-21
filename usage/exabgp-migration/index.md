# ExaBGP Migration

Ze provides tools for converting ExaBGP configurations and running existing ExaBGP plugins with ze as the BGP engine.
<!-- source: internal/plugins/exabgp/main.go -- Run; internal/exabgp/migration/ -- config conversion; internal/exabgp/bridge/ -- plugin bridge -->

## Config Migration

Convert an ExaBGP configuration file to ze-native format:

```bash
ze exabgp migrate exabgp.conf > ze.conf
ze config validate ze.conf              # Validate the result
```

### What Gets Converted

| ExaBGP | Ze |
|--------|-----|
| `neighbor <ip> { ... }` | `peer <ip> { ... }` |
| `local-as`, `peer-as` | `session { asn { local ... } }` / `session { asn { remote ... } }` |
| `router-id` | `session { router-id ... }` (same keyword, moved under `session`) |
| `family { ... }` | `family { ... }` |
| `capability { ... }` | `capability { ... }` |
| `static { route ... }` | Static route config or update commands |
| Template inheritance | Group-based inheritance |

### Limitations

- Complex ExaBGP features (watchdog groups, split configurations) may need manual adjustment
- Process scripts need the compatibility bridge (see below)
- Migration is one-time; the output is ze-native config

## Plugin Compatibility Bridge

Run existing ExaBGP process scripts with ze using the compatibility bridge:

```
plugin {
    external my-exabgp-plugin {
        run "ze exabgp plugin /path/to/my-plugin.py";
        encoder json;
    }
}

bgp {
    router-id 10.0.0.2;
    session { asn { local 65000; } }

    peer upstream1 {
        connection {
            remote { ip 10.0.0.1; }
            local { ip 10.0.0.2; }
        }
        session {
            asn { remote 65001; }
            family {
                ipv4/unicast {
                    prefix { maximum 10000; }
                }
            }
        }
        process my-exabgp-plugin {
            receive [ update state ];
        }
    }
}
```

### How It Works

The bridge translates bidirectionally:

| Direction | From | To |
|-----------|------|----|
| Events (to plugin) | Ze JSON events | ExaBGP JSON format |
| Commands (from plugin) | ExaBGP text commands | Ze command format |

<!-- source: internal/exabgp/bridge/bridge_event.go -- ZebgpToExabgpJSON -->
<!-- source: internal/exabgp/bridge/bridge_command.go -- ExabgpToZebgpCommand -->

When launched by ze's process manager, the bridge detects `ZE_PLUGIN_HUB_TOKEN` and
connects back via TLS using the SDK. The SDK handles the 5-stage startup protocol
and MuxConn multiplexing automatically. In standalone mode (no env var), the bridge
uses stdin/stdout with inline MuxConn framing.

After each route command, the bridge injects a `peer <addr> flush` to block until
the forward pool drains, ensuring the engine processes the route before continuing.

<!-- source: internal/plugins/exabgp/main_sdk.go -- runSDKMode TLS connect-back -->

### Bridge Flags

| Flag | Description |
|------|-------------|
| `--family <family>` | Address families to support (repeatable) |
| `--route-refresh` | Enable route refresh capability |
| `--add-path <mode>` | ADD-PATH mode: `receive`, `send`, or `both` |
<!-- source: internal/plugins/exabgp/main.go -- cmdPlugin flags -->

Example with flags:

```
run "ze exabgp plugin --family ipv4/unicast --family ipv6/unicast /path/to/plugin.py"
```

### Automatic Prefix Defaults

The migration tool adds `prefix { maximum 10000; }` to every converted address family. Ze requires per-family prefix limits (RFC 4486), and ExaBGP configs do not have them. The default of 10,000 is a conservative starting point; adjust per-peer based on expected route counts (full table peers typically need 1,000,000+).
<!-- source: internal/exabgp/migration/migrate_family.go -- convertFamilyToList -->

## Migration Workflow

1. **Convert config:** `ze exabgp migrate old.conf > new.conf`
2. **Review prefix limits:** Check `prefix { maximum ... }` values match expected route counts
3. **Validate:** `ze config validate new.conf`
4. **Bridge plugins:** Update `run` directives to use `ze exabgp plugin`
5. **Test:** Run ze with the new config and verify sessions establish
6. **Port plugins:** Gradually rewrite plugins to use the ze SDK directly

## Worked migration

Consider an ExaBGP process that receives updates from two transit sessions and
writes route commands to stdout:

```text
process inject-routes {
    run python3 /opt/scripts/inject.py;
    encoder json;
}

neighbor 10.0.0.1 {
    description "transit-a";
    router-id 10.0.0.2;
    local-address 10.0.0.2;
    local-as 65000;
    peer-as 65001;
    api {
        processes [ inject-routes ];
        receive {
            parsed;
            update;
        }
    }
}

neighbor 10.0.1.1 {
    description "transit-b";
    router-id 10.0.0.2;
    local-address 10.0.1.2;
    local-as 65000;
    peer-as 65002;
    api {
        processes [ inject-routes ];
        receive {
            parsed;
            update;
        }
    }
}
```

Convert the complete configuration, retain the original as a read-only
reference, and validate the generated Ze file:

```bash
cp exabgp.conf exabgp.conf.before-ze
ze exabgp migrate exabgp.conf > ze.conf
ze config validate ze.conf
```

Review the generated file before starting. In particular:

1. Give each peer a stable, meaningful name.
2. Confirm every local and remote address and ASN.
3. Adjust the generated per-family prefix maximum. The migration default is a
   starting point, not a full-table recommendation.
4. Confirm which peers bind the converted process.
5. Review static announcements and next-hop handling.
6. Remove capabilities that the remote peer should not negotiate.

Run the existing script through the bridge:

```text
plugin {
    external inject-routes {
        run "ze exabgp plugin python3 /opt/scripts/inject.py";
        encoder json;
    }
}
```

Bind that process only to the peers whose events it should receive. The bridge
keeps the ExaBGP JSON and text-command contract at the script boundary while Ze
uses its native event and command model internally.

Start Ze and verify each boundary rather than treating an Established session
as sufficient proof:

```console
ze start ze.conf
ze cli -c "show bgp summary"
ze cli -c "show bgp peer transit-a detail"
ze cli -c "show bgp rib status"
ze cli -c "show warnings"
ze cli -c "show errors"
```

Trigger one representative announcement and withdrawal through the existing
script. Confirm the route appears in the expected peer's outbound RIB, reaches
the adjacent router, and disappears after withdrawal.

### Cutover checklist

- The converted configuration validates.
- Every intended address family and capability is negotiated.
- Received and advertised prefix counts match the old deployment.
- Existing process scripts receive the events they depend on.
- Announcements, withdrawals, and watchdog actions behave as before.
- Warnings, errors, and operational reports contain no unexpected entries.
- The old ExaBGP configuration and binary remain available for rollback during
  the first maintenance window.

Port scripts to the native SDK one at a time after the bridge deployment is
stable. This avoids combining a routing-engine migration with an application
rewrite.

## When to Port Plugins

The compatibility bridge adds translation overhead. Consider porting to native ze plugins when:
- You need access to ze-specific features (RPKI events, cache commands, commit workflow)
- Performance matters (native plugins skip the translation layer)
- You want to use the Go SDK for direct in-process execution
<!-- source: internal/exabgp/bridge/ -- bidirectional translation; internal/exabgp/migration/ -- config converter -->
