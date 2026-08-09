# CLI and Output

**When:** adding or changing any CLI command, flag, exit code, output format, error message, JSON envelope, or agent-facing contract
**Severity:** blocking
**Related:** evidence, performance, protocol, repo-maintenance, git-safety

## Directives

- **Every CLI command must place a closed keyword before any user-supplied value.** This eliminates ambiguity where a free-form value could collide with a keyword.
- **All CLI commands MUST follow the patterns in "CLI Patterns" below.** Structural template: `ai/patterns/cli-command.md`. Rationale: `ai/rationale/cli-patterns.md`.
- **Every command that produces output MUST support all pipe operators.**
- **A row's state is a field or a column, never a character glued to a value.** No `*`, `>`, `+` or leading dot on an identifier. A sigil corrupts the value for `| grep` and has nowhere to live in `| json`, so the text and JSON forms stop agreeing on what the value is. `*` is also already an input token here, the selector wildcard. See "A value carries no marker" below.
- **Every error, log line, and failure output you write must let a human or an agent see what failed, why, and what to do next, without opening the source.** The error is the corrective signal: if it does not point at the fix, the reader cannot act and an agent cannot self-correct.
- **All JSON output MUST follow the conventions in "JSON Format" below.** Rationale: `ai/rationale/json-format.md`.
- **All agent-facing CLI output must follow the rules in "Agent Tooling Contract" below.**

## CLI Grammar: Keywords Before Values

### The Rule

```
<verb> <noun> <action> [<args>]
<verb> <noun> <selector-kind> <selector-value> <action> [<args>]
```

The first token after the noun (component/resource) MUST be a keyword from a
closed set known at compile time. If a command targets one member of a set,
the selector itself MUST also be typed by a keyword such as `name`, `id`,
`index`, `address`, or `type`. Free-form values MUST NOT appear in an untyped
positional slot.

### Correct vs Incorrect

| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show interface <name>` | `show interface name <name> detail` | `<name>` is untyped and could collide with keywords (`brief`, `errors`) |
| `show interface <name> counters` | `show interface name <name> counters` | Selector value appears before selector kind |
| `show l2tp session <id>` | `show l2tp session id <id> detail` | ID must be typed before use |
| `show vpn ipsec peer <name>` | `show vpn ipsec peer name <name> detail` | Named lookup needs an explicit selector kind |
| `cache <id> retain` | `cache retain <id>` | ID before action |
| `commit <name> start` | `commit start <name>` | Name before action |

### Typed Selectors

Use an explicit selector kind whenever a command addresses one member of a set.

- `name <name>` for operator-assigned names
- `id <id>` for string or numeric IDs
- `index <index>` for positional or kernel indexes
- `address <ip>` for IP-address lookup
- `type <type>` for category filters
- `key <key>` for generic key
- another schema-defined closed keyword when the domain needs something more specific

Examples:

- `show interface type dummy`
- `show interface name eth0 detail`
- `show interface name eth0 counters`
- `show sysctl key net.ipv4.ip_forward`
- `show sysctl profile router`

#### Peer Commands

Peer commands are the explicit exception to the generic typed-selector
keyword form.

Use:

- `show bgp peer <name|address> detail`
- `show bgp peer <name|address> rib`

Do not invent mutating peer examples here unless the exact grammar was
explicitly agreed in source or by the user.

Do not invent a user-facing `selector` keyword. `selector` may exist as an
internal concept in dispatcher code, but it MUST NOT leak into operator
syntax.

### Named-Resource Commands

The "named-resource" pattern `<resource> <id> <action>` violates this rule.
Correct form: `<resource> <action> <id>`.

- `cache retain <id>`, not `cache <id> retain`
- `commit start <name>`, not `commit <name> start`

The `list` action (no identifier) already works correctly in both.

### Compound Token vs Namespace Split (R9)

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
   `max-prefix`, `file-descriptors`). If yes, keep the hyphen.
3. A shared prefix is not proof of a namespace. `flow-export` (NetFlow/IPFIX) and
   `flow-recent` (conntrack ring) share "flow" by accident; they are not
   `show flow {export,recent}`. Split only when the prefix is a real object that owns
   every child.
4. A split namespace needs one owning module. If several components share the prefix,
   one module owns the container and the others augment it (as `trafficusage` augments
   `traffic`). Never a shared parent that multiple plugins reach up into: that is the
   plugin-self-containment break the old `show ip` grouping caused.

| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show traffic-stat` | `show traffic stat` | `traffic` is a real namespace (traffic-cmd owns it, trafficusage augments it); `stat` is a member |
| `show bgp-health` | `show bgp health` | `bgp` is the object namespace |
| `show metrics-query` | `show metrics query` | `metrics` is a real namespace |
| `show l2tp session-history` | `show l2tp session history` | `session` is a real container under `l2tp` |
| `resolve peeringdb as-set` | `resolve peeringdb as-set` (unchanged) | `as-set` is one IRR object; no `as` sibling; keep the hyphen |

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
vs the `traffic` container). Do not assume a root is ungoverned because it is not in the
tree.
Shipped commands awaiting the agreed rename are listed in `pendingNamespaceSplit`
(`scripts/checks/cli_grammar.go`) and reported as tracked debt, so the gate stays green
while a NEW collision still fails. Migrating one is a dispatch-key change (see
"Migrating a Built-in Command's Path" below): add the split path, keep the old form per
"Backward Compatibility", update `.ci` senders, and remove its `pendingNamespaceSplit`
entry.

