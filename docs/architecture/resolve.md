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

### An empty answer never replaces cached prefixes

An IRR server is a third party, so an unhelpful answer is an expected state
rather than an exception. `Refresh` writes new prefixes only when the lookup
returned some. A lookup that errored, and a lookup that succeeded and returned
nothing, both keep the cached prefixes and set `StaleSince` on the entry; the
second returns `ErrNoPrefixes`. Every consumer therefore keeps enforcing the
last data it had rather than an empty list, which for a firewall interface
binding means a drop-everything ruleset and for the BGP filter means rejecting
every UPDATE from the peer.

The decision is made per FAMILY, because each family is enforced separately.
IPv4 and IPv6 are two queries, and a server that holds no IPv6 route objects
answers the second one exactly as a server having a bad minute does: `D`, which
`lookupFamilyPrefixes` reads as an empty family and not as an error. `commit`
therefore keeps the cached prefixes of any family the answer carried nothing
for, dates the entry by the oldest data it now holds, and sets `StaleSince`. The
lookup still counts as a success, because it did learn prefixes. Without this,
an AS-SET that answered for IPv4 and not for IPv6 replaced the entry wholesale,
and the interface binding that lost its IPv6 accept term dropped every IPv6
packet arriving on the port: `buildIfaceTables` emits one accept term per family
that has prefixes and one drop term that names no family.

`CachedEntry.Stale` reports the condition, and `Purge` is the deliberate way to
remove prefixes for a name that is gone upstream.

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

## RIR Delegation Table

The delegation table maps an AS number to the Regional Internet Registry that
holds it, and to that registry's whois host. Two sources carry it, one format
reads both, and the newer of the two answers.

### The shipped seed

`internal/component/resolve/irr/rir-delegation.txt` is the table the binary
ships. `seed.go` embeds it and parses it once, so a lookup needs no network, no
daemon and no file on disk. Each line is `<start> <end> <registry-token>`. The
registry display name and its whois host are Go constants, so the file carries
the token alone and neither is repeated on 11,000 lines.
<!-- source: internal/component/resolve/irr/seed.go -- seedDelegation, seedTable -->

A comment header opens the file with `Generated:`, the date the registries'
data was collected, and one `Source:` line for each of the five delegation
files it was built from.
<!-- source: internal/component/resolve/irr/rir.go -- delegationTableHeader, RenderDelegationTable -->

`./le iana-asn write` rewrites that file. It reads the five registry delegation
files, parses each one, sorts, collapses adjacent ranges of one registry, and
writes the whole table in one call. It is the one generator whose input is the
network rather than the tree, so it has no check twin: a checkout cannot be
compared against it without asking five registries what they publish today.
<!-- source: internal/le/ianaasn/ianaasn.go -- Write -->

### The stored copy

`update resolve rir` refreshes the table into the managed store under the
`meta/rir/delegation` key. The refresh is all or nothing: a registry that does
not answer, or answers something the parser refuses, stops the run and names
its URL, so the previous table stays whole. A run that reached all five and
took no ASN record from them stops as well, because an empty table is not a
smaller answer, it is every AS number becoming unanswerable. The write goes
through `statestore.Put`, which is the config system's own zefs handle: a
second handle in that process would make the next config flush re-encode from a
stale tree and drop every state key.
<!-- source: internal/component/resolve/cmd/rir.go -- handleRIRRefresh -->
<!-- source: internal/component/resolve/irr/rir.go -- FetchDelegationTable -->

`./le iana-asn write` and `update resolve rir` run the same recipe. The parse,
the collapse, the date and the render all belong to the `irr` package, and both
callers reach them through `irr.FetchDelegationTable`. Two copies of that
recipe existed until 2026-09-02, and each held a guard the other lacked.
<!-- source: internal/component/resolve/irr/rir.go -- FetchDelegationTable, RenderDelegationTable -->

### Which source answers

