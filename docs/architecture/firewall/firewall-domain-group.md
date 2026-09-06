# Firewall Domain Groups

<!-- source: internal/component/firewall/plugins/domain/domain.go -- firewall-domain plugin entry point -->

A domain group populates an nftables set from what DNS names resolve to. This
page records the structural decisions behind it. The operator-facing description
is `docs/guide/firewall.md`, "DNS-Sourced Address Groups".

## Why a plugin

`firewall-domain` is a plugin under `internal/component/firewall/plugins/`,
beside `firewall-irr`, and it runs as its own process.

Every firewall feature that owns a timer is already a separate process, and that
directory is a codegen discovery path, so the registration machinery is proven
rather than invented: `registry.Register` with `RunEngine`, YANG glue,
`pluginserver.RegisterRPCs`, `firewall.RegisterTables` and
`diagnostic.RegisterDoctorCheck` each have a working precedent one directory
over.

The process boundary also bounds the blast radius. DNS answers are
attacker-influenced input reaching a parser, and a panic there costs one refresh
cycle rather than the daemon.

The Component Boundaries table in `docs/architecture/core-design.md` carries no
row permitting a firewall-to-resolve import, which is the other reason an in-hub
component was rejected.

## One resolver, reached over an RPC

A plugin must NOT build its own `Resolver`. A second one would double the query
load against the upstream server and give the two copies different views of the
same TTL, and the shared DNS cache was the whole argument for keeping this work
near the hub.

So the plugin asks the hub, over one call in the shape the other plugin-to-hub
calls already use:

| Piece | Where |
|-------|-------|
| Method constant, input and output types | `pkg/plugin/rpc/types.go`, `MethodResolveDNS` |
| Handler slot the hub fills | `pkg/plugin/rpc/bridge.go`, `RegisterDNSResolver` / `GetDNSResolver` |
| Plugin-side call | `pkg/plugin/sdk/sdk_engine.go`, `Plugin.ResolveDNS` |
| Engine-side handler | `internal/component/plugin/server/dispatch_resolve.go`, `opResolveDNS` |
| Registry entry the three transports derive from | `internal/component/plugin/server/dispatch_registry.go` |
| The one call that wires the real resolver in | `cmd/ze/hub/main_system.go`, `registerPluginDNSResolver` |

The handler reaches the resolver through the slot in `pkg/plugin/rpc` rather
than a field on `Server`, because the Component Boundaries table admits `aaa`
and `audit` into `internal/component/plugin/server` and nothing else. The hub
owns the wiring, because the hub is where the single resolver is built.

There is no typed DirectBridge slot, matching `route-install`: this is a
TTL-scheduled control-plane call and its only caller runs forked.

## The resolver had to learn to say why

`query` in `internal/component/resolve/dns/resolver.go` discarded `resp.Rcode`
and returned `(nil, 0, nil)` for every non-success code, so NXDOMAIN, SERVFAIL
and REFUSED were one signal.

A firewall cannot act on that. The same value would have to mean both "empty the
set" and "keep the last good answer", so either a failed server silently empties
a live filter or a deleted name enforces stale addresses forever.

`ResolveWithTTL` now returns a `dns.Status` beside the records and the TTL.
`Resolve`, `ResolveA`, `ResolveAAAA`, `ResolveTXT` and `ResolvePTR` keep their
signatures: they call `query` directly and ignore the new value.

`Status` is a typed enum, not the numeric RCODE. Its zero is
`StatusUnspecified`, which is what an error return carries, so a caller that
reads the status without checking the error gets a value no branch accepts
rather than a plausible one. `Authoritative` is the question every caller
actually asks: does this answer describe the NAME, or the server.

## The unit is a name in a family

`nameKey` (`schedule.go`) is a group, a DNS name inside it, and a family. It is
what the schedule arms, what the cache stores, and what a change-log record
names. One type rather than two, because a refresh unit and a cache entry are
the same tuple.

The family is part of it because a TTL is a per-answer property: a name with an
A record at 60 seconds and an AAAA record at an hour is asked for on both
clocks, and a change in one family neither reschedules nor reprograms the other.

One worker drives the whole schedule. It sleeps until the earliest-due unit and
wakes for that one, so N names and two families cost one goroutine and one timer
rather than 2N of each.

## Two stores, because they are two shapes

| What | Where | Why |
|------|-------|-----|
| Last-good addresses | zefs, `meta/firewall/domain-group/{group}/{name}/{family}` | Small, bounded, per-group state that must survive a restart |
| Change log | `<config-dir>/firewall-domain-group.dns.jsonl` | Append-only and unbounded in shape |

