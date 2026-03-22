# Plugin CLI Modes

Plugins have three distinct operating modes with different input/output formats.

## Modes Overview

| Mode | Invocation | Input Format | Use Case |
|------|------------|--------------|----------|
| **CLI Mode** | `ze plugin <name> --nlri <hex>` | Flag value or `-` for stdin | Direct user invocation |
| **Engine Decode Mode** | `ze plugin <name> --decode` | Protocol commands on stdin | Engine decode delegation |
| **Engine Mode** | `ze plugin <name>` | Full API protocol (stdin) | Engine-plugin communication |
<!-- source: cmd/ze/bgp/cmd_plugin.go -- plugin CLI dispatch -->

## Design Principle

**CLI mode is for humans. Engine mode is for machines.**

- CLI mode: Simple, direct input. No protocol framing.
- Engine decode mode: Protocol commands for decode delegation.
- Engine mode: Full structured protocol for engine-plugin IPC.

## Plugin Features

Plugins declare what decode features they support via `--features`:

```bash
ze plugin evpn --features
# Output: nlri

ze plugin hostname --features
# Output: capa yang

ze plugin gr --features
# Output: capa

ze plugin rib --features
# Output: (empty)
```

### Feature Matrix

| Plugin | --capa | --nlri | Notes |
|--------|--------|--------|-------|
| hostname | ✓ | ✗ | FQDN capability (code 73) |
| gr | ✓ | ✗ | Graceful Restart (code 64) |
| evpn | ✗ | ✓ | l2vpn/evpn NLRI |
| flowspec | ✗ | ✓ | ipv4/flow, ipv6/flow NLRI |
| rib | ✗ | ✗ | No decode features |
| rr | ✗ | ✗ | No decode features |
<!-- source: internal/component/plugin/registry/ -- plugin registration, Features field -->

### Feature Names

- `nlri` - Decode NLRI from UPDATE message (`--nlri <hex>`)
- `capa` - Decode capability from OPEN message (`--capa <hex>`)
- `yang` - Output YANG schema (`--yang`)

## CLI Mode (`--nlri` / `--capa`)

For direct command-line use. Takes raw hex input, outputs decoded result.

### NLRI Plugins (evpn, flowspec)

```bash
# JSON output (default)
ze plugin evpn --nlri 02210001252C37370001...

# Text output
ze plugin evpn --nlri 02210001252C37370001... --text

# From stdin
echo "02210001252C..." | ze plugin evpn --nlri -

# With family context (flowspec)
ze plugin flowspec --nlri 0718... --family ipv4/flow
```

### Capability Plugins (hostname, gr)

```bash
# JSON output (default)
ze plugin hostname --capa 07726f7574657231...

# Text output
ze plugin hostname --capa 07726f7574657231... --text

# From stdin
echo "07726f7574657231..." | ze plugin hostname --capa -
```

### Unsupported Features

Standard error when requesting unsupported decode type:

```bash
ze plugin hostname --nlri 02210001252C...
# stderr: error: plugin 'hostname' does not support --nlri (available: --capa)
# exit code: 1

ze plugin evpn --capa 07726f7574657231...
# stderr: error: plugin 'evpn' does not support --capa (available: --nlri)
# exit code: 1
```

### Output Formats

**JSON (default):**
```json
[
  {
    "code": 2,
    "parsed": true,
    "name": "MAC/IP advertisement",
    ...
  }
]
```

**Text (`--text` flag):**
```
MAC/IP advertisement rd=1:37.44.55.55:1 mac=FC:15:B4:78:7B:8F
```

### CLI Flags

| Flag | Type | Description |
|------|------|-------------|
| `--nlri <hex\|->` | string | Decode NLRI, output JSON (use `-` for stdin) |
| `--capa <hex\|->` | string | Decode capability, output JSON (use `-` for stdin) |
| `--text` | bool | Output human-readable text instead of JSON |
| `--family <fam>` | string | Address family context (flowspec only) |
| `--features` | bool | List supported decode features |
| `--yang` | bool | Output YANG schema |
<!-- source: cmd/ze/bgp/cmd_plugin.go -- CLI flag parsing, --nlri, --capa, --text -->

## Engine Decode Mode (`--decode` flag)

For engine decode delegation. Engine calls plugin with `--decode` flag and sends
protocol commands on stdin like `decode nlri l2vpn/evpn <hex>`.

### Invocation

```bash
# Started by engine's decode.go
ze plugin evpn --decode
# Then receives: decode nlri l2vpn/evpn <hex>
# Responds: decoded json [...]
```

## Engine Mode (no flags, no args)

For engine-plugin communication. Uses structured protocol on stdin/stdout.

### Invocation

```bash
# Started by engine
ze plugin evpn
```

### Protocol

Uses the plugin API protocol with line-based commands:

```
# Engine sends request
decode nlri l2vpn/evpn 02210001252C...

# Plugin responds
decoded json [{"code":2,"parsed":true,...}]
```

See `docs/architecture/api/plugin-protocol.md` for full protocol.

## Why Three Modes?

### CLI Mode Benefits

- **Simple invocation**: No need to know the API protocol
- **Pipeline friendly**: Works with standard Unix tools
- **Self-documenting**: `--help` shows all options
- **Discoverable**: `--features` shows what plugin can do

### Engine Decode Mode Benefits