**When the compound is genuine, exempt it -- do not split it.** `CheckSiblings` fires on
the mere existence of a sibling matching the LEFT segment, so it cannot tell a real
namespace from two names that share a word by accident (test 3). When test 2 wins -- the
token is one indivisible protocol / LSA / object name -- list the full command path in
`treeNamespaceExempt` (`scripts/checks/cli_grammar.go`) with a one-line reason. It is the
tree-side counterpart of `rootNamespaceExempt`, is counted and printed (`Tree
namespace-exempt`), and leaves every unlisted collision blocking. Reach for it only when
splitting would state something false about the object model; `show ospf database
router-information` is the worked case (RFC 7770's RI LSA is an *Opaque* LSA, so filing
it under the `router` sibling -- the Type 1 Router-LSA -- would be wrong). Note the check
only ever looks left, so the same shared-word situation on the right (`summary` /
`asbr-summary`, `external` / `nssa-external`) is never flagged at all.

### Choosing the Verb: Read vs Perturb (`show`/`monitor` vs `debug`)

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

### Engine-Owned Tree Mutation

Do not invent operational `add`, `del`, `remove`, or similar verbs for
objects that already live in the config YANG tree.

Config mutation belongs to the engine verbs:

- `set <path> <value>`
- `delete <path>`

If a change means "create/delete/change a node in the YANG config tree",
the command surface MUST stay in engine path form. Do not mirror that
operation as an RPC command just to make the grammar look regular.

Examples:

- Interface addresses and units are config-tree mutations. They belong under
  engine `set` / `delete`, not operational commands like `interface addr del`
  or `interface unit remove`.
- Runtime actions that are not tree mutation, such as `clear counters`,
  `teardown`, or `pause`, may have operational command verbs.

Before changing any command grammar, classify it first:

1. **Config tree mutation** -> engine `set` / `delete`
2. **Runtime operational action** -> CLI/RPC command grammar applies
3. **Read/query** -> `show ...`

If the command is class (1), stop. Do not redesign it as a verb-first RPC.

### YANG Module Ownership

Do not move a command family into a different YANG module while fixing
grammar unless the architecture explicitly requires that move.

First identify the owning module from source:

- the existing `ze:command` path
- the module `register.go` / `embed.go`
- the handler `RPCRegistration`

Then edit that module. Grammar cleanup is not permission to reshuffle
command ownership across components.

### Migrating a Built-in Command's Path (dispatch key = YANG path)

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
   / `DispatchCommandArgs` calls, `printf ... | ze ... plugin cli`, `SendCommand`. Update
   every one in the same change. (In-tree these are greppable; external plugins/scripts
   are not, so a hard cut with no deprecation carries that residual risk.)
2. The **programmatic plugin SDK is not affected**: it uses structured RPC wire methods
   (`ze-plugin-callback:*`, `pkg/plugin/sdk/sdk_dispatch.go`), not command-path strings.
   Only the interactive plugin CLI and `dispatch-command` carry command-path strings, and
   in-tree those are already verb-first (e.g. `bmp.go` sends `show bgp rib protocol`).
