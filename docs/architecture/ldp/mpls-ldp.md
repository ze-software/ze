# LDP Architecture

Ze implements LDP (RFC 5036): UDP hello discovery, a TCP session FSM, a Label
Information Base, and a dataplane write through the MPLS FIB event bus.

| Concern | File |
|---------|------|
| UDP hello discovery | `discovery.go` |
| Per-interface discovery lifecycle | `discovery_manager.go` |
| Session FSM | `session.go` |
| Wire codec | `wire.go` |
| Label Information Base | `lib.go` |
| Local FEC origination | `local.go` |
| Dataplane write | `fib.go` |
| Component registration and SDK lifecycle | `register.go` |
| Event bus types | `events.go` |
| `show ldp ...` proxies | `cmd_show.go` |
| Port 646 readiness check | `doctor.go` |

## Decision: reconcile discovery, do not restart the engine

A config reload diffs the desired interface set against the running per-interface
discovery goroutines. Added interfaces start, removed interfaces cancel. The
alternative, restarting the whole engine, was rejected because it disturbs
sessions on links that did not change.

Stopping discovery is enough to tear a session down. With no hellos the
neighbor's adjacency ages out through the existing hold timer and the session
follows. No per-interface session teardown exists, and none is needed.

<!-- source: internal/plugins/ldp/discovery_manager.go -- reconcile, startFn -->

The discovery start function is an injected field, so reconcile is unit-testable
with no multicast I/O.

## Decision: the doctor check is self-contained

The port 646 readiness check probes a UDP and a TCP bind through a test seam and
registers through the plugin's own `Registration.DoctorChecks`, not a central
check list. Removing the LDP plugin removes its check.

<!-- source: internal/plugins/ldp/doctor.go -- the port 646 readiness check -->

## Trap: the delivered config shape

`Tree.ToMap` and `BuildPluginConfigSections` deliver plugin config
**root-wrapped** (`{"ldp": {...}}`), with **numbers as strings** (`"5"`),
leaf-lists as **a scalar or an array** depending on element count, and YANG lists
as **maps keyed by the list key**. A parser that expects unwrapped, numeric or
array data produces an empty config. The engine then logs that no LSR ID is
configured and idles, with no error anywhere.

A unit test that builds the engine config struct directly cannot see this. Pin
the delivered shape in a parser test instead.

## Trap: a show proxy must forward, never re-dispatch

A builtin show handler that re-dispatches its own command string re-matches
itself and recurses until the stack overflows. Register the command with
`RPCRegistration.PluginCommand` and forward with `Dispatcher.ForwardToPlugin`.

<!-- source: internal/plugins/ldp/cmd_show.go -- the show ldp proxy registration -->

## Trap: the config schema is top-level

LDP configuration is a top-level `ldp { ... interfaces <name> }` block. A
`protocol { ldp { interface X } }` form was written into two functional tests and
the YANG never defined it.
