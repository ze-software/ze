# Spec: a send names its destination

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 1/9 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas set the grammar for every command that puts an operator's message on a
wire. It is settled, and this spec designs to it rather than re-opening it.

```
send <protocol> <selector> raw [<type>] <encoding> <data>
send <protocol> <selector> <message> [fields...]
```

His words, in order. First the shape: "we send `<destination>` `<protocol>`
`<raw|protocol info>`". Then the reason for the discriminator: "we need to not
have clash between request system (subsystem) and request system (command on
host)", and "we need to have request `<send or another better word>` destination
...". Then the correction that fixed the token order: "with the protocol first
so we know how to process the selector". Then the root, in two words: "send
bgp".

Two examples, one per production:

```
send bgp 192.0.2.1 raw hex FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
send bgp * announce unicast 10.0.0.0/24 next-hop 10.0.0.1
```

### Why `send` is a word of its own

`request` is followed by SUBSYSTEM names. Its children in the merged tree are
`as112`, `bgp`, `cache`, `commit`, `config`, `halt`, `interface`, `l2tp`, `log`,
`ospf`, `peer`, `quiesce`, `reboot`, `reload`, `shutdown`, `subscribe` and
`unsubscribe`. Every one is a fixed keyword.

A destination is free-form text: an address, a configured peer name, an AS
pattern, a glob, a comma-separated list, or `*`. Writing `request <destination>
<protocol>` would put an arbitrary string where `bgp` already names a subsystem,
and no parser can separate the two: a peer named `bgp` and the BGP subsystem
would be one token. `send` is the word that keeps the two apart, the same shape
as the `type` key on the CA root.

`send` is also the correct word on the axis
`docs/architecture/cli/command-verbs.md` states. `request` changes the system,
and `send` puts a message OUTSIDE it. A send is therefore not a kind of request,
and that is why the word is a root verb rather than a noun under one.

### Why the protocol comes BEFORE the selector

Thomas's reason is the parser's: the protocol decides how the selector is read.
The repository's reason is stronger, and it is the one that makes this a rule
rather than a preference.

**A free-form value in the first slot is the exact shape that produced the `peer`
trouble.** A slot that can hold an operator-chosen name is a slot where a name
and a keyword compete, and the parser cannot tell which one arrived. That is
`show bgp peer list` (row T-3 of `docs/architecture/cli/command-verbs.md`), where
a peer named `list` is unreachable. Putting the destination first would rebuild
that slot one level up, under `send` instead of under `peer`.

Protocol-first makes the token after `send` come from a CLOSED set derived from
the registry, and the selector then sits behind a keyword that has already told
the parser how to read it. Three surfaces key on closed sets and all three get an
answer instead of a guess: completion enumerates the protocols, the grammar
checker reads a fixed child list, and the authorization profiles match a path
whose every keyword is known before a value arrives.

**It also puts the selector grammar where the registration pattern says it
belongs.** `bgp` takes a peer selector. A future `dns` takes a resolver address,
and a future raw-frame protocol takes an interface. Each protocol's own package
declares its own selector leaf, with its own type, its own validator and its own
completion function, on its own container. No central switch enumerates them and
nothing has to guess a selector's kind from the value's shape
(`ai/rules/principles.md`, registration over a central enumeration). That is the
difference between this grammar and a parser that sniffs.

The protocol token therefore is not machinery with one user
(`ai/rules/simplicity.md`). It is the token that types the value after it.

### The root-verb question, answered

`send` is a ROOT VERB. Thomas ruled the fork on 2026-09-05, in two words: "send
bgp". The grammar is `send <protocol> <selector> ...`, and no token stands in
front of it.

The verb set is stated at two sites, and the implementation phase edits both.

| Site | Edit |
|------|------|
| `internal/component/command/verbs.go` | `Verbs` gains `send` with the role `VerbAction`. It is the single canonical verb map, and its own comment states that the grammar gate and the plugin registration gate (`validateCommandName`) both derive from it, with no second hardcoded list. Twelve entries today, thirteen after |
| `internal/component/command/verbs_test.go` | `TestVerbRegistryCanonical` pins the same map as a golden `want` value and asserts its length, so the row is added there too |

`VerbAction` is the role, and the file declares three. `send` is a runtime
operational action. It is not `VerbRead`, because bytes leave the router. It is
not `VerbMutation`, which exists to gate config-tree mutation in engine path
form, and a send writes no config node.

**A root verb costs nothing in `IsReadOnlyPath`, and an earlier draft of this
spec priced it wrong.** `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) is an ALLOWLIST of the read
verbs: it cuts the first token, matches it against `show`, `monitor`, `resolve`,
`validate`, `help`, `system`, `plugin` and `rib`, and returns false for every
other token. A verb the predicate has never heard of therefore counts as writing,
so it fails closed and `send` needs no entry. The root-verb option is cheaper
than the price Thomas was quoted when he answered.

### What a read-only operator reaches

Read at the producers, in the order the flag travels. `LoadBuiltins` registers
each path with `ReadOnly: IsReadOnlyPath(name)`. `Dispatch` passes that flag to
`isAuthorized`. `Profile.Authorize` reads the `Run` section when the flag is true
and the `Edit` section when it is false. `builtinReadOnlyProfile`
(`internal/component/authz/authz.go`) declares `Edit: Section{Default: Deny}`
with no entries at all.

So the built-in read-only profile denies every `send` command, by the section
default and without an entry that names the verb. The answer is the same as under
`request`, and it is reached by two defaults that each fail closed rather than by
a rule someone remembered to write. AC-16 asserts it from the dispatcher, because
a property that holds by default is the one a later edit removes in silence.

### What this replaces

`plan/spec-fixit-selector-narrows-through-a-pipe.md` is committed at HEAD and
specifies `request peer <sel> <verb>` for the four peer action subtrees. **This
spec supersedes the ACTION-MOVE half of it and nothing else.** Concretely, it
replaces that spec's AC-5, its Phase 5, and the rows of its "Behavior to change"
list that move `raw`, `update`, `announce` and `withdraw` to `request peer`. Its
DISPLAY half stands whole: the pipe narrowing of the eight display nodes,
`show bgp peer list`, the `ze:inherit "none"` question and the `| peer` to
`| limit` rename are untouched by this spec and remain that spec's work.

The two do not collide in the tree. That spec's display half edits `show`
subtrees; this spec's move edits the top-level `peer` root, the bare `announce`
and `withdraw` roots, and `request cache`. `ze-peer-cmd.yang` is the one file
both touch, in different containers.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/command-verbs.md` - the verb contract this spec implements
  → Constraint: "An action names the objects it acts on with a positional selector, typed BEFORE the action word", and "Peer commands are the one exception, and they drop the selector kind". The new grammar keeps both. The exception moves from `peer` to the protocol keyword, and it gets a better reason: the protocol token is what tells the parser how to read the value, which is the job a selector-kind keyword does elsewhere.
  → Constraint: "Every argument a handler reads MUST be declared in the schema. An undeclared argument reaches the handler and reaches nothing else." This decides the undeclared-argument question below, and row T-5 already records `peer raw` as breaking it.
  → Constraint: row T-3 is the failure the token order avoids: a slot that accepts both an operator-supplied value and a fixed keyword makes a peer named `list` unreachable. Row T-4 records the four commands at a second peer action root and names the superseded spec as their fix; this spec becomes that row's fix instead. **This spec edits the page for the root-verb decision alone: the `send` row, the change axis, the send shape and row L-8. The T-4 and T-5 repoint is a concurrent session's or a later one's.**
  → Decision: `request` is the residual of `create`, `delete` and `update`. A send creates nothing that outlives it, deletes nothing and rewrites nothing, so the residual is the correct family and `send` is a noun under it rather than a fifth state-changing verb.
- [ ] `docs/architecture/exabgp-bridge.md` - the Design doc `bridgeplugin/internal.go` declares, and the page that settles the compatibility question
  → Constraint: "Down, `ExabgpToZebgpCommand` reads one ExaBGP line and writes one ze CLI command." The translated half is decoupled: an ExaBGP script never sees Ze's spelling for it.
  → Constraint: "A line that does not match is returned unchanged, so it reaches ze's CLI verbatim. This makes ze's CLI a second external contract." That is the half this spec moves, and it is why the exemption exists.
  → Constraint: the page's three-row table is what decides the order of the bridge work. The six announce and withdraw forms plus `help` reach a script through the PASSTHROUGH, so a rename breaks scripts. `peer-update` is TRANSLATOR OUTPUT and costs one edit. `peer-raw` is NEITHER, "because the translator never writes it", so it costs one edit and no script is affected.
  → Constraint: "`ExtractPeerAddress` requires the literal prefix `peer ` and answers the empty string for any other command. Its three callers read that empty string as 'nothing to flush' and continue." A grammar move creates that silence, so the move carries the repair.
- [ ] `docs/architecture/cli/command-namespacing.md` - the Design doc `checker.go` and `cligrammar.go` both declare
  → Constraint: both files name this page in their `// Design:` header, so a change to `bridgeSurface` or to the exemption count owes an edit here in the same work.
- [ ] `docs/architecture/api/commands.md` - the Design doc `raw.go`, `update_text.go`, `cache.go` and `peer.go` declare
  → Constraint: it carries 28 occurrences of the literal `peer <...>` command shape, the largest concentration in the docs, so it is rewritten rather than appended to.
- [ ] `docs/architecture/api/update-syntax.md` - the UPDATE text grammar
  → Constraint: its Core Syntax line is `update <encoding> [<attr-sections>]... [nlri <family> add ...]` and it carries a source anchor at `ParseUpdateText`. The tokens BEFORE `update` change and the tokens after do not, so every example gains a prefix and the grammar line is otherwise unchanged.
- [ ] `docs/architecture/core-design.md` - the Design doc every `internal/exabgp/bridge/` file declares
  → Constraint: naming it is owed by the anchor audit because three changed files declare it. The translation detail belongs on `exabgp-bridge.md`, which is the page a bridge reader opens; this page is named as unaffected beyond a pointer.
- [ ] `docs/architecture/api/process-protocol.md` - the Design doc `plugin/server/command.go` declares
  → Constraint: `RequiresSelector` and the selector-extraction contract are described from this page's side, so a change to which commands carry the guard is named here.
- [ ] `docs/architecture/bgp/on-demand-origination.md` - the Design doc `announce.go` declares
  → Constraint: it documents the two announce paths, bare and peer-scoped. The new grammar collapses them to one, so the page's central claim changes.
- [ ] `docs/guide/mcp/overview.md` - the Design doc `cmd/ze/help_ai.go` declares
  → Constraint: `help_ai.go` prints the old spelling three times, in the grammar line and in two worked examples, so the generator and the page it declares move together.
