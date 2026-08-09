# Resolution Component

<!-- source: internal/component/resolve/resolvers.go -- Resolvers container -->

The resolution component (`internal/component/resolve/`) consolidates all external
data resolution services under a unified tree. Each resolver keeps its own typed API
and is constructed explicitly at hub startup.

## Structure

| Package | Purpose | Cache |
|---------|---------|-------|
| `resolve/` | `Resolvers` container struct | N/A |
| `resolve/cache/` | Generic TTL cache (map + mutex + expiry) | Shared by Cymru, PeeringDB, IRR |
| `resolve/dns/` | DNS resolver (miekg/dns wire protocol) | Own TTL-from-response LRU cache |
| `resolve/cymru/` | Team Cymru ASN name resolution via TXT DNS | 1h via shared cache |
| `resolve/peeringdb/` | PeeringDB HTTP client for prefix counts | 1h via shared cache + 1s rate limit |
| `resolve/irr/` | IRR whois client for AS-SET expansion | 1h via shared cache |
| `resolve/irr/store/` | Shared IRR prefix store (resolve + PeeringDB discovery + zefs persistence) | zefs `meta/irr/{name}` + in-memory map |

<!-- source: internal/component/resolve/dns/resolver.go -- DNS resolver -->
<!-- source: internal/component/resolve/cymru/cymru.go -- Cymru resolver -->
<!-- source: internal/component/resolve/peeringdb/client.go -- PeeringDB client -->
<!-- source: internal/component/resolve/irr/client.go -- IRR whois client -->
<!-- source: internal/component/resolve/irr/store/store.go -- IRR prefix store -->

## Construction

Hub startup creates a single `Resolvers` struct with one shared DNS instance.
Cymru receives a TXT resolver function wired to the DNS resolver. PeeringDB
and IRR are created independently with their configured server addresses.

<!-- source: cmd/ze/hub/main.go -- newResolvers function -->

## Consumers

| Consumer | Resolver | Entry Point |
|----------|----------|-------------|
| Web UI ASN decoration | Cymru | `decorator_asn.go` via `NewASNNameDecoratorFromCymru` |
| Looking glass graph | Cymru | `LGConfig.DecorateASN` callback |
| Prefix update command | PeeringDB | `prefix_update.go` imports `resolve/peeringdb` |

## Dependencies

```
cymru --> resolve/dns (sibling import, TXT queries)
peeringdb --> resolve/irr (AS-SET name validation)
irr/store --> resolve/irr (PrefixList, LookupPrefixes)
irr/store --> resolve/peeringdb (AS-SET discovery)
```

These are genuine data dependencies, not architectural coupling. `irr/store` is
a subpackage of `irr` precisely so it can import `peeringdb` without a cycle:
`peeringdb --> irr` and `irr` never imports the store, so
`store --> peeringdb --> irr` stays acyclic.

## PeeringDB Prefix Data

<!-- source: internal/component/resolve/peeringdb/client.go -- PeeringDB HTTP client -->

`ze bgp peer * prefix update` fills each peer's prefix maximum from PeeringDB
and applies a margin, default 10%.

- **Query PeeringDB at runtime.** The first design built a data pipeline:
  embedded routing data, a ZeFS store, build scripts and a source-url config
  leaf. Querying the service directly removes all four.
- **The settings live in `system { peeringdb { } }`, not under `bgp`.**
  PeeringDB is an external service, not a BGP concept, and another subsystem
  can use it later.
- **One hidden `updated` leaf per peer, not one per family.** The update
  command refreshes every family of a peer in one call, so a per-family
  timestamp would record the same instant several times.
- **Staleness is fixed at 180 days.** Six months is a defensible interval for
  every operator, and a configurable threshold is one more knob to explain.

`ze bgp peer X detail` reports `prefix-updated` and `prefix-stale`, the
`ze_bgp_prefix_ratio` and `ze_bgp_prefix_stale` gauges follow the same data,
and startup logs a warning per peer with stale data.

