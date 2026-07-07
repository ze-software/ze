# Spec: radius-admin-backend

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

> **BLOCKING PRODUCT DECISION (A-1) before this leaves `design`:** the current empty
> `radiusBackend.Build()` is a *deliberate* placeholder (`internal/component/radius/aaa.go:17-24`),
> not an oversight. Admin-auth-via-RADIUS must be confirmed as wanted before promoting this
> spec to `ready`/implementation. Many deployments deliberately use TACACS+ for device admin
> and RADIUS only for subscribers. Do not implement until A-1 is resolved with the user.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` — workflow rules.
3. `plan/learned/601-tacacs.md` (the reference backend), `plan/learned/598-aaa-registry.md` (the registry pattern), `plan/learned/658-l2tp-8b-radius.md` (the L2TP path that must not break).
4. Source: `internal/component/radius/aaa.go`, `internal/component/tacacs/register.go`, `internal/component/aaa/aaa.go`, `internal/component/aaa/types.go`.

## Task

Replace the placeholder `radiusBackend.Build()` (`internal/component/radius/aaa.go:22`, currently
returns an empty `aaa.Contribution`) with a real AAA backend that authenticates **operator/admin
logins** (SSH, web, MCP) against a RADIUS server, mirroring the existing TACACS+ backend. Add a
system-level `system/authentication/radius` YANG subtree, a config extractor, and a RADIUS
`Authenticator` that reuses the existing `internal/component/radius` client. The L2TP subscriber
RADIUS path (`internal/component/l2tp/plugins/authradius/`) must remain byte-for-byte unchanged.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/601-tacacs.md` — TACACS+ AAA (RFC 8907) admin backend; the exact template to copy.
  → Constraint: an admin-auth backend registers in `init()` via `aaa.Default.Register`, implements `Build(BuildParams) (Contribution, error)`, and returns an `Authenticator` only when servers are configured.
- [ ] `plan/learned/598-aaa-registry.md` — pluggable AAA backend registry.
  → Constraint: the registry orders backends by `Priority()` (lower = earlier), chains all contributed `Authenticator`s, and keeps only the FIRST contributed `Authorizer`/`Accountant`.
- [ ] `plan/learned/658-l2tp-8b-radius.md` — the L2TP RADIUS auth/acct/CoA plugin.
  → Constraint: L2TP RADIUS is a separate out-of-process plugin under the `l2tp` YANG root; the new admin backend is a distinct in-process backend and must not touch it.
- [ ] `plan/learned/600-user-login.md`, `plan/learned/780-rbac-audit.md` — local login baseline + RBAC/profile semantics the RADIUS profiles feed.
- [ ] `docs/research/l2tpv2-ze-integration.md` — the `// Design:` pointer on `radius/aaa.go`; explains the original L2TP-only rationale for RADIUS.

