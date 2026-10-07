# Live Credential Revocation

## Meta

| Field | Value |
|-------|-------|
| Name | Live Credential Revocation |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/aaa, internal/component/web/auth.go |
| Real-path tests | test/ui/api-user-removed-by-reload.ci, test/ui/web-user-removed-by-reload.ci, cmd/ze/hub/ssh_pubkey_live_test.go::TestInfraSetupSSHPublicKeyFollowsRunningConfig, cmd/ze/hub/main_reload_aaa_test.go::TestDoReloadRebuildsAAABundleFromReloadedConfig |
| Docs | docs/guide/authentication.md, docs/guide/config-reload.md |
| Doc review | 2026-10-07: AuthResult.GrantedByLocalBackend answers Source == SourceLocal in internal/component/aaa/types.go, as the row says |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/component/aaa or web/auth.go |
| Extra criteria | supported: SSH password revocation at reload = test/ui/ssh-user-removed-by-reload.ci |

## Description

Every credential surface reads the running config's user list, so a user the config stops declaring stops authenticating at the next reload: the web password, the web session cookie, the SSH password, the SSH public key, and REST/gRPC Bearer. The revoker is identified rather than assumed. An authentication result names the backend that produced it, and only a result the local backend granted is revoked by the local list; an empty source is not local, because reading silence as local would attach the local list's revocation to a session some other backend granted. A session a RADIUS or TACACS+ backend granted is therefore not revoked by an edit to the local users. Revocation governs new authentication; an open session survives by design. <!-- source: internal/component/aaa/types.go -- AuthResult.Source, GrantedByLocalBackend --> <!-- source: internal/component/web/auth.go -- cookie re-check against the running user list -->
