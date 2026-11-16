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
| Startup subscriptions | The owner subscribes to `interest-changed` and re-reads every interest key on a notice; each consumer subscribes to `dataset-stored`. both consumers read on it |
| Consumers start the owner | Each consumer lists `irr` in `Dependencies` |
| Reading `system irr`, the schedule, the fetch, the store, the notices, the commands, the conversion of `meta/irr/{name}`, the doctor check | In place (`config.go`, `owner.go`, `schedule.go`, `datasets.go`, `command.go`, `legacy.go`, `doctor.go`) |
| Read-only grant, per-key value bound | In place |
| The refresh metrics `ze_irr_refresh_outcomes_total` and `ze_irr_data_age_seconds` | The owner's (`metrics.go`); no consumer registers them |
| Completion of AS-SET names on the consumers' AS-SET leaves and after `update irr as-set` and `clear irr as-set` | In place (`completion.go`, `register_completion.go`, `register_commands.go`) |
| `bgp-filter-irr` reading the owner's keys and writing its interest | In place: it holds no IRR or PeeringDB client (`filter_irr/stored.go`) |
| `firewall-irr` reading the owner's keys and writing its interest | In place: it holds no IRR or PeeringDB client and no key of its own beyond its interest (`firewall/plugins/irr/stored.go`) |

<!-- source: internal/plugins/irr/irr.go -- runIRR -->
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
`system peeringdb url`. A consumer names only the AS-SET or ASN it enforces:
neither consumer's YANG carries a server, URL or interval leaf, because no
consumer fetches.

The config reaches the owner through the SDK's config transaction
(`sdk.NewConfigTx` over `parseOwnerConfig`): at boot and on every commit. A
commit that leaves the owner's config unchanged installs nothing, so no
schedule moves (AC-4). Verify refuses a server no dataset key can name and an
interval outside `0 | 60..86400`. A new server is asked for every referenced
set at once; a new source address or PeeringDB URL keeps every stored copy,
and a binding learned from another URL reads as absent, so its AS number is
asked again.

<!-- source: internal/plugins/irr/config.go -- parseOwnerConfig, applyIRRBlock, readSeconds -->

## The worker

