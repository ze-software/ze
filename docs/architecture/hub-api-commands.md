# Hub API Commands Reference

**Status:** Design specification for Hub Architecture

This document describes all commands in the Hub-based architecture, organized by protocol stage and namespace.

---

## Command Format

All commands follow the standard IPC protocol format:

```
#serial namespace verb [args...]     # Request (expects response)
namespace verb [args...]             # Fire-and-forget (no response)
@serial status [data]                # Response
```

**Status values:** `done`, `error`, `warning`, `ack`

---

## Stage 1: Declaration (Plugin → Hub)

During Stage 1, plugins declare their capabilities to the Hub.

### declare schema (NEW - consistent with declare cmd)

| Command | Description |
|---------|-------------|
| `declare schema module <name>` | YANG module name |
| `declare schema namespace <uri>` | YANG namespace URI |
| `declare schema handler <path>` | Config handler path (e.g., `bgp`, `bgp.peer`) |
| `declare schema yang <<EOF...EOF` | YANG content via heredoc |

### declare (EXISTING)

| Command | Description |
|---------|-------------|
| `declare cmd <name>` | Register a runtime command (e.g., `bgp peer list`) |
| `declare encoding <type>` | Set event encoding: `json` or `text` |
| `declare conf <pattern>` | Register config pattern interest |
| `declare receive <type>` | Register event interest |
| `declare done` | End of declarations (barrier) |

**Example - BGP subsystem:**
```
declare schema module ze-bgp
declare schema namespace urn:ze:bgp
declare schema handler bgp
declare schema handler bgp.peer
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}
EOF
declare cmd bgp peer list
declare cmd bgp peer show
declare encoding json
declare done
```

**Example - Third-party plugin:**
```
declare schema module acme-monitor
declare schema namespace urn:acme:monitor
declare schema handler acme-monitor
declare schema yang <<EOF
module acme-monitor { ... }
EOF
declare cmd acme monitor status
declare done
```

**Why `declare schema` prefix:**
- Consistent with existing `declare cmd` pattern
- Plugin commands can't accidentally shadow schema declarations
- Clear separation: `declare schema ...` vs `declare cmd ...`

---

## Schema Retrieval (CLI Command)

Schema commands are real CLI commands - usable both by Config Reader (programmatic) and humans (debugging).

### Schema CLI commands

Plugins declare schema CLI commands during Stage 1 like any other command:

| Who | Action |
|-----|--------|
| Plugin | Declares: `declare cmd bgp schema show` |
| Hub | Routes command to plugin |
| Config Reader | Can call command to get YANG |
| Human | Runs same command for debugging |

**Purpose:** Retrieve YANG schema on-demand via standard CLI.

### CLI Usage (Human)

```bash
# View schema for debugging
$ ze bgp schema show
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
  ...
}

# Validate config against schema
$ ze bgp schema validate /etc/ze/config.conf
OK: config valid

$ ze bgp schema validate /etc/ze/bad.conf
ERROR: line 42: unknown leaf 'typo' in bgp.peer

# Shell autocomplete (uses schema for suggestions)
$ ze bgp peer <TAB>
address=     peer-as=     group=       hold-time=

# Check what handlers a schema provides
$ ze bgp schema handlers
bgp
bgp.peer
bgp.peer-group
```

### Programmatic Usage (Config Reader)

Config Reader runs the same commands:
```bash
$ ze bgp schema show        # Get YANG for validation
$ ze rib schema show        # Get RIB schema
```

### Key Points

- **Same command for both uses** - no separate "internal" vs "external" API
- **Plain text output** - YANG on stdout, easy to pipe/debug
- **Enables tooling** - autocomplete, validation, linting all use schema commands
- **Human debuggable** - developers can run commands directly to troubleshoot

---

## Schema Operations (validate, complete, show)

CLI structure for schema operations:

```
ze <subsystem> schema <action> [args...]
```

| Component | Description |
|-----------|-------------|
| `ze` | Core engine |
| `<subsystem>` | bgp, rib, system, etc. |
| `schema` | Schema operation namespace |
| `<action>` | show, handlers, validate, complete |

### Actions

| Action | Purpose | Output |
|--------|---------|--------|
| `show` | Display YANG schema | YANG text on stdout |
| `handlers` | List handler paths | One path per line |
| `validate <path> <value>` | Check if value is valid | exit 0 = valid, exit 1 = invalid |
| `complete <path> [partial]` | List valid values | One value per line |

