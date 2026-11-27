# Spec: pki-show-private-key

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P0 |
| Release reason | Guard existing private-key bundle exposure independently of a new key-only output command. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Guard on `show pki certificate name <name> bundle pem` | Require BOTH an explicit RBAC `authz` grant (default-deny) AND an explicit `confirm` keyword, with redacted audit logging so private-key bytes never appear in logs or command history. |

## Task

Today `show pki certificate name <name> bundle pem` (`ze-pki:show-certificate-bundle-pem`) runs as an ordinary read-only `show` command with no explicit access-control gate and no confirmation keyword. On a daemon with no `system.authorization` configured, on any profile whose `run` section defaults to `allow` (including the built-in `read-only` and `admin` profiles), through the recovery account, through a shared API token, or from an external plugin endpoint without a restrictive bus send list, a caller can extract any stored device certificate's PKCS#8 private key in cleartext.

This release fix closes that exposure on `show pki certificate name <name> bundle pem` across every command surface (CLI, SSH, REST, gRPC, Web, MCP, wire-method RPC, and plugin dispatch) while preserving certificate-only `show pki certificate name <name> pem` unchanged (no guard) and preserving the exact `{"pem": "<cert chain + PKCS#8 private key>"}` output shape when authorized and confirmed:

1. Require an explicit RBAC `authz` grant (default-deny) via a reusable declarative registration flag (`RequireExplicitAuthz` on `pluginserver.RPCRegistration`, paired with `NotReadOnly: true` and an `aaa.ExplicitAuthorizer` interface enforced at the daemon dispatcher chokepoint). Neither unconfigured RBAC, nor `default-action allow`, nor the break-glass recovery profile, nor shared/no-auth API identities, nor unauthenticated internal RPC satisfy an explicit grant; plugin identities continue to be evaluated against the bus command profile compiled from their filter (`Default: Deny`) using the same explicit-entry rule so an endpoint with no explicit `cli:show pki certificate name bundle pem` grant is denied.
2. Require an explicit mandatory `confirm` keyword (`leaf confirm { type empty; mandatory true; }` in `ze-pki-cmd.yang`), validated by the command validator across every surface and re-checked in `handleShowPKICertificateBundlePEM` as defense-in-depth before reading or marshalling any private key.
3. Record a structured, redacted audit entry (`audit.ActionPKIPrivateKeyExport` = `pki-private-key-export`) on both successful and denied/refused bundle export attempts using the redacted command line only (never the response body), and redact PEM private-key blocks in `internal/core/redact` and CLI session transcripts (`TranscriptWriter.Record`) so private-key bytes never land in audit logs, daemon logs, command history, or transcript files.

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split divided the original combined private-key show specification into two specs. Status, existing dependencies, and recorded evidence are retained; no implementation, verification, or closure is claimed here for the post-release feature. Every test, documentation, and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Existing `bundle pem` guard, original AC-4, certificate-only AC-5, and original AC-3 for the existing bundle (`AC-3-bundle`) | `plan/release/fixes/spec-pki-show-private-key.md` |
| New key-only command (`show pki certificate name <name> private-key pem`), original AC-1/AC-2, and original AC-3 for the new form | `plan/post-release/features/spec-pki-key-only-output.md` |

Work assigned to `plan/post-release/features/spec-pki-key-only-output.md` is not performed or closed by this spec. When that post-release feature is implemented, it will reuse the `RequireExplicitAuthz`, mandatory `confirm`, and `audit.ActionPKIPrivateKeyExport` mechanisms introduced here rather than creating a second guard.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - Platform contract, one schema and one command registry across every surface, registration over hardcoding.
  → Decision: Declare the `confirm` keyword in `ze-pki-cmd.yang` and the guard metadata on `RPCRegistration` so CLI, SSH, REST, gRPC, Web, and MCP inherit both with zero per-surface edits.
  → Constraint: Do not special-case `bundle pem` in any adapter (`api`, `web`, `mcp`, `ssh`) or in central dispatcher switches.
- [ ] `ai/doors/access-security.md` - AAA chain, RBAC, PKI store, and `internal/core/audit` boundaries.
  → Decision: Keep `pki` free of any `authz` import; place the explicit-grant contract in `aaa` (`ExplicitAuthorizer`), evaluate it in `authz`, `tacacs`, and `cmd/ze/hub/aaa_lifecycle.go`, and enforce it in `plugin/server`.
  → Constraint: Allowed component imports in `docs/architecture/core-design.md`, section 19, permit `plugin/server` to import `aaa` and `audit`, and `authz`/`tacacs` to import `aaa`.
