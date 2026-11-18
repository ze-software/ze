# Adding Commands

A plugin's commands are defined in YANG, the way the commands compiled into
`ze` are. Ze builds the command and the check of its arguments from the
module, and asks your plugin only with values that check accepts. An external plugin and a compiled-in plugin define a command the same
way; they differ only in how they reach Ze (`docs/architecture/plugin/plugin-system.md`,
"One plugin, two ways to reach the bus").

A command takes three things:

| What | Where | What it gives |
|------|-------|---------------|
| The command path, its help and its arguments | A `-cmd` module | The command in the tree, its help text, and the check of each argument |
| The parameters the API surfaces publish | The `input` of an rpc, in an `-api` module, named by the command's `ze:rpc` | The parameter list MCP, REST and gRPC publish for the command |
| The route to your plugin | An entry in the Stage 1 `commands` list, and a handler | Ze asks your plugin when an operator runs the command |

<!-- source: internal/component/config/yang/join_command.go -- refuseCommandConflicts, refuseRPCLinks -->
<!-- source: internal/component/plugin/server/startup.go -- the Stage 1 commands registered for the process -->

## Defining a Command in YANG

This is the command module of the example plugin
([`examples/plugin/python/acme_monitor.py`](../../examples/plugin/python/acme_monitor.py),
and its Go twin [`examples/plugin/go/main.go`](../../examples/plugin/go/main.go)).
It defines `show acme-monitor status` and `show acme-monitor target <name>`:

```yang
module acme-monitor-cmd {
    namespace "urn:acme:monitor:cmd";
    prefix acmecmd;
    import ze-extensions { prefix ze; }

    container show {
        config false;

        container acme-monitor {
            config false;
            ze:help "Inspect the acme monitor.";

            container status {
                config false;
                ze:command "acme-monitor:show-status";
                ze:help "Show the target and the probe interval.";
            }

            container target {
                config false;
                ze:command "acme-monitor:show-target";
                ze:rpc "acme-monitor-api:show-target";
                ze:help "Show the configured target of the acme monitor.";

                leaf name {
                    type string {
                        length "1..16";
                    }
                    mandatory true;
                    ze:help "Name of the target.";
                }
            }
        }
    }
}
```

And the rpc that describes the argument of `target`:

```yang
module acme-monitor-api {
    namespace "urn:acme:monitor:api";
    prefix acmeapi;
    import ze-extensions { prefix ze; }

    rpc show-target {
        ze:help "Show the configured target of the acme monitor.";
        input {
            leaf name {
                type string {
                    length "1..16";
                }
                mandatory true;
                ze:help "Name of the target.";
            }
        }
    }
}
```
<!-- source: internal/test/externalyang/externalyang.go -- commandsText, apiText -->

How Ze reads the module:

- **The name ends in `-cmd`.** Ze takes commands only from a module with that
  suffix.
- **A command is a `config false` container that holds `ze:command`.** Every
  container on the way to it is `config false` too, and the first one is a
  verb (`show`, `request`, `clear`, ...). The path of the container is the
  command: `show acme-monitor target`.
- **The containers on the way are shared.** `show` is the verb container every
  module uses, and `acme-monitor` can hold commands from other modules too. A
  command path is yours alone: a path another module declares refuses the
  start.
- **`ze:command` names the command's wire method**, `<word>:<word>`, which no
  other module may declare.
- **Each leaf is an argument.** Its type, `length`, `range`, pattern or
  enumeration is what Ze checks the value against, and `mandatory true`
  makes it required.
- **`ze:help` is the one-line summary** shown in listings and completion, and
  `description` is the longer text, as for a config leaf
  ([schema.md](../schema/index.md), "Best Practices").
- **`ze:rpc "<module>:<rpc>"` names the rpc whose `input` describes the
  command's parameters**, in a module your plugin brings. Ze publishes the
  command with the parameters of that rpc. Give the input the same leaves as
  the command container, with the same types.

