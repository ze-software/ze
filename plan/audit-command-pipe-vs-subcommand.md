# Audit: pipe operation or subcommand

| Field | Value |
|-------|-------|
| Date | 2026-08-21 |
| Scope | every command Ze registers |
| Question | which leaves name a VIEW of the payload their parent already returns |
| Deliverable | verdicts and evidence. No implementation plan; the mechanism is a separate spec |

## What was asked

`show bgp summary` was removed on 2026-08-21 and replaced by `show bgp` plus
`show bgp | summary`, because `summary` was an aggregate of the same payload
(spec `cli-show-bgp-is-the-command`, closed and removed the same day; its
record is the commits it names and this repository's history). This audit asks
how far that pattern reaches.

## The rule applied

| Verdict | Test |
|---------|------|
| PIPE | the leaf's payload is a subset, aggregate, or reformat of the parent's payload. Same fetch, fewer or rolled-up fields |
| SUBCOMMAND | the leaf emits a field the parent does not produce, OR takes an argument that changes what is fetched, OR has no parent whose payload it could derive from |
| UNCLEAR | the payload relationship could not be determined |

A name that reads like a view (`summary`, `brief`, `detail`, `status`,
`counters`, `stats`) is a candidate, never a verdict. Every verdict below rests
on reading the function that BUILDS the response for both sides and comparing
the field names each emits.

## How the population was derived

`make ze-command-list` is incomplete: `scripts/inventory/commands.go` walks
`AllBuiltinRPCs` plus the streaming prefixes, and reports neither
plugin-registered commands nor `registry.MustRegisterLocal` ones. The population
here is the union of three sources, derived mechanically on 2026-08-21:

| Source | How | Paths |
|--------|-----|-------|
| YANG command tree | every node carrying `ze:command`, with its container chain as the CLI path and `augment` targets expanded | 384 |
| Plugin declarations | every `Name` in an `[]sdk.CommandDecl` literal outside `_test.go`, string constants resolved | 168 |
| Local handlers | every `registry.MustRegisterLocal` and `MustRegisterLocalMeta` path literal | 47 |

Union after dedup: **465 unique command paths**. The sources overlap heavily,
which is why the union is smaller than the sum.

## Counts

| Population | Count |
|------------|-------|
| Unique command paths registered | 465 |
| Action verbs (`request`, `clear`, `set`, `create`, `delete`, `update`, `debug`, `announce`, `withdraw`, `generate`, plus the test-plugin verbs) | 132 |
| Read verbs (`show`, `monitor`, `resolve`, `validate`, `help`, `system`, `plugin`) | 333 |
| Read commands of one or two tokens, so no parent is possible | 54 |
| **Read commands of three or more tokens: the candidate surface** | **279** |

### Verdicts

| Verdict | Count | Notes |
|---------|-------|-------|
| PIPE | 48 | listed in full below |
| SUBCOMMAND | 231 | |
| UNCLEAR | 0 | no pair was left undetermined once its producers were read |

One more PIPE sits outside the candidate surface because it is a SIBLING rather
than a child: `show ecmp-groups` is a view of `show rib`. It is counted
separately and treated in the findings.

The 132 action-verb commands are SUBCOMMAND by the third test: an action returns
an outcome, not a payload a sibling could reshape. They are counted once and not
enumerated.

**Coverage that is auditable**: 223 of the 279 candidates had the producing
function read on at least one side of their pair. The 56 not read break down as:

| Not read | Count | Why the rule decides them anyway |
|----------|-------|----------------------------------|
| `resolve` leaves (9), the `command help` / `command complete` triples (6), `show dns lookup`, `show route lookup`, `show bgp decode`, `show bgp encode` | 19 | each takes a mandatory argument that drives a lookup or a transform of operator-supplied input |
| `plugin session bye/ping/ready`, `monitor` leaves | 6 | protocol RPCs and streams whose branch root is not a command |
| `show test *`, `show fake* help` | 8 | test-plugin commands |
| `show isis hostname/interface/neighbor/spf-log`, `show system file-descriptors/goroutines/kernel-log/profile/sockets`, `show aaa accounting`, `show storage smart`, `show runtime memory`, `system version api/software` | 14 | branch root is not a command |
| **`show ospf` leaves: `border-routers`, `graceful-restart`, `instance`, `ldp-sync`, `te-database`, `virtual-links`, `ipv6 database extended`, `ipv6 graceful-restart`, `ipv6 instance`** | **9** | **this is the real gap. `show ospf` and `show ospf ipv6` ARE registered parents, so a PIPE verdict is still possible for these nine and was not tested** |

## Ranked findings

Ranked by how much an operator would notice.

### 1. `show host` and `show host all` are one command, spelled twice

This is the `show bgp` / `show bgp summary` defect, unfixed, in another family.
`container host` and `container host all` in
`internal/plugins/host-cmd/yang/ze-host-cmd.yang` BOTH carry
`ze:command "ze-show:host-all"`. One wire method, one handler
(`dispatchHostSection` in `internal/plugins/host-cmd/cmd/show_host.go`), two CLI
paths. The YANG's own description says so: "The bare 'show host' is an alias of
'show host all'."

It is worse than the BGP case, because `show host all` is also a SUPERSET of the
eight section leaves. `Detect` in `internal/component/host/inventory.go` emits
`host` (hostname, uptime-seconds, timezone) and `errors`, and `sectionDetectors`
has no `host` entry, so those fields are reachable through `all` and through no
section command.

The two spellings do not even share a cache: `host.Detect()` goes through
`globalCached` (`internal/component/host/cached.go`) while every single-section
call goes straight to `defaultDetector`.

### 2. `show bgp peer <selector> rib` is `show bgp rib | peer <selector>`

Both wire methods resolve to ONE function. In
`internal/component/bgp/plugins/cmd/rib/rib.go`, `ze-rib-api:routes` and
`ze-bgp:peer-rib` are both registered with `Handler: forwardRibRoutes` and
`PluginCommand: cmdRibShow`, and `forwardRibRoutes` forwards to `show bgp rib`
with `ctx.PeerSelector()`. The only difference is `RequiresSelector: true`.

`registerPipeFilters` in the same file registers `peer` as a `TakesArg` pipe
filter on `show bgp rib`. So the subcommand and the pipe filter are two
spellings of one fetch through one function.

### 3. `show bgp peer list` duplicates the `| peers` alias that just landed

`registerAliases` in `internal/component/bgp/plugins/cmd/peer/peer.go` registers
exactly two aliases, both on `show bgp`: `summary` and `peers`. `show bgp |
peers` returns the peer rows. `handleBgpPeerList` returns the same peer set from
the same `filterPeersByArgs` walk with five fields where `handleBgpSummary`
gives sixteen, and renames `description` to `group` on the way.

The spec that added `| peers` did not remove the subcommand that already did it.

### 4. `show ospf ipv6 database detail` IS `show ospf ipv6 database`

One `case` arm in `internal/plugins/ospf/register.go` serves both:

```
		case "show ospf ipv6 database", "show ospf ipv6 database detail":
			return statusDone, v6DatabaseDetail(v6set, "", ""), nil
```

Two registered commands, two wire methods (`ze-show:ospfv3-database`,
`ze-show:ospfv3-database-detail`), two YANG containers, one byte-identical
payload. The non-detail name is already the fully decoded view.

`show ospf ipv6 database segment-routing` and `show ospf ipv6 segment-routing`
are the same pair problem: both resolve to `v6eng.srSnapshot(interfaceFamilyIPv6)`
with the same fallback.

### 5. Three commands list the daemon's commands, and one is the union

| Command | Handler | Population walked | Row fields |
|---------|---------|-------------------|-----------|
| `system command list` | `handleSystemCommandList` to `commandRows` (`internal/component/plugin/server/system.go`) | builtins AND plugin commands | `Value`, `Help`, `Source` when verbose, `Hidden` on plugin rows |
| `show command list` | `handleBgpCommandList` (`internal/plugins/meta/cmd/help.go`) | builtins only | `Value`, `Help`, `Source` when verbose |
| `plugin command list` | `handlePluginCommandList` (`internal/component/plugin/server/plugin_rpc.go`) | plugin commands only | `name`, `description` |

Both narrow commands are strict subsets of the wide one, split by source, and
`system command list` already carries the `Source` field that names the split.

The same triple exists for `help` and `complete`. All six take a name or prefix
argument, so those stay SUBCOMMAND by the lookup test. The duplication is real
and is a different defect.

### 6. `show system subsystem list` and `system subsystem list` are byte-identical

`handleShowSystemSubsystemList` (`internal/component/cmd/show/system.go`) and
`handleSystemSubsystemList` (`internal/component/plugin/server/system.go`) build
the same map from the same two calls and emit `subsystems[]` of `name`, `stage`,
`running`, `command-count`, plus `count`. Two commands, one payload, in two
packages, with the same explanatory comment copied into both.

Neither is a view of the other. Pure duplication.

### 7. `show interface brief`, `errors`, `type` and `scan` are views of `show interface`

All five call `iface.ListInterfaces` through `showInterfaceAll`,
`showInterfaceBrief`, `showInterfaceErrors`, `showInterfaceByType` and
`DiscoverInterfaces` in `internal/component/iface/cmd/` and
`internal/component/iface/discover.go`. `brief` keeps four columns and flattens
`addresses[0]`; `errors` keeps four counters out of `stats` and drops all-zero
rows; `type` filters on `InterfaceInfo.Type`; `scan` keeps three fields and
re-domains `type`.

`show interface rate` is the exception and is a genuine subcommand:
`iface.ListRates` reads `globalTracker`, a two-sample delta the parent's payload
cannot carry.

### 8. `show bgp health` is a rename-and-rollup of `show bgp`

`handleShowBGPHealth` (`internal/component/bgp/plugins/cmd/peer/health.go`)
walks the same `ctx.Reactor().Peers()` and emits `peers[]` of `peer`, `state`,
`as`, `uptime` plus `count` and `not-established`. `peer` is `address` renamed,
`as` is `remote-as` renamed, `count` is `peers-configured`, and
`not-established` is `peers-configured` minus `peers-established`.

`show bgp healthcheck` is one character away in the CLI and completely
unrelated: `probeManager.handleShow` reports healthcheck probe FSM state. Two of
the three near-identical names are the same payload, and the odd one out is the
one whose spelling sits between them.

### 9. Six `show ospf database <type>` leaves are one row filter

`databaseSnapshotByType` calls `filterLSAsByType` over what `databaseSnapshot`
returns unfiltered (`internal/plugins/ospf/show_database.go`,
`internal/plugins/ospf/instance_snapshots.go`). `router`, `network`, `summary`,
`asbr-summary`, `external` and `nssa-external` each keep the rows whose `type`
field matches. The parent already emits `type`.

`show ospf ipv6 database scope link|area|as` is the same shape over the `scope`
field, through `v3ScopeSelector`.

### 10. Whole families where the singular leaf filters the plural

Each of these calls the plural's producer and then drops rows:

| Leaf | Producer | What it does |
|------|----------|--------------|
| `show vrrp interface name <x>` | `(*engine).snapshotsForInterface` (`internal/plugins/vrrp/engine.go`) | calls `e.snapshots()`, keeps rows whose `Interface` matches |
| `show bfd session address <x>` | `Loop.SessionDetail` (`internal/component/bfd/engine/snapshot.go`) | same `sessionEntry.snapshot()`, early-returns on a peer match |
| `show bfd profile name <x>` | `handleShowProfile` (`internal/component/bfd/cmd/bfd.go`) | the SAME function and the SAME wire method `ze-bfd-api:show-profile` as `show bfd profile` |
| `show vpn ipsec peer name <x>` | `handleShowVPNIPsecPeer` (`internal/component/ike/cmd/show_ipsec.go`) | same `engine.ActiveTable().All()` fetch, filters on `sa.PeerName` |
| `show policy chain peer <sel>` | `handleShowPolicyChain` (`internal/component/bgp/plugins/cmd/policy/handler.go`) | `ctx.Reactor().Peers()` is unconditional; `filterPeersByPolicySelector` accepts `*` |
| `show dns cache record <name>` | `getDNSCacheEntries(name)` (`internal/component/resolve/cmd/show_dns.go`) | the same function `show dns cache list` calls with an empty filter |
| `show metrics name <x>` | `handleShowMetricsQuery` (`internal/component/cmd/show/show.go`) | renders the whole `/metrics` exposition, then greps lines |
| `show rsvp-te tunnel` | `showTunnels` (`internal/plugins/rsvpte/show_data.go`) | same `lspTable.All()`, keeps `Role == RoleIngress`, and `role` is in the parent payload |

### 11. `show ddos status` counts what `show ddos incidents` lists

`handleShowDdos` and `handleShowDdosIncidents` (`internal/plugins/ddos/observe/show.go`)
both load `activeStore`. `active-attacks` is `store.activeCount()` and
`incidents` is `store.count()`, both counts over the ring `store.list()`
returns. The key `incidents` is an integer in one command and an array in the
other.

### 12. `show ecmp-groups` is a view of `show rib`, as a sibling not a child

`(*sysRIB).showECMPGroups` and `(*sysRIB).showRIB`
(`internal/component/sysrib/sysrib.go`) both read `s.lastECMP`, and `showRIB`
already inlines it as `ecmp-paths`. The differences are the key name and
dropping empty rows.

This one has a caveat that matters: `showECMPGroups` iterates `s.lastECMP` keys,
and a `lastECMP` key with no matching `s.best` entry produces no row in
`showRIB`. That stale state is the one thing stopping the two from being exactly
equivalent today.

## Every PIPE verdict, and what it would need

Groups A to D hold the 48 PIPE leaves of the candidate surface, plus
`show ecmp-groups`, the one sibling case that sits outside it. The two pure
duplicate PAIRS are listed after group D and are NOT counted as PIPE. Grouped by
the operator that would replace each leaf.

### A. A field subset or rollup that `| display` and `| count` already express

| Command | Parent | Evidence |
|---------|--------|----------|
| `show host cpu` | `show host` | `dispatchHostSection("cpu")` returns `Inventory.cpu` unwrapped |
| `show host dmi` | `show host` | `Inventory.dmi` |
| `show host kernel` | `show host` | `Inventory.kernel` |
| `show host memory` | `show host` | `Inventory.memory` |
| `show host nic` | `show host` | `Inventory.nics` |
| `show host platform` | `show host` | `Inventory.platform` |
| `show host storage` | `show host` | `Inventory.storage` |
| `show host thermal` | `show host` | `Inventory.thermal` |
| `show interface brief` | `show interface` | `showInterfaceBrief`: 4 of 18 fields, `address` from `addresses[0]` |
| `show bgp peer list` | `show bgp` | `handleBgpPeerList`: 5 fields, all in `handleBgpSummary`'s peer rows |
| `show bgp health` | `show bgp` | `handleShowBGPHealth`: 4 fields renamed, `count` and `not-established` derived |
| `show bgp rpki summary` | `show bgp rpki status` | `summaryCommand` versus `statusCommand`, see corrections below |
| `show bgp adj-rib-in status` | `show bgp adj-rib-in` | `(*AdjRIBInManager).status` counts what `show` lists, from the same `r.ribIn` |
| `show l2tp statistics` | `show l2tp` | `handleStatistics` and `handleSummary` both from one `svc.Snapshot()`; two fields renamed |
| `show metrics list` | `show metrics values` | `extractMetricNames` parses the output of the same `captureMetricsText` |
| `show event namespaces` | `show event recent` | `(*EventRing).NamespaceCounts` walks the same `r.records` ring |
| `show yang completion` | `show yang tree` | `collectCollisions` over the same `buildUnifiedTree()` |
| `show ospf ipv6 database router-information` | `show ospf database router-information` | `riDatabaseSnapshot(nil, v6ri)` is the `af == "v3"` subset of `riDatabaseSnapshot(eng, v6ri)` |
| `show ddos status` | `show ddos incidents` | `handleShowDdos` counts the ring `handleShowDdosIncidents` lists |
| `show ecmp-groups` | `show rib` (sibling) | both read `s.lastECMP`; `showRIB` inlines it as `ecmp-paths` |

### B. A row filter by field value, which NO generic operator provides

`knownPipeOps` in `internal/component/command/pipe.go` is `match`, `count`,
`no-more`, `table`, `text`, `yaml`, `raw`, `json`, `resolve`, `origin`,
`ndjson`, `log`, `first`, `last`, `display`, `fill`. `applyMatch` is a
case-insensitive SUBSTRING filter over the rendered text, not a field-aware
predicate. Field-aware row filtering exists only as per-command `PipeFilter`s,
and those are registered on `show bgp rib` and `show bgp rib best` and nowhere
else.

| Command | Parent |
|---------|--------|
| `show interface errors` | `show interface` |
| `show interface type <t>` | `show interface` |
| `show interface scan` | `show interface` |
| `show ospf database router` | `show ospf database` |
| `show ospf database network` | `show ospf database` |
| `show ospf database summary` | `show ospf database` |
| `show ospf database asbr-summary` | `show ospf database` |
| `show ospf database external` | `show ospf database` |
| `show ospf database nssa-external` | `show ospf database` |
| `show ospf ipv6 database scope link` | `show ospf ipv6 database` |
| `show ospf ipv6 database scope area` | `show ospf ipv6 database` |
| `show ospf ipv6 database scope as` | `show ospf ipv6 database` |
| `show ospf ipv6 database router detail` | `show ospf ipv6 database` |
| `show bgp peer <sel> rib` | `show bgp rib` |
| `show vrrp interface name <x>` | `show vrrp` |
| `show bfd session address <x>` | `show bfd sessions` |
| `show bfd profile name <x>` | `show bfd profile` |
| `show vpn ipsec peer name <x>` | `show vpn ipsec sa` |
| `show policy chain peer <sel>` | itself with `peer *` |
| `show dns cache record <name>` | `show dns cache list` |
| `show metrics name <x>` | `show metrics values` |
| `show rsvp-te tunnel` | `show rsvp-te lsp` |

### C. Arithmetic over emitted fields, which no operator provides

| Command | Parent | What it computes |
|---------|--------|------------------|
| `show bgp peer <sel> statistics` | `show bgp` | 10 fields all in the parent's peer rows, plus four `rate-*` = counter divided by uptime seconds |
| `show rsvp-te tunnel` | `show rsvp-te lsp` | also `ero-hops` = `len(ero)`, and `ero` is in the parent |

### D. Degenerate: the two spellings are the same payload, so one is deleted

| Command | Its twin | Evidence |
|---------|----------|----------|
| `show host all` | `show host` | one `ze:command "ze-show:host-all"` on both containers |
| `show ospf ipv6 database detail` | `show ospf ipv6 database` | one `case` arm |
| `show ospf ipv6 database segment-routing` | `show ospf ipv6 segment-routing` | both `v6eng.srSnapshot(interfaceFamilyIPv6)` |
| `show env registered` | `show env list` and `show env get` | `cmdRegistered` dispatches onto `showAll` and `showOne`; it has no producer of its own |
| `show command list` | `system command list` | subset by source |
| `plugin command list` | `system command list` | subset by source |

### Not PIPE, but the same clean-up: two pure duplicate PAIRS

Neither member is the other's parent, so the pipe rule does not reach them.
They are two commands emitting one payload, which is what the `show bgp summary`
removal was about.

| Pair | Evidence |
|------|----------|
| `show system subsystem list` and `system subsystem list` | `handleShowSystemSubsystemList` (`internal/component/cmd/show/system.go`) and `handleSystemSubsystemList` (`internal/component/plugin/server/system.go`) build the same map from the same two calls, with the same explanatory comment copied into both |
| `show system platform` and `show host platform` | `handleShowSystemPlatform` marshals `PlatformInfo` from `host.DetectPlatform()`; `dispatchHostSection("platform")` marshals the same struct from `Detector.DetectPlatform()` |

## Corrections to the RPKI reasoning I was asked to verify

The verdicts hold. Two evidence statements need correcting.

| Claim as given | What the source says |
|----------------|----------------------|
| "`statusCommand` emits all the same data (with vrp-count split by family)" | `statusCommand` does NOT emit a `sessions-established` field. `summaryCommand` counts `snap.State == sessionEstablish` (`sessionEstablish = "establish"`, `rtr_session.go`). `statusCommand` emits `cache-servers[].state`, so `sessions-established` is a COUNT over a field status does emit. PIPE is right; the derivation is a rollup, not a projection |
| "`show bgp rpki cache` emits per-session `version`, `session-id`, `serial`, refresh/retry/expire" | `version` is already in `statusCommand`'s `cache-servers[]`. What `cacheCommand` adds is `preference`, `session-id`, `serial`, `refresh-interval`, `retry-interval`, `expire-interval`. SUBCOMMAND still holds, on six new fields |

`show bgp rpki roa` and `aspa` are SUBCOMMAND for two reasons, not one: each
takes an argument that drives a lookup, AND each with no argument emits an
`entries[]` list (`prefix`, `max-length`, `asn`; `customer-asn`, providers) that
`statusCommand` never produces.

`show bgp rpki status` should become the bare parent `show bgp rpki`. Confirmed:
`rpki.go` declares six leaves and no parent, and `show bgp rpki` appears in
`cmdBgpChildren` (`internal/component/bgp/plugins/cmd/peer/peer.go`) purely to
block alias inheritance. The alias registry knows the branch root; the
dispatcher does not.

## Commands with no bare parent

A pipe cannot attach to a parent that does not exist. 121 prefixes are shared by
at least one leaf and are not themselves commands. These are the ones an
operator would type.

| Branch root | Leaves | Which leaf should become it | Why |
|-------------|--------|------------------------------|-----|
| `show system` | 15 | none | five modules declare `container system` under `show` and not one attaches a `ze:command`. Nothing local registers it either |
| `show config` | 7 | `show config dump` | the gRPC `GetRunningConfig` and the REST config handler already execute the literal string `show config dump` to mean "the config" |
| `show bgp peer` | 6 | none | `list` is `show bgp \| peers`; the branch is a namespace for selector-scoped views |
| `show bgp rpki` | 5 | `status` | |
| `show vpn ipsec` | 6 | `sa` | the widest payload, and the YANG description already says so |
| `show vpn ipsec dataplane` | 3 | `dataplane sa` | `policy` reads a different kernel table, `drift` is a join |
| `show isis` | 8 | none | no leaf covers the others |
| `show rsvp-te` | 5 | `lsp` | `tunnel` is a strict view of it |
| `show schema` | 5 | `list` | all five project one `buildSchemaRegistry()` build |
| `show bmp` | 4 | none | four disjoint fetches; the parent has to be composed |
| `show metrics` | 4 | `values` | then `list` and `name <x>` both become pipes |
| `show event` | 4 | `recent` | then `namespaces` becomes a pipe |
| `show dns cache` | 3 | `list` | then `record <name>` becomes a pipe |
| `show ddos` | 4 | `incidents` | `status` becomes `\| count` plus a match. `local` and `flowspec` are separate responder plugins and stay |
| `show vpp trace` | 3 | `show` | it is the only reader. `start` and `clear` mutate and belong under `request` and `clear` |
| `show policy` | 4 | `list`, weakly | see the inconsistency section: this is not one family |
| `show firewall` | 4 | none | |
| `show vpp` | 4 | none | `runtime` and `trace` are different VPP CLI calls |
| `show traffic` | 4 | none | four unrelated sources. `show traffic control` already holds the bare wire method `ze-show:traffic`, and the 2026-07-02 YANG revision records the rename from the bare command |
| `show data` | 3 | `ls` | |
| `show env` | 3 | `list` | `registered` with no argument is already byte-identical to it |
| `show yang` | 3 | `tree` | then `completion` becomes a pipe |
| `show fib` | 3 | none | three backends, three fetches |
| `show anomaly` | 3 | none | three plugins, three stores. `show anomaly detect` already holds the bare wire method `ze-show:anomaly` |
| `show l2tp session`, `show l2tp tunnel` | 3, 2 | neither | `sessions` and `tunnels` already answer at the parent level |
| `show bgp rs`, `show rr` | 2 each | `peers` | `status` in both returns the constant `{"running": true}` |
| `show bfd`, `show bfd session` | 4, 1 | `sessions`; delete `session` | |
| `show pki`, `show pki certificate` | 2, 1 | `certificates`; delete `certificate` | |
| `show ldp` | 2 | `neighbor`, weakly | `neighbor` and `binding` share no field |
| `show mpls` | 1 | fold the container away | one child, nothing to disambiguate |
| `show vrrp interface` | 1 | delete the branch | `interface name <x>` is a filter of `show vrrp` |
| `show interface name` | 2 | not a command | only `name <x> detail` and `name <x> counters` |
| `show ospf ipv6 database router`, `show ospf ipv6 database scope` | 1, 3 | not commands | pure grammar nodes |
| `show system subsystem` | 1 | not a command | while `show system update` and `show system ntp` ARE |
| `show log`, `show flow`, `show aaa`, `show route`, `show runtime`, `show storage`, `show capture` | 1 to 2 each | mixed | `show capture` IS a command; the rest are not |

The inverse count is the useful one: **35 read-verb command paths are both a
registered command AND the parent of at least one registered leaf.** Twelve of
those 35 are inside `show ospf`. The top-level families with a real answer at
their root are `show bgp`, `show host`, `show interface`, `show l2tp`,
`show ospf`, `show pppoe`, `show sysctl`, `show vrrp`, `show capture`,
`show debug` and `show route`. Every other prefix an operator would type is a
namespace with nothing at its root.

## Families whose naming is inconsistent with itself

### `peer` is a subcommand here and a pipe filter there

`show bgp rib | peer <x>` is a registered `PipeFilter`. `show bgp peer <x> rib`
is a subcommand path to the same handler. Same word, same job, two grammars, one
family.

### `summary` is a pipe alias here and a subcommand there

`registerAliases` registers `summary` as a pipe alias on `show bgp`.
`show ospf database summary` is a registered command that filters LSDB rows to
Type 3. Same word, two grammars, in the same CLI.

### `reason` is a pipe filter in BGP and a `detail` subcommand in OSPF

`show bgp rib best | reason` is a `PipeFilter` that explains best-path
selection. The OSPF analogue, per-prefix path preference, is
`show ospf spf detail`, and its payload literally carries a `reason` field.
Identical concept, opposite side of the pipe.

### `count`, `histogram` and `graph` roll up by pipe; `status` rolls up by subcommand

On `show bgp rib`, "roll this payload up" is spelled `| count`, `| histogram`,
`| graph`. On the same command it is ALSO spelled `show bgp rib status`. On
`show bgp rib best` the split is sharper: `reason` is a pipe filter and `status`
is a subcommand, and both are extra views of one best-path computation.

### `show host platform` and `show system platform` are one payload

`handleShowSystemPlatform` (`internal/component/cmd/show/system.go`) calls
`host.DetectPlatform()`; `dispatchHostSection("platform")` calls
`Detector.DetectPlatform()`. Same struct, same fields, two commands.
`show system cpu` is a strict SUPERSET of `show host cpu`: it nests the whole
`CPUInfo` under a `hardware` key and adds four Go-runtime fields, so
`show host cpu` is reachable as `show system cpu | display hardware`.

The wire methods are crossed in a way nobody would guess:
`ze-show:system-memory` is bound to `show runtime memory`, and
`ze-show:system-memory-map` is bound to `show system memory`.

### Enumerate has four spellings

`show sysctl keys` (bare plural), `show metrics list`, `show event list`,
`show dns cache list`, `show schema list`, `show env list`, `show data ls`,
`show config ls`, `show yang tree`. `ls` and `list` coexist in adjacent verbs
with no distinguishing principle.

### Counters have three spellings

`show vrrp statistics`, `show dns cache stats`, `show interface name <x>
counters`, `clear interface counters`, `show metrics pool`. The clear verbs
inherit the split: `clear vrrp statistics` beside `clear dns cache stats`.
`statistics` itself means two different things: in L2TP two gauges, in PPPoE a
per-interface table.

### "One of them" has six conventions

Plural node plus singular node with a bare argument (`show sysctl keys` /
`show sysctl key <k>`). Plural node plus a differently named singular node
(`show dns cache list` / `show dns cache record <name>`,
`show metrics list` / `show metrics name <x>`). A typed selector keyword
(`show vrrp interface name <x>`). A verb (`show env list` / `show env get
<key>`). An optional positional on the plural itself (`show schema methods
[module]`, `show data registered [key]`). A flag inverting the default
(`show yang doc --list` versus `show yang doc <cmd>`).

The same spelling pattern means "same data, filtered" in BFD
(`sessions` / `session address <x>` share one snapshot function) and "different
data" in PKI (`certificates` / `certificate name <x>` differ by eight fields and
a live chain verify).

