# Configuration and YANG

**When:** adding or changing a config option, YANG module, env var, listener endpoint, or code that reads config values
**Severity:** blocking
**Related:** go-standards, cli, architecture, evidence, plugins

## Directives

- **Config content MUST be manipulated through one of two methods only.** See Config Manipulation for the two methods and the forbidden list.
- **Every tunable setting MUST live at the right level.** Misplacement erodes operator trust: invisible knobs surprise, config-tree clutter confuses.
- **Names cross four layers (YANG, env var, Go struct, CLI).** Each layer has its own convention, but they MUST be derivable from each other. An operator reading `show configuration` SHOULD recognize the env var from the docs, and vice versa.
- **One concept, one spelling.** Where a shape already exists in the shared modules (`internal/component/config/yang/modules/ze-types.yang`, prefix `zt`; `ze-extensions.yang`, prefix `ze`), it MUST be reused. It MUST NOT be re-expressed locally.
- **Every coercion of a delivered config value MUST accept the string form.** See Config String Coercion.
- Config MUST NOT contain version numbers. Migration MUST be designed to be machine-transformable.
- Unknown keys MUST fail at any level. Silent ignore MUST NOT happen. The closest valid key MUST be suggested.
- Every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var registered via `env.MustRegister()`. Env vars are part of the config interface, not follow-up work.

Structural template: `ai/patterns/config-option.md`
Rationale: `ai/rationale/config-design.md`

## Config Surface: YANG Config vs Env Var

### Decision Table

| Question | If YES | If NO |
|----------|--------|-------|
| Would an operator change this during normal capacity planning or traffic engineering? | YANG config | Keep reading |
| Does it need validation, commit/rollback, or config diff? | YANG config | Keep reading |
| Should it appear in `show configuration` or config backups? | YANG config | Keep reading |
| Is it a debug, emergency, or development-only knob? | Env var only | YANG config |
| Is it needed before config loads (bootstrap)? | Env var only | YANG config |
| Is it a safety cap that should never be tuned in production? | Env var only | YANG config |

**Default answer: YANG config.** Env-only is the exception, not the default. When uncertain, the setting SHOULD go in YANG. Promoting later is a breaking workflow change for operators who already use the env var.

### YANG Config (operator-facing)

Settings that belong in the config tree:

**Settings in these categories SHOULD live in YANG config:**
- Queue depths, buffer sizes, batch limits, pool budgets
- Timers that affect convergence or session behavior
- Feature toggles that change observable routing behavior
- Capacity knobs (max peers, max prefixes, max routes)
- Any setting an operator would document in a change ticket

Properties: visible in `show configuration`, validated by YANG constraints, part of commit/rollback, included in config backups, discoverable via CLI completion.

### Env Var Only (internal/debug)

Settings that stay as env vars:

**Settings in these categories SHOULD stay env-only:**
- Emergency escape hatches (safety valves, deadline overrides)
- Debug instrumentation (artificial delays, verbose tracing)
- Bootstrap settings needed before config is parsed
- Internal safety caps that protect against code bugs, not traffic
- Metrics/observability plumbing intervals

Properties: invisible to operators unless they read the source, no validation, no commit/rollback, requires restart to change.

### When Both Exist

When a setting is promoted from env-only to YANG config, the env var remains as an override. Precedence (highest wins):

1. Env var (emergency override; MUST take precedence)
2. YANG config value
3. YANG default (from schema)

**The YANG leaf description MUST document that an env var override exists and name it.** Operators SHOULD NOT be surprised that their config value is being overridden.

### Promotion Signals

An env var should be promoted to YANG config when any of these are true:

**These signals indicate an env var SHOULD become YANG config:**
- It appears in runbooks or deployment documentation
- Multiple operators have asked about it or been told to set it
- It controls behavior visible in `show` commands or logs
- Changing it is part of normal scaling or tuning workflows
- It was added as env-only for expedience during implementation

### New Setting Checklist

Before adding any tunable setting:

```
[ ] Classified as YANG config or env-only using the decision table above
[ ] If YANG: leaf defined with type, range, default, description
[ ] If YANG: description mentions env var override if one exists
[ ] If env-only: env.MustRegister() with clear description
[ ] If env-only: document WHY it is not in YANG (debug, bootstrap, safety cap)
[ ] If promoting: old env var preserved, precedence documented
```

