# YANG Module Structure

**When:** authoring or editing any `*.yang` module (module identity, imports, value typing, layout). Complements `config-naming.md` (leaf/env/struct names) and `cli-grammar.md` (command grammar).
**Severity:** advisory
**Related:** config-naming, config-design, config-surface, cli-grammar, naming

## Directives

One concept, one spelling. Where a shape already exists in the shared modules
(`internal/component/config/yang/modules/ze-types.yang`, prefix `zt`;
`ze-extensions.yang`, prefix `ze`), reuse it -- do not re-express it locally.

## Module Identity

| Element | Canonical | Anti-pattern |
|---------|-----------|--------------|
| Module name | `ze-<component>[-<kind>]`, matches the filename | `exabgp` (unprefixed; external-compat only) |
| Namespace | `urn:ze:<component>:<kind>` -- `<kind>` (`conf`/`cmd`/`api`) is ALWAYS a final colon segment | `urn:ze:ddos-detect-conf` (kind baked with `-`), `urn:ze:role` (no kind segment) |
| Prefix | short, lowercase, **unquoted**, no hyphens, derived from the module | `prefix "bgp-mon-api";` (quoted, hyphens, abbreviated), `prefix updateshowcmd;` |
| `revision` | at least one `revision YYYY-MM-DD { description ...; }` | no revision statement |
| `description` | module-level `description` required | omitted |
| `organization` / `contact` | omit (not a project convention; present in only one legacy batch) | adding `organization` to new modules |

- `<component>` may contain hyphens for a multi-word name (`ddos-detect`,
  `firewall-irr`). The `:conf`/`:cmd`/`:api` kind is never fused into it with a
  hyphen and is never dropped. A plugin's `conf` and `cmd` modules MUST use the
  same scheme (today `ddos-local` mixes `urn:ze:ddos-local-conf` with
  `urn:ze:ddos-local:cmd` -- that is the bug this row forbids).
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. Never reuse them for
  another import.

## Imports and Shared Vocabulary

