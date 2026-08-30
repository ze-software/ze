# Configuration and YANG

**When:** adding or changing a config option, YANG module, env var, listener endpoint, or code that reads config values
**Severity:** blocking
**Related:** go-standards, cli, architecture, evidence, plugins

## Directives

- **Config content MUST be manipulated through one of two methods only.** See Config Manipulation.
- **Every tunable setting MUST live at the right level.** Misplacement erodes operator trust: invisible knobs surprise, config-tree clutter confuses.
- **Names cross four layers (YANG, env var, Go struct, CLI).** Each layer has its own convention, and the four MUST be derivable from each other. The conventions and the worked example are `ai/patterns/config-option.md`.
- **One concept, one spelling.** Where a shape already exists in the shared modules (`internal/component/config/yang/modules/ze-types.yang`, prefix `zt`; `ze-extensions.yang`, prefix `ze`), it MUST be reused. It MUST NOT be re-expressed locally.
- **Every coercion of a delivered config value MUST accept the string form.** See Config String Coercion.
- Config MUST NOT contain version numbers. Migration MUST be designed to be machine-transformable.
- Unknown keys MUST fail at any level. Silent ignore MUST NOT happen. The closest valid key MUST be suggested.
- Every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var registered via `env.MustRegister()`. Env vars are part of the config interface, not follow-up work.
- The structural template is `ai/patterns/config-option.md`; the YANG system it plugs into is `docs/architecture/config/yang-config-design.md`; the rationale is `ai/rationale/config-design.md`.

## Config Surface: YANG Config vs Env Var

**Default answer: YANG config.** Env-only is the exception, not the default. When uncertain, the setting SHOULD go in YANG. Promoting later is a breaking workflow change for operators who already use the env var.

**Settings in these categories SHOULD live in YANG config:**
- Queue depths, buffer sizes, batch limits, pool budgets
- Timers that affect convergence or session behavior
- Feature toggles that change observable routing behavior
- Capacity knobs (max peers, max prefixes, max routes)
- Any setting an operator would document in a change ticket

**Settings in these categories SHOULD stay env-only:**
- Emergency escape hatches (safety valves, deadline overrides)
- Debug instrumentation (artificial delays, verbose tracing)
- Bootstrap settings needed before config is parsed
- Internal safety caps that protect against code bugs, not traffic
- Metrics/observability plumbing intervals

1. Env var (emergency override; MUST take precedence)
2. YANG config value
3. YANG default (from schema)

**The YANG leaf description MUST document that an env var override exists and name it.** Operators SHOULD NOT be surprised that their config value is being overridden.

**These signals indicate an env var SHOULD become YANG config:**
- It appears in runbooks or deployment documentation
- Multiple operators have asked about it or been told to set it
- It controls behavior visible in `show` commands or logs
- Changing it is part of normal scaling or tuning workflows
- It was added as env-only for expedience during implementation

## Config Naming Conventions

**YANG leaf names MUST NOT use abbreviations.** Operators read YANG leaves in CLI completion and `show configuration`. `fwd` means nothing to someone who did not write the code. Leaf names MUST be spelled out in full: `forward`, `buffer`, `channel`, `maximum`.

**Ze has not shipped a release. So a singular `leaf-list` name MUST be corrected now, and the code that reads it fixed with it.**
A change whose only purpose is grammar consistency is CORRECT work while nothing is released, and MUST NOT be refused as churn.
The BGP YANG holds several singular leaf-lists today: `import`, `export`, `tag`, `strip`, `send`, `value` and `receive` among them. Each costs one commit now.
**After the first release, a configuration name is an API, and a rename MUST NOT land without a configuration migration tool.**
A rename breaks every deployed configuration that writes the old name. An operator whose configuration stops parsing after an upgrade has no way forward without that tool.
**The free period ends when the migration tool is owed, not on the release date alone.** MUST say which of the two states applies before you argue that a rename is too expensive.

**Env var leaf matches YANG leaf.** When a setting exists in both YANG and env, the final segment of the env var key MUST be the YANG leaf name. This makes the mapping mechanical and documentable.

**Legacy env vars that predate the YANG leaf keep their old key for backwards compatibility but MUST register an alias matching the YANG name.**

**When the YANG tree changes (leaf moves to a different container), the env var path MUST change too. The old path MUST become an alias.**

## Config Design

**Augment MUST be used only for cross-component plugin extensions.** Same-component shared structure MUST use grouping. If you are writing an augment and both the source and target are in the same component, a grouping MUST be used instead.

**All network listener endpoints MUST use the `zt:listener` grouping (`ip` + `port` from ze-types.yang) and the `ze:listener` extension (ze-extensions.yang) for port conflict detection.**

**`refine` MUST be used to set per-service defaults for ip and port.** The `ip` leaf MUST be `zt:ip-address` (numeric, not hostname) because listeners bind to local interfaces.

