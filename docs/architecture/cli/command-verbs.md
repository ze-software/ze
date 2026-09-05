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

It is a target the command tree does not fully meet. Three sections name every
place the two disagree: the read-verb list, the tree defects, and the design
positions.

Read this before you add a verb, a command, or a pipe operator.

## The axis: does the command change the system

One question decides the verb, and every rule below refines it.

**Does the command change the system?**

`show`, `monitor` and `resolve` change nothing. An operator runs one of them on
a production router at three in the morning, and stops to check nothing first.
That is the whole value of a read verb. It survives only while every command
under those three words keeps it.

Every other verb changes something. `set`, `delete`, `clear`, `create`, `update`
and `debug` each name a PARTICULAR change. `request` is the general word for a
change that none of them names. Thomas stated it in these words: "request lead
to a change in the system".

`send` sits on the axis in a third place, and that is why it is a verb of its
own. It changes something OUTSIDE the system. The bytes the operator supplies
leave the router, and what they change is a peer's state. So a send is not a
read, and a send is not a request either. L-8 records the decision.

The axis is declared in code rather than here. `verbRole` gives `show`,
`monitor` and `resolve` the role `VerbRead`, `set` and `delete` the role
`VerbMutation`, and every other canonical verb the role `VerbAction`.
<!-- source: internal/component/command/verbs.go -- Verbs, verbRole -->

The per-verb table below refines the second half of the split. It does not
replace the split. A verb that changes nothing and a verb that changes something
are two populations. No detail about the second one moves a command into the
first.

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

Thirteen verbs are canonical. `send` is the newest: decided by Thomas on
2026-09-05 and declared by
`plan/immediate/spec-fixit-send-names-its-destination.md`. `command.Verbs` is
the one statement of the set, and both the grammar gate and the plugin
registration check derive their verb set from it.
<!-- source: internal/component/command/verbs.go -- Verbs, verbRole -->

| Verb | Role | It promises | Side effect on | Repeat is safe | Commands |
|------|------|-------------|----------------|----------------|----------|
| `show` | Read | One snapshot of state, now | Nothing | Yes | 236 |
| `monitor` | Read | The same read, repeated until the operator stops it | Nothing | Yes | 8 |
| `resolve` | Read | An answer fetched from outside the daemon | Nothing | Yes | 11 |
| `set` | Mutation | One config tree node takes a new value | The running config | Yes | 1 |
| `delete` | Mutation | Something that existed stops existing: a config tree node, or a live kernel resource | The running config, or the kernel | Yes | 4 |
| `clear` | Action | Data that exists takes one fixed value, zero or empty | Counters and caches | Yes | 19 |
| `request` | Action | The system changes, and the change is none of create, delete or update | The session, the wire, or the process | No | 33 |
| `create` | Action | Something that did not exist exists after the command, today a live kernel resource | The kernel | Yes | 5 |
| `update` | Action | Data held for something that exists is rewritten | The config draft, or a cached set | Yes | 13 |
| `debug` | Action | Live protocol state is perturbed on purpose | The wire | No | 4 |
| `send` | Action | Bytes the operator supplies leave the router for a destination the operator names | The wire, and the peer that reads it | No | 0 while the move runs: `send bgp raw` answers, but the inventory reports each wire method at its SHORTEST path, and that is still `peer raw` |
| `cache` | Action | Declared, and no command uses it at root | - | - | 0 |
| `commit` | Action | Declared, and no command uses it at root | - | - | 0 |

The three read rows state the rule. The tree breaks it in 18 places, and "Where
a read verb changes the system" lists every one.

The counts come from `./le command list`, which reads the live handlers and
schemas and reports 363 commands on this checkout. Run it rather than trusting
this column.
<!-- source: internal/le/command/list/commandlist.go -- Collect, Answer -->

Five roots outside the thirteen carry commands, and each is exempt for a stated
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
| 1 | Does it change the system? No, it only reports | `show`, or `monitor` for the same read streamed |
| 2 | Does it read an answer that lives outside Ze? | `resolve` |
| 3 | Does it change a node of the config YANG tree? | `set`, or `delete` |
| 4 | Does it return accumulated runtime state to zero? | `clear` |
| 5 | Does it make something exist that did not exist? | `create` |
| 6 | Does it rewrite data held for something that exists? | `update` |
| 7 | Does it perturb live protocol state to test something? | `debug` |
| 8 | Does it put bytes on a destination? | `send` |
| 9 | It changes the system, and no question above answered it | `request` |

