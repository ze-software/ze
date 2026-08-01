# 831 -- appliance-auth-hardening

## Context

The appliance work introduced a local bootstrap account, generated install artifacts, and new web/API login paths in the same window. Review found two classes of bugs: local admin auth was drifting toward the outbound `meta/ssh/*` client state, and config-file users could authenticate on some surfaces without consistently hitting the same write authorization gates. The installer path also accepted weaker credential/checksum handling than the rest of the daemon. The goal was to harden the bootstrap account boundaries, make web/API writes honor the same RBAC model, and tighten installer/image validation without carrying compatibility code for an unreleased feature.

## Decisions

- Chose dedicated local-admin zefs keys (`meta/auth/local/{username,password}`) over reusing `meta/ssh/*`, because outbound remote selection and local bootstrap auth are different concerns
- Chose to reject `meta/ssh/*`-only databases over adding migration/fallback logic, because the appliance feature is unreleased and compatibility glue would add ambiguous auth behavior for no user benefit
- Chose to thread explicit config mutation auth commands through web `/cli`, REST, and gRPC over relying on "authenticated means writable", so config-file users authenticate broadly but still respect RBAC on write paths
- Chose bcrypt validation at image-server config parse time over accepting arbitrary strings and letting runtime auth semantics decide later
- Chose strict `.sha256` sidecar parsing over permissive empty/malformed handling, because a present checksum file must never silently disable image verification
- Chose `virtio-net-pci` as the installer QEMU default over `e1000`, matching the installer kernel baseline and avoiding test-only driver assumptions

## Consequences

- Local bootstrap auth now depends only on `meta/auth/local/*`; remote-client records no longer influence who can log into hub surfaces
- Config-file users can log into web, API, and SSH after config load, but config mutation paths now consult RBAC consistently on web integrated CLI, REST sessions, and gRPC sessions
- Fresh appliance/init writers must keep populating the local-admin keys; omitting them now fails closed instead of falling back
- Image-server configs with plaintext or malformed `ssh-password-hash` values now fail validation immediately
- Installer environments that publish a checksum sidecar must provide a real SHA-256 digest or the install stops before disk write

## Gotchas

- `usersFromZefsDB` feeds web auth, API auth, and the local AAA bundle; changing it is not a narrow web-only tweak
- The web UI has two config command surfaces, terminal and integrated `/cli`; both need the same authorization treatment
- API login and API write authorization are separate steps; accepting a username/password pair does not imply the caller may open or mutate config sessions
- The live AAA bundle must be built with the extracted authz store on every startup path; rebuilding it without profiles silently turns authenticated API/web writers into allow-all
- Appliance review initially pushed toward migration logic, but unreleased code changes the bar: deleting ambiguous compatibility paths is often safer than preserving them

## Files

- Auth loading and surface wiring: `cmd/ze/hub/main_servers.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/api.go`
- Web/API enforcement: `internal/component/web/cli.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`, `internal/component/api/types.go`
- Local-admin key writers: `internal/plugins/init/main.go`, `internal/appliance/cmd_assemble.go`, `internal/plugins/imageserver/handler.go`, `pkg/zefs/keys.go`
- Validation and install hardening: `internal/plugins/imageserver/config.go`, `tools/installer-initrd/init`, `scripts/evidence/effective-install-qemu.py`
- Regression coverage and docs: `cmd/ze/hub/zefs_users_test.go`, `internal/component/web/cli_test.go`, `test/plugin/rbac-web-config-deny.ci`, `test/parse/image-server-invalid-bcrypt.ci`, `docs/guide/authentication.md`, `docs/guide/ze-install.md`