- [ ] `docs/architecture/pki/pki-store.md` - PKI certificate store and show command handlers.
  → Decision: Each output form of `show pki certificate name` (`pem`, `bundle pem`, `fingerprint`) remains its own YANG command container and handler.
  → Constraint: Preserve `certPEM` (certificate-only, no guard) and `certBundlePEM` (leaf + intermediates + PKCS#8 `PRIVATE KEY` via `marshalPrivateKeyPEM`).
- [ ] `docs/architecture/api/commands.md` - Single command dispatcher, typed argument validation, usage line generation, and audit recording.
  → Decision: Model `confirm` as a mandatory `type empty` YANG leaf (`ArgFlag`) so `ValidateArgs`, `UsageLine`, completion, REST/gRPC/MCP parameter schemas, and Web forms all read one definition.
  → Constraint: Mandatory `ArgFlag` renders as `UsageKeyword` (`confirm`, no square brackets) in `UsageLine`, while optional `ArgFlag` keeps `UsageFlag` (`[flag]`).
- [ ] `docs/architecture/aaa-tacacs.md` - Plugged AAA chain, TACACS+ per-command authorization, and local RBAC fallback.
  → Decision: `tacacsAuthorizer` implements `ExplicitAuthorizer` by sending RFC 8907 AUTHOR REQUEST for the command tokens and falling back strictly to `aaa.AuthorizeExplicit(a.local, ...)` when unreachable and `strictFallback` is false.
  → Constraint: On `strictFallback` or missing/unsupported local authorizer, fail closed (`false`).
- [ ] `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - Centralized secret redaction in `internal/core/redact` and transcript/history/audit masking.
  → Decision: Add `redact.PrivateKeyPEM` in `internal/core/redact/redact.go` and route `redact.Command`, `redact.JSON`, and `TranscriptWriter.Record` through it so private-key PEM blocks never persist in logs, history, or transcripts.
  → Constraint: Fail closed on unclosed or truncated `-----BEGIN ... PRIVATE KEY-----` blocks by redacting through end-of-string.
- [ ] `docs/guide/operator-access-rbac.md` - Profile-based authorization, fail-closed semantics, and recovery account contract.
  → Decision: Explicit-grant commands never inherit `default-action allow`, unconfigured-RBAC allow-all, or `ReservedRecoveryProfile` bypass; an operator must write an explicit `allow` entry in a profile's `edit` section.
  → Constraint: Update `docs/guide/authorization.md`, `docs/guide/audit.md`, and `docs/guide/command-reference.md` to document the explicit grant and `confirm` requirement.
- [ ] `ai/rules/user-facing-errors.md` - Operator error contract (`opererr`) across every surface.
  → Decision: Missing `confirm` returns a structured operator error naming the problem, why private-key export requires explicit confirmation, and the exact command ending with `confirm`; authorization denial returns `ErrUnauthorized` (`command restricted by access control`) so HTTP 403 / gRPC `PermissionDenied` / MCP / SSH error mappings stay consistent.
  → Constraint: Error responses never include private-key bytes.

**Key insights:**
- `internal/component/plugin/server/command.go` already evaluates authorization before argument validation and handler execution in `Dispatcher.dispatchLine`, so an unauthorized caller is rejected before probing whether `confirm` is missing or whether a named certificate exists.
- `internal/component/plugin/server/handler.go` already supports `RPCRegistration.NotReadOnly` (forcing `edit`-section RBAC and read-only API refusal on `show` commands) and `RPCRegistration.AuditAction`. Adding `RPCRegistration.RequireExplicitAuthz` extends the exact same declarative pattern.
- `internal/component/aaa/command_args.go` already defines `LocalScopeAuthorizer` / `AuthorizeLocalScope` with fail-closed delegation across all ten production `aaa.Authorizer` types (`StoreAuthorizer`, `boundStoreAuthorizer`, `tacacsAuthorizer`, `acceptedGenerationAuthorizer`, and the six wrappers in `cmd/ze/hub/aaa_lifecycle.go`). `ExplicitAuthorizer` / `AuthorizeExplicit` mirrors this proven pattern.
- Inline selectors put `<name>` in the middle of the typed line (`show pki certificate name dev-1 bundle pem confirm`), while the canonical YANG command path is `show pki certificate name bundle pem`. Evaluating explicit `Allow` entries against both the exact canonical command path (for all certificates) and the full typed command containing every canonical token in order (for per-certificate grants such as `show pki certificate name dev-1 bundle pem`) supports both without letting broad prefixes like `show` or `show pki` grant private-key export.
- `internal/component/cli/transcript.go` (`TranscriptWriter.Record`) writes command output verbatim to disk when `ze.cli.transcript=true`; redacting PEM private-key blocks there closes a real transcript leakage path.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/pki/show.go` - Registers `ze-pki:show-certificate-bundle-pem` with default `RPCRegistration` (`NotReadOnly: false`, no `AuditAction`, no explicit-authz flag). `handleShowPKICertificateBundlePEM` calls `certNamed`, checks `unexpectedAfter(cert.rest, afterCertName)` over `validated.Tokens()`, and calls `certBundlePEM(cert.entry)`, which concatenates the leaf certificate, intermediates, and PKCS#8 private key from `marshalPrivateKeyPEM` into `{"pem": ...}`. `handleShowPKICertificatePEM` emits only certificates via `certPEM`.
- [ ] `internal/component/pki/store.go` - Holds atomic in-memory `storeState` and `ExportPEM` (file export for IPsec, separate from CLI display). Defines `pemBlockCertificate` (`CERTIFICATE`) and `pemBlockPrivateKey` (`PRIVATE KEY`).
- [ ] `internal/component/pki/show_test.go` - Tests `handleShowPKILocalCAPEM` only; has no `bundle pem` guard tests.
- [ ] `internal/component/pki/store_test.go` - Calls `handleShowPKICertificateBundlePEM(nil, commandtest.Args(...))` directly for `dev-1`, `test-ca`, and `no-key` without `confirm`.
- [ ] `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - Declares `show pki certificate name bundle pem` with `ze:command "ze-pki:show-certificate-bundle-pem"` and only `leaf name { type string; mandatory true; }`; declares no `confirm` leaf.
- [ ] `internal/component/command/usage.go` - `usageLeafKind` maps every `ArgFlag` leaf to `UsageFlag`, which `writeUsageToken` always renders with square brackets (`[flag]`), even when `def.mandatory` is true.
- [ ] `internal/component/command/argbind.go` - `ValidateArgs` binds `ArgFlag` tokens in Phase 1 (`flags[def.name] = true`, excluded from `positional`), skips `ArgFlag` in Phase 2 (`awaitsPositional`), and enforces `def.mandatory` in Phase 3 via `*MissingArgumentError` using `keywordUsage` (`confirm`).
- [ ] `internal/component/command/completer.go` - `completeArgs` already offers `def.name` as a completion suggestion for `ArgFlag` leaves.
- [ ] `internal/component/aaa/aaa.go` and `internal/component/aaa/command_args.go` - Define `Authorizer`, `CommandArgsAuthorizer`, and fail-closed `LocalScopeAuthorizer` / `AuthorizeLocalScope`. No explicit-grant authorizer interface exists yet.
- [ ] `internal/component/aaa/login_profiles.go` - `acceptedGenerationAuthorizer` checks `acceptedLocalUserHolds` and delegates `Authorize`, `AuthorizeCommandArgs`, and `AuthorizeLocalScope` (returning `true` when `a.next == nil`).
- [ ] `internal/component/authz/authz.go` - `Store.authorize` short-circuits `Allow` for `ReservedInternalRPCIdentity`, `ReservedSharedAPIUsername`, and `ReservedRecoveryProfile`, evaluates plugin identities against compiled bus profiles, and falls back to `Section.Default` when no entry matches.
- [ ] `internal/component/authz/register.go` - `StoreAuthorizer` and `boundStoreAuthorizer` allow human callers when `Store == nil` in `Authorize` / `AuthorizeCommandArgs`, and fail closed in `AuthorizeLocalScope`.
- [ ] `internal/component/tacacs/authorizer.go` - `tacacsAuthorizer` sends RFC 8907 AUTHOR REQUEST and falls back to `a.local` on server error when `!strictFallback`; implements `AuthorizeLocalScope`.
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - Defines all six hub authorizer wrappers (`liveLocalAuthorizer`, `liveLocalProfileAuthorizer`, `acceptedLocalGenerationAuthorizer`, `acceptedLocalGenerationProfileFallback`, `liveAAABundleAuthorizer`, `liveAAABundleProfileAuthorizer`), returning `true` on nil store/authorizer in `Authorize`/`AuthorizeCommandArgs` and delegating `AuthorizeLocalScope`.
- [ ] `internal/component/plugin/server/handler.go`, `command.go`, `server.go`, `system.go` - `RPCRegistration` carries `NotReadOnly` and `AuditAction`. `Dispatcher.dispatchLine` calls `isAuthorized` (returns `true` when `authorizer == nil`) before `argErr` and handler execution; `recordCommandAudit` records only successful runs using `config.DisplayCommand(input)` (never `resp`). `Server.wrapHandler` injects `internalRPCIdentity` and calls `IsAuthorized`. `handleArgComplete` checks `isAuthorizedCommandArgs`.
- [ ] `internal/component/api/engine.go` and `cmd/ze/hub/api.go` - `APIEngine.Execute` enforces `req.Caller.ReadOnly && !readOnly` (reading `cmd.ReadOnly` from the dispatcher) and delegates to `serverDispatcher`; `apiDispatchError` maps `pluginserver.ErrUnauthorized` to `api.ErrUnauthorized` (HTTP 403 / gRPC `PermissionDenied`).
- [ ] `internal/core/redact/redact.go` - Redacts password hashes, credential keywords, JSON secret keys, and URL userinfo; does not yet redact PEM private-key blocks.
- [ ] `internal/component/cli/transcript.go` and `internal/component/cli/history.go` - `History` records only command inputs masked via `DisplayCommandAtPath` (never responses). `TranscriptWriter.Record` redacts `command` via `config.DisplayCommand`, but writes `output` verbatim to disk.
- [ ] `internal/test/fixture/plugin_fixture_10.go` and `test/plugin/pki-certificate-export-show.ci` - External plugin fixture `fixture10PKIExport` currently calls `show pki certificate name dev-1 bundle pem` without RBAC or `confirm` and expects the private key.
- [ ] `internal/component/plugin/server/usage_model_test.go` and `test/ui/test-pki-certificate-forms-are-separate-commands.ci` - Pin the generated usage line for `show pki certificate name bundle pem`.

**Behavior to preserve:**
- `show pki certificate name <name> pem` continues to return certificate(s) only (`{"pem": "..."}`) with no authorization or confirmation guard (AC-5).
- `show pki certificate name <name>` (detail), `show pki certificate name <name> fingerprint [algo]`, `show pki certificates`, and `show pki local-ca pem` remain unchanged.
- `show pki certificate name <name> bundle pem confirm`, once authorized and confirmed, continues to return the exact same `{"pem": "<leaf cert + intermediates + PKCS#8 PRIVATE KEY>"}` response format via `certBundlePEM` and `marshalPrivateKeyPEM` (AC-4).
- `certBundlePEM` continues to refuse CA certificates and device certificates with no private key, without returning key material.
- Existing RBAC behavior for every command that does not set `RequireExplicitAuthz` remains unchanged (including unconfigured-RBAC allow-all, `default-action allow`, recovery admin break-glass, and success-only `recordCommandAudit`).
- `pki.ExportPEM` (file export for IPsec) remains unchanged.

**Behavior to change:**
- `show pki certificate name <name> bundle pem` now requires BOTH an explicit RBAC `authz` grant (`RequireExplicitAuthz: true`, `NotReadOnly: true`, default-deny) AND the mandatory `confirm` keyword (`show pki certificate name <name> bundle pem confirm`).
- Unauthorized or unconfirmed requests on any surface (CLI, SSH, REST, gRPC, Web, MCP, wire-method RPC, or plugin dispatch) are rejected without returning private-key material (`AC-3-bundle`).
- Both authorized+confirmed exports and denied/unconfirmed attempts record a structured, redacted audit entry under `audit.ActionPKIPrivateKeyExport` (`pki-private-key-export`), carrying the redacted command line only and zero private-key bytes.
- `redact.PrivateKeyPEM` redacts PEM private-key blocks in `redact.Command`, `redact.JSON`, and `TranscriptWriter.Record`.
- Mandatory `ArgFlag` leaves render without brackets (`confirm`) in `command.UsageLine`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PKI bundle PEM RPC handler | `internal/component/pki/show.go` `handleShowPKICertificateBundlePEM`, `certBundlePEM`, `marshalPrivateKeyPEM` | Registers `ze-pki:show-certificate-bundle-pem` and returns leaf + intermediates + PKCS#8 private key without explicit authz, confirmation, or audit | Extends: sets `NotReadOnly: true`, `RequireExplicitAuthz: true`, `AuditAction: audit.ActionPKIPrivateKeyExport`, enforces `validated.Flag("confirm")` as defense-in-depth, and keeps `certBundlePEM` and `certPEM` output unchanged |
| PKI show command YANG grammar | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` `container bundle/container pem` | Declares `show pki certificate name <name> bundle pem` with `leaf name` only | Extends: adds `leaf confirm { type empty; mandatory true; }` on `bundle/pem` so every surface (CLI, SSH, REST, gRPC, Web, MCP) derives the mandatory confirmation keyword from the single YANG model |
| YANG flag argument binding and usage rendering | `internal/component/command/argbind.go` `ValidateArgs`, `ValidatedArgs.Flag`; `internal/component/command/usage.go` `usageLeafKind` | Binds `type empty` leaves as `ArgFlag`, enforces `mandatory true` in Phase 3, and renders all `ArgFlag` tokens with `[flag]` brackets | Extends: updates `usageLeafKind` to render mandatory `ArgFlag` as `UsageKeyword` (no brackets) while preserving `[flag]` for optional flags and reusing `ValidateArgs` untouched |
| Declarative RPC registration metadata | `internal/component/plugin/server/handler.go` `RPCRegistration`; `internal/component/plugin/server/command.go` `RegisterOptions`, `Command`, `Dispatcher.dispatchLine`, `recordCommandAudit`; `server.go` `wrapHandler`; `system.go` `handleArgComplete` | Drives `NotReadOnly`, `RequiresSelector`, and `AuditAction` from owner registrations at one dispatch chokepoint without hardcoding command names | Extends: adds `RequireExplicitAuthz bool` on `RPCRegistration`/`RegisterOptions`/`Command`, enforces `isAuthorizedExplicit` at `dispatchLine`, `wrapHandler`, and `handleArgComplete`, and records `OutcomeDenied`/`OutcomeSuccess`/`OutcomeError` audit entries for explicit-authz commands |
| Typed fail-closed AAA authorizer extension | `internal/component/aaa/command_args.go` `LocalScopeAuthorizer`, `AuthorizeLocalScope`; `aaa/login_profiles.go`; `authz/authz.go`, `authz/register.go`; `tacacs/authorizer.go`; `cmd/ze/hub/aaa_lifecycle.go` | Extends `aaa.Authorizer` with an optional fail-closed interface (`AuthorizeLocalScope`) implemented by all ten authorizer wrappers so components never import `authz` directly | Extends: adds sibling `ExplicitAuthorizer` / `AuthorizeExplicit` following the exact same interface and wrapper pattern, enforcing default-deny explicit `Entry` matching in `authz.Store` and RFC 8907 authorization with explicit local fallback in `tacacsAuthorizer` |
| Structured local audit trail | `internal/core/audit/audit.go` `Entry`, `Recorder`, `OutcomeSuccess`, `OutcomeDenied`, `OutcomeError`; `internal/core/audit/action_*.go` | Records structured JSONL audit entries with action constants declared in `action_*.go` files and command details masked through `config.DisplayCommand` | Extends: adds `internal/core/audit/action_pki_private_key.go` (`ActionPKIPrivateKeyExport = "pki-private-key-export"`) and records both `OutcomeSuccess` and `OutcomeDenied` for explicit-authz commands |
| Shared secret redaction and CLI transcript | `internal/core/redact/redact.go` `Command`, `JSON`, `Placeholder`; `internal/component/cli/transcript.go` `TranscriptWriter.Record` | Redacts password hashes, credential keywords, JSON secret keys, and URL userinfo; `TranscriptWriter.Record` redacts `command` but writes `output` verbatim | Extends: adds fail-closed `redact.PrivateKeyPEM`, applies it inside `redact.Command`, `redactValue` (`redact.JSON`), and `TranscriptWriter.Record` on `output` so private-key PEM bytes never reach logs, history, or transcripts |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | system door | "The CLI, SSH, the web UI, REST, gRPC and MCP dispatch into the same command registry." | agrees | `leaf confirm` is declared once in `ze-pki-cmd.yang` and `RequireExplicitAuthz`/`NotReadOnly`/`AuditAction` are declared once on `RPCRegistration`, reaching every surface through the shared dispatcher without adapter edits. |
| `ai/doors/system-plugins.md` | Boundaries | door covering `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` | "Its commands, YANG, doctor checks and help live in its folder." | agrees | The mandatory `leaf confirm` grammar lives in `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` inside the `pki-cmd` plugin rather than in a central verb package. |
| `docs/architecture/api/commands.md` | Overview | `// Design:` page of `usage.go`, `handler.go`, `command.go`, `usage_model_test.go`; anchored and modified page | "Request bodies and plugin RPC payloads do not carry identity fields. Caller identity is injected only by trusted transport wiring." | agrees | Explicit authorization reads only the transport-injected `CommandContext.Username` and `Authorizer`, never request payload fields; `commands.md` is updated for the guarded `bundle pem confirm` form. |
| `docs/architecture/bgp/on-demand-origination.md` | On-Demand Route Origination | anchored to `internal/component/command/usage.go` | - | silent | Covers BGP on-demand route origination (`announce` registry); says nothing about PKI private-key export, explicit RBAC, or mandatory flag usage rendering. |
| `ai/doors/commands-cli.md` | Boundaries | door covering `usage.go`, `usage_test.go`, `transcript.go`, `transcript_test.go` | "The argument is a leaf the model declares, never a positional token the handler reads on its own" | agrees | `confirm` is declared as a mandatory YANG leaf on `ze-pki-cmd.yang` (`ArgFlag`) and validated by `ValidateArgs` rather than parsed ad hoc as an undeclared positional token. |
| `docs/architecture/aaa-tacacs.md` | Authorization arguments are enforced | `// Design:` page of `command_args.go`, `local_scope_test.go`, `authorizer_fleet_scope_test.go`; anchored to `tacacs/authorizer.go`, `aaa_lifecycle.go`, `command.go` | "Command authorization uses `service=shell`, `cmd` and ordered `cmd-arg` values." | agrees | `tacacsAuthorizer.AuthorizeExplicit` sends the exact ordered typed command tokens over RFC 8907 AUTHOR REQUEST and falls back strictly to `aaa.AuthorizeExplicit` on the local store when unreachable and `!strictFallback`. |
| `docs/architecture/fleet-config.md` | Device connections and configuration history | anchored to `command_args.go`, `aaa_lifecycle.go`, `pki/show.go`; `// Design:` page of `aaa_lifecycle_fleet_scope_test.go` | - | silent | Covers managed hub/client configuration transport, device registry, baseline/hold, and fleet visibility; says nothing about PKI `bundle pem` private-key export or explicit RBAC grants. |
| `ai/doors/access-security.md` | Who depends on it from outside | door covering `aaa`, `authz`, `tacacs`, `pki`, `audit` | "Login is not an SSH feature. SSH, web, REST and gRPC read one accepted generation of users, and command authorization reads its policy from the same generation" | agrees | All six hub authorizer wrappers in `cmd/ze/hub/aaa_lifecycle.go` delegate `AuthorizeExplicit` against the same accepted identity/policy generation across every surface. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | `// Design:` page of `login_profiles.go`, `authz.go`, `transcript.go`; anchored to `aaa_lifecycle.go` | "Each component under `internal/component/` is independently removable. Cross-component coupling follows a strict hierarchy:" | agrees | `pki` does not import `authz` or `tacacs`; `plugin/server` imports only `aaa` and `audit`, `authz` imports `aaa`, and `cli` imports `core/redact`, honoring the section 19 import table. |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `login_profiles.go`, `authz.go`, `tacacs/authorizer.go`, `aaa_lifecycle.go`, `handler.go`, `command.go`, `server.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus participant allow-list compilation is preserved unchanged; plugin invocations of `RequireExplicitAuthz` commands still evaluate the compiled bus profile under explicit-entry rules. |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `authz.go`, `handler.go`, `command.go` | "The axis is declared in code rather than here. `verbRole` gives `show`, `monitor` and `resolve` the role `RoleRead`, `set` and `delete` the role `RoleMutation`, and every other canonical verb the role `RoleAction`." | agrees | `show` remains `RoleRead` in `command.Verbs`; `bundle pem` uses the existing `RPCRegistration.NotReadOnly` override plus `RequireExplicitAuthz` so private-key export requires `edit`-section explicit authorization without altering verb roles. |
| `ai/patterns/registration.md` | The Model | `// Design:` page of `authz/register.go`, `cmd/ze/hub/aaa_lifecycle.go` | "Every module (plugin, schema, env var, command, validator) follows this model. The core never imports a specific plugin. Plugins never import each other." | agrees | `RequireExplicitAuthz`, `NotReadOnly`, and `AuditAction` are declared by `pki` in its `init()` `RegisterRPCs` call; the dispatcher enforces them generically with zero hardcoded `pki` knowledge. |
| `docs/architecture/appliance/builder.md` | Appliance builder | anchored to `cmd/ze/hub/aaa_lifecycle.go` | - | silent | Covers appliance image assembly and gokrazy boot command lines; says nothing about PKI `bundle pem` authorization, confirmation, or redaction. |
| `docs/architecture/web-interface.md` | Web Interface Architecture | anchored to `cmd/ze/hub/aaa_lifecycle.go`, `internal/component/plugin/server/command.go` | - | silent | Covers HTMX web views, routes, and asset rendering; web terminal and admin forms inherit the YANG `confirm` leaf and dispatcher `RequireExplicitAuthz` automatically without web-layer changes. |
| `ai/doors/hub-engine.md` | Boundaries | door covering `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/aaa_lifecycle_fleet_scope_test.go` | "No always-on file imports a gated package." | agrees | `cmd/ze/hub/aaa_lifecycle.go` imports only always-on `aaa` and `authz` and delegates `AuthorizeExplicit` through `aaa.AuthorizeExplicit`, adding no import of any gated package (`ze_tacacs` stays behind the AAA interface). |
| `docs/architecture/api/process-protocol.md` | Design decisions | `// Design:` page of `handler.go`, `command.go`, `server.go`, `system.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `authorizePluginIdentityExplicit` evaluates plugin identities using the profile compiled from their bus filter (`Default: Deny`) under the same explicit-entry rule rather than granting by prefix or bypassing the bus profile. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `handler.go`, `system.go` | - | silent | Covers IPC wire framing between processes; this spec changes no IPC framing or wire protocol. |
| `docs/architecture/command-ownership.md` | The Folder Test | anchored to `handler.go`, `command.go` | "delete a plugin folder, run codegen, and every one of its features disappears. Every other plugin and the core keep working. No dangling references, no broken builds." | agrees | Deleting `internal/plugins/pki-cmd` removes `show pki certificate name bundle pem` cleanly because neither `plugin/server` nor `authz` hardcodes any `pki` command path. |
| `docs/architecture/traffic/traffic-usage-monitor.md` | Traffic Stat: the Aggregation Layer | anchored to `internal/component/plugin/server/handler.go` | - | silent | Covers traffic statistics aggregation; unrelated to PKI private-key export or explicit RBAC authorization. |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `handler.go`, `command.go`, `server.go`, `system.go`, `command_audit_test.go`, `usage_model_test.go` | "The engine never enumerates plugins." | agrees | `Dispatcher` checks the generic `Command.RequireExplicitAuthz` and `Command.AuditAction` metadata registered by any owner, never enumerating `pki` or any plugin name. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to `internal/component/plugin/server/command.go` | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Response key `pem` and new audit action `pki-private-key-export` are lowercase kebab-case; existing `{"pem": ...}` shape is preserved. |
| `docs/architecture/cli/command-completion.md` | What Tab and `?` reveal | anchored to `command.go`, `system.go` | "The request retains its authenticated caller, authorization policy and context." | agrees | `handleArgComplete` enforces `isAuthorizedExplicit` using the caller's authenticated context and authorizer when `builtin.RequireExplicitAuthz` is true, and `completer.go` offers `confirm` from YANG `ArgFlag`. |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | `// Design:` page of `redact.go`; anchored to `command.go`, `transcript.go`; modified page | "The transcript is the one that writes to a FILE, so a credential typed at the prompt outlived the session until `Record` was routed through it." | agrees | Extends `internal/core/redact` with fail-closed `redact.PrivateKeyPEM` and routes `TranscriptWriter.Record` output through it so private-key PEM bytes never persist in transcript files, audit logs, or command history. |
| `docs/architecture/config/yang-config-design.md` | YANG Configuration System | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers YANG module loading and config transaction validation; `ze-pki-cmd.yang` adds a standard RFC 7950 `type empty` `mandatory true` command leaf without changing config schema mechanics. |
| `docs/architecture/doctor-and-health-checks.md` | Doctor and Runtime Health Checks | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers doctor readiness checks and codes; this spec introduces no new runtime dependency or doctor code (audit write failures continue using `doctor-audit-record-failed`). |
| `docs/architecture/diagnostics/production-diagnostics.md` | Production Diagnostics | anchored to `internal/component/plugin/server/command.go` | - | silent | Covers production diagnostics, operation correlation (`op-<N>`), reload history, and liveness watchdog; says nothing about PKI private-key export, explicit RBAC, or mandatory confirmation, and `recordCommandAuditOutcome` preserves `entry.OpID = ctx.OpID()`. |
| `docs/architecture/hub-architecture.md` | Hub Architecture | anchored to `server.go`, `system.go` | - | silent | Covers legacy/daemon hub startup and plugin lifecycle; says nothing about PKI private-key command guards. |
| `docs/architecture/overview.md` | Ze Architecture Overview | anchored to `internal/component/plugin/server/server.go` | - | silent | High-level system overview pointing to `core-design.md`; says nothing about PKI private-key export guards. |
| `docs/architecture/testing/runner-architecture.md` | Test Runner Architecture | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers `.ci`/`.wb` test execution architecture; this spec uses existing runner primitives unchanged. |
| `docs/architecture/update-cache.md` | UPDATE Cache Architecture | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers BGP UPDATE message caching; unrelated to PKI private-key export or RBAC. |
| `docs/architecture/testing/ci-format.md` | .ci Test File Format | anchored to `internal/component/plugin/server/system.go` | - | silent | Covers `.ci` test file syntax; `test/plugin/pki-show-private-key.ci` uses standard `.ci` directives unchanged. |
| `docs/architecture/pki/pki-store.md` | Decisions | `// Design:` page of `pki/show.go`, `pki/store_test.go`; modified page | "Each output form of `show pki certificate name` is its own command." | agrees | Keeps `pem`, `bundle pem`, and `fingerprint` as sibling YANG command containers; adds the mandatory `confirm` leaf and explicit RBAC + redacted audit guard to `bundle pem` alone while leaving `pem` unguarded and updating `pki-store.md`. |
| `docs/architecture/appliance/on-device-installer.md` | On-device installer and the un-brick path | anchored to `internal/core/redact/redact.go` | - | silent | Covers `ze install disk` appliance installation; unrelated to `redact.PrivateKeyPEM` or PKI show commands. |
| `docs/architecture/config/system-update.md` | System Update: the backend, and the version check | anchored to `internal/core/redact/redact.go` | - | silent | Covers firmware update checks and URL credential redaction; unaffected by adding `redact.PrivateKeyPEM`. |
| `docs/architecture/resolve.md` | Resolution Component | anchored to `internal/core/redact/redact.go` | - | silent | Covers DNS/Cymru resolution services; unrelated to PKI private-key export or PEM redaction. |
| `ai/doors/core.md` | Boundaries | door covering `internal/core/redact/redact.go`, `internal/core/redact/redact_test.go` | "Never import upward." | agrees | `redact.PrivateKeyPEM` uses only Go standard library primitives inside `internal/core/redact` and adds no upward component or plugin imports. |
| `docs/architecture/iface/netlink-monitor.md` | Netlink Monitor Command | anchored to `internal/test/fixture/plugin_fixture_10.go` | - | silent | Covers `monitor system netlink`; anchored only because `plugin_fixture_10.go` hosts multiple unrelated observer fixtures. |
| `ai/doors/le-tooling.md` | Boundaries | door covering `plugin_fixture_10.go`, `test/plugin/pki-show-private-key.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | Updated `fixture10PKIExport` returns a non-nil error if `bundle pem` succeeds or leaks private-key bytes to the external plugin. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `confirm` is a mandatory safety keyword on the action rather than a scope, presentation, or format pipe operator; output shaping (`\| json`, `\| yaml`, `\| raw`) remains in the pipe layer unchanged. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds no pipe-operation override and preserves `bundle pem`'s structured `{"pem": ...}` answer shape for all generic pipe operators. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `confirm` changes whether the command is permitted to execute (not a post-answer transform), so placing `confirm` as a mandatory command leaf rather than a pipe operation agrees with this boundary. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Certificates remain stored in the `pki` configuration block and managed listener reload/TLS serving (`ServerTLSMaterial`) is completely untouched. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Fleet web Config tab provisioning and editing are untouched; `AuthorizeLocalScope` and standard `edit` authorization remain unchanged for non-`RequireExplicitAuthz` commands. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches only PKI show command authorization, confirmation, and redaction, leaving VPP/kernel data-plane validation unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Unchanged; this spec introduces no config-load/start mismatch or data-plane verification change. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unchanged; firewall VPP verification and `ze:backend` annotations are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Unchanged; no data-plane backend selector or registration is touched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Unchanged; BGP export filter chain and next-hop rewriting are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Unchanged; BGP policy modify and next-hop chain refs are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Unchanged; BGP export chain compilation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Unchanged; BGP UPDATE next-hop folding and provenance are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Unchanged; BGP IPv4/IPv6 next-hop validation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Unchanged; BGP route origination next-hop handling is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Unchanged; implied filter-chain display is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no startup refusal or exit path; a daemon with no `system.authorization` configured still starts normally and simply denies `bundle pem` at command dispatch time until an explicit RBAC grant is configured. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Unchanged; startup kernel/VPP mismatch handling is untouched and this spec adds no startup refusal. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Unchanged; `\| implied` config display is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Unchanged; bus participant registration and naming are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Unchanged; `send > to` compilation and enforcement are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Unchanged; `receive > from` compilation and enforcement are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Unchanged; bus `send > allow` / `receive > allow` enforcement is preserved, and `authorizePluginIdentityExplicit` evaluates the compiled bus command profile under explicit-entry rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Unchanged; bus filter compilation and conflict refusal are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Unchanged; no bus arbitration or exclusive-role logic is added. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Unchanged; bus effective-filter display is untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `authorizePluginIdentityExplicit` authorizes plugin identities strictly through the profile compiled from their bus filter (`compilePluginProfile`, `Default: Deny`) using explicit-entry matching rather than prefix bypasses. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Unchanged; engine-to-plugin event delivery and queues are untouched. |

## Data Flow (MANDATORY)

### Entry Point
- Operator or API invocation of `show pki certificate name <name> bundle pem confirm` (or without `confirm`, which is refused) entering via CLI (`ze cli -c` / interactive SSH), Web terminal or admin form, REST (`POST /api/v1/execute`), gRPC (`Execute`), MCP (`tools/call`), wire-method RPC (`ze-pki:show-certificate-bundle-pem`), or plugin `dispatch-command`.
- Format at entry: text command line on CLI/SSH/Web/plugin dispatch, or structured parameters (`{"name": "<name>", "confirm": true}`) normalized to command tokens by `api.prepareCommand` / MCP / Web before reaching `Dispatcher.Dispatch`.

### Transformation Path
1. **Transport & Identity Injection:** The surface authenticates the caller (SSH, Web, REST, gRPC, MCP) and injects `CallerIdentity{Username, RemoteAddr, Surface, Authorizer, ReadOnly}` into `serverDispatcher` (`cmd/ze/hub/main_servers.go`), which constructs `pluginserver.CommandContext`. Unauthenticated REST/gRPC callers have `Caller.ReadOnly = true`, so `APIEngine.Execute` refuses `bundle pem` (`NotReadOnly: true`) immediately with `api.ErrUnauthorized` (HTTP 403 / gRPC `PermissionDenied`).
2. **Command Resolution & Held Argument Validation:** `Dispatcher.dispatchLine` (`internal/component/plugin/server/command.go`) scans the line (`command.ReadDispatchLine`), matches `show pki certificate name bundle pem`, extracts inline selector `name`, and computes `command.ValidateArgs(args, matchedCmd.ArgDefs, selectors)` (holding `argErr` until after authorization so unauthorized callers cannot probe arguments).
3. **Default-Deny Explicit Authorization Gate:** Because `matchedCmd.RequireExplicitAuthz` is true, `Dispatcher.dispatchLine` calls `d.isAuthorizedExplicit(ctx, matchedCmd.Name, input, line.TypedTokens(), matchedCmd.ReadOnly)` (with `matchedCmd.ReadOnly == false` because `NotReadOnly: true`).
   - If `authorizer == nil` or does not implement `aaa.ExplicitAuthorizer`: fails closed (`false`).
   - In `authz.Store.authorizeExplicit`: rejects empty username, `ReservedInternalRPCIdentity`, `ReservedSharedAPIUsername`, and `ReservedRecoveryProfile` (never short-circuiting `Allow`). For a plugin identity (`~ze~internal:plugin:<endpoint>`), calls `authorizePluginIdentityExplicit` to evaluate the plugin's compiled bus command profile (`compilePluginProfile`, `Default: Deny`) under the exact same explicit-entry matcher so plugins without an explicit `cli:show pki certificate name bundle pem` grant are denied while remaining governed by their compiled bus profile. For operator users, walks assigned/bound profiles' `edit` sections in order: first matching `Entry` wins (`Deny` matches `typedCommand` or `canonicalPath`; `Allow` prefix entry must match either exact `canonicalPath` `"show pki certificate name bundle pem"` or `typedCommand` while containing every canonical path token in order, such as `"show pki certificate name dev-1 bundle pem"`; `Allow` regex entry must match `typedCommand` and must not match an unrelated generic prefix such as `"show"` or `"show pki certificate name"`). Never falls back to `Section.Default`; if no entry matches, returns `Deny`.
   - In `tacacsAuthorizer.AuthorizeExplicit`: sends RFC 8907 AUTHOR REQUEST with the typed tokens; on server error/unreachable obeys `strictFallback` (deny) or delegates to `aaa.AuthorizeExplicit(a.local, ...)` (failing closed if `a.local == nil` or unsupported).
   - On denial: records a redacted audit entry (`Action: pki-private-key-export`, `Outcome: denied`, `Detail: config.DisplayCommand(input)`), and returns `ErrUnauthorized` with zero private-key bytes.
4. **Confirmation Gate (Model + Handler Defense-in-Depth):** If `argErr != nil` (for example, `confirm` was omitted, producing `*command.MissingArgumentError` restated with the exact command ending in `confirm`), `Dispatcher.dispatchLine` records a redacted audit entry (`Action: pki-private-key-export`, `Outcome: denied`) and returns the structured operator refusal before invoking the handler. Inside `handleShowPKICertificateBundlePEM`, the handler re-verifies `validated.Flag("confirm")` before reading or marshalling any key material, returning `plugin.OperatorErrorResponse` if false.
5. **Bundle Emission & Redacted Audit:** When authorized and confirmed, `handleShowPKICertificateBundlePEM` reads `validated.PositionalTokens()` (so `confirm` is not flagged by `unexpectedAfter`), resolves the named certificate via `certNamed`, and delegates to `certBundlePEM(cert.entry)` unchanged (`{"pem": "<leaf + intermediates + PKCS#8 PRIVATE KEY>"}`). On return, `Dispatcher.recordCommandAudit` records `Action: pki-private-key-export`, `Outcome: success` (or `Outcome: error` if `certBundlePEM` refused a CA or keyless cert), with `Detail: config.DisplayCommand(input)` — never the response payload.
6. **Transcript & Log Redaction:** Any command recorded in audit, accounting, history, or daemon logs passes through `config.DisplayCommand` -> `redact.Command` (which invokes `redact.PrivateKeyPEM`), and CLI transcripts (`TranscriptWriter.Record`) pass `output` through `redact.PrivateKeyPEM`, ensuring PEM private-key bytes never land in any log, history, or transcript file.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator surface (CLI / SSH / REST / gRPC / Web / MCP) ↔ `plugin/server.Dispatcher` | Unified `serverDispatcher` carrying trusted `CallerIdentity` and `CommandContext` | Yes |
| `plugin/server.Dispatcher` ↔ `aaa` / `authz` / `tacacs` | `aaa.ExplicitAuthorizer` interface (fail-closed `aaa.AuthorizeExplicit`); no `pki` import of `authz` | Yes |
| `plugin/server.Dispatcher` ↔ `internal/component/pki` | `pluginserver.RPCRegistration` (`NotReadOnly: true`, `RequireExplicitAuthz: true`, `AuditAction: audit.ActionPKIPrivateKeyExport`) and `handleShowPKICertificateBundlePEM` | Yes |
| `plugin/server.Dispatcher` ↔ `internal/core/audit` | `audit.Recorder.Record` with `audit.ActionPKIPrivateKeyExport` and redacted `Detail` | Yes |
| `internal/component/cli` ↔ `internal/core/redact` | `TranscriptWriter.Record` calls `redact.PrivateKeyPEM(output)` | Yes |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `test/`, and `docs/` with `git grep -n 'show-certificate-bundle-pem\|bundle pem'` (positive controls: `internal/component/pki/show.go` `handleShowPKICertificateBundlePEM` and `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` `ze:command "ze-pki:show-certificate-bundle-pem"`), `git grep -n 'ActionDataConfigRename\|ActionDataKeyChange'` for audit action consumers, and `git grep -n 'func.*AuthorizeCommandArgs(' internal/ cmd/` for every `aaa.Authorizer` implementation (10 production types across 4 files).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-pki:show-certificate-bundle-pem` CLI & RPC command | writer (schema & usage) | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` (`container bundle/container pem`), `internal/component/command/usage.go` (`usageLeafKind`) | Declares the command path and leaves; adds mandatory `leaf confirm` | Change: add mandatory `leaf confirm { type empty; mandatory true; }` and render mandatory `ArgFlag` without brackets |
| `ze-pki:show-certificate-bundle-pem` CLI & RPC command | reader (dispatcher & handler) | `internal/component/plugin/server/command.go` (`Dispatcher.dispatchLine`), `server.go` (`wrapHandler`), `system.go` (`handleArgComplete`), `internal/component/pki/show.go` (`handleShowPKICertificateBundlePEM`, `certBundlePEM`) | Dispatches and executes `bundle pem`; returns `{"pem": cert+key}` | Change guard only: enforce `RequireExplicitAuthz` + `confirm` before `certBundlePEM`; preserve exact `{"pem": ...}` response payload shape when authorized and confirmed |
| `ze-pki:show-certificate-bundle-pem` CLI & RPC command | writer (external/in-tree callers) | Operators on CLI/SSH/Web/REST/gRPC/MCP; `internal/test/fixture/plugin_fixture_10.go` (`fixture10PKIExport`); `internal/component/pki/store_test.go` (`TestShowPKICertBundlePEM*`); `internal/component/plugin/server/usage_model_test.go` | `fixture10PKIExport` sends `bundle pem` from an external plugin; `store_test.go` calls handler without `confirm`; `usage_model_test.go` checks exact usage string | Change tests to match guarded contract: `fixture10PKIExport` asserts plugin refusal without key leak, `store_test.go` tests both missing-confirm refusal and confirmed bundle output, `usage_model_test.go` expects trailing `confirm` |
| Structured audit JSONL (`audit.Entry`) | writer | `internal/core/audit/action_pki_private_key.go` (`ActionPKIPrivateKeyExport`), `internal/component/plugin/server/command.go` (`recordCommandAudit`, `recordCommandAuditOutcome`) | `Dispatcher` writes `audit.Entry` via `d.audit.Record` with `Detail: config.DisplayCommand(input)` | Change (additive): add new action constant `"pki-private-key-export"` and emit `OutcomeSuccess`/`OutcomeDenied`/`OutcomeError` entries with zero private-key bytes |
| Structured audit JSONL (`audit.Entry`) | reader | `internal/component/cmd/show/audit.go` (`show audit`), `internal/core/audit/query.go`, external log/SIEM consumers of `<config-base>.audit.jsonl` | Reads `Entry` fields (`timestamp`, `actor`, `remote-addr`, `surface`, `action`, `detail`, `outcome`) without a hardcoded action switch | Preserve: new action string `"pki-private-key-export"` flows through existing `show audit action pki-private-key-export` filter unchanged |

### Integration Points
- `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - Declares `leaf confirm { type empty; mandatory true; }` on `show pki certificate name bundle pem`.
- `internal/component/aaa/command_args.go` & `login_profiles.go` - Defines `ExplicitAuthorizer` and `AuthorizeExplicit` helper; wires `acceptedGenerationAuthorizer`.
- `internal/component/authz/authz.go` & `register.go` - Evaluates explicit default-deny RBAC rules in `Store` and exposes them on `StoreAuthorizer` and `boundStoreAuthorizer`.
- `internal/component/tacacs/authorizer.go` - Implements `ExplicitAuthorizer` over RFC 8907 AUTHOR REQUEST with fail-closed explicit local fallback.
- `cmd/ze/hub/aaa_lifecycle.go` - Implements `ExplicitAuthorizer` across all six live/accepted hub authorizer wrappers.
- `internal/component/plugin/server/handler.go`, `command.go`, `server.go`, `system.go` - Enforces `RequireExplicitAuthz` and records redacted audit outcomes at the single command dispatch chokepoint.
- `internal/component/pki/show.go` - Registers `ze-pki:show-certificate-bundle-pem` with `NotReadOnly: true`, `RequireExplicitAuthz: true`, `AuditAction: audit.ActionPKIPrivateKeyExport`, and checks `validated.Flag("confirm")`.
- `internal/core/audit/action_pki_private_key.go` - Registers the `ActionPKIPrivateKeyExport` constant.
- `internal/core/redact/redact.go` & `internal/component/cli/transcript.go` - Redacts PEM private-key blocks from commands, JSON captures, and CLI session transcripts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every surface (CLI, SSH, REST, gRPC, Web, MCP, plugin dispatch) routes through `Dispatcher.Dispatch`, wire RPC through `Server.wrapHandler`, and completion through `handleArgComplete`; all three enforce `isAuthorizedExplicit`, and `handleShowPKICertificateBundlePEM` re-checks `confirm`. |
| No unintended coupling (components stay isolated) | Yes | `pki` imports only `plugin/server`, `command`, `plugin`, and `internal/core/*` (`audit`, `opererr`, `textbuf`) — never `authz` or `tacacs`. `plugin/server` depends only on `aaa.ExplicitAuthorizer` and `audit.Recorder`, matching `docs/architecture/core-design.md`, section 19. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `RPCRegistration` (`NotReadOnly`, `AuditAction`), `aaa` typed authorizer seam (mirroring `LocalScopeAuthorizer`), `command.ArgFlag` (`type empty`), `audit.Recorder`, `redact`, and `marshalPrivateKeyPEM`. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not a BGP wire-forwarding hot path; `bundle pem` is a rare control-plane operational RPC. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `ze-pki:show-certificate-bundle-pem` declares `NotReadOnly: true`, `RequireExplicitAuthz: true`, and `AuditAction: audit.ActionPKIPrivateKeyExport` in its own `init()` registration; the dispatcher reads those generic options without ever naming `pki` or `bundle pem`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/api/`, `internal/component/mcp/`, `internal/component/web/`, `internal/component/command/`, `internal/component/config/infra/authz.go`, and `internal/component/cmd/show/audit.go`: REST/gRPC/MCP/Web parameter schemas, CLI usage, CLI completion, `unknownMatchEntries`, and `show audit` all derive from YANG and the command/audit registries with zero hardcoded command or action lists. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All ten production `aaa.Authorizer` implementations are accounted for across `aaa`, `authz`, `tacacs`, and `cmd/ze/hub/aaa_lifecycle.go` | `git grep -n 'func.*AuthorizeCommandArgs(' internal/ cmd/` returned exactly those 10 types | A surface wrapped in an un-updated authorizer would fail closed (deny even explicitly authorized users) | Unit tests covering every wrapper (`TestAuthorizeExplicitNilAndUnsupportedFailClosed`, `TestAuthorizeExplicitDefaultDenyAndExplicitGrants`, `TestTacacsAuthorizerAuthorizeExplicit`, `TestHubAuthorizerWrappersAuthorizeExplicit`) + live SSH/CLI daemon `test/plugin/pki-show-private-key.ci` (10/10 stress pass) | confirmed |
| A-2 | `unknownMatchEntries` in `internal/component/config/infra/authz.go` accepts both `match "show pki certificate name bundle pem"` and `match "show pki certificate name <cert> bundle pem"` without warning | `unknownMatchEntries` checks `strings.HasPrefix(cmd, match) \|\| strings.HasPrefix(match, cmd)` against `cmds` containing both `"show pki certificate name bundle pem"` and `"show pki certificate name"` | Config load would emit a spurious warning on valid explicit grants | Source verification at `internal/component/config/infra/authz.go:287-318` (`unknownMatchEntries`) + live daemon boot with `pki-rbac.conf` in `test/plugin/pki-show-private-key.ci` | confirmed |
| A-3 | `leaf confirm { type empty; mandatory true; }` is accepted by Ze's YANG loader, `ValidateArgs`, `completer.go`, REST/gRPC `prepareCommand`, MCP tool builder, and Web form builder | Read `yang/command.go` (`Yempty` -> `NewFlagArg`), `argbind.go` (Phase 1 `ArgFlag` + Phase 3 `mandatory`), `completer.go`, `api/rest/flag_params_test.go`, `web/graphical_supplied_commands.go` | Confirmation keyword would not validate or expose cleanly across all surfaces | Unit tests in `command` (`TestUsageRendersMandatoryFlagWithoutBrackets`), `pki` (`TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput`), `plugin/server` (`TestDispatcherExplicitAuthzAndRedactedAudit`, `TestUsageRendersTheDeclaredValues`), `./le cli grammar`, `./le doc yang-contract command-contract`, and `test/plugin/pki-show-private-key.ci` | confirmed |
| A-4 | No existing command combines `type empty` with `mandatory true`, so changing `usageLeafKind` for mandatory `ArgFlag` affects no other command's usage line | `git grep` over all `internal/**/yang/*.yang` found zero `type empty` leaves with `mandatory true` | Existing usage-line tests would change unexpectedly | `TestUsageRendersTheDeclaredValues` (`internal/component/plugin/server/usage_model_test.go`), `TestUsageRendersMandatoryFlagWithoutBrackets`, and `test/ui/test-pki-certificate-forms-are-separate-commands.ci` PASS | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Private-key bytes leak into audit log, daemon log, command history, error text, or CLI transcript | Any test searching audit JSONL, stderr/logs, history, or transcript finds `PRIVATE KEY` or base64 key bytes | Audit records `config.DisplayCommand(input)` only (never `resp`); errors never carry key material; `redact.PrivateKeyPEM` scrubs PEM private keys in `redact.Command`, `redact.JSON`, and `TranscriptWriter.Record` |
| R-2 | Guard bypassed on a secondary path (`wrapHandler` wire RPC, plugin `dispatch-command`, unauthenticated REST/gRPC, shared API token, recovery admin, or unconfigured RBAC) | Test on any secondary path receives key bytes instead of refusal | Enforce `RequireExplicitAuthz` at `Dispatcher.dispatchLine`, `Server.wrapHandler`, and `handleArgComplete`; `NotReadOnly: true` blocks read-only API; `authz.Store.authorizeExplicit` rejects all reserved/nil-store identities and evaluates plugins via `authorizePluginIdentityExplicit` (requiring an explicit `cli:show pki certificate name bundle pem` grant in the compiled bus profile) |
| R-3 | Broad prefix (`allow "show"`, `allow "show pki"`) or wildcard regex (`allow ".*"`) accidentally grants private-key export | `TestAuthorizeExplicitRejectsBroadPrefixAndWildcardRegex` fails | `authorizeExplicit` requires an `Allow` prefix entry to match either exact canonical path or include every canonical path token in order, and requires an `Allow` regex entry not to match unrelated generic prefixes (`"show"`, `"show pki certificate name"`) |
| R-4 | Existing `test/plugin/pki-certificate-export-show.ci` fails because its external plugin previously read `bundle pem` without RBAC or `confirm` | Functional suite `plugin` fails on `pki-certificate-export-show.ci` | Update `fixture10PKIExport` in `internal/test/fixture/plugin_fixture_10.go` to assert that an external plugin is refused `bundle pem` without receiving key bytes, while keeping `pem`, `fingerprint`, and `test-ca pem` passing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | If too permissive: private keys remain extractable by unauthorized users or plugins. If too restrictive (e.g., a wrapper omits `ExplicitAuthorizer`): operators with an explicit RBAC grant and `confirm` are denied until the wrapper delegates `AuthorizeExplicit`. Certificate-only `pem`, `detail`, `fingerprint`, and TLS listeners are unaffected. |
| How is it reverted? | Single commit revert; no stored config format or ZeFS key schema migration is introduced. |
| Who else touches this path? | `plan/post-release/features/spec-pki-key-only-output.md` will later add `show pki certificate name <name> private-key pem` reusing this exact guard mechanism. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show pki certificate name <name> bundle pem confirm` (authorized user with explicit RBAC `edit` grant) | → | `Dispatcher.dispatchLine` (`isAuthorizedExplicit`) → `ValidateArgs` (`confirm` flag) → `handleShowPKICertificateBundlePEM` → `certBundlePEM` → `recordCommandAudit` (`pki-private-key-export`, `success`) | `test/plugin/pki-show-private-key.ci` |
| `show pki certificate name <name> bundle pem` (authorized user, missing `confirm`) | → | `Dispatcher.dispatchLine` → `ValidateArgs` (`MissingArgumentError` for `confirm`) → `recordCommandAuditOutcome` (`denied`) | `test/plugin/pki-show-private-key.ci` |
| `show pki certificate name <name> bundle pem confirm` (unauthorized user: `read-only`, `admin` default-action allow without explicit entry, unconfigured RBAC, or plugin) | → | `Dispatcher.dispatchLine` (`isAuthorizedExplicit` -> `Deny`) → `recordCommandAuditOutcome` (`denied`) → `ErrUnauthorized` | `test/plugin/pki-show-private-key.ci`, `test/plugin/pki-certificate-export-show.ci` |
| `show pki certificate name <name> pem` (cert-only, any `read-only` user, no `confirm`) | → | `Dispatcher.dispatchLine` (`isAuthorized`, `ReadOnly: true`) → `handleShowPKICertificatePEM` → `certPEM` | `test/plugin/pki-show-private-key.ci`, `test/plugin/pki-certificate-export-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | Authenticated caller with an explicit RBAC `authz` grant (`edit` entry matching `show pki certificate name bundle pem` or `show pki certificate name <name> bundle pem`) runs `show pki certificate name <name> bundle pem confirm` for a device certificate with a private key | Command succeeds with `StatusDone`, returning `{"pem": "<leaf cert + intermediates + PKCS#8 PRIVATE KEY>"}` in the exact pre-existing certificate-before-key PEM order, and records a `pki-private-key-export` audit entry with `outcome: "success"` and redacted command detail containing zero private-key bytes | Calling `bundle pem confirm` on a CA certificate or a device certificate without a private key returns the existing PKI error response (`bundle is only available for device certificates` or `has no private key`) and records `outcome: "error"`, because those entries hold no private key to export. |
| AC-5 | Caller runs `show pki certificate name <name> pem` (with a normal `read-only` profile, no `confirm`, and no explicit bundle export grant) | Command succeeds with `StatusDone`, returning `{"pem": "<leaf cert + intermediates>"}` containing `BEGIN CERTIFICATE` and zero private-key blocks, unchanged from prior behavior | None: `pem` is certificate-only and requires neither `confirm` nor `RequireExplicitAuthz`. |
| AC-3-bundle | Caller runs `show pki certificate name <name> bundle pem` without `confirm` (even when explicitly authorized), OR runs `show pki certificate name <name> bundle pem confirm` without an explicit RBAC `authz` grant (unconfigured RBAC, `read-only`, `operator`, `admin` with only `default-action allow`, recovery admin without explicit entry, broad prefix `allow "show pki"`, shared/no-auth API, internal RPC, or plugin endpoint without an explicit compiled bus grant for `bundle pem`) | Request is rejected with an operator-visible refusal (`command restricted by access control` / `ErrUnauthorized` on authz denial; structured `opererr` naming `confirm` and the exact command on missing `confirm`), returns zero private-key bytes in the response, and records a redacted `pki-private-key-export` audit entry with `outcome: "denied"` containing zero private-key bytes | None: every unauthorized or unconfirmed path fails closed with zero key material emitted or logged. |
| AC-6 | Private-key bundle export runs while CLI session transcript recording (`ze.cli.transcript=true`), command history, daemon logging, and audit logging are enabled | Audit entries, command history entries, daemon log lines, and CLI transcript files record the command metadata with zero `PRIVATE KEY` PEM block bytes (`TranscriptWriter.Record` replaces any PEM private-key block in `output` with `<redacted>` while keeping `CERTIFICATE` blocks readable) | The operator's interactive stdout / API response body itself carries the requested PEM bundle when authorized and confirmed (AC-4), because delivering the bundle to the authorized caller is the purpose of the command. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Security administrator with an explicit RBAC `edit` grant runs `show pki certificate name dev-1 bundle pem confirm` over authenticated CLI/SSH | SSH/CLI → `serverDispatcher` → `Dispatcher.dispatchLine` → `isAuthorizedExplicit` (allow) → `ValidateArgs` (`confirm` present) → `handleShowPKICertificateBundlePEM` → `certBundlePEM` → `recordCommandAudit` (`success`, no key bytes) | `test/plugin/pki-show-private-key.ci` |
| 2 | Security administrator with an explicit RBAC grant omits `confirm` (`show pki certificate name dev-1 bundle pem`) | SSH/CLI → `Dispatcher.dispatchLine` → `isAuthorizedExplicit` (allow) → `ValidateArgs` (missing `confirm` refusal naming exact command) → `recordCommandAuditOutcome` (`denied`) | `test/plugin/pki-show-private-key.ci` |
| 3 | Read-only NOC user, default `admin` without explicit entry, or external plugin requests `show pki certificate name dev-1 bundle pem confirm` | Surface → `Dispatcher.dispatchLine` → `isAuthorizedExplicit` (deny) → `recordCommandAuditOutcome` (`denied`) → `ErrUnauthorized`, zero key bytes | `test/plugin/pki-show-private-key.ci`, `test/plugin/pki-certificate-export-show.ci` |
| 4 | Read-only NOC user runs `show pki certificate name dev-1 pem` | SSH/CLI → `Dispatcher.dispatchLine` → `isAuthorized` (`ReadOnly: true`, allow) → `handleShowPKICertificatePEM` → `certPEM` (cert only) | `test/plugin/pki-show-private-key.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthorizeExplicitDefaultDenyAndExplicitGrants` | `internal/component/authz/explicit_test.go` | Denies nil store, empty user, `default-action allow` (`admin`), `read-only`, `ReservedRecoveryProfile`, `ReservedSharedAPIUsername`, `ReservedInternalRPCIdentity`, plugin identities with absent or broad bus profiles, broad prefixes (`show`, `show pki`, `show pki certificate name`), and wildcard regex (`.*`); allows exact canonical path (`show pki certificate name bundle pem`), per-certificate path (`show pki certificate name dev-1 bundle pem`), plugin identity with an explicit compiled bus entry for `bundle pem`, targeted regex, and enforces deny-before-allow ordering | |
| `TestAuthorizeExplicitNilAndUnsupportedFailClosed` | `internal/component/aaa/local_scope_test.go` | `aaa.AuthorizeExplicit` fails closed on nil authorizer, legacy authorizer without `ExplicitAuthorizer`, and retired `acceptedGenerationAuthorizer` (including `a.next == nil`) | |
| `TestTacacsAuthorizerAuthorizeExplicit` | `internal/component/tacacs/authorizer_fleet_scope_test.go` | TACACS+ `AuthorizeExplicit` sends RFC 8907 AUTHOR REQUEST and falls back strictly to explicit local policy (or denies on `strictFallback` / nil local) | |
| `TestHubAuthorizerWrappersAuthorizeExplicit` | `cmd/ze/hub/aaa_lifecycle_fleet_scope_test.go` | All six hub authorizer wrappers delegate `AuthorizeExplicit`, fail closed when no store/bundle is installed, and never allow via `ReservedRecoveryProfile` | |
| `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput` | `internal/component/pki/show_test.go`, `internal/component/pki/store_test.go` | Missing `confirm` returns operator refusal with zero key material; confirmed call returns unchanged cert-before-PKCS#8-key PEM; `pem` remains unguarded and cert-only; CA and keyless cert still refuse cleanly | |
| `TestDispatcherExplicitAuthzAndRedactedAudit` | `internal/component/plugin/server/command_audit_test.go` | `RequireExplicitAuthz` command denies nil authorizer, `default-action allow`, `wrapHandler` (`internalRPCIdentity`), and `handleArgComplete`; records `OutcomeDenied` on authz/confirm refusal and `OutcomeSuccess` on confirmed authorized run with zero private-key bytes in audit entries | |
| `TestUsageRendersMandatoryFlagWithoutBrackets` | `internal/component/command/usage_test.go`, `internal/component/plugin/server/usage_model_test.go` | Mandatory `ArgFlag` renders as `confirm` (no brackets), optional `ArgFlag` stays `[flag]`, and `show pki certificate name bundle pem` renders `show pki certificate name <name> bundle pem confirm` | |
| `TestRedactPrivateKeyPEMAndTranscript` | `internal/core/redact/redact_test.go`, `internal/component/cli/transcript_test.go` | `redact.PrivateKeyPEM` redacts PKCS#8/RSA/EC/OPENSSH/truncated private-key PEM blocks while preserving `CERTIFICATE` blocks; `TranscriptWriter.Record` never writes private-key bytes to disk | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs introduced) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-show-private-key` | `test/plugin/pki-show-private-key.ci` | Authenticated daemon with RBAC profiles (`admin` default-allow only, `read-only`, `pki-exporter` with explicit `edit` allow): verifies authorized+confirmed `bundle pem confirm` returns cert+key bundle and writes redacted `pki-private-key-export` `success` audit record with zero key bytes; missing `confirm` is refused and audited as `denied`; `admin` (default-allow only) and `read-only` with `confirm` are refused and audited as `denied`; cert-only `pem` succeeds for `read-only` without `confirm`; zero private-key bytes appear in audit output or daemon logs | |
| `pki-certificate-export-show` | `test/plugin/pki-certificate-export-show.ci` | Updated existing test: external plugin still succeeds on `pem`, `fingerprint`, and `test-ca pem`, and is refused on `bundle pem` (with or without `confirm`) with zero private-key bytes returned | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - operational CLI/RBAC guard, not a wire-protocol feature | - | - | No wire protocol behavior changes | - |

## Files to Modify

- `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - add mandatory `leaf confirm { type empty; mandatory true; }` on `show pki certificate name bundle pem`.
- `internal/component/command/usage.go` - render mandatory `ArgFlag` leaves as `UsageKeyword` (no brackets) in `usageLeafKind`.
- `internal/component/command/usage_test.go` - unit test mandatory vs optional `ArgFlag` rendering.
- `internal/component/aaa/command_args.go` - add `ExplicitAuthorizer` interface and fail-closed `AuthorizeExplicit` helper.
- `internal/component/aaa/login_profiles.go` - implement `AuthorizeExplicit` on `acceptedGenerationAuthorizer` (fail-closed on `a.next == nil` or retired generation).
- `internal/component/aaa/local_scope_test.go` - unit tests for `AuthorizeExplicit` fail-closed behavior.
- `internal/component/authz/authz.go` - add `Store.AuthorizeExplicit` and `Store.AuthorizeExplicitWithProfiles` enforcing default-deny explicit `Entry` matching.
- `internal/component/authz/register.go` - implement `AuthorizeExplicit` on `StoreAuthorizer` and `boundStoreAuthorizer`.
- `internal/component/tacacs/authorizer.go` - implement `AuthorizeExplicit` on `*tacacsAuthorizer`.
- `internal/component/tacacs/authorizer_fleet_scope_test.go` - unit tests for TACACS+ `AuthorizeExplicit` and explicit local fallback.
- `cmd/ze/hub/aaa_lifecycle.go` - implement `AuthorizeExplicit` on all six hub authorizer wrappers.
- `cmd/ze/hub/aaa_lifecycle_fleet_scope_test.go` - unit tests for hub wrapper `AuthorizeExplicit` fail-closed behavior.
- `internal/component/plugin/server/handler.go` - add `RequireExplicitAuthz bool` to `RPCRegistration`.
- `internal/component/plugin/server/command.go` - propagate `RequireExplicitAuthz`, enforce `isAuthorizedExplicit` in `Dispatcher.dispatchLine`, and record redacted `OutcomeDenied`/`OutcomeSuccess`/`OutcomeError` audit entries for explicit-authz audited commands.
- `internal/component/plugin/server/server.go` - enforce `isAuthorizedExplicit` in `wrapHandler` when `reg.RequireExplicitAuthz` is true.
- `internal/component/plugin/server/system.go` - enforce `isAuthorizedExplicit` in `handleArgComplete` when `builtin.RequireExplicitAuthz` is true.
- `internal/component/plugin/server/command_audit_test.go` - unit tests for `RequireExplicitAuthz` enforcement and redacted audit recording.
- `internal/component/plugin/server/usage_model_test.go` - update expected usage line for `show pki certificate name bundle pem` to include `confirm`.
- `internal/component/pki/show.go` - set `NotReadOnly: true`, `RequireExplicitAuthz: true`, `AuditAction: audit.ActionPKIPrivateKeyExport` on `ze-pki:show-certificate-bundle-pem`; check `validated.Flag("confirm")` as defense-in-depth and read `validated.PositionalTokens()`.
- `internal/component/pki/show_test.go` - add unit tests for `bundle pem` confirmation refusal, confirmed output, and unguarded cert-only `pem`.
- `internal/component/pki/store_test.go` - update direct `handleShowPKICertificateBundlePEM` calls to pass `confirm` where testing bundle output and add missing-`confirm` refusal assertion.
- `internal/core/redact/redact.go` - add fail-closed `PrivateKeyPEM` and invoke it in `Command` and `redactValue`.
- `internal/core/redact/redact_test.go` - unit tests for `PrivateKeyPEM` across PKCS#8, RSA, EC, OpenSSH, truncated blocks, and certificate preservation.
- `internal/component/cli/transcript.go` - redact `output` with `redact.PrivateKeyPEM` in `TranscriptWriter.Record`.
- `internal/component/cli/transcript_test.go` - unit test proving private-key PEM blocks are redacted in transcripts while certificates remain visible.
- `internal/test/fixture/plugin_fixture_10.go` - update `fixture10PKIExport` so the external plugin verifies `bundle pem` is refused without key leakage while `pem` and `fingerprint` remain passing.
- `docs/architecture/pki/pki-store.md` - document the default-deny explicit RBAC + `confirm` guard and redacted `pki-private-key-export` audit trail on `bundle pem`.
- `docs/architecture/api/commands.md` - update `ze-pki:show-certificate` / `bundle pem` table entry to show `confirm` and the explicit RBAC guard, and note mandatory `ArgFlag` usage rendering.
- `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - document `redact.PrivateKeyPEM` and transcript output redaction.
- `docs/guide/command-reference.md` - update `show pki certificate name <name> bundle pem confirm` syntax and guard requirements.
- `docs/guide/authorization.md` - document explicit-grant default-deny commands (`RequireExplicitAuthz`) and the `edit` entry form for `show pki certificate name bundle pem`.
- `docs/guide/audit.md` - document `pki-private-key-export` (`success` and `denied` outcomes, redacted detail).

## Files to Create

- `internal/core/audit/action_pki_private_key.go` - `ActionPKIPrivateKeyExport = "pki-private-key-export"` constant.
- `internal/component/authz/explicit_test.go` - unit tests for `Store.AuthorizeExplicit` and `Store.AuthorizeExplicitWithProfiles`.
- `test/plugin/pki-show-private-key.ci` - functional test proving the full explicit-RBAC + `confirm` + redacted-audit matrix end to end.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` adds mandatory `leaf confirm { type empty; mandatory true; }` on `bundle/pem`. |
| YANG validation constraints | Yes | `type empty` with `mandatory true` is the tightest native YANG constraint for a required presence keyword. |
| YANG custom validators | No | Native `type empty` + `mandatory true` requires no custom validator. |
| CLI commands/flags | No | No offline `cmd/ze/` `--flag` added; `confirm` is a YANG command keyword. |
| CLI grammar (keyword before value) | Yes | `confirm` is a trailing keyword after `bundle pem` (`show pki certificate name <name> bundle pem confirm`), checked by `./le cli grammar`. |
| Editor autocomplete | Yes | Automatic from `ArgFlag` in `internal/component/command/completer.go`. |
| Functional test for new RPC/API | Yes | `test/plugin/pki-show-private-key.ci` plus updated `test/plugin/pki-certificate-export-show.ci`. |
| Pipe completeness | Yes | Existing `show pki certificate name <name> bundle pem confirm` continues to return `plugin.Map` (`ResponseData`) routed through `ApplyPipes`. |
| Env var registration | No | No environment variables added or changed. |
| Doctor check for runtime dependencies | No | No new runtime dependency (file, socket, binary, kernel feature, or port). |
| Doctor code per problem class | No | Uses existing `doctor-audit-record-failed` for audit write errors; authz and confirmation refusals are normal operator refusals, not `Warn`/`Error` daemon faults. |
| Prometheus counters/metrics | No | No new metrics added; existing audit and command accounting cover observability. |
| BGP family surface (new SAFI / capability / attribute) | N-A | PKI/AAA operational command guard; unrelated to BGP families. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Release security fix guarding an existing command; no new feature entry in `features/`. |
| 2 | Config syntax changed? | No | Reuses existing `system.authorization.profile.<name>.edit.entry` syntax unchanged. |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No plugin added or removed; `pki-cmd` YANG leaf updated in place. |
| 6 | Has a user guide page? | Yes | `docs/guide/authorization.md`, `docs/guide/audit.md` |
| 7 | Wire format changed? | No | No protocol wire format changed. |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` SDK or process-protocol change. |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Not an RFC protocol change. |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and fixture registry. |
| 11 | Affects daemon comparison? | No | Operational RBAC guard on an existing Ze command. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/pki/pki-store.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` |
| 13 | Route metadata keys added/changed? | No | Unrelated to BGP route metadata. |
| 14 | Prometheus counters added/changed? | No | No counters added or changed. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Same wire method `ze-pki:show-certificate-bundle-pem` and command path remain registered. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors`: modified `// Design:` docs listed in Files to Modify (`docs/architecture/pki/pki-store.md`, `docs/architecture/api/commands.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`); unaffected `// Design:` docs checked and confirmed intact: `ai/patterns/registration.md` (AAA/RPC registration patterns unchanged), `docs/architecture/api/process-protocol.md` (process wire protocol and plugin-profile compilation unchanged), `docs/architecture/fleet-config.md` (managed fleet scope and TLS reload unchanged), `docs/architecture/core-design.md` (Section 19 boundaries and AAA chain design unchanged), `docs/architecture/aaa-tacacs.md` (RFC 8907 AUTHOR REQUEST wire format unchanged). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verified `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/architecture/pki/pki-store.md`, `docs/architecture/pki/tls-listeners.md` (`tls-listeners.md` describes the PEM block order returned by `bundle pem`, which is unchanged). |

## Design presentation

### What changes

`show pki certificate name <name> bundle pem` now requires two independent safeguards on every Ze surface (CLI, SSH, REST, gRPC, Web, MCP, wire RPC, and plugin dispatch): an explicit RBAC `authz` grant in a profile's `edit` section (default-deny: unconfigured RBAC, `default-action allow`, recovery break-glass, shared/no-auth API, internal RPC, broad prefixes, and plugins are all denied) AND the explicit mandatory `confirm` keyword (`show pki certificate name <name> bundle pem confirm`). Both successful exports and denied/unconfirmed attempts record a structured, redacted `pki-private-key-export` audit entry with zero private-key bytes, and CLI session transcripts redact PEM private-key blocks on output. Certificate-only `show pki certificate name <name> pem` stays unguarded and unchanged, and authorized+confirmed `bundle pem confirm` keeps its exact existing certificate-chain-plus-PKCS#8-key PEM format.

### Before and after

1. **Unconfigured RBAC, `read-only`, `admin` (`default-action allow` only), or external plugin runs `bundle pem`:**
   - Before: `show pki certificate name dev-1 bundle pem` returned `{"pem": "<CERTIFICATE block>\n<PRIVATE KEY block>"}` with no RBAC check, no confirmation, and no audit record.
   - After: `show pki certificate name dev-1 bundle pem confirm` is refused with `error: command restricted by access control`, returns zero key material, and records `{"action":"pki-private-key-export","outcome":"denied","detail":"show pki certificate name dev-1 bundle pem confirm",...}` in the audit log.
2. **Explicitly authorized operator omits `confirm`:**
   - Before: `show pki certificate name dev-1 bundle pem` immediately printed the private key.
   - After: `show pki certificate name dev-1 bundle pem` is refused with an operator error naming the missing `confirm` keyword, why private-key export requires explicit confirmation, and the exact command `show pki certificate name dev-1 bundle pem confirm`, with an audit entry recording `outcome: "denied"` and zero key bytes.
3. **Explicitly authorized operator includes `confirm`:**
   - Before: `show pki certificate name dev-1 bundle pem` printed the bundle with no audit entry.
   - After: `show pki certificate name dev-1 bundle pem confirm` (with an `edit` entry matching `show pki certificate name bundle pem` or `show pki certificate name dev-1 bundle pem`) returns the exact same `{"pem": "<cert chain + PKCS#8 PRIVATE KEY>"}` bundle, records `{"action":"pki-private-key-export","outcome":"success","detail":"show pki certificate name dev-1 bundle pem confirm",...}` with zero key bytes, and redacts any PEM private-key block if CLI session transcript recording is enabled.
4. **Certificate-only export (`pem`):**
   - Before: `show pki certificate name dev-1 pem` returned certificate(s) only, no private key, no guard.
   - After: Identical — `show pki certificate name dev-1 pem` succeeds for any `read-only` user without `confirm` and without an explicit bundle export grant.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Rely on `NotReadOnly: true` alone without `RequireExplicitAuthz` | Still fails open whenever `system.authorization` is unconfigured, whenever an `admin` profile uses `edit { default-action allow }`, for `ReservedRecoveryProfile`, for `ReservedSharedAPIUsername`, and for `internalRPCIdentity` — violating the owner's default-deny explicit-grant decision. |
| Hardcode `"ze-pki:show-certificate-bundle-pem"` or `bundle pem` inside `authz` or `Dispatcher` | Violates Ze's registration-over-hardcoding invariant (`ai/rules/plugins.md`, `ai/doors/system.md`) and would force duplicate hardcoding when `spec-pki-key-only-output.md` adds `private-key pem`. |
| Check `authz` directly inside `internal/component/pki/show.go` | Violates component boundary rules (`docs/architecture/core-design.md`, section 19: `pki` must not import `authz`) and duplicates authorization outside the single dispatcher chokepoint. |
| Model `confirm` as a separate sub-command container (`bundle pem confirm`) instead of a mandatory `type empty` leaf | Would change the wire command path and force REST/gRPC/MCP/Web clients to call a different endpoint path instead of passing the typed `confirm` flag parameter already supported by `api/prepareCommand`, MCP, Web, and `ValidateArgs`. |
| Interactive `[y/N]` stdin prompt for confirmation | Cannot work across non-interactive SSH exec, REST, gRPC, MCP, or Web surfaces; an explicit `confirm` keyword works identically across every Ze surface. |

### Documents departed from

none

### Choices for Thomas

none (Owner decision recorded on 2026-11-24 settled both the dual guard — explicit default-deny RBAC `authz` grant plus explicit `confirm` keyword — and redacted audit logging, as well as the 2026-10-22 split keeping `private-key pem` in `plan/post-release/features/spec-pki-key-only-output.md`.)

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — add mandatory `leaf confirm { type empty; mandatory true; }` to `ze-pki-cmd.yang`, `RequireExplicitAuthz` to `RPCRegistration`/`RegisterOptions`/`Command`, `ActionPKIPrivateKeyExport` in `internal/core/audit/action_pki_private_key.go`, and write failing wiring tests (`test/plugin/pki-show-private-key.ci`, `TestDispatcherExplicitAuthzAndRedactedAudit`).
   - Tests: `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci`
   - Files: `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang`, `internal/core/audit/action_pki_private_key.go`, `internal/component/plugin/server/handler.go`, `internal/component/plugin/server/command.go`, `internal/component/pki/show.go`
   - Verify: new tests fail against unguarded handler before guard implementation lands.
2. **Phase: Explicit Default-Deny RBAC Across AAA/Authz/TACACS/Hub** — implement `aaa.ExplicitAuthorizer` / `aaa.AuthorizeExplicit`, `authz.Store.AuthorizeExplicit` / `AuthorizeExplicitWithProfiles`, `StoreAuthorizer` & `boundStoreAuthorizer`, `tacacsAuthorizer`, `acceptedGenerationAuthorizer`, and all six hub wrappers in `cmd/ze/hub/aaa_lifecycle.go`.
   - Tests: `TestAuthorizeExplicitDefaultDenyAndExplicitGrants`, `TestAuthorizeExplicitNilAndUnsupportedFailClosed`, `TestTacacsAuthorizerAuthorizeExplicit`, `TestHubAuthorizerWrappersAuthorizeExplicit`
   - Files: `internal/component/aaa/command_args.go`, `internal/component/aaa/login_profiles.go`, `internal/component/authz/authz.go`, `internal/component/authz/register.go`, `internal/component/tacacs/authorizer.go`, `cmd/ze/hub/aaa_lifecycle.go`
   - Verify: all unit tests fail first, then pass under `-race`.
3. **Phase: Dispatcher Enforcement, Audit Outcomes, Usage Rendering & PKI Handler Defense-in-Depth** — enforce `isAuthorizedExplicit` in `Dispatcher.dispatchLine`, `Server.wrapHandler`, and `handleArgComplete`; record redacted `OutcomeDenied`/`OutcomeSuccess`/`OutcomeError` audit entries; render mandatory `ArgFlag` without brackets in `command/usage.go`; update `handleShowPKICertificateBundlePEM` with `NotReadOnly: true`, `RequireExplicitAuthz: true`, `AuditAction: audit.ActionPKIPrivateKeyExport`, `validated.Flag("confirm")` check, and `validated.PositionalTokens()`.
   - Tests: `TestUsageRendersMandatoryFlagWithoutBrackets`, `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput`, `TestDispatcherExplicitAuthzAndRedactedAudit`
   - Files: `internal/component/command/usage.go`, `internal/component/plugin/server/command.go`, `internal/component/plugin/server/server.go`, `internal/component/plugin/server/system.go`, `internal/component/pki/show.go`
   - Verify: unit tests pass and existing `bundle pem` tests updated to match the guarded contract.
4. **Phase: Private-Key PEM Redaction & Transcript Protection** — add fail-closed `redact.PrivateKeyPEM` in `internal/core/redact/redact.go`, invoke it in `redact.Command` and `redactValue`, and redact `output` in `TranscriptWriter.Record`.
   - Tests: `TestRedactPrivateKeyPEMAndTranscript`
   - Files: `internal/core/redact/redact.go`, `internal/component/cli/transcript.go`
   - Verify: redaction and transcript tests pass with zero private-key bytes written.
5. **Phase: Functional Tests & Documentation** — update `internal/test/fixture/plugin_fixture_10.go` (`test/plugin/pki-certificate-export-show.ci`), finish `test/plugin/pki-show-private-key.ci`, and update all documentation pages listed in the Documentation Update Checklist.
   - Tests: `test/plugin/pki-show-private-key.ci`, `test/plugin/pki-certificate-export-show.ci`, `test/ui/test-pki-certificate-forms-are-separate-commands.ci`
   - Files: `internal/test/fixture/plugin_fixture_10.go`, `test/plugin/pki-show-private-key.ci`, `docs/architecture/pki/pki-store.md`, `docs/architecture/api/commands.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`, `docs/guide/command-reference.md`, `docs/guide/authorization.md`, `docs/guide/audit.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-4, AC-5, AC-3-bundle, and AC-6 each have an implementation and passing test; `private-key pem` is NOT added (remains owned by `spec-pki-key-only-output.md`) |
| Feature completeness | Every user story in End-to-End User Stories has a connected path and passing `.ci` proof |
| Correctness | Exact canonical path and per-certificate `edit` allow entries grant `bundle pem confirm`; `default-action allow`, broad prefixes, wildcard regex, nil store, recovery profile, shared API, internal RPC, and plugin endpoints without an explicit compiled bus grant all deny; deny-before-allow precedence holds |
| Naming | Audit action `pki-private-key-export` and JSON/YANG names are lowercase kebab-case; YANG `leaf confirm` matches `validated.Flag("confirm")` |
| Data flow | `pki` does not import `authz` or `tacacs`; all authorization flows through `aaa.ExplicitAuthorizer` at `Dispatcher.dispatchLine`, `Server.wrapHandler`, and `handleArgComplete` |
| Operator errors and doctor codes | Missing `confirm` returns an `opererr` naming the missing keyword, why it matters, and the exact command with `confirm`; authz refusal preserves `ErrUnauthorized` across HTTP/gRPC/MCP/SSH |
| Rule: `ai/rules/plugins.md` | Zero hardcoded `pki` or `bundle pem` command names in `plugin/server`, `authz`, `aaa`, `api`, `web`, or `mcp` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Explicit default-deny RBAC in `aaa`/`authz`/`tacacs`/`hub` | `./le test unit pkg path ./internal/component/aaa/` and `./internal/component/authz/` and `./internal/component/tacacs/` and `./cmd/ze/hub/` |
| Mandatory `confirm`, usage line, and PKI handler guard | `./le test unit pkg path ./internal/component/command/` and `./internal/component/pki/` and `./internal/component/plugin/server/` |
| PEM private-key redaction in `redact` and transcripts | `./le test unit pkg path ./internal/core/redact/` and `./internal/component/cli/` |
| End-to-end operator + plugin + UI functional proof | `./le test functional plugin` (covering `pki-show-private-key.ci` and `pki-certificate-export-show.ci`) and `./le test functional ui` |
| Grammar, architecture, YANG contract, and doc gates | `./le cli grammar`, `./le arch tier check`, `./le doc yang-contract command-contract`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed authorization | Nil authorizer, nil `authz.Store`, unconfigured `system.authorization`, `default-action allow`, `ReservedRecoveryProfile`, `ReservedSharedAPIUsername`, `ReservedInternalRPCIdentity`, plugin endpoints lacking an explicit compiled bus grant, and legacy `aaa.Authorizer` implementations without `ExplicitAuthorizer` all return `false` |
| Confirmation bypass | Missing `confirm`, `confirm: false` over REST/gRPC/MCP/Web, or direct handler invocation without `validated.Flag("confirm")` all fail closed before reading or marshalling private-key bytes |
| Broad grant prevention | Prefix entries shorter than the canonical command (`show`, `show pki`, `show pki certificate`, `show pki certificate name`) and wildcard regexes (`.*`) do not satisfy `AuthorizeExplicit` |
| Secret leakage | Audit entries, command history, daemon logs, error responses, and CLI transcripts contain zero private-key PEM bytes; truncated/unclosed PEM private-key blocks redact fail-closed |
| Secondary dispatch surfaces | `Server.wrapHandler` (wire RPC) and `handleArgComplete` (completion) both enforce `isAuthorizedExplicit` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE -->

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add `RequireExplicitAuthz` on `RPCRegistration` + `aaa.ExplicitAuthorizer` interface | Hardcoding `bundle pem` in `authz` or importing `authz` from `pki` | Keeps registration-over-hardcoding and component boundaries intact; directly reusable by `spec-pki-key-only-output.md` |
| Model `confirm` as `leaf confirm { type empty; mandatory true; }` in `ze-pki-cmd.yang` | Separate `bundle pem confirm` sub-command or interactive `[y/N]` prompt | Works uniformly across CLI, SSH, REST, gRPC, Web, and MCP via existing `ArgFlag` machinery with zero adapter edits |
| Evaluate explicit `Allow` against exact canonical path OR full typed command carrying all canonical tokens in order | Raw typed prefix only (breaks `match "show pki certificate name bundle pem"` due to inline `<name>`) or any matching prefix (allows broad `match "show"`) | Supports both all-cert (`show pki certificate name bundle pem`) and single-cert (`show pki certificate name dev-1 bundle pem`) explicit grants while blocking broad prefixes |
| Redact PEM private-key blocks in `internal/core/redact` and `TranscriptWriter.Record`, and audit both `success` and `denied` outcomes | Auditing only successes or leaving transcript output unredacted | Guarantees private-key bytes never reach logs, history, or transcripts and gives operators audit visibility into denied key-extraction attempts |

## Known Limitations
- Key-only private-key output (`show pki certificate name <name> private-key pem`) is deliberately out of scope and owned by `plan/post-release/features/spec-pki-key-only-output.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-4, AC-5, AC-3-bundle, AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A - none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A - operational CLI/RBAC guard)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-pki-show-private-key.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Added `audit.ActionPKIPrivateKeyExport = "pki-private-key-export"` in `internal/core/audit/action_pki_private_key.go`.
- Added mandatory `leaf confirm { type empty; mandatory true; }` on `show pki certificate name bundle pem` in `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang`, and updated `usageLeafKind` in `internal/component/command/usage.go` so mandatory `ArgFlag` leaves render as unbracketed `UsageKeyword` (`confirm`) while optional flags keep `[flag]`.
- Added `aaa.ExplicitAuthorizer` and fail-closed `aaa.AuthorizeExplicit` in `internal/component/aaa/command_args.go`, wired `acceptedGenerationAuthorizer` in `internal/component/aaa/login_profiles.go`, implemented default-deny explicit matching (`Store.AuthorizeExplicit`, `Store.authorizeExplicitWithProfiles`, `authorizePluginIdentityExplicit`, `StoreAuthorizer`, `boundStoreAuthorizer`) in `internal/component/authz/`, implemented RFC 8907 AUTHOR REQUEST with fail-closed local explicit fallback in `internal/component/tacacs/authorizer.go`, and implemented `AuthorizeExplicit` across all six hub wrappers in `cmd/ze/hub/aaa_lifecycle.go`.
- Added `RequireExplicitAuthz` on `RPCRegistration`, `RegisterOptions`, and `Command` in `internal/component/plugin/server/{handler.go, command.go}`, enforced `isAuthorizedExplicit` in `Dispatcher.dispatchLine`, `Server.wrapHandler`, and `handleArgComplete`, added `singleMissingArgOperatorError` for missing mandatory confirmation, and extended `recordCommandAudit`/`recordCommandAuditOutcome` to audit `OutcomeDenied`, `OutcomeError`, and `OutcomeSuccess` with redacted command detail and zero response payload bytes.
- Updated `ze-pki:show-certificate-bundle-pem` in `internal/component/pki/show.go` to declare `NotReadOnly: true`, `RequireExplicitAuthz: true`, and `AuditAction: audit.ActionPKIPrivateKeyExport`, added defense-in-depth `validated.Flag("confirm")` checking before key lookup/marshalling, and preserved `certBundlePEM` and unguarded certificate-only `handleShowPKICertificatePEM`.
- Added fail-closed `redact.PrivateKeyPEM` in `internal/core/redact/redact.go` (invoked by `redact.Command`, `redactValue`/`redact.JSON`, and `TranscriptWriter.Record` in `internal/component/cli/transcript.go`).
- Added unit tests across `aaa`, `authz`, `tacacs`, `cmd/ze/hub`, `command`, `pki`, `plugin/server`, `redact`, and `cli`; created `test/plugin/pki-show-private-key.ci`; updated `internal/test/fixture/plugin_fixture_10.go` (`test/plugin/pki-certificate-export-show.ci`); updated all six documentation pages and cleared external citers in `plan/post-release/features/spec-pki-key-only-output.md`; added news fragment `website/changes/fragments/pki-show-private-key.md`.

### Bugs Found/Fixed
- **Unwired exported symbol (`internal/component/authz/authz.go:571`):** `./le repo check` flagged `Store.AuthorizeExplicitWithProfiles` as having no cross-package non-test caller (only `boundStoreAuthorizer.AuthorizeExplicit` in same-package `register.go` calls it). Unexported to `authorizeExplicitWithProfiles`, covered by `TestAuthorizeExplicitDefaultDenyAndExplicitGrants` in `internal/component/authz/explicit_test.go`.
- **Adjacent malformed PEM header bypass (`internal/core/redact/redact.go:323`):** Input carrying a malformed non-key header adjacent to a `BEGIN PRIVATE KEY` block caused `PrivateKeyPEM` to advance past the real header delimiter, leaving private-key bytes unredacted. Fixed by advancing `pos = labelEnd` for non-key headers and `pos = endAfter - len(pemSuffix)` with `if beginIdx > written` guards; covered by the regression subtests in `TestRedactPrivateKeyPEMAndTranscript` (`internal/core/redact/redact_test.go`).

### Documentation Updates
- `docs/architecture/pki/pki-store.md` — documented explicit RBAC grant, mandatory `confirm`, and redacted `pki-private-key-export` audit trail (`<!-- source: internal/plugins/pki-cmd/yang/ze-pki-cmd.yang -- confirm leaf on bundle pem -->`, `<!-- source: internal/core/audit/action_pki_private_key.go -- ActionPKIPrivateKeyExport -->`).
- `docs/architecture/api/commands.md` — split `ze-pki:show-certificate*` table rows, documented `ze-pki:show-certificate-bundle-pem` (`<name> confirm`, `RequireExplicitAuthz`, `pki-private-key-export`), and documented mandatory `type empty` flag rendering (`<!-- source: internal/component/command/usage.go -- usageLeafKind -->` via existing anchors).
- `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` — documented fail-closed `redact.PrivateKeyPEM` and transcript output redaction (`<!-- source: internal/core/redact/redact.go -- ... PrivateKeyPEM ... -->`, `<!-- source: internal/component/plugin/server/command.go -- ... recordCommandAuditOutcome -->`).
- `docs/guide/command-reference.md` — updated `show pki certificate name <name> bundle pem confirm` syntax, guard, and audit description.
- `docs/guide/authorization.md` — added `Explicit-grant commands (default-deny)` section with canonical and per-certificate `edit` profile examples and TACACS+ explicit fallback (`<!-- source: internal/component/authz/authz.go -- ... Store.AuthorizeExplicit -->`, `<!-- source: internal/component/aaa/command_args.go -- ... ExplicitAuthorizer, AuthorizeExplicit -->`, `<!-- source: internal/component/plugin/server/command.go -- ... isAuthorizedExplicit -->`).
- `docs/guide/audit.md` — added `pki-private-key-export` row and documented `denied`/`error` audit recording for `RequireExplicitAuthz` commands (`<!-- source: internal/core/audit/action_pki_private_key.go -- ActionPKIPrivateKeyExport -->`, `<!-- source: internal/component/plugin/server/command.go -- recordCommandAudit, recordCommandAuditOutcome -->`).
- `./le doc check all`: zero errors on all 6 modified docs and `ze-pki-cmd.yang`; `./le doc ste check` 0 STE growth on all 6 docs and `website/changes/fragments/pki-show-private-key.md` (`ste check: OK`).

### Deviations from Plan
- Unexported `Store.AuthorizeExplicitWithProfiles` to `Store.authorizeExplicitWithProfiles` in `internal/component/authz/authz.go` during Run 1 review so `authz` exposes no unwired cross-package export (`./le repo check` clean); all cross-package callers continue to go through `aaa.AuthorizeExplicit((authz.StoreAuthorizer{...}).BindProfiles(...), ...)`.
- Added `docs/architecture/diagnostics/production-diagnostics.md` to `## Design documents` after commit `ebe5ba49aef` anchored that page to `internal/component/plugin/server/command.go`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `Store.AuthorizeExplicitWithProfiles` was exported mirroring `Store.AuthorizeWithProfiles` | Only same-package `boundStoreAuthorizer.AuthorizeExplicit` calls it; cross-package callers use `aaa.ExplicitAuthorizer` | `./le repo check` during Run 1 review | Unexported to `authorizeExplicitWithProfiles` in `authz.go`, `register.go`, and `explicit_test.go` |
| approach | `redact.PrivateKeyPEM` advanced `pos = headerEnd` on any non-key BEGIN header match | A malformed adjacent BEGIN header before `BEGIN PRIVATE KEY` made `labelEnd` land on the opening dashes of the real private-key header, skipping redaction | Run 1 & Run 2 adversarial security review of `redact.go` | Advanced `pos = labelEnd` for non-key headers and `pos = endAfter - len(pemSuffix)` for adjacent END-to-BEGIN blocks, plus regression tests in `redact_test.go` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Explicit default-deny RBAC (`RequireExplicitAuthz`, `NotReadOnly`, `aaa.ExplicitAuthorizer`) across all surfaces and all 10 authorizers | Done | `internal/component/aaa/command_args.go:66-85`, `login_profiles.go:136-152`, `internal/component/authz/authz.go:279-387,565-686,883-942`, `register.go:107-175`, `internal/component/tacacs/authorizer.go:151-169,338-354`, `cmd/ze/hub/aaa_lifecycle.go:434-1070`, `internal/component/plugin/server/{handler.go:397, command.go:1240-1261,1549-1562, server.go:287-300, system.go:604-611}` | Fail-closed on nil authorizer/store, `default-action allow`, recovery profile, shared API, internal RPC, broad prefixes, wildcard regexes, and plugins lacking explicit compiled bus grant |
| Mandatory `confirm` keyword in YANG, usage rendering, validator, and PKI handler defense-in-depth | Done | `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang:96-101`, `internal/component/command/usage.go:277-284`, `internal/component/plugin/server/command.go:1603-1609,1848-1870`, `internal/component/pki/show.go:40-46,228-281` | Validated by `ValidateArgs` and re-checked by `validated.Flag("confirm")` before reading/marshalling keys |
| Structured redacted audit (`audit.ActionPKIPrivateKeyExport`) on success/denied/error + PEM private-key redaction in `redact` and transcripts | Done | `internal/core/audit/action_pki_private_key.go:7`, `internal/component/plugin/server/command.go:1551,1591,1605,1639,1878-1930`, `internal/core/redact/redact.go:163,178-180,282-360`, `internal/component/cli/transcript.go:81` | Records redacted command line only (never response body); redacts closed/unclosed/JSON-escaped/adjacent-malformed PEM private-key blocks |
| Preserve certificate-only `pem` unguarded and preserve `bundle pem confirm` `{"pem": ...}` output shape | Done | `internal/component/pki/show.go:36-39,214-224,280` | `handleShowPKICertificatePEM` untouched; `certBundlePEM` unchanged |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-4 | Done | `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput`, `TestShowPKICertBundlePEM`, `TestShowPKICertBundlePEMRejectsCA`, `TestShowPKICertBundlePEMNoKey`, `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci` (steps 11-12) | Cert-before-PKCS#8-key PEM preserved; `pki-private-key-export` `success` (and `error` for CA/keyless) audited with zero key bytes |
| AC-5 | Done | `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput` (cert-only subtest), `test/plugin/pki-show-private-key.ci` (step 7), `test/plugin/pki-certificate-export-show.ci` | `read-only` user and external plugin still retrieve certificate-only `pem` with no `confirm` and no explicit grant |
| AC-3-bundle | Done | `TestAuthorizeExplicitDefaultDenyAndExplicitGrants`, `TestAuthorizeExplicitNilAndUnsupportedFailClosed`, `TestTacacsAuthorizerAuthorizeExplicit`, `TestHubAuthorizerWrappersAuthorizeExplicit`, `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci` (steps 3-4, 8-10, 12), `test/plugin/pki-certificate-export-show.ci` | Every unauthorized or unconfirmed path fails closed with zero key material and `OutcomeDenied` audit |
| AC-6 | Done | `TestRedactPrivateKeyPEMAndTranscript` (`redact_test.go` + `transcript_test.go`), `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci` (steps 4, 12, 13) | Zero `PRIVATE KEY` or key base64 bytes in audit JSONL, command history, daemon recent logs, or CLI transcripts |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAuthorizeExplicitDefaultDenyAndExplicitGrants` | Done | `internal/component/authz/explicit_test.go:26` | Covers nil store, reserved identities, broad prefixes, wildcard regexes, canonical/per-cert grants, deny-before-allow, plugin bus profiles |
| `TestAuthorizeExplicitNilAndUnsupportedFailClosed` | Done | `internal/component/aaa/local_scope_test.go:226` | Covers nil, legacy, supported, and retired/nil-delegate `acceptedGenerationAuthorizer` |
| `TestTacacsAuthorizerAuthorizeExplicit` | Done | `internal/component/tacacs/authorizer_fleet_scope_test.go:361` | Covers RFC 8907 PASS_ADD/FAIL and fail-closed explicit local fallback |
| `TestHubAuthorizerWrappersAuthorizeExplicit` | Done | `cmd/ze/hub/aaa_lifecycle_fleet_scope_test.go:444` | Covers all 6 hub wrappers, recovery denial, nil store/bundle, retired generation |
| `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput` | Done | `internal/component/pki/show_test.go:177` + `store_test.go:658` | Covers RPC metadata, missing-confirm refusal, confirmed output, cert-only `pem`, CA/keyless refusals |
| `TestDispatcherExplicitAuthzAndRedactedAudit` | Done | `internal/component/plugin/server/command_audit_test.go:488` | Covers loader propagation, nil authorizer, default-allow, missing confirm, confirmed run, handler error, `wrapHandler`, `handleArgComplete` |
| `TestUsageRendersMandatoryFlagWithoutBrackets` | Done | `internal/component/command/usage_test.go:708` + `usage_model_test.go:136` | Covers mandatory unbracketed `confirm`, optional `[flag]`, and full PKI usage line |
| `TestRedactPrivateKeyPEMAndTranscript` | Done | `internal/core/redact/redact_test.go:245` + `internal/component/cli/transcript_test.go:227` | Covers PKCS#8/RSA/EC/OpenSSH/encrypted/truncated/adjacent-malformed PEM blocks, `Command`, `JSON`, and `TranscriptWriter.Record` |
| `pki-show-private-key` | Done | `test/plugin/pki-show-private-key.ci` | 10/10 stress pass + promoted functional pass |
| `pki-certificate-export-show` | Done | `test/plugin/pki-certificate-export-show.ci` + `internal/test/fixture/plugin_fixture_10.go` | Verifies external plugin `pem`/`fingerprint`/`test-ca pem` pass and `bundle pem` / `bundle pem confirm` fail closed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/audit/action_pki_private_key.go` | Done | Created |
| `internal/component/authz/explicit_test.go` | Done | Created |
| `test/plugin/pki-show-private-key.ci` | Done | Created and promoted after 10/10 stress pass |
| `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` | Done | Modified |
| `internal/component/command/{usage.go, usage_test.go}` | Done | Modified |
| `internal/component/aaa/{command_args.go, login_profiles.go, local_scope_test.go}` | Done | Modified |
| `internal/component/authz/{authz.go, register.go}` | Done | Modified |
| `internal/component/tacacs/{authorizer.go, authorizer_fleet_scope_test.go}` | Done | Modified |
| `cmd/ze/hub/{aaa_lifecycle.go, aaa_lifecycle_fleet_scope_test.go}` | Done | Modified |
| `internal/component/plugin/server/{handler.go, command.go, server.go, system.go, command_audit_test.go, usage_model_test.go}` | Done | Modified (`command.go` via clean `hunks` patch isolating sibling-session hunks) |
| `internal/component/pki/{show.go, show_test.go, store_test.go}` | Done | Modified |
| `internal/core/redact/{redact.go, redact_test.go}` | Done | Modified |
| `internal/component/cli/{transcript.go, transcript_test.go}` | Done | Modified |
| `internal/test/fixture/plugin_fixture_10.go` | Done | Modified |
| `docs/architecture/{pki/pki-store.md, api/commands.md, ssh/fixit-bcrypt-hash-credential.md}` | Done | Modified (`commands.md` via clean `hunks` patch isolating sibling-session hunk) |
| `docs/guide/{command-reference.md, authorization.md, audit.md}` | Done | Modified |
| `plan/post-release/features/spec-pki-key-only-output.md` | Done | Cleared stale path citations to bare stem `spec-pki-show-private-key` |
| `website/changes/fragments/pki-show-private-key.md` | Done | Created news fragment (`ste check: OK`) |

### Audit Summary
- **Total items:** 35 planned files + 2 closure companion files (citer cleanup + news fragment), 4 ACs, 10 TDD tests
- **Done:** All items
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 minor items recorded in Deviations (`authorizeExplicitWithProfiles` unexported; `production-diagnostics.md` added to `## Design documents`)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Explicit default-deny RBAC (`RequireExplicitAuthz` + `NotReadOnly` + `aaa.ExplicitAuthorizer`) enforced across all surfaces and all 10 authorizers | unit (`-race`) + functional `.ci` | `TestAuthorizeExplicitDefaultDenyAndExplicitGrants`, `TestAuthorizeExplicitNilAndUnsupportedFailClosed`, `TestTacacsAuthorizerAuthorizeExplicit`, `TestHubAuthorizerWrappersAuthorizeExplicit`, `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci` (10/10 stress pass + PASS 9.2s), `test/plugin/pki-certificate-export-show.ci` (PASS 1.2s) |
| Mandatory `confirm` keyword in YANG + validator + PKI handler defense-in-depth | unit (`-race`) + functional `.ci` + contract gates | `TestUsageRendersMandatoryFlagWithoutBrackets`, `TestUsageRendersTheDeclaredValues`, `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput`, `test/plugin/pki-show-private-key.ci` (step 10 refusal naming `confirm` and exact command), `test/ui/test-pki-certificate-forms-are-separate-commands.ci` (PASS 2.8s), `./le cli grammar` OK, `./le doc yang-contract command-contract` + `usage-contract` OK |
| Redacted `pki-private-key-export` audit on allowed/denied/error outcomes + PEM private-key redaction in `redact` and CLI transcripts | unit (`-race`) + functional `.ci` | `TestDispatcherExplicitAuthzAndRedactedAudit`, `TestRedactPrivateKeyPEMAndTranscript` (`redact_test.go` + `transcript_test.go`), `test/plugin/pki-show-private-key.ci` (steps 4, 12, 13 verifying `denied` + `success` audit entries and daemon logs carry zero `PRIVATE KEY` or base64 key bytes) |
| Preserve certificate-only `pem` unguarded and preserve `bundle pem confirm` `{"pem": ...}` output shape | unit (`-race`) + functional `.ci` | `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput`, `TestShowPKICertBundlePEM`, `test/plugin/pki-show-private-key.ci` (steps 7, 11), `test/plugin/pki-certificate-export-show.ci` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| New key-only command `show pki certificate name <name> private-key pem` (original AC-1/AC-2 and AC-3 for key-only form) | Explicitly split out as a post-release feature on 2026-10-22 so this P0 release fix guards existing `bundle pem` exposure independently | `plan/post-release/features/spec-pki-key-only-output.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pki-show-private-key-b5c30003-3333-4000-8000-000000000003.md` |
| `./le spec review check` | `clean` (35 files/hunks, verdict=clean, rounds=3) |
| Rounds | 3 completed (Run 1: 2 ISSUEs fixed; Run 2: 2 `PrivateKeyPEM` edge-case ISSUEs fixed; Run 3: 1 test assertion fixed and verified CLEAN — 0 BLOCKER, 0 ISSUE) |
| Reviewer lenses used | logic+wiring, security+edge-cases, design-documents, style pass (`ze-go-style.md`), spec completeness (`/ze-review-spec`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Exported `Store.AuthorizeExplicitWithProfiles` had no cross-package non-test caller (`./le repo check` unwired export) | `internal/component/authz/authz.go:571` | Unexported to `Store.authorizeExplicitWithProfiles` and updated same-package caller in `register.go` and `explicit_test.go`; `./le repo check` now reports 0 findings on all spec files |
| 2 | ISSUE | Adjacent malformed header before `BEGIN PRIVATE KEY` bypassed `redact.PrivateKeyPEM` by consuming the real header's opening dashes | `internal/core/redact/redact.go:323` | Refined in Run 2 to advance `pos = labelEnd` (handling 5–9+ dash malformed BEGIN headers and repeated BEGIN prefixes without skipping real headers) |
| 3 | ISSUE | Run 2 caught 6–9 dash malformed BEGIN header and 5–9 dash adjacent `END`-to-`BEGIN` private-key boundaries bypassing `redact.PrivateKeyPEM` | `internal/core/redact/redact.go:327-351` | Set `pos = labelEnd` for non-key headers and `written = endAfter; pos = endAfter - len(pemSuffix)` with `if beginIdx > written` slice guards, plus 4 regression subtests in `internal/core/redact/redact_test.go` |
| 4 | ISSUE | Run 3 caught missing `FIRSTKEY` absence assertion in the adjacent `END`-to-`BEGIN` regression subtest | `internal/core/redact/redact_test.go:292-306` | Added `assert.NotContains(t, out, "FIRSTKEY", ...)` and 6-dash adjacent `END`-to-`BEGIN` subtest; verified CLEAN in Round 3 and `-race` PASS |

### Style Pass & Notes (Run 1)
- Style pass (`docs/contributing/ze-go-style.md`) run over all changed Go files: 0 `panic()` reachable from sockets or anywhere in diff, all loops strictly bounded (`PrivateKeyPEM` advances `pos` monotonically on every `continue`), 0 `fmt.Sprintf` in product code, names describe concepts, return types minimal, 0 hardcoded `pki`/`bundle pem` in generic infrastructure.
- NOTE: `singleMissingArgOperatorError` appends missing `usage` after `args` when extra positional tokens exist alongside missing `confirm` (harmless; handler rejects extra positional tokens via `unexpectedAfter`).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/audit/action_pki_private_key.go` | Yes | `-rw-r--r-- 1 thomas thomas 316 Nov 27 15:11 internal/core/audit/action_pki_private_key.go` |
| `internal/component/authz/explicit_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 14980 Nov 27 18:19 internal/component/authz/explicit_test.go` |
| `test/plugin/pki-show-private-key.ci` | Yes | `-rw-r--r-- 1 thomas thomas 9635 Nov 27 17:36 test/plugin/pki-show-private-key.ci` |
| `test/plugin/pki-certificate-export-show.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3134 Nov 24 19:37 test/plugin/pki-certificate-export-show.ci` |
| `test/ui/test-pki-certificate-forms-are-separate-commands.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1399 Aug 30 22:52 test/ui/test-pki-certificate-forms-are-separate-commands.ci` |
| `website/changes/fragments/pki-show-private-key.md` | Yes | `-rw-r--r-- 1 thomas thomas 1293 Nov 27 18:20 website/changes/fragments/pki-show-private-key.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-4 | Authorized + confirmed `bundle pem confirm` returns cert-before-key PEM and audits `success` with zero key bytes | `internal/component/pki/show.go:40-46,235-281`; `TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput` + `TestDispatcherExplicitAuthzAndRedactedAudit` PASS; `test/plugin/pki-show-private-key.ci` steps 11-12 PASS (10/10 stress + 9.2s) |
| AC-5 | Certificate-only `pem` stays unguarded for `read-only` without `confirm` | `internal/component/pki/show.go:36-39,214-224` (no `NotReadOnly`/`RequireExplicitAuthz`); `test/plugin/pki-show-private-key.ci` step 7 PASS; `test/plugin/pki-certificate-export-show.ci` PASS (1.2s) |
| AC-3-bundle | Unauthorized or unconfirmed `bundle pem` fails closed with zero key bytes and audits `denied` | `internal/component/plugin/server/{command.go:1549-1609, server.go:287-300, system.go:604-611}`, `internal/component/authz/authz.go:565-686,883-942`; `TestAuthorizeExplicitDefaultDenyAndExplicitGrants` (`-race` PASS 34.6s), `TestAuthorizeExplicitNilAndUnsupportedFailClosed`, `TestTacacsAuthorizerAuthorizeExplicit`, `TestHubAuthorizerWrappersAuthorizeExplicit`, `TestDispatcherExplicitAuthzAndRedactedAudit`, `test/plugin/pki-show-private-key.ci` steps 3-4, 8-10, 12 |
| AC-6 | Audit entries, command history, daemon logs, and CLI transcripts contain zero private-key PEM bytes | `internal/core/redact/redact.go:163,178-180,282-362`, `internal/component/cli/transcript.go:81`; `TestRedactPrivateKeyPEMAndTranscript` (`-race` PASS 1.16s after Run 3 fixes + `transcript_test.go` PASS 2.18s), `test/plugin/pki-show-private-key.ci` steps 4, 12, 13 PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show pki certificate name <name> bundle pem confirm` (authorized user with explicit RBAC `edit` grant) | `test/plugin/pki-show-private-key.ci` (steps 11-12) | Yes — read file; runs real SSH/CLI daemon with `pki-exporter` profile, asserts `BEGIN CERTIFICATE.*BEGIN PRIVATE KEY` and `pki-private-key-export` `success` audit with zero key bytes |
| `show pki certificate name <name> bundle pem` (authorized user, missing `confirm`) | `test/plugin/pki-show-private-key.ci` (steps 10, 12) | Yes — read file; asserts exit 1, stderr names `confirm` and exact command, zero `PRIVATE KEY`, and `denied` audit |
| `show pki certificate name <name> bundle pem confirm` (unconfigured RBAC, `read-only`, `admin` default-allow, external plugin) | `test/plugin/pki-show-private-key.ci` (steps 3-4, 8-9, 12), `test/plugin/pki-certificate-export-show.ci` (`fixture10PKIExport`) | Yes — read both files; asserts refusal with zero `PRIVATE KEY` and `denied` audit |
| `show pki certificate name <name> pem` (`read-only`, no `confirm`) | `test/plugin/pki-show-private-key.ci` (step 7), `test/plugin/pki-certificate-export-show.ci` | Yes — read both files; asserts `BEGIN CERTIFICATE` and rejects `PRIVATE KEY` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `git grep -n 'func.*AuthorizeCommandArgs(' internal/ cmd/` matches all 10 authorizers; all 10 implement `AuthorizeExplicit` and pass unit tests + live SSH/CLI `.ci` |
| A-2 | confirmed | `unknownMatchEntries` (`internal/component/config/infra/authz.go:287-318`) checks `strings.HasPrefix(cmd, match) \|\| strings.HasPrefix(match, cmd)` accepting both canonical and per-certificate entries; live daemon boots `pki-rbac.conf` cleanly |
| A-3 | confirmed | `ze-pki-cmd.yang` `leaf confirm { type empty; mandatory true; }` passes `./le cli grammar`, `./le doc yang-contract command-contract` + `usage-contract`, `TestUsageRendersTheDeclaredValues`, and `test/plugin/pki-show-private-key.ci` |
| A-4 | confirmed | `TestUsageRendersTheDeclaredValues`, `TestUsageRendersMandatoryFlagWithoutBrackets`, and `test/ui/test-pki-certificate-forms-are-separate-commands.ci` PASS with zero unintended usage changes on other commands |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI command reference (`docs/guide/command-reference.md`) | Verified against `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` and `internal/component/pki/show.go:40-46` | Yes |
| API/RPC docs (`docs/architecture/api/commands.md`) | Verified against `internal/component/pki/show.go` and `internal/component/command/usage.go:277-284` (`hunks` patch verified against `HEAD`) | Yes |
| User guides (`docs/guide/authorization.md`, `docs/guide/audit.md`) | Verified against `authz/authz.go`, `aaa/command_args.go`, `plugin/server/command.go`, and `core/audit/action_pki_private_key.go`; all anchors resolve in `./le doc check all` | Yes |
| Internal architecture (`docs/architecture/pki/pki-store.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`) | Verified against `pki/show.go`, `ze-pki-cmd.yang`, `core/redact/redact.go`, and `cli/transcript.go`; all anchors resolve in `./le doc check all` and 0 STE growth | Yes |
| News fragment (`website/changes/fragments/pki-show-private-key.md`) | Verified against `internal/le/commit/newsfragment.go` and `./le doc ste check file` (`ste check: OK`) | Yes |
| External spec citers (`plan/post-release/features/spec-pki-key-only-output.md`) | `git grep -n "spec-pki-show-private-key"` shows 0 remaining external `plan/release/fixes/spec-pki-show-private-key.md` path citations | Yes |
