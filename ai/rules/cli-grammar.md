# CLI Grammar: Keywords Before Values

**When:** adding or changing any CLI command, root command, or YANG command path
**Severity:** blocking

## Directives

Every CLI command must place a closed keyword before any
user-supplied value. This eliminates ambiguity where a free-form value
could collide with a keyword.

## The Rule

```
<verb> <noun> <action> [<args>]
<verb> <noun> <selector-kind> <selector-value> <action> [<args>]
```

The first token after the noun (component/resource) MUST be a keyword from a
closed set known at compile time. If a command targets one member of a set,
the selector itself MUST also be typed by a keyword such as `name`, `id`,
`index`, `address`, or `type`. Free-form values MUST NOT appear in an untyped
positional slot.

## Correct vs Incorrect

| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show interface <name>` | `show interface name <name> detail` | `<name>` is untyped and could collide with keywords (`brief`, `errors`) |
| `show interface <name> counters` | `show interface name <name> counters` | Selector value appears before selector kind |
| `show l2tp session <id>` | `show l2tp session id <id> detail` | ID must be typed before use |
| `show vpn ipsec peer <name>` | `show vpn ipsec peer name <name> detail` | Named lookup needs an explicit selector kind |
| `cache <id> retain` | `cache retain <id>` | ID before action |
| `commit <name> start` | `commit start <name>` | Name before action |

## Typed Selectors

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

### Peer Commands

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

## Named-Resource Commands

The "named-resource" pattern `<resource> <id> <action>` violates this rule.
Correct form: `<resource> <action> <id>`.

- `cache retain <id>`, not `cache <id> retain`
- `commit start <name>`, not `commit <name> start`

The `list` action (no identifier) already works correctly in both.

## Compound Token vs Namespace Split (R9)

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

## Choosing the Verb: Read vs Perturb (`show`/`monitor` vs `debug`)

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

## Engine-Owned Tree Mutation

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

## YANG Module Ownership

Do not move a command family into a different YANG module while fixing
grammar unless the architecture explicitly requires that move.

First identify the owning module from source:

- the existing `ze:command` path
- the module `register.go` / `embed.go`
- the handler `RPCRegistration`

Then edit that module. Grammar cleanup is not permission to reshuffle
command ownership across components.

## Migrating a Built-in Command's Path (dispatch key = YANG path)

The daemon dispatcher registers each built-in handler under its **YANG path**, not
its wire method: `LoadBuiltins` does `d.RegisterWithOptions(wireToPath[wireMethod], ...)`
(`internal/component/plugin/server/command.go`). So **moving a `ze:command` container
in the YANG tree changes the command's dispatch key.** Relocating `command list` to
`show command list` deletes the old `command list` key entirely.

That is fine for a command an operator types. It is a **wire break** for any command a
plugin or script sends by its bare path — over the plugin CLI protocol
(`dispatch-command` / `dispatch-command-args`) or an interactive `ze <subsystem> plugin cli`
session. A verb-first "rename" of such a command is a protocol break, not a cosmetic
change.

Before migrating any noun-first built-in to verb-first:

1. **Grep for programmatic senders of the bare path** — `.ci` tests, `DispatchCommand`
   / `DispatchCommandArgs` calls, `printf ... | ze ... plugin cli`, `SendCommand`. Update
   every one in the same change. (In-tree these are greppable; external plugins/scripts
   are not, so a hard cut with no deprecation carries that residual risk.)
2. The **programmatic plugin SDK is not affected** — it uses structured RPC wire methods
   (`ze-plugin-callback:*`, `pkg/plugin/sdk/sdk_dispatch.go`), not command-path strings.
   Only the interactive plugin CLI and `dispatch-command` carry command-path strings, and
   in-tree those are already verb-first (e.g. `bmp.go` sends `show bgp rib protocol`).
3. Some noun-first forms are a deliberate **namespace**, not un-migrated legacy. Keep
   them: `plugin encoding/format/ack` group plugin-session directives under `plugin`, and
   `set plugin` would collide with the config-tree `plugin` node (see Engine-Owned Tree
   Mutation). `command list/help/complete` (engine introspection) migrated cleanly to
   `show command …`; `plugin …` stayed.

When you migrate, also drop the command's verb from `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) if it was only there as a legacy
noun-first form.

## Identifiers Are Strings

Use string-typed identifiers even when the conventional representation is numeric.
Cache IDs, VLAN IDs, session IDs: accept and store as strings. Parse to numeric
only at the point of use if the underlying API requires it. This avoids:
- Grammar ambiguity between numeric keywords and numeric IDs
- Unnecessary coupling to a representation (IDs may become non-numeric later)
- Parsing errors surfacing at the wrong layer

## Backward Compatibility

If the wrong grammar has not shipped, replace it outright. Do not add
deprecation branches for unreleased syntax.

If fixing a released grammar, accept the old grammar with a deprecation warning.
Log the warning once per session. Remove old grammar after two release cycles.

## Mechanical Check

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

## Mechanical Enforcement (automated)

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
contains only conforming commands by construction -- a boot-and-dump audit would add
no catch value while depending on an all-plugins config that cannot exist (startup is
config-path-gated). The guard instead locks the two runtime sources against
regression cheaply and deterministically.

To add a verb, edit `command.Verbs` (one place; the plugin gate and the static gate
both derive from it). Category exemptions (the text bridge, `ze-plugin:`/`ze-system:`
wire-protocol directives, and `ze-editor:` modes) live in `grammar.ExemptCategory`,
keyed on the handler wire-method namespace -- never a per-command allowlist.

## YANG Tree

The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface name <name> detail`,
the YANG tree needs a `name` container under `interface`, then `detail`
under that selector. Filter forms like `show interface type <type>` need a
`type` container that consumes the typed selector value.

## No Flag Syntax in YANG (Filters Are Keyword Grammar)

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

## Applies To

All CLI commands: online (RPC handlers via YANG dispatch) and offline
(`cmd/ze/` subcommand dispatch). No exceptions for "simple" commands or
"obvious" identifier positions.
