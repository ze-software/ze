# CLI and Output

**When:** adding or changing any CLI command, flag, exit code, output format, error message, JSON envelope, or agent-facing contract
**Severity:** blocking
**Related:** evidence, performance, protocol, repo-maintenance, git-safety

## Directives

- **Every CLI command MUST place a closed keyword before any user-supplied value.** This eliminates ambiguity where a free-form value could collide with a keyword.

- **All CLI commands MUST follow the patterns in "CLI Patterns" below.** Structural template: `ai/patterns/cli-command.md`. Rationale: `ai/rationale/cli-patterns.md`.

- **Every command that produces output MUST answer every GLOBAL operator on every surface where the catalog makes that operator available.**
  - **Global formats:** Global means independent of answer shape, not execution surface. The six formats and `no-more` act on every answer everywhere.
  - **Local-only save:** `save` is catalogued `local-only`. It MUST be refused by name when a daemon expands a remote SSH or web chain.
  - **Streaming log:** A STREAM operator such as `log` is available only `when-streaming`, where the command keeps answering.
  - **Qualifier preservation:** Published command contracts and every consumer MUST preserve `always`, `with-rows`, and `when-streaming`. They MUST also preserve the independent `local-only` surface restriction. They MUST NOT flatten either qualifier into unconditional support.
  - **Catalog derivation:** The operator catalog (`internal/component/command/pipe_catalog.go`) is the source for class and surface. Each command's availability is derived from that catalog and its declarations, never from a hand-copied list.

- **Every command MUST answer every operator its answer's SHAPE supports, and MUST refuse the rest by name, saying why.** An answer holding one value has no rows, so `first`, `last`, `match`, `count`, `display` and `fill` have nothing to act on there, and `| origin` over a version string is meaningless. Refusing is the requirement, not a permission: accepting an operator and answering something is worse than refusing it, because the answer looks plausible and a caller cannot tell. `show bgp | count` answered 6, the number of top-level keys, for as long as that was allowed. A command DECLARES its shape with `RegisterShape` so the refusal can happen before it runs and so the published page can state what it supports; an undeclared command is still refused, from the shape of the answer in hand. These two directives replaced one that read "every command that produces output MUST support all pipe operators", which could not be met and could not be gated: it made a claim the product did not meet and a generated wiki page that repeated the claim on 381 entries.

- **A command's response payload MUST be structured data. It MUST NOT be text a renderer already formatted.** `| json`, `| yaml` and `| table` are three renderings of ONE payload, and a handler that answers with finished text has picked the reader's format for them. `ResponseData` (`internal/component/plugin/types.go`) is what keeps a bare string out of `Response.Data`. `Map`, `Slice[T]` and a struct embedding `DataMarker` satisfy it with structure. `RawJSON` satisfies it too and is the one implementor that can carry finished text past the compiler, so it MUST hold `json.Marshal` output over a value. A decoder or formatter that can emit either shape MUST be asked for the structured one, and the pipe layer renders it.

- **A row's state MUST be a field or a column; it MUST NOT be a character glued to a value.** No `*`, `>`, `+` or leading dot on an identifier. A sigil corrupts the value for `| grep` and has nowhere to live in `| json`, so the text and JSON forms stop agreeing on what the value is. `*` is also already an input token here, the selector wildcard. See "A value carries no marker" below.

- **Every error, log line, and failure output you write MUST let a human or an agent see what failed, why, and what to do next, without opening the source.** The error is the corrective signal: if it does not point at the fix, the reader cannot act and an agent cannot self-correct.

- **All JSON output MUST follow the conventions in "JSON Format" below.** Rationale: `ai/rationale/json-format.md`.

- **All agent-facing CLI output MUST follow the rules in "Agent Tooling Contract" below.**

## CLI Grammar: Keywords Before Values

```
<verb> <noun> <action> [<args>]
<verb> <noun> <selector-kind> <selector-value> <action> [<args>]
```

The first token after the noun (component/resource) MUST be a keyword from a
closed set known at compile time. If a command targets one member of a set,
the selector itself MUST also be typed by a keyword such as `name`, `id`,
`index`, `address`, or `type`. Free-form values MUST NOT appear in an untyped
positional slot.