3. Some noun-first forms are a deliberate **namespace**, not un-migrated legacy. Keep
   them: `plugin encoding/format/ack` group plugin-session directives under `plugin`, and
   `set plugin` would collide with the config-tree `plugin` node (see Engine-Owned Tree
   Mutation). `command list/help/complete` (engine introspection) migrated cleanly to
   `show command ...`; `plugin ...` stayed.

When you migrate, also drop the command's verb from `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) if it was only there as a legacy
noun-first form.

### Identifiers Are Strings

Use string-typed identifiers even when the conventional representation is numeric.
Cache IDs, VLAN IDs, session IDs: accept and store as strings. Parse to numeric
only at the point of use if the underlying API requires it. This avoids:
- Grammar ambiguity between numeric keywords and numeric IDs
- Unnecessary coupling to a representation (IDs may become non-numeric later)
- Parsing errors surfacing at the wrong layer

### Backward Compatibility

If the wrong grammar has not shipped, replace it outright. Do not add
deprecation branches for unreleased syntax.

If fixing a released grammar, accept the old grammar with a deprecation warning.
Log the warning once per session. Remove old grammar after two release cycles.

### Mechanical Check (grammar)

For every handler that dispatches on `args[0]`:

1. Is `args[0]` always a keyword from a known set? -> Correct.
2. If the command selects one member of a set, does the handler consume a
   selector-kind keyword before the free-form value? -> Correct.
3. Can any positional argument before the action or selector-kind be a
   user-supplied value? -> Violation.

```
grep -n 'args\[0\]' <handler-file> | grep -v 'case\|==.*"'
```

If any `args[0]` usage passes the value to a lookup/parse function (GetInterface,
ParseUint, etc.) without first matching it against a keyword set, that is a violation.

### Mechanical Enforcement (automated)

These rules are enforced automatically, not just by review. The reverse-engineered
ruleset is R1-R9 (verb-first, token form, no `--flag`, namespace discipline,
keyword-before-value, action-before-identifier, config-tree-mutation stays in
`set`/`delete`, string identifiers, compound-vs-namespace split), implemented once in
`internal/component/command/grammar` and read from the canonical verb registry
`internal/component/command` (`Verbs`). Five feeders enforce it:

| Feeder | What it checks | Run |
|--------|----------------|-----|
| Static gate | Every built-in command (YANG command tree) against R1-R9 (R9 sibling-collision is static-gate-only, it needs sibling context), plus no `--flag` in any `.yang` | `make ze-cli-grammar-check` (NOT a `make ze-verify` stage -- it is not in `stagesForMode`; the gate reaches CI through `TestCLIGrammarGateStatic` in `scripts/checks/cli_grammar_test.go`, which runs the same checker under the unit stage) |
| Registration | Every plugin `CommandDecl` at registration (`validateCommandName`) | plugin startup in functional/exabgp suites |
| Runtime guard | The runtime built-in assembly (`AllBuiltinRPCs` x `WireMethodToPaths`) re-checked with `ExemptCategory` by wire method; and the `CommandRegistry.Register` boundary rejecting a bad name | `TestRuntimeBuiltinSurfaceGrammar` / `TestRegistrationRejectsBadGrammar` (unit) |
| Root namespace | Every registered root command (`registry.MustRegisterRootHandler` / `RegisterRoot`, enumerated from source) against R9 across surfaces (`grammar.CheckRootNamespace`): a hyphenated root whose left segment names a YANG verb or container is a namespace member masquerading as a compound root. Root handlers never pass through the YANG-tree static gate, so this feeder is the only one that governs them | `make ze-cli-grammar-check` (same gate); `TestRootNamespaceGrammar` (unit) |
| Demo call sites | Every `ze <token>` invocation in `demos/terminal/**/*.sh`: the position-1 token must be a YANG verb, a registered root, or the `-` stdin sentinel. The other feeders check how commands are DECLARED; this one checks the repo's own CALL SITES, which no other gate reaches -- `make ze-verify` never executes the demos (Docker + VHS, run from `mk/terminal-demo.mk` at release time and by the gh-pages website workflow), so a removed launch form rots there silently -- and since main's own `pages.yml` was deleted, main no longer gets even the after-the-fact signal it once did: `ze <config-file>` stayed in thirteen demo scripts and failed the Deploy website job on every push for four days. This static gate is now the ONLY thing on main that sees a broken demo call site | `make ze-cli-grammar-check` (same gate); `TestCLIGrammarGateStatic` (unit) |

