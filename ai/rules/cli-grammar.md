# CLI Grammar: Keywords Before Values

**BLOCKING.** Every CLI command must place a closed keyword before any
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

## YANG Tree

The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface name <name> detail`,
the YANG tree needs a `name` container under `interface`, then `detail`
under that selector. Filter forms like `show interface type <type>` need a
`type` container that consumes the typed selector value.

## Applies To

All CLI commands: online (RPC handlers via YANG dispatch) and offline
(`cmd/ze/` subcommand dispatch). No exceptions for "simple" commands or
"obvious" identifier positions.
