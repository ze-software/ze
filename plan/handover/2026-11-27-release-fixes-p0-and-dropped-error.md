# Handover: Release P0 fixes and dropped-error umbrella complete; ready for P0 approvals and P1 fixes

Written 2026-11-27 after completing Items 1 and 2 of Thomas's instruction ("You can do one and two. Then come back to me to speak about three."). No spec is currently claimed by this session (`./le spec current` is empty). Remove this file through `./le commit create ... remove` once Thomas has reviewed the remaining P0s and decided how to sequence Item 3 (P1 release fixes).

## Handoff

**Spec:** none (all claimed specs closed and released)
**Branch:** `main` (HEAD `78d7a9ec4c`, 12 commits ahead of `origin/main` at `0a2661a511`; do NOT push without owner order)
**Goal:** Hand over a clean checkpoint after closing every zero-question P0 fix and the entire `dropped-error` umbrella, so the next session can act immediately on Thomas's approvals for the remaining P0s or start up to 3 judged `0-departs` P1 release fix specs at a time.

### Standing Owner Constraints

- **Concurrency cap:** Never implement more than **3 specs at a time** (explicit user instruction).
- **Away grant active (`2026-11-27` to `2026-11-30`):** Judged `0-departs` release fix specs may start without a fresh `./le spec design approve`, while any spec with a `departs` row or open owner choices MUST wait for Thomas's approval (`./le spec design pending` tracks pending starts).
- **Do not start Item 3 (P1 fixes) unprompted:** Thomas explicitly asked to discuss Item 3 before starting P1 implementation.
- **Shared checkout hygiene:** Sibling sessions have uncommitted work in the tree (notably `plan/journal/*`, `plan/release/fixes/spec-*-one-off-defects*`, `internal/component/bgp/format/raw_b64*`, `internal/component/plugin/server/command.go`, `docs/architecture/api/commands.md`, `docs/plugin-development/metrics.md`). Always commit explicit `file` / `file-list` / `hunks` paths via `./le commit create` and never touch unrelated dirty files.

### Status

- **Done (committed on `main`):**
  - **Item 2 — Complete `dropped-error` umbrella (100% done, already on `origin/main`):**
    - `spec-dropped-error-2-bgp-plugins` (`9d583df329` + `3c7535e4c6`) — all 63 sites resolved across 24 files, RFC 8671 `abortAdjReplay` fix, BMP `awaitingHandshake` gate.
    - `spec-dropped-error-9-api-cli-config` (`4ff83d07f1` + `0c388957ae`) — all 8 sites resolved, `doctor-resolve-ptr-failed` registered.
    - `spec-dropped-error-0-umbrella` (`e53fd6d67e` + `61f3aaaeff`) — tree-wide `./le arch refuse-by-default report '|' json` verified at **0** shipped-release `error-dropped-below-warn` findings.
  - **Item 1 — Zero-question P0 release fixes (100% done):**
    - `spec-pki-show-private-key` (`ad2c5de4b7` + `0a2661a511`, on `origin/main`) — explicit RBAC (`RequireExplicitAuthz` / `ExplicitAuthorizer`), mandatory YANG `confirm`, redacted `pki-private-key-export` audit, fail-closed `redact.PrivateKeyPEM`, `test/plugin/pki-show-private-key.ci`.
    - `spec-login-service-authorisation` (`3f90b4e3f4` + `cb680096c3`, local `main`, unpushed) — stdlib-only `internal/component/config/loginservice` registry, 5 `register_login_service.go` surface self-registrations (`ssh`, `web`, `rest`, `grpc`, `mcp`), YANG `system/authorization/login-service` + leafrefs on `user`, `tacacs-profile`, and `radius`, fail-closed `aaa.CheckLoginService` / `CheckLoginSet` across all 6 authentication paths, live web cookie & MCP reload revocation, `doctor-aaa-all-accounts-excluded`, `ze_aaa_login_service_refused_total{service}`, 22 unit tests (`-race`), 20 promoted `.ci` tests (`90/90` stress, `16/16` discrimination), real FreeRADIUS interop `radius-admin-login-service-refused-freeradius` PASS, 2-round independent Review Gate CLEAN.
    - `plan/release/fixes/spec-config-archive-location-credentials.md` (`02cb0f242f`, local `main`, unpushed) — upgraded from `skeleton` to full `design`, judged **APPROVE** (`digest f10224e6641b`, `ready to present`), ready for Thomas's approval.
