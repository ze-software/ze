# Shared DNS Server Harness

`internal/core/dnsserver` holds the DNS primitives that more than one DNS plugin
needs: listener lifecycle, client-IP selection, the authoritative answer shape,
and a CIDR longest-prefix matcher. geodns and as112 both build on it.

<!-- source: internal/core/dnsserver/manager.go -- Manager, New, Apply, Stop -->
<!-- source: internal/core/dnsserver/handler.go -- Peer, AnswerFunc, Authoritative, shapeAuthoritative -->

## Why a core package and not a shared plugin

A plugin must not import a sibling plugin (`ai/rules/plugins.md`). as112 needed
the geodns server internals, so the choices were:

| Option | Refused because |
|--------|-----------------|
| as112 imports geodns | forbidden by the plugin rules |
| as112 copies the authoritative handler | the copy holds a security invariant, and copies drift |
| a new component | the harness has no config-driven lifecycle, so `ai/rules/architecture.md` puts it in core |

`internal/core/probe` set the precedent for ping and traceroute. The extraction
went ahead at two consumers, below the usual "three use cases" heuristic,
because the alternative was a forbidden import or a security-sensitive copy.

## The recursion guard is enforced, not offered

An answer function never receives the full `dns.ResponseWriter`. It receives
`Peer`, which exposes `RemoteAddr` alone, so it cannot write its own reply.
`Authoritative` owns the single `WriteMsg` call and applies
`shapeAuthoritative` before AND after the answer function runs. An answer
function that sets `RecursionAvailable` or `Compress` cannot put that on the
wire.

<!-- source: internal/core/dnsserver/handler.go -- Authoritative, shapeAuthoritative -->

The wrapper also owns the RFC 4035 header bits: it copies CD from the query,
ignores AD on the query, and never asserts AD on the reply.

<!-- source: internal/core/dnsserver/rfc4035_test.go -- header bits of the authoritative reply -->

The first shipped design passed the full `dns.ResponseWriter` and left the
guard as a convention each consumer had to remember. Two review rounds replaced
it with the shape above. `dns.ResponseWriter` satisfies `Peer`, so the wrapper
passes it through with no per-query allocation, and the packet source stays
lazy: a refused query never pays for `RemoteAddr`.

## Apply must never believe a stale signature

`Manager.Apply` compares the desired endpoint set against `applied` and returns
early when they match. Two failure modes broke that short-circuit, and both are
fixed by invalidating `applied` rather than leaving it:

<!-- source: internal/core/dnsserver/manager.go -- unappliedSig, generation, Apply, Stop -->

| Sequence | Old result | Fix |
|----------|-----------|-----|
| good, then bad, then back to the same good set | the third `Apply` matched the still-stored good signature and bound zero listeners | any failure path resets `applied` to `unappliedSig` |
| an accept-loop goroutine dies without a `Stop` | a later `Apply` with the same set no-ops forever and believes the dead listener is up | `Stop` bumps a generation counter that `serve` compares against its bind-time snapshot |

Both bugs were found while implementing as112 and both live in shared code, so
geodns inherited the fixes.

## Freebind

`Options.Freebind` is off by default. The Linux build sets `IP_FREEBIND` with
`syscall.SetsockoptInt` and the raw constant 15, which the stdlib `syscall`
package does not export. This needs no `golang.org/x/sys/unix` dependency, and
mirrors the `SO_BINDTODEVICE` split in `internal/plugins/dhcpserver`.

<!-- source: internal/core/dnsserver/freebind_linux.go -- listenConfig -->
<!-- source: internal/core/dnsserver/freebind_other.go -- listenConfig -->

`net.ListenConfig.ListenPacket` and `Listen` have pointer receivers. Chaining
them on a function-call result does not compile, because the result is not
addressable. Assign the config to a local variable first.

## Vocabulary stays out of core

The matcher entry is `Entry{Prefix, Label}`. geodns calls its own labels
"host-set" and keeps that word in the plugin. The core package holds the
longest-prefix mechanism and none of the caller's vocabulary.

<!-- source: internal/core/dnsserver/matcher.go -- Entry, Matcher, Lookup -->

## Client IP

`ClientIP` returns the EDNS0 client-subnet address when the query carries one
and the configuration allows it, and the packet source otherwise. The packet
source arrives as a `netip.Addr` value, so the function is testable with no
fake `ResponseWriter`.

<!-- source: internal/core/dnsserver/client.go -- ClientIP, RemoteAddr -->