Feeder 3 is an **in-process** guard, not a daemon-boot audit: built-ins are 100%
YANG-derived (a handler with no YANG path is skipped, `LoadBuiltinsWithAliases`) so
they are a strict subset of the static gate's tree, and plugin commands are rejected
at registration by Feeder 2. The merged `system command list` surface therefore
contains only conforming commands by construction: a boot-and-dump audit would add
no catch value while depending on an all-plugins config that cannot exist (startup is
config-path-gated). The guard instead locks the two runtime sources against
regression cheaply and deterministically.

To add a verb, edit `command.Verbs` (one place; the plugin gate and the static gate
both derive from it). Category exemptions (the text bridge, `ze-plugin:`/`ze-system:`
wire-protocol directives, and `ze-editor:` modes) live in `grammar.ExemptCategory`,
keyed on the handler wire-method namespace, never a per-command allowlist.

### YANG Tree

The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface name <name> detail`,
the YANG tree needs a `name` container under `interface`, then `detail`
under that selector. Filter forms like `show interface type <type>` need a
`type` container that consumes the typed selector value.

### No Flag Syntax in YANG (Filters Are Keyword Grammar)

YANG schemas describe command structure and semantics, not CLI presentation.
`--flag` syntax MUST NOT appear anywhere in a `.yang` file: not in a
`description`, not in a `//` comment, not in examples.

A **filter** (address family, row limit, VRF, table, ...) is grammar, so it is a
YANG keyword selector, never a flag:

- Model it as a container or leaf the command consumes as `keyword value`
  (`... arp family ipv6`, `... route limit 50`). It is then visible to
  completion and RPC dispatch, which are built from the YANG tree.
- A `description` states what the leaf MEANS. It never prescribes a CLI spelling
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

Mechanical check (must return nothing):

```
grep -rnE '\-\-[a-z]' internal --include='*.yang' | grep -vE 'urn:|http|xml'
```

### Applies To

All CLI commands: online (RPC handlers via YANG dispatch) and offline
(`cmd/ze/` subcommand dispatch). No exceptions for "simple" commands or
"obvious" identifier positions.

## CLI Patterns

### Dispatch

Each domain: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`.
Handle `help`/`-h`/`--help` first, then dispatch.

### Flags

Each subcommand: own `flag.NewFlagSet` with custom `fs.Usage`. Parse flags, check required positional args, return exit codes.

#### Short Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `-v` | Verbose | `-q` | Quiet |
| `-o` | Output file | `-f` | Family/file |
| `-i` | Enable feature | `-a` | Local AS |
| `-z` | Peer AS | `-n` | Dry run/count |

#### Long Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `--json` | JSON output | `--text` | Human-readable |
| `--dry-run` | Preview | `--socket` | Unix socket path |
| `--log-level` | Logging level | `--no-header` | Exclude headers |

### Exit Codes

0 = success, 1 = general/validation/usage error, 2 = file not found/unreadable.

### Rules

- Errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Return exit codes, never `os.Exit()` in handlers
- `-` means stdin (read) / stdout (write): read/write a user-supplied path through
  `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), never a raw
  `os` call. `make ze-dash-stdio-check` fails any command that bypasses it. `--json` for JSON output
- Repeatable flags: `stringSlice` with `String()` + `Set()`

### Command Completion (BLOCKING)

**Every user-facing command MUST have tab-completion.** No exceptions by default.

The completion tree is built from two sources:
1. **YANG command schemas** for built-in commands (via `BuildCommandTree`)
2. **Plugin command registry** for SDK plugin commands (via `CommandRegistry`)

Both feed the same completion tree. A plugin that registers a `CommandDecl` gets
completion automatically without writing a YANG file.

**Opt-out:** Set `Hidden: true` on a `CommandDecl` to suppress a command from
completion and help. The command still works when typed in full. Use this only
for internal/diagnostic commands that operators should not discover through
tab-completion. Hidden is the exception, not the default.

