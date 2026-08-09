# BGP Reactions to Interface Events

BGP assumed a configured IP always existed. Listeners were created at startup
and never adjusted. When an address disappeared, sessions failed with opaque TCP
errors instead of shutting down cleanly. The reactor now subscribes to interface
events and reacts: start a listener when an address appears, drain sessions when
one disappears.

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- interface event handler, address matching, drain -->

## The decisions

**Subscribe to the `interface/` event prefix rather than poll.** This reuses the
reactor's existing subscription pattern, and prefix matching covers every
interface subtopic with one registration.

**Listener map keys come from `net.JoinHostPort(addr, port)`, never from string
concatenation.** Concatenation produces `::1:179` for IPv6 instead of
`[::1]:179`, and every lookup then misses. An earlier hand-written helper had
exactly that defect.

**Addresses are normalized with `addr.Unmap()` before comparison.** Netlink
delivers an IPv4 address as `::ffff:10.0.0.1` at times. Without `Unmap`, the
match against a peer `LocalAddress` of `10.0.0.1` fails in silence.

**A disappearing address drains gracefully with NOTIFICATION cease subcode 6,
"Other Configuration Change", per RFC 4486.** An immediate close was rejected:
the drain gives peers time to re-converge before TCP drops.

**`local-address` accepts an interface unit reference such as `eth0.0`, not an
IP only.** BGP resolves the unit's primary address and re-resolves it on an
address event. This is the VyOS `update-source` behavior.

## Constraints

**A reactor handler must never hold `r.mu` across an event-bus operation.** The
shutdown path deadlocked between `r.mu`, held during shutdown, and the delivery
worker waiting to deliver to the reactor. The unsubscribe must happen before
`r.mu` is locked.

**Peers that share a `LocalAddress` share one listener.** It is created once, on
the first matching address-added event.

A new listener starts before the old one drains, so sessions survive an
interface migration.
