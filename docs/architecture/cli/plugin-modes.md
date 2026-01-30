# Plugin CLI Modes

Plugins have three distinct operating modes with different input/output formats.

## Modes Overview

| Mode | Invocation | Input Format | Use Case |
|------|------------|--------------|----------|
| **CLI Mode** | `ze bgp plugin <name> --json <hex>` | Flag value or `-` for stdin | Direct user invocation |
| **Engine Decode Mode** | `ze bgp plugin <name> --decode` | Protocol commands on stdin | Engine decode delegation |
| **Engine Mode** | `ze bgp plugin <name>` | Full API protocol (stdin) | Engine-plugin communication |

## Design Principle

**CLI mode is for humans. Engine mode is for machines.**

- CLI mode: Simple, direct input. No protocol framing.
- Engine decode mode: Protocol commands for decode delegation.
- Engine mode: Full structured protocol for engine-plugin IPC.

## CLI Mode (`--json` / `--text`)

For direct command-line use. Takes raw hex input, outputs decoded result.

### Invocation

```bash
# JSON output (default format)
ze bgp plugin evpn --json 02210001252C37370001...

# Text output (human-readable)
ze bgp plugin evpn --text 02210001252C37370001...

# From stdin (use - as value)
echo "02210001252C..." | ze bgp plugin evpn --json -

# File input via stdin
ze bgp plugin evpn --text - < nlri.hex
```

### Output

With `--json` (pretty-printed for humans):
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

With `--text`:
```
MAC/IP advertisement rd=1:37.44.55.55:1 mac=FC:15:B4:78:7B:8F
```

### Options

| Flag | Description |
|------|-------------|
| `--json <hex\|->` | Decode hex, output JSON (use `-` for stdin) |
| `--text <hex\|->` | Decode hex, output text (use `-` for stdin) |

## Engine Decode Mode (`--decode` flag)

For engine decode delegation. Engine calls plugin with `--decode` flag and sends
protocol commands on stdin like `decode nlri l2vpn/evpn <hex>`.

### Invocation

```bash
# Started by engine's decode.go
ze bgp plugin evpn --decode
# Then receives: decode nlri l2vpn/evpn <hex>
# Responds: decoded json [...]
```

## Engine Mode (no flags, no args)

For engine-plugin communication. Uses structured protocol on stdin/stdout.

### Invocation

```bash
# Started by engine
ze bgp plugin evpn
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

## Why Two Modes?

### CLI Mode Benefits

- **Simple invocation**: No need to know the API protocol
- **Pipeline friendly**: Works with standard Unix tools
- **Self-documenting**: `--help` shows all options
- **Consistent**: Same pattern as `ze bgp decode`

### Engine Mode Benefits

- **Multiplexed**: Single process handles multiple requests
- **Stateful**: Can maintain state across requests
- **Bidirectional**: Engine can send config, receive events
- **Lifecycle managed**: Engine handles respawn, backpressure

## Implementation Pattern

Plugins implement three modes using `--json`/`--text` for CLI and `--decode` for engine:

```go
func cmdPluginFoo(args []string) int {
    fs := flag.NewFlagSet("plugin foo", flag.ExitOnError)
    decodeMode := fs.Bool("decode", false, "Engine decode protocol mode")
    textHex := fs.String("text", "", "Decode hex, output text (- for stdin)")
    jsonHex := fs.String("json", "", "Decode hex, output JSON (- for stdin)")
    fs.Parse(args)

    // CLI mode: --json <hex> or --text <hex>
    if *textHex != "" || *jsonHex != "" {
        hex := *jsonHex
        textOutput := *textHex != ""
        if textOutput {
            hex = *textHex
        }
        if hex == "-" {
            hex = readLineFromStdin()
        }
        return runCLIDecode(hex, textOutput)
    }

    // Engine decode mode: protocol commands on stdin
    if *decodeMode {
        return runDecodeProtocol()
    }

    // Engine mode: full plugin with startup protocol
    return runEngineMode()
}
```

**Design rationale:**

- `--json <hex>` / `--text <hex>`: Clean CLI for humans, format is explicit
- `--decode` (bool): Engine compatibility, protocol commands on stdin
- No flags: Full plugin mode with startup protocol

## Consistency Across Plugins

All decode-capable plugins follow the same pattern:

| Plugin | CLI JSON | CLI Text | Engine Decode |
|--------|----------|----------|---------------|
| evpn | `--json <hex>` | `--text <hex>` | `--decode` |
| flowspec | `--json <hex>` | `--text <hex>` | `--decode` |
| hostname | `--json <hex>` | `--text <hex>` | `--decode` |

## Related

- `docs/architecture/api/plugin-protocol.md` - Engine-plugin protocol
- `docs/architecture/debugging/plugin-testing.md` - Testing plugins
