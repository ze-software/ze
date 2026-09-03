# The reject-asn Filter Plugin

A peer that leaks its transit sends you paths that run through its upstream.
Ze's answer until now was the max-prefix limit, which stops the leak by dropping
the session, so the peer's legitimate routes go with the leaked ones and an
operator has to reset it by hand.

`bgp-filter-path-asn` is the cheaper check RFC 7454 Section 9 recommends. An
operator lists the ASNs that must not be reached through a peer. A route whose
AS_PATH carries one of them is dropped and the session stays up.

<!-- source: internal/component/bgp/plugins/filter_path_asn/filter_path_asn.go -- plugin entry point, configure, senderOf, handleFilterUpdate -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/register.go -- registration and the reject-asn filter type -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/config.go -- position, positionSet, positionsByKey, parseRejectASNLists -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/match.go -- senderASN, matchPositions, matchPattern, pathShape, positionAt -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn.yang -- the config schema -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/curated.go -- curatedNetwork, curatedTransitFree, curatedLookup -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/register_completion.go -- curatedASNValues, curatedASNHelp -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/command.go -- handleCommand, showRejectASN, showRejectASNName, showKnownTransitFree, countAttachments -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/register_command.go -- the wire methods, the forwarders and the pipe shapes -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/metrics.go -- recordReject, rejectSlot -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn-cmd.yang -- the command tree -->

The interop proof is the scenario `bgp-path-asn-leak-frr`
(`test/interop/scenarios/bgp-path-asn-leak-frr/`). FRR announces two prefixes on
one session that differ only in their AS_PATH, and ze must drop the one reached
through the listed ASN, install the other, and never re-advertise the dropped
one to BIRD downstream. The second half is what stops the scenario passing
against a ze that rejects everything.

## The config

```
bgp {
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 174 3356 ]
        }
    }
    peer peer-a {
        filter {
            import [ NO-TRANSIT ]
            export [ NO-TRANSIT ]
        }
    }
}
```

The chain names the list by its bare name, JunOS style. `reject-asn:NAME` and
`bgp-filter-path-asn:NAME` also resolve, for a tree where two filter types share
a name.

## The position vocabulary

A token of the flattened path holds a SET of properties, not one label.

Three of them PARTITION the path, so every token holds exactly one:

| Primitive | What occupies it |
|-----------|------------------|
| `direct` | The leading run of tokens that equal the SENDING peer's ASN, prepends collapsed. A run of four prepends is one direct position |
| `origin` | The last token, unless the leading run reaches it |
| `transit` | Every token between the two |

The leading run wins over the origin. A path of one ASN, sent by the peer that
owns it, is therefore direct and not the origin.

A fourth property CUTS ACROSS that partition. `nth n` says the token sits at
collapsed position n, counted from us and 1-based. In `[P A O]` the second ASN
is both `nth 2` and transit; in `[P A]` it is both `nth 2` and origin. That is
why a token cannot be reduced to one label, and why the matcher tests both
families for each token.

An operator writes one of seven keywords, never a primitive.

| Keyword | Matches where | An operator writes it to say |
|---------|---------------|------------------------------|
| `direct` | direct | The peer itself must not be this ASN |
| `indirect` | transit, origin | The route must not be REACHED through this ASN. The peer being the ASN is fine |
| `transit` | transit | The route must not pass through this ASN on its way |
| `origin` | origin | This ASN must not originate the route |
| `anywhere` | direct, transit, origin | The ASN must not appear anywhere |
| `nth <n>` | collapsed position n | This ASN must not be n hops away from us |
| `regex` | nothing | The values are patterns, not ASNs. See below |

`indirect` is the everyday keyword, because it says the operator's own sentence:
do not give me anything you reached through a transit provider, and peering with
that provider directly is still fine.

`direct` is RELATIONAL and `nth 1` is POSITIONAL, which is why both earn a place.
In `[3356 65001]` learned from AS65001, 3356 is at `nth 1` and is NOT `direct`.

The first five rows are declared once, as `positionsByKey`
(`internal/component/bgp/plugins/filter_path_asn/config.go`), and
`TestPositionKeyExpansion` asserts the table itself. An expansion that changed
silently would change every list in every config at once, and no test of the
matcher would name it. `nth` is not in that table: its position is a NUMBER, so
a fixed bitmask cannot hold it, and `parseOneList` reads it on its own.

## A run collapses, wherever it sits