- [ ] `ai/rules/cli.md` - the CLI grammar directives
  → Constraint: "A command addressing one member of a set MUST type the selector with `name`, `id`, `index`, `address`, `type`, `key` ... Peer commands are the one exception: they address a peer positionally." The exception is RESTATED for the send family: the value is typed by the protocol keyword rather than by a selector kind, and the protocol is what makes it readable.
  → Constraint: "`selector` MUST NOT be exposed as an operator keyword." The selector is positional and anchored to the protocol container, so the leaf's name is never typed. This is what lets the model keep one name for the concept (see Key Design Decisions).
  → Decision: "Ze is unreleased, so an unreleased grammar MUST be replaced outright rather than deprecated." No alias, no fallback, no compatibility spelling for any old path.
  → Constraint: "a rename of a programmatic command path breaks the wire, so every programmatic sender MUST be found first." 207 occurrences in 70 `.ci` files send these paths today, and `./le ci-dispatch` is the gate that reads them.
- [ ] `ai/patterns/cli-command.md` - the structural template
  → Constraint: its Peer Selector Mechanism section and its Full Command Inventory publish the shapes this spec moves; row T-12 already records the inventory as stale. The page is edited for the send family alone.
- [ ] `ai/rules/principles.md` - the zero-value and registration directives
  → Constraint: "a value that is silently wrong MUST NOT be reachable". Three producers in this spec's blast radius answer a zero a caller reads as absence: `PeerSelector` returns `*` for an unbound selector, `ExtractPeerAddress` returns the empty string for an unrecognized command, and the bridge's passthrough returns an untranslated line that dies at the dispatcher as an unknown command. Each is addressed rather than noted.
  → Constraint: "A new feature MUST register itself and be discovered." Each protocol declares its own selector leaf on its own container, so adding a protocol edits no central list and no code guesses a selector's kind from its value.
- [ ] `ai/rules/simplicity.md` - the shape of the fix
  → Constraint: the fix cuts machinery, never correctness. It is what chooses "teach the translator the bare forms" over "freeze six spellings because a script might type them": the first removes a coupling, the second preserves one.
- [ ] `ai/rules/spec-no-code.md` - spec form
  → Constraint: no code snippet in any language. Command lines and grammar productions are input samples, not code.

**Key insights:**
- The selector and the peer selector are ONE concept in the model. `applyExtractedSelectors` copies the leaf named `selector` into the context's peer field, `PeerSelector` reads it, and `RequiresSelector` guards it. The new grammar changes WHERE the operator types it, not what it is.
- `matchCommandTokens` already binds a bare token sitting between two key tokens to a leaf anchored to the earlier one, and `anchoredDef` reads the model's own answer for which leaf that is. `peer <sel> announce unicast` works by that mechanism today, so `bgp <sel> raw` needs no new dispatch machinery. Protocol-first suits it better than destination-first: the anchor word is the protocol, which is exactly the container that declares the leaf.
- `inheritArgDefs` moves a container's leaves DOWN to every command below it, anchored to the container's name, and clears the container's own list. A selector leaf declared on `bgp` therefore appears on each BGP send command's published argument list, which is what makes completion and usage state it.
- The grammar gate never checks the nodes this spec moves. `ExemptCategory` returns them as category E1 before `CheckName` and `CheckNode` run, so eight of the nine wire methods have never been checked against any rule R1 to R9.
- `handleRaw` is the ONLY member of the population that narrows to one peer. `handleUpdate`, the three announce forms, the three withdraw forms and `handleBgpCacheForward` all fan out, and the shipped bridge fixtures send a wildcard on purpose.
- The nine exempt methods reach a script three different ways, and the way decides what a rename costs. The bridge page's table is the statement of it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/raw/raw.go` - `handleRaw` calls `ResolveSinglePeer`, then reads `[<type>] <encoding> <data>` from `args`. It registers `RequiresSelector: true`. Its own doc comment is where the argument grammar is written down.
- [ ] `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang` - declares the top-level `container peer` (with the description the other three modules share) and `container raw` under it, carrying `leaf selector { mandatory true; }` and nothing else.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` - `handleUpdate` switches on the first argument as the encoding (`text`, `hex`, `b64`, `cursor`) and parses a route expression from the tail. It reads `PeerSelector` and passes it to `selector.ParseDefault`, so it FANS OUT.
- [ ] `internal/component/bgp/plugins/cmd/update/yang/ze-update-cmd.yang` - `container peer / container update` with `leaf selector { mandatory true; }` alone.
- [ ] `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - declares `announce-forms` and `withdraw-forms` as groupings, each instantiated TWICE: bare (reaching every peer) and under `container peer`, which declares the shared `leaf selector`. Every argument of every form is already declared as a typed leaf or a modifier group.
- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go` - the six forms register with NO `RequiresSelector`, and each reads `PeerSelector`, which answers `*` when nothing bound. That is how the bare path reaches every peer.
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go` - `handleCacheForwardRPC` reads the first argument as a cache id and the second as a peer selector; `handleBgpCacheForward` parses that selector and calls `ForwardUpdate`, which its own comment describes as putting "a whole UPDATE on each matched peer's wire". Neither argument is declared.
- [ ] `internal/component/plugin/server/command.go` - `ResolveSinglePeer` refuses the empty selector, `*` and every exclusion selector, refuses a selector matching more than one peer, passes a bare ADDRESS through when it matches no configured peer, and refuses a non-address selector that matches none. `PeerSelector` answers `*` when the context's peer field is empty. `applyExtractedSelectors` copies the bound `selector` leaf into that field. `matchCommandTokens` binds interior selector slots. `Dispatch` refuses a `RequiresSelector` command with no bound selector.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - the reactor adapter's `SendRawMessage` answers `ErrPeerNotFound` for an address with no peer, and refuses a sender the peer does not permit. Raw fails closed at the reactor as well as at resolution.
- [ ] `internal/component/command/grammar/checker.go` - `bridgeSurface` is a map of NINE literal wire-method strings; `ExemptCategory` returns them as E1 "bridge". `CheckNode` fires R6 only when a node has a mandatory free-form argument AND a subcommand child AND its own name is not one of `name`, `id`, `index`, `address`, `type`, `key`.
- [ ] `internal/le/cligrammar/cligrammar.go` - the tree walk calls `CheckName` and `CheckNode` only for a node with a wire method, and SKIPS both entirely when `ExemptCategory` answers. An exempt command is unchecked against every rule, not merely against verb-first.
- [ ] `internal/component/config/yang/command.go` - `inheritArgDefs` and `appendAnchored` push a container's leaves down to each command below, anchored to the container's name, and warn when a container states a value no command takes.
- [ ] `internal/exabgp/bridge/bridge_command.go` - `ExabgpToZebgpCommand` matches `neighbor <address> <rest>` with a regular expression, translates the match, and RETURNS ANY OTHER LINE UNCHANGED. Ten functions build the Ze-side string across fourteen sites: the passthrough in `ExabgpToZebgpCommand` itself, `convertAnnounce`, `convertWithdraw`, `convertAnnounceFamily`, `convertWithdrawFamily`, `convertAnnounceSRPolicy`, `convertWithdrawSRPolicy`, `convertAnnounceFlowSpec`, `convertWithdrawFlowSpec` and `convertAnnounceWithFamily`. None of them writes a `raw` command.
- [ ] `internal/exabgp/bridge/bridge_muxconn.go` - `ExtractPeerAddress` requires the literal prefix `peer ` and answers the empty string otherwise; `IsRouteCommand` answers on the substring `update text`, anywhere in the string.
- [ ] `internal/exabgp/bridge/bridge.go` - after dispatching a route command it calls `IsRouteCommand`, then `ExtractPeerAddress`, then flushes and blocks on the drain when the address is non-empty.
- [ ] `internal/plugins/exabgp/main_sdk.go` - the same shape, and it builds the flush as `request peer <addr> flush`.
- [ ] `internal/plugins/exabgp/bridgeplugin/internal.go` - the same shape again, third copy.
- [ ] `internal/test/fixture/plugin_fixture_06_exec.go` - the shipped bridge helper writes ONE line, `neighbor 127.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1 origin igp local-preference 100`. It is the translated half. No shipped fixture drives the passthrough half.
- [ ] `test/plugin/exabgp-bridge-internal.ci` and `test/plugin/exabgp-bridge-sdk.ci` - each drives the bridge end to end from that helper and asserts the resulting wire UPDATE in hex. Each also carries a Ze-side expectation line spelling `peer * update text ...`.
- [ ] `test/ui/cli-announce-reaches-the-wire.ci` - the shape a `.ci` uses to prove a command reaches the wire: a compiled fixture starts the daemon and a scripted peer, and the peer holds the assertion.
- [ ] `cmd/ze/help_ai.go` - publishes `peer <selector> update text ...` as the grammar line and `peer * update text ...` in two worked examples, for an agent reading the AI help.
- [ ] `plan/journal/command-takes-an-untyped-positional-value.md` - four rows. The 2026-09-04 row names `peer raw`, `peer update` and `request commit`, and records the reason it was not fixed: the superseded spec "changes where the command lives and not what its schema declares".

**Behavior to preserve:**
- The BGP selector vocabulary: an address, a configured peer name, an AS pattern such as `as65001`, a glob, a comma-separated list, or `*`. `selector.ParseDefault` reads it and this spec does not change it.
- `raw` cannot reach more than one session. `ResolveSinglePeer` is what enforces it, and it stays.
- The fan-out of `update`, `announce`, `withdraw` and `cache forward`. Each acts on every peer its selector matches.
- Every wire method string. This spec re-paths commands; it renames no `ze-bgp:` method, so a plugin dispatching by wire method is unaffected.
- The ExaBGP line protocol the bridge READS, in both its forms: the `neighbor`-prefixed lines the translator handles, and the bare lines that reach the dispatcher through the passthrough.
- The UPDATE text grammar from the encoding word onward, and every `.ci` hex expectation over the resulting wire bytes.

**Behavior to change:**
- Nine wire methods, reached at fifteen paths, move under `send bgp <selector> <form>`. The old paths match nothing.
- The selector becomes mandatory on every one of them, and each registers `RequiresSelector: true`. `announce unicast <prefix>` with no selector is refused; `send bgp * announce unicast <prefix>` is how an operator reaches every peer.
- The announce and withdraw groupings are instantiated ONCE instead of twice, because the selector is now always present.
- Every argument `handleRaw`, `handleUpdate` and `handleCacheForwardRPC` read is declared in YANG.
- The translator learns the BARE ExaBGP forms, so a script's `announce route ...` and `withdraw route ...` are translated rather than passed through.
- The passthrough narrows: an unrecognized line is refused BY NAME instead of being forwarded to Ze's dispatcher as an unknown command.
- Eight wire methods leave `bridgeSurface`, so the grammar gate checks them for the first time.
- The bridge's fourteen construction sites emit the new Ze-side grammar, and the selector they used travels with the command instead of being re-parsed out of it.

