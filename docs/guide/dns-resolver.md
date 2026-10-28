# LAN DNS Resolver

Ze's `dnsresolver` plugin is a caching DNS resolver for the hosts behind Ze.
It answers recursive queries from the LAN and subscriber prefixes you name, on
the addresses you name, and it resolves a name by forwarding the query to the
upstream resolvers you name. Hosts learn its address from DHCPv4 option 6, the
router advertisement RDNSS option or IPCP.

<!-- source: internal/plugins/dnsresolver/register.go -- dnsresolver registration -->
<!-- source: internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang -- service dns-resolver -->

## What it does not do

| Not done | Consequence | Owning spec |
|----------|-------------|-------------|
| Full recursion from the root | `mode forward` is the only mode: without a `forward server` the resolver has nowhere to resolve | `spec-dns-resolver-recursion` |
| DNSSEC validation | the resolver asks upstream with DO clear, never sets AD, and returns no DNSSEC records the client did not ask for | `spec-dns-resolver-dnssec-validation` |
| DNS over TLS or HTTPS | the listeners serve plain UDP and TCP only | |

<!-- source: internal/plugins/dnsresolver/config.go -- parseConfig mode -->
<!-- source: internal/plugins/dnsresolver/forward.go -- upstreamQuery -->
<!-- source: internal/plugins/dnsresolver/server.go -- resolverServer.reconcile -->

This resolver is not the resolver Ze uses for its own lookups
(`system { name-server }`, [DNS Resolver](../features/dns-resolver.md)). The
two share no configuration and no cache.

## Quick start

```
service {
	dns-resolver {
		enabled true
		listener lan {
			ip 192.168.1.1
		}
		listener loopback {
			ip 127.0.0.1
		}
		allow-from [ 192.168.1.0/24 127.0.0.0/8 ]
		forward {
			server 192.0.2.53
			server 198.51.100.53 {
				port 53
			}
		}
	}
}
```

`ze config validate` accepts this config. Each listener binds UDP and TCP on
port 53 of its address. The address does not have to be up yet: on Linux the
resolver listens on it before the host holds it, and answers once it does, so
a listener on an L2TP pool gateway works before the first subscriber connects.
A host in 192.168.1.0/24 that asks 192.168.1.1 gets an
answer from the cache, or from 192.0.2.53, or from 198.51.100.53 when the first
server does not answer.

<!-- source: internal/plugins/dnsresolver/config.go -- parseListener, parseForward -->
<!-- source: internal/plugins/dnsresolver/server.go -- newResolverServer -->

## Configuration reference

All leaves are under `service { dns-resolver { ... } }`.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `enabled` | boolean | false | Binds the listeners and answers queries. true needs at least one `listener`, one `allow-from` prefix and one `forward server`. |
| `listener <name> ip` | IP address | (none) | A LAN or loopback address to answer on. A wildcard (0.0.0.0, ::, or its IPv4-mapped form ::ffff:0:0) is refused, because the resolver would also answer on the WAN side. |
| `listener <name> port` | port | 53 | The port to answer on, UDP and TCP. 0 is refused. |
| `allow-from` | leaf-list of prefixes | (none) | Source prefixes the resolver answers. Any other source gets REFUSED. |
| `mode` | enumeration | `forward` | `forward` is the only value. |
| `forward server <ip>` | IP address | (none) | One upstream resolver, at most 8. They are tried in the order you write them. |
| `forward server <ip> port` | port | 53 | The upstream's port. 0 is refused. |
| `cache size` | uint32, 0-10000000 | 100000 | The most entries the answer cache holds, and the most the negative cache holds. 0 disables caching. |
| `cache max-ttl` | seconds | 86400 | The longest time a positive answer stays cached. |
| `cache max-negative-ttl` | seconds | 3600 | The longest time an NXDOMAIN or no-data answer stays cached. |
| `timeout` | seconds, 1-60 | 10 | The longest one resolution may take, every upstream included. After it the client gets SERVFAIL. |
| `limit outstanding` | 1-65535 | 2048 | The most resolutions in progress at once. A query that needs one more is dropped with no reply. |
| `limit per-client-rate` | queries per second | 0 | The most queries per second one source address is answered. Over it queries are dropped, one in ten still answered. 0 sets no limit. |

<!-- source: internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang -- YANG leaves -->
<!-- source: internal/plugins/dnsresolver/config.go -- parseConfig, parseBounds, checkEnabled -->

A commit or `ze config validate` refuses a config that the resolver cannot run
safely, and names the fix:

| Refused | Why |
|---------|-----|
| a wildcard listener address | the resolver would answer on every address of the host, the WAN side included |
| `enabled true` with no listener | it would answer on no address |
| `enabled true` with no `allow-from` | it would refuse every client; answering every source would make Ze an open resolver |
| `enabled true` with no `forward server` | it would have nowhere to resolve |
| `timeout 0`, `limit outstanding 0` | every query would get SERVFAIL, or be dropped |

<!-- source: internal/plugins/dnsresolver/config.go -- parseListener, checkEnabled, parseBounds -->

A disabled resolver may be staged incomplete. A listener port that another
service already uses is refused by the config's cross-service listener check.

