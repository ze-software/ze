# CLI Verbs and What They Promise

Two sibling pages decide where a command lives. `command-namespacing.md` decides
which object it roots at and which register each token belongs to.
`root-namespace-grammar.md` decides which gate checks it. Neither says what the
first word PROMISES the operator.

This page says that. For each verb it states four things:

- what the operator gets back,
- what changes when they press Enter,
- where the selector goes,
- what a new command under that verb MUST look like.

It is a target the command tree does not fully meet. The last two sections name
every place the two disagree.

Read this before you add a verb, a command, or a pipe operator.

## The three questions a command answers

Every operator line answers three questions, and each one has one home.

| Question | Name | Where it lives |
|----------|------|----------------|
| What is this about? | Subject | The command path: the verb, then the object, then the action |
| Which members of the subject? | Narrowing | A positional selector for an action, a pipe operator for a view |
| How much of each member? | Shaping | A pipe operator |

Keep the three apart and the grammar stays learnable. Fold two into one token
and the tree grows a slot that accepts both an operator-supplied name and a
fixed keyword. That ambiguity is what every rule below exists to prevent.

## The verb classes

Twelve verbs are canonical. `command.Verbs` is the one statement of the set, and
both the grammar gate and the plugin registration check derive their verb set
from it.
<!-- source: internal/component/command/verbs.go -- Verbs, verbRole -->

| Verb | Role | It promises | Side effect on | Repeat is safe | Commands |
|------|------|-------------|----------------|----------------|----------|
| `show` | Read | One snapshot of state, now | Nothing | Yes | 236 |
| `monitor` | Read | The same read, repeated until the operator stops it | Nothing | Yes | 8 |
| `resolve` | Read | An answer fetched from outside the daemon | Nothing inside Ze | Yes | 11 |
| `set` | Mutation | One config tree node takes a new value | The running config | Yes | 1 |
| `delete` | Mutation | One config tree node, or one live kernel resource, stops existing | The running config, or the kernel | Yes | 4 |
| `clear` | Action | Accumulated runtime state returns to zero | Counters and caches | Yes | 19 |
| `request` | Action | The running system performs an operation | The session, the wire, or the process | No | 33 |
| `create` | Action | A live kernel resource exists after the command | The kernel | Yes | 5 |
| `update` | Action | Data held for an object is fetched again and rewritten | The config draft, or a cached set | Yes | 13 |
| `debug` | Action | Live protocol state is perturbed on purpose | The wire | No | 4 |
| `cache` | Action | Declared, and no command uses it at root | - | - | 0 |
| `commit` | Action | Declared, and no command uses it at root | - | - | 0 |

The counts come from `./le command list`, which reads the live handlers and
schemas and reports 363 commands on this checkout. Run it rather than trusting
this column.
<!-- source: internal/le/command/list/commandlist.go -- Collect, Answer -->

Five roots outside the twelve carry commands, and each is exempt for a stated
reason:

- `peer` and `announce` mirror the ExaBGP text line protocol,
- `plugin` and `system` are process-boundary directives rather than operator
  commands,
- `help` is the bridge's own word.

`grammar.ExemptCategory` keys the exemption on the handler wire method, never on
a command name.
<!-- source: internal/component/command/grammar/checker.go -- bridgeSurface, ExemptCategory -->

## Choosing the verb

Ask one question, in this order, and stop at the first Yes.

| # | Question | Verb |
|---|----------|------|
| 1 | Does it change what the router does, emits, or forwards? No, it only reports | `show`, or `monitor` for the same read streamed |
| 2 | Does it read an answer that lives outside Ze? | `resolve` |
| 3 | Does it change a node of the config YANG tree? | `set`, or `delete` |
| 4 | Does it return accumulated runtime state to zero? | `clear` |
| 5 | Does it make a kernel resource exist? | `create` |
| 6 | Does it fetch data for an object again and rewrite it? | `update` |
| 7 | Does it perturb live protocol state to test something? | `debug` |
| 8 | Anything else the running system performs | `request` |