**A defect this spec does NOT fix, and does not create:** `main_sdk.go` builds
`request peer <addr> flush`, the new grammar, four lines inside an `if` whose
condition came from `ExtractPeerAddress` parsing the OLD `peer <addr> ...` shape.
Two grammar generations in one function. It works today because the translator
does emit that shape, so the prefix matches. It is recorded here because it tells
the reader the migration is real and partly done, not hypothetical.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Three entry points reach the same dispatcher, and all three carry the command as
TEXT.

| Entry | Format at entry |
|-------|-----------------|
| An operator at the CLI or over SSH | `send bgp 192.0.2.1 raw hex FFFF...` as typed tokens |
| A plugin over the process protocol | the same string inside a `ze-plugin-engine:dispatch-command` JSON payload |
| The ExaBGP bridge | ExaBGP line-protocol text on the plugin's stdout, either translated or (today) passed through unchanged |

### Transformation Path

1. `tokenize` splits the input. `matchBuiltinTokens` finds the longest matching
   command prefix. `anchoredDef` binds the token after `bgp` to the selector leaf
   that the `bgp` container declares, and removes it from the args.
2. `validateCommandArgs` binds the remaining positionals to the declared leaves,
   typed. Today the raw, update and cache-forward tails skip this because no leaf
   declares them; after this spec they do not.
3. `Dispatch` refuses the command when `RequiresSelector` is set and no selector
   bound, then authorizes, then calls the handler.
4. `applyExtractedSelectors` copies the selector into the context's peer field.
5. The handler resolves it: `ResolveSinglePeer` for `raw`,
   `selector.ParseDefault` plus the reactor's own matching for the rest.
6. The reactor encodes and writes: `SendRawMessage`, the update batch path,
   `announceAndTrack`, or `ForwardUpdate`.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ dispatcher | text command, YANG-declared tokens and leaves | No |
| Plugin ↔ engine | `dispatch-command` JSON carrying the command string | No |
| ExaBGP process ↔ bridge, translated forms | ExaBGP line protocol, external, decoupled from Ze's CLI | No |
| ExaBGP process ↔ bridge, passthrough forms | ExaBGP line protocol, external, COUPLED to Ze's CLI today | No |
| Bridge ↔ engine | a Ze command string the bridge constructs | No |
| Engine ↔ reactor | typed selector plus payload or route batch | No |
| Reactor ↔ wire | pooled buffers, `WriteTo(buf, off)` | No |

### Integration Points

- `internal/component/plugin/server/command.go` - the dispatcher binds the
  selector through the existing anchored-leaf mechanism. No new machinery.
- `internal/component/command/grammar/checker.go` - `bridgeSurface` shrinks, so
  the moved commands enter the gate's checked population.
- `internal/exabgp/bridge/bridge_command.go` - the one place that turns ExaBGP
  input into a Ze command, and the one place the passthrough lives.
- `internal/le/cligrammar` and `./le ci-dispatch` - the two gates that read the
  command tree and every command string the repository sends to its own daemon.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## The population

Enumerated from the merged tree, not from a list. The tool that reads the live
handlers and schemas is `./le command list`, which reports 363 commands and one
row per wire method.

**The test, two legs, both must be true:**

1. The operator supplies the MESSAGE: its bytes, or the protocol fields Ze
   encodes into one. Not merely a switch that makes Ze compose its own.
2. The command names WHERE the message goes, and that destination is the
   operator's to choose.

### In: nine wire methods, fifteen paths

| Wire method | Paths today | Under the new grammar | Arity today |
|-------------|-------------|-----------------------|-------------|
| `ze-bgp:peer-raw` | `peer raw <sel> [<type>] <enc> <data>` | `send bgp <sel> raw [type <t>] <enc> <data>` | exactly one, enforced |
| `ze-bgp:peer-update` | `peer update <sel> <encoding> ...` | `send bgp <sel> update <encoding> ...` | fan-out |
| `ze-bgp:announce-unicast` | `announce unicast ...` and `peer <sel> announce unicast ...` | `send bgp <sel> announce unicast ...` | fan-out |
| `ze-bgp:announce-blackhole` | the same two paths | `send bgp <sel> announce blackhole ...` | fan-out |
| `ze-bgp:announce-flowspec` | the same two paths | `send bgp <sel> announce flowspec ...` | fan-out |
| `ze-bgp:withdraw-tag` | `withdraw tag ...` and `peer <sel> withdraw tag ...` | `send bgp <sel> withdraw tag ...` | fan-out |
| `ze-bgp:withdraw-id` | the same two paths | `send bgp <sel> withdraw id <id>` | fan-out |
| `ze-bgp:withdraw-all` | the same two paths | `send bgp <sel> withdraw all` | fan-out |
| `ze-bgp:cache-forward` | `request cache forward <id> <sel>` | `send bgp <sel> cached <id>` | fan-out |

**Every row is a RESPELLING, not a new command.** `send bgp <sel> raw
<bytes>` is the same operation as `peer <sel> raw <bytes>`, reached by the same
handler through the same wire method, with the same guard and the same
resolution. Nine wire methods exist before this spec and nine exist after. The
work is a move, and the acceptance criteria are written as "the same behavior at
a different path", never as a feature.

`request cache forward` meets both legs: the operator supplies the message by
REFERENCE, a cache id, rather than by value, and names the peers it goes to.
`handleBgpCacheForward` puts a whole UPDATE on each matched peer's wire. Leaving
it where it sits would keep a second place where a BGP send names its
destination, and a second spelling of one concept is what habit 1 of
`ai/rules/writing.md` bans. Its three siblings, `retain`, `release` and `expire`,
act on the cache entry and send nothing, so they stay under `request cache`.

### Out: named, with the leg that fails

| Command | Leg that fails | Why |
|---------|----------------|-----|
| `request peer <sel> teardown\|pause\|resume\|flush\|refresh\|borr\|eorr\|clear soft` | 1 | Ze composes the message. The operator supplies a subcode or a family, never the message |
| `request bgp rib inject\|withdraw` | 2 | `injectRoute` inserts into the local adj-rib-in "as if received from a peer". Nothing reaches a wire, and its first positional names the pretended SOURCE |
| `debug ip ospf inject opaque`, `debug ipv6 ospf inject lsa` | 2 | The crafted LSA goes into this router's LSDB and floods the area. No destination is named, and `debug` is a distinct verb class with a fail-closed arming switch |
| `request l2tp outgoing-call remote <ip> called <n>` | 1 | The operator names a call target; Ze composes the L2TP control exchange |
| `resolve ping`, `resolve traceroute`, `monitor ping`, `monitor traceroute`, `show ping`, `show traceroute` | 1 | A probe, not an operator-composed message. Their verb placement is already an open divergence (L-1), and this spec does not touch it |
| `resolve dns a\|aaaa\|ptr\|txt`, `resolve cymru\|irr\|peeringdb`, `request as112 healthcheck` | 1 | A query Ze composes to a resolver it was configured with |
| `request subscribe`, `request unsubscribe`, every `plugin *` and `system *` | 1 and 2 | Process-boundary directives, exemption category E2 |

## The selector

**Who owns it.** The protocol container declares it. `container bgp` under `send`
carries the peer-selector leaf, with the BGP vocabulary in its description and
its completion. A second protocol declares a different leaf, of a different type,
on its own container: a resolver address, an interface name, a tunnel id. Nothing
central enumerates the protocols and nothing infers a selector's kind from the
value's shape, which is what the token order buys.

**What resolves it.** `applyExtractedSelectors` copies the bound leaf into the
context's peer field; `PeerSelector` reads it; `selector.ParseDefault` types it;
`PeersMatching` applies it to the reactor's peer list. `raw` alone then calls
`ResolveSinglePeer`, which narrows to exactly one.

**What happens when it names nothing.**

| Case | Today | Under the new grammar |
|------|-------|-----------------------|
| No selector typed at all | `PeerSelector` answers `*`, so `announce unicast <prefix>` reaches EVERY peer | refused: the leaf is mandatory and every send registers `RequiresSelector: true` |
| A bare ADDRESS matching no configured peer | `ResolveSinglePeer` passes it through; the reactor answers `ErrPeerNotFound` | unchanged |
| A NAME, ASN or glob matching no peer | refused by name, pointing at `show bgp peer list` | unchanged |
| An exclusion selector such as `!edge1` | refused for `raw`; the fan-out members apply it | unchanged |

The row that matters is the first. `PeerSelector` answering `*` for an unbound
selector is a zero that behaves correctly: a caller cannot tell "the operator
asked for every peer" from "nothing bound". Making the selector mandatory removes
the ambiguity for the send family, which is why `RequiresSelector` is an
acceptance criterion rather than an implementation note.

**How the single-session property survives.** It is `raw`'s alone. `handleUpdate`,
the six announce and withdraw forms and `handleBgpCacheForward` all fan out
today, and the shipped bridge fixtures send a wildcard on purpose. The grammar
therefore does NOT carry the arity, and it must not pretend to: the selector slot
accepts the whole vocabulary, and `handleRaw` narrows it with `ResolveSinglePeer`
exactly as it does now. A test asserts that `send bgp * raw ...` is still
refused, because that refusal is the whole safety property and it is now two
tokens further from the operator's eye.

## `raw` as a per-protocol escape hatch

`raw` is a BGP verb today. Under the new grammar it is the first slot of the
first production, so every protocol may offer one beside its structured forms.

**What a protocol must provide to have a `raw`:**

| Requirement | Why |
|-------------|-----|
| A selector leaf on its own container, and a resolver from that value to exactly ONE live session it owns | `raw` bypasses validation, so an unvalidated payload on more than one session is a fan-out of damage. This is the arity property, and it is why a `raw` is refusable |
| A send path that takes an opaque payload and does not interpret it | Otherwise it is a structured form with a hex spelling |
| A stated framing rule: which bytes Ze adds and which the operator supplies | BGP's is "a message-type keyword means Ze builds the header; without one, the data carries the marker and header too". Every protocol owes the equivalent sentence, in its `ze:help` |
| A permission answer at the reactor or its equivalent | BGP's `rawOrigin` requires the sending process to be attached to that peer, and an operator is not gated. A protocol with no attachment model must say what it gates on |

**A protocol with no structured form MAY offer `raw` alone.** `raw` is not sugar
over the structured forms and does not depend on them: it is the escape hatch
that exists precisely where no structured form does, which is what makes it
useful for conformance testing and fuzzing. The reverse does not hold: a protocol
that cannot resolve its selector to one session MUST NOT offer `raw`, and offers
its structured forms only.

## The undeclared arguments

`plan/journal/command-takes-an-untyped-positional-value.md` records the class,
and its 2026-09-04 row names `peer raw`, `peer update` and `request commit`.
**This spec fixes the two that are in its population, here, in the same work.**

The row's own stated reason for deferring was that the superseded spec "changes
where the command lives and not what its schema declares". That reason stops
holding: this spec REWRITES those YANG nodes and publishes a path no operator has
typed before. Moving a command without declaring its arguments would put a
knowingly incomplete grammar on a brand-new path, where completion offers nothing
after the form word and the generated usage line states a command that takes no
arguments. `docs/architecture/cli/command-verbs.md` states the rule without an
exception: "Every argument a handler reads MUST be declared in the schema."