### The two IRR families disagree with each other

`show bgp irr prefix <peer>` takes a peer address; `show firewall irr prefix
<asn-or-as-set>` takes a resource name. The BGP leaf returns one flat
`prefixes[]`; the firewall leaf returns `ipv4[]` and `ipv6[]`. Status
vocabularies disagree (`ok`/`error`/`pending` against `ok`/`stale`/`missing`).
Only BGP has `check`; only firewall has `clear`.

### Same key, different type, in one family

`incidents` is an integer in `show ddos status` and an array in `show ddos
incidents`. `count` is the number of registered loggers in `show log levels` and
the number of returned rows in `show log recent`. `top-source-ips` holds
`{address, bps}` in `show traffic stat` and seven feature fields in `show
traffic feature`. `next-hop` is unconditional in `show fib kernel` and `show fib
p4` and `omitempty` in `show fib vpp`.

### A word cannot collide with a pipe operator, and one collision IS enforced

`ParsePipe` (`internal/component/command/pipe.go`) cuts the input at the first
`|`, so command words are only ever read to the left and operator names to the
right. `show log ...` does not collide with `| log`, and `show capture raw` does
not collide with `| raw`. The collision ze does enforce is between an ALIAS name
and a PIPE FILTER name on overlapping command paths, and it is a
`panic("BUG:")` at init from whichever registers second (`RegisterAliases`,
`RegisterPipeFilters`). So a future `show log | levels` alias is safe and a
filter named `log` is not.

