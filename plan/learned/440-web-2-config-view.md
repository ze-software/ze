# 440 -- Config View (Read-Only Navigation)

## Context

With the web server foundation in place, the next step was rendering the YANG schema tree as navigable HTML. Each YANG node kind (container, list, leaf, flex, freeform, inline list) needed its own template. The CLI already walks the schema via `schemaGetter` -- the web handler follows the same pattern with type assertions.

## Decisions

- Implemented own `walkConfigPath` in web package over importing CLI editor's private `walkPath` -- avoids coupling to CLI internals while following the same pattern.
- One template per node kind over a single template with conditionals -- single concern per file.
- `FreeformNode` treated as terminal (no drill-down) -- it doesn't implement `schemaGetter`.
- Enum detection via `goyang.Entry.Type` over `ValueType` -- ValueType maps enums to TypeString, losing the enum values.
- All 10 ValueTypes mapped to HTML input types -- no "unknown type" fallback that silently renders wrong.

## Consequences

- Leaf subtypes (`MultiLeafNode`, `BracketLeafListNode`, `ValueOrArrayNode`) all return `NodeLeaf` from `Kind()` -- distinguished by concrete type assertion in the template data assembly.
- URL-encoded list keys (e.g., `%2F` for `/` in prefix keys) required -- standard URL encoding, decoded before schema walking.
- Breadcrumb navigation syncs with CLI `contextPath` concept -- clicking a breadcrumb segment is equivalent to typing `edit <path>`.

## Gotchas

- List keys consume 2 path segments during schema walk (list name + key value) -- the web handler must mirror this exactly or navigation breaks.
- `FreeformNode` has no `Get()` method -- attempting to walk into it panics. Must check `Kind()` before casting.
- HTMX partial swap requires detecting `HX-Request` header -- full page vs fragment response.

## Files

- `internal/component/web/handler_config.go` (schema walk, template data assembly)
- `internal/component/web/templates/container.html`, `list.html`, `leaf_input.html`, `flex.html`, `freeform.html`, `inline_list.html`, `breadcrumb.html`
