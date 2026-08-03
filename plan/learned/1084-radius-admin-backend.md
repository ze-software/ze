# 1084 -- radius-admin-backend

## Context

RADIUS in Ze was subscriber-only: the L2TP `authradius` plugin authenticated PPP
sessions, while operator/admin login (SSH, web, MCP) went through TACACS+ or local
bcrypt. The `radiusBackend.Build()` in `internal/component/radius/aaa.go` returned
an empty `aaa.Contribution` on purpose (the name/priority slot was reserved but no
authenticator was contributed). The user confirmed admin-auth-via-RADIUS is wanted,
so this spec replaced the placeholder with a real PAP authenticator that mirrors the
TACACS+ admin backend, adds a `system/authentication/radius` YANG subtree, maps
Access-Accept reply attributes to ze RBAC profiles, and leaves the L2TP subscriber
path byte-for-byte unchanged.

## Decisions

- Mirrored the TACACS+ backend structure (own YANG under `system/authentication`, `ExtractConfig`, an `aaa.Authenticator`) over extending the L2TP radius plugin: TACACS+ is the proven in-process admin-auth precedent; the L2TP plugin is out-of-process and `l2tp`-scoped, the wrong tier for admin login.
- Reused `internal/component/radius` (the client) unchanged over writing an admin-specific client: it already does Access-Request/failover/User-Password hiding/Response-Authenticator verification.
- PAP (RFC 2865 User-Password) for the MVP over CHAP/EAP: the client already hides User-Password (§5.2); CHAP/EAP deferred to skeleton specs `spec-radius-admin-chap`, `spec-radius-admin-eap`.
- Kept chain priority 50 (RADIUS tried before tacacs=100 and local=200) per user confirmation, over reordering.
- Profiles come from a configurable Access-Accept reply attribute (default Filter-Id §5.11, or Class §5.25) with a `default-profile` fallback, over a fixed vendor attribute or no authorization. Each attribute instance = one profile (mirrors tacacs priv-lvl→profile).
- Defined admin-specific wire constants (`serviceTypeLogin`=1, `attrClass`=25) LOCALLY in `authenticator.go`/`config.go` rather than adding them to the shared `dict.go`, to keep the diff off the wire code the L2TP subscriber path shares (R-1 mitigation).
- Registered the `doctor-radius-admin-unreachable` check from the radius component via `diagnostic.RegisterDoctorCheck` (the "component that is not a plugin" path), NOT by appending a function to the central `doctor/checks_reach.go` (the older tacacs pattern); the owner owns the check.
- Built a new `ze-test radius-mock` (`internal/test/mock/radius`) reusing the production radius wire package, over hand-rolling packets in the `.ci`.
- Deferred the FreeRADIUS interop scenario to `spec-radius-admin-interop-freeradius` (user-approved): the interop harness (`test/interop/interop.py`) is Docker/BGP-peer-only, and the functional `.ci` already drives the real RADIUS wire protocol while the client is shared with the already-interop-tested L2TP path.

## Consequences

- `system/authentication/radius` is now a real admin backend; the effective auth chain is RADIUS(50) → TACACS+(100) → local(200). A reject stops the chain; a timeout/unreachable falls through to the next backend.
- `authBudget` bounds the total time one login spends on RADIUS to `Timeout<<Retries × servers`, clamped to [5s, 2min], so a slow or unreachable server falls through to local instead of hanging the login (an exceeded budget is an infra error, never a reject).
- CHAP, EAP, and FreeRADIUS interop are tracked as skeleton follow-up specs, not silently dropped.
- Adding a new `system/authentication/<x>` backend is now a two-precedent pattern (tacacs, radius): own YANG module, `ExtractConfig`, an authenticator that maps accept/reject/error to the aaa chain semantics.

## Gotchas

- **Doctor tests need build tags.** `TestCollectSchemaListeners_SSHDefault/Explicit` FAIL under a plain `go test ./internal/component/doctor/` because the SSH YANG module is behind `ze_ssh`; without it, `DiscoverListenerServices` still finds the always-on listeners (bmp, l2tp, as112, geodns, wireguard, plugin-hub) so it takes the schema path instead of the hardcoded fallback that extracts SSH. Run doctor tests with `-tags 'ze_core ze_ssh ze_web ze_mcp ...'`. This is a build-tag artifact, not a regression; the radius module has no `ze:listener` and cannot affect listener discovery.
- **`ze:sensitive` redaction is schema-driven** by leaf (`yang_schema.go` `hasSensitiveExtension` → `node.Sensitive`), so a `key` leaf marked `ze:sensitive` is redacted in `show config`/DisplayStrip identically to tacacs. `ze config fmt` still prints the secret because it formats the operator's own on-disk file, not the running-config display; that is expected, not a leak (AC-8).
- **`SendToServers` mutates `pkt.Identifier` per call**, so each `Authenticate` MUST build its own `*Packet` (it does). Sharing one packet across concurrent logins would race on the identifier.
- **The `retries` leaf is the client's attempt count**, and `NewClient` coerces 0→3, so `retries 0` behaves like the default 3 rather than "send once".
- **Parallel-session churn:** a concurrent session was refactoring `bgp/filterapi` across the shared working tree during implementation. Verification was scoped to changed packages (`ai/rules/git-safety.md` Known-Red). A pre-existing `ze-doc-test` stale anchor (`docs/features/rfc-status.md`, an `nlri/*/register.go` glob the anchor checker does not expand) is unrelated to this work.

## Files

Created:
- `internal/component/radius/config.go` -- `ExtractConfig` + `ExtractedConfig` + `HasServers`.
- `internal/component/radius/authenticator.go` -- `radiusAuthenticator`, `authBudget`, `mapProfiles`.
- `internal/component/radius/doctor.go` -- `doctor-radius-admin-unreachable` check + probe.
- `internal/component/radius/yang/{ze-radius-conf.yang,doc.go}` (+ generated `embed.go`,`register.go`).
- `internal/component/radius/{config,authenticator,aaa,doctor}_test.go`.
- `internal/test/mock/radius/{radius.go,doc.go,radius_test.go}` -- `ze-test radius-mock`.
- `test/plugin/{aaa-radius-admin.ci,aaa-radius-fallback.ci}`.
- `plan/spec-radius-admin-{chap,eap,interop-freeradius}.md` -- deferred follow-up skeletons.
- `docs/guide/radius.md`.

Modified:
- `internal/component/radius/{aaa.go,register.go}` -- real `Build`, doctor registration.
- `internal/core/diagnostic/codes.go` -- `doctor-radius-admin-unreachable` metadata.
- `internal/test/cli/register.go` -- register `radius-mock`.
- `internal/component/doctor/doctor_test.go` -- coverage list + dependency inventory.
- `internal/component/plugin/all/all.go` -- blank-import `radius/yang` (generated).
- `rfc/short/rfc2865.md` -- Filter-Id (§5.11) + Class (§5.25) rows.
- `docs/{features.md,guide/README.md,guide/status.md,guide/configuration.md}`.
- `ai/{PACKAGE-MAP.md,DOCS-TO-CODE.md,LEARNED-FULL-INDEX.md}` -- regenerated.