**Runtime vs offline tree:** the runtime completion tree DOES inject plugin
`CommandRegistry` entries after startup (`internal/component/cli/client/inject.go`
`injectPluginCommands`), so plugin commands complete in the live CLI. The static
offline tree (`BuildCommandTree`, used when no daemon is reachable, and
`ze help command`) still sees only YANG-backed commands; a plugin whose commands
must complete offline should ship a `-cmd` YANG module.

### New Command Checklist

```
[ ] Grammar: action keyword before identifier (see "CLI Grammar: Keywords Before Values")
[ ] Handler: cmd<Name>(args []string) int
[ ] flag.NewFlagSet with fs.Usage including examples
[ ] Handle --help/-h at parent level
[ ] Check required positional args
[ ] Errors to stderr, proper exit codes
[ ] Register in parent dispatch
[ ] Tab-completion works (verify with tab in CLI)
[ ] Colors follow semantic roles (docs/architecture/cli/color-system.md)
[ ] Functional tests
```

## Pipe Completeness

### The Pipe Operators

| Pipe | Purpose | Operates on |
|------|---------|-------------|
| `\| json` | Raw JSON output | JSON string |
| `\| ndjson` | Newline-delimited JSON | JSON string |
| `\| table` | Tabular display (default) | JSON string |
| `\| text` | Plain text | JSON string |
| `\| yaml` | YAML output | JSON string |
| `\| match <pat>` | Grep output lines | formatted string |
| `\| count` | Count results | JSON string |
| `\| resolve` | Add reverse DNS for IPs | JSON (walks values) |
| `\| origin` | Add ASN/network for IPs | JSON (walks values) |
| `\| log` | Streaming log mode | display mode flag |
| `\| no-more` | Paging | display mode flag |

### The Rule (pipes)

When adding a new command or a new display mode (like `| log`):

1. The command MUST route its output through `ApplyPipes` or a `ProcessPipes*` wrapper.
2. If the command has a custom display path that bypasses `ApplyPipes` (e.g. `| log`
   rendering directly from in-memory state), that path MUST still honor data-transform
   pipes (`| resolve`, `| origin`) by applying them to the data before rendering.
3. Display-mode pipes (`| log`, `| no-more`) are flags, not data transforms. They
   change HOW output is shown, not WHAT data is shown. Data-transform pipes apply
   regardless of display mode.

### Mechanical Check (pipes)

For every new command or display mode:

```
grep -n 'ApplyPipes\|ProcessPipes\|formatFn' <new-file>
```

If the command has a rendering path that does NOT call `ApplyPipes`/`formatFn`,
verify that `| resolve` and `| origin` are still applied in that path.

### Known Violations (to fix)

| Command | Mode | Missing pipes | Where |
|---------|------|---------------|-------|
| _(none currently)_ | | | |

Both `monitor traceroute | log` and `monitor ping | log` bypass `ApplyPipes`
and render directly from hop/ping stats; they now apply `resolve`/`origin`
to their legend addresses via the shared `enrichAddr` helper
(`internal/component/cli/model_enrich.go`). Functional coverage:
`test/ui/monitor-ping-pipe-resolve-log.ci` drives the headless TUI with
`option=monitor:ping=fake` (deterministic ping factory + PTR/origin fakes in
`internal/component/cli/testing/fake_monitor.go`).

## Error Messages

### The contract: what / why / next

An error must answer three questions:

1. **What failed** -- the specific operation plus the identifying subject:
   `file:line`, config key, field name, command, NLRI, port, diagnostic code.
   Never a bare "operation failed" or "invalid input".
2. **Why -- the evidence** -- the offending value AND the expected one. Quote
   values with `%q` so empty, whitespace, or look-alike values are visible:
   `expected exit code %d, got %d`, `unknown field %q (want one of ...)`.
3. **What to do next** -- the corrective action, or a stable handle the reader
   can act on: a directive to add, a flag to set, a make target to run, or a
   registered `doctor-*` diagnostic code that `ze explain` expands.

If the next step needs more than one line, attach a diagnostic code (below)
rather than truncating the guidance.