| Command | What the handler reads today | What the schema declares after |
|---------|------------------------------|--------------------------------|
| `raw` | `[<type>] <encoding> <data>` from `args` | an optional `type` group over the five message-type keywords, an `encoding` enumeration of `hex` and `b64`, and a `data` leaf |
| `update` | the encoding word, then a whole route expression | an `encoding` enumeration of `text`, `hex`, `b64` and `cursor`; the route expression stays a tail the parser owns, and the schema says so rather than staying silent |
| `cache forward` | a cache id, then a peer selector | an `id` leaf; the selector becomes the protocol container's selector |

The update route expression is the one place a full declaration is not reachable
in this spec: `ParseUpdateText` accepts a recursive attribute and NLRI grammar
that no flat leaf list states. What is owed and delivered is the encoding word as
a typed enumeration, and a `ze:help` naming
`docs/architecture/api/update-syntax.md` as the grammar's home. That is not a
deferral of the row: the row is about arguments the model does not know exist,
and after this spec the model knows the encoding exists, knows its four values,
and states where the tail's grammar is written. `request commit` keeps its row:
it sends nothing and is not in this population.

## The ExaBGP bridge

Thomas raised this from a question to a requirement: "make sure the exabgp
bridge is updated". It is a phase with its own acceptance criteria, not a risk
row. `docs/architecture/exabgp-bridge.md` carries the facts below, and it was
silent on all of them until 2026-09-04.

### Three ways a script reaches a command, and three different costs

`ExabgpToZebgpCommand` matches `neighbor <address> <rest>` with a regular
expression. A line that matches is TRANSLATED. A line that does not match is
RETURNED UNCHANGED and reaches Ze's dispatcher verbatim. That branch, plus what
the translator actually writes, splits the nine exempt wire methods three ways.
The bridge page states the split as a table, and it is what orders the work.

| Group | Wire methods | How a script reaches it | Cost of the move |
|-------|--------------|-------------------------|------------------|
| Passthrough | the six `announce` and `withdraw` forms, and `help` | the script writes the bare form and it arrives unchanged | a break for every script that writes the bare form, UNLESS the translator learns those forms first |
| Translator output | `ze-bgp:peer-update` | the script writes `neighbor <address> announce ...` | one edit in the translator; no script notices |
| Neither | `ze-bgp:peer-raw` | it does not. The translator never writes a `raw` command, and no ExaBGP form produces one | one edit, and no script is affected |

`peer raw` is exempt because it starts with a noun, not because the line protocol
names it. So its respelling is free, and the same is true of `peer update` once
the translator is edited. Only the passthrough group carries a real
compatibility question, and the next section answers it.

### The passthrough is already broken for real ExaBGP scripts

