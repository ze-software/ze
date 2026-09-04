# Spec: selector narrows through a pipe

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas set the rule, verbatim:

> `show bgp | limit <name>` / `show bgp (all)` / `show bgp | summary` -- there
> should not be a `show bgp list` where `list` can be either a name or command.

and widened it, verbatim:

> review all the command with a selector or which add extra so we convert them
> to `| command`

The rule separates three roles that the command tree today folds into one token
sequence.

| Role | Who carries it today | Who carries it under the rule |
|------|----------------------|-------------------------------|
| Subject: what the command is about | the command path | the command path, unchanged |
| Narrowing: which members of the subject | a positional value in the path (`show bgp peer <sel> detail`) | a pipe operator that takes an argument (`\| limit <sel>`) |
| Shaping: how much of each member | a positional keyword in the path (`detail`, `list`) | a pipe operator (`\| summary`, `\| extensive`) |

Two consequences follow, and each is an acceptance criterion below. The bare
command answers for the whole subject, so `show bgp peer` with nothing after it
is every peer. And no slot in the path accepts both an operator-supplied name
and a fixed keyword, which is the ambiguity the rule names.

The symptom that opened this is `show bgp peer list`. `container peer` declares
`leaf selector { mandatory true; }`, so the token after `peer` is a peer name.
`list` is a container under `peer` carrying `ze:inherit "none"`, so the same
token position also accepts the fixed word `list`. The schema's own `ze:help`
calls it "the exception". `request interface migrate` has the identical shape
under `container interface`, and those two are the only `ze:inherit "none"`
nodes in the tree.

The goal is the rule applied to the whole merged command tree, not to those two
nodes. 89 command nodes take a mandatory value; the inventory below places every
one of them in a class and states what the rule does to that class.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - the grammar this rule replaces
  → Decision: the shape in force is `<verb> <noun> <selector-kind> <selector-value> <action> [<args>]`, and the rule deletes the middle two tokens from it. The page carries the shape as a fenced block and a six-row incorrect/correct table, so it is rewritten by this spec, not appended to.
  → Constraint: seven feeders enforce R1-R9 and `./le cli-grammar` runs them. The new rule is a feeder-level rule, so it lands in `internal/component/command/grammar` where R5 and R6 already live, and the static gate reads the whole YANG tree so the rule is enforced over every module at once.
- [ ] `docs/architecture/cli/command-namespacing.md` - the Design doc `grammar/checker.go` declares
  → Constraint: `checker.go` names this page in its `// Design:` header, so a rule added to the checker owes an edit here in the same work.
- [ ] `ai/rules/cli.md` - the CLI grammar directives
  → Constraint: "A command addressing one member of a set MUST type the selector with `name`, `id`, `index`, `address`, `type`, `key` ... Peer commands are the one exception: they address a peer positionally." The rule Thomas set replaces this sentence rather than adding to it, so the rule file is edited by this spec.
  → Constraint: "A command whose action enumerates the members of a set MUST spell that action `list`." Under the rule the bare command enumerates, so `list` as an action word is deleted rather than respelled, and this directive is rewritten too.
  → Decision: "Ze is unreleased, so an unreleased grammar MUST be replaced outright rather than deprecated." No alias, no fallback, no compatibility spelling for any converted command.
- [ ] `ai/patterns/cli-command.md` - the structural template
  → Constraint: the page carries "Peer commands are the explicit exception. Their public syntax keeps the peer selector immediately after `peer`", plus a worked `Peer Selector Mechanism` section and a Command Classes table naming a `Typed selector` and a `Peer-scoped` class. All four blocks state the grammar this spec removes.
- [ ] `ai/rules/simplicity.md` - the shape of the fix
  → Constraint: the fix cuts machinery and never correctness. `RequiresSelector` is a fail-closed guard on six action commands, so any conversion that drops it cuts correctness and is refused.
- [ ] `ai/rules/spec-no-code.md` - spec form
  → Constraint: no code snippet in any language; tables and prose only.