One goroutine, started by `OnStarted` and stopped by the plugin's context,
owns everything the owner knows: the config, every consumer's interest, the
pointers, bindings and datasets read from the store, and the schedule. A config
install and an interest notice set a flag and wake it (a channel of depth
one); a command runs as a closure on it, so a fetch a command starts is the
only fetch of that set. `show irr` and the doctor callback read a snapshot the
worker publishes after each pass, so they answer while it fetches. The first
snapshot is published right after the worker's first read of the store and
before its first fetch; until then both refuse ("has not read its stored IRR
datasets yet") rather than answer the empty snapshot, whose blank server,
interval 0 and empty dataset list are neither the config nor the store.
Nothing is read or fetched inside the startup handshake.

At its first pass the worker converts the old firewall lists, then reads every
interest key, every pointer with the dataset it names, and every binding. Each
dataset of the configured server seeds the resolver as the copy the next
answer merges into, so the per-family last-known-good rule
(`store.PrefixStore.Refresh`, `Seed`) holds across a restart. A config that
changes the server builds a new resolver and seeds it the same way, from every
stored dataset of the new server, so a server changed and changed back still
merges the next answer into the copy it stored. An answer the store refused
puts the resolver's copy back to the stored dataset, so the next answer merges
into what the consumers enforce.

<!-- source: internal/plugins/irr/owner.go -- owner, run, absorb, installConfig, plan, fetchDue -->
<!-- source: internal/plugins/irr/datasets.go -- load, readInterest, readPointers, readDataset, readBindings -->

## When a dataset is fetched

The owner fetches the union of the consumers' interest: every AS-SET named, and
the AS-SET bound to every bare AS number named. A set is due at its
`checked-at`, the last time the server answered for it, plus
`refresh-interval`; never at `refreshed-at`, which dates the oldest prefixes a
dataset holds and would make a one-family set due on every pass. A referenced
set with nothing stored, or stored from another server, or unreadable, is due
at once, whatever the interval. With `refresh-interval 0` a stored set has no
due time and `show irr` reports `manual`.

| Outcome of a fetch | Stored copy | `checked-at` | Next fetch |
|--------------------|-------------|--------------|-----------|
| The server answered prefixes | replaced, per-family merge | now | a full interval |
| The server answered nothing for a family | that family kept, `stale-since` set | now | a full interval |
| The server answered nothing, nothing stored | none written (an empty list rejects everything) | not stored | a full interval |
| The server did not answer | kept | unchanged | one minute |
| The list is over 500,000 prefixes in a family | kept, the answer refused whole | unchanged | a full interval |
| The store refused a write | kept | unchanged | one minute |

A bare AS number with no binding is asked of PeeringDB at once. Its answer,
the literal `AS<asn>` when PeeringDB holds no set, is stored as the binding;
with PeeringDB unreachable and nothing bound the literal is bound too, so the
queried set does not flip with each outage. A bound AS number is asked again
only when its set is due, and a PeeringDB failure then keeps the stored set.
A PeeringDB failure, or a question that panicked, is a doctor finding under the
binding's key until PeeringDB answers.

A binding PeeringDB moves to a set with no pointer is not written yet: the
consumers read the stored binding, and one naming a set nothing stored reads as
no list. The owner fetches the new set first, in the same pass, and writes the
binding once that set's pointer is stored. Until then the stored binding still
names the set the consumers enforce, whatever PeeringDB URL it was learned
from (`store.ReadBinding` does not judge the URL), so that set is still
fetched when due, and its due time asks PeeringDB again. A fetch that leaves
the new set with no list Ze can store (no answer, an empty one, one over the
caps, a refused write) is a `doctor-irr-dataset-unpublished` finding under the
binding's key that names the move, fixed by `update irr asn <n>`; it clears
once the binding is written, or once PeeringDB names the stored set again. A
first binding, with no list to keep, is written at once.

<!-- source: internal/plugins/irr/schedule.go -- dueOf, dueAfterAnswer, retryInterval -->
<!-- source: internal/plugins/irr/owner.go -- refreshSet, discoverASN, rediscoverBoundTo, bindTo -->

## Writing a dataset

A dataset key ends in the hex SHA-256 the value ends with, the digest its
pointer carries, so each distinct value has a key of its own and a write never
replaces the bytes the current pointer names. That makes the pointer flip the
only switch: until the new pointer is written, every reader still reads the old
pointer and the old dataset, both intact, whether the server changed or not.

Each write runs in one order: the dataset under its digest key, then
`StateIncrement` on the generation, then the pointer carrying that generation,
then each binding waiting to move to the set, then the notice. A binding moves
before the notice because the notice is what makes a consumer read every
binding again. The generation is raised before the pointer because the
pointer carries it; a pointer write that then fails leaves a generation no
pointer names, which a consumer reads as a gap and recovers by reading every
pointer. A failed generation or pointer write also removes the dataset it just
wrote, which no pointer names. After the flip, the dataset the old pointer
named is removed, unless the new value is byte for byte the old one and so
shares its key. At start, after reading every pointer, the owner removes every
dataset key no pointer names, except under an AS-SET whose pointer is listed
but could not be read: a store write or removal that failed, or a crash
between two writes, leaves no copy behind for longer than one start. Every
write logs `irr dataset stored` with the AS-SET, server, counts and
generation.

AS-SET names are keyed in upper case (`store.ASSetName`), so a consumer that
wrote `AS-Foo` and an operator who typed `AS-FOO` name one dataset.

<!-- source: internal/plugins/irr/datasets.go -- writeDataset, removeTorn, removeUnpointed, removeSet, announceRemoval, moveBindingsTo, seedStored, emitNotice -->
<!-- source: internal/component/resolve/irr/store/pointer.go -- ASSetName, Pointer, Binding, Interest -->

## Commands

`update irr as-set <s>` fetches the set now, referenced or not, and answers
its `show irr` row. `update irr asn <n>` asks PeeringDB, stores the binding and
fetches the set. `update irr all` does both for everything referenced and
names each set that failed. `clear irr as-set <s>` removes the pointer, then
the dataset, raises the generation and sends a notice with `removed` true; its
answer names every consumer whose interest still references the set.
`clear irr asn <n>` removes the stored copy of the set the AS number is bound
to, then the binding, and drops the binding's findings, whose fix `update irr
asn` would undo the clear. The set goes first, so a removal of it the store
refuses changes nothing: the binding still names the set the consumers
enforce, and the answer says nothing was cleared. A binding the store refuses
to remove after its set went is held with the set, and the answer names the
same clear, which finishes it. A cleared set is held: it is not
fetched again until `update irr` or an interest that newly names it. The hold
lives in the worker, so a restart fetches a still-referenced cleared set again.
A pointer removal the store refuses changes nothing: the set stays stored,
scheduled and seeded as the next answer's merge base. A removal whose
generation the store refuses removed the copy and told no consumer: the set is
held, the answer says so, `doctor-irr-dataset-unpublished` names `clear irr
as-set <s>` as its fix, and that clear, with nothing left to remove, raises the
generation and sends the notice.
`clear irr cache` and `clear peeringdb cache` reach the owner's clients
through `OnResolverCacheClear`; the stored datasets stay.

<!-- source: internal/plugins/irr/command.go -- handleCommand, updateASSet, updateASN, updateAll, clearASSet, clearASN -->

## Completion

Where an operator types an AS-SET name, Tab offers the AS-SETs the owner knows:
every name a consumer's interest key holds (`meta/irr-interest/{consumer}`,
decoded with `store.DecodeInterest`; its bare AS numbers are not offered) and
every name a pointer key names (`meta/irr-set/{as-set}`), once each, in the upper
case they are keyed in. Each one says whether a dataset is stored ("AS-SET with
a stored IRR dataset") or only a consumer names it so far ("AS-SET a consumer
references, nothing stored yet").

| Where | How it is reached |
|-------|-------------------|
| `bgp ... session irr as-set`, firewall-irr's `firewall irr interface <name> source-as-set`, and the `from source-as-set` and `from destination-as-set` of a firewall term | Each leaf carries `ze:validate "irr-as-set"`, which the owner declares as a suggestion (`configyang.RegisterSuggestion`, `register_completion.go`). It refuses nothing: an AS-SET nothing stored or referenced yet stays valid, because writing it is how an operator asks for it |
| `update irr as-set <name>`, `clear irr as-set <name>` | The owner's two `RPCRegistration`s carry `Complete: completeASSetArgument`, so `system command complete args` (what `ze cli` asks at a Tab) and the SSH tree's value hints answer them ([command completion](./cli/command-completion.md), path 3) |

Both read the daemon's store, under the owner's own grants (`daemonStore`:
`statestore.PluginList` and `PluginKeyReadable`), and never the worker. The
editor and the dispatcher run in the daemon, and the store is the daemon's, so
the names are the same when the owner runs forked and while it is stopped. A
process with no store offers none, and the editor prints the leaf's type hint.
A listing that fails offers none and logs a WARN with
`doctor-irr-dataset-unreadable`; an interest value that cannot be decoded costs
only its own names, and is logged the same way.

<!-- source: internal/plugins/irr/completion.go -- knownASSets, pointerNames, interestNames, daemonStore, daemonKnownASSets -->
<!-- source: internal/plugins/irr/register_completion.go -- asSetValidatorName, asSetValues, asSetHelp -->
<!-- source: internal/plugins/irr/register_commands.go -- completeASSetArgument -->

## Converting the firewall's stored lists

Before the owner, `firewall-irr` stored each list as JSON under
`meta/irr/{name}`. At its first pass the owner converts each one under the
configured server, with `checked-at` set to the entry's `refreshed-at` so the
set is due no later than it was, writes the pointer and, for an `AS<asn>`
name, the binding, then removes the old key. A set that already has a pointer
keeps it, and a key the owner cannot convert stays in place. The owner holds a
write grant on `meta/irr/{name}` for this. `meta/bgp/irr-cache` is not
converted.

<!-- source: internal/plugins/irr/legacy.go -- convertLegacy, convertLegacyKey -->

## Doctor

The owner declares one runtime check, `irr-datasets`, answered from its live
state through the doctor callback, because the owner may run forked. It reports
`doctor-irr-dataset-unreadable` (a stored value failed its read, decode or
digest and reads as absent), `doctor-irr-refresh-overdue` (the server or
PeeringDB did not answer when due), `doctor-irr-dataset-oversized` (an answer
over the caps was refused) and `doctor-irr-dataset-unpublished` (the store
refused a write, or the consumers were not told). Each finding carries its fix
command: `update irr asn <n>` for one under a binding's key. A finding clears
with its cause: an unreadable pointer or dataset once the set is stored again,
an unreadable binding once a binding is written under its key, a refused fetch
once the server answers, a PeeringDB failure once PeeringDB answers. A cleared
set's findings go with it, a cleared AS number's binding findings go with the
binding, and so do the findings of a set
no consumer names and no pointer names, and of a binding of an AS number no
consumer names: nothing fetches those again. A finding under a stored key that
is still unreadable stays. A stopped owner is reported by the engine's
plugin-stopped check.

<!-- source: internal/plugins/irr/doctor.go -- datasetCheckDecl, irrDiagnosticCodes, answerDoctorCheck, findings -->

## Stored keys and notices

| Key or event | Writer | Reader | Holds |
|--------------|--------|--------|-------|
| `meta/irr-data/{server}/{as-set}/{digest}` | owner | consumers, owner, doctor | One AS-SET's prefixes from one server, compact binary ending in a SHA-256 digest; `{digest}` is that digest in lower-case hex, the one the pointer carries |
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
AS-SET, server or digest than its key. The caller reads every refusal as "no stored
copy" and never as an empty list.

`DatasetServer` gives the server's key form, so every spelling of one server
names one dataset: the host lowercased without a trailing dot, port 43
dropped, any other port as `:port`, and an IPv6 literal in canonical form with
`_port` for another port, since its own colons would make `:port` ambiguous.

<!-- source: internal/component/resolve/irr/store/dataset.go -- NewDataset, Dataset.WriteTo, DecodeDataset, DatasetServer, DatasetKey -->
<!-- source: pkg/zefs/keys.go -- IRRDatasetValueMax -->

### What a consumer reads

A consumer reads the owner's values through one shared reader,
`internal/component/resolve/irr/store` `consumer.go`, so the order and the
meaning of each answer are declared once. It takes only the read half of the
SDK's state RPCs (`store.StateReader`, `StateGet`), all a read-only grant
allows:

| Function | Reads | Answers |
|----------|-------|---------|
| `ReadGeneration` | `meta/irr-generation` | The last generation raised, 0 when none was; a value that is not 4 bytes is `ErrGenerationMalformed` |
| `ReadBinding` | `meta/irr-asn/{asn}` | The set PeeringDB named for a bare AS number, or `ErrNotStored` |
| `ReadPointer` | `meta/irr-set/{as-set}` | The pointer, or `ErrNotStored` |
| `ReadDataset` | the dataset a pointer names | A `StoredSet` (pointer and decoded `Dataset`), refused when the dataset fails its own digest or decode, or is not the one the pointer names (`ErrDatasetNotPointed`). A missing dataset reads the pointer once more: the owner removes the replaced dataset right after a flip, so a pointer that moved is followed and its dataset answered with it; one that did not move is refused (`ErrDatasetNotPointed`), and one `clear irr` removed answers `ErrNotStored` <!-- source: internal/component/resolve/irr/store/consumer.go -- ReadDataset --> |
| `ReadSet` | pointer, then dataset | `ReadPointer` then `ReadDataset` |
| `PublishInterest` | writes `meta/irr-interest/{consumer}`, then emits `irr/interest-changed` | The first error |
| `DecodeDatasetNotice` | an `irr/dataset-stored` payload | The notice, its AS-SET in `ASSetName` form; generation 0 refused |

The contract a consumer keeps:

- `ErrNotStored` is the only answer meaning the owner holds nothing for the
  name (not fetched yet, or `clear irr` removed it): enforce no list. Every
  other error means a stored value exists and cannot be used: keep what is
  enforced, log a WARN with a doctor code, and report it (AC-22).
- Every name passes through `store.ASSetName` before a key is formed.
- Subscribe to `irr/dataset-stored` at stage 5, read at `OnStarted`, and read
  `ReadGeneration` before the pointers of a full pass. A notice whose
  generation skips one the consumer has not applied means a notice was lost:
  read every pointer again.
- On every notice, read the binding of every bare AS number again: a binding
  that moved is announced only by the notice of the set it moved to.
- A pointer equal to the one the enforced copy was read under needs no
  dataset read.
- Never read or write under a lock the filter evaluator or a publish takes:
  a state RPC can take the SDK's whole timeout under disk load (AC-18).

`bgp-filter-irr` follows it in `filter_irr/stored.go` (`storeReader`), and `firewall-irr` in `firewall/plugins/irr/stored.go` (`storeReader`);
`docs/architecture/bgp/filter-irr.md` and `docs/architecture/firewall/firewall-irr.md` describe their passes.

<!-- source: internal/component/resolve/irr/store/consumer.go -- ReadGeneration, ReadBinding, ReadPointer, ReadDataset, ReadSet, PublishInterest, DecodeDatasetNotice -->
<!-- source: internal/plugins/irr/metrics.go -- setMetricsRegistry, countOutcome, publishMetrics -->

## Tests

`test/plugin/irr-cold-start-stores.ci`, `irr-restart-keeps-bgp-list.ci`,
`irr-restart-keeps-firewall-sets.ci`, `irr-shared-dataset.ci`,
`irr-update-then-firewall-commit.ci` and `test/reload/irr-bgp-commit-new-set.ci`
drive the owner through the daemon. `le mock irr --query-log` records every
whois query so an observer (`internal/test/fixture/irr_owner_fixture.go`) can
prove a restart sent none. The BGP restart observer
(`register_irr_restart_keeps_bgp_list.go`) also requires `show bgp irr` to
report the stored set enforced and a peer's UPDATE announcing a listed prefix
to arrive accepted, with no reject (and so no hold) in the restart's log. The
owner stores what an interest key or
`update irr` names. Both consumers write their interest key and read the
owner's copies. `firewall-irr` reads nothing and programs nothing inside the
startup handshake: its first read is at `OnStarted`, so a start with stored
copies programs them once and logs no WARN.

<!-- source: internal/component/firewall/plugins/irr/stored.go -- storeReader, pass, passNow -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- handshakeConfigure, verifyStored -->

`internal/plugins/irr/irr_test.go` drives the worker's steps synchronously
over a fake store, a fake notice sink and a fake resolver: the write order,
a failed fetch, an empty answer, a restart with nothing due, a server change,
the conversion, `clear irr` naming its consumers, and a bound AS number not
asked at start. `failure_paths_test.go` proves a refused `clear irr` and a
server changed and changed back keep the per-family merge base, an
unannounced clear holds the set and is told by the next clear, findings clear
with their cause, a PeeringDB failure is a finding, a moving binding is
written only after its new set's pointer, a move that cannot complete after a
PeeringDB URL change keeps the enforced set refreshed and is a finding, and a
`clear irr asn` whose set removal is refused keeps the binding. `discovery_test.go` follows PeeringDB across due times, never
falls back to a set a consumer dropped, and keeps the stored copy when an
`update irr` fails. `command_asn_test.go` runs `update irr asn` and `clear irr
asn` on a running worker: a value outside the AS number range is refused
before any question is asked, and the forwarders recover the value the
dispatcher bound. `schedule_test.go` proves the due times.
`completion_test.go` reads the completion source over a fake store, then drives
the editor's `Completer.Complete` and the command completer over a daemon store
it wrote; `test/plugin/irr-as-set-completion.ci` asks a daemon whose owner runs
forked to complete both AS-SET commands.
