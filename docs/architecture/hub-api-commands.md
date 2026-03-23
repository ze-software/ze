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
<!-- source: internal/core/ipc/ -- IPC protocol format -->

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
| `declare wants config <root>` | Request config subtree delivery (e.g., `bgp`) |
| `declare receive <type>` | Register event interest |
| `declare done` | End of declarations (barrier) |
<!-- source: internal/component/plugin/registration.go -- declaration parsing -->

**Example - BGP subsystem:**
```
declare schema module ze-bgp-conf
declare schema namespace urn:ze:bgp
declare schema handler bgp
declare schema handler bgp.peer
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
  ...
}
EOF
declare cmd bgp peer list
declare cmd bgp peer detail
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

Schema commands are real CLI commands - usable both by Hub (programmatic) and humans (debugging).

### Schema CLI commands

Plugins declare schema CLI commands during Stage 1 like any other command:

| Who | Action |
|-----|--------|
| Plugin | Declares: `declare cmd bgp schema show` |
| Hub | Routes command to plugin |
| Hub | Can call command to get YANG |
| Human | Runs same command for debugging |

**Purpose:** Retrieve YANG schema on-demand via standard CLI.

### CLI Usage (Human)

```bash
# View schema for debugging
$ ze bgp schema show
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
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

### Programmatic Usage (Hub)

Hub runs the same commands:
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
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
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
# In ze-bgp-conf.yang
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

Hub validates leafrefs by constructing command from YANG path:

```
YANG:    type leafref { path "/bgp/peer-group/name"; }
Value:   "upstream"
Command: ze bgp schema validate peer-group name upstream
Result:  exit 0 → valid
```

### Usage: Shell Autocomplete

```bash
# User types:
$ ze set bgp peer 192.0.2.1 with asn <TAB>

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
declare schema module ze-bgp-conf
declare schema handler bgp
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp-conf { ... }
EOF
declare done
```

The `ze bgp schema` commands route to BGP subsystem.

---

## Stage 2: Config Notification (Hub → Plugin)

**Pull model:** Hub notifies plugins, plugins query for config. Hub never pushes config data.

### Notification Commands (Hub → Plugin)

| Command | Description |
|---------|-------------|
| `#serial config verify` | Notify plugin to verify pending changes |
| `#serial config apply` | Notify plugin to apply verified changes |
| `config done` | All plugins finished (barrier) |

### Query Commands (Plugin → Hub)

| Command | Description |
|---------|-------------|
| `#serial query config live path "<path>"` | Query running config |
| `#serial query config edit path "<path>"` | Query candidate config |

### Query Response (Hub → Plugin)

| Response | Description |
|----------|-------------|
| `@serial done data '<json>'` | Config data as JSON |
| `@serial error <message>` | Query failed |

---

## Stage 2: Verify/Apply Flow (VyOS-inspired)

Hub notifies plugins, plugins pull config and compute diff.

### config verify

| Direction | Command |
|-----------|---------|
| Hub → Plugin | `#serial config verify` |
| Plugin → Hub | `#serial query config live path "..."` |
| Hub → Plugin | `@serial done data '{...}'` |
| Plugin → Hub | `#serial query config edit path "..."` |
| Hub → Plugin | `@serial done data '{...}'` |
| Plugin → Hub | `@serial done` or `@serial error <message>` |

**Example flow:**
```
# Hub notifies BGP plugin to verify
Hub → BGP Plugin:
#1 config verify

# Plugin queries current (live) config
BGP Plugin → Hub:
#2 query config live path "bgp"

Hub → BGP Plugin:
@2 done data '{"local-as": 65001, "peer": [...]}'

# Plugin queries candidate (edit) config
BGP Plugin → Hub:
#3 query config edit path "bgp"

Hub → BGP Plugin:
@3 done data '{"local-as": 65001, "peer": [...new peer...]}'

# Plugin computes diff, validates, responds
BGP Plugin → Hub:
@1 done
```

**Rejection example:**
```
BGP Plugin → Hub:
@1 error peer-as cannot equal local-as
```

### config apply

| Direction | Command |
|-----------|---------|
| Hub → Plugin | `#serial config apply` |
| Plugin → Hub | `#serial query config edit path "..."` |
| Hub → Plugin | `@serial done data '{...}'` |
| Plugin → Hub | `@serial done` or `@serial error <message>` |