- Import shared modules with their reserved prefixes: `import ze-types { prefix zt; }`,
  `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, import `ze-types` and use the
  typedef. Not importing `ze-types` is not a licence to re-invent its constraints
  (see Value Typing).
- Network endpoints (binds and remote targets): use the shared groupings, never
  a hand-rolled pair or a combined string. See Network Endpoints below.

## Value Typing

Use the shared typedef; do not re-express the same constraint a second way.

| Concept | Use | Do NOT use |
|---------|-----|------------|
| IPv4 / IPv6 / either address | `zt:ipv4-address` / `zt:ipv6-address` / `zt:ip-address` | raw `type string`; `type string; ze:validate "ipv4-address"` |
| IPv4 / IPv6 / either prefix | `zt:prefix-ipv4` / `zt:prefix-ipv6` / `zt:ip-prefix` | `type string; ze:validate "ipv4-prefix\|ipv6-prefix"` |
| ASN, port | `zt:asn` / `zt:asn2`, `zt:port` / `zt:listener-port` | inline `uint32`/`uint16` with a copied range |
| Community / RD / address-family | `zt:community`, `zt:route-distinguisher`, `zt:address-family` | per-module patterns for the same shape |
| MAC address | `zt:mac-address` (add it to `ze-types` if absent) | per-plugin `ze:validate "mac-address"` |
| Duration / dimensioned value | an unsigned integer leaf with a YANG `units` statement (see Units below) | `type string` for a duration; the unit only implied in the description |

- `ze:validate` is for **runtime-determined valid sets only** -- registered
  address families, plugin names, IRR set references, or a union with a literal
  keyword (`nonzero-ipv4|literal-self`). It MUST NOT duplicate a constraint YANG
  native `pattern`/`range`/`enumeration` (or an existing `zt` typedef) already
  expresses. This is the stated contract in `ze-extensions.yang` on the
  `ze:validate` extension.

### Units

A leaf whose value carries a physical unit (time, rate, size) states the unit
**once**, via the YANG `units` statement, keeps the leaf name unit-free, and
carries a protocol-sane `default`.

```
leaf hello-interval { type uint32; units seconds; default 10; }
```

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| One mechanism | `type uint32; units milliseconds;` | unit in the leaf name (`min-tx-us`, `spf-delay-ms`, `teardown-grace-seconds`) |
| Full word, unquoted | `units microseconds;`, `units seconds;`, `units bytes/second;` | `units "seconds";` (quoted), `-us` / `-ms` / `-secs` abbreviations |
| Integer, not string | `type uint32; units seconds;` | `type string` for a duration |
| Protocol-sane default | every dimensioned leaf carries a `default` set to the protocol's standard/recommended value (OSPF `hello-interval` 10s, `dead-interval` 40s, BFD tx/rx per RFC 5880, ...) | no `default`, so omitting the leaf yields 0 or undefined timing |

Defaults are requirements, not suggestions (`design-principles.md`): a leaf the
operator omits must still produce correct, standards-conformant behaviour. Cite
the RFC/convention the default comes from in the leaf `description`.

This supersedes the "unit suffix in the leaf name" guidance for dimensioned
values; `config-naming.md` defers to this section.

## Network Endpoints

An endpoint (a place to bind, or a remote to connect to) is ALWAYS two structured
fields, and ALWAYS comes from a shared grouping. Never a combined `"host:port"`
string, never a hand-rolled `ip`/`port` pair.

| Endpoint kind | Grouping | Fields | Port type |
|---------------|----------|--------|-----------|
| Inbound bind (the service listens) | `uses zt:listener` + `ze:listener` extension | `ip` (local literal), `port` | `zt:listener-port` (0 = OS-assigned) |
| Outbound target (the service connects out) | `uses zt:endpoint` (add to `ze-types` if absent) | `address` (IP or hostname), `port` | `zt:port` (1..65535) |

- `ip` is a local literal address (`zt:ip-address`); `address` is a remote host
  that may be a name. The two field names encode that difference on purpose --
  do not use `host`, and do not use `ip` for a remote target.
- A combined `"host:port"` / `"address:port"` string is banned (structured data,
  `derive-not-hardcode.md`). Split it into the two fields.
- `port` is never a bare `uint16` and never an inline `uint16 { range ... }`;
  use the typedef.
- Hand-model the pair only for a documented exception (BGP peer-local
  `union`-with-`auto`); see `config-design.md`.

## Boolean Toggles and Flags

An on/off setting has one shape:

```
leaf enabled { type boolean; default false; }
```

| Rule | Detail |
|------|--------|
| Positive assertion, one word | `enabled`, not `enable`, `disable`, or `disabled` (`config-naming.md`, positive-boolean). |
| Standard admin-state words are the only exception | `shutdown` (BFD, RFC 5880 §6.8.16) and interface `disable` (kernel admin-down) are allowed because they are the canonical protocol/kernel terms -- but type them as `boolean` with `default false`, never `type empty`. |
| No boolean-as-enum | Do not model a two-value on/off as `enumeration { enum enable; enum disable; }`. If config inheritance genuinely needs a distinct unset state, justify that tri-state in the module -- it is an exception, not the default. |
| Bare flag | For "this section is on when present", use a `presence` container. Do not use a `type empty` leaf. |

## Defaults and Enums

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| Boolean default is unquoted | `default false;` | `default "false";` |
| enum `value N` only for wire numbers | assign `value` when the number is protocol-significant (AFI/SAFI/ORIGIN); otherwise omit | assigning arbitrary values to cosmetic enums |

## Layout

| Rule | Detail |
|------|--------|
| Indentation | 4 spaces per level. No tabs, no 2-space modules. |
| Compact leaf | A leaf whose body is only `type` (+ optional `default` and/or `description`) MAY be one line: `leaf med { type uint32; description "..."; }`. |
| Expanded leaf | A leaf with nested constraints (`pattern`, `range`, `enumeration`, `must`, multiple sub-statements) MUST be expanded, one statement per line. |
| List key | quoted: `key "name";`. Prefer `name` for the operator-assigned key. |

## Cross-Protocol Consistency

Equivalent concepts MUST be modelled the same way across BGP, OSPF, IS-IS, BFD,
LDP, and RSVP-TE. Having configured one protocol, an operator should recognize
the next. Reuse before you redefine.

| Concept | Canonical | Do NOT |
|---------|-----------|--------|
| BFD integration | `container bfd { leaf enabled; [leaf mode;] leaf profile; }` referencing a profile in the top-level `bfd { profile <name> }` list (BGP's pattern) | redefine BFD timers inline (`min-tx`/`min-rx`/`multiplier`); every protocol that supports BFD references a profile |
| Authentication | reference a shared `key-chains` list (IS-IS's model) via a `leaf key-chain`; name the auth container the same everywhere | a per-protocol private key store; container named `md5`/`auth`/`authentication` differently per protocol; reference leaf named `key-chain` in one place and `auth-key-chain` in another |
| Per-interface protocol config | `container interfaces { list interface { key "name"; ... } }` (OSPF/IS-IS) | a bare top-level `list interface` (RSVP-TE) or a `leaf-list interfaces` when per-interface settings exist (LDP) |
| Multiplier / interval / timer names | one vocabulary for the same concept; dimensioned via a `units` statement (see Units) | four names for one concept (`detect-multiplier` vs `multiplier` for the same BFD field) |
| Toggle | positive `enabled` at every nesting level, including sub-features | `enabled` on the interface but `enable` on its sub-blocks |

Genuine RFC-term differences are the ONLY allowed divergence and MUST be
justified in the leaf/container `description`:
- Metric name: OSPF `cost` vs IS-IS `metric` (each is that protocol's RFC term).
- Router identity: `router-id` (BGP/OSPF/RSVP-TE) vs `lsr-id` (LDP) vs
  `system-id` + `net` (IS-IS).

Before adding a concept to a protocol module, grep how the sibling protocols
model it (`ai/rules/design-context.md`) and match them.

## Command Modules (naming standardization deferred)

The `-cmd` (grammar tree) and `-api` (handler) modules for operational verbs are
currently named several ways for the same verb (`ze-cli-monitor-cmd` vs
`ze-monitor-cmd` vs `ze-command-monitor-cmd`; `ze-bgp-cmd-log-api` for a non-BGP
command). Converging them on one scheme is a rename that touches `//go:embed`,
`register.go`, and YANG dispatch keys, so it is tracked separately and NOT done
piecemeal. Until then: a NEW command module SHOULD follow the majority
`ze-cli-<verb>-cmd` / paired `-api` form and MUST NOT invent a fourth scheme.
Command ownership and grammar rules live in `cli-grammar.md` and
`plugin-self-containment.md`.

## Mechanical Check

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
