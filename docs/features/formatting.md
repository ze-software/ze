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

A chain takes one format operator: `json`, `ndjson`, `table`, `text`, `yaml`
or `raw`. Two together are rejected. Filter and display operators chain freely.

| Operator | Kind | Description |
|----------|------|-------------|
| `table` | format | Box-drawing table rendering |
| `text` | format | Space-aligned columns, no box-drawing |
| `json [compact]` | format | Pretty (default) or compact JSON |
| `ndjson` | format | One compact JSON object per line |
| `yaml` | format | YAML output |
| `raw` | format | The dispatcher's JSON, byte for byte. For a program that parses the answer |
| `match <pattern>` | filter | Case-insensitive grep on output lines |
| `count` | filter | Count items (JSON-aware: array length or map size) |
| `first <n>` / `last <n>` | filter | Take first or last N items |
| `display <field>...` | filter | Answer with these fields, in this order |
| `fill [alpha\|overall] [reverse]` | display | Bring the remaining columns back, in a named order |
| `resolve` | display | Add reverse DNS names for IP address values |
| `origin` | display | Add ASN and network name for IP address values |
| `log` | display | Append each update instead of replacing (monitor commands) |
| `no-more` | display | Disable paging (currently a no-op) |

<!-- source: internal/component/command/pipe.go -- knownPipeOps, ApplyPipes, ValidatePipes -->

### Scripting against the daemon: `| raw`

The SSH exec channel is two surfaces at once. An operator running
`ssh <host> 'show bgp peer list'` gets the format `environment cli format
default` names. A program that parses the answer wants the data behind that
rendering, and it must not change the day an operator changes the default.

`| raw` is how the second caller asks. It answers the command dispatcher's JSON
byte for byte, so it is the stable contract for a script:

```
ssh ze-host 'show bgp peer list | raw'
```

`| json` is a renderer, not this. It unwraps a single-key object holding an
array, so `{"commands": [...]}` reaches the caller as a bare `[...]`.

Ze's own tooling uses the same operator. Completion, the runtime command tree
and the live dashboard each parse an exec-channel answer. Each asks for it
through one helper, rather than composing the pipe itself.

<!-- source: internal/core/ssh/client/client.go -- RawCommand, ExecCommandRaw -->

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

<!-- terminal-demo: config-views -->

### Column order

`| table` and `| text` put a command's columns in the order the command
declares, and every column it does not name after those, alphabetically. A
command that declares nothing renders every column alphabetically, as before.

`show bgp summary` declares the order an operator reads a peer in:

```
address  name  description  remote-as  peer-type
state  uptime  state-changed  last-error
routes-received  routes-accepted  routes-sent
updates-received  updates-sent  keepalives-received  keepalives-sent
eor-received  eor-sent  connections-dropped
```

The columns come in the order you read a peer in:

- which peer the row is about
- whether the session is up, and why it last went down
- what the session carries
- the counters you reach for only when something is wrong

`| json`, `| ndjson` and `| yaml` keep their alphabetical keys. A program reads
those three, and key order carries no meaning for a program.

A column is never hidden. Ordering decides where a key renders, never whether
it renders. A field you do not see in the order the command declared is still
in the table, after the declared ones.

Only `show bgp summary` and `show bgp peer list` declare an order today. Each
declaration is an operator judgment about what leads, so commands take one as
somebody makes that judgment.

<!-- source: internal/component/command/column_order.go -- RegisterColumns, ColumnsForCommand -->
<!-- source: internal/component/command/pipe_table.go -- tableStyle.orderKeys, bestColumnOrder -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- registerColumns -->

### Choosing the columns: `| display` and `| fill`

A nineteen-column table answers many questions at once. `| display` cuts it to
the question you asked:

```
show bgp peer list | display state name
```

Those two columns render, in that order, and no other column does. Press Tab
after `| display` and the CLI offers the field names the command declared.

`| fill` brings the rest back, behind what you displayed:

```
show bgp peer list | display state | fill          # then the command's own order
show bgp peer list | display state | fill alpha    # then by field name
show bgp peer list | fill overall                 # every column, narrowest first
show bgp peer list | fill alpha reverse           # every column, reverse by name
```

| Written | The remaining columns come back |
|---------|--------------------------------|
| nothing | not at all |
| `fill` | in the order the command declares, and by name when it declares none |
| `fill alpha` | by field name, whatever the command declares |
| `fill overall` | by the width the column renders at, narrowest first |
| `fill ... reverse` | in the same way, flipped |

The two operators are independent. `| display` names the fields that lead, and
`| fill` says what happens to the ones it did not name. With no `| display`
every column is a remaining column, so `| fill overall` sorts the whole table.

Each takes one type of argument, and that is deliberate: `| display` takes field
names, `| fill` takes keywords. A token that is a field name in one position and
a keyword in another is a token you cannot complete and cannot read.

**`| display` reaches `| json`, `| ndjson` and `| yaml`. `| fill` does not.**
Which fields to answer with is a question you asked out loud, so a program gets
the answer you asked for. The sequence of JSON keys carries no meaning for a
program, so it stays alphabetical.

```
show bgp peer list | display state name | json     # two fields per peer
```

<!-- source: internal/component/command/pipe_columns.go -- parseDisplay, parseFill, applyDisplaySelect -->
<!-- source: internal/component/command/pipe_table.go -- tableStyle.orderKeys, fillKeys -->

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
| `histogram` | Count routes by family and prefix length |
| `graph` | Render an AS-path topology graph (box-drawing) |
| `reason` | Explain best-path selection (`show bgp rib best` only) |

These are parsed and validated the same way as generic pipes, but resolved
into command arguments before the command runs, rather than filtering
output afterward -- so `show bgp rib | peer 10.0.0.1 | count` counts only
that peer's routes server-side instead of fetching everything and counting
client-side.

<!-- source: internal/component/bgp/plugins/cmd/rib/rib.go -- PipeFilter registrations -->
<!-- source: internal/component/command/pipe.go -- foldFilters, lookupFilter, validateFilter -->
