# Spec: mcp-protected-resource-metadata-path

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's MCP server publishes RFC 9728 Protected Resource Metadata at a URL built
from its resource identifier. When the identifier's path ends in a slash
(`https://host/mcp/`), Ze strips that slash too and publishes at
`/.well-known/oauth-protected-resource/mcp`, so a client that derives the
metadata URL from the identifier as RFC 9728 Section 3.1 describes asks for
`.../mcp/`, which Ze does not serve. Found by the strict re-read of
`plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` (audit verdict
`wrong`), and read at the producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9728 Section 3.1 (RFC9728-3.1-3) | "If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components." | `internal/component/mcp/streamable_auth.go::resourceOriginAndPath` (`strings.TrimRight(u.EscapedPath(), "/")`), feeding `resourceMetadataURL` and `resourceMetadataPath` | Removes EVERY trailing slash of the path, not the slash after the host: `https://host/mcp/` becomes `/mcp`, `https://host/a/?q` becomes `/a?q`. The published URL and the served path then name a resource whose spelling differs from the identifier. `TestRFC9728TerminatingSlashRemovedBeforeInsertion` (`internal/component/mcp/metadata_rfc9728_test.go`) pins the `/mcp/` to `/mcp` case | wrong |

The identity rule makes the difference observable to a client, RFC 9728
Section 3.3: "The resource value returned MUST be identical to the protected
resource's resource identifier value into which the well-known URI path suffix
was inserted to create the URL used to retrieve the metadata. If these values
are not identical, the data contained in the response MUST NOT be used."

The older sentence this one replaced reads differently, RFC 8414 Section 3.1:
"any terminating "/" MUST be removed before inserting "/.well-known/"". RFC 9728
names the slash "following the host component", which is the slash between the
host and the path, and its own example (`https://resource.example.com/resource1`)
keeps the path spelled as given.

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | identifier `https://host/mcp/` publishes and serves `/.well-known/oauth-protected-resource/mcp/` (red against HEAD) |
| Both polarities | `https://host/` and `https://host` publish at the bare suffix; `https://host/mcp` keeps `/mcp`; a query-only identifier (`https://host?x`) and `https://host/a/?q` keep their spelling |
| Identity | the `resource` member served at the derived path equals the configured identifier byte for byte |
| Test correction | `TestRFC9728TerminatingSlashRemovedBeforeInsertion` asserts the stripped form; its `/mcp/` case is corrected, not deleted |
| Discrimination | `./le rfc discriminate-record` for RFC9728-3.1-3 |
| Real entry point | an HTTP test against the running MCP listener: the `WWW-Authenticate` `resource_metadata` URL is fetched and answers 200 |

## Owner decisions

| Row | Question |
|-----|----------|
| D1 | Confirm the RFC 9728 reading (only the slash after the host goes; a path's own trailing slash stays) over the RFC 8414 reading the code follows. A deployment already configured with a trailing-slash identifier sees its metadata URL move |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9728.md`, `rfc/full/rfc9728.txt` Sections 3.1 and 3.3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/streamable_auth.go` - `resourceOriginAndPath`, `resourceMetadataURL`, `resourceMetadataPath`
- [ ] `internal/component/mcp/metadata_rfc9728_test.go` - the pinning test

**Behavior to change:** D1.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- the `resource` (or audience) identifier in the MCP OAuth config; an HTTP GET on the metadata path; a 401 carrying `resource_metadata`

### Transformation Path
1. config identifier parsed by `oauthHTTPSURL`
2. `resourceOriginAndPath` splits origin and path plus query
3. `resourceMetadataURL` advertises, `resourceMetadataPath` routes the handler

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to MCP HTTP handler | OAuthConfig | No |
| handler to client | HTTP response and `WWW-Authenticate` header | No |

### Integration Points
- the MCP HTTP mux registration of the metadata path

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| MCP listener with identifier `https://host/mcp/` | → | `resourceOriginAndPath` | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | identifier `https://host/mcp/` | metadata advertised and served at `/.well-known/oauth-protected-resource/mcp/`; `resource` equals the identifier |
| AC-2 | identifier `https://host/` or `https://host` | metadata at `/.well-known/oauth-protected-resource` |
| AC-3 | identifier `https://host/a/?q` | metadata at `/.well-known/oauth-protected-resource/a/?q` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| path spelling kept | `internal/component/mcp/metadata_rfc9728_test.go` | D1, AC-1 to AC-3 | [to fill in design] |

## Files to Modify

- `internal/component/mcp/streamable_auth.go`, `internal/component/mcp/metadata_rfc9728_test.go`, `rfc/short/rfc9728.md` (support note), `rfc/audit/rfc9728.json`, the MCP OAuth page under `docs/architecture/mcp/`

## Implementation Steps

1. Owner confirms the reading; 2. failing test; 3. fix; 4. discrimination and verdict; 5. page edit.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
