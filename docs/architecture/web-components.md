# Spec: Web Interface Component Architecture

| Field | Value |
|-------|-------|
| Status | `in-progress` |
| Depends | `-` |
| Phase | `1/1` |
| Updated | `2026-03-28` |

## Task

Replace the monolithic page-render web interface with reusable HTMX components.
Each UI element is an independent fragment with its own endpoint. Navigation
triggers partial updates via HTMX out-of-band swaps -- no full page reloads
after initial load.

## Requirements from User Feedback

| # | Requirement |
|---|------------|
| 1 | YANG schema tree must be navigable from the browser |
| 2 | List nodes (peer, group) need a panel showing existing entries + Add button to create new ones |
| 3 | Fields must use type-appropriate inputs: checkbox for bool, dropdown for enum, number for integers |
| 4 | YANG metadata (type, options, default, min, max, pattern, description) embedded as data attributes so browser handles validation and autocomplete locally |
| 5 | Breadcrumb at bottom above CLI prompt, not at top |
| 6 | CLI bar: Tab and ? trigger autocomplete, completions are clickable |
| 7 | "set ?" appends space before querying so completions show what follows the verb |
| 8 | TLS certificate persisted in zefs, not regenerated every restart |
| 9 | `ze start --web` works without BGP config (web-only mode) |
| 10 | `ze init --force` works with piped input (reads data first, prompts on /dev/tty) |
| 11 | Suppress TLS handshake error log spam from browsers rejecting self-signed certs |
| 12 | Content area left-aligned, no wasted header space |

## Component Architecture

### Fragments

| Component | ID | Endpoint | Content |
|-----------|----|----------|---------|
| Detail | `#detail` | `GET /fragment/detail?path=X` | Navigation tiles + leaf fields for current node |
| Sidebar | `#sidebar` | `GET /fragment/sidebar?path=X` | List entries + Add form (only for list nodes, hidden otherwise) |
| Breadcrumb | `#breadcrumb` | `GET /fragment/breadcrumb?path=X` | Path trail with links |

### Navigation Flow

A click on a navigation tile or sidebar entry triggers:
1. `hx-get="/fragment/detail?path=NEW"` replaces `#detail`
2. Response includes `hx-swap-oob` fragments for `#sidebar` and `#breadcrumb`

One HTTP request, three component updates.

### Field Component

A single `ze-field` div carries YANG metadata as data attributes. One shared JS
function reads the attributes and constructs the appropriate input element.

| data attribute | Purpose | Example |
|---------------|---------|---------|
| `data-type` | YANG value type | `bool`, `string`, `enum`, `uint16`, `ip`, `prefix`, `duration` |
| `data-options` | Enum values (comma-separated) | `igp,egp,incomplete` |
| `data-default` | YANG default value | `igp` |
| `data-min` | Numeric minimum | `0` |
| `data-max` | Numeric maximum | `65535` |
| `data-pattern` | Validation regex | `^(\d{1,3}\.){3}\d{1,3}$` |
| `data-description` | YANG description (tooltip) | `BGP router identifier` |
| `data-path` | YANG path for POST | `bgp` |
| `data-leaf` | Leaf name for POST | `router-id` |
| `data-value` | Current configured value | `1.2.3.4` |

Browser JS renders:
- `bool` -> toggle/checkbox
- `enum` -> `<select>` with options
- `uint16`/`uint32`/`int` -> `<input type="number">` with min/max
- `ip`/`prefix` -> `<input type="text">` with pattern
- `string`/`duration` -> `<input type="text">`

Set button POSTs to `/config/set/<data-path>` with `leaf=<data-leaf>&value=<input-value>`.

### Layout

```
+------------------------------------------+
| #detail                                  |
|  [bgp]  [system]  [plugin]              |
|  router-id: [___________] [Set]         |
|  listen:    [___________] [Set]         |
+------------------------------------------+
| #sidebar  (list entries, only for lists) |
|  [1.2.3.4] [10.0.0.1]  [+ Add: ____]  |
+------------------------------------------+
| #breadcrumb  / > bgp > peer > 1.2.3.4  |
+------------------------------------------+
| /> set bgp peer 1.2.3.4 remote-as 65001 |
+------------------------------------------+
```

### Server Handler

One handler serves all fragments. Query params: `path`, `fragment` (optional).

- No `fragment` param -> full page (layout + all fragments)
- `fragment=detail` -> detail HTML only
- Response always includes OOB swaps for breadcrumb + sidebar

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/web/fragment.go` | New: fragment handlers |
| `internal/component/web/templates/layout.html` | Component slots with IDs |
| `internal/component/web/templates/detail.html` | New: detail fragment |
| `internal/component/web/templates/sidebar.html` | New: sidebar fragment |
| `internal/component/web/templates/breadcrumb.html` | Rewrite: standalone fragment |
| `internal/component/web/templates/field.html` | New: single field with data attributes |
| `internal/component/web/assets/style.css` | Component styles |
| `internal/component/web/assets/field.js` | New: field renderer + inline validation |
| `internal/component/web/handler_config.go` | Adapt to use fragments |
| `internal/component/web/handler_config_walk.go` | Add FieldMeta to carry YANG metadata |
| `internal/component/web/render.go` | Fragment rendering helpers |
| `cmd/ze/hub/main.go` | Wire fragment endpoint |