- **In progress:**
  - None. Zero active claims, zero uncommitted changes belonging to this session, zero running background tasks or subagents.
- **Remaining:**
  - **8 open P0 specs in `plan/release/fixes/` (all need Thomas's approval or design rulings before implementation):**
    1. `plan/release/fixes/spec-config-archive-location-credentials.md` (`design`, judged `APPROVE` at `f10224e6641b`) — 2 honest `departs` rows on `docs/guide/config-archive.md` (loopback-only `http://`, refuse userinfo in `location`) plus choices C-1 (flat `username`/`password` vs nested `authentication`) and C-2 (ignore vs refuse credentials on `file://`).
    2. `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` (`design`) — dependency on `spec-login-service-authorisation` is now satisfied; carries `departs` rows waiting for `./le spec design approve`.
    3. `plan/release/fixes/spec-selfupdate-manifest-authenticity.md` (`ready`) — 1 `departs` row on `self-update.md` waiting for `./le spec design approve` (also note `./le spec design check` previously flagged a newly anchored row `docs/architecture/diagnostics/production-diagnostics.md` after a later commit).
    4. `plan/release/fixes/spec-fixit-ca-root-location.md` (`design`) — carries `departs` rows waiting for `./le spec design approve`.
    5. `plan/release/fixes/spec-plugin-runner-inertness.md` (`design`) — carries `departs` rows waiting for `./le spec design approve`.
    6. `plan/release/fixes/spec-rfc5176-remaining-defect-classes.md` (`design`) — carries `departs` rows waiting for `./le spec design approve`.
    7. `plan/release/fixes/spec-ipsec-remote-access.md` (`design`) — carries `departs` rows waiting for `./le spec design approve`.
    8. `plan/release/fixes/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` (`skeleton`) — 3 open owner questions (A-1, A-4, A-5: leak-probe driver and `max-mitigation-duration`).
  - **Item 3 — 24 judged `0-departs` P1 release fix specs in `plan/release/fixes/` (do not start until Thomas agrees on priority/order):**
    - `ready` (1): `spec-single-owner-lifecycle.md`
    - `design` (23): `spec-anomaly-shape-limit-drops-nothing.md`, `spec-bgp-add-peer-listener-defects.md`, `spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md`, `spec-bgp-import-prepend-guards.md`, `spec-bgp-ls-receiver-accept-and-limit-controls.md`, `spec-bgp-mup-segment-discovery-originator-checks.md`, `spec-bgp-prefix-count-mode-metric.md`, `spec-bgp-received-update-counter-counts-wire-messages.md`, `spec-bgp-rib-state-subtree-has-no-producer.md`, `spec-bgp-rpki-ov-community-ebgp-boundary.md`, `spec-config-yang-loader-structural-checks.md`, `spec-ipsec-lifetime-volume.md`, `spec-rsvp-routing-and-interface-integration.md`, `spec-rsvpte-non-rsvp-neighbors.md`, `spec-sflow-export-rfc-defects.md`, `spec-single-owner-reload-reconcile-2-iface-reconcile.md`, `spec-single-owner-reload-reconcile.md`, `spec-srv6-bestpath-resolvability.md`, `spec-typed-identity-keys.md`, `spec-vpp-interface-in-use.md`, `spec-vpp-reply-deadline-iface-fib-static.md`, `spec-yang-enum-vocabulary-agreement-gate.md`, `spec-yang-single-source-of-truth.md`.
- **Not done, and the spec that owns each item:**
  - Per-user identity and `"looking-glass"` / `"gnmi"` registration into `internal/component/config/loginservice` -> `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`.
  - Remaining 5 post-release `error-dropped-below-warn` sites (3 dev tooling, 2 IKE MOBIKE) -> `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` and `plan/post-release/features/spec-ipsec-11-mobike.md`.
  - Pushing local commits (`3f90b4e3f4`, `cb680096c3`, `02cb0f242f`, plus this handover) to `origin/main` -> awaits explicit owner authorization via `./le commit push`.

### Files Already Handled (don't re-read)

- `internal/component/config/loginservice/registry.go` and `internal/component/config/loginservice/registry_test.go` — stdlib-only login-service name registry (`Register`, `MustRegister`, `ErrConflict`, `Names`, `Registered`, `Lookup`).
- `internal/component/{ssh,web,api/rest,api/grpc,mcp}/register_login_service.go` — surface `init()` registrations (`ssh`, `web`, `rest`, `grpc`, `mcp`).
- `internal/component/aaa/{types.go,login_profiles.go,doctor.go,register.go,register_doctor_codes.go}` — fail-closed `CheckLoginService` / `CheckLoginSet`, `ze_aaa_login_service_refused_total{service}`, and `doctor-aaa-all-accounts-excluded`.
- `cmd/ze/hub/{aaa_lifecycle.go,api.go,main.go,main_reload.go,service_mcp.go,service_registry.go}` — atomic `acceptedLocalIdentityState.loginSets`, web fallback gate, and MCP `LoginServiceGate` wiring.
- `test/interop-radius/Dockerfile.ze`, `internal/le/interoplab/radius/{radius.go,check_login_service.go}`, and `test/interop-radius/scenarios/radius-admin-login-service-refused-freeradius/` — real FreeRADIUS interop proof for `radius.login-service` using pre-initialized `/var/lib/ze-web-store`.
- `plan/release/fixes/spec-config-archive-location-credentials.md` — fully designed and judged `APPROVE` (`f10224e6641b`); do not re-research before Thomas approves or modifies C-1/C-2.

### Edits

No code edits are pending right now. Once Thomas replies, execute the matching branch below (at most 3 specs claimed at a time):

EDIT 1: If Thomas approves `plan/release/fixes/spec-config-archive-location-credentials.md` (recording C-1 and C-2 via `./le spec design approve` on his terminal)
- Action: Claim `plan/release/fixes/spec-config-archive-location-credentials.md` and implement `ze-system-conf.yang` (`username` and `password` with `ze:sensitive`), `ValidateArchiveLocation` (`validators_archive.go` reusing `loopbackFetchHosts`), `ArchiveConfig`/`ExtractConfigs`/`ToHTTP` (`archive/archive.go`), `config.RegisterServiceBlock("system/archive[*]", ...)` (`archive/register.go`), `test/ui/config-archive-credentials.ci`, and `docs/guide/config-archive.md`.

EDIT 2: If Thomas approves `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`
- Action: Claim `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` (now unblocked by `3f90b4e3f4`) and register `"looking-glass"` and `"gnmi"` in `internal/component/config/loginservice` using `register_login_service.go` (matching the `./le go style check` `init()` rule).

EDIT 3: If Thomas answers A-1/A-4/A-5 on `plan/release/fixes/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md`
- Action: Upgrade `plan/release/fixes/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` from `skeleton` to `design` with his answers, judge via `ze-design-judge`, and commit the judged design spec.

EDIT 4: If Thomas directs work onto Item 3 (P1 release fixes)
- Action: Pick up to 3 specs from the 24 judged `0-departs` P1 list above (re-running `./le spec design check <spec>` first on each to verify digest freshness against HEAD), claim them, and implement through TDD + independent `/ze-close` Review Gate.

### Then

Before starting the next spec in a fresh session, verify repo cleanliness and design-gate status on your chosen targets:

```bash
./le spec current
./le spec design check plan/release/fixes/spec-config-archive-location-credentials.md
./le spec design check plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md
git log --oneline origin/main..HEAD
```

*(Note: when running `./le test unit pkg`, `.ci` suites, or `./le commit create` in this environment, run with tmpfs/git access outside the read-only sandbox, and pass `--name <label>` to `./le` whenever `internal/test/fixture/` or `internal/le/interoplab/` changed so `le` rebuilds from the working tree.)*
