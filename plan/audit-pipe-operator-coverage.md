# Audit: pipe operator coverage, and what a tool author can learn without reading Go

| Field | Value |
|-------|-------|
| Date | 2026-08-23 |
| Question | do we expose all the modifiers for all commands, is it documented, and can a tool discover it |
| Deliverable | what is measured, what is wrong, and what must be built. No implementation plan |
| Related | `plan/audit-presentation-pipes.md` (which views should exist), `plan/audit-command-pipe-vs-subcommand.md` (the 465-command population) |

## The answer

| Question | Answer |
|----------|--------|
| Do we expose all the modifiers for all commands? | No. 38 commands reach no pipe layer on any surface, 8 more lose it on the `ze <verb>` form, and `\| match` filters nothing on the document path unless a format operator is typed first |
| Is it in the wiki? | Yes, generated, and wrong on every one of its 381 command entries: it names ten operators of sixteen |
| Is it on the website? | No. The website says `Global pipes: yes` per command and names no operator. The one accurate page is not in the website manifest |
| Can a tool discover it? | Only through an authenticated web completion endpoint. Every CLI and API surface either names ten of sixteen as prose or carries a boolean |
| Is the rule right? | No. "Every command supports every operator" cannot be met and cannot be gated. It has to become two rules, one per operator class |

## What this audit adds

The two earlier audits asked which VIEWS a command should offer. This one asks
whether the operator language Ze already has reaches every command, and whether
anybody outside the Go source can find out. It cites their tables rather than
repeating them.

The frame the owner set, which the findings are measured against:

1. Operators split into two classes. GLOBAL operations act on the answer
   whatever it holds and are owed by every command. DATA-DEPENDENT operations
   are owed only where the data supports them.
2. The class an operator falls into must be DERIVED from the shape of the
   answer, not maintained per command.
3. The published page must state, per command, what it supports, and it must be
   GENERATED from that derivation rather than written by hand.

So `ai/rules/cli.md`, "Every command that produces output MUST support all pipe
operators", is not merely unmet. It is the wrong rule, and section 6 says why
with evidence.

---

## 1. The operator set is 16, and five names in circulation are not operators at all

`knownPipeOps` (`internal/component/command/pipe.go`) holds sixteen names:
`match`, `count`, `no-more`, `table`, `text`, `yaml`, `raw`, `json`, `resolve`,
`origin`, `ndjson`, `log`, `first`, `last`, `display`, `fill`.

`established` is not one of them. It is the ARGUMENT in the documented example
`show bgp peer list | match established`, and reading the example as a list of
operators is how it entered the brief for this audit. Run it:

```
$ echo '{"peers":[...]}' | ze pipe established
error: unknown pipe operator: established
```

No `save`, `write`, `tee` or `file` operator exists either. The owner named
saving among the global operations he expects, so it is recorded as MISSING
rather than as a misremembering:

```
$ echo '{"peers":[...]}' | ze pipe save /tmp/x
error: unknown pipe operator: save /tmp/x
$ echo '{"peers":[...]}' | ze pipe tee /tmp/x
error: unknown pipe operator: tee /tmp/x
```

Nothing in the CLI writes an answer to a file. A tool author redirects the
process's stdout from the shell, which works for `ze cli -c` and for an SSH exec
channel, and is unavailable inside an interactive session, where the answer is
drawn to a terminal and never reaches a pipe.

### 1.1 The set is hand-copied in at least five places and no two agree

`knownPipeOps` is package-private and has no exported reader. Every surface that
names the operators carries its own literal.

| Surface | Producer | Names | Missing |
|---------|----------|-------|---------|
| the parser | `knownPipeOps`, `pipe.go` | 16 | none. This is the truth |
| Tab completion | `PipeOperators`, `completer.go` | 16 | none, by hand, with no test tying it to the parser |
| `ze help command --verbose` | `printCommandVerbose`, `cmd/ze/help_command.go` | 10 | `raw`, `log`, `first`, `last`, `display`, `fill` |
| the generated wiki page | `gen_wiki_commands.py` | 10 | the same six |
| `ze pipe help` | `pipeUsage`, `cmd/ze/ze_core_pipe.go` | 10 | `raw`, `origin`, `log`, `no-more`, `display`, `fill` |
| `ze help ai --json` | the `pipe` root command's `Meta.Subs`, `ze_core_dispatch.go` | 10 | as `ze pipe help` |
| `ai/rules/cli.md`, "The Pipe Operators" | the rule itself | 11 | `raw`, `first`, `last`, `display`, `fill` |
| `multipleFormatsError` | `pipe.go` | 6 formats | not a full list, and correct for its purpose |