**Key insights:**
- The pipe layer already pushes an argument-taking operator down to the handler. `foldFilters` rewrites a command-owned `PipeFilter` into positional command arguments and appends them to the command string, so `show bgp rib | peer 10.0.0.1` reaches the RIB plugin as `show bgp rib peer 10.0.0.1`. The narrowing operator this rule needs is therefore a mechanism that exists and is in production, not new machinery.
- Chaining two operators already works. `parsePipeOps` splits on every `|`, `expandAliases` runs before `foldFilters`, so `| limit <sel> | summary` parses, folds the filter into the command and leaves the alias expansion in the local chain.
- `summary` already exists, as a pipe alias on `show bgp`. `limit` and `extensive` do not exist anywhere.
- `list` is a SHAPE, not a filter. `handleBgpPeerList` and `handleBgpPeerDetail` both call `filterPeersByArgs` over the same peer population; `list` builds a five-field row and `detail` the full record. Removing the word `list` therefore changes which fields the bare command answers with, and changes no filtering.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - declares `container peer` with `leaf selector { mandatory true; }` under `show bgp`, `update bgp` and `request`; declares `list` with `ze:inherit "none"`; declares `delete bgp peer` with its own `leaf selector`.
- [ ] `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang` - declares `container peer` then `container raw` carrying its own `leaf selector`, so the operator types `peer raw <selector>`.
- [ ] `internal/component/bgp/plugins/cmd/update/yang/ze-update-cmd.yang` - the same shape, `peer update <selector>`.
- [ ] `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - declares `announce-forms` and `withdraw-forms` as groupings instantiated at two paths: bare (`announce unicast ...`, reaching every peer) and under `peer` with the selector (`peer <sel> announce unicast ...`).
- [ ] `internal/component/iface/yang/ze-iface-cmd.yang` - declares `container interface` with `leaf name { mandatory true; }` under `request`, and `migrate` with `ze:inherit "none"`.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - `extension inherit` takes `none` or `ancestors`; `ancestors` is the default.
- [ ] `internal/component/command/pipe_catalog.go` - the 17 global operators and their contracts. `PipeArgKind` already has `ArgText`, `ArgCount`, `ArgFields` and `ArgPath`, so argument-taking operators are the norm rather than an exception. There is no `limit`, no `extensive`. `match` is `ClassData`, `ArgText`, over `rowShapes`.
- [ ] `internal/component/command/pipe.go` - `ParsePipe`, `parsePipeOps`, `expandAliases`, `foldFilters`, `ApplyPipes`. `foldFilters` turns an owned filter into command arguments and appends them to the command string; a `Leading` filter is appended first.
- [ ] `internal/component/command/pipe_filter.go` - `PipeFilter` carries `Name`, `Description`, `TakesArg` and `Leading`. `RegisterPipeFilters` refuses a name a pipe alias of an overlapping path carries, with a `panic("BUG:")`. Lookup is longest registered command path, and an empty registration blocks a shorter prefix from matching.
- [ ] `internal/component/command/alias.go` - an alias is a fixed expansion into operators, takes no argument, and names no other alias. `RegisterAliases` refuses a name a pipe operator or an overlapping path's filter already carries.
- [ ] `internal/component/command/completer.go` - `PipeOperators` is derived from the catalog; `pipeExtras` adds a command's aliases and owned filters; `pipeSubArgs` is a hand-written map holding sub-argument completions for `json` and `fill` alone.
- [ ] `internal/component/command/grammar/checker.go` - `CheckName` (R1, R2, R3, R7), `CheckNode` (R5 keyword-before-value, R6 value-before-keyword, R8 string identifiers), `CheckSiblings` (R9), `CheckRootNamespace`.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleBgpPeerList` and `handleBgpPeerDetail` both call `filterPeersByArgs`; `registerAliases` declares `summary` and `peers` on `show bgp` and an empty alias set on every child; six RPC registrations carry `RequiresSelector: true`.
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` declares `peer`, `family`, `prefix`, `path`, `community`, `match`, `count`, `first`, `last`, `histogram`, `graph` on `show bgp rib`, with `received` and `advertised` as `Leading`.
- [ ] `internal/component/config/retired.go` - `retiredKeywords` and `RetiredKeywordHint` are CONFIG parser machinery. The map keys are config field names and the hint is appended to an unknown-field parse error. Nothing in it reaches the command tree.
- [ ] `test/ui/alias-summary.ci` - the functional-test shape for a pipe alias: a `ze-test fixture ui/<name>` invocation asserting exit 0 and `OK` on stdout.

**Behavior to preserve:**
- The peer selector vocabulary: an address, a peer name, an AS pattern such as `as65001`, a glob, a comma-separated list, or `*`. `peersel.ParseDefault` reads it and this spec does not change it.
- The six commands registered `RequiresSelector: true` MUST stay unable to run without an explicit selector, whatever spelling carries it.
- `show bgp | summary` and `show bgp | peers` keep their current meaning and their two `.ci` tests.
- The field sets each handler answers with. This spec moves which words select a field set; it does not change what a field set holds.
- `show bgp rib`'s push-down filtering. Its filters reach the RIB plugin as command arguments and that route is unchanged.

**Behavior to change:**
- Every command node in classes S-inherited and S-own (32 nodes) loses its positional selector and gains `| limit <selector>`.
- `show bgp peer list` and `request interface migrate` lose the `ze:inherit "none"` shape; the extension itself loses its only two users.
- `show bgp rib`'s `| peer <sel>` filter is respelled `| limit <sel>`: it is the same concept under a second name, which habit 1 of `ai/rules/writing.md` bans.
- Class K and class A: pending Thomas's answer to Q3 below. The recommendation is that class A is untouched and class K is decided by Q3.

## Data Flow (MANDATORY)

### Entry Point
- The operator's line, arriving at `ze cli` interactively, at `ze <verb> ...` on a shell command line, or over SSH into the CLI. Format at entry: one string holding the command and, after the first `|`, the operator chain.

### Transformation Path
1. `ParsePipe` cuts the string at the first `|`; `parsePipeOps` splits the tail on every remaining `|` into one operator and its argument each.
2. `expandAliases` replaces each unknown-operator segment that names a registered alias with the operators the alias stands for. One pass, no nesting.
3. `bindAddressFields` fixes address-field declarations onto `resolve` and `origin`.
4. `foldFilters` looks the command path up in the pipe-filter registry by longest prefix. A segment naming an owned filter is REMOVED from the local chain and appended to the command string as `<filter-name> <value>`; every other segment stays local.
5. The rewritten command string is matched against the YANG command tree. The dispatcher extracts a `peer <value>` selector positionally, removes it from the token stream, and matches what remains.
6. The handler receives `ctx.Peer` (the extracted selector) and `args` (the trailing tokens, which now include any folded filter). `filterPeersByArgs` prefers `args[0]` over `ctx.PeerSelector()`.
7. Whatever the handler answers is rendered through `ApplyPipes` with the local chain.

Under this spec, step 5's positional extraction is deleted for classes S-inherited and S-own, and step 4 becomes the only route by which a selector reaches step 6.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ CLI parser | one text line; the operator chain is text until `parsePipeOps` | No |
| CLI ↔ daemon | the rewritten command string, after `foldFilters` | No |
| Daemon ↔ plugin | `pluginserver.CommandContext` carrying `ctx.Peer`, plus `args` | No |
| Command tree ↔ completion | `TreeCompleter` over the merged `Node` tree, plus `pipeExtras` for the operator names | No |

### Integration Points
- `command.RegisterPipeFilters` - the registration each converted command's owner package calls to declare `limit`.
- `command.RegisterAliases` - where `extensive` would live if Q2 answers that it is a fixed expansion rather than a distinct server call.
- `grammar.CheckNode` - where the new rule is enforced over the YANG tree.
- `pluginserver.RPCRegistration.RequiresSelector` - the guard that must keep holding after the selector moves into the pipe.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## The Inventory

Every command node in the merged YANG tree that takes a MANDATORY value, own or
inherited from an ancestor container. 400 nodes carry `ze:command`; 89 of them
take such a value. The classes below partition those 89.

The population is derived by walking every `*.yang` under `internal/` for
`ze:command` nodes and collecting the mandatory leaves on the node and on its
container ancestors. Implementation replaces this hand-listed table with the
same walk inside `grammar.CheckNode`, so the list cannot go stale
(`ai/rules/principles.md`, one declaration).

| Class | What the value is | Count | What the rule does |
|-------|-------------------|-------|--------------------|
| S-inherited | a selector declared on an ancestor container, inherited by each command under it | 26 | the ancestor's `leaf selector` / `leaf name` is deleted; each command gains `\| limit <value>` |
| S-own | a leaf named `selector` declared on the command node itself | 6 | the leaf is deleted; the command gains `\| limit <value>` |
| K | a leaf named `name`, `id`, `type` or `key` naming ONE member of a set | 31 | Q3 decides |
| A | a value that is the command's subject or an argument to it, not a narrowing of a set | 26 | unchanged; the rule does not reach it |

Four rows below (`id`, `tag`, `blackhole`, `unicast`) are grouping bodies in
`ze-cli-announce-cmd.yang`, instantiated at two paths each: bare, and under
`peer <selector>`. Their bare path is already the shape the rule asks for; the
`peer <selector>` path is a class S-inherited instance.

#### Class S-inherited (26 nodes)

| Command path | Value slot | Module |
|---|---|---|
| `create interface bridge name address` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface bridge name unit` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface dummy name address` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface dummy name unit` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `delete interface name address` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `delete interface name unit` | `name` on `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `pki certificate name bundle pem` | `name` on `name` | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` |
| `pki certificate name fingerprint` | `name` on `name` | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` |
| `pki certificate name pem` | `name` on `name` | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` |
| `request interface down` | `name` on `interface` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `request interface mac` | `name` on `interface` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `request interface migrate` | `name` on `interface`, declined by `ze:inherit "none"` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `request interface mtu` | `name` on `interface` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `request interface up` | `name` on `interface` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `request peer flush` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `request peer pause` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `request peer plugin session ready` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `request peer resume` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `request peer teardown` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer capabilities` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer detail` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer history` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer list` | `selector` on `peer`, declined by `ze:inherit "none"` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer rib` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `show bgp peer statistics` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `update bgp peer prefix` | `selector` on `peer` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |

#### Class S-own (6 nodes)

| Command path | Value slot | Module |
|---|---|---|
| `delete bgp peer` | `selector` | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `peer raw` | `selector` | `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang` |
| `peer update` | `selector` | `internal/component/bgp/plugins/cmd/update/yang/ze-update-cmd.yang` |
| `request cache forward` | `id`, `selector` | `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` |
| `show policy chain peer` | `selector` | `internal/component/bgp/plugins/cmd/policy/yang/ze-policy-cmd.yang` |
| `show policy test peer` | `selector`, `direction` | `internal/component/bgp/plugins/cmd/policy/yang/ze-policy-cmd.yang` |

#### Class K (31 nodes) -- Q3 decides

| Command path | Value slot | Module |
|---|---|---|
| `bfd profile name` | `name` | `internal/component/bfd/yang/ze-bfd-cmd.yang` |
| `clear dns cache record` | `name` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `clear interface name counters` | `name` | `internal/component/iface/yang/ze-iface-interface-cmd.yang` |
| `create interface address` | `name`, `prefix` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface bridge name` | `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface dummy name` | `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface unit` | `name`, `vid` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `create interface veth name` | `name`, `peer` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `delete interface name` | `name` | `internal/component/iface/yang/ze-iface-cmd.yang` |
| `id` (withdraw form, two paths) | `id` | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` |
| `l2tp session id` | `id` | `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` |
| `l2tp tunnel id` | `id` | `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` |
| `pki certificate name` | `name` | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` |
| `pppoe session id` | `id` | `internal/component/l2tp/pppoe/cmd/yang/ze-pppoe-cmd.yang` |
| `request cache expire` | `id` | `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` |
| `request cache release` | `id` | `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` |
| `request cache retain` | `id` | `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` |
| `show bgp reject-asn name` | `name` | `internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn-cmd.yang` |
| `show config cat` | `id` | `internal/plugins/config-cli/yang/ze-config-cli-cmd.yang` |
| `show data cat` | `key` | `internal/plugins/config-storage/yang/ze-storage-cli-cmd.yang` |
| `show dns cache record` | `name` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `show env get` | `name` | `internal/plugins/env/yang/ze-env-cmd.yang` |
| `show firewall irr prefix` | `name` | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` |
| `show firewall ruleset` | `name` | `internal/component/firewall/yang/ze-firewall-cmd.yang` |
| `show interface name counters` | `name` | `internal/component/iface/yang/ze-iface-interface-cmd.yang` |
| `show interface name detail` | `name` | `internal/component/iface/yang/ze-iface-interface-cmd.yang` |
| `show interface type` | `type` | `internal/component/iface/yang/ze-iface-interface-cmd.yang` |
| `show metrics name` | `name` | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` |
| `show vpn ipsec peer name` | `name` | `internal/component/ike/yang/ze-ipsec-cmd.yang` |
| `subscriber id detail` | `id` | `internal/component/l2tp/subscriber/cmd/yang/ze-subscriber-cmd.yang` |
| `tag` (withdraw form, two paths) | `key` | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` |

#### Class A (26 nodes) -- the rule does not reach these

| Command path | Value slot | Module |
|---|---|---|
| `bfd session address` | `address` | `internal/component/bfd/yang/ze-bfd-cmd.yang` |
| `blackhole` (announce form, two paths) | `prefix` | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` |
| `clear firewall irr as-set` | `as-set` | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` |
| `clear firewall irr asn` | `asn` | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` |
| `request l2tp outgoing-call remote called` | `remote`, `called` | `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` |
| `request log level` | `logger`, `target` | `internal/plugins/log/yang/ze-log-cmd.yang` |
| `resolve cymru asn-name` | `asn` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve dns a` | `hostname` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve dns aaaa` | `hostname` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve dns ptr` | `ip-address` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve dns txt` | `hostname` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve peeringdb as-set` | `asn` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve peeringdb max-prefix` | `asn` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `resolve ping` | `target` | `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang` |
| `resolve traceroute` | `target` | `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang` |
| `show bgp irr check` | `peer`, `prefix` | `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` |
| `show bgp irr prefix` | `peer` | `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` |
| `show dns lookup` | `hostname` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `show resolve rir` | `asn` | `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` |
| `show route lookup` | `ip` | `internal/component/iface/yang/ze-iface-show-cmd.yang` |
| `show tcp-check` | `host`, `port` | `internal/plugins/diag/yang/ze-diag-cmd.yang` |
| `unicast` (announce form, two paths) | `prefix` | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` |
| `update bgp irr as-set` | `as-set` | `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` |
| `update bgp irr asn` | `asn` | `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` |
| `update firewall irr as-set` | `as-set` | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` |
| `update firewall irr asn` | `asn` | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` |

`show bgp irr check` and `show bgp irr prefix` sit in class A on a leaf name
that reads like a selector. Their `peer` leaf names WHICH PEER'S POLICY to
evaluate a prefix against, so it is an input to one computation rather than a
narrowing of a set of answers. Q3 covers them.

## The Grammar, Before and After

Recommended reading of the rule, for the view commands. Rows marked "pending
Q1" show the form that follows if Thomas answers that the rule reaches actions
as well as views.

| Before | After |
|--------|-------|
| `show bgp peer list` | `show bgp peer` |
| `show bgp peer <sel> detail` | `show bgp peer \| limit <sel> \| extensive` |
| `show bgp peer <sel> capabilities` | `show bgp peer capabilities \| limit <sel>` |
| `show bgp peer <sel> statistics` | `show bgp peer statistics \| limit <sel>` |
| `show bgp peer <sel> history` | `show bgp peer history \| limit <sel>` |
| `show bgp peer <sel> rib` | `show bgp peer rib \| limit <sel>` |
| `show bgp rib \| peer <sel>` | `show bgp rib \| limit <sel>` |
| `show bgp` | `show bgp` (unchanged) |
| `show bgp \| summary` | `show bgp \| summary` (unchanged) |
| `show policy chain peer <sel>` | `show policy chain \| limit <sel>` |
| `show policy test peer <sel> direction <d>` | `show policy test direction <d> \| limit <sel>` |
| `request interface <name> up` (pending Q1) | `request interface up \| limit <name>` |
| `request interface migrate from <a> to <b> address <p>` | unchanged; the node stops declining an inherited leaf because no ancestor declares one |
| `request peer <sel> teardown <subcode>` (pending Q1) | `request peer teardown <subcode> \| limit <sel>` |
| `request peer <sel> pause` (pending Q1) | `request peer pause \| limit <sel>` |
| `peer raw <sel> <bytes>` (pending Q1) | `peer raw <bytes> \| limit <sel>` |
| `peer update <sel> <payload>` (pending Q1) | `peer update <payload> \| limit <sel>` |
| `peer <sel> announce unicast <prefix>` (pending Q1) | `announce unicast <prefix> \| limit <sel>` |
| `delete bgp peer <sel>` (pending Q1) | `delete bgp peer \| limit <sel>` |
| `update bgp peer <sel> prefix` (pending Q1) | `update bgp peer prefix \| limit <sel>` |

The `peer raw` and `peer update` word order is settled by the same change:
today they publish `peer raw <selector>` while every sibling under `peer`
publishes `peer <selector> announce`. After the change no command under `peer`
carries a positional selector, so the inconsistency has nothing left to sit in.

## Open Questions (BLOCKING -- Thomas decides)

| ID | Question | Why it cannot be answered here |
|----|----------|--------------------------------|
| Q1 | Does the rule reach ACTION commands, or VIEW commands only? | Two readings, both coherent. See below. |
| Q2 | Is `extensive` a fixed alias over the fields the bare answer already carries, or a distinct server call returning a richer record? | `detail` today is a separate wire method answering more fields than `list`. An alias cannot fetch data the answer does not hold, so if the richer record is wanted the bare command must always fetch it and `\| extensive`/nothing selects among fields. That costs bandwidth on every bare call. The alternative is `extensive` as an owned filter that folds into the command and reaches the handler, which is cheap but makes `extensive` a server word rather than a rendering word. |
| Q3 | Does the rule reach class K, the 31 nodes whose value is typed by a `name`/`id`/`type`/`key` keyword? | `show interface name <n> detail` already satisfies the ambiguity test: `name` is a keyword, so no slot accepts both a keyword and a name. The rule's ambiguity clause does not bind them. Its uniformity clause arguably does: `show interface \| limit <n> \| extensive` is the same sentence as the peer one. Converting them is 31 more nodes and touches nine components. |
| Q4 | `show bgp rib` today has `\| peer <sel>`, which is `\| limit <sel>` under a second name. Rename it? | Recommended yes, on habit 1 of `ai/rules/writing.md`. It is listed as in-scope above; the question is whether Thomas wants it in this spec or separately. |
| Q5 | What does an operator who types the old form see? | Ze is unreleased, so `ai/rules/cli.md` says an unreleased grammar is replaced outright, not deprecated. `internal/component/config/retired.go` is CONFIG-only and offers no mechanism here. The old form therefore fails as an unknown command with the existing `suggest.Command` hint. A message naming the replacement would be new machinery. |

### Q1 in full: the two readings

**Reading A -- views only.** A pipe reads as "take this answer and narrow it",
which is what a view does. An action has no answer to narrow: the pipe would be
choosing the TARGET of a side effect, which is a second meaning for the same
character. Under A, `show bgp peer | limit edge1 | extensive` converts and
`request peer edge1 teardown 2` does not.

**Reading B -- everything.** One grammar for the whole tree. An operator who
learns `| limit` at `show` uses it at `request` and at `peer`, and the rule's
own words are "review all the command with a selector", which does not
distinguish. Under B, `peer announce unicast 10.0.0.0/24 | limit edge1`.

**Where reading B is dangerous.** In every row below, the selector is what
BOUNDS the blast radius, and a missing or mistyped pipe segment widens it
rather than narrowing it. Today the selector is a mandatory positional, so the
command does not parse without it, and six of these also carry
`RequiresSelector: true`, which refuses the invocation server-side. A pipe
segment has neither property: an absent filter means "no filter", which for
these commands means "every peer".

| Command | What a missing selector does | Guard today |
|---------|------------------------------|-------------|
| `request peer <sel> teardown <subcode>` | tears down EVERY session | mandatory leaf + `RequiresSelector` |
| `delete bgp peer <sel>` | removes every peer from the running config | mandatory leaf + `RequiresSelector` |
| `peer raw <sel> <bytes>` | injects unframed bytes into every peer's TCP stream | mandatory leaf |
| `peer <sel> announce unicast <prefix>` | originates the route to every peer | the bare path already exists and is deliberate, so this one is a rename rather than a new hazard |
| `peer <sel> withdraw all` | withdraws every tagged announcement from every peer | the bare path already exists |
| `peer update <sel> <payload>` | sends the UPDATE to every peer | mandatory leaf |
| `request peer <sel> pause` | stops reading every peer's socket | mandatory leaf + `RequiresSelector`; the handler also refuses a wildcard |
| `update bgp peer <sel> prefix` | rewrites max-prefix for every peer in the draft | mandatory leaf + `RequiresSelector` |
| `request interface <name> down` | takes every interface down | mandatory leaf |

If Thomas answers B, the spec owes a replacement guard: a command declares that
its `limit` filter is REQUIRED, and the dispatcher refuses the command when the
folded arguments carry no `limit`. That is the `RequiresSelector` contract moved
to the pipe layer, and it must land in the same change as the conversion, never
after it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A command-owned pipe filter reaches the handler as a command argument | `foldFilters` in `internal/component/command/pipe.go` appends `<filter-name> <value>` to the command string; `show bgp rib \| peer <sel>` is the production instance | `limit` cannot narrow server-side and the whole design collapses to local row filtering, which cannot express a glob or `as65001` against rows the daemon never sent | a unit test over `foldFilters` asserting the rewritten command string, plus a `.ci` asserting the daemon answers only the matched peer | unvalidated |
| A-2 | Two operators chain, with the narrowing one folding and the shaping one staying local | `parsePipeOps` splits on every `\|`; `expandAliases` runs before `foldFilters` | `\| limit <sel> \| summary` cannot be expressed and the three roles cannot be separated | `TestLimitFoldsAndSummaryStaysLocal` | unvalidated |
| A-3 | `limit` collides with no registered alias or filter | `RegisterAliases` and `RegisterPipeFilters` each panic on an overlapping-path name collision; no catalog operator is named `limit` and no alias is | the daemon panics at init | `./le verify current mode full` starting the daemon in the functional suite; a unit test asserting the registration does not panic | unvalidated |
| A-4 | Removing `container list` changes the field set the bare command answers, not the peer population | `handleBgpPeerList` and `handleBgpPeerDetail` both call `filterPeersByArgs` over `ctx.Reactor().Peers()` | the bare form would silently answer a different peer set, which is a correctness change nobody asked for | `TestBarePeerAnswersEveryPeer` comparing the bare answer's row count to the reactor peer count | unvalidated |
| A-5 | The selector slot has no value completion today, so the conversion loses none | no `ValueHints` is wired for any peer or interface node; `pipeSubArgs` holds `json` and `fill` alone | the change would remove working peer-name completion, which is a regression | a `.ci` under `test/ui/` asserting what `show bgp peer <TAB>` offers before and after | unvalidated |
| A-6 | Nine components own converted commands and each owns its own registration | the Module column of the inventory | a central edit would be needed, which `ai/rules/principles.md` forbids | the implementation touches only owner packages; `./le tier check` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reading B lands without the replacement guard, so a mistyped line tears down every session | a converted action command runs with no `limit` and does not refuse | the required-filter guard lands in the same change as the first action conversion; the wiring test for it is written first |
| R-2 | `limit` registered on `show bgp` shadows the RIB plugin's own filter set, or the reverse | `RegisterPipeFilters` panics at init, or `show bgp rib \| limit` reports an unknown filter | lookup is longest-prefix, so `show bgp rib` needs its own `limit` entry rather than inheriting one; Q4 settles the rename in the same edit |
| R-3 | The grammar gate cannot express the new rule, so it is enforced by review alone | the checker has no rule id for it and `./le cli-grammar` stays green over an unconverted node | the rule lands as a new rule id in `grammar.CheckNode` with the population count printed, before the first conversion |
| R-4 | Completion offers `limit` but cannot complete its argument, so the operator gets less help than the positional form gave | `pipeSubArgs` is a hand-written map with two entries | A-5 says the positional form offers nothing either; if it turns out to, the spec gains a value-hint route for filter arguments |
| R-5 | Documentation and demos carry the old forms in prose and in recorded terminal sessions | `./le cli-grammar` reads `demos/terminal/` sources and refuses a non-verb position-1 token; the doc pages carry the six-row incorrect/correct table verbatim | the doc and demo edits land in the same phase as the YANG edit, never in a closing pass (`ai/rules/documentation.md`) |
| R-6 | 89 nodes across nine components is more than one spec's worth of work | the phase list stops converging | the package is cut by class: S-inherited and S-own first, class K only if Q3 says yes, and the main thread re-cuts if the phase count grows |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every operator command line, every script that calls `ze`, every `.ci` fixture that types a converted command, every documentation example, and every recorded terminal demo. Under reading B, a wrong guard means a command with no selector acts on every peer: sessions dropped, routes withdrawn, config entries deleted. |
| How is it reverted? | A single commit revert restores the YANG and the registrations. No config migration and no on-disk state is involved: the command tree is built at startup from the schema. |
| Who else touches this path? | `plan/spec-cli-pipe-operator-coverage.md`, `plan/spec-cli-show-bgp-answer-shapes.md`, `plan/spec-announce-grammar-stated-and-enforced.md` and `plan/spec-cli-root-namespace-grammar-deferred-gate-reach.md` all work the command grammar or the pipe layer. `plan/journal/command-takes-an-untyped-positional-value.md` and `plan/journal/helper-bypassed-by-an-open-coded-copy.md` collect rows in this area. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| operator types `show bgp peer \| limit edge1` | → | `foldFilters` rewrites the command to `show bgp peer limit edge1`, `filterPeersByArgs` reads it | `TestLimitFoldsIntoTheCommandString` |
| operator types `show bgp peer \| limit edge1 \| extensive` | → | `foldFilters` folds `limit`, the shaping operator stays in the local chain | `TestLimitFoldsAndShapingStaysLocal` |
| operator types `show bgp peer` with no pipe | → | `handleBgpPeerList` over every peer | `test/ui/limit-bare-peer-answers-all.ci` |
| operator types `\|` and presses Tab after `show bgp peer` | → | `pipeExtras` returns the owned `limit` filter | `TestCompletionOffersLimitUnderShowBgpPeer` |
| a YANG node declares a mandatory selector in a path slot | → | `grammar.CheckNode` new rule | `TestCheckerRefusesAPositionalSelector` |
| under reading B: a converted action command runs with no `limit` | → | the required-filter guard | `TestActionRefusesAnAbsentLimit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any command node in the merged YANG tree | No slot in its path accepts both an operator-supplied value and a fixed keyword. `ze:inherit "none"` has no user left, and the extension is deleted with its last one. |
| AC-2 | `show bgp peer` typed with nothing after it, against a daemon with three configured peers | Answers a row for each of the three, in the one-line field set `list` answered before: name, address, ASN, state, uptime. |
| AC-3 | `show bgp peer \| limit <sel>` for each selector kind: an address, a peer name, `as65001`, a glob, a comma list, `*` | Answers exactly the peers `peersel.ParseDefault` matches, and the narrowing happens in the daemon: the answer sent over the wire carries the matched peers alone. |
| AC-4 | `show bgp peer \| limit <sel> \| <shape>` | Both operators take effect: the answer is narrowed to the selector AND shaped by the second operator. The chain order is honored. |
| AC-5 | `peer raw` and `peer update` | Their token order matches every sibling under `peer`. No command under `peer` carries a positional selector. |
| AC-6 | `show bgp rib \| limit <sel>` | Narrows by peer, and `\| peer <sel>` is no longer a spelling of it. |
| AC-7 | Tab pressed after `\|` on any converted command | `limit` is offered, with its description. |
| AC-8 | `./le cli-grammar` over the real checkout | Passes, and prints the size of the population it read for the new rule. A YANG node reintroducing a positional selector makes it fail, naming the node. |
| AC-9 | Under reading B only: a converted action command invoked with no `limit` | Refused, naming `limit`, with no side effect. The refusal is server-side, so it holds for a client that skips the CLI. |
| AC-10 | Every documentation page, rule file and terminal demo that showed a converted command | Shows the new form. `docs/architecture/cli/root-namespace-grammar.md`, `ai/rules/cli.md` and `ai/patterns/cli-command.md` state the new grammar and no longer state the old one. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | types `show bgp peer` to see every peer at a glance | CLI → tree match → `handleBgpPeerList` over every peer → `ApplyPipes` | `test/ui/limit-bare-peer-answers-all.ci` |
| 2 | types `show bgp peer \| limit edge1` to see one peer | CLI → `foldFilters` → command string → dispatcher → `filterPeersByArgs` | `test/ui/limit-narrows-to-one-peer.ci` |
| 3 | types `show bgp peer \| limit edge1 \| extensive` for one peer in full | CLI → `expandAliases` → `foldFilters` → daemon → `ApplyPipes` with the shaping operator | `test/ui/limit-then-shape-chains.ci` |
| 4 | presses Tab after `show bgp peer \|` | `completePipeForCommand` → `pipeExtras` → `filterSuggestions` | `test/ui/limit-completes-after-pipe.ci` |
| 5 | under reading B: tries to tear a session down and forgets the selector | CLI → dispatcher → required-filter guard → refusal | `test/ui/limit-required-on-action.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLimitFoldsIntoTheCommandString` | `internal/component/command/pipe_test.go` | `foldFilters` rewrites `show bgp peer \| limit edge1` to `show bgp peer limit edge1` and leaves an empty local chain | |
| `TestLimitFoldsAndShapingStaysLocal` | `internal/component/command/pipe_test.go` | a two-operator chain splits correctly between the folded half and the local half, in chain order | |
| `TestLimitRegistrationDoesNotCollide` | `internal/component/command/alias_test.go` | registering `limit` beside the `summary` and `peers` aliases on `show bgp` does not panic | |
| `TestCompletionOffersLimitUnderShowBgpPeer` | `internal/component/command/completer_test.go` | `pipeExtras` returns `limit` for each converted command path | |
| `TestCheckerRefusesAPositionalSelector` | `internal/component/command/grammar/checker_test.go` | the new rule fires on a node declaring a mandatory free-form value in a path slot, and does not fire on a class A argument | |
| `TestTheRealCheckoutHasNoPositionalSelector` | `internal/le/cligrammar/cligrammar_test.go` | the whole checkout passes the new rule, and the population count is non-zero | |
| `TestBarePeerAnswersEveryPeer` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | the bare command's row count equals the reactor's peer count | |
| `TestLimitSelectsEachSelectorKind` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | address, name, `as65001`, glob, comma list and `*` each match the same peers they matched positionally | |
| `TestActionRefusesAnAbsentLimit` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | reading B only: the guard refuses and the handler is never reached | |
| `TestInheritExtensionHasNoUser` | `internal/component/config/yang/...` | no module declares `ze:inherit`, so the extension is removable | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `limit` argument length | 1..1024 bytes (`maxArgLength`, `argvalidate.go`) | 1024 | 0 (empty argument, refused by `validateFilter`) | 1025 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `limit-bare-peer-answers-all` | `test/ui/limit-bare-peer-answers-all.ci` | the operator types `show bgp peer` and sees every configured peer, one line each | |
| `limit-narrows-to-one-peer` | `test/ui/limit-narrows-to-one-peer.ci` | the operator narrows to one peer with `\| limit`, and the answer carries that peer alone | |
| `limit-then-shape-chains` | `test/ui/limit-then-shape-chains.ci` | narrowing and shaping compose in one chain | |
| `limit-completes-after-pipe` | `test/ui/limit-completes-after-pipe.ci` | Tab after `\|` offers `limit`; Tab in the old selector position offers no peer value, because the slot is gone | |
| `limit-old-form-is-refused` | `test/ui/limit-old-form-is-refused.ci` | `show bgp peer list` and `show bgp peer edge1 detail` are both refused, and the refusal names the command | |
| `limit-required-on-action` | `test/ui/limit-required-on-action.ci` | reading B only: an action with no `limit` is refused and nothing changed | |