`show vpp trace show` says `show` twice.

### `show policy` is four unrelated things

`list` is a plugin registry of filter types; `routes` is Linux policy-based
routing from a separate plugin process that container-merges onto the same
namespace; `chain` and `test` are BGP per-peer. The `ze-policyroute-cmd.yang`
revision note shows the collision was deliberate.

## The rule this audit collides with

`ai/rules/cli.md` mandates the typed-selector grammar:

> If a command targets one member of a set, the selector itself MUST also be
> typed by a keyword such as `name`, `id`, `index`, `address`, or `type`.

and it lists `show interface type dummy`, `show sysctl key <k>`,
`show interface name eth0 detail` as the CORRECT forms.

Every leaf in group B above is exactly that construction. So `show interface type
dummy` is simultaneously mandated by the grammar rule and a PIPE by the payload
rule. The two rules disagree, and the disagreement is not incidental: it covers
22 of the 47 PIPE verdicts.

The rules agree cleanly on VIEW words (`summary`, `brief`, `all`, `status`,
`stats`, `detail`, `list`), where nothing in `cli.md` requires a subcommand.
They collide on SELECTOR words (`name`, `id`, `address`, `type`, `key`, `peer`),
where `cli.md` requires the keyword and the payload rule says the row filter
belongs after the pipe. Someone has to decide which rule wins before group B can
move, and that decision is not this audit's to make.