Measured, not assumed. ExaBGP's own bare form is `announce route <prefix>
next-hop <nh>`. Ze has no `announce route` command: `./le command list` reports
`announce blackhole`, `announce flowspec` and `announce unicast`, and
`convertAnnounce`, which knows the `announce route` spelling, is reached only
from the `neighbor` branch. So a genuine ExaBGP bare line passes through
untranslated and matches nothing.

What the passthrough actually preserves is not ExaBGP's bare vocabulary. It is
ZE's, for a script written against Ze's spelling. The coupling exists, and the
thing it protects is not the thing it appears to protect.

### The design: teach the translator the bare forms

This turns a compatibility break into an addition, and it is the answer
`ai/rules/simplicity.md` asks for, because it removes a coupling rather than
adding a compatibility layer.

`ExabgpToZebgpCommand` gains the bare ExaBGP forms beside the `neighbor`-prefixed
ones it already handles: `announce route ...`, `withdraw route ...` and the
family forms, each translating to `send bgp * ...`, since a bare ExaBGP
command addresses every neighbor and `*` is how the new grammar says that. Three
things follow, and all three are gains:

1. A real ExaBGP script's bare form starts working. Today it does not.
2. The six spellings stop being an external contract, so they move with the rest
   of the grammar and leave `bridgeSurface`.
3. The passthrough stops being the mechanism that carries them, so it can be
   narrowed to what it is actually for.

**Narrowing the passthrough is a behavior change and carries its own criterion.**
After the translator learns the bare forms, the only line that legitimately
reaches Ze's dispatcher unchanged is `help`, which is the bridge's own word and
the one member `bridgeSurface` keeps. Every other unrecognized line is refused BY
NAME, naming the line the script wrote. Today such a line is forwarded and dies
as an unknown command with no mention of the bridge, which is the same silence
`ai/rules/principles.md` names. The narrowing removes an undocumented escape
hatch by which a script could type any Ze command; that removal is stated in
Known Limitations rather than discovered later.

**Rejected alternative: freeze the six spellings.** Keeping `announce unicast`
and `withdraw all` where they are, so a passthrough script still reaches them,
would exempt six of the fifteen paths from the point of the spec, leave two
places where a BGP send names its destination, and preserve a contract that does
not carry ExaBGP's own vocabulary. It also leaves the bare ExaBGP form broken.

**Rejected alternative: take the break to Thomas.** That is the right route for a
compatibility break with no design around it. This one has a design around it,
and the design also fixes an existing gap, so putting it to him would be asking
permission to do less.

### The construction sites

Ten functions in `bridge_command.go` build the Ze-side string across fourteen
sites: the passthrough branch of `ExabgpToZebgpCommand`, `convertAnnounce`,
`convertWithdraw`, `convertAnnounceFamily`, `convertWithdrawFamily`,
`convertAnnounceSRPolicy`, `convertWithdrawSRPolicy`, `convertAnnounceFlowSpec`,
`convertWithdrawFlowSpec` and `convertAnnounceWithFamily`. Each emits the new
grammar, and each already knows the selector it is writing.

### The site that bites, and it is not a builder

`ExtractPeerAddress` routes on the literal prefix `peer ` and answers the empty
string when it does not match. Its three callers, in `bridge.go`, `main_sdk.go`
and `bridgeplugin/internal.go`, each read that empty string as "nothing to flush"
and continue. `IsRouteCommand` answers on the substring `update text`, which is
position-independent and keeps answering true after the move. So the two disagree
the moment the leading token changes: every route command is still recognized as
one, and every flush after it is skipped. No error, no log line, no red, and the
forward pool stops draining on the one path that needs it.

That is `ai/rules/principles.md`'s first directive exactly, and the change is what
creates it, so the change carries it. Three things are owed, and they are
separable:

1. The flush still happens, proven by a test that asserts it REACHES the peer,
   driven from the bridge's INPUT side with ExaBGP text.
2. The routing decision stops being a re-parse. The translator holds the selector
   at every one of the fourteen sites and then throws it away, so the caller
   re-derives it from a string it built a moment earlier. Returning the selector
   with the command is the fix, and it removes the prefix coupling rather than
   updating it.
3. Whichever way 2 goes, no helper may answer an empty string into a caller that
   reads empty as absence. Either it reports a failure the caller must handle, or
   it stops existing.

### The exemption

`bridgeSurface` holds nine wire methods and its comment claims "a documented set,
not an ad-hoc allowlist". Judged: it is keyed on the wire method rather than on a
command name, which is the sense in which it is structural, and it is a map of
nine literal strings, which is the sense in which it is an allowlist. Both are
true, and what decides its future is the reason each member carries, which is the
three-row table above.

Eight members lose their reason under this design: `peer-raw` never had the line
protocol's reason at all, `peer-update` is produced by the translator, and the six
passthrough forms become translator output too. They leave the map, and the gate
checks them for the first time. `ze-bgp:help` keeps the passthrough reason, so E1
survives with exactly one member and its comment is rewritten to say what it now
covers. An allowlist that outlives its reason is the thing this repo journals, so
the reason is restated at the one entry that keeps it.

### What an ExaBGP user must not notice

Their config and their scripts are unchanged, and the `neighbor`-prefixed forms
prove it directly: `test/plugin/exabgp-bridge-internal.ci` and
`test/plugin/exabgp-bridge-sdk.ci` each drive the bridge end to end from an
ExaBGP-format helper and assert the resulting UPDATE in hex on a scripted peer.
Those two fixtures are the goal validation, they exist today, and their hex
expectations MUST NOT change. Their Ze-side expectation line does change, because
that line is the internal side.

Neither fixture drives the bare forms, so the passthrough half has no test today.
That gap is closed by a new fixture rather than left, because the bare form is
exactly what this design changes.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The selector binds through the existing anchored-leaf mechanism with no dispatcher change | `matchCommandTokens` and `anchoredDef` already bind `peer <sel> announce unicast` this way, and the anchor is the container that declares the leaf, which is now `bgp` | new dispatch machinery is needed, which changes the size of the work | a wiring test that dispatches `send bgp <sel> raw hex ...` before any handler moves | confirmed: `TestSendRawReachesOnePeerAtItsNewPath` dispatches the new path through the real server and the selector reaches `CommandContext.Peer`. No dispatcher file is edited |
| A-2 | The moved nodes pass R1 to R9 with no gate change once they leave `bridgeSurface` | `CheckNode` fires R6 only for a mandatory free-form argument on a node with a subcommand child whose name is not a selector kind; the send forms carry modifier groups and leaves, not subcommands | the gate needs a rule amendment, which is a separate design decision | `./le cli-grammar` on the moved tree, before the exemption is removed and after | unvalidated |
| A-3 | The selector leaf keeps the model's existing name, so no second spelling enters `applyExtractedSelectors` | `selectorLeaf` is the one constant binding a YANG leaf to the context's peer field; six call sites read `ResolveSinglePeer` and five are outside this population | a rename touches five unrelated handlers or adds a second name for one concept | grep for `selectorLeaf` and `PeerSelector` after the move; no new spelling appears | unvalidated |
| A-4 | Every argument the three under-declared handlers read can be stated as a typed leaf, except the update route expression | the announce and withdraw forms already declare theirs; `handleRaw` reads three tokens; `handleCacheForwardRPC` reads two | the declaration is partial and the journal row stays open for a member of this population | the generated usage line for each moved command, asserted in a `.ci` | unvalidated |
| A-5 | The ExaBGP INPUT vocabulary only grows: the translator learns forms it did not handle, and unlearns none | `ExabgpToZebgpCommand` translates the `neighbor` branch and passes the rest through; the bare forms currently reach no Ze command | a script's working line stops working, which is a compatibility break and goes to Thomas as a question | a fixture driving a bare ExaBGP line before and after; both bridge `.ci` hex expectations unchanged | unvalidated |
| A-6 | `help` is the only line that legitimately reaches Ze's dispatcher through the passthrough once the bare forms are translated | the bridge page's table puts `help` in the passthrough group with the six forms and nothing else; ExaBGP's other session-level words match no Ze command | the narrowing refuses a line that used to work | enumerate the lines a script can send, dispatch each through the bridge, record which resolve today | unvalidated |
| A-7 | `peer raw` is reachable from no ExaBGP form, so its respelling costs nothing outside Ze | the bridge page states it as a table row, "neither, because the translator never writes it"; grepping `raw` in `bridge_command.go` matches only the word `withdraw` | a script reaches `raw` some other way and breaks | the same grep, re-run over the changed translator, plus `./le ci-dispatch` | unvalidated |
| A-8 | 207 occurrences in 70 `.ci` files plus `cmd/ze/help_ai.go` are the whole programmatic sender population | measured: 70 files carry a Ze-side send string (40 in `test/encode`, 29 in `test/plugin`, 1 in `test/editor/commands`); `help_ai.go` publishes the grammar line and two examples. `internal/analyze/inject.go` matched the grep on prose about MRT injection and is NOT a sender | a sender is missed and fails at runtime rather than at the gate | `./le ci-dispatch`, whose stated job is that every command string this repository sends to its own daemon still routes | unvalidated |
| A-9 | `docs/guide/command-reference.md` is prose and the generated command catalog is written by `./le wiki-catalog update` | the page says "run `ze help command`" for the live list and names `./le wiki-catalog update file <destination>` as the writer of the Markdown catalog | a hand edit to a generated page is reverted at the next regeneration | `./le wiki-catalog check` before and after the doc phase | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The bridge flush is silently skipped after the move, and the forward pool stops draining | none by construction: no error, no log, no red. That is the risk | the extractor stops being able to answer an empty string, and the flush is proven from the bridge's input side rather than over a helper's return value |
| R-2 | Narrowing the passthrough refuses a line some script relies on | a bridge run reports a refusal naming a line that used to be forwarded | A-6 enumerates the lines first. The refusal names the line and says the bridge refused it, which is strictly more than today's unknown-command death |
| R-3 | A programmatic sender still spells an old path and fails at runtime | `./le ci-dispatch` reports an unroutable command string | the sender migration is its own phase, run before the old paths are deleted, and `ci-dispatch` gates it |
| R-4 | Making the selector mandatory changes the meaning of a command an operator relies on: `announce unicast <prefix>` reached every peer and is now refused | a `.ci` that announced with no peer now fails | intended, and stated as an acceptance criterion. Every such call site becomes an explicit `*`, which is the point of the change |
| R-5 | The moved commands enter the grammar gate for the first time and fail a rule nobody has checked them against | `./le cli-grammar` reports findings on the send subtree | A-2 validates this before the exemption is removed. A finding is fixed in the move, never exempted around |
| R-6 | `send` inherits web permissions from a container it did not have before | the permission a moved command answers under differs before and after | assert the `ze:ui-permissions` and `ze:ui-resource` each moved command answers under, before and after. `container request/peer` carries `bgp-peer/index.html` and `network` today; the `send` container's are declared deliberately, not inherited by accident |
| R-7 | The two-path collapse loses a form: a bare `withdraw all` reached every announcement, and the peer-scoped one compared the recorded selector as TEXT | a withdraw that used to remove an announcement no longer does | `handleWithdrawID` compares the peer selector to the one the announcement was recorded with, as text. `send bgp * withdraw id <id>` must reproduce the bare path exactly, and a `.ci` asserts both the wildcard and the named-peer case |
| R-8 | The package is larger than one implementation session | the sender migration alone is 207 occurrences across 70 files | priced and accepted. Thomas read the whole population on 2026-09-05 and answered "seems about right", so the spec runs as one package and the two-package cut is withdrawn. An implementer trims no acceptance criterion, and a re-cut is the owner's |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator cannot inject a message, an ExaBGP script's routes stop reaching the wire, or a route command's flush is skipped so the forward pool does not drain. The last one is silent |
| How is it reverted? | A single commit revert of the YANG, the handler registrations and the bridge restores the old paths; the sender migration and the doc edits revert with it. Nothing persists on disk and no config migration is involved |
| Who else touches this path? | `plan/spec-fixit-selector-narrows-through-a-pipe.md` (its display half stands, and `ze-peer-cmd.yang` is the shared file), and a concurrent session editing `docs/architecture/cli/command-verbs.md`, which this spec edits for the root-verb decision alone |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| operator types `send bgp 192.0.2.1 raw hex FFFF...` | → | `anchoredDef` binds the selector to the leaf `bgp` declares, `handleRaw`, `ResolveSinglePeer`, `SendRawMessage` | `TestSendRawReachesOnePeerAtItsNewPath` |
| operator types `send bgp * raw hex FFFF...` | → | `ResolveSinglePeer` refuses the wildcard | `TestSendRawRefusesAWildcardSelector` |
| operator types `send bgp * announce unicast 10.0.0.0/24 next-hop 10.0.0.1` | → | `handleAnnounceUnicastCmd`, `announceAndTrack`, the reactor | `test/ui/send-announce-reaches-the-wire.ci` |
| operator types `send bgp announce unicast 10.0.0.0/24`, with no selector | → | `Dispatch` refuses on `RequiresSelector` | `TestSendRefusesAMissingSelector` |
| operator types an old path, `peer raw 192.0.2.1 hex FF` | → | no node matches; the dispatcher refuses and names the command | `test/ui/send-old-paths-are-refused.ci` |
| an ExaBGP process writes `neighbor 127.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1` | → | `ExabgpToZebgpCommand`, `DispatchCommand`, the reactor, the peer's wire | `test/plugin/exabgp-bridge-internal.ci` (existing, hex unchanged) |
| an ExaBGP process writes the BARE form `announce route 1.1.0.0/24 next-hop 101.1.101.1` | → | the translator's new bare branch, `send bgp * ...`, the reactor, the peer's wire | `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci` |
| an ExaBGP process writes a line the translator does not recognize | → | the narrowed passthrough refuses by name | `TestBridgeRefusesAnUnrecognizedLineByName` |
| the same ExaBGP input, followed by the drain | → | `IsRouteCommand`, the selector the translator returned, the flush | `test/plugin/exabgp-bridge-flush-still-reaches-the-peer.ci` |
| the grammar gate walks the merged tree | → | `ExemptCategory`, `CheckName`, `CheckNode` over the send subtree | `TestSendSubtreeIsCheckedNotExempt` |
| a read-only operator types `send bgp * announce unicast 10.0.0.0/24` | → | `IsReadOnlyPath` answers false, `Dispatch` calls `isAuthorized`, `Profile.Authorize` reads the `Edit` section, `builtinReadOnlyProfile` denies by default | `TestSendIsDeniedToAReadOnlyProfile` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The merged YANG tree, enumerated | No slot in the send family accepts both an operator-supplied value and a fixed keyword. `send` is a root verb declared in `command.Verbs`; every child of `send` is a protocol keyword from a set the registry derives; and the one free-form slot sits AFTER a protocol keyword that says how to read it. A peer named `bgp`, `cache`, `raw` or `system` is reachable as a selector and confusable with no subsystem and no form word |
| AC-2 | `send bgp * raw hex 00`, `send bgp !edge1 raw ...`, and a selector matching two peers | Each is refused BY NAME, naming the selector and saying a single peer is required. An address matching no configured peer reaches the reactor and answers `ErrPeerNotFound` |
| AC-3 | Any send command typed with no selector | Refused by the dispatcher. All nine wire methods register `RequiresSelector: true` and their protocol container declares a mandatory selector leaf. No send reaches a handler on a defaulted `*` |
| AC-4 | `ze help` and tab completion at each moved path | Every argument each handler reads is offered and typed: the raw message type, its encoding enumeration and its data; the update encoding enumeration; the cache id. A value outside a declared enumeration is refused by type before the handler runs |
| AC-5 | `send bgp * announce unicast <prefix>` and `send bgp edge1 withdraw all` | Each behaves exactly as the bare and peer-scoped paths behaved before: the same handler, the same wire method, the same bytes. The groupings are instantiated once, and `*` is the only way to reach every peer |
| AC-6 | Every old path: `peer raw ...`, `peer update ...`, `announce unicast ...`, `peer <sel> withdraw all`, `request cache forward <id> <sel>` | Matches nothing. The refusal names the command that was typed. No alias and no fallback spelling |
| AC-7 | The command inventory before and after | Nine send wire methods before, nine after. This spec respells commands and adds none, so no new handler, no new wire method and no new RPC registration appears in the diff |
| AC-8 | An unmodified ExaBGP script writing `neighbor`-prefixed lines | Unaffected. The resulting BGP UPDATE on the peer's wire is byte-identical, the two bridge `.ci` hex expectations are unedited, and the bridge diff removes no form the translator accepts today |
| AC-9 | An ExaBGP script writing the BARE forms, `announce route ...` and `withdraw route ...` | Translated to `send bgp * ...` and reaching the peer's wire. This is behavior the bridge does not have today: the same input currently passes through untranslated and matches no Ze command |
| AC-10 | A line the translator does not recognize | Refused BY NAME, naming the line the script wrote and saying the bridge refused it. It is not forwarded to Ze's dispatcher. `help` is the one line that still passes through |
| AC-11 | A route command through the bridge, driven from its ExaBGP input side | The per-peer flush still reaches the peer after the dispatch. The test asserts the flush ARRIVES, not that a helper returned the right string |
| AC-12 | Any bridge helper that answers a selector | It cannot answer an empty string into a caller that reads empty as absence. Either it reports a failure the caller handles, or it does not exist |
| AC-13 | `./le cli-grammar` over the merged tree | The eight BGP methods are gone from `bridgeSurface`, are CHECKED rather than counted as exempt, and produce no finding. E1 reports exactly one member, `ze-bgp:help`, with its reason restated as the passthrough |
| AC-14 | `docs/architecture/exabgp-bridge.md` | States the translation direction, names `ExabgpToZebgpCommand` as the translator, keeps the three-row table current, and records what the narrowed passthrough now accepts |
| AC-15 | `./le ci-dispatch` | Green. Every command string this repository sends to its own daemon still routes, over all 70 migrated `.ci` files |
| AC-16 | A user holding only the built-in `read-only` profile types any `send` command | Denied, and denied by two defaults rather than by an entry. `IsReadOnlyPath` does not name `send`, so it answers false and the command evaluates in the profile's `Edit` section, whose default is `Deny`. The test drives the whole path from the dispatcher, and it also asserts that no entry naming `send` was added to the profile |
| AC-17 | `command.Verbs` and the grammar gate | Thirteen canonical verbs, `send` among them with the role `VerbAction`. The gate accepts `send` as a first token because it derives its verb set from that map, and no second verb list is added anywhere |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | injects a crafted BGP message into one session for a conformance test | CLI → `send bgp <sel> raw` → `ResolveSinglePeer` → `SendRawMessage` → wire | `test/ui/send-raw-reaches-one-peer.ci` |
| 2 | originates a prefix to every peer | CLI → `send bgp * announce unicast` → `announceAndTrack` → reactor → wire | `test/ui/send-announce-reaches-the-wire.ci` |
| 3 | withdraws a tagged announcement from one peer | CLI → `send bgp edge1 withdraw tag <k> <v>` → the registry → wire | `test/ui/send-withdraw-by-tag.ci` |
| 4 | replays a cached UPDATE to the peers a selector names | CLI → `send bgp <sel> cached <id>` → `ForwardUpdate` → wire | `test/plugin/api-send-cached.ci` |
| 5 | runs an unmodified ExaBGP script using `neighbor` lines | ExaBGP text → `ExabgpToZebgpCommand` → dispatch → reactor → wire, then the flush | `test/plugin/exabgp-bridge-internal.ci`, `test/plugin/exabgp-bridge-flush-still-reaches-the-peer.ci` |
| 6 | runs an ExaBGP script using the bare forms | ExaBGP text → the translator's bare branch → `send bgp * ...` → wire | `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci` |
| 7 | discovers the grammar by typing `send ` and pressing tab | completion over the merged tree: the protocols, then the selector, then the forms | `test/ui/cli-completion-send-forms.ci` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendRawReachesOnePeerAtItsNewPath` | `internal/component/bgp/plugins/cmd/raw/raw_test.go` | the selector binds from the slot after `bgp`, and `handleRaw` receives it | PASS |
| `TestSendRawRefusesAWildcardSelector` | `internal/component/bgp/plugins/cmd/raw/raw_test.go` | `*`, an exclusion and a two-peer selector are each refused by name | |
| `TestSendRefusesAMissingSelector` | `internal/component/plugin/server/send_test.go` | every send registration carries `RequiresSelector`, and a send with nothing bound is refused rather than defaulted to `*` | PASS. It sits in the `server_test` package, not `command_test.go`: the real command tree needs `internal/component/plugin/all`, which the in-package tests cannot import |
| `TestSendArgumentsAreDeclared` | `internal/component/bgp/plugins/cmd/raw/raw_test.go` | the generated usage line for each moved command names every argument its handler reads | |
| `TestOldSendPathsMatchNothing` | `internal/component/plugin/server/command_test.go` | each of the fifteen old paths resolves to no node | |
| `TestSendAddsNoWireMethod` | `internal/component/plugin/server/command_test.go` | AC-7: the send wire-method set is identical before and after, so the change is a respelling | |
| `TestSendSubtreeIsCheckedNotExempt` | `internal/component/command/grammar/checker_test.go` | `ExemptCategory` answers false for the eight moved methods and true for `ze-bgp:help`, and E1 has one member | |
| `TestVerbRegistryCanonical` (existing) | `internal/component/command/verbs_test.go` | AC-17: the golden `want` map gains `send` with the role `VerbAction`, and the length assertion moves from twelve to thirteen | PASS. `TestSendIsAVerbTheGrammarGateAccepts` (`send_test.go`) adds the gate half: `grammar.CheckName("send bgp raw")` is empty |
| `TestSendIsDeniedToAReadOnlyProfile` | `internal/component/plugin/server/send_test.go` | AC-16: a read-only profile is refused a `send` command through the dispatcher, and the refusal comes from the `Edit` section default rather than from an entry naming the verb | PASS. Its second half, that no entry names `send`, is `TestBuiltinReadOnlyProfileDeniesEverySendByDefault` in `internal/component/authz/authz_test.go`: `builtinReadOnlyProfile` is unexported, so no other package can read the profile it builds |
| `TestAnnounceFormsAreInstantiatedOnce` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | each form is reachable at exactly one path, and `*` reproduces the old bare behavior | |
| `TestWithdrawIDMatchesTheRecordedSelector` | `internal/component/bgp/plugins/cmd/announce/withdraw_forms_test.go` | R-7: the wildcard and a named selector each behave as the two old paths did | |
| `TestBridgeTranslatesTheNeighborForms` | `internal/exabgp/bridge/bridge_test.go` | all fourteen construction sites emit `send bgp <ip> ...` from `neighbor`-prefixed input, and no accepted form is lost | |
| `TestBridgeTranslatesTheBareForms` | `internal/exabgp/bridge/bridge_test.go` | AC-9: `announce route ...` and `withdraw route ...` with no prefix become `send bgp * ...` | |
| `TestBridgeRefusesAnUnrecognizedLineByName` | `internal/exabgp/bridge/bridge_test.go` | AC-10: an unrecognized line is refused and named; `help` still passes through | |
| `TestBridgeSelectorCannotBeSilentlyEmpty` | `internal/exabgp/bridge/bridge_muxconn_test.go` | AC-12: the helper reports a failure the caller must handle rather than answering an empty string | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| raw message type keyword | the five names `open`, `update`, `notification`, `keepalive`, `route-refresh` | `route-refresh` | N/A (closed enumeration) | any sixth word, refused by type rather than read as an encoding |
| cache id | `1` to the largest id the store holds, as a string identifier | that id | `0`, and any non-numeric text | an id no cache entry carries |
| raw data length | 0 to the peer's negotiated maximum message size | the negotiated maximum | N/A (empty is valid, for a keepalive) | one byte past the maximum, refused by the reactor |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `send-raw-reaches-one-peer` | `test/ui/send-raw-reaches-one-peer.ci` | the operator injects raw bytes at the new path and a scripted peer reads them; a wildcard selector is refused | |
| `send-announce-reaches-the-wire` | `test/ui/send-announce-reaches-the-wire.ci` | `send bgp * announce unicast` puts the prefix on a peer's wire, with the hex the current announce fixture asserts | |
| `send-withdraw-by-tag` | `test/ui/send-withdraw-by-tag.ci` | a tagged announcement is withdrawn from one selector and stays for another | |
| `send-old-paths-are-refused` | `test/ui/send-old-paths-are-refused.ci` | all fifteen old paths match nothing and the refusal names the command | |
| `cli-completion-send-forms` | `test/ui/cli-completion-send-forms.ci` | completion after `send ` offers the protocols, then the selector, then the forms, then each form's declared arguments | |
| `api-send-cached` | `test/plugin/api-send-cached.ci` | a cached UPDATE is replayed to the peers a selector names | |
| `exabgp-bridge-internal` | `test/plugin/exabgp-bridge-internal.ci` (existing) | an ExaBGP-format helper's route reaches the wire; the hex expectation is UNCHANGED and the Ze-side expectation line moves | |
| `exabgp-bridge-sdk` | `test/plugin/exabgp-bridge-sdk.ci` (existing) | the same, in SDK mode | |
| `exabgp-bridge-bare-form-reaches-the-wire` | `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci` | AC-9: a helper writing the bare ExaBGP form puts the prefix on the wire. It is a new fixture because no shipped one drives the passthrough half | |
| `exabgp-bridge-flush-still-reaches-the-peer` | `test/plugin/exabgp-bridge-flush-still-reaches-the-peer.ci` | AC-11: after a route command from the ExaBGP side, the flush arrives at the peer | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No wire-visible behavior changes. This spec respells CLI commands; every byte Ze puts on the wire is produced by the same encoder from the same fields, and the bridge fixtures assert that in hex. ExaBGP compatibility is covered by `./le functional exabgp-test` and by the four bridge `.ci` fixtures, which are the goal validation for AC-8, AC-9 and AC-11 | |