### Interop Tests (Scope: protocol)
Not applicable. This is a CLI grammar change with no wire-visible effect on any
protocol. `ai/rules/interop-and-goal-validation.md` exempts "tooling with no
protocol peer" and "a config-only feature with no protocol impact"; the peer
sessions the converted commands act on are unchanged in every byte they send.

## Files to Modify
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - remove the three `leaf selector` declarations and `ze:inherit "none"`; rewrite the `ze:help` text that describes the positional selector
- `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang` - remove `leaf selector`
- `internal/component/bgp/plugins/cmd/update/yang/ze-update-cmd.yang` - remove `leaf selector`
- `internal/component/bgp/plugins/cmd/policy/yang/ze-policy-cmd.yang` - remove the two `leaf selector` declarations
- `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang` - remove the `selector` leaf of `request cache forward`
- `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - remove the `peer <selector>` instantiation of both groupings (pending Q1)
- `internal/component/iface/yang/ze-iface-cmd.yang` - remove `leaf name` from `container interface` and `ze:inherit "none"` from `migrate` (pending Q1)
- `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - remove the inherited `name` (pending Q3)
- `internal/component/bgp/plugins/cmd/peer/peer.go` - register the `limit` filter for each converted path; the six `RequiresSelector` registrations under reading B
- `internal/component/bgp/plugins/cmd/rib/rib.go` - rename the `peer` filter to `limit` (Q4)
- `internal/component/command/pipe_filter.go` - a `Required` field on `PipeFilter` under reading B, and its refusal
- `internal/component/command/pipe.go` - the required-filter check in `foldFilters` under reading B
- `internal/component/command/grammar/checker.go` - the new rule in `CheckNode`
- `internal/component/command/completer.go` - only if R-4 turns out to bind: a route from an owned filter to its argument completions
- `ai/rules/cli.md` - rewrite the keyword-before-value directive and the `list` directive
- `ai/patterns/cli-command.md` - rewrite the peer exception, the Peer Selector Mechanism section, the Command Classes table and the Full Command Inventory
- `docs/architecture/cli/root-namespace-grammar.md` - rewrite the command shape block and the incorrect/correct table (Design doc of `grammar/checker.go`)
- `docs/architecture/cli/command-namespacing.md` - the Design doc `grammar/checker.go` declares
- `docs/architecture/api/commands.md` - the Design doc of `pipe.go`, `pipe_filter.go`, `pipe_catalog.go`, `alias.go`, `completer.go`, `peer.go` and `rib.go`; it carries the pipe operator language and the command-owned filter section
- `docs/guide/command-reference.md` - every converted command's published form
- `demos/terminal/` - any recorded session typing a converted command

