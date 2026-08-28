# Audit: which commands need presentation pipes

| Field | Value |
|-------|-------|
| Date | 2026-08-22 |
| Scope | every command Ze registers, read as a matrix of scope against presentation |
| Question | which commands need `\|` operations of their own FOR PRESENTATION, and what structure generalizes across them |
| Deliverable | the structure, the views that are missing, and what each one needs the command to emit. No implementation plan |
| Related | `plan/audit-command-pipe-vs-subcommand.md` (which existing subcommands ARE pipes), `plan/spec-plugin-registers-pipe-operations.md` (the mechanism, closed) |

## What this audit adds

`plan/audit-command-pipe-vs-subcommand.md` read all 465 registered commands and
asked, of each leaf that exists, whether it is really a view of its parent. It
answered with 48 PIPE verdicts and 231 SUBCOMMAND verdicts.

This audit asks the opposite question. Which views SHOULD exist, including the
ones that exist in no spelling at all today. The two are complementary and the
verdict tables are not repeated here. Where a command appears in both, this one
cites the other rather than re-deriving it.

The worked example the owner set is BGP: an operator needs all the information,
a summary, and a per-peer view, and the per-peer view must itself accept
`| summary`. That last requirement is the whole design. It says scope and
presentation are separate axes and must compose, which makes the surface a
MATRIX and not a list of leaves.

---

## 1. The general structure

### 1.1 Four axes, not one

Every question an operator asks a `show` command is four questions at once.

| Axis | The question | Example |
|------|--------------|---------|
| SUBJECT | which data | BGP peers, interfaces, the RIB |
| SCOPE | how many rows of it | all of them, one peer, one family |
| PRESENTATION | how much of each row, and which rollups | everything, the short form, the totals, a count |
| FORMAT | how to render it | table, text, json, yaml, ndjson |

The axes are independent. Any scope combines with any presentation, and both
combine with any format. That is why the surface is a matrix. A command tree
that spells each cell as its own leaf multiplies out, which is what produced
`show bgp peer <sel> statistics` beside `show bgp peer list` beside
`show bgp health`, three leaves that are three cells of one matrix over one
fetch.

### 1.2 Which axis is a command path and which is a pipe

| Axis | Where it belongs | Why |
|------|------------------|-----|
| SUBJECT | the command path | it decides what is FETCHED. Nothing else can |
| SCOPE | a pipe | it selects rows of one fetch |
| PRESENTATION | a pipe | it selects fields and rollups of one answer |
| FORMAT | a pipe | already generic, already universal, nothing to design |

This is the owner ruling of 2026-08-21 applied to both middle axes, not just to
selector words. The command path names the subject and stops. Everything after
it is a pipe, and the chain reads left to right as a sentence:

```
show bgp | peer 10.0.0.1 | summary | table
   ^          ^              ^        ^
subject     scope       presentation format
```

FORMAT is already there. SCOPE exists on two commands. PRESENTATION exists on
two commands. The structure is built and it is used by one family.

### 1.3 The three mechanisms, and which axis each one serves

A `|` word can be served in three different ways, and the difference decides
what a proposal costs. This is the part the framing of this audit got half
right, so it is stated with the evidence.

| # | Mechanism | Registered by | Runs | Can it compute? |
|---|-----------|---------------|------|-----------------|
| 1 | generic operator | `knownPipeOps` in `internal/component/command/pipe.go` | on the answer | only `count`, which counts rows |
| 2 | pipe ALIAS | `command.RegisterAliases`, or `sdk.PipeDecl` for a plugin | on the answer, as a chain of mechanism 1 | no. It only names operators |
| 3 | command-owned pipe FILTER | `command.RegisterPipeFilters` | IN THE HANDLER, before the answer exists | yes. Anything the handler can compute |

Mechanism 3 is the one that changes the picture. `foldFilters`
(`internal/component/command/pipe.go`) rewrites a command-owned filter into
COMMAND ARGUMENTS and returns the rewritten command string, which the caller
then dispatches:

```go
	allServerArgs := make([]string, 0, len(leadingArgs)+len(serverArgs))
	allServerArgs = append(allServerArgs, leadingArgs...)
	allServerArgs = append(allServerArgs, serverArgs...)
	if len(allServerArgs) > 0 {
		var tb textbuf.Buffer
		command = tb.Str(trimmed).Byte(' ').Join(allServerArgs, " ").String()
	}
```

So `show bgp rib | peer 10.0.0.1` is dispatched as the literal command string
`show bgp rib peer 10.0.0.1`, and the RIB plugin computes the filtered answer.
`show bgp rib | histogram` and `show bgp rib | graph` are the proof that this
layer computes: both are `PipeFilter` names
(`internal/component/bgp/plugins/cmd/rib/rib.go`), and `histogramTerminal` and
`graphTerminal` (`internal/component/bgp/plugins/rib/rib_pipeline.go`,
`rib_topology.go`) build a bucket count and an AS topology drawing inside the
plugin.

**So "the pipe layer cannot compute" is true of mechanisms 1 and 2 and false of
mechanism 3.** The correct rule is about WHEN, not about pipes:

> A pipe that runs AFTER the answer is built can only select. A pipe that folds
> INTO the command runs before the answer is built and can compute anything.

This maps onto the axes exactly. SCOPE has to run at the source, because
narrowing after the fetch is wasteful and, for a per-item view that joins in
metadata, impossible. PRESENTATION should run on the answer, because it is pure
selection and every command should get it for free.

### 1.4 What a command must EMIT for a presentation to be selectable

`| display` selects keys. `| fill` re-sequences them
(`internal/component/command/pipe_columns.go`). Neither renames, sums, counts
by predicate, or invents a constant. So a presentation alias is possible only
when the command already wrote the fields the alias names.

`show bgp rpki` is the worked precedent and it is closed and landed. Four of the
seven fields `| summary` selects had to be added to the payload first, because
`vrp-count` is a sum, `sessions-established` is a count over a state,
`sessions-total` is a rename, and `validation-enabled` is a constant
(`appendSummaryFields`, `internal/component/bgp/plugins/rpki/rpki.go`).

Five emission rules follow, and they are what generalizes across families.

**E1. The answer is an object with a rows key. Never a bare array.**
A bare array has nowhere to hang an aggregate, so no `| summary` can ever be
added to it without breaking every existing reader. `show sysctl`
(`(*store).showEntries`, `internal/component/sysctl/sysctl.go`) returns a bare
`[]showEntry`. So does `show sysctl keys` (`listKnownKeys`). Those two commands
are shut out of the presentation axis until the shape changes.

