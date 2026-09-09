# Plugin CLI Modes

Plugins have four distinct operating modes with different input/output formats.

## Modes Overview

| Mode | Invocation | Input Format | Use Case |
|------|------------|--------------|----------|
| **CLI Mode** | `ze plugin <name> --nlri <hex>` | Flag value or `-` for stdin | Direct user invocation |
| **Engine Decode Mode** | `ze plugin <name> --decode` | Decode commands on stdin/stdout | Engine decode delegation |
| **Engine Mode** | `ze plugin <name>` | YANG RPC over TLS connect-back | Engine-plugin communication |
| **Query Mode** | `ZE_PLUGIN_MODE=declare ze plugin <name>` | No input: one environment variable | Read what a plugin declares, with no daemon |
<!-- source: internal/component/bgp/cli/cmd_plugin.go -- plugin CLI dispatch -->
<!-- source: internal/component/plugin/cli/main.go -- Run, answerQuery -->

## Design Principle

**CLI mode is for humans. Engine mode is for machines.**

- CLI mode: Simple, direct input. No protocol framing.
- Engine decode mode: stdin/stdout decode commands for one decoder process.
- Engine mode: full YANG RPC protocol over the plugin hub TLS connection.
- Query mode: one Stage 1 declaration on stdout, and none of a live start's work.

## Plugin Features

Plugins declare what decode features they support via `--features`:

```bash
ze plugin bgp-nlri-evpn --features
# Output: nlri

ze plugin bgp-hostname --features
# Output: capa yang

ze plugin bgp-gr --features
# Output: capa

ze plugin bgp-rib --features
# Output: yang
```

### Feature Matrix

| Plugin | --capa | --nlri | Notes |
|--------|--------|--------|-------|
| bgp-hostname | yes | no | FQDN capability (code 73) |
| bgp-gr | yes | no | Graceful Restart (code 64) |
| bgp-nlri-evpn | no | yes | l2vpn/evpn NLRI |
| bgp-nlri-flowspec | no | yes | ipv4/flow, ipv6/flow NLRI |
| bgp-rib | no | no | YANG only |
| bgp-rr | no | no | No decode features |
<!-- source: internal/component/plugin/registry/ -- plugin registration, Features field -->

### Feature Names

- `nlri` - Decode NLRI from UPDATE message (`--nlri <hex>`)
- `capa` - Decode capability from OPEN message (`--capa <hex>`)
- `yang` - Output YANG schema (`--yang`)

## CLI Mode (`--nlri` / `--capa`)

For direct command-line use. Takes raw hex input, outputs decoded result.

### NLRI Plugins (`bgp-nlri-evpn`, `bgp-nlri-flowspec`)

```bash
# JSON output (default)
ze plugin bgp-nlri-evpn --nlri 02210001252C37370001...

# Text output
ze plugin bgp-nlri-evpn --nlri 02210001252C37370001... --text

# From stdin
echo "02210001252C..." | ze plugin bgp-nlri-evpn --nlri -

# With family context (flowspec)
ze plugin bgp-nlri-flowspec --nlri 0718... --family ipv4/flow
```

### Capability Plugins (`bgp-hostname`, `bgp-gr`)

```bash
# JSON output (default)
ze plugin bgp-hostname --capa 07726f7574657231...

# Text output
ze plugin bgp-hostname --capa 07726f7574657231... --text

# From stdin
echo "07726f7574657231..." | ze plugin bgp-hostname --capa -
```

### Unsupported Features

Standard error when requesting unsupported decode type:

```bash
ze plugin bgp-hostname --nlri 02210001252C...
# stderr: error: plugin 'bgp-hostname' does not support --nlri (available: --capa)
# exit code: 1

ze plugin bgp-nlri-evpn --capa 07726f7574657231...
# stderr: error: plugin 'bgp-nlri-evpn' does not support --capa (available: --nlri)
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
<!-- source: internal/component/bgp/cli/cmd_plugin.go -- CLI flag parsing, --nlri, --capa, --text -->

## Engine Decode Mode (`--decode` flag)

For engine decode delegation. Engine calls plugin with `--decode` flag and sends
protocol commands on stdin like `decode nlri l2vpn/evpn <hex>`.

### Invocation

```bash
# Started by engine's decode.go
ze plugin bgp-nlri-evpn --decode
# Then receives: decode nlri l2vpn/evpn <hex>
# Responds: decoded json [...]
```

## Engine Mode (no flags, no args)

For engine-plugin communication. The process manager starts the plugin with
`ZE_PLUGIN_*` environment variables, then the plugin connects back to the
hub TLS listener and speaks newline-framed YANG RPC.

### Invocation

```bash
# Started by engine with ZE_PLUGIN_* env vars
ze plugin bgp-nlri-evpn
```

### Protocol

Uses the plugin process protocol:

```
# Plugin declares its registration
#1 ze-plugin-engine:declare-registration {"families":[{"name":"l2vpn/evpn","mode":"both"}]}

