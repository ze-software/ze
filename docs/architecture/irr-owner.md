# The IRR owner plugin

The `irr` plugin (`internal/plugins/irr/`) is the one component that fetches,
schedules and stores the IRR prefix lists Ze enforces. `bgp-filter-irr` and
`firewall-irr` are its consumers: they never fetch, they read what it stored.
A reboot then enforces exactly the lists enforced before it, because a stored
list is loaded at start and fetched again only when the operator's refresh
period says so. This is section 22 of `core-design.md` (sources and read views)
applied literally: the owner is the builder, the stored dataset is the read
view, and a notice tells a consumer to pull it.

## What is in place, and what is not

| Part | State |
|------|-------|
| Registration, `system irr` YANG, `show irr`, `update irr all\|asn\|as-set`, `clear irr asn\|as-set` | Registered (`register.go`, `register_commands.go`, `yang/`) |
| State keys and the owner's write grant | Registered (`pkg/zefs/keys.go`, `register.go`) |
| Each consumer's write grant on its own interest key | Registered (each consumer's `register.go`) |
| Event types `irr/dataset-stored`, `irr/interest-changed` | Registered (`internal/component/resolve/irr/store/notice.go`) |
| Startup subscriptions | The owner subscribes to `interest-changed` and each consumer to `dataset-stored` at stage 5; the handlers only acknowledge |
| Consumers start the owner | Each consumer lists `irr` in `Dependencies` |
| Reading `system irr`, the schedule, the fetch, the store, the notices, the commands | Not yet: every `irr` command answers that it is not available, and both consumers still fetch for themselves |
| Read-only grant, per-key value bound | Not yet: a consumer cannot read the owner's keys |

<!-- source: internal/plugins/irr/irr.go -- runIRR, handleCommand -->
<!-- source: internal/plugins/irr/register.go -- init -->

## Where it sits

The owner is an edge plugin: a config-driven engine nothing imports
(`module-tiers.md`, "The three tiers"). The consumers reach it only through
stored keys, the two notices and its commands, so each works when either side
runs forked. The names both sides share (the plugin name `irr`, the notice
namespace and event types, the payloads) live in
`internal/component/resolve/irr/store`, which both consumers already import;
the dataset codec joins them there.

<!-- source: internal/component/resolve/irr/store/notice.go -- OwnerPlugin, DatasetNotice, InterestNotice -->

## Configuration

`system irr { server; source-address; refresh-interval; }`, augmented into the
system root by `yang/ze-irr-conf.yang`. `server` defaults to `whois.radb.net`;
`refresh-interval` is `0 | 60..86400` seconds, default 3600, and 0 means manual
only: no automatic refresh, while an AS-SET with nothing stored is still
fetched once, and `update irr` fetches on demand. The PeeringDB endpoint is
`system peeringdb url`. A consumer names only the AS-SET or ASN it enforces.
The consumers' own `policy irr` and `firewall irr` server, URL and interval
leaves still exist until the consumers stop fetching.

## Stored keys and notices

| Key or event | Writer | Reader | Holds |
|--------------|--------|--------|-------|
| `meta/irr-data/{server}/{as-set}` | owner | consumers, owner, doctor | One AS-SET's prefixes from one server, compact binary with a SHA-256 digest |
| `meta/irr-set/{as-set}` | owner | consumers, owner, doctor | Which dataset is enforced now; written only after that dataset |
| `meta/irr-asn/{asn}` | owner | consumers, owner, doctor | The AS-SET PeeringDB named for a bare ASN |
| `meta/irr-generation` | owner (`StateIncrement`) | consumers | Raised once per pointer write or removal |
| `meta/irr-interest/{consumer}` | that consumer | owner | The AS-SETs and ASNs the consumer enforces |
| event `irr/dataset-stored` | owner | consumers | AS-SET, server, generation, digest, removed |
| event `irr/interest-changed` | consumers | owner | The consumer that rewrote its interest |