**E2. The rows are a list, and the identity is a FIELD of the row.**
Not a map keyed by identity. Two reasons, both in the code.
`applyDisplaySelect` streams exactly two shapes, `[...]` and `{"key":[...]}`,
one record at a time, and decodes every other shape whole
(`selectSequence`, `internal/component/command/pipe_columns.go`). And a map key
is not a field, so `| display address` cannot name it. `show bgp` writes
`peers` as a list of rows each carrying `address`. `show bgp peer <sel>` writes
`peers` as a map keyed by address. One family, two shapes, one key name.

**E3. The aggregates and the rows are siblings at the top level.**
This is what makes `| summary` a selection among siblings rather than a descent
into an envelope. `handleBgpSummary` says so in its own comment, and
`appendSummaryFields` plus `appendCacheServers` do the same for RPKI.

**E4. One subject, one row schema.** Every command of a family writes rows of
the same record with the same key spellings, whatever the scope. Scope changes
how many rows. Presentation changes how many fields. NOTHING RENAMES. This is
the rule BGP breaks three times, and it is the single reason
`show bgp peer 10.0.0.1 | summary` cannot be made to mean the same thing as
`show bgp | summary` today.

**E5. A computed aggregate is emitted by the command, beside the rows.**
A sum, a count over a state, a rename, and a constant are the four things a
presentation wants and `| display` cannot produce. RPKI needed all four. A
proposal that needs one of them is a payload change, and it is cheap: it is one
loop the handler already runs.

### 1.5 The field manifest, and why `RegisterColumns` is load-bearing

`command.RegisterColumns` looks cosmetic. It is not. It is the only in-process
statement of what a command emits, and two things read it:

- `completeDisplayFields` (`internal/component/command/completer.go`) offers
  `| display` field names from the declared order and from nowhere else. A
  command with no declared order offers no completion, and the operator has to
  know the key names by heart.
- the table and text renderers, for column sequence.

An alias expansion is a SECOND copy of that field list, and `display` reports no
miss. A field added to the payload and not to the expansion is dropped from the
alias in silence and no test fails. The RPKI conversion states the obligation
and discharges it with `summaryFieldNames`, `buildSummaryAliasExpansion` and two
tests. Every presentation this audit proposes carries the same obligation.

Two commands carry a real column order today: `show bgp` and
`show bgp peer list`.

### 1.6 A closed presentation vocabulary

A matrix needs the same words in every row of it. Five words cover every family
this audit read.

| Pipe | Meaning at EVERY scope | Mechanism | What the command must emit |
|------|------------------------|-----------|----------------------------|
| (none) | every field of every row in scope, plus the aggregates | already the default | E1, E2, E3 |
| `\| summary` | the short form: the aggregate keys, plus the few fields of each row that identify it and say whether it is healthy | 2, alias to `display` | the aggregates (E5) and a digest field set on the row |
| `\| <plural>` (`peers`, `interfaces`, `sessions`, `routes`) | the rows alone, full fields, no aggregates | 2, alias to `display <rows-key>` | nothing new. E1 and E3 are enough |
| `\| count` | how many rows are in scope | 1, exists | nothing, but read the next paragraph. It counts rows only after a rows selection |
| `\| errors` | only the rows that are not healthy | 3, a filter, or a new generic operator | a state field, and a handler that filters on it |

And two scope words, per family, always mechanism 3:

| Pipe | Meaning |
|------|---------|
| `\| <selector-keyword> <value>` (`peer`, `name`, `id`, `address`, `key`, `type`) | the rows whose identity matches |
| `\| family <afi/safi>` | the rows of one address family |

**`| count` and rule E3 are in conflict today, and somebody has to decide it.**
`countItems` (`internal/component/command/pipe.go`) unwraps a map of ONE key and
counts the array inside it. A map of two or more keys answers the NUMBER OF
KEYS. So the shape E3 asks for, aggregates beside the rows, is exactly the shape
that makes `| count` answer nonsense: `show bgp | count` answers 6, which is how
many top-level keys `handleBgpSummary` writes.

Composition still works, because a rows selection leaves one key:
`show bgp | peers | count` answers the peer count. But every command that grows a
`count` key beside its rows loses `| count`, and nine commands in the interface,
host, system and traffic families have already done it. Either `| count` learns
to find the rows key, or the vocabulary states that a count is always spelled
`| <plural> | count`. This audit recommends the first, because an operator will
type the second only after being told.

`| detail` is deliberately NOT in the list. The bare command is already the full
view, so `detail` can only mean "the same thing", which is how
`show ospf ipv6 database detail` came to be byte-identical to its parent. Worse,
`detail` REMOVES fields in OSPF today (`show ospf neighbor detail` drops `bfd`).
A word that means "more" in one command and "less" in another is not a word an
operator can use.

### 1.7 What `| summary` has to mean for the matrix to close

This is the one open decision in the structure, and it is the owner's.

`show bgp | summary` today expands to
`display router-id local-as uptime peers-configured peers-established`
(`registerAliases`, `internal/component/bgp/plugins/cmd/peer/peer.go`). It
returns the aggregates and DROPS the peer rows. `| peers` returns the rows and
drops the aggregates.

That meaning cannot compose with scope. The aggregates of one peer are "1
configured, 1 established", which answers nothing. So
`show bgp | peer 10.0.0.1 | summary` under the current meaning is useless, and
the owner's worked example asks for exactly that spelling.

Two ways out.

- **A. `summary` means "the short form of this answer".** It selects the
  aggregate keys AND a digest field set on each row. At full scope that is
  totals plus one line for each peer, which is what `show ip bgp summary` means
  on every other router an operator has used. At one-peer scope it is the one
  line for that peer. One word, one meaning, and the matrix closes.
- **B. `summary` keeps meaning "the aggregates alone", and a second word carries
  the digest.** `| brief` is the obvious candidate, and
  `show interface brief` and `show bgp peer list` are the two existing
  subcommands that already mean it.

A is recommended. It costs the existing alias one expansion change, it makes the
word mean what an operator arriving from another vendor expects, and it removes
the need for a sixth word. Under A, `| peers` keeps its job, which is the rows
at FULL width.

### 1.8 Why the matrix mostly works already, and the one barrier

The mechanism resolves an alias and a filter by the LONGEST REGISTERED COMMAND
PATH that is a prefix of the typed command (`commandRegistry.lookup` and
`commandMatchesPrefix`, `internal/component/command/column_order.go`). The
typed command still carries its arguments at that moment, so
`show bgp peer 10.0.0.1` resolves whatever `show bgp peer` declares, and
`show bgp` if `show bgp peer` declares nothing.

That is inheritance, and it is exactly what a matrix needs: register a
presentation once at the family root and every scope below it answers to the
same word.

Today inheritance is deliberately switched OFF for the whole of BGP.
`cmdBgpChildren` registers an EMPTY alias set and an EMPTY column order on all
ten direct children of `show bgp`, including `show bgp peer`, precisely so that
`| summary` and `| peers` do not reach an answer with no peer rows in it. The
barrier is correct for the payloads that exist. It is also the thing standing
between the operator and `show bgp peer 10.0.0.1 | summary`.