<!-- source: internal/component/config/yang/join.go -- refuseIdleModules, idleModuleRefusal -->
<!-- source: internal/component/config/yang/join_command.go -- claimCommand, refuseRPCLinks -->
<!-- source: internal/component/config/yang/rpc_publish.go -- PublishedRPCs -->

Your plugin sends every module in its Stage 1 `schemas` list, one entry for
each, beside its config module ([schema.md](../schema/index.md), "Registering a
Schema"). In Python, call `plugin.schema()` once for each module.

## Declaring and Handling Commands

Ze asks your plugin only for a command its Stage 1 `commands` list names.
List each command your `-cmd` module defines, by its path, and register a
handler. The entry needs no help text when the module carries it.

Every `ze:command` node your modules bring must be reached by a command your
Stage 1 `commands` list declares: the declared command is the node's path, or
a prefix of it that ends at a word boundary (`show acme-monitor` reaches
`show acme-monitor status`). Ze sends a typed command to the longest
registered command it starts with, whichever plugin registered it, so a node
no declared command reaches could never be answered, and the start is refused:

```
plugin "acme-monitor" brings command "show acme-monitor target" in module "acme-monitor-cmd", and declares no command at Stage 1 that reaches it
```

Only a name Ze registers counts: a name with leading or trailing spaces, in
upper case, or that breaks the command grammar, and a name a command of `ze`
already holds, reach nothing. In the daemon, a longer command another plugin
registered before yours takes the node from your shorter one.
`ze show plugin conformance run "<run string>"` reports a node no declared
command reaches in its `command-nodes-served` row, before the plugin is ever
started ([testing.md](../testing/index.md)).

A declared command with no `ze:command` node of its own is accepted, as it is
for a plugin compiled into `ze`.
<!-- source: internal/component/plugin/server/stage1_commands.go -- refuseUnservedCommandNodes, commandServed, unservedCommandRefusal -->
<!-- source: internal/component/plugin/server/command.go -- Dispatcher.dispatchPlugin, longestReach, ErrUnknownCommand -->
<!-- source: internal/component/plugin/server/command_registry.go -- commandNameRefusal -->

In Go:

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
    switch command {
    case "show acme-monitor status":
        return "done", map[string]string{"target": cfg.Target, "interval": cfg.intervalText()}, nil
    case "show acme-monitor target":
        // Ze checked the value against the leaf `name`: args holds it.
        if args[0] != cfg.Target {
            return "", nil, fmt.Errorf("acme-monitor has no target %s\nfix: show acme-monitor status", args[0])
        }
        return "done", map[string]string{"target": args[0], "interval": cfg.intervalText()}, nil
    }
    return "", nil, fmt.Errorf("acme-monitor does not serve %q", command)
})

err := p.Run(ctx, &sdk.Registration{
    Schemas: []sdk.SchemaDecl{
        {Module: "acme-monitor-conf", Namespace: "urn:acme:monitor:conf", YANGText: confYANG},
        {Module: "acme-monitor-cmd", Namespace: "urn:acme:monitor:cmd", YANGText: cmdYANG},
        {Module: "acme-monitor-api", Namespace: "urn:acme:monitor:api", YANGText: apiYANG},
    },
    WantsConfig: []string{"acme-monitor"},
    Commands: []sdk.CommandDecl{
        {Name: "show acme-monitor status"},
        {Name: "show acme-monitor target"},
    },
})
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, CommandDecl, SchemaDecl -->
<!-- source: examples/plugin/go/main.go -- declaration, monitor.command -->

In Python, one decorator both lists the command at Stage 1 and serves it:

```python
@plugin.command("show acme-monitor target")
def target(args):
    name = args[0]
    if name != state["target"]:
        raise CommandError("acme-monitor has no target %s" % name, fix="show acme-monitor status")
    return {"target": name, "interval": _interval_text()}
```
<!-- source: examples/plugin/python/ze_plugin.py -- Plugin.command, CommandError -->