`display` and `fill` appear in NONE of the six operator lists a user or a tool
can reach. They are the two newest operators and the two a tool author most
needs, because they are how a caller asks for a stable field set.

`completer_test.go` asserts that `CompletePipe` returns `len(PipeOperators)`
suggestions, which is a self-comparison. Nothing compares `PipeOperators`
against `knownPipeOps`, so the next operator added to the parser is silently
absent from completion.

---

## 2. Three paths run the chain, and the same operator means different things on each

Which operators work is decided by which path a command's answer takes, not by
the command. There are three.

| Path | Reached when | Implementation | `\| count` counts | `\| match` filters |
|------|--------------|----------------|-------------------|--------------------|
| DOCUMENT | the handler answered with a built payload (`plugin.Map`, `Slice`, a struct) | `ApplyPipes`, `pipe.go` | `countItems`: an array's length; a map of ONE key unwraps to its value; a map of two or more answers the number of KEYS | `applyMatch` over LINES of the string in hand |
| RECORD | the handler answered with a `plugin.Records` row generator | `ApplyPipesRecords`, `pipe_records.go` | rows, correctly | rows, correctly, one at a time |
| NONE | the client's local-handler registry answered first | there is no pipe layer | nothing | nothing |

The document path and the record path are the same operator names over
different data. A tool author who learns `| count` on one command and uses it on
another gets a different question answered, and nothing tells them which path
they are on.

Three handlers produce a record generator today: `streamedPluginResponse`
(`internal/component/plugin/server/command.go`), which covers every
out-of-process plugin answer that arrived as a streamed answer;
`handleSystemCommandList` (`server/system.go`); and `show bgp rib best` with no
terminal (`bestPipeline`, `internal/component/bgp/plugins/rib/rib_pipeline_best.go`).
Everything else in the tree is on the document path.

### 2.1 The shape metadata the owner wants is already on the wire, and nothing reads it

Every answer head declares an item type (`docs/architecture/api/wire-format.md`,
`AppendAnswerHead` in `pkg/plugin/rpc/message.go`): `doc` for one document,
`map` for a self-describing record per row, `tab` for a positional row read
against the head's column names. That is exactly the distinction the derivation
needs.

Two facts stop it being the derivation today, and both have to be fixed by
whatever spec follows.

**Nothing gates on it.** `ApplyPipes`, `ApplyPipesRecords` and `ValidatePipes`
never read the item type. `ValidatePipes` checks syntax only: an argument's
presence, a number's sign, one format operator. So an operator that cannot
apply to a shape is not refused. It runs and answers something. `show bgp | count`
answering 6 is that failure, and 6 is the number of top-level keys
`handleBgpSummary` writes.

**The item type describes the ANSWER, not the command.** `RenderRecords`
(`render_records.go`) sets `doc` for a walk that ends within
`rpc.AnswerBufferThreshold` (256 records) and `map` or `tab` for one that passes
it. So the same command answers `doc` with 200 rows and `tab` with 300. A tool
author cannot use the tag as a contract, because it flips with data volume.

The derivation therefore needs a shape declared by the COMMAND, which the head's
item type then reports for the answer in hand. 9.2 says what that costs.

### 2.2 The gap that blocks two operators: the head names columns, not their types

`checkAnswerType` (`pkg/plugin/rpc/message.go`) refuses a `tab` head that names
no columns and a `doc` or `map` head that names some, so the column NAMES are a
real, enforced part of the contract. Their TYPES are not declared anywhere.

`resolve` and `origin` need to know that a field holds an IP address.
`applyResolve` and `applyOrigin` walk every value and guess by parsing it, so
they decorate anything that parses as an address, including a field that is an
address by coincidence. No declaration exists that would let the derivation say
"this command's answer carries no address, so `| origin` is refused here". That
is the one place the owner's model needs metadata that does not exist.