`nth` counts RUNS rather than tokens. A run of consecutive identical ASNs
advances the count once, anywhere in the path, not only in the leading run.

The reason is the peer. Counting tokens would let AS3491 move itself off your
`nth 2` rule by sending `[65001 65001 3491]`, so the peer would choose which of
your rules fires. Counting runs takes that choice back.

## `direct` is the sender, not index zero

This is the one definition worth reading twice, because the wrong reading passes
almost every test and accepts the exact leak the filter exists to catch.

Take the path `[3356 65001]` and the list `indirect [ 3356 ];`.

| Learned from | 3356 is | The route is |
|--------------|---------|--------------|
| AS3356 | direct, because the sender's own leading run is direct | ACCEPTED. `indirect` excludes it |
| AS65001 | transit, because AS65001 sent it and 3356 is not AS65001 | REJECTED. AS65001 handed us its upstream |

3356 sits at index zero in both rows. The index decides nothing; the sender
does. RFC 7454 Section 9 names the same distinction for a route server, where
the first ASN is a member and the peer is the server.

`pathShape` measures the run and `positionAt` judges each index against it
(`internal/component/bgp/plugins/filter_path_asn/match.go`).

## The two arms of a list

A list is one reject set with two arms, and a route matching either is rejected.
There is no ordering between them.

| Arm | Subject | Cost |
|-----|---------|------|
| The five position leaf-lists and `nth` | Each token of the path, judged at the properties it holds | Zero allocation, linear in the path |
| The `regex` leaf-list | The whole space-separated path string | RE2, linear in the subject and no backtracking |

The zero-allocation guarantee covers the position arm. A list carrying patterns
pays RE2's cost for them, which is stated rather than hidden.

A pattern sees the same flattened string every other reader of the format sees,
so it reaches an AS_SET member and no bracket appears in the subject. That is
why an anchored pattern such as `^3356 174 ` keeps working at any path length.

## What the parse refuses

The SCHEMA makes most of the refusals. Every ASN leaf-list is `type uint32`,
`regex` is `type string` and an `nth` index is a `uint8` bounded 1..255, so a
name written where an ASN belongs, an ASN past 4294967295, an index outside the
range and a keyword nobody declared are each refused by
`config.ParseTreeWithYANG` before the plugin sees the config.

Three refusals are left to the plugin's own parse (`parseRejectASNLists`), each
naming the list and what is wrong.

| Config | Refused because |
|--------|-----------------|
| A list that names nothing: every leaf-list absent or empty and no `nth` entry | It rejects nothing while reading like a safety filter, and no schema can see it: an absent leaf-list has no node to walk. An empty leaf-list and an absent one are one state by the time the config is delivered |
| An `nth` entry with no ASN | The same, one level down |
| A pattern that does not compile, or one longer than 512 characters | A broken pattern that reached the hot path would be a rule the operator wrote and Ze never ran |

## The decisions

**The type name states the ACTION.** `as-path-list` and `prefix-list` are
data-shaped names because their entries carry `action accept|reject`. This
filter only ever rejects, so it follows `remove-private-as` instead. It names
the ASN rather than the path, because the ASN is what an operator lists and the
leaf-list it is written under already says where in the path it matters.

**A list is an unordered reject SET.** There is no ordering and no
first-match-wins, which is what keeps the type different from `as-path-list`.
Both stay, and neither subsumes the other.

**The position is a KEYWORD, with the ASNs beneath it.** One set of ASNs shares
one position far more often than one ASN needs its own position, so the set is
written once, as an ordinary bracketed leaf-list. Six of the seven keywords are
plain leaf-lists with no wrapper. `nth` alone takes a number, which a leaf-list
name cannot carry, so it alone is a keyed list.

The keyword names also let each one carry its own YANG TYPE, which an earlier
shape could not. The ASN leaf-lists are `uint32` and `regex` is a bounded string,
so the schema refuses a name where an ASN belongs. A single `asn` leaf-list under
a position key had to be a union of both, and a YANG union cannot say which arm
belongs under which key.

**`regex` is a keyword beside the position keywords rather than a modifier on
one.**
Its values are patterns, so no position applies to them. One leaf, one shape: an
operator who needs a shape question writes it in the same list rather than
attaching a second filter type.