## Config Naming Conventions

### YANG Leaves

| Rule | Example | Anti-pattern |
|------|---------|-------------|
| kebab-case, no abbreviations | `forward-queue-size` | `fwd-chan-size` |
| Noun or noun-phrase | `read-buffer-size` | `read-buf-sz` |
| Dimensioned value: state the unit via a YANG `units` statement, keep the name unit-free (see Units) | `teardown-grace` + `units seconds;` | `teardown-grace-seconds` (unit in the name), `teardown-grace` with no `units` |
| No `ze-` prefix (implicit in the tree) | `cache-ttl` | `ze-cache-ttl` |
| Boolean: positive assertion | `update-groups` | `no-update-groups`, `disable-update-groups` |
| **A `leaf-list` or a `list` is named in the PLURAL. A single `leaf` is named in the singular.** The name states how many values the operator may write, so a reader knows before reaching the type | `communities`, `prefixes`, `as-sets` | `community`, `prefix`, `as-set` on a leaf-list |

**Ze has not shipped a release. So a singular `leaf-list` name MUST be corrected now, and the code that reads it fixed with it.**
A change whose only purpose is grammar consistency is CORRECT work while nothing is released, and MUST NOT be refused as churn.
The BGP YANG holds several singular leaf-lists today: `import`, `export`, `tag`, `strip`, `send`, `value` and `receive` among them. Each costs one commit now.
**After the first release, a configuration name is an API, and a rename MUST NOT land without a configuration migration tool.**
A rename breaks every deployed configuration that writes the old name. An operator whose configuration stops parsing after an upgrade has no way forward without that tool.
**The free period ends when the migration tool is owed, not on the release date alone.** MUST say which of the two states applies before you argue that a rename is too expensive.

**YANG leaf names MUST NOT use abbreviations.** Operators read YANG leaves in CLI completion and `show configuration`. `fwd` means nothing to someone who did not write the code. Leaf names MUST be spelled out in full: `forward`, `buffer`, `channel`, `maximum`.

Exception: industry-standard abbreviations that are clearer than their expansion: `ttl`, `mtu`, `tcp`, `bgp`, `asn`, `med`, `ebgp`, `ibgp`.

### Env Vars

| Rule | Example |
|------|---------|
| Dot-separated, lowercase | `ze.bgp.reactor.forward-queue-size` |
| Prefix: `ze.<component>` | `ze.bgp.reactor.cache-ttl` |
| Leaf name matches YANG leaf exactly | YANG `forward-queue-size` = env `ze.bgp.reactor.forward-queue-size` |

**Env var leaf matches YANG leaf.** When a setting exists in both YANG and env, the final segment of the env var key MUST be the YANG leaf name. This makes the mapping mechanical and documentable.

**Legacy env vars that predate the YANG leaf keep their old key for backwards compatibility but MUST register an alias matching the YANG name.**

| Legacy env var | YANG leaf | Alias (MUST add) |
|----------------|-----------|------------------|
| `ze.fwd.chan.size` | `forward-queue-size` | `ze.bgp.reactor.forward-queue-size` |
| `ze.buf.read.size` | `read-buffer-size` | `ze.bgp.reactor.read-buffer-size` |

### Hierarchy: Env Var Path Mirrors YANG Path

The env var dotted path should mirror the YANG tree path from the component root down:

| YANG path | Env var |
|-----------|---------|
| `bgp / reactor / cache-ttl` | `ze.bgp.reactor.cache-ttl` |
| `bgp / reactor / forward-queue-size` | `ze.bgp.reactor.forward-queue-size` |
| `bgp / session / openwait` | `ze.bgp.session.openwait` |
| `hub / server / idle-timeout` | `ze.hub.server.idle-timeout` |

**When the YANG tree changes (leaf moves to a different container), the env var path MUST change too. The old path MUST become an alias.**

### Go Struct Fields

| Rule | Example |
|------|---------|
| PascalCase of the YANG leaf | `ForwardQueueSize` |
| Same word boundaries | `ReadBufferSize` (not `ReadBufSize`) |

### Container Naming

| Rule | Example | Anti-pattern |
|------|---------|-------------|
| Singular noun for the subsystem | `reactor` | `reactor-settings`, `reactor-config` |
| No `-config` or `-settings` suffix | `session` | `session-config` |
| Group related leaves, not one-per-container | `reactor { cache-ttl; cache-max; forward-queue-size; }` | `reactor-cache { ttl; max; }` + `reactor-forward { queue-size; }` |

