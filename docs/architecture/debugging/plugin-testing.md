# Plugin Testing and Debugging

Tools for testing plugin YANG schemas, config delivery, and capability injection.

## `ze plugin test`

Tests plugin configuration without starting a full BGP server.

### Usage

```bash
ze plugin test [options] <config-file>
```

### Options

| Flag | Description |
|------|-------------|
| `--plugin <name>` | Plugin to test (repeatable: `bgp-hostname`, `bgp-rib`, etc.) |
| `--schema` | Show schema fields for capability block |
| `--tree` | Show raw config tree that would be sent to plugins |
| `--json` | Show exact JSON delivery format |
| `--root <name>` | Config root to show (default: `bgp`) |
<!-- source: internal/component/plugin/cli/test_cmd.go -- cmdPluginTest, flag parsing -->

### Examples

**Verify plugin YANG schema is loaded:**
```bash
ze plugin test --plugin bgp-hostname --schema config.conf
```

Output shows schema structure including augmented fields:
```
📦 Plugin YANG modules loaded: 1
   - ze-hostname.yang

📋 Schema capability fields:
   bgp: *config.ContainerNode
   ...
       capability: *config.ContainerNode
         hostname: *config.ContainerNode    # ← Plugin augmentation
         ...
```

**Show config tree structure:**
```bash
ze plugin test --plugin bgp-hostname --tree config.conf
```

Output shows the parsed config as JSON:
```
🌳 Config tree (root=bgp):
   {
     "peer": {
       "127.0.0.1": {
         "capability": {
           "hostname": {
             "domain": "my-domain-name.com",
             "host": "my-host-name"
           }
         },
         ...
       }
     }
   }
```

**Show exact JSON delivery format:**
```bash
ze plugin test --plugin bgp-hostname --json config.conf
```

Output shows the exact line sent to plugins:
```
📤 JSON delivery for root=bgp:
   config json bgp {"peer":{"127.0.0.1":{"capability":{"hostname":{"domain":"my-domain-name.com","host":"my-host-name"}}}}}
```

### Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "unknown field in capability: hostname" | Plugin YANG not loaded | Add `--plugin bgp-hostname` |
| "root not found in tree" | Wrong root name | Check available roots in tree output |
| Empty hostname in tree | Config syntax error | Check config file syntax |
<!-- source: internal/component/plugin/cli/test_cmd.go -- cmdPluginTest, --schema/--tree/--json modes -->

## Unit Tests

### Config Delivery Tests

Location: `internal/component/plugin/server/config_test.go`

| Test | Validates |
|------|-----------|
| `TestConfigTreeStructure` | Config tree has correct nested structure for plugins |
| `TestHostnamePluginFullFlow` | Plugin parses config and registers capabilities |
| `TestParseCapabilityWithPeer` | Per-peer capability parsing works |

Run:
```bash
go test -v ./internal/component/plugin/server/... -run "TestConfigTree|TestHostname|TestParseCap"
```

### Hostname Plugin Tests

Location: `internal/component/bgp/plugins/hostname/hostname_test.go`

| Test | Validates |
|------|-----------|
| `TestExtractHostnameCapabilities` | JSON config parsing |
| `TestHostnamePluginEncode` | Wire format encoding |
| `TestExtractHostnameCapabilitiesMultiplePeers` | Per-peer config isolation |
| `TestHostnamePluginBoundary` | 255-byte length limits |
| `TestHostnamePluginYANG` | YANG schema registration |
<!-- source: internal/component/bgp/plugins/hostname/hostname_test.go -- TestExtractHostnameCapabilities, TestHostnamePluginEncode, TestHostnamePluginBoundary, TestHostnamePluginYANG -->
<!-- source: internal/component/plugin/server/config_test.go -- TestConfigTreeStructure, TestHostnamePluginCapabilityInjection -->

Run:
```bash
go test -v ./internal/component/bgp/plugins/hostname/...
```

### Capability Injection Tests

Location: `internal/component/plugin/capability_injection_test.go`

| Test | Validates |
|------|-----------|
| `TestCapabilityInjection` | Capabilities registered via API are retrievable |
| `TestCapabilityInjectionPerPeer` | Per-peer capabilities override global |
| `TestCapabilityInjectionConflict` | Duplicate capability codes rejected |
<!-- source: internal/component/plugin/capability_injection_test.go -- capability injection tests -->

Run:
```bash
go test -v ./internal/component/plugin/... -run "Capability"
```

## Debug Logging

Enable debug logging for plugin subsystems:

```bash
# All plugin server logging
export ze_log_server=debug

# Config parsing
export ze_log_config=debug

# Hostname plugin (if using --log-level flag)
ze plugin bgp-hostname --log-level debug
```

### Log Subsystems

| Variable | Subsystem |
|----------|-----------|
| `ze.log.plugin.server` | Plugin RPC server and stage protocol |
| `ze.log.plugin` | Plugin process lifecycle and event delivery |
| `ze.log.bgp.config` | Config parsing, YANG loading |
<!-- source: internal/core/slogutil/ -- Logger, PluginLogger, LazyLogger -->

## Debugging Checklist

When plugin capabilities aren't appearing in OPEN messages:

1. **Verify YANG loads:**
   ```bash
   ze plugin test --plugin bgp-hostname --schema config.conf
   ```
   Look for plugin fields in capability container.

2. **Verify config parses:**
   ```bash
   ze plugin test --plugin bgp-hostname --tree config.conf
   ```
   Check hostname values appear in tree.

3. **Verify JSON format:**
   ```bash
   ze plugin test --plugin bgp-hostname --json config.conf
   ```
   Compare with plugin's expected format.

4. **Run unit tests:**
   ```bash
   go test -v ./internal/component/bgp/plugins/hostname/...
   go test -v ./internal/component/plugin/... -run "Capability"
   ```

5. **Enable debug logging:**
   ```bash
   export ze_log_server=debug
   ze --plugin bgp-hostname config.conf
   ```