Two consequences follow, and neither is negotiable. How diagnostic a command
feels never decides its verb, so an introspection that changes nothing is `show`
however deep it reads. And an operational `add`, `del` or `remove` MUST NOT be
invented for an object the config YANG tree already holds. A change to a tree
node is `set` or `delete` in engine path form.

Question 1 asks about the system, never about the daemon alone. A command that
sends a packet changes the system, because the packet reaches a peer and the
peer answers. A command that writes Ze's own cache changes the system too.
Neither is a read.

### What earns `debug` a word of its own

Under the axis `debug` changes the system, exactly as `request` does. Three
mechanisms key on the word, and no `request` command carries any of them.

| Mechanism | What it does | Where |
|-----------|--------------|-------|
| Authorization | The read-only profile denies `debug`, entry 40 of the `run` section | `authz.go` `builtinReadOnlyProfile` |
| Runtime enablement | The engine refuses to originate a crafted LSA until an operator enables injection. It is not persisted, so a reboot returns the router to injection off | `debug_enable.go` `debugInjectEnabled` |
| Doctor | A check raises a Warning for as long as injection stays enabled | `doctor_debug.go` `codeOSPFDebugEnabled` |

Both gates fail closed, so a `debug` command refused by either one performs
nothing. The double gate is most of what earns the separate word. The Warning is
the third part. An operator who leaves injection enabled is told on every
`show doctor`.

## The four state-changing verbs

`create`, `delete`, `update` and `request` are one family. The first three each
answer a positive test. `request` is the residual: it is what the family holds
when none of the three tests answers (owner directive, 2026-09-03).

| Verb | The test it answers |
|------|---------------------|
| `create` | Something that did not exist exists after the command |
| `delete` | Something that existed stops existing |
| `update` | Data held for something that exists is rewritten |
| `request` | The system changes, and none of the three tests above answers |

Thomas stated the residual in these words: "request lead to a change in the
system, like create, delete or update but it is when the action is neither of
the three".

Two consequences follow.

**Test the three positive verbs first.** "Choosing the verb" asks `delete` at
question 3, `create` at 5 and `update` at 6. It asks `request` last, at 9. A
command reaches the residual only when all three have failed. A command that
carries `request` therefore states that none of the three describes it. The
residual is only as good as the three tests in front of it.

**A compound of two is neither of them.** `request interface migrate` moves an
address from one interface to another, which is a delete and a create together.
No single test describes the whole effect, so the command is the residual.
<!-- source: internal/component/iface/cmd/cmd.go -- handleInterfaceMigrate -->

### The boundary case an operator gets wrong

`request peer <sel> teardown` closes a BGP session and leaves the peer in the
running config. The peer exists after the command, and `show bgp peer` still
lists it. Nothing stopped existing, so `delete` does not answer and the command
is the residual. An operator arriving from any of the five reference CLIs
reaches for `delete` or `clear` here, and both are wrong. `delete bgp peer
<sel>` is the command that removes the peer.
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- request peer teardown, delete bgp peer -->

The two commands beside it settle the same way, and neither moves under
`request`. `delete bgp peer <sel>` removes the peer from the running config, so
it deletes. `update bgp peer prefix` refetches the max-prefix limits from
PeeringDB and rewrites them, so it updates.
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- delete bgp peer, update bgp peer prefix descriptions -->

### Where `clear` sits

`clear` is not a fifth member of the family. It writes one fixed value, zero or
empty, to data that exists. That is the `update` test, with the value named by
the verb rather than typed by the operator. "Choosing the verb" therefore asks
`clear` at question 4 and `update` at 6: the narrower test runs first.

Two facts keep the separate word, and the axis touches neither. All five
reference CLIs carry `clear`, so an operator arrives already holding it. And the
read-only authorization profile denies the word itself, entry 30 of the `run`
section. An operator is therefore refused `clear` by name rather than by path.
<!-- source: internal/component/authz/authz.go -- builtinReadOnlyProfile -->

So the evidence points to keeping `clear` as a root verb, and to policing what
sits under it. The word cannot be derived from the axis, because every command
under `clear` changes the system and so does every `request`. A command that
closes a session returns nothing to zero. Neither does one that removes a named
entry, and neither does one that replays a RIB. Each is `delete` or the
residual, whatever its noun.