## Files to Create
- `test/ui/limit-bare-peer-answers-all.ci` - the bare form answers for all
- `test/ui/limit-narrows-to-one-peer.ci` - narrowing through the pipe
- `test/ui/limit-then-shape-chains.ci` - two operators compose
- `test/ui/limit-completes-after-pipe.ci` - the completion surface follows the tree
- `test/ui/limit-old-form-is-refused.ci` - the old grammar is gone rather than aliased
- `test/ui/limit-required-on-action.ci` - reading B only

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | the eight modules named in Files to Modify; no RPC is added or removed, only the value slots each command declares |
| YANG validation constraints | Yes | the deleted leaves carried `type string` with no constraint; the `limit` argument is validated by `validateFilter` and `maxArgLength` instead |
| YANG custom validators | N-A | the selector vocabulary is parsed by `peersel.ParseDefault` at the handler, which is unchanged |
| CLI commands/flags | Yes | `internal/component/bgp/plugins/cmd/peer/peer.go` and every other owner package registering `limit` |
| CLI grammar (keyword before value) | Yes | this spec IS the grammar change; `grammar.CheckNode` gains the rule |
| Editor autocomplete | Yes | `pipeExtras` reaches the editor's command mode through the shared `TreeCompleter`; `test/ui/limit-completes-after-pipe.ci` covers it |
| Functional test for new RPC/API | Yes | the six `.ci` files under `test/ui/` |
| Pipe completeness | Yes | `limit` is an owned filter, so it folds; every other operator keeps its catalog contract and `ApplyPipes` is unchanged |
| Env var registration | N-A | no environment leaf is added |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module, binary or certificate |
| Prometheus counters/metrics | N-A | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- `limit` is a new operator every converted command answers |
| 2 | Config syntax changed? | No | no config keyword, container or leaf under `config true` changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- the operator language and the owned-filter section |
| 5 | Plugin added/changed? | No | no plugin is added, removed or re-homed |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md`, and any `docs/guide/` page showing a converted command |
| 7 | Wire format changed? | No | no protocol byte changes |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` if the plugin command surface publishes the selector; verify before answering No |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC requirement is touched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- six new `test/ui/` fixtures |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares protocol features |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/root-namespace-grammar.md`, `docs/architecture/cli/command-namespacing.md` |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` -- the command inventory changes for 32 or more commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-fixit-selector-narrows-through-a-pipe.md`. The `// Design:` headers of the changed Go files declare `docs/architecture/api/commands.md` and `docs/architecture/cli/command-namespacing.md`, and both are named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every example typing a converted command, in `docs/`, in `ai/`, and in `demos/terminal/` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove a folded `limit` reaches the handler before any command is converted
   - Tests: `TestLimitFoldsIntoTheCommandString`, `TestLimitFoldsAndShapingStaysLocal`, `TestLimitRegistrationDoesNotCollide`
   - Files: `internal/component/bgp/plugins/cmd/peer/peer.go` (register `limit` on one path), `internal/component/command/pipe_test.go`
   - Verify: the filter registers, folds, and reaches `filterPeersByArgs`, while the positional selector still works. Nothing is removed yet
