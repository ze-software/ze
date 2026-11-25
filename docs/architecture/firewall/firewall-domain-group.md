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

The worker starts from `OnStarted` and never from `OnConfigure`. Its first act is
a resolve, a resolve is an engine call, and `OnConfigure` runs while the engine is
waiting for its response. A name with no cached answer is due immediately, so a
worker started at config time asks for it at once, and the startup coordinator
reads that request where it expects the plugin's `ready`: the daemon then refuses
to start with `stage 5: expected ready, got ze-plugin-engine:resolve-dns`.
`OnStarted` is the callback the SDK names as the safe place for an engine call.

The handshake's `OnConfigure` only holds the config (`holdStartupConfig`).
`OnStarted` binds the state RPCs, then configures: it reads the stored answers,
programs the tables from them, and arms the schedule, and only then starts the
worker. A configure during the handshake could not read the store, so it would
arm every unit as unanswered and due at once, and the reset keeps a due time it
already holds: the start would then ask upstream for names whose stored answer
is still valid. A nudge that arrives before the worker exists is not lost,
because the worker reads the schedule as its first act.
<!-- source: internal/component/firewall/plugins/domain/domain.go -- runFirewallDomain, holdStartupConfig, configure, startRefreshWorker -->

## A start asks only for the answers that expired

Each stored answer carries `expires-at`: the time it arrived plus
`max(TTL, ttl-floor)`, capped at one day, which is the time the schedule asks
again. `schedule.reset` arms a unit it does not hold yet at that stored expiry
while it is still ahead, and due at once otherwise: when it has passed, when no
answer is stored, and when the stored answer predates the field. A reboot
therefore keeps enforcing the addresses it enforced before and asks upstream
only for the names whose answer RFC 1035 Section 3.2.1 no longer lets it keep.
A stored expiry further out than one day is armed at one day, the same ceiling
a live answer has: only a clock that stepped back or an edited store writes one.

The TTL bounds this determinism. A name whose stored expiry passed while the box
was down is asked at start, and a short TTL makes that every name.
<!-- source: internal/component/firewall/plugins/domain/schedule.go -- reset, startDue -->

A stored answer that exists and cannot be read or decoded reads as unresolved:
its name is asked at once, its WARN carries
`doctor-firewall-domain-group-unreadable-data`, and `ze doctor` names it until
the next answer rewrites it.
<!-- source: internal/component/firewall/plugins/domain/cache.go -- open, expiry, unreadableFor -->

## A write the store refuses

`store.put` keeps memory and the store equal: when the write fails, the answer
is not kept in memory either. So a changed answer the store refuses is not
programmed, the change log records nothing, and the name goes on enforcing
what it held. The schedule still asks again at the answer's expiry. The same
holds for a confirming answer, whose renewed expiry is lost, and for the
failing marker of an outage.

`put` records each refused unit with its error and time, and the next write of
that unit that succeeds clears it. A clear of the group drops it, and so does a
configure that no longer names the unit. The WARN carries
`doctor-firewall-domain-group-unsaved-answer`.

This record lives only in the plugin's process, and the plugin can run forked,
so the post-config check cannot see it: that check opens the store from the
process that answers doctor. The plugin declares a `runtime` check,
`firewall-domain-group-unsaved-answer`, in its Stage 1 registration, and
answers it through its doctor callback. Each finding names the group, the name,
the family and the store error, and its fix is
`update firewall domain-group <name>`. Only `show doctor`, and `ze doctor`
when the daemon answers, runs it.

The refused write is also raised on the report bus, so `show warnings` lists
it: a warning under the same code, through the SDK's `RaiseIssue`, because a
forked plugin's own bus is a copy nothing reads. Its subject is the group, the
family and the name, and its message and fix are the finding's. The warning is
set and dropped exactly when the record is: `put`, `open` and `purge` each make
the bus agree with the record after they change it, raising once per record
rather than once per failed write, and clearing with `ClearIssue` when a write
succeeds, the group is cleared, or a configure no longer names the unit. A
raise or clear the daemon did not take is logged under the same code and sent
again at the unit's next write. The code is a registered event code, which the
daemon requires of every raise a plugin sends.
<!-- source: internal/component/firewall/plugins/domain/cache.go -- put, unsavedFor -->
<!-- source: internal/component/firewall/plugins/domain/unsaved_issue.go -- syncIssue, syncAllIssues, raiseUnsaved, unsavedIssueSubject -->
<!-- source: internal/component/firewall/plugins/domain/register.go -- registerDomainDoctor -->
<!-- source: internal/component/firewall/plugins/domain/doctor.go -- unsavedAnswerCheckDecl, unsavedAnswerFindings, answerDoctorCheck -->
<!-- source: internal/component/firewall/plugins/domain/domain.go -- warnUnsavedAnswer -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- Plugin.OnStarted -->

## What the functional tests drive

