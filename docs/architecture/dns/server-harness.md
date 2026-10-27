# Shared DNS Server Harness

`internal/core/dnsserver` holds the DNS primitives that more than one DNS plugin
needs: listener lifecycle, client-IP selection, the authoritative answer shape,
and a CIDR longest-prefix matcher. geodns and as112 build on its authoritative
wrapper, and the LAN resolver on its recursive one.

<!-- source: internal/core/dnsserver/manager.go -- Manager, New, Stop -->
<!-- source: internal/core/dnsserver/secure.go -- ApplyWithSecure, reconcile -->
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

Owning the write also means owning its failure. `send` counts a refused write in
`ze_dns_reply_write_failure_total`, labelled by transport, and logs one line at
Debug. The counter is the harness's own, unlike the listener gauge a consumer
wires through `Options.OnListenerChange`, because the failure happens where no
consumer can see it: every plugin serving DNS calls
`dnsserver.SetMetricsRegistry` from its `ConfigureMetrics`, and the registry is
idempotent by name, so they share one counter. Debug is the level because the
transport refuses one reply at a time. A host that stops reading would put one
line in the log for every query it draws, and the counter is what an operator
alerts on instead.

<!-- source: internal/core/dnsserver/metrics.go -- the write-failure counter -->

Owning the query also means owning a panic in the answer function. Both wrappers
defer the harness's own `queryGuard` (`internal/core/taskguard`) at the top of
each query, so the consumer cannot opt out of the report. A contained panic
writes one ERROR line on the `dns.server` subsystem logger naming the client and
the question, counts one in `ze_task_panics_total{component="dns",task}` (task
`authoritative-query` or `recursive-query`), writes a crash record that
`ze show crashes` lists, and sends no reply: the client retries or gives up, and
the listener keeps answering. The line does not go through the consumer's log,
because a nil log means "no write-failure line", never "hide a Ze defect".
`SetMetricsRegistry` publishes the counter beside the write-failure one.

<!-- source: internal/core/dnsserver/guard.go -- queryGuard, describeQuery -->

The wrapper also owns the AA bit, and the RCODE decides it. Every reply carries
AA set except one with RCODE 5 (Refused), which an answer function returns for a
name under no zone it serves and for a service the operator turned off. RFC 1035
Section 4.1.1 gives AA one meaning, that the responder is an authority for the
name in the question section, so a Refused reply that kept AA set would assert
authority over a namespace Ze holds no zone for. The decision sits in
`shapeAuthoritative`, not at either plugin's call site.

The wrapper also owns the RFC 4035 header bits: it copies CD from the query,
ignores AD on the query, and never asserts AD on the reply.

<!-- source: internal/core/dnsserver/rfc4035_test.go -- header bits of the authoritative reply -->

The first shipped design passed the full `dns.ResponseWriter` and left the
guard as a convention each consumer had to remember. Two review rounds replaced
it with the shape above. `dns.ResponseWriter` satisfies `Peer`, so the wrapper
passes it through with no per-query allocation, and the packet source stays
lazy: a refused query never pays for `RemoteAddr`.

## A recursive service gets its own wrapper

The LAN resolver (`service dns-resolver`) answers recursive queries, so the
authoritative shape is wrong for it: it must set RA and must never set AA. It
does not take a flag on `Authoritative`, because a flag would turn the enforced
shape back into a choice each caller can get wrong, which is the convention the
section above replaced. `Recursive` is a second wrapper with the same contract:
the answer function receives `Peer`, the wrapper owns the single write, and
`shapeRecursive` runs before and after the answer function.

| Bit or field | `shapeAuthoritative` | `shapeRecursive` |
|--------------|----------------------|------------------|
| AA | set, unless RCODE 5 | always clear: a forwarder holds no zone |
| RA | always clear | always set, refusals included |
| Z, AD | clear | clear: the resolver validates nothing |
| CD | copied by `SetReply` | copied from the query on every reply |
| OPT | whatever the answer function adds | exactly one when the query carried one, advertising `RecursiveUDPSize` (1232) with DO clear; none otherwise |
| Datagram bound | the requestor's offer, never below 512 | the requestor's offer held to 1232, never below 512 |

Both wrappers answer a non-QUERY opcode with Not Implemented before the answer
function runs, and both share `send`, which takes the datagram bound the wrapper
chose. A consumer of `Recursive` also sets `Options.UDPSize` to
`RecursiveUDPSize`, so the UDP listener reads a query of that size whole; with
the field zero the read buffer stays at miekg's 512 octets, and a longer query
draws FORMERR from miekg without reaching the handler. The authoritative
consumers leave it zero today.

<!-- source: internal/core/dnsserver/recursive.go -- Recursive, shapeRecursive, recursiveReplyLimit, RecursiveUDPSize -->
<!-- source: internal/core/dnsserver/manager.go -- Options.UDPSize -->
<!-- test: internal/core/dnsserver/recursive_test.go -- TestRecursiveShape, TestRecursiveOpcode, TestRecursiveUDPSize -->
<!-- test: internal/core/dnsserver/handler_test.go -- TestAuthoritativeUnchanged -->

## An apply must never believe a stale signature

`Manager.ApplyWithSecure`, through `reconcile`, compares the desired listener
set against `applied` and returns early when they match. Two failure modes broke that short-circuit, and both are
fixed by invalidating `applied` rather than leaving it:

<!-- source: internal/core/dnsserver/manager.go -- unappliedSig, generation, Stop -->