Two consequences follow, and neither is negotiable. How diagnostic a command
feels never decides its verb, so an introspection that changes nothing is `show`
however deep it reads. And an operational `add`, `del` or `remove` MUST NOT be
invented for an object the config YANG tree already holds. A change to a tree
node is `set` or `delete` in engine path form.

`debug` is double-gated. Authorization admits the operator, and a fail-closed
runtime enablement admits the command, so a `debug` command refused by either
gate performs nothing.

## Read verbs and their guarantee

A read verb promises that the router behaves the same after the command as
before it. That promise is what lets a read-only operator run it, so the
promise is a security boundary and not a convenience.

`IsReadOnlyPath` reads the first word of the path and picks the authorization
section. A read gets the `run` section, and everything else gets the `edit`
section. A command rooted at a read verb is therefore reachable by every
operator holding read-only access.
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath -->
<!-- source: internal/component/authz/authz.go -- Profile.Authorize, Section -->

So one rule governs the whole `show` and `monitor` subtree, and it has no
exception: **a command under a read verb MUST NOT change state.** A command that
starts a capture, discards a buffer, or writes a file belongs under an action
verb, whatever it prints.

## Action verbs and their selector

An action names the objects it acts on with a positional selector, typed BEFORE
the action word. The order reads as a sentence: this peer, tear it down.

```
<verb> <noun> <selector-kind> <selector-value> <action> [<args>]
```

Peer commands are the one exception, and they drop the selector kind:

```
request peer <selector> teardown <cease-subcode>
```

A peer selector is an address, a peer name, an AS pattern such as `as65001`, a
glob, a comma-separated list, or `*`.
<!-- source: internal/component/config/yang/modules/ze-types.yang -- peer-selector -->

Two declarations make the selector mandatory, and both are owed.

| Declaration | What it does | Where |
|-------------|--------------|-------|
| `leaf selector { mandatory true; }` | Typed validation, completion, and the published grammar | The YANG container the operator types the selector after |
| `RequiresSelector: true` | The dispatcher refuses the command when no selector arrives | The handler's `RPCRegistration` |
<!-- source: internal/component/plugin/server/command.go -- selectorLeaf, RequiresSelector, Dispatch -->

The second is a fail-closed guard, not a duplicate of the first. Without it a
selector-less invocation reaches the handler and the handler decides, which is
where a wildcard becomes an action on every peer. 18 YANG nodes declare a
mandatory selector leaf and 14 registrations carry the guard.

**Every argument a handler reads MUST be declared in the schema.** An undeclared
argument reaches the handler and reaches nothing else. Completion cannot offer
it, help cannot print it, and typed validation cannot refuse a bad value.

## Narrowing and shaping: the pipe layer

Ze reads a command's answer the way Unix, nushell and PowerShell read a
pipeline. One difference decides everything else: the pipe carries RECORDS
rather than text. `| json`, `| yaml` and `| table` are three renderings of one
payload, so a handler that answers with finished text has chosen the reader's
format for them.
<!-- source: internal/component/plugin/types.go -- ResponseData, Records -->
<!-- source: internal/component/command/pipe_catalog.go -- pipeCatalog, PipeClass -->

**A pipe is a display limit.** It narrows an answer that already exists, and it
never chooses what a command ACTS on. A view narrows with a pipe. An action
narrows with a positional selector before the verb, because an action produces
no answer to narrow.

Three operator classes exist, and the class decides which commands owe the
operator.

| Class | Acts on | Owed by |
|-------|---------|---------|
| `ClassGlobal` | The answer, whatever it holds: the formats, paging, saving | Every command that produces output |
| `ClassData` | Rows and fields | Every command whose answer shape carries rows |
| `ClassStream` | A sequence of answers | The `monitor` commands alone |

A command that cannot support an operator MUST refuse it BY NAME. Silence tells
the operator their intent was honored.

A command can also own filters of its own. An owned filter is pushed down to the
handler rather than applied locally. `foldFilters` rewrites `show bgp rib | peer
10.0.0.1` into `show bgp rib peer 10.0.0.1`. The daemon does the selecting, and
only the matching rows cross the wire.
<!-- source: internal/component/command/pipe.go -- foldFilters, ParsePipe -->
<!-- source: internal/component/command/pipe_filter.go -- PipeFilter, RegisterPipeFilters -->