The barrier comes down per path, not globally, and only when the payload under
that path satisfies E2 and E4.

One asymmetry worth knowing, because it argues for the pipe spelling on its own:
`ColumnsForCommand` is resolved on the command as TYPED, before `foldFilters`
rewrites it (`ProcessPipesChecked` and `ProcessPipesDetectLog`,
`internal/component/command/pipe.go`). So `show bgp | peer 10.0.0.1` keeps the
column order declared on `show bgp`, while `show bgp peer 10.0.0.1` resolves the
empty declaration on `show bgp peer` and renders alphabetically. The pipe
spelling reads better than the path spelling for free.

---

## 2. BGP, worked fully

BGP is the reference case because it is the only family with any of this today,
and because the owner's example lives in it.

### 2.1 The matrix an operator needs

Rows are SCOPE. Columns are PRESENTATION. Each cell shows the spelling that
should exist and, in brackets, the spelling that exists today.

| Scope \ Presentation | full | `\| summary` | `\| peers` | `\| count` |
|---------------------|------|-------------|-----------|-----------|
| everything | `show bgp` (exists) | `show bgp \| summary` (exists, aggregates only) | `show bgp \| peers` (exists) | `show bgp \| peers \| count` (exists) |
| one family | `show bgp \| family ipv6/unicast` (exists as `show bgp ipv6`, a bare positional) | same, plus `\| summary` (missing) | missing | missing |
| one peer or a peer set | `show bgp \| peer <sel>` (exists as `show bgp peer <sel>`, different shape) | missing, and it is the owner's example | not meaningful at this scope | `show bgp \| peer <sel> \| count` (missing) |
| the routes | `show bgp rib` (exists) | `show bgp rib status` (exists as a subcommand) | n/a | `show bgp rib \| count` (exists) |
| one peer's routes | `show bgp rib \| peer <sel>` (exists) | `show bgp rib \| peer <sel> \| count` (exists) | n/a | exists |

The route half of the family is already a matrix. The peer half is not.

### 2.2 What each command emits today

Read from the producer, not from YANG.

| Command | Producer | Top-level keys | Row keys |
|---------|----------|----------------|----------|
| `show bgp` | `handleBgpSummary`, `internal/component/bgp/plugins/cmd/peer/summary.go` | `router-id`, `local-as`, `uptime`, `peers-configured`, `peers-established`, `peers`, and `family` plus `peers-in-family` when a family argument was given | `address`, `name`, `description`, `remote-as`, `peer-type`, `state`, `state-changed`, `last-error`, `uptime`, `updates-received`, `updates-sent`, `keepalives-received`, `keepalives-sent`, `eor-received`, `eor-sent`, `connections-dropped`, and `routes-received`, `routes-accepted`, `routes-sent` when the RIB plugin answered |
| `show bgp peer <sel>` | `handleBgpPeerDetail`, `internal/component/bgp/plugins/cmd/peer/peer.go` | `peers`, a MAP keyed by address | `remote-as`, `local-as`, `router-id`, `peer-type`, `timer`, `connect`, `accept`, `state`, `uptime`, `updates-*`, `keepalives-*`, `eor-*`, `messages`, `connections-established`, `connections-dropped`, `flap-count`, `connect-retry-counter`, `capabilities`, and about fifteen conditional keys |
| `show bgp peer list` | `handleBgpPeerList`, same file | `peers`, a MAP keyed by address | `remote-as`, `state`, `uptime`, and `name` and `group` when set |
| `show bgp peer <sel> statistics` | `handleBgpPeerStatistics`, `summary.go` | none. A BARE record for one peer, a BARE array for several | `address`, `remote-as`, `state`, `uptime`, `updates-*`, `keepalives-*`, `eor-*`, `rate-updates-received`, `rate-updates-sent`, `rate-keepalives-received`, `rate-keepalives-sent` |
| `show bgp health` | `handleShowBGPHealth`, `internal/component/bgp/plugins/cmd/peer/health.go` | `peers` (a list), `count`, `not-established` | `peer`, `state`, `as`, `uptime` |
| `show bgp rib status` | `(*RIBManager).status`, `internal/component/bgp/plugins/rib/rib_commands.go` | `running`, `peers`, `routes-in`, `routes-out`, `stale-routes`, `route-counts`, and `gr-state` when a peer is restarting | `route-counts` is a map of address to `{in, out}` |
| `show bgp rpki` | `appendSummaryFields` plus `appendCacheServers`, `internal/component/bgp/plugins/rpki/rpki.go` | `vrp-count`, `validation-enabled`, `sessions-total`, `sessions-established`, `sessions-synced`, `aspa-enabled`, `aspa-records`, `cache-servers` | `address`, `port`, `state`, `synced`, and more |

### 2.3 The three defects that stop the matrix closing

**D1. `peers` is a list here and a map there.** `show bgp` writes a list of rows
carrying `address`. `show bgp peer <sel>` and `show bgp peer list` write a map
keyed by address, with no `address` field in the row. Same key name, two types.
A presentation alias is written against a shape, so no single alias serves both.
E2 says the list wins, and streaming is the reason.

**D2. The same field has three names.** `address` in `show bgp`, the map key in
`show bgp peer`, and `peer` in `show bgp health`. `description` in `show bgp` and
`group` in `show bgp peer list` and `show bgp peer <sel>`. `remote-as` in
`show bgp` and `as` in `show bgp health`. Every rename is unreachable from
`| display`, which is why each of those three leaves still has to exist as its
own command.

**D3. `show bgp peer <sel> statistics` has no envelope at all.** One matching
peer returns a bare record and several return a bare array. It is the only BGP
answer with no top-level key, so E1 and E3 both fail and nothing can ever be
added beside it.

### 2.4 What `show bgp` must emit

For the presentations in section 2.1, and nothing more.

| Presentation | Fields it selects | Emitted today? |
|--------------|-------------------|----------------|
| `\| summary` at full scope, under meaning A | `router-id`, `local-as`, `uptime`, `peers-configured`, `peers-established`, and on each row `address`, `description`, `remote-as`, `state`, `uptime` | YES. Every one of these is already written. Meaning A costs an expansion change and no payload change |
| `\| summary` at one-peer scope | the same row digest | NO, and the reason is D1 and D2, not a missing field. The scoped handler writes a map, and it writes `group` where the parent writes `description`, and it writes no `address` at all |
| `\| peers` | `peers` | YES |
| `\| count` | none | YES |
| `\| errors` (the peers that are not established) | a row filter on `state`, which no generic operator provides | NO. See section 4 |
| a `\| health` that replaces `show bgp health` | `peers-not-established` beside `peers-configured` | NO. It is `peers-configured` minus `peers-established`, one subtraction the handler already has both terms for. This is an E5 payload change of one line |

