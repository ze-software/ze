# Output Formatting

Every command's output goes through the same pipe pipeline, whether you're
poking around interactively or scripting against ze. One operator set,
three ways to use it: set a persistent default, pipe it inline, or apply it
offline to already-captured output.

### The simple way: set a default once

`set cli format <name>` in the interactive CLI picks a default so every
command already displays that way, with no piping needed at all:

```
set cli format table
set cli format          # shows the current default
```

| Format | Description |
|--------|-------------|
| `text` | Space-aligned columns, no box-drawing (default) |
| `table` | Box-drawing table |
| `json` | Pretty-printed JSON |
| `yaml` | YAML output |
| `ndjson` | One compact JSON object per line |

The choice persists for the session via the `ze.cli.format` setting; it can
also be set permanently through YANG config.

<!-- source: internal/component/cli/model_keys.go -- handleSetCLIFormat, validCLIFormats -->
<!-- source: internal/component/command/pipe.go -- ze.cli.format env registration, configuredDefault -->

### Piping inline

Append `| <operator>` to any command, shell-like:

```
show bgp peer list | table
show bgp peer list | json compact
show bgp rib | match established
show bgp peer list | first 5
```

Only one format operator (`json`, `ndjson`, `table`, `text`, `yaml`) is
allowed per chain; combining two is rejected. Filter and display operators
chain freely.

| Operator | Kind | Description |
|----------|------|-------------|
| `table` | format | Box-drawing table rendering |
| `text` | format | Space-aligned columns, no box-drawing |
| `json [compact]` | format | Pretty (default) or compact JSON |
| `ndjson` | format | One compact JSON object per line |
| `yaml` | format | YAML output |
| `match <pattern>` | filter | Case-insensitive grep on output lines |
| `count` | filter | Count items (JSON-aware: array length or map size) |
| `first <n>` / `last <n>` | filter | Take first or last N items |
| `resolve` | display | Add reverse DNS names for IP address values |
| `origin` | display | Add ASN and network name for IP address values |
| `log` | display | Append each update instead of replacing (monitor commands) |
| `no-more` | display | Disable paging (currently a no-op) |

<!-- source: internal/component/command/pipe.go -- knownPipeOps, ApplyPipes, ValidatePipes -->

### The offline way: `ze pipe`

Scripts and pipelines outside an interactive session apply the same
operators to any captured JSON via `ze pipe`:

```
ze show host cpu | ze pipe table
ze debug show | ze pipe match reactor
ze debug show | ze pipe count
ze show bgp peer list | ze pipe yaml
ze show bgp peer list | ze pipe first 5
```

`ze pipe` reads stdin (up to 256 MB), applies the pipe chain given as
arguments, and writes the result to stdout. It's the same operator table
above, minus the display-only operators that only make sense inside a live
session (`log`, `no-more`).

<!-- source: cmd/ze/ze_core_pipe.go -- runPipe, pipeUsage -->

### Configuration presentation

Configuration uses the same separation between data and presentation. Operators
can inspect compact hierarchical blocks, while automation can consume one complete
`set` path per line:

```bash
ze config show router.conf bgp peer transit-a
ze config migrate --format set router.conf
```

`ze config migrate --format hierarchical` converts set syntax back to blocks.
Rendering both forms back to canonical set syntax provides a presentation-neutral
comparison.

<!-- source: internal/component/config/cli/cmd_show.go -- path-scoped hierarchical view -->
<!-- source: internal/component/config/cli/cmd_migrate.go -- set and hierarchical rendering -->

### Demo: Render one configuration for humans and automation

Show one BGP peer as hierarchical blocks and set commands, round-trip between both with identical canonical output, then compose match and count over Ze's plugin registry.

[Play the WebM recording](../../../assets/demos/config-views.webm?v=414a2e496b) · [View the poster](../../../assets/demos/config-views.png?v=e7acb6271c) · [Plain-text transcript](../../../assets/demos/config-views.txt?v=0f968daa34)

Recorded with Ze 26.07.18 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 34 seconds.

```console
$ ze config show router.conf bgp peer transit-a
connection {
    local ip 192.0.2.1
    remote ip 192.0.2.2
}
session {
    asn { local 65000; remote 65001; }
    family ipv4/unicast { prefix maximum 1000000; }
}
$ ze config migrate --format set router.conf | ze pipe match 'bgp peer transit-a'
set bgp peer transit-a connection local ip 192.0.2.1
set bgp peer transit-a connection remote ip 192.0.2.2
set bgp peer transit-a session asn local 65000
set bgp peer transit-a session asn remote 65001
...
$ cmp -s router.set roundtrip.set && echo 'canonical output: identical'
canonical output: identical
$ ze --plugins | ze pipe match flowspec
bgp-nlri-flowspec
flowspec-firewall
...
$ ze --plugins | ze pipe match flowspec | ze pipe count
{"count":3,"pipe":{"count":true}}

Hierarchical and set syntax are alternate presentations of the same parsed configuration. Converting to set syntax and back produces identical canonical set commands. The standalone formatter composes the same match and count operators for shell pipelines.
```


### Command-specific filters

Some commands extend the generic set with their own filter vocabulary,
folded into the command itself rather than staying client-side. `show bgp
rib`, for example, adds:

| Filter | Description |
|--------|-------------|
| `received` / `advertised` | Select the Adj-RIB-In or Adj-RIB-Out side |
| `peer <selector>` | Filter by peer |
| `family <afi/safi>` | Filter by address family |
| `prefix <pattern>` | Filter by prefix |
| `path <pattern>` | Filter by AS path |
| `community <value>` | Filter by standard community |
| `match <pattern>` | Cross-field structured match |
| `count` | Count matching routes without serializing rows |
| `first <n>` / `last <n>` | Take first or last N routes |
| `prefix-summary` | Summarize by family and prefix length |
| `graph` | Render an AS-path topology graph (box-drawing) |
| `reason` | Explain best-path selection (`show bgp rib best` only) |

These are parsed and validated the same way as generic pipes, but resolved
into command arguments before the command runs, rather than filtering
output afterward -- so `show bgp rib | peer 10.0.0.1 | count` counts only
that peer's routes server-side instead of fetching everything and counting
client-side.

<!-- source: internal/component/bgp/plugins/cmd/rib/rib.go -- PipeFilter registrations -->
<!-- source: internal/component/command/pipe.go -- FoldFilters, lookupPipeFilters -->