---

## 3. Coverage, measured

Population is the 465 command paths `plan/audit-command-pipe-vs-subcommand.md`
derived from the three registries. `ze help command --json` reports 395 of them,
so the catalog a tool author reads is itself short of the surface; section 8
returns to that.

### 3.1 Commands missing a GLOBAL operator

This is the number that matters, because a format operator is what a tool author
depends on. The local-handler registry holds 46 production paths
(`MustRegisterLocal` and `MustRegisterLocalMeta` outside `_test.go`). 27 of them
were run over `ze cli -c`, which is the only surface that could pipe them;
section 10 says which 19 were not and why the reading covers them.

| Class | Count | Which |
|-------|-------|-------|
| No wire method at all, so `ze help command --json` reports them honestly as `mode: offline` | 18 | `clear debug`, `delete debug module`, `delete debug profile name`, `doctor`, `explain`, `generate wireguard keypair`, `help ai`, `help command`, `set debug active name`, `set debug module`, `set debug profile name`, `set debug timeout`, `show config graph`, `show debug profile`, `skills`, `support`, `update serve`, `validate config` |
| A wire method in YANG that NO daemon handler implements, so the catalog reports `global-pipes: true` and the daemon answers `unknown command` | 20 | `show env get\|list\|registered`, `show schema events\|handlers\|list\|methods\|protocol`, `show data cat\|ls\|registered`, `show yang completion\|doc\|tree`, `show config cat\|diff\|dump\|fmt\|history\|ls` |
| Genuinely dual: a local handler AND a working daemon handler | 6 measured, 2 unmeasured | `show interface`, `show version`, `show bgp decode`, `show bgp encode`, `show ping`, `show traceroute`. `monitor ping` and `monitor traceroute` carry streaming wire methods and were not run; the exec channel routes a streaming command BEFORE it splits the chain, so their pipe handling is a separate question |

**38 commands reach no pipe layer on any surface.** For the first 18 there is no
daemon RPC to reach. For the 20 the RPC name exists in YANG and nothing serves
it. Both were measured:

```
$ ze cli -c "show env list | json"
error: unknown command
$ ze cli -c "show config dump | json"
error: unknown command
$ ze cli -c "show yang tree | json"
error: unknown command
```

Five whole families are in the second class: env, schema, data, yang and config.
Their register files call only `registry.MustRegisterLocal`
(`internal/plugins/env/register.go`,
`internal/component/config/{schema,storage,yang}/cli/register.go`,
`internal/component/config/cli/register.go`), so the handler prints text to
stdout and returns an exit code and `RunCommand` never reaches
`ProcessPipes*`. This corrects `plan/audit-presentation-pipes.md` section 3.8 in
one direction: those families are not merely outside the pipe layer, they are
ADVERTISED as inside it.

### 3.2 A dual command answers differently depending on how it is invoked

`matchLocalHandler` runs before the daemon dispatch in `RunCommand`, and
`registry.LookupLocal`'s shadow rule refuses a local handler only when a LONGER
path is declared. For the exact path, the local handler always wins:

```
ze show version                    -> the local handler, text, no pipes
ze cli -c "show version | table"   -> the daemon RPC, piped
┌─────────┬────────────────────────┐
│ version │ ze dev (built unknown) │
└─────────┴────────────────────────┘
```

`show interface` is the one that will hurt most: it is among the most typed
commands in the tree, and `ze show interface` takes the local path.

One more shape sits underneath. `ze cli -c "show debug profile | json"` answers,
but with `show debug`'s payload: the daemon's matcher took `profile` as an
argument to the declared `show debug`. So a command absent from the daemon can
still return something, and it is a different command's answer.

### 3.3 Commands missing a DATA-DEPENDENT operator, judged by shape

There is no per-command opt-out for a generic operator. Once an answer reaches
`ApplyPipes`, all sixteen are applied to the same string, so nothing REFUSES a
data-dependent operator anywhere. The failures are of a worse kind: the operator
is accepted and answers the wrong thing. Measured against the running daemon:

```
$ ze cli -c "show bgp | count"                 6      the top-level KEY count
$ ze cli -c "show bgp | peers | count"         2      correct, after a rows selection
$ ze cli -c "show bgp rpki | count"            8      the key count again
$ ze cli -c "show bgp | summary | count"       5      the alias selected 5 keys
$ ze cli -c "show host | count"                11
$ ze cli -c "system command list | count"      407    the RECORD path, and correct
```

`show bgp | first 1` and `| last 1` both answer the WHOLE payload:
`truncateItems` unwraps a map of one key and leaves a map of six alone.

| Failure | Operators lost | Population |
|---------|----------------|------------|
| rows AND aggregates as siblings, so `countItems` counts KEYS and `truncateItems` leaves the map alone | `count`, `first`, `last` | at least ten commands, named in `plan/audit-presentation-pipes.md` section 6 |
| the answer is one value, so no row operator has anything to act on and none is refused | `count`, `first`, `last`, `match`, `display`, `fill` | `show version \| first 1` answers `ze dev (built unknown)`, dropping the `version` key the bare command prints |
| a map keyed by identity rather than a list of rows | `display` cannot name the key; `count` answers the number of identities, right by accident | `show bgp peer <sel>`, `show bgp peer list` |
| `\| match` on the default chain | `match` | every command on the document path. Section 5 |

A command that answers one scalar and REFUSES `count` would be correct behavior
under the owner's model. Nothing in the tree refuses anything, so the correct
case does not exist yet either.

---

## 4. The client asymmetry

`ze cli -c` and an SSH exec channel let the DAEMON expand the chain
(`ProcessPipesDefaultFormatChecked`, `internal/component/ssh/ssh.go`). Bare
`ze cli` and `StreamMonitor` expand it in the CLIENT
(`model_mode.go`, `client/main.go`).

**None of the 16 generic operators splits on PARSING.** `knownPipeOps` is a
package-level map compiled into one binary that is both client and daemon, so
every name parses identically on both sides. The split is elsewhere, and there
are three of them.

| What splits | Daemon side | Client side |
|-------------|-------------|-------------|
| a plugin's pipe ALIAS | resolves | `pipe error: unknown pipe operator: <name>`. Already documented (`docs/architecture/api/commands.md`, "Where an alias resolves") |
| the SEMANTICS of `match`, `count`, `first`, `last`, `display` over a record answer | `RenderRecords` runs the per-record chain | the client asks for `\| raw`, receives the collapsed document, and runs the DOCUMENT chain over it |
| `\| log` | a no-op. `ApplyPipes` says "handled by caller" and the exec channel is not that caller | honored by the TUI monitor models |

The second is the one nobody has written down and the one a tool author will be
bitten by. `show bgp rib | match 10.0.0.0` over `ze cli -c` filters routes. The
same text in an interactive session greps the lines of a collapsed JSON
document, which for a compact payload is one line, so it answers all or nothing.

`| log` is in the owner's global class and is inert on the two surfaces a tool
author uses.

---

## 5. `| match` before a format operator does not filter rows, and repeated operators do not compose

Two defects in the document path, both measured with `ze pipe`, which runs the
same `ProcessPipesChecked` the CLI runs.

**`| match` greps the payload, not the rendering, unless a format operator comes
first.** `ProcessPipesDefaultFormatChecked` APPENDS the configured default format
to the END of the chain, so a bare `| match` runs over the dispatcher's JSON, and
compact JSON is one line. Three peers in, three peers out:

```
$ P='{"peers":[{"address":"192.0.2.1","state":"established"},{"address":"192.0.2.2","state":"idle"},{"address":"198.51.100.9","state":"idle"}]}'
$ echo "$P" | ze pipe match idle
{"peers":[{"address":"192.0.2.1","state":"established"},{"address":"192.0.2.2","state":"idle"},{"address":"198.51.100.9","state":"idle"}],"pipe":{"match":"idle"}}
$ echo "$P" | ze pipe text \| match idle
192.0.2.2     idle
198.51.100.9  idle
```

`docs/features/formatting.md` documents `match` as "Case-insensitive grep on
output lines". That is true only for the second spelling, and only the first
spelling is what an operator types.

**A repeated operator composes on some paths and replaces on others.**

