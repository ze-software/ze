# GeoDNS

The geodns plugin answers a query with the record set that matches the client
source address. It owns its config model, its DNS answer policy, its metrics,
its `show` command and its doctor check, so removing the plugin directory
removes the whole feature. The listener lifecycle, the client-IP selection and
the longest-prefix matcher come from `internal/core/dnsserver`
(`server-harness.md`).

<!-- source: internal/plugins/geodns/config.go -- parseConfig -->
<!-- source: internal/plugins/geodns/server.go -- geodnsServer, answerQuestions -->

## Config model

A `source` maps a CIDR to a named `host-set`. Many prefixes can point at one
set. Records are not inlined per source, because that duplicates the record
list across every prefix that shares it.

The ze YANG loader has no `leafref`, so the `host-set` reference is `type
string` and `parseConfig` checks that every reference resolves. A dangling
reference stops the commit.

<!-- source: internal/plugins/geodns/config.go -- parseConfig, host-set resolution -->
<!-- source: internal/plugins/geodns/record.go -- addrRecord -->

`parseConfig` also refuses a configured name the wire cannot carry: a label
that is empty (`www..t.example.`) or longer than 63 octets, and a name longer
than 255 wire octets (RFC 2181 section 11, RFC 1035 section 3.1). The packer
would refuse such a name at send time, and that refusal answers nothing, so
the commit fails instead.

<!-- source: internal/plugins/geodns/config.go -- checkName -->

`parseConfig` refuses an SRV record whose target lies in a zone geodns serves
and has no address record a client can draw. RFC 2782, "Target", says "There
MUST be one or more address records for this name", and geodns answers every
name in its zones, so it holds that record or nobody does. A client that drew
the SRV record from one host set draws the target's addresses from the same
set, so the target has to be a host of that set with an A or AAAA record, or
one of the synthesized `ns1..nsN` names of its zone, which every client gets.
A target configured only in another host set, or one that is itself an SRV
host, stops the commit with the line that fixes it. A target outside every
served zone is another operator's name and is not checked.

<!-- source: internal/plugins/geodns/config.go -- checkSRVTargets, srvTargetResolves -->

The record `type` leaf is optional. Without it, each address decides its own
type: IPv4 gives A, anything else gives AAAA. One host line with a mixed
address list gives both an A and an AAAA. An explicit `type` constrains the
family.

Listeners use the ze `zt:listener` grouping, like gnmi, web, mcp, ssh and api.
That gives config-time port-conflict detection through the `ze:listener`
extension at no cost. A `leaf-list listen-address` plus a scalar port was the
first cut and diverged from the ze convention. The Go default is 127.0.0.1:5300
and ::1:5300 when the operator configures no listener.

<!-- source: internal/plugins/geodns/yang/ze-geodns-conf.yang -- listener list -->

## State snapshot

`resolverState` holds the parsed config, the matcher and the SOA serial. It is
published as one atomic snapshot. The server and `show geodns` read the same
snapshot, so operator output never disagrees with what the server answers.

<!-- source: internal/plugins/geodns/state.go -- resolverState, storeState, loadState -->
<!-- source: internal/plugins/geodns/source.go -- buildMatcher -->

## Server

One `"."` mux handler reads the snapshot on each request. The handler holds no
state, so a host-data reload swaps the snapshot with no rebind. Only a change
of the endpoint set stops and rebinds the listeners, tracked by a signature
over each ip:port.

<!-- source: internal/plugins/geodns/server.go -- geodnsServer, apply, stopAll -->
<!-- source: internal/core/dnsserver/manager.go -- endpointSig -->

The response code says where the query name sits, and `ns1..nsN.<zone>` glue is
synthesized from the nameserver list.

| Query name | Answer |
|------------|--------|
| A name a served zone owns, with no record of the type asked for | NOERROR, empty answer, the zone SOA in the authority section |
| A name inside a served zone that the zone does not own | NXDOMAIN, the zone SOA in the authority section |
| A name under no served zone | REFUSED, AA clear, no record in any section (the client-subnet echo below aside) |

A name the zone owns keeps NOERROR whatever the client can see of it. Host sets
are chosen by source prefix, so a name configured in one host set and not
another has data for one client and none for another. Existence is a property
of the zone, so the second client gets no data of this type, never a name
error. The set of existing names is `resolverState.names`, built once per
config generation: each configured host, plus every interior node between that
host and its zone apex.

A client that resolves to nothing takes the same table. `client-ip-source
edns0` answers a query carrying no client-subnet option with no client at all,
and the zero address matches no source prefix, so no host set is selected. Which
zone owns a name does not depend on the client, so the three response codes are
unchanged.

RFC 2308 Section 3 requires the SOA in the authority section on both negative
answers, so a resolver can cache them.

<!-- source: internal/plugins/geodns/server.go -- answerQuestions, nameExists, inZone -->
<!-- source: internal/plugins/geodns/state.go -- buildNames -->

## Client subnet (RFC 7871)