**An owned filter takes the name of the field it selects on.** `show bgp rib`
owns `peer`, `family`, `prefix`, `path` and `community`, and each word names the
thing the operator is choosing by. A generic word for "select one of these"
teaches the operator a second name for a concept the field already names. L-6
below records the one place this rule is under review.
<!-- source: internal/component/bgp/plugins/cmd/rib/rib.go -- registerPipeFilters -->

## Idempotency: what the model records

The model records idempotency for a PIPE OPERATOR and for nothing else.
`PipeRepeat` answers one question: what does a second occurrence of this
operator in one chain mean? It has three values. `RepeatCompose` narrows
further, `RepeatIdempotent` changes nothing, and `RepeatRefuse` is an error
because no answer to "which one wins" is honest.
<!-- source: internal/component/command/pipe_catalog.go -- PipeRepeat -->

**No command carries an idempotency field.** `RPCRegistration` holds four
fields, and none of them is it. The column in the class table above is therefore
a property of the verb's CONTRACT, stated here. No gate can read it or check
it.
<!-- source: internal/component/plugin/server/handler.go -- RPCRegistration -->

One narrow obligation exists, and it is enforced at runtime rather than
declared. A creation handler reachable from a `ze:ensure-exists` path MUST be
idempotent. It proves it by answering `Data["created"]`, `true` when it created
the resource and `false` when the resource already existed. `wasCreated` refuses
a handler that answers neither, because reading a missing key as "not created"
would delete a resource the operator owns.
<!-- source: internal/component/plugin/server/ensure.go -- EnsureStep, wasCreated -->

`create interface dummy name d0` therefore succeeds twice. The second run
reports `created: false` and changes nothing, and only a type conflict is an
error.
<!-- source: internal/component/iface/cmd/manage.go -- handleCreateTyped -->

## Adding a command

Answer these in order. Each one has a single correct answer from this page.

1. Pick the verb from "Choosing the verb". Stop at the first Yes.
2. Root the command at the object, which is the component or plugin that owns
   it (`command-namespacing.md`).
3. Decide whether the command is a view or an action. A view narrows with a
   pipe. An action takes a positional selector before the action word.
4. For an action on one member, declare `leaf selector { mandatory true; }` on
   the container the operator types the selector after, and register the handler
   with `RequiresSelector: true`.
5. Declare every argument the handler reads as a YANG leaf.
6. Answer with structured data satisfying `ResponseData`, never with formatted
   text, and refuse by name every operator the shape cannot support.
7. Where the command enumerates the members of a set, spell that action `list`,
   and place it where no operator-supplied value can occupy the same slot.
8. Run `./le cli-grammar`. Seven feeders check the result.

## Least surprise: what an operator already knows

Ze's operators arrive from Cisco IOS, JunOS, Nokia SR OS, Arista EOS, FRR and
BIRD. A verb that matches what they know costs them nothing. A verb that does
not costs them a lookup on every use, so the divergence has to buy something.

| Ze verb | What they expect | Ze matches | Verdict |
|---------|------------------|------------|---------|
| `show` | Universal. FRR roots 56 distinct objects under `show`, and BIRD declares 25 `SHOW` commands, among them `SHOW ROUTE`, `SHOW PROTOCOLS` and `SHOW STATUS` | Yes | Matched |
| `monitor` | VyOS and JunOS both stream with it: `monitor traffic interface <i>`, `monitor interface <i>` | Yes | Matched |
| `request` | JunOS shape: `request system reboot`, `request system halt`, `request system software add` | Yes | Earned. It names one class, where Nokia uses `admin` and Arista uses `reload` and `install` |
| `clear` | Every one of the five resets a BGP session with it: FRR and Arista `clear ip bgp <n>`, JunOS `clear bgp neighbor <n>`, Nokia `clear router bgp neighbor <n>`, VyOS `reset ip bgp <n>` | Partly | Earned. Ze keeps `clear` for counters alone, because `delete` acts on config, `clear` on counters, and `remove` on a route. A session reset is not a counter, so it is `request peer <sel> teardown` |
| `set` and `delete` | Config mode in FRR and Cisco, `set` and `delete` in VyOS and JunOS | Yes | Matched |
| `create` | Neither FRR nor BIRD has it. VyOS and JunOS create through config | No | Earned. It names a LIVE kernel resource, which config-mode verbs cannot express |
| `debug` | FRR declares `debug` commands, BIRD declares `CF_CLI(DEBUG, ...)` | Partly | Earned. Both use `debug` for log verbosity, Ze uses it for a deliberate perturbation, and Ze's is the narrower promise |
| `resolve` | Nobody. VyOS spells it `force dns update`, JunOS `request dns resolve` | No | See divergence L-1 |
| `update` | Nobody uses it as a root verb | No | Merely different, and unresolved. `update bgp irr as-set` refetches an external set, which JunOS would spell `request` |
<!-- source: /Users/thomas/Code/github.com/FRRouting/frr -- bgpd/bgp_vty.c clear command strings, `show` command strings -->
<!-- source: /Users/thomas/Code/gitlab.nic.cz/labs/bird -- nest/config.Y CF_CLI declarations -->
<!-- source: docs/guide/command-catalogue.md -- the VyOS, JunOS, Nokia and Arista columns -->