### Examples

```bash
# Show YANG schema for BGP subsystem
$ ze bgp schema show
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}

# List handlers
$ ze bgp schema handlers
bgp
bgp.peer
bgp.peer-group

# Validate peer-group name exists
$ ze bgp schema validate peer-group name upstream
$ echo $?
0  # valid

$ ze bgp schema validate peer-group name nonexistent
$ echo $?
1  # invalid

# Complete peer-group names
$ ze bgp schema complete peer-group name
upstream
downstream
internal

# Complete with partial match
$ ze bgp schema complete peer-group name up
upstream

# Complete interface names (via system subsystem)
$ ze system schema complete interface name
eth0
eth1
lo
```

### Cross-Subsystem References

When BGP references system interfaces (leafref to another subsystem):

```yang
# In ze-bgp.yang
leaf update-source {
  type leafref {
    path "/system/interface/name";
  }
}
```

Validation goes through the subsystem that owns the data:
```bash
# System owns interface data
$ ze system schema validate interface name eth0
```

### Usage: Config Validation

Config Reader validates leafrefs by constructing command from YANG path:

```
YANG:    type leafref { path "/bgp/peer-group/name"; }
Value:   "upstream"
Command: ze bgp schema validate peer-group name upstream
Result:  exit 0 → valid
```

### Usage: Shell Autocomplete

```bash
# User types:
$ ze bgp peer add 192.0.2.1 group <TAB>

# Shell runs:
$ ze bgp schema complete peer-group name

# Returns:
upstream
downstream
internal
```

### Declaration

Plugins declare schema during Stage 1:

```
declare schema module ze-bgp
declare schema handler bgp
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp { ... }
EOF
declare done
```

The `ze bgp schema` commands route to BGP subsystem.

---

## Stage 2: Config Delivery (Hub → Plugin)

Hub delivers configuration to plugins (and Config Reader).

### config (to regular plugins)

| Command | Description |
|---------|-------------|
| `config <key> <value>` | Deliver config value |
| `config peer <addr> <key> <value>` | Deliver peer-specific config |
| `config done` | End of config (barrier) |

**Example:**
```
config peer 192.0.2.1 hold-time 90
config peer 192.0.2.1 restart-time 120
config done
```

### config (to Config Reader)

| Command | Description |
|---------|-------------|
| `config schema <module> handlers <list> yang <<EOF...EOF` | Schema module with YANG content |
| `config path <filepath>` | Config file to parse |
| `config done` | End of config delivery |

**Example:**
```
config schema ze-bgp handlers bgp,bgp.peer yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}
EOF
config schema ze-rib handlers rib yang <<EOF
module ze-rib { ... }
EOF
config path /etc/ze/config.conf
config done
```

Hub sends YANG content inline (collected from plugins in Stage 1).

---

## Stage 2: Verify/Apply (Config Reader ↔ Hub ↔ Plugin)

Config Reader validates config and sends verify/apply requests through Hub.

### config verify

| Direction | Command |
|-----------|---------|
| Config Reader → Hub | `#serial config verify handler "<handler>" action <type> path "<full-path>" data '<json>'` |
| Hub → Plugin | `#serial config verify action <type> path "<full-path>" data '<json>'` |
| Plugin → Hub | `@serial done` or `@serial error <message>` |
| Hub → Config Reader | `@serial done` or `@serial error <message>` |

**Action types:**
- `create` - New config block
- `modify` - Changed config block
- `delete` - Removed config block

**Example flow:**
```
# Config Reader asks Hub to verify new peer
Config Reader → Hub:
#1 config verify handler "bgp.peer" action create path "bgp.peer[address=192.0.2.1]" data '{"address":"192.0.2.1","peer-as":65002}'

# Hub routes to BGP plugin
Hub → BGP Plugin:
#a config verify action create path "bgp.peer[address=192.0.2.1]" data '{"address":"192.0.2.1","peer-as":65002}'

# Plugin validates and responds
BGP Plugin → Hub:
@a done

# Hub responds to Config Reader
Hub → Config Reader:
@1 done
```

**Rejection example:**
```
BGP Plugin → Hub:
@a error peer-as cannot equal local-as

Hub → Config Reader:
@1 error peer-as cannot equal local-as
```

