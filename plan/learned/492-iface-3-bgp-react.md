# 492 -- BGP Reactions to Interface Events

## Context

BGP assumed configured IPs always existed. There was no dynamic listener management -- listeners were created at startup and never adjusted. If an IP disappeared (interface down, address removed), BGP sessions would fail with opaque TCP errors rather than graceful shutdown. This phase makes the BGP reactor subscribe to `interface/` Bus events and react: start listeners when addresses appear, drain sessions when addresses disappear.

## Decisions

- `OnBusEvent("interface/", handler)` registration over direct polling: reuses the reactor Bus subscription pattern from spec-reactor-bus-subscribe, with prefix-based matching for all interface subtopics.
- `net.JoinHostPort(addr, port)` for listener map keys over string concatenation: produces correct bracket-wrapped keys for IPv6 (e.g., `[::1]:179`), preventing key mismatches.
- `addr.Unmap()` for IPv4-mapped IPv6 normalization over raw comparison: netlink sometimes delivers IPv4 addresses as `::ffff:10.0.0.1`. Without Unmap, matching against peer `LocalAddress` "10.0.0.1" fails silently.
- Graceful drain with NOTIFICATION cease subcode 6 ("Other Configuration Change") over immediate close: follows RFC 4486, gives peers time to re-converge before TCP drops.
- `local-address` accepts interface unit references (`eth0.0`) over IP-only: enables VyOS-style `update-source` where BGP resolves the unit's primary IP and re-resolves on address events.

## Consequences

- BGP sessions now survive interface migrations -- new listener starts before old one drains.
- Multiple peers sharing the same `LocalAddress` share a single listener, created once on first `addr/added` match.
- Reactor handler must never hold `r.mu` during Bus operations to avoid deadlock with the Bus delivery worker.

## Gotchas

- Original `itoa()` helper function produced incorrect listener map keys for IPv6 addresses -- bare colons without brackets caused lookup failures. Replaced with `net.JoinHostPort`.
- Reactor cleanup deadlocked between `r.mu` (held during shutdown) and Bus delivery worker (waiting to deliver to the reactor's consumer). Fix: `unsubscribeBus` must happen before `r.mu.Lock` in the shutdown sequence.
- `bgp/listener/ready` topic was initially never published -- migration dead code depended on it but the publish call was missing from the listener startup path. Had to wire the publish into `startListenerForAddressPort` completion.

## Files

- `internal/component/bgp/reactor/reactor_iface.go` -- interface event handler, addr matching, drain logic
- `internal/component/bgp/reactor/reactor.go` -- `OnBusEvent` registration, listener start/stop
- `internal/component/bgp/reactor/listener.go` -- dynamic listener management
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- `local-address` accepts interface unit refs