Three `.ci` files exercise the operator path, and all three drive the daemon over
SSH through `ze cli` rather than over a plugin channel. A `bus { endpoint ... { external { ... } } }`
block would hand the fixture a channel to dispatch on and would change what the
daemon under test is running, and the two wiring rows these files answer ask for
the real path.

| File | Suite | What it proves |
|------|-------|----------------|
| `test/plugin/firewall-domain-group-update.ci` | `./le test functional plugin` | Two updates around a changed DNS answer: the kernel set holds the new address and not the replaced one, and the change log names both |
| `test/plugin/firewall-domain-group-clear.ci` | `./le test functional plugin` | The set leaves the kernel with the cache, read back from nftables after the clear |
| `test/firewall/firewall-cli-domain-group-show.ci` | `./le test functional firewall` | The DNS name is rendered beside the address it supplied, in the same element object |

Each declares its term against the group in its own config rather than committing
one mid-test. A daemon start runs no verify, so the table is simply held back
until the first update resolves the group, which is the cold start these files
walk through.

All three serialize on `option=exclusive:group=dns-stub-port-53`. Their fixture
serves the `le mock dns` stub in process on port 53, which `system name-server`
reaches because that leaf is an address and carries no port.
<!-- source: internal/test/fixture/netfilter_fixture_domain_group.go -- domainGroupUpdate, domainGroupClear, domainGroupShow -->

A fourth file, still a draft, asks a question a daemon the runner starts once
cannot answer: does a second start honor the stored expiry the first one wrote.
`test/draft/plugin/firewall-domain-group-restart-keeps-answers.ci` runs a fixture that
starts the daemon itself, twice, over one config directory, in the same
exclusive group. Between the starts it waits for the table to leave the kernel
and changes both answers. It then checks, from the kernel and from the stub's
query log, that the group whose stored answer had an hour left keeps its stored
address and is not asked for, and that the group whose stored answer expired
is.
<!-- source: internal/test/fixture/netfilter_fixture_domain_group_restart.go -- domainGroupRestartKeepsAnswers -->

## Two stores, because they are two shapes

| What | Where | Why |
|------|-------|-----|
| Last-good addresses and their expiry | Managed store, `meta/firewall/domain-group/{group}/{name}/{family}` | Small, bounded, per-group state that must survive a restart |
| Change log | `<config-dir>/firewall-domain-group.dns.jsonl` | Append-only and unbounded in shape |

The cache sends state RPCs to the daemon's owned store after `OnStarted`.
Each write replaces one complete value. The append-only change log uses a
separate file so an added record does not rewrite all previous records.
<!-- source: internal/component/firewall/plugins/domain/cache.go -- open, put -->
<!-- source: internal/component/firewall/plugins/domain/domain.go -- runFirewallDomain -->
<!-- source: internal/component/firewall/plugins/domain/changelog.go -- append-only change log -->

The log is NOT the operator audit log. That one records operator actions and is
bounded by how often a person acts; a rotating name is neither, and sharing the
file would evict commit history to record DNS churn.

## What earns a write

Every authoritative answer writes its name and family's entry, because every
answer renews `expires-at`, and a restart arms the unit at it. Thomas chose
this over renewing at most hourly (spec-fetched-data-survives-restart, design
gate): an expiry left at an older answer reads as passed, and the start would
ask for a name whose answer is still valid. A name answering the same addresses
every 60 seconds therefore writes up to 1,440 times a day per family, the 60
second `ttl-floor` being the bound. Each such write replaces one value with
another of the same size (`expires-at` is UTC whole seconds, and the other
fields do not move), which `zefs-format.md` writes in place.

What a write carries besides the renewal is bounded per event:

- **The addresses changed.** The entry is replaced, and `resolved-at` moves.
  Once per actual move.
- **The first answer for a name and family**, even when it carries no address.
  "Never asked" and "asked, and the name holds nothing" are different facts, and
  only a written entry tells them apart: an IPv4-only name answers NOERROR-empty
  for AAAA forever, so without this its IPv6 family would read as unqueried for
  the life of the box. Once per entry.
- **The failing state flipped.** `FailingSince` is written when a name stops
  answering and cleared when it recovers. The addresses alone cannot say it: a
  name whose server has been down for a week holds exactly what it held before.
  Once per outage, at each end. A failure is not an answer, so a continuing
  outage writes nothing and the stored expiry stays the last good answer's.

A confirming answer keeps `resolved-at`: it dates the addresses, which did not
move, and `show firewall domain-group` and the data-age metric read it as that.
<!-- source: internal/component/firewall/plugins/domain/domain.go -- resolveAndRecord, recordSteadyAnswer, markFailing -->

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

Verify reads the candidate's groups into a store of its own over the same keys,
and only `configure` opens the store the running config enforces. Opening the
running store at verify replaced its entries with the candidate's and dropped
the unsaved records of every group the candidate removed, clearing their
warnings; a commit that another plugin's verify then aborted left the running
groups with no addresses for the next apply to program.
<!-- source: internal/component/firewall/plugins/domain/domain.go -- verify -->

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
