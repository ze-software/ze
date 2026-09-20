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

## Decision: the KeepAlive Time is read when the session opens

RFC 5036 section 3.5.3 exchanges the KeepAlive Time one time, in the
Initialization message, so a running session cannot renegotiate it. Ze reads
`ldp/keepalive-time` at the moment it builds a session, from the config in force
then, and never afterwards. A reload therefore reaches the sessions that open
after it, and an established session keeps the value it negotiated. Tearing
sessions down to apply a new timer was rejected: it drops every LSP the peer
carries, which is a large price for a timer change.

The value is read from the active config rather than from the copy the discovery
goroutine started with. Discovery reconciles per interface, and an interface that
is already running keeps its goroutine, so a reload that changes only a timer
would otherwise reach nothing.

<!-- source: internal/plugins/ldp/register.go -- sessionConfigForAdj, startSessionForAdj -->
<!-- source: internal/plugins/ldp/session.go -- SessionConfig, NewSession -->

## Decision: an unacceptable Initialization is NAK'd, never clamped

`processMessages` checks the Common Session Parameters before `handleInit` reads
them, and refuses two values: a Protocol Version other than 1, and a KeepAlive
Time of 0. Each refusal sends the Notification RFC 5036 section 3.9 names for it,
then returns the cause, which ends the read loop and closes the connection.

Clamping the KeepAlive Time to a floor was rejected. RFC 5036 section 3.5.3 makes
the field a "Two octet unsigned non zero integer", and section 3.5.1.2.5 makes an
unacceptable session parameter a fatal error, so the RFC's answer is to reject the
session rather than to invent a value the peer never proposed. The defect that
clamping would have hidden is worth naming: the negotiation takes the smaller of
the two proposals, so a peer sending 0 won it, the hold time became 0, and the
next read deadline of now+0 timed out and was reported as a keepalive expiry.

`sendNotification` bounds its write with a deadline. Every status ze sends is
fatal, so a peer that has stopped reading must not be able to hold the read loop
open by never draining its receive window.

This is the only path on which ze emits a Notification. Every other fatal error
still closes the session with nothing on the wire, which `rfc/short/rfc5036.md`
records as the remaining half of RFC5036-2.5.3-2 and RFC5036-3.5.1-1.

<!-- source: internal/plugins/ldp/session.go -- processMessages, rejectInit, sendNotification -->
<!-- source: internal/plugins/ldp/wire.go -- encodeNotification, statusSessionRejectedBadKeepaliveTime -->

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