### config apply

| Direction | Command |
|-----------|---------|
| Config Reader → Hub | `#serial config apply handler "<handler>" action <type> path "<full-path>" data '<json>'` |
| Hub → Plugin | `#serial config apply action <type> path "<full-path>" data '<json>'` |
| Plugin → Hub | `@serial done` or `@serial error <message>` |

**Note:** Apply is only sent after ALL verify requests pass.

### config complete

| Direction | Command |
|-----------|---------|
| Config Reader → Hub | `#serial config complete` |
| Hub → Config Reader | `@serial done` |
| Hub → All Plugins | `config done` |

Signals config processing is complete. Hub sends `config done` to all plugins.

---

## Stage 3: Capability (Plugin → Hub)

Plugins declare BGP capabilities to inject into OPEN messages.

### capability

| Command | Description |
|---------|-------------|
| `capability hex <code> <value>` | Global capability |
| `capability hex <code> <value> peer <addr>` | Per-peer capability |
| `capability done` | End of capabilities (barrier) |

**Example - GR plugin:**
```
capability hex 64 0078 peer 192.0.2.1
capability hex 64 005a peer 10.0.0.1
capability done
```

---

## Stage 4: Registry (Hub → Plugin)

Hub shares the command registry with plugins.

### registry

| Command | Description |
|---------|-------------|
| `registry cmd <name>` | Registered command |
| `registry done` | End of registry (barrier) |

**Example:**
```
registry cmd bgp peer list
registry cmd bgp peer show
registry cmd rib adjacent status
registry done
```

---

## Stage 5: Ready (Plugin → Hub)

### ready

| Command | Description |
|---------|-------------|
| `ready` | Plugin is ready (barrier) |

After all plugins send `ready`, BGP peers start.

---

## Runtime: System Namespace

### system schema

| Command | Description |
|---------|-------------|
| `system schema list` | List all registered schemas |
| `system schema show <module>` | Show specific schema YANG |
| `system schema handlers` | List handler → plugin mapping |
| `system schema protocol` | Protocol version and format info |

**Example:**
```
#1 system schema list
@1 done ze-bgp:bgp,bgp.peer ze-rib:rib
```

### system (existing)

| Command | Description |
|---------|-------------|
| `system subsystem list` | List available subsystems |
| `system shutdown` | Graceful shutdown |
| `system version software` | ZeBGP version |
| `system version api` | IPC protocol version |

---

## Runtime: Config Namespace

### config reload

| Direction | Command |
|-----------|---------|
| Any → Hub | `#serial config reload` |
| Hub → Config Reader | `#serial config reload` |
| Config Reader → Hub | `@serial done` |

Triggers config file re-read. Config Reader diffs current vs new and sends verify/apply for changes.

### config validate

| Command | Description |
|---------|-------------|
| `#serial config validate` | Validate current config without applying |

Returns validation errors if any.

---

## Runtime: Plugin Namespace (Existing)

| Command | Description |
|---------|-------------|
| `plugin session ready` | Signal plugin init complete |
| `plugin session ping` | Health check (returns PID) |
| `plugin session bye` | Disconnect |
| `plugin command list` | List plugin commands |
| `plugin command help "<cmd>"` | Command details |

---

## Runtime: BGP Namespace (Existing)

### Peer operations

| Command | Description |
|---------|-------------|
| `bgp peer <sel> list` | List matching peers |
| `bgp peer <sel> show` | Show peer details |
| `bgp peer <sel> teardown [subcode]` | Graceful close |
| `bgp peer <sel> update <enc> ...` | Announce/withdraw routes |
| `bgp peer <sel> ready` | Signal peer replay complete |

**Selector patterns:** `*` (all), `<ip>` (specific), `!<ip>` (all except)

### Cache operations

| Command | Description |
|---------|-------------|
| `bgp cache list` | List cached message IDs |
| `bgp cache <id> retain` | Keep in cache |
| `bgp cache <id> release` | Allow eviction |
| `bgp cache <id> expire` | Remove immediately |
| `bgp cache <id> forward <sel>` | Forward to peers |

### Plugin configuration

| Command | Description |
|---------|-------------|
| `bgp plugin encoding json\|text` | Set event encoding |
| `bgp plugin format hex\|base64\|parsed\|full` | Set wire bytes format |
| `bgp plugin ack sync\|async` | Set ACK timing |

