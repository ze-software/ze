# Derive Not Hardcode Rationale

Why: `.claude/rules/derive-not-hardcode.md`

## Why This Rule Exists

Ze is built around registration. Plugins, RPC handlers, address
families, CLI commands, config leaves, and inventory sections all live
in registries or maps declared once. The moment a second hardcoded
list appears (in help text, in a Meta struct, in an error message, in
docs, in a test matrix), the two copies drift. Adding a new entry
silently leaves the duplicate stale, and nobody notices until an
operator sees a wrong or incomplete list in production.

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

## Structured Data vs Pre-Formatted Strings (detail)

Ze's CLI pipe framework (`| json`, `| table`, `| yaml`) consumes the
structured `plugin.Response.Data`. If a handler pre-renders to a
string, the pipe framework cannot re-render for JSON, table, or web-UI
consumption -- the same data has to be rebuilt upstream, which is
exactly the duplication this rule forbids.

The offline `--text` mode in a CLI binary is the ONLY place where
text rendering is acceptable, and even there it must be a pure
function over the structured value, NOT a replacement for the JSON
path. The JSON path is the contract; the text path is a convenience.