zefs has no append: `BlobStore.WriteFile` replaces a whole value, and a value
that outgrows its capacity headroom forces a rewrite of the entire store through
a temp file and a rename. A log that grows would pay that on every entry.
`docs/architecture/zefs-format.md` already names `internal/core/audit` as a
raw-filesystem exception for the same reason, and this log follows it.

The log is NOT the operator audit log. That one records operator actions and is
bounded by how often a person acts; a rotating name is neither, and sharing the
file would evict commit history to record DNS churn.

## What earns a write

The steady state writes nothing. Three things earn a zefs write, and each is
bounded:

- **The addresses changed.** Once per actual move.
- **The first answer for a name and family**, even when it carries no address.
  "Never asked" and "asked, and the name holds nothing" are different facts, and
  only a written entry tells them apart: an IPv4-only name answers NOERROR-empty
  for AAAA forever, so without this its IPv6 family would read as unqueried for
  the life of the box. Once per entry.
- **The failing state flipped.** `FailingSince` is written when a name stops
  answering and cleared when it recovers. The addresses alone cannot say it: a
  name whose server has been down for a week holds exactly what it held before.
  Once per outage, at each end.

A name answering the same addresses every 60 seconds matches none of them.

## A group that never resolved is refused at commit

The registry's policy for a rule naming a set no owner supplies is to hold the
whole table back and warn (`dropTablesMissingAProvidedSet`,
`internal/component/firewall/registry.go`). Its own comment states the reason:
an unfiltered port beats a blackholed one.

That is right when the alternative is a blackhole and wrong here, because the
operator would learn their filter is not in the kernel from a log line, after
the traffic it was written for has passed. So `OnConfigVerify` reads the cache
and refuses the commit, naming the group and the command that resolves it. It
performs no network I/O: verify runs while the commit is held open, so a DNS
round trip there would put every timeout it can hit in the path of an operator
pressing return.

An empty set was rejected as an alternative because it inverts meaning: an empty
deny matches nothing, and an empty permit blocks everything.

A daemon START reads its config through `OnConfigure` alone, so no verify runs.
`warnUncachedGroups` logs the same sentence for every affected group instead.

## A rule reaches the set through a leaf, not through `@name`

`source-domain-group` and `destination-domain-group` are leaves under the
firewall's `from` block, and `domainSetMatch`
(`internal/component/firewall/config.go`) turns each into a `MatchInSet`
carrying `ProvidedType`.

The plain `source-address "@name"` route does not work for a set another owner
supplies. `validateMatch` refuses a match against a set the table does not
declare unless `ProvidedType` is set, because the two owners meet only at
`ApplyAll`, so every domain-group rule would be refused at verify with "match
references unknown set".

`DomainGroupSetNames` is exported from the firewall package and called by both
the parser and the plugin, so the two cannot spell `domain_v4_<group>`
differently. A divergence would leave every rule naming a set no owner supplies.

`expandProvidedTermV6` emits the IPv6 twin of a term matching a provided IPv4
set. It was `expandIRRTermV6` until this feature added the second owner; the
prefix pairs it knows are a table in the same file that already enumerates the
leaves producing them.

## The name beside the address

`SetElement` (`internal/component/firewall/model.go`) has nowhere to record
where an address came from, and a provenance field on it is a per-feature edit
to a shared field list that copp, policy-routes, flowspec, vrrp and
firewall-irr would all carry unused. `ai/rules/principles.md` refuses that
shape.

The show enricher registry is the route instead. The plugin declares an enricher
at registration (`EnricherDecl{Command: "show firewall ruleset", Key:
"domain-group"}`), the engine registers a proxy for it
(`internal/component/plugin/server/enricher.go`), and
`handleShowFirewallRuleset` calls `show.Enrich`. Remove the plugin and the
column disappears with it.

Two changes to that handler were needed. Its `Data` map carried `table`,
`family` and `chains` and no set elements at all, so an address had nowhere to
appear before a name could be attached to it; and it did not call `show.Enrich`.

The enricher answers with the whole `sets` value rather than a side map, because
`registerProxyEnrichers` merges with `maps.Copy` at the top level. It copies
through every set it does not own, so the other owners' sets render exactly as
they did.

## Related

- `docs/guide/firewall.md` -- the operator-facing description
- `docs/architecture/firewall/firewall-irr.md` -- the plugin this one is
  templated on
- `docs/architecture/zefs-format.md` -- the store, and the raw-filesystem
  exception the change log follows
- `docs/architecture/api/process-protocol.md` -- the plugin-engine RPC table