2. **Phase: the gate** -- the rule before the conversions, so no converted node can regress and no unconverted node hides
   - Tests: `TestCheckerRefusesAPositionalSelector`, `TestTheRealCheckoutHasNoPositionalSelector`
   - Files: `internal/component/command/grammar/checker.go`, `internal/le/cligrammar/`, `docs/architecture/cli/command-namespacing.md`
   - Verify: the rule fires on all 32 class S nodes and on none of class A. It is expected RED against the unconverted tree, and the count it prints is the work remaining
3. **Phase: reading B's guard** -- only if Q1 answers B; skipped entirely under A
   - Tests: `TestActionRefusesAnAbsentLimit`, `test/ui/limit-required-on-action.ci`
   - Files: `internal/component/command/pipe_filter.go`, `internal/component/command/pipe.go`
   - Verify: a command declaring a required filter is refused without it, server-side, before the handler runs
4. **Phase: the peer subtree** -- the eleven class S nodes under `show bgp peer`, `update bgp peer`, `request peer`, `delete bgp peer`, `peer raw` and `peer update`
   - Tests: `TestBarePeerAnswersEveryPeer`, `TestLimitSelectsEachSelectorKind`, `test/ui/limit-bare-peer-answers-all.ci`, `test/ui/limit-narrows-to-one-peer.ci`, `test/ui/limit-then-shape-chains.ci`, `test/ui/limit-old-form-is-refused.ci`
   - Files: the three peer YANG modules, `peer.go`, `rib.go` (Q4)
   - Verify: gate count drops by eleven; `show bgp | summary` and `show bgp | peers` still pass their existing `.ci`