So the answer to "what must `show bgp` emit" is short, and that is the point:
**`show bgp` already emits almost everything the presentation axis needs.** One
field is missing (`peers-not-established`). What is missing is not fields. It is
SHAPE AGREEMENT between `show bgp` and the peer-scoped commands, and the removal
of the empty alias declaration on `show bgp peer` once that agreement holds.

### 2.5 The BGP work, ranked by what an operator would notice

1. **`show bgp | peer <sel>` does not exist, and its path spelling answers with
   a different shape.** This is the owner's example. It needs a `peer` filter on
   `show bgp` (mechanism 3), and `handleBgpPeerDetail` rewritten to E2: `peers`
   as a list, `address` as a field, `description` not `group`.
2. **`| summary` does not compose.** Decide meaning A or B (section 1.7). Under
   A the expansion gains the row digest and every scope answers.
3. **`show bgp health` is a rename of `show bgp`.** Add `peers-not-established`,
   register `| health`, and the leaf goes.
4. **`show bgp peer list` is `show bgp | peers` with fewer columns.** It is the
   `| summary` row digest under another name. It goes when 1 and 2 land.
5. **`show bgp <family>` is a scope spelled as a bare positional argument.**
   Every other scope in the family is spelled `| family <afi/safi>`, on
   `show bgp rib` and `show bgp rib best`. One family, one concept, two
   grammars.
6. **`show bgp peer <sel> statistics` is four divisions and a shape.** The ten
   non-rate fields are all in the parent's rows. The four `rate-*` fields are a
   counter divided by uptime, which no operator provides, so they are an E5
   payload change on `show bgp` or the leaf stays.

---

## 3. The other families that need this

Ranked by how much an operator would notice. Each entry says what exists, which
presentations should exist, and what the command must emit for each one.

### 3.1 `show interface`

The most typed command in the tree after `show bgp`, and it has four leaves that
are three presentations and a scope.

| Registered path | What it really is |
|-----------------|-------------------|
| `show interface` | the everything view |
| `show interface brief` | a presentation |
| `show interface errors` | a presentation, and a row filter |
| `show interface type <t>` | a scope |
| `show interface scan` | a presentation |
| `show interface name <n> detail` | a scope, and its rows are IDENTICAL to the root's rows |
| `show interface name <n> counters` | a scope plus a presentation |
| `show interface rate` | a genuine subcommand. `iface.ListRates` reads a two-sample delta from `globalTracker` |

Three envelope shapes in one family. `show interface`, `show interface scan` and
`show interface rate` answer a BARE ARRAY. `show interface type <t>` and
`show interface errors` answer `{"interfaces":[...]}`. `show interface brief`
answers `{"interfaces":[...],"count":N}`. `show interface name <n> detail`
answers a bare object.

| Presentation | Must emit | Blocked on |
|--------------|-----------|-----------|
| `\| brief` | `address` as a row field. It is a CONCATENATION today, `Addresses[0].Address + "/" + PrefixLength`, built inside `showInterfaceBrief` (`internal/component/iface/cmd/show_interface.go`). `\| display` cannot build it | 4.1, one field |
| `\| errors` | the four counters are inside `stats` in the root and HOISTED to row level in `showInterfaceErrors`. `selectRecord` never hoists. And dropping the all-zero rows is a row predicate | 4.1 for the hoist, 4.3 for the predicate |
| `\| type <t>` | nothing. It is a scope, and the folded string `show interface type <t>` is the registered command | 4.2, and the handler already exists |
| `\| name <n>` | nothing. `showInterfaceDetail` emits the same `InterfaceInfo` the root emits, so E4 already holds | 4.2, and the handler already exists |
| anything at all on the root | an object with an `interfaces` key. The root answers a bare array today | E1 |

`show interface name <n> detail` is the one place in the whole audit where a
scoped view already satisfies E4 exactly. It is the proof the rule is reachable.

### 3.2 The session families: BFD, VRRP, IPsec, L2TP, PPPoE, subscriber, BMP, route server, route reflector, healthcheck

Ten families, one shape, one missing answer. Every one of them tracks a set of
sessions with a state, and the first question an operator asks each morning is
**how many are up out of how many are configured**.

**Exactly one family emits that number, and it emits it in the wrong command.**
`show vpn ipsec status` writes `engine-running`, `configured-peers`,
`active-ike-sas` and `established-sas`
(`handleShowVPNIPsecStatus`, `internal/component/ike/cmd/show_ipsec.go`), while
the rows live in `show vpn ipsec sa`. Two commands, one payload split in half,
so no alias can bridge them. That is the RPKI move, unmade: put the four
aggregate keys beside `peers` in one answer and `| summary` follows.

Nowhere else in the ten does the number exist in any spelling:

| Family | The aggregate today | What it must emit |
|--------|---------------------|-------------------|
| `show bfd` | none. `show bfd sessions` is a bare array | `sessions-total`, `sessions-up`, `sessions-down`, `sessions-admin-down`, and a root command to hold them |
| `show vrrp` | none. The root answers `{"vrrp":[...]}` | `groups-total`, `groups-master`, `groups-backup` beside `vrrp` |
| `show l2tp` | `tunnel-count`, `session-count`, and no state breakdown. The ceiling, `max-tunnels` and `max-sessions`, lives in `show l2tp config` | `tunnels-established`, `sessions-established`, and the two ceilings, on the root |
| `show pppoe` | `session-count`, which mixes discovery, session and teardown | `sessions-up`, counted over `state` |
| `show subscriber` | `total`, `pppoe`, `l2tp`. Counted by ACCESS TYPE, never by state | `active`, `authenticating`, `terminating` beside `total` |
| `show bmp` | none | `collectors-total`, `collectors-connected`, `peers-total`, `peers-up` |
| `show bgp rs`, `show rr` | `{"running": true}`, a hard-coded constant that is never false | `peers-configured`, `peers-established` |
| `show bgp healthcheck` | none. The answer is a bare array | `probes-total`, `probes-up`, `probes-down`, `probes-disabled`, and an envelope |
| `show bgp adj-rib-in` | `total-routes` and a per-peer count map. `r.peerUp` holds peer state and is never serialized | `peers-total`, `peers-up` |

Every entry in that table is a COUNT BY PREDICATE over a field the command
already has. None of it is reachable from `| display`. All of it is section 4.1
work, and it is the same five-line loop ten times.

**The good news is that E4 already holds in four of them**, which is more than
anywhere else in the tree:

| Family | Everything view | Per-item view | Agreement |
|--------|-----------------|---------------|-----------|
| `show bfd` | `show bfd sessions` | `show bfd session address <a>` | the same `SessionState`, array against object |
| `show vrrp` | `show vrrp` | `show vrrp interface name <if>` | the same rows, the same `vrrp` envelope key |
| `show vpn ipsec` | `show vpn ipsec sa` | `show vpn ipsec peer name <n>` | the same `saToMap` rows. The ENVELOPE key changes from `peers` to `ike-sas`, which is the only thing to fix |
| `show flow` | `show flow export` | `show flow export name <n>` | identical, promoted to the top level |

