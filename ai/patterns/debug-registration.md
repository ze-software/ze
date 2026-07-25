# Pattern: Debug Flag Registration

How a plugin declares its debug surface so operators can use
`ze debug <module> flag <flag>` and `ze debug <module> scope <kind> <value>`.

## What Each Plugin Implements

One file: `<owner>/yang/register_debug.go`. That is the entire contract.

```go
// internal/component/<owner>/yang/register_debug.go
package yang

import debugyang "github.com/ze-software/ze/internal/component/debug/yang"

func init() {
    debugyang.RegisterModule(debugyang.Module{
        Name:   "ze-<owner>-debug",
        Prefix: "<subsystem-prefix>",
        Flags:  []string{"flag1", "flag2", ...},
        Scopes: []string{"scope-kind1", "scope-kind2", ...},
    })
}
```

### Fields

| Field | Purpose | Example |
|-------|---------|---------|
| `Name` | Human identifier for the module | `"ze-bgp-debug"` |
| `Prefix` | Subsystem hierarchy prefix this module covers | `"bgp"` matches `bgp`, `bgp.reactor`, `bgp.reactor.peer` |
| `Flags` | Valid flag names for `debug <module> flag <flag>` | `[]string{"open", "update", "keepalive"}` |
| `Scopes` | Valid scope kinds for `debug <module> scope <kind> <value>` | `[]string{"neighbor", "group", "direction"}` |

### What Happens

1. Plugin's `yang/register_debug.go` runs during `init()` (package is already blank-imported)
2. The debug command validates flag/scope names against registered modules
3. Invalid flags are rejected with an error listing valid options
4. If no module covers a subsystem prefix, all flags are accepted (progressive rollout)

### Existing Registration (BGP)

`internal/component/bgp/yang/register_debug.go`:

- **Prefix:** `bgp` (covers bgp.reactor, bgp.server, bgp.config, etc.)
- **Flags:** open, update, keepalive, notification, refresh, route, policy, fsm, timer, socket, config, graceful-restart, bfd, capability
- **Scopes:** neighbor, group, direction

### Adding Debug to a New Plugin

1. Create `<owner>/yang/register_debug.go` with the registration call above
2. Create `<owner>/yang/debug_register_test.go` asserting the flags are registered
3. The test uses `debugyang.HasModule` and `debugyang.ValidateFlag` to verify

No changes needed to the debug command, CLI dispatch, or any central package.
The registration pattern is the same as config YANG, doctor checks, and capabilities.

### Scope Kinds

Scope kinds are plugin-defined. Common patterns:

| Kind | Used By | Meaning |
|------|---------|---------|
| `neighbor` | BGP | Filter to a specific peer address |
| `group` | BGP | Filter to a peer group |
| `direction` | Wire protocols (BGP, L2TP, BFD) | Filter to send or receive |
| `session` | L2TP, subscriber | Filter to a specific session |
| `interface` | iface, traffic | Filter to a specific interface |

Direction is NOT a first-class grammar element. It is a scope kind that
protocol-oriented plugins declare. Non-protocol plugins omit it.

### How Debug Filtering Works at Runtime

When an operator runs `ze debug bgp.reactor flag update scope direction receive`:

1. Profile adds flag `update` and scope `direction=receive` for `bgp.reactor`
2. `applyProfile()` calls `slogutil.ConfigureFilter()` on each matching subsystem
3. The `filterHandler` wrapping every logger checks record attributes against active filters
4. A log call `logger.Debug("msg", "flag", "update", "direction", "receive")` passes
5. A log call `logger.Debug("msg", "flag", "update", "direction", "send")` is blocked
6. A log call `logger.Debug("msg", "flag", "keepalive")` is blocked (wrong flag)

When no filters are configured (the common case), the filterHandler delegates
directly to the base handler with a single nil-check branch (zero overhead).