Under `client-ip-source edns0` and `edns0-then-packet` geodns reads the EDNS0
client-subnet option and answers as an RFC 7871 Authoritative Nameserver.
Under `packet` it reads no option, so it neither refuses one nor echoes one
(RFC 7871 Section 7.2.1: a server that has not enabled support "MUST NOT
include an ECS option within replies").

| Query | Reply |
|-------|-------|
| No option | No option, whatever the answer |
| A wrongly formatted option: a FAMILY other than 1 or 2 (FAMILY 0 is accepted with no address bits, the `dig +subnet=0` form), a non-zero SCOPE PREFIX-LENGTH, ADDRESS bits past SOURCE PREFIX-LENGTH, a prefix wider than the family, two options | FORMERR, nothing in any section, no option |
| A well-formed option | The answer, with an OPT holding one option whose FAMILY, SOURCE PREFIX-LENGTH and ADDRESS are the query's |

miekg/dns already refuses an unknown FAMILY and a prefix wider than the family
while it unpacks the query, and its server answers FORMERR before geodns runs.
`ClientIP` holds the same rules for a message that skipped that unpack, and
adds the SCOPE and ADDRESS-bit rules miekg/dns does not check. Too few or too
many ADDRESS octets cannot be seen after miekg/dns unpacked the option: it
pads a short ADDRESS with zeros and drops octets past the address width.

An option with SOURCE PREFIX-LENGTH 0 carries no address, so the selection
treats it as no option: `edns0-then-packet` answers from the packet source and
`edns0` from no client. A query that gave FAMILY 0 as well gets the family of
the packet source in its echo (RFC 7871 Section 7.3), with no ADDRESS octets.

SCOPE PREFIX-LENGTH says which network the answer holds for. It is 0 when the
answer did not come from the option's ADDRESS: no question read a host set
(SOA, NS, a name error, a refusal), or the host set was chosen from the packet
source (RFC 7871 Section 12.1). Otherwise it is the length of the `source`
prefix that matched, made longer where a more specific `source` prefix lies
inside that one, until the network round the client leaves every other prefix
out (`Matcher.Scope`). RFC 7871 Section 7.2.1 says an Authoritative Nameserver
"MUST NOT overlap prefixes" and lets it deaggregate the shorter prefix; this
is that deaggregation, computed for the one network the query falls in. With
10.0.0.0/16 to one host set and 10.0.5.0/24 to another, a query for
10.0.1.0/24 draws scope 22, never 16. A client in no `source` prefix gets the
shortest network round it that holds no configured prefix.

The OPT advertises 512 octets, the read buffer of an authoritative listener,
version 0, with DO clear. geodns adds it from its answer function: the
`Authoritative` wrapper itself still emits no OPT.

<!-- source: internal/core/dnsserver/client.go -- ClientIP, querySubnet, checkSubnet, EchoSubnet -->
<!-- source: internal/core/dnsserver/matcher.go -- Matcher.Scope, commonBits -->
<!-- source: internal/plugins/geodns/server.go -- answerQuery, subnetScope -->

## Zone boundary matching

Zone membership uses `dns.IsSubDomain(zone, name)`, never a string-suffix
comparison, at the query path (`inZone`, reached through `matchZone`) and at
config parse (`hasZoneSuffix`) alike. A suffix comparison places
`evilexample.com.` inside the zone `example.com.`, because the characters match
where the label sequence does not nest. Ze would then answer for a namespace it
serves no zone for.

### The SOA serial is a uint32, which bounds the format

<!-- source: internal/plugins/geodns/server.go -- computeSerial -->

| Mode | Value | Limit |
|------|-------|-------|
| `auto-epoch` (default) | `max(unix, prev+1)` | strictly monotonic at any update rate |
| `auto-datetime` | `YYYYMMDDnn` | 100 updates per day; a four-digit year times 1000 overflows |
| `fixed` | the configured leaf | operator owns it |

A `YYYYMMDDHHMMSS` literal has 14 digits and does not fit a uint32, whose
maximum has 10 digits. Use `auto-epoch` when the rate can exceed 100 per day.

The handler runs each query under `recover()`, so one bad query cannot stop the
daemon, and a rebind or a stop drains with a bounded `ShutdownContext`.

## Observability

Metrics are registered through `ConfigureMetrics(reg any)` into the host
registry and stored in an atomic pointer with a lazy no-op default. The query
path then never nil-checks, and the plugin needs no `init()` outside
`register.go`. `qtypeLabel` collapses an unknown query type to `OTHER`, which
bounds the label cardinality.

<!-- source: internal/plugins/geodns/metrics.go -- setMetricsRegistry, gmetrics, qtypeLabel -->
<!-- source: internal/component/plugin/registry/registry.go -- ConfigureMetrics -->

`show geodns` is owned by the plugin through container merge: the plugin's
command YANG declares `container show { container geodns { ze:command
"ze-geodns:show-geodns" } }` and the handler reads the same atomic snapshot the server
reads.

<!-- source: internal/plugins/geodns/show.go -- show handler -->
<!-- source: internal/plugins/geodns/yang/ze-geodns-cmd.yang -- show geodns command -->

The doctor check binds a capability, not the live port. It warns only when an
enabled geodns has a privileged listener port below 1024 that fails a bind
probe. The default port 5300 produces no diagnostic, so the check never fires
against the plugin's own running listener. Cross-service port conflicts are a
separate mechanism, detected by the `ze:listener` extension at parse time.

<!-- source: internal/plugins/geodns/doctor.go -- geodnsListenDiagnostic -->

The reference implementation counted folder-level parse errors, duplicate hosts
and critical files. YANG validation at commit makes those counters obsolete, so
they are dropped and `config_reload_total` replaces `zone_reload_total`.