**RULED 2026-08-21, by the owner: the pipe wins. All 48 convert, selector words
included.** So `show sysctl key <k>` becomes `show sysctl | key <k>` and
`show bgp peer <sel> rib` becomes `show bgp rib | peer <sel>`. The owner took
this answer knowing its cost, which is stated in the next paragraph and is the
thing that sequences the work: `| match` is a substring filter over rendered
text, so no field-aware row filter exists today. The 22 group B leaves are
therefore BLOCKED until one does, while the 26 leaves of groups A and D need
only `| display`, `| count`, or a deletion and can move first.

`ai/rules/cli.md` states the typed-selector grammar and now disagrees with this
ruling for selector words. The rule text MUST be corrected in the change that
implements the first group B conversion, not left to be discovered by the next
reader: a rule that contradicts a landed decision is worse than no rule.

Note also that `ai/rules/cli.md` already says "Every command that produces
output MUST support all pipe operators", which is the direction of travel.

## What surprised me

### The pipe surface built for `show bgp` is registered almost nowhere

```
$ grep -rn "RegisterColumns(" --include="*.go" internal/ | grep -v _test
internal/component/command/column_order.go:41:func RegisterColumns(commands []string, orders ...ColumnOrder) {
internal/component/bgp/plugins/cmd/peer/peer.go:125:	command.RegisterColumns([]string{cmdBgp},
internal/component/bgp/plugins/cmd/peer/peer.go:143:	command.RegisterColumns([]string{cmdBgpPeerList},
internal/component/bgp/plugins/cmd/peer/peer.go:154:		command.RegisterColumns([]string{child})
```