Use an explicit selector kind whenever a command addresses one member of a set.

A selector keyword MUST be one of these:

- `name <name>` for operator-assigned names
- `id <id>` for string or numeric IDs
- `index <index>` for positional or kernel indexes
- `address <ip>` for IP-address lookup
- `type <type>` for category filters
- `key <key>` for generic key
- another schema-defined closed keyword when the domain needs something more specific

Examples:

Typed-selector commands MUST take this form:

- `show interface type dummy`
- `show interface name eth0 detail`
- `show interface name eth0 counters`
- `show sysctl key net.ipv4.ip_forward`
- `show sysctl profile router`

Peer commands are the explicit exception to the generic typed-selector
keyword form.

Use:

Peer commands MUST take this form:

- `show bgp peer <name|address> detail`
- `show bgp peer <name|address> rib`

Do not invent mutating peer examples here unless the exact grammar was
explicitly agreed in source or by the user.

Do not invent a user-facing `selector` keyword. `selector` may exist as an
internal concept in dispatcher code, but it MUST NOT leak into operator
syntax.

The "named-resource" pattern `<resource> <id> <action>` violates this rule.
Correct form: `<resource> <action> <id>`.

The action MUST come before the identifier:

- `cache retain <id>`, not `cache <id> retain`
- `commit start <name>`, not `commit <name> start`

The `list` action (no identifier) already works correctly in both.

A hyphen inside a command token joins words that name **one indivisible thing**.
Two separate ideas are two tokens, never one hyphenated token.

Decision test, in order:

1. Would you naturally say the two parts separately about the object ("show the
   *health* of *bgp*", "show the *feature* signals for *traffic*")? If yes, they are
   two tokens. The left part becomes a container node so the tree stays object-rooted
   and completion can enumerate the members
   (`docs/architecture/cli/command-namespacing.md`).
2. Is the whole string the actual name of one thing you would never break apart? An
   industry term of art (`as-set`, `graceful-restart`, `segment-routing`,
   `adj-rib-in`, `class-of-service`), a protocol / LSA / object name (`opaque-area`,
   `asbr-summary`, `router-information`), or a single attribute (`asn-name`,
   `max-prefix`, `file-descriptors`). If yes, MUST keep the hyphen.
3. A shared prefix is not proof of a namespace. `flow-export` (NetFlow/IPFIX) and
   `flow-recent` (conntrack ring) share "flow" by accident; they are not
   `show flow {export,recent}`. MUST split only when the prefix is a real object that owns
   every child.
4. A split namespace needs one owning module. If several components share the prefix,
   one module owns the container and the others augment it (as `trafficusage` augments
   `traffic`). A shared parent that multiple plugins reach into breaks plugin
   self-containment, so it MUST NOT be created.

**Enforcement (R9).** Inside the YANG command tree, the static gate flags any child
token whose left segment is itself a sibling name at the same tree level
(`grammar.CheckSiblings`). R9-by-sibling needs sibling context, so ONLY the static gate
runs it: per-command registration cannot see siblings. It is deliberately conservative
and fires only when the namespace literally exists as a sibling, so a genuine compound
is never flagged. **Root commands are a separate surface:** they register via
`registry.MustRegisterRootHandler` / `RegisterRoot` and never enter the YANG tree, so
`CheckSiblings` cannot see them. The root-namespace feeder (`grammar.CheckRootNamespace`,
the fourth feeder below) governs them with a cross-surface check: a hyphenated root whose
left segment names a YANG verb or container is the same R9 violation (`traffic-control`
vs the `traffic` container). MUST NOT assume a root is ungoverned because it is not in the
tree.
Shipped commands awaiting the agreed rename are listed in `pendingNamespaceSplit`
(`internal/le/cligrammar/cligrammar.go`) and reported as tracked debt, so the gate stays green
while a NEW collision still fails. Migrating one is a dispatch-key change (see
"Migrating a Built-in Command's Path" below): MUST add the split path, keep the old form per
"Backward Compatibility", update `.ci` senders, and remove its `pendingNamespaceSplit`
entry.

**When the compound is genuine, MUST exempt it -- MUST NOT split it.** `CheckSiblings` fires on
the mere existence of a sibling matching the LEFT segment, so it cannot tell a real
namespace from two names that share a word by accident (test 3). When test 2 wins -- the
token is one indivisible protocol / LSA / object name -- MUST list the full command path in
`treeNamespaceExempt` (`internal/le/cligrammar/cligrammar.go`) with a one-line reason. It is the
tree-side counterpart of `rootNamespaceExempt`, is counted and printed (`Tree
namespace-exempt`), and leaves every unlisted collision blocking. MUST reach for it only when
splitting would state something false about the object model; `show ospf database
router-information` is the worked case (RFC 7770's RI LSA is an *Opaque* LSA, so filing
it under the `router` sibling -- the Type 1 Router-LSA -- would be wrong). Note the check
only ever looks left, so the same shared-word situation on the right (`summary` /
`asbr-summary`, `external` / `nssa-external`) is never flagged at all.

The verb is chosen by the command's **effect on live state**, not by how
"diagnostic" it feels. The deciding question:

> Does running this command change what the router does, emits, or forwards?

- **No -- it only reports current state.** Use `show` (one snapshot) or
  `monitor` (a continuous stream of the same read). These are the read-only
  verbs (`command.IsReadOnlyVerb`); they never alter protocol or dataplane
  state. **Deep introspection stays here:** a view that only *observes* internal
  state (`show ospf database opaque-area detail`, `show bgp peer name <n> rib`)
  is `show`, however low-level -- reading is not debugging.
- **Yes, and it is a normal operational action or lifecycle change.** Use the
  existing action verbs (`request`, `clear`, `create`, `set`/`delete`, `update`,
  `cache`). Not `debug`.
- **Yes, and the change is a deliberate diagnostic PERTURBATION of the running
  protocol/dataplane** -- inject, force, corrupt, drop, or toggle a fault/
  injection mode for testing or introspection. Use `debug`. A `debug` command
  changes the router's behaviour, so it MUST be double-gated: authz (`deny
  debug`) plus an explicit, fail-closed runtime enablement
  (see `internal/plugins/ospf/debug_enable.go`).

`debug` is a verb (first token) *and* a legitimate noun under a read verb
(`show debug` = display debug state). They do not collide: `show debug` reads,
`debug ...` perturbs.

Not `debug`: turning on verbose **logging** changes output, not protocol
behaviour -- model it as configuration (`set ... log level debug`), never as a
`debug` command. The `debug` verb is reserved for perturbing protocol/dataplane
state.

| Command | Verb | Why |
|---------|------|-----|
| `show ospf database opaque-area detail` | `show` | reads the LSDB; observes only |
| `monitor ospf ...` | `monitor` | streams events; observes only |
| `debug ospf inject enable` / `debug ip ospf inject opaque ...` | `debug` | injects a crafted LSA / toggles injection; perturbs the LSDB (double-gated) |
| `set ospf area <a> log level debug` | `set` | verbose logging is config, not perturbation |

Do not invent operational `add`, `del`, `remove`, or similar verbs for
objects that already live in the config YANG tree.

Config mutation belongs to the engine verbs:

Engine tree mutation MUST use these verbs:

- `set <path> <value>`
- `delete <path>`

If a change means "create/delete/change a node in the YANG config tree",
the command surface MUST stay in engine path form. Do not mirror that
operation as an RPC command just to make the grammar look regular.

Examples:

- Interface addresses and units are config-tree mutations. They MUST belong under
  engine `set` / `delete`, not operational commands like `interface addr del`
  or `interface unit remove`.
- Runtime actions that are not tree mutation, such as `clear counters`,
  `teardown`, or `pause`, MAY have operational command verbs.

Before changing any command grammar, classify it first:

Each command class MUST use this surface:

1. **Config tree mutation** -> engine `set` / `delete`
2. **Runtime operational action** -> CLI/RPC command grammar applies
3. **Read/query** -> `show ...`

If the command is class (1), stop. Do not redesign it as a verb-first RPC.

Do not move a command family into a different YANG module while fixing
grammar unless the architecture explicitly requires that move.

First identify the owning module from source:

The owning module MUST be recorded in one of these places:

- the existing `ze:command` path
- the module `register.go` / `embed.go`
- the handler `RPCRegistration`

Then edit that module. Grammar cleanup is not permission to reshuffle
command ownership across components.

The daemon dispatcher registers each built-in handler under its **YANG path**, not
its wire method: `LoadBuiltins` does `d.RegisterWithOptions(wireToPath[wireMethod], ...)`
(`internal/component/plugin/server/command.go`). So **moving a `ze:command` container
in the YANG tree changes the command's dispatch key.** Relocating `command list` to
`show command list` deletes the old `command list` key entirely.

That is fine for a command an operator types. It is a **wire break** for any command a
plugin or script sends by its bare path, over the plugin CLI protocol
(`dispatch-command` / `dispatch-command-args`) or an interactive `ze <subsystem> plugin cli`
session. A verb-first "rename" of such a command is a protocol break, not a cosmetic
change.

Before migrating any noun-first built-in to verb-first:

1. **Grep for programmatic senders of the bare path:** `.ci` tests, `DispatchCommand`
   / `DispatchCommandArgs` calls, `printf ... | ze ... plugin cli`, `SendCommand`. MUST
   update every one in the same change. (In-tree these are greppable; external plugins/scripts
   are not, so a hard cut with no deprecation carries that residual risk.)
2. The **programmatic plugin SDK is not affected**: it uses structured RPC wire methods
   (`ze-plugin-callback:*`, `pkg/plugin/sdk/sdk_dispatch.go`), not command-path strings.
   Only the interactive plugin CLI and `dispatch-command` carry command-path strings, and
   in-tree those are already verb-first (e.g. `bmp.go` sends `show bgp rib protocol`).
3. Some noun-first forms are a deliberate **namespace**, not un-migrated legacy. MUST keep
   them: `plugin encoding/format/ack` group plugin-session directives under `plugin`, and
   `set plugin` would collide with the config-tree `plugin` node (see Engine-Owned Tree
   Mutation). `command list/help/complete` (engine introspection) migrated cleanly to
   `show command ...`; `plugin ...` stayed.

When you migrate, also drop the command's verb from `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) if it was only there as a legacy
noun-first form.