```
$ echo "$P" | ze pipe text \| match idle \| match 192
192.0.2.2     idle
$ echo "$P" | ze pipe match idle \| match 192
{"peers":[ ...all three... ],"pipe":{"match":"192"}}
$ echo "$P" | ze pipe display state \| display address
{"peers":[{"address":"192.0.2.1"},{"address":"192.0.2.2"},{"address":"198.51.100.9"}]}
```

| Chain | Answer | Verdict |
|-------|--------|---------|
| `text \| match idle \| match 192` | one row | composes |
| `match idle \| match 192` | all rows, metadata `{"match":"192"}` | neither filter applied, and the first is lost from the metadata |
| `first 2 \| first 1` | one row, metadata `{"first":1}` | data composes, metadata keeps the last |
| `display address state \| display state` | `{state}` | narrows, but by REPLACEMENT |
| `display state \| display address` | `{address}` | **widens**. The second display recovers the field the first dropped |
| `fill alpha \| fill reverse` | the reverse order alone | replacement |
| `count \| count` | `{"count":1}` | counts the count |
| `json \| yaml` | refused, `multiple format operators` | correct |

`columnsInChain` (`pipe_columns.go`) assigns `request.display` and
`request.fill` for each occurrence rather than intersecting them, which is why
`display` replaces. `collectPipeMeta` (`pipe.go`) writes `meta["match"]`,
`meta["first"]`, `meta["last"]` and every unknown name into a map, so a repeat
overwrites the earlier one and the `pipe` key a tool author parses under-reports
the chain that ran.

`foldFilters` does the opposite for a command-owned filter: it APPENDS each
occurrence to the command's arguments, so `show bgp rib | match X | match Y`
reaches the handler as two `match` arguments and what happens next is the
handler's business, unstated anywhere.

The record path is a fourth answer, and it is the one that behaves.
`ApplyPipesRecords` wraps the sequence once for each op in chain order, so a
repeated `match` narrows twice and correctly:

```
$ ze cli -c "system command list | match bgp | count"        85
$ ze cli -c "system command list | match bgp | match rib"    the intersection
$ ze cli -c "system command list | first 3 | count"          3
```

It takes its column request from the same `columnsInChain`, so a repeated
`display` applies the LAST request twice, which is the replacement semantics
again. The replacement is live on the CLI, not only under `ze pipe`:

```
$ ze cli -c "show bgp | display state | display address"
peers   address
        192.0.2.1
        192.0.2.2
```

`address` is a field the first `display` had already dropped.

Four behaviors for one question, and a tool author has no way to know which
applies.

The error messages split the same way. A command-owned filter enumerates; a
generic operator does not:

```
$ ze cli -c "show bgp rib | nosuchfilter"
error: unknown pipe filter for show bgp rib: nosuchfilter (valid: advertised, community, count, family, first, graph, histogram, last, match, path, peer, prefix, received)
$ ze cli -c "show bgp | nosuchoperator"
error: unknown pipe operator: nosuchoperator
```

---

## 6. The rule, and why it is the wrong rule

`ai/rules/cli.md` and
`ai/rules/points/cli/directives/support-every-pipe-operator-in-every-command-that-produces-output.md`  <!-- doc-links: ignore (the point this audit judged; it was replaced by answer-every-global-operator-in-every-command-that-produces-output and answer-every-operator-the-answer-shape-supports-and-refuse-the-rest-by-name, which is the change section 6 asked for. Repointing it would destroy the record of what was audited) -->
both state: every command that produces output MUST support all pipe operators.

Counted against that rule, **38 commands violate it on every surface** and 8 more
violate it on the `ze <verb>` form while honoring it over `ze cli -c`. That is 46
of 465 where at least one way of invoking the command has no pipe layer at all,
and the failure is on the format operators, which is the worst place for a tool
author to be refused. Counted against the SHAPE the number is different and the
rule is the thing that is wrong:

- `| origin` on `show version` is meaningless. No amount of implementation makes
  a version string carry an ASN.
- `| first 3` on a command answering one scalar is meaningless.
- `| display` on a `doc` answer that is a single value has nothing to select.

The rule cannot distinguish those from a real gap, so it has produced a claim
the product does not meet and a wiki page that repeats the claim 381 times.

The right rule is two rules:

> A command MUST answer every operator that acts on the answer whatever it
> holds: the six formats, `no-more` and `log`.
>
> A command MUST answer every operator its answer's SHAPE supports, and MUST
> refuse the rest by name, saying why.