**`show subscriber` is the cheapest presentation win in the whole tree.** Its
root already carries `total`, `pppoe`, `l2tp` and `sessions` as siblings, which
is the `show bgp` shape. `| summary` (`display total pppoe l2tp`) and
`| sessions` (`display sessions`) are registrable today with NO handler change
at all. It is the only command outside BGP in that position.

One rename blocks the matrix there: the same Go field is spelled `interface` in
`sessionBrief` and `ppp-interface` in `sessionFull`
(`internal/component/l2tp/subscriber/cmd/subscriber.go`), so one alias name
cannot carry both scopes.

Two structural notes for this group.

- `show l2tp` and `show pppoe` have roots that emit ONLY aggregates, with no
  rows at all, and put the rows in sibling commands (`tunnels`, `sessions`).
  That is E3 inverted, and it costs the same thing: neither half can select the
  other.
- `show dhcp` does not exist in any spelling. `internal/plugins/dhcpserver`
  declares no commands and no `OnExecuteCommand`, and its `lease` type carries
  no JSON tags. There is nothing to present.

### 3.3 `show host`

`show host` and `show host all` are ONE command spelled twice, and eight section
leaves are each one key of the record it returns. This is the largest pure
`| display` conversion in the tree.

`host.Detect` returns `Inventory` with `cpu`, `nics`, `dmi`, `memory`,
`thermal`, `storage`, `kernel`, `host`, `platform`, `errors`
(`internal/component/host/inventory.go`). Every section command returns one of
those values unwrapped.

| Presentation | Must emit | Blocked on |
|--------------|-----------|-----------|
| `\| cpu`, `\| dmi`, `\| memory`, `\| kernel`, `\| platform`, `\| thermal`, `\| storage` | nothing. `show host \| display cpu` already answers what `show host cpu` answers | nothing. Seven aliases and seven deletions |
| `\| nic` | the key is `nics`, so the natural word does not select it. Either the alias is named `nics` or the key is renamed | 4.1, one choice |
| a one-line inventory digest (vendor, model, cores, RAM, kernel) | a `summary` section. Every field it wants is nested one or two levels down, and `\| display` cannot pull `dmi.system-vendor` to the top | 4.1, E5 |
| "is any of this unhealthy" | a health rollup over ECC errors, thermal alarms and throttles. Three counts by predicate | 4.1, E5 |

`Inventory.host` (`hostname`, `uptime-seconds`, `timezone`) and
`Inventory.errors` are reachable ONLY through the everything view, because
`sectionDetectors` has no entry for either.

### 3.4 The routing tables: `show route`, `show rib`, `show fib`, `show mpls`

An operator reads a routing table more often than anything except a peer table,
and asks two questions of it that no spelling answers: how many routes, and how
many by protocol and by family.

| Command | Producer | Shape |
|---------|----------|-------|
| `show route` | `handleShowRoute` and `dumpKernelRoutes`, `internal/component/iface/cmd/show_route.go` | `{routes: [...]}`, plus `truncated` and `limit` only when it truncated |
| `show rib` | `(*sysRIB).showRIB`, `internal/component/sysrib/sysrib.go` | a BARE ARRAY |
| `show ecmp-groups` | `(*sysRIB).showECMPGroups`, same file | a BARE ARRAY |
| `show fib kernel`, `vpp`, `p4` | three plugins | three row shapes, no root |
| `show mpls forwarding` | `handleShowMPLSForwarding`, `internal/component/mpls/show_forwarding.go` | `{entries: [...]}` |

| Presentation | Must emit | Blocked on |
|--------------|-----------|-----------|
| `\| summary` on `show route` and `show rib` | `routes-total`, `by-protocol`, `by-family`, beside the rows. Two of the three are counts by predicate | 4.1, E5 |
| `\| count` on `show route` | nothing. The envelope is single-key today, so it already works | nothing |
| anything on `show rib` and `show ecmp-groups` | an envelope. Both answer bare arrays | E1 |
| `\| ecmp` replacing `show ecmp-groups` | `showECMPGroups` writes `paths` where `showRIB` writes `ecmp-paths` for the same element type. One rename, then it is a row filter on "has more than one path" | 4.1 for the rename, 4.3 for the predicate |
| a `\| display next-hop` an operator can carry between families | `iface.KernelRoute` tags the field `nexthop` while sysrib, MPLS and OSPF all write `next-hop` (`internal/component/iface/iface.go`). The same key, two spellings, and `\| display next-hop` silently returns the whole record on `show route` | 4.1, one tag |
| a backend-agnostic `show fib` | a root that merges the three, each row carrying a `backend` constant | E1, E5, and the three row shapes have to agree first |

### 3.5 The link-state protocols: `show ospf`, `show isis`, `show ldp`, `show rsvp-te`

These four share one problem and it is not presentation. **None of them accepts
an argument at all.** Every out-of-process handler discards its args, and the
in-tree forwarder rejects any argument outright: `forwardToOSPF`
(`internal/plugins/ospf/cmd_show.go`), `forwardToISIS`
(`internal/plugins/isis/cmd_show.go`), `forwardToLDP`, `forwardToRSVPTE`. So
there is no per-item view of an OSPF neighbor, an IS-IS adjacency, an LDP
binding, or an RSVP-TE tunnel, in any spelling.

The scope axis in these families needs the forwarder relaxed and the engine
switch taught to read arguments. That is per-family work, and it is a
precondition for any scope pipe.

`show ospf` is a command at its root, and it answers from `e.cfg` alone
(`(*engine).processSummary`, `internal/plugins/ospf/show_summary.go`). It emits
`router-id`, `abr`, `asbr`, `area-count`, `areas`, `stub-router`, and it never
reads the neighbor table, the LSDB, the SPF result or the TED. So the one
command an operator types to ask "is OSPF healthy" is a config echo.

| Presentation | Must emit | Blocked on |
|--------------|-----------|-----------|
| `\| summary` on `show ospf` that means anything | `neighbors-total`, `neighbors-full`, `interfaces-total`, `interfaces-up`, `lsa-count`, `spf-last-run` | 4.1, E5, and the handler has to read the runtime state it currently ignores |
| a neighbor state histogram | `neighbors-by-state` | 4.1, E5 |
| `\| areas` and `\| stub-router` on `show ospf` | nothing. Both keys are already written | NOTHING. An alias is the one thing an out-of-process plugin CAN declare, and the OSPF plugin declares 58 commands and no `Pipes` at all (`internal/plugins/ospf/register.go`) |
| a `show isis` root at all | a new command emitting `system-id`, `hostname`, `levels`, `adjacencies-up`, `adjacencies-total`, `lsps-l1`, `lsps-l2`, `overload`, `last-spf` | there is no root |
| an envelope on any IS-IS, LDP or RSVP-TE view | every producer in all three families answers a BARE ARRAY | E1 |