**Leg 3 must be TRUE, not merely present.** A remediation that names a command
must name one that actually produces the promised effect. A command that looks
plausible but does not do what the message claims is worse than no advice: the
reader trusts it, follows it, and loses the time twice, then stops trusting the
tool's output at all. Verify the producer before you print the instruction -- if
the message says "re-run X to refresh Y", read the code that writes Y and confirm
X writes it (a lint target does not rewrite a verify record; only a verify run
does). This is the `doctor-vpp-lcp-netns` class of bug: advice that cannot work.

Scope of leg 3: it is mandatory on machine-facing surfaces (doctor, startup,
config apply/verify, readiness, plugin load -- the diagnostic-code surfaces
below). For internal errors that get wrapped upward, legs 1 and 2 plus a
wrapped cause (`%w`) are the requirement; add the corrective action whenever
a clear next step exists, but a deep internal error need not invent one.

### Format: humans scan, agents parse

| Rule | Why |
|------|-----|
| Lowercase start, no trailing punctuation, single line | Go convention; errors get wrapped, joined, and grepped |
| One **stable leading phrase** per failure kind (e.g. `reject=syslog pattern found:`) | Agents and log scanners match on it; do not reword per call site |
| Wrap the cause and add context: `fmt.Errorf("parse %s: %w", path, err)` | Preserves `errors.Is/errors.As` chains; each layer adds what it knows |
| Name the subject and the value, not just the type | "invalid value" with no value is unactionable |
| Truncate large blobs (bodies, dumps, hex) before embedding | A 10 MB error is unreadable for both humans and agents |
| No `fmt.Sprintf`/`fmt.Errorf` on hot paths -- see `ai/rules/performance.md` | Boundary and one-shot errors may use `fmt.Errorf`; hot paths use append builders |

### A value carries no marker: state is a field, never a sigil

**A row's state is a FIELD or a COLUMN. It is never a character glued to another
field's value.** No `*`, no `>`, no `+`, no leading dot. If a row is different,
say so in a place a reader and a pipe can both find.

Other implementations do decorate. FRR and Extreme both print the local system's
IS-IS LSP as `rtr.00-00 *`. Copying that here breaks two things at once.

| What breaks | Why |
|-------------|-----|
| The value | `\| grep <lsp-id>` stops matching, and a parsed field carries a character that is not part of the identifier. The text form and the JSON form then disagree about what the identifier IS |
| The token | `*` is already an INPUT token in Ze: the selector wildcard for "all" (`peer *`, `clear bgp rib in *`, `192.168.*.*`), documented in `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md` and `docs/guide/route-injection.md`. One character pointing in two directions |

A marker that exists only in the text rendering is not information, it is
decoration: `| json` has nowhere to put it, so the two forms carry different
facts. Every command here composes with the pipe operators, which is why the
sigil is a Ze-specific defect rather than a style preference.

**The fix is always the same shape.** Add the boolean or enum to the snapshot
type, render it as its own column, and give the column and the JSON field the
same name so a reader moving between forms learns one vocabulary. The value
column keeps the value and nothing else.

**Test the value, not only the marker.** A test that asserts the state appears
does not catch a sigil that also pollutes the identifier. Assert the identifier
field is EXACTLY the identifier. Without that assertion nothing stops the
asterisk returning the next time somebody reads another vendor's output as a
model.

### Fail closed, never vacuously

When a check, assertion, validation, or translation cannot be evaluated, return
an error that says so. Never return success, `nil`, or skip. A silent skip is a
false pass and a silent data loss, and it removes the corrective signal entirely.
An audit of the `.ci` runner found four of these at once: an assertion that was
parsed but never read in the decision path, an early `return true` that skipped
the later assertions, an empty pattern that matched everything, and a content
matcher that skipped a family it could not extract. See `ai/rules/protocol.md`.

### Machine-facing failures: carry a diagnostic code

User-facing and runtime failures (doctor, startup, config apply, readiness, plugin
load) must carry a registered code in `internal/core/diagnostic/codes.go` with
title, description, examples, and remediation, explainable via
`ze explain <code>`. Return the code plus structured fields, not a pre-formatted
sentence -- see `ai/rules/evidence.md`. The diagnostic code is what
makes the corrective action machine-readable for an agent.

