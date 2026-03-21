# ExaBGP Migration

Ze provides tools for converting ExaBGP configurations and running existing ExaBGP plugins with ze as the BGP engine.

## Config Migration

Convert an ExaBGP configuration file to ze-native format:

```bash
ze exabgp migrate exabgp.conf > ze.conf
ze config check ze.conf              # Validate the result
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

### Bridge Flags

| Flag | Description |
|------|-------------|
| `--family <family>` | Address families to support (repeatable) |
| `--route-refresh` | Enable route refresh capability |
| `--add-path <mode>` | ADD-PATH mode: `receive`, `send`, or `both` |

Example with flags:

```
run "ze exabgp plugin --family ipv4/unicast --family ipv6/unicast /path/to/plugin.py"
```

## Migration Workflow

1. **Convert config:** `ze exabgp migrate old.conf > new.conf`
2. **Validate:** `ze config check new.conf`
3. **Bridge plugins:** Update `run` directives to use `ze exabgp plugin`
4. **Test:** Run ze with the new config and verify sessions establish
5. **Port plugins:** Gradually rewrite plugins to use the ze SDK directly

## When to Port Plugins

The compatibility bridge adds translation overhead. Consider porting to native ze plugins when:
- You need access to ze-specific features (RPKI events, cache commands, commit workflow)
- Performance matters (native plugins skip the translation layer)
- You want to use the Go SDK for direct in-process execution