The tree fails that test in nine places, and T-21 lists them. The two costs pull
in opposite directions, and the internal one is larger. FRR, Arista, JunOS and
Nokia all reset a BGP session with `clear`, so the rule costs an operator their
first guess on BGP. The nine commands at T-21 make the same guess SUCCEED on
L2TP, OSPF, IS-IS and IPsec.

One word that promises two things across protocols costs more than one word that
disagrees with four vendors. An operator learns the vendor difference once. The
protocol difference is learnable nowhere.

## The send shape

A command that puts bytes on a destination takes one of two forms.

```
send <protocol> <destination> raw <encoding> <data>
send <protocol> <destination> <message> [<field> <value> ...]
```

`send` is a root verb, and the word exists to keep a destination out of a slot
that already holds keywords. The token after `request` is drawn from a closed set
the compiler knows: 17 distinct words over the 33 `request` commands. Nine name a
subsystem (`peer`, `interface`, `cache`, `bgp`, `ospf`, `log`, `l2tp`, `config`,
`as112`). Eight are bare actions (`commit`, `halt`, `quiesce`, `reboot`,
`reload`, `shutdown`, `subscribe`, `unsubscribe`).

A destination is free-form text. So `request <destination> <protocol> ...` puts
an arbitrary string in the slot where `bgp` already names a subsystem, and no
parser can tell the two apart. `send` gives the destination a root of its own,
and the protocol keyword in front of it types the slot. That is the
keyword-before-value rule applied where it is load bearing (`ai/rules/cli.md`).

The protocol comes FIRST because it says how to read the value after it. The
token after `send` is a protocol keyword from a closed set. Completion, the
grammar gate and the authorization profiles each meet a known word rather than a
guess. Each protocol's own package declares its own destination leaf, with its
own type and its own completion.

Four rules follow, and each one decides a command a reader is about to write.

**The token after the destination names WHAT is sent.** It never repeats the act
of sending. Thomas settled it on 2026-09-05, on the example
`send bgp * announce unicast 10.0.0.0/24 next-hop 10.0.0.1`: the word `announce`
"is redundant". `send` is the verb. `announce` was a second verb in the same
command, and to send a route IS to announce it. The BGP set is `unicast`,
`blackhole`, `flowspec`, `raw`, `update`, `cached` and `withdraw`.

`withdraw` stays, and the asymmetry is correct. A withdraw is the opposite act,
not the same act named twice. A withdrawal is also a thing an operator sends,
because RFC 4271 carries it in the UPDATE's withdrawn-routes field, and it is
the word BGP and ExaBGP both use. So an operator meets one spelling on each side
of the bridge.

