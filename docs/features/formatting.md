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

The complete, current set is generated from the operator catalog:
[`pipe-operators.generated.md`](pipe-operators.generated.md). It is the list to
build a tool against, and `ze-doc-verify` fails when it and the product
disagree. The table below describes what each operator does for a reader.

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
| `fill [alpha] [reverse]` | display | Bring the remaining columns back, in a named order |
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
ze show bgp peer list | ze pipe match established
ze show bgp peer list | ze pipe count
ze show bgp peer list | ze pipe yaml
ze show bgp peer list | ze pipe first 5
```

`ze pipe` reads stdin (up to 256 MB), applies the pipe chain given as
arguments, and writes the result to stdout. It's the same operator table
above, minus the display-only operators that only make sense inside a live
session (`log`, `no-more`).

`ze pipe help` lists every operator, split into the two classes below.

<!-- source: cmd/ze/ze_core_pipe.go -- runPipe, pipeUsage -->

### Asking the product: `ze pipe help --json`

Do not hand-copy the operator list into a tool. `ze pipe help --json` answers
the whole language, and it is generated from the same table the parser reads,
so it cannot fall behind:

```
ze pipe help --json
```

Each entry carries the operator's `name`, its `class`, the `shapes` of answer
it acts on, whether it takes an `arg`, what a second occurrence in one chain
means (`repeat`), and a one-line `description`.

The two classes are the contract:

| Class | Meaning |
|-------|---------|
| `global` | acts on the answer whatever it holds. Every command that reaches the pipe layer owes these |
| `data` | acts on rows or fields, so a command owes it only where its answer has them |

`shapes` names the answer shapes the operator applies to, using the same words
the answer head uses on the wire: `doc` for one document or one value, `map`
for rows that describe themselves, `tab` for rows read against declared column
names.

<!-- source: cmd/ze/ze_core_pipe.go -- printPipeCatalogJSON -->
<!-- source: internal/component/command/pipe_catalog.go -- pipeCatalog -->

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

`show bgp` declares the order an operator reads a peer in:

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

Only `show bgp` and `show bgp peer list` declare an order today. Each
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
show bgp peer list | fill alpha reverse            # every column, reverse by name
```

| Written | The remaining columns come back |
|---------|--------------------------------|
| nothing | not at all |
| `fill` | in the order the command declares, and by name when it declares none |
| `fill alpha` | by field name, whatever the command declares |
| `fill ... reverse` | in the same way, flipped |

The two operators are independent. `| display` names the fields that lead, and
`| fill` says what happens to the ones it did not name. With no `| display`
every column is a remaining column, so `| fill alpha` sorts the whole table.

Each takes one type of argument, and that is deliberate: `| display` takes field
names, `| fill` takes keywords. A token that is a field name in one position and
a keyword in another is a token you cannot complete and cannot read.

A third way was removed on 2026-08-19. `| fill overall` ordered the columns by
the width they render at, and that width is known only after every cell of the
whole answer has been rendered, so the first row could not be written until the
last had been read. A streamed answer cannot do that. `overall` is now refused
by name, and `| fill` and `| fill alpha` are unaffected.

**`| display` reaches `| json`, `| ndjson` and `| yaml`. `| fill` does not.**
Which fields to answer with is a question you asked out loud, so a program gets
the answer you asked for. The sequence of JSON keys carries no meaning for a
program, so it stays alphabetical.

```
show bgp peer list | display state name | json     # two fields per peer
```

<!-- source: internal/component/command/pipe_columns.go -- parseDisplay, parseFill, applyDisplaySelect -->
<!-- source: internal/component/command/pipe_table.go -- tableStyle.orderKeys, fillKeys -->

### A name for a chain: pipe aliases

Some selections are worth a name. `show bgp` answers its aggregate
fields and its peer rows side by side, and an operator usually wants one half:

```
show bgp | peers      # the peer rows, as a table
show bgp | summary    # router-id, local AS, uptime and the peer counts
```

Each is a name for a `| display` you would otherwise type in full. `| peers` is
`| display peers`, and `| summary` names the five aggregate fields. Press Tab
after the pipe character and both are offered beside the operators.

An alias is fixed at registration, which keeps it readable:

- It takes no argument. `| peers established` is refused by name.
- It never stands for another alias, so what you see is one substitution.
- It is expanded before the command runs, so `| json`, `| text` and every other
  format carry it.

A plugin names an alias for its own commands too. The BGP RPKI plugin declares
`summary` on `show bgp rpki`, so the counters come back without the cache server
rows:

```
show bgp rpki             # the counters and one row for each cache server
show bgp rpki | summary   # the counters alone
```

`command help "show bgp rpki"` lists the aliases a command answers to, with the
chain each one stands for.

A plugin's alias is registered in the daemon, and one client does not read it.
`ze cli` with no command argument runs its own copy of the interactive model and
expands the chain itself. A plugin's alias comes back there as
`pipe error: unknown pipe operator: summary`, and Tab does not offer it. Use
`ze cli -c "show bgp rpki | summary"`, or the interactive session a plain ssh
client reaches. The aliases built into Ze itself, `| summary` and `| peers` on
`show bgp`, resolve in every client.

<!-- source: internal/component/command/alias.go -- RegisterAliases, RegisterPluginAliases, AliasesForCommand -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- registerAliases -->
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, summaryAliasExpansion -->
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->

### Command-specific filters

Some commands extend the generic set with their own filter vocabulary,
folded into the command itself rather than applying to its answer. `show bgp
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
output afterward. `show bgp rib | peer 10.0.0.1 | count` counts only that peer's
routes in the RIB iterator. It does not serialize every route and count the
answer. Both kinds run in the daemon. A generic pipe filters what the command
already produced. A command filter stops it being produced.

<!-- source: internal/component/bgp/plugins/cmd/rib/rib.go -- PipeFilter registrations -->
<!-- source: internal/component/command/pipe.go -- foldFilters, lookupFilter, validateFilter -->