A consumer subscribes at stage 5 and reads in `OnStarted`, so a notice sent
between the two is still delivered. A consumer that sees a generation more than
one past the last it saw re-reads every pointer it uses, which recovers a lost
notice.

<!-- source: pkg/zefs/keys.go -- KeyIRRDataset, KeyIRRSet, KeyIRRASN, KeyIRRGeneration, KeyIRRInterest -->

### Who may touch which key

Each side's `register.go` declares its grants
([process protocol](./api/process-protocol.md), "Persistent runtime state"):

| Plugin | Write | Read only |
|--------|-------|-----------|
| `irr` | dataset, pointer, binding, generation | every `meta/irr-interest/{consumer}` |
| `bgp-filter-irr` | `meta/irr-interest/bgp-filter-irr` alone (`RegisterPluginKeyInstance`) | dataset, pointer, binding, generation |
| `firewall-irr` | `meta/irr-interest/firewall-irr` alone (`RegisterPluginKeyInstance`) | dataset, pointer, binding, generation |

A consumer therefore cannot write the owner's keys or another consumer's
interest, and `PluginKeyOwners`, which names writers only, names the owner for
a dataset and the one consumer for its interest key. Reads go to the daemon
store, not to the owner, so a consumer reads the stored copy while the owner
is stopped.

<!-- source: internal/plugins/irr/register.go -- init -->
<!-- source: internal/component/bgp/plugins/filter_irr/register.go -- init -->
<!-- source: internal/component/firewall/plugins/irr/register.go -- init -->

### The dataset value

`internal/component/resolve/irr/store` holds the codec both sides use, so the
owner writes and the consumers read one format. `NewDataset` validates what
the owner resolved and `Dataset.WriteTo(buf, off)` encodes it into one buffer
sized from `EncodedLen`; `DecodeDataset(key, value)` reads it back.

| Field | Encoding |
|-------|----------|
| version | 1 byte, 1 |
| refreshed-at, checked-at, stale-since | 3 x int64 Unix seconds, 0 for unset |
| as-set, server | uint16 length, then the bytes; the server in its key form |
| ipv4 count, ipv6 count, each followed by its prefixes | uint32, then per prefix one length byte and the significant bytes of the masked address |
| digest | SHA-256 over every byte before it |

The value is bounded by `zefs.IRRDatasetValueMax`, 12,000,000 bytes, declared
on the key, and each family by `DatasetFamilyMax`, 500,000 prefixes: a list
over it is refused whole with both counts in the error and the previous copy
stays. A read refuses, each with its own error, a value that is truncated, over
the bound, of another version, malformed (a prefix length over the family, host
bits set, bytes after the IPv6 list), fails its digest, or names another
AS-SET or server than its key. The caller reads every refusal as "no stored
copy" and never as an empty list.

`DatasetServer` gives the server's key form, so every spelling of one server
names one dataset: the host lowercased without a trailing dot, port 43
dropped, any other port as `:port`, and an IPv6 literal in canonical form with
`_port` for another port, since its own colons would make `:port` ambiguous.

<!-- source: internal/component/resolve/irr/store/dataset.go -- NewDataset, Dataset.WriteTo, DecodeDataset, DatasetServer, DatasetKey -->
<!-- source: pkg/zefs/keys.go -- IRRDatasetValueMax -->

## Tests

`test/plugin/irr-cold-start-stores.ci`, `irr-restart-keeps-bgp-list.ci`,
`irr-restart-keeps-firewall-sets.ci`, `irr-shared-dataset.ci`,
`irr-update-then-firewall-commit.ci` and `test/reload/irr-bgp-commit-new-set.ci`
drive the owner through the daemon. `le mock irr --query-log` records every
whois query so an observer (`internal/test/fixture/irr_owner_fixture.go`) can
prove a restart sent none. They stay red until the owner stores datasets.