**`raw` is not a BGP verb.** It is the escape hatch every protocol has, and it
sits beside that protocol's structured messages. `send bgp <destination> raw hex
<data>` and `send bgp <destination> update <field> <value>` are the same shape
with a different last part.

**The slot holds a DESTINATION, not a selector.** A configured peer name and a
bare address are both destinations, and the resolver decides which one arrived.
The command therefore never rules on whether the token names a configured
object, which is the ambiguity L-2 and T-3 record on the peer selector.

**The protocol is a PARAMETER, not a root.** `peer` stops needing to be a verb
root once the protocol moves into the command, which is what T-4 asks for.

## Read verbs and their guarantee

A read verb promises that the router behaves the same after the command as
before it. That promise is what lets a read-only operator run it, so the
promise is a security boundary and not a convenience.

`IsReadOnlyPath` reads the first word of the path and picks the authorization
section. A read gets the `run` section, and everything else gets the `edit`
section, which the read-only profile denies by default. A command rooted at a
read verb is therefore reachable by every operator holding read-only access.
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath -->
<!-- source: internal/component/authz/authz.go -- Profile.Authorize, Section, builtinReadOnlyProfile -->

One rule governs the whole `show`, `monitor` and `resolve` subtree, and it has
no exception: **a command under a read verb MUST NOT change state.** A
command that starts a capture, discards a buffer, sends a packet, or writes a
file belongs under an action verb, whatever it prints.

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
| `send` | Cisco IOS spells `send <line> <message>`, which writes a message to a terminal line. No reference CLI puts protocol bytes on a destination with it | Partly | Earned. `inject` implies forcing bytes into an EXISTING session, which is wrong for an arbitrary destination, and Ze's own raw path uses that word today. `transmit` and `emit` have no operator precedent, and `to` is a preposition in a verb slot. The Cisco collision is a different domain. Ze holds no Cisco checkout, so this row cites the published EXEC command rather than source |
| `clear` | Every one of the five resets a BGP session with it: FRR and Arista `clear ip bgp <n>`, JunOS `clear bgp neighbor <n>`, Nokia `clear router bgp neighbor <n>`, VyOS `reset ip bgp <n>` | Partly | Earned as a rule, and broken in the tree. The rule keeps `clear` for a value returned to zero, because `delete` acts on config, `clear` on counters, and `remove` on a route. A session reset is not a counter, so the rule spells it `request peer <sel> teardown`. Nine commands break the rule, and T-21 lists them |
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

## Where a read verb changes the system

Each row is a defect, and this is the list a reader acts on first. Every command
here is reachable by an operator holding read-only access, because
`IsReadOnlyPath` returns true for its first word. The Owes column names the verb
the axis forces, never a new word.

Eighteen of the 255 commands under `show`, `monitor` and `resolve` change the
system, and the 13 rows below hold them. The other 237 keep the promise.

| # | Command | What it changes | Owes | Evidence |
|---|---------|-----------------|------|----------|
| T-1 | `show vpp trace clear` | It discards the dataplane trace buffer. The buffer is gone after the command returns | `request` | `cmd_show.go` `handleVPPTraceClear` calls `vppcomp.TraceClear` |
| T-2 | `show vpp trace start` | It starts packet tracing in the dataplane, which costs per-packet work. Tracing stays on after the command returns | `request` | `cmd_show.go` `handleVPPTraceStart` calls `vppcomp.TraceStart` |
| T-22 | `show capture raw start`, `show capture raw stop` | They enable and disable raw frame capture on the BGP reactor, the BFD plugin and L2TP. The setting outlives the command | `request` | `capture_raw.go` `captureRawStart` calls `EnableRawCapture`, and `captureRawStop` calls `DisableRawCapture` |
| T-23 | `show capture interface` | It opens an AF_PACKET socket on the named interface and attaches a compiled BPF filter. It also holds an exclusivity flag, so a second capture on that interface is refused | `request` | `capture_interface_linux.go` `HandleCaptureInterface` calls `packet.Listen`, `conn.SetBPF`, and `activeCaptures.LoadOrStore` |
| T-24 | `show dns lookup` | It sends a DNS query and writes the answer into Ze's own cache. `clear dns cache` exists to undo it | `request` | `resolver.go` `Resolver.ResolveWithTTL` calls `r.query` then `r.cache.put` |
| T-25 | `show ping` | It sends ICMP echo requests from the router | `request` | `ping.go` `handleShowPing` calls `doPing` |
| T-26 | `show traceroute` | It sends probes with a rising TTL and reads the ICMP errors they provoke | `request` | `traceroute.go` `handleTraceroute` calls `doTraceroute` |
| T-27 | `show probe-round` | It sends one parallel round of probes | `request` | `probe_round.go` `HandleProbeRound` calls `doProbeRound` |
| T-28 | `show tcp-check` | It opens a TCP connection to a remote host, which creates state on that host | `request` | `tcp_check.go` `HandleTCPCheck` calls `dialer.DialContext` |
| T-29 | `show system profile cpu` | It starts the runtime CPU profiler for up to 60 seconds and takes a process-wide lock, so a second profile is refused while it runs | `request` | `profile.go` `profileCPU` calls `pprof.StartCPUProfile` under `cpuProfileMu` |
| T-30 | `monitor ping`, `monitor traceroute` | They send ICMP echo requests and probes until the operator stops them | `request` | `stream.go` `NewPingSession` starts `streamPing`, which opens an ICMP socket and sends |
| T-31 | `resolve dns a`, `resolve dns aaaa`, `resolve dns ptr`, `resolve dns txt` | Each one sends a DNS query and writes the answer into Ze's own cache. A read-only operator therefore fills that cache | `request` | `resolve.go` `handleDNSA` calls `Resolver.ResolveA`, which calls `r.cache.put` |
| T-32 | `resolve ping`, `resolve traceroute` | They emit packets. L-1 already records them as undefended, and the axis settles the read half of that question | `request` | The same producers as T-25 and T-26 |

Three commands look like changers and are not. `show bgp irr check` reads the
cached IRR state and refetching is `update bgp irr asn`. `show system update`
reads a stored status and the fetch is `update system firmware check`. `show
policy test peer` runs the peer's filter chain over an operator-supplied UPDATE
and installs nothing.
<!-- source: internal/component/bgp/plugins/filter_irr/command.go -- showIRRCheck -->
<!-- source: internal/plugins/update-cmd/cmd/show.go -- handleShowSystemUpdate -->
<!-- source: internal/component/bgp/reactor/policy_dryrun.go -- PolicyDryRun -->

## Where the tree diverges from this page

Each row is a defect. The page states the rule and the tree does not meet it.
The read-verb rows are in the section above.

| # | Command or symbol | The rule it breaks | Evidence |
|---|-------------------|--------------------|----------|
| T-3 | `show bgp peer list` | No slot accepts both a fixed keyword and an operator-supplied value. `container peer` declares a mandatory selector and `list` is a sibling container carrying `ze:inherit "none"`, so a peer named `list` is unreachable | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`. Fix: `plan/immediate/spec-fixit-selector-narrows-through-a-pipe.md` |
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
| T-15 | `request subscribe`, `request config archive`, `request l2tp outgoing-call`, `request bgp rib inject` | `create` answers before the residual is reached. Each one brings something into existence | `subscribe.go` `handleSubscribe` calls `Subscriptions().Add`. `archive.go` `handleArchiveTrigger` writes a named archive entry. `outgoing_call.go` `handleOutgoingCall` calls `PlaceOutgoingCall`. `rib_commands.go` injects into the Adj-RIB-In |
| T-16 | `request unsubscribe`, `request cache expire`, `request bgp rib withdraw` | `delete` answers first. Each one removes something that existed | `subscribe.go` `handleUnsubscribe` calls `Subscriptions().Remove`. `ze-cli-cache-cmd.yang` `expire` removes a cached message at once. `rib_commands.go` withdraw removes the route. `withdraw` is also RFC 4271's word for the operation, which is the one defense on this row |
| T-17 | `request interface <n> mtu`, `request interface <n> mac`, `request log level`, `request cache retain` | `update` answers first. Each one rewrites a value held for something that exists | `manage.go` `handleInterfaceMTU` calls `iface.SetMTU`, and `handleInterfaceMAC` calls `iface.SetMACAddress`. `handlers.go` `setLevel` calls `slogutil.SetLevel`. `ze-cli-cache-cmd.yang` `retain` sets the no-evict flag. `request cache release` undoes that flag for an API caller and acknowledges a message for a cache consumer, so one command carries both verbs |
| T-18 | `request as112 healthcheck` | The family's premise is a change to the system, and this command changes nothing. It sends one DNS SOA query and answers `healthy`, so question 1 answers it: `show` | `health.go` `handleAS112Health`. It emits a packet, so L-1 governs the same choice for it |
| T-19 | `request peer <sel> flush`, `request quiesce` | Neither changes the system nor reports state. Each blocks until queued work drains, and "Choosing the verb" has no question that answers a barrier | `ze-peer-cmd.yang` `flush` waits until every queued update for a peer is sent. `ze-system-cmd.yang` `quiesce` blocks until every subsystem has drained its pending async work |
| T-20 | `update system firmware restart` | It fetches nothing and rewrites no data. It restarts the daemon into the staged binary, which is the residual: `request` | `selfupdate.go` `manualRestart` |
| T-21 | `clear l2tp session all\|id`, `clear l2tp tunnel all\|id`, `clear ospf neighbor`, `clear ospf process`, `clear isis adjacency`, `clear vpn ipsec sa`, `clear bgp rib out` | `clear` returns data to zero, and L-4 states that Ze never resets a session with it. The first eight close live sessions instead, and `clear bgp rib out` replays the Adj-RIB-Out, which returns nothing to zero | `ze-ospf-cmd.yang` `neighbor` and `process`, `ze-isis-cmd.yang` `adjacency`. `l2tp.go` `handleSessionTeardown`. `ipsec.go` `handleClearIPsecSA` calls `engine.TerminateAllSAs`. `ze-rib-cmd.yang` `out` re-advertises every route to a peer |