# Engine responds
#1 ok
```

See `docs/architecture/api/process-protocol.md` for the full protocol.

## Query Mode (`ZE_PLUGIN_MODE=declare`)

For reading what a plugin declares. The process starts, and it knows that it is
interrogated rather than started.

### Invocation

```bash
ZE_PLUGIN_MODE=declare ze plugin bgp-rib
# stdout: #1 ze-plugin-engine:declare-registration {"commands":[...],"pipes":[...]}
# exit code: 0
```

### Carrier

The mode is carried by the `ze.plugin.mode` environment variable, read as
`ZE_PLUGIN_MODE`. An external plugin is started as a shell-quoted run string, so
a forker cannot append a flag to it, and the environment block is the one
carrier that string cannot swallow. `declare` is the only value the mode takes.
Any other value, and an absent variable, is a live start.
<!-- source: pkg/plugin/sdk/sdk.go -- EnvPluginMode, ModeDeclare, QueryModeRequested -->

The run string is given to `/bin/sh -c`, by the query and by the daemon's live
start, so a plugin the daemon can start is startable by the query. A host with
no shell starts neither: the query row reads `unstartable` and its reason names
the shell, and `ze doctor` reports the same dependency under
`doctor-plugin-shell-missing`.
<!-- source: internal/component/plugin/shell.go -- Shell, ShellAvailable -->
<!-- source: internal/component/plugin/declarations.go -- unstartableOutcome -->

### The answer

The answer is the Stage 1 `declare-registration` message, in the protocol's own
newline framing, written to stdout instead of to the hub connection. It carries
the commands and the pipe aliases the plugin declares. One message in one
format: a reader parses the framed line and ignores every other line, so a
banner or a log line does not corrupt the answer.

A plugin that declares no command and no pipe still writes the line, with an
empty declaration. "Declared nothing" and "sent nothing" are different answers,
and a reader that cannot tell them apart reports a working plugin as silent.

### Inertness

For a plugin the ze binary carries, inertness is unreachability rather than a
promise. `Run` looks the plugin up in the registry and answers from that
registration, so `CLIHandler` is never called and no plugin code runs at all: no
data initialization, no connection, no bind, no listener, no timer. The query
process reads no hub variable and opens no connection, so it joins no hub.

For a plugin binary ze does not carry, the same guarantee is bought by adopting
`sdk.RunOrDeclare(declaration, activate)`: the declaration and the activation
function are separate arguments, and the mode returns before `activate` is
called, so the plugin's side-effecting code is unreachable under a query. What
the plugin's `main` does ABOVE that call is outside the guarantee. A plugin that
adopts neither route runs its live start under a query, and the reader records
it as having sent nothing. For third-party code this is enforcement for adopters
and convention for everyone else.
<!-- source: pkg/plugin/sdk/sdk_query.go -- RunOrDeclare -->

## Why Four Modes?

### CLI Mode Benefits

- **Simple invocation**: No need to know the API protocol
- **Pipeline friendly**: Works with standard Unix tools
- **Self-documenting**: `--help` shows all options
- **Discoverable**: `--features` shows what plugin can do

### Engine Decode Mode Benefits

- **Stateless delegation**: Engine delegates decode to plugin
- **Protocol-based**: Consistent request/response format for decode delegation

### Engine Mode Benefits

- **Multiplexed**: Single process handles multiple requests
- **Stateful**: Can maintain state across requests
- **Bidirectional**: Engine can send config, receive events
- **Lifecycle managed**: Engine handles respawn, backpressure
<!-- source: internal/component/plugin/server/ -- plugin process management -->

### Query Mode Benefits

- **No daemon**: the declarations are readable with nothing running
- **Inert**: no plugin code runs for a plugin the ze binary carries
- **One format**: the answer is the Stage 1 message the engine already receives

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
ze bgp decode --plugin bgp-nlri-flowspec --nlri ipv4/flow 0501180a0000

# Internal mode: goroutine + pipe
ze bgp decode --plugin ze.bgp-nlri-flowspec --nlri ipv4/flow 0501180a0000

# Direct mode: synchronous in-process (fastest)
ze bgp decode --plugin ze-bgp-nlri-flowspec --nlri ipv4/flow 0501180a0000

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
| Communication | stdin/stdout decode protocol | Go io.Pipe | Function return |
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

- `docs/architecture/api/process-protocol.md` - Engine-plugin protocol
- `docs/architecture/debugging/plugin-testing.md` - Testing plugins
- `plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture" - invocation-mode design history (retired summary 198), including the internal-plugin 5-stage startup deadlock its Load-bearing invariants record