The same greps for the other two registries: `RegisterAliases` has one call site
outside its own definition plus the empty loop beside it, both in
`internal/component/bgp/plugins/cmd/peer/peer.go`. `RegisterPipeFilters` has
three call sites, all in `internal/component/bgp/plugins/cmd/rib/rib.go`.

So of 465 commands: TWO carry a real column order (`show bgp`, `show bgp peer
list`), ONE carries pipe aliases (`show bgp`), and TWO carry real pipe filters
(`show bgp rib`, `show bgp rib best`). The remaining registrations are
deliberate empties that block inheritance: ten `show bgp` children in
`cmdBgpChildren`, three scalar rib commands.

The mechanism is general and is used by one family. Every PIPE verdict in this
audit names a command that could use it and does not.

### `detail` REMOVES fields in OSPF

`show ospf neighbor detail` and `show ospf interface detail` both drop `bfd`,
and interface `detail` also drops `poll-interval` and `nbma-neighbors`. So
`detail` is not a superset of its parent anywhere in OSPF except the three
opaque views. `show ospf database opaque-area` appends the TE and RFC 7684
Extended Prefix/Link decode while `... detail` appends the generic decoded-body
block instead and emits neither, so which of the two is "more detailed" depends
on the opaque type you are looking at.

### `show ospf ipv6 interface detail` is not a detail of `show ospf ipv6 interface`