---

## 7. Documentation, judged as a tool author who does not read Go

**No.** A tool author cannot build against the published documentation today and
be right. The best page is complete about the operator names and wrong about
what they do; every other page is a shorter, differently wrong list.

| Page | Operators named | Verdict for a tool author |
|------|-----------------|---------------------------|
| `docs/features/formatting.md` | all 16 | the only complete list published. Opens with "Every command's output goes through the same pipe pipeline", which is false for 38 commands on every surface. Describes `match` as line grep, which holds only after a format operator. Its `ze pipe` examples name `ze debug show`, which is not a registered command |
| `docs/guide/command-reference.md` | 15. `\| raw` appears nowhere in the file | the primary command reference omits the operator its own sibling page calls "the stable contract for a script" |
| `docs/guide/cli.md` | 5 | the CLI tour the website links first, presenting five operators as the set. It spells the argument `\| match <regex>`; `applyMatch` is a case-insensitive `strings.Contains`, so a caller who writes a regex gets a literal match and no error |
| `docs/architecture/api/commands.md` | 13, omitting `count`, `first`, `last` | the deepest and most accurate page on aliases, filters and the client split. NOT in the website manifest, so it never publishes |
| `docs/guide/isis.md` | 7, presented as the set for `show isis` | understates by nine |
| `docs/guide/config-editor.md` | a DIFFERENT vocabulary: `blame`, `changes`, `compare`, `errors`, `history` | no published page says the config editor's `\|` and the operational `\|` are two languages. A reader of both pages builds one merged, wrong vocabulary |
| `README.md`, `CONTRIBUTING.md` | none | a tool author starting at the front door learns pipes exist from nowhere |

Nothing published says which commands support what. Four pages assert the
universal claim in prose. The per-command answer exists in one place and it is
generated: see 8.2.

---

## 8. Machine readability

### 8.1 What a tool can discover today

| Question | Best surface | What it omits |
|----------|--------------|---------------|
| which operators exist | `GET /cli/complete?input=<cmd>+%7C+&mode=operational` (`internal/component/web/cli.go`, `completeCLIInput`) | nothing: it returns all 16 with `type: "pipe"`. It needs the web server and auth, and the caller has to know to send a trailing `\|` |
| which operators exist, from the CLI | `ze help command --verbose` | six of sixteen, as prose |
| which commands accept them | `ze help command --json`, `"global-pipes": true` | the operator NAMES. It is a boolean, and `collectCommands` sets it unconditionally for every command carrying a wire method. It is an assertion, not a measurement, and for 20 commands it is FALSE: `show env list` is published with `"wire-method": "ze-show:env-list"` and `"global-pipes": true`, and the daemon answers `unknown command` |
| per-command FILTERS | `ze help command --json` `.pipes[]`, and `show command help` `.pipe-filters[]` | nothing. This part works, and it covers two commands, because two commands register filters |
| per-command ALIASES | `show command help` `.pipe-aliases[]` only (`internal/plugins/meta/cmd/help.go`) | absent from `ze help command --json`, which never calls `AliasesForCommand` |
| the `\| display` field names of a command | nowhere | `ColumnsForCommand` has no caller outside Tab completion and the renderers. Two commands declare an order |
| what an error teaches | `unknown pipe operator: <name>` | everything. It enumerates nothing. `unknownFilterError` does enumerate, but only command-owned filters, and only for a command that registered a set |

`ze help ai --json` and the MCP surface carry no pipe metadata at all. The REST
`/api/v1/commands` and its generated OpenAPI, the gRPC `DescribeCommand`, and
gNMI carry none either. `./le command list` walks `AllBuiltinRPCs` plus the
streaming prefixes and reports no pipe data and no plugin or local command.

### 8.2 The wiki is already generated, and it is still wrong

`./le wiki-catalog update` pipes `ze help command --json` through
the retired `scripts/dev/gen_wiki_commands.py` (current producer: `internal/le/wikicatalog/render.go`) into `../wiki/command-catalog.md`. The
machinery the owner wants exists.

It publishes a hand-typed operator list, and it publishes it on every command:

```
$ grep -c 'Global: `json`, `table`' ../wiki/command-catalog.md
381
```

381 commands are told they support ten operators. Six real ones are missing from
every one of those lines. Two commands get a `Command-specific:` block. Nothing
in the file distinguishes a command that can be counted from one that cannot,
and no `clear` or `set` command is marked as producing nothing to pipe, though
they all carry the line.

**This is the argument for deriving the page, made by the page itself.** It is
already generated, and it is wrong, because the generator holds its own copy of
the answer instead of reading it from the product.

### 8.3 The website has the per-command hook and puts nothing in it

Three website generators read the same JSON.
`render-command-equivalents.py` prints `Global pipes: yes` or `no` per command
and names no operator. `render-llms-txt.py`, which produces the page an AI tool
reads, sets a `pipes` meta flag and explains it as "the command supports the
shared output pipeline". `render-cli-catalog.py` reads the JSON, which carries
the per-command `pipes` array, and renders no pipe information at all: the
published CLI catalog silently drops every command-owned filter that exists.

`docs/architecture/api/commands.md` is the most accurate pipe documentation in
the repository and is not in `DOCS_MANIFEST` (`website/tools/page_registry.py`),
so it never reaches the website.

---

## 9. What must be built

Five pieces. Each is stated as an obligation, not a design.

### 9.1 One exported statement of the operator set, and one of each operator's contract

`knownPipeOps` becomes the source, with an exported reader beside
`PipeFiltersForCommand`, `AliasesForCommand` and `ColumnsForCommand`. Each
operator carries, in that one place:

| Property | Why it must be declared |
|----------|-------------------------|
| class: global or data-dependent | the derivation in 9.2 reads it |
| the shapes it applies to | so a shape can refuse it by name |
| whether it takes an argument, and of what kind | `ValidatePipes` already knows this and states it three times |
| whether repetition COMPOSES, REPLACES or is refused | section 5 shows four answers exist across the paths and none is written down |
| a one-line description | the five hand-typed copies all carry one |

`PipeOperators` in `completer.go`, the literal in `printCommandVerbose`, the one
in `gen_wiki_commands.py`, the one in `pipeUsage` and the `Subs` string in
`ze_core_dispatch.go` are all deleted and read from it. This is
`ai/rules/evidence.md`'s derive-not-hardcode directive applied to the one place
the repository has five copies.

### 9.2 A shape a COMMAND declares, which the answer head reports

The wire already carries `doc`, `map` and `tab`, and section 2.1 shows why the
head alone cannot be the contract: it is decided by walk length. So the command
declares its shape, the head reports the shape of the answer in hand, and the
two are checked against each other.

The derivation the owner asked for, stated as it must hold:

| Shape | Owed | Refused |
|-------|------|---------|
| `doc`, one value | the six formats, `no-more`, `log` | `first`, `last`, `match`, `display`, `fill`. `count` is 1 and answering it is worse than refusing it |
| `map`, rows, self-describing | the globals, plus `count`, `first`, `last`, `match`, `display` | `fill` cannot order what nothing declared |
| `tab`, rows with declared columns | everything `map` is owed, plus `fill`, and `display` COMPLETES from the head's names | nothing |

`resolve` and `origin` are owed by none of the three, because no shape says a
field holds an address. 9.4.

### 9.3 Refusal is part of the contract

An operator a shape does not support MUST be refused by name, with the reason.
Silently accepting it and answering something is the failure `show bgp | count`
already ships: an answer that looks plausible and is wrong.

`unknownFilterError` is the model. It names the command, the operator and the
valid set. `unknown pipe operator:` must do the same, and a shape refusal must
say which shape refused it.

### 9.4 The smallest honest mechanism for `resolve` and `origin`

Three candidates, in order of cost:

1. **A field-type on the head.** The head already carries column names for `tab`.
   Carrying a type beside each name is the same line, and it makes the refusal
   derivable for every command that answers `tab`. It does nothing for `map` or
   `doc`.
2. **A declaration on the command**, beside the column order it may already
   declare: which of its fields hold addresses. It covers all three shapes and it
   is a second copy of something the payload already knows.
3. **A key-name convention**, which the transforms half-use already by parsing
   every value. It is free and it is a guess, and
   `plan/audit-presentation-pipes.md` records that the same concept is spelled
   `nexthop` in one family and `next-hop` in three others, so a convention would
   be wrong on arrival.