The selector's POSITION matches FRR and diverges from BIRD. FRR takes the peer
selector before the action modifier, `clear [ip] bgp <*|A.B.C.D|WORD|ASNUM|
external|peer-group PGNAME> [soft [in|out]]`, which is Ze's order. BIRD takes it
after the verb, `reload in <pattern>`. Ze follows FRR, and FRR's selector
vocabulary is close to Ze's own.
<!-- source: /Users/thomas/Code/github.com/FRRouting/frr -- bgpd/bgp_vty.c, the clear_ip_bgp command string -->

## Where the tree diverges from this page

Each row is a defect. The page states the rule and the tree does not meet it.

| # | Command or symbol | The rule it breaks | Evidence |
|---|-------------------|--------------------|----------|
| T-1 | `show vpp trace clear` | A read verb MUST NOT change state. It discards the trace buffer, and `IsReadOnlyPath` returns true for it, so a read-only operator reaches it | `internal/plugins/iface/vpp/cmd_show.go` `handleVPPTraceClear` calls `vppcomp.TraceClear` |
| T-2 | `show vpp trace start` | The same rule. It starts packet tracing in the dataplane, which costs per-packet work | `internal/plugins/iface/vpp/cmd_show.go` `handleVPPTraceStart` |
| T-3 | `show bgp peer list` | No slot accepts both a fixed keyword and an operator-supplied value. `container peer` declares a mandatory selector and `list` is a sibling container carrying `ze:inherit "none"`, so a peer named `list` is unreachable | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`. Fix: `plan/spec-fixit-selector-narrows-through-a-pipe.md` |
| T-4 | `peer raw`, `peer update`, `peer announce`, `peer withdraw` | One object has one action root. These four sit at the top-level `peer` root while every other peer action sits under `request peer` | `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang`. Fix: the same spec moves all four to `request peer <sel> <verb>` |
| T-5 | `peer raw` arguments | Every argument a handler reads MUST be declared. The schema declares `selector` alone, and the handler reads a message type, an encoding and the data | `internal/component/bgp/plugins/cmd/raw/raw.go` `handleRaw` |
| T-6 | `request peer <sel> clear soft` | One concept carries one name. It sends ROUTE-REFRESH for every negotiated family. `request peer <sel> refresh <family>` sends it for one. The word `clear` is also a root verb, used here as a leaf word | `internal/component/bgp/plugins/route_refresh/handler/clear_soft.go` `handleBgpPeerClearSoft`, beside `refresh.go` `handleRefresh` |
| T-7 | `readOnlyVerbs` | Every fact is declared once. It holds `show`, `validate` and `monitor`, missing `resolve`, and `validate` is not a canonical verb | `internal/component/command/help.go` `readOnlyVerbs`, against `verbs.go` `Verbs` |
| T-8 | `IsReadOnlyPath` | The same rule, and this is the copy that gates authorization. It holds eight words where `Verbs` holds three reads | `internal/component/plugin/server/command.go` `IsReadOnlyPath` |
| T-9 | `classifyVerb` | The same rule, a fourth copy. It knows five verbs, so `./le command list` reports `-` for all 33 `request` commands and all 19 `clear` commands | `internal/le/command/list/commandlist.go` `classifyVerb` |
| T-10 | `cache` and `commit` | A verb is declared because commands use it. Both are canonical root verbs that no command uses, and both live under `request` instead | `verbs.go` `Verbs`, against `./le command list` |
| T-11 | `request quiesce` | Ze writes the plain verb rather than the specialist one, and `docs/contributing/writing-style.md` names this exact pair | `./le command list`, `ze-system:quiesce` |
| T-12 | `ai/patterns/cli-command.md` | A page MUST match the tree. Its Full Command Inventory publishes `cache list`, `commit start <name>`, `command list`, `log set` and `subscribe <type>`, which are the pre-verb-first spellings | The live paths are `request cache retain`, `request commit`, `system command list`, `request log level` and `request subscribe` |
| T-13 | `set system file-descriptors` | `set` is `VerbMutation`, which `verbs.go` defines as mutating the config YANG tree in engine path form. The one shipped `set` command calls `setrlimit` on the process and touches no config node | `internal/plugins/host-cmd/cmd/set_fd_linux.go` `handleSetSystemFD`, against `verbs.go` `VerbMutation` |
| T-14 | `docs/guide/command-catalogue.md` | The same rule. Its Naming convention section states "domain-first, verb-second" and reserves a `generate` root verb, and it prints `bgp monitor` | `verbs.go` `Verbs` holds no `generate`, and the live path is `monitor bgp` |

## Where this page diverges from what an operator expects

Each row is a design position rather than a defect. It has to be defended or
changed, and it is not a bug list.

| # | Position | The surprise | Defense |
|---|----------|--------------|---------|
| L-1 | `resolve ping` and `resolve traceroute` | Every reference CLI has `ping` bare or under a diagnostics verb. No CLI uses `resolve` as a root verb, and `docs/guide/command-catalogue.md` states that Ze keeps universal diagnostics bare | Undefended. `resolve` promises an answer fetched from outside the daemon, which fits a DNS lookup and does not fit sending ICMP. Both commands also emit packets, so classing them read-only stretches rule 1 of "Choosing the verb" |
| L-2 | The peer selector carries no kind keyword | Every other Ze noun types its selector: `show interface name <n> detail`, `show l2tp session id <id>` | Earned. The bare form is the ExaBGP order an operator already types, and it is the order FRR publishes. Its cost is T-3, and the cost falls on enumerating actions alone |
| L-3 | `delete` carries two roles | It removes a config tree node AND a live kernel resource, so one word means two things | Earned, and stated in `verbs.go`. `create` has no config sense, so pairing it with a second deletion verb would add a word to carry no new meaning |
| L-4 | `clear` never resets a session | All five reference CLIs reset a BGP session with `clear` or `reset` | Earned. Ze keeps one meaning for `clear`. The cost is that an operator's first guess fails, and the error message is what has to carry them to `request peer <sel> teardown` |
| L-5 | Rendering has no flag | Operators arriving from any of the five reach for a flag or a per-command keyword | Earned. Rendering belongs to the pipe layer on every command, so one operator set serves 363 commands and only the operator composes (`command-namespacing.md`) |
| L-6 | The name of the narrowing operator is OPEN | `plan/spec-fixit-selector-narrows-through-a-pipe.md` proposes one operator named `limit` for every display command, which would replace the field-named `\| peer <sel>` on `show bgp rib`. That reading of `limit` disagrees with the rule above, and it disagrees with the word's other uses | Undecided, and Thomas decides. `limit` already means a numeric row cap in three Ze surfaces, in FRR (`bgp listen limit`, `prefix-limit`) and in BIRD (`babel limit`, merge-paths `limit`). Ze also ships `\| first <n>` and `\| last <n>` beside it |

## See also

- `command-namespacing.md` for where a command roots and which register a token
  belongs to.
- `root-namespace-grammar.md` for the seven feeders and what each checks.
- `ai/rules/cli.md` for the blocking directives this page explains.
- `docs/guide/command-catalogue.md` for the cross-vendor command mapping.