`PeerInfo.PrefixUpdated` is populated from `PeerSettings.PrefixUpdated` in the
reactor API adapter. The peer detail handler computes staleness inline, because
it cannot import the reactor package.

## IRR Prefix Store

<!-- source: internal/component/resolve/irr/store/store.go -- PrefixStore -->

`resolve/irr/store` is a shared cache of IRR-resolved prefix lists keyed by name
(an ASN like `AS13335` or an AS-SET like `AS-CLOUDFLARE`). It owns the full
resolution pipeline: AS-SET discovery for bare ASNs via PeeringDB (falling back
to the literal `AS<asn>` name when PeeringDB has no answer), the IRR prefix
lookup, and persistence to zefs under per-entry keys `meta/irr/{name}`.

Consumers (the BGP `filter_irr` plugin, the upcoming `firewall-irr` plugin) are
process-isolated plugins; they do not share a `PrefixStore` instance. Each
builds its own and they share cached data through the zefs file on disk.
In-process writers are serialized by the store's own mutex (each persist flushes
atomically -- in-place for small updates, full rewrite on growth). zefs's `Lock` is an in-process mutex, not a
file lock, so two writer **processes** would clobber each other on flush: exactly
one process may write a given store file until zefs gains a cross-process lock (a
prerequisite for the firewall-irr consumer).

On `Open`, a legacy single-blob cache (`meta/bgp/irr-cache`, keyed by ASN) is
migrated once into per-entry keys, and the legacy key is removed only after
every per-entry key is written.

Three properties of the store keep one bad entry from taking the shared file
down. Each one is a guard, not a convention.

- A name of `.`, or a name that contains `..`, poisons the whole store.
  `irr.ValidateASSetName` permits `.`, so a refresh under that name writes the
  key `meta/irr/.`. The zefs decoder rejects that key and fails the whole file,
  so every other consumer's entries go with it. `validateName` rejects both
  names before they reach a key.
  <!-- source: internal/component/resolve/irr/store/store.go -- validateName -->
  <!-- source: pkg/zefs/store.go -- decode key validation -->
- `Open` keys the in-memory map by the on-disk zefs key segment, never by the
  entry's self-reported JSON `Name`. The file is shared, so a tampered blob
  under `meta/irr/AS13335` that claims `"name":"AS99999"` would otherwise land
  in the AS99999 slot. A segment that disagrees with the name is skipped with a
  warning.
- `Open` reads without a zefs write lock. It takes one only when a legacy blob
  must be migrated. A write lock on every configure delayed the first refresh of
  the BGP filter, and the filter then lost a race against an incoming UPDATE.
  <!-- source: internal/component/resolve/irr/store/store.go -- Open, migrate -->

## CLI

<!-- source: internal/component/resolve/cli/main.go -- resolve CLI dispatch -->

The `ze resolve` offline command exposes all resolvers as standalone tools.
Each subcommand creates a fresh resolver instance, queries, prints, and exits.
No running daemon required.

| Command | Flags | Output |
|---------|-------|--------|
| `ze resolve dns [--server <host>] <a\|aaaa\|txt\|ptr> <name>` | `--server` | One record per line |
| `ze resolve cymru [--dns-server <host>] asn-name <asn>` | `--dns-server` | Org name |
| `ze resolve peeringdb [--url <url>] max-prefix <asn>` | `--url` | `ipv4: N` / `ipv6: N` |
| `ze resolve peeringdb [--url <url>] as-set <asn>` | `--url` | One AS-SET per line |
| `ze resolve irr [--server <host>] as-set <name>` | `--server` | One `AS<N>` per line |
| `ze resolve irr [--server <host>] prefix <name>` | `--server` | One prefix per line |

## DNS Config

DNS resolver configuration comes from YANG (`ze-system-conf.yang`):
`system/name-server` (leaf-list of IP addresses) and `system/dns` with
leaves: `resolv-conf-path`, `timeout`, `cache-size`, `cache-ttl`.

<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system DNS config -->