**The plugin declares ZERO filters at Stage 1.** Filter names come from config,
so the registration declares the filter TYPE through `FilterTypes`, and
`BuildFilterRegistry` (`internal/component/bgp/config/filter_registry.go`)
discovers each instance from the `ze:filter` marker on the YANG list. This is
the route `filter_aspath` takes, and it is not the `filterapi.Register` route,
which is for a filter whose name is known at compile time.

**The plugin declares the config obligation its filter type discharges.** A
peer that declares an RFC 9234 role must name a filter that can refuse a path
through a transit provider, and the rule that enforces that
(`validateLeakFilterObligations`, `internal/component/bgp/config/peers.go`) must
not spell `reject-asn`: it asks the registry which filter types declare
`filterapi.ObligationTransitLeak`. The type name therefore lives in this plugin
alone, and a second implementation of the same check qualifies by declaring the
same obligation. A `grep` for `reject-asn` under
`internal/component/bgp/config/` returns nothing outside the tests.

**Ze ships NO ASN set.** Curated data feeds completion and a `show` annotation
column, never a decision. A network that leaves the Tier-1 club would otherwise
keep dropping routes with nothing in the config file to point at.

## The curated table

`curated.go` holds 15 transit-free networks as a Go literal: an ASN, a network
name, and, for one entry, a note. Its header names both sources and carries a
`Curated:` date rather than a `Generated:` date, because no authority publishes
this set in a machine-readable form. There is no refresh verb, and that absence
is a decision rather than unfinished work. `show bgp reject-asn known
transit-free` prints the date, which is the only staleness signal there is.

Three surfaces read the table and no others: the completion on the `asn`
leaf-list, the annotation column of `show bgp reject-asn`, and the pasteable
block `show bgp reject-asn known transit-free` prints. No config path, no filter
path and no commit rule reads it.
`TestCuratedTableDecidesNothing` reads `config.go`, `match.go` and
`filter_path_asn.go` and fails if any of them names it.

**AS174 is recorded as contested, not resolved.** The undated source omits it;
the dated one records it added and removed repeatedly, over a declared
paid-peering arrangement and a gap in its IPv6 peering. The table carries the
disagreement and the completion dropdown prints the word `contested`. Choosing a
side quietly would put an editorial judgement on a surface with no way to show
its working, and the table decides nothing, so there is nothing to decide.

## Completion is a suggestion, never a constraint

Each of the five position leaf-lists, and `nth`'s own `asn` leaf-list, carries
`ze:validate "transit-asn"`.
`register_completion.go` declares that name at `init()` through
`yang.RegisterSuggestion`, passing `curatedASNValues` (which ASNs to offer) and
`curatedASNHelp` (which network each one is). It declares no `ValidateFn`, so
the leaf refuses nothing its `uint32` type already admits: an ASN the table has
never heard of is accepted with no warning, and an operator who ignores the
dropdown loses nothing.

`RegisterSuggestion` exists because `RegisterValidators`
(`internal/component/config/validators_register.go`) is a central list: config
cannot import a plugin to write a row in it, and the plugin cannot reach it. The
validator it declares carries a nil `ValidateFn`, and `applyCustomValidators`
skips such an entry
(`docs/architecture/config/yang-config-design.md`, Completion-Only Validators).

`TestCompletionIsNeverAConstraint` holds the line at three levels: the registry
entry has no `ValidateFn`, the editor's value gate accepts an unlisted ASN, and
the full module walk reports nothing for one.

The seven keywords complete as ordinary children of the `reject-asn` list, each
with the help text its YANG `description` declares, so an operator meets the
vocabulary by pressing Tab inside the list.

## What an operator can ask

Three commands, owned by the plugin so removing it removes them with the
handlers and the config schema. Each answers structured data, so `| json`,
`| yaml` and `| table` all render it.

| Command | Answers |
|---------|---------|
| `show bgp reject-asn` | every list: each ASN once, with the effective position set, the curated annotation, and the peers that name the list on import and on export |
| `show bgp reject-asn name <name>` | the same record for one list |
| `show bgp reject-asn known transit-free` | the curated set as a pasteable `asn [ ... ];` block, with the sources and the curated date as comments |

**The listing prints the effective set, not the blocks.** An operator who wrote
3356 under `indirect` and under `direct` has ONE rule, and the filter acts on the
union. Printing the blocks back would make the reader compute it.

**An ASN the curated table does not know prints with an empty annotation.** It
is never omitted and never guessed. The operator has a policy decision to make
about that ASN, and an invented network name would be an input to it.