## Where this page diverges from what an operator expects

Each row is a design position rather than a defect. It has to be defended or
changed, and it is not a bug list.

| # | Position | The surprise | Defense |
|---|----------|--------------|---------|
| L-1 | `resolve ping` and `resolve traceroute` | Every reference CLI has `ping` bare or under a diagnostics verb. No CLI uses `resolve` as a root verb, and `docs/guide/command-catalogue.md` states that Ze keeps universal diagnostics bare | Undefended, and half of it is now settled. Both commands emit packets, so the axis rules them out of a read verb and T-32 records that. What stays open is the ROOT they take: a bare diagnostic root matches every reference CLI, and `request` is what the residual test gives them |
| L-2 | The peer selector carries no kind keyword | Every other Ze noun types its selector: `show interface name <n> detail`, `show l2tp session id <id>` | Earned. The bare form is the ExaBGP order an operator already types, and it is the order FRR publishes. Its cost is T-3, and the cost falls on enumerating actions alone |
| L-3 | `delete` carries two roles | It removes a config tree node AND a live kernel resource, so one word means two things | Earned, and stated in `verbs.go`. `create` has no config sense, so pairing it with a second deletion verb would add a word to carry no new meaning |
| L-4 | `clear` never resets a session | All five reference CLIs reset a BGP session with `clear` or `reset` | Earned AS A RULE and NOT TRUE TODAY, and the two must not be read together. The rule keeps one meaning for `clear`. The cost is that an operator's first guess fails, and the error message has to carry them to `request peer <sel> teardown`. The tree breaks it in nine places, listed at T-21. So the first guess currently SUCCEEDS everywhere except BGP. That is worse than either answer alone, because the word means one thing in one protocol and another everywhere else. Until T-21 is closed this row states a target, not behavior |
| L-5 | Rendering has no flag | Operators arriving from any of the five reach for a flag or a per-command keyword | Earned. Rendering belongs to the pipe layer on every command, so one operator set serves 363 commands and only the operator composes (`command-namespacing.md`) |
| L-6 | The name of the narrowing operator is OPEN | `plan/immediate/spec-fixit-selector-narrows-through-a-pipe.md` proposes one operator named `limit` for every display command, which would replace the field-named `\| peer <sel>` on `show bgp rib`. That reading of `limit` disagrees with the rule above, and it disagrees with the word's other uses | Undecided, and Thomas decides. `limit` already means a numeric row cap in three Ze surfaces, in FRR (`bgp listen limit`, `prefix-limit`) and in BIRD (`babel limit`, merge-paths `limit`). Ze also ships `\| first <n>` and `\| last <n>` beside it |
| L-7 | `request interface <name> up` and `down` | The admin flag is a field an operator reads back on the interface, which is the `update` test. It is also what drives the link transition, which is the residual | Undecided, and Thomas decides. T-17 moves `mtu` and `mac` because each carries a value the operator types. `up` and `down` carry none, so the word IS the value. Reading them as `update` produces `update interface name eth0 up`, and reading them as `request` keeps the pair beside `teardown` |
| L-8 | `send` is a root verb | It is a thirteenth canonical verb, where every other change class sits under one of twelve | Settled by Thomas on 2026-09-05, in two words: "send bgp". The axis decides it. `request` changes the system, and `send` puts a message outside it, so a send is not a request. The price is one entry in `command.Verbs` and one row in its golden test. Authorization costs nothing. `IsReadOnlyPath` allowlists the read verbs and answers false for every verb it does not name, so a new root verb counts as writing by default. A profile then denies it through its `Edit` section default. That default is the ZERO value, because `Deny` is `Action = iota`. So a profile that declares a `run` block and no `edit` block refuses `send`, with no entry naming it. `builtinReadOnlyProfile` is not the mechanism, because it has no non-test caller: the one shipped path that builds a profile reads operator config YANG. `ping` and `traceroute` are root verbs in every reference CLI, and `send` sits beside them |

## See also

- `command-namespacing.md` for where a command roots and which register a token
  belongs to.
- `root-namespace-grammar.md` for the seven feeders and what each checks.
- `ai/rules/cli.md` for the blocking directives this page explains.
- `docs/guide/command-catalogue.md` for the cross-vendor command mapping.