### Mechanical check (errors)

Before returning or logging an error, ask:

1. Does it name the specific subject (path/key/field/value), not just the operation?
2. Could a reader who has never seen this code take the next step from this line alone?
3. If the next step needs more than one line, is there a diagnostic code carrying it?
4. Is the leading phrase stable and greppable, or did I reword a shared failure?

Any "no" -- add the subject, the value, the corrective action, or the code.

### Banned

| Pattern | Fix |
|---------|-----|
| `errors.New("failed")`, `"invalid input"`, `"unexpected error"` | Name what, the value, and the expected |
| Dropping the cause inside `if err != nil` (`return errors.New("parse failed")`) | Wrap: `fmt.Errorf("parse %s: %w", name, err)` |
| Reporting a value as invalid without printing it | Include `%q` of the offending value |
| Rewording a stable error phrase per call site | Keep one phrase so it stays greppable |
| Returning `nil`/skip when a check cannot run | Return an error; fail closed |
| A user-facing failure with no diagnostic code or remediation | Register a `doctor-*` code, make it `ze explain`-able |

## JSON Format

### Field Naming: kebab-case (MANDATORY)

All JSON keys: lowercase kebab-case. Never camelCase or snake_case.

**Deriving the name:** the JSON key must match the YANG leaf name or config tree key.
A config key `remove-private-as` becomes `json:"remove-private-as"`, never
`remove-private` or `remove_private_as`. When no YANG leaf exists,
use the same kebab-case convention: lowercase words separated by hyphens.

**Go struct tags:** `json:"kebab-name"` or `json:"kebab-name,omitempty"`. The tag
is the contract. Go field names are PascalCase (Go convention), JSON tags are
kebab-case (Ze convention). Never let Go's default JSON marshaling (which uses the
Go field name) leak into output.

| Wrong | Right | Why |
|-------|-------|-----|
| `json:"remove-private"` | `json:"remove-private-as"` | Truncated; must match the full config key |
| `json:"policyAttrASPathRemovePrivate"` | `json:"remove-private-as"` | camelCase is not kebab-case |
| No `json` tag on exported field | `json:"kebab-name"` | Go default leaks PascalCase |

**Exception:** `internal/component/lg/handler_api.go` uses birdwatcher-convention `snake_case` for external compatibility with Alice-LG and other looking glass frontends. This is the only file exempt from kebab-case.

### Ze IPC Envelope

```json
{"type":"bgp","bgp":{"peer":{"address":"...","group":"...","name":"...","remote":{"as":N}},"message":{"id":N,"direction":"received","type":"open"},"open":{...}}}
```

### Attribute Names

| Attribute | Key | Type |
|-----------|-----|------|
| ORIGIN | `"origin"` | `"igp"` / `"egp"` / `"incomplete"` |
| AS_PATH | `"as-path"` | array of integers |
| NEXT_HOP | `"next-hop"` | IP string |
| MED | `"med"` | integer |
| LOCAL_PREF | `"local-preference"` | integer |
| ATOMIC_AGGREGATE | `"atomic-aggregate"` | boolean |
| AGGREGATOR | `"aggregator"` | `"asn:ip"` |
| ORIGINATOR_ID | `"originator-id"` | IP string |
| CLUSTER_LIST | `"cluster-list"` | array of strings |
| COMMUNITIES | `"community"` | array of strings |
| EXT_COMMUNITIES | `"extended-community"` | array of objects |

### Address Families

Format: `"afi/safi"`. Families are registered dynamically by plugins (not a static list). Current inventory:

| AFI | Families |
|-----|----------|
| ipv4 | `unicast`, `multicast`, `vpn`, `flow`, `flow-vpn`, `mpls-label`, `mup`, `mvpn`, `rtc` |
| ipv6 | `unicast`, `multicast`, `vpn`, `flow`, `flow-vpn`, `mpls-label`, `mup`, `mvpn` |
| l2vpn | `evpn`, `vpls` |
| bgp-ls | `bgp-ls`, `bgp-ls-vpn` |

Unicast and multicast are builtin (engine). All others registered by `bgp-nlri-*` plugins. Use `make ze-inventory` for the authoritative list.

### NLRI Operations