5. **Phase: the rest of class S** -- `show policy`, `request cache forward`, `announce`/`withdraw`, and the interface subtree
   - Tests: one `.ci` for each owner component's converted command
   - Files: the remaining modules and owner packages named in Files to Modify
   - Verify: the gate reaches zero for class S
6. **Phase: class K** -- only if Q3 answers yes
   - Tests: per-component, same shape as phase 5
   - Files: the nine components in the class K table
   - Verify: the gate reaches zero for class K
7. **Phase: the extension and the documentation** -- delete `extension inherit` once it has no user; complete the doc edits each phase started
   - Tests: `TestInheritExtensionHasNoUser`, `./le doc check verify`
   - Files: `internal/component/config/yang/modules/ze-extensions.yang`, the pages in the Documentation checklist, `demos/terminal/`
   - Verify: `./le cli-grammar` green, `./le doc check verify` green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 32 class S nodes is converted, or the class is reduced by Thomas in writing. The gate's printed count is the evidence, not a reading of the diff |
| Feature completeness | Each converted command answers bare, narrows with `limit`, and chains with a shaping operator. A command that only does the first is half converted |
| Correctness | Every selector kind `peersel.ParseDefault` accepts still selects the same peers through the pipe as it did positionally |
| Guard integrity | The six `RequiresSelector: true` registrations still refuse a selector-less invocation. A converted action that can run unbounded is a rung-1 failure, not a finding |
| Naming | One concept, one name: `limit` everywhere, and `\| peer <sel>` on `show bgp rib` is gone rather than kept beside it |
| Data flow | The narrowing happens in the daemon, not locally. A `.ci` asserting the answer carries one peer must fail if the daemon sends all of them and the CLI drops the rest |
| Rule: `ai/rules/no-layering.md` | The positional selector is DELETED before `limit` is written for that command. No command accepts both spellings at any commit |
| Rule: `ai/rules/documentation.md` | Each phase's page edits land in that phase. `/ze-close` verifies them; it is not where they are written |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No YANG node declares a mandatory positional selector | `./le cli-grammar` prints zero findings for the new rule and a non-zero population |
| `ze:inherit` has no user | `grep -rn 'ze:inherit' internal --include='*.yang'` answers nothing, and the extension is deleted |
| `limit` is registered for every converted command | a unit test walking the converted command paths and asserting `PipeFiltersForCommand` names `limit` |
| The bare form answers for all | `test/ui/limit-bare-peer-answers-all.ci` |
| Narrowing is server-side | `test/ui/limit-narrows-to-one-peer.ci` asserting the wire answer, not the rendered text |
| The old forms are gone | `test/ui/limit-old-form-is-refused.ci` |
| Documentation matches | `./le doc check verify`; `./le spec citation anchors spec plan/spec-fixit-selector-narrows-through-a-pipe.md` reports no unnamed owner |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Blast radius of an absent selector | Under reading B, an action command with no `limit` MUST refuse. The refusal is server-side so a non-CLI client cannot bypass it. This is the whole security content of the change |
| Input validation | The `limit` argument reaches `peersel.ParseDefault` exactly as the positional value did. `validateFilter` refuses an empty argument and `maxArgLength` bounds it at 1024 bytes |
| Authorization | `request peer` carries `ze:ui-permissions "network"`. Moving the selector out of the path must not move the command out of that permission's scope |
| Error leakage | A refusal names the filter and the command, never the peer list the operator was not allowed to see |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| The gate's population count is zero | The rule reads no node. STOP: a green gate that read nothing is the failure `./le cli-grammar` prints population sizes to prevent |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The narrowing operator needs no new pipe machinery. `foldFilters` already pushes an owned, argument-taking filter down to the handler as command arguments, and `show bgp rib | peer <sel>` has been doing exactly this narrowing for months. What the tree lacks is not the mechanism but the decision to use it uniformly.
- `list` reads like a filter and is a format. Both `handleBgpPeerList` and `handleBgpPeerDetail` filter the same population the same way; only the field set differs. A reader who assumes `list` is a filter designs `| limit` as its replacement and loses the one-line answer entirely.
- The positional selector is doing a second job nobody named: it is a fail-closed guard. A mandatory leaf cannot be forgotten, and a pipe segment can. That is the `zero-value-as-a-guard` shape `docs/contributing/ze-go-style.md` warns about, arriving in a grammar rather than in a type.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `limit` is a command-owned `PipeFilter`, not a global catalog operator | a global operator in `pipeCatalog` | a global operator acts on the answer LOCALLY, after the daemon has already sent every peer. The narrowing must reach the handler, and `foldFilters` is the only route that does. It is also the route `show bgp rib` already uses |
| `limit` is a new name rather than a reuse of `match` | reuse `match <text>` | `match` is a substring test over rendered row values. It cannot express a glob, `as65001`, or a comma list, and it cannot push down. Naming the new job `match` would give one word two meanings |
| The bare command answers the one-line shape that `list` answered | keep `list` as a shaping pipe operator | Thomas's `show bgp (all)` says the bare form is the everything form. A shaping operator for the FULL record (`extensive`) is the additive half; a shaping operator for the SHORT record would leave the bare form undefined |
| The old spellings are removed, not aliased | keep the positional form as a second spelling | `ai/rules/cli.md`: Ze is unreleased, so an unreleased grammar is replaced outright. `ai/rules/no-layering.md`: delete X, then write Y |
| The rule is enforced by a checker rule before any conversion | convert first, gate afterwards | a gate written after the conversions cannot be seen to go red, so it proves nothing (`ai/rules/interop-and-goal-validation.md`, the vacuity traps) |

## Known Limitations
- Class A is untouched by design: those 26 values are the command's subject or an argument to it, not a narrowing of a set. `show bgp irr check peer <p> prefix <x>` is the closest call and Q3 covers it.
- The published inventory in this spec is a snapshot. Implementation replaces it with the walk inside the checker, and the count that walk prints is the authority from then on.
- Value completion for the `limit` argument is out of scope unless A-5 is broken: no peer-name or interface-name value hints exist today at either spelling.

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
- [ ] AC-1..AC-10 all demonstrated
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
