# Spec: mcp-5-apps -- UI Resources and MCP Apps

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-mcp-0-umbrella.md` -- umbrella; Phase 5 owns AC-22..AC-23
4. `plan/learned/636-mcp-1-streamable-http.md` -- P1 (transport, methods dispatch)
5. `plan/learned/638-mcp-2-remote-oauth.md` -- P2 (Identity, auth for UI resources served to browsers)
6. `plan/learned/640-mcp-3-elicitation.md` -- P3 (capability negotiation pattern, reused here)
7. `plan/spec-mcp-4-tasks.md` -- P4 (related-task metadata pattern; NOT a hard dependency)
8. `internal/component/mcp/{streamable,handler,tools,session}.go` -- current impl
9. `docs/architecture/mcp/overview.md` -- transport shape, capability negotiation, roadmap
10. `modelcontextprotocol.io/extensions/apps/overview` -- authoritative

## Task

Deliver the MCP Apps extension (2026-01-26): the server advertises a
`resources` capability, tools that ship a UI bundle reference it via
`_meta.ui.resourceUri` with the `ui://` URI scheme, and the client
fetches the UI assets through `resources/list` + `resources/read`. The
HTML runs in a sandboxed iframe on the client side; ze only serves the
static assets. Every tool that currently answers a Claude-style chat
query (e.g. `show bgp peer`, `rib dump`) can attach a rich HTML view
that renders the same data as a panel inside the client UI.

Scope: AC-22..AC-23 from the umbrella. Adds the first consumer of the
MCP `resources` capability to ze. No new protocol wire features; the
work is bundling, serving, and metadata plumbing.

This phase is INDEPENDENT of Phase 4. It can land before, after, or
alongside tasks. The only cross-reference is shared capability-
negotiation plumbing in `doInitialize`.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/mcp/overview.md` -- Transport Shape, Capability Negotiation, Roadmap
  -> Constraint: `resources` goes in the server's capabilities block at initialize; the client's `resources` support is also declared there
  -> Decision: UI bundles are embedded at compile time via `//go:embed`; no on-disk lookup at runtime