```json
{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}
{"action":"del","nlri":["10.0.2.0/24"]}
```

### Conventions

- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x`. `"parsed":false` + `"raw":"DEADBEEF"`
- Numbers: JSON `float64` in Go -> use `formatNumber()` for integer display

## Agent Tooling Contract

### JSON Output Contract

Every new command that produces JSON output for agents must:

1. Use `encoding/json`, never string concatenation.
2. Use lower kebab-case keys (see "Field Naming: kebab-case" above).
3. Include `schema-version` in top-level envelopes via `diagnostic.NewValidateResult` or `diagnostic.NewFixPlan`.
4. Emit no ANSI escape sequences when stdout is not a terminal.

### Diagnostic Codes

Every validation error surfaced to agents must carry a stable diagnostic code.

1. Codes are lower-kebab: `config-parse`, `config-yang-type`, etc.
2. Every code must be registered in `internal/core/diagnostic/codes.go` with title, description, and related codes.
3. New validation stages must map errors to diagnostic codes, not pass raw strings.
4. The `ze explain` command must return an explanation for every registered code.

### Repair Plans

Repair metadata is plan-only. Commands must never edit config files.

1. Every repair carries a stable `id` (lower-kebab) and `summary`.
2. Safety labels: `format-only`, `section-local`, `behavior-preserving`, `api-changing`, `target-changing`, `requires-human-review`.
3. If Ze cannot prove a repair is safe, use `requires-human-review` with id `manual-review`.

### Prefer Skills Over Raw Agents

When a skill covers the task (`/ze-rfc`, `/ze-review`, `/ze-implement`, etc.),
use it instead of spawning a raw agent or improvising the workflow. Skills
encode project conventions, gates, and ordering that a raw agent will miss.

- **`.claude/hooks/pretool-agent-skill.py` BLOCKS the spawn** when the agent prompt asks for something a skill covers. It matches the ASK, never the subject: "review this diff" is routed, "explain how review works" is not.
- **Naming the skill in the prompt satisfies the gate**, so a subagent that must follow `/ze-explore` is spawned by saying so.
- The map it enforces: research is `/ze-explore`, review is `/ze-review`, spec conformance is `/ze-review-spec`, a red test is `/ze-debug`, spec work is `/ze-implement`, bug classes are `/ze-hunt`, spec audit is `/ze-audit`.
- A hand-written prompt reproduces a worse version of the skill and drops every gate it carries. That is what the gate exists to stop.

### Commit Script Generation

Use `scripts/dev/commit_helper.py` for commit script preparation. It owns
session ID reuse, message file creation, executable per-commit script
generation (the path comes from its `script=` line), ignored-path rejection, `git commit -F`, and the learned-summary
gate for workflow/tooling/rule changes. Hand-write a commit script only when the
helper cannot express the commit shape, and keep the same generated-script
contract from `ai/rules/git-safety.md`.

On explicit commit requests, commit-helper invocation is the work. Do not run
late completeness checks, health checks, recent-commit style reviews, or
remaining-work tables unless the user explicitly asks for them. Before any
verify target, run `scripts/dev/verify-status.sh check`; a FRESH result
forbids rerunning `make ze-verify` or `make ze-verify-changed`.

### Skills

Version-matched skill content is embedded in the binary.

1. New skills go in `internal/plugins/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `internal/plugins/skills/main.go` must list every embedded file.
3. `ze skills list` must show all bundled skills without a static list elsewhere.

### Validation at Boundaries

Config validation must run at every boundary where config enters the system:

1. `ze config validate` (CLI).
2. `ze config fix --plan --json` (CLI agent surface).
3. Web commit (pre-commit validation of pending changes).
4. Hub API config push (`ValidateContent`).
5. `ze config validate --pending` (validate zefs pending config without committing).

All boundaries use the same diagnostic pipeline.

### Documentation

When adding agent-facing features:

1. Update `docs/features/ai-first.md` with the new command or contract.
2. Update `docs/guide/mcp/overview.md` if MCP users need to discover the feature.
3. Update embedded skill files if the feature changes the agent workflow.
4. Satisfy `ai/rules/repo-maintenance.md`: add the keyword/task path in
   `ai/INDEX.md`, and document the verification target
   that proves the feature is discoverable.
