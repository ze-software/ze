# Plugin Protocol

ZeBGP plugins communicate with the engine via stdin/stdout using a 5-stage text protocol.

## Protocol Stages

### Stage 1: Declaration

Plugin sends declarations on startup:

```
declare encoding text
declare schema <yang-module-escaped>
declare handler <prefix>
declare cmd <command-name>
declare done
```

**Messages:**
- `declare encoding text` - Required, specifies text encoding
- `declare schema <text>` - YANG module (newlines escaped as `\n`)
- `declare handler <prefix>` - Handler path prefix (e.g., "bgp", "bgp.peer")
- `declare cmd <name>` - Command name (e.g., "status")
- `declare done` - End of declarations

### Stage 2: Config

Engine sends initial configuration. Plugin loads into candidate, then commits.

```
# Engine → Plugin
bgp peer create {"address":"192.0.2.1","peer-as":65002}
bgp peer create {"address":"192.0.2.2","peer-as":65003}
bgp commit

# Plugin → Engine
# Success: no output
# Failure: error message, startup aborted
```

**End of stage:**
```
config done
```

### Stage 3: Capability

Plugin confirms capability registration complete:

```
capability done
```

### Stage 4: Registry

Engine confirms all schemas registered:

```
registry done
```

### Stage 5: Ready

Plugin signals ready for commands:

```
ready
```

## Command Format

All commands use the plugin's namespace:

```
<namespace> <path> <action> {json}
<namespace> commit
<namespace> rollback
<namespace> diff
```

**Examples:**
```
bgp peer create {"address":"192.0.2.1","peer-as":65002}
bgp peer modify {"address":"192.0.2.1","hold-time":90}
bgp peer delete {"address":"192.0.2.1"}
bgp commit
bgp rollback
bgp diff
```

| Part | Description |
|------|-------------|
| `<namespace>` | Plugin namespace (e.g., `bgp`, `rib`, `acme-monitor`) |
| `<path>` | Handler path within namespace (e.g., `peer`, `peer-group`) |
| `<action>` | `create`, `modify`, or `delete` |
| `{json}` | Data including keys |

## Candidate/Running Model

Plugins maintain two configuration states:

```
┌─────────────────┐     ┌─────────────────┐
│    Running      │     │   Candidate     │
│  (committed)    │     │   (pending)     │
├─────────────────┤     ├─────────────────┤
│ Active config   │     │ Uncommitted     │
│ Last commit     │     │ changes         │
└─────────────────┘     └─────────────────┘
```

| Command | Effect |
|---------|--------|
| `<ns> ... create/modify/delete` | Modifies candidate |
| `<ns> commit` | Diff candidate vs running → verify → apply → candidate becomes running |
| `<ns> rollback` | Discard candidate, revert to running |
| `<ns> diff` | Show pending changes (candidate vs running) |

**Commit flow:**
1. Compute diff between candidate and running
2. Call verify handlers for each change
3. If all verify pass, call apply handlers
4. Candidate becomes new running

## Command Loop

After ready, plugin handles commands:

```
# Engine → Plugin
#<serial> <namespace> <command> [args...]

# Plugin → Engine
@<serial> ok [json-data]
@<serial> error <message>
```

**Serial numbers:**
- Engine uses alpha serials (a-j)
- Plugin uses numeric serials for callbacks

**Shutdown:**
```json
{"shutdown":true}
```

## Message Flow Example

```
Plugin                                        Engine
   │                                             │
   │── declare encoding text ───────────────────>│
   │── declare schema module... ────────────────>│
   │── declare handler bgp ─────────────────────>│
   │── declare handler bgp.peer ────────────────>│
   │── declare cmd status ──────────────────────>│
   │── declare done ────────────────────────────>│
   │                                             │
   │<── bgp peer create {"address":"192.0.2.1"}──│
   │<── bgp peer create {"address":"192.0.2.2"}──│
   │<── bgp commit ─────────────────────────────>│
   │    (verify all, then apply all)             │
   │<── config done ─────────────────────────────│
   │                                             │
   │── capability done ─────────────────────────>│
   │                                             │
   │<── registry done ───────────────────────────│
   │                                             │
   │── ready ───────────────────────────────────>│
   │                                             │
   │<── #a bgp status ──────────────────────────>│
   │── @a ok {"peers":2,"established":2} ───────>│
   │                                             │
   │<── #b bgp peer create {"address":"10.0.0.1"}│
   │<── #c bgp commit ───────────────────────────│
   │── @b ok ───────────────────────────────────>│
   │── @c ok ───────────────────────────────────>│
   │                                             │
   │<── {"shutdown":true} ───────────────────────│
   │    (plugin exits)                           │
```

## Error Handling

**Declaration errors:**
- Invalid YANG schema → startup fails

**Commit errors:**
- Verify handler returns error → commit rejected, candidate unchanged
- Apply handler returns error → partial apply (logged), may need recovery

**Command errors:**
- Return error → `@serial error message`

**Fatal errors:**
- Plugin should exit with non-zero code
- Engine will restart or report failure