The stored copy answers when its `Generated:` date is strictly after the
seed's, and the seed answers otherwise. An upgrade that ships fresher data than
the last refresh stored therefore takes over on its own, with nothing to
configure. A stored copy that cannot be parsed is passed over, the seed
answers, and the reason is logged: a half-read table is never used.
<!-- source: internal/component/resolve/irr/stored.go -- preferStoredDelegation -->

Nothing is cached between lookups. The seed is parsed once, because embedded
bytes never change, and the stored copy is read and parsed on every lookup. The
answer after a refresh therefore comes from what that refresh stored, with no
restart to remember and no invalidation to get wrong. The cost is one store
read for each lookup, which an operator command can afford and a wire path
could not.
<!-- source: internal/component/resolve/irr/stored.go -- delegationTable -->

### The two read paths

| Process | How it reads the stored copy |
|---------|------------------------------|
| The daemon, where a state store is registered | `statestore.Get` on the config system's own handle |
| Every other process: the host CLI, a plugin process | `{config-dir}/database.zefs` opened read-only, and only if that file exists |

<!-- source: internal/component/resolve/irr/stored.go -- storedDelegation, storedDelegationFile -->

Neither path writes. `zefs.Open` memory-maps the file and takes no lock, so the
host reads a store the daemon holds open without blocking it. A file that
exists and cannot be read answers "nothing stored" and says so, because a
corrupt or half-written store is not evidence that nobody refreshed.
<!-- source: internal/component/resolve/irr/stored.go -- storedDelegationFile -->

### Three answers, never two

A lookup has three outcomes and every caller branches on all three: the
registry that holds the AS number, `ErrASNUnallocated` for a table that was
read and holds no range covering it, and any other error for a table that could
not be read. "Nobody holds this AS number" and "I could not find out" are
different answers, and neither is ever reported as the other.
<!-- source: internal/component/resolve/irr/rir.go -- RegistryForASN, ErrASNUnallocated -->

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
| `ze resolve rir <asn>` | none | Registry name and its whois server |

`ze resolve rir` is the one subcommand that reaches no network. It reads the RIR
delegation table, so it answers with the cable unplugged. An AS number in no
delegated range and a table that cannot be read are two different answers: the
first names the range, the second names the table, and both exit 1. The table it
reads is the copy `update resolve rir` stored, when that copy was generated after
the seed the binary ships, and the shipped seed in every other case. On the host
the stored copy is read from `database.zefs`, read-only, and only if that file
exists.
<!-- source: internal/component/resolve/irr/rir.go -- RegistryForASN -->
<!-- source: internal/component/resolve/irr/stored.go -- preferStoredDelegation -->

### The two daemon commands

Inside the daemon the same table answers two commands. Both are declared in
`internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`, as a `container`
carrying a `ze:command` extension, which is how every `ze-show:` and
`ze-update:` method of this component is declared.

| Command | Wire method | Answers |
|---------|-------------|---------|
| `show resolve rir <asn>` | `ze-show:resolve-rir` | `asn`, `registry`, `whois`, `range-start`, `range-end` |
| `update resolve rir` | `ze-update:resolve-rir` | `key`, `ranges`, `generated` |

<!-- source: internal/component/resolve/cmd/register_rir.go -- the two RPC registrations -->
<!-- source: internal/component/resolve/cmd/rir.go -- handleRIRASN, handleRIRRefresh -->

Both answer structured data, so `| json`, `| yaml` and `| table` each render
it. A refresh that stored nothing reports an error and never reports success:
`statestore.Put` answers `(false, nil)` when no store is registered, and that
is a failure to store rather than a successful one.
<!-- source: internal/component/resolve/cmd/rir.go -- handleRIRRefresh -->

## DNS Config

DNS resolver configuration comes from YANG (`ze-system-conf.yang`):
`system/name-server` (leaf-list of IP addresses) and `system/dns` with
leaves: `resolv-conf-path`, `timeout`, `cache-size`, `cache-ttl`.

<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system DNS config -->