The handler gets the values in `args`, as the operator typed them, and only
after Ze accepted them. It does not check the type again, and it writes no
usage message: Ze refuses a bad line before the handler runs. It still answers
for what only it knows, such as a name the config does not hold, with an
error whose last line is `fix: ` and the command that clears it.

## What the Operator Gets

Typed in a `ze cli` session, sent over SSH or with `ze cli -c`, the command
runs in the running Ze, which asks your plugin:

```
$ ze cli -c "show acme-monitor target web.example.net"
```

A value the leaf's type refuses is refused by Ze, your plugin is not asked,
and the command exits 1:

```
$ ze cli -c "show acme-monitor target this-name-is-far-too-long"
error: show acme-monitor target name: "this-name-is-far-too-long" is outside the accepted length (25 characters)
name accepts 1..16 characters
fix: show acme-monitor target <1..16 characters>
```

A line with the mandatory leaf missing is refused before the plugin is asked,
with the whole command as its correction:

```
error: required argument missing: name
This command needs name: The target name, from 1 to 16 characters.
fix: show acme-monitor target name <name>
```
<!-- source: test/plugin/external-yang-command-argument.ci -- the refusals and the value the plugin receives -->

MCP, REST and gRPC list the command with the parameters of the rpc its
`ze:rpc` names, and send it to the same dispatcher, so the same check runs
before your plugin is asked. Each parameter carries the leaf's `ze:help` as
its summary, its `description`, and whether it is mandatory, as a compiled-in
command's parameters do; the leaf's length or range is not published.
<!-- source: internal/component/config/yang/join_command.go -- refuseRPCLinks, the publication rule -->
<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta, the parameters of a command only a plugin serves -->