**The peer counts are what tell a working list from an inert one.** A list no
chain names refuses nothing while reading like a safety filter, which is the
failure the parse already refuses for an empty list. The parse cannot see peers,
so `countAttachments` reads the chains at every config level, through all three
spellings of a reference, and the count surfaces it.

**`known transit-free` is the only route the well-known set takes into a
config.** Ze acts on no ASN it was not configured with, so the authoring aid is
what closes the gap between "Ze knows these ASNs" and "Ze ships a list that
decides". After the paste the config holds NUMBERS, and a later change to the
curated table cannot alter what that config does.
`TestKnownTransitFreePrintsPasteableBlock` feeds the printed block back through
the config parser, so a rendering change that stops it being pasteable is a red
test rather than an operator's problem.

**The value after `name` arrives as a SELECTOR.** `name` is both the last token
of the command path and the leaf under it, so `matchCommandTokens`
(`internal/component/plugin/server/command.go`) binds the value to that leaf and
hands the handler an empty args slice. `forwardShowRejectASNName` reads
`ctx.Selector("name")` and passes it to the plugin as its first argument.

## The counter

One metric: `ze_filter_path_asn_rejects_total`, a CounterVec labeled
`direction` (import, export), `position` (direct, transit, origin, nth, regex,
unspecified) and `reason` (listed-asn, unknown-list, unconfigured).

A dropped route is otherwise invisible: the reject log sits at Info and says
nothing about rate. The counter answers how many and which rule; the log line
answers which peer and which ASN. Peer identity stays out of the labels, because
a peer address there would grow the series count with the session count.

Every label value is a compile-time constant, so the series count is fixed at
two directions times seven slots, whatever an operator configures. Every child
is resolved once in `buildMetrics` and every series exists at 0 from startup, so
the reject path costs one atomic load and an increment, and an alert on a rate
does not wait for a series to appear.

`rejectSlot` is numerically equal to the `position` constants for its first four
values, so a hit converts to its slot with no lookup table. The `unspecified`
slot is never a real reject: `positionAt` answers direct, transit or origin
for every index, so a series that moves off zero says `positionAt` returned the
value that is not a place.

A pattern match reads `reason="listed-asn"` beside `position="regex"`. Both arms
of a list are the operator's listing, and the position label is what separates
them.

## Constraints

**Direction lives on the config attachment point, never on the filter.** The
engine supplies the direction string at each chain
(`(*Reactor).runIngressPolicyChain` and `runEgressPolicyChainASN4`,
`internal/component/bgp/reactor/filter_ordered.go`), so one list attaches to
import and to export independently.

**`FilterUpdateInput.PeerAS` means the SENDER on import and the DESTINATION on
export.** The leading-run exemption has an input to read on import. On export
nothing is direct, so `indirect` covers the whole path, and that asymmetry is a
consequence of the seam rather than a rule written into the list.

`senderOf` is the one place the direction is read
(`internal/component/bgp/plugins/filter_path_asn/filter_path_asn.go`). It hands
the matcher a sender that is either known or not, and the matcher never sees a
direction. The flag is explicit rather than a zero ASN, because AS0 is a value a
peer can put in a path and a zero that behaved like "no sender" would be a guard
nobody named.

**The as-path text flattens every segment type.** `(*ASPath).AppendText`
(`internal/core/bgp/attribute/text_append.go`) writes AS_SEQUENCE, AS_SET,
AS_CONFED_SEQUENCE and AS_CONFED_SET into one space-separated list with no
marker. A listed ASN inside an AS_SET is caught by a flat scan, which is what
this filter wants. A single-ASN path renders unbracketed (`as-path 65001`), and
an empty path emits no `as-path` token at all.

**The export filter runs BEFORE the local AS is prepended**
(`internal/component/bgp/reactor/reactor_api_forward.go`), so the text carries
the path as stored.

**A leaf TYPE is enforced by the parse; a leaf BOUND is enforced only by the
module walk.** The `uint32` on each ASN leaf, the `uint8` range on an `nth`
index, and the unknown-keyword refusal all fire in `config.ParseTreeWithYANG`, so
an operator meets them at load. The
512-character `length` on `regex` is reported by `ValidateTreeAllModules`, and
`bgp` is deliberately outside the `validatedSections` list that
`ValidateCustomSections` walks
(`internal/component/config/validate_sections.go`). That bound is therefore owed
by the plugin's own config parse, which is the table under "What the parse
refuses" above.
