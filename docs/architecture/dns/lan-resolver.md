# LAN DNS Resolver

The `dnsresolver` plugin (`service dns-resolver`) answers recursive queries
from the hosts behind Ze. It is a caching forwarder: it answers from its cache,
or it sends the question to the upstream resolvers the operator names and
caches what comes back. It does not do full recursion from the root
(`spec-dns-resolver-recursion` owns that) and it does not validate DNSSEC
(`spec-dns-resolver-dnssec-validation` owns that).

The plugin answers through `dnsserver.Recursive`, the harness's recursive
wrapper. The wrapper owns the reply header, the OPT record, the opcode check,
the datagram bound and the single write: see
[the server harness](server-harness.md#a-recursive-service-gets-its-own-wrapper).
This page covers what the plugin decides inside that wrapper. The operator
guide is [the DNS resolver guide](../../guide/dns-resolver.md).

<!-- source: internal/plugins/dnsresolver/register.go -- init, runResolverPlugin -->
<!-- source: internal/plugins/dnsresolver/server.go -- newResolverServer, recursiveHandler -->

## Runtime state

One `resolver` lives for the whole plugin run. It holds the published policy,
the answer cache, the negative cache, the in-flight table, the limits and the
counters. A config apply replaces only the policy and the cache bound, so a
reload keeps the cache. The policy is an immutable snapshot of the applied
config (the allow-from matcher, the forward list, the timeout and the limits),
and the query path reads it with one atomic load. Until the first apply no
policy exists, and a query is answered SERVFAIL; no listener is bound before
that apply.

<!-- source: internal/plugins/dnsresolver/state.go -- policy, newPolicy, resolver, newResolver, apply -->

## Request path

`resolver.answer` runs the checks cheapest first. The first check that decides
the query ends it.

| Step | Check | Result when it decides |
|------|-------|------------------------|
| 1 | The source address is inside an `allow-from` prefix | REFUSED, counted as reason `acl` |
| 2 | The source is under `limit per-client-rate` | dropped, no reply |
| 3 | RD is set | REFUSED, counted as reason `rd-clear`, whether or not the name is cached, so the cache cannot be snooped |
| 4 | The query holds exactly one question | FORMERR |
| 5 | The cache holds the answer | the cached answer, TTLs aged |
| 6 | The resolution (coalescing, outstanding bound, forwarding) | its answer, SERVFAIL, or a drop |

The source is the packet source (`dnsserver.RemoteAddr`), never an EDNS0
client-subnet option. A query with an opcode other than QUERY never reaches
these steps: the wrapper answers it Not Implemented, and `countRejected`
counts it in `ze_dns_resolver_queries_total` like every other reply. The
listeners run `dnsserver.RecursiveAccept` as their header filter, so both that
query and one with two questions reach the resolver's handler and are
answered in the recursive shape (RA set) rather than by the DNS library's own
filter (`server-harness.md`).

<!-- source: internal/plugins/dnsresolver/server.go -- resolver.answer, resolver.countRejected, newResolverServer -->
<!-- test: internal/plugins/dnsresolver/server_test.go -- TestACLRefused, TestRDZeroRefused, TestQDCount, TestRateLimit -->
<!-- test: internal/plugins/dnsresolver/accept_test.go -- TestMalformedQueryShapedAndCounted -->

## Coalescing and the outstanding bound

`resolveShared` keys each resolution by name (case folded), type and class. The
first query for a key leads the resolution, and an identical query that arrives
while it runs waits for the leader's result instead of sending its own upstream
query. A waiter waits for the timeout at most, then answers SERVFAIL.

The leader takes one slot of `limit outstanding` before it resolves. When every
slot is taken, the query is dropped with no reply, and every waiter on that key
is dropped with it: the client sends the query again after its own timeout.

A waiter takes no resolution slot, but it holds a request goroutine and a
timer, so the waiters are bounded as well: at most `limit outstanding` of them
at once, counted apart from the resolutions. A query that would wait beyond
that is dropped and counted in `ze_dns_resolver_dropped_total`, so a flood of
one uncached question cannot park unbounded work. Unbound's
`num-queries-per-thread` bounds the same way, counting every client waiting for
an answer.

The leader finishes its flight and returns its slot on every exit, a panic
included, so its waiters are released and the next identical question can
lead.

<!-- source: internal/plugins/dnsresolver/resolve.go -- resolveShared, resolveCounted, awaitFlight -->
<!-- source: internal/plugins/dnsresolver/limit.go -- flightTable, limiter.acquire, limiter.acquireWaiter -->
<!-- test: internal/plugins/dnsresolver/server_test.go -- TestCoalescing, TestOutstandingBound, TestSlowUpstreamDoesNotBlockCache -->
<!-- test: internal/plugins/dnsresolver/resolve_test.go -- TestCoalescedWaitersBounded, TestLeaderPanicReleasesFlight -->

## Per-client rate

`limit per-client-rate` counts each source address in one-second windows. Over
the rate, the query is dropped, and one in ten of the excess is still answered,
so a client behind a shared address gets some answers (Unbound's
`ip-ratelimit-factor` default). The window table holds at most 65536 sources. A
new source that arrives while the table is full, after the stale windows are
swept, is dropped. A rate of 0 sets no limit and keeps no table.

<!-- source: internal/plugins/dnsresolver/limit.go -- limiter.admit, clientsMax, rateExcessPassEvery -->

## Forwarding

`resolver.forward` sends the question to the `forward server` entries in the
order the operator wrote them, one pass over the list. Each attempt gets an
equal share of the time left before the deadline, so every upstream is tried
before the `timeout` ends. The next upstream is tried when one does not answer
in its share, when the exchange fails, or when it answers with an RCODE other
than NOERROR or NXDOMAIN. When no upstream gives a usable answer, the client
gets SERVFAIL, which is never cached.

A reply with TC set is asked again over TCP, to the same upstream, inside the
same attempt.

<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.forward, servfail -->
<!-- test: internal/plugins/dnsresolver/forward_test.go -- TestFailoverAndTimeout, TestUpstreamFailureRcode, TestTCPRetryOnTC -->

## The upstream query

The query sent upstream carries RD set, a fresh random identifier, the
question, and an OPT that advertises 1232 octets with DO clear. The resolver is
not security-aware, so it asks for no DNSSEC records, and it never forwards the
client's CD bit.

<!-- source: internal/plugins/dnsresolver/forward.go -- upstreamQuery, upstreamUDPSize -->
<!-- test: internal/plugins/dnsresolver/forward_test.go -- TestUpstreamQueryShape -->

## The exchange and reply matching

Each exchange dials its own socket, so the kernel picks a new ephemeral source
port for each one (RFC 5452 Section 9.2). The socket is connected, so a
datagram from another address or port never reaches the read. A reply is
accepted only when it has QR set, the query's identifier, exactly one question,
and the query's type, class and name (the name compared without case), as
RFC 5452 Section 9.1 asks.

On UDP a reply that does not match, or a datagram that does not parse, is
ignored and the read continues until the attempt's deadline. On TCP the stream
carries one reply, so a mismatch fails the exchange and the next upstream is
tried.

<!-- source: internal/plugins/dnsresolver/forward.go -- exchange, replyMatches -->
<!-- test: internal/plugins/dnsresolver/forward_test.go -- TestReplyMatching -->

## The work budget

One client query may cause at most 32 upstream queries (a TCP retry counts as
one) and 11 CNAME restarts, inside the `timeout` deadline (Unbound's
`max-sent-count` and `max-query-restarts` defaults, RFC 4035 Section 5.4). A
CNAME chain that comes back to a name it already passed is a loop. A query that
reaches a bound or a loop is answered SERVFAIL.

<!-- source: internal/plugins/dnsresolver/limit.go -- budget, upstreamQueriesMax, restartsMax -->
<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.resolve, resolver.absorb -->
<!-- test: internal/plugins/dnsresolver/forward_test.go -- TestWorkBudget -->

## Accepting a reply: bailiwick

`resolver.absorb` walks the reply's answer section from the question name along
its CNAME chain. It keeps only the RRsets of the question's class owned by the
question name or by a CNAME target on that chain, and an authority-section SOA
that owns the last name on the chain or one of its ancestors. Nothing else in
the reply is cached or returned, so an upstream cannot plant data for a name
nobody asked for.

When the upstream followed a CNAME but sent nothing about its target, and no
SOA says the target is empty, the resolver asks again for the target.

The chain follows the alias the client is given. When the cache holds a live
higher-ranked CNAME for a name on the chain, that CNAME is returned in place of
the reply's (RFC 2181 Section 5.4.1), and the walk continues from its target.
If that target differs from the reply's, the rest of the reply describes
another chain: its records, its RCODE and its SOA are not used, and the
resolver asks for the held target.

An ANY query is answered with every answer-section RRset the question name
owns, in the order the reply carries them, a whole RRSIG RRset included (RFC
2181 Section 5.3.1). A CNAME matches ANY as any other type at the node does, so
an ANY query follows no alias (RFC 1034 Section 4.3.2). Each RRset is cached
under its own type, where a later query for that type finds it, except the
RRSIG records: an ANY answer need not carry every signature the node holds, so
they are returned and never cached where an RRSIG query would read them. The ANY
answer itself is never cached: the per-type entries cannot prove they are every
RRset the node holds, so every ANY query is forwarded. Unbound caches a whole
ANY reply as one message instead.

<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.absorb, followsAlias, resolver.storeOwned, groupAnswer, findSOA -->
<!-- test: internal/plugins/dnsresolver/forward_test.go -- TestBailiwick -->
<!-- test: internal/plugins/dnsresolver/resolve_test.go -- TestAnyReturnsEveryOwnedRRset, TestAnyDoesNotFollowAlias, TestAnyDoesNotCacheRRSIGSet, TestHeldAliasRedirectsTheChain -->

## The answer cache

| Property | Behavior |
|----------|----------|
| Key | owner name (lower-cased), type and class; the records keep the case they arrived with |
| TTL | the lowest TTL of the RRset, capped by `cache max-ttl`; a TTL with the most significant bit set counts as 0 (RFC 2181 Section 8) |
| TTL 0 | not cached |
| Aging | a hit returns the records with the TTL left, in whole seconds; an expired entry is a miss |
| Replace | a new RRset replaces the cached one whole, and is never merged with it (RFC 2181 Section 5.4) |
| Rank | the RRset owned by the name a reply was sought for, in a reply with AA set, outranks every other answer-section RRset; in an authoritative reply to an alias only the CNAME is authoritative, and the data at its target ranks as a non-authoritative answer; a live entry of the higher rank is kept and the lower-ranked RRset ignored (RFC 2181 Section 5.4.1), and the client is answered with the kept RRset at its remaining TTL, never the ignored copy |
| Bound | `cache size` entries; the least recently used entry is evicted to make room; 0 caches nothing |

A forwarding upstream answers without AA, and RFC 2181 Section 5.2 lets a
client that sends every query to particular servers treat them as
authoritative for TTL purposes, which is the case here. Only answer-section
data is cached, so additional-section data is never returned as an answer.

A cache lookup follows cached CNAMEs within the restart bound. When any link
of the chain is missing, the whole question is resolved again from the start.

<!-- source: internal/plugins/dnsresolver/cache.go -- keyOf, rankOf, lru.put, lru.store, lru.get, receivedTTL, resolver.lookup -->
<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.storeRRset -->
<!-- test: internal/plugins/dnsresolver/resolve_test.go -- TestRefusedLowerRankAnswersCachedSet -->
<!-- test: internal/plugins/dnsresolver/cache_test.go -- TestCacheTTLDecrement, TestCacheCaseFold, TestCacheReplaceNotMerge, TestTTLHighBit, TestCacheMaxTTL, TestTrustRanking -->
<!-- test: internal/plugins/dnsresolver/rfc2181_resolver_test.go -- TestRFC2181LowerTrustNeverDisplacesHigher, TestRFC2181HigherTrustReplacesLower, TestRFC2181AliasTargetNotAuthoritative, TestRFC2181WholeSIGRRSet, TestRFC2181SectionSIGNotTheRRSet, TestRFC2181SectionSIGNotCached -->

## The negative cache

An NXDOMAIN answer is cached for the name, whatever the type. A NOERROR answer
with no data is cached for the name and the type. Each is cached only when the
reply carries an in-bailiwick SOA, for the lower of the SOA's TTL and its
MINIMUM field, capped by `cache max-negative-ttl` (RFC 2308 Section 5). A
negative answer without an SOA is returned to the client and not cached. A
cached negative answer returns the SOA in the authority section, with its TTL
aged.

The negative cache is a second LRU with the same `cache size` bound as the
answer cache, so the two together hold up to twice that many entries.

<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.storeNegative -->
<!-- source: internal/plugins/dnsresolver/cache.go -- typeNameError, resolver.lookup -->
<!-- source: internal/plugins/dnsresolver/state.go -- resolver.apply -->
<!-- test: internal/plugins/dnsresolver/cache_test.go -- TestNegativeCacheTTL, TestNegativeWithoutSOANotCached -->

## The sweeper

The resolver runs one long-lived goroutine. Every 10 seconds it frees the
expired entries of both caches and the rate windows older than the current
second, then publishes the cache-entry gauges. A lookup checks expiry itself,
so the interval bounds the memory dead entries hold, never what a client is
answered. The goroutine starts with the plugin and stops when it stops.

<!-- source: internal/plugins/dnsresolver/state.go -- sweepInterval, resolver.start, resolver.stop, resolver.sweep -->

## Reload

The config transaction parses the `service` section with the same `parseConfig`
the offline verifier runs, so `ze config validate` and a commit refuse the same
configs. While a reload reconciles the listeners, the resolver answers under the
running policy with its `allow-from` narrowed to the sources both the running
and the candidate list admit (`resolver.narrow`). A listener the reload binds
then never answers a source the running config refuses, a listener it keeps
never answers a source the candidate refuses, and a refused reload never
serves under the candidate's forward list or limits. The candidate's policy,
cache bound and shown config are published only once every listener it names
is bound. An unchanged listener set does not rebind: a reload that changes
only `allow-from`, `forward`, the cache or the limits keeps every socket, and a
TCP connection open across the reload carries on under the new policy.

A listener can name an address the host does not hold yet. The resolver sets
`IP_FREEBIND` on every listener socket (`dnsserver.Options.Freebind`, Linux
only), so the bind succeeds before the address is up, and the listener answers
from the moment the kernel holds it. A LAN resolver needs this for an address
that appears after the daemon starts: an L2TP pool gateway exists only while a
PPP session is up, and an interface address arrives when iface applies it. No
sysctl such as `net.ipv4.ip_nonlocal_bind` is needed. On another platform the
option has no effect, and such a bind fails.

At boot a listener that cannot bind is logged and the others serve. On a reload
a bind failure fails the commit, the harness binds the previous listeners
again, and the running policy and shown config are published again
(`resolver.restore`), so the running config keeps serving and `show
dns-resolver` reports it. When even those listeners cannot bind again, the
error carries `sdk.ErrRunningConfigLost`, so the transaction installs the
running config again or restarts the plugin.

<!-- source: internal/plugins/dnsresolver/config_tx.go -- resolverInstaller.install, configureAtBoot -->
<!-- source: internal/plugins/dnsresolver/state.go -- resolver.narrow, resolver.restore, intersectPrefixes -->
<!-- test: internal/plugins/dnsresolver/config_tx_test.go -- TestRefusedReloadKeepsRunningPolicy -->
<!-- test: internal/plugins/dnsresolver/state_test.go -- TestNarrowedReloadWindow -->
<!-- test: internal/plugins/dnsresolver/resolve_test.go -- TestIntersectPrefixes -->
<!-- source: internal/plugins/dnsresolver/server.go -- resolverServer.reconcile -->
<!-- source: internal/plugins/dnsresolver/config.go -- parseConfig, checkEnabled -->
<!-- source: internal/plugins/dnsresolver/server.go -- newResolverServer -->
<!-- source: internal/core/dnsserver/freebind_linux.go -- ListenConfig -->
<!-- test: internal/plugins/dnsresolver/config_tx_test.go -- TestReloadKeepsListeners -->
<!-- test: internal/plugins/dnsresolver/freebind_linux_test.go -- TestListenBeforeAddressIsUp -->
<!-- test: internal/plugins/dnsresolver/server_test.go -- TestAllowFromReload -->

## Show and clear

`show dns-resolver` reads the published policy, the applied config and the
counters, which are the state the query path reads. Each listener also
carries its `state`, one entry per transport (`udp`, `tcp`): `up` or `down` from
the harness's `OnListenerChange` edges, a crash included, keyed by address and
port, and `unknown` before the first edge. A listener a reload removes is
forgotten once the reload settles: its state and its
`ze_dns_resolver_listener_up` series are deleted (`listenerStates.prune`), so
no series reports a socket that no longer exists. `clear dns-resolver cache
all` empties both caches and keeps the counters. The bare
`clear dns-resolver cache` has no handler, so a word typed by mistake never
empties the cache. Both RPCs are owned by the plugin (`ze-dnsresolver:*`).

<!-- source: internal/plugins/dnsresolver/show.go -- handleShowDNSResolver, handleClearDNSResolverCache, resolver.show -->
<!-- source: internal/plugins/dnsresolver/yang/ze-dns-resolver-cmd.yang -- show and clear command tree -->
<!-- source: internal/plugins/dnsresolver/listener_state.go -- listenerStates -->
<!-- test: internal/plugins/dnsresolver/show_test.go -- TestShowAndClear -->
<!-- test: internal/plugins/dnsresolver/listener_state_test.go -- TestListenerStateByPort, TestPruneDeletesGaugeSeries -->

## Doctor

The plugin registers two `ze doctor` checks, which run only for an enabled
resolver whose config parses (config validation reports one that does not).

| Check | Code | Warns when |
|-------|------|------------|
| `dns-resolver-listen-capability` | `doctor-dns-resolver-port-unavailable` | a listener's bind is refused for lack of permission: a port below 1024 needs `CAP_NET_BIND_SERVICE` or root |
| `dns-resolver-upstream-reachable` | `doctor-dns-resolver-upstream-unreachable` | no `forward server` answers a probe for the root NS RRset within the doctor probe timeout (3 seconds at most); each server tried is named with its error |

A busy port is reported by the generic listener probe
(`doctor-listen-unavailable`), which sees the resolver's listeners through the
`ze:listener` marking, for UDP and TCP. An address that is not on the host yet
is not reported: the resolver binds with `IP_FREEBIND`, so a PPP pool gateway
that appears with the first subscriber session still binds, and the plugin
tells the probe so with `config.RegisterListenerFreebind`. Both probes then
bind with `IP_FREEBIND` too (`dnsserver.ListenConfig(true)`), as the resolver
does, so an absent address binds while a port another socket holds still
fails with `EADDRINUSE` and a refused permission still fails. A plain bind
would stop at `EADDRNOTAVAIL`, because Linux checks the address before the
port, and hide both. The upstream
probe sends the query the resolver itself sends, through the same exchange, and
any reply counts, whatever its RCODE.

<!-- source: internal/plugins/dnsresolver/doctor.go -- doctorChecks, listenerDiagnostics, upstreamDiagnostics, probeUpstream -->
<!-- source: internal/plugins/dnsresolver/register.go -- RegisterListenerProtocols, RegisterListenerFreebind, RegisterListenerEntryDefault -->
<!-- source: internal/plugins/dnsresolver/doctor.go -- probeBindable -->
<!-- source: internal/component/doctor/checks_listener.go -- probeListener, bindProbe -->
<!-- test: internal/plugins/dnsresolver/doctor_test.go -- TestDoctorListenerPortUnavailable, TestDoctorUpstreamUnreachable -->
<!-- test: internal/plugins/dnsresolver/freebind_linux_test.go -- TestProbeBindableFreebind -->
<!-- test: internal/component/doctor/checks_listener_freebind_linux_test.go -- TestProbeListenerFreebindAcceptsAbsentAddress, TestProbeListenerFreebindAbsentAddressBusyPort -->

## Metrics

| Metric | Type | Labels | Counts |
|--------|------|--------|--------|
| `ze_dns_resolver_queries_total` | CounterVec | transport, rcode | queries answered, by transport (`udp`, `tcp`) and RCODE name |
| `ze_dns_resolver_refused_total` | CounterVec | reason | REFUSED answers: `acl` or `rd-clear` |
| `ze_dns_resolver_rate_limited_total` | Counter | | queries dropped by `limit per-client-rate` |
| `ze_dns_resolver_dropped_total` | Counter | | queries dropped because every `limit outstanding` slot was taken, for a resolution or for a query waiting on one |
| `ze_dns_resolver_outstanding` | Gauge | | resolutions in progress |
| `ze_dns_resolver_cache_entries` | GaugeVec | cache | entries held: `answer` or `negative`, published by the sweeper and by a clear |
| `ze_dns_resolver_cache_hits_total` | Counter | | queries answered from the cache |
| `ze_dns_resolver_upstream_queries_total` | CounterVec | result | upstream exchanges: `answered`, `timeout`, `error`, `failure` (an RCODE other than NOERROR or NXDOMAIN), `truncated` (TC set, asked again over TCP) |
| `ze_dns_resolver_listener_up` | GaugeVec | protocol, address | 1 while a listener socket is bound, 0 once it is unbound or crashed; the series is deleted once a reload removes the listener; `address` is the IP and port (`127.0.0.1:53`), so two listeners on one IP are two series |

A dropped query gets no reply and is not counted in
`ze_dns_resolver_queries_total`. No label carries a client address or a queried
name, so the series count stays bounded.

<!-- source: internal/plugins/dnsresolver/metrics.go -- buildMetrics, resolverStats, upstreamResult -->