`ze show acme-monitor target web.example.net` typed in the shell, with no `cli -c`, asks
the running Ze and gets the answer `ze cli -c` gives. With no Ze running it is
reported as not available, with the fix `ze start`
([commands.md](../../../architecture/api/commands/index.md#a-path-the-ze-binary-does-not-hold-an-external-plugins-command),
"A path the `ze` binary does not hold").
<!-- source: cmd/ze/internal/cmdutil/unheld.go -- unheldRoute.answer, refuseNotAvailable -->

## Refusals at Startup

Ze judges your modules when `ze start` runs your plugin, before it parses the
config, by the rules the modules compiled into `ze` obey. A refusal stops the
start, names your plugin and module and the change that fixes it, and carries
the doctor code `doctor-plugin-schema-refused` ([schema.md](../schema/index.md)). The
first line of each refusal:

| You wrote | The refusal starts |
|-----------|--------------------|
| A command path another module declares | `Module "acme-monitor-cmd" of plugin "acme-monitor" declares command "<path>", which module "<other>" ... already declares` |
| A `ze:command` wire method another module declares | `Module "acme-monitor-cmd" of plugin "acme-monitor" declares wire method "<method>" at "<path>", which module "<other>" ... already declares` |
| A `ze:rpc` that names an rpc its module does not define, a `ze:rpc` on a container with no `ze:command`, or a malformed wire method | `The commands that the YANG modules of plugin "acme-monitor" declare do not match their rpcs` |
| A `ze:rpc` that names a module neither your plugin nor `ze` brings | `A command that plugin "acme-monitor" declares names an rpc in a module ze does not hold: ...` |
| A module that holds definitions ze reads nowhere: no config, no command, no rpc, and no other module imports it (a `-cmd` module with neither a `ze:command` nor a `ze:help`, or commands in a module whose name does not end in `-cmd`). A module that declares nothing at all is accepted | `Module "<module>" of plugin "acme-monitor" adds nothing to the config schema, the commands or the rpcs` |

<!-- source: internal/component/config/yang/join_command.go -- claimCommand, refuseRPCLinks -->
<!-- source: internal/component/config/yang/join.go -- idleModuleRefusal -->

## At Restart

Your modules change only when `ze` restarts. When Ze starts your plugin again
(`request plugin restart <name>`, or `failure-policy restart` after it ended),
the plugin MUST declare the same modules, each with the same name, namespace
and text. A changed command module is refused like a changed config module:
Ze does not run the plugin, and raises `doctor-plugin-schema-changed`, whose
fix is the restart of `ze` ([schema.md](../schema/index.md), "What works today").
<!-- source: internal/component/plugin/server/stage1_schema.go -- refuseChangedSchema, schemaDifference -->

## Wire Format

Commands are delivered to the plugin as `execute-command` RPCs over the MuxConn connection. The wire format uses `#<id> <verb> [<json>]` framing.
<!-- source: pkg/plugin/rpc/message.go -- AppendRequest, AppendResult, AppendError -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

### Request (engine to plugin)

```
#17 ze-plugin-callback:execute-command {"serial":"abc123","command":"show my-plugin status","args":[],"peer":""}
```

### Success Response (plugin to engine)

The answer is a head, its records and a terminator, on every connection. The
frame is the same whatever the payload is, so a handler that built one value
takes the same three lines as a handler that walked a table.

An answer line carries no verb and no key name. The field after the id is a
three-byte word saying what the line IS:

| Word | The line is |
|------|-------------|
| `top` | the head, which opens the answer |
| `row` | one record your command produced |
| `bad` | one record it rejected. The walk goes on |
| `end` | the terminator, which ends the answer |
| `wrn` | the terminator of a command that completed and states a warning |
| `nay` | the whole answer to a command text naming no command |

The head carries a second three-byte word saying how the records read:

| Word | The records are |
|------|-----------------|
| `doc` | one document. The whole answer is that one value |
| `map` | one map of names to values for each record |
| `tab` | one positional row for each record, read against the column names |

Every field after those is positional and takes one of two shapes. A NUMBER is
decimal digits closed by a space or by the end of the line. A TEXT is decimal
digits, a colon, then that many BYTES. **The count is a BYTE count, never a count
of characters.** A text of zero bytes is written `0:`, present and empty, so a
line's field count never varies.

```
#17 top doc 0: 0:
|   |   |   |  |
|   |   |   |  +----- column names, 0 BYTES, so the records are not positional
|   |   |   +-------- envelope name, 0 BYTES, so the document carries its own
|   |   +------------ item type doc: the whole answer is one document
|   +---------------- kind top: the head, always the first line
+-------------------- correlation id 17, echoed from the request

#17 row 34:{"status":"running","uptime":3600}
|   |   |  |
|   |   |  +----- those 34 bytes, what your handler returned, byte for byte
|   |   +-------- 34, the payload's BYTE count, then the colon every text carries
|   +------------ kind row: one record the command produced
+---------------- correlation id 17

#17 end 1 0 0:
|   |   | | |
|   |   | | +----- message, 0 BYTES, so the command stated none
|   |   | +------- 0 rows rejected
|   |   +--------- 1 record produced
|   +------------- kind end: the terminator, always the last line
+----------------- correlation id 17
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteDocumentAnswer -->

The engine reads that sequence back into one `ExecuteCommandOutput`: the record
is its `data`, and its `status` is derived from the terminator, which is the one
line an answer states an outcome on. A handler that returns status `warning`
has its terminator spelled `wrn`, and the engine reads `warning` back.
<!-- source: internal/component/plugin/ipc/rpc.go -- ExecuteCommandValue -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandOutput -->

### Error Response (plugin to engine)

```
#17 error {"message":"execute-command not supported"}
```

## Return Values

The `OnExecuteCommand` handler returns three values: `(status string, data any, err error)`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand, executeCommandOutput -->

### Success with Data

```go
return "done", map[string]any{
    "count": 42,
    "items": []string{"a", "b"},
}, nil
```

The SDK marshals this value once and sends it as the one record of the answer.
The three lines read exactly as the decoded ones above. The `doc` head names no
envelope and no columns, the `row` states 30 bytes and then the marshaled value,
and the terminator counts one record, no rejection and no message.
```
#17 top doc 0: 0:
#17 row 30:{"count":42,"items":["a","b"]}
#17 end 1 0 0:
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandOutput, executeCommandAnswer -->

### Success without Data

```go
return "done", nil, nil
```

Response. A command that reported nothing writes no record, and the terminator
says so: zero records produced, zero rejected, and a message of zero bytes.
Nothing is not the same answer as an empty collection:
```
#17 top doc 0: 0:
#17 end 0 0 0:
```
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteDocumentAnswer, writeDocumentLines -->

### Success with Rows

A command that walks a large collection returns an `sdk.Records` rather than a
built value. The SDK writes one line for each row, so neither the plugin nor the
engine holds the whole answer:

```go
return "done", sdk.Records{Key: "sessions", Rows: sessionRows()}, nil
```

```
#17 top map 8:sessions 0:
|   |   |   | |        |
|   |   |   | |        +----- column names, 0 BYTES, so the rows are not positional
|   |   |   | +-------------- those 8 bytes: sessions, your Key
|   |   |   +---------------- 8, the envelope name's BYTE count, then its colon
|   |   +-------------------- item type map: each record is one map of names to values
|   +------------------------ kind top: the head
+---------------------------- correlation id 17

#17 row 26:{"id":1,"peer":"10.0.0.1"}
#17 row 26:{"id":2,"peer":"10.0.0.2"}
#17 end 2 0 0:
```

Each `row` states the 26 bytes of its payload and then those bytes. The
terminator counts two records produced, none rejected, and no message.

`Key` names the envelope the rows belong under. A handler MUST NOT name it
`errors`, because that name holds the rows the walk rejected. `Rows` is walked
once, before the handler's call returns. A handler MUST NOT store the sequence,
and MUST keep whatever it reads alive until then. A row that no wire message can
carry is reported as a rejected row, and the walk continues.

A walk of 256 rows or fewer collapses to one `doc` record, which is the
JSON the command answered with before it produced rows at all. The encoder
decides that from the walk, and the handler states nothing about the wire. That
document is one line as well, so rows that each fit and collapse into something
no line can carry earn the same rejected row, and the head then states `map`.
<!-- source: pkg/plugin/records.go -- Row, Record, Records, Records.WriteAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, boundedRecord -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseRecords, AnswerErrorsKey -->

### Handler Error

If the handler returns a non-nil error, the SDK sends an error response:

```go
return "", nil, fmt.Errorf("operation failed: database timeout")

```
Response:
```
#17 error {"message":"operation failed: database timeout"}
```
<!-- source: pkg/plugin/rpc/message.go -- FormatError, NewErrorPayload -->

## ExecuteCommandInput Fields

The engine sends these fields in the `execute-command` RPC:
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput -->

| Field | Type | Purpose |
|-------|------|---------|
| `serial` | string | Correlation ID for the request |
| `command` | string | Command name (e.g., `"show my-plugin status"`) |
| `args` | []string | The words after the command path, as the operator typed them, once Ze accepted them (may be empty) |
| `peer` | string | Peer selector (may be empty) |

## Naming Conventions

| Pattern | Example | Purpose |
|---------|---------|---------|
| `show <plugin> status` | `show acme-monitor status` | Get current state |
| `show <plugin> <item> <name>` | `show acme-monitor target web` | Get one item, named by an argument leaf |
| `request <plugin> <action>` | `request acme-monitor probe` | Perform an action |
| `clear <plugin> <counters>` | `clear acme-monitor statistics` | Reset counters |

**Rules:**
- Start with a verb, then the plugin name, to avoid conflicts
- Use kebab-case for multi-word commands
- Keep commands short and memorable

## CommandDecl Fields

Each entry of the Stage 1 `commands` list has these fields. For a command
your `-cmd` module defines, `Name` is all it needs: the module carries the
help and the arguments.
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Command path (for example, `"show acme-monitor status"`). The first word MUST be one of the canonical verbs (`show`, `request`, `clear`, ...): the engine refuses any other name at registration |
| `ShortHelp` | string | No | The one-line SUMMARY, shown wherever the command appears on one line. One line, at most 256 bytes |
| `Description` | string | No | The LONG explanation this command's own help page prints, under the summary. At most 4096 bytes, newlines kept |
| `Args` | []string | No | Argument names, for the help and completion of a command no module defines. Ze checks no value against them |
| `Completable` | bool | No | Whether the command supports tab completion |
| `Shape` | string | No | What the answer holds: `doc`, `map`, `tab` or `blob` |
| `Columns` | []string | No | The answer's keys, in the order a person reads them. Needs a `Shape` that has rows |
| `AddressFields` | []string | No | The keys whose value holds an IP address or a prefix. Needs a `Shape` |

`ShortHelp` and `Description` are two texts, and neither is derived from the
other. Write the summary as one sentence a reader meets in a list. Write the
explanation as the paragraphs they read when they ask about that one command.
Declare no `Description` and the help page prints the summary alone.

The wire keys are `short-help` and `description`. The spelling is `description`
and not `help`, because `help` already names the summary in a completion row on
this same protocol.

## Declaring What Your Answer Holds

Declare a shape and the CLI publishes the pipe operators your command supports,
and refuses the others BY NAME before your handler runs. Declare nothing and the
CLI waits for the answer, then refuses from what it has in hand.

<!-- source: internal/component/command/pipe.go -- validateDeclaredShape -->

| Shape | Your handler answers |
|-------|----------------------|
| `doc` | one document or one value. No row operator applies |
| `map` | rows that carry their own keys, such as a map of name to object |
| `tab` | rows read against the column names you declare |

```go
err := p.Run(ctx, &sdk.Registration{
    Commands: []sdk.CommandDecl{{
        Name:          "show my-plugin peers",
        ShortHelp:     "Show the sessions",
        Shape:         "tab",
        Columns:       []string{"address", "state", "up"},
        AddressFields: []string{"address"},
    }},
})
```

Four rules decide whether the engine accepts the declaration:

- The shape MUST be `doc`, `map`, `tab` or `blob`. Any other spelling fails startup.
- A `Columns` or `AddressFields` list MUST come with a `Shape`.
- A column name MUST be a key your handler writes, in the same spelling. The
  engine cannot check this, and a name that names nothing orders nothing.
- One command declares at most 64 columns and 16 address fields, each name 1 to
  64 bytes.

Declare one shape for every argument the command takes. A command answering a
row set with no argument, and one bare object with an argument, declares neither
branch truthfully. Make both branches answer rows.

`AddressFields` is an admission gate. It decides whether `| resolve` and
`| origin` run at all. It does not decide which values they decorate: both walk
every key of the answer and decorate each string that parses as an address.

<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls -->
<!-- source: internal/component/command/pipe_resolve.go -- resolveJSON -->
<!-- source: internal/component/command/pipe_origin.go -- originJSON -->

## Naming a Pipe Alias for Your Command

A pipe alias is the word an operator types after the pipe character, standing
for an operator chain they would otherwise type in full. Declare one in the
`Pipes` list of the same `Registration`:
<!-- source: pkg/plugin/sdk/sdk_types.go -- PipeDecl, Registration -->

```go
err := p.Run(ctx, &sdk.Registration{
    Commands: []sdk.CommandDecl{
        {Name: "show my-plugin status", ShortHelp: "Show current status"},
    },
    Pipes: []sdk.PipeDecl{{
        Command:     "show my-plugin status",
        Name:        "totals",
        Description: "The counters, without the per-session rows",
        Expansion:   "display sessions-total sessions-established",
    }},
})
```

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Command` | string | Yes | Command path the alias sits on. MUST be one of your own declared commands |
| `Name` | string | Yes | The word typed after the pipe character (kebab-case) |
| `Description` | string | No | The line completion and `command help` show beside the name |
| `Expansion` | string | Yes | The operator chain the name stands for |

Three rules decide whether your command CAN have an alias.
<!-- source: internal/component/command/alias.go -- RegisterPluginAliases, checkAlias -->

- **The pipe layer selects. It does not compute.** `display` keeps named keys and
  drops the rest, and no operator renames a key, adds two numbers, or counts
  matching rows. Your handler MUST emit the aggregate fields beside the detail
  rows, as siblings at one level. `show bgp rpki` is the worked example.
- **The name MUST be free.** It cannot be a built-in operator name, a pipe filter
  name on an overlapping command path, or an alias name the exact same command
  path already carries. One refusal fails your whole Stage 1 registration.
- **The alias takes no argument, and names no other alias.** Its expansion parses
  to built-in operators alone.

The engine stops the alias reaching a command below the one it sits on. Declare
`show my-plugin status detail` in the same message and it inherits nothing.

`ze cli` with no command argument expands the chain in the client process, where
your alias is not registered, so it answers
`pipe error: unknown pipe operator: totals` there. It resolves over
`ze cli -c "<command>"` and in the interactive session a plain ssh client
reaches.
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->

## Complex Responses

Return structured JSON data for API consumers:

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
    if command == "monitor metrics" {
        data := struct {
            Checks    int     `json:"checks"`
            Failures  int     `json:"failures"`
            LatencyMs float64 `json:"latency-ms"`
            LastCheck string  `json:"last-check"`
        }{
            Checks:    state.checks,
            Failures:  state.failures,
            LatencyMs: state.latency,
            LastCheck: state.lastCheck.Format(time.RFC3339),
        }
        return "done", data, nil
    }
    return "error", map[string]string{"error": "unknown command"}, nil
})
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandOutput -->

Note: JSON keys use kebab-case per ze conventions (`"latency-ms"`, not `"latency_ms"`).

## Dispatching Commands to Other Plugins

Plugins can invoke commands on other plugins through the engine's command dispatcher:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand -->

```go
p.OnStarted(func(ctx context.Context) error {
    status, data, err := p.DispatchCommand(ctx, "rib show-in ipv4/unicast")
    if err != nil {
        return err
    }
    fmt.Printf("rib response: status=%s data=%s\n", status, data)
    return nil
})
```

The engine routes the command by longest-match registry lookup and returns the full `{status, data}` response from the target handler.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandInput, DispatchCommandOutput -->

Use `DispatchCommandResult` or `DispatchCommandArgsResult` to retain warning
lines and the optional original `RecordAnswer` counts. A partial or failed
record answer returns a Go error and keeps its completed data and refused
records in `result.Data`. Check that data even when the command failed. Counts
come from the original terminator, rather than from the document's keys.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchResult, DispatchCommandResult, DispatchCommandArgsResult -->

## Help Text

Write the help in the `-cmd` module: `ze:help` for the one-line summary and
`description` for the longer text, on each command container and each
argument leaf. The usage of a refused line is derived from the leaves: its
`fix:` line names each argument by what its type accepts, as above. A plugin
writes no help command of its own. `ShortHelp` and `Description` on the
Stage 1 entry are the help of a command no module defines. Where the module
and the Stage 1 entry both give a text, every surface shows the module's: the
session menu and its explanation, `system command help`, and the MCP, REST and
gRPC command lists. A text the module leaves empty is filled from the entry.
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/doctor_delivery.go -- commandModelHelp.texts -->
<!-- source: internal/test/externalyang/externalyang.go -- commandsText -->