## Files to Modify

- `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang` - declares `container send / container bgp`, with the mandatory selector leaf on `bgp` and the description the sibling modules share, then `container raw` with its type, encoding and data declared. The top-level `container peer` and its shared description go with the move
- `internal/component/bgp/plugins/cmd/raw/raw.go` - reads its arguments from the declared leaves rather than positionally; keeps `ResolveSinglePeer`
- `internal/component/bgp/plugins/cmd/update/yang/ze-update-cmd.yang` - re-declares `send/bgp` with no description, so the merge is silent, and moves `container update` under it with a typed encoding enumeration
- `internal/component/bgp/plugins/cmd/update/update_text.go` - reads the encoding from its declared leaf; registers `RequiresSelector: true`
- `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - the two grouping instantiations collapse to one, under `send/bgp`
- `internal/component/bgp/plugins/cmd/announce/announce.go` - the six forms register `RequiresSelector: true`
- `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` - `forward` leaves `request cache` and becomes `send bgp <sel> cached <id>` with the id declared
- `internal/component/bgp/plugins/cmd/cache/cache.go` - `handleCacheForwardRPC` reads the declared id and the bound selector
- `internal/component/command/verbs.go` - `Verbs` gains `send` with the role `VerbAction`, the thirteenth entry. It is the single canonical verb map, and both the grammar gate and `validateCommandName` derive from it, so no second verb list exists to edit
- `internal/component/command/verbs_test.go` - `TestVerbRegistryCanonical` pins that map as a golden `want` value and asserts its length, so it is the second and last verb-list edit site
- `internal/component/command/grammar/checker.go` - `bridgeSurface` drops eight entries; the comment states what the one remaining entry covers and why
- `internal/exabgp/bridge/bridge_command.go` - the ten translating functions emit the new Ze-side grammar and return the selector they used; the translator gains the bare ExaBGP forms; the passthrough narrows to a stated set and refuses the rest by name
- `internal/exabgp/bridge/bridge_muxconn.go` - the selector helper stops answering an empty string into a caller that reads it as absence
- `internal/exabgp/bridge/bridge.go` - the flush uses the selector the translator returned
- `internal/plugins/exabgp/main_sdk.go` - the same, and its two grammar generations become one
- `internal/plugins/exabgp/bridgeplugin/internal.go` - the same
- `internal/test/fixture/plugin_fixture_06_exec.go` - a second helper writing the bare ExaBGP form, for the new bridge fixture
- `cmd/ze/help_ai.go` - the grammar line and the two worked examples that publish the old spelling
- 70 `.ci` files under `test/encode`, `test/plugin` and `test/editor/commands` - 207 occurrences of a moved path
- `docs/architecture/exabgp-bridge.md` - AC-14: the translation direction, the translator, the three-row table kept current, and what the narrowed passthrough accepts
- `docs/guide/mcp/overview.md` - the page `help_ai.go` declares, which publishes the same grammar to an agent
- `docs/architecture/api/commands.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/bgp/on-demand-origination.md`, `docs/architecture/api/ipc_protocol.md`, `docs/guide/cli.md`, `docs/architecture/config/syntax.md`, `docs/architecture/hub-api-commands.md`, `docs/features/api-commands.md`, `docs/config-migration.md`, `docs/architecture/api/architecture.md`, `docs/guide/plugins.md`, `docs/architecture/api/text-format.md`, `docs/guide/production-diagnostics.md` - the pages carrying the literal `peer <...>` shape. 133 occurrences over 15 pages, measured. `docs/features/ai-first.md`, `docs/features/introspection.md` and `docs/architecture/bgp/as112-coordination.md` join them from the citation note in checklist row 16: each mentions a file this spec changes, and the first two are printed from `cmd/ze/help_ai.go`. `docs/guide/command-catalogue.md` and `docs/guide/command-reference.md` are the generated surface and are REGENERATED rather than edited (A-9), and `docs/architecture/cli/command-verbs.md` is the fifteenth. Its root-verb content is edited by this spec, listed above; its T-4 and T-5 repoint is a concurrent session's
- `ai/patterns/cli-command.md` - the Peer Selector Mechanism section and the Full Command Inventory, for the send family alone
- `ai/rules/cli.md` - GENERATED from `ai/rules/points/cli/`, so the edits land in the point files and the rule is regenerated. `cli-grammar-keywords-before-values/the-verb-is-chosen-by-the-command-s-effect-on-live-state.md` enumerates the existing action verbs and gains `send` once the verb is declared; `the-first-token-after-the-noun-must-be-a-keyword.md` restates the peer-selector exception for the send family, typed by the protocol keyword
- `docs/architecture/cli/command-verbs.md` - the per-verb table's `send` row states the promise and the count; the verb-count sentence moves from twelve to thirteen once `Verbs` carries it
- `docs/architecture/cli/command-namespacing.md` - the exemption count and what E1 now covers
- `docs/architecture/api/process-protocol.md` - which commands carry `RequiresSelector`, and why the selector is mandatory
- `docs/architecture/core-design.md` - named as unaffected beyond a pointer to `exabgp-bridge.md`, which is where the translation is documented

## Files to Create

- `test/ui/send-raw-reaches-one-peer.ci` plus its fixture in `internal/test/fixture/`
- `test/ui/send-announce-reaches-the-wire.ci` plus its fixture
- `test/ui/send-withdraw-by-tag.ci` plus its fixture
- `test/ui/send-old-paths-are-refused.ci` plus its fixture
- `test/ui/cli-completion-send-forms.ci` plus its fixture
- `test/plugin/api-send-cached.ci`
- `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci`
- `test/plugin/exabgp-bridge-flush-still-reaches-the-peer.ci`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-raw-cmd.yang` declares `send/bgp` and the selector leaf on `bgp`; `ze-update-cmd.yang`, `ze-cli-announce-cmd.yang` and `ze-cli-cache-cmd.yang` merge their forms onto it. No new RPC: nine existing wire methods are re-pathed |
| YANG validation constraints | Yes | the raw encoding and the update encoding become enumerations; the raw message type is an enumeration of the five names; the cache id is a string identifier with a length |
| YANG custom validators | N-A | every value is expressible with a native `enumeration`, `length` or `pattern`. The selector stays a string because `selector.ParseDefault` owns its vocabulary and already reports on it |
| CLI commands/flags | Yes | no offline command changes; the whole change is in the daemon command tree |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` and `internal/component/command/grammar/checker.go`. `send` joins `command.Verbs` as the thirteenth canonical verb, so the gate accepts it as a first token with no gate edit. The peer exception is restated for the send family, and eight methods enter the checked population |
| Editor autocomplete | Yes | automatic for the new enumerations, and the protocol set is a closed child list; `test/ui/cli-completion-send-forms.ci` asserts the whole chain |
| Functional test for new RPC/API | Yes | eight new `.ci` files listed above, plus the two existing bridge fixtures |
| Pipe completeness | N-A | every send answers a small status object through the existing response path; this spec changes no answer shape |
| Env var registration | N-A | no YANG leaf under `environment/` is added |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module, binary or certificate. The change is a command-tree respelling plus a translator addition |
| Prometheus counters/metrics | N-A | no new observable state. The sends already count through the reactor's existing metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute is added or changed |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` and `docs/features/api-commands.md` carry the send commands under their old spellings |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` carries 9 occurrences of the old shape |
| 3 | CLI command added/changed? | Yes | `docs/guide/cli.md` (12 occurrences). `docs/guide/command-reference.md` and `docs/guide/command-catalogue.md` are REGENERATED by `./le wiki-catalog update`, not edited (A-9) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (28), `docs/architecture/api/ipc_protocol.md` (15), `docs/architecture/api/update-syntax.md` (3), `docs/architecture/api/text-format.md` (3), `docs/architecture/api/architecture.md` (4), `docs/architecture/hub-api-commands.md` (9) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (3) and `docs/architecture/exabgp-bridge.md` (AC-14) |
| 6 | Has a user guide page? | Yes | `docs/guide/route-injection.md` and `docs/guide/production-diagnostics.md` |
| 7 | Wire format changed? | N-A | no byte on any wire changes; the bridge fixtures assert this in hex |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`: the selector becomes mandatory, so a plugin dispatching a send string must carry one |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC obligation changes. The messages, their encoding and their conditions are untouched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: eight new fixtures under the existing `test/ui` and `test/plugin` shapes |
| 11 | Affects daemon comparison? | Yes | `docs/exabgp/exabgp-differences.md`: the bare ExaBGP forms start working, which is a difference that closes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-namespacing.md` (the exemption), and `docs/architecture/core-design.md` named as unaffected beyond a pointer |
| 13 | Route metadata keys added/changed? | N-A | no metadata key is added or renamed |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/cli-commands.md` and `docs/plugin-overview.md`: fifteen command paths change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-fixit-send-names-its-destination.md`. The Design owners this spec's files declare are `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/bgp/on-demand-origination.md`, `docs/architecture/cli/command-namespacing.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md` and `docs/guide/mcp/overview.md`. All eight are named above. The same command also reports three pages that MENTION this spec's code without owning a Design anchor on it, and each is now named in Files to Modify: `docs/features/ai-first.md` and `docs/features/introspection.md` are mentioned by `cmd/ze/help_ai.go`, which prints the peer selector help this spec respells, and `docs/architecture/bgp/as112-coordination.md` is mentioned by `update_text.go`. Read each before you edit it, because a mention is not yet a claim that the page states the grammar |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | 133 occurrences of the literal `peer <...>` shape over 15 pages, measured. The rule that decides each edit: an occurrence that is a Ze CLI command gains the new spelling; an occurrence inside an ExaBGP line-protocol sample does NOT change; an occurrence on a generated page is regenerated |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- declare `send bgp <selector>` and prove the selector binds
   - Tests: `TestSendRawReachesOnePeerAtItsNewPath`, `TestSendRefusesAMissingSelector`
   - Tests, also: `TestVerbRegistryCanonical`, `TestSendIsDeniedToAReadOnlyProfile`
   - Files: `verbs.go` and `verbs_test.go` (the thirteenth verb, role `VerbAction`), `ze-raw-cmd.yang` (the `send/bgp` skeleton and the selector leaf), the `raw.go` registration
   - Verify: the new path dispatches and the selector reaches the context's peer field, with the handler still a stub. The grammar gate accepts `send` as a first token, and the read-only profile refuses the command. A-1 and AC-16 are answered here, before anything moves
2. **Phase: declare the arguments** -- raw, update and cache forward state what their handlers read
   - Tests: `TestSendArgumentsAreDeclared`, `test/ui/cli-completion-send-forms.ci`
   - Files: the three YANG modules and their three handlers
   - Verify: the generated usage line names every argument; a value outside an enumeration is refused before the handler runs; the journal row's two in-population members are answered
3. **Phase: the free moves** -- `peer raw` and `peer update`, which no passthrough script reaches
   - Tests: `TestOldSendPathsMatchNothing` for those two paths, `test/ui/send-raw-reaches-one-peer.ci`
   - Files: `ze-raw-cmd.yang`, `ze-update-cmd.yang`, their handlers, and the translator's `peer update` output
   - Verify: A-7 holds (no ExaBGP form reaches `raw`), and the `peer update` translator edit lands with the move so no bridge run is red between phases
4. **Phase: the translator learns the bare forms** -- before the six passthrough spellings move
   - Tests: `TestBridgeTranslatesTheBareForms`, `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci`
   - Files: `bridge_command.go`
   - Verify: a bare ExaBGP line reaches the wire, which it does not today. This phase runs BEFORE phase 5 so the passthrough spellings are already covered when they move
5. **Phase: the passthrough moves** -- announce, withdraw and `cache forward`, plus the mandatory selector
   - Tests: `TestAnnounceFormsAreInstantiatedOnce`, `TestWithdrawIDMatchesTheRecordedSelector`, `TestSendAddsNoWireMethod`, the `test/ui/send-*.ci` set
   - Files: `ze-cli-announce-cmd.yang`, `ze-cli-cache-cmd.yang`, their handlers; `RequiresSelector: true` on all nine
   - Verify: each command answers at its new path; the groupings are instantiated once; `*` reproduces the old bare behavior; R-6's permission assertion passes before and after
6. **Phase: the senders** -- every programmatic sender of an old path, found FIRST
   - Tests: `./le ci-dispatch`
   - Files: 70 `.ci` files (207 occurrences), `cmd/ze/help_ai.go`
   - Verify: `./le ci-dispatch` green
7. **Phase: the bridge's remaining work** -- the passthrough narrows and the flush stops re-parsing
   - Tests: `TestBridgeTranslatesTheNeighborForms`, `TestBridgeRefusesAnUnrecognizedLineByName`, `TestBridgeSelectorCannotBeSilentlyEmpty`, `test/plugin/exabgp-bridge-flush-still-reaches-the-peer.ci`, and the two existing bridge fixtures with unchanged hex
   - Files: `bridge_command.go`, `bridge_muxconn.go`, `bridge.go`, `main_sdk.go`, `bridgeplugin/internal.go`, the new helper in `internal/test/fixture/`
   - Verify: AC-8 through AC-12. A-6 is answered by enumeration BEFORE the passthrough is narrowed, and A-5 by the diff removing no accepted form
8. **Phase: the gate** -- the exemption shrinks and the moved commands are checked
   - Tests: `TestSendSubtreeIsCheckedNotExempt`
   - Files: `checker.go`, and `docs/architecture/cli/command-namespacing.md`
   - Verify: `./le cli-grammar` green with E1 reporting one member; a finding on a moved command is FIXED, never exempted around (R-5)
9. **Phase: the documentation** -- 133 occurrences over 15 pages, by the rule in row 17
   - Tests: `./le doc check verify`, `./le docvalid`, `./le wiki-catalog check`
   - Files: the pages listed in Files to Modify; `docs/architecture/exabgp-bridge.md` carries AC-14
   - Verify: no hand edit lands on a generated page (A-9); every ExaBGP line-protocol sample is unchanged except where the bare forms are newly documented as working

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-1 is checked over the merged tree rather than over the files that were edited |
| Feature completeness | All seven user stories reach the wire, including both bridge forms |
| Correctness | `raw` still refuses every selector that is not exactly one peer, reached through the new path rather than only through the handler |
| Respelling, not addition | AC-7: the wire-method set is identical before and after. A new handler or a new registration in the diff means the move became a feature |
| Naming | The selector is one concept with one name in the model. No second spelling enters `applyExtractedSelectors`, `RequiresSelector` or `ResolveSinglePeer` |
| Data flow | The translator's INPUT vocabulary only grows. Verified by reading the diff for a removed accepted form, not by asserting it |
| Guard: fail closed | `RequiresSelector` is set on all nine; no send reaches a handler on a defaulted `*`; no bridge helper answers an empty selector a caller reads as absence; the narrowed passthrough refuses rather than forwards |
| Rule: `ai/rules/cli.md` | The peer exception is restated for the send family in the rule file itself, not only in the spec, and it says the protocol keyword is what types the value |
| Rule: `ai/rules/no-layering.md` | No old path survives as an alias, and the passthrough is narrowed rather than kept beside the new translation |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Fifteen old paths match nothing | `test/ui/send-old-paths-are-refused.ci` |
| Nine wire methods answer at their new paths, and nine is still the count | `./le command list` shows each under `send bgp`, and `TestSendAddsNoWireMethod` passes |
| Every send carries the guard | `grep -rn "RequiresSelector" internal/component/bgp/plugins/cmd` names all nine |
| E1 has one member | `grep -n "bridgeSurface" -A 6 internal/component/command/grammar/checker.go` |
| No sender left behind | `./le ci-dispatch` |
| The ExaBGP input vocabulary only grew | the bridge diff removes no accepted form, and both existing bridge `.ci` hex lines are unchanged in the diff |
| The bare form works | `test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci` |
| The bridge page states the translation, the three groups and the passthrough | `docs/architecture/exabgp-bridge.md` |
| The grammar gate is green over the newly checked population | `./le cli-grammar` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The selector is operator text reaching `selector.ParseDefault`. Its vocabulary is unchanged, and the exclusion refusal in `ResolveSinglePeer` must still fire through the new path |
| Authorization | `IsReadOnlyPath` is an allowlist of the read verbs and does not name `send`, so the whole send family evaluates in the edit section and the built-in read-only profile denies it by default (AC-16). A verb the allowlist has never heard of is a write, which is the property that makes a new root verb safe to add. Assert the permission each moved command answers under before and after the move (R-6), including any `ze:ui-permissions` a moved node inherits from its new parent |
| Fail-closed guard | `RequiresSelector` is the guard that stops a send with no selector. It is being ADDED to eight commands that lacked it, so each one's refusal is tested rather than assumed |
| Passthrough narrowing | The passthrough is an untyped path from a plugin's stdout to Ze's dispatcher. Narrowing it REDUCES what a script process can reach, so the review checks that the remaining accepted set is exactly the one A-6 enumerated |
| Blast radius of `raw` | `raw` bypasses validation by design. Its single-session property and its sender permission, which requires the process to be attached to that peer, must both still hold at the new path |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `./le cli-grammar` reports a finding on a moved command | Fix the command. An exemption for a newly moved node is refused (R-5) |
| `./le ci-dispatch` reports an unroutable string | A sender was missed. Find it and migrate it; do not add an alias |
| A line the translator accepts today would stop being accepted | STOP. A-5 is broken and it is a compatibility break. Take it to Thomas as a question |
| A new wire method appears in the diff | STOP. AC-7 is broken: the move became a feature, and the extra command needs its own decision |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The token order is not a preference, it is the same defect twice avoided. `show bgp peer list` is unreachable for a peer named `list` because a slot holds both a name and a keyword. Destination-first would have rebuilt that slot one level up. Protocol-first puts a closed set in front of the value, and the closed set is also what tells the parser how to read it.
- Because the protocol types the value, the selector's grammar can live in the protocol's own package. That is the registration pattern applied to a parser: no central switch, and nothing infers a kind from a value's shape.
- The bridge answers the compatibility question three different ways, and nothing wrote any of them down. Passthrough, translator output, and neither. Reading only the translator gives the reassuring answer for one third of the surface.
- The passthrough protects the wrong thing. It preserves Ze's own bare spellings for a script that typed them, while ExaBGP's real bare form, `announce route ...`, matches no Ze command and dies. A contract nobody wrote down had drifted from the contract it appeared to be.
- A routing decision keyed on a leading literal is what makes a grammar rename dangerous. The translator knows the selector at every construction site and then throws it away, so the caller re-parses a string it built four lines earlier. The re-parse is the defect; the prefix is only where it shows.
- `PeerSelector` answering `*` is the same shape one level up: an unbound selector and "every peer" are the same value, so the fan-out commands cannot tell them apart. A mandatory slot has no unbound case.
- An exemption is not a policy, it is a claim with a date. Judge `bridgeSurface` by the REASON each member carries and the list splits three ways; judge it by its name and it looks like one thing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `send` is an explicit discriminator | `request <destination> <protocol> ...`, with no discriminator | Every child of `request` is a subsystem keyword today. A free-form value in that slot is unparseable against `bgp`, `cache` and `system`, and a peer named `bgp` would be unreachable |
| `send` is a ROOT VERB, the thirteenth | `send` as a noun under `request` | Thomas ruled it on 2026-09-05, in two words: "send bgp". The axis on `docs/architecture/cli/command-verbs.md` gives the reason: `request` changes the system, and `send` puts a message outside it, so a send is not a kind of request. It costs one entry in `command.Verbs` and one row in that map's golden test, and nothing in `IsReadOnlyPath`, which allowlists the read verbs and treats every verb it does not name as writing |
| The protocol comes BEFORE the selector | the shape first proposed, which put the destination in front of the protocol | Owner decision, and the repository's own reason is stronger: a closed keyword in front of the value tells the parser how to read it, keeps a name and a keyword out of one slot, and lets each protocol's package own its selector grammar. Completion, the grammar checker and the authz profiles all key on closed sets |
| The selector leaf is declared on the protocol container, not on `send` | one selector leaf on `send`, shared by every protocol | A BGP peer selector, a resolver address and an interface name are three types with three validators and three completions. One shared leaf would be a central declaration every protocol has to fit (`ai/rules/principles.md`) |
| The selector leaf keeps the model's existing name, and "destination" is prose | name the leaf `destination`, and either teach `applyExtractedSelectors` a second spelling or change `ResolveSinglePeer`'s signature at six call sites | One concept, one name in the model (`ai/rules/writing.md`, habit 1). The leaf is positional and anchored, so the operator never types its name, and `ai/rules/cli.md`'s ban is on exposing it as a keyword |
| `request cache forward` joins the population as `send bgp <sel> cached <id>` | leave it under `request cache`, beside `retain`, `release` and `expire` | It puts a whole UPDATE on each matched peer's wire and names the peers, so both legs hold. Leaving it keeps a second place where a BGP send names its destination. Its three siblings act on the cache entry and send nothing, so the family is split by effect rather than by noun |
| The arguments are declared in this spec rather than in a follow-up | keep the journal row open, as the superseded spec did | The row's stated reason stops holding: this spec publishes a path no operator has typed, and shipping it with an undeclared tail states a knowingly incomplete grammar at a brand-new path |
| The translator learns the bare ExaBGP forms, freeing the six passthrough spellings | freeze the six spellings so a passthrough script keeps reaching them; or take the compatibility break to Thomas | Teaching the translator removes the coupling instead of preserving it, and it fixes an existing gap: ExaBGP's own bare `announce route ...` matches no Ze command today. Freezing would exempt six of fifteen paths from the point of the spec. Taking it to Thomas would be asking permission to do less when a design exists |
| The free moves go first, the passthrough moves go after the translator learns | move all nine together | The bridge page's table says `peer raw` costs nothing and `peer update` costs one translator edit, while the six passthrough forms need the translator taught first. Ordering the phases by that table means no phase leaves an ExaBGP script broken between commits |
| The passthrough narrows to `help` and refuses the rest by name | keep forwarding every unrecognized line | Forwarding sends an unrecognized script line to Ze's dispatcher, where it dies as an unknown command with no mention of the bridge. A named refusal is the same outcome with the cause attached, and the narrowing is what makes the six spellings safe to move |
| E1 survives with one member rather than being deleted | delete `bridgeSurface` entirely; or keep all nine | Eight members lose their reason: `peer-raw` never had the line protocol's reason, `peer-update` is translator output, and the six become translator output too. `ze-bgp:help` keeps the passthrough reason, so the map keeps exactly one entry |
| The bridge's selector is returned by the translator rather than re-parsed from its output | update `ExtractPeerAddress` to recognize the new prefix | Re-parsing a string you just built to recover a value you held is the defect the prefix coupling only exposes. Returning it removes the coupling, and it is what makes the fail-closed criterion reachable without inventing a second guard |
| The two announce and withdraw grouping instantiations collapse to one | keep both, with an optional selector on the bare one | A mandatory selector makes the bare path unreachable by construction, and `*` states "every peer" explicitly. One instantiation is one declaration of the grammar |

## Known Limitations

- The update route expression after the encoding word stays a tail `ParseUpdateText` owns. A full YANG declaration of a recursive attribute and NLRI grammar is not reachable here, and the encoding enumeration plus a `ze:help` naming `docs/architecture/api/update-syntax.md` is what this spec delivers.
- Narrowing the passthrough removes an undocumented escape hatch: today a plugin's stdout can carry ANY Ze command through the bridge, and after this spec it carries the ExaBGP vocabulary and `help`. Nothing in the repository uses that hatch, and A-6 enumerates before the narrowing lands, but it is a capability that goes.
- `request commit` keeps its row in `plan/journal/command-takes-an-untyped-positional-value.md`. It sends nothing and is outside this population.
- `docs/architecture/cli/command-verbs.md` carries the root-verb decision, and its rows T-4 and T-5 are answered by this spec while their Evidence columns still point at the superseded spec. That repoint is a concurrent session's.
- The `send` row on that page and the `send` line in `ai/rules/cli.md` describe a verb `command.Verbs` does not hold yet. The implementation phase declares the verb and makes the two agree, and the page states which side is ahead until then.
- E1 keeps one member. Whether `help` should reach Ze through a passthrough at all is the bridge's own question, not this spec's.
- `bgp` is the only protocol under `send` when this lands. The token's value is the parser property and the registration point, not a second protocol that exists today.
- **Size, and it is not cut.** Nine wire methods at fifteen paths, 207 sender occurrences across 70 `.ci` files, 133 documentation occurrences across 15 pages, plus the bridge and the gate. Thomas read that population on 2026-09-05 and answered "seems about right", so the spec runs as one package. The phase 6 to phase 7 cut this section carried is withdrawn. An implementer who finds the package large trims no acceptance criterion and cuts nothing: the size was priced and accepted, and a re-cut is the owner's (`ai/rules/planning.md`).

## RFC Documentation (Scope: protocol)

No RFC obligation changes. The messages this spec respells are encoded by the
same producers from the same fields, and their conformance comments stay where
they are. RFC 4271 Section 4.3 (UPDATE format), RFC 4724 Section 2 (End-of-RIB),
RFC 7999 Section 3.1 (BLACKHOLE agreement) and RFC 8955 Section 7 (FlowSpec
traffic action) are each enforced below the command layer and are untouched.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