- **Stateless delegation**: Engine delegates decode to plugin
- **Protocol-based**: Consistent request/response format

### Engine Mode Benefits

- **Multiplexed**: Single process handles multiple requests
- **Stateful**: Can maintain state across requests
- **Bidirectional**: Engine can send config, receive events
- **Lifecycle managed**: Engine handles respawn, backpressure
<!-- source: internal/component/plugin/server/ -- plugin process management -->

## Implementation Pattern

Plugins implement using type-specific flags (`--nlri` or `--capa`):

```go
func cmdPluginFoo(args []string) int {
    fs := flag.NewFlagSet("plugin foo", flag.ExitOnError)
    decodeMode := fs.Bool("decode", false, "Engine decode protocol mode")
    nlriHex := fs.String("nlri", "", "Decode NLRI hex (- for stdin)")
    textOutput := fs.Bool("text", false, "Output text instead of JSON")
    features := fs.Bool("features", false, "List supported features")
    fs.Parse(args)

    // Features query
    if *features {
        fmt.Println("nlri yang")
        return 0
    }

    // CLI mode: --nlri <hex>
    if *nlriHex != "" {
        hex := *nlriHex
        if hex == "-" {
            hex = readLineFromStdin()
        }
        return runCLIDecode(hex, *textOutput)
    }

    // Unsupported feature check
    if capaHex != "" {
        fmt.Fprintln(os.Stderr, "error: plugin 'foo' does not support --capa (available: --nlri)")
        return 1
    }

    // Engine decode mode
    if *decodeMode {
        return runDecodeProtocol()
    }

    // Engine mode
    return runEngineMode()
}
```

## Decode Command Plugin Invocation

The `ze bgp decode` command supports three plugin invocation modes based on naming syntax.

### Syntax Overview

| Syntax | Mode | Execution | Use Case |
|--------|------|-----------|----------|
| `name` | Fork | Subprocess (`ze plugin <name> --decode`) | Default, with in-process fallback |
| `ze.name` | Internal | Goroutine + io.Pipe | Engine-style API, no fallback |
| `ze-name` | Direct | Synchronous in-process | CLI decode, tests, fastest |
| `/path/to/bin` | Fork | External binary with `--decode` | Custom decoders |
| `/path/to/bin --args` | Fork | External binary with args + `--decode` | Custom decoders with options |
<!-- source: internal/component/bgp/format/decode.go -- decode dispatch, fork/internal/direct modes -->

### Examples

```bash
# Fork mode: subprocess (default)
ze bgp decode --plugin flowspec --nlri ipv4/flow 0501180a0000

# Internal mode: goroutine + pipe
ze bgp decode --plugin ze.flowspec --nlri ipv4/flow 0501180a0000

# Direct mode: synchronous in-process (fastest)
ze bgp decode --plugin ze-flowspec --nlri ipv4/flow 0501180a0000

# Fork mode: external binary
ze bgp decode --plugin /usr/local/bin/my-decoder --nlri ipv4/custom abc123

# Fork mode: external binary with arguments
ze bgp decode --plugin "/usr/local/bin/my-decoder --verbose --format yaml" --nlri ipv4/custom abc123
```

### Mode Comparison

| Aspect | Fork | Internal | Direct |
|--------|------|----------|--------|
| Process | New process | Same process | Same process |
| Concurrency | OS-level | Goroutine | None (blocking) |
| Communication | stdin/stdout pipes | Go io.Pipe | Function return |
| Isolation | Full | Memory shared | Memory shared |
| Speed | Slowest | Medium | Fastest |
| Fallback | In-process retry | None | None |
<!-- source: internal/component/bgp/format/decode.go -- fork/internal/direct mode dispatch -->

### External Plugin API Contract

External plugins (paths containing `/`) must:

1. Accept `--decode` flag (appended after any user-provided arguments)
2. Read protocol commands from stdin: `decode nlri <family> <hex>`
3. Write responses to stdout: `decoded json <json>`

**Example external decoder:**

```bash
#!/bin/bash
# /usr/local/bin/my-decoder --decode
# Receives: decode nlri ipv4/custom abc123
# Returns: decoded json {"custom": "data"}

while read -r line; do
    if [[ "$line" == decode\ nlri\ * ]]; then
        echo 'decoded json {"custom": "parsed"}'
    fi
done
```

### Path Arguments

When specifying a path with arguments, use quotes:

```bash
# Arguments are passed before --decode
ze bgp decode --plugin "/opt/decoder --verbose --format json" --nlri ...

# Executes: /opt/decoder --verbose --format json --decode
# Then sends: decode nlri <family> <hex>
```

Arguments are split by whitespace. The `--decode` flag is always appended last.

### Error Handling

| Syntax | On Unknown Plugin |
|--------|-------------------|
| `ze.unknown` | Error: "internal plugin 'unknown' not registered" |
| `ze-unknown` | Error: "direct decoder 'unknown' not available" |
| `unknown` | Try subprocess -> fallback to direct -> nil |
| `/missing/path` | Process spawn fails, returns nil |
<!-- source: internal/component/bgp/format/decode.go -- error handling per mode -->

## Related

- `docs/architecture/api/plugin-protocol.md` - Engine-plugin protocol
- `docs/architecture/debugging/plugin-testing.md` - Testing plugins
- `plan/learned/198-plugin-invocation.md` - Implementation spec