**Note:** Apply only sent after ALL plugins verify successfully.

### config done

| Direction | Command |
|-----------|---------|
| Hub → All Plugins | `config done` |

Signals config cycle complete. Hub promotes edit to live.

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
registry cmd bgp peer detail
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
| `system version software` | Ze version |
| `system version api` | IPC protocol version |

---

## Runtime: Config Namespace

### config reload

| Direction | Command |
|-----------|---------|
| CLI → Hub | `#serial config reload` |
| Hub → CLI | `@serial done` |

Triggers config file re-read. Hub re-parses, stores as edit, runs verify/apply cycle with plugins.

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
<!-- source: internal/component/bgp/reactor/recent_cache.go -- cache operations -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- command dispatch -->

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
<!-- source: internal/component/plugin/events.go -- event subscription -->

**Example:**
```
subscribe bgp event update direction received
subscribe peer * bgp event state
subscribe rib event cache
```

---

## Summary: Commands for Hub Architecture

| Command | Stage/Context | Direction |
|---------|---------------|-----------|
| `declare schema module <name>` | Stage 1 | Plugin → Hub |
| `declare schema namespace <uri>` | Stage 1 | Plugin → Hub |
| `declare schema handler <path>` | Stage 1 | Plugin → Hub |
| `declare schema yang <<EOF...EOF` | Stage 1 | Plugin → Hub |
| `declare priority <number>` | Stage 1 | Plugin → Hub |
| `#N config verify` | Stage 2 | Hub → Plugin |
| `#N config apply` | Stage 2 | Hub → Plugin |
| `#N query config live/edit path "..."` | Stage 2 | Plugin → Hub |
| `@N done data '{...}'` | Stage 2 | Hub → Plugin |
| `config done` | Stage 2 | Hub → All Plugins |
| `config reload` | Runtime | CLI → Hub |
| `config commit` | Runtime | CLI → Hub |
| `system schema list\|show\|handlers\|protocol` | Runtime | Any → Hub |

**Key design:**
- Pull model: Hub notifies plugins, plugins query for config
- Hub never pushes config data
- Handler routing uses longest prefix match
<!-- source: internal/component/plugin/registration.go -- handler registration and routing -->

---

## Message Flow Summary

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              STARTUP                                        │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Stage 1: DECLARATION                                                      │
│    Plugin → Hub: declare schema module ze-bgp-conf                              │
│    Plugin → Hub: declare schema handler bgp                                │
│    Plugin → Hub: declare schema handler bgp.peer                           │
│    Plugin → Hub: declare priority 100                                      │
│    Plugin → Hub: declare schema yang <<EOF...EOF                           │
│    Plugin → Hub: declare cmd bgp peer list                                 │
│    Plugin → Hub: declare done                                              │
│                                                                            │
│  ─── BARRIER: All plugins declared ───                                     │
│                                                                            │
│  Stage 2: INITIAL COMMIT (pull model)                                      │
│    Hub parses config against combined YANG                                 │
│    Hub stores config as edit state (live is empty)                         │
│                                                                            │
│    For each plugin (by priority):                                          │
│      Hub → Plugin: #1 config verify                                        │
│      Plugin → Hub: #2 query config live path "bgp"                         │
│      Hub → Plugin: @2 done data '{}'                                       │
│      Plugin → Hub: #3 query config edit path "bgp"                         │
│      Hub → Plugin: @3 done data '{...}'                                    │
│      Plugin → Hub: @1 done                                                 │
│                                                                            │
│    All verify pass:                                                        │
│      Hub → Plugin: #4 config apply                                         │
│      Plugin → Hub: #5 query config edit path "bgp"                         │
│      Hub → Plugin: @5 done data '{...}'                                    │
│      Plugin → Hub: @4 done                                                 │
│                                                                            │
│    Hub: edit becomes live                                                  │
│    Hub → All Plugins: config done                                          │
│                                                                            │
│  ─── BARRIER: Initial commit complete ───                                  │
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
│  Config Reload (SIGHUP or CLI):                                            │
│    CLI → Hub: #1 config reload                                             │
│    Hub re-reads config, stores as edit                                     │
│    (verify/apply cycle with all plugins)                                   │
│    Hub → CLI: @1 done                                                      │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

---

**Last Updated:** 2026-02-02