**The nine-leaf OSPF gap the earlier audit left open is now closed.** Seven of
the nine emit state their parent never reads, so they are SUBCOMMAND:
`border-routers` (SPF output, `spf.Computer.BorderRouterSnapshot`),
`graceful-restart` and `ipv6 graceful-restart` (`e.gr`), `ldp-sync`
(`e.ldpSync`), `te-database` (`e.ted`), `virtual-links` (`e.virtualLinks`), and
`instance`, which is a rollup with `neighbor-count` and `lsa-count` the parent
never reads. Two are not:

- `show ospf ipv6 database extended` is `v6DatabaseDetail(set, "extended", "")`,
  which is the same function its SIBLING `show ospf ipv6 database` calls with an
  empty filter. It is a row filter of a sibling, not of the parent.
- `show ospf ipv6 instance` shares four of five keys with `show ospf ipv6` and
  reaches them through the same calls. If `afSummary` also emitted `areas`, the
  leaf would be exactly
  `show ospf ipv6 | display address-family instance-id router-id areas neighbors`.

So the PIPE count in the earlier audit rises by one leaf and one sibling case,
not by nine.

### 3.6 `show system`, `show runtime`, `show storage`, `show traffic`

Four families with NO command at their root. A pipe cannot attach to a path that
does not dispatch, so every presentation in these families is blocked on a root
command existing first. The earlier audit counted 121 such prefixes.

`show system` is the worst of them, because fifteen leaves across five owner
packages answer questions an operator asks together, and no spelling gathers
them. It is also where the family vocabulary breaks down: `show system memory`
is `/proc/self/status`, `show runtime memory` is the Go allocator, and
`show host memory` is the DIMMs. Three commands, one word, no shared key.

The cheapest presentation work in this group needs no root at all:

| Command | Words it already parses as ARGUMENTS | What a `PipeFilter` would cost |
|---------|--------------------------------------|-------------------------------|
| `show system sockets` | `port`, `state`, `tcp`, `udp` | a registration. No handler change |
| `show system kernel-log` | `level`, `count` | a registration. No handler change |
| `show capture` | `bgp`, `l2tp`, `peer`, `tunnel-id`, `count` | a registration. No handler change |
| `show traffic usage` | `name` | a registration. No handler change |

That is mechanism 3 with the handler side already written, which is the same
position `show bgp rib` was in before its filters were declared.

`show traffic stat <iface>` carries a defect the presentation axis exposes: the
scope filters `interfaces` and leaves `top-source-ips`, `top-dest-ips`,
`top-ports`, `protocol-mix` and `history` box-wide
(`internal/component/trafficstat/cmd/traffic.go`). A scope that narrows some
keys and not others is not a scope.

### 3.7 The record families: `show event`, `show log`, `show audit`, `show dns cache`, `show pki`, `show flow`, `show ddos`, `show anomaly`

These all answer "here are the records, newest first". They are the families
where a presentation vocabulary would pay back fastest, because an operator
reads them under time pressure.

Two of them already satisfy E4 exactly, and they are the only two in the whole
audit besides `show interface name <n> detail`:

- `show dns cache record <name>` emits the same row keys as
  `show dns cache list`. Both come from `getDNSCacheEntries`
  (`internal/component/resolve/cmd/show_dns.go`). The per-item form adds one
  top-level key, `filter`.
- `show flow export name <n>` emits the same keys as `show flow export`
  (`internal/plugins/flowexport/cmd_show.go`), promoted to the top level.

Everywhere else the per-item view renames, drops or re-types a field. `show pki`
is the sharpest case, because the rename hides a difference in MEANING:

| Concept | `show pki certificates` | `show pki certificate name <n>` |
|---------|-------------------------|----------------------------------|
| subject | `subject-cn`, the CN alone | `subject`, the full DN |
| issuer | `issuer-cn` | `issuer` |
| validity | `valid`, which is `now.Before(NotAfter)` | `chain-valid`, an x509 chain verify |

An operator who learns `| display valid` on the list and types it on one
certificate gets nothing back, and the field that looks like its equivalent
answers a different question.

| Presentation | Family | Must emit | Blocked on |
|--------------|--------|-----------|-----------|
| `\| summary` on any of them | all | a rollup key set. `show ddos status` already IS one, spelled as a sibling command | 4.1, E5 |
| `\| errors`, `\| failed`, `\| expiring` | audit, pki, flow | audit emits `outcome` and `parseAuditFilter` (`internal/component/cmd/show/audit.go`) has no `outcome` case, so the field cannot be filtered on even server side. PKI needs `days-remaining` and `expiring`, and `expiryWarnDays = 30` already exists in `internal/component/pki/health.go` where no show payload can reach it | 4.1 plus 4.3 |
| counts by a field (`by-type`, `by-level`, `by-actor`, `by-family`) | all | the count itself. It is a count by predicate, so the handler must write it | 4.1, E5 |
| `\| count` and `\| first N` at all | event, log, audit, pki, policy, dns cache, subsystem list | nothing. They break on the two-key envelope, see section 1.6 | the `\| count` decision |

`show ddos` carries the collision this audit's vocabulary is designed to stop:
`incidents` is an INTEGER in `show ddos status` and an ARRAY in
`show ddos incidents` (`internal/plugins/ddos/observe/show.go`). One family, one
word, two types.

`show anomaly` has the same disease between two EVERYTHING views:
`show anomaly detect` writes `at` where `show anomaly observe` writes
`start-time`, over overlapping data.

### 3.8 The families the pipe layer cannot reach at all

Five families are declared in YANG, registered with
`registry.MustRegisterLocal`, and have NO daemon RPC:
`show env`, `show data`, `show schema`, `show config`, `show yang`.

`registry.LookupLocal` is reached from `cmd/ze/ze_core_dispatch.go` and
`cmd/ze/internal/cmdutil/cmdutil.go` only, and `RunCommand` calls no
`ProcessPipes*` function. The handler prints text to stdout and returns an exit
code. `ProcessPipes*` is called from the SSH exec path, the TUI and the CLI
client (`internal/component/ssh/ssh.go`, `internal/component/cli/model_mode.go`,
`internal/component/cli/client/main.go`).

So for these five, no alias, no filter and no column order would ever be
consulted, and most of them answer text rather than JSON. `show config dump`
already carries a second job: the gRPC `GetRunningConfig` and the REST config
handler execute the literal string to mean "the config".

Presentation work on these five is blocked behind a DISPATCH decision, not a
pipe decision. They belong in a different spec.

## 4. What is blocked, and on what

Five groups, and only one of them waits on something nobody has built.