### Naming New Settings (checklist)

```
[ ] YANG leaf: full words, kebab-case, no abbreviations
[ ] YANG leaf: dimensioned value states its unit via a `units` statement, name unit-free (see Units)
[ ] Env var: ze.<component>.<container>.<yang-leaf-name>
[ ] Env var leaf segment matches YANG leaf name exactly
[ ] Go struct: PascalCase of YANG leaf, same word boundaries
[ ] If legacy env var exists: alias registered matching new convention
[ ] Boolean: positive form (enabled, not disabled)
```

## Config Design

### YANG Structure

| Pattern | Use |
|---------|-----|
| `grouping` + `uses` | Shared structure within or across components |
| `augment` | Only when a plugin extends another component's YANG |

**Augment MUST be used only for cross-component plugin extensions.** Same-component shared structure MUST use grouping. If you are writing an augment and both the source and target are in the same component, a grouping MUST be used instead.

### Listeners

**All network listener endpoints MUST use the `zt:listener` grouping (`ip` + `port` from ze-types.yang) and the `ze:listener` extension (ze-extensions.yang) for port conflict detection.**

| Pattern | When |
|---------|------|
| `container` + `ze:listener` + `uses zt:listener` | Single-endpoint services (web, SSH, MCP, LG, telemetry, BGP global listen) |
| `list` + `ze:listener` + `uses zt:listener` | Named multi-instance listeners (plugin hub server) |
| `container` + `ze:listener` + manual ip/port | When ip type differs from standard (BGP peer local: union with auto enum) |

**`refine` MUST be used to set per-service defaults for ip and port.** The `ip` leaf MUST be `zt:ip-address` (numeric, not hostname) because listeners bind to local interfaces.

## YANG Module Structure

### Module Identity

| Element | Canonical | Anti-pattern |
|---------|-----------|--------------|
| Module name | `ze-<component>[-<kind>]`, matches the filename | `exabgp` (unprefixed; external-compat only) |
| Namespace | `urn:ze:<component>:<kind>`, where `<kind>` (`conf`/`cmd`/`api`) is ALWAYS a final colon segment | `urn:ze:ddos-detect-conf` (kind baked with `-`), `urn:ze:role` (no kind segment) |
| Prefix | short, lowercase, **unquoted**, no hyphens, derived from the module | `prefix "bgp-mon-api";` (quoted, hyphens, abbreviated), `prefix updateshowcmd;` |
| `revision` | at least one `revision YYYY-MM-DD { description ...; }` | no revision statement |
| `description` | module-level `description` required | omitted |
| `organization` / `contact` | omit (not a project convention; present in only one legacy batch) | adding `organization` to new modules |

- `<component>` MAY contain hyphens for a multi-word name (`ddos-detect`, `firewall-irr`). The `:conf`/`:cmd`/`:api` kind MUST NOT be fused into it with a hyphen and MUST NOT be dropped. A plugin's `conf` and `cmd` modules MUST use the same scheme (today `ddos-local` mixes `urn:ze:ddos-local-conf` with `urn:ze:ddos-local:cmd`, and that is the bug this row forbids).
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. They MUST NOT be reused for another import.

### Imports and Shared Vocabulary