- [ ] `docs/architecture/api/commands.md` -- MCP Server-Initiated Methods section
  -> Constraint: `resources/*` is a NEW method group; add a section mirroring the tasks/* layout from Phase 4 (if it landed) or the Server-Initiated Methods section
- [ ] `plan/learned/638-mcp-2-remote-oauth.md` -- Identity + auth dispatcher
  -> Constraint: `resources/read` MUST honor the same auth-mode chain as `tools/call`; UI resources are NOT public assets
- [ ] `internal/component/mcp/tools.go` -- tool descriptor builder
  -> Constraint: `_meta.ui.resourceUri` attaches to the per-tool descriptor at `tools/list` time, derived from a YANG extension (`ze:ui-resource`) so no hardcoded mapping drifts
- [ ] `internal/component/mcp/session.go` -- session capability bits
  -> Constraint: add `clientResources bool` set from `capabilities.resources = {}` at initialize; server honors `resources/*` only if declared

### MCP Spec Pages (external)

- [ ] `modelcontextprotocol.io/extensions/apps/overview`
  -> Constraint: server declares `resources` capability; tool descriptors MAY carry `_meta.ui.resourceUri` (string, `ui://...`); the client resolves that URI via `resources/read` and renders the returned content in a sandboxed iframe
  -> Constraint: `_meta.ui` MAY also carry `permissions` (array of capability strings, e.g. `["network"]`) and `csp` (Content-Security-Policy string); servers SHOULD include both
  -> Constraint: UI resource content-type is `text/html` for the root document; fragment resources (CSS, JS, images) are served via `resources/read` with their own MIME types
  -> Constraint: App ↔ host conversation rides over `postMessage` between iframe and host; the MCP server does NOT handle postMessage. The server is purely a static asset server for the HTML and its JS/CSS
- [ ] `modelcontextprotocol.io/specification/2025-06-18/server/resources`
  -> Constraint: `resources/list` returns a paged list of `{uri, name, description, mimeType}`; `resources/read` returns `{contents: [{uri, mimeType, text|blob}]}`
  -> Constraint: URI MUST be a valid URI; multiple schemes permitted (`file://`, `https://`, custom like `ui://`)
  -> Constraint: resources MAY be updated; the server MAY send `notifications/resources/updated` on the GET SSE stream (out of scope this phase; UI bundles are immutable at runtime)

### Ze Rules

- [ ] `.claude/rules/derive-not-hardcode.md`
  -> Constraint: list of UI resources derived from the embedded `fs.FS`; no hardcoded URI list
- [ ] `.claude/rules/json-format.md`
  -> Constraint: MCP-dialect exemption applies -- resource content is returned under camelCase `mimeType` and `blob` keys per external spec
- [ ] `.claude/rules/goroutine-lifecycle.md`
  -> Constraint: serving a resource is synchronous under the POST handler; no new goroutines

**Key insights:**
- This phase is narrow. No new protocol transport; no state machine. Deliverable is: embedded assets + two methods + one YANG extension + one capability bit.
- Security surface is content-delivery (correct MIME types, CSP header in `_meta.ui`, no directory traversal out of the embedded FS). Auth is inherited from Phase 2.
- First UI bundle: pick ONE tool with clear panel value. Recommend `show bgp peer` (per-peer status, paths, communities) because it has the richest structured data. Second candidate: `rib dump` with a filter input form.
- `_meta.ui` is extensible; future phases could add `_meta.ui.language` (dark/light theme), `_meta.ui.windowHints`, etc. This phase ships the minimal shape.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE finalizing this spec)

- [ ] `internal/component/mcp/streamable.go` -- method dispatch switch in `runMethod`; `doInitialize` negotiates capabilities
- [ ] `internal/component/mcp/tools.go` -- tool-descriptor builder; `tools/list` output shape
- [ ] `internal/component/mcp/handler.go` -- legacy 2024-11-05 path (may be deleted in Phase 4 first)
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` -- existing `ze:command` / `ze:sensitive` extensions

**Behavior to preserve:**
- `tools/list` output shape (new fields are ADDED; existing fields unchanged)
- `initialize` capability block (add `resources: {}`; do not remove anything)
- `ze_execute` handcrafted tool (no UI bundle; remains a pure dispatch tool)
- Per-session capability bits stored on session (add `clientResources`)

**Behavior to change:**
- Server advertises `resources` capability in `initialize` result
- Server accepts `resources/list` and `resources/read` method dispatches
- Tool descriptors gain `_meta.ui.resourceUri` + `_meta.ui.permissions` + `_meta.ui.csp` for tools tagged with a UI resource in YANG

## Data Flow (MANDATORY)

### Entry Point

`POST /mcp` with JSON-RPC body:
- `method: "resources/list"` -> walk embedded FS, emit descriptor list
- `method: "resources/read"` -> resolve `ui://<name>` to embedded asset, emit content

Both paths share method dispatch with `tools/*` -- same auth + session + content-type gates.

### Transformation Path

1. `handlePOST` receives JSON-RPC; session + auth check; method dispatch routes into `Streamable.listResources` or `Streamable.readResource`.
2. `listResources` walks `embeddedUIFS` via `fs.WalkDir`; for each asset it emits a descriptor `{uri: "ui://<rel-path>", name: <rel-path>, description: <derived>, mimeType: <sniffed>}`.
3. `readResource` parses the `uri` param, validates the `ui://` scheme, resolves the remaining path against the embedded FS, reads the bytes, returns `{contents: [{uri, mimeType, text|blob}]}` where the text/blob choice depends on MIME type.
4. `tools/list` builder (`tools.go`) enriches each tool descriptor with `_meta.ui` when the tool's YANG command has a `ze:ui-resource` extension. The extension value is the root URI (e.g. `ui://bgp-peer/index.html`); permissions and csp come from extension sub-leaves.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| POST JSON-RPC -> resource handler | Method dispatch | [ ] |
| Auth -> resource access | Same dispatcher chain as `tools/call` | [ ] |
| Embedded FS -> HTTP response body | `fs.ReadFile` + MIME-type sniff | [ ] |
| Tool descriptor -> UI bundle reference | YANG extension -> tools.go enrichment | [ ] |

### Integration Points

- `//go:embed internal/component/mcp/ui` in a new `internal/component/mcp/ui/embed.go` -- ships the HTML/CSS/JS bundles with the binary
- `CommandLister` + new `ze:ui-resource` YANG extension -- drives per-tool UI advertisement
- `session.ClientSupportsResources()` -- gate resources methods on client capability

### Architectural Verification

- [ ] No bypassed layers: resources/* goes through session + auth + identity scope
- [ ] No unintended coupling: UI bundles live in `internal/component/mcp/ui/`; not shared with `docs/`, not imported by plugins
- [ ] No duplicated functionality: uses existing method dispatch; no new SSE path; no new worker goroutine
- [ ] No path traversal out of embedded FS (mechanical: `fs.ReadFile(embeddedUIFS, cleanPath)` rejects `..` outside root)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `tools/list` after client declared `capabilities.resources` | -> | Tool with `ze:ui-resource bgp-peer/index.html` returns descriptor carrying `_meta.ui.resourceUri = "ui://bgp-peer/index.html"` | `test/plugin/apps-tools-list-ui-meta.ci` |
| `resources/list` after capability declared | -> | Returns descriptors for every asset under `internal/component/mcp/ui/` with proper MIME types | `test/plugin/apps-resources-list.ci` |
| `resources/read` with `ui://bgp-peer/index.html` | -> | Returns HTML body + `mimeType=text/html`; response body matches the embedded file byte-for-byte | `test/plugin/apps-resources-read-html.ci` |
| `resources/read` with `ui://bgp-peer/style.css` | -> | Returns CSS body + `mimeType=text/css` | `test/plugin/apps-resources-read-css.ci` |
| `resources/read` with a traversal attempt like `ui://../../etc/passwd` | -> | `-32602 invalid uri` before any FS read | `test/plugin/apps-resources-traversal.ci` |
| `resources/read` without client declaring `capabilities.resources` | -> | `-32601 method not found` (server hides the capability entirely) | `test/plugin/apps-resources-no-capability.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-22 | `tools/list` includes a tool with `_meta.ui.resourceUri: "ui://bgp-peer/index.html"` | Client can `resources/read` that URI and receive HTML content; the response body bytes equal the embedded file |
| AC-22a | Tool's `_meta.ui` also carries `permissions: ["network"]` and `csp: "default-src 'self'; ..."` | Both fields appear in the `tools/list` output; values match the YANG `ze:ui-resource` sub-leaves |
| AC-22b | `tools/list` without the client declaring `capabilities.resources` | `_meta.ui` fields ARE STILL emitted (so the client can advertise support in a later session); but `resources/read` fails with `method not found` for that session |
| AC-23 | UI resource is served with declared `csp` in `_meta.ui.csp` | Resource content and metadata reach the client; UI metadata payload is well-formed (passes JSON Schema validation against the 2026-01-26 Apps schema) |
| AC-23a | `resources/list` during a capability-declared session | Returns all UI assets under `internal/component/mcp/ui/`; derived from a live `fs.WalkDir`, not a hardcoded list |
| AC-23b | `resources/read` with a URI referring to a file that does not exist | `-32602 resource not found`; no disk leak (never mentions filesystem paths) |
| AC-23c | `resources/read` with a malformed URI (missing `ui://` scheme) | `-32602 invalid uri`; error message does not echo the raw input |
| AC-23d | `resources/read` when the content is binary (e.g. `.png`, `.svg` with embedded binary) | Response uses the `blob` field with base64-encoded body; `text` field absent |

## TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestResources_ListWalksEmbeddedFS` | `internal/component/mcp/resources_test.go` | `resources/list` output matches embedded FS contents |
| `TestResources_ReadHTML` | same | AC-22 read returns exact bytes + correct mimeType |
| `TestResources_ReadCSS` | same | CSS content-type sniff |
| `TestResources_ReadBinaryUsesBlob` | same | AC-23d binary file -> base64 blob, no text field |
| `TestResources_ReadNotFound` | same | AC-23b missing file -> typed error |
| `TestResources_ReadInvalidURI` | same | AC-23c malformed uri -> typed error |
| `TestResources_ReadTraversalRejected` | same | `ui://../etc/passwd` -> invalid uri; never touches real FS |
| `TestResources_ReadNoCapability` | same | AC-22b client without resources cap -> method not found |
| `TestToolDescriptor_UIMetaFromYANG` | `internal/component/mcp/tools_test.go` | `_meta.ui.resourceUri` derived from YANG `ze:ui-resource` extension |
| `TestToolDescriptor_UIMetaPermissionsAndCSP` | same | `permissions` and `csp` fields populated from YANG sub-leaves |
| `TestInitialize_ServerAdvertisesResources` | `internal/component/mcp/streamable_test.go` | Initialize result includes `resources: {}` in the server capabilities |
| `TestInitialize_ClientCapabilityResources` | same | `capabilities.resources = {}` from client flips `session.clientResources = true` |
| `TestMimeType_Sniffing` | `internal/component/mcp/resources_mime_test.go` | `.html -> text/html`, `.css -> text/css`, `.js -> application/javascript`, `.svg -> image/svg+xml`, unknown -> `application/octet-stream` |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| UI resource size (bytes, per asset) | 1 -- 1 048 576 (1 MiB embedded per file cap) | 1 048 576 | 0 (empty file rejected) | 1 048 577 |
| `ui://` path depth | 1 -- 8 segments (bounded by embedded FS depth) | 8 | 0 (bare `ui://`) | 9 |
| URI length | 22 -- 2048 chars | 2048 | 7 (`ui://` alone is 5) | 2049 |

### Functional Tests

| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `apps-tools-list-ui-meta` | `test/plugin/apps-tools-list-ui-meta.ci` | `tools/list` emits `_meta.ui.resourceUri` for the tagged tool |
| `apps-resources-list` | `test/plugin/apps-resources-list.ci` | `resources/list` returns all UI assets |
| `apps-resources-read-html` | `test/plugin/apps-resources-read-html.ci` | `resources/read` on `ui://bgp-peer/index.html` returns the embedded HTML |
| `apps-resources-read-css` | `test/plugin/apps-resources-read-css.ci` | same for CSS; mimeType checked |
| `apps-resources-traversal` | `test/plugin/apps-resources-traversal.ci` | Path-traversal attempt rejected with invalid uri |
| `apps-resources-no-capability` | `test/plugin/apps-resources-no-capability.ci` | Client without capability cannot call `resources/*` |

## Files to Modify

- `internal/component/mcp/streamable.go` -- add `resources/list` and `resources/read` method dispatch; extend `doInitialize` with `parseResourcesCapability` helper; advertise `resources: {}` in the initialize result
- `internal/component/mcp/session.go` -- add `clientResources bool`; `ClientSupportsResources()` accessor
- `internal/component/mcp/tools.go` -- enrich tool descriptors with `_meta.ui.*` when YANG `ze:ui-resource` extension is present
- `internal/component/config/yang/modules/ze-extensions.yang` -- new `ze:ui-resource`, `ze:ui-permissions`, `ze:ui-csp` extensions
- `internal/component/config/yang/modules/<command-specific>.yang` -- tag at least one existing command (candidate: `bgp peer show`) with the new extensions
- `docs/architecture/mcp/overview.md` -- add Resources Capability subsection; new transport shape row; files table entry for `resources.go` + `ui/` embed
- `docs/architecture/api/commands.md` -- add `resources/list`/`resources/read` method entries
- `docs/guide/mcp/overview.md` -- client-facing resources capability advert
- `docs/functional-tests.md` -- `apps-*.ci` section

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extensions + tagged commands) | Yes | `ze-extensions.yang` + a command-specific YANG file |
| Env vars | No -- UI bundles are compile-time embedded | -- |
| CLI commands/flags | No | -- |
| Editor autocomplete | Yes -- YANG-driven | automatic once YANG updated |
| Functional test per capability | Yes -- 6 `.ci` scenarios above | `test/plugin/apps-*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- MCP Apps (UI resources) row |
| 2 | Config syntax changed? | No | -- |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- resources/* methods |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/apps.md` (new) |
| 7 | Wire format changed? | No (MCP is not BGP wire) | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No -- MCP spec is the reference | -- |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- `apps-*.ci` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- MCP Apps row (ze-only differentiator) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md` -- Resources Capability subsection |

## Files to Create

- `internal/component/mcp/resources.go` -- `resources/list`, `resources/read`, MIME sniffer, `ui://` URI resolver
- `internal/component/mcp/resources_test.go` -- unit tests listed above
- `internal/component/mcp/resources_mime_test.go` -- MIME sniffer tests (table-driven)
- `internal/component/mcp/ui/embed.go` -- `//go:embed` directive + `embeddedUIFS fs.FS`
- `internal/component/mcp/ui/bgp-peer/index.html` -- reference UI for `bgp peer show`
- `internal/component/mcp/ui/bgp-peer/style.css`
- `internal/component/mcp/ui/bgp-peer/app.js`
- `test/plugin/apps-tools-list-ui-meta.ci`, `apps-resources-list.ci`, `apps-resources-read-{html,css}.ci`, `apps-resources-traversal.ci`, `apps-resources-no-capability.ci`
- `docs/guide/mcp/apps.md` -- user guide

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Per finding |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1 -- Capability negotiation + server advert**
   - Tests: `TestInitialize_ServerAdvertisesResources`, `TestInitialize_ClientCapabilityResources`
   - Files: `session.go` (new bit), `streamable.go` (`parseResourcesCapability`, initialize advert)
   - Verify: unit tests pass; `resources/*` routed to a stub returning `method not found` pre-capability
2. **Phase 2 -- MIME sniffer + embedded FS scaffold**
   - Tests: `TestMimeType_Sniffing` (table-driven)
   - Files: `resources_mime_test.go`, `resources.go` (mime helper only), `ui/embed.go` (empty placeholder file)
   - Verify: MIME table passes
3. **Phase 3 -- `resources/list` + `resources/read`**
   - Tests: `TestResources_ListWalksEmbeddedFS`, `TestResources_ReadHTML`, `TestResources_ReadCSS`, `TestResources_ReadNotFound`, `TestResources_ReadInvalidURI`, `TestResources_ReadTraversalRejected`, `TestResources_ReadBinaryUsesBlob`, `TestResources_ReadNoCapability`
   - Files: `resources.go` full implementation
   - Verify: every AC test green
4. **Phase 4 -- YANG extensions + `_meta.ui.*` enrichment**
   - Tests: `TestToolDescriptor_UIMetaFromYANG`, `TestToolDescriptor_UIMetaPermissionsAndCSP`
   - Files: `ze-extensions.yang`, `tools.go`
   - Verify: `tools/list` shows `_meta.ui.*` for tagged tool
5. **Phase 5 -- First UI bundle (bgp-peer)**
   - Files: `ui/bgp-peer/{index.html,style.css,app.js}`; tag `bgp peer show` with `ze:ui-resource`
   - Verify: manual `curl` round-trip returns the HTML; browser (via a compatible MCP client) renders the panel
6. **Phase 6 -- Functional `.ci` tests**
   - Files: `test/plugin/apps-*.ci`
   - Verify: `bin/ze-test plugin -p apps` all pass
7. **Phase 7 -- Docs**
   - Per Documentation Update Checklist
8. **Phase 8 -- Full verify + learned summary + two-commit sequence per spec-preservation**

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | Every AC has a named test + file:line |
| Correctness | `resources/read` returns byte-exact embedded content; MIME types match the extension sniffer; traversal attempts rejected BEFORE any `fs.ReadFile` call |
| Naming | MCP dialect keeps camelCase: `mimeType`, `resourceUri`; Ze-internal types use Go camelcase; URIs use the `ui://` scheme |
| Data flow | `resources/read` path: POST -> session check -> auth -> capability gate -> URI validate -> FS read -> encode -> response. No alternate path |
| Rule: no-layering | UI bundles shipped via `//go:embed` only; no on-disk loading path |
| Rule: derive-not-hardcode | Resource list derived from `fs.WalkDir`; per-tool UI advertisement derived from YANG |
| Rule: exact-or-reject | Unknown `ui://` scheme variants (`UI://`, `ui:/file`) rejected with invalid uri, not silently normalised |
| Rule: integration-completeness | At least one real tool tagged with `ze:ui-resource`; `.ci` test proves `tools/list` -> `_meta.ui` -> `resources/read` end-to-end |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `resources/list` implemented | `grep 'case "resources/list"' internal/component/mcp/streamable.go` |
| `resources/read` implemented | `grep 'case "resources/read"' internal/component/mcp/streamable.go` |
| Embedded FS carries at least one UI bundle | `ls internal/component/mcp/ui/bgp-peer/index.html` |
| Server advertises `resources` capability | Initialize response JSON contains `"resources":{}` |
| At least one YANG command tagged `ze:ui-resource` | `grep 'ze:ui-resource' internal/component/config/yang/modules/*.yang` |
| `_meta.ui.*` appears in `tools/list` | Manual `ze-test mcp @tools/list` inspection |
| `.ci` tests pass | `bin/ze-test plugin -p apps` |
| `make ze-verify-fast` passes | tmp/ze-verify.log shows PASS |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Path traversal | `ui://../etc/passwd` rejected BEFORE any FS read; `fs.ReadFile` is invoked only on paths within `embeddedUIFS` root. `path.Clean` + prefix check are the gate |
| URI scheme validation | Only `ui://` accepted; `file://`, `https://`, bare paths rejected |
| MIME type mismatch | Extension-based sniff, not content-based; predictable; `unknown -> application/octet-stream` (never HTML so a malicious upload can't XSS -- though we ship only known files, defense-in-depth) |
| Auth bypass | `resources/*` goes through the SAME auth dispatcher as `tools/*`; no anonymous path |
| Capability gate | Server returns `method not found` for `resources/*` when client didn't declare capability -- NOT `unauthorized` -- so the capability's presence isn't fingerprintable via error distinguishing |
| Resource exhaustion | Per-asset size capped at build time (CI check: embedded files >1 MiB fail `make ze-lint` or a dedicated check); `resources/list` output bounded by number of embedded files (compile-time) |
| CSP enforcement | `_meta.ui.csp` is served as METADATA; clients enforce it in the iframe; the server doesn't serve an HTTP CSP header (the assets go through MCP, not a bare HTTP endpoint) |
| XSS via UI bundle | Bundles are hand-authored and code-reviewed; CI rejects pull requests that touch `ui/` without a human signoff (new `.github/CODEOWNERS` entry or equivalent). UI code SHOULD NOT eval user input; enforcement is review-gated |
| Secrets in UI bundle | Bundles must not embed secrets; CI grep on `ui/` for common secret patterns (future: optional; not blocking Phase 5) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Phase that introduced it |
| `//go:embed` pattern doesn't match | Check `ui/embed.go` path relative to package; embed paths are package-relative |
| Tool descriptor missing `_meta.ui.*` | Re-check YANG extension parser in `tools.go` |
| MIME sniff wrong | Extend the table; add the extension |
| Traversal test fails | Re-check `path.Clean` + root-prefix check order |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Serve UI bundles via a separate HTTP listener (like the web component) | Inconsistent with MCP's single-endpoint model; would bypass auth | Serve through `resources/read` on the same MCP endpoint |
| Load UI from disk at runtime | Violates single-binary deployment; adds FS-layout concerns | `//go:embed` at build time |
| Render UI server-side into HTML strings | MCP Apps is explicitly client-side rendering in an iframe | Static assets only; client does postMessage to host |
| Require `capabilities.resources` as a hard gate for `_meta.ui.*` at `tools/list` | Makes discovery awkward (client needs a round-trip to see which tools have UI); capability only gates `resources/read` | Emit `_meta.ui.*` unconditionally; gate only the read |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- **Apps is the smallest-surface phase in the MCP roadmap.** No state machine, no new goroutines, no correlation logic. Just two methods + embed + metadata. The complexity budget is spent on the UI bundles themselves (HTML/JS/CSS), which is out of scope for the protocol spec.
- **Embedded vs on-disk:** `//go:embed` locks UI into the binary. This is deliberate -- ze's deployment model is single-binary; operators deploy a new daemon when they want a new UI. Alternative (on-disk UI dir) introduces a file-layout config surface and a runtime search path; rejected.
- **Metadata advertisement unconditional, read gated:** `tools/list` emits `_meta.ui.*` even for clients that didn't declare `capabilities.resources`. This lets the client SEE there's a UI but know it can't fetch without declaring support. Symmetric with how `resources/updated` notifications work in the base spec.
- **`_meta.ui` shape exactly mirrors the MCP Apps extension schema.** Ze is a consumer of the spec, not an extender; adding Ze-specific fields inside `_meta.ui.*` is forbidden. Ze-specific metadata (if any ever lands) goes under `_meta.io.ze/*`.
- **First UI bundle target: `bgp peer show`.** The command returns structured data (peer state, timers, capabilities, sent/received counters) that benefits from a panel view. Rejected alternatives: `rib dump` (huge output, needs pagination before a panel makes sense), `monitor bgp` (already a TUI; a panel duplicates it), `config show` (too static; little UX win over text).
- **MIME sniffer is extension-based, not content-based.** Predictable, fast, no dependency on `http.DetectContentType`'s magic-number table. Binary blobs default to `application/octet-stream` -- safer than guessing wrong.
- **Size cap at 1 MiB per asset:** matches `session.maxBody`. Larger assets would exceed the JSON-RPC response body cap for `resources/read` anyway. Document the limit.

## RFC Documentation

MCP Apps (2026-01-26) and MCP 2025-06-18 resources are the authoritative specs. Inline comments at `resources/list`, `resources/read`, the MIME sniffer, and the `_meta.ui.*` builder cite:

- `modelcontextprotocol.io/extensions/apps/overview` -- UI resource conventions
- `modelcontextprotocol.io/specification/2025-06-18/server/resources` -- base resources capability

## Implementation Summary

_Filled after implementation per `/implement` stage 13._

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-22..AC-23d all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] At least one YANG command tagged `ze:ui-resource`; end-to-end `.ci` proves the chain
- [ ] One UI bundle ships in the binary

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (resources handler concrete, not a pluggable content backend)
- [ ] No speculative features (resources/updated notifications deferred; on-disk UI deferred)
- [ ] Single responsibility per new file (resources.go owns list+read; resources_mime.go owns sniffer; ui/embed.go owns embed directive)
- [ ] Minimal coupling (UI bundles live under internal/component/mcp/ui/; no plugin imports)

### TDD
- [ ] Tests written before code in each phase
- [ ] Tests FAIL (paste output per phase)
- [ ] Tests PASS (paste output per phase)
- [ ] Boundary tests for asset size, path depth, URI length
- [ ] Functional `.ci` tests for all 6 wiring rows

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-mcp-5-apps.md`