## How a query is answered

| Query | Answer |
|-------|--------|
| from a source outside `allow-from` | REFUSED, unless a check named below the table decides it first |
| from a source over `limit per-client-rate` | no reply |
| with more or less than one question | FORMERR, whatever RD is |
| with RD clear | REFUSED, also for a cached name |
| with more than one OPT RR | FORMERR |
| an EDNS version other than 0 | BADVERS |
| an opcode other than QUERY | Not Implemented |
| cached | the cached records, TTLs aged |
| not cached | forwarded upstream; the answer is cached and returned |
| no upstream answers in time, or the work budget runs out | SERVFAIL, not cached |

<!-- source: internal/plugins/dnsresolver/server.go -- resolver.answer -->
<!-- source: internal/core/dnsserver/recursive.go -- Recursive -->

Three checks run before `allow-from` and the client rate limit: more than one
OPT RR, an EDNS version other than 0, and an opcode other than QUERY. A query
that fails one of them gets FORMERR, BADVERS or Not Implemented from any
source. That reply is never larger than the query, and the query is never
forwarded.

A query with more than 16 questions, more than one answer record, more than
one authority record, or more than two additional records is refused by the
DNS library before the resolver sees it. That FORMERR is not built by the
resolver: it has no question and no OPT, its RA bit is the query's own (clear
for an ordinary query), and the resolver's metrics do not count it.

<!-- source: internal/core/dnsserver/recursive.go -- Recursive, RecursiveAccept -->

Every reply the resolver builds has RA set and AA and AD clear, and copies the
query's CD bit. An upstream that does not answer in its share of the timeout,
that fails, or that answers with an RCODE other than NOERROR or NXDOMAIN is passed over for the
next one. A truncated upstream reply is asked again over TCP. A reply that does
not match the query (identifier, name, class and type) is ignored. The details
are in [the LAN resolver design](../architecture/dns/lan-resolver.md).

<!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.forward -->
<!-- source: internal/plugins/dnsresolver/forward.go -- exchange, replyMatches -->

A reload that changes only `allow-from`, `forward`, `cache`, `timeout` or
`limit` keeps the listeners bound and keeps the cache. The next query uses the
new values.

<!-- source: internal/plugins/dnsresolver/config_tx.go -- resolverInstaller.install -->

## Show and clear

```
ze show dns-resolver                   # Config in force, cache sizes and hit rate, counters
ze clear dns-resolver cache all        # Empty the answer and the negative cache
```

`show dns-resolver` reports `enabled`, `mode`, `listeners` (each with its
`state`: `udp` and `tcp`, each `up`, `down`, or `unknown` until the listener
has been bound once), the `allow-from`
prefix count, `upstreams`, `timeout-seconds`, the limits, `outstanding`, the
cache bounds and entry counts, `cache-hits`, `cache-misses`,
`cache-hit-rate`, `queries`, `coalesced`, `refused-acl`, `refused-rd-clear`,
`rate-limited`, `dropped`, and `upstream-queries` by result. With the plugin
not running it reports `running false`.

`clear dns-resolver cache all` reports how many entries it removed from each
cache and keeps the counters. `clear dns-resolver cache` alone does nothing:
it exits 1 and lists `all`, the word you must type.

<!-- source: internal/plugins/dnsresolver/show.go -- resolver.show, handleClearDNSResolverCache -->
<!-- source: internal/plugins/dnsresolver/yang/ze-dns-resolver-cmd.yang -- show and clear command tree -->

## Doctor

`ze doctor` runs two checks for an enabled resolver:

| Code | Means | Fix |
|------|-------|-----|
| `doctor-dns-resolver-port-unavailable` | the system refuses to let ze bind a listener: a port below 1024 needs `CAP_NET_BIND_SERVICE` or root | run ze with that capability, or set the listener port to 1024 or above |
| `doctor-dns-resolver-upstream-unreachable` | no `forward server` answered a probe query; each server tried is named with its error | check the `forward server` addresses and the route and firewall to each upstream |

A port that another program holds is reported by the general listener check,
`doctor-listen-unavailable`. A listen address that is not on the host yet is
not a finding: the resolver binds it anyway, so a listener on a PPP pool
gateway is ready before the first subscriber connects. Both checks probe such
an address the way the resolver binds it, so a port another program holds, or
a port below 1024 without the capability, is still reported for it.

<!-- source: internal/plugins/dnsresolver/doctor.go -- listenerDiagnostics, upstreamDiagnostics, probeBindable -->

## Metrics

The `ze_dns_resolver_*` series count queries by transport and RCODE,
refusals by reason, rate-limited and dropped queries, cache hits and entries,
resolutions in progress and upstream exchanges by result.
`ze_dns_resolver_listener_up` is 1 while a listener socket is bound, by protocol
and address with port. The list is in
[Plugin Metrics](../plugin-development/metrics.md#full-inventory).

<!-- source: internal/plugins/dnsresolver/metrics.go -- buildMetrics -->
<!-- source: internal/plugins/dnsresolver/listener_state.go -- listenerStates.change, listenerStates.show -->
