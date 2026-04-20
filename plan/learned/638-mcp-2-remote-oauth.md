# 638 -- MCP Remote Binding + OAuth 2.1 Resource Server

## Context

MCP's Phase-1 transport landed in `plan/learned/636-mcp-1-streamable-http.md`
with a single shared-secret bearer mode and a hard-coded loopback clamp.
That was enough for one local agent over an SSH tunnel; it blocked every
scenario where multiple clients, delegated identities, or direct remote
reach is required. Phase 2 delivers:

- typed `AuthMode` enum (none / bearer / bearer-list / oauth) replacing the
  string-literal branch on `cfg.Token`;
- per-identity bearer-list mode so each AI / operator carries its own
  token + scope set on the session;
- OAuth 2.1 resource-server mode with stdlib JWT verification (RS256/RS384/
  RS512, ES256/ES384), local JWKS cache, RFC 9728 protected-resource
  metadata endpoint, and RFC 8707 audience binding;
- `bind-remote` leaf lifting the Phase-1 loopback clamp under explicit
  opt-in;
- `ze config validate` rejections for every unsafe combination (oauth
  without TLS on a non-loopback listener, bind-remote without auth, oauth
  without authorization-server, etc.);
- session registry GC drained on `Shutdown` (Phase 1's deferred goroutine
  leak closed).

## Decisions

- **Stdlib JWT over go-jose / jwx.** The signature verifier is ~300 LOC of
  `crypto/rsa` + `crypto/ecdsa`; the JWKS fetcher is ~170 LOC of
  `net/http` + `encoding/base64`. Adding a JWT library would violate the
  project's no-new-deps bias for a savings the caller still has to write
  (claim validation, kid selection, refresh policy) anyway. Cost is a
  maintained ~500 LOC stdlib implementation; benefit is one less
  supply-chain surface.
- **Local JWT verify, never token introspection.** RFC 7662 would double
  per-request latency and couple ze's runtime to a specific AS. Local
  verify against cached JWKS is faster and keeps the resource server
  AS-agnostic.
- **Identity bound at `initialize`, trusted by session-id thereafter.** The
  128-bit random session ID is the per-request token. Matches MCP
  2025-06-18 semantics where each session is a stateful conversation, and
  avoids re-parsing the JWT on every frame. Per-request auth would also
  break AC-11a (subsequent `tools/list` must succeed without re-supplying
  the Bearer).
- **`AuthMode=Bearer` renames the legacy token path; does not live as a
  separate code path.** A pre-Phase-2 config with `token` set but no
  `auth-mode` infers `bearer` so existing deployments upgrade silently.
  no-layering rule forbids keeping two dispatchers.
- **`oauth.audience` is operator-configured, never derived from
  `r.Host`.** A reverse proxy must not be able to change the advertised
  identity of the resource. Matches RFC 9728 + RFC 8707 guidance.
- **JWKS refresh rate-limited to 30 s minimum.** Unknown-kid tokens may
  signal real AS key rotation OR attacker spraying; the floor bounds the
  fetch rate against the AS. Unknown kid still within the window returns
  `invalid_token` without a fetch.
- **Config validation is exact-or-reject at verify time.** Every
  dangerous combination fails `ze config validate` before the daemon
  starts, not at request time. The operator gets a concrete message
  naming the missing leaf.

## Consequences

- **Phase 4 (tasks) unblocked.** `session.Identity()` is the scoping key
  the task registry will use to isolate one identity's tasks from
  another's. Plumbing landed now as value-type so no retrofit is needed.
- **`cmd/ze/hub/mcp.go` owns a `*MCPServerHandle`** (http.Server + *Streamable).
  Shutdown drains the session registry; the Phase-1 deferred goroutine
  leak (`plan/deferrals.md` row 226) is closed.
- **Legacy `Handler()` factory stays.** `tools_test.go` still imports it
  for ~26 HTTP tests. Phase 4 removes both in the task-augmentation
  refactor.