---

## Event Subscription (Existing)

| Command | Description |
|---------|-------------|
| `subscribe [peer <sel>] <ns> event <type> [direction <dir>]` | Subscribe to events |
| `unsubscribe [peer <sel>] <ns> event <type> [direction <dir>]` | Unsubscribe |

**Example:**
```
subscribe bgp event update direction received
subscribe peer * bgp event state
subscribe rib event cache
```

---

## Summary: New Commands for Hub Architecture

| Command | Stage/Context | Direction |
|---------|---------------|-----------|
| `declare schema module <name>` | Stage 1 | Plugin → Hub |
| `declare schema namespace <uri>` | Stage 1 | Plugin → Hub |
| `declare schema handler <path>` | Stage 1 | Plugin → Hub |
| `declare schema yang <<EOF...EOF` | Stage 1 | Plugin → Hub |
| `config schema <module> handlers <list> yang <<EOF...EOF` | Stage 2 | Hub → Config Reader |
| `config verify handler "<handler>" action <type> ...` | Stage 2 | Config Reader → Hub |
| `config verify action <type> path "<path>" ...` | Stage 2 | Hub → Plugin |
| `config apply handler "<handler>" action <type> ...` | Stage 2 | Config Reader → Hub |
| `config apply action <type> path "<path>" ...` | Stage 2 | Hub → Plugin |
| `config complete` | Stage 2 | Config Reader → Hub |
| `config reload` | Runtime | Any → Hub |
| `system schema list\|show\|handlers\|protocol` | Runtime | Any → Hub |

**Key design:**
- `declare schema` prefix matches `declare cmd` pattern
- Hub stores YANG content, passes to Config Reader
- Handler routing uses longest prefix match

---

## Message Flow Summary

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              STARTUP                                        │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Stage 1: DECLARATION                                                      │
│    Plugin → Hub: declare schema module ze-bgp                              │
│    Plugin → Hub: declare schema handler bgp                                │
│    Plugin → Hub: declare schema handler bgp.peer                           │
│    Plugin → Hub: declare schema yang <<EOF...EOF                           │
│    Plugin → Hub: declare cmd bgp peer list                                 │
│    Plugin → Hub: declare done                                              │
│                                                                            │
│  ─── BARRIER ───                                                           │
│                                                                            │
│  Stage 2: CONFIG (via Config Reader)                                       │
│    Hub → ConfigReader: config schema ze-bgp handlers bgp,bgp.peer          │
│                        yang <<EOF...EOF                                    │
│    Hub → ConfigReader: config path /etc/ze/config.conf                     │
│    Hub → ConfigReader: config done                                         │
│                                                                            │
│    ConfigReader parses config against YANG                                 │
│                                                                            │
│    ConfigReader → Hub: #1 config verify handler "bgp.peer" ...             │
│    Hub → Plugin: #a config verify action create ...                        │
│    Plugin → Hub: @a done                                                   │
│    Hub → ConfigReader: @1 done                                             │
│                                                                            │
│    ConfigReader → Hub: #2 config apply handler "bgp.peer" ...              │
│    Hub → Plugin: #b config apply action create ...                         │
│    Plugin → Hub: @b done                                                   │
│    Hub → ConfigReader: @2 done                                             │
│                                                                            │
│    ConfigReader → Hub: #3 config complete                                  │
│    Hub → All Plugins: config done                                          │
│                                                                            │
│  ─── BARRIER ───                                                           │
│                                                                            │
│  Stages 3-5: CAPABILITY, REGISTRY, READY (unchanged)                       │
│                                                                            │
│  ─── BGP PEERS START ───                                                   │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│                              RUNTIME                                        │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Schema Discovery:                                                         │
│    CLI → Hub: #1 system schema list                                        │
│    Hub → CLI: @1 done {"schemas":[...]}                                    │
│                                                                            │
│  Config Reload:                                                            │
│    CLI → Hub: #1 config reload                                             │
│    Hub → ConfigReader: #r1 config reload                                   │
│    ConfigReader → Hub: @r1 done                                            │
│    (verify/apply flow for changes)                                         │
│    Hub → CLI: @1 done                                                      │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

---

**Last Updated:** 2026-01-24
