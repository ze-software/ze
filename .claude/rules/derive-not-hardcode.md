# Derive, Never Hardcode

**BLOCKING:** When enumerated data has a canonical source (a registry,
a map, a typed enum, a function that returns the list), DERIVE every
display/help/error-message/usage/doc string from that source. Never
maintain a second hardcoded copy. The first copy is the only copy.

Rationale: ze is built around registration. Plugins, RPC handlers,
address families, CLI commands, config leaves, and inventory sections
all live in registries or maps declared once. The moment a second
hardcoded list appears (in help text, in a Meta struct, in an error
message, in docs, in a test matrix) the two copies drift. Adding a new
entry silently leaves the duplicate stale, and nobody notices until an
operator sees a wrong or incomplete list in production.

## Rule

For any enumerated set of things that has a canonical source in code,
the following surfaces MUST derive from that source, not from a
hand-maintained parallel list:

| Surface | Pull from |
|---------|----------|
| `fmt.Fprintf(os.Stderr, "valid: a,b,c")` error messages | the registry's `List()` / `Keys()` / equivalent |
| `cmdregistry.Meta.Subs` / `Description` strings | derived at `init()` from the registry |
| CLI `flag.NewFlagSet.Usage` banners | derived at call time |
| Help text and `--help` output | derived |
| `.ci` test expectations that list enumerated names | write the test to pull the list, not hardcode |
| Generated docs (command catalogues, feature tables) | generated from registry via `make ze-inventory` or similar |

If the registry lookup is expensive or awkward, the fix is to add a
cheap `List()` accessor to the registry — not to paste the list a
second time.

## Pattern

Canonical source:

```
var validSections = map[string]func() (any, error){
    "cpu":     ...,
    "nic":     ...,
    ...
}

func sectionList() string {
    names := maps.Keys(validSections)
    sort.Strings(names)
    return strings.Join(names, ", ")
}
```

Derived surfaces:

| Surface | Derivation |
|---------|-----------|
| Error: unknown section | `"unknown section; valid: " + sectionList()` |
| Usage line | `"Usage: ze host show [" + strings.ReplaceAll(sectionList(), ", ", "|") + "]"` |
| Registry metadata | `cmdregistry.Meta{Subs: "show [" + ... + "]"}` at `init()` |
| Online reject response | `"valid: " + validHostSections()` |

Anti-pattern — the forbidden shape:

```
// NO — second hardcoded copy that will drift
cmdregistry.RegisterRoot("host", cmdregistry.Meta{
    Subs: "show [cpu|nic|dmi|memory|thermal|storage|kernel|all] [--text]",
})
```

## Present Structured Data, Not Pre-Formatted Strings

A corollary of the same principle, applied to output: handlers and
library functions MUST return structured values (typed structs, maps,
typed enums), never pre-formatted display strings. The display layer
owns rendering.

Ze's CLI pipe framework (`| json`, `| table`, `| yaml` and similar)
consumes the structured `plugin.Response.Data`. If a handler pre-renders
to a string, the pipe framework cannot re-render for JSON, table, or
web-UI consumption — the same data has to be rebuilt upstream, which
is exactly the duplication this rule forbids.

| Do | Don't |
|----|-------|
| Return `*CPUInfo` / `[]NICInfo` / typed struct | Return `"CPU: Intel N100, 4 cores"` pre-formatted |
| Return numeric `*-bytes` / `*-mhz` fields | Return `"8.0 GiB"` human string |
| Emit kebab-case JSON with typed fields | Emit YAML-ish text blocks from the handler |
| Let `| table` / web UI render | Render as text in the handler and lose the structure |

The offline `--text` mode in a CLI binary is the ONLY place where text
rendering is acceptable, and even there it must be a pure function over
the structured value, NOT a replacement for the JSON path. The JSON
path is the contract; the text path is a convenience.

Both rules come from the same principle: **the registry/struct is the
truth; every other surface is a view of it**.

## Mechanical Check

Before committing any change that adds an enumerated list to code or
docs, grep for duplicate copies of the entries:

```
# Replace FOO, BAR, BAZ with 2-3 entries from your new list
grep -rn "FOO\|BAR\|BAZ" .claude docs cmd internal test plan
```

Every hit is a potential duplicate that will drift. If the hit is
legitimately derived, it should reference the canonical source in a
nearby comment. If it is a hand-maintained copy, replace with a
derivation.

For output shape: before returning from a handler, ask "could the pipe
framework and the web UI both render this without re-parsing?" If the
answer requires regex/string parsing, the handler is emitting text
that should have been a typed field.

## When a Hardcoded List Is OK

- The canonical registry does not exist yet; you are creating it. Add
  the registry in the same commit as any consumer.
- A test fixture deliberately encodes the expected list as a check
  ("the sorted valid list must be exactly these eight names"). The
  test is asserting against drift, which is the whole point.
- A schema file (YANG, JSON Schema) where the enum IS the contract
  the code derives from. In that case the YANG/schema is the
  canonical source.

In every other case: derive.
