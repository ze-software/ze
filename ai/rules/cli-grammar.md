# CLI Grammar: Action Before Identifier

**BLOCKING.** Every CLI command must place the action keyword before any
user-supplied identifier. This eliminates the entire class of ambiguity
where an identifier could collide with a keyword.

## The Rule

```
<verb> <noun> <action> [<identifier>] [<args>]
```

The first token after the noun (component/resource) MUST be a keyword (action),
never a user-supplied identifier (name, address, ID). Keywords are a closed set
known at compile time. Identifiers are open-ended user input.

## Correct vs Incorrect

| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show interface <name>` | `show interface detail <name>` | `<name>` could equal a keyword (`brief`, `errors`) |
| `show interface <name> counters` | `show interface counters <name>` | Action after identifier |
| `clear interface <name> counters` | `clear interface counters <name>` | Action after identifier |
| `cache <id> retain` | `cache retain <id>` | ID before action |
| `commit <name> start` | `commit start <name>` | Name before action |

## Named-Resource Commands

The "named-resource" pattern `<resource> <id> <action>` violates this rule.
Correct form: `<resource> <action> <id>`.

- `cache retain <id>`, not `cache <id> retain`
- `commit start <name>`, not `commit <name> start`

The `list` action (no identifier) already works correctly in both.

## Identifiers Are Strings

Use string-typed identifiers even when the conventional representation is numeric.
Cache IDs, VLAN IDs, session IDs: accept and store as strings. Parse to numeric
only at the point of use if the underlying API requires it. This avoids:
- Grammar ambiguity between numeric keywords and numeric IDs
- Unnecessary coupling to a representation (IDs may become non-numeric later)
- Parsing errors surfacing at the wrong layer

## Backward Compatibility

When fixing a violation, accept the old grammar with a deprecation warning.
Log the warning once per session. Remove old grammar after two release cycles.

## Mechanical Check

For every handler that dispatches on `args[0]`:

1. Is `args[0]` always a keyword from a known set? -> Correct.
2. Can `args[0]` be a user-supplied identifier? -> Violation. The handler
   must dispatch on a keyword first, then use subsequent args for identifiers.

```
grep -n 'args\[0\]' <handler-file> | grep -v 'case\|==.*"'
```

If any `args[0]` usage passes the value to a lookup/parse function (GetInterface,
ParseUint, etc.) without first matching it against a keyword set, that is a violation.

## YANG Tree

The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface detail <name>`, the
YANG tree needs a `detail` container under `interface`.

## Applies To

All CLI commands: online (RPC handlers via YANG dispatch) and offline
(`cmd/ze/` subcommand dispatch). No exceptions for "simple" commands or
"obvious" identifier positions.