| Sequence | Old result | Fix |
|----------|-----------|-----|
| good, then bad, then back to the same good set | the third apply matched the still-stored good signature and bound zero listeners | any failure path resets `applied` to `unappliedSig` |
| an accept-loop goroutine dies without a `Stop` | a later apply with the same set no-ops forever and believes the dead listener is up | `Stop` bumps a generation counter that `serve` compares against its bind-time snapshot |

Both bugs were found while implementing as112 and both live in shared code, so
geodns inherited the fixes.

## Boot binds leniently, a reload binds all or nothing

`ApplyWithSecure` takes a `BindPolicy` from its caller. `BindLenient` is the
boot (Stage 2) policy: a listener that cannot be bound, or DoT and DoH TLS
material that cannot be loaded, is logged and the rest serve; the apply fails
only when nothing bound. `BindStrict` is the reload policy. A TLS load failure
refuses before any listener stops, unless the secure listeners would not
change (the same secure config on the same addresses as the last accepted
apply) and that apply holds TLS material: the reload then keeps the material
those listeners serve and logs the failure, so a reload of host data alone is
not refused for a certificate file that left the disk. When the last accepted
apply holds none (a boot whose own load failed, so no DoT or DoH listener
serves), the reload is refused, because there is nothing to keep. A listener that cannot be bound fails the apply: every
listener this apply bound is stopped, the previous apply's listeners are bound
again, and the error names each endpoint that failed and the fix. The plugin's
installer then puts its previous snapshot back (as112 also its loopback
address registration; its redistribute producer takes a config only after the
listeners bound), so the commit fails and the running config keeps serving.

The previous listeners are the ones that served, so a listener the running
config never bound is not counted. If another process takes one of their ports
in the moment between the stop and the rebind, the running config is no
longer fully served, and the refusal says so: it names each previous listener
that is not serving and wraps `ErrPreviousListenersLost`, never claiming the
listeners were kept. The installer passes that on as
`sdk.ErrRunningConfigLost`, which marks the plugin's `sdk.ConfigTx` broken.
The transaction's rollback then installs the running config again; when that
fails too, the rollback answers an error, the engine acks it `CodeBroken`,
and the orchestrator restarts the plugin, which binds leniently at Stage 2.
While broken, a later apply of the running config is installed rather than
skipped as unchanged, and the harness's signature is cleared so that apply
binds. The error reaches the operator only after the rollback ran, so its
text claims neither outcome: the installer appends `sdk.RunningConfigLostNext`
(the rollback binds again or restarts the plugin; free the port, then, if a
listener stays down, stop the whole daemon and start it again by the routes of
`sdk.RestartSteps()`, which restarts every protocol and drops every session;
no command restarts one plugin, and `ze signal restart` leaves ze stopped), and
the orchestrator appends a `Rollback:` line for each plugin, naming what
happened to it
([transaction protocol](../config/transaction-protocol.md)). A commit of the
running config is no repair: it has no diff, so the reload stops at "no
changes" and no apply reaches the plugin. A reload that changes the listener set stops every listener before it
binds the new set, so serving pauses for that moment (and as112 withdraws and
announces its covering prefixes again, RFC 7534 Section 3.3). Every caller
names its policy, tests included: no entry point binds leniently by default.

<!-- source: internal/core/dnsserver/secure.go -- BindPolicy, reconcile, restorePrevious, bindListeners, ApplyWithSecure, keptSecure, buildSecureTLS -->
<!-- source: internal/plugins/as112/config_tx.go -- as112Installer.install, refuse, configureAtBoot -->
<!-- source: pkg/plugin/sdk/config_tx.go -- ErrRunningConfigLost, RunningConfigLostNext, ConfigTx.Apply, ConfigTx.Rollback -->
<!-- source: internal/plugins/geodns/config_tx.go -- geodnsInstaller.install, configureAtBoot -->

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

## What one TCP client can hold

Every plain TCP listener runs with explicit per-connection deadlines: 2 seconds
to read the first query, 8 seconds idle between queries, 2 seconds to write a
reply, and at most 128 queries on one connection. These are the miekg/dns
defaults, set at the call site so a changed library default cannot change Ze.

The deadlines bound how long a connection lives, not how many there are.
`Options.TCPConnMax` caps how many connections each plain TCP listener holds at
once. A connection accepted past the cap is closed at once, before any handler
or ACL runs, and counted in `ze_dns_tcp_connection_refused_total`. Closing it
rather than leaving it in the kernel backlog tells the client at once, so it
retries elsewhere instead of timing out. A held connection frees its slot when
it closes.

Zero means no cap. The authoritative services (as112, geodns) keep zero, so
their behavior is unchanged: they answer from memory and hold nothing per
connection beyond the deadlines above. The LAN resolver sets 64
(`resolverTCPConnMax`), because its ACL is checked per query, after accept, and
each connection can hold an upstream resolution open. The DoT and DoH
listeners are not capped by this option.

<!-- source: internal/core/dnsserver/connlimit.go -- connLimitListener, limitedConn, tcpReadTimeout, tcpIdleTimeout, tcpQueriesMax -->
<!-- source: internal/core/dnsserver/manager.go -- Options.TCPConnMax, Manager.capTCP, Manager.bind -->
<!-- source: internal/plugins/dnsresolver/server.go -- resolverTCPConnMax, newResolverServer -->
<!-- test: internal/core/dnsserver/connlimit_test.go -- TestManager_TCPConnMaxRefusesPastTheCap, TestManager_TCPConnMaxZeroIsUncapped -->

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