The parent is `ipsecInterfaceSnapshot` (interface, ipsec, protocol, spi,
installed); the child is `interfaceDetailRows(true)`, the ISM and timer view.
They share no field name and no data source. A reader chaining down the tree
gets a different subject.

### Four commands mutate behind a `show` verb

`HandleCaptureRaw` (`internal/plugins/diag/cmd/capture_raw.go`) accepts
`start|stop|dump`. `handleVPPTraceStart` and `handleVPPTraceClear`
(`internal/plugins/iface/vpp/cmd_show.go`) start a capture and discard a buffer.
`IsReadOnlyPath` (`internal/component/plugin/server/command.go`) classifies
every `show ...` path as read-only, so all four are authorized as reads.

`ai/rules/cli.md` already states the test: "Does running this command change
what the router does, emits, or forwards?" These four answer yes.

### None of the three `show fib` commands reads a dataplane

All three return a plugin-local shadow map fed from the sysrib `best-change`
event stream. `show fib p4`'s backend is `noopBackend` unconditionally
(`internal/plugins/fib/p4/backend.go`), so `show fib p4` reports routes that
were never programmed anywhere. Reported by the FIB family read; I did not
re-verify the `noopBackend` claim.

### `monitor traffic stat`'s RPC handler emits a constant

`handleMonitorTraffic` (`internal/component/trafficstat/cmd/traffic.go`) returns
`{status, filter}` and never touches the trafficstat service. The real payload
comes from a separate streaming registration whose map is identical in shape to
`show traffic stat`, once per second. Ze already has an operator naming that
relationship: `| log`.