- Shared modules MUST be imported with their reserved prefixes: `import ze-types { prefix zt; }`, `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, `ze-types` MUST be imported and its typedef MUST be used. Skipping the `ze-types` import MUST NOT be treated as licence to re-invent its constraints (see Value Typing).
- Network endpoints (binds and remote targets): the shared groupings MUST be used, never a hand-rolled pair or a combined string. See Network Endpoints below.

### Value Typing

Use the shared typedef; do not re-express the same constraint a second way.

| Concept | Use | Do NOT use |
|---------|-----|------------|
| IPv4 / IPv6 / either address | `zt:ipv4-address` / `zt:ipv6-address` / `zt:ip-address` | raw `type string`; `type string; ze:validate "ipv4-address"` |
| IPv4 / IPv6 / either prefix | `zt:prefix-ipv4` / `zt:prefix-ipv6` / `zt:ip-prefix` | `type string; ze:validate "ipv4-prefix\|ipv6-prefix"` |
| ASN, port | `zt:asn` / `zt:asn2`, `zt:port` / `zt:listener-port` | inline `uint32`/`uint16` with a copied range |
| Community / RD / address-family | `zt:community`, `zt:route-distinguisher`, `zt:address-family` | per-module patterns for the same shape |
| MAC address | `zt:mac-address` (add it to `ze-types` if absent) | per-plugin `ze:validate "mac-address"` |
| Duration / dimensioned value | an unsigned integer leaf with a YANG `units` statement (see Units below) | `type string` for a duration; the unit only implied in the description |

- `ze:validate` is for **runtime-determined valid sets only**: registered address families, plugin names, IRR set references, or a union with a literal keyword (`nonzero-ipv4|literal-self`). It MUST NOT duplicate a constraint YANG native `pattern`/`range`/`enumeration` (or an existing `zt` typedef) already expresses. This is the stated contract in `ze-extensions.yang` on the `ze:validate` extension.

### Units

**A leaf whose value carries a physical unit (time, rate, size) MUST state the unit once, via the YANG `units` statement, MUST keep the leaf name unit-free, and MUST carry a protocol-sane `default`:** `leaf hello-interval { type uint32; units seconds; default 10; }`

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| One mechanism | `type uint32; units milliseconds;` | unit in the leaf name (`min-tx-us`, `spf-delay-ms`, `teardown-grace-seconds`) |
| Full word, unquoted | `units microseconds;`, `units seconds;`, `units bytes/second;` | `units "seconds";` (quoted), `-us` / `-ms` / `-secs` abbreviations |
| Integer, not string | `type uint32; units seconds;` | `type string` for a duration |
| Protocol-sane default | every dimensioned leaf carries a `default` set to the protocol's standard/recommended value (OSPF `hello-interval` 10s, `dead-interval` 40s, BFD tx/rx per RFC 5880, ...) | no `default`, so omitting the leaf yields 0 or undefined timing |

**Defaults are requirements, not suggestions (`architecture.md`): a leaf the operator omits MUST still produce correct, standards-conformant behaviour.** The RFC/convention the default comes from MUST be cited in the leaf `description`.

This supersedes the "unit suffix in the leaf name" guidance for dimensioned values; the YANG Leaves table above defers to this section.

### Network Endpoints

**An endpoint (a place to bind, or a remote to connect to) MUST be two structured fields, and MUST come from a shared grouping.** A combined `"host:port"` string MUST NOT be used, and a hand-rolled `ip`/`port` pair MUST NOT be used.

| Endpoint kind | Grouping | Fields | Port type |
|---------------|----------|--------|-----------|
| Inbound bind (the service listens) | `uses zt:listener` + `ze:listener` extension | `ip` (local literal), `port` | `zt:listener-port` (0 = OS-assigned) |
| Outbound target (the service connects out) | `uses zt:endpoint` (add to `ze-types` if absent) | `address` (IP or hostname), `port` | `zt:port` (1..65535) |

- `ip` is a local literal address (`zt:ip-address`); `address` is a remote host that MAY be a name. The two field names encode that difference on purpose. `host` MUST NOT be used, and `ip` MUST NOT be used for a remote target.
- A combined `"host:port"` / `"address:port"` string MUST NOT be used (structured data, `evidence.md`). It MUST be split into the two fields.
- `port` MUST NOT be a bare `uint16` and MUST NOT be an inline `uint16 { range ... }`; the typedef MUST be used.
- The pair MAY be hand-modelled only for a documented exception (BGP peer-local `union`-with-`auto`); see Listeners above.

### Boolean Toggles and Flags

**An on/off setting MUST have one shape:** `leaf enabled { type boolean; default false; }`

| Rule | Detail |
|------|--------|
| Positive assertion, one word | `enabled`, not `enable`, `disable`, or `disabled` (see YANG Leaves, positive boolean). |
| Standard admin-state words are the only exception | `shutdown` (BFD, RFC 5880 §6.8.16) and interface `disable` (kernel admin-down) are allowed because they are the canonical protocol/kernel terms, but type them as `boolean` with `default false`, never `type empty`. |
| No boolean-as-enum | Do not model a two-value on/off as `enumeration { enum enable; enum disable; }`. If config inheritance genuinely needs a distinct unset state, justify that tri-state in the module: it is an exception, not the default. |
| Bare flag | For "this section is on when present", use a `presence` container. Do not use a `type empty` leaf. |

### Defaults and Enums

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| Boolean default is unquoted | `default false;` | `default "false";` |
| enum `value N` only for wire numbers | assign `value` when the number is protocol-significant (AFI/SAFI/ORIGIN); otherwise omit | assigning arbitrary values to cosmetic enums |

### Layout

| Rule | Detail |
|------|--------|
| Indentation | 4 spaces per level. No tabs, no 2-space modules. |
| Compact leaf | A leaf whose body is only `type` (+ optional `default` and/or `description`) MAY be one line: `leaf med { type uint32; description "..."; }`. |
| Expanded leaf | A leaf with nested constraints (`pattern`, `range`, `enumeration`, `must`, multiple sub-statements) MUST be expanded, one statement per line. |
| List key | quoted: `key "name";`. Prefer `name` for the operator-assigned key. |

### Cross-Protocol Consistency

**Equivalent concepts MUST be modelled the same way across BGP, OSPF, IS-IS, BFD, LDP, and RSVP-TE.** Having configured one protocol, an operator SHOULD recognize the next. Existing modelling MUST be reused before it is redefined.

| Concept | Canonical | Do NOT |
|---------|-----------|--------|
| BFD integration | `container bfd { leaf enabled; [leaf mode;] leaf profile; }` referencing a profile in the top-level `bfd { profile <name> }` list (BGP's pattern) | redefine BFD timers inline (`min-tx`/`min-rx`/`multiplier`); every protocol that supports BFD references a profile |
| Authentication | reference a shared `key-chains` list (IS-IS's model) via a `leaf key-chain`; name the auth container the same everywhere | a per-protocol private key store; container named `md5`/`auth`/`authentication` differently per protocol; reference leaf named `key-chain` in one place and `auth-key-chain` in another |
| Per-interface protocol config | `container interfaces { list interface { key "name"; ... } }` (OSPF/IS-IS) | a bare top-level `list interface` (RSVP-TE) or a `leaf-list interfaces` when per-interface settings exist (LDP) |
| Multiplier / interval / timer names | one vocabulary for the same concept; dimensioned via a `units` statement (see Units) | four names for one concept (`detect-multiplier` vs `multiplier` for the same BFD field) |
| Toggle | positive `enabled` at every nesting level, including sub-features | `enabled` on the interface but `enable` on its sub-blocks |

Genuine RFC-term differences are the ONLY allowed divergence and MUST be justified in the leaf/container `description`:
- Metric name: OSPF `cost` vs IS-IS `metric` (each is that protocol's RFC term).
- Router identity: `router-id` (BGP/OSPF/RSVP-TE) vs `lsr-id` (LDP) vs `system-id` + `net` (IS-IS).

**Before adding a concept to a protocol module, the sibling protocols MUST be grepped for how they model it (`ai/rules/architecture.md`) and matched.**

### Command Modules (naming standardization deferred)

The `-cmd` (grammar tree) and `-api` (handler) modules for operational verbs are currently named several ways for the same verb (`ze-cli-monitor-cmd` vs `ze-monitor-cmd` vs `ze-command-monitor-cmd`; `ze-bgp-cmd-log-api` for a non-BGP command). Converging them on one scheme is a rename that touches `//go:embed`, `register.go`, and YANG dispatch keys, so it is tracked separately and NOT done piecemeal.

