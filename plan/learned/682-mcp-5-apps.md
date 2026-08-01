# 682 -- MCP 5 Apps (UI Resources)

## Context

MCP Apps (2026-01-26 extension) lets the server advertise UI resources
alongside tools: tool descriptors carry `_meta.ui.resourceUri` pointing
to embedded HTML/CSS/JS assets served via `resources/list` and
`resources/read`. Clients render the HTML in a sandboxed iframe; the
server is purely a static asset server. This was Phase 5 of the MCP
modernization umbrella (spec-mcp-0-umbrella).

## Decisions

- **Embedded FS only.** UI bundles ship via `//go:embed` in
  `internal/component/mcp/ui/embed.go`. No on-disk loading path.
  Single-binary deployment model; operators get new UI by deploying a
  new daemon.

- **Capability gate on read, not on advertisement.** `_meta.ui.*` is
  emitted in `tools/list` unconditionally so clients discover UI-capable
  tools without a round-trip. `resources/list` and `resources/read` are
  gated on `capabilities.resources = {}` declared at initialize.

- **MIME sniffing is extension-based, not content-based.** Predictable,
  fast, no dependency on `http.DetectContentType`. Unknown extensions
  default to `application/octet-stream`.

- **YANG extensions for UI metadata.** `ze:ui-resource` (path),
  `ze:ui-permissions` (capabilities), `ze:ui-csp` (Content-Security-Policy)
  on command-tree grouping containers. Follows the same
  extension-on-container pattern as `ze:task-support`. `PathToUIResource`
  walks -cmd modules; hub wiring propagates to `CommandInfo.UIResource`;
  `buildToolDef` emits `_meta.ui` when non-nil.

- **First UI bundle: bgp-peer.** Peer status panel with state, ASN, and
  name. Uses safe DOM methods (no innerHTML). Tagged on the `peer`
  container in `ze-peer-cmd.yang`.

## What Worked

- The `ze:task-support` extension pattern (YANG -> command.go walker ->
  hub map -> CommandInfo field -> buildToolDef emission) transferred
  directly to UI resources. Three new YANG extensions, one new walker,
  one new field, one enrichment block.

- Path-traversal prevention is simple with `path.Clean` + prefix check
  before any `fs.ReadFile`. The embedded FS adds a second layer
  (rejects `..` outside its root).

- The `groupUIResource` function inherits the UI annotation from any
  action in the group, so tagging the parent container (not individual
  commands) works correctly with the existing grouping logic.

## What to Watch

- `resources/updated` notifications are deferred. UI bundles are
  immutable at runtime so there is no notification trigger today.

- `resources/list` pagination is not implemented. The embedded FS is
  small enough that the full list fits comfortably in one response.

- The `lookupUIResource` function in hub walks parent paths to find the
  annotation. If a deeply nested command should NOT inherit the UI
  resource from its parent, an override mechanism would be needed.

## Files

| File | Change |
|------|--------|
| `internal/component/mcp/resources.go` | New: MIME sniffer, URI validator, `listResources`, `readResource`, `resourcesList`/`resourcesRead` Streamable methods |
| `internal/component/mcp/resources_test.go` | New: 16 tests covering list, read, traversal, capability gate, binary blob |
| `internal/component/mcp/ui/embed.go` | New: `//go:embed bgp-peer` exposing `embed.FS` |
| `internal/component/mcp/ui/bgp-peer/` | New: `index.html`, `style.css`, `app.js`, `icon.png` |
| `internal/component/mcp/session.go` | `clientResources` bit, `ClientSupportsResources()`, `CreateWithCapabilities` 5th arg |
| `internal/component/mcp/streamable.go` | `resources/list`/`resources/read` dispatch, `parseResourcesCapability`, `resources: {}` in initialize |
| `internal/component/mcp/tools.go` | `UIResourceInfo`, `uiResource` on action/toolGroup, `groupUIResource`, `_meta.ui` in `buildToolDef` |
| `internal/component/mcp/tools_test.go` | 3 tests for `_meta.ui` enrichment |
| `internal/component/config/yang/command.go` | `UIResourceInfo`, `PathToUIResource`, `getUIResourceExtensions` |
| `internal/component/config/yang/modules/ze-extensions.yang` | `ze:ui-resource`, `ze:ui-permissions`, `ze:ui-csp` extensions |
| `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` | Tagged `peer` container with UI extensions |
| `cmd/ze/hub/main.go` | `uiResourceByPath` in `serverCommandLister`, `lookupUIResource` call |
| `cmd/ze/hub/service_mcp.go` | `lookupUIResource` function |
| `docs/architecture/mcp/overview.md` | Resources Capability section, files table, capability table, roadmap |
| `docs/architecture/api/commands.md` | `resources/list`, `resources/read` method entries |
| `docs/features.md` | MCP row updated with Apps |
| `docs/comparison.md` | MCP Apps row added |