### RFC Summaries
- [ ] `rfc/short/rfc2865.md` — RADIUS. Access-Request/Accept/Reject, User-Password hiding (§5.2), NAS-* attributes, Filter-Id (§5.11), Service-Type (§5.6). Verify a summary exists; create via `/ze-rfc` if missing.
  → Constraint: User-Password MUST be hidden per §5.2 (the client's `Exchange` already does this — `internal/component/radius/client.go:116-193`).

**Key insights:** TACACS+ is a complete working precedent for every surface (backend register, own YANG under `system/authentication`, config extractor, authenticator mapping PASS/FAIL/ERROR → `AuthResult`/`ErrAuthRejected`/infra-error). RADIUS has the client but neither a system-AAA YANG nor a `config.go`. The only genuinely new design decisions vs TACACS+ are (a) auth method (PAP/User-Password), (b) how Access-Accept reply attributes map to ze authz profiles, and (c) chain priority.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/radius/aaa.go` — registers `radiusBackend` (name `"radius"` at `:9`, priority `50` at `:10`) in `aaa.Default` via `register.go:13-18`; `Build` returns an empty `Contribution` (`:22-24`), so RADIUS contributes NO authenticator to admin login today.
  → Constraint: the name/priority slot is already reserved; the spec replaces the empty `Build`, it does not re-register.
- [ ] `internal/component/aaa/aaa.go` — `Authenticator.Authenticate(AuthRequest) (AuthResult, error)` (`:22-24`); `ErrAuthRejected` (`:15`) semantics (`:17-21`): `nil`=success, `ErrAuthRejected`=explicit reject (chain stops), other error=infra failure (chain tries next). `Backend` (`:42-46`).
- [ ] `internal/component/aaa/types.go` — `BuildParams` (`:42-53`), `Contribution` (`:58-63`), `AuthRequest` (`:95-100`), `AuthResult` (`:88-92`), `ChainAuthenticator` (`:107-131`), `BackendRegistry.Build` (`:212-263`), priority ordering (`:199-208`), first-authorizer-wins (`:231-248`).
- [ ] `internal/component/tacacs/register.go` — reference `Build` (`:24-71`): `ExtractConfig`, empty Contribution when no servers, else `NewTacacsClient` + `NewTacacsAuthenticator`, optional Authorizer/Accountant, Close hook.
- [ ] `internal/component/tacacs/authenticator.go` — reference `Authenticate` (`:43-74`) mapping client result → `AuthResult`; priv-lvl→profile in `handlePass` (`:77-103`).
- [ ] `internal/component/tacacs/config.go` — `ExtractConfig(tree)` (`:36-109`) reading `system.authentication.tacacs`.
- [ ] `internal/component/tacacs/yang/ze-tacacs-conf.yang` — `system/authentication/tacacs` shape (`:14-93`): `list server{address,port,key[ze:sensitive]}`, `timeout`, `source-address`, `authorization`, `strict-fallback`, `accounting`, `tacacs-profile` map.
- [ ] `internal/component/radius/client.go` — `Server{Address,SharedKey}` (`:20-23`), `ClientConfig` (`:26-32`), `NewClient` (`:58-88`), `Exchange` (`:116-193`, single server, retransmit + User-Password hide), `SendToServers` (`:304-316`, failover). No convenience `Authenticate`: callers build an Access-Request `Packet`.
- [ ] `internal/component/radius/packet.go` / `attr.go` — `RandomAuthenticator` (`packet.go:31`), `FindAttr`/`FindAllAttr` (`packet.go:123,133`), `AttrString` (`attr.go:143`), `AttrUint32` (`attr.go:137`); codes in `dict.go`.
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` — reference caller: builds Access-Request `{Code, Authenticator, Attrs}` (`:78-91`), calls `SendToServers` (`:95`), branches Accept/Reject (`:112-148`), assembles NAS-*/User-*/CHAP attrs (`buildAuthAttrs`, `:153-202`). **This path must not change.**
- [ ] `cmd/ze/hub/infra_setup.go` — `buildAAABundle` (`:30-43`) → `aaa.Default.Build(params)` (`:42`); bundle's `Authenticator` handed to SSH (`:139-148`); live swap `swapAAABundle` (`:133`).
- [ ] Admin login call sites: `internal/component/ssh/ssh.go:431`, `internal/component/web/auth.go:191,235`, `internal/component/mcp/streamable.go:393` — all call `Authenticator.Authenticate(...)` on the composed chain.

**Behavior to preserve:**
- L2TP RADIUS subscriber auth (`authradius` plugin) unchanged: separate process, `l2tp` YANG root, its own `radius.NewClient`.
- Existing AAA chain semantics: priority ordering, chain-stops-on-reject, first-authorizer-wins, empty-Contribution-when-unconfigured.
- The reserved `radius` name and priority `50` slot (unless A-2 changes the priority deliberately).

**Behavior to change:**
- `radiusBackend.Build()` returns a real `Authenticator` when `system/authentication/radius` has servers; still empty when it has none.

## Data Flow (MANDATORY)

### Entry Point
- Operator SSH/web/MCP login attempt → `Authenticator.Authenticate(AuthRequest{Username,Password,RemoteAddr,Service})`.
- Config: `system/authentication/radius` YANG subtree in the config tree passed via `BuildParams.ConfigTree`.

### Transformation Path
1. Config tree → `radius.ExtractConfig(tree)` (new) → `ExtractedConfig{Servers,Timeout,Retries,SourceAddress,ProfileAttr,DefaultProfiles}`.
2. `radiusBackend.Build(params)` (new) → `radius.NewClient(ClientConfig)` + `newRadiusAuthenticator(client, cfg, logger)`; returns `Contribution{Authenticator, Close}`.
3. On login: authenticator builds an Access-Request `Packet` (User-Name + hidden User-Password + NAS attrs) and calls `client.SendToServers(ctx, pkt)`.
4. Access-Accept → `AuthResult{Authenticated:true, Profiles:<from reply attrs>, Source:"radius"}`; Access-Reject → `ErrAuthRejected`; timeout/socket error → infra error (chain continues).
5. Result flows back through `ChainAuthenticator` (`aaa/types.go:261`) to the SSH/web/MCP call site.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ backend | `ExtractConfig` reads `system.authentication.radius` | [ ] |
| Backend ↔ RADIUS server | UDP Access-Request/Access-Accept via `radius.Client` | [ ] |
| Backend ↔ AAA chain | `Contribution.Authenticator` joins `ChainAuthenticator` by priority | [ ] |

### Integration Points
- `aaa.Default` registry — already has the `radius` slot (`radius/register.go`).
- `radius.Client` — reused as-is (no client changes).
- `configyang.RegisterModule` — new `ze-radius-conf.yang` registration (parallel to `tacacs/yang/register.go:10`).

### Architectural Verification
- [ ] No bypassed layers (config → Build → client → chain).
- [ ] No unintended coupling (does not import or touch the L2TP `authradius` plugin).
- [ ] No duplicated functionality (reuses `radius.Client`; mirrors tacacs backend structure).
- [ ] Registration over hardcoding — backend already registers via `aaa.Default.Register`; new YANG via `configyang.RegisterModule`; no new switch/field in a core package.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Admin-auth-via-RADIUS is a wanted feature | User asked for a spec; but `radius/aaa.go:17-24` empty Contribution is deliberate | Whole spec is moot | **User confirmation (BLOCKING)** | unvalidated |
| A-2 | RADIUS should keep chain priority 50 (ahead of tacacs=100, local=200) | `radius/aaa.go:11`, `tacacs/register.go:20`, `authz/register.go:43-45` | Wrong auth precedence when both radius+tacacs configured | User confirmation; grep priorities | unvalidated |
| A-3 | PAP (RFC 2865 User-Password, hidden per §5.2) is the acceptable admin auth method | `client.go:116-193` already hides User-Password; simplest for device admin | Need CHAP/EAP instead → larger scope | User confirmation; `rfc/short/rfc2865.md` | unvalidated |
| A-4 | Access-Accept → ze profiles via a configurable reply attribute (default Filter-Id, RFC 2865 §5.11) with a default-profile fallback | Mirrors tacacs priv-lvl→profile (`authenticator.go:77-103`); Filter-Id is the RFC-standard authz carrier | Operators can't map RADIUS users to ze RBAC profiles | User confirmation; test with freeradius | unvalidated |
| A-5 | `radius.Client.SendToServers` is safe to call from the login goroutine and is concurrency-safe | `client.go:304-316` used the same way by L2TP `handler.go:95` | Races/deadlock on concurrent logins | Read client; `-race` unit test | unvalidated |
| A-6 | The AAA `BuildParams.ConfigTree` contains the full `system/authentication` subtree at Build time | tacacs reads it the same way (`config.go:36-109`) | Backend sees no config, stays empty | grep/read `buildAAABundle` (`infra_setup.go:30-43`) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Change accidentally alters the L2TP RADIUS path | L2TP RADIUS interop/functional tests fail | Keep all changes under `internal/component/radius/{aaa,config,authenticator,yang}.go`; do not touch `l2tp/plugins/authradius/*`; run its tests |
| R-2 | Priority 50 makes RADIUS shadow local/tacacs unexpectedly | Login uses RADIUS when operator expected local-first | Resolve A-2; document the effective chain order in docs |
| R-3 | Shared secret leaked in logs/config dumps | Secret visible in `show config` or logs | Mark the YANG `key` leaf `ze:sensitive` (as tacacs does, `ze-tacacs-conf.yang:23-44`); never log it |
| R-4 | RADIUS server unreachable silently blocks admin login | Operators locked out | Return infra-error (not reject) so the chain falls through to local; add a `doctor-radius-admin-unreachable` check mirroring the L2TP plugin's `doctor-radius-unreachable` |
| R-5 | Timeout too long → slow login / DoS on the login goroutine | Login hangs | Bounded timeout + retries from config; sane defaults with `range` constraints |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system/authentication/radius` config with a server | → | `radiusBackend.Build` returns a non-empty `Contribution` | `TestRadiusBuildReturnsAuthenticatorWhenConfigured` |
| No `radius` config | → | `radiusBackend.Build` returns empty `Contribution` | `TestRadiusBuildEmptyWhenUnconfigured` |
| SSH login attempt with RADIUS configured | → | `radiusAuthenticator.Authenticate` sends Access-Request | `test/plugin/aaa-radius-admin.ci` (mock RADIUS server) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `system/authentication/radius` has ≥1 server | `Build` returns `Contribution` with a non-nil `Authenticator` (+ `Close`) |
| AC-2 | No radius servers configured | `Build` returns an empty `Contribution` (current behavior preserved) |
| AC-3 | Server returns Access-Accept | `Authenticate` → `AuthResult{Authenticated:true, Profiles:<mapped>, Source:"radius"}`, nil error |
| AC-4 | Server returns Access-Reject | `Authenticate` → `(zero, ErrAuthRejected)` so the chain stops |
| AC-5 | Server unreachable / timeout | `Authenticate` → `(zero, non-ErrAuthRejected error)` so the chain tries the next backend (local fallback) |
| AC-6 | Access-Accept carries the configured profile attribute | `AuthResult.Profiles` = attribute values; absent → configured default profiles |
| AC-7 | L2TP RADIUS subscriber auth | Unchanged: `authradius` plugin + its tests pass identically |
| AC-8 | `show config` / logs after RADIUS configured | Shared secret never rendered in cleartext (leaf `ze:sensitive`) |
| AC-9 | radius + tacacs + local all configured | Chain order matches the resolved A-2 decision, documented |
| AC-10 | Malformed / unparseable radius config | `ExtractConfig`/validate rejects with a clear error; no partial backend |

## End-to-End User Stories (MANDATORY)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | SSH admin logs in, RADIUS accepts | `ssh.go:431` → chain → `radiusAuthenticator.Authenticate` → `client.SendToServers` → Access-Accept → profiles | `test/plugin/aaa-radius-admin.ci` |
| 2 | RADIUS rejects, no fallthrough | login → radius → `ErrAuthRejected` → chain stops (no local attempt) | `TestRadiusRejectStopsChain` (+ `.ci`) |
| 3 | RADIUS unreachable, local fallback | login → radius infra-error → chain → local backend accepts | `test/plugin/aaa-radius-fallback.ci` |
| 4 | Web login via RADIUS | `web/auth.go:191` → same chain | covered by chain unit test + `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractRadiusConfig` | `internal/component/radius/config_test.go` | YANG subtree → `ExtractedConfig` (servers, timeout, retries, source-address, profile-attr, default-profiles) | |
| `TestRadiusBuildReturnsAuthenticatorWhenConfigured` | `internal/component/radius/aaa_test.go` | AC-1 | |
| `TestRadiusBuildEmptyWhenUnconfigured` | `internal/component/radius/aaa_test.go` | AC-2 | |
| `TestRadiusAuthenticateAccept` | `internal/component/radius/authenticator_test.go` | AC-3 (mock server/packet) | |
| `TestRadiusAuthenticateReject` | `internal/component/radius/authenticator_test.go` | AC-4 | |
| `TestRadiusAuthenticateInfraError` | `internal/component/radius/authenticator_test.go` | AC-5 | |
| `TestRadiusProfileMapping` | `internal/component/radius/authenticator_test.go` | AC-6 (Filter-Id → profiles + default) | |
| `TestRadiusSecretNotLogged` | `internal/component/radius/config_test.go` | AC-8 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| server port | 1..65535 | 65535 | 0 | 65536 |
| timeout (s) | 1..60 | 60 | 0 | 61 |
| retries | 0..10 | 10 | N/A | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-admin` | `test/plugin/aaa-radius-admin.ci` | Configure radius admin backend, login accepted, profile assigned | |
| `aaa-radius-fallback` | `test/plugin/aaa-radius-fallback.ci` | RADIUS unreachable → local fallback succeeds | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-radius-admin-freeradius` | `test/interop/scenarios/` | FreeRADIUS | Real Access-Request/Accept/Reject + Filter-Id profile mapping against a real RADIUS server | |

### Future (deferring)
- RADIUS **accounting** (Accounting-Request Start/Stop → an `Accountant` contribution) and **authorization** beyond profile mapping. Deferred to a phase 2; MVP is Authenticator-only. Requires user approval to defer.
- CHAP / EAP admin auth methods (A-3 assumes PAP).

## Files to Modify
- `internal/component/radius/aaa.go` — replace empty `Build` with a real one (config → client → authenticator + Close).
- `internal/component/radius/aaa.go` priority (`:10-11`) — only if A-2 changes it.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (system/authentication/radius) | Yes | `internal/component/radius/yang/ze-radius-conf.yang` (new), parallel to `tacacs/yang/ze-tacacs-conf.yang` |
| YANG validation constraints | Yes | `range` on port/timeout/retries; `ze:sensitive` on `key`; `ze-inet-address` type on address/source-address |
| YANG custom validators | Maybe | If profile-attr needs an enum of known RADIUS attribute names; else plain `enumeration` |
| CLI commands/flags | No (reuse) | Existing `aaa-cmd` show plugin surfaces backends; verify radius appears |
| CLI grammar | N/A | No new verbs |
| Editor autocomplete | Yes | Automatic for YANG enum/type leaves |
| Functional test for new behavior | Yes | `test/plugin/aaa-radius-admin.ci`, `aaa-radius-fallback.ci` |
| Env var registration | No | Config is YANG, not env |
| Doctor check for runtime dependency | Yes | RADIUS server reachability: `doctor-radius-admin-unreachable` in `internal/component/radius/` + code in `internal/core/diagnostic/codes.go` (mirror L2TP plugin's `doctor-radius-unreachable`) |
| Prometheus counters/metrics | Optional | radius admin auth accept/reject/timeout counters; defer with L2TP-parity if not in MVP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (system/authentication/radius) |
| 3 | CLI command added/changed? | No | verify `aaa` show already lists backends |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | radius is a component, not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/` AAA / authentication page (RADIUS admin section) |
| 7 | Wire format changed? | No | reuses existing RADIUS client |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2865.md` (ensure summary covers Access-Request/Accept/Reject, §5.2, Filter-Id) |
| 10 | Test infrastructure changed? | Maybe | `docs/functional-tests.md` if a mock RADIUS harness is added |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` (admin AAA methods) |
| 12 | Internal architecture changed? | Yes | AAA subsystem doc: RADIUS is now a real admin backend |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | If added | telemetry doc |
| 15 | Registered backend/inventory changed? | Yes | AAA backend inventory / `docs/guide/status.md` |
| 16 | Source anchors on changed files? | Yes | grep `docs/` for anchors on `radius/aaa.go` |
| 17 | Existing docs show config examples for this area? | Yes | verify tacacs auth examples; add radius equivalents |

## Files to Create
- `internal/component/radius/config.go` — `ExtractConfig(tree)` + `ExtractedConfig` struct + `HasServers()`.
- `internal/component/radius/authenticator.go` — `radiusAuthenticator` implementing `aaa.Authenticator`; builds Access-Request, maps Accept/Reject/error, maps reply attr → profiles.
- `internal/component/radius/yang/ze-radius-conf.yang` — `system/authentication/radius` module.
- `internal/component/radius/yang/register.go` + `embed.go` + `doc.go` — module registration (mirror `tacacs/yang/`).
- `internal/component/radius/config_test.go`, `aaa_test.go`, `authenticator_test.go`.
- `test/plugin/aaa-radius-admin.ci`, `test/plugin/aaa-radius-fallback.ci`.
- `test/interop/scenarios/NN-radius-admin-freeradius/` (config + check).
- Diagnostic code entry in `internal/core/diagnostic/codes.go`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-lint && ze-unit-test && ze-functional-test` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 14. Summary | Executive Summary |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — add `ze-radius-conf.yang` (system/authentication/radius) + `yang/register.go`; `ExtractConfig` skeleton; `Build` returns empty unless servers present.
   - Tests: `TestRadiusBuildEmptyWhenUnconfigured`, `TestRadiusBuildReturnsAuthenticatorWhenConfigured` (fails until authenticator exists).
   - Verify: config subtree parses; Build reachable from `aaa.Default.Build`.
2. **Phase: Config extraction** — `ExtractConfig` fully reads servers/timeout/retries/source-address/profile-attr/default-profiles; validation + boundary tests.
   - Tests: `TestExtractRadiusConfig`, boundary tests, `TestRadiusSecretNotLogged`.
3. **Phase: Authenticator** — `radiusAuthenticator.Authenticate`: build Access-Request (User-Name + hidden User-Password + NAS-Identifier/NAS-IP), `SendToServers`, map Accept→profiles / Reject→`ErrAuthRejected` / error→infra; profile mapping from reply attribute + default.
   - Tests: `TestRadiusAuthenticateAccept/Reject/InfraError`, `TestRadiusProfileMapping` (mock server).
4. **Phase: Build wiring + Close** — `Build` constructs `radius.NewClient` + authenticator, returns `Contribution{Authenticator, Close}`; Close drains the client.
5. **Phase: Doctor check** — `doctor-radius-admin-unreachable` + diagnostic code + unit/functional test.
6. **Functional + interop** — `.ci` mock-server tests; FreeRADIUS interop scenario.
7. **Docs** — features, config syntax, AAA subsystem doc, source anchors; `make ze-doc-test`.
8. **Complete spec** — audit tables, learned summary `plan/learned/NNN-radius-admin-backend.md`, two-commit close.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | Reject vs infra-error distinction correct (chain-stop vs fallthrough); User-Password hidden |
| L2TP untouched | `git diff` shows no change under `internal/component/l2tp/plugins/authradius/`; its tests pass |
| Naming | YANG kebab-case; JSON/profile names consistent with tacacs |
| Data flow | Config → Build → client → chain only; no coupling to L2TP plugin |
| Registration over hardcoding | Backend via `aaa.Default.Register` (existing), YANG via `configyang.RegisterModule` |
| Doctor checks | `doctor-radius-admin-unreachable` registered + diagnostic code |
| YANG validation | port/timeout/retries have `range`; `key` is `ze:sensitive`; addresses typed |
| Secret handling | secret never logged, never in `show config` cleartext |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Real `Build` | `grep -n 'Authenticator' internal/component/radius/aaa.go` shows contribution set |
| system radius YANG | `ls internal/component/radius/yang/ze-radius-conf.yang` |
| Authenticator | `go test ./internal/component/radius/ -run Authenticate` |
| L2TP unchanged | `git diff --stat internal/component/l2tp/plugins/authradius/` empty |
| Doctor check | `grep radius-admin-unreachable internal/core/diagnostic/codes.go` |
| Functional | `.ci` files exist and pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret handling | Shared secret `ze:sensitive`, never logged, not in config dump |
| User-Password hiding | RFC 2865 §5.2 applied (client `Exchange`); no cleartext password on the wire |
| Response validation | Verify Access-Accept Response Authenticator (MD5) before trusting it (check `client.go` does this; if not, add) |
| Injection | Username/attrs length-checked before packet build; bounded allocations |
| DoS / lockout | Bounded timeout+retries; unreachable server → fallthrough, not hang; no infinite retry |
| Info leakage | Reject reason not leaked to unauthenticated client beyond generic failure |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in introducing phase |
| Reject/infra mis-mapped | Re-read `aaa.go:17-21` semantics |
| L2TP test breaks | Revert coupling; keep changes radius-local |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mirror the TACACS+ backend structure (own YANG under system/authentication, ExtractConfig, Authenticator) | Extend the L2TP radius plugin to also do admin auth | TACACS+ is the proven admin-auth precedent; L2TP plugin is out-of-process and l2tp-scoped, wrong tier for admin login |
| Reuse `internal/component/radius` client unchanged | New admin-specific RADIUS client | Client already supports Access-Request/failover/User-Password hiding; avoid duplication |
| PAP (User-Password) for MVP | CHAP / EAP | Simplest standard device-admin method; client already hides User-Password; CHAP/EAP deferrable (A-3) |
| Profiles from a configurable reply attribute (default Filter-Id) + default fallback | Fixed vendor attribute; no authorization | Filter-Id is the RFC-standard authz carrier; mirrors tacacs priv-lvl→profile; configurable for real deployments (A-4) |
| Authenticator-only MVP; accounting/authorization deferred | Full auth+authz+acct in one pass | Keeps scope bounded; TACACS+ added acct incrementally too |
| Chain priority to be confirmed (currently 50) | Force radius after tacacs/local | Preserve existing constant unless the user wants different precedence (A-2) |

## Known Limitations
- MVP does not implement RADIUS accounting or CoA for admin sessions (subscriber CoA stays in L2TP).
- Only PAP admin auth in MVP (A-3).
- No dynamic backend-order config; order is the `Priority()` constant.

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Operator can log in via RADIUS | functional + interop | `aaa-radius-admin.ci`, `NN-radius-admin-freeradius` |
| RADIUS unreachable falls back to local | functional | `aaa-radius-fallback.ci` |
| L2TP path unchanged | regression | L2TP radius tests green; empty `git diff` under authradius |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | (to be filled during /ze-implement) | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Pre-Commit Verification
(to be filled during implementation)

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/component/radius/*`)
- [ ] L2TP RADIUS path proven unchanged
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (A-1 is BLOCKING and must be confirmed before implementation)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] A-1 product decision confirmed with the user
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary + Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-radius-admin-backend.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-radius-admin-backend.md`