**Until then: a NEW command module SHOULD follow the majority `ze-cli-<verb>-cmd` / paired `-api` form and MUST NOT invent a fourth scheme.** Command ownership and grammar rules live in `cli.md` and `plugins.md`.

### YANG Mechanical Check

Before saving a `.yang` edit:

```
[ ] Namespace is urn:ze:<component>:<kind> (kind is a colon segment)
[ ] Prefix is short, unquoted, no hyphens; zt/ze not reused
[ ] Module has a revision and a description; no stray organization
[ ] Every IP/prefix/ASN/port/community leaf uses the zt typedef, not a copy
[ ] ze:validate used only for runtime sets, never to duplicate a pattern/range
[ ] Dimensioned leaf: integer + `units <full-word>` + protocol-sane `default`; no unit in the name
[ ] Endpoint: uses zt:listener (bind) or zt:endpoint (target); no combined host:port string
[ ] BFD integration references a bfd profile; auth references the shared key-chains
[ ] Cross-protocol concept matches its siblings (grep OSPF/IS-IS/BGP first)
[ ] Toggles are positive `enabled` booleans; no type empty, no enable/disable enum
[ ] 4-space indent; compact leaves only for type(+default/description)
```

## Config Manipulation

Config content MUST be manipulated through one of two methods only.