### 4.1 Needs only a payload change

The command already fetches everything. It writes the answer in a shape or with
key names that no operator can select from. The fix is in the handler that
builds the answer, and it is usually one loop and a few keys.

| What | Which rule | Cost |
|------|-----------|------|
| An answer that is a bare array gets an object with a rows key | E1 | one wrapper, and every existing reader of that command changes with it |
| Rows keyed by identity become a list carrying the identity as a field | E2 | one loop, and the same reader cost |
| An aggregate that no command emits (`peers-not-established`, "how many sessions are up") | E5 | one counter in a loop the handler already runs |
| A renamed key (`peer` for `address`, `as` for `remote-as`, `group` for `description`) | E4 | one string, and the leaf that carried the rename disappears |

This group is the majority of the presentation work in every family this audit
read. It blocks on nothing.

### 4.2 Needs a per-command handler change, and no new mechanism

Every SCOPE pipe is here. `RegisterPipeFilters` plus `foldFilters` already turn
`| <name> <value>` into command arguments, and the dispatcher already passes
tokens no `ArgDef` claims through to the handler (`validateCommandArgs`,
`internal/component/plugin/server/command.go`, phase 2: "With nothing left to
fill, the token is the handler's business and passes through"). So a scope pipe
needs two things and neither is generic work:

- a `PipeFilter` registration on the family root.
- a handler that reads the folded arguments.

For a family whose per-item command already exists, the second is FREE, because
the folded string is the command an operator types today. `show sysctl | key
net.ipv4.ip_forward` folds to `show sysctl key net.ipv4.ip_forward`, and that
is exactly the registered command
(`internal/component/sysctl/register.go`, the `OnExecuteCommand` switch).

Three pairs were checked byte for byte, and in all three the fold reproduces the
registered per-item command string exactly:

| Typed | Folds to | Which is registered as |
|-------|----------|------------------------|
| `show sysctl \| key <k>` | `show sysctl key <k>` | a plugin command, `internal/component/sysctl/register.go` |
| `show metrics \| name <n>` | `show metrics name <n>` | an in-tree RPC, `ze-show:metrics-query` |
| `show dns cache \| record <n>` | `show dns cache record <n>` | an in-tree RPC, `ze-show:dns-cache-record` |

**This corrects the framing this audit was given.** `show sysctl | key <k>` is
NOT blocked on a generic field-aware row filter. The filter it needs is
command-owned, the fold produces the exact command that already answers, and
sysctl is an IN-PROCESS plugin whose package is compiled into the daemon, so an
in-tree companion can call `RegisterPipeFilters` for it the way
`internal/component/bgp/plugins/cmd/rib` does for the RIB plugin.

Two conditions come with it. The parent path must itself be a dispatchable
command, and for `show metrics` and `show dns cache` it is not: both are bare
grammar containers, so the operator would type a non-command on the left of the
pipe. And every sibling under the parent needs an empty filter declaration, or
it inherits the filter by longest prefix. That is 4.6.

The families where mechanism 3 is genuinely blocked are the ones behind an
in-tree forwarder that REFUSES arguments: OSPF, IS-IS, LDP and RSVP-TE. There
the fold produces a valid string and `forwardTo*` rejects it before the plugin
sees it.

What the fold does NOT give for free is agreement between the two answers. The
scoped command must satisfy E4, or `show sysctl | key <k>` answers with
`description`, `type`, `min` and `max` that `show sysctl` never writes, and
drops `persistent` that it does (`(*store).showEntries` and `(*store).describeKey`,
`internal/component/sysctl/sysctl.go`). That is a payload change from 4.1, not a
mechanism gap.

### 4.3 Needs a new generic operator

Three, and two of them are already parked.

| Operator | What it would do | Status |
|----------|------------------|--------|
| a field-aware row predicate, `\| where <field> <op> <value>` or the narrow `\| errors` | keep the rows whose named field holds a given value. `applyMatch` is a case-insensitive SUBSTRING filter over rendered TEXT and matches a value in any column | NOT parked anywhere. This audit is the first to name it |
| row ordering | sort rows by a field's value. `knownPipeOps` holds no operator that touches row order | parked, `plan/future/spec-cli-pipe-column-modifiers.md` |
| exclusion, `\| hide <field>...` | name the columns to DROP instead of the seventeen to keep | parked, same file |

Only the FIRST one blocks a presentation this audit proposes, and only one
presentation: `| errors`, meaning "the rows that are not healthy". Everything
else on the presentation axis is field selection, which `| display` already
does.

`ai/rules/cli.md` makes that operator cheap to specify when somebody writes it:
"A row's state MUST be a FIELD or a COLUMN. It MUST NOT be a character glued to
another field's value." So every family that has a health question already has a
field to test.

A fourth item is adjacent and worth naming: an operator cannot define an alias
of their own (parked, `plan/future/spec-cli-operator-defined-aliases.md`). Every
alias in this document has to be written in Go by Ze.

### 4.4 Needs a registration surface a plugin does not have

`sdk.PipeDecl` carries `command`, `name`, `description` and `expansion`
(`pkg/plugin/rpc/types.go`). That is an ALIAS and nothing else. A plugin cannot
declare:

- a pipe FILTER, so no plugin-owned command can have a scope pipe.
- a COLUMN ORDER, so a plugin-owned command renders alphabetically and offers no
  `| display` completion.

This is the real blocker on the scope axis, and it is wide. Every family whose
commands come from an out-of-process plugin is inside it. `show bgp rpki`
declared an alias and could go no further for exactly this reason.

The RPKI conversion also measured the column cost: a converted plugin command
and its pipe form sort differently, and the pipe form reads better, because
`display` orders what it names.

### 4.5 Needs the client to learn the daemon's alias table

A plugin's alias resolves in the process that PARSES the chain, and for two
surfaces that process is the client:

| Surface | Parses | A plugin's alias works |
|---------|--------|------------------------|
| `ze cli -c "<command>"`, any SSH exec channel, the TUI | the daemon | yes |
| `ze cli` with no command argument | the client | no |
| a streaming monitor command | the client | no |

On the two client rows the operator reads `pipe error: unknown pipe operator`
and Tab offers nothing. The repair is a wire surface carrying the daemon's alias
table to the client at session start, and it is NOT built
(`docs/architecture/api/commands.md`, "Where an alias resolves, and where it does
not").

Every presentation this audit proposes for a plugin-owned family inherits that
gap.

### 4.6 One barrier is missing from one registry

`cmdBgpChildren` (`internal/component/bgp/plugins/cmd/peer/peer.go`) registers
an empty ALIAS set and an empty COLUMN order on all ten direct children of
`show bgp`, so neither reaches an answer that cannot carry it.

There is no equivalent for the FILTER registry. `RegisterPipeFilters` is called
in one file, `internal/component/bgp/plugins/cmd/rib/rib.go`, and `show bgp`
itself declares nothing. The moment a filter is registered on `show bgp`, every
child that declares no filter set of its own inherits it by longest prefix, and
`show bgp rpki | peer 10.0.0.1` would parse, fold, and reach the RPKI handler as
two arguments it does not understand. The barrier has to be written into all
three registries at once.

A plugin does not have this problem. `aliasBarriers`
(`internal/component/command/alias.go`) derives the barrier from the plugin's
own command list, so a plugin author writes nothing. The in-tree form is written
by hand, and by hand is where it was forgotten.

## 5. What must NOT be a pipe

### 5.1 The test

The existing audit's rule stands and is not weakened here:

> A pipe reshapes what the command already returned. It cannot fetch and it
> cannot drive a lookup.

Mechanism 3 makes it necessary to say WHY that rule survives, because a folded
filter does reach the handler and the handler can do anything, lookups
included. The rule is not about the mechanism. It is about the answer:

> **Could the command have produced this by writing FEWER ROWS or FEWER FIELDS
> of what it already built? If yes, it is a pipe. If the command would have to
> ask a different source, it is a subcommand.**

A folded filter that returns a row carrying a field the unfiltered answer never
carries has failed the test even though the mechanism allowed it. That failure
has a fix and the fix is E4: make the wide command emit the field. It is not a
reason to widen the rule.

### 5.2 The classes that fail it

| Class | Examples | Why |
|-------|----------|-----|
| Transforms operator input, not daemon state | `show bgp decode <hex>`, `show bgp encode`, `show dns lookup <name>`, `show route lookup`, `show bgp irr check <peer> <prefix>` (`internal/component/bgp/plugins/filter_irr/command.go`), every `resolve` leaf, every `validate` leaf | the input is not a row of any parent. There is nothing to filter |
| Mutates behind a `show` verb | `show capture raw start\|stop`, `show vpp trace start`, `show vpp trace clear` | these are not reads at all. `ai/rules/cli.md` already states the test they fail |
| Carries a field from a different source | `show interface rate` (a two-sample delta from `globalTracker`), `show bgp rpki cache` (six protocol fields `status` never writes), `show ospf ipv6 interface detail` (the ISM view, which shares NO field with its parent) | the parent cannot write the field, so no selection reaches it |
| Siblings that share no field | `show ldp neighbor` against `show ldp binding`, the four `show bmp` fetches, the three `show fib` backends | a pipe implies one payload. These are several |
| Would need the pipe to name a new fetch | `show fib kernel` against `vpp` against `p4`, `show vpp runtime` against `show vpp trace` | the word after the pipe would be choosing a data source, which is the SUBJECT axis |

One more class deserves its own line because it looks like a presentation and is
not. **A word that means "more" cannot be a presentation pipe.** The bare
command is the full view by definition, so `| detail`, `| all`, and `| extended`
can only mean "what you already have". Where they mean something else today, the
parent is emitting less than it should.

---

## 6. Corrections to what this audit was given, and to the earlier one

Five statements this audit was asked to work from are wrong or incomplete. Each
one changes what the work costs.

| Statement | What the source says |
|-----------|----------------------|
| "The pipe layer cannot compute" | True of the generic operators and of aliases. FALSE of a command-owned filter, which `foldFilters` rewrites into command arguments so the HANDLER computes. `show bgp rib \| histogram` and `\| graph` are two computed answers already shipping through that path |
| "`show sysctl key <k>` is BLOCKED on a field-aware row filter which does not exist" | The filter it needs is command-owned, not generic. The fold produces `show sysctl key <k>`, which is the registered command that already answers. The same holds for `show metrics \| name <n>` and `show dns cache \| record <n>`. What is genuinely blocked is a family behind a forwarder that refuses arguments, which is OSPF, IS-IS, LDP and RSVP-TE |
| "The 22 group B leaves are BLOCKED until a field-aware row filter exists" | They are blocked on a per-command filter registration and, for some, a handler that reads the folded argument. That is real work, and it is not a missing mechanism. A GENERIC row predicate is still missing, and exactly one presentation in this audit needs it: `\| errors` |
| The earlier audit's open gap, "nine `show ospf` leaves could each still be a PIPE" | Seven are SUBCOMMAND on evidence: each reads engine state `processSummary` never touches. One, `ipv6 database extended`, is a row filter of a SIBLING. One, `ipv6 instance`, is a near-duplicate of its parent that one field keeps from being a pipe |
| The earlier audit's count of pipe registrations | Still accurate. Two commands carry a column order, one carries aliases, two carry filters. Nothing has been added since |

Two further findings contradict nothing but change the picture.

**`| count` answers the wrong number on the shape this design asks for.**
`countItems` unwraps a map of one key and counts the array inside. A map of two
or more keys answers the KEY COUNT. So `show bgp | count` answers 6.
`truncateItems` has the same rule, so `| first N` and `| last N` are inert on
the same shapes. At least ten commands already carry a `count` key beside their
rows and have lost all three operators without anybody noticing.

**The pipe spelling already renders better than the path spelling.**
`ColumnsForCommand` resolves on the command as TYPED, before the fold. So
`show bgp | peer 10.0.0.1` keeps the column order declared on `show bgp`, and
`show bgp peer 10.0.0.1` resolves the deliberate empty declaration on
`show bgp peer` and renders alphabetically.

## 7. Method and limits

The population is the same 465 command paths the earlier audit derived, from the
same three registries: YANG nodes carrying `ze:command`, `[]sdk.CommandDecl`
literals outside `_test.go`, and `registry.MustRegisterLocal` path literals.
`./le command-list` was not used. It walks `AllBuiltinRPCs` plus the
streaming prefixes and reports neither plugin-registered commands nor local
ones.

Four family sweeps ran in parallel, each required to read the producing function
before naming a field. Every field name quoted in this document came from a
handler, a struct tag, or a `textbuf` write, never from a YANG description.

Four limits.

- **The presentation vocabulary is a proposal, not a finding.** Section 1.6 is
  design. Sections 2 and 3 are evidence. The one decision the vocabulary needs
  from the owner is what `| summary` means (section 1.7).
- **Row shapes were read, row VALUES were not.** No command was run. A claim
  about what a handler writes is a claim about the code, and a payload that
  varies with runtime state (`show capture`, whose top-level keys depend on
  build tags and on whether the reactor is attached) was read as written.
- **Three claims are marked unverified in place** and are repeated here:
  whether `trafficstat.Snapshot` carries per-interface attribution for its top
  talkers, whether `handleSession` in
  `internal/component/l2tp/pppoe/cmd/pppoe.go` receives the `id` selector it
  reads from `args`, and the VPP `RouteLookup` key set.
- **`show ospf` carries 48 of the 279 candidate leaves**, so anything counted
  across families is weighted by OSPF, as it was in the earlier audit.
