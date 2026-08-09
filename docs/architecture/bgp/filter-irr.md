# The IRR Filter Plugin

Ze had IRR whois and PeeringDB clients and no way to reach the BGP filter chain
with them. An operator ran bgpq4 outside ze, generated config and pasted it in.
`bgp-filter-irr` queries IRR for AS-SET prefixes, applies them as live import
filters, and refreshes on its own.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- plugin entry point, refreshASN -->
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- config parsing from OnConfigure JSON -->
<!-- source: internal/component/bgp/plugins/filter_irr/match.go -- prefix matching -->
<!-- source: internal/component/bgp/plugins/filter_irr/cache.go -- PrefixStore wiring -->

## The decisions

**A new plugin, not an extension of `filter_prefix`.** Static config-driven
lists and dynamic external data are separate concerns, which is the same split
RPKI and RIB already make.

**`resolve/irr` is imported directly, not reached over RPC.** BGP plugins run in
process, so RPC adds latency and isolates nothing. There is no import cycle:
`irr` imports only cache, textbuf and the standard library.

**About 30 lines of prefix matching are duplicated from `filter_prefix`.** Every
`filter_prefix` type is unexported, so sharing would mean exporting them or
extracting a common package, and that couples two independent plugins for a
small saving. Shared matching later means either exporting the types or an
extraction to `filter_common/`.

**Fail closed on an empty result or an error.** Accepting unvalidated routes is
worse than rejecting valid ones for a while. RPKI behaves the same way.

**The operator writes `import bgp-filter-irr:$remote_as`. Nothing is
auto-injected.** The reactor has no dynamic filter injection API, and
`$remote_as` already resolves through the existing filter variable mechanism.

**`refresh-interval` is `uint32`.** The YANG range is 60 to 86400, which exceeds
the `uint16` maximum of 65535.

**Each configure cycle gets its own `refreshStop` channel.** `OnConfigure` fires
on every config commit, and without a stop channel per cycle the refresh
goroutines accumulate.

Removing the `filter_irr/` directory removes every IRR filter feature. The
plugin self-containment test proves it.

## Constraints

**The per-peer `irr { as-set }` YANG augment must cover three paths**: a
standalone peer, a grouped peer, and the group-level session.

**`handleConfigure` publishes every mutable plugin field under `plug.mu`
together**: `byASN`, `prefixStore`, `config` and `refreshStop`. A worker that
captured `st := byASN[asn]` under an RLock must re-read `st = byASN[asn]` under
the later write lock before it mutates. A reconfigure can swap `byASN` in
between, which orphans the captured pointer and loses the refresh result in
silence. This is one bug class, not four separate races.

**The shared prefix store holds entries this plugin never enrolled**, because
other consumers write to the same file. `loadFromStore` keeps the enrollment
gate and applies only the configured ASNs. See `docs/architecture/resolve.md`
for the store itself.

**On a lookup error, `Refresh` returns a non-nil `CachedEntry` alongside the
error**, carrying the resolved AS-SET and no prefixes, so this plugin can still
record the fallback AS-SET. It must not update its in-memory map or the store on
that path.

**The IRR client caches lookups for one hour in memory**, so a `Refresh` inside
the TTL is a silent no-op. A test that must see a lookup failure needs a fresh
client pointed at an unreachable server.

**`strings.Cut(text, "nlri ")` matches a substring.** A test case that means to
lack the token must not contain it anywhere, and `"no nlri here"` does.