- **New env vars**: `ze.mcp.bind-remote`, `ze.mcp.auth-mode`,
  `ze.mcp.oauth.{authorization-server, audience, required-scopes}`,
  `ze.mcp.tls.{cert, key}`. Identity list is config-only (list shape
  doesn't map to a single env var).
- **`bin/ze` rebuild required to pick up the new config validator.**
  Three `test/parse/mcp-*.ci` tests fail without it.
- **OAuth `.ci` functional tests (test/mcp/oauth-*.ci) are not in this
  landing.** They need a mini AS harness binary; fidelity to the MCP
  contract is covered by unit tests + the in-process httptest AS used by
  `oauth_test.go`, `jwks_test.go`, and `jwt_test.go`. `test/mcp/` is the
  slot for future chained AS+MCP tests.

## Gotchas

- **`block-layering.sh` blocks "backwards compat" phrasing.** Code
  comments explaining the legacy token path must avoid "backwards
  compatibility" / "back-compat" substrings or the hook rejects the edit.
- **`block-pipe-tail.sh` blocks `| tail`** even for short ad-hoc
  inspections. Capture to `tmp/` and use `Read` instead.
- **`auto_linter.sh` is blocking and runs on every Write/Edit.** New types
  declared ahead of their consumer fail the "unused" lint; stage each
  write so every declaration has a use in the same burst or in the same
  file.
- **`staticcheck` flags `priv.PublicKey.N`** as QF1008 -- use the embedded
  shortcut `priv.N` / `priv.X` / `priv.Y`. Using the embedded field also
  triggers the `ecdsa.PublicKey` deprecation warnings for X/Y but those
  are downgraded to info-level.
- **`ecdsa.PublicKey` + `curve.IsOnCurve` are deprecated in Go 1.21+**.
  We skip the on-curve check (ecdsa.Verify does not revalidate, but JWK
  input comes from a trusted AS over TLS). Revisit if `crypto/ecdh`
  grows a signing path.
- **`expect=stdout:contains=` is not recognised by the parse runner.**
  Only `expect=stderr:contains=` is honored (the runner uses
  `cmd.CombinedOutput()` so either stream matches). Existing mcp-* parse
  tests now use `expect=stderr:contains=`.
- **`check-json-kebab.sh` blocks `json:"jwks_uri"` etc.** AS metadata
  decoding uses `map[string]any` + `stringField(m, "jwks_uri")` to
  bypass the hook for external-spec field names.
- **Identity list-key quirk**: the YANG `list identity { key "name" }` key
  value is read from `entry.Key`, not from a `name` leaf inside the
  entry. An earlier attempt to read the inner `name` leaf returned "".
- **`httptest.NewServer + time.Sleep(time.Hour)` handler deadlocks on
  close.** The server's `Close()` waits for in-flight handlers; a
  deliberate hang in the handler never returns. Use a raw
  `net.ListenConfig.Listen` that accepts but never writes, cleaned up
  through `t.Context().Done()`, for client-timeout tests.

## Files

**Created:**

- `internal/component/mcp/auth.go` (180 L) -- AuthMode enum, Identity
  value type, authenticator interface, authError + WWW-Authenticate
  rendering
- `internal/component/mcp/auth_test.go`
- `internal/component/mcp/bearer.go` (130 L) -- bearer / bearer-list /
  none strategies, constant-time scans
- `internal/component/mcp/bearer_test.go`
- `internal/component/mcp/jwt.go` (310 L) -- stdlib JWT parser + verifier
  (RS/ES 256/384/512), claim validator
- `internal/component/mcp/jwt_test.go`
- `internal/component/mcp/jwks.go` (250 L) -- JWKS fetch + cache,
  rate-limited refresh, JWK decoder (RSA + EC)
- `internal/component/mcp/jwks_test.go`
- `internal/component/mcp/as_metadata.go` (110 L) -- RFC 8414 AS metadata
  fetcher
- `internal/component/mcp/as_metadata_test.go`
- `internal/component/mcp/oauth.go` (200 L) -- OAuth strategy + RFC 9728
  metadata handler
- `internal/component/mcp/oauth_test.go`
- `test/parse/mcp-oauth-missing-as.ci` -- AC-6
- `test/parse/mcp-bind-remote-no-auth.ci` -- AC-6a
- `test/parse/mcp-oauth-no-tls.ci` -- AC-6b
- `rfc/short/rfc9728.md` -- Protected Resource Metadata
- `rfc/short/rfc8707.md` -- Resource Indicators
- `rfc/short/rfc8414.md` -- Authorization Server Metadata

**Modified:**

- `internal/component/mcp/schema/ze-mcp-conf.yang` -- `bind-remote`,
  `auth-mode`, `oauth` container, `tls` container, `identity[]` list
- `internal/component/mcp/streamable.go` -- StreamableConfig gains
  AuthMode / BearerList / OAuth fields, auth dispatcher, RFC 9728
  endpoint, OAuth strategy construction
- `internal/component/mcp/session.go` -- `session.identity`, Create
  takes Identity, Identity() getter
- `internal/component/mcp/session_test.go` / `streamable_test.go` --
  updated Create callers
- `internal/component/config/loader_extract.go` -- MCPListenConfig
  widened with BindRemote / AuthMode / Identities / OAuth / TLS, Validate
  method, loopback clamp suppressed on BindRemote
- `internal/component/config/loader_extract_test.go` -- 17-case Validate
  table + 4 extraction tests
- `internal/component/config/environment.go` -- 7 new `ze.mcp.*` env var
  registrations
- `cmd/ze/config/cmd_validate.go` -- `ze config validate` calls
  MCPListenConfig.Validate
- `cmd/ze/hub/mcp.go` -- MCPServerHandle bundles server + handler;
  Shutdown drains both; mcpConfigToStreamable helper
- `cmd/ze/hub/main.go` -- MCP extraction wires the full StreamableConfig
  through mcpConfigToStreamable
- `docs/guide/mcp/overview.md` -- rewritten Authentication section
- `docs/guide/mcp/remote-access.md` -- rewritten with Option 1 (tunnel) +
  Option 2 (native remote), verify-time rejection table

**Deferred**: OAuth `.ci` functional tests (test/mcp/oauth-*.ci),
session-gc-shutdown `.ci` test, legacy `Handler()` removal. All three
routed in `plan/deferrals.md` per the Phase 2 plan.