Recommendation: 1 for `tab`, and refuse `resolve` and `origin` on `doc` and `map`
until a command declares otherwise. It is the only option that cannot drift,
because the head is written by the producer on every answer.

### 9.5 The page is generated, and a gate fails when the product and the page disagree

`./le wiki-catalog update` already generates the wiki catalog from
`ze help command --json`. The work is to make the JSON carry the answer and the
generator carry none of it.

| Piece | What it becomes |
|-------|-----------------|
| source of truth | the operator table of 9.1 plus each command's declared shape of 9.2, both in-process |
| the machine surface | `ze help command --json` grows a per-command operator list, replacing the `global-pipes` boolean, and gains the aliases it does not currently read. This is the API a tool author reads |
| the wiki page | `gen_wiki_commands.py` prints the list it is given and holds no literal |
| the website | `website/tools/render-cli-catalog.py` already consumes the same JSON and renders no pipe information at all. It renders this |
| the docs pages | `docs/features/formatting.md` keeps the prose and takes the operator table from a generated include. Every other page links to it rather than re-listing |
| the gate | `./le doc check verify` already runs `doc_drift.go`, which compares documentation claims against the registry. It grows one check: every operator name in `docs/` and in the wiki is in the exported set, and no command's published list disagrees with what the command declares |

`./le doc check verify` is the right home because it already owns "docs claims vs
registry" and already fails a build. A new parallel generator would be a sixth
copy of the problem this section exists to end.

---

## 10. Method and limits

The population is the 465 paths of `plan/audit-command-pipe-vs-subcommand.md`,
re-derived here only for the local registry (46 production paths from
`MustRegisterLocal` and `MustRegisterLocalMeta` literals outside `_test.go`).
`./le command list` was not used, for the reason that audit records.

A `ze` binary was built from this tree with the repository's feature tags and
driven two ways. `ze pipe` ran the operator chain over fixed JSON with no daemon,
which is the same `ProcessPipesChecked` the CLI runs, for 25 probes. A daemon was
started from a two-peer BGP config with an ephemeral SSH listener, the shape
`test/ui/alias-summary.ci` uses, and driven with `ze cli -c`.

**What was run: 418 daemon probes that answered, plus 25 offline ones.** All 254
`show` commands the catalog lists were run bare; 76 answered and 178 refused for
a missing argument, an unloaded plugin, or a platform this box is not. 27 of the
46 local-registry paths were run over `ze cli -c`, which is how 3.1's second row
is a measurement: all 20 of that row, plus the 6 confirmed dual and
`show debug profile`. 145 further probes ran operator chains over the commands
that answer.

Five limits.

- **The per-command claim in 3.3 is CLASSIFIED, not each run.** Reaching the pipe
  layer is a property of the dispatch path, and the exec channel calls
  `ProcessPipesDefaultFormatChecked` for every dispatched command, so nothing is
  learned by running the same chain over a 254th command. The commands run are
  the boundary of each class and the defects.
- **235 probes of a 508-probe sweep are VOID and are not counted above.** The
  daemon stopped answering partway through, after the sweep reached
  `show bgp irr prefix`, which reaches a live whois server. Every probe after
  that point timed out at 30 seconds. Nothing is claimed from them, and the
  batches that produced the evidence here ran against fresh daemons.
- **`| resolve` and `| origin` were not exercised against live DNS.** Their
  behavior is read from `applyResolve` and `applyOrigin`, not measured.
- **19 local paths were NOT run**: 17 of the 18 the catalog reports as
  `mode: offline` (all but `show debug profile`), plus the two streaming
  monitors. For those, "no daemon RPC exists" is read from the catalog and from
  their register files, not measured.
- **The web `/cli/complete` route is read, not run.** `completeCLIInput` with
  `mode=operational` calls `TreeCompleter.Complete`, which routes a trailing `|`
  to the operator completions, and the handler encodes `text`, `description` and
  `type` as JSON. It is the only surface that emits all 16 names to a machine, so
  a reader should run it before building on it. It also truncates at
  `maxCompletionResults` (50), which is above 16 today and is not a guarantee.