### `show vpn ipsec status` survives as a subcommand on one honest technicality

Three of its four values are countable from the `sa` payload. Only
`configured-peers` and `engine-running` are not, and they come from
`engine.ActivePeers()` and `engine.ActiveTable() != nil`. That technicality is
the important one: `sa` returns `{"peers":[]}` both when the engine is down and
when it is up with no SAs.

### `show event list` is BGP-only

`bgpEventTypes` calls `events.ValidEventNames(bgpevents.Namespace)` and nothing
else, while the YANG description promises "every event type you can subscribe
to". Its sibling `show event namespaces` enumerates every namespace in the ring.
Two commands under one parent disagree about what "event" means.

### `show config graph` is missing from the declared command tree

Unlike its six siblings it has no `ze:command` node in
`internal/plugins/config-cli/yang/ze-config-cli-cmd.yang`, so it is reachable
only through the local registry and is absent from CLI completion and the MCP
tool list.

### `show l2tp pool` and `show l2tp shaper` are in no YANG tree

Both are declared only as `sdk.CommandDecl` in their plugins' `register.go`, so
they carry no YANG description and no tree-derived authz path. Reported by the
L2TP family read; I did not re-verify the authorization consequence.

### Incidental defects found on the way

Each is a candidate for a journal row. Each is named with the function that
produces it. The last three were reported by family reads and I did not verify
them a second time.

