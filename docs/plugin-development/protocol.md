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
- `declare handler <prefix>` - Handler path prefix (e.g., "acme-monitor")
- `declare cmd <name>` - Command name (e.g., "status")
- `declare done` - End of declarations

### Stage 2: Config

Engine sends config verify requests, plugin receives and processes:

```
# Engine → Plugin
config verify action <action> path "<path>" data '<json>'

# Plugin → Engine (implicit via verify handler return)
# Success: no output, failure: Run() returns error
```

**Actions:**
- `create` - New config block
- `modify` - Changed config block
- `delete` - Removed config block

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

## Command Loop

After ready, plugin handles commands:

```
# Engine → Plugin
#<serial> <command> [args...]

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
Plugin                              Engine
   │                                   │
   │── declare encoding text ─────────>│
   │── declare schema module... ──────>│
   │── declare handler acme-monitor ──>│
   │── declare cmd status ────────────>│
   │── declare done ──────────────────>│
   │                                   │
   │<── config verify action create ───│
   │    (verify handler runs)          │
   │<── config done ───────────────────│
   │                                   │
   │── capability done ───────────────>│
   │                                   │
   │<── registry done ─────────────────│
   │                                   │
   │── ready ─────────────────────────>│
   │                                   │
   │<── #a acme-monitor status ────────│
   │── @a ok {"status":"running"} ────>│
   │                                   │
   │<── {"shutdown":true} ─────────────│
   │    (plugin exits)                 │
```

## Error Handling

**Declaration errors:**
- Invalid YANG schema → startup fails

**Verify errors:**
- Handler returns error → config rejected, startup aborted

**Command errors:**
- Return error → `@serial error message`

**Fatal errors:**
- Plugin should exit with non-zero code
- Engine will restart or report failure