| Method | When |
|--------|------|
| Parsed YANG tree | When you have a loaded config tree in memory |
| Set command lines | When building or merging config text |

### Forbidden

**Config MUST NOT be edited by any of these methods:**
- Raw text surgery (regex, string replace, brace counting, line insertion)
- Custom merge functions that parse config syntax outside the config system
- Any manipulation that assumes config structure from text patterns

The config format IS set commands. Duplicate blocks are additive. The parser handles merging. Concatenating two valid config texts produces valid config.

## Config String Coercion

### The problem

The plugin config framework delivers every YANG leaf value to a plugin's `ParseConfig` as a JSON **string** (`"true"`, `"50000"`, `"3.5"`), never the native JSON type. A hand-written parser that coerces a config value with a native-type assertion always fails the assertion on that string and silently falls back to the leaf's **default**:

**These patterns MUST NOT be used to read a config value:**
- `if b, ok := v.(bool); ok { cfg.Enabled = b }`: `v` is `"true"`, the assertion fails, `cfg.Enabled` keeps its `false` default. For a boolean `enabled` gate this disables the **entire feature** with no error, no panic, no log line.
- a `toInt`/`toFloat` helper whose type switch handles `int`/`int64`/`float64` but has no `case string:` arm returns `(0, false)` for every string, so the operator's configured value is silently ignored and the default is used.

Confirmed real instance: `ddos-detect` never ran in any daemon. `enabled` parsed `false` from the string `"true"`, so the detector never subscribed to the rate feed and never fired (session 6503). The BPS/persistence/confidence code was correct; it was never reached.

### The rule

**Every coercion of a delivered config value MUST accept the string form.** Use a helper with a `case string:` arm (see `internal/plugins/trafficusage/config.go` `cfgBool` and the `case string:` arms in its `toInt`/`toFloat`), shown under Examples.

**A config value MUST NOT be asserted directly with `v.(bool)` / `v.(float64)`, and a numeric/bool type switch MUST NOT omit a `case string:` arm.**

### The mechanical check

`internal/le/config/coercion/configcoercion.go` (`./le config coercion check`, wired into `./le verify current mode full`) parses every `internal/**/config.go` and fails on a type switch whose cases include a numeric/bool type but not `string`, or a direct type assertion to a numeric/bool type.

**An allowlist entry MUST be added, with a stated reason, only for a genuine non-config coercion.** The companion `--selftest` proves the AST detection fires on isolated fixtures.

## Config List Shapes

`Tree.ToMap` does not hand a reader what the YANG node type suggests, and JSON
delivery adds one more shape on top.

| Node | Members | Shape in process | Shape after JSON |
|------|---------|------------------|------------------|
| `leaf-list` | none active | key absent | key absent |
| `leaf-list` | exactly one | bare `string` | bare `string` |
| `leaf-list` | two or more | `[]string` | `[]any` |
| `list` | any count, one included | `map[string]any` keyed by the list key | `map[string]any` keyed by the list key |

A `list` is never a slice, and its key leaf is the map key rather than a field
inside the entry.

**A YANG `leaf-list` MUST be read with `configvalue.LeafList`, and a YANG `list`
with `configvalue.ListEntries` (`internal/core/configvalue`). A slice MUST NOT
be asserted on either.** `m["k"].([]any)` succeeds on a leaf-list at two members
and fails at one, so the option works when the operator writes two values and
vanishes when they write one. On a list it fails at every count, so the option
never applies at all.

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
wrong evaluation order. `serverEntries`
(`internal/component/l2tp/plugins/authradius/config.go`) was that failure in the
tree until 2026-08-23: it sorted an `ordered-by user` `list server` by name for
what its comment called deterministic failover ordering, so RADIUS failover ran
alphabetically.

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

## Examples

String-tolerant coercion helper (Config String Coercion):

```go
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
	}
	return false, false
}
```

Dimensioned leaf (Units) and boolean toggle (Boolean Toggles and Flags):

```
leaf hello-interval { type uint32; units seconds; default 10; }
leaf enabled { type boolean; default false; }
```