## YANG Module Structure

- `<component>` MAY contain hyphens for a multi-word name (`ddos-detect`, `firewall-irr`). The `:conf`, `:cmd` or `:api` kind MUST NOT be fused into it with a hyphen, and it MUST NOT be dropped. A plugin's `conf` and `cmd` modules MUST use the same scheme. The module identity table, and the one plugin that currently breaks this, is `docs/architecture/config/yang-config-design.md`.
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. They MUST NOT be reused for another import.

- Shared modules MUST be imported with their reserved prefixes: `import ze-types { prefix zt; }`, `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, `ze-types` MUST be imported and its typedef MUST be used. Skipping the `ze-types` import MUST NOT be treated as licence to re-invent its constraints. The typedef for each concept is `docs/architecture/config/yang-config-design.md`.
- Network endpoints, both binds and remote targets, MUST use the shared groupings. A hand-rolled pair MUST NOT be used, and a combined string MUST NOT be used.

- `ze:validate` is for **runtime-determined valid sets only**: registered address families, plugin names, IRR set references, or a union with a literal keyword (`nonzero-ipv4|literal-self`). It MUST NOT duplicate a constraint YANG native `pattern`/`range`/`enumeration` (or an existing `zt` typedef) already expresses. This is the stated contract in `ze-extensions.yang` on the `ze:validate` extension.

**A leaf whose value carries a physical unit (time, rate, size) MUST state the unit once, via the YANG `units` statement, MUST keep the leaf name unit-free, and MUST carry a protocol-sane `default`:** `leaf hello-interval { type uint32; units seconds; default 10; }`

**Defaults are requirements, not suggestions (`architecture.md`): a leaf the operator omits MUST still produce correct, standards-conformant behaviour.** The RFC/convention the default comes from MUST be cited in the leaf `description`.

**An endpoint (a place to bind, or a remote to connect to) MUST be two structured fields, and MUST come from a shared grouping.** A combined `"host:port"` string MUST NOT be used, and a hand-rolled `ip`/`port` pair MUST NOT be used.

- `ip` is a local literal address (`zt:ip-address`); `address` is a remote host that MAY be a name. The two field names encode that difference on purpose. `host` MUST NOT be used, and `ip` MUST NOT be used for a remote target.
- A combined `"host:port"` or `"address:port"` string MUST NOT be used (structured data, `ai/rules/evidence.md`). It MUST be split into the two fields.
- `port` MUST NOT be a bare `uint16` and MUST NOT be an inline `uint16 { range ... }`; the typedef MUST be used.
- The pair MAY be hand-modelled only for a documented exception, and the BGP peer-local `union`-with-`auto` is the one that exists. The grouping and port type for each endpoint kind is `docs/architecture/config/yang-config-design.md`.

**An on/off setting MUST have one shape:** `leaf enabled { type boolean; default false; }`

**Equivalent concepts MUST be modelled the same way across BGP, OSPF, IS-IS, BFD, LDP and RSVP-TE**, so an operator who has configured one protocol recognizes the next. Existing modelling MUST be reused before it is redefined. The canonical model for each shared concept, and the two divergences that are allowed, is `docs/architecture/config/yang-config-design.md`.

**A genuine RFC-term difference is the ONLY allowed divergence from a sibling protocol's modelling, and it MUST be justified in the leaf or container `description`.** Two exist today: the metric name (OSPF `cost` against IS-IS `metric`) and the router identity (`router-id` for BGP, OSPF and RSVP-TE, `lsr-id` for LDP, `system-id` plus `net` for IS-IS). The canonical model for every shared concept is `docs/architecture/config/yang-config-design.md`.

**Before adding a concept to a protocol module, the sibling protocols MUST be grepped for how they model it (`ai/rules/architecture.md`) and matched.**

**A NEW command module for an operational verb SHOULD take the majority `ze-cli-<verb>-cmd` form with a paired `-api`, and it MUST NOT invent a fourth scheme.** Converging the existing names is a rename tracked separately, and it is described in `docs/architecture/config/yang-config-design.md`. Command ownership and grammar rules are `ai/rules/cli.md` and `ai/rules/plugins.md`.

## Config Manipulation

**Config content MUST be manipulated through one of two methods only: a parsed YANG tree when a loaded tree is in memory, or set command lines when building or merging config text.** Why concatenating two valid config texts is itself valid is `docs/architecture/config/yang-config-design.md`.

**Config MUST NOT be edited by any of these methods:**
- Raw text surgery (regex, string replace, brace counting, line insertion)
- Custom merge functions that parse config syntax outside the config system
- Any manipulation that assumes config structure from text patterns

## Config String Coercion

**A config value MUST NOT be asserted directly with `v.(bool)` or `v.(float64)`, and a numeric or bool type switch MUST NOT omit a `case string:` arm.** `./le config coercion check` (`internal/le/config/coercion/configcoercion.go`, wired into `./le verify current mode full`) refuses both shapes across every `internal/**/config.go`. Why every delivered value is a JSON string is `docs/architecture/config/yang-config-design.md`.

**These patterns MUST NOT be used to read a config value:**
- `if b, ok := v.(bool); ok { cfg.Enabled = b }`: `v` is `"true"`, the assertion fails, `cfg.Enabled` keeps its `false` default. For a boolean `enabled` gate this disables the **entire feature** with no error, no panic, no log line.
- a `toInt`/`toFloat` helper whose type switch handles `int`/`int64`/`float64` but has no `case string:` arm returns `(0, false)` for every string, so the operator's configured value is silently ignored and the default is used.

**Every coercion of a delivered config value MUST accept the string form.** Use a helper with a `case string:` arm; `cfgBool` and the `case string:` arms of `toInt` and `toFloat` in `internal/plugins/trafficusage/config.go` are the reference, and the worked shape is `docs/architecture/config/yang-config-design.md`.

**An allowlist entry MUST be added, with a stated reason, only for a genuine non-config coercion.** The companion `--selftest` proves the AST detection fires on isolated fixtures.

## Config List Shapes

**A YANG `leaf-list` MUST be read with `configvalue.LeafList`, and a YANG `list`
with `configvalue.ListEntries` (`internal/core/configvalue`). A slice MUST NOT
be asserted on either.** `m["k"].([]any)` succeeds on a leaf-list at two members
and fails at one, so the option works when the operator writes two values and
vanishes when they write one. On a list it fails at every count, so the option
never applies at all. The shape each node arrives in is
`docs/architecture/config/yang-config-design.md`.

**A local helper that coerces a leaf-list or a list to STRINGS or to entries
MUST NOT be written.** One helper per package is how four readers came to handle
three different subsets of the shapes above, each a wrong model for the next
reader to copy.

**A coercion whose RESULT TYPE `configvalue` does not produce MAY stay local, and
it MUST accept all three leaf-list shapes.** Two exist and both are named here,
so a reader can tell a permitted local coercion from a rediscovered copy.

| Local coercion | Why it cannot delegate |
|----------------|------------------------|
| `configInstanceIDs` (`internal/plugins/ospf/config.go`) | yields `[]uint8`, and `LeafList` yields strings |
| `keyedList` (`internal/plugins/isis/config.go`) | sorts a list numerically for `key-id`, an order `ListEntries` does not offer |

**A list declared `ordered-by user` MUST be read with `configorder.Entries`
(`internal/core/configorder`), and MUST NOT be read through `ListEntries`.**
`ListEntries` sorts by key, so for first-match-wins semantics it substitutes a
lexical order for the configured one, which turns a loud refusal into a silently
wrong evaluation order.

| Reader | List kind | What it returns |
|--------|-----------|-----------------|
| `configvalue.ListEntries` | a list whose evaluation does not depend on order | entries sorted by key |
| `configorder.Entries` | a list declared `ordered-by user` | entries in the operator's order, or an error |

**An `ordered-by user` list whose order is RECOVERABLE FROM THE ENTRY DATA is
the one exception, and such a reader MUST sort on that data rather than reach
for `configorder.Entries`.** The delivered position would be a second spelling
of a fact the entry already carries, and two spellings of one fact disagree.
Two exist and both are named here, so a reader can tell a permitted sort from a
rediscovered one.

| Permitted sort | What carries the order |
|----------------|------------------------|
| `parseERO` (`internal/plugins/rsvpte/register.go`) | the list KEY is the numeric hop index, so sorting by key sorts by the operator's order |
| `parsePolicyRoute` (`internal/plugins/policyroute/config.go`) | each rule carries an explicit `order` leaf the operator writes |

**The order is carried by the LOWERING, so a plugin's config MUST be lowered
with `Tree.ToPluginMap` and MUST NOT be lowered with `Tree.ToMap`.**
`ToPluginMap` emits the entry order of every list holding two or more entries,
beside the list, under `configorder.OrderKey(listName)`. `ToMap` stays the
general-purpose lowering and carries no order, because gNMI, `ze config show`,
the web config handler, the support bundle and `ValidateTreeAllModules` all read
its output, and every key in it MUST be a name the YANG schema declares.

**A multi-entry ordered list delivered with NO order MUST be refused, and a
reader MUST NOT sort as a fallback.** `configorder.Entries` makes that refusal
for every caller, so no reader spells it. One entry needs no order and is
answered without the key, which is why the key stays out of the payload for the
common case.

**A test for a leaf-list reader MUST exercise the SINGLE-member case, and a test
for a list reader MUST use the keyed-map shape.** A multi-member leaf-list
fixture passes with the assertion bug in place, so it discriminates nothing, and
an array-of-entries fixture for a list feeds a shape no producer emits.