Use string-typed identifiers even when the conventional representation is numeric.
Cache IDs, VLAN IDs, session IDs: accept and store as strings. Parse to numeric
only at the point of use if the underlying API requires it. This avoids:
- Grammar ambiguity between numeric keywords and numeric IDs
- Unnecessary coupling to a representation (IDs may become non-numeric later)
- Parsing errors surfacing at the wrong layer

Replace a wrong grammar outright. Ze has never been released, so no grammar is
owed to a user and none is kept working alongside its replacement: delete the
old form, then implement the new one (`ai/rules/go-standards.md`, "No Backwards
Compatibility"; `ai/rules/no-layering.md`).

For every handler that dispatches on `args[0]`:

A positional argument before the action or selector-kind MUST NOT be a
user-supplied value. Ask these questions to decide whether a handler conforms:

1. Is `args[0]` always a keyword from a known set? -> Correct.
2. If the command selects one member of a set, does the handler consume a
   selector-kind keyword before the free-form value? -> Correct.
3. Can any positional argument before the action or selector-kind be a
   user-supplied value? -> Violation.

If any `args[0]` usage passes the value to a lookup/parse function (GetInterface,
ParseUint, etc.) without first matching it against a keyword set, that is a violation.

These rules are enforced automatically, not just by review. The reverse-engineered
ruleset is R1-R9 (verb-first, token form, no `--flag`, namespace discipline,
keyword-before-value, action-before-identifier, config-tree-mutation stays in
`set`/`delete`, string identifiers, compound-vs-namespace split), implemented once in
`internal/component/command/grammar` and read from the canonical verb registry
`internal/component/command` (`Verbs`). Seven feeders enforce it:

Feeder 3 is an **in-process** guard, not a daemon-boot audit: built-ins are 100%
YANG-derived (a handler with no YANG path is skipped, `LoadBuiltinsWithAliases`) so
they are a strict subset of the static gate's tree, and plugin commands are rejected
at registration by Feeder 2. The merged `system command list` surface therefore
contains only conforming commands by construction: a boot-and-dump audit would add
no catch value while depending on an all-plugins config that cannot exist (startup is
config-path-gated). The guard instead locks the two runtime sources against
regression cheaply and deterministically.

The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface name <name> detail`,
the YANG tree needs a `name` container under `interface`, then `detail`
under that selector. Filter forms like `show interface type <type>` need a
`type` container that consumes the typed selector value.

YANG schemas describe command structure and semantics, not CLI presentation.
`--flag` syntax MUST NOT appear anywhere in a `.yang` file: not in a
`description`, not in a `//` comment, not in examples.

A **filter** (address family, row limit, VRF, table, ...) is grammar, so it is a
YANG keyword selector, never a flag:

- MUST model it as a container or leaf the command consumes as `keyword value`
  (`... arp family ipv6`, `... route limit 50`). It is then visible to
  completion and RPC dispatch, which are built from the YANG tree.
- A `description` states what the leaf MEANS. It MUST NOT prescribe a CLI spelling
  ("Filter by address family", not "Filter with `--family`"; not "as a positional
  argument" either).

The `--flag` form (`--json`, `--socket`, `--limit`) is a presentation artifact of
the offline `cmd/ze/` Go flag tooling (`flag.NewFlagSet`) and belongs ONLY there,
never in the YANG layer. A `--flag` baked into a YANG description is documentation
lying about structure: it is invisible to completion and dispatch, and it couples
the shared model to one front-end.

Rationale and the vendor namespacing logic behind family-as-filter (Cisco `ip`
vs Nokia `router` vs Juniper `show route`):
`docs/architecture/cli/command-namespacing.md`.

- **A `--flag` belongs to the PROCESS that runs a command; a bare keyword belongs to the COMMAND itself.** The daemon states the cut in the error it returns when a flag reaches it: `flags are interpreted by the client, not the daemon` (`firstFlagToken`, `internal/component/plugin/server/command.go`).
- **A filter names part of the question, so it MUST take the first register and never the third.** `family`, `limit`, `vrf` and `table` are keywords: `show bgp neighbor ipv6`, never `--family ipv6`.

| Test, applied in order, first yes wins | Register | Form |
|----------------------------------------|----------|------|
| Does it change WHICH answer is produced: the object, the sub-section, the selector, the filter, the variant? | Command grammar | bare keyword, declared in YANG or a `CommandDecl` |
| Does it change how an answer already in hand is rendered or reduced? | Pipe operator | `\| json`, `\| count`, `\| match` |
| Does it change how this process starts, before any command exists: which config, which daemon, which credentials, which plugins, which listener, which colors? | Process option | `--flag` |

| A `--flag` MUST NOT be | Why | Use instead |
|------------------------|-----|-------------|
| A command name | A flag that dispatches is a verb in disguise: it enters no tree, so completion, `ze help command` and the grammar gate never see it | a registered root, or a path under a read verb |
| One of a mutually exclusive set | Several booleans of which exactly one is legal is a closed keyword set the type system is not checking | one keyword slot |
| A second spelling of a pipe operator | `--json` and `\| json` are one job under two names, and only one of them composes. A flag MAY set a session default only by lowering into the operator, as `commandWithFormat` does (`internal/component/cli/client/main.go`) | `\| json`, over an answer registered through `registry.MustRegisterLocalData` |
| A filter that exists as grammar elsewhere | The operator learns one concept twice, and the two surfaces then disagree about it | the keyword |
| Silently ignored when unknown | The operator's intent is dropped and the exit code reports it was honored | name the token in the error and exit non-zero |

- **A client MUST NOT build a flag into a command string it sends to the daemon.** `(*Dispatcher).Dispatch` (`internal/component/plugin/server/command.go`) refuses any flag-shaped token before the handler runs, so such a command fails on every invocation while its client half and its daemon-side parser both read as finished code.

- **The `--flag` form MUST stay in the offline `cmd/ze/` tools that reach no daemon: `appliance`, `analyze`, `perf`, `chaos`, `le`, `install`, `provision` and the mock servers.** These are build, lab and analysis tools, not the router's operator language.
- **A RENDERING flag is banned there too, and everywhere else: `--json`, `--yaml`, `--table`, `--text`, `--format` and `--no-header` MUST NOT exist on any command.** Rendering belongs to the pipe layer, so a command that has an answer MUST register it through `registry.MustRegisterLocalData` and let `| json`, `| yaml` and `| table` render it; a command that renders nothing needs no rendering flag either.
- **"This command does not reach the pipe layer" is a defect, never an exemption.** Whether a command reaches it is decided by how the command was registered and not by anything about the command, so the answer is always to register the answer.
- **A command that crosses to the daemon MUST take keywords, on every front end an operator can type it from.** The SSH CLI, the web terminal, `ze cli -c` and the offline `ze <verb>` dispatch all reach the same command, so one spelling MUST serve all four.
- **MUST NOT give any command a flag spelling, with one exception: `--version`, `-V`, `--help` and `-h`, which every Unix program answers.** A person meeting `ze` for the first time types one of the four before any help exists to tell them otherwise, so `ze` answers them.
- **The four are ADDITIVE, never a replacement: `ze version` and `ze help` MUST exist as commands and MUST stay the canonical form.** The command is what the tree declares, what completion offers and what the documentation names; the flag answers identically beside it. This is a Unix convention, not a compatibility shim, so `ai/rules/go-standards.md` "No Backwards Compatibility" does not reach it and nothing else joins the set.

- **Every flag an offline command accepts MUST be declared once through `registry.RegisterCommandFlags` (`internal/component/command/registry/flags.go`).** A flag declared only in `Meta.Subs` prose is invisible to completion, and prose drifts from the parser in both directions: a flag the handler parses and the help never names, and a flag the help names and the handler never reads.

All CLI commands: online (RPC handlers via YANG dispatch) and offline
(`cmd/ze/` subcommand dispatch). No exceptions for "simple" commands or
"obvious" identifier positions.

## CLI Patterns

- MUST send errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- MUST return exit codes; MUST NOT call `os.Exit()` in handlers
- `-` means stdin (read) / stdout (write): MUST read/write a user-supplied path through
  `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), MUST NOT make a raw
  `os` call. `./le dash-stdio check` fails any command that bypasses it
- Repeatable flags MUST use `stringSlice` with `String()` + `Set()`

**Every user-facing command MUST have tab-completion.** No exceptions by default.

**Opt-out:** MUST set `Hidden: true` on a `CommandDecl` to suppress a command from
completion and help. The command still works when typed in full. MUST use this only
for internal/diagnostic commands that operators SHOULD NOT discover through
tab-completion. Hidden is the exception, not the default.

**Runtime vs offline tree:** the runtime completion tree DOES inject plugin
`CommandRegistry` entries after startup (`internal/component/cli/client/inject.go`
`injectPluginCommands`), so plugin commands complete in the live CLI. The static
offline tree (`BuildCommandTree`, used when no daemon is reachable, and
`ze help command`) still sees only YANG-backed commands; a plugin whose commands
MUST complete offline SHOULD ship a `-cmd` YANG module.

## Pipe Completeness

1. The command MUST route its output through `ApplyPipes` or a `ProcessPipes*` wrapper.
2. If the command has a custom display path that bypasses `ApplyPipes` (e.g. `| log`
   rendering directly from in-memory state), that path MUST still honor data-transform
   pipes (`| resolve`, `| origin`) by applying them to the data before rendering.
3. Display-mode pipes (`| log`, `| no-more`) are flags, not data transforms. They
   change HOW output is shown, not WHAT data is shown. Data-transform pipes apply
   regardless of display mode.

What each operator does, and which class it belongs to, is
`docs/features/formatting.md`; the catalog it is generated from is
`internal/component/command/pipe_catalog.go`.

## Error Messages

An error MUST carry these three legs:

1. **What failed** -- the specific operation plus the identifying subject:
   `file:line`, config key, field name, command, NLRI, port, diagnostic code.
   MUST NOT be a bare "operation failed" or "invalid input".
2. **Why -- the evidence** -- the offending value AND the expected one. MUST quote
   values with `%q` so empty, whitespace, or look-alike values are visible:
   `expected exit code %d, got %d`, `unknown field %q (want one of ...)`.
3. **What to do next** -- the corrective action, or a stable handle the reader
   can act on: a directive to add, a flag to set, a native action to run, or a
   registered `doctor-*` diagnostic code that `ze explain` expands.

**When the next step needs more than one line, a diagnostic code MUST be attached rather than the guidance truncated.**

**Leg 3 MUST be TRUE, not merely present.** A remediation that names a command
MUST name one that actually produces the promised effect. A command that looks
plausible but does not do what the message claims is worse than no advice: the
reader trusts it, follows it, and loses the time twice, then stops trusting the
tool's output at all. MUST verify the producer before you print the instruction -- if
the message says "re-run X to refresh Y", MUST read the code that writes Y and confirm
X writes it (a lint target does not rewrite a verify record; only a verify run
does). This is the `doctor-vpp-lcp-netns` class of bug: advice that cannot work.

**The corrective action MUST be carried on a machine-facing surface (doctor, startup, config apply and verify, readiness, plugin load).** An internal error wrapped upward MUST carry the first two legs and a wrapped cause (`%w`), and SHOULD carry the corrective action whenever a clear next step exists. A deep internal error MUST NOT invent one.

**Operator-facing text MUST NOT name the library, driver or vendor package Ze
implements a feature with.** It names what the OPERATOR configured. This covers
every surface a person reads: an error string, a log line, CLI output, the TUI,
and a `ze explain` description.

The operator wrote `traffic { control { backend vpp } }`, so `vpp` is theirs and
it belongs in the message. `govpp` is the Go package Ze happens to speak VPP
with. It appears in no configuration, no documentation an operator is given, and
no search they would think to run. A message naming it asks them to debug a
dependency they did not choose.

**A message MUST NOT name a Go symbol either.** `govpp WaitConnected: not
connected after 5s` names a library AND a method. `vpp not connected after 5s`
says the same thing to the person who has to start VPP.

The test is one question: could the operator have typed this word into their
configuration, or read it in the guide? A backend name, an interface name, a
peer address and a protocol all pass. A package, a type, a function and a
vendored module do not.

The rule stops at the boundary. A wrapped error travelling between packages MAY
name whatever helps a developer, and an identifier in the code is not
operator-facing text. What crosses to a person is what this governs, and the
last wrap before that crossing is where the internal name gets removed.

**A row's state MUST be a FIELD or a COLUMN. It MUST NOT be a character glued to another
field's value.** No `*`, no `>`, no `+`, no leading dot. If a row is different,
say so in a place a reader and a pipe can both find.

**The fix is always the same shape.** MUST add the boolean or enum to the snapshot
type, render it as its own column, and give the column and the JSON field the
same name so a reader moving between forms learns one vocabulary. The value
column MUST keep the value and nothing else.

**MUST test the value, not only the marker.** A test that asserts the state appears
does not catch a sigil that also pollutes the identifier. MUST assert the identifier
field is EXACTLY the identifier. Without that assertion nothing stops the
asterisk returning the next time somebody reads another vendor's output as a
model.

**A check, assertion, validation or translation that cannot be evaluated MUST return an error saying so, and MUST NOT return success, `nil`, or skip.** A silent skip is a false pass and a silent data loss, and it removes the corrective signal entirely. See `ai/rules/protocol.md`.

**A user-facing or runtime failure (doctor, startup, config apply, readiness, plugin load) MUST carry a registered code in `internal/core/diagnostic/codes.go`, and the handler MUST return that code with structured fields rather than a pre-formatted sentence** (`ai/rules/evidence.md`). What each code holds, and how it reaches an operator, is `docs/architecture/cli/error-surface.md`.

MUST ask these questions before returning an error:

1. Does it name the specific subject (path/key/field/value), not just the operation?
2. Could a reader who has never seen this code take the next step from this line alone?
3. If the next step needs more than one line, is there a diagnostic code carrying it?
4. Is the leading phrase stable and greppable, or did I reword a shared failure?

## JSON Format

**Every JSON key MUST be lowercase kebab-case, and MUST NOT be camelCase or snake_case.** The key set, the address families, the envelope shape and the one exemption are `docs/architecture/api/json-format.md`.

**Deriving the name:** the JSON key MUST match the YANG leaf name or config tree key.
A config key `remove-private-as` becomes `json:"remove-private-as"`, MUST NOT be
`remove-private` or `remove_private_as`. When no YANG leaf exists,
MUST use the same kebab-case convention: lowercase words separated by hyphens.

**Go struct tags:** `json:"kebab-name"` or `json:"kebab-name,omitempty"`. The tag
is the contract. Go field names are PascalCase (Go convention), JSON tags are
kebab-case (Ze convention). MUST NOT let Go's default JSON marshaling (which uses the
Go field name) leak into output.

Envelopes and values MUST follow these conventions:

- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x`. `"parsed":false` + `"raw":"DEADBEEF"`
- Numbers: JSON `float64` in Go -> use `formatNumber()` for integer display

## Agent Tooling Contract

**Every new command that produces JSON for an agent MUST meet the obligations below.**

1. MUST use `encoding/json`; MUST NOT use string concatenation.
2. MUST use lower kebab-case keys (see "Field Naming: kebab-case" above).
3. MUST include `schema-version` in top-level envelopes via `diagnostic.NewValidateResult` or `diagnostic.NewFixPlan`.
4. MUST NOT emit ANSI escape sequences when stdout is not a terminal.

**Every validation error surfaced to an agent MUST carry a stable diagnostic code.**

1. Codes are lower-kebab: `config-parse`, `config-yang-type`, etc.
2. Every code MUST be registered in `internal/core/diagnostic/codes.go` with title, description, and related codes.
3. New validation stages MUST map errors to diagnostic codes, not pass raw strings.
4. The `ze explain` command MUST return an explanation for every registered code.

**Repair metadata is a plan only: a command MUST NOT edit a config file.**

1. Every repair MUST carry a stable `id` (lower-kebab) and `summary`.
2. Safety labels: `format-only`, `section-local`, `behavior-preserving`, `api-changing`, `target-changing`, `requires-human-review`.
3. If Ze cannot prove a repair is safe, MUST use `requires-human-review` with id `manual-review`.

**When a skill covers the task (`/ze-rfc`, `/ze-review`, `/ze-implement`, and the rest), it MUST be used instead of spawning a raw agent or improvising the workflow.** A skill encodes the conventions, gates and ordering a raw agent misses. Skill content is embedded in the binary, so it matches the version in hand.

- **The native `pretool-agent-skill` action in `internal/le/hookruntime/agent.go` BLOCKS the spawn** when the prompt asks for something a skill covers. It matches the ask, never the subject.
- **Naming the skill in the prompt satisfies the gate**, so a subagent that MUST follow `/ze-explore` is spawned by saying so.
- The map it enforces: research is `/ze-explore`, review is `/ze-review`, spec conformance is `/ze-review-spec`, a red test is `/ze-debug`, spec work is `/ze-implement`, bug classes are `/ze-hunt`, spec audit is `/ze-audit`.
- A hand-written prompt reproduces a worse version of the skill and drops every gate it carries. That is what the gate exists to stop.

**A commit script MUST be prepared with `./le commit create`, and the path its `script=` line prints MUST be the one that is run.** `internal/le/commit` owns session id reuse, message creation, explicit add and remove validation, script generation and the pre-staging gates. A hand-written compatibility path MUST NOT be used.

**On an explicit commit request, preparing the commit IS the work: a late completeness check, health check, recent-commit style review or remaining-work table MUST NOT be run unless the user asks for one.** `./le verify status check` MUST be run before any verify target, and a FRESH result MUST NOT be followed by a rerun of `./le verify worktree`.

1. New skills MUST go in `internal/plugins/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `internal/plugins/skills/main.go` MUST list every embedded file.
3. `ze skills list` MUST show all bundled skills without a static list elsewhere.

**Config validation MUST run at every boundary where config enters the system, and every boundary MUST use the same diagnostic pipeline.**

Every one of these boundaries MUST validate config:

1. `ze config validate` (CLI).
2. `ze config fix --plan` (CLI agent surface).
3. Web commit (pre-commit validation of pending changes).
4. Hub API config push (`ValidateContent`).
5. `ze config validate --pending` (validate zefs pending config without committing).

1. MUST update `docs/features/ai-first.md` with the new command or contract.
2. MUST update `docs/guide/mcp/overview.md` if MCP users need to discover the feature.
3. MUST update embedded skill files if the feature changes the agent workflow.
4. MUST satisfy `ai/rules/repo-maintenance.md`: add the keyword/task path in
   `ai/INDEX.md`, and document the verification target
   that proves the feature is discoverable.
