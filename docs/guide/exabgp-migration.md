# ExaBGP Migration

Ze provides tools for converting ExaBGP configurations and running existing ExaBGP plugins with ze as the BGP engine.
<!-- source: cmd/ze/exabgp/main.go -- Run; internal/exabgp/migration/ -- config conversion; internal/exabgp/bridge/ -- plugin bridge -->

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
| `local-as`, `peer-as`, `router-id` | Same keywords in ze syntax |
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
        run "ze exabgp plugin /path/to/my-plugin.py"
        encoder json
    }
}

bgp {
    peer upstream1 {
        remote { ip 10.0.0.1; as 65001; }
        ...
        process my-exabgp-plugin {
            receive [ update state ]
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

<!-- source: cmd/ze/exabgp/main_sdk.go -- runSDKMode TLS connect-back -->

### Bridge Flags

| Flag | Description |
|------|-------------|
| `--family <family>` | Address families to support (repeatable) |
| `--route-refresh` | Enable route refresh capability |
| `--add-path <mode>` | ADD-PATH mode: `receive`, `send`, or `both` |
<!-- source: cmd/ze/exabgp/main.go -- cmdPlugin flags -->

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

## When to Port Plugins

The compatibility bridge adds translation overhead. Consider porting to native ze plugins when:
- You need access to ze-specific features (RPKI events, cache commands, commit workflow)
- Performance matters (native plugins skip the translation layer)
- You want to use the Go SDK for direct in-process execution
<!-- source: internal/exabgp/bridge/ -- bidirectional translation; internal/exabgp/migration/ -- config converter -->