| Where | What |
|-------|------|
| `registerInjectCommands`, `internal/component/bgp/plugins/rib/rib_inject.go` | the positional-selector test checks three keyword maps and `peer` is in none of them, so `show bgp rib protocol bmp peer 10.0.0.1` binds the literal `peer` as the selector and then fails on the address |
| `ribEventList`, `internal/component/bgp/plugins/rib/rib_commands.go` | hardcodes `cache`, `route`, `peer`, `memory`; the namespace `runRIBPlugin` registers is `cache`, `route`, `best-change`, `replay-request` |
| `showInterfaceCounters`, `internal/component/iface/cmd/show_interface.go` | emits `stats` as an object, or as the STRING "no counters available" when `info.Stats` is nil |
| `enrichSubscriberBrief`, `internal/plugins/cos/enricher.go` | calls `extractSessionKey`, which needs `tunnel-id` and `session-id`; `sessionBrief` never writes those keys, so `cos-profile` can never appear in a summary row |
| `formatTreeJSON`, `internal/component/config/yang/cli/format.go` | the text form renders `[mandatory]`, `[default: X]` and range constraints; the JSON form omits all three, so the machine-readable output carries less than the human one |
| `Serialize`, `internal/component/config/serialize.go` | `show config fmt` is the only config inspection leaf with no secret-masking pass. `dump`, `diff` and `config show` each mask deliberately |
| `handleShowTraffic`, `internal/component/trafficstat/cmd/traffic.go` | `show traffic stat name eth0` silently filters on the literal string `"name"`. `matchCommandTokens` returns the unmatched token tail as args and `validateCommandArgs` never rewrites it, so the handler receives `["name","eth0"]` and answers with an empty `interfaces` array and no error. Same for `show traffic feature` |
| `(*fibVPP).showInstalled`, `internal/plugins/fib/vpp/fibvpp.go` | returns the Go string `"[]"` when the plugin has no fib, which marshals as a JSON string rather than an empty array. `showNHTable` in sysrib carries a comment warning about exactly that double encoding |

## Method and its limits

Eight family reads ran in parallel, each required to read the producing function
for both sides of every pair and to enumerate the emitted field names. A pair
whose producer was not read was to be marked UNCLEAR rather than guessed; none
was.

Four limits.

- The 231 SUBCOMMAND verdicts are not equally examined. Every PIPE verdict and
  every close call had both producers read. 56 leaves were decided by the
  parent-absent or lookup clause without a field-by-field read, and the table in
  the Counts section names all 56 and why.
- **The one real gap is nine `show ospf` leaves.** `show ospf` and `show ospf
  ipv6` are registered parents, so `border-routers`, `graceful-restart`,
  `instance`, `ldp-sync`, `te-database`, `virtual-links`, `ipv6 database
  extended`, `ipv6 graceful-restart` and `ipv6 instance` could each still be a
  PIPE. None was tested. The PIPE count is therefore a floor, not a total.
- `show ospf` carries 48 of the 279 candidates on its own, so the OSPF results
  weight the totals.
- The counts come from a snapshot on 2026-08-21. The extraction is
  re-derivable: a walk of `ze:command` nodes, `[]sdk.CommandDecl` literals and
  `MustRegisterLocal` path literals, deduplicated.
